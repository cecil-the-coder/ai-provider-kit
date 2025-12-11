package vertex

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
)

func TestNewVertexMiddleware(t *testing.T) {
	tests := []struct {
		name    string
		config  *VertexConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &VertexConfig{
				ProjectID:   "test-project",
				Region:      "us-east5",
				AuthType:    AuthTypeBearerToken,
				BearerToken: "test-token",
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing project",
			config: &VertexConfig{
				Region:      "us-east5",
				AuthType:    AuthTypeBearerToken,
				BearerToken: "test-token",
			},
			wantErr: true,
		},
		{
			name: "invalid config - missing region",
			config: &VertexConfig{
				ProjectID:   "test-project",
				AuthType:    AuthTypeBearerToken,
				BearerToken: "test-token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw, err := NewVertexMiddleware(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVertexMiddleware() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && mw == nil {
				t.Error("NewVertexMiddleware() returned nil middleware")
			}
		})
	}
}

func TestVertexMiddleware_ProcessRequest(t *testing.T) {
	config := &VertexConfig{
		ProjectID:   "test-project",
		Region:      "us-east5",
		AuthType:    AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	mw, err := NewVertexMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	tests := []struct {
		name           string
		requestPath    string
		requestBody    map[string]interface{}
		wantURLPattern string
		wantModelInCtx bool
		wantErr        bool
	}{
		{
			name:        "transform anthropic messages request",
			requestPath: "/v1/messages",
			requestBody: map[string]interface{}{
				"model":      "claude-3-5-sonnet-20241022",
				"max_tokens": 1024,
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Hello"},
				},
			},
			wantURLPattern: "aiplatform.googleapis.com/v1/projects/test-project/locations/us-east5/publishers/anthropic/models/claude-3-5-sonnet-v2@20241022:streamRawPredict",
			wantModelInCtx: true,
			wantErr:        false,
		},
		{
			name:        "non-messages endpoint passes through",
			requestPath: "/v1/models",
			requestBody: map[string]interface{}{},
			wantErr:     false,
		},
		{
			name:        "missing model field",
			requestPath: "/v1/messages",
			requestBody: map[string]interface{}{
				"max_tokens": 1024,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			bodyBytes, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "https://api.anthropic.com"+tt.requestPath, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			ctx := context.Background()
			newCtx, newReq, err := mw.ProcessRequest(ctx, req)

			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Check URL transformation
			if tt.wantURLPattern != "" {
				if !strings.Contains(newReq.URL.String(), tt.wantURLPattern) {
					t.Errorf("ProcessRequest() URL = %v, want pattern %v", newReq.URL.String(), tt.wantURLPattern)
				}
			}

			// Check context
			if tt.wantModelInCtx {
				modelValue := newCtx.Value(middleware.ContextKeyModel)
				if modelValue == nil {
					t.Error("ProcessRequest() did not set model in context")
				}
			}

			// Check auth header
			if tt.requestPath == "/v1/messages" {
				authHeader := newReq.Header.Get("Authorization")
				if !strings.HasPrefix(authHeader, "Bearer ") {
					t.Errorf("ProcessRequest() Authorization header = %v, want Bearer token", authHeader)
				}
			}
		})
	}
}

func TestVertexMiddleware_ProcessResponse(t *testing.T) {
	config := &VertexConfig{
		ProjectID:   "test-project",
		Region:      "us-east5",
		AuthType:    AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	mw, err := NewVertexMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	tests := []struct {
		name         string
		requestURL   string
		responseBody map[string]interface{}
		contextModel string
		checkModel   bool
	}{
		{
			name:       "transform vertex response",
			requestURL: "https://us-east5-aiplatform.googleapis.com/v1/projects/test-project/locations/us-east5/publishers/anthropic/models/claude-3-5-sonnet-v2@20241022:streamRawPredict",
			responseBody: map[string]interface{}{
				"id":      "msg-123",
				"type":    "message",
				"role":    "assistant",
				"content": []interface{}{map[string]interface{}{"type": "text", "text": "Hello!"}},
				"model":   "claude-3-5-sonnet-v2@20241022",
			},
			contextModel: "claude-3-5-sonnet-20241022",
			checkModel:   true,
		},
		{
			name:       "non-vertex response passes through",
			requestURL: "https://api.anthropic.com/v1/messages",
			responseBody: map[string]interface{}{
				"id": "msg-123",
			},
			checkModel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest("POST", tt.requestURL, nil)

			// Create response
			bodyBytes, _ := json.Marshal(tt.responseBody)
			resp := &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
				Header:     make(http.Header),
			}
			resp.Header.Set("Content-Type", "application/json")

			// Create context with model if specified
			ctx := context.Background()
			if tt.contextModel != "" {
				ctx = context.WithValue(ctx, middleware.ContextKeyModel, tt.contextModel)
			}

			newCtx, newResp, err := mw.ProcessResponse(ctx, req, resp)
			if err != nil {
				t.Errorf("ProcessResponse() error = %v", err)
				return
			}

			// Read and parse response
			responseBytes, err := io.ReadAll(newResp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			var responseData map[string]interface{}
			if err := json.Unmarshal(responseBytes, &responseData); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Check if model was restored from context
			if tt.checkModel && tt.contextModel != "" {
				if model, ok := responseData["model"].(string); ok {
					if model != tt.contextModel {
						t.Errorf("ProcessResponse() model = %v, want %v", model, tt.contextModel)
					}
				} else {
					t.Error("ProcessResponse() did not restore model from context")
				}
			}

			_ = newCtx // Avoid unused variable warning
		})
	}
}

func TestVertexMiddleware_StreamingResponse(t *testing.T) {
	config := &VertexConfig{
		ProjectID:   "test-project",
		Region:      "us-east5",
		AuthType:    AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	mw, err := NewVertexMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Create streaming response
	req := httptest.NewRequest("POST", "https://us-east5-aiplatform.googleapis.com/v1/projects/test-project/locations/us-east5/publishers/anthropic/models/claude-3-5-sonnet-v2@20241022:streamRawPredict", nil)

	streamingBody := `event: message_start
data: {"type":"message_start","message":{"id":"msg-123"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

event: message_stop
data: {"type":"message_stop"}
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(streamingBody))),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "text/event-stream")

	ctx := context.Background()
	_, newResp, err := mw.ProcessResponse(ctx, req, resp)
	if err != nil {
		t.Errorf("ProcessResponse() error = %v", err)
		return
	}

	// Read streaming response
	responseBytes, err := io.ReadAll(newResp.Body)
	if err != nil {
		t.Fatalf("Failed to read streaming response: %v", err)
	}

	// Should pass through unchanged
	if string(responseBytes) != streamingBody {
		t.Error("ProcessResponse() modified streaming response")
	}
}

func TestVertexMiddleware_ErrorResponse(t *testing.T) {
	config := &VertexConfig{
		ProjectID:   "test-project",
		Region:      "us-east5",
		AuthType:    AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	mw, err := NewVertexMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Create error response
	req := httptest.NewRequest("POST", "https://us-east5-aiplatform.googleapis.com/v1/projects/test-project/locations/us-east5/publishers/anthropic/models/claude-3-5-sonnet-v2@20241022:streamRawPredict", nil)

	errorBody := `{"error":{"type":"invalid_request_error","message":"Invalid request"}}`
	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(bytes.NewReader([]byte(errorBody))),
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")

	ctx := context.Background()
	_, newResp, err := mw.ProcessResponse(ctx, req, resp)
	if err != nil {
		t.Errorf("ProcessResponse() error = %v", err)
		return
	}

	// Read response
	responseBytes, err := io.ReadAll(newResp.Body)
	if err != nil {
		t.Fatalf("Failed to read error response: %v", err)
	}

	// Error response should pass through (JSON may be reformatted but content should be the same)
	var gotError, wantError map[string]interface{}
	if err := json.Unmarshal(responseBytes, &gotError); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if err := json.Unmarshal([]byte(errorBody), &wantError); err != nil {
		t.Fatalf("Failed to parse expected body: %v", err)
	}

	// Check that error structure is preserved
	if gotError["error"] == nil {
		t.Error("ProcessResponse() missing error field")
	}
}

func TestVertexMiddleware_ModelAvailability(t *testing.T) {
	config := &VertexConfig{
		ProjectID:   "test-project",
		Region:      "unknown-region",
		AuthType:    AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	mw, err := NewVertexMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Request with model not available in region
	requestBody := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	}

	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.Background()

	// Should succeed for unknown region (availability check returns true)
	_, _, err = mw.ProcessRequest(ctx, req)
	if err != nil {
		t.Errorf("ProcessRequest() unexpected error for unknown region: %v", err)
	}
}

func TestVertexMiddleware_CustomModelMapping(t *testing.T) {
	config := &VertexConfig{
		ProjectID:   "test-project",
		Region:      "us-east5",
		AuthType:    AuthTypeBearerToken,
		BearerToken: "test-token",
		ModelVersionMap: map[string]string{
			"custom-model": "custom-vertex-model@20241022",
		},
	}

	mw, err := NewVertexMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	requestBody := map[string]interface{}{
		"model":      "custom-model",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	}

	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.Background()
	_, newReq, err := mw.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("ProcessRequest() error = %v", err)
	}

	// Check that custom model mapping was applied
	if !strings.Contains(newReq.URL.String(), "custom-vertex-model@20241022") {
		t.Errorf("ProcessRequest() did not apply custom model mapping: %v", newReq.URL.String())
	}
}

func TestVertexMiddleware_GetConfig(t *testing.T) {
	config := &VertexConfig{
		ProjectID:   "test-project",
		Region:      "us-east5",
		AuthType:    AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	mw, err := NewVertexMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	gotConfig := mw.GetConfig()
	if gotConfig != config {
		t.Error("GetConfig() returned different config")
	}
}

func TestVertexMiddleware_GetAuthProvider(t *testing.T) {
	config := &VertexConfig{
		ProjectID:   "test-project",
		Region:      "us-east5",
		AuthType:    AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	mw, err := NewVertexMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	authProvider := mw.GetAuthProvider()
	if authProvider == nil {
		t.Error("GetAuthProvider() returned nil")
	}
}

func TestVertexMiddleware_ValidateAuth(t *testing.T) {
	config := &VertexConfig{
		ProjectID:   "test-project",
		Region:      "us-east5",
		AuthType:    AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	mw, err := NewVertexMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	ctx := context.Background()
	if err := mw.ValidateAuth(ctx); err != nil {
		t.Errorf("ValidateAuth() error = %v", err)
	}
}
