package models

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

//go:embed defaults.json
var defaultsJSON []byte

// ModelsDevProvider represents a provider in the models.dev dataset
type ModelsDevProvider struct {
	ID     string                    `json:"id"`
	Name   string                    `json:"name"`
	API    string                    `json:"api"`
	Models map[string]ModelsDevModel `json:"models"`
}

// ModelsDevModel represents a model in the models.dev dataset
type ModelsDevModel struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Attachment  bool                `json:"attachment"`
	Reasoning   bool                `json:"reasoning"`
	ToolCall    bool                `json:"tool_call"`
	Temperature bool                `json:"temperature"`
	Knowledge   string              `json:"knowledge"`
	ReleaseDate string              `json:"release_date"`
	LastUpdated string              `json:"last_updated"`
	Modalities  ModelsDevModalities `json:"modalities"`
	OpenWeights bool                `json:"open_weights"`
	Cost        *ModelsDevCost      `json:"cost,omitempty"`
	Limit       ModelsDevLimit      `json:"limit"`
}

// ModelsDevModalities represents input/output modalities
type ModelsDevModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// ModelsDevCost represents pricing information
type ModelsDevCost struct {
	Input     float64 `json:"input"`
	Output    float64 `json:"output"`
	CacheRead float64 `json:"cache_read,omitempty"`
}

// ModelsDevLimit represents context and output limits
type ModelsDevLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

// DefaultsRegistry manages the models.dev defaults
type DefaultsRegistry struct {
	providers map[string]*ModelsDevProvider
	mu        sync.RWMutex
	loaded    bool
}

var (
	defaultRegistry     *DefaultsRegistry
	defaultRegistryOnce sync.Once
)

// GetDefaultsRegistry returns the singleton defaults registry
func GetDefaultsRegistry() *DefaultsRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = &DefaultsRegistry{
			providers: make(map[string]*ModelsDevProvider),
		}
		defaultRegistry.load()
	})
	return defaultRegistry
}

// load loads the defaults from the embedded JSON
func (r *DefaultsRegistry) load() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.loaded {
		return
	}

	var data map[string]*ModelsDevProvider
	if err := json.Unmarshal(defaultsJSON, &data); err != nil {
		// Silent fail - defaults are optional
		return
	}

	r.providers = data
	r.loaded = true
}

// GetModelDefaults returns the default capabilities for a model ID
// It searches across all providers for a matching model
func (r *DefaultsRegistry) GetModelDefaults(modelID string) *ModelMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Search for the model across all providers
	for _, provider := range r.providers {
		if model, exists := provider.Models[modelID]; exists {
			return r.convertToMetadata(&model)
		}
	}

	// Try fuzzy matching for versioned models
	for _, provider := range r.providers {
		for id, model := range provider.Models {
			if strings.HasPrefix(modelID, id) || strings.HasPrefix(id, modelID) {
				return r.convertToMetadata(&model)
			}
		}
	}

	return nil
}

// GetProviderModels returns all models for a specific provider ID
func (r *DefaultsRegistry) GetProviderModels(providerID string) map[string]*ModelMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[providerID]
	if !exists {
		return nil
	}

	result := make(map[string]*ModelMetadata)
	for id, model := range provider.Models {
		result[id] = r.convertToMetadata(&model)
	}

	return result
}

// convertToMetadata converts a ModelsDevModel to ModelMetadata
func (r *DefaultsRegistry) convertToMetadata(model *ModelsDevModel) *ModelMetadata {
	if model == nil {
		return nil
	}

	metadata := &ModelMetadata{
		DisplayName: model.Name,
		MaxTokens:   model.Limit.Context,
		Description: "",
		Capabilities: ModelCapabilities{
			SupportsTools:     model.ToolCall,
			SupportsStreaming: true, // Assume streaming is supported by default
			SupportsVision:    r.hasVisionSupport(model),
		},
	}

	if model.Cost != nil {
		metadata.CostPerMToken = CostInfo{
			InputCostPerMToken:  model.Cost.Input,
			OutputCostPerMToken: model.Cost.Output,
		}
	}

	return metadata
}

// hasVisionSupport checks if a model supports vision based on its modalities
func (r *DefaultsRegistry) hasVisionSupport(model *ModelsDevModel) bool {
	for _, modality := range model.Modalities.Input {
		if modality == "image" || modality == "vision" {
			return true
		}
	}
	return false
}

// GetAllProviders returns all provider IDs
func (r *DefaultsRegistry) GetAllProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]string, 0, len(r.providers))
	for id := range r.providers {
		providers = append(providers, id)
	}
	return providers
}

// ApplyUserOverride applies user-defined capability overrides to metadata
func ApplyUserOverride(metadata *ModelMetadata, override types.ModelCapabilityOverride) *ModelMetadata {
	if metadata == nil {
		metadata = &ModelMetadata{}
	}

	// Create a copy to avoid modifying the original
	result := *metadata

	// Apply overrides - user overrides take precedence
	if override.MaxTokens != nil {
		result.MaxTokens = *override.MaxTokens
	}

	if override.ContextWindow != nil {
		result.MaxTokens = *override.ContextWindow
	}

	if override.SupportsStreaming != nil {
		result.Capabilities.SupportsStreaming = *override.SupportsStreaming
	}

	if override.SupportsTools != nil {
		result.Capabilities.SupportsTools = *override.SupportsTools
	}

	if override.SupportsVision != nil {
		result.Capabilities.SupportsVision = *override.SupportsVision
	}

	return &result
}
