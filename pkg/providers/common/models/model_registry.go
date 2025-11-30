package models

import (
	"strings"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ModelMetadataRegistry provides centralized model metadata management
// It stores and enriches model information with display names, descriptions,
// token limits, and capabilities for all providers.
type ModelMetadataRegistry struct {
	metadata map[string]*ModelMetadata
	mu       sync.RWMutex
}

// ModelMetadata contains comprehensive metadata for a model
type ModelMetadata struct {
	DisplayName   string
	MaxTokens     int
	Description   string
	Capabilities  ModelCapabilities
	CostPerMToken CostInfo
}

// ModelCapabilities defines what a model can do
type ModelCapabilities struct {
	SupportsTools     bool
	SupportsStreaming bool
	SupportsVision    bool
}

// CostInfo contains pricing information per million tokens
type CostInfo struct {
	InputCostPerMToken  float64
	OutputCostPerMToken float64
}

// NewModelMetadataRegistry creates a new model metadata registry
func NewModelMetadataRegistry() *ModelMetadataRegistry {
	return &ModelMetadataRegistry{
		metadata: make(map[string]*ModelMetadata),
	}
}

// RegisterMetadata registers metadata for a specific model ID
func (r *ModelMetadataRegistry) RegisterMetadata(modelID string, metadata *ModelMetadata) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metadata[modelID] = metadata
}

// GetMetadata retrieves metadata for a model ID
// Returns nil if no metadata is registered for the model
func (r *ModelMetadataRegistry) GetMetadata(modelID string) *ModelMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact match first
	if metadata, exists := r.metadata[modelID]; exists {
		return metadata
	}

	// Try prefix match for versioned models
	for key, metadata := range r.metadata {
		if strings.HasPrefix(modelID, key) {
			return metadata
		}
	}

	return nil
}

// EnrichModel enriches a model with metadata from the registry
// If metadata is not found, returns the model unchanged
func (r *ModelMetadataRegistry) EnrichModel(model *types.Model) *types.Model {
	if model == nil {
		return nil
	}

	metadata := r.GetMetadata(model.ID)
	if metadata == nil {
		return model
	}

	// Create enriched copy
	enriched := *model

	// Apply metadata
	if metadata.DisplayName != "" {
		enriched.Name = metadata.DisplayName
	}
	if metadata.MaxTokens > 0 {
		enriched.MaxTokens = metadata.MaxTokens
	}
	if metadata.Description != "" {
		enriched.Description = metadata.Description
	}

	// Apply capabilities
	enriched.SupportsToolCalling = metadata.Capabilities.SupportsTools
	enriched.SupportsStreaming = metadata.Capabilities.SupportsStreaming

	return &enriched
}

// EnrichModels enriches multiple models with metadata
func (r *ModelMetadataRegistry) EnrichModels(models []types.Model) []types.Model {
	if len(models) == 0 {
		return models
	}

	enriched := make([]types.Model, len(models))
	for i := range models {
		enrichedModel := r.EnrichModel(&models[i])
		if enrichedModel != nil {
			enriched[i] = *enrichedModel
		} else {
			enriched[i] = models[i]
		}
	}

	return enriched
}

// RegisterBulkMetadata registers multiple model metadata entries at once
func (r *ModelMetadataRegistry) RegisterBulkMetadata(entries map[string]*ModelMetadata) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for modelID, metadata := range entries {
		r.metadata[modelID] = metadata
	}
}

// GetAllModelIDs returns all registered model IDs
func (r *ModelMetadataRegistry) GetAllModelIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.metadata))
	for id := range r.metadata {
		ids = append(ids, id)
	}
	return ids
}

// Clear removes all registered metadata
func (r *ModelMetadataRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metadata = make(map[string]*ModelMetadata)
}
