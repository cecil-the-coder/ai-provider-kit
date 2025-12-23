// Package copilot provides tests for the GitHub Copilot AI provider implementation.
package copilot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCopilotProvider_VariousConfigs tests provider creation with different configurations
func TestNewCopilotProvider_VariousConfigs(t *testing.T) {
	tests := []struct {
		name        string
		config      types.ProviderConfig
		checkFunc   func(t *testing.T, p *CopilotProvider)
		expectError bool
	}{
		{
			name: "minimal config",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
			},
			checkFunc: func(t *testing.T, p *CopilotProvider) {
				assert.Equal(t, "Copilot", p.Name())
				assert.Equal(t, types.ProviderTypeCopilot, p.Type())
				assert.Equal(t, copilotDefaultModel, p.GetDefaultModel())
				assert.Equal(t, CopilotBaseURL, p.GetBaseURL())
				assert.Equal(t, AccountTypeIndividual, p.config.AccountType)
			},
		},
		{
			name: "with custom display name",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"display_name": "My Copilot",
				},
			},
			checkFunc: func(t *testing.T, p *CopilotProvider) {
				assert.Equal(t, "My Copilot", p.Name())
			},
		},
		{
			name: "with custom model",
			config: types.ProviderConfig{
				Type:         types.ProviderTypeCopilot,
				DefaultModel: "gpt-4o-mini",
			},
			checkFunc: func(t *testing.T, p *CopilotProvider) {
				assert.Equal(t, "gpt-4o-mini", p.GetDefaultModel())
			},
		},
		{
			name: "business account type",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"account_type": AccountTypeBusiness,
				},
			},
			checkFunc: func(t *testing.T, p *CopilotProvider) {
				assert.Equal(t, AccountTypeBusiness, p.config.AccountType)
				assert.Equal(t, CopilotBusinessBaseURL, p.GetBaseURL())
			},
		},
		{
			name: "enterprise account type",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"account_type": AccountTypeEnterprise,
				},
			},
			checkFunc: func(t *testing.T, p *CopilotProvider) {
				assert.Equal(t, AccountTypeEnterprise, p.config.AccountType)
				assert.Equal(t, CopilotEnterpriseBaseURL, p.GetBaseURL())
			},
		},
		{
			name: "with custom base URL",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"base_url": "https://custom.example.com",
				},
			},
			checkFunc: func(t *testing.T, p *CopilotProvider) {
				assert.Equal(t, "https://custom.example.com", p.GetBaseURL())
			},
		},
		{
			name: "with GitHub token only",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"github_token": "gh_test_token_123",
				},
			},
			checkFunc: func(t *testing.T, p *CopilotProvider) {
				assert.Equal(t, "gh_test_token_123", p.githubToken)
			},
		},
		{
			name: "with Copilot token",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"copilot_token": "copilot_test_token_456",
				},
			},
			checkFunc: func(t *testing.T, p *CopilotProvider) {
				assert.Equal(t, "copilot_test_token_456", p.copilotToken)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCopilotProvider(tt.config)
			require.NotNil(t, provider)
			if tt.checkFunc != nil {
				tt.checkFunc(t, provider)
			}
		})
	}
}

// TestCopilotProvider_Description tests provider description methods
func TestCopilotProvider_Description(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	}
	provider := NewCopilotProvider(config)

	assert.Equal(t, "GitHub Copilot with OpenAI-compatible API", provider.Description())
}

// TestCopilotProvider_Supports tests provider capabilities
func TestCopilotProvider_Supports(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	}
	provider := NewCopilotProvider(config)

	assert.True(t, provider.SupportsToolCalling(), "should support tool calling")
	assert.True(t, provider.SupportsStreaming(), "should support streaming")
	assert.False(t, provider.SupportsResponsesAPI(), "should not support Responses API")
	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat(), "should use OpenAI tool format")
}

// TestCopilotProvider_Configure tests provider configuration updates
func TestCopilotProvider_Configure(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	tests := []struct {
		name     string
		config   types.ProviderConfig
		validate func(t *testing.T, p *CopilotProvider)
	}{
		{
			name: "update display name and account type",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"display_name": "Updated Copilot",
					"account_type": AccountTypeBusiness,
				},
			},
			validate: func(t *testing.T, p *CopilotProvider) {
				assert.Equal(t, "Updated Copilot", p.displayName)
				assert.Equal(t, AccountTypeBusiness, p.config.AccountType)
			},
		},
		{
			name: "update GitHub token",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"github_token": "new_github_token",
				},
			},
			validate: func(t *testing.T, p *CopilotProvider) {
				p.tokenMutex.RLock()
				defer p.tokenMutex.RUnlock()
				assert.Equal(t, "new_github_token", p.githubToken)
			},
		},
		{
			name: "update Copilot token",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"copilot_token": "new_copilot_token",
				},
			},
			validate: func(t *testing.T, p *CopilotProvider) {
				p.tokenMutex.RLock()
				defer p.tokenMutex.RUnlock()
				assert.Equal(t, "new_copilot_token", p.copilotToken)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.Configure(tt.config)
			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, provider)
			}
		})
	}
}

// TestCopilotProvider_IsAuthenticated tests authentication state checking
func TestCopilotProvider_IsAuthenticated(t *testing.T) {
	t.Run("not authenticated initially", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		assert.False(t, provider.IsAuthenticated())
	})

	t.Run("authenticated with both tokens", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		provider.tokenMutex.Lock()
		provider.githubToken = "test_github_token"
		provider.copilotToken = "test_copilot_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		assert.True(t, provider.IsAuthenticated())
	})

	t.Run("not authenticated with only GitHub token", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		provider.tokenMutex.Lock()
		provider.githubToken = "test_github_token"
		provider.copilotToken = ""
		provider.tokenMutex.Unlock()

		assert.False(t, provider.IsAuthenticated())
	})

	t.Run("not authenticated with expired Copilot token", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		provider.tokenMutex.Lock()
		provider.githubToken = "test_github_token"
		provider.copilotToken = "test_copilot_token"
		provider.copilotTokenExpiry = time.Now().Add(-1 * time.Hour) // Expired
		provider.tokenMutex.Unlock()

		assert.False(t, provider.IsAuthenticated())
	})
}

// TestCopilotProvider_IsOAuthConfigured tests OAuth configuration check
func TestCopilotProvider_IsOAuthConfigured(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		assert.False(t, provider.IsOAuthConfigured())
	})

	t.Run("configured with GitHub token", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"github_token": "test_github_token",
			},
		})
		assert.True(t, provider.IsOAuthConfigured())
	})
}

// TestCopilotProvider_IsAPIKeyConfigured tests API key configuration check
func TestCopilotProvider_IsAPIKeyConfigured(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		assert.False(t, provider.IsAPIKeyConfigured())
	})

	t.Run("configured with valid Copilot token", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_copilot_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		assert.True(t, provider.IsAPIKeyConfigured())
	})

	t.Run("not configured with expired Copilot token", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_copilot_token"
		provider.copilotTokenExpiry = time.Now().Add(-1 * time.Hour)
		provider.tokenMutex.Unlock()

		assert.False(t, provider.IsAPIKeyConfigured())
	})
}

// TestCopilotProvider_GetAuthStatus tests authentication status retrieval
func TestCopilotProvider_GetAuthStatus(t *testing.T) {
	t.Run("not authenticated", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		status := provider.GetAuthStatus()

		assert.False(t, status["github_authenticated"].(bool))
		assert.False(t, status["copilot_authenticated"].(bool))
	})

	t.Run("authenticated", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})
		provider.tokenMutex.Lock()
		provider.githubToken = "test_github_token"
		provider.copilotToken = "test_copilot_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.refreshIn = 1200
		provider.tokenMutex.Unlock()

		status := provider.GetAuthStatus()

		assert.True(t, status["github_authenticated"].(bool))
		assert.True(t, status["copilot_authenticated"].(bool))
		assert.NotNil(t, status["token_expires_at"])
		assert.Equal(t, 1200, status["refresh_in_seconds"])
		assert.NotEmpty(t, status["time_until_expiry"])
	})
}

// TestCopilotProvider_Logout tests logout functionality
func TestCopilotProvider_Logout(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})
	provider.tokenMutex.Lock()
	provider.githubToken = "test_github_token"
	provider.copilotToken = "test_copilot_token"
	provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
	provider.cancelRefresh = func() {} // Mock cancel function
	provider.tokenMutex.Unlock()

	assert.True(t, provider.IsAuthenticated())

	ctx := context.Background()
	err := provider.Logout(ctx)
	require.NoError(t, err)

	assert.False(t, provider.IsAuthenticated())
}

// TestCopilotProvider_InvokeServerTool tests server tool invocation
func TestCopilotProvider_InvokeServerTool(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	ctx := context.Background()
	result, err := provider.InvokeServerTool(ctx, "test_tool", map[string]interface{}{"key": "value"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
	assert.Nil(t, result)
}

// TestCopilotProvider_TestConnectivity tests connectivity testing
func TestCopilotProvider_TestConnectivity(t *testing.T) {
	t.Run("successful connectivity test", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "Bearer test_token", r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4o","name":"GPT-4o"}]}`))
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:         types.ProviderTypeCopilot,
			BaseURL:      server.URL,
			ProviderConfig: map[string]interface{}{
				"copilot_token": "test_token",
			},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		err := provider.TestConnectivity(context.Background())
		assert.NoError(t, err)
	})

	t.Run("unauthorized response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:         types.ProviderTypeCopilot,
			BaseURL:      server.URL,
			ProviderConfig: map[string]interface{}{
				"copilot_token": "test_token",
			},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		err := provider.TestConnectivity(context.Background())
		assert.Error(t, err)
	})

	t.Run("no Copilot token", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})

		err := provider.TestConnectivity(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get Copilot token")
	})
}

// TestCopilotProvider_RefreshAllOAuthTokens tests token refresh
func TestCopilotProvider_RefreshAllOAuthTokens(t *testing.T) {
	t.Run("refresh without GitHub token returns error", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})

		err := provider.RefreshAllOAuthTokens(context.Background())
		assert.Error(t, err)
	})
}

// TestCopilotProvider_HealthCheck tests health check functionality
func TestCopilotProvider_HealthCheck(t *testing.T) {
	t.Run("healthy when authenticated", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"object":"list","data":[]}`))
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.githubToken = "test_github_token"
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		err := provider.HealthCheck(context.Background())
		assert.NoError(t, err)
	})

	t.Run("unhealthy when not authenticated", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})

		err := provider.HealthCheck(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not authenticated")
	})
}

// TestHasVisionContent tests vision content detection
func TestHasVisionContent(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	tests := []struct {
		name     string
		messages []ChatMessage
		expected bool
	}{
		{
			name: "no vision content",
			messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
			expected: false,
		},
		{
			name: "with vision content",
			messages: []ChatMessage{
				{
					Role: "user",
					Content: []ContentPart{
						{Type: "text", Text: "What's in this image?"},
						{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/image.png"}},
					},
				},
			},
			expected: true,
		},
		{
			name: "mixed content",
			messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{
					Role: "user",
					Content: []ContentPart{
						{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/image.png"}},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.hasVisionContent(tt.messages)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetInitiator tests initiator determination
func TestGetInitiator(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	tests := []struct {
		name     string
		messages []ChatMessage
		expected string
	}{
		{
			name:     "user initiated",
			messages: []ChatMessage{{Role: "user", Content: "Hello"}},
			expected: "user",
		},
		{
			name: "agent initiated",
			messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			},
			expected: "agent",
		},
		{
			name: "tool response",
			messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi!", ToolCalls: []ToolCall{{ID: "call1"}}},
				{Role: "tool", Content: "Result"},
			},
			expected: "agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.getInitiator(tt.messages)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCopilotProvider_GetModels tests models retrieval with fallback
func TestCopilotProvider_GetModels(t *testing.T) {
	t.Run("fallback when not authenticated", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})

		models, err := provider.GetModels(context.Background())
		require.NoError(t, err)
		assert.NotEmpty(t, models, "should return fallback models")

		// Check for expected models
		modelMap := make(map[string]types.Model)
		for _, m := range models {
			modelMap[m.ID] = m
		}
		assert.Contains(t, modelMap, "gpt-4o")
		assert.Contains(t, modelMap, "gpt-4o-mini")
	})
}
