package extensions

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Example Interceptors

// LoggingInterceptor logs before and after provider calls.
type LoggingInterceptor struct {
	logs []string
	mu   sync.Mutex
}

func NewLoggingInterceptor() *LoggingInterceptor {
	return &LoggingInterceptor{
		logs: make([]string, 0),
	}
}

func (l *LoggingInterceptor) Intercept(ctx context.Context, req *GenerateRequest, next ProviderFunc) (*GenerateResponse, error) {
	l.mu.Lock()
	l.logs = append(l.logs, fmt.Sprintf("before: prompt=%s", req.Prompt))
	l.mu.Unlock()

	resp, err := next(ctx, req)

	l.mu.Lock()
	if err != nil {
		l.logs = append(l.logs, fmt.Sprintf("error: %v", err))
	} else {
		l.logs = append(l.logs, fmt.Sprintf("after: content=%s", resp.Content))
	}
	l.mu.Unlock()

	return resp, err
}

func (l *LoggingInterceptor) GetLogs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]string, len(l.logs))
	copy(result, l.logs)
	return result
}

// TimeoutInterceptor enforces a timeout on provider calls.
type TimeoutInterceptor struct {
	timeout time.Duration
}

func NewTimeoutInterceptor(timeout time.Duration) *TimeoutInterceptor {
	return &TimeoutInterceptor{timeout: timeout}
}

func (t *TimeoutInterceptor) Intercept(ctx context.Context, req *GenerateRequest, next ProviderFunc) (*GenerateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	type result struct {
		resp *GenerateResponse
		err  error
	}

	resultChan := make(chan result, 1)

	go func() {
		resp, err := next(ctx, req)
		resultChan <- result{resp: resp, err: err}
	}()

	select {
	case res := <-resultChan:
		return res.resp, res.err
	case <-ctx.Done():
		return nil, fmt.Errorf("provider call timed out after %v", t.timeout)
	}
}

// CachingInterceptor caches responses based on request prompt.
type CachingInterceptor struct {
	cache map[string]*GenerateResponse
	mu    sync.RWMutex
}

func NewCachingInterceptor() *CachingInterceptor {
	return &CachingInterceptor{
		cache: make(map[string]*GenerateResponse),
	}
}

func (c *CachingInterceptor) Intercept(ctx context.Context, req *GenerateRequest, next ProviderFunc) (*GenerateResponse, error) {
	// Check cache
	c.mu.RLock()
	if cached, ok := c.cache[req.Prompt]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	// Call next and cache result
	resp, err := next(ctx, req)
	if err == nil {
		c.mu.Lock()
		c.cache[req.Prompt] = resp
		c.mu.Unlock()
	}

	return resp, err
}

// MetricsInterceptor tracks call counts and durations.
type MetricsInterceptor struct {
	callCount     int
	totalDuration time.Duration
	mu            sync.Mutex
}

func NewMetricsInterceptor() *MetricsInterceptor {
	return &MetricsInterceptor{}
}

func (m *MetricsInterceptor) Intercept(ctx context.Context, req *GenerateRequest, next ProviderFunc) (*GenerateResponse, error) {
	start := time.Now()
	resp, err := next(ctx, req)
	duration := time.Since(start)

	m.mu.Lock()
	m.callCount++
	m.totalDuration += duration
	m.mu.Unlock()

	return resp, err
}

func (m *MetricsInterceptor) GetStats() (count int, avgDuration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callCount == 0 {
		return 0, 0
	}
	return m.callCount, m.totalDuration / time.Duration(m.callCount)
}

// Mock provider function for testing
func mockProviderFunc(content string, err error) ProviderFunc {
	return func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
		if err != nil {
			return nil, err
		}
		return &GenerateResponse{
			Content:  content,
			Model:    "mock-model",
			Provider: "mock-provider",
		}, nil
	}
}

// Tests for InterceptorChain

func TestNewInterceptorChain(t *testing.T) {
	chain := NewInterceptorChain()
	assert.NotNil(t, chain)
	assert.NotNil(t, chain.interceptors)
	assert.Empty(t, chain.interceptors)
}

func TestInterceptorChain_Add(t *testing.T) {
	t.Run("add single interceptor", func(t *testing.T) {
		chain := NewInterceptorChain()
		logger := NewLoggingInterceptor()

		chain.Add(logger)

		chain.mu.RLock()
		assert.Len(t, chain.interceptors, 1)
		chain.mu.RUnlock()
	})

	t.Run("add multiple interceptors", func(t *testing.T) {
		chain := NewInterceptorChain()
		logger := NewLoggingInterceptor()
		metrics := NewMetricsInterceptor()

		chain.Add(logger)
		chain.Add(metrics)

		chain.mu.RLock()
		assert.Len(t, chain.interceptors, 2)
		chain.mu.RUnlock()
	})
}

func TestInterceptorChain_Execute(t *testing.T) {
	t.Run("execute with no interceptors", func(t *testing.T) {
		chain := NewInterceptorChain()
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		resp, err := chain.Execute(ctx, req, mockProviderFunc("response", nil))

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "response", resp.Content)
	})

	t.Run("execute with single interceptor", func(t *testing.T) {
		chain := NewInterceptorChain()
		logger := NewLoggingInterceptor()
		chain.Add(logger)

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		resp, err := chain.Execute(ctx, req, mockProviderFunc("response", nil))

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "response", resp.Content)

		logs := logger.GetLogs()
		assert.Len(t, logs, 2)
		assert.Contains(t, logs[0], "before: prompt=test")
		assert.Contains(t, logs[1], "after: content=response")
	})

	t.Run("execute with multiple interceptors", func(t *testing.T) {
		chain := NewInterceptorChain()
		logger := NewLoggingInterceptor()
		metrics := NewMetricsInterceptor()

		chain.Add(logger)
		chain.Add(metrics)

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		resp, err := chain.Execute(ctx, req, mockProviderFunc("response", nil))

		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify logger ran
		logs := logger.GetLogs()
		assert.Len(t, logs, 2)

		// Verify metrics ran
		count, _ := metrics.GetStats()
		assert.Equal(t, 1, count)
	})

	t.Run("execute with provider error", func(t *testing.T) {
		chain := NewInterceptorChain()
		logger := NewLoggingInterceptor()
		chain.Add(logger)

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}
		providerErr := errors.New("provider error")

		resp, err := chain.Execute(ctx, req, mockProviderFunc("", providerErr))

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, providerErr, err)

		logs := logger.GetLogs()
		assert.Len(t, logs, 2)
		assert.Contains(t, logs[0], "before: prompt=test")
		assert.Contains(t, logs[1], "error: provider error")
	})

	t.Run("interceptor execution order", func(t *testing.T) {
		chain := NewInterceptorChain()
		var order []string
		mu := sync.Mutex{}

		interceptor1 := &mockOrderInterceptor{name: "first", order: &order, mu: &mu}
		interceptor2 := &mockOrderInterceptor{name: "second", order: &order, mu: &mu}
		interceptor3 := &mockOrderInterceptor{name: "third", order: &order, mu: &mu}

		chain.Add(interceptor1)
		chain.Add(interceptor2)
		chain.Add(interceptor3)

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		_, _ = chain.Execute(ctx, req, mockProviderFunc("response", nil))

		assert.Equal(t, []string{
			"first-before",
			"second-before",
			"third-before",
			"third-after",
			"second-after",
			"first-after",
		}, order)
	})
}

// Helper interceptor to test execution order
type mockOrderInterceptor struct {
	name  string
	order *[]string
	mu    *sync.Mutex
}

func (m *mockOrderInterceptor) Intercept(ctx context.Context, req *GenerateRequest, next ProviderFunc) (*GenerateResponse, error) {
	m.mu.Lock()
	*m.order = append(*m.order, m.name+"-before")
	m.mu.Unlock()

	resp, err := next(ctx, req)

	m.mu.Lock()
	*m.order = append(*m.order, m.name+"-after")
	m.mu.Unlock()

	return resp, err
}

// Tests for LoggingInterceptor

func TestLoggingInterceptor(t *testing.T) {
	t.Run("logs successful call", func(t *testing.T) {
		logger := NewLoggingInterceptor()
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test prompt"}

		resp, err := logger.Intercept(ctx, req, mockProviderFunc("test response", nil))

		assert.NoError(t, err)
		assert.NotNil(t, resp)

		logs := logger.GetLogs()
		assert.Len(t, logs, 2)
		assert.Equal(t, "before: prompt=test prompt", logs[0])
		assert.Equal(t, "after: content=test response", logs[1])
	})

	t.Run("logs error call", func(t *testing.T) {
		logger := NewLoggingInterceptor()
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test prompt"}
		testErr := errors.New("test error")

		resp, err := logger.Intercept(ctx, req, mockProviderFunc("", testErr))

		assert.Error(t, err)
		assert.Nil(t, resp)

		logs := logger.GetLogs()
		assert.Len(t, logs, 2)
		assert.Equal(t, "before: prompt=test prompt", logs[0])
		assert.Equal(t, "error: test error", logs[1])
	})
}

// Tests for TimeoutInterceptor

func TestTimeoutInterceptor(t *testing.T) {
	t.Run("succeeds within timeout", func(t *testing.T) {
		timeout := NewTimeoutInterceptor(100 * time.Millisecond)
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		fastProvider := func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
			time.Sleep(10 * time.Millisecond)
			return &GenerateResponse{Content: "response"}, nil
		}

		resp, err := timeout.Intercept(ctx, req, fastProvider)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "response", resp.Content)
	})

	t.Run("times out on slow provider", func(t *testing.T) {
		timeout := NewTimeoutInterceptor(50 * time.Millisecond)
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		slowProvider := func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
			time.Sleep(200 * time.Millisecond)
			return &GenerateResponse{Content: "response"}, nil
		}

		resp, err := timeout.Intercept(ctx, req, slowProvider)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "timed out")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		timeout := NewTimeoutInterceptor(1 * time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		req := &GenerateRequest{Prompt: "test"}

		slowProvider := func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return &GenerateResponse{Content: "response"}, nil
			}
		}

		// Cancel context before timeout
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		resp, err := timeout.Intercept(ctx, req, slowProvider)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// Tests for CachingInterceptor

func TestCachingInterceptor(t *testing.T) {
	t.Run("caches successful response", func(t *testing.T) {
		cache := NewCachingInterceptor()
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		callCount := 0
		provider := func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
			callCount++
			return &GenerateResponse{Content: fmt.Sprintf("response-%d", callCount)}, nil
		}

		// First call - should hit provider
		resp1, err := cache.Intercept(ctx, req, provider)
		assert.NoError(t, err)
		assert.Equal(t, "response-1", resp1.Content)
		assert.Equal(t, 1, callCount)

		// Second call - should hit cache
		resp2, err := cache.Intercept(ctx, req, provider)
		assert.NoError(t, err)
		assert.Equal(t, "response-1", resp2.Content) // Same as first response
		assert.Equal(t, 1, callCount)                // Provider not called again
	})

	t.Run("does not cache errors", func(t *testing.T) {
		cache := NewCachingInterceptor()
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		callCount := 0
		provider := func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("error")
			}
			return &GenerateResponse{Content: "success"}, nil
		}

		// First call - error
		resp1, err := cache.Intercept(ctx, req, provider)
		assert.Error(t, err)
		assert.Nil(t, resp1)
		assert.Equal(t, 1, callCount)

		// Second call - should retry provider (no cached error)
		resp2, err := cache.Intercept(ctx, req, provider)
		assert.NoError(t, err)
		assert.Equal(t, "success", resp2.Content)
		assert.Equal(t, 2, callCount)
	})

	t.Run("different prompts have separate cache entries", func(t *testing.T) {
		cache := NewCachingInterceptor()
		ctx := context.Background()

		callCount := 0
		provider := func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
			callCount++
			return &GenerateResponse{Content: req.Prompt + "-response"}, nil
		}

		resp1, _ := cache.Intercept(ctx, &GenerateRequest{Prompt: "prompt1"}, provider)
		resp2, _ := cache.Intercept(ctx, &GenerateRequest{Prompt: "prompt2"}, provider)

		assert.Equal(t, "prompt1-response", resp1.Content)
		assert.Equal(t, "prompt2-response", resp2.Content)
		assert.Equal(t, 2, callCount)
	})
}

// Tests for MetricsInterceptor

func TestMetricsInterceptor(t *testing.T) {
	t.Run("tracks call count", func(t *testing.T) {
		metrics := NewMetricsInterceptor()
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		for i := 0; i < 5; i++ {
			_, _ = metrics.Intercept(ctx, req, mockProviderFunc("response", nil))
		}

		count, _ := metrics.GetStats()
		assert.Equal(t, 5, count)
	})

	t.Run("tracks duration", func(t *testing.T) {
		metrics := NewMetricsInterceptor()
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		provider := func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
			time.Sleep(10 * time.Millisecond)
			return &GenerateResponse{Content: "response"}, nil
		}

		_, _ = metrics.Intercept(ctx, req, provider)

		count, avgDuration := metrics.GetStats()
		assert.Equal(t, 1, count)
		assert.True(t, avgDuration >= 10*time.Millisecond)
	})

	t.Run("tracks stats even with errors", func(t *testing.T) {
		metrics := NewMetricsInterceptor()
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		_, _ = metrics.Intercept(ctx, req, mockProviderFunc("", errors.New("error")))

		count, _ := metrics.GetStats()
		assert.Equal(t, 1, count)
	})
}

// Tests for InterceptorRegistry

func TestNewInterceptorRegistry(t *testing.T) {
	registry := NewInterceptorRegistry()
	assert.NotNil(t, registry)

	list := registry.List()
	assert.Empty(t, list)
}

func TestInterceptorRegistry_Register(t *testing.T) {
	t.Run("register interceptor successfully", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		logger := NewLoggingInterceptor()

		err := registry.Register("logger", logger)
		assert.NoError(t, err)

		retrieved, ok := registry.Get("logger")
		assert.True(t, ok)
		assert.Equal(t, logger, retrieved)
	})

	t.Run("duplicate registration returns error", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		logger1 := NewLoggingInterceptor()
		logger2 := NewLoggingInterceptor()

		err := registry.Register("logger", logger1)
		assert.NoError(t, err)

		err = registry.Register("logger", logger2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("register multiple interceptors", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		logger := NewLoggingInterceptor()
		metrics := NewMetricsInterceptor()
		cache := NewCachingInterceptor()

		assert.NoError(t, registry.Register("logger", logger))
		assert.NoError(t, registry.Register("metrics", metrics))
		assert.NoError(t, registry.Register("cache", cache))

		list := registry.List()
		assert.Len(t, list, 3)
	})
}

func TestInterceptorRegistry_Get(t *testing.T) {
	t.Run("get existing interceptor", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		logger := NewLoggingInterceptor()
		_ = registry.Register("logger", logger)

		retrieved, ok := registry.Get("logger")
		assert.True(t, ok)
		assert.Equal(t, logger, retrieved)
	})

	t.Run("get non-existing interceptor returns false", func(t *testing.T) {
		registry := NewInterceptorRegistry()

		retrieved, ok := registry.Get("non-existent")
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})
}

func TestInterceptorRegistry_List(t *testing.T) {
	t.Run("list returns interceptor names in registration order", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		logger := NewLoggingInterceptor()
		metrics := NewMetricsInterceptor()
		cache := NewCachingInterceptor()

		_ = registry.Register("logger", logger)
		_ = registry.Register("metrics", metrics)
		_ = registry.Register("cache", cache)

		list := registry.List()
		assert.Len(t, list, 3)
		assert.Equal(t, "logger", list[0])
		assert.Equal(t, "metrics", list[1])
		assert.Equal(t, "cache", list[2])
	})

	t.Run("list returns empty slice for empty registry", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		list := registry.List()
		assert.Empty(t, list)
		assert.NotNil(t, list)
	})
}

func TestInterceptorRegistry_Unregister(t *testing.T) {
	t.Run("unregister existing interceptor", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		logger := NewLoggingInterceptor()
		_ = registry.Register("logger", logger)

		err := registry.Unregister("logger")
		assert.NoError(t, err)

		retrieved, ok := registry.Get("logger")
		assert.False(t, ok)
		assert.Nil(t, retrieved)

		list := registry.List()
		assert.Empty(t, list)
	})

	t.Run("unregister non-existing interceptor returns error", func(t *testing.T) {
		registry := NewInterceptorRegistry()

		err := registry.Unregister("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("unregister maintains order of remaining interceptors", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		logger := NewLoggingInterceptor()
		metrics := NewMetricsInterceptor()
		cache := NewCachingInterceptor()

		_ = registry.Register("logger", logger)
		_ = registry.Register("metrics", metrics)
		_ = registry.Register("cache", cache)

		_ = registry.Unregister("metrics")

		list := registry.List()
		assert.Len(t, list, 2)
		assert.Equal(t, "logger", list[0])
		assert.Equal(t, "cache", list[1])
	})
}

// Tests for concurrent access

func TestInterceptorChain_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent execution", func(t *testing.T) {
		chain := NewInterceptorChain()
		metrics := NewMetricsInterceptor()
		chain.Add(metrics)

		ctx := context.Background()
		var wg sync.WaitGroup

		numGoroutines := 50
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := &GenerateRequest{Prompt: "test"}
				_, _ = chain.Execute(ctx, req, mockProviderFunc("response", nil))
			}()
		}

		wg.Wait()

		count, _ := metrics.GetStats()
		assert.Equal(t, numGoroutines, count)
	})

	t.Run("concurrent add and execute", func(t *testing.T) {
		chain := NewInterceptorChain()
		ctx := context.Background()
		var wg sync.WaitGroup

		// Add interceptors concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				chain.Add(NewLoggingInterceptor())
			}()
		}

		// Execute concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := &GenerateRequest{Prompt: "test"}
				_, _ = chain.Execute(ctx, req, mockProviderFunc("response", nil))
			}()
		}

		wg.Wait()
	})
}

func TestInterceptorRegistry_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent register and get", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		var wg sync.WaitGroup

		numGoroutines := 50

		// Register interceptors concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				interceptor := NewLoggingInterceptor()
				_ = registry.Register(fmt.Sprintf("logger-%d", id), interceptor)
			}(i)
		}

		wg.Wait()

		// Verify all registered
		list := registry.List()
		assert.Len(t, list, numGoroutines)

		// Concurrent get operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				interceptor, ok := registry.Get(fmt.Sprintf("logger-%d", id))
				assert.True(t, ok)
				assert.NotNil(t, interceptor)
			}(i)
		}

		wg.Wait()
	})
}

// Integration tests

func TestInterceptorChain_Integration(t *testing.T) {
	t.Run("complete interceptor chain with all example interceptors", func(t *testing.T) {
		chain := NewInterceptorChain()
		logger := NewLoggingInterceptor()
		metrics := NewMetricsInterceptor()
		cache := NewCachingInterceptor()

		chain.Add(logger)
		chain.Add(metrics)
		chain.Add(cache)

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		// First call - should hit provider
		resp1, err := chain.Execute(ctx, req, mockProviderFunc("response", nil))
		require.NoError(t, err)
		assert.Equal(t, "response", resp1.Content)

		// Verify logger recorded the call
		logs := logger.GetLogs()
		assert.Len(t, logs, 2)

		// Verify metrics recorded the call
		count, _ := metrics.GetStats()
		assert.Equal(t, 1, count)

		// Second call - should hit cache (but metrics still tracks the chain execution)
		resp2, err := chain.Execute(ctx, req, mockProviderFunc("different", nil))
		require.NoError(t, err)
		assert.Equal(t, "response", resp2.Content) // Same as first

		// Metrics tracks both executions of the chain (even though cache short-circuits)
		count, _ = metrics.GetStats()
		assert.Equal(t, 2, count)
	})

	t.Run("interceptor short-circuiting with cache", func(t *testing.T) {
		chain := NewInterceptorChain()
		cache := NewCachingInterceptor()
		chain.Add(cache)

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		callCount := 0
		provider := func(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
			callCount++
			return &GenerateResponse{Content: "response"}, nil
		}

		// Multiple calls should only hit provider once
		for i := 0; i < 5; i++ {
			_, err := chain.Execute(ctx, req, provider)
			assert.NoError(t, err)
		}

		assert.Equal(t, 1, callCount, "provider should only be called once due to caching")
	})
}

func TestInterceptorRegistry_Integration(t *testing.T) {
	t.Run("build chain from registry", func(t *testing.T) {
		registry := NewInterceptorRegistry()
		logger := NewLoggingInterceptor()
		metrics := NewMetricsInterceptor()

		_ = registry.Register("logger", logger)
		_ = registry.Register("metrics", metrics)

		// Build a chain from registered interceptors
		chain := NewInterceptorChain()
		for _, name := range registry.List() {
			interceptor, ok := registry.Get(name)
			require.True(t, ok)
			chain.Add(interceptor)
		}

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		resp, err := chain.Execute(ctx, req, mockProviderFunc("response", nil))
		assert.NoError(t, err)
		assert.Equal(t, "response", resp.Content)

		// Verify both interceptors ran
		logs := logger.GetLogs()
		assert.Len(t, logs, 2)

		count, _ := metrics.GetStats()
		assert.Equal(t, 1, count)
	})
}
