package oauthmanager

import (
	"sync"
	"time"
)

// CredentialMetrics tracks usage and performance metrics for a single credential
type CredentialMetrics struct {
	mu sync.RWMutex

	// Usage tracking
	RequestCount int64 // Total requests using this credential
	SuccessCount int64 // Successful requests
	ErrorCount   int64 // Failed requests

	// Token usage
	TokensUsed int64 // Cumulative tokens consumed

	// Performance
	TotalLatency   time.Duration // Cumulative latency
	AverageLatency time.Duration // Calculated average

	// Timing
	FirstUsed time.Time // When credential was first used
	LastUsed  time.Time // Most recent usage

	// Refresh tracking
	RefreshCount    int       // Number of token refreshes
	LastRefreshTime time.Time // Last successful refresh
}

// NewCredentialMetrics creates a new metrics tracker for a credential
func NewCredentialMetrics() *CredentialMetrics {
	return &CredentialMetrics{
		FirstUsed: time.Now(),
	}
}

// recordRequest updates metrics for a single request
func (m *CredentialMetrics) recordRequest(tokensUsed int64, latency time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RequestCount++
	m.LastUsed = time.Now()

	if success {
		m.SuccessCount++
	} else {
		m.ErrorCount++
	}

	if tokensUsed > 0 {
		m.TokensUsed += tokensUsed
	}

	if latency > 0 {
		m.TotalLatency += latency
		m.AverageLatency = m.TotalLatency / time.Duration(m.RequestCount)
	}

	// Set first used if this is the first request
	if m.FirstUsed.IsZero() {
		m.FirstUsed = time.Now()
	}
}

// recordRefresh updates metrics for a token refresh
func (m *CredentialMetrics) recordRefresh() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RefreshCount++
	m.LastRefreshTime = time.Now()
}

// GetSnapshot returns a thread-safe copy of the current metrics
func (m *CredentialMetrics) GetSnapshot() *CredentialMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &CredentialMetrics{
		RequestCount:    m.RequestCount,
		SuccessCount:    m.SuccessCount,
		ErrorCount:      m.ErrorCount,
		TokensUsed:      m.TokensUsed,
		TotalLatency:    m.TotalLatency,
		AverageLatency:  m.AverageLatency,
		FirstUsed:       m.FirstUsed,
		LastUsed:        m.LastUsed,
		RefreshCount:    m.RefreshCount,
		LastRefreshTime: m.LastRefreshTime,
	}
}

// GetSuccessRate returns the success rate as a percentage (0.0 to 1.0)
func (m *CredentialMetrics) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.RequestCount == 0 {
		return 1.0
	}
	return float64(m.SuccessCount) / float64(m.RequestCount)
}

// GetRequestsPerHour returns the average requests per hour based on usage history
func (m *CredentialMetrics) GetRequestsPerHour() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.FirstUsed.IsZero() || m.RequestCount == 0 {
		return 0.0
	}

	// Calculate duration since first use
	duration := time.Since(m.FirstUsed)
	if duration.Hours() < 0.01 { // Less than ~30 seconds
		// Not enough data, return current count as approximation
		return float64(m.RequestCount)
	}

	return float64(m.RequestCount) / duration.Hours()
}

// RecordRequest records metrics for a single request on the manager
func (m *OAuthKeyManager) RecordRequest(credentialID string, tokensUsed int64, latency time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.credMetrics[credentialID]
	if !exists {
		metrics = NewCredentialMetrics()
		m.credMetrics[credentialID] = metrics
	}

	metrics.recordRequest(tokensUsed, latency, success)
}

// GetCredentialMetrics returns the metrics for a specific credential
// Returns nil if the credential ID is not found
func (m *OAuthKeyManager) GetCredentialMetrics(credentialID string) *CredentialMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.credMetrics[credentialID]
	if !exists {
		return nil
	}

	return metrics.GetSnapshot()
}

// GetAllMetrics returns a map of all credential metrics
// Returns a map of credential ID to metrics snapshot
func (m *OAuthKeyManager) GetAllMetrics() map[string]*CredentialMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*CredentialMetrics, len(m.credMetrics))
	for id, metrics := range m.credMetrics {
		result[id] = metrics.GetSnapshot()
	}

	return result
}
