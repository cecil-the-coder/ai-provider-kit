package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExponentialBackoffStrategy(t *testing.T) {
	policy := DefaultRetryPolicy()
	strategy := NewExponentialBackoffStrategy(policy)

	require.NotNil(t, strategy)
	assert.Equal(t, policy, strategy.policy)
	assert.Equal(t, EqualJitter, strategy.jitterType)
	assert.NotNil(t, strategy.rng)
}

func TestExponentialBackoffStrategy_NextDelay(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.0, // No jitter for predictable testing
	}

	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(NoJitter)

	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{"first retry", 0, 1 * time.Second},
		{"second retry", 1, 2 * time.Second},
		{"third retry", 2, 4 * time.Second},
		{"fourth retry", 3, 8 * time.Second},
		{"fifth retry", 4, 16 * time.Second},
		{"sixth retry capped", 5, 30 * time.Second},
		{"seventh retry capped", 6, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.NextDelay(tt.attempt, nil)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestExponentialBackoffStrategy_NoJitter(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     0, // No max
		Multiplier:   2.0,
		Jitter:       0.1, // Jitter specified but NoJitter type
	}

	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(NoJitter)

	// Should return exact values despite jitter in policy
	assert.Equal(t, 1*time.Second, strategy.NextDelay(0, nil))
	assert.Equal(t, 2*time.Second, strategy.NextDelay(1, nil))
	assert.Equal(t, 4*time.Second, strategy.NextDelay(2, nil))
}

func TestExponentialBackoffStrategy_FullJitter(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       1.0,
	}

	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(FullJitter)

	// With full jitter, delay should be between 0 and calculated delay
	for i := 0; i < 10; i++ {
		delay := strategy.NextDelay(2, nil) // 4 seconds base
		assert.True(t, delay >= 0, "Delay should be non-negative")
		assert.True(t, delay <= 4*time.Second, "Delay should not exceed base delay")
	}
}

func TestExponentialBackoffStrategy_EqualJitter(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 2 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       1.0,
	}

	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(EqualJitter)

	// With equal jitter, delay should be between half and full calculated delay
	for i := 0; i < 10; i++ {
		delay := strategy.NextDelay(2, nil) // 8 seconds base
		assert.True(t, delay >= 4*time.Second, "Delay should be at least half of base")
		assert.True(t, delay <= 8*time.Second, "Delay should not exceed base delay")
	}
}

func TestExponentialBackoffStrategy_DecorrelatedJitter(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       1.0,
	}

	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(DecorrelatedJitter)

	// First call should use initial delay
	delay1 := strategy.NextDelay(0, nil)
	assert.True(t, delay1 >= policy.InitialDelay)

	// Subsequent calls should be influenced by previous delay
	for i := 1; i < 5; i++ {
		delay := strategy.NextDelay(i, nil)
		assert.True(t, delay >= policy.InitialDelay)
		assert.True(t, delay <= policy.MaxDelay)
	}
}

func TestExponentialBackoffStrategy_Reset(t *testing.T) {
	policy := DefaultRetryPolicy()
	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(DecorrelatedJitter)

	// Generate some delays to set internal state
	strategy.NextDelay(0, nil)
	strategy.NextDelay(1, nil)
	assert.NotEqual(t, time.Duration(0), strategy.previousDelay)

	// Reset should clear state
	strategy.Reset()
	assert.Equal(t, time.Duration(0), strategy.previousDelay)
}

func TestConstantBackoffStrategy(t *testing.T) {
	delay := 5 * time.Second
	strategy := NewConstantBackoffStrategy(delay)

	// Should return same delay regardless of attempt
	for i := 0; i < 10; i++ {
		result := strategy.NextDelay(i, nil)
		assert.Equal(t, delay, result, "Constant backoff should return same delay for attempt %d", i)
	}
}

func TestConstantBackoffStrategy_Reset(t *testing.T) {
	strategy := NewConstantBackoffStrategy(5 * time.Second)

	// Reset should be a no-op
	strategy.Reset()

	// Should still return same delay
	assert.Equal(t, 5*time.Second, strategy.NextDelay(0, nil))
}

func TestLinearBackoffStrategy(t *testing.T) {
	initialDelay := 1 * time.Second
	increment := 2 * time.Second
	maxDelay := 20 * time.Second

	strategy := NewLinearBackoffStrategy(initialDelay, increment, maxDelay)

	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{"first retry", 0, 1 * time.Second},
		{"second retry", 1, 3 * time.Second},
		{"third retry", 2, 5 * time.Second},
		{"fourth retry", 3, 7 * time.Second},
		{"tenth retry capped", 9, 19 * time.Second},     // 1 + (9*2) = 19
		{"eleventh retry capped", 10, 20 * time.Second}, // 1 + (10*2) = 21, capped at 20
		{"twentieth retry capped", 19, 20 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.NextDelay(tt.attempt, nil)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestLinearBackoffStrategy_NoMaxDelay(t *testing.T) {
	strategy := NewLinearBackoffStrategy(1*time.Second, 1*time.Second, 0)

	// Without max delay, should continue increasing
	assert.Equal(t, 1*time.Second, strategy.NextDelay(0, nil))
	assert.Equal(t, 2*time.Second, strategy.NextDelay(1, nil))
	assert.Equal(t, 11*time.Second, strategy.NextDelay(10, nil))
	assert.Equal(t, 101*time.Second, strategy.NextDelay(100, nil))
}

func TestLinearBackoffStrategy_Reset(t *testing.T) {
	strategy := NewLinearBackoffStrategy(1*time.Second, 1*time.Second, 10*time.Second)

	// Reset should be a no-op
	strategy.Reset()

	// Should still calculate correctly
	assert.Equal(t, 1*time.Second, strategy.NextDelay(0, nil))
}

func TestBackoffStrategy_Interface(t *testing.T) {
	// Verify all strategies implement BackoffStrategy interface
	var _ BackoffStrategy = (*ExponentialBackoffStrategy)(nil)
	var _ BackoffStrategy = (*ConstantBackoffStrategy)(nil)
	var _ BackoffStrategy = (*LinearBackoffStrategy)(nil)
}

func TestExponentialBackoffStrategy_MaxDelay(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.0,
	}

	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(NoJitter)

	// Should be capped at 5 seconds
	assert.Equal(t, 1*time.Second, strategy.NextDelay(0, nil))
	assert.Equal(t, 2*time.Second, strategy.NextDelay(1, nil))
	assert.Equal(t, 4*time.Second, strategy.NextDelay(2, nil))
	assert.Equal(t, 5*time.Second, strategy.NextDelay(3, nil)) // Capped
	assert.Equal(t, 5*time.Second, strategy.NextDelay(4, nil)) // Still capped
}

func TestExponentialBackoffStrategy_WithJitterType(t *testing.T) {
	policy := DefaultRetryPolicy()
	strategy := NewExponentialBackoffStrategy(policy)

	// Should return same instance for chaining
	result := strategy.WithJitterType(FullJitter)
	assert.Equal(t, strategy, result)
	assert.Equal(t, FullJitter, strategy.jitterType)
}

func TestExponentialBackoffStrategy_JitterVariability(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       1.0,
	}

	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(FullJitter)

	// Generate multiple delays and check they're different (with very high probability)
	delays := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		delay := strategy.NextDelay(3, nil)
		delays[delay] = true
	}

	// Should have at least 50 different values (randomness means we won't get all 100)
	assert.True(t, len(delays) > 50, "Expected diverse jittered delays, got %d unique values", len(delays))
}

func TestBackoffStrategies_DefaultJitter(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.2,
	}

	strategy := NewExponentialBackoffStrategy(policy)

	// Default jitter behavior (EqualJitter: half fixed, half random)
	// For attempt 2, base is 4 seconds. With EqualJitter: 2s + random(0, 2s) = 2s to 4s
	for i := 0; i < 10; i++ {
		delay := strategy.NextDelay(2, nil)
		assert.True(t, delay >= 2*time.Second && delay <= 4*time.Second,
			"Expected delay between 2s and 4s, got %v", delay)
	}
}

func TestExponentialBackoffStrategy_LargeMultiplier(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     60 * time.Second,
		Multiplier:   10.0, // Large multiplier
		Jitter:       0.0,
	}

	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(NoJitter)

	assert.Equal(t, 100*time.Millisecond, strategy.NextDelay(0, nil))
	assert.Equal(t, 1*time.Second, strategy.NextDelay(1, nil))
	assert.Equal(t, 10*time.Second, strategy.NextDelay(2, nil))
	assert.Equal(t, 60*time.Second, strategy.NextDelay(3, nil)) // Capped
}

func TestLinearBackoffStrategy_ZeroIncrement(t *testing.T) {
	strategy := NewLinearBackoffStrategy(5*time.Second, 0, 10*time.Second)

	// With zero increment, should return initial delay for all attempts
	for i := 0; i < 5; i++ {
		assert.Equal(t, 5*time.Second, strategy.NextDelay(i, nil))
	}
}

func TestConstantBackoffStrategy_ZeroDelay(t *testing.T) {
	strategy := NewConstantBackoffStrategy(0)

	// Should return zero delay
	for i := 0; i < 5; i++ {
		assert.Equal(t, time.Duration(0), strategy.NextDelay(i, nil))
	}
}

// TestExtractHeaders verifies the extractHeaders helper function
func TestExtractHeaders(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		headers := extractHeaders(nil)
		assert.Nil(t, headers)
	})

	t.Run("regular error", func(t *testing.T) {
		err := errors.New("regular error")
		headers := extractHeaders(err)
		assert.Nil(t, headers)
	})

	// Note: Testing with actual header-providing errors would require
	// extending the retryableError type to support headers,
	// which is not currently implemented but shows the extensibility
}

func TestDecorrelatedJitter_MaxDelayCap(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       1.0,
	}

	strategy := NewExponentialBackoffStrategy(policy).WithJitterType(DecorrelatedJitter)

	// All delays should be capped at MaxDelay
	for i := 0; i < 20; i++ {
		delay := strategy.NextDelay(i, nil)
		assert.True(t, delay <= policy.MaxDelay, "Delay %v exceeds max delay %v", delay, policy.MaxDelay)
		assert.True(t, delay >= policy.InitialDelay, "Delay %v is less than initial delay %v", delay, policy.InitialDelay)
	}
}

func TestJitterType_Values(t *testing.T) {
	// Verify jitter type constants
	assert.Equal(t, JitterType(0), NoJitter)
	assert.Equal(t, JitterType(1), FullJitter)
	assert.Equal(t, JitterType(2), EqualJitter)
	assert.Equal(t, JitterType(3), DecorrelatedJitter)
}
