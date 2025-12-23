package factory

import (
	"context"
	"os"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/testutil"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// skipIfNoAPIKey skips the test if QWEN_API_KEY is not set
func skipIfNoQwenAPIKey(t *testing.T) {
	if os.Getenv("QWEN_API_KEY") == "" {
		t.Skip("Skipping integration test: QWEN_API_KEY not set")
	}
}

func TestQwenProvider_FactoryIntegration(t *testing.T) {
	t.Parallel()
	// Create a new factory
	f := NewProviderFactory()

	// Register all default providers
	RegisterDefaultProviders(f)

	// Create a Qwen provider config using helper
	config := testutil.DefaultProviderConfig(types.ProviderTypeQwen, "test-qwen")
	config.BaseURL = "https://portal.qwen.ai/v1"
	config.DefaultModel = "qwen3-coder-flash"

	// Create the provider using the factory
	provider, err := f.CreateProvider(types.ProviderTypeQwen, config)
	if err != nil {
		t.Fatalf("Expected no error creating provider, got %v", err)
	}

	// Verify the provider has the correct type
	if provider.Type() != types.ProviderTypeQwen {
		t.Fatalf("Expected provider type %s, got %s", types.ProviderTypeQwen, provider.Type())
	}

	// Test basic provider methods
	if provider.Name() != "Qwen" {
		t.Errorf("Expected name 'Qwen', got '%s'", provider.Name())
	}

	if provider.GetDefaultModel() != "qwen3-coder-flash" {
		t.Errorf("Expected default model 'qwen3-coder-flash', got '%s'", provider.GetDefaultModel())
	}

	// Test authentication using helper
	ctx := context.Background()
	authConfig := testutil.DefaultAuthConfig(config)

	err = provider.Authenticate(ctx, authConfig)
	if err != nil {
		t.Fatalf("Expected no error authenticating, got %v", err)
	}

	testutil.RequireProviderAuthenticated(t, provider)

	// Test getting models
	models, err := provider.GetModels(ctx)
	if err != nil {
		t.Fatalf("Expected no error getting models, got %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	// Verify known models exist
	expectedModels := []string{"qwen3-coder-flash", "qwen3-coder-plus"}
	modelMap := make(map[string]bool)
	for _, model := range models {
		modelMap[model.ID] = true
	}

	for _, expectedID := range expectedModels {
		if !modelMap[expectedID] {
			t.Errorf("Expected model %s not found", expectedID)
		}
	}

	// Test capabilities
	if !provider.SupportsToolCalling() {
		t.Error("Expected provider to support tool calling")
	}

	if !provider.SupportsStreaming() {
		t.Error("Expected provider to support streaming")
	}

	if provider.SupportsResponsesAPI() {
		t.Error("Expected provider to not support responses API")
	}

	if provider.GetToolFormat() != types.ToolFormatOpenAI {
		t.Errorf("Expected tool format %s, got %s", types.ToolFormatOpenAI, provider.GetToolFormat())
	}
}

func TestQwenProvider_OAuthFlow(t *testing.T) {
	t.Parallel()
	// Create a new factory
	f := NewProviderFactory()

	// Register all default providers
	RegisterDefaultProviders(f)

	// Create a Qwen provider config with OAuth using helper
	oauthCreds := testutil.MultiOAuthTestConfig(1)
	oauthCreds[0].ClientID = "test-client-id"
	oauthCreds[0].ClientSecret = "test-client-secret"

	config := testutil.ProviderConfigWithOAuth(types.ProviderTypeQwen, "test-qwen-oauth", oauthCreds)

	// Create the provider using the factory
	provider, err := f.CreateProvider(types.ProviderTypeQwen, config)
	if err != nil {
		t.Fatalf("Expected no error creating provider, got %v", err)
	}

	// Verify the provider has the correct type
	if provider.Type() != types.ProviderTypeQwen {
		t.Fatalf("Expected provider type %s, got %s", types.ProviderTypeQwen, provider.Type())
	}

	// OAuth authentication is now handled through OAuthCredentials in ProviderConfig
	// The provider should already be set up with OAuth if credentials were provided
	// Test that the provider was created successfully
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	// Note: OAuth authentication flow has changed - credentials are now managed
	// by OAuthKeyManager internally. We just verify the provider was created.
	t.Log("Provider created successfully with OAuth credentials configuration")
}

func TestQwenProvider_HappyPathScenario(t *testing.T) {
	t.Parallel()
	skipIfNoQwenAPIKey(t)

	// Create a new factory
	f := NewProviderFactory()

	// Register all default providers
	RegisterDefaultProviders(f)

	// Create provider using helper
	config := testutil.DefaultProviderConfig(types.ProviderTypeQwen, "qwen-happy-path")

	provider, err := f.CreateProvider(types.ProviderTypeQwen, config)
	if err != nil {
		t.Fatalf("Expected no error creating provider, got %v", err)
	}

	// Run standard happy path scenario
	ctx := testutil.NewTestContext(t, defaultTestTimeout)
	testutil.HappyPathScenario(t, provider, ctx)
}
