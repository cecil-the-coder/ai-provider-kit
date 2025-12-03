package common

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConnectivityCache(t *testing.T) {
	t.Run("WithCustomConfig", func(t *testing.T) {
		config := ConnectivityCacheConfig{
			TTL:     60 * time.Second,
			Enabled: true,
		}
		cache := NewConnectivityCache(config)
		assert.NotNil(t, cache)
		assert.Equal(t, config.TTL, cache.config.TTL)
		assert.Equal(t, config.Enabled, cache.config.Enabled)
		assert.NotNil(t, cache.cache)
	})

	t.Run("WithDefaultConfig", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()
		assert.NotNil(t, cache)
		assert.Equal(t, 30*time.Second, cache.config.TTL)
		assert.True(t, cache.config.Enabled)
	})
}

func TestDefaultConnectivityCacheConfig(t *testing.T) {
	config := DefaultConnectivityCacheConfig()
	assert.Equal(t, 30*time.Second, config.TTL)
	assert.True(t, config.Enabled)
}

func TestConnectivityCache_TestConnectivity(t *testing.T) {
	t.Run("CacheHit", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()
		callCount := 0

		testFunc := func(ctx context.Context) error {
			callCount++
			return nil
		}

		// First call should execute the test function
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Second call should hit the cache
		err = cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount, "Test function should not be called again on cache hit")
	})

	t.Run("CacheMiss", func(t *testing.T) {
		cache := NewConnectivityCache(ConnectivityCacheConfig{
			TTL:     1 * time.Millisecond, // Very short TTL for testing expiration
			Enabled: true,
		})
		callCount := 0

		testFunc := func(ctx context.Context) error {
			callCount++
			return nil
		}

		// First call
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Wait for cache to expire
		time.Sleep(5 * time.Millisecond)

		// Second call should miss the cache
		err = cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount, "Test function should be called again after cache expiration")
	})

	t.Run("CacheBypass", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()
		callCount := 0

		testFunc := func(ctx context.Context) error {
			callCount++
			return nil
		}

		// First call
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Second call with bypass=true should call the function again
		err = cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, true)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount, "Test function should be called when bypass is true")

		// Third call without bypass should use the updated cache
		err = cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount, "Test function should not be called after bypass updated cache")
	})

	t.Run("CacheDisabled", func(t *testing.T) {
		cache := NewConnectivityCache(ConnectivityCacheConfig{
			TTL:     30 * time.Second,
			Enabled: false,
		})
		callCount := 0

		testFunc := func(ctx context.Context) error {
			callCount++
			return nil
		}

		// First call
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Second call should also execute (caching disabled)
		err = cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount, "Test function should be called every time when caching is disabled")
	})

	t.Run("CacheErrorResult", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()
		callCount := 0
		testError := errors.New("connectivity test failed")

		testFunc := func(ctx context.Context) error {
			callCount++
			return testError
		}

		// First call should cache the error
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.Error(t, err)
		assert.Equal(t, testError, err)
		assert.Equal(t, 1, callCount)

		// Second call should return the cached error
		err = cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.Error(t, err)
		assert.Equal(t, testError, err)
		assert.Equal(t, 1, callCount, "Test function should not be called again on cache hit")
	})

	t.Run("DifferentProviders", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()
		callCounts := make(map[types.ProviderType]int)
		mu := sync.Mutex{}

		testFunc := func(providerType types.ProviderType) func(context.Context) error {
			return func(ctx context.Context) error {
				mu.Lock()
				callCounts[providerType]++
				mu.Unlock()
				return nil
			}
		}

		// Test multiple providers
		providers := []types.ProviderType{
			types.ProviderTypeOpenAI,
			types.ProviderTypeAnthropic,
			types.ProviderTypeCerebras,
		}

		for _, provider := range providers {
			err := cache.TestConnectivity(context.Background(), provider, testFunc(provider), false)
			assert.NoError(t, err)
		}

		// Verify each provider was called once
		for _, provider := range providers {
			assert.Equal(t, 1, callCounts[provider], "Each provider should be called once")
		}

		// Call again - should hit cache for all
		for _, provider := range providers {
			err := cache.TestConnectivity(context.Background(), provider, testFunc(provider), false)
			assert.NoError(t, err)
		}

		// Verify no additional calls
		for _, provider := range providers {
			assert.Equal(t, 1, callCounts[provider], "Cached results should be used")
		}
	})
}

func TestConnectivityCache_Clear(t *testing.T) {
	cache := NewDefaultConnectivityCache()
	callCount := 0

	testFunc := func(ctx context.Context) error {
		callCount++
		return nil
	}

	// First call
	err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Clear the cache
	cache.Clear(types.ProviderTypeOpenAI)

	// Second call should miss the cache
	err = cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount, "Test function should be called after cache clear")
}

func TestConnectivityCache_ClearAll(t *testing.T) {
	cache := NewDefaultConnectivityCache()
	callCounts := make(map[types.ProviderType]int)
	mu := sync.Mutex{}

	testFunc := func(providerType types.ProviderType) func(context.Context) error {
		return func(ctx context.Context) error {
			mu.Lock()
			callCounts[providerType]++
			mu.Unlock()
			return nil
		}
	}

	providers := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
	}

	// Cache results for multiple providers
	for _, provider := range providers {
		err := cache.TestConnectivity(context.Background(), provider, testFunc(provider), false)
		assert.NoError(t, err)
	}

	// Clear all
	cache.ClearAll()

	// All providers should miss the cache
	for _, provider := range providers {
		err := cache.TestConnectivity(context.Background(), provider, testFunc(provider), false)
		assert.NoError(t, err)
	}

	// Verify all were called twice
	for _, provider := range providers {
		assert.Equal(t, 2, callCounts[provider], "Each provider should be called again after ClearAll")
	}
}

func TestConnectivityCache_GetCachedResult(t *testing.T) {
	t.Run("WithValidCache", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()
		testFunc := func(ctx context.Context) error {
			return nil
		}

		// Cache a result
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)

		// Get cached result
		cachedErr, timestamp, found := cache.GetCachedResult(types.ProviderTypeOpenAI)
		assert.True(t, found)
		assert.NoError(t, cachedErr)
		assert.False(t, timestamp.IsZero())
		assert.True(t, time.Since(timestamp) < 1*time.Second)
	})

	t.Run("WithExpiredCache", func(t *testing.T) {
		cache := NewConnectivityCache(ConnectivityCacheConfig{
			TTL:     1 * time.Millisecond,
			Enabled: true,
		})
		testFunc := func(ctx context.Context) error {
			return nil
		}

		// Cache a result
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)

		// Wait for expiration
		time.Sleep(5 * time.Millisecond)

		// Get cached result - should not be found
		_, _, found := cache.GetCachedResult(types.ProviderTypeOpenAI)
		assert.False(t, found)
	})

	t.Run("WithNoCache", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()

		// Get cached result without caching anything
		_, _, found := cache.GetCachedResult(types.ProviderTypeOpenAI)
		assert.False(t, found)
	})

	t.Run("WithErrorResult", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()
		testError := errors.New("test error")
		testFunc := func(ctx context.Context) error {
			return testError
		}

		// Cache an error result
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.Error(t, err)

		// Get cached result
		cachedErr, timestamp, found := cache.GetCachedResult(types.ProviderTypeOpenAI)
		assert.True(t, found)
		assert.Error(t, cachedErr)
		assert.Equal(t, testError, cachedErr)
		assert.False(t, timestamp.IsZero())
	})
}

func TestConnectivityCache_GetSetConfig(t *testing.T) {
	cache := NewDefaultConnectivityCache()

	// Get initial config
	config := cache.GetConfig()
	assert.Equal(t, 30*time.Second, config.TTL)
	assert.True(t, config.Enabled)

	// Update config
	newConfig := ConnectivityCacheConfig{
		TTL:     60 * time.Second,
		Enabled: false,
	}
	cache.SetConfig(newConfig)

	// Verify config was updated
	config = cache.GetConfig()
	assert.Equal(t, 60*time.Second, config.TTL)
	assert.False(t, config.Enabled)
}

func TestConnectivityCache_GetStats(t *testing.T) {
	t.Run("EmptyCache", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()
		stats := cache.GetStats()
		assert.Equal(t, 0, stats.TotalEntries)
		assert.Equal(t, 0, stats.ValidEntries)
		assert.Equal(t, 0, stats.ExpiredEntries)
		assert.Equal(t, 0, stats.SuccessfulChecks)
		assert.Equal(t, 0, stats.FailedChecks)
	})

	t.Run("WithValidEntries", func(t *testing.T) {
		cache := NewDefaultConnectivityCache()

		// Add successful check
		successFunc := func(ctx context.Context) error { return nil }
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, successFunc, false)
		assert.NoError(t, err)

		// Add failed check
		failFunc := func(ctx context.Context) error { return errors.New("test error") }
		err = cache.TestConnectivity(context.Background(), types.ProviderTypeAnthropic, failFunc, false)
		assert.Error(t, err)

		stats := cache.GetStats()
		assert.Equal(t, 2, stats.TotalEntries)
		assert.Equal(t, 2, stats.ValidEntries)
		assert.Equal(t, 0, stats.ExpiredEntries)
		assert.Equal(t, 1, stats.SuccessfulChecks)
		assert.Equal(t, 1, stats.FailedChecks)
	})

	t.Run("WithExpiredEntries", func(t *testing.T) {
		cache := NewConnectivityCache(ConnectivityCacheConfig{
			TTL:     10 * time.Millisecond,
			Enabled: true,
		})

		// Add entries
		testFunc := func(ctx context.Context) error { return nil }
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		err = cache.TestConnectivity(context.Background(), types.ProviderTypeAnthropic, testFunc, false)
		assert.NoError(t, err)

		// Wait for partial expiration
		time.Sleep(15 * time.Millisecond)

		// Add a fresh entry with same TTL
		err = cache.TestConnectivity(context.Background(), types.ProviderTypeCerebras, testFunc, false)
		assert.NoError(t, err)

		stats := cache.GetStats()
		assert.Equal(t, 3, stats.TotalEntries)
		assert.Equal(t, 1, stats.ValidEntries, "Only one entry should be valid")
		assert.Equal(t, 2, stats.ExpiredEntries, "Two entries should be expired")
		assert.Equal(t, 1, stats.SuccessfulChecks)
		assert.Equal(t, 0, stats.FailedChecks)
	})
}

func TestConnectivityCache_CleanupExpired(t *testing.T) {
	cache := NewConnectivityCache(ConnectivityCacheConfig{
		TTL:     10 * time.Millisecond,
		Enabled: true,
	})

	// Add entries
	testFunc := func(ctx context.Context) error { return nil }
	err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
	assert.NoError(t, err)
	err = cache.TestConnectivity(context.Background(), types.ProviderTypeAnthropic, testFunc, false)
	assert.NoError(t, err)

	// Wait for expiration
	time.Sleep(15 * time.Millisecond)

	// Add a fresh entry with the same TTL
	err = cache.TestConnectivity(context.Background(), types.ProviderTypeCerebras, testFunc, false)
	assert.NoError(t, err)

	// Cleanup expired entries
	removed := cache.CleanupExpired()
	assert.Equal(t, 2, removed, "Two expired entries should be removed")

	stats := cache.GetStats()
	assert.Equal(t, 1, stats.TotalEntries, "Only one entry should remain")
	assert.Equal(t, 1, stats.ValidEntries)
	assert.Equal(t, 0, stats.ExpiredEntries)
}

func TestConnectivityCache_ThreadSafety(t *testing.T) {
	cache := NewDefaultConnectivityCache()
	callCounts := make(map[types.ProviderType]*int)
	mu := sync.Mutex{}

	providers := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeCerebras,
	}

	// Initialize call counts
	for _, provider := range providers {
		count := 0
		callCounts[provider] = &count
	}

	testFunc := func(providerType types.ProviderType) func(context.Context) error {
		return func(ctx context.Context) error {
			mu.Lock()
			(*callCounts[providerType])++
			mu.Unlock()
			return nil
		}
	}

	// Run two waves of concurrent tests to verify cache works across waves
	var wg sync.WaitGroup

	// First wave - will populate the cache
	for i := 0; i < 5; i++ {
		for _, provider := range providers {
			wg.Add(1)
			go func(p types.ProviderType) {
				defer wg.Done()
				err := cache.TestConnectivity(context.Background(), p, testFunc(p), false)
				assert.NoError(t, err)
			}(provider)
		}
	}
	wg.Wait()

	// Record first wave counts
	firstWaveCounts := make(map[types.ProviderType]int)
	mu.Lock()
	for _, provider := range providers {
		firstWaveCounts[provider] = *callCounts[provider]
	}
	mu.Unlock()

	// Second wave - should hit the cache
	for i := 0; i < 5; i++ {
		for _, provider := range providers {
			wg.Add(1)
			go func(p types.ProviderType) {
				defer wg.Done()
				err := cache.TestConnectivity(context.Background(), p, testFunc(p), false)
				assert.NoError(t, err)
			}(provider)
		}
	}
	wg.Wait()

	// Verify the cache prevented calls in the second wave
	mu.Lock()
	defer mu.Unlock()
	for _, provider := range providers {
		totalCalls := *callCounts[provider]
		firstCalls := firstWaveCounts[provider]

		// First wave may have had races, but should have at least 1 call
		assert.Greater(t, firstCalls, 0, "Provider %s should be called at least once in first wave", provider)

		// Second wave should have NO additional calls (all hits from cache)
		assert.Equal(t, firstCalls, totalCalls,
			"Provider %s should not have additional calls in second wave (cache hit)", provider)
	}
}

func TestConnectivityCache_ContextCancellation(t *testing.T) {
	cache := NewDefaultConnectivityCache()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	testFunc := func(ctx context.Context) error {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	err := cache.TestConnectivity(ctx, types.ProviderTypeOpenAI, testFunc, false)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestConnectivityCache_Integration(t *testing.T) {
	// Simulate a real-world scenario with multiple rapid health checks
	cache := NewConnectivityCache(ConnectivityCacheConfig{
		TTL:     100 * time.Millisecond,
		Enabled: true,
	})

	callCount := 0
	testFunc := func(ctx context.Context) error {
		callCount++
		// Simulate network delay
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	// Simulate rapid health checks (like what would happen in a load balancer)
	for i := 0; i < 10; i++ {
		err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
		assert.NoError(t, err)
		time.Sleep(5 * time.Millisecond)
	}

	// Should have only called the function once (all subsequent calls hit cache)
	assert.Equal(t, 1, callCount, "With 100ms TTL and 5ms between calls, only one actual check should occur")

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Next call should trigger a new check
	err := cache.TestConnectivity(context.Background(), types.ProviderTypeOpenAI, testFunc, false)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount, "After TTL expiration, a new check should occur")
}
