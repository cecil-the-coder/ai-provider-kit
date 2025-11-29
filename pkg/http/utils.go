package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// NewJSONRequest creates a JSON HTTP request with proper headers
func NewJSONRequest(method, url string, body interface{}) (*http.Request, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// APIError represents a standardized API error with context
type APIError struct {
	StatusCode int
	Message    string
	Type       string
	Code       string
	RawBody    string
	Timestamp  time.Time
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// RequestBuilder helps build HTTP requests with common patterns
type RequestBuilder struct {
	method  string
	url     string
	headers map[string]string
	body    interface{}
	ctx     context.Context
}

// NewRequestBuilder creates a new request builder
func NewRequestBuilder(method, url string) *RequestBuilder {
	return &RequestBuilder{
		method:  method,
		url:     url,
		headers: make(map[string]string),
		ctx:     context.Background(),
	}
}

// WithContext sets the request context
func (rb *RequestBuilder) WithContext(ctx context.Context) *RequestBuilder {
	rb.ctx = ctx
	return rb
}

// WithHeaders adds headers to the request
func (rb *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	for k, v := range headers {
		rb.headers[k] = v
	}
	return rb
}

// WithHeader adds a single header
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.headers[key] = value
	return rb
}

// WithJSONBody sets a JSON body
func (rb *RequestBuilder) WithJSONBody(body interface{}) *RequestBuilder {
	rb.body = body
	rb.headers["Content-Type"] = "application/json"
	return rb
}

// Build creates the HTTP request
func (rb *RequestBuilder) Build() (*http.Request, error) {
	var bodyReader io.Reader

	if rb.body != nil {
		jsonBody, err := json.Marshal(rb.body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(rb.ctx, rb.method, rb.url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range rb.headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// CommonHTTPHeaders returns commonly used HTTP headers for AI providers
func CommonHTTPHeaders() map[string]string {
	return map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
		"User-Agent":   "ai-provider-kit/1.0",
	}
}

// AuthHeaders creates authentication headers for different methods
func AuthHeaders(method, token string) map[string]string {
	switch method {
	case "bearer":
		return map[string]string{
			"Authorization": "Bearer " + token,
		}
	case "api-key":
		return map[string]string{
			"x-api-key": token,
		}
	case "openai":
		return map[string]string{
			"Authorization": "Bearer " + token,
		}
	case "anthropic":
		return map[string]string{
			"x-api-key":         token,
			"anthropic-version": "2023-06-01",
		}
	default:
		return map[string]string{
			"Authorization": token,
		}
	}
}

// DefaultConfigProvider provides configuration for common AI providers
type DefaultConfigProvider struct{}

// GetHTTPConfig returns HTTP configuration for different provider types
func (d *DefaultConfigProvider) GetHTTPConfig(providerType types.ProviderType) HTTPClientConfig {
	baseConfig := HTTPClientConfig{
		Timeout:           60 * time.Second,
		MaxRetries:        3,
		BaseRetryDelay:    time.Second,
		MaxRetryDelay:     60 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors:   []string{"429", "500", "502", "503", "504"},
		Headers:           CommonHTTPHeaders(),
		EnableMetrics:     true,
	}

	// Provider-specific adjustments
	switch providerType {
	case types.ProviderTypeAnthropic:
		baseConfig.Headers["anthropic-version"] = "2023-06-01"
	case types.ProviderTypeOpenAI:
		baseConfig.Timeout = 120 * time.Second // Longer timeout for OpenAI
	case types.ProviderTypeCerebras:
		baseConfig.Timeout = 120 * time.Second
	}

	return baseConfig
}

// DefaultClient creates an HTTP client with sensible defaults for AI providers
func DefaultClient(providerType types.ProviderType) *HTTPClient {
	provider := &DefaultConfigProvider{}
	config := provider.GetHTTPConfig(providerType)
	return NewHTTPClient(config)
}
