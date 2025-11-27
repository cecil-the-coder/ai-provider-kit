package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
)

// Assistant maintains the state of the CLI chatbot
type Assistant struct {
	provider     types.Provider
	providerName string
	model        string
	systemPrompt string
	history      []types.ChatMessage
	totalUsage   types.Usage
	factory      *factory.DefaultProviderFactory
	config       *config.DemoConfig
	configPath   string
}

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	providerName := flag.String("provider", "", "Initial provider (e.g., anthropic, openai)")
	modelOverride := flag.String("model", "", "Override the default model")
	systemPrompt := flag.String("system", "You are a helpful AI assistant.", "System prompt")
	flag.Parse()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Print welcome banner
	printBanner()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("%sError:%s Failed to load config from %s: %v\n", colorRed, colorReset, *configPath, err)
		fmt.Printf("%sHint:%s Create a config.yaml file or specify path with -config flag\n", colorYellow, colorReset)
		os.Exit(1)
	}

	// Initialize factory
	providerFactory := factory.NewProviderFactory()
	factory.RegisterDefaultProviders(providerFactory)

	// Determine initial provider
	initialProvider := *providerName
	if initialProvider == "" && len(cfg.Providers.Enabled) > 0 {
		initialProvider = cfg.Providers.Enabled[0]
	}

	if initialProvider == "" {
		fmt.Printf("%sError:%s No provider specified and no enabled providers in config\n", colorRed, colorReset)
		os.Exit(1)
	}

	// Create assistant
	assistant := &Assistant{
		providerName: initialProvider,
		model:        *modelOverride,
		systemPrompt: *systemPrompt,
		history:      []types.ChatMessage{},
		factory:      providerFactory,
		config:       cfg,
		configPath:   *configPath,
	}

	// Initialize provider
	if err := assistant.initProvider(); err != nil {
		fmt.Printf("%sError:%s Failed to initialize provider '%s': %v\n", colorRed, colorReset, initialProvider, err)
		os.Exit(1)
	}

	// Print initial info
	assistant.printStatus()
	fmt.Println()
	fmt.Printf("Type %s/help%s for available commands, or start chatting!\n\n", colorCyan, colorReset)

	// Create scanner for input
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for long inputs

	// Main REPL loop
	for {
		// Print prompt
		fmt.Printf("%s%s>%s ", colorBold, colorGreen, colorReset)

		// Check for signals
		select {
		case <-sigChan:
			fmt.Println("\n\nGoodbye!")
			os.Exit(0)
		default:
		}

		// Read input
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Printf("\n%sError reading input:%s %v\n", colorRed, colorReset, err)
			}
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle special commands
		if strings.HasPrefix(input, "/") {
			shouldQuit := assistant.handleCommand(input)
			if shouldQuit {
				break
			}
			continue
		}

		// Check for multi-line input (ends with \)
		for strings.HasSuffix(input, "\\") {
			input = strings.TrimSuffix(input, "\\") + "\n"
			fmt.Printf("%s%s...%s ", colorDim, colorGreen, colorReset)
			if scanner.Scan() {
				input += scanner.Text()
			}
		}

		// Send message to AI
		assistant.chat(input)
	}

	fmt.Println("\nGoodbye!")
}

func printBanner() {
	fmt.Printf("\n%s%s", colorBold, colorCyan)
	fmt.Println("  CLI Assistant - AI Provider Kit Demo")
	fmt.Printf("%s%s", colorReset, colorDim)
	fmt.Println("  Interactive chat with streaming responses")
	fmt.Printf("%s\n", colorReset)
}

func (a *Assistant) printStatus() {
	fmt.Printf("%sProvider:%s %s\n", colorYellow, colorReset, a.providerName)
	model := a.model
	if model == "" && a.provider != nil {
		model = a.provider.GetDefaultModel()
	}
	fmt.Printf("%sModel:%s    %s\n", colorYellow, colorReset, model)
}

func (a *Assistant) initProvider() error {
	// Get provider config entry
	providerEntry := config.GetProviderEntry(a.config, a.providerName)
	if providerEntry == nil {
		return fmt.Errorf("provider '%s' not found in config", a.providerName)
	}

	// Build provider configuration
	providerCfg := config.BuildProviderConfig(a.providerName, providerEntry)
	providerCfg.Timeout = 120 * time.Second

	// Override model if specified
	if a.model != "" {
		providerCfg.DefaultModel = a.model
	}

	// Handle API keys
	if len(providerEntry.APIKeys) > 0 {
		if providerCfg.ProviderConfig == nil {
			providerCfg.ProviderConfig = make(map[string]interface{})
		}
		providerCfg.ProviderConfig["api_keys"] = providerEntry.APIKeys
	}

	// Handle OAuth credentials
	if len(providerEntry.OAuthCredentials) > 0 {
		credSets := config.ConvertOAuthCredentials(providerEntry.OAuthCredentials)
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
	provider, err := a.factory.CreateProvider(providerCfg.Type, providerCfg)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Configure the provider
	if err := provider.Configure(providerCfg); err != nil {
		return fmt.Errorf("failed to configure provider: %w", err)
	}

	// Authenticate if API key is available
	if providerCfg.APIKey != "" {
		authCfg := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: providerCfg.APIKey,
		}
		if err := provider.Authenticate(context.Background(), authCfg); err != nil {
			// Don't fail on auth errors, just warn
			fmt.Printf("%sWarning:%s Authentication issue: %v\n", colorYellow, colorReset, err)
		}
	}

	a.provider = provider
	return nil
}

func (a *Assistant) handleCommand(input string) bool {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help":
		a.showHelp()

	case "/quit", "/exit":
		return true

	case "/clear":
		a.history = []types.ChatMessage{}
		fmt.Printf("%sConversation history cleared.%s\n\n", colorGreen, colorReset)

	case "/model":
		if len(parts) < 2 {
			model := a.model
			if model == "" && a.provider != nil {
				model = a.provider.GetDefaultModel()
			}
			fmt.Printf("Current model: %s\n", model)
			fmt.Printf("Usage: /model <model-name>\n\n")
		} else {
			a.model = parts[1]
			fmt.Printf("%sModel changed to:%s %s\n\n", colorGreen, colorReset, a.model)
		}

	case "/provider":
		if len(parts) < 2 {
			fmt.Printf("Current provider: %s\n", a.providerName)
			fmt.Printf("Available providers: %v\n", a.config.Providers.Enabled)
			fmt.Printf("Usage: /provider <provider-name>\n\n")
		} else {
			newProvider := parts[1]
			oldProvider := a.providerName
			a.providerName = newProvider
			a.model = "" // Reset model override when switching providers

			if err := a.initProvider(); err != nil {
				fmt.Printf("%sError:%s Failed to switch to provider '%s': %v\n", colorRed, colorReset, newProvider, err)
				a.providerName = oldProvider
				a.initProvider() // Try to restore old provider
			} else {
				fmt.Printf("%sProvider changed to:%s %s\n", colorGreen, colorReset, newProvider)
				a.printStatus()
				fmt.Println()
			}
		}

	case "/history":
		if len(a.history) == 0 {
			fmt.Printf("%sNo conversation history.%s\n\n", colorDim, colorReset)
		} else {
			fmt.Printf("%s--- Conversation History ---\n%s", colorCyan, colorReset)
			for i, msg := range a.history {
				roleColor := colorBlue
				if msg.Role == "assistant" {
					roleColor = colorMagenta
				}
				content := msg.Content
				if len(content) > 100 {
					content = content[:100] + "..."
				}
				fmt.Printf("%d. %s[%s]%s %s\n", i+1, roleColor, msg.Role, colorReset, content)
			}
			fmt.Println()
		}

	case "/usage":
		fmt.Printf("%s--- Token Usage ---\n%s", colorCyan, colorReset)
		fmt.Printf("Prompt tokens:     %d\n", a.totalUsage.PromptTokens)
		fmt.Printf("Completion tokens: %d\n", a.totalUsage.CompletionTokens)
		fmt.Printf("Total tokens:      %d\n\n", a.totalUsage.TotalTokens)

	case "/system":
		if len(parts) < 2 {
			fmt.Printf("Current system prompt: %s\n\n", a.systemPrompt)
		} else {
			a.systemPrompt = strings.Join(parts[1:], " ")
			fmt.Printf("%sSystem prompt updated.%s\n\n", colorGreen, colorReset)
		}

	default:
		fmt.Printf("%sUnknown command:%s %s\n", colorRed, colorReset, cmd)
		fmt.Printf("Type %s/help%s for available commands.\n\n", colorCyan, colorReset)
	}

	return false
}

func (a *Assistant) showHelp() {
	fmt.Printf("\n%s--- Available Commands ---\n%s", colorCyan, colorReset)
	fmt.Printf("  %s/help%s      - Show this help message\n", colorYellow, colorReset)
	fmt.Printf("  %s/quit%s      - Exit the program (also /exit)\n", colorYellow, colorReset)
	fmt.Printf("  %s/clear%s     - Clear conversation history\n", colorYellow, colorReset)
	fmt.Printf("  %s/model%s     - Show or change the model\n", colorYellow, colorReset)
	fmt.Printf("  %s/provider%s  - Show or change the provider\n", colorYellow, colorReset)
	fmt.Printf("  %s/history%s   - Show conversation history\n", colorYellow, colorReset)
	fmt.Printf("  %s/usage%s     - Show cumulative token usage\n", colorYellow, colorReset)
	fmt.Printf("  %s/system%s    - Show or set system prompt\n", colorYellow, colorReset)
	fmt.Printf("\n%sTips:%s\n", colorCyan, colorReset)
	fmt.Printf("  - End a line with \\ for multi-line input\n")
	fmt.Printf("  - Press Ctrl+C to exit gracefully\n\n")
}

func (a *Assistant) chat(userMessage string) {
	// Add user message to history
	a.history = append(a.history, types.ChatMessage{
		Role:    "user",
		Content: userMessage,
	})

	// Build messages for the API call
	messages := []types.ChatMessage{}

	// Add system prompt
	if a.systemPrompt != "" {
		messages = append(messages, types.ChatMessage{
			Role:    "system",
			Content: a.systemPrompt,
		})
	}

	// Add conversation history
	messages = append(messages, a.history...)

	// Prepare options
	model := a.model
	if model == "" && a.provider != nil {
		model = a.provider.GetDefaultModel()
	}

	options := types.GenerateOptions{
		Messages:    messages,
		Model:       model,
		MaxTokens:   4096,
		Temperature: 0.7,
		Stream:      true,
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Start timing
	startTime := time.Now()

	// Generate response
	stream, err := a.provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		fmt.Printf("\n%sError:%s %v\n\n", colorRed, colorReset, err)
		// Remove failed user message from history
		a.history = a.history[:len(a.history)-1]
		return
	}

	// Print assistant response header
	fmt.Printf("\n%s%sAssistant:%s ", colorBold, colorMagenta, colorReset)

	// Collect response
	var response strings.Builder
	var usage types.Usage

	for {
		chunk, err := stream.Next()
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "EOF") && err.Error() != "no more chunks" {
				fmt.Printf("\n%sStream error:%s %v\n", colorRed, colorReset, err)
			}
			break
		}

		// Print content as it arrives
		if chunk.Content != "" {
			fmt.Print(chunk.Content)
			response.WriteString(chunk.Content)
		}

		// Capture usage if available
		if chunk.Usage.TotalTokens > 0 {
			usage = chunk.Usage
		}

		if chunk.Done {
			break
		}
	}

	// Close stream
	_ = stream.Close()

	// Calculate elapsed time
	elapsed := time.Since(startTime)

	// Print newlines and stats
	fmt.Println()

	// Add assistant response to history
	if response.Len() > 0 {
		a.history = append(a.history, types.ChatMessage{
			Role:    "assistant",
			Content: response.String(),
		})
	}

	// Update cumulative usage
	if usage.TotalTokens > 0 {
		a.totalUsage.PromptTokens += usage.PromptTokens
		a.totalUsage.CompletionTokens += usage.CompletionTokens
		a.totalUsage.TotalTokens += usage.TotalTokens

		// Print usage info
		fmt.Printf("%s[%v | %d tokens]%s\n\n",
			colorDim, elapsed.Round(time.Millisecond), usage.TotalTokens, colorReset)
	} else {
		fmt.Printf("%s[%v]%s\n\n", colorDim, elapsed.Round(time.Millisecond), colorReset)
	}
}
