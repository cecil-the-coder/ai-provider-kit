package ratelimit

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// OpenAIParser implements the Parser interface for OpenAI's rate limit headers.
// OpenAI provides rate limit information in the following format:
//   - x-ratelimit-limit-requests: Maximum requests allowed per time window
//   - x-ratelimit-remaining-requests: Requests remaining in current window
//   - x-ratelimit-reset-requests: Duration until window resets (e.g., "6m0s", "1h30m")
//   - x-ratelimit-limit-tokens: Maximum tokens allowed per time window
//   - x-ratelimit-remaining-tokens: Tokens remaining in current window
//   - x-ratelimit-reset-tokens: Duration until token window resets
//   - x-request-id: Unique identifier for the request
//   - retry-after: Optional retry delay in seconds
type OpenAIParser struct{}

// Parse extracts rate limit information from OpenAI response headers.
// It handles both request-based and token-based rate limits.
// Reset times are provided as duration strings (e.g., "6m0s") which are parsed
// and converted to absolute timestamps.
func (p *OpenAIParser) Parse(headers http.Header, model string) (*Info, error) {
	info := &Info{
		Provider: "openai",
		Model:    model,
	}

	// Parse request-based rate limits
	if limit := headers.Get("x-ratelimit-limit-requests"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			info.RequestsLimit = val
		}
	}

	if remaining := headers.Get("x-ratelimit-remaining-requests"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			info.RequestsRemaining = val
		}
	}

	if reset := headers.Get("x-ratelimit-reset-requests"); reset != "" {
		if duration, err := time.ParseDuration(reset); err == nil {
			info.RequestsReset = time.Now().Add(duration)
		}
	}

	// Parse token-based rate limits
	if limit := headers.Get("x-ratelimit-limit-tokens"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			info.TokensLimit = val
		}
	}

	if remaining := headers.Get("x-ratelimit-remaining-tokens"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			info.TokensRemaining = val
		}
	}

	if reset := headers.Get("x-ratelimit-reset-tokens"); reset != "" {
		if duration, err := time.ParseDuration(reset); err == nil {
			info.TokensReset = time.Now().Add(duration)
		}
	}

	// Parse optional request ID
	if requestID := headers.Get("x-request-id"); requestID != "" {
		info.RequestID = requestID
	}

	// Parse optional retry-after header (in seconds)
	if retryAfter := headers.Get("retry-after"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			info.RetryAfter = time.Duration(seconds) * time.Second
		}
	}

	return info, nil
}

// ProviderName returns "openai" as the provider identifier.
func (p *OpenAIParser) ProviderName() string {
	return "openai"
}

// NewOpenAIParser creates a new OpenAI rate limit parser.
func NewOpenAIParser() *OpenAIParser {
	return &OpenAIParser{}
}

// FormatDuration formats a duration string in a human-readable way.
// This is useful for displaying reset times.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
