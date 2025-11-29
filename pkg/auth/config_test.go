package auth

import (
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Error("Expected non-nil config")
	}

	// Test token storage config
	if config.TokenStorage.Type != "file" {
		t.Error("Expected file storage type")
	}

	if config.TokenStorage.File.Directory != "./tokens" {
		t.Error("Expected default tokens directory")
	}

	if config.TokenStorage.Memory.MaxTokens != 100 {
		t.Error("Expected max tokens to be 100")
	}

	// Test OAuth config
	if !config.OAuth.PKCE.Enabled {
		t.Error("Expected PKCE to be enabled")
	}

	if config.OAuth.PKCE.Method != "S256" {
		t.Error("Expected S256 PKCE method")
	}

	if !config.OAuth.State.EnableValidation {
		t.Error("Expected state validation to be enabled")
	}

	if !config.OAuth.Refresh.Enabled {
		t.Error("Expected token refresh to be enabled")
	}

	// Test API key config
	if config.APIKey.Strategy != "round_robin" {
		t.Error("Expected round_robin strategy")
	}

	if !config.APIKey.Health.Enabled {
		t.Error("Expected health tracking to be enabled")
	}

	if config.APIKey.Health.FailureThreshold != 3 {
		t.Error("Expected failure threshold to be 3")
	}

	// Test security config
	if !config.Security.TokenMasking.Enabled {
		t.Error("Expected token masking to be enabled")
	}

	if config.Security.TokenMasking.PrefixLength != 8 {
		t.Error("Expected prefix length to be 8")
	}

	// Test retry config
	if !config.Retry.Enabled {
		t.Error("Expected retry to be enabled")
	}

	if config.Retry.MaxAttempts != 3 {
		t.Error("Expected max retry attempts to be 3")
	}

	// Test timeout config
	if config.Timeouts.Auth != 30*time.Second {
		t.Error("Expected auth timeout to be 30s")
	}
}

func TestProviderConfigToAuthConfig(t *testing.T) {
	providerConfig := types.ProviderConfig{
		Type:         types.ProviderTypeOpenAI,
		APIKey:       "test-key",
		BaseURL:      "https://api.example.com",
		DefaultModel: "test-model",
	}

	authConfig := ProviderConfigToAuthConfig(providerConfig)

	if authConfig.Method != types.AuthMethodAPIKey {
		t.Error("Expected API key method")
	}

	if authConfig.APIKey != "test-key" {
		t.Error("Expected API key to match")
	}

	if authConfig.BaseURL != "https://api.example.com" {
		t.Error("Expected base URL to match")
	}

	if authConfig.DefaultModel != "test-model" {
		t.Error("Expected default model to match")
	}
}

func TestAuthConfigToProviderConfig(t *testing.T) {
	authConfig := types.AuthConfig{
		Method:       types.AuthMethodAPIKey,
		APIKey:       "test-key",
		BaseURL:      "https://api.example.com",
		DefaultModel: "test-model",
	}

	providerConfig := AuthConfigToProviderConfig(authConfig, types.ProviderTypeOpenAI)

	if providerConfig.Type != types.ProviderTypeOpenAI {
		t.Error("Expected OpenAI provider type")
	}

	if providerConfig.APIKey != "test-key" {
		t.Error("Expected API key to match")
	}

	if providerConfig.BaseURL != "https://api.example.com" {
		t.Error("Expected base URL to match")
	}

	if providerConfig.DefaultModel != "test-model" {
		t.Error("Expected default model to match")
	}
}

func TestAuthError(t *testing.T) {
	t.Run("WithDetails", func(t *testing.T) {
		err := &AuthError{
			Provider: "test",
			Code:     "test_error",
			Message:  "test message",
			Details:  "additional details",
		}

		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Expected non-empty error message")
		}
	})

	t.Run("WithoutDetails", func(t *testing.T) {
		err := &AuthError{
			Provider: "test",
			Code:     "test_error",
			Message:  "test message",
		}

		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Expected non-empty error message")
		}
	})

	t.Run("IsRetryable", func(t *testing.T) {
		err := &AuthError{
			Provider: "test",
			Code:     "network_error",
			Message:  "network error",
			Retry:    true,
		}

		if !err.IsRetryable() {
			t.Error("Expected error to be retryable")
		}
	})

	t.Run("NotRetryable", func(t *testing.T) {
		err := &AuthError{
			Provider: "test",
			Code:     "invalid_credentials",
			Message:  "invalid credentials",
			Retry:    false,
		}

		if err.IsRetryable() {
			t.Error("Expected error not to be retryable")
		}
	})
}

func TestConfigStructures(t *testing.T) {
	t.Run("FileStorageConfig", func(t *testing.T) {
		config := &FileStorageConfig{
			Directory:            "/test/tokens",
			FilePermissions:      "0600",
			DirectoryPermissions: "0700",
			Backup: BackupConfig{
				Enabled:   true,
				Directory: "/test/backup",
				Interval:  24 * time.Hour,
				MaxFiles:  10,
			},
		}

		if config.Directory != "/test/tokens" {
			t.Error("Expected directory to match")
		}
		if !config.Backup.Enabled {
			t.Error("Expected backup to be enabled")
		}
	})

	t.Run("MemoryStorageConfig", func(t *testing.T) {
		config := &MemoryStorageConfig{
			MaxTokens:         50,
			CleanupInterval:   30 * time.Minute,
			EnablePersistence: true,
			PersistenceFile:   "/test/tokens.json",
		}

		if config.MaxTokens != 50 {
			t.Error("Expected max tokens to match")
		}
		if !config.EnablePersistence {
			t.Error("Expected persistence to be enabled")
		}
	})

	t.Run("EncryptionConfig", func(t *testing.T) {
		config := &EncryptionConfig{
			Enabled:   true,
			Key:       "test-key",
			KeyFile:   "/test/key.txt",
			Algorithm: "aes-256-gcm",
			KeyDerivation: KeyDerivationConfig{
				Function:   "pbkdf2",
				Salt:       "test-salt",
				Iterations: 100000,
				KeyLength:  32,
			},
		}

		if !config.Enabled {
			t.Error("Expected encryption to be enabled")
		}
		if config.KeyDerivation.Function != "pbkdf2" {
			t.Error("Expected pbkdf2 function")
		}
	})

	t.Run("OAuthConfig", func(t *testing.T) {
		config := &OAuthConfig{
			DefaultScopes: []string{"read", "write"},
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
				Enabled:          true,
				Buffer:           5 * time.Minute,
				MaxRetries:       3,
				RefreshOnFailure: true,
			},
		}

		if len(config.DefaultScopes) != 2 {
			t.Error("Expected 2 default scopes")
		}
		if !config.PKCE.Enabled {
			t.Error("Expected PKCE to be enabled")
		}
	})

	t.Run("APIKeyConfig", func(t *testing.T) {
		config := &APIKeyConfig{
			Strategy: "weighted",
			Health: HealthConfig{
				Enabled:          true,
				FailureThreshold: 5,
				SuccessThreshold: 2,
				Backoff: BackoffConfig{
					Initial:    2 * time.Second,
					Maximum:    120 * time.Second,
					Multiplier: 3.0,
					Jitter:     true,
				},
			},
			Failover: FailoverConfig{
				Enabled:     true,
				MaxAttempts: 5,
				Strategy:    "sequential",
				CircuitBreaker: CircuitBreakerConfig{
					Enabled:             true,
					FailureThreshold:    10,
					RecoveryTimeout:     60 * time.Second,
					HalfOpenMaxRequests: 3,
				},
			},
		}

		if config.Strategy != "weighted" {
			t.Error("Expected weighted strategy")
		}
		if !config.Failover.CircuitBreaker.Enabled {
			t.Error("Expected circuit breaker to be enabled")
		}
	})

	t.Run("SecurityConfig", func(t *testing.T) {
		config := &SecurityConfig{
			AuditLogging: true,
			TokenMasking: TokenMaskingConfig{
				Enabled:      true,
				PrefixLength: 10,
				SuffixLength: 5,
				MaskChar:     "#",
			},
			RateLimiting: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 100,
				Burst:             20,
			},
		}

		if !config.AuditLogging {
			t.Error("Expected audit logging to be enabled")
		}
		if config.TokenMasking.MaskChar != "#" {
			t.Error("Expected mask char to be #")
		}
	})
}

func TestTokenInfoAndMetadata(t *testing.T) {
	t.Run("TokenInfo", func(t *testing.T) {
		expiresAt := time.Now().Add(1 * time.Hour)
		info := &TokenInfo{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			TokenType:    "Bearer",
			ExpiresAt:    expiresAt,
			Scopes:       []string{"read", "write"},
			IsExpired:    false,
			ExpiresIn:    3600,
		}

		if info.AccessToken != "access-token" {
			t.Error("Expected access token to match")
		}
		if info.IsExpired {
			t.Error("Expected token not to be expired")
		}
	})

	t.Run("TokenMetadata", func(t *testing.T) {
		metadata := &TokenMetadata{
			Provider:     "test",
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			ExpiresAt:    time.Now().Add(1 * time.Hour),
			IsEncrypted:  true,
		}

		if metadata.Provider != "test" {
			t.Error("Expected provider to match")
		}
		if !metadata.IsEncrypted {
			t.Error("Expected token to be encrypted")
		}
	})

	t.Run("AuthState", func(t *testing.T) {
		state := &AuthState{
			Provider:      "test",
			Authenticated: true,
			Method:        types.AuthMethodOAuth,
			LastAuth:      time.Now(),
			ExpiresAt:     time.Now().Add(1 * time.Hour),
			CanRefresh:    true,
		}

		if !state.Authenticated {
			t.Error("Expected authenticated state")
		}
		if !state.CanRefresh {
			t.Error("Expected can refresh to be true")
		}
	})
}

func TestProviderAuthConfig(t *testing.T) {
	config := &ProviderAuthConfig{
		Provider:       "test",
		AuthMethod:     types.AuthMethodOAuth,
		DisplayName:    "Test Provider",
		Description:    "A test provider",
		OAuthURL:       "https://example.com/oauth",
		RequiredScopes: []string{"read"},
		OptionalScopes: []string{"write"},
		Features: AuthFeatureFlags{
			SupportsOAuth:    true,
			SupportsAPIKey:   true,
			SupportsMultiKey: true,
			RequiresPKCE:     true,
			TokenRefresh:     true,
		},
	}

	if config.Provider != "test" {
		t.Error("Expected provider to match")
	}

	if !config.Features.SupportsOAuth {
		t.Error("Expected OAuth support")
	}

	if !config.Features.RequiresPKCE {
		t.Error("Expected PKCE requirement")
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []string{
		ErrCodeInvalidCredentials,
		ErrCodeTokenExpired,
		ErrCodeRefreshFailed,
		ErrCodeOAuthFlowFailed,
		ErrCodeInvalidConfig,
		ErrCodeNetworkError,
		ErrCodeProviderUnavailable,
		ErrCodeScopeInsufficient,
		ErrCodeKeyRotationFailed,
		ErrCodeAllKeysExhausted,
		ErrCodeStorageError,
		ErrCodeEncryptionError,
	}

	for _, code := range codes {
		if code == "" {
			t.Error("Expected non-empty error code")
		}
	}
}
