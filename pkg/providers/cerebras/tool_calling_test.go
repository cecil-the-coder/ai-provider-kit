package cerebras

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
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}
	provider := NewCerebrasProvider(config)

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

	// Test tool conversion
	cerebrasTools := convertToCerebrasTools(tools)

	// Verify tools are correctly converted
	assert.Len(t, cerebrasTools, 1)
	assert.Equal(t, "function", cerebrasTools[0].Type)
	assert.Equal(t, "get_weather", cerebrasTools[0].Function.Name)
	assert.Equal(t, "Get the current weather in a location", cerebrasTools[0].Function.Description)
	assert.NotNil(t, cerebrasTools[0].Function.Parameters)

	// Verify the provider supports tool calling
	assert.True(t, provider.SupportsToolCalling())
	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())
}

// TestToolCalling_ResponseParsing tests that tool calls are parsed from responses
func TestToolCalling_ResponseParsing(t *testing.T) {
	// Create a mock HTTP server that returns tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response with tool calls
		response := map[string]interface{}{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "zai-glm-4.6",
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
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: server.URL,
	}
	provider := NewCerebrasProvider(config)

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
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "What's the weather in SF?",
			},
		},
		Tools: tools,
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
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send streaming chunks
		chunks := []string{
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"zai-glm-4.6","choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_xyz","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":""}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"zai-glm-4.6","choices":[{"index":0,"message":{"tool_calls":[{"function":{"arguments":"{\"location\""}}]},"finish_reason":""}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"zai-glm-4.6","choices":[{"index":0,"message":{"tool_calls":[{"function":{"arguments":":\"Boston\"}"}}]},"finish_reason":""}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"zai-glm-4.6","choices":[{"index":0,"message":{},"finish_reason":"tool_calls"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "%s\n\n", chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: server.URL,
	}
	provider := NewCerebrasProvider(config)

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "What's the weather?",
			},
		},
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

	// Verify we got chunks
	assert.NotEmpty(t, chunks)

	_ = stream.Close()
}

// TestToolCalling_ToolCallsInMessages tests that tool calls in messages are converted
func TestToolCalling_ToolCallsInMessages(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}
	provider := NewCerebrasProvider(config)

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

	// Convert to Cerebras format
	cerebrasToolCalls := convertToCerebrasToolCalls(toolCalls)

	// Verify conversion
	assert.Len(t, cerebrasToolCalls, 1)
	assert.Equal(t, "call_123", cerebrasToolCalls[0].ID)
	assert.Equal(t, "function", cerebrasToolCalls[0].Type)
	assert.Equal(t, "get_weather", cerebrasToolCalls[0].Function.Name)

	// Verify the provider info
	assert.True(t, provider.SupportsToolCalling())
}

// TestToolCalling_ToolResponses tests that tool responses are included
func TestToolCalling_ToolResponses(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}
	provider := NewCerebrasProvider(config)

	messages := []types.ChatMessage{
		{
			Role:       "tool",
			Content:    `{"temperature": 72, "unit": "fahrenheit"}`,
			ToolCallID: "call_123",
		},
	}

	// Build Cerebras messages
	cerebrasMessages := make([]CerebrasMessage, 0, len(messages))
	for _, msg := range messages {
		cerebrasMsg := CerebrasMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		if msg.ToolCallID != "" {
			cerebrasMsg.ToolCallID = msg.ToolCallID
		}

		cerebrasMessages = append(cerebrasMessages, cerebrasMsg)
	}

	// Verify tool call ID is included
	assert.Len(t, cerebrasMessages, 1)
	assert.Equal(t, "call_123", cerebrasMessages[0].ToolCallID)
	assert.Equal(t, "tool", cerebrasMessages[0].Role)

	// Verify provider info
	assert.True(t, provider.SupportsToolCalling())
}

// TestToolCalling_ConversionHelpers tests the tool conversion helper functions
func TestToolCalling_ConversionHelpers(t *testing.T) {
	t.Run("ConvertToCerebrasTools", func(t *testing.T) {
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

		cerebrasTools := convertToCerebrasTools(tools)

		assert.Len(t, cerebrasTools, 1)
		assert.Equal(t, "function", cerebrasTools[0].Type)
		assert.Equal(t, "test_tool", cerebrasTools[0].Function.Name)
		assert.Equal(t, "A test tool", cerebrasTools[0].Function.Description)
		assert.NotNil(t, cerebrasTools[0].Function.Parameters)
	})

	t.Run("ConvertCerebrasToolCallsToUniversal", func(t *testing.T) {
		cerebrasToolCalls := []CerebrasToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: CerebrasToolCallFunction{
					Name:      "test_function",
					Arguments: `{"arg":"value"}`,
				},
			},
		}

		universalToolCalls := convertCerebrasToolCallsToUniversal(cerebrasToolCalls)

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

		// Convert to Cerebras format and back
		cerebrasFormat := convertToCerebrasToolCalls(original)
		backToUniversal := convertCerebrasToolCallsToUniversal(cerebrasFormat)

		assert.Equal(t, original, backToUniversal)
	})
}

// TestToolCalling_ModelMetadata tests that models have correct tool calling metadata
func TestToolCalling_ModelMetadata(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}
	provider := NewCerebrasProvider(config)

	models, err := provider.GetModels(context.Background())
	require.NoError(t, err)

	// All models should support tool calling
	for _, model := range models {
		assert.True(t, model.SupportsToolCalling, "Model %s should support tool calling", model.ID)
	}
}
