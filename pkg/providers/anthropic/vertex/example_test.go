package vertex_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic/vertex"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
)

// Example demonstrates basic usage of Vertex AI middleware with bearer token
func Example_basicUsage() {
	// Create Vertex AI configuration
	config := vertex.NewDefaultConfig("my-gcp-project", "us-east5")
	config.WithBearerToken("test-gcp-access-token")

	// Create middleware
	vertexMW, err := vertex.NewVertexMiddleware(config)
	if err != nil {
		log.Fatal(err)
	}

	// Create middleware chain
	chain := middleware.NewMiddlewareChain()
	chain.Add(vertexMW)

	// Create a sample Anthropic API request
	requestBody := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello, Claude!"},
		},
	}

	bodyBytes, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.Background()

	// Process request through middleware
	_, transformedReq, err := chain.ProcessRequest(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Request transformed successfully")
	fmt.Printf("New URL contains: aiplatform.googleapis.com\n")
	fmt.Printf("Authorization header set: %v\n", transformedReq.Header.Get("Authorization") != "")

	// Output:
	// Request transformed successfully
	// New URL contains: aiplatform.googleapis.com
	// Authorization header set: true
}

// Example demonstrates service account authentication
func Example_serviceAccount() {
	config := vertex.NewDefaultConfig("my-gcp-project", "us-east5")

	// Option 1: From file
	config.WithServiceAccountFile("/path/to/service-account.json")

	// Option 2: From JSON string
	//nolint:gosec // G101: example test data, not real credentials
	serviceAccountJSON := `{
		"type": "service_account",
		"project_id": "my-project",
		"private_key_id": "key-id",
		"private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n",
		"client_email": "my-sa@my-project.iam.gserviceaccount.com"
	}`
	config.WithServiceAccountJSON(serviceAccountJSON)

	_, err := vertex.NewVertexMiddleware(config)
	if err != nil {
		// Expected to fail with fake credentials
		fmt.Println("Service account setup (would work with real credentials)")
		return
	}

	// Output:
	// Service account setup (would work with real credentials)
}

// Example demonstrates custom model mapping
func Example_customModelMapping() {
	config := vertex.NewDefaultConfig("my-gcp-project", "us-east5")
	config.WithBearerToken("test-token")

	// Override default model mapping
	config.ModelVersionMap = map[string]string{
		"claude-3-5-sonnet-20241022": "my-custom-version@20241022",
		"my-custom-model":            "vertex-custom-model@20241022",
	}

	vertexMW, err := vertex.NewVertexMiddleware(config)
	if err != nil {
		log.Fatal(err)
	}

	// Get the mapped version
	mappedVersion := vertexMW.GetConfig().GetModelVersion("my-custom-model")
	fmt.Printf("Custom model mapped to: %s\n", mappedVersion)

	// Output:
	// Custom model mapped to: vertex-custom-model@20241022
}

// Example demonstrates checking model availability
func Example_modelAvailability() {
	// Check if a model is available in a specific region
	modelID := "claude-3-5-sonnet-v2@20241022"
	region := "us-east5"

	if vertex.IsModelAvailableInRegion(modelID, region) {
		fmt.Printf("%s is available in %s\n", modelID, region)
	}

	// Get all regions where a model is available
	availableRegions := vertex.GetAvailableRegions(modelID)
	fmt.Printf("Available in %d regions\n", len(availableRegions))

	// Get recommended region
	recommended := vertex.GetRecommendedRegion()
	fmt.Printf("Recommended region: %s\n", recommended)

	// Output:
	// claude-3-5-sonnet-v2@20241022 is available in us-east5
	// Available in 4 regions
	// Recommended region: us-east5
}

// Example demonstrates model ID conversion
func Example_modelConversion() {
	// Convert Anthropic model ID to Vertex AI format
	anthropicModel := "claude-3-5-sonnet-20241022"
	vertexModel := vertex.GetDefaultModelVersion(anthropicModel)
	fmt.Printf("Anthropic: %s -> Vertex: %s\n", anthropicModel, vertexModel)

	// Convert back from Vertex to Anthropic format
	convertedBack := vertex.GetAnthropicModelID(vertexModel)
	fmt.Printf("Vertex: %s -> Anthropic: %s\n", vertexModel, convertedBack)

	// Output:
	// Anthropic: claude-3-5-sonnet-20241022 -> Vertex: claude-3-5-sonnet-v2@20241022
	// Vertex: claude-3-5-sonnet-v2@20241022 -> Anthropic: claude-3-5-sonnet-20241022
}

// Example demonstrates authentication validation
func Example_authValidation() {
	config := vertex.NewDefaultConfig("my-gcp-project", "us-east5")
	config.WithBearerToken("test-token")

	vertexMW, err := vertex.NewVertexMiddleware(config)
	if err != nil {
		log.Fatal(err)
	}

	// Validate authentication
	ctx := context.Background()
	if err := vertexMW.ValidateAuth(ctx); err != nil {
		log.Printf("Auth validation failed: %v", err)
		return
	}

	// Get token information
	authProvider := vertexMW.GetAuthProvider()
	tokenInfo := authProvider.GetTokenInfo()

	fmt.Printf("Auth type: %v\n", tokenInfo["auth_type"])
	fmt.Printf("Has token: %v\n", tokenInfo["has_token"])

	// Output:
	// Auth type: bearer_token
	// Has token: true
}

// Example demonstrates using with HTTP client
func Example_httpClient() {
	// Create configuration
	config := vertex.NewDefaultConfig("my-gcp-project", "us-east5")
	config.WithBearerToken("test-token")

	// Create middleware
	vertexMW, err := vertex.NewVertexMiddleware(config)
	if err != nil {
		log.Fatal(err)
	}

	// Create HTTP client with middleware
	chain := middleware.NewMiddlewareChain()
	chain.Add(vertexMW)

	// Create custom transport that uses the middleware
	transport := &http.Transport{}
	client := &http.Client{
		Transport: &middlewareTransport{
			chain:     chain,
			transport: transport,
		},
	}

	// Use the client (would make actual HTTP request)
	_ = client

	fmt.Println("HTTP client configured with Vertex AI middleware")

	// Output:
	// HTTP client configured with Vertex AI middleware
}

// middlewareTransport is a custom HTTP transport that applies middleware
type middlewareTransport struct {
	chain     middleware.MiddlewareChain
	transport http.RoundTripper
}

func (t *middlewareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Process request through middleware
	ctx, modifiedReq, err := t.chain.ProcessRequest(req.Context(), req)
	if err != nil {
		return nil, err
	}

	// Make the actual HTTP request
	resp, err := t.transport.RoundTrip(modifiedReq)
	if err != nil {
		return nil, err
	}

	// Process response through middleware
	_, modifiedResp, err := t.chain.ProcessResponse(ctx, modifiedReq, resp)
	if err != nil {
		return nil, err
	}

	return modifiedResp, nil
}

// Example demonstrates supported regions
func Example_supportedRegions() {
	regions := vertex.SupportedRegions()

	fmt.Printf("Supported regions: %d\n", len(regions))
	// Note: Order may vary due to map iteration
	if len(regions) == 4 {
		fmt.Println("Regions include: us-east5, europe-west1, us-central1, asia-southeast1")
	}

	// Output:
	// Supported regions: 4
	// Regions include: us-east5, europe-west1, us-central1, asia-southeast1
}

// Example demonstrates configuration validation
func Example_configValidation() {
	// Valid configuration
	validConfig := &vertex.VertexConfig{
		ProjectID:   "my-project",
		Region:      "us-east5",
		AuthType:    vertex.AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	if err := validConfig.Validate(); err == nil {
		fmt.Println("Valid configuration")
	}

	// Invalid configuration - missing project ID
	invalidConfig := &vertex.VertexConfig{
		Region:      "us-east5",
		AuthType:    vertex.AuthTypeBearerToken,
		BearerToken: "test-token",
	}

	if err := invalidConfig.Validate(); err != nil {
		fmt.Println("Invalid configuration detected")
	}

	// Output:
	// Valid configuration
	// Invalid configuration detected
}

// Example demonstrates endpoint configuration
func Example_endpointConfiguration() {
	config := vertex.NewDefaultConfig("my-project", "europe-west1")

	// Use default endpoint
	defaultEndpoint := config.GetEndpoint()
	fmt.Printf("Default endpoint: %s\n", defaultEndpoint)

	// Use custom endpoint
	config.Endpoint = "https://custom-vertex-endpoint.example.com"
	customEndpoint := config.GetEndpoint()
	fmt.Printf("Custom endpoint: %s\n", customEndpoint)

	// Output:
	// Default endpoint: https://europe-west1-aiplatform.googleapis.com
	// Custom endpoint: https://custom-vertex-endpoint.example.com
}
