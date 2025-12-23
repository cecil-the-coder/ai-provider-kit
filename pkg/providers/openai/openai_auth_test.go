package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
)

// TestOpenAIProvider_Authenticate tests the Authenticate method
func TestOpenAIProvider_Authenticate(t *testing.T) {
	t.Run("APIKeyAuthentication", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method:       types.AuthMethodAPIKey,
			APIKey:       "sk-new-key",
			BaseURL:      "https://custom.openai.com/v1",
			DefaultModel: "gpt-4",
		}

		err := provider.Authenticate(context.Background(), authConfig)

		assert.NoError(t, err)
		assert.True(t, provider.IsAuthenticated())
		assert.Equal(t, "https://custom.openai.com/v1", provider.baseURL)
		assert.Equal(t, "gpt-4", provider.GetDefaultModel())
	})

	t.Run("UnsupportedAuthMethod", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodOAuth,
		}

		err := provider.Authenticate(context.Background(), authConfig)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OpenAI only supports API key authentication")
	})

	t.Run("BearerTokenAuthentication", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodBearerToken,
			APIKey: "bearer-token",
		}

		err := provider.Authenticate(context.Background(), authConfig)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OpenAI only supports API key authentication")
	})
}

// TestOpenAIProvider_IsAuthenticated tests the IsAuthenticated method
func TestOpenAIProvider_IsAuthenticated(t *testing.T) {
	t.Run("Authenticated", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		assert.True(t, provider.IsAuthenticated())
	})

	t.Run("NotAuthenticated", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
		}
		provider := NewOpenAIProvider(config)

		assert.False(t, provider.IsAuthenticated())
	})
}

// TestOpenAIProvider_Logout tests the Logout method
func TestOpenAIProvider_Logout(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenAI,
		APIKey: "sk-test-key",
	}
	provider := NewOpenAIProvider(config)

	assert.True(t, provider.IsAuthenticated())

	err := provider.Logout(context.Background())

	assert.NoError(t, err)
	assert.False(t, provider.IsAuthenticated())
}

// TestOpenAIProvider_TestConnectivity tests the TestConnectivity method
func TestOpenAIProvider_TestConnectivity(t *testing.T) {
	t.Run("SuccessfulConnectivity", func(t *testing.T) {
		// Create a mock HTTP server that returns a valid models response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the request is for the models endpoint
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "GET", r.Method)

			// Verify authorization header
			authHeader := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer sk-test-key", authHeader)

			// Return a valid models response
			response := map[string]interface{}{
				"object": "list",
				"data": []interface{}{
					map[string]interface{}{
						"id":       "gpt-4",
						"object":   "model",
						"created":  1687882411,
						"owned_by": "openai",
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

		err := provider.TestConnectivity(context.Background())
		assert.NoError(t, err)
	})

	t.Run("NoAPIKeys", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
			// No API key configured
		}
		provider := NewOpenAIProvider(config)

		err := provider.TestConnectivity(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no API keys configured")
	})

	t.Run("InvalidAPIKey", func(t *testing.T) {
		// Create a mock server that returns unauthorized
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid API key",
					"type":    "invalid_request_error",
					"code":    "invalid_api_key",
				},
			})
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-invalid-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		err := provider.TestConnectivity(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connectivity test failed")
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("ForbiddenAccess", func(t *testing.T) {
		// Create a mock server that returns forbidden
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "API key does not have access to models endpoint",
					"type":    "invalid_request_error",
				},
			})
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-forbidden-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		err := provider.TestConnectivity(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not have access to models endpoint")
	})

	t.Run("InvalidResponse", func(t *testing.T) {
		// Create a mock server that returns invalid JSON
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("invalid json response"))
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		err := provider.TestConnectivity(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid response from models endpoint")
	})

	t.Run("OpenAICompatibleProvider", func(t *testing.T) {
		// Test that OpenAI-compatible providers (Groq, xAI, etc.) work even if they
		// don't return the standard "object": "list" field
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				// No "object" field - simulating OpenAI-compatible provider
				"data": []interface{}{
					map[string]interface{}{
						"id":       "llama-3.1-70b",
						"object":   "model",
						"created":  1687882411,
						"owned_by": "groq",
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

		err := provider.TestConnectivity(context.Background())
		assert.NoError(t, err) // Should succeed without Object field validation
	})

	t.Run("NetworkError", func(t *testing.T) {
		// Use an invalid URL to simulate network error
		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: "http://invalid-url-that-does-not-exist.local",
		}
		provider := NewOpenAIProvider(config)

		err := provider.TestConnectivity(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connectivity test failed")
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		// Create a mock server that delays response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second) // Longer than our context timeout
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"object": "list",
				"data":   []interface{}{},
			})
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			APIKey:  "sk-test-key",
			BaseURL: server.URL,
		}
		provider := NewOpenAIProvider(config)

		// Create a context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := provider.TestConnectivity(ctx)
		assert.Error(t, err)
		// The error should be related to context cancellation or timeout
		errStr := err.Error()
		assert.True(t, strings.Contains(errStr, "context") ||
			strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "canceled") ||
			strings.Contains(errStr, "deadline exceeded") ||
			strings.Contains(errStr, "Client.Timeout exceeded") ||
			strings.Contains(errStr, "network"),
			"Expected context/timeout/network error, got: %s", errStr)
	})
}
