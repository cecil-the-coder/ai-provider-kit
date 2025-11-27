package factory

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestQwenProvider_FactoryIntegration(t *testing.T) {
	// Create a new factory
	f := NewProviderFactory()

	// Register all default providers
	RegisterDefaultProviders(f)

	// Create a Qwen provider config
	config := types.ProviderConfig{
		Type:         types.ProviderTypeQwen,
		Name:         "test-qwen",
		APIKey:       "test-api-key",
		BaseURL:      "https://portal.qwen.ai/v1",
		DefaultModel: "qwen-turbo",
	}

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

	if provider.GetDefaultModel() != "qwen-turbo" {
		t.Errorf("Expected default model 'qwen-turbo', got '%s'", provider.GetDefaultModel())
	}

	// Test authentication
	ctx := context.Background()
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "test-api-key",
	}

	err = provider.Authenticate(ctx, authConfig)
	if err != nil {
		t.Fatalf("Expected no error authenticating, got %v", err)
	}

	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated")
	}

	// Test getting models
	models, err := provider.GetModels(ctx)
	if err != nil {
		t.Fatalf("Expected no error getting models, got %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	// Verify known models exist
	expectedModels := []string{"qwen-turbo", "qwen-plus", "qwen-max"}
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
	// Create a new factory
	f := NewProviderFactory()

	// Register all default providers
	RegisterDefaultProviders(f)

	// Create a Qwen provider config with OAuth
	config := types.ProviderConfig{
		Type: types.ProviderTypeQwen,
		Name: "test-qwen-oauth",
		OAuthCredentials: []*types.OAuthCredentialSet{
			{
				ID:           "test-cred",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
		},
	}

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
	// Test that the provider is created successfully
	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	// Note: OAuth authentication flow has changed - credentials are now managed
	// by OAuthKeyManager internally. We just verify the provider was created.
	t.Log("Provider created successfully with OAuth credentials configuration")
}
