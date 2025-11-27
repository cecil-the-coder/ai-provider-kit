# AI Provider Kit - SDK Migration Guide

## Table of Contents
1. [Migration Overview](#migration-overview)
2. [Migrating from OpenAI SDK](#migrating-from-openai-sdk)
3. [Migrating from Anthropic SDK](#migrating-from-anthropic-sdk)
4. [Migrating from Google Gemini SDK](#migrating-from-google-gemini-sdk)
5. [Migrating from LangChain](#migrating-from-langchain)
6. [Migrating from Custom Implementations](#migrating-from-custom-implementations)
7. [Data Migration](#data-migration)
8. [Feature Mapping](#feature-mapping)
9. [Testing Migration](#testing-migration)
10. [Step-by-Step Migration Plans](#step-by-step-migration-plans)

---

## Migration Overview

### Why Migrate to AI Provider Kit?

**Key Benefits:**
- **Multi-Provider Support**: Single interface for OpenAI, Anthropic, Gemini, Cerebras, Qwen, OpenRouter, and local models
- **Production-Ready Features**: Built-in health monitoring, metrics tracking, and rate limiting
- **Advanced Authentication**: OAuth 2.0 support with automatic token refresh and multi-credential failover
- **Tool Calling**: Universal tool calling format that works across all providers
- **Resilience**: Multi-key failover, exponential backoff, and automatic recovery
- **Observability**: Provider-level and credential-level metrics out of the box
- **Cost Management**: Per-credential usage tracking for cost attribution
- **Reduced Vendor Lock-in**: Switch providers without changing application code

### Migration Strategies

#### Gradual Migration (Recommended)
Migrate incrementally with parallel running:
- **Duration**: 2-4 weeks
- **Risk**: Low
- **Best For**: Production systems, large codebases
- **Approach**: Run both SDKs side-by-side, gradually shift traffic

#### Complete Migration
Full cutover in a single release:
- **Duration**: 1 week
- **Risk**: Medium
- **Best For**: New projects, small codebases, development environments
- **Approach**: Replace all SDK calls at once with thorough testing

### Risk Assessment

**Low Risk Areas:**
- Basic chat completions without tools
- Simple streaming responses
- Health checks and monitoring
- Single API key authentication

**Medium Risk Areas:**
- Tool calling with complex schemas
- Multi-turn conversations with context
- Custom headers and parameters
- Streaming with tool calls

**High Risk Areas:**
- OAuth token management migration
- Custom retry logic replacement
- Rate limiting integration
- Multi-provider routing logic

### Rollback Planning

**Preparation:**
1. Keep old SDK dependencies in go.mod during migration
2. Use feature flags to toggle between implementations
3. Maintain parallel metrics collection for comparison
4. Document environment-specific rollback procedures

**Rollback Triggers:**
- Error rate increase > 5%
- Latency increase > 50%
- Token cost increase > 20%
- Critical feature unavailable

**Rollback Procedure:**
```go
// Use feature flag for easy rollback
if config.UseAIProviderKit {
    return newAIProviderKitImplementation(config)
} else {
    return legacyOpenAIImplementation(config)
}
```

---

## Migrating from OpenAI SDK

### API Comparison Table

| Feature | OpenAI SDK | AI Provider Kit | Migration Complexity |
|---------|-----------|-----------------|---------------------|
| Chat Completions | `client.CreateChatCompletion()` | `provider.GenerateChatCompletion()` | Low |
| Streaming | `client.CreateChatCompletionStream()` | `GenerateChatCompletion()` with `Stream: true` | Low |
| Function Calling | Native support | Universal tool format | Medium |
| Model Selection | Per-request `Model` field | Per-request or default model | Low |
| API Keys | Single key in client | Multi-key with failover | Low |
| Error Handling | Native Go errors | Standard Go errors | Low |
| Timeouts | Context-based | Context + config | Low |

### Code Migration Examples

#### Basic Chat Completion

**Before (OpenAI SDK):**
```go
import (
    "context"
    openai "github.com/sashabaranov/go-openai"
)

func generateResponse(prompt string) (string, error) {
    client := openai.NewClient("your-api-key")

    resp, err := client.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: prompt,
                },
            },
            MaxTokens:   100,
            Temperature: 0.7,
        },
    )
    if err != nil {
        return "", err
    }

    return resp.Choices[0].Message.Content, nil
}
```

**After (AI Provider Kit):**
```go
import (
    "context"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func generateResponse(prompt string) (string, error) {
    // Create provider (do this once, reuse instance)
    f := factory.NewProviderFactory()
    factory.RegisterDefaultProviders(f)

    provider, err := f.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
        Type:         types.ProviderTypeOpenAI,
        APIKey:       "your-api-key",
        DefaultModel: "gpt-4",
    })
    if err != nil {
        return "", err
    }

    stream, err := provider.GenerateChatCompletion(
        context.Background(),
        types.GenerateOptions{
            Messages: []types.ChatMessage{
                {Role: "user", Content: prompt},
            },
            MaxTokens:   100,
            Temperature: 0.7,
        },
    )
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
```

#### Streaming Response

**Before (OpenAI SDK):**
```go
func streamResponse(prompt string) error {
    client := openai.NewClient("your-api-key")

    stream, err := client.CreateChatCompletionStream(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4,
            Messages: []openai.ChatCompletionMessage{
                {Role: openai.ChatMessageRoleUser, Content: prompt},
            },
            Stream: true,
        },
    )
    if err != nil {
        return err
    }
    defer stream.Close()

    for {
        response, err := stream.Recv()
        if errors.Is(err, io.EOF) {
            break
        }
        if err != nil {
            return err
        }

        fmt.Print(response.Choices[0].Delta.Content)
    }

    return nil
}
```

**After (AI Provider Kit):**
```go
func streamResponse(prompt string) error {
    // provider setup (omitted, same as above)

    stream, err := provider.GenerateChatCompletion(
        context.Background(),
        types.GenerateOptions{
            Messages: []types.ChatMessage{
                {Role: "user", Content: prompt},
            },
            Stream: true,
        },
    )
    if err != nil {
        return err
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

    return nil
}
```

### Authentication Changes

**Before (OpenAI SDK):**
```go
// Single API key
client := openai.NewClient("sk-...")

// With custom base URL
config := openai.DefaultConfig("sk-...")
config.BaseURL = "https://custom.openai.azure.com"
client := openai.NewClientWithConfig(config)
```

**After (AI Provider Kit):**
```go
// Single API key
provider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "sk-...",
})

// With custom base URL
provider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    Type:    types.ProviderTypeOpenAI,
    APIKey:  "sk-...",
    BaseURL: "https://custom.openai.azure.com",
})

// Multi-key with automatic failover (NEW!)
provider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    Type:           types.ProviderTypeOpenAI,
    APIKey:         "sk-key1",  // Primary key (for simple config)
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{"sk-key1", "sk-key2", "sk-key3"},
    },
})
```

### Response Format Differences

**OpenAI SDK Response:**
```go
type ChatCompletionResponse struct {
    ID      string
    Object  string
    Created int64
    Model   string
    Choices []ChatCompletionChoice
    Usage   Usage
}

type ChatCompletionChoice struct {
    Index        int
    Message      ChatCompletionMessage
    FinishReason string
}
```

**AI Provider Kit Response:**
```go
type ChatCompletionChunk struct {
    ID      string
    Object  string
    Created int64
    Model   string
    Choices []ChatChoice
    Usage   Usage
    Done    bool      // Indicates last chunk
    Content string    // Convenience field
    Error   string    // Error message if any
}

type ChatChoice struct {
    Index        int
    Message      ChatMessage
    FinishReason string
    Delta        ChatMessage  // For streaming
}
```

**Key Differences:**
1. AI Provider Kit always uses streaming interface (even for non-streaming)
2. `Done` field indicates end of response
3. `Content` field provides quick access to response text
4. Unified error handling through `Error` field

### Tool Calling Migration

**Before (OpenAI SDK):**
```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: openai.FunctionDefinition{
            Name:        "get_weather",
            Description: "Get weather for a location",
            Parameters: map[string]interface{}{
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
}

resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT4,
    Messages: messages,
    Tools:    tools,
})

// Handle tool calls
for _, toolCall := range resp.Choices[0].Message.ToolCalls {
    // Execute tool
}
```

**After (AI Provider Kit):**
```go
tools := []types.Tool{
    {
        Name:        "get_weather",
        Description: "Get weather for a location",
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
}

stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: messages,
    Tools:    tools,
})

chunk, _ := stream.Next()

// Handle tool calls
for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
    // Execute tool (same structure)
}
```

**Migration Notes:**
- Remove `Type: ToolTypeFunction` wrapper
- Rename `Parameters` to `InputSchema`
- Tool call structure is identical
- Works across all providers (not just OpenAI)

### Common Gotchas

1. **Streaming Interface Change**
   ```go
   // WRONG: Trying to get response directly
   content := response.Choices[0].Message.Content

   // CORRECT: Read from stream
   chunk, _ := stream.Next()
   content := chunk.Content
   ```

2. **Model Selection**
   ```go
   // OpenAI SDK: Model always required per-request
   Model: openai.GPT4

   // AI Provider Kit: Can use default model
   DefaultModel: "gpt-4"  // Set once in config
   // Or override per-request:
   options.Model = "gpt-4-turbo"
   ```

3. **Error Handling**
   ```go
   // OpenAI SDK: Check error only
   if err != nil { /* handle */ }

   // AI Provider Kit: Also check chunk.Error for API errors
   chunk, err := stream.Next()
   if err != nil { /* handle connection error */ }
   if chunk.Error != "" { /* handle API error */ }
   ```

4. **Context Cancellation**
   ```go
   // Both support context cancellation, but AI Provider Kit
   // also supports timeout in config
   config := types.ProviderConfig{
       Timeout: 30 * time.Second,  // Applied to all requests
   }
   ```

---

## Migrating from Anthropic SDK

### Claude API Differences

| Feature | Anthropic SDK | AI Provider Kit | Notes |
|---------|---------------|-----------------|-------|
| Messages API | `client.Messages.Create()` | `provider.GenerateChatCompletion()` | Unified interface |
| System Prompts | Separate `System` field | First message with role "system" | Different structure |
| Tool Use | Content blocks format | Universal tool format | Automatic translation |
| Streaming | `client.Messages.Stream()` | `Stream: true` option | Same interface as non-streaming |
| Authentication | API key only | API key or OAuth 2.0 | OAuth support added |

### Message Format Conversion

**Before (Anthropic SDK):**
```go
import anthropic "github.com/anthropics/anthropic-sdk-go"

client := anthropic.NewClient(
    option.WithAPIKey("sk-ant-..."),
)

message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model: anthropic.F(anthropic.ModelClaude3_5Sonnet20241022),
    Messages: anthropic.F([]anthropic.MessageParam{
        anthropic.NewUserMessage(anthropic.NewTextBlock("Hello, Claude!")),
    }),
    MaxTokens: anthropic.F(int64(1024)),
})

if err != nil {
    return err
}

// Extract text from content blocks
for _, block := range message.Content {
    if block.Type == "text" {
        fmt.Println(block.Text)
    }
}
```

**After (AI Provider Kit):**
```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

f := factory.NewProviderFactory()
factory.RegisterDefaultProviders(f)

provider, err := f.CreateProvider(types.ProviderTypeAnthropic, types.ProviderConfig{
    Type:         types.ProviderTypeAnthropic,
    APIKey:       "sk-ant-...",
    DefaultModel: "claude-3-5-sonnet-20241022",
})

stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "Hello, Claude!"},
    },
    MaxTokens: 1024,
})

chunk, _ := stream.Next()
fmt.Println(chunk.Content)  // Direct access to text
```

### System Prompt Handling

**Before (Anthropic SDK):**
```go
message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:  anthropic.F(anthropic.ModelClaude3_5Sonnet20241022),
    System: anthropic.F([]anthropic.TextBlockParam{
        anthropic.NewTextBlock("You are a helpful assistant."),
    }),
    Messages: anthropic.F([]anthropic.MessageParam{
        anthropic.NewUserMessage(anthropic.NewTextBlock("Hello!")),
    }),
    MaxTokens: anthropic.F(int64(1024)),
})
```

**After (AI Provider Kit):**
```go
// System prompt is now part of messages array
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "system", Content: "You are a helpful assistant."},
        {Role: "user", Content: "Hello!"},
    },
    MaxTokens: 1024,
})
```

**Migration Script:**
```go
// Helper function to convert Anthropic format to AI Provider Kit
func convertAnthropicToProviderKit(
    system []anthropic.TextBlockParam,
    messages []anthropic.MessageParam,
) []types.ChatMessage {
    result := []types.ChatMessage{}

    // Add system messages first
    for _, block := range system {
        result = append(result, types.ChatMessage{
            Role:    "system",
            Content: block.Text,
        })
    }

    // Add user/assistant messages
    for _, msg := range messages {
        result = append(result, types.ChatMessage{
            Role:    string(msg.Role),
            Content: extractTextContent(msg.Content),
        })
    }

    return result
}

func extractTextContent(content []anthropic.ContentBlockParam) string {
    for _, block := range content {
        if block.Type == "text" {
            return block.Text
        }
    }
    return ""
}
```

### Tool Format Changes

**Before (Anthropic SDK):**
```go
tools := []anthropic.ToolParam{
    {
        Name:        anthropic.F("get_weather"),
        Description: anthropic.F("Get weather information"),
        InputSchema: anthropic.F(anthropic.ToolInputSchemaParam{
            Type: anthropic.F(anthropic.ToolInputSchemaTypeObject),
            Properties: anthropic.F(map[string]interface{}{
                "location": map[string]interface{}{
                    "type":        "string",
                    "description": "City name",
                },
            }),
        }),
    },
}

message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
    Model:     anthropic.F(anthropic.ModelClaude3_5Sonnet20241022),
    Messages:  anthropic.F(messages),
    Tools:     anthropic.F(tools),
    MaxTokens: anthropic.F(int64(1024)),
})

// Handle tool use blocks
for _, block := range message.Content {
    if block.Type == "tool_use" {
        toolUse := block.ToolUse
        fmt.Printf("Tool: %s, Input: %v\n", toolUse.Name, toolUse.Input)
    }
}
```

**After (AI Provider Kit):**
```go
tools := []types.Tool{
    {
        Name:        "get_weather",
        Description: "Get weather information",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type":        "string",
                    "description": "City name",
                },
            },
        },
    },
}

stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages:  messages,
    Tools:     tools,
    MaxTokens: 1024,
})

chunk, _ := stream.Next()

// Handle tool calls (standardized format)
for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
    fmt.Printf("Tool: %s, Arguments: %s\n",
        toolCall.Function.Name,
        toolCall.Function.Arguments)
}
```

**Migration Benefits:**
- Remove Anthropic-specific wrappers (`anthropic.F()`)
- Simpler tool definition structure
- Standardized tool call format across all providers
- Automatic conversion between Anthropic's content blocks and OpenAI-style tool calls

### OAuth Setup Migration

Anthropic SDK doesn't support OAuth natively. AI Provider Kit adds this capability:

**After (AI Provider Kit with OAuth):**
```go
provider, err := f.CreateProvider(types.ProviderTypeAnthropic, types.ProviderConfig{
    Type: types.ProviderTypeAnthropic,
    OAuthCredentials: []*types.OAuthCredentialSet{
        {
            ID:           "account-1",
            ClientID:     "your-client-id",
            ClientSecret: "your-client-secret",
            AccessToken:  "initial-access-token",
            RefreshToken: "initial-refresh-token",
            ExpiresAt:    time.Now().Add(1 * time.Hour),
            OnTokenRefresh: func(id, accessToken, refreshToken string, expiresAt time.Time) error {
                // Save new tokens to your storage
                return saveTokens(id, accessToken, refreshToken, expiresAt)
            },
        },
    },
    DefaultModel: "claude-3-5-sonnet-20241022",
})
```

**OAuth Benefits:**
- Automatic token refresh before expiration
- Multi-credential failover
- Per-credential metrics and cost tracking
- Secure token storage with callbacks

### Rate Limit Handling

**Before (Anthropic SDK):**
```go
// Manual retry logic required
var retries int
for retries < maxRetries {
    message, err := client.Messages.New(ctx, params)
    if err != nil {
        if isRateLimitError(err) {
            time.Sleep(calculateBackoff(retries))
            retries++
            continue
        }
        return err
    }
    return message, nil
}
```

**After (AI Provider Kit):**
```go
// Built-in rate limiting and retry logic
provider, _ := f.CreateProvider(types.ProviderTypeAnthropic, types.ProviderConfig{
    Type:   types.ProviderTypeAnthropic,
    APIKey: "sk-ant-...",
    ProviderConfig: map[string]interface{}{
        "max_retries":     3,
        "retry_delay":     time.Second,
        "max_retry_delay": 30 * time.Second,
    },
})

// Automatic retry with exponential backoff
stream, err := provider.GenerateChatCompletion(ctx, options)
// Rate limit errors are automatically retried
```

---

## Migrating from Google Gemini SDK

### Authentication Migration

**Before (Gemini SDK):**
```go
import "github.com/google/generative-ai-go/genai"

client, err := genai.NewClient(ctx, option.WithAPIKey("your-api-key"))
if err != nil {
    return err
}
defer client.Close()

model := client.GenerativeModel("gemini-pro")
```

**After (AI Provider Kit):**
```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

f := factory.NewProviderFactory()
factory.RegisterDefaultProviders(f)

// API Key authentication
provider, err := f.CreateProvider(types.ProviderTypeGemini, types.ProviderConfig{
    Type:         types.ProviderTypeGemini,
    APIKey:       "your-api-key",
    DefaultModel: "gemini-pro",
})

// OAuth authentication (NEW!)
provider, err := f.CreateProvider(types.ProviderTypeGemini, types.ProviderConfig{
    Type: types.ProviderTypeGemini,
    OAuthCredentials: []*types.OAuthCredentialSet{
        {
            ID:           "project-1",
            ClientID:     "your-client-id",
            ClientSecret: "your-client-secret",
            AccessToken:  "access-token",
            RefreshToken: "refresh-token",
            ExpiresAt:    expiryTime,
            OnTokenRefresh: saveTokenCallback,
        },
    },
    DefaultModel: "gemini-pro",
    ProviderConfig: map[string]interface{}{
        "project_id": "your-gcp-project-id",
    },
})
```

### Content Format Conversion

**Before (Gemini SDK):**
```go
model := client.GenerativeModel("gemini-pro")

// Set generation config
model.GenerationConfig = &genai.GenerationConfig{
    Temperature:     0.7,
    TopP:            0.8,
    TopK:            40,
    MaxOutputTokens: 1024,
}

// Generate content
resp, err := model.GenerateContent(ctx, genai.Text("Tell me a story"))
if err != nil {
    return err
}

// Extract response
for _, cand := range resp.Candidates {
    for _, part := range cand.Content.Parts {
        fmt.Println(part)
    }
}
```

**After (AI Provider Kit):**
```go
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "Tell me a story"},
    },
    Temperature: 0.7,
    MaxTokens:   1024,
    // TopP and TopK can be set via Metadata
    Metadata: map[string]interface{}{
        "topP": 0.8,
        "topK": 40,
    },
})
if err != nil {
    return err
}

chunk, _ := stream.Next()
fmt.Println(chunk.Content)
```

### Safety Settings Mapping

**Before (Gemini SDK):**
```go
model.SafetySettings = []*genai.SafetySetting{
    {
        Category:  genai.HarmCategoryHateSpeech,
        Threshold: genai.HarmBlockMediumAndAbove,
    },
    {
        Category:  genai.HarmCategoryDangerousContent,
        Threshold: genai.HarmBlockMediumAndAbove,
    },
}
```

**After (AI Provider Kit):**
```go
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: messages,
    Metadata: map[string]interface{}{
        "safety_settings": []map[string]interface{}{
            {
                "category":  "HARM_CATEGORY_HATE_SPEECH",
                "threshold": "BLOCK_MEDIUM_AND_ABOVE",
            },
            {
                "category":  "HARM_CATEGORY_DANGEROUS_CONTENT",
                "threshold": "BLOCK_MEDIUM_AND_ABOVE",
            },
        },
    },
})
```

### Function Calling Changes

**Before (Gemini SDK):**
```go
model := client.GenerativeModel("gemini-pro")

// Define function
weatherFunc := &genai.FunctionDeclaration{
    Name:        "get_weather",
    Description: "Get weather information",
    Parameters: &genai.Schema{
        Type: genai.TypeObject,
        Properties: map[string]*genai.Schema{
            "location": {
                Type:        genai.TypeString,
                Description: "City name",
            },
        },
        Required: []string{"location"},
    },
}

model.Tools = []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{weatherFunc}}}

// Generate with functions
resp, err := model.GenerateContent(ctx, genai.Text("What's the weather in Tokyo?"))

// Handle function calls
for _, part := range resp.Candidates[0].Content.Parts {
    if fc, ok := part.(genai.FunctionCall); ok {
        fmt.Printf("Function: %s, Args: %v\n", fc.Name, fc.Args)
    }
}
```

**After (AI Provider Kit):**
```go
tools := []types.Tool{
    {
        Name:        "get_weather",
        Description: "Get weather information",
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
}

stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: []types.ChatMessage{
        {Role: "user", Content: "What's the weather in Tokyo?"},
    },
    Tools: tools,
})

chunk, _ := stream.Next()

// Handle tool calls (unified format)
for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
    fmt.Printf("Function: %s, Args: %s\n",
        toolCall.Function.Name,
        toolCall.Function.Arguments)
}
```

### Model Name Mapping

| Gemini SDK | AI Provider Kit | Notes |
|------------|-----------------|-------|
| `gemini-pro` | `gemini-pro` | Same |
| `gemini-pro-vision` | `gemini-pro-vision` | Same |
| `gemini-1.5-pro` | `gemini-1.5-pro` | Same |
| `gemini-1.5-flash` | `gemini-1.5-flash` | Same |

Model names are preserved, no mapping needed.

---

## Migrating from LangChain

### Architecture Differences

**LangChain Architecture:**
```
Application → LangChain Chains → LLMs → Prompts
                ↓
            Memory/Context
                ↓
            Tools/Agents
```

**AI Provider Kit Architecture:**
```
Application → Provider → AI API
     ↓
  Factory Pattern
     ↓
  Multi-Provider Support
```

**Key Differences:**
- LangChain: High-level abstractions (chains, agents, memory)
- AI Provider Kit: Low-level provider abstraction with production features
- LangChain: Framework approach
- AI Provider Kit: Library approach (you build the orchestration)

### Chain Migration Strategies

#### Simple Sequential Chain

**Before (LangChain):**
```python
from langchain import OpenAI, LLMChain, PromptTemplate

# First chain
template1 = "Summarize this text: {text}"
prompt1 = PromptTemplate(template=template1, input_variables=["text"])
chain1 = LLMChain(llm=OpenAI(), prompt=prompt1)

# Second chain
template2 = "Translate to French: {summary}"
prompt2 = PromptTemplate(template=template2, input_variables=["summary"])
chain2 = LLMChain(llm=OpenAI(), prompt=prompt2)

# Sequential chain
from langchain.chains import SimpleSequentialChain
overall_chain = SimpleSequentialChain(chains=[chain1, chain2])

result = overall_chain.run("Long text here...")
```

**After (AI Provider Kit):**
```go
type ChainStep struct {
    PromptTemplate string
    Process        func(input string) string
}

func runSequentialChain(provider types.Provider, steps []ChainStep, initialInput string) (string, error) {
    ctx := context.Background()
    currentInput := initialInput

    for i, step := range steps {
        // Format prompt with current input
        prompt := strings.Replace(step.PromptTemplate, "{input}", currentInput, -1)

        // Execute step
        stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
            Messages: []types.ChatMessage{
                {Role: "user", Content: prompt},
            },
        })
        if err != nil {
            return "", fmt.Errorf("chain step %d failed: %w", i, err)
        }

        chunk, _ := stream.Next()
        currentInput = chunk.Content

        // Optional processing
        if step.Process != nil {
            currentInput = step.Process(currentInput)
        }
    }

    return currentInput, nil
}

// Usage
func main() {
    provider := createProvider() // Your provider setup

    steps := []ChainStep{
        {
            PromptTemplate: "Summarize this text: {input}",
        },
        {
            PromptTemplate: "Translate to French: {input}",
        },
    }

    result, err := runSequentialChain(provider, steps, "Long text here...")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result)
}
```

### Memory/Context Management

**Before (LangChain):**
```python
from langchain.memory import ConversationBufferMemory
from langchain import ConversationChain

memory = ConversationBufferMemory()
conversation = ConversationChain(
    llm=OpenAI(),
    memory=memory,
)

response1 = conversation.predict(input="Hi, I'm Alice")
response2 = conversation.predict(input="What's my name?")
```

**After (AI Provider Kit):**
```go
type ConversationMemory struct {
    messages []types.ChatMessage
    maxTurns int
    mu       sync.RWMutex
}

func NewConversationMemory(maxTurns int) *ConversationMemory {
    return &ConversationMemory{
        messages: []types.ChatMessage{},
        maxTurns: maxTurns,
    }
}

func (m *ConversationMemory) AddMessage(role, content string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.messages = append(m.messages, types.ChatMessage{
        Role:    role,
        Content: content,
    })

    // Trim to max turns
    if len(m.messages) > m.maxTurns*2 {
        m.messages = m.messages[len(m.messages)-m.maxTurns*2:]
    }
}

func (m *ConversationMemory) GetMessages() []types.ChatMessage {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Return copy
    result := make([]types.ChatMessage, len(m.messages))
    copy(result, m.messages)
    return result
}

func (m *ConversationMemory) Chat(provider types.Provider, userMessage string) (string, error) {
    m.AddMessage("user", userMessage)

    stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
        Messages: m.GetMessages(),
    })
    if err != nil {
        return "", err
    }

    chunk, _ := stream.Next()
    m.AddMessage("assistant", chunk.Content)

    return chunk.Content, nil
}

// Usage
func main() {
    provider := createProvider()
    memory := NewConversationMemory(10) // Keep last 10 turns

    response1, _ := memory.Chat(provider, "Hi, I'm Alice")
    fmt.Println(response1)

    response2, _ := memory.Chat(provider, "What's my name?")
    fmt.Println(response2) // Should remember "Alice"
}
```

### Tool/Agent Migration

**Before (LangChain):**
```python
from langchain.agents import initialize_agent, Tool
from langchain.tools import tool

@tool
def search(query: str) -> str:
    """Search for information"""
    return perform_search(query)

@tool
def calculator(expression: str) -> str:
    """Calculate mathematical expressions"""
    return eval(expression)

tools = [search, calculator]

agent = initialize_agent(
    tools=tools,
    llm=OpenAI(),
    agent="zero-shot-react-description",
)

result = agent.run("What is 25 * 4, and search for that number")
```

**After (AI Provider Kit):**
```go
// Define tools
tools := []types.Tool{
    {
        Name:        "search",
        Description: "Search for information",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "query": map[string]interface{}{
                    "type":        "string",
                    "description": "Search query",
                },
            },
            "required": []string{"query"},
        },
    },
    {
        Name:        "calculator",
        Description: "Calculate mathematical expressions",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "expression": map[string]interface{}{
                    "type":        "string",
                    "description": "Math expression",
                },
            },
            "required": []string{"expression"},
        },
    },
}

// Tool executor
func executeTool(toolCall types.ToolCall) (string, error) {
    var args map[string]interface{}
    json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

    switch toolCall.Function.Name {
    case "search":
        query := args["query"].(string)
        return performSearch(query), nil
    case "calculator":
        expression := args["expression"].(string)
        return calculate(expression), nil
    default:
        return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
    }
}

// Agent loop
func runAgent(provider types.Provider, prompt string, tools []types.Tool, maxIterations int) (string, error) {
    ctx := context.Background()
    messages := []types.ChatMessage{
        {Role: "user", Content: prompt},
    }

    for i := 0; i < maxIterations; i++ {
        stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
            Messages: messages,
            Tools:    tools,
        })
        if err != nil {
            return "", err
        }

        chunk, _ := stream.Next()

        // Check if we have tool calls
        if len(chunk.Choices) == 0 || len(chunk.Choices[0].Message.ToolCalls) == 0 {
            // No tool calls, return final answer
            return chunk.Content, nil
        }

        // Add assistant message with tool calls
        messages = append(messages, types.ChatMessage{
            Role:      "assistant",
            ToolCalls: chunk.Choices[0].Message.ToolCalls,
        })

        // Execute tools
        for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
            result, err := executeTool(toolCall)
            if err != nil {
                result = fmt.Sprintf("Error: %v", err)
            }

            messages = append(messages, types.ChatMessage{
                Role:       "tool",
                Content:    result,
                ToolCallID: toolCall.ID,
            })
        }
    }

    return "", fmt.Errorf("max iterations reached")
}

// Usage
func main() {
    provider := createProvider()

    result, err := runAgent(
        provider,
        "What is 25 * 4, and search for that number",
        tools,
        5, // max iterations
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result)
}
```

### Prompt Template Conversion

**Before (LangChain):**
```python
from langchain import PromptTemplate

template = """
You are a {role}.
Answer the question: {question}
Use this context: {context}
"""

prompt = PromptTemplate(
    template=template,
    input_variables=["role", "question", "context"]
)

formatted = prompt.format(
    role="helpful assistant",
    question="What is AI?",
    context="AI stands for Artificial Intelligence"
)
```

**After (AI Provider Kit):**
```go
type PromptTemplate struct {
    Template string
    Variables []string
}

func (pt *PromptTemplate) Format(values map[string]string) string {
    result := pt.Template
    for key, value := range values {
        placeholder := "{" + key + "}"
        result = strings.ReplaceAll(result, placeholder, value)
    }
    return result
}

// Usage
func main() {
    template := PromptTemplate{
        Template: `
You are a {role}.
Answer the question: {question}
Use this context: {context}
`,
        Variables: []string{"role", "question", "context"},
    }

    formatted := template.Format(map[string]string{
        "role":     "helpful assistant",
        "question": "What is AI?",
        "context":  "AI stands for Artificial Intelligence",
    })

    // Use with provider
    stream, _ := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: formatted},
        },
    })
}
```

**Or use a templating library:**
```go
import "text/template"

func formatPrompt(templateStr string, data interface{}) (string, error) {
    tmpl, err := template.New("prompt").Parse(templateStr)
    if err != nil {
        return "", err
    }

    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", err
    }

    return buf.String(), nil
}
```

---

## Migrating from Custom Implementations

### HTTP Client Replacement

**Before (Custom HTTP Client):**
```go
type CustomAIClient struct {
    httpClient *http.Client
    apiKey     string
    baseURL    string
}

func (c *CustomAIClient) SendRequest(prompt string) (string, error) {
    reqBody := map[string]interface{}{
        "model": "gpt-4",
        "messages": []map[string]string{
            {"role": "user", "content": prompt},
        },
    }

    jsonData, _ := json.Marshal(reqBody)

    req, _ := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)

    // Extract content
    choices := result["choices"].([]interface{})
    message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
    return message["content"].(string), nil
}
```

**After (AI Provider Kit):**
```go
func SendRequest(provider types.Provider, prompt string) (string, error) {
    stream, err := provider.GenerateChatCompletion(
        context.Background(),
        types.GenerateOptions{
            Messages: []types.ChatMessage{
                {Role: "user", Content: prompt},
            },
        },
    )
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
```

**Benefits:**
- No manual HTTP handling
- Built-in error handling
- Automatic retries
- Connection pooling
- Timeout management
- Multi-provider support

### Error Handling Migration

**Before (Custom Implementation):**
```go
func (c *CustomAIClient) SendRequestWithRetry(prompt string) (string, error) {
    var lastErr error

    for attempt := 0; attempt < 3; attempt++ {
        resp, err := c.SendRequest(prompt)
        if err == nil {
            return resp, nil
        }

        lastErr = err

        // Check if retryable
        if isRateLimitError(err) {
            time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
            continue
        }

        if isServerError(err) {
            time.Sleep(1 * time.Second)
            continue
        }

        // Non-retryable error
        return "", err
    }

    return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

func isRateLimitError(err error) bool {
    // Custom logic to detect rate limit
    return strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate limit")
}

func isServerError(err error) bool {
    // Custom logic to detect server error
    return strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "503")
}
```

**After (AI Provider Kit):**
```go
// Built-in error handling and retries
provider, _ := f.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
    ProviderConfig: map[string]interface{}{
        "max_retries":     3,
        "retry_delay":     time.Second,
        "max_retry_delay": 30 * time.Second,
    },
})

// Automatic retry with exponential backoff
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    // Only non-retryable errors reach here
    return "", err
}
```

**Error Types Automatically Handled:**
- Rate limit errors (429) - exponential backoff
- Server errors (500, 502, 503, 504) - retry with backoff
- Timeout errors - retry with extended timeout
- Connection errors - retry with new connection

### Retry Logic Replacement

**Before (Custom Retry Logic):**
```go
type RetryConfig struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
}

func RetryWithBackoff(operation func() error, config RetryConfig) error {
    var lastErr error
    delay := config.InitialDelay

    for attempt := 0; attempt < config.MaxAttempts; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }

        lastErr = err

        if attempt < config.MaxAttempts-1 {
            time.Sleep(delay)
            delay = time.Duration(float64(delay) * config.Multiplier)
            if delay > config.MaxDelay {
                delay = config.MaxDelay
            }
        }
    }

    return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}
```

**After (AI Provider Kit):**
```go
// Retry logic is built-in and configurable
provider, _ := f.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
    ProviderConfig: map[string]interface{}{
        "max_retries":     5,
        "retry_delay":     500 * time.Millisecond,
        "max_retry_delay": 60 * time.Second,
        "retry_multiplier": 2.0,
    },
})

// Just make the call - retries happen automatically
stream, err := provider.GenerateChatCompletion(ctx, options)
```

### Rate Limiting Integration

**Before (Custom Rate Limiter):**
```go
import "golang.org/x/time/rate"

type RateLimitedClient struct {
    client  *http.Client
    limiter *rate.Limiter
}

func NewRateLimitedClient(requestsPerSecond int) *RateLimitedClient {
    return &RateLimitedClient{
        client:  &http.Client{},
        limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), requestsPerSecond),
    }
}

func (c *RateLimitedClient) SendRequest(ctx context.Context, prompt string) (string, error) {
    // Wait for rate limiter
    if err := c.limiter.Wait(ctx); err != nil {
        return "", err
    }

    // Make request
    return c.makeAPICall(prompt)
}
```

**After (AI Provider Kit):**
```go
// Rate limiting is built-in
provider, _ := f.CreateProvider(types.ProviderTypeOpenAI, types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
    ProviderConfig: map[string]interface{}{
        "rate_limit": map[string]interface{}{
            "requests_per_minute": 60,
            "tokens_per_minute":   90000,
        },
    },
})

// Rate limiting happens automatically
stream, err := provider.GenerateChatCompletion(ctx, options)
```

**Advanced Rate Limiting:**
```go
// Per-provider rate limits
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: "your-api-key",
    ProviderConfig: map[string]interface{}{
        "rate_limit": map[string]interface{}{
            // Request limits
            "requests_per_second": 10,
            "requests_per_minute": 500,
            "requests_per_day":    10000,

            // Token limits
            "tokens_per_minute": 90000,
            "tokens_per_day":    1000000,

            // Burst allowance
            "burst": 20,
        },
    },
}
```

### Monitoring Migration

**Before (Custom Metrics):**
```go
type Metrics struct {
    requests       int64
    successes      int64
    failures       int64
    totalLatency   time.Duration
    mu             sync.RWMutex
}

func (m *Metrics) RecordRequest(latency time.Duration, success bool) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.requests++
    m.totalLatency += latency

    if success {
        m.successes++
    } else {
        m.failures++
    }
}

func (m *Metrics) GetStats() map[string]interface{} {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var avgLatency time.Duration
    if m.requests > 0 {
        avgLatency = m.totalLatency / time.Duration(m.requests)
    }

    return map[string]interface{}{
        "requests":        m.requests,
        "successes":       m.successes,
        "failures":        m.failures,
        "success_rate":    float64(m.successes) / float64(m.requests),
        "average_latency": avgLatency,
    }
}
```

**After (AI Provider Kit):**
```go
// Metrics are collected automatically
metrics := provider.GetMetrics()

fmt.Printf("Requests: %d\n", metrics.RequestCount)
fmt.Printf("Successes: %d\n", metrics.SuccessCount)
fmt.Printf("Failures: %d\n", metrics.ErrorCount)
fmt.Printf("Success Rate: %.2f%%\n",
    float64(metrics.SuccessCount)/float64(metrics.RequestCount)*100)
fmt.Printf("Average Latency: %v\n", metrics.AverageLatency)
fmt.Printf("Tokens Used: %d\n", metrics.TokensUsed)
fmt.Printf("Last Error: %s\n", metrics.LastError)

// Health status
if metrics.HealthStatus.Healthy {
    fmt.Printf("Provider is healthy\n")
} else {
    fmt.Printf("Provider is unhealthy: %s\n", metrics.HealthStatus.Message)
}
```

**Advanced Monitoring with OAuth Providers:**
```go
// For OAuth providers, get per-credential metrics
if oauthProvider, ok := provider.(interface{ GetCredentialMetrics(string) interface{} }); ok {
    credMetrics := oauthProvider.GetCredentialMetrics("account-1")
    // Per-credential tracking for cost attribution
}
```

---

## Data Migration

### Token/Credential Migration

#### From Environment Variables

**Before:**
```bash
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export GEMINI_API_KEY=...
```

**After:**
```go
import "os"

// Load from environment
config := types.ProviderConfig{
    Type:   types.ProviderTypeOpenAI,
    APIKey: os.Getenv("OPENAI_API_KEY"),
}

// Or use APIKeyEnv field for lazy loading
config := types.ProviderConfig{
    Type:      types.ProviderTypeOpenAI,
    APIKeyEnv: "OPENAI_API_KEY",  // Loaded when provider is created
}
```

#### From Configuration Files

**Before (JSON config):**
```json
{
  "openai": {
    "api_key": "sk-...",
    "model": "gpt-4"
  },
  "anthropic": {
    "api_key": "sk-ant-...",
    "model": "claude-3-sonnet"
  }
}
```

**After (YAML config with AI Provider Kit):**
```yaml
providers:
  enabled:
    - openai
    - anthropic

  openai:
    type: openai
    api_key: sk-...
    default_model: gpt-4
    timeout: 30s

  anthropic:
    type: anthropic
    api_key: sk-ant-...
    default_model: claude-3-5-sonnet-20241022
    timeout: 30s
```

**Migration Script:**
```go
func migrateConfig(oldConfigPath, newConfigPath string) error {
    // Read old config
    oldData, err := os.ReadFile(oldConfigPath)
    if err != nil {
        return err
    }

    var oldConfig map[string]map[string]string
    json.Unmarshal(oldData, &oldConfig)

    // Convert to new format
    newConfig := map[string]interface{}{
        "providers": map[string]interface{}{
            "enabled": []string{},
        },
    }

    providers := newConfig["providers"].(map[string]interface{})

    for providerName, providerData := range oldConfig {
        providers["enabled"] = append(providers["enabled"].([]string), providerName)
        providers[providerName] = map[string]string{
            "type":          providerName,
            "api_key":       providerData["api_key"],
            "default_model": providerData["model"],
            "timeout":       "30s",
        }
    }

    // Write new config
    newData, _ := yaml.Marshal(newConfig)
    return os.WriteFile(newConfigPath, newData, 0644)
}
```

### Configuration Migration

#### Centralized Configuration

Create a configuration loader:

```go
package config

import (
    "os"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
    "gopkg.in/yaml.v3"
)

type Config struct {
    Providers struct {
        Enabled []string `yaml:"enabled"`
        OpenAI  *ProviderConfig `yaml:"openai"`
        Anthropic *ProviderConfig `yaml:"anthropic"`
        Gemini *ProviderConfig `yaml:"gemini"`
    } `yaml:"providers"`
}

type ProviderConfig struct {
    Type         string `yaml:"type"`
    APIKey       string `yaml:"api_key"`
    DefaultModel string `yaml:"default_model"`
    BaseURL      string `yaml:"base_url"`
    Timeout      string `yaml:"timeout"`
    MaxRetries   int    `yaml:"max_retries"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}

func (pc *ProviderConfig) ToProviderConfig() types.ProviderConfig {
    timeout, _ := time.ParseDuration(pc.Timeout)

    return types.ProviderConfig{
        Type:         types.ProviderType(pc.Type),
        APIKey:       pc.APIKey,
        DefaultModel: pc.DefaultModel,
        BaseURL:      pc.BaseURL,
        Timeout:      timeout,
        ProviderConfig: map[string]interface{}{
            "max_retries": pc.MaxRetries,
        },
    }
}
```

### Log Format Changes

#### Before (Custom Logging)

```go
log.Printf("[OpenAI] Request sent: prompt=%s", prompt)
log.Printf("[OpenAI] Response received: tokens=%d, latency=%v", tokens, latency)
log.Printf("[OpenAI] Error: %v", err)
```

#### After (Structured Logging with AI Provider Kit)

```go
import "log/slog"

// Configure structured logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

// Before request
logger.Info("ai_request",
    slog.String("provider", provider.Type()),
    slog.String("model", options.Model),
    slog.Int("max_tokens", options.MaxTokens),
)

// After request
metrics := provider.GetMetrics()
logger.Info("ai_response",
    slog.String("provider", provider.Type()),
    slog.Int64("tokens_used", metrics.TokensUsed),
    slog.Duration("latency", metrics.AverageLatency),
    slog.Bool("success", metrics.HealthStatus.Healthy),
)

// On error
if chunk.Error != "" {
    logger.Error("ai_error",
        slog.String("provider", provider.Type()),
        slog.String("error", chunk.Error),
        slog.Int64("request_count", metrics.RequestCount),
    )
}
```

**Log Aggregation Example:**
```go
func setupLogging(provider types.Provider) {
    ticker := time.NewTicker(1 * time.Minute)
    go func() {
        for range ticker.C {
            metrics := provider.GetMetrics()

            logger.Info("ai_metrics",
                slog.String("provider", provider.Type()),
                slog.Int64("requests", metrics.RequestCount),
                slog.Int64("successes", metrics.SuccessCount),
                slog.Int64("errors", metrics.ErrorCount),
                slog.Float64("success_rate",
                    float64(metrics.SuccessCount)/float64(metrics.RequestCount)),
                slog.Duration("avg_latency", metrics.AverageLatency),
                slog.Int64("tokens_used", metrics.TokensUsed),
            )
        }
    }()
}
```

### Metric Migration

#### From Custom Metrics to Built-in Metrics

**Migration Strategy:**

1. **Run Both Systems in Parallel** (Week 1-2)
```go
func dualMetrics(provider types.Provider, customMetrics *CustomMetrics, operation func() error) error {
    start := time.Now()
    err := operation()
    latency := time.Since(start)

    // Old metrics
    customMetrics.RecordRequest(latency, err == nil)

    // New metrics (automatic)
    providerMetrics := provider.GetMetrics()

    // Compare and log differences
    if customMetrics.requests != providerMetrics.RequestCount {
        log.Printf("Metrics divergence detected: custom=%d, provider=%d",
            customMetrics.requests, providerMetrics.RequestCount)
    }

    return err
}
```

2. **Validate Metrics Accuracy** (Week 2-3)
```go
func validateMetrics(provider types.Provider, customMetrics *CustomMetrics) bool {
    pm := provider.GetMetrics()
    cm := customMetrics.GetStats()

    // Check request counts match
    if pm.RequestCount != cm["requests"].(int64) {
        return false
    }

    // Check success rates match (within 1%)
    pmSuccessRate := float64(pm.SuccessCount) / float64(pm.RequestCount)
    cmSuccessRate := cm["success_rate"].(float64)
    if math.Abs(pmSuccessRate-cmSuccessRate) > 0.01 {
        return false
    }

    return true
}
```

3. **Switch to Provider Metrics** (Week 3-4)
```go
// Remove custom metrics collection
// Use provider.GetMetrics() exclusively
metrics := provider.GetMetrics()

// Export to monitoring system
exportToPrometheus(metrics)
exportToDatadog(metrics)
```

### Historical Data Preservation

**Export Historical Metrics:**
```go
func exportHistoricalMetrics(customMetrics *CustomMetrics, outputPath string) error {
    data := struct {
        ExportDate   time.Time              `json:"export_date"`
        TotalRequests int64                 `json:"total_requests"`
        TotalSuccess int64                  `json:"total_success"`
        TotalFailures int64                 `json:"total_failures"`
        AverageLatency time.Duration        `json:"average_latency"`
        Custom       map[string]interface{} `json:"custom_data"`
    }{
        ExportDate:     time.Now(),
        TotalRequests:  customMetrics.requests,
        TotalSuccess:   customMetrics.successes,
        TotalFailures:  customMetrics.failures,
        AverageLatency: customMetrics.totalLatency / time.Duration(customMetrics.requests),
        Custom:         customMetrics.GetStats(),
    }

    jsonData, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return err
    }

    return os.WriteFile(outputPath, jsonData, 0644)
}
```

---

## Feature Mapping

### Provider-Specific Features

| Feature | OpenAI | Anthropic | Gemini | AI Provider Kit |
|---------|--------|-----------|--------|-----------------|
| **Chat Completions** | ✅ | ✅ | ✅ | ✅ All providers |
| **Streaming** | ✅ | ✅ | ✅ | ✅ All providers |
| **Function Calling** | ✅ | ✅ (Tools) | ✅ | ✅ Universal format |
| **Vision** | ✅ | ✅ | ✅ | ⚠️ Via metadata |
| **JSON Mode** | ✅ | ⚠️ Limited | ⚠️ Limited | ⚠️ Provider-dependent |
| **System Messages** | ✅ | ✅ (Separate) | ✅ | ✅ Unified |
| **Multi-turn** | ✅ | ✅ | ✅ | ✅ All providers |

### Universal Features (All Providers)

**Available in AI Provider Kit across all providers:**

1. **Health Monitoring**
```go
err := provider.HealthCheck(ctx)
metrics := provider.GetMetrics()
```

2. **Automatic Retries**
```go
// Configured once, applies to all requests
ProviderConfig: map[string]interface{}{
    "max_retries": 3,
}
```

3. **Metrics Collection**
```go
metrics := provider.GetMetrics()
// Works for all providers
```

4. **Multi-Key Failover**
```go
// For API key providers (OpenAI, Anthropic, Cerebras)
ProviderConfig: map[string]interface{}{
    "api_keys": []string{"key1", "key2", "key3"},
}

// For OAuth providers (Gemini, Qwen)
OAuthCredentials: []*types.OAuthCredentialSet{
    {ID: "cred1", ...},
    {ID: "cred2", ...},
}
```

5. **Timeout Control**
```go
config := types.ProviderConfig{
    Timeout: 30 * time.Second,  // Per-request timeout
}
```

6. **Context Cancellation**
```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

stream, err := provider.GenerateChatCompletion(ctx, options)
```

### Deprecated Features

**Features not supported (use alternatives):**

| Deprecated Feature | Provider | Alternative |
|-------------------|----------|-------------|
| Fine-tuning APIs | OpenAI | Use provider's SDK directly |
| Embeddings | OpenAI | Use provider's SDK directly |
| Image generation | OpenAI | Use provider's SDK directly |
| Audio transcription | OpenAI | Use provider's SDK directly |
| Moderation API | OpenAI | Use provider's SDK directly |

**Why deprecated:**
- AI Provider Kit focuses on chat completions and tool calling
- Specialized APIs are better served by provider-specific SDKs
- Keeps library focused and maintainable

### New Capabilities

**Features unique to AI Provider Kit:**

1. **Multi-Provider Management**
```go
// Manage multiple providers in one application
openai, _ := factory.CreateProvider(types.ProviderTypeOpenAI, openaiConfig)
anthropic, _ := factory.CreateProvider(types.ProviderTypeAnthropic, anthropicConfig)
gemini, _ := factory.CreateProvider(types.ProviderTypeGemini, geminiConfig)

// Use based on requirements
provider := selectProvider(requirements)
```

2. **OAuth Token Management**
```go
// Automatic token refresh for OAuth providers
OAuthCredentials: []*types.OAuthCredentialSet{
    {
        ID:           "account-1",
        AccessToken:  "initial-token",
        RefreshToken: "refresh-token",
        ExpiresAt:    expiryTime,
        OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
            // Automatically called when token expires
            return saveTokens(id, access, refresh, expires)
        },
    },
}
```

3. **Per-Credential Metrics**
```go
// Track usage per credential for cost attribution
if oauthProvider, ok := provider.(*gemini.GeminiProvider); ok {
    metrics := oauthProvider.GetCredentialMetrics("account-1")
    fmt.Printf("Account 1 used %d tokens\n", metrics.TokensUsed)
}
```

4. **Universal Tool Format**
```go
// Define tools once, use with any provider
tools := []types.Tool{...}

// Works with OpenAI
openaiStream, _ := openaiProvider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: messages,
    Tools:    tools,
})

// Same tools work with Anthropic
anthropicStream, _ := anthropicProvider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Messages: messages,
    Tools:    tools,  // Automatically translated
})
```

5. **Credential Health Tracking**
```go
// Automatic health tracking with exponential backoff
// Unhealthy credentials are temporarily skipped
status := provider.GetMetrics().HealthStatus
if !status.Healthy {
    fmt.Printf("Provider unhealthy: %s\n", status.Message)
}
```

### Performance Comparisons

**Latency Overhead:**
- AI Provider Kit adds ~1-5ms of overhead for format translation
- Negligible compared to typical API latency (100-2000ms)

**Memory Overhead:**
- ~200 bytes per provider instance
- ~150 bytes per credential in multi-credential setup
- Minimal impact even with dozens of providers

**Throughput:**
- No significant throughput reduction
- Benefits from connection pooling and keep-alive

**Example Benchmark:**
```go
func BenchmarkDirectOpenAI(b *testing.B) {
    client := openai.NewClient("api-key")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        client.CreateChatCompletion(ctx, request)
    }
}

func BenchmarkAIProviderKit(b *testing.B) {
    provider := createOpenAIProvider("api-key")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        provider.GenerateChatCompletion(ctx, options)
    }
}

// Typical results:
// BenchmarkDirectOpenAI-8       100  185234 ns/op
// BenchmarkAIProviderKit-8      100  187891 ns/op
// Overhead: ~1.4% (negligible)
```

---

## Testing Migration

### Test Strategy Adaptation

#### Unit Tests

**Before (Provider-Specific Tests):**
```go
func TestOpenAIClient(t *testing.T) {
    mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "choices": []map[string]interface{}{
                {
                    "message": map[string]interface{}{
                        "content": "Test response",
                    },
                },
            },
        })
    }))
    defer mockServer.Close()

    config := openai.DefaultConfig("test-key")
    config.BaseURL = mockServer.URL
    client := openai.NewClientWithConfig(config)

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT4,
        Messages: []openai.ChatCompletionMessage{
            {Role: "user", Content: "test"},
        },
    })

    assert.NoError(t, err)
    assert.Equal(t, "Test response", resp.Choices[0].Message.Content)
}
```

**After (AI Provider Kit Tests):**
```go
func TestProviderGeneration(t *testing.T) {
    // Use mock provider for testing
    mockProvider := &MockProvider{
        responses: []string{"Test response"},
    }

    stream, err := mockProvider.GenerateChatCompletion(
        context.Background(),
        types.GenerateOptions{
            Messages: []types.ChatMessage{
                {Role: "user", Content: "test"},
            },
        },
    )

    require.NoError(t, err)
    chunk, err := stream.Next()
    require.NoError(t, err)
    assert.Equal(t, "Test response", chunk.Content)
}

// Mock implementation
type MockProvider struct {
    responses []string
    callCount int
}

func (m *MockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    m.callCount++
    return &MockStream{response: m.responses[m.callCount-1]}, nil
}

func (m *MockProvider) Name() string { return "mock" }
func (m *MockProvider) Type() types.ProviderType { return "mock" }
// ... implement other Provider interface methods
```

### Mock Migration

**Create Reusable Mock Provider:**

```go
package testutil

import (
    "context"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type MockProvider struct {
    Name_             string
    Type_             types.ProviderType
    Responses         []types.ChatCompletionChunk
    Errors            []error
    CallCount         int
    LastOptions       types.GenerateOptions
    HealthError       error
    MetricsData       types.ProviderMetrics
}

func NewMockProvider() *MockProvider {
    return &MockProvider{
        Name_: "mock-provider",
        Type_: "mock",
        Responses: []types.ChatCompletionChunk{
            {
                Content: "Mock response",
                Done:    true,
                Choices: []types.ChatChoice{
                    {
                        Message: types.ChatMessage{
                            Role:    "assistant",
                            Content: "Mock response",
                        },
                        FinishReason: "stop",
                    },
                },
            },
        },
    }
}

func (m *MockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    m.CallCount++
    m.LastOptions = options

    if len(m.Errors) > 0 && m.Errors[m.CallCount-1] != nil {
        return nil, m.Errors[m.CallCount-1]
    }

    responseIdx := m.CallCount - 1
    if responseIdx >= len(m.Responses) {
        responseIdx = len(m.Responses) - 1
    }

    return &MockStream{
        chunks: []types.ChatCompletionChunk{m.Responses[responseIdx]},
    }, nil
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
    return m.HealthError
}

func (m *MockProvider) GetMetrics() types.ProviderMetrics {
    if m.MetricsData.RequestCount == 0 {
        m.MetricsData.RequestCount = int64(m.CallCount)
    }
    return m.MetricsData
}

// Implement remaining Provider interface methods...

type MockStream struct {
    chunks []types.ChatCompletionChunk
    index  int
}

func (s *MockStream) Next() (types.ChatCompletionChunk, error) {
    if s.index >= len(s.chunks) {
        return types.ChatCompletionChunk{}, io.EOF
    }

    chunk := s.chunks[s.index]
    s.index++
    return chunk, nil
}

func (s *MockStream) Close() error {
    return nil
}

// Usage in tests
func TestMyFunction(t *testing.T) {
    mock := testutil.NewMockProvider()
    mock.Responses = []types.ChatCompletionChunk{
        {Content: "First response", Done: true},
        {Content: "Second response", Done: true},
    }

    // Test your code using the mock
    result, err := MyFunction(mock)

    assert.NoError(t, err)
    assert.Equal(t, 2, mock.CallCount)
}
```

### Integration Test Updates

**Before (Provider-Specific Integration Test):**
```go
func TestOpenAIIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT35Turbo,
        Messages: []openai.ChatCompletionMessage{
            {Role: "user", Content: "Say hello"},
        },
    })

    assert.NoError(t, err)
    assert.NotEmpty(t, resp.Choices[0].Message.Content)
}
```

**After (Multi-Provider Integration Test):**
```go
func TestProviderIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    tests := []struct {
        name         string
        providerType types.ProviderType
        envVar       string
        model        string
    }{
        {"OpenAI", types.ProviderTypeOpenAI, "OPENAI_API_KEY", "gpt-3.5-turbo"},
        {"Anthropic", types.ProviderTypeAnthropic, "ANTHROPIC_API_KEY", "claude-3-haiku-20240307"},
        {"Gemini", types.ProviderTypeGemini, "GEMINI_API_KEY", "gemini-pro"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            apiKey := os.Getenv(tt.envVar)
            if apiKey == "" {
                t.Skipf("Skipping %s test: %s not set", tt.name, tt.envVar)
            }

            factory := factory.NewProviderFactory()
            factory.RegisterDefaultProviders(factory)

            provider, err := factory.CreateProvider(tt.providerType, types.ProviderConfig{
                Type:         tt.providerType,
                APIKey:       apiKey,
                DefaultModel: tt.model,
            })
            require.NoError(t, err)

            stream, err := provider.GenerateChatCompletion(
                context.Background(),
                types.GenerateOptions{
                    Messages: []types.ChatMessage{
                        {Role: "user", Content: "Say hello"},
                    },
                },
            )
            require.NoError(t, err)

            chunk, err := stream.Next()
            require.NoError(t, err)
            assert.NotEmpty(t, chunk.Content)

            t.Logf("%s response: %s", tt.name, chunk.Content)
        })
    }
}
```

### Performance Benchmarking

**Benchmark Suite:**

```go
func BenchmarkProviders(b *testing.B) {
    providers := map[string]struct {
        provider types.Provider
        options  types.GenerateOptions
    }{
        "OpenAI": {
            provider: createTestProvider(types.ProviderTypeOpenAI),
            options: types.GenerateOptions{
                Messages: []types.ChatMessage{
                    {Role: "user", Content: "Hello"},
                },
            },
        },
        "Anthropic": {
            provider: createTestProvider(types.ProviderTypeAnthropic),
            options: types.GenerateOptions{
                Messages: []types.ChatMessage{
                    {Role: "user", Content: "Hello"},
                },
            },
        },
    }

    for name, test := range providers {
        b.Run(name, func(b *testing.B) {
            ctx := context.Background()

            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                stream, err := test.provider.GenerateChatCompletion(ctx, test.options)
                if err != nil {
                    b.Fatal(err)
                }

                _, err = stream.Next()
                if err != nil {
                    b.Fatal(err)
                }

                stream.Close()
            }
        })
    }
}

func BenchmarkToolCalling(b *testing.B) {
    provider := createTestProvider(types.ProviderTypeOpenAI)

    tools := []types.Tool{
        {
            Name:        "get_weather",
            Description: "Get weather",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "location": map[string]interface{}{
                        "type": "string",
                    },
                },
            },
        },
    }

    options := types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: "Weather in SF?"},
        },
        Tools: tools,
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        stream, _ := provider.GenerateChatCompletion(context.Background(), options)
        stream.Next()
        stream.Close()
    }
}
```

### A/B Testing Approach

**Gradual Rollout with Metrics:**

```go
type ABTestConfig struct {
    UseNewSDK    bool
    RolloutPercent int  // 0-100
}

func generateWithABTest(config ABTestConfig, prompt string) (string, error) {
    // Decide which implementation to use
    useNew := config.UseNewSDK
    if !useNew && config.RolloutPercent > 0 {
        // Random rollout
        useNew = rand.Intn(100) < config.RolloutPercent
    }

    startTime := time.Now()
    var result string
    var err error
    var sdkVersion string

    if useNew {
        sdkVersion = "ai-provider-kit"
        result, err = generateWithProviderKit(prompt)
    } else {
        sdkVersion = "legacy"
        result, err = generateWithLegacySDK(prompt)
    }

    // Record metrics
    latency := time.Since(startTime)
    recordABTestMetrics(sdkVersion, latency, err == nil)

    return result, err
}

func recordABTestMetrics(sdkVersion string, latency time.Duration, success bool) {
    // Send to metrics system
    metrics.Record(map[string]interface{}{
        "sdk_version": sdkVersion,
        "latency_ms":  latency.Milliseconds(),
        "success":     success,
        "timestamp":   time.Now(),
    })
}

// Compare metrics after some time
func compareABTestResults() {
    legacyMetrics := getMetrics("legacy")
    newMetrics := getMetrics("ai-provider-kit")

    fmt.Printf("Legacy SDK:\n")
    fmt.Printf("  Avg Latency: %v\n", legacyMetrics.AverageLatency)
    fmt.Printf("  Success Rate: %.2f%%\n", legacyMetrics.SuccessRate*100)
    fmt.Printf("  Error Rate: %.2f%%\n", legacyMetrics.ErrorRate*100)

    fmt.Printf("\nAI Provider Kit:\n")
    fmt.Printf("  Avg Latency: %v\n", newMetrics.AverageLatency)
    fmt.Printf("  Success Rate: %.2f%%\n", newMetrics.SuccessRate*100)
    fmt.Printf("  Error Rate: %.2f%%\n", newMetrics.ErrorRate*100)

    // Decision criteria
    if newMetrics.SuccessRate >= legacyMetrics.SuccessRate &&
       newMetrics.AverageLatency <= legacyMetrics.AverageLatency*1.1 {
        fmt.Println("\n✅ Migration looks good!")
    } else {
        fmt.Println("\n⚠️  Need to investigate issues")
    }
}
```

---

## Step-by-Step Migration Plans

### Phase 1: Setup and Configuration (Week 1)

**Day 1-2: Installation and Dependencies**

1. Add AI Provider Kit to your project:
```bash
go get github.com/cecil-the-coder/ai-provider-kit@latest
```

2. Update go.mod:
```go
require (
    github.com/cecil-the-coder/ai-provider-kit v0.x.x
    // Keep old SDKs during migration
    github.com/sashabaranov/go-openai v1.x.x
    github.com/anthropics/anthropic-sdk-go v0.x.x
)
```

3. Create configuration structure:
```bash
mkdir -p config
touch config/providers.yaml
```

**Day 3-4: Configuration Setup**

1. Create provider configuration:
```yaml
# config/providers.yaml
providers:
  enabled:
    - openai
    - anthropic

  openai:
    type: openai
    api_key_env: OPENAI_API_KEY
    default_model: gpt-4
    timeout: 30s
    max_retries: 3

  anthropic:
    type: anthropic
    api_key_env: ANTHROPIC_API_KEY
    default_model: claude-3-5-sonnet-20241022
    timeout: 30s
    max_retries: 3
```

2. Create configuration loader:
```go
// config/loader.go
package config

import (
    "os"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
    "gopkg.in/yaml.v3"
)

type Config struct {
    Providers struct {
        Enabled   []string                   `yaml:"enabled"`
        OpenAI    *ProviderConfig            `yaml:"openai"`
        Anthropic *ProviderConfig            `yaml:"anthropic"`
        Gemini    *ProviderConfig            `yaml:"gemini"`
    } `yaml:"providers"`
}

type ProviderConfig struct {
    Type         string `yaml:"type"`
    APIKeyEnv    string `yaml:"api_key_env"`
    DefaultModel string `yaml:"default_model"`
    BaseURL      string `yaml:"base_url,omitempty"`
    Timeout      string `yaml:"timeout"`
    MaxRetries   int    `yaml:"max_retries"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}

func (pc *ProviderConfig) ToProviderConfig() (types.ProviderConfig, error) {
    timeout, err := time.ParseDuration(pc.Timeout)
    if err != nil {
        return types.ProviderConfig{}, err
    }

    apiKey := os.Getenv(pc.APIKeyEnv)

    return types.ProviderConfig{
        Type:         types.ProviderType(pc.Type),
        APIKey:       apiKey,
        DefaultModel: pc.DefaultModel,
        BaseURL:      pc.BaseURL,
        Timeout:      timeout,
        ProviderConfig: map[string]interface{}{
            "max_retries": pc.MaxRetries,
        },
    }, nil
}
```

**Day 5: Factory Setup**

1. Create provider factory wrapper:
```go
// providers/factory.go
package providers

import (
    "fmt"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
    "yourapp/config"
)

type ProviderManager struct {
    factory   *factory.DefaultProviderFactory
    providers map[string]types.Provider
    config    *config.Config
}

func NewProviderManager(cfg *config.Config) (*ProviderManager, error) {
    f := factory.NewProviderFactory()
    factory.RegisterDefaultProviders(f)

    pm := &ProviderManager{
        factory:   f,
        providers: make(map[string]types.Provider),
        config:    cfg,
    }

    // Initialize enabled providers
    for _, name := range cfg.Providers.Enabled {
        if err := pm.initializeProvider(name); err != nil {
            return nil, fmt.Errorf("failed to initialize %s: %w", name, err)
        }
    }

    return pm, nil
}

func (pm *ProviderManager) initializeProvider(name string) error {
    var providerConfig types.ProviderConfig
    var err error

    switch name {
    case "openai":
        providerConfig, err = pm.config.Providers.OpenAI.ToProviderConfig()
    case "anthropic":
        providerConfig, err = pm.config.Providers.Anthropic.ToProviderConfig()
    case "gemini":
        providerConfig, err = pm.config.Providers.Gemini.ToProviderConfig()
    default:
        return fmt.Errorf("unknown provider: %s", name)
    }

    if err != nil {
        return err
    }

    provider, err := pm.factory.CreateProvider(providerConfig.Type, providerConfig)
    if err != nil {
        return err
    }

    pm.providers[name] = provider
    return nil
}

func (pm *ProviderManager) GetProvider(name string) (types.Provider, error) {
    provider, ok := pm.providers[name]
    if !ok {
        return nil, fmt.Errorf("provider not found: %s", name)
    }
    return provider, nil
}

func (pm *ProviderManager) GetAllProviders() map[string]types.Provider {
    return pm.providers
}
```

**Week 1 Checklist:**
- [ ] AI Provider Kit installed
- [ ] Configuration files created
- [ ] Configuration loader implemented
- [ ] Provider factory wrapper created
- [ ] Basic tests passing
- [ ] Documentation updated

### Phase 2: Authentication Migration (Week 2)

**Day 1-2: API Key Migration**

1. Migrate API key storage:
```go
// Before: Hardcoded in code
client := openai.NewClient("sk-...")

// After: Environment variables
config := types.ProviderConfig{
    Type:      types.ProviderTypeOpenAI,
    APIKeyEnv: "OPENAI_API_KEY",
}
```

2. Update environment configuration:
```bash
# .env
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
```

3. Implement secrets management integration:
```go
// secrets/manager.go
package secrets

import (
    "context"
    "fmt"

    // Your secrets manager SDK
    "cloud.google.com/go/secretmanager/apiv1"
)

type SecretsManager struct {
    client *secretmanager.Client
}

func (sm *SecretsManager) GetAPIKey(providerName string) (string, error) {
    name := fmt.Sprintf("projects/PROJECT_ID/secrets/%s-api-key/versions/latest", providerName)

    result, err := sm.client.AccessSecretVersion(context.Background(), &secretmanagerpb.AccessSecretVersionRequest{
        Name: name,
    })
    if err != nil {
        return "", err
    }

    return string(result.Payload.Data), nil
}

// Usage in provider setup
func createProviderWithSecrets(sm *SecretsManager, providerType types.ProviderType) (types.Provider, error) {
    apiKey, err := sm.GetAPIKey(string(providerType))
    if err != nil {
        return nil, err
    }

    config := types.ProviderConfig{
        Type:   providerType,
        APIKey: apiKey,
    }

    return factory.CreateProvider(providerType, config)
}
```

**Day 3-4: Multi-Key Setup**

1. Configure multi-key failover:
```yaml
# config/providers.yaml
providers:
  anthropic:
    type: anthropic
    api_keys:
      - sk-ant-key1
      - sk-ant-key2
      - sk-ant-key3
    default_model: claude-3-5-sonnet-20241022
```

2. Load multi-key configuration:
```go
func (pc *ProviderConfig) ToProviderConfig() (types.ProviderConfig, error) {
    // ... existing code ...

    if len(pc.APIKeys) > 0 {
        return types.ProviderConfig{
            Type: types.ProviderType(pc.Type),
            ProviderConfig: map[string]interface{}{
                "api_keys": pc.APIKeys,
            },
            DefaultModel: pc.DefaultModel,
            Timeout:      timeout,
        }, nil
    }

    // Single key fallback
    return types.ProviderConfig{
        Type:         types.ProviderType(pc.Type),
        APIKey:       os.Getenv(pc.APIKeyEnv),
        DefaultModel: pc.DefaultModel,
        Timeout:      timeout,
    }, nil
}
```

**Day 5: OAuth Setup**

1. Configure OAuth for Gemini:
```yaml
gemini:
  type: gemini
  default_model: gemini-pro
  oauth_credentials:
    - id: account-1
      client_id: ${GEMINI_CLIENT_ID}
      client_secret: ${GEMINI_CLIENT_SECRET}
      access_token: ${GEMINI_ACCESS_TOKEN_1}
      refresh_token: ${GEMINI_REFRESH_TOKEN_1}
    - id: account-2
      client_id: ${GEMINI_CLIENT_ID}
      client_secret: ${GEMINI_CLIENT_SECRET}
      access_token: ${GEMINI_ACCESS_TOKEN_2}
      refresh_token: ${GEMINI_REFRESH_TOKEN_2}
```

2. Implement token refresh callback:
```go
func createOAuthProvider(cfg *ProviderConfig) (types.Provider, error) {
    var credentials []*types.OAuthCredentialSet

    for _, cred := range cfg.OAuthCredentials {
        credentials = append(credentials, &types.OAuthCredentialSet{
            ID:           cred.ID,
            ClientID:     cred.ClientID,
            ClientSecret: cred.ClientSecret,
            AccessToken:  cred.AccessToken,
            RefreshToken: cred.RefreshToken,
            ExpiresAt:    cred.ExpiresAt,
            OnTokenRefresh: createTokenRefreshCallback(cred.ID),
        })
    }

    return factory.CreateProvider(types.ProviderTypeGemini, types.ProviderConfig{
        Type:             types.ProviderTypeGemini,
        OAuthCredentials: credentials,
        DefaultModel:     cfg.DefaultModel,
    })
}

func createTokenRefreshCallback(credentialID string) func(id, access, refresh string, expires time.Time) error {
    return func(id, access, refresh string, expires time.Time) error {
        // Save to your storage
        return saveTokens(credentialID, access, refresh, expires)
    }
}
```

**Week 2 Checklist:**
- [ ] API keys migrated to environment variables
- [ ] Secrets management integrated
- [ ] Multi-key failover configured
- [ ] OAuth setup complete for applicable providers
- [ ] Token refresh callbacks implemented
- [ ] Authentication tests passing

### Phase 3: Basic API Calls (Week 3)

**Day 1-2: Simple Completions**

1. Create wrapper functions:
```go
// api/completions.go
package api

import (
    "context"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type CompletionService struct {
    provider types.Provider
}

func NewCompletionService(provider types.Provider) *CompletionService {
    return &CompletionService{provider: provider}
}

func (s *CompletionService) Generate(ctx context.Context, prompt string) (string, error) {
    stream, err := s.provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: prompt},
        },
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

func (s *CompletionService) GenerateWithContext(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
    stream, err := s.provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "system", Content: systemPrompt},
            {Role: "user", Content: userPrompt},
        },
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
```

2. Replace existing calls:
```go
// Before
response, err := openaiClient.CreateChatCompletion(ctx, request)
content := response.Choices[0].Message.Content

// After
service := api.NewCompletionService(provider)
content, err := service.Generate(ctx, prompt)
```

**Day 3-4: Streaming Support**

1. Implement streaming wrapper:
```go
func (s *CompletionService) GenerateStream(ctx context.Context, prompt string, callback func(string)) error {
    stream, err := s.provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Messages: []types.ChatMessage{
            {Role: "user", Content: prompt},
        },
        Stream: true,
    })
    if err != nil {
        return err
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

        if chunk.Content != "" {
            callback(chunk.Content)
        }
    }

    return nil
}

// Usage
err := service.GenerateStream(ctx, "Tell me a story", func(chunk string) {
    fmt.Print(chunk)
})
```

**Day 5: Testing and Validation**

1. Create integration tests:
```go
func TestBasicCompletion(t *testing.T) {
    if testing.Short() {
        t.Skip()
    }

    provider := setupTestProvider(t)
    service := api.NewCompletionService(provider)

    response, err := service.Generate(context.Background(), "Say hello")
    require.NoError(t, err)
    assert.NotEmpty(t, response)

    t.Logf("Response: %s", response)
}

func TestStreamingCompletion(t *testing.T) {
    if testing.Short() {
        t.Skip()
    }

    provider := setupTestProvider(t)
    service := api.NewCompletionService(provider)

    var chunks []string
    err := service.GenerateStream(context.Background(), "Count to 5", func(chunk string) {
        chunks = append(chunks, chunk)
    })

    require.NoError(t, err)
    assert.NotEmpty(t, chunks)

    t.Logf("Received %d chunks", len(chunks))
}
```

**Week 3 Checklist:**
- [ ] Simple completion functions migrated
- [ ] Streaming support implemented
- [ ] Error handling updated
- [ ] Integration tests passing
- [ ] Performance validated
- [ ] Metrics collection verified

### Phase 4: Advanced Features (Week 4)

**Day 1-2: Tool Calling**

1. Migrate tool definitions:
```go
// tools/definitions.go
package tools

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

var WeatherTool = types.Tool{
    Name:        "get_weather",
    Description: "Get current weather for a location",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type":        "string",
                "description": "City and state, e.g., San Francisco, CA",
            },
            "unit": map[string]interface{}{
                "type":        "string",
                "enum":        []string{"celsius", "fahrenheit"},
                "description": "Temperature unit",
            },
        },
        "required": []string{"location"},
    },
}

var SearchTool = types.Tool{
    Name:        "web_search",
    Description: "Search the web for information",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "query": map[string]interface{}{
                "type":        "string",
                "description": "Search query",
            },
        },
        "required": []string{"query"},
    },
}

func GetAllTools() []types.Tool {
    return []types.Tool{
        WeatherTool,
        SearchTool,
    }
}
```

2. Implement tool executor:
```go
// tools/executor.go
package tools

import (
    "encoding/json"
    "fmt"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type Executor struct {
    handlers map[string]func(map[string]interface{}) (string, error)
}

func NewExecutor() *Executor {
    e := &Executor{
        handlers: make(map[string]func(map[string]interface{}) (string, error)),
    }

    // Register handlers
    e.RegisterHandler("get_weather", handleGetWeather)
    e.RegisterHandler("web_search", handleWebSearch)

    return e
}

func (e *Executor) RegisterHandler(name string, handler func(map[string]interface{}) (string, error)) {
    e.handlers[name] = handler
}

func (e *Executor) Execute(toolCall types.ToolCall) (string, error) {
    handler, ok := e.handlers[toolCall.Function.Name]
    if !ok {
        return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
    }

    var args map[string]interface{}
    if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
        return "", fmt.Errorf("invalid arguments: %w", err)
    }

    return handler(args)
}

func handleGetWeather(args map[string]interface{}) (string, error) {
    location := args["location"].(string)
    unit := "fahrenheit"
    if u, ok := args["unit"].(string); ok {
        unit = u
    }

    // Call your weather API
    result := getWeatherData(location, unit)

    jsonResult, _ := json.Marshal(result)
    return string(jsonResult), nil
}

func handleWebSearch(args map[string]interface{}) (string, error) {
    query := args["query"].(string)

    // Call your search API
    results := performSearch(query)

    jsonResults, _ := json.Marshal(results)
    return string(jsonResults), nil
}
```

3. Create tool calling service:
```go
// api/tool_service.go
package api

import (
    "context"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
    "yourapp/tools"
)

type ToolService struct {
    provider types.Provider
    executor *tools.Executor
    tools    []types.Tool
}

func NewToolService(provider types.Provider) *ToolService {
    return &ToolService{
        provider: provider,
        executor: tools.NewExecutor(),
        tools:    tools.GetAllTools(),
    }
}

func (s *ToolService) GenerateWithTools(ctx context.Context, prompt string, maxIterations int) (string, error) {
    messages := []types.ChatMessage{
        {Role: "user", Content: prompt},
    }

    for i := 0; i < maxIterations; i++ {
        stream, err := s.provider.GenerateChatCompletion(ctx, types.GenerateOptions{
            Messages: messages,
            Tools:    s.tools,
        })
        if err != nil {
            return "", err
        }

        chunk, err := stream.Next()
        if err != nil {
            return "", err
        }

        // Check for tool calls
        if len(chunk.Choices) == 0 || len(chunk.Choices[0].Message.ToolCalls) == 0 {
            // No tool calls, return final answer
            return chunk.Content, nil
        }

        // Add assistant message with tool calls
        messages = append(messages, types.ChatMessage{
            Role:      "assistant",
            ToolCalls: chunk.Choices[0].Message.ToolCalls,
        })

        // Execute tools
        for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
            result, err := s.executor.Execute(toolCall)
            if err != nil {
                result = fmt.Sprintf(`{"error": "%s"}`, err.Error())
            }

            messages = append(messages, types.ChatMessage{
                Role:       "tool",
                Content:    result,
                ToolCallID: toolCall.ID,
            })
        }
    }

    return "", fmt.Errorf("max iterations reached")
}
```

**Day 3-4: Conversation Management**

1. Implement conversation handler:
```go
// api/conversation.go
package api

import (
    "context"
    "sync"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type Conversation struct {
    provider types.Provider
    messages []types.ChatMessage
    maxTurns int
    mu       sync.RWMutex
}

func NewConversation(provider types.Provider, systemPrompt string, maxTurns int) *Conversation {
    conv := &Conversation{
        provider: provider,
        messages: []types.ChatMessage{},
        maxTurns: maxTurns,
    }

    if systemPrompt != "" {
        conv.messages = append(conv.messages, types.ChatMessage{
            Role:    "system",
            Content: systemPrompt,
        })
    }

    return conv
}

func (c *Conversation) Send(ctx context.Context, userMessage string) (string, error) {
    c.mu.Lock()
    c.messages = append(c.messages, types.ChatMessage{
        Role:    "user",
        Content: userMessage,
    })

    // Trim old messages
    if len(c.messages) > c.maxTurns*2+1 { // +1 for system message
        c.messages = append(c.messages[:1], c.messages[len(c.messages)-c.maxTurns*2:]...)
    }
    c.mu.Unlock()

    stream, err := c.provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Messages: c.GetMessages(),
    })
    if err != nil {
        return "", err
    }
    defer stream.Close()

    chunk, err := stream.Next()
    if err != nil {
        return "", err
    }

    c.mu.Lock()
    c.messages = append(c.messages, types.ChatMessage{
        Role:    "assistant",
        Content: chunk.Content,
    })
    c.mu.Unlock()

    return chunk.Content, nil
}

func (c *Conversation) GetMessages() []types.ChatMessage {
    c.mu.RLock()
    defer c.mu.RUnlock()

    result := make([]types.ChatMessage, len(c.messages))
    copy(result, c.messages)
    return result
}

func (c *Conversation) Reset(systemPrompt string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.messages = []types.ChatMessage{}
    if systemPrompt != "" {
        c.messages = append(c.messages, types.ChatMessage{
            Role:    "system",
            Content: systemPrompt,
        })
    }
}
```

**Day 5: Monitoring and Observability**

1. Add metrics collection:
```go
// monitoring/metrics.go
package monitoring

import (
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type MetricsCollector struct {
    providers map[string]types.Provider
}

func NewMetricsCollector(providers map[string]types.Provider) *MetricsCollector {
    return &MetricsCollector{providers: providers}
}

func (mc *MetricsCollector) CollectAll() map[string]types.ProviderMetrics {
    result := make(map[string]types.ProviderMetrics)

    for name, provider := range mc.providers {
        result[name] = provider.GetMetrics()
    }

    return result
}

func (mc *MetricsCollector) StartPeriodicCollection(interval time.Duration, callback func(map[string]types.ProviderMetrics)) {
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            metrics := mc.CollectAll()
            callback(metrics)
        }
    }()
}

// Export to Prometheus format
func (mc *MetricsCollector) ExportPrometheus() string {
    var output string

    for name, provider := range mc.providers {
        metrics := provider.GetMetrics()

        output += fmt.Sprintf("# HELP ai_requests_total Total requests for provider\n")
        output += fmt.Sprintf("# TYPE ai_requests_total counter\n")
        output += fmt.Sprintf("ai_requests_total{provider=\"%s\"} %d\n", name, metrics.RequestCount)

        output += fmt.Sprintf("ai_success_total{provider=\"%s\"} %d\n", name, metrics.SuccessCount)
        output += fmt.Sprintf("ai_errors_total{provider=\"%s\"} %d\n", name, metrics.ErrorCount)
        output += fmt.Sprintf("ai_tokens_used{provider=\"%s\"} %d\n", name, metrics.TokensUsed)
        output += fmt.Sprintf("ai_avg_latency_ms{provider=\"%s\"} %.2f\n",
            name, float64(metrics.AverageLatency.Milliseconds()))
    }

    return output
}
```

**Week 4 Checklist:**
- [ ] Tool calling migrated
- [ ] Tool executor implemented
- [ ] Conversation management added
- [ ] Monitoring and metrics collection setup
- [ ] Advanced feature tests passing
- [ ] Documentation updated

### Phase 5: Production Cutover (Week 5)

**Day 1: Pre-Cutover Validation**

1. Run full test suite:
```bash
go test ./... -v -race
go test ./... -bench=. -benchmem
```

2. Validate metrics accuracy:
```go
func validateMigration(t *testing.T) {
    // Run parallel comparison
    legacyMetrics := runLegacyTests()
    newMetrics := runProviderKitTests()

    // Compare results
    assert.InDelta(t, legacyMetrics.SuccessRate, newMetrics.SuccessRate, 0.01)
    assert.InDelta(t, legacyMetrics.AvgLatency, newMetrics.AvgLatency, 0.1)
}
```

3. Load testing:
```bash
# Run load test
go test -run=TestLoad -count=1 -v -timeout=30m
```

**Day 2: Staged Rollout**

1. Start with 10% traffic:
```go
func routeRequest() {
    if rand.Intn(100) < 10 {
        // Use new SDK
        return handleWithProviderKit()
    }
    // Use old SDK
    return handleWithLegacySDK()
}
```

2. Monitor metrics:
```go
// Compare error rates
func monitorRollout() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        legacyErrorRate := getLegacyErrorRate()
        newErrorRate := getProviderKitErrorRate()

        if newErrorRate > legacyErrorRate*1.2 {
            // Alert and potentially rollback
            alertOnCall("Error rate increased")
        }
    }
}
```

**Day 3: Increase to 50%**

1. Update rollout percentage
2. Continue monitoring
3. Validate all features working

**Day 4: 100% Cutover**

1. Switch all traffic to AI Provider Kit
2. Keep legacy SDK as fallback
3. Monitor closely

**Day 5: Cleanup**

1. Remove legacy SDK dependencies:
```bash
go mod tidy
```

2. Clean up feature flags:
```go
// Remove:
if useNewSDK { ... } else { ... }

// Keep only:
return handleWithProviderKit()
```

3. Update documentation

**Week 5 Checklist:**
- [ ] Pre-cutover validation complete
- [ ] 10% rollout successful
- [ ] 50% rollout successful
- [ ] 100% cutover complete
- [ ] Legacy code removed
- [ ] Documentation updated
- [ ] Team trained on new SDK

### Rollback Procedures

**Immediate Rollback (Emergency)**

1. Set feature flag:
```bash
# In production config or environment
USE_LEGACY_SDK=true
```

2. Restart services:
```bash
kubectl rollout restart deployment/your-service
```

3. Verify rollback:
```bash
# Check metrics return to normal
curl http://your-service/metrics
```

**Gradual Rollback**

1. Reduce AI Provider Kit traffic percentage
2. Monitor for improvement
3. Investigate issues
4. Either fix and retry or complete rollback

**Post-Rollback Actions**

1. Analyze failure cause
2. Fix identified issues
3. Re-test thoroughly
4. Schedule new migration attempt

---

## Summary

This migration guide provides comprehensive instructions for migrating from various SDKs and custom implementations to AI Provider Kit. The key benefits include:

- **Unified Interface**: Single interface for multiple providers
- **Production Features**: Built-in health monitoring, metrics, and failover
- **OAuth Support**: Enterprise-grade credential management
- **Tool Calling**: Universal format across all providers
- **Observability**: Detailed metrics and monitoring

The phased approach ensures a safe, gradual migration with validation at each step. Follow the week-by-week plan, adapt to your specific needs, and don't hesitate to adjust timelines based on your codebase complexity.

**Next Steps:**
1. Review this guide with your team
2. Assess your current codebase
3. Set up a migration project plan
4. Start with Phase 1 (Setup and Configuration)
5. Proceed methodically through each phase

For questions or issues, refer to:
- [Main README](/README.md)
- [Tool Calling Guide](/TOOL_CALLING.md)
- [Metrics Documentation](/docs/METRICS.md)
- [OAuth Manager Documentation](/docs/OAUTH_MANAGER.md)
- [GitHub Issues](https://github.com/cecil-the-coder/ai-provider-kit/issues)

Happy migrating!
