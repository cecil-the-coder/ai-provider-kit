package types

import (
	"fmt"
	"net/http"
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
	Code        ErrorCode    // Categorized error code
	Message     string       // Human-readable message
	StatusCode  int          // HTTP status code (0 if not applicable)
	Provider    ProviderType // Which provider generated this error
	Operation   string       // What operation failed (e.g., "chat_completion", "list_models")
	OriginalErr error        // Wrapped original error
	RetryAfter  int          // Seconds to wait before retry (for rate limits)
	RequestID   string       // Provider's request ID if available
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
