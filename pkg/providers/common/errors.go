// Package common provides shared utilities and infrastructure for AI provider implementations.
// This includes standardized error handling, authentication helpers, configuration management,
// health checking, metrics collection, and other common functionality across providers.
package common

import (
	"fmt"
	"net/http"
	"strings"
)

// APIErrorType classifies API errors
type APIErrorType string

const (
	APIErrorTypeRateLimit      APIErrorType = "rate_limit"
	APIErrorTypeAuth           APIErrorType = "auth"
	APIErrorTypeNotFound       APIErrorType = "not_found"
	APIErrorTypeInvalidRequest APIErrorType = "invalid_request"
	APIErrorTypeServer         APIErrorType = "server_error"
	APIErrorTypeUnknown        APIErrorType = "unknown"
)

// APIError represents a standardized provider error
type APIError struct {
	StatusCode int
	Type       APIErrorType
	Message    string
	RawBody    string
	Retryable  bool
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("[%s] %s (status: %d)", e.Type, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("[%s] HTTP %d error", e.Type, e.StatusCode)
}

// IsRateLimit checks if the error is a rate limit error
func (e *APIError) IsRateLimit() bool {
	return e.Type == APIErrorTypeRateLimit
}

// IsRetryable checks if the error is retryable
func (e *APIError) IsRetryable() bool {
	return e.Retryable
}

// ClassifyHTTPError creates an APIError from HTTP status code
// This provides a basic classification based on standard HTTP status codes
func ClassifyHTTPError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{
		StatusCode: statusCode,
		RawBody:    string(body),
	}

	// Classify based on status code
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		apiErr.Type = APIErrorTypeAuth
		apiErr.Message = "authentication failed"
		apiErr.Retryable = false

	case statusCode == http.StatusNotFound:
		apiErr.Type = APIErrorTypeNotFound
		apiErr.Message = "resource not found"
		apiErr.Retryable = false

	case statusCode == http.StatusBadRequest:
		apiErr.Type = APIErrorTypeInvalidRequest
		apiErr.Message = "invalid request"
		apiErr.Retryable = false

	case statusCode == http.StatusTooManyRequests:
		apiErr.Type = APIErrorTypeRateLimit
		apiErr.Message = "rate limit exceeded"
		apiErr.Retryable = true

	case statusCode >= 500 && statusCode < 600:
		apiErr.Type = APIErrorTypeServer
		apiErr.Message = "server error"
		apiErr.Retryable = true

	default:
		apiErr.Type = APIErrorTypeUnknown
		apiErr.Message = fmt.Sprintf("unknown error with status %d", statusCode)
		apiErr.Retryable = false
	}

	// Try to extract more details from body if available
	if len(body) > 0 && len(body) < 1000 {
		bodyStr := strings.TrimSpace(string(body))
		if bodyStr != "" && bodyStr != "{}" {
			// Add body content to message if it's reasonably short
			apiErr.Message = fmt.Sprintf("%s: %s", apiErr.Message, bodyStr)
		}
	}

	return apiErr
}

// ErrorClassifier interface for provider-specific error parsing
// Providers can implement this to provide more detailed error classification
// based on their specific API error response formats
type ErrorClassifier interface {
	Classify(statusCode int, body []byte) *APIError
}
