package models_test

import (
	"fmt"
	"os"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/models"
)

// ExampleGetDefaultsRegistry demonstrates basic usage of the model registry.
func ExampleGetDefaultsRegistry() {
	registry := models.GetDefaultsRegistry()

	// Get metadata for a specific model
	metadata := registry.GetModelDefaults("gpt-4o")
	if metadata != nil {
		fmt.Printf("Model: %s\n", metadata.DisplayName)
		fmt.Printf("MaxTokens: %d\n", metadata.MaxTokens)
		fmt.Printf("SupportsTools: %v\n", metadata.Capabilities.SupportsTools)
		fmt.Printf("SupportsStreaming: %v\n", metadata.Capabilities.SupportsStreaming)
		fmt.Printf("SupportsVision: %v\n", metadata.Capabilities.SupportsVision)
	}

	// Get all available providers
	providers := registry.GetAllProviders()
	fmt.Printf("Providers: %v\n", providers)

	// Get all models for a specific provider
	providerModels := registry.GetProviderModels("openai")
	for modelID, meta := range providerModels {
		fmt.Printf("  %s: %s\n", modelID, meta.DisplayName)
	}
}

// ExampleGetDefaultsRegistry_multipleModels demonstrates looking up multiple models.
func ExampleGetDefaultsRegistry_multipleModels() {
	registry := models.GetDefaultsRegistry()

	// Look up various models
	models := []string{
		"gpt-4o",
		"claude-sonnet-4",
		"gemini-2.5-pro",
	}

	for _, modelID := range models {
		metadata := registry.GetModelDefaults(modelID)
		if metadata != nil {
			fmt.Printf("%s: %s (max tokens: %d)\n",
				modelID, metadata.DisplayName, metadata.MaxTokens)
		}
	}
}

// ExampleModelMetadata shows how to work with model metadata.
func ExampleModelMetadata() {
	registry := models.GetDefaultsRegistry()
	metadata := registry.GetModelDefaults("gpt-4o")

	if metadata == nil {
		fmt.Println("Model not found")
		os.Exit(1)
	}

	// Access display information
	_ = metadata.DisplayName
	_ = metadata.Description

	// Access capabilities
	capabilities := metadata.Capabilities
	_ = capabilities.SupportsTools
	_ = capabilities.SupportsStreaming
	_ = capabilities.SupportsVision

	// Access token limits
	_ = metadata.MaxTokens

	// Access pricing information (if available)
	cost := metadata.CostPerMToken
	_ = cost.InputCostPerMToken
	_ = cost.OutputCostPerMToken

	fmt.Printf("Model metadata: %s\n", metadata.DisplayName)
}
