package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderTypeConstants tests all provider type constants
func TestProviderTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant ProviderType
		expected string
	}{
		{"OpenAI", ProviderTypeOpenAI, "openai"},
		{"Anthropic", ProviderTypeAnthropic, "anthropic"},
		{"Gemini", ProviderTypeGemini, "gemini"},
		{"Qwen", ProviderTypeQwen, "qwen"},
		{"Cerebras", ProviderTypeCerebras, "cerebras"},
		{"OpenRouter", ProviderTypeOpenRouter, "openrouter"},
		{"Synthetic", ProviderTypeSynthetic, "synthetic"},
		{"xAI", ProviderTypexAI, "xai"},
		{"Fireworks", ProviderTypeFireworks, "fireworks"},
		{"Deepseek", ProviderTypeDeepseek, "deepseek"},
		{"Mistral", ProviderTypeMistral, "mistral"},
		{"LMStudio", ProviderTypeLMStudio, "lmstudio"},
		{"LlamaCpp", ProviderTypeLlamaCpp, "llamacpp"},
		{"Ollama", ProviderTypeOllama, "ollama"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.constant))

			// Test JSON marshaling/unmarshaling
			data, err := json.Marshal(tt.constant)
			require.NoError(t, err)
			assert.Equal(t, `"`+tt.expected+`"`, string(data))

			var result ProviderType
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)
			assert.Equal(t, tt.constant, result)
		})
	}
}

// TestProviderTypeValidation tests provider type validation
func TestProviderTypeValidation(t *testing.T) {
	validProviders := []ProviderType{
		ProviderTypeOpenAI, ProviderTypeAnthropic, ProviderTypeGemini,
		ProviderTypeQwen, ProviderTypeCerebras, ProviderTypeOpenRouter,
		ProviderTypeSynthetic, ProviderTypexAI, ProviderTypeFireworks,
		ProviderTypeDeepseek, ProviderTypeMistral, ProviderTypeLMStudio,
		ProviderTypeLlamaCpp, ProviderTypeOllama,
	}

	// All valid providers should have non-empty string representations
	for _, provider := range validProviders {
		t.Run(string(provider), func(t *testing.T) {
			assert.NotEmpty(t, string(provider))
			assert.NotEqual(t, "", string(provider))
		})
	}

	// Test custom provider type
	customProvider := ProviderType("custom-provider")
	assert.Equal(t, "custom-provider", string(customProvider))
}

// TestAuthMethodConstants tests all authentication method constants
func TestAuthMethodConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant AuthMethod
		expected string
	}{
		{"APIKey", AuthMethodAPIKey, "api_key"},
		{"BearerToken", AuthMethodBearerToken, "bearer_token"},
		{"OAuth", AuthMethodOAuth, "oauth"},
		{"Custom", AuthMethodCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.constant))

			// Test JSON marshaling/unmarshaling
			data, err := json.Marshal(tt.constant)
			require.NoError(t, err)
			assert.Equal(t, `"`+tt.expected+`"`, string(data))

			var result AuthMethod
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)
			assert.Equal(t, tt.constant, result)
		})
	}
}

// TestToolFormatConstants tests all tool format constants
func TestToolFormatConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant ToolFormat
		expected string
	}{
		{"OpenAI", ToolFormatOpenAI, "openai"},
		{"Anthropic", ToolFormatAnthropic, "anthropic"},
		{"XML", ToolFormatXML, "xml"},
		{"Hermes", ToolFormatHermes, "hermes"},
		{"Text", ToolFormatText, "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.constant))

			// Test JSON marshaling/unmarshaling
			data, err := json.Marshal(tt.constant)
			require.NoError(t, err)
			assert.Equal(t, `"`+tt.expected+`"`, string(data))

			var result ToolFormat
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)
			assert.Equal(t, tt.constant, result)
		})
	}
}

// TestHealthStatus tests the HealthStatus struct
func TestHealthStatus(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var hs HealthStatus
		assert.False(t, hs.Healthy)
		assert.True(t, hs.LastChecked.IsZero())
		assert.Empty(t, hs.Message)
		assert.Equal(t, 0.0, hs.ResponseTime)
		assert.Equal(t, 0, hs.StatusCode)
	})

	t.Run("Creation", func(t *testing.T) {
		now := time.Now()
		hs := HealthStatus{
			Healthy:      true,
			LastChecked:  now,
			Message:      "All good",
			ResponseTime: 150.5,
			StatusCode:   200,
		}
		assert.True(t, hs.Healthy)
		assert.Equal(t, now, hs.LastChecked)
		assert.Equal(t, "All good", hs.Message)
		assert.Equal(t, 150.5, hs.ResponseTime)
		assert.Equal(t, 200, hs.StatusCode)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		hs := HealthStatus{
			Healthy:      true,
			LastChecked:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Message:      "Service healthy",
			ResponseTime: 123.45,
			StatusCode:   200,
		}

		data, err := json.Marshal(hs)
		require.NoError(t, err)

		var result HealthStatus
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, hs, result)
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		hs := HealthStatus{}

		// Update to healthy
		hs.UpdateStatus(true, 200, 100.5, "OK")
		assert.True(t, hs.Healthy)
		assert.Equal(t, 200, hs.StatusCode)
		assert.Equal(t, 100.5, hs.ResponseTime)
		assert.Equal(t, "OK", hs.Message)
		assert.False(t, hs.LastChecked.IsZero())

		// Update to unhealthy
		hs.UpdateStatus(false, 500, 1000.0, "Internal Server Error")
		assert.False(t, hs.Healthy)
		assert.Equal(t, 500, hs.StatusCode)
		assert.Equal(t, 1000.0, hs.ResponseTime)
		assert.Equal(t, "Internal Server Error", hs.Message)
	})
}

// UpdateStatus is a helper method to update health status
func (h *HealthStatus) UpdateStatus(healthy bool, statusCode int, responseTime float64, message string) {
	h.Healthy = healthy
	h.StatusCode = statusCode
	h.ResponseTime = responseTime
	h.Message = message
	h.LastChecked = time.Now()
}

// TestOAuthConfig tests the OAuthConfig struct
func TestOAuthConfig(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var config OAuthConfig
		assert.Empty(t, config.ClientID)
		assert.Empty(t, config.ClientSecret)
		assert.Empty(t, config.RedirectURL)
		assert.Empty(t, config.Scopes)
		assert.Empty(t, config.AuthURL)
		assert.Empty(t, config.TokenURL)
		assert.Empty(t, config.RefreshToken)
		assert.Empty(t, config.AccessToken)
		assert.True(t, config.ExpiresAt.IsZero())
	})

	t.Run("Creation", func(t *testing.T) {
		expiresAt := time.Now().Add(time.Hour)
		config := OAuthConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "https://example.com/callback",
			Scopes:       []string{"read", "write"},
			AuthURL:      "https://auth.example.com/oauth/authorize",
			TokenURL:     "https://auth.example.com/oauth/token",
			RefreshToken: "refresh-token",
			AccessToken:  "access-token",
			ExpiresAt:    expiresAt,
		}

		assert.Equal(t, "test-client-id", config.ClientID)
		assert.Equal(t, "test-client-secret", config.ClientSecret)
		assert.Equal(t, "https://example.com/callback", config.RedirectURL)
		assert.Equal(t, []string{"read", "write"}, config.Scopes)
		assert.Equal(t, "https://auth.example.com/oauth/authorize", config.AuthURL)
		assert.Equal(t, "https://auth.example.com/oauth/token", config.TokenURL)
		assert.Equal(t, "refresh-token", config.RefreshToken)
		assert.Equal(t, "access-token", config.AccessToken)
		assert.Equal(t, expiresAt, config.ExpiresAt)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		expiresAt := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
		config := OAuthConfig{
			ClientID:     "client123",
			ClientSecret: "secret456",
			Scopes:       []string{"profile", "email"},
			ExpiresAt:    expiresAt,
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var result OAuthConfig
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, config, result)
	})

	t.Run("TokenExpired", func(t *testing.T) {
		config := OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(-time.Hour), // Expired 1 hour ago
		}
		assert.True(t, config.IsTokenExpired())

		config.ExpiresAt = time.Now().Add(time.Hour) // Expires in 1 hour
		assert.False(t, config.IsTokenExpired())

		config.ExpiresAt = time.Time{} // No expiration set
		assert.False(t, config.IsTokenExpired())
	})

	t.Run("TokenValid", func(t *testing.T) {
		config := OAuthConfig{}
		assert.False(t, config.IsTokenValid())

		config.AccessToken = "valid-token"
		assert.True(t, config.IsTokenValid()) // Valid token with no expiration

		config.ExpiresAt = time.Now().Add(time.Hour)
		assert.True(t, config.IsTokenValid())

		config.ExpiresAt = time.Now().Add(-time.Hour)
		assert.False(t, config.IsTokenValid()) // Expired
	})
}

// IsTokenExpired checks if the access token is expired
func (c *OAuthConfig) IsTokenExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false // No expiration set, assume not expired
	}
	return time.Now().After(c.ExpiresAt)
}

// IsTokenValid checks if the access token is valid (not expired and has a value)
func (c *OAuthConfig) IsTokenValid() bool {
	return c.AccessToken != "" && !c.IsTokenExpired()
}

// TestProviderConfig tests the ProviderConfig struct
func TestProviderConfig(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var config ProviderConfig
		assert.Equal(t, ProviderType(""), config.Type)
		assert.Empty(t, config.Name)
		assert.Empty(t, config.BaseURL)
		assert.Empty(t, config.APIKey)
		assert.Empty(t, config.APIKeyEnv)
		assert.Empty(t, config.DefaultModel)
		assert.Empty(t, config.Description)
		assert.Nil(t, config.ProviderConfig)
		assert.Nil(t, config.OAuthCredentials)
		assert.False(t, config.SupportsStreaming)
		assert.False(t, config.SupportsToolCalling)
		assert.False(t, config.SupportsResponsesAPI)
		assert.Equal(t, 0, config.MaxTokens)
		assert.Equal(t, time.Duration(0), config.Timeout)
		assert.Equal(t, ToolFormat(""), config.ToolFormat)
	})

	t.Run("FullConfiguration", func(t *testing.T) {
		oauthCreds := []*OAuthCredentialSet{
			{
				ID:           "cred-1",
				ClientID:     "oauth-client",
				ClientSecret: "oauth-secret",
				Scopes:       []string{"api"},
			},
		}

		extraConfig := map[string]interface{}{
			"temperature": 0.7,
			"top_p":       0.9,
			"custom":      "value",
		}

		config := ProviderConfig{
			Type:                 ProviderTypeOpenAI,
			Name:                 "test-openai",
			BaseURL:              "https://api.openai.com",
			APIKey:               "sk-test-key",
			APIKeyEnv:            "OPENAI_API_KEY",
			DefaultModel:         "gpt-4",
			Description:          "Test OpenAI provider",
			ProviderConfig:       extraConfig,
			OAuthCredentials:     oauthCreds,
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
			MaxTokens:            4096,
			Timeout:              30 * time.Second,
			ToolFormat:           ToolFormatOpenAI,
		}

		assert.Equal(t, ProviderTypeOpenAI, config.Type)
		assert.Equal(t, "test-openai", config.Name)
		assert.Equal(t, "https://api.openai.com", config.BaseURL)
		assert.Equal(t, "sk-test-key", config.APIKey)
		assert.Equal(t, "OPENAI_API_KEY", config.APIKeyEnv)
		assert.Equal(t, "gpt-4", config.DefaultModel)
		assert.Equal(t, "Test OpenAI provider", config.Description)
		assert.Equal(t, extraConfig, config.ProviderConfig)
		assert.Equal(t, oauthCreds, config.OAuthCredentials)
		assert.True(t, config.SupportsStreaming)
		assert.True(t, config.SupportsToolCalling)
		assert.False(t, config.SupportsResponsesAPI)
		assert.Equal(t, 4096, config.MaxTokens)
		assert.Equal(t, 30*time.Second, config.Timeout)
		assert.Equal(t, ToolFormatOpenAI, config.ToolFormat)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		extraConfig := map[string]interface{}{
			"custom_field": "custom_value",
			"number":       42,
		}

		config := ProviderConfig{
			Type:           ProviderTypeAnthropic,
			Name:           "test-anthropic",
			APIKey:         "sk-ant-test",
			DefaultModel:   "claude-3-sonnet",
			Description:    "Test Anthropic provider",
			ProviderConfig: extraConfig,
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var result ProviderConfig
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, config.Type, result.Type)
		assert.Equal(t, config.Name, result.Name)
		assert.Equal(t, config.APIKey, result.APIKey)
		assert.Equal(t, config.DefaultModel, result.DefaultModel)
		assert.Equal(t, config.Description, result.Description)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name   string
			config ProviderConfig
			valid  bool
		}{
			{
				name:   "Empty",
				config: ProviderConfig{},
				valid:  false,
			},
			{
				name: "OnlyType",
				config: ProviderConfig{
					Type: ProviderTypeOpenAI,
				},
				valid: false,
			},
			{
				name: "TypeAndName",
				config: ProviderConfig{
					Type: ProviderTypeOpenAI,
					Name: "test",
				},
				valid: true,
			},
			{
				name: "WithAPIKey",
				config: ProviderConfig{
					Type:   ProviderTypeOpenAI,
					Name:   "test",
					APIKey: "sk-test",
				},
				valid: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				valid := tt.config.Validate()
				assert.Equal(t, tt.valid, valid)
			})
		}
	})
}

// Validate checks if the provider configuration is valid
func (c *ProviderConfig) Validate() bool {
	return c.Type != "" && c.Name != ""
}

// TestProviderMetrics tests the ProviderMetrics struct
func TestProviderMetrics(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var metrics ProviderMetrics
		assert.Equal(t, int64(0), metrics.RequestCount)
		assert.Equal(t, int64(0), metrics.SuccessCount)
		assert.Equal(t, int64(0), metrics.ErrorCount)
		assert.Equal(t, time.Duration(0), metrics.TotalLatency)
		assert.Equal(t, time.Duration(0), metrics.AverageLatency)
		assert.True(t, metrics.LastRequestTime.IsZero())
		assert.True(t, metrics.LastSuccessTime.IsZero())
		assert.True(t, metrics.LastErrorTime.IsZero())
		assert.Empty(t, metrics.LastError)
		assert.Equal(t, int64(0), metrics.TokensUsed)
	})

	t.Run("RecordSuccess", func(t *testing.T) {
		metrics := ProviderMetrics{}
		latency := 100 * time.Millisecond
		tokens := int64(50)

		metrics.RecordRequest(true, latency, &Usage{
			PromptTokens:     20,
			CompletionTokens: 30,
			TotalTokens:      50,
		})

		assert.Equal(t, int64(1), metrics.RequestCount)
		assert.Equal(t, int64(1), metrics.SuccessCount)
		assert.Equal(t, int64(0), metrics.ErrorCount)
		assert.Equal(t, latency, metrics.TotalLatency)
		assert.Equal(t, latency, metrics.AverageLatency)
		assert.Equal(t, tokens, metrics.TokensUsed)
		assert.False(t, metrics.LastRequestTime.IsZero())
		assert.False(t, metrics.LastSuccessTime.IsZero())
		assert.True(t, metrics.LastErrorTime.IsZero())
		assert.Empty(t, metrics.LastError)
	})

	t.Run("RecordError", func(t *testing.T) {
		metrics := ProviderMetrics{}
		latency := 200 * time.Millisecond

		metrics.RecordRequest(false, latency, nil)

		assert.Equal(t, int64(1), metrics.RequestCount)
		assert.Equal(t, int64(0), metrics.SuccessCount)
		assert.Equal(t, int64(1), metrics.ErrorCount)
		assert.Equal(t, latency, metrics.TotalLatency)
		assert.Equal(t, latency, metrics.AverageLatency)
		assert.False(t, metrics.LastRequestTime.IsZero())
		assert.True(t, metrics.LastSuccessTime.IsZero())
		assert.False(t, metrics.LastErrorTime.IsZero())
	})

	t.Run("AverageLatency", func(t *testing.T) {
		metrics := ProviderMetrics{}

		// Record multiple requests
		latencies := []time.Duration{
			100 * time.Millisecond,
			200 * time.Millisecond,
			300 * time.Millisecond,
		}

		for _, latency := range latencies {
			metrics.RecordRequest(true, latency, nil)
		}

		expectedAverage := time.Duration(200) * time.Millisecond
		assert.Equal(t, expectedAverage, metrics.AverageLatency)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		metrics := ProviderMetrics{
			RequestCount:    100,
			SuccessCount:    95,
			ErrorCount:      5,
			TotalLatency:    10000 * time.Millisecond,
			AverageLatency:  100 * time.Millisecond,
			LastRequestTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			LastSuccessTime: time.Date(2024, 1, 1, 11, 59, 0, 0, time.UTC),
			LastErrorTime:   time.Date(2024, 1, 1, 11, 58, 0, 0, time.UTC),
			LastError:       "timeout error",
			TokensUsed:      10000,
		}

		data, err := json.Marshal(metrics)
		require.NoError(t, err)

		var result ProviderMetrics
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, metrics.RequestCount, result.RequestCount)
		assert.Equal(t, metrics.SuccessCount, result.SuccessCount)
		assert.Equal(t, metrics.ErrorCount, result.ErrorCount)
		assert.Equal(t, metrics.TokensUsed, result.TokensUsed)
		assert.Equal(t, metrics.LastError, result.LastError)
	})
}

// RecordRequest records a request and updates metrics
func (m *ProviderMetrics) RecordRequest(success bool, latency time.Duration, usage *Usage) {
	m.RequestCount++
	m.TotalLatency += latency
	m.AverageLatency = m.TotalLatency / time.Duration(m.RequestCount)
	m.LastRequestTime = time.Now()

	if success {
		m.SuccessCount++
		m.LastSuccessTime = time.Now()
		if usage != nil {
			m.TokensUsed += int64(usage.TotalTokens)
		}
	} else {
		m.ErrorCount++
		m.LastErrorTime = time.Now()
	}
}

// TestProviderInfo tests the ProviderInfo struct
func TestProviderInfo(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var info ProviderInfo
		assert.Empty(t, info.Name)
		assert.Equal(t, ProviderType(""), info.Type)
		assert.Empty(t, info.Description)
		assert.Empty(t, info.Models)
		assert.Empty(t, info.SupportedTools)
		assert.Empty(t, info.DefaultModel)
	})

	t.Run("FullInformation", func(t *testing.T) {
		models := []Model{
			{ID: "gpt-4", Name: "GPT-4"},
			{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo"},
		}
		healthStatus := HealthStatus{
			Healthy:      true,
			LastChecked:  time.Now(),
			Message:      "All systems operational",
			ResponseTime: 50.0,
			StatusCode:   200,
		}

		info := ProviderInfo{
			Name:           "OpenAI",
			Type:           ProviderTypeOpenAI,
			Description:    "OpenAI API provider",
			HealthStatus:   healthStatus,
			Models:         models,
			SupportedTools: []string{"code_interpreter", "browsing"},
			DefaultModel:   "gpt-4",
		}

		assert.Equal(t, "OpenAI", info.Name)
		assert.Equal(t, ProviderTypeOpenAI, info.Type)
		assert.Equal(t, "OpenAI API provider", info.Description)
		assert.Equal(t, healthStatus, info.HealthStatus)
		assert.Equal(t, models, info.Models)
		assert.Equal(t, []string{"code_interpreter", "browsing"}, info.SupportedTools)
		assert.Equal(t, "gpt-4", info.DefaultModel)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		info := ProviderInfo{
			Name:         "Anthropic",
			Type:         ProviderTypeAnthropic,
			Description:  "Anthropic Claude API",
			DefaultModel: "claude-3-sonnet",
		}

		data, err := json.Marshal(info)
		require.NoError(t, err)

		var result ProviderInfo
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, info.Name, result.Name)
		assert.Equal(t, info.Type, result.Type)
		assert.Equal(t, info.Description, result.Description)
		assert.Equal(t, info.DefaultModel, result.DefaultModel)
	})
}

// TestAuthConfig tests the AuthConfig struct
func TestAuthConfig(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var config AuthConfig
		assert.Equal(t, AuthMethod(""), config.Method)
		assert.Empty(t, config.APIKey)
		assert.Empty(t, config.BaseURL)
		assert.Empty(t, config.DefaultModel)
	})

	t.Run("APIKeyAuth", func(t *testing.T) {
		config := AuthConfig{
			Method:       AuthMethodAPIKey,
			APIKey:       "sk-test-key",
			BaseURL:      "https://api.example.com",
			DefaultModel: "model-name",
		}

		assert.Equal(t, AuthMethodAPIKey, config.Method)
		assert.Equal(t, "sk-test-key", config.APIKey)
		assert.Equal(t, "https://api.example.com", config.BaseURL)
		assert.Equal(t, "model-name", config.DefaultModel)
	})

	t.Run("BearerTokenAuth", func(t *testing.T) {
		config := AuthConfig{
			Method:       AuthMethodBearerToken,
			APIKey:       "bearer-token-12345",
			BaseURL:      "https://api.example.com",
			DefaultModel: "model-name",
		}

		assert.Equal(t, AuthMethodBearerToken, config.Method)
		assert.Equal(t, "bearer-token-12345", config.APIKey)
		assert.Equal(t, "https://api.example.com", config.BaseURL)
		assert.Equal(t, "model-name", config.DefaultModel)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		config := AuthConfig{
			Method:       AuthMethodBearerToken,
			APIKey:       "bearer-token",
			BaseURL:      "https://api.example.com",
			DefaultModel: "gpt-4",
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)

		var result AuthConfig
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, config.Method, result.Method)
		assert.Equal(t, config.APIKey, result.APIKey)
		assert.Equal(t, config.BaseURL, result.BaseURL)
		assert.Equal(t, config.DefaultModel, result.DefaultModel)
	})
}

// TestInterfaces tests that interface types can be implemented
func TestInterfaces(t *testing.T) {
	t.Run("TokenStorage", func(t *testing.T) {
		storage := &MockTokenStorage{
			tokens: make(map[string]*OAuthConfig),
		}

		config := &OAuthConfig{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		}

		// Test StoreToken
		err := storage.StoreToken("test-key", config)
		assert.NoError(t, err)

		// Test RetrieveToken
		retrieved, err := storage.RetrieveToken("test-key")
		assert.NoError(t, err)
		assert.Equal(t, config, retrieved)

		// Test ListTokens
		keys, err := storage.ListTokens()
		assert.NoError(t, err)
		assert.Contains(t, keys, "test-key")

		// Test DeleteToken
		err = storage.DeleteToken("test-key")
		assert.NoError(t, err)

		keys, err = storage.ListTokens()
		assert.NoError(t, err)
		assert.NotContains(t, keys, "test-key")
	})
}

// MockTokenStorage is a mock implementation of TokenStorage interface
type MockTokenStorage struct {
	tokens map[string]*OAuthConfig
}

func (m *MockTokenStorage) StoreToken(key string, token *OAuthConfig) error {
	if m.tokens == nil {
		m.tokens = make(map[string]*OAuthConfig)
	}
	m.tokens[key] = token
	return nil
}

func (m *MockTokenStorage) RetrieveToken(key string) (*OAuthConfig, error) {
	if token, exists := m.tokens[key]; exists {
		return token, nil
	}
	return nil, nil
}

func (m *MockTokenStorage) DeleteToken(key string) error {
	delete(m.tokens, key)
	return nil
}

func (m *MockTokenStorage) ListTokens() ([]string, error) {
	keys := make([]string, 0, len(m.tokens))
	for key := range m.tokens {
		keys = append(keys, key)
	}
	return keys, nil
}

// BenchmarkProviderConfigMarshal benchmarks JSON marshaling of ProviderConfig
func BenchmarkProviderConfigMarshal(b *testing.B) {
	config := ProviderConfig{
		Type:         ProviderTypeOpenAI,
		Name:         "test-provider",
		APIKey:       "sk-test-key",
		DefaultModel: "gpt-4",
		Description:  "Test provider for benchmarking",
		ProviderConfig: map[string]interface{}{
			"temperature": 0.7,
			"max_tokens":  2048,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProviderConfigUnmarshal benchmarks JSON unmarshaling of ProviderConfig
func BenchmarkProviderConfigUnmarshal(b *testing.B) {
	config := ProviderConfig{
		Type:         ProviderTypeOpenAI,
		Name:         "test-provider",
		APIKey:       "sk-test-key",
		DefaultModel: "gpt-4",
		Description:  "Test provider for benchmarking",
		ProviderConfig: map[string]interface{}{
			"temperature": 0.7,
			"max_tokens":  2048,
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result ProviderConfig
		err := json.Unmarshal(data, &result)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProviderMetricsUpdate benchmarks updating provider metrics
func BenchmarkProviderMetricsUpdate(b *testing.B) {
	metrics := ProviderMetrics{}
	latency := 100 * time.Millisecond
	usage := &Usage{
		PromptTokens:     20,
		CompletionTokens: 30,
		TotalTokens:      50,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordRequest(true, latency, usage)
	}
}
