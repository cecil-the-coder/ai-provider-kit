// Package keymanager provides API key management with load balancing and failover capabilities.
// It includes health tracking, circuit breakers, and intelligent key rotation for AI providers.
package keymanager

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// KeyManager manages multiple API keys with load balancing and failover
type KeyManager struct {
	providerName string
	keys         []string
	currentIndex uint32 // Atomic counter for round-robin
	keyHealth    map[string]*keyHealth
	mu           sync.RWMutex
}

// keyHealth tracks the health status of an individual API key
type keyHealth struct {
	failureCount int
	lastFailure  time.Time
	lastSuccess  time.Time
	isHealthy    bool
	backoffUntil time.Time
}

// NewKeyManager creates a new multi-key manager
func NewKeyManager(providerName string, keys []string) *KeyManager {
	if len(keys) == 0 {
		return nil
	}

	manager := &KeyManager{
		providerName: providerName,
		keys:         keys,
		currentIndex: 0,
		keyHealth:    make(map[string]*keyHealth),
	}

	// Initialize health tracking for all keys
	for _, key := range keys {
		manager.keyHealth[key] = &keyHealth{
			isHealthy:   true,
			lastSuccess: time.Now(),
		}
	}

	return manager
}

// ExecuteWithFailover attempts an operation with automatic failover to next key on failure
func (m *KeyManager) ExecuteWithFailover(ctx context.Context, operation func(context.Context, string) (string, *types.Usage, error)) (string, *types.Usage, error) {
	if len(m.keys) == 0 {
		return "", nil, fmt.Errorf("no API keys configured for %s", m.providerName)
	}

	var lastErr error
	attemptsLimit := min(len(m.keys), 3) // Try up to 3 keys or all keys, whichever is less

	for attempt := 0; attempt < attemptsLimit; attempt++ {
		// Get next available key
		key, err := m.GetNextKey()
		if err != nil {
			// All keys exhausted or unavailable
			if lastErr != nil {
				return "", nil, fmt.Errorf("%s: all API keys failed, last error: %w", m.providerName, lastErr)
			}
			return "", nil, err
		}

		// Execute the operation
		result, usage, err := operation(ctx, key)
		if err != nil {
			lastErr = err
			m.ReportFailure(key, err)
			continue
		}

		// Success!
		m.ReportSuccess(key)
		return result, usage, nil
	}

	// All attempts failed
	return "", nil, fmt.Errorf("%s: all %d failover attempts failed, last error: %w",
		m.providerName, attemptsLimit, lastErr)
}

// GetNextKey returns the next available API key using round-robin load balancing
func (m *KeyManager) GetNextKey() (string, error) {
	if len(m.keys) == 0 {
		return "", fmt.Errorf("no API keys configured for %s", m.providerName)
	}

	// Single key fast path
	if len(m.keys) == 1 {
		m.mu.RLock()
		health := m.keyHealth[m.keys[0]]
		available := m.isKeyAvailable(m.keys[0], health)
		m.mu.RUnlock()

		if available {
			return m.keys[0], nil
		}
		return "", fmt.Errorf("only API key for %s is unavailable (in backoff)", m.providerName)
	}

	// Try all keys in round-robin order
	if len(m.keys) == 0 {
		return "", fmt.Errorf("no API keys configured for %s", m.providerName)
	}
	keysLen := uint32(len(m.keys)) // #nosec G115 -- len() returns non-negative int, keys slice won't exceed uint32 max
	startIndex := atomic.AddUint32(&m.currentIndex, 1) % keysLen

	for i := 0; i < len(m.keys); i++ {
		index := (int(startIndex) + i) % len(m.keys)
		key := m.keys[index]

		m.mu.RLock()
		health := m.keyHealth[key]
		available := m.isKeyAvailable(key, health)
		m.mu.RUnlock()

		if available {
			return key, nil
		}
	}

	return "", fmt.Errorf("all %d API keys for %s are currently unavailable", len(m.keys), m.providerName)
}

// isKeyAvailable checks if a key is available (not in backoff)
func (m *KeyManager) isKeyAvailable(_ string, health *keyHealth) bool {
	if health == nil {
		return true
	}

	// Check if key is in backoff period
	if time.Now().Before(health.backoffUntil) {
		return false
	}

	return true
}

// ReportSuccess reports that an API call succeeded with this key
func (m *KeyManager) ReportSuccess(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.keyHealth[key]
	if !exists {
		return
	}

	health.lastSuccess = time.Now()
	health.failureCount = 0
	health.isHealthy = true
	health.backoffUntil = time.Time{} // Clear backoff
}

// ReportFailure reports that an API call failed with this key
func (m *KeyManager) ReportFailure(key string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.keyHealth[key]
	if !exists {
		return
	}

	health.lastFailure = time.Now()
	health.failureCount++

	// Exponential backoff: 1s, 2s, 4s, 8s, max 60s
	failureCount := health.failureCount - 1
	if failureCount < 0 {
		failureCount = 0
	}
	if failureCount > 6 {
		failureCount = 6
	}
	backoffSeconds := 1 << uint(failureCount) // #nosec G115 -- failureCount is capped at 6, safe conversion
	if backoffSeconds > 60 {
		backoffSeconds = 60
	}
	health.backoffUntil = time.Now().Add(time.Duration(backoffSeconds) * time.Second)

	// Mark as unhealthy after 3 consecutive failures
	if health.failureCount >= 3 {
		health.isHealthy = false
	}
}

// GetKeys returns a copy of the keys slice
func (m *KeyManager) GetKeys() []string {
	if m == nil {
		return nil
	}
	return append([]string{}, m.keys...)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
