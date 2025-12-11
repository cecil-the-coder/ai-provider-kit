// Package vertex provides Google Vertex AI integration middleware for Anthropic Claude models.
//
// This package enables accessing Anthropic Claude models through Google Cloud Platform's Vertex AI service,
// which offers integration with Google Cloud IAM and billing, regional deployment options,
// and enterprise-grade SLAs.
//
// # Overview
//
// The Vertex AI integration consists of:
//   - VertexMiddleware: Request/response transformation middleware
//   - VertexConfig: Configuration for GCP project, region, and authentication
//   - AuthProvider: GCP authentication handling (bearer token, service account, ADC)
//   - Model mapping: Automatic conversion between Anthropic and Vertex AI model identifiers
//
// # Quick Start
//
//	config := vertex.NewDefaultConfig("my-gcp-project", "us-east5")
//	config.WithBearerToken("your-gcp-access-token")
//
//	vertexMW, err := vertex.NewVertexMiddleware(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	chain := middleware.NewMiddlewareChain()
//	chain.Add(vertexMW)
//
// # Authentication Methods
//
// The package supports three authentication methods:
//
// Bearer Token:
//
//	config.WithBearerToken("ya29.c.xxxxx")
//
// Service Account Key File:
//
//	config.WithServiceAccountFile("/path/to/key.json")
//
// Application Default Credentials:
//
//	config.WithApplicationDefault()
//
// # Model Mapping
//
// Models are automatically mapped between Anthropic and Vertex AI formats:
//   - claude-3-5-sonnet-20241022 → claude-3-5-sonnet-v2@20241022
//   - claude-3-opus-20240229 → claude-3-opus@20240229
//   - claude-3-haiku-20240307 → claude-3-haiku@20240307
//
// Custom mappings can be configured:
//
//	config.ModelVersionMap = map[string]string{
//	    "my-model": "vertex-model@20241022",
//	}
//
// # Supported Regions
//
// The following GCP regions support Claude models:
//   - us-east5
//   - us-central1
//   - europe-west1
//   - asia-southeast1
//
// # Request Transformation
//
// The middleware transforms requests from Anthropic API format to Vertex AI format:
//
//  1. URL: https://api.anthropic.com/v1/messages
//     → https://{region}-aiplatform.googleapis.com/v1/projects/{project}/locations/{region}/publishers/anthropic/models/{model}:streamRawPredict
//
//  2. Authentication: Anthropic API key headers → GCP OAuth2 bearer tokens
//
//  3. Request body: Removes model field (included in URL for Vertex AI)
//
// # Error Handling
//
// Errors are returned for:
//   - Invalid configuration
//   - Authentication failures
//   - Models not available in the specified region
//   - Network errors
//   - Invalid request format
//
// # Examples
//
// See the package examples and tests for detailed usage patterns.
//
// # Requirements
//
//   - Go 1.21 or later
//   - Valid GCP project with Vertex AI API enabled
//   - IAM role: roles/aiplatform.user or equivalent permissions
//
// # See Also
//
//   - Google Vertex AI documentation: https://cloud.google.com/vertex-ai/docs/generative-ai/model-reference/claude
//   - Anthropic API documentation: https://docs.anthropic.com/
package vertex
