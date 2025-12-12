package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChatService tests the ChatService example
func TestChatService(t *testing.T) {
	t.Run("NewChatService", func(t *testing.T) {
		mock := &MockProvider{
			name:         "chat-provider",
			providerType: ProviderTypeOpenAI,
		}

		service := NewChatService(mock)
		assert.NotNil(t, service)
		assert.NotNil(t, service.provider)
	})

	t.Run("GenerateResponse", func(t *testing.T) {
		mock := &MockProvider{
			name:         "chat-provider",
			providerType: ProviderTypeOpenAI,
		}

		service := NewChatService(mock)

		ctx := context.Background()
		options := GenerateOptions{
			Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
		}

		stream, err := service.GenerateResponse(ctx, options)
		require.NoError(t, err)
		assert.NotNil(t, stream)

		// Verify we can read from stream
		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.NotNil(t, chunk)

		err = stream.Close()
		assert.NoError(t, err)
	})
}

// TestToolExecutionService tests the ToolExecutionService example
func TestToolExecutionService(t *testing.T) {
	t.Run("NewToolExecutionService", func(t *testing.T) {
		mock := &MockProvider{
			name:         "tool-provider",
			providerType: ProviderTypeOpenAI,
		}

		service := NewToolExecutionService(mock)
		assert.NotNil(t, service)
		assert.NotNil(t, service.provider)
	})

	t.Run("ExecuteTool", func(t *testing.T) {
		mock := &MockProvider{
			name:         "tool-provider",
			providerType: ProviderTypeOpenAI,
		}

		service := NewToolExecutionService(mock)

		ctx := context.Background()
		result, err := service.ExecuteTool(ctx, "test-tool", map[string]interface{}{"param": "value"})
		require.NoError(t, err)
		assert.Equal(t, "tool result", result)
	})
}

// TestProviderInfoService tests the ProviderInfoService example
func TestProviderInfoService(t *testing.T) {
	t.Run("NewProviderInfoService", func(t *testing.T) {
		service := NewProviderInfoService()
		assert.NotNil(t, service)
	})

	t.Run("AddProvider", func(t *testing.T) {
		service := NewProviderInfoService()
		mock := &MockProvider{
			name:         "info-provider",
			providerType: ProviderTypeOpenAI,
			description:  "Test provider",
		}

		service.AddProvider(mock)
		assert.Len(t, service.providers, 1)
	})

	t.Run("ListProviders", func(t *testing.T) {
		service := NewProviderInfoService()
		mock1 := &MockProvider{
			name:         "provider1",
			providerType: ProviderTypeOpenAI,
			description:  "Provider 1",
		}
		mock2 := &MockProvider{
			name:         "provider2",
			providerType: ProviderTypeAnthropic,
			description:  "Provider 2",
		}

		service.AddProvider(mock1)
		service.AddProvider(mock2)

		providers := service.ListProviders()
		assert.Len(t, providers, 2)
		assert.Equal(t, "provider1", providers[0].Name)
		assert.Equal(t, "provider2", providers[1].Name)
	})
}

// TestMultiPurposeService tests the MultiPurposeService example
func TestMultiPurposeService(t *testing.T) {
	t.Run("NewMultiPurposeService", func(t *testing.T) {
		mock := &MockProvider{
			name:         "multi-provider",
			providerType: ProviderTypeOpenAI,
			description:  "Multi-purpose provider",
		}

		service := NewMultiPurposeService(mock)
		assert.NotNil(t, service)
		assert.Equal(t, mock, service.provider)
	})

	t.Run("GetProviderInfo", func(t *testing.T) {
		mock := &MockProvider{
			name:         "test-provider",
			providerType: ProviderTypeOpenAI,
			description:  "Test provider",
		}

		service := NewMultiPurposeService(mock)
		info := service.GetProviderInfo()

		assert.Contains(t, info, "test-provider")
		assert.Contains(t, info, "openai")
		assert.Contains(t, info, "Test provider")
	})

	t.Run("SupportsTools", func(t *testing.T) {
		mock := &MockProvider{
			name:         "tool-provider",
			providerType: ProviderTypeOpenAI,
		}

		service := NewMultiPurposeService(mock)
		assert.True(t, service.SupportsTools())
	})

	t.Run("GetHealth", func(t *testing.T) {
		mock := &MockProvider{
			name:      "healthy-provider",
			isHealthy: true,
		}

		service := NewMultiPurposeService(mock)
		ctx := context.Background()

		err := service.GetHealth(ctx)
		assert.NoError(t, err)

		// Test unhealthy
		mock.isHealthy = false
		err = service.GetHealth(ctx)
		assert.Error(t, err)
	})
}

// TestProviderTypeAssertions tests that MockProvider can be cast to specific interfaces
func TestProviderTypeAssertions(t *testing.T) {
	t.Run("ModelProvider", func(t *testing.T) {
		mock := &MockProvider{
			name:         "model-provider",
			providerType: ProviderTypeOpenAI,
			models: []Model{
				{ID: "gpt-4", Name: "GPT-4"},
			},
		}

		var modelProvider ModelProvider = mock
		assert.NotNil(t, modelProvider)

		ctx := context.Background()
		models, err := modelProvider.GetModels(ctx)
		require.NoError(t, err)
		assert.Len(t, models, 1)
		assert.Equal(t, "gpt-4", models[0].ID)
	})

	t.Run("ChatProvider", func(t *testing.T) {
		mock := &MockProvider{
			name:         "chat-provider",
			providerType: ProviderTypeOpenAI,
		}

		var chatProvider ChatProvider = mock
		assert.NotNil(t, chatProvider)

		ctx := context.Background()
		options := GenerateOptions{
			Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
		}

		stream, err := chatProvider.GenerateChatCompletion(ctx, options)
		require.NoError(t, err)
		assert.NotNil(t, stream)
	})

	t.Run("HealthCheckProvider", func(t *testing.T) {
		mock := &MockProvider{
			name:      "health-provider",
			isHealthy: true,
		}

		var healthProvider HealthCheckProvider = mock
		assert.NotNil(t, healthProvider)

		ctx := context.Background()
		err := healthProvider.HealthCheck(ctx)
		assert.NoError(t, err)

		metrics := healthProvider.GetMetrics()
		assert.NotNil(t, metrics)
	})
}

// TestMockStream tests the MockStream implementation
func TestMockStream(t *testing.T) {
	t.Run("Next", func(t *testing.T) {
		stream := &MockStream{}
		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.NotNil(t, chunk)
	})

	t.Run("Close", func(t *testing.T) {
		stream := &MockStream{}
		err := stream.Close()
		assert.NoError(t, err)
	})
}

// TestAllInterfaceImplementations tests that MockProvider implements all interfaces
func TestAllInterfaceImplementations(t *testing.T) {
	mock := &MockProvider{
		name:         "complete-provider",
		providerType: ProviderTypeOpenAI,
		description:  "Complete test provider",
		models: []Model{
			{ID: "model-1", Name: "Model 1"},
		},
		isHealthy: true,
	}

	ctx := context.Background()

	t.Run("CoreProvider", func(t *testing.T) {
		var _ CoreProvider = mock
		assert.Equal(t, "complete-provider", mock.Name())
		assert.Equal(t, ProviderTypeOpenAI, mock.Type())
		assert.Equal(t, "Complete test provider", mock.Description())
	})

	t.Run("ModelProvider", func(t *testing.T) {
		var _ ModelProvider = mock
		models, err := mock.GetModels(ctx)
		require.NoError(t, err)
		assert.Len(t, models, 1)
		assert.Equal(t, "default-model", mock.GetDefaultModel())
	})

	t.Run("AuthenticatedProvider", func(t *testing.T) {
		var _ AuthenticatedProvider = mock
		err := mock.Authenticate(ctx, AuthConfig{})
		assert.NoError(t, err)
		assert.True(t, mock.IsAuthenticated())
		err = mock.Logout(ctx)
		assert.NoError(t, err)
	})

	t.Run("ConfigurableProvider", func(t *testing.T) {
		var _ ConfigurableProvider = mock
		err := mock.Configure(ProviderConfig{})
		assert.NoError(t, err)
		config := mock.GetConfig()
		assert.NotNil(t, config)
	})

	t.Run("ChatProvider", func(t *testing.T) {
		var _ ChatProvider = mock
		stream, err := mock.GenerateChatCompletion(ctx, GenerateOptions{})
		require.NoError(t, err)
		assert.NotNil(t, stream)
	})

	t.Run("ToolCallingProvider", func(t *testing.T) {
		var _ ToolCallingProvider = mock
		assert.True(t, mock.SupportsToolCalling())
		assert.Equal(t, ToolFormatOpenAI, mock.GetToolFormat())
		result, err := mock.InvokeServerTool(ctx, "test", nil)
		require.NoError(t, err)
		assert.Equal(t, "tool result", result)
	})

	t.Run("CapabilityProvider", func(t *testing.T) {
		var _ CapabilityProvider = mock
		assert.True(t, mock.SupportsStreaming())
		assert.False(t, mock.SupportsResponsesAPI())
	})

	t.Run("HealthCheckProvider", func(t *testing.T) {
		var _ HealthCheckProvider = mock
		err := mock.HealthCheck(ctx)
		assert.NoError(t, err)
		metrics := mock.GetMetrics()
		assert.NotNil(t, metrics)
	})

	t.Run("FullProvider", func(t *testing.T) {
		var _ Provider = mock
	})
}

// TestGetAllModelsHelper tests the helper function
func TestGetAllModelsHelper(t *testing.T) {
	t.Run("SingleProvider", func(t *testing.T) {
		providers := []ModelProvider{
			&MockProvider{
				models: []Model{
					{ID: "model-1", Name: "Model 1"},
					{ID: "model-2", Name: "Model 2"},
				},
			},
		}

		ctx := context.Background()
		allModels, err := GetAllModels(ctx, providers)
		require.NoError(t, err)
		assert.Len(t, allModels, 1)
		assert.Len(t, allModels[0].Models, 2)
	})

	t.Run("MultipleProviders", func(t *testing.T) {
		providers := []ModelProvider{
			&MockProvider{
				name: "provider1",
				models: []Model{
					{ID: "model-1"},
				},
			},
			&MockProvider{
				name: "provider2",
				models: []Model{
					{ID: "model-2"},
					{ID: "model-3"},
				},
			},
		}

		ctx := context.Background()
		allModels, err := GetAllModels(ctx, providers)
		require.NoError(t, err)
		assert.Len(t, allModels, 2)
		assert.Len(t, allModels[0].Models, 1)
		assert.Len(t, allModels[1].Models, 2)
	})

	t.Run("EmptyProviders", func(t *testing.T) {
		providers := []ModelProvider{}
		ctx := context.Background()

		allModels, err := GetAllModels(ctx, providers)
		require.NoError(t, err)
		assert.Len(t, allModels, 0)
	})
}

// GetAllModels is a helper function to get all models from multiple providers
func GetAllModels(ctx context.Context, providers []ModelProvider) ([]struct {
	Provider string
	Models   []Model
}, error) {
	result := make([]struct {
		Provider string
		Models   []Model
	}, 0, len(providers))

	for _, provider := range providers {
		var providerName string
		if coreProvider, ok := provider.(CoreProvider); ok {
			providerName = coreProvider.Name()
		}

		models, err := provider.GetModels(ctx)
		if err != nil {
			return nil, err
		}

		result = append(result, struct {
			Provider string
			Models   []Model
		}{
			Provider: providerName,
			Models:   models,
		})
	}

	return result, nil
}

// BenchmarkInterfaceSegregation benchmarks different interface usages
func BenchmarkInterfaceSegregation(b *testing.B) {
	mock := &MockProvider{
		name:         "bench-provider",
		providerType: ProviderTypeOpenAI,
		description:  "Benchmark provider",
	}

	b.Run("CoreProvider", func(b *testing.B) {
		var provider CoreProvider = mock
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.Name()
			_ = provider.Type()
			_ = provider.Description()
		}
	})

	b.Run("FullProvider", func(b *testing.B) {
		var provider Provider = mock
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = provider.Name()
			_, _ = provider.GetModels(ctx)
			_ = provider.SupportsToolCalling()
		}
	})
}
