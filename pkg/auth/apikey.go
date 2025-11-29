// Package auth provides authentication and authorization utilities for AI providers.
// It includes API key management, OAuth flows, security helpers, and credential storage.
package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	pkghttp "github.com/cecil-the-coder/ai-provider-kit/pkg/http"
)

// APIKeyManagerImpl implements APIKeyManager with load balancing and failover
type APIKeyManagerImpl struct {
	providerName string
	config       *APIKeyConfig
	keys         []string
	keyHealth    map[string]*keyHealth
	currentIndex uint32 // Atomic counter for round-robin
	mu           sync.RWMutex
}

// keyHealth tracks the health status of an individual API key
type keyHealth struct {
	failureCount   int
	successCount   int
	lastFailure    time.Time
	lastSuccess    time.Time
	isHealthy      bool
	backoffUntil   time.Time
	requestCount   int64
	errorRate      float64
	circuitBreaker *circuitBreaker
}

// circuitBreaker implements the circuit breaker pattern
type circuitBreaker struct {
	state       circuitState
	failures    int
	requests    int
	lastFailure time.Time
	lastReset   time.Time
	mu          sync.RWMutex
	config      *CircuitBreakerConfig
}

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager(providerName string, keys []string, config *APIKeyConfig) (*APIKeyManagerImpl, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one API key is required")
	}

	if config == nil {
		config = &APIKeyConfig{
			Strategy: "round_robin",
			Health: HealthConfig{
				Enabled:          true,
				FailureThreshold: 3,
				SuccessThreshold: 2,
				Backoff: BackoffConfig{
					Initial:    1 * time.Second,
					Maximum:    60 * time.Second,
					Multiplier: 2.0,
					Jitter:     true,
				},
				CheckInterval: 5 * time.Minute,
			},
			Failover: FailoverConfig{
				Enabled:     true,
				MaxAttempts: 3,
				Strategy:    "fail_fast",
			},
		}
	}

	manager := &APIKeyManagerImpl{
		providerName: providerName,
		config:       config,
		keys:         keys,
		currentIndex: 0,
		keyHealth:    make(map[string]*keyHealth),
	}

	// Initialize health tracking for all keys
	for _, key := range keys {
		circuitBreaker := newCircuitBreaker(&config.Failover.CircuitBreaker)
		manager.keyHealth[key] = &keyHealth{
			isHealthy:      true,
			lastSuccess:    time.Now(),
			circuitBreaker: circuitBreaker,
		}
	}

	return manager, nil
}

// newCircuitBreaker creates a new circuit breaker
func newCircuitBreaker(config *CircuitBreakerConfig) *circuitBreaker {
	if config == nil {
		config = &CircuitBreakerConfig{
			Enabled:             false,
			FailureThreshold:    5,
			RecoveryTimeout:     60 * time.Second,
			HalfOpenMaxRequests: 3,
		}
	}

	return &circuitBreaker{
		state:  circuitClosed,
		config: config,
	}
}

// GetCurrentKey returns the first available API key without advancing the round-robin counter
func (m *APIKeyManagerImpl) GetCurrentKey() string {
	if len(m.keys) == 0 {
		return ""
	}

	// Try to find the first available key
	for _, key := range m.keys {
		m.mu.RLock()
		health := m.keyHealth[key]
		m.mu.RUnlock()

		if m.isKeyAvailable(key, health) {
			return key
		}
	}

	// If all keys are in backoff, return the first key anyway
	return m.keys[0]
}

// GetNextKey returns the next available API key using round-robin load balancing
func (m *APIKeyManagerImpl) GetNextKey() (string, error) {
	if len(m.keys) == 0 {
		return "", fmt.Errorf("no API keys configured for %s", m.providerName)
	}

	// Single key fast path
	if len(m.keys) == 1 {
		m.mu.RLock()
		health := m.keyHealth[m.keys[0]]
		m.mu.RUnlock()

		if m.isKeyAvailable(m.keys[0], health) {
			return m.keys[0], nil
		}
		return "", fmt.Errorf("only API key for %s is unavailable (in backoff)", m.providerName)
	}

	// Try all keys based on strategy
	switch m.config.Strategy {
	case "round_robin":
		return m.getNextKeyRoundRobin()
	case "random":
		return m.getNextKeyRandom()
	case "weighted":
		return m.getNextKeyWeighted()
	default:
		return m.getNextKeyRoundRobin()
	}
}

// getNextKeyRoundRobin implements round-robin key selection
func (m *APIKeyManagerImpl) getNextKeyRoundRobin() (string, error) {
	keysLen := uint32(len(m.keys)) // #nosec G115 -- len() returns non-negative int, keys slice won't exceed uint32 max
	startIndex := atomic.AddUint32(&m.currentIndex, 1) % keysLen

	for i := 0; i < len(m.keys); i++ {
		index := (int(startIndex) + i) % len(m.keys)
		key := m.keys[index]

		m.mu.RLock()
		health := m.keyHealth[key]
		m.mu.RUnlock()

		if m.isKeyAvailable(key, health) {
			return key, nil
		}
	}

	return "", fmt.Errorf("all %d API keys for %s are currently unavailable", len(m.keys), m.providerName)
}

// getNextKeyRandom implements random key selection
func (m *APIKeyManagerImpl) getNextKeyRandom() (string, error) {
	availableKeys := make([]string, 0)

	m.mu.RLock()
	for _, key := range m.keys {
		if health := m.keyHealth[key]; m.isKeyAvailable(key, health) {
			availableKeys = append(availableKeys, key)
		}
	}
	m.mu.RUnlock()

	if len(availableKeys) == 0 {
		return "", fmt.Errorf("all API keys for %s are currently unavailable", m.providerName)
	}

	// Select random key from available ones
	if len(availableKeys) == 0 {
		return "", fmt.Errorf("no available keys for random selection")
	}

	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(availableKeys))))
	if err != nil {
		// Fallback to first key if crypto/rand fails
		return availableKeys[0], nil
	}
	selectedIndex := int(nBig.Int64())
	return availableKeys[selectedIndex], nil
}

// getNextKeyWeighted implements weighted key selection based on health
func (m *APIKeyManagerImpl) getNextKeyWeighted() (string, error) {
	var totalWeight int
	keyWeights := make(map[string]int)

	m.mu.RLock()
	for _, key := range m.keys {
		health := m.keyHealth[key]
		if m.isKeyAvailable(key, health) {
			weight := m.calculateKeyWeight(health)
			keyWeights[key] = weight
			totalWeight += weight
		}
	}
	m.mu.RUnlock()

	if totalWeight == 0 {
		return "", fmt.Errorf("all API keys for %s are currently unavailable", m.providerName)
	}

	// Select key based on weight
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(totalWeight)))
	if err != nil {
		// Fallback to first available key if crypto/rand fails
		for key := range keyWeights {
			return key, nil
		}
	}
	randomValue := int(nBig.Int64())
	currentWeight := 0
	for key, weight := range keyWeights {
		currentWeight += weight
		if randomValue < currentWeight {
			return key, nil
		}
	}

	// Fallback to first available key
	for key := range keyWeights {
		return key, nil
	}

	return "", fmt.Errorf("no available keys for %s", m.providerName)
}

// calculateKeyWeight calculates weight for a key based on its health
func (m *APIKeyManagerImpl) calculateKeyWeight(health *keyHealth) int {
	if !health.isHealthy {
		return 0
	}

	// Base weight
	weight := 10

	// Reduce weight based on failure count
	if health.failureCount > 0 {
		weight -= health.failureCount * 2
	}

	// Increase weight based on success count
	if health.successCount > 0 {
		weight += health.successCount
	}

	// Ensure minimum weight
	if weight < 1 {
		weight = 1
	}

	return weight
}

// isKeyAvailable checks if a key is available (not in backoff and circuit is closed)
func (m *APIKeyManagerImpl) isKeyAvailable(_ string, health *keyHealth) bool {
	if health == nil {
		return true
	}

	// Check if key is in backoff period
	if time.Now().Before(health.backoffUntil) {
		return false
	}

	// Check circuit breaker state
	if health.circuitBreaker != nil && health.circuitBreaker.isOpen() {
		return false
	}

	return true
}

// ReportSuccess reports that an API call succeeded with this key
func (m *APIKeyManagerImpl) ReportSuccess(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.keyHealth[key]
	if !exists {
		return
	}

	health.lastSuccess = time.Now()
	health.successCount++
	health.failureCount = 0 // Reset failure count on success
	health.isHealthy = true
	health.backoffUntil = time.Time{} // Clear backoff
	atomic.AddInt64(&health.requestCount, 1)

	// Update error rate
	if health.requestCount > 0 {
		health.errorRate = float64(health.failureCount) / float64(health.requestCount)
	}

	// Record success in circuit breaker
	if health.circuitBreaker != nil {
		health.circuitBreaker.recordSuccess()
	}
}

// ReportFailure reports that an API call failed with this key
func (m *APIKeyManagerImpl) ReportFailure(key string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.keyHealth[key]
	if !exists {
		return
	}

	health.lastFailure = time.Now()
	health.failureCount++
	atomic.AddInt64(&health.requestCount, 1)

	// Update error rate
	if health.requestCount > 0 {
		health.errorRate = float64(health.failureCount) / float64(health.requestCount)
	}

	// Calculate backoff duration
	backoffDuration := m.calculateBackoff(health.failureCount)
	health.backoffUntil = time.Now().Add(backoffDuration)

	// Mark as unhealthy after threshold
	if m.config.Health.Enabled && health.failureCount >= m.config.Health.FailureThreshold {
		health.isHealthy = false
	}

	// Record failure in circuit breaker
	if health.circuitBreaker != nil {
		health.circuitBreaker.recordFailure()
	}
}

// calculateBackoff calculates exponential backoff duration
// Uses the shared CalculateBackoff function from pkg/http
func (m *APIKeyManagerImpl) calculateBackoff(failureCount int) time.Duration {
	if !m.config.Health.Enabled || failureCount <= 0 {
		return 0
	}

	config := m.config.Health.Backoff

	// Use the shared backoff calculator
	backoffConfig := pkghttp.BackoffConfig{
		BaseDelay:   config.Initial,
		MaxDelay:    config.Maximum,
		Multiplier:  config.Multiplier,
		MaxAttempts: failureCount,
	}

	backoffDuration := pkghttp.CalculateBackoff(backoffConfig, failureCount)

	// Add jitter if enabled
	if config.Jitter {
		// Use crypto/rand for jitter calculation
		jitterBig, err := rand.Int(rand.Reader, big.NewInt(100))
		if err == nil {
			jitterPercent := float64(jitterBig.Int64()) / 100.0      // 0-1% jitter
			jitter := float64(backoffDuration) * jitterPercent * 0.1 // 10% of 0-1% = 0-0.1% jitter
			backoffDuration += time.Duration(jitter)
		}
	}

	return backoffDuration
}

// ExecuteWithFailover attempts an operation with automatic failover to next key on failure
func (m *APIKeyManagerImpl) ExecuteWithFailover(operation func(apiKey string) (string, error)) (string, error) {
	if len(m.keys) == 0 {
		return "", fmt.Errorf("no API keys configured for %s", m.providerName)
	}

	var lastErr error
	maxAttempts := m.config.Failover.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = min(len(m.keys), 3)
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Get next available key
		key, err := m.GetNextKey()
		if err != nil {
			// All keys exhausted or unavailable
			if lastErr != nil {
				return "", fmt.Errorf("%s: all API keys failed, last error: %w", m.providerName, lastErr)
			}
			return "", err
		}

		// Execute the operation
		result, err := operation(key)
		if err != nil {
			lastErr = err
			m.ReportFailure(key, err)
			continue
		}

		// Success!
		m.ReportSuccess(key)
		if attempt > 0 {
			// Log successful failover if needed
			_ = fmt.Sprintf("successful failover after %d attempts for provider %s", attempt, m.providerName)
		}
		return result, nil
	}

	// All attempts failed
	return "", fmt.Errorf("%s: all %d failover attempts failed, last error: %w",
		m.providerName, maxAttempts, lastErr)
}

// GetStatus returns the current health status of all keys
func (m *APIKeyManagerImpl) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]interface{})
	status["provider"] = m.providerName
	status["total_keys"] = len(m.keys)
	status["strategy"] = m.config.Strategy

	healthyCount := 0
	keyStatuses := make([]map[string]interface{}, 0, len(m.keys))

	for i, key := range m.keys {
		health := m.keyHealth[key]
		keyMasked := maskAPIKey(key)

		keyStatus := map[string]interface{}{
			"index":         i + 1,
			"key_masked":    keyMasked,
			"healthy":       health.isHealthy,
			"failure_count": health.failureCount,
			"success_count": health.successCount,
			"request_count": atomic.LoadInt64(&health.requestCount),
			"error_rate":    health.errorRate,
			"last_success":  health.lastSuccess.Format(time.RFC3339),
		}

		if !health.lastFailure.IsZero() {
			keyStatus["last_failure"] = health.lastFailure.Format(time.RFC3339)
		}

		if time.Now().Before(health.backoffUntil) {
			keyStatus["in_backoff"] = true
			keyStatus["backoff_until"] = health.backoffUntil.Format(time.RFC3339)
		}

		if health.circuitBreaker != nil {
			keyStatus["circuit_breaker"] = health.circuitBreaker.getState()
		}

		if health.isHealthy {
			healthyCount++
		}

		keyStatuses = append(keyStatuses, keyStatus)
	}

	status["healthy_keys"] = healthyCount
	status["unhealthy_keys"] = len(m.keys) - healthyCount
	status["keys"] = keyStatuses
	status["is_healthy"] = m.IsHealthy()

	return status
}

// GetKeys returns all configured keys (masked for security)
func (m *APIKeyManagerImpl) GetKeys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	maskedKeys := make([]string, len(m.keys))
	for i, key := range m.keys {
		maskedKeys[i] = maskAPIKey(key)
	}
	return maskedKeys
}

// AddKey adds a new API key to the manager
func (m *APIKeyManagerImpl) AddKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if key already exists
	for _, existingKey := range m.keys {
		if existingKey == key {
			return fmt.Errorf("key already exists")
		}
	}

	// Add key with initial health tracking
	circuitBreaker := newCircuitBreaker(&m.config.Failover.CircuitBreaker)
	m.keys = append(m.keys, key)
	m.keyHealth[key] = &keyHealth{
		isHealthy:      true,
		lastSuccess:    time.Now(),
		circuitBreaker: circuitBreaker,
	}

	return nil
}

// RemoveKey removes an API key from the manager
func (m *APIKeyManagerImpl) RemoveKey(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove key
	for i, existingKey := range m.keys {
		if existingKey == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			delete(m.keyHealth, key)
			return nil
		}
	}

	return fmt.Errorf("key not found")
}

// IsHealthy returns true if at least one key is healthy
func (m *APIKeyManagerImpl) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, health := range m.keyHealth {
		if health.isHealthy && m.isKeyAvailable("", health) {
			return true
		}
	}
	return false
}

// Circuit breaker methods

func (cb *circuitBreaker) isOpen() bool {
	if !cb.config.Enabled {
		return false
	}

	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case circuitOpen:
		// Check if recovery timeout has passed
		if time.Since(cb.lastFailure) >= cb.config.RecoveryTimeout {
			cb.state = circuitHalfOpen
			return false
		}
		return true
	case circuitHalfOpen:
		return false
	default:
		return false
	}
}

func (cb *circuitBreaker) recordSuccess() {
	if !cb.config.Enabled {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.requests++

	if cb.state == circuitHalfOpen {
		// If we get enough successes in half-open state, close the circuit
		if cb.requests >= cb.config.HalfOpenMaxRequests {
			cb.state = circuitClosed
			cb.failures = 0
			cb.requests = 0
			cb.lastReset = time.Now()
		}
	} else {
		cb.state = circuitClosed
		cb.failures = 0
		cb.requests = 0
		cb.lastReset = time.Now()
	}
}

func (cb *circuitBreaker) recordFailure() {
	if !cb.config.Enabled {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.requests++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.config.FailureThreshold {
		cb.state = circuitOpen
	}
}

func (cb *circuitBreaker) getState() string {
	if !cb.config.Enabled {
		return "disabled"
	}

	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case circuitOpen:
		return "open"
	case circuitHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

// Utility functions

func maskAPIKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:8] + "..." + key[len(key)-4:]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
