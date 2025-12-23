package retry

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRetryableError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		retryable  bool
		statusCode int
		wantNil    bool
	}{
		{
			name:       "retryable error",
			err:        errors.New("temporary failure"),
			retryable:  true,
			statusCode: 503,
			wantNil:    false,
		},
		{
			name:       "non-retryable error",
			err:        errors.New("permanent failure"),
			retryable:  false,
			statusCode: 400,
			wantNil:    false,
		},
		{
			name:       "nil error",
			err:        nil,
			retryable:  true,
			statusCode: 500,
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewRetryableError(tt.err, tt.retryable, tt.statusCode)

			if tt.wantNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)

			// Check error message
			assert.Equal(t, tt.err.Error(), result.Error())

			// Check retryable interface
			retryErr, ok := result.(RetryableError)
			require.True(t, ok)
			assert.Equal(t, tt.retryable, retryErr.IsRetryable())

			// Check status code
			concreteErr, ok := result.(*retryableError)
			require.True(t, ok)
			assert.Equal(t, tt.statusCode, concreteErr.StatusCode())

			// Check unwrap
			assert.Equal(t, tt.err, errors.Unwrap(result))
		})
	}
}

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		{"507 Insufficient Storage", http.StatusInsufficientStorage, true},
		{"511 Network Auth Required", 511, true},
		{"200 OK", http.StatusOK, false},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableStatusCode(tt.statusCode)
			assert.Equal(t, tt.want, result)
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
			name: "retryable error",
			err:  NewRetryableError(errors.New("test"), true, 503),
			want: true,
		},
		{
			name: "non-retryable error",
			err:  NewRetryableError(errors.New("test"), false, 400),
			want: false,
		},
		{
			name: "wrapped retryable error",
			err:  fmt.Errorf("wrapped: %w", NewRetryableError(errors.New("test"), true, 500)),
			want: true,
		},
		{
			name: "wrapped non-retryable error",
			err:  fmt.Errorf("wrapped: %w", NewRetryableError(errors.New("test"), false, 400)),
			want: false,
		},
		{
			name: "regular error",
			err:  errors.New("regular error"),
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
			result := IsRetryableError(tt.err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "error with status code 503",
			err:  NewRetryableError(errors.New("test"), true, 503),
			want: 503,
		},
		{
			name: "error with status code 400",
			err:  NewRetryableError(errors.New("test"), false, 400),
			want: 400,
		},
		{
			name: "wrapped error with status code",
			err:  fmt.Errorf("wrapped: %w", NewRetryableError(errors.New("test"), true, 500)),
			want: 500,
		},
		{
			name: "regular error without status code",
			err:  errors.New("regular error"),
			want: 0,
		},
		{
			name: "nil error",
			err:  nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStatusCode(tt.err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMarkRetryable(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		wantNil    bool
	}{
		{
			name:       "mark error as retryable",
			err:        errors.New("test error"),
			statusCode: 503,
			wantNil:    false,
		},
		{
			name:       "mark nil error",
			err:        nil,
			statusCode: 500,
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MarkRetryable(tt.err, tt.statusCode)

			if tt.wantNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.True(t, IsRetryableError(result))
			assert.Equal(t, tt.statusCode, GetStatusCode(result))
		})
	}
}

func TestMarkNonRetryable(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		wantNil    bool
	}{
		{
			name:       "mark error as non-retryable",
			err:        errors.New("test error"),
			statusCode: 400,
			wantNil:    false,
		},
		{
			name:       "mark nil error",
			err:        nil,
			statusCode: 400,
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MarkNonRetryable(tt.err, tt.statusCode)

			if tt.wantNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.False(t, IsRetryableError(result))
			assert.Equal(t, tt.statusCode, GetStatusCode(result))
		})
	}
}

func TestRetryableError_Error(t *testing.T) {
	baseErr := errors.New("base error message")
	retryErr := NewRetryableError(baseErr, true, 503)

	assert.Equal(t, "base error message", retryErr.Error())
}

func TestRetryableError_Unwrap(t *testing.T) {
	baseErr := errors.New("base error")
	retryErr := NewRetryableError(baseErr, true, 503)

	unwrapped := errors.Unwrap(retryErr)
	assert.Equal(t, baseErr, unwrapped)
}

func TestRetryableError_IsRetryable(t *testing.T) {
	retryableErr := NewRetryableError(errors.New("test"), true, 503)
	nonRetryableErr := NewRetryableError(errors.New("test"), false, 400)

	assert.True(t, IsRetryableError(retryableErr))
	assert.False(t, IsRetryableError(nonRetryableErr))
}

func TestRetryableError_StatusCode(t *testing.T) {
	err := NewRetryableError(errors.New("test"), true, 503)

	concreteErr, ok := err.(*retryableError)
	require.True(t, ok)
	assert.Equal(t, 503, concreteErr.StatusCode())
}

// TestRetryableError_Interface verifies that retryableError implements RetryableError
func TestRetryableError_Interface(t *testing.T) {
	var _ RetryableError = (*retryableError)(nil)
}

// TestErrorChaining tests that errors can be properly chained and unwrapped
func TestErrorChaining(t *testing.T) {
	baseErr := errors.New("base error")
	retryErr := MarkRetryable(baseErr, 503)
	wrappedErr := fmt.Errorf("operation failed: %w", retryErr)
	doubleWrappedErr := fmt.Errorf("request failed: %w", wrappedErr)

	// Should be able to detect retryability through multiple wrapping levels
	assert.True(t, IsRetryableError(doubleWrappedErr))
	assert.Equal(t, 503, GetStatusCode(doubleWrappedErr))

	// Should be able to unwrap to the original error
	var retryable RetryableError
	assert.True(t, errors.As(doubleWrappedErr, &retryable))
	assert.True(t, retryable.IsRetryable())
}

// TestRetryableStatusCodes verifies all retryable status codes are properly defined
func TestRetryableStatusCodes(t *testing.T) {
	expectedRetryable := []int{
		429, // Too Many Requests
		500, // Internal Server Error
		502, // Bad Gateway
		503, // Service Unavailable
		504, // Gateway Timeout
		507, // Insufficient Storage
		511, // Network Authentication Required
	}

	for _, code := range expectedRetryable {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			assert.True(t, IsRetryableStatusCode(code), "Status code %d should be retryable", code)
		})
	}
}

// TestNonRetryableStatusCodes verifies non-retryable status codes
func TestNonRetryableStatusCodes(t *testing.T) {
	nonRetryable := []int{
		200, // OK
		201, // Created
		400, // Bad Request
		401, // Unauthorized
		403, // Forbidden
		404, // Not Found
		405, // Method Not Allowed
		422, // Unprocessable Entity
	}

	for _, code := range nonRetryable {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			assert.False(t, IsRetryableStatusCode(code), "Status code %d should not be retryable", code)
		})
	}
}
