# AI Provider Kit Documentation

Complete documentation for the AI Provider Kit - a unified interface for multiple AI providers with enterprise-grade credential management.

## Core Documentation

### Architecture & API Design

- **[Phase 3 Comprehensive Guide](/docs/PHASE_3_COMPREHENSIVE_GUIDE.md)** - Complete guide to interface segregation and standardized core API with practical examples
- **[Standardized API Guide](/docs/STANDARDIZED_API_GUIDE.md)** - Using the new standardized core API across providers

### SDK Documentation

- **[Getting Started](/docs/SDK_GETTING_STARTED.md)** - Quick start guide for the AI Provider Kit SDK
- **[API Reference](/docs/SDK_API_REFERENCE.md)** - Complete API documentation including new interfaces
- **[Best Practices](/docs/SDK_BEST_PRACTICES.md)** - Recommended patterns and practices, including interface segregation
- **[Provider Guides](/docs/SDK_PROVIDER_GUIDES.md)** - Provider-specific documentation with new patterns
- **[Migration Guide](/docs/SDK_MIGRATION_GUIDE.md)** - Migrating between providers and versions
- **[Advanced Features](/docs/SDK_ADVANCED_FEATURES.md)** - Advanced SDK features and capabilities

### Credential Management

- **[OAuth Manager](/docs/OAUTH_MANAGER.md)** - Enterprise OAuth credential management with automatic failover, token refresh, rotation, and monitoring
- **[Multi-Key Strategies](/docs/MULTI_KEY_STRATEGIES.md)** - Comparison of credential management approaches (simple vs advanced)
- **[Authentication Guide](/docs/SDK_AUTHENTICATION_GUIDE.md)** - Comprehensive authentication documentation

### Monitoring & Observability

- **[Metrics](/docs/METRICS.md)** - Provider-level and credential-level metrics tracking, export formats, and best practices
- **[Common Utilities](/docs/SDK_COMMON_UTILITIES.md)** - Shared utilities and helpers including Phase 3 core API

### Troubleshooting

- **[Troubleshooting FAQ](/docs/SDK_TROUBLESHOOTING_FAQ.md)** - Common issues and solutions

## Package Documentation

### Credential Managers

- **[OAuthManager](/pkg/oauthmanager/)** - Multi-OAuth credential management for providers like Gemini and Qwen
- **[KeyManager](/pkg/keymanager/)** - Multi-API-key management for providers like Anthropic and Cerebras

### Providers

- **[Anthropic](/pkg/providers/anthropic/)** - Claude models with multi-key failover
- **[Cerebras](/pkg/providers/cerebras/)** - Ultra-fast inference with multi-key support
- **[Gemini](/pkg/providers/gemini/)** - Google Gemini with multi-OAuth support
- **[Qwen](/pkg/providers/qwen/)** - Alibaba Qwen with multi-OAuth support
- **[Custom Providers](/pkg/providers/)** - OpenAI-compatible providers (Groq, Zen, etc.)

## Quick Start

### Basic Usage

```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Create provider
provider := anthropic.NewAnthropicProvider(types.ProviderConfig{
    APIKey: "your-api-key",
})

// Generate completion
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "Hello!"},
    },
})
```

### Multi-Credential Management

#### API Keys (Simple)

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/keymanager"

// Automatic failover across multiple API keys
manager := keymanager.NewKeyManager("Anthropic", []string{
    "sk-ant-key-1",
    "sk-ant-key-2",
    "sk-ant-key-3",
})
```

#### OAuth Credentials (Advanced)

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/oauthmanager"

// Automatic failover, token refresh, rotation, and monitoring
manager := oauthmanager.NewOAuthKeyManager("Gemini", credentials, refreshFunc)
```

See [OAuth Manager](/docs/OAUTH_MANAGER.md) for complete documentation.

## Features by Provider

| Provider | Multi-Key | Multi-OAuth | Health Tracking | Metrics | Token Refresh |
|----------|-----------|-------------|-----------------|---------|---------------|
| **Anthropic** | ✅ | ❌ | ✅ Advanced | ✅ | N/A |
| **Cerebras** | ✅ | ❌ | ✅ Advanced | ✅ | N/A |
| **Gemini** | ❌ | ✅ | ✅ Advanced | ✅ Provider + Credential | ✅ Automatic |
| **Qwen** | ❌ | ✅ | ✅ Advanced | ✅ Provider + Credential | ✅ Automatic |
| **OpenAI** | ✅ | ❌ | ✅ Basic | ✅ | N/A |
| **Custom** | ✅ | ❌ | ✅ Basic | ✅ | N/A |

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Provider Interface                          │
│  - GenerateChatCompletion()                                  │
│  - GetMetrics()                                              │
│  - HealthCheck()                                             │
└─────────────────────────────────────────────────────────────┘
                              │
                ┌─────────────┴─────────────┐
                ▼                           ▼
    ┌───────────────────────┐   ┌───────────────────────┐
    │   KeyManager          │   │   OAuthManager        │
    │   (API Keys)          │   │   (OAuth Creds)       │
    ├───────────────────────┤   ├───────────────────────┤
    │ - Round-robin         │   │ - Round-robin         │
    │ - Health tracking     │   │ - Health tracking     │
    │ - Exponential backoff │   │ - Exponential backoff │
    │ - Load balancing      │   │ - Token refresh       │
    │                       │   │ - Credential metrics  │
    │                       │   │ - Rotation policies   │
    │                       │   │ - Monitoring/alerts   │
    └───────────────────────┘   └───────────────────────┘
                │                           │
                ▼                           ▼
    ┌───────────────────────┐   ┌───────────────────────┐
    │  Anthropic, Cerebras  │   │   Gemini, Qwen        │
    │  OpenAI, Custom       │   │   (OAuth providers)   │
    └───────────────────────┘   └───────────────────────┘
```

## Key Concepts

### Provider-Level Metrics

All providers track aggregate metrics:
- Request counts (total, success, error)
- Performance (latency, throughput)
- Resource usage (tokens)
- Health status

See [Metrics Documentation](/docs/METRICS.md#provider-level-metrics).

### Credential-Level Metrics

OAuth providers track per-credential metrics:
- Individual account usage
- Per-credential performance
- Cost attribution
- Refresh patterns

See [Metrics Documentation](/docs/METRICS.md#credential-level-metrics).

### Health Tracking

All credential managers implement:
- **Failure counting** - Track consecutive failures
- **Exponential backoff** - 1s → 2s → 4s → 8s → max 60s
- **Health status** - Unhealthy after 3 consecutive failures
- **Automatic recovery** - Reset on first success

### Load Balancing

Both KeyManager and OAuthManager use:
- **Round-robin distribution** - Fair load across credentials
- **Backoff skipping** - Skip credentials in backoff period
- **Atomic operations** - Thread-safe rotation

## Examples

Complete working examples are available in [`/examples`](/examples):

- **[Demo Client](/examples/demo-client/)** - Comprehensive example using multiple providers with OAuth callbacks and metrics

## Best Practices

### Security

- **Never commit credentials** - Use environment variables or secrets management
- **Rotate credentials regularly** - Implement rotation policies for OAuth providers
- **Monitor for anomalies** - Alert on unusual patterns or high failure rates
- **Audit token refreshes** - Maintain logs for security compliance

### Performance

- **Use appropriate strategies** - Simple for speed, advanced for reliability
- **Monitor metrics** - Track latency and request distribution
- **Configure buffers appropriately** - Balance between refresh frequency and safety
- **Enable adaptive refresh** - Let the system adjust to usage patterns

### Cost Management

- **Track usage per credential** - Monitor token consumption per account
- **Set budgets and alerts** - Prevent unexpected costs
- **Export metrics** - Integrate with billing and cost tracking systems
- **Use credential-level metrics** - Understand costs at granular level

## Contributing

See the main project README for contribution guidelines.

## License

Part of the AI Provider Kit project.
