package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TokenManager handles OAuth token updates (simplified for streaming demo)
type TokenManager struct {
	mu sync.Mutex
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	provider := flag.String("provider", "", "Provider to test (anthropic, gemini, qwen, cerebras, openrouter, openai, or all)")
	prompt := flag.String("prompt", "Write a haiku about artificial intelligence", "Prompt to test")
	compare := flag.Bool("compare", false, "Compare streaming vs non-streaming side-by-side")
	flag.Parse()

	fmt.Printf("%s%s╔═══════════════════════════════════════════════════════════╗%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s║       AI Provider Kit - Streaming Demo                   ║%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s╚═══════════════════════════════════════════════════════════╝%s\n\n", colorBold, colorCyan, colorReset)

	// Load config
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("❌ Failed to load config from '%s': %v\nHint: Create a config.yaml file or use -config flag to specify the path", *configPath, err)
	}

	// Initialize factory
	providerFactory := factory.NewProviderFactory()
	factory.RegisterDefaultProviders(providerFactory)

	// Create token manager for OAuth callbacks
	tokenManager := &TokenManager{}

	// Determine which providers to test
	providersToTest := []string{}
	if *provider == "all" {
		// Use enabled providers from config
		providersToTest = cfg.Providers.Enabled
	} else if *provider != "" {
		providersToTest = []string{*provider}
	} else {
		// Default to enabled providers
		providersToTest = cfg.Providers.Enabled
	}

	fmt.Printf("%s📋 Providers to test:%s %v\n\n", colorYellow, colorReset, providersToTest)

	// Test each provider
	for _, providerName := range providersToTest {
		if *compare {
			testProviderComparison(providerFactory, cfg, tokenManager, providerName, *prompt)
		} else {
			testProviderStreaming(providerFactory, cfg, tokenManager, providerName, *prompt)
		}
		fmt.Println()
	}

	fmt.Printf("\n%s✨ Demo completed!%s\n", colorGreen, colorReset)
}

// CreateMultiOAuthCallback creates a callback for multi-OAuth token refresh (simplified)
func (tm *TokenManager) CreateMultiOAuthCallback(providerName, credentialID string) func(id, accessToken, refreshToken string, expiresAt time.Time) error {
	return func(id, accessToken, refreshToken string, expiresAt time.Time) error {
		tm.mu.Lock()
		defer tm.mu.Unlock()

		fmt.Printf("\n🔄 [Token Refresh] Provider '%s' credential '%s' refreshed\n", providerName, credentialID)
		fmt.Printf("   New token expires: %s\n", expiresAt.Format(time.RFC3339))
		return nil
	}
}

func createProvider(factory *factory.DefaultProviderFactory, cfg *config.DemoConfig, tm *TokenManager, providerName string) (types.Provider, error) {
	// Get provider config entry
	providerEntry := config.GetProviderEntry(cfg, providerName)
	if providerEntry == nil {
		return nil, fmt.Errorf("provider '%s' not found in config", providerName)
	}

	// Build provider configuration
	providerCfg := config.BuildProviderConfig(providerName, providerEntry)
	providerCfg.Timeout = 60 * time.Second

	// Handle API keys (for load balancing)
	if len(providerEntry.APIKeys) > 0 {
		if providerCfg.ProviderConfig == nil {
			providerCfg.ProviderConfig = make(map[string]interface{})
		}
		providerCfg.ProviderConfig["api_keys"] = providerEntry.APIKeys
	}

	// Handle OAuth configuration with token refresh callbacks
	if len(providerEntry.OAuthCredentials) > 0 {
		credSets := config.ConvertOAuthCredentials(providerEntry.OAuthCredentials)

		// Add token refresh callbacks
		for _, credSet := range credSets {
			credSet.OnTokenRefresh = tm.CreateMultiOAuthCallback(providerName, credSet.ID)
		}

		providerCfg.OAuthCredentials = credSets

		// For Gemini, add project ID
		if providerCfg.Type == types.ProviderTypeGemini && providerEntry.ProjectID != "" {
			if providerCfg.ProviderConfig == nil {
				providerCfg.ProviderConfig = make(map[string]interface{})
			}
			providerCfg.ProviderConfig["project_id"] = providerEntry.ProjectID
		}
	}

	// Create provider instance
	provider, err := factory.CreateProvider(providerCfg.Type, providerCfg)
	if err != nil {
		return nil, err
	}

	// Configure the provider
	if err := provider.Configure(providerCfg); err != nil {
		return nil, err
	}

	// Authenticate if needed (API key only, OAuth is handled via OAuthCredentials)
	if providerCfg.APIKey != "" {
		authCfg := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: providerCfg.APIKey,
		}
		if err := provider.Authenticate(context.Background(), authCfg); err != nil {
			// Don't fail on auth errors, just warn
			fmt.Printf("⚠️  Authentication warning for '%s': %v\n", providerName, err)
		}
	}

	return provider, nil
}

func testProviderStreaming(factory *factory.DefaultProviderFactory, cfg *config.DemoConfig, tm *TokenManager, providerName string, prompt string) {
	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorBold, colorBlue, colorReset)
	fmt.Printf("%s%s  Provider: %s%s\n", colorBold, colorBlue, providerName, colorReset)
	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", colorBold, colorBlue, colorReset)

	provider, err := createProvider(factory, cfg, tm, providerName)
	if err != nil {
		fmt.Printf("%s❌ Failed to create provider: %v%s\n", colorRed, err, colorReset)
		return
	}

	if !provider.SupportsStreaming() {
		fmt.Printf("%s⚠️  Provider does not support streaming%s\n", colorYellow, colorReset)
		return
	}

	fmt.Printf("%s📝 Prompt:%s %s\n\n", colorCyan, colorReset, prompt)

	ctx := context.Background()
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   200,
		Temperature: 0.7,
		Stream:      true,
	}

	fmt.Printf("%s🌊 Streaming response:%s\n", colorGreen, colorReset)
	fmt.Printf("%s┌─────────────────────────────────────────────────────────┐%s\n", colorWhite, colorReset)
	fmt.Print("│ ")

	startTime := time.Now()
	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		fmt.Printf("\n%s❌ Failed to start streaming: %v%s\n", colorRed, err, colorReset)
		return
	}

	response := ""
	chunkCount := 0
	col := 2 // Track column position for word wrapping

	for {
		chunk, err := stream.Next()
		if err != nil {
			if err.Error() != "EOF" && !strings.Contains(err.Error(), "EOF") && err.Error() != "no more chunks" {
				fmt.Printf("\n%s❌ Error reading stream: %v%s\n", colorRed, err, colorReset)
			}
			break
		}

		if chunk.Content != "" {
			response += chunk.Content
			chunkCount++

			// Print with word wrapping at 57 characters
			for _, char := range chunk.Content {
				fmt.Print(string(char))
				col++
				if char == '\n' {
					fmt.Print("│ ")
					col = 2
				} else if col >= 57 {
					fmt.Printf(" │\n│ ")
					col = 2
				}
			}
		}

		if chunk.Done {
			break
		}
	}

	// Close the box
	if col > 2 {
		fmt.Print(strings.Repeat(" ", 57-col) + " │")
	}
	fmt.Printf("\n%s└─────────────────────────────────────────────────────────┘%s\n", colorWhite, colorReset)

	duration := time.Since(startTime)

	// Show stats
	fmt.Printf("\n%s📊 Statistics:%s\n", colorPurple, colorReset)
	fmt.Printf("   ⏱️  Duration: %v\n", duration)
	fmt.Printf("   📦 Chunks received: %d\n", chunkCount)
	fmt.Printf("   📝 Total characters: %d\n", len(response))
	if chunkCount > 0 {
		fmt.Printf("   ⚡ Avg chunk size: %.1f chars\n", float64(len(response))/float64(chunkCount))
		fmt.Printf("   🚀 Throughput: %.1f chars/sec\n", float64(len(response))/duration.Seconds())
	}
}

func testProviderComparison(factory *factory.DefaultProviderFactory, cfg *config.DemoConfig, tm *TokenManager, providerName string, prompt string) {
	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", colorBold, colorBlue, colorReset)
	fmt.Printf("%s%s  Comparison: Streaming vs Non-Streaming - %s%s\n", colorBold, colorBlue, providerName, colorReset)
	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", colorBold, colorBlue, colorReset)

	provider, err := createProvider(factory, cfg, tm, providerName)
	if err != nil {
		fmt.Printf("%s❌ Failed to create provider: %v%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("%s📝 Prompt:%s %s\n\n", colorCyan, colorReset, prompt)

	ctx := context.Background()
	baseOptions := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   150,
		Temperature: 0.7,
	}

	// Test 1: Non-Streaming
	fmt.Printf("%s%s┌─── Non-Streaming Mode ───────────────────────────────────┐%s\n", colorBold, colorYellow, colorReset)
	nonStreamOptions := baseOptions
	nonStreamOptions.Stream = false

	startTime := time.Now()
	fmt.Printf("│ %sRequesting...%s", colorYellow, colorReset)

	stream, err := provider.GenerateChatCompletion(ctx, nonStreamOptions)
	if err != nil {
		fmt.Printf("\n│ %s❌ Error: %v%s\n", colorRed, err, colorReset)
		fmt.Printf("%s└─────────────────────────────────────────────────────────┘%s\n\n", colorYellow, colorReset)
		return
	}

	response := ""
	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}
		if chunk.Content != "" {
			response += chunk.Content
		}
		if chunk.Done {
			break
		}
	}

	nonStreamDuration := time.Since(startTime)

	// Show the response appeared all at once
	fmt.Printf("\r│ %s[%v] Response arrived:%s\n", colorGreen, nonStreamDuration, colorReset)
	fmt.Printf("│ %s\n", wordWrap(response, 57))
	fmt.Printf("%s└─────────────────────────────────────────────────────────┘%s\n", colorYellow, colorReset)
	fmt.Printf("%s   ⏱️  Total time: %v (waited entire duration for complete response)%s\n\n", colorYellow, nonStreamDuration, colorReset)

	// Test 2: Streaming
	fmt.Printf("%s%s┌─── Streaming Mode (Real-time) ───────────────────────────┐%s\n", colorBold, colorGreen, colorReset)
	streamOptions := baseOptions
	streamOptions.Stream = true

	startTime = time.Now()
	stream, err = provider.GenerateChatCompletion(ctx, streamOptions)
	if err != nil {
		fmt.Printf("│ %s❌ Error: %v%s\n", colorRed, err, colorReset)
		fmt.Printf("%s└─────────────────────────────────────────────────────────┘%s\n\n", colorGreen, colorReset)
		return
	}

	fmt.Print("│ ")
	response = ""
	chunkCount := 0
	firstChunkTime := time.Duration(0)
	col := 2

	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}

		if chunk.Content != "" {
			if chunkCount == 0 {
				firstChunkTime = time.Since(startTime)
			}
			response += chunk.Content
			chunkCount++

			// Simulate real-time display with slight delay
			for _, char := range chunk.Content {
				fmt.Print(string(char))
				time.Sleep(5 * time.Millisecond) // Visual effect
				col++
				if char == '\n' {
					fmt.Print("│ ")
					col = 2
				} else if col >= 57 {
					fmt.Printf(" │\n│ ")
					col = 2
				}
			}
		}

		if chunk.Done {
			break
		}
	}

	if col > 2 {
		fmt.Print(strings.Repeat(" ", 57-col) + " │")
	}
	fmt.Print("\n")

	streamDuration := time.Since(startTime)
	fmt.Printf("%s└─────────────────────────────────────────────────────────┘%s\n", colorGreen, colorReset)
	fmt.Printf("%s   ⏱️  Time to first chunk: %v%s\n", colorGreen, firstChunkTime, colorReset)
	fmt.Printf("%s   ⏱️  Total time: %v (but user saw output immediately!)%s\n", colorGreen, streamDuration, colorReset)
	fmt.Printf("%s   📦 Received %d chunks progressively%s\n\n", colorGreen, chunkCount, colorReset)

	// Comparison summary
	fmt.Printf("%s%s╔═══════════════════════════════════════════════════════════╗%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s║  Comparison Summary                                       ║%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s╠═══════════════════════════════════════════════════════════╣%s\n", colorBold, colorCyan, colorReset)

	if firstChunkTime > 0 {
		improvement := float64(nonStreamDuration-firstChunkTime) / float64(nonStreamDuration) * 100
		fmt.Printf("║  %sTime to first output:%s                                   ║\n", colorBold, colorReset)
		fmt.Printf("║    Non-Streaming: %v %s(100%% wait)%s                 ║\n",
			nonStreamDuration, colorRed, colorReset)
		fmt.Printf("║    Streaming:     %v %s(%.0f%% faster!)%s            ║\n",
			firstChunkTime, colorGreen, improvement, colorReset)
		fmt.Printf("║                                                           ║\n")
	}

	fmt.Printf("║  %sUser Experience:%s                                        ║\n", colorBold, colorReset)
	fmt.Printf("║    Non-Streaming: ❌ Wait, then see all at once           ║\n")
	fmt.Printf("║    Streaming:     ✅ See results as they're generated     ║\n")
	fmt.Printf("%s%s╚═══════════════════════════════════════════════════════════╝%s\n", colorBold, colorCyan, colorReset)
}

func wordWrap(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 <= width {
			if currentLine == "" {
				currentLine = word
			} else {
				currentLine += " " + word
			}
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return strings.Join(lines, "\n│ ")
}
