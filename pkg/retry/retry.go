// Package retry provides retry policies with configurable backoff strategies for resilient operations.
//
// This package offers a comprehensive retry mechanism with multiple backoff strategies,
// error classification, and flexible policy configuration. It is designed for handling
// transient failures in network operations, API calls, and other unreliable operations.
//
// # Basic Usage
//
// Use the default executor for simple retry scenarios:
//
//	executor := retry.NewDefaultRetryExecutor()
//	err := executor.Execute(ctx, func() error {
//	    return apiCall()
//	})
//
// # Custom Policies
//
// Create custom retry policies for specific requirements:
//
//	policy := retry.DefaultRetryPolicy().
//	    WithMaxRetries(5).
//	    WithInitialDelay(100 * time.Millisecond)
//	strategy := retry.NewExponentialBackoffStrategy(policy)
//	executor := retry.NewRetryExecutor(policy, strategy)
//
// # Backoff Strategies
//
// Multiple backoff strategies are available:
//
//   - ExponentialBackoffStrategy: Exponential delay increase with jitter
//   - ConstantBackoffStrategy: Fixed delay between retries
//   - LinearBackoffStrategy: Linear delay increase
//
// # Error Classification
//
// Errors can be marked as retryable or non-retryable:
//
//	err := retry.MarkRetryable(errors.New("temporary failure"), 503)
//	if retry.IsRetryableError(err) {
//	    // Handle retryable error
//	}
package retry

import (
	"context"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/retry"
)

// Policy defines the configuration for retry behavior.
// It specifies how many times to retry, delays between attempts,
// and which errors/status codes should trigger retries.
type Policy = retry.RetryPolicy

// BackoffStrategy defines the interface for calculating retry delays.
// Implementations provide different algorithms for determining how long
// to wait between retry attempts.
type BackoffStrategy = retry.BackoffStrategy

// RetryStrategy defines the interface for retry execution.
// It provides the NextDelay method for calculating delays before retry attempts.
type RetryStrategy = retry.RetryStrategy

// JitterType defines different types of jitter strategies for randomizing delays.
// Jitter helps prevent thundering herd problems when multiple clients retry simultaneously.
type JitterType = retry.JitterType

// Jitter type constants.
const (
	// NoJitter applies no randomization to the delay.
	NoJitter = retry.NoJitter

	// FullJitter randomizes the delay between 0 and the calculated delay.
	FullJitter = retry.FullJitter

	// EqualJitter splits the delay evenly between fixed and random components.
	EqualJitter = retry.EqualJitter

	// DecorrelatedJitter uses AWS's decorrelated jitter algorithm.
	DecorrelatedJitter = retry.DecorrelatedJitter
)

// Common HTTP status codes that may trigger retries.
const (
	StatusTooManyRequests     = retry.StatusTooManyRequests     // 429
	StatusInternalServerError = retry.StatusInternalServerError // 500
	StatusBadGateway          = retry.StatusBadGateway          // 502
	StatusServiceUnavailable  = retry.StatusServiceUnavailable  // 503
	StatusGatewayTimeout      = retry.StatusGatewayTimeout      // 504
	StatusInsufficientStorage = retry.StatusInsufficientStorage // 507
	StatusNetworkAuthRequired = retry.StatusNetworkAuthRequired // 511
)

// RetryableError is an interface for errors that can indicate retry possibility.
// Errors implementing this interface can explicitly declare whether they should be retried.
type RetryableError = retry.RetryableError

// ExponentialBackoffStrategy implements exponential backoff with configurable jitter.
// The delay increases exponentially with each retry attempt, optionally randomized with jitter.
type ExponentialBackoffStrategy = retry.ExponentialBackoffStrategy

// ConstantBackoffStrategy implements a constant delay between retries.
// The same delay is used for all retry attempts.
type ConstantBackoffStrategy = retry.ConstantBackoffStrategy

// LinearBackoffStrategy implements a linear increase in delay.
// The delay increases by a fixed increment with each retry attempt.
type LinearBackoffStrategy = retry.LinearBackoffStrategy

// RetryExecutor handles the execution of operations with retry logic.
// It combines a retry policy with a backoff strategy to provide
// automatic retry functionality for operations.
type RetryExecutor struct {
	executor *retry.RetryExecutor
}

// NewRetryExecutor creates a new retry executor with the given policy and strategy.
//
// Example:
//
//	policy := retry.DefaultRetryPolicy()
//	strategy := retry.NewExponentialBackoffStrategy(policy)
//	executor := retry.NewRetryExecutor(policy, strategy)
func NewRetryExecutor(policy *Policy, strategy BackoffStrategy) *RetryExecutor {
	return &RetryExecutor{
		executor: retry.NewRetryExecutor(policy, strategy),
	}
}

// NewDefaultRetryExecutor creates a retry executor with default settings.
// Uses DefaultRetryPolicy() and ExponentialBackoffStrategy with EqualJitter.
func NewDefaultRetryExecutor() *RetryExecutor {
	return &RetryExecutor{
		executor: retry.NewDefaultRetryExecutor(),
	}
}

// Execute executes a function with retry logic.
// The function is called repeatedly until it succeeds, a non-retryable error occurs,
// or the maximum number of retries is reached.
//
// The context can be used to cancel the retry operation.
func (r *RetryExecutor) Execute(ctx context.Context, operation func() error) error {
	return r.executor.Execute(ctx, operation)
}

// ExecuteWithResult executes a function that returns a result and error with retry logic.
// Returns the result from the first successful attempt or the last result if all retries fail.
func (r *RetryExecutor) ExecuteWithResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	return r.executor.ExecuteWithResult(ctx, operation)
}

// ExecuteWithCallback executes a function with retry logic and callback notifications.
// The onRetry callback is invoked before each retry attempt, allowing for logging,
// metrics collection, or custom retry handling.
func (r *RetryExecutor) ExecuteWithCallback(
	ctx context.Context,
	operation func() error,
	onRetry OnRetryFunc,
) error {
	return r.executor.ExecuteWithCallback(ctx, operation, onRetry)
}

// WithPolicy creates a new executor with a different policy.
// Returns a new RetryExecutor instance without modifying the original.
func (r *RetryExecutor) WithPolicy(policy *Policy) *RetryExecutor {
	return &RetryExecutor{
		executor: r.executor.WithPolicy(policy),
	}
}

// WithStrategy creates a new executor with a different strategy.
// Returns a new RetryExecutor instance without modifying the original.
func (r *RetryExecutor) WithStrategy(strategy BackoffStrategy) *RetryExecutor {
	return &RetryExecutor{
		executor: r.executor.WithStrategy(strategy),
	}
}

// GetPolicy returns the current retry policy.
func (r *RetryExecutor) GetPolicy() *Policy {
	return r.executor.GetPolicy()
}

// GetStrategy returns the current backoff strategy.
func (r *RetryExecutor) GetStrategy() BackoffStrategy {
	return r.executor.GetStrategy()
}

// ExecuteFunc is a generic function type that works with any return type.
// This is useful for strongly-typed operations.
type ExecuteFunc[T any] func() (T, error)

// ExecuteTyped executes a typed function with retry logic.
// It wraps the generic ExecuteWithResult to provide type-safe returns.
//
// Example:
//
//	result, err := retry.ExecuteTyped(ctx, executor, func() (string, error) {
//	    return api.Call()
//	})
func ExecuteTyped[T any](ctx context.Context, executor *RetryExecutor, operation ExecuteFunc[T]) (T, error) {
	var zero T
	result, err := executor.ExecuteWithResult(ctx, func() (interface{}, error) {
		return operation()
	})
	if err != nil {
		if result != nil {
			if typed, ok := result.(T); ok {
				return typed, err
			}
		}
		return zero, err
	}
	if result == nil {
		return zero, nil
	}
	return result.(T), nil
}

// OnRetryFunc is a callback function type that is called before each retry attempt.
// It receives the current attempt number, the error that triggered the retry,
// and the delay before the next attempt.
type OnRetryFunc = retry.OnRetryFunc

// NewExponentialBackoffStrategy creates a new exponential backoff strategy.
// Uses EqualJitter by default, which can be changed with WithJitterType.
//
// Example:
//
//	policy := retry.DefaultRetryPolicy()
//	strategy := retry.NewExponentialBackoffStrategy(policy).
//	    WithJitterType(retry.FullJitter)
func NewExponentialBackoffStrategy(policy *Policy) *ExponentialBackoffStrategy {
	return retry.NewExponentialBackoffStrategy(policy)
}

// NewConstantBackoffStrategy creates a new constant backoff strategy.
// The same delay is used for all retry attempts.
//
// Example:
//
//	strategy := retry.NewConstantBackoffStrategy(2 * time.Second)
func NewConstantBackoffStrategy(delay time.Duration) *ConstantBackoffStrategy {
	return retry.NewConstantBackoffStrategy(delay)
}

// NewLinearBackoffStrategy creates a new linear backoff strategy.
// The delay increases by a fixed increment with each retry attempt, capped at maxDelay.
//
// Example:
//
//	strategy := retry.NewLinearBackoffStrategy(
//	    1*time.Second,  // initialDelay
//	    500*time.Millisecond, // increment
//	    30*time.Second, // maxDelay
//	)
func NewLinearBackoffStrategy(initialDelay, increment, maxDelay time.Duration) *LinearBackoffStrategy {
	return retry.NewLinearBackoffStrategy(initialDelay, increment, maxDelay)
}

// DefaultRetryPolicy returns a retry policy with sensible defaults:
//   - MaxRetries: 3
//   - InitialDelay: 1 second
//   - MaxDelay: 30 seconds
//   - Multiplier: 2.0
//   - Jitter: 0.1 (10%)
func DefaultRetryPolicy() *Policy {
	return retry.DefaultRetryPolicy()
}

// NoRetryPolicy returns a policy that never retries (MaxRetries: 0).
// Useful for disabling retry behavior while maintaining a consistent API.
func NoRetryPolicy() *Policy {
	return retry.NoRetryPolicy()
}

// AggressiveRetryPolicy returns a policy with more retry attempts and shorter delays.
// Useful for testing or services with very high availability:
//   - MaxRetries: 5
//   - InitialDelay: 500ms
//   - MaxDelay: 10 seconds
//   - Multiplier: 1.5
//   - Jitter: 0.2 (20%)
func AggressiveRetryPolicy() *Policy {
	return retry.AggressiveRetryPolicy()
}

// ConservativeRetryPolicy returns a policy with fewer retries and longer delays.
// Useful for rate-limited APIs or to avoid overwhelming services:
//   - MaxRetries: 2
//   - InitialDelay: 2 seconds
//   - MaxDelay: 60 seconds
//   - Multiplier: 3.0
//   - Jitter: 0.05 (5%)
func ConservativeRetryPolicy() *Policy {
	return retry.ConservativeRetryPolicy()
}

// NewRetryableError creates a new retryable error with the specified status code.
// The retryable parameter determines if the error should trigger retries.
//
// Example:
//
//	err := retry.NewRetryableError(errors.New("API error"), true, 503)
func NewRetryableError(err error, retryable bool, statusCode int) error {
	return retry.NewRetryableError(err, retryable, statusCode)
}

// MarkRetryable wraps an error to mark it as retryable with the given status code.
// This is a convenience function for NewRetryableError with retryable=true.
//
// Example:
//
//	return retry.MarkRetryable(apiErr, 503)
func MarkRetryable(err error, statusCode int) error {
	return retry.MarkRetryable(err, statusCode)
}

// MarkNonRetryable wraps an error to mark it as non-retryable with the given status code.
// This is a convenience function for NewRetryableError with retryable=false.
//
// Example:
//
//	return retry.MarkNonRetryable(authErr, 401)
func MarkNonRetryable(err error, statusCode int) error {
	return retry.MarkNonRetryable(err, statusCode)
}

// IsRetryableError checks if an error is retryable.
// It examines the error chain for RetryableError implementations.
//
// Example:
//
//	if retry.IsRetryableError(err) {
//	    // Handle retryable error
//	}
func IsRetryableError(err error) bool {
	return retry.IsRetryableError(err)
}

// IsRetryableStatusCode checks if an HTTP status code is retryable.
// Returns true for common transient status codes (429, 500, 502, 503, 504, 507, 511).
//
// Example:
//
//	if retry.IsRetryableStatusCode(resp.StatusCode) {
//	    // Trigger retry
//	}
func IsRetryableStatusCode(statusCode int) bool {
	return retry.IsRetryableStatusCode(statusCode)
}

// GetStatusCode extracts the HTTP status code from an error if available.
// Returns 0 if no status code is associated with the error.
func GetStatusCode(err error) int {
	return retry.GetStatusCode(err)
}
