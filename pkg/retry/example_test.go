package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/retry"
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
	policy := retry.DefaultRetryPolicy().
		WithMaxRetries(5).
		WithInitialDelay(100 * time.Millisecond)

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
	policy := retry.DefaultRetryPolicy().
		WithJitter(0.0) // No jitter for predictable output

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

// Example_presetPolicies demonstrates using preset retry policies
func Example_presetPolicies() {
	// Default policy (balanced retries)
	defaultPolicy := retry.DefaultRetryPolicy()
	fmt.Printf("Default MaxRetries: %d\n", defaultPolicy.MaxRetries)
	fmt.Printf("Default InitialDelay: %v\n", defaultPolicy.InitialDelay)

	// No retry policy (disable retries)
	noRetryPolicy := retry.NoRetryPolicy()
	fmt.Printf("NoRetry MaxRetries: %d\n", noRetryPolicy.MaxRetries)

	// Aggressive policy (more retries, shorter delays)
	aggressivePolicy := retry.AggressiveRetryPolicy()
	fmt.Printf("Aggressive MaxRetries: %d\n", aggressivePolicy.MaxRetries)
	fmt.Printf("Aggressive InitialDelay: %v\n", aggressivePolicy.InitialDelay)

	// Conservative policy (fewer retries, longer delays)
	conservativePolicy := retry.ConservativeRetryPolicy()
	fmt.Printf("Conservative MaxRetries: %d\n", conservativePolicy.MaxRetries)
	fmt.Printf("Conservative InitialDelay: %v\n", conservativePolicy.InitialDelay)

	// Output:
	// Default MaxRetries: 3
	// Default InitialDelay: 1s
	// NoRetry MaxRetries: 0
	// Aggressive MaxRetries: 5
	// Aggressive InitialDelay: 500ms
	// Conservative MaxRetries: 2
	// Conservative InitialDelay: 2s
}

// Example_executorWithCustomPolicy demonstrates modifying executor behavior
func Example_executorWithCustomPolicy() {
	ctx := context.Background()
	executor := retry.NewDefaultRetryExecutor()

	// Use a conservative policy for this specific operation
	conservativeExecutor := executor.WithPolicy(retry.ConservativeRetryPolicy())

	err := conservativeExecutor.Execute(ctx, func() error {
		// Operation with conservative retry behavior
		return nil
	})

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
	} else {
		fmt.Println("Success with conservative retry")
	}
	// Output: Success with conservative retry
}

// Example_executorWithCustomStrategy demonstrates modifying backoff strategy
func Example_executorWithCustomStrategy() {
	ctx := context.Background()
	executor := retry.NewDefaultRetryExecutor()

	// Use constant backoff instead of exponential
	constantExecutor := executor.WithStrategy(
		retry.NewConstantBackoffStrategy(1 * time.Second),
	)

	err := constantExecutor.Execute(ctx, func() error {
		// Operation with constant backoff
		return nil
	})

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
	} else {
		fmt.Println("Success with constant backoff")
	}
	// Output: Success with constant backoff
}
