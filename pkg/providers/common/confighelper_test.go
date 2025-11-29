package common

import (
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewConfigHelper(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	if helper == nil {
		t.Fatal("expected non-nil helper")
	}

	if helper.providerName != "openai" {
		t.Errorf("got provider name %q, expected %q", helper.providerName, "openai")
	}

	if helper.providerType != types.ProviderTypeOpenAI {
		t.Errorf("got provider type %s, expected %s", helper.providerType, types.ProviderTypeOpenAI)
	}
}

func TestConfigHelper_ValidateProviderConfig(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	tests := []struct {
		name        string
		config      types.ProviderConfig
		expectValid bool
	}{
		{
			name: "valid config",
			config: types.ProviderConfig{
				Type:    types.ProviderTypeOpenAI,
				APIKey:  "test-key",
				Timeout: 30 * time.Second,
			},
			expectValid: true,
		},
		{
			name: "wrong provider type",
			config: types.ProviderConfig{
				Type: types.ProviderTypeAnthropic,
			},
			expectValid: false,
		},
		{
			name: "negative timeout",
			config: types.ProviderConfig{
				Type:    types.ProviderTypeOpenAI,
				Timeout: -5 * time.Second,
			},
			expectValid: false,
		},
		{
			name: "negative max tokens",
			config: types.ProviderConfig{
				Type:      types.ProviderTypeOpenAI,
				MaxTokens: -100,
			},
			expectValid: false,
		},
		{
			name: "zero timeout allowed",
			config: types.ProviderConfig{
				Type:    types.ProviderTypeOpenAI,
				Timeout: 0,
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.ValidateProviderConfig(tt.config)

			if result.Valid != tt.expectValid {
				t.Errorf("got valid=%v, expected %v; errors: %v", result.Valid, tt.expectValid, result.Errors)
			}
		})
	}
}

func TestConfigHelper_ExtractAPIKeys(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	tests := []struct {
		name           string
		config         types.ProviderConfig
		expectedLength int
	}{
		{
			name: "single API key",
			config: types.ProviderConfig{
				APIKey: "test-key",
			},
			expectedLength: 1,
		},
		{
			name: "multiple API keys",
			config: types.ProviderConfig{
				APIKey: "key1",
				ProviderConfig: map[string]interface{}{
					"api_keys": []string{"key2", "key3"},
				},
			},
			expectedLength: 3,
		},
		{
			name:           "no API keys",
			config:         types.ProviderConfig{},
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := helper.ExtractAPIKeys(tt.config)

			if len(keys) != tt.expectedLength {
				t.Errorf("got %d keys, expected %d", len(keys), tt.expectedLength)
			}
		})
	}
}

func TestConfigHelper_ExtractBaseURL(t *testing.T) {
	tests := []struct {
		providerType types.ProviderType
		config       types.ProviderConfig
		expected     string
	}{
		{
			providerType: types.ProviderTypeOpenAI,
			config:       types.ProviderConfig{},
			expected:     "https://api.openai.com/v1",
		},
		{
			providerType: types.ProviderTypeAnthropic,
			config:       types.ProviderConfig{},
			expected:     "https://api.anthropic.com",
		},
		{
			providerType: types.ProviderTypeGemini,
			config:       types.ProviderConfig{},
			expected:     "https://generativelanguage.googleapis.com/v1beta",
		},
		{
			providerType: types.ProviderTypeCerebras,
			config:       types.ProviderConfig{},
			expected:     "https://api.cerebras.ai/v1",
		},
		{
			providerType: types.ProviderTypeOpenRouter,
			config:       types.ProviderConfig{},
			expected:     "https://openrouter.ai/api/v1",
		},
		{
			providerType: types.ProviderTypeOpenAI,
			config: types.ProviderConfig{
				BaseURL: "https://custom.api.com",
			},
			expected: "https://custom.api.com",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			helper := NewConfigHelper(string(tt.providerType), tt.providerType)
			result := helper.ExtractBaseURL(tt.config)

			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestConfigHelper_ExtractDefaultModel(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	config := types.ProviderConfig{
		DefaultModel: "gpt-4",
	}

	result := helper.ExtractDefaultModel(config)
	if result != "gpt-4" {
		t.Errorf("got %q, expected %q", result, "gpt-4")
	}

	// Test empty
	config = types.ProviderConfig{}
	result = helper.ExtractDefaultModel(config)
	if result != "" {
		t.Errorf("got %q, expected empty string", result)
	}
}

func TestConfigHelper_ExtractTimeout(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	tests := []struct {
		name     string
		config   types.ProviderConfig
		expected time.Duration
	}{
		{
			name: "custom timeout",
			config: types.ProviderConfig{
				Timeout: 120 * time.Second,
			},
			expected: 120 * time.Second,
		},
		{
			name:     "default timeout",
			config:   types.ProviderConfig{},
			expected: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.ExtractTimeout(tt.config)

			if result != tt.expected {
				t.Errorf("got %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestConfigHelper_ExtractMaxTokens(t *testing.T) {
	tests := []struct {
		providerType types.ProviderType
		config       types.ProviderConfig
		expected     int
	}{
		{
			providerType: types.ProviderTypeOpenAI,
			config:       types.ProviderConfig{},
			expected:     4096,
		},
		{
			providerType: types.ProviderTypeAnthropic,
			config:       types.ProviderConfig{},
			expected:     4096,
		},
		{
			providerType: types.ProviderTypeGemini,
			config:       types.ProviderConfig{},
			expected:     8192,
		},
		{
			providerType: types.ProviderTypeOpenAI,
			config: types.ProviderConfig{
				MaxTokens: 8000,
			},
			expected: 8000,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			helper := NewConfigHelper(string(tt.providerType), tt.providerType)
			result := helper.ExtractMaxTokens(tt.config)

			if result != tt.expected {
				t.Errorf("got %d, expected %d", result, tt.expected)
			}
		})
	}
}

func TestConfigHelper_ExtractStringField(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	tests := []struct {
		name     string
		config   types.ProviderConfig
		field    string
		fallback string
		expected string
	}{
		{
			name: "field exists",
			config: types.ProviderConfig{
				ProviderConfig: map[string]interface{}{
					"custom_field": "custom_value",
				},
			},
			field:    "custom_field",
			fallback: "default",
			expected: "custom_value",
		},
		{
			name: "field missing",
			config: types.ProviderConfig{
				ProviderConfig: map[string]interface{}{},
			},
			field:    "missing_field",
			fallback: "default",
			expected: "default",
		},
		{
			name:     "no provider config",
			config:   types.ProviderConfig{},
			field:    "any_field",
			fallback: "default",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.ExtractStringField(tt.config, tt.field, tt.fallback)

			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestConfigHelper_ExtractStringSliceField(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	tests := []struct {
		name           string
		config         types.ProviderConfig
		field          string
		expectedLength int
	}{
		{
			name: "slice exists",
			config: types.ProviderConfig{
				ProviderConfig: map[string]interface{}{
					"tags": []string{"tag1", "tag2"},
				},
			},
			field:          "tags",
			expectedLength: 2,
		},
		{
			name: "field missing",
			config: types.ProviderConfig{
				ProviderConfig: map[string]interface{}{},
			},
			field:          "missing",
			expectedLength: 0,
		},
		{
			name:           "no provider config",
			config:         types.ProviderConfig{},
			field:          "tags",
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.ExtractStringSliceField(tt.config, tt.field)

			if len(result) != tt.expectedLength {
				t.Errorf("got length %d, expected %d", len(result), tt.expectedLength)
			}
		})
	}
}

func TestConfigHelper_ExtractDefaultOAuthClientID(t *testing.T) {
	tests := []struct {
		providerType types.ProviderType
		expected     string
	}{
		{
			providerType: types.ProviderTypeOpenAI,
			expected:     "app_EMoamEEZ73f0CkXaXp7hrann",
		},
		{
			providerType: types.ProviderTypeAnthropic,
			expected:     "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		},
		{
			providerType: types.ProviderTypeGemini,
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			helper := NewConfigHelper(string(tt.providerType), tt.providerType)
			result := helper.ExtractDefaultOAuthClientID()

			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestConfigHelper_GetProviderCapabilities(t *testing.T) {
	tests := []struct {
		providerType       types.ProviderType
		expectToolCalling  bool
		expectStreaming    bool
		expectResponsesAPI bool
	}{
		{
			providerType:       types.ProviderTypeOpenAI,
			expectToolCalling:  true,
			expectStreaming:    true,
			expectResponsesAPI: true,
		},
		{
			providerType:       types.ProviderTypeAnthropic,
			expectToolCalling:  true,
			expectStreaming:    true,
			expectResponsesAPI: false,
		},
		{
			providerType:       types.ProviderTypeGemini,
			expectToolCalling:  true,
			expectStreaming:    true,
			expectResponsesAPI: false,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			helper := NewConfigHelper(string(tt.providerType), tt.providerType)
			toolCalling, streaming, responsesAPI := helper.GetProviderCapabilities()

			if toolCalling != tt.expectToolCalling {
				t.Errorf("got toolCalling=%v, expected %v", toolCalling, tt.expectToolCalling)
			}
			if streaming != tt.expectStreaming {
				t.Errorf("got streaming=%v, expected %v", streaming, tt.expectStreaming)
			}
			if responsesAPI != tt.expectResponsesAPI {
				t.Errorf("got responsesAPI=%v, expected %v", responsesAPI, tt.expectResponsesAPI)
			}
		})
	}
}

func TestConfigHelper_SanitizeConfigForLogging(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	config := types.ProviderConfig{
		APIKey: "secret-key",
		ProviderConfig: map[string]interface{}{
			"api_key":       "another-secret",
			"client_secret": "secret",
			"safe_field":    "visible",
		},
	}

	sanitized := helper.SanitizeConfigForLogging(config)

	if sanitized.APIKey != "" {
		t.Error("expected APIKey to be cleared")
	}

	if _, exists := sanitized.ProviderConfig["api_key"]; exists {
		t.Error("expected api_key to be removed")
	}

	if _, exists := sanitized.ProviderConfig["client_secret"]; exists {
		t.Error("expected client_secret to be removed")
	}

	if val, exists := sanitized.ProviderConfig["safe_field"]; !exists || val != "visible" {
		t.Error("expected safe_field to be preserved")
	}
}

func TestConfigHelper_ConfigSummary(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	config := types.ProviderConfig{
		Type:         types.ProviderTypeOpenAI,
		APIKey:       "test-key",
		DefaultModel: "gpt-4",
		Timeout:      60 * time.Second,
		MaxTokens:    4096,
	}

	summary := helper.ConfigSummary(config)

	if summary["provider"] != "openai" {
		t.Errorf("got provider %v, expected %q", summary["provider"], "openai")
	}

	if summary["type"] != types.ProviderTypeOpenAI {
		t.Errorf("got type %v, expected %v", summary["type"], types.ProviderTypeOpenAI)
	}

	if summary["default_model"] != "gpt-4" {
		t.Errorf("got default_model %v, expected %q", summary["default_model"], "gpt-4")
	}

	authMethods, ok := summary["auth_methods"].([]string)
	if !ok {
		t.Fatal("expected auth_methods to be []string")
	}

	foundAPIKey := false
	for _, method := range authMethods {
		if method == "api_key" {
			foundAPIKey = true
		}
	}
	if !foundAPIKey {
		t.Error("expected auth_methods to include 'api_key'")
	}

	capabilities, ok := summary["capabilities"].(map[string]bool)
	if !ok {
		t.Fatal("expected capabilities to be map[string]bool")
	}

	if !capabilities["tool_calling"] {
		t.Error("expected tool_calling capability")
	}
	if !capabilities["streaming"] {
		t.Error("expected streaming capability")
	}
}

func TestConfigHelper_MergeWithDefaults(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	config := types.ProviderConfig{
		Type: types.ProviderTypeOpenAI,
	}

	merged := helper.MergeWithDefaults(config)

	if merged.BaseURL == "" {
		t.Error("expected BaseURL to be filled with default")
	}

	if merged.Timeout == 0 {
		t.Error("expected Timeout to be filled with default")
	}

	if merged.MaxTokens == 0 {
		t.Error("expected MaxTokens to be filled with default")
	}

	// Test that existing values are preserved
	config2 := types.ProviderConfig{
		Type:      types.ProviderTypeOpenAI,
		BaseURL:   "https://custom.com",
		Timeout:   120 * time.Second,
		MaxTokens: 8000,
	}

	merged2 := helper.MergeWithDefaults(config2)

	if merged2.BaseURL != "https://custom.com" {
		t.Errorf("got BaseURL %q, expected custom value", merged2.BaseURL)
	}

	if merged2.Timeout != 120*time.Second {
		t.Errorf("got Timeout %v, expected custom value", merged2.Timeout)
	}

	if merged2.MaxTokens != 8000 {
		t.Errorf("got MaxTokens %d, expected custom value", merged2.MaxTokens)
	}
}

func TestConfigHelper_ExtractProviderSpecificConfig(t *testing.T) {
	helper := NewConfigHelper("openai", types.ProviderTypeOpenAI)

	type CustomConfig struct {
		Organization string `json:"organization"`
		ProjectID    string `json:"project_id"`
	}

	config := types.ProviderConfig{
		ProviderConfig: map[string]interface{}{
			"organization": "test-org",
			"project_id":   "test-project",
		},
	}

	var target CustomConfig
	err := helper.ExtractProviderSpecificConfig(config, &target)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if target.Organization != "test-org" {
		t.Errorf("got organization %q, expected %q", target.Organization, "test-org")
	}

	if target.ProjectID != "test-project" {
		t.Errorf("got project_id %q, expected %q", target.ProjectID, "test-project")
	}

	// Test with nil ProviderConfig
	config2 := types.ProviderConfig{}
	var target2 CustomConfig
	err = helper.ExtractProviderSpecificConfig(config2, &target2)

	if err != nil {
		t.Errorf("unexpected error with nil config: %v", err)
	}
}

func TestConfigHelper_ExtractBaseURL_AllProviders(t *testing.T) {
	qwen := types.ProviderTypeQwen
	helper := NewConfigHelper("qwen", qwen)

	result := helper.ExtractBaseURL(types.ProviderConfig{})
	if result == "" {
		t.Error("expected non-empty base URL for Qwen")
	}
}

func TestConfigHelper_ExtractMaxTokens_AllProviders(t *testing.T) {
	tests := []types.ProviderType{
		types.ProviderTypeQwen,
		types.ProviderTypeOpenRouter,
	}

	for _, pt := range tests {
		helper := NewConfigHelper(string(pt), pt)
		result := helper.ExtractMaxTokens(types.ProviderConfig{})
		if result == 0 {
			t.Errorf("expected non-zero max tokens for %s", pt)
		}
	}
}

func TestConfigHelper_ApplyTopLevelOverrides(t *testing.T) {
	helper := NewConfigHelper("test", types.ProviderTypeOpenAI)

	config := types.ProviderConfig{
		APIKey:       "test-key",
		BaseURL:      "https://test.com",
		DefaultModel: "test-model",
	}

	type ProviderConfig struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
	}

	providerConfig := &ProviderConfig{}

	err := helper.ApplyTopLevelOverrides(config, providerConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if providerConfig.APIKey != "test-key" {
		t.Errorf("got API key %q, expected %q", providerConfig.APIKey, "test-key")
	}
}

func TestConfigHelper_ExtractDefaultOAuthClientID_AllProviders(t *testing.T) {
	unknownProvider := types.ProviderTypeCerebras
	helper := NewConfigHelper("cerebras", unknownProvider)

	result := helper.ExtractDefaultOAuthClientID()
	if result != "" {
		t.Errorf("expected empty client ID for Cerebras, got %q", result)
	}
}
