# Vertex AI Integration for Anthropic Provider

This package provides Google Vertex AI integration middleware for accessing Anthropic Claude models through Google Cloud's Vertex AI platform.

## Overview

The Vertex AI integration allows you to use Claude models through Google Cloud Platform's Vertex AI service, which offers:

- Integration with Google Cloud IAM and billing
- Regional deployment options
- Enterprise-grade SLAs and support
- Compliance with Google Cloud security standards

## Features

- **Request/Response Transformation**: Automatically converts between Anthropic API format and Vertex AI format
- **Model Mapping**: Maps Anthropic model IDs to Vertex AI model versions
- **Multiple Authentication Methods**:
  - Bearer token authentication
  - Service account JSON key file
  - Application Default Credentials (ADC)
- **Region Support**: Multiple GCP regions with model availability checking
- **Streaming Support**: Full support for streaming responses
- **Middleware Pattern**: Implements the common middleware interfaces for easy integration

## Quick Start

### Basic Usage with Bearer Token

```go
import (
    "context"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic/vertex"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
)

// Create Vertex AI configuration
config := vertex.NewDefaultConfig("my-gcp-project", "us-east5")
config.WithBearerToken("your-gcp-access-token")

// Create middleware
vertexMW, err := vertex.NewVertexMiddleware(config)
if err != nil {
    log.Fatal(err)
}

// Add to middleware chain
chain := middleware.NewMiddlewareChain()
chain.Add(vertexMW)

// Use with your HTTP client
// Requests to /v1/messages will be automatically transformed for Vertex AI
```

### Service Account Authentication

```go
config := vertex.NewDefaultConfig("my-gcp-project", "us-east5")
config.WithServiceAccountFile("/path/to/service-account.json")

vertexMW, err := vertex.NewVertexMiddleware(config)
if err != nil {
    log.Fatal(err)
}
```

### Application Default Credentials

```go
config := vertex.NewDefaultConfig("my-gcp-project", "us-east5")
config.WithApplicationDefault()

vertexMW, err := vertex.NewVertexMiddleware(config)
if err != nil {
    log.Fatal(err)
}
```

## Configuration

### VertexConfig

The `VertexConfig` struct contains all configuration options:

```go
type VertexConfig struct {
    // Required fields
    ProjectID string  // GCP project ID
    Region    string  // GCP region (e.g., "us-east5", "europe-west1")
    AuthType  AuthType // Authentication method

    // Authentication credentials (based on AuthType)
    BearerToken        string // For AuthTypeBearerToken
    ServiceAccountFile string // For AuthTypeServiceAccount
    ServiceAccountJSON string // For AuthTypeServiceAccount (alternative to file)

    // Optional fields
    ModelVersionMap map[string]string // Custom model version mapping
    Endpoint        string            // Custom endpoint (defaults to regional endpoint)
}
```

## Model Mapping

The package automatically maps Anthropic model IDs to Vertex AI format:

| Anthropic Model ID | Vertex AI Model Version |
|-------------------|-------------------------|
| claude-3-5-sonnet-20241022 | claude-3-5-sonnet-v2@20241022 |
| claude-3-5-sonnet-20240620 | claude-3-5-sonnet@20240620 |
| claude-3-5-haiku-20241022 | claude-3-5-haiku@20241022 |
| claude-3-opus-20240229 | claude-3-opus@20240229 |
| claude-3-sonnet-20240229 | claude-3-sonnet@20240229 |
| claude-3-haiku-20240307 | claude-3-haiku@20240307 |

### Custom Model Mapping

You can override the default mapping:

```go
config := vertex.NewDefaultConfig("my-project", "us-east5")
config.ModelVersionMap = map[string]string{
    "claude-3-5-sonnet-20241022": "my-custom-version@20241022",
}
```

## Supported Regions

The following GCP regions support Claude models on Vertex AI:

- `us-east5`
- `us-central1`
- `europe-west1`
- `asia-southeast1`

The middleware automatically checks model availability in your configured region.

## Request Transformation

The middleware transforms requests as follows:

1. **URL Transformation**: Converts Anthropic API URLs to Vertex AI endpoints
   ```
   https://api.anthropic.com/v1/messages
   → https://{region}-aiplatform.googleapis.com/v1/projects/{project}/locations/{region}/publishers/anthropic/models/{model}:streamRawPredict
   ```

2. **Authentication**: Replaces Anthropic API key headers with GCP OAuth2 bearer tokens

3. **Request Body**: Removes the `model` field (included in URL path for Vertex AI)

4. **Response**: Restores the original model ID in responses for compatibility

## Authentication Methods

### Bearer Token

Use a pre-obtained GCP access token:

```go
config.WithBearerToken("ya29.c.xxxxx")
```

**Note**: Bearer tokens expire after 1 hour. You'll need to refresh them manually.

### Service Account Key File

Use a service account JSON key file:

```go
config.WithServiceAccountFile("/path/to/key.json")
```

The middleware will automatically handle token refresh.

### Service Account JSON

Provide service account credentials as a JSON string:

```go
jsonContent := `{"type":"service_account",...}`
config.WithServiceAccountJSON(jsonContent)
```

### Application Default Credentials (ADC)

Use the default credentials from the environment:

```go
config.WithApplicationDefault()
```

ADC checks credentials in this order:
1. `GOOGLE_APPLICATION_CREDENTIALS` environment variable
2. GCP metadata server (when running on GCP)
3. gcloud CLI credentials

## Error Handling

The middleware returns errors for:

- Invalid configuration
- Authentication failures
- Model not available in region
- Network errors
- Invalid request format

All errors are properly typed and can be inspected:

```go
ctx, req, err := vertexMW.ProcessRequest(ctx, req)
if err != nil {
    // Handle error
    log.Printf("Vertex AI error: %v", err)
}
```

## Advanced Usage

### Validate Authentication

```go
ctx := context.Background()
if err := vertexMW.ValidateAuth(ctx); err != nil {
    log.Fatalf("Authentication validation failed: %v", err)
}
```

### Get Token Information

```go
authProvider := vertexMW.GetAuthProvider()
info := authProvider.GetTokenInfo()
fmt.Printf("Token info: %+v\n", info)
```

### Check Model Availability

```go
vertexModelID := "claude-3-5-sonnet-v2@20241022"
region := "us-east5"

if !vertex.IsModelAvailableInRegion(vertexModelID, region) {
    availableRegions := vertex.GetAvailableRegions(vertexModelID)
    log.Printf("Model not available in %s, available in: %v", region, availableRegions)
}
```

## Testing

The package includes comprehensive tests:

```bash
cd pkg/providers/anthropic/vertex
go test -v ./...
```

## Requirements

- Go 1.21 or later
- `golang.org/x/oauth2` for authentication
- Valid GCP project with Vertex AI API enabled
- Appropriate IAM permissions for the service account or user

## IAM Permissions

Your service account or user needs the following GCP IAM roles:

- `roles/aiplatform.user` - To use Vertex AI
- Or custom role with permissions:
  - `aiplatform.endpoints.predict`
  - `aiplatform.endpoints.streamRawPredict`

## Limitations

- Only supports Claude models available on Vertex AI
- Requires GCP project with Vertex AI API enabled
- Model availability varies by region
- Some Anthropic features may not be available through Vertex AI

## Examples

See the [examples](../../../../examples/) directory for complete working examples.

## Support

For issues specific to Vertex AI integration:
- Check [Vertex AI documentation](https://cloud.google.com/vertex-ai/docs/generative-ai/model-reference/claude)
- Review GCP IAM permissions
- Verify model availability in your region

For general SDK issues, see the main [README](../../../../README.md).
