package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: isChatModel, getDisplayName, getMaxTokens, supportsToolCalling, and getDescription
// tests have been removed as these functions are now handled by the
// centralized ModelMetadataRegistry in pkg/providers/common/model_registry.go
// The isChatModel filter was removed entirely to support OpenAI-compatible providers like Groq

// TestEnrichModels tests the enrichModels helper function
func TestEnrichModels(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	models := []types.Model{
		{
			ID:       "gpt-4o",
			Provider: types.ProviderTypeOpenAI,
		},
		{
			ID:       "gpt-3.5-turbo",
			Provider: types.ProviderTypeOpenAI,
		},
	}

	enriched := provider.enrichModels(models)

	require.Len(t, enriched, 2)

	// Check GPT-4o
	assert.Equal(t, "gpt-4o", enriched[0].ID)
	assert.Equal(t, "GPT-4o", enriched[0].Name)
	assert.Equal(t, types.ProviderTypeOpenAI, enriched[0].Provider)
	assert.Equal(t, 128000, enriched[0].MaxTokens)
	assert.True(t, enriched[0].SupportsStreaming)
	assert.True(t, enriched[0].SupportsToolCalling)
	assert.Contains(t, enriched[0].Description, "flagship")

	// Check GPT-3.5-turbo
	assert.Equal(t, "gpt-3.5-turbo", enriched[1].ID)
	assert.Equal(t, "GPT-3.5 Turbo", enriched[1].Name)
	assert.Equal(t, 4096, enriched[1].MaxTokens)
	assert.True(t, enriched[1].SupportsStreaming)
	assert.True(t, enriched[1].SupportsToolCalling)
}

// TestGetAuthStatus tests the GetAuthStatus method
func TestGetAuthStatus(t *testing.T) {
	t.Run("Authenticated", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		status := provider.GetAuthStatus()

		assert.NotNil(t, status)
		assert.True(t, status["authenticated"].(bool))
		assert.Contains(t, status, "has_api_keys")
		assert.True(t, status["has_api_keys"].(bool))
		assert.Equal(t, 1, status["api_keys_configured"])
	})

	t.Run("NotAuthenticated", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		status := provider.GetAuthStatus()

		assert.NotNil(t, status)
		assert.False(t, status["authenticated"].(bool))
	})
}

// TestFetchModelsFromAPI tests the fetchModelsFromAPI method
func TestFetchModelsFromAPI(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the request
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "Bearer sk-test-key", r.Header.Get("Authorization"))

			// Return mock models response
			response := OpenAIModelsResponse{
				Object: "list",
				Data: []OpenAIModel{
					{
						ID:      "gpt-4o",
						Object:  "model",
						Created: 1234567890,
						OwnedBy: "openai",
					},
					{
						ID:      "gpt-3.5-turbo",
						Object:  "model",
						Created: 1234567890,
						OwnedBy: "openai",
					},
					{
						ID:      "text-davinci-003",
						Object:  "model",
						Created: 1234567890,
						OwnedBy: "openai",
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		models, err := provider.fetchModelsFromAPI(context.Background())
		require.NoError(t, err)

		// Should include all models (no filtering - supports OpenAI-compatible providers)
		assert.Len(t, models, 3)
		assert.Equal(t, "gpt-4o", models[0].ID)
		assert.Equal(t, "gpt-3.5-turbo", models[1].ID)
		assert.Equal(t, "text-davinci-003", models[2].ID)
	})

	t.Run("NoAPIKey", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		models, err := provider.fetchModelsFromAPI(context.Background())
		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "no OpenAI API key configured")
	})

	t.Run("HTTPError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-invalid-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		models, err := provider.fetchModelsFromAPI(context.Background())
		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "failed to fetch models")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		models, err := provider.fetchModelsFromAPI(context.Background())
		assert.Error(t, err)
		assert.Nil(t, models)
		assert.Contains(t, err.Error(), "failed to parse models response")
	})
}

// TestConvertToOpenAIToolChoice tests the convertToOpenAIToolChoice helper function
func TestConvertToOpenAIToolChoice(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		result := convertToOpenAIToolChoice(nil)
		assert.Nil(t, result)
	})

	t.Run("Auto", func(t *testing.T) {
		toolChoice := &types.ToolChoice{
			Mode: types.ToolChoiceAuto,
		}
		result := convertToOpenAIToolChoice(toolChoice)
		assert.Equal(t, "auto", result)
	})

	t.Run("Required", func(t *testing.T) {
		toolChoice := &types.ToolChoice{
			Mode: types.ToolChoiceRequired,
		}
		result := convertToOpenAIToolChoice(toolChoice)
		assert.Equal(t, "required", result)
	})

	t.Run("None", func(t *testing.T) {
		toolChoice := &types.ToolChoice{
			Mode: types.ToolChoiceNone,
		}
		result := convertToOpenAIToolChoice(toolChoice)
		assert.Equal(t, "none", result)
	})

	t.Run("Specific", func(t *testing.T) {
		toolChoice := &types.ToolChoice{
			Mode:         types.ToolChoiceSpecific,
			FunctionName: "get_weather",
		}
		result := convertToOpenAIToolChoice(toolChoice)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "function", resultMap["type"])

		functionMap, ok := resultMap["function"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "get_weather", functionMap["name"])
	})

	t.Run("UnknownMode", func(t *testing.T) {
		toolChoice := &types.ToolChoice{
			Mode: "unknown",
		}
		result := convertToOpenAIToolChoice(toolChoice)
		assert.Equal(t, "auto", result) // Should default to auto
	})
}
