package racing

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Virtual Model Tests
// ============================================================================

func TestVirtualModels_GetModels_EmptyConfig(t *testing.T) {
	config := &Config{} // No virtual models configured
	rp := NewRacingProvider("test", config)
	ctx := context.Background()

	models, err := rp.GetModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}

	defaultModel := rp.GetDefaultModel()
	if defaultModel != "" {
		t.Errorf("expected empty default model, got '%s'", defaultModel)
	}
}

func TestVirtualModels_GetModels_WithValidation(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		expectedModels  int
		expectedDefault string
		shouldErr       bool
	}{
		{
			name: "single virtual model",
			config: &Config{
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Racing Model",
						Description: "The fastest virtual model",
						Providers: []ProviderReference{
							{Name: "provider1"},
							{Name: "provider2"},
						},
					},
				},
			},
			expectedModels:  1,
			expectedDefault: "fast-model",
		},
		{
			name: "multiple virtual models with explicit default",
			config: &Config{
				DefaultVirtualModel: "quality-model",
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Racing Model",
						Description: "The fastest virtual model",
					},
					"quality-model": {
						DisplayName: "Quality Racing Model",
						Description: "The highest quality virtual model",
					},
					"balanced-model": {
						DisplayName: "Balanced Racing Model",
						Description: "Balanced speed and quality",
					},
				},
			},
			expectedModels:  3,
			expectedDefault: "quality-model",
		},
		{
			name: "invalid default model",
			config: &Config{
				DefaultVirtualModel: "nonexistent",
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Racing Model",
						Description: "The fastest virtual model",
					},
				},
			},
			expectedModels:  1,
			expectedDefault: "fast-model", // Should fall back to first available
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp := NewRacingProvider("test", tt.config)
			ctx := context.Background()

			models, err := rp.GetModels(ctx)
			if tt.shouldErr && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(models) != tt.expectedModels {
				t.Errorf("expected %d models, got %d", tt.expectedModels, len(models))
			}

			defaultModel := rp.GetDefaultModel()
			if defaultModel != tt.expectedDefault {
				t.Errorf("expected default model '%s', got '%s'", tt.expectedDefault, defaultModel)
			}

			// Validate model properties
			for _, model := range models {
				if model.Provider != "racing" {
					t.Errorf("expected provider 'racing', got '%s'", model.Provider)
				}
				if model.ID == "" {
					t.Error("expected non-empty model ID")
				}
				if model.Name == "" {
					t.Error("expected non-empty model name")
				}
			}
		})
	}
}

func TestVirtualModels_ModelValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		shouldErr   bool
		errContains string
	}{
		{
			name: "valid config",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Description: "A valid virtual model",
						Providers: []ProviderReference{
							{Name: "provider1", Model: "model1"},
						},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "missing default virtual model",
			config: &Config{
				TimeoutMS: 5000,
				VirtualModels: map[string]VirtualModelConfig{
					"model1": {
						DisplayName: "Model 1",
						Providers:   []ProviderReference{{Name: "provider1"}},
					},
				},
			},
			shouldErr:   true,
			errContains: "default_virtual_model",
		},
		{
			name: "no virtual models",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels:       map[string]VirtualModelConfig{},
			},
			shouldErr:   true,
			errContains: "at least one virtual model",
		},
		{
			name: "virtual model with no providers",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Providers:   []ProviderReference{}, // Empty providers
					},
				},
			},
			shouldErr:   true,
			errContains: "must have at least one provider",
		},
		{
			name: "provider with empty name",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Providers: []ProviderReference{
							{Name: ""}, // Empty provider name
						},
					},
				},
			},
			shouldErr:   true,
			errContains: "provider name cannot be empty",
		},
		{
			name: "negative timeout",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Providers:   []ProviderReference{{Name: "provider1"}},
						TimeoutMS:   -100, // Negative timeout
					},
				},
			},
			shouldErr:   true,
			errContains: "must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.shouldErr {
				if err == nil {
					t.Fatal("expected validation error but got none")
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain '%s', got: %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestVirtualModel_DifferentStrategies(t *testing.T) {
	tests := []struct {
		name             string
		virtualModel     string
		expectedStrategy Strategy
		expectedWinner   string
		providers        []types.Provider
	}{
		{
			name:             "first_wins strategy",
			virtualModel:     "fast-model",
			expectedStrategy: StrategyFirstWins,
			expectedWinner:   "fast-provider",
			providers: []types.Provider{
				&mockChatProvider{
					name:     "slow-provider",
					delay:    200 * time.Millisecond,
					response: "slow response",
				},
				&mockChatProvider{
					name:     "fast-provider",
					delay:    10 * time.Millisecond,
					response: "fast response",
				},
			},
		},
		{
			name:             "weighted strategy with history",
			virtualModel:     "quality-model",
			expectedStrategy: StrategyWeighted,
			// StrategyWeighted waits for all providers then does weighted selection
			// quality-provider wins due to better performance history
			expectedWinner: "quality-provider",
			providers: []types.Provider{
				&mockChatProvider{
					name:     "fast-provider",
					delay:    10 * time.Millisecond,
					response: "fast response",
				},
				&mockChatProvider{
					name:     "quality-provider",
					delay:    50 * time.Millisecond,
					response: "quality response",
				},
			},
		},
		{
			name:             "quality strategy",
			virtualModel:     "balanced-model",
			expectedStrategy: StrategyQuality,
			expectedWinner:   "fast-provider", // Should pick based on adjusted score
			providers: []types.Provider{
				&mockChatProvider{
					name:     "fast-provider",
					delay:    10 * time.Millisecond,
					response: "fast response",
				},
				&mockChatProvider{
					name:     "slow-provider",
					delay:    100 * time.Millisecond,
					response: "slow response",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				DefaultVirtualModel: "fast-model",
				TimeoutMS:           5000,
				GracePeriodMS:       100,
				Strategy:            StrategyFirstWins, // Default strategy
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Model",
						Description: "Fast virtual model",
						Strategy:    StrategyFirstWins,
						TimeoutMS:   5000,
						Providers: []ProviderReference{
							{Name: "slow-provider"},
							{Name: "fast-provider"},
						},
					},
					"quality-model": {
						DisplayName: "Quality Model",
						Description: "Quality virtual model",
						Strategy:    StrategyWeighted,
						TimeoutMS:   5000,
						Providers: []ProviderReference{
							{Name: "fast-provider"},
							{Name: "quality-provider"},
						},
					},
					"balanced-model": {
						DisplayName: "Balanced Model",
						Description: "Balanced virtual model",
						Strategy:    StrategyQuality,
						TimeoutMS:   5000,
						Providers: []ProviderReference{
							{Name: "fast-provider"},
							{Name: "slow-provider"},
						},
					},
				},
			}

			rp := NewRacingProvider("test", config)
			rp.SetProviders(tt.providers)

			// Pre-seed performance history for weighted strategy test
			if tt.expectedStrategy == StrategyWeighted {
				rp.performance.RecordWin("quality-provider", 50*time.Millisecond)
				rp.performance.RecordWin("quality-provider", 60*time.Millisecond)
				rp.performance.RecordLoss("fast-provider", 10*time.Millisecond)
			}

			ctx := context.Background()
			opts := types.GenerateOptions{
				Model:  tt.virtualModel,
				Prompt: "test",
			}

			stream, err := rp.GenerateChatCompletion(ctx, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = stream.Close() }()

			chunk, err := stream.Next()
			if err != nil && err != io.EOF {
				t.Fatalf("unexpected error reading chunk: %v", err)
			}

			winner, ok := chunk.Metadata["racing_winner"].(string)
			if !ok {
				t.Fatal("expected racing_winner in metadata")
			}

			if winner != tt.expectedWinner {
				t.Errorf("expected winner '%s', got '%s'", tt.expectedWinner, winner)
			}

			// Note: Virtual model metadata is only sent to metrics collector, not included in response metadata
			// Only racing metadata should be in the response
		})
	}
}

func TestVirtualModel_PerVirtualModelTimeouts(t *testing.T) {
	tests := []struct {
		name          string
		virtualModel  string
		timeoutMS     int
		shouldTimeout bool
		providerDelay time.Duration
	}{
		{
			name:          "short timeout should fail",
			virtualModel:  "fast-model",
			timeoutMS:     100, // Very short timeout
			shouldTimeout: true,
			providerDelay: 500 * time.Millisecond,
		},
		{
			name:          "long timeout should succeed",
			virtualModel:  "slow-model",
			timeoutMS:     2000, // Longer timeout
			shouldTimeout: false,
			providerDelay: 100 * time.Millisecond,
		},
		{
			name:          "default timeout fallback",
			virtualModel:  "default-model",
			timeoutMS:     0, // Use default
			shouldTimeout: false,
			providerDelay: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				DefaultVirtualModel: "default-model",
				TimeoutMS:           1000, // Default timeout
				Strategy:            StrategyFirstWins,
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Model",
						TimeoutMS:   tt.timeoutMS,
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
					"slow-model": {
						DisplayName: "Slow Model",
						TimeoutMS:   tt.timeoutMS,
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
					"default-model": {
						DisplayName: "Default Model",
						// No timeout specified, should use default
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
				},
			}

			providers := []types.Provider{
				&mockChatProvider{
					name:     "provider1",
					delay:    tt.providerDelay,
					response: "response",
				},
			}

			rp := NewRacingProvider("test", config)
			rp.SetProviders(providers)

			ctx := context.Background()
			opts := types.GenerateOptions{
				Model:  tt.virtualModel,
				Prompt: "test",
			}

			start := time.Now()
			stream, err := rp.GenerateChatCompletion(ctx, opts)
			elapsed := time.Since(start)

			if tt.shouldTimeout {
				if err == nil {
					t.Fatal("expected timeout error but got none")
				}
				if elapsed > 500*time.Millisecond {
					t.Errorf("timeout took too long: %v", elapsed)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if stream != nil {
					_ = stream.Close()
				}
			}
		})
	}
}

func TestVirtualModel_ProviderReferences(t *testing.T) {
	tests := []struct {
		name              string
		virtualModel      string
		providers         []ProviderReference
		expectedProviders []string
		shouldErr         bool
		errContains       string
	}{
		{
			name:         "valid provider references",
			virtualModel: "multi-provider",
			providers: []ProviderReference{
				{Name: "provider1", Model: "model1", Priority: 1},
				{Name: "provider2", Model: "model2", Priority: 2},
				{Name: "provider3", Model: "model3"}, // No priority
			},
			expectedProviders: []string{"provider1", "provider2", "provider3"},
		},
		{
			name:         "missing provider",
			virtualModel: "missing-provider",
			providers: []ProviderReference{
				{Name: "nonexistent-provider", Model: "model1"},
			},
			shouldErr:   true,
			errContains: "provider not found",
		},
		{
			name:         "empty provider list",
			virtualModel: "empty-providers",
			providers:    []ProviderReference{},
			shouldErr:    true,
			errContains:  "no providers configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				DefaultVirtualModel: "multi-provider",
				TimeoutMS:           5000,
				Strategy:            StrategyFirstWins,
				VirtualModels: map[string]VirtualModelConfig{
					"multi-provider": {
						DisplayName: "Multi Provider Model",
						Providers: []ProviderReference{
							{Name: "provider1", Model: "model1", Priority: 1},
							{Name: "provider2", Model: "model2", Priority: 2},
							{Name: "provider3", Model: "model3"},
						},
					},
					"missing-provider": {
						DisplayName: "Missing Provider Model",
						Providers: []ProviderReference{
							{Name: "nonexistent-provider", Model: "model1"},
						},
					},
					"empty-providers": {
						DisplayName: "Empty Providers Model",
						Providers:   []ProviderReference{},
					},
				},
			}

			// Set up available providers
			availableProviders := []types.Provider{
				&mockChatProvider{name: "provider1", response: "response1"},
				&mockChatProvider{name: "provider2", response: "response2"},
				&mockChatProvider{name: "provider3", response: "response3"},
			}

			rp := NewRacingProvider("test", config)
			rp.SetProviders(availableProviders)

			ctx := context.Background()
			opts := types.GenerateOptions{
				Model:  tt.virtualModel,
				Prompt: "test",
			}

			stream, err := rp.GenerateChatCompletion(ctx, opts)

			if tt.shouldErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain '%s', got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if stream != nil {
					_ = stream.Close()
				}
			}
		})
	}
}

func TestVirtualModel_FallbackToDefaults(t *testing.T) {
	config := &Config{
		DefaultVirtualModel: "default",
		TimeoutMS:           5000,
		GracePeriodMS:       1000,
		Strategy:            StrategyWeighted, // Default strategy
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				Description: "Uses default configuration",
				// No strategy or timeout specified, should use defaults
				Providers: []ProviderReference{
					{Name: "provider1"},
					{Name: "provider2"},
				},
			},
			"custom": {
				DisplayName: "Custom Model",
				Description: "Custom configuration",
				Strategy:    StrategyFirstWins, // Override default
				TimeoutMS:   2000,              // Override default
				Providers: []ProviderReference{
					{Name: "provider1"},
					{Name: "provider2"},
				},
			},
		},
	}

	providers := []types.Provider{
		&mockChatProvider{
			name:     "provider1",
			delay:    10 * time.Millisecond,
			response: "response from provider1",
		},
		&mockChatProvider{
			name:     "provider2",
			delay:    50 * time.Millisecond,
			response: "response from provider2",
		},
	}

	rp := NewRacingProvider("test", config)
	rp.SetProviders(providers)

	// Test default virtual model (should use default strategy)
	t.Run("default model uses default strategy", func(t *testing.T) {
		ctx := context.Background()
		opts := types.GenerateOptions{
			Model:  "default",
			Prompt: "test",
		}

		stream, err := rp.GenerateChatCompletion(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = stream.Close() }()

		chunk, _ := stream.Next()
		winner := chunk.Metadata["racing_winner"].(string)

		// Should use weighted strategy (default)
		if winner != "provider1" && winner != "provider2" {
			t.Errorf("expected valid winner, got '%s'", winner)
		}
	})

	// Test custom virtual model (should use custom strategy)
	t.Run("custom model uses custom strategy", func(t *testing.T) {
		ctx := context.Background()
		opts := types.GenerateOptions{
			Model:  "custom",
			Prompt: "test",
		}

		stream, err := rp.GenerateChatCompletion(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = stream.Close() }()

		chunk, _ := stream.Next()
		winner := chunk.Metadata["racing_winner"].(string)

		// Should use first_wins strategy, so provider1 should win
		if winner != "provider1" {
			t.Errorf("expected provider1 to win with first_wins strategy, got '%s'", winner)
		}
	})
}

func TestVirtualModel_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		configFunc  func() *Config
		shouldErr   bool
		errContains string
	}{
		{
			name: "valid complete configuration",
			configFunc: func() *Config {
				return &Config{
					TimeoutMS:           5000,
					DefaultVirtualModel: "complete",
					VirtualModels: map[string]VirtualModelConfig{
						"complete": {
							DisplayName: "Complete Model",
							Description: "Fully configured virtual model",
							Strategy:    StrategyWeighted,
							TimeoutMS:   3000,
							Providers: []ProviderReference{
								{Name: "provider1", Model: "model1", Priority: 1},
								{Name: "provider2", Model: "model2", Priority: 2},
							},
						},
					},
				}
			},
			shouldErr: false,
		},
		{
			name: "minimal valid configuration",
			configFunc: func() *Config {
				return &Config{
					TimeoutMS:           5000,
					DefaultVirtualModel: "minimal",
					VirtualModels: map[string]VirtualModelConfig{
						"minimal": {
							DisplayName: "Minimal Model",
							Providers: []ProviderReference{
								{Name: "provider1"},
							},
						},
					},
				}
			},
			shouldErr: false,
		},
		{
			name: "missing default virtual model reference",
			configFunc: func() *Config {
				return &Config{
					TimeoutMS:           5000,
					DefaultVirtualModel: "nonexistent",
					VirtualModels: map[string]VirtualModelConfig{
						"existing": {
							DisplayName: "Existing Model",
							Providers: []ProviderReference{
								{Name: "provider1"},
							},
						},
					},
				}
			},
			shouldErr:   true,
			errContains: "must reference an existing virtual model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.configFunc()
			err := config.Validate()

			if tt.shouldErr {
				if err == nil {
					t.Fatal("expected validation error but got none")
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain '%s', got: %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}
