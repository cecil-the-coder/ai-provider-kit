// Example demonstrating how to use native Anthropic tools like web_search
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	// Create provider factory and register providers
	providerFactory := factory.NewProviderFactory()
	factory.RegisterDefaultProviders(providerFactory)

	// Create Anthropic provider
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: apiKey,
	}

	provider, err := providerFactory.CreateProvider(types.ProviderTypeAnthropic, config)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Example 1: Basic web_search tool
	basicWebSearchTool := types.Tool{
		Type: "web_search_20250305",
		Name: "web_search",
	}

	// Example 2: Web search with domain filtering
	filteredWebSearchTool := types.Tool{
		Type: "web_search_20250305",
		Name: "web_search",
		InputSchema: map[string]interface{}{
			"allowed_domains": []string{"docs.anthropic.com", "github.com"},
			"blocked_domains": []string{"spam.com"},
			"max_uses":        3,
		},
	}

	// Example 3: Custom tool alongside native tool
	customTool := types.Tool{
		Name:        "get_weather",
		Description: "Get current weather for a location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "City and state, e.g., San Francisco, CA",
				},
			},
			"required": []string{"location"},
		},
	}

	// Use basic web search
	fmt.Println("Example 1: Basic web search")
	options1 := types.GenerateOptions{
		Prompt: "Search for the latest news about Claude AI",
		Tools:  []types.Tool{basicWebSearchTool},
	}

	stream1, err := provider.GenerateChatCompletion(context.Background(), options1)
	if err != nil {
		log.Printf("Error with basic web search: %v", err)
	} else {
		handleStream(stream1, "Basic Web Search")
	}

	// Use filtered web search
	fmt.Println("\nExample 2: Filtered web search")
	options2 := types.GenerateOptions{
		Prompt: "Search for Anthropic API documentation",
		Tools:  []types.Tool{filteredWebSearchTool},
	}

	stream2, err := provider.GenerateChatCompletion(context.Background(), options2)
	if err != nil {
		log.Printf("Error with filtered web search: %v", err)
	} else {
		handleStream(stream2, "Filtered Web Search")
	}

	// Mix native and custom tools
	fmt.Println("\nExample 3: Mixed tools")
	options3 := types.GenerateOptions{
		Prompt: "Search for weather API information and get the weather in SF",
		Tools:  []types.Tool{basicWebSearchTool, customTool},
	}

	stream3, err := provider.GenerateChatCompletion(context.Background(), options3)
	if err != nil {
		log.Printf("Error with mixed tools: %v", err)
	} else {
		handleStream(stream3, "Mixed Tools")
	}
}

func handleStream(stream types.ChatCompletionStream, label string) {
	defer stream.Close()

	chunk, err := stream.Next()
	if err != nil {
		log.Printf("%s error: %v", label, err)
		return
	}

	if len(chunk.Choices) > 0 {
		msg := chunk.Choices[0].Message
		if msg.Content != "" {
			fmt.Printf("%s Response: %s\n", label, msg.Content)
		}
		if len(msg.ToolCalls) > 0 {
			fmt.Printf("%s Tool Calls:\n", label)
			for _, tc := range msg.ToolCalls {
				fmt.Printf("  - %s: %s\n", tc.Function.Name, tc.Function.Arguments)
			}
		}
	}

	fmt.Printf("%s Usage - Prompt: %d, Completion: %d, Total: %d\n",
		label,
		chunk.Usage.PromptTokens,
		chunk.Usage.CompletionTokens,
		chunk.Usage.TotalTokens,
	)
}
