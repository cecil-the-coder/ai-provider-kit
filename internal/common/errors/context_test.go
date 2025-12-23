package errors

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewErrorContext(t *testing.T) {
	ctx := NewErrorContext()

	if ctx == nil {
		t.Fatal("NewErrorContext returned nil")
	}

	if ctx.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestErrorContext_Chaining(t *testing.T) {
	ctx := NewErrorContext().
		WithRequestID("req-123").
		WithCorrelationID("corr-456").
		WithProvider(types.ProviderTypeAnthropic).
		WithModel("claude-3-opus").
		WithOperation("chat_completion").
		WithDuration(100 * time.Millisecond)

	if ctx.RequestID != "req-123" {
		t.Errorf("Expected RequestID req-123, got %s", ctx.RequestID)
	}

	if ctx.CorrelationID != "corr-456" {
		t.Errorf("Expected CorrelationID corr-456, got %s", ctx.CorrelationID)
	}

	if ctx.Provider != types.ProviderTypeAnthropic {
		t.Errorf("Expected Provider anthropic, got %s", ctx.Provider)
	}

	if ctx.Model != "claude-3-opus" {
		t.Errorf("Expected Model claude-3-opus, got %s", ctx.Model)
	}

	if ctx.Operation != "chat_completion" {
		t.Errorf("Expected Operation chat_completion, got %s", ctx.Operation)
	}

	if ctx.Duration != 100*time.Millisecond {
		t.Errorf("Expected Duration 100ms, got %s", ctx.Duration)
	}
}

func TestNewRequestSnapshot(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		config         *SnapshotConfig
		validateResult func(*testing.T, *RequestSnapshot)
	}{
		{
			name: "nil request",
			setupRequest: func() *http.Request {
				return nil
			},
			validateResult: func(t *testing.T, snapshot *RequestSnapshot) {
				if snapshot != nil {
					t.Error("Expected nil snapshot for nil request")
				}
			},
		},
		{
			name: "basic GET request",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "https://api.example.com/v1/chat", nil)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer secret-key")
				return req
			},
			validateResult: func(t *testing.T, snapshot *RequestSnapshot) {
				if snapshot == nil {
					t.Fatal("Expected non-nil snapshot")
				}

				if snapshot.Method != "GET" {
					t.Errorf("Expected Method GET, got %s", snapshot.Method)
				}

				if !strings.Contains(snapshot.URL, "api.example.com") {
					t.Errorf("Expected URL to contain api.example.com, got %s", snapshot.URL)
				}

				if len(snapshot.Headers) == 0 {
					t.Error("Expected headers to be captured")
				}

				// Check that Authorization header is masked
				if authHeaders, ok := snapshot.Headers["Authorization"]; ok {
					if len(authHeaders) > 0 && !strings.Contains(authHeaders[0], "MASKED") {
						t.Errorf("Expected Authorization header to be masked, got %s", authHeaders[0])
					}
				}
			},
		},
		{
			name: "POST request with body",
			setupRequest: func() *http.Request {
				body := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
				req := httptest.NewRequest("POST", "https://api.example.com/v1/chat", bytes.NewBufferString(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			validateResult: func(t *testing.T, snapshot *RequestSnapshot) {
				if snapshot == nil {
					t.Fatal("Expected non-nil snapshot")
				}

				if snapshot.Body == "" {
					t.Error("Expected body to be captured")
				}

				if !strings.Contains(snapshot.Body, "gpt-4") {
					t.Errorf("Expected body to contain model name, got %s", snapshot.Body)
				}

				if snapshot.BodyTruncated {
					t.Error("Expected body not to be truncated for small request")
				}
			},
		},
		{
			name: "large body truncation",
			setupRequest: func() *http.Request {
				largeBody := strings.Repeat("x", 10000)
				req := httptest.NewRequest("POST", "https://api.example.com/v1/chat", bytes.NewBufferString(largeBody))
				return req
			},
			config: &SnapshotConfig{
				MaxBodySize:    1000,
				IncludeHeaders: true,
				IncludeBody:    true,
				Masker:         DefaultCredentialMasker(),
			},
			validateResult: func(t *testing.T, snapshot *RequestSnapshot) {
				if snapshot == nil {
					t.Fatal("Expected non-nil snapshot")
				}

				if len(snapshot.Body) > 1000 {
					t.Errorf("Expected body to be truncated to 1000 bytes, got %d", len(snapshot.Body))
				}

				if !snapshot.BodyTruncated {
					t.Error("Expected BodyTruncated to be true for large body")
				}
			},
		},
		{
			name: "body disabled in config",
			setupRequest: func() *http.Request {
				body := `{"test": "data"}`
				req := httptest.NewRequest("POST", "https://api.example.com/v1/chat", bytes.NewBufferString(body))
				return req
			},
			config: &SnapshotConfig{
				MaxBodySize:    4096,
				IncludeHeaders: true,
				IncludeBody:    false,
				Masker:         DefaultCredentialMasker(),
			},
			validateResult: func(t *testing.T, snapshot *RequestSnapshot) {
				if snapshot == nil {
					t.Fatal("Expected non-nil snapshot")
				}

				if snapshot.Body != "" {
					t.Error("Expected body to be empty when IncludeBody is false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			config := tt.config
			if config == nil {
				config = DefaultSnapshotConfig()
			}

			snapshot := NewRequestSnapshot(req, config)
			tt.validateResult(t, snapshot)

			// Verify request body can still be read after snapshot
			if req != nil && req.Body != nil {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Errorf("Failed to read request body after snapshot: %v", err)
				}
				if len(body) == 0 && tt.config == nil {
					// Only check if we expect a body
					if tt.name == "POST request with body" || tt.name == "large body truncation" {
						t.Error("Request body should be readable after snapshot")
					}
				}
			}
		})
	}
}

func TestNewResponseSnapshot(t *testing.T) {
	tests := []struct {
		name           string
		setupResponse  func() *http.Response
		config         *SnapshotConfig
		validateResult func(*testing.T, *ResponseSnapshot)
	}{
		{
			name: "nil response",
			setupResponse: func() *http.Response {
				return nil
			},
			validateResult: func(t *testing.T, snapshot *ResponseSnapshot) {
				if snapshot != nil {
					t.Error("Expected nil snapshot for nil response")
				}
			},
		},
		{
			name: "successful response",
			setupResponse: func() *http.Response {
				body := `{"id": "chatcmpl-123", "choices": [{"message": {"content": "Hello!"}}]}`
				resp := &http.Response{
					StatusCode: 200,
					Header:     http.Header{},
					Body:       io.NopCloser(bytes.NewBufferString(body)),
				}
				resp.Header.Set("Content-Type", "application/json")
				resp.Header.Set("X-Request-ID", "req-123")
				return resp
			},
			validateResult: func(t *testing.T, snapshot *ResponseSnapshot) {
				if snapshot == nil {
					t.Fatal("Expected non-nil snapshot")
				}

				if snapshot.StatusCode != 200 {
					t.Errorf("Expected StatusCode 200, got %d", snapshot.StatusCode)
				}

				if len(snapshot.Headers) == 0 {
					t.Error("Expected headers to be captured")
				}

				if snapshot.Body == "" {
					t.Error("Expected body to be captured")
				}

				if !strings.Contains(snapshot.Body, "chatcmpl-123") {
					t.Errorf("Expected body to contain response ID, got %s", snapshot.Body)
				}
			},
		},
		{
			name: "error response",
			setupResponse: func() *http.Response {
				body := `{"error": {"message": "Invalid API key", "type": "invalid_request_error"}}`
				resp := &http.Response{
					StatusCode: 401,
					Header:     http.Header{},
					Body:       io.NopCloser(bytes.NewBufferString(body)),
				}
				return resp
			},
			validateResult: func(t *testing.T, snapshot *ResponseSnapshot) {
				if snapshot == nil {
					t.Fatal("Expected non-nil snapshot")
				}

				if snapshot.StatusCode != 401 {
					t.Errorf("Expected StatusCode 401, got %d", snapshot.StatusCode)
				}

				if !strings.Contains(snapshot.Body, "Invalid API key") {
					t.Errorf("Expected body to contain error message, got %s", snapshot.Body)
				}
			},
		},
		{
			name: "large response truncation",
			setupResponse: func() *http.Response {
				largeBody := strings.Repeat("y", 10000)
				resp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(largeBody)),
				}
				return resp
			},
			config: &SnapshotConfig{
				MaxBodySize:    1000,
				IncludeHeaders: true,
				IncludeBody:    true,
				Masker:         DefaultCredentialMasker(),
			},
			validateResult: func(t *testing.T, snapshot *ResponseSnapshot) {
				if snapshot == nil {
					t.Fatal("Expected non-nil snapshot")
				}

				if len(snapshot.Body) > 1000 {
					t.Errorf("Expected body to be truncated to 1000 bytes, got %d", len(snapshot.Body))
				}

				if !snapshot.BodyTruncated {
					t.Error("Expected BodyTruncated to be true for large response")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.setupResponse()
			config := tt.config
			if config == nil {
				config = DefaultSnapshotConfig()
			}

			snapshot := NewResponseSnapshot(resp, config)
			tt.validateResult(t, snapshot)

			// Verify response body can still be read after snapshot
			if resp != nil && resp.Body != nil {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Errorf("Failed to read response body after snapshot: %v", err)
				}
				if len(body) == 0 && tt.config == nil {
					// Only check if we expect a body
					if tt.name == "successful response" || tt.name == "error response" {
						t.Error("Response body should be readable after snapshot")
					}
				}
			}
		})
	}
}

func TestDefaultSnapshotConfig(t *testing.T) {
	config := DefaultSnapshotConfig()

	if config == nil {
		t.Fatal("DefaultSnapshotConfig returned nil")
	}

	if config.MaxBodySize != 4096 {
		t.Errorf("Expected MaxBodySize 4096, got %d", config.MaxBodySize)
	}

	if !config.IncludeHeaders {
		t.Error("Expected IncludeHeaders to be true")
	}

	if !config.IncludeBody {
		t.Error("Expected IncludeBody to be true")
	}

	if config.Masker == nil {
		t.Error("Expected Masker to be set")
	}
}
