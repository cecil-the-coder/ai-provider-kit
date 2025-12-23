package sentinel

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		check func(error) bool
	}{
		// Authentication errors
		{
			name:  "ErrNotAuthenticated",
			err:   ErrNotAuthenticated,
			check: func(e error) bool { return errors.Is(e, ErrNotAuthenticated) },
		},
		{
			name:  "ErrUnauthorized",
			err:   ErrUnauthorized,
			check: func(e error) bool { return errors.Is(e, ErrUnauthorized) },
		},
		// Rate limiting
		{
			name:  "ErrRateLimited",
			err:   ErrRateLimited,
			check: func(e error) bool { return errors.Is(e, ErrRateLimited) },
		},
		// Request errors
		{
			name:  "ErrInvalidRequest",
			err:   ErrInvalidRequest,
			check: func(e error) bool { return errors.Is(e, ErrInvalidRequest) },
		},
		{
			name:  "ErrModelNotFound",
			err:   ErrModelNotFound,
			check: func(e error) bool { return errors.Is(e, ErrModelNotFound) },
		},
		{
			name:  "ErrContextLengthExceeded",
			err:   ErrContextLengthExceeded,
			check: func(e error) bool { return errors.Is(e, ErrContextLengthExceeded) },
		},
		{
			name:  "ErrContentFiltered",
			err:   ErrContentFiltered,
			check: func(e error) bool { return errors.Is(e, ErrContentFiltered) },
		},
		// Network/timeout errors
		{
			name:  "ErrTimeout",
			err:   ErrTimeout,
			check: func(e error) bool { return errors.Is(e, ErrTimeout) },
		},
		{
			name:  "ErrCancelled",
			err:   ErrCancelled,
			check: func(e error) bool { return errors.Is(e, ErrCancelled) },
		},
		{
			name:  "ErrNetworkError",
			err:   ErrNetworkError,
			check: func(e error) bool { return errors.Is(e, ErrNetworkError) },
		},
		// Server errors
		{
			name:  "ErrServerError",
			err:   ErrServerError,
			check: func(e error) bool { return errors.Is(e, ErrServerError) },
		},
		{
			name:  "ErrServiceUnavailable",
			err:   ErrServiceUnavailable,
			check: func(e error) bool { return errors.Is(e, ErrServiceUnavailable) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(tt.err) {
				t.Errorf("%s: error check failed", tt.name)
			}
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	tests := []struct {
		name          string
		sentinelError error
		wrapped       error
	}{
		{
			name:          "wrapped ErrNotAuthenticated",
			sentinelError: ErrNotAuthenticated,
			wrapped:       errors.New("authentication failed"),
		},
		{
			name:          "wrapped ErrRateLimited",
			sentinelError: ErrRateLimited,
			wrapped:       errors.New("too many requests"),
		},
		{
			name:          "wrapped ErrModelNotFound",
			sentinelError: ErrModelNotFound,
			wrapped:       errors.New("model not available"),
		},
		{
			name:          "wrapped ErrTimeout",
			sentinelError: ErrTimeout,
			wrapped:       errors.New("request timed out"),
		},
		{
			name:          "wrapped ErrCancelled",
			sentinelError: ErrCancelled,
			wrapped:       errors.New("request cancelled"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrappedErr := fmt.Errorf("wrapped: %w: %v", tt.sentinelError, tt.wrapped)
			if !errors.Is(wrappedErr, tt.sentinelError) {
				t.Errorf("wrapped error should still be identified as %T", tt.sentinelError)
			}
		})
	}
}

func TestIsAuthenticationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ErrNotAuthenticated",
			err:  ErrNotAuthenticated,
			want: true,
		},
		{
			name: "ErrUnauthorized",
			err:  ErrUnauthorized,
			want: true,
		},
		{
			name: "wrapped ErrNotAuthenticated",
			err:  fmt.Errorf("auth failed: %w", ErrNotAuthenticated),
			want: true,
		},
		{
			name: "ErrRateLimited",
			err:  ErrRateLimited,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error",
			err:  errors.New("some other error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthenticationError(tt.err); got != tt.want {
				t.Errorf("IsAuthenticationError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ErrRateLimited",
			err:  ErrRateLimited,
			want: true,
		},
		{
			name: "ErrTimeout",
			err:  ErrTimeout,
			want: true,
		},
		{
			name: "ErrNetworkError",
			err:  ErrNetworkError,
			want: true,
		},
		{
			name: "ErrServerError",
			err:  ErrServerError,
			want: true,
		},
		{
			name: "ErrServiceUnavailable",
			err:  ErrServiceUnavailable,
			want: true,
		},
		{
			name: "wrapped ErrRateLimited",
			err:  fmt.Errorf("rate limit exceeded: %w", ErrRateLimited),
			want: true,
		},
		{
			name: "ErrNotAuthenticated",
			err:  ErrNotAuthenticated,
			want: false,
		},
		{
			name: "ErrInvalidRequest",
			err:  ErrInvalidRequest,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableError(tt.err); got != tt.want {
				t.Errorf("IsRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsClientError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "ErrNotAuthenticated",
			err:  ErrNotAuthenticated,
			want: true,
		},
		{
			name: "ErrUnauthorized",
			err:  ErrUnauthorized,
			want: true,
		},
		{
			name: "ErrInvalidRequest",
			err:  ErrInvalidRequest,
			want: true,
		},
		{
			name: "ErrModelNotFound",
			err:  ErrModelNotFound,
			want: true,
		},
		{
			name: "ErrContextLengthExceeded",
			err:  ErrContextLengthExceeded,
			want: true,
		},
		{
			name: "ErrContentFiltered",
			err:  ErrContentFiltered,
			want: true,
		},
		{
			name: "wrapped ErrModelNotFound",
			err:  fmt.Errorf("model not found: %w", ErrModelNotFound),
			want: true,
		},
		{
			name: "ErrRateLimited",
			err:  ErrRateLimited,
			want: false,
		},
		{
			name: "ErrTimeout",
			err:  ErrTimeout,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsClientError(tt.err); got != tt.want {
				t.Errorf("IsClientError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSentinelErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "ErrNotAuthenticated message",
			err:  ErrNotAuthenticated,
			want: "not authenticated",
		},
		{
			name: "ErrRateLimited message",
			err:  ErrRateLimited,
			want: "rate limited",
		},
		{
			name: "ErrTimeout message",
			err:  ErrTimeout,
			want: "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got == "" {
				t.Errorf("error message is empty")
			}
			// Check if want is a substring of got
			if len(tt.want) > 0 && !contains(got, tt.want) {
				t.Errorf("error message %q does not contain %q", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
