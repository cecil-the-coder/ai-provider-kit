package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestProviderError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ProviderError
		expected string
	}{
		{
			name: "error with status code",
			err: &ProviderError{
				Provider:   ProviderTypeCerebras,
				Message:    "request failed",
				StatusCode: 401,
				Code:       ErrCodeAuthentication,
			},
			expected: "[cerebras] request failed (status=401, code=authentication)",
		},
		{
			name: "error without status code",
			err: &ProviderError{
				Provider: ProviderTypeOpenAI,
				Message:  "network timeout",
				Code:     ErrCodeTimeout,
			},
			expected: "[openai] network timeout (code=timeout)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProviderError_Unwrap(t *testing.T) {
	originalErr := errors.New("underlying error")
	providerErr := &ProviderError{
		Provider:    ProviderTypeCerebras,
		Message:     "wrapped error",
		Code:        ErrCodeUnknown,
		OriginalErr: originalErr,
	}

	unwrapped := providerErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}

	// Test with errors.Is
	if !errors.Is(providerErr, originalErr) {
		t.Error("errors.Is should recognize the wrapped error")
	}
}

func TestProviderError_IsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected bool
	}{
		{"rate limit is retryable", ErrCodeRateLimit, true},
		{"server error is retryable", ErrCodeServerError, true},
		{"timeout is retryable", ErrCodeTimeout, true},
		{"network error is retryable", ErrCodeNetwork, true},
		{"authentication is not retryable", ErrCodeAuthentication, false},
		{"invalid request is not retryable", ErrCodeInvalidRequest, false},
		{"not found is not retryable", ErrCodeNotFound, false},
		{"content filter is not retryable", ErrCodeContentFilter, false},
		{"context length is not retryable", ErrCodeContextLength, false},
		{"unknown is not retryable", ErrCodeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ProviderError{
				Provider: ProviderTypeCerebras,
				Message:  "test error",
				Code:     tt.code,
			}
			if got := err.IsRetryable(); got != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProviderError_ChainableMethods(t *testing.T) {
	originalErr := errors.New("original")
	err := NewProviderError(ProviderTypeCerebras, ErrCodeUnknown, "test").
		WithOperation("chat_completion").
		WithStatusCode(500).
		WithOriginalErr(originalErr).
		WithRequestID("req-123").
		WithRetryAfter(60)

	if err.Operation != "chat_completion" {
		t.Errorf("Operation = %v, want chat_completion", err.Operation)
	}
	if err.StatusCode != 500 {
		t.Errorf("StatusCode = %v, want 500", err.StatusCode)
	}
	if err.OriginalErr != originalErr {
		t.Errorf("OriginalErr = %v, want %v", err.OriginalErr, originalErr)
	}
	if err.RequestID != "req-123" {
		t.Errorf("RequestID = %v, want req-123", err.RequestID)
	}
	if err.RetryAfter != 60 {
		t.Errorf("RetryAfter = %v, want 60", err.RetryAfter)
	}
}

func TestNewProviderError(t *testing.T) {
	err := NewProviderError(ProviderTypeCerebras, ErrCodeInvalidRequest, "test message")

	if err.Provider != ProviderTypeCerebras {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeCerebras)
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeInvalidRequest)
	}
	if err.Message != "test message" {
		t.Errorf("Message = %v, want test message", err.Message)
	}
}

func TestNewAuthError(t *testing.T) {
	err := NewAuthError(ProviderTypeOpenAI, "invalid API key")

	if err.Provider != ProviderTypeOpenAI {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeOpenAI)
	}
	if err.Code != ErrCodeAuthentication {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeAuthentication)
	}
	if err.Message != "invalid API key" {
		t.Errorf("Message = %v, want invalid API key", err.Message)
	}
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError(ProviderTypeGemini, 120)

	if err.Provider != ProviderTypeGemini {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeGemini)
	}
	if err.Code != ErrCodeRateLimit {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeRateLimit)
	}
	if err.RetryAfter != 120 {
		t.Errorf("RetryAfter = %v, want 120", err.RetryAfter)
	}
	if err.Message != "rate limit exceeded" {
		t.Errorf("Message = %v, want rate limit exceeded", err.Message)
	}
}

func TestNewServerError(t *testing.T) {
	err := NewServerError(ProviderTypeAnthropic, 503, "service unavailable")

	if err.Provider != ProviderTypeAnthropic {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeAnthropic)
	}
	if err.Code != ErrCodeServerError {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeServerError)
	}
	if err.StatusCode != 503 {
		t.Errorf("StatusCode = %v, want 503", err.StatusCode)
	}
	if err.Message != "service unavailable" {
		t.Errorf("Message = %v, want service unavailable", err.Message)
	}
}

func TestNewInvalidRequestError(t *testing.T) {
	err := NewInvalidRequestError(ProviderTypeQwen, "invalid model")

	if err.Provider != ProviderTypeQwen {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeQwen)
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeInvalidRequest)
	}
	if err.Message != "invalid model" {
		t.Errorf("Message = %v, want invalid model", err.Message)
	}
}

func TestNewNetworkError(t *testing.T) {
	err := NewNetworkError(ProviderTypeCerebras, "connection refused")

	if err.Provider != ProviderTypeCerebras {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeCerebras)
	}
	if err.Code != ErrCodeNetwork {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeNetwork)
	}
	if err.Message != "connection refused" {
		t.Errorf("Message = %v, want connection refused", err.Message)
	}
}

func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError(ProviderTypeOpenRouter, "request timeout")

	if err.Provider != ProviderTypeOpenRouter {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeOpenRouter)
	}
	if err.Code != ErrCodeTimeout {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeTimeout)
	}
	if err.Message != "request timeout" {
		t.Errorf("Message = %v, want request timeout", err.Message)
	}
}

func TestNewContextLengthError(t *testing.T) {
	err := NewContextLengthError(ProviderTypeOpenAI, "context too long")

	if err.Provider != ProviderTypeOpenAI {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeOpenAI)
	}
	if err.Code != ErrCodeContextLength {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeContextLength)
	}
	if err.Message != "context too long" {
		t.Errorf("Message = %v, want context too long", err.Message)
	}
}

func TestNewContentFilterError(t *testing.T) {
	err := NewContentFilterError(ProviderTypeGemini, "content blocked")

	if err.Provider != ProviderTypeGemini {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeGemini)
	}
	if err.Code != ErrCodeContentFilter {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeContentFilter)
	}
	if err.Message != "content blocked" {
		t.Errorf("Message = %v, want content blocked", err.Message)
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError(ProviderTypeAnthropic, "model not found")

	if err.Provider != ProviderTypeAnthropic {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeAnthropic)
	}
	if err.Code != ErrCodeNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeNotFound)
	}
	if err.Message != "model not found" {
		t.Errorf("Message = %v, want model not found", err.Message)
	}
}

func TestClassifyHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   ErrorCode
	}{
		{"401 unauthorized", http.StatusUnauthorized, ErrCodeAuthentication},
		{"403 forbidden", http.StatusForbidden, ErrCodeAuthentication},
		{"429 too many requests", http.StatusTooManyRequests, ErrCodeRateLimit},
		{"400 bad request", http.StatusBadRequest, ErrCodeInvalidRequest},
		{"404 not found", http.StatusNotFound, ErrCodeNotFound},
		{"500 internal server error", http.StatusInternalServerError, ErrCodeServerError},
		{"502 bad gateway", http.StatusBadGateway, ErrCodeServerError},
		{"503 service unavailable", http.StatusServiceUnavailable, ErrCodeServerError},
		{"504 gateway timeout", http.StatusGatewayTimeout, ErrCodeServerError},
		{"200 ok", http.StatusOK, ErrCodeUnknown},
		{"418 teapot", http.StatusTeapot, ErrCodeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyHTTPError(tt.statusCode); got != tt.expected {
				t.Errorf("ClassifyHTTPError(%d) = %v, want %v", tt.statusCode, got, tt.expected)
			}
		})
	}
}

func TestProviderError_ErrorsAs(t *testing.T) {
	originalErr := fmt.Errorf("base error")
	providerErr := NewNetworkError(ProviderTypeCerebras, "network failed").
		WithOriginalErr(originalErr)

	var targetErr *ProviderError
	if !errors.As(providerErr, &targetErr) {
		t.Error("errors.As should recognize ProviderError")
	}
	if targetErr.Code != ErrCodeNetwork {
		t.Errorf("errors.As target Code = %v, want %v", targetErr.Code, ErrCodeNetwork)
	}
}

func TestProviderError_WithChaining(t *testing.T) {
	// Test that all With* methods return the same error instance
	err := NewProviderError(ProviderTypeCerebras, ErrCodeUnknown, "test")
	err2 := err.WithOperation("test_op")
	err3 := err2.WithStatusCode(500)
	err4 := err3.WithRequestID("req-456")

	// All should point to the same instance
	if err != err2 || err2 != err3 || err3 != err4 {
		t.Error("With* methods should return the same error instance for chaining")
	}

	// Verify all fields are set
	if err.Operation != "test_op" {
		t.Errorf("Operation = %v, want test_op", err.Operation)
	}
	if err.StatusCode != 500 {
		t.Errorf("StatusCode = %v, want 500", err.StatusCode)
	}
	if err.RequestID != "req-456" {
		t.Errorf("RequestID = %v, want req-456", err.RequestID)
	}
}

func TestProviderError_CompleteErrorMessage(t *testing.T) {
	originalErr := errors.New("connection reset")
	err := NewNetworkError(ProviderTypeCerebras, "failed to connect").
		WithOperation("chat_completion").
		WithStatusCode(0).
		WithOriginalErr(originalErr).
		WithRequestID("req-789")

	errMsg := err.Error()
	expected := "[cerebras] failed to connect (code=network)"
	if errMsg != expected {
		t.Errorf("Error() = %v, want %v", errMsg, expected)
	}

	// Test that we can still unwrap
	if !errors.Is(err, originalErr) {
		t.Error("should be able to unwrap to original error")
	}
}

func TestProviderError_RetryableScenarios(t *testing.T) {
	// Simulate a rate limit scenario
	rateLimitErr := NewRateLimitError(ProviderTypeCerebras, 60).
		WithOperation("chat_completion").
		WithStatusCode(429)

	if !rateLimitErr.IsRetryable() {
		t.Error("rate limit error should be retryable")
	}
	if rateLimitErr.RetryAfter != 60 {
		t.Errorf("RetryAfter = %v, want 60", rateLimitErr.RetryAfter)
	}

	// Simulate a server error scenario
	serverErr := NewServerError(ProviderTypeCerebras, 503, "service unavailable").
		WithOperation("list_models")

	if !serverErr.IsRetryable() {
		t.Error("server error should be retryable")
	}

	// Simulate a non-retryable auth error
	authErr := NewAuthError(ProviderTypeCerebras, "invalid key").
		WithOperation("chat_completion").
		WithStatusCode(401)

	if authErr.IsRetryable() {
		t.Error("auth error should not be retryable")
	}
}

func TestProviderError_NilOriginalErr(t *testing.T) {
	err := NewProviderError(ProviderTypeCerebras, ErrCodeUnknown, "test")

	if err.Unwrap() != nil {
		t.Error("Unwrap() should return nil when OriginalErr is not set")
	}
}

func TestProviderError_WithRequestDump(t *testing.T) {
	err := NewProviderError(ProviderTypeOpenAI, ErrCodeUnknown, "test")
	dump := "POST /v1/chat/completions HTTP/1.1\nAuthorization: Bearer sk-1234"

	result := err.WithRequestDump(dump)

	if result != err {
		t.Error("WithRequestDump should return the same error instance")
	}
	if err.RequestDump != dump {
		t.Errorf("RequestDump = %v, want %v", err.RequestDump, dump)
	}
}

func TestProviderError_WithResponseDump(t *testing.T) {
	err := NewProviderError(ProviderTypeOpenAI, ErrCodeUnknown, "test")
	dump := "HTTP/1.1 401 Unauthorized\nContent-Type: application/json"

	result := err.WithResponseDump(dump)

	if result != err {
		t.Error("WithResponseDump should return the same error instance")
	}
	if err.ResponseDump != dump {
		t.Errorf("ResponseDump = %v, want %v", err.ResponseDump, dump)
	}
}

func TestProviderError_WithRequestLatency(t *testing.T) {
	err := NewProviderError(ProviderTypeOpenAI, ErrCodeUnknown, "test")
	latency := 250 * time.Millisecond

	result := err.WithRequestLatency(latency)

	if result != err {
		t.Error("WithRequestLatency should return the same error instance")
	}
	if err.RequestLatency != latency {
		t.Errorf("RequestLatency = %v, want %v", err.RequestLatency, latency)
	}
}

func TestProviderError_WithCorrelationID(t *testing.T) {
	err := NewProviderError(ProviderTypeOpenAI, ErrCodeUnknown, "test")
	correlationID := "corr-12345"

	result := err.WithCorrelationID(correlationID)

	if result != err {
		t.Error("WithCorrelationID should return the same error instance")
	}
	if err.CorrelationID != correlationID {
		t.Errorf("CorrelationID = %v, want %v", err.CorrelationID, correlationID)
	}
}

func TestProviderError_DumpRequest(t *testing.T) {
	tests := []struct {
		name           string
		setupReq       func() *http.Request
		wantContain    []string
		wantNotContain []string
	}{
		{
			name: "masks bearer token",
			setupReq: func() *http.Request {
				req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
				req.Header.Set("Authorization", "Bearer sk-1234567890abcdef")
				return req
			},
			wantContain:    []string{"Authorization:", "***MASKED***"},
			wantNotContain: []string{"sk-1234567890abcdef"},
		},
		{
			name: "masks API key header",
			setupReq: func() *http.Request {
				req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
				req.Header.Set("X-Api-Key", "sk-ant-api-key-secret")
				return req
			},
			wantContain:    []string{"X-Api-Key:", "***MASKED***"},
			wantNotContain: []string{"sk-ant-api-key-secret"},
		},
		{
			name: "masks generic API key",
			setupReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "https://api.example.com/models", nil)
				req.Header.Set("Api-Key", "my-secret-key")
				return req
			},
			wantContain:    []string{"Api-Key:", "***MASKED***"},
			wantNotContain: []string{"my-secret-key"},
		},
		{
			name: "nil request",
			setupReq: func() *http.Request {
				return nil
			},
			wantContain:    []string{},
			wantNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewProviderError(ProviderTypeOpenAI, ErrCodeUnknown, "test")
			req := tt.setupReq()

			result := err.DumpRequest(req)

			if result != err {
				t.Error("DumpRequest should return the same error instance")
			}

			if req == nil && err.RequestDump != "" {
				t.Error("RequestDump should be empty for nil request")
				return
			}

			if req != nil {
				for _, want := range tt.wantContain {
					if !strings.Contains(err.RequestDump, want) {
						t.Errorf("RequestDump should contain %q, got: %s", want, err.RequestDump)
					}
				}

				for _, notWant := range tt.wantNotContain {
					if strings.Contains(err.RequestDump, notWant) {
						t.Errorf("RequestDump should not contain %q, got: %s", notWant, err.RequestDump)
					}
				}
			}
		})
	}
}

func TestProviderError_DumpResponse(t *testing.T) {
	tests := []struct {
		name        string
		setupResp   func() *http.Response
		wantContain []string
	}{
		{
			name: "dumps response correctly",
			setupResp: func() *http.Response {
				return &http.Response{
					StatusCode: 401,
					Status:     "401 Unauthorized",
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(bytes.NewBufferString(`{"error":"invalid_api_key"}`)),
				}
			},
			wantContain: []string{"401", "application/json"},
		},
		{
			name: "nil response",
			setupResp: func() *http.Response {
				return nil
			},
			wantContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewProviderError(ProviderTypeOpenAI, ErrCodeUnknown, "test")
			resp := tt.setupResp()

			result := err.DumpResponse(resp)

			if result != err {
				t.Error("DumpResponse should return the same error instance")
			}

			if resp == nil && err.ResponseDump != "" {
				t.Error("ResponseDump should be empty for nil response")
				return
			}

			if resp != nil {
				for _, want := range tt.wantContain {
					if !strings.Contains(err.ResponseDump, want) {
						t.Errorf("ResponseDump should contain %q, got: %s", want, err.ResponseDump)
					}
				}
			}
		})
	}
}

func TestMaskCredentials(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantMasked    []string
		wantNotMasked []string
	}{
		{
			name:          "bearer token",
			input:         "Authorization: Bearer sk-1234567890abcdef\r\n",
			wantMasked:    []string{"sk-1234567890abcdef"},
			wantNotMasked: []string{"Authorization:", "Bearer", "***MASKED***"},
		},
		{
			name:          "api key header",
			input:         "X-Api-Key: sk-ant-secret-key\r\n",
			wantMasked:    []string{"sk-ant-secret-key"},
			wantNotMasked: []string{"X-Api-Key:", "***MASKED***"},
		},
		{
			name:          "json api key",
			input:         `{"api_key": "secret123", "model": "gpt-4"}`,
			wantMasked:    []string{"secret123"},
			wantNotMasked: []string{`"api_key"`, "***MASKED***", "gpt-4"},
		},
		{
			name:          "json password",
			input:         `{"username": "user", "password": "pass123"}`,
			wantMasked:    []string{"pass123"},
			wantNotMasked: []string{`"password"`, "***MASKED***", "user"},
		},
		{
			name:          "multiple headers",
			input:         "Authorization: Bearer token1\r\nX-API-KEY: key2\r\nContent-Type: application/json\r\n",
			wantMasked:    []string{"token1", "key2"},
			wantNotMasked: []string{"Authorization:", "X-API-KEY:", "***MASKED***", "Content-Type:", "application/json"},
		},
		{
			name:          "case insensitive",
			input:         "authorization: bearer SECRET\r\napi-key: KEY\r\n",
			wantMasked:    []string{"SECRET", "KEY"},
			wantNotMasked: []string{"***MASKED***"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskCredentials(tt.input)

			for _, masked := range tt.wantMasked {
				if strings.Contains(result, masked) {
					t.Errorf("Result should not contain masked value %q, got: %s", masked, result)
				}
			}

			for _, notMasked := range tt.wantNotMasked {
				if !strings.Contains(result, notMasked) {
					t.Errorf("Result should contain %q, got: %s", notMasked, result)
				}
			}
		})
	}
}

func TestProviderError_GetDebugInfo(t *testing.T) {
	originalErr := errors.New("connection refused")
	err := NewAuthError(ProviderTypeOpenAI, "invalid API key").
		WithOperation("chat_completion").
		WithStatusCode(401).
		WithOriginalErr(originalErr).
		WithRequestID("req-123").
		WithRequestLatency(250 * time.Millisecond).
		WithCorrelationID("corr-456").
		WithRequestDump("POST /v1/chat HTTP/1.1\nAuthorization: Bearer ***MASKED***").
		WithResponseDump("HTTP/1.1 401 Unauthorized")

	debugInfo := err.GetDebugInfo()

	// Check that all expected fields are present
	expectedFields := []string{
		"[openai]",
		"invalid API key",
		"authentication",
		"chat_completion",
		"req-123",
		"corr-456",
		"250ms",
		"Request Dump",
		"Response Dump",
		"connection refused",
	}

	for _, field := range expectedFields {
		if !strings.Contains(debugInfo, field) {
			t.Errorf("GetDebugInfo should contain %q, got: %s", field, debugInfo)
		}
	}
}

func TestProviderError_FullChaining(t *testing.T) {
	// Test that all new methods can be chained together
	originalErr := errors.New("timeout")
	err := NewTimeoutError(ProviderTypeAnthropic, "request timeout").
		WithOperation("chat_completion").
		WithStatusCode(504).
		WithOriginalErr(originalErr).
		WithRequestID("req-789").
		WithRetryAfter(30).
		WithRequestDump("POST /v1/messages HTTP/1.1").
		WithResponseDump("HTTP/1.1 504 Gateway Timeout").
		WithRequestLatency(5 * time.Second).
		WithCorrelationID("corr-abc")

	// Verify all fields are set
	if err.Operation != "chat_completion" {
		t.Errorf("Operation = %v, want chat_completion", err.Operation)
	}
	if err.StatusCode != 504 {
		t.Errorf("StatusCode = %v, want 504", err.StatusCode)
	}
	if err.OriginalErr != originalErr {
		t.Errorf("OriginalErr = %v, want %v", err.OriginalErr, originalErr)
	}
	if err.RequestID != "req-789" {
		t.Errorf("RequestID = %v, want req-789", err.RequestID)
	}
	if err.RetryAfter != 30 {
		t.Errorf("RetryAfter = %v, want 30", err.RetryAfter)
	}
	if err.RequestDump != "POST /v1/messages HTTP/1.1" {
		t.Errorf("RequestDump = %v, want POST /v1/messages HTTP/1.1", err.RequestDump)
	}
	if err.ResponseDump != "HTTP/1.1 504 Gateway Timeout" {
		t.Errorf("ResponseDump = %v, want HTTP/1.1 504 Gateway Timeout", err.ResponseDump)
	}
	if err.RequestLatency != 5*time.Second {
		t.Errorf("RequestLatency = %v, want 5s", err.RequestLatency)
	}
	if err.CorrelationID != "corr-abc" {
		t.Errorf("CorrelationID = %v, want corr-abc", err.CorrelationID)
	}
}

func TestProviderError_BackwardCompatibility(t *testing.T) {
	// Test that existing code still works without new fields
	err := NewProviderError(ProviderTypeCerebras, ErrCodeInvalidRequest, "test").
		WithOperation("list_models").
		WithStatusCode(400)

	if err.Provider != ProviderTypeCerebras {
		t.Errorf("Provider = %v, want %v", err.Provider, ProviderTypeCerebras)
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Errorf("Code = %v, want %v", err.Code, ErrCodeInvalidRequest)
	}
	if err.Message != "test" {
		t.Errorf("Message = %v, want test", err.Message)
	}

	// New fields should have zero values
	if err.RequestDump != "" {
		t.Error("RequestDump should be empty by default")
	}
	if err.ResponseDump != "" {
		t.Error("ResponseDump should be empty by default")
	}
	if err.RequestLatency != 0 {
		t.Error("RequestLatency should be zero by default")
	}
	if err.CorrelationID != "" {
		t.Error("CorrelationID should be empty by default")
	}
}

func TestProviderError_DumpRequestWithBody(t *testing.T) {
	// Test request dump with body content
	body := `{"model": "gpt-4", "api_key": "secret123"}`
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer sk-secret")
	req.Header.Set("Content-Type", "application/json")

	err := NewProviderError(ProviderTypeOpenAI, ErrCodeUnknown, "test")
	_ = err.DumpRequest(req)

	// Check that sensitive data in both headers and body is masked
	if strings.Contains(err.RequestDump, "sk-secret") {
		t.Error("RequestDump should not contain unmasked bearer token")
	}
	if strings.Contains(err.RequestDump, "secret123") {
		t.Error("RequestDump should not contain unmasked api_key from body")
	}
	if !strings.Contains(err.RequestDump, "***MASKED***") {
		t.Error("RequestDump should contain masked values")
	}
	if !strings.Contains(err.RequestDump, "gpt-4") {
		t.Error("RequestDump should preserve non-sensitive data")
	}
}
