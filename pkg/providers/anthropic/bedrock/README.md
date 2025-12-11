# AWS Bedrock Integration Middleware for Anthropic Provider

This package provides middleware to route Anthropic API requests through AWS Bedrock with automatic request/response transformation and AWS Signature V4 signing.

## Features

- **Transparent Request Transformation**: Converts Anthropic API requests to AWS Bedrock format
- **Model ID Mapping**: Automatically maps Anthropic model names to Bedrock model IDs
- **AWS Signature V4 Signing**: Signs all requests with proper AWS authentication
- **Streaming Support**: Handles both regular and streaming requests
- **Flexible Configuration**: Support for environment variables, explicit config, and custom mappings
- **Comprehensive Testing**: Full test coverage with unit and integration tests

## Installation

```bash
go get github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic/bedrock
```

## Quick Start

### Using Environment Variables

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic/bedrock"

// Set environment variables:
// AWS_REGION=us-east-1
// AWS_ACCESS_KEY_ID=your-access-key
// AWS_SECRET_ACCESS_KEY=your-secret-key

config, err := bedrock.NewConfigFromEnv()
if err != nil {
    log.Fatal(err)
}

middleware, err := bedrock.NewBedrockMiddleware(config)
if err != nil {
    log.Fatal(err)
}

// Add to middleware chain
chain := middleware.NewMiddlewareChain()
chain.Add(middleware)
```

### Manual Configuration

```go
config := bedrock.NewConfig(
    "us-west-2",
    "AKIAIOSFODNN7EXAMPLE",
    "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
)

// Optional: Add session token for temporary credentials
config.WithSessionToken("temporary-session-token")

// Optional: Enable debug logging
config.WithDebug(true)

middleware, err := bedrock.NewBedrockMiddleware(config)
```

## Configuration

### BedrockConfig

```go
type BedrockConfig struct {
    Region          string              // AWS region (required)
    AccessKeyID     string              // AWS access key (required)
    SecretAccessKey string              // AWS secret key (required)
    SessionToken    string              // Optional session token
    Endpoint        string              // Optional custom endpoint
    ModelMappings   map[string]string   // Optional custom model mappings
    Debug           bool                // Enable debug logging
}
```

### Environment Variables

- `AWS_REGION` or `AWS_DEFAULT_REGION`: AWS region
- `AWS_ACCESS_KEY_ID`: AWS access key ID
- `AWS_SECRET_ACCESS_KEY`: AWS secret access key
- `AWS_SESSION_TOKEN`: Optional session token for temporary credentials
- `BEDROCK_DEBUG`: Set to "true" to enable debug logging

## Model Mappings

The package includes default mappings for all Claude models:

| Anthropic Model | Bedrock Model ID |
|----------------|------------------|
| claude-3-opus-20240229 | anthropic.claude-3-opus-20240229-v1:0 |
| claude-3-sonnet-20240229 | anthropic.claude-3-sonnet-20240229-v1:0 |
| claude-3-haiku-20240307 | anthropic.claude-3-haiku-20240307-v1:0 |
| claude-3-5-sonnet-20241022 | anthropic.claude-3-5-sonnet-20241022-v2:0 |
| claude-3-5-haiku-20241022 | anthropic.claude-3-5-haiku-20241022-v1:0 |

### Custom Model Mappings

```go
config.WithModelMappings(map[string]string{
    "my-custom-model": "anthropic.my-custom-model-v1:0",
})
```

### Using Model Mapper Directly

```go
mapper := bedrock.NewModelMapper()

// Convert Anthropic to Bedrock
bedrockID, found := mapper.ToBedrockModelID("claude-3-opus-20240229")
// Returns: "anthropic.claude-3-opus-20240229-v1:0", true

// Reverse lookup
anthropicName, found := mapper.ToAnthropicModelName("anthropic.claude-3-opus-20240229-v1:0")
// Returns: "claude-3-opus-20240229", true
```

## Request Transformation

The middleware transforms requests as follows:

### URL Rewriting

```
From: https://api.anthropic.com/v1/messages
To:   https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke
```

For streaming requests:
```
To:   https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke-with-response-stream
```

### Header Transformation

**Removed Headers:**
- `x-api-key`
- `anthropic-version`
- `anthropic-beta`

**Added Headers:**
- `Authorization`: AWS Signature V4 signature
- `X-Amz-Date`: Request timestamp
- `X-Amz-Content-Sha256`: Payload hash
- `X-Amz-Security-Token`: Session token (if present)

### Body Transformation

- Removes `model` from body (moved to URL path)
- Removes `anthropic_version`
- Adds default `max_tokens: 4096` if not specified
- Preserves all other fields (messages, tools, system, etc.)

## AWS Signature V4 Signing

The package implements complete AWS Signature V4 signing for Bedrock requests:

- Canonical request construction
- String to sign generation
- Signature calculation with HMAC-SHA256
- Authorization header building
- Proper handling of query strings and headers

```go
signer := bedrock.NewSigner(config)
err := signer.SignRequest(req)
```

## Streaming Support

Streaming requests are automatically detected and routed to the appropriate endpoint:

```go
// Request with stream=true will use:
// /model/{modelId}/invoke-with-response-stream
```

## Error Handling

The middleware returns detailed errors with context:

```go
middleware, err := bedrock.NewBedrockMiddleware(config)
if err != nil {
    // Handle configuration errors
}

ctx, req, err := middleware.ProcessRequest(ctx, req)
if err != nil {
    // Handle request transformation errors
}
```

## Testing

Run tests:

```bash
go test ./pkg/providers/anthropic/bedrock/...
```

Run with verbose output:

```bash
go test -v ./pkg/providers/anthropic/bedrock/...
```

Run specific test:

```bash
go test -run TestBedrockMiddleware_ProcessRequest ./pkg/providers/anthropic/bedrock/...
```

## Examples

See `example_test.go` for comprehensive examples:

- Basic usage with environment variables
- Manual configuration
- Custom model mappings
- Model mapper usage
- Request transformation
- Multiple regions
- Configuration validation
- Config cloning

## Security Considerations

- **Never commit AWS credentials to version control**
- Use IAM roles when running on AWS infrastructure
- Use temporary credentials (session tokens) when possible
- Rotate access keys regularly
- Follow AWS security best practices
- Enable debug mode only in development

## Limitations

- Some Anthropic features may not be available on Bedrock
- Model availability varies by AWS region
- Bedrock pricing and rate limits differ from direct Anthropic API
- Newer Claude models may not yet be available on Bedrock

## Contributing

When adding new model mappings:

1. Update `DefaultModelMappings` in `models.go`
2. Add test cases in `models_test.go`
3. Update this README

## License

This package is part of the AI Provider Kit and follows the same license.

## References

- [AWS Bedrock Documentation](https://docs.aws.amazon.com/bedrock/)
- [Anthropic API Documentation](https://docs.anthropic.com/)
- [AWS Signature V4 Signing Process](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html)
