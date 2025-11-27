# AI Provider Kit - Authentication and Security Guide

**Version:** 1.0
**Last Updated:** 2025-11-18

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication Methods](#authentication-methods)
3. [API Key Authentication](#api-key-authentication)
4. [OAuth 2.0 Authentication](#oauth-20-authentication)
5. [Multi-Credential Management](#multi-credential-management)
6. [Token Management](#token-management)
7. [Security Features](#security-features)
8. [Provider-Specific Authentication](#provider-specific-authentication)
9. [Monitoring and Alerts](#monitoring-and-alerts)
10. [Best Practices](#best-practices)
11. [Troubleshooting](#troubleshooting)

---

## Overview

The AI Provider Kit provides a comprehensive authentication and security framework supporting multiple authentication methods, automatic credential rotation, token refresh, and enterprise-grade security features.

### Supported Authentication Methods

| Method | Description | Providers | Use Case |
|--------|-------------|-----------|----------|
| **API Key** | Simple bearer token authentication | OpenAI, Anthropic, Gemini, Cerebras, OpenRouter | Production APIs with stable keys |
| **OAuth 2.0** | Token-based authentication with refresh | OpenAI, Anthropic, Gemini, Qwen, OpenRouter | Enterprise applications, multiple accounts |
| **Bearer Token** | Custom bearer token authentication | Custom providers | Internal services |
| **Device Code Flow** | OAuth for limited-input devices | Gemini, Qwen | CLI tools, headless servers |
| **PKCE Flow** | OAuth with proof key exchange | OpenAI, OpenRouter | Public clients, enhanced security |

### Key Features

- **Multi-Key/Multi-Credential Support** - Load balancing and automatic failover
- **Automatic Token Refresh** - Zero-downtime OAuth token management
- **Health Tracking** - Per-credential health monitoring with exponential backoff
- **Encryption** - AES-256-GCM encryption for stored tokens
- **PKCE Support** - Proof Key for Code Exchange for enhanced OAuth security
- **Audit Logging** - Comprehensive security event logging
- **Metrics & Monitoring** - Prometheus-compatible metrics export

---

## Authentication Methods

### Quick Comparison

```go
// API Key Authentication (Simple)
config := types.ProviderConfig{
    APIKey: "sk-your-api-key",
}

// OAuth Authentication (Advanced)
config := types.ProviderConfig{
    OAuthCredentials: []*types.OAuthCredentialSet{
        {
            ClientID:     "your-client-id",
            ClientSecret: "your-client-secret",
            AccessToken:  "access-token",
            RefreshToken: "refresh-token",
            ExpiresAt:    time.Now().Add(1 * time.Hour),
        },
    },
}
```

### Configuration File Examples

**YAML Configuration:**

```yaml
providers:
  # API Key Authentication
  anthropic:
    api_keys:
      - sk-ant-api-key-1
      - sk-ant-api-key-2  # Failover key
    model: claude-3-5-sonnet-20241022

  # OAuth Authentication
  gemini:
    oauth_credentials:
      - id: team-account
        client_id: your-client-id
        client_secret: your-client-secret
        access_token: ya29.a0...
        refresh_token: 1//06...
        expires_at: "2025-11-18T10:00:00Z"
        scopes:
          - https://www.googleapis.com/auth/cloud-platform
    model: gemini-2.0-flash-exp
```

---

## API Key Authentication

API key authentication is the simplest method, suitable for providers like OpenAI, Anthropic, Cerebras, and OpenRouter.

### Single API Key Setup

**Programmatic Configuration:**

```go
package main

import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    config := types.ProviderConfig{
        Type:   types.ProviderTypeAnthropic,
        APIKey: "sk-ant-api03-...",
        Model:  "claude-3-5-sonnet-20241022",
    }

    provider, err := factory.CreateProvider(config)
    if err != nil {
        panic(err)
    }

    // Provider is ready to use
}
```

**Environment Variables:**

```bash
export ANTHROPIC_API_KEY="sk-ant-api03-..."
export OPENAI_API_KEY="sk-proj-..."
export CEREBRAS_API_KEY="csk-..."
```

```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeAnthropic,
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    Model:  "claude-3-5-sonnet-20241022",
}
```

### Multi-Key Configuration for Failover

Multi-key setups provide automatic failover and load balancing across multiple API keys.

**Configuration:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/keymanager"

// Create key manager with multiple keys
manager := keymanager.NewKeyManager("Anthropic", []string{
    "sk-ant-api-key-1",
    "sk-ant-api-key-2",
    "sk-ant-api-key-3",
})

// Execute with automatic failover
result, usage, err := manager.ExecuteWithFailover(ctx,
    func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
        // Make API call with the provided key
        return makeAPICall(ctx, apiKey, options)
    })

if err != nil {
    log.Fatal(err)
}
```

**YAML Configuration:**

```yaml
providers:
  anthropic:
    api_keys:
      - sk-ant-api-key-1
      - sk-ant-api-key-2
      - sk-ant-api-key-3
    model: claude-3-5-sonnet-20241022
```

### How Multi-Key Failover Works

1. **Round-Robin Load Balancing** - Distributes requests evenly across keys
2. **Health Tracking** - Each key maintains independent health status
3. **Exponential Backoff** - Failed keys enter backoff periods (1s → 2s → 4s → 8s → max 60s)
4. **Automatic Recovery** - Keys recover after successful requests
5. **Failover Limit** - Tries up to 3 keys per request before failing

**Health Status Example:**

```go
// Key health tracking (automatic)
type keyHealth struct {
    failureCount int           // Consecutive failures
    lastFailure  time.Time     // Last failure timestamp
    lastSuccess  time.Time     // Last success timestamp
    isHealthy    bool          // Healthy if < 3 consecutive failures
    backoffUntil time.Time     // When key becomes available again
}
```

### Key Rotation Strategies

**Manual Rotation:**

```go
// Update config with new key
config.APIKey = "sk-ant-new-key"
provider.UpdateConfig(config)
```

**Graceful Rotation (Multi-Key):**

```go
// Add new key while keeping old ones
manager := keymanager.NewKeyManager("Anthropic", []string{
    "sk-ant-old-key-1",    // Will be phased out
    "sk-ant-old-key-2",    // Will be phased out
    "sk-ant-new-key-1",    // New production key
})

// After validation, remove old keys
// Both old and new keys are active during transition
```

### Security Considerations

**DO:**
- ✅ Store keys in environment variables or secure vaults
- ✅ Use different keys for different environments (dev/staging/prod)
- ✅ Rotate keys regularly (every 90 days recommended)
- ✅ Monitor key usage for anomalies
- ✅ Use multi-key setups for critical applications

**DON'T:**
- ❌ Commit keys to version control
- ❌ Share keys across multiple applications
- ❌ Log full API keys (use masking)
- ❌ Store keys in plain text configuration files
- ❌ Use the same key for all environments

**API Key Format Validation:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/auth"

securityUtils := auth.NewSecurityUtils(nil)

// Validate format before use
err := securityUtils.ValidateAPIKeyFormat("sk-ant-api03-...", "anthropic")
if err != nil {
    log.Fatal("Invalid API key format:", err)
}

// Expected formats:
// OpenAI:    sk-* or sk-proj-*
// Anthropic: sk-ant-*
// Others:    Provider-specific validation
```

---

## OAuth 2.0 Authentication

OAuth 2.0 provides secure, token-based authentication with automatic refresh capabilities. Supported by Gemini and Qwen providers.

### OAuth Flow Overview

```
┌─────────────┐                                    ┌──────────────┐
│             │  1. Request Device Code            │              │
│             │ ──────────────────────────────────>│              │
│             │                                    │              │
│             │  2. Return User Code & URL         │   OAuth      │
│   Client    │ <──────────────────────────────────│   Server     │
│ Application │                                    │  (Google/    │
│             │  4. Poll for Token                 │   Qwen)      │
│             │ ──────────────────────────────────>│              │
│             │                                    │              │
│             │  5. Return Access & Refresh Token  │              │
│             │ <──────────────────────────────────│              │
└─────────────┘                                    └──────────────┘
       │                                                  ▲
       │                                                  │
       │ 3. User Opens Browser & Authorizes              │
       └─────────────────────────────────────────────────┘
```

### Initial Setup with Device Code Flow

**Gemini (Google) Example:**

```go
package main

import (
    "context"
    "fmt"
    "time"

    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
)

const (
    GoogleClientID     = "your-client-id.apps.googleusercontent.com"
    GoogleClientSecret = "GOCSPX-your-client-secret"
    GoogleScope        = "https://www.googleapis.com/auth/cloud-platform"
)

func authenticateWithDeviceCode() error {
    ctx := context.Background()

    // Step 1: Request device code
    deviceCodeResp, err := requestDeviceCode(ctx)
    if err != nil {
        return err
    }

    // Step 2: Display user code to user
    fmt.Printf("\n==============================================\n")
    fmt.Printf("Please visit: %s\n", deviceCodeResp.VerificationURL)
    fmt.Printf("Enter code: %s\n", deviceCodeResp.UserCode)
    fmt.Printf("==============================================\n\n")

    // Step 3: Open browser automatically (optional)
    openBrowser(deviceCodeResp.VerificationURL)

    // Step 4: Poll for token
    token, err := pollForToken(ctx, deviceCodeResp.DeviceCode)
    if err != nil {
        return err
    }

    // Step 5: Save tokens
    return saveTokens(token)
}

func requestDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
    data := url.Values{}
    data.Set("client_id", GoogleClientID)
    data.Set("scope", GoogleScope)

    resp, err := http.Post(
        "https://oauth2.googleapis.com/device/code",
        "application/x-www-form-urlencoded",
        strings.NewReader(data.Encode()),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var deviceCode DeviceCodeResponse
    if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
        return nil, err
    }

    return &deviceCode, nil
}

type DeviceCodeResponse struct {
    DeviceCode              string `json:"device_code"`
    UserCode                string `json:"user_code"`
    VerificationURL         string `json:"verification_url"`
    ExpiresIn               int    `json:"expires_in"`
    Interval                int    `json:"interval"`
}
```

**Complete Working Example:**

See `examples/gemini-oauth-flow/` and `examples/qwen-oauth-flow/` for full implementation.

### Token Refresh Mechanism

Tokens are automatically refreshed before expiration with zero downtime.

**Automatic Refresh:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/oauthmanager"

// Define provider-specific refresh function
refreshFunc := func(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
    oauth2Config := &oauth2.Config{
        ClientID:     cred.ClientID,
        ClientSecret: cred.ClientSecret,
        Endpoint:     google.Endpoint,
    }

    token := &oauth2.Token{
        RefreshToken: cred.RefreshToken,
    }

    newToken, err := oauth2Config.TokenSource(ctx, token).Token()
    if err != nil {
        return nil, err
    }

    // Return updated credential
    updated := *cred
    updated.AccessToken = newToken.AccessToken
    updated.RefreshToken = newToken.RefreshToken
    updated.ExpiresAt = newToken.Expiry
    updated.LastRefresh = time.Now()
    updated.RefreshCount++

    return &updated, nil
}

// Create OAuth manager with automatic refresh
manager := oauthmanager.NewOAuthKeyManager("Gemini", credentials, refreshFunc)

// Tokens are automatically refreshed before expiration (default: 5 min buffer)
result, usage, err := manager.ExecuteWithFailover(ctx, operation)
```

**Manual Refresh:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/auth"

authenticator := auth.NewOAuthAuthenticator("gemini", storage, oauthConfig)

// Manually trigger refresh
err := authenticator.RefreshToken(ctx)
if err != nil {
    log.Fatal("Token refresh failed:", err)
}

// Get updated token
token, err := authenticator.GetToken()
```

### Multi-Credential OAuth Setup

Multiple OAuth credentials provide load balancing and redundancy.

**Configuration:**

```yaml
providers:
  gemini:
    oauth_credentials:
      - id: team-account
        client_id: client-id-1
        client_secret: client-secret-1
        access_token: ya29.a0...
        refresh_token: 1//06...
        expires_at: "2025-11-18T10:00:00Z"
        scopes:
          - https://www.googleapis.com/auth/cloud-platform

      - id: personal-account
        client_id: client-id-2
        client_secret: client-secret-2
        access_token: ya29.a0...
        refresh_token: 1//06...
        expires_at: "2025-11-18T11:00:00Z"
        scopes:
          - https://www.googleapis.com/auth/cloud-platform
```

**Programmatic Setup:**

```go
credentials := []*types.OAuthCredentialSet{
    {
        ID:           "team-account",
        ClientID:     os.Getenv("GEMINI_CLIENT_ID_1"),
        ClientSecret: os.Getenv("GEMINI_CLIENT_SECRET_1"),
        AccessToken:  loadToken("team-account", "access"),
        RefreshToken: loadToken("team-account", "refresh"),
        ExpiresAt:    loadExpiry("team-account"),
        OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
            // Persist updated tokens to storage
            return saveTokens(id, access, refresh, expires)
        },
    },
    {
        ID:           "personal-account",
        ClientID:     os.Getenv("GEMINI_CLIENT_ID_2"),
        ClientSecret: os.Getenv("GEMINI_CLIENT_SECRET_2"),
        AccessToken:  loadToken("personal-account", "access"),
        RefreshToken: loadToken("personal-account", "refresh"),
        ExpiresAt:    loadExpiry("personal-account"),
        OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
            return saveTokens(id, access, refresh, expires)
        },
    },
}

manager := oauthmanager.NewOAuthKeyManager("Gemini", credentials, refreshFunc)
```

### Token Storage and Encryption

Tokens must be stored securely with encryption at rest.

**File-Based Storage with Encryption:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/auth"

// Configure encrypted token storage
config := &auth.Config{
    TokenStorage: auth.TokenStorageConfig{
        Type: "file",
        File: auth.FileStorageConfig{
            Directory:            "./tokens",
            FilePermissions:      "0600",  // Read/write by owner only
            DirectoryPermissions: "0700",  // Access by owner only
        },
        Encryption: auth.EncryptionConfig{
            Enabled:   true,
            Key:       "your-32-byte-encryption-key!",
            Algorithm: "aes-256-gcm",
            KeyDerivation: auth.KeyDerivationConfig{
                Function:   "pbkdf2",
                Iterations: 100000,
                KeyLength:  32,
            },
        },
    },
}

// Create encrypted storage
storage, err := auth.NewFileTokenStorage(
    &config.TokenStorage.File,
    &config.TokenStorage.Encryption,
)
if err != nil {
    log.Fatal(err)
}

// Store token (automatically encrypted)
err = storage.StoreToken("gemini", &types.OAuthConfig{
    AccessToken:  "ya29.a0...",
    RefreshToken: "1//06...",
    ExpiresAt:    time.Now().Add(1 * time.Hour),
})

// Retrieve token (automatically decrypted)
token, err := storage.RetrieveToken("gemini")
```

**Token Storage Best Practices:**

```go
// 1. Use file permissions to restrict access
filePermissions := 0600  // rw------- (owner only)
dirPermissions := 0700   // rwx------ (owner only)

// 2. Enable encryption for all stored tokens
encryptionConfig := auth.EncryptionConfig{
    Enabled:   true,
    Key:       generateSecureKey(32),  // Use crypto/rand
    Algorithm: "aes-256-gcm",
}

// 3. Implement secure key derivation
keyDerivation := auth.KeyDerivationConfig{
    Function:   "pbkdf2",
    Iterations: 100000,  // OWASP recommended minimum
    KeyLength:  32,      // 256 bits
}

// 4. Regular cleanup of expired tokens
storage.CleanupExpired()

// 5. Backup tokens securely (encrypted backups)
backupConfig := auth.BackupConfig{
    Enabled:  true,
    Interval: 24 * time.Hour,
    MaxFiles: 10,
}
```

---

## Multi-Credential Management

The SDK provides two specialized managers for handling multiple credentials.

### KeyManager for API Keys

**Features:**
- Round-robin load balancing
- Health tracking per key
- Exponential backoff for failed keys
- Automatic failover (up to 3 attempts)
- Thread-safe concurrent access

**Implementation:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/keymanager"

// Create manager
manager := keymanager.NewKeyManager("Anthropic", []string{
    "sk-ant-key-1",
    "sk-ant-key-2",
    "sk-ant-key-3",
})

// Execute with automatic failover
result, usage, err := manager.ExecuteWithFailover(ctx,
    func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
        // The manager provides a healthy key
        return makeAPICall(ctx, apiKey)
    })

// Report results for health tracking
if err != nil {
    manager.ReportFailure(apiKey, err)
} else {
    manager.ReportSuccess(apiKey)
}
```

### OAuthManager for OAuth Credentials

**Features:**
- All KeyManager features plus:
- Automatic token refresh
- Token expiration detection
- Per-credential metrics
- Refresh strategies (Default, Adaptive, Conservative)
- Token rotation policies
- Monitoring and alerting

**Implementation:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/oauthmanager"

// Define refresh function
refreshFunc := func(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
    // Provider-specific token refresh logic
    return refreshedCredential, nil
}

// Create manager
manager := oauthmanager.NewOAuthKeyManager("Gemini", credentials, refreshFunc)

// Use with automatic token refresh and failover
result, usage, err := manager.ExecuteWithFailover(ctx,
    func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
        // Manager ensures token is fresh (refreshes if needed)
        return makeAPICall(ctx, cred.AccessToken)
    })
```

### Load Balancing Strategies

**Round-Robin (Default):**

```go
// Distributes requests evenly across all healthy credentials
// Request 1 → credential-1
// Request 2 → credential-2
// Request 3 → credential-3
// Request 4 → credential-1 (cycle repeats)

// Automatically skips credentials in backoff period
```

**Health-Based Selection:**

```go
// Only healthy credentials are selected
// Credentials with ≥3 consecutive failures are marked unhealthy
// Unhealthy credentials enter exponential backoff: 1s → 2s → 4s → 8s → max 60s
// Credentials recover after successful request
```

### Failover Configuration

**Automatic Failover:**

```go
// KeyManager attempts up to 3 keys per request
attemptsLimit := min(len(keys), 3)

// OAuthManager attempts up to 3 credentials per request
// Each attempt may trigger token refresh if needed

// Failover process:
// 1. Select next healthy credential (round-robin)
// 2. Check if token needs refresh (OAuth only)
// 3. Refresh token if needed (OAuth only)
// 4. Execute operation
// 5. If failed, report failure and try next credential
// 6. Repeat until success or all attempts exhausted
```

**Custom Failover Logic:**

```go
// Implement custom retry logic
maxAttempts := 5  // Try more credentials
var lastErr error

for attempt := 0; attempt < maxAttempts; attempt++ {
    key, err := manager.GetNextKey()
    if err != nil {
        break
    }

    result, usage, err := operation(ctx, key)
    if err != nil {
        lastErr = err
        manager.ReportFailure(key, err)

        // Custom backoff
        time.Sleep(time.Duration(attempt) * time.Second)
        continue
    }

    manager.ReportSuccess(key)
    return result, usage, nil
}

return "", nil, lastErr
```

### Health Tracking and Recovery

**Health Status Structure:**

```go
type keyHealth struct {
    failureCount int           // Consecutive failures
    lastFailure  time.Time     // Last failure timestamp
    lastSuccess  time.Time     // Last success timestamp
    isHealthy    bool          // Healthy if < 3 failures
    backoffUntil time.Time     // When key becomes available
}

type credentialHealth struct {
    keyHealth                  // Inherits key health
    refreshFailCount int       // Separate tracking for refresh failures
    lastRefreshError error     // Last refresh error
}
```

**Monitoring Health:**

```go
// Get credential health info
info := manager.GetCredentialHealthInfo("team-account")
fmt.Printf("Healthy: %v\n", info.IsHealthy)
fmt.Printf("Available: %v\n", info.IsAvailable)
fmt.Printf("Failure Count: %d\n", info.FailureCount)
fmt.Printf("Success Rate: %.2f%%\n", info.SuccessRate*100)

// Get health summary
summary := manager.GetHealthSummary()
fmt.Printf("Total Credentials: %d\n", summary["total_credentials"])
fmt.Printf("Healthy: %d\n", summary["healthy_credentials"])
fmt.Printf("In Backoff: %d\n", summary["backoff_credentials"])
fmt.Printf("Overall Success Rate: %.2f%%\n", summary["success_rate"].(float64)*100)
```

**Recovery Process:**

```go
// Credentials automatically recover after successful request
func (m *KeyManager) ReportSuccess(key string) {
    health := m.keyHealth[key]
    health.lastSuccess = time.Now()
    health.failureCount = 0         // Reset failure count
    health.isHealthy = true          // Mark as healthy
    health.backoffUntil = time.Time{} // Clear backoff
}

// Manual recovery (force reset)
manager.ReportSuccess(key)  // Resets all failure counters
```

---

## Token Management

### Automatic Token Refresh

OAuth tokens are automatically refreshed before expiration.

**Default Refresh Behavior:**

```go
// Tokens are refreshed when:
// 1. Expires within 5 minutes (default buffer)
// 2. Request is made with expired token
// 3. Refresh is triggered manually

// Refresh is automatic and transparent
result, usage, err := manager.ExecuteWithFailover(ctx, operation)
// Token is guaranteed fresh (or error is returned)
```

**Refresh Configuration:**

```go
// In pkg/auth/config.go
type RefreshConfig struct {
    Enabled          bool          // Enable automatic refresh
    Buffer           time.Duration // Refresh buffer (default: 5 min)
    MaxRetries       int           // Max refresh attempts (default: 3)
    RefreshOnFailure bool          // Refresh on API failure
}

// Configure in auth config
oauthConfig := &auth.OAuthConfig{
    Refresh: auth.RefreshConfig{
        Enabled:          true,
        Buffer:           5 * time.Minute,
        MaxRetries:       3,
        RefreshOnFailure: true,
    },
}
```

### Refresh Strategies

**Default Strategy (5-minute buffer):**

```go
manager.SetRefreshStrategy(oauthmanager.DefaultRefreshStrategy())

// Refreshes tokens when they expire within 5 minutes
// Simple and predictable
// Good for most use cases
```

**Adaptive Strategy (adjusts based on usage):**

```go
manager.SetRefreshStrategy(oauthmanager.AdaptiveRefreshStrategy())

// Automatically adjusts buffer time based on:
// - Average request latency (higher latency = larger buffer)
// - Request rate (higher rate = larger buffer)
// - Error rate (more errors = larger buffer)
//
// Example adjustments:
// - High latency (100ms): +30s buffer per 100ms
// - High request rate (10+ req/hour): +30s per 10 requests
// - Error rate <95%: +(0.95-rate)*600s buffer
//
// Ensures tokens are refreshed in time under varying load
```

**Conservative Strategy (15-minute buffer):**

```go
manager.SetRefreshStrategy(oauthmanager.ConservativeRefreshStrategy())

// Refreshes tokens 15 minutes before expiry
// Enables preemptive refresh for high-traffic credentials
// Maximum safety margin
// Higher refresh frequency
```

**Custom Strategy:**

```go
customStrategy := &oauthmanager.RefreshStrategy{
    BufferTime:               10 * time.Minute, // Base buffer
    AdaptiveBuffer:           true,             // Enable adaptation
    MinBuffer:                2 * time.Minute,  // Minimum buffer
    MaxBuffer:                30 * time.Minute, // Maximum buffer
    PreemptiveRefresh:        true,             // Early refresh under load
    HighTrafficThreshold:     100,              // 100 req/hour = high traffic
}
manager.SetRefreshStrategy(customStrategy)
```

### Token Rotation Policies

Implement zero-downtime credential rotation with grace periods.

**Configuration:**

```go
policy := &oauthmanager.RotationPolicy{
    Enabled:          true,
    RotationInterval: 30 * 24 * time.Hour, // Rotate every 30 days
    GracePeriod:      7 * 24 * time.Hour,  // 7-day overlap
    AutoDecommission: true,                 // Auto-remove old credentials

    // Optional callbacks
    OnRotationNeeded: func(credentialID string) error {
        log.Printf("Credential %s needs rotation\n", credentialID)
        // Send notification, create ticket, etc.
        return nil
    },
    OnDecommission: func(credentialID string) error {
        log.Printf("Decommissioning credential %s\n", credentialID)
        // Cleanup, archival, etc.
        return nil
    },
}

manager.SetRotationPolicy(policy)
```

**Rotation Process:**

```go
// 1. Check which credentials need rotation
needsRotation := manager.CheckRotationNeeded()
for _, credID := range needsRotation {
    fmt.Printf("Credential %s needs rotation\n", credID)
}

// 2. Create new credential
newCred := &types.OAuthCredentialSet{
    ID:           "new-team-account",
    ClientID:     "new-client-id",
    ClientSecret: "new-client-secret",
    // Authenticate and get tokens via OAuth flow
}

// 3. Mark old credential for rotation
err := manager.MarkForRotation("old-team-account", newCred)

// 4. During grace period (7 days):
//    - Both credentials are active
//    - Load is gradually shifted to new credential
//    - Old credential continues to work

// 5. After grace period:
//    - Old credential is automatically decommissioned
//    - OnDecommission callback is called
//    - Only new credential remains active

// Zero downtime throughout the entire process!
```

### Grace Periods and Zero-Downtime Rotation

**How Grace Periods Work:**

```
Day 0: Rotation initiated
       Old Credential: Active (100% traffic)
       New Credential: Added (0% traffic)

Day 1-7: Grace period
         Old Credential: Active (degrading)
         New Credential: Active (ramping up)
         Both credentials handle requests via round-robin

Day 7: Grace period expires
       Old Credential: Decommissioned (0% traffic)
       New Credential: Active (100% traffic)
```

**Implementation Details:**

```go
type RotationPolicy struct {
    Enabled          bool
    RotationInterval time.Duration  // When to rotate
    GracePeriod      time.Duration  // Overlap period
    AutoDecommission bool           // Auto-remove after grace period

    // Callbacks for lifecycle management
    OnRotationNeeded func(credentialID string) error
    OnDecommission   func(credentialID string) error
}

// Grace period tracking per credential
type credentialRotationState struct {
    markedForRotation bool
    rotationStarted   time.Time
    gracePeriodEnds   time.Time
    replacement       *types.OAuthCredentialSet
}
```

### Refresh Callbacks for Persistence

Token refresh callbacks enable automatic persistence of updated tokens.

**Callback Implementation:**

```go
type OAuthCredentialSet struct {
    // ... other fields ...

    // Callback invoked after successful token refresh
    OnTokenRefresh func(id, accessToken, refreshToken string, expiresAt time.Time) error
}

// Example: Save to YAML config file
credentials := []*types.OAuthCredentialSet{
    {
        ID:           "team-account",
        ClientID:     "client-id",
        ClientSecret: "client-secret",
        OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
            return saveToYAML(id, access, refresh, expires)
        },
    },
}

func saveToYAML(id, access, refresh string, expires time.Time) error {
    // Load existing config
    config, err := loadConfig("~/.config/app/config.yaml")
    if err != nil {
        return err
    }

    // Update tokens for this credential
    for i, cred := range config.Providers.Gemini.OAuthCredentials {
        if cred.ID == id {
            config.Providers.Gemini.OAuthCredentials[i].AccessToken = access
            config.Providers.Gemini.OAuthCredentials[i].RefreshToken = refresh
            config.Providers.Gemini.OAuthCredentials[i].ExpiresAt = expires
            break
        }
    }

    // Save atomically
    return saveConfigAtomic(config, "~/.config/app/config.yaml")
}
```

**Database Persistence:**

```go
OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
    query := `
        UPDATE oauth_credentials
        SET access_token = $1,
            refresh_token = $2,
            expires_at = $3,
            updated_at = NOW()
        WHERE credential_id = $4
    `
    _, err := db.Exec(query, access, refresh, expires, id)
    return err
}
```

**Encrypted Storage Persistence:**

```go
OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
    // Encrypt tokens before storage
    encryptedAccess, err := encrypt(access, encryptionKey)
    if err != nil {
        return err
    }

    encryptedRefresh, err := encrypt(refresh, encryptionKey)
    if err != nil {
        return err
    }

    // Save to secure storage
    return storage.Save(id, encryptedAccess, encryptedRefresh, expires)
}
```

---

## Security Features

### AES-256-GCM Encryption

The SDK uses AES-256-GCM (Galois/Counter Mode) for authenticated encryption.

**Implementation:**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/auth"

securityUtils := auth.NewSecurityUtils(nil)

// Generate secure encryption key (32 bytes for AES-256)
key := make([]byte, 32)
_, err := rand.Read(key)

// Encrypt sensitive data
encryptedData, err := securityUtils.EncryptSensitiveData(
    []byte("sensitive-token"),
    key,
)

// Decrypt sensitive data
decryptedData, err := securityUtils.DecryptSensitiveData(
    encryptedData,
    key,
)
```

**Encryption Properties:**

- **Algorithm**: AES-256-GCM (Advanced Encryption Standard)
- **Key Size**: 256 bits (32 bytes)
- **Mode**: GCM (Galois/Counter Mode) - provides both confidentiality and authenticity
- **Nonce**: Random 12-byte nonce generated per encryption
- **Authentication**: Built-in authentication tag prevents tampering

**Why AES-256-GCM?**

✅ **Authenticated Encryption** - Prevents tampering
✅ **AEAD Cipher** - Authentication and encryption in one operation
✅ **NIST Approved** - Government-grade security
✅ **Performance** - Hardware-accelerated on modern CPUs
✅ **Nonce Misuse Resistance** - Random nonce per operation

### PKCE Implementation

Proof Key for Code Exchange (PKCE) enhances OAuth security for public clients.

**Configuration:**

```go
pkceConfig := auth.PKCEConfig{
    Enabled:        true,
    Method:         "S256",  // SHA-256 hashing
    VerifierLength: 128,     // Length of code verifier
}

oauthConfig := &auth.OAuthConfig{
    PKCE: pkceConfig,
    // ... other config
}
```

**How PKCE Works:**

```go
// 1. Generate code verifier (random string)
verifier := generateRandomString(128)
// Example: "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

// 2. Generate code challenge (SHA-256 hash of verifier)
hash := sha256.Sum256([]byte(verifier))
challenge := base64.RawURLEncoding.EncodeToString(hash[:])
// Example: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

// 3. Send challenge with authorization request
authURL := buildAuthURL(challenge, "S256")

// 4. Send verifier with token request
token := exchangeCodeForToken(authorizationCode, verifier)

// Server verifies: SHA256(verifier) == challenge
```

**PKCE Methods:**

```go
// S256 (Recommended) - SHA-256 hashing
challenge := base64.RawURLEncoding.EncodeToString(
    sha256.Sum256([]byte(verifier))[:],
)

// plain (Not recommended) - No hashing
challenge := verifier
```

### State Validation

OAuth state parameter prevents CSRF attacks.

**Configuration:**

```go
stateConfig := auth.StateConfig{
    Length:           32,              // State string length
    Expiration:       10 * time.Minute, // How long state is valid
    EnableValidation: true,            // Enable validation
}

oauthConfig := &auth.OAuthConfig{
    State: stateConfig,
    // ... other config
}
```

**State Flow:**

```go
// 1. Generate random state
state := generateSecureState(32)
// Example: "k8h3l2j4h5g6f7d8s9a0p1o2i3u4y5t6"

// 2. Store state with expiration
stateStore[state] = StateEntry{
    Value:     state,
    ExpiresAt: time.Now().Add(10 * time.Minute),
}

// 3. Send state with authorization URL
authURL := buildAuthURL(state)

// 4. Validate state in callback
func handleCallback(code, returnedState string) error {
    entry, exists := stateStore[returnedState]
    if !exists {
        return errors.New("invalid state - possible CSRF attack")
    }

    if time.Now().After(entry.ExpiresAt) {
        return errors.New("state expired")
    }

    // State is valid, proceed with token exchange
    delete(stateStore, returnedState)  // One-time use
    return exchangeToken(code)
}
```

**Security Benefits:**

✅ **CSRF Protection** - Prevents cross-site request forgery
✅ **Replay Protection** - One-time use prevents replay attacks
✅ **Session Binding** - Ties authorization to specific session
✅ **Expiration** - Limited validity window

### Token Storage Security

**File Permissions:**

```go
config := auth.FileStorageConfig{
    Directory:            "./tokens",
    FilePermissions:      "0600",  // rw------- (owner only)
    DirectoryPermissions: "0700",  // rwx------ (owner only)
}

// On Unix systems:
// 0600 = User: read/write, Group: none, Others: none
// 0700 = User: read/write/execute, Group: none, Others: none
```

**Encryption at Rest:**

```go
config := auth.EncryptionConfig{
    Enabled:   true,
    Key:       "your-32-byte-encryption-key!",
    Algorithm: "aes-256-gcm",
    KeyDerivation: auth.KeyDerivationConfig{
        Function:   "pbkdf2",
        Iterations: 100000,  // OWASP recommended minimum
        KeyLength:  32,
    },
}

// Tokens are encrypted before writing to disk
// Decrypted only when loaded into memory
// Keys are derived from master key using PBKDF2
```

**Secure Deletion:**

```go
// Overwrite file content before deletion
func secureDelete(filepath string) error {
    // 1. Open file
    file, err := os.OpenFile(filepath, os.O_WRONLY, 0)
    if err != nil {
        return err
    }
    defer file.Close()

    // 2. Get file size
    stat, err := file.Stat()
    if err != nil {
        return err
    }

    // 3. Overwrite with zeros
    zeros := make([]byte, stat.Size())
    _, err = file.Write(zeros)
    if err != nil {
        return err
    }

    // 4. Sync to disk
    file.Sync()

    // 5. Remove file
    return os.Remove(filepath)
}
```

### Audit Logging

Comprehensive security event logging for compliance and monitoring.

**Configuration:**

```go
securityConfig := &auth.SecurityConfig{
    AuditLogging: true,
    TokenMasking: auth.TokenMaskingConfig{
        Enabled:      true,
        PrefixLength: 8,   // Show first 8 chars
        SuffixLength: 4,   // Show last 4 chars
        MaskChar:     "*", // Mask with asterisks
    },
}

auditor := auth.NewSecurityAuditor(logger, securityConfig)
```

**Logged Events:**

```go
// Authentication attempts
auditor.LogAuthAttempt("anthropic", "api_key", true, "192.168.1.100")
// Output: [INFO] Authentication attempt provider=anthropic method=api_key status=SUCCESS ip=192.168.1.100

// Token operations
auditor.LogTokenOperation("refresh", "gemini", true)
// Output: [INFO] Token operation operation=refresh provider=gemini status=SUCCESS

// Security events
auditor.LogSecurityEvent("invalid_state", "gemini", "State mismatch detected")
// Output: [WARN] Security event event=invalid_state provider=gemini details="State mismatch detected"
```

**Token Masking:**

```go
securityUtils := auth.NewSecurityUtils(securityConfig)

token := "sk-ant-api03-1234567890abcdefghij"
masked := securityUtils.MaskToken(token)
// Output: "sk-ant-a*********************ghij"

// Prefix: 8 chars, Suffix: 4 chars, Middle: masked
```

**Audit Log Format:**

```json
{
  "timestamp": "2025-11-18T10:30:00Z",
  "level": "INFO",
  "event_type": "authentication_attempt",
  "provider": "anthropic",
  "method": "api_key",
  "status": "success",
  "ip_address": "192.168.1.100",
  "user_agent": "ai-provider-kit/1.0",
  "metadata": {
    "api_key": "sk-ant-a***ghij",
    "duration_ms": 123
  }
}
```

---

## Provider-Specific Authentication

### Anthropic (API Key + OAuth)

**API Key Authentication:**

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeAnthropic,
    APIKey: "sk-ant-api03-...",
    Model: "claude-3-5-sonnet-20241022",
}
```

**Multi-Key Setup:**

```yaml
providers:
  anthropic:
    api_keys:
      - sk-ant-api03-key-1
      - sk-ant-api03-key-2
      - sk-ant-api03-key-3
    model: claude-3-5-sonnet-20241022
```

**Headers:**

```
Authorization: x-api-key sk-ant-api03-...
anthropic-version: 2023-06-01
```

### OpenAI (API Key + OAuth)

**API Key Authentication:**

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeOpenAI,
    APIKey: "sk-proj-...",
    Model: "gpt-4",
}
```

**OAuth 2.0 Authentication (NEW):**

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeOpenAI,
    OAuthCredentials: []*types.OAuthCredentialSet{
        {
            ID:           "personal",
            ClientID:     "app_EMoamEEZ73f0CkXaXp7hrann", // Public client
            AccessToken:  "access-token...",
            RefreshToken: "refresh-token...",
            ExpiresAt:    time.Now().Add(24 * time.Hour),
            Scopes:       []string{"openid", "profile", "email", "offline_access"},
        },
    },
    ProviderConfig: map[string]interface{}{
        "oauth_client_id":   "app_EMoamEEZ73f0CkXaXp7hrann",
        "organization_id":   "org-123", // Optional
    },
}
```

**OpenAI OAuth Features:**
- PKCE with S256 method
- Form-encoded token requests
- Account type detection (personal vs enterprise)
- Automatic token refresh
- Organization header support

**Headers:**

```
# API Key mode
Authorization: Bearer sk-proj-...

# OAuth mode
Authorization: Bearer ya29.access-token...
openai-organization: org-123  # Optional
```

### Gemini (OAuth + API Key)

**OAuth Authentication (Recommended):**

```go
credentials := []*types.OAuthCredentialSet{
    {
        ID:           "default",
        ClientID:     "681255809395-....apps.googleusercontent.com",
        ClientSecret: "GOCSPX-...",
        AccessToken:  "ya29.a0...",
        RefreshToken: "1//06...",
        ExpiresAt:    time.Now().Add(1 * time.Hour),
        Scopes:       []string{"https://www.googleapis.com/auth/cloud-platform"},
    },
}
```

**Device Code Flow:**

```bash
# Run OAuth flow helper
cd examples/gemini-oauth-flow
go run main.go

# Follow prompts:
# 1. Visit https://www.google.com/device
# 2. Enter code: ABCD-EFGH
# 3. Tokens saved to ~/.mcp-code-api/config.yaml
```

**API Key Authentication (NEW):**

```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeGemini,
    APIKey: "AIza...",
    Model:  "gemini-2.0-flash-exp",
}

// Multi-key support with failover
config := types.ProviderConfig{
    Type: types.ProviderTypeGemini,
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{
            "AIza-key1...",
            "AIza-key2...",
            "AIza-key3...",
        },
    },
}
```

**Gemini API Key Features:**
- Simpler setup than OAuth
- Multi-key failover support
- Uses standard Gemini API endpoint
- No project ID required
- 100 requests/day free tier

**Headers:**

```
# OAuth mode (CloudCode API)
Authorization: Bearer ya29.a0...
x-goog-user-project: your-project-id

# API Key mode (Gemini API)
x-goog-api-key: AIza...
```

### Qwen (OAuth Device Flow)

**OAuth Authentication:**

```go
credentials := []*types.OAuthCredentialSet{
    {
        ID:           "default",
        ClientID:     "qwen-client-id",
        ClientSecret: "qwen-client-secret",
        AccessToken:  "access-token",
        RefreshToken: "refresh-token",
        ExpiresAt:    time.Now().Add(1 * time.Hour),
    },
}
```

**Device Code Flow:**

```bash
# Run OAuth flow helper
cd examples/qwen-oauth-flow
go run main.go
```

**Headers:**

```
Authorization: Bearer access-token
```

### Cerebras (API Key)

**API Key Authentication:**

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeCerebras,
    APIKey: "csk-...",
    Model: "llama3.1-8b",
}
```

**Headers:**

```
Authorization: Bearer csk-...
```

### OpenRouter (API Key + OAuth)

**API Key Authentication:**

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeOpenRouter,
    APIKey: "sk-or-v1-...",
    Model: "anthropic/claude-3-opus",
}
```

**OAuth PKCE Flow (NEW):**

```go
// Step 1: Start OAuth flow
authURL, flowState, err := provider.StartOAuthFlow(ctx, "https://yourapp.com/callback")
// User visits authURL and authorizes

// Step 2: Handle callback
apiKey, err := provider.HandleOAuthCallback(ctx, authCode, flowState)
// apiKey is automatically added to the key manager pool

// The obtained API key works identically to manually configured keys
```

**OpenRouter OAuth Features:**
- PKCE with S256 method for security
- No client_id/client_secret required
- Returns permanent API keys (not temporary tokens)
- Keys auto-added to failover pool
- User-controlled spending limits

**OAuth Configuration:**

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeOpenRouter,
    ProviderConfig: map[string]interface{}{
        "oauth_callback_url": "https://yourapp.com/oauth/callback",
    },
}

provider, _ := factory.CreateProvider(types.ProviderTypeOpenRouter, config)

// OAuth flow generates API keys that work like regular keys
authURL, flowState, _ := provider.StartOAuthFlow(ctx, "")
```

**Headers:**

```
Authorization: Bearer sk-or-v1-...
HTTP-Referer: https://your-app.com  # Optional
X-Title: Your App Name              # Optional
```

---

## Monitoring and Alerts

### Credential Health Monitoring

**Real-Time Health Status:**

```go
// Get health info for specific credential
info := manager.GetCredentialHealthInfo("team-account")

fmt.Printf("Credential: %s\n", info.ID)
fmt.Printf("Healthy: %v\n", info.IsHealthy)
fmt.Printf("Available: %v\n", info.IsAvailable)
fmt.Printf("Requests: %d\n", info.RequestCount)
fmt.Printf("Success Rate: %.2f%%\n", info.SuccessRate*100)
fmt.Printf("Avg Latency: %v\n", info.AverageLatency)
fmt.Printf("Last Success: %v\n", info.LastSuccess)
fmt.Printf("Last Failure: %v\n", info.LastFailure)
fmt.Printf("Failure Count: %d\n", info.FailureCount)
fmt.Printf("Backoff Until: %v\n", info.BackoffUntil)

// Get overall health summary
summary := manager.GetHealthSummary()
fmt.Printf("Total Credentials: %d\n", summary["total_credentials"])
fmt.Printf("Healthy: %d\n", summary["healthy_credentials"])
fmt.Printf("Unhealthy: %d\n", summary["unhealthy_credentials"])
fmt.Printf("In Backoff: %d\n", summary["backoff_credentials"])
fmt.Printf("Success Rate: %.2f%%\n", summary["success_rate"].(float64)*100)
```

### Webhook Notifications

**Configuration:**

```go
monitoringConfig := &oauthmanager.MonitoringConfig{
    // Webhook settings
    WebhookURL:    "https://your-monitoring-service.com/webhook",
    WebhookEvents: []string{"refresh", "failure", "rotation", "expiry_warning"},

    // Alert thresholds
    AlertOnHighFailureRate: true,
    FailureRateThreshold:   0.25,  // Alert if >25% failure rate

    AlertOnRefreshFailure: true,

    AlertOnExpirySoon:  true,
    ExpiryWarningTime: 24 * time.Hour,  // Warn 24h before expiry
}

manager.SetMonitoringConfig(monitoringConfig)
```

**Webhook Event Format:**

```json
{
  "type": "failure",
  "timestamp": "2025-11-18T10:30:00Z",
  "credential_id": "team-account",
  "provider": "Gemini",
  "message": "High failure rate detected: 35.0%",
  "severity": "warning",
  "details": {
    "failure_rate": 0.35,
    "success_rate": 0.65,
    "total_requests": 100,
    "error_count": 35,
    "last_error": "rate limit exceeded"
  }
}
```

**Event Types:**

```go
// Token refresh events
{
  "type": "refresh",
  "message": "Token refreshed successfully",
  "credential_id": "team-account",
  "details": {
    "old_expiry": "2025-11-18T10:00:00Z",
    "new_expiry": "2025-11-18T11:00:00Z"
  }
}

// Failure events
{
  "type": "failure",
  "message": "API request failed",
  "credential_id": "team-account",
  "details": {
    "error": "rate limit exceeded",
    "status_code": 429,
    "retry_after": 60
  }
}

// Rotation events
{
  "type": "rotation",
  "message": "Credential rotation initiated",
  "credential_id": "old-team-account",
  "details": {
    "new_credential_id": "new-team-account",
    "grace_period_ends": "2025-11-25T10:00:00Z"
  }
}

// Expiry warnings
{
  "type": "expiry_warning",
  "message": "Token expires soon",
  "credential_id": "team-account",
  "details": {
    "expires_at": "2025-11-19T10:00:00Z",
    "time_remaining": "24h0m0s"
  }
}
```

### Metrics Export (Prometheus, JSON)

**Prometheus Export:**

```go
// Export metrics in Prometheus format
promMetrics := manager.ExportPrometheus()

// Counter metrics
fmt.Printf("# HELP requests_total Total number of requests\n")
fmt.Printf("# TYPE requests_total counter\n")
for credID, count := range promMetrics.RequestsTotal {
    fmt.Printf("requests_total{credential=\"%s\"} %d\n", credID, count)
}

// Success/error counters
fmt.Printf("# HELP success_total Total successful requests\n")
fmt.Printf("# TYPE success_total counter\n")
for credID, count := range promMetrics.SuccessTotal {
    fmt.Printf("success_total{credential=\"%s\"} %d\n", credID, count)
}

// Gauge metrics
fmt.Printf("# HELP tokens_used_total Total tokens consumed\n")
fmt.Printf("# TYPE tokens_used_total counter\n")
for credID, count := range promMetrics.TokensUsed {
    fmt.Printf("tokens_used_total{credential=\"%s\"} %d\n", credID, count)
}

// Histogram metrics
fmt.Printf("# HELP request_duration_seconds Request duration\n")
fmt.Printf("# TYPE request_duration_seconds histogram\n")
for credID, latency := range promMetrics.AverageLatency {
    fmt.Printf("request_duration_seconds{credential=\"%s\"} %f\n",
        credID, latency.Seconds())
}
```

**JSON Export:**

```go
// Export all metrics as JSON
jsonData, err := manager.ExportJSON()
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(jsonData))
```

**JSON Format:**

```json
{
  "provider": "Gemini",
  "timestamp": "2025-11-18T10:30:00Z",
  "credentials": [
    {
      "id": "team-account",
      "health": {
        "is_healthy": true,
        "is_available": true,
        "failure_count": 0,
        "last_success": "2025-11-18T10:29:50Z",
        "last_failure": "2025-11-18T09:15:30Z",
        "backoff_until": "0001-01-01T00:00:00Z"
      },
      "metrics": {
        "request_count": 1250,
        "success_count": 1200,
        "error_count": 50,
        "success_rate": 0.96,
        "tokens_used": 125000,
        "average_latency": "0.350s",
        "requests_per_hour": 75.5,
        "first_used": "2025-11-17T10:00:00Z",
        "last_used": "2025-11-18T10:29:50Z"
      },
      "token": {
        "expires_at": "2025-11-18T11:00:00Z",
        "time_until_expiry": "30m10s",
        "last_refresh": "2025-11-18T10:00:00Z",
        "refresh_count": 24
      }
    }
  ],
  "summary": {
    "total_credentials": 2,
    "healthy_credentials": 2,
    "unhealthy_credentials": 0,
    "backoff_credentials": 0,
    "total_requests": 2500,
    "total_success": 2400,
    "total_errors": 100,
    "overall_success_rate": 0.96,
    "total_tokens_used": 250000
  }
}
```

### Alert Configurations

**Check Alerts Programmatically:**

```go
// Check for alert conditions
alerts := manager.CheckAlerts()

for _, alert := range alerts {
    fmt.Printf("[%s] %s - %s\n", alert.Severity, alert.Type, alert.Message)

    // Send to monitoring system
    sendAlert(alert)
}
```

**Alert Structure:**

```go
type Alert struct {
    Type         string                 // "failure", "expiry", "rotation"
    Severity     string                 // "info", "warning", "critical"
    Message      string                 // Human-readable message
    CredentialID string                 // Affected credential
    Timestamp    time.Time              // When alert was generated
    Details      map[string]interface{} // Additional context
}
```

**Alert Examples:**

```go
// High failure rate alert
Alert{
    Type:         "failure",
    Severity:     "warning",
    Message:      "High failure rate detected: 35.0%",
    CredentialID: "team-account",
    Details: map[string]interface{}{
        "failure_rate": 0.35,
        "threshold":    0.25,
        "action":       "Check API quota and network connectivity",
    },
}

// Token expiry alert
Alert{
    Type:         "expiry",
    Severity:     "critical",
    Message:      "Token expires in 1 hour",
    CredentialID: "team-account",
    Details: map[string]interface{}{
        "expires_at":      "2025-11-18T11:30:00Z",
        "time_remaining":  "1h0m0s",
        "auto_refresh":    true,
    },
}

// Rotation needed alert
Alert{
    Type:         "rotation",
    Severity:     "info",
    Message:      "Credential rotation recommended",
    CredentialID: "team-account",
    Details: map[string]interface{}{
        "age":                "32 days",
        "rotation_interval":  "30 days",
        "grace_period":       "7 days",
    },
}
```

---

## Best Practices

### Security Best Practices

**1. Credential Management**

```go
// DO: Use environment variables
apiKey := os.Getenv("ANTHROPIC_API_KEY")

// DO: Use secret management systems
apiKey := secretManager.GetSecret("anthropic-api-key")

// DON'T: Hardcode credentials
apiKey := "sk-ant-api03-..." // NEVER DO THIS!

// DON'T: Commit to version control
// .gitignore should include:
//   .env
//   config.yaml
//   *.key
//   tokens/
```

**2. Token Storage**

```go
// DO: Use encrypted storage
config := auth.EncryptionConfig{
    Enabled:   true,
    Key:       loadSecureKey(),  // From environment or KMS
    Algorithm: "aes-256-gcm",
}

// DO: Set restrictive file permissions
filePermissions := 0600  // rw------- (owner only)

// DON'T: Store tokens unencrypted
// DON'T: Use world-readable permissions (0644, 0666)
```

**3. Key Rotation**

```go
// DO: Rotate regularly
rotationPolicy := &oauthmanager.RotationPolicy{
    Enabled:          true,
    RotationInterval: 30 * 24 * time.Hour,  // 30 days
    GracePeriod:      7 * 24 * time.Hour,   // 7-day overlap
}

// DO: Implement monitoring
OnRotationNeeded: func(credID string) error {
    sendNotification("Credential rotation needed: " + credID)
    createTicket("ROTATE-" + credID)
    return nil
}

// DON'T: Use credentials indefinitely
// DON'T: Skip grace periods (causes downtime)
```

**4. Audit Logging**

```go
// DO: Enable comprehensive logging
securityConfig := &auth.SecurityConfig{
    AuditLogging: true,
    TokenMasking: auth.TokenMaskingConfig{
        Enabled: true,
    },
}

// DO: Monitor security events
auditor.LogSecurityEvent("invalid_state", "gemini", details)

// DON'T: Log full credentials
log.Printf("Token: %s", token)  // NEVER DO THIS!

// DO: Log masked credentials
log.Printf("Token: %s", securityUtils.MaskToken(token))
```

**5. Network Security**

```go
// DO: Use HTTPS only
config.BaseURL = "https://api.anthropic.com"

// DO: Validate TLS certificates
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    },
}

// DON'T: Disable certificate verification
// DON'T: Use HTTP for production
```

### Operational Best Practices

**1. Multi-Credential Setup**

```go
// DO: Use multiple credentials for resilience
credentials := []*types.OAuthCredentialSet{
    primaryCredential,
    backupCredential1,
    backupCredential2,
}

// DO: Monitor health across all credentials
summary := manager.GetHealthSummary()
if summary["healthy_credentials"].(int) < 2 {
    alertOps("Low credential availability")
}

// DON'T: Rely on single credential for critical services
```

**2. Token Refresh Strategy**

```go
// DO: Use adaptive refresh for production
manager.SetRefreshStrategy(oauthmanager.AdaptiveRefreshStrategy())

// DO: Configure appropriate buffers
customStrategy := &oauthmanager.RefreshStrategy{
    BufferTime:    10 * time.Minute,  // Refresh 10min early
    MinBuffer:     2 * time.Minute,   // Minimum safety margin
    MaxBuffer:     30 * time.Minute,  // Maximum for high-traffic
}

// DON'T: Use default strategy without understanding your traffic patterns
```

**3. Error Handling**

```go
// DO: Implement comprehensive error handling
result, usage, err := manager.ExecuteWithFailover(ctx, operation)
if err != nil {
    // Log error with context
    log.Printf("Operation failed: provider=%s, error=%v", provider, err)

    // Check if recoverable
    if isRecoverable(err) {
        // Retry with backoff
        time.Sleep(calculateBackoff(attempt))
        return retry(ctx, operation, attempt+1)
    }

    // Alert and fail gracefully
    alertOps("Unrecoverable error: " + err.Error())
    return fallbackResponse, err
}

// DON'T: Ignore errors or fail silently
```

**4. Monitoring**

```go
// DO: Set up comprehensive monitoring
monitoringConfig := &oauthmanager.MonitoringConfig{
    WebhookURL:             "https://ops.example.com/webhook",
    WebhookEvents:          []string{"refresh", "failure", "rotation"},
    AlertOnHighFailureRate: true,
    FailureRateThreshold:   0.10,  // Alert at 10% failure rate
}

// DO: Regular health checks
ticker := time.NewTicker(5 * time.Minute)
go func() {
    for range ticker.C {
        summary := manager.GetHealthSummary()
        recordMetrics(summary)

        alerts := manager.CheckAlerts()
        for _, alert := range alerts {
            handleAlert(alert)
        }
    }
}()

// DON'T: Deploy without monitoring
```

**5. Testing**

```go
// DO: Test failover behavior
func TestFailover(t *testing.T) {
    manager := setupTestManager()

    // Simulate primary credential failure
    manager.ReportFailure(primaryKey, errors.New("rate limit"))

    // Verify failover to backup
    result, _, err := manager.ExecuteWithFailover(ctx, operation)
    assert.NoError(t, err)
    assert.NotEmpty(t, result)
}

// DO: Test token refresh
func TestTokenRefresh(t *testing.T) {
    // Create credential expiring soon
    cred := &types.OAuthCredentialSet{
        ExpiresAt: time.Now().Add(2 * time.Minute),
    }

    // Verify refresh is triggered
    refreshed, err := refreshFunc(ctx, cred)
    assert.NoError(t, err)
    assert.True(t, refreshed.ExpiresAt.After(cred.ExpiresAt))
}

// DON'T: Deploy without testing credential failures
```

### Performance Best Practices

**1. Concurrent Access**

```go
// DO: Managers are thread-safe, use concurrently
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        result, _, err := manager.ExecuteWithFailover(ctx, operation)
        // Process result
    }()
}
wg.Wait()

// DO: Use connection pooling
client := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}

// DON'T: Create new manager per request (heavy initialization)
```

**2. Caching**

```go
// DO: Cache validated tokens
type TokenCache struct {
    tokens map[string]*CachedToken
    mu     sync.RWMutex
}

func (tc *TokenCache) GetToken(credID string) (string, bool) {
    tc.mu.RLock()
    defer tc.mu.RUnlock()

    cached, exists := tc.tokens[credID]
    if !exists || time.Now().After(cached.ExpiresAt) {
        return "", false
    }

    return cached.Token, true
}

// DON'T: Validate token on every request (adds latency)
```

**3. Resource Management**

```go
// DO: Cleanup expired tokens periodically
ticker := time.NewTicker(1 * time.Hour)
go func() {
    for range ticker.C {
        storage.CleanupExpired()
    }
}()

// DO: Limit concurrent refreshes
refreshSemaphore := make(chan struct{}, 5)  // Max 5 concurrent refreshes

func refreshWithLimit(cred *types.OAuthCredentialSet) error {
    refreshSemaphore <- struct{}{}        // Acquire
    defer func() { <-refreshSemaphore }() // Release

    return refreshToken(cred)
}

// DON'T: Allow unlimited concurrent operations
```

---

## Troubleshooting

### Common Issues

**1. "Token expired" errors**

```
Error: token expired at 2025-11-18T10:00:00Z
```

**Solution:**

```go
// Check refresh configuration
strategy := manager.GetRefreshStrategy()
if strategy.BufferTime < 5*time.Minute {
    // Increase buffer time
    strategy.BufferTime = 10 * time.Minute
    manager.SetRefreshStrategy(strategy)
}

// Manually trigger refresh
err := authenticator.RefreshToken(ctx)

// Verify refresh callback is working
OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
    log.Printf("Token refreshed for %s, expires at %v", id, expires)
    return saveTokens(id, access, refresh, expires)
}
```

**2. "All credentials unavailable" errors**

```
Error: all 3 OAuth credentials for Gemini are currently unavailable
```

**Solution:**

```go
// Check health status
summary := manager.GetHealthSummary()
fmt.Printf("Healthy: %d/%d\n",
    summary["healthy_credentials"],
    summary["total_credentials"])

// Check individual credential health
for _, credID := range credentialIDs {
    info := manager.GetCredentialHealthInfo(credID)
    fmt.Printf("%s: healthy=%v, backoffUntil=%v\n",
        credID, info.IsHealthy, info.BackoffUntil)
}

// Force reset health (if appropriate)
for _, credID := range credentialIDs {
    manager.ReportSuccess(credID)  // Clears failures
}
```

**3. "Invalid OAuth state" errors**

```
Error: OAuth state mismatch - potential CSRF attack
```

**Solution:**

```go
// Ensure state validation is configured correctly
oauthConfig := &auth.OAuthConfig{
    State: auth.StateConfig{
        EnableValidation: true,
        Length:           32,
        Expiration:       10 * time.Minute,
    },
}

// Verify state is stored and retrieved correctly
stateStore := make(map[string]StateEntry)

// When starting OAuth flow
state := generateState()
stateStore[state] = StateEntry{
    Value:     state,
    ExpiresAt: time.Now().Add(10 * time.Minute),
}

// When handling callback
entry, exists := stateStore[returnedState]
if !exists {
    return errors.New("invalid state")
}
delete(stateStore, returnedState)  // One-time use
```

**4. "Refresh failed" errors**

```
Error: failed to refresh OAuth token: network error
```

**Solution:**

```go
// Implement retry logic
var refreshErr error
for attempt := 0; attempt < 3; attempt++ {
    refreshErr = authenticator.RefreshToken(ctx)
    if refreshErr == nil {
        break
    }

    // Exponential backoff
    time.Sleep(time.Duration(1<<attempt) * time.Second)
}

// Check network connectivity
client := &http.Client{Timeout: 10 * time.Second}
resp, err := client.Get("https://oauth2.googleapis.com")
if err != nil {
    log.Printf("Network issue detected: %v", err)
}

// Verify refresh token is valid
tokenInfo, err := authenticator.GetTokenInfo()
if err == nil && tokenInfo.RefreshToken == "" {
    log.Printf("No refresh token available - need to re-authenticate")
}
```

**5. High failure rate alerts**

```
Alert: High failure rate detected: 35.0%
```

**Solution:**

```go
// Check API quota/rate limits
metrics := manager.GetCredentialMetrics(credID)
fmt.Printf("Requests/hour: %.2f\n", metrics.GetRequestsPerHour())

// Verify credentials are valid
for _, credID := range credentialIDs {
    token, err := getToken(credID)
    if err != nil {
        log.Printf("Invalid credential: %s - %v", credID, err)
    }
}

// Implement rate limiting
rateLimiter := rate.NewLimiter(rate.Limit(10), 1)  // 10 req/sec
rateLimiter.Wait(ctx)

// Add more credentials for load distribution
newCred := createNewCredential()
manager.AddCredential(newCred)
```

**6. Encryption/decryption errors**

```
Error: failed to decrypt data: message authentication failed
```

**Solution:**

```go
// Verify encryption key is correct
key := []byte("your-32-byte-encryption-key!")
if len(key) != 32 {
    log.Fatal("Encryption key must be 32 bytes for AES-256")
}

// Check for key rotation issues
// If you rotated the key, old encrypted data can't be decrypted
// Solution: Decrypt with old key, re-encrypt with new key

oldKey := loadOldKey()
newKey := loadNewKey()

// Migrate encrypted data
data, err := decryptWithKey(encryptedData, oldKey)
if err != nil {
    log.Fatal("Failed to decrypt with old key:", err)
}
newEncryptedData, err := encryptWithKey(data, newKey)
if err != nil {
    log.Fatal("Failed to encrypt with new key:", err)
}

// Verify file permissions
stat, err := os.Stat("tokens/gemini.json")
if err == nil {
    mode := stat.Mode()
    if mode.Perm() != 0600 {
        log.Printf("Warning: Insecure file permissions: %v", mode.Perm())
        os.Chmod("tokens/gemini.json", 0600)
    }
}
```

### Debug Mode

Enable debug logging for troubleshooting:

```go
import "log"

// Enable verbose logging
logger := log.New(os.Stdout, "[AUTH] ", log.LstdFlags|log.Lshortfile)

// Log all authentication attempts
auditor := auth.NewSecurityAuditor(logger, &auth.SecurityConfig{
    AuditLogging: true,
})

// Log OAuth flow steps
authenticator := auth.NewOAuthAuthenticator("gemini", storage, oauthConfig)
authURL, err := authenticator.StartOAuthFlow(ctx, scopes)
logger.Printf("OAuth URL: %s", authURL)

// Log token refresh
err = authenticator.RefreshToken(ctx)
logger.Printf("Token refresh result: %v", err)

// Log health status changes
manager.ReportFailure(credID, err)
logger.Printf("Credential %s marked unhealthy", credID)
```

### Getting Help

- **Documentation**: See `/docs` directory for detailed guides
- **Examples**: Check `/examples` for working code samples
- **Issues**: Report bugs at github.com/cecil-the-coder/ai-provider-kit
- **Community**: Join discussions in GitHub Discussions

---

## Appendix

### Supported OAuth Flows

| Flow | Providers | Use Case |
|------|-----------|----------|
| Device Code (RFC 8628) | Gemini, Qwen | CLI tools, headless servers |
| Authorization Code | Custom | Web applications |
| Client Credentials | Custom | Service-to-service |

### Encryption Algorithms

| Algorithm | Key Size | Mode | Authentication |
|-----------|----------|------|----------------|
| AES-256-GCM | 256 bits | GCM | Built-in |
| AES-192-GCM | 192 bits | GCM | Built-in |
| AES-128-GCM | 128 bits | GCM | Built-in |

**Recommended**: AES-256-GCM for maximum security.

### Environment Variables Reference

```bash
# API Keys
export OPENAI_API_KEY="sk-proj-..."
export ANTHROPIC_API_KEY="sk-ant-api03-..."
export CEREBRAS_API_KEY="csk-..."
export OPENROUTER_API_KEY="sk-or-v1-..."

# OAuth Credentials (Gemini)
export GEMINI_CLIENT_ID="681255809395-....apps.googleusercontent.com"
export GEMINI_CLIENT_SECRET="GOCSPX-..."
export GEMINI_PROJECT_ID="your-project-id"

# OAuth Credentials (Qwen)
export QWEN_CLIENT_ID="qwen-client-id"
export QWEN_CLIENT_SECRET="qwen-client-secret"

# Encryption
export AUTH_ENCRYPTION_KEY="your-32-byte-encryption-key!"

# Storage
export AUTH_TOKEN_DIRECTORY="./tokens"
export AUTH_CONFIG_FILE="~/.config/ai-provider-kit/config.yaml"
```

### Security Checklist

Before deploying to production:

- [ ] API keys stored in environment variables or secret manager
- [ ] Token encryption enabled (AES-256-GCM)
- [ ] File permissions set to 0600 (tokens) and 0700 (directories)
- [ ] PKCE enabled for OAuth flows
- [ ] State validation enabled for OAuth
- [ ] Audit logging enabled
- [ ] Token masking enabled in logs
- [ ] Multi-credential setup for critical services
- [ ] Health monitoring configured
- [ ] Alert webhooks configured
- [ ] Key rotation policy defined
- [ ] Backup strategy in place
- [ ] HTTPS enforced for all API calls
- [ ] TLS 1.2+ required
- [ ] Regular security audits scheduled

---

**Document Version**: 1.0
**Last Updated**: 2025-11-18
**Maintained By**: AI Provider Kit Team
**License**: Same as ai-provider-kit project
