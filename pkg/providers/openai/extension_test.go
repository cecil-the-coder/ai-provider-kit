package openai

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewOpenAIExtension tests the creation of the OpenAI extension
func TestNewOpenAIExtension(t *testing.T) {
	ext := NewOpenAIExtension()

	assert.NotNil(t, ext)
	assert.Equal(t, "openai", ext.Name())
	assert.Equal(t, "1.0.0", ext.Version())
	assert.Contains(t, ext.Description(), "OpenAI API extension")

	// Verify capabilities
	capabilities := ext.GetCapabilities()
	expectedCapabilities := []string{
		"chat", "streaming", "tool_calling", "function_calling",
		"json_mode", "system_messages", "temperature", "top_p",
		"max_tokens", "stop_sequences", "seed", "parallel_tool_calls",
	}

	for _, cap := range expectedCapabilities {
		assert.Contains(t, capabilities, cap)
	}
}

// TestStandardToProvider tests converting standard request to OpenAI format
func TestStandardToProvider(t *testing.T) {
	ext := NewOpenAIExtension()

	t.Run("BasicRequest", func(t *testing.T) {
		request := types.StandardRequest{
			Model:       "gpt-4",
			MaxTokens:   100,
			Temperature: 0.7,
			Stream:      false,
			Messages: []types.ChatMessage{
				{Role: "system", Content: "You are helpful."},
				{Role: "user", Content: "Hello!"},
			},
		}

		result, err := ext.StandardToProvider(request)
		require.NoError(t, err)

		openAIReq, ok := result.(OpenAIRequest)
		require.True(t, ok)

		assert.Equal(t, "gpt-4", openAIReq.Model)
		assert.Equal(t, 100, openAIReq.MaxTokens)
		assert.Equal(t, 0.7, openAIReq.Temperature)
		assert.False(t, openAIReq.Stream)
		assert.Len(t, openAIReq.Messages, 2)
		assert.Equal(t, "system", openAIReq.Messages[0].Role)
		assert.Equal(t, "You are helpful.", openAIReq.Messages[0].Content)
	})

	t.Run("WithTools", func(t *testing.T) {
		request := types.StandardRequest{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{Role: "user", Content: "What's the weather?"},
			},
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
			ToolChoice: &types.ToolChoice{
				Mode: types.ToolChoiceAuto,
			},
		}

		result, err := ext.StandardToProvider(request)
		require.NoError(t, err)

		openAIReq, ok := result.(OpenAIRequest)
		require.True(t, ok)

		assert.Len(t, openAIReq.Tools, 1)
		assert.Equal(t, "function", openAIReq.Tools[0].Type)
		assert.Equal(t, "get_weather", openAIReq.Tools[0].Function.Name)
		assert.Equal(t, "auto", openAIReq.ToolChoice)
	})

	t.Run("WithStopSequences", func(t *testing.T) {
		request := types.StandardRequest{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Test"},
			},
			Stop: []string{"END", "STOP"},
		}

		result, err := ext.StandardToProvider(request)
		require.NoError(t, err)

		openAIReq, ok := result.(OpenAIRequest)
		require.True(t, ok)

		assert.Equal(t, []string{"END", "STOP"}, openAIReq.Stop)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		seed := 42
		request := types.StandardRequest{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Test"},
			},
			Metadata: map[string]interface{}{
				"top_p": 0.9,
				"seed":  seed,
				"response_format": map[string]interface{}{
					"type": "json_object",
				},
				"parallel_tool_calls": true,
			},
		}

		result, err := ext.StandardToProvider(request)
		require.NoError(t, err)

		openAIReq, ok := result.(OpenAIRequest)
		require.True(t, ok)

		assert.Equal(t, 0.9, openAIReq.TopP)
		assert.NotNil(t, openAIReq.Seed)
		assert.Equal(t, 42, *openAIReq.Seed)
		assert.NotNil(t, openAIReq.ResponseFormat)
		assert.Equal(t, "json_object", openAIReq.ResponseFormat["type"])
		assert.NotNil(t, openAIReq.ParallelToolCalls)
		assert.True(t, *openAIReq.ParallelToolCalls)
	})

	t.Run("WithToolCalls", func(t *testing.T) {
		request := types.StandardRequest{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{
					Role:    "assistant",
					Content: "",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: types.ToolCallFunction{
								Name:      "get_weather",
								Arguments: `{"location":"SF"}`,
							},
						},
					},
				},
			},
		}

		result, err := ext.StandardToProvider(request)
		require.NoError(t, err)

		openAIReq, ok := result.(OpenAIRequest)
		require.True(t, ok)

		assert.Len(t, openAIReq.Messages, 1)
		assert.Len(t, openAIReq.Messages[0].ToolCalls, 1)
		assert.Equal(t, "call_123", openAIReq.Messages[0].ToolCalls[0].ID)
	})

	t.Run("WithToolCallID", func(t *testing.T) {
		request := types.StandardRequest{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{
					Role:       "tool",
					Content:    `{"result":"sunny"}`,
					ToolCallID: "call_123",
				},
			},
		}

		result, err := ext.StandardToProvider(request)
		require.NoError(t, err)

		openAIReq, ok := result.(OpenAIRequest)
		require.True(t, ok)

		assert.Len(t, openAIReq.Messages, 1)
		assert.Equal(t, "call_123", openAIReq.Messages[0].ToolCallID)
	})
}

// TestProviderToStandard tests converting OpenAI response to standard format
func TestProviderToStandard(t *testing.T) {
	ext := NewOpenAIExtension()

	t.Run("BasicResponse", func(t *testing.T) {
		openAIResp := &OpenAIResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "Hello there!",
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
			SystemFingerprint: "fp_123",
		}

		result, err := ext.ProviderToStandard(openAIResp)
		require.NoError(t, err)

		assert.Equal(t, "chatcmpl-123", result.ID)
		assert.Equal(t, "gpt-4", result.Model)
		assert.Equal(t, "chat.completion", result.Object)
		assert.Equal(t, int64(1234567890), result.Created)
		assert.Len(t, result.Choices, 1)
		assert.Equal(t, "assistant", result.Choices[0].Message.Role)
		assert.Equal(t, "Hello there!", result.Choices[0].Message.Content)
		assert.Equal(t, "stop", result.Choices[0].FinishReason)
		assert.Equal(t, 10, result.Usage.PromptTokens)
		assert.Equal(t, 5, result.Usage.CompletionTokens)
		assert.Equal(t, 15, result.Usage.TotalTokens)
		assert.Equal(t, "fp_123", result.ProviderMetadata["system_fingerprint"])
	})

	t.Run("WithToolCalls", func(t *testing.T) {
		openAIResp := &OpenAIResponse{
			ID:      "chatcmpl-456",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: "",
						ToolCalls: []OpenAIToolCall{
							{
								ID:   "call_abc",
								Type: "function",
								Function: OpenAIToolCallFunction{
									Name:      "get_weather",
									Arguments: `{"location":"NYC"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     20,
				CompletionTokens: 10,
				TotalTokens:      30,
			},
		}

		result, err := ext.ProviderToStandard(openAIResp)
		require.NoError(t, err)

		assert.Len(t, result.Choices, 1)
		assert.Len(t, result.Choices[0].Message.ToolCalls, 1)
		assert.Equal(t, "call_abc", result.Choices[0].Message.ToolCalls[0].ID)
		assert.Equal(t, "function", result.Choices[0].Message.ToolCalls[0].Type)
		assert.Equal(t, "get_weather", result.Choices[0].Message.ToolCalls[0].Function.Name)
		assert.Equal(t, `{"location":"NYC"}`, result.Choices[0].Message.ToolCalls[0].Function.Arguments)
	})

	t.Run("InvalidType", func(t *testing.T) {
		result, err := ext.ProviderToStandard("not a response")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not an OpenAI response")
	})
}

// TestProviderToStandardChunk tests converting OpenAI stream chunk to standard format
func TestProviderToStandardChunk(t *testing.T) {
	ext := NewOpenAIExtension()

	t.Run("BasicChunk", func(t *testing.T) {
		openAIChunk := &OpenAIStreamResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion.chunk",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index: 0,
					Delta: OpenAIDelta{
						Role:    "assistant",
						Content: "Hello",
					},
					FinishReason: "",
				},
			},
		}

		result, err := ext.ProviderToStandardChunk(openAIChunk)
		require.NoError(t, err)

		assert.Equal(t, "chatcmpl-123", result.ID)
		assert.Equal(t, "gpt-4", result.Model)
		assert.Len(t, result.Choices, 1)
		assert.Equal(t, "assistant", result.Choices[0].Delta.Role)
		assert.Equal(t, "Hello", result.Choices[0].Delta.Content)
		assert.False(t, result.Done)
	})

	t.Run("FinalChunk", func(t *testing.T) {
		openAIChunk := &OpenAIStreamResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion.chunk",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index:        0,
					Delta:        OpenAIDelta{},
					FinishReason: "stop",
				},
			},
			Usage: &OpenAIUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
			SystemFingerprint: "fp_456",
		}

		result, err := ext.ProviderToStandardChunk(openAIChunk)
		require.NoError(t, err)

		assert.True(t, result.Done)
		assert.NotNil(t, result.Usage)
		assert.Equal(t, 10, result.Usage.PromptTokens)
		assert.Equal(t, 20, result.Usage.CompletionTokens)
		assert.Equal(t, 30, result.Usage.TotalTokens)
		assert.Equal(t, "fp_456", result.ProviderMetadata["system_fingerprint"])
	})

	t.Run("ChunkWithToolCalls", func(t *testing.T) {
		openAIChunk := &OpenAIStreamResponse{
			ID:      "chatcmpl-789",
			Object:  "chat.completion.chunk",
			Created: 1234567890,
			Model:   "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index: 0,
					Delta: OpenAIDelta{
						ToolCalls: []OpenAIToolCall{
							{
								ID:   "call_xyz",
								Type: "function",
								Function: OpenAIToolCallFunction{
									Name:      "calculator",
									Arguments: `{"expr":"2+2"}`,
								},
							},
						},
					},
					FinishReason: "",
				},
			},
		}

		result, err := ext.ProviderToStandardChunk(openAIChunk)
		require.NoError(t, err)

		assert.Len(t, result.Choices, 1)
		assert.Len(t, result.Choices[0].Delta.ToolCalls, 1)
		assert.Equal(t, "call_xyz", result.Choices[0].Delta.ToolCalls[0].ID)
	})

	t.Run("InvalidType", func(t *testing.T) {
		result, err := ext.ProviderToStandardChunk("not a chunk")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not an OpenAI stream response")
	})
}

// TestValidateOptions tests validation of OpenAI-specific options
func TestValidateOptions(t *testing.T) {
	ext := NewOpenAIExtension()

	t.Run("ValidTopP", func(t *testing.T) {
		options := map[string]interface{}{
			"top_p": 0.5,
		}
		err := ext.ValidateOptions(options)
		assert.NoError(t, err)
	})

	t.Run("InvalidTopPTooLow", func(t *testing.T) {
		options := map[string]interface{}{
			"top_p": -0.1,
		}
		err := ext.ValidateOptions(options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "top_p must be between 0 and 1")
	})

	t.Run("InvalidTopPTooHigh", func(t *testing.T) {
		options := map[string]interface{}{
			"top_p": 1.5,
		}
		err := ext.ValidateOptions(options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "top_p must be between 0 and 1")
	})

	t.Run("ValidSeed", func(t *testing.T) {
		options := map[string]interface{}{
			"seed": 42,
		}
		err := ext.ValidateOptions(options)
		assert.NoError(t, err)
	})

	t.Run("InvalidSeedNegative", func(t *testing.T) {
		options := map[string]interface{}{
			"seed": -1,
		}
		err := ext.ValidateOptions(options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "seed must be non-negative")
	})

	t.Run("ValidResponseFormat", func(t *testing.T) {
		options := map[string]interface{}{
			"response_format": map[string]interface{}{
				"type": "json_object",
			},
		}
		err := ext.ValidateOptions(options)
		assert.NoError(t, err)
	})

	t.Run("ValidResponseFormatJsonSchema", func(t *testing.T) {
		options := map[string]interface{}{
			"response_format": map[string]interface{}{
				"type": "json_schema",
			},
		}
		err := ext.ValidateOptions(options)
		assert.NoError(t, err)
	})

	t.Run("InvalidResponseFormatType", func(t *testing.T) {
		options := map[string]interface{}{
			"response_format": map[string]interface{}{
				"type": "invalid_type",
			},
		}
		err := ext.ValidateOptions(options)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "response_format.type must be one of")
	})

	t.Run("MultipleValidOptions", func(t *testing.T) {
		options := map[string]interface{}{
			"top_p": 0.8,
			"seed":  100,
			"response_format": map[string]interface{}{
				"type": "text",
			},
		}
		err := ext.ValidateOptions(options)
		assert.NoError(t, err)
	})

	t.Run("EmptyOptions", func(t *testing.T) {
		options := map[string]interface{}{}
		err := ext.ValidateOptions(options)
		assert.NoError(t, err)
	})
}
