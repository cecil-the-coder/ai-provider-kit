//go:build ignore
// +build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/anthropic"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
	fmt.Println("=== Anthropic Tool Calling Examples ===\n")

	// Example 1: Basic tool calling
	basicToolCalling()

	// Example 2: ToolChoice modes
	toolChoiceModes()

	// Example 3: Parallel tool calls
	parallelToolCalls()

	// Example 4: Multi-turn conversation
	multiTurnConversation()
}

// Example 1: Basic tool calling
func basicToolCalling() {
	fmt.Println("--- Example 1: Basic Tool Calling ---")

	// Create provider
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "your-api-key-here", // Replace with actual API key
	}
	provider := anthropic.NewAnthropicProvider(config)

	// Define a weather tool
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

	fmt.Println()
}

// Example 2: ToolChoice modes
func toolChoiceModes() {
	fmt.Println("--- Example 2: ToolChoice Modes ---")

	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "your-api-key-here",
	}
	provider := anthropic.NewAnthropicProvider(config)

	tools := []types.Tool{
		{
			Name:        "calculate",
			Description: "Perform mathematical calculations",
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

	// Auto mode (default)
	fmt.Println("1. Auto mode - model decides:")
	options := types.GenerateOptions{
		Prompt: "What is 25 * 4?",
		Tools:  tools,
		ToolChoice: &types.ToolChoice{
			Mode: types.ToolChoiceAuto,
		},
	}
	demonstrateToolChoice(provider, options)

	// Required mode (Anthropic uses "any")
	fmt.Println("2. Required mode - must use a tool:")
	options = types.GenerateOptions{
		Prompt: "Calculate 100 / 5",
		Tools:  tools,
		ToolChoice: &types.ToolChoice{
			Mode: types.ToolChoiceRequired,
		},
	}
	demonstrateToolChoice(provider, options)

	// Specific mode (Anthropic uses "tool")
	fmt.Println("3. Specific mode - force specific tool:")
	options = types.GenerateOptions{
		Prompt: "What is 7 + 8?",
		Tools:  tools,
		ToolChoice: &types.ToolChoice{
			Mode:         types.ToolChoiceSpecific,
			FunctionName: "calculate",
		},
	}
	demonstrateToolChoice(provider, options)

	fmt.Println()
}

// Example 3: Parallel tool calls
func parallelToolCalls() {
	fmt.Println("--- Example 3: Parallel Tool Calls ---")

	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "your-api-key-here",
	}
	provider := anthropic.NewAnthropicProvider(config)

	tools := []types.Tool{
		{
			Name:        "get_weather",
			Description: "Get current weather for a location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"location"},
			},
		},
		{
			Name:        "get_time",
			Description: "Get current time in a timezone",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timezone": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"timezone"},
			},
		},
	}

	options := types.GenerateOptions{
		Prompt: "What's the weather in New York, London, and what time is it in Tokyo?",
		Tools:  tools,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer stream.Close()

	chunk, err := stream.Next()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	if len(chunk.Choices) > 0 && len(chunk.Choices[0].Message.ToolCalls) > 0 {
		fmt.Printf("Received %d parallel tool calls:\n", len(chunk.Choices[0].Message.ToolCalls))
		for i, toolCall := range chunk.Choices[0].Message.ToolCalls {
			fmt.Printf("%d. %s(%s)\n", i+1, toolCall.Function.Name, toolCall.Function.Arguments)
		}
	}

	fmt.Println()
}

// Example 4: Multi-turn conversation with tool execution
func multiTurnConversation() {
	fmt.Println("--- Example 4: Multi-Turn Conversation ---")

	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "your-api-key-here",
	}
	provider := anthropic.NewAnthropicProvider(config)

	tools := []types.Tool{
		{
			Name:        "get_stock_price",
			Description: "Get the current stock price for a ticker symbol",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ticker": map[string]interface{}{
						"type":        "string",
						"description": "Stock ticker symbol (e.g., AAPL, GOOGL)",
					},
				},
				"required": []string{"ticker"},
			},
		},
	}

	// Initial request
	fmt.Println("Step 1: Initial request")
	options := types.GenerateOptions{
		Prompt: "What's Apple's stock price?",
		Tools:  tools,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer stream.Close()

	chunk, err := stream.Next()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Check for tool calls
	if len(chunk.Choices) == 0 || len(chunk.Choices[0].Message.ToolCalls) == 0 {
		fmt.Println("No tool calls received")
		return
	}

	fmt.Println("Step 2: Tool call received")
	toolCall := chunk.Choices[0].Message.ToolCalls[0]
	fmt.Printf("  Function: %s\n", toolCall.Function.Name)
	fmt.Printf("  Arguments: %s\n", toolCall.Function.Arguments)

	// Execute tool (mock execution)
	fmt.Println("Step 3: Executing tool...")
	toolResult := executeStockPriceTool(toolCall)
	fmt.Printf("  Result: %s\n", toolResult)

	// Send result back
	fmt.Println("Step 4: Sending result back")
	messages := []types.ChatMessage{
		{
			Role:    "user",
			Content: "What's Apple's stock price?",
		},
		{
			Role:      "assistant",
			Content:   "",
			ToolCalls: []types.ToolCall{toolCall},
		},
		{
			Role:       "tool",
			Content:    toolResult,
			ToolCallID: toolCall.ID,
		},
	}

	options = types.GenerateOptions{
		Messages: messages,
		Tools:    tools,
	}

	stream, err = provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer stream.Close()

	chunk, err = stream.Next()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Step 5: Final response")
	fmt.Printf("  %s\n", chunk.Content)

	fmt.Println()
}

// Helper functions

func demonstrateToolChoice(provider *anthropic.AnthropicProvider, options types.GenerateOptions) {
	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer stream.Close()

	chunk, err := stream.Next()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if len(chunk.Choices) > 0 && len(chunk.Choices[0].Message.ToolCalls) > 0 {
		for _, toolCall := range chunk.Choices[0].Message.ToolCalls {
			fmt.Printf("   Tool call: %s(%s)\n", toolCall.Function.Name, toolCall.Function.Arguments)
		}
	} else {
		fmt.Printf("   Response: %s\n", chunk.Content)
	}
}

func executeStockPriceTool(toolCall types.ToolCall) string {
	// Parse arguments
	var args map[string]interface{}
	json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

	// Mock stock price lookup
	ticker := args["ticker"].(string)

	result := map[string]interface{}{
		"ticker":       ticker,
		"price":        175.43,
		"currency":     "USD",
		"change":       2.15,
		"change_pct":   1.24,
		"last_updated": "2024-11-16T15:30:00Z",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON)
}
