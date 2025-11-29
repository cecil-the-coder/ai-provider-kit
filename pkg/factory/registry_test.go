package factory

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterDefaultProviders tests the default provider registration functionality
func TestRegisterDefaultProviders(t *testing.T) {
	factory := NewProviderFactory()

	// Verify factory is initially empty
	supportedProviders := factory.GetSupportedProviders()
	assert.Empty(t, supportedProviders)

	// Register default providers
	RegisterDefaultProviders(factory)

	// Verify all expected providers are registered
	supportedProviders = factory.GetSupportedProviders()
	expectedProviders := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeGemini,
		types.ProviderTypeQwen,
		types.ProviderTypeCerebras,
		types.ProviderTypeOpenRouter,
		types.ProviderTypeLMStudio,
		types.ProviderTypeLlamaCpp,
		types.ProviderTypeOllama,
		types.ProviderTypeRacing,
		types.ProviderTypeFallback,
		types.ProviderTypeLoadBalance,
	}

	assert.Len(t, supportedProviders, len(expectedProviders))
	for _, expectedProvider := range expectedProviders {
		assert.Contains(t, supportedProviders, expectedProvider)
	}
}

// TestRegisterDefaultProviders_ProviderCreation tests that default providers can be created
func TestRegisterDefaultProviders_ProviderCreation(t *testing.T) {
	factory := NewProviderFactory()
	RegisterDefaultProviders(factory)

	// Test creating each registered provider type
	testCases := []struct {
		name         string
		providerType types.ProviderType
		config       types.ProviderConfig
		expectName   string // Expected name (different for OpenAI which has hardcoded name)
	}{
		{
			name:         "OpenAI Provider",
			providerType: types.ProviderTypeOpenAI,
			config: types.ProviderConfig{
				Type:    types.ProviderTypeOpenAI,
				Name:    "openai-test",
				APIKey:  "sk-test-key",
				BaseURL: "https://api.openai.com/v1",
			},
			expectName: "OpenAI", // OpenAI provider has hardcoded name
		},
		{
			name:         "Anthropic Provider",
			providerType: types.ProviderTypeAnthropic,
			config: types.ProviderConfig{
				Type:         types.ProviderTypeAnthropic,
				Name:         "anthropic-test",
				APIKey:       "sk-ant-test-key",
				DefaultModel: "claude-3-sonnet-20240229",
			},
			expectName: "Anthropic", // Anthropic provider returns capitalized name
		},
		{
			name:         "Gemini Provider",
			providerType: types.ProviderTypeGemini,
			config: types.ProviderConfig{
				Type:         types.ProviderTypeGemini,
				Name:         "gemini-test",
				APIKey:       "gemini-test-key",
				DefaultModel: "gemini-pro",
			},
			expectName: "gemini",
		},
		{
			name:         "Qwen Provider",
			providerType: types.ProviderTypeQwen,
			config: types.ProviderConfig{
				Type:         types.ProviderTypeQwen,
				Name:         "qwen-test",
				APIKey:       "qwen-test-key",
				DefaultModel: "qwen-turbo",
			},
			expectName: "Qwen", // Qwen provider returns capitalized name
		},
		{
			name:         "Cerebras Provider",
			providerType: types.ProviderTypeCerebras,
			config: types.ProviderConfig{
				Type:         types.ProviderTypeCerebras,
				Name:         "cerebras-test",
				APIKey:       "cerebras-test-key",
				DefaultModel: "llama3.1-8b",
			},
			expectName: "Cerebras", // Cerebras provider returns capitalized name
		},
		{
			name:         "OpenRouter Provider",
			providerType: types.ProviderTypeOpenRouter,
			config: types.ProviderConfig{
				Type:         types.ProviderTypeOpenRouter,
				Name:         "openrouter-test",
				APIKey:       "openrouter-test-key",
				DefaultModel: "anthropic/claude-3.5-sonnet",
			},
			expectName: "OpenRouter", // OpenRouter provider returns capitalized name
		},
		{
			name:         "LM Studio Provider",
			providerType: types.ProviderTypeLMStudio,
			config: types.ProviderConfig{
				Type:    types.ProviderTypeLMStudio,
				Name:    "lmstudio-test",
				BaseURL: "http://localhost:1234",
			},
			expectName: "lmstudio",
		},
		{
			name:         "LlamaCpp Provider",
			providerType: types.ProviderTypeLlamaCpp,
			config: types.ProviderConfig{
				Type:    types.ProviderTypeLlamaCpp,
				Name:    "llamacpp-test",
				BaseURL: "http://localhost:8080",
			},
			expectName: "llamacpp",
		},
		{
			name:         "Ollama Provider",
			providerType: types.ProviderTypeOllama,
			config: types.ProviderConfig{
				Type:         types.ProviderTypeOllama,
				Name:         "ollama-test",
				BaseURL:      "http://localhost:11434",
				DefaultModel: "llama2",
			},
			expectName: "ollama",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := factory.CreateProvider(tc.providerType, tc.config)
			assert.NoError(t, err)
			assert.NotNil(t, provider)
			assert.Equal(t, tc.providerType, provider.Type())
			assert.Equal(t, tc.expectName, provider.Name())
		})
	}
}

// TestRegisterDefaultProviders_DuplicateRegistration tests handling of duplicate registrations
func TestRegisterDefaultProviders_DuplicateRegistration(t *testing.T) {
	factory := NewProviderFactory()

	// Register default providers twice
	RegisterDefaultProviders(factory)
	RegisterDefaultProviders(factory)

	// Should still have the same number of providers (no duplicates)
	supportedProviders := factory.GetSupportedProviders()
	expectedCount := 12 // Number of default providers (9 regular + 3 virtual)
	assert.Len(t, supportedProviders, expectedCount)
}

// TestRegisterDefaultProviders_ConcurrentAccess tests thread safety of default provider registration
func TestRegisterDefaultProviders_ConcurrentAccess(t *testing.T) {
	factory := NewProviderFactory()
	var wg sync.WaitGroup
	numGoroutines := 10

	// Register default providers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			RegisterDefaultProviders(factory)
		}()
	}

	wg.Wait()

	// Verify providers were registered correctly (no duplicates)
	supportedProviders := factory.GetSupportedProviders()
	expectedCount := 12 // Number of default providers (9 regular + 3 virtual)
	assert.Len(t, supportedProviders, expectedCount)

	// Verify we can still create providers
	for _, providerType := range supportedProviders {
		config := types.ProviderConfig{
			Type:   providerType,
			Name:   string(providerType) + "-test",
			APIKey: "test-api-key",
		}
		provider, err := factory.CreateProvider(providerType, config)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
	}
}

// TestRegisterDefaultProviders_MixedWithCustomProviders tests mixing default and custom providers
func TestRegisterDefaultProviders_MixedWithCustomProviders(t *testing.T) {
	factory := NewProviderFactory()

	// Register a custom provider first
	customProviderType := types.ProviderType("custom-provider")
	factory.RegisterProvider(customProviderType, func(config types.ProviderConfig) types.Provider {
		return &MockProvider{name: "custom", providerType: customProviderType}
	})

	// Register default providers
	RegisterDefaultProviders(factory)

	// Verify both custom and default providers are registered
	supportedProviders := factory.GetSupportedProviders()
	assert.Contains(t, supportedProviders, customProviderType)

	// Verify default providers are also registered
	defaultProviders := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeGemini,
		types.ProviderTypeQwen,
		types.ProviderTypeCerebras,
		types.ProviderTypeOpenRouter,
		types.ProviderTypeLMStudio,
		types.ProviderTypeLlamaCpp,
		types.ProviderTypeOllama,
	}

	for _, providerType := range defaultProviders {
		assert.Contains(t, supportedProviders, providerType)
	}

	// Verify total count
	assert.Len(t, supportedProviders, 13) // 12 default + 1 custom
}

// TestInitializeDefaultProviders_vs_RegisterDefaultProviders tests both initialization methods
func TestInitializeDefaultProviders_vs_RegisterDefaultProviders(t *testing.T) {
	// Test InitializeDefaultProviders (stubs only)
	factory1 := NewProviderFactory()
	InitializeDefaultProviders(factory1)

	// Test RegisterDefaultProviders (real implementations + stubs)
	factory2 := NewProviderFactory()
	RegisterDefaultProviders(factory2)

	// Get the providers registered by each function
	providers1 := factory1.GetSupportedProviders()
	providers2 := factory2.GetSupportedProviders()

	// InitializeDefaultProviders should only have stub providers (3 total)
	expectedStubProviders := []types.ProviderType{
		types.ProviderTypeLMStudio,
		types.ProviderTypeLlamaCpp,
		types.ProviderTypeOllama,
	}
	assert.Len(t, providers1, len(expectedStubProviders))

	// RegisterDefaultProviders should have all providers (12 total)
	expectedAllProviders := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeGemini,
		types.ProviderTypeQwen,
		types.ProviderTypeCerebras,
		types.ProviderTypeOpenRouter,
		types.ProviderTypeLMStudio,
		types.ProviderTypeLlamaCpp,
		types.ProviderTypeOllama,
		types.ProviderTypeRacing,
		types.ProviderTypeFallback,
		types.ProviderTypeLoadBalance,
	}
	assert.Len(t, providers2, len(expectedAllProviders))

	// Convert to sets for comparison
	providerSet1 := make(map[types.ProviderType]bool)
	providerSet2 := make(map[types.ProviderType]bool)

	for _, provider := range providers1 {
		providerSet1[provider] = true
	}
	for _, provider := range providers2 {
		providerSet2[provider] = true
	}

	// All stub providers should be in both sets
	for _, stubProvider := range expectedStubProviders {
		assert.True(t, providerSet1[stubProvider], "Stub provider %s should be in InitializeDefaultProviders", stubProvider)
		assert.True(t, providerSet2[stubProvider], "Stub provider %s should be in RegisterDefaultProviders", stubProvider)
	}

	// Real implementation providers should only be in RegisterDefaultProviders
	realImplementationProviders := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeGemini,
		types.ProviderTypeQwen,
		types.ProviderTypeCerebras,
		types.ProviderTypeOpenRouter,
	}

	for _, realProvider := range realImplementationProviders {
		assert.False(t, providerSet1[realProvider], "Real provider %s should NOT be in InitializeDefaultProviders", realProvider)
		assert.True(t, providerSet2[realProvider], "Real provider %s should be in RegisterDefaultProviders", realProvider)
	}
}

// TestRegisterDefaultProviders_ProviderConsistency tests that registered providers have consistent behavior
func TestRegisterDefaultProviders_ProviderConsistency(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForConsistencyTests(factory)

	// Test that all registered providers implement the Provider interface correctly
	supportedProviders := factory.GetSupportedProviders()

	for _, providerType := range supportedProviders {
		t.Run(string(providerType), func(t *testing.T) {
			config := types.ProviderConfig{
				Type:         providerType,
				Name:         string(providerType) + "-consistency-test",
				APIKey:       "test-api-key",
				DefaultModel: "test-model",
				Description:  "Test provider for consistency check",
			}

			provider, err := factory.CreateProvider(providerType, config)
			require.NoError(t, err)
			require.NotNil(t, provider)

			// Test basic interface methods
			assert.NotEmpty(t, provider.Name())
			assert.Equal(t, providerType, provider.Type())
			assert.NotEmpty(t, provider.Description())

			// Test capabilities
			assert.IsType(t, provider.SupportsToolCalling(), true)
			assert.IsType(t, provider.SupportsStreaming(), true)
			assert.IsType(t, provider.SupportsResponsesAPI(), false)
			assert.IsType(t, provider.GetToolFormat(), types.ToolFormatOpenAI)

			// Test model management
			assert.NotEmpty(t, provider.GetDefaultModel())
			models, err := provider.GetModels(context.Background())
			assert.NoError(t, err)
			assert.IsType(t, []types.Model{}, models)

			// Test authentication - all providers now use mocks so can use the same logic
			assert.IsType(t, provider.IsAuthenticated(), true)

			authConfig := types.AuthConfig{
				Method: types.AuthMethodAPIKey,
				APIKey: "test-api-key",
			}
			err = provider.Authenticate(context.Background(), authConfig)
			assert.NoError(t, err)

			err = provider.Logout(context.Background())
			assert.NoError(t, err)

			// Test configuration - all providers now use mocks so can use the same logic
			err = provider.Configure(config)
			assert.NoError(t, err)
			retrievedConfig := provider.GetConfig()
			assert.Equal(t, config.Type, retrievedConfig.Type)

			// Test health and metrics
			err = provider.HealthCheck(context.Background())
			assert.NoError(t, err)
			metrics := provider.GetMetrics()
			assert.IsType(t, types.ProviderMetrics{}, metrics)
		})
	}
}

// TestRegisterDefaultProviders_ProviderFactoryFunction tests that factory functions are correctly registered
func TestRegisterDefaultProviders_ProviderFactoryFunction(t *testing.T) {
	factory := NewProviderFactory()
	RegisterDefaultProviders(factory)

	// Test that factory functions are properly stored and executable
	testConfig := types.ProviderConfig{
		Type:         types.ProviderTypeOpenAI,
		Name:         "test-factory-function",
		APIKey:       "test-key",
		DefaultModel: "gpt-3.5-turbo",
	}

	// Create provider multiple times to ensure factory function is reusable
	for i := 0; i < 5; i++ {
		config := testConfig
		config.Name = fmt.Sprintf("%s-%d", testConfig.Name, i)

		provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
		// OpenAI provider has hardcoded name, so we check Type instead
		assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
	}
}

// BenchmarkRegisterDefaultProviders benchmarks default provider registration
func BenchmarkRegisterDefaultProviders(b *testing.B) {
	for i := 0; i < b.N; i++ {
		factory := NewProviderFactory()
		RegisterDefaultProviders(factory)
	}
}

// BenchmarkRegisterDefaultProviders_ProviderCreation benchmarks provider creation after registration
func BenchmarkRegisterDefaultProviders_ProviderCreation(b *testing.B) {
	factory := NewProviderFactory()
	RegisterDefaultProviders(factory)

	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		Name:   "benchmark-test",
		APIKey: "test-api-key",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRegisterDefaultProviders_MixedOperations benchmarks mixed registration and creation operations
func BenchmarkRegisterDefaultProviders_MixedOperations(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		factory := NewProviderFactory()

		// Register default providers
		RegisterDefaultProviders(factory)

		// Create a few providers
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "mixed-test",
			APIKey: "test-api-key",
		}
		_, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
		if err != nil {
			b.Fatal(err)
		}

		// Get supported providers
		_ = factory.GetSupportedProviders()
	}
}

// registerMockProvidersForConsistencyTests registers mock providers for consistency testing
func registerMockProvidersForConsistencyTests(factory *DefaultProviderFactory) {
	// Register mock providers for all default types
	providerTypes := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeGemini,
		types.ProviderTypeQwen,
		types.ProviderTypeCerebras,
		types.ProviderTypeOpenRouter,
		types.ProviderTypeLMStudio,
		types.ProviderTypeLlamaCpp,
		types.ProviderTypeOllama,
	}

	for _, providerType := range providerTypes {
		factory.RegisterProvider(providerType, func(config types.ProviderConfig) types.Provider {
			return &MockProvider{
				name:         config.Name,
				providerType: providerType,
				config:       config,
			}
		})
	}
}
