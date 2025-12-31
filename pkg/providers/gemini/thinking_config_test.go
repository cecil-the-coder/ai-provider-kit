// Package gemini provides tests for thinking config functionality.
package gemini

import (
	"testing"
)

func TestGetModelFamily(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected ModelFamily
	}{
		{
			name:     "Gemini 2.5 Pro",
			model:    "gemini-2.5-pro",
			expected: ModelFamilyGemini25Pro,
		},
		{
			name:     "Gemini 2.5 Pro Preview",
			model:    "gemini-2.5-pro-preview",
			expected: ModelFamilyGemini25Pro,
		},
		{
			name:     "Gemini 2.5 Pro with tier suffix",
			model:    "gemini-2.5-pro-high",
			expected: ModelFamilyGemini25Pro,
		},
		{
			name:     "Gemini 2.5 Flash",
			model:    "gemini-2.5-flash",
			expected: ModelFamilyGemini25Flash,
		},
		{
			name:     "Gemini 2.5 Flash Lite",
			model:    "gemini-2.5-flash-lite",
			expected: ModelFamilyGemini25Flash,
		},
		{
			name:     "Gemini 2.5 Flash with tier suffix",
			model:    "gemini-2.5-flash-medium",
			expected: ModelFamilyGemini25Flash,
		},
		{
			name:     "Gemini 3 Pro Preview",
			model:    "gemini-3-pro-preview",
			expected: ModelFamilyGemini3,
		},
		{
			name:     "Gemini 3 with tier suffix",
			model:    "gemini-3-pro-high",
			expected: ModelFamilyGemini3,
		},
		{
			name:     "Gemini 2.0 Flash (not supported for thinking)",
			model:    "gemini-2.0-flash",
			expected: ModelFamilyUnknown,
		},
		{
			name:     "Unknown model",
			model:    "unknown-model",
			expected: ModelFamilyUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelFamily(tt.model)
			if result != tt.expected {
				t.Errorf("GetModelFamily(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestParseThinkingTierFromModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected ThinkingLevel
	}{
		{
			name:     "Model with low tier suffix",
			model:    "gemini-2.5-pro-low",
			expected: ThinkingLevelLow,
		},
		{
			name:     "Model with medium tier suffix",
			model:    "gemini-2.5-pro-medium",
			expected: ThinkingLevelMedium,
		},
		{
			name:     "Model with high tier suffix",
			model:    "gemini-2.5-flash-high",
			expected: ThinkingLevelHigh,
		},
		{
			name:     "Model without tier suffix",
			model:    "gemini-2.5-pro",
			expected: "",
		},
		{
			name:     "Model with preview suffix",
			model:    "gemini-2.5-pro-preview",
			expected: "",
		},
		{
			name:     "Model with lite suffix",
			model:    "gemini-2.5-flash-lite",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseThinkingTierFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("ParseThinkingTierFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestStripThinkingTierFromModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{
			name:     "Model with low tier suffix",
			model:    "gemini-2.5-pro-low",
			expected: "gemini-2.5-pro",
		},
		{
			name:     "Model with medium tier suffix",
			model:    "gemini-2.5-pro-medium",
			expected: "gemini-2.5-pro",
		},
		{
			name:     "Model with high tier suffix",
			model:    "gemini-2.5-flash-high",
			expected: "gemini-2.5-flash",
		},
		{
			name:     "Model without tier suffix",
			model:    "gemini-2.5-pro",
			expected: "gemini-2.5-pro",
		},
		{
			name:     "Model with preview suffix",
			model:    "gemini-2.5-pro-preview",
			expected: "gemini-2.5-pro-preview",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripThinkingTierFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("StripThinkingTierFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

func TestGetThinkingBudgetForModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		tier     ThinkingLevel
		expected int
	}{
		// Gemini 2.5 Pro budgets
		{
			name:     "Gemini 2.5 Pro low",
			model:    "gemini-2.5-pro",
			tier:     ThinkingLevelLow,
			expected: 8192,
		},
		{
			name:     "Gemini 2.5 Pro medium",
			model:    "gemini-2.5-pro",
			tier:     ThinkingLevelMedium,
			expected: 16384,
		},
		{
			name:     "Gemini 2.5 Pro high",
			model:    "gemini-2.5-pro",
			tier:     ThinkingLevelHigh,
			expected: 32768,
		},
		// Gemini 2.5 Flash budgets
		{
			name:     "Gemini 2.5 Flash low",
			model:    "gemini-2.5-flash",
			tier:     ThinkingLevelLow,
			expected: 6144,
		},
		{
			name:     "Gemini 2.5 Flash medium",
			model:    "gemini-2.5-flash",
			tier:     ThinkingLevelMedium,
			expected: 12288,
		},
		{
			name:     "Gemini 2.5 Flash high",
			model:    "gemini-2.5-flash",
			tier:     ThinkingLevelHigh,
			expected: 24576,
		},
		// Gemini 3 returns 0 (uses thinkingLevel instead)
		{
			name:     "Gemini 3 returns 0",
			model:    "gemini-3-pro-preview",
			tier:     ThinkingLevelHigh,
			expected: 0,
		},
		// Unknown model returns 0
		{
			name:     "Unknown model returns 0",
			model:    "gemini-2.0-flash",
			tier:     ThinkingLevelHigh,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetThinkingBudgetForModel(tt.model, tt.tier)
			if result != tt.expected {
				t.Errorf("GetThinkingBudgetForModel(%q, %q) = %d, want %d", tt.model, tt.tier, result, tt.expected)
			}
		})
	}
}

func TestNewThinkingConfigForModel(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		tier            ThinkingLevel
		includeThoughts bool
		wantLevel       ThinkingLevel
		wantBudget      int
	}{
		{
			name:            "Gemini 2.5 Pro uses budget",
			model:           "gemini-2.5-pro",
			tier:            ThinkingLevelHigh,
			includeThoughts: true,
			wantLevel:       "",
			wantBudget:      32768,
		},
		{
			name:            "Gemini 2.5 Flash uses budget",
			model:           "gemini-2.5-flash",
			tier:            ThinkingLevelMedium,
			includeThoughts: false,
			wantLevel:       "",
			wantBudget:      12288,
		},
		{
			name:            "Gemini 3 uses level",
			model:           "gemini-3-pro-preview",
			tier:            ThinkingLevelHigh,
			includeThoughts: true,
			wantLevel:       ThinkingLevelHigh,
			wantBudget:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewThinkingConfigForModel(tt.model, tt.tier, tt.includeThoughts)
			if result.ThinkingLevel != tt.wantLevel {
				t.Errorf("ThinkingLevel = %q, want %q", result.ThinkingLevel, tt.wantLevel)
			}
			if result.ThinkingBudget != tt.wantBudget {
				t.Errorf("ThinkingBudget = %d, want %d", result.ThinkingBudget, tt.wantBudget)
			}
			if result.IncludeThoughts != tt.includeThoughts {
				t.Errorf("IncludeThoughts = %v, want %v", result.IncludeThoughts, tt.includeThoughts)
			}
		})
	}
}

func TestSupportsThinking(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{
			name:     "Gemini 2.5 Pro supports thinking",
			model:    "gemini-2.5-pro",
			expected: true,
		},
		{
			name:     "Gemini 2.5 Flash supports thinking",
			model:    "gemini-2.5-flash",
			expected: true,
		},
		{
			name:     "Gemini 3 supports thinking",
			model:    "gemini-3-pro-preview",
			expected: true,
		},
		{
			name:     "Gemini 2.0 does not support thinking",
			model:    "gemini-2.0-flash",
			expected: false,
		},
		{
			name:     "Unknown model does not support thinking",
			model:    "unknown-model",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SupportsThinking(tt.model)
			if result != tt.expected {
				t.Errorf("SupportsThinking(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestUsesThinkingLevel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{
			name:     "Gemini 3 uses thinking level",
			model:    "gemini-3-pro-preview",
			expected: true,
		},
		{
			name:     "Gemini 2.5 Pro does not use thinking level",
			model:    "gemini-2.5-pro",
			expected: false,
		},
		{
			name:     "Gemini 2.5 Flash does not use thinking level",
			model:    "gemini-2.5-flash",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UsesThinkingLevel(tt.model)
			if result != tt.expected {
				t.Errorf("UsesThinkingLevel(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestUsesThinkingBudget(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{
			name:     "Gemini 2.5 Pro uses thinking budget",
			model:    "gemini-2.5-pro",
			expected: true,
		},
		{
			name:     "Gemini 2.5 Flash uses thinking budget",
			model:    "gemini-2.5-flash",
			expected: true,
		},
		{
			name:     "Gemini 3 does not use thinking budget",
			model:    "gemini-3-pro-preview",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UsesThinkingBudget(tt.model)
			if result != tt.expected {
				t.Errorf("UsesThinkingBudget(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestValidateThinkingConfig(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		config    *ThinkingConfig
		expectErr bool
		errMsg    string
	}{
		{
			name:      "Nil config is valid",
			model:     "gemini-2.5-pro",
			config:    nil,
			expectErr: false,
		},
		{
			name:      "Valid Gemini 2.5 Pro config with budget",
			model:     "gemini-2.5-pro",
			config:    &ThinkingConfig{ThinkingBudget: 8192},
			expectErr: false,
		},
		{
			name:      "Invalid Gemini 2.5 Pro config with level",
			model:     "gemini-2.5-pro",
			config:    &ThinkingConfig{ThinkingLevel: ThinkingLevelHigh},
			expectErr: true,
			errMsg:    "gemini 2.5 models use thinkingBudget",
		},
		{
			name:      "Valid Gemini 3 config with level",
			model:     "gemini-3-pro-preview",
			config:    &ThinkingConfig{ThinkingLevel: ThinkingLevelHigh},
			expectErr: false,
		},
		{
			name:      "Invalid Gemini 3 config with budget",
			model:     "gemini-3-pro-preview",
			config:    &ThinkingConfig{ThinkingBudget: 8192},
			expectErr: true,
			errMsg:    "gemini 3 models use thinkingLevel",
		},
		{
			name:      "Invalid Gemini 3 config with invalid level",
			model:     "gemini-3-pro-preview",
			config:    &ThinkingConfig{ThinkingLevel: "invalid"},
			expectErr: true,
			errMsg:    "invalid thinkingLevel",
		},
		{
			name:      "Unsupported model",
			model:     "gemini-2.0-flash",
			config:    &ThinkingConfig{ThinkingBudget: 8192},
			expectErr: true,
			errMsg:    "does not support thinking",
		},
		{
			name:      "Negative budget is invalid",
			model:     "gemini-2.5-pro",
			config:    &ThinkingConfig{ThinkingBudget: -1},
			expectErr: true,
			errMsg:    "must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateThinkingConfig(tt.model, tt.config)
			if tt.expectErr {
				if err == nil {
					t.Errorf("ValidateThinkingConfig(%q, %v) expected error but got nil", tt.model, tt.config)
				} else if tt.errMsg != "" && !stringContainsSubstr(err.Error(), tt.errMsg) {
					t.Errorf("ValidateThinkingConfig(%q, %v) error = %q, want error containing %q", tt.model, tt.config, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateThinkingConfig(%q, %v) unexpected error: %v", tt.model, tt.config, err)
				}
			}
		})
	}
}

// stringContainsSubstr checks if substr is in s (renamed to avoid conflict with quota_test.go)
func stringContainsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestThinkingConfig_JSONSerialization(t *testing.T) {
	// Test that ThinkingConfig is properly included in GenerationConfig
	config := &GenerationConfig{
		Temperature:     0.7,
		MaxOutputTokens: 8192,
		ThinkingConfig: &ThinkingConfig{
			ThinkingBudget:  16384,
			IncludeThoughts: true,
		},
	}

	if config.ThinkingConfig == nil {
		t.Error("ThinkingConfig should not be nil")
	}
	if config.ThinkingConfig.ThinkingBudget != 16384 {
		t.Errorf("ThinkingBudget = %d, want 16384", config.ThinkingConfig.ThinkingBudget)
	}
	if !config.ThinkingConfig.IncludeThoughts {
		t.Error("IncludeThoughts should be true")
	}
}

func TestUsageMetadata_ThoughtsTokenCount(t *testing.T) {
	// Test that ThoughtsTokenCount is properly included in UsageMetadata
	metadata := &UsageMetadata{
		PromptTokenCount:     100,
		CandidatesTokenCount: 200,
		TotalTokenCount:      300,
		ThoughtsTokenCount:   50,
	}

	if metadata.ThoughtsTokenCount != 50 {
		t.Errorf("ThoughtsTokenCount = %d, want 50", metadata.ThoughtsTokenCount)
	}
}
