package retry

import (
	"math"
	"math/rand"
	"net/http"
	"time"
)

// JitterType defines different types of jitter strategies
type JitterType int

const (
	// NoJitter applies no randomization to the delay
	NoJitter JitterType = iota

	// FullJitter randomizes the delay between 0 and the calculated delay
	// delay = random(0, calculatedDelay)
	FullJitter

	// EqualJitter splits the delay evenly between fixed and random components
	// delay = calculatedDelay/2 + random(0, calculatedDelay/2)
	EqualJitter

	// DecorrelatedJitter uses AWS's decorrelated jitter algorithm
	// delay = min(maxDelay, random(initialDelay, previousDelay * 3))
	DecorrelatedJitter
)

// BackoffStrategy is the interface that all backoff strategies must implement
type BackoffStrategy interface {
	// NextDelay calculates the next retry delay based on attempt number and error
	NextDelay(attempt int, err error) time.Duration

	// Reset resets the strategy's internal state (useful for decorrelated jitter)
	Reset()
}

// ExponentialBackoffStrategy implements exponential backoff with configurable jitter
type ExponentialBackoffStrategy struct {
	policy        *RetryPolicy
	jitterType    JitterType
	rng           *rand.Rand
	previousDelay time.Duration // For decorrelated jitter
}

// NewExponentialBackoffStrategy creates a new exponential backoff strategy
func NewExponentialBackoffStrategy(policy *RetryPolicy) *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{
		policy:        policy,
		jitterType:    EqualJitter,                                     // Default to equal jitter
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec // G404: math/rand is sufficient for jitter
		previousDelay: 0,
	}
}

// WithJitterType sets the jitter type for the strategy
func (s *ExponentialBackoffStrategy) WithJitterType(jitterType JitterType) *ExponentialBackoffStrategy {
	s.jitterType = jitterType
	return s
}

// NextDelay calculates the next delay using exponential backoff
func (s *ExponentialBackoffStrategy) NextDelay(attempt int, err error) time.Duration {
	// Check for Retry-After header if error provides HTTP headers
	if headers := extractHeaders(err); headers != nil {
		if retryAfter := ParseRetryAfter(headers); retryAfter > 0 {
			s.previousDelay = retryAfter
			return retryAfter
		}
	}

	// Calculate base exponential delay
	delay := s.policy.InitialDelay
	if attempt > 0 {
		// Calculate exponential backoff: initialDelay * multiplier^attempt
		multiplier := math.Pow(s.policy.Multiplier, float64(attempt))
		delay = time.Duration(float64(s.policy.InitialDelay) * multiplier)
	}

	// Cap at max delay
	if s.policy.MaxDelay > 0 && delay > s.policy.MaxDelay {
		delay = s.policy.MaxDelay
	}

	// Apply jitter
	delay = s.applyJitter(delay, attempt)

	// Store for decorrelated jitter
	s.previousDelay = delay

	return delay
}

// applyJitter applies the configured jitter type to the delay
func (s *ExponentialBackoffStrategy) applyJitter(delay time.Duration, attempt int) time.Duration {
	if s.policy.Jitter == 0 {
		return delay
	}

	switch s.jitterType {
	case NoJitter:
		return delay

	case FullJitter:
		// Random delay between 0 and calculated delay
		maxJitter := float64(delay)
		return time.Duration(s.rng.Float64() * maxJitter)

	case EqualJitter:
		// Half fixed, half random
		halfDelay := delay / 2
		maxJitter := float64(delay - halfDelay)
		randomJitter := time.Duration(s.rng.Float64() * maxJitter)
		return halfDelay + randomJitter

	case DecorrelatedJitter:
		// AWS decorrelated jitter: random between initialDelay and previousDelay * 3
		if s.previousDelay == 0 || attempt == 0 {
			s.previousDelay = s.policy.InitialDelay
		}
		minDelay := float64(s.policy.InitialDelay)
		maxDelay := float64(s.previousDelay * 3)
		if s.policy.MaxDelay > 0 && time.Duration(maxDelay) > s.policy.MaxDelay {
			maxDelay = float64(s.policy.MaxDelay)
		}
		randomDelay := minDelay + s.rng.Float64()*(maxDelay-minDelay)
		return time.Duration(randomDelay)

	default:
		// Fallback to simple jitter based on policy.Jitter factor
		jitterAmount := float64(delay) * s.policy.Jitter
		randomJitter := time.Duration(s.rng.Float64() * jitterAmount)
		return delay - randomJitter/2 + randomJitter
	}
}

// Reset resets the strategy's internal state
func (s *ExponentialBackoffStrategy) Reset() {
	s.previousDelay = 0
}

// ConstantBackoffStrategy implements a constant delay between retries
type ConstantBackoffStrategy struct {
	delay time.Duration
}

// NewConstantBackoffStrategy creates a new constant backoff strategy
func NewConstantBackoffStrategy(delay time.Duration) *ConstantBackoffStrategy {
	return &ConstantBackoffStrategy{
		delay: delay,
	}
}

// NextDelay returns a constant delay regardless of attempt number
func (s *ConstantBackoffStrategy) NextDelay(attempt int, err error) time.Duration {
	// Check for Retry-After header if error provides HTTP headers
	if headers := extractHeaders(err); headers != nil {
		if retryAfter := ParseRetryAfter(headers); retryAfter > 0 {
			return retryAfter
		}
	}

	return s.delay
}

// Reset resets the strategy (no-op for constant backoff)
func (s *ConstantBackoffStrategy) Reset() {
	// No state to reset
}

// LinearBackoffStrategy implements a linear increase in delay
type LinearBackoffStrategy struct {
	initialDelay time.Duration
	increment    time.Duration
	maxDelay     time.Duration
}

// NewLinearBackoffStrategy creates a new linear backoff strategy
func NewLinearBackoffStrategy(initialDelay, increment, maxDelay time.Duration) *LinearBackoffStrategy {
	return &LinearBackoffStrategy{
		initialDelay: initialDelay,
		increment:    increment,
		maxDelay:     maxDelay,
	}
}

// NextDelay calculates the next delay using linear backoff
func (s *LinearBackoffStrategy) NextDelay(attempt int, err error) time.Duration {
	// Check for Retry-After header if error provides HTTP headers
	if headers := extractHeaders(err); headers != nil {
		if retryAfter := ParseRetryAfter(headers); retryAfter > 0 {
			return retryAfter
		}
	}

	// Calculate linear backoff: initialDelay + (increment * attempt)
	delay := s.initialDelay + time.Duration(attempt)*s.increment

	// Cap at max delay
	if s.maxDelay > 0 && delay > s.maxDelay {
		delay = s.maxDelay
	}

	return delay
}

// Reset resets the strategy (no-op for linear backoff)
func (s *LinearBackoffStrategy) Reset() {
	// No state to reset
}

// extractHeaders attempts to extract HTTP headers from an error
// This is a helper to check for Retry-After headers in errors
func extractHeaders(err error) http.Header {
	if err == nil {
		return nil
	}

	// Try to extract headers from retryableError
	type headerProvider interface {
		Headers() http.Header
	}

	if hp, ok := err.(headerProvider); ok {
		return hp.Headers()
	}

	// Could extend this to check wrapped errors, but for now keep it simple
	return nil
}
