package errors_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/errors"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Example_basicErrorWrapping demonstrates wrapping an error with rich context
func Example_basicErrorWrapping() {
	// Simulate an API error
	baseErr := fmt.Errorf("API request failed: rate limit exceeded")

	// Wrap with rich context
	richErr := errors.Wrap(baseErr).
		WithRequestID("req-abc-123").
		WithProvider(types.ProviderTypeOpenAI).
		WithModel("gpt-4").
		WithOperation("chat_completion").
		WithTiming(250 * time.Millisecond)

	// Print the error
	fmt.Println(richErr.Error())
	// Output: API request failed: rate limit exceeded
}

// Example_credentialMasking demonstrates automatic credential masking
func Example_credentialMasking() {
	masker := errors.DefaultCredentialMasker()

	// Mask API keys in JSON
	jsonData := `{"model": "gpt-4", "api_key": "sk-1234567890abcdef"}`
	masked := masker.MaskString(jsonData)
	fmt.Println(masked)

	// Mask headers
	headers := http.Header{
		"Authorization": []string{"Bearer secret-token"},
		"Content-Type":  []string{"application/json"},
	}
	maskedHeaders := masker.MaskHeaders(headers)
	fmt.Println("Authorization:", maskedHeaders["Authorization"][0])
	fmt.Println("Content-Type:", maskedHeaders["Content-Type"][0])

	// Output:
	// {"model": "gpt-4", "api_key": "***MASKED***"}
	// Authorization: ***MASKED***
	// Content-Type: application/json
}

// Example_customMaskingPattern demonstrates adding custom masking patterns
func Example_customMaskingPattern() {
	masker := errors.NewCredentialMasker()

	// Add custom pattern for session IDs
	masker.AddPattern(
		regexp.MustCompile(`session_id=([A-Za-z0-9]+)`),
		"session_id=***MASKED***",
	)

	data := "User request with session_id=abc123def456"
	masked := masker.MaskString(data)
	fmt.Println(masked)

	// Output: User request with session_id=***MASKED***
}

// Example_requestSnapshot demonstrates capturing request snapshots
func Example_requestSnapshot() {
	// Create a sample request
	body := `{"model": "claude-3-opus", "messages": [{"role": "user", "content": "Hello"}]}`
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBufferString(body))
	req.Header.Set("X-API-Key", "sk-ant-secret-key")
	req.Header.Set("Content-Type", "application/json")

	// Create snapshot with default config
	config := errors.DefaultSnapshotConfig()
	snapshot := errors.NewRequestSnapshot(req, config)

	fmt.Println("Method:", snapshot.Method)
	fmt.Println("URL:", snapshot.URL)
	fmt.Println("Has headers:", len(snapshot.Headers) > 0)
	fmt.Println("Has body:", snapshot.Body != "")
	fmt.Println("Body truncated:", snapshot.BodyTruncated)

	// Output:
	// Method: POST
	// URL: https://api.anthropic.com/v1/messages
	// Has headers: true
	// Has body: true
	// Body truncated: false
}

// Example_errorContextMiddleware demonstrates using middleware for automatic context capture
func Example_errorContextMiddleware() {
	// Create middleware
	config := errors.DefaultErrorContextMiddlewareConfig(types.ProviderTypeAnthropic)
	errorMw := errors.NewErrorContextMiddleware(config)

	// Create middleware chain
	chain := middleware.NewMiddlewareChain()
	chain.Add(errorMw)

	// Create request
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBufferString(`{}`))
	ctx := context.Background()

	// Process request
	ctx, _, _ = chain.ProcessRequest(ctx, req)

	// Get error context
	errCtx := errors.GetErrorContext(ctx)
	if errCtx != nil {
		fmt.Println("Provider:", errCtx.Provider)
		fmt.Println("Has request ID:", errCtx.RequestID != "")
		fmt.Println("Has correlation ID:", errCtx.CorrelationID != "")
		fmt.Println("Has request snapshot:", errCtx.Request != nil)
	}

	// Output:
	// Provider: anthropic
	// Has request ID: true
	// Has correlation ID: true
	// Has request snapshot: true
}

// Example_enrichError demonstrates enriching errors from context
func Example_enrichError() {
	// Simulate an error occurring during a request
	baseErr := fmt.Errorf("connection timeout")

	// Create error context
	errCtx := errors.NewErrorContext().
		WithRequestID("req-123").
		WithProvider(types.ProviderTypeOpenAI).
		WithModel("gpt-4").
		WithDuration(30 * time.Second)

	// Store in context
	ctx := context.WithValue(context.Background(), errors.ContextKeyErrorContext, errCtx)

	// Enrich the error
	enriched := errors.EnrichError(ctx, baseErr)

	// The enriched error has full context
	fmt.Println(enriched.Error())

	// Output: connection timeout
}

// Example_correlationMiddleware demonstrates simple correlation tracking
func Example_correlationMiddleware() {
	// Create correlation middleware
	corrMw := errors.NewCorrelationMiddleware("X-Trace-ID", true)

	// Create request
	req, _ := http.NewRequest("GET", "https://api.example.com/test", nil)
	ctx := context.Background()

	// Process request
	ctx, req, _ = corrMw.ProcessRequest(ctx, req)

	// Retrieve correlation ID
	correlationID := errors.GetCorrelationID(ctx)
	headerID := req.Header.Get("X-Trace-ID")

	fmt.Println("Has correlation ID:", correlationID != "")
	fmt.Println("Header matches context:", correlationID == headerID)

	// Output:
	// Has correlation ID: true
	// Header matches context: true
}

// Example_completeWorkflow demonstrates a complete error handling workflow
func Example_completeWorkflow() {
	// 1. Create middleware
	config := errors.DefaultErrorContextMiddlewareConfig(types.ProviderTypeOpenAI)
	errorMw := errors.NewErrorContextMiddleware(config)

	chain := middleware.NewMiddlewareChain()
	chain.Add(errorMw)

	// 2. Make a request
	body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBufferString(body))
	ctx := context.Background()

	// 3. Process request
	ctx, req, _ = chain.ProcessRequest(ctx, req)

	// 4. Simulate a response
	resp := &http.Response{
		StatusCode: 401,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(`{"error": {"message": "Invalid API key"}}`)),
	}

	// 5. Process response
	ctx, _, _ = chain.ProcessResponse(ctx, req, resp)

	// 6. Handle error
	apiErr := fmt.Errorf("API authentication failed")
	enrichedErr := errors.EnrichError(ctx, apiErr)

	// 7. Check the enriched error
	if richErr, ok := enrichedErr.(*errors.RichError); ok {
		errCtx := richErr.Context()
		fmt.Println("Provider:", errCtx.Provider)
		fmt.Println("Has request:", errCtx.Request != nil)
		fmt.Println("Has response:", errCtx.Response != nil)
		fmt.Println("Response status:", errCtx.Response.StatusCode)
	}

	// Output:
	// Provider: openai
	// Has request: true
	// Has response: true
	// Response status: 401
}

// Example_customSnapshotConfig demonstrates configuring snapshot capture
func Example_customSnapshotConfig() {
	// Create custom config
	config := &errors.SnapshotConfig{
		MaxBodySize:    1024, // Only capture first 1KB
		IncludeHeaders: true,
		IncludeBody:    false, // Don't capture body
		Masker:         errors.DefaultCredentialMasker(),
	}

	// Create a request
	req, _ := http.NewRequest("POST", "https://api.example.com/v1/chat", bytes.NewBufferString(`{"test": "data"}`))
	req.Header.Set("Content-Type", "application/json")
	snapshot := errors.NewRequestSnapshot(req, config)

	fmt.Println("Method:", snapshot.Method)
	fmt.Println("Has headers:", len(snapshot.Headers) > 0)
	fmt.Println("Has body:", snapshot.Body != "")

	// Output:
	// Method: POST
	// Has headers: true
	// Has body: false
}
