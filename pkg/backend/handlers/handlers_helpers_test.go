package handlers

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/auth"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/extensions"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/middleware"
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
