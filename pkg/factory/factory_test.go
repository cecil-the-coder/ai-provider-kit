package factory

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/testutil"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewProviderFactory tests factory creation and initialization
func TestNewProviderFactory(t *testing.T) {
	t.Parallel()
	// Create a new factory
	factory := NewProviderFactory()

	// Verify factory is properly initialized
	assert.NotNil(t, factory)
	assert.NotNil(t, factory.providers)
	assert.Empty(t, factory.providers)
}

// TestDefaultProviderFactory_RegisterProvider tests provider registration functionality
func TestDefaultProviderFactory_RegisterProvider(t *testing.T) {
	t.Parallel()
	factory := NewProviderFactory()

	// Test registering a provider
	providerType := types.ProviderType("test-provider")
	factoryFunc := func(config types.ProviderConfig) types.Provider {
		return testutil.NewMockProvider("test", providerType)
	}

	factory.RegisterProvider(providerType, factoryFunc)

	// Verify provider is registered
	supportedProviders := factory.GetSupportedProviders()
	assert.Contains(t, supportedProviders, providerType)
	assert.Len(t, supportedProviders, 1)
}

// TestDefaultProviderFactory_RegisterProvider_ConcurrentAccess tests thread safety of provider registration
func TestDefaultProviderFactory_RegisterProvider_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	factory := NewProviderFactory()
	var wg sync.WaitGroup
	numGoroutines := 100

	// Register providers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			providerType := types.ProviderType(fmt.Sprintf("provider-%d", i))
			factoryFunc := func(config types.ProviderConfig) types.Provider {
				return testutil.NewMockProvider(fmt.Sprintf("test-%d", i), providerType)
			}
			factory.RegisterProvider(providerType, factoryFunc)
		}(i)
	}

	wg.Wait()

	// Verify all providers were registered
	supportedProviders := factory.GetSupportedProviders()
	assert.Len(t, supportedProviders, numGoroutines)
}

// TestDefaultProviderFactory_CreateProvider tests provider creation functionality
func TestDefaultProviderFactory_CreateProvider(t *testing.T) {
	t.Parallel()
	factory := NewProviderFactory()

	// Register a test provider
	providerType := types.ProviderType("test-provider")
	expectedProvider := testutil.NewMockProvider("test", providerType)
	factoryFunc := func(config types.ProviderConfig) types.Provider {
		return expectedProvider
	}
	factory.RegisterProvider(providerType, factoryFunc)

	// Create provider with valid config
	config := types.ProviderConfig{
		Type:    providerType,
		Name:    "test-provider-instance",
		APIKey:  "test-api-key",
		BaseURL: "https://api.example.com",
	}

	provider, err := factory.CreateProvider(providerType, config)

	// Verify provider creation
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, expectedProvider, provider)
}

// TestDefaultProviderFactory_CreateProvider_UnknownProvider tests error handling for unknown providers
func TestDefaultProviderFactory_CreateProvider_UnknownProvider(t *testing.T) {
	t.Parallel()
	factory := NewProviderFactory()

	// Try to create a provider that wasn't registered
	unknownProviderType := types.ProviderType("unknown-provider")
	config := types.ProviderConfig{
		Type:   unknownProviderType,
		Name:   "unknown-provider-instance",
		APIKey: "test-api-key",
	}

	provider, err := factory.CreateProvider(unknownProviderType, config)

	// Verify error handling
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "provider type unknown-provider not registered")
}

// TestDefaultProviderFactory_CreateProvider_ConcurrentAccess tests thread safety of provider creation
func TestDefaultProviderFactory_CreateProvider_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	factory := NewProviderFactory()

	// Register a test provider
	providerType := types.ProviderType("concurrent-test-provider")
	factoryFunc := func(config types.ProviderConfig) types.Provider {
		return testutil.NewMockProvider(config.Name, providerType)
	}
	factory.RegisterProvider(providerType, factoryFunc)

	var wg sync.WaitGroup
	numGoroutines := 50
	errors := make(chan error, numGoroutines)
	providers := make([]types.Provider, numGoroutines)

	// Create providers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			config := types.ProviderConfig{
				Type:   providerType,
				Name:   fmt.Sprintf("provider-%d", i),
				APIKey: "test-api-key",
			}
			provider, err := factory.CreateProvider(providerType, config)
			if err != nil {
				errors <- err
				return
			}
			providers[i] = provider
		}(i)
	}

	wg.Wait()
	close(errors)

	// Verify no errors occurred and all providers were created
	for err := range errors {
		t.Errorf("Unexpected error during concurrent provider creation: %v", err)
	}

	for i, provider := range providers {
		assert.NotNil(t, provider, "Provider %d should not be nil", i)
		mockProvider, ok := provider.(*testutil.MockProvider)
		require.True(t, ok, "Provider should be of type MockProvider")
		assert.Equal(t, fmt.Sprintf("provider-%d", i), mockProvider.Name())
	}
}

// TestDefaultProviderFactory_GetSupportedProviders tests provider list retrieval
func TestDefaultProviderFactory_GetSupportedProviders(t *testing.T) {
	t.Parallel()
	factory := NewProviderFactory()

	// Initially no providers should be supported
	supportedProviders := factory.GetSupportedProviders()
	assert.Empty(t, supportedProviders)

	// Register multiple providers
	providerTypes := []types.ProviderType{
		"provider-1",
		"provider-2",
		"provider-3",
	}

	for _, providerType := range providerTypes {
		pt := providerType // Capture variable for closure
		factory.RegisterProvider(providerType, func(config types.ProviderConfig) types.Provider {
			return testutil.NewMockProvider(string(pt), pt)
		})
	}

	// Verify all providers are returned
	supportedProviders = factory.GetSupportedProviders()
	assert.Len(t, supportedProviders, len(providerTypes))

	for _, providerType := range providerTypes {
		assert.Contains(t, supportedProviders, providerType)
	}
}

// TestDefaultProviderFactory_GetSupportedProviders_ConcurrentAccess tests thread safety of provider listing
func TestDefaultProviderFactory_GetSupportedProviders_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	factory := NewProviderFactory()

	// Register initial providers
	for i := 0; i < 10; i++ {
		providerType := types.ProviderType(fmt.Sprintf("provider-%d", i))
		pt := providerType // Capture variable for closure
		factory.RegisterProvider(providerType, func(config types.ProviderConfig) types.Provider {
			return testutil.NewMockProvider(string(pt), pt)
		})
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	results := make(chan []types.ProviderType, numGoroutines)

	// Get supported providers concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			providers := factory.GetSupportedProviders()
			results <- providers
		}()
	}

	wg.Wait()
	close(results)

	// Verify all results are consistent
	expectedLength := 10
	for providers := range results {
		assert.Len(t, providers, expectedLength)
	}
}

// TestInitializeDefaultProviders tests stub provider initialization
func TestInitializeDefaultProviders(t *testing.T) {
	t.Parallel()
	factory := NewProviderFactory()

	// Initialize default stub providers (only for providers without real implementations)
	InitializeDefaultProviders(factory)

	// Verify only stub providers are registered
	supportedProviders := factory.GetSupportedProviders()
	expectedProviders := []types.ProviderType{
		types.ProviderTypeLMStudio,
		types.ProviderTypeLlamaCpp,
	}

	assert.Len(t, supportedProviders, len(expectedProviders))
	for _, expectedProvider := range expectedProviders {
		assert.Contains(t, supportedProviders, expectedProvider)
	}

	// Verify we can create each stub provider
	for _, providerType := range expectedProviders {
		config := types.ProviderConfig{
			Type:   providerType,
			Name:   string(providerType) + "-test",
			APIKey: "test-api-key",
		}
		provider, err := factory.CreateProvider(providerType, config)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, providerType, provider.Type())
	}

	// Verify that providers with real implementations are NOT registered by this function
	realImplementationProviders := []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeAnthropic,
		types.ProviderTypeGemini,
		types.ProviderTypeCerebras,
		types.ProviderTypeOpenRouter,
		types.ProviderTypeOllama,
	}

	for _, providerType := range realImplementationProviders {
		config := types.ProviderConfig{
			Type:   providerType,
			Name:   string(providerType) + "-test",
			APIKey: "test-api-key",
		}
		provider, err := factory.CreateProvider(providerType, config)
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "provider type "+string(providerType)+" not registered")
	}
}

// TestSimpleProviderStub tests the SimpleProviderStub implementation
func TestSimpleProviderStub(t *testing.T) {
	t.Parallel()
	config := types.ProviderConfig{
		Type:         types.ProviderTypeOpenAI,
		Name:         "test-provider",
		BaseURL:      "https://api.openai.com",
		APIKey:       "test-api-key",
		DefaultModel: "gpt-3.5-turbo",
		Description:  "Test OpenAI provider",
	}

	provider := &SimpleProviderStub{
		name:         "test-provider",
		providerType: types.ProviderTypeOpenAI,
		config:       config,
	}

	// Test basic provider information
	assert.Equal(t, "test-provider", provider.Name())
	assert.Equal(t, types.ProviderTypeOpenAI, provider.Type())
	assert.Equal(t, "test-provider provider", provider.Description())

	// Test model management
	models, err := provider.GetModels(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, models) // SimpleProviderStub now returns mock models
	assert.Len(t, models, 2)   // Should return 2 mock models
	assert.Equal(t, "openai-model-1", models[0].ID)
	assert.Equal(t, "openai Default Model", models[0].Name)
	assert.Equal(t, "default-model", provider.GetDefaultModel())

	// Test capabilities
	assert.True(t, provider.SupportsToolCalling())
	assert.True(t, provider.SupportsStreaming())
	assert.False(t, provider.SupportsResponsesAPI())
	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())

	// Test authentication
	assert.True(t, provider.IsAuthenticated())
	err = provider.Authenticate(context.Background(), types.AuthConfig{})
	assert.NoError(t, err)
	err = provider.Logout(context.Background())
	assert.NoError(t, err)

	// Test configuration
	err = provider.Configure(config)
	assert.NoError(t, err)
	assert.Equal(t, config, provider.GetConfig())

	// Test generation
	options := types.GenerateOptions{
		Prompt:      "Hello, world!",
		MaxTokens:   100,
		Temperature: 0.7,
	}
	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Test streaming
	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.True(t, chunk.Done)
	err = stream.Close()
	assert.NoError(t, err)

	// Test tool invocation
	result, err := provider.InvokeServerTool(context.Background(), "test-tool", map[string]interface{}{"param": "value"})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tool calling not implemented")

	// Test health check
	err = provider.HealthCheck(context.Background())
	assert.NoError(t, err)

	// Test metrics - provider should have 1 request from the generation test above
	metrics := provider.GetMetrics()
	assert.Equal(t, int64(1), metrics.RequestCount)
	assert.Equal(t, int64(1), metrics.SuccessCount)
	assert.Equal(t, int64(0), metrics.ErrorCount)
}

// TestFactoryMockStream tests the FactoryMockStream implementation
func TestFactoryMockStream(t *testing.T) {
	t.Parallel()
	stream := &FactoryMockStream{}

	// Test streaming
	chunk, err := stream.Next()
	assert.NoError(t, err)
	assert.True(t, chunk.Done)

	// Test closing
	err = stream.Close()
	assert.NoError(t, err)
}

// BenchmarkRegisterProvider benchmarks provider registration
func BenchmarkRegisterProvider(b *testing.B) {
	factory := NewProviderFactory()
	providerType := types.ProviderType("benchmark-provider")
	factoryFunc := func(config types.ProviderConfig) types.Provider {
		return testutil.NewMockProvider("benchmark", providerType)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		factory.RegisterProvider(
			types.ProviderType(fmt.Sprintf("%s-%d", providerType, i)),
			factoryFunc,
		)
	}
}

// BenchmarkCreateProvider benchmarks provider creation
func BenchmarkCreateProvider(b *testing.B) {
	factory := NewProviderFactory()
	providerType := types.ProviderType("benchmark-provider")
	factoryFunc := func(config types.ProviderConfig) types.Provider {
		return testutil.NewMockProvider("benchmark", providerType)
	}
	factory.RegisterProvider(providerType, factoryFunc)

	config := types.ProviderConfig{
		Type:   providerType,
		Name:   "benchmark-provider-instance",
		APIKey: "test-api-key",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := factory.CreateProvider(providerType, config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetSupportedProviders benchmarks provider listing
func BenchmarkGetSupportedProviders(b *testing.B) {
	factory := NewProviderFactory()

	// Register multiple providers
	for i := 0; i < 100; i++ {
		providerType := types.ProviderType(fmt.Sprintf("provider-%d", i))
		name := fmt.Sprintf("provider-%d", i)
		pt := providerType // Capture variable for closure
		factoryFunc := func(config types.ProviderConfig) types.Provider {
			return testutil.NewMockProvider(name, pt)
		}
		factory.RegisterProvider(providerType, factoryFunc)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = factory.GetSupportedProviders()
	}
}

// Note: MockProvider has been moved to pkg/testutil
