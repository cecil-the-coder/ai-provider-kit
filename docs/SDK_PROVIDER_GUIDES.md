# Provider Implementation Guides

Comprehensive guides for each supported AI provider in the AI Provider Kit SDK.

## Table of Contents

1. [OpenAI Provider Guide](#openai-provider-guide)
2. [Anthropic (Claude) Provider Guide](#anthropic-claude-provider-guide)
3. [Google Gemini Provider Guide](#google-gemini-provider-guide)
4. [Cerebras Provider Guide](#cerebras-provider-guide)
5. [Qwen Provider Guide](#qwen-provider-guide)
6. [OpenRouter Provider Guide](#openrouter-provider-guide)
7. [Custom Provider Support](#custom-provider-support)

---

## OpenAI Provider Guide

The OpenAI provider supports GPT models with native API access, including GPT-4o, GPT-4 Turbo, and GPT-3.5 Turbo.

### Supported Models

| Model ID | Display Name | Context Window | Tool Calling | Streaming |
|----------|-------------|----------------|--------------|-----------|
| gpt-4o | GPT-4o | 128K tokens | Yes | Yes |
| gpt-4o-mini | GPT-4o Mini | 128K tokens | Yes | Yes |
| gpt-4-turbo | GPT-4 Turbo | 128K tokens | Yes | Yes |
| gpt-4 | GPT-4 | 8K tokens | Yes | Yes |
| gpt-3.5-turbo | GPT-3.5 Turbo | 4K tokens | Yes | Yes |
| gpt-3.5-turbo-16k | GPT-3.5 Turbo 16K | 16K tokens | Yes | Yes |

### Quick Start Example

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
    // Create factory
    f := factory.NewProviderFactory()
    factory.RegisterDefaultProviders(f)

    // Configure OpenAI provider
    config := types.ProviderConfig{
        Type:         types.ProviderTypeOpenAI,
        APIKey:       "sk-your-api-key-here",
        DefaultModel: "gpt-4o",
        BaseURL:      "https://api.openai.com/v1", // Optional: custom endpoint
    }

    // Create provider
    provider, err := f.CreateProvider(types.ProviderTypeOpenAI, config)
    if err != nil {
        log.Fatal(err)
    }

    // Generate completion
    stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Prompt: "Explain quantum computing in simple terms",
        Model:  "gpt-4o",
        Stream: false,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()

    // Get response
    chunk, _ := stream.Next()
    fmt.Println(chunk.Content)
}
```

### Configuration Reference

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    Name:         "openai-primary",
    APIKey:       "sk-...",           // Single API key
    BaseURL:      "https://api.openai.com/v1",
    DefaultModel: "gpt-4o",

    // Multi-key support for load balancing
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{
            "sk-key1...",
            "sk-key2...",
            "sk-key3...",
        },
    },
}
```

### Authentication Setup

OpenAI supports both API key and OAuth 2.0 authentication:

#### API Key Authentication

```go
// Single API key
provider.Authenticate(context.Background(), types.AuthConfig{
    Method: types.AuthMethodAPIKey,
    APIKey: "sk-your-api-key",
})

// Multiple keys with automatic failover
config := types.ProviderConfig{
    Type: types.ProviderTypeOpenAI,
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{
            "sk-primary-key",
            "sk-backup-key-1",
            "sk-backup-key-2",
        },
    },
}
```

#### OAuth 2.0 Authentication (NEW)

```go
// OAuth configuration with PKCE
config := types.ProviderConfig{
    Type: types.ProviderTypeOpenAI,
    OAuthCredentials: []*types.OAuthCredentialSet{
        {
            ID:           "personal-account",
            ClientID:     "app_EMoamEEZ73f0CkXaXp7hrann", // Public client
            AccessToken:  "access-token...",
            RefreshToken: "refresh-token...",
            ExpiresAt:    time.Now().Add(24 * time.Hour),
            Scopes:       []string{"openid", "profile", "email", "offline_access"},
        },
    },
    ProviderConfig: map[string]interface{}{
        "oauth_client_id":   "app_EMoamEEZ73f0CkXaXp7hrann",
        "organization_id":   "org-xyz", // Optional
    },
}

// OAuth features:
// - Automatic token refresh (form-encoded)
// - Multi-OAuth account support with failover
// - Organization header support
// - PKCE with S256 security
```

### Rate Limiting Behavior

OpenAI provides comprehensive rate limit headers:

- **Headers tracked**: `x-ratelimit-limit-requests`, `x-ratelimit-remaining-requests`, `x-ratelimit-limit-tokens`, `x-ratelimit-remaining-tokens`
- **Reset tracking**: `x-ratelimit-reset-requests`, `x-ratelimit-reset-tokens`
- **Automatic retry**: SDK automatically waits when rate limits are hit

```go
// Check rate limits before making request
info, exists := provider.GetTrackedRateLimits("gpt-4o")
if exists {
    fmt.Printf("Requests remaining: %d/%d\n",
        info.RequestsRemaining, info.RequestsLimit)
    fmt.Printf("Tokens remaining: %d/%d\n",
        info.TokensRemaining, info.TokensLimit)
}
```

### Streaming Implementation

```go
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Write a short story",
    Model:  "gpt-4o",
    Stream: true, // Enable streaming
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

// Process stream chunks
for {
    chunk, err := stream.Next()
    if err != nil {
        break
    }

    if chunk.Done {
        fmt.Printf("\nTotal tokens used: %d\n", chunk.Usage.TotalTokens)
        break
    }

    fmt.Print(chunk.Content)
}
```

### Tool Calling Format

OpenAI uses the native tool calling format:

```go
tools := []types.Tool{
    {
        Name:        "get_weather",
        Description: "Get current weather for a location",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type":        "string",
                    "description": "City and state, e.g. San Francisco, CA",
                },
                "unit": map[string]interface{}{
                    "type": "string",
                    "enum": []string{"celsius", "fahrenheit"},
                },
            },
            "required": []string{"location"},
        },
    },
}

// Control tool usage
toolChoice := &types.ToolChoice{
    Mode: types.ToolChoiceRequired, // auto, required, none, or specific
}

stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt:     "What's the weather in Boston?",
    Tools:      tools,
    ToolChoice: toolChoice,
})
```

### Best Practices

1. **Use GPT-4o for complex tasks**: Best performance and quality
2. **Use GPT-4o-mini for simple tasks**: Cost-effective and fast
3. **Enable streaming for long responses**: Better UX with real-time feedback
4. **Implement retry logic**: Handle rate limits gracefully
5. **Monitor token usage**: Track costs with usage metrics
6. **Use multiple API keys**: Implement load balancing for high-volume applications

### Common Issues and Solutions

**Issue**: Rate limit exceeded (429 error)
```go
// Solution: Implement exponential backoff
for retries := 0; retries < 3; retries++ {
    stream, err := provider.GenerateChatCompletion(ctx, options)
    if err == nil {
        break
    }

    if strings.Contains(err.Error(), "rate limit") {
        waitTime := time.Duration(retries+1) * time.Second
        time.Sleep(waitTime)
        continue
    }

    return err
}
```

**Issue**: Context length exceeded
```go
// Solution: Monitor token usage and truncate if needed
if estimatedTokens > 128000 { // GPT-4o limit
    // Truncate input or split into multiple requests
}
```

### Migration from OpenAI SDK

```go
// Native OpenAI SDK
import "github.com/sashabaranov/go-openai"

client := openai.NewClient(apiKey)
resp, err := client.CreateChatCompletion(
    context.Background(),
    openai.ChatCompletionRequest{
        Model: openai.GPT4,
        Messages: []openai.ChatCompletionMessage{
            {Role: "user", Content: "Hello"},
        },
    },
)

// AI Provider Kit (unified interface)
provider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, config)
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Hello",
    Model:  "gpt-4",
})
```

---

## Anthropic (Claude) Provider Guide

Anthropic provider supports Claude models with dual authentication (API key and OAuth) and advanced features like multi-key failover.

### Claude Models Overview

| Model ID | Display Name | Context Window | Input/Output Tracking | Tool Calling |
|----------|-------------|----------------|----------------------|--------------|
| claude-3-5-sonnet-20241022 | Claude 3.5 Sonnet (Oct 2024) | 200K tokens | Yes | Yes |
| claude-3-5-haiku-20241022 | Claude 3.5 Haiku (Oct 2024) | 200K tokens | Yes | Yes |
| claude-3-opus-20240229 | Claude 3 Opus | 200K tokens | Yes | Yes |
| claude-3-sonnet-20240229 | Claude 3 Sonnet | 200K tokens | Yes | Yes |
| claude-3-haiku-20240307 | Claude 3 Haiku | 200K tokens | Yes | Yes |

### Quick Start Example

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeAnthropic,
    APIKey:       "sk-ant-...",
    DefaultModel: "claude-3-5-sonnet-20241022",
}

provider, err := factory.CreateProvider(types.ProviderTypeAnthropic, config)
if err != nil {
    log.Fatal(err)
}

stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Explain machine learning",
    Model:  "claude-3-5-sonnet-20241022",
})
```

### Dual Authentication Support

Anthropic supports both API key and OAuth authentication:

#### API Key Authentication

```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeAnthropic,
    APIKey: "sk-ant-api03-...",
}

// Or multiple API keys for failover
config := types.ProviderConfig{
    Type: types.ProviderTypeAnthropic,
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{
            "sk-ant-api03-key1...",
            "sk-ant-api03-key2...",
        },
    },
}
```

#### OAuth Authentication

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeAnthropic,
    OAuthCredentials: []types.OAuthCredentialSet{
        {
            ID:           "primary",
            ClientID:     "9d1c250a-e61b-44d9-88ed-5944d1962f5e", // MAX plan
            AccessToken:  "sk-ant-oat01-...",
            RefreshToken: "sk-ant-ort01-...",
            ExpiresAt:    time.Now().Add(24 * time.Hour),
        },
    },
    ProviderConfig: map[string]interface{}{
        "oauth_client_id": "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
    },
}
```

### Multi-OAuth Configuration

Support multiple OAuth accounts with automatic failover:

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeAnthropic,
    OAuthCredentials: []types.OAuthCredentialSet{
        {
            ID:           "account1",
            AccessToken:  "sk-ant-oat01-...",
            RefreshToken: "sk-ant-ort01-...",
            ExpiresAt:    time.Now().Add(24 * time.Hour),
        },
        {
            ID:           "account2",
            AccessToken:  "sk-ant-oat01-...",
            RefreshToken: "sk-ant-ort01-...",
            ExpiresAt:    time.Now().Add(24 * time.Hour),
        },
    },
}
```

### Input/Output Token Tracking

Anthropic provides separate tracking for input and output tokens:

```go
// Rate limit info includes separate input/output tracking
info, _ := provider.GetTrackedRateLimits("claude-3-5-sonnet-20241022")

fmt.Printf("Input tokens: %d/%d remaining\n",
    info.InputTokensRemaining, info.InputTokensLimit)
fmt.Printf("Output tokens: %d/%d remaining\n",
    info.OutputTokensRemaining, info.OutputTokensLimit)

// Usage information
chunk, _ := stream.Next()
fmt.Printf("Prompt tokens: %d\n", chunk.Usage.PromptTokens)
fmt.Printf("Completion tokens: %d\n", chunk.Usage.CompletionTokens)
```

### System Prompts for Claude Code

When using OAuth with Claude Code client ID, system prompts are automatically prepended:

```go
// OAuth requests automatically include:
systemPrompt := map[string]string{
    "type": "text",
    "text": "You are Claude Code, Anthropic's official CLI for Claude.",
}

// For API key requests, system prompts are configured manually
request := AnthropicRequest{
    System: "You are an expert programmer...",
    Messages: messages,
}
```

### Rate Limiting Specifics

Anthropic provides detailed rate limit headers:

```go
// Headers tracked:
// - anthropic-ratelimit-requests-limit
// - anthropic-ratelimit-requests-remaining
// - anthropic-ratelimit-requests-reset
// - anthropic-ratelimit-tokens-limit
// - anthropic-ratelimit-tokens-remaining
// - anthropic-ratelimit-tokens-reset
// - anthropic-ratelimit-input-tokens-limit
// - anthropic-ratelimit-input-tokens-remaining
// - anthropic-ratelimit-output-tokens-limit
// - anthropic-ratelimit-output-tokens-remaining

// Example: Check before making request
info, exists := tracker.Get("claude-3-5-sonnet-20241022")
if exists && !tracker.CanMakeRequest("claude-3-5-sonnet-20241022", 4000) {
    waitTime := tracker.GetWaitTime("claude-3-5-sonnet-20241022")
    time.Sleep(waitTime)
}
```

### Tool Calling with Content Blocks

Anthropic uses content blocks format for tool calling:

```go
tools := []types.Tool{
    {
        Name:        "calculate",
        Description: "Perform mathematical calculations",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "expression": map[string]interface{}{
                    "type":        "string",
                    "description": "Mathematical expression to evaluate",
                },
            },
            "required": []string{"expression"},
        },
    },
}

// Anthropic tool choice format
toolChoice := &types.ToolChoice{
    Mode: types.ToolChoiceRequired, // Converts to {"type": "any"}
}

stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "What is 25 * 4?"},
    },
    Tools:      tools,
    ToolChoice: toolChoice,
})

// Handle tool use responses
chunk, _ := stream.Next()
if len(chunk.Choices) > 0 && len(chunk.Choices[0].Message.ToolCalls) > 0 {
    for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
        fmt.Printf("Tool: %s\n", toolCall.Function.Name)
        fmt.Printf("Arguments: %s\n", toolCall.Function.Arguments)
    }
}
```

### Best Practices

1. **Use OAuth for production**: Better rate limits and features
2. **Leverage multi-key failover**: Ensure high availability
3. **Monitor separate input/output limits**: Anthropic tracks them independently
4. **Use streaming for long responses**: Better performance with large contexts
5. **Set appropriate max_tokens**: Optimize cost and latency
6. **Handle tool use blocks properly**: Anthropic's format differs from OpenAI

### Common Issues and Solutions

**Issue**: OAuth token expired
```go
// Solution: SDK automatically refreshes tokens
// Tokens are persisted to config file after refresh
```

**Issue**: Content blocks parsing
```go
// Solution: SDK handles conversion automatically
// Access both text and tool_use blocks through unified interface
```

---

## Google Gemini Provider Guide

Google Gemini provider supports large context windows (up to 2M tokens) and OAuth integration with Google Cloud.

### Model Variants and Capabilities

| Model ID | Display Name | Context Window | Special Features |
|----------|-------------|----------------|------------------|
| gemini-2.0-flash-exp | Gemini 2.0 Flash (Experimental) | 8K tokens | Latest experimental model |
| gemini-1.5-pro | Gemini 1.5 Pro | 2M tokens | Largest context window |
| gemini-1.5-flash | Gemini 1.5 Flash | 1M tokens | Fast and efficient |
| gemini-1.0-pro | Gemini 1.0 Pro | 32K tokens | Stable production model |
| gemini-pro-vision | Gemini Pro Vision | 16K tokens | Multimodal support |

### Quick Start Example

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeGemini,
    APIKey:       "AIza...",
    DefaultModel: "gemini-2.0-flash-exp",
    ProviderConfig: map[string]interface{}{
        "project_id": "my-gcp-project", // For CloudCode API
    },
}

provider, err := factory.CreateProvider(types.ProviderTypeGemini, config)
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Summarize this document",
    Model:  "gemini-1.5-pro",
})
```

### Authentication Setup

Gemini supports both OAuth 2.0 and API key authentication:

#### OAuth Setup with Google Cloud (Recommended)

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeGemini,
    OAuthCredentials: []types.OAuthCredentialSet{
        {
            ID:           "default",
            ClientID:     "your-client-id.apps.googleusercontent.com",
            ClientSecret: "GOCSPX-...",
            AccessToken:  "ya29.a0...",
            RefreshToken: "1//06...",
            ExpiresAt:    time.Now().Add(time.Hour),
            Scopes:       []string{"https://www.googleapis.com/auth/cloud-platform"},
        },
    },
    ProviderConfig: map[string]interface{}{
        "project_id": "my-gcp-project",
    },
}
```

#### API Key Authentication (NEW - Simpler Setup)

```go
// Single API key
config := types.ProviderConfig{
    Type:   types.ProviderTypeGemini,
    APIKey: "AIza...",
    Model:  "gemini-2.0-flash-exp",
}

// Multi-key support with automatic failover
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

// API Key features:
// - Simpler than OAuth setup
// - No project ID required
// - Multi-key failover support
// - Uses standard Gemini API endpoint
// - 100 requests/day free tier
```

### CloudCode API vs Standard API

The SDK supports both APIs with automatic detection:

#### Standard Gemini API (API Key)

```go
// Uses: https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?key={apiKey}
config := types.ProviderConfig{
    Type:   types.ProviderTypeGemini,
    APIKey: "AIza...",
}
```

#### CloudCode API (OAuth)

```go
// Uses: https://cloudcode-pa.googleapis.com/v1internal/:generateContent
// Requires: OAuth token and project_id
config := types.ProviderConfig{
    Type: types.ProviderTypeGemini,
    OAuthCredentials: []types.OAuthCredentialSet{ /* ... */ },
    ProviderConfig: map[string]interface{}{
        "project_id": "my-gcp-project",
    },
}
```

### Client-Side Rate Limiting

Gemini doesn't provide rate limit headers, so SDK uses client-side token bucket:

```go
// Default: 15 requests per minute (free tier)
// Update for your tier:
provider.UpdateRateLimitTier(360) // Pay-as-you-go: 360 RPM

// Rate limits enforced before request
// Automatic waiting when limit reached
```

### Safety Filtering

Gemini includes safety filters that may block content:

```go
// Check finish reason
chunk, _ := stream.Next()
if chunk.Choices[0].FinishReason == "SAFETY" {
    fmt.Println("Content filtered due to safety concerns")
}

// Safety settings are configured server-side
// Adjust in Google Cloud Console if needed
```

### Large Context Windows (2M tokens)

Take advantage of Gemini's massive context:

```go
// Load large documents
largeDocument := readFile("book.txt") // e.g., 500K tokens

stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Summarize the key themes in this book:\n\n" + largeDocument,
    Model:  "gemini-1.5-pro", // 2M token context
})

// Pro tip: Split very large documents into chunks
// and use context caching (when available)
```

### Best Practices

1. **Use gemini-1.5-pro for large documents**: Take advantage of 2M token context
2. **Use gemini-1.5-flash for fast responses**: Optimal balance of speed and quality
3. **Configure client-side rate limits**: Match your API tier
4. **Handle safety filtering**: Check finish reasons
5. **Leverage multimodal capabilities**: Use gemini-pro-vision for image analysis

### Common Issues and Solutions

**Issue**: Project ID not found
```go
// Solution: Set project_id in config
config.ProviderConfig["project_id"] = "your-gcp-project-id"

// Or onboard automatically (SDK attempts this)
```

**Issue**: Content filtered by safety
```go
// Solution: Rephrase prompt or adjust safety settings in Google Cloud Console
```

---

## Cerebras Provider Guide

Cerebras provides ultra-fast inference with extended timeouts and dual rate limiting (per-minute and daily).

### Ultra-Fast Inference Features

- **Speed**: 10-100x faster than traditional inference
- **Models**: Llama-based models optimized for Cerebras hardware
- **Extended timeouts**: 120-second default (vs 60s for other providers)
- **OpenAI compatibility**: Drop-in replacement for OpenAI API

### Model Selection

| Model ID | Display Name | Context Window | Performance |
|----------|-------------|----------------|-------------|
| zai-glm-4.6 | ZAI GLM-4.6 | 131K tokens | Ultra-fast code generation |
| llama3.1-8b | Llama 3.1 8B | 8K tokens | Fast general purpose |
| llama3.1-70b | Llama 3.1 70B | 8K tokens | High quality analysis |

### Quick Start Example

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeCerebras,
    APIKey:       "csk-...",
    DefaultModel: "zai-glm-4.6",
    ProviderConfig: map[string]interface{}{
        "max_tokens":  131072,
        "temperature": 0.6,
    },
}

provider, err := factory.CreateProvider(types.ProviderTypeCerebras, config)
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Write a Python function to sort a list",
    Model:  "zai-glm-4.6",
    Stream: true, // Real-time streaming
})
```

### Daily and Per-Minute Rate Limits

Cerebras tracks two types of rate limits:

```go
// Headers tracked:
// - x-ratelimit-limit-requests (per-minute)
// - x-ratelimit-remaining-requests (per-minute)
// - x-ratelimit-limit-requests-daily (per-day)
// - x-ratelimit-remaining-requests-daily (per-day)

// SDK automatically tracks both limits
info, _ := tracker.Get("zai-glm-4.6")

fmt.Printf("Per-minute: %d/%d remaining\n",
    info.RequestsRemaining, info.RequestsLimit)
fmt.Printf("Daily: %d/%d remaining\n",
    info.DailyRequestsRemaining, info.DailyRequestsLimit)

// Automatic waiting when either limit is hit
```

### Extended Timeouts

```go
// Default timeout: 120 seconds (vs 60s for other providers)
client := &http.Client{
    Timeout: 120 * time.Second,
}

// For very long generations, increase further:
config := types.ProviderConfig{
    Type:   types.ProviderTypeCerebras,
    APIKey: "csk-...",
    // Custom timeout handled by HTTP client
}
```

### OpenAI Compatibility Mode

Cerebras is OpenAI-compatible, so you can use it as a drop-in replacement:

```go
// Option 1: Use Cerebras provider directly
provider, _ := factory.CreateProvider(types.ProviderTypeCerebras, config)

// Option 2: Use as custom OpenAI-compatible provider
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    BaseURL:      "https://api.cerebras.ai/v1",
    APIKey:       "csk-...",
    DefaultModel: "zai-glm-4.6",
}
```

### Multi-Key Configuration

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeCerebras,
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{
            "csk-key1...",
            "csk-key2...",
        },
    },
}

// SDK automatically uses failover when rate limits hit
```

### Best Practices

1. **Use streaming for immediate feedback**: Cerebras is fast, show results in real-time
2. **Monitor both rate limits**: Track per-minute and daily limits
3. **Use multi-key setup for high volume**: Distribute load across keys
4. **Optimize for speed**: Cerebras excels at fast generation
5. **Use appropriate models**: zai-glm-4.6 for code, llama3.1-70b for analysis

### Common Issues and Solutions

**Issue**: Daily limit exceeded
```go
// Solution: Use multiple API keys or upgrade plan
config.ProviderConfig["api_keys"] = []string{"csk-1", "csk-2"}
```

**Issue**: Timeout on very long generations
```go
// Solution: Increase timeout
client := &http.Client{
    Timeout: 180 * time.Second,
}
```

---

## Qwen Provider Guide

Qwen (Alibaba Cloud) provider supports device code OAuth flow and client-side rate limiting with code generation models.

### Quick Start Example

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeQwen,
    BaseURL:      "https://portal.qwen.ai/v1",
    DefaultModel: "qwen-turbo",
}

// Add OAuth credentials
config.OAuthCredentials = []types.OAuthCredentialSet{
    {
        ID:           "default",
        ClientID:     "f0304373b74a44d2b584a3fb70ca9e56",
        AccessToken:  "...",
        RefreshToken: "...",
        Scopes:       []string{"model.completion"},
    },
}

provider, err := factory.CreateProvider(types.ProviderTypeQwen, config)
```

### Alibaba Cloud Integration

Qwen is integrated with Alibaba Cloud services:

```go
// Base URL options:
// - Portal API: https://portal.qwen.ai/v1
// - DashScope API: https://dashscope.aliyuncs.com/api/v1

config := types.ProviderConfig{
    Type:    types.ProviderTypeQwen,
    BaseURL: "https://portal.qwen.ai/v1", // Portal API recommended
}
```

### Device Code OAuth Flow

Qwen uses device code flow for OAuth:

```go
// 1. Initiate device code flow
// (handled by SDK during authentication)

// 2. User visits URL and enters code

// 3. SDK polls for token

// 4. Tokens are automatically persisted
config := types.ProviderConfig{
    Type: types.ProviderTypeQwen,
    OAuthCredentials: []types.OAuthCredentialSet{
        {
            ID:           "default",
            ClientID:     "f0304373b74a44d2b584a3fb70ca9e56", // Public client
            ClientSecret: "", // Not required for device flow
            AccessToken:  "token-from-device-flow",
            RefreshToken: "refresh-token",
        },
    },
}
```

### Client-Side Rate Limiting Needs

Qwen doesn't provide rate limit headers, so SDK uses client-side enforcement:

```go
// Default: 60 requests/minute, 2000/day (free tier)
// Token bucket algorithm enforces limits

// Rate limiting is automatic
// Requests wait if bucket is empty

// Logging shows when rate limiting occurs:
// "Qwen: Rate limit wait..."
```

### Code Generation Models

| Model ID | Display Name | Context Window | Specialization |
|----------|-------------|----------------|----------------|
| qwen-turbo | Qwen Turbo | 8K tokens | General purpose |
| qwen-plus | Qwen Plus | 32K tokens | Enhanced capabilities |
| qwen-max | Qwen Max | 8K tokens | Most capable |
| qwen-max-longcontext | Qwen Max Long | 32K tokens | Extended context |
| qwen-coder-turbo | Qwen Coder Turbo | 8K tokens | Fast code generation |
| qwen-coder-plus | Qwen Coder Plus | 32K tokens | Advanced coding |

### Configuration Persistence

```go
// OAuth tokens are automatically persisted to:
// ~/.mcp-code-api/config.yaml

// Manual persistence:
err := provider.persistTokenToConfig(
    "qwen",
    accessToken,
    refreshToken,
    expiresAt,
)
```

### Best Practices

1. **Use qwen-coder models for programming**: Specialized for code generation
2. **Monitor client-side rate limits**: SDK enforces 60 RPM / 2000 daily
3. **Persist OAuth tokens**: Avoid re-authentication
4. **Use appropriate model for task**: qwen-turbo for speed, qwen-max for quality
5. **Handle device flow properly**: Guide users through OAuth process

### Common Issues and Solutions

**Issue**: OAuth token expired
```go
// Solution: SDK automatically refreshes
// Check token status:
if !provider.IsAuthenticated() {
    // Re-authenticate
}
```

**Issue**: Rate limit hit
```go
// Solution: SDK automatically waits
// Or use multiple OAuth accounts:
config.OAuthCredentials = []types.OAuthCredentialSet{
    { /* account 1 */ },
    { /* account 2 */ },
}
```

---

## OpenRouter Provider Guide

OpenRouter is a universal gateway providing access to all major AI models through a single API.

### Universal Gateway Concept

OpenRouter routes requests to multiple providers:

- **Access all models**: OpenAI, Anthropic, Google, Meta, and more
- **Unified API**: Single endpoint for all providers
- **Dynamic routing**: Automatic failover between providers
- **Cost optimization**: Choose between free and paid models

### Quick Start Example

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenRouter,
    APIKey:       "sk-or-...",
    DefaultModel: "anthropic/claude-3.5-sonnet",
    ProviderConfig: map[string]interface{}{
        "site_url":  "https://yourapp.com",
        "site_name": "Your App Name",
    },
}

provider, err := factory.CreateProvider(types.ProviderTypeOpenRouter, config)
```

### Authentication Setup

OpenRouter supports both API key and OAuth PKCE authentication:

#### API Key Authentication

```go
config := types.ProviderConfig{
    Type:    types.ProviderTypeOpenRouter,
    APIKey:  "sk-or-v1-...",
}
```

#### OAuth PKCE Flow (NEW)

```go
// Step 1: Configure OAuth callback
config := types.ProviderConfig{
    Type: types.ProviderTypeOpenRouter,
    ProviderConfig: map[string]interface{}{
        "oauth_callback_url": "https://yourapp.com/oauth/callback",
    },
}

provider, _ := factory.CreateProvider(types.ProviderTypeOpenRouter, config)

// Step 2: Start OAuth flow
authURL, flowState, err := provider.StartOAuthFlow(ctx, "")
// User visits authURL and authorizes at openrouter.ai/auth

// Step 3: Handle callback
apiKey, err := provider.HandleOAuthCallback(ctx, authCode, flowState)
// OAuth-obtained API key is automatically added to the key pool

// OAuth features:
// - PKCE with S256 security (no client_id/secret needed)
// - Returns permanent API keys (not temporary tokens)
// - Keys auto-added to failover pool
// - User-controlled spending limits
// - No token refresh needed (keys are permanent)
```

### Model Selection Strategies

OpenRouter supports multiple model selection strategies:

```go
// Failover strategy (default)
config := types.ProviderConfig{
    Type: types.ProviderTypeOpenRouter,
    ProviderConfig: map[string]interface{}{
        "models": []string{
            "anthropic/claude-3.5-sonnet",
            "openai/gpt-4o",
            "google/gemini-pro-1.5",
        },
        "model_strategy": "failover", // Try models in order
    },
}

// Load balancing strategy
config.ProviderConfig["model_strategy"] = "round_robin"

// Cost optimization
config.ProviderConfig["model_strategy"] = "cheapest_first"
```

### Free Tier vs Paid Models

```go
// Free models only
config := types.ProviderConfig{
    Type: types.ProviderTypeOpenRouter,
    ProviderConfig: map[string]interface{}{
        "free_only": true,
        "models": []string{
            "deepseek/deepseek-coder-v3.1:free",
            "meta-llama/llama-3.1-8b-instruct:free",
        },
    },
}

// Paid models (better performance)
config.ProviderConfig["free_only"] = false
config.ProviderConfig["models"] = []string{
    "anthropic/claude-3.5-sonnet",
    "openai/gpt-4o",
}
```

### Credit System

OpenRouter uses a credit-based system:

```go
// Check credit balance
rateLimits, err := provider.GetRateLimits(ctx)
if err == nil {
    fmt.Printf("Credits remaining: %.2f\n", *rateLimits.LimitRemaining)
    fmt.Printf("Is free tier: %v\n", rateLimits.IsFreeTier)
}

// Monitor credit usage
metrics := provider.GetMetrics()
fmt.Printf("Total tokens used: %d\n", metrics.TokensUsed)
```

### Required Headers

OpenRouter requires specific headers:

```go
// Automatically set by SDK:
// - Authorization: Bearer {api_key}
// - HTTP-Referer: {site_url}
// - X-Title: {site_name}

// Custom configuration:
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenRouter,
    APIKey: "sk-or-...",
    ProviderConfig: map[string]interface{}{
        "site_url":  "https://yourapp.com",
        "site_name": "Your Application",
    },
}
```

### Dynamic Model Discovery

```go
// Get available models
models, err := provider.GetModels(ctx)
if err != nil {
    log.Fatal(err)
}

// Models include pricing information
for _, model := range models {
    fmt.Printf("Model: %s\n", model.ID)
    fmt.Printf("  Context: %d tokens\n", model.MaxTokens)
    fmt.Printf("  Description: %s\n", model.Description)
}

// Popular models:
// - anthropic/claude-3.5-sonnet
// - openai/gpt-4o
// - google/gemini-pro-1.5
// - meta-llama/llama-3.1-70b-instruct
// - qwen/qwen3-coder
```

### Best Practices

1. **Use failover for reliability**: Configure multiple models
2. **Monitor credit usage**: Track costs with metrics
3. **Set appropriate site info**: Required for attribution
4. **Choose models based on task**: Balance cost and quality
5. **Handle rate limits**: OpenRouter has per-model limits

### Common Issues and Solutions

**Issue**: Credit limit exceeded
```go
// Solution: Add more credits or use free models
if rateLimits.IsFreeTier && *rateLimits.LimitRemaining <= 0 {
    // Switch to free models or add credits
}
```

**Issue**: Model not available
```go
// Solution: Use fallback models
config.ProviderConfig["models"] = []string{
    "primary-model",
    "fallback-model-1",
    "fallback-model-2",
}
```

---

## Implementing Custom Providers with Phase 2 Utilities

The SDK provides comprehensive shared utilities (Phase 2) that dramatically simplify custom provider implementation. This section shows how to create new providers using all the shared utilities for consistent behavior with minimal code.

### Complete Provider with Phase 2 Shared Utilities

Here's a complete, production-ready custom provider using all Phase 2 utilities:

```go
package myprovider

import (
    "context"
    "fmt"
    "net/http"
    "sync"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type MyProvider struct {
    // Phase 2 utilities - all the heavy lifting is done by these helpers
    authHelper   *common.AuthHelper      // Handles API keys, OAuth, and failover
    configHelper *common.ConfigHelper    // Handles validation, defaults, and extraction
    customizer   common.ProviderCustomizer // Handles prompt formatting

    // HTTP client with connection pooling
    client *http.Client

    // Basic provider info
    config types.ProviderConfig
    mutex  sync.RWMutex
}

func NewMyProvider(config types.ProviderConfig) (*MyProvider, error) {
    // 1. Setup configuration helper for validation and defaults
    configHelper := common.NewConfigHelper("myprovider", types.ProviderTypeMyProvider)

    // Validate configuration
    validation := configHelper.ValidateProviderConfig(config)
    if !validation.Valid {
        return nil, fmt.Errorf("invalid configuration: %v", validation.Errors)
    }

    // Create optimized HTTP client
    client := &http.Client{
        Timeout: configHelper.ExtractTimeout(config),
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 10,
            IdleConnTimeout:     90 * time.Second,
            DisableCompression:  false,
        },
    }

    // 2. Setup authentication helper for API keys and OAuth
    authHelper := common.NewAuthHelper("myprovider", config, client)
    authHelper.SetupAPIKeys()

    // Setup OAuth if configured
    if len(config.OAuthCredentials) > 0 {
        factory := common.NewRefreshFuncFactory("myprovider", client)
        refreshFunc := factory.CreateGenericRefreshFunc("https://api.myprovider.com/oauth/token")
        authHelper.SetupOAuth(refreshFunc)
    }

    // 3. Create provider customizer for prompt formatting
    customizer := &MyProviderCustomizer{}

    return &MyProvider{
        authHelper:   authHelper,
        configHelper: configHelper,
        customizer:   customizer,
        client:       client,
        config:       config,
    }, nil
}
```

### Provider Customizer Implementation

```go
// Custom provider-specific prompt customizations
type MyProviderCustomizer struct {
    *common.DefaultCustomizer
}

func NewMyProviderCustomizer() *MyProviderCustomizer {
    return &MyProviderCustomizer{
        DefaultCustomizer: common.NewDefaultCustomizer(true, common.FormatMessageArray),
    }
}

func (c *MyProviderCustomizer) CustomizeSystemMessage(base string) string {
    // Add provider-specific system message enhancements
    return base + "\n\nYou are responding via MyProvider API. Format responses clearly and concisely."
}

func (c *MyProviderCustomizer) CustomizeUserMessage(base string) string {
    // Add provider-specific user message formatting
    if len(base) > 10000 {
        // For long prompts, add a summary
        return fmt.Sprintf("[LONG PROMPT] %s\n\nPlease provide a comprehensive response.", base[:5000])
    }
    return base
}

func (c *MyProviderCustomizer) SupportsSystemMessages() bool {
    return true // MyProvider supports system messages
}

func (c *MyProviderCustomizer) GetMessageFormat() common.MessageFormat {
    return common.FormatMessageArray // Use structured messages
}
```

### Required Interface Implementation (Simplified)

```go
func (p *MyProvider) Name() string {
    return "myprovider"
}

func (p *MyProvider) Type() types.ProviderType {
    return types.ProviderTypeMyProvider
}

func (p *MyProvider) Description() string {
    return "My Custom AI Provider with Phase 2 utilities"
}

func (p *MyProvider) GetDefaultModel() string {
    return p.configHelper.ExtractDefaultModel(p.config)
}

func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    // Use static models list for simplicity
    return []types.Model{
        {
            ID:                 "my-model-v1",
            Name:               "My Model v1",
            Provider:           p.Type(),
            MaxTokens:          4096,
            SupportsStreaming:  true,
            SupportsToolCalling: true,
        },
        {
            ID:                 "my-model-v2",
            Name:               "My Model v2",
            Provider:           p.Type(),
            MaxTokens:          8192,
            SupportsStreaming:  true,
            SupportsToolCalling: true,
        },
    }, nil
}
```

### Chat Completion with All Phase 2 Utilities

```go
func (p *MyProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // 1. Direct prompt handling - no prompt builder abstraction
    // Applications construct prompts directly
    finalOptions := options

    // 2. Handle streaming vs non-streaming
    if options.Stream {
        return p.handleStreamingRequest(ctx, finalOptions)
    }

    return p.handleNonStreamingRequest(ctx, finalOptions)
}

func (p *MyProvider) handleStreamingRequest(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Execute with authentication failover using auth helper
    _, _, err := p.authHelper.ExecuteWithAuth(ctx, options,
        // OAuth operation
        func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
            return p.makeStreamingRequest(ctx, "oauth", cred.AccessToken, options)
        },
        // API key operation
        func(ctx context.Context, key string) (string, *types.Usage, error) {
            return p.makeStreamingRequest(ctx, "api_key", key, options)
        },
    )

    if err != nil {
        return nil, err
    }

    // Create mock stream for this example
    return common.NewMockStream([]types.ChatCompletionChunk{
        {Content: "Hello from MyProvider!", Done: false},
        {Content: " This is Phase 2.", Done: false},
        {Content: "", Done: true, Usage: types.Usage{TotalTokens: 15}},
    }), nil
}

func (p *MyProvider) handleNonStreamingRequest(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Execute with authentication failover
    result, usage, err := p.authHelper.ExecuteWithAuth(ctx, options,
        p.executeWithOAuth,
        p.executeWithAPIKey,
    )

    if err != nil {
        return nil, err
    }

    // Return mock stream with single response
    return common.NewMockStream([]types.ChatCompletionChunk{
        {Content: result, Done: true, Usage: *usage},
    }), nil
}

func (p *MyProvider) executeWithOAuth(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
    resp, err := p.makeStreamingRequest(ctx, "oauth", cred.AccessToken, types.GenerateOptions{})
    if err != nil {
        return "", nil, err
    }
    return resp, &types.Usage{TotalTokens: 20}, nil
}

func (p *MyProvider) executeWithAPIKey(ctx context.Context, key string) (string, *types.Usage, error) {
    resp, err := p.makeStreamingRequest(ctx, "api_key", key, types.GenerateOptions{})
    if err != nil {
        return "", nil, err
    }
    return resp, &types.Usage{TotalTokens: 20}, nil
}
```

### HTTP Request Handling with Phase 2 Utilities

```go
func (p *MyProvider) makeStreamingRequest(ctx context.Context, authType, authToken string, options types.GenerateOptions) (string, error) {
    // 1. Build endpoint URL using config helper
    baseURL := p.configHelper.ExtractBaseURL(p.config)
    endpoint := baseURL + "/v1/chat/completions"

    // 2. Create request with context
    req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
    if err != nil {
        return "", fmt.Errorf("failed to create request: %w", err)
    }

    // 3. Set authentication headers using auth helper
    p.authHelper.SetAuthHeaders(req, authToken, authType)

    // 4. Set provider-specific headers using auth helper
    p.authHelper.SetProviderSpecificHeaders(req)

    // 5. Set common headers
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")

    // 6. Make request (simplified for example)
    resp, err := p.client.Do(req)
    if err != nil {
        return "", fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    // 7. Handle authentication errors with helper
    if resp.StatusCode >= 400 {
        return "", p.authHelper.HandleAuthError(fmt.Errorf("API error: %d", resp.StatusCode), resp.StatusCode)
    }

    // 8. Return response (simplified)
    return "Response from MyProvider", nil
}
```

### Health Check with Utilities

```go
func (p *MyProvider) HealthCheck(ctx context.Context) error {
    // 1. Check authentication status using auth helper
    status := p.authHelper.GetAuthStatus()
    if !status["authenticated"].(bool) {
        return fmt.Errorf("provider not authenticated: %s", status["method"])
    }

    // 2. Make health check request
    baseURL := p.configHelper.ExtractBaseURL(p.config)
    req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health", nil)
    if err != nil {
        return fmt.Errorf("failed to create health check request: %w", err)
    }

    // 3. Set provider-specific headers
    p.authHelper.SetProviderSpecificHeaders(req)

    resp, err := p.client.Do(req)
    if err != nil {
        return fmt.Errorf("health check request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("health check failed: status %d", resp.StatusCode)
    }

    return nil
}
```

### Configuration and Registration

```go
// Factory function for easy provider creation
func CreateMyProvider(config types.ProviderConfig) (types.Provider, error) {
    return NewMyProvider(config)
}

// Register with factory
func RegisterWithFactory(f *factory.ProviderFactory) {
    f.RegisterProvider(types.ProviderTypeMyProvider, CreateMyProvider)
}

// Example configuration
func ExampleConfig() types.ProviderConfig {
    return types.ProviderConfig{
        Type:         types.ProviderTypeMyProvider,
        DefaultModel: "my-model-v2",
        Timeout:      60 * time.Second,
        MaxTokens:    8192,

        // Single API key
        APIKey: "my-provider-key",

        // Or multiple API keys with failover
        ProviderConfig: map[string]interface{}{
            "api_keys": []string{
                "key1", "key2", "key3",
            },
        },

        // Or OAuth configuration
        OAuthCredentials: []*types.OAuthCredentialSet{
            {
                ID:           "primary",
                ClientID:     "my-client-id",
                ClientSecret: "my-client-secret",
                AccessToken:  "access-token",
                RefreshToken: "refresh-token",
                ExpiresAt:    time.Now().Add(24 * time.Hour),
            },
        },
    }
}
```

### Benefits of Phase 2 Utilities

**Dramatically Reduced Code:**
- **Authentication**: 0 lines of OAuth/token management code
- **Configuration**: 0 lines of validation/defaults code
- **Prompt Building**: 0 lines of prompt construction code
- **Streaming**: 0 lines of stream parsing code
- **Error Handling**: 0 lines of auth error handling code

**Production-Ready Features:**
- Automatic OAuth token refresh
- Multi-key failover with rotation
- Provider-specific prompt formatting
- Configuration validation and sanitization
- Safe logging without sensitive data
- Rate limiting integration ready
- Health checks with auth validation

**Consistency:**
- All providers behave identically
- Same error handling patterns
- Same configuration structure
- Same streaming interface

### Testing Your Phase 2 Provider

```go
func TestMyProviderPhase2(t *testing.T) {
    config := types.ProviderConfig{
        Type:         types.ProviderTypeMyProvider,
        APIKey:       "test-key",
        DefaultModel: "my-model-v1",
    }

    provider, err := NewMyProvider(config)
    require.NoError(t, err)

    // Test configuration helper
    assert.Equal(t, "my-model-v1", provider.configHelper.ExtractDefaultModel(config))
    assert.Equal(t, 60*time.Second, provider.configHelper.ExtractTimeout(config))

    // Test authentication helper
    status := provider.authHelper.GetAuthStatus()
    assert.True(t, status["authenticated"].(bool))
    assert.Equal(t, "api_key", status["method"])

    // Test direct prompt handling (no prompt builder abstraction)
    options := types.GenerateOptions{
        Prompt: "Test prompt",
        ContextFiles: []string{"test.go"},
    }

    // Applications construct prompts directly
    assert.NotEmpty(t, options.Prompt)

    // Test generation
    ctx := context.Background()
    stream, err := provider.GenerateChatCompletion(ctx, options)
    require.NoError(t, err)
    defer stream.Close()

    chunk, err := stream.Next()
    require.NoError(t, err)
    assert.NotEmpty(t, chunk.Content)
}
```

This complete example shows how Phase 2 utilities enable creating full-featured providers with minimal code while maintaining all production features.

### Implementing Required Interface Methods

#### Basic Provider Information

```go
func (p *MyProvider) Name() string {
    return "myprovider"
}

func (p *MyProvider) Type() types.ProviderType {
    return types.ProviderTypeMyProvider
}

func (p *MyProvider) Description() string {
    return "My Custom AI Provider"
}

func (p *MyProvider) GetDefaultModel() string {
    return p.config.DefaultModel
}
```

#### Model Management with Caching

Use the shared model cache for efficient model list management:

```go
func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        // Fetch fresh models from API
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx)
        },
        // Fallback to static list if API fails
        func() []types.Model {
            return []types.Model{
                {
                    ID:                "my-model-v1",
                    Name:              "My Model v1",
                    MaxTokens:         4096,
                    SupportsStreaming: true,
                    SupportsToolCalling: true,
                },
                {
                    ID:                "my-model-v2",
                    Name:              "My Model v2",
                    MaxTokens:         8192,
                    SupportsStreaming: true,
                    SupportsToolCalling: true,
                },
            }
        },
    )
}

func (p *MyProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
    // Make API call to get models
    resp, err := p.client.Get(p.buildURL("/models"))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // Parse response and convert to []types.Model
    var apiResponse struct {
        Data []struct {
            ID           string `json:"id"`
            Object       string `json:"object"`
            Created      int64  `json:"created"`
            OwnedBy      string `json:"owned_by"`
        } `json:"data"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
        return nil, err
    }

    models := make([]types.Model, len(apiResponse.Data))
    for i, model := range apiResponse.Data {
        models[i] = types.Model{
            ID:                 model.ID,
            Name:               model.ID,
            Provider:           p.Type(),
            SupportsStreaming:  true,
            SupportsToolCalling: true,
            MaxTokens:          inferTokenLimit(model.ID),
        }
    }

    return models, nil
}
```

#### Rate Limiting Integration

Use the shared rate limit helper for consistent rate limiting:

```go
func (p *MyProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Estimate tokens for rate limiting
    estimatedTokens := p.estimateTokens(options)

    // Check rate limits and wait if necessary
    if !p.rateLimitHelper.CanMakeRequest(options.Model, estimatedTokens) {
        p.rateLimitHelper.CheckRateLimitAndWait(options.Model, estimatedTokens)
    }

    // Build and make request
    req, err := p.buildRequest(options)
    if err != nil {
        return nil, err
    }

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }

    // Update rate limits from response headers
    p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, options.Model)

    // Process response
    if options.Stream {
        return p.handleStreamingResponse(resp)
    }
    return p.handleNonStreamingResponse(resp)
}
```

#### Custom Rate Limit Parser

Implement a provider-specific rate limit parser:

```go
type MyProviderParser struct{}

func (p *MyProviderParser) ProviderName() string {
    return "MyProvider"
}

func (p *MyProviderParser) Parse(headers http.Header, model string) (*ratelimit.Info, error) {
    info := &ratelimit.Info{
        Model: model,
    }

    // Parse standard rate limit headers
    if limit := headers.Get("x-ratelimit-limit-requests"); limit != "" {
        info.RequestsLimit, _ = strconv.Atoi(limit)
    }
    if remaining := headers.Get("x-ratelimit-remaining-requests"); remaining != "" {
        info.RequestsRemaining, _ = strconv.Atoi(remaining)
    }
    if reset := headers.Get("x-ratelimit-reset-requests"); reset != "" {
        if timestamp, err := strconv.ParseInt(reset, 10, 64); err == nil {
            info.RequestsReset = time.Unix(timestamp, 0)
        }
    }

    // Parse token limits
    if tokenLimit := headers.Get("x-ratelimit-limit-tokens"); tokenLimit != "" {
        info.TokensLimit, _ = strconv.Atoi(tokenLimit)
    }
    if tokenRemaining := headers.Get("x-ratelimit-remaining-tokens"); tokenRemaining != "" {
        info.TokensRemaining, _ = strconv.Atoi(tokenRemaining)
    }

    return info, nil
}
```

#### File Processing with Language Detection

Use shared file utilities for context file processing:

```go
func (p *MyProvider) processContextFiles(options *types.GenerateOptions) error {
    if len(options.ContextFiles) == 0 {
        return nil
    }

    // Filter out output file from context
    filteredFiles := common.FilterContextFiles(options.ContextFiles, options.OutputFile)

    var contextBuilder strings.Builder
    for _, file := range filteredFiles {
        // Detect language for potential syntax highlighting
        language := common.DetectLanguage(file)

        // Read file content
        content, err := common.ReadFileContent(file)
        if err != nil {
            return fmt.Errorf("failed to read context file %s: %w", file, err)
        }

        // Add to context with language annotation
        contextBuilder.WriteString(fmt.Sprintf("// File: %s (Language: %s)\n", file, language))
        contextBuilder.WriteString(content)
        contextBuilder.WriteString("\n\n")
    }

    // Add context to prompt
    if contextBuilder.Len() > 0 {
        if options.Prompt == "" {
            options.Prompt = contextBuilder.String()
        } else {
            options.Prompt = contextBuilder.String() + "\n\n" + options.Prompt
        }
    }

    return nil
}
```

### Testing Your Provider

Use the comprehensive testing helpers:

```go
func TestMyProvider(t *testing.T) {
    // Create test configuration
    config := types.ProviderConfig{
        Type:         types.ProviderTypeMyProvider,
        APIKey:       "test-api-key",
        DefaultModel: "my-model-v1",
    }

    provider := NewMyProvider(config)

    // Run comprehensive test suite
    t.Run("Comprehensive", func(t *testing.T) {
        common.RunProviderTests(t, provider, config, []string{
            "my-model-v1",
            "my-model-v2",
        })
    })

    // Test tool calling if supported
    if provider.SupportsToolCalling() {
        t.Run("ToolCalling", func(t *testing.T) {
            toolHelper := common.NewToolCallTestHelper(t)
            toolHelper.StandardToolTestSuite(provider)
        })
    }
}

func TestMyProviderWithMock(t *testing.T) {
    // Create mock server for testing
    mockResponse := `{
        "id": "chatcmpl-test",
        "object": "chat.completion",
        "created": 1234567890,
        "model": "my-model-v1",
        "choices": [{
            "index": 0,
            "message": {
                "role": "assistant",
                "content": "Hello, world!"
            },
            "finish_reason": "stop"
        }],
        "usage": {
            "prompt_tokens": 10,
            "completion_tokens": 5,
            "total_tokens": 15
        }
    }`

    server := common.NewMockServer(mockResponse, 200)
    server.SetHeader("Content-Type", "application/json")
    server.SetHeader("x-ratelimit-limit-requests", "100")
    server.SetHeader("x-ratelimit-remaining-requests", "99")
    url := server.Start()
    defer server.Close()

    // Configure provider to use mock server
    config := types.ProviderConfig{
        Type:    types.ProviderTypeMyProvider,
        BaseURL: url,
        APIKey:  "test-key",
    }

    provider := NewMyProvider(config)
    ctx := context.Background()

    // Test basic functionality
    stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Prompt: "Hello",
        Model:  "my-model-v1",
    })
    require.NoError(t, err)
    defer stream.Close()

    chunk, err := stream.Next()
    require.NoError(t, err)
    assert.Equal(t, "Hello, world!", chunk.Content)
    assert.Equal(t, int64(15), chunk.Usage.TotalTokens)
}
```

### Provider Registration

Register your provider with the factory:

```go
package myprovider

import "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"

func RegisterWithFactory(f *factory.ProviderFactory) {
    f.RegisterProvider(types.ProviderTypeMyProvider, func(config types.ProviderConfig) types.Provider {
        return NewMyProvider(config)
    })
}
```

### Configuration Support

Support configuration through YAML/JSON:

```go
// config.yaml example
providers:
  myprovider:
    type: myprovider
    api_key: ${MYPROVIDER_API_KEY}
    base_url: https://api.myprovider.com/v1
    default_model: my-model-v1
    timeout: 30
    supports_tool_calling: true
    supports_streaming: true
```

### OpenAI-Compatible Providers

For OpenAI-compatible providers, you can extend the base OpenAI provider:

```go
package openrouter

import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/openai"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type OpenRouterProvider struct {
    *openai.OpenAIProvider
    siteURL  string
    siteName string
}

func NewOpenRouterProvider(config types.ProviderConfig) *OpenRouterProvider {
    // Create base OpenAI provider with custom base URL
    baseProvider := openai.NewOpenAIProvider(config)

    // Extract OpenRouter-specific config
    siteURL, _ := config.ProviderConfig["site_url"].(string)
    siteName, _ := config.ProviderConfig["site_name"].(string)

    return &OpenRouterProvider{
        OpenAIProvider: baseProvider,
        siteURL:        siteURL,
        siteName:       siteName,
    }
}

// Override request building to add OpenRouter headers
func (p *OpenRouterProvider) buildRequest(options types.GenerateOptions) (*http.Request, error) {
    req, err := p.OpenAIProvider.BuildRequest(options)
    if err != nil {
        return nil, err
    }

    // Add OpenRouter-specific headers
    if p.siteURL != "" {
        req.Header.Set("HTTP-Referer", p.siteURL)
    }
    if p.siteName != "" {
        req.Header.Set("X-Title", p.siteName)
    }

    return req, nil
}
```

### Benefits of Using Shared Utilities

1. **Reduced Boilerplate**: Common functionality is already implemented
2. **Consistent Behavior**: All providers handle rate limiting, caching, etc. consistently
3. **Easier Testing**: Comprehensive test helpers reduce testing overhead
4. **Faster Development**: Focus on provider-specific logic, not infrastructure
5. **Better Maintainability**: Shared code is maintained centrally
6. **Rate Limiting**: Automatic rate limit tracking and enforcement
7. **Model Caching**: Efficient model list management with TTL
8. **File Processing**: Consistent file handling with language detection

### Minimal Provider Template

Here's a minimal template for a new provider:

```go
package minimalprovider

import (
    "context"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type MinimalProvider struct {
    config          types.ProviderConfig
    rateLimitHelper *common.RateLimitHelper
    modelCache      *common.ModelCache
}

func NewMinimalProvider(config types.ProviderConfig) *MinimalProvider {
    return &MinimalProvider{
        config:          config,
        rateLimitHelper: common.NewRateLimitHelper(&MinimalParser{}),
        modelCache:      common.NewModelCache(1 * time.Hour),
    }
}

// Implement required interface methods...
func (p *MinimalProvider) Name() string { return "minimal" }
func (p *MinimalProvider) Type() types.ProviderType { return types.ProviderTypeMinimal }
func (p *MinimalProvider) Description() string { return "Minimal Provider" }
func (p *MinimalProvider) GetDefaultModel() string { return p.config.DefaultModel }

func (p *MinimalProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        func() ([]types.Model, error) { return p.fetchModels(ctx) },
        func() []types.Model { return p.getStaticModels() },
    )
}

// Implement other methods using shared utilities...
```

This approach allows you to focus on the unique aspects of your provider while leveraging battle-tested shared infrastructure.

## Phase 3 Interface Segregation with Providers

Phase 3 introduced interface segregation, allowing you to work with providers using smaller, focused interfaces instead of the full Provider interface. This is particularly useful when you only need specific capabilities from a provider.

### Using Segregated Interfaces

**Example: Model Discovery Service**

```go
// Service that only needs model discovery capabilities
type ModelComparisonService struct {
    providers []types.ModelProvider  // Only needs model discovery
}

func (s *ModelComparisonService) AddProvider(provider types.ModelProvider) {
    s.providers = append(s.providers, provider)
}

func (s *ModelComparisonService) FindBestModel(ctx context.Context, requirements ModelRequirements) (*types.Model, error) {
    var bestModel *types.Model

    for _, provider := range s.providers {
        models, err := provider.GetModels(ctx)
        if err != nil {
            continue
        }

        for _, model := range models {
            if s.meetsRequirements(model, requirements) {
                if bestModel == nil || s.isBetter(model, bestModel) {
                    bestModel = &model
                }
            }
        }
    }

    return bestModel, nil
}

// Usage with any provider
factory := factory.NewProviderFactory()
factory.RegisterDefaultProviders(factory)

// Create providers (any type)
anthropicProvider, _ := factory.CreateModelProvider(types.ProviderTypeAnthropic, config)
openaiProvider, _ := factory.CreateModelProvider(types.ProviderTypeOpenAI, config)

service := NewModelComparisonService()
service.AddProvider(anthropicProvider)  // Only ModelProvider interface needed
service.AddProvider(openaiProvider)
```

**Example: Health Monitoring Service**

```go
// Service that only needs health monitoring
type HealthMonitoringService struct {
    providers []types.HealthCheckProvider  // Only needs health checking
    interval  time.Duration
}

func (s *HealthMonitoringService) AddProvider(provider types.HealthCheckProvider) {
    s.providers = append(s.providers, provider)
}

func (s *HealthMonitoringService) StartMonitoring(ctx context.Context) {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.checkAllProviders(ctx)
        }
    }
}

func (s *HealthMonitoringService) checkAllProviders(ctx context.Context) {
    for _, provider := range s.providers {
        go func(p types.HealthCheckProvider) {
            start := time.Now()
            err := p.HealthCheck(ctx)
            latency := time.Since(start)

            metrics := p.GetMetrics()

            if err != nil {
                log.Printf("Provider %s unhealthy: %v (latency: %v)",
                    p.GetDefaultModel(), err, latency)
            } else {
                log.Printf("Provider %s healthy (latency: %v, requests: %d)",
                    p.GetDefaultModel(), latency, metrics.RequestCount)
            }
        }(provider)
    }
}
```

**Example: Chat-Only Service**

```go
// Service that only needs chat generation
type SimpleChatService struct {
    provider types.ChatProvider  // Only needs chat generation
}

func NewSimpleChatService(provider types.ChatProvider) *SimpleChatService {
    return &SimpleChatService{provider: provider}
}

func (s *SimpleChatService) Generate(ctx context.Context, prompt string) (string, error) {
    stream, err := s.provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Prompt: prompt,
        Model:  s.provider.GetDefaultModel(),
        Stream: false,
    })
    if err != nil {
        return "", err
    }
    defer stream.Close()

    chunk, err := stream.Next()
    if err != nil {
        return "", err
    }

    return chunk.Content, nil
}

// Create with any chat-capable provider
provider, _ := factory.CreateChatProvider(types.ProviderTypeAnthropic, config)
chatService := NewSimpleChatService(provider)
```

### Creating Focused Providers

**Example: Read-Only Provider for Monitoring**

```go
// Provider that only implements health monitoring
type MonitoringProvider struct {
    name         string
    model        string
    endpoint     string
    client       *http.Client
}

// Implement only HealthCheckProvider interface
func (p *MonitoringProvider) HealthCheck(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, "GET", p.endpoint+"/health", nil)
    resp, err := p.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("health check failed: %d", resp.StatusCode)
    }
    return nil
}

func (p *MonitoringProvider) GetMetrics() types.ProviderMetrics {
    // Return synthetic metrics for monitoring
    return types.ProviderMetrics{
        RequestCount:  0,
        SuccessCount:  0,
        ErrorCount:    0,
        HealthStatus: types.HealthStatus{Healthy: true},
    }
}

// Implement CoreProvider for basic info
func (p *MonitoringProvider) Name() string { return p.name }
func (p *MonitoringProvider) Type() types.ProviderType { return types.ProviderTypeCustom }
func (p *MonitoringProvider) Description() string { return "Monitoring-only provider" }

// Implement ModelProvider for model info
func (p *MonitoringProvider) GetDefaultModel() string { return p.model }
func (p *MonitoringProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return []types.Model{{ID: p.model, Name: p.model}}, nil
}
```

### Capability-Based Interfaces

**Example: Adaptive Service Based on Capabilities**

```go
// Service that adapts based on provider capabilities
type AdaptiveService struct {
    provider interface{}
}

func NewAdaptiveService(provider interface{}) *AdaptiveService {
    return &AdaptiveService{provider: provider}
}

func (s *AdaptiveService) Generate(ctx context.Context, options GenerateOptions) (*Response, error) {
    // Check capabilities and use appropriate features
    switch p := s.provider.(type) {
    case types.ToolCallingProvider:
        return s.generateWithTools(ctx, p, options)
    case types.StreamingProvider:
        return s.generateWithStreaming(ctx, p, options)
    case types.ChatProvider:
        return s.generateBasic(ctx, p, options)
    default:
        return nil, fmt.Errorf("provider does not support chat generation")
    }
}

func (s *AdaptiveService) generateWithTools(ctx context.Context, provider types.ToolCallingProvider, options GenerateOptions) (*Response, error) {
    // Use tool calling capabilities
    if options.Tools != nil {
        // Provider supports tool calling, use it
        return s.executeWithTools(ctx, provider, options)
    }
    return s.executeBasic(ctx, provider, options)
}
```

### Using the Standardized Core API

**Example: Provider with Core API Extension**

```go
// Wrap any provider with standardized API
func CreateStandardizedProvider(providerType types.ProviderType, config types.ProviderConfig) (types.CoreChatProvider, error) {
    // Create legacy provider
    legacyProvider, err := factory.CreateProvider(providerType, config)
    if err != nil {
        return nil, err
    }

    // Get appropriate extension
    extension, err := GetExtensionForProvider(providerType)
    if err != nil {
        return nil, err
    }

    // Create core provider adapter
    return types.NewCoreProviderAdapter(legacyProvider, extension), nil
}

// Use with standardized request builder
func (s *ModernChatService) Generate(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Build standardized request
    request, err := types.NewCoreRequestBuilder().
        WithMessages(req.Messages).
        WithModel(req.Model).
        WithMaxTokens(req.MaxTokens).
        WithTemperature(req.Temperature).
        WithTools(req.Tools).
        Build()
    if err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }

    // Generate using standardized API
    response, err := s.coreProvider.GenerateStandardCompletion(ctx, *request)
    if err != nil {
        return nil, fmt.Errorf("generation failed: %w", err)
    }

    return &ChatResponse{
        Content: response.Choices[0].Message.Content,
        Usage:   response.Usage,
        Model:   response.Model,
        ID:      response.ID,
    }, nil
}
```

### Migration from Full Provider Interface

**Step 1: Identify Required Capabilities**

```go
// Before: Using full Provider interface
type OldService struct {
    provider types.Provider  // Requires all 17 methods
}

// After: Using only required interfaces
type NewService struct {
    chatProvider types.ChatProvider           // Only 1 method needed
    healthProvider types.HealthCheckProvider  // Only 2 methods needed
}

func NewService(provider types.Provider) *NewService {
    return &NewService{
        chatProvider:    provider,  // Provider implements all interfaces
        healthProvider: provider,
    }
}
```

**Step 2: Update Factory Usage**

```go
// Before: Create full provider
provider, err := factory.CreateProvider(types.ProviderTypeAnthropic, config)

// After: Create specific interface provider
chatProvider, err := factory.CreateChatProvider(types.ProviderTypeAnthropic, config)
healthProvider, err := factory.CreateHealthCheckProvider(types.ProviderTypeAnthropic, config)
```

### Benefits of Interface Segregation

1. **Reduced Coupling**: Services depend only on methods they actually use
2. **Better Testing**: Create focused mocks for specific interfaces
3. **Cleaner Code**: Dependencies are explicit and minimal
4. **Easier Maintenance**: Changes to unused methods don't affect your code
5. **Performance**: No need to implement unused functionality

### Best Practices

1. **Choose the smallest interface** that meets your needs
2. **Compose interfaces** to create focused abstractions
3. **Use type assertions** for optional capabilities
4. **Create focused mocks** for testing specific interfaces
5. **Document interface requirements** clearly in your API

## OpenAI-Compatible Providers

The SDK also supports adding OpenAI-compatible providers for local models and custom endpoints using the patterns above.

### Adding OpenAI-Compatible Providers

Many providers offer OpenAI-compatible APIs:

#### Groq

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI, // Use OpenAI type
    Name:         "groq",
    BaseURL:      "https://api.groq.com/openai/v1",
    APIKey:       "gsk_...",
    DefaultModel: "llama-3.1-8b-instant",
}
```

#### LM Studio

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    Name:         "lmstudio",
    BaseURL:      "http://localhost:1234/v1",
    APIKey:       "not-needed", // LM Studio doesn't require API key
    DefaultModel: "local-model",
}
```

#### Ollama

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    Name:         "ollama",
    BaseURL:      "http://localhost:11434/v1",
    APIKey:       "ollama", // Dummy key
    DefaultModel: "llama2",
}
```

### Configuration for Custom Endpoints

```yaml
# config.yaml
providers:
  custom:
    groq:
      type: openai
      base_url: https://api.groq.com/openai/v1
      api_key: gsk_...
      default_model: llama-3.1-8b-instant
      models:
        - llama-3.1-8b-instant
        - llama-3.3-70b-versatile

    lmstudio:
      type: openai
      base_url: http://localhost:1234/v1
      api_key: not-needed
      default_model: local-model
```

### Examples: Groq, LM Studio, Ollama

#### Groq (Cloud)

```go
// Ultra-fast inference with Groq
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    Name:         "groq",
    BaseURL:      "https://api.groq.com/openai/v1",
    APIKey:       "gsk_your-groq-api-key-here", // Replace with your actual Groq API key
    DefaultModel: "llama-3.3-70b-versatile",
}

provider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, config)
stream, _ := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Explain recursion",
    Model:  "llama-3.3-70b-versatile",
})
```

#### LM Studio (Local)

```go
// Run models locally with LM Studio
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    Name:         "lmstudio",
    BaseURL:      "http://localhost:1234/v1",
    APIKey:       "not-needed",
    DefaultModel: "TheBloke/Mixtral-8x7B-Instruct-v0.1-GGUF",
}

// No external API calls - completely local
provider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, config)
```

#### Ollama (Local)

```go
// Run models with Ollama
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    Name:         "ollama",
    BaseURL:      "http://localhost:11434/v1",
    APIKey:       "ollama",
    DefaultModel: "llama2",
}

// Pull model first: ollama pull llama2
provider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, config)
```

### Local Model Support

Benefits of local models:

1. **Privacy**: Data never leaves your machine
2. **Cost**: No API fees
3. **Availability**: No rate limits or internet required
4. **Customization**: Fine-tune models for your use case

### Performance Comparison

| Provider | Speed | Cost | Privacy | Quality |
|----------|-------|------|---------|---------|
| GPT-4 (OpenAI) | Medium | High | Low | Excellent |
| Claude (Anthropic) | Medium | High | Low | Excellent |
| Groq | Ultra Fast | Low | Low | Good |
| LM Studio | Fast | Free | High | Good |
| Ollama | Medium | Free | High | Good |

### Best Practices for Custom Providers

1. **Test locally first**: Verify endpoint works with curl/httpie
2. **Check OpenAI compatibility**: Not all providers are 100% compatible
3. **Configure timeouts appropriately**: Local models may be slower
4. **Monitor resource usage**: Local models use CPU/GPU
5. **Keep models updated**: Pull latest versions regularly

### Common Issues and Solutions

**Issue**: Connection refused (local providers)
```bash
# Solution: Ensure service is running
lm-studio server start  # LM Studio
ollama serve           # Ollama
```

**Issue**: Model not found
```bash
# Solution: Pull model first
ollama pull llama2
```

**Issue**: Slow inference (local)
```go
// Solution: Increase timeout and use smaller models
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    BaseURL:      "http://localhost:1234/v1",
    DefaultModel: "7b-model", // Use smaller model
}
```

---

## Appendix: Configuration Examples

### Complete Multi-Provider Configuration

```yaml
providers:
  enabled:
    - openai
    - anthropic
    - gemini
    - cerebras
    - qwen
    - openrouter
    - groq

  preferred_order:
    - cerebras  # Fastest
    - anthropic # Best quality
    - openai    # Reliable
    - gemini    # Large context
    - openrouter # Fallback

  openai:
    type: openai
    api_key: sk-...
    default_model: gpt-4o
    api_keys:
      - sk-key1...
      - sk-key2...

  anthropic:
    type: anthropic
    oauth_credentials:
      - id: primary
        access_token: sk-ant-oat01-...
        refresh_token: sk-ant-ort01-...
    api_keys:
      - sk-ant-api03-key1...
      - sk-ant-api03-key2...

  gemini:
    type: gemini
    oauth_credentials:
      - id: default
        client_id: ...apps.googleusercontent.com
        client_secret: GOCSPX-...
        access_token: ya29.a0...
        refresh_token: 1//06...
    project_id: my-gcp-project

  cerebras:
    type: cerebras
    api_keys:
      - csk-key1...
      - csk-key2...
    default_model: zai-glm-4.6
    max_tokens: 131072

  qwen:
    type: qwen
    oauth_credentials:
      - id: default
        client_id: f0304373b74a44d2b584a3fb70ca9e56
        access_token: token...
        refresh_token: refresh...

  openrouter:
    type: openrouter
    api_key: sk-or-...
    models:
      - anthropic/claude-3.5-sonnet
      - openai/gpt-4o
    model_strategy: failover
    free_only: false

  custom:
    groq:
      type: openai
      base_url: https://api.groq.com/openai/v1
      api_key: gsk_...
      default_model: llama-3.3-70b-versatile
```

### Environment Variables

```bash
# API Keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GEMINI_API_KEY="AIza..."
export CEREBRAS_API_KEY="csk-..."
export QWEN_API_KEY="..."
export OPENROUTER_API_KEY="sk-or-..."

# OAuth Credentials
export ANTHROPIC_ACCESS_TOKEN="sk-ant-oat01-..."
export ANTHROPIC_REFRESH_TOKEN="sk-ant-ort01-..."
export GEMINI_ACCESS_TOKEN="ya29.a0..."
export GEMINI_REFRESH_TOKEN="1//06..."

# Configuration
export PROVIDER_CONFIG_PATH="~/.ai-provider-kit/config.yaml"
```

---

## Support and Resources

- **Documentation**: [pkg.go.dev](https://pkg.go.dev/github.com/cecil-the-coder/ai-provider-kit)
- **Examples**: `/examples` directory
- **Issue Tracker**: [GitHub Issues](https://github.com/cecil-the-coder/ai-provider-kit/issues)
- **Rate Limiting Guide**: See `RATE_LIMIT_IMPLEMENTATION.md`
- **Tool Calling Guide**: See `TOOL_CALLING.md`
- **OAuth Guide**: See `docs/OAUTH_MANAGER.md`

---

**Last Updated**: 2025-11-18
**SDK Version**: 1.0.0
