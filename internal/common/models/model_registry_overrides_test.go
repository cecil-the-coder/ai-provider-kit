package models

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelMetadataRegistry_GetMetadataWithFallback(t *testing.T) {
	registry := NewModelMetadataRegistry()

	// Register some provider-specific metadata
	registry.RegisterMetadata("provider-model", &ModelMetadata{
		DisplayName: "Provider Model",
		MaxTokens:   1000,
		Capabilities: ModelCapabilities{
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsVision:    false,
		},
	})

	t.Run("ProviderSpecificMetadata", func(t *testing.T) {
		// Should return provider-specific metadata first
		metadata := registry.GetMetadataWithFallback("provider-model")
		require.NotNil(t, metadata)
		assert.Equal(t, "Provider Model", metadata.DisplayName)
		assert.Equal(t, 1000, metadata.MaxTokens)
	})

	t.Run("FallbackToDefaults", func(t *testing.T) {
		// Should fall back to embedded defaults for models not in registry
		// Get a model from the defaults registry
		defaultsRegistry := GetDefaultsRegistry()
		providers := defaultsRegistry.GetAllProviders()
		require.NotEmpty(t, providers)

		providerID := providers[0]
		providerModels := defaultsRegistry.GetProviderModels(providerID)
		require.NotEmpty(t, providerModels)

		var testModelID string
		for id := range providerModels {
			testModelID = id
			break
		}

		metadata := registry.GetMetadataWithFallback(testModelID)
		assert.NotNil(t, metadata)
	})

	t.Run("NoMetadataAvailable", func(t *testing.T) {
		metadata := registry.GetMetadataWithFallback("completely-unknown-model-xyz")
		assert.Nil(t, metadata)
	})
}

func TestModelMetadataRegistry_GetMetadataWithOverrides(t *testing.T) {
	registry := NewModelMetadataRegistry()

	// Register base metadata
	registry.RegisterMetadata("test-model", &ModelMetadata{
		DisplayName: "Test Model",
		MaxTokens:   2000,
		Capabilities: ModelCapabilities{
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsVision:    false,
		},
	})

	t.Run("NoOverride", func(t *testing.T) {
		metadata := registry.GetMetadataWithOverrides("test-model", nil)
		require.NotNil(t, metadata)
		assert.Equal(t, 2000, metadata.MaxTokens)
		assert.True(t, metadata.Capabilities.SupportsTools)
	})

	t.Run("WithOverride", func(t *testing.T) {
		maxTokens := 4000
		supportsVision := true
		override := types.ModelCapabilityOverride{
			MaxTokens:      &maxTokens,
			SupportsVision: &supportsVision,
		}

		metadata := registry.GetMetadataWithOverrides("test-model", &override)
		require.NotNil(t, metadata)
		assert.Equal(t, 4000, metadata.MaxTokens)
		assert.True(t, metadata.Capabilities.SupportsVision)
		// Other capabilities should remain unchanged
		assert.True(t, metadata.Capabilities.SupportsTools)
		assert.True(t, metadata.Capabilities.SupportsStreaming)
	})

	t.Run("OverrideWithFallback", func(t *testing.T) {
		// For a model not in registry, should fall back to defaults and then apply override
		defaultsRegistry := GetDefaultsRegistry()
		providers := defaultsRegistry.GetAllProviders()
		require.NotEmpty(t, providers)

		providerID := providers[0]
		providerModels := defaultsRegistry.GetProviderModels(providerID)
		require.NotEmpty(t, providerModels)

		var testModelID string
		for id := range providerModels {
			testModelID = id
			break
		}

		maxTokens := 10000
		override := types.ModelCapabilityOverride{
			MaxTokens: &maxTokens,
		}

		metadata := registry.GetMetadataWithOverrides(testModelID, &override)
		require.NotNil(t, metadata)
		assert.Equal(t, 10000, metadata.MaxTokens)
	})
}

func TestModelMetadataRegistry_EnrichModelWithOverrides(t *testing.T) {
	registry := NewModelMetadataRegistry()

	// Register metadata
	registry.RegisterMetadata("test-model", &ModelMetadata{
		DisplayName: "Test Model",
		MaxTokens:   2000,
		Description: "A test model",
		Capabilities: ModelCapabilities{
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsVision:    false,
		},
	})

	t.Run("NoOverrides", func(t *testing.T) {
		model := &types.Model{
			ID:       "test-model",
			Provider: types.ProviderTypeOpenAI,
		}

		enriched := registry.EnrichModelWithOverrides(model, nil)
		require.NotNil(t, enriched)
		assert.Equal(t, "Test Model", enriched.Name)
		assert.Equal(t, 2000, enriched.MaxTokens)
		assert.Equal(t, "A test model", enriched.Description)
		assert.True(t, enriched.SupportsToolCalling)
		assert.True(t, enriched.SupportsStreaming)
	})

	t.Run("WithOverrides", func(t *testing.T) {
		model := &types.Model{
			ID:       "test-model",
			Provider: types.ProviderTypeOpenAI,
		}

		maxTokens := 5000
		supportsTools := false
		overrides := map[string]types.ModelCapabilityOverride{
			"test-model": {
				MaxTokens:     &maxTokens,
				SupportsTools: &supportsTools,
			},
		}

		enriched := registry.EnrichModelWithOverrides(model, overrides)
		require.NotNil(t, enriched)
		assert.Equal(t, 5000, enriched.MaxTokens)
		assert.False(t, enriched.SupportsToolCalling)
		// Streaming should remain from base metadata
		assert.True(t, enriched.SupportsStreaming)
	})

	t.Run("OverrideForDifferentModel", func(t *testing.T) {
		model := &types.Model{
			ID:       "test-model",
			Provider: types.ProviderTypeOpenAI,
		}

		maxTokens := 5000
		overrides := map[string]types.ModelCapabilityOverride{
			"other-model": {
				MaxTokens: &maxTokens,
			},
		}

		enriched := registry.EnrichModelWithOverrides(model, overrides)
		require.NotNil(t, enriched)
		// Should use base metadata, not the override
		assert.Equal(t, 2000, enriched.MaxTokens)
	})

	t.Run("NilModel", func(t *testing.T) {
		enriched := registry.EnrichModelWithOverrides(nil, nil)
		assert.Nil(t, enriched)
	})
}

func TestModelMetadataRegistry_EnrichModelsWithOverrides(t *testing.T) {
	registry := NewModelMetadataRegistry()

	// Register metadata
	registry.RegisterMetadata("model-1", &ModelMetadata{
		DisplayName: "Model 1",
		MaxTokens:   1000,
		Capabilities: ModelCapabilities{
			SupportsTools:     true,
			SupportsStreaming: true,
		},
	})

	registry.RegisterMetadata("model-2", &ModelMetadata{
		DisplayName: "Model 2",
		MaxTokens:   2000,
		Capabilities: ModelCapabilities{
			SupportsTools:     false,
			SupportsStreaming: true,
		},
	})

	t.Run("MultipleModelsWithOverrides", func(t *testing.T) {
		models := []types.Model{
			{ID: "model-1", Provider: types.ProviderTypeOpenAI},
			{ID: "model-2", Provider: types.ProviderTypeOpenAI},
		}

		maxTokens1 := 1500
		maxTokens2 := 2500
		supportsTools2 := true
		overrides := map[string]types.ModelCapabilityOverride{
			"model-1": {
				MaxTokens: &maxTokens1,
			},
			"model-2": {
				MaxTokens:     &maxTokens2,
				SupportsTools: &supportsTools2,
			},
		}

		enriched := registry.EnrichModelsWithOverrides(models, overrides)
		require.Len(t, enriched, 2)

		// Model 1 should have override applied
		assert.Equal(t, "Model 1", enriched[0].Name)
		assert.Equal(t, 1500, enriched[0].MaxTokens)
		assert.True(t, enriched[0].SupportsToolCalling)

		// Model 2 should have overrides applied
		assert.Equal(t, "Model 2", enriched[1].Name)
		assert.Equal(t, 2500, enriched[1].MaxTokens)
		assert.True(t, enriched[1].SupportsToolCalling)
	})

	t.Run("EmptyModels", func(t *testing.T) {
		enriched := registry.EnrichModelsWithOverrides([]types.Model{}, nil)
		assert.Empty(t, enriched)
	})
}

func TestCapabilityPrecedence(t *testing.T) {
	// This test verifies the precedence order:
	// 1. User overrides (highest)
	// 2. Provider API response
	// 3. Embedded defaults
	// 4. Name inference (lowest)

	registry := NewModelMetadataRegistry()

	t.Run("UserOverrideTakesPrecedence", func(t *testing.T) {
		// Register provider-specific metadata
		registry.RegisterMetadata("precedence-test", &ModelMetadata{
			DisplayName: "Provider Metadata",
			MaxTokens:   2000,
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: true,
				SupportsVision:    false,
			},
		})

		model := &types.Model{
			ID:                  "precedence-test",
			Provider:            types.ProviderTypeOpenAI,
			MaxTokens:           3000, // From API response
			SupportsToolCalling: true,
		}

		// User override should take precedence over everything
		maxTokens := 5000
		supportsTools := false
		overrides := map[string]types.ModelCapabilityOverride{
			"precedence-test": {
				MaxTokens:     &maxTokens,
				SupportsTools: &supportsTools,
			},
		}

		enriched := registry.EnrichModelWithOverrides(model, overrides)
		require.NotNil(t, enriched)

		// User override should win
		assert.Equal(t, 5000, enriched.MaxTokens)
		assert.False(t, enriched.SupportsToolCalling)
	})

	t.Run("ProviderMetadataTakesPrecedenceOverDefaults", func(t *testing.T) {
		// Register provider-specific metadata that differs from defaults
		registry.RegisterMetadata("provider-override-test", &ModelMetadata{
			DisplayName: "Provider Override",
			MaxTokens:   9999,
			Capabilities: ModelCapabilities{
				SupportsTools:     true,
				SupportsStreaming: false,
			},
		})

		metadata := registry.GetMetadataWithFallback("provider-override-test")
		require.NotNil(t, metadata)

		// Should use provider metadata, not defaults
		assert.Equal(t, "Provider Override", metadata.DisplayName)
		assert.Equal(t, 9999, metadata.MaxTokens)
	})
}
