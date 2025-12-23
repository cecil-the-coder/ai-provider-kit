// Package config provides configuration utilities for AI provider implementations
package config

import (
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// CacheConfig holds cache configuration for providers
type CacheConfig struct {
	ModelCacheTTL time.Duration
}

// DefaultCacheConfigs maps provider types to their default cache configs
var DefaultCacheConfigs = map[types.ProviderType]CacheConfig{
	types.ProviderTypeAnthropic:  {ModelCacheTTL: 6 * time.Hour},
	types.ProviderTypeOpenAI:     {ModelCacheTTL: 24 * time.Hour},
	types.ProviderTypeGemini:     {ModelCacheTTL: 2 * time.Hour},
	types.ProviderTypeCerebras:   {ModelCacheTTL: 6 * time.Hour},
	types.ProviderTypeQwen:       {ModelCacheTTL: 6 * time.Hour},
	types.ProviderTypeOpenRouter: {ModelCacheTTL: 12 * time.Hour},
}

// GetModelCacheTTL returns the model cache TTL for a provider type.
// If the provider type is not found in DefaultCacheConfigs, it returns
// a default fallback of 6 hours.
func GetModelCacheTTL(providerType types.ProviderType) time.Duration {
	if config, ok := DefaultCacheConfigs[providerType]; ok {
		return config.ModelCacheTTL
	}
	return 6 * time.Hour // Default fallback
}
