# Standardized Core API Demo

This example demonstrates all features of the AI Provider Kit's standardized core API. It showcases how to use the unified interface across different AI providers while maintaining provider-specific capabilities.

## Features Demonstrated

### 1. Basic Chat Completion
- Creating providers with standardized configuration
- Building requests using the builder pattern
- Request validation
- Metadata support

### 2. Streaming Chat Completion
- Setting up streaming requests
- Timeout configuration
- Context management

### 3. Tool Calling
- Multiple tool definitions
- Tool choice modes (auto, required, none, specific)
- Complex tool schemas
- Function calling support

### 4. Provider Capabilities and Extensions
- Discovering provider capabilities
- Core API feature enumeration
- Extension system overview

### 5. Error Handling and Validation
- Request validation errors
- Temperature limits
- Token limits
- Tool validation
- Error types and handling

### 6. Advanced Features
- Response format specification (JSON mode)
- Metadata with various data types
- Timeout configuration
- Context support with cancellation
- Stop sequences

### 7. Multiple Provider Comparison
- OpenAI, Anthropic, Gemini, and other providers
- Consistent request/response format
- Provider-specific capabilities

### 8. Request Building Patterns
- Simple requests
- Chain building pattern
- Legacy options conversion
- Complex requests with all features
- Back-and-forth conversion

## Running the Demo

### Prerequisites
- Go 1.21 or higher
- AI Provider Kit module

### Build and Run
```bash
# Build the demo
go build -o standardized-api-demo .

# Run the demo
./standardized-api-demo
```

### Expected Output
The demo will run through 8 sections, each demonstrating different aspects of the standardized API:

```
=== AI Provider Kit - Standardized Core API Demo ===
This demo showcases all features of the standardized core API
Note: This demo uses mock API keys and simulates API calls for demonstration purposes.

============================================================
DEMO 1: Basic Chat Completion
============================================================
...

[Demo continues through all 8 sections]

============================================================
DEMO COMPLETE
============================================================
All standardized core API features have been demonstrated.
Check the source code to see implementation details.
```

## Key Concepts

### Standardized Request Format
The demo shows how to use `StandardRequest` which provides:
- Unified message format
- Common parameters (model, temperature, max_tokens, etc.)
- Tool support
- Metadata and context
- Timeout and cancellation

### Builder Pattern
The `CoreRequestBuilder` provides a fluent interface:
```go
request, err := types.NewCoreRequestBuilder().
    WithMessages([]types.ChatMessage{
        {Role: "user", Content: "Hello"},
    }).
    WithModel("gpt-4o").
    WithMaxTokens(100).
    WithTemperature(0.7).
    WithStreaming(true).
    WithTimeout(30 * time.Second).
    Build()
```

### Provider Extensions
The demo shows how providers can extend the core API while maintaining compatibility:
- Custom capabilities
- Provider-specific features
- Validation and error handling

### Error Handling
Comprehensive error handling with specific error types:
- `ValidationError` - Request validation failures
- `ProviderError` - Provider-specific errors
- `NetworkError` - Network connectivity issues
- `AuthenticationError` - API key/auth problems
- `RateLimitError` - Rate limiting issues
- `TimeoutError` - Request timeout

## Real Usage Examples

### Basic Completion
```go
// In a real scenario, you would call:
response, err := coreProvider.GenerateStandardCompletion(context.Background(), *request)
if err != nil {
    log.Printf("Failed to generate completion: %v", err)
    return
}
fmt.Printf("Response: %s\n", response.Choices[0].Message.Content)
```

### Streaming
```go
stream, err := coreProvider.GenerateStandardStream(context.Background(), *request)
if err != nil {
    log.Printf("Failed to generate stream: %v", err)
    return
}
defer stream.Close()

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

## Notes

- This demo uses mock API keys and does not make actual API calls
- Provider creation failures are expected in the demo environment
- All request building and validation works without actual API calls
- The demo focuses on demonstrating the API surface and patterns

## Next Steps

1. Replace mock API keys with real credentials
2. Test with actual API calls
3. Explore provider-specific extensions
4. Implement custom error handling
5. Add monitoring and metrics
6. Build applications using the standardized API

## Related Documentation

- [AI Provider Kit Core API Documentation](../../docs/)
- [Provider-Specific Guides](../../docs/SDK_PROVIDER_GUIDES.md)
- [Getting Started Guide](../../docs/SDK_GETTING_STARTED.md)
- [Best Practices](../../docs/SDK_BEST_PRACTICES.md)