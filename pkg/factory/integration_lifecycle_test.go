package factory

import (
	"context"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/testutil"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultTestTimeout = 30 * time.Second

// TestScenario1_CompleteProviderWorkflow tests the entire provider lifecycle
func TestScenario1_CompleteProviderWorkflow(t *testing.T) {
	// Create factory with mock providers instead of real implementations
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	// Register a custom provider
	customProviderType := types.ProviderType("custom-test")
	factory.RegisterProvider(customProviderType, func(config types.ProviderConfig) types.Provider {
		return testutil.NewMockProvider(config.Name, customProviderType)
	})

	// Test factory supports our custom provider
	supportedProviders := factory.GetSupportedProviders()
	assert.Contains(t, supportedProviders, customProviderType)

	// Use integration helpers for test setup
	setup := testutil.NewProviderTestSetup(t, types.ProviderTypeOpenAI, "test-openai-provider")

	// Create provider using the factory
	provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, setup.Config)
	require.NoError(t, err)
	require.NotNil(t, provider)
	setup.WithProvider(provider)

	// Test provider lifecycle: configuration
	err = provider.Configure(setup.Config)
	assert.NoError(t, err)
	retrievedConfig := provider.GetConfig()
	assert.Equal(t, setup.Config.Name, retrievedConfig.Name)
	assert.Equal(t, setup.Config.APIKey, retrievedConfig.APIKey)

	// Test provider lifecycle: authentication
	err = provider.Authenticate(context.Background(), setup.AuthConfig)
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
	options := testutil.NewGenerateOptionsBuilder().
		WithPrompt("Hello, how are you?").
		WithMaxTokens(100).
		WithTemperature(0.7).
		WithMessages([]types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		}).
		Build()

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

// TestScenario1_WithHelpers demonstrates using the integration helpers
func TestScenario1_WithHelpers(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	// Create provider setup using helpers
	setup := testutil.NewProviderTestSetup(t, types.ProviderTypeAnthropic, "anthropic-test")

	provider, err := factory.CreateProvider(types.ProviderTypeAnthropic, setup.Config)
	require.NoError(t, err)
	setup.WithProvider(provider)

	// Use helper to authenticate
	setup.AuthenticateProvider(t)

	// Use helper to verify capabilities
	setup.VerifyProviderCapabilities(t, true, true, false)

	// Use helper for health check
	testutil.RequireProviderHealthy(t, provider, setup.Context)

	// Use helper for authenticated check
	testutil.RequireProviderAuthenticated(t, provider)

	// Cleanup
	setup.Cleanup(t)
}

// TestScenario1_HappyPath tests the happy path scenario using integration helpers
func TestScenario1_HappyPath(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	provider, err := factory.CreateProvider(types.ProviderTypeOpenAI,
		testutil.DefaultProviderConfig(types.ProviderTypeOpenAI, "happy-path-test"))
	require.NoError(t, err)

	// Run the standard happy path scenario
	ctx := testutil.NewTestContext(t, 30*time.Second)
	testutil.HappyPathScenario(t, provider, ctx)
}

// TestScenario1_ErrorScenarios tests error scenarios using integration helpers
func TestScenario1_ErrorScenarios(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	provider, err := factory.CreateProvider(types.ProviderTypeGemini,
		testutil.DefaultProviderConfig(types.ProviderTypeGemini, "error-test"))
	require.NoError(t, err)

	ctx := testutil.NewTestContext(t, 30*time.Second)
	testutil.ErrorScenario(t, provider, ctx)
}

// TestScenario1_StreamingScenario tests streaming using integration helpers
func TestScenario1_StreamingScenario(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	provider, err := factory.CreateProvider(types.ProviderTypeQwen,
		testutil.DefaultProviderConfig(types.ProviderTypeQwen, "streaming-test"))
	require.NoError(t, err)

	ctx := testutil.NewTestContext(t, 30*time.Second)
	testutil.StreamingScenario(t, provider, ctx)
}

// TestScenario1_ConcurrentScenario tests concurrent access using integration helpers
func TestScenario1_ConcurrentScenario(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	provider, err := factory.CreateProvider(types.ProviderTypeCerebras,
		testutil.DefaultProviderConfig(types.ProviderTypeCerebras, "concurrent-test"))
	require.NoError(t, err)

	ctx := testutil.NewTestContext(t, 30*time.Second)
	testutil.ConcurrentScenario(t, provider, ctx, 10)
}
