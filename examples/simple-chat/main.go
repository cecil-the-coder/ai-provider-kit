// simple-chat demonstrates basic usage of the ai-provider-kit library.
// It loads configuration from YAML, creates a provider, and sends a chat completion request.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
	// Command line flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	providerName := flag.String("provider", "", "Provider to use (required)")
	modelOverride := flag.String("model", "", "Model to use (overrides default)")
	flag.Parse()

	// Validate required flags
	if *providerName == "" {
		log.Fatal("Error: -provider flag is required")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config from %s: %v", *configPath, err)
	}

	// Get provider configuration
	providerEntry := config.GetProviderEntry(cfg, *providerName)
	if providerEntry == nil {
		log.Fatalf("Provider '%s' not found in config", *providerName)
	}

	// Build provider config
	providerCfg := config.BuildProviderConfig(*providerName, providerEntry)
	providerCfg.Timeout = 30 * time.Second

	// Initialize provider factory and register providers
	providerFactory := factory.NewProviderFactory()
	factory.RegisterDefaultProviders(providerFactory)

	// Create provider instance
	provider, err := providerFactory.CreateProvider(providerCfg.Type, providerCfg)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Configure the provider
	if err := provider.Configure(providerCfg); err != nil {
		log.Fatalf("Failed to configure provider: %v", err)
	}

	// Determine which model to use
	model := provider.GetDefaultModel()
	if *modelOverride != "" {
		model = *modelOverride
	}

	fmt.Printf("Using provider: %s\n", *providerName)
	fmt.Printf("Using model: %s\n", model)

	// Create chat completion request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	options := types.GenerateOptions{
		Model: model,
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "What is the capital of France? Answer in one sentence.",
			},
		},
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}

	// Send request
	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		log.Fatalf("Failed to generate completion: %v", err)
	}

	// Read response
	var response strings.Builder
	for {
		chunk, err := stream.Next()
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "EOF") && err.Error() != "no more chunks" {
				log.Fatalf("Error reading response: %v", err)
			}
			break
		}
		if chunk.Content != "" {
			response.WriteString(chunk.Content)
		}
		if chunk.Done {
			break
		}
	}

	fmt.Printf("\nResponse: %s\n", response.String())
}
