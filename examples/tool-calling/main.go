// Package main demonstrates tool calling functionality with the ai-provider-kit.
// This example shows how to define tools, send requests that require tool use,
// parse tool call responses, execute tools locally, and send results back.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Tool Definitions
// =============================================================================

// Define the tools that the AI can call

var getWeatherTool = types.Tool{
	Name:        "get_weather",
	Description: "Get the current weather for a location. Returns temperature, conditions, humidity, and wind speed.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": map[string]interface{}{
				"type":        "string",
				"description": "The city and country, e.g. 'Tokyo, Japan' or 'New York, USA'",
			},
		},
		"required": []string{"location"},
	},
}

var calculateTool = types.Tool{
	Name:        "calculate",
	Description: "Evaluate a mathematical expression. Supports basic arithmetic operations (+, -, *, /).",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"expression": map[string]interface{}{
				"type":        "string",
				"description": "The mathematical expression to evaluate, e.g. '15 * 23' or '100 / 4'",
			},
		},
		"required": []string{"expression"},
	},
}

var getTimeTool = types.Tool{
	Name:        "get_time",
	Description: "Get the current time in a specific timezone.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "The timezone name, e.g. 'America/New_York', 'Europe/London', 'Asia/Tokyo'",
			},
		},
		"required": []string{"timezone"},
	},
}

// =============================================================================
// Tool Execution Functions
// =============================================================================

// executeToolCall executes a tool call and returns the result as a string
func executeToolCall(toolCall types.ToolCall) (string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments JSON: %w", err)
	}

	switch toolCall.Function.Name {
	case "get_weather":
		return getWeather(args)
	case "calculate":
		return calculate(args)
	case "get_time":
		return getTime(args)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
}

// getWeather returns mock weather data for a location
func getWeather(args map[string]interface{}) (string, error) {
	location, ok := args["location"].(string)
	if !ok {
		return "", fmt.Errorf("location must be a string")
	}

	// Mock weather data based on location
	weatherData := map[string]map[string]interface{}{
		"tokyo": {
			"temperature": 18,
			"unit":        "celsius",
			"conditions":  "partly cloudy",
			"humidity":    65,
			"wind_speed":  12,
			"wind_unit":   "km/h",
		},
		"new york": {
			"temperature": 22,
			"unit":        "celsius",
			"conditions":  "sunny",
			"humidity":    45,
			"wind_speed":  8,
			"wind_unit":   "km/h",
		},
		"london": {
			"temperature": 14,
			"unit":        "celsius",
			"conditions":  "overcast",
			"humidity":    78,
			"wind_speed":  15,
			"wind_unit":   "km/h",
		},
	}

	// Find weather data (case-insensitive partial match)
	locationLower := strings.ToLower(location)
	var result map[string]interface{}
	for city, data := range weatherData {
		if strings.Contains(locationLower, city) {
			result = data
			break
		}
	}

	// Default weather if location not found
	if result == nil {
		result = map[string]interface{}{
			"temperature": 20,
			"unit":        "celsius",
			"conditions":  "clear",
			"humidity":    50,
			"wind_speed":  10,
			"wind_unit":   "km/h",
		}
	}

	// Add location to result
	result["location"] = location

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

// calculate evaluates a mathematical expression
func calculate(args map[string]interface{}) (string, error) {
	expression, ok := args["expression"].(string)
	if !ok {
		return "", fmt.Errorf("expression must be a string")
	}

	// Simple expression evaluator using Go's AST parser
	result, err := evalExpression(expression)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate expression: %w", err)
	}

	response := map[string]interface{}{
		"expression": expression,
		"result":     result,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return string(resultJSON), nil
}

// evalExpression evaluates a simple mathematical expression
func evalExpression(expr string) (float64, error) {
	// Clean up the expression
	expr = strings.ReplaceAll(expr, " ", "")

	// Use Go's AST parser to safely evaluate the expression
	fset := token.NewFileSet()
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("invalid expression: %w", err)
	}

	return evalNode(node, fset)
}

// evalNode evaluates an AST node
func evalNode(node ast.Expr, fset *token.FileSet) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		// Parse number literal
		val, err := strconv.ParseFloat(n.Value, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", n.Value)
		}
		return val, nil

	case *ast.BinaryExpr:
		// Evaluate binary expression
		left, err := evalNode(n.X, fset)
		if err != nil {
			return 0, err
		}
		right, err := evalNode(n.Y, fset)
		if err != nil {
			return 0, err
		}

		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("unsupported operator: %s", n.Op)
		}

	case *ast.ParenExpr:
		// Evaluate parenthesized expression
		return evalNode(n.X, fset)

	case *ast.UnaryExpr:
		// Handle unary minus
		val, err := evalNode(n.X, fset)
		if err != nil {
			return 0, err
		}
		if n.Op == token.SUB {
			return -val, nil
		}
		return val, nil

	default:
		return 0, fmt.Errorf("unsupported expression type")
	}
}

// getTime returns the current time in a specific timezone
func getTime(args map[string]interface{}) (string, error) {
	timezone, ok := args["timezone"].(string)
	if !ok {
		return "", fmt.Errorf("timezone must be a string")
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone '%s': %w", timezone, err)
	}

	now := time.Now().In(loc)

	result := map[string]interface{}{
		"timezone":    timezone,
		"time":        now.Format("15:04:05"),
		"date":        now.Format("2006-01-02"),
		"datetime":    now.Format(time.RFC3339),
		"day_of_week": now.Weekday().String(),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

// =============================================================================
// Tool Calling Flow
// =============================================================================

// runToolCallingDemo demonstrates the complete tool calling flow
func runToolCallingDemo(provider types.Provider, prompt string, verbose bool) error {
	ctx := context.Background()

	// Define available tools
	tools := []types.Tool{getWeatherTool, calculateTool, getTimeTool}

	fmt.Println("Available tools:")
	for _, tool := range tools {
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}
	fmt.Println()

	// Start conversation
	conversation := []types.ChatMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	fmt.Printf("User: %s\n\n", prompt)

	// Maximum number of turns to prevent infinite loops
	maxTurns := 10
	turn := 0

	for turn < maxTurns {
		turn++

		// Create request with tools
		options := types.GenerateOptions{
			Model:    provider.GetDefaultModel(),
			Messages: conversation,
			Tools:    tools,
			Stream:   true,
		}

		// Send request
		stream, err := provider.GenerateChatCompletion(ctx, options)
		if err != nil {
			return fmt.Errorf("failed to generate completion: %w", err)
		}

		// Collect response
		var response types.ChatMessage
		response.Role = "assistant"
		toolCallsMap := make(map[string]*types.ToolCall)

		for {
			chunk, err := stream.Next()
			if err == io.EOF || chunk.Done {
				break
			}
			if err != nil {
				return fmt.Errorf("failed to receive chunk: %w", err)
			}

			// Process choices from chunk
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.Role != "" {
					response.Role = delta.Role
				}
				if delta.Content != "" {
					response.Content += delta.Content
				}

				// Accumulate tool calls by ID
				for _, tc := range delta.ToolCalls {
					if tc.ID != "" {
						if _, exists := toolCallsMap[tc.ID]; !exists {
							toolCallsMap[tc.ID] = &types.ToolCall{
								ID:   tc.ID,
								Type: tc.Type,
								Function: types.ToolCallFunction{
									Name:      tc.Function.Name,
									Arguments: tc.Function.Arguments,
								},
							}
						} else {
							toolCallsMap[tc.ID].Function.Arguments += tc.Function.Arguments
						}
					} else if len(toolCallsMap) > 0 {
						// Continue accumulating to last tool call
						for _, existingTC := range toolCallsMap {
							existingTC.Function.Arguments += tc.Function.Arguments
							break
						}
					}
				}
			}

			// Also check convenience Content field
			if chunk.Content != "" {
				response.Content += chunk.Content
			}
		}

		// Convert tool calls map to slice
		for _, tc := range toolCallsMap {
			response.ToolCalls = append(response.ToolCalls, *tc)
		}

		// If no tool calls, we have the final response
		if len(response.ToolCalls) == 0 {
			fmt.Printf("Assistant: %s\n", response.Content)
			break
		}

		// Add assistant response with tool calls to conversation
		conversation = append(conversation, response)

		// Execute each tool call
		fmt.Printf("Assistant: [Making %d tool call(s)]\n\n", len(response.ToolCalls))

		for i, toolCall := range response.ToolCalls {
			if verbose {
				fmt.Printf("Tool Call %d/%d:\n", i+1, len(response.ToolCalls))
				fmt.Printf("  Name: %s\n", toolCall.Function.Name)
				fmt.Printf("  Arguments: %s\n", toolCall.Function.Arguments)
			} else {
				fmt.Printf("Executing: %s(%s)\n", toolCall.Function.Name, toolCall.Function.Arguments)
			}

			// Execute the tool
			result, err := executeToolCall(toolCall)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%s"}`, err.Error())
				fmt.Printf("  Error: %s\n", err)
			} else if verbose {
				fmt.Printf("  Result:\n%s\n", result)
			}

			// Add tool result to conversation
			conversation = append(conversation, types.ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}

		fmt.Println()
	}

	if turn >= maxTurns {
		return fmt.Errorf("reached maximum number of turns (%d)", maxTurns)
	}

	return nil
}

// =============================================================================
// Main
// =============================================================================

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	providerName := flag.String("provider", "anthropic", "Provider to use (openai, anthropic, etc.)")
	prompt := flag.String("prompt", "What's the weather in Tokyo and what's 15 * 23?", "Prompt to send to the AI")
	verbose := flag.Bool("verbose", false, "Show verbose output including tool arguments and results")

	flag.Parse()

	fmt.Println("========================================================================")
	fmt.Println("AI Provider Kit - Tool Calling Example")
	fmt.Println("========================================================================")
	fmt.Println()
	fmt.Println("This example demonstrates:")
	fmt.Println("  1. Defining tools the AI can call")
	fmt.Println("  2. Sending a request that requires tool use")
	fmt.Println("  3. Parsing tool call responses")
	fmt.Println("  4. Executing the tool locally")
	fmt.Println("  5. Sending tool results back to continue conversation")
	fmt.Println()

	// Check if config file exists
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Fatalf("Config file not found: %s\n\nPlease create a config.yaml file with your API keys.\nSee README.md for the expected format.", *configPath)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get provider configuration
	providerEntry := config.GetProviderEntry(cfg, *providerName)
	if providerEntry == nil {
		log.Fatalf("Provider '%s' not found in config", *providerName)
	}

	// Build provider config
	providerConfig := config.BuildProviderConfig(*providerName, providerEntry)

	// Create factory and register providers
	providerFactory := factory.NewProviderFactory()
	factory.RegisterDefaultProviders(providerFactory)

	// Create provider
	provider, err := providerFactory.CreateProvider(providerConfig.Type, providerConfig)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Configure the provider
	if err := provider.Configure(providerConfig); err != nil {
		log.Fatalf("Failed to configure provider: %v", err)
	}

	// Authenticate
	if providerConfig.APIKey != "" {
		authCfg := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: providerConfig.APIKey,
		}
		if err := provider.Authenticate(context.Background(), authCfg); err != nil {
			log.Fatalf("Failed to authenticate: %v", err)
		}
	}

	// Display provider info
	fmt.Printf("Provider: %s (%s)\n", provider.Name(), provider.Type())
	if providerConfig.DefaultModel != "" {
		fmt.Printf("Model: %s\n", providerConfig.DefaultModel)
	}
	fmt.Printf("Supports Tool Calling: %v\n", provider.SupportsToolCalling())
	fmt.Println()

	// Check if provider supports tool calling
	if !provider.SupportsToolCalling() {
		log.Fatalf("Provider %s does not support tool calling", provider.Name())
	}

	fmt.Println("------------------------------------------------------------------------")
	fmt.Println()

	// Run the demo
	if err := runToolCallingDemo(provider, *prompt, *verbose); err != nil {
		log.Fatalf("Demo failed: %v", err)
	}

	fmt.Println()
	fmt.Println("------------------------------------------------------------------------")
	fmt.Println("Demo Complete!")
	fmt.Println("========================================================================")
}
