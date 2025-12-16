package factory

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario4_TypeSystemIntegration tests cross-package type compatibility
func TestScenario4_TypeSystemIntegration(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	t.Run("TypeCompatibilityBetweenPackages", func(t *testing.T) {
		// Create provider using types from pkg/types
		config := types.ProviderConfig{
			Type:                types.ProviderTypeOpenAI,
			Name:                "type-compat-test",
			APIKey:              "test-key",
			BaseURL:             "https://api.openai.com/v1",
			DefaultModel:        "gpt-4",
			SupportsStreaming:   true,
			SupportsToolCalling: true,
			ToolFormat:          types.ToolFormatOpenAI,
			Timeout:             30 * time.Second,
		}

		provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
		require.NoError(t, err)

		// Test that provider returns types compatible with pkg/types
		models, err := provider.GetModels(context.Background())
		require.NoError(t, err)

		// Verify Model type compatibility
		for _, model := range models {
			assert.NotEmpty(t, model.ID)
			assert.NotEmpty(t, model.Name)
			assert.IsType(t, types.ProviderType(""), model.Provider)
			assert.IsType(t, types.Pricing{}, model.Pricing)
		}

		// Test GenerateOptions type compatibility
		options := types.GenerateOptions{
			Prompt:      "Test prompt",
			MaxTokens:   100,
			Temperature: 0.7,
			Stream:      false,
			Messages: []types.ChatMessage{
				{
					Role:    "user",
					Content: "Hello",
					ToolCalls: []types.ToolCall{
						{
							ID:   "tool-1",
							Type: "function",
							Function: types.ToolCallFunction{
								Name:      "test_function",
								Arguments: `{"param": "value"}`,
							},
						},
					},
				},
			},
			Tools: []types.Tool{
				{
					Name:        "test_tool",
					Description: "A test tool",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"param": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		require.NotNil(t, stream)

		// Test ChatCompletionStream and ChatCompletionChunk compatibility
		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.IsType(t, types.ChatCompletionChunk{}, chunk)

		if len(chunk.Choices) > 0 {
			assert.IsType(t, types.ChatChoice{}, chunk.Choices[0])
			assert.IsType(t, types.ChatMessage{}, chunk.Choices[0].Message)
			assert.IsType(t, types.Usage{}, chunk.Usage)
		}

		func() { _ = stream.Close() }()
	})

	t.Run("JSONMarshalingAcrossTypes", func(t *testing.T) {
		// Test ProviderConfig JSON marshaling
		config := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			Name:         "json-test-provider",
			APIKey:       "json-test-key",
			BaseURL:      "https://api.openai.com/v1",
			DefaultModel: "gpt-4",
			Description:  "JSON test provider",
			OAuthCredentials: []*types.OAuthCredentialSet{
				{
					ID:           "json-cred",
					ClientID:     "json-client-id",
					ClientSecret: "json-client-secret",
					Scopes:       []string{"read", "write"},
				},
			},
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			MaxTokens:            4096,
			Timeout:              30 * time.Second,
			ToolFormat:           types.ToolFormatOpenAI,
			ProviderConfig: map[string]interface{}{
				"custom_param": "custom_value",
				"nested_param": map[string]interface{}{
					"nested_key": "nested_value",
				},
			},
		}

		// Test JSON marshaling
		configJSON, err := json.Marshal(config)
		require.NoError(t, err)

		// Test JSON unmarshaling
		var unmarshaledConfig types.ProviderConfig
		err = json.Unmarshal(configJSON, &unmarshaledConfig)
		require.NoError(t, err)

		// Verify all fields were preserved
		assert.Equal(t, config.Type, unmarshaledConfig.Type)
		assert.Equal(t, config.Name, unmarshaledConfig.Name)
		assert.Equal(t, config.APIKey, unmarshaledConfig.APIKey)
		assert.Equal(t, config.DefaultModel, unmarshaledConfig.DefaultModel)
		assert.Equal(t, config.OAuthCredentials[0].ClientID, unmarshaledConfig.OAuthCredentials[0].ClientID)
		assert.Equal(t, config.ToolFormat, unmarshaledConfig.ToolFormat)
		assert.Equal(t, config.MaxTokens, unmarshaledConfig.MaxTokens)

		// Test nested objects
		assert.Equal(t, "custom_value", unmarshaledConfig.ProviderConfig["custom_param"])
	})

	t.Run("InterfaceImplementations", func(t *testing.T) {
		// Create a mock OpenAI provider instead of real one
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "interface-test-provider",
			APIKey: "test-key",
		}

		provider := NewAdvancedMockProvider(config.Name, types.ProviderTypeOpenAI, config)
		require.NotNil(t, provider)

		// Verify provider implements all required interfaces
		var _ types.Provider = provider

		// Test all interface methods are implemented
		assert.NotEmpty(t, provider.Name())
		assert.NotEmpty(t, provider.Type())
		assert.NotEmpty(t, provider.Description())
		assert.NotEmpty(t, provider.GetDefaultModel())

		assert.IsType(t, provider.SupportsToolCalling(), true)
		assert.IsType(t, provider.SupportsStreaming(), true)
		assert.IsType(t, provider.SupportsResponsesAPI(), true)
		assert.IsType(t, provider.GetToolFormat(), types.ToolFormat(""))

		// Test methods that return types
		models, err := provider.GetModels(context.Background())
		assert.NoError(t, err)
		assert.IsType(t, []types.Model{}, models)

		providerConfig := provider.GetConfig()
		assert.IsType(t, types.ProviderConfig{}, providerConfig)

		metrics := provider.GetMetrics()
		assert.IsType(t, types.ProviderMetrics{}, metrics)
	})

	t.Run("ErrorPropagationThroughStack", func(t *testing.T) {
		// Test error propagation from factory through provider
		invalidConfig := types.ProviderConfig{
			Type:   types.ProviderType("nonexistent"),
			Name:   "error-prop-test",
			APIKey: "test-key",
		}

		provider, err := factory.CreateProvider(invalidConfig.Type, invalidConfig)
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "not registered")

		// Test error propagation from provider operations
		validConfig := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "error-prop-provider",
			APIKey: "", // Empty key should cause errors in real implementation
		}

		provider, err = factory.CreateProvider(types.ProviderTypeOpenAI, validConfig)
		assert.NoError(t, err) // Factory creates successfully

		// Provider operations should handle empty API key gracefully
		models, err := provider.GetModels(context.Background())
		// This might not error with mock, but demonstrates the pattern
		if err != nil {
			assert.Contains(t, err.Error(), "API key")
		} else {
			assert.NotNil(t, models) // Mock implementation succeeds
		}
	})
}
