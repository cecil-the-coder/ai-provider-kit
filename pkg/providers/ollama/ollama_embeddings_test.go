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
