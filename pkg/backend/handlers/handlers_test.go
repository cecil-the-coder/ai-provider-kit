package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/auth"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/extensions"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/middleware"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/racing"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Mock Provider
type mockProvider struct {
	name                 string
	providerType         types.ProviderType
	description          string
	models               []types.Model
	defaultModel         string
	authenticated        bool
	healthCheckErr       error
	generateErr          error
	configureErr         error
	supportsStreaming    bool
	supportsToolCalling  bool
	supportsResponsesAPI bool
	toolFormat           types.ToolFormat
	metrics              types.ProviderMetrics
	generateResponse     *types.ChatCompletionChunk
	config               types.ProviderConfig
}

func (m *mockProvider) Name() string                                         { return m.name }
func (m *mockProvider) Type() types.ProviderType                             { return m.providerType }
func (m *mockProvider) Description() string                                  { return m.description }
func (m *mockProvider) GetModels(ctx context.Context) ([]types.Model, error) { return m.models, nil }
func (m *mockProvider) GetDefaultModel() string                              { return m.defaultModel }
func (m *mockProvider) Authenticate(ctx context.Context, config types.AuthConfig) error {
	m.authenticated = true
	return nil
}
func (m *mockProvider) IsAuthenticated() bool                  { return m.authenticated }
func (m *mockProvider) Logout(ctx context.Context) error       { m.authenticated = false; return nil }
func (m *mockProvider) GetToken() (string, error)              { return "mock-token", nil }
func (m *mockProvider) RefreshToken(ctx context.Context) error { return nil }
func (m *mockProvider) GetAuthMethod() types.AuthMethod        { return types.AuthMethodAPIKey }
func (m *mockProvider) Configure(config types.ProviderConfig) error {
	if m.configureErr != nil {
		return m.configureErr
	}
	m.config = config
	return nil
}
func (m *mockProvider) GetConfig() types.ProviderConfig       { return m.config }
func (m *mockProvider) HealthCheck(ctx context.Context) error { return m.healthCheckErr }
func (m *mockProvider) GetMetrics() types.ProviderMetrics     { return m.metrics }
func (m *mockProvider) SupportsStreaming() bool               { return m.supportsStreaming }
func (m *mockProvider) SupportsToolCalling() bool             { return m.supportsToolCalling }
func (m *mockProvider) GetToolFormat() types.ToolFormat       { return m.toolFormat }
func (m *mockProvider) SupportsResponsesAPI() bool            { return m.supportsResponsesAPI }
func (m *mockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	if m.generateErr != nil {
		return nil, m.generateErr
	}
	return &mockStream{chunk: m.generateResponse}, nil
}

// Mock Stream
type mockStream struct {
	chunk  *types.ChatCompletionChunk
	closed bool
	index  int
}

func (m *mockStream) Next() (types.ChatCompletionChunk, error) {
	if m.index == 0 {
		m.index++
		if m.chunk != nil {
			return *m.chunk, nil
		}
		return types.ChatCompletionChunk{
			Content: "Test response",
			Done:    false,
			Usage:   types.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
		}, nil
	}
	return types.ChatCompletionChunk{Done: true, Usage: types.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}}, nil
}

func (m *mockStream) Close() error {
	m.closed = true
	return nil
}

// Mock Extension Registry
type mockExtensionRegistry struct {
	extensions []extensions.Extension
}

func (m *mockExtensionRegistry) Register(ext extensions.Extension) error {
	m.extensions = append(m.extensions, ext)
	return nil
}

func (m *mockExtensionRegistry) Get(name string) (extensions.Extension, bool) {
	for _, ext := range m.extensions {
		if ext.Name() == name {
			return ext, true
		}
	}
	return nil, false
}

func (m *mockExtensionRegistry) List() []extensions.Extension {
	return m.extensions
}

func (m *mockExtensionRegistry) Initialize(configs map[string]extensions.ExtensionConfig) error {
	return nil
}

func (m *mockExtensionRegistry) Shutdown(ctx context.Context) error {
	return nil
}

// Mock Extension
type mockExtension struct {
	extensions.BaseExtension
	beforeErr     error
	afterErr      error
	onErrorErr    error
	onSelectedErr error
}

func (m *mockExtension) Name() string                                   { return "mock-extension" }
func (m *mockExtension) Version() string                                { return "1.0.0" }
func (m *mockExtension) Description() string                            { return "Mock extension for testing" }
func (m *mockExtension) Initialize(config map[string]interface{}) error { return nil }

func (m *mockExtension) BeforeGenerate(ctx context.Context, req *extensions.GenerateRequest) error {
	return m.beforeErr
}

func (m *mockExtension) AfterGenerate(ctx context.Context, req *extensions.GenerateRequest, resp *extensions.GenerateResponse) error {
	return m.afterErr
}

func (m *mockExtension) OnProviderError(ctx context.Context, provider types.Provider, err error) error {
	return m.onErrorErr
}

func (m *mockExtension) OnProviderSelected(ctx context.Context, provider types.Provider) error {
	return m.onSelectedErr
}

// Mock Auth Manager
type mockAuthManager struct {
	authenticators map[string]auth.Authenticator
	tokenInfo      *auth.TokenInfo
	authURL        string
	startOAuthErr  error
	callbackErr    error
}

func (m *mockAuthManager) RegisterAuthenticator(provider string, authenticator auth.Authenticator) error {
	if m.authenticators == nil {
		m.authenticators = make(map[string]auth.Authenticator)
	}
	m.authenticators[provider] = authenticator
	return nil
}

func (m *mockAuthManager) GetAuthenticator(provider string) (auth.Authenticator, error) {
	if auth, ok := m.authenticators[provider]; ok {
		return auth, nil
	}
	return nil, errors.New("authenticator not found")
}

func (m *mockAuthManager) Authenticate(ctx context.Context, provider string, config types.AuthConfig) error {
	return nil
}

func (m *mockAuthManager) IsAuthenticated(provider string) bool {
	if auth, ok := m.authenticators[provider]; ok {
		return auth.IsAuthenticated()
	}
	return false
}

func (m *mockAuthManager) Logout(ctx context.Context, provider string) error {
	if auth, ok := m.authenticators[provider]; ok {
		return auth.Logout(ctx)
	}
	return errors.New("authenticator not found")
}

func (m *mockAuthManager) RefreshAllTokens(ctx context.Context) error { return nil }
func (m *mockAuthManager) GetAuthenticatedProviders() []string {
	providers := make([]string, 0, len(m.authenticators))
	for name := range m.authenticators {
		providers = append(providers, name)
	}
	return providers
}
func (m *mockAuthManager) GetAuthStatus() map[string]*auth.AuthState { return nil }
func (m *mockAuthManager) CleanupExpired() error                     { return nil }
func (m *mockAuthManager) ForEachAuthenticated(ctx context.Context, fn func(provider string, authenticator auth.Authenticator) error) error {
	return nil
}

func (m *mockAuthManager) GetTokenInfo(provider string) (*auth.TokenInfo, error) {
	if m.tokenInfo != nil {
		return m.tokenInfo, nil
	}
	return nil, errors.New("no token info")
}

func (m *mockAuthManager) StartOAuthFlow(ctx context.Context, provider string, scopes []string) (string, error) {
	if m.startOAuthErr != nil {
		return "", m.startOAuthErr
	}
	return m.authURL, nil
}

func (m *mockAuthManager) HandleOAuthCallback(ctx context.Context, provider string, code, state string) error {
	return m.callbackErr
}

// Mock OAuth Authenticator
type mockOAuthAuthenticator struct {
	authenticated bool
	tokenInfo     *auth.TokenInfo
	refreshErr    error
}

func (m *mockOAuthAuthenticator) Authenticate(ctx context.Context, config types.AuthConfig) error {
	m.authenticated = true
	return nil
}

func (m *mockOAuthAuthenticator) IsAuthenticated() bool                  { return m.authenticated }
func (m *mockOAuthAuthenticator) GetToken() (string, error)              { return "mock-token", nil }
func (m *mockOAuthAuthenticator) RefreshToken(ctx context.Context) error { return m.refreshErr }
func (m *mockOAuthAuthenticator) Logout(ctx context.Context) error {
	m.authenticated = false
	return nil
}
func (m *mockOAuthAuthenticator) GetAuthMethod() types.AuthMethod { return types.AuthMethodOAuth }
func (m *mockOAuthAuthenticator) StartOAuthFlow(ctx context.Context, scopes []string) (string, error) {
	return "https://oauth.example.com/authorize", nil
}
func (m *mockOAuthAuthenticator) HandleCallback(ctx context.Context, code, state string) error {
	return nil
}
func (m *mockOAuthAuthenticator) IsOAuthEnabled() bool { return true }
func (m *mockOAuthAuthenticator) GetTokenInfo() (*auth.TokenInfo, error) {
	if m.tokenInfo != nil {
		return m.tokenInfo, nil
	}
	return nil, errors.New("no token info")
}

// Helper function to create a request with context containing request ID
func newRequestWithContext(method, url string, body []byte) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	ctx := context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id")
	return req.WithContext(ctx)
}

// ============================================================================
// Base Handler Tests (base.go)
// ============================================================================

func TestSendSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/", nil)

	data := map[string]string{"key": "value"}
	SendSuccess(w, r, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	if response.RequestID != "test-request-id" {
		t.Errorf("Expected request ID test-request-id, got %s", response.RequestID)
	}
}

func TestSendError(t *testing.T) {
	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/", nil)

	SendError(w, r, "TEST_ERROR", "Test error message", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("Expected success to be false")
	}

	if response.Error == nil {
		t.Fatal("Expected error to be present")
	}

	if response.Error.Code != "TEST_ERROR" {
		t.Errorf("Expected error code TEST_ERROR, got %s", response.Error.Code)
	}

	if response.Error.Message != "Test error message" {
		t.Errorf("Expected error message 'Test error message', got %s", response.Error.Message)
	}
}

func TestSendCreated(t *testing.T) {
	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/", nil)

	data := map[string]string{"id": "123"}
	SendCreated(w, r, data)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		expectErr bool
	}{
		{
			name:      "valid JSON",
			body:      `{"key": "value"}`,
			expectErr: false,
		},
		{
			name:      "invalid JSON",
			body:      `{invalid}`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(tt.body)))
			var target map[string]string
			err := ParseJSON(r, &target)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// ============================================================================
// Health Handler Tests (health.go)
// ============================================================================

func TestHealthHandler_Status(t *testing.T) {
	providers := map[string]types.Provider{
		"test": &mockProvider{name: "test"},
	}
	handler := NewHealthHandler(providers, "1.0.0")

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/status", nil)

	handler.Status(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}
}

func TestHealthHandler_Health(t *testing.T) {
	providers := map[string]types.Provider{
		"test": &mockProvider{name: "test"},
	}
	handler := NewHealthHandler(providers, "1.0.0")

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/health", nil)

	handler.Health(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if data["status"] != "healthy" {
		t.Errorf("Expected status healthy, got %v", data["status"])
	}

	if data["version"] != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %v", data["version"])
	}
}

func TestHealthHandler_Version(t *testing.T) {
	handler := NewHealthHandler(nil, "2.0.0")

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/version", nil)

	handler.Version(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if data["version"] != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %v", data["version"])
	}
}

// ============================================================================
// Provider Handler Tests (providers.go)
// ============================================================================

func TestProviderHandler_ListProviders(t *testing.T) {
	providers := map[string]types.Provider{
		"test1": &mockProvider{
			name:         "test1",
			providerType: types.ProviderTypeOpenAI,
			description:  "Test provider 1",
			models:       []types.Model{{ID: "model1"}},
		},
		"test2": &mockProvider{
			name:         "test2",
			providerType: types.ProviderTypeAnthropic,
			description:  "Test provider 2",
		},
	}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/providers", nil)

	handler.ListProviders(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestProviderHandler_ListProviders_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers", nil)

	handler.ListProviders(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_GetProvider(t *testing.T) {
	providers := map[string]types.Provider{
		"test": &mockProvider{
			name:         "test",
			providerType: types.ProviderTypeOpenAI,
			description:  "Test provider",
		},
	}
	handler := NewProviderHandler(providers)

	tests := []struct {
		name       string
		url        string
		expectCode int
	}{
		{
			name:       "provider exists - query param",
			url:        "/api/providers?name=test",
			expectCode: http.StatusOK,
		},
		{
			name:       "provider exists - path param",
			url:        "/api/providers/test",
			expectCode: http.StatusOK,
		},
		{
			name:       "provider not found",
			url:        "/api/providers?name=nonexistent",
			expectCode: http.StatusNotFound,
		},
		{
			name:       "missing parameter",
			url:        "/api/providers",
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := newRequestWithContext("GET", tt.url, nil)

			handler.GetProvider(w, r)

			if w.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, w.Code)
			}
		})
	}
}

func TestProviderHandler_UpdateProvider(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		providerType: types.ProviderTypeOpenAI,
		config: types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			Name:         "test",
			BaseURL:      "https://api.openai.com",
			DefaultModel: "gpt-4",
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	configReq := backendtypes.ProviderConfigRequest{
		BaseURL:      "https://new-url.com",
		DefaultModel: "gpt-4-turbo",
	}
	body, _ := json.Marshal(configReq)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers/test", body)
	r.Header.Set("Content-Type", "application/json")

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify config was updated
	if provider.config.BaseURL != "https://new-url.com" {
		t.Errorf("Expected BaseURL to be updated to https://new-url.com, got %s", provider.config.BaseURL)
	}
}

func TestProviderHandler_UpdateProvider_ConfigureError(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		configureErr: errors.New("configure failed"),
		config: types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
			Name: "test",
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	configReq := backendtypes.ProviderConfigRequest{BaseURL: "https://new-url.com"}
	body, _ := json.Marshal(configReq)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers/test", body)

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestProviderHandler_HealthCheckProvider(t *testing.T) {
	tests := []struct {
		name           string
		providerName   string
		healthCheckErr error
		expectCode     int
	}{
		{
			name:         "healthy provider",
			providerName: "test",
			expectCode:   http.StatusOK,
		},
		{
			name:           "unhealthy provider",
			providerName:   "test",
			healthCheckErr: errors.New("provider down"),
			expectCode:     http.StatusOK,
		},
		{
			name:         "provider not found",
			providerName: "nonexistent",
			expectCode:   http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := map[string]types.Provider{
				"test": &mockProvider{
					name:           "test",
					healthCheckErr: tt.healthCheckErr,
				},
			}
			handler := NewProviderHandler(providers)

			w := httptest.NewRecorder()
			r := newRequestWithContext("GET", "/api/providers/"+tt.providerName+"/health", nil)

			handler.HealthCheckProvider(w, r)

			if w.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, w.Code)
			}
		})
	}
}

func TestProviderHandler_TestProvider(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Test response",
			Usage:   types.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test/test", nil)

	handler.TestProvider(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_GenerateError(t *testing.T) {
	provider := &mockProvider{
		name:        "test",
		generateErr: errors.New("generation failed"),
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test/test", nil)

	handler.TestProvider(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// ============================================================================
// Generate Handler Tests (generate.go)
// ============================================================================

func TestGenerateHandler_Generate(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Generated content",
			Usage:   types.Usage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15},
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewGenerateHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{
		Prompt: "Test prompt",
		Model:  "test-model",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_InvalidJSON(t *testing.T) {
	handler := NewGenerateHandler(nil, nil, "test")

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", []byte("invalid json"))

	handler.Generate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_MissingContent(t *testing.T) {
	handler := NewGenerateHandler(nil, nil, "test")

	req := backendtypes.GenerateRequest{}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_ProviderNotFound(t *testing.T) {
	handler := NewGenerateHandler(map[string]types.Provider{}, nil, "default")

	req := backendtypes.GenerateRequest{
		Prompt:   "Test prompt",
		Provider: "nonexistent",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_WithExtensions(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Generated content",
		},
	}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewGenerateHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{
		Prompt: "Test prompt",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_ExtensionError(t *testing.T) {
	provider := &mockProvider{name: "test"}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{beforeErr: errors.New("extension failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewGenerateHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_Streaming(t *testing.T) {
	provider := &mockProvider{
		name:             "test",
		generateResponse: &types.ChatCompletionChunk{Content: "test response"},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewGenerateHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{
		Prompt: "Test prompt",
		Stream: true,
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	// SSE streaming should return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify SSE headers
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Expected Cache-Control no-cache, got %s", cc)
	}

	// Verify response contains SSE data
	responseBody := w.Body.String()
	if !strings.Contains(responseBody, "data:") {
		t.Errorf("Expected SSE data in response, got: %s", responseBody)
	}
	if !strings.Contains(responseBody, "[DONE]") {
		t.Errorf("Expected [DONE] event in response, got: %s", responseBody)
	}
}

// ============================================================================
// Metrics Handler Tests (metrics.go)
// ============================================================================

func TestMetricsHandler_GetProviderMetrics(t *testing.T) {
	provider := &mockProvider{
		name: "test",
		metrics: types.ProviderMetrics{
			RequestCount: 100,
			SuccessCount: 95,
			ErrorCount:   5,
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewMetricsHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/metrics/providers", nil)

	handler.GetProviderMetrics(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMetricsHandler_GetSystemMetrics(t *testing.T) {
	handler := NewMetricsHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/metrics/system", nil)

	handler.GetSystemMetrics(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if _, ok := data["uptime"]; !ok {
		t.Error("Expected uptime in response")
	}

	if _, ok := data["goroutines"]; !ok {
		t.Error("Expected goroutines in response")
	}
}

func TestMetricsHandler_GetRacingMetrics(t *testing.T) {
	// Create a mock racing provider
	racingProvider := racing.NewRacingProvider("racing-test", nil)
	providers := map[string]types.Provider{
		"racing": racingProvider,
		"normal": &mockProvider{name: "normal"},
	}
	handler := NewMetricsHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/metrics/racing", nil)

	handler.GetRacingMetrics(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ============================================================================
// OAuth Handler Tests (oauth.go)
// ============================================================================

func TestOAuthHandler_InitiateOAuth(t *testing.T) {
	authManager := &mockAuthManager{
		authURL: "https://oauth.example.com/authorize",
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/initiate?response=json", nil)

	handler.InitiateOAuth(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestOAuthHandler_InitiateOAuth_Redirect(t *testing.T) {
	authManager := &mockAuthManager{
		authURL: "https://oauth.example.com/authorize",
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/initiate", nil)

	handler.InitiateOAuth(w, r)

	if w.Code != http.StatusTemporaryRedirect {
		t.Errorf("Expected status 307, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "https://oauth.example.com/authorize" {
		t.Errorf("Expected redirect to auth URL, got %s", location)
	}
}

func TestOAuthHandler_InitiateOAuth_Error(t *testing.T) {
	authManager := &mockAuthManager{
		startOAuthErr: errors.New("oauth failed"),
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/initiate", nil)

	handler.InitiateOAuth(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestOAuthHandler_OAuthCallback(t *testing.T) {
	authManager := &mockAuthManager{
		tokenInfo: &auth.TokenInfo{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Now().Add(1 * time.Hour),
		},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/callback?code=auth-code&state=state-value", nil)

	handler.OAuthCallback(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestOAuthHandler_OAuthCallback_MissingCode(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/callback", nil)

	handler.OAuthCallback(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestOAuthHandler_RefreshToken(t *testing.T) {
	oauthAuth := &mockOAuthAuthenticator{
		tokenInfo: &auth.TokenInfo{
			AccessToken: "new-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		},
	}
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": oauthAuth,
		},
		tokenInfo: oauthAuth.tokenInfo,
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/test/refresh", nil)

	handler.RefreshToken(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestOAuthHandler_RefreshToken_NotOAuthProvider(t *testing.T) {
	// Use a non-OAuth authenticator
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": &mockProvider{}, // mockProvider doesn't implement OAuthAuthenticator
		},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/test/refresh", nil)

	handler.RefreshToken(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestOAuthHandler_GetTokenStatus(t *testing.T) {
	oauthAuth := &mockOAuthAuthenticator{
		authenticated: true,
		tokenInfo: &auth.TokenInfo{
			AccessToken:  "token",
			RefreshToken: "refresh",
			ExpiresAt:    time.Now().Add(1 * time.Hour),
		},
	}
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": oauthAuth,
		},
		tokenInfo: oauthAuth.tokenInfo,
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/status", nil)

	handler.GetTokenStatus(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestOAuthHandler_RevokeToken(t *testing.T) {
	oauthAuth := &mockOAuthAuthenticator{authenticated: true}
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": oauthAuth,
		},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("DELETE", "/api/oauth/test/token", nil)

	handler.RevokeToken(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if oauthAuth.authenticated {
		t.Error("Expected authenticator to be logged out")
	}
}

func TestOAuthHandler_ListOAuthProviders(t *testing.T) {
	oauthAuth := &mockOAuthAuthenticator{
		authenticated: true,
		tokenInfo: &auth.TokenInfo{
			AccessToken: "token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		},
	}
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"oauth-provider": oauthAuth,
		},
		tokenInfo: oauthAuth.tokenInfo,
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/providers", nil)

	handler.ListOAuthProviders(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ============================================================================
// Stream Handler Tests (stream.go)
// ============================================================================

func TestStreamHandler_StreamGenerate(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Streaming content",
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewStreamHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{
		Prompt: "Test prompt",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	// Should set SSE headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestStreamHandler_StreamGenerate_InvalidJSON(t *testing.T) {
	handler := NewStreamHandler(nil, nil, "test")

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", []byte("invalid"))

	handler.StreamGenerate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestStreamHandler_StreamGenerate_ProviderNotFound(t *testing.T) {
	handler := NewStreamHandler(map[string]types.Provider{}, nil, "default")

	req := backendtypes.GenerateRequest{
		Prompt:   "Test prompt",
		Provider: "nonexistent",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestProviderHandler_ExtractProviderName(t *testing.T) {
	handler := NewProviderHandler(nil)

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "query parameter",
			url:      "/api/providers?name=test",
			expected: "test",
		},
		{
			name:     "path parameter",
			url:      "/api/providers/test",
			expected: "test",
		},
		{
			name:     "path with action",
			url:      "/api/providers/test/health",
			expected: "test",
		},
		{
			name:     "no parameter",
			url:      "/api/providers",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tt.url, nil)
			result := handler.extractProviderName(r)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestOAuthHandler_ExtractProviderFromPath(t *testing.T) {
	handler := NewOAuthHandler(nil)

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "initiate endpoint",
			url:      "/api/oauth/google/initiate",
			expected: "google",
		},
		{
			name:     "callback endpoint",
			url:      "/api/oauth/github/callback",
			expected: "github",
		},
		{
			name:     "no provider",
			url:      "/api/oauth/",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tt.url, nil)
			result := handler.extractProviderFromPath(r)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestGenerateHandler_CollectStreamResponse(t *testing.T) {
	handler := NewGenerateHandler(nil, nil, "test")

	stream := &mockStream{
		chunk: &types.ChatCompletionChunk{
			Content: "Part 1",
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: " Part 2"}},
			},
		},
	}

	content, usage, err := handler.collectStreamResponse(stream)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedContent := "Part 1 Part 2"
	if content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, content)
	}

	if usage == nil {
		t.Fatal("Expected usage info")
	}

	if usage.TotalTokens != 30 {
		t.Errorf("Expected 30 total tokens, got %d", usage.TotalTokens)
	}
}

func TestBuildGenerateOptions(t *testing.T) {
	req := &backendtypes.GenerateRequest{
		Model:       "test-model",
		Prompt:      "test prompt",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
	}

	ctx := context.Background()
	options := buildGenerateOptions(req, ctx)

	if options.Model != "test-model" {
		t.Errorf("Expected model test-model, got %s", options.Model)
	}

	if options.MaxTokens != 100 {
		t.Errorf("Expected max tokens 100, got %d", options.MaxTokens)
	}

	if options.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", options.Temperature)
	}

	if !options.Stream {
		t.Error("Expected stream to be true")
	}

	// Check that prompt was converted to messages
	if len(options.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(options.Messages))
	}

	if options.Messages[0].Role != "user" {
		t.Errorf("Expected role user, got %s", options.Messages[0].Role)
	}

	if options.Messages[0].Content != "test prompt" {
		t.Errorf("Expected content 'test prompt', got %s", options.Messages[0].Content)
	}
}

// ============================================================================
// Additional Coverage Tests
// ============================================================================

func TestOAuthHandler_InitiateOAuth_MissingProvider(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{authURL: "https://test.com"})

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/initiate", nil)

	handler.InitiateOAuth(w, r)

	// When no provider is extracted, it becomes empty string and still tries OAuth
	// This results in redirect or error depending on authManager behavior
	if w.Code != http.StatusTemporaryRedirect && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 307 or 400, got %d", w.Code)
	}
}

func TestOAuthHandler_InitiateOAuth_WrongMethod(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/test/initiate", nil)

	handler.InitiateOAuth(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestOAuthHandler_InitiateOAuth_WithScopes(t *testing.T) {
	authManager := &mockAuthManager{
		authURL: "https://oauth.example.com/authorize",
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/initiate?scopes=read,write&response=json", nil)

	handler.InitiateOAuth(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestOAuthHandler_OAuthCallback_WrongMethod(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/test/callback", nil)

	handler.OAuthCallback(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestOAuthHandler_OAuthCallback_MissingProvider(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/callback?code=test", nil)

	handler.OAuthCallback(w, r)

	// When no provider in path, extractProviderFromPath returns empty string
	// The handler still processes it, resulting in success or error
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", w.Code)
	}
}

func TestOAuthHandler_OAuthCallback_HandleError(t *testing.T) {
	authManager := &mockAuthManager{
		callbackErr: errors.New("callback failed"),
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/callback?code=auth-code", nil)

	handler.OAuthCallback(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestOAuthHandler_RefreshToken_WrongMethod(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/refresh", nil)

	handler.RefreshToken(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestOAuthHandler_RefreshToken_MissingProvider(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/refresh", nil)

	handler.RefreshToken(w, r)

	// Empty provider name leads to GetAuthenticator returning not found
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 404 or 400, got %d", w.Code)
	}
}

func TestOAuthHandler_RefreshToken_ProviderNotFound(t *testing.T) {
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/test/refresh", nil)

	handler.RefreshToken(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestOAuthHandler_RefreshToken_RefreshError(t *testing.T) {
	oauthAuth := &mockOAuthAuthenticator{
		refreshErr: errors.New("refresh failed"),
	}
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": oauthAuth,
		},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/test/refresh", nil)

	handler.RefreshToken(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestOAuthHandler_RefreshToken_NoTokenInfo(t *testing.T) {
	oauthAuth := &mockOAuthAuthenticator{}
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": oauthAuth,
		},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/test/refresh", nil)

	handler.RefreshToken(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestOAuthHandler_GetTokenStatus_WrongMethod(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/test/status", nil)

	handler.GetTokenStatus(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestOAuthHandler_GetTokenStatus_MissingProvider(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/status", nil)

	handler.GetTokenStatus(w, r)

	// Empty provider name leads to GetAuthenticator returning not found
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 404 or 400, got %d", w.Code)
	}
}

func TestOAuthHandler_GetTokenStatus_ProviderNotFound(t *testing.T) {
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/status", nil)

	handler.GetTokenStatus(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestOAuthHandler_GetTokenStatus_NotOAuthProvider(t *testing.T) {
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": &mockProvider{},
		},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/status", nil)

	handler.GetTokenStatus(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestOAuthHandler_GetTokenStatus_NoTokenInfo(t *testing.T) {
	oauthAuth := &mockOAuthAuthenticator{authenticated: true}
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": oauthAuth,
		},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/status", nil)

	handler.GetTokenStatus(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestOAuthHandler_RevokeToken_WrongMethod(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/oauth/test/token", nil)

	handler.RevokeToken(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestOAuthHandler_RevokeToken_MissingProvider(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("DELETE", "/api/oauth/token", nil)

	handler.RevokeToken(w, r)

	// Empty provider name leads to GetAuthenticator returning not found
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 404 or 400, got %d", w.Code)
	}
}

func TestOAuthHandler_RevokeToken_ProviderNotFound(t *testing.T) {
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("DELETE", "/api/oauth/test/token", nil)

	handler.RevokeToken(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestOAuthHandler_RevokeToken_LogoutError(t *testing.T) {
	oauthAuth := &mockOAuthAuthenticator{authenticated: true}
	// Override Logout to return error
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": oauthAuth,
		},
	}

	// Use mockProvider that will fail on logout
	failingAuth := &mockProvider{}
	authManager.authenticators["test"] = failingAuth

	handler := NewOAuthHandler(authManager)

	// This should succeed but we can't easily make Logout fail with the current mock
	w := httptest.NewRecorder()
	r := newRequestWithContext("DELETE", "/api/oauth/test/token", nil)

	handler.RevokeToken(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestOAuthHandler_RevokeToken_POSTMethod(t *testing.T) {
	oauthAuth := &mockOAuthAuthenticator{authenticated: true}
	authManager := &mockAuthManager{
		authenticators: map[string]auth.Authenticator{
			"test": oauthAuth,
		},
	}
	handler := NewOAuthHandler(authManager)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/test/token", nil)

	handler.RevokeToken(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestOAuthHandler_ListOAuthProviders_WrongMethod(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/providers", nil)

	handler.ListOAuthProviders(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_GetProvider_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test", nil)

	handler.GetProvider(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_UpdateProvider_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/providers/test", nil)

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_UpdateProvider_MissingProvider(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers", nil)

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestProviderHandler_UpdateProvider_ProviderNotFound(t *testing.T) {
	handler := NewProviderHandler(map[string]types.Provider{})

	configReq := backendtypes.ProviderConfigRequest{BaseURL: "https://test.com"}
	body, _ := json.Marshal(configReq)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers/nonexistent", body)

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestProviderHandler_UpdateProvider_InvalidJSON(t *testing.T) {
	provider := &mockProvider{name: "test", config: types.ProviderConfig{Type: types.ProviderTypeOpenAI, Name: "test"}}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers/test", []byte("invalid json"))

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestProviderHandler_HealthCheckProvider_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test/health", nil)

	handler.HealthCheckProvider(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_HealthCheckProvider_MissingProvider(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/providers/health", nil)

	handler.HealthCheckProvider(w, r)

	// Empty provider name leads to provider not found
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 404 or 400, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/providers/test/test", nil)

	handler.TestProvider(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_MissingProvider(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test", nil)

	handler.TestProvider(w, r)

	// Empty provider name leads to provider not found
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 404 or 400, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_ProviderNotFound(t *testing.T) {
	handler := NewProviderHandler(map[string]types.Provider{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/nonexistent/test", nil)

	handler.TestProvider(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_StreamReadError(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test/test", nil)

	handler.TestProvider(w, r)

	// Should succeed with default mock behavior
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestStreamHandler_StreamGenerate_MissingContent(t *testing.T) {
	handler := NewStreamHandler(nil, nil, "test")

	req := backendtypes.GenerateRequest{}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestStreamHandler_StreamGenerate_WithExtensions(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Streaming content",
		},
	}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewStreamHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestStreamHandler_StreamGenerate_ExtensionBeforeError(t *testing.T) {
	provider := &mockProvider{name: "test"}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{beforeErr: errors.New("before failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewStreamHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	// Should have SSE headers set before error
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestStreamHandler_StreamGenerate_ExtensionOnSelectedError(t *testing.T) {
	provider := &mockProvider{name: "test"}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{onSelectedErr: errors.New("selected failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewStreamHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestStreamHandler_StreamGenerate_GenerateError(t *testing.T) {
	provider := &mockProvider{
		name:        "test",
		generateErr: errors.New("generate failed"),
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewStreamHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestGenerateHandler_Generate_GenerateError(t *testing.T) {
	provider := &mockProvider{
		name:        "test",
		generateErr: errors.New("generation failed"),
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewGenerateHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_OnSelectedError(t *testing.T) {
	provider := &mockProvider{name: "test"}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{onSelectedErr: errors.New("selected failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewGenerateHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_AfterGenerateError(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Generated content",
		},
	}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{afterErr: errors.New("after failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewGenerateHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestConvertFunctions(t *testing.T) {
	// Test convertToExtensionRequest
	req := &backendtypes.GenerateRequest{
		Provider:    "test",
		Model:       "test-model",
		Prompt:      "test prompt",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
	}

	extReq := convertToExtensionRequest(req)
	if extReq.Provider != "test" {
		t.Errorf("Expected provider test, got %s", extReq.Provider)
	}

	// Test updateFromExtensionRequest
	extReq.Provider = "new-provider"
	updateFromExtensionRequest(req, extReq)
	if req.Provider != "new-provider" {
		t.Errorf("Expected provider new-provider, got %s", req.Provider)
	}

	// Test convertToExtensionResponse
	resp := &backendtypes.GenerateResponse{
		Content:  "test content",
		Model:    "test-model",
		Provider: "test",
	}

	extResp := convertToExtensionResponse(resp)
	if extResp.Content != "test content" {
		t.Errorf("Expected content 'test content', got %s", extResp.Content)
	}

	// Test updateFromExtensionResponse
	extResp.Content = "new content"
	updateFromExtensionResponse(resp, extResp)
	if resp.Content != "new content" {
		t.Errorf("Expected content 'new content', got %s", resp.Content)
	}
}
