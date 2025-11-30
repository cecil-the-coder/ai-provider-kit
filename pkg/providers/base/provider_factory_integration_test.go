package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// =============================================================================
// Mock Providers and HTTP Servers for Testing
// =============================================================================

// mockHTTPServer creates a test HTTP server that can be configured to respond with different status codes
type mockHTTPServer struct {
	server     *httptest.Server
	response   string
	statusCode int
	headers    map[string]string
	delay      time.Duration
	mutex      sync.RWMutex
}

func newMockHTTPServer() *mockHTTPServer {
	s := &mockHTTPServer{
		response:   `{"success": true}`,
		statusCode: 200,
		headers:    make(map[string]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.server = httptest.NewServer(mux)
	return s
}

func (m *mockHTTPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Add delay if configured
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	// Set headers
	for key, value := range m.headers {
		w.Header().Set(key, value)
	}

	w.WriteHeader(m.statusCode)
	w.Write([]byte(m.response))
}

func (m *mockHTTPServer) setResponse(response string, statusCode int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.response = response
	m.statusCode = statusCode
}

func (m *mockHTTPServer) setDelay(delay time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.delay = delay
}

func (m *mockHTTPServer) setHeaders(headers map[string]string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.headers = headers
}

func (m *mockHTTPServer) url() string {
	return m.server.URL
}

func (m *mockHTTPServer) close() {
	m.server.Close()
}

// mockProvider implements a basic provider interface for testing
type mockProvider struct {
	providerType types.ProviderType
	shouldFail   bool
	failReason   string
	failPhase    types.TestPhase
	testable     bool
	oauth        bool
	tokenInfo    *types.TokenInfo
	refreshToken bool
	models       []types.Model
	modelsError  error
	healthError  error
}

func (m *mockProvider) Name() string {
	return string(m.providerType) + " Mock"
}

func (m *mockProvider) Type() types.ProviderType {
	return m.providerType
}

func (m *mockProvider) Description() string {
	return "Mock provider for testing"
}

func (m *mockProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	if m.modelsError != nil {
		return nil, m.modelsError
	}
	return m.models, nil
}

func (m *mockProvider) GetDefaultModel() string {
	if len(m.models) > 0 {
		return m.models[0].ID
	}
	return ""
}

func (m *mockProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return errors.New("not implemented in mock")
}

func (m *mockProvider) IsAuthenticated() bool {
	return true
}

func (m *mockProvider) Logout(ctx context.Context) error {
	return nil
}

func (m *mockProvider) Configure(config types.ProviderConfig) error {
	return nil
}

func (m *mockProvider) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{}
}

func (m *mockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *mockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *mockProvider) SupportsToolCalling() bool {
	return false
}

func (m *mockProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (m *mockProvider) SupportsStreaming() bool {
	return false
}

func (m *mockProvider) SupportsResponsesAPI() bool {
	return false
}

func (m *mockProvider) GetMetrics() types.ProviderMetrics {
	return types.ProviderMetrics{}
}

func (m *mockProvider) HealthCheck(ctx context.Context) error {
	if m.healthError != nil {
		return m.healthError
	}
	return nil
}

func (m *mockProvider) TestConnectivity(ctx context.Context) error {
	if !m.testable {
		return fmt.Errorf("connectivity testing not supported")
	}

	if m.shouldFail && m.failPhase == types.TestPhaseConnectivity {
		return errors.New(m.failReason)
	}

	return nil
}


// mockOAuthProvider is a separate struct for OAuth providers to avoid interface conflicts
type mockOAuthProvider struct {
	*mockProvider
}

func (m *mockOAuthProvider) ValidateToken(ctx context.Context) (*types.TokenInfo, error) {
	if m.mockProvider.shouldFail && m.mockProvider.failPhase == types.TestPhaseAuthentication {
		return nil, errors.New(m.mockProvider.failReason)
	}

	if m.mockProvider.tokenInfo != nil {
		return m.mockProvider.tokenInfo, nil
	}

	return &types.TokenInfo{
		Valid:     true,
		ExpiresAt: time.Now().Add(time.Hour),
		Scope:     []string{"read", "write"},
		UserInfo: map[string]interface{}{
			"id":    "test-user",
			"email": "test@example.com",
		},
	}, nil
}

func (m *mockOAuthProvider) RefreshToken(ctx context.Context) error {
	if !m.mockProvider.refreshToken {
		return errors.New("refresh not supported")
	}

	if m.mockProvider.shouldFail && strings.Contains(m.mockProvider.failReason, "refresh") {
		return errors.New(m.mockProvider.failReason)
	}

	// Simulate successful refresh
	if m.mockProvider.tokenInfo != nil {
		m.mockProvider.tokenInfo.ExpiresAt = time.Now().Add(time.Hour)
	}

	return nil
}

func (m *mockOAuthProvider) GetAuthURL(redirectURI string, state string) string {
	return fmt.Sprintf("https://example.com/oauth/auth?redirect_uri=%s&state=%s", redirectURI, state)
}

// =============================================================================
// Integration Test Suite
// =============================================================================

// TestProviderFactory_EndToEndFlow tests the complete end-to-end flow for different provider types
func TestProviderFactory_EndToEndFlow(t *testing.T) {
	factory := NewProviderFactory()
	mockServer := newMockHTTPServer()
	defer mockServer.close()

	testCases := []struct {
		name            string
		providerName    string
		providerType    types.ProviderType
		config          map[string]interface{}
		setupProvider   func() *mockProvider
		expectedStatus  types.TestStatus
		expectedPhase   types.TestPhase
		expectedDetails map[string]string
	}{
		{
			name:         "OAuth provider successful flow",
			providerName: "gemini",
			providerType: types.ProviderTypeGemini,
			config: map[string]interface{}{
				"oauth_configured": true,
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					testable:     true,
					oauth:        true,
					refreshToken: true,
					models: []types.Model{
						{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: types.ProviderTypeGemini},
						{ID: "gemini-2.0-pro", Name: "Gemini 2.0 Pro", Provider: types.ProviderTypeGemini},
					},
				}
			},
			expectedStatus: types.TestStatusSuccess,
			expectedPhase:  types.TestPhaseCompleted,
			expectedDetails: map[string]string{
				"auth_method":               "oauth",
				"token_valid":               "true",
				"supports_connectivity_test": "true",
				"supports_models":           "true",
				"models_count":              "2",
			},
		},
		{
			name:         "API key provider successful flow",
			providerName: "openai",
			providerType: types.ProviderTypeOpenAI,
			config: map[string]interface{}{
				"api_key": "test-api-key",
				"base_url": mockServer.url(),
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					testable:     true,
					models: []types.Model{
						{ID: "gpt-4", Name: "GPT-4", Provider: types.ProviderTypeOpenAI},
						{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Provider: types.ProviderTypeOpenAI},
					},
				}
			},
			expectedStatus: types.TestStatusSuccess,
			expectedPhase:  types.TestPhaseCompleted,
			expectedDetails: map[string]string{
				"auth_method":               "api_key",
				"config_validated":          "true",
				"supports_connectivity_test": "true",
				"supports_models":           "true",
				"models_count":              "2",
			},
		},
		{
			name:         "Virtual provider (no test methods)",
			providerName: "fallback",
			providerType: types.ProviderTypeFallback,
			config: map[string]interface{}{
				"providers": []string{"openai", "anthropic"},
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeFallback,
					testable:     false, // Virtual providers typically don't implement TestableProvider
					oauth:        false, // Explicitly set to false
					models:       []types.Model{}, // May not provide models
				}
			},
			expectedStatus: types.TestStatusSuccess,
			expectedPhase:  types.TestPhaseCompleted,
			expectedDetails: map[string]string{
				"auth_method":               "api_key",
				"config_validated":          "true",
				"supports_connectivity_test": "false",
				"supports_models":           "false",
				"connectivity_test":         "skipped",
				"skip_reason":               "no_test_method_available",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Register the mock provider
			baseProvider := tc.setupProvider()
			factory.RegisterProvider(tc.providerType, func(config types.ProviderConfig) types.Provider {
				if baseProvider.oauth {
					return &mockOAuthProvider{mockProvider: baseProvider}
				}
				return baseProvider
			})

			// Test the provider
			result, err := factory.TestProvider(context.Background(), tc.providerName, tc.config)
			if err != nil {
				t.Fatalf("TestProvider returned error: %v", err)
			}

			if result == nil {
				t.Fatal("TestProvider returned nil result")
			}

			// Verify status
			if result.Status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, result.Status)
			}

			// Verify phase
			if result.Phase != tc.expectedPhase {
				t.Errorf("Expected phase %s, got %s", tc.expectedPhase, result.Phase)
			}

			// Verify provider type
			if result.ProviderType != tc.providerType {
				t.Errorf("Expected provider type %s, got %s", tc.providerType, result.ProviderType)
			}

			// Verify details
			for key, expectedValue := range tc.expectedDetails {
				actualValue, exists := result.GetDetail(key)
				if !exists {
					t.Errorf("Expected detail '%s' not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected detail '%s' = '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Verify timing information is reasonable
			if result.Duration < 0 {
				t.Error("Expected non-negative duration")
			}
			if result.Timestamp.IsZero() {
				t.Error("Expected non-zero timestamp")
			}
			if time.Since(result.Timestamp) > time.Minute {
				t.Error("Expected recent timestamp")
			}
		})
	}
}

// TestProviderFactory_ErrorScenarios tests various error conditions
func TestProviderFactory_ErrorScenarios(t *testing.T) {
	factory := NewProviderFactory()
	mockServer := newMockHTTPServer()
	defer mockServer.close()

	testCases := []struct {
		name            string
		providerName    string
		providerType    types.ProviderType
		config          map[string]interface{}
		setupProvider   func() *mockProvider
		expectedStatus  types.TestStatus
		expectedPhase   types.TestPhase
		expectedError   string
		expectedDetails map[string]string
	}{
		{
			name:         "Invalid OAuth token",
			providerName: "gemini",
			providerType: types.ProviderTypeGemini,
			config: map[string]interface{}{
				"oauth_configured": true,
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					testable:     true,
					oauth:        true,
					shouldFail:   true,
					failReason:   "invalid_token",
					failPhase:    types.TestPhaseAuthentication,
				}
			},
			expectedStatus: types.TestStatusAuthFailed,
			expectedPhase:  types.TestPhaseAuthentication,
			expectedError:  "invalid_token",
			expectedDetails: map[string]string{
				"auth_method": "oauth",
			},
		},
		{
			name:         "Expired OAuth token with successful refresh",
			providerName: "gemini",
			providerType: types.ProviderTypeGemini,
			config: map[string]interface{}{
				"oauth_configured": true,
			},
			setupProvider: func() *mockProvider {
				expiredTime := time.Now().Add(-time.Hour)
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					testable:     true,
					oauth:        true,
					refreshToken: true,
					tokenInfo: &types.TokenInfo{
						Valid:     true,
						ExpiresAt: expiredTime, // Expired token
						Scope:     []string{"read"},
					},
				}
			},
			expectedStatus: types.TestStatusSuccess,
			expectedPhase:  types.TestPhaseCompleted,
			expectedDetails: map[string]string{
				"auth_method": "oauth",
				"token_valid": "true",
			},
		},
		{
			name:         "Expired OAuth token with failed refresh",
			providerName: "gemini",
			providerType: types.ProviderTypeGemini,
			config: map[string]interface{}{
				"oauth_configured": true,
			},
			setupProvider: func() *mockProvider {
				expiredTime := time.Now().Add(-time.Hour)
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					testable:     true,
					oauth:        true,
					refreshToken: true,
					shouldFail:   true,
					failReason:   "refresh token failed",
					failPhase:    types.TestPhaseAuthentication,
					tokenInfo: &types.TokenInfo{
						Valid:     true,
						ExpiresAt: expiredTime, // Expired token
						Scope:     []string{"read"},
					},
				}
			},
			expectedStatus: types.TestStatusTokenFailed,
			expectedPhase:  types.TestPhaseAuthentication,
			expectedError:  "Token expired and refresh failed",
			expectedDetails: map[string]string{
				"auth_method": "oauth",
			},
		},
		{
			name:         "Connectivity failure",
			providerName: "openai",
			providerType: types.ProviderTypeOpenAI,
			config: map[string]interface{}{
				"api_key": "test-api-key",
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					testable:     true,
					shouldFail:   true,
					failReason:   "connection refused",
					failPhase:    types.TestPhaseConnectivity,
				}
			},
			expectedStatus: types.TestStatusConnectivityFailed,
			expectedPhase:  types.TestPhaseConnectivity,
			expectedError:  "connection refused",
			expectedDetails: map[string]string{
				"auth_method": "api_key",
			},
		},
		{
			name:         "Models fetch failure",
			providerName: "anthropic",
			providerType: types.ProviderTypeAnthropic,
			config: map[string]interface{}{
				"api_key": "test-api-key",
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeAnthropic,
					testable:     true,
					modelsError:  errors.New("models API unavailable"),
				}
			},
			expectedStatus: types.TestStatusConnectivityFailed,
			expectedPhase:  types.TestPhaseModelFetch,
			expectedError:  "Failed to fetch models",
			expectedDetails: map[string]string{
				"auth_method": "api_key",
				"models_error": "models API unavailable",
			},
		},
		{
			name:         "Rate limit error",
			providerName: "openai",
			providerType: types.ProviderTypeOpenAI,
			config: map[string]interface{}{
				"api_key": "test-api-key",
			},
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					testable:     true,
					shouldFail:   true,
					failReason:   "rate limit exceeded",
					failPhase:    types.TestPhaseConnectivity,
				}
			},
			expectedStatus: types.TestStatusRateLimited,
			expectedPhase:  types.TestPhaseConnectivity,
			expectedError:  "rate limit exceeded",
			expectedDetails: map[string]string{
				"auth_method": "api_key",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Register the mock provider
			baseProvider := tc.setupProvider()
			factory.RegisterProvider(tc.providerType, func(config types.ProviderConfig) types.Provider {
				if baseProvider.oauth {
					return &mockOAuthProvider{mockProvider: baseProvider}
				}
				return baseProvider
			})

			// Test the provider
			result, err := factory.TestProvider(context.Background(), tc.providerName, tc.config)
			if err != nil {
				t.Fatalf("TestProvider returned error: %v", err)
			}

			if result == nil {
				t.Fatal("TestProvider returned nil result")
			}

			// Verify status
			if result.Status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, result.Status)
			}

			// Verify phase
			if result.Phase != tc.expectedPhase {
				t.Errorf("Expected phase %s, got %s", tc.expectedPhase, result.Phase)
			}

			// Verify error message contains expected text
			if tc.expectedError != "" && !strings.Contains(result.Error, tc.expectedError) {
				t.Errorf("Expected error to contain '%s', got '%s'", tc.expectedError, result.Error)
			}

			// Verify TestError details
			if result.TestError != nil {
				if result.TestError.ProviderType != tc.providerType {
					t.Errorf("Expected TestError.ProviderType %s, got %s", tc.providerType, result.TestError.ProviderType)
				}
				if result.TestError.Phase != tc.expectedPhase {
					t.Errorf("Expected TestError.Phase %s, got %s", tc.expectedPhase, result.TestError.Phase)
				}
			}

			// Verify expected details
			for key, expectedValue := range tc.expectedDetails {
				actualValue, exists := result.GetDetail(key)
				if !exists {
					t.Errorf("Expected detail '%s' not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected detail '%s' = '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Verify result is not successful
			if result.IsSuccess() {
				t.Error("Expected result to indicate failure")
			}

			// Verify result indicates error
			if !result.IsError() {
				t.Error("Expected result to indicate error")
			}
		})
	}
}

// TestProviderFactory_InterfaceCompliance tests interface compliance and type casting
func TestProviderFactory_InterfaceCompliance(t *testing.T) {
	factory := NewProviderFactory()

	testCases := []struct {
		name         string
		providerName string
		providerType types.ProviderType
		setupProvider func() *mockProvider
		expectedOAuth      bool
		expectedTestable  bool
	}{
		{
			name:         "OAuth-only provider",
			providerName: "gemini",
			providerType: types.ProviderTypeGemini,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					oauth:        true,
					testable:     false,
				}
			},
			expectedOAuth:     true,
			expectedTestable: false,
		},
		{
			name:         "Testable-only provider",
			providerName: "openai",
			providerType: types.ProviderTypeOpenAI,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					oauth:        false,
					testable:     true,
				}
			},
			expectedOAuth:     false,
			expectedTestable: true,
		},
		{
			name:         "Both interfaces provider",
			providerName: "anthropic",
			providerType: types.ProviderTypeAnthropic,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeAnthropic,
					oauth:        true,
					testable:     true,
					refreshToken: true,
				}
			},
			expectedOAuth:     true,
			expectedTestable: true,
		},
		{
			name:         "No special interfaces provider",
			providerName: "fallback",
			providerType: types.ProviderTypeFallback,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeFallback,
					oauth:        false,
					testable:     false,
				}
			},
			expectedOAuth:     false,
			expectedTestable: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Register the mock provider
			baseProvider := tc.setupProvider()
			factory.RegisterProvider(tc.providerType, func(config types.ProviderConfig) types.Provider {
				if baseProvider.oauth {
					return &mockOAuthProvider{mockProvider: baseProvider}
				}
				return baseProvider
			})

			// Create provider instance to test interfaces
			providerConfig := types.ProviderConfig{
				Type:           tc.providerType,
				ProviderConfig: map[string]interface{}{},
			}
			instance, err := factory.CreateProvider(tc.providerType, providerConfig)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			// Test interface detection
			isOAuth := types.IsOAuthProvider(instance)
			if isOAuth != tc.expectedOAuth {
				t.Errorf("Expected IsOAuthProvider = %v, got %v", tc.expectedOAuth, isOAuth)
			}

			isTestable := types.IsTestableProvider(instance)
			if isTestable != tc.expectedTestable {
				t.Errorf("Expected IsTestableProvider = %v, got %v", tc.expectedTestable, isTestable)
			}

			// Test interface casting
			if tc.expectedOAuth {
				oauthProvider, ok := types.AsOAuthProvider(instance)
				if !ok {
					t.Error("Expected successful cast to OAuthProvider")
				}
				if oauthProvider == nil {
					t.Error("Expected non-nil OAuthProvider after casting")
				}
			} else {
				_, ok := types.AsOAuthProvider(instance)
				if ok {
					t.Error("Expected failed cast to OAuthProvider")
				}
			}

			if tc.expectedTestable {
				testableProvider, ok := types.AsTestableProvider(instance)
				if !ok {
					t.Error("Expected successful cast to TestableProvider")
				}
				if testableProvider == nil {
					t.Error("Expected non-nil TestableProvider after casting")
				}
			} else {
				_, ok := types.AsTestableProvider(instance)
				if ok {
					t.Error("Expected failed cast to TestableProvider")
				}
			}

			// Test through TestProvider
			result, err := factory.TestProvider(context.Background(), tc.providerName, map[string]interface{}{})
			if err != nil {
				t.Fatalf("TestProvider returned error: %v", err)
			}

			if result == nil {
				t.Fatal("TestProvider returned nil result")
			}

			// Verify interface capability details
			authMethod, exists := result.GetDetail("auth_method")
			if tc.expectedOAuth {
				if !exists || authMethod != "oauth" {
					t.Errorf("Expected auth_method 'oauth', got '%s'", authMethod)
				}
			} else {
				if !exists || authMethod != "api_key" {
					t.Errorf("Expected auth_method 'api_key', got '%s'", authMethod)
				}
			}

			supportsConnectivity, exists := result.GetDetail("supports_connectivity_test")
			if tc.expectedTestable {
				if !exists || supportsConnectivity != "true" {
					t.Errorf("Expected supports_connectivity_test 'true', got '%s'", supportsConnectivity)
				}
			}
		})
	}
}

// TestProviderFactory_ContextCancellation tests context cancellation and timeout scenarios
func TestProviderFactory_ContextCancellation(t *testing.T) {
	factory := NewProviderFactory()

	// Test context cancellation
	t.Run("Context cancellation", func(t *testing.T) {
		// Create a provider that simulates long-running operations
		provider := &mockProvider{
			providerType: types.ProviderTypeOpenAI,
			testable:     true,
			shouldFail:   true,
			failReason:   "operation cancelled",
			failPhase:    types.TestPhaseConnectivity,
		}

		factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
			return provider
		})

		// Create a cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel the context immediately
		cancel()

		// Test the provider with cancelled context
		result, err := factory.TestProvider(ctx, "openai", map[string]interface{}{"api_key": "test"})
		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		if result == nil {
			t.Fatal("TestProvider returned nil result")
		}

		// Result should indicate failure due to context cancellation
		if result.IsSuccess() {
			t.Error("Expected result to indicate failure")
		}
	})

	// Test timeout
	t.Run("Context timeout", func(t *testing.T) {
		// Create a provider that takes a long time to respond
		mockServer := newMockHTTPServer()
		defer mockServer.close()

		// Set a long delay to trigger timeout
		mockServer.setDelay(5 * time.Second)

		provider := &mockProvider{
			providerType: types.ProviderTypeOpenAI,
			testable:     true,
		}

		factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
			return provider
		})

		// Create a context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		startTime := time.Now()
		result, err := factory.TestProvider(ctx, "openai", map[string]interface{}{"api_key": "test"})
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		if result == nil {
			t.Fatal("TestProvider returned nil result")
		}

		// Test should complete quickly due to timeout (within reasonable tolerance)
		if duration > 2*time.Second {
			t.Errorf("Expected test to complete quickly due to timeout, took %v", duration)
		}

		// Result should indicate failure
		if result.IsSuccess() {
			t.Error("Expected result to indicate failure")
		}

		// Should have timeout-related error
		if result.Status != types.TestStatusTimeoutFailed {
			t.Errorf("Expected timeout status, got %s", result.Status)
		}
	})
}

// TestProviderFactory_ConcurrentTesting tests concurrent provider testing
func TestProviderFactory_ConcurrentTesting(t *testing.T) {
	factory := NewProviderFactory()
	mockServer := newMockHTTPServer()
	defer mockServer.close()

	// Register multiple providers
	providers := []struct {
		name         string
		providerType types.ProviderType
		setupProvider func() *mockProvider
	}{
		{
			name:         "openai",
			providerType: types.ProviderTypeOpenAI,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeOpenAI,
					testable:     true,
					oauth:        false, // Explicitly set to false for API key providers
					models: []types.Model{
						{ID: "gpt-4", Name: "GPT-4", Provider: types.ProviderTypeOpenAI},
					},
				}
			},
		},
		{
			name:         "anthropic",
			providerType: types.ProviderTypeAnthropic,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeAnthropic,
					testable:     true,
					oauth:        false, // Explicitly set to false for API key providers
					models: []types.Model{
						{ID: "claude-3", Name: "Claude 3", Provider: types.ProviderTypeAnthropic},
					},
				}
			},
		},
		{
			name:         "gemini",
			providerType: types.ProviderTypeGemini,
			setupProvider: func() *mockProvider {
				return &mockProvider{
					providerType: types.ProviderTypeGemini,
					testable:     true,
					oauth:        true,
					models: []types.Model{
						{ID: "gemini-pro", Name: "Gemini Pro", Provider: types.ProviderTypeGemini},
					},
				}
			},
		},
	}

	// Register all providers
	for _, p := range providers {
		provider := p.setupProvider()
		factory.RegisterProvider(p.providerType, func(config types.ProviderConfig) types.Provider {
			return provider
		})
	}

	// Test concurrent execution
	const numGoroutines = 10
	const numIterations = 5

	var wg sync.WaitGroup
	results := make(chan *types.TestResult, numGoroutines*numIterations)
	errors := make(chan error, numGoroutines*numIterations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				provider := providers[j%len(providers)]
				result, err := factory.TestProvider(context.Background(), provider.name, map[string]interface{}{
					"api_key": fmt.Sprintf("test-key-%d-%d", goroutineID, j),
				})

				if err != nil {
					errors <- fmt.Errorf("goroutine %d iteration %d: %v", goroutineID, j, err)
					continue
				}

				if result == nil {
					errors <- fmt.Errorf("goroutine %d iteration %d: nil result", goroutineID, j)
					continue
				}

				results <- result
			}
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify results
	resultCount := 0
	for result := range results {
		resultCount++

		// All tests should be successful in this scenario
		if !result.IsSuccess() {
			t.Errorf("Expected successful result, got status %s: %s", result.Status, result.Error)
		}

		// Verify provider type is valid
		validType := false
		for _, p := range providers {
			if result.ProviderType == p.providerType {
				validType = true
				break
			}
		}
		if !validType {
			t.Errorf("Invalid provider type in result: %s", result.ProviderType)
		}

		// Verify timing information
		if result.Duration < 0 {
			t.Error("Expected non-negative duration")
		}
		if result.Timestamp.IsZero() {
			t.Error("Expected non-zero timestamp")
		}
	}

	expectedResultCount := numGoroutines * numIterations
	if resultCount != expectedResultCount {
		t.Errorf("Expected %d results, got %d", expectedResultCount, resultCount)
	}
}

// TestProviderFactory_TestResultValidation tests TestResult serialization and validation
func TestProviderFactory_TestResultValidation(t *testing.T) {
	factory := NewProviderFactory()
	mockServer := newMockHTTPServer()
	defer mockServer.close()

	// Create a provider with all features enabled
	provider := &mockProvider{
		providerType: types.ProviderTypeGemini,
		testable:     true,
		oauth:        true,
		refreshToken: true,
		models: []types.Model{
			{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: types.ProviderTypeGemini},
			{ID: "gemini-2.0-pro", Name: "Gemini 2.0 Pro", Provider: types.ProviderTypeGemini},
		},
	}

	factory.RegisterProvider(types.ProviderTypeGemini, func(config types.ProviderConfig) types.Provider {
		return provider
	})

	// Test provider and get result
	result, err := factory.TestProvider(context.Background(), "gemini", map[string]interface{}{
		"oauth_configured": true,
	})
	if err != nil {
		t.Fatalf("TestProvider returned error: %v", err)
	}

	if result == nil {
		t.Fatal("TestProvider returned nil result")
	}

	// Test JSON serialization
	jsonData, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Verify JSON is valid
	var parsedResult types.TestResult
	err = json.Unmarshal(jsonData, &parsedResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify all fields are preserved
	if parsedResult.Status != result.Status {
		t.Errorf("Status not preserved: expected %s, got %s", result.Status, parsedResult.Status)
	}

	if parsedResult.ProviderType != result.ProviderType {
		t.Errorf("ProviderType not preserved: expected %s, got %s", result.ProviderType, parsedResult.ProviderType)
	}

	if parsedResult.ModelsCount != result.ModelsCount {
		t.Errorf("ModelsCount not preserved: expected %d, got %d", result.ModelsCount, parsedResult.ModelsCount)
	}

	if parsedResult.Phase != result.Phase {
		t.Errorf("Phase not preserved: expected %s, got %s", result.Phase, parsedResult.Phase)
	}

	// Verify details are preserved
	for key, expectedValue := range result.Details {
		actualValue, exists := parsedResult.GetDetail(key)
		if !exists {
			t.Errorf("Detail '%s' not preserved", key)
		} else if actualValue != expectedValue {
			t.Errorf("Detail '%s' not preserved: expected '%s', got '%s'", key, expectedValue, actualValue)
		}
	}

	// Verify TestError is preserved if present
	if result.TestError != nil {
		if parsedResult.TestError == nil {
			t.Error("TestError not preserved")
		} else {
			if parsedResult.TestError.ErrorType != result.TestError.ErrorType {
				t.Errorf("TestError.ErrorType not preserved: expected %s, got %s",
					result.TestError.ErrorType, parsedResult.TestError.ErrorType)
			}
			if parsedResult.TestError.Message != result.TestError.Message {
				t.Errorf("TestError.Message not preserved: expected %s, got %s",
					result.TestError.Message, parsedResult.TestError.Message)
			}
		}
	}

	// Test JSON string serialization
	jsonString, err := result.ToJSONString()
	if err != nil {
		t.Fatalf("ToJSONString failed: %v", err)
	}

	if jsonString == "" {
		t.Error("ToJSONString returned empty string")
	}

	// Verify JSON string can be parsed back
	var parsedFromString types.TestResult
	err = json.Unmarshal([]byte(jsonString), &parsedFromString)
	if err != nil {
		t.Fatalf("Failed to parse JSON string: %v", err)
	}

	if parsedFromString.Status != result.Status {
		t.Error("JSON string serialization failed to preserve status")
	}
}

// TestProviderFactory_RealWorldScenarios tests real-world scenarios
func TestProviderFactory_RealWorldScenarios(t *testing.T) {
	factory := NewProviderFactory()
	mockServer := newMockHTTPServer()
	defer mockServer.close()

	t.Run("Token expiration and refresh", func(t *testing.T) {
		// Create provider with expiring token
		expiringTime := time.Now().Add(10 * time.Millisecond) // Will expire very soon
		provider := &mockProvider{
			providerType: types.ProviderTypeGemini,
			testable:     true,
			oauth:        true,
			refreshToken: true,
			tokenInfo: &types.TokenInfo{
				Valid:     true,
				ExpiresAt: expiringTime,
				Scope:     []string{"read", "write"},
				UserInfo: map[string]interface{}{
					"id": "test-user",
				},
			},
		}

		factory.RegisterProvider(types.ProviderTypeGemini, func(config types.ProviderConfig) types.Provider {
			return provider
		})

		// Wait for token to expire
		time.Sleep(20 * time.Millisecond)

		// Test should trigger refresh
		result, err := factory.TestProvider(context.Background(), "gemini", map[string]interface{}{
			"oauth_configured": true,
		})
		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		if result == nil {
			t.Fatal("TestProvider returned nil result")
		}

		// Test should be successful after refresh
		if !result.IsSuccess() {
			t.Errorf("Expected successful test after token refresh, got status %s: %s",
				result.Status, result.Error)
		}

		// Verify token was refreshed (should have new expiration time)
		if provider.tokenInfo != nil {
			if time.Now().After(provider.tokenInfo.ExpiresAt) {
				t.Error("Token was not properly refreshed")
			}
		}
	})

	t.Run("API key rotation scenario", func(t *testing.T) {
		// Create provider with failing first API key
		provider := &mockProvider{
			providerType: types.ProviderTypeOpenAI,
			testable:     true,
			shouldFail:   true,
			failReason:   "invalid API key",
			failPhase:    types.TestPhaseAuthentication,
		}

		factory.RegisterProvider(types.ProviderTypeOpenAI, func(config types.ProviderConfig) types.Provider {
			return provider
		})

		// Test with invalid API key should fail
		result, err := factory.TestProvider(context.Background(), "openai", map[string]interface{}{
			"api_key": "invalid-key",
		})
		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		if result == nil {
			t.Fatal("TestProvider returned nil result")
		}

		// Should fail with authentication error
		if result.IsSuccess() {
			t.Error("Expected test to fail with invalid API key")
		}

		if result.Status != types.TestStatusAuthFailed {
			t.Errorf("Expected auth failure status, got %s", result.Status)
		}

		// Now test with valid API key
		provider.shouldFail = false
		provider.failReason = ""

		result, err = factory.TestProvider(context.Background(), "openai", map[string]interface{}{
			"api_key": "valid-key",
		})
		if err != nil {
			t.Fatalf("TestProvider returned error: %v", err)
		}

		// Should succeed with valid API key
		if !result.IsSuccess() {
			t.Errorf("Expected test to succeed with valid API key, got status %s: %s",
				result.Status, result.Error)
		}
	})

	t.Run("Multiple provider testing in sequence", func(t *testing.T) {
		// Create multiple providers
		providers := []struct {
			name         string
			providerType types.ProviderType
			config       map[string]interface{}
			setupProvider func() *mockProvider
		}{
			{
				name:         "openai",
				providerType: types.ProviderTypeOpenAI,
				config:       map[string]interface{}{"api_key": "test-openai"},
				setupProvider: func() *mockProvider {
					return &mockProvider{
						providerType: types.ProviderTypeOpenAI,
						testable:     true,
						models: []types.Model{{ID: "gpt-4", Name: "GPT-4", Provider: types.ProviderTypeOpenAI}},
					}
				},
			},
			{
				name:         "anthropic",
				providerType: types.ProviderTypeAnthropic,
				config:       map[string]interface{}{"api_key": "test-anthropic"},
				setupProvider: func() *mockProvider {
					return &mockProvider{
						providerType: types.ProviderTypeAnthropic,
						testable:     true,
						models: []types.Model{{ID: "claude-3", Name: "Claude 3", Provider: types.ProviderTypeAnthropic}},
					}
				},
			},
			{
				name:         "gemini",
				providerType: types.ProviderTypeGemini,
				config:       map[string]interface{}{"oauth_configured": true},
				setupProvider: func() *mockProvider {
					return &mockProvider{
						providerType: types.ProviderTypeGemini,
						testable:     true,
						oauth:        true,
						models: []types.Model{{ID: "gemini-pro", Name: "Gemini Pro", Provider: types.ProviderTypeGemini}},
					}
				},
			},
		}

		// Register all providers
		for _, p := range providers {
			provider := p.setupProvider()
			factory.RegisterProvider(p.providerType, func(config types.ProviderConfig) types.Provider {
				return provider
			})
		}

		// Test all providers in sequence
		results := make([]*types.TestResult, 0, len(providers))
		for _, p := range providers {
			result, err := factory.TestProvider(context.Background(), p.name, p.config)
			if err != nil {
				t.Fatalf("TestProvider for %s returned error: %v", p.name, err)
			}
			if result == nil {
				t.Fatalf("TestProvider for %s returned nil result", p.name)
			}
			results = append(results, result)
		}

		// All tests should be successful
		for i, result := range results {
			if !result.IsSuccess() {
				t.Errorf("Provider %d test failed: status %s, error %s",
					i, result.Status, result.Error)
			}

			// Verify each provider was tested correctly
			if result.ProviderType != providers[i].providerType {
				t.Errorf("Provider %d type mismatch: expected %s, got %s",
					i, providers[i].providerType, result.ProviderType)
			}

			// Verify timing is reasonable
			if result.Duration < 0 {
				t.Errorf("Provider %d has negative duration: %v", i, result.Duration)
			}

			if result.Timestamp.IsZero() {
				t.Errorf("Provider %d has zero timestamp", i)
			}
		}
	})
}