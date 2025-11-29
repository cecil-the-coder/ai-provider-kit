package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModelSerialization tests Model struct serialization
func TestModelSerialization(t *testing.T) {
	t.Run("FullModel", func(t *testing.T) {
		model := Model{
			ID:          "gpt-4",
			Name:        "GPT-4",
			Description: "Latest GPT-4 model",
			Provider:    ProviderTypeOpenAI,
			MaxTokens:   8192,
			Pricing: Pricing{
				InputTokenPrice:  0.03,
				OutputTokenPrice: 0.06,
				Unit:             "per 1K tokens",
			},
		}

		data, err := json.Marshal(model)
		require.NoError(t, err)

		var result Model
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, model.ID, result.ID)
		assert.Equal(t, model.Name, result.Name)
		assert.Equal(t, model.Description, result.Description)
		assert.Equal(t, model.Provider, result.Provider)
		assert.Equal(t, model.MaxTokens, result.MaxTokens)
		assert.Equal(t, model.Pricing.InputTokenPrice, result.Pricing.InputTokenPrice)
	})
}

// TestChatMessageSerialization tests ChatMessage serialization
func TestChatMessageSerialization(t *testing.T) {
	t.Run("SimpleMessage", func(t *testing.T) {
		msg := ChatMessage{
			Role:    "user",
			Content: "Hello, how are you?",
		}

		data, err := json.Marshal(msg)
		require.NoError(t, err)

		var result ChatMessage
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, msg.Role, result.Role)
		assert.Equal(t, msg.Content, result.Content)
	})

	t.Run("MessageWithToolCalls", func(t *testing.T) {
		msg := ChatMessage{
			Role:    "assistant",
			Content: "I'll search for that",
			ToolCalls: []ToolCall{
				{
					ID:   "call-123",
					Type: "function",
					Function: ToolCallFunction{
						Name:      "search",
						Arguments: `{"query":"test"}`,
					},
				},
			},
		}

		data, err := json.Marshal(msg)
		require.NoError(t, err)

		var result ChatMessage
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, msg.Role, result.Role)
		assert.Len(t, result.ToolCalls, 1)
		assert.Equal(t, "call-123", result.ToolCalls[0].ID)
	})
}

// TestToolSerialization tests Tool and related structs
func TestToolSerialization(t *testing.T) {
	t.Run("FunctionTool", func(t *testing.T) {
		tool := Tool{
			Name:        "get_weather",
			Description: "Get current weather",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]string{"type": "string"},
				},
			},
		}

		data, err := json.Marshal(tool)
		require.NoError(t, err)

		var result Tool
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, tool.Name, result.Name)
		assert.Equal(t, tool.Description, result.Description)
		assert.NotNil(t, result.InputSchema)
	})
}

// TestUsageSerialization tests Usage struct
func TestUsageSerialization(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	data, err := json.Marshal(usage)
	require.NoError(t, err)

	var result Usage
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, usage.PromptTokens, result.PromptTokens)
	assert.Equal(t, usage.CompletionTokens, result.CompletionTokens)
	assert.Equal(t, usage.TotalTokens, result.TotalTokens)
}

// TestChatCompletionChunkSerialization tests ChatCompletionChunk
func TestChatCompletionChunkSerialization(t *testing.T) {
	chunk := ChatCompletionChunk{
		ID:      "chunk-123",
		Content: "Hello",
		Done:    false,
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	data, err := json.Marshal(chunk)
	require.NoError(t, err)

	var result ChatCompletionChunk
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, chunk.ID, result.ID)
	assert.Equal(t, chunk.Content, result.Content)
	assert.Equal(t, chunk.Done, result.Done)
}

// TestGenerateOptionsSerialization tests GenerateOptions
func TestGenerateOptionsSerialization(t *testing.T) {
	options := GenerateOptions{
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
		Stop:        []string{"END"},
	}

	data, err := json.Marshal(options)
	require.NoError(t, err)

	var result GenerateOptions
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, options.Model, result.Model)
	assert.Equal(t, options.MaxTokens, result.MaxTokens)
	assert.Equal(t, options.Temperature, result.Temperature)
	assert.Equal(t, options.Stream, result.Stream)
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	t.Run("EmptyModel", func(t *testing.T) {
		var model Model
		assert.Empty(t, model.ID)
		assert.Empty(t, model.Name)
		assert.Equal(t, 0.0, model.Pricing.InputTokenPrice)
	})

	t.Run("EmptyMessage", func(t *testing.T) {
		var msg ChatMessage
		assert.Empty(t, msg.Role)
		assert.Empty(t, msg.Content)
		assert.Nil(t, msg.ToolCalls)
	})

	t.Run("EmptyUsage", func(t *testing.T) {
		var usage Usage
		assert.Equal(t, 0, usage.PromptTokens)
		assert.Equal(t, 0, usage.CompletionTokens)
		assert.Equal(t, 0, usage.TotalTokens)
	})
}

// BenchmarkJSONOperations benchmarks JSON serialization
func BenchmarkJSONOperations(b *testing.B) {
	model := Model{
		ID:          "gpt-4",
		Name:        "GPT-4",
		Description: "Latest model",
		MaxTokens:   8192,
	}

	b.Run("MarshalModel", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(model)
		}
	})

	data, _ := json.Marshal(model)
	b.Run("UnmarshalModel", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var result Model
			_ = json.Unmarshal(data, &result)
		}
	})
}
