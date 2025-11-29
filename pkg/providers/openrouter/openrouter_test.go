package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewOpenRouterProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeOpenRouter,
		APIKey:       "test-key",
		BaseURL:      "https://openrouter.ai/api",
		DefaultModel: "qwen/qwen3-coder",
		ProviderConfig: map[string]interface{}{
			"site_url":       "https://example.com",
			"site_name":      "Test Site",
			"models":         []string{"model1", "model2"},
			"model_strategy": "failover",
			"free_only":      false,
		},
	}

	provider := NewOpenRouterProvider(config)

	if provider.Name() != "OpenRouter" {
		t.Errorf("Expected name 'OpenRouter', got '%s'", provider.Name())
	}

	if provider.Type() != types.ProviderTypeOpenRouter {
		t.Errorf("Expected type '%s', got '%s'", types.ProviderTypeOpenRouter, provider.Type())
	}

	if !provider.SupportsToolCalling() {
		t.Error("Expected provider to support tool calling")
	}

	if !provider.SupportsStreaming() {
		t.Error("Expected provider to support streaming")
	}

	if provider.SupportsResponsesAPI() {
		t.Error("Expected provider to not support responses API")
	}
}

func TestGetModels(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	ctx := context.Background()
	models, err := provider.GetModels(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	// Check if common models are included
	modelMap := make(map[string]bool)
	for _, model := range models {
		modelMap[model.ID] = true
	}

	expectedModels := []string{
		"qwen/qwen3-coder",
		"anthropic/claude-3.5-sonnet",
		"openai/gpt-4o",
	}

	for _, expectedModel := range expectedModels {
		if !modelMap[expectedModel] {
			t.Errorf("Expected model '%s' not found", expectedModel)
		}
	}
}

func TestGetDefaultModel(t *testing.T) {
	tests := []struct {
		name          string
		config        types.ProviderConfig
		expectedModel string
	}{
		{
			name: "Default model from config",
			config: types.ProviderConfig{
				Type:         types.ProviderTypeOpenRouter,
				DefaultModel: "custom-model",
			},
			expectedModel: "custom-model",
		},
		{
			name: "Default model from provider config",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeOpenRouter,
				APIKey: "test-key",
				ProviderConfig: map[string]interface{}{
					"model": "provider-custom-model",
				},
			},
			expectedModel: "provider-custom-model",
		},
		{
			name: "Default fallback model",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeOpenRouter,
				APIKey: "test-key",
			},
			expectedModel: "qwen/qwen3-coder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenRouterProvider(tt.config)
			model := provider.GetDefaultModel()
			if model != tt.expectedModel {
				t.Errorf("Expected model '%s', got '%s'", tt.expectedModel, model)
			}
		})
	}
}

func TestModelSelector(t *testing.T) {
	models := []string{"model1", "model2", "model3"}

	tests := []struct {
		name     string
		strategy string
	}{
		{"Failover strategy", "failover"},
		{"Round-robin strategy", "round-robin"},
		{"Random strategy", "random"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := NewModelSelector(models, tt.strategy)

			// Test that we can select models
			for i := 0; i < 10; i++ {
				model, err := selector.SelectModel()
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}

				found := false
				for _, expectedModel := range models {
					if model == expectedModel {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Selected model '%s' not in expected models", model)
				}
			}
		})
	}
}

func TestAPIKeyManager(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	manager := NewAPIKeyManager("TestProvider", keys)

	if manager == nil {
		t.Fatal("Expected non-nil APIKeyManager")
	}

	// Test GetCurrentKey
	currentKey := manager.GetCurrentKey()
	if currentKey == "" {
		t.Error("Expected non-empty current key")
	}

	// Test GetNextKey
	for i := 0; i < 10; i++ {
		key, err := manager.GetNextKey()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if key == "" {
			t.Error("Expected non-empty key")
		}
	}

	// Test status
	status := manager.GetStatus()
	if status["provider"] != "TestProvider" {
		t.Errorf("Expected provider 'TestProvider', got %v", status["provider"])
	}
	if status["total_keys"] != 3 {
		t.Errorf("Expected 3 total keys, got %v", status["total_keys"])
	}
}

func TestConfigure(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "initial-key",
	})

	newConfig := types.ProviderConfig{
		Type:         types.ProviderTypeOpenRouter,
		APIKey:       "new-key",
		BaseURL:      "https://new.example.com",
		DefaultModel: "new-model",
		ProviderConfig: map[string]interface{}{
			"site_url":       "https://new-site.com",
			"site_name":      "New Site",
			"models":         []string{"new-model1", "new-model2"},
			"model_strategy": "round-robin",
			"free_only":      true,
		},
	}

	err := provider.Configure(newConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if provider.GetDefaultModel() != "new-model" {
		t.Errorf("Expected default model 'new-model', got '%s'", provider.GetDefaultModel())
	}
}

func TestDescription(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	desc := provider.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
	if !strings.Contains(desc, "OpenRouter") {
		t.Error("Expected description to contain 'OpenRouter'")
	}
}

func TestIsAuthenticated(t *testing.T) {
	tests := []struct {
		name     string
		config   types.ProviderConfig
		expected bool
	}{
		{
			name: "Authenticated with API key",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeOpenRouter,
				APIKey: "test-key",
			},
			expected: true,
		},
		{
			name: "Not authenticated without API key",
			config: types.ProviderConfig{
				Type: types.ProviderTypeOpenRouter,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewOpenRouterProvider(tt.config)
			if provider.IsAuthenticated() != tt.expected {
				t.Errorf("Expected IsAuthenticated() to be %v, got %v", tt.expected, provider.IsAuthenticated())
			}
		})
	}
}

func TestAuthenticate(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type: types.ProviderTypeOpenRouter,
	})

	authConfig := types.AuthConfig{
		Method:       types.AuthMethodAPIKey,
		APIKey:       "new-api-key",
		BaseURL:      "https://test.openrouter.ai/api",
		DefaultModel: "test-model",
	}

	err := provider.Authenticate(context.Background(), authConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test invalid auth method
	invalidConfig := types.AuthConfig{
		Method: types.AuthMethodOAuth,
	}
	err = provider.Authenticate(context.Background(), invalidConfig)
	if err == nil {
		t.Error("Expected error for OAuth method, got nil")
	}
}

func TestLogout(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated initially")
	}

	err := provider.Logout(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if provider.IsAuthenticated() {
		t.Error("Expected provider to not be authenticated after logout")
	}
}

func TestInvokeServerTool(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	_, err := provider.InvokeServerTool(context.Background(), "test-tool", nil)
	if err == nil {
		t.Error("Expected error for InvokeServerTool, got nil")
	}
}

func TestGetLastUsedModel(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	// Initially empty
	if provider.GetLastUsedModel() != "" {
		t.Errorf("Expected empty last used model, got '%s'", provider.GetLastUsedModel())
	}

	// Prepare a request to set the last used model
	options := types.GenerateOptions{
		Model:  "test-model",
		Prompt: "Hello",
	}
	_, err := provider.prepareRequest(options)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if provider.GetLastUsedModel() != "test-model" {
		t.Errorf("Expected last used model 'test-model', got '%s'", provider.GetLastUsedModel())
	}
}

func TestGetMetrics(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	metrics := provider.GetMetrics()
	// Just verify we can get metrics without error
	_ = metrics
}

func TestHealthCheck(t *testing.T) {
	// Create a mock server for health check
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_free_tier": false,
		})
	}))
	defer server.Close()

	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:    types.ProviderTypeOpenRouter,
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	err := provider.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("Expected no error from health check, got %v", err)
	}

	// Test unauthenticated
	provider2 := NewOpenRouterProvider(types.ProviderConfig{
		Type: types.ProviderTypeOpenRouter,
	})
	err = provider2.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected error for unauthenticated health check")
	}
}

func TestGetTrackedRateLimits(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	// Should return nil for model with no tracking
	limits := provider.GetTrackedRateLimits("test-model")
	if limits != nil {
		t.Error("Expected nil for untracked model")
	}
}

func TestGetCombinedRateLimits(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_free_tier": false,
			"usage":        0.0,
		})
	}))
	defer server.Close()

	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:    types.ProviderTypeOpenRouter,
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	apiLimits, headerInfo, err := provider.GetCombinedRateLimits(context.Background(), "test-model")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if apiLimits == nil {
		t.Error("Expected non-nil API limits")
	}
	// Header info may be nil since we haven't made requests yet
	_ = headerInfo
}

func TestModelSelectorRecordFailureAndReset(t *testing.T) {
	models := []string{"model1", "model2", "model3"}
	selector := NewModelSelector(models, "failover")

	// Record failures
	selector.RecordFailure("model1")
	selector.RecordFailure("model2")

	// Next selection should be model3
	model, err := selector.SelectModel()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if model != "model3" {
		t.Errorf("Expected model3, got %s", model)
	}

	// Reset should clear failures
	selector.Reset()
	model, err = selector.SelectModel()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if model != "model1" {
		t.Errorf("Expected model1 after reset, got %s", model)
	}
}

func TestAPIKeyManagerReportSuccessAndFailure(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	manager := NewAPIKeyManager("TestProvider", keys)

	// Report success
	manager.ReportSuccess("key1")
	status := manager.GetStatus()
	keyStatuses := status["keys"].([]map[string]interface{})
	if keyStatuses[0]["failure_count"] != 0 {
		t.Error("Expected failure count to be 0 after success")
	}

	// Report failure
	manager.ReportFailure("key1", fmt.Errorf("test error"))
	status = manager.GetStatus()
	keyStatuses = status["keys"].([]map[string]interface{})
	if keyStatuses[0]["failure_count"] != 1 {
		t.Error("Expected failure count to be 1 after failure")
	}
}

func TestAPIKeyManagerExecuteWithFailover(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	manager := NewAPIKeyManager("TestProvider", keys)

	// Test successful execution
	callCount := 0
	result, usage, err := manager.ExecuteWithFailover(func(apiKey string) (string, *types.Usage, error) {
		callCount++
		return "success", &types.Usage{TotalTokens: 100}, nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("Expected result 'success', got '%s'", result)
	}
	if usage.TotalTokens != 100 {
		t.Errorf("Expected total tokens 100, got %d", usage.TotalTokens)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// Test failover scenario
	attemptNum := 0
	_, _, err = manager.ExecuteWithFailover(func(apiKey string) (string, *types.Usage, error) {
		attemptNum++
		if attemptNum < 3 {
			return "", nil, fmt.Errorf("temporary error")
		}
		return "success after failover", &types.Usage{}, nil
	})

	if err != nil {
		t.Fatalf("Expected no error after failover, got %v", err)
	}

	// Test all keys failing
	_, _, err = manager.ExecuteWithFailover(func(apiKey string) (string, *types.Usage, error) {
		return "", nil, fmt.Errorf("all keys failed")
	})

	if err == nil {
		t.Error("Expected error when all keys fail")
	}
}

func TestConfigureWithInvalidType(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	invalidConfig := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "new-key",
	}

	err := provider.Configure(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid provider type")
	}
}

func TestPrepareRequestWithMessages(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	messages := []types.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	options := types.GenerateOptions{
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   100,
	}

	req, err := provider.prepareRequest(options)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(req.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(req.Messages))
	}
	if req.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", req.Temperature)
	}
	if req.MaxTokens != 100 {
		t.Errorf("Expected max tokens 100, got %d", req.MaxTokens)
	}
}

func TestPrepareRequestWithFreeOnly(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
		ProviderConfig: map[string]interface{}{
			"free_only": true,
		},
	})

	options := types.GenerateOptions{
		Model:  "test-model",
		Prompt: "Hello",
	}

	req, err := provider.prepareRequest(options)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !strings.HasSuffix(req.Model, ":free") {
		t.Errorf("Expected model to have :free suffix, got '%s'", req.Model)
	}
}

func TestChatCompletionWithMockServer(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limit check requests
		if r.URL.Path == "/v1/key" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"is_free_tier": false,
			})
			return
		}

		response := map[string]interface{}{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "test-model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello! How can I help?",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:    types.ProviderTypeOpenRouter,
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	options := types.GenerateOptions{
		Prompt: "Hello",
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error reading chunk, got %v", err)
	}

	if !chunk.Done {
		t.Error("Expected chunk to be done")
	}
	if chunk.Content != "Hello! How can I help?" {
		t.Errorf("Expected content 'Hello! How can I help?', got '%s'", chunk.Content)
	}

	stream.Close()
}

func TestFetchModelsFromAPIErrors(t *testing.T) {
	// Test no authentication
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type: types.ProviderTypeOpenRouter,
	})

	_, err := provider.fetchModelsFromAPI(context.Background())
	if err == nil {
		t.Error("Expected error for unauthenticated fetch")
	}

	// Test HTTP error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider2 := NewOpenRouterProvider(types.ProviderConfig{
		Type:    types.ProviderTypeOpenRouter,
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	_, err = provider2.fetchModelsFromAPI(context.Background())
	if err == nil {
		t.Error("Expected error for HTTP error response")
	}
}

func TestGetStaticFallback(t *testing.T) {
	provider := NewOpenRouterProvider(types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	})

	models := provider.getStaticFallback()
	if len(models) == 0 {
		t.Error("Expected non-empty static fallback models")
	}

	// Check for common models
	found := false
	for _, model := range models {
		if model.ID == "qwen/qwen3-coder" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected qwen/qwen3-coder in static fallback")
	}
}
