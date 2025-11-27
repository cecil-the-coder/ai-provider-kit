// Package oauthmanager provides OAuth credential management and rotation for AI providers.
// It includes health tracking, automatic refresh, failover, and monitoring for OAuth tokens.
package oauthmanager

import (
	"time"
)

// credentialHealth tracks the health status of an individual OAuth credential
type credentialHealth struct {
	failureCount int
	lastFailure  time.Time
	lastSuccess  time.Time
	isHealthy    bool
	backoffUntil time.Time
	lastRefresh  time.Time

	// Refresh tracking
	refreshInFlight  bool
	lastRefreshError error
	refreshFailCount int
}

// isCredentialAvailable checks if a credential is available (not in backoff)
func (h *credentialHealth) isCredentialAvailable() bool {
	if h == nil {
		return true
	}

	// Check if credential is in backoff period
	if time.Now().Before(h.backoffUntil) {
		return false
	}

	return true
}

// calculateBackoff calculates exponential backoff duration based on failure count
// Returns backoff duration: 1s, 2s, 4s, 8s, 16s, 32s, max 60s
func (h *credentialHealth) calculateBackoff() time.Duration {
	if h.failureCount == 0 {
		return 0
	}

	// Safe exponential backoff: 1s, 2s, 4s, 8s, 16s, 32s
	failureCount := h.failureCount - 1
	if failureCount < 0 {
		failureCount = 0
	}
	if failureCount > 6 {
		failureCount = 6
	}
	backoffSeconds := 1 << uint(failureCount) // #nosec G115 -- failureCount is capped at 6, safe conversion

	// Cap at 60 seconds
	if backoffSeconds > 60 {
		backoffSeconds = 60
	}

	return time.Duration(backoffSeconds) * time.Second
}

// recordSuccess updates health metrics for a successful operation
func (h *credentialHealth) recordSuccess() {
	h.lastSuccess = time.Now()
	h.failureCount = 0
	h.isHealthy = true
	h.backoffUntil = time.Time{} // Clear backoff
}

// recordFailure updates health metrics for a failed operation
func (h *credentialHealth) recordFailure() {
	h.lastFailure = time.Now()
	h.failureCount++

	// Calculate and apply backoff
	backoffDuration := h.calculateBackoff()
	h.backoffUntil = time.Now().Add(backoffDuration)

	// Mark as unhealthy after 3 consecutive failures
	if h.failureCount >= 3 {
		h.isHealthy = false
	}
}

// recordRefreshSuccess updates health metrics for a successful token refresh
func (h *credentialHealth) recordRefreshSuccess() {
	h.lastRefresh = time.Now()
	h.refreshFailCount = 0
	h.lastRefreshError = nil
	h.refreshInFlight = false
	// Successful refresh also counts as general health improvement
	// Reset API failure count since we have fresh tokens
	h.failureCount = 0
	h.isHealthy = true
	h.backoffUntil = time.Time{}
}

// recordRefreshFailure updates health metrics for a failed token refresh
func (h *credentialHealth) recordRefreshFailure(err error) {
	h.refreshFailCount++
	h.lastRefreshError = err
	h.refreshInFlight = false

	// Too many refresh failures (5+) should mark credential as unhealthy
	// This is different from API failures - refresh failures are more serious
	if h.refreshFailCount >= 5 {
		h.isHealthy = false
		// Apply longer backoff for refresh failures (up to 5 minutes)
		refreshFailureCount := h.refreshFailCount - 5
		if refreshFailureCount < 0 {
			refreshFailureCount = 0
		}
		if refreshFailureCount > 3 {
			refreshFailureCount = 3
		}
		backoffSeconds := 60 << uint(refreshFailureCount) // #nosec G115 -- refreshFailureCount is capped at 3, safe conversion
		if backoffSeconds > 480 {
			backoffSeconds = 480
		}
		h.backoffUntil = time.Now().Add(time.Duration(backoffSeconds) * time.Second)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
