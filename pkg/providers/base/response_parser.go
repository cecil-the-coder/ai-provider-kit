// Package base provides common functionality and utilities for AI providers.
package base

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
)

// RateLimitInfo contains rate limit information extracted from HTTP headers
type RateLimitInfo struct {
	RequestsRemaining int
	RequestsLimit     int
	RequestsReset     time.Time
	TokensRemaining   int
	TokensLimit       int
	TokensReset       time.Time
	RetryAfter        time.Duration
}

// ResponseParser interface for parsing HTTP responses
// Provides a unified way to handle API responses across different providers
type ResponseParser interface {
	// ParseJSON reads and unmarshals JSON response body into target
	ParseJSON(resp *http.Response, target interface{}) error

	// ParseError extracts error information from HTTP response
	ParseError(resp *http.Response) error

	// ReadBody reads the entire response body and returns it as a string
	ReadBody(resp *http.Response) (string, error)

	// CheckStatusCode validates HTTP status code and returns error if not OK
	CheckStatusCode(resp *http.Response) error
}

// DefaultResponseParser provides a default implementation of ResponseParser
type DefaultResponseParser struct {
	rateLimitHelper *common.RateLimitHelper
}

// NewDefaultResponseParser creates a new DefaultResponseParser
func NewDefaultResponseParser(rateLimitHelper *common.RateLimitHelper) *DefaultResponseParser {
	return &DefaultResponseParser{
		rateLimitHelper: rateLimitHelper,
	}
}

// ParseJSON reads and unmarshals JSON response body into target
func (p *DefaultResponseParser) ParseJSON(resp *http.Response, target interface{}) error {
	if resp == nil {
		return fmt.Errorf("response is nil")
	}

	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// ParseError extracts error information from HTTP response
func (p *DefaultResponseParser) ParseError(resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("response is nil")
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil // Not an error
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("HTTP %d - failed to read error response: %w", resp.StatusCode, err)
	}

	// Try to parse as JSON error
	var errorData map[string]interface{}
	if json.Unmarshal(body, &errorData) == nil {
		// Extract error message from common patterns
		if errMsg, ok := errorData["error"].(map[string]interface{}); ok {
			if msg, ok := errMsg["message"].(string); ok {
				return fmt.Errorf("HTTP %d - %s", resp.StatusCode, msg)
			}
		}
		if errMsg, ok := errorData["error"].(string); ok {
			return fmt.Errorf("HTTP %d - %s", resp.StatusCode, errMsg)
		}
		if msg, ok := errorData["message"].(string); ok {
			return fmt.Errorf("HTTP %d - %s", resp.StatusCode, msg)
		}
	}

	// Return raw body if can't parse as JSON
	return fmt.Errorf("HTTP %d - %s", resp.StatusCode, string(body))
}

// ReadBody reads the entire response body and returns it as a string
func (p *DefaultResponseParser) ReadBody(resp *http.Response) (string, error) {
	if resp == nil {
		return "", fmt.Errorf("response is nil")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// CheckStatusCode validates HTTP status code and returns error if not OK
func (p *DefaultResponseParser) CheckStatusCode(resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("response is nil")
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return p.ParseError(resp)
}

// ExtractRateLimits extracts rate limit information from response headers
// This is a helper method that uses the RateLimitHelper if available
func (p *DefaultResponseParser) ExtractRateLimits(resp *http.Response, model string) *RateLimitInfo {
	if resp == nil || p.rateLimitHelper == nil {
		return nil
	}

	// Parse and update rate limits using the helper
	p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, model)

	// Get the updated info
	info, exists := p.rateLimitHelper.GetRateLimitInfo(model)
	if !exists {
		return nil
	}

	return convertToRateLimitInfo(info)
}

// convertToRateLimitInfo converts ratelimit.Info to RateLimitInfo
func convertToRateLimitInfo(info *ratelimit.Info) *RateLimitInfo {
	if info == nil {
		return nil
	}

	return &RateLimitInfo{
		RequestsRemaining: info.RequestsRemaining,
		RequestsLimit:     info.RequestsLimit,
		RequestsReset:     info.RequestsReset,
		TokensRemaining:   info.TokensRemaining,
		TokensLimit:       info.TokensLimit,
		TokensReset:       info.TokensReset,
		RetryAfter:        info.RetryAfter,
	}
}

// ParseRateLimitHeaders extracts rate limit info directly from headers (static helper)
// This is a fallback for cases where RateLimitHelper is not available
func ParseRateLimitHeaders(headers http.Header) *RateLimitInfo {
	info := &RateLimitInfo{}

	// Parse common rate limit headers
	if val := headers.Get("X-RateLimit-Remaining-Requests"); val != "" {
		if num, err := strconv.Atoi(val); err == nil {
			info.RequestsRemaining = num
		}
	}
	if val := headers.Get("X-RateLimit-Limit-Requests"); val != "" {
		if num, err := strconv.Atoi(val); err == nil {
			info.RequestsLimit = num
		}
	}
	if val := headers.Get("X-RateLimit-Remaining-Tokens"); val != "" {
		if num, err := strconv.Atoi(val); err == nil {
			info.TokensRemaining = num
		}
	}
	if val := headers.Get("X-RateLimit-Limit-Tokens"); val != "" {
		if num, err := strconv.Atoi(val); err == nil {
			info.TokensLimit = num
		}
	}

	// Parse reset times
	if val := headers.Get("X-RateLimit-Reset-Requests"); val != "" {
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			info.RequestsReset = t
		}
	}
	if val := headers.Get("X-RateLimit-Reset-Tokens"); val != "" {
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			info.TokensReset = t
		}
	}

	// Parse retry-after
	if val := headers.Get("Retry-After"); val != "" {
		if seconds, err := strconv.Atoi(val); err == nil {
			info.RetryAfter = time.Duration(seconds) * time.Second
		}
	}

	return info
}
