package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChatMessage tests the ChatMessage struct
func TestChatMessage(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var message ChatMessage
		assert.Empty(t, message.Role)
		assert.Empty(t, message.Content)
		assert.Empty(t, message.ToolCalls)
		assert.Empty(t, message.ToolCallID)
		assert.Nil(t, message.Metadata)
	})

	t.Run("BasicMessage", func(t *testing.T) {
		message := ChatMessage{
			Role:    "user",
			Content: "Hello, how are you?",
		}

		assert.Equal(t, "user", message.Role)
		assert.Equal(t, "Hello, how are you?", message.Content)
		assert.Empty(t, message.ToolCalls)
		assert.Empty(t, message.ToolCallID)
		assert.Nil(t, message.Metadata)
	})

	t.Run("MessageWithMetadata", func(t *testing.T) {
		metadata := map[string]interface{}{
			"timestamp": 1234567890,
			"source":    "web",
			"priority":  "high",
		}

		message := ChatMessage{
			Role:     "assistant",
			Content:  "I'm doing well, thank you!",
			Metadata: metadata,
		}

		assert.Equal(t, "assistant", message.Role)
		assert.Equal(t, "I'm doing well, thank you!", message.Content)
		assert.Equal(t, metadata, message.Metadata)
	})

	t.Run("MessageWithToolCalls", func(t *testing.T) {
		toolCalls := []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: ToolCallFunction{
					Name:      "get_weather",
					Arguments: `{"location": "New York"}`,
				},
			},
			{
				ID:   "call_2",
				Type: "function",
				Function: ToolCallFunction{
					Name:      "get_time",
					Arguments: `{}`,
				},
			},
		}

		message := ChatMessage{
			Role:       "assistant",
			Content:    "I'll help you with that.",
			ToolCalls:  toolCalls,
			ToolCallID: "msg_123",
		}

		assert.Equal(t, "assistant", message.Role)
		assert.Equal(t, "I'll help you with that.", message.Content)
		assert.Equal(t, toolCalls, message.ToolCalls)
		assert.Equal(t, "msg_123", message.ToolCallID)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		metadata := map[string]interface{}{
			"model": "gpt-4",
			"cost":  0.05,
		}

		message := ChatMessage{
			Role:       "user",
			Content:    "What's the weather like?",
			ToolCallID: "tool_call_123",
			Metadata:   metadata,
		}

		data, err := json.Marshal(message)
		require.NoError(t, err)

		var result ChatMessage
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, message.Role, result.Role)
		assert.Equal(t, message.Content, result.Content)
		assert.Equal(t, message.ToolCallID, result.ToolCallID)
		assert.Equal(t, metadata, result.Metadata)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			message ChatMessage
			valid   bool
		}{
			{
				name:    "EmptyMessage",
				message: ChatMessage{},
				valid:   false,
			},
			{
				name: "OnlyRole",
				message: ChatMessage{
					Role: "user",
				},
				valid: false,
			},
			{
				name: "RoleAndContent",
				message: ChatMessage{
					Role:    "user",
					Content: "Hello",
				},
				valid: true,
			},
			{
				name: "ToolResponse",
				message: ChatMessage{
					Role:       "tool",
					Content:    "The result is 42",
					ToolCallID: "call_123",
				},
				valid: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid := tt.message.Validate()
				assert.Equal(t, tt.valid, valid)
			})
		}
	})
}

// TestChatCompletionChunk tests the ChatCompletionChunk struct
func TestChatCompletionChunk(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var chunk ChatCompletionChunk
		assert.Empty(t, chunk.ID)
		assert.Empty(t, chunk.Object)
		assert.Equal(t, int64(0), chunk.Created)
		assert.Empty(t, chunk.Model)
		assert.Empty(t, chunk.Choices)
		assert.Equal(t, Usage{}, chunk.Usage)
		assert.False(t, chunk.Done)
		assert.Empty(t, chunk.Content)
		assert.Empty(t, chunk.Error)
	})

	t.Run("FullChunk", func(t *testing.T) {
		choices := []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: "Hello!",
				},
				FinishReason: "stop",
			},
		}

		usage := Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		}

		chunk := ChatCompletionChunk{
			ID:      "chunk_123",
			Object:  "chat.completion.chunk",
			Created: 1640995200,
			Model:   "gpt-4",
			Choices: choices,
			Usage:   usage,
			Done:    true,
			Content: "Hello!",
		}

		assert.Equal(t, "chunk_123", chunk.ID)
		assert.Equal(t, "chat.completion.chunk", chunk.Object)
		assert.Equal(t, int64(1640995200), chunk.Created)
		assert.Equal(t, "gpt-4", chunk.Model)
		assert.Equal(t, choices, chunk.Choices)
		assert.Equal(t, usage, chunk.Usage)
		assert.True(t, chunk.Done)
		assert.Equal(t, "Hello!", chunk.Content)
	})

	t.Run("ErrorChunk", func(t *testing.T) {
		chunk := ChatCompletionChunk{
			ID:    "error_chunk",
			Done:  true,
			Error: "Rate limit exceeded",
		}

		assert.Equal(t, "error_chunk", chunk.ID)
		assert.True(t, chunk.Done)
		assert.Equal(t, "Rate limit exceeded", chunk.Error)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		chunk := ChatCompletionChunk{
			ID:      "test_chunk",
			Object:  "chat.completion.chunk",
			Created: 1640995200,
			Model:   "test-model",
			Done:    false,
			Content: "Hello",
		}

		data, err := json.Marshal(chunk)
		require.NoError(t, err)

		var result ChatCompletionChunk
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, chunk.ID, result.ID)
		assert.Equal(t, chunk.Object, result.Object)
		assert.Equal(t, chunk.Created, result.Created)
		assert.Equal(t, chunk.Model, result.Model)
		assert.Equal(t, chunk.Done, result.Done)
		assert.Equal(t, chunk.Content, result.Content)
	})
}

// TestChatChoice tests the ChatChoice struct
func TestChatChoice(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var choice ChatChoice
		assert.Equal(t, 0, choice.Index)
		assert.Equal(t, ChatMessage{}, choice.Message)
		assert.Empty(t, choice.FinishReason)
		assert.Equal(t, ChatMessage{}, choice.Delta)
	})

	t.Run("FullChoice", func(t *testing.T) {
		message := ChatMessage{
			Role:    "assistant",
			Content: "The answer is 42.",
		}

		delta := ChatMessage{
			Role:    "assistant",
			Content: "The answer",
		}

		choice := ChatChoice{
			Index:        0,
			Message:      message,
			FinishReason: "stop",
			Delta:        delta,
		}

		assert.Equal(t, 0, choice.Index)
		assert.Equal(t, message, choice.Message)
		assert.Equal(t, "stop", choice.FinishReason)
		assert.Equal(t, delta, choice.Delta)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		message := ChatMessage{
			Role:    "assistant",
			Content: "Complete response",
		}

		choice := ChatChoice{
			Index:        1,
			Message:      message,
			FinishReason: "length",
		}

		data, err := json.Marshal(choice)
		require.NoError(t, err)

		var result ChatChoice
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, choice.Index, result.Index)
		assert.Equal(t, choice.Message.Role, result.Message.Role)
		assert.Equal(t, choice.Message.Content, result.Message.Content)
		assert.Equal(t, choice.FinishReason, result.FinishReason)
	})
}
