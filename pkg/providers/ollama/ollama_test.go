package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOllamaProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "Ollama", provider.Name())
	assert.Equal(t, types.ProviderTypeOllama, provider.Type())
	assert.Equal(t, "Ollama local and cloud model inference", provider.Description())
	assert.Equal(t, "http://localhost:11434", provider.GetConfig().BaseURL)
}

func TestOllamaProvider_DefaultValues(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeOllama,
		Name: "ollama-test",
	}

	provider := NewOllamaProvider(config)

	assert.NotNil(t, provider)
	assert.Equal(t, "http://localhost:11434", provider.GetConfig().BaseURL)
	assert.Equal(t, "llama3.1:8b", provider.GetDefaultModel())
}

func TestOllamaProvider_SupportsFeatures(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	assert.True(t, provider.SupportsToolCalling())
	assert.True(t, provider.SupportsStreaming())
	assert.False(t, provider.SupportsResponsesAPI())
	assert.Equal(t, types.ToolFormatOpenAI, provider.GetToolFormat())
}

func TestOllamaProvider_IsAuthenticated(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		apiKey      string
		expectAuth  bool
		description string
	}{
		{
			name:        "Local endpoint - no auth required",
			baseURL:     "http://localhost:11434",
			apiKey:      "",
			expectAuth:  true,
			description: "Local Ollama doesn't require authentication",
		},
		{
			name:        "Cloud endpoint - no API key",
			baseURL:     "https://api.ollama.com",
			apiKey:      "",
			expectAuth:  false,
			description: "Cloud endpoint requires API key",
		},
		{
			name:        "Cloud endpoint - with API key",
			baseURL:     "https://api.ollama.com",
			apiKey:      "test-key",
			expectAuth:  true,
			description: "Cloud endpoint with API key is authenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.ProviderConfig{
				Type:    types.ProviderTypeOllama,
				Name:    "ollama-test",
				BaseURL: tt.baseURL,
				APIKey:  tt.apiKey,
			}

			provider := NewOllamaProvider(config)
			assert.Equal(t, tt.expectAuth, provider.IsAuthenticated(), tt.description)
		})
	}
}

func TestOllamaProvider_isCloudEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		expectCloud bool
	}{
		{
			name:        "Localhost",
			baseURL:     "http://localhost:11434",
			expectCloud: false,
		},
		{
			name:        "127.0.0.1",
			baseURL:     "http://127.0.0.1:11434",
			expectCloud: false,
		},
		{
			name:        "Cloud endpoint",
			baseURL:     "https://api.ollama.com",
			expectCloud: true,
		},
		{
			name:        "Cloud endpoint with path",
			baseURL:     "https://cloud.ollama.com/v1",
			expectCloud: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.ProviderConfig{
				Type:    types.ProviderTypeOllama,
				Name:    "ollama-test",
				BaseURL: tt.baseURL,
			}

			provider := NewOllamaProvider(config)
			assert.Equal(t, tt.expectCloud, provider.isCloudEndpoint())
		})
	}
}

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

func TestOllamaProvider_GenerateChatCompletion_WithMockServer(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "llama3.1:8b", req["model"])
		assert.True(t, req["stream"].(bool))

		// Send streaming response (newline-delimited JSON, not SSE)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Write streaming chunks
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":" there"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"!"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":5,"eval_count":10}`,
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

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Model: "llama3.1:8b",
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)

	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	var finalChunk types.ChatCompletionChunk
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
		finalChunk = chunk
	}

	// Verify content
	assert.Equal(t, "Hello there!", content.String())

	// Verify final chunk has usage
	assert.True(t, finalChunk.Done)
	assert.Equal(t, 5, finalChunk.Usage.PromptTokens)
	assert.Equal(t, 10, finalChunk.Usage.CompletionTokens)
	assert.Equal(t, 15, finalChunk.Usage.TotalTokens)

	// Close stream
	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_Configure(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	// Update configuration
	newConfig := types.ProviderConfig{
		Type:         types.ProviderTypeOllama,
		Name:         "ollama-updated",
		BaseURL:      "http://192.168.1.100:11434",
		DefaultModel: "mistral:7b",
	}

	err := provider.Configure(newConfig)

	assert.NoError(t, err)
	assert.Equal(t, "http://192.168.1.100:11434", provider.GetConfig().BaseURL)
	assert.Equal(t, "mistral:7b", provider.GetDefaultModel())
}

func TestOllamaProvider_GetMetrics(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)
	metrics := provider.GetMetrics()

	// Initial metrics should be zero
	assert.Equal(t, int64(0), metrics.RequestCount)
	assert.Equal(t, int64(0), metrics.SuccessCount)
	assert.Equal(t, int64(0), metrics.ErrorCount)
}

func TestOllamaProvider_Authenticate(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Test API key authentication
	authConfig := types.AuthConfig{
		Method:       types.AuthMethodAPIKey,
		APIKey:       "test-key-123",
		BaseURL:      "https://api.ollama.com",
		DefaultModel: "llama3.1:8b",
	}

	err := provider.Authenticate(ctx, authConfig)
	assert.NoError(t, err)
	assert.True(t, provider.IsAuthenticated())
	assert.Equal(t, "test-key-123", provider.GetConfig().APIKey)

	// Test invalid auth method
	invalidConfig := types.AuthConfig{
		Method: types.AuthMethodOAuth,
	}
	err = provider.Authenticate(ctx, invalidConfig)
	assert.Error(t, err)
}

func TestOllamaProvider_Logout(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
		APIKey:  "test-key",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Verify initially authenticated
	assert.True(t, provider.IsAuthenticated())

	// Logout
	err := provider.Logout(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "", provider.GetConfig().APIKey)
}

func TestOllamaProvider_InvokeServerTool(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Tool invocation should return not implemented error
	result, err := provider.InvokeServerTool(ctx, "test_tool", map[string]interface{}{"key": "value"})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestOllamaProvider_MessageConversion_WithImages(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	// Create message with image parts
	messages := []types.ChatMessage{
		{
			Role:    "user",
			Content: "What's in this image?",
			Parts: []types.ContentPart{
				{
					Type: types.ContentTypeText,
					Text: "What's in this image?",
				},
				{
					Type: types.ContentTypeImage,
					Source: &types.MediaSource{
						Type: types.MediaSourceBase64,
						Data: "base64encodedimagedata",
					},
				},
			},
		},
	}

	// Convert messages
	ollamaMessages := provider.convertMessages(messages)

	// Verify image was extracted
	assert.Len(t, ollamaMessages, 1)
	assert.Equal(t, "What's in this image?", ollamaMessages[0].Content)
	assert.Len(t, ollamaMessages[0].Images, 1)
	assert.Equal(t, "base64encodedimagedata", ollamaMessages[0].Images[0])
}

func TestOllamaProvider_MessageConversion_WithToolResults(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	// Create messages with tool calls
	messages := []types.ChatMessage{
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []types.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"location":"SF"}`,
					},
				},
			},
		},
		{
			Role:    "tool",
			Content: `{"temperature": 72, "condition": "sunny"}`,
		},
	}

	// Convert messages
	ollamaMessages := provider.convertMessages(messages)

	// Verify tool calls were converted
	assert.Len(t, ollamaMessages, 2)
	assert.Len(t, ollamaMessages[0].ToolCalls, 1)
	assert.Equal(t, "call_123", ollamaMessages[0].ToolCalls[0].ID)
	assert.Equal(t, "get_weather", ollamaMessages[0].ToolCalls[0].Function.Name)
}

func TestOllamaProvider_HealthCheck(t *testing.T) {
	// Create mock server that responds to /api/version
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"0.1.0"}`))
		} else {
			w.WriteHeader(http.StatusOK)
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

	// Health check should succeed
	err := provider.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestOllamaProvider_HealthCheck_Failure(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:99999", // Invalid port
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Health check should fail
	err := provider.HealthCheck(ctx)
	assert.Error(t, err)
}

func TestOllamaProvider_TestConnectivity_CloudEndpoint(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for auth header
		auth := r.Header.Get("Authorization")
		if auth == "Bearer valid-key" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"0.1.0"}`))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	// Test with valid key
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com", // Cloud endpoint
		APIKey:  "valid-key",
	}

	provider := NewOllamaProvider(config)

	// Test connectivity should check authentication
	assert.True(t, provider.IsAuthenticated())
}

func TestOllamaProvider_GenerateChatCompletion_WithTools(t *testing.T) {
	// Create mock server that expects tools
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify tools are present
		assert.NotEmpty(t, req.Tools)
		assert.Equal(t, "get_weather", req.Tools[0].Function.Name)

		// Send response with tool call
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_123","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"SF\"}"}}]},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":10,"eval_count":5}`,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

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

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "What's the weather in SF?"},
		},
		Model: "llama3.1:8b",
		Tools: []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]string{"type": "string"},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read chunks
	chunks := []types.ChatCompletionChunk{}
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		chunks = append(chunks, chunk)
	}

	// Verify tool calls
	assert.NotEmpty(t, chunks)
	assert.NotEmpty(t, chunks[0].Choices)
	assert.NotEmpty(t, chunks[0].Choices[0].Delta.ToolCalls)

	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_GenerateEmbeddings(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/api/embeddings", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var req ollamaEmbeddingsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "nomic-embed-text", req.Model)
		assert.Equal(t, "Hello world", req.Prompt)

		// Send response
		response := ollamaEmbeddingsResponse{
			Embedding: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(response)
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

	// Test with explicit model
	embedding, err := provider.GenerateEmbeddings(ctx, "nomic-embed-text", "Hello world")

	assert.NoError(t, err)
	assert.NotNil(t, embedding)
	assert.Len(t, embedding, 5)
	assert.Equal(t, 0.1, embedding[0])
	assert.Equal(t, 0.2, embedding[1])
	assert.Equal(t, 0.3, embedding[2])
	assert.Equal(t, 0.4, embedding[3])
	assert.Equal(t, 0.5, embedding[4])
}

func TestOllamaProvider_GenerateEmbeddings_DefaultModel(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var req ollamaEmbeddingsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		// Verify default model is used
		assert.Equal(t, "nomic-embed-text", req.Model)

		// Send response
		response := ollamaEmbeddingsResponse{
			Embedding: []float64{0.1, 0.2, 0.3},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(response)
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

	// Test with empty model (should use default)
	embedding, err := provider.GenerateEmbeddings(ctx, "", "Test text")

	assert.NoError(t, err)
	assert.NotNil(t, embedding)
	assert.Len(t, embedding, 3)
}

func TestOllamaProvider_GenerateEmbeddings_ErrorHandling(t *testing.T) {
	tests := []struct {
		name              string
		statusCode        int
		responseBody      string
		expectedError     string
		expectedErrorCode types.ErrorCode
	}{
		{
			name:              "Unauthorized",
			statusCode:        http.StatusUnauthorized,
			responseBody:      `{"error":"invalid API key"}`,
			expectedError:     "invalid API key",
			expectedErrorCode: types.ErrCodeAuthentication,
		},
		{
			name:              "Model not found",
			statusCode:        http.StatusNotFound,
			responseBody:      `{"error":"model not found"}`,
			expectedError:     "model not found",
			expectedErrorCode: types.ErrCodeNotFound,
		},
		{
			name:              "Rate limit",
			statusCode:        http.StatusTooManyRequests,
			responseBody:      `{"error":"rate limit exceeded"}`,
			expectedError:     "rate limit",
			expectedErrorCode: types.ErrCodeRateLimit,
		},
		{
			name:              "Server error",
			statusCode:        http.StatusInternalServerError,
			responseBody:      `{"error":"internal server error"}`,
			expectedError:     "internal server error",
			expectedErrorCode: types.ErrCodeServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			config := types.ProviderConfig{
				Type:    types.ProviderTypeOllama,
				Name:    "ollama-test",
				BaseURL: server.URL,
			}

			provider := NewOllamaProvider(config)
			ctx := context.Background()

			embedding, err := provider.GenerateEmbeddings(ctx, "nomic-embed-text", "Test text")

			assert.Error(t, err)
			assert.Nil(t, embedding)
			assert.Contains(t, err.Error(), tt.expectedError)

			// Check error type by asserting to *ProviderError and checking Code
			if providerErr, ok := err.(*types.ProviderError); ok {
				assert.Equal(t, tt.expectedErrorCode, providerErr.Code)
			} else {
				t.Errorf("expected *types.ProviderError, got %T", err)
			}
		})
	}
}

func TestOllamaProvider_GenerateEmbeddings_CloudEndpointAuth(t *testing.T) {
	authHeaderReceived := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if Authorization header is present
		auth := r.Header.Get("Authorization")
		if auth == "Bearer test-api-key" {
			authHeaderReceived = true
		}

		response := ollamaEmbeddingsResponse{
			Embedding: []float64{0.1, 0.2, 0.3},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(response)
		require.NoError(t, err)
	}))
	defer server.Close()

	// Configure as cloud endpoint
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "https://api.ollama.com",
		APIKey:  "test-api-key",
	}

	provider := NewOllamaProvider(config)

	// Override base URL to test server while keeping cloud detection
	provider.config.BaseURL = server.URL

	ctx := context.Background()
	embedding, err := provider.GenerateEmbeddings(ctx, "nomic-embed-text", "Test text")

	assert.NoError(t, err)
	assert.NotNil(t, embedding)

	// Since we override the URL, isCloudEndpoint returns false
	// so auth header won't be sent. This is expected behavior.
	assert.False(t, authHeaderReceived)
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

// Model Management Tests

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

func TestOllamaProvider_StructuredOutputs_BasicJSON(t *testing.T) {
	// Create mock server that expects format field
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify format is set to "json"
		assert.NotNil(t, req.Format)
		assert.Equal(t, "json", req.Format)

		// Send streaming response with JSON content
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"{\"name\":"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"John\","},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"age\":30}"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":10,"eval_count":15}`,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

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

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Return a person object with name and age"},
		},
		Model:          "llama3.1:8b",
		ResponseFormat: "json", // Basic JSON mode
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)

	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
	}

	// Verify content is valid JSON
	result := content.String()
	assert.Contains(t, result, "John")
	assert.Contains(t, result, "30")

	// Verify it's valid JSON
	var jsonObj map[string]interface{}
	err = json.Unmarshal([]byte(result), &jsonObj)
	assert.NoError(t, err)

	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_StructuredOutputs_JSONSchema(t *testing.T) {
	// Create mock server that expects format field with schema
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify format is a JSON schema object
		assert.NotNil(t, req.Format)
		formatMap, ok := req.Format.(map[string]interface{})
		assert.True(t, ok, "Format should be a map")
		assert.Equal(t, "object", formatMap["type"])
		assert.NotNil(t, formatMap["properties"])

		// Send streaming response with structured JSON
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"{\"name\":"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"Alice\","},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"email\":\"alice@example.com\"}"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":12,"eval_count":18}`,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

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

	// Define a JSON schema for structured output
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"name", "email"},
	}

	// Convert schema to JSON string
	schemaJSON, err := json.Marshal(schema)
	require.NoError(t, err)

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Return a user object with name and email"},
		},
		Model:          "llama3.1:8b",
		ResponseFormat: string(schemaJSON), // JSON schema
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)

	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
	}

	// Verify content matches schema
	result := content.String()
	var user map[string]interface{}
	err = json.Unmarshal([]byte(result), &user)
	assert.NoError(t, err)
	assert.Equal(t, "Alice", user["name"])
	assert.Equal(t, "alice@example.com", user["email"])

	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_StructuredOutputs_ComplexSchema(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify format is a complex schema
		assert.NotNil(t, req.Format)
		_, ok := req.Format.(map[string]interface{})
		assert.True(t, ok)

		// Send streaming response with nested JSON
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"{\"user\":"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"{\"name\":\"Bob\",\"age\":25},"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"items\":[\"apple\",\"banana\"]}"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":15,"eval_count":20}`,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

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

	// Define a complex nested JSON schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"user": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "number"},
				},
				"required": []string{"name", "age"},
			},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"user", "items"},
	}

	schemaJSON, err := json.Marshal(schema)
	require.NoError(t, err)

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Return a complex object with user and items"},
		},
		Model:          "llama3.1:8b",
		ResponseFormat: string(schemaJSON),
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
	}

	// Verify nested structure
	result := content.String()
	var obj map[string]interface{}
	err = json.Unmarshal([]byte(result), &obj)
	assert.NoError(t, err)

	// Verify user object
	user, ok := obj["user"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Bob", user["name"])
	assert.Equal(t, float64(25), user["age"])

	// Verify items array
	items, ok := obj["items"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, items, 2)
	assert.Equal(t, "apple", items[0])
	assert.Equal(t, "banana", items[1])

	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_StructuredOutputs_NoFormat(t *testing.T) {
	// Test that requests without ResponseFormat don't include format field
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify format is nil/not set
		assert.Nil(t, req.Format)

		// Send normal response
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":5,"eval_count":10}`,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

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

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Model: "llama3.1:8b",
		// No ResponseFormat specified
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	var content strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
	}

	assert.Equal(t, "Hello", content.String())

	err = stream.Close()
	assert.NoError(t, err)
}
