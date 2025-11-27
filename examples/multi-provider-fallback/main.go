// Package main demonstrates multi-provider fallback capabilities in ai-provider-kit.
// It shows how to configure multiple AI providers and automatically failover
// to secondary providers when the primary provider fails.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ProviderResult represents the result of a provider attempt
type ProviderResult struct {
	ProviderName string
	Success      bool
	Response     string
	Error        error
	Duration     time.Duration
	Metrics      types.ProviderMetrics
}

// FallbackManager manages provider failover logic
type FallbackManager struct {
	factory   *factory.DefaultProviderFactory
	config    *config.DemoConfig
	providers []string
	timeout   time.Duration
}

// NewFallbackManager creates a new fallback manager
func NewFallbackManager(f *factory.DefaultProviderFactory, cfg *config.DemoConfig, providers []string, timeout time.Duration) *FallbackManager {
	return &FallbackManager{
		factory:   f,
		config:    cfg,
		providers: providers,
		timeout:   timeout,
	}
}

// Execute tries providers in order until one succeeds
func (fm *FallbackManager) Execute(prompt string) (*ProviderResult, []ProviderResult) {
	var allResults []ProviderResult

	for i, providerName := range fm.providers {
		fmt.Printf("\n[%d/%d] Attempting provider: %s\n", i+1, len(fm.providers), providerName)

		result := fm.tryProvider(providerName, prompt)
		allResults = append(allResults, result)

		if result.Success {
			fmt.Printf("[SUCCESS] Provider %s responded successfully in %v\n", providerName, result.Duration)
			return &result, allResults
		}

		fmt.Printf("[FAILED] Provider %s failed: %v\n", providerName, result.Error)
		if i < len(fm.providers)-1 {
			fmt.Printf("[FALLBACK] Attempting next provider...\n")
		}
	}

	fmt.Printf("\n[ERROR] All providers failed\n")
	return nil, allResults
}

// tryProvider attempts to use a single provider
func (fm *FallbackManager) tryProvider(providerName string, prompt string) ProviderResult {
	result := ProviderResult{
		ProviderName: providerName,
		Success:      false,
	}

	// Get provider config
	providerEntry := config.GetProviderEntry(fm.config, providerName)
	if providerEntry == nil {
		result.Error = fmt.Errorf("provider '%s' not found in config", providerName)
		return result
	}

	// Build provider config
	providerCfg := config.BuildProviderConfig(providerName, providerEntry)
	providerCfg.Timeout = fm.timeout
	providerCfg.Name = providerName

	// Create provider instance
	provider, err := fm.factory.CreateProvider(providerCfg.Type, providerCfg)
	if err != nil {
		result.Error = fmt.Errorf("failed to create provider: %w", err)
		return result
	}

	// Configure the provider
	if err := provider.Configure(providerCfg); err != nil {
		result.Error = fmt.Errorf("failed to configure provider: %w", err)
		return result
	}

	// Authenticate if API key is available
	if providerCfg.APIKey != "" {
		authCfg := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: providerCfg.APIKey,
		}
		if err := provider.Authenticate(context.Background(), authCfg); err != nil {
			// Log warning but continue - some providers don't need explicit auth
			fmt.Printf("   [WARN] Authentication warning: %v\n", err)
		}
	}

	// Run health check first
	ctx, cancel := context.WithTimeout(context.Background(), fm.timeout)
	defer cancel()

	fmt.Printf("   Running health check for %s...\n", providerName)
	if err := provider.HealthCheck(ctx); err != nil {
		result.Error = fmt.Errorf("health check failed: %w", err)
		result.Metrics = provider.GetMetrics()
		return result
	}
	fmt.Printf("   Health check passed\n")

	// Attempt chat completion
	fmt.Printf("   Sending prompt to %s...\n", providerName)

	startTime := time.Now()
	options := types.GenerateOptions{
		Model: provider.GetDefaultModel(),
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   500,
		Temperature: 0.7,
		Stream:      false,
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		result.Error = fmt.Errorf("generation failed: %w", err)
		result.Duration = time.Since(startTime)
		result.Metrics = provider.GetMetrics()
		return result
	}

	// Read the response
	var responseBuilder strings.Builder
	iterations := 0
	maxIterations := 100

	for iterations < maxIterations {
		iterations++
		chunk, err := stream.Next()
		if err != nil {
			errStr := err.Error()
			if errStr != "EOF" && !strings.Contains(errStr, "EOF") && errStr != "no more chunks" {
				result.Error = fmt.Errorf("error reading stream: %w", err)
				result.Duration = time.Since(startTime)
				result.Metrics = provider.GetMetrics()
				return result
			}
			break
		}
		if chunk.Content != "" {
			responseBuilder.WriteString(chunk.Content)
		}
		if chunk.Done {
			break
		}
	}

	result.Success = true
	result.Response = responseBuilder.String()
	result.Duration = time.Since(startTime)
	result.Metrics = provider.GetMetrics()

	return result
}

func main() {
	// Command line flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	providersFlag := flag.String("providers", "", "Comma-separated list of providers in priority order (e.g., 'anthropic,openai,cerebras')")
	promptFlag := flag.String("prompt", "What is the capital of France? Answer in one sentence.", "Prompt to send to providers")
	timeoutFlag := flag.Duration("timeout", 30*time.Second, "Timeout per provider attempt")
	flag.Parse()

	fmt.Println("==========================================================")
	fmt.Println("  Multi-Provider Fallback Demo")
	fmt.Println("  AI Provider Kit")
	fmt.Println("==========================================================")

	// Load configuration
	fmt.Printf("\nLoading configuration from: %s\n", *configPath)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize provider factory
	providerFactory := factory.NewProviderFactory()
	factory.RegisterDefaultProviders(providerFactory)

	// Determine providers to use
	var providers []string
	if *providersFlag != "" {
		// Use command-line specified providers
		providers = strings.Split(*providersFlag, ",")
		for i := range providers {
			providers[i] = strings.TrimSpace(providers[i])
		}
	} else if len(cfg.Providers.PreferredOrder) > 0 {
		// Use preferred order from config
		providers = cfg.Providers.PreferredOrder
	} else if len(cfg.Providers.Enabled) > 0 {
		// Fall back to enabled providers
		providers = cfg.Providers.Enabled
	} else {
		log.Fatal("No providers specified. Use -providers flag or configure 'enabled' or 'preferred_order' in config.yaml")
	}

	fmt.Printf("Provider priority order: %v\n", providers)
	fmt.Printf("Timeout per provider: %v\n", *timeoutFlag)
	fmt.Printf("Prompt: %s\n", *promptFlag)

	// Create fallback manager and execute
	fm := NewFallbackManager(providerFactory, cfg, providers, *timeoutFlag)
	successResult, allResults := fm.Execute(*promptFlag)

	// Print results
	fmt.Println("\n==========================================================")
	fmt.Println("  Results")
	fmt.Println("==========================================================")

	if successResult != nil {
		fmt.Printf("\nSuccessful Provider: %s\n", successResult.ProviderName)
		fmt.Printf("Response Time: %v\n", successResult.Duration)
		fmt.Printf("\nResponse:\n%s\n", successResult.Response)
	} else {
		fmt.Println("\nNo provider succeeded.")
		fmt.Println("\nAll attempts failed:")
		for _, result := range allResults {
			fmt.Printf("  - %s: %v\n", result.ProviderName, result.Error)
		}
		os.Exit(1)
	}

	// Print metrics summary
	fmt.Println("\n==========================================================")
	fmt.Println("  Provider Metrics Summary")
	fmt.Println("==========================================================")

	for _, result := range allResults {
		fmt.Printf("\n%s:\n", result.ProviderName)
		fmt.Printf("  Status: %s\n", map[bool]string{true: "SUCCESS", false: "FAILED"}[result.Success])
		fmt.Printf("  Duration: %v\n", result.Duration)
		fmt.Printf("  Requests: %d\n", result.Metrics.RequestCount)
		fmt.Printf("  Successes: %d\n", result.Metrics.SuccessCount)
		fmt.Printf("  Errors: %d\n", result.Metrics.ErrorCount)
		if result.Metrics.TokensUsed > 0 {
			fmt.Printf("  Tokens Used: %d\n", result.Metrics.TokensUsed)
		}
		if result.Error != nil {
			fmt.Printf("  Error: %v\n", result.Error)
		}
	}

	// Print aggregate statistics
	fmt.Println("\n==========================================================")
	fmt.Println("  Aggregate Statistics")
	fmt.Println("==========================================================")

	totalAttempts := len(allResults)
	successCount := 0
	var totalDuration time.Duration
	for _, result := range allResults {
		totalDuration += result.Duration
		if result.Success {
			successCount++
		}
	}

	fmt.Printf("Total Attempts: %d\n", totalAttempts)
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", totalAttempts-successCount)
	fmt.Printf("Total Time: %v\n", totalDuration)
	if successResult != nil {
		fmt.Printf("Effective Provider: %s (attempt #%d)\n",
			successResult.ProviderName,
			findProviderIndex(allResults, successResult.ProviderName)+1)
	}

	fmt.Println("\nDemo completed!")
}

// findProviderIndex returns the index of a provider in the results
func findProviderIndex(results []ProviderResult, name string) int {
	for i, r := range results {
		if r.ProviderName == name {
			return i
		}
	}
	return -1
}
