package keymanager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestNewKeyManager tests the creation of a new KeyManager
func TestNewKeyManager(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		keys         []string
		expectNil    bool
	}{
		{
			name:         "Valid single key",
			providerName: "openai",
			keys:         []string{"key1"},
			expectNil:    false,
		},
		{
			name:         "Valid multiple keys",
			providerName: "anthropic",
			keys:         []string{"key1", "key2", "key3"},
			expectNil:    false,
		},
		{
			name:         "Empty keys slice",
			providerName: "openai",
			keys:         []string{},
			expectNil:    true,
		},
		{
			name:         "Nil keys slice",
			providerName: "openai",
			keys:         nil,
			expectNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km := NewKeyManager(tt.providerName, tt.keys)
			if tt.expectNil {
				if km != nil {
					t.Errorf("Expected nil KeyManager, got %v", km)
				}
				return
			}

			if km == nil {
				t.Fatal("Expected non-nil KeyManager")
			}

			if km.providerName != tt.providerName {
				t.Errorf("Expected provider name %s, got %s", tt.providerName, km.providerName)
			}

			if len(km.keys) != len(tt.keys) {
				t.Errorf("Expected %d keys, got %d", len(tt.keys), len(km.keys))
			}

			// Verify health tracking is initialized
			for _, key := range tt.keys {
				health, exists := km.keyHealth[key]
				if !exists {
					t.Errorf("Health tracking not initialized for key %s", key)
				}
				if !health.isHealthy {
					t.Errorf("Key %s should be healthy initially", key)
				}
			}
		})
	}
}

// TestGetKeys tests retrieving keys from the manager
func TestGetKeys(t *testing.T) {
	tests := []struct {
		name string
		keys []string
	}{
		{
			name: "Single key",
			keys: []string{"key1"},
		},
		{
			name: "Multiple keys",
			keys: []string{"key1", "key2", "key3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km := NewKeyManager("test", tt.keys)
			retrievedKeys := km.GetKeys()

			if len(retrievedKeys) != len(tt.keys) {
				t.Errorf("Expected %d keys, got %d", len(tt.keys), len(retrievedKeys))
			}

			for i, key := range tt.keys {
				if retrievedKeys[i] != key {
					t.Errorf("Expected key %s at index %d, got %s", key, i, retrievedKeys[i])
				}
			}

			// Verify it returns a copy (modifying returned slice doesn't affect original)
			if len(retrievedKeys) > 0 {
				retrievedKeys[0] = "modified"
				if km.keys[0] == "modified" {
					t.Error("GetKeys should return a copy, not the original slice")
				}
			}
		})
	}

	t.Run("Nil manager", func(t *testing.T) {
		var km *KeyManager
		keys := km.GetKeys()
		if keys != nil {
			t.Errorf("Expected nil for nil manager, got %v", keys)
		}
	})
}

// TestGetNextKey tests the round-robin key selection
func TestGetNextKey(t *testing.T) {
	t.Run("Single key", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1"})
		for i := 0; i < 5; i++ {
			key, err := km.GetNextKey()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if key != "key1" {
				t.Errorf("Expected key1, got %s", key)
			}
		}
	})

	t.Run("Multiple keys round-robin", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		km := NewKeyManager("test", keys)

		// Test round-robin distribution
		seenKeys := make(map[string]bool)
		for i := 0; i < len(keys)*2; i++ {
			key, err := km.GetNextKey()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			seenKeys[key] = true
		}

		// All keys should be seen
		if len(seenKeys) != len(keys) {
			t.Errorf("Expected all %d keys to be used, only saw %d", len(keys), len(seenKeys))
		}
	})

	t.Run("No keys configured", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1"})
		km.keys = []string{} // Manually clear keys to test error path

		_, err := km.GetNextKey()
		if err == nil {
			t.Error("Expected error for empty keys, got nil")
		}
		if !strings.Contains(err.Error(), "no API keys configured") {
			t.Errorf("Expected 'no API keys configured' error, got: %v", err)
		}
	})

	t.Run("All keys in backoff", func(t *testing.T) {
		keys := []string{"key1", "key2"}
		km := NewKeyManager("test", keys)

		// Put all keys in backoff
		futureTime := time.Now().Add(1 * time.Hour)
		for _, key := range keys {
			km.keyHealth[key].backoffUntil = futureTime
		}

		_, err := km.GetNextKey()
		if err == nil {
			t.Error("Expected error when all keys are in backoff")
		}
		if !strings.Contains(err.Error(), "currently unavailable") {
			t.Errorf("Expected 'currently unavailable' error, got: %v", err)
		}
	})

	t.Run("Single key in backoff", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1"})
		km.keyHealth["key1"].backoffUntil = time.Now().Add(1 * time.Hour)

		_, err := km.GetNextKey()
		if err == nil {
			t.Error("Expected error when only key is in backoff")
		}
		if !strings.Contains(err.Error(), "unavailable (in backoff)") {
			t.Errorf("Expected 'unavailable (in backoff)' error, got: %v", err)
		}
	})
}

// TestReportSuccess tests reporting successful API calls
func TestReportSuccess(t *testing.T) {
	km := NewKeyManager("test", []string{"key1", "key2"})

	// Simulate a failure first
	km.ReportFailure("key1", errors.New("test error"))
	health := km.keyHealth["key1"]
	if health.failureCount != 1 {
		t.Errorf("Expected failure count 1, got %d", health.failureCount)
	}

	// Report success
	km.ReportSuccess("key1")
	health = km.keyHealth["key1"]

	if health.failureCount != 0 {
		t.Errorf("Expected failure count 0 after success, got %d", health.failureCount)
	}
	if !health.isHealthy {
		t.Error("Key should be healthy after success")
	}
	if !health.backoffUntil.IsZero() {
		t.Error("Backoff should be cleared after success")
	}
	if health.lastSuccess.IsZero() {
		t.Error("lastSuccess should be set")
	}
}

// TestReportFailure tests reporting failed API calls
func TestReportFailure(t *testing.T) {
	t.Run("Single failure", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1"})
		km.ReportFailure("key1", errors.New("test error"))

		health := km.keyHealth["key1"]
		if health.failureCount != 1 {
			t.Errorf("Expected failure count 1, got %d", health.failureCount)
		}
		if !health.isHealthy {
			t.Error("Key should still be healthy after 1 failure (requires 3 to be unhealthy)")
		}
		if health.lastFailure.IsZero() {
			t.Error("lastFailure should be set")
		}
		if health.backoffUntil.IsZero() {
			t.Error("backoffUntil should be set")
		}
	})

	t.Run("Multiple failures mark unhealthy", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1"})

		// Report 3 failures
		for i := 0; i < 3; i++ {
			km.ReportFailure("key1", errors.New("test error"))
		}

		health := km.keyHealth["key1"]
		if health.failureCount != 3 {
			t.Errorf("Expected failure count 3, got %d", health.failureCount)
		}
		if health.isHealthy {
			t.Error("Key should be unhealthy after 3 failures")
		}
	})

	t.Run("Exponential backoff calculation", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1"})
		now := time.Now()

		// First failure: 1 second backoff
		km.ReportFailure("key1", errors.New("test error"))
		backoff1 := km.keyHealth["key1"].backoffUntil.Sub(now)
		if backoff1 < 1*time.Second || backoff1 > 2*time.Second {
			t.Errorf("First backoff should be ~1s, got %v", backoff1)
		}

		// Second failure: 2 second backoff
		time.Sleep(10 * time.Millisecond) // Small delay to ensure timestamp changes
		now = time.Now()
		km.ReportFailure("key1", errors.New("test error"))
		backoff2 := km.keyHealth["key1"].backoffUntil.Sub(now)
		if backoff2 < 2*time.Second || backoff2 > 3*time.Second {
			t.Errorf("Second backoff should be ~2s, got %v", backoff2)
		}
	})

	t.Run("Backoff maximum cap", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1"})

		// Report many failures to hit the cap
		for i := 0; i < 10; i++ {
			km.ReportFailure("key1", errors.New("test error"))
		}

		health := km.keyHealth["key1"]
		backoff := health.backoffUntil.Sub(time.Now())

		// Should be capped at 60 seconds
		if backoff > 61*time.Second {
			t.Errorf("Backoff should be capped at 60s, got %v", backoff)
		}
	})

	t.Run("Non-existent key", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1"})
		// Should not panic
		km.ReportFailure("nonexistent", errors.New("test error"))
	})
}

// TestIsKeyAvailable tests key availability checking
func TestIsKeyAvailable(t *testing.T) {
	km := NewKeyManager("test", []string{"key1"})

	t.Run("Nil health is available", func(t *testing.T) {
		available := km.isKeyAvailable("key1", nil)
		if !available {
			t.Error("Nil health should be considered available")
		}
	})

	t.Run("Healthy key is available", func(t *testing.T) {
		health := &keyHealth{
			isHealthy:    true,
			backoffUntil: time.Time{},
		}
		available := km.isKeyAvailable("key1", health)
		if !available {
			t.Error("Healthy key without backoff should be available")
		}
	})

	t.Run("Key in backoff is not available", func(t *testing.T) {
		health := &keyHealth{
			isHealthy:    true,
			backoffUntil: time.Now().Add(1 * time.Hour),
		}
		available := km.isKeyAvailable("key1", health)
		if available {
			t.Error("Key in backoff should not be available")
		}
	})

	t.Run("Key past backoff is available", func(t *testing.T) {
		health := &keyHealth{
			isHealthy:    true,
			backoffUntil: time.Now().Add(-1 * time.Hour), // Past backoff
		}
		available := km.isKeyAvailable("key1", health)
		if !available {
			t.Error("Key past backoff should be available")
		}
	})
}

// TestExecuteWithFailover tests the failover mechanism
func TestExecuteWithFailover(t *testing.T) {
	t.Run("Success on first try", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1", "key2"})
		callCount := 0

		operation := func(ctx context.Context, key string) (string, *types.Usage, error) {
			callCount++
			return "success", &types.Usage{TotalTokens: 100}, nil
		}

		result, usage, err := km.ExecuteWithFailover(context.Background(), operation)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != "success" {
			t.Errorf("Expected result 'success', got %s", result)
		}
		if usage == nil || usage.TotalTokens != 100 {
			t.Errorf("Expected usage with 100 tokens, got %v", usage)
		}
		if callCount != 1 {
			t.Errorf("Expected 1 call, got %d", callCount)
		}

		// Verify success was reported
		health := km.keyHealth["key1"]
		if health.failureCount != 0 {
			t.Errorf("Expected 0 failures after success, got %d", health.failureCount)
		}
	})

	t.Run("Failover to next key", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1", "key2", "key3"})

		callCount := 0
		var firstKeyTried string

		operation := func(ctx context.Context, key string) (string, *types.Usage, error) {
			callCount++
			// Fail the first key that gets tried
			if callCount == 1 {
				firstKeyTried = key
				return "", nil, errors.New("first key failed")
			}
			return "success with " + key, &types.Usage{TotalTokens: 50}, nil
		}

		result, usage, err := km.ExecuteWithFailover(context.Background(), operation)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !strings.Contains(result, "success") {
			t.Errorf("Expected success result, got %s", result)
		}
		if usage == nil || usage.TotalTokens != 50 {
			t.Errorf("Expected usage with 50 tokens, got %v", usage)
		}
		// Should have tried exactly 2 keys (one failed, one succeeded)
		if callCount != 2 {
			t.Errorf("Expected exactly 2 calls (one failure then success), got %d", callCount)
		}

		// Verify the first key that was tried has recorded the failure
		health := km.keyHealth[firstKeyTried]
		if health.failureCount != 1 {
			t.Errorf("Expected first key %s to have 1 failure, got %d", firstKeyTried, health.failureCount)
		}
	})

	t.Run("All keys fail", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1", "key2"})

		operation := func(ctx context.Context, key string) (string, *types.Usage, error) {
			return "", nil, errors.New("all keys fail")
		}

		result, usage, err := km.ExecuteWithFailover(context.Background(), operation)
		if err == nil {
			t.Error("Expected error when all keys fail")
		}
		if result != "" {
			t.Errorf("Expected empty result on failure, got %s", result)
		}
		if usage != nil {
			t.Errorf("Expected nil usage on failure, got %v", usage)
		}
		if !strings.Contains(err.Error(), "failover attempts failed") {
			t.Errorf("Expected failover error message, got: %v", err)
		}
	})

	t.Run("No keys configured", func(t *testing.T) {
		km := &KeyManager{
			providerName: "test",
			keys:         []string{},
			keyHealth:    make(map[string]*keyHealth),
		}

		operation := func(ctx context.Context, key string) (string, *types.Usage, error) {
			return "should not be called", nil, nil
		}

		_, _, err := km.ExecuteWithFailover(context.Background(), operation)
		if err == nil {
			t.Error("Expected error when no keys configured")
		}
		if !strings.Contains(err.Error(), "no API keys configured") {
			t.Errorf("Expected 'no API keys configured' error, got: %v", err)
		}
	})

	t.Run("Attempt limit with many keys", func(t *testing.T) {
		// Create manager with 5 keys
		km := NewKeyManager("test", []string{"key1", "key2", "key3", "key4", "key5"})
		callCount := 0

		operation := func(ctx context.Context, key string) (string, *types.Usage, error) {
			callCount++
			return "", nil, errors.New("fail")
		}

		_, _, err := km.ExecuteWithFailover(context.Background(), operation)
		if err == nil {
			t.Error("Expected error")
		}

		// Should try up to 3 keys (min of len(keys)=5 and 3)
		if callCount != 3 {
			t.Errorf("Expected 3 attempts (limit), got %d", callCount)
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		km := NewKeyManager("test", []string{"key1", "key2"})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		operation := func(ctx context.Context, key string) (string, *types.Usage, error) {
			select {
			case <-ctx.Done():
				return "", nil, ctx.Err()
			default:
				return "success", nil, nil
			}
		}

		_, _, err := km.ExecuteWithFailover(ctx, operation)
		if err == nil {
			t.Error("Expected context cancellation error")
		}
	})
}

// TestConcurrentAccess tests thread safety
func TestConcurrentAccess(t *testing.T) {
	km := NewKeyManager("test", []string{"key1", "key2", "key3"})
	concurrency := 100
	var wg sync.WaitGroup

	// Test concurrent GetNextKey
	t.Run("Concurrent GetNextKey", func(t *testing.T) {
		wg.Add(concurrency)
		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				_, _ = km.GetNextKey()
			}()
		}
		wg.Wait()
	})

	// Test concurrent ReportSuccess/Failure
	t.Run("Concurrent health reporting", func(t *testing.T) {
		wg.Add(concurrency * 2)
		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				defer wg.Done()
				if idx%2 == 0 {
					km.ReportSuccess("key1")
				} else {
					km.ReportFailure("key1", errors.New("test"))
				}
			}(i)

			go func(idx int) {
				defer wg.Done()
				if idx%2 == 0 {
					km.ReportSuccess("key2")
				} else {
					km.ReportFailure("key2", errors.New("test"))
				}
			}(i)
		}
		wg.Wait()
	})

	// Test concurrent ExecuteWithFailover
	t.Run("Concurrent ExecuteWithFailover", func(t *testing.T) {
		operation := func(ctx context.Context, key string) (string, *types.Usage, error) {
			time.Sleep(1 * time.Millisecond) // Simulate some work
			return "result", &types.Usage{TotalTokens: 10}, nil
		}

		wg.Add(concurrency)
		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				_, _, _ = km.ExecuteWithFailover(context.Background(), operation)
			}()
		}
		wg.Wait()
	})
}

// TestMinFunction tests the helper min function
func TestMinFunction(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 5, -1},
		{10, 10, 10},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("min(%d,%d)", tt.a, tt.b), func(t *testing.T) {
			result := min(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("min(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestKeyRotation tests that keys are properly rotated
func TestKeyRotation(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	km := NewKeyManager("test", keys)

	// Track which keys are used
	keyUsage := make(map[string]int)
	iterations := 30

	for i := 0; i < iterations; i++ {
		key, err := km.GetNextKey()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		keyUsage[key]++
	}

	// Each key should be used approximately equally (within reason)
	expectedUsage := iterations / len(keys)
	for key, count := range keyUsage {
		if count < expectedUsage-2 || count > expectedUsage+2 {
			t.Errorf("Key %s used %d times, expected ~%d (uneven distribution)", key, count, expectedUsage)
		}
	}
}

// TestPartialKeyAvailability tests scenario where some keys are unavailable
func TestPartialKeyAvailability(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	km := NewKeyManager("test", keys)

	// Put key2 in backoff
	km.keyHealth["key2"].backoffUntil = time.Now().Add(1 * time.Hour)

	// Track which keys are used
	keyUsage := make(map[string]int)
	iterations := 20

	for i := 0; i < iterations; i++ {
		key, err := km.GetNextKey()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		keyUsage[key]++
	}

	// key2 should never be used
	if count, exists := keyUsage["key2"]; exists && count > 0 {
		t.Errorf("key2 is in backoff but was used %d times", count)
	}

	// key1 and key3 should both be used
	if keyUsage["key1"] == 0 {
		t.Error("key1 should have been used")
	}
	if keyUsage["key3"] == 0 {
		t.Error("key3 should have been used")
	}
}

// TestHealthRecovery tests that keys can recover from failures
func TestHealthRecovery(t *testing.T) {
	km := NewKeyManager("test", []string{"key1"})

	// Report multiple failures
	for i := 0; i < 5; i++ {
		km.ReportFailure("key1", errors.New("test error"))
	}

	health := km.keyHealth["key1"]
	if health.isHealthy {
		t.Error("Key should be unhealthy after multiple failures")
	}
	if health.failureCount != 5 {
		t.Errorf("Expected 5 failures, got %d", health.failureCount)
	}

	// Report success - should recover
	km.ReportSuccess("key1")

	health = km.keyHealth["key1"]
	if !health.isHealthy {
		t.Error("Key should be healthy after successful call")
	}
	if health.failureCount != 0 {
		t.Errorf("Failure count should be reset to 0, got %d", health.failureCount)
	}
	if !health.backoffUntil.IsZero() {
		t.Error("Backoff should be cleared after success")
	}
}

// TestExecuteWithFailoverAllKeysUnavailable tests when all keys are unavailable during failover
func TestExecuteWithFailoverAllKeysUnavailable(t *testing.T) {
	km := NewKeyManager("test", []string{"key1", "key2"})

	// Put all keys in long backoff
	futureTime := time.Now().Add(1 * time.Hour)
	km.keyHealth["key1"].backoffUntil = futureTime
	km.keyHealth["key2"].backoffUntil = futureTime

	operation := func(ctx context.Context, key string) (string, *types.Usage, error) {
		return "should not be called", nil, nil
	}

	_, _, err := km.ExecuteWithFailover(context.Background(), operation)
	if err == nil {
		t.Error("Expected error when all keys are unavailable")
	}
	if !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("Expected 'unavailable' in error message, got: %v", err)
	}
}

// TestExecuteWithFailoverGetNextKeyErrorWithPriorFailures tests the edge case where
// GetNextKey fails after some operations have already failed
func TestExecuteWithFailoverGetNextKeyErrorWithPriorFailures(t *testing.T) {
	km := NewKeyManager("test", []string{"key1", "key2"})

	callCount := 0
	operation := func(ctx context.Context, key string) (string, *types.Usage, error) {
		callCount++
		// First call fails
		if callCount == 1 {
			// After this failure, put both keys in backoff so GetNextKey will fail
			km.keyHealth["key1"].backoffUntil = time.Now().Add(1 * time.Hour)
			km.keyHealth["key2"].backoffUntil = time.Now().Add(1 * time.Hour)
			return "", nil, errors.New("first operation failed")
		}
		return "success", nil, nil
	}

	_, _, err := km.ExecuteWithFailover(context.Background(), operation)
	if err == nil {
		t.Error("Expected error")
	}
	// Should get the "all API keys failed" error since lastErr is set
	if !strings.Contains(err.Error(), "all API keys failed") && !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("Expected 'all API keys failed' or 'unavailable' in error, got: %v", err)
	}
}

// TestReportSuccessNonExistentKey tests reporting success for a key that doesn't exist
func TestReportSuccessNonExistentKey(t *testing.T) {
	km := NewKeyManager("test", []string{"key1"})
	// Should not panic
	km.ReportSuccess("nonexistent-key")

	// Original key should be unaffected
	health := km.keyHealth["key1"]
	if health.failureCount != 0 {
		t.Error("Original key should not be affected by success report on non-existent key")
	}
}

// TestDuplicateKeyHandling tests behavior when duplicate keys are provided
func TestDuplicateKeyHandling(t *testing.T) {
	// The implementation doesn't prevent duplicates, so we test the actual behavior
	km := NewKeyManager("test", []string{"key1", "key1", "key2"})

	if len(km.keys) != 3 {
		t.Errorf("Expected 3 keys (including duplicates), got %d", len(km.keys))
	}

	// Both instances of key1 share the same health tracking
	km.ReportFailure("key1", errors.New("test"))
	health := km.keyHealth["key1"]
	if health.failureCount != 1 {
		t.Errorf("Expected 1 failure, got %d", health.failureCount)
	}
}
