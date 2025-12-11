package retry

import (
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy defines the configuration for retry behavior
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts (0 means no retries)
	MaxRetries int

	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the factor by which the delay increases between retries
	// For exponential backoff, this is typically 2.0
	Multiplier float64

	// Jitter adds randomness to delays to prevent thundering herd
	// Range: 0.0 (no jitter) to 1.0 (full jitter)
	Jitter float64

	// RetryableStatusCodes is a custom set of retryable HTTP status codes
	// If nil, the default set will be used
	RetryableStatusCodes map[int]bool

	// RetryableErrors is a custom set of error types that should trigger retries
	// If nil, the default RetryableError interface check will be used
	RetryableErrors []error
}

// DefaultRetryPolicy returns a retry policy with sensible defaults
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1, // 10% jitter
		// Use default retryable status codes (nil means use package defaults)
		RetryableStatusCodes: nil,
		RetryableErrors:      nil,
	}
}

// NoRetryPolicy returns a policy that never retries
func NoRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:   0,
		InitialDelay: 0,
		MaxDelay:     0,
		Multiplier:   1.0,
		Jitter:       0.0,
	}
}

// AggressiveRetryPolicy returns a policy with more retry attempts and shorter delays
// Useful for testing or services with very high availability
func AggressiveRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:   5,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   1.5,
		Jitter:       0.2, // 20% jitter
	}
}

// ConservativeRetryPolicy returns a policy with fewer retries and longer delays
// Useful for rate-limited APIs or to avoid overwhelming services
func ConservativeRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:   2,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
		Jitter:       0.05, // 5% jitter
	}
}

// ShouldRetry determines if an error should trigger a retry attempt
// It checks the attempt number, error type, and status code
func (p *RetryPolicy) ShouldRetry(err error, attempt int) bool {
	// Check if we've exceeded max retries
	if attempt >= p.MaxRetries {
		return false
	}

	// If no error, don't retry
	if err == nil {
		return false
	}

	// Check if error is retryable using the interface
	if IsRetryableError(err) {
		return true
	}

	// Check status code if available
	statusCode := GetStatusCode(err)
	if statusCode > 0 {
		return p.isRetryableStatusCode(statusCode)
	}

	// Default to not retrying if we can't determine retryability
	return false
}

// isRetryableStatusCode checks if a status code is retryable
func (p *RetryPolicy) isRetryableStatusCode(statusCode int) bool {
	// Use custom status codes if provided
	if p.RetryableStatusCodes != nil {
		return p.RetryableStatusCodes[statusCode]
	}

	// Otherwise use package defaults
	return IsRetryableStatusCode(statusCode)
}

// ParseRetryAfter parses the Retry-After header from an HTTP response
// It supports both delay-seconds (integer) and HTTP-date formats
// Returns the duration to wait before retrying, or 0 if not present/invalid
func ParseRetryAfter(headers http.Header) time.Duration {
	retryAfter := headers.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try to parse as delay-seconds (integer)
	if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil {
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		return 0
	}

	// Try to parse as HTTP-date
	if t, err := http.ParseTime(retryAfter); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return duration
		}
		return 0
	}

	// Unable to parse, return 0
	return 0
}

// GetRetryDelay calculates the retry delay for a given attempt
// It respects the Retry-After header if present, otherwise uses backoff strategy
func (p *RetryPolicy) GetRetryDelay(attempt int, headers http.Header) time.Duration {
	// Check for Retry-After header first
	if retryAfter := ParseRetryAfter(headers); retryAfter > 0 {
		// Cap at MaxDelay if configured
		if p.MaxDelay > 0 && retryAfter > p.MaxDelay {
			return p.MaxDelay
		}
		return retryAfter
	}

	// No Retry-After header, use backoff strategy
	// This will be delegated to the strategy, but we provide a basic calculation
	// for policies used without a strategy
	delay := p.InitialDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * p.Multiplier)
		if p.MaxDelay > 0 && delay > p.MaxDelay {
			delay = p.MaxDelay
			break
		}
	}

	return delay
}

// Clone creates a copy of the retry policy
func (p *RetryPolicy) Clone() *RetryPolicy {
	clone := &RetryPolicy{
		MaxRetries:   p.MaxRetries,
		InitialDelay: p.InitialDelay,
		MaxDelay:     p.MaxDelay,
		Multiplier:   p.Multiplier,
		Jitter:       p.Jitter,
	}

	// Deep copy status codes map if present
	if p.RetryableStatusCodes != nil {
		clone.RetryableStatusCodes = make(map[int]bool, len(p.RetryableStatusCodes))
		for k, v := range p.RetryableStatusCodes {
			clone.RetryableStatusCodes[k] = v
		}
	}

	// Copy error slice if present
	if p.RetryableErrors != nil {
		clone.RetryableErrors = make([]error, len(p.RetryableErrors))
		copy(clone.RetryableErrors, p.RetryableErrors)
	}

	return clone
}

// WithMaxRetries returns a new policy with updated MaxRetries
func (p *RetryPolicy) WithMaxRetries(maxRetries int) *RetryPolicy {
	clone := p.Clone()
	clone.MaxRetries = maxRetries
	return clone
}

// WithInitialDelay returns a new policy with updated InitialDelay
func (p *RetryPolicy) WithInitialDelay(delay time.Duration) *RetryPolicy {
	clone := p.Clone()
	clone.InitialDelay = delay
	return clone
}

// WithMaxDelay returns a new policy with updated MaxDelay
func (p *RetryPolicy) WithMaxDelay(delay time.Duration) *RetryPolicy {
	clone := p.Clone()
	clone.MaxDelay = delay
	return clone
}

// WithMultiplier returns a new policy with updated Multiplier
func (p *RetryPolicy) WithMultiplier(multiplier float64) *RetryPolicy {
	clone := p.Clone()
	clone.Multiplier = multiplier
	return clone
}

// WithJitter returns a new policy with updated Jitter
func (p *RetryPolicy) WithJitter(jitter float64) *RetryPolicy {
	clone := p.Clone()
	clone.Jitter = jitter
	return clone
}
