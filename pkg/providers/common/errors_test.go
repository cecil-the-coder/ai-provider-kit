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

func TestClassifyHTTPError(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		body           []byte
		expectedType   APIErrorType
		expectedMsg    string
		expectedRetry  bool
	}{
		{
			name:          "unauthorized",
			statusCode:    http.StatusUnauthorized,
			body:          []byte("Invalid API key"),
			expectedType:  APIErrorTypeAuth,
			expectedMsg:   "authentication failed: Invalid API key",
			expectedRetry: false,
		},
		{
			name:          "forbidden",
			statusCode:    http.StatusForbidden,
			body:          []byte("Access denied"),
			expectedType:  APIErrorTypeAuth,
			expectedMsg:   "authentication failed: Access denied",
			expectedRetry: false,
		},
		{
			name:          "not found",
			statusCode:    http.StatusNotFound,
			body:          []byte("Model not found"),
			expectedType:  APIErrorTypeNotFound,
			expectedMsg:   "resource not found: Model not found",
			expectedRetry: false,
		},
		{
			name:          "bad request",
			statusCode:    http.StatusBadRequest,
			body:          []byte("Invalid parameters"),
			expectedType:  APIErrorTypeInvalidRequest,
			expectedMsg:   "invalid request: Invalid parameters",
			expectedRetry: false,
		},
		{
			name:          "rate limit",
			statusCode:    http.StatusTooManyRequests,
			body:          []byte("Rate limit exceeded"),
			expectedType:  APIErrorTypeRateLimit,
			expectedMsg:   "rate limit exceeded: Rate limit exceeded",
			expectedRetry: true,
		},
		{
			name:          "internal server error",
			statusCode:    http.StatusInternalServerError,
			body:          []byte("Internal error"),
			expectedType:  APIErrorTypeServer,
			expectedMsg:   "server error: Internal error",
			expectedRetry: true,
		},
		{
			name:          "bad gateway",
			statusCode:    http.StatusBadGateway,
			body:          []byte("Bad gateway"),
			expectedType:  APIErrorTypeServer,
			expectedMsg:   "server error: Bad gateway",
			expectedRetry: true,
		},
		{
			name:          "service unavailable",
			statusCode:    http.StatusServiceUnavailable,
			body:          []byte("Service unavailable"),
			expectedType:  APIErrorTypeServer,
			expectedMsg:   "server error: Service unavailable",
			expectedRetry: true,
		},
		{
			name:          "unknown status code",
			statusCode:    418, // I'm a teapot
			body:          []byte("I'm a teapot"),
			expectedType:  APIErrorTypeUnknown,
			expectedMsg:   "unknown error with status 418: I'm a teapot",
			expectedRetry: false,
		},
		{
			name:          "empty body",
			statusCode:    http.StatusUnauthorized,
			body:          []byte(""),
			expectedType:  APIErrorTypeAuth,
			expectedMsg:   "authentication failed",
			expectedRetry: false,
		},
		{
			name:          "empty object body",
			statusCode:    http.StatusBadRequest,
			body:          []byte("{}"),
			expectedType:  APIErrorTypeInvalidRequest,
			expectedMsg:   "invalid request",
			expectedRetry: false,
		},
		{
			name:          "large body",
			statusCode:    http.StatusInternalServerError,
			body:          []byte(string(make([]byte, 2000))), // Very large body
			expectedType:  APIErrorTypeServer,
			expectedMsg:   "server error",
			expectedRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := ClassifyHTTPError(tt.statusCode, tt.body)

			assert.Equal(t, tt.statusCode, apiErr.StatusCode)
			assert.Equal(t, tt.expectedType, apiErr.Type)
			assert.Contains(t, apiErr.Message, tt.expectedMsg)
			assert.Equal(t, tt.expectedRetry, apiErr.Retryable)
			assert.Equal(t, string(tt.body), apiErr.RawBody)
		})
	}
}

func TestClassifyHTTPError_RawBodyPreservation(t *testing.T) {
	body := []byte(`{"error": "detailed error message"}`)
	apiErr := ClassifyHTTPError(http.StatusBadRequest, body)

	assert.Equal(t, string(body), apiErr.RawBody)
	assert.NotEmpty(t, apiErr.RawBody)
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

// TestClassifyHTTPError_AllStatusCodes tests various HTTP status code classifications
func TestClassifyHTTPError_AllStatusCodes(t *testing.T) {
	testCases := []struct {
		statusCode    int
		expectedType  APIErrorType
		expectedRetry bool
	}{
		{200, APIErrorTypeUnknown, false}, // Success codes should not normally be classified
		{201, APIErrorTypeUnknown, false},
		{400, APIErrorTypeInvalidRequest, false},
		{401, APIErrorTypeAuth, false},
		{403, APIErrorTypeAuth, false},
		{404, APIErrorTypeNotFound, false},
		{405, APIErrorTypeUnknown, false},
		{429, APIErrorTypeRateLimit, true},
		{500, APIErrorTypeServer, true},
		{501, APIErrorTypeServer, true},
		{502, APIErrorTypeServer, true},
		{503, APIErrorTypeServer, true},
		{504, APIErrorTypeServer, true},
	}

	for _, tc := range testCases {
		t.Run(http.StatusText(tc.statusCode), func(t *testing.T) {
			apiErr := ClassifyHTTPError(tc.statusCode, nil)
			assert.Equal(t, tc.expectedType, apiErr.Type)
			assert.Equal(t, tc.expectedRetry, apiErr.Retryable)
		})
	}
}
