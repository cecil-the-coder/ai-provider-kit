package errors

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewErrorContextMiddleware(t *testing.T) {
	config := DefaultErrorContextMiddlewareConfig(types.ProviderTypeAnthropic)
	mw := NewErrorContextMiddleware(config)

	if mw == nil {
		t.Fatal("NewErrorContextMiddleware returned nil")
	}

	if mw.provider != types.ProviderTypeAnthropic {
		t.Errorf("Expected provider anthropic, got: %s", mw.provider)
	}

	if !mw.generateRequestID {
		t.Error("Expected generateRequestID to be true by default")
	}

	if !mw.generateCorrelationID {
		t.Error("Expected generateCorrelationID to be true by default")
	}

	if mw.correlationIDHeader != "X-Correlation-ID" {
		t.Errorf("Expected correlationIDHeader X-Correlation-ID, got: %s", mw.correlationIDHeader)
	}
}

func TestNewErrorContextMiddleware_NilConfig(t *testing.T) {
	mw := NewErrorContextMiddleware(nil)

	if mw == nil {
		t.Fatal("NewErrorContextMiddleware returned nil")
	}

	if mw.config == nil {
		t.Error("Expected default config to be set")
	}
}

func TestErrorContextMiddleware_ProcessRequest(t *testing.T) {
	config := DefaultErrorContextMiddlewareConfig(types.ProviderTypeOpenAI)
	mw := NewErrorContextMiddleware(config)

	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBufferString(`{"model": "gpt-4"}`))
	ctx := context.Background()

	newCtx, newReq, err := mw.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}

	// Check that error context was added
	errCtx := GetErrorContext(newCtx)
	if errCtx == nil {
		t.Fatal("Expected error context to be added")
	}

	// Check that request ID was generated and added to header
	if errCtx.RequestID == "" {
		t.Error("Expected request ID to be generated")
	}

	if newReq.Header.Get("X-Request-ID") == "" {
		t.Error("Expected X-Request-ID header to be set")
	}

	if newReq.Header.Get("X-Request-ID") != errCtx.RequestID {
		t.Error("Expected X-Request-ID header to match context request ID")
	}

	// Check that correlation ID was generated
	if errCtx.CorrelationID == "" {
		t.Error("Expected correlation ID to be generated")
	}

	if newReq.Header.Get("X-Correlation-ID") == "" {
		t.Error("Expected X-Correlation-ID header to be set")
	}

	// Check that provider was set
	if errCtx.Provider != types.ProviderTypeOpenAI {
		t.Errorf("Expected provider openai, got: %s", errCtx.Provider)
	}

	// Check that timestamp was set
	if errCtx.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	// Check that request snapshot was created
	if errCtx.Request == nil {
		t.Error("Expected request snapshot to be created")
	}
}

func TestErrorContextMiddleware_ProcessRequest_ExistingIDs(t *testing.T) {
	config := DefaultErrorContextMiddlewareConfig(types.ProviderTypeAnthropic)
	mw := NewErrorContextMiddleware(config)

	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
	req.Header.Set("X-Request-ID", "existing-req-id")
	req.Header.Set("X-Correlation-ID", "existing-corr-id")

	ctx := context.Background()

	newCtx, newReq, err := mw.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}

	errCtx := GetErrorContext(newCtx)
	if errCtx == nil {
		t.Fatal("Expected error context to be added")
	}

	// Should use existing IDs
	if errCtx.RequestID != "existing-req-id" {
		t.Errorf("Expected request ID existing-req-id, got: %s", errCtx.RequestID)
	}

	if errCtx.CorrelationID != "existing-corr-id" {
		t.Errorf("Expected correlation ID existing-corr-id, got: %s", errCtx.CorrelationID)
	}

	// Headers should not be changed
	if newReq.Header.Get("X-Request-ID") != "existing-req-id" {
		t.Error("Expected X-Request-ID header to preserve existing value")
	}

	if newReq.Header.Get("X-Correlation-ID") != "existing-corr-id" {
		t.Error("Expected X-Correlation-ID header to preserve existing value")
	}
}

func TestErrorContextMiddleware_ProcessRequest_ContextValues(t *testing.T) {
	config := DefaultErrorContextMiddlewareConfig(types.ProviderTypeOpenAI)
	mw := NewErrorContextMiddleware(config)

	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)

	// Add provider and model to context
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyProvider, "custom-provider")
	ctx = context.WithValue(ctx, middleware.ContextKeyModel, "gpt-4-turbo")

	newCtx, _, err := mw.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}

	errCtx := GetErrorContext(newCtx)
	if errCtx == nil {
		t.Fatal("Expected error context to be added")
	}

	// Should extract from context
	if string(errCtx.Provider) != "custom-provider" {
		t.Errorf("Expected provider custom-provider, got: %s", errCtx.Provider)
	}

	if errCtx.Model != "gpt-4-turbo" {
		t.Errorf("Expected model gpt-4-turbo, got: %s", errCtx.Model)
	}
}

func TestErrorContextMiddleware_ProcessResponse(t *testing.T) {
	config := DefaultErrorContextMiddlewareConfig(types.ProviderTypeAnthropic)
	mw := NewErrorContextMiddleware(config)

	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)

	// First process the request to set up context
	ctx, _, err := mw.ProcessRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}

	// Get the error context before processing response
	errCtxBefore := GetErrorContext(ctx)
	timestampBefore := errCtxBefore.Timestamp

	// Wait a bit to ensure duration is measurable
	time.Sleep(10 * time.Millisecond)

	// Create a response
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewBufferString(`{"id": "msg-123"}`)),
	}
	resp.Header.Set("Content-Type", "application/json")

	// Process the response
	newCtx, newResp, err := mw.ProcessResponse(ctx, req, resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	errCtx := GetErrorContext(newCtx)
	if errCtx == nil {
		t.Fatal("Expected error context to be present")
	}

	// Check that duration was calculated
	if errCtx.Duration <= 0 {
		t.Error("Expected positive duration")
	}

	// Check that timestamp wasn't changed
	if errCtx.Timestamp != timestampBefore {
		t.Error("Expected timestamp to remain unchanged")
	}

	// Check that response snapshot was created
	if errCtx.Response == nil {
		t.Error("Expected response snapshot to be created")
	}

	if errCtx.Response.StatusCode != 200 {
		t.Errorf("Expected status code 200, got: %d", errCtx.Response.StatusCode)
	}

	// Response should still be usable
	body, err := io.ReadAll(newResp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !strings.Contains(string(body), "msg-123") {
		t.Errorf("Expected response body to be readable, got: %s", string(body))
	}
}

func TestErrorContextMiddleware_ProcessResponse_NoContext(t *testing.T) {
	config := DefaultErrorContextMiddlewareConfig(types.ProviderTypeOpenAI)
	mw := NewErrorContextMiddleware(config)

	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"id": "chatcmpl-123"}`)),
	}

	// Process response without first processing request
	ctx := context.Background()
	newCtx, _, err := mw.ProcessResponse(ctx, req, resp)
	if err != nil {
		t.Fatalf("ProcessResponse failed: %v", err)
	}

	// Should create a new error context
	errCtx := GetErrorContext(newCtx)
	if errCtx == nil {
		t.Fatal("Expected error context to be created")
	}

	if errCtx.Response == nil {
		t.Error("Expected response snapshot to be created")
	}
}

func TestGetErrorContext(t *testing.T) {
	t.Run("with error context", func(t *testing.T) {
		errCtx := NewErrorContext().WithRequestID("test-123")
		ctx := context.WithValue(context.Background(), ContextKeyErrorContext, errCtx)

		retrieved := GetErrorContext(ctx)
		if retrieved == nil {
			t.Fatal("Expected error context to be retrieved")
		}

		if retrieved.RequestID != "test-123" {
			t.Errorf("Expected RequestID test-123, got: %s", retrieved.RequestID)
		}
	})

	t.Run("without error context", func(t *testing.T) {
		ctx := context.Background()
		retrieved := GetErrorContext(ctx)

		if retrieved != nil {
			t.Error("Expected nil for context without error context")
		}
	})

	t.Run("nil context", func(t *testing.T) {
		//nolint:staticcheck // SA1012: testing nil context behavior
		retrieved := GetErrorContext(nil)

		if retrieved != nil {
			t.Error("Expected nil for nil context")
		}
	})
}

func TestGetCorrelationID(t *testing.T) {
	t.Run("from error context", func(t *testing.T) {
		errCtx := NewErrorContext().WithCorrelationID("corr-from-errctx")
		ctx := context.WithValue(context.Background(), ContextKeyErrorContext, errCtx)

		id := GetCorrelationID(ctx)
		if id != "corr-from-errctx" {
			t.Errorf("Expected corr-from-errctx, got: %s", id)
		}
	})

	t.Run("from direct context value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ContextKeyCorrelationID, "corr-direct")

		id := GetCorrelationID(ctx)
		if id != "corr-direct" {
			t.Errorf("Expected corr-direct, got: %s", id)
		}
	})

	t.Run("without correlation ID", func(t *testing.T) {
		ctx := context.Background()
		id := GetCorrelationID(ctx)

		if id != "" {
			t.Errorf("Expected empty string, got: %s", id)
		}
	})

	t.Run("nil context", func(t *testing.T) {
		//nolint:staticcheck // SA1012: testing nil context behavior
		id := GetCorrelationID(nil)

		if id != "" {
			t.Errorf("Expected empty string, got: %s", id)
		}
	})
}

func TestEnrichError(t *testing.T) {
	t.Run("with error context", func(t *testing.T) {
		baseErr := context.DeadlineExceeded
		errCtx := NewErrorContext().
			WithRequestID("req-123").
			WithProvider(types.ProviderTypeOpenAI)

		ctx := context.WithValue(context.Background(), ContextKeyErrorContext, errCtx)

		enriched := EnrichError(ctx, baseErr)
		if enriched == nil {
			t.Fatal("Expected enriched error")
		}

		richErr, ok := enriched.(*RichError)
		if !ok {
			t.Fatal("Expected RichError type")
		}

		if richErr.context.RequestID != "req-123" {
			t.Errorf("Expected RequestID req-123, got: %s", richErr.context.RequestID)
		}
	})

	t.Run("without error context", func(t *testing.T) {
		baseErr := context.DeadlineExceeded
		ctx := context.Background()

		enriched := EnrichError(ctx, baseErr)
		if enriched != baseErr {
			t.Error("Expected original error when no error context")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		ctx := context.Background()
		enriched := EnrichError(ctx, nil)

		if enriched != nil {
			t.Error("Expected nil for nil error")
		}
	})
}

func TestCorrelationMiddleware(t *testing.T) {
	mw := NewCorrelationMiddleware("X-Trace-ID", true)

	if mw.headerName != "X-Trace-ID" {
		t.Errorf("Expected headerName X-Trace-ID, got: %s", mw.headerName)
	}

	if !mw.generate {
		t.Error("Expected generate to be true")
	}
}

func TestCorrelationMiddleware_ProcessRequest(t *testing.T) {
	t.Run("generate new ID", func(t *testing.T) {
		mw := NewCorrelationMiddleware("X-Trace-ID", true)
		req := httptest.NewRequest("GET", "https://api.example.com/test", nil)
		ctx := context.Background()

		newCtx, newReq, err := mw.ProcessRequest(ctx, req)
		if err != nil {
			t.Fatalf("ProcessRequest failed: %v", err)
		}

		// Should generate and set header
		traceID := newReq.Header.Get("X-Trace-ID")
		if traceID == "" {
			t.Error("Expected X-Trace-ID header to be set")
		}

		// Should be in context
		ctxID := GetCorrelationID(newCtx)
		if ctxID != traceID {
			t.Errorf("Expected correlation ID in context to match header, got: %s vs %s", ctxID, traceID)
		}
	})

	t.Run("use existing ID", func(t *testing.T) {
		mw := NewCorrelationMiddleware("X-Trace-ID", true)
		req := httptest.NewRequest("GET", "https://api.example.com/test", nil)
		req.Header.Set("X-Trace-ID", "existing-trace-id")
		ctx := context.Background()

		newCtx, newReq, err := mw.ProcessRequest(ctx, req)
		if err != nil {
			t.Fatalf("ProcessRequest failed: %v", err)
		}

		// Should use existing header
		if newReq.Header.Get("X-Trace-ID") != "existing-trace-id" {
			t.Error("Expected existing X-Trace-ID to be preserved")
		}

		// Should be in context
		ctxID := GetCorrelationID(newCtx)
		if ctxID != "existing-trace-id" {
			t.Errorf("Expected correlation ID existing-trace-id, got: %s", ctxID)
		}
	})

	t.Run("don't generate", func(t *testing.T) {
		mw := NewCorrelationMiddleware("X-Trace-ID", false)
		req := httptest.NewRequest("GET", "https://api.example.com/test", nil)
		ctx := context.Background()

		newCtx, newReq, err := mw.ProcessRequest(ctx, req)
		if err != nil {
			t.Fatalf("ProcessRequest failed: %v", err)
		}

		// Should not generate header
		if newReq.Header.Get("X-Trace-ID") != "" {
			t.Error("Expected X-Trace-ID header not to be generated")
		}

		// Should not be in context
		ctxID := GetCorrelationID(newCtx)
		if ctxID != "" {
			t.Errorf("Expected empty correlation ID, got: %s", ctxID)
		}
	})
}

func TestCorrelationMiddleware_DefaultHeader(t *testing.T) {
	mw := NewCorrelationMiddleware("", true)

	if mw.headerName != "X-Correlation-ID" {
		t.Errorf("Expected default header X-Correlation-ID, got: %s", mw.headerName)
	}
}

func TestDefaultErrorContextMiddlewareConfig(t *testing.T) {
	config := DefaultErrorContextMiddlewareConfig(types.ProviderTypeAnthropic)

	if config.Provider != types.ProviderTypeAnthropic {
		t.Errorf("Expected provider anthropic, got: %s", config.Provider)
	}

	if config.SnapshotConfig == nil {
		t.Error("Expected SnapshotConfig to be set")
	}

	if !config.GenerateRequestID {
		t.Error("Expected GenerateRequestID to be true")
	}

	if !config.GenerateCorrelationID {
		t.Error("Expected GenerateCorrelationID to be true")
	}

	if config.CorrelationIDHeader != "X-Correlation-ID" {
		t.Errorf("Expected CorrelationIDHeader X-Correlation-ID, got: %s", config.CorrelationIDHeader)
	}
}
