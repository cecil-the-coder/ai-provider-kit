// Package factory provides validation and configuration parsing utilities for AI providers.
// It includes provider configuration validation and helper functions for config map parsing.
package factory

import (
	"fmt"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func ValidateProviderConfig(config types.ProviderConfig) error {
	if config.Type == "" {
		return fmt.Errorf("provider type is required")
	}
	if config.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if config.APIKey == "" && len(config.OAuthCredentials) == 0 {
		return fmt.Errorf("either api_key or oauth_credentials are required")
	}
	return nil
}

func CreateProviderFromConfig(factory *DefaultProviderFactory, configMap map[string]interface{}) (types.Provider, error) {
	providerType, ok := configMap["type"].(string)
	if !ok {
		return nil, fmt.Errorf("provider type is required")
	}

	name, ok := configMap["name"].(string)
	if !ok {
		return nil, fmt.Errorf("provider name is required")
	}

	config := types.ProviderConfig{
		Type:                 types.ProviderType(providerType),
		Name:                 name,
		APIKey:               getString(configMap, "api_key"),
		BaseURL:              getString(configMap, "base_url"),
		DefaultModel:         getString(configMap, "default_model"),
		Description:          getString(configMap, "description"),
		SupportsStreaming:    getBool(configMap, "supports_streaming"),
		SupportsToolCalling:  getBool(configMap, "supports_tool_calling"),
		SupportsResponsesAPI: getBool(configMap, "supports_responses_api"),
	}

	// Parse multi-OAuth credentials if provided
	if oauthCreds, ok := configMap["oauth_credentials"].([]interface{}); ok {
		for _, credInterface := range oauthCreds {
			if credMap, ok := credInterface.(map[string]interface{}); ok {
				cred := &types.OAuthCredentialSet{
					ID:           getString(credMap, "id"),
					ClientID:     getString(credMap, "client_id"),
					ClientSecret: getString(credMap, "client_secret"),
					AccessToken:  getString(credMap, "access_token"),
					RefreshToken: getString(credMap, "refresh_token"),
					Scopes:       getStringSlice(credMap, "scopes"),
				}
				config.OAuthCredentials = append(config.OAuthCredentials, cred)
			}
		}
	}

	return factory.CreateProvider(types.ProviderType(providerType), config)
}

// Helper functions for configuration map parsing
func getString(configMap map[string]interface{}, key string) string {
	if val, ok := configMap[key].(string); ok {
		return val
	}
	return ""
}

func getBool(configMap map[string]interface{}, key string) bool {
	if val, ok := configMap[key].(bool); ok {
		return val
	}
	return false
}

func getStringSlice(configMap map[string]interface{}, key string) []string {
	if val, ok := configMap[key].([]interface{}); ok {
		var result []string
		for _, v := range val {
			if str, ok := v.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}
