package racing

import (
	"testing"
)

// ============================================================================
// Configuration Tests
// ============================================================================

func TestConfig_DefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.TimeoutMS != 5000 {
		t.Errorf("expected default timeout 5000, got %d", config.TimeoutMS)
	}

	if config.GracePeriodMS != 1000 {
		t.Errorf("expected default grace period 1000, got %d", config.GracePeriodMS)
	}

	if config.Strategy != StrategyFirstWins {
		t.Errorf("expected default strategy '%s', got '%s'", StrategyFirstWins, config.Strategy)
	}

	if config.DefaultVirtualModel != "default" {
		t.Errorf("expected default virtual model 'default', got '%s'", config.DefaultVirtualModel)
	}

	if len(config.VirtualModels) != 1 {
		t.Errorf("expected 1 virtual model, got %d", len(config.VirtualModels))
	}

	defaultVM, exists := config.VirtualModels["default"]
	if !exists {
		t.Fatal("expected default virtual model to exist")
	}

	if defaultVM.DisplayName != "Default Racing Model" {
		t.Errorf("expected display name 'Default Racing Model', got '%s'", defaultVM.DisplayName)
	}
}

func TestConfig_ResolveVirtualModelConfig(t *testing.T) {
	baseConfig := &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"partial": {
				DisplayName: "Partial Model",
				Strategy:    StrategyWeighted, // Override strategy
				// No timeout, should use default
				Providers: []ProviderReference{{Name: "provider1"}},
			},
			"complete": {
				DisplayName: "Complete Model",
				Strategy:    StrategyQuality,
				TimeoutMS:   2000,
				Providers:   []ProviderReference{{Name: "provider1"}},
			},
		},
	}

	tests := []struct {
		name             string
		modelID          string
		expectedStrategy Strategy
		expectedTimeout  int
		shouldErr        bool
	}{
		{
			name:             "partial config with strategy override",
			modelID:          "partial",
			expectedStrategy: StrategyWeighted,
			expectedTimeout:  5000, // Should use default
		},
		{
			name:             "complete config",
			modelID:          "complete",
			expectedStrategy: StrategyQuality,
			expectedTimeout:  2000,
		},
		{
			name:      "nonexistent model",
			modelID:   "nonexistent",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vmConfig, err := baseConfig.resolveVirtualModelConfig(tt.modelID)

			if tt.shouldErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if vmConfig.Strategy != tt.expectedStrategy {
				t.Errorf("expected strategy '%s', got '%s'", tt.expectedStrategy, vmConfig.Strategy)
			}

			if vmConfig.TimeoutMS != tt.expectedTimeout {
				t.Errorf("expected timeout %d, got %d", tt.expectedTimeout, vmConfig.TimeoutMS)
			}

			// Verify we get a copy, not a reference
			vmConfig.Strategy = StrategyQuality
			original, _ := baseConfig.resolveVirtualModelConfig(tt.modelID)
			if original.Strategy == StrategyQuality && tt.expectedStrategy != StrategyQuality {
				t.Error("resolveVirtualModelConfig should return a copy, not modify original")
			}
		})
	}
}
