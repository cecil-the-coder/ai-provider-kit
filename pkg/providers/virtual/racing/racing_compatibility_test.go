package racing

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Backward Compatibility Tests
// ============================================================================

func TestBackwardCompatibility_OldConfigurationFormat(t *testing.T) {
	// Test old format without virtual models
	oldConfig := &Config{
		TimeoutMS:     3000,
		GracePeriodMS: 500,
		Strategy:      StrategyFirstWins,
		ProviderNames: []string{"provider1", "provider2"},
		// No virtual models configured
	}

	rp := NewRacingProvider("old-format", oldConfig)

	// Test that old configuration still works for basic functionality
	if rp.config.TimeoutMS != 3000 {
		t.Errorf("expected timeout 3000, got %d", rp.config.TimeoutMS)
	}

	if rp.config.Strategy != StrategyFirstWins {
		t.Errorf("expected strategy FirstWins, got %s", rp.config.Strategy)
	}

	// Test GetModels with no virtual models
	ctx := context.Background()
	models, err := rp.GetModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("expected 0 models for old format, got %d", len(models))
	}

	defaultModel := rp.GetDefaultModel()
	if defaultModel != "" {
		t.Errorf("expected empty default model for old format, got '%s'", defaultModel)
	}
}

func TestBackwardCompatibility_ConfigMigration(t *testing.T) {
	tests := []struct {
		name             string
		oldConfig        *Config
		expectedBehavior string
	}{
		{
			name: "old config with provider names",
			oldConfig: &Config{
				TimeoutMS:     2000,
				GracePeriodMS: 200,
				Strategy:      StrategyWeighted,
				ProviderNames: []string{"provider1", "provider2"},
			},
			expectedBehavior: "should preserve old settings",
		},
		{
			name: "config with both old and new format",
			oldConfig: &Config{
				TimeoutMS:           2000,
				Strategy:            StrategyFirstWins,
				ProviderNames:       []string{"provider1"},
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
				},
			},
			expectedBehavior: "should use new virtual models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp := NewRacingProvider("migration-test", tt.oldConfig)

			// Verify basic settings are preserved
			if rp.config.TimeoutMS != tt.oldConfig.TimeoutMS {
				t.Errorf("expected timeout %d, got %d", tt.oldConfig.TimeoutMS, rp.config.TimeoutMS)
			}

			if rp.config.Strategy != tt.oldConfig.Strategy {
				t.Errorf("expected strategy %s, got %s", tt.oldConfig.Strategy, rp.config.Strategy)
			}

			// Test behavior based on configuration type
			if len(tt.oldConfig.VirtualModels) > 0 {
				// New format - should have virtual models
				ctx := context.Background()
				models, err := rp.GetModels(ctx)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if len(models) == 0 {
					t.Error("expected virtual models to be available")
				}
			} else {
				// Old format - should not have virtual models
				ctx := context.Background()
				models, err := rp.GetModels(ctx)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if len(models) != 0 {
					t.Errorf("expected no virtual models for old format, got %d", len(models))
				}
			}
		})
	}
}

func TestBackwardCompatibility_ProviderSetters(t *testing.T) {
	// Test that SetProviders still works with old configuration
	config := &Config{
		TimeoutMS:     5000,
		Strategy:      StrategyFirstWins,
		ProviderNames: []string{"provider1", "provider2"},
	}

	rp := NewRacingProvider("backward-compat", config)

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "response1"},
		&mockChatProvider{name: "provider2", response: "response2"},
	}

	// This should still work even with old config format
	rp.SetProviders(providers)

	// Verify providers were set
	if len(rp.providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(rp.providers))
	}

	// Test that we can still make requests without virtual models (legacy mode)
	ctx := context.Background()
	opts := types.GenerateOptions{
		// No model specified - old format behavior (legacy mode)
		Prompt: "test",
	}

	// This should work in legacy mode when no virtual models are configured
	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error in legacy mode: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Verify we get a response
	chunk, err := stream.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error reading chunk: %v", err)
	}

	// Verify racing metadata is present
	if chunk.Metadata["racing_winner"] == nil {
		t.Error("expected racing_winner in metadata")
	}

	// Verify virtual model metadata shows legacy mode
	if chunk.Metadata["virtual_model"] != "legacy_mode" {
		t.Errorf("expected virtual_model to be 'legacy_mode', got %v", chunk.Metadata["virtual_model"])
	}
}

func TestBackwardCompatibility_ConfigMethods(t *testing.T) {
	config := &Config{
		TimeoutMS:     3000,
		GracePeriodMS: 300,
		Strategy:      StrategyWeighted,
		ProviderNames: []string{"provider1", "provider2"},
	}

	rp := NewRacingProvider("config-test", config)

	// Test GetConfig with old format
	providerConfig := rp.GetConfig()
	if providerConfig.Type != "racing" {
		t.Errorf("expected type 'racing', got '%s'", providerConfig.Type)
	}

	if providerConfig.Name != "config-test" {
		t.Errorf("expected name 'config-test', got '%s'", providerConfig.Name)
	}

	// Test Configure with old format
	newConfig := types.ProviderConfig{
		Type: "racing",
		Name: "config-test",
		ProviderConfig: map[string]interface{}{
			"timeout_ms":      4000,
			"grace_period_ms": 400,
			"strategy":        "first_wins",
			"providers":       []string{"provider1", "provider2", "provider3"},
		},
	}

	err := rp.Configure(newConfig)
	if err != nil {
		t.Fatalf("unexpected error configuring: %v", err)
	}

	// Verify configuration was updated
	if rp.config.TimeoutMS != 4000 {
		t.Errorf("expected updated timeout 4000, got %d", rp.config.TimeoutMS)
	}

	if rp.config.Strategy != StrategyFirstWins {
		t.Errorf("expected updated strategy FirstWins, got %s", rp.config.Strategy)
	}
}

func TestBackwardCompatibility_PerformanceTracking(t *testing.T) {
	// Test that performance tracking still works as before
	config := &Config{
		TimeoutMS:     5000,
		Strategy:      StrategyFirstWins,
		ProviderNames: []string{"provider1", "provider2"},
	}

	rp := NewRacingProvider("perf-test", config)

	// Performance tracking should be available
	stats := rp.GetPerformanceStats()
	if stats == nil {
		t.Error("expected performance stats to be available")
	}

	// Should be able to record wins/losses
	rp.performance.RecordWin("provider1", 100*time.Millisecond)
	rp.performance.RecordLoss("provider2", 200*time.Millisecond)

	stats = rp.GetPerformanceStats()
	if stats["provider1"].Wins != 1 {
		t.Errorf("expected provider1 to have 1 win, got %d", stats["provider1"].Wins)
	}

	if stats["provider2"].Losses != 1 {
		t.Errorf("expected provider2 to have 1 loss, got %d", stats["provider2"].Losses)
	}
}

func TestBackwardCompatibility_ErrorHandling(t *testing.T) {
	config := &Config{
		TimeoutMS:     1000,
		Strategy:      StrategyFirstWins,
		ProviderNames: []string{"provider1"},
	}

	rp := NewRacingProvider("error-test", config)
	rp.SetProviders([]types.Provider{
		&mockChatProvider{name: "provider1", err: errors.New("provider error")},
	})

	ctx := context.Background()
	opts := types.GenerateOptions{
		Model:  "nonexistent-model", // Should use legacy mode when no virtual models configured
		Prompt: "test",
	}

	// Should handle provider errors gracefully in legacy mode
	_, err := rp.GenerateChatCompletion(ctx, opts)
	if err == nil {
		t.Fatal("expected error when provider fails")
	}

	// Error should indicate failure, message varies by strategy implementation
	// StrategyFirstWins (returns fast) uses "no successful candidates"
	// StrategyWeighted (waits for all) uses "all providers failed"
	errStr := err.Error()
	if !containsString(errStr, "all providers failed") && !containsString(errStr, "no successful candidates") {
		t.Errorf("expected error to indicate failure, got: %v", err)
	}
}

// TestBackwardCompatibility_MixedUsage tests scenarios where old and new configurations
// might be mixed during migration
func TestBackwardCompatibility_MixedUsage(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() *Config
		testFunc    func(*RacingProvider)
	}{
		{
			name: "old config with new virtual model added later",
			setupConfig: func() *Config {
				return &Config{
					TimeoutMS:     3000,
					GracePeriodMS: 300,
					Strategy:      StrategyFirstWins,
					ProviderNames: []string{"provider1"},
				}
			},
			testFunc: func(rp *RacingProvider) {
				// Add virtual model configuration at runtime
				rp.config.VirtualModels = map[string]VirtualModelConfig{
					"new-model": {
						DisplayName: "New Virtual Model",
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
				}
				rp.config.DefaultVirtualModel = "new-model"

				// Should now work with virtual models
				ctx := context.Background()
				models, err := rp.GetModels(ctx)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if len(models) != 1 {
					t.Errorf("expected 1 model after adding virtual model, got %d", len(models))
				}
			},
		},
		{
			name: "new config used with old-style provider setting",
			setupConfig: func() *Config {
				return &Config{
					DefaultVirtualModel: "modern",
					TimeoutMS:           5000,
					VirtualModels: map[string]VirtualModelConfig{
						"modern": {
							DisplayName: "Modern Model",
							Providers: []ProviderReference{
								{Name: "provider1"},
								{Name: "provider2"},
							},
						},
					},
				}
			},
			testFunc: func(rp *RacingProvider) {
				// Use old SetProviders method
				providers := []types.Provider{
					&mockChatProvider{name: "provider1", response: "response1"},
					&mockChatProvider{name: "provider2", response: "response2"},
				}
				rp.SetProviders(providers)

				// Should work with new virtual model format
				ctx := context.Background()
				opts := types.GenerateOptions{
					Model:  "modern",
					Prompt: "test",
				}

				stream, err := rp.GenerateChatCompletion(ctx, opts)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				_ = stream.Close()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.setupConfig()
			rp := NewRacingProvider("mixed-test", config)
			tt.testFunc(rp)
		})
	}
}
