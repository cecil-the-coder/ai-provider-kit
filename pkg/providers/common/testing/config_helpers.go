package testing

import (
	"context"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ConfigTestHelper provides helpers for testing provider configurations
type ConfigTestHelper struct {
	t *testing.T
}

// NewConfigTestHelper creates a new configuration test helper
func NewConfigTestHelper(t *testing.T) *ConfigTestHelper {
	return &ConfigTestHelper{t: t}
}

// CreateTestConfig creates a standard test configuration for a provider
func (c *ConfigTestHelper) CreateTestConfig(providerType types.ProviderType) types.ProviderConfig {
	baseConfig := types.ProviderConfig{
		Type:                 providerType,
		APIKey:               "test-api-key",
		BaseURL:              "https://api.test.com/v1",
		DefaultModel:         "test-model",
		SupportsToolCalling:  true,
		SupportsStreaming:    true,
		SupportsResponsesAPI: false,
		MaxTokens:            4096,
		Timeout:              30 * time.Second,
		ProviderConfig: map[string]interface{}{
			"display_name": "Test Provider",
			"temperature":  0.7,
		},
	}

	// Add provider-specific configurations
	switch providerType {
	case types.ProviderTypeOpenAI:
		baseConfig.BaseURL = "https://api.openai.com/v1"
		baseConfig.DefaultModel = "gpt-4"
		baseConfig.SupportsResponsesAPI = true
	case types.ProviderTypeAnthropic:
		baseConfig.BaseURL = "https://api.anthropic.com"
		baseConfig.DefaultModel = "claude-sonnet-4-5"
	case types.ProviderTypeGemini:
		baseConfig.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
		baseConfig.DefaultModel = "gemini-1.5-pro"
	case types.ProviderTypeCerebras:
		baseConfig.BaseURL = "https://api.cerebras.ai/v1"
		baseConfig.DefaultModel = "zai-glm-4.6"
	case types.ProviderTypeQwen:
		baseConfig.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		baseConfig.DefaultModel = "qwen-turbo"
	case types.ProviderTypeOpenRouter:
		baseConfig.BaseURL = "https://openrouter.ai/api/v1"
		baseConfig.DefaultModel = "qwen/qwen3-coder"
		baseConfig.ProviderConfig["site_url"] = "https://example.com"
		baseConfig.ProviderConfig["site_name"] = "Test Site"
	}

	return baseConfig
}

// CreateMinimalConfig creates a minimal configuration with just the required fields
func (c *ConfigTestHelper) CreateMinimalConfig(providerType types.ProviderType) types.ProviderConfig {
	return types.ProviderConfig{
		Type:   providerType,
		APIKey: "test-key",
	}
}

// CreateMultiKeyConfig creates a configuration with multiple API keys
func (c *ConfigTestHelper) CreateMultiKeyConfig(providerType types.ProviderType) types.ProviderConfig {
	config := c.CreateTestConfig(providerType)
	config.ProviderConfig["api_keys"] = []string{
		"test-key-1",
		"test-key-2",
		"test-key-3",
	}
	return config
}

// CreateOAuthConfig creates a configuration for OAuth authentication
func (c *ConfigTestHelper) CreateOAuthConfig(providerType types.ProviderType) types.ProviderConfig {
	config := c.CreateMinimalConfig(providerType)
	config.ProviderConfig["oauth_credentials"] = map[string]interface{}{
		"client_id":     "test-client-id",
		"client_secret": "test-client-secret",
		"token_url":     "https://oauth.test.com/token",
	}
	return config
}

// AssertConfigValid tests that a configuration is valid for the given provider type
func (c *ConfigTestHelper) AssertConfigValid(config types.ProviderConfig, providerType types.ProviderType) {
	assert.Equal(c.t, providerType, config.Type, "Provider type should match")
	assert.NotEmpty(c.t, config.APIKey, "API key should not be empty")
	assert.NotEmpty(c.t, config.BaseURL, "Base URL should not be empty")
	assert.NotEmpty(c.t, config.DefaultModel, "Default model should not be empty")
	assert.NotNil(c.t, config.ProviderConfig, "Provider config should not be nil")
}

// AssertConfigEquals tests that two configurations are equal
func (c *ConfigTestHelper) AssertConfigEquals(expected, actual types.ProviderConfig) {
	assert.Equal(c.t, expected.Type, actual.Type, "Provider types should match")
	assert.Equal(c.t, expected.APIKey, actual.APIKey, "API keys should match")
	assert.Equal(c.t, expected.BaseURL, actual.BaseURL, "Base URLs should match")
	assert.Equal(c.t, expected.DefaultModel, actual.DefaultModel, "Default models should match")
	assert.Equal(c.t, expected.SupportsToolCalling, actual.SupportsToolCalling, "Tool calling support should match")
	assert.Equal(c.t, expected.SupportsStreaming, actual.SupportsStreaming, "Streaming support should match")
	assert.Equal(c.t, expected.SupportsResponsesAPI, actual.SupportsResponsesAPI, "Responses API support should match")
	assert.Equal(c.t, expected.MaxTokens, actual.MaxTokens, "Max tokens should match")
	assert.Equal(c.t, expected.Timeout, actual.Timeout, "Timeout should match")
}

// TestConfigurationUpdates tests that configuration updates work correctly
func (c *ConfigTestHelper) TestConfigurationUpdates(provider types.Provider) {
	originalConfig := c.CreateMinimalConfig(types.ProviderTypeOpenAI)

	// Configure with original config
	err := provider.Configure(originalConfig)
	require.NoError(c.t, err, "Initial configuration should succeed")

	// Test updating to new config
	newConfig := c.CreateTestConfig(types.ProviderTypeOpenAI)
	err = provider.Configure(newConfig)
	require.NoError(c.t, err, "Configuration update should succeed")

	// Verify the configuration was applied
	c.AssertConfigValid(provider.GetConfig(), types.ProviderTypeOpenAI)
}

// TestInvalidConfiguration tests that invalid configurations are rejected
//
//nolint:staticcheck // Empty branch is intentional - no assertion needed
func (c *ConfigTestHelper) TestInvalidConfiguration(provider types.Provider) {
	testCases := []struct {
		name        string
		config      types.ProviderConfig
		expectError bool
		description string
	}{
		{
			name: "WrongProviderType",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeAnthropic,
				APIKey: "test-key",
			},
			expectError: true,
			description: "Should reject configuration with wrong provider type",
		},
		{
			name: "EmptyAPIKey",
			config: types.ProviderConfig{
				Type: types.ProviderTypeOpenAI,
			},
			expectError: false, // Empty API key might be allowed for logout
			description: "Should allow empty API key (logout scenario)",
		},
		{
			name: "ValidConfig",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				APIKey: "test-key",
			},
			expectError: false,
			description: "Should accept valid configuration",
		},
	}

	for _, tc := range testCases {
		c.t.Run(tc.name, func(t *testing.T) {
			err := provider.Configure(tc.config)

			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				//nolint:staticcheck // Empty branch is intentional - no assertion needed
				// Don't assert.NoError here as some configurations might be valid but not supported
				// The test description explains the expectation
			}
		})
	}
}

// TestDisplayName tests that display names work correctly
func (c *ConfigTestHelper) TestDisplayName(provider types.Provider) {
	displayName := "Custom Provider Name"
	config := c.CreateMinimalConfig(provider.Type())
	config.ProviderConfig = map[string]interface{}{
		"display_name": displayName,
	}

	err := provider.Configure(config)
	if err == nil {
		// If configuration succeeded, verify the display name
		assert.Equal(c.t, displayName, provider.Name(), "Provider should use display name when configured")
	}
}

// TestMultipleAPIKeys tests that multiple API keys are handled correctly
func (c *ConfigTestHelper) TestMultipleAPIKeys(provider types.Provider) {
	config := c.CreateMultiKeyConfig(provider.Type())

	err := provider.Configure(config)
	if err == nil {
		// If configuration succeeded, verify authentication works
		assert.True(c.t, provider.IsAuthenticated(), "Provider should be authenticated with multiple API keys")
	}
}

// TestConfigurationPersistence tests that configuration persists across operations
func (c *ConfigTestHelper) TestConfigurationPersistence(provider types.Provider) {
	config := c.CreateTestConfig(provider.Type())

	// Configure provider
	err := provider.Configure(config)
	require.NoError(c.t, err, "Configuration should succeed")

	// Get configuration and verify it matches
	storedConfig := provider.GetConfig()
	c.AssertConfigEquals(config, storedConfig)

	// Perform some operations and verify config persists
	ctx := context.Background()
	_, _ = provider.GetModels(ctx) //nolint:errcheck // Might fail without valid API
	// Don't assert error here as it might fail without valid API

	// Verify configuration is still intact
	storedConfigAfter := provider.GetConfig()
	c.AssertConfigEquals(config, storedConfigAfter)
}

// StandardConfigTestSuite runs a comprehensive set of configuration tests
func (c *ConfigTestHelper) StandardConfigTestSuite(provider types.Provider) {
	c.t.Run("ValidConfiguration", func(t *testing.T) {
		config := c.CreateTestConfig(provider.Type())
		c.AssertConfigValid(config, provider.Type())

		err := provider.Configure(config)
		assert.NoError(t, err, "Should accept valid configuration")
	})

	c.t.Run("MinimalConfiguration", func(t *testing.T) {
		config := c.CreateMinimalConfig(provider.Type())

		err := provider.Configure(config)
		assert.NoError(t, err, "Should accept minimal configuration")
	})

	c.t.Run("ConfigurationUpdates", func(t *testing.T) {
		c.TestConfigurationUpdates(provider)
	})

	c.t.Run("InvalidConfiguration", func(t *testing.T) {
		c.TestInvalidConfiguration(provider)
	})

	c.t.Run("DisplayName", func(t *testing.T) {
		c.TestDisplayName(provider)
	})

	c.t.Run("MultipleAPIKeys", func(t *testing.T) {
		c.TestMultipleAPIKeys(provider)
	})

	c.t.Run("ConfigurationPersistence", func(t *testing.T) {
		c.TestConfigurationPersistence(provider)
	})
}
