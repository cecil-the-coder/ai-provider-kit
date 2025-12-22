package common

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		apiErr   *APIError
		expected string
	}{
		{
			name: "with message",
			apiErr: &APIError{
				StatusCode: http.StatusUnauthorized,
				Type:       APIErrorTypeAuth,
				Message:    "authentication failed",
			},
			expected: "[auth] authentication failed (status: 401)",
		},
		{
			name: "without message",
			apiErr: &APIError{
				StatusCode: http.StatusInternalServerError,
				Type:       APIErrorTypeServer,
			},
			expected: "[server_error] HTTP 500 error",
		},
		{
			name: "rate limit error",
			apiErr: &APIError{
				StatusCode: http.StatusTooManyRequests,
				Type:       APIErrorTypeRateLimit,
				Message:    "rate limit exceeded",
			},
			expected: "[rate_limit] rate limit exceeded (status: 429)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.apiErr.Error())
		})
	}
}

func TestAPIError_IsRateLimit(t *testing.T) {
	tests := []struct {
		name     string
		apiErr   *APIError
		expected bool
	}{
		{
			name: "rate limit error",
			apiErr: &APIError{
				Type: APIErrorTypeRateLimit,
			},
			expected: true,
		},
		{
			name: "auth error",
			apiErr: &APIError{
				Type: APIErrorTypeAuth,
			},
			expected: false,
		},
		{
			name: "server error",
			apiErr: &APIError{
				Type: APIErrorTypeServer,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.apiErr.IsRateLimit())
		})
	}
}

func TestAPIError_IsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		apiErr   *APIError
		expected bool
	}{
		{
			name: "retryable error",
			apiErr: &APIError{
				Retryable: true,
			},
			expected: true,
		},
		{
			name: "non-retryable error",
			apiErr: &APIError{
				Retryable: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.apiErr.IsRetryable())
		})
	}
}

func TestAPIErrorType_Constants(t *testing.T) {
	// Test that error type constants are correctly defined
	assert.Equal(t, APIErrorType("rate_limit"), APIErrorTypeRateLimit)
	assert.Equal(t, APIErrorType("auth"), APIErrorTypeAuth)
	assert.Equal(t, APIErrorType("not_found"), APIErrorTypeNotFound)
	assert.Equal(t, APIErrorType("invalid_request"), APIErrorTypeInvalidRequest)
	assert.Equal(t, APIErrorType("server_error"), APIErrorTypeServer)
	assert.Equal(t, APIErrorType("unknown"), APIErrorTypeUnknown)
}

// TestErrorClassifierInterface verifies the ErrorClassifier interface can be implemented
func TestErrorClassifierInterface(t *testing.T) {
	// Create a mock implementation
	var _ ErrorClassifier = (*mockErrorClassifier)(nil)
}

// mockErrorClassifier is a test implementation of ErrorClassifier
type mockErrorClassifier struct{}

func (m *mockErrorClassifier) Classify(statusCode int, body []byte) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Type:       APIErrorTypeUnknown,
		Message:    "mock error",
		RawBody:    string(body),
		Retryable:  false,
	}
}

func TestMockErrorClassifier(t *testing.T) {
	classifier := &mockErrorClassifier{}
	apiErr := classifier.Classify(http.StatusBadRequest, []byte("test error"))

	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, APIErrorTypeUnknown, apiErr.Type)
	assert.Equal(t, "mock error", apiErr.Message)
	assert.Equal(t, "test error", apiErr.RawBody)
	assert.False(t, apiErr.Retryable)
}

// TestAPIError_AsError verifies APIError implements error interface
func TestAPIError_AsError(t *testing.T) {
	var err error = &APIError{
		StatusCode: http.StatusInternalServerError,
		Type:       APIErrorTypeServer,
		Message:    "test error",
	}

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test error")
}
