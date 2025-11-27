package ratelimit

import (
	"net/http"
	"strconv"
	"time"
)

// GeminiParser implements the Parser interface for Google's Gemini API rate limit headers.
//
// IMPORTANT LIMITATION: The Gemini API does NOT provide rate limit information in normal
// API responses. Unlike OpenAI, Anthropic, and other providers, Gemini does not include
// headers like x-ratelimit-limit-requests or x-ratelimit-remaining-requests in successful
// responses (200 OK).
//
// This parser can only extract information from error responses (429 Too Many Requests):
//   - retry-after: Number of seconds to wait before retrying (or HTTP date)
//
// For proactive rate limiting with Gemini, client-side tracking is required:
//   - Track your own request counts and timing
//   - Implement token bucket or leaky bucket algorithms
//   - Use official quota limits from Google Cloud Console
//   - Monitor usage through Google Cloud Console/API
//
// The Gemini API follows these rate limits (as of 2024):
//   - Free tier: 15 RPM (requests per minute), 1 million TPM (tokens per minute)
//   - Pay-as-you-go: 360 RPM, 4 million TPM (varies by model)
//   - Limits are per project and can be viewed in Google Cloud Console
//
// Since these limits are not provided in headers, this parser primarily serves to:
//  1. Extract retry-after duration from 429 error responses
//  2. Provide a consistent interface with other provider parsers
//  3. Return minimal Info with Provider and Model for tracking purposes
type GeminiParser struct{}

// Parse extracts rate limit information from Gemini API response headers.
//
// Unlike other providers, Gemini does not return proactive rate limit headers.
// This parser only extracts the retry-after header when present (typically in 429 responses).
//
// The retry-after header can be in two formats:
//   - Integer seconds: "60" (wait 60 seconds)
//   - HTTP date: "Wed, 21 Oct 2015 07:28:00 GMT" (wait until this time)
//
// For normal successful responses (200 OK), this will return minimal Info with:
//   - Provider: "gemini"
//   - Model: the provided model name
//   - All limit/remaining fields: 0 (unknown)
//   - RetryAfter: 0 (no retry needed)
//
// Parameters:
//   - headers: HTTP response headers from Gemini API
//   - model: The model identifier (e.g., "gemini-pro", "gemini-pro-vision")
//
// Returns:
//   - Info with minimal rate limit information
//   - error is always nil (this parser doesn't fail)
func (p *GeminiParser) Parse(headers http.Header, model string) (*Info, error) {
	info := &Info{
		Provider: "gemini",
		Model:    model,
	}

	// Gemini does not provide proactive rate limit headers.
	// All standard limit/remaining fields remain at their zero values (0).
	// This indicates that rate limit information is not available.

	// Extract retry-after header if present (typically only in 429 responses)
	if retryAfter := headers.Get("retry-after"); retryAfter != "" {
		// Try parsing as integer seconds first
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			info.RetryAfter = time.Duration(seconds) * time.Second
		} else {
			// Try parsing as HTTP date (RFC1123 format)
			if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
				// Calculate duration from now until the specified time
				info.RetryAfter = time.Until(t)
				// Ensure non-negative duration
				if info.RetryAfter < 0 {
					info.RetryAfter = 0
				}
			}
		}
	}

	// Extract request ID if present (useful for debugging and logging)
	if requestID := headers.Get("x-request-id"); requestID != "" {
		info.RequestID = requestID
	}

	// Note: Since Gemini doesn't provide rate limit headers, all limit and remaining
	// fields will be 0, and reset times will be the zero value (IsZero() == true).
	// Consumers of this Info should implement client-side rate limiting instead.

	return info, nil
}

// ProviderName returns "gemini" as the provider identifier.
func (p *GeminiParser) ProviderName() string {
	return "gemini"
}

// NewGeminiParser creates a new Gemini rate limit parser.
func NewGeminiParser() *GeminiParser {
	return &GeminiParser{}
}
