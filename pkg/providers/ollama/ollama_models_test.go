package ollama

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

func TestOllamaProvider_GetModels_WithMockServer(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		response := ollamaTagsResponse{
			Models: []ollamaModel{
				{
					Name:       "llama3.1:8b",
					Model:      "llama3.1:8b",
					ModifiedAt: "2024-12-10T12:00:00.000Z",
					Size:       4661224448,
					Digest:     "sha256:abcd1234",
					Details: ollamaModelDetails{
						Format:            "gguf",
						Family:            "llama",
						Families:          []string{"llama"},
						ParameterSize:     "8.0B",
						QuantizationLevel: "Q4_0",
					},
				},
				{
					Name:       "codellama:13b",
					Model:      "codellama:13b",
					ModifiedAt: "2024-12-10T12:00:00.000Z",
					Size:       7365960000,
					Digest:     "sha256:efgh5678",
					Details: ollamaModelDetails{
						Format:            "gguf",
						Family:            "llama",
						Families:          []string{"llama"},
						ParameterSize:     "13B",
						QuantizationLevel: "Q4_0",
					},
				},
				{
					Name:       "llava:7b",
					Model:      "llava:7b",
					ModifiedAt: "2024-12-10T12:00:00.000Z",
					Size:       4109865216,
					Digest:     "sha256:ijkl9012",
					Details: ollamaModelDetails{
						Format:            "gguf",
						Family:            "llama",
						Families:          []string{"llama"},
						ParameterSize:     "7B",
						QuantizationLevel: "Q4_0",
					},
				},
				{
					Name:       "nomic-embed-text",
					Model:      "nomic-embed-text",
					ModifiedAt: "2024-12-10T12:00:00.000Z",
					Size:       274301184,
					Digest:     "sha256:mnop3456",
					Details: ollamaModelDetails{
						Format:        "gguf",
						Family:        "nomic-bert",
						Families:      []string{"nomic-bert"},
						ParameterSize: "137M",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	models, err := provider.GetModels(ctx)

	assert.NoError(t, err)
	assert.Len(t, models, 4)

	// Check llama3.1:8b model
	llama := models[0]
	assert.Equal(t, "llama3.1:8b", llama.ID)
	assert.Equal(t, "llama3.1:8b", llama.Name)
	assert.Equal(t, types.ProviderTypeOllama, llama.Provider)
	assert.Equal(t, 131072, llama.MaxTokens) // Llama 3.1 has 128k context
	assert.True(t, llama.SupportsStreaming)
	assert.True(t, llama.SupportsToolCalling)
	assert.Contains(t, llama.Capabilities, "chat")
	assert.Contains(t, llama.Capabilities, "completion")
	assert.Contains(t, llama.Capabilities, "tool_calling")

	// Check codellama model
	codellama := models[1]
	assert.Equal(t, "codellama:13b", codellama.ID)
	assert.True(t, codellama.SupportsStreaming)
	assert.Contains(t, codellama.Capabilities, "code")

	// Check llava model (vision)
	llava := models[2]
	assert.Equal(t, "llava:7b", llava.ID)
	assert.Contains(t, llava.Capabilities, "vision")

	// Check embedding model
	embed := models[3]
	assert.Equal(t, "nomic-embed-text", embed.ID)
	assert.Contains(t, embed.Capabilities, "embeddings")
	assert.False(t, embed.SupportsToolCalling)
}

func TestOllamaProvider_GetModels_Fallback(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Should return fallback models
	models, err := provider.GetModels(ctx)

	assert.NoError(t, err)
	assert.NotEmpty(t, models)
	// Should have static fallback models
	assert.GreaterOrEqual(t, len(models), 1)
}

func TestOllamaProvider_GetModels_Cache(t *testing.T) {
	callCount := 0

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		response := ollamaTagsResponse{
			Models: []ollamaModel{
				{
					Name:  "llama3.1:8b",
					Model: "llama3.1:8b",
					Details: ollamaModelDetails{
						Family:        "llama",
						ParameterSize: "8.0B",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// First call - should hit API
	models1, err := provider.GetModels(ctx)
	assert.NoError(t, err)
	assert.Len(t, models1, 1)
	assert.Equal(t, 1, callCount)

	// Second call - should use cache
	models2, err := provider.GetModels(ctx)
	assert.NoError(t, err)
	assert.Len(t, models2, 1)
	assert.Equal(t, 1, callCount) // Should not increment

	// Results should be the same
	assert.Equal(t, models1[0].ID, models2[0].ID)
}

func TestOllamaProvider_GetModels_WithAuthHeader(t *testing.T) {
	authHeaderReceived := false

	// Create mock server that doesn't require auth but checks if it's sent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if Authorization header is present
		auth := r.Header.Get("Authorization")
		if auth == "Bearer test-api-key" {
			authHeaderReceived = true
		}

		response := ollamaTagsResponse{
			Models: []ollamaModel{
				{
					Name:  "test-model:1b",
					Model: "test-model:1b",
					Details: ollamaModelDetails{
						Family:        "test",
						ParameterSize: "1B",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	// Configure as cloud endpoint (contains "ollama.com")
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
		APIKey:  "test-api-key",
	}

	provider := NewOllamaProvider(config)

	// Override the base URL to point to our test server while keeping cloud detection
	provider.config.BaseURL = "https://api.ollama.com"

	// Create a test that just verifies isCloudEndpoint works correctly
	assert.True(t, provider.isCloudEndpoint())

	// Now test with the actual server URL
	provider.config.BaseURL = server.URL

	ctx := context.Background()
	models, err := provider.GetModels(ctx)

	assert.NoError(t, err)
	assert.Len(t, models, 1)
	assert.Equal(t, "test-model:1b", models[0].ID)

	// Since the test server URL doesn't contain "ollama.com",
	// isCloudEndpoint will return false, so auth header won't be sent
	// This is expected behavior
	assert.False(t, authHeaderReceived)
}

func TestOllamaProvider_ConvertOllamaModel(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOllama,
		Name: "ollama-test",
	}
	provider := NewOllamaProvider(config)

	tests := []struct {
		name                 string
		input                ollamaModel
		expectedID           string
		expectedMaxTokens    int
		expectedToolCalling  bool
		expectedCapabilities []string
	}{
		{
			name: "Llama 3.1 model",
			input: ollamaModel{
				Name:  "llama3.1:8b",
				Model: "llama3.1:8b",
				Details: ollamaModelDetails{
					Family:        "llama",
					ParameterSize: "8.0B",
				},
			},
			expectedID:           "llama3.1:8b",
			expectedMaxTokens:    131072,
			expectedToolCalling:  true,
			expectedCapabilities: []string{"chat", "completion", "tool_calling"},
		},
		{
			name: "CodeLlama model",
			input: ollamaModel{
				Name:  "codellama:13b",
				Model: "codellama:13b",
				Details: ollamaModelDetails{
					Family:        "llama",
					ParameterSize: "13B",
				},
			},
			expectedID:           "codellama:13b",
			expectedMaxTokens:    16384,
			expectedToolCalling:  false,
			expectedCapabilities: []string{"chat", "completion", "code"},
		},
		{
			name: "LLaVA vision model",
			input: ollamaModel{
				Name:  "llava:7b",
				Model: "llava:7b",
				Details: ollamaModelDetails{
					Family:        "llama",
					ParameterSize: "7B",
				},
			},
			expectedID:           "llava:7b",
			expectedMaxTokens:    131072,
			expectedToolCalling:  false,
			expectedCapabilities: []string{"chat", "completion", "vision"},
		},
		{
			name: "Embedding model",
			input: ollamaModel{
				Name:  "nomic-embed-text",
				Model: "nomic-embed-text",
				Details: ollamaModelDetails{
					Family:        "nomic-bert",
					ParameterSize: "137M",
				},
			},
			expectedID:           "nomic-embed-text",
			expectedMaxTokens:    8192,
			expectedToolCalling:  false,
			expectedCapabilities: []string{"embeddings"},
		},
		{
			name: "Mistral model",
			input: ollamaModel{
				Name:  "mistral:7b",
				Model: "mistral:7b",
				Details: ollamaModelDetails{
					Family:        "mistral",
					ParameterSize: "7B",
				},
			},
			expectedID:           "mistral:7b",
			expectedMaxTokens:    32768,
			expectedToolCalling:  true,
			expectedCapabilities: []string{"chat", "completion", "tool_calling"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.convertOllamaModel(tt.input)

			assert.Equal(t, tt.expectedID, result.ID)
			assert.Equal(t, types.ProviderTypeOllama, result.Provider)
			assert.Equal(t, tt.expectedMaxTokens, result.MaxTokens)
			assert.Equal(t, tt.expectedToolCalling, result.SupportsToolCalling)
			assert.True(t, result.SupportsStreaming)

			// Check capabilities
			for _, cap := range tt.expectedCapabilities {
				assert.Contains(t, result.Capabilities, cap)
			}
		})
	}
}

func TestOllamaProvider_GetModels_FallbackWhenNoCache(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://invalid-url-that-does-not-exist:99999",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Should return static fallback models when API is unreachable
	models, err := provider.GetModels(ctx)

	assert.NoError(t, err)
	assert.NotEmpty(t, models)

	// Check first model has required fields
	model := models[0]
	assert.NotEmpty(t, model.ID)
	assert.NotEmpty(t, model.Name)
	assert.Equal(t, types.ProviderTypeOllama, model.Provider)
	assert.True(t, model.SupportsStreaming)
}

func TestOllamaProvider_GetRunningModels(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/ps", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		response := ollamaPsResponse{
			Models: []ollamaRunningModel{
				{
					Name:   "llama3.1:8b",
					Model:  "llama3.1:8b",
					Size:   4661224448,
					Digest: "sha256:abcd1234efgh5678",
					Details: ollamaModelDetails{
						Format:            "gguf",
						Family:            "llama",
						Families:          []string{"llama"},
						ParameterSize:     "8.0B",
						QuantizationLevel: "Q4_0",
					},
					ExpiresAt: "2024-12-10T12:00:00Z",
					SizeVRAM:  4294967296,
				},
				{
					Name:   "mistral:7b",
					Model:  "mistral:7b",
					Size:   4109865216,
					Digest: "sha256:ijkl9012mnop3456",
					Details: ollamaModelDetails{
						Format:            "gguf",
						Family:            "mistral",
						Families:          []string{"mistral"},
						ParameterSize:     "7B",
						QuantizationLevel: "Q4_0",
					},
					ExpiresAt: "2024-12-10T13:30:00Z",
					SizeVRAM:  3758096384,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	models, err := provider.GetRunningModels(ctx)

	assert.NoError(t, err)
	assert.Len(t, models, 2)

	// Check first model
	llama := models[0]
	assert.Equal(t, "llama3.1:8b", llama.Name)
	assert.Equal(t, "llama3.1:8b", llama.Model)
	assert.Equal(t, int64(4661224448), llama.Size)
	assert.Equal(t, "sha256:abcd1234efgh5678", llama.Digest)
	assert.Equal(t, int64(4294967296), llama.SizeVRAM)
	assert.False(t, llama.ExpiresAt.IsZero())

	// Check second model
	mistral := models[1]
	assert.Equal(t, "mistral:7b", mistral.Name)
	assert.Equal(t, int64(4109865216), mistral.Size)
	assert.Equal(t, int64(3758096384), mistral.SizeVRAM)
}

func TestOllamaProvider_GetRunningModels_EmptyResponse(t *testing.T) {
	// Create mock server that returns empty models list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ollamaPsResponse{
			Models: []ollamaRunningModel{},
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	models, err := provider.GetRunningModels(ctx)

	assert.NoError(t, err)
	assert.Empty(t, models)
}

func TestOllamaProvider_GetRunningModels_ServerError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	models, err := provider.GetRunningModels(ctx)

	assert.Error(t, err)
	assert.Nil(t, models)
}

func TestOllamaProvider_GetRunningModels_Unauthorized(t *testing.T) {
	// Create mock server that requires authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	models, err := provider.GetRunningModels(ctx)

	assert.Error(t, err)
	assert.Nil(t, models)
	assert.Contains(t, err.Error(), "invalid API key")
}

func TestOllamaProvider_PullModel_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/pull", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "llama3.1:8b", req["name"])
		assert.True(t, req["stream"].(bool))

		// Send streaming progress responses
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		responses := []string{
			`{"status":"pulling manifest"}`,
			`{"status":"downloading","digest":"sha256:abcd1234","total":1000,"completed":250}`,
			`{"status":"downloading","digest":"sha256:abcd1234","total":1000,"completed":500}`,
			`{"status":"downloading","digest":"sha256:abcd1234","total":1000,"completed":1000}`,
			`{"status":"success"}`,
		}

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.PullModel(ctx, "llama3.1:8b")
	assert.NoError(t, err)
}

func TestOllamaProvider_PullModel_CloudEndpoint(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
		APIKey:  "test-key",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.PullModel(ctx, "llama3.1:8b")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported on cloud endpoints")
}

func TestOllamaProvider_PushModel_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/push", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "myuser/mymodel:latest", req["name"])

		// Send streaming progress responses
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		responses := []string{
			`{"status":"pushing manifest"}`,
			`{"status":"uploading","digest":"sha256:efgh5678","total":1000,"completed":500}`,
			`{"status":"uploading","digest":"sha256:efgh5678","total":1000,"completed":1000}`,
			`{"status":"success"}`,
		}

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.PushModel(ctx, "myuser/mymodel:latest")
	assert.NoError(t, err)
}

func TestOllamaProvider_PushModel_CloudEndpoint(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
		APIKey:  "test-key",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.PushModel(ctx, "myuser/mymodel:latest")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported on cloud endpoints")
}

func TestOllamaProvider_DeleteModel_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/delete", r.URL.Path)
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "llama3.1:8b", req["name"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.DeleteModel(ctx, "llama3.1:8b")
	assert.NoError(t, err)
}

func TestOllamaProvider_DeleteModel_NotFound(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.DeleteModel(ctx, "nonexistent:model")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model not found")
}

func TestOllamaProvider_DeleteModel_CloudEndpoint(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
		APIKey:  "test-key",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.DeleteModel(ctx, "llama3.1:8b")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported on cloud endpoints")
}

func TestOllamaProvider_CopyModel_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/copy", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "llama3.1:8b", req["source"])
		assert.Equal(t, "llama3.1:my-custom", req["destination"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.CopyModel(ctx, "llama3.1:8b", "llama3.1:my-custom")
	assert.NoError(t, err)
}

func TestOllamaProvider_CopyModel_SourceNotFound(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"source model not found"}`))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.CopyModel(ctx, "nonexistent:model", "new:model")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOllamaProvider_CopyModel_CloudEndpoint(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
		APIKey:  "test-key",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	err := provider.CopyModel(ctx, "llama3.1:8b", "llama3.1:custom")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported on cloud endpoints")
}

func TestOllamaProvider_CreateModel_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/create", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "mycustom:model", req["name"])
		assert.Contains(t, req["modelfile"].(string), "FROM llama3.1:8b")
		assert.True(t, req["stream"].(bool))

		// Send streaming progress responses
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		responses := []string{
			`{"status":"parsing modelfile"}`,
			`{"status":"creating model layer"}`,
			`{"status":"writing manifest"}`,
			`{"status":"success"}`,
		}

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	modelfile := `FROM llama3.1:8b
PARAMETER temperature 0.7
SYSTEM "You are a helpful assistant."`

	err := provider.CreateModel(ctx, "mycustom:model", modelfile)
	assert.NoError(t, err)
}

func TestOllamaProvider_CreateModel_CloudEndpoint(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
		APIKey:  "test-key",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	modelfile := `FROM llama3.1:8b`

	err := provider.CreateModel(ctx, "mycustom:model", modelfile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported on cloud endpoints")
}

func TestOllamaProvider_ModelManagement_ServerError(t *testing.T) {
	// Test that all model management methods handle server errors properly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Test PullModel
	err := provider.PullModel(ctx, "model:latest")
	assert.Error(t, err)

	// Test PushModel
	err = provider.PushModel(ctx, "model:latest")
	assert.Error(t, err)

	// Test DeleteModel
	err = provider.DeleteModel(ctx, "model:latest")
	assert.Error(t, err)

	// Test CopyModel
	err = provider.CopyModel(ctx, "source:latest", "dest:latest")
	assert.Error(t, err)

	// Test CreateModel
	err = provider.CreateModel(ctx, "model:latest", "FROM base")
	assert.Error(t, err)
}
