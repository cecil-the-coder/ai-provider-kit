package auth

import (
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Config represents the configuration for the authentication system
type Config struct {
	// Token storage configuration
	TokenStorage TokenStorageConfig `json:"token_storage"`

	// OAuth configuration
	OAuth OAuthConfig `json:"oauth"`

	// API key management configuration
	APIKey APIKeyConfig `json:"api_key"`

	// Security configuration
	Security SecurityConfig `json:"security"`

	// Retry configuration
	Retry RetryConfig `json:"retry"`

	// Default timeouts
	Timeouts TimeoutConfig `json:"timeouts"`
}

// TokenStorageConfig represents configuration for token storage
type TokenStorageConfig struct {
	// Storage backend type (file, memory, custom)
	Type string `json:"type"`

	// File storage configuration
	File FileStorageConfig `json:"file"`

	// Memory storage configuration
	Memory MemoryStorageConfig `json:"memory"`

	// Encryption configuration
	Encryption EncryptionConfig `json:"encryption"`
}

// FileStorageConfig represents configuration for file-based token storage
type FileStorageConfig struct {
	// Directory to store tokens
	Directory string `json:"directory"`

	// File permissions
	FilePermissions string `json:"file_permissions"`

	// Directory permissions
	DirectoryPermissions string `json:"directory_permissions"`

	// Backup configuration
	Backup BackupConfig `json:"backup"`
}

// BackupConfig represents configuration for token backup
type BackupConfig struct {
	// Enable backup
	Enabled bool `json:"enabled"`

	// Backup directory
	Directory string `json:"directory"`

	// Backup interval
	Interval time.Duration `json:"interval"`

	// Max backup files to keep
	MaxFiles int `json:"max_files"`
}

// MemoryStorageConfig represents configuration for memory-based token storage
type MemoryStorageConfig struct {
	// Maximum number of tokens to store
	MaxTokens int `json:"max_tokens"`

	// Cleanup interval
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// Enable persistence to file
	EnablePersistence bool `json:"enable_persistence"`

	// Persistence file path
	PersistenceFile string `json:"persistence_file"`
}

// EncryptionConfig represents configuration for token encryption
type EncryptionConfig struct {
	// Enable encryption
	Enabled bool `json:"enabled"`

	// Encryption key
	Key string `json:"key"`

	// Key file path
	KeyFile string `json:"key_file"`

	// Encryption algorithm
	Algorithm string `json:"algorithm"`

	// Key derivation configuration
	KeyDerivation KeyDerivationConfig `json:"key_derivation"`
}

// KeyDerivationConfig represents configuration for key derivation
type KeyDerivationConfig struct {
	// Key derivation function
	Function string `json:"function"`

	// Salt
	Salt string `json:"salt"`

	// Number of iterations
	Iterations int `json:"iterations"`

	// Key length
	KeyLength int `json:"key_length"`
}

// OAuthConfig represents configuration for OAuth
type OAuthConfig struct {
	// Default scopes
	DefaultScopes []string `json:"default_scopes"`

	// State management
	State StateConfig `json:"state"`

	// PKCE configuration
	PKCE PKCEConfig `json:"pkce"`

	// Token refresh configuration
	Refresh RefreshConfig `json:"refresh"`

	// HTTP client configuration
	HTTP HTTPConfig `json:"http"`
}

// StateConfig represents configuration for OAuth state management
type StateConfig struct {
	// State length
	Length int `json:"length"`

	// State expiration
	Expiration time.Duration `json:"expiration"`

	// Enable state validation
	EnableValidation bool `json:"enable_validation"`
}

// PKCEConfig represents configuration for PKCE
type PKCEConfig struct {
	// Enable PKCE
	Enabled bool `json:"enabled"`

	// Code challenge method
	Method string `json:"method"`

	// Code verifier length
	VerifierLength int `json:"verifier_length"`
}

// RefreshConfig represents configuration for token refresh
type RefreshConfig struct {
	// Enable automatic refresh
	Enabled bool `json:"enabled"`

	// Refresh buffer (refresh token before it expires)
	Buffer time.Duration `json:"buffer"`

	// Max retry attempts
	MaxRetries int `json:"max_retries"`

	// Enable refresh on failure
	RefreshOnFailure bool `json:"refresh_on_failure"`
}

// HTTPConfig represents configuration for HTTP clients
type HTTPConfig struct {
	// Timeout
	Timeout time.Duration `json:"timeout"`

	// User agent
	UserAgent string `json:"user_agent"`

	// TLS configuration
	TLS TLSConfig `json:"tls"`

	// Proxy configuration
	Proxy ProxyConfig `json:"proxy"`
}

// TLSConfig represents configuration for TLS
type TLSConfig struct {
	// Insecure skip verify
	InsecureSkipVerify bool `json:"insecure_skip_verify"`

	// CA certificate file
	CAFile string `json:"ca_file"`

	// Certificate file
	CertFile string `json:"cert_file"`

	// Key file
	KeyFile string `json:"key_file"`
}

// ProxyConfig represents configuration for HTTP proxy
type ProxyConfig struct {
	// Enable proxy
	Enabled bool `json:"enabled"`

	// Proxy URL
	URL string `json:"url"`

	// Proxy authentication
	Auth ProxyAuthConfig `json:"auth"`
}

// ProxyAuthConfig represents configuration for proxy authentication
type ProxyAuthConfig struct {
	// Username
	Username string `json:"username"`

	// Password
	Password string `json:"password"`
}

// APIKeyConfig represents configuration for API key management
type APIKeyConfig struct {
	// Load balancing strategy
	Strategy string `json:"strategy"`

	// Health check configuration
	Health HealthConfig `json:"health"`

	// Failover configuration
	Failover FailoverConfig `json:"failover"`

	// Rotation configuration
	Rotation RotationConfig `json:"rotation"`
}

// HealthConfig represents configuration for API key health tracking
type HealthConfig struct {
	// Enable health tracking
	Enabled bool `json:"enabled"`

	// Failure threshold
	FailureThreshold int `json:"failure_threshold"`

	// Success threshold
	SuccessThreshold int `json:"success_threshold"`

	// Backoff configuration
	Backoff BackoffConfig `json:"backoff"`

	// Health check interval
	CheckInterval time.Duration `json:"check_interval"`
}

// BackoffConfig represents configuration for exponential backoff
type BackoffConfig struct {
	// Initial backoff duration
	Initial time.Duration `json:"initial"`

	// Maximum backoff duration
	Maximum time.Duration `json:"maximum"`

	// Backoff multiplier
	Multiplier float64 `json:"multiplier"`

	// Jitter
	Jitter bool `json:"jitter"`
}

// FailoverConfig represents configuration for API key failover
type FailoverConfig struct {
	// Enable failover
	Enabled bool `json:"enabled"`

	// Maximum attempts
	MaxAttempts int `json:"max_attempts"`

	// Failover strategy
	Strategy string `json:"strategy"`

	// Circuit breaker configuration
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker"`
}

// CircuitBreakerConfig represents configuration for circuit breaker
type CircuitBreakerConfig struct {
	// Enable circuit breaker
	Enabled bool `json:"enabled"`

	// Failure threshold
	FailureThreshold int `json:"failure_threshold"`

	// Recovery timeout
	RecoveryTimeout time.Duration `json:"recovery_timeout"`

	// Half-open max requests
	HalfOpenMaxRequests int `json:"half_open_max_requests"`
}

// RotationConfig represents configuration for API key rotation
type RotationConfig struct {
	// Enable rotation
	Enabled bool `json:"enabled"`

	// Rotation interval
	Interval time.Duration `json:"interval"`

	// Rotation strategy
	Strategy string `json:"strategy"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	// Enable audit logging
	AuditLogging bool `json:"audit_logging"`

	// Token masking
	TokenMasking TokenMaskingConfig `json:"token_masking"`

	// Rate limiting
	RateLimiting RateLimitConfig `json:"rate_limiting"`
}

// TokenMaskingConfig represents configuration for token masking
type TokenMaskingConfig struct {
	// Enable masking
	Enabled bool `json:"enabled"`

	// Prefix length to show
	PrefixLength int `json:"prefix_length"`

	// Suffix length to show
	SuffixLength int `json:"suffix_length"`

	// Mask character
	MaskChar string `json:"mask_char"`
}

// RateLimitConfig represents configuration for rate limiting
type RateLimitConfig struct {
	// Enable rate limiting
	Enabled bool `json:"enabled"`

	// Requests per minute
	RequestsPerMinute int `json:"requests_per_minute"`

	// Burst size
	Burst int `json:"burst"`
}

// RetryConfig represents configuration for retry operations
type RetryConfig struct {
	// Enable retry
	Enabled bool `json:"enabled"`

	// Maximum attempts
	MaxAttempts int `json:"max_attempts"`

	// Initial delay
	InitialDelay time.Duration `json:"initial_delay"`

	// Maximum delay
	MaxDelay time.Duration `json:"max_delay"`

	// Backoff multiplier
	Multiplier float64 `json:"multiplier"`

	// Jitter
	Jitter bool `json:"jitter"`
}

// TimeoutConfig represents timeout configuration
type TimeoutConfig struct {
	// Authentication timeout
	Auth time.Duration `json:"auth"`

	// Token refresh timeout
	TokenRefresh time.Duration `json:"token_refresh"`

	// OAuth flow timeout
	OAuthFlow time.Duration `json:"oauth_flow"`

	// Storage operation timeout
	Storage time.Duration `json:"storage"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		TokenStorage: TokenStorageConfig{
			Type: "file",
			File: FileStorageConfig{
				Directory:            "./tokens",
				FilePermissions:      "0600",
				DirectoryPermissions: "0700",
				Backup: BackupConfig{
					Enabled:  false,
					Interval: 24 * time.Hour,
					MaxFiles: 10,
				},
			},
			Memory: MemoryStorageConfig{
				MaxTokens:         100,
				CleanupInterval:   1 * time.Hour,
				EnablePersistence: false,
			},
			Encryption: EncryptionConfig{
				Enabled:   true,
				Algorithm: "aes-256-gcm",
				KeyDerivation: KeyDerivationConfig{
					Function:   "pbkdf2",
					Iterations: 100000,
					KeyLength:  32,
				},
			},
		},
		OAuth: OAuthConfig{
			DefaultScopes: []string{},
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
			HTTP: HTTPConfig{
				Timeout:   30 * time.Second,
				UserAgent: "ai-provider-kit/1.0",
			},
		},
		APIKey: APIKeyConfig{
			Strategy: "round_robin",
			Health: HealthConfig{
				Enabled:          true,
				FailureThreshold: 3,
				SuccessThreshold: 2,
				Backoff: BackoffConfig{
					Initial:    1 * time.Second,
					Maximum:    60 * time.Second,
					Multiplier: 2.0,
					Jitter:     true,
				},
				CheckInterval: 5 * time.Minute,
			},
			Failover: FailoverConfig{
				Enabled:     true,
				MaxAttempts: 3,
				Strategy:    "fail_fast",
			},
			Rotation: RotationConfig{
				Enabled:  false,
				Interval: 24 * time.Hour,
				Strategy: "round_robin",
			},
		},
		Security: SecurityConfig{
			AuditLogging: false,
			TokenMasking: TokenMaskingConfig{
				Enabled:      true,
				PrefixLength: 8,
				SuffixLength: 4,
				MaskChar:     "*",
			},
			RateLimiting: RateLimitConfig{
				Enabled:           false,
				RequestsPerMinute: 60,
				Burst:             10,
			},
		},
		Retry: RetryConfig{
			Enabled:      true,
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
			Jitter:       true,
		},
		Timeouts: TimeoutConfig{
			Auth:         30 * time.Second,
			TokenRefresh: 30 * time.Second,
			OAuthFlow:    10 * time.Minute,
			Storage:      5 * time.Second,
		},
	}
}

// ProviderConfigToAuthConfig converts a ProviderConfig to AuthConfig
func ProviderConfigToAuthConfig(providerConfig types.ProviderConfig) types.AuthConfig {
	return types.AuthConfig{
		Method:       types.AuthMethodAPIKey, // Default to API key
		APIKey:       providerConfig.APIKey,
		BaseURL:      providerConfig.BaseURL,
		DefaultModel: providerConfig.DefaultModel,
	}
}

// AuthConfigToProviderConfig converts an AuthConfig to ProviderConfig
func AuthConfigToProviderConfig(authConfig types.AuthConfig, providerType types.ProviderType) types.ProviderConfig {
	return types.ProviderConfig{
		Type:         providerType,
		APIKey:       authConfig.APIKey,
		BaseURL:      authConfig.BaseURL,
		DefaultModel: authConfig.DefaultModel,
	}
}
