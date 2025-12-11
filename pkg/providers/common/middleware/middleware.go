// Package middleware provides a flexible middleware infrastructure for AI provider requests and responses.
// It enables request transformation, response processing, logging, metrics collection, and other
// cross-cutting concerns through a composable middleware chain pattern.
package middleware

import (
	"context"
	"net/http"
	"sync"
)

// ContextKey is the type used for middleware context keys
type ContextKey string

// Common context keys for passing data between middleware
const (
	// ContextKeyRequestID stores a unique request identifier
	ContextKeyRequestID ContextKey = "middleware:request_id"
	// ContextKeyStartTime stores the request start time
	ContextKeyStartTime ContextKey = "middleware:start_time"
	// ContextKeyProvider stores the provider name
	ContextKeyProvider ContextKey = "middleware:provider"
	// ContextKeyModel stores the model name
	ContextKeyModel ContextKey = "middleware:model"
	// ContextKeyMetadata stores arbitrary metadata
	ContextKeyMetadata ContextKey = "middleware:metadata"
	// ContextKeyError stores error information
	ContextKeyError ContextKey = "middleware:error"
	// ContextKeyRetryCount stores the retry attempt count
	ContextKeyRetryCount ContextKey = "middleware:retry_count"
)

// RequestMiddleware transforms requests before they are sent to the provider
type RequestMiddleware interface {
	// ProcessRequest processes an HTTP request before sending
	// It can modify the request, context, or return an error to abort the request
	ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error)
}

// ResponseMiddleware transforms responses after they are received from the provider
type ResponseMiddleware interface {
	// ProcessResponse processes an HTTP response after receiving
	// It can modify the response, context, or return an error
	ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error)
}

// Middleware is a combined interface for both request and response processing
// Middleware implementations can implement either or both interfaces
type Middleware interface{}

// MiddlewareChain manages an ordered collection of middleware
type MiddlewareChain interface {
	// Add appends middleware to the end of the chain
	Add(middleware Middleware) MiddlewareChain

	// AddBefore inserts middleware before another middleware in the chain
	// Returns false if the target middleware is not found
	AddBefore(target Middleware, middleware Middleware) bool

	// AddAfter inserts middleware after another middleware in the chain
	// Returns false if the target middleware is not found
	AddAfter(target Middleware, middleware Middleware) bool

	// Remove removes middleware from the chain
	// Returns false if the middleware is not found
	Remove(middleware Middleware) bool

	// ProcessRequest executes all request middleware in order
	ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error)

	// ProcessResponse executes all response middleware in reverse order
	ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error)

	// Clear removes all middleware from the chain
	Clear()

	// Len returns the number of middleware in the chain
	Len() int
}

// DefaultMiddlewareChain is the default implementation of MiddlewareChain
type DefaultMiddlewareChain struct {
	middleware []Middleware
	mu         sync.RWMutex
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain() *DefaultMiddlewareChain {
	return &DefaultMiddlewareChain{
		middleware: make([]Middleware, 0),
	}
}

// Add appends middleware to the end of the chain
func (c *DefaultMiddlewareChain) Add(middleware Middleware) MiddlewareChain {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.middleware = append(c.middleware, middleware)
	return c
}

// AddBefore inserts middleware before another middleware in the chain
func (c *DefaultMiddlewareChain) AddBefore(target Middleware, middleware Middleware) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the target middleware
	for i, mw := range c.middleware {
		if mw == target {
			// Insert before target
			c.middleware = append(c.middleware[:i], append([]Middleware{middleware}, c.middleware[i:]...)...)
			return true
		}
	}

	return false
}

// AddAfter inserts middleware after another middleware in the chain
func (c *DefaultMiddlewareChain) AddAfter(target Middleware, middleware Middleware) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the target middleware
	for i, mw := range c.middleware {
		if mw == target {
			// Insert after target
			if i+1 < len(c.middleware) {
				c.middleware = append(c.middleware[:i+1], append([]Middleware{middleware}, c.middleware[i+1:]...)...)
			} else {
				c.middleware = append(c.middleware, middleware)
			}
			return true
		}
	}

	return false
}

// Remove removes middleware from the chain
func (c *DefaultMiddlewareChain) Remove(middleware Middleware) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, mw := range c.middleware {
		if mw == middleware {
			c.middleware = append(c.middleware[:i], c.middleware[i+1:]...)
			return true
		}
	}

	return false
}

// ProcessRequest executes all request middleware in order
func (c *DefaultMiddlewareChain) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	c.mu.RLock()
	// Create a copy of the middleware slice to avoid holding the lock during execution
	middlewareCopy := make([]Middleware, len(c.middleware))
	copy(middlewareCopy, c.middleware)
	c.mu.RUnlock()

	var err error
	for _, mw := range middlewareCopy {
		// Check if this middleware implements RequestMiddleware
		if reqMw, ok := mw.(RequestMiddleware); ok {
			ctx, req, err = reqMw.ProcessRequest(ctx, req)
			if err != nil {
				return ctx, req, err
			}
		}
	}

	return ctx, req, nil
}

// ProcessResponse executes all response middleware in reverse order
func (c *DefaultMiddlewareChain) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	c.mu.RLock()
	// Create a copy of the middleware slice to avoid holding the lock during execution
	middlewareCopy := make([]Middleware, len(c.middleware))
	copy(middlewareCopy, c.middleware)
	c.mu.RUnlock()

	var err error
	// Execute in reverse order
	for i := len(middlewareCopy) - 1; i >= 0; i-- {
		mw := middlewareCopy[i]
		// Check if this middleware implements ResponseMiddleware
		if respMw, ok := mw.(ResponseMiddleware); ok {
			ctx, resp, err = respMw.ProcessResponse(ctx, req, resp)
			if err != nil {
				return ctx, resp, err
			}
		}
	}

	return ctx, resp, nil
}

// Clear removes all middleware from the chain
func (c *DefaultMiddlewareChain) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.middleware = make([]Middleware, 0)
}

// Len returns the number of middleware in the chain
func (c *DefaultMiddlewareChain) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.middleware)
}

// RequestMiddlewareFunc is a function adapter for RequestMiddleware
type RequestMiddlewareFunc func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error)

// ProcessRequest implements RequestMiddleware
func (f RequestMiddlewareFunc) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	return f(ctx, req)
}

// ResponseMiddlewareFunc is a function adapter for ResponseMiddleware
type ResponseMiddlewareFunc func(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error)

// ProcessResponse implements ResponseMiddleware
func (f ResponseMiddlewareFunc) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	return f(ctx, req, resp)
}

// CombinedMiddleware is a middleware that implements both RequestMiddleware and ResponseMiddleware
type CombinedMiddleware struct {
	RequestProcessor  RequestMiddleware
	ResponseProcessor ResponseMiddleware
}

// ProcessRequest implements RequestMiddleware
func (cm *CombinedMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	if cm.RequestProcessor != nil {
		return cm.RequestProcessor.ProcessRequest(ctx, req)
	}
	return ctx, req, nil
}

// ProcessResponse implements ResponseMiddleware
func (cm *CombinedMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	if cm.ResponseProcessor != nil {
		return cm.ResponseProcessor.ProcessResponse(ctx, req, resp)
	}
	return ctx, resp, nil
}

// NewCombinedMiddleware creates a new combined middleware
func NewCombinedMiddleware(reqProcessor RequestMiddleware, respProcessor ResponseMiddleware) *CombinedMiddleware {
	return &CombinedMiddleware{
		RequestProcessor:  reqProcessor,
		ResponseProcessor: respProcessor,
	}
}
