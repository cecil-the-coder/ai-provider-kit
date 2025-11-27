package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewOpenAIProvider tests the creation of a new OpenAI provider
func TestNewOpenAIProvider(t *testing.T) {
	t.Run("ValidConfiguration", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:                 types.ProviderTypeOpenAI,
			Name:                 "test-openai",
			APIKey:               "sk-test-key",
			BaseURL:              "https://api.openai.com/v1",
			DefaultModel:         "gpt-4",
			SupportsResponsesAPI: true,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
		}

		provider := NewOpenAIProvider(config)

		assert.Equal(t, "OpenAI", provider.Name())
		assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
		assert.Equal(t, "OpenAI - GPT models with native API access", provider.Description())
		assert.True(t, provider.IsAuthenticated())
		assert.Equal(t, "https://api.openai.com/v1", provider.baseURL)
		assert.True(t, provider.useResponsesAPI)
		assert.NotNil(t, provider.BaseProvider)
	})

	t.Run("DefaultBaseURL", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}

		provider := NewOpenAIProvider(config)

		assert.Equal(t, "https://api.openai.com/v1", provider.baseURL)
	})

	t.Run("APIKeyFromEnvironment", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:      types.ProviderTypeOpenAI,
			APIKey:    "",
			APIKeyEnv: "sk-env-key",
		}

		provider := NewOpenAIProvider(config)

		// Verify provider has API key (test behavior, not implementation)
		assert.False(t, provider.IsAuthenticated()) // Empty APIKeyEnv doesn't set key directly
	})

	t.Run("EmptyAPIKey", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}

		provider := NewOpenAIProvider(config)

		assert.False(t, provider.IsAuthenticated())
	})
}

// TestOpenAIProvider_GetModels tests the GetModels method
func TestOpenAIProvider_GetModels(t *testing.T) {
	t.Run("AuthenticatedProvider", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		models, err := provider.GetModels(context.Background())

		assert.NoError(t, err)
		assert.NotEmpty(t, models)

		// Verify expected models are present
		modelIDs := make(map[string]bool)
		for _, model := range models {
			modelIDs[model.ID] = true
			assert.Equal(t, types.ProviderTypeOpenAI, model.Provider)
			assert.NotEmpty(t, model.Name)
			assert.NotZero(t, model.MaxTokens)
		}

		expectedModels := []string{
			"gpt-4o", "gpt-4o-mini", "gpt-4-turbo",
			"gpt-3.5-turbo",
		}

		for _, modelID := range expectedModels {
			assert.True(t, modelIDs[modelID], "Expected model %s to be present", modelID)
		}
	})

	t.Run("UnauthenticatedProvider", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		models, err := provider.GetModels(context.Background())

		// When unauthenticated, GetModels should return fallback models, not an error
		assert.NoError(t, err)
		assert.NotEmpty(t, models)

		// Verify fallback models are present
		modelIDs := make(map[string]bool)
		for _, model := range models {
			modelIDs[model.ID] = true
		}
		assert.True(t, modelIDs["gpt-4o"], "Expected fallback model gpt-4o to be present")
		assert.True(t, modelIDs["gpt-3.5-turbo"], "Expected fallback model gpt-3.5-turbo to be present")
	})

	t.Run("ModelCapabilities", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		models, err := provider.GetModels(context.Background())
		require.NoError(t, err)

		// Find GPT-4o model and verify its capabilities
		var gpt4o *types.Model
		for _, model := range models {
			if model.ID == "gpt-4o" {
				gpt4o = &model
				break
			}
		}

		require.NotNil(t, gpt4o, "GPT-4o model should be present")
		assert.Equal(t, "GPT-4o", gpt4o.Name)
		assert.Equal(t, 128000, gpt4o.MaxTokens)
		assert.True(t, gpt4o.SupportsStreaming)
		assert.True(t, gpt4o.SupportsToolCalling)
		assert.Contains(t, gpt4o.Description, "OpenAI's latest high-intelligence flagship model")
	})
}

// TestOpenAIProvider_GetDefaultModel tests the GetDefaultModel method
func TestOpenAIProvider_GetDefaultModel(t *testing.T) {
	t.Run("WithDefaultModelInConfig", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			DefaultModel: "gpt-3.5-turbo",
		}
		provider := NewOpenAIProvider(config)

		assert.Equal(t, "gpt-3.5-turbo", provider.GetDefaultModel())
	})

	t.Run("WithoutDefaultModelInConfig", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		assert.Equal(t, "gpt-4o", provider.GetDefaultModel())
	})
}

// TestOpenAIProvider_Authenticate tests the Authenticate method
func TestOpenAIProvider_Authenticate(t *testing.T) {
	t.Run("APIKeyAuthentication", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method:       types.AuthMethodAPIKey,
			APIKey:       "sk-new-key",
			BaseURL:      "https://custom.openai.com/v1",
			DefaultModel: "gpt-4",
		}

		err := provider.Authenticate(context.Background(), authConfig)

		assert.NoError(t, err)
		assert.True(t, provider.IsAuthenticated())
		assert.Equal(t, "https://custom.openai.com/v1", provider.baseURL)
		assert.Equal(t, "gpt-4", provider.GetDefaultModel())
	})

	t.Run("UnsupportedAuthMethod", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodOAuth,
		}

		err := provider.Authenticate(context.Background(), authConfig)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OpenAI only supports API key authentication")
	})

	t.Run("BearerTokenAuthentication", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodBearerToken,
			APIKey: "bearer-token",
		}

		err := provider.Authenticate(context.Background(), authConfig)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OpenAI only supports API key authentication")
	})
}

// TestOpenAIProvider_IsAuthenticated tests the IsAuthenticated method
func TestOpenAIProvider_IsAuthenticated(t *testing.T) {
	t.Run("Authenticated", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		assert.True(t, provider.IsAuthenticated())
	})

	t.Run("NotAuthenticated", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		assert.False(t, provider.IsAuthenticated())
	})
}

// TestOpenAIProvider_Logout tests the Logout method
func TestOpenAIProvider_Logout(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	assert.True(t, provider.IsAuthenticated())

	err := provider.Logout(context.Background())

	assert.NoError(t, err)
	assert.False(t, provider.IsAuthenticated())
}

// TestOpenAIProvider_Configure tests the Configure method
func TestOpenAIProvider_Configure(t *testing.T) {
	t.Run("ValidConfiguration", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		newConfig := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			APIKey:       "sk-new-key",
			BaseURL:      "https://api.openai.com/v1",
			DefaultModel: "gpt-4",
		}

		err := provider.Configure(newConfig)

		assert.NoError(t, err)
		assert.True(t, provider.IsAuthenticated())
		assert.Equal(t, "gpt-4", provider.GetDefaultModel())
	})

	t.Run("InvalidProviderType", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		invalidConfig := types.ProviderConfig{
			Type:   types.ProviderTypeAnthropic,
			APIKey: "sk-test-key",
		}

		err := provider.Configure(invalidConfig)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid provider type for OpenAI")
	})

	t.Run("MissingAPIKey", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		// Empty API key is allowed (treated as logout)
		invalidConfig := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}

		err := provider.Configure(invalidConfig)

		assert.NoError(t, err)                      // Empty API key is allowed for logout
		assert.False(t, provider.IsAuthenticated()) // Should be cleared
	})
}

// TestOpenAIProvider_SupportsToolCalling tests the SupportsToolCalling method
func TestOpenAIProvider_SupportsToolCalling(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewOpenAIProvider(config)

	assert.True(t, provider.SupportsToolCalling())
}

// TestOpenAIProvider_SupportsStreaming tests the SupportsStreaming method
func TestOpenAIProvider_SupportsStreaming(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewOpenAIProvider(config)

	assert.True(t, provider.SupportsStreaming())
}

// TestOpenAIProvider_SupportsResponsesAPI tests the SupportsResponsesAPI method
func TestOpenAIProvider_SupportsResponsesAPI(t *testing.T) {
	t.Run("ResponsesAPIEnabled", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:                 types.ProviderTypeOpenAI,
			SupportsResponsesAPI: true,
		}
		provider := NewOpenAIProvider(config)

		assert.True(t, provider.SupportsResponsesAPI())
	})

	t.Run("ResponsesAPIDisabled", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:                 types.ProviderTypeOpenAI,
			SupportsResponsesAPI: false,
		}
		provider := NewOpenAIProvider(config)

		assert.False(t, provider.SupportsResponsesAPI())
	})
}

// TestOpenAIProvider_GetToolFormat tests the GetToolFormat method
func TestOpenAIProvider_GetToolFormat(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewOpenAIProvider(config)

	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())
}

// TestOpenAIProvider_InvokeServerTool tests the InvokeServerTool method
func TestOpenAIProvider_InvokeServerTool(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewOpenAIProvider(config)

	result, err := provider.InvokeServerTool(context.Background(), "test-tool", map[string]interface{}{
		"param": "value",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool invocation not yet implemented")
	assert.Nil(t, result)
}

// TestOpenAIProvider_GenerateChatCompletion tests the GenerateChatCompletion method
func TestOpenAIProvider_GenerateChatCompletion(t *testing.T) {
	t.Run("BasicGeneration", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: "https://api.openai.com/v1",
		}
		provider := NewOpenAIProvider(config)

		options := types.GenerateOptions{
			Prompt:      "Hello, world!",
			MaxTokens:   100,
			Temperature: 0.7,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)

		// The real API call will fail with test credentials, which is expected
		// In a real scenario, you'd use a mock server or valid credentials
		if err != nil {
			assert.Contains(t, err.Error(), "invalid OpenAI")
			return
		}
		assert.NotNil(t, stream)

		// Test the mock stream
		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.Contains(t, chunk.Content, "Hello, world!")
		assert.True(t, chunk.Done)

		// Test closing the stream
		err = stream.Close()
		assert.NoError(t, err)
	})

	t.Run("WithMessages", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "What is the capital of France?"},
			},
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)

		// The real API call will fail with test credentials, which is expected
		if err != nil {
			assert.Contains(t, err.Error(), "invalid OpenAI")
			return
		}
		assert.NotNil(t, stream)

		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.Contains(t, chunk.Content, "What is the capital of France?")
		assert.True(t, chunk.Done)

		_ = stream.Close()
	})

	t.Run("WithTools", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		tools := []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get the current weather in a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state, e.g. San Francisco, CA",
						},
					},
					"required": []string{"location"},
				},
			},
		}

		options := types.GenerateOptions{
			Prompt: "What's the weather like in New York?",
			Tools:  tools,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)

		// The real API call will fail with test credentials, which is expected
		if err != nil {
			assert.Contains(t, err.Error(), "invalid OpenAI")
			return
		}
		assert.NotNil(t, stream)

		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.Contains(t, chunk.Content, "What's the weather like in New York?")
		assert.True(t, chunk.Done)

		_ = stream.Close()
	})
}

// TestOpenAIProvider_Integration tests integration scenarios
func TestOpenAIProvider_Integration(t *testing.T) {
	t.Run("FullLifecycle", func(t *testing.T) {
		// Create provider
		config := types.ProviderConfig{
			Type:                 types.ProviderTypeOpenAI,
			APIKey:               "sk-test-key",
			BaseURL:              "https://api.openai.com/v1",
			DefaultModel:         "gpt-4",
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: true,
		}
		provider := NewOpenAIProvider(config)

		// Verify initial state
		assert.Equal(t, "OpenAI", provider.Name())
		assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
		assert.True(t, provider.IsAuthenticated())
		assert.Equal(t, "gpt-4", provider.GetDefaultModel())

		// Get models
		models, err := provider.GetModels(context.Background())
		assert.NoError(t, err)
		assert.NotEmpty(t, models)

		// Generate completion
		options := types.GenerateOptions{
			Prompt: "Test prompt",
		}
		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		// The real API call will fail with test credentials, which is expected
		if err != nil {
			assert.Contains(t, err.Error(), "invalid OpenAI")
			// Skip the remaining integration tests since they rely on a successful completion
			return
		}
		assert.NotNil(t, stream)
		_ = stream.Close()

		// Check capabilities
		assert.True(t, provider.SupportsToolCalling())
		assert.True(t, provider.SupportsStreaming())
		assert.True(t, provider.SupportsResponsesAPI())
		assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())

		// Health check
		err = provider.HealthCheck(context.Background())
		assert.NoError(t, err)

		// Get metrics
		metrics := provider.GetMetrics()
		assert.NotNil(t, metrics)

		// Logout
		err = provider.Logout(context.Background())
		assert.NoError(t, err)
		assert.False(t, provider.IsAuthenticated())
	})

	t.Run("ConfigurationUpdate", func(t *testing.T) {
		// Create initial provider
		config := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			APIKey:       "sk-initial-key",
			DefaultModel: "gpt-3.5-turbo",
		}
		provider := NewOpenAIProvider(config)

		// Update configuration
		newConfig := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			APIKey:       "sk-updated-key",
			BaseURL:      "https://custom.openai.com/v1",
			DefaultModel: "gpt-4",
		}

		err := provider.Configure(newConfig)
		assert.NoError(t, err)

		// Verify updates
		assert.True(t, provider.IsAuthenticated())
		assert.Equal(t, "https://custom.openai.com/v1", provider.baseURL)
		assert.Equal(t, "gpt-4", provider.GetDefaultModel())
	})
}

// TestOpenAIProvider_ErrorHandling tests error handling scenarios
func TestOpenAIProvider_ErrorHandling(t *testing.T) {
	t.Run("InvalidAuthMethod", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodOAuth,
		}

		err := provider.Authenticate(context.Background(), authConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OpenAI only supports API key authentication")
	})

	t.Run("InvalidProviderType", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		invalidConfig := types.ProviderConfig{
			Type:   types.ProviderTypeAnthropic,
			APIKey: "sk-test-key",
		}

		err := provider.Configure(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid provider type for OpenAI")
	})

	t.Run("MissingAPIKey", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		// Try to get models without API key - should return fallback models
		models, err := provider.GetModels(context.Background())
		assert.NoError(t, err)
		assert.NotEmpty(t, models) // Falls back to static models

		// Try to configure without API key (should be allowed for logout)
		invalidConfig := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}

		err = provider.Configure(invalidConfig)
		assert.NoError(t, err) // Empty API key is allowed for logout
		assert.False(t, provider.IsAuthenticated())
	})
}

// TestOpenAIProvider_ConcurrentAccess tests concurrent access to provider methods
func TestOpenAIProvider_ConcurrentAccess(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent model access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			models, err := provider.GetModels(context.Background())
			assert.NoError(t, err)
			assert.NotEmpty(t, models)
		}()
	}

	// Test concurrent configuration access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = provider.GetConfig()
			_ = provider.IsAuthenticated()
			_ = provider.GetDefaultModel()
		}()
	}

	// Test concurrent completion generation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			options := types.GenerateOptions{
				Prompt: fmt.Sprintf("Concurrent test %d", index),
			}
			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			// API calls with test credentials will fail, which is expected
			if err == nil && stream != nil {
				_ = stream.Close()
			}
		}(i)
	}

	wg.Wait()
}

// TestOpenAIProvider_MockStream tests the MockStream implementation
func TestOpenAIProvider_MockStream(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	options := types.GenerateOptions{
		Prompt: "Test prompt for mock stream",
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)

	// The real API call will fail with test credentials, which is expected
	if err != nil {
		assert.Contains(t, err.Error(), "invalid OpenAI")
		return
	}
	require.NotNil(t, stream)

	// Test reading chunks
	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.Contains(t, chunk.Content, "Test prompt for mock stream")
	assert.True(t, chunk.Done)

	// Test stream exhaustion
	chunk, err = stream.Next()
	assert.NoError(t, err)
	assert.Empty(t, chunk.Content)

	// Test closing and reusing
	err = stream.Close()
	assert.NoError(t, err)

	// Should be able to read from beginning again
	chunk, err = stream.Next()
	assert.NoError(t, err)
	assert.Contains(t, chunk.Content, "Test prompt for mock stream")
}

// TestOpenAIProvider_HTTPClient tests HTTP client configuration
func TestOpenAIProvider_HTTPClient(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewOpenAIProvider(config)

	// Verify the provider was created successfully
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.BaseProvider)
}

// BenchmarkOpenAIProvider_Creation benchmarks provider creation
func BenchmarkOpenAIProvider_Creation(b *testing.B) {
	config := types.ProviderConfig{
		Type:                 types.ProviderTypeOpenAI,
		APIKey:               "sk-benchmark-key",
		BaseURL:              "https://api.openai.com/v1",
		DefaultModel:         "gpt-4",
		SupportsStreaming:    true,
		SupportsToolCalling:  true,
		SupportsResponsesAPI: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewOpenAIProvider(config)
	}
}

// BenchmarkOpenAIProvider_GetModels benchmarks model retrieval
func BenchmarkOpenAIProvider_GetModels(b *testing.B) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "", // Use empty API key to avoid real API calls
	}
	provider := NewOpenAIProvider(config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.GetModels(ctx)
		// Handle expected errors gracefully in benchmarks
		if err != nil {
			if strings.Contains(err.Error(), "no OpenAI API key configured") {
				continue
			}
			b.Fatal(err)
		}
	}
}

// BenchmarkOpenAIProvider_GenerateChatCompletion benchmarks chat completion generation
func BenchmarkOpenAIProvider_GenerateChatCompletion(b *testing.B) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "", // Use empty API key to avoid real API calls
	}
	provider := NewOpenAIProvider(config)
	options := types.GenerateOptions{
		Prompt: "benchmark prompt",
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := provider.GenerateChatCompletion(ctx, options)
		// Handle expected errors gracefully in benchmarks
		if err != nil {
			if strings.Contains(err.Error(), "no API keys configured") {
				continue
			}
			b.Fatal(err)
		}
		if stream != nil {
			_ = stream.Close()
		}
	}
}

// BenchmarkOpenAIProvider_Authentication benchmarks authentication operations
func BenchmarkOpenAIProvider_Authentication(b *testing.B) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}
	provider := NewOpenAIProvider(config)
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "sk-local-benchmark-key", // Use fake key but avoid network calls
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark only the authentication method call, not the actual validation
		_ = provider.Authenticate(ctx, authConfig)
		_ = provider.IsAuthenticated()
	}
}

// TestOpenAIProvider_RealAPI tests with a mock HTTP server to simulate real API calls
func TestOpenAIProvider_RealAPI(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		authHeader := r.Header.Get("Authorization")
		expectedAuth := "Bearer sk-test-key"
		if authHeader != expectedAuth {
			http.Error(w, "Invalid authorization", http.StatusUnauthorized)
			return
		}

		// Verify content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			http.Error(w, "Invalid content type", http.StatusBadRequest)
			return
		}

		// Verify URL path
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		// Parse request to verify it's valid JSON
		var request map[string]interface{}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Return mock response
		response := map[string]interface{}{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-4",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "This is a mock response from the test server",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 15,
				"total_tokens":      25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	t.Run("MockServerInteraction", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}

		provider := NewOpenAIProvider(config)

		// Test that the provider was created with the mock server URL
		assert.Equal(t, server.URL, provider.baseURL)

		// Test basic provider methods
		assert.Equal(t, "OpenAI", provider.Name())
		assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
		assert.True(t, provider.IsAuthenticated())
	})
}

// TestOpenAIProvider_ConfigurationValidation tests comprehensive configuration validation
func TestOpenAIProvider_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      types.ProviderConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "ValidMinimalConfig",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				APIKey: "sk-test-key",
			},
			expectError: false,
		},
		{
			name: "ValidFullConfig",
			config: types.ProviderConfig{
				Type:                 types.ProviderTypeOpenAI,
				Name:                 "test-openai",
				APIKey:               "sk-test-key",
				BaseURL:              "https://api.openai.com/v1",
				DefaultModel:         "gpt-4",
				SupportsStreaming:    true,
				SupportsToolCalling:  true,
				SupportsResponsesAPI: false,
				MaxTokens:            4096,
				Timeout:              30 * time.Second,
				ToolFormat:           types.ToolFormatOpenAI,
			},
			expectError: false,
		},
		{
			name: "InvalidType",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeAnthropic,
				APIKey: "sk-test-key",
			},
			expectError: true,
			errorMsg:    "invalid provider type for OpenAI",
		},
		{
			name: "MissingAPIKey",
			config: types.ProviderConfig{
				Type: types.ProviderTypeOpenAI,
			},
			expectError: false, // Empty API key is allowed (logout scenario)
		},
		{
			name: "EmptyAPIKey",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				APIKey: "",
			},
			expectError: false, // Empty API key is allowed (logout scenario)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenAIProvider(types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				APIKey: "initial-key", // Start with valid config
			})

			err := provider.Configure(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify the configuration was applied (for non-empty API key)
				if tt.config.APIKey != "" {
					assert.True(t, provider.IsAuthenticated())
				}
				if tt.config.BaseURL != "" {
					assert.Equal(t, tt.config.BaseURL, provider.baseURL)
				}
			}
		})
	}
}

// TestOpenAIProvider_ModelConsistency tests model consistency across different methods
func TestOpenAIProvider_ModelConsistency(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	// Get models
	models, err := provider.GetModels(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, models)

	// Get default model
	defaultModel := provider.GetDefaultModel()
	assert.NotEmpty(t, defaultModel)

	// Verify default model is in the models list
	defaultModelExists := false
	for _, model := range models {
		if model.ID == defaultModel {
			defaultModelExists = true
			break
		}
	}
	assert.True(t, defaultModelExists, "Default model should be in the models list")

	// Test with custom default model in config
	config.DefaultModel = "gpt-3.5-turbo"
	providerWithCustomDefault := NewOpenAIProvider(config)
	customDefault := providerWithCustomDefault.GetDefaultModel()
	assert.Equal(t, "gpt-3.5-turbo", customDefault)
}

// TestOpenAIProvider_ToolCalling tests tool calling functionality
func TestOpenAIProvider_ToolCalling(t *testing.T) {
	t.Run("ToolsConvertedInRequest", func(t *testing.T) {
		provider := createTestProvider(t)
		tools := []types.Tool{createTestTool()}
		testToolConversionInRequest(t, provider, tools, "What's the weather like?")
	})

	t.Run("ToolCallsConvertedInMessages", func(t *testing.T) {
		provider := createTestProvider(t)
		toolCalls := []types.ToolCall{createTestToolCall()}
		testToolCallsInMessages(t, provider, toolCalls)
	})

	t.Run("ToolResponsesIncluded", func(t *testing.T) {
		provider := createTestProvider(t)
		testToolResponses(t, provider)
	})

	t.Run("MockServerWithToolCalls", func(t *testing.T) {
		server := createMockToolCallServer(t)
		defer server.Close()

		provider := createProviderWithMockServer(t, server)

		tools := []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get the current weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		}

		options := types.GenerateOptions{
			Prompt: "What's the weather in SF?",
			Tools:  tools,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		require.NotNil(t, stream)

		// Read the response
		chunk, err := stream.Next()
		assert.NoError(t, err)
		assert.True(t, chunk.Done)

		// Verify tool calls are present
		testToolCallResponse(t, chunk)

		_ = stream.Close()
	})

	t.Run("StreamingWithToolCalls", func(t *testing.T) {
		server := createMockStreamingToolCallServer(t)
		defer server.Close()

		provider := createProviderWithMockServer(t, server)

		options := types.GenerateOptions{
			Prompt: "What's the weather?",
			Stream: true,
			Tools:  []types.Tool{createSimpleTestTool()},
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		require.NotNil(t, stream)

		testStreamingToolCalls(t, stream)
	})
}

// TestOpenAIProvider_ToolConversionHelpers tests the tool conversion helper functions
func TestOpenAIProvider_ToolConversionHelpers(t *testing.T) {
	t.Run("ConvertToOpenAITools", func(t *testing.T) {
		tools := []types.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"param1": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		}

		openaiTools := convertToOpenAITools(tools)

		assert.Len(t, openaiTools, 1)
		assert.Equal(t, "function", openaiTools[0].Type)
		assert.Equal(t, "test_tool", openaiTools[0].Function.Name)
		assert.Equal(t, "A test tool", openaiTools[0].Function.Description)
		assert.NotNil(t, openaiTools[0].Function.Parameters)
	})

	t.Run("ConvertOpenAIToolCallsToUniversal", func(t *testing.T) {
		openaiToolCalls := []OpenAIToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: OpenAIToolCallFunction{
					Name:      "test_function",
					Arguments: `{"arg":"value"}`,
				},
			},
		}

		universalToolCalls := convertOpenAIToolCallsToUniversal(openaiToolCalls)

		assert.Len(t, universalToolCalls, 1)
		assert.Equal(t, "call_123", universalToolCalls[0].ID)
		assert.Equal(t, "function", universalToolCalls[0].Type)
		assert.Equal(t, "test_function", universalToolCalls[0].Function.Name)
		assert.Equal(t, `{"arg":"value"}`, universalToolCalls[0].Function.Arguments)
	})

	t.Run("ConvertToOpenAIToolCalls", func(t *testing.T) {
		toolCalls := []types.ToolCall{
			{
				ID:   "call_456",
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      "another_function",
					Arguments: `{"key":"val"}`,
				},
			},
		}

		openaiToolCalls := convertToOpenAIToolCalls(toolCalls)

		assert.Len(t, openaiToolCalls, 1)
		assert.Equal(t, "call_456", openaiToolCalls[0].ID)
		assert.Equal(t, "function", openaiToolCalls[0].Type)
		assert.Equal(t, "another_function", openaiToolCalls[0].Function.Name)
		assert.Equal(t, `{"key":"val"}`, openaiToolCalls[0].Function.Arguments)
	})

	t.Run("RoundTripConversion", func(t *testing.T) {
		// Test that converting back and forth preserves data
		original := []types.ToolCall{
			{
				ID:   "call_round_trip",
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      "round_trip_func",
					Arguments: `{"test":"data","nested":{"key":"value"}}`,
				},
			},
		}

		// Convert to OpenAI format and back
		openaiFormat := convertToOpenAIToolCalls(original)
		backToUniversal := convertOpenAIToolCallsToUniversal(openaiFormat)

		assert.Equal(t, original, backToUniversal)
	})
}

// TestOpenAIProvider_ProviderInterface ensures OpenAIProvider implements all required interface methods
func TestOpenAIProvider_ProviderInterface(t *testing.T) {
	// This test ensures that OpenAIProvider correctly implements the Provider interface
	var _ types.Provider = NewOpenAIProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "test-key",
	})

	config := types.ProviderConfig{
		Type:                 types.ProviderTypeOpenAI,
		Name:                 "test-openai",
		APIKey:               "sk-test-key",
		BaseURL:              "https://api.openai.com/v1",
		DefaultModel:         "gpt-4",
		SupportsStreaming:    true,
		SupportsToolCalling:  true,
		SupportsResponsesAPI: true,
	}
	provider := NewOpenAIProvider(config)

	// Test all interface methods
	assert.Equal(t, "OpenAI", provider.Name())
	assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
	assert.Equal(t, "OpenAI - GPT models with native API access", provider.Description())

	// Model management
	models, err := provider.GetModels(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, models)
	assert.Equal(t, "gpt-4", provider.GetDefaultModel())

	// Authentication
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "new-key",
	}
	assert.NoError(t, provider.Authenticate(context.Background(), authConfig))
	assert.True(t, provider.IsAuthenticated())
	assert.NoError(t, provider.Logout(context.Background()))

	// Configuration
	newConfig := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "configured-key",
	}
	assert.NoError(t, provider.Configure(newConfig))
	finalConfig := provider.GetConfig()
	assert.Equal(t, newConfig.Type, finalConfig.Type)
	assert.Equal(t, newConfig.APIKey, finalConfig.APIKey)

	// Core capabilities
	options := types.GenerateOptions{Prompt: "test"}
	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	// The real API call will fail with test credentials, which is expected
	if err == nil {
		assert.NotNil(t, stream)
		_ = stream.Close()
	} else {
		assert.Contains(t, err.Error(), "invalid OpenAI")
	}

	result, err := provider.InvokeServerTool(context.Background(), "test", nil)
	assert.Error(t, err) // Expected to fail in current implementation
	assert.Nil(t, result)

	// Feature flags
	assert.True(t, provider.SupportsToolCalling())
	assert.True(t, provider.SupportsStreaming())
	assert.True(t, provider.SupportsResponsesAPI())
	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())

	// Health and metrics
	assert.NoError(t, provider.HealthCheck(context.Background()))
	metrics := provider.GetMetrics()
	assert.NotNil(t, metrics)
}
