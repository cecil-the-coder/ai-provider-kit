package common

import (
	"context"
	"net/http"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewAuthHelper(t *testing.T) {
	config := types.ProviderConfig{
		APIKey: "test-key",
	}
	client := &http.Client{}

	helper := NewAuthHelper("openai", config, client)

	if helper == nil {
		t.Fatal("expected non-nil helper")
	}

	if helper.ProviderName != "openai" {
		t.Errorf("got provider name %q, expected %q", helper.ProviderName, "openai")
	}

	if helper.HTTPClient != client {
		t.Error("HTTP client not set correctly")
	}
}

func TestAuthHelper_SetupAPIKeys(t *testing.T) {
	tests := []struct {
		name     string
		config   types.ProviderConfig
		expected int
	}{
		{
			name: "single API key",
			config: types.ProviderConfig{
				APIKey: "key1",
			},
			expected: 1,
		},
		{
			name: "multiple API keys",
			config: types.ProviderConfig{
				APIKey: "key1",
				ProviderConfig: map[string]interface{}{
					"api_keys": []string{"key2", "key3"},
				},
			},
			expected: 3,
		},
		{
			name:     "no API keys",
			config:   types.ProviderConfig{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper := NewAuthHelper("test", tt.config, &http.Client{})
			helper.SetupAPIKeys()

			if tt.expected == 0 {
				if helper.KeyManager != nil {
					t.Error("expected KeyManager to be nil when no keys")
				}
			} else {
				if helper.KeyManager == nil {
					t.Fatal("expected KeyManager to be initialized")
				}
				keys := helper.KeyManager.GetKeys()
				if len(keys) != tt.expected {
					t.Errorf("got %d keys, expected %d", len(keys), tt.expected)
				}
			}
		})
	}
}

func TestAuthHelper_SetupOAuth(t *testing.T) {
	config := types.ProviderConfig{
		OAuthCredentials: []*types.OAuthCredentialSet{
			{
				ID:          "cred1",
				AccessToken: "token1",
			},
		},
	}

	helper := NewAuthHelper("test", config, &http.Client{})
	helper.SetupOAuth(nil) // nil refresh func for testing

	if helper.OAuthManager == nil {
		t.Error("expected OAuthManager to be initialized")
	}
}

func TestAuthHelper_IsAuthenticated(t *testing.T) {
	tests := []struct {
		name        string
		setupHelper func(*AuthHelper)
		expected    bool
	}{
		{
			name: "with API key",
			setupHelper: func(h *AuthHelper) {
				h.Config.APIKey = "test-key"
				h.SetupAPIKeys()
			},
			expected: true,
		},
		{
			name: "with OAuth",
			setupHelper: func(h *AuthHelper) {
				h.Config.OAuthCredentials = []*types.OAuthCredentialSet{
					{ID: "cred1", AccessToken: "token1"},
				}
				h.SetupOAuth(nil)
			},
			expected: true,
		},
		{
			name:        "no auth",
			setupHelper: func(h *AuthHelper) {},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper := NewAuthHelper("test", types.ProviderConfig{}, &http.Client{})
			tt.setupHelper(helper)

			result := helper.IsAuthenticated()
			if result != tt.expected {
				t.Errorf("got %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestAuthHelper_GetAuthMethod(t *testing.T) {
	tests := []struct {
		name        string
		setupHelper func(*AuthHelper)
		expected    string
	}{
		{
			name: "OAuth method",
			setupHelper: func(h *AuthHelper) {
				h.Config.OAuthCredentials = []*types.OAuthCredentialSet{
					{ID: "cred1", AccessToken: "token1"},
				}
				h.SetupOAuth(nil)
			},
			expected: "oauth",
		},
		{
			name: "API key method",
			setupHelper: func(h *AuthHelper) {
				h.Config.APIKey = "test-key"
				h.SetupAPIKeys()
			},
			expected: "api_key",
		},
		{
			name:        "no method",
			setupHelper: func(h *AuthHelper) {},
			expected:    "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper := NewAuthHelper("test", types.ProviderConfig{}, &http.Client{})
			tt.setupHelper(helper)

			result := helper.GetAuthMethod()
			if result != tt.expected {
				t.Errorf("got %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestAuthHelper_SetAuthHeaders(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		authToken    string
		authType     string
		expectedKey  string
		expectedVal  string
	}{
		{
			name:         "OAuth/Bearer for OpenAI",
			providerName: "openai",
			authToken:    "token123",
			authType:     "oauth",
			expectedKey:  "Authorization",
			expectedVal:  "Bearer token123",
		},
		{
			name:         "API key for Anthropic",
			providerName: "anthropic",
			authToken:    "sk-ant-123",
			authType:     "api_key",
			expectedKey:  "x-api-key",
			expectedVal:  "sk-ant-123",
		},
		{
			name:         "API key for Gemini",
			providerName: "gemini",
			authToken:    "gemini-key",
			authType:     "api_key",
			expectedKey:  "x-goog-api-key",
			expectedVal:  "gemini-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper := NewAuthHelper(tt.providerName, types.ProviderConfig{}, &http.Client{})
			req, _ := http.NewRequest("GET", "http://example.com", nil)

			helper.SetAuthHeaders(req, tt.authToken, tt.authType)

			value := req.Header.Get(tt.expectedKey)
			if value != tt.expectedVal {
				t.Errorf("got %q, expected %q", value, tt.expectedVal)
			}
		})
	}
}

func TestAuthHelper_SetProviderSpecificHeaders(t *testing.T) {
	tests := []struct {
		name            string
		providerName    string
		config          types.ProviderConfig
		authMethod      string
		expectedHeaders map[string]string
	}{
		{
			name:         "Anthropic standard headers",
			providerName: "anthropic",
			config:       types.ProviderConfig{},
			authMethod:   "api_key",
			expectedHeaders: map[string]string{
				"anthropic-version": "2023-06-01",
			},
		},
		{
			name:         "Anthropic OAuth headers",
			providerName: "anthropic",
			config: types.ProviderConfig{
				OAuthCredentials: []*types.OAuthCredentialSet{{ID: "test"}},
			},
			authMethod: "oauth",
			expectedHeaders: map[string]string{
				"anthropic-version": "2023-06-01",
				"anthropic-beta":    "oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14",
			},
		},
		{
			name:         "OpenAI with organization",
			providerName: "openai",
			config: types.ProviderConfig{
				ProviderConfig: map[string]interface{}{
					"organization_id": "org-123",
				},
			},
			authMethod: "api_key",
			expectedHeaders: map[string]string{
				"openai-organization": "org-123",
			},
		},
		{
			name:         "OpenRouter with site info",
			providerName: "openrouter",
			config: types.ProviderConfig{
				ProviderConfig: map[string]interface{}{
					"site_url":  "https://example.com",
					"site_name": "My App",
				},
			},
			authMethod: "api_key",
			expectedHeaders: map[string]string{
				"HTTP-Referer": "https://example.com",
				"X-Title":      "My App",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper := NewAuthHelper(tt.providerName, tt.config, &http.Client{})
			if tt.authMethod == "oauth" {
				helper.SetupOAuth(nil)
			}

			req, _ := http.NewRequest("GET", "http://example.com", nil)
			helper.SetProviderSpecificHeaders(req)

			for key, expectedVal := range tt.expectedHeaders {
				actualVal := req.Header.Get(key)
				if actualVal != expectedVal {
					t.Errorf("header %q: got %q, expected %q", key, actualVal, expectedVal)
				}
			}
		})
	}
}

func TestAuthHelper_HandleAuthError(t *testing.T) {
	helper := NewAuthHelper("test", types.ProviderConfig{}, &http.Client{})

	tests := []struct {
		name       string
		err        error
		statusCode int
		expectNil  bool
		contains   string
	}{
		{
			name:      "nil error",
			err:       nil,
			expectNil: true,
		},
		{
			name:       "invalid API key error",
			err:        http.ErrNotSupported, // Just a placeholder
			statusCode: 401,
			contains:   "authentication failed",
		},
		{
			name:       "403 forbidden",
			err:        http.ErrNotSupported,
			statusCode: 403,
			contains:   "access forbidden",
		},
		{
			name:       "429 rate limit",
			err:        http.ErrNotSupported,
			statusCode: 429,
			contains:   "rate limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := helper.HandleAuthError(tt.err, tt.statusCode)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected error but got nil")
			}
		})
	}
}

func TestAuthHelper_ClearAuthentication(t *testing.T) {
	config := types.ProviderConfig{
		APIKey: "test-key",
		OAuthCredentials: []*types.OAuthCredentialSet{
			{ID: "cred1", AccessToken: "token1"},
		},
	}

	helper := NewAuthHelper("test", config, &http.Client{})
	helper.SetupAPIKeys()
	helper.SetupOAuth(nil)

	// Verify setup
	if helper.KeyManager == nil || helper.OAuthManager == nil {
		t.Fatal("auth managers should be initialized")
	}

	// Clear
	helper.ClearAuthentication()

	// Verify cleared
	if helper.KeyManager != nil {
		t.Error("KeyManager should be nil after clear")
	}
	if helper.OAuthManager != nil {
		t.Error("OAuthManager should be nil after clear")
	}
}

func TestAuthHelper_ValidateAuthConfig(t *testing.T) {
	helper := NewAuthHelper("test", types.ProviderConfig{}, &http.Client{})

	tests := []struct {
		name        string
		authConfig  types.AuthConfig
		expectError bool
	}{
		{
			name: "valid API key",
			authConfig: types.AuthConfig{
				Method: types.AuthMethodAPIKey,
				APIKey: "test-key",
			},
			expectError: false,
		},
		{
			name: "missing API key",
			authConfig: types.AuthConfig{
				Method: types.AuthMethodAPIKey,
			},
			expectError: true,
		},
		{
			name: "valid bearer token",
			authConfig: types.AuthConfig{
				Method: types.AuthMethodBearerToken,
				APIKey: "token",
			},
			expectError: false,
		},
		{
			name: "unsupported method",
			authConfig: types.AuthConfig{
				Method: "unsupported",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.ValidateAuthConfig(tt.authConfig)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestAuthHelper_GetAuthStatus(t *testing.T) {
	config := types.ProviderConfig{
		APIKey: "test-key",
	}

	helper := NewAuthHelper("test", config, &http.Client{})
	helper.SetupAPIKeys()

	status := helper.GetAuthStatus()

	if status["provider"] != "test" {
		t.Errorf("got provider %v, expected %q", status["provider"], "test")
	}

	if !status["authenticated"].(bool) {
		t.Error("expected authenticated to be true")
	}

	if status["method"] != "api_key" {
		t.Errorf("got method %v, expected %q", status["method"], "api_key")
	}

	if status["api_keys_configured"].(int) != 1 {
		t.Errorf("got %v api keys, expected 1", status["api_keys_configured"])
	}
}

func TestAuthHelper_RefreshAllOAuthTokens(t *testing.T) {
	helper := NewAuthHelper("test", types.ProviderConfig{}, &http.Client{})

	// No OAuth manager
	err := helper.RefreshAllOAuthTokens(context.Background())
	if err == nil {
		t.Error("expected error when no OAuth manager")
	}
}
