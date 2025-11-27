package openrouter

import (
	"context"
	"testing"

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
