package errors

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// RichError wraps an error with contextual information for debugging and tracing
type RichError struct {
	// err is the underlying error
	err error

	// context contains the error context
	context *ErrorContext

	// snapshotConfig controls how snapshots are created
	snapshotConfig *SnapshotConfig
}

// NewRichError creates a new RichError wrapping the given error
func NewRichError(err error) *RichError {
	return &RichError{
		err:            err,
		context:        NewErrorContext(),
		snapshotConfig: DefaultSnapshotConfig(),
	}
}

// NewRichErrorWithConfig creates a new RichError with a custom snapshot configuration
func NewRichErrorWithConfig(err error, config *SnapshotConfig) *RichError {
	return &RichError{
		err:            err,
		context:        NewErrorContext(),
		snapshotConfig: config,
	}
}

// Error implements the error interface
func (e *RichError) Error() string {
	if e.err == nil {
		return "unknown error"
	}
	return e.err.Error()
}

// Unwrap returns the underlying error for error chain support
func (e *RichError) Unwrap() error {
	return e.err
}

// Context returns the error context
func (e *RichError) Context() *ErrorContext {
	return e.context
}

// WithRequestID sets the request ID and returns the error for chaining
func (e *RichError) WithRequestID(id string) *RichError {
	e.context.RequestID = id
	return e
}

// WithCorrelationID sets the correlation ID and returns the error for chaining
func (e *RichError) WithCorrelationID(id string) *RichError {
	e.context.CorrelationID = id
	return e
}

// WithProvider sets the provider and returns the error for chaining
func (e *RichError) WithProvider(provider types.ProviderType) *RichError {
	e.context.Provider = provider
	return e
}

// WithModel sets the model and returns the error for chaining
func (e *RichError) WithModel(model string) *RichError {
	e.context.Model = model
	return e
}

// WithOperation sets the operation and returns the error for chaining
func (e *RichError) WithOperation(operation string) *RichError {
	e.context.Operation = operation
	return e
}

// WithTiming sets the duration and returns the error for chaining
func (e *RichError) WithTiming(duration time.Duration) *RichError {
	e.context.Duration = duration
	return e
}

// WithTimingStart sets the duration based on a start time and returns the error for chaining
func (e *RichError) WithTimingStart(startTime time.Time) *RichError {
	e.context.Duration = time.Since(startTime)
	return e
}

// WithRequestSnapshot creates and attaches a request snapshot
func (e *RichError) WithRequestSnapshot(req *http.Request) *RichError {
	if req != nil {
		e.context.Request = NewRequestSnapshot(req, e.snapshotConfig)
	}
	return e
}

// WithResponseSnapshot creates and attaches a response snapshot
func (e *RichError) WithResponseSnapshot(resp *http.Response) *RichError {
	if resp != nil {
		e.context.Response = NewResponseSnapshot(resp, e.snapshotConfig)
	}
	return e
}

// WithContext sets the entire error context
func (e *RichError) WithContext(ctx *ErrorContext) *RichError {
	e.context = ctx
	return e
}

// Format returns a detailed formatted error message including all context
func (e *RichError) Format() string {
	var b strings.Builder

	// Error message
	b.WriteString("Error: ")
	b.WriteString(e.Error())
	b.WriteString("\n")

	// Context information
	if e.context != nil {
		e.formatContext(&b)
		e.formatRequestSnapshot(&b)
		e.formatResponseSnapshot(&b)
	}

	return b.String()
}

// formatContext writes context fields to the builder
func (e *RichError) formatContext(b *strings.Builder) {
	b.WriteString("\nContext:\n")

	if e.context.RequestID != "" {
		fmt.Fprintf(b, "  Request ID: %s\n", e.context.RequestID)
	}
	if e.context.CorrelationID != "" {
		fmt.Fprintf(b, "  Correlation ID: %s\n", e.context.CorrelationID)
	}
	if e.context.Provider != "" {
		fmt.Fprintf(b, "  Provider: %s\n", e.context.Provider)
	}
	if e.context.Model != "" {
		fmt.Fprintf(b, "  Model: %s\n", e.context.Model)
	}
	if e.context.Operation != "" {
		fmt.Fprintf(b, "  Operation: %s\n", e.context.Operation)
	}
	if e.context.Duration > 0 {
		fmt.Fprintf(b, "  Duration: %s\n", e.context.Duration)
	}
	if !e.context.Timestamp.IsZero() {
		fmt.Fprintf(b, "  Timestamp: %s\n", e.context.Timestamp.Format(time.RFC3339))
	}
}

// formatRequestSnapshot writes request snapshot to the builder
func (e *RichError) formatRequestSnapshot(b *strings.Builder) {
	if e.context.Request == nil {
		return
	}
	b.WriteString("\nRequest:\n")
	fmt.Fprintf(b, "  Method: %s\n", e.context.Request.Method)
	fmt.Fprintf(b, "  URL: %s\n", e.context.Request.URL)

	formatHeaders(b, e.context.Request.Headers)
	formatBody(b, e.context.Request.Body, e.context.Request.BodyTruncated)
}

// formatResponseSnapshot writes response snapshot to the builder
func (e *RichError) formatResponseSnapshot(b *strings.Builder) {
	if e.context.Response == nil {
		return
	}
	b.WriteString("\nResponse:\n")
	fmt.Fprintf(b, "  Status Code: %d\n", e.context.Response.StatusCode)

	formatHeaders(b, e.context.Response.Headers)
	formatBody(b, e.context.Response.Body, e.context.Response.BodyTruncated)
}

// formatHeaders writes headers to the builder
func formatHeaders(b *strings.Builder, headers http.Header) {
	if len(headers) == 0 {
		return
	}
	b.WriteString("  Headers:\n")
	for key, values := range headers {
		for _, value := range values {
			fmt.Fprintf(b, "    %s: %s\n", key, value)
		}
	}
}

// formatBody writes body to the builder
func formatBody(b *strings.Builder, body string, truncated bool) {
	if body == "" {
		return
	}
	b.WriteString("  Body:\n")
	b.WriteString("    ")
	b.WriteString(body)
	if truncated {
		b.WriteString("\n    [... truncated]")
	}
	b.WriteString("\n")
}

// String returns a string representation of the error
func (e *RichError) String() string {
	return e.Format()
}

// Wrap wraps an error with rich context
// This is a convenience function for NewRichError(err)
func Wrap(err error) *RichError {
	if err == nil {
		return nil
	}
	return NewRichError(err)
}

// WrapWithContext wraps an error with an existing error context
func WrapWithContext(err error, ctx *ErrorContext) *RichError {
	if err == nil {
		return nil
	}
	return &RichError{
		err:            err,
		context:        ctx,
		snapshotConfig: DefaultSnapshotConfig(),
	}
}
