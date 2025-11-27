package ratelimit

import (
	"net/http"
	"strconv"
	"time"
)

// OpenRouterParser implements the Parser interface for OpenRouter's rate limit headers.
// OpenRouter provides rate limit information in the following format:
//   - x-ratelimit-limit: Maximum credits or requests allowed per time window
//   - x-ratelimit-remaining: Credits or requests remaining in current window
//   - x-ratelimit-reset: Milliseconds since epoch when the limit resets
//   - x-ratelimit-requests: Optional request count limit
//   - x-ratelimit-tokens: Optional token count limit
//
// OpenRouter uses a credit-based system where different models consume different amounts
// of credits per request. The free tier has different limits than paid tiers.
//
// Note: OpenRouter also provides a proactive rate limit checking endpoint at /api/v1/key
// which can be used to query rate limits without making actual model requests. This parser
// only handles rate limit information from response headers.
type OpenRouterParser struct{}

// Parse extracts rate limit information from OpenRouter response headers.
// OpenRouter uses a hybrid system that can include both credit-based and request-based limits.
//
// Key differences from other providers:
//   - Reset time is in MILLISECONDS since epoch (not seconds or duration)
//   - May have credit-based limits (x-ratelimit-limit/remaining as floats)
//   - May have request-based limits (x-ratelimit-requests)
//   - May have token-based limits (x-ratelimit-tokens)
//   - Free tier accounts may have different limits
func (p *OpenRouterParser) Parse(headers http.Header, model string) (*Info, error) {
	info := &Info{
		Provider: "openrouter",
		Model:    model,
	}

	// Parse credit-based rate limits
	p.parseCreditLimits(headers, info)

	// Parse reset time (OpenRouter uses milliseconds since epoch)
	p.parseResetTime(headers, info)

	// Parse optional request and token limits
	p.parseOptionalLimits(headers, info)

	// Detect free tier
	p.detectFreeTier(headers, info)

	// Parse optional metadata
	p.parseOptionalMetadata(headers, info)

	return info, nil
}

// ProviderName returns "openrouter" as the provider identifier.
func (p *OpenRouterParser) ProviderName() string {
	return "openrouter"
}

// NewOpenRouterParser creates a new OpenRouter rate limit parser.
func NewOpenRouterParser() *OpenRouterParser {
	return &OpenRouterParser{}
}

// =============================================================================
// Helper Methods for Rate Limit Parsing
// =============================================================================

// parseCreditLimits parses credit-based rate limit headers
func (p *OpenRouterParser) parseCreditLimits(headers http.Header, info *Info) {
	// Parse the primary rate limit (could be credits or requests)
	if limit := headers.Get("x-ratelimit-limit"); limit != "" {
		if val, err := strconv.ParseFloat(limit, 64); err == nil {
			p.setCreditOrRequestLimit(info, val)
		}
	}

	if remaining := headers.Get("x-ratelimit-remaining"); remaining != "" {
		if val, err := strconv.ParseFloat(remaining, 64); err == nil {
			p.setCreditOrRequestRemaining(info, val)
		}
	}
}

// setCreditOrRequestLimit sets credit or request limit based on value type
func (p *OpenRouterParser) setCreditOrRequestLimit(info *Info, val float64) {
	// If it has a decimal component, it's credits
	if val != float64(int(val)) {
		info.CreditsLimit = val
	} else {
		// Integer value could be either credits or requests
		info.CreditsLimit = val
		info.RequestsLimit = int(val)
	}
}

// setCreditOrRequestRemaining sets credit or request remaining based on value type
func (p *OpenRouterParser) setCreditOrRequestRemaining(info *Info, val float64) {
	// If it has a decimal component, it's credits
	if val != float64(int(val)) {
		info.CreditsRemaining = val
	} else {
		// Integer value could be either credits or requests
		info.CreditsRemaining = val
		info.RequestsRemaining = int(val)
	}
}

// parseResetTime parses the reset time from headers (milliseconds since epoch)
func (p *OpenRouterParser) parseResetTime(headers http.Header, info *Info) {
	if reset := headers.Get("x-ratelimit-reset"); reset != "" {
		if ms, err := strconv.ParseInt(reset, 10, 64); err == nil {
			resetTime := time.Unix(0, ms*int64(time.Millisecond))
			info.RequestsReset = resetTime
			info.TokensReset = resetTime // Use same reset for both
		}
	}
}

// parseOptionalLimits parses optional request and token-based limits
func (p *OpenRouterParser) parseOptionalLimits(headers http.Header, info *Info) {
	// Parse optional request-based rate limits
	if requests := headers.Get("x-ratelimit-requests"); requests != "" {
		if val, err := strconv.Atoi(requests); err == nil {
			info.RequestsLimit = val
		}
	}

	// Parse optional token-based rate limits
	if tokens := headers.Get("x-ratelimit-tokens"); tokens != "" {
		if val, err := strconv.Atoi(tokens); err == nil {
			info.TokensLimit = val
		}
	}
}

// detectFreeTier determines if this is a free tier account
func (p *OpenRouterParser) detectFreeTier(headers http.Header, info *Info) {
	// Free tier typically has lower credits
	if info.CreditsLimit > 0 && info.CreditsLimit <= 10.0 {
		info.IsFreeTier = true
	}

	// Check for explicit free tier indicators in headers
	if freeTier := headers.Get("x-ratelimit-free-tier"); freeTier != "" {
		if val, err := strconv.ParseBool(freeTier); err == nil {
			info.IsFreeTier = val
		}
	}
}

// parseOptionalMetadata parses optional metadata like request ID and retry-after
func (p *OpenRouterParser) parseOptionalMetadata(headers http.Header, info *Info) {
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
}
