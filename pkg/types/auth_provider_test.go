package types

import (
	"context"
	"testing"
	"time"
)

// TestTokenInfoCreation validates that TokenInfo struct can be created and used properly
func TestTokenInfoCreation(t *testing.T) {
	tokenInfo := &TokenInfo{
		Valid:     true,
		ExpiresAt: time.Now().Add(time.Hour),
		Scope:     []string{"read", "write"},
		UserInfo: map[string]interface{}{
			"id":    "12345",
			"email": "test@example.com",
		},
	}

	// Test methods
	if tokenInfo.IsExpired() {
		t.Error("Token should not be expired")
	}

	if !tokenInfo.HasScope("read") {
		t.Error("Token should have 'read' scope")
	}

	if tokenInfo.HasScope("admin") {
		t.Error("Token should not have 'admin' scope")
	}
}

// TestInterfaceExport validates that interfaces are properly exported
func TestInterfaceExport(t *testing.T) {
	// This test validates that the interfaces are properly exported
	// by ensuring they can be used as types
	var _ OAuthProvider = (*mockOAuthProvider)(nil)
	var _ TestableProvider = (*mockTestableProvider)(nil)
	var _ Provider = (*mockOAuthProvider)(nil)
	var _ Provider = (*mockTestableProvider)(nil)
}

// mockOAuthProvider implements OAuthProvider for testing
type mockOAuthProvider struct {
	// Embed a mock Provider implementation
	mockProvider
}

func (m *mockOAuthProvider) ValidateToken(ctx context.Context) (*TokenInfo, error) {
	return &TokenInfo{
		Valid:     true,
		ExpiresAt: time.Now().Add(time.Hour),
		Scope:     []string{"read", "write"},
		UserInfo:  map[string]interface{}{},
	}, nil
}

func (m *mockOAuthProvider) RefreshToken(ctx context.Context) error {
	return nil
}

func (m *mockOAuthProvider) GetAuthURL(redirectURI, state string) string {
	return "https://oauth.example.com/auth?redirect_uri=" + redirectURI + "&state=" + state
}

// mockTestableProvider implements TestableProvider for testing
type mockTestableProvider struct {
	// Embed a mock Provider implementation
	mockProvider
}

func (m *mockTestableProvider) TestConnectivity(ctx context.Context) error {
	return nil
}

// mockProvider implements the basic Provider interface for testing
type mockProvider struct{}

func (m *mockProvider) Name() string                                                  { return "mock" }
func (m *mockProvider) Type() ProviderType                                            { return "synthetic" }
func (m *mockProvider) Description() string                                           { return "mock provider" }
func (m *mockProvider) GetModels(ctx context.Context) ([]Model, error)                { return nil, nil }
func (m *mockProvider) GetDefaultModel() string                                       { return "mock-model" }
func (m *mockProvider) Authenticate(ctx context.Context, authConfig AuthConfig) error { return nil }
func (m *mockProvider) IsAuthenticated() bool                                         { return true }
func (m *mockProvider) Logout(ctx context.Context) error                              { return nil }
func (m *mockProvider) Configure(config ProviderConfig) error                         { return nil }
func (m *mockProvider) GetConfig() ProviderConfig                                     { return ProviderConfig{} }
func (m *mockProvider) GenerateChatCompletion(ctx context.Context, options GenerateOptions) (ChatCompletionStream, error) {
	return nil, nil
}
func (m *mockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockProvider) SupportsToolCalling() bool             { return false }
func (m *mockProvider) GetToolFormat() ToolFormat             { return ToolFormatOpenAI }
func (m *mockProvider) SupportsStreaming() bool               { return true }
func (m *mockProvider) SupportsResponsesAPI() bool            { return false }
func (m *mockProvider) HealthCheck(ctx context.Context) error { return nil }
func (m *mockProvider) GetMetrics() ProviderMetrics           { return ProviderMetrics{} }

// TestTypeGuards validates the type guard functions work correctly
func TestTypeGuards(t *testing.T) {
	mockOAuth := &mockOAuthProvider{}
	mockTestable := &mockTestableProvider{}

	// Test OAuth provider type guards
	if !IsOAuthProvider(mockOAuth) {
		t.Error("mockOAuth should be recognized as OAuthProvider")
	}

	if IsOAuthProvider(mockTestable) {
		t.Error("mockTestable should not be recognized as OAuthProvider")
	}

	if _, ok := AsOAuthProvider(mockOAuth); !ok {
		t.Error("mockOAuth should be castable to OAuthProvider")
	}

	// Test Testable provider type guards
	if !IsTestableProvider(mockTestable) {
		t.Error("mockTestable should be recognized as TestableProvider")
	}

	if IsTestableProvider(mockOAuth) {
		t.Error("mockOAuth should not be recognized as TestableProvider")
	}

	if _, ok := AsTestableProvider(mockTestable); !ok {
		t.Error("mockTestable should be castable to TestableProvider")
	}
}
