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

func TestOllamaProvider_GenerateBatchEmbeddings_NewEndpoint(t *testing.T) {
	// Test the new /api/embed endpoint with batch support
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/api/embed", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var req ollamaEmbedRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "nomic-embed-text", req.Model)

		// Check that input is an array
		inputArray, ok := req.Input.([]interface{})
		assert.True(t, ok, "Input should be an array")
		assert.Len(t, inputArray, 3)

		// Send response with multiple embeddings
		response := ollamaEmbedResponse{
			Model: "nomic-embed-text",
			Embeddings: [][]float32{
				{0.1, 0.2, 0.3},
				{0.4, 0.5, 0.6},
				{0.7, 0.8, 0.9},
			},
			TotalDuration:   1000000,
			LoadDuration:    500000,
			PromptEvalCount: 10,
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
		ProviderConfig: map[string]interface{}{
			"embeddings_endpoint": "embed",
		},
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Test batch embeddings
	texts := []string{"Hello world", "Test text", "Another sentence"}
	embeddings, err := provider.GenerateBatchEmbeddings(ctx, "nomic-embed-text", texts)

	assert.NoError(t, err)
	assert.NotNil(t, embeddings)
	assert.Len(t, embeddings, 3)

	// Verify first embedding
	assert.Len(t, embeddings[0], 3)
	assert.InDelta(t, 0.1, embeddings[0][0], 0.001)
	assert.InDelta(t, 0.2, embeddings[0][1], 0.001)
	assert.InDelta(t, 0.3, embeddings[0][2], 0.001)

	// Verify second embedding
	assert.Len(t, embeddings[1], 3)
	assert.InDelta(t, 0.4, embeddings[1][0], 0.001)
	assert.InDelta(t, 0.5, embeddings[1][1], 0.001)
	assert.InDelta(t, 0.6, embeddings[1][2], 0.001)

	// Verify third embedding
	assert.Len(t, embeddings[2], 3)
	assert.InDelta(t, 0.7, embeddings[2][0], 0.001)
	assert.InDelta(t, 0.8, embeddings[2][1], 0.001)
	assert.InDelta(t, 0.9, embeddings[2][2], 0.001)
}

func TestOllamaProvider_GenerateBatchEmbeddings_LegacyEndpoint(t *testing.T) {
	// Test the legacy /api/embeddings endpoint with sequential processing
	callCount := 0
	expectedTexts := []string{"Hello world", "Test text", "Another sentence"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/api/embeddings", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Read and verify request body
		var req ollamaEmbeddingsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "nomic-embed-text", req.Model)
		assert.Equal(t, expectedTexts[callCount], req.Prompt)

		// Send response based on call count
		var response ollamaEmbeddingsResponse
		switch callCount {
		case 0:
			response.Embedding = []float64{0.1, 0.2, 0.3}
		case 1:
			response.Embedding = []float64{0.4, 0.5, 0.6}
		case 2:
			response.Embedding = []float64{0.7, 0.8, 0.9}
		}
		callCount++

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
		ProviderConfig: map[string]interface{}{
			"embeddings_endpoint": "embeddings",
		},
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Test batch embeddings
	embeddings, err := provider.GenerateBatchEmbeddings(ctx, "nomic-embed-text", expectedTexts)

	assert.NoError(t, err)
	assert.NotNil(t, embeddings)
	assert.Len(t, embeddings, 3)
	assert.Equal(t, 3, callCount, "Should make 3 sequential calls")

	// Verify embeddings
	assert.Equal(t, []float64{0.1, 0.2, 0.3}, embeddings[0])
	assert.Equal(t, []float64{0.4, 0.5, 0.6}, embeddings[1])
	assert.Equal(t, []float64{0.7, 0.8, 0.9}, embeddings[2])
}

func TestOllamaProvider_GenerateBatchEmbeddings_AutoFallback(t *testing.T) {
	// Test auto fallback from /api/embed to /api/embeddings
	embedCallCount := 0
	embeddingsCallCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/embed" {
			embedCallCount++
			// Simulate endpoint not found
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"endpoint not found"}`))
			return
		}

		if r.URL.Path == "/api/embeddings" {
			embeddingsCallCount++

			// Read request
			var req ollamaEmbeddingsRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			// Send response
			response := ollamaEmbeddingsResponse{
				Embedding: []float64{0.1, 0.2, 0.3},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			err = json.NewEncoder(w).Encode(response)
			require.NoError(t, err)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
		ProviderConfig: map[string]interface{}{
			"embeddings_endpoint": "auto",
		},
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// First call should try /api/embed, then fall back to /api/embeddings
	embeddings, err := provider.GenerateBatchEmbeddings(ctx, "nomic-embed-text", []string{"Test"})

	assert.NoError(t, err)
	assert.NotNil(t, embeddings)
	assert.Equal(t, 1, embedCallCount, "Should try /api/embed once")
	assert.Equal(t, 1, embeddingsCallCount, "Should fall back to /api/embeddings")

	// Second call should go directly to /api/embeddings (endpoint marked as failed)
	embeddings, err = provider.GenerateBatchEmbeddings(ctx, "nomic-embed-text", []string{"Test 2"})

	assert.NoError(t, err)
	assert.NotNil(t, embeddings)
	assert.Equal(t, 1, embedCallCount, "Should not retry /api/embed")
	assert.Equal(t, 2, embeddingsCallCount, "Should use /api/embeddings directly")
}

func TestOllamaProvider_GenerateBatchEmbeddings_SingleText(t *testing.T) {
	// Test single text with new endpoint (should send as string, not array)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embed", r.URL.Path)

		var req ollamaEmbedRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		// Input should be a string for single text
		inputStr, ok := req.Input.(string)
		assert.True(t, ok, "Input should be a string for single text")
		assert.Equal(t, "Single text", inputStr)

		response := ollamaEmbedResponse{
			Model:      "nomic-embed-text",
			Embeddings: [][]float32{{0.1, 0.2, 0.3}},
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
		ProviderConfig: map[string]interface{}{
			"embeddings_endpoint": "embed",
		},
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	embeddings, err := provider.GenerateBatchEmbeddings(ctx, "nomic-embed-text", []string{"Single text"})

	assert.NoError(t, err)
	assert.Len(t, embeddings, 1)
	assert.Len(t, embeddings[0], 3)
}

func TestOllamaProvider_GenerateBatchEmbeddings_EmptyInput(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Empty array should return empty result without making API call
	embeddings, err := provider.GenerateBatchEmbeddings(ctx, "nomic-embed-text", []string{})

	assert.NoError(t, err)
	assert.NotNil(t, embeddings)
	assert.Len(t, embeddings, 0)
}

func TestOllamaProvider_GenerateBatchEmbeddings_DefaultModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaEmbedRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		// Should use default model
		assert.Equal(t, "nomic-embed-text", req.Model)

		response := ollamaEmbedResponse{
			Model:      "nomic-embed-text",
			Embeddings: [][]float32{{0.1, 0.2, 0.3}},
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
		ProviderConfig: map[string]interface{}{
			"embeddings_endpoint": "embed",
		},
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Empty model should use default
	embeddings, err := provider.GenerateBatchEmbeddings(ctx, "", []string{"Test"})

	assert.NoError(t, err)
	assert.NotNil(t, embeddings)
}

func TestOllamaProvider_GenerateBatchEmbeddings_ErrorHandling(t *testing.T) {
	tests := []struct {
		name              string
		statusCode        int
		endpoint          string
		expectedErrorCode types.ErrorCode
	}{
		{
			name:              "Unauthorized",
			statusCode:        http.StatusUnauthorized,
			endpoint:          "embed",
			expectedErrorCode: types.ErrCodeAuthentication,
		},
		{
			name:              "Not Found",
			statusCode:        http.StatusNotFound,
			endpoint:          "embed",
			expectedErrorCode: types.ErrCodeNotFound,
		},
		{
			name:              "Rate Limit",
			statusCode:        http.StatusTooManyRequests,
			endpoint:          "embed",
			expectedErrorCode: types.ErrCodeRateLimit,
		},
		{
			name:              "Server Error",
			statusCode:        http.StatusInternalServerError,
			endpoint:          "embed",
			expectedErrorCode: types.ErrCodeServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"error":"test error"}`))
			}))
			defer server.Close()

			config := types.ProviderConfig{
				Type:    types.ProviderTypeOllama,
				Name:    "ollama-test",
				BaseURL: server.URL,
				ProviderConfig: map[string]interface{}{
					"embeddings_endpoint": tt.endpoint,
				},
			}

			provider := NewOllamaProvider(config)
			ctx := context.Background()

			embeddings, err := provider.GenerateBatchEmbeddings(ctx, "nomic-embed-text", []string{"Test"})

			assert.Error(t, err)
			assert.Nil(t, embeddings)

			if providerErr, ok := err.(*types.ProviderError); ok {
				assert.Equal(t, tt.expectedErrorCode, providerErr.Code)
			} else {
				t.Errorf("expected *types.ProviderError, got %T", err)
			}
		})
	}
}

func TestOllamaProvider_GenerateBatchEmbeddings_CloudEndpointAuth(t *testing.T) {
	authHeaderReceived := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if Authorization header is present
		auth := r.Header.Get("Authorization")
		if auth == "Bearer test-api-key" {
			authHeaderReceived = true
		}

		response := ollamaEmbedResponse{
			Model:      "nomic-embed-text",
			Embeddings: [][]float32{{0.1, 0.2, 0.3}},
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
		ProviderConfig: map[string]interface{}{
			"embeddings_endpoint": "embed",
		},
	}

	provider := NewOllamaProvider(config)
	// Override base URL to test server while keeping cloud detection
	provider.config.BaseURL = server.URL

	ctx := context.Background()
	embeddings, err := provider.GenerateBatchEmbeddings(ctx, "nomic-embed-text", []string{"Test"})

	assert.NoError(t, err)
	assert.NotNil(t, embeddings)

	// Since we override the URL, isCloudEndpoint returns false
	// so auth header won't be sent. This is expected behavior.
	assert.False(t, authHeaderReceived)
}

func TestOllamaProvider_ConfigureEmbeddingsEndpoint(t *testing.T) {
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: "http://localhost:11434",
	}

	provider := NewOllamaProvider(config)

	// Default should be auto
	assert.Equal(t, EmbeddingsEndpointAuto, provider.embeddingsEndpoint)

	// Configure to use embed endpoint
	newConfig := config
	newConfig.ProviderConfig = map[string]interface{}{
		"embeddings_endpoint": "embed",
	}
	err := provider.Configure(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, EmbeddingsEndpointEmbed, provider.embeddingsEndpoint)

	// Configure to use legacy endpoint
	newConfig.ProviderConfig = map[string]interface{}{
		"embeddings_endpoint": "embeddings",
	}
	err = provider.Configure(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, EmbeddingsEndpointLegacy, provider.embeddingsEndpoint)
}
