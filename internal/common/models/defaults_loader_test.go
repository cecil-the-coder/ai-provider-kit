package models

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultsRegistry(t *testing.T) {
	registry := GetDefaultsRegistry()
	require.NotNil(t, registry)

	// Should return the same instance (singleton)
	registry2 := GetDefaultsRegistry()
	assert.Equal(t, registry, registry2)

	// Should have loaded providers
	providers := registry.GetAllProviders()
	assert.NotEmpty(t, providers, "Should have loaded providers from defaults.json")
}

func TestDefaultsRegistry_GetModelDefaults(t *testing.T) {
	registry := GetDefaultsRegistry()

	t.Run("ExistingModel", func(t *testing.T) {
		// Try to find any model from the defaults
		providers := registry.GetAllProviders()
		require.NotEmpty(t, providers)

		// Get models from first provider
		providerID := providers[0]
		providerModels := registry.GetProviderModels(providerID)
		require.NotEmpty(t, providerModels)

		// Get metadata for first model
		var testModelID string
		for id := range providerModels {
			testModelID = id
			break
		}

		metadata := registry.GetModelDefaults(testModelID)
		assert.NotNil(t, metadata)
		assert.NotEmpty(t, metadata.DisplayName)
		assert.Greater(t, metadata.MaxTokens, 0)
	})

	t.Run("NonExistingModel", func(t *testing.T) {
		metadata := registry.GetModelDefaults("non-existent-model-xyz-123")
		assert.Nil(t, metadata)
	})
}

func TestDefaultsRegistry_GetProviderModels(t *testing.T) {
	registry := GetDefaultsRegistry()

	t.Run("ExistingProvider", func(t *testing.T) {
		providers := registry.GetAllProviders()
		require.NotEmpty(t, providers)

		providerID := providers[0]
		models := registry.GetProviderModels(providerID)
		assert.NotEmpty(t, models)

		// Verify all models have metadata
		for id, metadata := range models {
			assert.NotEmpty(t, id)
			assert.NotNil(t, metadata)
			assert.NotEmpty(t, metadata.DisplayName)
		}
	})

	t.Run("NonExistingProvider", func(t *testing.T) {
		models := registry.GetProviderModels("non-existent-provider")
		assert.Nil(t, models)
	})
}

func TestApplyUserOverride(t *testing.T) {
	baseMetadata := &ModelMetadata{
		DisplayName: "Test Model",
		MaxTokens:   1000,
		Description: "A test model",
		Capabilities: ModelCapabilities{
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsVision:    false,
		},
	}

	t.Run("NoOverride", func(t *testing.T) {
		result := ApplyUserOverride(baseMetadata, types.ModelCapabilityOverride{})
		assert.Equal(t, baseMetadata.DisplayName, result.DisplayName)
		assert.Equal(t, baseMetadata.MaxTokens, result.MaxTokens)
		assert.Equal(t, baseMetadata.Capabilities, result.Capabilities)
	})

	t.Run("OverrideMaxTokens", func(t *testing.T) {
		maxTokens := 2000
		override := types.ModelCapabilityOverride{
			MaxTokens: &maxTokens,
		}
		result := ApplyUserOverride(baseMetadata, override)
		assert.Equal(t, 2000, result.MaxTokens)
		assert.Equal(t, baseMetadata.Capabilities, result.Capabilities)
	})

	t.Run("OverrideContextWindow", func(t *testing.T) {
		contextWindow := 4000
		override := types.ModelCapabilityOverride{
			ContextWindow: &contextWindow,
		}
		result := ApplyUserOverride(baseMetadata, override)
		assert.Equal(t, 4000, result.MaxTokens)
	})

	t.Run("OverrideCapabilities", func(t *testing.T) {
		supportsTools := false
		supportsStreaming := false
		supportsVision := true
		override := types.ModelCapabilityOverride{
			SupportsTools:     &supportsTools,
			SupportsStreaming: &supportsStreaming,
			SupportsVision:    &supportsVision,
		}
		result := ApplyUserOverride(baseMetadata, override)
		assert.False(t, result.Capabilities.SupportsTools)
		assert.False(t, result.Capabilities.SupportsStreaming)
		assert.True(t, result.Capabilities.SupportsVision)
	})

	t.Run("OverrideAll", func(t *testing.T) {
		maxTokens := 5000
		supportsTools := false
		supportsStreaming := false
		supportsVision := true
		override := types.ModelCapabilityOverride{
			MaxTokens:         &maxTokens,
			SupportsTools:     &supportsTools,
			SupportsStreaming: &supportsStreaming,
			SupportsVision:    &supportsVision,
		}
		result := ApplyUserOverride(baseMetadata, override)
		assert.Equal(t, 5000, result.MaxTokens)
		assert.False(t, result.Capabilities.SupportsTools)
		assert.False(t, result.Capabilities.SupportsStreaming)
		assert.True(t, result.Capabilities.SupportsVision)
	})

	t.Run("NilBaseMetadata", func(t *testing.T) {
		maxTokens := 3000
		override := types.ModelCapabilityOverride{
			MaxTokens: &maxTokens,
		}
		result := ApplyUserOverride(nil, override)
		assert.NotNil(t, result)
		assert.Equal(t, 3000, result.MaxTokens)
	})
}

func TestModelsDevModel_HasVisionSupport(t *testing.T) {
	registry := &DefaultsRegistry{}

	t.Run("WithImageModality", func(t *testing.T) {
		model := &ModelsDevModel{
			Modalities: ModelsDevModalities{
				Input: []string{"text", "image"},
			},
		}
		assert.True(t, registry.hasVisionSupport(model))
	})

	t.Run("WithVisionModality", func(t *testing.T) {
		model := &ModelsDevModel{
			Modalities: ModelsDevModalities{
				Input: []string{"text", "vision"},
			},
		}
		assert.True(t, registry.hasVisionSupport(model))
	})

	t.Run("WithoutVisionModality", func(t *testing.T) {
		model := &ModelsDevModel{
			Modalities: ModelsDevModalities{
				Input: []string{"text"},
			},
		}
		assert.False(t, registry.hasVisionSupport(model))
	})
}

func TestDefaultsRegistry_ConvertToMetadata(t *testing.T) {
	registry := &DefaultsRegistry{}

	t.Run("CompleteModel", func(t *testing.T) {
		model := &ModelsDevModel{
			ID:       "test-model",
			Name:     "Test Model",
			ToolCall: true,
			Modalities: ModelsDevModalities{
				Input: []string{"text", "image"},
			},
			Cost: &ModelsDevCost{
				Input:  0.5,
				Output: 1.5,
			},
			Limit: ModelsDevLimit{
				Context: 8192,
				Output:  4096,
			},
		}

		metadata := registry.convertToMetadata(model)
		assert.NotNil(t, metadata)
		assert.Equal(t, "Test Model", metadata.DisplayName)
		assert.Equal(t, 8192, metadata.MaxTokens)
		assert.True(t, metadata.Capabilities.SupportsTools)
		assert.True(t, metadata.Capabilities.SupportsStreaming)
		assert.True(t, metadata.Capabilities.SupportsVision)
		assert.Equal(t, 0.5, metadata.CostPerMToken.InputCostPerMToken)
		assert.Equal(t, 1.5, metadata.CostPerMToken.OutputCostPerMToken)
	})

	t.Run("NilModel", func(t *testing.T) {
		metadata := registry.convertToMetadata(nil)
		assert.Nil(t, metadata)
	})

	t.Run("ModelWithoutCost", func(t *testing.T) {
		model := &ModelsDevModel{
			ID:   "test-model",
			Name: "Test Model",
			Limit: ModelsDevLimit{
				Context: 4096,
			},
		}

		metadata := registry.convertToMetadata(model)
		assert.NotNil(t, metadata)
		assert.Equal(t, 4096, metadata.MaxTokens)
		assert.Equal(t, 0.0, metadata.CostPerMToken.InputCostPerMToken)
		assert.Equal(t, 0.0, metadata.CostPerMToken.OutputCostPerMToken)
	})
}
