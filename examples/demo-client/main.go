package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"gopkg.in/yaml.v3"
)

// TokenManager handles OAuth token updates and persistence
// Note: Uses shared config package types
type TokenManager struct {
	configPath string
	mu         sync.Mutex
}

// NewTokenManager creates a new token manager
func NewTokenManager(configPath string) *TokenManager {
	return &TokenManager{
		configPath: configPath,
	}
}

// CreateMultiOAuthCallback creates a callback for multi-OAuth token refresh
func (tm *TokenManager) CreateMultiOAuthCallback(providerName, credentialID string) func(id, accessToken, refreshToken string, expiresAt time.Time) error {
	return func(id, accessToken, refreshToken string, expiresAt time.Time) error {
		tm.mu.Lock()
		defer tm.mu.Unlock()

		fmt.Printf("\n🔄 [Multi-OAuth Refresh] Provider '%s' credential '%s' refreshed\n", providerName, credentialID)
		fmt.Printf("   New Access Token: %s...%s\n", accessToken[:20], accessToken[len(accessToken)-10:])
		if refreshToken != "" {
			fmt.Printf("   New Refresh Token: %s...%s\n", refreshToken[:20], refreshToken[len(refreshToken)-10:])
		}
		fmt.Printf("   Expires At: %s\n", expiresAt.Format(time.RFC3339))

		// Load current config using shared package
		data, err := os.ReadFile(tm.configPath)
		if err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}

		var cfg config.DemoConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}

		// Find and update the specific credential in the specific provider
		var provider *config.ProviderConfigEntry
		switch providerName {
		case "anthropic":
			provider = cfg.Providers.Anthropic
		case "cerebras":
			provider = cfg.Providers.Cerebras
		case "gemini":
			provider = cfg.Providers.Gemini
		case "qwen":
			provider = cfg.Providers.Qwen
		default:
			// Check custom providers
			if cfg.Providers.Custom != nil {
				if customConfig, ok := cfg.Providers.Custom[providerName]; ok {
					provider = &customConfig
					defer func() {
						if provider != nil {
							cfg.Providers.Custom[providerName] = *provider
						}
					}()
				}
			}
		}

		if provider != nil && len(provider.OAuthCredentials) > 0 {
			// Find and update the specific credential
			for i := range provider.OAuthCredentials {
				if provider.OAuthCredentials[i].ID == credentialID {
					provider.OAuthCredentials[i].AccessToken = accessToken
					if refreshToken != "" {
						provider.OAuthCredentials[i].RefreshToken = refreshToken
					}
					provider.OAuthCredentials[i].ExpiresAt = expiresAt.Format(time.RFC3339)
					break
				}
			}
		}

		// Save updated config
		updatedData, err := yaml.Marshal(&cfg)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		if err := os.WriteFile(tm.configPath, updatedData, 0644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("✅ Successfully persisted refreshed tokens to config.yaml\n\n")
		return nil
	}
}

// TestProvider tests a single provider
func TestProvider(provider types.Provider, providerName string, prompt string, verbose bool) {
	TestProviderWithOptions(provider, providerName, prompt, verbose, "")
}

// TestProviderWithOptions tests a provider with custom options
func TestProviderWithOptions(provider types.Provider, providerName string, prompt string, verbose bool, modelOverride string) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("Testing Provider: %s\n", providerName)
	fmt.Printf(strings.Repeat("=", 60) + "\n")

	ctx := context.Background()

	// Check authentication status
	if !provider.IsAuthenticated() {
		fmt.Printf("⚠️  Provider is not authenticated\n")
	} else {
		fmt.Printf("✅ Provider is authenticated\n")
	}

	// Get provider info
	fmt.Printf("📊 Provider Type: %s\n", provider.Type())
	fmt.Printf("📊 Description: %s\n", provider.Description())
	fmt.Printf("📊 Default Model: %s\n", provider.GetDefaultModel())

	// Check capabilities
	fmt.Printf("\n🔧 Capabilities:\n")
	fmt.Printf("   Streaming: %v\n", provider.SupportsStreaming())
	fmt.Printf("   Tool Calling: %v\n", provider.SupportsToolCalling())
	fmt.Printf("   Responses API: %v\n", provider.SupportsResponsesAPI())
	if provider.SupportsToolCalling() {
		fmt.Printf("   Tool Format: %s\n", provider.GetToolFormat())
	}

	// Health check
	fmt.Printf("\n🏥 Running health check...\n")
	if err := provider.HealthCheck(ctx); err != nil {
		fmt.Printf("❌ Health check failed: %v\n", err)
	} else {
		fmt.Printf("✅ Health check passed\n")
	}

	// Get available models (optional, might fail for some providers)
	fmt.Printf("\n📦 Fetching available models...\n")
	models, err := provider.GetModels(ctx)
	if err != nil {
		fmt.Printf("⚠️  Could not fetch models: %v\n", err)
	} else {
		fmt.Printf("Found %d models\n", len(models))
		if verbose && len(models) > 0 {
			for i, model := range models {
				if i >= 5 {
					fmt.Printf("   ... and %d more\n", len(models)-5)
					break
				}
				fmt.Printf("   - %s\n", model.ID)
			}
		}
	}

	// Test chat completion
	fmt.Printf("\n💬 Testing chat completion...\n")
	fmt.Printf("Prompt: %s\n", prompt)

	// Build options
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   2000, // High limit for reasoning models (GLM-4.6, etc.) that think before answering
		Temperature: 0.7,
		Stream:      false,
	}

	// Apply model override if specified
	if modelOverride != "" {
		options.Model = modelOverride
		fmt.Printf("🎯 Model Override: %s (instead of default: %s)\n", modelOverride, provider.GetDefaultModel())
	} else {
		fmt.Printf("📋 Using Default Model: %s\n", provider.GetDefaultModel())
	}

	startTime := time.Now()
	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		fmt.Printf("❌ Failed to generate completion: %v\n", err)
		return
	}

	// Read the response
	response := ""
	iterations := 0
	maxIterations := 100 // Prevent infinite loop
	for iterations < maxIterations {
		iterations++
		chunk, err := stream.Next()
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "EOF") && err.Error() != "no more chunks" {
				fmt.Printf("❌ Error reading stream: %v\n", err)
			}
			break
		}
		if chunk.Content != "" {
			response += chunk.Content
		}
		if chunk.Done {
			break
		}
	}
	if iterations >= maxIterations {
		fmt.Printf("⚠️ Warning: Stopped reading after %d iterations\n", maxIterations)
	}

	duration := time.Since(startTime)
	fmt.Printf("✅ Response received in %v\n", duration)
	fmt.Printf("\nResponse: %s\n", response)

	// Get metrics
	metrics := provider.GetMetrics()
	fmt.Printf("\n📈 Provider Metrics:\n")
	fmt.Printf("   Total Requests: %d\n", metrics.RequestCount)
	fmt.Printf("   Successful: %d\n", metrics.SuccessCount)
	fmt.Printf("   Failed: %d\n", metrics.ErrorCount)
	if metrics.TokensUsed > 0 {
		fmt.Printf("   Tokens Used: %d\n", metrics.TokensUsed)
	}
	if metrics.AverageLatency > 0 {
		fmt.Printf("   Average Latency: %v\n", metrics.AverageLatency)
	}
}

// TestProviderWithStreaming tests streaming capability
func TestProviderWithStreaming(provider types.Provider, providerName string, prompt string, modelOverride string) {
	fmt.Printf("\n🌊 Testing streaming for %s...\n", providerName)

	ctx := context.Background()
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   50,
		Temperature: 0.7,
		Stream:      true,
		Model:       modelOverride, // Use model override if provided
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		fmt.Printf("❌ Failed to start streaming: %v\n", err)
		return
	}

	fmt.Printf("Streaming response: ")
	iterations := 0
	maxIterations := 100
	for iterations < maxIterations {
		iterations++
		chunk, err := stream.Next()
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "EOF") && err.Error() != "no more chunks" {
				fmt.Printf("\n❌ Error in stream: %v\n", err)
			}
			break
		}
		if chunk.Content != "" {
			fmt.Print(chunk.Content)
		}
		if chunk.Done {
			break
		}
	}
	fmt.Println("\n✅ Streaming completed")
}

func main() {
	// Command line flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	verbose := flag.Bool("verbose", false, "Verbose output")
	testStreaming := flag.Bool("stream", false, "Test streaming capabilities")
	specificProvider := flag.String("provider", "", "Test specific provider only (supports provider:model format)")
	customPrompt := flag.String("prompt", "Explain the concept of recursion in one sentence.", "Custom prompt to test")
	interactive := flag.Bool("interactive", false, "Enable interactive mode after tests")
	flag.Parse()

	// Load configuration using shared config package
	fmt.Println("🚀 AI Provider Kit Demo Client")
	fmt.Printf("📁 Loading configuration from: %s\n", *configPath)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config from '%s': %v\nHint: Create a config.yaml file or use -config flag to specify the path", *configPath, err)
	}

	// Create token manager for OAuth callbacks
	tokenManager := NewTokenManager(*configPath)

	// Initialize provider factory
	providerFactory := factory.NewProviderFactory()
	factory.RegisterDefaultProviders(providerFactory)

	// Track successful providers
	successfulProviders := []string{}
	failedProviders := []string{}

	// Parse provider string (supports "provider:model" format)
	var providersToTest []string
	var modelOverride string

	if *specificProvider != "" {
		// Parse provider:model format
		parts := strings.SplitN(*specificProvider, ":", 2)
		providersToTest = []string{parts[0]}
		if len(parts) == 2 {
			modelOverride = parts[1]
			fmt.Printf("\n🎯 Model override detected: %s\n", modelOverride)
		}
	} else {
		providersToTest = cfg.Providers.Enabled
	}

	fmt.Printf("📋 Providers to test: %v\n", providersToTest)

	// Test each enabled provider
	for _, providerName := range providersToTest {
		// Get provider config using shared package
		providerEntry := config.GetProviderEntry(cfg, providerName)
		if providerEntry == nil {
			fmt.Printf("\n⚠️  Provider '%s' not found in config, skipping\n", providerName)
			continue
		}

		// Build provider config using shared package
		providerCfg := config.BuildProviderConfig(providerName, providerEntry)

		// Set additional fields not handled by shared package
		providerCfg.Timeout = 30 * time.Second
		providerCfg.Name = providerName

		// Handle OAuth configuration (add token refresh callbacks)
		if len(providerEntry.OAuthCredentials) > 0 {
			// The shared package already converted OAuthCredentials,
			// but we need to add token refresh callbacks
			for i, credEntry := range providerEntry.OAuthCredentials {
				if i < len(providerCfg.OAuthCredentials) {
					providerCfg.OAuthCredentials[i].OnTokenRefresh = tokenManager.CreateMultiOAuthCallback(providerName, credEntry.ID)
				}
			}

			// For Gemini, add project ID
			if providerCfg.Type == types.ProviderTypeGemini && providerEntry.ProjectID != "" {
				if providerCfg.ProviderConfig == nil {
					providerCfg.ProviderConfig = make(map[string]interface{})
				}
				providerCfg.ProviderConfig["project_id"] = providerEntry.ProjectID
			}
		}

		// Create provider instance
		fmt.Printf("\n🔧 Creating provider: %s\n", providerName)
		provider, err := providerFactory.CreateProvider(providerCfg.Type, providerCfg)
		if err != nil {
			fmt.Printf("❌ Failed to create provider '%s': %v\n", providerName, err)
			failedProviders = append(failedProviders, providerName)
			continue
		}

		// Configure the provider
		if err := provider.Configure(providerCfg); err != nil {
			fmt.Printf("❌ Failed to configure provider '%s': %v\n", providerName, err)
			failedProviders = append(failedProviders, providerName)
			continue
		}

		// Authenticate if needed (API key only, OAuth is handled via OAuthCredentials)
		if providerCfg.APIKey != "" {
			authCfg := types.AuthConfig{
				Method: types.AuthMethodAPIKey,
				APIKey: providerCfg.APIKey,
			}
			if err := provider.Authenticate(context.Background(), authCfg); err != nil {
				fmt.Printf("⚠️  Authentication warning for '%s': %v\n", providerName, err)
			}
		}

		// Test the provider (with optional model override from provider:model format)
		TestProviderWithOptions(provider, providerName, *customPrompt, *verbose, modelOverride)

		// Test streaming if requested
		if *testStreaming && provider.SupportsStreaming() {
			TestProviderWithStreaming(provider, providerName, "Count from 1 to 5", modelOverride)
		}

		successfulProviders = append(successfulProviders, providerName)
	}

	// Summary
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Println("📊 Test Summary")
	fmt.Printf(strings.Repeat("=", 60) + "\n")
	fmt.Printf("✅ Successful providers (%d): %v\n", len(successfulProviders), successfulProviders)
	if len(failedProviders) > 0 {
		fmt.Printf("❌ Failed providers (%d): %v\n", len(failedProviders), failedProviders)
	}

	// Demonstrate OAuth token refresh information
	if *verbose {
		fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
		fmt.Println("🔄 OAuth Token Refresh Information")
		fmt.Printf(strings.Repeat("=", 60) + "\n")
		fmt.Println("When OAuth tokens are refreshed during API calls, the multi-OAuth callback will:")
		fmt.Println("1. Receive the new access and refresh tokens for the specific credential")
		fmt.Println("2. Update the in-memory configuration")
		fmt.Println("3. Persist the new tokens to config.yaml")
		fmt.Println("4. Continue the API call with the new token")
		fmt.Println("\nAll providers now use multi-OAuth configuration format.")
	}

	fmt.Println("\n✨ Demo completed!")

	// Interactive mode (only if flag is set)
	if *interactive && len(successfulProviders) > 0 {
		fmt.Println("\n🎮 Starting interactive mode...")
		runInteractiveMode(successfulProviders, providerFactory, cfg, tokenManager)
	}
}

// Helper function to check if slice contains string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Interactive mode for testing providers
func runInteractiveMode(providers []string, factory *factory.DefaultProviderFactory, cfg *config.DemoConfig, tm *TokenManager) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n🎮 Interactive Mode")
	fmt.Println("Commands:")
	fmt.Println("  list - Show available providers")
	fmt.Println("  use <provider> - Select a provider")
	fmt.Println("  prompt <text> - Send a prompt to current provider")
	fmt.Println("  stream <text> - Test streaming with current provider")
	fmt.Println("  health - Check health of current provider")
	fmt.Println("  metrics - Show metrics for current provider")
	fmt.Println("  quit - Exit interactive mode")

	// Provider instance tracking is handled via currentProviderName and recreation on demand
	var currentProviderName string

	for {
		fmt.Print("\n> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		parts := strings.SplitN(input, " ", 2)
		if len(parts) == 0 {
			continue
		}

		command := parts[0]

		switch command {
		case "quit", "exit":
			fmt.Println("👋 Goodbye!")
			return

		case "list":
			fmt.Println("Available providers:")
			for _, p := range providers {
				if p == currentProviderName {
					fmt.Printf("  * %s (current)\n", p)
				} else {
					fmt.Printf("  - %s\n", p)
				}
			}

		case "use":
			if len(parts) < 2 {
				fmt.Println("Usage: use <provider>")
				continue
			}
			providerName := parts[1]
			if !contains(providers, providerName) {
				fmt.Printf("Provider '%s' not available\n", providerName)
				continue
			}

			// Create provider (simplified, reusing logic from main)
			fmt.Printf("Switching to provider: %s\n", providerName)
			currentProviderName = providerName
			// Note: In a real implementation, you'd recreate the provider here
			fmt.Printf("✅ Now using: %s\n", currentProviderName)

		case "prompt":
			if len(parts) < 2 {
				fmt.Println("Usage: prompt <text>")
				continue
			}
			if currentProviderName == "" {
				fmt.Println("No provider selected. Use 'use <provider>' first")
				continue
			}
			// Send prompt to current provider
			fmt.Printf("Sending prompt to %s...\n", currentProviderName)
			// Implementation would go here

		case "stream":
			if len(parts) < 2 {
				fmt.Println("Usage: stream <text>")
				continue
			}
			if currentProviderName == "" {
				fmt.Println("No provider selected. Use 'use <provider>' first")
				continue
			}
			fmt.Printf("Testing streaming with %s...\n", currentProviderName)
			// Implementation would go here

		case "health":
			if currentProviderName == "" {
				fmt.Println("No provider selected. Use 'use <provider>' first")
				continue
			}
			fmt.Printf("Checking health of %s...\n", currentProviderName)
			// Implementation would go here

		case "metrics":
			if currentProviderName == "" {
				fmt.Println("No provider selected. Use 'use <provider>' first")
				continue
			}
			fmt.Printf("Metrics for %s:\n", currentProviderName)
			// Implementation would go here

		default:
			fmt.Printf("Unknown command: %s\n", command)
		}
	}
}
