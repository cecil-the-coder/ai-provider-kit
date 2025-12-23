# Ollama Provider Implementation Specification

## Table of Contents
1. [Overview](#overview)
2. [Authentication](#authentication)
3. [API Endpoints](#api-endpoints)
4. [Implementation Architecture](#implementation-architecture)
5. [Code Structure](#code-structure)
6. [Implementation Steps](#implementation-steps)
7. [Testing Strategy](#testing-strategy)
8. [References](#references)

---

## Overview

### What is Ollama Cloud?
Ollama Cloud is a preview feature that enables users to run larger AI models without powerful local GPUs by offloading execution to Ollama's cloud service. It maintains API compatibility with local Ollama installations, allowing seamless switching between local and cloud deployments.

### Key Capabilities
- **Local & Cloud Support**: Unified interface for both `http://localhost:11434` and `https://ollama.com/api`
- **Large Model Access**: Run models like `gpt-oss:120b-cloud` without local GPU
- **OpenAI Compatibility**: Full support for `/api/chat/completions` endpoint
- **Native API**: Access to Ollama-specific endpoints for advanced features
- **Streaming**: Native streaming support for real-time responses
- **Tool Calling**: Support for function/tool calling
- **Structured Outputs**: Support for structured response formats

### Provider Type
- **Constant**: `ProviderTypeOllama` (already defined in `pkg/types/provider.go:27`)
- **Name**: "Ollama"
- **Description**: "Ollama local and cloud model inference with OpenAI-compatible API"

---

## Authentication

### Local Ollama (Default)
- **Endpoint**: `http://localhost:11434`
- **Authentication**: None required
- **Use Case**: Local model serving, development, testing

### Ollama Cloud
- **Endpoint**: `https://ollama.com/api`
- **Authentication Method 1 - CLI Signin**:
  ```bash
  ollama signin
  ```
  - Automatic authentication for local CLI commands
  - Session-based authentication managed by Ollama CLI

- **Authentication Method 2 - API Key**:
  ```bash
  export OLLAMA_API_KEY=your_api_key_here
  ```
  - Required for direct API access
  - Created in account settings at https://ollama.com
  - Used as Bearer token: `Authorization: Bearer $OLLAMA_API_KEY`

### Implementation Requirements
- Auto-detect endpoint based on `BaseURL` configuration
- Support `OLLAMA_API_KEY` environment variable
- Optional API key for cloud endpoints
- No authentication for local endpoints

---

## API Endpoints

### Primary Endpoints

#### 1. Chat Completions (OpenAI-Compatible)
**Endpoint**: `POST /api/chat/completions`

**Request Format**:
```json
{
  "model": "llama3.1:8b",
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "stream": true,
  "tools": [...],  // Optional tool definitions
  "temperature": 0.7,
  "max_tokens": 1000
}
```

**Response Format** (Non-Streaming):
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "llama3.1:8b",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! I'm doing well, thank you for asking."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 15,
    "total_tokens": 25
  }
}
```

**Response Format** (Streaming):
```json
{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}
{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}
{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":15,"total_tokens":25}}
```

#### 2. Model Listing
**Endpoint**: `GET /api/tags`

**Response Format**:
```json
{
  "models": [
    {
      "name": "llama3.1:8b",
      "model": "llama3.1:8b",
      "modified_at": "2024-12-10T12:00:00.000Z",
      "size": 4661224448,
      "digest": "sha256:abcd1234...",
      "details": {
        "parent_model": "",
        "format": "gguf",
        "family": "llama",
        "families": ["llama"],
        "parameter_size": "8.0B",
        "quantization_level": "Q4_0"
      }
    }
  ]
}
```

**Note**: Cloud models have `-cloud` suffix (e.g., `gpt-oss:120b-cloud`)

#### 3. Model Information
**Endpoint**: `POST /api/show`

**Request Format**:
```json
{
  "name": "llama3.1:8b"
}
```

**Response Format**:
```json
{
  "modelfile": "...",
  "parameters": "...",
  "template": "...",
  "details": {
    "parent_model": "",
    "format": "gguf",
    "family": "llama",
    "families": ["llama"],
    "parameter_size": "8.0B",
    "quantization_level": "Q4_0"
  },
  "model_info": {
    "general.architecture": "llama",
    "general.file_type": "Q4_0",
    "general.parameter_count": 8030261248,
    "llama.context_length": 131072,
    "llama.embedding_length": 4096
  }
}
```

#### 4. Native Generate (Ollama-Specific)
**Endpoint**: `POST /api/generate`

**Request Format**:
```json
{
  "model": "llama3.1:8b",
  "prompt": "Hello, how are you?",
  "stream": true
}
```

**Response Format** (Streaming):
```json
{"model":"llama3.1:8b","created_at":"2024-12-10T12:00:00.000Z","response":"Hello","done":false}
{"model":"llama3.1:8b","created_at":"2024-12-10T12:00:00.000Z","response":"!","done":false}
{"model":"llama3.1:8b","created_at":"2024-12-10T12:00:00.000Z","response":"","done":true,"total_duration":5000000000,"load_duration":1000000000,"prompt_eval_count":10,"prompt_eval_duration":2000000000,"eval_count":15,"eval_duration":2000000000}
```

**Note**: All durations in nanoseconds

#### 5. Health Check
**Endpoint**: `GET /` or `GET /api/version`

**Response Format**:
```json
{
  "version": "0.5.2"
}
```

### Additional Endpoints (For Future Enhancement)
- `POST /api/embeddings` - Generate embeddings
- `GET /api/ps` - List running models
- `POST /api/pull` - Pull a model from registry
- `POST /api/push` - Push a model to registry
- `POST /api/copy` - Copy a model
- `DELETE /api/delete` - Delete a model
- `POST /api/create` - Create a model from Modelfile

---

## Implementation Architecture

### Design Decision: Unified Provider (Option 1)

**Rationale**:
- Single provider handles both local and cloud deployments
- Users switch by changing `BaseURL` configuration
- Matches user mental model of "Ollama with different backends"
- Simpler codebase maintenance
- Follows existing pattern from other providers

### Provider Structure

```go
type OllamaProvider struct {
    *base.BaseProvider
    config            types.ProviderConfig
    httpClient        *http.Client
    authHelper        *auth.AuthHelper
    modelCache        *models.ModelCache
    connectivityCache *common.ConnectivityCache
    rateLimitHelper   *common.RateLimitHelper
}
```

### Key Components

#### 1. Authentication Helper
- **Purpose**: Manage API keys for cloud access
- **Features**:
  - Read from `OLLAMA_API_KEY` environment variable
  - Support multiple API keys for failover
  - Auto-detect cloud vs local based on BaseURL
  - Skip authentication for local endpoints

#### 2. Model Cache
- **Purpose**: Cache model listings to reduce API calls
- **TTL**: 5 minutes (configurable)
- **Fallback**: Static model list if API unavailable

#### 3. Connectivity Cache
- **Purpose**: Track endpoint health and avoid repeated failures
- **Features**:
  - Exponential backoff on failures
  - Per-endpoint health tracking
  - Automatic recovery attempts

#### 4. Rate Limit Helper
- **Purpose**: Parse and respect rate limit headers
- **Implementation**: Custom parser for Ollama rate limit format (if any)

### Feature Support Matrix

| Feature | Local Ollama | Ollama Cloud |
|---------|--------------|--------------|
| Chat Completions | ✅ | ✅ |
| Streaming | ✅ | ✅ |
| Tool Calling | ✅ | ✅ |
| Model Listing | ✅ | ✅ |
| Model Info | ✅ | ✅ |
| Embeddings | ✅ | ✅ |
| Native Generate | ✅ | ✅ |
| Authentication | ❌ | ✅ (API Key) |
| Model Management | ✅ | ❌ |

### Configuration Examples

#### Local Ollama
```go
config := types.ProviderConfig{
    Type:    types.ProviderTypeOllama,
    Name:    "ollama-local",
    BaseURL: "http://localhost:11434",
    // No APIKey needed
    DefaultModel: "llama3.1:8b",
}
```

#### Ollama Cloud
```go
config := types.ProviderConfig{
    Type:      types.ProviderTypeOllama,
    Name:      "ollama-cloud",
    BaseURL:   "https://ollama.com/api",
    APIKeyEnv: "OLLAMA_API_KEY",
    DefaultModel: "gpt-oss:120b-cloud",
}
```

#### Auto-Detection Logic
```go
func (p *OllamaProvider) isCloudEndpoint() bool {
    return strings.Contains(p.config.BaseURL, "ollama.com")
}

func (p *OllamaProvider) requiresAuth() bool {
    return p.isCloudEndpoint()
}
```

---

## Code Structure

### Directory Layout
```
pkg/providers/ollama/
├── provider.go            # Main provider implementation (standardized naming)
├── ollama_test.go         # Unit tests
├── models.go              # Model listing and enrichment
├── models_test.go         # Model tests
├── chat.go                # Chat completion implementation
├── chat_test.go           # Chat tests
├── streaming.go           # Streaming implementation
├── streaming_test.go      # Streaming tests
└── README.md              # Provider-specific documentation
```

### File Responsibilities

#### provider.go
- Provider struct definition
- Constructor: `NewOllamaProvider(config)`
- Core interface implementations:
  - `Name()`, `Type()`, `Description()`
  - `Authenticate()`, `IsAuthenticated()`, `Logout()`
  - `Configure()`, `GetConfig()`
  - `HealthCheck()`, `GetMetrics()`
  - `GetDefaultModel()`, `GetToolFormat()`
  - `SupportsToolCalling()`, `SupportsStreaming()`, `SupportsResponsesAPI()`

#### models.go
- `GetModels(ctx)` - Fetch models from API or cache
- `fetchModelsFromAPI(ctx)` - Call `/api/tags` endpoint
- `enrichModels(models)` - Add provider-specific metadata
- `getStaticFallback()` - Return hardcoded model list
- `parseModelDetails(details)` - Parse Ollama model info

#### chat.go
- `GenerateChatCompletion(ctx, options)` - Main chat method
- `buildChatRequest(options)` - Convert to Ollama format
- `parseChatResponse(resp)` - Parse Ollama response
- `handleToolCalls(resp)` - Extract tool calls

#### streaming.go
- `OllamaStream` struct implementation
- `Next()` - Read next chunk
- `Close()` - Cleanup resources
- SSE parsing for streaming responses

### Integration Points

#### Factory Registration
```go
// In pkg/factory/registry.go or similar
func init() {
    defaultFactory.RegisterProvider(
        types.ProviderTypeOllama,
        func(config types.ProviderConfig) types.Provider {
            return ollama.NewOllamaProvider(config)
        },
    )
}
```

#### Common Package Usage
```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
)
```

---

## Implementation Steps

### Phase 1: Basic Provider Setup
1. ✅ Create `pkg/providers/ollama/` directory
2. ✅ Implement `provider.go` with basic provider structure
3. ✅ Add constructor `NewOllamaProvider(config)`
4. ✅ Implement core interface methods (Name, Type, Description)
5. ✅ Setup authentication helper with cloud/local detection
6. ✅ Implement `IsAuthenticated()` logic
7. ✅ Add basic health check (GET `/api/version`)

### Phase 2: Model Management
1. ✅ Implement `models.go`
2. ✅ Add `GetModels(ctx)` with caching
3. ✅ Implement `/api/tags` API call
4. ✅ Parse Ollama model response format
5. ✅ Enrich models with capabilities metadata
6. ✅ Add static fallback model list
7. ✅ Handle cloud vs local model suffixes

**Static Model List** (Fallback):
```go
func (p *OllamaProvider) getStaticFallback() []types.Model {
    return []types.Model{
        // Popular Llama models
        {
            ID:          "llama3.1:8b",
            Name:        "Llama 3.1 8B",
            Provider:    types.ProviderTypeOllama,
            Description: "Meta's Llama 3.1 model with 8B parameters",
            MaxTokens:   131072,
            SupportsStreaming: true,
            SupportsToolCalling: true,
            Capabilities: []string{"chat", "completion", "tool_calling"},
        },
        {
            ID:          "llama3.1:70b",
            Name:        "Llama 3.1 70B",
            Provider:    types.ProviderTypeOllama,
            Description: "Meta's Llama 3.1 model with 70B parameters",
            MaxTokens:   131072,
            SupportsStreaming: true,
            SupportsToolCalling: true,
            Capabilities: []string{"chat", "completion", "tool_calling"},
        },
        // Cloud models
        {
            ID:          "gpt-oss:120b-cloud",
            Name:        "GPT OSS 120B Cloud",
            Provider:    types.ProviderTypeOllama,
            Description: "Large open-source GPT model on Ollama Cloud",
            MaxTokens:   8192,
            SupportsStreaming: true,
            SupportsToolCalling: true,
            Capabilities: []string{"chat", "completion", "tool_calling"},
        },
        // Code models
        {
            ID:          "codellama:7b",
            Name:        "CodeLlama 7B",
            Provider:    types.ProviderTypeOllama,
            Description: "Meta's CodeLlama for code generation",
            MaxTokens:   16384,
            SupportsStreaming: true,
            SupportsToolCalling: false,
            Capabilities: []string{"chat", "completion", "code"},
        },
        // Vision models
        {
            ID:          "llava:7b",
            Name:        "LLaVA 7B",
            Provider:    types.ProviderTypeOllama,
            Description: "Visual language model",
            MaxTokens:   4096,
            SupportsStreaming: true,
            SupportsToolCalling: false,
            Capabilities: []string{"chat", "vision"},
        },
        // Embedding models
        {
            ID:          "nomic-embed-text",
            Name:        "Nomic Embed Text",
            Provider:    types.ProviderTypeOllama,
            Description: "Text embedding model",
            MaxTokens:   8192,
            SupportsStreaming: false,
            SupportsToolCalling: false,
            Capabilities: []string{"embeddings"},
        },
    }
}
```

### Phase 3: Chat Completions
1. ✅ Implement `chat.go`
2. ✅ Add `GenerateChatCompletion(ctx, options)`
3. ✅ Convert `types.GenerateOptions` to Ollama request format
4. ✅ Handle multimodal content (images in messages)
5. ✅ Implement tool/function calling support
6. ✅ Parse Ollama response to `types.ChatCompletionStream`
7. ✅ Error handling and retry logic
8. ✅ Support for both `/api/chat/completions` and `/api/generate`

**Request Translation**:
```go
// StandardRequest -> Ollama Chat Request
type ollamaChatRequest struct {
    Model    string                   `json:"model"`
    Messages []ollamaChatMessage      `json:"messages"`
    Stream   bool                     `json:"stream"`
    Tools    []types.Tool             `json:"tools,omitempty"`
    Options  map[string]interface{}   `json:"options,omitempty"`
}

type ollamaChatMessage struct {
    Role    string   `json:"role"`
    Content string   `json:"content"`
    Images  []string `json:"images,omitempty"` // base64 encoded
}

func buildOllamaRequest(opts types.GenerateOptions) ollamaChatRequest {
    req := ollamaChatRequest{
        Model:    opts.Model,
        Messages: convertMessages(opts.Messages),
        Stream:   true, // Always stream internally
        Tools:    opts.Tools,
    }

    // Map standard options to Ollama options
    if opts.Temperature != nil {
        req.Options["temperature"] = *opts.Temperature
    }
    if opts.MaxTokens != nil {
        req.Options["num_predict"] = *opts.MaxTokens
    }
    if opts.TopP != nil {
        req.Options["top_p"] = *opts.TopP
    }

    return req
}
```

### Phase 4: Streaming Support
1. ✅ Implement `streaming.go`
2. ✅ Create `OllamaStream` struct
3. ✅ Implement `Next()` method with SSE parsing
4. ✅ Handle chunked responses
5. ✅ Accumulate deltas for final response
6. ✅ Parse usage information from final chunk
7. ✅ Implement `Close()` with cleanup

**Streaming Implementation**:
```go
type OllamaStream struct {
    reader  *bufio.Reader
    body    io.ReadCloser
    decoder *json.Decoder
    done    bool
}

func (s *OllamaStream) Next() (types.ChatCompletionChunk, error) {
    if s.done {
        return types.ChatCompletionChunk{}, io.EOF
    }

    var chunk ollamaStreamChunk
    if err := s.decoder.Decode(&chunk); err != nil {
        if err == io.EOF {
            s.done = true
        }
        return types.ChatCompletionChunk{}, err
    }

    // Convert to standard chunk
    return convertToStandardChunk(chunk), nil
}
```

### Phase 5: Testing & Documentation
1. ✅ Write unit tests for all components
2. ✅ Add integration tests with mock server
3. ✅ Test cloud vs local endpoint switching
4. ✅ Test authentication flow
5. ✅ Test error handling and retries
6. ✅ Create provider-specific documentation
7. ✅ Add examples to `examples/ollama/`
8. ✅ Update main README.md

### Phase 6: Advanced Features (Future)
1. ⏳ Embeddings support (`POST /api/embeddings`)
2. ⏳ Model management (pull, push, delete)
3. ⏳ Streaming with function calls
4. ⏳ Structured output support
5. ⏳ Multi-modal improvements
6. ⏳ Custom model support (Modelfile)

---

## Testing Strategy

### Unit Tests

#### Authentication Tests
```go
func TestOllamaProvider_IsAuthenticated(t *testing.T) {
    // Test local endpoint (no auth needed)
    localConfig := types.ProviderConfig{
        BaseURL: "http://localhost:11434",
    }
    localProvider := NewOllamaProvider(localConfig)
    assert.True(t, localProvider.IsAuthenticated())

    // Test cloud endpoint (auth required)
    cloudConfig := types.ProviderConfig{
        BaseURL: "https://ollama.com/api",
        APIKey:  "test-key",
    }
    cloudProvider := NewOllamaProvider(cloudConfig)
    assert.True(t, cloudProvider.IsAuthenticated())

    // Test cloud endpoint (no auth)
    noAuthConfig := types.ProviderConfig{
        BaseURL: "https://ollama.com/api",
    }
    noAuthProvider := NewOllamaProvider(noAuthConfig)
    assert.False(t, noAuthProvider.IsAuthenticated())
}
```

#### Model Listing Tests
```go
func TestOllamaProvider_GetModels(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "/api/tags", r.URL.Path)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "models": []map[string]interface{}{
                {
                    "name": "llama3.1:8b",
                    "modified_at": "2024-12-10T12:00:00Z",
                    "size": 4661224448,
                },
            },
        })
    }))
    defer server.Close()

    config := types.ProviderConfig{
        BaseURL: server.URL,
    }
    provider := NewOllamaProvider(config)

    models, err := provider.GetModels(context.Background())
    assert.NoError(t, err)
    assert.Len(t, models, 1)
    assert.Equal(t, "llama3.1:8b", models[0].ID)
}
```

#### Chat Completion Tests
```go
func TestOllamaProvider_GenerateChatCompletion(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "/api/chat/completions", r.URL.Path)
        assert.Equal(t, "POST", r.Method)

        // Stream response
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"Hello"}}]}`)
        fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"!"}}]}`)
        fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"total_tokens":25}}`)
        fmt.Fprintf(w, "data: [DONE]\n\n")
    }))
    defer server.Close()

    config := types.ProviderConfig{
        BaseURL: server.URL,
    }
    provider := NewOllamaProvider(config)

    stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Model: "llama3.1:8b",
        Messages: []types.Message{
            {Role: "user", Content: "Hello"},
        },
    })
    assert.NoError(t, err)

    var content string
    for {
        chunk, err := stream.Next()
        if err == io.EOF {
            break
        }
        assert.NoError(t, err)
        content += chunk.Content
    }

    assert.Equal(t, "Hello!", content)
}
```

### Integration Tests

#### End-to-End Test
```go
func TestOllamaIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Requires local Ollama running
    config := types.ProviderConfig{
        Type:    types.ProviderTypeOllama,
        BaseURL: "http://localhost:11434",
        DefaultModel: "llama3.1:8b",
    }

    provider := NewOllamaProvider(config)

    // Test health check
    err := provider.HealthCheck(context.Background())
    assert.NoError(t, err)

    // Test model listing
    models, err := provider.GetModels(context.Background())
    assert.NoError(t, err)
    assert.NotEmpty(t, models)

    // Test chat completion
    stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Model: "llama3.1:8b",
        Messages: []types.Message{
            {Role: "user", Content: "Say hello"},
        },
    })
    assert.NoError(t, err)

    chunk, err := stream.Next()
    assert.NoError(t, err)
    assert.NotEmpty(t, chunk.Content)
}
```

### Manual Testing Checklist
- [ ] Local Ollama connection (http://localhost:11434)
- [ ] Cloud Ollama connection (https://ollama.com/api)
- [ ] API key authentication
- [ ] Model listing (local and cloud)
- [ ] Chat completion (streaming and non-streaming)
- [ ] Tool calling
- [ ] Multimodal (images in messages)
- [ ] Error handling (network errors, auth errors, rate limits)
- [ ] Health checks
- [ ] Metrics collection

---

## References

### Official Documentation
- [Ollama Cloud Documentation](https://docs.ollama.com/cloud)
- [Ollama API Documentation](https://github.com/ollama/ollama/blob/main/docs/api.md)
- [Ollama Authentication Guide](https://docs.ollama.com/api/authentication)
- [Cloud Models Announcement](https://ollama.com/blog/cloud-models)

### API Specifications
- [OpenAI API Compatibility](https://platform.openai.com/docs/api-reference/chat)
- [Ollama REST API on Postman](https://www.postman.com/postman-student-programs/ollama-api/documentation/suc47x8/ollama-rest-api)

### Community Resources
- [Ollama GitHub Repository](https://github.com/ollama/ollama)
- [Ollama Model Library](https://ollama.com/search?c=cloud)
- [n8n Ollama Integration](https://docs.n8n.io/integrations/builtin/credentials/ollama/)

### Internal References
- Provider interface: `pkg/types/provider.go`
- Base provider: `pkg/providers/base/provider.go`
- Auth helper: `pkg/providers/common/auth/auth_helper.go`
- Model cache: `pkg/providers/common/models/model_cache.go`
- Example provider: `pkg/providers/cerebras/provider.go` (similar structure)
- Factory: `pkg/factory/factory.go`

---

## Implementation Notes

### Design Patterns
- **Factory Pattern**: Provider registration via factory
- **Adapter Pattern**: Convert between Ollama and standard formats
- **Strategy Pattern**: Different auth strategies for local vs cloud
- **Template Pattern**: Base provider provides common functionality

### Error Handling
- Use typed errors from `pkg/types/errors.go`
- Map Ollama HTTP status codes to appropriate error types:
  - 401 → `AuthError`
  - 404 → `NotFoundError`
  - 429 → `RateLimitError`
  - 500+ → `ProviderError`
  - Network errors → `NetworkError`

### Performance Considerations
- Cache model listings (5 min TTL)
- Reuse HTTP client and connections
- Stream responses by default (reduce memory)
- Implement request timeout (30s default)
- Connection pooling via http.Client

### Security Considerations
- Never log API keys
- Use HTTPS for cloud endpoints
- Validate BaseURL to prevent SSRF
- Sanitize user inputs in requests
- Handle rate limits gracefully

### Code Quality
- Follow Go conventions and idioms
- 100% test coverage goal
- Use golangci-lint for static analysis
- Document all exported functions
- Include usage examples in godoc

---

## Success Criteria

### Minimum Viable Product (MVP)
- ✅ Provider registration in factory
- ✅ Basic authentication (API key for cloud)
- ✅ Model listing with caching
- ✅ Chat completion (streaming)
- ✅ Health checks
- ✅ Unit tests (>80% coverage)
- ✅ Integration tests
- ✅ Documentation

### Production Ready
- ✅ All MVP criteria
- ✅ Tool calling support
- ✅ Multi-key failover
- ✅ Rate limit handling
- ✅ Comprehensive error handling
- ✅ Metrics collection
- ✅ Examples and tutorials
- ✅ Performance benchmarks

### Future Enhancements
- ⏳ Embeddings API
- ⏳ Model management (pull/push/delete)
- ⏳ Structured outputs
- ⏳ Custom Modelfile support
- ⏳ Advanced multimodal (audio, video)
- ⏳ Fine-tuning support (if available)

---

**Last Updated**: 2024-12-10
**Status**: Planning Phase
**Next Steps**: Begin Phase 1 implementation
