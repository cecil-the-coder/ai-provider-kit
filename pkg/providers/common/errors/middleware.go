package errors

import (
	"context"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/google/uuid"
)

// ContextKey type for error context keys
type ContextKey string

const (
	// ContextKeyErrorContext stores the error context in the request context
	ContextKeyErrorContext ContextKey = "errors:error_context"
	// ContextKeyCorrelationID stores the correlation ID in the request context
	ContextKeyCorrelationID ContextKey = "errors:correlation_id"
)

// ErrorContextMiddleware captures error context automatically for requests
type ErrorContextMiddleware struct {
	// provider is the provider name
	provider types.ProviderType

	// config is the snapshot configuration
	config *SnapshotConfig

	// generateRequestID determines if request IDs should be generated
	generateRequestID bool

	// generateCorrelationID determines if correlation IDs should be generated
	generateCorrelationID bool

	// correlationIDHeader is the header name for correlation IDs
	correlationIDHeader string
}

// ErrorContextMiddlewareConfig configures the error context middleware
type ErrorContextMiddlewareConfig struct {
	// Provider is the provider name
	Provider types.ProviderType

	// SnapshotConfig controls how snapshots are created
	SnapshotConfig *SnapshotConfig

	// GenerateRequestID determines if request IDs should be generated
	GenerateRequestID bool

	// GenerateCorrelationID determines if correlation IDs should be generated
	GenerateCorrelationID bool

	// CorrelationIDHeader is the header name for correlation IDs
	// Defaults to "X-Correlation-ID"
	CorrelationIDHeader string
}

// DefaultErrorContextMiddlewareConfig returns the default configuration
func DefaultErrorContextMiddlewareConfig(provider types.ProviderType) *ErrorContextMiddlewareConfig {
	return &ErrorContextMiddlewareConfig{
		Provider:              provider,
		SnapshotConfig:        DefaultSnapshotConfig(),
		GenerateRequestID:     true,
		GenerateCorrelationID: true,
		CorrelationIDHeader:   "X-Correlation-ID",
	}
}

// NewErrorContextMiddleware creates a new error context middleware
func NewErrorContextMiddleware(config *ErrorContextMiddlewareConfig) *ErrorContextMiddleware {
	if config == nil {
		config = DefaultErrorContextMiddlewareConfig("")
	}

	if config.SnapshotConfig == nil {
		config.SnapshotConfig = DefaultSnapshotConfig()
	}

	if config.CorrelationIDHeader == "" {
		config.CorrelationIDHeader = "X-Correlation-ID"
	}

	return &ErrorContextMiddleware{
		provider:              config.Provider,
		config:                config.SnapshotConfig,
		generateRequestID:     config.GenerateRequestID,
		generateCorrelationID: config.GenerateCorrelationID,
		correlationIDHeader:   config.CorrelationIDHeader,
	}
}

// ProcessRequest implements RequestMiddleware
func (m *ErrorContextMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	// Create error context
	errCtx := NewErrorContext()
	errCtx.Provider = m.provider
	errCtx.Timestamp = time.Now()

	// Extract or generate request ID
	if m.generateRequestID {
		requestID := req.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateID()
			req.Header.Set("X-Request-ID", requestID)
		}
		errCtx.RequestID = requestID
	}

	// Extract or generate correlation ID
	correlationID := req.Header.Get(m.correlationIDHeader)
	if correlationID == "" && m.generateCorrelationID {
		correlationID = generateID()
		req.Header.Set(m.correlationIDHeader, correlationID)
	}
	if correlationID != "" {
		errCtx.CorrelationID = correlationID
	}

	// Extract provider and model from context if available
	if provider, ok := ctx.Value(middleware.ContextKeyProvider).(string); ok {
		errCtx.Provider = types.ProviderType(provider)
	}
	if model, ok := ctx.Value(middleware.ContextKeyModel).(string); ok {
		errCtx.Model = model
	}

	// Create request snapshot
	errCtx.Request = NewRequestSnapshot(req, m.config)

	// Store error context in request context
	ctx = context.WithValue(ctx, ContextKeyErrorContext, errCtx)
	ctx = context.WithValue(ctx, middleware.ContextKeyRequestID, errCtx.RequestID)
	if errCtx.CorrelationID != "" {
		ctx = context.WithValue(ctx, ContextKeyCorrelationID, errCtx.CorrelationID)
	}

	return ctx, req, nil
}

// ProcessResponse implements ResponseMiddleware
func (m *ErrorContextMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	// Retrieve error context from request context
	errCtx, ok := ctx.Value(ContextKeyErrorContext).(*ErrorContext)
	if !ok {
		// If we don't have an error context, create one now
		errCtx = NewErrorContext()
		errCtx.Provider = m.provider
	}

	// Calculate duration
	errCtx.Duration = time.Since(errCtx.Timestamp)

	// Create response snapshot
	errCtx.Response = NewResponseSnapshot(resp, m.config)

	// Update context in case it was created here
	ctx = context.WithValue(ctx, ContextKeyErrorContext, errCtx)

	return ctx, resp, nil
}

// GetErrorContext retrieves the error context from a context
func GetErrorContext(ctx context.Context) *ErrorContext {
	if ctx == nil {
		return nil
	}

	errCtx, ok := ctx.Value(ContextKeyErrorContext).(*ErrorContext)
	if !ok {
		return nil
	}

	return errCtx
}

// GetCorrelationID retrieves the correlation ID from a context
func GetCorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	// Try to get from error context first
	if errCtx := GetErrorContext(ctx); errCtx != nil && errCtx.CorrelationID != "" {
		return errCtx.CorrelationID
	}

	// Fall back to direct context value
	if id, ok := ctx.Value(ContextKeyCorrelationID).(string); ok {
		return id
	}

	return ""
}

// EnrichError adds error context to an error
// If the context contains error context, it will be attached to the error
func EnrichError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	errCtx := GetErrorContext(ctx)
	if errCtx == nil {
		// No error context available, return original error
		return err
	}

	// Wrap the error with rich context
	return WrapWithContext(err, errCtx)
}

// generateID generates a unique ID for requests and correlations
func generateID() string {
	return uuid.New().String()
}

// CorrelationMiddleware is a simple middleware that adds correlation IDs to requests
// This is a lighter-weight alternative to ErrorContextMiddleware when you only need correlation tracking
type CorrelationMiddleware struct {
	headerName string
	generate   bool
}

// NewCorrelationMiddleware creates a new correlation middleware
func NewCorrelationMiddleware(headerName string, generate bool) *CorrelationMiddleware {
	if headerName == "" {
		headerName = "X-Correlation-ID"
	}

	return &CorrelationMiddleware{
		headerName: headerName,
		generate:   generate,
	}
}

// ProcessRequest implements RequestMiddleware
func (m *CorrelationMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	correlationID := req.Header.Get(m.headerName)
	if correlationID == "" && m.generate {
		correlationID = generateID()
		req.Header.Set(m.headerName, correlationID)
	}

	if correlationID != "" {
		ctx = context.WithValue(ctx, ContextKeyCorrelationID, correlationID)
	}

	return ctx, req, nil
}
