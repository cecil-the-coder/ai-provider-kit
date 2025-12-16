package factory

import (
	"context"
	"testing"
	"time"

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
	openaiConfig := types.ProviderConfig{
		Type:                types.ProviderTypeOpenAI,
		Name:                "primary-openai",
		APIKey:              "sk-test-key",
		BaseURL:             "https://api.openai.com/v1",
		DefaultModel:        "gpt-4",
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		MaxTokens:           4096,
		Timeout:             30 * time.Second,
		ToolFormat:          types.ToolFormatOpenAI,
	}

	openaiProvider, err := factory.CreateProvider(types.ProviderTypeOpenAI, openaiConfig)
	require.NoError(t, err)
	providers["openai"] = openaiProvider

	// Anthropic for complex reasoning
	anthropicConfig := types.ProviderConfig{
		Type:                types.ProviderTypeAnthropic,
		Name:                "reasoning-anthropic",
		APIKey:              "sk-ant-test-key",
		DefaultModel:        "claude-3-sonnet",
		SupportsStreaming:   true,
		SupportsToolCalling: true,
		MaxTokens:           8192,
		Timeout:             60 * time.Second,
		ToolFormat:          types.ToolFormatAnthropic,
	}

	anthropicProvider, err := factory.CreateProvider(types.ProviderTypeAnthropic, anthropicConfig)
	require.NoError(t, err)
	providers["anthropic"] = anthropicProvider

	// Step 3: Authenticate all providers
	for name, provider := range providers {
		err := provider.Authenticate(context.Background(), types.AuthConfig{
			Method:       types.AuthMethodAPIKey,
			APIKey:       providers[name].GetConfig().APIKey,
			BaseURL:      providers[name].GetConfig().BaseURL,
			DefaultModel: providers[name].GetConfig().DefaultModel,
		})
		assert.NoError(t, err, "Failed to authenticate %s provider", name)
		assert.True(t, provider.IsAuthenticated(), "%s provider should be authenticated", name)
	}

	// Step 4: Perform health checks
	for name, provider := range providers {
		err := provider.HealthCheck(context.Background())
		assert.NoError(t, err, "%s provider health check failed", name)
	}

	// Step 5: Use providers for different tasks
	ctx := context.Background()

	// General chat with OpenAI
	chatOptions := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Explain quantum computing in simple terms."},
		},
		MaxTokens:   500,
		Temperature: 0.7,
		Stream:      false,
	}

	stream, err := providers["openai"].GenerateChatCompletion(ctx, chatOptions)
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	assert.NoError(t, err)
	t.Logf("OpenAI response: %s", chunk.Content)

	// Complex reasoning with Anthropic
	reasoningOptions := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "system", Content: "You are a expert analyst."},
			{Role: "user", Content: "Analyze the potential impact of AI on healthcare over the next decade."},
		},
		MaxTokens:   1000,
		Temperature: 0.3,
		Stream:      true,
	}

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
