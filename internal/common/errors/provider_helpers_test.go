package errors

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewProviderErrorHelper(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeAnthropic)

	if helper.provider != types.ProviderTypeAnthropic {
		t.Errorf("Expected provider anthropic, got: %s", helper.provider)
	}

	if helper.snapshotConfig == nil {
		t.Error("Expected snapshot config to be initialized")
	}
}

func TestNewProviderErrorHelperWithConfig(t *testing.T) {
	config := &SnapshotConfig{
		MaxBodySize:    1024,
		IncludeHeaders: false,
		IncludeBody:    true,
		Masker:         DefaultCredentialMasker(),
	}

	helper := NewProviderErrorHelperWithConfig(types.ProviderTypeOpenAI, config)

	if helper.provider != types.ProviderTypeOpenAI {
		t.Errorf("Expected provider openai, got: %s", helper.provider)
	}

	if helper.snapshotConfig != config {
		t.Error("Expected custom config to be used")
	}
}

func TestProviderErrorHelper_WrapHTTPError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeAnthropic)

	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
	req.Header.Set("X-API-Key", "sk-ant-test")

	resp := &http.Response{
		StatusCode: 401,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error": "unauthorized"}`))),
	}

	richErr := helper.WrapHTTPError(req, resp, nil)

	if richErr == nil {
		t.Fatal("Expected non-nil rich error")
	}

	ctx := richErr.Context()
	if ctx.Provider != types.ProviderTypeAnthropic {
		t.Errorf("Expected Provider anthropic, got: %s", ctx.Provider)
	}

	if ctx.Request == nil {
		t.Error("Expected request snapshot to be set")
	}

	if ctx.Response == nil {
		t.Error("Expected response snapshot to be set")
	}

	if ctx.Response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got: %d", ctx.Response.StatusCode)
	}
}

func TestProviderErrorHelper_WrapHTTPErrorWithNetError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeGemini)

	req := httptest.NewRequest("GET", "https://api.example.com/test", nil)

	baseErr := errors.New("connection refused")
	richErr := helper.WrapHTTPError(req, nil, baseErr)

	if richErr == nil {
		t.Fatal("Expected non-nil rich error")
	}

	ctx := richErr.Context()
	if ctx.Provider != types.ProviderTypeGemini {
		t.Errorf("Expected Provider gemini, got: %s", ctx.Provider)
	}

	if ctx.Request == nil {
		t.Error("Expected request snapshot to be set")
	}

	// Should wrap the network error
	if !errors.Is(richErr, baseErr) {
		t.Error("Expected base error to be wrapped")
	}
}

func TestProviderErrorHelper_WrapRequestError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeOpenAI)

	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	baseErr := errors.New("failed to encode request body")

	richErr := helper.WrapRequestError("chat_completion", req, baseErr)

	if richErr == nil {
		t.Fatal("Expected non-nil rich error")
	}

	ctx := richErr.Context()
	if ctx.Provider != types.ProviderTypeOpenAI {
		t.Errorf("Expected Provider openai, got: %s", ctx.Provider)
	}

	if ctx.Operation != "chat_completion" {
		t.Errorf("Expected Operation chat_completion, got: %s", ctx.Operation)
	}

	if !errors.Is(richErr, baseErr) {
		t.Error("Expected base error to be wrapped")
	}
}

func TestProviderErrorHelper_WrapRequestErrorNil(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeOpenAI)

	richErr := helper.WrapRequestError("chat_completion", nil, nil)

	if richErr != nil {
		t.Error("Expected nil error for nil input")
	}
}

func TestProviderErrorHelper_WrapResponseError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeCopilot)

	req := httptest.NewRequest("POST", "https://api.githubcopilot.com/v1/chat", nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"data": "test"}`))),
	}
	baseErr := errors.New("failed to decode response")

	richErr := helper.WrapResponseError("parse_response", req, resp, baseErr)

	if richErr == nil {
		t.Fatal("Expected non-nil rich error")
	}

	ctx := richErr.Context()
	if ctx.Provider != types.ProviderTypeCopilot {
		t.Errorf("Expected Provider copilot, got: %s", ctx.Provider)
	}

	if ctx.Request == nil {
		t.Error("Expected request snapshot to be set")
	}

	if ctx.Response == nil {
		t.Error("Expected response snapshot to be set")
	}
}

func TestProviderErrorHelper_NewAuthError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeAnthropic)

	richErr := helper.NewAuthError("invalid API key")

	if richErr == nil {
		t.Fatal("Expected non-nil rich error")
	}

	if !errors.Is(richErr, ErrNotAuthenticated) {
		t.Error("Expected ErrNotAuthenticated to be wrapped")
	}

	ctx := richErr.Context()
	if ctx.Provider != types.ProviderTypeAnthropic {
		t.Errorf("Expected Provider anthropic, got: %s", ctx.Provider)
	}

	if ctx.Operation != "authenticate" {
		t.Errorf("Expected Operation authenticate, got: %s", ctx.Operation)
	}
}

func TestProviderErrorHelper_WrapAuthError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeGemini)

	baseErr := errors.New("OAuth token expired")
	richErr := helper.WrapAuthError("refresh_token", baseErr)

	if richErr == nil {
		t.Fatal("Expected non-nil rich error")
	}

	if !errors.Is(richErr, baseErr) {
		t.Error("Expected base error to be wrapped")
	}
}

func TestProviderErrorHelper_NewRateLimitError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeOpenAI)

	richErr := helper.NewRateLimitError("rate limit exceeded")

	if !errors.Is(richErr, ErrRateLimited) {
		t.Error("Expected ErrRateLimited to be wrapped")
	}

	ctx := richErr.Context()
	if ctx.Provider != types.ProviderTypeOpenAI {
		t.Errorf("Expected Provider openai, got: %s", ctx.Provider)
	}
}

func TestProviderErrorHelper_WrapRateLimitError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeAnthropic)

	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
	resp := &http.Response{
		StatusCode: 429,
		Header:     http.Header{"Retry-After": []string{"60"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error": "rate limit"}`))),
	}

	richErr := helper.WrapRateLimitError(req, resp)

	if !errors.Is(richErr, ErrRateLimited) {
		t.Error("Expected ErrRateLimited to be wrapped")
	}

	ctx := richErr.Context()
	if ctx.Request == nil {
		t.Error("Expected request snapshot to be set")
	}

	if ctx.Response == nil {
		t.Error("Expected response snapshot to be set")
	}
}

func TestProviderErrorHelper_NewInvalidRequestError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeGemini)

	richErr := helper.NewInvalidRequestError("model not found")

	if !errors.Is(richErr, ErrInvalidRequest) {
		t.Error("Expected ErrInvalidRequest to be wrapped")
	}
}

func TestProviderErrorHelper_NewNetworkError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeOpenAI)

	richErr := helper.NewNetworkError("connection refused")

	if !errors.Is(richErr, ErrNetworkError) {
		t.Error("Expected ErrNetworkError to be wrapped")
	}
}

func TestProviderErrorHelper_NewTimeoutError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeAnthropic)

	richErr := helper.NewTimeoutError("request timed out")

	if !errors.Is(richErr, ErrTimeout) {
		t.Error("Expected ErrTimeout to be wrapped")
	}
}

func TestProviderErrorHelper_NewServerError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeGemini)

	richErr := helper.NewServerError(500, "internal server error")

	if richErr == nil {
		t.Fatal("Expected non-nil rich error")
	}

	ctx := richErr.Context()
	if ctx.Provider != types.ProviderTypeGemini {
		t.Errorf("Expected Provider gemini, got: %s", ctx.Provider)
	}

	if !errors.Is(richErr, ErrServerError) {
		t.Error("Expected ErrServerError to be wrapped")
	}
}

func TestProviderErrorHelper_WrapServerError(t *testing.T) {
	helper := NewProviderErrorHelper(types.ProviderTypeCopilot)

	req := httptest.NewRequest("POST", "https://api.githubcopilot.com/v1/chat", nil)
	resp := &http.Response{
		StatusCode: 503,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error": "service unavailable"}`))),
	}

	richErr := helper.WrapServerError(req, resp)

	if richErr == nil {
		t.Fatal("Expected non-nil rich error")
	}

	ctx := richErr.Context()
	if ctx.Request == nil {
		t.Error("Expected request snapshot to be set")
	}

	if ctx.Response == nil {
		t.Error("Expected response snapshot to be set")
	}

	if ctx.Response.StatusCode != 503 {
		t.Errorf("Expected status code 503, got: %d", ctx.Response.StatusCode)
	}
}

func TestIsRetryableErrorRich(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "rate limit rich error",
			err:      NewRichError(ErrRateLimited),
			expected: true,
		},
		{
			name:     "timeout rich error",
			err:      NewRichError(ErrTimeout),
			expected: true,
		},
		{
			name:     "auth rich error",
			err:      NewRichError(ErrNotAuthenticated),
			expected: false,
		},
		{
			name:     "sentinel rate limit error",
			err:      ErrRateLimited,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableErrorRich(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsAuthenticationErrorRich(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "auth rich error",
			err:      NewRichError(ErrNotAuthenticated),
			expected: true,
		},
		{
			name:     "unauthorized rich error",
			err:      NewRichError(ErrUnauthorized),
			expected: true,
		},
		{
			name:     "rate limit rich error",
			err:      NewRichError(ErrRateLimited),
			expected: false,
		},
		{
			name:     "sentinel auth error",
			err:      ErrNotAuthenticated,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAuthenticationErrorRich(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsClientErrorRich(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "auth rich error",
			err:      NewRichError(ErrNotAuthenticated),
			expected: true,
		},
		{
			name:     "invalid request rich error",
			err:      NewRichError(ErrInvalidRequest),
			expected: true,
		},
		{
			name:     "rate limit rich error",
			err:      NewRichError(ErrRateLimited),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsClientErrorRich(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConvertProviderError(t *testing.T) {
	tests := []struct {
		name          string
		providerError *types.ProviderError
		checkSentinel error
	}{
		{
			name: "auth error",
			providerError: &types.ProviderError{
				Code:       types.ErrCodeAuthentication,
				Message:    "invalid API key",
				Provider:   types.ProviderTypeAnthropic,
				Operation:  "authenticate",
				RequestID:  "req-123",
				RetryAfter: 0,
			},
			checkSentinel: ErrNotAuthenticated,
		},
		{
			name: "rate limit error",
			providerError: &types.ProviderError{
				Code:       types.ErrCodeRateLimit,
				Message:    "rate limit exceeded",
				Provider:   types.ProviderTypeOpenAI,
				Operation:  "chat_completion",
				RetryAfter: 60,
			},
			checkSentinel: ErrRateLimited,
		},
		{
			name: "invalid request error",
			providerError: &types.ProviderError{
				Code:      types.ErrCodeInvalidRequest,
				Message:   "invalid model",
				Provider:  types.ProviderTypeGemini,
				Operation: "get_models",
			},
			checkSentinel: ErrInvalidRequest,
		},
		{
			name: "server error",
			providerError: &types.ProviderError{
				Code:       types.ErrCodeServerError,
				Message:    "internal server error",
				Provider:   types.ProviderTypeCopilot,
				StatusCode: 500,
			},
			checkSentinel: ErrServerError,
		},
		{
			name:          "nil error",
			providerError: nil,
			checkSentinel: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.providerError == nil {
				richErr := ConvertProviderError(nil)
				if richErr != nil {
					t.Error("Expected nil for nil input")
				}
				return
			}

			richErr := ConvertProviderError(tt.providerError)

			if richErr == nil {
				t.Fatal("Expected non-nil rich error")
			}

			if tt.checkSentinel != nil {
				if !errors.Is(richErr, tt.checkSentinel) {
					t.Errorf("Expected sentinel error %v to be wrapped", tt.checkSentinel)
				}
			}

			ctx := richErr.Context()
			if ctx.Provider != tt.providerError.Provider {
				t.Errorf("Expected Provider %s, got: %s", tt.providerError.Provider, ctx.Provider)
			}

			if ctx.Operation != tt.providerError.Operation {
				t.Errorf("Expected Operation %s, got: %s", tt.providerError.Operation, ctx.Operation)
			}

			if ctx.RequestID != tt.providerError.RequestID {
				t.Errorf("Expected RequestID %s, got: %s", tt.providerError.RequestID, ctx.RequestID)
			}

			if ctx.Response != nil && tt.providerError.StatusCode != 0 {
				if ctx.Response.StatusCode != tt.providerError.StatusCode {
					t.Errorf("Expected StatusCode %d, got: %d", tt.providerError.StatusCode, ctx.Response.StatusCode)
				}
			}
		})
	}
}

func TestExtractProviderError(t *testing.T) {
	tests := []struct {
		name              string
		richErr           *RichError
		expectedCode      types.ErrorCode
		expectedProvider  types.ProviderType
		expectedOperation string
	}{
		{
			name: "auth error",
			richErr: NewRichError(ErrNotAuthenticated).
				WithProvider(types.ProviderTypeAnthropic).
				WithOperation("authenticate"),
			expectedCode:      types.ErrCodeAuthentication,
			expectedProvider:  types.ProviderTypeAnthropic,
			expectedOperation: "authenticate",
		},
		{
			name: "rate limit error",
			richErr: NewRichError(ErrRateLimited).
				WithProvider(types.ProviderTypeOpenAI).
				WithOperation("chat_completion").
				WithTiming(time.Second),
			expectedCode:      types.ErrCodeRateLimit,
			expectedProvider:  types.ProviderTypeOpenAI,
			expectedOperation: "chat_completion",
		},
		{
			name: "server error with response",
			richErr: func() *RichError {
				resp := &http.Response{
					StatusCode: 500,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"error": "internal"}`))),
				}
				return NewRichError(ErrServerError).
					WithProvider(types.ProviderTypeGemini).
					WithResponseSnapshot(resp)
			}(),
			expectedCode:     types.ErrCodeServerError,
			expectedProvider: types.ProviderTypeGemini,
		},
		{
			name:              "nil error",
			richErr:           nil,
			expectedCode:      "",
			expectedProvider:  "",
			expectedOperation: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.richErr == nil {
				providerErr := ExtractProviderError(nil)
				if providerErr != nil {
					t.Error("Expected nil for nil input")
				}
				return
			}

			providerErr := ExtractProviderError(tt.richErr)

			if providerErr == nil {
				t.Fatal("Expected non-nil provider error")
			}

			if providerErr.Code != tt.expectedCode {
				t.Errorf("Expected Code %s, got: %s", tt.expectedCode, providerErr.Code)
			}

			if providerErr.Provider != tt.expectedProvider {
				t.Errorf("Expected Provider %s, got: %s", tt.expectedProvider, providerErr.Provider)
			}

			if tt.expectedOperation != "" && providerErr.Operation != tt.expectedOperation {
				t.Errorf("Expected Operation %s, got: %s", tt.expectedOperation, providerErr.Operation)
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	// Create a provider error
	originalErr := &types.ProviderError{
		Code:           types.ErrCodeAuthentication,
		Message:        "invalid API key",
		Provider:       types.ProviderTypeAnthropic,
		Operation:      "chat_completion",
		RequestID:      "req-abc-123",
		CorrelationID:  "corr-xyz-789",
		RequestLatency: time.Second,
		StatusCode:     401,
	}

	// Convert to RichError
	richErr := ConvertProviderError(originalErr)

	// Convert back to ProviderError
	convertedErr := ExtractProviderError(richErr)

	// Verify key fields are preserved
	if convertedErr.Code != originalErr.Code {
		t.Errorf("Code not preserved: expected %s, got %s", originalErr.Code, convertedErr.Code)
	}

	if convertedErr.Provider != originalErr.Provider {
		t.Errorf("Provider not preserved: expected %s, got %s", originalErr.Provider, convertedErr.Provider)
	}

	if convertedErr.Operation != originalErr.Operation {
		t.Errorf("Operation not preserved: expected %s, got %s", originalErr.Operation, convertedErr.Operation)
	}

	if convertedErr.RequestID != originalErr.RequestID {
		t.Errorf("RequestID not preserved: expected %s, got %s", originalErr.RequestID, convertedErr.RequestID)
	}

	if convertedErr.CorrelationID != originalErr.CorrelationID {
		t.Errorf("CorrelationID not preserved: expected %s, got %s", originalErr.CorrelationID, convertedErr.CorrelationID)
	}

	if convertedErr.RequestLatency != originalErr.RequestLatency {
		t.Errorf("RequestLatency not preserved: expected %s, got %s", originalErr.RequestLatency, convertedErr.RequestLatency)
	}
}
