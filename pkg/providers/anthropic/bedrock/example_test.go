package bedrock_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http/httptest"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic/bedrock"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/middleware"
)

// Example_basic demonstrates basic Bedrock middleware usage
func Example_basic() {
	// In production, create configuration from environment variables
	// config, err := bedrock.NewConfigFromEnv()
	// For this example, we'll create manually
	config := bedrock.NewConfig(
		"us-east-1",
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	)

	// Create middleware
	bedrockMiddleware, err := bedrock.NewBedrockMiddleware(config)
	if err != nil {
		log.Fatal(err)
	}

	// Create middleware chain
	chain := middleware.NewMiddlewareChain()
	chain.Add(bedrockMiddleware)

	fmt.Println("Bedrock middleware configured successfully")
	// Output: Bedrock middleware configured successfully
}

// Example_manualConfig demonstrates manual configuration
func Example_manualConfig() {
	// Create configuration manually
	config := bedrock.NewConfig(
		"us-east-1",
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	)

	// Add optional session token for temporary credentials
	config.WithSessionToken("temporary-session-token")

	// Enable debug mode
	config.WithDebug(true)

	// Create middleware
	_, err := bedrock.NewBedrockMiddleware(config)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Middleware created for region: %s\n", config.Region)
	// Output: Middleware created for region: us-east-1
}

// Example_customModelMappings demonstrates custom model mappings
func Example_customModelMappings() {
	config := bedrock.NewConfig(
		"us-west-2",
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	)

	// Add custom model mappings
	customMappings := map[string]string{
		"my-custom-model": "anthropic.my-custom-model-v1:0",
		"my-test-model":   "anthropic.my-test-model-v2:0",
	}
	config.WithModelMappings(customMappings)

	_, err := bedrock.NewBedrockMiddleware(config)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Custom model mappings configured")
	// Output: Custom model mappings configured
}

// Example_modelMapper demonstrates using the model mapper directly
func Example_modelMapper() {
	// Create model mapper
	mapper := bedrock.NewModelMapper()

	// Convert Anthropic model name to Bedrock model ID
	bedrockID, found := mapper.ToBedrockModelID("claude-3-haiku-20240307")
	if found {
		fmt.Printf("Bedrock ID: %s\n", bedrockID)
	}

	// Check if model mapping is valid
	err := mapper.ValidateModelMapping("claude-3-opus-20240229")
	if err == nil {
		fmt.Println("Model mapping is valid")
	}

	// Get all supported models
	models := mapper.GetSupportedModels()
	fmt.Printf("Total supported models: %d\n", len(models))

	// Output:
	// Bedrock ID: anthropic.claude-3-haiku-20240307-v1:0
	// Model mapping is valid
	// Total supported models: 24
}

// Example_requestTransformation demonstrates request transformation
func Example_requestTransformation() {
	config := bedrock.NewConfig(
		"us-east-1",
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	)

	bedrockMiddleware, err := bedrock.NewBedrockMiddleware(config)
	if err != nil {
		log.Fatal(err)
	}

	// Create a mock Anthropic API request with valid JSON body
	requestBody := `{"model":"claude-3-opus-20240229","max_tokens":100,"messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(
		"POST",
		"https://api.anthropic.com/v1/messages",
		bytes.NewBufferString(requestBody),
	)
	req.Header.Set("x-api-key", "sk-ant-api-key")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	// Process request through middleware
	ctx := context.Background()
	_, transformedReq, err := bedrockMiddleware.ProcessRequest(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Original Host: api.anthropic.com\n")
	fmt.Printf("Transformed Host: %s\n", transformedReq.URL.Host)
	fmt.Printf("Has AWS Signature: %t\n", transformedReq.Header.Get("Authorization") != "")

	// Output:
	// Original Host: api.anthropic.com
	// Transformed Host: bedrock-runtime.us-east-1.amazonaws.com
	// Has AWS Signature: true
}

// Example_multipleRegions demonstrates using multiple regions
func Example_multipleRegions() {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}

	for _, region := range regions {
		config := bedrock.NewConfig(
			region,
			"AKIAIOSFODNN7EXAMPLE",
			"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		)

		endpoint := config.GetEndpoint()
		fmt.Printf("Region %s: %s\n", region, endpoint)
	}

	// Output:
	// Region us-east-1: bedrock-runtime.us-east-1.amazonaws.com
	// Region us-west-2: bedrock-runtime.us-west-2.amazonaws.com
	// Region eu-west-1: bedrock-runtime.eu-west-1.amazonaws.com
}

// Example_validation demonstrates configuration validation
func Example_validation() {
	// Invalid config - missing required fields
	invalidConfig := &bedrock.BedrockConfig{
		Region: "us-east-1",
		// Missing AccessKeyID and SecretAccessKey
	}

	err := invalidConfig.Validate()
	if err != nil {
		fmt.Println("Validation failed:", err)
	}

	// Valid config
	validConfig := &bedrock.BedrockConfig{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	err = validConfig.Validate()
	if err == nil {
		fmt.Println("Validation passed")
	}

	// Output:
	// Validation failed: bedrock: access_key_id is required
	// Validation passed
}

// Example_cloneConfig demonstrates configuration cloning
func Example_cloneConfig() {
	original := bedrock.NewConfig(
		"us-east-1",
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	)
	original.WithDebug(true)

	// Clone the configuration
	clone := original.Clone()

	// Modify clone
	clone.Region = "us-west-2"
	clone.Debug = false

	fmt.Printf("Original region: %s, debug: %t\n", original.Region, original.Debug)
	fmt.Printf("Clone region: %s, debug: %t\n", clone.Region, clone.Debug)

	// Output:
	// Original region: us-east-1, debug: true
	// Clone region: us-west-2, debug: false
}

// Example_supportedModels demonstrates listing supported models
func Example_supportedModels() {
	mapper := bedrock.NewModelMapper()

	models := mapper.GetSupportedModels()
	fmt.Printf("Total supported models: %d\n", len(models))

	// Check if specific models are supported
	testModels := []string{
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}

	for _, model := range testModels {
		bedrockID, found := mapper.ToBedrockModelID(model)
		if found {
			fmt.Printf("✓ %s -> %s\n", model, bedrockID)
		}
	}

	// Output (at least):
	// Total supported models: 23
	// ✓ claude-3-opus-20240229 -> anthropic.claude-3-opus-20240229-v1:0
	// ✓ claude-3-sonnet-20240229 -> anthropic.claude-3-sonnet-20240229-v1:0
	// ✓ claude-3-haiku-20240307 -> anthropic.claude-3-haiku-20240307-v1:0
}
