package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ANSI color codes for better output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[37m"
	ColorBold   = "\033[1m"
)

// Global variables
var (
	providerFactory *factory.DefaultProviderFactory
	configFile      = flag.String("config", "config.yaml", "Path to config file (default: config.yaml in current directory)")
	globalConfig    *config.DemoConfig
)

func main() {
	flag.Parse()

	fmt.Println(ColorBold + ColorCyan + "🤖 AI Provider Kit - Dynamic Model Discovery Demo" + ColorReset)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Load configuration
	var providers []string

	// Check if config file exists
	if _, err := os.Stat(*configFile); err == nil {
		// Load from config file using shared config package
		fmt.Printf(ColorCyan+"Loading configuration from %s..."+ColorReset+"\n", *configFile)
		cfg, err := config.LoadConfig(*configFile)
		if err != nil {
			fmt.Printf(ColorRed+"❌ Failed to load config: %v"+ColorReset+"\n", err)
			os.Exit(1)
		}
		globalConfig = cfg
		providers = cfg.Providers.Enabled
		fmt.Printf(ColorGreen+"✓ Loaded config with %d enabled providers"+ColorReset+"\n\n", len(providers))
	} else {
		// Fall back to environment variables
		fmt.Println(ColorYellow + "Config file not found at " + *configFile + ", using environment variables" + ColorReset)
		providers = checkEnvironmentVariables()
	}

	if len(providers) == 0 {
		fmt.Println(ColorRed + "❌ No providers configured. Please either:" + ColorReset)
		fmt.Println("   1. Use -config flag with a config file, or")
		fmt.Println("   2. Set environment variables:")
		fmt.Println("      - OPENAI_API_KEY")
		fmt.Println("      - ANTHROPIC_API_KEY")
		fmt.Println("      - GEMINI_API_KEY")
		fmt.Println("      - CEREBRAS_API_KEY")
		fmt.Println("      - OPENROUTER_API_KEY")
		fmt.Println("      - QWEN_API_KEY")
		os.Exit(1)
	}

	fmt.Println(ColorGreen + "✓ Found configuration for providers:" + ColorReset)
	for _, p := range providers {
		fmt.Printf("  - %s\n", p)
	}
	fmt.Println()

	// Run demos
	ctx := context.Background()

	// 1. Individual provider demos
	fmt.Println(ColorBold + "1️⃣  Individual Provider Model Discovery" + ColorReset)
	fmt.Println(strings.Repeat("-", 80))
	discoverModelsForProviders(ctx, providers)
	fmt.Println()

	// 2. Cache behavior demo
	fmt.Println(ColorBold + "2️⃣  Cache Behavior Demonstration" + ColorReset)
	fmt.Println(strings.Repeat("-", 80))
	demonstrateCaching(ctx, providers)
	fmt.Println()

	// 3. Provider comparison
	fmt.Println(ColorBold + "3️⃣  Cross-Provider Model Comparison" + ColorReset)
	fmt.Println(strings.Repeat("-", 80))
	compareProviders(ctx, providers)
	fmt.Println()

	// 4. Export to JSON
	fmt.Println(ColorBold + "4️⃣  Export Models to JSON" + ColorReset)
	fmt.Println(strings.Repeat("-", 80))
	exportToJSON(ctx, providers)
	fmt.Println()

	fmt.Println(ColorGreen + ColorBold + "✅ Demo completed successfully!" + ColorReset)
}

// checkEnvironmentVariables checks which provider API keys are available
func checkEnvironmentVariables() []string {
	var available []string

	envVars := map[string]string{
		"OPENAI_API_KEY":     "OpenAI",
		"ANTHROPIC_API_KEY":  "Anthropic",
		"GEMINI_API_KEY":     "Gemini",
		"CEREBRAS_API_KEY":   "Cerebras",
		"OPENROUTER_API_KEY": "OpenRouter",
		"QWEN_API_KEY":       "Qwen",
	}

	for env, name := range envVars {
		if os.Getenv(env) != "" {
			available = append(available, name)
		}
	}

	sort.Strings(available)
	return available
}

// discoverModelsForProviders demonstrates model discovery for each provider
// This function only requires the ModelProvider interface, not the full Provider interface
func discoverModelsForProviders(ctx context.Context, providers []string) {
	for _, providerName := range providers {
		fmt.Printf("\n%s%s Provider:%s\n", ColorBold, providerName, ColorReset)

		provider, err := createModelProvider(providerName)
		if err != nil {
			fmt.Printf("%s  ❌ Failed to create provider: %v%s\n", ColorRed, err, ColorReset)
			continue
		}

		startTime := time.Now()
		models, err := provider.GetModels(ctx)
		elapsed := time.Since(startTime)

		if err != nil {
			fmt.Printf("%s  ❌ Failed to get models: %v%s\n", ColorRed, err, ColorReset)
			continue
		}

		fmt.Printf("%s  ✓ Fetched %d models in %v%s\n", ColorGreen, len(models), elapsed, ColorReset)

		// Display model details
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  ID\tName\tMax Tokens\tStreaming\tTools")
		fmt.Fprintln(w, "  "+strings.Repeat("-", 70))

		// Show first 10 models
		displayCount := len(models)
		if displayCount > 10 {
			displayCount = 10
		}

		for i := 0; i < displayCount; i++ {
			model := models[i]
			streaming := "✓"
			if !model.SupportsStreaming {
				streaming = "✗"
			}
			tools := "✓"
			if !model.SupportsToolCalling {
				tools = "✗"
			}

			maxTokens := fmt.Sprintf("%d", model.MaxTokens)
			if model.MaxTokens == 0 {
				maxTokens = "-"
			}

			// Truncate long IDs
			modelID := model.ID
			if len(modelID) > 35 {
				modelID = modelID[:32] + "..."
			}

			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
				modelID,
				truncate(model.Name, 25),
				maxTokens,
				streaming,
				tools,
			)
		}

		if len(models) > 10 {
			fmt.Fprintf(w, "  %s... and %d more models%s\n",
				ColorGray, len(models)-10, ColorReset)
		}

		w.Flush()
	}
}

// demonstrateCaching shows how the caching mechanism works
// This function only requires the ModelProvider interface, not the full Provider interface
func demonstrateCaching(ctx context.Context, providers []string) {
	if len(providers) == 0 {
		return
	}

	// Use first available provider
	providerName := providers[0]
	fmt.Printf("Testing with %s%s%s provider...\n\n", ColorCyan, providerName, ColorReset)

	provider, err := createModelProvider(providerName)
	if err != nil {
		fmt.Printf("%s❌ Failed to create provider: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	// First call - should hit API
	fmt.Println(ColorYellow + "First call (should fetch from API):" + ColorReset)
	startTime := time.Now()
	models1, err := provider.GetModels(ctx)
	elapsed1 := time.Since(startTime)

	if err != nil {
		fmt.Printf("%s❌ Error: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("  ⏱️  Time: %v\n", elapsed1)
	fmt.Printf("  📊 Models: %d\n", len(models1))
	fmt.Println()

	// Second call - should use cache
	fmt.Println(ColorYellow + "Second call (should use cache):" + ColorReset)
	startTime = time.Now()
	models2, err := provider.GetModels(ctx)
	elapsed2 := time.Since(startTime)

	if err != nil {
		fmt.Printf("%s❌ Error: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("  ⏱️  Time: %v\n", elapsed2)
	fmt.Printf("  📊 Models: %d\n", len(models2))
	fmt.Println()

	// Compare times
	speedup := float64(elapsed1) / float64(elapsed2)
	fmt.Printf("%s💡 Cache speedup: %.0fx faster%s\n", ColorGreen, speedup, ColorReset)

	if elapsed2 < 10*time.Millisecond {
		fmt.Printf("%s✓ Cache is working correctly (< 10ms)%s\n", ColorGreen, ColorReset)
	} else {
		fmt.Printf("%s⚠️  Cache might not be working (took %v)%s\n", ColorYellow, elapsed2, ColorReset)
	}
}

// compareProviders compares models across different providers
// This function only requires the ModelProvider interface, not the full Provider interface
func compareProviders(ctx context.Context, providers []string) {
	type ProviderStats struct {
		Name           string
		ModelCount     int
		AvgMaxTokens   int
		StreamingPct   int
		ToolCallingPct int
	}

	var stats []ProviderStats

	for _, providerName := range providers {
		provider, err := createModelProvider(providerName)
		if err != nil {
			continue
		}

		models, err := provider.GetModels(ctx)
		if err != nil {
			continue
		}

		if len(models) == 0 {
			continue
		}

		// Calculate statistics
		totalTokens := 0
		streamingCount := 0
		toolsCount := 0

		for _, model := range models {
			if model.MaxTokens > 0 {
				totalTokens += model.MaxTokens
			}
			if model.SupportsStreaming {
				streamingCount++
			}
			if model.SupportsToolCalling {
				toolsCount++
			}
		}

		avgTokens := 0
		if len(models) > 0 {
			avgTokens = totalTokens / len(models)
		}

		streamingPct := (streamingCount * 100) / len(models)
		toolsPct := (toolsCount * 100) / len(models)

		stats = append(stats, ProviderStats{
			Name:           providerName,
			ModelCount:     len(models),
			AvgMaxTokens:   avgTokens,
			StreamingPct:   streamingPct,
			ToolCallingPct: toolsPct,
		})
	}

	// Display comparison table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Provider\tModels\tAvg Max Tokens\tStreaming\tTool Calling")
	fmt.Fprintln(w, strings.Repeat("-", 70))

	for _, stat := range stats {
		fmt.Fprintf(w, "%s\t%d\t%s\t%d%%\t%d%%\n",
			stat.Name,
			stat.ModelCount,
			formatTokens(stat.AvgMaxTokens),
			stat.StreamingPct,
			stat.ToolCallingPct,
		)
	}

	w.Flush()
	fmt.Println()

	// Find provider with most models
	if len(stats) > 0 {
		maxModels := stats[0]
		for _, stat := range stats {
			if stat.ModelCount > maxModels.ModelCount {
				maxModels = stat
			}
		}
		fmt.Printf("%s🏆 %s has the most models (%d)%s\n",
			ColorGreen, maxModels.Name, maxModels.ModelCount, ColorReset)
	}
}

// exportToJSON exports all models to JSON files
// This function only requires the ModelProvider interface, not the full Provider interface
func exportToJSON(ctx context.Context, providers []string) {
	outputDir := "model-discovery-output"

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("%s❌ Failed to create output directory: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	exportedCount := 0

	for _, providerName := range providers {
		provider, err := createModelProvider(providerName)
		if err != nil {
			continue
		}

		models, err := provider.GetModels(ctx)
		if err != nil {
			continue
		}

		// Export to JSON
		filename := fmt.Sprintf("%s/%s-models.json", outputDir, strings.ToLower(providerName))
		data, err := json.MarshalIndent(models, "", "  ")
		if err != nil {
			fmt.Printf("%s❌ Failed to marshal %s models: %v%s\n", ColorRed, providerName, err, ColorReset)
			continue
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Printf("%s❌ Failed to write %s: %v%s\n", ColorRed, filename, err, ColorReset)
			continue
		}

		fmt.Printf("%s✓ Exported %d models to %s%s\n",
			ColorGreen, len(models), filename, ColorReset)
		exportedCount++
	}

	if exportedCount > 0 {
		fmt.Printf("\n%s💾 All model data saved to ./%s/%s\n", ColorCyan, outputDir, ColorReset)
	}
}

// Note: loadConfig is now provided by the shared config package

func init() {
	// Create and initialize factory
	providerFactory = factory.NewProviderFactory()
	factory.RegisterDefaultProviders(providerFactory)
}

// createProvider creates a provider instance based on name
func createProvider(name string) (types.Provider, error) {
	var providerConfig types.ProviderConfig
	nameLower := strings.ToLower(name)

	// If we have a global config, try to get provider config from it
	if globalConfig != nil {
		// Use shared config package functions
		providerEntry := config.GetProviderEntry(globalConfig, nameLower)
		if providerEntry != nil {
			providerConfig = config.BuildProviderConfig(nameLower, providerEntry)
		} else {
			return nil, fmt.Errorf("provider %s not found in config", name)
		}
	} else {
		// Fall back to environment variables
		providerConfig = buildProviderConfigFromEnv(nameLower)
	}

	return providerFactory.CreateProvider(providerConfig.Type, providerConfig)
}

// createModelProvider creates a ModelProvider instance based on name
// This function returns the ModelProvider interface to demonstrate interface segregation
func createModelProvider(name string) (types.ModelProvider, error) {
	// Since all current providers implement the full Provider interface,
	// we can create a full provider and return it as a ModelProvider
	provider, err := createProvider(name)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// Note: getProviderConfig and buildProviderConfig are now provided by the shared config package

// buildProviderConfigFromEnv builds a types.ProviderConfig from environment variables
func buildProviderConfigFromEnv(name string) types.ProviderConfig {
	providerConfig := types.ProviderConfig{
		Type: config.DetermineProviderType(name, nil),
	}

	switch name {
	case "openai":
		providerConfig.APIKey = os.Getenv("OPENAI_API_KEY")
	case "anthropic":
		providerConfig.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	case "gemini":
		providerConfig.APIKey = os.Getenv("GEMINI_API_KEY")
	case "cerebras":
		providerConfig.APIKey = os.Getenv("CEREBRAS_API_KEY")
	case "openrouter":
		providerConfig.APIKey = os.Getenv("OPENROUTER_API_KEY")
	case "qwen":
		providerConfig.APIKey = os.Getenv("QWEN_API_KEY")
	}

	return providerConfig
}

// Note: getProviderType is now provided by the shared config package as DetermineProviderType

// Helper functions

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatTokens(tokens int) string {
	if tokens == 0 {
		return "-"
	}
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.0fK", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}
