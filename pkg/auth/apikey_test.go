package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewAPIKeyManager(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		config := &APIKeyConfig{
			Strategy: "round_robin",
		}

		manager, err := NewAPIKeyManager("test", keys, config)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if manager == nil {
			t.Error("Expected non-nil manager")
		}
	})

	t.Run("NoKeys", func(t *testing.T) {
		_, err := NewAPIKeyManager("test", []string{}, nil)
		if err == nil {
			t.Error("Expected error for no keys")
		}
	})

	t.Run("WithNilConfig", func(t *testing.T) {
		keys := []string{"key1"}
		manager, err := NewAPIKeyManager("test", keys, nil)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if manager.config == nil {
			t.Error("Expected default config")
		}
	})
}

func TestGetCurrentKey(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	config := &APIKeyConfig{
		Strategy: "round_robin",
	}

	manager, _ := NewAPIKeyManager("test", keys, config)

	key := manager.GetCurrentKey()
	if key == "" {
		t.Error("Expected non-empty key")
	}
}

func TestGetNextKey(t *testing.T) {
	t.Run("SingleKey", func(t *testing.T) {
		keys := []string{"only-key"}
		manager, _ := NewAPIKeyManager("test", keys, nil)

		key, err := manager.GetNextKey()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if key != "only-key" {
			t.Errorf("Expected 'only-key', got '%s'", key)
		}
	})

	t.Run("RoundRobin", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		config := &APIKeyConfig{
			Strategy: "round_robin",
		}
		manager, _ := NewAPIKeyManager("test", keys, config)

		// Get several keys and verify rotation
		seenKeys := make(map[string]bool)
		for i := 0; i < 5; i++ {
			key, err := manager.GetNextKey()
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			seenKeys[key] = true
		}

		if len(seenKeys) < 2 {
			t.Error("Expected multiple keys to be used in rotation")
		}
	})

	t.Run("RandomStrategy", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		config := &APIKeyConfig{
			Strategy: "random",
		}
		manager, _ := NewAPIKeyManager("test", keys, config)

		key, err := manager.GetNextKey()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if key == "" {
			t.Error("Expected non-empty key")
		}
	})

	t.Run("WeightedStrategy", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		config := &APIKeyConfig{
			Strategy: "weighted",
		}
		manager, _ := NewAPIKeyManager("test", keys, config)

		key, err := manager.GetNextKey()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if key == "" {
			t.Error("Expected non-empty key")
		}
	})
}

func TestReportSuccessFailure(t *testing.T) {
	keys := []string{"key1", "key2"}
	config := &APIKeyConfig{
		Health: HealthConfig{
			Enabled:          true,
			FailureThreshold: 3,
		},
	}
	manager, _ := NewAPIKeyManager("test", keys, config)

	t.Run("ReportSuccess", func(t *testing.T) {
		manager.ReportSuccess("key1")

		manager.mu.RLock()
		health := manager.keyHealth["key1"]
		manager.mu.RUnlock()

		if health.successCount != 1 {
			t.Errorf("Expected success count 1, got %d", health.successCount)
		}
		if health.failureCount != 0 {
			t.Errorf("Expected failure count 0, got %d", health.failureCount)
		}
		if !health.isHealthy {
			t.Error("Expected key to be healthy")
		}
	})

	t.Run("ReportFailure", func(t *testing.T) {
		manager.ReportFailure("key2", errors.New("test error"))

		manager.mu.RLock()
		health := manager.keyHealth["key2"]
		manager.mu.RUnlock()

		if health.failureCount != 1 {
			t.Errorf("Expected failure count 1, got %d", health.failureCount)
		}
	})

	t.Run("MultipleFailures", func(t *testing.T) {
		manager.ReportFailure("key1", errors.New("error 1"))
		manager.ReportFailure("key1", errors.New("error 2"))
		manager.ReportFailure("key1", errors.New("error 3"))

		manager.mu.RLock()
		health := manager.keyHealth["key1"]
		manager.mu.RUnlock()

		if health.failureCount != 3 {
			t.Errorf("Expected failure count 3, got %d", health.failureCount)
		}
		if health.isHealthy {
			t.Error("Expected key to be unhealthy after threshold")
		}
	})
}

func TestExecuteWithFailover(t *testing.T) {
	t.Run("SuccessFirstTry", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		config := &APIKeyConfig{
			Failover: FailoverConfig{
				Enabled:     true,
				MaxAttempts: 3,
			},
		}
		manager, _ := NewAPIKeyManager("test", keys, config)

		result, usage, err := manager.ExecuteWithFailover(context.Background(), func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
			return "success", &types.Usage{TotalTokens: 100}, nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result != "success" {
			t.Errorf("Expected 'success', got '%s'", result)
		}
		if usage == nil || usage.TotalTokens != 100 {
			t.Errorf("Expected usage with 100 tokens, got %v", usage)
		}
	})

	t.Run("FailoverToSecondKey", func(t *testing.T) {
		keys := []string{"key1", "key2"}
		config := &APIKeyConfig{
			Failover: FailoverConfig{
				Enabled:     true,
				MaxAttempts: 2,
			},
		}
		manager, _ := NewAPIKeyManager("test", keys, config)

		attempt := 0
		result, usage, err := manager.ExecuteWithFailover(context.Background(), func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
			attempt++
			if attempt == 1 {
				return "", nil, errors.New("first key failed")
			}
			return "success", &types.Usage{TotalTokens: 50}, nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result != "success" {
			t.Error("Expected success after failover")
		}
		if usage == nil || usage.TotalTokens != 50 {
			t.Errorf("Expected usage with 50 tokens, got %v", usage)
		}
	})

	t.Run("AllKeysFail", func(t *testing.T) {
		keys := []string{"key1", "key2"}
		config := &APIKeyConfig{
			Failover: FailoverConfig{
				Enabled:     true,
				MaxAttempts: 2,
			},
		}
		manager, _ := NewAPIKeyManager("test", keys, config)

		_, _, err := manager.ExecuteWithFailover(context.Background(), func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
			return "", nil, errors.New("key failed")
		})

		if err == nil {
			t.Error("Expected error when all keys fail")
		}
	})
}

func TestGetStatus(t *testing.T) {
	keys := []string{"key1", "key2"}
	config := &APIKeyConfig{
		Strategy: "round_robin",
	}
	manager, _ := NewAPIKeyManager("test", keys, config)

	manager.ReportSuccess("key1")
	manager.ReportFailure("key2", errors.New("test error"))

	status := manager.GetStatus()

	if status["provider"] != "test" {
		t.Error("Expected provider name in status")
	}
	if status["total_keys"] != 2 {
		t.Errorf("Expected 2 total keys, got %v", status["total_keys"])
	}
	if status["strategy"] != "round_robin" {
		t.Errorf("Expected round_robin strategy, got %v", status["strategy"])
	}
}

func TestGetKeys(t *testing.T) {
	keys := []string{"key1-long-key", "key2-long-key"}
	manager, _ := NewAPIKeyManager("test", keys, nil)

	retrievedKeys := manager.GetKeys()

	if len(retrievedKeys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(retrievedKeys))
	}

	// Verify keys match the original keys
	for i, key := range retrievedKeys {
		if key != keys[i] {
			t.Errorf("Expected key %s at index %d, got %s", keys[i], i, key)
		}
	}

	// Verify it returns a copy (modifying returned slice doesn't affect original)
	if len(retrievedKeys) > 0 {
		retrievedKeys[0] = "modified"
		if manager.keys[0] == "modified" {
			t.Error("GetKeys should return a copy, not the original slice")
		}
	}
}

func TestAddKey(t *testing.T) {
	keys := []string{"key1"}
	manager, _ := NewAPIKeyManager("test", keys, nil)

	t.Run("Success", func(t *testing.T) {
		err := manager.AddKey("key2")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if len(manager.keys) != 2 {
			t.Errorf("Expected 2 keys, got %d", len(manager.keys))
		}
	})

	t.Run("EmptyKey", func(t *testing.T) {
		err := manager.AddKey("")
		if err == nil {
			t.Error("Expected error for empty key")
		}
	})

	t.Run("DuplicateKey", func(t *testing.T) {
		err := manager.AddKey("key1")
		if err == nil {
			t.Error("Expected error for duplicate key")
		}
	})
}

func TestRemoveKey(t *testing.T) {
	keys := []string{"key1", "key2"}
	manager, _ := NewAPIKeyManager("test", keys, nil)

	t.Run("Success", func(t *testing.T) {
		err := manager.RemoveKey("key1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if len(manager.keys) != 1 {
			t.Errorf("Expected 1 key, got %d", len(manager.keys))
		}
	})

	t.Run("NonExistentKey", func(t *testing.T) {
		err := manager.RemoveKey("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent key")
		}
	})
}

func TestIsHealthy(t *testing.T) {
	keys := []string{"key1", "key2"}
	config := &APIKeyConfig{
		Health: HealthConfig{
			Enabled:          true,
			FailureThreshold: 2,
		},
	}
	manager, _ := NewAPIKeyManager("test", keys, config)

	t.Run("AllHealthy", func(t *testing.T) {
		if !manager.IsHealthy() {
			t.Error("Expected manager to be healthy")
		}
	})

	t.Run("OneUnhealthy", func(t *testing.T) {
		manager.ReportFailure("key1", errors.New("error 1"))
		manager.ReportFailure("key1", errors.New("error 2"))

		if !manager.IsHealthy() {
			t.Error("Expected manager to still be healthy with one unhealthy key")
		}
	})

	t.Run("AllUnhealthy", func(t *testing.T) {
		manager.ReportFailure("key2", errors.New("error 1"))
		manager.ReportFailure("key2", errors.New("error 2"))

		if manager.IsHealthy() {
			t.Error("Expected manager to be unhealthy")
		}
	})
}

func TestCalculateBackoff(t *testing.T) {
	config := &APIKeyConfig{
		Health: HealthConfig{
			Enabled: true,
			Backoff: BackoffConfig{
				Initial:    1 * time.Second,
				Maximum:    60 * time.Second,
				Multiplier: 2.0,
				Jitter:     false,
			},
		},
	}
	manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)

	t.Run("FirstFailure", func(t *testing.T) {
		backoff := manager.calculateBackoff(1)
		if backoff <= 0 {
			t.Error("Expected positive backoff")
		}
	})

	t.Run("MultipleFailures", func(t *testing.T) {
		backoff1 := manager.calculateBackoff(1)
		backoff2 := manager.calculateBackoff(2)
		backoff3 := manager.calculateBackoff(3)

		if backoff2 <= backoff1 {
			t.Error("Expected increasing backoff")
		}
		if backoff3 <= backoff2 {
			t.Error("Expected increasing backoff")
		}
	})

	t.Run("MaximumBackoff", func(t *testing.T) {
		backoff := manager.calculateBackoff(100)
		if backoff > 60*time.Second {
			t.Error("Expected backoff to be capped at maximum")
		}
	})

	t.Run("WithJitter", func(t *testing.T) {
		config2 := &APIKeyConfig{
			Health: HealthConfig{
				Enabled: true,
				Backoff: BackoffConfig{
					Initial:    1 * time.Second,
					Maximum:    60 * time.Second,
					Multiplier: 2.0,
					Jitter:     true,
				},
			},
		}
		manager2, _ := NewAPIKeyManager("test", []string{"key1"}, config2)

		backoff := manager2.calculateBackoff(1)
		if backoff <= 0 {
			t.Error("Expected positive backoff with jitter")
		}
	})
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("NewCircuitBreaker", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled:             true,
			FailureThreshold:    3,
			RecoveryTimeout:     30 * time.Second,
			HalfOpenMaxRequests: 2,
		}
		cb := newCircuitBreaker(config)

		if cb.state != circuitClosed {
			t.Error("Expected circuit to be closed initially")
		}
	})

	t.Run("NewCircuitBreakerNilConfig", func(t *testing.T) {
		cb := newCircuitBreaker(nil)
		if cb == nil {
			t.Fatal("Expected non-nil circuit breaker")
		}
		if cb.config == nil {
			t.Error("Expected default config")
		}
	})

	t.Run("RecordFailure", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 2,
		}
		cb := newCircuitBreaker(config)

		cb.recordFailure()
		if cb.failures != 1 {
			t.Errorf("Expected 1 failure, got %d", cb.failures)
		}

		cb.recordFailure()
		if cb.state != circuitOpen {
			t.Error("Expected circuit to open after threshold")
		}
	})

	t.Run("RecordSuccess", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled: true,
		}
		cb := newCircuitBreaker(config)

		cb.recordFailure()
		cb.recordSuccess()

		if cb.state != circuitClosed {
			t.Error("Expected circuit to close after success")
		}
		if cb.failures != 0 {
			t.Error("Expected failures to be reset")
		}
	})

	t.Run("HalfOpenSuccess", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled:             true,
			FailureThreshold:    2,
			HalfOpenMaxRequests: 2,
		}
		cb := newCircuitBreaker(config)

		// Trigger circuit open
		cb.recordFailure()
		cb.recordFailure()

		// Manually set to half-open for testing
		cb.mu.Lock()
		cb.state = circuitHalfOpen
		cb.requests = 0
		cb.mu.Unlock()

		// Record successes to close circuit
		cb.recordSuccess()
		cb.recordSuccess()

		if cb.state != circuitClosed {
			t.Error("Expected circuit to close after half-open successes")
		}
	})

	t.Run("IsOpen", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 2,
			RecoveryTimeout:  100 * time.Millisecond,
		}
		cb := newCircuitBreaker(config)

		// Trigger circuit open
		cb.recordFailure()
		cb.recordFailure()

		if !cb.isOpen() {
			t.Error("Expected circuit to be open")
		}

		// Wait for recovery timeout
		time.Sleep(150 * time.Millisecond)

		if cb.isOpen() {
			t.Error("Expected circuit to transition to half-open after timeout")
		}
	})

	t.Run("GetState", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled: true,
		}
		cb := newCircuitBreaker(config)

		if cb.getState() != "closed" {
			t.Error("Expected closed state")
		}

		// Set to open
		cb.mu.Lock()
		cb.state = circuitOpen
		cb.mu.Unlock()

		if cb.getState() != "open" {
			t.Error("Expected open state")
		}

		// Set to half-open
		cb.mu.Lock()
		cb.state = circuitHalfOpen
		cb.mu.Unlock()

		if cb.getState() != "half_open" {
			t.Error("Expected half_open state")
		}
	})

	t.Run("DisabledCircuitBreaker", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Enabled: false,
		}
		cb := newCircuitBreaker(config)

		if cb.getState() != "disabled" {
			t.Error("Expected disabled state")
		}

		// Recording failures and successes should do nothing
		cb.recordFailure()
		cb.recordSuccess()

		if cb.isOpen() {
			t.Error("Expected circuit to remain closed when disabled")
		}
	})
}

func TestMaskAPIKey(t *testing.T) {
	t.Run("LongKey", func(t *testing.T) {
		key := "sk-1234567890abcdefghijklmnop"
		masked := maskAPIKey(key)

		if masked == key {
			t.Error("Expected key to be masked")
		}
		if len(masked) == 0 {
			t.Error("Expected non-empty masked key")
		}
	})

	t.Run("ShortKey", func(t *testing.T) {
		key := "short"
		masked := maskAPIKey(key)

		if masked != "***" {
			t.Errorf("Expected '***' for short key, got '%s'", masked)
		}
	})
}

func TestMinFunction(t *testing.T) {
	t.Run("AIsSmaller", func(t *testing.T) {
		result := min(5, 10)
		if result != 5 {
			t.Errorf("Expected 5, got %d", result)
		}
	})

	t.Run("BIsSmaller", func(t *testing.T) {
		result := min(10, 5)
		if result != 5 {
			t.Errorf("Expected 5, got %d", result)
		}
	})

	t.Run("Equal", func(t *testing.T) {
		result := min(5, 5)
		if result != 5 {
			t.Errorf("Expected 5, got %d", result)
		}
	})
}

func TestCalculateKeyWeight(t *testing.T) {
	config := &APIKeyConfig{
		Strategy: "weighted",
	}
	manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)

	t.Run("HealthyKeyWithSuccesses", func(t *testing.T) {
		health := &keyHealth{
			isHealthy:    true,
			successCount: 5,
			failureCount: 0,
		}
		weight := manager.calculateKeyWeight(health)
		if weight <= 10 {
			t.Error("Expected weight to be increased by successes")
		}
	})

	t.Run("KeyWithFailures", func(t *testing.T) {
		health := &keyHealth{
			isHealthy:    true,
			successCount: 0,
			failureCount: 3,
		}
		weight := manager.calculateKeyWeight(health)
		if weight >= 10 {
			t.Error("Expected weight to be decreased by failures")
		}
	})

	t.Run("UnhealthyKey", func(t *testing.T) {
		health := &keyHealth{
			isHealthy: false,
		}
		weight := manager.calculateKeyWeight(health)
		if weight != 0 {
			t.Errorf("Expected 0 weight for unhealthy key, got %d", weight)
		}
	})

	t.Run("MinimumWeight", func(t *testing.T) {
		health := &keyHealth{
			isHealthy:    true,
			failureCount: 100,
		}
		weight := manager.calculateKeyWeight(health)
		if weight < 1 {
			t.Error("Expected minimum weight to be 1")
		}
	})
}

func TestIsKeyAvailable(t *testing.T) {
	config := &APIKeyConfig{
		Health: HealthConfig{
			Enabled: true,
		},
	}
	manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)

	t.Run("AvailableKey", func(t *testing.T) {
		health := &keyHealth{
			isHealthy:      true,
			backoffUntil:   time.Time{},
			circuitBreaker: newCircuitBreaker(&CircuitBreakerConfig{Enabled: false}),
		}
		if !manager.isKeyAvailable("key1", health) {
			t.Error("Expected key to be available")
		}
	})

	t.Run("KeyInBackoff", func(t *testing.T) {
		health := &keyHealth{
			isHealthy:      true,
			backoffUntil:   time.Now().Add(1 * time.Minute),
			circuitBreaker: newCircuitBreaker(&CircuitBreakerConfig{Enabled: false}),
		}
		if manager.isKeyAvailable("key1", health) {
			t.Error("Expected key to be unavailable in backoff")
		}
	})

	t.Run("CircuitBreakerOpen", func(t *testing.T) {
		cb := newCircuitBreaker(&CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 1,
			RecoveryTimeout:  10 * time.Minute, // Long timeout to keep circuit open
		})
		cb.recordFailure()
		// Force circuit to stay open by ensuring it's not past recovery timeout
		cb.mu.Lock()
		cb.lastFailure = time.Now()
		cb.mu.Unlock()

		health := &keyHealth{
			isHealthy:      true,
			backoffUntil:   time.Time{},
			circuitBreaker: cb,
		}
		if manager.isKeyAvailable("key1", health) {
			t.Error("Expected key to be unavailable with open circuit")
		}
	})

	t.Run("NilHealth", func(t *testing.T) {
		if !manager.isKeyAvailable("key1", nil) {
			t.Error("Expected key to be available with nil health")
		}
	})
}

func TestAPIKeyAuthenticator(t *testing.T) {
	config := &APIKeyConfig{
		Strategy: "round_robin",
	}

	t.Run("Authenticate", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
		}

		authConfig := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: "test-key",
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !auth.isAuth {
			t.Error("Expected to be authenticated")
		}
	})

	t.Run("WrongMethod", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
		}

		authConfig := types.AuthConfig{
			Method: types.AuthMethodOAuth,
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err == nil {
			t.Error("Expected error for wrong method")
		}
	})

	t.Run("EmptyAPIKey", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
		}

		authConfig := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: "",
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err == nil {
			t.Error("Expected error for empty API key")
		}
	})

	t.Run("IsAuthenticated", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
			isAuth:   true,
		}

		if !auth.IsAuthenticated() {
			t.Error("Expected to be authenticated")
		}
	})

	t.Run("GetToken", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
			isAuth:   true,
		}

		token, err := auth.GetToken()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if token == "" {
			t.Error("Expected non-empty token")
		}
	})

	t.Run("RefreshToken", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
		}

		ctx := context.Background()
		err := auth.RefreshToken(ctx)
		if err != nil {
			t.Errorf("Expected no error (no-op), got: %v", err)
		}
	})

	t.Run("Logout", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
			isAuth:   true,
		}

		ctx := context.Background()
		err := auth.Logout(ctx)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if auth.isAuth {
			t.Error("Expected to be logged out")
		}
	})

	t.Run("GetAuthMethod", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
		}

		if auth.GetAuthMethod() != types.AuthMethodAPIKey {
			t.Error("Expected API key auth method")
		}
	})

	t.Run("GetKeyManager", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
		}

		if auth.GetKeyManager() != manager {
			t.Error("Expected same manager instance")
		}
	})

	t.Run("RotateKey", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1", "key2"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
		}

		err := auth.RotateKey()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("ReportKeySuccess", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
		}

		err := auth.ReportKeySuccess("key1")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("ReportKeyFailure", func(t *testing.T) {
		manager, _ := NewAPIKeyManager("test", []string{"key1"}, config)
		auth := &APIKeyAuthenticatorImpl{
			provider: "test",
			manager:  manager,
			config:   config,
		}

		err := auth.ReportKeyFailure("key1", errors.New("test error"))
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})
}
