package oauthmanager

import (
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// RefreshStrategy defines how and when OAuth tokens should be refreshed
type RefreshStrategy struct {
	// Buffer configuration
	BufferTime time.Duration // Time before expiry to refresh (default: 5 min)

	// Adaptive refresh
	AdaptiveBuffer bool          // Adjust buffer based on usage patterns
	MinBuffer      time.Duration // Minimum buffer (default: 1 min)
	MaxBuffer      time.Duration // Maximum buffer (default: 15 min)

	// Preemptive refresh
	PreemptiveRefresh    bool  // Refresh before buffer for high-traffic creds
	HighTrafficThreshold int64 // Requests/hour to trigger preemptive (default: 100)
}

// DefaultRefreshStrategy returns the default refresh strategy with sensible defaults
func DefaultRefreshStrategy() *RefreshStrategy {
	return &RefreshStrategy{
		BufferTime:           5 * time.Minute,
		AdaptiveBuffer:       false,
		MinBuffer:            1 * time.Minute,
		MaxBuffer:            15 * time.Minute,
		PreemptiveRefresh:    false,
		HighTrafficThreshold: 100,
	}
}

// AdaptiveRefreshStrategy returns a refresh strategy with adaptive buffer enabled
func AdaptiveRefreshStrategy() *RefreshStrategy {
	return &RefreshStrategy{
		BufferTime:           5 * time.Minute,
		AdaptiveBuffer:       true,
		MinBuffer:            1 * time.Minute,
		MaxBuffer:            15 * time.Minute,
		PreemptiveRefresh:    true,
		HighTrafficThreshold: 100,
	}
}

// ConservativeRefreshStrategy returns a strategy that refreshes tokens very early
func ConservativeRefreshStrategy() *RefreshStrategy {
	return &RefreshStrategy{
		BufferTime:           15 * time.Minute,
		AdaptiveBuffer:       false,
		MinBuffer:            10 * time.Minute,
		MaxBuffer:            30 * time.Minute,
		PreemptiveRefresh:    true,
		HighTrafficThreshold: 50,
	}
}

// ShouldRefresh determines if a credential should be refreshed based on the strategy
func (s *RefreshStrategy) ShouldRefresh(cred *types.OAuthCredentialSet, metrics *CredentialMetrics) bool {
	if cred == nil || cred.ExpiresAt.IsZero() {
		return false
	}

	// Calculate the buffer time to use
	bufferTime := s.CalculateBufferTime(metrics)

	// Check if token expires within the buffer time
	expiresWithinBuffer := time.Now().After(cred.ExpiresAt.Add(-bufferTime))

	// If preemptive refresh is enabled and this is a high-traffic credential,
	// refresh even earlier
	if s.PreemptiveRefresh && metrics != nil {
		requestsPerHour := metrics.GetRequestsPerHour()
		if requestsPerHour >= float64(s.HighTrafficThreshold) {
			// For high-traffic credentials, double the buffer time
			preemptiveBuffer := bufferTime * 2
			// Only apply MaxBuffer constraint if it's explicitly set (non-zero)
			if s.MaxBuffer > 0 && preemptiveBuffer > s.MaxBuffer {
				preemptiveBuffer = s.MaxBuffer
			}
			return time.Now().After(cred.ExpiresAt.Add(-preemptiveBuffer))
		}
	}

	return expiresWithinBuffer
}

// CalculateBufferTime calculates the appropriate buffer time based on usage metrics
func (s *RefreshStrategy) CalculateBufferTime(metrics *CredentialMetrics) time.Duration {
	// If adaptive buffer is disabled, use the fixed buffer time
	if !s.AdaptiveBuffer || metrics == nil {
		return s.BufferTime
	}

	// Start with the base buffer time
	bufferTime := s.BufferTime

	// Adjust based on average latency
	// If requests are slow (high latency), increase buffer to ensure refresh happens in time
	if metrics.AverageLatency > 0 {
		// For every 100ms of average latency, add 30 seconds to buffer
		latencyFactor := metrics.AverageLatency.Seconds() / 0.1
		additionalBuffer := time.Duration(latencyFactor*30) * time.Second
		bufferTime += additionalBuffer
	}

	// Adjust based on request rate
	// Higher request rate means more risk, so increase buffer
	requestsPerHour := metrics.GetRequestsPerHour()
	if requestsPerHour > 10 {
		// For every 10 requests/hour above baseline, add 30 seconds
		rateFactor := (requestsPerHour - 10) / 10
		additionalBuffer := time.Duration(rateFactor*30) * time.Second
		bufferTime += additionalBuffer
	}

	// Adjust based on error rate
	// Higher error rate suggests instability, so increase buffer for safety
	successRate := metrics.GetSuccessRate()
	if successRate < 0.95 { // Less than 95% success rate
		errorRatePenalty := (0.95 - successRate) * 10 // 0-0.95 range
		additionalBuffer := time.Duration(errorRatePenalty*60) * time.Second
		bufferTime += additionalBuffer
	}

	// Ensure buffer stays within min/max bounds
	if s.MinBuffer > 0 && bufferTime < s.MinBuffer {
		bufferTime = s.MinBuffer
	}
	if s.MaxBuffer > 0 && bufferTime > s.MaxBuffer {
		bufferTime = s.MaxBuffer
	}

	return bufferTime
}

// SetRefreshStrategy sets the refresh strategy for the manager
func (m *OAuthKeyManager) SetRefreshStrategy(strategy *RefreshStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strategy == nil {
		strategy = DefaultRefreshStrategy()
	}

	m.refreshStrategy = strategy
}

// GetRefreshStrategy returns the current refresh strategy
func (m *OAuthKeyManager) GetRefreshStrategy() *RefreshStrategy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.refreshStrategy == nil {
		return DefaultRefreshStrategy()
	}

	// Return a copy to prevent external modification
	return &RefreshStrategy{
		BufferTime:           m.refreshStrategy.BufferTime,
		AdaptiveBuffer:       m.refreshStrategy.AdaptiveBuffer,
		MinBuffer:            m.refreshStrategy.MinBuffer,
		MaxBuffer:            m.refreshStrategy.MaxBuffer,
		PreemptiveRefresh:    m.refreshStrategy.PreemptiveRefresh,
		HighTrafficThreshold: m.refreshStrategy.HighTrafficThreshold,
	}
}
