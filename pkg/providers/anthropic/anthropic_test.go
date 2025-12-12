package anthropic

import (
	"context"
	"fmt"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/auth"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/streaming"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewAnthropicProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeAnthropic,
		Name:         "anthropic",
		APIKey:       "test-key",
		BaseURL:      "https://api.anthropic.com",
		DefaultModel: "claude-3-5-sonnet-20241022",
	}

	provider := NewAnthropicProvider(config)

	if provider == nil {
		t.Fatal("Provider should not be nil")
	}

	if provider.Name() != "Anthropic" {
		t.Errorf("Expected name 'Anthropic', got '%s'", provider.Name())
	}

	if provider.Type() != types.ProviderTypeAnthropic {
		t.Errorf("Expected type '%s', got '%s'", types.ProviderTypeAnthropic, provider.Type())
	}

	if !provider.IsAuthenticated() {
		t.Error("Provider should be authenticated with API key")
	}
}

func TestAnthropicProviderWithDisplayName(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		Name:   "anthropic",
		APIKey: "test-key",
		ProviderConfig: map[string]interface{}{
			"display_name": "z.ai",
		},
	}

	provider := NewAnthropicProvider(config)

	if provider.Name() != "z.ai" {
		t.Errorf("Expected display name 'z.ai', got '%s'", provider.Name())
	}
}

func TestAnthropicProviderMultipleKeys(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		Name:   "anthropic",
		APIKey: "test-key-1",
		ProviderConfig: map[string]interface{}{
			"api_keys": []string{"test-key-2", "test-key-3"},
		},
	}

	provider := NewAnthropicProvider(config)

	if provider == nil {
		t.Fatal("Provider should not be nil")
	}

	if !provider.IsAuthenticated() {
		t.Error("Provider should be authenticated with API keys")
	}
}

func TestAnthropicProviderGetModels(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "test-key",
	}

	provider := NewAnthropicProvider(config)
	models, err := provider.GetModels(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error getting models: %v", err)
	}

	if len(models) == 0 {
		t.Error("Should return at least one model")
	}

	// Check for Claude models
	foundSonnet := false
	for _, model := range models {
		if model.ID == "claude-3-5-sonnet-20241022" {
			foundSonnet = true
			if !model.SupportsToolCalling {
				t.Error("Claude 3.5 Sonnet should support tool calling")
			}
			if !model.SupportsStreaming {
				t.Error("Claude 3.5 Sonnet should support streaming")
			}
			break
		}
	}

	if !foundSonnet {
		t.Error("Should include Claude 3.5 Sonnet model")
	}
}

func TestAnthropicProviderGetDefaultModel(t *testing.T) {
	tests := []struct {
		name     string
		config   types.ProviderConfig
		expected string
	}{
		{
			name: "default model",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeAnthropic,
				APIKey: "test-key",
			},
			expected: "claude-sonnet-4-5",
		},
		{
			name: "custom model",
			config: types.ProviderConfig{
				Type:         types.ProviderTypeAnthropic,
				APIKey:       "test-key",
				DefaultModel: "claude-3-opus-20240229",
			},
			expected: "claude-3-opus-20240229",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewAnthropicProvider(tt.config)
			if provider.GetDefaultModel() != tt.expected {
				t.Errorf("Expected default model '%s', got '%s'", tt.expected, provider.GetDefaultModel())
			}
		})
	}
}

func TestMultiKeyManager(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	manager, err := auth.NewAPIKeyManager("test", keys, nil)

	if err != nil {
		t.Fatalf("Unexpected error creating manager: %v", err)
	}
	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	// Test key rotation
	key1, err := manager.GetNextKey()
	if err != nil {
		t.Fatalf("Unexpected error getting next key: %v", err)
	}

	key2, err := manager.GetNextKey()
	if err != nil {
		t.Fatalf("Unexpected error getting next key: %v", err)
	}

	if key1 == key2 {
		t.Error("Keys should be different for round-robin")
	}
}

func TestSimplePromptHandling(t *testing.T) {
	options := types.GenerateOptions{
		Prompt:     "Create a function that adds two numbers",
		Context:    "This is for a math library",
		OutputFile: "calculator.go",
	}

	// Test that prompt is handled directly without the buildFullPrompt method
	if options.Prompt == "" {
		t.Error("Prompt should not be empty")
	}

	if !contains(options.Prompt, "Create a function that adds two numbers") {
		t.Error("Prompt should contain the main request")
	}

	// Context and outputFile are now handled directly by the provider
	if options.Context == "" {
		t.Error("Context should be preserved")
	}

	if options.OutputFile == "" {
		t.Error("OutputFile should be preserved")
	}
}

func TestPrepareRequest(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:         types.ProviderTypeAnthropic,
		DefaultModel: "claude-3-opus-20240229",
	})

	options := types.GenerateOptions{
		Prompt:     "Generate a JavaScript function",
		OutputFile: "test.js",
	}

	model := provider.GetDefaultModel()
	request := provider.prepareRequest(options, model, 4096)

	if request.Model != "claude-3-opus-20240229" {
		t.Errorf("Expected model 'claude-3-opus-20240229', got '%s'", request.Model)
	}

	if request.MaxTokens != 4096 {
		t.Errorf("Expected MaxTokens 4096, got %d", request.MaxTokens)
	}

	// System can be string or interface{} (for OAuth), so we need to check
	if systemStr, ok := request.System.(string); ok {
		if !contains(systemStr, "Claude Code") {
			t.Error("System prompt should contain the Claude Code identifier")
		}
		// Should NOT contain the expert programmer fallback when no system prompts provided
		if contains(systemStr, "expert programmer") {
			t.Error("System prompt should not contain expert programmer fallback")
		}
	}

	if len(request.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(request.Messages))
	}

	// Content can be string or interface{} (for content blocks)
	if contentStr, ok := request.Messages[0].Content.(string); ok {
		if contentStr != options.Prompt {
			t.Error("Message content should match the prompt")
		}
	}
}

func TestAnthropicProviderFeatures(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type:   types.ProviderTypeAnthropic,
		APIKey: "test-key",
	})

	if !provider.SupportsToolCalling() {
		t.Error("Anthropic provider should support tool calling")
	}

	if !provider.SupportsStreaming() {
		t.Error("Anthropic provider should support streaming")
	}

	if provider.SupportsResponsesAPI() {
		t.Error("Anthropic provider should not support Responses API")
	}

	if provider.GetToolFormat() != types.ToolFormatAnthropic {
		t.Errorf("Expected tool format %s, got %s", types.ToolFormatAnthropic, provider.GetToolFormat())
	}
}

func TestAnthropicProviderAuthentication(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type: types.ProviderTypeAnthropic,
	})

	// Test without API key
	if provider.IsAuthenticated() {
		t.Error("Provider should not be authenticated without API key")
	}

	// Test authentication
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "new-test-key",
	}

	err := provider.Authenticate(context.Background(), authConfig)
	if err != nil {
		t.Errorf("Unexpected authentication error: %v", err)
	}

	if !provider.IsAuthenticated() {
		t.Error("Provider should be authenticated after setting API key")
	}
}

func TestAnthropicProviderConfigure(t *testing.T) {
	provider := NewAnthropicProvider(types.ProviderConfig{
		Type: types.ProviderTypeAnthropic,
	})

	// Test invalid provider type
	invalidConfig := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}

	err := provider.Configure(invalidConfig)
	if err == nil {
		t.Error("Should return error for invalid provider type")
	}

	// Test valid configuration
	validConfig := types.ProviderConfig{
		Type:         types.ProviderTypeAnthropic,
		APIKey:       "test-key",
		BaseURL:      "https://api.anthropic.com",
		DefaultModel: "claude-3-5-sonnet-20241022",
		ProviderConfig: map[string]interface{}{
			"display_name": "Test Provider",
		},
	}

	err = provider.Configure(validConfig)
	if err != nil {
		t.Errorf("Unexpected configuration error: %v", err)
	}

	if !provider.IsAuthenticated() {
		t.Error("Provider should be authenticated after configuration")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}

func TestMultiKeyManagerFailover(t *testing.T) {
	keys := []string{"key1", "key2"}
	manager, err := auth.NewAPIKeyManager("test", keys, nil)

	if err != nil {
		t.Fatalf("Unexpected error creating manager: %v", err)
	}
	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	// Test successful operation
	ctx := context.Background()
	result, _, err := manager.ExecuteWithFailover(ctx, func(ctx context.Context, key string) (string, *types.Usage, error) {
		if key == "key1" {
			manager.ReportFailure(key, fmt.Errorf("simulated failure"))
			return "", nil, fmt.Errorf("simulated failure")
		}
		usage := &types.Usage{TotalTokens: 100}
		return "success", usage, nil
	})

	if err != nil {
		t.Errorf("Unexpected error in failover: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got '%s'", result)
	}
}

func TestMockStream(t *testing.T) {
	usage := types.Usage{TotalTokens: 5}
	chunks := []types.ChatCompletionChunk{
		{Content: "Hello", Done: false},
		{Content: " world", Done: false},
		{Content: "!", Done: true, Usage: usage},
	}

	stream := streaming.NewMockStream(chunks)

	// Test Next()
	chunk, err := stream.Next()
	if err != nil {
		t.Errorf("Unexpected error getting next chunk: %v", err)
	}
	if chunk.Content != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", chunk.Content)
	}

	// Test all chunks
	receivedChunks := 0
	for {
		chunk, err := stream.Next()
		if err != nil {
			// EOF is expected at the end of the stream
			if err.Error() == "EOF" {
				break
			}
			t.Errorf("Unexpected error in stream iteration: %v", err)
			break
		}
		if chunk.Done {
			break
		}
		receivedChunks++
		if receivedChunks > len(chunks) {
			t.Error("Stream should not produce more chunks than available")
			break
		}
	}

	// Test Close()
	err = stream.Close()
	if err != nil {
		t.Errorf("Unexpected error closing stream: %v", err)
	}

	// After close, should start from beginning
	chunk, err = stream.Next()
	if err != nil {
		t.Errorf("Unexpected error getting chunk after close: %v", err)
	}
	if chunk.Content != "Hello" {
		t.Errorf("Expected 'Hello' after close, got '%s'", chunk.Content)
	}
}
