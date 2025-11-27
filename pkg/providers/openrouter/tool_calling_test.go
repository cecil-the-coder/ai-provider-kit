package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolCalling_RequestConversion tests that tools are properly converted in requests
func TestToolCalling_RequestConversion(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "sk-or-test-key",
	}
	provider := NewOpenRouterProvider(config)

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
					"unit": map[string]interface{}{
						"type": "string",
						"enum": []string{"celsius", "fahrenheit"},
					},
				},
				"required": []string{"location"},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather like?",
		Tools:  tools,
	}

	request, err := provider.prepareRequest(options)
	require.NoError(t, err)

	// Verify tools are included in request
	assert.NotNil(t, request.Tools)
	assert.Len(t, request.Tools, 1)
	assert.Equal(t, "function", request.Tools[0].Type)
	assert.Equal(t, "get_weather", request.Tools[0].Function.Name)
	assert.Equal(t, "Get the current weather in a location", request.Tools[0].Function.Description)
	assert.NotNil(t, request.Tools[0].Function.Parameters)
}

// TestToolCalling_ResponseParsing tests that tool calls are parsed from responses
func TestToolCalling_ResponseParsing(t *testing.T) {
	// Create a mock HTTP server that returns tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limit check requests
		if r.URL.Path == "/v1/key" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"is_free_tier": false,
			})
			return
		}

		// Return response with tool calls
		response := map[string]interface{}{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "qwen/qwen3-coder",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_abc123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_weather",
									"arguments": `{"location":"San Francisco, CA","unit":"fahrenheit"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     25,
				"completion_tokens": 20,
				"total_tokens":      45,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenRouter,
		APIKey:  "sk-or-test-key",
		BaseURL: server.URL,
	}
	provider := NewOpenRouterProvider(config)

	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get the current weather",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather in SF?",
		Tools:  tools,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read the response
	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.True(t, chunk.Done)

	// Verify tool calls are present
	require.Len(t, chunk.Choices, 1)
	require.Len(t, chunk.Choices[0].Message.ToolCalls, 1)

	toolCall := chunk.Choices[0].Message.ToolCalls[0]
	assert.Equal(t, "call_abc123", toolCall.ID)
	assert.Equal(t, "function", toolCall.Type)
	assert.Equal(t, "get_weather", toolCall.Function.Name)
	assert.Contains(t, toolCall.Function.Arguments, "San Francisco")

	_ = stream.Close()
}

// TestToolCalling_StreamingToolCalls tests streaming tool calls
func TestToolCalling_StreamingToolCalls(t *testing.T) {
	// Create a mock HTTP server that streams tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limit check requests
		if r.URL.Path == "/v1/key" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"is_free_tier": false,
			})
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send streaming chunks
		chunks := []string{
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"qwen/qwen3-coder","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_xyz","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"Boston\"}"}}]},"finish_reason":""}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"qwen/qwen3-coder","choices":[{"index":0,"message":{"content":""},"finish_reason":"tool_calls"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "%s\n\n", chunk)
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenRouter,
		APIKey:  "sk-or-test-key",
		BaseURL: server.URL,
	}
	provider := NewOpenRouterProvider(config)

	options := types.GenerateOptions{
		Prompt: "What's the weather?",
		Stream: true,
		Tools: []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read all chunks
	var chunks []types.ChatCompletionChunk
	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}
		if chunk.Done {
			chunks = append(chunks, chunk)
			break
		}
		chunks = append(chunks, chunk)
	}

	// Verify we got tool call chunks
	assert.NotEmpty(t, chunks)
	hasToolCalls := false
	for _, chunk := range chunks {
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			hasToolCalls = true
			break
		}
	}
	assert.True(t, hasToolCalls, "Should have received tool call chunks")

	_ = stream.Close()
}

// TestToolCalling_ToolCallsInMessages tests that tool calls in messages are converted
func TestToolCalling_ToolCallsInMessages(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "sk-or-test-key",
	}
	provider := NewOpenRouterProvider(config)

	toolCalls := []types.ToolCall{
		{
			ID:   "call_123",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"San Francisco, CA"}`,
			},
		},
	}

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:      "assistant",
				Content:   "",
				ToolCalls: toolCalls,
			},
		},
	}

	request, err := provider.prepareRequest(options)
	require.NoError(t, err)

	// Verify tool calls are included in messages
	assert.Len(t, request.Messages, 1)
	assert.Len(t, request.Messages[0].ToolCalls, 1)
	assert.Equal(t, "call_123", request.Messages[0].ToolCalls[0].ID)
	assert.Equal(t, "function", request.Messages[0].ToolCalls[0].Type)
	assert.Equal(t, "get_weather", request.Messages[0].ToolCalls[0].Function.Name)
}

// TestToolCalling_ToolResponses tests that tool responses are included
func TestToolCalling_ToolResponses(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "sk-or-test-key",
	}
	provider := NewOpenRouterProvider(config)

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:       "tool",
				Content:    `{"temperature": 72, "unit": "fahrenheit"}`,
				ToolCallID: "call_123",
			},
		},
	}

	request, err := provider.prepareRequest(options)
	require.NoError(t, err)

	// Verify tool call ID is included
	assert.Len(t, request.Messages, 1)
	assert.Equal(t, "call_123", request.Messages[0].ToolCallID)
	assert.Equal(t, "tool", request.Messages[0].Role)
}

// TestToolCalling_ConversionHelpers tests the tool conversion helper functions
func TestToolCalling_ConversionHelpers(t *testing.T) {
	t.Run("ConvertToOpenRouterTools", func(t *testing.T) {
		tools := []types.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"param1": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		}

		openrouterTools := convertToOpenRouterTools(tools)

		assert.Len(t, openrouterTools, 1)
		assert.Equal(t, "function", openrouterTools[0].Type)
		assert.Equal(t, "test_tool", openrouterTools[0].Function.Name)
		assert.Equal(t, "A test tool", openrouterTools[0].Function.Description)
		assert.NotNil(t, openrouterTools[0].Function.Parameters)
	})

	t.Run("ConvertOpenRouterToolCallsToUniversal", func(t *testing.T) {
		openrouterToolCalls := []OpenRouterToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: OpenRouterToolCallFunction{
					Name:      "test_function",
					Arguments: `{"arg":"value"}`,
				},
			},
		}

		universalToolCalls := convertOpenRouterToolCallsToUniversal(openrouterToolCalls)

		assert.Len(t, universalToolCalls, 1)
		assert.Equal(t, "call_123", universalToolCalls[0].ID)
		assert.Equal(t, "function", universalToolCalls[0].Type)
		assert.Equal(t, "test_function", universalToolCalls[0].Function.Name)
		assert.Equal(t, `{"arg":"value"}`, universalToolCalls[0].Function.Arguments)
	})

	t.Run("RoundTripConversion", func(t *testing.T) {
		// Test that converting back and forth preserves data
		original := []types.ToolCall{
			{
				ID:   "call_round_trip",
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      "round_trip_func",
					Arguments: `{"test":"data","nested":{"key":"value"}}`,
				},
			},
		}

		// Convert to OpenRouter format and back
		openrouterFormat := convertToOpenRouterToolCalls(original)
		backToUniversal := convertOpenRouterToolCallsToUniversal(openrouterFormat)

		assert.Equal(t, original, backToUniversal)
	})
}

// TestToolCalling_OpenAICompatibility tests that OpenRouter uses OpenAI-compatible format
func TestToolCalling_OpenAICompatibility(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "sk-or-test-key",
	}
	provider := NewOpenRouterProvider(config)

	// Verify that OpenRouter reports OpenAI tool format
	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())

	// Create a sample tool and verify the structure matches OpenAI format
	tools := []types.Tool{
		{
			Name:        "test_tool",
			Description: "Test description",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	openrouterTools := convertToOpenRouterTools(tools)

	// Verify structure matches OpenAI format
	assert.Equal(t, "function", openrouterTools[0].Type)
	assert.Equal(t, "test_tool", openrouterTools[0].Function.Name)
	assert.Equal(t, "Test description", openrouterTools[0].Function.Description)
	assert.NotNil(t, openrouterTools[0].Function.Parameters)

	// Verify the JSON structure would be compatible with OpenAI
	jsonBytes, err := json.Marshal(openrouterTools[0])
	require.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "function", unmarshaled["type"])
	assert.NotNil(t, unmarshaled["function"])
}
