package auth

import (
	"context"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ExampleNewAuthManager shows how to create a new auth manager
func ExampleNewAuthManager() {
	// Create default configuration with encryption disabled for testing
	config := DefaultConfig()
	config.TokenStorage.Encryption.Enabled = false

	// Create memory-based token storage for testing
	storage := NewMemoryTokenStorage(&config.TokenStorage.Memory)

	// Create auth manager
	authManager := NewAuthManager(storage, config)

	// Register OAuth authenticator for Anthropic
	authenticator := NewOAuthAuthenticator("anthropic", storage, &config.OAuth)
	if err := authManager.RegisterAuthenticator("anthropic", authenticator); err != nil {
		// In real usage, you'd handle this error properly
		println("Warning: failed to register authenticator:", err.Error())
	}

	// Check auth status
	status := authManager.GetAuthStatus()
	for provider, state := range status {
		println("Provider:", provider, "Authenticated:", state.Authenticated)
	}
}

// ExampleAPIKeyManager shows how to use the API key manager with multi-key support
func ExampleAPIKeyManager() {
	config := &APIKeyConfig{
		Strategy: "round_robin",
		Health: HealthConfig{
			Enabled:          true,
			FailureThreshold: 3,
			Backoff: BackoffConfig{
				Initial:    1 * time.Second,
				Maximum:    60 * time.Second,
				Multiplier: 2.0,
				Jitter:     true,
			},
		},
		Failover: FailoverConfig{
			Enabled:     true,
			MaxAttempts: 3,
		},
	}

	keys := []string{
		"sk-1234567890abcdef",
		"sk-abcdef1234567890",
		"sk-7890abcdef123456",
	}

	manager, err := NewAPIKeyManager("openai", keys, config)
	if err != nil {
		panic(err)
	}

	// Execute with failover
	result, err := manager.ExecuteWithFailover(func(apiKey string) (string, error) {
		// Simulate API call
		println("Using API key:", apiKey[:8]+"...")
		return "API response", nil
	})

	if err != nil {
		panic(err)
	}

	println("Result:", result)

	// Get status
	status := manager.GetStatus()
	println("Healthy keys:", status["healthy_keys"])
}

// ExampleOAuthAuthenticator shows how to implement OAuth flow
func ExampleOAuthAuthenticator() {
	// Create storage and config
	storage := NewMemoryTokenStorage(nil)
	oauthConfig := &OAuthConfig{
		DefaultScopes: []string{"profile", "email"},
		State: StateConfig{
			Length:           32,
			Expiration:       10 * time.Minute,
			EnableValidation: true,
		},
		PKCE: PKCEConfig{
			Enabled:        true,
			Method:         "S256",
			VerifierLength: 128,
		},
		Refresh: RefreshConfig{
			Enabled:    true,
			Buffer:     5 * time.Minute,
			MaxRetries: 3,
		},
	}

	// Create OAuth authenticator
	authenticator := NewOAuthAuthenticator("google", storage, oauthConfig)
	var oauthAuthenticator OAuthAuthenticator = authenticator

	ctx := context.Background()

	// Configure OAuth
	authConfig := types.AuthConfig{
		Method: types.AuthMethodOAuth,
		APIKey: "", // OAuth uses token storage, not direct API key
	}

	err := oauthAuthenticator.Authenticate(ctx, authConfig)
	if err != nil {
		// Need to start OAuth flow
		authURL, err := oauthAuthenticator.StartOAuthFlow(ctx, []string{"profile", "email"})
		if err != nil {
			panic(err)
		}

		println("Visit this URL to authorize:", authURL)

		// After user authorization, handle callback
		// err = oauthAuthenticator.HandleCallback(ctx, "authorization_code", "state")
		// if err != nil {
		//     panic(err)
		// }
	}

	// Get token info
	if tokenInfo, err := oauthAuthenticator.GetTokenInfo(); err == nil {
		println("Token expires at:", tokenInfo.ExpiresAt.Format(time.RFC3339))
		println("Token is expired:", tokenInfo.IsExpired)
	}
}

// TestMigrationPlaceholder shows migration functionality placeholder
// Note: This functionality is currently being refactored
func TestMigrationPlaceholder(t *testing.T) {
	// Migration helper functionality is being redesigned
	// This example is temporarily disabled
	t.Log("Migration helper example temporarily disabled during refactoring")
}

// ExampleNewAuthenticatorFactory shows how to use the authenticator factory
func ExampleNewAuthenticatorFactory() {
	// Create factory
	config := DefaultConfig()
	storage := NewMemoryTokenStorage(&config.TokenStorage.Memory)
	factory := NewAuthenticatorFactory(config, storage)

	// Create API key authenticator
	apiKeyAuth, err := factory.CreateAuthenticator("openai", types.AuthMethodAPIKey, "sk-1234567890abcdef")
	if err != nil {
		panic(err)
	}

	// Create OAuth authenticator (placeholder - OAuth config structure has changed)
	oauthAuth, err := factory.CreateAuthenticator("google", types.AuthMethodOAuth, &types.AuthConfig{
		Method: types.AuthMethodOAuth,
	})
	if err != nil {
		// OAuth configuration now uses OAuthCredentialSet in provider config
		println("OAuth authenticator creation:", err)
	}

	// Create custom authenticator
	customAuth := &CustomAuthenticator{}
	wrappedAuth, err := factory.CreateAuthenticator("custom", types.AuthMethodCustom, customAuth)
	if err != nil {
		panic(err)
	}

	// Use authenticators
	_ = apiKeyAuth
	_ = oauthAuth
	_ = wrappedAuth
}

// CustomAuthenticator is an example custom authenticator
type CustomAuthenticator struct {
	token string
}

func (c *CustomAuthenticator) Authenticate(ctx context.Context, config types.AuthConfig) error {
	c.token = config.APIKey
	return nil
}

func (c *CustomAuthenticator) IsAuthenticated() bool {
	return c.token != ""
}

func (c *CustomAuthenticator) GetToken() (string, error) {
	return c.token, nil
}

func (c *CustomAuthenticator) RefreshToken(ctx context.Context) error {
	return nil
}

func (c *CustomAuthenticator) Logout(ctx context.Context) error {
	c.token = ""
	return nil
}

func (c *CustomAuthenticator) GetAuthMethod() types.AuthMethod {
	return types.AuthMethodCustom
}

// ExampleNewFileTokenStorage shows how to use token encryption
func ExampleNewFileTokenStorage() {
	config := &EncryptionConfig{
		Enabled:   false, // Disabled for testing
		Key:       "my-encryption-key-32-bytes-long!",
		Algorithm: "aes-256-gcm",
		KeyDerivation: KeyDerivationConfig{
			Function:   "sha256",
			Iterations: 100000,
			KeyLength:  32,
		},
	}

	storageConfig := &FileStorageConfig{
		Directory:            "./test-tokens",
		FilePermissions:      "0600",
		DirectoryPermissions: "0700",
	}

	storage, err := NewFileTokenStorage(storageConfig, config)
	if err != nil {
		panic(err)
	}

	// Store encrypted token
	token := &types.OAuthConfig{
		AccessToken:  "secret-access-token",
		RefreshToken: "secret-refresh-token",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}

	err = storage.StoreToken("test-provider", token)
	if err != nil {
		panic(err)
	}

	// Retrieve and decrypt token
	retrievedToken, err := storage.RetrieveToken("test-provider")
	if err != nil {
		panic(err)
	}

	println("Retrieved token:", retrievedToken.AccessToken)

	// Get metadata
	metadata, err := storage.GetTokenInfo("test-provider")
	if err != nil {
		panic(err)
	}

	println("Token is encrypted:", metadata.IsEncrypted)
	println("Created at:", metadata.CreatedAt.Format(time.RFC3339))
}

// Test functions to ensure examples compile
func TestExampleAuthManager(t *testing.T) {
	ExampleNewAuthManager()
}

func TestExampleAPIKeyManager(t *testing.T) {
	ExampleAPIKeyManager()
}

func TestExampleOAuthAuthenticator(t *testing.T) {
	// This test is skipped because the OAuth authenticator requires a fully configured
	// OAuth provider (client ID, client secret, redirect URI, etc.) which is not available
	// in the test environment. OAuth configuration is now handled through OAuthCredentialSet
	// in the provider configuration.
	t.Skip("OAuth example requires full OAuth configuration which is not available in tests")
}

func TestExampleNewAuthenticatorFactory(t *testing.T) {
	ExampleNewAuthenticatorFactory()
}

func TestExampleNewFileTokenStorage(t *testing.T) {
	ExampleNewFileTokenStorage()
}
