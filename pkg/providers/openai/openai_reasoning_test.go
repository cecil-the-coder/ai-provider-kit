package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReasoningModelRequest tests that reasoning_effort is properly included in requests
func TestReasoningModelRequest(t *testing.T) {
	tests := []struct {
		name            string
		reasoningEffort types.ReasoningEffort
		expectInRequest bool
	}{
		{
			name:            "Low reasoning effort",
			reasoningEffort: types.ReasoningEffortLow,
			expectInRequest: true,
		},
		{
			name:            "Medium reasoning effort",
			reasoningEffort: types.ReasoningEffortMedium,
			expectInRequest: true,
		},
		{
			name:            "High reasoning effort",
			reasoningEffort: types.ReasoningEffortHigh,
			expectInRequest: true,
		},
		{
			name:            "No reasoning effort",
			reasoningEffort: "",
			expectInRequest: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server to capture the request
			var capturedRequest OpenAIRequest
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Decode the request body
				err := json.NewDecoder(r.Body).Decode(&capturedRequest)
				require.NoError(t, err)

				// Return a mock response
				response := OpenAIResponse{
					ID:      "test-id",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   "o1-preview",
					Choices: []OpenAIChoice{
						{
							Index: 0,
							Message: OpenAIMessage{
								Role:    "assistant",
								Content: "Test response",
							},
							FinishReason: "stop",
						},
					},
					Usage: OpenAIUsage{
						PromptTokens:     10,
						CompletionTokens: 20,
						TotalTokens:      30,
						ReasoningTokens:  15,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			config := types.ProviderConfig{
				Type:    types.ProviderTypeOpenAI,
				APIKey:  "sk-test-key",
				BaseURL: server.URL,
			}
			provider := NewOpenAIProvider(config)

			options := types.GenerateOptions{
				Model:           "o1-preview",
				Prompt:          "Test prompt",
				ReasoningEffort: tt.reasoningEffort,
			}

			_, err := provider.GenerateChatCompletion(context.Background(), options)
			require.NoError(t, err)

			// Verify the request
			if tt.expectInRequest {
				assert.Equal(t, string(tt.reasoningEffort), capturedRequest.ReasoningEffort)
			} else {
				assert.Empty(t, capturedRequest.ReasoningEffort)
			}
		})
	}
}

// TestReasoningTokensTracking tests that reasoning_tokens are properly tracked in usage
func TestReasoningTokensTracking(t *testing.T) {
	// Create a test server that returns reasoning tokens
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := OpenAIResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "o1-preview",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "Test response with reasoning",
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     100,
				CompletionTokens: 200,
				TotalTokens:      300,
				ReasoningTokens:  150, // o1 models include reasoning tokens
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenAI,
		APIKey:  "sk-test-key",
		BaseURL: server.URL,
	}
	provider := NewOpenAIProvider(config)

	options := types.GenerateOptions{
		Model:           "o1-preview",
		Prompt:          "Solve this problem",
		ReasoningEffort: types.ReasoningEffortHigh,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read the response
	chunk, err := stream.Next()
	require.NoError(t, err)

	// Verify reasoning tokens are tracked
	assert.Equal(t, 100, chunk.Usage.PromptTokens)
	assert.Equal(t, 200, chunk.Usage.CompletionTokens)
	assert.Equal(t, 300, chunk.Usage.TotalTokens)
	assert.Equal(t, 150, chunk.Usage.ReasoningTokens)

	stream.Close()
}

// TestReasoningModels tests specific reasoning model configurations
func TestReasoningModels(t *testing.T) {
	models := []struct {
		name            string
		model           string
		reasoningEffort types.ReasoningEffort
	}{
		{
			name:            "o1-preview with high reasoning",
			model:           "o1-preview",
			reasoningEffort: types.ReasoningEffortHigh,
		},
		{
			name:            "o1-mini with medium reasoning",
			model:           "o1-mini",
			reasoningEffort: types.ReasoningEffortMedium,
		},
		{
			name:            "o3-mini with low reasoning",
			model:           "o3-mini",
			reasoningEffort: types.ReasoningEffortLow,
		},
	}

	for _, tt := range models {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request OpenAIRequest
				json.NewDecoder(r.Body).Decode(&request)

				// Verify model and reasoning effort
				assert.Equal(t, tt.model, request.Model)
				assert.Equal(t, string(tt.reasoningEffort), request.ReasoningEffort)

				response := OpenAIResponse{
					ID:      "test-id",
					Object:  "chat.completion",
					Created: 1234567890,
					Model:   tt.model,
					Choices: []OpenAIChoice{
						{
							Index: 0,
							Message: OpenAIMessage{
								Role:    "assistant",
								Content: "Reasoning response",
							},
							FinishReason: "stop",
						},
					},
					Usage: OpenAIUsage{
						PromptTokens:     50,
						CompletionTokens: 100,
						TotalTokens:      150,
						ReasoningTokens:  75,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			config := types.ProviderConfig{
				Type:    types.ProviderTypeOpenAI,
				APIKey:  "sk-test-key",
				BaseURL: server.URL,
			}
			provider := NewOpenAIProvider(config)

			options := types.GenerateOptions{
				Model:           tt.model,
				Prompt:          "Test reasoning prompt",
				ReasoningEffort: tt.reasoningEffort,
			}

			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			require.NoError(t, err)
			require.NotNil(t, stream)

			chunk, err := stream.Next()
			require.NoError(t, err)
			assert.Equal(t, 75, chunk.Usage.ReasoningTokens)

			stream.Close()
		})
	}
}

// TestReasoningTokensInStreaming tests reasoning tokens in streaming responses
// Note: This test uses a simplified streaming approach since OpenAI typically
// sends usage in a final chunk before [DONE]
func TestReasoningTokensInStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send streaming chunks with reasoning tokens
		// OpenAI sends usage in a separate chunk before finish_reason in practice
		chunks := []string{
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"o1-preview","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"o1-preview","choices":[{"index":0,"delta":{"content":"Thinking"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"o1-preview","choices":[{"index":0,"delta":{"content":" about"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"o1-preview","choices":[{"index":0,"delta":{"content":" this"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"o1-preview","choices":[{"index":0,"delta":{},"finish_reason":null}],"usage":{"prompt_tokens":100,"completion_tokens":200,"total_tokens":300,"reasoning_tokens":150}}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"o1-preview","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenAI,
		APIKey:  "sk-test-key",
		BaseURL: server.URL,
	}
	provider := NewOpenAIProvider(config)

	options := types.GenerateOptions{
		Model:           "o1-preview",
		Prompt:          "Stream test",
		Stream:          true,
		ReasoningEffort: types.ReasoningEffortHigh,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	var chunkWithUsage types.ChatCompletionChunk
	var fullContent string
	var foundUsage bool

	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}
		fullContent += chunk.Content
		// Keep the chunk that has usage information
		if chunk.Usage.TotalTokens > 0 {
			chunkWithUsage = chunk
			foundUsage = true
		}
	}

	// Verify content was streamed
	assert.Contains(t, fullContent, "Thinking about this")

	// Verify we found usage information in the stream
	assert.True(t, foundUsage, "Expected to find usage information in stream")

	// If we found usage, verify the reasoning tokens
	if foundUsage {
		assert.Equal(t, 100, chunkWithUsage.Usage.PromptTokens, "Prompt tokens mismatch")
		assert.Equal(t, 200, chunkWithUsage.Usage.CompletionTokens, "Completion tokens mismatch")
		assert.Equal(t, 300, chunkWithUsage.Usage.TotalTokens, "Total tokens mismatch")
		assert.Equal(t, 150, chunkWithUsage.Usage.ReasoningTokens, "Reasoning tokens mismatch")
	}

	stream.Close()
}

// TestBuildOpenAIRequestWithReasoningEffort tests the request building logic
func TestBuildOpenAIRequestWithReasoningEffort(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tests := []struct {
		name            string
		options         types.GenerateOptions
		expectedEffort  string
		shouldBePresent bool
	}{
		{
			name: "With reasoning effort low",
			options: types.GenerateOptions{
				Model:           "o1-preview",
				Prompt:          "Test",
				ReasoningEffort: types.ReasoningEffortLow,
			},
			expectedEffort:  "low",
			shouldBePresent: true,
		},
		{
			name: "With reasoning effort medium",
			options: types.GenerateOptions{
				Model:           "o1-mini",
				Prompt:          "Test",
				ReasoningEffort: types.ReasoningEffortMedium,
			},
			expectedEffort:  "medium",
			shouldBePresent: true,
		},
		{
			name: "With reasoning effort high",
			options: types.GenerateOptions{
				Model:           "o3-mini",
				Prompt:          "Test",
				ReasoningEffort: types.ReasoningEffortHigh,
			},
			expectedEffort:  "high",
			shouldBePresent: true,
		},
		{
			name: "Without reasoning effort",
			options: types.GenerateOptions{
				Model:  "gpt-4o",
				Prompt: "Test",
			},
			expectedEffort:  "",
			shouldBePresent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := provider.buildOpenAIRequest(tt.options)

			if tt.shouldBePresent {
				assert.Equal(t, tt.expectedEffort, request.ReasoningEffort)
			} else {
				assert.Empty(t, request.ReasoningEffort)
			}

			// Verify other fields are set correctly
			assert.NotEmpty(t, request.Messages)
			assert.Equal(t, tt.options.Model, request.Model)
		})
	}
}

// TestReasoningEffortConstants tests the reasoning effort constant values
func TestReasoningEffortConstants(t *testing.T) {
	assert.Equal(t, types.ReasoningEffort("low"), types.ReasoningEffortLow)
	assert.Equal(t, types.ReasoningEffort("medium"), types.ReasoningEffortMedium)
	assert.Equal(t, types.ReasoningEffort("high"), types.ReasoningEffortHigh)
}

// TestReasoningModelWithMessagesAPI tests reasoning models with message-based API
func TestReasoningModelWithMessagesAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request OpenAIRequest
		json.NewDecoder(r.Body).Decode(&request)

		// Verify messages format
		assert.NotEmpty(t, request.Messages)
		assert.Equal(t, "o1-preview", request.Model)
		assert.Equal(t, "high", request.ReasoningEffort)

		response := OpenAIResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "o1-preview",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "Detailed reasoning response",
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     200,
				CompletionTokens: 400,
				TotalTokens:      600,
				ReasoningTokens:  300,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenAI,
		APIKey:  "sk-test-key",
		BaseURL: server.URL,
	}
	provider := NewOpenAIProvider(config)

	options := types.GenerateOptions{
		Model: "o1-preview",
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Solve this complex problem"},
		},
		ReasoningEffort: types.ReasoningEffortHigh,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	chunk, err := stream.Next()
	require.NoError(t, err)

	assert.Equal(t, "Detailed reasoning response", chunk.Content)
	assert.Equal(t, 300, chunk.Usage.ReasoningTokens)
	assert.Equal(t, 600, chunk.Usage.TotalTokens)

	stream.Close()
}

// TestReasoningTokensZeroValue tests that missing reasoning_tokens default to zero
func TestReasoningTokensZeroValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Response without reasoning_tokens (e.g., non-reasoning model)
		response := OpenAIResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "Regular response",
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     50,
				CompletionTokens: 100,
				TotalTokens:      150,
				// No ReasoningTokens field
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenAI,
		APIKey:  "sk-test-key",
		BaseURL: server.URL,
	}
	provider := NewOpenAIProvider(config)

	options := types.GenerateOptions{
		Model:  "gpt-4o",
		Prompt: "Regular prompt",
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)

	chunk, err := stream.Next()
	require.NoError(t, err)

	// Verify reasoning tokens default to zero
	assert.Equal(t, 0, chunk.Usage.ReasoningTokens)
	assert.Equal(t, 150, chunk.Usage.TotalTokens)

	stream.Close()
}
