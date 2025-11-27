// Package ratelimit provides rate limiting functionality for AI provider API requests.
// It supports tracking rate limits across multiple providers including Anthropic, OpenAI,
// Cerebras, OpenRouter, and others.
package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// Info contains rate limit information for a specific model from an AI provider.
// It includes standard rate limit fields as well as provider-specific fields
// to accommodate the varying rate limit schemes used by different AI providers.
type Info struct {
	// Provider is the name of the AI provider (e.g., "anthropic", "openai")
	Provider string `json:"provider"`

	// Model is the specific model identifier (e.g., "claude-3-opus-20240229")
	Model string `json:"model"`

	// Timestamp is when this rate limit information was captured
	Timestamp time.Time `json:"timestamp"`

	// Standard rate limit fields (common across most providers)

	// RequestsLimit is the maximum number of requests allowed in the current window
	RequestsLimit int `json:"requests_limit"`

	// RequestsRemaining is the number of requests remaining in the current window
	RequestsRemaining int `json:"requests_remaining"`

	// RequestsReset is when the request limit counter will reset
	RequestsReset time.Time `json:"requests_reset"`

	// TokensLimit is the maximum number of tokens allowed in the current window
	TokensLimit int `json:"tokens_limit"`

	// TokensRemaining is the number of tokens remaining in the current window
	TokensRemaining int `json:"tokens_remaining"`

	// TokensReset is when the token limit counter will reset
	TokensReset time.Time `json:"tokens_reset"`

	// Anthropic-specific fields

	// InputTokensLimit is the maximum number of input tokens allowed (Anthropic)
	InputTokensLimit int `json:"input_tokens_limit,omitempty"`

	// InputTokensRemaining is the number of input tokens remaining (Anthropic)
	InputTokensRemaining int `json:"input_tokens_remaining,omitempty"`

	// InputTokensReset is when the input token limit resets (Anthropic)
	InputTokensReset time.Time `json:"input_tokens_reset,omitempty"`

	// OutputTokensLimit is the maximum number of output tokens allowed (Anthropic)
	OutputTokensLimit int `json:"output_tokens_limit,omitempty"`

	// OutputTokensRemaining is the number of output tokens remaining (Anthropic)
	OutputTokensRemaining int `json:"output_tokens_remaining,omitempty"`

	// OutputTokensReset is when the output token limit resets (Anthropic)
	OutputTokensReset time.Time `json:"output_tokens_reset,omitempty"`

	// Cerebras-specific fields

	// DailyRequestsLimit is the maximum number of requests per day (Cerebras)
	DailyRequestsLimit int `json:"daily_requests_limit,omitempty"`

	// DailyRequestsRemaining is the number of daily requests remaining (Cerebras)
	DailyRequestsRemaining int `json:"daily_requests_remaining,omitempty"`

	// DailyRequestsReset is when the daily request limit resets (Cerebras)
	DailyRequestsReset time.Time `json:"daily_requests_reset,omitempty"`

	// OpenRouter-specific fields

	// CreditsLimit is the maximum credits available (OpenRouter)
	CreditsLimit float64 `json:"credits_limit,omitempty"`

	// CreditsRemaining is the number of credits remaining (OpenRouter)
	CreditsRemaining float64 `json:"credits_remaining,omitempty"`

	// IsFreeTier indicates if the account is on the free tier (OpenRouter)
	IsFreeTier bool `json:"is_free_tier,omitempty"`

	// Generic fields for additional data

	// RequestID is the unique identifier for the request that generated this info
	RequestID string `json:"request_id,omitempty"`

	// RetryAfter indicates how long to wait before retrying (from Retry-After header)
	RetryAfter time.Duration `json:"retry_after,omitempty"`

	// CustomData holds any additional provider-specific data that doesn't fit standard fields
	CustomData map[string]interface{} `json:"custom_data,omitempty"`
}

// Parser is the interface that must be implemented by provider-specific rate limit parsers.
// Each AI provider has different header formats and schemes for communicating rate limits,
// so each provider needs its own parser implementation.
type Parser interface {
	// Parse extracts rate limit information from HTTP response headers.
	// It takes the response headers and model name as input and returns
	// a populated Info struct or an error if parsing fails.
	Parse(headers http.Header, model string) (*Info, error)

	// ProviderName returns the name of the provider this parser handles.
	// This is used for logging and tracking purposes.
	ProviderName() string
}

// Tracker provides thread-safe tracking of rate limit information across multiple models.
// It maintains the current rate limit state for each model and provides methods to
// check if requests can be made and when to retry.
type Tracker struct {
	// mu protects concurrent access to the tracker's internal state
	mu sync.RWMutex

	// info maps model names to their current rate limit information
	info map[string]*Info

	// lastUpdate tracks when the tracker was last updated
	lastUpdate time.Time
}

// NewTracker creates a new Tracker instance for tracking rate limits.
func NewTracker() *Tracker {
	return &Tracker{
		info: make(map[string]*Info),
	}
}

// Update updates the rate limit information for a model.
// This method is thread-safe and can be called concurrently.
func (t *Tracker) Update(info *Info) {
	if info == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.info[info.Model] = info
	t.lastUpdate = time.Now()
}

// Get retrieves the rate limit information for a specific model.
// It returns the Info and a boolean indicating whether the model was found.
// This method is thread-safe and can be called concurrently.
func (t *Tracker) Get(model string) (*Info, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	info, exists := t.info[model]
	return info, exists
}

// CanMakeRequest checks if a request can be made for the given model
// with the estimated number of tokens.
// It returns true if the request is likely to succeed based on current rate limits,
// false otherwise. This method is thread-safe.
func (t *Tracker) CanMakeRequest(model string, estimatedTokens int) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	info, exists := t.info[model]
	if !exists {
		// No rate limit info available, assume we can make the request
		return true
	}

	now := time.Now()

	// Check if request limits have reset
	if t.hasRequestLimitReset(info, now) {
		return true
	}

	// Check various limit types
	if !t.canMakeRequestByLimits(info, now, estimatedTokens) {
		return false
	}

	return true
}

// hasRequestLimitReset checks if request limits have reset
func (t *Tracker) hasRequestLimitReset(info *Info, now time.Time) bool {
	return !info.RequestsReset.IsZero() && now.After(info.RequestsReset)
}

// canMakeRequestByLimits checks all limit types to determine if request can proceed
func (t *Tracker) canMakeRequestByLimits(info *Info, now time.Time, estimatedTokens int) bool {
	// Check request limits
	if info.RequestsLimit > 0 && info.RequestsRemaining <= 0 {
		return false
	}

	// Check token limits
	if !t.canMakeRequestByTokens(info, now, estimatedTokens) {
		return false
	}

	// Check Cerebras daily limits
	if !t.canMakeRequestByDailyLimits(info, now) {
		return false
	}

	// Check OpenRouter credits
	if !t.canMakeRequestByCredits(info) {
		return false
	}

	return true
}

// canMakeRequestByTokens checks token-related limits
func (t *Tracker) canMakeRequestByTokens(info *Info, now time.Time, estimatedTokens int) bool {
	if estimatedTokens <= 0 {
		return true
	}

	// Check standard token limits
	if !info.TokensReset.IsZero() && now.Before(info.TokensReset) {
		if info.TokensLimit > 0 && info.TokensRemaining < estimatedTokens {
			return false
		}
	}

	// Check Anthropic-specific input token limits
	if !info.InputTokensReset.IsZero() && now.Before(info.InputTokensReset) {
		if info.InputTokensLimit > 0 && info.InputTokensRemaining < estimatedTokens {
			return false
		}
	}

	return true
}

// canMakeRequestByDailyLimits checks daily request limits (Cerebras)
func (t *Tracker) canMakeRequestByDailyLimits(info *Info, now time.Time) bool {
	if info.DailyRequestsReset.IsZero() || !now.Before(info.DailyRequestsReset) {
		return true
	}

	return !(info.DailyRequestsLimit > 0 && info.DailyRequestsRemaining <= 0)
}

// canMakeRequestByCredits checks credit limits (OpenRouter)
func (t *Tracker) canMakeRequestByCredits(info *Info) bool {
	return !(info.CreditsLimit > 0 && info.CreditsRemaining <= 0)
}

// GetWaitTime returns the duration to wait before the next request can be made
// for the given model. If no waiting is required, it returns 0.
// This method is thread-safe.
func (t *Tracker) GetWaitTime(model string) time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	info, exists := t.info[model]
	if !exists {
		return 0
	}

	// If RetryAfter is specified, use that
	if info.RetryAfter > 0 {
		return info.RetryAfter
	}

	now := time.Now()
	var waitUntil time.Time

	// Find the earliest reset time that's in the future
	resetTimes := []time.Time{
		info.RequestsReset,
		info.TokensReset,
		info.InputTokensReset,
		info.OutputTokensReset,
		info.DailyRequestsReset,
	}

	for _, resetTime := range resetTimes {
		if resetTime.IsZero() {
			continue
		}

		if now.Before(resetTime) {
			if waitUntil.IsZero() || resetTime.Before(waitUntil) {
				waitUntil = resetTime
			}
		}
	}

	if waitUntil.IsZero() {
		return 0
	}

	return time.Until(waitUntil)
}

// ShouldThrottle determines if requests should be throttled based on
// the current rate limit usage. The threshold parameter is a value between 0 and 1
// representing the percentage of limits consumed at which throttling should begin.
// For example, threshold=0.8 means throttle when 80% of limits are consumed.
// This method is thread-safe.
func (t *Tracker) ShouldThrottle(model string, threshold float64) bool {
	if threshold < 0 || threshold > 1 {
		threshold = 0.8 // Default to 80%
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	info, exists := t.info[model]
	if !exists {
		return false
	}

	now := time.Now()

	// Check various limit types using helper methods
	if t.shouldThrottleByRequests(info, now, threshold) {
		return true
	}
	if t.shouldThrottleByTokens(info, now, threshold) {
		return true
	}
	if t.shouldThrottleByInputTokens(info, now, threshold) {
		return true
	}
	if t.shouldThrottleByOutputTokens(info, now, threshold) {
		return true
	}
	if t.shouldThrottleByDailyRequests(info, now, threshold) {
		return true
	}
	if t.shouldThrottleByCredits(info, threshold) {
		return true
	}

	return false
}

// shouldThrottleByRequests checks if requests should be throttled based on request limits
func (t *Tracker) shouldThrottleByRequests(info *Info, now time.Time, threshold float64) bool {
	if info.RequestsLimit <= 0 || info.RequestsReset.IsZero() || !now.Before(info.RequestsReset) {
		return false
	}
	usageRatio := 1.0 - (float64(info.RequestsRemaining) / float64(info.RequestsLimit))
	return usageRatio >= threshold
}

// shouldThrottleByTokens checks if requests should be throttled based on token limits
func (t *Tracker) shouldThrottleByTokens(info *Info, now time.Time, threshold float64) bool {
	if info.TokensLimit <= 0 || info.TokensReset.IsZero() || !now.Before(info.TokensReset) {
		return false
	}
	usageRatio := 1.0 - (float64(info.TokensRemaining) / float64(info.TokensLimit))
	return usageRatio >= threshold
}

// shouldThrottleByInputTokens checks if requests should be throttled based on input token limits
func (t *Tracker) shouldThrottleByInputTokens(info *Info, now time.Time, threshold float64) bool {
	if info.InputTokensLimit <= 0 || info.InputTokensReset.IsZero() || !now.Before(info.InputTokensReset) {
		return false
	}
	usageRatio := 1.0 - (float64(info.InputTokensRemaining) / float64(info.InputTokensLimit))
	return usageRatio >= threshold
}

// shouldThrottleByOutputTokens checks if requests should be throttled based on output token limits
func (t *Tracker) shouldThrottleByOutputTokens(info *Info, now time.Time, threshold float64) bool {
	if info.OutputTokensLimit <= 0 || info.OutputTokensReset.IsZero() || !now.Before(info.OutputTokensReset) {
		return false
	}
	usageRatio := 1.0 - (float64(info.OutputTokensRemaining) / float64(info.OutputTokensLimit))
	return usageRatio >= threshold
}

// shouldThrottleByDailyRequests checks if requests should be throttled based on daily request limits
func (t *Tracker) shouldThrottleByDailyRequests(info *Info, now time.Time, threshold float64) bool {
	if info.DailyRequestsLimit <= 0 || info.DailyRequestsReset.IsZero() || !now.Before(info.DailyRequestsReset) {
		return false
	}
	usageRatio := 1.0 - (float64(info.DailyRequestsRemaining) / float64(info.DailyRequestsLimit))
	return usageRatio >= threshold
}

// shouldThrottleByCredits checks if requests should be throttled based on credit limits
func (t *Tracker) shouldThrottleByCredits(info *Info, threshold float64) bool {
	if info.CreditsLimit <= 0 {
		return false
	}
	usageRatio := 1.0 - (info.CreditsRemaining / info.CreditsLimit)
	return usageRatio >= threshold
}
