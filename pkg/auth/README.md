# Authentication Package

The `auth` package provides a comprehensive authentication system for AI providers, supporting multiple authentication methods including OAuth 2.0, API keys, and bearer tokens with features like token encryption, multi-key management, and automatic failover.

## Features

### 🔐 Multiple Authentication Methods
- **OAuth 2.0**: Complete OAuth flow implementation with PKCE support
- **API Keys**: Single and multi-key management with load balancing
- **Bearer Tokens**: Simple token-based authentication
- **Custom**: Extensible for provider-specific authentication

### 🛡️ Security & Encryption
- AES-GCM token encryption with key derivation
- Secure token storage with automatic expiration
- PKCE (Proof Key for Code Exchange) support
- Token masking for secure logging

### 🔄 Advanced Features
- Multi-key load balancing with round-robin, random, and weighted strategies
- Circuit breaker pattern for fault tolerance
- Automatic token refresh with configurable buffers
- Health tracking with exponential backoff
- Comprehensive error handling with retry logic

### 📊 Monitoring & Observability
- Detailed health status reporting
- Request metrics and success rates
- Token metadata and expiration tracking
- Migration reporting and validation

## Quick Start

### Basic Setup

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/auth"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Create default configuration
    config := auth.DefaultConfig()

    // Create file-based token storage with encryption
    storage, err := auth.NewFileTokenStorage(
        &config.TokenStorage.File,
        &config.TokenStorage.Encryption,
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create auth manager
    authManager := auth.NewAuthManager(storage, config)

    // Register authenticator
    authenticator := auth.NewOAuthAuthenticator("anthropic", storage, &config.OAuth)
    err = authManager.RegisterAuthenticator("anthropic", authenticator)
    if err != nil {
        log.Fatal(err)
    }

    // Use authentication
    ctx := context.Background()
    err = authManager.Authenticate(ctx, "anthropic", types.AuthConfig{
        Method: types.AuthMethodOAuth,
        OAuthConfig: &types.OAuthConfig{
            ClientID:     "your-client-id",
            ClientSecret: "your-client-secret",
            AuthURL:      "https://auth.anthropic.com/oauth/authorize",
            TokenURL:     "https://auth.anthropic.com/oauth/token",
            RedirectURL:  "http://localhost:8080/callback",
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

### API Key Management

```go
// Configure API key manager
apiKeyConfig := &auth.APIKeyConfig{
    Strategy: "round_robin",
    Health: auth.HealthConfig{
        Enabled:          true,
        FailureThreshold: 3,
        Backoff: auth.BackoffConfig{
            Initial:   1 * time.Second,
            Maximum:   60 * time.Second,
            Multiplier: 2.0,
            Jitter:    true,
        },
    },
}

// Create manager with multiple keys
manager, err := auth.NewAPIKeyManager("openai", []string{
    "sk-key1...",
    "sk-key2...",
    "sk-key3...",
}, apiKeyConfig)

// Execute with automatic failover
result, err := manager.ExecuteWithFailover(func(apiKey string) (string, error) {
    // Make API call with key
    return callAPI(apiKey)
})

// Report success/failure
manager.ReportSuccess(apiKey)
manager.ReportFailure(apiKey, err)

// Get health status
status := manager.GetStatus()
```

### OAuth Flow Implementation

```go
// Start OAuth flow
authenticator := auth.NewOAuthAuthenticator("google", storage, &oauthConfig)

authURL, err := authenticator.StartOAuthFlow(ctx, []string{"profile", "email"})
if err != nil {
    log.Fatal(err)
}

// Redirect user to authURL
// After authorization, handle callback
err = authenticator.HandleCallback(ctx, authorizationCode, state)
if err != nil {
    log.Fatal(err)
}

// Get token information
tokenInfo, err := authenticator.GetTokenInfo()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Token expires at: %v\n", tokenInfo.ExpiresAt)
```

## Configuration

### Complete Configuration

```go
config := &auth.Config{
    TokenStorage: auth.TokenStorageConfig{
        Type: "file",
        File: auth.FileStorageConfig{
            Directory:           "./tokens",
            FilePermissions:     "0600",
            DirectoryPermissions: "0700",
            Backup: auth.BackupConfig{
                Enabled: true,
                Interval: 24 * time.Hour,
                MaxFiles: 10,
            },
        },
        Encryption: auth.EncryptionConfig{
            Enabled:  true,
            Key:      "your-encryption-key-32-bytes!",
            Algorithm: "aes-256-gcm",
            KeyDerivation: auth.KeyDerivationConfig{
                Function:  "pbkdf2",
                Iterations: 100000,
                KeyLength: 32,
            },
        },
    },
    OAuth: auth.OAuthConfig{
        DefaultScopes: []string{},
        State: auth.StateConfig{
            Length:          32,
            Expiration:      10 * time.Minute,
            EnableValidation: true,
        },
        PKCE: auth.PKCEConfig{
            Enabled:       true,
            Method:        "S256",
            VerifierLength: 128,
        },
        Refresh: auth.RefreshConfig{
            Enabled:         true,
            Buffer:          5 * time.Minute,
            MaxRetries:      3,
            RefreshOnFailure: true,
        },
    },
    APIKey: auth.APIKeyConfig{
        Strategy: "round_robin",
        Health: auth.HealthConfig{
            Enabled:          true,
            FailureThreshold: 3,
            Backoff: auth.BackoffConfig{
                Initial:   1 * time.Second,
                Maximum:   60 * time.Second,
                Multiplier: 2.0,
                Jitter:    true,
            },
        },
        Failover: auth.FailoverConfig{
            Enabled:     true,
            MaxAttempts: 3,
        },
    },
}
```

### Environment Variables

The authentication system supports configuration through environment variables:

```bash
# Token Storage
AUTH_STORAGE_TYPE=file
AUTH_STORAGE_DIRECTORY=./tokens
AUTH_ENCRYPTION_ENABLED=true
AUTH_ENCRYPTION_KEY=your-encryption-key

# OAuth
AUTH_OAUTH_STATE_LENGTH=32
AUTH_OAUTH_PKCE_ENABLED=true
AUTH_OAUTH_REFRESH_BUFFER=5m

# API Keys
AUTH_APIKEY_STRATEGY=round_robin
AUTH_APIKEY_FAILURE_THRESHOLD=3
AUTH_APIKEY_MAX_ATTEMPTS=3
```

## Migration from provider-library

The package includes a migration helper to transition from the old authentication system:

```go
// Create migration helper
helper := auth.NewMigrationHelper(auth.DefaultConfig())

// Define providers to migrate
configs := []auth.ProviderMigrationConfig{
    {
        Provider:      "anthropic",
        AuthMethod:    types.AuthMethodAPIKey,
        OldConfig:     "sk-ant-api03-...",
        MigrateTokens: false,
    },
    {
        Provider:      "gemini",
        AuthMethod:    types.AuthMethodOAuth,
        OldConfig: &types.AuthConfig{
            OAuthConfig: &types.OAuthConfig{
                AccessToken: "ya29.a0AfH6SMB...",
                ExpiresAt:   time.Now().Add(1 * time.Hour),
            },
        },
        MigrateTokens: true,
    },
}

// Migrate all providers
authenticators, err := helper.MigrateAllProviders(ctx, configs)
if err != nil {
    log.Fatal(err)
}

// Create auth manager with migrated providers
authManager, err := helper.CreateAuthManager()
if err != nil {
    log.Fatal(err)
}

// Register migrated authenticators
for provider, authenticator := range authenticators {
    authManager.RegisterAuthenticator(provider, authenticator)
}

// Generate migration report
report := helper.GetMigrationReport(ctx, authenticators)
fmt.Printf("Migration completed: %d/%d providers\n",
    report.SuccessfulMigrations, report.TotalProviders)
```

## API Reference

### Core Interfaces

#### Authenticator
```go
type Authenticator interface {
    Authenticate(ctx context.Context, config types.AuthConfig) error
    IsAuthenticated() bool
    GetToken() (string, error)
    RefreshToken(ctx context.Context) error
    Logout(ctx context.Context) error
    GetAuthMethod() types.AuthMethod
}
```

#### OAuthAuthenticator
```go
type OAuthAuthenticator interface {
    Authenticator
    StartOAuthFlow(ctx context.Context, scopes []string) (string, error)
    HandleCallback(ctx context.Context, code, state string) error
    IsOAuthEnabled() bool
    GetTokenInfo() (*TokenInfo, error)
}
```

#### APIKeyAuthenticator
```go
type APIKeyAuthenticator interface {
    Authenticator
    GetKeyManager() APIKeyManager
    RotateKey() error
    ReportKeySuccess(key string) error
    ReportKeyFailure(key string, err error) error
}
```

#### AuthManager
```go
type AuthManager interface {
    RegisterAuthenticator(provider string, authenticator Authenticator) error
    GetAuthenticator(provider string) (Authenticator, error)
    Authenticate(ctx context.Context, provider string, config types.AuthConfig) error
    IsAuthenticated(provider string) bool
    Logout(ctx context.Context, provider string) error
    RefreshAllTokens(ctx context.Context) error
    GetAuthenticatedProviders() []string
    GetAuthStatus() map[string]*AuthState
    StartOAuthFlow(ctx context.Context, provider string, scopes []string) (string, error)
    HandleOAuthCallback(ctx context.Context, provider string, code, state string) error
}
```

### Storage Interfaces

#### TokenStorage
```go
type TokenStorage interface {
    StoreToken(key string, token *types.OAuthConfig) error
    RetrieveToken(key string) (*types.OAuthConfig, error)
    DeleteToken(key string) error
    ListTokens() ([]string, error)
    IsTokenValid(key string) bool
    CleanupExpired() error
    GetTokenInfo(key string) (*TokenMetadata, error)
}
```

## Error Handling

The authentication system provides detailed error information:

```go
type AuthError struct {
    Provider string `json:"provider"`
    Code     string `json:"code"`
    Message  string `json:"message"`
    Details  string `json:"details,omitempty"`
    Retry    bool   `json:"retry,omitempty"`
}

// Error codes
const (
    ErrCodeInvalidCredentials  = "invalid_credentials"
    ErrCodeTokenExpired        = "token_expired"
    ErrCodeRefreshFailed       = "refresh_failed"
    ErrCodeOAuthFlowFailed     = "oauth_flow_failed"
    ErrCodeNetworkError        = "network_error"
    ErrCodeStorageError        = "storage_error"
    ErrCodeEncryptionError     = "encryption_error"
)
```

## Best Practices

### Security
1. **Always enable encryption** for token storage
2. **Use strong encryption keys** (32 bytes minimum)
3. **Implement proper key rotation** for long-lived tokens
4. **Validate OAuth state** to prevent CSRF attacks
5. **Use PKCE** for public OAuth clients

### Performance
1. **Use memory storage** for short-lived tokens
2. **Configure appropriate backoff** for retry logic
3. **Implement circuit breakers** for external services
4. **Monitor health metrics** for proactive issue detection

### Reliability
1. **Configure multiple API keys** for redundancy
2. **Implement proper error handling** with retry logic
3. **Set appropriate timeouts** for network operations
4. **Monitor token expiration** and refresh proactively

## Examples

See the `example_test.go` file for comprehensive examples of:

- OAuth flow implementation
- API key management with failover
- Token encryption and storage
- Migration from old auth systems
- Custom authenticator implementation

## Testing

Run the test suite:

```bash
go test ./pkg/auth/...
```

Run examples:

```bash
go test -run Example ./pkg/auth/...
```

## Contributing

1. Follow Go coding standards
2. Add comprehensive tests for new features
3. Update documentation for API changes
4. Ensure backward compatibility where possible

## License

This package is part of the ai-provider-kit project and follows the same license terms.