// Package main demonstrates streaming chat completions using the ai-provider-kit.
// It loads configuration from a YAML file, creates a provider using the factory,
// and streams the response tokens in real-time.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	providerName := flag.String("provider", "anthropic", "Provider to use (openai, anthropic, gemini, etc.)")
	prompt := flag.String("prompt", "Tell me a short joke", "Prompt to send to the model")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Get provider configuration entry
	providerEntry := config.GetProviderEntry(cfg, *providerName)
	if providerEntry == nil {
		_, _ = fmt.Fprintf(os.Stderr, "Provider '%s' not found in config\n", *providerName)
		_, _ = fmt.Fprintf(os.Stderr, "Available providers: %v\n", cfg.Providers.Enabled)
		os.Exit(1)
	}

	// Build provider configuration
	providerConfig := config.BuildProviderConfig(*providerName, providerEntry)
	providerConfig.Timeout = 60 * time.Second

	// Handle multiple API keys if present
	if len(providerEntry.APIKeys) > 0 {
		if providerConfig.ProviderConfig == nil {
			providerConfig.ProviderConfig = make(map[string]interface{})
		}
		providerConfig.ProviderConfig["api_keys"] = providerEntry.APIKeys
	}

	// Handle OAuth credentials
	if len(providerEntry.OAuthCredentials) > 0 {
		credSets := config.ConvertOAuthCredentials(providerEntry.OAuthCredentials)
		providerConfig.OAuthCredentials = credSets

		// For Gemini, add project ID
		if providerConfig.Type == types.ProviderTypeGemini && providerEntry.ProjectID != "" {
			if providerConfig.ProviderConfig == nil {
				providerConfig.ProviderConfig = make(map[string]interface{})
			}
			providerConfig.ProviderConfig["project_id"] = providerEntry.ProjectID
		}
	}

	// Initialize factory and register providers
	providerFactory := factory.NewProviderFactory()
	factory.RegisterDefaultProviders(providerFactory)

	// Create provider instance
	provider, err := providerFactory.CreateProvider(providerConfig.Type, providerConfig)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error creating provider: %v\n", err)
		os.Exit(1)
	}

	// Configure the provider
	if err := provider.Configure(providerConfig); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error configuring provider: %v\n", err)
		os.Exit(1)
	}

	// Authenticate with API key if present
	if providerConfig.APIKey != "" {
		authConfig := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: providerConfig.APIKey,
		}
		if err := provider.Authenticate(context.Background(), authConfig); err != nil {
			// Don't fail on auth errors, OAuth providers may not need explicit auth
			_, _ = fmt.Fprintf(os.Stderr, "Warning: Authentication failed: %v\n", err)
		}
	}

	fmt.Printf("Provider: %s\n", provider.Name())
	fmt.Printf("Model: %s\n", provider.GetDefaultModel())
	fmt.Printf("Prompt: %s\n", *prompt)
	fmt.Println()
	fmt.Println("Response:")
	fmt.Println("─────────────────────────────────────────")

	// Create generate options for streaming
	options := types.GenerateOptions{
		Model: provider.GetDefaultModel(),
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: *prompt,
			},
		},
		MaxTokens:   500,
		Temperature: 0.7,
		Stream:      true,
	}

	// Start streaming
	ctx := context.Background()
	startTime := time.Now()

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "\nError starting stream: %v\n", err)
		os.Exit(1)
	}

	// Track statistics
	var totalContent string
	var chunkCount int
	var usage types.Usage

	// Read stream tokens
	for {
		chunk, err := stream.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			// Check for EOF-like errors (some providers use custom error messages)
			if err.Error() == "EOF" || err.Error() == "no more chunks" {
				break
			}
			_, _ = fmt.Fprintf(os.Stderr, "\nError reading stream: %v\n", err)
			break
		}

		// Print content immediately without newline for streaming effect
		if chunk.Content != "" {
			fmt.Print(chunk.Content)
			totalContent += chunk.Content
			chunkCount++
		}

		// Capture usage statistics
		if chunk.Usage.TotalTokens > 0 {
			usage = chunk.Usage
		}

		// Check if stream is done
		if chunk.Done {
			break
		}
	}

	// Close the stream
	if closer, ok := stream.(interface{ Close() error }); ok {
		closer.Close()
	}

	duration := time.Since(startTime)

	// Print final statistics
	fmt.Println()
	fmt.Println("─────────────────────────────────────────")
	fmt.Println()
	fmt.Println("Statistics:")
	fmt.Printf("  Duration: %v\n", duration)
	fmt.Printf("  Chunks received: %d\n", chunkCount)
	fmt.Printf("  Total characters: %d\n", len(totalContent))

	if usage.TotalTokens > 0 {
		fmt.Println()
		fmt.Println("Token Usage:")
		fmt.Printf("  Prompt tokens: %d\n", usage.PromptTokens)
		fmt.Printf("  Completion tokens: %d\n", usage.CompletionTokens)
		fmt.Printf("  Total tokens: %d\n", usage.TotalTokens)
	}

	if chunkCount > 0 && duration.Seconds() > 0 {
		fmt.Printf("  Throughput: %.1f chars/sec\n", float64(len(totalContent))/duration.Seconds())
	}
}
