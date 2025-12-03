# Error Code Mapping Guide

Complete reference for error codes in the AI Provider Kit, including their meanings, HTTP status mappings, and handling strategies.

## Table of Contents

- [Overview](#overview)
- [Error Code Reference](#error-code-reference)
- [HTTP Status Code Mappings](#http-status-code-mappings)
- [Error Handling Strategies](#error-handling-strategies)
- [Error Types](#error-types)
- [Code Examples](#code-examples)
- [Best Practices](#best-practices)

## Overview

The AI Provider Kit uses a standardized error system across all providers. There are two primary error types:

1. **ProviderError** (`pkg/types/provider_error.go`) - High-level errors from providers with rich context
2. **APIError** (`pkg/providers/common/errors.go`) - Lower-level HTTP API errors

Both error types provide:
- Standardized error codes
- Retryability detection
- HTTP status code mapping
- Provider context
- Original error wrapping (for debugging)

## Error Code Reference

### Core Error Codes

All error codes are defined in `pkg/types/provider_error.go`:

| Error Code | Constant | Description | Retryable | Typical HTTP Status |
|------------|----------|-------------|-----------|---------------------|
| `unknown` | `ErrCodeUnknown` | Unclassified error | No | Any |
| `authentication` | `ErrCodeAuthentication` | Authentication/authorization failure | No | 401, 403 |
| `rate_limit` | `ErrCodeRateLimit` | Rate limit exceeded | Yes | 429 |
| `invalid_request` | `ErrCodeInvalidRequest` | Malformed request or invalid parameters | No | 400 |
| `not_found` | `ErrCodeNotFound` | Resource not found (model, endpoint, etc.) | No | 404 |
| `server_error` | `ErrCodeServerError` | Provider server error | Yes | 500-599 |
| `timeout` | `ErrCodeTimeout` | Request timeout | Yes | 408, 504 |
| `network` | `ErrCodeNetwork` | Network connectivity issue | Yes | N/A |
| `context_length` | `ErrCodeContextLength` | Input exceeds model's context window | No | 400 |
| `content_filter` | `ErrCodeContentFilter` | Content blocked by safety filters | No | 400 |

### Convenience Aliases

These aliases map error codes to test status values for easier health check implementations:

| Alias | Maps To | Use Case |
|-------|---------|----------|
| `ErrCodeAuthFailed` | `ErrCodeAuthentication` | Health check authentication failures |
| `ErrCodeConnectivityFailed` | `ErrCodeNetwork` | Health check connectivity failures |
| `ErrCodeTimeoutFailed` | `ErrCodeTimeout` | Health check timeout failures |

## HTTP Status Code Mappings

### Classification Function

The `ClassifyHTTPError(statusCode int) ErrorCode` function maps HTTP status codes to error codes:

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

errorCode := types.ClassifyHTTPError(401)
// Returns: ErrCodeAuthentication
```

### Mapping Table

| HTTP Status | Code | Error Code | Description |
|-------------|------|------------|-------------|
| 400 | Bad Request | `invalid_request` | Malformed request |
| 401 | Unauthorized | `authentication` | Invalid credentials |
| 403 | Forbidden | `authentication` | Insufficient permissions |
| 404 | Not Found | `not_found` | Resource not found |
| 408 | Request Timeout | `timeout` | Request took too long |
| 429 | Too Many Requests | `rate_limit` | Rate limit exceeded |
| 500 | Internal Server Error | `server_error` | Provider internal error |
| 502 | Bad Gateway | `server_error` | Gateway error |
| 503 | Service Unavailable | `server_error` | Service temporarily unavailable |
| 504 | Gateway Timeout | `server_error` | Gateway timeout |
| Other 5xx | Various | `server_error` | Other server errors |
| All Others | Various | `unknown` | Unclassified |

## Error Handling Strategies

### Retryable vs Non-Retryable Errors

Use the `IsRetryable()` method to determine if an error should be retried:

```go
err := doRequest()
if providerErr, ok := err.(*types.ProviderError); ok {
    if providerErr.IsRetryable() {
        // Implement retry logic with backoff
        time.Sleep(time.Duration(providerErr.RetryAfter) * time.Second)
        return retry()
    } else {
        // Don't retry - handle or return error
        return handlePermanentError(providerErr)
    }
}
```

#### Retryable Error Codes

These errors are typically transient and may succeed on retry:

- `rate_limit` - Wait for `RetryAfter` seconds before retrying
- `server_error` - Use exponential backoff
- `timeout` - Use exponential backoff
- `network` - Use exponential backoff

#### Non-Retryable Error Codes

These errors indicate permanent failures that won't be fixed by retrying:

- `authentication` - Fix credentials
- `invalid_request` - Fix request parameters
- `not_found` - Use correct resource identifier
- `context_length` - Reduce input size
- `content_filter` - Modify content
- `unknown` - Investigate and handle specifically

### Rate Limit Handling

Rate limit errors include a `RetryAfter` field indicating how long to wait:

```go
if providerErr.Code == types.ErrCodeRateLimit {
    waitSeconds := providerErr.RetryAfter
    if waitSeconds == 0 {
        waitSeconds = 60 // Default wait time
    }
    log.Printf("Rate limited, waiting %d seconds", waitSeconds)
    time.Sleep(time.Duration(waitSeconds) * time.Second)
    // Retry the request
}
```

### Exponential Backoff

For retryable errors without specific retry timing:

```go
func retryWithBackoff(fn func() error, maxRetries int) error {
    backoff := 1 * time.Second
    maxBackoff := 60 * time.Second

    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }

        providerErr, ok := err.(*types.ProviderError)
        if !ok || !providerErr.IsRetryable() {
            return err // Don't retry non-retryable errors
        }

        if i < maxRetries-1 {
            time.Sleep(backoff)
            backoff *= 2
            if backoff > maxBackoff {
                backoff = maxBackoff
            }
        }
    }
    return fmt.Errorf("max retries exceeded")
}
```

## Error Types

### ProviderError

The main error type with rich context:

```go
type ProviderError struct {
    Code        ErrorCode    // Categorized error code
    Message     string       // Human-readable message
    StatusCode  int          // HTTP status code (0 if not applicable)
    Provider    ProviderType // Which provider generated this error
    Operation   string       // What operation failed
    OriginalErr error        // Wrapped original error
    RetryAfter  int          // Seconds to wait before retry (for rate limits)
    RequestID   string       // Provider's request ID if available
}
```

#### Constructor Functions

Convenience functions for creating specific error types:

| Function | Error Code | Use Case |
|----------|------------|----------|
| `NewProviderError(provider, code, message)` | Any | Generic error creation |
| `NewAuthError(provider, message)` | `authentication` | Authentication failures |
| `NewRateLimitError(provider, retryAfter)` | `rate_limit` | Rate limiting |
| `NewServerError(provider, statusCode, message)` | `server_error` | Server errors |
| `NewInvalidRequestError(provider, message)` | `invalid_request` | Bad requests |
| `NewNetworkError(provider, message)` | `network` | Network issues |
| `NewTimeoutError(provider, message)` | `timeout` | Timeouts |
| `NewContextLengthError(provider, message)` | `context_length` | Context too long |
| `NewContentFilterError(provider, message)` | `content_filter` | Content blocked |
| `NewNotFoundError(provider, message)` | `not_found` | Not found |

#### Chainable Methods

Build rich error context with method chaining:

```go
err := types.NewAuthError(types.ProviderTypeAnthropic, "invalid API key").
    WithOperation("chat_completion").
    WithStatusCode(401).
    WithRequestID("req-abc123").
    WithOriginalErr(originalError)
```

Available methods:
- `WithOperation(operation string)` - Set the operation that failed
- `WithStatusCode(statusCode int)` - Set HTTP status code
- `WithOriginalErr(err error)` - Wrap original error
- `WithRequestID(requestID string)` - Set provider request ID
- `WithRetryAfter(retryAfter int)` - Set retry delay in seconds

### APIError (Common Package)

Lower-level API error type used internally:

```go
type APIError struct {
    StatusCode int          // HTTP status code
    Type       APIErrorType // Error type classification
    Message    string       // Error message
    RawBody    string       // Raw response body
    Retryable  bool         // Whether error is retryable
}
```

#### API Error Types

| Type | Constant | Maps To ProviderError Code |
|------|----------|----------------------------|
| `rate_limit` | `APIErrorTypeRateLimit` | `ErrCodeRateLimit` |
| `auth` | `APIErrorTypeAuth` | `ErrCodeAuthentication` |
| `not_found` | `APIErrorTypeNotFound` | `ErrCodeNotFound` |
| `invalid_request` | `APIErrorTypeInvalidRequest` | `ErrCodeInvalidRequest` |
| `server_error` | `APIErrorTypeServer` | `ErrCodeServerError` |
| `unknown` | `APIErrorTypeUnknown` | `ErrCodeUnknown` |

### Embedded Errors

Some providers return errors embedded in successful HTTP responses. Use `utils.CheckEmbeddedErrors` to detect these:

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

// Check for common error patterns in response body
if embeddedErr := utils.CheckCommonErrors(responseBody); embeddedErr != nil {
    // Handle embedded error
    log.Printf("Embedded error: %s (context: %s)",
        embeddedErr.Pattern, embeddedErr.Context)
}
```

Common embedded error patterns:
- `"token quota is not enough"`
- `"rate limit exceeded"`
- `"context length exceeded"`
- `"insufficient_quota"`
- `"model_not_found"`
- `"invalid_api_key"`
- `"quota exceeded"`
- `"capacity exceeded"`
- `"overloaded"`

## Code Examples

### Basic Error Handling

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    provider := anthropic.NewAnthropicProvider(types.ProviderConfig{
        APIKey: "your-api-key",
    })

    stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: "Hello!"},
        },
    })

    if err != nil {
        handleError(err)
        return
    }

    // Process stream...
}

func handleError(err error) {
    // Type assert to ProviderError
    providerErr, ok := err.(*types.ProviderError)
    if !ok {
        log.Printf("Unknown error: %v", err)
        return
    }

    // Handle based on error code
    switch providerErr.Code {
    case types.ErrCodeAuthentication:
        log.Printf("Authentication failed: %s", providerErr.Message)
        // Fix credentials and retry

    case types.ErrCodeRateLimit:
        log.Printf("Rate limited, retry after %d seconds", providerErr.RetryAfter)
        // Wait and retry

    case types.ErrCodeInvalidRequest:
        log.Printf("Invalid request: %s", providerErr.Message)
        // Fix request parameters

    case types.ErrCodeContextLength:
        log.Printf("Context too long: %s", providerErr.Message)
        // Reduce input size

    case types.ErrCodeServerError:
        if providerErr.IsRetryable() {
            log.Printf("Server error, will retry: %s", providerErr.Message)
            // Implement retry with backoff
        }

    default:
        log.Printf("Error [%s]: %s", providerErr.Code, providerErr.Message)
    }
}
```

### Advanced Error Handling with Retry

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func generateWithRetry(ctx context.Context, provider types.Provider, opts types.GenerateOptions) (*types.GenerateStream, error) {
    maxRetries := 3
    baseDelay := 1 * time.Second
    maxDelay := 60 * time.Second

    var lastErr error

    for attempt := 0; attempt < maxRetries; attempt++ {
        stream, err := provider.GenerateChatCompletion(ctx, opts)
        if err == nil {
            return stream, nil
        }

        lastErr = err

        // Type assert to ProviderError
        var providerErr *types.ProviderError
        if !errors.As(err, &providerErr) {
            // Not a ProviderError, don't retry
            return nil, err
        }

        // Check if error is retryable
        if !providerErr.IsRetryable() {
            return nil, err
        }

        // Calculate delay
        var delay time.Duration
        if providerErr.Code == types.ErrCodeRateLimit && providerErr.RetryAfter > 0 {
            // Use provider's retry-after header
            delay = time.Duration(providerErr.RetryAfter) * time.Second
        } else {
            // Use exponential backoff
            delay = baseDelay * time.Duration(1<<attempt)
            if delay > maxDelay {
                delay = maxDelay
            }
        }

        // Don't wait on last attempt
        if attempt < maxRetries-1 {
            fmt.Printf("Attempt %d failed with %s, retrying in %v...\n",
                attempt+1, providerErr.Code, delay)

            select {
            case <-time.After(delay):
                // Continue to retry
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

### Multi-Provider Fallback

```go
package main

import (
    "context"
    "errors"
    "fmt"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type ProviderWithFallback struct {
    primary   types.Provider
    fallbacks []types.Provider
}

func (p *ProviderWithFallback) Generate(ctx context.Context, opts types.GenerateOptions) (*types.GenerateStream, error) {
    // Try primary provider
    stream, err := p.primary.GenerateChatCompletion(ctx, opts)
    if err == nil {
        return stream, nil
    }

    // Check if we should try fallback
    var providerErr *types.ProviderError
    if errors.As(err, &providerErr) {
        // Don't fall back for certain error types
        switch providerErr.Code {
        case types.ErrCodeInvalidRequest,
             types.ErrCodeContextLength,
             types.ErrCodeContentFilter:
            // These errors won't be fixed by trying another provider
            return nil, err
        }
    }

    // Try fallback providers
    for i, fallback := range p.fallbacks {
        fmt.Printf("Primary provider failed, trying fallback %d...\n", i+1)
        stream, err = fallback.GenerateChatCompletion(ctx, opts)
        if err == nil {
            return stream, nil
        }
    }

    return nil, fmt.Errorf("all providers failed: %w", err)
}
```

### Error Logging and Monitoring

```go
package main

import (
    "context"
    "errors"
    "log"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type ErrorMetrics struct {
    authErrors        int
    rateLimitErrors   int
    serverErrors      int
    networkErrors     int
    otherErrors       int
}

func (m *ErrorMetrics) recordError(err error) {
    var providerErr *types.ProviderError
    if !errors.As(err, &providerErr) {
        m.otherErrors++
        return
    }

    switch providerErr.Code {
    case types.ErrCodeAuthentication:
        m.authErrors++
        log.Printf("[CRITICAL] Auth error for provider %s: %s",
            providerErr.Provider, providerErr.Message)

    case types.ErrCodeRateLimit:
        m.rateLimitErrors++
        log.Printf("[WARNING] Rate limit for provider %s, retry after %ds",
            providerErr.Provider, providerErr.RetryAfter)

    case types.ErrCodeServerError:
        m.serverErrors++
        log.Printf("[ERROR] Server error for provider %s (status %d): %s",
            providerErr.Provider, providerErr.StatusCode, providerErr.Message)

    case types.ErrCodeNetwork:
        m.networkErrors++
        log.Printf("[ERROR] Network error for provider %s: %s",
            providerErr.Provider, providerErr.Message)

    default:
        m.otherErrors++
        log.Printf("[ERROR] %s error for provider %s: %s",
            providerErr.Code, providerErr.Provider, providerErr.Message)
    }

    // Include request ID if available for debugging
    if providerErr.RequestID != "" {
        log.Printf("  Request ID: %s", providerErr.RequestID)
    }

    // Include original error for debugging
    if providerErr.OriginalErr != nil {
        log.Printf("  Original error: %v", providerErr.OriginalErr)
    }
}

func (m *ErrorMetrics) printSummary() {
    log.Printf("Error Summary:")
    log.Printf("  Authentication: %d", m.authErrors)
    log.Printf("  Rate Limit: %d", m.rateLimitErrors)
    log.Printf("  Server: %d", m.serverErrors)
    log.Printf("  Network: %d", m.networkErrors)
    log.Printf("  Other: %d", m.otherErrors)
}
```

## Best Practices

### 1. Always Check Error Codes

Don't just log errors - check the error code to determine appropriate handling:

```go
// Bad
if err != nil {
    log.Printf("Error: %v", err)
    return err
}

// Good
if err != nil {
    if providerErr, ok := err.(*types.ProviderError); ok {
        switch providerErr.Code {
        case types.ErrCodeRateLimit:
            // Handle rate limit
        case types.ErrCodeAuthentication:
            // Handle auth error
        default:
            // Handle other errors
        }
    }
    return err
}
```

### 2. Use IsRetryable() for Retry Logic

Let the error tell you if it's retryable:

```go
// Bad
if err != nil && (statusCode == 429 || statusCode >= 500) {
    retry()
}

// Good
if providerErr, ok := err.(*types.ProviderError); ok && providerErr.IsRetryable() {
    retry()
}
```

### 3. Respect RetryAfter for Rate Limits

Don't ignore the provider's retry timing:

```go
// Bad
if providerErr.Code == types.ErrCodeRateLimit {
    time.Sleep(5 * time.Second) // Arbitrary delay
}

// Good
if providerErr.Code == types.ErrCodeRateLimit {
    delay := time.Duration(providerErr.RetryAfter) * time.Second
    if delay == 0 {
        delay = 60 * time.Second // Reasonable default
    }
    time.Sleep(delay)
}
```

### 4. Use Error Wrapping

Preserve the error chain for debugging:

```go
// Bad
if err != nil {
    return fmt.Errorf("operation failed")
}

// Good
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

### 5. Log Contextual Information

Include provider, operation, and request ID in logs:

```go
// Bad
log.Printf("Error: %v", err)

// Good
if providerErr, ok := err.(*types.ProviderError); ok {
    log.Printf("[%s] %s failed: %s (code=%s, requestID=%s)",
        providerErr.Provider,
        providerErr.Operation,
        providerErr.Message,
        providerErr.Code,
        providerErr.RequestID)
}
```

### 6. Don't Retry Non-Retryable Errors

Avoid wasting time and resources:

```go
// Bad
for retries := 0; retries < 5; retries++ {
    err := doRequest()
    if err == nil {
        break
    }
    time.Sleep(time.Second)
}

// Good
for retries := 0; retries < 5; retries++ {
    err := doRequest()
    if err == nil {
        break
    }

    providerErr, ok := err.(*types.ProviderError)
    if !ok || !providerErr.IsRetryable() {
        return err // Don't retry
    }

    time.Sleep(time.Second)
}
```

### 7. Handle Context Length Errors Proactively

Implement token counting and truncation:

```go
func truncateToContextLimit(messages []types.ChatMessage, maxTokens int) []types.ChatMessage {
    // Implement token counting and truncation
    // Return truncated messages
}

opts := types.GenerateOptions{
    Messages: truncateToContextLimit(messages, 4096),
}

stream, err := provider.GenerateChatCompletion(ctx, opts)
if err != nil {
    if providerErr, ok := err.(*types.ProviderError); ok {
        if providerErr.Code == types.ErrCodeContextLength {
            // Even after truncation, still too long
            // Further reduce or split the request
        }
    }
}
```

### 8. Monitor Error Patterns

Track error rates to detect issues early:

```go
type ErrorRateMonitor struct {
    window      time.Duration
    threshold   float64
    errors      []time.Time
}

func (m *ErrorRateMonitor) recordError() {
    m.errors = append(m.errors, time.Now())

    // Keep only errors within window
    cutoff := time.Now().Add(-m.window)
    i := 0
    for i < len(m.errors) && m.errors[i].Before(cutoff) {
        i++
    }
    m.errors = m.errors[i:]
}

func (m *ErrorRateMonitor) isAlerting() bool {
    rate := float64(len(m.errors)) / m.window.Seconds()
    return rate > m.threshold
}
```

## Related Documentation

- [SDK API Reference](SDK_API_REFERENCE.md) - Complete API documentation
- [SDK Best Practices](SDK_BEST_PRACTICES.md) - General best practices
- [SDK Troubleshooting FAQ](SDK_TROUBLESHOOTING_FAQ.md) - Common issues and solutions
- [OAuth Manager](OAUTH_MANAGER.md) - OAuth credential management and error handling
- [Multi-Key Strategies](MULTI_KEY_STRATEGIES.md) - Credential failover strategies

## Summary

The AI Provider Kit's error system provides:

1. **Standardized error codes** across all providers
2. **Automatic retryability detection** via `IsRetryable()`
3. **Rich error context** with provider, operation, and request ID
4. **HTTP status code mapping** for consistent error classification
5. **Rate limit handling** with `RetryAfter` information
6. **Error wrapping** for debugging with `OriginalErr`
7. **Convenience constructors** for common error types
8. **Chainable methods** for building detailed error context

By following the patterns in this guide, you can build robust applications that gracefully handle errors from AI providers.
