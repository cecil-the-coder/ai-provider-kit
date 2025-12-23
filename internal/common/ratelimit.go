// Package common provides shared utilities for AI provider implementations.
package common

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
)

// RateLimitHelper provides shared rate limiting functionality for all AI providers.
// It encapsulates the common patterns of rate limit tracking, parsing, and enforcement
// used across different provider implementations.
type RateLimitHelper struct {
	tracker *ratelimit.Tracker
	parser  ratelimit.Parser
	mutex   sync.RWMutex
}

// NewRateLimitHelper creates a new RateLimitHelper with the given provider-specific parser.
func NewRateLimitHelper(parser ratelimit.Parser) *RateLimitHelper {
	return &RateLimitHelper{
		tracker: ratelimit.NewTracker(),
		parser:  parser,
	}
}

// ParseAndUpdateRateLimits parses rate limit headers from an HTTP response and updates the tracker.
// This is the most common operation performed after receiving API responses.
func (h *RateLimitHelper) ParseAndUpdateRateLimits(headers http.Header, model string) {
	if info, err := h.parser.Parse(headers, model); err == nil {
		h.mutex.Lock()
		h.tracker.Update(info)
		h.mutex.Unlock()

		// Log rate limit information using provider-specific formatting
		h.logRateLimitInfo(info)
	}
}

// CanMakeRequest checks if a request can be made for the given model and estimated tokens.
// Returns true if the request should proceed, false if rate limited.
func (h *RateLimitHelper) CanMakeRequest(model string, estimatedTokens int) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.tracker.CanMakeRequest(model, estimatedTokens)
}

// GetWaitTime returns the duration to wait before the next request can be made.
// Returns 0 if no waiting is required.
func (h *RateLimitHelper) GetWaitTime(model string) time.Duration {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.tracker.GetWaitTime(model)
}

// CheckRateLimitAndWait checks rate limits and waits if necessary before making a request.
// This combines the common pattern of checking limits and sleeping if needed.
// Returns true if the caller should proceed with the request, false if rate limited.
func (h *RateLimitHelper) CheckRateLimitAndWait(model string, estimatedTokens int) bool {
	if !h.CanMakeRequest(model, estimatedTokens) {
		waitTime := h.GetWaitTime(model)
		if waitTime > 0 {
			log.Printf("%s: Rate limit reached for model %s, waiting %v",
				h.parser.ProviderName(), model, waitTime)
			time.Sleep(waitTime)
		}
		return false // Rate limit was hit
	}
	return true // Can proceed
}

// GetRateLimitInfo retrieves the current rate limit information for a model.
// Returns the info and a boolean indicating whether data exists for the model.
func (h *RateLimitHelper) GetRateLimitInfo(model string) (*ratelimit.Info, bool) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.tracker.Get(model)
}

// ShouldThrottle determines if requests should be throttled based on current usage.
// threshold is a value between 0 and 1 representing the percentage of limits consumed
// at which throttling should begin (e.g., 0.8 = throttle at 80% usage).
func (h *RateLimitHelper) ShouldThrottle(model string, threshold float64) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.tracker.ShouldThrottle(model, threshold)
}

// logRateLimitInfo logs rate limit information using provider-specific formatting.
// This centralizes the logging logic while allowing for provider-specific details.
func (h *RateLimitHelper) logRateLimitInfo(info *ratelimit.Info) {
	providerName := h.parser.ProviderName()

	// Standard rate limit logging (common to all providers)
	if info.RequestsRemaining > 0 {
		log.Printf("%s: Requests remaining: %d/%d, resets at %v",
			providerName, info.RequestsRemaining, info.RequestsLimit, info.RequestsReset)
	}

	// Token limit logging
	if info.TokensRemaining > 0 {
		log.Printf("%s: Tokens remaining: %d/%d, resets at %v",
			providerName, info.TokensRemaining, info.TokensLimit, info.TokensReset)
	}

	// Provider-specific logging
	switch providerName {
	case "anthropic":
		h.logAnthropicRateLimits(info)
	case "openai":
		h.logOpenAIRateLimits(info)
	case "cerebras":
		h.logCerebrasRateLimits(info)
	case "openrouter":
		h.logOpenRouterRateLimits(info)
	case "gemini":
		h.logGeminiRateLimits(info)
	case "qwen":
		h.logQwenRateLimits(info)
	}

	// Retry-after logging (common to all providers)
	if info.RetryAfter > 0 {
		log.Printf("%s: Rate limited, retry after %v", providerName, info.RetryAfter)
	}
}

// Provider-specific logging methods

func (h *RateLimitHelper) logAnthropicRateLimits(info *ratelimit.Info) {
	// Log separate input/output token tracking (Anthropic-specific feature)
	if info.InputTokensRemaining > 0 || info.OutputTokensRemaining > 0 {
		log.Printf("Anthropic: Input tokens remaining: %d/%d, Output tokens remaining: %d/%d",
			info.InputTokensRemaining, info.InputTokensLimit,
			info.OutputTokensRemaining, info.OutputTokensLimit)
	}
}

func (h *RateLimitHelper) logOpenAIRateLimits(info *ratelimit.Info) {
	// OpenAI typically uses standard request/token limits
	if info.RequestsRemaining > 0 && info.TokensRemaining > 0 {
		log.Printf("OpenAI: Requests remaining: %d/%d, Tokens remaining: %d/%d",
			info.RequestsRemaining, info.RequestsLimit,
			info.TokensRemaining, info.TokensLimit)
	}
}

func (h *RateLimitHelper) logCerebrasRateLimits(info *ratelimit.Info) {
	// Log dual tracking (daily and per-minute limits)
	if info.DailyRequestsRemaining > 0 || info.RequestsRemaining > 0 {
		log.Printf("Cerebras: Daily requests remaining: %d, Per-minute requests remaining: %d",
			info.DailyRequestsRemaining, info.RequestsRemaining)
	}
}

func (h *RateLimitHelper) logOpenRouterRateLimits(info *ratelimit.Info) {
	// OpenRouter uses credits system
	if info.RequestsRemaining > 0 {
		log.Printf("OpenRouter: Requests remaining: %d/%d",
			info.RequestsRemaining, info.RequestsLimit)
	}
}

func (h *RateLimitHelper) logGeminiRateLimits(info *ratelimit.Info) {
	// Gemini often doesn't provide rate limit headers, so minimal logging
	if info.RequestsRemaining > 0 {
		log.Printf("Gemini: Requests remaining: %d/%d, resets at %v",
			info.RequestsRemaining, info.RequestsLimit, info.RequestsReset)
	}
}

func (h *RateLimitHelper) logQwenRateLimits(info *ratelimit.Info) {
	// Log captured headers for documentation (Qwen has variable headers)
	if len(info.CustomData) > 0 {
		log.Printf("Qwen: Rate limit headers captured: %+v", info.CustomData)
	}
	if info.RequestsRemaining > 0 {
		log.Printf("Qwen: Requests remaining: %d/%d, resets at %v",
			info.RequestsRemaining, info.RequestsLimit, info.RequestsReset)
	}
}

// UpdateRateLimitInfo directly updates the rate limit info for a model.
// This is useful for providers that get rate limit info from API endpoints
// rather than response headers.
func (h *RateLimitHelper) UpdateRateLimitInfo(info *ratelimit.Info) {
	if info == nil {
		return
	}

	h.mutex.Lock()
	h.tracker.Update(info)
	h.mutex.Unlock()

	h.logRateLimitInfo(info)
}

// GetTracker returns the underlying rate limit tracker for advanced operations.
// This should be used sparingly when the helper methods don't provide sufficient functionality.
func (h *RateLimitHelper) GetTracker() *ratelimit.Tracker {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.tracker
}

// GetParser returns the underlying rate limit parser for advanced operations.
// This should be used sparingly when the helper methods don't provide sufficient functionality.
func (h *RateLimitHelper) GetParser() ratelimit.Parser {
	return h.parser
}
