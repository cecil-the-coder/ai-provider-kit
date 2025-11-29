// Package http provides HTTP client utilities and helpers for AI providers.
package http

import (
	"time"
)

// BackoffConfig configures exponential backoff behavior
type BackoffConfig struct {
	BaseDelay   time.Duration // Initial delay for the first retry
	MaxDelay    time.Duration // Maximum delay cap
	Multiplier  float64       // Exponential multiplier (typically 2.0)
	MaxAttempts int           // Maximum number of retry attempts
}

// DefaultBackoffConfig returns sensible defaults for exponential backoff
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		BaseDelay:   1 * time.Second,
		MaxDelay:    60 * time.Second,
		Multiplier:  2.0,
		MaxAttempts: 3,
	}
}

// CalculateBackoff returns the delay for a given attempt number using exponential backoff
// attempt is 1-indexed (first retry is attempt 1)
func CalculateBackoff(config BackoffConfig, attempt int) time.Duration {
	if attempt <= 0 {
		return config.BaseDelay
	}

	// Safe bit shifting to prevent overflow
	if attempt > 30 { // 1 << 30 would overflow int32
		attempt = 30
	}

	// Calculate exponential backoff: baseDelay * multiplier * (2^(attempt-1))
	multiplier := float64(int(1)<<uint(attempt-1)) * config.Multiplier // #nosec G115 -- attempt is capped at 30, safe conversion
	delay := time.Duration(float64(config.BaseDelay) * multiplier)

	// Cap at maximum delay
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}
