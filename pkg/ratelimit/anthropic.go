package ratelimit

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// AnthropicParser implements the Parser interface for Anthropic's rate limit headers.
// Anthropic uses RFC 3339 timestamps for reset times and tracks input/output tokens separately.
//
// Header format:
//   - anthropic-ratelimit-requests-limit: Maximum requests allowed
//   - anthropic-ratelimit-requests-remaining: Requests remaining
//   - anthropic-ratelimit-requests-reset: RFC 3339 timestamp when limit resets
//   - anthropic-ratelimit-tokens-limit: Maximum total tokens allowed
//   - anthropic-ratelimit-tokens-remaining: Total tokens remaining
//   - anthropic-ratelimit-tokens-reset: RFC 3339 timestamp when token limit resets
//   - anthropic-ratelimit-input-tokens-limit: Maximum input tokens allowed
//   - anthropic-ratelimit-input-tokens-remaining: Input tokens remaining
//   - anthropic-ratelimit-input-tokens-reset: RFC 3339 timestamp when input token limit resets
//   - anthropic-ratelimit-output-tokens-limit: Maximum output tokens allowed
//   - anthropic-ratelimit-output-tokens-remaining: Output tokens remaining
//   - anthropic-ratelimit-output-tokens-reset: RFC 3339 timestamp when output token limit resets
//   - request-id: Unique request identifier
//   - retry-after: Seconds to wait before retrying (on 429 responses)
type AnthropicParser struct{}

// NewAnthropicParser creates a new Anthropic rate limit parser.
func NewAnthropicParser() *AnthropicParser {
	return &AnthropicParser{}
}

// Parse extracts rate limit information from Anthropic API response headers.
// It handles both standard token limits and separate input/output token limits.
// Missing headers are handled gracefully by leaving the corresponding fields at zero values.
func (p *AnthropicParser) Parse(headers http.Header, model string) (*Info, error) {
	info := &Info{
		Provider:  "anthropic",
		Model:     model,
		Timestamp: time.Now(),
	}

	// Parse different types of rate limits
	p.parseRequestLimits(headers, info)
	p.parseTokenLimits(headers, info)
	p.parseInputTokenLimits(headers, info)
	p.parseOutputTokenLimits(headers, info)
	p.parseAdditionalHeaders(headers, info)

	return info, nil
}

// parseRequestLimits parses request-based rate limit headers
func (p *AnthropicParser) parseRequestLimits(headers http.Header, info *Info) {
	p.parseIntHeader(headers, "anthropic-ratelimit-requests-limit", &info.RequestsLimit)
	p.parseIntHeader(headers, "anthropic-ratelimit-requests-remaining", &info.RequestsRemaining)
	p.parseTimeHeader(headers, "anthropic-ratelimit-requests-reset", &info.RequestsReset)
}

// parseTokenLimits parses aggregate token limit headers
func (p *AnthropicParser) parseTokenLimits(headers http.Header, info *Info) {
	p.parseIntHeader(headers, "anthropic-ratelimit-tokens-limit", &info.TokensLimit)
	p.parseIntHeader(headers, "anthropic-ratelimit-tokens-remaining", &info.TokensRemaining)
	p.parseTimeHeader(headers, "anthropic-ratelimit-tokens-reset", &info.TokensReset)
}

// parseInputTokenLimits parses input token limit headers (Anthropic-specific)
func (p *AnthropicParser) parseInputTokenLimits(headers http.Header, info *Info) {
	p.parseIntHeader(headers, "anthropic-ratelimit-input-tokens-limit", &info.InputTokensLimit)
	p.parseIntHeader(headers, "anthropic-ratelimit-input-tokens-remaining", &info.InputTokensRemaining)
	p.parseTimeHeader(headers, "anthropic-ratelimit-input-tokens-reset", &info.InputTokensReset)
}

// parseOutputTokenLimits parses output token limit headers (Anthropic-specific)
func (p *AnthropicParser) parseOutputTokenLimits(headers http.Header, info *Info) {
	p.parseIntHeader(headers, "anthropic-ratelimit-output-tokens-limit", &info.OutputTokensLimit)
	p.parseIntHeader(headers, "anthropic-ratelimit-output-tokens-remaining", &info.OutputTokensRemaining)
	p.parseTimeHeader(headers, "anthropic-ratelimit-output-tokens-reset", &info.OutputTokensReset)
}

// parseAdditionalHeaders parses request ID and retry-after headers
func (p *AnthropicParser) parseAdditionalHeaders(headers http.Header, info *Info) {
	// Parse request ID for debugging
	if val := headers.Get("request-id"); val != "" {
		info.RequestID = val
	}

	// Parse retry-after header (typically present on 429 responses)
	if val := headers.Get("retry-after"); val != "" {
		if seconds, err := strconv.Atoi(val); err == nil {
			info.RetryAfter = time.Duration(seconds) * time.Second
		}
	}
}

// parseIntHeader safely parses an integer header value
func (p *AnthropicParser) parseIntHeader(headers http.Header, headerName string, target *int) {
	if val := headers.Get(headerName); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			*target = parsed
		}
	}
}

// parseTimeHeader safely parses an RFC3339 time header value
func (p *AnthropicParser) parseTimeHeader(headers http.Header, headerName string, target *time.Time) {
	if val := headers.Get(headerName); val != "" {
		if parsed, err := time.Parse(time.RFC3339, val); err == nil {
			*target = parsed
		}
	}
}

// ProviderName returns "anthropic" as the provider identifier.
func (p *AnthropicParser) ProviderName() string {
	return "anthropic"
}

// ParseAndValidate is a convenience method that parses headers and validates
// that at least some rate limit information was found.
func (p *AnthropicParser) ParseAndValidate(headers http.Header, model string) (*Info, error) {
	info, err := p.Parse(headers, model)
	if err != nil {
		return nil, err
	}

	// Check if we got any meaningful rate limit data
	hasData := info.RequestsLimit > 0 ||
		info.TokensLimit > 0 ||
		info.InputTokensLimit > 0 ||
		info.OutputTokensLimit > 0 ||
		info.RequestID != ""

	if !hasData {
		return nil, fmt.Errorf("no rate limit information found in headers")
	}

	return info, nil
}
