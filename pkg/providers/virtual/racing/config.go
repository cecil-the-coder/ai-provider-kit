package racing

import (
	"fmt"
	"strings"
)

// Config represents configuration for the racing provider
type Config struct {
	// Default configuration for backwards compatibility
	TimeoutMS     int      `yaml:"timeout_ms"`
	GracePeriodMS int      `yaml:"grace_period_ms"`
	Strategy      Strategy `yaml:"strategy"`
	ProviderNames []string `yaml:"providers"`

	// Virtual models configuration
	VirtualModels      map[string]VirtualModelConfig `yaml:"virtual_models"`
	DefaultVirtualModel string                       `yaml:"default_virtual_model"`
	PerformanceFile    string                       `yaml:"performance_file,omitempty"`
}

// VirtualModelConfig represents configuration for a single virtual model
type VirtualModelConfig struct {
	DisplayName string               `yaml:"display_name"`
	Description string               `yaml:"description"`
	Providers   []ProviderReference  `yaml:"providers"`
	Strategy    Strategy             `yaml:"strategy"`
	TimeoutMS   int                  `yaml:"timeout_ms"`
}

// ProviderReference represents a reference to a provider with optional configuration
type ProviderReference struct {
	Name     string                 `yaml:"name"`
	Model    string                 `yaml:"model,omitempty"`
	Priority int                    `yaml:"priority,omitempty"` // Higher numbers = higher priority
	Config   map[string]interface{} `yaml:"config,omitempty"`
}

// resolveVirtualModelConfig resolves a virtual model configuration by merging with defaults
func (c *Config) resolveVirtualModelConfig(modelID string) (*VirtualModelConfig, error) {
	if c.VirtualModels == nil {
		return nil, fmt.Errorf("no virtual models configured")
	}

	vmConfig, exists := c.VirtualModels[modelID]
	if !exists {
		return nil, fmt.Errorf("virtual model '%s' not found", modelID)
	}

	// Create a copy to avoid mutating the original
	resolved := &VirtualModelConfig{
		DisplayName: vmConfig.DisplayName,
		Description: vmConfig.Description,
		Providers:   make([]ProviderReference, len(vmConfig.Providers)),
		Strategy:    vmConfig.Strategy,
		TimeoutMS:   vmConfig.TimeoutMS,
	}

	copy(resolved.Providers, vmConfig.Providers)

	// Merge with defaults if not specified
	if resolved.Strategy == "" {
		resolved.Strategy = c.Strategy
	}
	if resolved.TimeoutMS == 0 {
		resolved.TimeoutMS = c.TimeoutMS
	}

	return resolved, nil
}

// generateModelDescription generates a description for a virtual model
func generateModelDescription(vmConfig *VirtualModelConfig, strategy Strategy) string {
	providerNames := make([]string, 0, len(vmConfig.Providers))
	for _, provider := range vmConfig.Providers {
		providerNames = append(providerNames, provider.Name)
	}

	description := fmt.Sprintf("Virtual model racing %s", strings.Join(providerNames, ", "))

	if strategy != "" {
		description += fmt.Sprintf(" using %s strategy", strategy)
	}

	return description
}

// GetVirtualModel returns the virtual model configuration for the given model name
func (c *Config) GetVirtualModel(modelName string) *VirtualModelConfig {
	if c.VirtualModels == nil {
		return nil
	}

	// If no model name specified, return the default
	if modelName == "" {
		if c.DefaultVirtualModel != "" {
			if vmConfig, exists := c.VirtualModels[c.DefaultVirtualModel]; exists {
				return c.resolveVirtualModelDefaults(&vmConfig)
			}
		}
		return nil
	}

	// Look for the specific model
	if vmConfig, exists := c.VirtualModels[modelName]; exists {
		return c.resolveVirtualModelDefaults(&vmConfig)
	}

	// Don't fallback to default when a specific model doesn't exist
	// Return nil to indicate the requested model doesn't exist
	return nil
}

// resolveVirtualModelDefaults resolves virtual model defaults (strategy and timeout)
func (c *Config) resolveVirtualModelDefaults(vmConfig *VirtualModelConfig) *VirtualModelConfig {
	if vmConfig == nil {
		return nil
	}

	// Create a copy to avoid mutating the original
	resolved := &VirtualModelConfig{
		DisplayName: vmConfig.DisplayName,
		Description: vmConfig.Description,
		Providers:   make([]ProviderReference, len(vmConfig.Providers)),
		Strategy:    vmConfig.Strategy,
		TimeoutMS:   vmConfig.TimeoutMS,
	}

	copy(resolved.Providers, vmConfig.Providers)

	// Merge with defaults if not specified
	if resolved.Strategy == "" {
		resolved.Strategy = c.Strategy
	}
	if resolved.TimeoutMS == 0 {
		resolved.TimeoutMS = c.TimeoutMS
	}

	return resolved
}

// GetEffectiveTimeout returns the effective timeout for a given virtual model
// Falls back to default config values if the virtual model doesn't exist
func (c *Config) GetEffectiveTimeout(modelName string) int {
	vmConfig := c.GetVirtualModel(modelName)
	if vmConfig != nil && vmConfig.TimeoutMS > 0 {
		return vmConfig.TimeoutMS
	}
	return c.TimeoutMS
}

// GetEffectiveStrategy returns the effective strategy for a given virtual model
// Falls back to default config values if the virtual model doesn't exist
func (c *Config) GetEffectiveStrategy(modelName string) Strategy {
	vmConfig := c.GetVirtualModel(modelName)
	if vmConfig != nil && vmConfig.Strategy != "" {
		return vmConfig.Strategy
	}
	return c.Strategy
}

// DefaultConfig returns a default configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		TimeoutMS:          5000,
		GracePeriodMS:      1000,
		Strategy:           StrategyFirstWins,
		DefaultVirtualModel: "default",
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Racing Model",
				Description: "Default virtual racing model with balanced providers",
				Strategy:    StrategyFirstWins,
				TimeoutMS:   5000,
			},
		},
	}
}

// Validate performs validation on the configuration
func (c *Config) Validate() error {
	if c.TimeoutMS <= 0 {
		return &ConfigError{Field: "timeout_ms", Message: "must be positive"}
	}

	if c.GracePeriodMS < 0 {
		return &ConfigError{Field: "grace_period_ms", Message: "must be non-negative"}
	}

	if c.DefaultVirtualModel == "" {
		return &ConfigError{Field: "default_virtual_model", Message: "cannot be empty"}
	}

	if len(c.VirtualModels) == 0 {
		return &ConfigError{Field: "virtual_models", Message: "at least one virtual model must be defined"}
	}

	// Check if default virtual model exists
	if _, exists := c.VirtualModels[c.DefaultVirtualModel]; !exists {
		return &ConfigError{
			Field:   "default_virtual_model",
			Message: "must reference an existing virtual model",
		}
	}

	// Validate each virtual model
	for name, vmConfig := range c.VirtualModels {
		if name == "" {
			return &ConfigError{Field: "virtual_model_name", Message: "virtual model name cannot be empty"}
		}

		if len(vmConfig.Providers) == 0 {
			return &ConfigError{
				Field:   "virtual_models." + name + ".providers",
				Message: "must have at least one provider",
			}
		}

		// Validate each provider reference
		for i, provider := range vmConfig.Providers {
			if provider.Name == "" {
				return &ConfigError{
					Field:   "virtual_models." + name + ".providers[" + string(rune(i)) + "].name",
					Message: "provider name cannot be empty",
				}
			}
		}

		if vmConfig.TimeoutMS < 0 {
			return &ConfigError{
				Field:   "virtual_models." + name + ".timeout_ms",
				Message: "must be non-negative",
			}
		}
	}

	return nil
}

// ConfigError represents a configuration validation error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}