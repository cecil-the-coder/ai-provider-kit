package auth

import (
	"context"
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// AuthenticatorFactory creates authenticators for different providers and methods
type AuthenticatorFactory struct {
	config  *Config
	storage TokenStorage
	logger  Logger
}

// NewAuthenticatorFactory creates a new authenticator factory
func NewAuthenticatorFactory(config *Config, storage TokenStorage) *AuthenticatorFactory {
	if config == nil {
		config = DefaultConfig()
	}

	return &AuthenticatorFactory{
		config:  config,
		storage: storage,
		logger:  &DefaultLogger{},
	}
}

// SetLogger sets the logger for the factory
func (f *AuthenticatorFactory) SetLogger(logger Logger) {
	f.logger = logger
}

// CreateAuthenticator creates an authenticator for the specified provider and method
func (f *AuthenticatorFactory) CreateAuthenticator(provider string, method types.AuthMethod, config interface{}) (Authenticator, error) {
	switch method {
	case types.AuthMethodOAuth:
		return f.createOAuthAuthenticator(provider, config)
	case types.AuthMethodAPIKey:
		return f.createAPIKeyAuthenticator(provider, config)
	case types.AuthMethodBearerToken:
		return f.createBearerTokenAuthenticator(provider, config)
	case types.AuthMethodCustom:
		return f.createCustomAuthenticator(provider, config)
	default:
		return nil, &AuthError{
			Provider: provider,
			Code:     ErrCodeInvalidConfig,
			Message:  fmt.Sprintf("Unsupported authentication method: %s", method),
		}
	}
}

// createOAuthAuthenticator creates an OAuth authenticator
func (f *AuthenticatorFactory) createOAuthAuthenticator(provider string, config interface{}) (Authenticator, error) {
	var oauthConfig *types.OAuthConfig

	switch c := config.(type) {
	case *types.OAuthConfig:
		oauthConfig = c
	default:
		return nil, &AuthError{
			Provider: provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "Invalid OAuth configuration type - use OAuthConfig directly",
		}
	}

	if oauthConfig == nil {
		return nil, &AuthError{
			Provider: provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "OAuth configuration is required",
		}
	}

	return NewOAuthAuthenticator(provider, f.storage, &f.config.OAuth), nil
}

// createAPIKeyAuthenticator creates an API key authenticator
func (f *AuthenticatorFactory) createAPIKeyAuthenticator(provider string, config interface{}) (Authenticator, error) {
	var keys []string
	var apiKeyConfig *APIKeyConfig

	switch c := config.(type) {
	case string:
		keys = []string{c}
		apiKeyConfig = &f.config.APIKey
	case []string:
		keys = c
		apiKeyConfig = &f.config.APIKey
	case *types.AuthConfig:
		if c.APIKey != "" {
			keys = []string{c.APIKey}
		}
		apiKeyConfig = &f.config.APIKey
	case APIKeyConfig:
		apiKeyConfig = &c
		if len(c.Strategy) > 0 {
			// Strategy provided in config
			_ = fmt.Sprintf("using strategy: %s", c.Strategy)
		}
	default:
		return nil, &AuthError{
			Provider: provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "Invalid API key configuration type",
		}
	}

	if len(keys) == 0 {
		return nil, &AuthError{
			Provider: provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "API keys are required",
		}
	}

	manager, err := NewAPIKeyManager(provider, keys, apiKeyConfig)
	if err != nil {
		return nil, &AuthError{
			Provider: provider,
			Code:     ErrCodeInvalidConfig,
			Message:  fmt.Sprintf("Failed to create API key manager: %v", err),
		}
	}

	return &APIKeyAuthenticatorImpl{
		provider: provider,
		manager:  manager,
		config:   apiKeyConfig,
	}, nil
}

// createBearerTokenAuthenticator creates a bearer token authenticator
func (f *AuthenticatorFactory) createBearerTokenAuthenticator(provider string, config interface{}) (Authenticator, error) {
	var token string

	switch c := config.(type) {
	case string:
		token = c
	case *types.AuthConfig:
		token = c.APIKey
	case types.AuthConfig:
		token = c.APIKey
	default:
		return nil, &AuthError{
			Provider: provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "Invalid bearer token configuration type",
		}
	}

	if token == "" {
		return nil, &AuthError{
			Provider: provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "Bearer token is required",
		}
	}

	return &BearerTokenAuthenticatorImpl{
		provider: provider,
		token:    token,
		isAuth:   true,
	}, nil
}

// createCustomAuthenticator creates a custom authenticator
func (f *AuthenticatorFactory) createCustomAuthenticator(provider string, config interface{}) (Authenticator, error) {
	// For custom authenticators, we expect the config to be an Authenticator instance
	if authenticator, ok := config.(Authenticator); ok {
		return authenticator, nil
	}

	return nil, &AuthError{
		Provider: provider,
		Code:     ErrCodeInvalidConfig,
		Message:  "Custom authenticator must implement the Authenticator interface",
	}
}

// APIKeyAuthenticatorImpl implements APIKeyAuthenticator
type APIKeyAuthenticatorImpl struct {
	provider string
	manager  APIKeyManager
	config   *APIKeyConfig
	isAuth   bool
}

// Authenticate performs authentication with the given config
func (a *APIKeyAuthenticatorImpl) Authenticate(ctx context.Context, config types.AuthConfig) error {
	if config.Method != types.AuthMethodAPIKey {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "API key authenticator only supports API key method",
		}
	}

	if config.APIKey == "" {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "API key is required",
		}
	}

	// For single API key mode, create a new manager
	if len(a.manager.GetKeys()) == 0 || config.APIKey != "" {
		keys := []string{config.APIKey}
		manager, err := NewAPIKeyManager(a.provider, keys, a.config)
		if err != nil {
			return &AuthError{
				Provider: a.provider,
				Code:     ErrCodeInvalidConfig,
				Message:  fmt.Sprintf("Failed to create API key manager: %v", err),
			}
		}
		a.manager = manager
	}

	a.isAuth = true
	return nil
}

// IsAuthenticated checks if currently authenticated
func (a *APIKeyAuthenticatorImpl) IsAuthenticated() bool {
	return a.isAuth && a.manager.IsHealthy()
}

// GetToken returns the current authentication token
func (a *APIKeyAuthenticatorImpl) GetToken() (string, error) {
	if !a.IsAuthenticated() {
		return "", &AuthError{
			Provider: a.provider,
			Code:     ErrCodeInvalidCredentials,
			Message:  "Not authenticated",
		}
	}

	return a.manager.GetCurrentKey(), nil
}

// RefreshToken refreshes the authentication token (no-op for API keys)
func (a *APIKeyAuthenticatorImpl) RefreshToken(ctx context.Context) error {
	// API keys don't need refresh
	return nil
}

// Logout clears authentication state
func (a *APIKeyAuthenticatorImpl) Logout(ctx context.Context) error {
	a.isAuth = false
	return nil
}

// GetAuthMethod returns the authentication method type
func (a *APIKeyAuthenticatorImpl) GetAuthMethod() types.AuthMethod {
	return types.AuthMethodAPIKey
}

// GetKeyManager returns the API key manager
func (a *APIKeyAuthenticatorImpl) GetKeyManager() APIKeyManager {
	return a.manager
}

// RotateKey rotates to the next available API key
func (a *APIKeyAuthenticatorImpl) RotateKey() error {
	_, err := a.manager.GetNextKey()
	return err
}

// ReportKeySuccess reports successful API key usage
func (a *APIKeyAuthenticatorImpl) ReportKeySuccess(key string) error {
	a.manager.ReportSuccess(key)
	return nil
}

// ReportKeyFailure reports failed API key usage
func (a *APIKeyAuthenticatorImpl) ReportKeyFailure(key string, err error) error {
	a.manager.ReportFailure(key, err)
	return nil
}

// BearerTokenAuthenticatorImpl implements simple bearer token authentication
type BearerTokenAuthenticatorImpl struct {
	provider string
	token    string
	isAuth   bool
}

// Authenticate performs authentication with the given config
func (b *BearerTokenAuthenticatorImpl) Authenticate(ctx context.Context, config types.AuthConfig) error {
	if config.Method != types.AuthMethodBearerToken {
		return &AuthError{
			Provider: b.provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "Bearer token authenticator only supports bearer token method",
		}
	}

	if config.APIKey == "" {
		return &AuthError{
			Provider: b.provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "Bearer token is required",
		}
	}

	b.token = config.APIKey
	b.isAuth = true
	return nil
}

// IsAuthenticated checks if currently authenticated
func (b *BearerTokenAuthenticatorImpl) IsAuthenticated() bool {
	return b.isAuth && b.token != ""
}

// GetToken returns the current authentication token
func (b *BearerTokenAuthenticatorImpl) GetToken() (string, error) {
	if !b.IsAuthenticated() {
		return "", &AuthError{
			Provider: b.provider,
			Code:     ErrCodeInvalidCredentials,
			Message:  "Not authenticated",
		}
	}

	return b.token, nil
}

// RefreshToken refreshes the authentication token (no-op for bearer tokens)
func (b *BearerTokenAuthenticatorImpl) RefreshToken(ctx context.Context) error {
	// Bearer tokens don't need refresh
	return nil
}

// Logout clears authentication state
func (b *BearerTokenAuthenticatorImpl) Logout(ctx context.Context) error {
	b.token = ""
	b.isAuth = false
	return nil
}

// GetAuthMethod returns the authentication method type
func (b *BearerTokenAuthenticatorImpl) GetAuthMethod() types.AuthMethod {
	return types.AuthMethodBearerToken
}

// ProviderAuthRegistry provides a registry of provider-specific authentication configurations
type ProviderAuthRegistry struct {
	providers map[string]*ProviderAuthConfig
}

// NewProviderAuthRegistry creates a new provider auth registry
func NewProviderAuthRegistry() *ProviderAuthRegistry {
	return &ProviderAuthRegistry{
		providers: make(map[string]*ProviderAuthConfig),
	}
}

// RegisterProvider registers a provider's authentication configuration
func (r *ProviderAuthRegistry) RegisterProvider(config *ProviderAuthConfig) {
	r.providers[config.Provider] = config
}

// GetProvider returns a provider's authentication configuration
func (r *ProviderAuthRegistry) GetProvider(provider string) (*ProviderAuthConfig, bool) {
	config, exists := r.providers[provider]
	return config, exists
}

// ListProviders returns a list of all registered providers
func (r *ProviderAuthRegistry) ListProviders() []string {
	providers := make([]string, 0, len(r.providers))
	for provider := range r.providers {
		providers = append(providers, provider)
	}
	return providers
}

// GetProvidersByMethod returns providers that support the specified authentication method
func (r *ProviderAuthRegistry) GetProvidersByMethod(method types.AuthMethod) []string {
	var providers []string
	for _, config := range r.providers {
		switch method {
		case types.AuthMethodAPIKey:
			if config.Features.SupportsAPIKey {
				providers = append(providers, config.Provider)
			}
		case types.AuthMethodOAuth:
			if config.Features.SupportsOAuth {
				providers = append(providers, config.Provider)
			}
		case types.AuthMethodBearerToken:
			if config.Features.SupportsAPIKey { // Bearer token uses API key field
				providers = append(providers, config.Provider)
			}
		}
	}
	return providers
}

// CreateStandardRegistry creates a registry with standard provider configurations
func CreateStandardRegistry() *ProviderAuthRegistry {
	registry := NewProviderAuthRegistry()

	// OpenAI
	registry.RegisterProvider(&ProviderAuthConfig{
		Provider:    "openai",
		AuthMethod:  types.AuthMethodAPIKey,
		DisplayName: "OpenAI",
		Description: "OpenAI API authentication",
		Features: AuthFeatureFlags{
			SupportsAPIKey:   true,
			SupportsMultiKey: true,
			SupportsOAuth:    false,
		},
	})

	// Anthropic
	registry.RegisterProvider(&ProviderAuthConfig{
		Provider:    "anthropic",
		AuthMethod:  types.AuthMethodAPIKey,
		DisplayName: "Anthropic",
		Description: "Anthropic Claude API authentication",
		Features: AuthFeatureFlags{
			SupportsAPIKey:   true,
			SupportsMultiKey: true,
			SupportsOAuth:    false,
		},
	})

	// Google Gemini
	registry.RegisterProvider(&ProviderAuthConfig{
		Provider:       "gemini",
		AuthMethod:     types.AuthMethodOAuth,
		DisplayName:    "Google Gemini",
		Description:    "Google Gemini API authentication",
		OAuthURL:       "https://accounts.google.com/o/oauth2/v2/auth",
		RequiredScopes: []string{"https://www.googleapis.com/auth/generative-language"},
		Features: AuthFeatureFlags{
			SupportsAPIKey:   true,
			SupportsOAuth:    true,
			SupportsMultiKey: true,
			RequiresPKCE:     true,
			TokenRefresh:     true,
		},
	})

	// OpenRouter
	registry.RegisterProvider(&ProviderAuthConfig{
		Provider:    "openrouter",
		AuthMethod:  types.AuthMethodAPIKey,
		DisplayName: "OpenRouter",
		Description: "OpenRouter API authentication",
		Features: AuthFeatureFlags{
			SupportsAPIKey:   true,
			SupportsMultiKey: true,
			SupportsOAuth:    false,
		},
	})

	return registry
}
