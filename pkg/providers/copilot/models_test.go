// Package copilot provides models API tests for GitHub Copilot AI provider.
package copilot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetchModelsFromAPI tests fetching models from Copilot API
func TestFetchModelsFromAPI(t *testing.T) {
	t.Run("successful models fetch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/models", r.URL.Path)

			// Verify authentication
			assert.Equal(t, "Bearer test_token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, CopilotIntegrationID, r.Header.Get("copilot-integration-id"))

			// Return models response
			w.Header().Set("Content-Type", "application/json")
			response := ModelsResponse{
				Object: "list",
				Data: []ModelData{
					{
						ID:                 "gpt-4o",
						Object:             "model",
						Created:            time.Now().Unix(),
						OwnedBy:            "openai",
						Name:               "GPT-4o",
						Vendor:             "openai",
						Version:            "1.0.0",
						Preview:            false,
						ModelPickerEnabled: true,
						Capabilities: ModelCapabilities{
							Object: "model.capabilities",
							Family: "gpt-4",
							Limits: ModelLimits{
								MaxContextWindowTokens: 128000,
								MaxOutputTokens:        4096,
								MaxPromptTokens:        123904,
							},
							Supports: ModelSupports{
								ToolCalls:         true,
								ParallelToolCalls: true,
							},
							Tokenizer: "cl100k_base",
							Type:      "chat",
						},
					},
					{
						ID:                 "gpt-4o-mini",
						Object:             "model",
						Created:            time.Now().Unix(),
						OwnedBy:            "openai",
						Name:               "GPT-4o mini",
						Vendor:             "openai",
						Version:            "1.0.0",
						Preview:            false,
						ModelPickerEnabled: true,
						Capabilities: ModelCapabilities{
							Object: "model.capabilities",
							Family: "gpt-4",
							Limits: ModelLimits{
								MaxContextWindowTokens: 128000,
								MaxOutputTokens:        16384,
								MaxPromptTokens:        111360,
							},
							Supports: ModelSupports{
								ToolCalls:         true,
								ParallelToolCalls: true,
							},
							Tokenizer: "cl100k_base",
							Type:      "chat",
						},
					},
					{
						ID:                 "o1-preview",
						Object:             "model",
						Created:            time.Now().Unix(),
						OwnedBy:            "openai",
						Name:               "O1 Preview",
						Vendor:             "openai",
						Version:            "1.0.0",
						Preview:            true,
						ModelPickerEnabled: true,
						Capabilities: ModelCapabilities{
							Object: "model.capabilities",
							Family: "o1",
							Limits: ModelLimits{
								MaxContextWindowTokens: 128000,
								MaxOutputTokens:        32768,
								MaxPromptTokens:        95232,
							},
							Supports: ModelSupports{
								ToolCalls:         false,
								ParallelToolCalls: false,
							},
							Tokenizer: "o1",
							Type:      "chat",
						},
					},
					{
						// This model should be filtered out
						ID:                 "disabled-model",
						Object:             "model",
						Created:            time.Now().Unix(),
						OwnedBy:            "openai",
						Name:               "Disabled Model",
						Vendor:             "openai",
						Version:            "1.0.0",
						Preview:            false,
						ModelPickerEnabled: false, // Disabled
						Capabilities: ModelCapabilities{
							Object: "model.capabilities",
							Family: "gpt-4",
							Limits: ModelLimits{
								MaxContextWindowTokens: 128000,
								MaxOutputTokens:        4096,
								MaxPromptTokens:        123904,
							},
							Supports: ModelSupports{
								ToolCalls:         false,
								ParallelToolCalls: false,
							},
							Tokenizer: "cl100k_base",
							Type:      "chat",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		models, err := provider.fetchModelsFromAPI(context.Background())
		require.NoError(t, err)

		// Should have 3 models (disabled one filtered out)
		assert.Equal(t, 3, len(models))

		// Check first model
		gpt4o := models[0]
		assert.Equal(t, "gpt-4o", gpt4o.ID)
		assert.Equal(t, "GPT-4o", gpt4o.Name)
		assert.Equal(t, types.ProviderTypeCopilot, gpt4o.Provider)
		assert.Equal(t, 4096, gpt4o.MaxTokens)
		assert.True(t, gpt4o.SupportsStreaming)
		assert.True(t, gpt4o.SupportsToolCalling)
		assert.Contains(t, gpt4o.Description, "openai")

		// Check second model
		gpt4oMini := models[1]
		assert.Equal(t, "gpt-4o-mini", gpt4oMini.ID)
		assert.Equal(t, "GPT-4o mini", gpt4oMini.Name)
		assert.Equal(t, 16384, gpt4oMini.MaxTokens)
		assert.True(t, gpt4oMini.SupportsToolCalling)

		// Check third model (no tool support)
		o1Preview := models[2]
		assert.Equal(t, "o1-preview", o1Preview.ID)
		assert.Equal(t, "O1 Preview", o1Preview.Name)
		assert.False(t, o1Preview.SupportsToolCalling)
	})

	t.Run("models fetch with authentication error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid token",
					"type":    "authentication_error",
				},
			})
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "invalid_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		_, err := provider.fetchModelsFromAPI(context.Background())
		assert.Error(t, err)
	})

	t.Run("models fetch with server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		_, err := provider.fetchModelsFromAPI(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("models fetch with invalid response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		_, err := provider.fetchModelsFromAPI(context.Background())
		assert.Error(t, err)
	})

	t.Run("models fetch with empty response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ModelsResponse{
				Object: "list",
				Data:   []ModelData{},
			})
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		models, err := provider.fetchModelsFromAPI(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 0, len(models))
	})
}

// TestGetModels tests the GetModels method with caching
func TestGetModels(t *testing.T) {
	t.Run("returns fallback when not authenticated", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})

		models, err := provider.GetModels(context.Background())
		require.NoError(t, err)

		// Should return static fallback models
		assert.NotEmpty(t, models)
		assert.GreaterOrEqual(t, len(models), 2)

		// Check for expected fallback models
		modelMap := make(map[string]types.Model)
		for _, m := range models {
			modelMap[m.ID] = m
		}

		assert.Contains(t, modelMap, "gpt-4o")
		assert.Contains(t, modelMap, "gpt-4o-mini")
	})

	t.Run("caches models successfully", func(t *testing.T) {
		requestCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			w.Header().Set("Content-Type", "application/json")
			response := ModelsResponse{
				Object: "list",
				Data: []ModelData{
					{
						ID:                 "gpt-4o",
						Object:             "model",
						Created:            time.Now().Unix(),
						OwnedBy:            "openai",
						Name:               "GPT-4o",
						Vendor:             "openai",
						Version:            "1.0.0",
						Preview:            false,
						ModelPickerEnabled: true,
						Capabilities: ModelCapabilities{
							Object: "model.capabilities",
							Family: "gpt-4",
							Limits: ModelLimits{
								MaxContextWindowTokens: 128000,
								MaxOutputTokens:        4096,
								MaxPromptTokens:        123904,
							},
							Supports: ModelSupports{
								ToolCalls: true,
							},
							Tokenizer: "cl100k_base",
							Type:      "chat",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		// First call - should hit API
		models1, err := provider.GetModels(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, requestCount)
		assert.Equal(t, 1, len(models1))

		// Second call - should use cache (no additional request)
		models2, err := provider.GetModels(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, requestCount, "should not make additional request")
		assert.Equal(t, models1[0].ID, models2[0].ID)
	})

	t.Run("returns fallback on API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server error"))
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: server.URL,
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		// Should return fallback instead of error
		models, err := provider.GetModels(context.Background())
		require.NoError(t, err)
		assert.NotEmpty(t, models)

		// Verify fallback models
		modelMap := make(map[string]types.Model)
		for _, m := range models {
			modelMap[m.ID] = m
		}
		assert.Contains(t, modelMap, "gpt-4o")
	})
}

// TestGetStaticFallback tests the static fallback model list
func TestGetStaticFallback(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	models := provider.getStaticFallback()

	require.NotEmpty(t, models)
	assert.GreaterOrEqual(t, len(models), 2)

	// Verify structure
	for _, model := range models {
		assert.NotEmpty(t, model.ID)
		assert.NotEmpty(t, model.Name)
		assert.Equal(t, types.ProviderTypeCopilot, model.Provider)
		assert.Greater(t, model.MaxTokens, 0)
		assert.True(t, model.SupportsStreaming)
		assert.True(t, model.SupportsToolCalling)
		assert.NotEmpty(t, model.Description)
	}
}

// TestModelsResponseSerialization tests that models response serializes correctly
func TestModelsResponseSerialization(t *testing.T) {
	response := ModelsResponse{
		Object: "list",
		Data: []ModelData{
			{
				ID:                 "test-model",
				Object:             "model",
				Created:            1234567890,
				OwnedBy:            "test-vendor",
				Name:               "Test Model",
				Vendor:             "test-vendor",
				Version:            "1.0.0",
				Preview:            false,
				ModelPickerEnabled: true,
				Capabilities: ModelCapabilities{
					Object: "model.capabilities",
					Family: "test-family",
					Limits: ModelLimits{
						MaxContextWindowTokens: 100000,
						MaxOutputTokens:        5000,
						MaxPromptTokens:        95000,
					},
					Supports: ModelSupports{
						ToolCalls:         true,
						ParallelToolCalls: true,
					},
					Tokenizer: "test-tokenizer",
					Type:      "chat",
				},
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(response)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var unmarshaled ModelsResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, response.Object, unmarshaled.Object)
	assert.Equal(t, 1, len(unmarshaled.Data))
	assert.Equal(t, "test-model", unmarshaled.Data[0].ID)
	assert.Equal(t, "Test Model", unmarshaled.Data[0].Name)
	assert.True(t, unmarshaled.Data[0].Capabilities.Supports.ToolCalls)
}

// TestModelDataToInternalModel tests conversion from ModelData to internal Model
func TestModelDataToInternalModel(t *testing.T) {
	modelData := ModelData{
		ID:                 "gpt-4o",
		Object:             "model",
		Created:            time.Now().Unix(),
		OwnedBy:            "openai",
		Name:               "GPT-4o",
		Vendor:             "openai",
		Version:            "1.0.0",
		Preview:            false,
		ModelPickerEnabled: true,
		Capabilities: ModelCapabilities{
			Object: "model.capabilities",
			Family: "gpt-4",
			Limits: ModelLimits{
				MaxContextWindowTokens: 128000,
				MaxOutputTokens:        4096,
				MaxPromptTokens:        123904,
			},
			Supports: ModelSupports{
				ToolCalls:         true,
				ParallelToolCalls: true,
			},
			Tokenizer: "cl100k_base",
			Type:      "chat",
		},
	}

	// Simulate the conversion that happens in fetchModelsFromAPI
	maxTokens := modelData.Capabilities.Limits.MaxOutputTokens
	supportsToolCalling := modelData.Capabilities.Supports.ToolCalls

	assert.Equal(t, 4096, maxTokens)
	assert.True(t, supportsToolCalling)
}

// TestGetModels_AccountTypes tests models endpoint for different account types
func TestGetModels_AccountTypes(t *testing.T) {
	accountTypes := []struct {
		accountType string
		baseURL     string
	}{
		{AccountTypeIndividual, CopilotBaseURL},
		{AccountTypeBusiness, CopilotBusinessBaseURL},
		{AccountTypeEnterprise, CopilotEnterpriseBaseURL},
	}

	for _, at := range accountTypes {
		t.Run(at.accountType+" account", func(t *testing.T) {
			receivedURL := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedURL = r.URL.String()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(ModelsResponse{
					Object: "list",
					Data: []ModelData{
						{
							ID:                 "gpt-4o",
							Object:             "model",
							Created:            time.Now().Unix(),
							OwnedBy:            "openai",
							Name:               "GPT-4o",
							Vendor:             "openai",
							Version:            "1.0.0",
							Preview:            false,
							ModelPickerEnabled: true,
							Capabilities: ModelCapabilities{
								Object: "model.capabilities",
								Family: "gpt-4",
								Limits: ModelLimits{
									MaxOutputTokens: 4096,
								},
								Supports: ModelSupports{
									ToolCalls: true,
								},
								Type: "chat",
							},
						},
					},
				})
			}))
			defer server.Close()

			provider := NewCopilotProvider(types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"account_type": at.accountType,
					"base_url":     server.URL,
				},
			})
			provider.tokenMutex.Lock()
			provider.copilotToken = "test_token"
			provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
			provider.tokenMutex.Unlock()

			models, err := provider.fetchModelsFromAPI(context.Background())
			require.NoError(t, err)
			assert.Equal(t, "/models", receivedURL)
			assert.Equal(t, 1, len(models))
			assert.Equal(t, "gpt-4o", models[0].ID)
		})
	}
}

// TestGetDefaultModel tests getting the default model
func TestGetDefaultModel(t *testing.T) {
	tests := []struct {
		name              string
		config            types.ProviderConfig
		expectedModel     string
	}{
		{
			name: "default model when not specified",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
			},
			expectedModel: copilotDefaultModel,
		},
		{
			name: "custom model in config",
			config: types.ProviderConfig{
				Type:         types.ProviderTypeCopilot,
				DefaultModel: "gpt-4o-mini",
			},
			expectedModel: "gpt-4o-mini",
		},
		{
			name: "custom model in provider config",
			config: types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"model": "o1-preview",
				},
			},
			expectedModel: "o1-preview",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewCopilotProvider(tt.config)
			assert.Equal(t, tt.expectedModel, provider.GetDefaultModel())
		})
	}
}

// TestModelCache tests the model cache functionality
func TestModelCache(t *testing.T) {
	t.Run("cache stores and retrieves models", func(t *testing.T) {
		cache := NewModelCache(1 * time.Hour)

		expectedModels := []types.Model{
			{ID: "model1", Name: "Model 1"},
			{ID: "model2", Name: "Model 2"},
		}

		models, err := cache.GetModels(
			func() ([]types.Model, error) {
				return expectedModels, nil
			},
			func() []types.Model {
				return []types.Model{{ID: "fallback", Name: "Fallback"}}
			},
		)

		require.NoError(t, err)
		assert.Equal(t, expectedModels, models)
	})

	t.Run("cache uses fallback on error", func(t *testing.T) {
		cache := NewModelCache(1 * time.Hour)

		fallbackModels := []types.Model{
			{ID: "fallback1", Name: "Fallback 1"},
		}

		models, err := cache.GetModels(
			func() ([]types.Model, error) {
				return nil, io.EOF
			},
			func() []types.Model {
				return fallbackModels
			},
		)

		require.NoError(t, err)
		assert.Equal(t, fallbackModels, models)
	})

	t.Run("cache expires after TTL", func(t *testing.T) {
		cache := NewModelCache(10 * time.Millisecond)

		firstModels := []types.Model{{ID: "first", Name: "First"}}
		secondModels := []types.Model{{ID: "second", Name: "Second"}}

		callCount := 0

		// First call
		models, _ := cache.GetModels(
			func() ([]types.Model, error) {
				callCount++
				if callCount == 1 {
					return firstModels, nil
				}
				return secondModels, nil
			},
			func() []types.Model {
				return nil
			},
		)

		assert.Equal(t, firstModels, models)
		assert.Equal(t, 1, callCount)

		// Wait for cache to expire
		time.Sleep(50 * time.Millisecond)

		// Second call - should fetch again
		models, _ = cache.GetModels(
			func() ([]types.Model, error) {
				callCount++
				if callCount == 1 {
					return firstModels, nil
				}
				return secondModels, nil
			},
			func() []types.Model {
				return nil
			},
		)

		assert.Equal(t, secondModels, models)
		assert.Equal(t, 2, callCount)
	})

	t.Run("cache is thread-safe", func(t *testing.T) {
		cache := NewModelCache(1 * time.Hour)

		results := make(chan []types.Model, 10)

		// Concurrent reads
		for i := 0; i < 10; i++ {
			go func() {
				models, _ := cache.GetModels(
					func() ([]types.Model, error) {
						return []types.Model{{ID: "shared", Name: "Shared"}}, nil
					},
					func() []types.Model {
						return nil
					},
				)
				results <- models
			}()
		}

		// Collect results
		for i := 0; i < 10; i++ {
			models := <-results
			assert.Equal(t, 1, len(models))
			assert.Equal(t, "shared", models[0].ID)
		}
	})
}

// TestGetModelsAPIHeaders tests that correct headers are sent to models endpoint
func TestGetModelsAPIHeaders(t *testing.T) {
	headersReceived := make(map[string]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture headers
		headersReceived["Authorization"] = r.Header.Get("Authorization")
		headersReceived["Content-Type"] = r.Header.Get("Content-Type")
		headersReceived["copilot-integration-id"] = r.Header.Get("copilot-integration-id")
		headersReceived["editor-version"] = r.Header.Get("editor-version")
		headersReceived["editor-plugin-version"] = r.Header.Get("editor-plugin-version")
		headersReceived["user-agent"] = r.Header.Get("user-agent")
		headersReceived["openai-intent"] = r.Header.Get("openai-intent")
		headersReceived["x-github-api-version"] = r.Header.Get("x-github-api-version")
		headersReceived["x-vscode-user-agent-library-version"] = r.Header.Get("x-vscode-user-agent-library-version")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ModelsResponse{
			Object: "list",
			Data:   []ModelData{},
		})
	}))
	defer server.Close()

	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
		ProviderConfig: map[string]interface{}{
			"base_url": server.URL,
		},
	})
	provider.tokenMutex.Lock()
	provider.copilotToken = "test_token"
	provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
	provider.tokenMutex.Unlock()

	_, err := provider.fetchModelsFromAPI(context.Background())
	require.NoError(t, err)

	// Verify all required headers were sent
	assert.Equal(t, "Bearer test_token", headersReceived["Authorization"])
	assert.Equal(t, "application/json", headersReceived["Content-Type"])
	assert.Equal(t, CopilotIntegrationID, headersReceived["copilot-integration-id"])
	assert.Contains(t, headersReceived["editor-version"], "vscode/")
	assert.Equal(t, EditorPluginVersion, headersReceived["editor-plugin-version"])
	assert.Equal(t, UserAgent, headersReceived["user-agent"])
	assert.Equal(t, OpenAIIntent, headersReceived["openai-intent"])
	assert.Equal(t, GitHubAPIVersion, headersReceived["x-github-api-version"])
	assert.Equal(t, VSCodeUserAgentLibrary, headersReceived["x-vscode-user-agent-library-version"])
}
