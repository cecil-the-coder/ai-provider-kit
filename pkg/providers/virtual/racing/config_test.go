package racing

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfig_YAML_Parsing(t *testing.T) {
	yamlConfig := `
timeout_ms: 5000
grace_period_ms: 1000
strategy: weighted
default_virtual_model: multi
virtual_models:
  multi:
    display_name: "Multi-Provider Racing"
    description: "Races OpenAI GPT-4 and Anthropic Claude with weighted strategy"
    providers:
      - name: openai
        model: gpt-4
      - name: anthropic
        model: claude-3-5-sonnet-20241022
    strategy: weighted
    timeout_ms: 3000
  fast:
    display_name: "Fast Response"
    description: "First wins strategy for fastest response"
    providers:
      - name: openai
        model: gpt-3.5-turbo
      - name: qwen
        model: qwen-turbo
    strategy: first_wins
    timeout_ms: 2000
`

	var config Config
	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	if err != nil {
		t.Fatalf("Failed to parse YAML config: %v", err)
	}

	// Test default settings
	if config.TimeoutMS != 5000 {
		t.Errorf("Expected TimeoutMS=5000, got %d", config.TimeoutMS)
	}
	if config.GracePeriodMS != 1000 {
		t.Errorf("Expected GracePeriodMS=1000, got %d", config.GracePeriodMS)
	}
	if config.Strategy != StrategyWeighted {
		t.Errorf("Expected Strategy=weighted, got %s", config.Strategy)
	}
	if config.DefaultVirtualModel != "multi" {
		t.Errorf("Expected DefaultVirtualModel=multi, got %s", config.DefaultVirtualModel)
	}

	// Test virtual models
	if len(config.VirtualModels) != 2 {
		t.Fatalf("Expected 2 virtual models, got %d", len(config.VirtualModels))
	}

	// Test multi virtual model
	multiVM := config.VirtualModels["multi"]
	if multiVM.DisplayName != "Multi-Provider Racing" {
		t.Errorf("Expected multi display name 'Multi-Provider Racing', got '%s'", multiVM.DisplayName)
	}
	if multiVM.Description != "Races OpenAI GPT-4 and Anthropic Claude with weighted strategy" {
		t.Errorf("Expected multi description mismatch")
	}
	if len(multiVM.Providers) != 2 {
		t.Errorf("Expected 2 providers in multi model, got %d", len(multiVM.Providers))
	}
	if multiVM.Strategy != StrategyWeighted {
		t.Errorf("Expected multi strategy=weighted, got %s", multiVM.Strategy)
	}
	if multiVM.TimeoutMS != 3000 {
		t.Errorf("Expected multi timeout_ms=3000, got %d", multiVM.TimeoutMS)
	}

	// Test provider references
	openaiRef := multiVM.Providers[0]
	if openaiRef.Name != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", openaiRef.Name)
	}
	if openaiRef.Model != "gpt-4" {
		t.Errorf("Expected provider model 'gpt-4', got '%s'", openaiRef.Model)
	}

	anthropicRef := multiVM.Providers[1]
	if anthropicRef.Name != "anthropic" {
		t.Errorf("Expected provider name 'anthropic', got '%s'", anthropicRef.Name)
	}
	if anthropicRef.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected provider model 'claude-3-5-sonnet-20241022', got '%s'", anthropicRef.Model)
	}

	// Test fast virtual model
	fastVM := config.VirtualModels["fast"]
	if fastVM.Strategy != StrategyFirstWins {
		t.Errorf("Expected fast strategy=first_wins, got %s", fastVM.Strategy)
	}
	if fastVM.TimeoutMS != 2000 {
		t.Errorf("Expected fast timeout_ms=2000, got %d", fastVM.TimeoutMS)
	}
}

func TestConfig_JSON_Serialization(t *testing.T) {
	config := &Config{
		TimeoutMS:           5000,
		GracePeriodMS:       1000,
		Strategy:            StrategyWeighted,
		DefaultVirtualModel: "multi",
		VirtualModels: map[string]VirtualModelConfig{
			"multi": {
				DisplayName: "Multi-Provider Racing",
				Description: "Races multiple providers",
				Providers: []ProviderReference{
					{Name: "openai", Model: "gpt-4"},
					{Name: "anthropic", Model: "claude-3-5-sonnet-20241022"},
				},
				Strategy:  StrategyWeighted,
				TimeoutMS: 3000,
			},
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaledConfig Config
	err = json.Unmarshal(jsonData, &unmarshaledConfig)
	if err != nil {
		t.Fatalf("Failed to unmarshal config from JSON: %v", err)
	}

	// Verify values match
	if unmarshaledConfig.TimeoutMS != config.TimeoutMS {
		t.Errorf("JSON marshaling/unmarshaling changed TimeoutMS: expected %d, got %d",
			config.TimeoutMS, unmarshaledConfig.TimeoutMS)
	}
	if unmarshaledConfig.Strategy != config.Strategy {
		t.Errorf("JSON marshaling/unmarshaling changed Strategy: expected %s, got %s",
			config.Strategy, unmarshaledConfig.Strategy)
	}
	if unmarshaledConfig.DefaultVirtualModel != config.DefaultVirtualModel {
		t.Errorf("JSON marshaling/unmarshaling changed DefaultVirtualModel: expected %s, got %s",
			config.DefaultVirtualModel, unmarshaledConfig.DefaultVirtualModel)
	}

	if len(unmarshaledConfig.VirtualModels) != len(config.VirtualModels) {
		t.Errorf("JSON marshaling/unmarshaling changed virtual models count: expected %d, got %d",
			len(config.VirtualModels), len(unmarshaledConfig.VirtualModels))
	}

	multiVM := unmarshaledConfig.VirtualModels["multi"]
	if multiVM.DisplayName != "Multi-Provider Racing" {
		t.Errorf("JSON marshaling/unmarshaling changed display name: expected 'Multi-Provider Racing', got '%s'",
			multiVM.DisplayName)
	}
	if len(multiVM.Providers) != 2 {
		t.Errorf("JSON marshaling/unmarshaling changed provider count: expected 2, got %d",
			len(multiVM.Providers))
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	// Create a basic config with default values
	config := &Config{
		TimeoutMS:           5000,
		GracePeriodMS:       1000,
		Strategy:            StrategyFirstWins,
		DefaultVirtualModel: "default",
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Racing Model",
				Description: "Default virtual racing model with balanced providers",
				Strategy:    StrategyFirstWins,
				TimeoutMS:   5000,
			},
		},
	}

	// Test config values
	if config.TimeoutMS != 5000 {
		t.Errorf("Expected TimeoutMS=5000, got %d", config.TimeoutMS)
	}
	if config.GracePeriodMS != 1000 {
		t.Errorf("Expected GracePeriodMS=1000, got %d", config.GracePeriodMS)
	}
	if config.Strategy != StrategyFirstWins {
		t.Errorf("Expected Strategy=first_wins, got %s", config.Strategy)
	}
	if config.DefaultVirtualModel != "default" {
		t.Errorf("Expected DefaultVirtualModel=default, got %s", config.DefaultVirtualModel)
	}

	// Test virtual model
	defaultVM := config.VirtualModels["default"]
	if defaultVM.DisplayName != "Default Racing Model" {
		t.Errorf("Expected default display name 'Default Racing Model', got '%s'", defaultVM.DisplayName)
	}
	if defaultVM.Strategy != StrategyFirstWins {
		t.Errorf("Expected default strategy=first_wins, got %s", defaultVM.Strategy)
	}
	if defaultVM.TimeoutMS != 5000 {
		t.Errorf("Expected default timeout_ms=5000, got %d", defaultVM.TimeoutMS)
	}
}

func TestConfig_GetVirtualModel(t *testing.T) {
	config := &Config{
		DefaultVirtualModel: "default",
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				Description: "Default virtual model",
				Providers: []ProviderReference{
					{Name: "openai", Model: "gpt-4"},
				},
				Strategy:  StrategyFirstWins,
				TimeoutMS: 5000,
			},
			"custom": {
				DisplayName: "Custom Model",
				Description: "Custom virtual model",
				Providers: []ProviderReference{
					{Name: "anthropic", Model: "claude-3-5-sonnet-20241022"},
				},
				Strategy:  StrategyWeighted,
				TimeoutMS: 3000,
			},
		},
	}

	// Test getting existing virtual model
	vm := config.GetVirtualModel("custom")
	if vm == nil {
		t.Fatal("Expected virtual model 'custom', got nil")
	}
	if vm.DisplayName != "Custom Model" {
		t.Errorf("Expected display name 'Custom Model', got '%s'", vm.DisplayName)
	}

	// Test getting default virtual model when model name is empty
	vm = config.GetVirtualModel("")
	if vm == nil {
		t.Fatal("Expected default virtual model, got nil")
	}
	if vm.DisplayName != "Default Model" {
		t.Errorf("Expected default display name 'Default Model', got '%s'", vm.DisplayName)
	}

	// Test getting default virtual model explicitly
	vm = config.GetVirtualModel("default")
	if vm == nil {
		t.Fatal("Expected default virtual model, got nil")
	}
	if vm.DisplayName != "Default Model" {
		t.Errorf("Expected default display name 'Default Model', got '%s'", vm.DisplayName)
	}

	// Test getting non-existent virtual model (should return default or nil)
	vm = config.GetVirtualModel("nonexistent")
	if vm != nil {
		t.Errorf("Expected nil for nonexistent virtual model, got %v", vm)
	}
}

func TestConfig_GetEffectiveTimeout(t *testing.T) {
	config := &Config{
		TimeoutMS:           5000,
		DefaultVirtualModel: "default",
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				TimeoutMS:   0, // Use default
			},
			"fast": {
				DisplayName: "Fast Model",
				TimeoutMS:   2000, // Override default
			},
		},
	}

	// Test default timeout
	timeout := config.GetEffectiveTimeout("default")
	if timeout != 5000 {
		t.Errorf("Expected default timeout 5000, got %d", timeout)
	}

	// Test overridden timeout
	timeout = config.GetEffectiveTimeout("fast")
	if timeout != 2000 {
		t.Errorf("Expected fast timeout 2000, got %d", timeout)
	}

	// Test non-existent model (should use default)
	timeout = config.GetEffectiveTimeout("nonexistent")
	if timeout != 5000 {
		t.Errorf("Expected default timeout for nonexistent model 5000, got %d", timeout)
	}
}

func TestConfig_GetEffectiveStrategy(t *testing.T) {
	config := &Config{
		Strategy:            StrategyFirstWins,
		DefaultVirtualModel: "default",
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				Strategy:    "", // Use default
			},
			"weighted": {
				DisplayName: "Weighted Model",
				Strategy:    StrategyWeighted, // Override default
			},
		},
	}

	// Test default strategy
	strategy := config.GetEffectiveStrategy("default")
	if strategy != StrategyFirstWins {
		t.Errorf("Expected default strategy first_wins, got %s", strategy)
	}

	// Test overridden strategy
	strategy = config.GetEffectiveStrategy("weighted")
	if strategy != StrategyWeighted {
		t.Errorf("Expected weighted strategy, got %s", strategy)
	}

	// Test non-existent model (should use default)
	strategy = config.GetEffectiveStrategy("nonexistent")
	if strategy != StrategyFirstWins {
		t.Errorf("Expected default strategy for nonexistent model first_wins, got %s", strategy)
	}
}

func TestConfig_ConfigurationIntegration(t *testing.T) {
	// Test that all the methods work together correctly
	config := &Config{
		TimeoutMS:           5000,
		GracePeriodMS:       1000,
		Strategy:            StrategyFirstWins,
		DefaultVirtualModel: "default",
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				Description: "Default virtual model",
				Providers: []ProviderReference{
					{Name: "openai", Model: "gpt-4", Priority: 1},
				},
				Strategy:  StrategyFirstWins,
				TimeoutMS: 5000,
			},
			"weighted": {
				DisplayName: "Weighted Model",
				Description: "Weighted virtual model",
				Providers: []ProviderReference{
					{Name: "anthropic", Model: "claude-3-5-sonnet-20241022", Priority: 2},
				},
				Strategy:  StrategyWeighted,
				TimeoutMS: 3000,
			},
		},
	}

	// Test GetVirtualModel method
	vm := config.GetVirtualModel("weighted")
	if vm == nil {
		t.Fatal("Expected virtual model 'weighted', got nil")
	}
	if vm.DisplayName != "Weighted Model" {
		t.Errorf("Expected display name 'Weighted Model', got '%s'", vm.DisplayName)
	}

	// Test GetEffectiveTimeout
	timeout := config.GetEffectiveTimeout("weighted")
	if timeout != 3000 {
		t.Errorf("Expected effective timeout 3000, got %d", timeout)
	}

	defaultTimeout := config.GetEffectiveTimeout("default")
	if defaultTimeout != 5000 {
		t.Errorf("Expected default timeout 5000, got %d", defaultTimeout)
	}

	// Test GetEffectiveStrategy
	strategy := config.GetEffectiveStrategy("weighted")
	if strategy != StrategyWeighted {
		t.Errorf("Expected weighted strategy, got %s", strategy)
	}

	defaultStrategy := config.GetEffectiveStrategy("default")
	if defaultStrategy != StrategyFirstWins {
		t.Errorf("Expected first_wins strategy, got %s", defaultStrategy)
	}
}

func TestProviderReference_Parsing(t *testing.T) {
	yamlConfig := `
providers:
  - name: openai
    model: gpt-4
    priority: 1
    config:
      temperature: 0.7
  - name: anthropic
    model: claude-3-5-sonnet-20241022
    priority: 2
  - name: gemini
    model: gemini-pro
    priority: 3
`

	var structWithRefs struct {
		Providers []ProviderReference `yaml:"providers"`
	}

	err := yaml.Unmarshal([]byte(yamlConfig), &structWithRefs)
	if err != nil {
		t.Fatalf("Failed to parse provider references: %v", err)
	}

	if len(structWithRefs.Providers) != 3 {
		t.Fatalf("Expected 3 provider references, got %d", len(structWithRefs.Providers))
	}

	// Test first provider
	ref1 := structWithRefs.Providers[0]
	if ref1.Name != "openai" {
		t.Errorf("Expected provider name 'openai', got '%s'", ref1.Name)
	}
	if ref1.Model != "gpt-4" {
		t.Errorf("Expected provider model 'gpt-4', got '%s'", ref1.Model)
	}
	if ref1.Priority != 1 {
		t.Errorf("Expected provider priority 1, got %d", ref1.Priority)
	}
	if ref1.Config == nil {
		t.Error("Expected provider config, got nil")
	} else if temp, ok := ref1.Config["temperature"].(float64); !ok || temp != 0.7 {
		t.Errorf("Expected temperature 0.7, got %v", ref1.Config["temperature"])
	}

	// Test second provider
	ref2 := structWithRefs.Providers[1]
	if ref2.Name != "anthropic" {
		t.Errorf("Expected provider name 'anthropic', got '%s'", ref2.Name)
	}
	if ref2.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected provider model 'claude-3-5-sonnet-20241022', got '%s'", ref2.Model)
	}
	if ref2.Priority != 2 {
		t.Errorf("Expected provider priority 2, got %d", ref2.Priority)
	}

	// Test third provider
	ref3 := structWithRefs.Providers[2]
	if ref3.Name != "gemini" {
		t.Errorf("Expected provider name 'gemini', got '%s'", ref3.Name)
	}
	if ref3.Model != "gemini-pro" {
		t.Errorf("Expected provider model 'gemini-pro', got '%s'", ref3.Model)
	}
	if ref3.Priority != 3 {
		t.Errorf("Expected provider priority 3, got %d", ref3.Priority)
	}
}

func TestStrategyOverrides(t *testing.T) {
	config := &Config{
		Strategy: StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				Strategy:    "", // Should inherit from config
			},
			"weighted": {
				DisplayName: "Weighted Model",
				Strategy:    StrategyWeighted,
			},
			"quality": {
				DisplayName: "Quality Model",
				Strategy:    StrategyQuality,
			},
		},
	}

	// Test strategy inheritance
	// The actual implementation returns the config as-is, without inheritance in GetVirtualModel
	// Inheritance happens in GetEffectiveStrategy
	effectiveStrategy := config.GetEffectiveStrategy("default")
	if effectiveStrategy != StrategyFirstWins {
		t.Errorf("Expected inherited strategy first_wins, got %s", effectiveStrategy)
	}

	// Test explicit strategy override
	weightedStrategy := config.GetEffectiveStrategy("weighted")
	if weightedStrategy != StrategyWeighted {
		t.Errorf("Expected weighted strategy, got %s", weightedStrategy)
	}

	qualityStrategy := config.GetEffectiveStrategy("quality")
	if qualityStrategy != StrategyQuality {
		t.Errorf("Expected quality strategy, got %s", qualityStrategy)
	}
}
