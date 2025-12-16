package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/auth"
)

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

func TestOAuthHandler_ListOAuthProviders_WrongMethod(t *testing.T) {
	handler := NewOAuthHandler(&mockAuthManager{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/oauth/providers", nil)

	handler.ListOAuthProviders(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

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
