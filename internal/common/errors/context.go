// Package errors provides rich error context for AI provider operations.
// It includes request/response snapshots, timing information, correlation IDs,
// and credential masking for secure debugging and tracing.
package errors

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ErrorContext contains contextual information about an error
type ErrorContext struct {
	// RequestID is a unique identifier for this request
	RequestID string

	// CorrelationID is a correlation identifier for tracing across services
	CorrelationID string

	// Timestamp is when the error occurred
	Timestamp time.Time

	// Duration is how long the request took before failing
	Duration time.Duration

	// Provider is the AI provider name
	Provider types.ProviderType

	// Model is the model being used
	Model string

	// Operation is the operation that failed (e.g., "chat_completion", "list_models")
	Operation string

	// Request is a snapshot of the HTTP request
	Request *RequestSnapshot

	// Response is a snapshot of the HTTP response (if available)
	Response *ResponseSnapshot
}

// RequestSnapshot captures key information from an HTTP request for debugging
type RequestSnapshot struct {
	// Method is the HTTP method (GET, POST, etc.)
	Method string

	// URL is the request URL
	URL string

	// Headers are the HTTP headers (with sensitive values masked)
	Headers map[string][]string

	// Body is the request body (truncated if too large)
	Body string

	// BodyTruncated indicates if the body was truncated
	BodyTruncated bool
}

// ResponseSnapshot captures key information from an HTTP response for debugging
type ResponseSnapshot struct {
	// StatusCode is the HTTP status code
	StatusCode int

	// Headers are the HTTP headers (with sensitive values masked)
	Headers map[string][]string

	// Body is the response body (truncated if too large)
	Body string

	// BodyTruncated indicates if the body was truncated
	BodyTruncated bool
}

// SnapshotConfig controls how snapshots are created
type SnapshotConfig struct {
	// MaxBodySize is the maximum body size to capture (default: 4KB)
	MaxBodySize int

	// IncludeHeaders determines if headers should be included
	IncludeHeaders bool

	// IncludeBody determines if request/response bodies should be included
	IncludeBody bool

	// Masker is the credential masker to use
	Masker CredentialMasker
}

// DefaultSnapshotConfig returns the default snapshot configuration
func DefaultSnapshotConfig() *SnapshotConfig {
	return &SnapshotConfig{
		MaxBodySize:    4096, // 4KB
		IncludeHeaders: true,
		IncludeBody:    true,
		Masker:         DefaultCredentialMasker(),
	}
}

// NewRequestSnapshot creates a snapshot of an HTTP request
// The request body will be read and restored so the request can still be used
func NewRequestSnapshot(req *http.Request, config *SnapshotConfig) *RequestSnapshot {
	if req == nil {
		return nil
	}

	if config == nil {
		config = DefaultSnapshotConfig()
	}

	snapshot := &RequestSnapshot{
		Method: req.Method,
		URL:    req.URL.String(),
	}

	// Capture headers if enabled
	if config.IncludeHeaders {
		snapshot.Headers = config.Masker.MaskHeaders(req.Header)
	}

	// Capture body if enabled and present
	if config.IncludeBody && req.Body != nil {
		bodyBytes, err := io.ReadAll(io.LimitReader(req.Body, int64(config.MaxBodySize+1)))
		if err == nil {
			// Restore the body so it can be read again
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// Check if we truncated
			if len(bodyBytes) > config.MaxBodySize {
				bodyBytes = bodyBytes[:config.MaxBodySize]
				snapshot.BodyTruncated = true
			}

			// Mask the body content
			snapshot.Body = config.Masker.MaskString(string(bodyBytes))
		}
	}

	return snapshot
}

// NewResponseSnapshot creates a snapshot of an HTTP response
// The response body will be read and restored so the response can still be used
func NewResponseSnapshot(resp *http.Response, config *SnapshotConfig) *ResponseSnapshot {
	if resp == nil {
		return nil
	}

	if config == nil {
		config = DefaultSnapshotConfig()
	}

	snapshot := &ResponseSnapshot{
		StatusCode: resp.StatusCode,
	}

	// Capture headers if enabled
	if config.IncludeHeaders {
		snapshot.Headers = config.Masker.MaskHeaders(resp.Header)
	}

	// Capture body if enabled and present
	if config.IncludeBody && resp.Body != nil {
		bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(config.MaxBodySize+1)))
		if err == nil {
			// Restore the body so it can be read again
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// Check if we truncated
			if len(bodyBytes) > config.MaxBodySize {
				bodyBytes = bodyBytes[:config.MaxBodySize]
				snapshot.BodyTruncated = true
			}

			// Mask the body content
			snapshot.Body = config.Masker.MaskString(string(bodyBytes))
		}
	}

	return snapshot
}

// NewErrorContext creates a new error context
func NewErrorContext() *ErrorContext {
	return &ErrorContext{
		Timestamp: time.Now(),
	}
}

// WithRequestID sets the request ID
func (ec *ErrorContext) WithRequestID(id string) *ErrorContext {
	ec.RequestID = id
	return ec
}

// WithCorrelationID sets the correlation ID
func (ec *ErrorContext) WithCorrelationID(id string) *ErrorContext {
	ec.CorrelationID = id
	return ec
}

// WithDuration sets the request duration
func (ec *ErrorContext) WithDuration(d time.Duration) *ErrorContext {
	ec.Duration = d
	return ec
}

// WithProvider sets the provider
func (ec *ErrorContext) WithProvider(provider types.ProviderType) *ErrorContext {
	ec.Provider = provider
	return ec
}

// WithModel sets the model
func (ec *ErrorContext) WithModel(model string) *ErrorContext {
	ec.Model = model
	return ec
}

// WithOperation sets the operation
func (ec *ErrorContext) WithOperation(operation string) *ErrorContext {
	ec.Operation = operation
	return ec
}

// WithRequest sets the request snapshot
func (ec *ErrorContext) WithRequest(snapshot *RequestSnapshot) *ErrorContext {
	ec.Request = snapshot
	return ec
}

// WithResponse sets the response snapshot
func (ec *ErrorContext) WithResponse(snapshot *ResponseSnapshot) *ErrorContext {
	ec.Response = snapshot
	return ec
}
