package common

import (
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ModelCache stores cached model list with timestamp and thread-safe access
type ModelCache struct {
	models    []types.Model
	timestamp time.Time
	ttl       time.Duration
	mutex     sync.RWMutex
}

// NewModelCache creates a new model cache with the specified TTL
func NewModelCache(ttl time.Duration) *ModelCache {
	return &ModelCache{
		models:    []types.Model{},
		timestamp: time.Time{},
		ttl:       ttl,
		mutex:     sync.RWMutex{},
	}
}

// IsStale checks if the cache is expired
func (mc *ModelCache) IsStale() bool {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	return time.Since(mc.timestamp) > mc.ttl
}

// Get returns cached models (thread-safe)
func (mc *ModelCache) Get() []types.Model {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	return mc.models
}

// Update updates the cache with new models (thread-safe)
func (mc *ModelCache) Update(models []types.Model) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.models = models
	mc.timestamp = time.Now()
}

// GetModels returns cached models if available and fresh, or calls the fetch function
// This is a convenience method that implements the common cache-check-fetch-update pattern
func (mc *ModelCache) GetModels(fetchFunc func() ([]types.Model, error), fallbackFunc func() []types.Model) ([]types.Model, error) {
	// 1. Check cache first
	if !mc.IsStale() {
		cached := mc.Get()
		if len(cached) > 0 {
			return cached, nil
		}
	}

	// 2. Fetch from API/function
	models, err := fetchFunc()
	if err != nil {
		// 3. Fallback to static list or stale cache
		stale := mc.Get()
		if len(stale) > 0 {
			return stale, nil
		}
		if fallbackFunc != nil {
			return fallbackFunc(), nil
		}
		return nil, err
	}

	// 4. Update cache and return fresh models
	mc.Update(models)
	return models, nil
}

// Clear empties the cache and resets the timestamp
func (mc *ModelCache) Clear() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.models = []types.Model{}
	mc.timestamp = time.Time{}
}

// GetTimestamp returns when the cache was last updated
func (mc *ModelCache) GetTimestamp() time.Time {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	return mc.timestamp
}

// GetTTL returns the cache's time-to-live duration
func (mc *ModelCache) GetTTL() time.Duration {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	return mc.ttl
}

// SetTTL updates the cache's time-to-live duration
func (mc *ModelCache) SetTTL(ttl time.Duration) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.ttl = ttl
}
