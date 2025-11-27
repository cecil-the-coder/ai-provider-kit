package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// IsRetryableError checks if an error should trigger a retry
func IsRetryableError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
		return false
	}

	// Check for network-related errors
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "temporary")
}

// ProcessResponse processes an HTTP response and handles errors
func ProcessResponse(resp *http.Response) ([]byte, error) {
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ParseAPIError(resp.StatusCode, string(body))
	}

	return body, nil
}

// ProcessJSONResponse processes an HTTP response and unmarshals JSON
func ProcessJSONResponse(resp *http.Response, target interface{}) error {
	body, err := ProcessResponse(resp)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return nil
}

// ParseAPIError creates a standardized API error from response
func ParseAPIError(statusCode int, body string) *APIError {
	apiErr := &APIError{
		StatusCode: statusCode,
		RawBody:    body,
		Timestamp:  time.Now(),
	}

	// Try to parse structured error response
	var errorResp ErrorResponse
	if err := json.Unmarshal([]byte(body), &errorResp); err == nil {
		apiErr.Message = errorResp.Error.Message
		apiErr.Type = errorResp.Error.Type
		apiErr.Code = errorResp.Error.Code
	} else {
		// Fallback to simple error message
		apiErr.Message = strings.TrimSpace(body)
		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(statusCode)
		}
	}

	return apiErr
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

// StreamingResponse handles streaming responses
type StreamingResponse struct {
	resp *http.Response
}

// NewStreamingResponse creates a new streaming response handler
func NewStreamingResponse(resp *http.Response) *StreamingResponse {
	return &StreamingResponse{resp: resp}
}

// ReadLine reads a single line from the streaming response
func (sr *StreamingResponse) ReadLine() ([]byte, error) {
	return readLine(sr.resp.Body)
}

// ReadChunk reads data until a delimiter
func (sr *StreamingResponse) ReadChunk(delimiter string) ([]byte, error) {
	var buffer bytes.Buffer
	chunk := make([]byte, 1024)

	for {
		n, err := sr.resp.Body.Read(chunk)
		if n > 0 {
			buffer.Write(chunk[:n])
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Check if delimiter is found
		if bytes.Contains(buffer.Bytes(), []byte(delimiter)) {
			break
		}
	}

	return buffer.Bytes(), nil
}

// Close closes the streaming response
func (sr *StreamingResponse) Close() error {
	if sr.resp != nil && sr.resp.Body != nil {
		return sr.resp.Body.Close()
	}
	return nil
}

// readLine reads a single line from a reader
func readLine(r io.Reader) ([]byte, error) {
	var buf [1]byte
	var line []byte

	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			ch := buf[0]
			line = append(line, ch)
			if ch == '\n' {
				break
			}
		}
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				break
			}
			return line, err
		}
	}

	return line, nil
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
