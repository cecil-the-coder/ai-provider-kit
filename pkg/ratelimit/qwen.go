package ratelimit

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// QwenParser implements the Parser interface for Qwen's rate limit headers.
//
// Qwen (DashScope API) uses a combination of:
//  1. OpenAI-compatible headers in compatible-mode (x-ratelimit-*)
//  2. DashScope-specific headers (dashscope-*, x-dashscope-*)
//  3. Request tracking headers (x-request-id, req-cost-time)
//  4. Standard retry-after headers for rate limit recovery
//
// DISCOVERED HEADERS (through API testing):
//
//	x-request-id: Unique request identifier
//	req-cost-time: Request processing time in milliseconds
//	req-arrive-time: Request arrival timestamp
//	resp-start-time: Response start timestamp
//
// LIKELY RATE LIMIT HEADERS (based on OpenAI-compatible mode):
//
//	x-ratelimit-limit-requests: Maximum requests allowed per time window
//	x-ratelimit-remaining-requests: Requests remaining in current window
//	x-ratelimit-reset-requests: Duration until window resets
//	x-ratelimit-limit-tokens: Maximum tokens allowed per time window
//	x-ratelimit-remaining-tokens: Tokens remaining in current window
//	x-ratelimit-reset-tokens: Duration until token window resets
//	retry-after: Seconds to wait before retrying (on 429 responses)
//
// POTENTIAL DASHSCOPE-SPECIFIC HEADERS:
//
//	dashscope-ratelimit-*: Possible DashScope-specific rate limit headers
//	x-dashscope-*: Alternative DashScope header prefix
type QwenParser struct {
	// logHeaders enables logging of all rate limit headers for debugging
	logHeaders bool
}

// Parse extracts rate limit information from Qwen response headers.
// It attempts multiple parsing strategies to handle undocumented header formats:
//  1. Standard x-ratelimit-* headers (OpenAI-compatible)
//  2. Qwen-specific qwen-ratelimit-* headers
//  3. Retry-After header for backoff timing
//
// The implementation is deliberately flexible to accommodate various possible formats.
func (p *QwenParser) Parse(headers http.Header, model string) (*Info, error) {
	info := &Info{
		Provider:   "qwen",
		Model:      model,
		CustomData: make(map[string]interface{}),
	}

	// Log all headers if debugging is enabled (useful for documenting real API behavior)
	if p.logHeaders {
		p.logRateLimitHeaders(headers)
	}

	// Strategy 1: Try standard x-ratelimit-* headers (OpenAI-compatible format)
	p.parseStandardHeaders(headers, info)

	// Strategy 2: Try qwen-specific headers
	p.parseQwenHeaders(headers, info)

	// Strategy 3: Parse retry-after header
	p.parseRetryAfter(headers, info)

	// Strategy 4: Parse any custom headers we find
	p.parseCustomHeaders(headers, info)

	return info, nil
}

// parseStandardHeaders parses OpenAI-compatible x-ratelimit-* headers
func (p *QwenParser) parseStandardHeaders(headers http.Header, info *Info) {
	// Parse request-based rate limits
	if limit := headers.Get("x-ratelimit-limit-requests"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			info.RequestsLimit = val
			info.CustomData["x-ratelimit-limit-requests"] = limit
		}
	}

	if remaining := headers.Get("x-ratelimit-remaining-requests"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			info.RequestsRemaining = val
			info.CustomData["x-ratelimit-remaining-requests"] = remaining
		}
	}

	// Parse request reset - try multiple formats
	if reset := headers.Get("x-ratelimit-reset-requests"); reset != "" {
		info.RequestsReset = p.parseResetTime(reset)
		info.CustomData["x-ratelimit-reset-requests"] = reset
	}

	// Parse token-based rate limits
	if limit := headers.Get("x-ratelimit-limit-tokens"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			info.TokensLimit = val
			info.CustomData["x-ratelimit-limit-tokens"] = limit
		}
	}

	if remaining := headers.Get("x-ratelimit-remaining-tokens"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			info.TokensRemaining = val
			info.CustomData["x-ratelimit-remaining-tokens"] = remaining
		}
	}

	// Parse token reset - try multiple formats
	if reset := headers.Get("x-ratelimit-reset-tokens"); reset != "" {
		info.TokensReset = p.parseResetTime(reset)
		info.CustomData["x-ratelimit-reset-tokens"] = reset
	}
}

// parseQwenHeaders parses Qwen-specific headers (qwen-ratelimit-*)
func (p *QwenParser) parseQwenHeaders(headers http.Header, info *Info) {
	// Parse qwen-specific request limits
	if limit := headers.Get("qwen-ratelimit-limit-requests"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			// Only override if not already set
			if info.RequestsLimit == 0 {
				info.RequestsLimit = val
			}
			info.CustomData["qwen-ratelimit-limit-requests"] = limit
		}
	}

	if remaining := headers.Get("qwen-ratelimit-remaining-requests"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			if info.RequestsRemaining == 0 {
				info.RequestsRemaining = val
			}
			info.CustomData["qwen-ratelimit-remaining-requests"] = remaining
		}
	}

	if reset := headers.Get("qwen-ratelimit-reset-requests"); reset != "" {
		parsedTime := p.parseResetTime(reset)
		if !parsedTime.IsZero() && info.RequestsReset.IsZero() {
			info.RequestsReset = parsedTime
		}
		info.CustomData["qwen-ratelimit-reset-requests"] = reset
	}

	// Parse qwen-specific token limits
	if limit := headers.Get("qwen-ratelimit-limit-tokens"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			if info.TokensLimit == 0 {
				info.TokensLimit = val
			}
			info.CustomData["qwen-ratelimit-limit-tokens"] = limit
		}
	}

	if remaining := headers.Get("qwen-ratelimit-remaining-tokens"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			if info.TokensRemaining == 0 {
				info.TokensRemaining = val
			}
			info.CustomData["qwen-ratelimit-remaining-tokens"] = remaining
		}
	}

	if reset := headers.Get("qwen-ratelimit-reset-tokens"); reset != "" {
		parsedTime := p.parseResetTime(reset)
		if !parsedTime.IsZero() && info.TokensReset.IsZero() {
			info.TokensReset = parsedTime
		}
		info.CustomData["qwen-ratelimit-reset-tokens"] = reset
	}
}

// parseRetryAfter parses the Retry-After header
func (p *QwenParser) parseRetryAfter(headers http.Header, info *Info) {
	if retryAfter := headers.Get("retry-after"); retryAfter != "" {
		// Retry-After can be either seconds (integer) or HTTP date
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			info.RetryAfter = time.Duration(seconds) * time.Second
			info.CustomData["retry-after"] = retryAfter
		} else if httpDate, err := http.ParseTime(retryAfter); err == nil {
			info.RetryAfter = time.Until(httpDate)
			info.CustomData["retry-after"] = retryAfter
		}
	}
}

// parseCustomHeaders captures any other headers that might contain rate limit info
func (p *QwenParser) parseCustomHeaders(headers http.Header, info *Info) {
	p.parseRequestIDs(headers, info)
	p.parseDashScopeHeaders(headers, info)
	p.parseDashScopeRateLimits(headers, info)
	p.captureAdditionalQwenHeaders(headers, info)
}

// parseRequestIDs extracts request ID headers
func (p *QwenParser) parseRequestIDs(headers http.Header, info *Info) {
	// Capture standard request ID if present
	if requestID := headers.Get("x-request-id"); requestID != "" {
		info.RequestID = requestID
		info.CustomData["x-request-id"] = requestID
	}

	// Check for qwen-specific request ID
	if requestID := headers.Get("qwen-request-id"); requestID != "" {
		if info.RequestID == "" {
			info.RequestID = requestID
		}
		info.CustomData["qwen-request-id"] = requestID
	}
}

// parseDashScopeHeaders extracts DashScope-specific headers
func (p *QwenParser) parseDashScopeHeaders(headers http.Header, info *Info) {
	dashScopeHeaders := []string{
		"req-cost-time",
		"req-arrive-time",
		"resp-start-time",
	}

	for _, header := range dashScopeHeaders {
		if value := headers.Get(header); value != "" {
			if header == "req-cost-time" {
				if costTime, err := strconv.ParseInt(value, 10, 64); err == nil {
					info.CustomData["req-cost-time-ms"] = costTime
				}
			}
			info.CustomData[header] = value
		}
	}
}

// parseDashScopeRateLimits extracts and parses DashScope rate limit headers
func (p *QwenParser) parseDashScopeRateLimits(headers http.Header, info *Info) {
	for key := range headers {
		lowerKey := strings.ToLower(key)
		if !p.isDashScopeRateLimitHeader(lowerKey) {
			continue
		}

		value := headers.Get(key)
		info.CustomData[key] = value

		p.tryParseDashScopeRateLimitValue(lowerKey, value, info)
	}
}

// isDashScopeRateLimitHeader checks if a header is a DashScope rate limit header
func (p *QwenParser) isDashScopeRateLimitHeader(lowerKey string) bool {
	return strings.HasPrefix(lowerKey, "dashscope-ratelimit-") ||
		strings.HasPrefix(lowerKey, "x-dashscope-ratelimit-")
}

// tryParseDashScopeRateLimitValue attempts to parse rate limit values from DashScope headers
func (p *QwenParser) tryParseDashScopeRateLimitValue(lowerKey, value string, info *Info) {
	// Only parse headers that end with -requests or -tokens
	if !strings.HasSuffix(lowerKey, "-requests") && !strings.HasSuffix(lowerKey, "-tokens") {
		return
	}

	val, err := strconv.Atoi(value)
	if err != nil {
		return
	}

	if strings.Contains(lowerKey, "-limit-") {
		p.updateRateLimitFromDashScopeHeader(lowerKey, val, info, true)
	} else if strings.Contains(lowerKey, "-remaining-") {
		p.updateRateLimitFromDashScopeHeader(lowerKey, val, info, false)
	}
}

// updateRateLimitFromDashScopeHeader updates rate limit info from DashScope header
func (p *QwenParser) updateRateLimitFromDashScopeHeader(lowerKey string, val int, info *Info, isLimit bool) {
	if strings.Contains(lowerKey, "-requests") {
		if isLimit {
			if info.RequestsLimit == 0 {
				info.RequestsLimit = val
			}
		} else {
			if info.RequestsRemaining == 0 {
				info.RequestsRemaining = val
			}
		}
	} else if strings.Contains(lowerKey, "-tokens") {
		if isLimit {
			if info.TokensLimit == 0 {
				info.TokensLimit = val
			}
		} else {
			if info.TokensRemaining == 0 {
				info.TokensRemaining = val
			}
		}
	}
}

// captureAdditionalQwenHeaders captures any other qwen-* headers for future reference
func (p *QwenParser) captureAdditionalQwenHeaders(headers http.Header, info *Info) {
	for key := range headers {
		if p.isQwenHeader(key) {
			value := headers.Get(key)
			info.CustomData[key] = value
		}
	}
}

// isQwenHeader checks if a header is a qwen header
func (p *QwenParser) isQwenHeader(key string) bool {
	return len(key) > 5 && strings.ToLower(key[:5]) == "qwen-"
}

// parseResetTime attempts to parse a reset time from various formats:
//  1. Duration string (e.g., "60s", "5m30s")
//  2. Integer seconds
//  3. Unix timestamp
func (p *QwenParser) parseResetTime(reset string) time.Time {
	// Try parsing as duration string first (e.g., "60s", "5m")
	if duration, err := time.ParseDuration(reset); err == nil {
		return time.Now().Add(duration)
	}

	// Try parsing as integer seconds
	if seconds, err := strconv.ParseInt(reset, 10, 64); err == nil {
		// If it's a small number, treat it as seconds from now
		// If it's a large number (> 1000000000), treat it as unix timestamp
		if seconds < 1000000000 {
			return time.Now().Add(time.Duration(seconds) * time.Second)
		}
		return time.Unix(seconds, 0)
	}

	// Try parsing as RFC3339 timestamp
	if t, err := time.Parse(time.RFC3339, reset); err == nil {
		return t
	}

	// Unable to parse, return zero time
	return time.Time{}
}

// logRateLimitHeaders logs all rate limit related headers for debugging
func (p *QwenParser) logRateLimitHeaders(headers http.Header) {
	log.Println("=== Qwen/DashScope Rate Limit Headers (for documentation) ===")
	for key := range headers {
		lowerKey := strings.ToLower(key)
		// Log headers that might contain rate limit info
		if strings.HasPrefix(lowerKey, "x-rat") ||
			strings.HasPrefix(lowerKey, "qwen-") ||
			strings.HasPrefix(lowerKey, "dashscope") ||
			lowerKey == "retry-after" ||
			lowerKey == "x-request-id" ||
			lowerKey == "req-cost-time" ||
			lowerKey == "req-arrive-time" ||
			lowerKey == "resp-start-time" {
			log.Printf("%s: %s", key, headers.Get(key))
		}
	}
	log.Println("=== End Qwen/DashScope Rate Limit Headers ===")
}

// ProviderName returns "qwen" as the provider identifier.
func (p *QwenParser) ProviderName() string {
	return "qwen"
}

// NewQwenParser creates a new Qwen rate limit parser.
// Set logHeaders to true to enable header logging for debugging and documentation.
func NewQwenParser(logHeaders bool) *QwenParser {
	return &QwenParser{
		logHeaders: logHeaders,
	}
}

// FormatQwenInfo formats rate limit info for human-readable display.
// This is useful for logging and debugging.
func FormatQwenInfo(info *Info) string {
	if info == nil {
		return "No rate limit info available"
	}

	result := fmt.Sprintf("Qwen Rate Limits for model '%s':\n", info.Model)

	if info.RequestsLimit > 0 {
		result += fmt.Sprintf("  Requests: %d/%d remaining", info.RequestsRemaining, info.RequestsLimit)
		if !info.RequestsReset.IsZero() {
			result += fmt.Sprintf(" (resets in %v)", time.Until(info.RequestsReset).Round(time.Second))
		}
		result += "\n"
	}

	if info.TokensLimit > 0 {
		result += fmt.Sprintf("  Tokens: %d/%d remaining", info.TokensRemaining, info.TokensLimit)
		if !info.TokensReset.IsZero() {
			result += fmt.Sprintf(" (resets in %v)", time.Until(info.TokensReset).Round(time.Second))
		}
		result += "\n"
	}

	if info.RetryAfter > 0 {
		result += fmt.Sprintf("  Retry After: %v\n", info.RetryAfter.Round(time.Second))
	}

	if len(info.CustomData) > 0 {
		result += "  Custom Headers:\n"
		for key, value := range info.CustomData {
			result += fmt.Sprintf("    %s: %v\n", key, value)
		}
	}

	return result
}
