package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock middleware implementations for testing

type mockRequestMiddleware struct {
	called bool
	mu     sync.Mutex
	err    error
}

func (m *mockRequestMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	if m.err != nil {
		return ctx, req, m.err
	}
	// Add a header to prove this middleware was called
	req.Header.Set("X-Request-Processed", "true")
	return ctx, req, nil
}

func (m *mockRequestMiddleware) wasCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

func (m *mockRequestMiddleware) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = false
}

type mockResponseMiddleware struct {
	called bool
	mu     sync.Mutex
	err    error
}

func (m *mockResponseMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	if m.err != nil {
		return ctx, resp, m.err
	}
	// Add a header to prove this middleware was called
	resp.Header.Set("X-Response-Processed", "true")
	return ctx, resp, nil
}

func (m *mockResponseMiddleware) wasCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

func (m *mockResponseMiddleware) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = false
}

type mockBothMiddleware struct {
	requestCalled  bool
	responseCalled bool
	mu             sync.Mutex
	requestErr     error
	responseErr    error
}

func (m *mockBothMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestCalled = true
	if m.requestErr != nil {
		return ctx, req, m.requestErr
	}
	req.Header.Set("X-Both-Request", "true")
	return ctx, req, nil
}

func (m *mockBothMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseCalled = true
	if m.responseErr != nil {
		return ctx, resp, m.responseErr
	}
	resp.Header.Set("X-Both-Response", "true")
	return ctx, resp, nil
}

func (m *mockBothMiddleware) wasRequestCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requestCalled
}

func (m *mockBothMiddleware) wasResponseCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.responseCalled
}

// Order tracking middleware

type orderTrackingMiddleware struct {
	id            string
	order         *[]string
	mu            *sync.Mutex
	requestIndex  int
	responseIndex int
}

func (o *orderTrackingMiddleware) ProcessRequest(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.requestIndex = len(*o.order)
	*o.order = append(*o.order, "req:"+o.id)
	return ctx, req, nil
}

func (o *orderTrackingMiddleware) ProcessResponse(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.responseIndex = len(*o.order)
	*o.order = append(*o.order, "resp:"+o.id)
	return ctx, resp, nil
}

// Tests

func TestNewMiddlewareChain(t *testing.T) {
	chain := NewMiddlewareChain()
	assert.NotNil(t, chain)
	assert.Equal(t, 0, chain.Len())
}

func TestMiddlewareChain_Add(t *testing.T) {
	chain := NewMiddlewareChain()
	mw := &mockRequestMiddleware{}

	result := chain.Add(mw)
	assert.Equal(t, chain, result) // Should return self for chaining
	assert.Equal(t, 1, chain.Len())
}

func TestMiddlewareChain_AddMultiple(t *testing.T) {
	chain := NewMiddlewareChain()
	mw1 := &mockRequestMiddleware{}
	mw2 := &mockResponseMiddleware{}
	mw3 := &mockBothMiddleware{}

	chain.Add(mw1).Add(mw2).Add(mw3)
	assert.Equal(t, 3, chain.Len())
}

func TestMiddlewareChain_Remove(t *testing.T) {
	chain := NewMiddlewareChain()
	mw1 := &mockRequestMiddleware{}
	mw2 := &mockResponseMiddleware{}
	mw3 := &mockBothMiddleware{}

	chain.Add(mw1).Add(mw2).Add(mw3)
	assert.Equal(t, 3, chain.Len())

	// Remove middle middleware
	removed := chain.Remove(mw2)
	assert.True(t, removed)
	assert.Equal(t, 2, chain.Len())

	// Try to remove non-existent middleware
	removed = chain.Remove(mw2)
	assert.False(t, removed)
	assert.Equal(t, 2, chain.Len())
}

func TestMiddlewareChain_Clear(t *testing.T) {
	chain := NewMiddlewareChain()
	chain.Add(&mockRequestMiddleware{})
	chain.Add(&mockResponseMiddleware{})
	chain.Add(&mockBothMiddleware{})

	assert.Equal(t, 3, chain.Len())
	chain.Clear()
	assert.Equal(t, 0, chain.Len())
}

func TestMiddlewareChain_AddBefore(t *testing.T) {
	chain := NewMiddlewareChain()
	mw1 := &mockRequestMiddleware{}
	mw2 := &mockResponseMiddleware{}
	mw3 := &mockBothMiddleware{}

	chain.Add(mw1).Add(mw3)

	// Add mw2 before mw3
	added := chain.AddBefore(mw3, mw2)
	assert.True(t, added)
	assert.Equal(t, 3, chain.Len())

	// Verify order by processing
	order := make([]string, 0)
	mu := &sync.Mutex{}
	orderMw1 := &orderTrackingMiddleware{id: "1", order: &order, mu: mu}
	orderMw2 := &orderTrackingMiddleware{id: "2", order: &order, mu: mu}
	orderMw3 := &orderTrackingMiddleware{id: "3", order: &order, mu: mu}

	chain2 := NewMiddlewareChain()
	chain2.Add(orderMw1).Add(orderMw3)
	chain2.AddBefore(orderMw3, orderMw2)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := context.Background()
	_, _, err := chain2.ProcessRequest(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, []string{"req:1", "req:2", "req:3"}, order)
}

func TestMiddlewareChain_AddAfter(t *testing.T) {
	chain := NewMiddlewareChain()
	mw1 := &mockRequestMiddleware{}
	mw2 := &mockResponseMiddleware{}
	mw3 := &mockBothMiddleware{}

	chain.Add(mw1).Add(mw3)

	// Add mw2 after mw1
	added := chain.AddAfter(mw1, mw2)
	assert.True(t, added)
	assert.Equal(t, 3, chain.Len())

	// Verify order
	order := make([]string, 0)
	mu := &sync.Mutex{}
	orderMw1 := &orderTrackingMiddleware{id: "1", order: &order, mu: mu}
	orderMw2 := &orderTrackingMiddleware{id: "2", order: &order, mu: mu}
	orderMw3 := &orderTrackingMiddleware{id: "3", order: &order, mu: mu}

	chain2 := NewMiddlewareChain()
	chain2.Add(orderMw1).Add(orderMw3)
	chain2.AddAfter(orderMw1, orderMw2)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := context.Background()
	_, _, err := chain2.ProcessRequest(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, []string{"req:1", "req:2", "req:3"}, order)
}

func TestMiddlewareChain_AddBeforeNotFound(t *testing.T) {
	chain := NewMiddlewareChain()
	mw1 := &mockRequestMiddleware{}
	mw2 := &mockResponseMiddleware{}
	mw3 := &mockBothMiddleware{}

	chain.Add(mw1)

	// Try to add before non-existent middleware
	added := chain.AddBefore(mw3, mw2)
	assert.False(t, added)
	assert.Equal(t, 1, chain.Len())
}

func TestMiddlewareChain_AddAfterNotFound(t *testing.T) {
	chain := NewMiddlewareChain()
	mw1 := &mockRequestMiddleware{}
	mw2 := &mockResponseMiddleware{}
	mw3 := &mockBothMiddleware{}

	chain.Add(mw1)

	// Try to add after non-existent middleware
	added := chain.AddAfter(mw3, mw2)
	assert.False(t, added)
	assert.Equal(t, 1, chain.Len())
}

func TestMiddlewareChain_ProcessRequest(t *testing.T) {
	chain := NewMiddlewareChain()
	reqMw := &mockRequestMiddleware{}
	respMw := &mockResponseMiddleware{}
	bothMw := &mockBothMiddleware{}

	chain.Add(reqMw).Add(respMw).Add(bothMw)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := context.Background()

	newCtx, newReq, err := chain.ProcessRequest(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, newCtx)
	assert.NotNil(t, newReq)

	// Verify that request middleware was called
	assert.True(t, reqMw.wasCalled())
	assert.False(t, respMw.wasCalled()) // Response middleware should not be called
	assert.True(t, bothMw.wasRequestCalled())
	assert.False(t, bothMw.wasResponseCalled())

	// Verify headers were set
	assert.Equal(t, "true", newReq.Header.Get("X-Request-Processed"))
	assert.Equal(t, "true", newReq.Header.Get("X-Both-Request"))
}

func TestMiddlewareChain_ProcessResponse(t *testing.T) {
	chain := NewMiddlewareChain()
	reqMw := &mockRequestMiddleware{}
	respMw := &mockResponseMiddleware{}
	bothMw := &mockBothMiddleware{}

	chain.Add(reqMw).Add(respMw).Add(bothMw)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}
	ctx := context.Background()

	newCtx, newResp, err := chain.ProcessResponse(ctx, req, resp)
	require.NoError(t, err)
	assert.NotNil(t, newCtx)
	assert.NotNil(t, newResp)

	// Verify that response middleware was called
	assert.False(t, reqMw.wasCalled()) // Request middleware should not be called
	assert.True(t, respMw.wasCalled())
	assert.False(t, bothMw.wasRequestCalled())
	assert.True(t, bothMw.wasResponseCalled())

	// Verify headers were set
	assert.Equal(t, "true", newResp.Header.Get("X-Response-Processed"))
	assert.Equal(t, "true", newResp.Header.Get("X-Both-Response"))
}

func TestMiddlewareChain_ProcessRequestError(t *testing.T) {
	chain := NewMiddlewareChain()
	mw1 := &mockRequestMiddleware{}
	mw2 := &mockRequestMiddleware{err: errors.New("request error")}
	mw3 := &mockRequestMiddleware{}

	chain.Add(mw1).Add(mw2).Add(mw3)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := context.Background()

	_, _, err := chain.ProcessRequest(ctx, req)
	assert.Error(t, err)
	assert.Equal(t, "request error", err.Error())

	// First middleware should be called
	assert.True(t, mw1.wasCalled())
	// Second middleware should be called and return error
	assert.True(t, mw2.wasCalled())
	// Third middleware should not be called due to error
	assert.False(t, mw3.wasCalled())
}

func TestMiddlewareChain_ProcessResponseError(t *testing.T) {
	chain := NewMiddlewareChain()
	mw1 := &mockResponseMiddleware{}
	mw2 := &mockResponseMiddleware{err: errors.New("response error")}
	mw3 := &mockResponseMiddleware{}

	chain.Add(mw1).Add(mw2).Add(mw3)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{Header: make(http.Header)}
	ctx := context.Background()

	_, _, err := chain.ProcessResponse(ctx, req, resp)
	assert.Error(t, err)
	assert.Equal(t, "response error", err.Error())

	// Response middleware executes in reverse order
	// Third middleware should be called first
	assert.True(t, mw3.wasCalled())
	// Second middleware should be called and return error
	assert.True(t, mw2.wasCalled())
	// First middleware should not be called due to error
	assert.False(t, mw1.wasCalled())
}

func TestMiddlewareChain_ExecutionOrder(t *testing.T) {
	chain := NewMiddlewareChain()
	order := make([]string, 0)
	mu := &sync.Mutex{}

	mw1 := &orderTrackingMiddleware{id: "1", order: &order, mu: mu}
	mw2 := &orderTrackingMiddleware{id: "2", order: &order, mu: mu}
	mw3 := &orderTrackingMiddleware{id: "3", order: &order, mu: mu}

	chain.Add(mw1).Add(mw2).Add(mw3)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{Header: make(http.Header)}
	ctx := context.Background()

	// Process request
	_, _, err := chain.ProcessRequest(ctx, req)
	require.NoError(t, err)

	// Process response
	_, _, err = chain.ProcessResponse(ctx, req, resp)
	require.NoError(t, err)

	// Verify order: requests in order, responses in reverse
	expected := []string{
		"req:1", "req:2", "req:3", // Request order: 1, 2, 3
		"resp:3", "resp:2", "resp:1", // Response order: 3, 2, 1 (reverse)
	}
	assert.Equal(t, expected, order)

	// Verify indices
	assert.Equal(t, 0, mw1.requestIndex)
	assert.Equal(t, 5, mw1.responseIndex)
	assert.Equal(t, 1, mw2.requestIndex)
	assert.Equal(t, 4, mw2.responseIndex)
	assert.Equal(t, 2, mw3.requestIndex)
	assert.Equal(t, 3, mw3.responseIndex)
}

func TestMiddlewareChain_Concurrency(t *testing.T) {
	chain := NewMiddlewareChain()
	mw := &mockBothMiddleware{}
	chain.Add(mw)

	var wg sync.WaitGroup
	numGoroutines := 100

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{Header: make(http.Header)}
	ctx := context.Background()

	// Test concurrent requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := chain.ProcessRequest(ctx, req)
			assert.NoError(t, err)
		}()
	}

	// Test concurrent responses
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := chain.ProcessResponse(ctx, req, resp)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
	assert.True(t, mw.wasRequestCalled())
	assert.True(t, mw.wasResponseCalled())
}

func TestRequestMiddlewareFunc(t *testing.T) {
	called := false
	fn := RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		called = true
		req.Header.Set("X-Func", "true")
		return ctx, req, nil
	})

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := context.Background()

	newCtx, newReq, err := fn.ProcessRequest(ctx, req)
	require.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, newCtx)
	assert.NotNil(t, newReq)
	assert.Equal(t, "true", newReq.Header.Get("X-Func"))
}

func TestResponseMiddlewareFunc(t *testing.T) {
	called := false
	fn := ResponseMiddlewareFunc(func(ctx context.Context, req *http.Request, resp *http.Response) (context.Context, *http.Response, error) {
		called = true
		resp.Header.Set("X-Func", "true")
		return ctx, resp, nil
	})

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{Header: make(http.Header)}
	ctx := context.Background()

	newCtx, newResp, err := fn.ProcessResponse(ctx, req, resp)
	require.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, newCtx)
	assert.NotNil(t, newResp)
	assert.Equal(t, "true", newResp.Header.Get("X-Func"))
}

func TestCombinedMiddleware(t *testing.T) {
	reqMw := &mockRequestMiddleware{}
	respMw := &mockResponseMiddleware{}

	combined := NewCombinedMiddleware(reqMw, respMw)
	assert.NotNil(t, combined)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{Header: make(http.Header)}
	ctx := context.Background()

	// Test request processing
	_, _, err := combined.ProcessRequest(ctx, req)
	require.NoError(t, err)
	assert.True(t, reqMw.wasCalled())

	// Test response processing
	_, _, err = combined.ProcessResponse(ctx, req, resp)
	require.NoError(t, err)
	assert.True(t, respMw.wasCalled())
}

func TestCombinedMiddleware_NilProcessors(t *testing.T) {
	combined := NewCombinedMiddleware(nil, nil)
	assert.NotNil(t, combined)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{Header: make(http.Header)}
	ctx := context.Background()

	// Should not panic with nil processors
	_, _, err := combined.ProcessRequest(ctx, req)
	require.NoError(t, err)

	_, _, err = combined.ProcessResponse(ctx, req, resp)
	require.NoError(t, err)
}

func TestContextKeys(t *testing.T) {
	// Test that context keys are distinct
	keys := []ContextKey{
		ContextKeyRequestID,
		ContextKeyStartTime,
		ContextKeyProvider,
		ContextKeyModel,
		ContextKeyMetadata,
		ContextKeyError,
		ContextKeyRetryCount,
	}

	// Verify all keys are unique
	keyMap := make(map[ContextKey]bool)
	for _, key := range keys {
		assert.False(t, keyMap[key], "duplicate context key: %s", key)
		keyMap[key] = true
	}

	// Test using context keys
	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeyRequestID, "req-123")
	ctx = context.WithValue(ctx, ContextKeyProvider, "openai")
	ctx = context.WithValue(ctx, ContextKeyRetryCount, 3)

	assert.Equal(t, "req-123", ctx.Value(ContextKeyRequestID))
	assert.Equal(t, "openai", ctx.Value(ContextKeyProvider))
	assert.Equal(t, 3, ctx.Value(ContextKeyRetryCount))
}

func TestMiddlewareChain_EmptyChain(t *testing.T) {
	chain := NewMiddlewareChain()

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{Header: make(http.Header)}
	ctx := context.Background()

	// Should work fine with empty chain
	newCtx, newReq, err := chain.ProcessRequest(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctx, newCtx)
	assert.Equal(t, req, newReq)

	newCtx, newResp, err := chain.ProcessResponse(ctx, req, resp)
	require.NoError(t, err)
	assert.Equal(t, ctx, newCtx)
	assert.Equal(t, resp, newResp)
}

func TestMiddlewareChain_AddAfterLast(t *testing.T) {
	chain := NewMiddlewareChain()
	mw1 := &mockRequestMiddleware{}
	mw2 := &mockResponseMiddleware{}
	mw3 := &mockBothMiddleware{}

	chain.Add(mw1).Add(mw2)

	// Add after the last middleware
	added := chain.AddAfter(mw2, mw3)
	assert.True(t, added)
	assert.Equal(t, 3, chain.Len())

	// Verify it's actually at the end
	order := make([]string, 0)
	mu := &sync.Mutex{}
	orderMw1 := &orderTrackingMiddleware{id: "1", order: &order, mu: mu}
	orderMw2 := &orderTrackingMiddleware{id: "2", order: &order, mu: mu}
	orderMw3 := &orderTrackingMiddleware{id: "3", order: &order, mu: mu}

	chain2 := NewMiddlewareChain()
	chain2.Add(orderMw1).Add(orderMw2)
	chain2.AddAfter(orderMw2, orderMw3)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := context.Background()
	_, _, err := chain2.ProcessRequest(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, []string{"req:1", "req:2", "req:3"}, order)
}

func TestMiddlewareChain_ContextModification(t *testing.T) {
	chain := NewMiddlewareChain()

	// Middleware that adds to context
	mw1 := RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		ctx = context.WithValue(ctx, ContextKeyRequestID, "test-123")
		return ctx, req, nil
	})

	// Middleware that reads from context
	mw2 := RequestMiddlewareFunc(func(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
		requestID := ctx.Value(ContextKeyRequestID)
		if requestID != nil {
			req.Header.Set("X-Request-ID", requestID.(string))
		}
		return ctx, req, nil
	})

	chain.Add(mw1).Add(mw2)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := context.Background()

	newCtx, newReq, err := chain.ProcessRequest(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, newCtx)
	assert.Equal(t, "test-123", newReq.Header.Get("X-Request-ID"))
	assert.Equal(t, "test-123", newCtx.Value(ContextKeyRequestID))
}

func BenchmarkMiddlewareChain_ProcessRequest(b *testing.B) {
	chain := NewMiddlewareChain()
	for i := 0; i < 10; i++ {
		chain.Add(&mockRequestMiddleware{})
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = chain.ProcessRequest(ctx, req)
	}
}

func BenchmarkMiddlewareChain_ProcessResponse(b *testing.B) {
	chain := NewMiddlewareChain()
	for i := 0; i < 10; i++ {
		chain.Add(&mockResponseMiddleware{})
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{Header: make(http.Header)}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = chain.ProcessResponse(ctx, req, resp)
	}
}
