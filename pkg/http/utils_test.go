package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewJSONRequest(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		url         string
		body        interface{}
		expectError bool
	}{
		{
			name:        "valid POST with body",
			method:      "POST",
			url:         "http://example.com/api",
			body:        map[string]string{"key": "value"},
			expectError: false,
		},
		{
			name:        "valid GET without body",
			method:      "GET",
			url:         "http://example.com/api",
			body:        nil,
			expectError: false,
		},
		{
			name:        "invalid body",
			method:      "POST",
			url:         "http://example.com/api",
			body:        make(chan int), // Cannot be marshaled to JSON
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewJSONRequest(tt.method, tt.url, tt.body)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.Method != tt.method {
				t.Errorf("expected method %s, got %s", tt.method, req.Method)
			}
			if req.URL.String() != tt.url {
				t.Errorf("expected URL %s, got %s", tt.url, req.URL.String())
			}
			if tt.body != nil {
				if req.Header.Get("Content-Type") != "application/json" {
					t.Error("expected Content-Type to be application/json")
				}
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	apiErr := &APIError{
		StatusCode: 404,
		Message:    "Not Found",
	}

	expectedMsg := "API error 404: Not Found"
	if apiErr.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, apiErr.Error())
	}
}

func TestRequestBuilder_WithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	builder := NewRequestBuilder("GET", "http://example.com")
	req, err := builder.WithContext(ctx).Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Context() != ctx {
		t.Error("expected request context to match provided context")
	}
}

func TestRequestBuilder_WithHeaders(t *testing.T) {
	headers := map[string]string{
		"X-Custom-1": "value1",
		"X-Custom-2": "value2",
	}

	builder := NewRequestBuilder("GET", "http://example.com")
	req, err := builder.WithHeaders(headers).Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for key, value := range headers {
		if req.Header.Get(key) != value {
			t.Errorf("expected header %s=%s, got %s", key, value, req.Header.Get(key))
		}
	}
}

func TestRequestBuilder_WithHeader(t *testing.T) {
	builder := NewRequestBuilder("GET", "http://example.com")
	req, err := builder.WithHeader("Authorization", "Bearer token123").Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Header.Get("Authorization") != "Bearer token123" {
		t.Errorf("expected Authorization header, got %s", req.Header.Get("Authorization"))
	}
}

func TestRequestBuilder_WithJSONBody(t *testing.T) {
	body := map[string]string{"key": "value"}

	builder := NewRequestBuilder("POST", "http://example.com")
	req, err := builder.WithJSONBody(body).Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Header.Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type to be application/json")
	}

	var decodedBody map[string]string
	if err = json.NewDecoder(req.Body).Decode(&decodedBody); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if decodedBody["key"] != "value" {
		t.Errorf("expected key=value, got key=%s", decodedBody["key"])
	}
}

func TestRequestBuilder_WithJSONBody_InvalidBody(t *testing.T) {
	builder := NewRequestBuilder("POST", "http://example.com")
	_, err := builder.WithJSONBody(make(chan int)).Build()
	if err == nil {
		t.Error("expected error for invalid JSON body")
	}
}

func TestRequestBuilder_ChainedCalls(t *testing.T) {
	ctx := context.Background()
	builder := NewRequestBuilder("POST", "http://example.com/api")

	req, err := builder.
		WithContext(ctx).
		WithHeader("X-API-Key", "secret").
		WithHeaders(map[string]string{"X-Custom": "value"}).
		WithJSONBody(map[string]string{"data": "test"}).
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Header.Get("X-API-Key") != "secret" {
		t.Error("expected X-API-Key header")
	}
	if req.Header.Get("X-Custom") != "value" {
		t.Error("expected X-Custom header")
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type header")
	}
}

func TestCommonHTTPHeaders(t *testing.T) {
	headers := CommonHTTPHeaders()

	expectedHeaders := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
		"User-Agent":   "ai-provider-kit/1.0",
	}

	for key, expected := range expectedHeaders {
		if headers[key] != expected {
			t.Errorf("expected %s=%s, got %s", key, expected, headers[key])
		}
	}
}

func TestAuthHeaders(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		token         string
		expectedKey   string
		expectedValue string
	}{
		{
			name:          "bearer auth",
			method:        "bearer",
			token:         "token123",
			expectedKey:   "Authorization",
			expectedValue: "Bearer token123",
		},
		{
			name:          "api-key auth",
			method:        "api-key",
			token:         "key123",
			expectedKey:   "x-api-key",
			expectedValue: "key123",
		},
		{
			name:          "openai auth",
			method:        "openai",
			token:         "sk-123",
			expectedKey:   "Authorization",
			expectedValue: "Bearer sk-123",
		},
		{
			name:          "anthropic auth",
			method:        "anthropic",
			token:         "ant-123",
			expectedKey:   "x-api-key",
			expectedValue: "ant-123",
		},
		{
			name:          "default auth",
			method:        "custom",
			token:         "custom-token",
			expectedKey:   "Authorization",
			expectedValue: "custom-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := AuthHeaders(tt.method, tt.token)
			if headers[tt.expectedKey] != tt.expectedValue {
				t.Errorf("expected %s=%s, got %s", tt.expectedKey, tt.expectedValue, headers[tt.expectedKey])
			}
		})
	}
}

func TestAuthHeaders_Anthropic_WithVersion(t *testing.T) {
	headers := AuthHeaders("anthropic", "token")
	if headers["anthropic-version"] != "2023-06-01" {
		t.Errorf("expected anthropic-version=2023-06-01, got %s", headers["anthropic-version"])
	}
}

func TestDefaultConfigProvider_GetHTTPConfig(t *testing.T) {
	provider := &DefaultConfigProvider{}

	tests := []struct {
		name            string
		providerType    types.ProviderType
		expectedTimeout time.Duration
	}{
		{
			name:            "anthropic provider",
			providerType:    types.ProviderTypeAnthropic,
			expectedTimeout: 60 * time.Second,
		},
		{
			name:            "openai provider",
			providerType:    types.ProviderTypeOpenAI,
			expectedTimeout: 120 * time.Second,
		},
		{
			name:            "cerebras provider",
			providerType:    types.ProviderTypeCerebras,
			expectedTimeout: 120 * time.Second,
		},
		{
			name:            "generic provider",
			providerType:    types.ProviderTypeGemini,
			expectedTimeout: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := provider.GetHTTPConfig(tt.providerType)

			if config.Timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, config.Timeout)
			}
			if config.MaxRetries != 3 {
				t.Errorf("expected max retries 3, got %d", config.MaxRetries)
			}
			if config.BaseRetryDelay != time.Second {
				t.Errorf("expected base retry delay 1s, got %v", config.BaseRetryDelay)
			}
			if !config.EnableMetrics {
				t.Error("expected metrics to be enabled")
			}
		})
	}
}

func TestDefaultConfigProvider_GetHTTPConfig_AnthropicHeaders(t *testing.T) {
	provider := &DefaultConfigProvider{}
	config := provider.GetHTTPConfig(types.ProviderTypeAnthropic)

	if config.Headers["anthropic-version"] != "2023-06-01" {
		t.Errorf("expected anthropic-version header, got %s", config.Headers["anthropic-version"])
	}
}

func TestDefaultClient(t *testing.T) {
	tests := []struct {
		name         string
		providerType types.ProviderType
	}{
		{"openai", types.ProviderTypeOpenAI},
		{"anthropic", types.ProviderTypeAnthropic},
		{"cerebras", types.ProviderTypeCerebras},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := DefaultClient(tt.providerType)
			if client == nil {
				t.Fatal("expected non-nil client")
			}
			if client.config.EnableMetrics != true {
				t.Error("expected metrics to be enabled")
			}
		})
	}
}

func TestNewJSONRequest_InvalidURL(t *testing.T) {
	// Test with invalid URL that contains invalid characters
	_, err := NewJSONRequest("GET", "http://[::1]:namedport", nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestRequestBuilder_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "value" {
			t.Error("expected X-Test header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type header")
		}

		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["message"] != "hello" {
			t.Errorf("expected message=hello, got %s", body["message"])
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
	}))
	defer server.Close()

	req, err := NewRequestBuilder("POST", server.URL).
		WithHeader("X-Test", "value").
		WithJSONBody(map[string]string{"message": "hello"}).
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestErrorResponse_Unmarshaling(t *testing.T) {
	jsonData := `{
		"error": {
			"type": "authentication_error",
			"message": "Invalid API key",
			"code": "invalid_api_key"
		},
		"details": {
			"key": "value"
		}
	}`

	var errResp ErrorResponse
	err := json.Unmarshal([]byte(jsonData), &errResp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if errResp.Error.Type != "authentication_error" {
		t.Errorf("expected type authentication_error, got %s", errResp.Error.Type)
	}
	if errResp.Error.Message != "Invalid API key" {
		t.Errorf("expected message 'Invalid API key', got %s", errResp.Error.Message)
	}
	if errResp.Error.Code != "invalid_api_key" {
		t.Errorf("expected code invalid_api_key, got %s", errResp.Error.Code)
	}
}
