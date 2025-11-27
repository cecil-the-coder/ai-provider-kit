package qwen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewQwenProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeQwen,
		Name:         "test-qwen",
		APIKey:       "test-api-key",
		BaseURL:      "https://test.qwen.ai",
		DefaultModel: "qwen-turbo",
	}

	provider := NewQwenProvider(config)

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	if provider.Name() != "Qwen" {
		t.Errorf("Expected name 'Qwen', got '%s'", provider.Name())
	}

	if provider.Type() != types.ProviderTypeQwen {
		t.Errorf("Expected type %s, got %s", types.ProviderTypeQwen, provider.Type())
	}

	if provider.GetDefaultModel() != "qwen-turbo" {
		t.Errorf("Expected default model 'qwen-turbo', got '%s'", provider.GetDefaultModel())
	}
}

func TestQwenProvider_GetModels(t *testing.T) {
	provider := NewQwenProvider(types.ProviderConfig{
		Type:   types.ProviderTypeQwen,
		APIKey: "test-key",
	})

	ctx := context.Background()
	models, err := provider.GetModels(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(models) == 0 {
		t.Fatal("Expected at least one model")
	}

	// Check for known models
	expectedModels := []string{
		"qwen-turbo",
		"qwen-plus",
		"qwen-max",
		"qwen-coder-turbo",
		"qwen-coder-plus",
	}

	modelMap := make(map[string]bool)
	for _, model := range models {
		modelMap[model.ID] = true
	}

	for _, expectedID := range expectedModels {
		if !modelMap[expectedID] {
			t.Errorf("Expected model %s not found", expectedID)
		}
	}
}

func TestQwenProvider_Authenticate_APIKey(t *testing.T) {
	provider := NewQwenProvider(types.ProviderConfig{
		Type: types.ProviderTypeQwen,
	})

	ctx := context.Background()
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "test-api-key",
	}

	err := provider.Authenticate(ctx, authConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated")
	}
}

func TestQwenProvider_Authenticate_OAuth(t *testing.T) {
	// OAuth authentication is now handled through the OAuthKeyManager
	// This test is skipped as OAuth setup requires OAuthCredentials in ProviderConfig
	t.Skip("OAuth authentication is handled via OAuthCredentials in ProviderConfig")
}

func TestQwenProvider_Authenticate_InvalidMethod(t *testing.T) {
	provider := NewQwenProvider(types.ProviderConfig{
		Type: types.ProviderTypeQwen,
	})

	ctx := context.Background()
	authConfig := types.AuthConfig{
		Method: "invalid",
	}

	err := provider.Authenticate(ctx, authConfig)
	if err == nil {
		t.Error("Expected error for invalid auth method")
	}
}

func TestQwenProvider_Logout(t *testing.T) {
	provider := NewQwenProvider(types.ProviderConfig{
		Type: types.ProviderTypeQwen,
	})

	// First authenticate
	ctx := context.Background()
	authConfig := types.AuthConfig{
		Method: types.AuthMethodAPIKey,
		APIKey: "test-api-key",
	}

	err := provider.Authenticate(ctx, authConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated")
	}

	// Then logout
	err = provider.Logout(ctx)
	if err != nil {
		t.Fatalf("Expected no error during logout, got %v", err)
	}

	if provider.IsAuthenticated() {
		t.Error("Expected provider to be not authenticated after logout")
	}
}

func TestQwenProvider_Supports(t *testing.T) {
	provider := NewQwenProvider(types.ProviderConfig{
		Type: types.ProviderTypeQwen,
	})

	if !provider.SupportsToolCalling() {
		t.Error("Expected provider to support tool calling")
	}

	if !provider.SupportsStreaming() {
		t.Error("Expected provider to support streaming")
	}

	if provider.SupportsResponsesAPI() {
		t.Error("Expected provider to not support responses API")
	}

	if provider.GetToolFormat() != types.ToolFormatOpenAI {
		t.Errorf("Expected tool format %s, got %s", types.ToolFormatOpenAI, provider.GetToolFormat())
	}
}

func TestQwenProvider_GenerateChatCompletion_APIKey(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header 'Bearer test-api-key', got '%s'", auth)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}

		// Return mock response
		response := `{
			"id": "test-id",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "qwen-turbo",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello from Qwen!"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 5,
				"total_tokens": 15
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	provider := NewQwenProvider(types.ProviderConfig{
		Type:    types.ProviderTypeQwen,
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	})

	ctx := context.Background()
	options := types.GenerateOptions{
		Prompt: "Hello, Qwen!",
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Get the response
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error getting chunk, got %v", err)
	}

	if chunk.Content != "Hello from Qwen!" {
		t.Errorf("Expected content 'Hello from Qwen!', got '%s'", chunk.Content)
	}

	if chunk.Usage.PromptTokens != 10 {
		t.Errorf("Expected 10 prompt tokens, got %d", chunk.Usage.PromptTokens)
	}

	if chunk.Usage.CompletionTokens != 5 {
		t.Errorf("Expected 5 completion tokens, got %d", chunk.Usage.CompletionTokens)
	}

	if chunk.Usage.TotalTokens != 15 {
		t.Errorf("Expected 15 total tokens, got %d", chunk.Usage.TotalTokens)
	}

	// Close the stream
	err = stream.Close()
	if err != nil {
		t.Errorf("Expected no error closing stream, got %v", err)
	}
}

func TestQwenProvider_GenerateChatCompletion_NoAuth(t *testing.T) {
	provider := NewQwenProvider(types.ProviderConfig{
		Type: types.ProviderTypeQwen,
	})

	ctx := context.Background()
	options := types.GenerateOptions{
		Prompt: "Hello, Qwen!",
	}

	_, err := provider.GenerateChatCompletion(ctx, options)
	if err == nil {
		t.Error("Expected error when not authenticated")
	}

	if err.Error() != "no authentication method configured" {
		t.Errorf("Expected 'no authentication method configured', got '%s'", err.Error())
	}
}

func TestAPIKeyManager(t *testing.T) {
	manager := NewAPIKeyManager()

	// Test adding keys
	manager.addKey("key1")
	manager.addKey("key2")
	manager.addKey("key1") // Duplicate

	keys := manager.getKeys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	// Test getting current key
	currentKey := manager.getCurrentKey()
	if currentKey != "key1" {
		t.Errorf("Expected current key 'key1', got '%s'", currentKey)
	}

	// Test rotating keys
	nextKey := manager.rotateKey()
	if nextKey != "key2" {
		t.Errorf("Expected next key 'key2', got '%s'", nextKey)
	}

	// Test clearing keys
	manager.clear()
	keys = manager.getKeys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after clear, got %d", len(keys))
	}
}

func TestQwenStream(t *testing.T) {
	usage := &types.Usage{
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
	}

	stream := &QwenStream{
		content: "Test content",
		usage:   usage,
		model:   "qwen-turbo",
		closed:  false,
	}

	// Test getting chunk
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if chunk.Content != "Test content" {
		t.Errorf("Expected content 'Test content', got '%s'", chunk.Content)
	}

	if chunk.Model != "qwen-turbo" {
		t.Errorf("Expected model 'qwen-turbo', got '%s'", chunk.Model)
	}

	if chunk.Usage.PromptTokens != 10 {
		t.Errorf("Expected 10 prompt tokens, got %d", chunk.Usage.PromptTokens)
	}

	// Test getting chunk after it's done
	chunk, err = stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if chunk.Content != "" {
		t.Errorf("Expected empty content after done, got '%s'", chunk.Content)
	}

	// Test closing stream
	err = stream.Close()
	if err != nil {
		t.Errorf("Expected no error closing stream, got %v", err)
	}
}

func TestQwenProvider_TokenExpiration(t *testing.T) {
	// This test is removed because isTokenExpired is a private method.
	// Token expiration is now handled by the OAuthKeyManager internally.
	// The expiration logic is tested through IsAuthenticated() behavior.
	t.Skip("Skipping test for private method isTokenExpired - handled by OAuthKeyManager")
}

func TestQwenProvider_Configure(t *testing.T) {
	provider := NewQwenProvider(types.ProviderConfig{
		Type: types.ProviderTypeQwen,
	})

	// Test valid configuration
	config := types.ProviderConfig{
		Type:         types.ProviderTypeQwen,
		Name:         "test-qwen",
		APIKey:       "new-api-key",
		BaseURL:      "https://new.qwen.ai",
		DefaultModel: "qwen-plus",
	}

	err := provider.Configure(config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test invalid provider type
	invalidConfig := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "test-key",
	}

	err = provider.Configure(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid provider type")
	}
}

func TestMockStream(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "Chunk 1", Done: false},
		{Content: "Chunk 2", Done: true},
	}

	stream := &MockStream{
		chunks: chunks,
		index:  0,
	}

	// Test getting chunks
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if chunk.Content != "Chunk 1" {
		t.Errorf("Expected content 'Chunk 1', got '%s'", chunk.Content)
	}

	chunk, err = stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if chunk.Content != "Chunk 2" {
		t.Errorf("Expected content 'Chunk 2', got '%s'", chunk.Content)
	}

	// Test getting chunk after all are consumed
	chunk, err = stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if chunk.Content != "" {
		t.Errorf("Expected empty content after all chunks, got '%s'", chunk.Content)
	}

	// Test closing and resetting
	err = stream.Close()
	if err != nil {
		t.Errorf("Expected no error closing stream, got %v", err)
	}

	// After closing, we should be able to get chunks again
	chunk, err = stream.Next()
	if err != nil {
		t.Fatalf("Expected no error after close, got %v", err)
	}

	if chunk.Content != "Chunk 1" {
		t.Errorf("Expected content 'Chunk 1' after close, got '%s'", chunk.Content)
	}
}
