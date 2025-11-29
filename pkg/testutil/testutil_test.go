package testutil

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestNewConfigurableMockProvider tests the creation of a mock provider
func TestNewConfigurableMockProvider(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)

	AssertEqual(t, "TestProvider", provider.Name())
	AssertEqual(t, types.ProviderTypeOpenAI, provider.Type())
	AssertTrue(t, provider.IsAuthenticated())
	AssertEqual(t, "mock-model", provider.GetDefaultModel())
}

// TestConfigurableMockProviderGenerateChatCompletion tests the GenerateChatCompletion method
func TestConfigurableMockProviderGenerateChatCompletion(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := BackgroundContext(t)

	// Test successful generation
	stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		Model:    "mock-model",
	})

	AssertNoError(t, err)
	RequireNotNil(t, stream)

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

	AssertEqual(t, "Mock response", content)
	AssertEqual(t, 1, provider.GetGenerateChatCallCount())

	// Test error scenario
	expectedErr := errors.New("generation failed")
	provider.SetGenerateError(expectedErr)

	_, err = provider.GenerateChatCompletion(ctx, types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		Model:    "mock-model",
	})

	AssertError(t, err)
	AssertEqual(t, expectedErr, err)
}

// TestConfigurableMockProviderToolCalling tests the InvokeServerTool method
func TestConfigurableMockProviderToolCalling(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := BackgroundContext(t)

	result, err := provider.InvokeServerTool(ctx, "test_tool", map[string]interface{}{"arg": "value"})

	AssertNoError(t, err)
	RequireNotNil(t, result)

	resultMap, ok := result.(map[string]interface{})
	AssertTrue(t, ok)
	AssertEqual(t, "test_tool", resultMap["tool"])
	AssertEqual(t, "mock result", resultMap["result"])
}

// TestConfigurableMockProviderHealthCheck tests the HealthCheck method
func TestConfigurableMockProviderHealthCheck(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := BackgroundContext(t)

	// Test successful health check
	err := provider.HealthCheck(ctx)
	AssertNoError(t, err)
	AssertEqual(t, 1, provider.GetHealthCheckCallCount())

	// Test error scenario
	expectedErr := errors.New("health check failed")
	provider.SetHealthCheckError(expectedErr)

	err = provider.HealthCheck(ctx)
	AssertError(t, err)
	AssertEqual(t, expectedErr, err)
}

// TestConfigurableMockProviderGetModels tests the GetModels method
func TestConfigurableMockProviderGetModels(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := BackgroundContext(t)

	models, err := provider.GetModels(ctx)
	AssertNoError(t, err)
	AssertLen(t, models, 1)
	AssertEqual(t, "mock-model", models[0].ID)

	// Test with custom models
	customModels := []types.Model{
		{ID: "model-1", Name: "Model 1"},
		{ID: "model-2", Name: "Model 2"},
	}
	provider.SetModels(customModels)

	models, err = provider.GetModels(ctx)
	AssertNoError(t, err)
	AssertLen(t, models, 2)
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

	AssertEqual(t, "Hello world", content)

	// Test reset
	stream.Reset()
	chunk, err := stream.Next()
	AssertNoError(t, err)
	AssertEqual(t, "Hello", chunk.Choices[0].Delta.Content)
}

// TestTestFixtures tests the fixture creation
func TestTestFixtures(t *testing.T) {
	fixtures := NewTestFixtures()

	// Test provider configs
	AssertEqual(t, types.ProviderTypeOpenAI, fixtures.OpenAIConfig.Type)
	AssertEqual(t, types.ProviderTypeAnthropic, fixtures.AnthropicConfig.Type)
	AssertNotEmpty(t, fixtures.OpenAIConfig.APIKey)

	// Test messages
	AssertEqual(t, "user", fixtures.SimpleMessage.Role)
	AssertNotEmpty(t, fixtures.SimpleMessage.Content)
	AssertNotEmpty(t, fixtures.MultiTurnMessages)

	// Test tools
	AssertEqual(t, "get_weather", fixtures.WeatherTool.Name)
	AssertEqual(t, "calculate", fixtures.CalculatorTool.Name)
	AssertLen(t, fixtures.AllTools, 3)

	// Test models
	AssertNotEmpty(t, fixtures.OpenAIModels)
	AssertNotEmpty(t, fixtures.AnthropicModels)

	// Test responses
	AssertNotEmpty(t, fixtures.StreamChunks)
	AssertNotEmpty(t, fixtures.StandardResponse.Choices)
}

// TestTestFixturesHelpers tests the fixture helper methods
func TestTestFixturesHelpers(t *testing.T) {
	fixtures := NewTestFixtures()

	// Test NewProviderConfig
	config := fixtures.NewProviderConfig(types.ProviderTypeOpenAI, "custom-key")
	AssertEqual(t, "custom-key", config.APIKey)
	AssertEqual(t, types.ProviderTypeOpenAI, config.Type)

	// Test NewMessage
	msg := fixtures.NewMessage("system", "You are helpful")
	AssertEqual(t, "system", msg.Role)
	AssertEqual(t, "You are helpful", msg.Content)

	// Test NewToolCall
	toolCall := fixtures.NewToolCall("call_123", "test_func", `{"key":"value"}`)
	AssertEqual(t, "call_123", toolCall.ID)
	AssertEqual(t, "test_func", toolCall.Function.Name)

	// Test NewGenerateOptions
	opts := fixtures.NewGenerateOptions(
		[]types.ChatMessage{{Role: "user", Content: "Test"}},
		"gpt-4",
	)
	AssertEqual(t, "gpt-4", opts.Model)
	AssertLen(t, opts.Messages, 1)
}

// TestAssertions tests various assertion helpers
func TestAssertions(t *testing.T) {
	// Test AssertNoError
	AssertNoError(t, nil)

	// Test AssertError
	AssertError(t, errors.New("test error"))

	// Test AssertEqual
	AssertEqual(t, 1, 1)
	AssertEqual(t, "test", "test")

	// Test AssertNotEqual
	AssertNotEqual(t, 1, 2)

	// Test AssertTrue/False
	AssertTrue(t, true)
	AssertFalse(t, false)

	// Test AssertNil/NotNil
	AssertNil(t, nil)
	AssertNotNil(t, "not nil")

	// Test AssertEmpty/NotEmpty
	AssertEmpty(t, "")
	AssertEmpty(t, []int{})
	AssertNotEmpty(t, "not empty")
	AssertNotEmpty(t, []int{1})

	// Test AssertContains
	AssertContains(t, "hello world", "world")

	// Test AssertNotContains
	AssertNotContains(t, "hello world", "xyz")

	// Test AssertLen
	AssertLen(t, []int{1, 2, 3}, 3)

	// Test AssertStatusCode
	AssertStatusCode(t, http.StatusOK, 200)

	// Test AssertStatusOK
	AssertStatusOK(t, http.StatusOK)
}

// TestContextHelpers tests the context helper functions
func TestContextHelpers(t *testing.T) {
	// Test TestContext
	ctx, cancel := TestContext(t)
	defer cancel()
	RequireNotNil(t, ctx)

	deadline, ok := ctx.Deadline()
	AssertTrue(t, ok)
	AssertTrue(t, deadline.After(time.Now()))

	// Test ShortTestContext
	shortCtx, shortCancel := ShortTestContext(t)
	defer shortCancel()
	RequireNotNil(t, shortCtx)

	// Test LongTestContext
	longCtx, longCancel := LongTestContext(t)
	defer longCancel()
	RequireNotNil(t, longCtx)

	// Test TestContextWithTimeout
	customCtx, customCancel := TestContextWithTimeout(t, 10*time.Second)
	defer customCancel()
	RequireNotNil(t, customCtx)

	// Test TestContextWithCancel
	cancelCtx, cancelFunc := TestContextWithCancel(t)
	defer cancelFunc()
	RequireNotNil(t, cancelCtx)

	// Test BackgroundContext
	bgCtx := BackgroundContext(t)
	RequireNotNil(t, bgCtx)
	AssertEqual(t, context.Background(), bgCtx)

	// Test ContextWithValue
	valueCtx := ContextWithValue(t, "key", "value")
	RequireNotNil(t, valueCtx)
	AssertEqual(t, "value", valueCtx.Value("key"))
}

// TestProviderMetrics tests that metrics are tracked correctly
func TestProviderMetrics(t *testing.T) {
	provider := NewConfigurableMockProvider("TestProvider", types.ProviderTypeOpenAI)
	ctx := BackgroundContext(t)

	// Initial metrics should be zero
	metrics := provider.GetMetrics()
	AssertEqual(t, int64(0), metrics.RequestCount)
	AssertEqual(t, int64(0), metrics.SuccessCount)
	AssertEqual(t, int64(0), metrics.ErrorCount)

	// Make a successful request
	stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
	})
	AssertNoError(t, err)
	RequireNotNil(t, stream)

	// Check metrics updated
	metrics = provider.GetMetrics()
	AssertEqual(t, int64(1), metrics.RequestCount)
	AssertEqual(t, int64(1), metrics.SuccessCount)
	AssertEqual(t, int64(0), metrics.ErrorCount)

	// Make a failing request
	provider.SetGenerateError(errors.New("test error"))
	_, err = provider.GenerateChatCompletion(ctx, types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
	})
	AssertError(t, err)

	// Check metrics updated
	metrics = provider.GetMetrics()
	AssertEqual(t, int64(2), metrics.RequestCount)
	AssertEqual(t, int64(1), metrics.SuccessCount)
	AssertEqual(t, int64(1), metrics.ErrorCount)
}
