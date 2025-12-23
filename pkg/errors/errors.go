// Package errors provides rich error handling with context for AI provider operations.
//
// This package re-exports the public API from internal/common/errors, providing
// external consumers with rich error context, credential masking, and debugging
// capabilities for AI provider operations.
//
// # Rich Errors
//
// RichError wraps errors with comprehensive debugging context:
//
//	err := doAPICall()
//	if err != nil {
//	    richErr := errors.Wrap(err).
//	        WithRequestID(requestID).
//	        WithProvider(types.ProviderTypeOpenAI).
//	        WithModel("gpt-4").
//	        WithRequestSnapshot(req).
//	        WithResponseSnapshot(resp).
//	        WithTimingStart(startTime)
//
//	    log.Error(richErr.Format())
//	    return richErr
//	}
//
// # Error Context
//
// ErrorContext captures structured information about when and where an error occurred:
//
//	ctx := errors.NewErrorContext().
//	    WithRequestID("req-123").
//	    WithCorrelationID("corr-456").
//	    WithProvider(types.ProviderTypeAnthropic).
//	    WithModel("claude-3-opus").
//	    WithOperation("chat_completion").
//	    WithDuration(150 * time.Millisecond)
//
// # Credential Masking
//
// The package provides automatic masking of sensitive information in logs:
//
//	masker := errors.DefaultCredentialMasker()
//	safe := masker.MaskString(`{"api_key": "secret123"}`)
//	// Result: {"api_key": "***MASKED***"}
package errors

import (
	"context"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/errors"
)

// RichError is an enhanced error type with context for debugging and tracing.
// It wraps an underlying error with structured context including request/response
// snapshots, timing information, correlation IDs, and provider details.
type RichError = errors.RichError

// ErrorContext contains contextual information about an error.
// This includes request/response snapshots, timing information, provider details,
// and correlation identifiers for tracing across services.
type ErrorContext = errors.ErrorContext

// RequestSnapshot captures key information from an HTTP request for debugging.
// Sensitive information (API keys, tokens, etc.) is automatically masked.
type RequestSnapshot = errors.RequestSnapshot

// ResponseSnapshot captures key information from an HTTP response for debugging.
// Sensitive information (API keys, tokens, etc.) is automatically masked.
type ResponseSnapshot = errors.ResponseSnapshot

// SnapshotConfig controls how request/response snapshots are created.
type SnapshotConfig = errors.SnapshotConfig

// CredentialMasker provides methods to mask sensitive information in logs and errors.
type CredentialMasker = errors.CredentialMasker

// DefaultMasker is the default implementation of CredentialMasker.
type DefaultMasker = errors.DefaultMasker

// NewRichError creates a new RichError wrapping the given error.
// The returned error has a default ErrorContext and SnapshotConfig attached.
func NewRichError(err error) *errors.RichError {
	return errors.NewRichError(err)
}

// NewRichErrorWithConfig creates a new RichError with a custom snapshot configuration.
// Use this when you need to control snapshot behavior (body size limits, headers, etc.).
func NewRichErrorWithConfig(err error, config *errors.SnapshotConfig) *errors.RichError {
	return errors.NewRichErrorWithConfig(err, config)
}

// Wrap wraps an error with rich context.
// This is a convenience function for NewRichError(err).
// Returns nil if err is nil.
func Wrap(err error) *errors.RichError {
	return errors.Wrap(err)
}

// WrapWithContext wraps an error with an existing error context.
// Use this when you have already built an ErrorContext and want to attach it
// to an error. Returns nil if err is nil.
func WrapWithContext(err error, ctx *errors.ErrorContext) *errors.RichError {
	return errors.WrapWithContext(err, ctx)
}

// GetErrorContext retrieves the error context from a context.Context.
// Returns nil if no error context is stored in the context.
// This is typically used with middleware that captures error context.
func GetErrorContext(ctx context.Context) *errors.ErrorContext {
	return errors.GetErrorContext(ctx)
}

// NewErrorContext creates a new error context with the timestamp set to now.
func NewErrorContext() *errors.ErrorContext {
	return errors.NewErrorContext()
}

// DefaultSnapshotConfig returns the default snapshot configuration.
// The default configuration captures up to 4KB of body content with headers
// and uses the default credential masker.
func DefaultSnapshotConfig() *errors.SnapshotConfig {
	return errors.DefaultSnapshotConfig()
}

// NewRequestSnapshot creates a snapshot of an HTTP request.
// The request body will be read and restored so the request can still be used.
// Returns nil if req is nil.
func NewRequestSnapshot(req *http.Request, config *errors.SnapshotConfig) *errors.RequestSnapshot {
	return errors.NewRequestSnapshot(req, config)
}

// NewResponseSnapshot creates a snapshot of an HTTP response.
// The response body will be read and restored so the response can still be used.
// Returns nil if resp is nil.
func NewResponseSnapshot(resp *http.Response, config *errors.SnapshotConfig) *errors.ResponseSnapshot {
	return errors.NewResponseSnapshot(resp, config)
}

// DefaultCredentialMasker creates a new credential masker with default patterns.
// The default masker handles common sensitive data patterns including:
//   - Bearer tokens
//   - API keys
//   - Authorization headers
//   - Passwords and secrets
//   - AWS access keys
func DefaultCredentialMasker() *errors.DefaultMasker {
	return errors.DefaultCredentialMasker()
}

// NewCredentialMasker creates a new credential masker with no default patterns.
// Use this if you want complete control over what gets masked.
func NewCredentialMasker() *errors.DefaultMasker {
	return errors.NewCredentialMasker()
}

// MaskURL masks sensitive information in URLs (query parameters, etc.).
func MaskURL(url string) string {
	return errors.MaskURL(url)
}
