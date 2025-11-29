// Package common provides shared utilities for AI provider implementations.
package common

import (
	"net/http"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
)

// RateLimitTracker wraps RateLimitHelper with automatic response tracking.
// It provides a cleaner API that reduces code duplication by combining
// rate limit parsing and tracking into a single method call.
type RateLimitTracker struct {
	helper    *RateLimitHelper
	mu        sync.RWMutex
	lastCheck *RateLimitInfo
}

// RateLimitInfo holds parsed rate limit information in a simplified format.
// This provides easy access to the most commonly used rate limit fields
// without requiring knowledge of the underlying ratelimit.Info structure.
type RateLimitInfo struct {
	RequestsLimit     int
	RequestsRemaining int
	RequestsReset     int64 // Unix timestamp
	TokensLimit       int
	TokensRemaining   int
	TokensReset       int64 // Unix timestamp
	RetryAfter        int   // Seconds to wait
}

// NewRateLimitTracker creates a new tracker with the given parser.
func NewRateLimitTracker(parser ratelimit.Parser) *RateLimitTracker {
	return &RateLimitTracker{
		helper: NewRateLimitHelper(parser),
	}
}

// TrackResponse extracts and stores rate limit info from response headers.
// This is the single point where rate limits are parsed, replacing multiple
// ParseAndUpdateRateLimits() calls throughout the codebase.
//
// Usage:
//
//	info := p.rateLimitTracker.TrackResponse(resp.Header, modelName)
//	if info != nil && info.RetryAfter > 0 {
//	    // Handle rate limit
//	}
func (t *RateLimitTracker) TrackResponse(headers http.Header, model string) *RateLimitInfo {
	if headers == nil {
		return nil
	}

	// Parse and update rate limits using the helper
	t.helper.ParseAndUpdateRateLimits(headers, model)

	// Get the updated rate limit info
	rateLimitInfo, exists := t.helper.GetRateLimitInfo(model)
	if !exists {
		return nil
	}

	// Convert to simplified format and cache
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastCheck = &RateLimitInfo{
		RequestsLimit:     rateLimitInfo.RequestsLimit,
		RequestsRemaining: rateLimitInfo.RequestsRemaining,
		RequestsReset:     rateLimitInfo.RequestsReset.Unix(),
		TokensLimit:       rateLimitInfo.TokensLimit,
		TokensRemaining:   rateLimitInfo.TokensRemaining,
		TokensReset:       rateLimitInfo.TokensReset.Unix(),
		RetryAfter:        int(rateLimitInfo.RetryAfter.Seconds()),
	}

	return t.lastCheck
}

// GetLastInfo returns the most recent rate limit info.
// This can be used to check rate limits without making another API call.
func (t *RateLimitTracker) GetLastInfo() *RateLimitInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastCheck
}

// IsRateLimited returns true if we should back off based on the last check.
// This provides a simple way to check if rate limits have been exceeded.
func (t *RateLimitTracker) IsRateLimited() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.lastCheck == nil {
		return false
	}
	return t.lastCheck.RequestsRemaining <= 0 || t.lastCheck.TokensRemaining <= 0
}

// GetRetryAfter returns seconds to wait, or 0 if not rate limited.
func (t *RateLimitTracker) GetRetryAfter() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.lastCheck == nil {
		return 0
	}
	return t.lastCheck.RetryAfter
}

// GetHelper returns the underlying helper for backwards compatibility.
// This allows access to advanced features like CheckRateLimitAndWait,
// ShouldThrottle, and provider-specific rate limit information.
func (t *RateLimitTracker) GetHelper() *RateLimitHelper {
	return t.helper
}
