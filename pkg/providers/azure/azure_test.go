package azure

import (
	"context"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAzureOpenAIProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure",
		APIKey: "test-key",
		ProviderConfig: map[string]interface{}{
			"resource_name": "my-resource",
			"deployment_id": "gpt-4-deployment",
			"api_version":   "2024-02-15-preview",
		},
	}

	provider := NewAzureOpenAIProvider(config)
	require.NotNil(t, provider)

	assert.Equal(t, "Azure OpenAI", provider.Name())
	assert.Equal(t, types.ProviderTypeAzureOpenAI, provider.Type())
	assert.Equal(t, "Azure OpenAI Service - Enterprise GPT models with Azure integration", provider.Description())
	assert.Equal(t, "my-resource", provider.resourceName)
	assert.Equal(t, "gpt-4-deployment", provider.deploymentID)
	assert.Equal(t, "2024-02-15-preview", provider.apiVersion)
}

func TestAzureOpenAIProvider_GetModels(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure",
		APIKey: "test-key",
		ProviderConfig: map[string]interface{}{
			"resource_name": "my-resource",
			"deployment_id": "gpt-4-deployment",
		},
	}

	provider := NewAzureOpenAIProvider(config)
	models, err := provider.GetModels(context.Background())

	require.NoError(t, err)
	assert.NotEmpty(t, models)

	// Check that we have common Azure OpenAI models
	modelIDs := make([]string, len(models))
	for i, m := range models {
		modelIDs[i] = m.ID
		assert.Equal(t, types.ProviderTypeAzureOpenAI, m.Provider)
	}

	assert.Contains(t, modelIDs, "gpt-4")
	assert.Contains(t, modelIDs, "gpt-35-turbo")
}

func TestAzureOpenAIProvider_GetDefaultModel(t *testing.T) {
	t.Run("UseConfiguredDefaultModel", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:         types.ProviderTypeAzureOpenAI,
			Name:         "test-azure",
			APIKey:       "test-key",
			DefaultModel: "custom-model",
		}

		provider := NewAzureOpenAIProvider(config)
		assert.Equal(t, "custom-model", provider.GetDefaultModel())
	})

	t.Run("UseDeploymentID", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeAzureOpenAI,
			Name:   "test-azure",
			APIKey: "test-key",
			ProviderConfig: map[string]interface{}{
				"deployment_id": "my-deployment",
			},
		}

		provider := NewAzureOpenAIProvider(config)
		assert.Equal(t, "my-deployment", provider.GetDefaultModel())
	})

	t.Run("UseFallbackDefault", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeAzureOpenAI,
			Name:   "test-azure",
			APIKey: "test-key",
		}

		provider := NewAzureOpenAIProvider(config)
		assert.Equal(t, "gpt-35-turbo", provider.GetDefaultModel())
	})
}

func TestAzureOpenAIProvider_Capabilities(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure",
		APIKey: "test-key",
	}

	provider := NewAzureOpenAIProvider(config)

	assert.True(t, provider.SupportsToolCalling())
	assert.True(t, provider.SupportsStreaming())
	assert.False(t, provider.SupportsResponsesAPI())
	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())
}

func TestAzureOpenAIProvider_Authenticate(t *testing.T) {
	t.Run("APIKeyAuth", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeAzureOpenAI,
			Name:   "test-azure",
			APIKey: "old-key",
		}

		provider := NewAzureOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: "new-key",
		}

		err := provider.Authenticate(context.Background(), authConfig)
		assert.NoError(t, err)
		assert.True(t, provider.IsAuthenticated())
	})

	t.Run("AzureADAuth", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeAzureOpenAI,
			Name:   "test-azure",
			APIKey: "key",
		}

		provider := NewAzureOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodBearerToken,
			APIKey: "bearer-token",
		}

		err := provider.Authenticate(context.Background(), authConfig)
		assert.NoError(t, err)
	})

	t.Run("UnsupportedAuthMethod", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeAzureOpenAI,
			Name:   "test-azure",
			APIKey: "key",
		}

		provider := NewAzureOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodOAuth,
		}

		err := provider.Authenticate(context.Background(), authConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "azure OpenAI supports API key and Azure AD")
	})
}

func TestAzureOpenAIProvider_Configure(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure",
		APIKey: "key",
		ProviderConfig: map[string]interface{}{
			"resource_name": "resource1",
			"deployment_id": "deployment1",
		},
	}

	provider := NewAzureOpenAIProvider(config)

	newConfig := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure-updated",
		APIKey: "new-key",
		ProviderConfig: map[string]interface{}{
			"resource_name": "resource2",
			"deployment_id": "deployment2",
			"api_version":   "2024-03-01-preview",
		},
	}

	err := provider.Configure(newConfig)
	require.NoError(t, err)

	assert.Equal(t, "resource2", provider.resourceName)
	assert.Equal(t, "deployment2", provider.deploymentID)
	assert.Equal(t, "2024-03-01-preview", provider.apiVersion)
	assert.Equal(t, "https://resource2.openai.azure.com", provider.baseURL)
}

func TestAzureOpenAIProvider_GetAuthStatus(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure",
		APIKey: "key",
		ProviderConfig: map[string]interface{}{
			"resource_name": "my-resource",
			"deployment_id": "my-deployment",
			"api_version":   "2024-02-15-preview",
			"use_azure_ad":  true,
		},
	}

	provider := NewAzureOpenAIProvider(config)
	status := provider.GetAuthStatus()

	assert.NotNil(t, status)
	assert.Equal(t, true, status["use_azure_ad"])
	assert.Equal(t, "my-resource", status["resource_name"])
	assert.Equal(t, "my-deployment", status["deployment_id"])
	assert.Equal(t, "2024-02-15-preview", status["api_version"])
}

func TestAzureOpenAIProvider_Logout(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure",
		APIKey: "key",
	}

	provider := NewAzureOpenAIProvider(config)
	assert.True(t, provider.IsAuthenticated())

	err := provider.Logout(context.Background())
	assert.NoError(t, err)
}

func TestAzureOpenAIProvider_SetTokenProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure",
		APIKey: "key",
	}

	provider := NewAzureOpenAIProvider(config)

	mockProvider := &mockTokenProvider{token: "test-token"}
	provider.SetTokenProvider(mockProvider)

	assert.NotNil(t, provider.tokenProvider)
	if provider.middleware != nil {
		assert.NotNil(t, provider.middleware.config.TokenProvider)
	}
}

func TestAzureOpenAIProvider_DefaultAPIVersion(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure",
		APIKey: "key",
		ProviderConfig: map[string]interface{}{
			"resource_name": "my-resource",
		},
	}

	provider := NewAzureOpenAIProvider(config)
	assert.Equal(t, azureDefaultAPIVersion, provider.apiVersion)
}

func TestAzureOpenAIProvider_BaseURLGeneration(t *testing.T) {
	t.Run("GenerateFromResourceName", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeAzureOpenAI,
			Name:   "test-azure",
			APIKey: "key",
			ProviderConfig: map[string]interface{}{
				"resource_name": "my-resource",
			},
		}

		provider := NewAzureOpenAIProvider(config)
		assert.Equal(t, "https://my-resource.openai.azure.com", provider.baseURL)
	})

	t.Run("UseProvidedBaseURL", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:    types.ProviderTypeAzureOpenAI,
			Name:    "test-azure",
			APIKey:  "key",
			BaseURL: "https://custom.endpoint.com",
		}

		provider := NewAzureOpenAIProvider(config)
		assert.Equal(t, "https://custom.endpoint.com", provider.baseURL)
	})
}

func TestAzureOpenAIProvider_ModelCache(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeAzureOpenAI,
		Name:   "test-azure",
		APIKey: "key",
	}

	provider := NewAzureOpenAIProvider(config)

	// First call
	models1, err1 := provider.GetModels(context.Background())
	require.NoError(t, err1)

	// Second call should use cache
	models2, err2 := provider.GetModels(context.Background())
	require.NoError(t, err2)

	assert.Equal(t, len(models1), len(models2))
}

func TestAzureOpenAIProvider_Timeout(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeAzureOpenAI,
		Name:    "test-azure",
		APIKey:  "key",
		Timeout: 15 * time.Second,
	}

	provider := NewAzureOpenAIProvider(config)
	assert.NotNil(t, provider.httpClient)
}

// mockTokenProvider is a mock implementation of TokenProvider for testing
type mockTokenProvider struct {
	token string
	err   error
}

func (m *mockTokenProvider) GetToken(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.token, nil
}
