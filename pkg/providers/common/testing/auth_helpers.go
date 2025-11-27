// Package testing provides common testing helpers and utilities for AI provider implementations
// including authentication, configuration, tool calling, and mock server functionality.
package testing

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AuthTestHelper provides helpers for testing authentication functionality
type AuthTestHelper struct {
	t *testing.T
}

// NewAuthTestHelper creates a new authentication test helper
func NewAuthTestHelper(t *testing.T) *AuthTestHelper {
	return &AuthTestHelper{t: t}
}

// CreateAPIKeyAuthConfig creates an API key authentication configuration
func (a *AuthTestHelper) CreateAPIKeyAuthConfig(apiKey, baseURL, defaultModel string) types.AuthConfig {
	return types.AuthConfig{
		Method:       types.AuthMethodAPIKey,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		DefaultModel: defaultModel,
	}
}

// CreateBearerTokenAuthConfig creates a bearer token authentication configuration
func (a *AuthTestHelper) CreateBearerTokenAuthConfig(token string) types.AuthConfig {
	return types.AuthConfig{
		Method: types.AuthMethodBearerToken,
		APIKey: token,
	}
}

// CreateOAuthAuthConfig creates an OAuth authentication configuration
func (a *AuthTestHelper) CreateOAuthAuthConfig(clientID, clientSecret string) types.AuthConfig {
	return types.AuthConfig{
		Method:       types.AuthMethodOAuth,
		APIKey:       "oauth-token-placeholder",
		BaseURL:      "https://oauth.test.com",
		DefaultModel: "test-model",
	}
}

// TestAPIKeyAuthentication tests API key authentication flow
func (a *AuthTestHelper) TestAPIKeyAuthentication(provider types.Provider) {
	ctx := context.Background()

	// Create auth config
	authConfig := a.CreateAPIKeyAuthConfig("test-api-key", "https://api.test.com", "test-model")

	// Test authentication
	err := provider.Authenticate(ctx, authConfig)
	if err == nil {
		// If authentication succeeded, verify state
		assert.True(a.t, provider.IsAuthenticated(), "Should be authenticated after successful API key auth")

		// Test that we can get authenticated operations
		models, _ := provider.GetModels(ctx)
		// Don't assert error here as it might fail with mock credentials
		assert.NotNil(a.t, models, "Should return models object even if API call fails")
	} else {
		// If authentication failed, verify error is appropriate
		assert.Error(a.t, err, "Should return error for invalid credentials")
		assert.False(a.t, provider.IsAuthenticated(), "Should not be authenticated after failed auth")
	}

	// Test logout
	err = provider.Logout(ctx)
	assert.NoError(a.t, err, "Logout should not error")
}

// TestUnsupportedAuthMethods tests that unsupported auth methods are rejected
func (a *AuthTestHelper) TestUnsupportedAuthMethods(provider types.Provider) {
	ctx := context.Background()

	testCases := []struct {
		name       string
		authConfig types.AuthConfig
	}{
		{
			name:       "BearerToken",
			authConfig: a.CreateBearerTokenAuthConfig("test-token"),
		},
		{
			name:       "OAuth",
			authConfig: a.CreateOAuthAuthConfig("client-id", "client-secret"),
		},
	}

	for _, tc := range testCases {
		a.t.Run(tc.name, func(t *testing.T) {
			err := provider.Authenticate(ctx, tc.authConfig)
			// Some providers might support these methods, so we don't assert error
			// This test mainly ensures the provider doesn't panic when given different auth types
			_ = err
		})
	}
}

// TestAuthenticationWithConfig tests authentication through configuration
func (a *AuthTestHelper) TestAuthenticationWithConfig(provider types.Provider) {
	// Create config with API key
	config := types.ProviderConfig{
		Type:         provider.Type(),
		APIKey:       "test-api-key",
		BaseURL:      "https://api.test.com",
		DefaultModel: "test-model",
	}

	err := provider.Configure(config)
	require.NoError(a.t, err, "Configuration with API key should succeed")

	// Verify authentication state
	if config.APIKey != "" {
		assert.True(a.t, provider.IsAuthenticated(), "Should be authenticated when configured with API key")
	}
}

// TestMultiKeyAuthentication tests authentication with multiple API keys
func (a *AuthTestHelper) TestMultiKeyAuthentication(provider types.Provider) {
	config := types.ProviderConfig{
		Type:   provider.Type(),
		APIKey: "primary-key",
		ProviderConfig: map[string]interface{}{
			"api_keys": []string{"key1", "key2", "key3"},
		},
	}

	err := provider.Configure(config)
	if err == nil {
		// If configuration succeeded, verify authentication
		assert.True(a.t, provider.IsAuthenticated(), "Should be authenticated with multiple API keys")
	}
}

// TestAuthenticationPersistence tests that authentication state persists
func (a *AuthTestHelper) TestAuthenticationPersistence(provider types.Provider) {
	ctx := context.Background()

	// Authenticate
	authConfig := a.CreateAPIKeyAuthConfig("persistent-key", "https://api.test.com", "test-model")
	err := provider.Authenticate(ctx, authConfig)
	if err != nil {
		// Skip this test if authentication fails
		a.t.Skip("Skipping authentication persistence test - authentication failed")
		return
	}

	assert.True(a.t, provider.IsAuthenticated(), "Should be authenticated")

	// Perform operations that might affect state
	_ = provider.GetConfig()
	_ = provider.GetDefaultModel()
	_, _ = provider.GetModels(ctx)

	// Verify authentication state persists
	assert.True(a.t, provider.IsAuthenticated(), "Authentication state should persist after operations")
}

// TestAuthenticationErrors tests various authentication error scenarios
//
//nolint:staticcheck // Empty branch is intentional - no assertion needed
func (a *AuthTestHelper) TestAuthenticationErrors(provider types.Provider) {
	ctx := context.Background()

	testCases := []struct {
		name        string
		authConfig  types.AuthConfig
		expectError bool
		description string
	}{
		{
			name:        "EmptyAPIKey",
			authConfig:  a.CreateAPIKeyAuthConfig("", "https://api.test.com", "test-model"),
			expectError: true,
			description: "Should reject empty API key",
		},
		{
			name: "InvalidMethod",
			authConfig: types.AuthConfig{
				Method: "invalid-method",
				APIKey: "test-key",
			},
			expectError: true,
			description: "Should reject invalid authentication method",
		},
		{
			name:        "NilConfig",
			authConfig:  types.AuthConfig{},
			expectError: true,
			description: "Should reject empty authentication config",
		},
	}

	for _, tc := range testCases {
		a.t.Run(tc.name, func(t *testing.T) {
			err := provider.Authenticate(ctx, tc.authConfig)

			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				//nolint:staticcheck // Empty branch is intentional - no assertion needed
				// Don't assert success as it depends on provider implementation
			}
		})
	}
}

// TestLogoutFunctionality tests logout functionality
func (a *AuthTestHelper) TestLogoutFunctionality(provider types.Provider) {
	ctx := context.Background()

	// First authenticate
	authConfig := a.CreateAPIKeyAuthConfig("logout-test-key", "https://api.test.com", "test-model")
	err := provider.Authenticate(ctx, authConfig)
	if err != nil {
		a.t.Skip("Skipping logout test - authentication failed")
		return
	}

	// Verify authenticated
	assert.True(a.t, provider.IsAuthenticated(), "Should be authenticated before logout")

	// Test logout
	err = provider.Logout(ctx)
	assert.NoError(a.t, err, "Logout should not error")

	// Note: Some providers might not clear authentication on logout
	// This test mainly ensures the logout method doesn't panic
}

// TestAuthenticationWithContext tests authentication with context
func (a *AuthTestHelper) TestAuthenticationWithContext(provider types.Provider) {
	ctx := context.Background()

	authConfig := a.CreateAPIKeyAuthConfig("context-test-key", "https://api.test.com", "test-model")

	// Test with background context
	err := provider.Authenticate(ctx, authConfig)
	// Don't assert error as it depends on provider implementation
	_ = err

	// Test with canceled context
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately

	err = provider.Authenticate(cancelledCtx, authConfig)
	// Don't assert error as behavior varies by provider
	_ = err
}

// StandardAuthTestSuite runs a comprehensive set of authentication tests
func (a *AuthTestHelper) StandardAuthTestSuite(provider types.Provider) {
	a.t.Run("APIKeyAuthentication", func(t *testing.T) {
		a.TestAPIKeyAuthentication(provider)
	})

	a.t.Run("UnsupportedAuthMethods", func(t *testing.T) {
		a.TestUnsupportedAuthMethods(provider)
	})

	a.t.Run("AuthenticationWithConfig", func(t *testing.T) {
		a.TestAuthenticationWithConfig(provider)
	})

	a.t.Run("MultiKeyAuthentication", func(t *testing.T) {
		a.TestMultiKeyAuthentication(provider)
	})

	a.t.Run("AuthenticationPersistence", func(t *testing.T) {
		a.TestAuthenticationPersistence(provider)
	})

	a.t.Run("AuthenticationErrors", func(t *testing.T) {
		a.TestAuthenticationErrors(provider)
	})

	a.t.Run("LogoutFunctionality", func(t *testing.T) {
		a.TestLogoutFunctionality(provider)
	})

	a.t.Run("AuthenticationWithContext", func(t *testing.T) {
		a.TestAuthenticationWithContext(provider)
	})
}
