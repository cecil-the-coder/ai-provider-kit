// Package streaming provides a shared streaming request executor for AI providers.
// This consolidates common streaming patterns to reduce boilerplate across providers.
package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// StreamCreator creates a ChatCompletionStream from an HTTP response
type StreamCreator func(*http.Response) types.ChatCompletionStream

// RequestPreparer allows providers to customize request before sending
type RequestPreparer func(*http.Request) error

// ResponseHandler allows providers to handle response before creating stream
type ResponseHandler func(*http.Response) error

// StreamingExecutorConfig holds configuration for the streaming executor
type StreamingExecutorConfig struct {
	// Provider type for error reporting
	ProviderType types.ProviderType

	// HTTP client for making requests
	Client *http.Client

	// Rate limit helper for tracking rate limits
	RateLimitHelper *common.RateLimitHelper

	// StreamCreator converts the HTTP response to a ChatCompletionStream
	StreamCreator StreamCreator

	// Optional: PrepareRequest allows modifying the request before sending
	PrepareRequest RequestPreparer

	// Optional: HandleResponse allows processing the response before creating stream
	HandleResponse ResponseHandler

	// Optional: ExtraHeaders to add to all requests
	ExtraHeaders map[string]string

	// Optional: BaseURL for the provider
	BaseURL string

	// Optional: Endpoint path for streaming (default: /chat/completions or provider-specific)
	Endpoint string
}

// StreamingExecutor handles common streaming request execution patterns
type StreamingExecutor struct {
	config StreamingExecutorConfig
}

// NewStreamingExecutor creates a new streaming executor
func NewStreamingExecutor(config StreamingExecutorConfig) *StreamingExecutor {
	return &StreamingExecutor{
		config: config,
	}
}

// ExecuteStreamRequest executes a streaming request with standard error handling
// and rate limit tracking. This consolidates the common pattern found across
// Anthropic, OpenAI, Qwen, Gemini, and other providers.
func (e *StreamingExecutor) ExecuteStreamRequest(
	ctx context.Context,
	method, url string,
	requestBody interface{},
	authToken string,
	authType string,
) (types.ChatCompletionStream, error) {
	// Prepare request
	req, err := e.prepareRequest(ctx, method, url, requestBody)
	if err != nil {
		return nil, e.wrapError("failed to prepare request", err)
	}

	// Set authentication headers
	e.setAuthHeaders(req, authToken, authType)

	// Set standard headers
	e.setStandardHeaders(req)

	// Set extra headers if configured
	e.setExtraHeaders(req)

	// Allow provider-specific request preparation
	if e.config.PrepareRequest != nil {
		if err := e.config.PrepareRequest(req); err != nil {
			return nil, e.wrapError("request preparation failed", err)
		}
	}

	// Execute request
	resp, err := e.config.Client.Do(req)
	if err != nil {
		return nil, e.wrapError("request failed", err)
	}

	// Track rate limits
	e.trackRateLimits(resp, requestBody)

	// Handle non-OK responses
	if resp.StatusCode != http.StatusOK {
		return nil, e.handleErrorResponse(resp)
	}

	// Allow provider-specific response handling
	if e.config.HandleResponse != nil {
		if err := e.config.HandleResponse(resp); err != nil {
			return nil, err
		}
	}

	// Create stream from response
	stream := e.config.StreamCreator(resp)

	// Wrap with context awareness for cancellation support
	return StreamFromContext(ctx, stream), nil
}

// ExecuteWithJSONBody is a convenience method that marshals the request body to JSON
func (e *StreamingExecutor) ExecuteWithJSONBody(
	ctx context.Context,
	url string,
	requestBody interface{},
	authToken string,
	authType string,
) (types.ChatCompletionStream, error) {
	return e.ExecuteStreamRequest(ctx, "POST", url, requestBody, authToken, authType)
}

// prepareRequest creates an HTTP request with JSON body
func (e *StreamingExecutor) prepareRequest(
	ctx context.Context,
	method, url string,
	requestBody interface{},
) (*http.Request, error) {
	var reqBody io.Reader

	if requestBody != nil {
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return req, nil
}

// setAuthHeaders sets authentication headers based on auth type
func (e *StreamingExecutor) setAuthHeaders(req *http.Request, authToken string, authType string) {
	switch authType {
	case "oauth", "bearer":
		req.Header.Set("Authorization", "Bearer "+authToken)
	case "api_key":
		// Different providers use different header names
		switch e.config.ProviderType {
		case types.ProviderTypeAnthropic:
			req.Header.Set("x-api-key", authToken)
		case types.ProviderTypeGemini:
			req.Header.Set("x-goog-api-key", authToken)
		default:
			req.Header.Set("Authorization", "Bearer "+authToken)
		}
	}
}

// setStandardHeaders sets standard headers common to all streaming requests
func (e *StreamingExecutor) setStandardHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Provider-specific standard headers
	switch e.config.ProviderType {
	case types.ProviderTypeAnthropic:
		req.Header.Set("anthropic-version", "2023-06-01")
	}
}

// setExtraHeaders sets any extra headers configured
func (e *StreamingExecutor) setExtraHeaders(req *http.Request) {
	if e.config.ExtraHeaders == nil {
		return
	}

	for key, value := range e.config.ExtraHeaders {
		req.Header.Set(key, value)
	}
}

// trackRateLimits tracks rate limit information from response headers
func (e *StreamingExecutor) trackRateLimits(resp *http.Response, requestBody interface{}) {
	if e.config.RateLimitHelper == nil {
		return
	}

	// Extract model name from request body if possible
	model := ""
	if m, ok := requestBody.(interface{ GetModel() string }); ok {
		model = m.GetModel()
	} else if m, ok := requestBody.(map[string]interface{}); ok {
		if modelName, ok := m["model"].(string); ok {
			model = modelName
		}
	}

	e.config.RateLimitHelper.ParseAndUpdateRateLimits(resp.Header, model)
}

// handleErrorResponse handles error responses
func (e *StreamingExecutor) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Classify error by status code
	errCode := types.ClassifyHTTPError(resp.StatusCode)

	return types.NewProviderError(
		e.config.ProviderType,
		errCode,
		string(body),
	).
		WithOperation("streaming_request").
		WithStatusCode(resp.StatusCode)
}

// wrapError wraps an error with provider context
func (e *StreamingExecutor) wrapError(message string, err error) error {
	return types.NewNetworkError(e.config.ProviderType, message).
		WithOperation("streaming_request").
		WithOriginalErr(err)
}

// GetBaseURL returns the base URL for the executor
func (e *StreamingExecutor) GetBaseURL() string {
	if e.config.BaseURL != "" {
		return e.config.BaseURL
	}
	return ""
}

// BuildURL builds a full URL from base URL and endpoint
func (e *StreamingExecutor) BuildURL(endpoint string) string {
	baseURL := e.GetBaseURL()
	if baseURL == "" {
		return endpoint
	}
	return baseURL + endpoint
}

// RequestBuilder helps build streaming requests with a fluent API
type RequestBuilder struct {
	config StreamingExecutorConfig
}

// NewRequestBuilder creates a new request builder
func NewRequestBuilder(providerType types.ProviderType, client *http.Client) *RequestBuilder {
	return &RequestBuilder{
		config: StreamingExecutorConfig{
			ProviderType: providerType,
			Client:       client,
		},
	}
}

// WithRateLimitHelper sets the rate limit helper
func (b *RequestBuilder) WithRateLimitHelper(helper *common.RateLimitHelper) *RequestBuilder {
	b.config.RateLimitHelper = helper
	return b
}

// WithStreamCreator sets the stream creator
func (b *RequestBuilder) WithStreamCreator(creator StreamCreator) *RequestBuilder {
	b.config.StreamCreator = creator
	return b
}

// WithBaseURL sets the base URL
func (b *RequestBuilder) WithBaseURL(baseURL string) *RequestBuilder {
	b.config.BaseURL = baseURL
	return b
}

// WithEndpoint sets the endpoint path
func (b *RequestBuilder) WithEndpoint(endpoint string) *RequestBuilder {
	b.config.Endpoint = endpoint
	return b
}

// WithExtraHeaders sets extra headers
func (b *RequestBuilder) WithExtraHeaders(headers map[string]string) *RequestBuilder {
	b.config.ExtraHeaders = headers
	return b
}

// WithRequestPreparer sets the request preparer
func (b *RequestBuilder) WithRequestPreparer(preparer RequestPreparer) *RequestBuilder {
	b.config.PrepareRequest = preparer
	return b
}

// WithResponseHandler sets the response handler
func (b *RequestBuilder) WithResponseHandler(handler ResponseHandler) *RequestBuilder {
	b.config.HandleResponse = handler
	return b
}

// Build creates the streaming executor
func (b *RequestBuilder) Build() *StreamingExecutor {
	return NewStreamingExecutor(b.config)
}
