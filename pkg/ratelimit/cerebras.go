package ratelimit

import (
	"net/http"
	"strconv"
	"time"
)

// CerebrasParser implements the Parser interface for Cerebras API rate limits.
// Cerebras tracks both daily requests and per-minute limits for requests and tokens.
//
// Cerebras Rate Limit Headers:
//   - x-ratelimit-limit-requests-day: Daily request limit
//   - x-ratelimit-remaining-requests-day: Remaining daily requests
//   - x-ratelimit-reset-requests-day: Daily limit reset time (float seconds)
//   - x-ratelimit-limit-requests-minute: Per-minute request limit
//   - x-ratelimit-remaining-requests-minute: Remaining per-minute requests
//   - x-ratelimit-reset-requests-minute: Per-minute limit reset time (float seconds)
//   - x-ratelimit-limit-tokens-minute: Per-minute token limit
//   - x-ratelimit-remaining-tokens-minute: Remaining per-minute tokens
//   - x-ratelimit-reset-tokens-minute: Per-minute token reset time (float seconds)
//
// Custom Headers:
//   - cerebras-request-id: Unique request identifier
//   - cerebras-processing-time: Processing time in seconds
//   - cerebras-region: Data center region
type CerebrasParser struct{}

// Parse extracts rate limit information from Cerebras API response headers.
// The model parameter is stored in the Info but doesn't affect parsing logic.
//
// Note: Cerebras uses FLOAT SECONDS for reset times (e.g., "33011.382867"),
// which are converted to absolute time.Time values by adding to time.Now().
func (p *CerebrasParser) Parse(headers http.Header, model string) (*Info, error) {
	info := &Info{
		Provider:   "cerebras",
		Model:      model,
		CustomData: make(map[string]interface{}),
	}

	// Parse per-minute request limits (used as standard request fields)
	p.parseLimitHeader(headers, info, "requests-minute",
		func(val int) { info.RequestsLimit = val },
		func(val int) { info.RequestsRemaining = val },
		func() { info.RequestsReset = parseFloatSecondsToTime(headers.Get("x-ratelimit-reset-requests-minute")) })

	// Parse per-minute token limits
	p.parseLimitHeader(headers, info, "tokens-minute",
		func(val int) { info.TokensLimit = val },
		func(val int) { info.TokensRemaining = val },
		func() { info.TokensReset = parseFloatSecondsToTime(headers.Get("x-ratelimit-reset-tokens-minute")) })

	// Parse daily request limits
	p.parseLimitHeader(headers, info, "requests-day",
		func(val int) { info.DailyRequestsLimit = val },
		func(val int) { info.DailyRequestsRemaining = val },
		func() {
			info.DailyRequestsReset = parseFloatSecondsToTime(headers.Get("x-ratelimit-reset-requests-day"))
		})

	// Parse custom Cerebras headers
	p.parseCustomHeaders(headers, info)

	return info, nil
}

// ProviderName returns "cerebras" as the provider identifier.
func (p *CerebrasParser) ProviderName() string {
	return "cerebras"
}

// parseFloatSecondsToTime converts a float seconds string (e.g., "33011.382867")
// to an absolute time.Time by parsing it as float64 and adding to time.Now().
// Returns zero time if parsing fails.
func parseFloatSecondsToTime(resetStr string) time.Time {
	if resetStr == "" {
		return time.Time{}
	}

	seconds, err := strconv.ParseFloat(resetStr, 64)
	if err != nil {
		return time.Time{}
	}

	duration := time.Duration(seconds * float64(time.Second))
	return time.Now().Add(duration)
}

// =============================================================================
// Helper Functions for Cerebras Rate Limit Parsing
// =============================================================================

// parseLimitHeader parses a rate limit header with limit, remaining, and reset components
func (p *CerebrasParser) parseLimitHeader(headers http.Header, _ *Info, suffix string,
	setLimit, setRemaining func(int), setReset func()) {

	// Parse limit
	limitHeader := "x-ratelimit-limit-" + suffix
	if limit := headers.Get(limitHeader); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			setLimit(val)
		}
	}

	// Parse remaining
	remainingHeader := "x-ratelimit-remaining-" + suffix
	if remaining := headers.Get(remainingHeader); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			setRemaining(val)
		}
	}

	// Parse reset
	setReset()
}

// parseCustomHeaders parses custom Cerebras-specific headers
func (p *CerebrasParser) parseCustomHeaders(headers http.Header, info *Info) {
	// Parse request ID
	if requestID := headers.Get("cerebras-request-id"); requestID != "" {
		info.CustomData["request_id"] = requestID
	}

	// Parse processing time
	if processingTime := headers.Get("cerebras-processing-time"); processingTime != "" {
		info.CustomData["processing_time"] = processingTime
	}

	// Parse region
	if region := headers.Get("cerebras-region"); region != "" {
		info.CustomData["region"] = region
	}
}
