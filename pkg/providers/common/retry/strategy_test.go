package retry

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRetryExecutor(t *testing.T) {
	policy := DefaultRetryPolicy()
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)

	require.NotNil(t, executor)
	assert.Equal(t, policy, executor.policy)
	assert.Equal(t, strategy, executor.strategy)
}

func TestNewDefaultRetryExecutor(t *testing.T) {
	executor := NewDefaultRetryExecutor()

	require.NotNil(t, executor)
	require.NotNil(t, executor.policy)
	require.NotNil(t, executor.strategy)

	assert.Equal(t, 3, executor.policy.MaxRetries)
}

func TestRetryExecutor_Execute_Success(t *testing.T) {
	executor := NewDefaultRetryExecutor()
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		return nil
	}

	err := executor.Execute(ctx, operation)

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "Should succeed on first attempt")
}

func TestRetryExecutor_Execute_SuccessAfterRetries(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0.0,
	}
	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(NoJitter)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 3 {
			return MarkRetryable(errors.New("temporary error"), 503)
		}
		return nil
	}

	err := executor.Execute(ctx, operation)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount, "Should succeed on third attempt")
}

func TestRetryExecutor_Execute_MaxRetriesExceeded(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0.0,
	}
	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(NoJitter)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		return MarkRetryable(errors.New("persistent error"), 503)
	}

	err := executor.Execute(ctx, operation)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retries")
	assert.Equal(t, 3, callCount, "Should attempt initial + 2 retries")
}

func TestRetryExecutor_Execute_NonRetryableError(t *testing.T) {
	executor := NewDefaultRetryExecutor()
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		return MarkNonRetryable(errors.New("permanent error"), 400)
	}

	err := executor.Execute(ctx, operation)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-retryable error")
	assert.Equal(t, 1, callCount, "Should not retry non-retryable errors")
}

func TestRetryExecutor_Execute_ContextCancellation(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	operation := func() error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel after second attempt
		}
		return MarkRetryable(errors.New("error"), 503)
	}

	err := executor.Execute(ctx, operation)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
	assert.True(t, callCount >= 2 && callCount <= 3, "Should stop after context cancellation")
}

func TestRetryExecutor_Execute_ContextTimeout(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   10,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	callCount := 0
	operation := func() error {
		callCount++
		return MarkRetryable(errors.New("error"), 503)
	}

	err := executor.Execute(ctx, operation)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestRetryExecutor_ExecuteWithResult_Success(t *testing.T) {
	executor := NewDefaultRetryExecutor()
	ctx := context.Background()

	operation := func() (interface{}, error) {
		return "success", nil
	}

	result, err := executor.ExecuteWithResult(ctx, operation)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestRetryExecutor_ExecuteWithResult_SuccessAfterRetries(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	callCount := 0
	operation := func() (interface{}, error) {
		callCount++
		if callCount < 3 {
			return nil, MarkRetryable(errors.New("temporary error"), 503)
		}
		return "success", nil
	}

	result, err := executor.ExecuteWithResult(ctx, operation)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 3, callCount)
}

func TestRetryExecutor_ExecuteWithResult_PartialResult(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	operation := func() (interface{}, error) {
		return "partial", MarkRetryable(errors.New("persistent error"), 503)
	}

	result, err := executor.ExecuteWithResult(ctx, operation)

	assert.Error(t, err)
	assert.Equal(t, "partial", result, "Should return last partial result")
}

func TestExecuteTyped_Success(t *testing.T) {
	executor := NewDefaultRetryExecutor()
	ctx := context.Background()

	operation := func() (int, error) {
		return 42, nil
	}

	result, err := ExecuteTyped(ctx, executor, operation)

	assert.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestExecuteTyped_SuccessAfterRetries(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	callCount := 0
	operation := func() (string, error) {
		callCount++
		if callCount < 3 {
			return "", MarkRetryable(errors.New("temporary error"), 503)
		}
		return "success", nil
	}

	result, err := ExecuteTyped(ctx, executor, operation)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 3, callCount)
}

func TestExecuteTyped_MaxRetriesExceeded(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	operation := func() (int, error) {
		return 0, MarkRetryable(errors.New("persistent error"), 503)
	}

	result, err := ExecuteTyped(ctx, executor, operation)

	assert.Error(t, err)
	assert.Equal(t, 0, result)
	assert.Contains(t, err.Error(), "max retries")
}

func TestExecuteTyped_ContextCancellation(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)

	ctx, cancel := context.WithCancel(context.Background())

	var callCount int32
	operation := func() (string, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 2 {
			cancel()
		}
		return "", MarkRetryable(errors.New("error"), 503)
	}

	result, err := ExecuteTyped(ctx, executor, operation)

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestRetryExecutor_ExecuteWithCallback(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	callbackCalls := 0
	var callbackAttempts []int
	var callbackDelays []time.Duration

	onRetry := func(attempt int, err error, delay time.Duration) {
		callbackCalls++
		callbackAttempts = append(callbackAttempts, attempt)
		callbackDelays = append(callbackDelays, delay)
	}

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 3 {
			return MarkRetryable(errors.New("temporary error"), 503)
		}
		return nil
	}

	err := executor.ExecuteWithCallback(ctx, operation, onRetry)

	assert.NoError(t, err)
	assert.Equal(t, 2, callbackCalls, "Callback should be called for each retry")
	assert.Equal(t, []int{0, 1}, callbackAttempts)
	assert.True(t, len(callbackDelays) == 2)
}

func TestRetryExecutor_ExecuteWithCallback_NilCallback(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 2 {
			return MarkRetryable(errors.New("temporary error"), 503)
		}
		return nil
	}

	// Should not panic with nil callback
	err := executor.ExecuteWithCallback(ctx, operation, nil)

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestRetryExecutor_WithPolicy(t *testing.T) {
	policy1 := DefaultRetryPolicy()
	policy2 := AggressiveRetryPolicy()
	strategy := NewExponentialBackoffStrategy(policy1)
	executor1 := NewRetryExecutor(policy1, strategy)

	executor2 := executor1.WithPolicy(policy2)

	assert.Equal(t, policy1, executor1.policy, "Original executor should be unchanged")
	assert.Equal(t, policy2, executor2.policy, "New executor should have new policy")
	assert.Equal(t, strategy, executor2.strategy, "Strategy should be same")
}

func TestRetryExecutor_WithStrategy(t *testing.T) {
	policy := DefaultRetryPolicy()
	strategy1 := NewExponentialBackoffStrategy(policy)
	strategy2 := NewConstantBackoffStrategy(1 * time.Second)
	executor1 := NewRetryExecutor(policy, strategy1)

	executor2 := executor1.WithStrategy(strategy2)

	assert.Equal(t, strategy1, executor1.strategy, "Original executor should be unchanged")
	assert.Equal(t, strategy2, executor2.strategy, "New executor should have new strategy")
	assert.Equal(t, policy, executor2.policy, "Policy should be same")
}

func TestRetryExecutor_GetPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)

	assert.Equal(t, policy, executor.GetPolicy())
}

func TestRetryExecutor_GetStrategy(t *testing.T) {
	policy := DefaultRetryPolicy()
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)

	assert.Equal(t, strategy, executor.GetStrategy())
}

func TestRetryExecutor_Execute_RegularError(t *testing.T) {
	executor := NewDefaultRetryExecutor()
	ctx := context.Background()

	operation := func() error {
		return errors.New("regular error")
	}

	err := executor.Execute(ctx, operation)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-retryable error")
}

func TestRetryExecutor_Execute_NilError(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		if callCount == 1 {
			return MarkRetryable(errors.New("first error"), 503)
		}
		return nil // Success on second attempt
	}

	err := executor.Execute(ctx, operation)

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestRetryExecutor_ContextCancelledBeforeFirstAttempt(t *testing.T) {
	executor := NewDefaultRetryExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	operation := func() error {
		return errors.New("should not be called")
	}

	err := executor.Execute(ctx, operation)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestRetryExecutor_IntegrationWithDifferentStrategies(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	ctx := context.Background()

	tests := []struct {
		name     string
		strategy BackoffStrategy
	}{
		{
			name:     "exponential backoff",
			strategy: NewExponentialBackoffStrategy(policy),
		},
		{
			name:     "constant backoff",
			strategy: NewConstantBackoffStrategy(20 * time.Millisecond),
		},
		{
			name:     "linear backoff",
			strategy: NewLinearBackoffStrategy(10*time.Millisecond, 10*time.Millisecond, 100*time.Millisecond),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewRetryExecutor(policy, tt.strategy)

			callCount := 0
			operation := func() error {
				callCount++
				if callCount < 3 {
					return MarkRetryable(errors.New("temporary error"), 503)
				}
				return nil
			}

			err := executor.Execute(ctx, operation)

			assert.NoError(t, err)
			assert.Equal(t, 3, callCount)
		})
	}
}

func TestRetryExecutor_StrategyReset(t *testing.T) {
	policy := DefaultRetryPolicy()
	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(DecorrelatedJitter)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	// First execution
	callCount1 := 0
	operation1 := func() error {
		callCount1++
		if callCount1 < 2 {
			return MarkRetryable(errors.New("error"), 503)
		}
		return nil
	}

	err1 := executor.Execute(ctx, operation1)
	assert.NoError(t, err1)

	// Strategy should be reset for second execution
	callCount2 := 0
	operation2 := func() error {
		callCount2++
		if callCount2 < 2 {
			return MarkRetryable(errors.New("error"), 503)
		}
		return nil
	}

	err2 := executor.Execute(ctx, operation2)
	assert.NoError(t, err2)
}

// Benchmark tests
func BenchmarkRetryExecutor_NoRetries(b *testing.B) {
	executor := NewDefaultRetryExecutor()
	ctx := context.Background()

	operation := func() error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = executor.Execute(ctx, operation)
	}
}

func BenchmarkRetryExecutor_WithRetries(b *testing.B) {
	policy := &RetryPolicy{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}
	strategy := NewExponentialBackoffStrategy(policy)
	executor := NewRetryExecutor(policy, strategy)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callCount := 0
		operation := func() error {
			callCount++
			if callCount < 2 {
				return MarkRetryable(fmt.Errorf("retry"), 503)
			}
			return nil
		}
		_ = executor.Execute(ctx, operation)
	}
}

func BenchmarkExecuteTyped(b *testing.B) {
	executor := NewDefaultRetryExecutor()
	ctx := context.Background()

	operation := func() (int, error) {
		return 42, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExecuteTyped(ctx, executor, operation)
	}
}
