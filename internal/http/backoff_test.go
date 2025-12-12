package http

import (
	"testing"
	"time"
)

func TestDefaultBackoffConfig(t *testing.T) {
	config := DefaultBackoffConfig()

	if config.BaseDelay != 1*time.Second {
		t.Errorf("Expected BaseDelay to be 1s, got %v", config.BaseDelay)
	}
	if config.MaxDelay != 60*time.Second {
		t.Errorf("Expected MaxDelay to be 60s, got %v", config.MaxDelay)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier to be 2.0, got %f", config.Multiplier)
	}
	if config.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts to be 3, got %d", config.MaxAttempts)
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name     string
		config   BackoffConfig
		attempt  int
		expected time.Duration
	}{
		{
			name:     "zero attempt",
			config:   DefaultBackoffConfig(),
			attempt:  0,
			expected: 1 * time.Second,
		},
		{
			name:     "first attempt",
			config:   DefaultBackoffConfig(),
			attempt:  1,
			expected: 2 * time.Second, // 1s * 2.0 * (2^0)
		},
		{
			name:     "second attempt",
			config:   DefaultBackoffConfig(),
			attempt:  2,
			expected: 4 * time.Second, // 1s * 2.0 * (2^1)
		},
		{
			name:     "third attempt",
			config:   DefaultBackoffConfig(),
			attempt:  3,
			expected: 8 * time.Second, // 1s * 2.0 * (2^2)
		},
		{
			name:     "fourth attempt",
			config:   DefaultBackoffConfig(),
			attempt:  4,
			expected: 16 * time.Second, // 1s * 2.0 * (2^3)
		},
		{
			name:     "fifth attempt",
			config:   DefaultBackoffConfig(),
			attempt:  5,
			expected: 32 * time.Second, // 1s * 2.0 * (2^4)
		},
		{
			name:     "sixth attempt hits max delay",
			config:   DefaultBackoffConfig(),
			attempt:  6,
			expected: 60 * time.Second, // Capped at MaxDelay
		},
		{
			name: "custom config",
			config: BackoffConfig{
				BaseDelay:   500 * time.Millisecond,
				MaxDelay:    10 * time.Second,
				Multiplier:  1.5,
				MaxAttempts: 5,
			},
			attempt:  1,
			expected: 750 * time.Millisecond, // 500ms * 1.5 * (2^0)
		},
		{
			name: "custom config second attempt",
			config: BackoffConfig{
				BaseDelay:   500 * time.Millisecond,
				MaxDelay:    10 * time.Second,
				Multiplier:  1.5,
				MaxAttempts: 5,
			},
			attempt:  2,
			expected: 1500 * time.Millisecond, // 500ms * 1.5 * (2^1)
		},
		{
			name: "very high attempt number",
			config: BackoffConfig{
				BaseDelay:   1 * time.Second,
				MaxDelay:    60 * time.Second,
				Multiplier:  2.0,
				MaxAttempts: 100,
			},
			attempt:  100,
			expected: 60 * time.Second, // Should be capped at MaxDelay
		},
		{
			name: "overflow protection",
			config: BackoffConfig{
				BaseDelay:   1 * time.Second,
				MaxDelay:    100 * time.Second,
				Multiplier:  2.0,
				MaxAttempts: 50,
			},
			attempt:  50,
			expected: 100 * time.Second, // Should be capped, not overflow
		},
		{
			name: "negative attempt",
			config: BackoffConfig{
				BaseDelay:   1 * time.Second,
				MaxDelay:    60 * time.Second,
				Multiplier:  2.0,
				MaxAttempts: 3,
			},
			attempt:  -1,
			expected: 1 * time.Second, // Should return BaseDelay
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBackoff(tt.config, tt.attempt)
			if result != tt.expected {
				t.Errorf("CalculateBackoff(%v, %d) = %v, want %v",
					tt.config, tt.attempt, result, tt.expected)
			}
		})
	}
}

func TestCalculateBackoffExponentialGrowth(t *testing.T) {
	config := BackoffConfig{
		BaseDelay:   1 * time.Second,
		MaxDelay:    1 * time.Hour, // High max to test exponential growth
		Multiplier:  2.0,
		MaxAttempts: 10,
	}

	var previousDelay time.Duration
	for attempt := 1; attempt <= 10; attempt++ {
		delay := CalculateBackoff(config, attempt)

		// Verify delay is growing
		if attempt > 1 && delay <= previousDelay {
			t.Errorf("Attempt %d: delay %v should be greater than previous %v",
				attempt, delay, previousDelay)
		}

		previousDelay = delay
	}
}

func TestCalculateBackoffMaxDelayCapping(t *testing.T) {
	config := BackoffConfig{
		BaseDelay:   1 * time.Second,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
		MaxAttempts: 10,
	}

	// After a few attempts, all subsequent delays should be capped at MaxDelay
	for attempt := 5; attempt <= 10; attempt++ {
		delay := CalculateBackoff(config, attempt)
		if delay > config.MaxDelay {
			t.Errorf("Attempt %d: delay %v exceeds MaxDelay %v",
				attempt, delay, config.MaxDelay)
		}
		if delay != config.MaxDelay {
			t.Errorf("Attempt %d: expected delay to be capped at %v, got %v",
				attempt, config.MaxDelay, delay)
		}
	}
}

func TestCalculateBackoffDifferentMultipliers(t *testing.T) {
	multipliers := []float64{1.5, 2.0, 3.0}

	for _, mult := range multipliers {
		config := BackoffConfig{
			BaseDelay:   1 * time.Second,
			MaxDelay:    1 * time.Hour,
			Multiplier:  mult,
			MaxAttempts: 5,
		}

		// Verify the multiplier affects the growth rate
		delay1 := CalculateBackoff(config, 1)
		delay2 := CalculateBackoff(config, 2)

		// The ratio between consecutive delays should reflect the multiplier
		expectedDelay1 := time.Duration(float64(config.BaseDelay) * mult)
		if delay1 != expectedDelay1 {
			t.Errorf("Multiplier %f: first delay %v, expected %v",
				mult, delay1, expectedDelay1)
		}

		// Second attempt should be roughly 2x the first (due to exponential component)
		expectedDelay2 := time.Duration(float64(config.BaseDelay) * mult * 2)
		if delay2 != expectedDelay2 {
			t.Errorf("Multiplier %f: second delay %v, expected %v",
				mult, delay2, expectedDelay2)
		}
	}
}

func TestCalculateBackoffConsistency(t *testing.T) {
	config := DefaultBackoffConfig()
	attempt := 3

	// Multiple calls with same parameters should return same result
	delay1 := CalculateBackoff(config, attempt)
	delay2 := CalculateBackoff(config, attempt)
	delay3 := CalculateBackoff(config, attempt)

	if delay1 != delay2 || delay2 != delay3 {
		t.Errorf("Inconsistent delays: %v, %v, %v", delay1, delay2, delay3)
	}
}

func BenchmarkCalculateBackoff(b *testing.B) {
	config := DefaultBackoffConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateBackoff(config, 5)
	}
}

func BenchmarkCalculateBackoffHighAttempt(b *testing.B) {
	config := DefaultBackoffConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateBackoff(config, 100)
	}
}
