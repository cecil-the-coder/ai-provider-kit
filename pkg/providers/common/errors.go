// Package common provides shared utilities and infrastructure for AI provider implementations.
// This includes standardized error handling, authentication helpers, configuration management,
// health checking, metrics collection, and other common functionality across providers.
package common

import (
	"fmt"
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

// ErrorClassifier interface for provider-specific error parsing
// Providers can implement this to provide more detailed error classification
// based on their specific API error response formats
type ErrorClassifier interface {
	Classify(statusCode int, body []byte) *APIError
}
