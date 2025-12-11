package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/retry"
)

// Example_basic demonstrates basic retry usage
func Example_basic() {
	ctx := context.Background()
	executor := retry.NewDefaultRetryExecutor()

	// Simulate an operation that fails twice then succeeds
	attempt := 0
	err := executor.Execute(ctx, func() error {
		attempt++
		if attempt < 3 {
			return retry.MarkRetryable(errors.New("temporary error"), 503)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
	} else {
		fmt.Printf("Succeeded after %d attempts\n", attempt)
	}
	// Output: Succeeded after 3 attempts
}

// Example_customPolicy demonstrates using a custom retry policy
func Example_customPolicy() {
	ctx := context.Background()

	// Create a custom policy with aggressive retries
	policy := &retry.RetryPolicy{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}

	strategy := retry.NewExponentialBackoffStrategy(policy).
		WithJitterType(retry.FullJitter)

	executor := retry.NewRetryExecutor(policy, strategy)

	// Execute with custom policy
	err := executor.Execute(ctx, func() error {
		// Your operation here
		return nil
	})

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
	} else {
		fmt.Println("Success")
	}
	// Output: Success
}

// Example_typed demonstrates using typed retry operations
func Example_typed() {
	ctx := context.Background()
	executor := retry.NewDefaultRetryExecutor()

	// Execute a typed operation
	attempt := 0
	result, err := retry.ExecuteTyped(ctx, executor, func() (string, error) {
		attempt++
		if attempt < 2 {
			return "", retry.MarkRetryable(errors.New("temporary error"), 503)
		}
		return "success", nil
	})

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n", result)
	}
	// Output: Result: success
}

// Example_callback demonstrates using retry callbacks for monitoring
func Example_callback() {
	ctx := context.Background()
	executor := retry.NewDefaultRetryExecutor()

	attempt := 0
	err := executor.ExecuteWithCallback(
		ctx,
		func() error {
			attempt++
			if attempt < 3 {
				return retry.MarkRetryable(errors.New("temporary error"), 503)
			}
			return nil
		},
		func(attemptNum int, err error, delay time.Duration) {
			// This callback is called before each retry
			// In a real application, you might log this or send metrics
			_ = attemptNum
			_ = err
			_ = delay
		},
	)

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
	} else {
		fmt.Println("Success")
	}
	// Output: Success
}

// Example_backoffStrategies demonstrates different backoff strategies
func Example_backoffStrategies() {
	policy := &retry.RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.0, // No jitter for predictable output
	}

	// Exponential backoff (without jitter for predictable output)
	expStrategy := retry.NewExponentialBackoffStrategy(policy).
		WithJitterType(retry.NoJitter)
	fmt.Printf("Exponential delay (attempt 0): %v\n", expStrategy.NextDelay(0, nil))
	fmt.Printf("Exponential delay (attempt 1): %v\n", expStrategy.NextDelay(1, nil))

	// Constant backoff
	constStrategy := retry.NewConstantBackoffStrategy(2 * time.Second)
	fmt.Printf("Constant delay (attempt 0): %v\n", constStrategy.NextDelay(0, nil))
	fmt.Printf("Constant delay (attempt 1): %v\n", constStrategy.NextDelay(1, nil))

	// Linear backoff
	linearStrategy := retry.NewLinearBackoffStrategy(1*time.Second, 1*time.Second, 10*time.Second)
	fmt.Printf("Linear delay (attempt 0): %v\n", linearStrategy.NextDelay(0, nil))
	fmt.Printf("Linear delay (attempt 1): %v\n", linearStrategy.NextDelay(1, nil))
	// Output:
	// Exponential delay (attempt 0): 1s
	// Exponential delay (attempt 1): 2s
	// Constant delay (attempt 0): 2s
	// Constant delay (attempt 1): 2s
	// Linear delay (attempt 0): 1s
	// Linear delay (attempt 1): 2s
}

// Example_errorClassification demonstrates error classification
func Example_errorClassification() {
	// Check if status codes are retryable
	fmt.Printf("429 retryable: %v\n", retry.IsRetryableStatusCode(429))
	fmt.Printf("500 retryable: %v\n", retry.IsRetryableStatusCode(500))
	fmt.Printf("400 retryable: %v\n", retry.IsRetryableStatusCode(400))

	// Create and check retryable errors
	err := retry.MarkRetryable(errors.New("temporary failure"), 503)
	fmt.Printf("Error retryable: %v\n", retry.IsRetryableError(err))

	// Output:
	// 429 retryable: true
	// 500 retryable: true
	// 400 retryable: false
	// Error retryable: true
}
