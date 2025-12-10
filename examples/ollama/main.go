package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/ollama"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
	fmt.Println("=== Ollama Provider Example ===")

	// Example 1: Local Ollama instance
	fmt.Println("1. Local Ollama Instance")
	runLocalExample()

	fmt.Println("\n" + repeat("=", 50) + "\n")

	// Example 2: Cloud Ollama with API key (if configured)
	if os.Getenv("OLLAMA_API_KEY") != "" {
		fmt.Println("2. Ollama Cloud with API Key")
		runCloudExample()
	} else {
		fmt.Println("2. Ollama Cloud (Skipped - OLLAMA_API_KEY not set)")
	}

	fmt.Println("\n" + repeat("=", 50) + "\n")

	// Example 3: Model discovery
	fmt.Println("3. Model Discovery")
	runModelDiscoveryExample()

	fmt.Println("\n" + repeat("=", 50) + "\n")

	// Example 4: Tool calling
	fmt.Println("4. Tool Calling Example")
	runToolCallingExample()
}

// runLocalExample demonstrates basic chat with a local Ollama instance
func runLocalExample() {
	// Configure local Ollama provider
	config := types.ProviderConfig{
		Type:         types.ProviderTypeOllama,
		Name:         "ollama-local",
		BaseURL:      "http://localhost:11434",
		DefaultModel: "llama3.1:8b",
	}

	provider := ollama.NewOllamaProvider(config)

	// Test connectivity
	ctx := context.Background()
	if err := provider.HealthCheck(ctx); err != nil {
		log.Printf("Warning: Local Ollama not available: %v\n", err)
		log.Println("Make sure Ollama is running: 'ollama serve'")
		return
	}

	fmt.Println("Status: Connected to local Ollama")
	fmt.Printf("Default Model: %s\n\n", provider.GetDefaultModel())

	// Generate chat completion
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "Write a haiku about coding",
			},
		},
		Model:       "llama3.1:8b",
		Temperature: 0.7,
		MaxTokens:   100,
	}

	fmt.Println("Prompt: Write a haiku about coding")
	fmt.Println("Response: ")

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer func() { _ = stream.Close() }()

	// Stream the response
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading stream: %v\n", err)
			return
		}
		fmt.Print(chunk.Content)
	}
	fmt.Println()
}

// runCloudExample demonstrates using Ollama Cloud with API key
func runCloudExample() {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeOllama,
		Name:         "ollama-cloud",
		BaseURL:      "https://api.ollama.com",
		APIKeyEnv:    "OLLAMA_API_KEY",
		DefaultModel: "llama3.1:8b",
	}

	provider := ollama.NewOllamaProvider(config)

	// Verify authentication
	if !provider.IsAuthenticated() {
		log.Println("Error: Not authenticated with Ollama Cloud")
		return
	}

	fmt.Println("Status: Authenticated with Ollama Cloud")

	ctx := context.Background()

	// Test connectivity
	if err := provider.HealthCheck(ctx); err != nil {
		log.Printf("Error: Cloud connectivity failed: %v\n", err)
		return
	}

	// Generate completion
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "What are the three laws of robotics?",
			},
		},
		Model:       "llama3.1:8b",
		Temperature: 0.5,
	}

	fmt.Println("Prompt: What are the three laws of robotics?")
	fmt.Println("Response: ")

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer func() { _ = stream.Close() }()

	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error: %v\n", err)
			return
		}
		fmt.Print(chunk.Content)
	}
	fmt.Println()
}

// runModelDiscoveryExample demonstrates model discovery and capability detection
func runModelDiscoveryExample() {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeOllama,
		Name:         "ollama-local",
		BaseURL:      "http://localhost:11434",
		DefaultModel: "llama3.1:8b",
	}

	provider := ollama.NewOllamaProvider(config)
	ctx := context.Background()

	// Get available models
	models, err := provider.GetModels(ctx)
	if err != nil {
		log.Printf("Error fetching models: %v\n", err)
		return
	}

	fmt.Printf("Found %d models:\n\n", len(models))

	// Display first 5 models
	displayCount := 5
	if len(models) < displayCount {
		displayCount = len(models)
	}

	for i := 0; i < displayCount; i++ {
		model := models[i]
		fmt.Printf("Model: %s\n", model.ID)
		fmt.Printf("  Description: %s\n", model.Description)
		fmt.Printf("  Max Tokens: %d\n", model.MaxTokens)
		fmt.Printf("  Streaming: %v\n", model.SupportsStreaming)
		fmt.Printf("  Tool Calling: %v\n", model.SupportsToolCalling)
		fmt.Printf("  Capabilities: %v\n", model.Capabilities)
		fmt.Println()
	}

	if len(models) > displayCount {
		fmt.Printf("... and %d more models\n", len(models)-displayCount)
	}
}

// runToolCallingExample demonstrates function calling with tools
func runToolCallingExample() {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeOllama,
		Name:         "ollama-local",
		BaseURL:      "http://localhost:11434",
		DefaultModel: "llama3.1:8b",
	}

	provider := ollama.NewOllamaProvider(config)
	ctx := context.Background()

	// Check connectivity
	if err := provider.HealthCheck(ctx); err != nil {
		log.Printf("Warning: Ollama not available: %v\n", err)
		return
	}

	// Define tools
	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get the current weather for a location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "The city and state, e.g. San Francisco, CA",
					},
					"unit": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"celsius", "fahrenheit"},
						"description": "Temperature unit",
					},
				},
				"required": []string{"location"},
			},
		},
		{
			Name:        "calculate",
			Description: "Perform a mathematical calculation",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{
						"type":        "string",
						"description": "The mathematical expression to evaluate",
					},
				},
				"required": []string{"expression"},
			},
		},
	}

	// Generate with tools
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: "What's the weather like in San Francisco and what's 42 times 17?",
			},
		},
		Model:       "llama3.1:8b",
		Tools:       tools,
		Temperature: 0.3,
	}

	fmt.Println("Prompt: What's the weather like in San Francisco and what's 42 times 17?")
	fmt.Println("Available Tools: get_weather, calculate")
	fmt.Println("\nResponse:")

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	defer func() { _ = stream.Close() }()

	hasToolCalls := false
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error: %v\n", err)
			return
		}

		// Check for tool calls
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			if !hasToolCalls {
				fmt.Println("\nTool Calls Requested:")
				hasToolCalls = true
			}
			for _, toolCall := range chunk.Choices[0].Delta.ToolCalls {
				fmt.Printf("  - %s: %s(%s)\n",
					toolCall.Type,
					toolCall.Function.Name,
					toolCall.Function.Arguments)
			}
		} else if chunk.Content != "" {
			fmt.Print(chunk.Content)
		}

		// Display usage on final chunk
		if chunk.Done && chunk.Usage.TotalTokens > 0 {
			fmt.Printf("\n\nToken Usage:")
			fmt.Printf("\n  Prompt: %d tokens", chunk.Usage.PromptTokens)
			fmt.Printf("\n  Completion: %d tokens", chunk.Usage.CompletionTokens)
			fmt.Printf("\n  Total: %d tokens\n", chunk.Usage.TotalTokens)
		}
	}
	fmt.Println()
}

// Helper function to repeat strings (Go doesn't have this built-in)
func repeat(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
