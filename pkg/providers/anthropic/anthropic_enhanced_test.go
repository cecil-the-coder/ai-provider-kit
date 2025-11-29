package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderDescription tests the Description method
func TestProviderDescription(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "test-key",
	})

	desc := provider.Description()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "Anthropic")
}

// TestGetAuthStatus tests GetAuthStatus method
func TestGetAuthStatus(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "test-key",
	})

	status := provider.GetAuthStatus()
	assert.NotNil(t, status)
	assert.Contains(t, status, "authenticated")
}

// TestLogout tests the Logout method
func TestLogout(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "test-key",
	})

	assert.True(t, provider.IsAuthenticated())

	err := provider.Logout(context.Background())
	assert.NoError(t, err)

	// After logout, should not be authenticated
	assert.False(t, provider.IsAuthenticated())
}

// TestInvokeServerTool tests InvokeServerTool (should return error for Anthropic)
func TestInvokeServerTool(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "test-key",
	})

	result, err := provider.InvokeServerTool(context.Background(), "test_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not yet implemented")
}

// TestRefreshAllOAuthTokens tests OAuth token refresh
func TestRefreshAllOAuthTokens(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "test-key",
	})

	err := provider.RefreshAllOAuthTokens(context.Background())
	// Should error without OAuth configured
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth manager")
}

// TestEnrichModels tests model enrichment with metadata
func TestEnrichModels(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "test-key",
	})

	models := []types.Model{
		{ID: "claude-sonnet-4-5-20250929", Name: "Claude Sonnet 4.5", Provider: types.ProviderTypeAnthropic},
		{ID: "unknown-model", Name: "Unknown", Provider: types.ProviderTypeAnthropic},
	}

	enriched := provider.enrichModels(models)

	assert.Len(t, enriched, 2)

	// Known model should have metadata
	assert.Equal(t, 200000, enriched[0].MaxTokens)
	assert.True(t, enriched[0].SupportsStreaming)
	assert.True(t, enriched[0].SupportsToolCalling)
	assert.NotEmpty(t, enriched[0].Description)

	// Unknown model should have defaults
	assert.Equal(t, 200000, enriched[1].MaxTokens)
	assert.Equal(t, "Anthropic Claude model", enriched[1].Description)
}

// TestConvertToAnthropicToolChoice tests tool choice conversion
func TestConvertToAnthropicToolChoice(t *testing.T) {
	tests := []struct {
		name       string
		toolChoice *types.ToolChoice
		expected   map[string]interface{}
	}{
		{
			name: "auto mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceAuto,
			},
			expected: map[string]interface{}{
				"type": "auto",
			},
		},
		{
			name: "required mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceRequired,
			},
			expected: map[string]interface{}{
				"type": "any",
			},
		},
		{
			name: "none mode",
			toolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceNone,
			},
			expected: map[string]interface{}{
				"type": "auto",
			},
		},
		{
			name: "specific mode",
			toolChoice: &types.ToolChoice{
				Mode:         types.ToolChoiceSpecific,
				FunctionName: "test_function",
			},
			expected: map[string]interface{}{
				"type": "tool",
				"name": "test_function",
			},
		},
		{
			name:       "nil tool choice",
			toolChoice: nil,
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToAnthropicToolChoice(tt.toolChoice)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				resultMap, ok := result.(map[string]interface{})
				if ok {
					for k, v := range tt.expected {
						assert.Equal(t, v, resultMap[k])
					}
				} else {
					resultStr, ok := result.(map[string]string)
					assert.True(t, ok)
					for k, v := range tt.expected {
						assert.Equal(t, v, resultStr[k])
					}
				}
			}
		})
	}
}

// TestChatCompletionWithHTTPError tests error handling for HTTP errors
func TestChatCompletionWithHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		response := map[string]interface{}{
			"type": "error",
			"error": map[string]interface{}{
				"type":    "authentication_error",
				"message": "Invalid API key",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		APIKey:  "invalid-key",
		BaseURL: server.URL,
	})

	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "claude-3-5-sonnet-20241022",
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	assert.Error(t, err)
	assert.Nil(t, stream)
	assert.Contains(t, err.Error(), "Invalid API key")
}

// TestChatCompletionWithNoContent tests empty content handling
func TestChatCompletionWithNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":      "msg_test",
			"type":    "message",
			"role":    "assistant",
			"model":   "claude-3-5-sonnet-20241022",
			"content": []interface{}{}, // Empty content
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 0,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "claude-3-5-sonnet-20241022",
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	assert.Error(t, err)
	assert.Nil(t, stream)
	assert.Contains(t, err.Error(), "no content")
}

// TestStreamingWithAPIKey tests streaming with API key
func TestStreamingWithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		events := []string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022"}}`,
			``,
			`event: content_block_start`,
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
			``,
			`event: content_block_stop`,
			`data: {"type":"content_block_stop","index":0}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}

		for _, event := range events {
			w.Write([]byte(event + "\n"))
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "claude-3-5-sonnet-20241022",
		Stream: true,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read chunks
	var chunks []types.ChatCompletionChunk
	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}
		chunks = append(chunks, chunk)
		if chunk.Done {
			break
		}
	}

	assert.NotEmpty(t, chunks)
	stream.Close()
}

// TestStreamingWithError tests streaming error handling
func TestStreamingWithError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		response := map[string]interface{}{
			"type": "error",
			"error": map[string]interface{}{
				"type":    "invalid_request_error",
				"message": "Invalid request",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "claude-3-5-sonnet-20241022",
		Stream: true,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	assert.Error(t, err)
	assert.Nil(t, stream)
}

// TestGetModelsWithOAuth tests model fetching with OAuth
func TestGetModelsWithOAuth(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type: types.ProviderTypeAnthropic,
		OAuthCredentials: []*types.OAuthCredentialSet{
			{
				ID:          "test-oauth",
				AccessToken: "sk-ant-oat-test-token",
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
		},
	})

	models, err := provider.GetModels(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, models)
	// Should use static fallback for OAuth tokens with sk-ant-oat prefix
}

// TestFetchModelsHelper tests the shared model fetching helper
func TestFetchModelsHelper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authentication header
		apiKey := r.Header.Get("x-api-key")
		if apiKey == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":           "claude-3-5-sonnet-20241022",
					"display_name": "Claude 3.5 Sonnet",
					"created_at":   "2024-10-22T00:00:00Z",
					"type":         "model",
				},
			},
			"has_more": false,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	modelsJSON, usage, err := provider.fetchModelsHelper(context.Background(), "api_key", "test-key")
	assert.NoError(t, err)
	assert.NotEmpty(t, modelsJSON)
	assert.NotNil(t, usage)

	// Verify JSON response
	var response AnthropicModelsResponse
	err = json.Unmarshal([]byte(modelsJSON), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Data, 1)
}

// TestFetchModelsWithOAuth tests OAuth model fetching
func TestFetchModelsWithOAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify OAuth header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":           "claude-3-5-sonnet-20241022",
					"display_name": "Claude 3.5 Sonnet",
					"created_at":   "2024-10-22T00:00:00Z",
					"type":         "model",
				},
			},
			"has_more": false,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		BaseURL: server.URL,
		OAuthCredentials: []*types.OAuthCredentialSet{
			{
				ID:          "test",
				AccessToken: "test-oauth-token",
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
		},
	})

	modelsJSON, usage, err := provider.fetchModelsWithOAuth(context.Background(), "test-oauth-token")
	assert.NoError(t, err)
	assert.NotEmpty(t, modelsJSON)
	assert.NotNil(t, usage)
}

// TestMakeAPICallWithOAuth tests OAuth API calls
func TestMakeAPICallWithOAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify OAuth header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response := map[string]interface{}{
			"id":    "msg_test",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-5-sonnet-20241022",
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Hello from OAuth!",
				},
			},
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		BaseURL: server.URL,
	})

	requestData := provider.prepareRequest(types.GenerateOptions{
		Prompt: "Hello",
		Model:  "claude-3-5-sonnet-20241022",
	}, "claude-3-5-sonnet-20241022")

	content, usage, err := provider.makeAPICallWithOAuth(context.Background(), requestData, "test-oauth-token")
	assert.NoError(t, err)
	assert.Contains(t, content, "Hello from OAuth!")
	assert.NotNil(t, usage)
	assert.Equal(t, 15, usage.TotalTokens)
}

// TestMakeStreamingAPICallWithOAuth tests OAuth streaming
func TestMakeStreamingAPICallWithOAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify OAuth header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		events := []string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022"}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"OAuth stream"}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}

		for _, event := range events {
			w.Write([]byte(event + "\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		BaseURL: server.URL,
	})

	requestData := provider.prepareRequest(types.GenerateOptions{
		Prompt: "Hello",
		Model:  "claude-3-5-sonnet-20241022",
		Stream: true,
	}, "claude-3-5-sonnet-20241022")

	stream, err := provider.makeStreamingAPICallWithOAuth(context.Background(), requestData, "test-oauth-token")
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	stream.Close()
}

// TestNoModelSpecifiedError tests error when no model is specified
func TestNoModelSpecifiedError(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type: types.ProviderTypeAnthropic,
		OAuthCredentials: []*types.OAuthCredentialSet{
			{
				ID:          "test",
				AccessToken: "non-anthropic-token", // No sk-ant-oat prefix
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
		},
	})

	// Should return empty string for non-Anthropic OAuth
	defaultModel := provider.GetDefaultModel()
	assert.Empty(t, defaultModel)

	// GenerateChatCompletion should error without model
	options := types.GenerateOptions{
		Prompt: "Hello",
		// No Model specified
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	assert.Error(t, err)
	assert.Nil(t, stream)
	assert.Contains(t, err.Error(), "no model specified")
}

// TestExecuteStreamWithAuthNoAuth tests streaming without authentication
func TestExecuteStreamWithAuthNoAuth(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type: types.ProviderTypeAnthropic,
		// No auth configured
	})

	// Clear both OAuth and API keys to test the "no auth" path
	provider.authHelper.OAuthManager = nil
	provider.authHelper.KeyManager = nil

	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "claude-3-5-sonnet-20241022",
		Stream: true,
	}

	stream, err := provider.executeStreamWithAuth(context.Background(), options, "claude-3-5-sonnet-20241022")
	assert.Error(t, err)
	assert.Nil(t, stream)
	assert.Contains(t, err.Error(), "no valid authentication")
}

// TestGetModelsWithAPIError tests model fetching with API error
func TestGetModelsWithAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:    types.ProviderTypeAnthropic,
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	// Should fall back to static list on error
	models, err := provider.GetModels(context.Background())
	assert.NoError(t, err) // Should not error due to fallback
	assert.NotEmpty(t, models)
}

// TestPrepareRequestWithMessages tests prepareRequest with Messages
func TestPrepareRequestWithMessages(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "test-key",
	})

	options := types.GenerateOptions{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []types.ChatMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Hello!",
			},
		},
	}

	request := provider.prepareRequest(options, "claude-3-5-sonnet-20241022")

	// Should have extracted system message
	assert.NotNil(t, request.System)
	// Should have only user message in Messages
	assert.Len(t, request.Messages, 1)
	assert.Equal(t, "user", request.Messages[0].Role)
}

// TestConvertToAnthropicContentWithTextAndToolCalls tests conversion of mixed content
func TestConvertToAnthropicContentWithTextAndToolCalls(t *testing.T) {
	msg := types.ChatMessage{
		Role:    "assistant",
		Content: "I'll help you with that.",
		ToolCalls: []types.ToolCall{
			{
				ID:   "tool_1",
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      "test_function",
					Arguments: `{"arg":"value"}`,
				},
			},
		},
	}

	content := convertToAnthropicContent(msg)

	blocks, ok := content.([]AnthropicContentBlock)
	require.True(t, ok)
	require.Len(t, blocks, 2) // Text + tool_use

	assert.Equal(t, "text", blocks[0].Type)
	assert.Equal(t, "I'll help you with that.", blocks[0].Text)

	assert.Equal(t, "tool_use", blocks[1].Type)
	assert.Equal(t, "tool_1", blocks[1].ID)
	assert.Equal(t, "test_function", blocks[1].Name)
}
