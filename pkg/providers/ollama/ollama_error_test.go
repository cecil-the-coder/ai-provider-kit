package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
)

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

func TestOllamaProvider_TestConnectivityWithOptions_Success(t *testing.T) {
	// Create mock server that responds successfully
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"0.1.0"}`))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// Test with bypassCache=false (should use cache on second call)
	err1 := provider.TestConnectivityWithOptions(ctx, false)
	assert.NoError(t, err1)

	err2 := provider.TestConnectivityWithOptions(ctx, false)
	assert.NoError(t, err2)

	// Test with bypassCache=true (should always perform fresh check)
	err3 := provider.TestConnectivityWithOptions(ctx, true)
	assert.NoError(t, err3)
}

func TestOllamaProvider_TestConnectivityWithOptions_BypassCache(t *testing.T) {
	// Track the number of requests made
	requestCount := 0

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"0.1.0"}`))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// First call - should make request
	err := provider.TestConnectivityWithOptions(ctx, true)
	assert.NoError(t, err)
	assert.Equal(t, 1, requestCount)

	// Second call with bypassCache=true - should make another request (bypass cache)
	err = provider.TestConnectivityWithOptions(ctx, true)
	assert.NoError(t, err)
	assert.Equal(t, 2, requestCount)
}

func TestOllamaProvider_TestConnectivityWithOptions_Failure(t *testing.T) {
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

	// Test with bypassCache=true
	err := provider.TestConnectivityWithOptions(ctx, true)
	assert.Error(t, err)
}

func TestOllamaProvider_TestConnectivity_DelegatesToWithOptions(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"0.1.0"}`))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	// TestConnectivity should delegate to TestConnectivityWithOptions with bypassCache=false
	err := provider.TestConnectivity(ctx)
	assert.NoError(t, err)
}
