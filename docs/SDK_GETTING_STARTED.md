# Getting Started with AI Provider Kit

A comprehensive guide to getting started with the AI Provider Kit - a unified Go SDK for multiple AI providers with enterprise-grade features.

## Table of Contents

- [Introduction](#introduction)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Basic Usage Examples](#basic-usage-examples)
- [Configuration](#configuration)
- [Provider Selection](#provider-selection)
- [Next Steps](#next-steps)

---

## Introduction

### What is AI Provider Kit?

AI Provider Kit is a comprehensive Go library that provides a unified interface for interacting with multiple AI providers. Instead of learning and maintaining separate SDKs for each AI service, you can use one consistent API that works across all supported providers.

### Key Benefits and Features

- **Unified Interface**: Write once, run on any provider - switch between providers without code changes
- **Multi-Provider Support**: Works with 9+ AI providers including cloud and local options
- **Enterprise-Grade Authentication**: API keys, OAuth 2.0, and multi-credential management
- **Automatic Failover**: Load balancing across multiple API keys/credentials with health tracking
- **Streaming Support**: Real-time response streaming for better user experience
- **Tool Calling**: Universal tool/function calling format across all providers
- **Health Monitoring**: Built-in health checks and comprehensive metrics collection
- **Shared Utilities**: Common functionality for language detection, file processing, rate limiting, and caching
- **Production Ready**: 100% test coverage, extensive validation, and proven in production
- **Extensible**: Easy to add custom providers using shared utilities and testing framework

### Supported Providers

#### Cloud Providers
- **OpenAI** - GPT-4, GPT-3.5, and embeddings
- **Anthropic** - Claude 3.5 Sonnet, Claude 3 Opus, and other Claude models
- **Google Gemini** - Gemini 2.5 Pro, Gemini 2.5 Flash, and other Gemini models
- **Cerebras** - Ultra-fast inference with Llama models
- **OpenRouter** - Multi-provider routing service
- **Qwen** - Alibaba's Qwen models
- **xAI** - Grok models (via OpenAI-compatible API)
- **Mistral** - Mistral AI models (via OpenAI-compatible API)
- **Deepseek** - Deepseek models (via OpenAI-compatible API)

#### Local Providers
- **LM Studio** - Local model serving
- **Ollama** - Local model management
- **Llama.cpp** - Efficient local inference

### Prerequisites

- **Go 1.24.0 or later** - The SDK requires Go 1.24.0+
- **API keys or OAuth credentials** - For the providers you want to use
- **Network access** - To cloud provider APIs (not needed for local providers)

---

## Installation

### Using Go Modules

Install the AI Provider Kit using Go modules:

```bash
go get github.com/cecil-the-coder/ai-provider-kit@latest
```

### Version Requirements

The SDK requires:
- Go 1.24.0 or later
- No additional system dependencies

### Dependencies Overview

The SDK has minimal external dependencies:

```go
require (
    github.com/google/uuid v1.6.0           // UUID generation
    golang.org/x/oauth2 v0.33.0             // OAuth 2.0 support
    golang.org/x/time v0.14.0               // Rate limiting
    gopkg.in/yaml.v3 v3.0.1                 // YAML configuration
)
```

All dependencies are automatically managed by Go modules.

---

## Quick Start

### Minimal Working Example

Here's the simplest way to get started - a complete working example in just 10 lines:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // 1. Create provider factory
    f := factory.NewProviderFactory()
    factory.RegisterDefaultProviders(f)

    // 2. Configure provider
    config := types.ProviderConfig{
        Type:         types.ProviderTypeOpenAI,
        APIKey:       "your-api-key-here",
        DefaultModel: "gpt-4",
    }

    // 3. Create provider instance
    provider, err := f.CreateProvider(types.ProviderTypeOpenAI, config)
    if err != nil {
        log.Fatal(err)
    }

    // 4. Generate completion
    stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Prompt: "What is the capital of France?",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()

    // 5. Read response
    chunk, _ := stream.Next()
    fmt.Println(chunk.Content)
}
```

### Step-by-Step Explanation

1. **Create Provider Factory**: The factory manages provider registration and creation
2. **Register Default Providers**: Registers all built-in providers (OpenAI, Anthropic, etc.)
3. **Configure Provider**: Specify which provider to use and authentication details
4. **Create Provider Instance**: Factory creates a configured provider instance
5. **Generate Completion**: Send a prompt and receive a response stream
6. **Read Response**: Extract content from the response

### Expected Output

```
Paris
```

The response will vary depending on the model and prompt, but you'll receive a natural language answer to your question.

---

## Basic Usage Examples

### Creating a Provider Instance

#### Using the Factory Pattern (Recommended)

```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Create and initialize factory
f := factory.NewProviderFactory()
factory.RegisterDefaultProviders(f)

// Create OpenAI provider
provider, err := f.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    APIKey:       "sk-...",
    DefaultModel: "gpt-4",
})
```

#### Direct Provider Creation

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic"

// Create Anthropic provider directly
provider := anthropic.NewAnthropicProvider(types.ProviderConfig{
    APIKey:       "sk-ant-...",
    DefaultModel: "claude-sonnet-4-5",
})
```

### Making Your First API Call

#### Simple Prompt

```go
// Simple text prompt
stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
    Prompt: "Explain recursion in one sentence.",
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

// Read response
chunk, err := stream.Next()
if err != nil {
    log.Fatal(err)
}
fmt.Println(chunk.Content)
```

#### Chat Messages Format

```go
// Using messages for multi-turn conversations
stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "system", Content: "You are a helpful assistant."},
        {Role: "user", Content: "What is 2+2?"},
        {Role: "assistant", Content: "4"},
        {Role: "user", Content: "What is 3+3?"},
    },
    MaxTokens:   1000,
    Temperature: 0.7,
})
```

### Handling Responses

#### Non-Streaming Response

```go
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Hello, world!",
    Stream: false,  // Non-streaming mode
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

// Read single response
chunk, err := stream.Next()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Response: %s\n", chunk.Content)
fmt.Printf("Tokens Used: %d\n", chunk.Usage.TotalTokens)
```

#### Streaming Response

```go
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Write a short story about a robot.",
    Stream: true,  // Streaming mode
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

// Read chunks as they arrive
fmt.Print("Response: ")
for {
    chunk, err := stream.Next()
    if err != nil {
        break
    }

    fmt.Print(chunk.Content)

    if chunk.Done {
        break
    }
}
fmt.Println()
```

### Error Handling Basics

#### Comprehensive Error Handling

```go
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Hello!",
})
if err != nil {
    // Handle creation errors
    log.Printf("Failed to create completion: %v", err)
    return
}
defer stream.Close()

// Read response with error handling
chunk, err := stream.Next()
if err != nil {
    // Check error type
    if strings.Contains(err.Error(), "rate limit") {
        log.Println("Rate limited - please retry later")
    } else if strings.Contains(err.Error(), "authentication") {
        log.Println("Authentication failed - check your API key")
    } else {
        log.Printf("Request failed: %v", err)
    }
    return
}

fmt.Println(chunk.Content)
```

#### Health Checks

```go
// Check provider health before making requests
err := provider.HealthCheck(context.Background())
if err != nil {
    log.Printf("Provider is unhealthy: %v", err)
    // Handle unhealthy provider (retry, switch provider, etc.)
}
```

#### Metrics Monitoring

```go
// Get provider metrics for monitoring
metrics := provider.GetMetrics()

fmt.Printf("Total Requests: %d\n", metrics.RequestCount)
fmt.Printf("Success Rate: %.2f%%\n",
    float64(metrics.SuccessCount)/float64(metrics.RequestCount)*100)
fmt.Printf("Average Latency: %v\n", metrics.AverageLatency)

if metrics.ErrorCount > 0 {
    fmt.Printf("Last Error: %s\n", metrics.LastError)
}
```

---

## Configuration

### YAML Configuration Basics

The SDK supports YAML configuration files for managing multiple providers:

```yaml
providers:
  enabled:
    - openai
    - anthropic

  openai:
    api_key: sk-proj-YOUR_KEY_HERE
    default_model: gpt-4

  anthropic:
    api_key: sk-ant-YOUR_KEY_HERE
    default_model: claude-sonnet-4-5
```

#### Loading YAML Configuration

```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/examples/config"
)

// Load configuration from file
cfg, err := config.LoadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}

// Get provider configuration
entry := config.GetProviderEntry(cfg, "openai")
providerConfig := config.BuildProviderConfig("openai", entry)

// Create provider from config
provider, err := factory.CreateProvider(providerConfig.Type, providerConfig)
```

### Environment Variables

Instead of YAML files, you can use environment variables:

```bash
# Set environment variables
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GEMINI_PROJECT_ID="your-project-id"
```

```go
// Use environment variables directly
import "os"

provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    APIKey:       os.Getenv("OPENAI_API_KEY"),
    DefaultModel: "gpt-4",
})
```

### Programmatic Configuration

Configure providers entirely in code:

```go
// Full configuration in code
config := types.ProviderConfig{
    Type:         types.ProviderTypeAnthropic,
    Name:         "anthropic-primary",
    APIKey:       "sk-ant-...",
    DefaultModel: "claude-sonnet-4-5",
    Timeout:      30 * time.Second,
    MaxTokens:    8192,

    // Enable features
    SupportsStreaming:   true,
    SupportsToolCalling: true,
}

provider, err := factory.CreateProvider(config.Type, config)
if err != nil {
    log.Fatal(err)
}
```

### Config File Examples for Common Providers

#### OpenAI Configuration

```yaml
providers:
  enabled:
    - openai

  openai:
    api_key: sk-proj-YOUR_OPENAI_API_KEY
    default_model: gpt-4o
    max_tokens: 4096
    temperature: 0.7
```

#### Anthropic Configuration

```yaml
providers:
  enabled:
    - anthropic

  anthropic:
    api_key: sk-ant-YOUR_ANTHROPIC_API_KEY
    default_model: claude-sonnet-4-5
    max_tokens: 8192
    temperature: 0.7
```

#### Gemini with OAuth Configuration

```yaml
providers:
  enabled:
    - gemini

  gemini:
    project_id: your-gcp-project-id
    default_model: gemini-2.5-pro
    oauth_credentials:
      - id: default
        client_id: YOUR_CLIENT_ID.apps.googleusercontent.com
        client_secret: YOUR_CLIENT_SECRET
        access_token: ya29.YOUR_ACCESS_TOKEN
        refresh_token: 1//YOUR_REFRESH_TOKEN
        expires_at: "2025-12-31T23:59:59Z"
        scopes:
          - https://www.googleapis.com/auth/cloud-platform
```

#### Multiple API Keys (Load Balancing)

```yaml
providers:
  enabled:
    - cerebras

  cerebras:
    api_keys:
      - csk-KEY_1
      - csk-KEY_2
      - csk-KEY_3
    default_model: llama3.1-8b
    max_tokens: 131072
```

#### Custom OpenAI-Compatible Provider

```yaml
providers:
  enabled:
    - groq

  custom:
    groq:
      type: openai
      base_url: https://api.groq.com/openai/v1
      api_key: gsk_YOUR_GROQ_API_KEY
      default_model: llama-3.3-70b-versatile
      max_tokens: 8192
```

---

## Provider Selection

### How to Choose a Provider

Different providers excel at different tasks. Here's a guide to help you choose:

**For General Purpose Tasks:**
- **OpenAI GPT-4** - Best overall quality, excellent reasoning
- **Anthropic Claude 3.5 Sonnet** - Strong reasoning, large context window
- **Gemini 2.5 Pro** - Great for multimodal tasks

**For Fast Responses:**
- **Cerebras** - Extremely fast inference (10x+ faster)
- **OpenAI GPT-3.5** - Fast and cost-effective

**For Long Context:**
- **Anthropic Claude** - Up to 200K tokens context
- **Gemini 2.5 Pro** - Up to 1M tokens context

**For Code Generation:**
- **OpenAI GPT-4** - Excellent code quality
- **Qwen Coder** - Specialized for coding tasks
- **Anthropic Claude** - Good at explaining code

**For Cost Optimization:**
- **Local Models** (Ollama, LM Studio) - Free, runs on your hardware
- **Cerebras** - Fast and cost-effective cloud option

**For Privacy-Sensitive Applications:**
- **Local Providers** (Ollama, LM Studio, Llama.cpp) - Data never leaves your machine

### Provider Capabilities Comparison Table

| Provider | Streaming | Tool Calling | Max Context | Speed | Cost |
|----------|-----------|--------------|-------------|-------|------|
| **OpenAI** | ✅ | ✅ | 128K | Medium | $$$ |
| **Anthropic** | ✅ | ✅ | 200K | Medium | $$$ |
| **Gemini** | ✅ | ✅ | 1M | Medium | $$ |
| **Cerebras** | ✅ | ✅ | 128K | Very Fast | $ |
| **Qwen** | ✅ | ✅ | 32K | Fast | $$ |
| **OpenRouter** | ✅ | ✅ | Varies | Varies | Varies |
| **Local Models** | ✅ | ✅ | Varies | Depends on HW | Free |

**Authentication Methods:**

| Provider | API Key | OAuth 2.0 | Multi-Key Support |
|----------|---------|-----------|-------------------|
| **OpenAI** | ✅ | ❌ | ✅ |
| **Anthropic** | ✅ | ✅ | ✅ |
| **Gemini** | ❌ | ✅ | ✅ (OAuth) |
| **Cerebras** | ✅ | ❌ | ✅ |
| **Qwen** | ❌ | ✅ | ✅ (OAuth) |
| **OpenRouter** | ✅ | ❌ | ✅ |
| **Local Models** | ❌ | ❌ | N/A |

### Switching Between Providers

One of the key benefits of AI Provider Kit is easy provider switching:

#### Switching in Code

```go
// Start with OpenAI
provider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    DefaultModel: "gpt-4",
})

// Same code works with Anthropic
provider, _ = factory.CreateProvider(types.ProviderTypeAnthropic, types.ProviderConfig{
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    DefaultModel: "claude-sonnet-4-5",
})

// Or Gemini - exact same API!
provider, _ = factory.CreateProvider(types.ProviderTypeGemini, geminiConfig)

// Your application code doesn't change
stream, _ := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Hello, world!",
})
```

#### Switching via Configuration

```yaml
# config.yaml
providers:
  # Just change which provider is enabled
  enabled:
    - anthropic  # Was: openai

  openai:
    api_key: sk-...
    default_model: gpt-4

  anthropic:
    api_key: sk-ant-...
    default_model: claude-sonnet-4-5
```

No code changes needed - just update the configuration file!

#### Runtime Provider Selection

```go
// Let users choose provider at runtime
func createProviderFromUserChoice(choice string) (types.Provider, error) {
    var providerType types.ProviderType
    var apiKey string
    var model string

    switch choice {
    case "openai":
        providerType = types.ProviderTypeOpenAI
        apiKey = os.Getenv("OPENAI_API_KEY")
        model = "gpt-4"
    case "anthropic":
        providerType = types.ProviderTypeAnthropic
        apiKey = os.Getenv("ANTHROPIC_API_KEY")
        model = "claude-sonnet-4-5"
    case "cerebras":
        providerType = types.ProviderTypeCerebras
        apiKey = os.Getenv("CEREBRAS_API_KEY")
        model = "llama3.1-8b"
    default:
        return nil, fmt.Errorf("unknown provider: %s", choice)
    }

    return factory.CreateProvider(providerType, types.ProviderConfig{
        Type:         providerType,
        APIKey:       apiKey,
        DefaultModel: model,
    })
}
```

---

## Next Steps

### Advanced Guides

Now that you understand the basics, explore these advanced topics:

1. **[Phase 3 Comprehensive Guide](PHASE_3_COMPREHENSIVE_GUIDE.md)** - Learn about interface segregation and standardized core API for better architecture

2. **[OAuth Manager Documentation](OAUTH_MANAGER.md)** - Learn about multi-OAuth credential management with automatic failover and token refresh

3. **[Multi-Key Strategies](MULTI_KEY_STRATEGIES.md)** - Implement load balancing and failover across multiple API keys

4. **[Metrics Documentation](METRICS.md)** - Monitor provider performance and track usage at provider and credential levels

5. **[Tool Calling Guide](../TOOL_CALLING.md)** - Implement function calling with universal format across all providers

### Common Use Cases

#### Building a Chat Application

See the [demo-client example](../examples/demo-client/) for a complete interactive chat application with:
- Multiple provider support
- OAuth token refresh
- Metrics tracking
- Streaming responses

```bash
cd examples/demo-client
go run main.go -config config.yaml
```

#### Streaming Responses

See the [demo-client-streaming example](../examples/demo-client-streaming/) for real-time streaming:

```bash
cd examples/demo-client-streaming
go run main.go -config config.yaml
```

#### Model Discovery

See the [model-discovery-demo](../examples/model-discovery-demo/) to dynamically fetch and cache model lists:

```bash
cd examples/model-discovery-demo
go run main.go -config config.yaml
```

#### Tool/Function Calling

See the [tool-calling-demo](../examples/tool-calling-demo/) for implementing function calling:

```bash
cd examples/tool-calling-demo
go run main.go
```

### Community Resources

#### Documentation

- **[Go Package Documentation](https://pkg.go.dev/github.com/cecil-the-coder/ai-provider-kit)** - Complete API reference
- **[Examples Directory](../examples/)** - Working examples for all features
- **[Main README](../README.md)** - Project overview and quick reference

#### Development

- **[GitHub Repository](https://github.com/cecil-the-coder/ai-provider-kit)** - Source code and latest updates
- **[Issue Tracker](https://github.com/cecil-the-coder/ai-provider-kit/issues)** - Report bugs or request features
- **[Discussions](https://github.com/cecil-the-coder/ai-provider-kit/discussions)** - Ask questions and share ideas
- **[Changelog](../CHANGELOG.md)** - Version history and updates

#### Testing Your Integration

Run the test suite to ensure everything works:

```bash
# Run all tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific provider tests
go test ./pkg/providers/openai/...
go test ./pkg/providers/anthropic/...
```

#### Contributing

We welcome contributions! Here's how to get started:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes with tests
4. Run the test suite (`go test ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

See the [Contributing Guidelines](../README.md#contributing) for more details.

### Getting Help

If you need help:

1. **Check the Documentation** - Most questions are answered in the docs
2. **Review Examples** - The examples directory has working code for common scenarios
3. **Search Issues** - Someone may have already solved your problem
4. **Ask in Discussions** - Get help from the community
5. **Open an Issue** - Report bugs or request features

### What's Next?

You're now ready to build with AI Provider Kit! Here's a suggested learning path:

**Beginner Path:**
1. ✅ Complete this Getting Started guide
2. Run the [demo-client example](../examples/demo-client/)
3. Try different providers with your API keys
4. Experiment with streaming responses

**Intermediate Path:**
1. Set up YAML configuration for multiple providers
2. Implement error handling and metrics monitoring
3. Try tool calling with the [tool-calling-demo](../examples/tool-calling-demo/)
4. Explore OAuth authentication with [Gemini](../examples/gemini-oauth-flow/) or [Qwen](../examples/qwen-oauth-flow/)

**Advanced Path:**
1. Implement multi-key load balancing
2. Set up OAuth credential management with automatic refresh
3. Build a custom provider for an OpenAI-compatible API
4. Integrate metrics with your monitoring system

Happy coding! 🚀
