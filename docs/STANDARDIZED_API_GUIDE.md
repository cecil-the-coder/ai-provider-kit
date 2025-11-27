# Standardized Core API Guide

This guide explains how to use the new standardized core API for the AI Provider Kit, which provides consistent request/response patterns across all providers while preserving provider-specific capabilities through extension points.

## Overview

The standardized core API consists of:

1. **StandardRequest/StandardResponse** - Universal request/response formats
2. **Provider Extensions** - Provider-specific format conversion and capabilities
3. **Extension Registry** - Centralized management of provider extensions
4. **Core Provider Interface** - Unified interface for all providers
5. **Adapter Pattern** - Backward compatibility with existing providers

## Key Concepts

### StandardRequest

The `StandardRequest` contains only fields that are universally supported across all providers:

```go
type StandardRequest struct {
    // Core content
    Messages []ChatMessage `json:"messages"`

    // Model selection
    Model string `json:"model,omitempty"`

    // Generation parameters
    MaxTokens   int     `json:"max_tokens,omitempty"`
    Temperature float64 `json:"temperature,omitempty"`
    Stop        []string `json:"stop,omitempty"`

    // Streaming control
    Stream bool `json:"stream"`

    // Tool support (if provider supports it)
    Tools      []Tool      `json:"tools,omitempty"`
    ToolChoice *ToolChoice `json:"tool_choice,omitempty"`

    // Response format (for providers that support structured output)
    ResponseFormat string `json:"response_format,omitempty"`

    // Context and metadata
    Context  context.Context        `json:"-"`
    Timeout  time.Duration         `json:"-"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}
```

### StandardResponse

The `StandardResponse` provides a consistent response format:

```go
type StandardResponse struct {
    ID               string               `json:"id"`
    Model            string               `json:"model"`
    Object           string               `json:"object"`
    Created          int64                `json:"created"`
    Choices          []StandardChoice     `json:"choices"`
    Usage            Usage                `json:"usage"`
    ProviderMetadata map[string]interface{} `json:"provider_metadata,omitempty"`
}
```

### Provider Extensions

Provider extensions handle conversion between standard and provider-specific formats:

```go
type CoreProviderExtension interface {
    // Extension information
    Name() string
    Version() string
    Description() string

    // Convert between standard and provider-specific formats
    StandardToProvider(request StandardRequest) (interface{}, error)
    ProviderToStandard(response interface{}) (*StandardResponse, error)
    ProviderToStandardChunk(chunk interface{}) (*StandardStreamChunk, error)

    // Validate provider-specific options
    ValidateOptions(options map[string]interface{}) error

    // Get provider-specific capabilities
    GetCapabilities() []string
}
```

## Usage Examples

### Basic Chat Completion

```go
import (
    "context"
    "fmt"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Initialize factory with core API support
providerFactory := factory.NewProviderFactory()
coreFactory := types.NewDefaultProviderFactoryExtensions(providerFactory)

// Create provider
providerConfig := types.ProviderConfig{
    Type:    types.ProviderTypeOpenAI,
    APIKey:  "your-api-key",
    BaseURL: "https://api.openai.com/v1",
}

coreProvider, err := coreFactory.CreateCoreProvider(types.ProviderTypeOpenAI, providerConfig)
if err != nil {
    return err
}

// Build standard request
request, err := types.NewCoreRequestBuilder().
    WithMessages([]types.ChatMessage{
        {Role: "user", Content: "Hello, world!"},
    }).
    WithModel("gpt-4").
    WithMaxTokens(100).
    WithTemperature(0.7).
    Build()

if err != nil {
    return err
}

// Generate completion
response, err := coreProvider.GenerateStandardCompletion(context.Background(), *request)
if err != nil {
    return err
}

fmt.Printf("Response: %s\n", response.Choices[0].Message.Content)
```

### Streaming

```go
// Build streaming request
request, err := types.NewCoreRequestBuilder().
    WithMessages([]types.ChatMessage{
        {Role: "user", Content: "Tell me a story"},
    }).
    WithModel("gpt-4").
    WithStreaming(true).
    Build()

if err != nil {
    return err
}

// Generate stream
stream, err := coreProvider.GenerateStandardStream(context.Background(), *request)
if err != nil {
    return err
}
defer stream.Close()

// Process stream
for {
    chunk, err := stream.Next()
    if err != nil {
        break
    }

    if chunk != nil {
        fmt.Print(chunk.Choices[0].Delta.Content)
        if chunk.Done {
            break
        }
    }
}
```

### Tool Calling

```go
// Define tool
weatherTool := types.Tool{
    Name:        "get_weather",
    Description: "Get current weather for a location",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type":        "string",
                "description": "City and state, e.g. San Francisco, CA",
            },
        },
        "required": []string{"location"},
    },
}

// Build request with tools
request, err := types.NewCoreRequestBuilder().
    WithMessages([]types.ChatMessage{
        {Role: "user", Content: "What's the weather in New York?"},
    }).
    WithModel("gpt-4").
    WithTools([]types.Tool{weatherTool}).
    WithToolChoice(&types.ToolChoice{Mode: types.ToolChoiceAuto}).
    Build()

if err != nil {
    return err
}

// Generate completion with tool calling
response, err := coreProvider.GenerateStandardCompletion(context.Background(), *request)
if err != nil {
    return err
}

// Check for tool calls
if len(response.Choices[0].Message.ToolCalls) > 0 {
    for _, toolCall := range response.Choices[0].Message.ToolCalls {
        fmt.Printf("Tool call: %s\n", toolCall.Function.Name)
        fmt.Printf("Arguments: %s\n", toolCall.Function.Arguments)
    }
} else {
    fmt.Printf("Response: %s\n", response.Choices[0].Message.Content)
}
```

### Provider-Specific Features

Use the `Metadata` field to access provider-specific features:

```go
// OpenAI-specific: JSON mode
request, err := types.NewCoreRequestBuilder().
    WithMessages([]types.ChatMessage{
        {Role: "user", Content: "Generate a JSON object"},
    }).
    WithModel("gpt-4").
    WithMetadata("response_format", map[string]interface{}{
        "type": "json_object",
    }).
    Build()

// Anthropic-specific: Thinking mode
request, err := types.NewCoreRequestBuilder().
    WithMessages([]types.ChatMessage{
        {Role: "user", Content: "Solve this complex problem"},
    }).
    WithModel("claude-3-5-sonnet-20241022").
    WithMetadata("thinking", true).
    Build()

// Provider-specific: Top K parameter
request, err := types.NewCoreRequestBuilder().
    WithMessages([]types.ChatMessage{
        {Role: "user", Content: "Generate text"},
    }).
    WithModel("gemini-pro").
    WithMetadata("top_k", 40).
    Build()
```

## Creating Provider Extensions

To add a new provider or extend an existing one:

### 1. Create the Extension

```go
package myprovider

import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type MyProviderExtension struct {
    *types.BaseExtension
}

func NewMyProviderExtension() *MyProviderExtension {
    capabilities := []string{
        "chat", "streaming", "tool_calling", "custom_feature",
    }

    return &MyProviderExtension{
        BaseExtension: types.NewBaseExtension(
            "myprovider",
            "1.0.0",
            "My Provider API extension",
            capabilities,
        ),
    }
}

func (e *MyProviderExtension) StandardToProvider(request types.StandardRequest) (interface{}, error) {
    // Convert standard request to your provider's format
    myRequest := MyProviderRequest{
        Model:       request.Model,
        Messages:    convertMessages(request.Messages),
        MaxTokens:   request.MaxTokens,
        Temperature: request.Temperature,
        Stream:      request.Stream,
    }

    // Handle provider-specific metadata
    if request.Metadata != nil {
        if customFeature, ok := request.Metadata["custom_feature"].(bool); ok {
            myRequest.CustomFeature = customFeature
        }
    }

    return myRequest, nil
}

func (e *MyProviderExtension) ProviderToStandard(response interface{}) (*types.StandardResponse, error) {
    // Convert your provider's response to standard format
    myResp := response.(*MyProviderResponse)

    choices := []types.StandardChoice{
        {
            Index: 0,
            Message: types.ChatMessage{
                Role:    myResp.Message.Role,
                Content: myResp.Message.Content,
            },
            FinishReason: myResp.StopReason,
        },
    }

    usage := types.Usage{
        PromptTokens:     myResp.Usage.InputTokens,
        CompletionTokens: myResp.Usage.OutputTokens,
        TotalTokens:      myResp.Usage.InputTokens + myResp.Usage.OutputTokens,
    }

    return &types.StandardResponse{
        ID:      myResp.ID,
        Model:   myResp.Model,
        Object:  "chat.completion",
        Choices: choices,
        Usage:   usage,
    }, nil
}

func (e *MyProviderExtension) ValidateOptions(options map[string]interface{}) error {
    // Validate provider-specific options
    if customParam, ok := options["custom_feature"].(bool); ok && customParam {
        // Validate custom feature requirements
    }
    return nil
}
```

### 2. Register the Extension

```go
func init() {
    types.RegisterDefaultExtension(types.ProviderTypeMyProvider, NewMyProviderExtension())
}
```

## Migration Guide

### From Legacy API

**Before:**
```go
options := types.GenerateOptions{
    Messages:    messages,
    Model:       "gpt-4",
    MaxTokens:   100,
    Temperature: 0.7,
}

stream, err := provider.GenerateChatCompletion(ctx, options)
```

**After:**
```go
request, err := types.NewCoreRequestBuilder().
    WithMessages(messages).
    WithModel("gpt-4").
    WithMaxTokens(100).
    WithTemperature(0.7).
    Build()

stream, err := coreProvider.GenerateStandardStream(ctx, *request)
```

### Backward Compatibility

The new API is fully backward compatible. Existing providers continue to work through the adapter pattern:

```go
// Existing providers automatically get core API support
provider := factory.NewProviderFactory().CreateProvider(types.ProviderTypeOpenAI, config)
coreProvider := types.NewCoreProviderAdapter(provider, extension)
```

## Best Practices

### 1. Use the Request Builder

Always use the `CoreRequestBuilder` to create requests:

```go
// Good
request, err := types.NewCoreRequestBuilder().
    WithMessages(messages).
    WithModel("gpt-4").
    Build()

// Avoid
request := types.StandardRequest{
    Messages: messages,
    Model:    "gpt-4",
} // No validation
```

### 2. Handle Provider Capabilities

Check provider capabilities before using features:

```go
capabilities := coreProvider.GetStandardCapabilities()
hasToolCalling := false
for _, capability := range capabilities {
    if capability == "tool_calling" {
        hasToolCalling = true
        break
    }
}

if hasToolCalling {
    // Use tool calling
}
```

### 3. Validate Requests

Let the provider validate requests:

```go
err := coreProvider.ValidateStandardRequest(*request)
if err != nil {
    return err
}
```

### 4. Handle Provider Metadata

Use provider metadata for provider-specific information:

```go
if response.ProviderMetadata != nil {
    if systemFingerprint, ok := response.ProviderMetadata["system_fingerprint"].(string); ok {
        fmt.Printf("System fingerprint: %s\n", systemFingerprint)
    }
}
```

### 5. Proper Error Handling

Handle validation and provider errors appropriately:

```go
response, err := coreProvider.GenerateStandardCompletion(ctx, request)
if err != nil {
    if types.IsValidationError(err) {
        // Handle validation error
        return fmt.Errorf("invalid request: %w", err)
    }
    // Handle provider error
    return fmt.Errorf("provider error: %w", err)
}
```

## Advanced Features

### Custom Extensions

Create custom extensions for unique provider capabilities:

```go
type CustomExtension struct {
    *types.BaseExtension
}

func (e *CustomExtension) GetCapabilities() []string {
    return append(e.BaseExtension.GetCapabilities(), "custom_capability")
}
```

### Request/Response Interceptors

Implement middleware patterns for logging, caching, etc.:

```go
type LoggingMiddleware struct {
    coreProvider types.CoreChatProvider
}

func (m *LoggingMiddleware) GenerateStandardCompletion(ctx context.Context, request types.StandardRequest) (*types.StandardResponse, error) {
    log.Printf("Request: model=%s, tokens=%d", request.Model, request.MaxTokens)

    response, err := m.coreProvider.GenerateStandardCompletion(ctx, request)

    log.Printf("Response: tokens=%d", response.Usage.TotalTokens)
    return response, err
}
```

## Testing

### Mock Providers

Use mock providers for testing:

```go
type MockCoreProvider struct {
    responses []*types.StandardResponse
    index     int
}

func (m *MockCoreProvider) GenerateStandardCompletion(ctx context.Context, request types.StandardRequest) (*types.StandardResponse, error) {
    if m.index >= len(m.responses) {
        return nil, fmt.Errorf("no more responses")
    }

    response := m.responses[m.index]
    m.index++
    return response, nil
}
```

### Validation Testing

Test request validation:

```go
func TestInvalidTemperature(t *testing.T) {
    _, err := types.NewCoreRequestBuilder().
        WithMessages([]types.ChatMessage{{Role: "user", Content: "test"}}).
        WithTemperature(3.0). // Invalid
        Build()

    if !types.IsValidationError(err) {
        t.Errorf("Expected validation error")
    }
}
```

## Conclusion

The standardized core API provides:

- **Consistency**: Uniform interface across all providers
- **Flexibility**: Provider-specific capabilities through extensions
- **Compatibility**: Works with existing providers
- **Validation**: Built-in request validation
- **Extensibility**: Easy to add new providers and features

For more examples, see the `examples/standardized-api-demo` directory.