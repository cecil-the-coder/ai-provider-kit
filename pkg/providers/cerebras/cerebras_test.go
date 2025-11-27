package cerebras

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Mock server for testing
func createMockServer(response string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(response))
	}))
}

func TestNewCerebrasProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: "https://api.cerebras.ai/v1",
		ProviderConfig: map[string]interface{}{
			"temperature": 0.7,
		},
	}

	provider := NewCerebrasProvider(config)

	if provider.Name() != "Cerebras" {
		t.Errorf("Expected name 'Cerebras', got %s", provider.Name())
	}

	if provider.Type() != types.ProviderTypeCerebras {
		t.Errorf("Expected type %s, got %s", types.ProviderTypeCerebras, provider.Type())
	}

	if !provider.SupportsStreaming() {
		t.Error("Expected provider to support streaming")
	}

	if !provider.SupportsToolCalling() {
		t.Error("Expected provider to support tool calling")
	}
}

func TestCerebrasProvider_GetModels(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}

	provider := NewCerebrasProvider(config)
	models, err := provider.GetModels(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error getting models: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}

	// Check for ZAI model
	foundZAI := false
	for _, model := range models {
		if model.ID == "zai-glm-4.6" {
			foundZAI = true
			if !model.SupportsStreaming {
				t.Error("Expected ZAI model to support streaming")
			}
			if model.MaxTokens != 131072 {
				t.Errorf("Expected ZAI model max tokens 131072, got %d", model.MaxTokens)
			}
		}
	}

	if !foundZAI {
		t.Error("Expected to find zai-glm-4.6 model")
	}
}

func TestCerebrasProvider_GetDefaultModel(t *testing.T) {
	tests := []struct {
		name     string
		config   types.ProviderConfig
		expected string
	}{
		{
			name: "Default model when not specified",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeCerebras,
				APIKey: "test-key",
			},
			expected: "zai-glm-4.6",
		},
		{
			name: "Custom default model",
			config: types.ProviderConfig{
				Type:         types.ProviderTypeCerebras,
				APIKey:       "test-key",
				DefaultModel: "llama3.1-8b",
			},
			expected: "llama3.1-8b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCerebrasProvider(tt.config)
			if provider.GetDefaultModel() != tt.expected {
				t.Errorf("Expected default model %s, got %s", tt.expected, provider.GetDefaultModel())
			}
		})
	}
}

func TestCerebrasProvider_Authenticate(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "old-key",
	}

	provider := NewCerebrasProvider(config)

	// Test successful authentication
	authConfig := types.AuthConfig{
		Method:       types.AuthMethodAPIKey,
		APIKey:       "new-key",
		BaseURL:      "https://api.cerebras.ai/v1",
		DefaultModel: "llama3.1-70b",
	}

	err := provider.Authenticate(context.Background(), authConfig)
	if err != nil {
		t.Fatalf("Unexpected authentication error: %v", err)
	}

	newConfig := provider.GetConfig()
	if newConfig.APIKey != "new-key" {
		t.Errorf("Expected API key 'new-key', got %s", newConfig.APIKey)
	}

	// Test unsupported auth method
	authConfig.Method = types.AuthMethodOAuth
	err = provider.Authenticate(context.Background(), authConfig)
	if err == nil {
		t.Error("Expected error for unsupported auth method")
	}
}

func TestCerebrasProvider_IsAuthenticated(t *testing.T) {
	tests := []struct {
		name     string
		config   types.ProviderConfig
		expected bool
	}{
		{
			name: "Authenticated with API key",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeCerebras,
				APIKey: "test-key",
			},
			expected: true,
		},
		{
			name:     "Not authenticated without API key",
			config:   types.ProviderConfig{Type: types.ProviderTypeCerebras},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCerebrasProvider(tt.config)
			if provider.IsAuthenticated() != tt.expected {
				t.Errorf("Expected authenticated %v, got %v", tt.expected, provider.IsAuthenticated())
			}
		})
	}
}

func TestCerebrasProvider_MultipleAPIKeys(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "primary-key",
		ProviderConfig: map[string]interface{}{
			"api_keys": []string{"key1", "key2", "key3"},
		},
	}

	provider := NewCerebrasProvider(config)

	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated with multiple API keys")
	}

	// Test key rotation using the authHelper's KeyManager
	keys := []string{}
	for i := 0; i < 5; i++ {
		key, err := provider.authHelper.KeyManager.GetNextKey()
		if err != nil {
			t.Fatalf("Unexpected error getting next key: %v", err)
		}
		keys = append(keys, key)
	}

	// Should cycle through keys
	if len(keys) != 5 {
		t.Errorf("Expected 5 keys retrieved, got %d", len(keys))
	}
}

func TestCerebrasProvider_GenerateChatCompletion(t *testing.T) {
	// Mock server response
	mockResponse := `{
		"id": "test-id",
		"object": "chat.completion",
		"created": 1234567890,
		"model": "zai-glm-4.6",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello, world!"
				},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 5,
			"total_tokens": 15
		}
	}`

	server := createMockServer(mockResponse, http.StatusOK)
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	provider := NewCerebrasProvider(config)
	options := types.GenerateOptions{
		Prompt:      "Hello",
		Temperature: 0.5,
		MaxTokens:   100,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("Unexpected error generating completion: %v", err)
	}

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Unexpected error getting chunk: %v", err)
	}

	if chunk.Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got %s", chunk.Content)
	}

	if chunk.Usage.TotalTokens != 15 {
		t.Errorf("Expected total tokens 15, got %d", chunk.Usage.TotalTokens)
	}

	err = stream.Close()
	if err != nil {
		t.Errorf("Unexpected error closing stream: %v", err)
	}
}

func TestCerebrasProvider_GenerateChatCompletion_NoAPIKey(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeCerebras,
	}

	provider := NewCerebrasProvider(config)
	options := types.GenerateOptions{
		Prompt: "Hello",
	}

	_, err := provider.GenerateChatCompletion(context.Background(), options)
	if err == nil {
		t.Error("Expected error when no API key configured")
	}
}

func TestCerebrasProvider_GenerateChatCompletion_APIFailover(t *testing.T) {
	// Create a server that fails the first request but succeeds the second
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "invalid token"}`))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "test-id",
				"choices": [{"message": {"role": "assistant", "content": "Success"}}],
				"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
			}`))
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		BaseURL: server.URL,
		ProviderConfig: map[string]interface{}{
			"api_keys": []string{"bad-key", "good-key"},
		},
	}

	provider := NewCerebrasProvider(config)
	options := types.GenerateOptions{
		Prompt: "Hello",
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("Unexpected error with failover: %v", err)
	}

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Unexpected error getting chunk: %v", err)
	}

	if chunk.Content != "Success" {
		t.Errorf("Expected content 'Success', got %s", chunk.Content)
	}
}

func TestCerebrasProvider_HealthCheck(t *testing.T) {
	server := createMockServer(`{"data": []}`, http.StatusOK)
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	provider := NewCerebrasProvider(config)
	err := provider.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("Unexpected health check error: %v", err)
	}
}

func TestCerebrasProvider_HealthCheck_NoAPIKey(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeCerebras,
	}

	provider := NewCerebrasProvider(config)
	err := provider.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected health check error when no API key configured")
	}
}

func TestCerebrasStream(t *testing.T) {
	usage := types.Usage{
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
	}

	stream := &CerebrasStream{
		content: "Test content",
		usage:   usage,
		model:   "test-model",
		closed:  false,
	}

	// Test Next
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Unexpected error getting chunk: %v", err)
	}

	if chunk.Content != "Test content" {
		t.Errorf("Expected content 'Test content', got %s", chunk.Content)
	}

	if chunk.Usage.TotalTokens != 15 {
		t.Errorf("Expected total tokens 15, got %d", chunk.Usage.TotalTokens)
	}

	// Test that subsequent calls return empty chunks
	chunk, err = stream.Next()
	if err != nil {
		t.Fatalf("Unexpected error getting second chunk: %v", err)
	}

	if chunk.Content != "" {
		t.Errorf("Expected empty content on second call, got %s", chunk.Content)
	}

	// Test Close
	err = stream.Close()
	if err != nil {
		t.Errorf("Unexpected error closing stream: %v", err)
	}
}

func TestCerebrasProvider_Configure(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "initial-key",
	}

	provider := NewCerebrasProvider(config)

	// Update configuration
	newConfig := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "new-key",
		ProviderConfig: map[string]interface{}{
			"temperature": 0.8,
			"api_keys":    []string{"key1", "key2"},
		},
	}

	err := provider.Configure(newConfig)
	if err != nil {
		t.Fatalf("Unexpected error configuring provider: %v", err)
	}

	updatedConfig := provider.GetConfig()
	if updatedConfig.APIKey != "new-key" {
		t.Errorf("Expected API key 'new-key', got %s", updatedConfig.APIKey)
	}
}

func TestCerebrasProvider_Configure_InvalidType(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI, // Wrong type
		APIKey: "test-key",
	}

	provider := NewCerebrasProvider(types.ProviderConfig{
		Type: types.ProviderTypeCerebras,
	})

	err := provider.Configure(config)
	if err == nil {
		t.Error("Expected error when configuring with wrong provider type")
	}
}

func TestCerebrasProvider_InvokeServerTool(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}

	provider := NewCerebrasProvider(config)

	_, err := provider.InvokeServerTool(context.Background(), "test-tool", map[string]interface{}{"param": "value"})
	if err == nil {
		t.Error("Expected error for tool invocation (not implemented)")
	}
}

// Benchmark tests
func BenchmarkCerebrasProvider_GetNextAPIKey(b *testing.B) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
		ProviderConfig: map[string]interface{}{
			"api_keys": []string{"key1", "key2", "key3", "key4", "key5"},
		},
	}

	provider := NewCerebrasProvider(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.authHelper.KeyManager.GetNextKey()
	}
}

func BenchmarkCerebrasStream_Next(b *testing.B) {
	usage := types.Usage{
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
	}

	stream := &CerebrasStream{
		content: "Test content",
		usage:   usage,
		model:   "test-model",
		closed:  false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset stream state for each iteration
		stream.closed = false
		stream.index = 0
		_, _ = stream.Next()
	}
}
