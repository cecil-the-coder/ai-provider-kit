package factory

import (
	"context"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario1_CompleteProviderWorkflow tests the entire provider lifecycle
func TestScenario1_CompleteProviderWorkflow(t *testing.T) {
	// Create factory with mock providers instead of real implementations
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	// Register a custom provider
	customProviderType := types.ProviderType("custom-test")
	factory.RegisterProvider(customProviderType, func(config types.ProviderConfig) types.Provider {
		return &MockProvider{
			name:         config.Name,
			providerType: customProviderType,
			config:       config,
		}
	})

	// Test factory supports our custom provider
	supportedProviders := factory.GetSupportedProviders()
	assert.Contains(t, supportedProviders, customProviderType)

	// Test provider creation from configuration
	config := types.ProviderConfig{
		Type:                 types.ProviderTypeOpenAI,
		Name:                 "test-openai-provider",
		APIKey:               "test-api-key",
		BaseURL:              "https://api.openai.com/v1",
		DefaultModel:         "gpt-4",
		Description:          "Test OpenAI provider instance",
		SupportsStreaming:    true,
		SupportsToolCalling:  true,
		SupportsResponsesAPI: false,
		MaxTokens:            4096,
		Timeout:              30 * time.Second,
		ToolFormat:           types.ToolFormatOpenAI,
	}

	provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Test provider lifecycle: configuration
	err = provider.Configure(config)
	assert.NoError(t, err)
	retrievedConfig := provider.GetConfig()
	assert.Equal(t, config.Name, retrievedConfig.Name)
	assert.Equal(t, config.APIKey, retrievedConfig.APIKey)

	// Test provider lifecycle: authentication
	authConfig := types.AuthConfig{
		Method:       types.AuthMethodAPIKey,
		APIKey:       "new-api-key",
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4-turbo",
	}
	err = provider.Authenticate(context.Background(), authConfig)
	assert.NoError(t, err)
	assert.True(t, provider.IsAuthenticated())

	// Test provider lifecycle: model retrieval
	models, err := provider.GetModels(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, models)

	// Verify model structure
	for _, model := range models {
		assert.NotEmpty(t, model.ID)
		assert.NotEmpty(t, model.Name)
		assert.Equal(t, provider.Type(), model.Provider)
	}

	// Test provider lifecycle: chat completion
	options := types.GenerateOptions{
		Prompt:      "Hello, how are you?",
		MaxTokens:   100,
		Temperature: 0.7,
		Messages: []types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		},
		Stream: false,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Test streaming
	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.NotNil(t, chunk)
	err = stream.Close()
	assert.NoError(t, err)

	// Test provider lifecycle: metrics
	metrics := provider.GetMetrics()
	assert.NotNil(t, metrics)
	assert.GreaterOrEqual(t, metrics.RequestCount, int64(0))

	// Test provider lifecycle: health check
	err = provider.HealthCheck(context.Background())
	assert.NoError(t, err)

	// Test provider lifecycle: cleanup
	err = provider.Logout(context.Background())
	assert.NoError(t, err)
}
