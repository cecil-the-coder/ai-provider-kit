package common

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewModelCache(t *testing.T) {
	ttl := 5 * time.Minute
	cache := NewModelCache(ttl)

	if cache == nil {
		t.Fatal("expected non-nil cache")
	}

	if cache.ttl != ttl {
		t.Errorf("got TTL %v, expected %v", cache.ttl, ttl)
	}

	if len(cache.models) != 0 {
		t.Errorf("expected empty models, got %d", len(cache.models))
	}
}

func TestModelCache_IsStale(t *testing.T) {
	cache := NewModelCache(1 * time.Second)

	// Initially stale (no timestamp)
	if !cache.IsStale() {
		t.Error("expected cache to be stale initially")
	}

	// Update cache
	models := []types.Model{
		{ID: "model1", Name: "Model 1"},
	}
	cache.Update(models)

	// Should not be stale immediately
	if cache.IsStale() {
		t.Error("expected cache to be fresh after update")
	}

	// Wait for TTL to expire
	time.Sleep(1100 * time.Millisecond)

	// Should be stale now
	if !cache.IsStale() {
		t.Error("expected cache to be stale after TTL")
	}
}

func TestModelCache_GetAndUpdate(t *testing.T) {
	cache := NewModelCache(5 * time.Minute)

	// Initially empty
	models := cache.Get()
	if len(models) != 0 {
		t.Errorf("expected empty cache, got %d models", len(models))
	}

	// Update with models
	testModels := []types.Model{
		{ID: "model1", Name: "Model 1"},
		{ID: "model2", Name: "Model 2"},
	}
	cache.Update(testModels)

	// Verify models are stored
	models = cache.Get()
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}

	if models[0].ID != "model1" || models[1].ID != "model2" {
		t.Error("models don't match expected values")
	}
}

func TestModelCache_GetModels(t *testing.T) {
	cache := NewModelCache(1 * time.Second)

	fetchCalled := 0
	fetchFunc := func() ([]types.Model, error) {
		fetchCalled++
		return []types.Model{
			{ID: "fetched1", Name: "Fetched 1"},
		}, nil
	}

	fallbackCalled := 0
	fallbackFunc := func() []types.Model {
		fallbackCalled++
		return []types.Model{
			{ID: "fallback1", Name: "Fallback 1"},
		}
	}

	// First call - cache is empty and stale, should fetch
	models, err := cache.GetModels(fetchFunc, fallbackFunc)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "fetched1" {
		t.Error("expected fetched models")
	}
	if fetchCalled != 1 {
		t.Errorf("expected fetch to be called once, called %d times", fetchCalled)
	}

	// Second call - cache is fresh, should return cached
	models, err = cache.GetModels(fetchFunc, fallbackFunc)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "fetched1" {
		t.Error("expected cached models")
	}
	if fetchCalled != 1 {
		t.Errorf("expected fetch not to be called again, called %d times", fetchCalled)
	}

	// Wait for cache to become stale
	time.Sleep(1100 * time.Millisecond)

	// Third call - cache is stale, should fetch again
	models, err = cache.GetModels(fetchFunc, fallbackFunc)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fetchCalled != 2 {
		t.Errorf("expected fetch to be called twice, called %d times", fetchCalled)
	}

	// Test error case with fallback (but stale cache exists, so it returns that)
	fetchErrorFunc := func() ([]types.Model, error) {
		return nil, errors.New("fetch error")
	}

	models, err = cache.GetModels(fetchErrorFunc, fallbackFunc)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Fallback is NOT called because stale cache exists and is returned
	// The stale cached data is "fetched1" from earlier
	if len(models) != 1 || models[0].ID != "fetched1" {
		t.Error("expected stale cached models")
	}
}

func TestModelCache_GetModels_ErrorHandling(t *testing.T) {
	cache := NewModelCache(5 * time.Minute)

	// Test with error and no fallback
	fetchErrorFunc := func() ([]types.Model, error) {
		return nil, errors.New("fetch failed")
	}

	models, err := cache.GetModels(fetchErrorFunc, nil)
	if err == nil {
		t.Error("expected error but got none")
	}
	if models != nil {
		t.Error("expected nil models on error")
	}

	// Update cache with some models
	cache.Update([]types.Model{{ID: "stale1", Name: "Stale 1"}})

	// Make cache stale
	cache.timestamp = time.Now().Add(-10 * time.Minute)

	// Fetch fails but stale cache exists
	models, err = cache.GetModels(fetchErrorFunc, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "stale1" {
		t.Error("expected stale cached models")
	}
}

func TestModelCache_Clear(t *testing.T) {
	cache := NewModelCache(5 * time.Minute)

	// Add models
	cache.Update([]types.Model{
		{ID: "model1", Name: "Model 1"},
	})

	// Verify models exist
	if len(cache.Get()) != 1 {
		t.Error("expected 1 model before clear")
	}

	// Clear cache
	cache.Clear()

	// Verify cache is empty
	models := cache.Get()
	if len(models) != 0 {
		t.Errorf("expected empty cache after clear, got %d models", len(models))
	}

	// Verify timestamp is reset
	if !cache.timestamp.IsZero() {
		t.Error("expected timestamp to be zero after clear")
	}
}

func TestModelCache_GetTimestamp(t *testing.T) {
	cache := NewModelCache(5 * time.Minute)

	// Initially zero timestamp
	ts := cache.GetTimestamp()
	if !ts.IsZero() {
		t.Error("expected zero timestamp initially")
	}

	// Update cache
	beforeUpdate := time.Now()
	cache.Update([]types.Model{{ID: "model1"}})
	afterUpdate := time.Now()

	// Get timestamp
	ts = cache.GetTimestamp()
	if ts.Before(beforeUpdate) || ts.After(afterUpdate) {
		t.Error("timestamp not in expected range")
	}
}

func TestModelCache_GetTTL(t *testing.T) {
	expectedTTL := 10 * time.Minute
	cache := NewModelCache(expectedTTL)

	ttl := cache.GetTTL()
	if ttl != expectedTTL {
		t.Errorf("got TTL %v, expected %v", ttl, expectedTTL)
	}
}

func TestModelCache_SetTTL(t *testing.T) {
	cache := NewModelCache(5 * time.Minute)

	newTTL := 15 * time.Minute
	cache.SetTTL(newTTL)

	ttl := cache.GetTTL()
	if ttl != newTTL {
		t.Errorf("got TTL %v, expected %v", ttl, newTTL)
	}
}

func TestModelCache_ThreadSafety(t *testing.T) {
	cache := NewModelCache(5 * time.Minute)

	// Concurrent reads and writes
	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent Updates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			models := []types.Model{
				{ID: "model" + string(rune(id)), Name: "Model"},
			}
			cache.Update(models)
		}(i)
	}

	// Concurrent Gets
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cache.Get()
		}()
	}

	// Concurrent IsStale checks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cache.IsStale()
		}()
	}

	// Concurrent GetTimestamp
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cache.GetTimestamp()
		}()
	}

	// Concurrent SetTTL
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(duration int) {
			defer wg.Done()
			cache.SetTTL(time.Duration(duration) * time.Minute)
		}(i)
	}

	wg.Wait()
	// If we get here without a race condition, test passes
}

func TestModelCache_GetModels_ConcurrentFetch(t *testing.T) {
	cache := NewModelCache(1 * time.Second)

	fetchCount := 0
	var mu sync.Mutex

	fetchFunc := func() ([]types.Model, error) {
		mu.Lock()
		fetchCount++
		mu.Unlock()
		time.Sleep(100 * time.Millisecond) // Simulate slow fetch
		return []types.Model{{ID: "model1"}}, nil
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Multiple goroutines trying to fetch simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.GetModels(fetchFunc, nil)
		}()
	}

	wg.Wait()

	// All goroutines should complete without errors
	// The exact number of fetches depends on timing but should work correctly
}
