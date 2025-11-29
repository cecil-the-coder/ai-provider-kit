package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		retryable  bool
	}{
		{
			name: "429 too many requests",
			err: &APIError{StatusCode: http.StatusTooManyRequests},
			retryable: true,
		},
		{
			name: "500 internal server error",
			err: &APIError{StatusCode: http.StatusInternalServerError},
			retryable: true,
		},
		{
			name: "502 bad gateway",
			err: &APIError{StatusCode: http.StatusBadGateway},
			retryable: true,
		},
		{
			name: "503 service unavailable",
			err: &APIError{StatusCode: http.StatusServiceUnavailable},
			retryable: true,
		},
		{
			name: "504 gateway timeout",
			err: &APIError{StatusCode: http.StatusGatewayTimeout},
			retryable: true,
		},
		{
			name: "404 not found",
			err: &APIError{StatusCode: http.StatusNotFound},
			retryable: false,
		},
		{
			name: "400 bad request",
			err: &APIError{StatusCode: http.StatusBadRequest},
			retryable: false,
		},
		{
			name: "connection refused",
			err: errors.New("connection refused"),
			retryable: true,
		},
		{
			name: "timeout error",
			err: errors.New("timeout exceeded"),
			retryable: true,
		},
		{
			name: "network error",
			err: errors.New("network unreachable"),
			retryable: true,
		},
		{
			name: "temporary error",
			err: errors.New("temporary failure"),
			retryable: true,
		},
		{
			name: "non-retryable error",
			err: errors.New("invalid request"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("expected retryable %v, got %v", tt.retryable, result)
			}
		})
	}
}

func TestProcessResponse_Success(t *testing.T) {
	expectedBody := []byte("success response")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(expectedBody)),
	}

	body, err := ProcessResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(body, expectedBody) {
		t.Errorf("expected body %s, got %s", expectedBody, body)
	}
}

func TestProcessResponse_Error(t *testing.T) {
	errorBody := `{"error":{"type":"invalid_request","message":"Invalid parameter"}}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewReader([]byte(errorBody))),
	}

	_, err := ProcessResponse(resp)
	if err == nil {
		t.Fatal("expected error but got none")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status code 400, got %d", apiErr.StatusCode)
	}
}

func TestProcessJSONResponse_Success(t *testing.T) {
	type TestResponse struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}

	expectedResp := TestResponse{Message: "success", Code: 200}
	respBody, _ := json.Marshal(expectedResp)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	var target TestResponse
	err := ProcessJSONResponse(resp, &target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Message != expectedResp.Message {
		t.Errorf("expected message %s, got %s", expectedResp.Message, target.Message)
	}
	if target.Code != expectedResp.Code {
		t.Errorf("expected code %d, got %d", expectedResp.Code, target.Code)
	}
}

func TestProcessJSONResponse_InvalidJSON(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("invalid json"))),
	}

	var target map[string]interface{}
	err := ProcessJSONResponse(resp, &target)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse JSON response") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestProcessJSONResponse_HTTPError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(bytes.NewReader([]byte("server error"))),
	}

	var target map[string]interface{}
	err := ProcessJSONResponse(resp, &target)
	if err == nil {
		t.Error("expected error for HTTP error status")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status code 500, got %d", apiErr.StatusCode)
	}
}

func TestParseAPIError_WithStructuredError(t *testing.T) {
	errorBody := `{
		"error": {
			"type": "invalid_request_error",
			"message": "Missing required parameter",
			"code": "missing_param"
		}
	}`

	apiErr := ParseAPIError(http.StatusBadRequest, errorBody)

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status code 400, got %d", apiErr.StatusCode)
	}
	if apiErr.Type != "invalid_request_error" {
		t.Errorf("expected type invalid_request_error, got %s", apiErr.Type)
	}
	if apiErr.Message != "Missing required parameter" {
		t.Errorf("expected message 'Missing required parameter', got %s", apiErr.Message)
	}
	if apiErr.Code != "missing_param" {
		t.Errorf("expected code 'missing_param', got %s", apiErr.Code)
	}
	if apiErr.RawBody != errorBody {
		t.Error("expected raw body to be preserved")
	}
}

func TestParseAPIError_WithPlainText(t *testing.T) {
	errorBody := "Internal Server Error"

	apiErr := ParseAPIError(http.StatusInternalServerError, errorBody)

	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status code 500, got %d", apiErr.StatusCode)
	}
	if apiErr.Message != errorBody {
		t.Errorf("expected message %q, got %q", errorBody, apiErr.Message)
	}
}

func TestParseAPIError_WithEmptyBody(t *testing.T) {
	apiErr := ParseAPIError(http.StatusNotFound, "")

	if apiErr.Message != http.StatusText(http.StatusNotFound) {
		t.Errorf("expected message %q, got %q", http.StatusText(http.StatusNotFound), apiErr.Message)
	}
}

func TestRequestBuilder_Build(t *testing.T) {
	builder := NewRequestBuilder("POST", "http://example.com/api")

	req, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Method != "POST" {
		t.Errorf("expected method POST, got %s", req.Method)
	}
	if req.URL.String() != "http://example.com/api" {
		t.Errorf("expected URL http://example.com/api, got %s", req.URL.String())
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
	err = json.NewDecoder(req.Body).Decode(&decodedBody)
	if err != nil {
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

func TestStreamingResponse_ReadLine(t *testing.T) {
	body := "line1\nline2\nline3\n"
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte(body))),
	}

	sr := NewStreamingResponse(resp)
	defer sr.Close()

	line1, err := sr.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(line1) != "line1\n" {
		t.Errorf("expected 'line1\\n', got %q", string(line1))
	}

	line2, err := sr.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(line2) != "line2\n" {
		t.Errorf("expected 'line2\\n', got %q", string(line2))
	}
}

func TestStreamingResponse_ReadLine_EOF(t *testing.T) {
	body := "single line"
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte(body))),
	}

	sr := NewStreamingResponse(resp)
	defer sr.Close()

	line, err := sr.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(line) != body {
		t.Errorf("expected %q, got %q", body, string(line))
	}

	// Next read should return EOF
	_, err = sr.ReadLine()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestStreamingResponse_ReadChunk(t *testing.T) {
	body := "chunk1---chunk2---chunk3"
	delimiter := "---"
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte(body))),
	}

	sr := NewStreamingResponse(resp)
	defer sr.Close()

	chunk, err := sr.ReadChunk(delimiter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(chunk), "chunk1") {
		t.Errorf("expected chunk to contain 'chunk1', got %q", string(chunk))
	}
}

func TestStreamingResponse_Close(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte("data"))),
	}

	sr := NewStreamingResponse(resp)
	err := sr.Close()
	if err != nil {
		t.Fatalf("unexpected error on close: %v", err)
	}

	// Second close should not panic
	err = sr.Close()
	if err != nil {
		t.Fatalf("unexpected error on second close: %v", err)
	}
}

func TestStreamingResponse_Close_NilResponse(t *testing.T) {
	sr := &StreamingResponse{resp: nil}
	err := sr.Close()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadLine_EmptyInput(t *testing.T) {
	reader := bytes.NewReader([]byte(""))
	line, err := readLine(reader)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
	if len(line) != 0 {
		t.Errorf("expected empty line, got %q", string(line))
	}
}

func TestReadLine_MultipleLines(t *testing.T) {
	data := "first\nsecond\nthird"
	reader := bytes.NewReader([]byte(data))

	line1, err := readLine(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(line1) != "first\n" {
		t.Errorf("expected 'first\\n', got %q", string(line1))
	}

	line2, err := readLine(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(line2) != "second\n" {
		t.Errorf("expected 'second\\n', got %q", string(line2))
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
		name       string
		method     string
		token      string
		expectedKey string
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
		name         string
		providerType types.ProviderType
		expectedTimeout time.Duration
	}{
		{
			name:         "anthropic provider",
			providerType: types.ProviderTypeAnthropic,
			expectedTimeout: 60 * time.Second,
		},
		{
			name:         "openai provider",
			providerType: types.ProviderTypeOpenAI,
			expectedTimeout: 120 * time.Second,
		},
		{
			name:         "cerebras provider",
			providerType: types.ProviderTypeCerebras,
			expectedTimeout: 120 * time.Second,
		},
		{
			name:         "generic provider",
			providerType: types.ProviderTypeGemini,
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

func TestProcessResponse_ReadError(t *testing.T) {
	// Create a reader that fails
	failingReader := &failingReadCloser{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       failingReader,
	}

	_, err := ProcessResponse(resp)
	if err == nil {
		t.Error("expected error from failing reader")
	}
}

// Helper type for testing read errors
type failingReadCloser struct{}

func (f *failingReadCloser) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func (f *failingReadCloser) Close() error {
	return nil
}

func TestNewJSONRequest_InvalidURL(t *testing.T) {
	// Test with invalid URL that contains invalid characters
	_, err := NewJSONRequest("GET", "http://[::1]:namedport", nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestStreamingResponse_ReadChunk_EOF(t *testing.T) {
	body := "short data"
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader([]byte(body))),
	}

	sr := NewStreamingResponse(resp)
	defer sr.Close()

	chunk, err := sr.ReadChunk("NOTFOUND")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return all data even if delimiter not found
	if !strings.Contains(string(chunk), body) {
		t.Errorf("expected chunk to contain %q, got %q", body, string(chunk))
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
		json.NewDecoder(r.Body).Decode(&body)
		if body["message"] != "hello" {
			t.Errorf("expected message=hello, got %s", body["message"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"response": "ok"})
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
	defer resp.Body.Close()

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

func TestAPIError_Timestamp(t *testing.T) {
	before := time.Now()
	apiErr := ParseAPIError(500, "error")
	after := time.Now()

	if apiErr.Timestamp.Before(before) || apiErr.Timestamp.After(after) {
		t.Error("expected timestamp to be set to current time")
	}
}

func TestReadLine_SingleCharacter(t *testing.T) {
	reader := bytes.NewReader([]byte("a"))
	line, err := readLine(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(line) != "a" {
		t.Errorf("expected 'a', got %q", string(line))
	}
}

func TestReadLine_OnlyNewline(t *testing.T) {
	reader := bytes.NewReader([]byte("\n"))
	line, err := readLine(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(line) != "\n" {
		t.Errorf("expected newline, got %q", string(line))
	}
}
