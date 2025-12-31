// Package gemini provides header style definitions and dual quota management for Code Assist API.
// This module handles automatic fallback between different header styles to maximize quota usage.
package gemini

import (
	"net/http"
	"sync"
	"time"
)

// HeaderStyle represents the type of header configuration to use for Code Assist requests.
// Different header styles access different quota buckets on the backend.
type HeaderStyle string

const (
	// HeaderStyleAntigravity uses the default headers for Code Assist (Antigravity style).
	// This is the current default used by the Code Assist API.
	HeaderStyleAntigravity HeaderStyle = "antigravity"

	// HeaderStyleGeminiCLI uses alternative headers that access a different quota bucket.
	// This style mimics the Gemini CLI tool's header configuration.
	HeaderStyleGeminiCLI HeaderStyle = "gemini-cli"
)

// HeaderConfig defines the headers to use for a specific style.
type HeaderConfig struct {
	// UserAgent is the User-Agent header value
	UserAgent string

	// ExtraHeaders contains additional headers specific to this style
	ExtraHeaders map[string]string
}

// headerConfigs maps each header style to its configuration.
// These configurations determine which quota bucket the request will use.
var headerConfigs = map[HeaderStyle]HeaderConfig{
	HeaderStyleAntigravity: {
		// Default Antigravity/Code Assist style headers
		UserAgent: "google-cloud-sdk gcloud/526.0.0 command/gcloud.ai.models.describe invocation-id/placeholder-id environment/devshell environment-version/None interactive/True from-script/False python/3.11.8 term/xterm-256color (Linux 6.1.85+)",
		ExtraHeaders: map[string]string{
			"x-goog-api-client": "cred-type/u",
		},
	},
	HeaderStyleGeminiCLI: {
		// Gemini CLI style headers (different quota bucket)
		UserAgent: "gemini-cli/0.1.0",
		ExtraHeaders: map[string]string{
			"x-goog-api-client": "gemini-cli/0.1.0 gl-go/1.22.0",
		},
	},
}

// GetHeaderConfig returns the header configuration for a given style.
func GetHeaderConfig(style HeaderStyle) HeaderConfig {
	if config, ok := headerConfigs[style]; ok {
		return config
	}
	// Default to Antigravity style
	return headerConfigs[HeaderStyleAntigravity]
}

// ApplyHeaderStyle applies the header configuration for a given style to an HTTP request.
func ApplyHeaderStyle(req *http.Request, style HeaderStyle) {
	config := GetHeaderConfig(style)

	// Set User-Agent
	req.Header.Set("User-Agent", config.UserAgent)

	// Set extra headers
	for key, value := range config.ExtraHeaders {
		req.Header.Set(key, value)
	}
}

// RateLimitState tracks the rate limit status for a specific header style.
type RateLimitState struct {
	// IsRateLimited indicates if this style is currently rate limited
	IsRateLimited bool

	// RateLimitedAt is when the rate limit was detected
	RateLimitedAt time.Time

	// RetryAfter is the suggested retry delay
	RetryAfter time.Duration

	// LastSuccessAt is when the last successful request was made
	LastSuccessAt time.Time
}

// DualQuotaManager manages the dual quota source header fallback mechanism.
// It tracks which header style to use and handles fallback on rate limits.
type DualQuotaManager struct {
	mu sync.RWMutex

	// preferredStyle is the current preferred header style
	preferredStyle HeaderStyle

	// styleLimitState tracks rate limit state per header style
	styleLimitState map[HeaderStyle]*RateLimitState

	// fallbackEnabled controls whether fallback between styles is enabled
	fallbackEnabled bool

	// rateLimitCooldown is how long to wait before retrying a rate-limited style
	rateLimitCooldown time.Duration
}

// NewDualQuotaManager creates a new dual quota manager with default settings.
func NewDualQuotaManager() *DualQuotaManager {
	return &DualQuotaManager{
		preferredStyle: HeaderStyleAntigravity,
		styleLimitState: map[HeaderStyle]*RateLimitState{
			HeaderStyleAntigravity: {},
			HeaderStyleGeminiCLI:   {},
		},
		fallbackEnabled:   true,
		rateLimitCooldown: 60 * time.Second, // Default 60 second cooldown
	}
}

// NewDualQuotaManagerWithOptions creates a new dual quota manager with custom settings.
func NewDualQuotaManagerWithOptions(preferredStyle HeaderStyle, fallbackEnabled bool, cooldown time.Duration) *DualQuotaManager {
	return &DualQuotaManager{
		preferredStyle: preferredStyle,
		styleLimitState: map[HeaderStyle]*RateLimitState{
			HeaderStyleAntigravity: {},
			HeaderStyleGeminiCLI:   {},
		},
		fallbackEnabled:   fallbackEnabled,
		rateLimitCooldown: cooldown,
	}
}

// GetPreferredStyle returns the current preferred header style.
func (m *DualQuotaManager) GetPreferredStyle() HeaderStyle {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.preferredStyle
}

// SetPreferredStyle sets the preferred header style.
func (m *DualQuotaManager) SetPreferredStyle(style HeaderStyle) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preferredStyle = style
}

// IsFallbackEnabled returns whether fallback between styles is enabled.
func (m *DualQuotaManager) IsFallbackEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fallbackEnabled
}

// SetFallbackEnabled enables or disables fallback between styles.
func (m *DualQuotaManager) SetFallbackEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallbackEnabled = enabled
}

// GetActiveStyle returns the header style to use for the next request.
// It considers rate limit state and returns the best available style.
func (m *DualQuotaManager) GetActiveStyle() HeaderStyle {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if preferred style is available
	if m.isStyleAvailable(m.preferredStyle) {
		return m.preferredStyle
	}

	// If fallback is disabled, return preferred style anyway
	if !m.fallbackEnabled {
		return m.preferredStyle
	}

	// Try alternate style
	alternateStyle := m.getAlternateStyle(m.preferredStyle)
	if m.isStyleAvailable(alternateStyle) {
		return alternateStyle
	}

	// Both styles are rate limited - return the one with shorter remaining cooldown
	return m.getStyleWithShortestCooldown()
}

// isStyleAvailable checks if a style is available (not rate limited or cooldown expired).
// Must be called with mu held (at least RLock).
func (m *DualQuotaManager) isStyleAvailable(style HeaderStyle) bool {
	state, ok := m.styleLimitState[style]
	if !ok {
		return true
	}

	if !state.IsRateLimited {
		return true
	}

	// Check if cooldown has expired
	cooldownExpiry := state.RateLimitedAt.Add(m.rateLimitCooldown)
	if state.RetryAfter > 0 {
		cooldownExpiry = state.RateLimitedAt.Add(state.RetryAfter)
	}

	return time.Now().After(cooldownExpiry)
}

// getAlternateStyle returns the alternate style for a given style.
// Must be called with mu held (at least RLock).
func (m *DualQuotaManager) getAlternateStyle(style HeaderStyle) HeaderStyle {
	if style == HeaderStyleAntigravity {
		return HeaderStyleGeminiCLI
	}
	return HeaderStyleAntigravity
}

// getStyleWithShortestCooldown returns the style with the shortest remaining cooldown.
// Must be called with mu held (at least RLock).
func (m *DualQuotaManager) getStyleWithShortestCooldown() HeaderStyle {
	antigravityExpiry := m.getCooldownExpiry(HeaderStyleAntigravity)
	geminiCLIExpiry := m.getCooldownExpiry(HeaderStyleGeminiCLI)

	if antigravityExpiry.Before(geminiCLIExpiry) {
		return HeaderStyleAntigravity
	}
	return HeaderStyleGeminiCLI
}

// getCooldownExpiry calculates when the cooldown expires for a style.
// Must be called with mu held (at least RLock).
func (m *DualQuotaManager) getCooldownExpiry(style HeaderStyle) time.Time {
	state, ok := m.styleLimitState[style]
	if !ok || !state.IsRateLimited {
		return time.Time{} // No cooldown - available now
	}

	cooldownDuration := m.rateLimitCooldown
	if state.RetryAfter > 0 {
		cooldownDuration = state.RetryAfter
	}

	return state.RateLimitedAt.Add(cooldownDuration)
}

// RecordRateLimit records that a style received a rate limit response.
func (m *DualQuotaManager) RecordRateLimit(style HeaderStyle, retryAfter time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.styleLimitState[style]
	if !ok {
		state = &RateLimitState{}
		m.styleLimitState[style] = state
	}

	state.IsRateLimited = true
	state.RateLimitedAt = time.Now()
	state.RetryAfter = retryAfter
}

// RecordSuccess records a successful request for a style.
func (m *DualQuotaManager) RecordSuccess(style HeaderStyle) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.styleLimitState[style]
	if !ok {
		state = &RateLimitState{}
		m.styleLimitState[style] = state
	}

	state.IsRateLimited = false
	state.LastSuccessAt = time.Now()

	// If this was the alternate style and it succeeded, consider switching preferred
	if style != m.preferredStyle && m.fallbackEnabled {
		// Only switch if the preferred style is still rate limited
		if prefState, ok := m.styleLimitState[m.preferredStyle]; ok && prefState.IsRateLimited {
			m.preferredStyle = style
		}
	}
}

// GetRateLimitState returns the rate limit state for a style.
func (m *DualQuotaManager) GetRateLimitState(style HeaderStyle) *RateLimitState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if state, ok := m.styleLimitState[style]; ok {
		// Return a copy to prevent external modification
		return &RateLimitState{
			IsRateLimited: state.IsRateLimited,
			RateLimitedAt: state.RateLimitedAt,
			RetryAfter:    state.RetryAfter,
			LastSuccessAt: state.LastSuccessAt,
		}
	}
	return &RateLimitState{}
}

// ShouldTryAlternateStyle returns true if we should try the alternate style
// after a rate limit error on the given style.
func (m *DualQuotaManager) ShouldTryAlternateStyle(currentStyle HeaderStyle) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.fallbackEnabled {
		return false
	}

	alternateStyle := m.getAlternateStyle(currentStyle)
	return m.isStyleAvailable(alternateStyle)
}

// GetAlternateStyle returns the alternate style for the given style.
func (m *DualQuotaManager) GetAlternateStyle(style HeaderStyle) HeaderStyle {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getAlternateStyle(style)
}

// Reset clears all rate limit state and resets to default.
func (m *DualQuotaManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.styleLimitState = map[HeaderStyle]*RateLimitState{
		HeaderStyleAntigravity: {},
		HeaderStyleGeminiCLI:   {},
	}
	m.preferredStyle = HeaderStyleAntigravity
}

// GetStatus returns a status summary for debugging/monitoring.
func (m *DualQuotaManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"preferred_style":   string(m.preferredStyle),
		"fallback_enabled":  m.fallbackEnabled,
		"cooldown_duration": m.rateLimitCooldown.String(),
		"active_style":      string(m.GetActiveStyle()),
	}

	// Add state for each style
	for style, state := range m.styleLimitState {
		styleStatus := map[string]interface{}{
			"is_rate_limited": state.IsRateLimited,
		}
		if state.IsRateLimited {
			styleStatus["rate_limited_at"] = state.RateLimitedAt.Format(time.RFC3339)
			styleStatus["retry_after"] = state.RetryAfter.String()
		}
		if !state.LastSuccessAt.IsZero() {
			styleStatus["last_success_at"] = state.LastSuccessAt.Format(time.RFC3339)
		}
		status[string(style)+"_state"] = styleStatus
	}

	return status
}
