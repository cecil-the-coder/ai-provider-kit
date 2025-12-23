//go:build ignore
// +build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/openai"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
	// This is an example showing how to use tool calling with the OpenAI provider

	// Create provider
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "your-api-key-here", // Replace with actual API key
	}
	provider := openai.NewOpenAIProvider(config)

	// Define a tool
	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get the current weather in a location",
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
						"description": "The temperature unit",
					},
				},
				"required": []string{"location"},
			},
		},
	}

	// Make a request with tools
	options := types.GenerateOptions{
		Prompt: "What's the weather like in San Francisco?",
		Tools:  tools,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer stream.Close()

	// Read response
	chunk, err := stream.Next()
	if err != nil {
		log.Fatalf("Error reading response: %v", err)
	}

	// Check if we got tool calls
	if len(chunk.Choices) > 0 && len(chunk.Choices[0].Message.ToolCalls) > 0 {
		fmt.Println("Tool calls received:")
		for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
			fmt.Printf("  ID: %s\n", toolCall.ID)
			fmt.Printf("  Type: %s\n", toolCall.Type)
			fmt.Printf("  Function: %s\n", toolCall.Function.Name)
			fmt.Printf("  Arguments: %s\n", toolCall.Function.Arguments)

			// Parse arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
				fmt.Printf("  Parsed Arguments: %+v\n", args)
			}
		}
	} else {
		fmt.Printf("Response content: %s\n", chunk.Content)
	}
}
