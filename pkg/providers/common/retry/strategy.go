package retry

import (
	"context"
	"fmt"
	"log"
	"time"
)

// RetryStrategy defines the interface for retry execution
type RetryStrategy interface {
	// NextDelay calculates the delay before the next retry attempt
	NextDelay(attempt int, err error) time.Duration
}

// RetryExecutor handles the execution of operations with retry logic
type RetryExecutor struct {
	policy   *RetryPolicy
	strategy BackoffStrategy
}

// NewRetryExecutor creates a new retry executor with the given policy and strategy
func NewRetryExecutor(policy *RetryPolicy, strategy BackoffStrategy) *RetryExecutor {
	return &RetryExecutor{
		policy:   policy,
		strategy: strategy,
	}
}

// NewDefaultRetryExecutor creates a retry executor with default settings
func NewDefaultRetryExecutor() *RetryExecutor {
	policy := DefaultRetryPolicy()
	strategy := NewExponentialBackoffStrategy(policy)
	return NewRetryExecutor(policy, strategy)
}

// Execute executes a function with retry logic
// The function is called repeatedly until it succeeds, a non-retryable error occurs,
// or the maximum number of retries is reached
func (r *RetryExecutor) Execute(ctx context.Context, operation func() error) error {
	var lastErr error
	attempt := 0

	// Reset strategy state before starting
	r.strategy.Reset()

	for {
		// Check context cancellation before attempting
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("context cancelled after %d attempts: %w", attempt, lastErr)
			}
			return ctx.Err()
		default:
		}

		// Execute the operation
		err := operation()
		if err == nil {
			// Success!
			if attempt > 0 {
				log.Printf("[RetryExecutor] Operation succeeded after %d retries", attempt)
			}
			return nil
		}

		// Store the error
		lastErr = err

		// Check if we should retry
		if !r.policy.ShouldRetry(err, attempt) {
			if attempt >= r.policy.MaxRetries {
				return fmt.Errorf("max retries (%d) exceeded: %w", r.policy.MaxRetries, err)
			}
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Calculate delay before next retry
		delay := r.strategy.NextDelay(attempt, err)
		log.Printf("[RetryExecutor] Attempt %d failed: %v. Retrying in %v", attempt+1, err, delay)

		// Wait for the delay, respecting context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry wait after %d attempts: %w", attempt, lastErr)
		case <-time.After(delay):
			// Continue to next attempt
		}

		attempt++
	}
}

// ExecuteWithResult executes a function that returns a result and error with retry logic
func (r *RetryExecutor) ExecuteWithResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	var lastErr error
	var result interface{}
	attempt := 0

	// Reset strategy state before starting
	r.strategy.Reset()

	for {
		// Check context cancellation before attempting
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("context cancelled after %d attempts: %w", attempt, lastErr)
			}
			return nil, ctx.Err()
		default:
		}

		// Execute the operation
		res, err := operation()
		if err == nil {
			// Success!
			if attempt > 0 {
				log.Printf("[RetryExecutor] Operation succeeded after %d retries", attempt)
			}
			return res, nil
		}

		// Store the error and result (might be partial)
		lastErr = err
		result = res

		// Check if we should retry
		if !r.policy.ShouldRetry(err, attempt) {
			if attempt >= r.policy.MaxRetries {
				return result, fmt.Errorf("max retries (%d) exceeded: %w", r.policy.MaxRetries, err)
			}
			return result, fmt.Errorf("non-retryable error: %w", err)
		}

		// Calculate delay before next retry
		delay := r.strategy.NextDelay(attempt, err)
		log.Printf("[RetryExecutor] Attempt %d failed: %v. Retrying in %v", attempt+1, err, delay)

		// Wait for the delay, respecting context cancellation
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("context cancelled during retry wait after %d attempts: %w", attempt, lastErr)
		case <-time.After(delay):
			// Continue to next attempt
		}

		attempt++
	}
}

// ExecuteWithData is a generic version that works with any return type
// This is useful for strongly-typed operations
type ExecuteFunc[T any] func() (T, error)

// ExecuteTyped executes a typed function with retry logic
func ExecuteTyped[T any](ctx context.Context, executor *RetryExecutor, operation ExecuteFunc[T]) (T, error) {
	var lastErr error
	var result T
	attempt := 0

	// Reset strategy state before starting
	executor.strategy.Reset()

	for {
		// Check context cancellation before attempting
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return result, fmt.Errorf("context cancelled after %d attempts: %w", attempt, lastErr)
			}
			return result, ctx.Err()
		default:
		}

		// Execute the operation
		res, err := operation()
		if err == nil {
			// Success!
			if attempt > 0 {
				log.Printf("[RetryExecutor] Operation succeeded after %d retries", attempt)
			}
			return res, nil
		}

		// Store the error and result (might be partial)
		lastErr = err
		result = res

		// Check if we should retry
		if !executor.policy.ShouldRetry(err, attempt) {
			if attempt >= executor.policy.MaxRetries {
				return result, fmt.Errorf("max retries (%d) exceeded: %w", executor.policy.MaxRetries, err)
			}
			return result, fmt.Errorf("non-retryable error: %w", err)
		}

		// Calculate delay before next retry
		delay := executor.strategy.NextDelay(attempt, err)
		log.Printf("[RetryExecutor] Attempt %d failed: %v. Retrying in %v", attempt+1, err, delay)

		// Wait for the delay, respecting context cancellation
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("context cancelled during retry wait after %d attempts: %w", attempt, lastErr)
		case <-time.After(delay):
			// Continue to next attempt
		}

		attempt++
	}
}

// OnRetry is a callback function that can be called before each retry
type OnRetryFunc func(attempt int, err error, delay time.Duration)

// ExecuteWithCallback executes a function with retry logic and callback notifications
func (r *RetryExecutor) ExecuteWithCallback(
	ctx context.Context,
	operation func() error,
	onRetry OnRetryFunc,
) error {
	var lastErr error
	attempt := 0

	// Reset strategy state before starting
	r.strategy.Reset()

	for {
		// Check context cancellation before attempting
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("context cancelled after %d attempts: %w", attempt, lastErr)
			}
			return ctx.Err()
		default:
		}

		// Execute the operation
		err := operation()
		if err == nil {
			// Success!
			if attempt > 0 {
				log.Printf("[RetryExecutor] Operation succeeded after %d retries", attempt)
			}
			return nil
		}

		// Store the error
		lastErr = err

		// Check if we should retry
		if !r.policy.ShouldRetry(err, attempt) {
			if attempt >= r.policy.MaxRetries {
				return fmt.Errorf("max retries (%d) exceeded: %w", r.policy.MaxRetries, err)
			}
			return fmt.Errorf("non-retryable error: %w", err)
		}

		// Calculate delay before next retry
		delay := r.strategy.NextDelay(attempt, err)

		// Call the callback if provided
		if onRetry != nil {
			onRetry(attempt, err, delay)
		}

		log.Printf("[RetryExecutor] Attempt %d failed: %v. Retrying in %v", attempt+1, err, delay)

		// Wait for the delay, respecting context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry wait after %d attempts: %w", attempt, lastErr)
		case <-time.After(delay):
			// Continue to next attempt
		}

		attempt++
	}
}

// WithPolicy creates a new executor with a different policy
func (r *RetryExecutor) WithPolicy(policy *RetryPolicy) *RetryExecutor {
	return &RetryExecutor{
		policy:   policy,
		strategy: r.strategy,
	}
}

// WithStrategy creates a new executor with a different strategy
func (r *RetryExecutor) WithStrategy(strategy BackoffStrategy) *RetryExecutor {
	return &RetryExecutor{
		policy:   r.policy,
		strategy: strategy,
	}
}

// GetPolicy returns the current retry policy
func (r *RetryExecutor) GetPolicy() *RetryPolicy {
	return r.policy
}

// GetStrategy returns the current backoff strategy
func (r *RetryExecutor) GetStrategy() BackoffStrategy {
	return r.strategy
}
