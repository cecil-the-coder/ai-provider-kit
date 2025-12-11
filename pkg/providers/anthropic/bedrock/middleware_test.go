package bedrock

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	mw "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBedrockMiddleware(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	middleware, err := NewBedrockMiddleware(config)
	require.NoError(t, err)
	assert.NotNil(t, middleware)
	assert.NotNil(t, middleware.signer)
	assert.NotNil(t, middleware.modelMapper)
}

func TestNewBedrockMiddleware_InvalidConfig(t *testing.T) {
	config := &BedrockConfig{
		// Missing required fields
		Region: "us-east-1",
	}

	_, err := NewBedrockMiddleware(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bedrock config")
}

func TestNewBedrockMiddleware_CustomModelMappings(t *testing.T) {
	customMappings := map[string]string{
		"custom-model": "anthropic.custom-model-v1:0",
	}

	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		ModelMappings:   customMappings,
	}

	middleware, err := NewBedrockMiddleware(config)
	require.NoError(t, err)

	// Verify custom mapping is loaded
	bedrockID, found := middleware.modelMapper.ToBedrockModelID("custom-model")
	assert.True(t, found)
	assert.Equal(t, "anthropic.custom-model-v1:0", bedrockID)
}

func TestBedrockMiddleware_ProcessRequest(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	middleware, err := NewBedrockMiddleware(config)
	require.NoError(t, err)

	// Create an Anthropic messages request
	requestBody := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-key")
	req.Header.Set("anthropic-version", "2023-06-01")

	ctx := context.Background()
	newCtx, newReq, err := middleware.ProcessRequest(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, newReq)

	// Verify URL was transformed to Bedrock
	assert.Equal(t, "bedrock-runtime.us-east-1.amazonaws.com", newReq.URL.Host)
	assert.Contains(t, newReq.URL.Path, "/model/anthropic.claude-3-opus-20240229-v1:0/invoke")

	// Verify Anthropic headers were removed
	assert.Empty(t, newReq.Header.Get("x-api-key"))
	assert.Empty(t, newReq.Header.Get("anthropic-version"))

	// Verify AWS signature headers were added
	assert.NotEmpty(t, newReq.Header.Get("Authorization"))
	assert.NotEmpty(t, newReq.Header.Get("X-Amz-Date"))
	assert.NotEmpty(t, newReq.Header.Get("X-Amz-Content-Sha256"))

	// Verify context was updated
	assert.Equal(t, "bedrock", newCtx.Value(mw.ContextKeyProvider))
}

func TestBedrockMiddleware_ProcessRequest_StreamingRequest(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	middleware, err := NewBedrockMiddleware(config)
	require.NoError(t, err)

	requestBody := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
		"stream": true,
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages?stream=true", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.Background()
	_, newReq, err := middleware.ProcessRequest(ctx, req)
	require.NoError(t, err)

	// Verify streaming endpoint
	assert.Contains(t, newReq.URL.Path, "/invoke-with-response-stream")
}

func TestBedrockMiddleware_ProcessRequest_NonMessagesRequest(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	middleware, err := NewBedrockMiddleware(config)
	require.NoError(t, err)

	// Non-messages request (e.g., models list)
	req := httptest.NewRequest("GET", "https://api.anthropic.com/v1/models", nil)

	ctx := context.Background()
	_, newReq, err := middleware.ProcessRequest(ctx, req)
	require.NoError(t, err)

	// Should pass through unchanged
	assert.Equal(t, req, newReq)
	assert.Equal(t, "api.anthropic.com", newReq.URL.Host)
}

func TestBedrockMiddleware_ProcessResponse(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	middleware, err := NewBedrockMiddleware(config)
	require.NoError(t, err)

	// Create a Bedrock response
	responseBody := map[string]interface{}{
		"id":   "msg_123",
		"type": "message",
		"role": "assistant",
		"content": []map[string]interface{}{
			{"type": "text", "text": "Hello!"},
		},
		"model": "anthropic.claude-3-opus-20240229-v1:0",
		"usage": map[string]int{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}
	bodyBytes, err := json.Marshal(responseBody)
	require.NoError(t, err)

	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
	}
	resp.Header.Set("Content-Type", "application/json")

	req := httptest.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/test/invoke", nil)

	// Set context to indicate this is a Bedrock response
	ctx := context.WithValue(context.Background(), mw.ContextKeyProvider, "bedrock")

	_, newResp, err := middleware.ProcessResponse(ctx, req, resp)
	require.NoError(t, err)
	assert.NotNil(t, newResp)

	// Read and verify response body
	responseBytes, err := io.ReadAll(newResp.Body)
	require.NoError(t, err)

	var transformedResp map[string]interface{}
	err = json.Unmarshal(responseBytes, &transformedResp)
	require.NoError(t, err)

	// Verify response structure is preserved
	assert.Equal(t, "msg_123", transformedResp["id"])
	assert.Equal(t, "message", transformedResp["type"])
}

func TestBedrockMiddleware_ProcessResponse_NonBedrockResponse(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	middleware, err := NewBedrockMiddleware(config)
	require.NoError(t, err)

	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte("{}"))),
	}

	req := httptest.NewRequest("GET", "https://api.anthropic.com/v1/models", nil)

	// Context without bedrock provider
	ctx := context.Background()

	_, newResp, err := middleware.ProcessResponse(ctx, req, resp)
	require.NoError(t, err)

	// Should pass through unchanged
	assert.Equal(t, resp, newResp)
}

func TestBedrockMiddleware_isAnthropicMessagesRequest(t *testing.T) {
	middleware := &BedrockMiddleware{}

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "messages endpoint",
			url:      "https://api.anthropic.com/v1/messages",
			expected: true,
		},
		{
			name:     "models endpoint",
			url:      "https://api.anthropic.com/v1/models",
			expected: false,
		},
		{
			name:     "other endpoint",
			url:      "https://api.anthropic.com/v1/other",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			result := middleware.isAnthropicMessagesRequest(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBedrockMiddleware_isStreamingResponse(t *testing.T) {
	middleware := &BedrockMiddleware{}

	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{
			name:        "SSE content type",
			contentType: "text/event-stream",
			expected:    true,
		},
		{
			name:        "AWS event stream",
			contentType: "application/vnd.amazon.eventstream",
			expected:    true,
		},
		{
			name:        "JSON content type",
			contentType: "application/json",
			expected:    false,
		},
		{
			name:        "no content type",
			contentType: "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}
			if tt.contentType != "" {
				resp.Header.Set("Content-Type", tt.contentType)
			}
			result := middleware.isStreamingResponse(resp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBedrockMiddleware_updateRequestURL(t *testing.T) {
	config := &BedrockConfig{
		Region: "us-west-2",
	}
	middleware := &BedrockMiddleware{config: config}

	tests := []struct {
		name         string
		originalURL  string
		modelID      string
		expectedHost string
		expectedPath string
	}{
		{
			name:         "non-streaming request",
			originalURL:  "https://api.anthropic.com/v1/messages",
			modelID:      "anthropic.claude-3-opus-20240229-v1:0",
			expectedHost: "bedrock-runtime.us-west-2.amazonaws.com",
			expectedPath: "/model/anthropic.claude-3-opus-20240229-v1:0/invoke",
		},
		{
			name:         "streaming request",
			originalURL:  "https://api.anthropic.com/v1/messages?stream=true",
			modelID:      "anthropic.claude-3-haiku-20240307-v1:0",
			expectedHost: "bedrock-runtime.us-west-2.amazonaws.com",
			expectedPath: "/model/anthropic.claude-3-haiku-20240307-v1:0/invoke-with-response-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.originalURL, nil)
			middleware.updateRequestURL(req, tt.modelID)

			assert.Equal(t, tt.expectedHost, req.URL.Host)
			assert.Equal(t, tt.expectedPath, req.URL.Path)
			assert.Equal(t, "https", req.URL.Scheme)
		})
	}
}

func TestBedrockMiddleware_transformHeaders(t *testing.T) {
	middleware := &BedrockMiddleware{}

	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
	req.Header.Set("x-api-key", "test-key")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "test-beta")
	req.Header.Set("User-Agent", "test-agent")

	middleware.transformHeaders(req)

	// Verify Anthropic headers removed
	assert.Empty(t, req.Header.Get("x-api-key"))
	assert.Empty(t, req.Header.Get("anthropic-version"))
	assert.Empty(t, req.Header.Get("anthropic-beta"))

	// Verify standard headers preserved
	assert.Equal(t, "test-agent", req.Header.Get("User-Agent"))

	// Verify required headers added
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
}

func TestBedrockMiddleware_transformRequestToBedrock(t *testing.T) {
	middleware := &BedrockMiddleware{}

	anthropicReq := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
		"anthropic_version": "2023-06-01",
	}

	bedrockReq := middleware.transformRequestToBedrock(anthropicReq)

	// Model should not be in body (it's in URL)
	assert.NotContains(t, bedrockReq, "model")

	// anthropic_version should be removed
	assert.NotContains(t, bedrockReq, "anthropic_version")

	// Other fields should be preserved
	assert.Equal(t, 100, bedrockReq["max_tokens"])
	assert.NotNil(t, bedrockReq["messages"])
}

func TestBedrockMiddleware_transformRequestToBedrock_DefaultMaxTokens(t *testing.T) {
	middleware := &BedrockMiddleware{}

	anthropicReq := map[string]interface{}{
		"model": "claude-3-opus-20240229",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}

	bedrockReq := middleware.transformRequestToBedrock(anthropicReq)

	// Should add default max_tokens
	assert.Equal(t, 4096, bedrockReq["max_tokens"])
}

func TestBedrockMiddleware_transformResponseToAnthropic(t *testing.T) {
	middleware := &BedrockMiddleware{}

	bedrockResp := map[string]interface{}{
		"id":   "msg_123",
		"type": "message",
		"role": "assistant",
		"content": []map[string]interface{}{
			{"type": "text", "text": "Hello!"},
		},
		"usage": map[string]interface{}{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}

	anthropicResp, err := middleware.transformResponseToAnthropic(bedrockResp)
	require.NoError(t, err)

	// Currently pass-through, so should be identical
	assert.Equal(t, bedrockResp, anthropicResp)
}

func TestBedrockMiddleware_EndToEnd(t *testing.T) {
	config := &BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	middleware, err := NewBedrockMiddleware(config)
	require.NoError(t, err)

	// Create request
	requestBody := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": "Test"},
		},
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	// Process request
	ctx := context.Background()
	ctx, transformedReq, err := middleware.ProcessRequest(ctx, req)
	require.NoError(t, err)

	// Verify request transformation
	assert.Equal(t, "bedrock-runtime.us-east-1.amazonaws.com", transformedReq.URL.Host)
	assert.NotEmpty(t, transformedReq.Header.Get("Authorization"))

	// Create mock response
	responseBody := map[string]interface{}{
		"id":   "msg_123",
		"type": "message",
		"content": []map[string]interface{}{
			{"type": "text", "text": "Response"},
		},
	}
	respBytes, err := json.Marshal(responseBody)
	require.NoError(t, err)

	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(respBytes)),
	}
	resp.Header.Set("Content-Type", "application/json")

	// Process response
	_, transformedResp, err := middleware.ProcessResponse(ctx, transformedReq, resp)
	require.NoError(t, err)

	// Verify response
	assert.NotNil(t, transformedResp)
	assert.Equal(t, 200, transformedResp.StatusCode)
}
