package retry

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	assert.Equal(t, 3, policy.MaxRetries)
	assert.Equal(t, 1*time.Second, policy.InitialDelay)
	assert.Equal(t, 30*time.Second, policy.MaxDelay)
	assert.Equal(t, 2.0, policy.Multiplier)
	assert.Equal(t, 0.1, policy.Jitter)
	assert.Nil(t, policy.RetryableStatusCodes)
	assert.Nil(t, policy.RetryableErrors)
}

func TestNoRetryPolicy(t *testing.T) {
	policy := NoRetryPolicy()

	assert.Equal(t, 0, policy.MaxRetries)
	assert.Equal(t, time.Duration(0), policy.InitialDelay)
	assert.Equal(t, time.Duration(0), policy.MaxDelay)
	assert.Equal(t, 1.0, policy.Multiplier)
	assert.Equal(t, 0.0, policy.Jitter)
}

func TestAggressiveRetryPolicy(t *testing.T) {
	policy := AggressiveRetryPolicy()

	assert.Equal(t, 5, policy.MaxRetries)
	assert.Equal(t, 500*time.Millisecond, policy.InitialDelay)
	assert.Equal(t, 10*time.Second, policy.MaxDelay)
	assert.Equal(t, 1.5, policy.Multiplier)
	assert.Equal(t, 0.2, policy.Jitter)
}

func TestConservativeRetryPolicy(t *testing.T) {
	policy := ConservativeRetryPolicy()

	assert.Equal(t, 2, policy.MaxRetries)
	assert.Equal(t, 2*time.Second, policy.InitialDelay)
	assert.Equal(t, 60*time.Second, policy.MaxDelay)
	assert.Equal(t, 3.0, policy.Multiplier)
	assert.Equal(t, 0.05, policy.Jitter)
}

func TestRetryPolicy_ShouldRetry(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name    string
		err     error
		attempt int
		want    bool
	}{
		{
			name:    "nil error",
			err:     nil,
			attempt: 0,
			want:    false,
		},
		{
			name:    "retryable error within limit",
			err:     MarkRetryable(errors.New("temporary error"), 503),
			attempt: 0,
			want:    true,
		},
		{
			name:    "retryable error at limit",
			err:     MarkRetryable(errors.New("temporary error"), 503),
			attempt: 3,
			want:    false,
		},
		{
			name:    "retryable error beyond limit",
			err:     MarkRetryable(errors.New("temporary error"), 503),
			attempt: 5,
			want:    false,
		},
		{
			name:    "non-retryable error",
			err:     MarkNonRetryable(errors.New("permanent error"), 400),
			attempt: 0,
			want:    false,
		},
		{
			name:    "regular error",
			err:     errors.New("regular error"),
			attempt: 0,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.ShouldRetry(tt.err, tt.attempt)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestRetryPolicy_ShouldRetry_CustomStatusCodes(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries: 3,
		RetryableStatusCodes: map[int]bool{
			400: true, // Make 400 retryable (normally not)
			500: true,
		},
	}

	// Error with custom retryable status code (not pre-marked as retryable)
	err400 := NewRetryableError(errors.New("bad request"), false, 400)
	// Should retry because 400 is in custom status codes
	assert.True(t, policy.ShouldRetry(err400, 0))

	// Error with standard retryable status code not in custom list
	err503 := NewRetryableError(errors.New("service unavailable"), false, 503)
	// Should not retry because 503 is not in custom status codes
	assert.False(t, policy.ShouldRetry(err503, 0))
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{
			name:   "delay-seconds format",
			header: "120",
			want:   120 * time.Second,
		},
		{
			name:   "zero seconds",
			header: "0",
			want:   0,
		},
		{
			name:   "negative seconds",
			header: "-5",
			want:   0,
		},
		{
			name:   "empty header",
			header: "",
			want:   0,
		},
		{
			name:   "invalid format",
			header: "invalid",
			want:   0,
		},
		{
			name:   "http-date format (future)",
			header: time.Now().Add(5 * time.Minute).Format(http.TimeFormat),
			want:   5 * time.Minute, // Approximate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.header != "" {
				headers.Set("Retry-After", tt.header)
			}

			result := ParseRetryAfter(headers)

			if tt.name == "http-date format (future)" {
				// For time-based headers, check it's in the right ballpark
				assert.True(t, result > 4*time.Minute && result < 6*time.Minute,
					"Expected duration around 5 minutes, got %v", result)
			} else {
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	// Test with a specific HTTP date
	futureTime := time.Now().Add(10 * time.Second)
	headers := http.Header{}
	headers.Set("Retry-After", futureTime.Format(http.TimeFormat))

	result := ParseRetryAfter(headers)

	// Should be approximately 10 seconds
	assert.True(t, result > 9*time.Second && result < 11*time.Second,
		"Expected duration around 10 seconds, got %v", result)
}

func TestParseRetryAfter_PastHTTPDate(t *testing.T) {
	// Test with a past HTTP date (should return 0)
	pastTime := time.Now().Add(-10 * time.Second)
	headers := http.Header{}
	headers.Set("Retry-After", pastTime.Format(http.TimeFormat))

	result := ParseRetryAfter(headers)
	assert.Equal(t, time.Duration(0), result)
}

func TestRetryPolicy_GetRetryDelay(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.0, // No jitter for predictable testing
	}

	tests := []struct {
		name    string
		attempt int
		headers http.Header
		want    time.Duration
	}{
		{
			name:    "first attempt",
			attempt: 0,
			headers: http.Header{},
			want:    1 * time.Second,
		},
		{
			name:    "second attempt",
			attempt: 1,
			headers: http.Header{},
			want:    2 * time.Second,
		},
		{
			name:    "third attempt",
			attempt: 2,
			headers: http.Header{},
			want:    4 * time.Second,
		},
		{
			name:    "capped at max delay",
			attempt: 5,
			headers: http.Header{},
			want:    10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.GetRetryDelay(tt.attempt, tt.headers)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestRetryPolicy_GetRetryDelay_WithRetryAfter(t *testing.T) {
	policy := DefaultRetryPolicy()

	// Test with Retry-After header
	headers := http.Header{}
	headers.Set("Retry-After", "5")

	result := policy.GetRetryDelay(0, headers)
	assert.Equal(t, 5*time.Second, result)
}

func TestRetryPolicy_GetRetryDelay_RetryAfterExceedsMax(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}

	// Retry-After exceeds MaxDelay
	headers := http.Header{}
	headers.Set("Retry-After", "60")

	result := policy.GetRetryDelay(0, headers)
	assert.Equal(t, 10*time.Second, result) // Should be capped at MaxDelay
}

func TestRetryPolicy_Clone(t *testing.T) {
	original := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryableStatusCodes: map[int]bool{
			500: true,
			503: true,
		},
		RetryableErrors: []error{
			errors.New("error1"),
		},
	}

	clone := original.Clone()

	// Verify all fields are copied
	assert.Equal(t, original.MaxRetries, clone.MaxRetries)
	assert.Equal(t, original.InitialDelay, clone.InitialDelay)
	assert.Equal(t, original.MaxDelay, clone.MaxDelay)
	assert.Equal(t, original.Multiplier, clone.Multiplier)
	assert.Equal(t, original.Jitter, clone.Jitter)

	// Verify deep copy of maps and slices
	// Maps and slices should be equal in content but different instances
	assert.Equal(t, original.RetryableStatusCodes, clone.RetryableStatusCodes)
	assert.Equal(t, len(original.RetryableErrors), len(clone.RetryableErrors))

	// Modify clone and verify original is unchanged
	clone.MaxRetries = 5
	clone.RetryableStatusCodes[429] = true
	assert.Equal(t, 3, original.MaxRetries)
	assert.False(t, original.RetryableStatusCodes[429])
}

func TestRetryPolicy_WithMethods(t *testing.T) {
	original := DefaultRetryPolicy()

	t.Run("WithMaxRetries", func(t *testing.T) {
		modified := original.WithMaxRetries(5)
		assert.Equal(t, 5, modified.MaxRetries)
		assert.Equal(t, 3, original.MaxRetries) // Original unchanged
	})

	t.Run("WithInitialDelay", func(t *testing.T) {
		modified := original.WithInitialDelay(2 * time.Second)
		assert.Equal(t, 2*time.Second, modified.InitialDelay)
		assert.Equal(t, 1*time.Second, original.InitialDelay) // Original unchanged
	})

	t.Run("WithMaxDelay", func(t *testing.T) {
		modified := original.WithMaxDelay(60 * time.Second)
		assert.Equal(t, 60*time.Second, modified.MaxDelay)
		assert.Equal(t, 30*time.Second, original.MaxDelay) // Original unchanged
	})

	t.Run("WithMultiplier", func(t *testing.T) {
		modified := original.WithMultiplier(3.0)
		assert.Equal(t, 3.0, modified.Multiplier)
		assert.Equal(t, 2.0, original.Multiplier) // Original unchanged
	})

	t.Run("WithJitter", func(t *testing.T) {
		modified := original.WithJitter(0.5)
		assert.Equal(t, 0.5, modified.Jitter)
		assert.Equal(t, 0.1, original.Jitter) // Original unchanged
	})
}

func TestRetryPolicy_WithMethods_Chaining(t *testing.T) {
	policy := DefaultRetryPolicy().
		WithMaxRetries(5).
		WithInitialDelay(500 * time.Millisecond).
		WithMaxDelay(20 * time.Second).
		WithMultiplier(1.5).
		WithJitter(0.2)

	assert.Equal(t, 5, policy.MaxRetries)
	assert.Equal(t, 500*time.Millisecond, policy.InitialDelay)
	assert.Equal(t, 20*time.Second, policy.MaxDelay)
	assert.Equal(t, 1.5, policy.Multiplier)
	assert.Equal(t, 0.2, policy.Jitter)
}

func TestRetryPolicy_isRetryableStatusCode(t *testing.T) {
	t.Run("default status codes", func(t *testing.T) {
		policy := DefaultRetryPolicy()

		assert.True(t, policy.isRetryableStatusCode(429))
		assert.True(t, policy.isRetryableStatusCode(500))
		assert.True(t, policy.isRetryableStatusCode(503))
		assert.False(t, policy.isRetryableStatusCode(400))
		assert.False(t, policy.isRetryableStatusCode(404))
	})

	t.Run("custom status codes", func(t *testing.T) {
		policy := &RetryPolicy{
			RetryableStatusCodes: map[int]bool{
				400: true, // Custom: make 400 retryable
				500: true,
			},
		}

		assert.True(t, policy.isRetryableStatusCode(400))
		assert.True(t, policy.isRetryableStatusCode(500))
		assert.False(t, policy.isRetryableStatusCode(503)) // Not in custom list
	})
}

func TestRetryPolicy_EdgeCases(t *testing.T) {
	t.Run("zero max retries", func(t *testing.T) {
		policy := NoRetryPolicy()
		err := MarkRetryable(errors.New("test"), 503)

		assert.False(t, policy.ShouldRetry(err, 0))
	})

	t.Run("negative attempt", func(t *testing.T) {
		policy := DefaultRetryPolicy()
		err := MarkRetryable(errors.New("test"), 503)

		// Negative attempt should still be allowed (counts as first attempt)
		assert.True(t, policy.ShouldRetry(err, -1))
	})

	t.Run("nil policy fields", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxRetries:           3,
			RetryableStatusCodes: nil,
			RetryableErrors:      nil,
		}

		clone := policy.Clone()
		assert.Nil(t, clone.RetryableStatusCodes)
		assert.Nil(t, clone.RetryableErrors)
	})
}
