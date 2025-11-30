package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ConfigHelper provides standardized configuration validation, extraction, and defaults
// for all AI providers, eliminating duplicate configuration code.
type ConfigHelper struct {
	providerName string
	providerType types.ProviderType
}

// NewConfigHelper creates a new configuration helper for a specific provider
func NewConfigHelper(providerName string, providerType types.ProviderType) *ConfigHelper {
	return &ConfigHelper{
		providerName: providerName,
		providerType: providerType,
	}
}

// ValidationResult contains the result of configuration validation
type ValidationResult struct {
	Valid  bool
	Errors []string
}

// ValidateProviderConfig performs comprehensive configuration validation
func (h *ConfigHelper) ValidateProviderConfig(config types.ProviderConfig) ValidationResult {
	var errors []string

	// Validate provider type
	if config.Type != h.providerType {
		errors = append(errors, fmt.Sprintf("invalid provider type for %s: %s", h.providerName, config.Type))
	}

	// Note: We don't require authentication validation here since providers should handle
	// missing credentials gracefully (e.g., for logout scenarios)
	// Providers will fail at runtime if authentication is required but not provided

	// Validate timeout if specified (zero is allowed for defaults)
	if config.Timeout < 0 {
		errors = append(errors, "timeout cannot be negative")
	}

	// Validate max tokens if specified
	if config.MaxTokens < 0 {
		errors = append(errors, "max_tokens cannot be negative")
	}

	return ValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

// ExtractAPIKeys extracts and consolidates API keys from various sources
func (h *ConfigHelper) ExtractAPIKeys(config types.ProviderConfig) []string {
	var keys []string

	// Add single API key if present
	if config.APIKey != "" {
		keys = append(keys, config.APIKey)
	}

	// Add multiple API keys from ProviderConfig
	if config.ProviderConfig != nil {
		if multiKeys, ok := config.ProviderConfig["api_keys"].([]string); ok {
			keys = append(keys, multiKeys...)
		}
	}

	return keys
}

// ExtractBaseURL extracts base URL with provider-specific defaults
func (h *ConfigHelper) ExtractBaseURL(config types.ProviderConfig) string {
	if config.BaseURL != "" {
		return config.BaseURL
	}

	// Return provider-specific default
	switch h.providerType {
	case types.ProviderTypeOpenAI:
		return "https://api.openai.com/v1"
	case types.ProviderTypeAnthropic:
		return "https://api.anthropic.com"
	case types.ProviderTypeGemini:
		return "https://generativelanguage.googleapis.com/v1beta"
	case types.ProviderTypeCerebras:
		return "https://api.cerebras.ai/v1"
	case types.ProviderTypeQwen:
		return "https://portal.qwen.ai/v1"
	case types.ProviderTypeOpenRouter:
		return "https://openrouter.ai/api/v1"
	default:
		return ""
	}
}

// ExtractDefaultModel extracts default model from config
// Note: Does not provide fallback defaults - each provider handles its own defaults via GetDefaultModel()
func (h *ConfigHelper) ExtractDefaultModel(config types.ProviderConfig) string {
	return config.DefaultModel
}

// ExtractTimeout extracts timeout with sensible defaults
func (h *ConfigHelper) ExtractTimeout(config types.ProviderConfig) time.Duration {
	if config.Timeout > 0 {
		return config.Timeout
	}
	return 60 * time.Second // Default timeout
}

// ExtractMaxTokens extracts max tokens with provider-specific defaults
func (h *ConfigHelper) ExtractMaxTokens(config types.ProviderConfig) int {
	if config.MaxTokens > 0 {
		return config.MaxTokens
	}

	// Provider-specific defaults
	switch h.providerType {
	case types.ProviderTypeOpenAI:
		return 4096
	case types.ProviderTypeAnthropic:
		return 4096
	case types.ProviderTypeGemini:
		return 8192
	case types.ProviderTypeCerebras:
		return 4096
	case types.ProviderTypeQwen:
		return 8192
	case types.ProviderTypeOpenRouter:
		return 4096
	default:
		return 4096
	}
}

// ExtractProviderSpecificConfig extracts provider-specific configuration into a struct
func (h *ConfigHelper) ExtractProviderSpecificConfig(config types.ProviderConfig, target interface{}) error {
	if config.ProviderConfig == nil {
		return nil
	}

	// Marshal and unmarshal to copy ProviderConfig to target struct
	configBytes, err := json.Marshal(config.ProviderConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal provider config: %w", err)
	}

	if err := json.Unmarshal(configBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal provider config: %w", err)
	}

	return nil
}

// ExtractStringField extracts a string field from provider config with fallback
func (h *ConfigHelper) ExtractStringField(config types.ProviderConfig, fieldName, fallback string) string {
	if config.ProviderConfig == nil {
		return fallback
	}

	if value, ok := config.ProviderConfig[fieldName].(string); ok && value != "" {
		return value
	}

	return fallback
}

// ExtractStringSliceField extracts a string slice field from provider config
func (h *ConfigHelper) ExtractStringSliceField(config types.ProviderConfig, fieldName string) []string {
	if config.ProviderConfig == nil {
		return nil
	}

	if value, ok := config.ProviderConfig[fieldName].([]string); ok {
		return value
	}

	return nil
}

// ApplyTopLevelOverrides applies top-level config fields to provider-specific config
func (h *ConfigHelper) ApplyTopLevelOverrides(config types.ProviderConfig, providerConfig interface{}) error {
	// Use reflection or type assertion to apply overrides
	// For now, we'll use a map-based approach
	configMap := make(map[string]interface{})
	configBytes, err := json.Marshal(providerConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal provider config: %w", err)
	}

	if err := json.Unmarshal(configBytes, &configMap); err != nil {
		return fmt.Errorf("failed to unmarshal provider config: %w", err)
	}

	// Apply common overrides
	if configMap["api_key"] == nil || configMap["api_key"].(string) == "" {
		configMap["api_key"] = config.APIKey
	}
	if configMap["base_url"] == nil || configMap["base_url"].(string) == "" {
		configMap["base_url"] = config.BaseURL
	}
	if configMap["model"] == nil || configMap["model"].(string) == "" {
		configMap["model"] = config.DefaultModel
	}

	// Marshal back
	updatedBytes, err := json.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	return json.Unmarshal(updatedBytes, providerConfig)
}

// ExtractDefaultOAuthClientID returns provider-specific default OAuth client IDs
func (h *ConfigHelper) ExtractDefaultOAuthClientID() string {
	switch h.providerType {
	case types.ProviderTypeOpenAI:
		return "app_EMoamEEZ73f0CkXaXp7hrann" // OpenAI public client ID
	case types.ProviderTypeAnthropic:
		return "9d1c250a-e61b-44d9-88ed-5944d1962f5e" // Anthropic MAX plan client ID
	case types.ProviderTypeGemini:
		return "" // Gemini doesn't have a public client ID
	default:
		return ""
	}
}

// GetProviderCapabilities returns the capabilities flags for the provider
func (h *ConfigHelper) GetProviderCapabilities() (supportsToolCalling, supportsStreaming, supportsResponsesAPI bool) {
	switch h.providerType {
	case types.ProviderTypeOpenAI:
		return true, true, true
	case types.ProviderTypeAnthropic:
		return true, true, false
	case types.ProviderTypeGemini:
		return true, true, false
	case types.ProviderTypeCerebras:
		return true, true, false
	case types.ProviderTypeQwen:
		return true, true, false
	case types.ProviderTypeOpenRouter:
		return true, true, false
	default:
		return false, false, false
	}
}

// SanitizeConfigForLogging returns a configuration copy with sensitive data removed
func (h *ConfigHelper) SanitizeConfigForLogging(config types.ProviderConfig) types.ProviderConfig {
	sanitized := config

	// Clear sensitive fields
	sanitized.APIKey = ""
	sanitized.OAuthCredentials = nil

	// Create a copy of ProviderConfig without sensitive fields
	if config.ProviderConfig != nil {
		sanitizedProviderConfig := make(map[string]interface{})
		for key, value := range config.ProviderConfig {
			switch key {
			case "api_key", "api_keys", "client_secret", "refresh_token", "access_token":
				// Skip sensitive fields
				continue
			default:
				sanitizedProviderConfig[key] = value
			}
		}
		sanitized.ProviderConfig = sanitizedProviderConfig
	}

	return sanitized
}

// ConfigSummary provides a human-readable summary of the configuration
func (h *ConfigHelper) ConfigSummary(config types.ProviderConfig) map[string]interface{} {
	summary := make(map[string]interface{})

	summary["provider"] = h.providerName
	summary["type"] = h.providerType
	summary["base_url"] = h.ExtractBaseURL(config)
	summary["default_model"] = h.ExtractDefaultModel(config)
	summary["timeout"] = h.ExtractTimeout(config)
	summary["max_tokens"] = h.ExtractMaxTokens(config)

	// Count authentication methods
	authMethods := []string{}
	if config.APIKey != "" {
		authMethods = append(authMethods, "api_key")
	}
	if len(h.ExtractAPIKeys(config)) > 1 {
		authMethods = append(authMethods, "multiple_api_keys")
	}
	if len(config.OAuthCredentials) > 0 {
		authMethods = append(authMethods, "oauth")
	}
	summary["auth_methods"] = authMethods

	// Capabilities
	toolCalling, streaming, responsesAPI := h.GetProviderCapabilities()
	summary["capabilities"] = map[string]bool{
		"tool_calling":  toolCalling,
		"streaming":     streaming,
		"responses_api": responsesAPI,
	}

	return summary
}

// MergeWithDefaults merges the provided config with provider defaults
func (h *ConfigHelper) MergeWithDefaults(config types.ProviderConfig) types.ProviderConfig {
	merged := config

	// Apply defaults for empty fields
	if merged.BaseURL == "" {
		merged.BaseURL = h.ExtractBaseURL(config)
	}
	if merged.DefaultModel == "" {
		merged.DefaultModel = h.ExtractDefaultModel(config)
	}
	if merged.Timeout == 0 {
		merged.Timeout = h.ExtractTimeout(config)
	}
	if merged.MaxTokens == 0 {
		merged.MaxTokens = h.ExtractMaxTokens(config)
	}

	// Apply capability defaults
	if !merged.SupportsToolCalling && !merged.SupportsStreaming && !merged.SupportsResponsesAPI {
		toolCalling, streaming, responsesAPI := h.GetProviderCapabilities()
		merged.SupportsToolCalling = toolCalling
		merged.SupportsStreaming = streaming
		merged.SupportsResponsesAPI = responsesAPI
	}

	return merged
}
