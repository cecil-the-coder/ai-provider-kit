package factory

import (
	"encoding/json"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenario2_ConfigurationManagement tests configuration handling across types
func TestScenario2_ConfigurationManagement(t *testing.T) {
	factory := NewProviderFactory()
	registerMockProvidersForIntegrationTests(factory)

	t.Run("ProviderConfigurationFromMaps", func(t *testing.T) {
		// Test configuration created from maps
		configMap := map[string]interface{}{
			"type":                  "openai",
			"name":                  "map-configured-provider",
			"api_key":               "map-api-key",
			"base_url":              "https://api.openai.com/v1",
			"default_model":         "gpt-4",
			"supports_streaming":    true,
			"supports_tool_calling": true,
			"max_tokens":            4096,
			"timeout":               30000000000,
			"tool_format":           "openai",
			"provider_config": map[string]interface{}{
				"custom_param": "custom_value",
			},
		}

		// Convert map to JSON then to ProviderConfig
		configJSON, err := json.Marshal(configMap)
		require.NoError(t, err)

		var config types.ProviderConfig
		err = json.Unmarshal(configJSON, &config)
		require.NoError(t, err)

		// Verify configuration
		assert.Equal(t, types.ProviderTypeOpenAI, config.Type)
		assert.Equal(t, "map-configured-provider", config.Name)
		assert.Equal(t, "map-api-key", config.APIKey)
		assert.Equal(t, "gpt-4", config.DefaultModel)
		assert.True(t, config.SupportsStreaming)
		assert.True(t, config.SupportsToolCalling)
		assert.Equal(t, 4096, config.MaxTokens)

		// Create provider with this configuration
		provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, config)
		require.NoError(t, err)
		require.NotNil(t, provider)

		// Verify provider was configured correctly
		retrievedConfig := provider.GetConfig()
		assert.Equal(t, config.Name, retrievedConfig.Name)
		assert.Equal(t, config.APIKey, retrievedConfig.APIKey)
	})

	t.Run("OAuthConfigurationHandling", func(t *testing.T) {
		// Test OAuth configuration
		oauthCreds := []*types.OAuthCredentialSet{
			{
				ID:           "test-cred-1",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				Scopes:       []string{"read", "write"},
			},
		}

		config := types.ProviderConfig{
			Type:             types.ProviderTypeOpenAI,
			Name:             "oauth-provider",
			OAuthCredentials: oauthCreds,
		}

		// Test JSON serialization/deserialization
		configJSON, err := json.Marshal(config)
		require.NoError(t, err)

		var retrievedConfig types.ProviderConfig
		err = json.Unmarshal(configJSON, &retrievedConfig)
		require.NoError(t, err)

		assert.Equal(t, oauthCreds[0].ClientID, retrievedConfig.OAuthCredentials[0].ClientID)
		assert.Equal(t, oauthCreds[0].ClientSecret, retrievedConfig.OAuthCredentials[0].ClientSecret)
		assert.Equal(t, oauthCreds[0].Scopes, retrievedConfig.OAuthCredentials[0].Scopes)
	})

	t.Run("ConfigurationValidationAcrossTypes", func(t *testing.T) {
		// Test invalid configurations
		invalidConfigs := []struct {
			name   string
			config types.ProviderConfig
			errMsg string
		}{
			{
				name: "missing_type",
				config: types.ProviderConfig{
					Name:   "no-type-provider",
					APIKey: "test-key",
				},
				errMsg: "provider type",
			},
			{
				name: "invalid_provider_type",
				config: types.ProviderConfig{
					Type:   types.ProviderType("invalid"),
					Name:   "invalid-type-provider",
					APIKey: "test-key",
				},
				errMsg: "not registered",
			},
		}

		for _, tc := range invalidConfigs {
			t.Run(tc.name, func(t *testing.T) {
				_, err := factory.CreateProvider(tc.config.Type, tc.config)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			})
		}
	})

	t.Run("ErrorRecoveryFromInvalidConfigs", func(t *testing.T) {
		// Test that factory remains operational after invalid config attempts
		originalProviders := factory.GetSupportedProviders()
		originalCount := len(originalProviders)

		// Try to create provider with invalid config
		invalidConfig := types.ProviderConfig{
			Type: types.ProviderType("nonexistent"),
			Name: "invalid",
		}
		_, err := factory.CreateProvider(invalidConfig.Type, invalidConfig)
		assert.Error(t, err)

		// Verify factory state is unchanged
		currentProviders := factory.GetSupportedProviders()
		assert.Equal(t, originalCount, len(currentProviders))

		// Verify we can still create valid providers
		validConfig := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			Name:   "valid-provider",
			APIKey: "test-key",
		}
		provider, err := factory.CreateProvider(types.ProviderTypeOpenAI, validConfig)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
	})
}
