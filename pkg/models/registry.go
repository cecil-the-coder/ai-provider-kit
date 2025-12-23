// Package models provides model metadata, registry, and discovery functionality
// for AI provider operations.
//
// This package re-exports the model registry from internal/common/models,
// providing access to model metadata including capabilities, defaults, and
// parameters. It is primarily used for model discovery, validation, and
// dynamic provider model lookups.
//
// # GetDefaultsRegistry
//
// The primary entry point is GetDefaultsRegistry(), which returns the singleton
// defaults registry containing model metadata from the embedded models.dev dataset.
// This registry provides comprehensive information about AI models including:
//
//   - Display names and descriptions
//   - Token limits and context windows
//   - Capability flags (tools, streaming, vision)
//   - Pricing information
//
// Example usage:
//
//	registry := models.GetDefaultsRegistry()
//	metadata := registry.GetModelDefaults("gpt-4o")
//	if metadata != nil {
//	    fmt.Printf("Model: %s, MaxTokens: %d\n", metadata.DisplayName, metadata.MaxTokens)
//	}
//
// # Related Types
//
// The package exports the following key types:
//
//   - ModelMetadata: Comprehensive metadata for a model
//   - ModelCapabilities: Capabilities that a model supports
//   - CostInfo: Pricing information per million tokens
//   - DefaultsRegistry: Registry for model defaults
package models

import (
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/models"
)

// ModelMetadata contains comprehensive metadata for a model.
//
// It includes display names, token limits, descriptions, capabilities,
// and cost information. This type is returned by the DefaultsRegistry
// when querying model information.
type ModelMetadata = models.ModelMetadata

// ModelCapabilities defines what capabilities a model supports.
//
// These flags indicate whether a model can use tools, supports streaming
// responses, or processes vision/multimodal inputs.
type ModelCapabilities = models.ModelCapabilities

// CostInfo contains pricing information per million tokens.
//
// This struct provides input and output token costs for models that
// have pricing information available in the registry.
type CostInfo = models.CostInfo

// DefaultsRegistry manages the models.dev defaults.
//
// The registry provides access to model metadata from the embedded
// models.dev dataset, including capabilities, token limits, and pricing.
// Use GetDefaultsRegistry() to obtain the singleton instance.
//
// Methods:
//   - GetModelDefaults(modelID string) *ModelMetadata
//   - GetProviderModels(providerID string) map[string]*ModelMetadata
//   - GetAllProviders() []string
type DefaultsRegistry = models.DefaultsRegistry

// GetDefaultsRegistry returns the singleton defaults registry.
//
// The registry contains model metadata loaded from the embedded models.dev
// dataset. It provides comprehensive information about AI models including
// capabilities, defaults, and parameters.
//
// This is the primary entry point for model discovery and validation.
// The returned registry is safe for concurrent use and is initialized
// once on first access.
//
// Example:
//
//	registry := models.GetDefaultsRegistry()
//	metadata := registry.GetModelDefaults("gpt-4o")
//	if metadata != nil {
//	    fmt.Printf("Model: %s, MaxTokens: %d\n",
//	        metadata.DisplayName, metadata.MaxTokens)
//	}
func GetDefaultsRegistry() *models.DefaultsRegistry {
	return models.GetDefaultsRegistry()
}
