package base

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	_, _ = w.Write([]byte(m.response))
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
