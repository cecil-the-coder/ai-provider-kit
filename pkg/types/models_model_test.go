package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModel tests the Model struct
func TestModel(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var model Model
		assert.Empty(t, model.ID)
		assert.Empty(t, model.Name)
		assert.Equal(t, ProviderType(""), model.Provider)
		assert.Empty(t, model.Description)
		assert.Equal(t, 0, model.MaxTokens)
		assert.Equal(t, 0, model.InputTokens)
		assert.Equal(t, 0, model.OutputTokens)
		assert.False(t, model.SupportsStreaming)
		assert.False(t, model.SupportsToolCalling)
		assert.False(t, model.SupportsResponsesAPI)
		assert.Empty(t, model.Capabilities)
		assert.Equal(t, Pricing{}, model.Pricing)
	})

	t.Run("FullModel", func(t *testing.T) {
		pricing := Pricing{
			InputTokenPrice:  0.001,
			OutputTokenPrice: 0.002,
			Unit:             "USD",
		}
		capabilities := []string{"text", "vision", "function_calling"}

		model := Model{
			ID:                   "gpt-4",
			Name:                 "GPT-4",
			Provider:             ProviderTypeOpenAI,
			Description:          "Large language model",
			MaxTokens:            8192,
			InputTokens:          4096,
			OutputTokens:         4096,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			Capabilities:         capabilities,
			Pricing:              pricing,
		}

		assert.Equal(t, "gpt-4", model.ID)
		assert.Equal(t, "GPT-4", model.Name)
		assert.Equal(t, ProviderTypeOpenAI, model.Provider)
		assert.Equal(t, "Large language model", model.Description)
		assert.Equal(t, 8192, model.MaxTokens)
		assert.Equal(t, 4096, model.InputTokens)
		assert.Equal(t, 4096, model.OutputTokens)
		assert.True(t, model.SupportsStreaming)
		assert.True(t, model.SupportsToolCalling)
		assert.False(t, model.SupportsResponsesAPI)
		assert.Equal(t, capabilities, model.Capabilities)
		assert.Equal(t, pricing, model.Pricing)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		model := Model{
			ID:                   "claude-3-sonnet",
			Name:                 "Claude 3 Sonnet",
			Provider:             ProviderTypeAnthropic,
			Description:          "Anthropic's Claude 3 Sonnet model",
			MaxTokens:            200000,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: true,
			Capabilities:         []string{"text", "vision", "tools"},
			Pricing: Pricing{
				InputTokenPrice:  0.003,
				OutputTokenPrice: 0.015,
				Unit:             "USD",
			},
		}

		data, err := json.Marshal(model)
		require.NoError(t, err)

		var result Model
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, model, result)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name   string
			model  Model
			valid  bool
			reason string
		}{
			{
				name:   "EmptyModel",
				model:  Model{},
				valid:  false,
				reason: "ID is required",
			},
			{
				name: "OnlyID",
				model: Model{
					ID: "test-model",
				},
				valid:  false,
				reason: "Name is required",
			},
			{
				name: "IDAndName",
				model: Model{
					ID:   "test-model",
					Name: "Test Model",
				},
				valid:  true,
				reason: "Valid model",
			},
			{
				name: "FullModel",
				model: Model{
					ID:                   "full-model",
					Name:                 "Full Model",
					Provider:             ProviderTypeOpenAI,
					Description:          "Complete model definition",
					MaxTokens:            4096,
					SupportsStreaming:    true,
					SupportsToolCalling:  true,
					SupportsResponsesAPI: true,
					Capabilities:         []string{"text"},
				},
				valid:  true,
				reason: "Valid model",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid, reason := tt.model.Validate()
				assert.Equal(t, tt.valid, valid)
				assert.Equal(t, tt.reason, reason)
			})
		}
	})

	t.Run("HasCapability", func(t *testing.T) {
		model := Model{
			Capabilities: []string{"text", "vision", "function_calling", "code_generation"},
		}

		assert.True(t, model.HasCapability("text"))
		assert.True(t, model.HasCapability("vision"))
		assert.True(t, model.HasCapability("function_calling"))
		assert.True(t, model.HasCapability("code_generation"))
		assert.False(t, model.HasCapability("audio"))
		assert.False(t, model.HasCapability("image_generation"))
	})
}

// TestCodeGenerationResult tests the CodeGenerationResult struct
func TestCodeGenerationResult(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var result CodeGenerationResult
		assert.Empty(t, result.Code)
		assert.Nil(t, result.Usage)
	})

	t.Run("WithUsage", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     100,
			CompletionTokens: 200,
			TotalTokens:      300,
		}
		result := CodeGenerationResult{
			Code:  "function hello() { return 'world'; }",
			Usage: &usage,
		}

		assert.Equal(t, "function hello() { return 'world'; }", result.Code)
		assert.Equal(t, &usage, result.Usage)
	})

	t.Run("WithoutUsage", func(t *testing.T) {
		result := CodeGenerationResult{
			Code: "console.log('Hello, world!');",
		}

		assert.Equal(t, "console.log('Hello, world!');", result.Code)
		assert.Nil(t, result.Usage)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		usage := Usage{
			PromptTokens:     50,
			CompletionTokens: 150,
			TotalTokens:      200,
		}
		result := CodeGenerationResult{
			Code:  "print('Hello, Python!')",
			Usage: &usage,
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		var parsedResult CodeGenerationResult
		err = json.Unmarshal(data, &parsedResult)
		require.NoError(t, err)
		assert.Equal(t, result.Code, parsedResult.Code)
		require.NotNil(t, parsedResult.Usage)
		assert.Equal(t, usage, *parsedResult.Usage)
	})
}
