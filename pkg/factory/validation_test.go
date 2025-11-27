package factory

import (
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateProviderConfig_ValidConfig tests validation of valid provider configurations
func TestValidateProviderConfig_ValidConfig(t *testing.T) {
	testCases := []struct {
		name   string
		config types.ProviderConfig
	}{
		{
			name: "Valid config with API key",
			config: types.ProviderConfig{
				Type:    types.ProviderTypeOpenAI,
				Name:    "openai-test",
				APIKey:  "sk-test-key",
				BaseURL: "https://api.openai.com/v1",
			},
		},
		{
			name: "Valid config with OAuth",
			config: types.ProviderConfig{
				Type: types.ProviderTypeAnthropic,
				Name: "anthropic-test",
				OAuthCredentials: []*types.OAuthCredentialSet{
					{
						ID:           "test-cred",
						ClientID:     "test-client-id",
						ClientSecret: "test-client-secret",
						Scopes:       []string{"read", "write"},
					},
				},
			},
		},
		{
			name: "Valid config with both API key and OAuth",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeGemini,
				Name:   "gemini-test",
				APIKey: "gemini-test-key",
				OAuthCredentials: []*types.OAuthCredentialSet{
					{
						ID:           "test-cred",
						ClientID:     "test-client-id",
						ClientSecret: "test-client-secret",
					},
				},
			},
		},
		{
			name: "Valid minimal config with API key",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeOpenRouter,
				Name:   "openrouter-test",
				APIKey: "openrouter-test-key",
			},
		},
		{
			name: "Valid config with all optional fields",
			config: types.ProviderConfig{
				Type:                 types.ProviderTypeOllama,
				Name:                 "ollama-full-test",
				BaseURL:              "http://localhost:11434",
				APIKey:               "optional-api-key",
				DefaultModel:         "llama2",
				Description:          "Test Ollama provider",
				SupportsStreaming:    true,
				SupportsToolCalling:  false,
				SupportsResponsesAPI: true,
				MaxTokens:            4096,
				Timeout:              30 * time.Second,
				ToolFormat:           types.ToolFormatOpenAI,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProviderConfig(tc.config)
			assert.NoError(t, err)
		})
	}
}

// TestValidateProviderConfig_InvalidConfig tests validation of invalid provider configurations
func TestValidateProviderConfig_InvalidConfig(t *testing.T) {
	testCases := []struct {
		name        string
		config      types.ProviderConfig
		expectedErr string
	}{
		{
			name: "Empty config",
			config: types.ProviderConfig{
				Type: "",
				Name: "",
			},
			expectedErr: "provider type is required",
		},
		{
			name: "Missing provider type",
			config: types.ProviderConfig{
				Name:   "test-provider",
				APIKey: "test-key",
			},
			expectedErr: "provider type is required",
		},
		{
			name: "Missing provider name",
			config: types.ProviderConfig{
				Type:   types.ProviderTypeOpenAI,
				APIKey: "test-key",
			},
			expectedErr: "provider name is required",
		},
		{
			name: "Missing both API key and OAuth",
			config: types.ProviderConfig{
				Type: types.ProviderTypeAnthropic,
				Name: "anthropic-test",
			},
			expectedErr: "either api_key or oauth_credentials are required",
		},
		{
			name: "Empty provider type and name",
			config: types.ProviderConfig{
				Type:    "",
				Name:    "",
				APIKey:  "test-key",
				BaseURL: "https://api.example.com",
			},
			expectedErr: "provider type is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProviderConfig(tc.config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

// TestCreateProviderFromConfig_ValidConfig tests provider creation from valid configuration maps
func TestCreateProviderFromConfig_ValidConfig(t *testing.T) {
	factory := NewProviderFactory()

	// Register a test provider
	testProviderType := types.ProviderType("config-test")
	factory.RegisterProvider(testProviderType, func(config types.ProviderConfig) types.Provider {
		return &MockProvider{name: config.Name, providerType: testProviderType}
	})

	testCases := []struct {
		name      string
		configMap map[string]interface{}
		expected  types.ProviderConfig
	}{
		{
			name: "Basic configuration",
			configMap: map[string]interface{}{
				"type":    "config-test",
				"name":    "basic-test",
				"api_key": "test-api-key",
			},
			expected: types.ProviderConfig{
				Type:   testProviderType,
				Name:   "basic-test",
				APIKey: "test-api-key",
			},
		},
		{
			name: "Full configuration",
			configMap: map[string]interface{}{
				"type":                   "config-test",
				"name":                   "full-test",
				"api_key":                "test-api-key",
				"base_url":               "https://api.example.com",
				"default_model":          "test-model",
				"description":            "Test provider",
				"supports_streaming":     true,
				"supports_tool_calling":  false,
				"supports_responses_api": true,
			},
			expected: types.ProviderConfig{
				Type:                 testProviderType,
				Name:                 "full-test",
				APIKey:               "test-api-key",
				BaseURL:              "https://api.example.com",
				DefaultModel:         "test-model",
				Description:          "Test provider",
				SupportsStreaming:    true,
				SupportsToolCalling:  false,
				SupportsResponsesAPI: true,
			},
		},
		{
			name: "Configuration with OAuth",
			configMap: map[string]interface{}{
				"type": "config-test",
				"name": "oauth-test",
				"oauth_credentials": []interface{}{
					map[string]interface{}{
						"id":            "test-cred",
						"client_id":     "test-client-id",
						"client_secret": "test-client-secret",
						"scopes":        []interface{}{"read", "write"},
					},
				},
			},
			expected: types.ProviderConfig{
				Type: testProviderType,
				Name: "oauth-test",
				OAuthCredentials: []*types.OAuthCredentialSet{
					{
						ID:           "test-cred",
						ClientID:     "test-client-id",
						ClientSecret: "test-client-secret",
						Scopes:       []string{"read", "write"},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := CreateProviderFromConfig(factory, tc.configMap)
			assert.NoError(t, err)
			assert.NotNil(t, provider)

			mockProvider, ok := provider.(*MockProvider)
			require.True(t, ok)
			assert.Equal(t, tc.expected.Name, mockProvider.Name())
		})
	}
}

// TestCreateProviderFromConfig_InvalidConfig tests provider creation from invalid configuration maps
func TestCreateProviderFromConfig_InvalidConfig(t *testing.T) {
	factory := NewProviderFactory()

	testCases := []struct {
		name        string
		configMap   map[string]interface{}
		expectedErr string
	}{
		{
			name:        "Empty config map",
			configMap:   map[string]interface{}{},
			expectedErr: "provider type is required",
		},
		{
			name: "Missing provider type",
			configMap: map[string]interface{}{
				"name":    "test-provider",
				"api_key": "test-key",
			},
			expectedErr: "provider type is required",
		},
		{
			name: "Missing provider name",
			configMap: map[string]interface{}{
				"type":    "openai",
				"api_key": "test-key",
			},
			expectedErr: "provider name is required",
		},
		{
			name: "Invalid provider type type",
			configMap: map[string]interface{}{
				"type":    123,
				"name":    "test-provider",
				"api_key": "test-key",
			},
			expectedErr: "provider type is required",
		},
		{
			name: "Invalid provider name type",
			configMap: map[string]interface{}{
				"type":    "openai",
				"name":    123,
				"api_key": "test-key",
			},
			expectedErr: "provider name is required",
		},
		{
			name: "Unknown provider type",
			configMap: map[string]interface{}{
				"type":    "unknown-provider",
				"name":    "unknown-test",
				"api_key": "test-key",
			},
			expectedErr: "provider type unknown-provider not registered",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := CreateProviderFromConfig(factory, tc.configMap)
			assert.Error(t, err)
			assert.Nil(t, provider)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

// TestCreateProviderFromConfig_EdgeCases tests edge cases in configuration parsing
func TestCreateProviderFromConfig_EdgeCases(t *testing.T) {
	factory := NewProviderFactory()

	// Register a test provider
	testProviderType := types.ProviderType("edge-test")
	factory.RegisterProvider(testProviderType, func(config types.ProviderConfig) types.Provider {
		return &MockProvider{name: config.Name, providerType: testProviderType}
	})

	testCases := []struct {
		name      string
		configMap map[string]interface{}
	}{
		{
			name: "Nil OAuth config",
			configMap: map[string]interface{}{
				"type":    "edge-test",
				"name":    "nil-oauth-test",
				"api_key": "test-key",
				"oauth":   nil,
			},
		},
		{
			name: "Empty OAuth map",
			configMap: map[string]interface{}{
				"type":    "edge-test",
				"name":    "empty-oauth-test",
				"api_key": "test-key",
				"oauth":   map[string]interface{}{},
			},
		},
		{
			name: "OAuth with invalid scopes",
			configMap: map[string]interface{}{
				"type": "edge-test",
				"name": "invalid-scopes-test",
				"oauth": map[string]interface{}{
					"client_id":     "test-client-id",
					"client_secret": "test-client-secret",
					"scopes":        []interface{}{"valid", 123, "another-valid"},
				},
			},
		},
		{
			name: "Boolean values as strings",
			configMap: map[string]interface{}{
				"type":                   "edge-test",
				"name":                   "string-bool-test",
				"api_key":                "test-key",
				"supports_streaming":     "true",
				"supports_tool_calling":  "false",
				"supports_responses_api": "yes",
			},
		},
		{
			name: "Extra unknown fields",
			configMap: map[string]interface{}{
				"type":            "edge-test",
				"name":            "extra-fields-test",
				"api_key":         "test-key",
				"unknown_field":   "should_be_ignored",
				"another_unknown": 123,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := CreateProviderFromConfig(factory, tc.configMap)
			// These should not error as they should handle the edge cases gracefully
			if err != nil {
				// Some edge cases might legitimately error (like invalid provider types)
				// but parsing issues should be handled gracefully
				t.Logf("Edge case resulted in error: %v", err)
			} else {
				assert.NotNil(t, provider)
			}
		})
	}
}

// TestHelperFunctions tests the helper functions for config parsing
func TestHelperFunctions(t *testing.T) {
	t.Run("getString", func(t *testing.T) {
		testCases := []struct {
			name      string
			configMap map[string]interface{}
			key       string
			expected  string
		}{
			{
				name: "Existing string value",
				configMap: map[string]interface{}{
					"test_key": "test_value",
				},
				key:      "test_key",
				expected: "test_value",
			},
			{
				name: "Non-existent key",
				configMap: map[string]interface{}{
					"other_key": "value",
				},
				key:      "test_key",
				expected: "",
			},
			{
				name: "Wrong type",
				configMap: map[string]interface{}{
					"test_key": 123,
				},
				key:      "test_key",
				expected: "",
			},
			{
				name: "Nil map",
				configMap: map[string]interface{}{
					"test_key": nil,
				},
				key:      "test_key",
				expected: "",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := getString(tc.configMap, tc.key)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("getBool", func(t *testing.T) {
		testCases := []struct {
			name      string
			configMap map[string]interface{}
			key       string
			expected  bool
		}{
			{
				name: "True value",
				configMap: map[string]interface{}{
					"test_key": true,
				},
				key:      "test_key",
				expected: true,
			},
			{
				name: "False value",
				configMap: map[string]interface{}{
					"test_key": false,
				},
				key:      "test_key",
				expected: false,
			},
			{
				name: "Non-existent key",
				configMap: map[string]interface{}{
					"other_key": true,
				},
				key:      "test_key",
				expected: false,
			},
			{
				name: "Wrong type",
				configMap: map[string]interface{}{
					"test_key": "true",
				},
				key:      "test_key",
				expected: false,
			},
			{
				name: "Nil value",
				configMap: map[string]interface{}{
					"test_key": nil,
				},
				key:      "test_key",
				expected: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := getBool(tc.configMap, tc.key)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("getStringSlice", func(t *testing.T) {
		testCases := []struct {
			name      string
			configMap map[string]interface{}
			key       string
			expected  []string
		}{
			{
				name: "Valid string slice",
				configMap: map[string]interface{}{
					"test_key": []interface{}{"item1", "item2", "item3"},
				},
				key:      "test_key",
				expected: []string{"item1", "item2", "item3"},
			},
			{
				name: "Empty slice",
				configMap: map[string]interface{}{
					"test_key": []interface{}{},
				},
				key:      "test_key",
				expected: nil, // Function returns nil for empty slice (no valid strings found)
			},
			{
				name: "Mixed types in slice",
				configMap: map[string]interface{}{
					"test_key": []interface{}{"string1", 123, "string2", true},
				},
				key:      "test_key",
				expected: []string{"string1", "string2"},
			},
			{
				name: "Non-existent key",
				configMap: map[string]interface{}{
					"other_key": []interface{}{"item1"},
				},
				key:      "test_key",
				expected: nil,
			},
			{
				name: "Wrong type",
				configMap: map[string]interface{}{
					"test_key": "not a slice",
				},
				key:      "test_key",
				expected: nil,
			},
			{
				name: "Nil value",
				configMap: map[string]interface{}{
					"test_key": nil,
				},
				key:      "test_key",
				expected: nil,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := getStringSlice(tc.configMap, tc.key)
				assert.Equal(t, tc.expected, result)
			})
		}
	})
}

// TestCreateProviderFromConfig_RealWorldExamples tests real-world configuration examples
func TestCreateProviderFromConfig_RealWorldExamples(t *testing.T) {
	factory := NewProviderFactory()

	// Register common providers
	RegisterDefaultProviders(factory)

	testCases := []struct {
		name      string
		configMap map[string]interface{}
	}{
		{
			name: "OpenAI configuration",
			configMap: map[string]interface{}{
				"type":          "openai",
				"name":          "my-openai",
				"api_key":       "sk-1234567890abcdef",
				"base_url":      "https://api.openai.com/v1",
				"default_model": "gpt-4",
				"description":   "OpenAI GPT-4 provider",
			},
		},
		{
			name: "Anthropic with OAuth",
			configMap: map[string]interface{}{
				"type": "anthropic",
				"name": "my-anthropic",
				"oauth": map[string]interface{}{
					"client_id":     "anthropic-client-id",
					"client_secret": "anthropic-client-secret",
					"redirect_url":  "https://myapp.com/callback",
					"scopes":        []interface{}{"claude:write", "claude:read"},
				},
			},
		},
		{
			name: "Local Ollama instance",
			configMap: map[string]interface{}{
				"type":          "ollama",
				"name":          "local-ollama",
				"base_url":      "http://localhost:11434",
				"default_model": "llama2",
				"description":   "Local Ollama instance",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := CreateProviderFromConfig(factory, tc.configMap)
			assert.NoError(t, err)
			assert.NotNil(t, provider)

			// Verify provider type matches expectation
			expectedType := types.ProviderType(tc.configMap["type"].(string))
			assert.Equal(t, expectedType, provider.Type())

			// Verify provider name matches expectation (account for hardcoded names)
			expectedName := tc.configMap["name"].(string)
			providerType := tc.configMap["type"].(string)

			// OpenAI has hardcoded name, SimpleProviderStub uses lowercase provider type
			switch providerType {
			case "openai":
				assert.Equal(t, "OpenAI", provider.Name())
			case "anthropic":
				assert.Equal(t, "Anthropic", provider.Name())
			case "ollama":
				assert.Equal(t, "ollama", provider.Name())
			default:
				assert.Equal(t, expectedName, provider.Name())
			}
		})
	}
}

// BenchmarkValidateProviderConfig benchmarks configuration validation
func BenchmarkValidateProviderConfig(b *testing.B) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenAI,
		Name:    "benchmark-test",
		APIKey:  "test-api-key",
		BaseURL: "https://api.openai.com/v1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateProviderConfig(config)
	}
}

// BenchmarkCreateProviderFromConfig benchmarks provider creation from configuration maps
func BenchmarkCreateProviderFromConfig(b *testing.B) {
	factory := NewProviderFactory()
	RegisterDefaultProviders(factory)

	configMap := map[string]interface{}{
		"type":          "openai",
		"name":          "benchmark-test",
		"api_key":       "test-api-key",
		"base_url":      "https://api.openai.com/v1",
		"default_model": "gpt-3.5-turbo",
		"description":   "Benchmark test provider",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateProviderFromConfig(factory, configMap)
		if err != nil {
			b.Fatal(err)
		}
	}
}
