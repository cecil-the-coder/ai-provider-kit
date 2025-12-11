// Package bedrock provides AWS Bedrock integration middleware for the Anthropic provider.
//
// This package enables routing Anthropic API requests through AWS Bedrock with automatic
// request/response transformation, model ID mapping, and AWS Signature V4 signing.
//
// # Overview
//
// AWS Bedrock provides managed access to Anthropic's Claude models through AWS infrastructure.
// This middleware transparently transforms Anthropic API requests to work with Bedrock:
//
//   - Converts Anthropic model names to Bedrock model IDs
//   - Rewrites URLs from api.anthropic.com to bedrock-runtime.{region}.amazonaws.com
//   - Signs requests with AWS Signature V4 authentication
//   - Transforms request/response bodies as needed
//
// # Basic Usage
//
// The simplest way to use Bedrock middleware is with environment variables:
//
//	export AWS_REGION=us-east-1
//	export AWS_ACCESS_KEY_ID=your-access-key
//	export AWS_SECRET_ACCESS_KEY=your-secret-key
//
//	config, err := bedrock.NewConfigFromEnv()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	middleware, err := bedrock.NewBedrockMiddleware(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Add to middleware chain
//	chain := middleware.NewMiddlewareChain()
//	chain.Add(middleware)
//
// # Manual Configuration
//
// You can also create configuration manually:
//
//	config := bedrock.NewConfig(
//	    "us-west-2",
//	    "AKIAIOSFODNN7EXAMPLE",
//	    "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
//	)
//
//	// Optional: Add session token for temporary credentials
//	config.WithSessionToken("temporary-session-token")
//
//	// Optional: Add custom model mappings
//	config.WithModelMappings(map[string]string{
//	    "my-model": "anthropic.my-custom-model-v1:0",
//	})
//
//	middleware, err := bedrock.NewBedrockMiddleware(config)
//
// # Model Mapping
//
// The package includes default mappings for all Claude models:
//
//   - claude-3-opus-20240229 → anthropic.claude-3-opus-20240229-v1:0
//   - claude-3-sonnet-20240229 → anthropic.claude-3-sonnet-20240229-v1:0
//   - claude-3-haiku-20240307 → anthropic.claude-3-haiku-20240307-v1:0
//   - claude-3-5-sonnet-20241022 → anthropic.claude-3-5-sonnet-20241022-v2:0
//   - And many more...
//
// You can also use model aliases:
//
//   - claude-3-opus → anthropic.claude-3-opus-20240229-v1:0
//   - claude-3-sonnet → anthropic.claude-3-sonnet-20240229-v1:0
//
// Custom mappings can be added via configuration:
//
//	config.WithModelMappings(map[string]string{
//	    "my-alias": "anthropic.specific-model-v1:0",
//	})
//
// # AWS Authentication
//
// The middleware supports multiple AWS authentication methods:
//
// 1. Environment Variables (recommended):
//
//   - AWS_ACCESS_KEY_ID
//
//   - AWS_SECRET_ACCESS_KEY
//
//   - AWS_SESSION_TOKEN (optional, for temporary credentials)
//
//   - AWS_REGION or AWS_DEFAULT_REGION
//
//     2. Explicit Configuration:
//     config := bedrock.NewConfig(region, accessKey, secretKey)
//     config.WithSessionToken(sessionToken) // if using temporary credentials
//
// # Request Transformation
//
// The middleware transforms requests as follows:
//
// 1. URL Rewriting:
//   - From: https://api.anthropic.com/v1/messages
//   - To: https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke
//
// 2. Header Transformation:
//   - Removes: x-api-key, anthropic-version, anthropic-beta
//   - Adds: AWS Signature V4 headers (Authorization, X-Amz-Date, etc.)
//
// 3. Body Transformation:
//   - Removes model from body (moved to URL path)
//   - Removes anthropic_version
//   - Preserves all other fields (messages, max_tokens, tools, etc.)
//
// # Streaming Support
//
// The middleware automatically handles streaming requests:
//
//   - Detects streaming via stream=true parameter or request body
//   - Routes to /invoke-with-response-stream endpoint
//   - Preserves streaming response format
//
// # Debugging
//
// Enable debug logging to see request transformation details:
//
//	config.WithDebug(true)
//
// Or via environment variable:
//
//	export BEDROCK_DEBUG=true
//
// # Security Considerations
//
//   - AWS credentials are sensitive - use IAM roles when possible
//   - Never commit credentials to version control
//   - Use temporary credentials (session tokens) for enhanced security
//   - Rotate access keys regularly
//   - Follow AWS security best practices
//
// # Limitations
//
//   - Some Anthropic features may not be available on Bedrock
//   - Model availability varies by AWS region
//   - Bedrock pricing and rate limits differ from direct Anthropic API
//   - Some newer Claude models may not yet be available on Bedrock
//
// # Integration with Anthropic Provider
//
// This middleware is designed to work with the Anthropic provider from
// pkg/providers/anthropic. Add it to the provider's middleware chain:
//
//	provider := anthropic.NewAnthropicProvider(providerConfig)
//	bedrockMiddleware, _ := bedrock.NewBedrockMiddleware(bedrockConfig)
//
//	// If provider supports middleware chains:
//	provider.AddMiddleware(bedrockMiddleware)
//
// # Error Handling
//
// The middleware returns errors for:
//   - Invalid configuration (missing region, credentials, etc.)
//   - Failed request signing
//   - Invalid request/response formats
//
// All errors are wrapped with context for debugging.
package bedrock
