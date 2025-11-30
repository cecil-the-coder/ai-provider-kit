package base

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ExampleUsage demonstrates how to use the ProviderFactory and TestProvider method
func ExampleUsage() {
	// Create a new provider factory
	factory := NewProviderFactory()

	// Register providers (in a real application, this would be done during initialization)
	factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
		// In a real implementation, this would create and return an OpenAI provider instance
		// For this example, we'll return nil since we don't have the actual provider implementation
		return nil
	})

	factory.RegisterProvider(types.ProviderTypeAnthropic, func(config types.ProviderConfig) types.Provider {
		// In a real implementation, this would create and return an Anthropic provider instance
		return nil
	})

	factory.RegisterProvider(types.ProviderTypeGemini, func(config types.ProviderConfig) types.Provider {
		// In a real implementation, this would create and return a Gemini provider instance
		return nil
	})

	// Example 1: Test an OpenAI provider with API key authentication
	fmt.Println("Testing OpenAI provider with API key...")
	openAIConfig := map[string]interface{}{
		"api_key":       "sk-your-openai-api-key",
		"base_url":      "https://api.openai.com/v1",
		"default_model": "gpt-4",
	}

	result, err := factory.TestProvider(context.Background(), "openai", openAIConfig)
	if err != nil {
		log.Printf("Error testing OpenAI provider: %v", err)
		return
	}

	printTestResult(result)

	// Example 2: Test an Anthropic provider
	fmt.Println("\nTesting Anthropic provider...")
	anthropicConfig := map[string]interface{}{
		"api_key":  "sk-ant-your-anthropic-api-key",
		"base_url": "https://api.anthropic.com",
	}

	result, err = factory.TestProvider(context.Background(), "anthropic", anthropicConfig)
	if err != nil {
		log.Printf("Error testing Anthropic provider: %v", err)
		return
	}

	printTestResult(result)

	// Example 3: Test a Gemini provider with OAuth authentication
	fmt.Println("\nTesting Gemini provider with OAuth...")
	geminiConfig := map[string]interface{}{
		"client_id":     "your-gemini-client-id",
		"client_secret": "your-gemini-client-secret",
		"auth_url":      "https://accounts.google.com/o/oauth2/auth",
		"token_url":     "https://oauth2.googleapis.com/token",
		"redirect_url":  "http://localhost:8080/callback",
		"scopes":        []string{"https://www.googleapis.com/auth/generative-language"},
	}

	result, err = factory.TestProvider(context.Background(), "gemini", geminiConfig)
	if err != nil {
		log.Printf("Error testing Gemini provider: %v", err)
		return
	}

	printTestResult(result)

	// Example 4: Test an invalid provider name
	fmt.Println("\nTesting invalid provider name...")
	result, err = factory.TestProvider(context.Background(), "invalid-provider", nil)
	if err != nil {
		log.Printf("Error testing invalid provider: %v", err)
		return
	}

	printTestResult(result)
}

// printTestResult prints a formatted test result
func printTestResult(result *types.TestResult) {
	fmt.Printf("Provider: %s\n", result.ProviderType)
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Phase: %s\n", result.Phase)
	fmt.Printf("Duration: %v\n", result.Duration)

	if result.IsSuccess() {
		fmt.Printf("✅ Test passed!\n")
		if result.ModelsCount > 0 {
			fmt.Printf("Models available: %d\n", result.ModelsCount)
		}

		// Print details
		if authMethod, exists := result.GetDetail("auth_method"); exists {
			fmt.Printf("Authentication method: %s\n", authMethod)
		}
		if connTest, exists := result.GetDetail("connectivity_test"); exists {
			fmt.Printf("Connectivity test: %s\n", connTest)
		}
	} else {
		fmt.Printf("❌ Test failed!\n")
		fmt.Printf("Error: %s\n", result.Error)

		if result.TestError != nil {
			fmt.Printf("Error type: %s\n", result.TestError.ErrorType)
			fmt.Printf("Retryable: %t\n", result.TestError.Retryable)
			if result.TestError.StatusCode > 0 {
				fmt.Printf("Status code: %d\n", result.TestError.StatusCode)
			}
		}

		// Print details
		if detail, exists := result.GetDetail("creation_error"); exists {
			fmt.Printf("Creation error: %s\n", detail)
		}
	}

	// Print timestamp
	fmt.Printf("Timestamp: %s\n\n", result.Timestamp.Format("2006-01-02 15:04:05"))
}

// ExampleUsageWithJSON demonstrates how to work with JSON serialization
func ExampleUsageWithJSON() {
	factory := NewProviderFactory()

	// Register a provider
	factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
		return nil // Mock provider
	})

	// Test provider
	config := map[string]interface{}{
		"api_key": "sk-test-key",
	}

	result, err := factory.TestProvider(context.Background(), "openai", config)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// Convert result to JSON
	jsonData, err := result.ToJSON()
	if err != nil {
		log.Printf("Error converting to JSON: %v", err)
		return
	}

	fmt.Printf("Test result as JSON:\n%s\n", string(jsonData))

	// Parse result from JSON (useful for storing/retrieving test results)
	parsedResult, err := types.TestResultFromJSON(jsonData)
	if err != nil {
		log.Printf("Error parsing from JSON: %v", err)
		return
	}

	fmt.Printf("Parsed result status: %s\n", parsedResult.Status)
}

// ExampleUsageWithTimeout demonstrates how to use context with timeout
func ExampleUsageWithTimeout() {
	factory := NewProviderFactory()

	// Register a provider
	factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
		return nil // Mock provider
	})

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := map[string]interface{}{
		"api_key": "sk-test-key",
	}

	result, err := factory.TestProvider(ctx, "openai", config)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	printTestResult(result)
}
