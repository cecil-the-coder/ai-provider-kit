# AI Provider Kit - SDK API Reference

Complete API reference for the AI Provider Kit SDK. This document provides detailed information about all interfaces, types, methods, and error handling patterns.

**Version:** 1.0
**Last Updated:** 2025-11-18

---

## Table of Contents

1. [Core Interfaces](#core-interfaces)
2. [Factory Package](#factory-package)
3. [Types Package](#types-package)
4. [Provider Methods](#provider-methods)
5. [Streaming API](#streaming-api)
6. [Tool Calling API](#tool-calling-api)
7. [Authentication Package](#authentication-package)
8. [Common Utilities Package](#common-utilities-package)
9. [Error Handling](#error-handling)
10. [Constants and Enums](#constants-and-enums)
11. [HTTP Utilities](#http-utilities)
12. [Rate Limiting](#rate-limiting)

---

## Core Interfaces

### Interface Segregation (Phase 3)

Phase 3 introduced interface segregation, breaking down the monolithic Provider interface into smaller, focused interfaces that follow the Interface Segregation Principle.

#### Core Interfaces

**CoreProvider** - Basic provider information (3 methods)

```go
type CoreProvider interface {
    Name() string
    Type() ProviderType
    Description() string
}
```

**ModelProvider** - Model discovery and management (2 methods)

```go
type ModelProvider interface {
    GetModels(ctx context.Context) ([]Model, error)
    GetDefaultModel() string
}
```

**AuthenticatedProvider** - Authentication management (3 methods)

```go
type AuthenticatedProvider interface {
    Authenticate(ctx context.Context, authConfig AuthConfig) error
    IsAuthenticated() bool
    Logout(ctx context.Context) error
}
```

**ConfigurableProvider** - Runtime configuration (2 methods)

```go
type ConfigurableProvider interface {
    Configure(config ProviderConfig) error
    GetConfig() ProviderConfig
}
```

#### Capability Interfaces

**ChatProvider** - Core chat completion (1 method)

```go
type ChatProvider interface {
    GenerateChatCompletion(ctx context.Context, options GenerateOptions) (ChatCompletionStream, error)
}
```

**ToolCallingProvider** - Tool/function calling (3 methods)

```go
type ToolCallingProvider interface {
    InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error)
    SupportsToolCalling() bool
    GetToolFormat() ToolFormat
}
```

**StreamingProvider** - Streaming support indication (1 method)

```go
type StreamingProvider interface {
    SupportsStreaming() bool
}
```

**ResponsesAPIProvider** - Structured responses support (1 method)

```go
type ResponsesAPIProvider interface {
    SupportsResponsesAPI() bool
}
```

**HealthCheckProvider** - Health monitoring (2 methods)

```go
type HealthCheckProvider interface {
    HealthCheck(ctx context.Context) error
    GetMetrics() ProviderMetrics
}
```

#### Composite Provider Interface

The original `Provider` interface is preserved for backward compatibility:

```go
type Provider interface {
    CoreProvider
    ModelProvider
    AuthenticatedProvider
    ConfigurableProvider
    ChatProvider
    ToolCallingProvider
    StreamingProvider
    ResponsesAPIProvider
    HealthCheckProvider
}
```

#### When to Use Specific Interfaces

| Interface | Use Case | Methods |
|-----------|----------|---------|
| **CoreProvider** | Provider identification and info | Name(), Type(), Description() |
| **ModelProvider** | Model discovery and selection | GetModels(), GetDefaultModel() |
| **AuthenticatedProvider** | Authentication management | Authenticate(), IsAuthenticated(), Logout() |
| **ConfigurableProvider** | Runtime configuration | Configure(), GetConfig() |
| **ChatProvider** | Text generation | GenerateChatCompletion() |
| **ToolCallingProvider** | Function/tool calling | InvokeServerTool(), SupportsToolCalling(), GetToolFormat() |
| **StreamingProvider** | Streaming support check | SupportsStreaming() |
| **ResponsesAPIProvider** | Structured responses | SupportsResponsesAPI() |
| **HealthCheckProvider** | Health monitoring | HealthCheck(), GetMetrics() |

### Standardized Core API (Phase 3)

Phase 3 also introduced a standardized core API with provider-specific extensions.

#### Standard Types

**StandardRequest** - Universal request format

```go
type StandardRequest struct {
    Messages     []ChatMessage          `json:"messages"`
    Model        string                 `json:"model"`
    MaxTokens    int                    `json:"max_tokens,omitempty"`
    Temperature  float64                `json:"temperature,omitempty"`
    Stream       bool                   `json:"stream,omitempty"`
    Tools        []Tool                 `json:"tools,omitempty"`
    ToolChoice   *ToolChoice            `json:"tool_choice,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
```

**StandardResponse** - Consistent response format

```go
type StandardResponse struct {
    ID      string         `json:"id"`
    Object  string         `json:"object"`
    Created int64          `json:"created"`
    Model   string         `json:"model"`
    Choices []StandardChoice `json:"choices"`
    Usage   Usage          `json:"usage"`
}

type StandardChoice struct {
    Index        int         `json:"index"`
    Message      ChatMessage `json:"message"`
    FinishReason string      `json:"finish_reason"`
}
```

#### CoreRequestBuilder

```go
type CoreRequestBuilder struct {
    // Internal builder state
}

func NewCoreRequestBuilder() *CoreRequestBuilder
func (b *CoreRequestBuilder) WithMessages(messages []ChatMessage) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithModel(model string) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithMaxTokens(maxTokens int) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithTemperature(temperature float64) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithStreaming(streaming bool) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithTools(tools []Tool) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithToolChoice(toolChoice *ToolChoice) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithMetadata(key string, value interface{}) *CoreRequestBuilder
func (b *CoreRequestBuilder) Build() (*StandardRequest, error)
```

#### CoreChatProvider Interface

```go
type CoreChatProvider interface {
    GenerateStandardCompletion(ctx context.Context, request StandardRequest) (*StandardResponse, error)
    GenerateStandardStream(ctx context.Context, request StandardRequest) (StandardStream, error)
    GetCoreExtension() CoreProviderExtension
    GetStandardCapabilities() []string
    ValidateStandardRequest(request StandardRequest) error
}
```

#### CoreProviderExtension Interface

```go
type CoreProviderExtension interface {
    Name() string
    Version() string
    Description() string
    StandardToProvider(request StandardRequest) (interface{}, error)
    ProviderToStandard(response interface{}) (*StandardResponse, error)
    ProviderToStandardChunk(chunk interface{}) (*StandardStreamChunk, error)
    ValidateOptions(options map[string]interface{}) error
    GetCapabilities() []string
}
```

#### Extension Registry

```go
type ExtensionRegistry struct {
    // Internal registry state
}

func NewExtensionRegistry() *ExtensionRegistry
func (r *ExtensionRegistry) RegisterExtension(providerType ProviderType, extension CoreProviderExtension) error
func (r *ExtensionRegistry) GetExtension(providerType ProviderType) (CoreProviderExtension, error)
func (r *ExtensionRegistry) ListExtensions() []ProviderType
func (r *ExtensionRegistry) GetCapabilities(providerType ProviderType) []string
```

#### Core Provider Adapter

```go
func NewCoreProviderAdapter(provider Provider, extension CoreProviderExtension) CoreChatProvider
```

Creates a CoreChatProvider that wraps an existing Provider with an extension.

### Legacy Provider Interface

The original `Provider` interface is still available for backward compatibility:

```go
type Provider interface {
    // Basic provider information
    Name() string
    Type() ProviderType
    Description() string

    // Model management
    GetModels(ctx context.Context) ([]Model, error)
    GetDefaultModel() string

    // Authentication
    Authenticate(ctx context.Context, authConfig AuthConfig) error
    IsAuthenticated() bool
    Logout(ctx context.Context) error

    // Configuration
    Configure(config ProviderConfig) error
    GetConfig() ProviderConfig

    // Core capabilities
    GenerateChatCompletion(ctx context.Context, options GenerateOptions) (ChatCompletionStream, error)
    InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error)

    // Feature support
    SupportsToolCalling() bool
    SupportsStreaming() bool
    SupportsResponsesAPI() bool
    GetToolFormat() ToolFormat

    // Health and metrics
    HealthCheck(ctx context.Context) error
    GetMetrics() ProviderMetrics
}
```

#### Method Details

**Name() string**

Returns the provider's unique identifier name.

- **Returns:** Provider name (e.g., "openai", "anthropic", "gemini")
- **Example:**
  ```go
  name := provider.Name() // "openai"
  ```

**Type() ProviderType**

Returns the provider's type as a ProviderType enum.

- **Returns:** ProviderType constant
- **Example:**
  ```go
  providerType := provider.Type() // types.ProviderTypeOpenAI
  ```

**Description() string**

Returns a human-readable description of the provider.

- **Returns:** Provider description string
- **Example:**
  ```go
  desc := provider.Description() // "OpenAI GPT models"
  ```

**GetModels(ctx context.Context) ([]Model, error)**

Retrieves the list of available models from the provider.

- **Parameters:**
  - `ctx`: Context for cancellation and timeout control
- **Returns:**
  - `[]Model`: List of available models with capabilities and pricing
  - `error`: Error if model discovery fails
- **Errors:**
  - Authentication errors if not authenticated
  - Network errors if API is unreachable
  - Rate limit errors if quota exceeded
- **Example:**
  ```go
  ctx := context.Background()
  models, err := provider.GetModels(ctx)
  if err != nil {
      log.Fatalf("Failed to get models: %v", err)
  }
  for _, model := range models {
      fmt.Printf("Model: %s (%d tokens)\n", model.Name, model.MaxTokens)
  }
  ```

**GetDefaultModel() string**

Returns the default model ID for this provider.

- **Returns:** Default model identifier
- **Example:**
  ```go
  defaultModel := provider.GetDefaultModel() // "gpt-4"
  ```

**Authenticate(ctx context.Context, authConfig AuthConfig) error**

Authenticates with the provider using the provided configuration.

- **Parameters:**
  - `ctx`: Context for timeout control
  - `authConfig`: Authentication configuration (API key, OAuth, etc.)
- **Returns:**
  - `error`: Authentication error or nil on success
- **Errors:**
  - `ErrInvalidCredentials`: Invalid API key or credentials
  - `ErrNetworkError`: Network connectivity issues
  - `ErrProviderUnavailable`: Provider service is down
- **Example:**
  ```go
  authConfig := types.AuthConfig{
      Method: types.AuthMethodAPIKey,
      APIKey: "sk-your-api-key",
  }
  err := provider.Authenticate(ctx, authConfig)
  if err != nil {
      log.Fatalf("Authentication failed: %v", err)
  }
  ```

**IsAuthenticated() bool**

Checks whether the provider is currently authenticated.

- **Returns:** `true` if authenticated, `false` otherwise
- **Example:**
  ```go
  if !provider.IsAuthenticated() {
      // Perform authentication
  }
  ```

**Logout(ctx context.Context) error**

Logs out and clears authentication state.

- **Parameters:**
  - `ctx`: Context for timeout control
- **Returns:**
  - `error`: Logout error or nil on success
- **Example:**
  ```go
  err := provider.Logout(ctx)
  ```

**Configure(config ProviderConfig) error**

Updates the provider's configuration.

- **Parameters:**
  - `config`: New provider configuration
- **Returns:**
  - `error`: Configuration error or nil on success
- **Example:**
  ```go
  config := types.ProviderConfig{
      Type:         types.ProviderTypeOpenAI,
      APIKey:       "new-api-key",
      DefaultModel: "gpt-4-turbo",
      MaxTokens:    4096,
  }
  err := provider.Configure(config)
  ```

**GetConfig() ProviderConfig**

Retrieves the current provider configuration.

- **Returns:** Current ProviderConfig
- **Example:**
  ```go
  config := provider.GetConfig()
  fmt.Printf("Using model: %s\n", config.DefaultModel)
  ```

**GenerateChatCompletion(ctx context.Context, options GenerateOptions) (ChatCompletionStream, error)**

Generates a chat completion (streaming or non-streaming).

- **Parameters:**
  - `ctx`: Context for cancellation and timeout
  - `options`: Generation options including messages, model, tools, etc.
- **Returns:**
  - `ChatCompletionStream`: Stream interface for reading responses
  - `error`: Generation error or nil on success
- **Errors:**
  - `ErrNotAuthenticated`: Provider not authenticated
  - `ErrRateLimitExceeded`: Rate limit exceeded
  - `ErrInvalidRequest`: Invalid request parameters
  - `ErrModelNotFound`: Requested model not available
- **Example:**
  ```go
  options := types.GenerateOptions{
      Messages: []types.ChatMessage{
          {Role: "user", Content: "Hello, how are you?"},
      },
      Model:       "gpt-4",
      MaxTokens:   1000,
      Temperature: 0.7,
      Stream:      true,
  }

  stream, err := provider.GenerateChatCompletion(ctx, options)
  if err != nil {
      log.Fatalf("Generation failed: %v", err)
  }
  defer stream.Close()

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

**InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error)**

Invokes a server-side tool (provider-specific).

- **Parameters:**
  - `ctx`: Context for timeout control
  - `toolName`: Name of the tool to invoke
  - `params`: Tool parameters
- **Returns:**
  - `interface{}`: Tool execution result
  - `error`: Invocation error or nil on success
- **Note:** Most providers implement tool calling via GenerateChatCompletion
- **Example:**
  ```go
  result, err := provider.InvokeServerTool(ctx, "code_interpreter", params)
  ```

**SupportsToolCalling() bool**

Indicates whether the provider supports function/tool calling.

- **Returns:** `true` if tool calling is supported
- **Example:**
  ```go
  if provider.SupportsToolCalling() {
      // Can use tools
  }
  ```

**SupportsStreaming() bool**

Indicates whether the provider supports streaming responses.

- **Returns:** `true` if streaming is supported
- **Example:**
  ```go
  if provider.SupportsStreaming() {
      options.Stream = true
  }
  ```

**SupportsResponsesAPI() bool**

Indicates whether the provider supports the Responses API.

- **Returns:** `true` if Responses API is supported
- **Note:** Provider-specific feature
- **Example:**
  ```go
  supportsResponses := provider.SupportsResponsesAPI()
  ```

**GetToolFormat() ToolFormat**

Returns the tool calling format used by this provider.

- **Returns:** ToolFormat constant (OpenAI, Anthropic, XML, etc.)
- **Example:**
  ```go
  format := provider.GetToolFormat() // types.ToolFormatOpenAI
  ```

**HealthCheck(ctx context.Context) error**

Performs a health check on the provider.

- **Parameters:**
  - `ctx`: Context for timeout control
- **Returns:**
  - `error`: Health check error or nil if healthy
- **Example:**
  ```go
  if err := provider.HealthCheck(ctx); err != nil {
      log.Printf("Provider unhealthy: %v", err)
  }
  ```

**GetMetrics() ProviderMetrics**

Retrieves current performance metrics for the provider.

- **Returns:** ProviderMetrics with request counts, latency, etc.
- **Example:**
  ```go
  metrics := provider.GetMetrics()
  fmt.Printf("Requests: %d, Errors: %d, Avg Latency: %v\n",
      metrics.RequestCount,
      metrics.ErrorCount,
      metrics.AverageLatency)
  ```

### ChatCompletionStream Interface

Interface for reading streaming chat completion responses.

```go
type ChatCompletionStream interface {
    Next() (ChatCompletionChunk, error)
    Close() error
}
```

**Next() (ChatCompletionChunk, error)**

Reads the next chunk from the stream.

- **Returns:**
  - `ChatCompletionChunk`: Next chunk of data
  - `error`: io.EOF when stream is complete, or other errors
- **Example:**
  ```go
  for {
      chunk, err := stream.Next()
      if err == io.EOF {
          break
      }
      if err != nil {
          log.Fatalf("Stream error: %v", err)
      }

      if chunk.Done {
          break
      }

      fmt.Print(chunk.Content)
  }
  ```

**Close() error**

Closes the stream and releases resources.

- **Returns:** Error if close fails
- **Note:** Always defer Close() after creating a stream
- **Example:**
  ```go
  stream, err := provider.GenerateChatCompletion(ctx, options)
  if err != nil {
      return err
  }
  defer stream.Close()
  ```

---

## Factory Package

The factory package provides provider registration and instantiation.

### ProviderFactory Interface

```go
type ProviderFactory interface {
    RegisterProvider(providerType ProviderType, factoryFunc func(ProviderConfig) Provider)
    CreateProvider(providerType ProviderType, config ProviderConfig) (Provider, error)
    GetSupportedProviders() []ProviderType

    // New interface-specific factory methods (Phase 3)
    CreateModelProvider(providerType ProviderType, config ProviderConfig) (ModelProvider, error)
    CreateChatProvider(providerType ProviderType, config ProviderConfig) (ChatProvider, error)
    CreateCoreProvider(providerType ProviderType, config ProviderConfig) (CoreChatProvider, error)
}
```

### DefaultProviderFactory

Default implementation of ProviderFactory.

**NewProviderFactory() *DefaultProviderFactory**

Creates a new provider factory.

- **Returns:** New DefaultProviderFactory instance
- **Example:**
  ```go
  factory := factory.NewProviderFactory()
  ```

**RegisterProvider(providerType ProviderType, factoryFunc func(ProviderConfig) Provider)**

Registers a provider type with a factory function.

- **Parameters:**
  - `providerType`: Provider type constant
  - `factoryFunc`: Function that creates provider instances
- **Example:**
  ```go
  factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
      return openai.NewOpenAIProvider(config)
  })
  ```

**CreateProvider(providerType ProviderType, config ProviderConfig) (Provider, error)**

Creates a provider instance of the specified type.

- **Parameters:**
  - `providerType`: Type of provider to create
  - `config`: Provider configuration
- **Returns:**
  - `Provider`: New provider instance
  - `error`: Error if provider type not registered
- **Errors:**
  - Returns error if provider type is not registered
- **Example:**
  ```go
  config := types.ProviderConfig{
      Type:   types.ProviderTypeOpenAI,
      APIKey: "sk-your-api-key",
  }

  provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
  if err != nil {
      log.Fatalf("Failed to create provider: %v", err)
  }
  ```

**GetSupportedProviders() []ProviderType**

Returns list of registered provider types.

- **Returns:** Slice of supported ProviderType constants
- **Example:**
  ```go
  providers := factory.GetSupportedProviders()
  for _, pt := range providers {
      fmt.Printf("Supported: %s\n", pt)
  }
  ```

### Helper Functions

**RegisterDefaultProviders(factory *DefaultProviderFactory)**

Registers all built-in providers with the factory.

- **Parameters:**
  - `factory`: Factory to register providers with
- **Providers Registered:**
  - OpenAI
  - Anthropic
  - Gemini
  - Qwen
  - Cerebras
  - OpenRouter
- **Example:**
  ```go
  factory := factory.NewProviderFactory()
  factory.RegisterDefaultProviders(factory)
  ```

---

## Types Package

Core type definitions used throughout the SDK.

### GenerateOptions

Options for generating chat completions.

```go
type GenerateOptions struct {
    Prompt         string                 // User prompt (legacy, use Messages)
    Model          string                 // Per-request model override
    Context        string                 // Additional context
    ContextObj     context.Context        // Internal context for operations
    OutputFile     string                 // Output file path (optional)
    Language       *string                // Language hint (optional)
    ContextFiles   []string               // Files for context (optional)
    Messages       []ChatMessage          // Chat messages (primary input)
    MaxTokens      int                    // Maximum tokens to generate
    Temperature    float64                // Sampling temperature (0.0-2.0)
    Stop           []string               // Stop sequences
    Stream         bool                   // Enable streaming
    Tools          []Tool                 // Available tools
    ToolChoice     *ToolChoice            // Tool selection control
    ResponseFormat string                 // Response format (e.g., "json")
    Timeout        time.Duration          // Request timeout
    Metadata       map[string]interface{} // Additional metadata
}
```

**Field Descriptions:**

- **Prompt**: Legacy single-turn prompt (prefer Messages for chat)
- **Model**: Override the default model for this request
- **Messages**: Array of chat messages (user, assistant, system, tool)
- **MaxTokens**: Maximum tokens in the response (0 = provider default)
- **Temperature**: Controls randomness (0.0 = deterministic, 2.0 = very random)
- **Stop**: Sequences where generation should stop
- **Stream**: If true, returns streaming response
- **Tools**: Function/tool definitions available to the model
- **ToolChoice**: Control which tools can be used
- **ResponseFormat**: Structured output format (provider-specific)
- **Timeout**: Override default timeout for this request
- **Metadata**: Custom metadata passed through to callbacks

**Example:**

```go
options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "system", Content: "You are a helpful assistant."},
        {Role: "user", Content: "What's the weather in Paris?"},
    },
    Model:       "gpt-4",
    MaxTokens:   1000,
    Temperature: 0.7,
    Stream:      true,
    Tools: []types.Tool{
        {
            Name:        "get_weather",
            Description: "Get current weather for a location",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "location": map[string]interface{}{
                        "type":        "string",
                        "description": "City name",
                    },
                },
                "required": []string{"location"},
            },
        },
    },
    ToolChoice: &types.ToolChoice{
        Mode: types.ToolChoiceAuto,
    },
}
```

### ChatMessage

Represents a single message in a conversation.

```go
type ChatMessage struct {
    Role       string                 // "user", "assistant", "system", "tool"
    Content    string                 // Message content
    ToolCalls  []ToolCall             // Tool calls made by assistant
    ToolCallID string                 // ID for tool result messages
    Metadata   map[string]interface{} // Additional metadata
}
```

**Roles:**

- **"system"**: System-level instructions
- **"user"**: User messages
- **"assistant"**: AI assistant responses
- **"tool"**: Tool execution results

**Example:**

```go
messages := []types.ChatMessage{
    {
        Role:    "system",
        Content: "You are a helpful coding assistant.",
    },
    {
        Role:    "user",
        Content: "Write a Hello World in Go",
    },
    {
        Role:    "assistant",
        Content: "Here's a simple Hello World program in Go...",
    },
}
```

### Tool

Defines a function/tool available to the model.

```go
type Tool struct {
    Name        string                 // Tool name/identifier
    Description string                 // Human-readable description
    InputSchema map[string]interface{} // JSON Schema for parameters
}
```

**Example:**

```go
tool := types.Tool{
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
}
```

### ToolCall

Represents a tool invocation by the model.

```go
type ToolCall struct {
    ID       string                 // Unique call identifier
    Type     string                 // "function" (standard type)
    Function ToolCallFunction       // Function details
    Metadata map[string]interface{} // Additional metadata
}

type ToolCallFunction struct {
    Name      string // Function name
    Arguments string // JSON-encoded arguments
}
```

**Example:**

```go
// Received in assistant message
toolCall := types.ToolCall{
    ID:   "call_abc123",
    Type: "function",
    Function: types.ToolCallFunction{
        Name:      "get_weather",
        Arguments: `{"location": "Paris"}`,
    },
}

// Parse arguments
var args map[string]interface{}
json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
location := args["location"].(string)

// Execute function and submit result
result := getWeather(location)
resultMessage := types.ChatMessage{
    Role:       "tool",
    ToolCallID: toolCall.ID,
    Content:    result,
}
```

### ToolChoice

Controls tool selection behavior.

```go
type ToolChoice struct {
    Mode         ToolChoiceMode // Selection mode
    FunctionName string         // Specific function (for "specific" mode)
}

type ToolChoiceMode string

const (
    ToolChoiceAuto     ToolChoiceMode = "auto"     // Model decides
    ToolChoiceRequired ToolChoiceMode = "required" // Must use a tool
    ToolChoiceNone     ToolChoiceMode = "none"     // Don't use tools
    ToolChoiceSpecific ToolChoiceMode = "specific" // Force specific tool
)
```

**Modes:**

- **Auto**: Model decides whether to use tools
- **Required**: Model must call at least one tool
- **None**: Model cannot use tools
- **Specific**: Force a specific tool to be called

**Example:**

```go
// Let model decide
autoChoice := &types.ToolChoice{
    Mode: types.ToolChoiceAuto,
}

// Require tool use
requiredChoice := &types.ToolChoice{
    Mode: types.ToolChoiceRequired,
}

// Force specific tool
specificChoice := &types.ToolChoice{
    Mode:         types.ToolChoiceSpecific,
    FunctionName: "get_weather",
}
```

### ChatCompletionChunk

Represents a chunk in a streaming response.

```go
type ChatCompletionChunk struct {
    ID      string       // Completion ID
    Object  string       // "chat.completion.chunk"
    Created int64        // Unix timestamp
    Model   string       // Model used
    Choices []ChatChoice // Response choices
    Usage   Usage        // Token usage (in final chunk)
    Done    bool         // True if this is the final chunk
    Content string       // Accumulated content
    Error   string       // Error message if present
}

type ChatChoice struct {
    Index        int         // Choice index
    Message      ChatMessage // Complete message (non-streaming)
    FinishReason string      // Reason for completion
    Delta        ChatMessage // Delta for streaming
}
```

**FinishReason Values:**

- **"stop"**: Natural completion
- **"length"**: Max tokens reached
- **"tool_calls"**: Model wants to call a tool
- **"content_filter"**: Content filtered
- **""**: Not finished yet (streaming)

**Example:**

```go
stream, _ := provider.GenerateChatCompletion(ctx, options)
defer stream.Close()

var fullContent string
for {
    chunk, err := stream.Next()
    if err == io.EOF || chunk.Done {
        break
    }
    if err != nil {
        log.Fatalf("Error: %v", err)
    }

    fullContent += chunk.Content
    fmt.Print(chunk.Content)

    if chunk.Done {
        fmt.Printf("\n\nUsage: %d tokens\n", chunk.Usage.TotalTokens)
    }
}
```

### Model

Represents an AI model with capabilities and pricing.

```go
type Model struct {
    ID                   string       // Model identifier
    Name                 string       // Human-readable name
    Provider             ProviderType // Provider type
    Description          string       // Model description
    MaxTokens            int          // Maximum context tokens
    InputTokens          int          // Max input tokens
    OutputTokens         int          // Max output tokens
    SupportsStreaming    bool         // Streaming support
    SupportsToolCalling  bool         // Tool calling support
    SupportsResponsesAPI bool         // Responses API support
    Capabilities         []string     // Feature list
    Pricing              Pricing      // Pricing information
}

type Pricing struct {
    InputTokenPrice  float64 // Price per input token
    OutputTokenPrice float64 // Price per output token
    Unit             string  // Pricing unit (e.g., "1M tokens")
}
```

**Example:**

```go
models, _ := provider.GetModels(ctx)
for _, model := range models {
    fmt.Printf("Model: %s\n", model.Name)
    fmt.Printf("  Max Tokens: %d\n", model.MaxTokens)
    fmt.Printf("  Streaming: %v\n", model.SupportsStreaming)
    fmt.Printf("  Tool Calling: %v\n", model.SupportsToolCalling)
    fmt.Printf("  Input Price: $%.6f per token\n", model.Pricing.InputTokenPrice)
    fmt.Printf("  Output Price: $%.6f per token\n", model.Pricing.OutputTokenPrice)
}
```

### Usage

Token usage information.

```go
type Usage struct {
    PromptTokens     int // Input tokens
    CompletionTokens int // Output tokens
    TotalTokens      int // Total tokens
}
```

**Example:**

```go
// Get usage from final chunk
for {
    chunk, err := stream.Next()
    if err == io.EOF || chunk.Done {
        if chunk.Usage.TotalTokens > 0 {
            cost := float64(chunk.Usage.PromptTokens)*model.Pricing.InputTokenPrice +
                    float64(chunk.Usage.CompletionTokens)*model.Pricing.OutputTokenPrice
            fmt.Printf("Tokens used: %d (cost: $%.6f)\n", chunk.Usage.TotalTokens, cost)
        }
        break
    }
}
```

### ProviderConfig

Configuration for a provider instance.

```go
type ProviderConfig struct {
    Type           ProviderType           // Provider type
    Name           string                 // Provider name
    BaseURL        string                 // API base URL (optional)
    APIKey         string                 // API key
    APIKeyEnv      string                 // Environment variable for API key
    DefaultModel   string                 // Default model ID
    Description    string                 // Provider description
    ProviderConfig map[string]interface{} // Provider-specific config

    // OAuth configuration
    OAuthCredentials []*OAuthCredentialSet // OAuth credentials (multi-OAuth)

    // Feature flags
    SupportsStreaming    bool // Streaming support
    SupportsToolCalling  bool // Tool calling support
    SupportsResponsesAPI bool // Responses API support

    // Limits
    MaxTokens int           // Default max tokens
    Timeout   time.Duration // Request timeout

    // Tool format
    ToolFormat ToolFormat // Tool calling format
}
```

**Example:**

```go
config := types.ProviderConfig{
    Type:                 types.ProviderTypeOpenAI,
    Name:                 "openai",
    APIKey:               os.Getenv("OPENAI_API_KEY"),
    DefaultModel:         "gpt-4",
    SupportsStreaming:    true,
    SupportsToolCalling:  true,
    SupportsResponsesAPI: false,
    MaxTokens:            4096,
    Timeout:              60 * time.Second,
    ToolFormat:           types.ToolFormatOpenAI,
}
```

### AuthConfig

Authentication configuration.

```go
type AuthConfig struct {
    Method       AuthMethod // Authentication method
    APIKey       string     // API key
    BaseURL      string     // Base URL override
    DefaultModel string     // Default model
}
```

**Example:**

```go
authConfig := types.AuthConfig{
    Method: types.AuthMethodAPIKey,
    APIKey: "sk-your-api-key",
}

err := provider.Authenticate(ctx, authConfig)
```

### OAuthCredentialSet

OAuth credentials for multi-OAuth support.

```go
type OAuthCredentialSet struct {
    ID           string    // Unique credential identifier
    ClientID     string    // OAuth client ID
    ClientSecret string    // OAuth client secret
    AccessToken  string    // Current access token
    RefreshToken string    // Refresh token
    ExpiresAt    time.Time // Token expiration
    Scopes       []string  // OAuth scopes
    LastRefresh  time.Time // Last refresh time
    RefreshCount int       // Number of refreshes

    // Callback for token refresh
    OnTokenRefresh func(id, accessToken, refreshToken string, expiresAt time.Time) error
}
```

**Example:**

```go
oauthCred := &types.OAuthCredentialSet{
    ID:           "account-1",
    ClientID:     "your-client-id",
    ClientSecret: "your-client-secret",
    AccessToken:  "current-access-token",
    RefreshToken: "refresh-token",
    ExpiresAt:    time.Now().Add(1 * time.Hour),
    Scopes:       []string{"profile", "email"},
    OnTokenRefresh: func(id, accessToken, refreshToken string, expiresAt time.Time) error {
        // Persist new tokens
        return saveTokens(id, accessToken, refreshToken, expiresAt)
    },
}

config := types.ProviderConfig{
    Type:             types.ProviderTypeAnthropic,
    OAuthCredentials: []*types.OAuthCredentialSet{oauthCred},
}
```

### ProviderMetrics

Performance metrics for a provider.

```go
type ProviderMetrics struct {
    RequestCount    int64         // Total requests
    SuccessCount    int64         // Successful requests
    ErrorCount      int64         // Failed requests
    TotalLatency    time.Duration // Cumulative latency
    AverageLatency  time.Duration // Average latency
    LastRequestTime time.Time     // Last request timestamp
    LastSuccessTime time.Time     // Last success timestamp
    LastErrorTime   time.Time     // Last error timestamp
    LastError       string        // Last error message
    TokensUsed      int64         // Total tokens consumed
    HealthStatus    HealthStatus  // Current health status
}

type HealthStatus struct {
    Healthy      bool      // Health status
    LastChecked  time.Time // Last check timestamp
    Message      string    // Status message
    ResponseTime float64   // Response time (ms)
    StatusCode   int       // HTTP status code
}
```

**Example:**

```go
metrics := provider.GetMetrics()
fmt.Printf("Requests: %d (Success: %d, Errors: %d)\n",
    metrics.RequestCount,
    metrics.SuccessCount,
    metrics.ErrorCount)

successRate := float64(metrics.SuccessCount) / float64(metrics.RequestCount) * 100
fmt.Printf("Success Rate: %.2f%%\n", successRate)
fmt.Printf("Average Latency: %v\n", metrics.AverageLatency)
fmt.Printf("Tokens Used: %d\n", metrics.TokensUsed)

if !metrics.HealthStatus.Healthy {
    fmt.Printf("Warning: Provider unhealthy - %s\n", metrics.HealthStatus.Message)
}
```

### ProviderInfo

Information about a provider.

```go
type ProviderInfo struct {
    Name           string       // Provider name
    Type           ProviderType // Provider type
    Description    string       // Description
    HealthStatus   HealthStatus // Health status
    Models         []Model      // Available models
    SupportedTools []string     // Supported tools
    DefaultModel   string       // Default model
}
```

---

## Provider Methods

Detailed information about common provider method implementations.

### Creating a Provider

```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/openai"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Method 1: Direct instantiation
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "sk-your-api-key",
}
provider := openai.NewOpenAIProvider(config)

// Method 2: Using factory
factory := factory.NewProviderFactory()
factory.RegisterDefaultProviders(factory)
provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
```

### Chat Completion (Non-Streaming)

```go
ctx := context.Background()

options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "Explain quantum computing"},
    },
    Model:       "gpt-4",
    MaxTokens:   500,
    Temperature: 0.7,
    Stream:      false,
}

stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    log.Fatalf("Error: %v", err)
}
defer stream.Close()

// Read single response
chunk, err := stream.Next()
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Println(chunk.Content)
fmt.Printf("Tokens: %d\n", chunk.Usage.TotalTokens)
```

### Chat Completion (Streaming)

```go
options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "Write a poem about AI"},
    },
    Stream: true,
}

stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    log.Fatalf("Error: %v", err)
}
defer stream.Close()

var fullResponse string
for {
    chunk, err := stream.Next()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatalf("Stream error: %v", err)
    }

    if chunk.Done {
        fmt.Printf("\n\nTotal tokens: %d\n", chunk.Usage.TotalTokens)
        break
    }

    fullResponse += chunk.Content
    fmt.Print(chunk.Content)
}
```

### Multi-Turn Conversation

```go
messages := []types.ChatMessage{
    {Role: "system", Content: "You are a helpful assistant."},
    {Role: "user", Content: "What's the capital of France?"},
}

// First turn
options := types.GenerateOptions{Messages: messages}
stream, _ := provider.GenerateChatCompletion(ctx, options)
chunk, _ := stream.Next()
stream.Close()

// Add assistant response
messages = append(messages, types.ChatMessage{
    Role:    "assistant",
    Content: chunk.Content,
})

// Second turn
messages = append(messages, types.ChatMessage{
    Role:    "user",
    Content: "What's its population?",
})

options = types.GenerateOptions{Messages: messages}
stream, _ = provider.GenerateChatCompletion(ctx, options)
defer stream.Close()
// ... process response
```

### Model Discovery

```go
ctx := context.Background()

models, err := provider.GetModels(ctx)
if err != nil {
    log.Fatalf("Failed to get models: %v", err)
}

// Filter by capability
var streamingModels []types.Model
for _, model := range models {
    if model.SupportsStreaming {
        streamingModels = append(streamingModels, model)
    }
}

// Sort by max tokens
sort.Slice(models, func(i, j int) bool {
    return models[i].MaxTokens > models[j].MaxTokens
})

// Find cheapest model
cheapest := models[0]
for _, model := range models {
    if model.Pricing.InputTokenPrice < cheapest.Pricing.InputTokenPrice {
        cheapest = model
    }
}
```

### Per-Request Model Override

```go
// Provider configured with default model
config := types.ProviderConfig{
    Type:         types.ProviderTypeOpenAI,
    APIKey:       "key",
    DefaultModel: "gpt-3.5-turbo",
}
provider := openai.NewOpenAIProvider(config)

// Override for specific request
options := types.GenerateOptions{
    Model:    "gpt-4", // Override default
    Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
}

stream, _ := provider.GenerateChatCompletion(ctx, options)
```

---

## Streaming API

### Stream Reading Patterns

**Pattern 1: Simple Loop**

```go
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    return err
}
defer stream.Close()

for {
    chunk, err := stream.Next()
    if err == io.EOF || chunk.Done {
        break
    }
    if err != nil {
        return err
    }

    fmt.Print(chunk.Content)
}
```

**Pattern 2: With Buffering**

```go
var buffer strings.Builder

for {
    chunk, err := stream.Next()
    if err == io.EOF || chunk.Done {
        break
    }
    if err != nil {
        return err
    }

    buffer.WriteString(chunk.Content)

    // Flush buffer periodically
    if buffer.Len() > 1024 {
        fmt.Print(buffer.String())
        buffer.Reset()
    }
}

// Flush remaining
if buffer.Len() > 0 {
    fmt.Print(buffer.String())
}
```

**Pattern 3: With Channel**

```go
chunks := make(chan types.ChatCompletionChunk, 10)
errors := make(chan error, 1)

go func() {
    defer close(chunks)
    defer close(errors)

    for {
        chunk, err := stream.Next()
        if err == io.EOF || chunk.Done {
            return
        }
        if err != nil {
            errors <- err
            return
        }
        chunks <- chunk
    }
}()

// Process chunks
for {
    select {
    case chunk, ok := <-chunks:
        if !ok {
            return nil
        }
        fmt.Print(chunk.Content)
    case err := <-errors:
        return err
    }
}
```

### Error Handling in Streams

```go
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    // Handle creation error
    if apiErr, ok := err.(*http.APIError); ok {
        if apiErr.StatusCode == 429 {
            // Rate limited
            time.Sleep(time.Minute)
            // Retry
        }
    }
    return err
}
defer stream.Close()

for {
    chunk, err := stream.Next()
    if err == io.EOF {
        // Normal end of stream
        break
    }
    if err != nil {
        // Stream error
        log.Printf("Stream error: %v", err)
        return err
    }

    if chunk.Error != "" {
        // Provider-specific error in chunk
        log.Printf("Chunk error: %s", chunk.Error)
        return fmt.Errorf("stream error: %s", chunk.Error)
    }

    if chunk.Done {
        // Final chunk
        break
    }

    // Process chunk
    fmt.Print(chunk.Content)
}
```

### Stream Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

options.ContextObj = ctx
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    return err
}
defer stream.Close()

// Stream will be cancelled after 30 seconds or on manual cancel()
for {
    chunk, err := stream.Next()
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            log.Println("Stream timeout")
        }
        break
    }
    // Process...
}
```

---

## Tool Calling API

### Defining Tools

```go
tools := []types.Tool{
    {
        Name:        "get_weather",
        Description: "Get the current weather in a given location",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type":        "string",
                    "description": "The city and state, e.g. San Francisco, CA",
                },
                "unit": map[string]interface{}{
                    "type":        "string",
                    "enum":        []string{"celsius", "fahrenheit"},
                    "description": "The temperature unit to use",
                },
            },
            "required": []string{"location"},
        },
    },
    {
        Name:        "search_web",
        Description: "Search the web for information",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "query": map[string]interface{}{
                    "type":        "string",
                    "description": "The search query",
                },
                "num_results": map[string]interface{}{
                    "type":        "integer",
                    "description": "Number of results to return",
                    "default":     5,
                },
            },
            "required": []string{"query"},
        },
    },
}
```

### Tool Calling Flow

```go
// 1. Initial request with tools
options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "What's the weather in Tokyo?"},
    },
    Tools:      tools,
    ToolChoice: &types.ToolChoice{Mode: types.ToolChoiceAuto},
}

stream, _ := provider.GenerateChatCompletion(ctx, options)
chunk, _ := stream.Next()
stream.Close()

// 2. Check for tool calls
if len(chunk.Choices) > 0 && len(chunk.Choices[0].Message.ToolCalls) > 0 {
    // 3. Execute tools
    messages := append(options.Messages, chunk.Choices[0].Message)

    for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
        // Parse arguments
        var args map[string]interface{}
        json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

        // Execute function
        result := executeTool(toolCall.Function.Name, args)

        // 4. Submit result
        messages = append(messages, types.ChatMessage{
            Role:       "tool",
            ToolCallID: toolCall.ID,
            Content:    result,
        })
    }

    // 5. Continue conversation with tool results
    options.Messages = messages
    stream, _ = provider.GenerateChatCompletion(ctx, options)
    finalChunk, _ := stream.Next()
    stream.Close()

    fmt.Println(finalChunk.Content)
}
```

### Tool Implementation Example

```go
func executeTool(name string, args map[string]interface{}) string {
    switch name {
    case "get_weather":
        location := args["location"].(string)
        unit := "fahrenheit"
        if u, ok := args["unit"].(string); ok {
            unit = u
        }

        // Call weather API
        weather := getWeatherAPI(location, unit)

        // Return JSON result
        result, _ := json.Marshal(map[string]interface{}{
            "location":    location,
            "temperature": weather.Temp,
            "unit":        unit,
            "conditions":  weather.Conditions,
        })
        return string(result)

    case "search_web":
        query := args["query"].(string)
        numResults := 5
        if n, ok := args["num_results"].(float64); ok {
            numResults = int(n)
        }

        // Perform search
        results := searchWeb(query, numResults)

        resultJSON, _ := json.Marshal(results)
        return string(resultJSON)

    default:
        return fmt.Sprintf(`{"error": "Unknown tool: %s"}`, name)
    }
}
```

### Forcing Specific Tool

```go
// Force model to use get_weather tool
options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "What's the weather?"},
    },
    Tools: tools,
    ToolChoice: &types.ToolChoice{
        Mode:         types.ToolChoiceSpecific,
        FunctionName: "get_weather",
    },
}
```

### Parallel Tool Calls

```go
stream, _ := provider.GenerateChatCompletion(ctx, options)
chunk, _ := stream.Next()
stream.Close()

if len(chunk.Choices[0].Message.ToolCalls) > 0 {
    // Execute tools in parallel
    var wg sync.WaitGroup
    results := make([]types.ChatMessage, len(chunk.Choices[0].Message.ToolCalls))

    for i, toolCall := range chunk.Choices[0].Message.ToolCalls {
        wg.Add(1)
        go func(idx int, tc types.ToolCall) {
            defer wg.Done()

            var args map[string]interface{}
            json.Unmarshal([]byte(tc.Function.Arguments), &args)

            result := executeTool(tc.Function.Name, args)

            results[idx] = types.ChatMessage{
                Role:       "tool",
                ToolCallID: tc.ID,
                Content:    result,
            }
        }(i, toolCall)
    }

    wg.Wait()

    // Continue with all results
    messages := append(options.Messages, chunk.Choices[0].Message)
    messages = append(messages, results...)

    options.Messages = messages
    stream, _ = provider.GenerateChatCompletion(ctx, options)
    // ... process final response
}
```

---

## Authentication Package

The `auth` package provides comprehensive authentication management.

### Authenticator Interface

```go
type Authenticator interface {
    Authenticate(ctx context.Context, config types.AuthConfig) error
    IsAuthenticated() bool
    GetToken() (string, error)
    RefreshToken(ctx context.Context) error
    Logout(ctx context.Context) error
    GetAuthMethod() types.AuthMethod
}
```

### API Key Authentication

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/auth"

// Single API key
authConfig := types.AuthConfig{
    Method: types.AuthMethodAPIKey,
    APIKey: "sk-your-api-key",
}

err := provider.Authenticate(ctx, authConfig)
```

### Multi-Key Management

```go
config := &auth.APIKeyConfig{
    Strategy: "round_robin",
    Health: auth.HealthConfig{
        Enabled:          true,
        FailureThreshold: 3,
        SuccessThreshold: 2,
        Backoff: auth.BackoffConfig{
            Initial:   1 * time.Second,
            Maximum:   60 * time.Second,
            Multiplier: 2.0,
            Jitter:    true,
        },
    },
    Failover: auth.FailoverConfig{
        Enabled:     true,
        MaxAttempts: 3,
    },
}

keys := []string{
    "sk-key1",
    "sk-key2",
    "sk-key3",
}

manager, err := auth.NewAPIKeyManager("openai", keys, config)
if err != nil {
    log.Fatal(err)
}

// Execute with automatic failover
result, err := manager.ExecuteWithFailover(func(apiKey string) (string, error) {
    // Make API call with this key
    return makeAPICall(apiKey)
})
```

### OAuth Authentication

```go
// Create OAuth authenticator
storage := auth.NewMemoryTokenStorage(nil)
oauthConfig := &auth.OAuthConfig{
    DefaultScopes: []string{"profile", "email"},
    PKCE: auth.PKCEConfig{
        Enabled: true,
        Method:  "S256",
    },
}

authenticator := auth.NewOAuthAuthenticator("google", storage, oauthConfig)

// Start OAuth flow
authURL, err := authenticator.StartOAuthFlow(ctx, []string{"profile"})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Visit: %s\n", authURL)

// Handle callback (in your HTTP handler)
err = authenticator.HandleCallback(ctx, code, state)
if err != nil {
    log.Fatal(err)
}

// Get token
token, err := authenticator.GetToken()
```

### AuthManager

```go
// Create auth manager
config := auth.DefaultConfig()
storage := auth.NewMemoryTokenStorage(&config.TokenStorage.Memory)
authManager := auth.NewAuthManager(storage, config)

// Register authenticators
openaiAuth := auth.NewAPIKeyAuthenticator("openai", "sk-key", nil)
authManager.RegisterAuthenticator("openai", openaiAuth)

anthropicAuth := auth.NewOAuthAuthenticator("anthropic", storage, &config.OAuth)
authManager.RegisterAuthenticator("anthropic", anthropicAuth)

// Check auth status
status := authManager.GetAuthStatus()
for provider, state := range status {
    fmt.Printf("%s: authenticated=%v\n", provider, state.Authenticated)
}

// Refresh all tokens
err := authManager.RefreshAllTokens(ctx)
```

---

## Common Utilities Package (Phase 2)

The `pkg/providers/common` package provides comprehensive shared utilities used across all provider implementations to reduce code duplication and ensure consistent behavior. Phase 2 introduces powerful new utilities for streaming, authentication, and configuration management.

### Streaming Abstraction

#### StreamProcessor

Provides common streaming functionality for all providers with unified interface.

```go
type StreamProcessor struct {
    response *http.Response
    reader   *bufio.Reader
    done     bool
    mutex    sync.Mutex
}
```

**NewStreamProcessor**

```go
func NewStreamProcessor(response *http.Response) *StreamProcessor
```

**Parameters:**
- `response`: HTTP response containing streaming data

**Returns:** New StreamProcessor instance

**Example:**
```go
resp, err := client.Do(req)
if err != nil {
    return err
}
processor := common.NewStreamProcessor(resp)
```

**NextChunk**

```go
func (sp *StreamProcessor) NextChunk(processLine ProcessLineFunc) (types.ChatCompletionChunk, error)
```

**Parameters:**
- `processLine`: Function to process each line from the stream

**Returns:** Chat completion chunk and any error

**Example:**
```go
chunk, err := processor.NextChunk(func(line string) (types.ChatCompletionChunk, error, bool) {
    // Process streaming line
    return chunk, nil, false
})
```

**Standard Stream Parsers**

```go
// OpenAI-compatible streaming
func CreateOpenAIStream(response *http.Response) types.ChatCompletionStream

// Anthropic streaming
func CreateAnthropicStream(response *http.Response) types.ChatCompletionStream

// Custom streaming with parser
func CreateCustomStream(response *http.Response, parser StreamParser) types.ChatCompletionStream
```

**Example:**
```go
// OpenAI-compatible streaming
stream := common.CreateOpenAIStream(resp)
defer stream.Close()

for {
    chunk, err := stream.Next()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    fmt.Print(chunk.Content)
}
```

#### StandardStreamParser

Provides standard parsing for OpenAI-compatible streaming responses.

```go
type StandardStreamParser struct {
    ContentField    string
    DoneField       string
    UsageField      string
    ToolCallsField  string
    FinishReason    string
}
```

**NewStandardStreamParser**

```go
func NewStandardStreamParser() *StandardStreamParser
```

**Example:**
```go
parser := common.NewStandardStreamParser()
// Customize field mappings for provider-specific responses
parser.ContentField = "choices.0.delta.content"
parser.ToolCallsField = "choices.0.delta.tool_calls"
```

#### AnthropicStreamParser

Specialized parser for Anthropic's streaming format.

```go
func NewAnthropicStreamParser() *AnthropicStreamParser
```

**Example:**
```go
// Handle Anthropic streaming events
stream := common.CreateAnthropicStream(resp)
chunk, err := stream.Next()
if err == nil && chunk.Content != "" {
    fmt.Print(chunk.Content) // Real-time content
}
```

### Authentication Helpers

#### AuthHelper

Provides unified authentication functionality supporting both API keys and OAuth with automatic failover.

```go
type AuthHelper struct {
    ProviderName    string
    KeyManager      *keymanager.KeyManager
    OAuthManager    *oauthmanager.OAuthKeyManager
    HTTPClient      *http.Client
    Config          types.ProviderConfig
}
```

**NewAuthHelper**

```go
func NewAuthHelper(providerName string, config types.ProviderConfig, client *http.Client) *AuthHelper
```

**SetupAPIKeys**

```go
func (h *AuthHelper) SetupAPIKeys()
```

Extracts API keys from both single and multi-key configurations.

**Example:**
```go
config := types.ProviderConfig{
    APIKey: "single-key",
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{"key1", "key2", "key3"},
    },
}

helper := common.NewAuthHelper("openai", config, client)
helper.SetupAPIKeys() // Automatically configures key management
```

**SetupOAuth**

```go
func (h *AuthHelper) SetupOAuth(refreshFunc oauthmanager.RefreshFunc)
```

Configures OAuth management with multiple credential support.

**Example:**
```go
helper.SetupOAuth(func(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
    // Custom refresh logic
    return refreshOAuthToken(ctx, cred)
})
```

**ExecuteWithAuth**

```go
func (h *AuthHelper) ExecuteWithAuth(
    ctx context.Context,
    options types.GenerateOptions,
    oauthOperation func(context.Context, *types.OAuthCredentialSet) (string, *types.Usage, error),
    apiKeyOperation func(context.Context, string) (string, *types.Usage, error),
) (string, *types.Usage, error)
```

Executes operations using available authentication methods with automatic failover.

**Example:**
```go
result, usage, err := authHelper.ExecuteWithAuth(ctx, options,
    func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
        return makeOAuthRequest(ctx, cred)
    },
    func(ctx context.Context, key string) (string, *types.Usage, error) {
        return makeAPIKeyRequest(ctx, key)
    },
)
// Automatically tries OAuth first, then API keys with failover
```

**SetAuthHeaders**

```go
func (h *AuthHelper) SetAuthHeaders(req *http.Request, authToken string, authType string)
```

Sets appropriate authentication headers for different providers.

**Example:**
```go
helper.SetAuthHeaders(req, "sk-key123", "api_key")
// Sets Authorization: Bearer sk-key123 for OpenAI
// Sets x-api-key: sk-key123 for Anthropic
// Sets x-goog-api-key: sk-key123 for Gemini
```

**GetAuthStatus**

```go
func (h *AuthHelper) GetAuthStatus() map[string]interface{}
```

Returns detailed authentication status including configured methods.

**Example:**
```go
status := helper.GetAuthStatus()
fmt.Printf("Authenticated: %v\n", status["authenticated"])
fmt.Printf("Method: %s\n", status["method"])
fmt.Printf("OAuth credentials: %d\n", status["oauth_credentials_count"])
```

### OAuth Token Refresh

#### OAuthRefreshHelper

Provides pre-built OAuth token refresh implementations for major providers.

```go
type OAuthRefreshHelper struct {
    ProviderName string
    HTTPClient   *http.Client
}
```

**NewOAuthRefreshHelper**

```go
func NewOAuthRefreshHelper(providerName string, client *http.Client) *OAuthRefreshHelper
```

**Provider-Specific Refresh Methods**

```go
// Anthropic OAuth refresh
func (h *OAuthRefreshHelper) AnthropicOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error)

// OpenAI OAuth refresh
func (h *OAuthRefreshHelper) OpenAIOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error)

// Gemini OAuth refresh
func (h *OAuthRefreshHelper) GeminiOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error)

// Qwen OAuth refresh
func (h *OAuthRefreshHelper) QwenOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error)

// Generic OAuth refresh
func (h *OAuthRefreshHelper) GenericOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet, tokenURL string) (*types.OAuthCredentialSet, error)
```

**Example:**
```go
helper := common.NewOAuthRefreshHelper("anthropic", client)

// Refresh Anthropic token
updatedCred, err := helper.AnthropicOAuthRefresh(ctx, credential)
if err != nil {
    return fmt.Errorf("failed to refresh token: %w", err)
}

// Use RefreshFuncFactory for easy integration
factory := common.NewRefreshFuncFactory("openai", client)
refreshFunc := factory.CreateOpenAIRefreshFunc()
oauthManager := oauthmanager.NewOAuthKeyManager("openai", credentials, refreshFunc)
```

#### RefreshFuncFactory

Creates refresh functions for different providers.

```go
type RefreshFuncFactory struct {
    Helper *OAuthRefreshHelper
}
```

**Example:**
```go
factory := common.NewRefreshFuncFactory("anthropic", client)

// Create provider-specific refresh functions
anthropicRefresh := factory.CreateAnthropicRefreshFunc()
openaiRefresh := factory.CreateOpenAIRefreshFunc()
geminiRefresh := factory.CreateGeminiRefreshFunc()

// Or create generic refresh for custom providers
genericRefresh := factory.CreateGenericRefreshFunc("https://api.example.com/oauth/token")
```


### Configuration Helper

#### ConfigHelper

Provides standardized configuration validation, extraction, and defaults for all AI providers.

```go
type ConfigHelper struct {
    providerName string
    providerType types.ProviderType
}
```

**NewConfigHelper**

```go
func NewConfigHelper(providerName string, providerType types.ProviderType) *ConfigHelper
```

**ValidateProviderConfig**

```go
func (h *ConfigHelper) ValidateProviderConfig(config types.ProviderConfig) ValidationResult
```

**Example:**
```go
helper := common.NewConfigHelper("openai", types.ProviderTypeOpenAI)
result := helper.ValidateProviderConfig(config)

if !result.Valid {
    for _, err := range result.Errors {
        log.Printf("Config error: %s", err)
    }
}
```

**Configuration Extraction Methods**

```go
// Extract API keys from various sources
func (h *ConfigHelper) ExtractAPIKeys(config types.ProviderConfig) []string

// Extract base URL with provider-specific defaults
func (h *ConfigHelper) ExtractBaseURL(config types.ProviderConfig) string

// Extract default model with provider fallbacks
func (h *ConfigHelper) ExtractDefaultModel(config types.ProviderConfig) string

// Extract timeout with sensible defaults
func (h *ConfigHelper) ExtractTimeout(config types.ProviderConfig) time.Duration

// Extract max tokens with provider defaults
func (h *ConfigHelper) ExtractMaxTokens(config types.ProviderConfig) int

// Extract provider-specific configuration
func (h *ConfigHelper) ExtractProviderSpecificConfig(config types.ProviderConfig, target interface{}) error

// Extract string field with fallback
func (h *ConfigHelper) ExtractStringField(config types.ProviderConfig, fieldName, fallback string) string

// Extract string slice field
func (h *ConfigHelper) ExtractStringSliceField(config types.ProviderConfig, fieldName string) []string
```

**Example:**
```go
helper := common.NewConfigHelper("openai", types.ProviderTypeOpenAI)

// Extract configuration with defaults
baseURL := helper.ExtractBaseURL(config)         // "https://api.openai.com/v1" if not set
defaultModel := helper.ExtractDefaultModel(config) // "gpt-4o" if not set
timeout := helper.ExtractTimeout(config)         // 60s if not set
maxTokens := helper.ExtractMaxTokens(config)     // 4096 if not set

// Extract API keys from multiple sources
keys := helper.ExtractAPIKeys(config)
// Returns: ["sk-single", "sk-multi1", "sk-multi2", ...]

// Extract provider-specific config
var openAIConfig struct {
    OrganizationID string `json:"organization_id"`
    ProjectID      string `json:"project_id"`
}
err := helper.ExtractProviderSpecificConfig(config, &openAIConfig)
```

**Configuration Management Methods**

```go
// Apply top-level overrides to provider config
func (h *ConfigHelper) ApplyTopLevelOverrides(config types.ProviderConfig, providerConfig interface{}) error

// Get default OAuth client ID for provider
func (h *ConfigHelper) ExtractDefaultOAuthClientID() string

// Get provider capabilities
func (h *ConfigHelper) GetProviderCapabilities() (supportsToolCalling, supportsStreaming, supportsResponsesAPI bool)

// Sanitize config for logging (remove sensitive data)
func (h *ConfigHelper) SanitizeConfigForLogging(config types.ProviderConfig) types.ProviderConfig

// Get human-readable config summary
func (h *ConfigHelper) ConfigSummary(config types.ProviderConfig) map[string]interface{}

// Merge config with provider defaults
func (h *ConfigHelper) MergeWithDefaults(config types.ProviderConfig) types.ProviderConfig
```

**Example:**
```go
helper := common.NewConfigHelper("anthropic", types.ProviderTypeAnthropic)

// Get provider capabilities
toolCalling, streaming, responsesAPI := helper.GetProviderCapabilities()
// Returns: true, true, false for Anthropic

// Sanitize config for logging
safeConfig := helper.SanitizeConfigForLogging(config)
log.Printf("Config: %+v", safeConfig) // API keys and OAuth tokens removed

// Get config summary
summary := helper.ConfigSummary(config)
fmt.Printf("Provider: %s\n", summary["provider"])
fmt.Printf("Auth methods: %v\n", summary["auth_methods"])
fmt.Printf("Capabilities: %v\n", summary["capabilities"])

// Merge with defaults
completeConfig := helper.MergeWithDefaults(partialConfig)
// Fills in missing base URL, default model, timeout, etc.
```

### Legacy Utilities

#### Language Detection

#### DetectLanguage

Detects the programming language based on file extension or special filenames.

```go
func DetectLanguage(filename string) string
```

**Parameters:**
- `filename`: File name or path to analyze

**Returns:** Language name as a string (e.g., "go", "javascript", "python")

**Example:**
```go
language := common.DetectLanguage("main.go")        // "go"
language := common.DetectLanguage("script.py")       // "python"
language := common.DetectLanguage("Dockerfile")     // "dockerfile"
language := common.DetectLanguage("README.md")      // "markdown"
language := common.DetectLanguage("unknown.xyz")    // "text"
```

**Supported Languages:**
- Go (.go)
- JavaScript (.js, .jsx)
- TypeScript (.ts, .tsx)
- Python (.py)
- Java (.java)
- C/C++ (.c, .cpp, .cc, .cxx)
- C# (.cs)
- PHP (.php)
- Ruby (.rb)
- Swift (.swift)
- Kotlin (.kt)
- Rust (.rs)
- HTML (.html)
- CSS (.css)
- SCSS (.scss, .sass)
- JSON (.json)
- XML (.xml)
- YAML (.yaml, .yml)
- SQL (.sql)
- Bash (.sh, .bash)
- PowerShell (.ps1)
- Markdown (.md)
- Special files: Dockerfile, Makefile, README

### File Utilities

#### ReadFileContent

Reads file content and returns it as a string.

```go
func ReadFileContent(filename string) (string, error)
```

**Parameters:**
- `filename`: Path to the file to read

**Returns:** File content as string and any error encountered

**Example:**
```go
content, err := common.ReadFileContent("config.yaml")
if err != nil {
    log.Printf("Failed to read file: %v", err)
    return
}
fmt.Printf("File content: %s", content)
```

#### FilterContextFiles

Filters out the output file from context files to avoid duplication.

```go
func FilterContextFiles(contextFiles []string, outputFile string) []string
```

**Parameters:**
- `contextFiles`: List of context file paths
- `outputFile`: Path to the output file to exclude from context

**Returns:** Filtered list of context files excluding the output file

**Example:**
```go
contextFiles := []string{"file1.txt", "file2.txt", "output.txt"}
outputFile := "output.txt"

filtered := common.FilterContextFiles(contextFiles, outputFile)
// Result: ["file1.txt", "file2.txt"]
```

#### ReadConfigFile

Reads a configuration file and returns its raw byte content.

```go
func ReadConfigFile(configPath string) ([]byte, error)
```

**Parameters:**
- `configPath`: Path to the configuration file

**Returns:** Raw file content as bytes and any error encountered

**Example:**
```go
data, err := common.ReadConfigFile("config.yaml")
if err != nil {
    log.Fatal(err)
}

var config Config
err = yaml.Unmarshal(data, &config)
```

### Rate Limit Helper

#### RateLimitHelper

Provides shared rate limiting functionality for all AI providers, encapsulating common patterns of rate limit tracking, parsing, and enforcement.

```go
type RateLimitHelper struct {
    // Contains internal tracker, parser, and mutex
}
```

#### NewRateLimitHelper

Creates a new RateLimitHelper with the given provider-specific parser.

```go
func NewRateLimitHelper(parser ratelimit.Parser) *RateLimitHelper
```

**Parameters:**
- `parser`: Provider-specific rate limit parser

**Returns:** New RateLimitHelper instance

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"

// For OpenAI provider
openaiParser := ratelimit.NewOpenAIParser()
helper := common.NewRateLimitHelper(openaiParser)
```

#### ParseAndUpdateRateLimits

Parses rate limit headers from an HTTP response and updates the tracker.

```go
func (h *RateLimitHelper) ParseAndUpdateRateLimits(headers http.Header, model string)
```

**Parameters:**
- `headers`: HTTP response headers containing rate limit information
- `model`: Model identifier for tracking

**Example:**
```go
resp, err := client.Do(req)
if err == nil {
    helper.ParseAndUpdateRateLimits(resp.Header, "gpt-4")
}
```

#### CanMakeRequest

Checks if a request can be made for the given model and estimated tokens.

```go
func (h *RateLimitHelper) CanMakeRequest(model string, estimatedTokens int) bool
```

**Parameters:**
- `model`: Model identifier
- `estimatedTokens`: Estimated number of tokens the request will use

**Returns:** `true` if the request should proceed, `false` if rate limited

**Example:**
```go
if helper.CanMakeRequest("gpt-4", 1000) {
    // Make request
} else {
    // Wait or use different provider
}
```

#### CheckRateLimitAndWait

Checks rate limits and waits if necessary before making a request.

```go
func (h *RateLimitHelper) CheckRateLimitAndWait(model string, estimatedTokens int) bool
```

**Parameters:**
- `model`: Model identifier
- `estimatedTokens`: Estimated token usage

**Returns:** `true` if the caller should proceed with the request, `false` if rate limited

**Example:**
```go
if !helper.CheckRateLimitAndWait("gpt-4", 1000) {
    // Rate limit was hit, we waited but should retry
    return
}
// Proceed with request
```

#### GetRateLimitInfo

Retrieves current rate limit information for a model.

```go
func (h *RateLimitHelper) GetRateLimitInfo(model string) (*ratelimit.Info, bool)
```

**Parameters:**
- `model`: Model identifier

**Returns:** Rate limit info and boolean indicating whether data exists

**Example:**
```go
if info, exists := helper.GetRateLimitInfo("gpt-4"); exists {
    fmt.Printf("Requests remaining: %d/%d\n", info.RequestsRemaining, info.RequestsLimit)
    fmt.Printf("Tokens remaining: %d/%d\n", info.TokensRemaining, info.TokensLimit)
}
```

#### ShouldThrottle

Determines if requests should be throttled based on current usage.

```go
func (h *RateLimitHelper) ShouldThrottle(model string, threshold float64) bool
```

**Parameters:**
- `model`: Model identifier
- `threshold`: Percentage of limits consumed at which throttling should begin (0.0-1.0)

**Returns:** `true` if throttling should be applied

**Example:**
```go
if helper.ShouldThrottle("gpt-4", 0.8) {
    // At 80% of limits, start throttling
    time.Sleep(1 * time.Second)
}
```

### Model Cache

#### ModelCache

Stores cached model lists with timestamp and thread-safe access.

```go
type ModelCache struct {
    // Contains models, timestamp, TTL, and mutex
}
```

#### NewModelCache

Creates a new model cache with the specified TTL.

```go
func NewModelCache(ttl time.Duration) *ModelCache
```

**Parameters:**
- `ttl`: Time-to-live duration for cached data

**Returns:** New ModelCache instance

**Example:**
```go
cache := common.NewModelCache(1 * time.Hour)
```

#### GetModels

Returns cached models if available and fresh, or calls the fetch function.

```go
func (mc *ModelCache) GetModels(fetchFunc func() ([]types.Model, error), fallbackFunc func() []types.Model) ([]types.Model, error)
```

**Parameters:**
- `fetchFunc`: Function to fetch fresh models from API
- `fallbackFunc`: Optional function to return static models if fetch fails

**Returns:** Model list and any error

**Example:**
```go
models, err := cache.GetModels(
    func() ([]types.Model, error) {
        return provider.GetModelsFromAPI(ctx)
    },
    func() []types.Model {
        return getStaticModels() // fallback
    },
)
```

#### IsStale

Checks if the cache is expired.

```go
func (mc *ModelCache) IsStale() bool
```

**Returns:** `true` if cache is expired, `false` if still fresh

#### Get

Returns cached models (thread-safe).

```go
func (mc *ModelCache) Get() []types.Model
```

**Returns:** Currently cached models

#### Update

Updates the cache with new models (thread-safe).

```go
func (mc *ModelCache) Update(models []types.Model)
```

**Parameters:**
- `models`: New model list to cache

#### Clear

Empties the cache and resets the timestamp.

```go
func (mc *ModelCache) Clear()
```

### Testing Helpers

The `common/testing` package provides utilities for testing provider implementations.

#### MockServer

Configurable mock HTTP server for testing providers.

```go
type MockServer struct {
    // Contains server, response, status, and headers
}
```

**Example:**
```go
server := common.NewMockServer(`{"choices":[{"message":{"content":"Hello"}}]}`, 200)
server.SetHeader("Content-Type", "application/json")
url := server.Start()
defer server.Close()

// Configure provider to use mock server URL
config := types.ProviderConfig{
    BaseURL: url,
}
```

#### ProviderTestHelpers

Common helper functions for provider testing.

```go
helper := common.NewProviderTestHelpers(t, provider)

// Test basic functionality
helper.AssertProviderBasics("openai", "openai")
helper.AssertModelExists("gpt-4")
helper.AssertSupportsFeatures(true, true, false)

// Run comprehensive test suite
common.RunProviderTests(t, provider, config, []string{"gpt-4", "gpt-3.5-turbo"})
```

#### ToolCallTestHelper

Helpers for testing tool calling functionality.

```go
toolHelper := common.NewToolCallTestHelper(t)

// Test tool calling
toolHelper.StandardToolTestSuite(provider)

// Test specific tool scenarios
toolHelper.TestToolChoiceModes(provider, tools)
toolHelper.TestParallelToolCalls(provider)
```

#### CreateTestTool

Creates a standard test tool for testing tool calling.

```go
func CreateTestTool(name, description string) types.Tool
```

**Example:**
```go
tool := common.CreateTestTool("get_weather", "Get current weather")
// Returns: Tool with standard JSON schema for weather queries
```

#### TestProviderInterface

Ensures a provider implements all required interface methods.

```go
func TestProviderInterface(t *testing.T, provider types.Provider)
```

**Example:**
```go
func TestMyProvider(t *testing.T) {
    provider := NewMyProvider(config)
    common.TestProviderInterface(t, provider)
}
```

---

## Error Handling

### Error Types

**APIError**

```go
type APIError struct {
    StatusCode int       // HTTP status code
    Message    string    // Error message
    Type       string    // Error type
    Code       string    // Error code
    RawBody    string    // Raw response body
    Timestamp  time.Time // Error timestamp
}

func (e *APIError) Error() string
```

**AuthError**

```go
type AuthError struct {
    Provider string // Provider name
    Code     string // Error code
    Message  string // Error message
    Details  string // Additional details
    Retry    bool   // Whether error is retryable
}

func (e *AuthError) Error() string
func (e *AuthError) IsRetryable() bool
```

### Error Codes

```go
const (
    // Authentication errors
    ErrCodeInvalidCredentials  = "invalid_credentials"
    ErrCodeTokenExpired        = "token_expired"
    ErrCodeRefreshFailed       = "refresh_failed"
    ErrCodeOAuthFlowFailed     = "oauth_flow_failed"

    // Configuration errors
    ErrCodeInvalidConfig       = "invalid_config"

    // Network errors
    ErrCodeNetworkError        = "network_error"
    ErrCodeProviderUnavailable = "provider_unavailable"

    // Authorization errors
    ErrCodeScopeInsufficient   = "scope_insufficient"

    // Key management errors
    ErrCodeKeyRotationFailed   = "key_rotation_failed"
    ErrCodeAllKeysExhausted    = "all_keys_exhausted"

    // Storage errors
    ErrCodeStorageError        = "storage_error"
    ErrCodeEncryptionError     = "encryption_error"
)
```

### Error Handling Patterns

**Basic Error Handling**

```go
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    // Check error type
    if apiErr, ok := err.(*http.APIError); ok {
        switch apiErr.StatusCode {
        case 401:
            log.Println("Authentication failed")
        case 429:
            log.Println("Rate limited")
        case 500:
            log.Println("Server error")
        default:
            log.Printf("API error %d: %s", apiErr.StatusCode, apiErr.Message)
        }
        return err
    }

    // Generic error
    log.Printf("Error: %v", err)
    return err
}
```

**Retryable Errors**

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/http"

maxRetries := 3
for attempt := 0; attempt < maxRetries; attempt++ {
    stream, err := provider.GenerateChatCompletion(ctx, options)
    if err == nil {
        // Success
        return stream, nil
    }

    if !http.IsRetryableError(err) {
        // Non-retryable error
        return nil, err
    }

    // Exponential backoff
    backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
    log.Printf("Retry %d/%d after %v", attempt+1, maxRetries, backoff)
    time.Sleep(backoff)
}

return nil, fmt.Errorf("max retries exceeded")
```

**Timeout Handling**

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        log.Println("Request timed out")
        return fmt.Errorf("timeout: %w", err)
    }
    return err
}
```

**Graceful Degradation**

```go
primaryProvider := providers["openai"]
fallbackProvider := providers["anthropic"]

stream, err := primaryProvider.GenerateChatCompletion(ctx, options)
if err != nil {
    log.Printf("Primary provider failed: %v, trying fallback", err)

    // Try fallback
    stream, err = fallbackProvider.GenerateChatCompletion(ctx, options)
    if err != nil {
        return nil, fmt.Errorf("all providers failed: %w", err)
    }
}
```

---

## Constants and Enums

### ProviderType

```go
type ProviderType string

const (
    ProviderTypeOpenAI     ProviderType = "openai"
    ProviderTypeAnthropic  ProviderType = "anthropic"
    ProviderTypeGemini     ProviderType = "gemini"
    ProviderTypeQwen       ProviderType = "qwen"
    ProviderTypeCerebras   ProviderType = "cerebras"
    ProviderTypeOpenRouter ProviderType = "openrouter"
    ProviderTypeSynthetic  ProviderType = "synthetic"
    ProviderTypexAI        ProviderType = "xai"
    ProviderTypeFireworks  ProviderType = "fireworks"
    ProviderTypeDeepseek   ProviderType = "deepseek"
    ProviderTypeMistral    ProviderType = "mistral"
    ProviderTypeLMStudio   ProviderType = "lmstudio"
    ProviderTypeLlamaCpp   ProviderType = "llamacpp"
    ProviderTypeOllama     ProviderType = "ollama"
)
```

### AuthMethod

```go
type AuthMethod string

const (
    AuthMethodAPIKey      AuthMethod = "api_key"
    AuthMethodBearerToken AuthMethod = "bearer_token"
    AuthMethodOAuth       AuthMethod = "oauth"
    AuthMethodCustom      AuthMethod = "custom"
)
```

### ToolFormat

```go
type ToolFormat string

const (
    ToolFormatOpenAI    ToolFormat = "openai"
    ToolFormatAnthropic ToolFormat = "anthropic"
    ToolFormatXML       ToolFormat = "xml"
    ToolFormatHermes    ToolFormat = "hermes"
    ToolFormatText      ToolFormat = "text"
)
```

### ToolChoiceMode

```go
type ToolChoiceMode string

const (
    ToolChoiceAuto     ToolChoiceMode = "auto"
    ToolChoiceRequired ToolChoiceMode = "required"
    ToolChoiceNone     ToolChoiceMode = "none"
    ToolChoiceSpecific ToolChoiceMode = "specific"
)
```

---

## HTTP Utilities

### Request Building

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/http"

// Create JSON request
req, err := http.NewJSONRequest("POST", "https://api.example.com/v1/chat", requestBody)

// Or use RequestBuilder
req, err := http.NewRequestBuilder("POST", "https://api.example.com/v1/chat").
    WithHeader("Authorization", "Bearer "+token).
    WithJSONBody(requestBody).
    Build()
```

### Response Processing

```go
resp, err := client.Do(req)
if err != nil {
    return err
}

// Process JSON response
var result ChatResponse
err = http.ProcessJSONResponse(resp, &result)
if err != nil {
    if apiErr, ok := err.(*http.APIError); ok {
        log.Printf("API Error %d: %s", apiErr.StatusCode, apiErr.Message)
    }
    return err
}
```

### Streaming Responses

```go
resp, err := client.Do(req)
if err != nil {
    return err
}

stream := http.NewStreamingResponse(resp)
defer stream.Close()

for {
    line, err := stream.ReadLine()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }

    // Process line
    fmt.Println(string(line))
}
```

### Common Headers

```go
// Get common headers
headers := http.CommonHTTPHeaders()

// Get auth headers
authHeaders := http.AuthHeaders("openai", apiKey)

// Provider-specific headers
anthropicHeaders := http.AuthHeaders("anthropic", apiKey)
// Returns: {
//   "x-api-key": apiKey,
//   "anthropic-version": "2023-06-01",
// }
```

---

## Rate Limiting

### Rate Limit Parsing

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"

// Parse OpenAI headers
parser := ratelimit.NewOpenAIParser()
info, err := parser.Parse(response.Header, "gpt-4")
if err != nil {
    log.Printf("Failed to parse rate limits: %v", err)
}

fmt.Printf("Requests: %d/%d remaining\n", info.RequestsRemaining, info.RequestsLimit)
fmt.Printf("Tokens: %d/%d remaining\n", info.TokensRemaining, info.TokensLimit)
fmt.Printf("Reset in: %v\n", time.Until(info.RequestsReset))
```

### Rate Limit Tracking

```go
tracker := ratelimit.NewTracker()

// After each request
parser := ratelimit.NewOpenAIParser()
info, _ := parser.Parse(response.Header, model)
tracker.Update(info)

// Check before making request
if !tracker.CanMakeRequest(model, estimatedTokens) {
    waitTime := tracker.GetWaitTime(model)
    log.Printf("Rate limited. Wait %v", waitTime)
    time.Sleep(waitTime)
}

// Check if approaching limits
if tracker.ShouldThrottle(model, 0.9) {
    log.Println("Approaching rate limits - throttling")
    time.Sleep(1 * time.Second)
}
```

### Provider-Specific Parsers

```go
// OpenAI
openaiParser := ratelimit.NewOpenAIParser()

// Anthropic
anthropicParser := ratelimit.NewAnthropicParser()

// Cerebras
cerebrasParser := ratelimit.NewCerebrasParser()

// Qwen
qwenParser := ratelimit.NewQwenParser()
```

---

## Complete Example

Here's a complete example demonstrating multiple features:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Initialize factory
    f := factory.NewProviderFactory()
    factory.RegisterDefaultProviders(f)

    // Create provider
    config := types.ProviderConfig{
        Type:   types.ProviderTypeOpenAI,
        APIKey: os.Getenv("OPENAI_API_KEY"),
    }

    provider, err := f.CreateProvider(types.ProviderTypeOpenAI, config)
    if err != nil {
        log.Fatalf("Failed to create provider: %v", err)
    }

    // Authenticate
    ctx := context.Background()
    authConfig := types.AuthConfig{
        Method: types.AuthMethodAPIKey,
        APIKey: config.APIKey,
    }
    if err := provider.Authenticate(ctx, authConfig); err != nil {
        log.Fatalf("Authentication failed: %v", err)
    }

    // Get models
    models, err := provider.GetModels(ctx)
    if err != nil {
        log.Fatalf("Failed to get models: %v", err)
    }
    fmt.Printf("Available models: %d\n", len(models))

    // Define tools
    tools := []types.Tool{
        {
            Name:        "get_weather",
            Description: "Get weather for a location",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "location": map[string]interface{}{
                        "type": "string",
                    },
                },
                "required": []string{"location"},
            },
        },
    }

    // Make request with tools
    options := types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: "What's the weather in Paris?"},
        },
        Model:       "gpt-4",
        MaxTokens:   1000,
        Temperature: 0.7,
        Stream:      true,
        Tools:       tools,
        ToolChoice:  &types.ToolChoice{Mode: types.ToolChoiceAuto},
    }

    stream, err := provider.GenerateChatCompletion(ctx, options)
    if err != nil {
        log.Fatalf("Chat completion failed: %v", err)
    }
    defer stream.Close()

    // Read response
    chunk, err := stream.Next()
    if err != nil {
        log.Fatalf("Failed to read response: %v", err)
    }

    // Check for tool calls
    if len(chunk.Choices) > 0 && len(chunk.Choices[0].Message.ToolCalls) > 0 {
        fmt.Println("Tool calls received:")
        for _, tc := range chunk.Choices[0].Message.ToolCalls {
            fmt.Printf("  Function: %s\n", tc.Function.Name)

            var args map[string]interface{}
            json.Unmarshal([]byte(tc.Function.Arguments), &args)
            fmt.Printf("  Arguments: %+v\n", args)

            // Execute tool (mock)
            result := executeWeatherTool(args["location"].(string))

            // Submit result
            options.Messages = append(options.Messages, chunk.Choices[0].Message)
            options.Messages = append(options.Messages, types.ChatMessage{
                Role:       "tool",
                ToolCallID: tc.ID,
                Content:    result,
            })

            // Continue conversation
            stream2, _ := provider.GenerateChatCompletion(ctx, options)
            defer stream2.Close()

            chunk2, _ := stream2.Next()
            fmt.Printf("\nFinal response: %s\n", chunk2.Content)
        }
    } else {
        fmt.Printf("Response: %s\n", chunk.Content)
    }

    // Get metrics
    metrics := provider.GetMetrics()
    fmt.Printf("\nMetrics:\n")
    fmt.Printf("  Requests: %d\n", metrics.RequestCount)
    fmt.Printf("  Success: %d\n", metrics.SuccessCount)
    fmt.Printf("  Errors: %d\n", metrics.ErrorCount)
    fmt.Printf("  Avg Latency: %v\n", metrics.AverageLatency)
    fmt.Printf("  Tokens Used: %d\n", metrics.TokensUsed)
}

func executeWeatherTool(location string) string {
    weather := map[string]interface{}{
        "location":    location,
        "temperature": 22,
        "unit":        "celsius",
        "conditions":  "sunny",
    }
    result, _ := json.Marshal(weather)
    return string(result)
}
```

---

## Best Practices

### 1. Always Use Context

```go
// Good
ctx := context.Background()
stream, err := provider.GenerateChatCompletion(ctx, options)

// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
stream, err := provider.GenerateChatCompletion(ctx, options)
```

### 2. Always Close Streams

```go
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    return err
}
defer stream.Close() // Always defer close
```

### 3. Handle Errors Properly

```go
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    if apiErr, ok := err.(*http.APIError); ok {
        // Handle API errors
        if apiErr.StatusCode == 429 {
            // Rate limited - implement backoff
        }
    }
    return err
}
```

### 4. Use Metrics for Monitoring

```go
metrics := provider.GetMetrics()
if metrics.ErrorCount > metrics.SuccessCount {
    log.Printf("Warning: High error rate for %s", provider.Name())
}
```

### 5. Implement Graceful Degradation

```go
providers := []types.Provider{primaryProvider, fallbackProvider}
for _, p := range providers {
    stream, err := p.GenerateChatCompletion(ctx, options)
    if err == nil {
        return stream, nil
    }
    log.Printf("Provider %s failed: %v", p.Name(), err)
}
return nil, fmt.Errorf("all providers failed")
```

### 6. Validate Tool Schemas

```go
// Ensure required fields are present
tool := types.Tool{
    Name:        "my_function",
    Description: "Clear description",
    InputSchema: map[string]interface{}{
        "type":       "object",
        "properties": map[string]interface{}{...},
        "required":   []string{"field1"},
    },
}
```

---

## Additional Resources

- [Main Documentation](README.md)
- [OAuth Manager Guide](OAUTH_MANAGER.md)
- [Metrics Guide](METRICS.md)
- [Multi-Key Strategies](MULTI_KEY_STRATEGIES.md)

For detailed documentation on the common utilities package, see [SDK_COMMON_UTILITIES.md](SDK_COMMON_UTILITIES.md).

---

## Cross-References

### Related Documentation

- **[Common Utilities Reference](SDK_COMMON_UTILITIES.md)** - Comprehensive guide to shared utilities
- **[Best Practices Guide](SDK_BEST_PRACTICES.md)** - Patterns and guidelines for using shared utilities
- **[Provider Implementation Guide](SDK_PROVIDER_GUIDES.md)** - Custom provider development with shared utilities
- **[Getting Started Guide](SDK_GETTING_STARTED.md)** - Introduction to the SDK

### Quick Navigation

| Topic | Documentation |
|-------|---------------|
| Shared Utilities Overview | [SDK_COMMON_UTILITIES.md#overview](SDK_COMMON_UTILITIES.md#overview) |
| Language Detection | [SDK_COMMON_UTILITIES.md#language-detection](SDK_COMMON_UTILITIES.md#language-detection) |
| File Operations | [SDK_COMMON_UTILITIES.md#file-utilities](SDK_COMMON_UTILITIES.md#file-utilities) |
| Rate Limiting | [SDK_COMMON_UTILITIES.md#rate-limit-helper](SDK_COMMON_UTILITIES.md#rate-limit-helper) |
| Model Caching | [SDK_COMMON_UTILITIES.md#model-cache](SDK_COMMON_UTILITIES.md#model-cache) |
| Testing Framework | [SDK_COMMON_UTILITIES.md#testing-framework](SDK_COMMON_UTILITIES.md#testing-framework) |
| Provider Development | [SDK_COMMON_UTILITIES.md#provider-development-guide](SDK_COMMON_UTILITIES.md#provider-development-guide) |

---

**Document Version:** 1.0
**Last Updated:** 2025-11-19
**Maintainers:** AI Provider Kit Team
