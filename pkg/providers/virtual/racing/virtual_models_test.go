package racing

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestVirtualModels_GetModels(t *testing.T) {
	config := &Config{
		DefaultVirtualModel: "default",
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Racing Model",
				Description: "Default virtual racing model",
				Strategy:    StrategyFirstWins,
				TimeoutMS:   5000,
				Providers: []ProviderReference{
					{Name: "provider1", Model: "model1"},
					{Name: "provider2", Model: "model2"},
				},
			},
			"fast": {
				DisplayName: "Fast Racing Model",
				Description: "Fast virtual racing model with optimized providers",
				Strategy:    StrategyFirstWins,
				TimeoutMS:   3000,
				Providers: []ProviderReference{
					{Name: "fast-provider1", Model: "fast-model1"},
					{Name: "fast-provider2", Model: "fast-model2"},
				},
			},
		},
	}

	provider := NewRacingProvider("test", config)

	// Add mock providers
	mockProviders := []types.Provider{
		&mockChatProvider{name: "provider1"},
		&mockChatProvider{name: "provider2"},
		&mockChatProvider{name: "fast-provider1"},
		&mockChatProvider{name: "fast-provider2"},
	}
	provider.SetProviders(mockProviders)

	models, err := provider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}

	// Check default model
	defaultModel := provider.GetDefaultModel()
	if defaultModel != "default" {
		t.Errorf("Expected default model 'default', got '%s'", defaultModel)
	}

	// Verify model properties
	modelMap := make(map[string]types.Model)
	for _, model := range models {
		modelMap[model.ID] = model
	}

	defaultModelInfo, exists := modelMap["default"]
	if !exists {
		t.Fatal("Default model not found in models list")
	}

	if defaultModelInfo.Name != "Default Racing Model" {
		t.Errorf("Expected display name 'Default Racing Model', got '%s'", defaultModelInfo.Name)
	}

	if defaultModelInfo.Description != "Default virtual racing model" {
		t.Errorf("Expected description 'Default virtual racing model', got '%s'", defaultModelInfo.Description)
	}

	if defaultModelInfo.Provider != "racing" {
		t.Errorf("Expected provider 'racing', got '%s'", defaultModelInfo.Provider)
	}

	// Check fast model
	fastModelInfo, exists := modelMap["fast"]
	if !exists {
		t.Fatal("Fast model not found in models list")
	}

	if fastModelInfo.Name != "Fast Racing Model" {
		t.Errorf("Expected display name 'Fast Racing Model', got '%s'", fastModelInfo.Name)
	}
}

func TestVirtualModels_Config(t *testing.T) {
	config := &Config{
		DefaultVirtualModel: "default",
		TimeoutMS:           5000,
		GracePeriodMS:       1000,
		Strategy:            StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				Description: "Default virtual model",
				Strategy:    StrategyWeighted, // Override default strategy
				TimeoutMS:   3000,             // Override default timeout
				Providers: []ProviderReference{
					{Name: "provider1", Model: "model1"},
				},
			},
			"custom": {
				DisplayName: "Custom Model",
				Description: "Custom virtual model using defaults",
				// No strategy or timeout specified, should use defaults
				Providers: []ProviderReference{
					{Name: "provider2", Model: "model2"},
				},
			},
		},
	}

	// Test GetVirtualModel
	vmConfig := config.GetVirtualModel("default")
	if vmConfig == nil {
		t.Fatal("Expected virtual model config for 'default', got nil")
	}

	if vmConfig.Strategy != StrategyWeighted {
		t.Errorf("Expected strategy 'weighted', got '%s'", vmConfig.Strategy)
	}

	if vmConfig.TimeoutMS != 3000 {
		t.Errorf("Expected timeout 3000, got %d", vmConfig.TimeoutMS)
	}

	// Test GetVirtualModel with defaults
	customVMConfig := config.GetVirtualModel("custom")
	if customVMConfig == nil {
		t.Fatal("Expected virtual model config for 'custom', got nil")
	}

	if customVMConfig.Strategy != StrategyFirstWins {
		t.Errorf("Expected default strategy 'first_wins', got '%s'", customVMConfig.Strategy)
	}

	if customVMConfig.TimeoutMS != 5000 {
		t.Errorf("Expected default timeout 5000, got %d", customVMConfig.TimeoutMS)
	}

	// Test GetEffectiveTimeout
	timeout := config.GetEffectiveTimeout("default")
	if timeout != 3000 {
		t.Errorf("Expected effective timeout 3000, got %d", timeout)
	}

	timeout = config.GetEffectiveTimeout("custom")
	if timeout != 5000 {
		t.Errorf("Expected effective timeout 5000, got %d", timeout)
	}

	// Test GetEffectiveStrategy
	strategy := config.GetEffectiveStrategy("default")
	if strategy != StrategyWeighted {
		t.Errorf("Expected effective strategy 'weighted', got '%s'", strategy)
	}

	strategy = config.GetEffectiveStrategy("custom")
	if strategy != StrategyFirstWins {
		t.Errorf("Expected effective strategy 'first_wins', got '%s'", strategy)
	}
}

func TestVirtualModels_GenerateChatCompletion(t *testing.T) {
	config := &Config{
		DefaultVirtualModel: "default",
		TimeoutMS:           5000,
		Strategy:            StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				Description: "Default virtual model",
				Strategy:    StrategyFirstWins,
				TimeoutMS:   5000,
				Providers: []ProviderReference{
					{Name: "provider1", Model: "model1"},
					{Name: "provider2", Model: "model2"},
				},
			},
		},
	}

	provider := NewRacingProvider("test", config)

	// Add mock providers
	mockProviders := []types.Provider{
		&mockChatProvider{name: "provider1", response: "Response from provider1"},
		&mockChatProvider{name: "provider2", response: "Response from provider2"},
	}
	provider.SetProviders(mockProviders)

	opts := types.GenerateOptions{
		Model: "default",
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), opts)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if stream == nil {
		t.Fatal("Expected non-nil stream")
	}

	// Test that we can read from the stream
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error reading chunk, got %v", err)
	}

	if chunk.Content == "" {
		t.Error("Expected non-empty content")
	}

	// Check that racing metadata is included
	if chunk.Metadata == nil {
		t.Error("Expected metadata in chunk")
	}

	if _, exists := chunk.Metadata["racing_winner"]; !exists {
		t.Error("Expected racing_winner metadata")
	}

	// Clean up
	err = stream.Close()
	if err != nil {
		t.Errorf("Expected no error closing stream, got %v", err)
	}
}

func TestVirtualModels_InvalidModel(t *testing.T) {
	config := &Config{
		DefaultVirtualModel: "default",
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				Providers:   []ProviderReference{{Name: "provider1", Model: "model1"}},
			},
		},
	}

	provider := NewRacingProvider("test", config)
	provider.SetProviders([]types.Provider{&mockChatProvider{name: "provider1"}})

	opts := types.GenerateOptions{
		Model: "nonexistent",
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	_, err := provider.GenerateChatCompletion(context.Background(), opts)
	if err == nil {
		t.Fatal("Expected error for nonexistent virtual model")
	}

	expectedError := "virtual model not found: nonexistent"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}
