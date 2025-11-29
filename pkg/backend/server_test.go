package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
)

// MockProvider is a mock implementation of types.Provider for testing
type MockProvider struct {
	name          string
	providerType  types.ProviderType
	config        types.ProviderConfig
	authenticated bool
	healthErr     error
	metrics       types.ProviderMetrics
}

func NewMockProvider(name string, providerType types.ProviderType) *MockProvider {
	return &MockProvider{
		name:          name,
		providerType:  providerType,
		authenticated: true,
		config: types.ProviderConfig{
			Name: name,
			Type: providerType,
		},
		metrics: types.ProviderMetrics{
			HealthStatus: types.HealthStatus{
				Healthy:     true,
				LastChecked: time.Now(),
				Message:     "OK",
			},
		},
	}
}

func (m *MockProvider) Name() string                    { return m.name }
func (m *MockProvider) Type() types.ProviderType        { return m.providerType }
func (m *MockProvider) Description() string             { return "Mock provider for testing" }
func (m *MockProvider) GetConfig() types.ProviderConfig { return m.config }
func (m *MockProvider) Configure(config types.ProviderConfig) error {
	m.config = config
	return nil
}
func (m *MockProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return []types.Model{{ID: "test-model", Name: "Test Model"}}, nil
}
func (m *MockProvider) GetDefaultModel() string { return "test-model" }
func (m *MockProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	m.authenticated = true
	return nil
}
func (m *MockProvider) IsAuthenticated() bool                 { return m.authenticated }
func (m *MockProvider) Logout(ctx context.Context) error      { return nil }
func (m *MockProvider) HealthCheck(ctx context.Context) error { return m.healthErr }
func (m *MockProvider) GetMetrics() types.ProviderMetrics     { return m.metrics }
func (m *MockProvider) SupportsToolCalling() bool             { return false }
func (m *MockProvider) SupportsStreaming() bool               { return true }
func (m *MockProvider) SupportsResponsesAPI() bool            { return false }
func (m *MockProvider) GetToolFormat() types.ToolFormat       { return types.ToolFormatOpenAI }
func (m *MockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *MockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	return &MockStream{}, nil
}

// MockStream is a mock implementation of types.ChatCompletionStream
type MockStream struct {
	closed bool
}

func (m *MockStream) Next() (types.ChatCompletionChunk, error) {
	return types.ChatCompletionChunk{Content: "test", Done: true}, nil
}

func (m *MockStream) Close() error {
	m.closed = true
	return nil
}

// TestNewServer tests the NewServer function
func TestNewServer(t *testing.T) {
	t.Run("CreateServerWithValidConfig", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:            "localhost",
				Port:            8080,
				Version:         "1.0.0",
				ReadTimeout:     30 * time.Second,
				WriteTimeout:    30 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			},
			Auth: backendtypes.AuthConfig{
				Enabled: false,
			},
			CORS: backendtypes.CORSConfig{
				Enabled: false,
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		assert.NotNil(t, server)
		assert.Equal(t, config, server.GetConfig())
		assert.Equal(t, providers, server.GetProviders())
		assert.NotNil(t, server.mux)
		assert.NotNil(t, server.extensions)
	})

	t.Run("CreateServerWithMultipleProviders", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:    "localhost",
				Port:    8080,
				Version: "1.0.0",
			},
		}

		providers := map[string]types.Provider{
			"openai":    NewMockProvider("openai", types.ProviderTypeOpenAI),
			"anthropic": NewMockProvider("anthropic", types.ProviderTypeAnthropic),
			"gemini":    NewMockProvider("gemini", types.ProviderTypeGemini),
		}

		server := NewServer(config, providers)

		assert.NotNil(t, server)
		assert.Len(t, server.GetProviders(), 3)
		assert.Contains(t, server.GetProviders(), "openai")
		assert.Contains(t, server.GetProviders(), "anthropic")
		assert.Contains(t, server.GetProviders(), "gemini")
	})

	t.Run("CreateServerWithNoProviders", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:    "localhost",
				Port:    8080,
				Version: "1.0.0",
			},
		}

		providers := map[string]types.Provider{}

		server := NewServer(config, providers)

		assert.NotNil(t, server)
		assert.Empty(t, server.GetProviders())
	})

	t.Run("CreateServerWithExtensions", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:    "localhost",
				Port:    8080,
				Version: "1.0.0",
			},
			Extensions: map[string]backendtypes.ExtensionConfig{
				"test-extension": {
					Enabled: true,
					Config: map[string]interface{}{
						"key": "value",
					},
				},
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		assert.NotNil(t, server)
		assert.NotNil(t, server.extensions)
	})
}

// setupTestServer is a helper function that creates a test server with standard configuration
func setupTestServer() *Server {
	config := backendtypes.BackendConfig{
		Server: backendtypes.ServerConfig{
			Host:    "localhost",
			Port:    8080,
			Version: "1.0.0",
		},
	}

	providers := map[string]types.Provider{
		"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
	}

	return NewServer(config, providers)
}

// routeTestCase defines a test case for route testing
type routeTestCase struct {
	name           string
	method         string
	path           string
	expectedStatus int
	notExpected    bool // if true, assert status != expectedStatus instead of ==
}

// runRouteTests runs a set of route test cases against the server
func runRouteTests(t *testing.T, server *Server, tests []routeTestCase) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)
			if tc.notExpected {
				assert.NotEqual(t, tc.expectedStatus, w.Code)
			} else {
				assert.Equal(t, tc.expectedStatus, w.Code)
			}
		})
	}
}

// TestServer_setupRoutes tests that routes are properly registered
func TestServer_setupRoutes(t *testing.T) {
	server := setupTestServer()
	runRouteTests(t, server, []routeTestCase{
		{"HealthEndpoint", http.MethodGet, "/health", http.StatusOK, false},
		{"StatusEndpoint", http.MethodGet, "/status", http.StatusOK, false},
		{"VersionEndpoint", http.MethodGet, "/version", http.StatusOK, false},
		{"ListProvidersEndpoint", http.MethodGet, "/api/providers", http.StatusOK, false},
		{"GenerateEndpoint", http.MethodPost, "/api/generate", http.StatusNotFound, true},
	})
}

// TestServer_routeProviderRequests tests provider-specific routing
func TestServer_routeProviderRequests(t *testing.T) {
	server := setupTestServer()
	runRouteTests(t, server, []routeTestCase{
		{"ProviderHealthCheckRoute", http.MethodPost, "/api/providers/health", http.StatusNotFound, true},
		{"ProviderTestRoute", http.MethodPost, "/api/providers/test-provider/test", http.StatusNotFound, true},
		{"ProviderGetRoute", http.MethodGet, "/api/providers/test-provider", http.StatusNotFound, true},
		{"ProviderUpdateRoute", http.MethodPut, "/api/providers/test-provider", http.StatusNotFound, true},
		{"ProviderInvalidMethodRoute", http.MethodDelete, "/api/providers/test-provider", http.StatusMethodNotAllowed, false},
	})
}

// TestServer_applyMiddleware tests middleware chain application
func TestServer_applyMiddleware(t *testing.T) {
	t.Run("MiddlewareChainWithoutAuth", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:    "localhost",
				Port:    8080,
				Version: "1.0.0",
			},
			Auth: backendtypes.AuthConfig{
				Enabled: false,
			},
			CORS: backendtypes.CORSConfig{
				Enabled: false,
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		handler := server.applyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should have Request-Id header from middleware
		assert.NotEmpty(t, w.Header().Get("X-Request-Id"))
	})

	t.Run("MiddlewareChainWithAuth", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:    "localhost",
				Port:    8080,
				Version: "1.0.0",
			},
			Auth: backendtypes.AuthConfig{
				Enabled:     true,
				APIPassword: "test-password",
				PublicPaths: []string{"/health", "/version"},
			},
			CORS: backendtypes.CORSConfig{
				Enabled: false,
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		handler := server.applyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))

		// Test without auth - should be unauthorized
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// Test with auth - should succeed
		req = httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer test-password")
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MiddlewareChainWithCORS", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:    "localhost",
				Port:    8080,
				Version: "1.0.0",
			},
			Auth: backendtypes.AuthConfig{
				Enabled: false,
			},
			CORS: backendtypes.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
				AllowedHeaders: []string{"Content-Type", "Authorization"},
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		handler := server.applyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("MiddlewareChainWithAllEnabled", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:    "localhost",
				Port:    8080,
				Version: "1.0.0",
			},
			Auth: backendtypes.AuthConfig{
				Enabled:     true,
				APIPassword: "test-password",
				PublicPaths: []string{"/health"},
			},
			CORS: backendtypes.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		handler := server.applyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Public path should work without auth
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get("X-Request-Id"))
	})
}

// TestServer_Shutdown tests graceful shutdown
func TestServer_Shutdown(t *testing.T) {
	t.Run("ShutdownWithoutStartingServer", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:            "localhost",
				Port:            8080,
				Version:         "1.0.0",
				ShutdownTimeout: 5 * time.Second,
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err := server.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("ShutdownWithContextTimeout", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:            "localhost",
				Port:            8080,
				Version:         "1.0.0",
				ShutdownTimeout: 5 * time.Second,
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		// Create already-cancelled context
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		time.Sleep(2 * time.Millisecond)
		defer cancel()

		err := server.Shutdown(ctx)
		// Should still complete even with cancelled context
		assert.NoError(t, err)
	})
}

// TestServer_GetMethods tests getter methods
func TestServer_GetMethods(t *testing.T) {
	config := backendtypes.BackendConfig{
		Server: backendtypes.ServerConfig{
			Host:    "localhost",
			Port:    8080,
			Version: "1.0.0",
		},
	}

	providers := map[string]types.Provider{
		"openai": NewMockProvider("openai", types.ProviderTypeOpenAI),
	}

	server := NewServer(config, providers)

	t.Run("GetConfig", func(t *testing.T) {
		retrievedConfig := server.GetConfig()
		assert.Equal(t, config, retrievedConfig)
	})

	t.Run("GetProviders", func(t *testing.T) {
		retrievedProviders := server.GetProviders()
		assert.Equal(t, providers, retrievedProviders)
		assert.Len(t, retrievedProviders, 1)
	})

	t.Run("GetExtensionRegistry", func(t *testing.T) {
		registry := server.GetExtensionRegistry()
		assert.NotNil(t, registry)
	})
}

// TestServer_ExtensionManagement tests extension registration
func TestServer_ExtensionManagement(t *testing.T) {
	config := backendtypes.BackendConfig{
		Server: backendtypes.ServerConfig{
			Host:    "localhost",
			Port:    8080,
			Version: "1.0.0",
		},
	}

	providers := map[string]types.Provider{
		"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
	}

	server := NewServer(config, providers)

	t.Run("GetExtensionRegistry", func(t *testing.T) {
		registry := server.GetExtensionRegistry()
		assert.NotNil(t, registry)
	})
}

// TestServer_ListenAndServeWithGracefulShutdown tests the graceful shutdown helper
func TestServer_ListenAndServeWithGracefulShutdown(t *testing.T) {
	t.Run("ShutdownSignalReceived", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:            "localhost",
				Port:            18888, // Use high port to avoid conflicts
				Version:         "1.0.0",
				ShutdownTimeout: 1 * time.Second,
				ReadTimeout:     5 * time.Second,
				WriteTimeout:    5 * time.Second,
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		shutdownSignal := make(chan struct{})

		errChan := make(chan error, 1)
		go func() {
			errChan <- server.ListenAndServeWithGracefulShutdown(shutdownSignal)
		}()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Send shutdown signal
		close(shutdownSignal)

		// Wait for shutdown to complete
		err := <-errChan
		assert.NoError(t, err)
	})

	t.Run("ShutdownWithDefaultTimeout", func(t *testing.T) {
		config := backendtypes.BackendConfig{
			Server: backendtypes.ServerConfig{
				Host:            "localhost",
				Port:            18889, // Use different port
				Version:         "1.0.0",
				ShutdownTimeout: 0, // Should use default 30s
				ReadTimeout:     5 * time.Second,
				WriteTimeout:    5 * time.Second,
			},
		}

		providers := map[string]types.Provider{
			"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
		}

		server := NewServer(config, providers)

		shutdownSignal := make(chan struct{})

		errChan := make(chan error, 1)
		go func() {
			errChan <- server.ListenAndServeWithGracefulShutdown(shutdownSignal)
		}()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Send shutdown signal
		close(shutdownSignal)

		// Wait for shutdown to complete
		err := <-errChan
		assert.NoError(t, err)
	})
}

// TestServer_TransportConfiguration tests that transport is properly configured
func TestServer_TransportConfiguration(t *testing.T) {
	t.Run("SharedTransportSettings", func(t *testing.T) {
		assert.Equal(t, 100, SharedTransport.MaxIdleConns)
		assert.Equal(t, 10, SharedTransport.MaxIdleConnsPerHost)
		assert.Equal(t, 90*time.Second, SharedTransport.IdleConnTimeout)
	})

	t.Run("SharedClientSettings", func(t *testing.T) {
		assert.NotNil(t, SharedClient)
		assert.Equal(t, SharedTransport, SharedClient.Transport)
		assert.Equal(t, 30*time.Second, SharedClient.Timeout)
	})
}

// TestServer_Integration tests end-to-end server functionality
func TestServer_Integration(t *testing.T) {
	config := backendtypes.BackendConfig{
		Server: backendtypes.ServerConfig{
			Host:            "localhost",
			Port:            8080,
			Version:         "1.0.0",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		Auth: backendtypes.AuthConfig{
			Enabled:     false,
			PublicPaths: []string{"/health", "/version"},
		},
		CORS: backendtypes.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type"},
		},
	}

	providers := map[string]types.Provider{
		"openai":    NewMockProvider("openai", types.ProviderTypeOpenAI),
		"anthropic": NewMockProvider("anthropic", types.ProviderTypeAnthropic),
	}

	server := NewServer(config, providers)

	// Apply middleware to server mux
	handler := server.applyMiddleware(server.mux)

	t.Run("HealthCheckWorks", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("VersionWorks", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/version", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ListProvidersWorks", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("CORSHeadersPresent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/providers", nil)
		req.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("RequestIDHeaderPresent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.NotEmpty(t, w.Header().Get("X-Request-Id"))
	})
}

// TestServer_ConcurrentRequests tests handling of concurrent requests
func TestServer_ConcurrentRequests(t *testing.T) {
	config := backendtypes.BackendConfig{
		Server: backendtypes.ServerConfig{
			Host:    "localhost",
			Port:    8080,
			Version: "1.0.0",
		},
	}

	providers := map[string]types.Provider{
		"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
	}

	server := NewServer(config, providers)
	handler := server.applyMiddleware(server.mux)

	// Make 100 concurrent requests
	concurrentRequests := 100
	done := make(chan bool, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < concurrentRequests; i++ {
		<-done
	}
}

// TestServer_ProviderRouting tests provider-specific endpoint routing
func TestServer_ProviderRouting(t *testing.T) {
	config := backendtypes.BackendConfig{
		Server: backendtypes.ServerConfig{
			Host:    "localhost",
			Port:    8080,
			Version: "1.0.0",
		},
	}

	providers := map[string]types.Provider{
		"provider1": NewMockProvider("provider1", types.ProviderTypeOpenAI),
	}

	server := NewServer(config, providers)

	t.Run("ProviderHealthWithName", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/providers/provider1/health", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusNotFound, w.Code)
	})

	t.Run("ProviderTestWithName", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/providers/provider1/test", nil)
		w := httptest.NewRecorder()
		server.mux.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusNotFound, w.Code)
	})
}

// BenchmarkServer_NewServer benchmarks server creation
func BenchmarkServer_NewServer(b *testing.B) {
	config := backendtypes.BackendConfig{
		Server: backendtypes.ServerConfig{
			Host:    "localhost",
			Port:    8080,
			Version: "1.0.0",
		},
	}

	providers := map[string]types.Provider{
		"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewServer(config, providers)
	}
}

// BenchmarkServer_HealthEndpoint benchmarks health endpoint
func BenchmarkServer_HealthEndpoint(b *testing.B) {
	config := backendtypes.BackendConfig{
		Server: backendtypes.ServerConfig{
			Host:    "localhost",
			Port:    8080,
			Version: "1.0.0",
		},
	}

	providers := map[string]types.Provider{
		"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
	}

	server := NewServer(config, providers)
	handler := server.applyMiddleware(server.mux)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

// BenchmarkServer_MiddlewareChain benchmarks middleware execution
func BenchmarkServer_MiddlewareChain(b *testing.B) {
	config := backendtypes.BackendConfig{
		Server: backendtypes.ServerConfig{
			Host:    "localhost",
			Port:    8080,
			Version: "1.0.0",
		},
		Auth: backendtypes.AuthConfig{
			Enabled: false,
		},
		CORS: backendtypes.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
		},
	}

	providers := map[string]types.Provider{
		"test-provider": NewMockProvider("test-provider", types.ProviderTypeOpenAI),
	}

	server := NewServer(config, providers)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler := server.applyMiddleware(testHandler)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
