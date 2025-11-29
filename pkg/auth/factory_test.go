package auth

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewAuthenticatorFactory(t *testing.T) {
	t.Run("WithConfig", func(t *testing.T) {
		config := DefaultConfig()
		storage := NewMemoryTokenStorage(nil)
		factory := NewAuthenticatorFactory(config, storage)

		if factory == nil {
			t.Fatal("Expected non-nil factory")
		}
		if factory.config != config {
			t.Error("Expected config to be set")
		}
		if factory.storage != storage {
			t.Error("Expected storage to be set")
		}
	})

	t.Run("WithNilConfig", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		factory := NewAuthenticatorFactory(nil, storage)

		if factory == nil {
			t.Fatal("Expected non-nil factory")
		}
		if factory.config == nil {
			t.Error("Expected default config")
		}
	})
}

func TestFactorySetLogger(t *testing.T) {
	factory := NewAuthenticatorFactory(nil, NewMemoryTokenStorage(nil))
	logger := &TestLogger{}

	factory.SetLogger(logger)

	if factory.logger != logger {
		t.Error("Expected logger to be set")
	}
}

func TestCreateOAuthAuthenticator(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	factory := NewAuthenticatorFactory(nil, storage)

	t.Run("ValidConfig", func(t *testing.T) {
		oauthConfig := &types.OAuthConfig{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			AuthURL:      "https://example.com/auth",
			TokenURL:     "https://example.com/token",
			RedirectURL:  "https://example.com/callback",
		}

		auth, err := factory.CreateAuthenticator("test", types.AuthMethodOAuth, oauthConfig)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if auth == nil {
			t.Error("Expected non-nil authenticator")
		}
		if auth.GetAuthMethod() != types.AuthMethodOAuth {
			t.Error("Expected OAuth auth method")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		_, err := factory.CreateAuthenticator("test", types.AuthMethodOAuth, nil)
		if err == nil {
			t.Error("Expected error for nil config")
		}
	})

	t.Run("WrongConfigType", func(t *testing.T) {
		_, err := factory.CreateAuthenticator("test", types.AuthMethodOAuth, "wrong-type")
		if err == nil {
			t.Error("Expected error for wrong config type")
		}
	})
}

func TestCreateAPIKeyAuthenticator(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	factory := NewAuthenticatorFactory(nil, storage)

	t.Run("StringKey", func(t *testing.T) {
		auth, err := factory.CreateAuthenticator("test", types.AuthMethodAPIKey, "test-key")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if auth == nil {
			t.Error("Expected non-nil authenticator")
		}
		if auth.GetAuthMethod() != types.AuthMethodAPIKey {
			t.Error("Expected API key auth method")
		}
	})

	t.Run("StringSliceKeys", func(t *testing.T) {
		keys := []string{"key1", "key2", "key3"}
		auth, err := factory.CreateAuthenticator("test", types.AuthMethodAPIKey, keys)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if auth == nil {
			t.Error("Expected non-nil authenticator")
		}
	})

	t.Run("AuthConfig", func(t *testing.T) {
		authConfig := &types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: "test-key",
		}

		auth, err := factory.CreateAuthenticator("test", types.AuthMethodAPIKey, authConfig)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if auth == nil {
			t.Error("Expected non-nil authenticator")
		}
	})

	t.Run("APIKeyConfig", func(t *testing.T) {
		apiKeyConfig := APIKeyConfig{
			Strategy: "round_robin",
		}

		// This should work even though no keys are provided in config
		// because we're testing the config type handling
		auth, err := factory.CreateAuthenticator("test", types.AuthMethodAPIKey, apiKeyConfig)
		if err == nil {
			t.Error("Expected error for no keys in config")
		}
		if auth != nil {
			t.Error("Expected nil authenticator for invalid config")
		}
	})

	t.Run("EmptyKeys", func(t *testing.T) {
		_, err := factory.CreateAuthenticator("test", types.AuthMethodAPIKey, []string{})
		if err == nil {
			t.Error("Expected error for empty keys")
		}
	})

	t.Run("WrongConfigType", func(t *testing.T) {
		_, err := factory.CreateAuthenticator("test", types.AuthMethodAPIKey, 123)
		if err == nil {
			t.Error("Expected error for wrong config type")
		}
	})
}

func TestCreateBearerTokenAuthenticator(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	factory := NewAuthenticatorFactory(nil, storage)

	t.Run("StringToken", func(t *testing.T) {
		auth, err := factory.CreateAuthenticator("test", types.AuthMethodBearerToken, "bearer-token")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if auth == nil {
			t.Error("Expected non-nil authenticator")
		}
		if auth.GetAuthMethod() != types.AuthMethodBearerToken {
			t.Error("Expected bearer token auth method")
		}
	})

	t.Run("AuthConfigPointer", func(t *testing.T) {
		authConfig := &types.AuthConfig{
			Method: types.AuthMethodBearerToken,
			APIKey: "test-token",
		}

		auth, err := factory.CreateAuthenticator("test", types.AuthMethodBearerToken, authConfig)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if auth == nil {
			t.Error("Expected non-nil authenticator")
		}
	})

	t.Run("AuthConfigValue", func(t *testing.T) {
		authConfig := types.AuthConfig{
			Method: types.AuthMethodBearerToken,
			APIKey: "test-token",
		}

		auth, err := factory.CreateAuthenticator("test", types.AuthMethodBearerToken, authConfig)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if auth == nil {
			t.Error("Expected non-nil authenticator")
		}
	})

	t.Run("EmptyToken", func(t *testing.T) {
		_, err := factory.CreateAuthenticator("test", types.AuthMethodBearerToken, "")
		if err == nil {
			t.Error("Expected error for empty token")
		}
	})

	t.Run("WrongConfigType", func(t *testing.T) {
		_, err := factory.CreateAuthenticator("test", types.AuthMethodBearerToken, 123)
		if err == nil {
			t.Error("Expected error for wrong config type")
		}
	})
}

func TestCreateCustomAuthenticator(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	factory := NewAuthenticatorFactory(nil, storage)

	t.Run("ValidAuthenticator", func(t *testing.T) {
		custom := &CustomAuthenticator{}
		auth, err := factory.CreateAuthenticator("test", types.AuthMethodCustom, custom)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if auth != custom {
			t.Error("Expected same authenticator instance")
		}
	})

	t.Run("InvalidType", func(t *testing.T) {
		_, err := factory.CreateAuthenticator("test", types.AuthMethodCustom, "not-an-authenticator")
		if err == nil {
			t.Error("Expected error for invalid type")
		}
	})
}

func TestUnsupportedAuthMethod(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	factory := NewAuthenticatorFactory(nil, storage)

	_, err := factory.CreateAuthenticator("test", "unsupported", "config")
	if err == nil {
		t.Error("Expected error for unsupported auth method")
	}
}

func TestBearerTokenAuthenticator(t *testing.T) {
	t.Run("Authenticate", func(t *testing.T) {
		auth := &BearerTokenAuthenticatorImpl{
			provider: "test",
		}

		authConfig := types.AuthConfig{
			Method: types.AuthMethodBearerToken,
			APIKey: "test-token",
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !auth.isAuth {
			t.Error("Expected to be authenticated")
		}
		if auth.token != "test-token" {
			t.Error("Expected token to be set")
		}
	})

	t.Run("WrongMethod", func(t *testing.T) {
		auth := &BearerTokenAuthenticatorImpl{
			provider: "test",
		}

		authConfig := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err == nil {
			t.Error("Expected error for wrong method")
		}
	})

	t.Run("EmptyToken", func(t *testing.T) {
		auth := &BearerTokenAuthenticatorImpl{
			provider: "test",
		}

		authConfig := types.AuthConfig{
			Method: types.AuthMethodBearerToken,
			APIKey: "",
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err == nil {
			t.Error("Expected error for empty token")
		}
	})

	t.Run("IsAuthenticated", func(t *testing.T) {
		auth := &BearerTokenAuthenticatorImpl{
			provider: "test",
			token:    "test-token",
			isAuth:   true,
		}

		if !auth.IsAuthenticated() {
			t.Error("Expected to be authenticated")
		}
	})

	t.Run("GetToken", func(t *testing.T) {
		auth := &BearerTokenAuthenticatorImpl{
			provider: "test",
			token:    "test-token",
			isAuth:   true,
		}

		token, err := auth.GetToken()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if token != "test-token" {
			t.Errorf("Expected 'test-token', got '%s'", token)
		}
	})

	t.Run("GetTokenNotAuthenticated", func(t *testing.T) {
		auth := &BearerTokenAuthenticatorImpl{
			provider: "test",
		}

		_, err := auth.GetToken()
		if err == nil {
			t.Error("Expected error for not authenticated")
		}
	})

	t.Run("RefreshToken", func(t *testing.T) {
		auth := &BearerTokenAuthenticatorImpl{
			provider: "test",
		}

		ctx := context.Background()
		err := auth.RefreshToken(ctx)
		if err != nil {
			t.Errorf("Expected no error (no-op), got: %v", err)
		}
	})

	t.Run("Logout", func(t *testing.T) {
		auth := &BearerTokenAuthenticatorImpl{
			provider: "test",
			token:    "test-token",
			isAuth:   true,
		}

		ctx := context.Background()
		err := auth.Logout(ctx)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if auth.isAuth {
			t.Error("Expected to be logged out")
		}
		if auth.token != "" {
			t.Error("Expected token to be cleared")
		}
	})

	t.Run("GetAuthMethod", func(t *testing.T) {
		auth := &BearerTokenAuthenticatorImpl{
			provider: "test",
		}

		if auth.GetAuthMethod() != types.AuthMethodBearerToken {
			t.Error("Expected bearer token auth method")
		}
	})
}

func TestProviderAuthRegistry(t *testing.T) {
	t.Run("NewRegistry", func(t *testing.T) {
		registry := NewProviderAuthRegistry()
		if registry == nil {
			t.Error("Expected non-nil registry")
		}
	})

	t.Run("RegisterAndGet", func(t *testing.T) {
		registry := NewProviderAuthRegistry()
		config := &ProviderAuthConfig{
			Provider:    "test",
			AuthMethod:  types.AuthMethodAPIKey,
			DisplayName: "Test Provider",
		}

		registry.RegisterProvider(config)

		retrieved, exists := registry.GetProvider("test")
		if !exists {
			t.Error("Expected provider to exist")
		}
		if retrieved.Provider != "test" {
			t.Error("Expected provider name to match")
		}
	})

	t.Run("GetNonexistent", func(t *testing.T) {
		registry := NewProviderAuthRegistry()
		_, exists := registry.GetProvider("nonexistent")
		if exists {
			t.Error("Expected provider not to exist")
		}
	})

	t.Run("ListProviders", func(t *testing.T) {
		registry := NewProviderAuthRegistry()
		registry.RegisterProvider(&ProviderAuthConfig{Provider: "test1"})
		registry.RegisterProvider(&ProviderAuthConfig{Provider: "test2"})

		providers := registry.ListProviders()
		if len(providers) != 2 {
			t.Errorf("Expected 2 providers, got %d", len(providers))
		}
	})

	t.Run("GetProvidersByMethod", func(t *testing.T) {
		registry := NewProviderAuthRegistry()

		registry.RegisterProvider(&ProviderAuthConfig{
			Provider:   "apikey1",
			AuthMethod: types.AuthMethodAPIKey,
			Features: AuthFeatureFlags{
				SupportsAPIKey: true,
			},
		})

		registry.RegisterProvider(&ProviderAuthConfig{
			Provider:   "oauth1",
			AuthMethod: types.AuthMethodOAuth,
			Features: AuthFeatureFlags{
				SupportsOAuth: true,
			},
		})

		apiKeyProviders := registry.GetProvidersByMethod(types.AuthMethodAPIKey)
		if len(apiKeyProviders) != 1 {
			t.Errorf("Expected 1 API key provider, got %d", len(apiKeyProviders))
		}

		oauthProviders := registry.GetProvidersByMethod(types.AuthMethodOAuth)
		if len(oauthProviders) != 1 {
			t.Errorf("Expected 1 OAuth provider, got %d", len(oauthProviders))
		}
	})

	t.Run("CreateStandardRegistry", func(t *testing.T) {
		registry := CreateStandardRegistry()

		providers := registry.ListProviders()
		if len(providers) < 3 {
			t.Errorf("Expected at least 3 standard providers, got %d", len(providers))
		}

		// Check for standard providers
		_, openaiExists := registry.GetProvider("openai")
		if !openaiExists {
			t.Error("Expected OpenAI provider to exist")
		}

		_, anthropicExists := registry.GetProvider("anthropic")
		if !anthropicExists {
			t.Error("Expected Anthropic provider to exist")
		}

		_, geminiExists := registry.GetProvider("gemini")
		if !geminiExists {
			t.Error("Expected Gemini provider to exist")
		}
	})
}
