// Package sentinel provides sentinel errors for common AI provider scenarios.
//
// These sentinel errors enable standardized error checking using errors.Is(),
// making it easier to handle specific error scenarios consistently across
// different providers and applications.
//
// # Usage
//
// Check for specific error scenarios using errors.Is():
//
//	if errors.Is(err, sentinel.ErrNotAuthenticated) {
//	    // Handle authentication failure
//	    return fmt.Errorf("please provide valid credentials")
//	}
//
//	if errors.Is(err, sentinel.ErrRateLimited) {
//	    // Handle rate limiting with backoff
//	    time.Sleep(time.Second * 5)
//	    return retry()
//	}
//
//	if errors.Is(err, sentinel.ErrModelNotFound) {
//	    // Handle unknown model
//	    return fmt.Errorf("model %s is not available", modelName)
//	}
//
// # Returning Sentinel Errors
//
// Providers should return sentinel errors using fmt.Errorf with %w:
//
//	if resp.StatusCode == 401 {
//	    return fmt.Errorf("authentication failed: %w", sentinel.ErrNotAuthenticated)
//	}
//
//	if resp.StatusCode == 429 {
//	    return fmt.Errorf("rate limit exceeded: %w", sentinel.ErrRateLimited)
//	}
//
// # Sentinel Error Categories
//
// Authentication Errors:
//   - ErrNotAuthenticated - Missing or invalid credentials
//   - ErrUnauthorized - Insufficient permissions for the requested operation
//
// Rate Limiting:
//   - ErrRateLimited - Rate limit has been exceeded
//
// Request Errors:
//   - ErrInvalidRequest - Malformed or invalid request
//   - ErrModelNotFound - Requested model does not exist
//
// Network/Timeout Errors:
//   - ErrTimeout - Request timed out
//   - ErrCancelled - Request was cancelled
//   - ErrNetworkError - Network connectivity issue
//
// Server Errors:
//   - ErrServerError - Internal server error from provider
//   - ErrServiceUnavailable - Provider service is temporarily unavailable
package sentinel

import "errors"

// Authentication Errors

// ErrNotAuthenticated indicates that the request lacks valid authentication
// credentials. This is returned when an API key, token, or other credential
// is missing, invalid, or expired.
var ErrNotAuthenticated = errors.New("not authenticated: missing or invalid credentials")

// ErrUnauthorized indicates that the client is authenticated but lacks
// sufficient permissions for the requested operation. This is distinct from
// ErrNotAuthenticated - the credentials are valid, but don't grant access
// to the specific resource or operation.
var ErrUnauthorized = errors.New("unauthorized: insufficient permissions for the requested operation")

// Rate Limiting Errors

// ErrRateLimited indicates that the client has exceeded the rate limit
// for API requests. This error typically includes information about when
// the rate limit will reset, allowing clients to implement proper backoff.
var ErrRateLimited = errors.New("rate limited: too many requests")

// Request Errors

// ErrInvalidRequest indicates that the request was malformed or invalid.
// This can include missing required parameters, invalid parameter values,
// or other client-side validation errors.
var ErrInvalidRequest = errors.New("invalid request: malformed or invalid parameters")

// ErrModelNotFound indicates that the requested model is not available
// or does not exist. This can be used when a provider doesn't support
// a specific model or when a model name is misspelled.
var ErrModelNotFound = errors.New("model not found: the requested model is not available")

// ErrContextLengthExceeded indicates that the input context exceeds
// the model's maximum context length. This is a specific type of
// invalid request error that can be handled differently.
var ErrContextLengthExceeded = errors.New("context length exceeded: input is too large for the model")

// ErrContentFiltered indicates that content was rejected by the provider's
// content filtering policy. This can happen when input or output violates
// safety guidelines.
var ErrContentFiltered = errors.New("content filtered: request rejected by content policy")

// Network/Timeout Errors

// ErrTimeout indicates that the request timed out. This can be due to
// network issues, slow provider responses, or context deadline exceeded.
var ErrTimeout = errors.New("timeout: request timed out")

// ErrCancelled indicates that the request was cancelled by the client.
// This typically happens when the context is cancelled.
var ErrCancelled = errors.New("cancelled: request was cancelled")

// ErrNetworkError indicates a general network connectivity issue.
// This can include DNS resolution failures, connection refused, etc.
var ErrNetworkError = errors.New("network error: unable to reach the provider")

// Server Errors

// ErrServerError indicates an internal server error from the provider.
// This is typically a 5xx error indicating an unexpected server condition.
var ErrServerError = errors.New("server error: an internal error occurred on the server")

// ErrServiceUnavailable indicates that the provider service is temporarily
// unavailable. This is typically a 503 error and may be retried after a delay.
var ErrServiceUnavailable = errors.New("service unavailable: the service is temporarily unavailable")

// Helper functions for common error scenarios

// IsAuthenticationError returns true if the error is an authentication-related error.
func IsAuthenticationError(err error) bool {
	return errors.Is(err, ErrNotAuthenticated) || errors.Is(err, ErrUnauthorized)
}

// IsRetryableError returns true if the error is potentially retryable.
// This includes rate limits, timeouts, network errors, and server errors.
func IsRetryableError(err error) bool {
	return errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrNetworkError) ||
		errors.Is(err, ErrServerError) ||
		errors.Is(err, ErrServiceUnavailable)
}

// IsClientError returns true if the error is a client-side error that
// won't be fixed by retrying without changes to the request.
func IsClientError(err error) bool {
	return errors.Is(err, ErrNotAuthenticated) ||
		errors.Is(err, ErrUnauthorized) ||
		errors.Is(err, ErrInvalidRequest) ||
		errors.Is(err, ErrModelNotFound) ||
		errors.Is(err, ErrContextLengthExceeded) ||
		errors.Is(err, ErrContentFiltered)
}
