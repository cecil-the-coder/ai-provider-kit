# AI Provider Kit

[![Go Reference](https://pkg.go.dev/badge/github.com/cecil-the-coder/ai-provider-kit.svg)](https://pkg.go.dev/github.com/cecil-the-coder/ai-provider-kit)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/cecil-the-coder/ai-provider-kit)](https://goreportcard.com/report/github.com/cecil-the-coder/ai-provider-kit)
[![CI](https://github.com/cecil-the-coder/ai-provider-kit/workflows/CI/badge.svg)](https://github.com/cecil-the-coder/ai-provider-kit/actions)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/cecil-the-coder/ai-provider-kit)](https://github.com/cecil-the-coder/ai-provider-kit/releases/latest)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/cecil-the-coder/ai-provider-kit)](https://github.com/cecil-the-coder/ai-provider-kit)
[![GitHub downloads](https://img.shields.io/github/downloads/cecil-the-coder/ai-provider-kit/total)](https://github.com/cecil-the-coder/ai-provider-kit/releases)

A comprehensive Go library for creating and managing AI provider instances with support for multiple AI services.

## Features

- **Multi-Provider Support**: OpenAI, Anthropic, Gemini, Cerebras, OpenRouter, Qwen, and more
- **Interface Segregation**: Focused interfaces for specific capabilities
- **Standardized Core API**: Consistent request/response patterns across providers
- **Provider Extensions**: Preserve provider-specific capabilities while using standardized API
- **Multimodal Content**: First-class support for images, documents, and audio with automatic provider translation
- **Factory Pattern**: Dynamic provider creation and configuration
- **Authentication Support**: API keys, OAuth 2.0, and custom authentication methods
- **Health Monitoring**: Automatic health checks and metrics collection
- **Load Balancing**: Multiple API keys with round-robin distribution
- **Extensible**: Easy to add new providers through the factory pattern
- **Local Models**: Support for LM Studio, Ollama, and Llama.cpp
- **Streaming**: Real-time response streaming
- **Tool Calling**: Built-in support for function calling
- **Comprehensive Testing**: 100% test coverage with unit and integration tests

## Architecture

The AI Provider Kit features a modern, flexible architecture with interface segregation and standardized patterns:

- **Interface Segregation**: 9 focused interfaces instead of a monolithic provider interface
- **Standardized Core API**: Universal request/response types with provider-specific extensions
- **Extensible Design**: Easy to add new providers and capabilities

For complete architecture documentation, see the [Architecture Guide](docs/ARCHITECTURE.md).

## Installation

```bash
go get github.com/cecil-the-coder/ai-provider-kit@latest
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Create a new provider factory
    f := factory.NewProviderFactory()

    // Initialize default providers (with real implementations)
    factory.RegisterDefaultProviders(f)

    // Configure an OpenAI provider
    config := types.ProviderConfig{
        Type:         types.ProviderTypeOpenAI,
        Name:         "openai-primary",
        APIKey:       "your-api-key",
        DefaultModel: "gpt-4",
    }

    // Create provider instance
    provider, err := f.CreateProvider(types.ProviderTypeOpenAI, config)
    if err != nil {
        log.Fatal(err)
    }

    // Generate completion
    stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Prompt: "Hello, world!",
        Stream: false,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer stream.Close()
}
```

## Supported Providers

### Cloud Providers
- **OpenAI**: GPT models, embeddings, and more
- **Anthropic**: Claude models
- **Google Gemini**: Gemini models
- **Cerebras**: High-performance inference
- **OpenRouter**: Multi-provider routing
- **Qwen**: Alibaba Cloud models

### Local Providers
- **LM Studio**: Local model serving
- **Ollama**: Local model management
- **Llama.cpp**: Efficient local inference

## Configuration

Each provider can be configured with the following options:

```go
type ProviderConfig struct {
    Type              ProviderType
    Name              string
    APIKey            string
    BaseURL           string
    DefaultModel      string
    Timeout           time.Duration
    MaxRetries        int
    RateLimit         RateLimitConfig
    OAuth             *OAuthConfig
    Headers           map[string]string
}
```

## Authentication

### API Key Authentication
```go
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key-here",
}
```

### OAuth 2.0 Authentication
```go
config := types.ProviderConfig{
    Type: types.ProviderTypeAnthropic,
    OAuth: &types.OAuthConfig{
        ClientID:     "your-client-id",
        ClientSecret: "your-client-secret",
        TokenURL:     "https://api.anthropic.com/oauth/token",
    },
}
```

## Load Balancing

Configure multiple API keys for load balancing:

```go
provider, _ := f.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKeys: []string{
        "key1",
        "key2",
        "key3",
    },
    LoadBalancer: &types.LoadBalancerConfig{
        Strategy: types.LoadBalancerStrategyRoundRobin,
    },
})
```

## Streaming

Enable streaming for real-time responses:

```go
stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
    Prompt: "Explain quantum computing",
    Stream: true,
})

for {
    chunk, err := stream.Next()
    if err != nil {
        break
    }

    if chunk.Done {
        break
    }

    fmt.Print(chunk.Content)
}
```

## Multimodal Content

ai-provider-kit provides first-class support for multimodal content including images, documents, and audio. The library handles provider-specific format translations automatically while maintaining a clean, unified API.

### Design Philosophy

The multimodal system is built on **primitives, not patterns**. We provide the building blocks (`ContentPart`, `MediaSource`) that let you compose rich multimodal interactions without imposing high-level abstractions. This design enables you to build your own patterns - whether that's caching layers, agent delegation systems, or preprocessing pipelines.

### Quick Example

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// Simple text message (backwards compatible)
msg := types.ChatMessage{
    Role:    "user",
    Content: "Hello, world!",
}

// Image with text using Parts
msg := types.ChatMessage{
    Role: "user",
    Parts: []types.ContentPart{
        types.NewTextPart("What's in this image?"),
        types.NewImagePart("image/png", base64Data),
    },
}

// Image from URL
msg := types.ChatMessage{
    Role: "user",
    Parts: []types.ContentPart{
        types.NewTextPart("Describe this image"),
        types.NewImageURLPart("image/jpeg", "https://example.com/image.jpg"),
    },
}

// Document (PDF)
msg := types.ChatMessage{
    Role: "user",
    Parts: []types.ContentPart{
        types.NewTextPart("Summarize this document"),
        types.NewDocumentPart("application/pdf", pdfBase64Data),
    },
}
```

### Supported Content Types

The library supports the following content types through the `ContentPart` struct:

- **text** - Plain text content
- **image** - Images (PNG, JPEG, GIF, WebP)
- **document** - Documents (PDF, etc.)
- **audio** - Audio files (provider-dependent)
- **tool_use** - Tool/function calls from the model
- **tool_result** - Results returned from tool execution
- **thinking** - Extended reasoning/thinking blocks

### Media Sources

Media content can be provided in two ways:

```go
// Base64-encoded data
source := &types.MediaSource{
    Type:      types.MediaSourceBase64,
    MediaType: "image/png",
    Data:      base64EncodedString,
}

// URL reference
source := &types.MediaSource{
    Type:      types.MediaSourceURL,
    MediaType: "image/jpeg",
    URL:       "https://example.com/image.jpg",
}
```

### Helper Methods

The `ChatMessage` type includes several helper methods for working with multimodal content:

```go
// Get unified content as []ContentPart (handles both Content string and Parts)
parts := message.GetContentParts()

// Extract all text content from message
text := message.GetTextContent()

// Check if message contains images
if message.HasImages() {
    // Handle image content
}

// Check if message contains any media (images, documents, audio)
if message.HasMedia() {
    // Handle media content
}

// Add content parts dynamically
message.AddContentPart(types.NewTextPart("Additional text"))
message.AddContentPart(types.NewImagePart("image/png", imageData))
```

### Provider Support

Different providers support different content types. The library automatically handles format translation for each provider:

| Content Type | Anthropic | OpenAI | Gemini |
|-------------|-----------|--------|--------|
| text | ✅ | ✅ | ✅ |
| image (base64) | ✅ | ✅ (data URL) | ✅ (inlineData) |
| image (url) | ✅ | ✅ | ✅ (fileData) |
| document | ✅ | ❌ | ✅ |
| audio | ❌ | ❌ | ✅ |
| tool_use | ✅ | ✅ | ✅ |
| tool_result | ✅ | ✅ | ✅ |

**Provider-Specific Notes:**

- **OpenAI**: Images are converted to data URLs (`data:image/png;base64,...`). Documents and audio are not supported in chat completions.
- **Anthropic**: Native support for images and PDFs through content blocks. Images and documents use the `source` structure.
- **Gemini**: Most comprehensive support including audio. Images/documents use `inlineData` for base64 and `fileData` for URLs.

### Advanced Use Cases

The primitive-based design enables sophisticated patterns:

**Image Caching Layer**
```go
// Build your own caching system for frequently-used images
type ImageCache struct {
    cache map[string]string // URL -> base64
}

func (c *ImageCache) GetOrFetch(url string) types.ContentPart {
    if data, exists := c.cache[url]; exists {
        return types.NewImagePart("image/jpeg", data)
    }
    // Fetch and cache...
}
```

**Agent-Based Delegation**
```go
// Route multimodal requests to specialized agents
if message.HasImages() {
    // Send to vision-specialized model
    return visionAgent.Process(message)
} else if message.HasMedia() {
    // Send to multimodal-capable model
    return multimodalAgent.Process(message)
}
```

**Preprocessing Pipelines**
```go
// Transform content before sending to provider
for i, part := range message.Parts {
    if part.Type == types.ContentTypeImage {
        // Resize, compress, or convert format
        message.Parts[i] = preprocessImage(part)
    }
}
```

### Backwards Compatibility

The multimodal system is fully backwards compatible. Existing code using `Content` string continues to work:

```go
// Legacy approach - still works
msg := types.ChatMessage{
    Role:    "user",
    Content: "Hello!",
}

// Multimodal approach - new capability
msg := types.ChatMessage{
    Role: "user",
    Parts: []types.ContentPart{
        types.NewTextPart("Hello!"),
        types.NewImagePart("image/png", data),
    },
}
```

Both approaches use the same `ChatMessage` type, and the library handles the translation automatically through `GetContentParts()`.

## Tool Calling

ai-provider-kit provides comprehensive tool calling support across all providers with format translation, validation, and advanced control features.

### Quick Example

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
                    "description": "The city and state, e.g. San Francisco, CA",
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

options := types.GenerateOptions{
    Prompt: "What's the weather in San Francisco?",
    Tools:  tools,
}

stream, err := provider.GenerateChatCompletion(context.Background(), options)
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

chunk, err := stream.Next()
if err != nil {
    log.Fatal(err)
}

// Handle tool calls
if len(chunk.Choices) > 0 && len(chunk.Choices[0].Message.ToolCalls) > 0 {
    for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
        // Execute tool locally
        result := executeToolCall(toolCall)

        // Send result back
        // (see TOOL_CALLING.md for complete multi-turn example)
    }
}
```

For a comprehensive guide, see [TOOL_CALLING.md](./TOOL_CALLING.md)

### Features

- **Universal Format**: Define tools once, use across all providers
- **ToolChoice Control**: Fine-grained control over tool selection
  - `auto` - Model decides whether to use tools
  - `required` - Must use at least one tool
  - `none` - Don't use tools
  - `specific` - Force a specific tool
- **Parallel Tool Calling**: Handle multiple tool calls in a single response
- **Tool Validation**: Optional validation via `pkg/toolvalidator`
- **Streaming Support**: Tool calls work with streaming responses
- **Format Translation**: Automatic conversion between provider formats

### Supported Providers

All 6 providers support tool calling:
- OpenAI (native format)
- Anthropic (content blocks format)
- Gemini (function declarations format)
- Cerebras (OpenAI-compatible)
- Qwen (OpenAI-compatible)
- OpenRouter (OpenAI-compatible)

## Health Monitoring

Monitor provider health and performance:

```go
// Health check
err := provider.HealthCheck(context.Background())
if err != nil {
    log.Printf("Provider unhealthy: %v", err)
}

// Get metrics
metrics := provider.GetMetrics()
log.Printf("Requests: %d, Success Rate: %.2f%%",
    metrics.RequestCount,
    float64(metrics.SuccessCount)/float64(metrics.RequestCount)*100)
```

## Custom Providers

Add your own provider implementations:

```go
// Implement the Provider interface
type CustomProvider struct {
    // your fields
}

func (p *CustomProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // your implementation
}

// Register your provider
factory.RegisterProvider(types.ProviderTypeCustom, func(config types.ProviderConfig) types.Provider {
    return &CustomProvider{config: config}
})
```

## Development

### Prerequisites
- Go 1.23 or later
- Docker (optional)

### Running Tests
```bash
# Run all tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. ./...
```

### Building
```bash
# Build library
go build ./...

# Build CLI tool (if available)
go build -o ai-provider-kit ./cmd/ai-provider-kit
```

### Docker
```bash
# Build Docker image
docker build -t ai-provider-kit .

# Run with Docker
docker run -p 8080:8080 ai-provider-kit
```

## Documentation

Comprehensive documentation is available in the [/docs](/docs) directory:

- **[Getting Started](/docs/SDK_GETTING_STARTED.md)** - Quick start guide
- **[API Reference](/docs/SDK_API_REFERENCE.md)** - Complete API documentation
- **[Error Code Mapping](/docs/ERROR_CODE_MAPPING.md)** - Error handling and troubleshooting
- **[Best Practices](/docs/SDK_BEST_PRACTICES.md)** - Recommended patterns
- **[Provider Guides](/docs/SDK_PROVIDER_GUIDES.md)** - Provider-specific documentation
- **[OAuth Manager](/docs/OAUTH_MANAGER.md)** - Enterprise credential management
- **[Metrics](/docs/METRICS.md)** - Monitoring and observability

See the [complete documentation index](/docs/README.md) for all available guides.

## Contributing

Contributions are welcome! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Summary:** You are free to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the software, and to permit persons to whom the software is furnished to do so, subject to the following conditions:

- The copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
- The software is provided "AS IS", without warranty of any kind.

For the full license text, see the [LICENSE](LICENSE) file.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for a list of changes and version history.

For detailed upgrade instructions between versions, including breaking changes and API modifications, see the [Version-Specific Migration Guide](docs/VERSION_MIGRATION_GUIDE.md).

## Support

- 📖 [Documentation](https://pkg.go.dev/github.com/cecil-the-coder/ai-provider-kit)
- 🐛 [Issue Tracker](https://github.com/cecil-the-coder/ai-provider-kit/issues)
- 💬 [Discussions](https://github.com/cecil-the-coder/ai-provider-kit/discussions)