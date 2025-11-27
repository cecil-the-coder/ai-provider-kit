# AI Provider Kit - Metrics Documentation

## Overview

The AI Provider Kit includes comprehensive metrics tracking at two levels:
1. **Provider-Level Metrics** - Overall performance and usage per provider
2. **Credential-Level Metrics** - Detailed tracking per OAuth credential (for OAuth providers)

All metrics are automatically collected during API operations, providing complete visibility into request patterns, performance characteristics, and resource utilization.

## Architecture

### Two-Tier Metrics System

```
┌─────────────────────────────────────────┐
│         Provider-Level Metrics          │
│  (BaseProvider - all providers)         │
│  - Total requests across all creds      │
│  - Aggregate success/error rates        │
│  - Overall latency and token usage      │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│      Credential-Level Metrics           │
│  (OAuthKeyManager - OAuth providers)    │
│  - Per-credential request counts        │
│  - Individual performance metrics       │
│  - Token usage per account              │
│  - Refresh tracking                     │
└─────────────────────────────────────────┘
```

## Provider-Level Metrics

### Metrics Structure

All providers inherit from `BaseProvider` which tracks:

```go
type ProviderMetrics struct {
    // Request tracking
    RequestCount    int64
    SuccessCount    int64
    ErrorCount      int64

    // Performance metrics
    TotalLatency    time.Duration
    AverageLatency  time.Duration

    // Resource usage
    TokensUsed      int64

    // Timestamps
    LastRequestTime time.Time
    LastSuccessTime time.Time
    LastErrorTime   time.Time
    LastError       string

    // Health status
    HealthStatus    HealthStatus
}

type HealthStatus struct {
    Healthy      bool
    Message      string
    LastChecked  time.Time
    ResponseTime float64
}
```

### API Reference

#### Retrieving Metrics

```go
metrics := provider.GetMetrics()
```

Returns a thread-safe copy of current metrics.

#### Automatic Tracking

Metrics are automatically updated during operations:

```go
func (p *Provider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Track request start
    p.IncrementRequestCount()
    startTime := time.Now()

    // Make API call
    response, err := p.makeAPICall(ctx, options)
    if err != nil {
        p.RecordError(err)
        return nil, err
    }

    // Extract token usage
    var tokensUsed int64
    if response.Usage != nil {
        tokensUsed = int64(response.Usage.TotalTokens)
    }

    // Record success
    p.RecordSuccess(time.Since(startTime), tokensUsed)

    return stream, nil
}
```

### Usage Examples

#### Basic Metrics Retrieval

```go
import (
    "fmt"
    "context"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/openai"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Create provider
    config := types.ProviderConfig{
        Type:    types.ProviderTypeOpenAI,
        APIKey:  "your-api-key",
        BaseURL: "https://api.openai.com/v1",
    }
    provider := openai.NewOpenAIProvider(config)

    // Use the provider (metrics tracked automatically)
    ctx := context.Background()
    stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: "Hello!"},
        },
    })

    // Retrieve metrics
    metrics := provider.GetMetrics()
    fmt.Printf("Requests: %d\n", metrics.RequestCount)
    fmt.Printf("Successes: %d\n", metrics.SuccessCount)
    fmt.Printf("Failures: %d\n", metrics.ErrorCount)
    fmt.Printf("Tokens Used: %d\n", metrics.TokensUsed)

    if metrics.AverageLatency > 0 {
        fmt.Printf("Avg Latency: %v\n", metrics.AverageLatency)
    }
}
```

#### Monitoring Health Status

```go
// Perform health check
err := provider.HealthCheck(ctx)

// Get health metrics
metrics := provider.GetMetrics()
if metrics.HealthStatus.Healthy {
    fmt.Printf("Provider is healthy: %s\n", metrics.HealthStatus.Message)
    fmt.Printf("Response time: %.2fms\n", metrics.HealthStatus.ResponseTime)
} else {
    fmt.Printf("Provider unhealthy: %s\n", metrics.HealthStatus.Message)
}
```

#### Building a Dashboard

```go
func displayDashboard(providers map[string]types.Provider) {
    fmt.Println("Provider Performance Dashboard")
    fmt.Println("==============================")

    for name, provider := range providers {
        metrics := provider.GetMetrics()

        fmt.Printf("\n%s:\n", name)
        fmt.Printf("  Requests: %d (Success: %d, Failed: %d)\n",
            metrics.RequestCount, metrics.SuccessCount, metrics.ErrorCount)

        if metrics.SuccessCount > 0 {
            successRate := float64(metrics.SuccessCount) / float64(metrics.RequestCount) * 100
            fmt.Printf("  Success Rate: %.2f%%\n", successRate)
            fmt.Printf("  Avg Latency: %v\n", metrics.AverageLatency)
            fmt.Printf("  Tokens Used: %d\n", metrics.TokensUsed)

            if metrics.SuccessCount > 0 {
                avgTokensPerRequest := metrics.TokensUsed / metrics.SuccessCount
                fmt.Printf("  Avg Tokens/Request: %d\n", avgTokensPerRequest)
            }
        }

        if metrics.LastError != "" {
            fmt.Printf("  Last Error: %s (at %s)\n",
                metrics.LastError, metrics.LastErrorTime.Format("15:04:05"))
        }
    }
}
```

## Credential-Level Metrics

### Overview

OAuth providers (Gemini, Qwen) using the `OAuthKeyManager` track detailed metrics for each individual credential, enabling:
- Per-account usage tracking
- Individual credential performance analysis
- Cost attribution per credential
- Health monitoring per account

### Metrics Structure

```go
type CredentialMetrics struct {
    mu sync.RWMutex

    // Usage tracking
    RequestCount int64 // Total requests using this credential
    SuccessCount int64 // Successful requests
    ErrorCount   int64 // Failed requests

    // Token usage
    TokensUsed int64 // Cumulative tokens consumed

    // Performance
    TotalLatency   time.Duration // Cumulative latency
    AverageLatency time.Duration // Calculated average

    // Timing
    FirstUsed time.Time // When credential was first used
    LastUsed  time.Time // Most recent usage

    // Refresh tracking
    RefreshCount    int       // Number of token refreshes
    LastRefreshTime time.Time // Last successful refresh
}
```

### API Reference

#### Get Metrics for Specific Credential

```go
metrics := manager.GetCredentialMetrics("account-1")
if metrics != nil {
    fmt.Printf("Requests: %d\n", metrics.RequestCount)
    fmt.Printf("Success Rate: %.2f%%\n", metrics.GetSuccessRate()*100)
    fmt.Printf("Tokens Used: %d\n", metrics.TokensUsed)
    fmt.Printf("Avg Latency: %v\n", metrics.AverageLatency)
    fmt.Printf("Requests/Hour: %.2f\n", metrics.GetRequestsPerHour())
}
```

#### Get All Credential Metrics

```go
allMetrics := manager.GetAllMetrics()
for credID, metrics := range allMetrics {
    fmt.Printf("%s: %d requests, %.2f%% success\n",
        credID, metrics.RequestCount, metrics.GetSuccessRate()*100)
}
```

#### Manual Recording (Advanced)

```go
// Metrics are automatically recorded by ExecuteWithFailover
// Manual recording is rarely needed

manager.RecordRequest(
    "account-1",        // credential ID
    100,                // tokens used
    50*time.Millisecond, // latency
    true,               // success
)
```

### Helper Methods

#### GetSuccessRate()

```go
successRate := metrics.GetSuccessRate() // Returns 0.0 to 1.0
fmt.Printf("Success Rate: %.2f%%\n", successRate*100)
```

Returns the success rate as a fraction (0.0 = 0%, 1.0 = 100%).

#### GetRequestsPerHour()

```go
reqPerHour := metrics.GetRequestsPerHour()
fmt.Printf("Requests per hour: %.2f\n", reqPerHour)
```

Calculates average requests per hour based on usage history since first use.

### Usage Examples

#### Monitoring Individual Credentials

```go
func monitorCredentials(manager *oauthmanager.OAuthKeyManager) {
    allMetrics := manager.GetAllMetrics()

    fmt.Println("Credential Performance Report")
    fmt.Println("=============================")

    for credID, metrics := range allMetrics {
        fmt.Printf("\nCredential: %s\n", credID)
        fmt.Printf("  Requests: %d\n", metrics.RequestCount)
        fmt.Printf("  Success Rate: %.2f%%\n", metrics.GetSuccessRate()*100)
        fmt.Printf("  Tokens Used: %d\n", metrics.TokensUsed)
        fmt.Printf("  Avg Latency: %v\n", metrics.AverageLatency)
        fmt.Printf("  Refresh Count: %d\n", metrics.RefreshCount)

        if !metrics.FirstUsed.IsZero() {
            fmt.Printf("  Active Since: %s\n", metrics.FirstUsed.Format("2006-01-02 15:04"))
            fmt.Printf("  Requests/Hour: %.2f\n", metrics.GetRequestsPerHour())
        }

        if !metrics.LastRefreshTime.IsZero() {
            fmt.Printf("  Last Refreshed: %s\n", metrics.LastRefreshTime.Format("15:04:05"))
        }
    }
}
```

#### Cost Tracking Per Credential

```go
func calculateCostPerCredential(manager *oauthmanager.OAuthKeyManager, costPer1kTokens float64) map[string]float64 {
    costs := make(map[string]float64)
    allMetrics := manager.GetAllMetrics()

    for credID, metrics := range allMetrics {
        cost := float64(metrics.TokensUsed) / 1000.0 * costPer1kTokens
        costs[credID] = cost
    }

    return costs
}

func main() {
    // Calculate costs at $0.01 per 1k tokens
    costs := calculateCostPerCredential(manager, 0.01)

    totalCost := 0.0
    for credID, cost := range costs {
        fmt.Printf("%s: $%.4f\n", credID, cost)
        totalCost += cost
    }
    fmt.Printf("Total Cost: $%.4f\n", totalCost)
}
```

#### Load Distribution Analysis

```go
func analyzeLoadDistribution(manager *oauthmanager.OAuthKeyManager) {
    allMetrics := manager.GetAllMetrics()

    totalRequests := int64(0)
    for _, metrics := range allMetrics {
        totalRequests += metrics.RequestCount
    }

    fmt.Println("Load Distribution Report")
    fmt.Println("========================")
    fmt.Printf("Total Requests: %d\n\n", totalRequests)

    for credID, metrics := range allMetrics {
        percentage := float64(metrics.RequestCount) / float64(totalRequests) * 100
        fmt.Printf("%s: %d requests (%.1f%%)\n", credID, metrics.RequestCount, percentage)
    }
}
```

#### Performance Comparison

```go
func compareCredentialPerformance(manager *oauthmanager.OAuthKeyManager) {
    allMetrics := manager.GetAllMetrics()

    type perfData struct {
        credID     string
        latency    time.Duration
        successRate float64
    }

    var perf []perfData
    for credID, metrics := range allMetrics {
        perf = append(perf, perfData{
            credID:     credID,
            latency:    metrics.AverageLatency,
            successRate: metrics.GetSuccessRate(),
        })
    }

    // Sort by latency (fastest first)
    sort.Slice(perf, func(i, j int) bool {
        return perf[i].latency < perf[j].latency
    })

    fmt.Println("Credential Performance Ranking")
    fmt.Println("==============================")
    for i, p := range perf {
        fmt.Printf("%d. %s - %v latency, %.1f%% success\n",
            i+1, p.credID, p.latency, p.successRate*100)
    }
}
```

## Metrics Export

### Prometheus Format

OAuth managers can export metrics in Prometheus format:

```go
promMetrics := manager.ExportPrometheus()

fmt.Println("# HELP oauth_requests_total Total requests per credential")
fmt.Println("# TYPE oauth_requests_total counter")
for credID, count := range promMetrics.RequestsTotal {
    fmt.Printf("oauth_requests_total{credential=\"%s\"} %d\n", credID, count)
}

fmt.Println("# HELP oauth_success_total Successful requests per credential")
fmt.Println("# TYPE oauth_success_total counter")
for credID, count := range promMetrics.SuccessTotal {
    fmt.Printf("oauth_success_total{credential=\"%s\"} %d\n", credID, count)
}

fmt.Println("# HELP oauth_tokens_total Tokens used per credential")
fmt.Println("# TYPE oauth_tokens_total counter")
for credID, count := range promMetrics.TokensUsedTotal {
    fmt.Printf("oauth_tokens_total{credential=\"%s\"} %d\n", credID, count)
}
```

**PrometheusMetrics Structure**:
```go
type PrometheusMetrics struct {
    RequestsTotal   map[string]int64 // credentialID -> request count
    SuccessTotal    map[string]int64 // credentialID -> success count
    ErrorsTotal     map[string]int64 // credentialID -> error count
    TokensUsedTotal map[string]int64 // credentialID -> tokens used
    RefreshesTotal  map[string]int   // credentialID -> refresh count
}
```

### JSON Export

```go
jsonData, err := manager.ExportJSON()
if err != nil {
    log.Fatal(err)
}

// Pretty print
var prettyJSON bytes.Buffer
json.Indent(&prettyJSON, jsonData, "", "  ")
fmt.Println(prettyJSON.String())
```

**JSON Format**:
```json
{
  "account-1": {
    "request_count": 1250,
    "success_count": 1230,
    "error_count": 20,
    "tokens_used": 125000,
    "average_latency_ms": 145.5,
    "success_rate": 0.984,
    "requests_per_hour": 83.3,
    "refresh_count": 3,
    "first_used": "2025-01-15T10:00:00Z",
    "last_used": "2025-01-15T25:00:00Z",
    "last_refresh": "2025-01-15T18:30:00Z"
  },
  "account-2": {
    "request_count": 980,
    "success_count": 975,
    "error_count": 5,
    "tokens_used": 98000,
    "average_latency_ms": 132.8,
    "success_rate": 0.995,
    "requests_per_hour": 65.3,
    "refresh_count": 2,
    "first_used": "2025-01-15T10:00:00Z",
    "last_used": "2025-01-15T25:00:00Z",
    "last_refresh": "2025-01-15T19:15:00Z"
  }
}
```

### Health Summary

```go
summary := manager.GetHealthSummary()

fmt.Printf("Total Credentials: %d\n", summary["total_credentials"])
fmt.Printf("Healthy Credentials: %d\n", summary["healthy_credentials"])
fmt.Printf("Total Requests: %d\n", summary["total_requests"])
fmt.Printf("Success Rate: %.2f%%\n", summary["success_rate"].(float64)*100)
fmt.Printf("Total Tokens: %d\n", summary["total_tokens"])
```

### Detailed Credential Info

```go
info := manager.GetCredentialHealthInfo("account-1")
if info != nil {
    fmt.Printf("ID: %s\n", info.ID)
    fmt.Printf("Healthy: %v\n", info.IsHealthy)
    fmt.Printf("Available: %v\n", info.IsAvailable)
    fmt.Printf("In Backoff: %v\n", info.InBackoff)
    fmt.Printf("Requests: %d\n", info.RequestCount)
    fmt.Printf("Success Rate: %.2f%%\n", info.SuccessRate*100)
    fmt.Printf("Tokens Used: %d\n", info.TokensUsed)
    fmt.Printf("Avg Latency: %v\n", info.AverageLatency)
    fmt.Printf("Last Used: %s\n", info.LastUsed.Format("15:04:05"))
}
```

## Best Practices

### Provider-Level Metrics

#### Regular Health Checks

```go
// Set up periodic health checks
ticker := time.NewTicker(5 * time.Minute)
go func() {
    for range ticker.C {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        provider.HealthCheck(ctx)
        cancel()
    }
}()
```

#### Metrics-Based Circuit Breaking

```go
func shouldCircuitBreak(provider types.Provider) bool {
    metrics := provider.GetMetrics()

    // Circuit break if error rate is too high
    if metrics.RequestCount > 10 {
        errorRate := float64(metrics.ErrorCount) / float64(metrics.RequestCount)
        if errorRate > 0.5 { // 50% error rate
            return true
        }
    }

    // Circuit break if latency is too high
    if metrics.AverageLatency > 10*time.Second {
        return true
    }

    return false
}
```

#### Performance Alerting

```go
func checkPerformance(provider types.Provider) []string {
    metrics := provider.GetMetrics()
    var alerts []string

    // Alert on high error rate
    if metrics.RequestCount > 0 {
        errorRate := float64(metrics.ErrorCount) / float64(metrics.RequestCount)
        if errorRate > 0.1 {
            alerts = append(alerts,
                fmt.Sprintf("High error rate: %.2f%%", errorRate*100))
        }
    }

    // Alert on high latency
    if metrics.AverageLatency > 5*time.Second {
        alerts = append(alerts,
            fmt.Sprintf("High latency: %v", metrics.AverageLatency))
    }

    // Alert on unhealthy status
    if !metrics.HealthStatus.Healthy {
        alerts = append(alerts,
            fmt.Sprintf("Unhealthy: %s", metrics.HealthStatus.Message))
    }

    return alerts
}
```

### Credential-Level Metrics

#### Monitoring Refresh Patterns

```go
func monitorRefreshPatterns(manager *oauthmanager.OAuthKeyManager) {
    allMetrics := manager.GetAllMetrics()

    for credID, metrics := range allMetrics {
        if metrics.RequestCount > 0 {
            requestsPerRefresh := float64(metrics.RequestCount) / float64(metrics.RefreshCount)

            // Alert if refresh rate is too high
            if requestsPerRefresh < 10 {
                fmt.Printf("WARNING: %s is refreshing too frequently (%.1f requests per refresh)\n",
                    credID, requestsPerRefresh)
            }
        }
    }
}
```

#### Detecting Imbalanced Load

```go
func detectLoadImbalance(manager *oauthmanager.OAuthKeyManager) {
    allMetrics := manager.GetAllMetrics()

    if len(allMetrics) < 2 {
        return // Need at least 2 credentials to compare
    }

    var counts []int64
    for _, metrics := range allMetrics {
        counts = append(counts, metrics.RequestCount)
    }

    // Calculate average
    var sum int64
    for _, count := range counts {
        sum += count
    }
    avg := float64(sum) / float64(len(counts))

    // Check for outliers (> 50% deviation from average)
    for credID, metrics := range allMetrics {
        deviation := math.Abs(float64(metrics.RequestCount)-avg) / avg
        if deviation > 0.5 {
            fmt.Printf("WARNING: %s has unbalanced load (%.1f%% deviation from average)\n",
                credID, deviation*100)
        }
    }
}
```

#### Cost Budget Monitoring

```go
func monitorCostBudget(manager *oauthmanager.OAuthKeyManager, budgetPerCredential float64, costPer1kTokens float64) {
    allMetrics := manager.GetAllMetrics()

    for credID, metrics := range allMetrics {
        cost := float64(metrics.TokensUsed) / 1000.0 * costPer1kTokens

        if cost > budgetPerCredential {
            fmt.Printf("ALERT: %s exceeded budget ($%.2f / $%.2f)\n",
                credID, cost, budgetPerCredential)
        } else {
            remaining := budgetPerCredential - cost
            percentUsed := (cost / budgetPerCredential) * 100
            fmt.Printf("%s: $%.2f used ($%.2f remaining, %.1f%% of budget)\n",
                credID, cost, remaining, percentUsed)
        }
    }
}
```

## Thread Safety

All metrics operations are thread-safe:

- **Provider Metrics**: Protected by `sync.RWMutex` in `BaseProvider`
- **Credential Metrics**: Protected by `sync.RWMutex` per `CredentialMetrics`
- **Read Operations**: Use `RLock()/RUnlock()` for concurrent reads
- **Write Operations**: Use `Lock()/Unlock()` for exclusive writes
- **Snapshots**: `GetMetrics()` and `GetCredentialMetrics()` return copies

## Testing

### Unit Testing Metrics

```go
func TestProviderMetrics(t *testing.T) {
    provider := createTestProvider()

    // Initial state
    metrics := provider.GetMetrics()
    assert.Equal(t, int64(0), metrics.RequestCount)

    // Make a request
    ctx := context.Background()
    _, err := provider.GenerateChatCompletion(ctx, testOptions)

    // Verify metrics updated
    metrics = provider.GetMetrics()
    assert.Equal(t, int64(1), metrics.RequestCount)

    if err == nil {
        assert.Equal(t, int64(1), metrics.SuccessCount)
        assert.Greater(t, metrics.TokensUsed, int64(0))
    } else {
        assert.Equal(t, int64(1), metrics.ErrorCount)
        assert.NotEmpty(t, metrics.LastError)
    }
}
```

### Integration Testing

See `examples/demo-client/` for a complete working example that demonstrates metrics collection across multiple providers with both provider-level and credential-level tracking.

## Performance Considerations

### Memory Usage

- Provider metrics: ~200 bytes per provider
- Credential metrics: ~150 bytes per credential
- Total for 10 providers with 3 credentials each: ~5 KB

### CPU Overhead

- Metrics updates: ~100ns per operation (negligible)
- Mutex contention: Minimal with RWMutex design
- GetMetrics() creates copy: ~50ns per call

### Best Practices

- **Read snapshots**: Use `GetMetrics()` to get a copy, don't repeatedly call it
- **Batch operations**: Metrics are updated automatically, no manual batching needed
- **Export frequency**: Export metrics every 10-60 seconds for monitoring systems
- **Cleanup**: No manual cleanup needed, metrics are cleared when provider is destroyed

## Related Documentation

- [OAuth Manager Package](/pkg/oauthmanager/) - Credential-level metrics implementation
- [KeyManager Package](/pkg/keymanager/) - Simple multi-key management
- [Provider Interface](./PROVIDER_INTERFACE.md) - Provider interface specification
- [Examples](../examples/) - Complete working examples

## License

Part of the AI Provider Kit project.
