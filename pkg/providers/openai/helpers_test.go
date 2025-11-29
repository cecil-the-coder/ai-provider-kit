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

// TestIsChatModel tests the isChatModel helper function
func TestIsChatModel(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tests := []struct {
		name     string
		modelID  string
		expected bool
	}{
		{"GPT-4", "gpt-4", true},
		{"GPT-4-0613", "gpt-4-0613", true},
		{"GPT-4-32k", "gpt-4-32k", true},
		{"GPT-4-32k-0613", "gpt-4-32k-0613", true},
		{"GPT-4-Turbo", "gpt-4-turbo", true},
		{"GPT-4-Turbo-Preview", "gpt-4-turbo-preview", true},
		{"GPT-3.5-Turbo", "gpt-3.5-turbo", true},
		{"GPT-3.5-Turbo-16k", "gpt-3.5-turbo-16k", true},
		{"GPT-3.5-Turbo-0613", "gpt-3.5-turbo-0613", true},
		{"GPT-4o", "gpt-4o", true},
		{"GPT-4o-Mini", "gpt-4o-mini", true},
		{"GPT-4o-2024-05-13", "gpt-4o-2024-05-13", true},
		{"Non-Chat Model", "text-davinci-003", false},
		{"Random Model", "random-model", false},
		{"Empty String", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.isChatModel(tt.modelID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetDisplayName tests the getDisplayName helper function
func TestGetDisplayName(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tests := []struct {
		name     string
		modelID  string
		expected string
	}{
		{"GPT-4o", "gpt-4o", "GPT-4o"},
		{"GPT-4o Mini", "gpt-4o-mini", "GPT-4o Mini"},
		{"GPT-4", "gpt-4", "GPT-4"},
		{"GPT-4 Turbo", "gpt-4-turbo", "GPT-4 Turbo"},
		{"GPT-3.5 Turbo", "gpt-3.5-turbo", "GPT-3.5 Turbo"},
		{"GPT-4 with date", "gpt-4-0613", "GPT-4"},
		{"GPT-3.5 with date", "gpt-3.5-turbo-0613", "GPT-3.5 Turbo"},
		{"Unknown Model", "unknown-model", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.getDisplayName(tt.modelID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetMaxTokens tests the getMaxTokens helper function
func TestGetMaxTokens(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tests := []struct {
		name     string
		modelID  string
		expected int
	}{
		{"GPT-4o", "gpt-4o", 128000},
		{"GPT-4o Mini", "gpt-4o-mini", 128000},
		{"GPT-4", "gpt-4", 8192},
		{"GPT-4-32k", "gpt-4-32k", 32768},
		{"GPT-4 Turbo", "gpt-4-turbo", 128000},
		{"GPT-3.5 Turbo", "gpt-3.5-turbo", 4096},
		{"GPT-3.5 Turbo 16k", "gpt-3.5-turbo-16k", 16384},
		{"GPT-4 with date", "gpt-4-0613", 8192},
		{"GPT-3.5 with date", "gpt-3.5-turbo-0613", 4096},
		{"Unknown Model", "unknown-model", 4096}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.getMaxTokens(tt.modelID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSupportsToolCalling tests the supportsToolCalling helper function
func TestSupportsToolCalling(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tests := []struct {
		name     string
		modelID  string
		expected bool
	}{
		{"GPT-4", "gpt-4", true},
		{"GPT-4o", "gpt-4o", true},
		{"GPT-4-Turbo", "gpt-4-turbo", true},
		{"GPT-3.5-Turbo", "gpt-3.5-turbo", true},
		{"GPT-3.5-Turbo-16k", "gpt-3.5-turbo-16k", true},
		{"Non-tool model", "text-davinci-003", false},
		{"Unknown", "unknown-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.supportsToolCalling(tt.modelID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetDescription tests the getDescription helper function
func TestGetDescription(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	tests := []struct {
		name     string
		modelID  string
		contains string
	}{
		{"GPT-4o", "gpt-4o", "flagship"},
		{"GPT-4o Mini", "gpt-4o-mini", "efficient"},
		{"GPT-4", "gpt-4", "previous flagship"},
		{"GPT-4 Turbo", "gpt-4-turbo", "balanced"},
		{"GPT-3.5 Turbo", "gpt-3.5-turbo", "fast"},
		{"GPT-4 with date", "gpt-4-0613", "previous flagship"},
		{"Unknown Model", "unknown-model", "OpenAI language model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.getDescription(tt.modelID)
			assert.Contains(t, result, tt.contains)
		})
	}
}

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

		// Should only include chat models
		assert.Len(t, models, 2)
		assert.Equal(t, "gpt-4o", models[0].ID)
		assert.Equal(t, "gpt-3.5-turbo", models[1].ID)
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
