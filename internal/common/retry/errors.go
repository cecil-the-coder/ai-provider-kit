// Package retry provides retry infrastructure for AI provider implementations.
// This includes retry policies, strategies, backoff algorithms, and error classification
// to handle transient failures gracefully across all providers.
package retry

import (
	"errors"
	"net/http"
)

// RetryableError is an interface for errors that can indicate retry possibility
type RetryableError interface {
	error
	IsRetryable() bool
}

// retryableError is a concrete implementation of RetryableError
type retryableError struct {
	err        error
	retryable  bool
	statusCode int
}

// Error implements the error interface
func (e *retryableError) Error() string {
	return e.err.Error()
}

// IsRetryable returns whether the error is retryable
func (e *retryableError) IsRetryable() bool {
	return e.retryable
}

// StatusCode returns the HTTP status code associated with the error
func (e *retryableError) StatusCode() int {
	return e.statusCode
}

// Unwrap returns the underlying error
func (e *retryableError) Unwrap() error {
	return e.err
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error, retryable bool, statusCode int) error {
	if err == nil {
		return nil
	}
	return &retryableError{
		err:        err,
		retryable:  retryable,
		statusCode: statusCode,
	}
}

// Common retryable HTTP status codes
const (
	StatusTooManyRequests     = http.StatusTooManyRequests     // 429
	StatusInternalServerError = http.StatusInternalServerError // 500
	StatusBadGateway          = http.StatusBadGateway          // 502
	StatusServiceUnavailable  = http.StatusServiceUnavailable  // 503
	StatusGatewayTimeout      = http.StatusGatewayTimeout      // 504
	StatusInsufficientStorage = http.StatusInsufficientStorage // 507
	StatusNetworkAuthRequired = 511                            // 511
)

// retryableStatusCodes contains HTTP status codes that should trigger retries
var retryableStatusCodes = map[int]bool{
	StatusTooManyRequests:     true,
	StatusInternalServerError: true,
	StatusBadGateway:          true,
	StatusServiceUnavailable:  true,
	StatusGatewayTimeout:      true,
	StatusInsufficientStorage: true,
	StatusNetworkAuthRequired: true,
}

// IsRetryableStatusCode checks if an HTTP status code is retryable
func IsRetryableStatusCode(statusCode int) bool {
	return retryableStatusCodes[statusCode]
}

// IsRetryableError checks if an error is retryable
// It examines the error chain for RetryableError implementations
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if error implements RetryableError interface
	var retryErr RetryableError
	if errors.As(err, &retryErr) {
		return retryErr.IsRetryable()
	}

	// If not, assume not retryable
	return false
}

// GetStatusCode extracts the HTTP status code from an error if available
func GetStatusCode(err error) int {
	if err == nil {
		return 0
	}

	// Check for retryableError with status code
	var retryErr *retryableError
	if errors.As(err, &retryErr) {
		return retryErr.StatusCode()
	}

	return 0
}

// MarkRetryable wraps an error to mark it as retryable
func MarkRetryable(err error, statusCode int) error {
	if err == nil {
		return nil
	}
	return NewRetryableError(err, true, statusCode)
}

// MarkNonRetryable wraps an error to mark it as non-retryable
func MarkNonRetryable(err error, statusCode int) error {
	if err == nil {
		return nil
	}
	return NewRetryableError(err, false, statusCode)
}
