# Retry Infrastructure

The retry package provides comprehensive retry logic for AI provider implementations, including retry policies, backoff strategies, and error classification.

## Features

- **Retry Policies**: Configurable policies with max retries, delays, multipliers, and jitter
- **Backoff Strategies**: Exponential, linear, and constant backoff with multiple jitter types
- **Error Classification**: Retryable error detection based on HTTP status codes and custom criteria
- **Context Support**: Graceful cancellation and timeout handling
- **Retry-After Header**: Automatic parsing and respect for server-side rate limit headers

## Quick Start

### Basic Usage

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/retry"

// Create a default retry executor
executor := retry.NewDefaultRetryExecutor()

// Execute an operation with retry logic
err := executor.Execute(ctx, func() error {
    // Your operation that might fail
    resp, err := makeAPICall()
    if err != nil {
        // Mark errors as retryable based on status code
        if isTransientError(err) {
            return retry.MarkRetryable(err, statusCode)
        }
        return err
    }
    return nil
})
```

### Custom Retry Policy

```go
// Create a custom retry policy
policy := &retry.RetryPolicy{
    MaxRetries:   5,
    InitialDelay: 1 * time.Second,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
    Jitter:       0.1,
}

strategy := retry.NewExponentialBackoffStrategy(policy)
executor := retry.NewRetryExecutor(policy, strategy)
```

### Using Preset Policies

```go
// Aggressive retry (more attempts, shorter delays)
executor := retry.NewRetryExecutor(
    retry.AggressiveRetryPolicy(),
    retry.NewExponentialBackoffStrategy(retry.AggressiveRetryPolicy()),
)

// Conservative retry (fewer attempts, longer delays)
executor := retry.NewRetryExecutor(
    retry.ConservativeRetryPolicy(),
    retry.NewExponentialBackoffStrategy(retry.ConservativeRetryPolicy()),
)

// No retry
executor := retry.NewRetryExecutor(
    retry.NoRetryPolicy(),
    retry.NewConstantBackoffStrategy(0),
)
```

### Typed Operations

```go
// Execute with strongly-typed results
result, err := retry.ExecuteTyped(ctx, executor, func() (*APIResponse, error) {
    return makeAPICall()
})
```

### Backoff Strategies

```go
// Exponential backoff with full jitter
strategy := retry.NewExponentialBackoffStrategy(policy).
    WithJitterType(retry.FullJitter)

// Constant backoff (same delay every time)
strategy := retry.NewConstantBackoffStrategy(5 * time.Second)

// Linear backoff (gradual increase)
strategy := retry.NewLinearBackoffStrategy(
    1*time.Second,  // initial delay
    2*time.Second,  // increment per retry
    30*time.Second, // max delay
)
```

### Jitter Types

The package supports multiple jitter strategies to prevent thundering herd:

- **NoJitter**: No randomization
- **FullJitter**: Random delay between 0 and calculated delay
- **EqualJitter**: Half fixed, half random
- **DecorrelatedJitter**: AWS-style decorrelated jitter

```go
strategy := retry.NewExponentialBackoffStrategy(policy).
    WithJitterType(retry.DecorrelatedJitter)
```

### Error Classification

```go
// Check if an error is retryable
if retry.IsRetryableError(err) {
    // Handle retryable error
}

// Check if a status code is retryable
if retry.IsRetryableStatusCode(statusCode) {
    // Status code indicates a transient error
}

// Create retryable errors
err := retry.MarkRetryable(baseErr, 503)
err := retry.MarkNonRetryable(baseErr, 400)
```

### Retry Callbacks

```go
// Execute with callbacks for monitoring
err := executor.ExecuteWithCallback(ctx, operation, func(attempt int, err error, delay time.Duration) {
    log.Printf("Retry attempt %d failed: %v, waiting %v", attempt, err, delay)
})
```

### Custom Retryable Status Codes

```go
policy := &retry.RetryPolicy{
    MaxRetries:   3,
    InitialDelay: 1 * time.Second,
    RetryableStatusCodes: map[int]bool{
        408: true, // Request Timeout
        425: true, // Too Early
        429: true, // Too Many Requests
        500: true, // Internal Server Error
        502: true, // Bad Gateway
        503: true, // Service Unavailable
        504: true, // Gateway Timeout
    },
}
```

## Default Retryable Status Codes

The following HTTP status codes are considered retryable by default:

- 429 (Too Many Requests)
- 500 (Internal Server Error)
- 502 (Bad Gateway)
- 503 (Service Unavailable)
- 504 (Gateway Timeout)
- 507 (Insufficient Storage)
- 511 (Network Authentication Required)

## Retry-After Header Support

The package automatically parses and respects the `Retry-After` header:

```go
// The executor will automatically use Retry-After if present in error
// No additional code needed - it's handled internally
```

## Policy Builder Pattern

```go
policy := retry.DefaultRetryPolicy().
    WithMaxRetries(5).
    WithInitialDelay(500 * time.Millisecond).
    WithMaxDelay(20 * time.Second).
    WithMultiplier(1.5).
    WithJitter(0.2)
```

## Context Cancellation

All retry operations respect context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// Will stop retrying if context times out
err := executor.Execute(ctx, operation)
```

## Best Practices

1. **Use Default Policy**: Start with `DefaultRetryPolicy()` and customize as needed
2. **Add Jitter**: Always use some jitter (0.1-0.2) to prevent thundering herd
3. **Set Max Delay**: Cap maximum delay to prevent excessive wait times
4. **Respect Retry-After**: The executor handles this automatically
5. **Context Timeouts**: Always use context with timeouts for retry operations
6. **Log Retries**: Use callbacks for monitoring and debugging
7. **Mark Errors Properly**: Use `MarkRetryable`/`MarkNonRetryable` for clear intent

## Thread Safety

All retry executors and strategies are safe for concurrent use. Each execution creates its own state.

## Examples

See the test files for comprehensive examples of all features:
- `errors_test.go` - Error classification examples
- `policy_test.go` - Policy configuration examples
- `backoff_test.go` - Backoff strategy examples
- `strategy_test.go` - Executor usage examples
