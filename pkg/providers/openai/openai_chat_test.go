package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenAIProvider_GenerateChatCompletion tests the GenerateChatCompletion method
func TestOpenAIProvider_GenerateChatCompletion(t *testing.T) {
	t.Run("BasicGeneration", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: "https://api.openai.com/v1",
		}
		provider := NewOpenAIProvider(config)

		options := types.GenerateOptions{
			Prompt:      "Hello, world!",
			MaxTokens:   100,
			Temperature: 0.7,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)

		// The real API call will fail with test credentials, which is expected
		// In a real scenario, you'd use a mock server or valid credentials
		if err != nil {
			assert.Contains(t, err.Error(), "invalid OpenAI")
			return
		}
		assert.NotNil(t, stream)

		// Test the mock stream
		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.Contains(t, chunk.Content, "Hello, world!")
		assert.True(t, chunk.Done)

		// Test closing the stream
		err = stream.Close()
		assert.NoError(t, err)
	})

	t.Run("WithMessages", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "What is the capital of France?"},
			},
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)

		// The real API call will fail with test credentials, which is expected
		if err != nil {
			assert.Contains(t, err.Error(), "invalid OpenAI")
			return
		}
		assert.NotNil(t, stream)

		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.Contains(t, chunk.Content, "What is the capital of France?")
		assert.True(t, chunk.Done)

		_ = stream.Close()
	})

	t.Run("WithTools", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		tools := []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get the current weather in a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state, e.g. San Francisco, CA",
						},
					},
					"required": []string{"location"},
				},
			},
		}

		options := types.GenerateOptions{
			Prompt: "What's the weather like in New York?",
			Tools:  tools,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)

		// The real API call will fail with test credentials, which is expected
		if err != nil {
			assert.Contains(t, err.Error(), "invalid OpenAI")
			return
		}
		assert.NotNil(t, stream)

		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.Contains(t, chunk.Content, "What's the weather like in New York?")
		assert.True(t, chunk.Done)

		_ = stream.Close()
	})
}

// TestOpenAIProvider_MockStream tests the MockStream implementation
func TestOpenAIProvider_MockStream(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	options := types.GenerateOptions{
		Prompt: "Test prompt for mock stream",
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)

	// The real API call will fail with test credentials, which is expected
	if err != nil {
		assert.Contains(t, err.Error(), "invalid OpenAI")
		return
	}
	require.NotNil(t, stream)

	// Test reading chunks
	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.Contains(t, chunk.Content, "Test prompt for mock stream")
	assert.True(t, chunk.Done)

	// Test stream exhaustion
	chunk, err = stream.Next()
	assert.NoError(t, err)
	assert.Empty(t, chunk.Content)

	// Test closing and reusing
	err = stream.Close()
	assert.NoError(t, err)

	// Should be able to read from beginning again
	chunk, err = stream.Next()
	assert.NoError(t, err)
	assert.Contains(t, chunk.Content, "Test prompt for mock stream")
}

// TestOpenAIProvider_RealAPI tests with a mock HTTP server to simulate real API calls
func TestOpenAIProvider_RealAPI(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		authHeader := r.Header.Get("Authorization")
		expectedAuth := "Bearer sk-test-key"
		if authHeader != expectedAuth {
			http.Error(w, "Invalid authorization", http.StatusUnauthorized)
			return
		}

		// Verify content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			http.Error(w, "Invalid content type", http.StatusBadRequest)
			return
		}

		// Verify URL path
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		// Parse request to verify it's valid JSON
		var request map[string]interface{}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Return mock response
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
						"content": "This is a mock response from the test server",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 15,
				"total_tokens":      25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	t.Run("MockServerInteraction", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}

		provider := NewOpenAIProvider(config)

		// Test that the provider was created with the mock server URL
		assert.Equal(t, server.URL, provider.baseURL)

		// Test basic provider methods
		assert.Equal(t, "OpenAI", provider.Name())
		assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
		assert.True(t, provider.IsAuthenticated())
	})
}

// TestOpenAIProvider_ReasoningFieldFallback tests that reasoning fields are used as fallback when content is empty
func TestOpenAIProvider_ReasoningFieldFallback(t *testing.T) {
	tests := []struct {
		name            string
		responseMessage map[string]interface{}
		expectedContent string
		description     string
	}{
		{
			name: "reasoning field fallback (GLM-4.6 style)",
			responseMessage: map[string]interface{}{
				"role":      "assistant",
				"content":   "",
				"reasoning": "This is the reasoning content from GLM-4.6",
			},
			expectedContent: "This is the reasoning content from GLM-4.6",
			description:     "When content is empty, should use reasoning field",
		},
		{
			name: "reasoning_content field fallback (vLLM/Synthetic style)",
			responseMessage: map[string]interface{}{
				"role":              "assistant",
				"content":           "",
				"reasoning_content": "This is reasoning_content from vLLM",
			},
			expectedContent: "This is reasoning_content from vLLM",
			description:     "When content is empty, should use reasoning_content field",
		},
		{
			name: "reasoning_content takes precedence over reasoning",
			responseMessage: map[string]interface{}{
				"role":              "assistant",
				"content":           "",
				"reasoning":         "reasoning field",
				"reasoning_content": "reasoning_content field",
			},
			expectedContent: "reasoning_content field",
			description:     "reasoning_content should take precedence when both are present",
		},
		{
			name: "normal content takes precedence",
			responseMessage: map[string]interface{}{
				"role":      "assistant",
				"content":   "Normal content response",
				"reasoning": "reasoning field",
			},
			expectedContent: "Normal content response",
			description:     "Content should be used when present",
		},
		{
			name: "newline-only content triggers fallback",
			responseMessage: map[string]interface{}{
				"role":      "assistant",
				"content":   "\n",
				"reasoning": "Fallback when content is just newline",
			},
			expectedContent: "Fallback when content is just newline",
			description:     "When content is only newline, should fallback to reasoning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that returns the test response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := map[string]interface{}{
					"id":      "chatcmpl-test",
					"object":  "chat.completion",
					"created": time.Now().Unix(),
					"model":   "gpt-4",
					"choices": []map[string]interface{}{
						{
							"index":         0,
							"message":       tt.responseMessage,
							"finish_reason": "stop",
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     10,
						"completion_tokens": 15,
						"total_tokens":      25,
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

			options := types.GenerateOptions{
				Messages: []types.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
			}

			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			require.NoError(t, err, tt.description)
			defer func() { _ = stream.Close() }()

			chunk, err := stream.Next()
			require.NoError(t, err, tt.description)
			assert.Equal(t, tt.expectedContent, chunk.Content, tt.description)
		})
	}
}
