package openai

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenAIProvider_ToolCalling tests tool calling functionality
func TestOpenAIProvider_ToolCalling(t *testing.T) {
	t.Run("ToolsConvertedInRequest", func(t *testing.T) {
		provider := createTestProvider(t)
		tools := []types.Tool{createTestTool()}
		testToolConversionInRequest(t, provider, tools, "What's the weather like?")
	})

	t.Run("ToolCallsConvertedInMessages", func(t *testing.T) {
		provider := createTestProvider(t)
		toolCalls := []types.ToolCall{createTestToolCall()}
		testToolCallsInMessages(t, provider, toolCalls)
	})

	t.Run("ToolResponsesIncluded", func(t *testing.T) {
		provider := createTestProvider(t)
		testToolResponses(t, provider)
	})

	t.Run("MockServerWithToolCalls", func(t *testing.T) {
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
	})

	t.Run("StreamingWithToolCalls", func(t *testing.T) {
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
	})
}

// TestOpenAIProvider_ToolConversionHelpers tests the tool conversion helper functions
func TestOpenAIProvider_ToolConversionHelpers(t *testing.T) {
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

	t.Run("ConvertToOpenAIToolCalls", func(t *testing.T) {
		toolCalls := []types.ToolCall{
			{
				ID:   "call_456",
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      "another_function",
					Arguments: `{"key":"val"}`,
				},
			},
		}

		openaiToolCalls := convertToOpenAIToolCalls(toolCalls)

		assert.Len(t, openaiToolCalls, 1)
		assert.Equal(t, "call_456", openaiToolCalls[0].ID)
		assert.Equal(t, "function", openaiToolCalls[0].Type)
		assert.Equal(t, "another_function", openaiToolCalls[0].Function.Name)
		assert.Equal(t, `{"key":"val"}`, openaiToolCalls[0].Function.Arguments)
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
