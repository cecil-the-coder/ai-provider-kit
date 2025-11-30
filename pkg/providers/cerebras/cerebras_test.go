package cerebras

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
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

// Test Description method
func TestCerebrasProvider_Description(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}
	provider := NewCerebrasProvider(config)

	desc := provider.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
	if desc != "Cerebras ultra-fast inference with multi-key failover and load balancing" {
		t.Errorf("Unexpected description: %s", desc)
	}
}

// Test SupportsResponsesAPI method
func TestCerebrasProvider_SupportsResponsesAPI(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}
	provider := NewCerebrasProvider(config)

	if provider.SupportsResponsesAPI() {
		t.Error("Expected SupportsResponsesAPI to return false for Cerebras")
	}
}

// Test GetMetrics method
func TestCerebrasProvider_GetMetrics(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}
	provider := NewCerebrasProvider(config)

	metrics := provider.GetMetrics()
	if metrics.RequestCount < 0 {
		t.Error("Expected non-negative request count")
	}
}

// Test Logout method
func TestCerebrasProvider_Logout(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}
	provider := NewCerebrasProvider(config)

	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated initially")
	}

	err := provider.Logout(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error during logout: %v", err)
	}

	if provider.IsAuthenticated() {
		t.Error("Expected provider to be unauthenticated after logout")
	}

	newConfig := provider.GetConfig()
	if newConfig.APIKey != "" {
		t.Error("Expected API key to be cleared after logout")
	}
}

// Test fetchModelsFromAPI with successful response
func TestCerebrasProvider_FetchModelsFromAPI_Success(t *testing.T) {
	mockResponse := `{
		"object": "list",
		"data": [
			{"id": "zai-glm-4.6", "object": "model", "created": 1234567890, "owned_by": "cerebras"},
			{"id": "llama3.1-8b", "object": "model", "created": 1234567890, "owned_by": "cerebras"}
		]
	}`

	server := createMockServer(mockResponse, http.StatusOK)
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	provider := NewCerebrasProvider(config)
	models, err := provider.fetchModelsFromAPI(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error fetching models: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}

	if models[0].ID != "zai-glm-4.6" {
		t.Errorf("Expected first model ID 'zai-glm-4.6', got %s", models[0].ID)
	}
}

// Test fetchModelsFromAPI with error response
func TestCerebrasProvider_FetchModelsFromAPI_Error(t *testing.T) {
	server := createMockServer(`{"error": "unauthorized"}`, http.StatusUnauthorized)
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "bad-key",
		BaseURL: server.URL,
	}

	provider := NewCerebrasProvider(config)
	_, err := provider.fetchModelsFromAPI(context.Background())

	if err == nil {
		t.Error("Expected error when fetching models with bad API key")
	}
}

// Test fetchModelsFromAPI without API key
func TestCerebrasProvider_FetchModelsFromAPI_NoAPIKey(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeCerebras,
	}

	provider := NewCerebrasProvider(config)
	_, err := provider.fetchModelsFromAPI(context.Background())

	if err == nil {
		t.Error("Expected error when fetching models without API key")
	}
}

// Test enrichModels
func TestCerebrasProvider_EnrichModels(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeCerebras,
		APIKey: "test-key",
	}
	provider := NewCerebrasProvider(config)

	models := []types.Model{
		{ID: "zai-glm-4.6", Provider: types.ProviderTypeCerebras},
		{ID: "llama3.1-8b", Provider: types.ProviderTypeCerebras},
		{ID: "unknown-model", Provider: types.ProviderTypeCerebras},
	}

	enriched := provider.enrichModels(models)

	if len(enriched) != 3 {
		t.Errorf("Expected 3 enriched models, got %d", len(enriched))
	}

	// Check ZAI model enrichment
	if enriched[0].Name != "ZAI GLM-4.6" {
		t.Errorf("Expected name 'ZAI GLM-4.6', got %s", enriched[0].Name)
	}
	if enriched[0].MaxTokens != 131072 {
		t.Errorf("Expected max tokens 131072, got %d", enriched[0].MaxTokens)
	}
	if !enriched[0].SupportsStreaming {
		t.Error("Expected ZAI model to support streaming")
	}
	if !enriched[0].SupportsToolCalling {
		t.Error("Expected ZAI model to support tool calling")
	}

	// Check unknown model gets defaults
	if enriched[2].Name != "unknown-model" {
		t.Errorf("Expected unknown model name to be 'unknown-model', got %s", enriched[2].Name)
	}
	if enriched[2].MaxTokens != 8192 {
		t.Errorf("Expected unknown model max tokens 8192, got %d", enriched[2].MaxTokens)
	}
}

// Test convertToCerebrasToolChoice with different modes
func TestConvertToCerebrasToolChoice(t *testing.T) {
	tests := []struct {
		name     string
		choice   *types.ToolChoice
		expected interface{}
	}{
		{
			name:     "Nil choice",
			choice:   nil,
			expected: nil,
		},
		{
			name:     "Auto mode",
			choice:   &types.ToolChoice{Mode: types.ToolChoiceAuto},
			expected: "auto",
		},
		{
			name:     "Required mode",
			choice:   &types.ToolChoice{Mode: types.ToolChoiceRequired},
			expected: "required",
		},
		{
			name:     "None mode",
			choice:   &types.ToolChoice{Mode: types.ToolChoiceNone},
			expected: "none",
		},
		{
			name:     "Specific mode",
			choice:   &types.ToolChoice{Mode: types.ToolChoiceSpecific, FunctionName: "get_weather"},
			expected: map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "get_weather"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToCerebrasToolChoice(tt.choice)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}

			switch expected := tt.expected.(type) {
			case string:
				if result != expected {
					t.Errorf("Expected %s, got %v", expected, result)
				}
			case map[string]interface{}:
				resultMap, ok := result.(map[string]interface{})
				if !ok {
					t.Errorf("Expected map, got %T", result)
					return
				}
				if resultMap["type"] != expected["type"] {
					t.Errorf("Expected type %v, got %v", expected["type"], resultMap["type"])
				}
			}
		})
	}
}

// Test streaming with real stream
func TestCerebrasRealStream_Next(t *testing.T) {
	mockStreamResponse := `data: {"id":"test","choices":[{"delta":{"content":"Hello"},"finish_reason":""}],"usage":{"total_tokens":0}}

data: {"id":"test","choices":[{"delta":{"content":" world"},"finish_reason":""}],"usage":{"total_tokens":0}}

data: {"id":"test","choices":[{"delta":{"content":"!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}

data: [DONE]
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockStreamResponse))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	provider := NewCerebrasProvider(config)
	options := types.GenerateOptions{
		Prompt: "Hello",
		Stream: true,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("Unexpected error creating stream: %v", err)
	}

	var content string
	for {
		chunk, err := stream.Next()
		// Add content even if there's an error (like io.EOF) as long as chunk has content
		if chunk.Content != "" {
			content += chunk.Content
		}
		if err != nil {
			break
		}
		if chunk.Done {
			break
		}
	}

	expected := "Hello world!"
	if content != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, content)
	}

	err = stream.Close()
	if err != nil {
		t.Errorf("Unexpected error closing stream: %v", err)
	}
}

// Test streaming with error
func TestCerebrasProvider_StreamingError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	provider := NewCerebrasProvider(config)
	options := types.GenerateOptions{
		Prompt: "Hello",
		Stream: true,
	}

	_, err := provider.GenerateChatCompletion(context.Background(), options)
	if err == nil {
		t.Error("Expected error when streaming with server error")
	}
}

// Test HealthCheck with failed request
func TestCerebrasProvider_HealthCheck_RequestError(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: "http://invalid-url-that-does-not-exist-12345.com",
	}

	provider := NewCerebrasProvider(config)
	err := provider.HealthCheck(context.Background())

	if err == nil {
		t.Error("Expected health check error with invalid URL")
	}
}

// Test HealthCheck with HTTP error
func TestCerebrasProvider_HealthCheck_HTTPError(t *testing.T) {
	server := createMockServer(`{"error": "unauthorized"}`, http.StatusUnauthorized)
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "bad-key",
		BaseURL: server.URL,
	}

	provider := NewCerebrasProvider(config)
	err := provider.HealthCheck(context.Background())

	if err == nil {
		t.Error("Expected health check error with HTTP error status")
	}
}

// Test GenerateChatCompletion with custom messages
func TestCerebrasProvider_GenerateChatCompletion_CustomMessages(t *testing.T) {
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
					"content": "Response to user message"
				},
				"finish_reason": "stop"
			}
		],
		"usage": {
			"prompt_tokens": 20,
			"completion_tokens": 10,
			"total_tokens": 30
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
		Messages: []types.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
		},
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("Unexpected error generating completion: %v", err)
	}

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Unexpected error getting chunk: %v", err)
	}

	if chunk.Content != "Response to user message" {
		t.Errorf("Expected specific response, got %s", chunk.Content)
	}
}

// Test GenerateChatCompletion with temperature and max tokens from config
func TestCerebrasProvider_GenerateChatCompletion_ConfigParams(t *testing.T) {
	mockResponse := `{
		"id": "test-id",
		"object": "chat.completion",
		"created": 1234567890,
		"model": "zai-glm-4.6",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "OK"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
	}`

	server := createMockServer(mockResponse, http.StatusOK)
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeCerebras,
		APIKey:  "test-key",
		BaseURL: server.URL,
		ProviderConfig: map[string]interface{}{
			"temperature": 0.8,
			"max_tokens":  512,
		},
	}

	provider := NewCerebrasProvider(config)
	options := types.GenerateOptions{
		Prompt: "Test",
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Unexpected error getting chunk: %v", err)
	}

	if chunk.Content != "OK" {
		t.Errorf("Expected 'OK', got %s", chunk.Content)
	}
}

// Test CerebrasStream with tool calls in chunk
func TestCerebrasStream_WithToolCalls(t *testing.T) {
	usage := types.Usage{
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
	}

	chunk := types.ChatCompletionChunk{
		Content: "",
		Done:    true,
		Usage:   usage,
		Choices: []types.ChatChoice{
			{
				Message: types.ChatMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: types.ToolCallFunction{
								Name:      "get_weather",
								Arguments: `{"location":"NYC"}`,
							},
						},
					},
				},
			},
		},
	}

	stream := &CerebrasStream{
		content: "",
		usage:   usage,
		model:   "test-model",
		closed:  false,
		chunk:   chunk,
	}

	result, err := stream.Next()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(result.Choices))
	}

	if len(result.Choices[0].Message.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(result.Choices[0].Message.ToolCalls))
	}
}

// TestCerebrasProvider_TestConnectivity tests the TestConnectivity method
func TestCerebrasProvider_TestConnectivity(t *testing.T) {
	t.Run("SuccessfulConnectivity", func(t *testing.T) {
		// Create a mock HTTP server that returns a valid models response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the request is for the models endpoint
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			// Verify authorization header
			authHeader := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer sk-test-key", authHeader)

			// Return a valid models response
			response := map[string]interface{}{
				"object": "list",
				"data": []interface{}{
					map[string]interface{}{
						"id":      "zai-glm-4.6",
						"object":  "model",
						"created": 1687882411,
						"owned_by": "cerebras",
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeCerebras,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}
		provider := NewCerebrasProvider(config)

		err := provider.TestConnectivity(context.Background())
		assert.NoError(t, err)
	})

	t.Run("NoAPIKeys", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeCerebras,
			// No API key configured
		}
		provider := NewCerebrasProvider(config)

		err := provider.TestConnectivity(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no API keys configured")
	})

	t.Run("InvalidAPIKey", func(t *testing.T) {
		// Create a mock server that returns unauthorized
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid API key",
					"type":    "invalid_request_error",
				},
			})
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeCerebras,
			APIKey:  "sk-invalid-key",
			BaseURL: server.URL,
		}
		provider := NewCerebrasProvider(config)

		err := provider.TestConnectivity(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid API key")
	})
}
