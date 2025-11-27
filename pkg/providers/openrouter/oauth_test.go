package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestGeneratePKCEParams(t *testing.T) {
	helper := NewOAuthHelper("https://openrouter.ai")

	pkce, err := helper.GeneratePKCEParams()
	if err != nil {
		t.Fatalf("Failed to generate PKCE params: %v", err)
	}

	if pkce.CodeVerifier == "" {
		t.Error("Expected non-empty code verifier")
	}

	if pkce.CodeChallenge == "" {
		t.Error("Expected non-empty code challenge")
	}

	if pkce.Method != "S256" {
		t.Errorf("Expected method 'S256', got '%s'", pkce.Method)
	}

	// Verify verifier and challenge are different
	if pkce.CodeVerifier == pkce.CodeChallenge {
		t.Error("Code verifier and challenge should be different")
	}

	// Verify that generating multiple times produces different values
	pkce2, err := helper.GeneratePKCEParams()
	if err != nil {
		t.Fatalf("Failed to generate second PKCE params: %v", err)
	}

	if pkce.CodeVerifier == pkce2.CodeVerifier {
		t.Error("Multiple generations should produce different verifiers")
	}
}

func TestBuildAuthURL(t *testing.T) {
	helper := NewOAuthHelper("https://openrouter.ai")

	pkce, err := helper.GeneratePKCEParams()
	if err != nil {
		t.Fatalf("Failed to generate PKCE params: %v", err)
	}

	callbackURL := "https://example.com/callback"
	authURL, err := helper.BuildAuthURL(callbackURL, pkce)
	if err != nil {
		t.Fatalf("Failed to build auth URL: %v", err)
	}

	// Verify URL structure
	if !strings.HasPrefix(authURL, "https://openrouter.ai/auth?") {
		t.Errorf("Auth URL should start with base URL, got: %s", authURL)
	}

	if !strings.Contains(authURL, "callback_url="+callbackURL) {
		t.Errorf("Auth URL should contain callback_url parameter, got: %s", authURL)
	}

	if !strings.Contains(authURL, "code_challenge="+pkce.CodeChallenge) {
		t.Errorf("Auth URL should contain code_challenge parameter, got: %s", authURL)
	}

	if !strings.Contains(authURL, "code_challenge_method=S256") {
		t.Errorf("Auth URL should contain code_challenge_method parameter, got: %s", authURL)
	}
}

func TestBuildAuthURLErrors(t *testing.T) {
	helper := NewOAuthHelper("https://openrouter.ai")

	tests := []struct {
		name        string
		callbackURL string
		pkce        *PKCEParams
		expectError bool
	}{
		{
			name:        "Empty callback URL",
			callbackURL: "",
			pkce:        &PKCEParams{CodeVerifier: "test", CodeChallenge: "test", Method: "S256"},
			expectError: true,
		},
		{
			name:        "Nil PKCE params",
			callbackURL: "https://example.com/callback",
			pkce:        nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := helper.BuildAuthURL(tt.callbackURL, tt.pkce)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestExchangeCodeForAPIKey(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Parse request body
		var req TokenExchangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Verify request fields
		if req.Code == "" {
			t.Error("Expected non-empty authorization code")
		}
		if req.CodeVerifier == "" {
			t.Error("Expected non-empty code verifier")
		}
		if req.CodeChallengeMethod != "S256" {
			t.Errorf("Expected code_challenge_method 'S256', got '%s'", req.CodeChallengeMethod)
		}

		// Return success response
		resp := TokenExchangeResponse{
			Key: "sk-or-v1-test-key-123456789",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create helper with test server URL
	helper := NewOAuthHelper(server.URL)
	helper.tokenURL = server.URL + "/api/v1/auth/keys"

	ctx := context.Background()
	apiKey, err := helper.ExchangeCodeForAPIKey(ctx, "test-auth-code", "test-verifier")
	if err != nil {
		t.Fatalf("Failed to exchange code: %v", err)
	}

	if !strings.HasPrefix(apiKey, "sk-or-") {
		t.Errorf("Expected API key to start with 'sk-or-', got: %s", apiKey)
	}
}

func TestExchangeCodeForAPIKeyErrors(t *testing.T) {
	tests := []struct {
		name          string
		authCode      string
		codeVerifier  string
		serverHandler http.HandlerFunc
		expectError   bool
	}{
		{
			name:         "Empty auth code",
			authCode:     "",
			codeVerifier: "test-verifier",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError: true,
		},
		{
			name:         "Empty code verifier",
			authCode:     "test-code",
			codeVerifier: "",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectError: true,
		},
		{
			name:         "Server error",
			authCode:     "test-code",
			codeVerifier: "test-verifier",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal server error"))
			},
			expectError: true,
		},
		{
			name:         "OAuth error response",
			authCode:     "test-code",
			codeVerifier: "test-verifier",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := TokenExchangeResponse{
					Error: "invalid_grant",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			expectError: true,
		},
		{
			name:         "Invalid key format",
			authCode:     "test-code",
			codeVerifier: "test-verifier",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := TokenExchangeResponse{
					Key: "invalid-key-format",
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.serverHandler)
			defer server.Close()

			helper := NewOAuthHelper(server.URL)
			helper.tokenURL = server.URL + "/api/v1/auth/keys"

			ctx := context.Background()
			_, err := helper.ExchangeCodeForAPIKey(ctx, tt.authCode, tt.codeVerifier)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidateCallback(t *testing.T) {
	helper := NewOAuthHelper("https://openrouter.ai")

	tests := []struct {
		name        string
		code        string
		errorParam  string
		expectError bool
	}{
		{
			name:        "Valid callback",
			code:        "auth-code-123",
			errorParam:  "",
			expectError: false,
		},
		{
			name:        "OAuth error",
			code:        "",
			errorParam:  "access_denied",
			expectError: true,
		},
		{
			name:        "Missing code",
			code:        "",
			errorParam:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.ValidateCallback(tt.code, tt.errorParam)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestOAuthFlowState(t *testing.T) {
	state := NewOAuthFlowState("test-verifier", "https://example.com/callback")

	if state.CodeVerifier != "test-verifier" {
		t.Errorf("Expected verifier 'test-verifier', got '%s'", state.CodeVerifier)
	}

	if state.CallbackURL != "https://example.com/callback" {
		t.Errorf("Expected callback URL 'https://example.com/callback', got '%s'", state.CallbackURL)
	}

	if state.IsExpired() {
		t.Error("Newly created state should not be expired")
	}

	// Test validation
	if err := state.Validate(); err != nil {
		t.Errorf("Valid state should pass validation: %v", err)
	}

	// Test expired state
	state.ExpiresAt = time.Now().Add(-1 * time.Minute)
	if !state.IsExpired() {
		t.Error("State with past expiration should be expired")
	}

	if err := state.Validate(); err == nil {
		t.Error("Expired state should fail validation")
	}
}

func TestProviderOAuthIntegration(t *testing.T) {
	// Create a mock server for OAuth endpoints
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/keys" {
			resp := TokenExchangeResponse{
				Key: "sk-or-v1-oauth-test-key",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create provider with OAuth callback URL configured
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "initial-key",
		ProviderConfig: map[string]interface{}{
			"oauth_callback_url": "https://example.com/callback",
		},
	}
	provider := NewOpenRouterProvider(config)
	provider.baseURL = server.URL

	ctx := context.Background()

	// Test StartOAuthFlow
	authURL, flowState, err := provider.StartOAuthFlow(ctx, "")
	if err != nil {
		t.Fatalf("Failed to start OAuth flow: %v", err)
	}

	if authURL == "" {
		t.Error("Expected non-empty auth URL")
	}

	if flowState == nil {
		t.Fatal("Expected non-nil flow state")
	}

	// Test HandleOAuthCallback
	apiKey, err := provider.HandleOAuthCallback(ctx, "test-auth-code", flowState)
	if err != nil {
		t.Fatalf("Failed to handle OAuth callback: %v", err)
	}

	if !strings.HasPrefix(apiKey, "sk-or-") {
		t.Errorf("Expected API key to start with 'sk-or-', got: %s", apiKey)
	}

	// Verify key was added to manager
	maskedKeys := provider.GetAPIKeys()
	if len(maskedKeys) != 2 { // initial-key + oauth-key
		t.Errorf("Expected 2 keys in manager, got %d", len(maskedKeys))
	}
}

func TestAddRemoveAPIKey(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "key-1",
	}
	provider := NewOpenRouterProvider(config)

	// Test adding a key
	err := provider.AddAPIKey("sk-or-v1-new-key")
	if err != nil {
		t.Fatalf("Failed to add API key: %v", err)
	}

	maskedKeys := provider.GetAPIKeys()
	if len(maskedKeys) != 2 {
		t.Errorf("Expected 2 keys after adding, got %d", len(maskedKeys))
	}

	// Test adding duplicate key
	err = provider.AddAPIKey("sk-or-v1-new-key")
	if err != nil {
		t.Errorf("Adding duplicate key should not error: %v", err)
	}

	maskedKeys = provider.GetAPIKeys()
	if len(maskedKeys) != 2 {
		t.Errorf("Expected 2 keys after adding duplicate, got %d", len(maskedKeys))
	}

	// Test removing a key
	err = provider.RemoveAPIKey("sk-or-v1-new-key")
	if err != nil {
		t.Fatalf("Failed to remove API key: %v", err)
	}

	maskedKeys = provider.GetAPIKeys()
	if len(maskedKeys) != 1 {
		t.Errorf("Expected 1 key after removing, got %d", len(maskedKeys))
	}

	// Test removing non-existent key
	err = provider.RemoveAPIKey("non-existent-key")
	if err == nil {
		t.Error("Expected error when removing non-existent key")
	}
}

func TestOAuthFlowWithCustomCallbackURL(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
	}
	provider := NewOpenRouterProvider(config)

	ctx := context.Background()

	// Test with explicit callback URL
	customCallback := "https://custom.example.com/oauth/callback"
	authURL, flowState, err := provider.StartOAuthFlow(ctx, customCallback)
	if err != nil {
		t.Fatalf("Failed to start OAuth flow with custom callback: %v", err)
	}

	if !strings.Contains(authURL, customCallback) {
		t.Errorf("Auth URL should contain custom callback URL, got: %s", authURL)
	}

	if flowState.CallbackURL != customCallback {
		t.Errorf("Flow state should store custom callback URL, got: %s", flowState.CallbackURL)
	}
}

func TestOAuthFlowWithoutCallbackURL(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeOpenRouter,
		APIKey: "test-key",
		// No oauth_callback_url configured
	}
	provider := NewOpenRouterProvider(config)

	ctx := context.Background()

	// Test without callback URL should fail
	_, _, err := provider.StartOAuthFlow(ctx, "")
	if err == nil {
		t.Error("Expected error when starting OAuth flow without callback URL")
	}
}
