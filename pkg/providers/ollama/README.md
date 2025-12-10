# Ollama Provider

The Ollama provider enables integration with both local Ollama instances and Ollama Cloud for AI-powered chat completions, streaming, and tool calling.

## Features

- **Local and Cloud Support**: Connect to local Ollama instances or Ollama Cloud
- **Streaming Chat Completions**: Real-time response streaming with newline-delimited JSON or SSE format
- **Tool/Function Calling**: OpenAI-compatible tool calling for function execution
- **Multimodal Support**: Image inputs for vision-capable models (like LLaVA)
- **Model Discovery**: Automatic detection and capability inference for installed models
- **Flexible Endpoints**: Support for both native Ollama API and OpenAI-compatible endpoints

## Installation

The Ollama provider is included in the ai-provider-kit package:

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/ollama"
```

## Configuration

### Local Ollama Instance

For local Ollama installations (default port 11434):

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOllama,
    Name:         "my-ollama",
    BaseURL:      "http://localhost:11434",
    DefaultModel: "llama3.1:8b",
}

provider := ollama.NewOllamaProvider(config)
```

### Ollama Cloud

For Ollama Cloud endpoints with API key authentication:

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOllama,
    Name:         "ollama-cloud",
    BaseURL:      "https://api.ollama.com",
    APIKey:       "your-api-key-here",
    APIKeyEnv:    "OLLAMA_API_KEY", // Optional: load from environment
    DefaultModel: "llama3.1:8b",
}

provider := ollama.NewOllamaProvider(config)
```

### OpenAI-Compatible Endpoint

To use OpenAI-compatible streaming format (SSE):

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeOllama,
    Name:         "ollama-openai",
    BaseURL:      "http://localhost:11434",
    DefaultModel: "llama3.1:8b",
    ProviderConfig: map[string]interface{}{
        "stream_endpoint": "openai", // Use OpenAI-compatible /v1/chat/completions
    },
}

provider := ollama.NewOllamaProvider(config)
```

## Usage Examples

### Basic Chat Completion

```go
ctx := context.Background()

options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {
            Role:    "user",
            Content: "Explain quantum computing in simple terms",
        },
    },
    Model:       "llama3.1:8b",
    Temperature: 0.7,
    MaxTokens:   500,
}

stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

// Read streaming response
for {
    chunk, err := stream.Next()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Print(chunk.Content)
}
```

### Tool Calling

```go
options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {
            Role:    "user",
            Content: "What's the weather in San Francisco?",
        },
    },
    Model: "llama3.1:8b",
    Tools: []types.Tool{
        {
            Name:        "get_weather",
            Description: "Get the current weather for a location",
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
    },
}

stream, err := provider.GenerateChatCompletion(ctx, options)
// Handle tool calls from stream...
```

### Multimodal (Vision)

```go
options := types.GenerateOptions{
    Messages: []types.ChatMessage{
        {
            Role:    "user",
            Content: "What's in this image?",
            Parts: []types.ContentPart{
                {
                    Type: types.ContentTypeText,
                    Text: "What's in this image?",
                },
                {
                    Type: types.ContentTypeImage,
                    Source: &types.MediaSource{
                        Type: types.MediaSourceBase64,
                        Data: base64ImageData, // Base64-encoded image
                    },
                },
            },
        },
    },
    Model: "llava:7b", // Vision-capable model
}

stream, err := provider.GenerateChatCompletion(ctx, options)
// Process response...
```

### Model Discovery

```go
models, err := provider.GetModels(ctx)
if err != nil {
    log.Fatal(err)
}

for _, model := range models {
    fmt.Printf("Model: %s\n", model.ID)
    fmt.Printf("  Max Tokens: %d\n", model.MaxTokens)
    fmt.Printf("  Tool Calling: %v\n", model.SupportsToolCalling)
    fmt.Printf("  Capabilities: %v\n", model.Capabilities)
}
```

## API Endpoints

The provider supports multiple Ollama endpoints:

### Native Ollama API

- **POST /api/chat** - Chat completions with streaming (newline-delimited JSON)
- **GET /api/tags** - List available models
- **GET /api/version** - Health check and version info

### OpenAI-Compatible API

- **POST /v1/chat/completions** - Chat completions with streaming (SSE format)

Configure the endpoint format using the `stream_endpoint` option:
- `"ollama"` (default) - Native Ollama format
- `"openai"` - OpenAI-compatible format

## Streaming Formats

### Native Ollama Format

Newline-delimited JSON chunks:

```json
{"model":"llama3.1:8b","message":{"role":"assistant","content":"Hello"},"done":false}
{"model":"llama3.1:8b","message":{"role":"assistant","content":" world"},"done":false}
{"model":"llama3.1:8b","message":{"role":"assistant","content":"!"},"done":true,"prompt_eval_count":5,"eval_count":10}
```

### OpenAI-Compatible Format

Server-Sent Events (SSE):

```
data: {"id":"chatcmpl-123","choices":[{"delta":{"content":"Hello"},"index":0}]}

data: {"id":"chatcmpl-123","choices":[{"delta":{"content":" world"},"index":0}]}

data: [DONE]
```

## Model Capabilities

The provider automatically infers model capabilities:

| Model Family | Tool Calling | Vision | Max Tokens | Use Case |
|-------------|--------------|--------|------------|----------|
| llama3.1    | ✓           | ✗      | 128K       | General chat, coding |
| llama3.2    | ✓           | ✓      | 128K       | Multimodal |
| codellama   | ✗           | ✗      | 16K        | Code generation |
| mistral     | ✓           | ✗      | 32K        | General chat |
| llava       | ✗           | ✓      | 128K       | Vision tasks |
| nomic-embed | ✗           | ✗      | 8K         | Embeddings |

## Health Checks

Test connectivity to Ollama:

```go
err := provider.HealthCheck(ctx)
if err != nil {
    log.Printf("Ollama not reachable: %v", err)
}
```

## Authentication

### Local Instance

No authentication required - Ollama runs without API keys locally.

### Cloud Endpoint

Requires API key authentication:

```go
// Method 1: Direct configuration
config.APIKey = "your-api-key"

// Method 2: Environment variable
config.APIKeyEnv = "OLLAMA_API_KEY"

// Method 3: Runtime authentication
authConfig := types.AuthConfig{
    Method:  types.AuthMethodAPIKey,
    APIKey:  "your-api-key",
    BaseURL: "https://api.ollama.com",
}
err := provider.Authenticate(ctx, authConfig)
```

## Error Handling

The provider returns typed errors for different failure scenarios:

```go
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    switch e := err.(type) {
    case *types.AuthError:
        log.Printf("Authentication failed: %v", e)
    case *types.NetworkError:
        log.Printf("Network error: %v", e)
    case *types.RateLimitError:
        log.Printf("Rate limited, retry after: %v", e.RetryAfter)
    case *types.NotFoundError:
        log.Printf("Model not found: %v", e)
    default:
        log.Printf("Error: %v", e)
    }
}
```

## Metrics

Track provider usage:

```go
metrics := provider.GetMetrics()
fmt.Printf("Requests: %d\n", metrics.RequestCount)
fmt.Printf("Success: %d\n", metrics.SuccessCount)
fmt.Printf("Errors: %d\n", metrics.ErrorCount)
fmt.Printf("Total Tokens: %d\n", metrics.TokensUsed)
```

## Advanced Configuration

### Custom Timeouts

```go
config := types.ProviderConfig{
    Type:    types.ProviderTypeOllama,
    BaseURL: "http://localhost:11434",
    ProviderConfig: map[string]interface{}{
        "timeout": 60 * time.Second, // 60 second timeout
    },
}
```

### Model Caching

Models are cached for 5 minutes by default to reduce API calls during model listing.

## Supported Models

Popular models available through Ollama:

- **Llama 3.1/3.2** - General purpose and multimodal
- **Mistral/Mixtral** - High-performance instruction following
- **CodeLlama** - Specialized for code generation
- **Phi** - Microsoft's efficient small models
- **Gemma** - Google's open models
- **Qwen** - Alibaba's multilingual models
- **DeepSeek** - Coding and reasoning models
- **LLaVA** - Vision and image understanding

See the [Ollama Model Library](https://ollama.com/library) for the complete list.

## Contributing

Contributions are welcome! Please see the main project README for contribution guidelines.

## License

This provider is part of the ai-provider-kit project and follows the same license.

## References

- [Ollama Documentation](https://github.com/ollama/ollama/blob/main/docs/api.md)
- [Ollama Model Library](https://ollama.com/library)
- [OpenAI API Compatibility](https://github.com/ollama/ollama/blob/main/docs/openai.md)
