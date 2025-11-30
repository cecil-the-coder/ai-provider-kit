package common

import (
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewProviderInitializer(t *testing.T) {
	config := InitializerConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     5,
	}

	initializer := NewProviderInitializer(config)

	if initializer == nil {
		t.Fatal("expected non-nil initializer")
	}

	if initializer.config.DefaultTimeout != 30*time.Second {
		t.Errorf("got timeout %v, expected %v", initializer.config.DefaultTimeout, 30*time.Second)
	}

	if initializer.config.MaxRetries != 5 {
		t.Errorf("got max retries %d, expected 5", initializer.config.MaxRetries)
	}
}

func TestNewProviderInitializer_Defaults(t *testing.T) {
	config := InitializerConfig{} // Empty config

	initializer := NewProviderInitializer(config)

	// Verify defaults are applied
	if initializer.config.DefaultTimeout != 60*time.Second {
		t.Errorf("expected default timeout of 60s, got %v", initializer.config.DefaultTimeout)
	}

	if initializer.config.MaxRetries != 3 {
		t.Errorf("expected default max retries of 3, got %d", initializer.config.MaxRetries)
	}

	if initializer.config.HealthCheckInterval != 5*time.Minute {
		t.Errorf("expected default health check interval of 5m, got %v", initializer.config.HealthCheckInterval)
	}

	if initializer.config.ModelCacheTTL != time.Hour {
		t.Errorf("expected default model cache TTL of 1h, got %v", initializer.config.ModelCacheTTL)
	}
}

func TestProviderInitializer_GetModelDisplayName(t *testing.T) {
	initializer := NewProviderInitializer(InitializerConfig{})

	tests := []struct {
		modelID  string
		expected string
	}{
		{"gpt-4", "GPT-4"},
		{"gpt-4-turbo", "GPT-4 Turbo"},
		{"gpt-3.5-turbo", "GPT-3.5 Turbo"},
		{"gpt-4o", "GPT-4o"},
		{"gpt-4o-mini", "GPT-4o Mini"},
		{"unknown-model", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			result := initializer.getModelDisplayName(tt.modelID)
			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestProviderInitializer_GetModelMaxTokens(t *testing.T) {
	initializer := NewProviderInitializer(InitializerConfig{})

	tests := []struct {
		modelID  string
		expected int
	}{
		{"gpt-4", 8192},
		{"gpt-4-turbo", 128000},
		{"gpt-3.5-turbo", 4096},
		{"gpt-4o", 128000},
		{"gpt-4o-mini", 128000},
		{"unknown-model", 4096}, // Default fallback
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			result := initializer.getModelMaxTokens(tt.modelID)
			if result != tt.expected {
				t.Errorf("got %d, expected %d", result, tt.expected)
			}
		})
	}
}

func TestProviderInitializer_ModelSupportsStreaming(t *testing.T) {
	initializer := NewProviderInitializer(InitializerConfig{})

	// All models should support streaming
	if !initializer.modelSupportsStreaming("gpt-4") {
		t.Error("expected gpt-4 to support streaming")
	}

	if !initializer.modelSupportsStreaming("any-model") {
		t.Error("expected any model to support streaming")
	}
}

func TestProviderInitializer_ModelSupportsTools(t *testing.T) {
	initializer := NewProviderInitializer(InitializerConfig{})

	tests := []struct {
		modelID  string
		expected bool
	}{
		{"gpt-4", true},
		{"gpt-4-turbo", true},
		{"gpt-3.5-turbo", true},
		{"claude-3", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			result := initializer.modelSupportsTools(tt.modelID)
			if result != tt.expected {
				t.Errorf("got %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestProviderInitializer_ValidateModel(t *testing.T) {
	initializer := NewProviderInitializer(InitializerConfig{})

	tests := []struct {
		name         string
		modelID      string
		providerType types.ProviderType
		expectError  bool
	}{
		{
			name:         "empty model ID",
			modelID:      "",
			providerType: types.ProviderTypeOpenAI,
			expectError:  true,
		},
		{
			name:         "valid OpenAI model",
			modelID:      "gpt-4",
			providerType: types.ProviderTypeOpenAI,
			expectError:  false,
		},
		{
			name:         "valid Anthropic model",
			modelID:      "claude-3-opus",
			providerType: types.ProviderTypeAnthropic,
			expectError:  false,
		},
		{
			name:         "valid Cerebras model",
			modelID:      "llama3.1-70b",
			providerType: types.ProviderTypeCerebras,
			expectError:  false,
		},
		{
			name:         "valid Gemini model",
			modelID:      "gemini-pro",
			providerType: types.ProviderTypeGemini,
			expectError:  false,
		},
		{
			name:         "unknown model - allowed",
			modelID:      "custom-model",
			providerType: types.ProviderTypeOpenAI,
			expectError:  false, // Custom models are allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := initializer.validateModel(tt.modelID, tt.providerType)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestProviderInitializer_GetStaticModels(t *testing.T) {
	initializer := NewProviderInitializer(InitializerConfig{})

	// Test Anthropic static models
	anthropicModels := initializer.getStaticAnthropicModels()
	if len(anthropicModels) == 0 {
		t.Error("expected Anthropic models")
	}

	for _, model := range anthropicModels {
		if model.Provider != types.ProviderTypeAnthropic {
			t.Errorf("expected Anthropic provider, got %s", model.Provider)
		}
		if !model.SupportsStreaming {
			t.Error("expected Anthropic models to support streaming")
		}
		if !model.SupportsToolCalling {
			t.Error("expected Anthropic models to support tool calling")
		}
	}

	// Test Cerebras static models
	cerebrasModels := initializer.getStaticCerebrasModels()
	if len(cerebrasModels) == 0 {
		t.Error("expected Cerebras models")
	}

	for _, model := range cerebrasModels {
		if model.Provider != types.ProviderTypeCerebras {
			t.Errorf("expected Cerebras provider, got %s", model.Provider)
		}
	}

	// Test Gemini static models
	geminiModels := initializer.getStaticGeminiModels()
	if len(geminiModels) == 0 {
		t.Error("expected Gemini models")
	}

	for _, model := range geminiModels {
		if model.Provider != types.ProviderTypeGemini {
			t.Errorf("expected Gemini provider, got %s", model.Provider)
		}
		if !model.SupportsStreaming {
			t.Error("expected Gemini models to support streaming")
		}
		if !model.SupportsToolCalling {
			t.Error("expected Gemini models to support tool calling")
		}
	}
}

func TestInitializerConfig_Defaults(t *testing.T) {
	config := InitializerConfig{
		EnableHealthCheck: true,
		EnableMetrics:     true,
		AutoDetectModels:  true,
		CacheModels:       true,
	}

	initializer := NewProviderInitializer(config)

	if !config.EnableHealthCheck {
		t.Error("expected health check to be enabled")
	}

	if initializer.healthCheck == nil {
		t.Error("expected health checker to be initialized")
	}

	if initializer.metrics == nil {
		t.Error("expected metrics to be initialized")
	}
}

func TestModelCapability_Structure(t *testing.T) {
	capability := &models.ModelCapability{
		MaxTokens:         8192,
		SupportsStreaming: true,
		SupportsTools:     true,
		SupportsVision:    false,
		Providers:         []types.ProviderType{types.ProviderTypeOpenAI},
		InputPrice:        0.01,
		OutputPrice:       0.03,
		Categories:        []string{"text", "code"},
	}

	if capability.MaxTokens != 8192 {
		t.Errorf("got max tokens %d, expected 8192", capability.MaxTokens)
	}

	if !capability.SupportsStreaming {
		t.Error("expected streaming support")
	}

	if !capability.SupportsTools {
		t.Error("expected tools support")
	}

	if capability.SupportsVision {
		t.Error("expected no vision support")
	}

	if len(capability.Providers) != 1 {
		t.Errorf("got %d providers, expected 1", len(capability.Providers))
	}

	if len(capability.Categories) != 2 {
		t.Errorf("got %d categories, expected 2", len(capability.Categories))
	}
}
