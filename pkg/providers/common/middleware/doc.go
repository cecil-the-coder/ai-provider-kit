// Package middleware provides a flexible middleware infrastructure for AI provider requests and responses.
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
// The package provides several key interfaces and types:
//
//   - RequestMiddleware: Processes HTTP requests before sending
//   - ResponseMiddleware: Processes HTTP responses after receiving
//   - Middleware: Combined interface for both request and response processing
//   - MiddlewareChain: Manages ordered middleware execution
//
// # Basic Usage
//
// Creating and using a middleware chain:
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
// # Creating Custom Middleware
//
// Implementing RequestMiddleware:
//
//	type LoggingMiddleware struct {
//	    logger *log.Logger
//	}
//
//	func (m *LoggingMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
//	    m.logger.Printf("Request: %s %s", req.Method, req.URL)
//	    // Add request ID to context
//	    ctx = context.WithValue(ctx, middleware.ContextKeyRequestID, generateID())
//	    return ctx, req, nil
//	}
//
// Implementing ResponseMiddleware:
//
//	type MetricsMiddleware struct {
//	    metrics MetricsCollector
//	}
//
//	func (m *MetricsMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
//	    // Record metrics
//	    m.metrics.RecordStatusCode(resp.StatusCode)
//	    return ctx, resp, nil
//	}
//
// Implementing both interfaces:
//
//	type TimingMiddleware struct {
//	    logger *log.Logger
//	}
//
//	func (m *TimingMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
//	    ctx = context.WithValue(ctx, middleware.ContextKeyStartTime, time.Now())
//	    return ctx, req, nil
//	}
//
//	func (m *TimingMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
//	    if startTime, ok := ctx.Value(middleware.ContextKeyStartTime).(time.Time); ok {
//	        duration := time.Since(startTime)
//	        m.logger.Printf("Request took %v", duration)
//	    }
//	    return ctx, resp, nil
//	}
//
// # Using Function Adapters
//
// For simple middleware, use function adapters:
//
//	// Request middleware function
//	headerMiddleware := middleware.RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
//	    req.Header.Set("User-Agent", "AI-Provider-Kit/1.0")
//	    return ctx, req, nil
//	})
//
//	// Response middleware function
//	statusMiddleware := middleware.ResponseMiddlewareFunc(func(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
//	    if resp.StatusCode >= 500 {
//	        return ctx, resp, fmt.Errorf("server error: %d", resp.StatusCode)
//	    }
//	    return ctx, resp, nil
//	})
//
//	chain.Add(headerMiddleware).Add(statusMiddleware)
//
// # Advanced Chain Operations
//
// Inserting middleware at specific positions:
//
//	// Add before another middleware
//	chain.AddBefore(existingMiddleware, newMiddleware)
//
//	// Add after another middleware
//	chain.AddAfter(existingMiddleware, newMiddleware)
//
//	// Remove middleware
//	chain.Remove(middlewareToRemove)
//
//	// Clear all middleware
//	chain.Clear()
//
// # Context Keys
//
// The package provides standard context keys for passing data between middleware:
//
//   - ContextKeyRequestID: Unique request identifier
//   - ContextKeyStartTime: Request start time
//   - ContextKeyProvider: Provider name (e.g., "openai", "anthropic")
//   - ContextKeyModel: Model name (e.g., "gpt-4", "claude-3")
//   - ContextKeyMetadata: Arbitrary metadata map
//   - ContextKeyError: Error information
//   - ContextKeyRetryCount: Retry attempt count
//
// Using context keys:
//
//	// Store data in context
//	ctx = context.WithValue(ctx, middleware.ContextKeyProvider, "openai")
//	ctx = context.WithValue(ctx, middleware.ContextKeyModel, "gpt-4")
//	ctx = context.WithValue(ctx, middleware.ContextKeyRetryCount, 0)
//
//	// Retrieve data from context
//	if provider, ok := ctx.Value(middleware.ContextKeyProvider).(string); ok {
//	    // Use provider
//	}
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
//
// # Error Handling
//
// Middleware can return errors to abort the chain:
//
//	func (m *ValidationMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
//	    if req.Header.Get("Authorization") == "" {
//	        return ctx, req, errors.New("missing authorization header")
//	    }
//	    return ctx, req, nil
//	}
//
// When an error is returned:
//   - Request middleware: Subsequent middleware are not executed
//   - Response middleware: Subsequent middleware (earlier in chain) are not executed
//
// # Thread Safety
//
// DefaultMiddlewareChain is thread-safe and can be used concurrently:
//
//   - Adding/removing middleware uses write locks
//   - Processing requests/responses uses read locks
//   - Multiple goroutines can process requests/responses simultaneously
//
// # Performance Considerations
//
// The middleware chain is designed for efficiency:
//
//   - Minimal allocations during processing
//   - Lock-free execution after chain is built
//   - Benchmarks show ~900ns per request with 10 middleware
//
// # Complete Example
//
//	package main
//
//	import (
//	    "context"
//	    "log"
//	    "net/http"
//	    "time"
//
//	    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
//	)
//
//	type LoggingMiddleware struct {
//	    logger *log.Logger
//	}
//
//	func (m *LoggingMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
//	    m.logger.Printf("-> %s %s", req.Method, req.URL)
//	    ctx = context.WithValue(ctx, middleware.ContextKeyStartTime, time.Now())
//	    return ctx, req, nil
//	}
//
//	func (m *LoggingMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
//	    if startTime, ok := ctx.Value(middleware.ContextKeyStartTime).(time.Time); ok {
//	        m.logger.Printf("<- %s %s (%d) in %v", req.Method, req.URL, resp.StatusCode, time.Since(startTime))
//	    }
//	    return ctx, resp, nil
//	}
//
//	func main() {
//	    // Create middleware chain
//	    chain := middleware.NewMiddlewareChain()
//
//	    // Add middleware
//	    chain.Add(&LoggingMiddleware{logger: log.Default()})
//
//	    // Process request
//	    req, _ := http.NewRequest("GET", "https://api.example.com/v1/models", nil)
//	    ctx, req, _ := chain.ProcessRequest(context.Background(), req)
//
//	    // Make HTTP call
//	    client := &http.Client{}
//	    resp, _ := client.Do(req)
//	    defer resp.Body.Close()
//
//	    // Process response
//	    _, _, _ = chain.ProcessResponse(ctx, req, resp)
//	}
//
// # Best Practices
//
//  1. Keep middleware focused on a single concern
//  2. Use context keys to pass data between middleware
//  3. Handle errors appropriately (abort vs. log and continue)
//  4. Consider middleware order carefully
//  5. Use function adapters for simple, stateless middleware
//  6. Avoid holding locks or blocking in middleware
//  7. Test middleware in isolation and as part of a chain
package middleware
