# Provider Factory with Authentication-Aware Testing

This package provides a comprehensive `ProviderFactory` with a `TestProvider` method that performs authentication-aware testing across all provider types using the interfaces and error structures we've already built.

## Overview

The `ProviderFactory` orchestrates the testing logic across all provider types using the `OAuthProvider`, `TestableProvider`, and other interfaces. It provides comprehensive testing with detailed phases, error types, and diagnostic information.

## Key Features

### Authentication-Aware Testing
- **OAuth Providers**: Uses `OAuthProvider` methods for token validation, refresh, and OAuth-specific error handling
- **API Key Providers**: Validates configuration and uses `TestableProvider.TestConnectivity()` for connectivity testing
- **Virtual Providers**: Tests through their underlying providers
- **Graceful Fallbacks**: Handles providers that don't implement testing interfaces

### Comprehensive Test Phases
1. **Configuration Phase**: Provider creation and configuration validation
2. **Authentication Phase**: Token validation (OAuth) or API key validation
3. **Connectivity Phase**: Network connectivity testing
4. **Model Fetch Phase**: Model discovery (if supported)

### Detailed Error Handling
- Uses `TestResult` structures with detailed phases and error types
- Leverages OAuthProvider and TestableProvider interfaces
- Handles providers that don't implement testing interfaces gracefully
- Provides detailed error messages for debugging
- Distinguishes between auth errors, connectivity errors, configuration errors
- Includes timing information and retryable flags

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Create factory
    factory := base.NewProviderFactory()

    // Register providers
    factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
        return &OpenAIProvider{...} // Your provider implementation
    })

    // Test provider
    config := map[string]interface{}{
        "api_key": "sk-your-api-key",
        "base_url": "https://api.openai.com/v1",
    }

    result, err := factory.TestProvider(context.Background(), "openai", config)
    if err != nil {
        log.Fatal(err)
    }

    if result.IsSuccess() {
        log.Printf("✅ Provider test passed! (%v)", result.Duration)
        log.Printf("Models available: %d", result.ModelsCount)
    } else {
        log.Printf("❌ Provider test failed: %s", result.Error)
        log.Printf("Phase: %s, Retryable: %t", result.Phase, result.IsRetryable())
    }
}
```

### Testing OAuth Providers

```go
// Register OAuth provider
factory.RegisterProvider(types.ProviderTypeGemini, func(config types.ProviderConfig) types.Provider {
    return &GeminiProvider{...} // Implements OAuthProvider interface
})

// Test with OAuth configuration
oauthConfig := map[string]interface{}{
    "client_id":     "your-client-id",
    "client_secret": "your-client-secret",
    "auth_url":      "https://accounts.google.com/o/oauth2/auth",
    "token_url":     "https://oauth2.googleapis.com/token",
}

result, err := factory.TestProvider(context.Background(), "gemini", oauthConfig)
```

### Provider Name Variations

The factory supports various provider name variations:

```go
// All of these work:
factory.TestProvider(ctx, "openai", config)
factory.TestProvider(ctx, "gpt", config)       // Maps to OpenAI
factory.TestProvider(ctx, "anthropic", config)
factory.TestProvider(ctx, "claude", config)    // Maps to Anthropic
factory.TestProvider(ctx, "xai", config)       // Maps to xAI
factory.TestProvider(ctx, "x.ai", config)      // Maps to xAI
```

## Test Result Structure

The `TestResult` provides comprehensive information:

```go
type TestResult struct {
    Status      TestStatus        `json:"status"`        // success, auth_failed, connectivity_failed, etc.
    Error       string            `json:"error,omitempty"`
    Details     map[string]string `json:"details,omitempty"`
    ModelsCount int               `json:"models_count,omitempty"`
    Phase       TestPhase         `json:"phase"`         // authentication, connectivity, etc.
    Timestamp   time.Time         `json:"timestamp"`
    Duration    time.Duration     `json:"duration"`
    ProviderType ProviderType      `json:"provider_type"`
    TestError   *TestError        `json:"test_error,omitempty"`
}
```

## Error Types and Handling

The factory provides detailed error categorization:

- **Authentication Errors**: Invalid tokens, missing credentials
- **Token Errors**: Expired tokens, refresh failures
- **OAuth Errors**: OAuth-specific failures
- **Connectivity Errors**: Network failures, timeouts
- **Configuration Errors**: Invalid configuration, missing parameters
- **Server Errors**: HTTP 5xx errors from providers
- **Rate Limit Errors**: API rate limiting
- **Unknown Errors**: Unclassified errors

Each error includes:
- Retryable flag
- Phase of failure
- Detailed error message
- Status codes (when applicable)

## Interface Integration

The factory integrates with existing interfaces:

### OAuthProvider
```go
type OAuthProvider interface {
    Provider
    ValidateToken(ctx context.Context) (*TokenInfo, error)
    RefreshToken(ctx context.Context) error
    GetAuthURL(redirectURI, state string) string
}
```

### TestableProvider
```go
type TestableProvider interface {
    Provider
    TestConnectivity(ctx context.Context) error
}
```

## Context Support

The factory respects context cancellation and timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := factory.TestProvider(ctx, "openai", config)
```

## JSON Serialization

Test results can be serialized to/from JSON:

```go
// To JSON
jsonData, err := result.ToJSON()

// From JSON
parsedResult, err := types.TestResultFromJSON(jsonData)
```

## Implementation Details

### Test Phases
1. **Configuration**: Provider creation and validation
2. **Authentication**: OAuth token validation or API key check
3. **Connectivity**: Network connectivity testing
4. **Model Fetch**: Model discovery (optional)

### Error Detection
- HTTP status code extraction from error messages
- Error type classification based on patterns
- Retryable determination based on error type

### Fallback Testing
- Uses `HealthCheck()` method if `TestableProvider` not implemented
- Provides appropriate test results based on available interfaces

## Files

- `provider_factory.go` - Main implementation
- `provider_factory_test.go` - Test cases
- `example_usage.go` - Usage examples
- `README.md` - This documentation

## Dependencies

- `github.com/cecil-the-coder/ai-provider-kit/pkg/types` - Type definitions and interfaces

The implementation provides a robust, authentication-aware testing framework that works with all provider types while providing detailed diagnostic information and graceful error handling.