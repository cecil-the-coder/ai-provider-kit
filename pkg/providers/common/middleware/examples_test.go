package middleware_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
)

// Example of creating a simple logging middleware
func ExampleRequestMiddlewareFunc() {
	logMiddleware := middleware.RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		fmt.Printf("Request: %s %s\n", req.Method, req.URL.Path)
		return ctx, req, nil
	})

	chain := middleware.NewMiddlewareChain()
	chain.Add(logMiddleware)

	req := httptest.NewRequest("GET", "http://example.com/api/v1/test", nil)
	ctx := context.Background()

	_, _, err := chain.ProcessRequest(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// Request: GET /api/v1/test
}

// Example of creating a response status checking middleware
func ExampleResponseMiddlewareFunc() {
	statusMiddleware := middleware.ResponseMiddlewareFunc(func(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
		fmt.Printf("Response status: %d\n", resp.StatusCode)
		return ctx, resp, nil
	})

	chain := middleware.NewMiddlewareChain()
	chain.Add(statusMiddleware)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}
	ctx := context.Background()

	_, _, err := chain.ProcessResponse(ctx, req, resp)
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// Response status: 200
}

// TimingMiddleware is a middleware that tracks request duration
type TimingMiddleware struct{}

func (m *TimingMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	ctx = context.WithValue(ctx, middleware.ContextKeyStartTime, time.Now())
	fmt.Println("Request started")
	return ctx, req, nil
}

func (m *TimingMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	if startTime, ok := ctx.Value(middleware.ContextKeyStartTime).(time.Time); ok {
		duration := time.Since(startTime)
		// Normalize output for testing
		if duration >= time.Millisecond {
			fmt.Println("Request completed")
		}
	}
	return ctx, resp, nil
}

// Example of creating a timing middleware that implements both interfaces
func ExampleNewMiddlewareChain() {
	// Create chain and add middleware
	chain := middleware.NewMiddlewareChain()
	chain.Add(&TimingMiddleware{})

	// Process request
	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	ctx := context.Background()
	ctx, req, _ = chain.ProcessRequest(ctx, req)

	// Simulate some work
	time.Sleep(1 * time.Millisecond)

	// Process response
	resp := &http.Response{StatusCode: 200, Header: make(http.Header)}
	_, _, _ = chain.ProcessResponse(ctx, req, resp)

	// Output:
	// Request started
	// Request completed
}

// Example of using context keys to pass data between middleware
func ExampleContextKey() {
	// First middleware adds request ID
	requestIDMiddleware := middleware.RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		ctx = context.WithValue(ctx, middleware.ContextKeyRequestID, "req-12345")
		ctx = context.WithValue(ctx, middleware.ContextKeyProvider, "openai")
		return ctx, req, nil
	})

	// Second middleware uses the request ID
	logMiddleware := middleware.RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		requestID := ctx.Value(middleware.ContextKeyRequestID)
		provider := ctx.Value(middleware.ContextKeyProvider)
		fmt.Printf("Processing request %v for provider %v\n", requestID, provider)
		return ctx, req, nil
	})

	chain := middleware.NewMiddlewareChain()
	chain.Add(requestIDMiddleware).Add(logMiddleware)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	ctx := context.Background()

	_, _, err := chain.ProcessRequest(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// Processing request req-12345 for provider openai
}

// ExampleMiddleware demonstrates a simple middleware implementation
type ExampleMiddleware struct {
	name string
}

func (m *ExampleMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	fmt.Printf("Middleware %s\n", m.name)
	return ctx, req, nil
}

// Example of adding middleware in specific positions
func ExampleDefaultMiddlewareChain_AddBefore() {
	mw1 := &ExampleMiddleware{name: "1"}
	mw2 := &ExampleMiddleware{name: "2"}
	mw3 := &ExampleMiddleware{name: "3"}

	chain := middleware.NewMiddlewareChain()
	chain.Add(mw1).Add(mw3)

	// Insert mw2 before mw3
	chain.AddBefore(mw3, mw2)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	ctx := context.Background()

	_, _, err := chain.ProcessRequest(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// Middleware 1
	// Middleware 2
	// Middleware 3
}

// Example of combining multiple middleware types
func ExampleCombinedMiddleware() {
	// Header middleware
	headerMw := middleware.RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		req.Header.Set("User-Agent", "AI-Provider-Kit/1.0")
		fmt.Println("Added User-Agent header")
		return ctx, req, nil
	})

	// Response validation middleware
	validationMw := middleware.ResponseMiddlewareFunc(func(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
		if resp.StatusCode >= 400 {
			fmt.Printf("Error response: %d\n", resp.StatusCode)
		} else {
			fmt.Println("Successful response")
		}
		return ctx, resp, nil
	})

	// Combine them
	combined := middleware.NewCombinedMiddleware(headerMw, validationMw)

	chain := middleware.NewMiddlewareChain()
	chain.Add(combined)

	req := httptest.NewRequest("POST", "http://example.com/api", nil)
	ctx := context.Background()

	// Process request
	_, req, _ = chain.ProcessRequest(ctx, req)

	// Process response
	resp := &http.Response{StatusCode: 200, Header: make(http.Header)}
	_, _, _ = chain.ProcessResponse(ctx, req, resp)

	// Output:
	// Added User-Agent header
	// Successful response
}

// Example of error handling in middleware
func ExampleMiddlewareChain_ProcessRequest_error() {
	validationMw := middleware.RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		if req.Header.Get("Authorization") == "" {
			return ctx, req, fmt.Errorf("missing authorization header")
		}
		return ctx, req, nil
	})

	chain := middleware.NewMiddlewareChain()
	chain.Add(validationMw)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	ctx := context.Background()

	_, _, err := chain.ProcessRequest(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Output:
	// Error: missing authorization header
}

// Example showing execution order: requests forward, responses reverse
func ExampleDefaultMiddlewareChain_executionOrder() {
	mw1 := middleware.RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		fmt.Println("Request: MW1")
		return ctx, req, nil
	})

	mw2 := middleware.RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		fmt.Println("Request: MW2")
		return ctx, req, nil
	})

	respMw1 := middleware.ResponseMiddlewareFunc(func(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
		fmt.Println("Response: MW1")
		return ctx, resp, nil
	})

	respMw2 := middleware.ResponseMiddlewareFunc(func(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
		fmt.Println("Response: MW2")
		return ctx, resp, nil
	})

	// Create combined middleware
	combined1 := middleware.NewCombinedMiddleware(mw1, respMw1)
	combined2 := middleware.NewCombinedMiddleware(mw2, respMw2)

	chain := middleware.NewMiddlewareChain()
	chain.Add(combined1).Add(combined2)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	resp := &http.Response{StatusCode: 200, Header: make(http.Header)}
	ctx := context.Background()

	ctx, req, _ = chain.ProcessRequest(ctx, req)
	fmt.Println("--- HTTP Call ---")
	_, _, _ = chain.ProcessResponse(ctx, req, resp)

	// Output:
	// Request: MW1
	// Request: MW2
	// --- HTTP Call ---
	// Response: MW2
	// Response: MW1
}
