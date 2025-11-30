package base

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Test utility functions without needing to mock the full Provider interface
func TestParseProviderType(t *testing.T) {
	factory := NewProviderFactory()

	// Test provider name variations
	testCases := []struct {
		input    string
		expected types.ProviderType
	}{
		{"openai", types.ProviderTypeOpenAI},
		{"gpt", types.ProviderTypeOpenAI},
		{"OpenAI", types.ProviderTypeOpenAI},
		{"anthropic", types.ProviderTypeAnthropic},
		{"claude", types.ProviderTypeAnthropic},
		{"gemini", types.ProviderTypeGemini},
		{"cerebras", types.ProviderTypeCerebras},
		{"qwen", types.ProviderTypeQwen},
		{"openrouter", types.ProviderTypeOpenRouter},
		{"xai", types.ProviderTypexAI},
		{"x.ai", types.ProviderTypexAI},
		{"fireworks", types.ProviderTypeFireworks},
		{"deepseek", types.ProviderTypeDeepseek},
		{"mistral", types.ProviderTypeMistral},
		{"lmstudio", types.ProviderTypeLMStudio},
		{"llamacpp", types.ProviderTypeLlamaCpp},
		{"llama.cpp", types.ProviderTypeLlamaCpp},
		{"ollama", types.ProviderTypeOllama},
		{"synthetic", types.ProviderTypeSynthetic},
		{"racing", types.ProviderTypeRacing},
		{"fallback", types.ProviderTypeFallback},
		{"loadbalance", types.ProviderTypeLoadBalance},
		{"load-balance", types.ProviderTypeLoadBalance},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			providerType, err := factory.parseProviderType(tc.input)
			if err != nil {
				t.Fatalf("Unexpected error for %s: %v", tc.input, err)
			}
			if providerType != tc.expected {
				t.Errorf("Expected %s, got %s for input %s", tc.expected, providerType, tc.input)
			}
		})
	}

	// Test invalid provider
	_, err := factory.parseProviderType("invalid-provider")
	if err == nil {
		t.Error("Expected error for invalid provider")
	}
}

func TestConvertConfig(t *testing.T) {
	factory := NewProviderFactory()
	providerType := types.ProviderTypeOpenAI

	// Test ProviderConfig input
	config1 := types.ProviderConfig{
		Type:           providerType,
		ProviderConfig: map[string]interface{}{"api_key": "test"},
	}
	result1, err := factory.convertConfig(providerType, config1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result1.Type != config1.Type {
		t.Error("Config type mismatch")
	}

	// Test map[string]interface{} input
	config2 := map[string]interface{}{"api_key": "test"}
	result2, err := factory.convertConfig(providerType, config2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result2.Type != providerType {
		t.Error("Provider type mismatch")
	}

	// Test nil input
	result3, err := factory.convertConfig(providerType, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result3.Type != providerType {
		t.Error("Provider type mismatch for nil config")
	}

	// Test invalid input
	_, err = factory.convertConfig(providerType, "invalid")
	if err == nil {
		t.Error("Expected error for invalid config type")
	}
}

func TestExtractStatusCode(t *testing.T) {
	factory := NewProviderFactory()

	testCases := []struct {
		input    string
		expected int
	}{
		{"HTTP 404 Not Found", 404},
		{"status 401 unauthorized", 401},
		{"server returned 500 error", 500},
		{"403 forbidden", 403},
		{"response: 409 conflict", 409},
		{"Request failed with HTTP 422", 422},
		{"No status code here", 0},
		{"Invalid status: abc", 0},
		{"999 invalid code", 0}, // Not in HTTP range
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := factory.extractStatusCode(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %d, got %d for input %s", tc.expected, result, tc.input)
			}
		})
	}
}

func TestNewProviderFactory(t *testing.T) {
	factory := NewProviderFactory()
	if factory == nil {
		t.Fatal("Expected factory to be non-nil")
	}
	if len(factory.GetSupportedProviders()) != 0 {
		t.Fatalf("Expected no supported providers initially, got %d", len(factory.GetSupportedProviders()))
	}
}

func TestRegisterProvider(t *testing.T) {
	factory := NewProviderFactory()

	// Register a provider
	factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
		return nil // We don't need to implement the full provider for this test
	})

	supported := factory.GetSupportedProviders()
	if len(supported) != 1 {
		t.Fatalf("Expected 1 supported provider, got %d", len(supported))
	}
	if supported[0] != types.ProviderTypeOpenAI {
		t.Errorf("Expected ProviderTypeOpenAI, got %s", supported[0])
	}
}

func TestTestProvider_ConfigurationErrors(t *testing.T) {
	factory := NewProviderFactory()

	// Test invalid provider name
	result, err := factory.TestProvider(context.Background(), "invalid-provider", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}
	if result.IsSuccess() {
		t.Error("Expected failure for invalid provider name")
	}
	if result.Status != types.TestStatusConfigFailed {
		t.Errorf("Expected TestStatusConfigFailed, got %s", result.Status)
	}

	// Test invalid config
	factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
		return nil
	})
	result, err = factory.TestProvider(context.Background(), "openai", "invalid-config")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}
	if result.IsSuccess() {
		t.Error("Expected failure for invalid config")
	}
	if result.Status != types.TestStatusConfigFailed {
		t.Errorf("Expected TestStatusConfigFailed, got %s", result.Status)
	}

	// Test unsupported provider type (registered but factory returns nil)
	result, err = factory.TestProvider(context.Background(), "openai", map[string]interface{}{"api_key": "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}
	if result.IsSuccess() {
		t.Error("Expected failure for nil provider")
	}
	if result.Status != types.TestStatusConfigFailed {
		t.Errorf("Expected TestStatusConfigFailed, got %s", result.Status)
	}
}