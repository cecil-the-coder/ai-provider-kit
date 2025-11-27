package openai

import (
	"context"
	"encoding/json"
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
	provider := createTestProvider(t)
	tools := []types.Tool{createTestTool()}
	testToolConversionInRequest(t, provider, tools, "What's the weather like?")
}

// TestToolCalling_ResponseParsing tests that tool calls are parsed from responses
func TestToolCalling_ResponseParsing(t *testing.T) {
	server := createMockToolCallServer(t)
	defer server.Close()

	provider := createProviderWithMockServer(t, server)

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
	testToolCallResponse(t, chunk)

	_ = stream.Close()
}

// TestToolCalling_StreamingToolCalls tests streaming tool calls
func TestToolCalling_StreamingToolCalls(t *testing.T) {
	server := createMockStreamingToolCallServer(t)
	defer server.Close()

	provider := createProviderWithMockServer(t, server)

	options := types.GenerateOptions{
		Prompt: "What's the weather?",
		Stream: true,
		Tools:  []types.Tool{createSimpleTestTool()},
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	testStreamingToolCalls(t, stream)
}

// TestToolCalling_ToolCallsInMessages tests that tool calls in messages are converted
func TestToolCalling_ToolCallsInMessages(t *testing.T) {
	provider := createTestProvider(t)
	toolCalls := []types.ToolCall{createTestToolCall()}
	testToolCallsInMessages(t, provider, toolCalls)
}

// TestToolCalling_ToolResponses tests that tool responses are included
func TestToolCalling_ToolResponses(t *testing.T) {
	provider := createTestProvider(t)
	testToolResponses(t, provider)
}

// TestToolCalling_ConversionHelpers tests the tool conversion helper functions
func TestToolCalling_ConversionHelpers(t *testing.T) {
	t.Run("ConvertToOpenAITools", func(t *testing.T) {
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

		openaiTools := convertToOpenAITools(tools)

		assert.Len(t, openaiTools, 1)
		assert.Equal(t, "function", openaiTools[0].Type)
		assert.Equal(t, "test_tool", openaiTools[0].Function.Name)
		assert.Equal(t, "A test tool", openaiTools[0].Function.Description)
		assert.NotNil(t, openaiTools[0].Function.Parameters)
	})

	t.Run("ConvertOpenAIToolCallsToUniversal", func(t *testing.T) {
		openaiToolCalls := []OpenAIToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: OpenAIToolCallFunction{
					Name:      "test_function",
					Arguments: `{"arg":"value"}`,
				},
			},
		}

		universalToolCalls := convertOpenAIToolCallsToUniversal(openaiToolCalls)

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

		// Convert to OpenAI format and back
		openaiFormat := convertToOpenAIToolCalls(original)
		backToUniversal := convertOpenAIToolCallsToUniversal(openaiFormat)

		assert.Equal(t, original, backToUniversal)
	})
}

// TestToolChoice_AutoMode tests tool choice in auto mode
func TestToolChoice_AutoMode(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get weather",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather?",
		Tools:  tools,
		ToolChoice: &types.ToolChoice{
			Mode: types.ToolChoiceAuto,
		},
	}

	request := provider.buildOpenAIRequest(options)

	assert.NotNil(t, request.Tools)
	assert.Equal(t, "auto", request.ToolChoice)
}

// TestToolChoice_RequiredMode tests tool choice in required mode
func TestToolChoice_RequiredMode(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tools := []types.Tool{
		{
			Name:        "calculator",
			Description: "Perform calculations",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "Calculate 5 + 3",
		Tools:  tools,
		ToolChoice: &types.ToolChoice{
			Mode: types.ToolChoiceRequired,
		},
	}

	request := provider.buildOpenAIRequest(options)

	assert.NotNil(t, request.Tools)
	assert.Equal(t, "required", request.ToolChoice)
}

// TestToolChoice_NoneMode tests tool choice in none mode
func TestToolChoice_NoneMode(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tools := []types.Tool{
		{
			Name:        "search",
			Description: "Search the web",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "Tell me about AI",
		Tools:  tools,
		ToolChoice: &types.ToolChoice{
			Mode: types.ToolChoiceNone,
		},
	}

	request := provider.buildOpenAIRequest(options)

	assert.NotNil(t, request.Tools)
	assert.Equal(t, "none", request.ToolChoice)
}

// TestToolChoice_SpecificMode tests tool choice in specific mode
func TestToolChoice_SpecificMode(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get weather",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_time",
			Description: "Get current time",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather?",
		Tools:  tools,
		ToolChoice: &types.ToolChoice{
			Mode:         types.ToolChoiceSpecific,
			FunctionName: "get_weather",
		},
	}

	request := provider.buildOpenAIRequest(options)

	assert.NotNil(t, request.Tools)
	assert.Len(t, request.Tools, 2)

	// Verify specific tool choice format
	toolChoiceMap, ok := request.ToolChoice.(map[string]interface{})
	require.True(t, ok, "ToolChoice should be a map for specific mode")
	assert.Equal(t, "function", toolChoiceMap["type"])

	functionMap, ok := toolChoiceMap["function"].(map[string]interface{})
	require.True(t, ok, "function should be a map")
	assert.Equal(t, "get_weather", functionMap["name"])
}

// TestToolChoice_NoToolChoice tests default behavior when ToolChoice is not specified
func TestToolChoice_NoToolChoice(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tools := []types.Tool{
		{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt:     "Test prompt",
		Tools:      tools,
		ToolChoice: nil, // No tool choice specified
	}

	request := provider.buildOpenAIRequest(options)

	assert.NotNil(t, request.Tools)
	assert.Nil(t, request.ToolChoice) // Should be nil (defaults to "auto" on OpenAI's side)
}

// TestToolChoice_WithMultipleTools tests tool choice with multiple tools
func TestToolChoice_WithMultipleTools(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tools := []types.Tool{
		{
			Name:        "tool1",
			Description: "First tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "tool2",
			Description: "Second tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "tool3",
			Description: "Third tool",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "Use tools",
		Tools:  tools,
		ToolChoice: &types.ToolChoice{
			Mode: types.ToolChoiceRequired,
		},
	}

	request := provider.buildOpenAIRequest(options)

	assert.Len(t, request.Tools, 3)
	assert.Equal(t, "required", request.ToolChoice)
}

// TestParallelToolCalls tests that multiple tool calls in one response are parsed correctly
func TestParallelToolCalls(t *testing.T) {
	// Create a mock HTTP server that returns multiple tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":      "chatcmpl-parallel",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_weather",
									"arguments": `{"location":"New York"}`,
								},
							},
							{
								"id":   "call_2",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_weather",
									"arguments": `{"location":"London"}`,
								},
							},
							{
								"id":   "call_3",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_time",
									"arguments": `{"timezone":"UTC"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     20,
				"completion_tokens": 30,
				"total_tokens":      50,
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

	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get weather",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_time",
			Description: "Get time",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather in NY and London, and what time is it in UTC?",
		Tools:  tools,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.True(t, chunk.Done)

	// Verify we got 3 tool calls
	require.Len(t, chunk.Choices, 1)
	require.Len(t, chunk.Choices[0].Message.ToolCalls, 3)

	// Verify first tool call
	assert.Equal(t, "call_1", chunk.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", chunk.Choices[0].Message.ToolCalls[0].Function.Name)
	assert.Contains(t, chunk.Choices[0].Message.ToolCalls[0].Function.Arguments, "New York")

	// Verify second tool call
	assert.Equal(t, "call_2", chunk.Choices[0].Message.ToolCalls[1].ID)
	assert.Equal(t, "get_weather", chunk.Choices[0].Message.ToolCalls[1].Function.Name)
	assert.Contains(t, chunk.Choices[0].Message.ToolCalls[1].Function.Arguments, "London")

	// Verify third tool call
	assert.Equal(t, "call_3", chunk.Choices[0].Message.ToolCalls[2].ID)
	assert.Equal(t, "get_time", chunk.Choices[0].Message.ToolCalls[2].Function.Name)
	assert.Contains(t, chunk.Choices[0].Message.ToolCalls[2].Function.Arguments, "UTC")

	_ = stream.Close()
}
