// Package config provides shared configuration structures and utilities for
// AI Provider Kit example programs. It handles parsing the standard config.yaml
// format and converting it to types.ProviderConfig structures.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"gopkg.in/yaml.v3"
)

// =============================================================================
// Config Structures
// =============================================================================

// DemoConfig represents the complete configuration structure
type DemoConfig struct {
	Providers ProvidersConfig `yaml:"providers"`
	Metrics   MetricsConfig   `yaml:"metrics,omitempty"`
	Async     AsyncConfig     `yaml:"async,omitempty"`
}

// ProvidersConfig contains all provider configurations
type ProvidersConfig struct {
	// List of enabled providers
	Enabled []string `yaml:"enabled"`

	// Preferred order for provider selection
	PreferredOrder []string `yaml:"preferred_order"`

	// Built-in provider configurations
	Anthropic  *ProviderConfigEntry `yaml:"anthropic,omitempty"`
	OpenAI     *ProviderConfigEntry `yaml:"openai,omitempty"`
	Cerebras   *ProviderConfigEntry `yaml:"cerebras,omitempty"`
	Gemini     *ProviderConfigEntry `yaml:"gemini,omitempty"`
	Qwen       *ProviderConfigEntry `yaml:"qwen,omitempty"`
	OpenRouter *ProviderConfigEntry `yaml:"openrouter,omitempty"`

	// Custom providers (map of provider name to config)
	Custom map[string]ProviderConfigEntry `yaml:"custom,omitempty"`
}

// ProviderConfigEntry represents a single provider's configuration
type ProviderConfigEntry struct {
	// Provider type (e.g., "openai", "anthropic", "gemini")
	// For custom providers, this specifies which provider API to use
	Type string `yaml:"type"`

	// Default model to use for this provider
	DefaultModel string `yaml:"default_model"`

	// Base URL for the provider API (optional, uses provider default if not set)
	BaseURL string `yaml:"base_url"`

	// Single API key (used if only one key is needed)
	APIKey string `yaml:"api_key"`

	// Multiple API keys for load balancing or failover
	APIKeys []string `yaml:"api_keys"`

	// Project ID (used by some providers like Gemini)
	ProjectID string `yaml:"project_id"`

	// Maximum tokens for completions
	MaxTokens int `yaml:"max_tokens"`

	// Temperature for response generation
	Temperature float64 `yaml:"temperature"`

	// OAuth credentials (can have multiple sets for failover)
	OAuthCredentials []OAuthCredentialEntry `yaml:"oauth_credentials"`

	// Custom provider-specific models list
	Models interface{} `yaml:"models,omitempty"`
}

// OAuthCredentialEntry represents a single set of OAuth credentials
type OAuthCredentialEntry struct {
	// Unique identifier for this credential set
	ID string `yaml:"id"`

	// OAuth client credentials
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`

	// OAuth tokens
	AccessToken  string `yaml:"access_token"`
	RefreshToken string `yaml:"refresh_token"`

	// Token expiration
	ExpiresAt string `yaml:"expires_at"`

	// OAuth scopes
	Scopes []string `yaml:"scopes"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// AsyncConfig represents async configuration
type AsyncConfig struct {
	Enabled bool `yaml:"enabled"`
}

// =============================================================================
// Configuration Loading
// =============================================================================

// LoadConfig loads and parses a YAML configuration file
func LoadConfig(filename string) (*DemoConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config DemoConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
}

// GetProviderEntry retrieves the provider config entry for a given provider name
func GetProviderEntry(config *DemoConfig, name string) *ProviderConfigEntry {
	// Check built-in providers
	switch name {
	case "anthropic":
		return config.Providers.Anthropic
	case "openai":
		return config.Providers.OpenAI
	case "cerebras":
		return config.Providers.Cerebras
	case "gemini":
		return config.Providers.Gemini
	case "qwen":
		return config.Providers.Qwen
	case "openrouter":
		return config.Providers.OpenRouter
	}

	// Check custom providers
	if entry, ok := config.Providers.Custom[name]; ok {
		return &entry
	}

	return nil
}

// =============================================================================
// ProviderConfig Construction
// =============================================================================

// BuildProviderConfig constructs a types.ProviderConfig from a ProviderConfigEntry
// This converts the config file format into the structure required by the ai-provider-kit module.
func BuildProviderConfig(name string, entry *ProviderConfigEntry) types.ProviderConfig {
	config := types.ProviderConfig{
		Name: name,
		Type: DetermineProviderType(name, entry),
	}

	// Set API key based on priority:
	// 1. Single api_key field
	// 2. First entry from api_keys array
	// 3. Access token from first OAuth credential
	if entry.APIKey != "" {
		config.APIKey = entry.APIKey
	} else if len(entry.APIKeys) > 0 {
		config.APIKey = entry.APIKeys[0]
	} else if len(entry.OAuthCredentials) > 0 {
		config.APIKey = entry.OAuthCredentials[0].AccessToken
	}

	// Set optional fields
	if entry.BaseURL != "" {
		config.BaseURL = entry.BaseURL
	}

	if entry.DefaultModel != "" {
		config.DefaultModel = entry.DefaultModel
	}

	if entry.MaxTokens > 0 {
		config.MaxTokens = entry.MaxTokens
	}

	// Convert OAuth credentials to types.OAuthCredentialSet
	if len(entry.OAuthCredentials) > 0 {
		config.OAuthCredentials = ConvertOAuthCredentials(entry.OAuthCredentials)
	}

	return config
}

// DetermineProviderType determines the types.ProviderType based on name and config
func DetermineProviderType(name string, entry *ProviderConfigEntry) types.ProviderType {
	// If entry has an explicit type field (for custom providers), use that
	if entry.Type != "" {
		switch entry.Type {
		case "openai":
			return types.ProviderTypeOpenAI
		case "anthropic":
			return types.ProviderTypeAnthropic
		case "gemini":
			return types.ProviderTypeGemini
		case "qwen":
			return types.ProviderTypeQwen
		case "cerebras":
			return types.ProviderTypeCerebras
		case "openrouter":
			return types.ProviderTypeOpenRouter
		}
	}

	// Otherwise, use the provider name
	switch name {
	case "openai":
		return types.ProviderTypeOpenAI
	case "anthropic":
		return types.ProviderTypeAnthropic
	case "gemini":
		return types.ProviderTypeGemini
	case "qwen":
		return types.ProviderTypeQwen
	case "cerebras":
		return types.ProviderTypeCerebras
	case "openrouter":
		return types.ProviderTypeOpenRouter
	default:
		// Default to OpenAI for custom providers (most are OpenAI-compatible)
		return types.ProviderTypeOpenAI
	}
}

// ConvertOAuthCredentials converts []OAuthCredentialEntry to []*types.OAuthCredentialSet
func ConvertOAuthCredentials(entries []OAuthCredentialEntry) []*types.OAuthCredentialSet {
	credSets := make([]*types.OAuthCredentialSet, 0, len(entries))

	for _, entry := range entries {
		// Parse expiration time
		var expiresAt time.Time
		if entry.ExpiresAt != "" {
			// Try parsing as RFC3339 format
			if t, err := time.Parse(time.RFC3339, entry.ExpiresAt); err == nil {
				expiresAt = t
			}
		}

		credSet := &types.OAuthCredentialSet{
			ID:           entry.ID,
			ClientID:     entry.ClientID,
			ClientSecret: entry.ClientSecret,
			AccessToken:  entry.AccessToken,
			RefreshToken: entry.RefreshToken,
			ExpiresAt:    expiresAt,
			Scopes:       entry.Scopes,
		}

		credSets = append(credSets, credSet)
	}

	return credSets
}

// =============================================================================
// Utility Functions
// =============================================================================

// MaskAPIKey masks an API key for display purposes
func MaskAPIKey(key string) string {
	if key == "" {
		return ""
	}

	if len(key) <= 8 {
		return "***"
	}

	// Show first 4 and last 4 characters
	return key[:4] + "..." + key[len(key)-4:]
}
