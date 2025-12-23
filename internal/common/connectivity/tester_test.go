// Package connectivity provides shared connectivity testing utilities for AI providers.
package connectivity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTester(t *testing.T) {
	t.Run("DefaultTester", func(t *testing.T) {
		tester := NewTester()
		assert.NotNil(t, tester)
		assert.NotNil(t, tester.cache)
	})

	t.Run("TesterWithCustomCache", func(t *testing.T) {
		customCache := common.NewConnectivityCache(common.ConnectivityCacheConfig{
			TTL:     60 * time.Second,
			Enabled: true,
		})
		tester := NewTesterWithCache(customCache)
		assert.NotNil(t, tester)
		assert.Same(t, customCache, tester.cache)
	})
}

func TestNewModelsEndpointTest(t *testing.T) {
	config := NewModelsEndpointTest(types.ProviderTypeOpenAI, "https://api.openai.com/v1", "test-key")

	assert.Equal(t, types.ProviderTypeOpenAI, config.ProviderType)
	assert.Equal(t, "https://api.openai.com/v1", config.BaseURL)
	assert.Equal(t, EndpointTypeModels, config.EndpointType)
	assert.Equal(t, "test-key", config.AuthToken)
	assert.Equal(t, "api_key", config.AuthMethod)
	assert.NotEmpty(t, config.Headers["User-Agent"])
}

func TestNewChatEndpointTest(t *testing.T) {
	config := NewChatEndpointTest(types.ProviderTypeOpenAI, "https://api.openai.com/v1", "test-key", "gpt-4")

	assert.Equal(t, types.ProviderTypeOpenAI, config.ProviderType)
	assert.Equal(t, "https://api.openai.com/v1", config.BaseURL)
	assert.Equal(t, EndpointTypeChat, config.EndpointType)
	assert.Equal(t, "test-key", config.AuthToken)
	assert.Equal(t, "api_key", config.AuthMethod)
	assert.Equal(t, "gpt-4", config.TestModel)
}

func TestNewOAuthTest(t *testing.T) {
	config := NewOAuthTest(types.ProviderTypeGemini, "https://api.example.com/v1", "oauth-token", "gemini-pro")

	assert.Equal(t, types.ProviderTypeGemini, config.ProviderType)
	assert.Equal(t, "https://api.example.com/v1", config.BaseURL)
	assert.Equal(t, EndpointTypeChat, config.EndpointType)
	assert.Equal(t, "oauth-token", config.AuthToken)
	assert.Equal(t, "oauth", config.AuthMethod)
	assert.Equal(t, "gemini-pro", config.TestModel)
}

func TestNewCustomTest(t *testing.T) {
	config := NewCustomTest(types.ProviderTypeAnthropic, "https://api.anthropic.com/v1", "custom-key", "bearer", EndpointTypeGenerate)

	assert.Equal(t, types.ProviderTypeAnthropic, config.ProviderType)
	assert.Equal(t, "https://api.anthropic.com/v1", config.BaseURL)
	assert.Equal(t, EndpointTypeGenerate, config.EndpointType)
	assert.Equal(t, "custom-key", config.AuthToken)
	assert.Equal(t, "bearer", config.AuthMethod)
}

func TestCheckAuthentication(t *testing.T) {
	t.Run("NoCredentials", func(t *testing.T) {
		err := CheckAuthentication(false, false, false, types.ProviderTypeOpenAI)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no API keys or OAuth credentials configured")
	})

	t.Run("HasAPIKeys", func(t *testing.T) {
		err := CheckAuthentication(true, false, false, types.ProviderTypeOpenAI)
		assert.NoError(t, err)
	})

	t.Run("HasOAuth", func(t *testing.T) {
		err := CheckAuthentication(false, true, false, types.ProviderTypeOpenAI)
		assert.NoError(t, err)
	})

	t.Run("HasContextOAuth", func(t *testing.T) {
		err := CheckAuthentication(false, false, true, types.ProviderTypeOpenAI)
		assert.NoError(t, err)
	})
}

func TestValidateStatusCode(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		err := ValidateStatusCode(types.ProviderTypeOpenAI, http.StatusOK)
		assert.NoError(t, err)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		err := ValidateStatusCode(types.ProviderTypeOpenAI, http.StatusUnauthorized)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid authentication credentials")
	})

	t.Run("Forbidden", func(t *testing.T) {
		err := ValidateStatusCode(types.ProviderTypeOpenAI, http.StatusForbidden)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "do not have access")
	})

	t.Run("OtherError", func(t *testing.T) {
		err := ValidateStatusCode(types.ProviderTypeOpenAI, http.StatusInternalServerError)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code")
	})
}

func TestGetAuthStatus(t *testing.T) {
	t.Run("NoAuth", func(t *testing.T) {
		status := GetAuthStatus(false, false, 0, 0)
		assert.False(t, status["authenticated"].(bool))
		assert.Equal(t, "none", status["method"])
	})

	t.Run("APIKeysOnly", func(t *testing.T) {
		status := GetAuthStatus(true, false, 3, 0)
		assert.True(t, status["authenticated"].(bool))
		assert.Equal(t, "api_key", status["method"])
		assert.Equal(t, 3, status["api_key_count"])
	})

	t.Run("OAuthOnly", func(t *testing.T) {
		status := GetAuthStatus(false, true, 0, 2)
		assert.True(t, status["authenticated"].(bool))
		assert.Equal(t, "oauth", status["method"])
		assert.Equal(t, 2, status["oauth_credential_count"])
	})

	t.Run("BothAPIKeyAndOAuth", func(t *testing.T) {
		status := GetAuthStatus(true, true, 2, 3)
		assert.True(t, status["authenticated"].(bool))
		assert.Equal(t, "api_key", status["method"]) // API key takes precedence
		assert.Equal(t, 2, status["api_key_count"])
	})
}

func TestNewTestClient(t *testing.T) {
	t.Run("DefaultTimeout", func(t *testing.T) {
		client := NewTestClient(0)
		assert.NotNil(t, client)
		assert.Equal(t, 10*time.Second, client.Timeout)
	})

	t.Run("CustomTimeout", func(t *testing.T) {
		client := NewTestClient(5 * time.Second)
		assert.NotNil(t, client)
		assert.Equal(t, 5*time.Second, client.Timeout)
	})
}

func TestTester_TestConnectivity(t *testing.T) {
	t.Run("SuccessfulModelsEndpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/models", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[{"id":"gpt-4"},{"id":"gpt-3.5-turbo"}]}`))
		}))
		defer server.Close()

		tester := NewTester()
		config := NewModelsEndpointTest(types.ProviderTypeOpenAI, server.URL, "test-key")

		err := tester.TestConnectivity(context.Background(), config, false)
		assert.NoError(t, err)
	})

	t.Run("SuccessfulChatEndpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/chat/completions", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"choices":[{"message":{"content":"Hi"}}]}`))
		}))
		defer server.Close()

		tester := NewTester()
		config := NewChatEndpointTest(types.ProviderTypeOpenAI, server.URL, "test-key", "gpt-4")

		err := tester.TestConnectivity(context.Background(), config, false)
		assert.NoError(t, err)
	})

	t.Run("UnauthorizedResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid API key"}`))
		}))
		defer server.Close()

		tester := NewTester()
		config := NewModelsEndpointTest(types.ProviderTypeOpenAI, server.URL, "invalid-key")

		err := tester.TestConnectivity(context.Background(), config, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid authentication credentials")
	})

	t.Run("ForbiddenResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"access denied"}`))
		}))
		defer server.Close()

		tester := NewTester()
		config := NewModelsEndpointTest(types.ProviderTypeOpenAI, server.URL, "test-key")

		err := tester.TestConnectivity(context.Background(), config, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "do not have access")
	})

	t.Run("NetworkError", func(t *testing.T) {
		tester := NewTester()
		config := NewModelsEndpointTest(types.ProviderTypeOpenAI, "http://invalid-url-12345.com/v1", "test-key")

		err := tester.TestConnectivity(context.Background(), config, false)
		assert.Error(t, err)
	})
}

func TestTester_TestConnectivityWithResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[{"id":"gpt-4"}]}`))
	}))
	defer server.Close()

	tester := NewTester()
	config := NewModelsEndpointTest(types.ProviderTypeOpenAI, server.URL, "test-key")

	result := tester.TestConnectivityWithResult(context.Background(), config, false)

	assert.True(t, result.Success)
	assert.NoError(t, result.Error)
	assert.Greater(t, result.Latency, time.Duration(0))
}

func TestTester_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[{"id":"gpt-4"}]}`))
	}))
	defer server.Close()

	tester := NewTester()
	config := NewModelsEndpointTest(types.ProviderTypeOpenAI, server.URL, "test-key")

	// First call should hit the server
	err := tester.TestConnectivity(context.Background(), config, false)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second call should use cache
	err = tester.TestConnectivity(context.Background(), config, false)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "Should not make another request due to caching")

	// Third call with bypass should hit the server again
	err = tester.TestConnectivity(context.Background(), config, true)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "Should make another request when bypassing cache")
}

func TestTester_ClearCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[{"id":"gpt-4"}]}`))
	}))
	defer server.Close()

	tester := NewTester()
	config := NewModelsEndpointTest(types.ProviderTypeOpenAI, server.URL, "test-key")

	// First call
	err := tester.TestConnectivity(context.Background(), config, false)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Clear cache
	tester.ClearCache(types.ProviderTypeOpenAI)

	// Second call should hit the server again
	err = tester.TestConnectivity(context.Background(), config, false)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "Should make another request after clearing cache")
}

func TestTester_ClearAllCache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[{"id":"gpt-4"}]}`))
	}))
	defer server.Close()

	tester := NewTester()
	config := NewModelsEndpointTest(types.ProviderTypeOpenAI, server.URL, "test-key")

	// First call
	err := tester.TestConnectivity(context.Background(), config, false)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Clear all cache
	tester.ClearAllCache()

	// Second call should hit the server again
	err = tester.TestConnectivity(context.Background(), config, false)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "Should make another request after clearing all cache")
}

func TestTester_CustomSuccessValidator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"custom":"response"}`))
	}))
	defer server.Close()

	tester := NewTester()
	config := TestRequestConfig{
		ProviderType: types.ProviderTypeOpenAI,
		BaseURL:      server.URL,
		EndpointType: EndpointTypeModels,
		AuthToken:    "test-key",
		AuthMethod:   "api_key",
		SuccessValidator: func(resp *http.Response, body []byte) error {
			// Custom validation for this test
			if string(body) != `{"custom":"response"}` {
				return assert.AnError
			}
			return nil
		},
	}

	err := tester.TestConnectivity(context.Background(), config, false)
	assert.NoError(t, err)
}

func TestTester_CustomRequestCreator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify custom request was created correctly
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/custom/health", r.URL.Path)
		assert.Equal(t, "custom-value", r.Header.Get("Custom-Header"))

		// Return a valid JSON response for the models endpoint
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[{"id":"model-1"}]}`))
	}))
	defer server.Close()

	tester := NewTester()
	config := TestRequestConfig{
		ProviderType: types.ProviderTypeOpenAI,
		BaseURL:      server.URL,
		EndpointType: EndpointTypeModels,
		AuthToken:    "test-key",
		AuthMethod:   "api_key",
		CreateRequest: func(ctx context.Context, baseURL string, cfg TestRequestConfig) (*http.Request, error) {
			req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/custom/health", nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Custom-Header", "custom-value")
			return req, nil
		},
	}

	err := tester.TestConnectivity(context.Background(), config, false)
	assert.NoError(t, err)
}

func TestTester_OAuthAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify OAuth Bearer token was set
		assert.Equal(t, "Bearer oauth-token-123", r.Header.Get("Authorization"))
		assert.NotContains(t, r.Header.Get("Authorization"), "api-key")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[{"id":"model-1"}]}`))
	}))
	defer server.Close()

	tester := NewTester()
	config := NewOAuthTest(types.ProviderTypeGemini, server.URL, "oauth-token-123", "gemini-pro")

	err := tester.TestConnectivity(context.Background(), config, false)
	assert.NoError(t, err)
}

func TestTester_ContextCancellation(t *testing.T) {
	tester := NewTester()
	config := NewModelsEndpointTest(types.ProviderTypeOpenAI, "http://invalid-url-12345.com/v1", "test-key")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := tester.TestConnectivity(ctx, config, false)
	assert.Error(t, err)
}

func TestPerformSimpleHTTPTest(t *testing.T) {
	t.Run("SuccessfulGET", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/health", r.URL.Path)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewTestClient(0)
		err := PerformSimpleHTTPTest(context.Background(), client, server.URL+"/health", "test-token", "bearer")
		assert.NoError(t, err)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		client := NewTestClient(0)
		err := PerformSimpleHTTPTest(context.Background(), client, server.URL+"/health", "bad-token", "bearer")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status code")
	})

	t.Run("APIKeyAuth", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Should have Authorization header with Bearer token for API key
			assert.Equal(t, "Bearer api-key-123", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewTestClient(0)
		err := PerformSimpleHTTPTest(context.Background(), client, server.URL+"/health", "api-key-123", "api_key")
		assert.NoError(t, err)
	})
}
