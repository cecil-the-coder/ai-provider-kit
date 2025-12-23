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

// TestIntegrationExample demonstrates a complete real-world usage example
func TestIntegrationExample(t *testing.T) {
	// This test serves as documentation and validation of a complete usage pattern

	// Step 1: Create and initialize factory
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	// Step 2: Configure multiple providers for different use cases
	providers := make(map[string]types.Provider)

	// OpenAI for general chat completion
	openaiConfig := testutil.DefaultProviderConfig(types.ProviderTypeOpenAI, "primary-openai")

	openaiProvider, err := factory.CreateProvider(types.ProviderTypeOpenAI, openaiConfig)
	require.NoError(t, err)
	providers["openai"] = openaiProvider

	// Anthropic for complex reasoning
	anthropicConfig := testutil.DefaultProviderConfig(types.ProviderTypeAnthropic, "reasoning-anthropic")

	anthropicProvider, err := factory.CreateProvider(types.ProviderTypeAnthropic, anthropicConfig)
	require.NoError(t, err)
	providers["anthropic"] = anthropicProvider

	// Step 3: Authenticate all providers using helper
	for name, provider := range providers {
		authConfig := testutil.DefaultAuthConfig(provider.GetConfig())
		err := provider.Authenticate(context.Background(), authConfig)
		assert.NoError(t, err, "Failed to authenticate %s provider", name)
		testutil.RequireProviderAuthenticated(t, provider)
	}

	// Step 4: Perform health checks using helper
	for _, provider := range providers {
		testutil.RequireProviderHealthy(t, provider, context.Background())
	}

	// Step 5: Use providers for different tasks
	ctx := context.Background()

	// General chat with OpenAI using builder
	chatOptions := testutil.NewGenerateOptionsBuilder().
		WithMessages([]types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Explain quantum computing in simple terms."},
		}).
		WithMaxTokens(500).
		WithTemperature(0.7).
		Build()

	stream, err := providers["openai"].GenerateChatCompletion(ctx, chatOptions)
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	assert.NoError(t, err)
	t.Logf("OpenAI response: %s", chunk.Content)

	// Complex reasoning with Anthropic
	reasoningOptions := testutil.NewGenerateOptionsBuilder().
		WithMessages([]types.ChatMessage{
			{Role: "system", Content: "You are a expert analyst."},
			{Role: "user", Content: "Analyze the potential impact of AI on healthcare over the next decade."},
		}).
		WithMaxTokens(1000).
		WithTemperature(0.3).
		WithStreaming().
		Build()

	stream, err = providers["anthropic"].GenerateChatCompletion(ctx, reasoningOptions)
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	// Process streaming response
	responseContent := ""
	for {
		chunk, err := stream.Next()
		if err != nil || chunk.Done {
			break
		}
		responseContent += chunk.Content
	}

	t.Logf("Anthropic streaming response length: %d characters", len(responseContent))

	// Step 6: Collect and analyze metrics
	for name, provider := range providers {
		metrics := provider.GetMetrics()
		t.Logf("Provider %s metrics:", name)
		t.Logf("  Requests: %d", metrics.RequestCount)
		t.Logf("  Success: %d", metrics.SuccessCount)
		t.Logf("  Errors: %d", metrics.ErrorCount)
		t.Logf("  Avg Latency: %v", metrics.AverageLatency)
		t.Logf("  Tokens Used: %d", metrics.TokensUsed)
	}

	// Step 7: Cleanup
	for name, provider := range providers {
		err := provider.Logout(ctx)
		assert.NoError(t, err, "Failed to logout %s provider", name)
	}

	t.Log("Complete integration example executed successfully")
}

// TestIntegrationExample_WithOAuth demonstrates OAuth configuration using helpers
func TestIntegrationExample_WithOAuth(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	// Create provider with OAuth credentials using helper
	oauthCreds := testutil.MultiOAuthTestConfig(2)
	config := testutil.ProviderConfigWithOAuth(types.ProviderTypeQwen, "oauth-test", oauthCreds)

	provider, err := factory.CreateProvider(types.ProviderTypeQwen, config)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Verify OAuth credentials are set
	retrievedConfig := provider.GetConfig()
	assert.Len(t, retrievedConfig.OAuthCredentials, 2)
	assert.Empty(t, retrievedConfig.APIKey, "API key should be empty with OAuth")
}

// TestIntegrationExample_WithBuilders demonstrates using builder patterns
func TestIntegrationExample_WithBuilders(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	provider, err := factory.CreateProvider(types.ProviderTypeOpenAI,
		testutil.DefaultProviderConfig(types.ProviderTypeOpenAI, "builder-test"))
	require.NoError(t, err)

	ctx := context.Background()
	authConfig := testutil.DefaultAuthConfig(provider.GetConfig())
	err = provider.Authenticate(ctx, authConfig)
	require.NoError(t, err)

	// Build complex chat message using builder
	messages := testutil.NewChatMessageBuilder().
		WithSystem("You are a helpful assistant.").
		WithUser("What is the capital of France?").
		Build()

	// Build generate options using builder
	options := testutil.NewGenerateOptionsBuilder().
		WithMessages(messages).
		WithMaxTokens(100).
		WithTemperature(0.7).
		WithMetadata(map[string]interface{}{
			"request_id": "test-123",
			"user_id":    "test-user",
		}).
		Build()

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.NotNil(t, chunk)

	// Verify metadata was passed through
	assert.Equal(t, "test-123", options.Metadata["request_id"])
}

// TestIntegrationExample_ProviderSetup demonstrates using ProviderTestSetup
func TestIntegrationExample_ProviderSetup(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	// Create a complete test setup using helper
	setup := testutil.NewProviderTestSetup(t, types.ProviderTypeGemini, "gemini-test").
		WithContext(context.Background()).
		WithCleanup(func() error {
			// Custom cleanup logic
			t.Log("Custom cleanup executed")
			return nil
		})

	provider, err := factory.CreateProvider(types.ProviderTypeGemini, setup.Config)
	require.NoError(t, err)
	setup.WithProvider(provider)

	// Authenticate using setup helper
	setup.AuthenticateProvider(t)

	// Verify capabilities using setup helper
	setup.VerifyProviderCapabilities(t, true, true, false)

	// Perform operations
	_, err = provider.GetModels(setup.Context)
	require.NoError(t, err)

	// Cleanup using setup helper
	setup.Cleanup(t)
}

// TestIntegrationExample_CredentialProviders demonstrates using credential providers
func TestIntegrationExample_CredentialProviders(t *testing.T) {
	// Static credential provider
	staticProvider := testutil.NewStaticCredentialProvider(
		"static-api-key",
		"https://api.example.com",
		"model-name",
	)

	assert.True(t, staticProvider.IsConfigured())
	assert.Equal(t, "static-api-key", staticProvider.GetAPIKey())

	// Rotating credential provider for failover testing
	credentials := []types.ProviderConfig{
		{APIKey: "key-1", Name: "provider-1"},
		{APIKey: "key-2", Name: "provider-2"},
		{APIKey: "key-3", Name: "provider-3"},
	}

	rotatingProvider := testutil.NewRotatingCredentialProvider(credentials)

	// Test rotation
	cred1 := rotatingProvider.GetNext()
	cred2 := rotatingProvider.GetNext()
	cred3 := rotatingProvider.GetNext()
	cred4 := rotatingProvider.GetNext() // Should wrap back to first

	assert.Equal(t, "key-1", cred1.APIKey)
	assert.Equal(t, "key-2", cred2.APIKey)
	assert.Equal(t, "key-3", cred3.APIKey)
	assert.Equal(t, "key-1", cred4.APIKey)
}

// TestIntegrationExample_WaitForCondition demonstrates using WaitForCondition
func TestIntegrationExample_WaitForCondition(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	_, err := factory.CreateProvider(types.ProviderTypeOpenAI,
		testutil.DefaultProviderConfig(types.ProviderTypeOpenAI, "wait-test"))
	require.NoError(t, err)

	// Simulate a condition that becomes true after some work
	conditionMet := false
	go func() {
		time.Sleep(50 * time.Millisecond)
		conditionMet = true
	}()

	// Wait for condition using helper
	testutil.WaitForCondition(
		t,
		func() bool { return conditionMet },
		200*time.Millisecond,
		10*time.Millisecond,
		"condition to become true",
	)

	assert.True(t, conditionMet)
}

// TestIntegrationExample_MetricsComparison demonstrates metrics comparison
func TestIntegrationExample_MetricsComparison(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	provider, err := factory.CreateProvider(types.ProviderTypeOpenAI,
		testutil.DefaultProviderConfig(types.ProviderTypeOpenAI, "metrics-test"))
	require.NoError(t, err)

	ctx := context.Background()
	authConfig := testutil.DefaultAuthConfig(provider.GetConfig())
	err = provider.Authenticate(ctx, authConfig)
	require.NoError(t, err)

	// Get initial metrics
	beforeMetrics := provider.GetMetrics()

	// Perform some operations
	options := testutil.NewGenerateOptionsBuilder().
		WithPrompt("Test prompt").
		WithMaxTokens(10).
		Build()

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	_ = stream.Close()

	// Get final metrics
	afterMetrics := provider.GetMetrics()

	// Verify metrics incremented using helper
	testutil.AssertMetricsIncremented(t, beforeMetrics, afterMetrics, 1)
}
