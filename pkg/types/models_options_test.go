package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateOptions tests the GenerateOptions struct
func TestGenerateOptions(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var options GenerateOptions
		assert.Empty(t, options.Prompt)
		assert.Empty(t, options.Context)
		assert.Empty(t, options.OutputFile)
		assert.Nil(t, options.Language)
		assert.Empty(t, options.ContextFiles)
		assert.Empty(t, options.Messages)
		assert.Equal(t, 0, options.MaxTokens)
		assert.Equal(t, 0.0, options.Temperature)
		assert.Empty(t, options.Stop)
		assert.False(t, options.Stream)
		assert.Empty(t, options.Tools)
		assert.Empty(t, options.ResponseFormat)
		assert.Equal(t, time.Duration(0), options.Timeout)
		assert.Nil(t, options.Metadata)
	})

	t.Run("FullOptions", func(t *testing.T) {
		language := "go"
		messages := []ChatMessage{
			{
				Role:    "user",
				Content: "Write a hello world function",
			},
		}
		tools := []Tool{
			{
				Name:        "code_executor",
				Description: "Execute code",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		}
		metadata := map[string]interface{}{
			"request_id": "req_123",
			"priority":   "high",
		}

		options := GenerateOptions{
			Prompt:         "Create a function that returns a greeting",
			Context:        "This is for a web application",
			OutputFile:     "greeting.go",
			Language:       &language,
			ContextFiles:   []string{"utils.go", "config.go"},
			Messages:       messages,
			MaxTokens:      1000,
			Temperature:    0.7,
			Stop:           []string{"\n", "```"},
			Stream:         true,
			Tools:          tools,
			ResponseFormat: "json",
			Timeout:        30 * time.Second,
			Metadata:       metadata,
		}

		assert.Equal(t, "Create a function that returns a greeting", options.Prompt)
		assert.Equal(t, "This is for a web application", options.Context)
		assert.Equal(t, "greeting.go", options.OutputFile)
		assert.Equal(t, &language, options.Language)
		assert.Equal(t, []string{"utils.go", "config.go"}, options.ContextFiles)
		assert.Equal(t, messages, options.Messages)
		assert.Equal(t, 1000, options.MaxTokens)
		assert.Equal(t, 0.7, options.Temperature)
		assert.Equal(t, []string{"\n", "```"}, options.Stop)
		assert.True(t, options.Stream)
		assert.Equal(t, tools, options.Tools)
		assert.Equal(t, "json", options.ResponseFormat)
		assert.Equal(t, 30*time.Second, options.Timeout)
		assert.Equal(t, metadata, options.Metadata)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		language := "python"
		options := GenerateOptions{
			Prompt:    "Generate a simple API",
			Language:  &language,
			MaxTokens: 500,
			Stream:    false,
		}

		data, err := json.Marshal(options)
		require.NoError(t, err)

		var result GenerateOptions
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, options.Prompt, result.Prompt)
		require.NotNil(t, result.Language)
		assert.Equal(t, language, *result.Language)
		assert.Equal(t, options.MaxTokens, result.MaxTokens)
		assert.Equal(t, options.Stream, result.Stream)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			options GenerateOptions
			valid   bool
		}{
			{
				name:    "EmptyOptions",
				options: GenerateOptions{},
				valid:   false,
			},
			{
				name: "OnlyPrompt",
				options: GenerateOptions{
					Prompt: "Generate code",
				},
				valid: true,
			},
			{
				name: "OnlyMessages",
				options: GenerateOptions{
					Messages: []ChatMessage{
						{
							Role:    "user",
							Content: "Hello",
						},
					},
				},
				valid: true,
			},
			{
				name: "NegativeMaxTokens",
				options: GenerateOptions{
					Prompt:    "Test",
					MaxTokens: -100,
				},
				valid: false,
			},
			{
				name: "InvalidTemperature",
				options: GenerateOptions{
					Prompt:      "Test",
					Temperature: 2.5, // Should be between 0 and 2
				},
				valid: false,
			},
			{
				name: "ValidCompleteOptions",
				options: GenerateOptions{
					Prompt:      "Write a function",
					MaxTokens:   1000,
					Temperature: 0.7,
					Stream:      false,
				},
				valid: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid := tt.options.Validate()
				assert.Equal(t, tt.valid, valid)
			})
		}
	})

	t.Run("WithDefaults", func(t *testing.T) {
		options := GenerateOptions{
			Prompt: "Generate code",
		}

		options.ApplyDefaults()
		assert.Equal(t, 0, options.MaxTokens)     // No default for MaxTokens
		assert.Equal(t, 0.0, options.Temperature) // No default for Temperature
		assert.False(t, options.Stream)           // Default is false
	})
}
