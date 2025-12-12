package testutil

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewConfigurableMockProvider tests the creation of a mock provider
func TestNewConfigurableMockProvider(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)

	assert.Equal(t, "TestProvider", provider.Name())
	assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
	assert.True(t, provider.IsAuthenticated())
	assert.Equal(t, "mock-model", provider.GetDefaultModel())
}

// TestConfigurableMockProviderGenerateChatCompletion tests the GenerateChatCompletion method
func TestConfigurableMockProviderGenerateChatCompletion(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := context.Background()

	// Test successful generation
	stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		Model:    "mock-model",
	})

	require.NoError(t, err)
	require.NotNil(t, stream)

	// Consume the stream to get content
	var content string
	for {
		chunk, err := stream.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
		}

		if len(chunk.Choices) > 0 {
			content += chunk.Choices[0].Delta.Content
		}

		if chunk.Done {
			break
		}
	}

	assert.Equal(t, "Mock response", content)
	assert.Equal(t, 1, provider.GetGenerateChatCallCount())

	// Test error scenario
	expectedErr := errors.New("generation failed")
	provider.SetGenerateError(expectedErr)

	_, err = provider.GenerateChatCompletion(ctx, types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		Model:    "mock-model",
	})

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

// TestConfigurableMockProviderToolCalling tests the InvokeServerTool method
func TestConfigurableMockProviderToolCalling(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := context.Background()

	result, err := provider.InvokeServerTool(ctx, "test_tool", map[string]interface{}{"arg": "value"})

	require.NoError(t, err)
	require.NotNil(t, result)

	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "test_tool", resultMap["tool"])
	assert.Equal(t, "mock result", resultMap["result"])
}

// TestConfigurableMockProviderHealthCheck tests the HealthCheck method
func TestConfigurableMockProviderHealthCheck(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := context.Background()

	// Test successful health check
	err := provider.HealthCheck(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, provider.GetHealthCheckCallCount())

	// Test error scenario
	expectedErr := errors.New("health check failed")
	provider.SetHealthCheckError(expectedErr)

	err = provider.HealthCheck(ctx)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

// TestConfigurableMockProviderGetModels tests the GetModels method
func TestConfigurableMockProviderGetModels(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := context.Background()

	models, err := provider.GetModels(ctx)
	require.NoError(t, err)
	assert.Len(t, models, 1)
	assert.Equal(t, "mock-model", models[0].ID)

	// Test with custom models
	customModels := []types.Model{
		{ID: "model-1", Name: "Model 1"},
		{ID: "model-2", Name: "Model 2"},
	}
	provider.SetModels(customModels)

	models, err = provider.GetModels(ctx)
	require.NoError(t, err)
	assert.Len(t, models, 2)
}

// TestConfigurableMockStream tests the mock stream
func TestConfigurableMockStream(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: "Hello"}},
			},
			Done: false,
		},
		{
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: " world"}},
			},
			Done: true,
		},
	}

	stream := NewConfigurableMockStream(chunks)

	// Test streaming
	var content string
	for {
		chunk, err := stream.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("Unexpected error: %v", err)
		}

		if len(chunk.Choices) > 0 {
			content += chunk.Choices[0].Delta.Content
		}

		if chunk.Done {
			break
		}
	}

	assert.Equal(t, "Hello world", content)

	// Test reset
	stream.Reset()
	chunk, err := stream.Next()
	require.NoError(t, err)
	assert.Equal(t, "Hello", chunk.Choices[0].Delta.Content)
}

// TestTestFixtures tests the fixture creation
func TestTestFixtures(t *testing.T) {
	fixtures := NewTestFixtures()

	// Test provider configs
	assert.Equal(t, types.ProviderTypeOpenAI, fixtures.OpenAIConfig.Type)
	assert.Equal(t, types.ProviderTypeAnthropic, fixtures.AnthropicConfig.Type)
	assert.NotEmpty(t, fixtures.OpenAIConfig.APIKey)

	// Test messages
	assert.Equal(t, "user", fixtures.SimpleMessage.Role)
	assert.NotEmpty(t, fixtures.SimpleMessage.Content)
	assert.NotEmpty(t, fixtures.MultiTurnMessages)

	// Test tools
	assert.Equal(t, "get_weather", fixtures.WeatherTool.Name)
	assert.Equal(t, "calculate", fixtures.CalculatorTool.Name)
	assert.Len(t, fixtures.AllTools, 3)

	// Test models
	assert.NotEmpty(t, fixtures.OpenAIModels)
	assert.NotEmpty(t, fixtures.AnthropicModels)

	// Test responses
	assert.NotEmpty(t, fixtures.StreamChunks)
	assert.NotEmpty(t, fixtures.StandardResponse.Choices)
}

// TestTestFixturesHelpers tests the fixture helper methods
func TestTestFixturesHelpers(t *testing.T) {
	fixtures := NewTestFixtures()

	// Test NewProviderConfig
	config := fixtures.NewProviderConfig(types.ProviderTypeOpenAI, "custom-key")
	assert.Equal(t, "custom-key", config.APIKey)
	assert.Equal(t, types.ProviderTypeOpenAI, config.Type)

	// Test NewMessage
	msg := fixtures.NewMessage("system", "You are helpful")
	assert.Equal(t, "system", msg.Role)
	assert.Equal(t, "You are helpful", msg.Content)

	// Test NewToolCall
	toolCall := fixtures.NewToolCall("call_123", "test_func", `{"key":"value"}`)
	assert.Equal(t, "call_123", toolCall.ID)
	assert.Equal(t, "test_func", toolCall.Function.Name)

	// Test NewGenerateOptions
	opts := fixtures.NewGenerateOptions(
		[]types.ChatMessage{{Role: "user", Content: "Test"}},
		"gpt-4",
	)
	assert.Equal(t, "gpt-4", opts.Model)
	assert.Len(t, opts.Messages, 1)
}

// TestProviderMetrics tests that metrics are tracked correctly
func TestProviderMetrics(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := context.Background()

	// Initial metrics should be zero
	metrics := provider.GetMetrics()
	assert.Equal(t, int64(0), metrics.RequestCount)
	assert.Equal(t, int64(0), metrics.SuccessCount)
	assert.Equal(t, int64(0), metrics.ErrorCount)

	// Make a successful request
	stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
	})
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Check metrics updated
	metrics = provider.GetMetrics()
	assert.Equal(t, int64(1), metrics.RequestCount)
	assert.Equal(t, int64(1), metrics.SuccessCount)
	assert.Equal(t, int64(0), metrics.ErrorCount)

	// Make a failing request
	provider.SetGenerateError(errors.New("test error"))
	_, err = provider.GenerateChatCompletion(ctx, types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
	})
	require.Error(t, err)

	// Check metrics updated
	metrics = provider.GetMetrics()
	assert.Equal(t, int64(2), metrics.RequestCount)
	assert.Equal(t, int64(1), metrics.SuccessCount)
	assert.Equal(t, int64(1), metrics.ErrorCount)
}
