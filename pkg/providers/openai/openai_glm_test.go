package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenAIProvider_GLMNoResponseRetry tests the GLM "No response requested" retry logic
func TestOpenAIProvider_GLMNoResponseRetry(t *testing.T) {
	t.Run("RetryOnNoResponseRequested", func(t *testing.T) {
		requestCount := 0
		var lastRequestBody map[string]interface{}
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestCount++
			currentRequest := requestCount
			mu.Unlock()

			// Parse request body to check thinking parameter
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &lastRequestBody)

			var response map[string]interface{}
			if currentRequest == 1 {
				// First request: return "No response requested."
				response = map[string]interface{}{
					"id":      "chatcmpl-test",
					"object":  "chat.completion",
					"created": time.Now().Unix(),
					"model":   "glm-4.6",
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":    "assistant",
								"content": "No response requested.",
							},
							"finish_reason": "end_turn",
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     100,
						"completion_tokens": 9,
						"total_tokens":      109,
					},
				}
			} else {
				// Second request (retry): return proper response
				response = map[string]interface{}{
					"id":      "chatcmpl-test-retry",
					"object":  "chat.completion",
					"created": time.Now().Unix(),
					"model":   "glm-4.6",
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":    "assistant",
								"content": "Here is the actual response after retry.",
							},
							"finish_reason": "stop",
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     100,
						"completion_tokens": 50,
						"total_tokens":      150,
					},
				}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			APIKey:       "sk-test-key",
			BaseURL:      server.URL,
			DefaultModel: "glm-4.6",
		}
		provider := NewOpenAIProvider(config)

		stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
			Model: "glm-4.6",
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		})
		require.NoError(t, err)

		// Read response
		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "Here is the actual response after retry.", chunk.Content)

		// Verify that retry happened
		mu.Lock()
		assert.Equal(t, 2, requestCount, "Expected 2 requests (initial + retry)")
		mu.Unlock()

		// Verify that retry request had thinking enabled
		assert.NotNil(t, lastRequestBody["thinking"], "Retry request should have thinking parameter")
		if thinking, ok := lastRequestBody["thinking"].(map[string]interface{}); ok {
			assert.Equal(t, "enabled", thinking["type"], "Thinking should be enabled in retry")
		}
	})

	t.Run("NoRetryForNonGLMModels", func(t *testing.T) {
		requestCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestCount++
			mu.Unlock()

			response := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "gpt-4",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "No response requested.",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     100,
					"completion_tokens": 9,
					"total_tokens":      109,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		})
		require.NoError(t, err)

		chunk, _ := stream.Next()
		// For non-GLM models, we should get the response as-is without retry
		assert.Equal(t, "No response requested.", chunk.Content)

		mu.Lock()
		assert.Equal(t, 1, requestCount, "Should not retry for non-GLM models")
		mu.Unlock()
	})

	t.Run("NoRetryIfAlreadyRetried", func(t *testing.T) {
		requestCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestCount++
			mu.Unlock()

			// Always return "No response requested" to test retry limit
			response := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "glm-4.6",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "No response requested.",
						},
						"finish_reason": "end_turn",
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     100,
					"completion_tokens": 9,
					"total_tokens":      109,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			APIKey:       "sk-test-key",
			BaseURL:      server.URL,
			DefaultModel: "glm-4.6",
		}
		provider := NewOpenAIProvider(config)

		stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
			Model: "glm-4.6",
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		})
		require.NoError(t, err)

		chunk, _ := stream.Next()
		// After retry fails too, we should get the response as-is
		assert.Equal(t, "No response requested.", chunk.Content)

		mu.Lock()
		// Should retry only once (2 total requests)
		assert.Equal(t, 2, requestCount, "Should only retry once")
		mu.Unlock()
	})
}

// TestIsGLMModel tests the GLM model detection helper
func TestIsGLMModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"glm-4.6", true},
		{"GLM-4.6", true},
		{"glm-4.5", true},
		{"synthetic:hf:zai-org/GLM-4.6", true},
		{"racing:glm-fast", true},
		{"gpt-4", false},
		{"claude-3", false},
		{"gemini-pro", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isGLMModel(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsNoResponseRequested tests the "No response requested" detection helper
func TestIsNoResponseRequested(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"No response requested.", true},
		{"  No response requested.  ", true},
		{"\nNo response requested.\n", true},
		{"No response requested", false},  // Missing period
		{"no response requested.", false}, // Lowercase
		{"Hello world", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			result := isNoResponseRequested(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}
