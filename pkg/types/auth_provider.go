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

// =============================================================================
// Credential Provider Interface
// =============================================================================

// CredentialProvider is an interface for dynamically providing OAuth credentials
// This allows external systems to manage credential storage and provide fresh
// credentials on-demand, rather than relying on cached credentials.
//
// When a CredentialProvider is configured, the OAuthKeyManager will call
// GetCredentials() to fetch the current credentials instead of using its
// internal cached copy. This ensures that any external token refresh
// (e.g., from a separate OAuth extension) is immediately visible.
type CredentialProvider interface {
	// GetCredentials returns the current OAuth credentials for a provider
	// The provider name is used to look up credentials in the storage
	// Returns an empty slice if no credentials are available
	GetCredentials(ctx context.Context, providerName string) ([]*OAuthCredentialSet, error)

	// UpdateCredential updates a specific credential after token refresh
	// This is called by OAuthKeyManager after it refreshes a token
	// The implementation should persist the updated credential
	UpdateCredential(ctx context.Context, providerName string, credential *OAuthCredentialSet) error
}

// CredentialProviderAware is an interface for providers that support dynamic credential providers
// Providers implementing this interface can have their OAuth credentials managed externally
type CredentialProviderAware interface {
	// SetCredentialProvider sets a dynamic credential provider for OAuth credentials
	SetCredentialProvider(provider CredentialProvider)
}

// AsCredentialProviderAware safely casts a provider to CredentialProviderAware interface
func AsCredentialProviderAware(provider Provider) (CredentialProviderAware, bool) {
	aware, ok := provider.(CredentialProviderAware)
	return aware, ok
}
