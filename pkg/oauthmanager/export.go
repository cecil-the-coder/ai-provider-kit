package oauthmanager

import (
	"encoding/json"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// PrometheusMetrics represents metrics in Prometheus-compatible format
type PrometheusMetrics struct {
	RequestsTotal    map[string]int64           // Counter per credential
	SuccessTotal     map[string]int64           // Success counter per credential
	ErrorsTotal      map[string]int64           // Error counter per credential
	TokensUsedTotal  map[string]int64           // Token usage counter per credential
	RefreshesTotal   map[string]int64           // Refresh counter per credential
	LatencyHistogram map[string][]time.Duration // Latency samples per credential
}

// HealthSummary provides a high-level health overview
type HealthSummary struct {
	TotalCredentials     int                             `json:"total_credentials"`
	HealthyCredentials   int                             `json:"healthy_credentials"`
	UnhealthyCredentials int                             `json:"unhealthy_credentials"`
	CredentialsInBackoff int                             `json:"credentials_in_backoff"`
	TotalRequests        int64                           `json:"total_requests"`
	SuccessRate          float64                         `json:"success_rate"`
	Credentials          map[string]CredentialHealthInfo `json:"credentials"`
}

// CredentialHealthInfo provides detailed health info for a single credential
type CredentialHealthInfo struct {
	ID                string        `json:"id"`
	IsHealthy         bool          `json:"is_healthy"`
	IsAvailable       bool          `json:"is_available"`
	FailureCount      int           `json:"failure_count"`
	SuccessRate       float64       `json:"success_rate"`
	RequestCount      int64         `json:"request_count"`
	TokensUsed        int64         `json:"tokens_used"`
	AverageLatency    time.Duration `json:"average_latency"`
	LastUsed          time.Time     `json:"last_used"`
	LastSuccess       time.Time     `json:"last_success"`
	LastFailure       time.Time     `json:"last_failure"`
	BackoffUntil      time.Time     `json:"backoff_until,omitempty"`
	RefreshCount      int           `json:"refresh_count"`
	LastRefresh       time.Time     `json:"last_refresh"`
	RefreshFailCount  int           `json:"refresh_fail_count"`
	ExpiresAt         time.Time     `json:"expires_at,omitempty"`
	MarkedForRotation bool          `json:"marked_for_rotation"`
	DecommissionAt    time.Time     `json:"decommission_at,omitempty"`
}

// ExportPrometheus exports metrics in Prometheus-compatible format
func (m *OAuthKeyManager) ExportPrometheus() *PrometheusMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := &PrometheusMetrics{
		RequestsTotal:    make(map[string]int64),
		SuccessTotal:     make(map[string]int64),
		ErrorsTotal:      make(map[string]int64),
		TokensUsedTotal:  make(map[string]int64),
		RefreshesTotal:   make(map[string]int64),
		LatencyHistogram: make(map[string][]time.Duration),
	}

	for credID, credMetrics := range m.credMetrics {
		snapshot := credMetrics.GetSnapshot()

		metrics.RequestsTotal[credID] = snapshot.RequestCount
		metrics.SuccessTotal[credID] = snapshot.SuccessCount
		metrics.ErrorsTotal[credID] = snapshot.ErrorCount
		metrics.TokensUsedTotal[credID] = snapshot.TokensUsed
		metrics.RefreshesTotal[credID] = int64(snapshot.RefreshCount)

		// For histogram, we store the average latency as a single sample
		// In a real implementation, you'd want to track actual histogram buckets
		if snapshot.AverageLatency > 0 {
			metrics.LatencyHistogram[credID] = []time.Duration{snapshot.AverageLatency}
		}
	}

	return metrics
}

// ExportJSON exports all metrics and health data as JSON
func (m *OAuthKeyManager) ExportJSON() ([]byte, error) {
	summary := m.GetHealthSummary()
	return json.MarshalIndent(summary, "", "  ")
}

// GetHealthSummary returns a comprehensive health summary for all credentials
func (m *OAuthKeyManager) GetHealthSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := HealthSummary{
		TotalCredentials: len(m.credentials),
		Credentials:      make(map[string]CredentialHealthInfo),
	}

	var totalRequests int64
	var totalSuccess int64

	for _, cred := range m.credentials {
		// Get health info
		health := m.credHealth[cred.ID]
		if health == nil {
			continue
		}

		// Get metrics
		metrics := m.credMetrics[cred.ID]
		var metricsSnapshot *CredentialMetrics
		if metrics != nil {
			metricsSnapshot = metrics.GetSnapshot()
			totalRequests += metricsSnapshot.RequestCount
			totalSuccess += metricsSnapshot.SuccessCount
		}

		// Get rotation state
		rotationState := m.rotationState[cred.ID]

		// Build credential info
		info := CredentialHealthInfo{
			ID:               cred.ID,
			IsHealthy:        health.isHealthy,
			IsAvailable:      health.isCredentialAvailable(),
			FailureCount:     health.failureCount,
			LastSuccess:      health.lastSuccess,
			LastFailure:      health.lastFailure,
			BackoffUntil:     health.backoffUntil,
			RefreshCount:     cred.RefreshCount,
			LastRefresh:      health.lastRefresh,
			RefreshFailCount: health.refreshFailCount,
			ExpiresAt:        cred.ExpiresAt,
		}

		if metricsSnapshot != nil {
			info.RequestCount = metricsSnapshot.RequestCount
			info.TokensUsed = metricsSnapshot.TokensUsed
			info.AverageLatency = metricsSnapshot.AverageLatency
			info.LastUsed = metricsSnapshot.LastUsed
			info.SuccessRate = metricsSnapshot.GetSuccessRate()
		}

		if rotationState != nil {
			info.MarkedForRotation = rotationState.MarkedForRotation
			info.DecommissionAt = rotationState.DecommissionAt
		}

		summary.Credentials[cred.ID] = info

		// Update summary counts
		if health.isHealthy {
			summary.HealthyCredentials++
		} else {
			summary.UnhealthyCredentials++
		}

		if !health.isCredentialAvailable() {
			summary.CredentialsInBackoff++
		}
	}

	// Calculate overall success rate
	summary.TotalRequests = totalRequests
	if totalRequests > 0 {
		summary.SuccessRate = float64(totalSuccess) / float64(totalRequests)
	} else {
		summary.SuccessRate = 1.0
	}

	// Convert to map for JSON export
	return map[string]interface{}{
		"total_credentials":      summary.TotalCredentials,
		"healthy_credentials":    summary.HealthyCredentials,
		"unhealthy_credentials":  summary.UnhealthyCredentials,
		"credentials_in_backoff": summary.CredentialsInBackoff,
		"total_requests":         summary.TotalRequests,
		"success_rate":           summary.SuccessRate,
		"credentials":            summary.Credentials,
	}
}

// GetCredentialHealthInfo returns detailed health info for a specific credential
func (m *OAuthKeyManager) GetCredentialHealthInfo(credentialID string) *CredentialHealthInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find the credential
	var cred *types.OAuthCredentialSet
	for _, c := range m.credentials {
		if c.ID == credentialID {
			cred = c
			break
		}
	}

	if cred == nil {
		return nil
	}

	// Get health info
	health := m.credHealth[credentialID]
	if health == nil {
		return nil
	}

	// Get metrics
	metrics := m.credMetrics[credentialID]
	var metricsSnapshot *CredentialMetrics
	if metrics != nil {
		metricsSnapshot = metrics.GetSnapshot()
	}

	// Get rotation state
	rotationState := m.rotationState[credentialID]

	// Build credential info
	info := &CredentialHealthInfo{
		ID:               cred.ID,
		IsHealthy:        health.isHealthy,
		IsAvailable:      health.isCredentialAvailable(),
		FailureCount:     health.failureCount,
		LastSuccess:      health.lastSuccess,
		LastFailure:      health.lastFailure,
		BackoffUntil:     health.backoffUntil,
		RefreshCount:     cred.RefreshCount,
		LastRefresh:      health.lastRefresh,
		RefreshFailCount: health.refreshFailCount,
		ExpiresAt:        cred.ExpiresAt,
	}

	if metricsSnapshot != nil {
		info.RequestCount = metricsSnapshot.RequestCount
		info.TokensUsed = metricsSnapshot.TokensUsed
		info.AverageLatency = metricsSnapshot.AverageLatency
		info.LastUsed = metricsSnapshot.LastUsed
		info.SuccessRate = metricsSnapshot.GetSuccessRate()
	}

	if rotationState != nil {
		info.MarkedForRotation = rotationState.MarkedForRotation
		info.DecommissionAt = rotationState.DecommissionAt
	}

	return info
}
