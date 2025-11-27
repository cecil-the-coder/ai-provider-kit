package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
	fmt.Println("=== AI Provider Kit - Standardized Core API Demo ===")
	fmt.Println("This demo showcases all features of the standardized core API")
	fmt.Println("Note: This demo uses mock API keys and simulates API calls for demonstration purposes.")

	// Initialize the factory with core API support
	providerFactory := factory.NewProviderFactory()
	coreFactory := types.NewDefaultProviderFactoryExtensions(providerFactory)

	// Demo 1: Basic chat completion with StandardRequest
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DEMO 1: Basic Chat Completion")
	fmt.Println(strings.Repeat("=", 60))
	basicChatDemo(coreFactory)

	// Demo 2: Streaming chat completion
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DEMO 2: Streaming Chat Completion")
	fmt.Println(strings.Repeat("=", 60))
	streamingDemo(coreFactory)

	// Demo 3: Tool calling with standardized tools
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DEMO 3: Tool Calling")
	fmt.Println(strings.Repeat("=", 60))
	toolCallingDemo(coreFactory)

	// Demo 4: Provider capabilities and extensions
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DEMO 4: Provider Capabilities and Extensions")
	fmt.Println(strings.Repeat("=", 60))
	capabilitiesDemo(coreFactory)

	// Demo 5: Error handling and validation
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DEMO 5: Error Handling and Validation")
	fmt.Println(strings.Repeat("=", 60))
	errorHandlingDemo(coreFactory)

	// Demo 6: Advanced features (response format, metadata, timeout)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DEMO 6: Advanced Features")
	fmt.Println(strings.Repeat("=", 60))
	advancedFeaturesDemo(coreFactory)

	// Demo 7: Multiple provider comparison
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DEMO 7: Multiple Provider Comparison")
	fmt.Println(strings.Repeat("=", 60))
	multipleProviderDemo(coreFactory)

	// Demo 8: Request building patterns
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DEMO 8: Request Building Patterns")
	fmt.Println(strings.Repeat("=", 60))
	requestBuildingDemo()

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("DEMO COMPLETE")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("All standardized core API features have been demonstrated.")
	fmt.Println("Check the source code to see implementation details.")
}

// basicChatDemo demonstrates basic chat completion using the StandardRequest
func basicChatDemo(coreFactory types.ProviderFactoryExtensions) {
	fmt.Println("Creating OpenAI provider with standardized API...")

	// Create provider configuration
	providerConfig := types.ProviderConfig{
		Type:         types.ProviderTypeOpenAI,
		APIKey:       "sk-demo-key-for-testing", // Demo key
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4o-mini",
	}

	// Create core provider
	coreProvider, err := coreFactory.CreateCoreProvider(types.ProviderTypeOpenAI, providerConfig)
	if err != nil {
		log.Printf("❌ Failed to create core provider: %v", err)
		return
	}
	fmt.Printf("✅ Successfully created %s core provider\n", providerConfig.Type)

	// Create a standard request using the builder pattern
	fmt.Println("\nBuilding standard request...")
	request, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Explain quantum computing in one sentence."},
		}).
		WithModel("gpt-4o-mini").
		WithMaxTokens(100).
		WithTemperature(0.7).
		WithStop([]string{"\n\n"}).
		WithMetadata("request_id", "demo-001").
		WithMetadata("user_id", "demo-user").
		Build()

	if err != nil {
		log.Printf("❌ Failed to build request: %v", err)
		return
	}

	// Display request details
	fmt.Printf("✅ Standard request created:\n")
	fmt.Printf("   Model: %s\n", request.Model)
	fmt.Printf("   Max Tokens: %d\n", request.MaxTokens)
	fmt.Printf("   Temperature: %.1f\n", request.Temperature)
	fmt.Printf("   Messages: %d\n", len(request.Messages))
	fmt.Printf("   Stop sequences: %v\n", request.Stop)
	fmt.Printf("   Metadata keys: %v\n", getMetadataKeys(request.Metadata))

	// In a real scenario, you would call:
	fmt.Println("\n💡 Real usage would be:")
	fmt.Println("   response, err := coreProvider.GenerateStandardCompletion(context.Background(), *request)")
	fmt.Println("   if err != nil {")
	fmt.Println("       log.Printf('Failed to generate completion: %v', err)")
	fmt.Println("       return")
	fmt.Println("   }")
	fmt.Println("   fmt.Printf('Response: %s\\n', response.Choices[0].Message.Content)")

	// Validate the request with the provider
	fmt.Println("\nValidating request with provider...")
	if err := coreProvider.ValidateStandardRequest(*request); err != nil {
		log.Printf("⚠️  Request validation failed: %v", err)
	} else {
		fmt.Println("✅ Request validation passed")
	}
}

// streamingDemo demonstrates streaming chat completion
func streamingDemo(coreFactory types.ProviderFactoryExtensions) {
	fmt.Println("Creating Anthropic provider for streaming demo...")

	// Create provider configuration
	providerConfig := types.ProviderConfig{
		Type:         types.ProviderTypeAnthropic,
		APIKey:       "sk-ant-demo-key-for-testing", // Demo key
		BaseURL:      "https://api.anthropic.com",
		DefaultModel: "claude-3-5-haiku-20241022",
	}

	// Create core provider
	_, err := coreFactory.CreateCoreProvider(types.ProviderTypeAnthropic, providerConfig)
	if err != nil {
		log.Printf("❌ Failed to create core provider: %v", err)
		return
	}
	fmt.Printf("✅ Successfully created %s core provider\n", providerConfig.Type)

	// Create a streaming request
	fmt.Println("\nBuilding streaming request...")
	request, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{
			{Role: "user", Content: "Write a short poem about programming."},
		}).
		WithModel("claude-3-5-haiku-20241022").
		WithMaxTokens(200).
		WithTemperature(0.8).
		WithStreaming(true).
		WithTimeout(30 * time.Second).
		Build()

	if err != nil {
		log.Printf("❌ Failed to build streaming request: %v", err)
		return
	}

	fmt.Printf("✅ Streaming request created:\n")
	fmt.Printf("   Model: %s\n", request.Model)
	fmt.Printf("   Streaming: %t\n", request.Stream)
	fmt.Printf("   Timeout: %v\n", request.Timeout)

	// In a real scenario, you would call:
	fmt.Println("\n💡 Real streaming usage would be:")
	fmt.Println("   stream, err := coreProvider.GenerateStandardStream(context.Background(), *request)")
	fmt.Println("   if err != nil {")
	fmt.Println("       log.Printf('Failed to generate stream: %v', err)")
	fmt.Println("       return")
	fmt.Println("   }")
	fmt.Println("   defer stream.Close()")
	fmt.Println("   ")
	fmt.Println("   for {")
	fmt.Println("       chunk, err := stream.Next()")
	fmt.Println("       if err != nil {")
	fmt.Println("           break")
	fmt.Println("       }")
	fmt.Println("       if chunk != nil {")
	fmt.Println("           fmt.Print(chunk.Choices[0].Delta.Content)")
	fmt.Println("           if chunk.Done {")
	fmt.Println("               break")
	fmt.Println("           }")
	fmt.Println("       }")
	fmt.Println("   }")
}

// toolCallingDemo demonstrates tool calling capabilities
func toolCallingDemo(coreFactory types.ProviderFactoryExtensions) {
	fmt.Println("Setting up tool calling demonstration...")

	// Define multiple tools for comprehensive demonstration
	weatherTool := types.Tool{
		Name:        "get_weather",
		Description: "Get the current weather for a location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city and state, e.g. San Francisco, CA",
				},
				"units": map[string]interface{}{
					"type":        "string",
					"description": "Temperature units: celsius or fahrenheit",
					"enum":        []string{"celsius", "fahrenheit"},
					"default":     "celsius",
				},
			},
			"required": []string{"location"},
		},
	}

	calculatorTool := types.Tool{
		Name:        "calculate",
		Description: "Perform mathematical calculations",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "Mathematical expression to evaluate, e.g., '2 + 2 * 3'",
				},
			},
			"required": []string{"expression"},
		},
	}

	// Create a request with multiple tools
	fmt.Println("Building tool calling request...")
	request, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant with access to tools."},
			{Role: "user", Content: "What's the weather like in New York? Also, what's 15 * 8?"},
		}).
		WithModel("gpt-4o").
		WithMaxTokens(300).
		WithTemperature(0.2).
		WithTools([]types.Tool{weatherTool, calculatorTool}).
		WithToolChoice(&types.ToolChoice{
			Mode: types.ToolChoiceAuto,
		}).
		Build()

	if err != nil {
		log.Printf("❌ Failed to build tool calling request: %v", err)
		return
	}

	fmt.Printf("✅ Tool calling request created:\n")
	fmt.Printf("   Tools: %d\n", len(request.Tools))
	fmt.Printf("   Tool Choice: %s\n", request.ToolChoice.Mode)
	for i, tool := range request.Tools {
		fmt.Printf("   Tool %d: %s - %s\n", i+1, tool.Name, tool.Description)
	}

	// Demonstrate different tool choice modes
	fmt.Println("\n📋 Tool Choice Modes:")

	// Auto mode
	autoRequest, _ := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
		WithTools([]types.Tool{weatherTool}).
		WithToolChoice(&types.ToolChoice{Mode: types.ToolChoiceAuto}).
		Build()
	fmt.Printf("   Auto mode: %s\n", autoRequest.ToolChoice.Mode)

	// Required mode
	requiredRequest, _ := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
		WithTools([]types.Tool{weatherTool}).
		WithToolChoice(&types.ToolChoice{Mode: types.ToolChoiceRequired}).
		Build()
	fmt.Printf("   Required mode: %s\n", requiredRequest.ToolChoice.Mode)

	// None mode
	noneRequest, _ := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
		WithTools([]types.Tool{weatherTool}).
		WithToolChoice(&types.ToolChoice{Mode: types.ToolChoiceNone}).
		Build()
	fmt.Printf("   None mode: %s\n", noneRequest.ToolChoice.Mode)

	// Specific tool mode
	specificRequest, _ := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
		WithTools([]types.Tool{weatherTool, calculatorTool}).
		WithToolChoice(&types.ToolChoice{
			Mode:         types.ToolChoiceSpecific,
			FunctionName: "get_weather",
		}).
		Build()
	fmt.Printf("   Specific mode: %s -> %s\n", specificRequest.ToolChoice.Mode, specificRequest.ToolChoice.FunctionName)
}

// capabilitiesDemo shows provider capabilities and extensions
func capabilitiesDemo(coreFactory types.ProviderFactoryExtensions) {
	fmt.Println("Discovering provider capabilities...")

	// Get all supported providers for the core API
	supportedProviders := coreFactory.GetSupportedCoreProviders()
	fmt.Printf("Providers supporting Core API: %d\n", len(supportedProviders))

	if len(supportedProviders) == 0 {
		fmt.Println("⚠️  No providers are currently registered with the core API")
		fmt.Println("   This is expected in a demo environment without actual extensions")
		return
	}

	for _, providerType := range supportedProviders {
		fmt.Printf("\n📱 Provider: %s\n", providerType)
		capabilities := getProviderCapabilities(providerType)
		for _, capability := range capabilities {
			fmt.Printf("   ✓ %s\n", capability)
		}
	}

	// Demonstrate core API features
	fmt.Println("\n🔧 Core API Features:")
	fmt.Println("   ✓ Standardized request/response format")
	fmt.Println("   ✓ Unified streaming interface")
	fmt.Println("   ✓ Tool calling support")
	fmt.Println("   ✓ Provider extensions")
	fmt.Println("   ✓ Validation and error handling")
	fmt.Println("   ✓ Metadata and context support")
	fmt.Println("   ✓ Timeout and cancellation")
	fmt.Println("   ✓ Response format specification")
}

// errorHandlingDemo demonstrates validation and error handling
func errorHandlingDemo(coreFactory types.ProviderFactoryExtensions) {
	fmt.Println("Testing validation and error handling...")

	testCases := []struct {
		name   string
		build  func() (*types.StandardRequest, error)
		expect string
	}{
		{
			name: "Invalid temperature (> 2.0)",
			build: func() (*types.StandardRequest, error) {
				return types.NewCoreRequestBuilder().
					WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
					WithTemperature(3.0). // Invalid: > 2.0
					Build()
			},
			expect: "temperature must be between 0 and 2",
		},
		{
			name: "Negative max tokens",
			build: func() (*types.StandardRequest, error) {
				return types.NewCoreRequestBuilder().
					WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
					WithMaxTokens(-100). // Invalid: negative
					Build()
			},
			expect: "max_tokens must be non-negative",
		},
		{
			name: "No messages",
			build: func() (*types.StandardRequest, error) {
				return types.NewCoreRequestBuilder().
					WithModel("gpt-4").
					Build()
			},
			expect: "at least one message is required",
		},
		{
			name: "Tool choice without tools",
			build: func() (*types.StandardRequest, error) {
				return types.NewCoreRequestBuilder().
					WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
					WithToolChoice(&types.ToolChoice{Mode: types.ToolChoiceAuto}).
					Build()
			},
			expect: "tool_choice specified but no tools provided",
		},
		{
			name: "Specific tool without function name",
			build: func() (*types.StandardRequest, error) {
				tool := types.Tool{Name: "test", Description: "test tool", InputSchema: map[string]interface{}{}}
				return types.NewCoreRequestBuilder().
					WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
					WithTools([]types.Tool{tool}).
					WithToolChoice(&types.ToolChoice{Mode: types.ToolChoiceSpecific}).
					Build()
			},
			expect: "", // This might not error out, just test the structure
		},
	}

	for i, testCase := range testCases {
		fmt.Printf("\n%d. Testing: %s\n", i+1, testCase.name)
		request, err := testCase.build()

		if err != nil {
			if testCase.expect != "" && contains(err.Error(), testCase.expect) {
				fmt.Printf("   ✅ Caught expected error: %v\n", err)
			} else {
				fmt.Printf("   ⚠️  Unexpected error: %v\n", err)
			}
		} else {
			if testCase.expect == "" {
				fmt.Printf("   ✅ Request built successfully\n")
				if request != nil {
					fmt.Printf("      Model: %s, Messages: %d\n", request.Model, len(request.Messages))
				}
			} else {
				fmt.Printf("   ❌ Expected error but request was built successfully\n")
			}
		}
	}

	// Demonstrate error types
	fmt.Println("\n📋 Error Types:")
	fmt.Println("   • ValidationError - Request validation failures")
	fmt.Println("   • ProviderError - Provider-specific errors")
	fmt.Println("   • NetworkError - Network connectivity issues")
	fmt.Println("   • AuthenticationError - API key/auth problems")
	fmt.Println("   • RateLimitError - Rate limiting issues")
	fmt.Println("   • TimeoutError - Request timeout")
}

// advancedFeaturesDemo demonstrates advanced features like response format, metadata, etc.
func advancedFeaturesDemo(coreFactory types.ProviderFactoryExtensions) {
	fmt.Println("Demonstrating advanced features...")

	// Response format demo
	fmt.Println("\n📄 Response Format Specification:")

	jsonFormatRequest, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{
			{Role: "user", Content: "List 3 programming languages with their year of creation"},
		}).
		WithModel("gpt-4o").
		WithResponseFormat("json_object").
		WithMaxTokens(200).
		Build()

	if err != nil {
		log.Printf("❌ Failed to build JSON format request: %v", err)
	} else {
		fmt.Printf("   ✅ JSON format request created\n")
		fmt.Printf("      Response format: %s\n", jsonFormatRequest.ResponseFormat)
	}

	// Metadata demo
	fmt.Println("\n🏷️  Metadata Support:")

	metadataRequest, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
		WithModel("gpt-4o").
		WithMetadata("user_id", "user-12345").
		WithMetadata("session_id", "sess-abcdef").
		WithMetadata("request_type", "demo").
		WithMetadata("priority", "high").
		WithMetadata("tags", []string{"demo", "testing"}).
		WithMetadata("debug", true).
		Build()

	if err != nil {
		log.Printf("❌ Failed to build metadata request: %v", err)
	} else {
		fmt.Printf("   ✅ Metadata request created\n")
		fmt.Printf("      Metadata keys: %v\n", getMetadataKeys(metadataRequest.Metadata))
		for key, value := range metadataRequest.Metadata {
			fmt.Printf("      %s: %v\n", key, value)
		}
	}

	// Timeout demo
	fmt.Println("\n⏱️  Timeout Support:")

	shortTimeoutRequest, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Quick response"}}).
		WithModel("gpt-4o").
		WithTimeout(5 * time.Second).
		Build()

	if err != nil {
		log.Printf("❌ Failed to build timeout request: %v", err)
	} else {
		fmt.Printf("   ✅ Short timeout request: %v\n", shortTimeoutRequest.Timeout)
	}

	longTimeoutRequest, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Long response"}}).
		WithModel("gpt-4o").
		WithTimeout(2 * time.Minute).
		Build()

	if err != nil {
		log.Printf("❌ Failed to build long timeout request: %v", err)
	} else {
		fmt.Printf("   ✅ Long timeout request: %v\n", longTimeoutRequest.Timeout)
	}

	// Context demo
	fmt.Println("\n🌐 Context Support:")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	contextRequest, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Response with context"}}).
		WithModel("gpt-4o").
		WithContext(ctx).
		WithMetadata("trace_id", "trace-123").
		Build()

	if err != nil {
		log.Printf("❌ Failed to build context request: %v", err)
	} else {
		fmt.Printf("   ✅ Context request created\n")
		deadline, hasDeadline := contextRequest.Context.Deadline()
		fmt.Printf("      Context has deadline: %t\n", hasDeadline && deadline.After(time.Now()))
	}

	// Stop sequences demo
	fmt.Println("\n🛑 Stop Sequences:")

	stopRequest, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Tell me a story"}}).
		WithModel("gpt-4o").
		WithStop([]string{"\n\n", "THE END", "---"}).
		Build()

	if err != nil {
		log.Printf("❌ Failed to build stop sequences request: %v", err)
	} else {
		fmt.Printf("   ✅ Stop sequences request created\n")
		fmt.Printf("      Stop sequences: %v\n", stopRequest.Stop)
	}
}

// multipleProviderDemo demonstrates comparing multiple providers
func multipleProviderDemo(coreFactory types.ProviderFactoryExtensions) {
	fmt.Println("Comparing multiple providers...")

	// Define providers to compare
	providers := []struct {
		name     string
		provider types.ProviderType
		model    string
		config   types.ProviderConfig
	}{
		{
			name:     "OpenAI",
			provider: types.ProviderTypeOpenAI,
			model:    "gpt-4o-mini",
			config: types.ProviderConfig{
				Type:         types.ProviderTypeOpenAI,
				APIKey:       "sk-demo-key",
				BaseURL:      "https://api.openai.com/v1",
				DefaultModel: "gpt-4o-mini",
			},
		},
		{
			name:     "Anthropic",
			provider: types.ProviderTypeAnthropic,
			model:    "claude-3-5-haiku-20241022",
			config: types.ProviderConfig{
				Type:         types.ProviderTypeAnthropic,
				APIKey:       "sk-ant-demo-key",
				BaseURL:      "https://api.anthropic.com",
				DefaultModel: "claude-3-5-haiku-20241022",
			},
		},
		{
			name:     "Gemini",
			provider: types.ProviderTypeGemini,
			model:    "gemini-1.5-flash",
			config: types.ProviderConfig{
				Type:         types.ProviderTypeGemini,
				APIKey:       "gemini-demo-key",
				BaseURL:      "https://generativelanguage.googleapis.com",
				DefaultModel: "gemini-1.5-flash",
			},
		},
	}

	// Create identical requests for each provider
	baseMessage := []types.ChatMessage{
		{Role: "user", Content: "What is artificial intelligence? Explain in one sentence."},
	}

	for _, p := range providers {
		fmt.Printf("\n🔍 Testing %s Provider:\n", p.name)

		// Check if provider supports core API
		if !coreFactory.SupportsCoreAPI(p.provider) {
			fmt.Printf("   ⚠️  %s does not support core API (expected in demo)\n", p.name)
			continue
		}

		// Try to create provider
		coreProvider, err := coreFactory.CreateCoreProvider(p.provider, p.config)
		if err != nil {
			fmt.Printf("   ⚠️  Failed to create %s provider: %v\n", p.name, err)
			continue
		}

		fmt.Printf("   ✅ %s provider created successfully\n", p.name)

		// Create request
		request, err := types.NewCoreRequestBuilder().
			WithMessages(baseMessage).
			WithModel(p.model).
			WithMaxTokens(50).
			WithTemperature(0.7).
			WithMetadata("provider", p.name).
			Build()

		if err != nil {
			fmt.Printf("   ❌ Failed to build request: %v\n", err)
			continue
		}

		// Validate request
		if err := coreProvider.ValidateStandardRequest(*request); err != nil {
			fmt.Printf("   ⚠️  Request validation failed: %v\n", err)
		} else {
			fmt.Printf("   ✅ Request validated successfully\n")
		}

		fmt.Printf("   📋 Request details:\n")
		fmt.Printf("      Model: %s\n", request.Model)
		fmt.Printf("      Max tokens: %d\n", request.MaxTokens)
		fmt.Printf("      Temperature: %.1f\n", request.Temperature)
		fmt.Printf("      Capabilities: %v\n", getProviderCapabilities(p.provider))
	}
}

// requestBuildingDemo demonstrates various request building patterns
func requestBuildingDemo() {
	fmt.Println("Demonstrating request building patterns...")

	// Pattern 1: Simple request
	fmt.Println("\n1️⃣ Simple Request Pattern:")
	simpleRequest, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
		Build()
	if err != nil {
		log.Printf("❌ Simple request failed: %v", err)
	} else {
		fmt.Printf("   ✅ Simple request: %d messages\n", len(simpleRequest.Messages))
	}

	// Pattern 2: Chain building
	fmt.Println("\n2️⃣ Chain Building Pattern:")
	chainRequest := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
		WithModel("gpt-4o").
		WithMaxTokens(100).
		WithTemperature(0.7).
		WithMetadata("pattern", "chained")

	finalRequest, err := chainRequest.Build()
	if err != nil {
		log.Printf("❌ Chain request failed: %v", err)
	} else {
		fmt.Printf("   ✅ Chain request: model=%s, max_tokens=%d\n",
			finalRequest.Model, finalRequest.MaxTokens)
	}

	// Pattern 3: From legacy options
	fmt.Println("\n3️⃣ Legacy Options Conversion:")
	legacyOptions := types.GenerateOptions{
		Messages:    []types.ChatMessage{{Role: "user", Content: "Legacy conversion demo"}},
		Model:       "gpt-4o",
		MaxTokens:   150,
		Temperature: 0.5,
		Stream:      false,
		Timeout:     30 * time.Second,
		Metadata:    map[string]interface{}{"source": "legacy"},
	}

	legacyRequest, err := types.NewCoreRequestBuilder().
		FromGenerateOptions(legacyOptions).
		Build()
	if err != nil {
		log.Printf("❌ Legacy conversion failed: %v", err)
	} else {
		fmt.Printf("   ✅ Legacy conversion: timeout=%v, metadata_keys=%v\n",
			legacyRequest.Timeout, getMetadataKeys(legacyRequest.Metadata))
	}

	// Pattern 4: Complex request with all features
	fmt.Println("\n4️⃣ Complex Request Pattern:")
	complexTool := types.Tool{
		Name:        "complex_tool",
		Description: "A complex tool for demonstration",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input parameter",
				},
			},
			"required": []string{"input"},
		},
	}

	complexRequest, err := types.NewCoreRequestBuilder().
		WithMessages([]types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant with tools."},
			{Role: "user", Content: "Use the tool to process this data"},
		}).
		WithModel("gpt-4o").
		WithMaxTokens(500).
		WithTemperature(0.3).
		WithStop([]string{"\n\n"}).
		WithStreaming(true).
		WithTools([]types.Tool{complexTool}).
		WithToolChoice(&types.ToolChoice{Mode: types.ToolChoiceAuto}).
		WithResponseFormat("text").
		WithTimeout(60*time.Second).
		WithMetadata("complex", true).
		WithMetadata("demo_type", "comprehensive").
		Build()

	if err != nil {
		log.Printf("❌ Complex request failed: %v", err)
	} else {
		fmt.Printf("   ✅ Complex request created successfully\n")
		fmt.Printf("      Features used: messages(%d), tools(%d), streaming(%t), timeout(%v)\n",
			len(complexRequest.Messages), len(complexRequest.Tools),
			complexRequest.Stream, complexRequest.Timeout)
	}

	// Pattern 5: Request to legacy conversion
	fmt.Println("\n5️⃣ Back to Legacy Conversion:")
	backToLegacy := simpleRequest.ToGenerateOptions()
	fmt.Printf("   ✅ Converted back: model=%s, max_tokens=%d\n",
		backToLegacy.Model, backToLegacy.MaxTokens)
}

// Helper functions

// getProviderCapabilities returns mock capabilities for demo purposes
func getProviderCapabilities(providerType types.ProviderType) []string {
	capabilities := map[types.ProviderType][]string{
		types.ProviderTypeOpenAI: {
			"chat", "streaming", "tool_calling", "function_calling",
			"json_mode", "system_messages", "temperature", "top_p",
			"response_format", "parallel_tools", "vision",
		},
		types.ProviderTypeAnthropic: {
			"chat", "streaming", "tool_calling", "function_calling",
			"system_messages", "temperature", "top_p", "thinking_mode",
			"prompt_caching", "vision", "long_context",
		},
		types.ProviderTypeGemini: {
			"chat", "streaming", "tool_calling", "function_calling",
			"system_messages", "temperature", "top_p", "top_k",
			"vision", "code_execution", "multimodal",
		},
		types.ProviderTypeCerebras: {
			"chat", "streaming", "tool_calling", "function_calling",
			"system_messages", "temperature", "top_p", "high_speed",
		},
		types.ProviderTypeOpenRouter: {
			"chat", "streaming", "tool_calling", "function_calling",
			"model_routing", "fallback", "load_balancing", "cost_optimization",
		},
		types.ProviderTypeQwen: {
			"chat", "streaming", "tool_calling", "function_calling",
			"system_messages", "temperature", "top_p", "chinese_support",
			"code_generation", "reasoning",
		},
	}

	if caps, exists := capabilities[providerType]; exists {
		return caps
	}
	return []string{"chat", "streaming", "basic_tools"}
}

// getMetadataKeys extracts keys from metadata map
func getMetadataKeys(metadata map[string]interface{}) []string {
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	return keys
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(strings.Contains(s, substr))
}
