# Virtual Providers

Composite provider implementations that combine multiple underlying providers with different strategies.

## Overview

Virtual providers implement the standard `types.Provider` interface but delegate to multiple underlying providers. This allows you to build sophisticated request handling patterns without changing your application code.

**Key Benefits:**

- **Minimize latency** by racing providers
- **Maximize reliability** with automatic failover
- **Distribute load** across multiple API keys
- **Transparent integration** - use like any other provider

## Available Providers

### Racing Provider

Sends requests to multiple providers **concurrently** and returns the first successful response.

**Use Cases:**
- Minimize latency by racing fast providers
- Get best-of-N responses with quality validation
- Automatic failover when one provider is slow

**Location:** `racing/provider.go`

### Fallback Provider

Tries providers **sequentially**, falling back to the next on failure.

**Use Cases:**
- High availability with primary/backup providers
- Cost optimization (try cheaper providers first)
- Graceful degradation during outages

**Location:** `fallback/provider.go`

### Load Balance Provider

Distributes requests **across providers** using round-robin or random selection.

**Use Cases:**
- Distribute load across multiple API keys
- Rate limit management
- Geographic distribution

**Location:** `loadbalance/provider.go`

## Racing Provider

### Basic Configuration

```yaml
providers:
  fast-ai:
    type: racing
    config:
      providers:
        - openai
        - anthropic
      timeout_ms: 5000
      strategy: first_wins
```

### Strategies

#### 1. First Wins (Default)

Returns immediately when any provider succeeds.

```go
config := &racing.Config{
    ProviderNames: []string{"openai", "anthropic", "gemini"},
    TimeoutMS:     5000,
    Strategy:      racing.StrategyFirstWins,
}

racingProvider := racing.NewRacingProvider("fast-ai", config)
```

**Best for:** Minimizing latency

#### 2. Weighted

Waits for a grace period to collect multiple responses, then picks the best based on historical performance.

```go
config := &racing.Config{
    ProviderNames:  []string{"openai", "anthropic"},
    TimeoutMS:      5000,
    GracePeriodMS:  500, // Wait 500ms for additional responses
    Strategy:       racing.StrategyWeighted,
    PerformanceFile: "performance-stats.json",
}
```

**Best for:** Balancing speed with quality

#### 3. Quality

Similar to weighted but emphasizes response quality over speed.

```go
config := &racing.Config{
    ProviderNames:  []string{"openai", "anthropic"},
    TimeoutMS:      10000,
    GracePeriodMS:  1000,
    Strategy:       racing.StrategyQuality,
}
```

**Best for:** Quality-critical applications

### Usage Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/racing"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Create underlying providers
    openai := factory.NewOpenAI(types.ProviderConfig{
        Type:     "openai",
        Name:     "openai",
        APIKeyEnv: "OPENAI_API_KEY",
    })

    anthropic := factory.NewAnthropic(types.ProviderConfig{
        Type:     "anthropic",
        Name:     "anthropic",
        APIKeyEnv: "ANTHROPIC_API_KEY",
    })

    // Create racing provider
    config := &racing.Config{
        TimeoutMS:     5000,
        GracePeriodMS: 500,
        Strategy:      racing.StrategyWeighted,
    }

    racer := racing.NewRacingProvider("fast-ai", config)
    racer.SetProviders([]types.Provider{openai, anthropic})

    // Use like any provider
    stream, err := racer.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: "Hello!"},
        },
    })

    if err != nil {
        panic(err)
    }

    chunk, _ := stream.Next()

    // Check which provider won
    winner := chunk.Metadata["racing_winner"]
    latency := chunk.Metadata["racing_latency_ms"]
    fmt.Printf("Winner: %s (latency: %dms)\n", winner, latency)
}
```

### Performance Tracking

The racing provider tracks performance metrics for each provider:

```go
// Get performance stats
stats := racingProvider.GetPerformanceStats()

for name, stat := range stats {
    fmt.Printf("%s: %.2f win rate, avg latency: %dms\n",
        name, stat.WinRate, stat.AvgLatency)
}
```

### Response Metadata

Racing providers add metadata to responses:

```json
{
  "racing_winner": "openai",
  "racing_latency_ms": 342
}
```

## Fallback Provider

### Basic Configuration

```yaml
providers:
  reliable-ai:
    type: fallback
    config:
      providers:
        - openai        # Try first
        - anthropic     # Then this
        - gemini        # Finally this
      max_retries: 3
```

### Usage Example

```go
package main

import (
    "context"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/fallback"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Create providers in priority order
    primary := factory.NewOpenAI(types.ProviderConfig{
        Type: "openai",
        Name: "openai",
        APIKeyEnv: "OPENAI_API_KEY",
    })

    secondary := factory.NewAnthropic(types.ProviderConfig{
        Type: "anthropic",
        Name: "anthropic",
        APIKeyEnv: "ANTHROPIC_API_KEY",
    })

    backup := factory.NewGemini(types.ProviderConfig{
        Type: "gemini",
        Name: "gemini",
        APIKeyEnv: "GEMINI_API_KEY",
    })

    // Create fallback provider
    config := &fallback.Config{
        MaxRetries: 3,
    }

    fb := fallback.NewFallbackProvider("reliable-ai", config)
    fb.SetProviders([]types.Provider{primary, secondary, backup})

    // Use normally - will try providers in order
    stream, err := fb.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: "Hello!"},
        },
    })

    if err != nil {
        panic(err)
    }

    chunk, _ := stream.Next()

    // Check which provider succeeded
    provider := chunk.Metadata["fallback_provider"]
    index := chunk.Metadata["fallback_index"]
    println("Used provider:", provider, "at index:", index)
}
```

### Response Metadata

Fallback providers add metadata indicating which provider succeeded:

```json
{
  "fallback_provider": "anthropic",
  "fallback_index": 1
}
```

### Error Handling

The fallback provider tries each provider in sequence. If all fail, it returns the last error:

```
all providers failed, last error: anthropic: rate limit exceeded
```

## Load Balance Provider

### Basic Configuration

```yaml
providers:
  balanced-ai:
    type: loadbalance
    config:
      providers:
        - openai-key1
        - openai-key2
        - openai-key3
      strategy: round_robin
```

### Strategies

#### Round Robin

Distributes requests evenly in circular order.

```go
config := &loadbalance.Config{
    ProviderNames: []string{"openai-1", "openai-2", "openai-3"},
    Strategy:      loadbalance.StrategyRoundRobin,
}
```

#### Random

Randomly selects a provider for each request.

```go
config := &loadbalance.Config{
    ProviderNames: []string{"openai-1", "openai-2"},
    Strategy:      loadbalance.StrategyRandom,
}
```

### Usage Example

```go
package main

import (
    "context"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/loadbalance"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Create multiple providers with different API keys
    provider1 := factory.NewOpenAI(types.ProviderConfig{
        Type:   "openai",
        Name:   "openai-1",
        APIKey: "key-1",
    })

    provider2 := factory.NewOpenAI(types.ProviderConfig{
        Type:   "openai",
        Name:   "openai-2",
        APIKey: "key-2",
    })

    provider3 := factory.NewOpenAI(types.ProviderConfig{
        Type:   "openai",
        Name:   "openai-3",
        APIKey: "key-3",
    })

    // Create load balancer
    config := &loadbalance.Config{
        Strategy: loadbalance.StrategyRoundRobin,
    }

    lb := loadbalance.NewLoadBalanceProvider("balanced-ai", config)
    lb.SetProviders([]types.Provider{provider1, provider2, provider3})

    // Each request uses a different provider
    for i := 0; i < 5; i++ {
        stream, _ := lb.GenerateChatCompletion(context.Background(), types.GenerateOptions{
            Messages: []types.ChatMessage{
                {Role: "user", Content: "Hello!"},
            },
        })

        chunk, _ := stream.Next()
        println("Request", i, "content:", chunk.Content)
    }
}
```

## Combining Virtual Providers

Virtual providers can be nested for sophisticated patterns:

### Racing + Fallback

Race fast providers, with fallback to slower but reliable ones:

```go
// Fast tier: race OpenAI vs Anthropic
fastRacer := racing.NewRacingProvider("fast", &racing.Config{
    TimeoutMS: 3000,
    Strategy:  racing.StrategyFirstWins,
})
fastRacer.SetProviders([]types.Provider{openai, anthropic})

// Slow tier: Gemini as backup
slowProvider := gemini

// Fallback from fast to slow
fb := fallback.NewFallbackProvider("fast-with-backup", &fallback.Config{})
fb.SetProviders([]types.Provider{fastRacer, slowProvider})
```

### Load Balance + Fallback

Distribute across multiple accounts with fallback:

```go
// Primary load balancer
primaryLB := loadbalance.NewLoadBalanceProvider("primary", &loadbalance.Config{
    Strategy: loadbalance.StrategyRoundRobin,
})
primaryLB.SetProviders([]types.Provider{openai1, openai2, openai3})

// Backup provider
backup := anthropic

// Fallback if all primary accounts fail
fb := fallback.NewFallbackProvider("lb-with-backup", &fallback.Config{})
fb.SetProviders([]types.Provider{primaryLB, backup})
```

### Racing + Load Balance

Race across load-balanced provider pools:

```go
// Pool 1: OpenAI accounts
openaiLB := loadbalance.NewLoadBalanceProvider("openai-pool", &loadbalance.Config{
    Strategy: loadbalance.StrategyRoundRobin,
})
openaiLB.SetProviders([]types.Provider{openai1, openai2})

// Pool 2: Anthropic accounts
anthropicLB := loadbalance.NewLoadBalanceProvider("anthropic-pool", &loadbalance.Config{
    Strategy: loadbalance.StrategyRoundRobin,
})
anthropicLB.SetProviders([]types.Provider{anthropic1, anthropic2})

// Race the pools
racer := racing.NewRacingProvider("pool-racer", &racing.Config{
    TimeoutMS: 5000,
    Strategy:  racing.StrategyWeighted,
})
racer.SetProviders([]types.Provider{openaiLB, anthropicLB})
```

## Configuration Examples

### YAML Configuration

```yaml
providers:
  # Simple racing
  fast:
    type: racing
    config:
      providers: [openai, anthropic]
      timeout_ms: 5000
      strategy: first_wins

  # Weighted racing with performance tracking
  smart:
    type: racing
    config:
      providers: [openai, anthropic, gemini]
      timeout_ms: 10000
      grace_period_ms: 500
      strategy: weighted
      performance_file: ./performance.json

  # Primary/backup fallback
  reliable:
    type: fallback
    config:
      providers: [openai, anthropic, gemini]
      max_retries: 3

  # Cost-optimized fallback
  cheap-first:
    type: fallback
    config:
      providers: [gemini, openai, anthropic]  # Cheapest first

  # Load balancing across accounts
  distributed:
    type: loadbalance
    config:
      providers: [openai-1, openai-2, openai-3]
      strategy: round_robin

  # Random distribution
  random-lb:
    type: loadbalance
    config:
      providers: [provider-a, provider-b]
      strategy: random
```

### Factory Integration

```go
package main

import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Load config from YAML
    config := loadConfig("config.yaml")

    // Initialize all providers (including virtual ones)
    providers, err := factory.InitializeProviders(config.Providers)
    if err != nil {
        panic(err)
    }

    // Use virtual providers like normal providers
    fastProvider := providers["fast"]
    reliableProvider := providers["reliable"]

    // They work with backend server
    server := backend.NewServer(config, providers)
    server.Start()
}
```

## Best Practices

### Racing Provider

1. **Use fast providers** - Racing only helps if providers have different latencies
2. **Set reasonable timeouts** - Too short and all providers fail, too long defeats the purpose
3. **Use weighted strategy** - For best balance of speed and quality
4. **Track performance** - Monitor which providers win to optimize configuration
5. **Consider cost** - Racing uses multiple API calls (charges from multiple providers)

### Fallback Provider

1. **Order by priority** - Put most reliable/preferred provider first
2. **Consider cost** - Put cheaper providers before expensive ones if quality is similar
3. **Set appropriate retries** - Usually 2-3 is sufficient
4. **Monitor fallback rate** - High fallback rates indicate primary provider issues
5. **Don't chain too many** - 3-4 providers max, more delays error reporting

### Load Balance Provider

1. **Use for rate limits** - Distribute across multiple API keys
2. **Round-robin for fairness** - Even distribution across providers
3. **Random for independence** - When you don't want predictable patterns
4. **Monitor all providers** - One bad provider affects percentage of requests
5. **Combine with fallback** - For resilience when providers fail

### General

1. **Test thoroughly** - Virtual providers add complexity
2. **Monitor metadata** - Track which providers are used
3. **Log performance** - Understand actual vs expected behavior
4. **Start simple** - Add complexity only when needed
5. **Document intent** - Comment why specific providers/strategies are chosen

## Troubleshooting

### All providers failing

```go
// Check provider health individually
for name, provider := range providers {
    err := provider.HealthCheck(context.Background())
    fmt.Printf("%s: %v\n", name, err)
}
```

### Racing always picks same provider

- Check if other providers are actually faster
- Verify all providers are configured correctly
- Ensure timeout is sufficient for slower providers
- Consider using weighted strategy with grace period

### Fallback always uses backup

- Primary provider likely failing
- Check API keys and quotas
- Verify network connectivity
- Check provider-specific configuration

### Load balancer not distributing evenly

- Confirm strategy is round_robin
- Check if some providers are failing
- Verify all providers in the list are valid

## Performance Considerations

| Provider Type | API Calls per Request | Latency | Cost Impact |
|---------------|----------------------|---------|-------------|
| Racing | N (all providers) | Minimum of all | N× API calls |
| Fallback | 1 to N (stops at success) | Single provider | 1-N× API calls |
| Load Balance | 1 | Single provider | 1× API calls |

**Recommendations:**

- Use **racing** when latency is critical and cost is acceptable
- Use **fallback** when reliability is critical
- Use **load balance** when distributing load or managing rate limits

## Examples

See the [examples/virtual-providers/](../../../examples/virtual-providers/) directory for:

- `racing/` - Racing provider with performance tracking
- `fallback/` - Multi-tier fallback configuration
- `loadbalance/` - Load balancing across API keys
- `combined/` - Complex nested virtual provider setups
