package factory

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/cerebras"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/testutil"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestCerebrasProvider_FactoryIntegration(t *testing.T) {
	t.Parallel()
	// Create a new factory
	f := NewProviderFactory()

	// Register all default providers
	RegisterDefaultProviders(f)

	// Create a Cerebras provider config using helper
	config := testutil.DefaultProviderConfig(types.ProviderTypeCerebras, "test-cerebras")
	config.DefaultModel = "zai-glm-4.6"
	config.ProviderConfig = map[string]interface{}{
		"temperature": 0.7,
		"api_keys":    []string{"key1", "key2"},
	}

	// Create the provider using the factory
	provider, err := f.CreateProvider(types.ProviderTypeCerebras, config)
	if err != nil {
		t.Fatalf("Expected no error creating provider, got %v", err)
	}

	// Verify the provider is a CerebrasProvider
	cerebrasProvider, ok := provider.(*cerebras.CerebrasProvider)
	if !ok {
		t.Fatalf("Expected *cerebras.CerebrasProvider, got %T", provider)
	}

	// Test basic provider methods
	if cerebrasProvider.Name() != "Cerebras" {
		t.Errorf("Expected name 'Cerebras', got '%s'", cerebrasProvider.Name())
	}

	if cerebrasProvider.Type() != types.ProviderTypeCerebras {
		t.Errorf("Expected type %s, got %s", types.ProviderTypeCerebras, cerebrasProvider.Type())
	}

	if cerebrasProvider.GetDefaultModel() != "zai-glm-4.6" {
		t.Errorf("Expected default model 'zai-glm-4.6', got '%s'", cerebrasProvider.GetDefaultModel())
	}

	// Test authentication using helper
	testutil.RequireProviderAuthenticated(t, cerebrasProvider)

	// Test getting models
	models, err := cerebrasProvider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("Expected no error getting models, got %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	// Verify ZAI model exists
	foundZAI := false
	for _, model := range models {
		if model.ID == "zai-glm-4.6" {
			foundZAI = true
			break
		}
	}

	if !foundZAI {
		t.Error("Expected to find zai-glm-4.6 model")
	}

	// Test capabilities
	if !cerebrasProvider.SupportsToolCalling() {
		t.Error("Expected provider to support tool calling")
	}

	if !cerebrasProvider.SupportsStreaming() {
		t.Error("Expected provider to support streaming")
	}

	if cerebrasProvider.SupportsResponsesAPI() {
		t.Error("Expected provider not to support responses API")
	}

	if cerebrasProvider.GetToolFormat() != types.ToolFormatOpenAI {
		t.Errorf("Expected tool format %s, got %s", types.ToolFormatOpenAI, cerebrasProvider.GetToolFormat())
	}
}

func TestCerebrasProvider_MultipleAPIKeys_FactoryIntegration(t *testing.T) {
	t.Parallel()
	// Create a new factory
	f := NewProviderFactory()

	// Register all default providers
	RegisterDefaultProviders(f)

	// Create a Cerebras provider config with multiple API keys using helper
	config := testutil.DefaultProviderConfig(types.ProviderTypeCerebras, "test-cerebras-multi-key")
	config.ProviderConfig = map[string]interface{}{
		"api_keys": []string{"key1", "key2", "key3"},
	}

	// Create the provider using the factory
	provider, err := f.CreateProvider(types.ProviderTypeCerebras, config)
	if err != nil {
		t.Fatalf("Expected no error creating provider, got %v", err)
	}

	cerebrasProvider := provider.(*cerebras.CerebrasProvider)

	// Test that multiple API keys are configured
	testutil.RequireProviderAuthenticated(t, cerebrasProvider)

	// Test that the provider is properly configured
	retrievedConfig := cerebrasProvider.GetConfig()
	if retrievedConfig.APIKey != "test-cerebras-api-key" {
		t.Errorf("Expected API key 'test-cerebras-api-key', got '%s'", retrievedConfig.APIKey)
	}
}

func TestCerebrasProvider_HappyPathScenario(t *testing.T) {
	t.Parallel()
	// Create a new factory
	f := NewProviderFactory()

	// Register all default providers
	RegisterDefaultProviders(f)

	// Create provider using helper
	config := testutil.DefaultProviderConfig(types.ProviderTypeCerebras, "cerebras-happy-path")

	provider, err := f.CreateProvider(types.ProviderTypeCerebras, config)
	if err != nil {
		t.Fatalf("Expected no error creating provider, got %v", err)
	}

	// Run standard happy path scenario
	ctx := testutil.NewTestContext(t, defaultTestTimeout)
	testutil.HappyPathScenario(t, provider, ctx)
}

func TestCerebrasProvider_StreamingScenario(t *testing.T) {
	t.Parallel()
	// Create a new factory
	f := NewProviderFactory()

	// Register all default providers
	RegisterDefaultProviders(f)

	// Create provider using helper
	config := testutil.DefaultProviderConfig(types.ProviderTypeCerebras, "cerebras-streaming")

	provider, err := f.CreateProvider(types.ProviderTypeCerebras, config)
	if err != nil {
		t.Fatalf("Expected no error creating provider, got %v", err)
	}

	// Run streaming scenario
	ctx := testutil.NewTestContext(t, defaultTestTimeout)
	testutil.StreamingScenario(t, provider, ctx)
}
