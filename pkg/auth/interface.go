// Package auth provides authentication and authorization utilities for AI providers.
// It includes API key management, OAuth flows, security helpers, and credential storage.
package auth

import (
	"context"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Authenticator defines the interface for authentication methods
type Authenticator interface {
	// Authenticate performs authentication with the given config
	Authenticate(ctx context.Context, config types.AuthConfig) error

	// IsAuthenticated checks if currently authenticated
	IsAuthenticated() bool

	// GetToken returns the current authentication token
	GetToken() (string, error)

	// RefreshToken refreshes the authentication token if needed
	RefreshToken(ctx context.Context) error

	// Logout clears authentication state
	Logout(ctx context.Context) error

	// GetAuthMethod returns the authentication method type
	GetAuthMethod() types.AuthMethod
}

// OAuthAuthenticator extends Authenticator for OAuth-specific functionality
type OAuthAuthenticator interface {
	Authenticator

	// StartOAuthFlow initiates the OAuth flow and returns the auth URL
	StartOAuthFlow(ctx context.Context, scopes []string) (string, error)

	// HandleCallback processes the OAuth callback
	HandleCallback(ctx context.Context, code, state string) error

	// IsOAuthEnabled checks if OAuth is properly configured
	IsOAuthEnabled() bool

	// GetTokenInfo returns detailed token information
	GetTokenInfo() (*TokenInfo, error)
}

// APIKeyAuthenticator extends Authenticator for API key-specific functionality
type APIKeyAuthenticator interface {
	Authenticator

	// GetKeyManager returns the API key manager for multi-key operations
	GetKeyManager() APIKeyManager

	// RotateKey rotates to the next available API key
	RotateKey() error

	// ReportKeySuccess reports successful API key usage
	ReportKeySuccess(key string) error

	// ReportKeyFailure reports failed API key usage
	ReportKeyFailure(key string, err error) error
}

// TokenStorage defines the interface for token persistence
// Extends the kit version with additional functionality
type TokenStorage interface {
	types.TokenStorage

	// IsTokenValid checks if a token exists and is not expired
	IsTokenValid(key string) bool

	// CleanupExpired removes all expired tokens
	CleanupExpired() error

	// GetTokenInfo returns metadata about a stored token
	GetTokenInfo(key string) (*TokenMetadata, error)
}

// AuthManager manages authentication for multiple providers
type AuthManager interface {
	// RegisterAuthenticator registers an authenticator for a provider
	RegisterAuthenticator(provider string, authenticator Authenticator) error

	// GetAuthenticator returns the authenticator for a provider
	GetAuthenticator(provider string) (Authenticator, error)

	// Authenticate authenticates a provider
	Authenticate(ctx context.Context, provider string, config types.AuthConfig) error

	// IsAuthenticated checks if a provider is authenticated
	IsAuthenticated(provider string) bool

	// Logout logs out a provider
	Logout(ctx context.Context, provider string) error

	// RefreshAllTokens refreshes all tokens that need it
	RefreshAllTokens(ctx context.Context) error

	// GetAuthenticatedProviders returns a list of authenticated providers
	GetAuthenticatedProviders() []string

	// GetAuthStatus returns the authentication status for all providers
	GetAuthStatus() map[string]*AuthState

	// CleanupExpired removes expired tokens and cleans up authenticators
	CleanupExpired() error

	// ForEachAuthenticated executes a function for each authenticated provider
	ForEachAuthenticated(ctx context.Context, fn func(provider string, authenticator Authenticator) error) error

	// GetTokenInfo returns token information for a specific provider
	GetTokenInfo(provider string) (*TokenInfo, error)

	// StartOAuthFlow starts OAuth flow for a provider
	StartOAuthFlow(ctx context.Context, provider string, scopes []string) (string, error)

	// HandleOAuthCallback handles OAuth callback for a provider
	HandleOAuthCallback(ctx context.Context, provider string, code, state string) error
}

// APIKeyManager manages multiple API keys with load balancing and failover
type APIKeyManager interface {
	// GetCurrentKey returns the first available API key without advancing the round-robin counter
	GetCurrentKey() string

	// GetNextKey returns the next available API key using round-robin load balancing
	GetNextKey() (string, error)

	// ReportSuccess reports that an API call succeeded with this key
	ReportSuccess(key string)

	// ReportFailure reports that an API call failed with this key
	ReportFailure(key string, err error)

	// ExecuteWithFailover attempts an operation with automatic failover to next key on failure
	ExecuteWithFailover(operation func(apiKey string) (string, error)) (string, error)

	// GetStatus returns the current health status of all keys
	GetStatus() map[string]interface{}

	// GetKeys returns all configured keys (masked for security)
	GetKeys() []string

	// AddKey adds a new API key to the manager
	AddKey(key string) error

	// RemoveKey removes an API key from the manager
	RemoveKey(key string) error

	// IsHealthy returns true if at least one key is healthy
	IsHealthy() bool
}

// TokenInfo represents information about an authentication token
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       []string  `json:"scopes"`
	IsExpired    bool      `json:"is_expired"`
	ExpiresIn    int64     `json:"expires_in"`
}

// AuthState represents the current authentication state
type AuthState struct {
	Provider      string           `json:"provider"`
	Authenticated bool             `json:"authenticated"`
	Method        types.AuthMethod `json:"method"`
	LastAuth      time.Time        `json:"last_auth"`
	ExpiresAt     time.Time        `json:"expires_at,omitempty"`
	CanRefresh    bool             `json:"can_refresh"`
}

// TokenMetadata represents metadata about a stored token
type TokenMetadata struct {
	Provider     string    `json:"provider"`
	CreatedAt    time.Time `json:"created_at"`
	LastAccessed time.Time `json:"last_accessed"`
	ExpiresAt    time.Time `json:"expires_at"`
	IsEncrypted  bool      `json:"is_encrypted"`
}

// ProviderAuthConfig represents provider-specific authentication configuration
type ProviderAuthConfig struct {
	Provider       string           `json:"provider"`
	AuthMethod     types.AuthMethod `json:"auth_method"`
	DisplayName    string           `json:"display_name"`
	Description    string           `json:"description"`
	OAuthURL       string           `json:"oauth_url,omitempty"`
	RequiredScopes []string         `json:"required_scopes,omitempty"`
	OptionalScopes []string         `json:"optional_scopes,omitempty"`
	Features       AuthFeatureFlags `json:"features"`
}

// AuthFeatureFlags represents authentication feature flags for a provider
type AuthFeatureFlags struct {
	SupportsOAuth    bool `json:"supports_oauth"`
	SupportsAPIKey   bool `json:"supports_api_key"`
	SupportsMultiKey bool `json:"supports_multi_key"`
	RequiresPKCE     bool `json:"requires_pkce"`
	TokenRefresh     bool `json:"token_refresh"`
}

// AuthError represents authentication-related errors
type AuthError struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Details  string `json:"details,omitempty"`
	Retry    bool   `json:"retry,omitempty"`
}

func (e *AuthError) Error() string {
	if e.Details != "" {
		return e.Provider + " authentication error: " + e.Message + " (" + e.Code + ") - " + e.Details
	}
	return e.Provider + " authentication error: " + e.Message + " (" + e.Code + ")"
}

// IsRetryable returns true if the error is retryable
func (e *AuthError) IsRetryable() bool {
	return e.Retry
}

// Common error codes
const (
	//nolint:gosec // This is an error code string, not a hardcoded credential
	ErrCodeInvalidCredentials  = "invalid_credentials"
	ErrCodeTokenExpired        = "token_expired"
	ErrCodeRefreshFailed       = "refresh_failed"
	ErrCodeOAuthFlowFailed     = "oauth_flow_failed"
	ErrCodeInvalidConfig       = "invalid_config"
	ErrCodeNetworkError        = "network_error"
	ErrCodeProviderUnavailable = "provider_unavailable"
	ErrCodeScopeInsufficient   = "scope_insufficient"
	ErrCodeKeyRotationFailed   = "key_rotation_failed"
	ErrCodeAllKeysExhausted    = "all_keys_exhausted"
	ErrCodeStorageError        = "storage_error"
	ErrCodeEncryptionError     = "encryption_error"
)
