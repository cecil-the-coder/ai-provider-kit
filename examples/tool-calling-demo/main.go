package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/toolvalidator"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Tool Definitions
// =============================================================================

var weatherTool = types.Tool{
	Name:        "get_weather",
	Description: "Get current weather for a location",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"location": map[string]interface{}{
				"type":        "string",
				"description": "City and state, e.g. San Francisco, CA",
			},
			"unit": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"celsius", "fahrenheit"},
				"description": "Temperature unit",
			},
		},
		"required": []string{"location"},
	},
}

var calculatorTool = types.Tool{
	Name:        "calculate",
	Description: "Perform mathematical calculations",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"add", "subtract", "multiply", "divide", "sqrt"},
				"description": "Mathematical operation",
			},
			"a": map[string]interface{}{
				"type":        "number",
				"description": "First operand",
			},
			"b": map[string]interface{}{
				"type":        "number",
				"description": "Second operand (optional for sqrt)",
			},
		},
		"required": []string{"operation", "a"},
	},
}

var stockPriceTool = types.Tool{
	Name:        "get_stock_price",
	Description: "Get current stock price for a symbol",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"symbol": map[string]interface{}{
				"type":        "string",
				"description": "Stock symbol (e.g., AAPL, GOOGL)",
			},
		},
		"required": []string{"symbol"},
	},
}

// =============================================================================
// Tool Execution Functions
// =============================================================================

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
	case "get_stock_price":
		return getStockPrice(args)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
}

func getWeather(args map[string]interface{}) (string, error) {
	location := args["location"].(string)
	unit := "fahrenheit"
	if u, ok := args["unit"].(string); ok {
		unit = u
	}

	// Mock weather data
	temp := 72.0
	if unit == "celsius" {
		temp = 22.0
	}

	result := map[string]interface{}{
		"location":    location,
		"temperature": temp,
		"unit":        unit,
		"conditions":  "partly cloudy",
		"humidity":    65,
		"wind_speed":  10,
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

func calculate(args map[string]interface{}) (string, error) {
	operation := args["operation"].(string)
	a := args["a"].(float64)

	var result float64
	switch operation {
	case "sqrt":
		if a < 0 {
			return "", fmt.Errorf("cannot calculate square root of negative number")
		}
		result = math.Sqrt(a)
	case "add":
		b := args["b"].(float64)
		result = a + b
	case "subtract":
		b := args["b"].(float64)
		result = a - b
	case "multiply":
		b := args["b"].(float64)
		result = a * b
	case "divide":
		b := args["b"].(float64)
		if b == 0 {
			return "", fmt.Errorf("division by zero")
		}
		result = a / b
	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}

	response := map[string]interface{}{
		"operation": operation,
		"result":    result,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return string(resultJSON), nil
}

func getStockPrice(args map[string]interface{}) (string, error) {
	symbol := args["symbol"].(string)

	// Mock stock data - different prices for different symbols
	prices := map[string]float64{
		"AAPL":  150.25,
		"GOOGL": 2750.50,
		"MSFT":  310.75,
		"TSLA":  245.30,
	}

	price, ok := prices[symbol]
	if !ok {
		price = 100.00 // Default price for unknown symbols
	}

	result := map[string]interface{}{
		"symbol":         symbol,
		"price":          price,
		"change":         2.5,
		"change_percent": 1.7,
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// processStreamingResponse processes a streaming response and collects tool calls
func processStreamingResponse(provider types.Provider, ctx context.Context, options types.GenerateOptions, verbose bool) (types.ChatMessage, error) {
	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		return types.ChatMessage{}, fmt.Errorf("failed to generate completion: %w", err)
	}

	var response types.ChatMessage
	response.Role = "assistant"                      // Default role for assistant responses
	toolCallsMap := make(map[string]*types.ToolCall) // Track tool calls by ID for accumulation

	for {
		chunk, err := stream.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			return types.ChatMessage{}, fmt.Errorf("failed to receive chunk: %w", err)
		}

		if verbose {
			fmt.Printf("[DEBUG] Chunk: Choices=%d, Content='%s', Done=%v\n",
				len(chunk.Choices), chunk.Content, chunk.Done)
			if len(chunk.Choices) > 0 {
				fmt.Printf("[DEBUG] Delta: Role='%s', Content='%s', ToolCalls=%d\n",
					chunk.Choices[0].Delta.Role, chunk.Choices[0].Delta.Content,
					len(chunk.Choices[0].Delta.ToolCalls))
			}
		}

		response, toolCallsMap = processChunk(response, chunk, toolCallsMap)
	}

	// Convert map to slice
	for _, tc := range toolCallsMap {
		response.ToolCalls = append(response.ToolCalls, *tc)
	}

	return response, nil
}

// processChunk processes a single chunk from the streaming response
func processChunk(response types.ChatMessage, chunk types.ChatCompletionChunk, toolCallsMap map[string]*types.ToolCall) (types.ChatMessage, map[string]*types.ToolCall) {
	// Access delta from choices
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
				// New tool call or tool call with ID
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
					// Accumulate arguments
					toolCallsMap[tc.ID].Function.Arguments += tc.Function.Arguments
				}
			} else if len(toolCallsMap) > 0 {
				// No ID means it's a continuation of the last tool call
				// Get the last tool call (assuming single tool call for now)
				for _, existingTC := range toolCallsMap {
					existingTC.Function.Arguments += tc.Function.Arguments
					break // Only update the first/last one
				}
			}
		}
	}
	// Also check convenience Content field
	if chunk.Content != "" {
		response.Content += chunk.Content
	}

	return response, toolCallsMap
}

// executeToolCallsAndExecute executes tool calls and updates the conversation
func executeToolCallsAndUpdateConversation(provider types.Provider, ctx context.Context, conversation []types.ChatMessage, response types.ChatMessage, options types.GenerateOptions, verbose bool) ([]types.ChatMessage, error) {
	// Execute all tool calls
	for _, toolCall := range response.ToolCalls {
		result, err := executeToolCall(toolCall)
		if err != nil {
			return conversation, fmt.Errorf("failed to execute tool: %w", err)
		}

		if verbose {
			fmt.Printf("Tool result:\n%s\n\n", result)
		}

		// Add tool result to conversation
		conversation = append(conversation, types.ChatMessage{
			Role:       "tool",
			Content:    result,
			ToolCallID: toolCall.ID,
		})
	}

	return conversation, nil
}

// printStreamingResponse prints a streaming response to stdout
func printStreamingResponse(provider types.Provider, ctx context.Context, options types.GenerateOptions) error {
	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		return fmt.Errorf("failed to generate completion: %w", err)
	}

	for {
		chunk, err := stream.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive chunk: %w", err)
		}
		if chunk.Content != "" {
			fmt.Print(chunk.Content)
		}
	}
	fmt.Println()

	return nil
}

// validateTools validates a slice of tools using the tool validator
func validateTools(tools []types.Tool, verbose bool) error {
	validator := toolvalidator.New(true)
	for _, tool := range tools {
		if err := validator.ValidateToolDefinition(tool); err != nil {
			return fmt.Errorf("invalid tool definition: %w", err)
		}
	}

	if verbose {
		fmt.Println("Tools validated successfully")
		for _, tool := range tools {
			fmt.Printf("Available tools: %s\n", tool.Name)
		}
		fmt.Println()
	}

	return nil
}

// =============================================================================
// Demo Scenarios
// =============================================================================

func DemoBasicToolCalling(provider types.Provider, verbose bool) error {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("DEMO 1: Basic Tool Calling")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Println("This demo shows a simple tool calling flow:")
	fmt.Println("1. Send a request with tools")
	fmt.Println("2. Model decides to call a tool")
	fmt.Println("3. Execute the tool locally")
	fmt.Println("4. Send results back")
	fmt.Println("5. Get final answer")
	fmt.Println()

	ctx := context.Background()
	tools := []types.Tool{weatherTool}

	// Validate tools first
	if err := validateTools(tools, verbose); err != nil {
		return err
	}

	// Initial conversation
	conversation := []types.ChatMessage{
		{
			Role:    "user",
			Content: "What's the weather like in San Francisco?",
		},
	}

	fmt.Println("User: What's the weather like in San Francisco?")
	fmt.Println()

	// First request
	options := types.GenerateOptions{
		Messages: conversation,
		Tools:    tools,
		Stream:   true,
	}

	// Get response with tool calls
	response, err := processStreamingResponse(provider, ctx, options, verbose)
	if err != nil {
		return fmt.Errorf("failed to get response: %w", err)
	}

	// Check if model wants to call a tool
	if len(response.ToolCalls) == 0 {
		fmt.Println("Assistant:", response.Content)
		fmt.Println()
		fmt.Println("Note: Model chose not to use tools (it may have cached knowledge)")
		return nil
	}

	// Add assistant response to conversation
	conversation = append(conversation, response)

	if verbose {
		fmt.Println("Assistant wants to call tool:")
		fmt.Printf("  Tool: %s\n", response.ToolCalls[0].Function.Name)
		fmt.Printf("  Arguments: %s\n\n", response.ToolCalls[0].Function.Arguments)
	} else {
		fmt.Printf("Assistant: [Calling tool: %s]\n\n", response.ToolCalls[0].Function.Name)
	}

	// Execute tool calls and update conversation
	conversation, err = executeToolCallsAndUpdateConversation(provider, ctx, conversation, response, options, verbose)
	if err != nil {
		return err
	}

	// Get final response
	options.Messages = conversation
	return printStreamingResponse(provider, ctx, options)
}

func DemoToolChoice(provider types.Provider, verbose bool) error {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("DEMO 2: ToolChoice Modes")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Println("This demo shows different ToolChoice modes:")
	fmt.Println("- auto: Model decides whether to use tools")
	fmt.Println("- required: Model must use a tool")
	fmt.Println("- none: Tools are disabled")
	fmt.Println("- specific: Force a specific tool")
	fmt.Println()

	ctx := context.Background()
	tools := []types.Tool{calculatorTool, weatherTool}

	// Validate tools
	if err := validateTools(tools, false); err != nil {
		return err
	}

	// Test different modes
	modes := []struct {
		name        string
		toolChoice  *types.ToolChoice
		prompt      string
		description string
	}{
		{
			name:        "Auto Mode",
			toolChoice:  &types.ToolChoice{Mode: types.ToolChoiceAuto},
			prompt:      "What is 25 + 17?",
			description: "Model decides whether to use calculator",
		},
		{
			name:        "Required Mode",
			toolChoice:  &types.ToolChoice{Mode: types.ToolChoiceRequired},
			prompt:      "Tell me about the weather",
			description: "Model must use a tool",
		},
		{
			name:        "Specific Mode",
			toolChoice:  &types.ToolChoice{Mode: types.ToolChoiceSpecific, FunctionName: "calculate"},
			prompt:      "What is 15 multiplied by 3?",
			description: "Force use of calculate tool",
		},
	}

	for i, mode := range modes {
		err := runToolChoiceMode(provider, ctx, tools, mode, i, len(modes), verbose)
		if err != nil {
			fmt.Printf("Error in mode %s: %v\n", mode.name, err)
		}
	}

	return nil
}

// runToolChoiceMode runs a single ToolChoice mode test
func runToolChoiceMode(provider types.Provider, ctx context.Context, tools []types.Tool, mode struct {
	name        string
	toolChoice  *types.ToolChoice
	prompt      string
	description string
}, index, total int, verbose bool) error {
	fmt.Printf("[%d/%d] %s\n", index+1, total, mode.name)
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("Description: %s\n", mode.description)
	fmt.Printf("Prompt: %s\n\n", mode.prompt)

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: mode.prompt},
		},
		Tools:      tools,
		ToolChoice: mode.toolChoice,
		Stream:     true,
	}

	// Get simplified response (we don't need complex tool call handling for this demo)
	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		fmt.Printf("Error: %v\n\n", err)
		return nil // Don't fail the entire demo
	}

	// Collect simple response
	var response types.ChatMessage
	response.Role = "assistant"
	for {
		chunk, err := stream.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			fmt.Printf("Error receiving chunk: %v\n\n", err)
			break
		}

		// Simple accumulation
		if chunk.Content != "" {
			response.Content += chunk.Content
		}
		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
			response.ToolCalls = append(response.ToolCalls, chunk.Choices[0].Delta.ToolCalls...)
		}
	}

	if len(response.ToolCalls) > 0 {
		fmt.Printf("Result: Model called tool '%s'\n", response.ToolCalls[0].Function.Name)
		if verbose {
			fmt.Printf("Arguments: %s\n", response.ToolCalls[0].Function.Arguments)
		}
	} else {
		fmt.Printf("Result: No tool calls (response: %s)\n", response.Content)
	}

	fmt.Println()
	return nil
}

func DemoParallelToolCalls(provider types.Provider, verbose bool) error {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("DEMO 3: Parallel Tool Calling")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Println("This demo shows handling multiple tool calls in one response.")
	fmt.Println("The model can call the same tool multiple times with different arguments.")
	fmt.Println()

	ctx := context.Background()
	tools := []types.Tool{weatherTool}

	// Validate tools
	if err := validateTools(tools, false); err != nil {
		return err
	}

	// Request that should trigger parallel calls
	conversation := []types.ChatMessage{
		{
			Role:    "user",
			Content: "What's the weather in New York, London, and Tokyo?",
		},
	}

	fmt.Println("User: What's the weather in New York, London, and Tokyo?")
	fmt.Println()

	options := types.GenerateOptions{
		Messages: conversation,
		Tools:    tools,
		Stream:   true,
	}

	// Get response with tool calls
	response, err := processStreamingResponse(provider, ctx, options, verbose)
	if err != nil {
		return fmt.Errorf("failed to get response: %w", err)
	}

	if len(response.ToolCalls) == 0 {
		fmt.Println("Assistant:", response.Content)
		fmt.Println()
		fmt.Println("Note: Model chose not to use tools")
		return nil
	}

	fmt.Printf("Assistant: [Making %d tool call(s)]\n\n", len(response.ToolCalls))

	// Add assistant response to conversation
	conversation = append(conversation, response)

	// Execute all tool calls with parallel display
	err = executeParallelToolCalls(provider, ctx, conversation, response.ToolCalls, options, verbose)
	if err != nil {
		return err
	}

	fmt.Println()

	// Get final response
	options.Messages = conversation
	return printStreamingResponse(provider, ctx, options)
}

// executeParallelToolCalls executes multiple tool calls with display formatting
func executeParallelToolCalls(provider types.Provider, ctx context.Context, conversation []types.ChatMessage, toolCalls []types.ToolCall, options types.GenerateOptions, verbose bool) error {
	for i, toolCall := range toolCalls {
		if verbose {
			fmt.Printf("Executing tool call %d/%d:\n", i+1, len(toolCalls))
			fmt.Printf("  Tool: %s\n", toolCall.Function.Name)
			fmt.Printf("  Arguments: %s\n", toolCall.Function.Arguments)
		} else {
			fmt.Printf("Executing: %s\n", toolCall.Function.Name)
		}

		result, err := executeToolCall(toolCall)
		if err != nil {
			return fmt.Errorf("failed to execute tool %d: %w", i+1, err)
		}

		if verbose {
			fmt.Printf("  Result:\n%s\n\n", result)
		}

		// Add tool result to conversation
		conversation = append(conversation, types.ChatMessage{
			Role:       "tool",
			Content:    result,
			ToolCallID: toolCall.ID,
		})
	}

	return nil
}

func DemoToolValidation(provider types.Provider, verbose bool) error {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("DEMO 4: Tool Validation")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Println("This demo shows the tool validation features:")
	fmt.Println("1. Validating tool definitions before use")
	fmt.Println("2. Validating tool calls from the model")
	fmt.Println("3. Handling validation errors gracefully")
	fmt.Println()

	// Create validator
	validator := toolvalidator.New(verbose)

	// Test valid tool
	fmt.Println("[1] Validating well-formed tool definition:")
	fmt.Printf("    Tool: %s\n", weatherTool.Name)
	if err := validator.ValidateToolDefinition(weatherTool); err != nil {
		fmt.Printf("    Result: INVALID - %v\n\n", err)
	} else {
		fmt.Printf("    Result: VALID\n\n")
	}

	// Test invalid tool - missing required fields
	invalidTool := types.Tool{
		Name: "invalid_tool",
		// Missing Description and InputSchema
	}

	fmt.Println("[2] Validating invalid tool definition (missing fields):")
	fmt.Printf("    Tool: %s\n", invalidTool.Name)
	if err := validator.ValidateToolDefinition(invalidTool); err != nil {
		fmt.Printf("    Result: INVALID - %v\n\n", err)
	} else {
		fmt.Printf("    Result: VALID\n\n")
	}

	// Test invalid tool - bad schema
	badSchemaTool := types.Tool{
		Name:        "bad_schema",
		Description: "Tool with invalid schema",
		InputSchema: map[string]interface{}{
			"type": "invalid_type", // Should be "object"
		},
	}

	fmt.Println("[3] Validating tool with invalid schema:")
	fmt.Printf("    Tool: %s\n", badSchemaTool.Name)
	if err := validator.ValidateToolDefinition(badSchemaTool); err != nil {
		fmt.Printf("    Result: INVALID - %v\n\n", err)
	} else {
		fmt.Printf("    Result: VALID\n\n")
	}

	// Test tool call validation
	validToolCall := types.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      "get_weather",
			Arguments: `{"location": "San Francisco", "unit": "celsius"}`,
		},
	}

	fmt.Println("[4] Validating tool call against definition:")
	fmt.Printf("    Tool: %s\n", validToolCall.Function.Name)
	fmt.Printf("    Arguments: %s\n", validToolCall.Function.Arguments)
	if err := validator.ValidateToolCall(weatherTool, validToolCall); err != nil {
		fmt.Printf("    Result: INVALID - %v\n\n", err)
	} else {
		fmt.Printf("    Result: VALID\n\n")
	}

	// Test tool call with missing required field
	invalidToolCall := types.ToolCall{
		ID:   "call_456",
		Type: "function",
		Function: types.ToolCallFunction{
			Name:      "get_weather",
			Arguments: `{"unit": "celsius"}`, // Missing required "location"
		},
	}

	fmt.Println("[5] Validating tool call with missing required field:")
	fmt.Printf("    Tool: %s\n", invalidToolCall.Function.Name)
	fmt.Printf("    Arguments: %s\n", invalidToolCall.Function.Arguments)
	if err := validator.ValidateToolCall(weatherTool, invalidToolCall); err != nil {
		fmt.Printf("    Result: INVALID - %v\n\n", err)
	} else {
		fmt.Printf("    Result: VALID\n\n")
	}

	fmt.Println("Validation demo complete!")

	return nil
}

func DemoMultiTurnConversation(provider types.Provider, verbose bool) error {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("DEMO 5: Multi-Turn Conversation with Tools")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Println("This demo shows a multi-turn conversation where the model uses")
	fmt.Println("tools multiple times to answer complex questions.")
	fmt.Println()

	ctx := context.Background()
	tools := []types.Tool{calculatorTool, stockPriceTool}

	// Validate tools
	if err := validateTools(tools, false); err != nil {
		return err
	}

	// Build a conversation
	conversation := []types.ChatMessage{
		{
			Role:    "user",
			Content: "What's the current price of AAPL stock?",
		},
	}

	turns := []string{
		"What's the current price of AAPL stock?",
		"Now calculate what 100 shares would cost at that price",
	}

	for turnNum, userMessage := range turns {
		conversation, err := runMultiTurnTurn(provider, ctx, conversation, tools, turnNum, userMessage, verbose)
		if err != nil {
			return fmt.Errorf("failed to run turn %d: %w", turnNum+1, err)
		}
	}

	return nil
}

// runMultiTurnTurn runs a single turn in the multi-turn conversation
func runMultiTurnTurn(provider types.Provider, ctx context.Context, conversation []types.ChatMessage, tools []types.Tool, turnNum int, userMessage string, verbose bool) ([]types.ChatMessage, error) {
	fmt.Printf("Turn %d\n", turnNum+1)
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("User: %s\n\n", userMessage)

	// Update conversation for turn 2+
	if turnNum > 0 {
		conversation = append(conversation, types.ChatMessage{
			Role:    "user",
			Content: userMessage,
		})
	}

	// Get response
	options := types.GenerateOptions{
		Messages: conversation,
		Tools:    tools,
		Stream:   true,
	}

	response, err := processStreamingResponse(provider, ctx, options, verbose)
	if err != nil {
		return conversation, fmt.Errorf("failed to get response: %w", err)
	}

	// Handle tool calls if any
	if len(response.ToolCalls) > 0 {
		return handleToolCallsInMultiTurn(provider, ctx, conversation, response, options, verbose)
	}

	fmt.Printf("Assistant: %s\n", response.Content)
	conversation = append(conversation, response)
	fmt.Println()

	return conversation, nil
}

// handleToolCallsInMultiTurn processes tool calls in multi-turn conversation
func handleToolCallsInMultiTurn(provider types.Provider, ctx context.Context, conversation []types.ChatMessage, response types.ChatMessage, options types.GenerateOptions, verbose bool) ([]types.ChatMessage, error) {
	conversation = append(conversation, response)

	for _, toolCall := range response.ToolCalls {
		if verbose {
			fmt.Printf("Assistant: [Calling %s with %s]\n", toolCall.Function.Name, toolCall.Function.Arguments)
		} else {
			fmt.Printf("Assistant: [Calling %s]\n", toolCall.Function.Name)
		}

		result, err := executeToolCall(toolCall)
		if err != nil {
			return conversation, fmt.Errorf("failed to execute tool: %w", err)
		}

		if verbose {
			fmt.Printf("Tool result: %s\n\n", result)
		}

		conversation = append(conversation, types.ChatMessage{
			Role:       "tool",
			Content:    result,
			ToolCallID: toolCall.ID,
		})
	}

	// Get final response after tool execution
	options.Messages = conversation
	var finalResponse types.ChatMessage
	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		return conversation, fmt.Errorf("failed to generate final completion: %w", err)
	}

	fmt.Print("Assistant: ")
	for {
		chunk, err := stream.Next()
		if err == io.EOF || chunk.Done {
			break
		}
		if err != nil {
			return conversation, fmt.Errorf("failed to receive chunk: %w", err)
		}
		if chunk.Content != "" {
			fmt.Print(chunk.Content)
		}
		finalResponse.Role = "assistant"
		finalResponse.Content += chunk.Content
	}
	fmt.Println()

	conversation = append(conversation, finalResponse)
	fmt.Println()

	return conversation, nil
}

// =============================================================================
// Main
// =============================================================================

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to config file (default: config.yaml in current directory)")
	providerName := flag.String("provider", "openai", "Provider to use (openai, anthropic, etc.)")
	demo := flag.String("demo", "all", "Demo to run (basic, toolchoice, parallel, validation, multi-turn, all)")
	verbose := flag.Bool("verbose", false, "Verbose output")

	flag.Parse()

	fmt.Println("=======================================================================")
	fmt.Println("AI Provider Kit - Tool Calling Demo")
	fmt.Println("=======================================================================")
	fmt.Println()
	fmt.Println("This demo showcases the complete tool calling functionality including:")
	fmt.Println("- Basic tool calling with multi-turn conversations")
	fmt.Println("- ToolChoice modes (auto, required, none, specific)")
	fmt.Println("- Parallel tool calling")
	fmt.Println("- Tool validation")
	fmt.Println("- Multi-turn conversations with tools")
	fmt.Println()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config from '%s': %v\nHint: Create a config.yaml file or use -config flag to specify the path", *configPath, err)
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

	// Create provider using factory
	provider, err := providerFactory.CreateProvider(providerConfig.Type, providerConfig)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Configure the provider
	if err := provider.Configure(providerConfig); err != nil {
		log.Fatalf("Failed to configure provider: %v", err)
	}

	// Authenticate if API key is provided
	if providerConfig.APIKey != "" {
		authCfg := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: providerConfig.APIKey,
		}
		if err := provider.Authenticate(context.Background(), authCfg); err != nil {
			log.Fatalf("Failed to authenticate provider: %v", err)
		}
	}

	fmt.Printf("Using provider: %s (%s)\n", provider.Name(), provider.Type())
	if providerConfig.DefaultModel != "" {
		fmt.Printf("Default model: %s\n", providerConfig.DefaultModel)
	}
	fmt.Printf("Supports tool calling: %v\n", provider.SupportsToolCalling())
	fmt.Println()

	if !provider.SupportsToolCalling() {
		log.Fatalf("Provider %s does not support tool calling", provider.Name())
	}

	// Run demos
	switch *demo {
	case "basic":
		if err := DemoBasicToolCalling(provider, *verbose); err != nil {
			log.Fatalf("Basic demo failed: %v", err)
		}
	case "toolchoice":
		if err := DemoToolChoice(provider, *verbose); err != nil {
			log.Fatalf("ToolChoice demo failed: %v", err)
		}
	case "parallel":
		if err := DemoParallelToolCalls(provider, *verbose); err != nil {
			log.Fatalf("Parallel demo failed: %v", err)
		}
	case "validation":
		if err := DemoToolValidation(provider, *verbose); err != nil {
			log.Fatalf("Validation demo failed: %v", err)
		}
	case "multi-turn":
		if err := DemoMultiTurnConversation(provider, *verbose); err != nil {
			log.Fatalf("Multi-turn demo failed: %v", err)
		}
	case "all":
		demos := []struct {
			name string
			fn   func(types.Provider, bool) error
		}{
			{"basic", DemoBasicToolCalling},
			{"toolchoice", DemoToolChoice},
			{"parallel", DemoParallelToolCalls},
			{"validation", DemoToolValidation},
			{"multi-turn", DemoMultiTurnConversation},
		}

		for _, d := range demos {
			if err := d.fn(provider, *verbose); err != nil {
				log.Printf("Demo '%s' failed: %v", d.name, err)
				// Continue with other demos
			}
		}
	default:
		log.Fatalf("Unknown demo: %s (use: basic, toolchoice, parallel, validation, multi-turn, all)", *demo)
	}

	fmt.Println()
	fmt.Println("=======================================================================")
	fmt.Println("Demo Complete!")
	fmt.Println("=======================================================================")
	fmt.Println()
}
