// Package models provides model metadata, caching, and registry functionality
// for AI providers, including capability tracking and discovery.
package models

import (
	"strings"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ModelRegistry manages model information and caching
type ModelRegistry struct {
	models        map[string]*ModelCapability
	providerCache map[types.ProviderType][]types.Model
	cacheTime     map[string]time.Time
	ttl           time.Duration
	mu            sync.RWMutex
}

// NewModelRegistry creates a new model registry
func NewModelRegistry(ttl time.Duration) *ModelRegistry {
	return &ModelRegistry{
		models:        make(map[string]*ModelCapability),
		providerCache: make(map[types.ProviderType][]types.Model),
		cacheTime:     make(map[string]time.Time),
		ttl:           ttl,
	}
}

// RegisterModel registers a model with its capabilities
func (mr *ModelRegistry) RegisterModel(modelID string, capability *ModelCapability) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.models[modelID] = capability
}

// GetModelCapability returns the capabilities for a model
func (mr *ModelRegistry) GetModelCapability(modelID string) *ModelCapability {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	capability, exists := mr.models[modelID]
	if !exists {
		return nil
	}

	// Return a copy
	copy := *capability
	return &copy
}

// CacheModels caches models for a provider
func (mr *ModelRegistry) CacheModels(providerType types.ProviderType, models []types.Model) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.providerCache[providerType] = models
	mr.cacheTime[string(providerType)] = time.Now()

	// Also register individual model capabilities
	for _, model := range models {
		capability := mr.inferModelCapability(model)
		mr.models[model.ID] = capability
	}
}

// GetCachedModels returns cached models for a provider if not expired
func (mr *ModelRegistry) GetCachedModels(providerType types.ProviderType) []types.Model {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	cachedTime, exists := mr.cacheTime[string(providerType)]
	if !exists || time.Since(cachedTime) > mr.ttl {
		return nil
	}

	models, exists := mr.providerCache[providerType]
	if !exists {
		return nil
	}

	// Return a copy
	copy := make([]types.Model, len(models))
	for i, model := range models {
		copy[i] = model
	}

	return copy
}

// inferModelCapability infers model capabilities from model information
func (mr *ModelRegistry) inferModelCapability(model types.Model) *ModelCapability {
	return &ModelCapability{
		MaxTokens:         model.MaxTokens,
		SupportsStreaming: model.SupportsStreaming,
		SupportsTools:     model.SupportsToolCalling,
		Providers:         []types.ProviderType{model.Provider},
		// Default pricing - would be overridden with real data
		InputPrice:  0.001,
		OutputPrice: 0.002,
		Categories:  mr.inferCategories(model.ID, model.Provider),
	}
}

// inferCategories infers model categories from model ID and provider
func (mr *ModelRegistry) inferCategories(modelID string, provider types.ProviderType) []string {
	var categories []string
	modelLower := strings.ToLower(modelID)

	// Provider-specific categories
	switch provider {
	case types.ProviderTypeOpenAI:
		categories = append(categories, "text", "code")
	case types.ProviderTypeAnthropic:
		categories = append(categories, "text", "code")
	case types.ProviderTypeGemini:
		categories = append(categories, "text", "code", "multimodal")
	case types.ProviderTypeCerebras:
		categories = append(categories, "text", "code")
	default:
		categories = append(categories, "text")
	}

	// Vision/multimodal detection based on model name patterns
	if inferSupportsVision(modelLower) {
		categories = append(categories, "multimodal", "vision")
	}

	// Model-specific categories
	if contains(modelID, "instruct") || contains(modelID, "chat") {
		categories = append(categories, "chat")
	}
	if contains(modelID, "embed") {
		categories = append(categories, "embedding")
	}

	return unique(categories)
}

// inferSupportsVision checks if a model likely supports vision based on its name
func inferSupportsVision(modelLower string) bool {
	// OpenAI vision models
	if strings.Contains(modelLower, "gpt-4o") || // GPT-4o, GPT-4o-mini
		strings.Contains(modelLower, "gpt-4-turbo") || // GPT-4 Turbo with vision
		strings.Contains(modelLower, "gpt-4-vision") { // Explicit vision model
		return true
	}

	// Anthropic Claude 3+ models (all support vision)
	if strings.Contains(modelLower, "claude-3") ||
		strings.Contains(modelLower, "claude-sonnet-4") ||
		strings.Contains(modelLower, "claude-opus-4") {
		return true
	}

	// Google Gemini models (all support vision)
	if strings.Contains(modelLower, "gemini") {
		return true
	}

	// LLaVA models (vision-language models)
	if strings.Contains(modelLower, "llava") {
		return true
	}

	// Qwen-VL models
	if strings.Contains(modelLower, "qwen-vl") ||
		strings.Contains(modelLower, "qwen2-vl") {
		return true
	}

	// Generic vision indicators
	if strings.Contains(modelLower, "-vision") ||
		strings.Contains(modelLower, "-vl") || // Vision-Language suffix
		strings.Contains(modelLower, "vision-") {
		return true
	}

	// Pixtral (Mistral's vision model)
	if strings.Contains(modelLower, "pixtral") {
		return true
	}

	// Meta Llama vision models
	if strings.Contains(modelLower, "llama") && strings.Contains(modelLower, "vision") {
		return true
	}

	return false
}

// SearchModels searches for models matching criteria
func (mr *ModelRegistry) SearchModels(criteria SearchCriteria) []types.Model {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	var results []types.Model

	for _, models := range mr.providerCache {
		for _, model := range models {
			if mr.matchesCriteria(model, criteria) {
				results = append(results, model)
			}
		}
	}

	return results
}

// SearchCriteria defines search criteria for models
type SearchCriteria struct {
	Provider          *types.ProviderType `json:"provider,omitempty"`
	MinTokens         *int                `json:"min_tokens,omitempty"`
	MaxTokens         *int                `json:"max_tokens,omitempty"`
	SupportsStreaming *bool               `json:"supports_streaming,omitempty"`
	SupportsTools     *bool               `json:"supports_tools,omitempty"`
	Categories        []string            `json:"categories,omitempty"`
	NameContains      string              `json:"name_contains,omitempty"`
}

// matchesCriteria checks if a model matches search criteria
func (mr *ModelRegistry) matchesCriteria(model types.Model, criteria SearchCriteria) bool {
	if !mr.matchesProviderCriteria(model, criteria) {
		return false
	}

	if !mr.matchesTokenCriteria(model, criteria) {
		return false
	}

	if !mr.matchesCapabilityCriteria(model, criteria) {
		return false
	}

	if !mr.matchesNameCriteria(model, criteria) {
		return false
	}

	if !mr.matchesCategoriesCriteria(model, criteria) {
		return false
	}

	return true
}

// GetModelsByProvider returns all models for a specific provider
func (mr *ModelRegistry) GetModelsByProvider(providerType types.ProviderType) []types.Model {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	models, exists := mr.providerCache[providerType]
	if !exists {
		return []types.Model{}
	}

	// Return a copy
	copy := make([]types.Model, len(models))
	for i, model := range models {
		copy[i] = model
	}

	return copy
}

// GetProviderCount returns the number of providers with cached models
func (mr *ModelRegistry) GetProviderCount() int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	return len(mr.providerCache)
}

// GetTotalModelCount returns the total number of cached models
func (mr *ModelRegistry) GetTotalModelCount() int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	count := 0
	for _, models := range mr.providerCache {
		count += len(models)
	}

	return count
}

// ClearCache clears the cache for a specific provider or all providers
func (mr *ModelRegistry) ClearCache(providerType *types.ProviderType) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if providerType != nil {
		delete(mr.providerCache, *providerType)
		delete(mr.cacheTime, string(*providerType))
	} else {
		// Clear all caches
		mr.providerCache = make(map[types.ProviderType][]types.Model)
		mr.cacheTime = make(map[string]time.Time)
	}
}

// GetCacheInfo returns information about the cache
func (mr *ModelRegistry) GetCacheInfo() CacheInfo {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	info := CacheInfo{
		ProviderCount: len(mr.providerCache),
		TotalModels:   0,
		CacheTimes:    make(map[string]time.Time),
		OldestCache:   time.Time{},
		NewestCache:   time.Time{},
	}

	for providerType, models := range mr.providerCache {
		info.TotalModels += len(models)
		info.ProviderModels[string(providerType)] = len(models)

		if cacheTime, exists := mr.cacheTime[string(providerType)]; exists {
			info.CacheTimes[string(providerType)] = cacheTime
			if info.OldestCache.IsZero() || cacheTime.Before(info.OldestCache) {
				info.OldestCache = cacheTime
			}
			if info.NewestCache.IsZero() || cacheTime.After(info.NewestCache) {
				info.NewestCache = cacheTime
			}
		}
	}

	return info
}

// CacheInfo contains information about the model cache
type CacheInfo struct {
	ProviderCount  int                  `json:"provider_count"`
	TotalModels    int                  `json:"total_models"`
	ProviderModels map[string]int       `json:"provider_models"`
	CacheTimes     map[string]time.Time `json:"cache_times"`
	OldestCache    time.Time            `json:"oldest_cache"`
	NewestCache    time.Time            `json:"newest_cache"`
}

// Utility functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func unique(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// matchesProviderCriteria checks if model matches provider criteria
func (mr *ModelRegistry) matchesProviderCriteria(model types.Model, criteria SearchCriteria) bool {
	return criteria.Provider == nil || model.Provider == *criteria.Provider
}

// matchesTokenCriteria checks if model matches token constraints
func (mr *ModelRegistry) matchesTokenCriteria(model types.Model, criteria SearchCriteria) bool {
	if criteria.MinTokens != nil && model.MaxTokens < *criteria.MinTokens {
		return false
	}
	if criteria.MaxTokens != nil && model.MaxTokens > *criteria.MaxTokens {
		return false
	}
	return true
}

// matchesCapabilityCriteria checks if model matches capability criteria
func (mr *ModelRegistry) matchesCapabilityCriteria(model types.Model, criteria SearchCriteria) bool {
	if criteria.SupportsStreaming != nil && model.SupportsStreaming != *criteria.SupportsStreaming {
		return false
	}
	if criteria.SupportsTools != nil && model.SupportsToolCalling != *criteria.SupportsTools {
		return false
	}
	return true
}

// matchesNameCriteria checks if model matches name criteria
func (mr *ModelRegistry) matchesNameCriteria(model types.Model, criteria SearchCriteria) bool {
	if criteria.NameContains == "" {
		return true
	}
	return contains(model.Name, criteria.NameContains) || contains(model.ID, criteria.NameContains)
}

// matchesCategoriesCriteria checks if model matches categories criteria
func (mr *ModelRegistry) matchesCategoriesCriteria(model types.Model, criteria SearchCriteria) bool {
	if len(criteria.Categories) == 0 {
		return true
	}

	capability := mr.models[model.ID]
	if capability == nil {
		return false
	}

	return mr.hasAnyCategory(capability.Categories, criteria.Categories)
}

// hasAnyCategory checks if any of the required categories exist in model categories
func (mr *ModelRegistry) hasAnyCategory(modelCategories, requiredCategories []string) bool {
	for _, required := range requiredCategories {
		for _, category := range modelCategories {
			if category == required {
				return true
			}
		}
	}
	return false
}
