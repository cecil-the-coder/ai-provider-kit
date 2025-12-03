package common

import (
	"context"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ConnectivityCacheConfig holds configuration for the connectivity cache
type ConnectivityCacheConfig struct {
	// TTL is the time-to-live for cached connectivity results
	// After this duration, the cache entry will be considered stale and a fresh check will be performed
	TTL time.Duration

	// Enabled controls whether caching is enabled
	// When disabled, all TestConnectivity calls will perform actual connectivity checks
	Enabled bool
}

// DefaultConnectivityCacheConfig returns the default cache configuration
// Default TTL is 30 seconds to prevent hammering provider APIs during rapid health checks
func DefaultConnectivityCacheConfig() ConnectivityCacheConfig {
	return ConnectivityCacheConfig{
		TTL:     30 * time.Second,
		Enabled: true,
	}
}

// connectivityCacheEntry represents a single cached connectivity result
type connectivityCacheEntry struct {
	// error is the result of the connectivity check (nil for success, error for failure)
	error error

	// timestamp is when this result was cached
	timestamp time.Time
}

// ConnectivityCache provides thread-safe caching of connectivity test results
// It prevents hammering provider APIs during rapid health checks by caching results for a configurable TTL
type ConnectivityCache struct {
	config ConnectivityCacheConfig
	cache  map[types.ProviderType]*connectivityCacheEntry
	mu     sync.RWMutex
}

// NewConnectivityCache creates a new connectivity cache with the given configuration
func NewConnectivityCache(config ConnectivityCacheConfig) *ConnectivityCache {
	return &ConnectivityCache{
		config: config,
		cache:  make(map[types.ProviderType]*connectivityCacheEntry),
	}
}

// NewDefaultConnectivityCache creates a new connectivity cache with default configuration
func NewDefaultConnectivityCache() *ConnectivityCache {
	return NewConnectivityCache(DefaultConnectivityCacheConfig())
}

// TestConnectivity performs a cached connectivity test
// If a valid cached result exists (not expired), it returns the cached result
// Otherwise, it calls the actual test function and caches the result
//
// Parameters:
//   - ctx: Context for the connectivity test
//   - providerType: The type of provider being tested
//   - testFunc: The actual connectivity test function to call if cache miss or bypass
//   - bypassCache: If true, forces a fresh connectivity check and updates the cache
//
// Returns an error if the connectivity test fails, or nil if successful
func (cc *ConnectivityCache) TestConnectivity(
	ctx context.Context,
	providerType types.ProviderType,
	testFunc func(context.Context) error,
	bypassCache bool,
) error {
	// If caching is disabled or bypass is requested, always perform actual test
	if !cc.config.Enabled || bypassCache {
		err := testFunc(ctx)
		// Still update cache even when bypassing, so subsequent calls can benefit
		if cc.config.Enabled {
			cc.set(providerType, err)
		}
		return err
	}

	// Try to get from cache
	if cachedErr, found := cc.get(providerType); found {
		return cachedErr
	}

	// Cache miss - perform actual test
	err := testFunc(ctx)
	cc.set(providerType, err)
	return err
}

// get retrieves a cached result if it exists and is not expired
// Returns the cached error (nil for success, error for failure) and a boolean indicating if found
func (cc *ConnectivityCache) get(providerType types.ProviderType) (error, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	entry, exists := cc.cache[providerType]
	if !exists {
		return nil, false
	}

	// Check if the entry has expired
	if time.Since(entry.timestamp) > cc.config.TTL {
		return nil, false
	}

	return entry.error, true
}

// set stores a connectivity test result in the cache
func (cc *ConnectivityCache) set(providerType types.ProviderType, err error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cache[providerType] = &connectivityCacheEntry{
		error:     err,
		timestamp: time.Now(),
	}
}

// Clear removes a specific provider's cached result
// This is useful when you want to force a fresh check on the next call without using bypassCache
func (cc *ConnectivityCache) Clear(providerType types.ProviderType) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	delete(cc.cache, providerType)
}

// ClearAll removes all cached results
// This is useful for resetting the cache state
func (cc *ConnectivityCache) ClearAll() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cache = make(map[types.ProviderType]*connectivityCacheEntry)
}

// GetCachedResult returns the cached result for a provider without performing a test
// Returns the cached error, timestamp of the cache entry, and whether a valid (non-expired) entry exists
func (cc *ConnectivityCache) GetCachedResult(providerType types.ProviderType) (error, time.Time, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	entry, exists := cc.cache[providerType]
	if !exists {
		return nil, time.Time{}, false
	}

	// Check if the entry has expired
	if time.Since(entry.timestamp) > cc.config.TTL {
		return nil, time.Time{}, false
	}

	return entry.error, entry.timestamp, true
}

// GetConfig returns the current cache configuration
func (cc *ConnectivityCache) GetConfig() ConnectivityCacheConfig {
	return cc.config
}

// SetConfig updates the cache configuration
// Note: Changing the TTL doesn't invalidate existing cache entries
func (cc *ConnectivityCache) SetConfig(config ConnectivityCacheConfig) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.config = config
}

// Stats represents statistics about the connectivity cache
type ConnectivityCacheStats struct {
	// TotalEntries is the current number of cached entries
	TotalEntries int

	// ValidEntries is the number of non-expired cached entries
	ValidEntries int

	// ExpiredEntries is the number of expired cached entries
	ExpiredEntries int

	// SuccessfulChecks is the number of cached successful connectivity checks
	SuccessfulChecks int

	// FailedChecks is the number of cached failed connectivity checks
	FailedChecks int
}

// GetStats returns statistics about the cache
func (cc *ConnectivityCache) GetStats() ConnectivityCacheStats {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	stats := ConnectivityCacheStats{
		TotalEntries: len(cc.cache),
	}

	for _, entry := range cc.cache {
		if time.Since(entry.timestamp) > cc.config.TTL {
			stats.ExpiredEntries++
		} else {
			stats.ValidEntries++
			if entry.error == nil {
				stats.SuccessfulChecks++
			} else {
				stats.FailedChecks++
			}
		}
	}

	return stats
}

// CleanupExpired removes all expired entries from the cache
// This is useful for periodic cleanup to prevent memory growth
func (cc *ConnectivityCache) CleanupExpired() int {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	removed := 0
	for providerType, entry := range cc.cache {
		if time.Since(entry.timestamp) > cc.config.TTL {
			delete(cc.cache, providerType)
			removed++
		}
	}

	return removed
}
