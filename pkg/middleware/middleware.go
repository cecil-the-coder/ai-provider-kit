// Package middleware provides HTTP middleware chain for request and response processing.
//
// This package re-exports the public API from internal/common/middleware, providing
// external consumers with a flexible middleware infrastructure for AI provider requests
// and responses.
//
// # Overview
//
// The middleware package enables request transformation, response processing, logging,
// metrics collection, and other cross-cutting concerns through a composable middleware
// chain pattern. Middleware can process requests before they are sent to AI providers
// and responses after they are received.
//
// # Key Components
//
//   - RequestMiddleware: Processes HTTP requests before sending
//   - ResponseMiddleware: Processes HTTP responses after receiving
//   - Middleware: Combined interface for both request and response processing
//   - MiddlewareChain: Manages ordered middleware execution
//
// # Basic Usage
//
//	// Create a new middleware chain
//	chain := middleware.NewMiddlewareChain()
//
//	// Add middleware to the chain
//	chain.Add(loggingMiddleware).
//	      Add(metricsMiddleware).
//	      Add(retryMiddleware)
//
//	// Process a request
//	ctx := context.Background()
//	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", body)
//	newCtx, newReq, err := chain.ProcessRequest(ctx, req)
//	if err != nil {
//	    // Handle error
//	}
//
//	// Make the HTTP call
//	resp, err := client.Do(newReq)
//	if err != nil {
//	    // Handle error
//	}
//
//	// Process the response
//	newCtx, newResp, err := chain.ProcessResponse(newCtx, newReq, resp)
//
// # Context Keys
//
// The package provides standard context keys for passing data between middleware:
//
//   - ContextKeyRequestID: Unique request identifier
//
//   - ContextKeyStartTime: Request start time
//
//   - ContextKeyProvider: Provider name (e.g., "openai", "anthropic")
//
//   - ContextKeyModel: Model name (e.g., "gpt-4", "claude-3")
//
//   - ContextKeyMetadata: Arbitrary metadata map
//
//   - ContextKeyError: Error information
//
//   - ContextKeyRetryCount: Retry attempt count
//
//     // Store data in context
//     ctx = context.WithValue(ctx, middleware.ContextKeyProvider, "openai")
//     ctx = context.WithValue(ctx, middleware.ContextKeyModel, "gpt-4")
//     ctx = context.WithValue(ctx, middleware.ContextKeyRetryCount, 0)
//
//     // Retrieve data from context
//     if provider, ok := ctx.Value(middleware.ContextKeyProvider).(string); ok {
//     // Use provider
//     }
//
// # Execution Order
//
// The middleware chain executes in a specific order:
//
//   - Request middleware: Execute in the order they were added (first to last)
//   - Response middleware: Execute in reverse order (last to first)
//
// This ensures symmetric processing, similar to nested function calls:
//
//	Request:  MW1 -> MW2 -> MW3 -> [HTTP Call]
//	Response: MW1 <- MW2 <- MW3 <- [HTTP Call]
package middleware

import (
	"context"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/middleware"
)

// Type aliases ensure standard library types are available for users of this package.
//
// These blank references prevent "imported and not used" errors while documenting
// that the package works with standard library types.
var (
	_ context.Context = context.Background()
	_ http.Request    = http.Request{}
	_ http.Response   = http.Response{}
)

// ContextKey is the type used for middleware context keys.
type ContextKey = middleware.ContextKey

// Standard context keys for passing data between middleware.
const (
	// ContextKeyRequestID stores a unique request identifier.
	ContextKeyRequestID ContextKey = middleware.ContextKeyRequestID
	// ContextKeyStartTime stores the request start time.
	ContextKeyStartTime ContextKey = middleware.ContextKeyStartTime
	// ContextKeyProvider stores the provider name.
	ContextKeyProvider ContextKey = middleware.ContextKeyProvider
	// ContextKeyModel stores the model name.
	ContextKeyModel ContextKey = middleware.ContextKeyModel
	// ContextKeyMetadata stores arbitrary metadata.
	ContextKeyMetadata ContextKey = middleware.ContextKeyMetadata
	// ContextKeyError stores error information.
	ContextKeyError ContextKey = middleware.ContextKeyError
	// ContextKeyRetryCount stores the retry attempt count.
	ContextKeyRetryCount ContextKey = middleware.ContextKeyRetryCount
)

// RequestMiddleware transforms requests before they are sent to the provider.
// Implement ProcessRequest to modify the request, context, or return an error to abort.
type RequestMiddleware = middleware.RequestMiddleware

// ResponseMiddleware transforms responses after they are received from the provider.
// Implement ProcessResponse to modify the response, context, or return an error.
type ResponseMiddleware = middleware.ResponseMiddleware

// Middleware is a combined interface for both request and response processing.
// Middleware implementations can implement either or both interfaces.
type Middleware = middleware.Middleware

// MiddlewareChain manages an ordered collection of middleware.
// Provides methods to add, remove, and execute middleware in the chain.
type MiddlewareChain = middleware.MiddlewareChain

// DefaultMiddlewareChain is the default implementation of MiddlewareChain.
// It is thread-safe and can be used concurrently.
type DefaultMiddlewareChain = middleware.DefaultMiddlewareChain

// RequestMiddlewareFunc is a function adapter for RequestMiddleware.
// Allows simple functions to be used as request middleware.
type RequestMiddlewareFunc = middleware.RequestMiddlewareFunc

// ResponseMiddlewareFunc is a function adapter for ResponseMiddleware.
// Allows simple functions to be used as response middleware.
type ResponseMiddlewareFunc = middleware.ResponseMiddlewareFunc

// CombinedMiddleware is a middleware that implements both RequestMiddleware and ResponseMiddleware.
type CombinedMiddleware = middleware.CombinedMiddleware

// NewMiddlewareChain creates a new middleware chain.
// The returned chain is empty and ready to have middleware added.
func NewMiddlewareChain() *middleware.DefaultMiddlewareChain {
	return middleware.NewMiddlewareChain()
}

// NewCombinedMiddleware creates a new combined middleware.
// Use this to combine separate request and response processors into a single middleware.
func NewCombinedMiddleware(reqProcessor middleware.RequestMiddleware, respProcessor middleware.ResponseMiddleware) *middleware.CombinedMiddleware {
	return middleware.NewCombinedMiddleware(reqProcessor, respProcessor)
}
