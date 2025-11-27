# Multi-Key Management Strategies

## Overview

The AI Provider Kit implements different multi-key management strategies across providers, optimized for each provider's specific characteristics and use cases. This document describes the various approaches and their trade-offs.

## Provider Implementations

### Anthropic - Advanced Multi-Key Manager

**Description**: "Anthropic Claude models with multi-key failover and load balancing"

#### Features
- **Health Tracking**: Each API key maintains individual health metrics
- **Exponential Backoff**: Failed keys enter backoff periods (1s, 2s, 4s, 8s, max 60s)
- **Automatic Failover**: Tries up to 3 keys per request
- **Load Balancing**: Round-robin distribution across healthy keys
- **Health Recovery**: Keys automatically recover after successful requests

#### Implementation Details
```go
type MultiKeyManager struct {
    keys         []string
    currentIndex uint32 // Atomic counter for round-robin
    keyHealth    map[string]*keyHealth
}

type keyHealth struct {
    failureCount int
    lastFailure  time.Time
    lastSuccess  time.Time
    isHealthy    bool
    backoffUntil time.Time
}
```

#### When to Use This Strategy
- High-volume production environments
- When API keys have rate limits
- Need for automatic failure recovery
- Critical applications requiring high availability

### Cerebras - Simple Round-Robin Failover

**Description**: "Cerebras ultra-fast inference with API key failover"

#### Features
- **Simple Rotation**: Basic round-robin through available keys
- **Fast Failover**: Immediately tries next key on failure
- **Minimal Overhead**: No health tracking or backoff delays
- **All-Key Retry**: Attempts all keys before failing

#### Implementation Details
```go
type CerebrasProvider struct {
    apiKeys         []string
    currentKeyIndex int
}

// Simple rotation without health tracking
func (p *CerebrasProvider) getNextAPIKey() string {
    key := p.apiKeys[p.currentKeyIndex]
    p.currentKeyIndex = (p.currentKeyIndex + 1) % len(p.apiKeys)
    return key
}
```

#### When to Use This Strategy
- Low-latency requirements (Cerebras focuses on ultra-fast inference)
- Small number of API keys (2-3)
- When keys are unlikely to fail or have rate limits
- Minimal overhead is critical

### Comparison Matrix

| Feature | Anthropic | Cerebras |
|---------|-----------|----------|
| **Health Tracking** | ✅ Per-key health status | ❌ No tracking |
| **Exponential Backoff** | ✅ 1s-60s backoff | ❌ Immediate retry |
| **Load Balancing** | ✅ Round-robin | ✅ Round-robin |
| **Failure Memory** | ✅ 3 strikes rule | ❌ No memory |
| **Recovery Time** | Gradual with backoff | Immediate |
| **Overhead** | Higher (health tracking) | Minimal |
| **Best For** | High availability | Low latency |

## Configuration Examples

### Anthropic with Multiple Keys
```yaml
anthropic:
  model: claude-3-5-sonnet-20241022
  api_keys:
    - sk-ant-api-key-1
    - sk-ant-api-key-2
    - sk-ant-api-key-3
```

With this configuration, Anthropic will:
1. Distribute requests across all three keys
2. Track health of each key independently
3. Automatically failover if a key fails
4. Put failed keys in exponential backoff
5. Recover keys after successful requests

### Cerebras with Multiple Keys
```yaml
cerebras:
  model: zai-glm-4.6
  api_keys:
    - csk-key-1
    - csk-key-2
```

With this configuration, Cerebras will:
1. Alternate between keys for each request
2. Immediately try the next key on failure
3. No delay or backoff between attempts
4. Fail fast if all keys are exhausted

## Design Rationale

### Why Different Strategies?

1. **Performance Characteristics**
   - Anthropic: Focused on reliability and availability
   - Cerebras: Optimized for ultra-low latency

2. **Provider Behavior**
   - Anthropic: May have rate limits, benefits from backoff
   - Cerebras: Fast inference, minimal overhead preferred

3. **Use Case Optimization**
   - Anthropic: Complex reasoning tasks where reliability matters
   - Cerebras: Real-time applications where speed is critical

## Implementation Guidelines

### When to Implement Advanced Multi-Key Management

Consider the Anthropic-style approach when:
- Provider has rate limits or quotas
- Keys may become temporarily unavailable
- High availability is critical
- You have many API keys to manage
- Request distribution needs to be balanced

### When to Keep It Simple

Use the Cerebras-style approach when:
- Latency is the primary concern
- Provider is highly reliable
- You have few API keys (1-3)
- Overhead must be minimized
- Fast failure detection is needed

## Future Enhancements

### Potential Improvements for Cerebras
While the simple approach works well for Cerebras' use case, potential enhancements could include:
- Optional health tracking (disabled by default)
- Configurable retry strategies
- Metrics per API key

### Potential Improvements for Anthropic
The current implementation could be enhanced with:
- Weighted load balancing based on key performance
- Adaptive backoff based on error patterns
- Key priority levels
- Circuit breaker pattern

## Best Practices

### API Key Management
1. **Separate Keys by Environment**: Use different keys for dev/staging/production
2. **Monitor Key Usage**: Track metrics per key to identify issues
3. **Rotate Keys Regularly**: Implement key rotation for security
4. **Set Alerts**: Monitor for key failures or high error rates

### Configuration
1. **Start Simple**: Begin with single keys and add multi-key as needed
2. **Test Failover**: Regularly test failover behavior
3. **Document Keys**: Keep clear documentation of key purposes
4. **Secure Storage**: Never commit keys to version control

### Error Handling
1. **Log Key Failures**: Track which keys are failing and why
2. **Alert on Patterns**: Notify when multiple keys fail
3. **Graceful Degradation**: Have fallback strategies
4. **User Communication**: Inform users of degraded service

## Code Examples

### Using Multi-Key Providers

```go
// Anthropic with multiple keys and automatic failover
config := types.ProviderConfig{
    Type: types.ProviderTypeAnthropic,
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{
            os.Getenv("ANTHROPIC_KEY_1"),
            os.Getenv("ANTHROPIC_KEY_2"),
            os.Getenv("ANTHROPIC_KEY_3"),
        },
    },
}
provider := anthropic.NewAnthropicProvider(config)

// The provider will automatically:
// - Distribute load across keys
// - Failover on errors
// - Track key health
// - Apply exponential backoff
```

### Monitoring Key Health

```go
// Get provider metrics to see key performance
metrics := provider.GetMetrics()
fmt.Printf("Total Requests: %d\n", metrics.RequestCount)
fmt.Printf("Failed Requests: %d\n", metrics.ErrorCount)
fmt.Printf("Success Rate: %.2f%%\n",
    float64(metrics.SuccessCount)/float64(metrics.RequestCount)*100)
```

## Conclusion

The AI Provider Kit's multi-key strategies are tailored to each provider's characteristics:
- **Anthropic**: Sophisticated management for high availability
- **Cerebras**: Simple and fast for low-latency requirements

Choose the appropriate strategy based on your specific needs for reliability, latency, and operational complexity.