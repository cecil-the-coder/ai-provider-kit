package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// AuthManagerImpl manages authentication for multiple providers
type AuthManagerImpl struct {
	authenticators map[string]Authenticator
	storage        TokenStorage
	config         *Config
	mutex          sync.RWMutex
	cleanupTicker  *time.Ticker
	logger         Logger
}

// Logger interface for authentication events
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// DefaultLogger provides a simple no-op logger
type DefaultLogger struct{}

func (l *DefaultLogger) Debug(msg string, fields ...interface{}) {}
func (l *DefaultLogger) Info(msg string, fields ...interface{})  {}
func (l *DefaultLogger) Warn(msg string, fields ...interface{})  {}
func (l *DefaultLogger) Error(msg string, fields ...interface{}) {}

// NewAuthManager creates a new authentication manager
func NewAuthManager(storage TokenStorage, config *Config) *AuthManagerImpl {
	if config == nil {
		config = DefaultConfig()
	}

	manager := &AuthManagerImpl{
		authenticators: make(map[string]Authenticator),
		storage:        storage,
		config:         config,
		logger:         &DefaultLogger{},
	}

	// Start cleanup ticker
	if config.TokenStorage.File.Backup.Enabled {
		manager.startCleanupTicker()
	}

	return manager
}

// SetLogger sets the logger for the auth manager
func (am *AuthManagerImpl) SetLogger(logger Logger) {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	am.logger = logger
}

// RegisterAuthenticator registers an authenticator for a provider
func (am *AuthManagerImpl) RegisterAuthenticator(provider string, authenticator Authenticator) error {
	if provider == "" {
		return &AuthError{
			Code:    ErrCodeInvalidConfig,
			Message: "provider name cannot be empty",
		}
	}
	if authenticator == nil {
		return &AuthError{
			Code:    ErrCodeInvalidConfig,
			Message: "authenticator cannot be nil",
		}
	}

	am.mutex.Lock()
	defer am.mutex.Unlock()

	am.authenticators[provider] = authenticator
	am.logger.Info("Registered authenticator for provider", "provider", provider)
	return nil
}

// GetAuthenticator returns the authenticator for a provider
func (am *AuthManagerImpl) GetAuthenticator(provider string) (Authenticator, error) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	authenticator, exists := am.authenticators[provider]
	if !exists {
		return nil, &AuthError{
			Provider: provider,
			Code:     ErrCodeProviderUnavailable,
			Message:  "No authenticator registered for provider",
		}
	}

	return authenticator, nil
}

// Authenticate authenticates a provider
func (am *AuthManagerImpl) Authenticate(ctx context.Context, provider string, config types.AuthConfig) error {
	authenticator, err := am.GetAuthenticator(provider)
	if err != nil {
		return err
	}

	am.logger.Info("Authenticating provider", "provider", provider, "method", config.Method)

	err = authenticator.Authenticate(ctx, config)
	if err != nil {
		am.logger.Error("Authentication failed", "provider", provider, "error", err.Error())
		return err
	}

	am.logger.Info("Authentication successful", "provider", provider)
	return nil
}

// IsAuthenticated checks if a provider is authenticated
func (am *AuthManagerImpl) IsAuthenticated(provider string) bool {
	authenticator, err := am.GetAuthenticator(provider)
	if err != nil {
		return false
	}

	return authenticator.IsAuthenticated()
}

// Logout logs out a provider
func (am *AuthManagerImpl) Logout(ctx context.Context, provider string) error {
	authenticator, err := am.GetAuthenticator(provider)
	if err != nil {
		return err
	}

	am.logger.Info("Logging out provider", "provider", provider)

	err = authenticator.Logout(ctx)
	if err != nil {
		am.logger.Error("Logout failed", "provider", provider, "error", err.Error())
		return err
	}

	am.logger.Info("Logout successful", "provider", provider)
	return nil
}

// RefreshAllTokens refreshes all tokens that need it
func (am *AuthManagerImpl) RefreshAllTokens(ctx context.Context) error {
	am.mutex.RLock()
	authenticators := make(map[string]Authenticator)
	for provider, auth := range am.authenticators {
		authenticators[provider] = auth
	}
	am.mutex.RUnlock()

	am.logger.Info("Refreshing all tokens", "providers", len(authenticators))

	var errors []error
	refreshedCount := 0

	for provider, authenticator := range authenticators {
		if authenticator.IsAuthenticated() {
			err := authenticator.RefreshToken(ctx)
			if err != nil {
				am.logger.Warn("Token refresh failed", "provider", provider, "error", err.Error())
				errors = append(errors, fmt.Errorf("failed to refresh %s: %w", provider, err))
			} else {
				refreshedCount++
				am.logger.Debug("Token refreshed successfully", "provider", provider)
			}
		}
	}

	am.logger.Info("Token refresh completed", "refreshed", refreshedCount, "failed", len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("refresh errors: %v", errors)
	}

	return nil
}

// GetAuthenticatedProviders returns a list of authenticated providers
func (am *AuthManagerImpl) GetAuthenticatedProviders() []string {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	var authenticated []string
	for provider, authenticator := range am.authenticators {
		if authenticator.IsAuthenticated() {
			authenticated = append(authenticated, provider)
		}
	}

	return authenticated
}

// GetAuthStatus returns the authentication status for all providers
func (am *AuthManagerImpl) GetAuthStatus() map[string]*AuthState {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	status := make(map[string]*AuthState)
	for provider, authenticator := range am.authenticators {
		state := &AuthState{
			Provider: provider,
		}

		if auth := authenticator.IsAuthenticated(); auth {
			state.Authenticated = true
			state.Method = authenticator.GetAuthMethod()
			state.LastAuth = time.Now()

			// Get token info if available
			if oauthAuth, ok := authenticator.(OAuthAuthenticator); ok {
				if tokenInfo, err := oauthAuth.GetTokenInfo(); err == nil {
					state.ExpiresAt = tokenInfo.ExpiresAt
					state.CanRefresh = tokenInfo.RefreshToken != ""
				}
			}
		}

		status[provider] = state
	}

	return status
}

// CleanupExpired removes expired tokens and cleans up authenticators
func (am *AuthManagerImpl) CleanupExpired() error {
	am.mutex.RLock()
	logger := am.logger
	am.mutex.RUnlock()

	logger.Info("Cleaning up expired tokens")

	// Get stored tokens
	storedTokens, err := am.storage.ListTokens()
	if err != nil {
		logger.Error("Failed to list stored tokens", "error", err.Error())
		return fmt.Errorf("failed to list stored tokens: %w", err)
	}

	var expired []string
	for _, provider := range storedTokens {
		if !am.storage.IsTokenValid(provider) {
			expired = append(expired, provider)
		}
	}

	// Remove expired tokens
	for _, provider := range expired {
		if err := am.storage.DeleteToken(provider); err != nil {
			logger.Error("Failed to delete expired token", "provider", provider, "error", err.Error())
			return fmt.Errorf("failed to delete expired token for %s: %w", provider, err)
		}
		logger.Debug("Deleted expired token", "provider", provider)
	}

	// Clean up authenticators for expired tokens
	am.mutex.RLock()
	for _, provider := range expired {
		if authenticator, exists := am.authenticators[provider]; exists {
			if !authenticator.IsAuthenticated() {
				// Authenticator is no longer valid, could be cleaned up
				logger.Debug("Authenticator no longer valid", "provider", provider)
			}
		}
	}
	am.mutex.RUnlock()

	logger.Info("Cleanup completed", "removed", len(expired))
	return nil
}

// ForEachAuthenticated executes a function for each authenticated provider
func (am *AuthManagerImpl) ForEachAuthenticated(ctx context.Context, fn func(provider string, authenticator Authenticator) error) error {
	authenticated := am.GetAuthenticatedProviders()

	am.logger.Debug("Executing function for authenticated providers", "count", len(authenticated))

	for _, provider := range authenticated {
		authenticator, err := am.GetAuthenticator(provider)
		if err != nil {
			am.logger.Warn("Failed to get authenticator", "provider", provider, "error", err.Error())
			continue
		}

		if err := fn(provider, authenticator); err != nil {
			am.logger.Error("Function execution failed", "provider", provider, "error", err.Error())
			return fmt.Errorf("error processing %s: %w", provider, err)
		}
	}

	return nil
}

// GetTokenInfo returns token information for a specific provider
func (am *AuthManagerImpl) GetTokenInfo(provider string) (*TokenInfo, error) {
	authenticator, err := am.GetAuthenticator(provider)
	if err != nil {
		return nil, err
	}

	if oauthAuth, ok := authenticator.(OAuthAuthenticator); ok {
		return oauthAuth.GetTokenInfo()
	}

	return nil, &AuthError{
		Provider: provider,
		Code:     ErrCodeInvalidConfig,
		Message:  "Provider does not support OAuth token info",
	}
}

// StartOAuthFlow starts OAuth flow for a provider
func (am *AuthManagerImpl) StartOAuthFlow(ctx context.Context, provider string, scopes []string) (string, error) {
	authenticator, err := am.GetAuthenticator(provider)
	if err != nil {
		return "", err
	}

	if oauthAuth, ok := authenticator.(OAuthAuthenticator); ok {
		am.logger.Info("Starting OAuth flow", "provider", provider, "scopes", scopes)
		return oauthAuth.StartOAuthFlow(ctx, scopes)
	}

	return "", &AuthError{
		Provider: provider,
		Code:     ErrCodeInvalidConfig,
		Message:  "Provider does not support OAuth flow",
	}
}

// HandleOAuthCallback handles OAuth callback for a provider
func (am *AuthManagerImpl) HandleOAuthCallback(ctx context.Context, provider string, code, state string) error {
	authenticator, err := am.GetAuthenticator(provider)
	if err != nil {
		return err
	}

	if oauthAuth, ok := authenticator.(OAuthAuthenticator); ok {
		am.logger.Info("Handling OAuth callback", "provider", provider)
		return oauthAuth.HandleCallback(ctx, code, state)
	}

	return &AuthError{
		Provider: provider,
		Code:     ErrCodeInvalidConfig,
		Message:  "Provider does not support OAuth callback",
	}
}

// GetStorage returns the token storage instance
func (am *AuthManagerImpl) GetStorage() TokenStorage {
	return am.storage
}

// GetConfig returns the auth manager configuration
func (am *AuthManagerImpl) GetConfig() *Config {
	return am.config
}

// GetProviders returns a list of all registered providers
func (am *AuthManagerImpl) GetProviders() []string {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	providers := make([]string, 0, len(am.authenticators))
	for provider := range am.authenticators {
		providers = append(providers, provider)
	}
	return providers
}

// RemoveAuthenticator removes an authenticator for a provider
func (am *AuthManagerImpl) RemoveAuthenticator(provider string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if _, exists := am.authenticators[provider]; !exists {
		return &AuthError{
			Provider: provider,
			Code:     ErrCodeProviderUnavailable,
			Message:  "No authenticator registered for provider",
		}
	}

	// Logout before removing
	ctx := context.Background()
	if authenticator := am.authenticators[provider]; authenticator != nil {
		if err := authenticator.Logout(ctx); err != nil {
			// Log the error but don't fail the operation - logout during cleanup is often non-critical
			am.logger.Warn("Logout failed during removal operation", "provider", provider, "error", err.Error())
		} else {
			am.logger.Debug("Logout successful during removal", "provider", provider)
		}
	}

	delete(am.authenticators, provider)
	am.logger.Info("Removed authenticator for provider", "provider", provider)
	return nil
}

// Close closes the auth manager and cleans up resources
func (am *AuthManagerImpl) Close() error {
	if am.cleanupTicker != nil {
		am.cleanupTicker.Stop()
	}

	// Logout all providers
	ctx := context.Background()
	am.mutex.RLock()
	providers := make([]string, 0, len(am.authenticators))
	for provider := range am.authenticators {
		providers = append(providers, provider)
	}
	am.mutex.RUnlock()

	for _, provider := range providers {
		if authenticator, err := am.GetAuthenticator(provider); err == nil {
			am.safeLogout(ctx, provider, authenticator)
		} else {
			// Log that we couldn't get the authenticator, but continue with cleanup
			am.logger.Warn("Failed to get authenticator during close", "provider", provider, "error", err.Error())
		}
	}

	// Close storage if it supports Close
	if closer, ok := am.storage.(interface{ Close() error }); ok {
		return closer.Close()
	}

	return nil
}

// Helper methods

// safeLogout performs a logout operation with error logging but doesn't fail the operation
func (am *AuthManagerImpl) safeLogout(ctx context.Context, provider string, authenticator Authenticator) {
	am.mutex.RLock()
	logger := am.logger
	am.mutex.RUnlock()

	if err := authenticator.Logout(ctx); err != nil {
		// Log the error but don't fail the operation - logout during cleanup is often non-critical
		logger.Warn("Logout failed during cleanup operation", "provider", provider, "error", err.Error())
	} else {
		logger.Debug("Logout successful during cleanup", "provider", provider)
	}
}

func (am *AuthManagerImpl) startCleanupTicker() {
	am.mutex.RLock()
	interval := am.config.TokenStorage.File.Backup.Interval
	am.mutex.RUnlock()

	if interval <= 0 {
		interval = 1 * time.Hour // Default cleanup interval
	}

	am.cleanupTicker = time.NewTicker(interval)
	go func() {
		for range am.cleanupTicker.C {
			if err := am.CleanupExpired(); err != nil {
				// Get logger safely
				am.mutex.RLock()
				logger := am.logger
				am.mutex.RUnlock()
				// Log cleanup errors but don't crash the background goroutine
				logger.Warn("Periodic cleanup failed", "error", err.Error())
			}
		}
	}()
}

// AuthManagerBuilder provides a builder pattern for creating AuthManager instances
type AuthManagerBuilder struct {
	storage        TokenStorage
	config         *Config
	logger         Logger
	authenticators map[string]Authenticator
}

// NewAuthManagerBuilder creates a new auth manager builder
func NewAuthManagerBuilder() *AuthManagerBuilder {
	return &AuthManagerBuilder{
		config:         DefaultConfig(),
		authenticators: make(map[string]Authenticator),
	}
}

// WithStorage sets the token storage
func (b *AuthManagerBuilder) WithStorage(storage TokenStorage) *AuthManagerBuilder {
	b.storage = storage
	return b
}

// WithConfig sets the configuration
func (b *AuthManagerBuilder) WithConfig(config *Config) *AuthManagerBuilder {
	b.config = config
	return b
}

// WithLogger sets the logger
func (b *AuthManagerBuilder) WithLogger(logger Logger) *AuthManagerBuilder {
	b.logger = logger
	return b
}

// WithAuthenticator adds an authenticator
func (b *AuthManagerBuilder) WithAuthenticator(provider string, authenticator Authenticator) *AuthManagerBuilder {
	b.authenticators[provider] = authenticator
	return b
}

// Build creates the auth manager
func (b *AuthManagerBuilder) Build() (*AuthManagerImpl, error) {
	if b.storage == nil {
		// Create default memory storage
		b.storage = NewMemoryTokenStorage(&b.config.TokenStorage.Memory)
	}

	manager := NewAuthManager(b.storage, b.config)

	if b.logger != nil {
		manager.SetLogger(b.logger)
	}

	// Register authenticators
	for provider, authenticator := range b.authenticators {
		if err := manager.RegisterAuthenticator(provider, authenticator); err != nil {
			return nil, fmt.Errorf("failed to register authenticator for %s: %w", provider, err)
		}
	}

	return manager, nil
}
