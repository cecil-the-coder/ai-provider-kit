package types

import (
	"context"
	"time"
)

// TokenInfo represents authentication token information for OAuth providers
// This structure provides comprehensive token validation and metadata
type TokenInfo struct {
	// Valid indicates whether the token is currently valid
	Valid bool `json:"valid"`

	// ExpiresAt indicates when the token expires
	ExpiresAt time.Time `json:"expires_at"`

	// Scope contains the OAuth scopes granted to the token
	Scope []string `json:"scope"`

	// UserInfo contains user information from the OAuth provider
	UserInfo map[string]interface{} `json:"user_info"`
}

// IsExpired returns true if the token has expired
func (ti *TokenInfo) IsExpired() bool {
	return time.Now().After(ti.ExpiresAt)
}

// HasScope returns true if the token includes the specified scope
func (ti *TokenInfo) HasScope(scope string) bool {
	for _, s := range ti.Scope {
		if s == scope {
			return true
		}
	}
	return false
}

// OAuthProvider extends the Provider interface with OAuth-specific authentication methods.
// This interface should be implemented by providers that use OAuth authentication
// and need token management capabilities.
type OAuthProvider interface {
	// Embed the full Provider interface to maintain compatibility
	Provider

	// ValidateToken validates the current OAuth token and returns detailed token information
	// This method should check token expiration, scope validity, and retrieve user info
	ValidateToken(ctx context.Context) (*TokenInfo, error)

	// RefreshToken refreshes the OAuth token using the stored refresh token
	// This method should update the internal token state and persist the new tokens
	RefreshToken(ctx context.Context) error

	// GetAuthURL generates an OAuth authorization URL for re-authentication
	// redirectURI: The URI where the OAuth provider should redirect after authentication
	// state: A random string to prevent CSRF attacks
	// Returns the complete authorization URL that the user should be redirected to
	GetAuthURL(redirectURI string, state string) string
}

// TestableProvider extends the Provider interface with connectivity testing capabilities.
// This interface should be implemented by providers that need to test their
// connection to the underlying service independently of health checks.
type TestableProvider interface {
	// Embed the full Provider interface to maintain compatibility
	Provider

	// TestConnectivity performs a connectivity test to verify the provider can reach its service
	// This method should perform a lightweight operation to verify network connectivity,
	// authentication, and basic service availability. It should not perform heavy operations
	// that might impact rate limits or incur costs.
	// Returns an error if connectivity cannot be established
	TestConnectivity(ctx context.Context) error
}

// =============================================================================
// Provider Type Guards and Utilities
// =============================================================================

// IsOAuthProvider checks if a provider implements OAuthProvider interface
func IsOAuthProvider(provider Provider) bool {
	_, ok := provider.(OAuthProvider)
	return ok
}

// IsTestableProvider checks if a provider implements TestableProvider interface
func IsTestableProvider(provider Provider) bool {
	_, ok := provider.(TestableProvider)
	return ok
}

// AsOAuthProvider safely casts a provider to OAuthProvider interface
func AsOAuthProvider(provider Provider) (OAuthProvider, bool) {
	oauthProvider, ok := provider.(OAuthProvider)
	return oauthProvider, ok
}

// AsTestableProvider safely casts a provider to TestableProvider interface
func AsTestableProvider(provider Provider) (TestableProvider, bool) {
	testableProvider, ok := provider.(TestableProvider)
	return testableProvider, ok
}
