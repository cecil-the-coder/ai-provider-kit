package types

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"time"
)

// ErrorCode categorizes provider errors
type ErrorCode string

const (
	ErrCodeUnknown        ErrorCode = "unknown"
	ErrCodeAuthentication ErrorCode = "authentication"
	ErrCodeRateLimit      ErrorCode = "rate_limit"
	ErrCodeInvalidRequest ErrorCode = "invalid_request"
	ErrCodeNotFound       ErrorCode = "not_found"
	ErrCodeServerError    ErrorCode = "server_error"
	ErrCodeTimeout        ErrorCode = "timeout"
	ErrCodeNetwork        ErrorCode = "network"
	ErrCodeContextLength  ErrorCode = "context_length"
	ErrCodeContentFilter  ErrorCode = "content_filter"

	// Aliases for TestResult status compatibility.
	// These convenience constants make it easier to map between TestStatus values
	// and ErrorCode constants when implementing health checks and provider testing.
	// Example: if testResult.Status == TestStatusAuthFailed { return NewProviderError(..., ErrCodeAuthFailed, ...) }
	ErrCodeAuthFailed         = ErrCodeAuthentication // Maps to TestStatusAuthFailed
	ErrCodeConnectivityFailed = ErrCodeNetwork        // Maps to TestStatusConnectivityFailed
	ErrCodeTimeoutFailed      = ErrCodeTimeout        // Maps to TestStatusTimeoutFailed
)

// ProviderError represents a standardized error from a provider
type ProviderError struct {
	Code           ErrorCode     // Categorized error code
	Message        string        // Human-readable message
	StatusCode     int           // HTTP status code (0 if not applicable)
	Provider       ProviderType  // Which provider generated this error
	Operation      string        // What operation failed (e.g., "chat_completion", "list_models")
	OriginalErr    error         // Wrapped original error
	RetryAfter     int           // Seconds to wait before retry (for rate limits)
	RequestID      string        // Provider's request ID if available
	RequestDump    string        // HTTP request dump (credentials masked)
	ResponseDump   string        // HTTP response dump
	RequestLatency time.Duration // Time taken for the request
	CorrelationID  string        // Unique ID to correlate logs and errors
}

// Error implements the error interface
func (e *ProviderError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("[%s] %s (status=%d, code=%s)", e.Provider, e.Message, e.StatusCode, e.Code)
	}
	return fmt.Sprintf("[%s] %s (code=%s)", e.Provider, e.Message, e.Code)
}

// Unwrap returns the original error for errors.Is/As
func (e *ProviderError) Unwrap() error {
	return e.OriginalErr
}

// IsRetryable returns true if the error is potentially recoverable with retry
func (e *ProviderError) IsRetryable() bool {
	switch e.Code {
	case ErrCodeRateLimit, ErrCodeServerError, ErrCodeTimeout, ErrCodeNetwork:
		return true
	}
	return false
}

// WithOperation sets the operation field and returns the error for chaining
func (e *ProviderError) WithOperation(operation string) *ProviderError {
	e.Operation = operation
	return e
}

// WithMessage sets the message field and returns the error for chaining
func (e *ProviderError) WithMessage(message string) *ProviderError {
	e.Message = message
	return e
}

// WithStatusCode sets the status code field and returns the error for chaining
func (e *ProviderError) WithStatusCode(statusCode int) *ProviderError {
	e.StatusCode = statusCode
	return e
}

// WithOriginalErr sets the original error field and returns the error for chaining
func (e *ProviderError) WithOriginalErr(err error) *ProviderError {
	e.OriginalErr = err
	return e
}

// WithRequestID sets the request ID field and returns the error for chaining
func (e *ProviderError) WithRequestID(requestID string) *ProviderError {
	e.RequestID = requestID
	return e
}

// WithRetryAfter sets the retry after field and returns the error for chaining
func (e *ProviderError) WithRetryAfter(retryAfter int) *ProviderError {
	e.RetryAfter = retryAfter
	return e
}

// WithRequestDump sets the request dump field and returns the error for chaining
func (e *ProviderError) WithRequestDump(dump string) *ProviderError {
	e.RequestDump = dump
	return e
}

// WithResponseDump sets the response dump field and returns the error for chaining
func (e *ProviderError) WithResponseDump(dump string) *ProviderError {
	e.ResponseDump = dump
	return e
}

// WithRequestLatency sets the request latency field and returns the error for chaining
func (e *ProviderError) WithRequestLatency(latency time.Duration) *ProviderError {
	e.RequestLatency = latency
	return e
}

// WithCorrelationID sets the correlation ID field and returns the error for chaining
func (e *ProviderError) WithCorrelationID(correlationID string) *ProviderError {
	e.CorrelationID = correlationID
	return e
}

// NewProviderError creates a new ProviderError
func NewProviderError(provider ProviderType, code ErrorCode, message string) *ProviderError {
	return &ProviderError{
		Code:     code,
		Message:  message,
		Provider: provider,
	}
}

// NewAuthError creates a new authentication error
func NewAuthError(provider ProviderType, message string) *ProviderError {
	return &ProviderError{
		Code:     ErrCodeAuthentication,
		Message:  message,
		Provider: provider,
	}
}

// NewRateLimitError creates a new rate limit error
func NewRateLimitError(provider ProviderType, retryAfter int) *ProviderError {
	return &ProviderError{
		Code:       ErrCodeRateLimit,
		Message:    "rate limit exceeded",
		Provider:   provider,
		RetryAfter: retryAfter,
	}
}

// NewServerError creates a new server error
func NewServerError(provider ProviderType, statusCode int, message string) *ProviderError {
	return &ProviderError{
		Code:       ErrCodeServerError,
		Message:    message,
		Provider:   provider,
		StatusCode: statusCode,
	}
}

// NewInvalidRequestError creates a new invalid request error
func NewInvalidRequestError(provider ProviderType, message string) *ProviderError {
	return &ProviderError{
		Code:     ErrCodeInvalidRequest,
		Message:  message,
		Provider: provider,
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(provider ProviderType, message string) *ProviderError {
	return &ProviderError{
		Code:     ErrCodeNetwork,
		Message:  message,
		Provider: provider,
	}
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(provider ProviderType, message string) *ProviderError {
	return &ProviderError{
		Code:     ErrCodeTimeout,
		Message:  message,
		Provider: provider,
	}
}

// NewContextLengthError creates a new context length error
func NewContextLengthError(provider ProviderType, message string) *ProviderError {
	return &ProviderError{
		Code:     ErrCodeContextLength,
		Message:  message,
		Provider: provider,
	}
}

// NewContentFilterError creates a new content filter error
func NewContentFilterError(provider ProviderType, message string) *ProviderError {
	return &ProviderError{
		Code:     ErrCodeContentFilter,
		Message:  message,
		Provider: provider,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(provider ProviderType, message string) *ProviderError {
	return &ProviderError{
		Code:     ErrCodeNotFound,
		Message:  message,
		Provider: provider,
	}
}

// ClassifyHTTPError determines error code from HTTP status
func ClassifyHTTPError(statusCode int) ErrorCode {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrCodeAuthentication
	case http.StatusTooManyRequests:
		return ErrCodeRateLimit
	case http.StatusBadRequest:
		return ErrCodeInvalidRequest
	case http.StatusNotFound:
		return ErrCodeNotFound
	default:
		if statusCode >= 500 {
			return ErrCodeServerError
		}
		return ErrCodeUnknown
	}
}

// DumpRequest dumps the HTTP request with credentials masked
// Returns the error itself for chaining
func (e *ProviderError) DumpRequest(req *http.Request) *ProviderError {
	if req == nil {
		return e
	}

	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		e.RequestDump = fmt.Sprintf("failed to dump request: %v", err)
		return e
	}

	e.RequestDump = maskCredentials(string(dump))
	return e
}

// DumpResponse dumps the HTTP response
// Returns the error itself for chaining
func (e *ProviderError) DumpResponse(resp *http.Response) *ProviderError {
	if resp == nil {
		return e
	}

	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		e.ResponseDump = fmt.Sprintf("failed to dump response: %v", err)
		return e
	}

	e.ResponseDump = string(dump)
	return e
}

// maskCredentials masks sensitive information in HTTP dumps
func maskCredentials(dump string) string {
	// Patterns to mask sensitive information
	patterns := []struct {
		regex       *regexp.Regexp
		replacement string
	}{
		// Authorization header (Bearer tokens, API keys, etc.)
		{
			regex:       regexp.MustCompile(`(?i)(Authorization:\s*)(Bearer\s+)?([^\s\r\n]+)`),
			replacement: "${1}${2}***MASKED***",
		},
		// API key headers (various formats)
		{
			regex:       regexp.MustCompile(`(?i)((?:X-)?Api-Key:\s*)([^\s\r\n]+)`),
			replacement: "${1}***MASKED***",
		},
		{
			regex:       regexp.MustCompile(`(?i)(X-API-KEY:\s*)([^\s\r\n]+)`),
			replacement: "${1}***MASKED***",
		},
		// Anthropic specific
		{
			regex:       regexp.MustCompile(`(?i)(X-Api-Key:\s*)([^\s\r\n]+)`),
			replacement: "${1}***MASKED***",
		},
		// OpenAI specific
		{
			regex:       regexp.MustCompile(`(?i)(OpenAI-Organization:\s*)([^\s\r\n]+)`),
			replacement: "${1}***MASKED***",
		},
		// Generic token/secret headers
		{
			regex:       regexp.MustCompile(`(?i)((?:X-)?(?:Auth|Access|Secret)-Token:\s*)([^\s\r\n]+)`),
			replacement: "${1}***MASKED***",
		},
		// JSON body with api_key or apiKey fields
		{
			regex:       regexp.MustCompile(`(?i)("(?:api_?key|auth_?token|secret)":\s*")([^"]+)(")`),
			replacement: "${1}***MASKED***${3}",
		},
		// Password fields in JSON
		{
			regex:       regexp.MustCompile(`(?i)("password":\s*")([^"]+)(")`),
			replacement: "${1}***MASKED***${3}",
		},
	}

	result := dump
	for _, p := range patterns {
		result = p.regex.ReplaceAllString(result, p.replacement)
	}

	return result
}

// GetDebugInfo returns a formatted string with all debug information
func (e *ProviderError) GetDebugInfo() string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("Error: %s\n", e.Error()))
	buf.WriteString(fmt.Sprintf("Provider: %s\n", e.Provider))
	buf.WriteString(fmt.Sprintf("Code: %s\n", e.Code))
	buf.WriteString(fmt.Sprintf("Operation: %s\n", e.Operation))

	if e.CorrelationID != "" {
		buf.WriteString(fmt.Sprintf("CorrelationID: %s\n", e.CorrelationID))
	}
	if e.RequestID != "" {
		buf.WriteString(fmt.Sprintf("RequestID: %s\n", e.RequestID))
	}
	if e.RequestLatency > 0 {
		buf.WriteString(fmt.Sprintf("RequestLatency: %s\n", e.RequestLatency))
	}
	if e.RetryAfter > 0 {
		buf.WriteString(fmt.Sprintf("RetryAfter: %ds\n", e.RetryAfter))
	}

	if e.RequestDump != "" {
		buf.WriteString("\n--- Request Dump ---\n")
		buf.WriteString(e.RequestDump)
		buf.WriteString("\n")
	}

	if e.ResponseDump != "" {
		buf.WriteString("\n--- Response Dump ---\n")
		buf.WriteString(e.ResponseDump)
		buf.WriteString("\n")
	}

	if e.OriginalErr != nil {
		buf.WriteString(fmt.Sprintf("\nOriginal Error: %v\n", e.OriginalErr))
	}

	return strings.TrimSpace(buf.String())
}
