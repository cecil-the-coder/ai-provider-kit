package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewOAuthAuthenticator(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{
		DefaultScopes: []string{"read", "write"},
		PKCE: PKCEConfig{
			Enabled: true,
			Method:  "S256",
		},
	}

	auth := NewOAuthAuthenticator("test", storage, config)
	if auth == nil {
		t.Fatal("Expected non-nil authenticator")
	}
	if auth.provider != "test" {
		t.Error("Expected provider to be 'test'")
	}
}

func TestOAuthAuthenticate(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{}

	t.Run("WithStoredToken", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)

		// Store a valid token
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}
		_ = storage.StoreToken("test", token)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodOAuth,
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !auth.isAuth {
			t.Error("Expected authenticator to be authenticated")
		}
	})

	t.Run("WithExpiredToken", func(t *testing.T) {
		storage2 := NewMemoryTokenStorage(nil)
		auth := NewOAuthAuthenticator("test2", storage2, config)

		// Store an expired token
		token := &types.OAuthConfig{
			AccessToken:  "expired-token",
			RefreshToken: "",
			ExpiresAt:    time.Now().Add(-1 * time.Hour),
		}
		_ = storage2.StoreToken("test2", token)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodOAuth,
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err == nil {
			t.Error("Expected error for expired token without refresh")
		}
	})

	t.Run("WrongMethod", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		authConfig := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err == nil {
			t.Error("Expected error for wrong method")
		}
	})

	t.Run("NoConfig", func(t *testing.T) {
		storage3 := NewMemoryTokenStorage(nil)
		auth := NewOAuthAuthenticator("test3", storage3, config)

		authConfig := types.AuthConfig{
			Method: types.AuthMethodOAuth,
		}

		ctx := context.Background()
		err := auth.Authenticate(ctx, authConfig)
		if err == nil {
			t.Error("Expected error for missing OAuth config")
		}
	})
}

func TestOAuthIsAuthenticated(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{}

	t.Run("NotAuthenticated", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		if auth.IsAuthenticated() {
			t.Error("Expected not authenticated")
		}
	})

	t.Run("Authenticated", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}
		auth.isAuth = true

		if !auth.IsAuthenticated() {
			t.Error("Expected authenticated")
		}
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{
			AccessToken: "expired-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour),
		}
		auth.isAuth = true

		if auth.IsAuthenticated() {
			t.Error("Expected not authenticated with expired token")
		}
	})
}

func TestOAuthGetToken(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{}

	t.Run("Success", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}
		auth.isAuth = true

		token, err := auth.GetToken()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if token != "test-token" {
			t.Errorf("Expected 'test-token', got '%s'", token)
		}
	})

	t.Run("NotAuthenticated", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		_, err := auth.GetToken()
		if err == nil {
			t.Error("Expected error for not authenticated")
		}
	})
}

func TestOAuthRefreshToken(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)

	t.Run("NoRefreshToken", func(t *testing.T) {
		config := &OAuthConfig{
			Refresh: RefreshConfig{
				Enabled: true,
			},
		}
		auth := NewOAuthAuthenticator("test", storage, config)

		ctx := context.Background()
		err := auth.RefreshToken(ctx)
		if err == nil {
			t.Error("Expected error for no refresh token")
		}
	})

	t.Run("RefreshDisabled", func(t *testing.T) {
		config := &OAuthConfig{
			Refresh: RefreshConfig{
				Enabled: false,
			},
		}
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{
			RefreshToken: "refresh-token",
		}

		ctx := context.Background()
		err := auth.RefreshToken(ctx)
		if err == nil {
			t.Error("Expected error for disabled refresh")
		}
	})

	t.Run("WithinRefreshBuffer", func(t *testing.T) {
		config := &OAuthConfig{
			Refresh: RefreshConfig{
				Enabled: true,
				Buffer:  10 * time.Minute,
			},
		}
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Now().Add(20 * time.Minute),
		}

		ctx := context.Background()
		err := auth.RefreshToken(ctx)
		if err != nil {
			t.Errorf("Expected no error (within buffer), got: %v", err)
		}
	})
}

func TestOAuthLogout(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{}

	t.Run("Success", func(t *testing.T) {
		// Store a token first
		token := &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}
		_ = storage.StoreToken("test", token)

		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = token
		auth.isAuth = true

		ctx := context.Background()
		err := auth.Logout(ctx)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if auth.isAuth {
			t.Error("Expected isAuth to be false")
		}
		if auth.config != nil {
			t.Error("Expected config to be nil")
		}
		if auth.state != "" {
			t.Error("Expected state to be empty")
		}
	})
}

func TestOAuthGetAuthMethod(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{}
	auth := NewOAuthAuthenticator("test", storage, config)

	if auth.GetAuthMethod() != types.AuthMethodOAuth {
		t.Error("Expected OAuth auth method")
	}
}

func TestOAuthStartOAuthFlow(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{
		DefaultScopes: []string{"default1", "default2"},
		PKCE: PKCEConfig{
			Enabled: true,
			Method:  "S256",
		},
		State: StateConfig{
			Length:           32,
			EnableValidation: true,
		},
	}

	t.Run("Success", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{
			ClientID:    "test-client",
			AuthURL:     "https://example.com/auth",
			RedirectURL: "https://example.com/callback",
		}

		ctx := context.Background()
		authURL, err := auth.StartOAuthFlow(ctx, []string{"custom-scope"})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if authURL == "" {
			t.Error("Expected auth URL")
		}
		if auth.state == "" {
			t.Error("Expected state to be set")
		}
	})

	t.Run("NoConfig", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)

		ctx := context.Background()
		_, err := auth.StartOAuthFlow(ctx, []string{"scope"})
		if err == nil {
			t.Error("Expected error for no config")
		}
	})

	t.Run("UseDefaultScopes", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{
			ClientID:    "test-client",
			AuthURL:     "https://example.com/auth",
			RedirectURL: "https://example.com/callback",
		}

		ctx := context.Background()
		authURL, err := auth.StartOAuthFlow(ctx, nil)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if authURL == "" {
			t.Error("Expected auth URL")
		}
	})
}

func TestOAuthHandleCallback(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)

	t.Run("StateValidationError", func(t *testing.T) {
		config := &OAuthConfig{
			State: StateConfig{
				EnableValidation: true,
			},
		}
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{}
		auth.state = "expected-state"

		ctx := context.Background()
		err := auth.HandleCallback(ctx, "code", "wrong-state")
		if err == nil {
			t.Error("Expected error for invalid state")
		}
	})

	t.Run("NoConfig", func(t *testing.T) {
		config := &OAuthConfig{}
		auth := NewOAuthAuthenticator("test", storage, config)

		ctx := context.Background()
		err := auth.HandleCallback(ctx, "code", "state")
		if err == nil {
			t.Error("Expected error for no config")
		}
	})
}

func TestOAuthIsOAuthEnabled(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{}

	t.Run("Enabled", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{
			ClientID:     "client",
			ClientSecret: "secret",
			AuthURL:      "https://example.com/auth",
			TokenURL:     "https://example.com/token",
		}

		if !auth.IsOAuthEnabled() {
			t.Error("Expected OAuth to be enabled")
		}
	})

	t.Run("Disabled", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		if auth.IsOAuthEnabled() {
			t.Error("Expected OAuth to be disabled")
		}
	})

	t.Run("MissingClientID", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		auth.config = &types.OAuthConfig{
			ClientSecret: "secret",
			AuthURL:      "https://example.com/auth",
			TokenURL:     "https://example.com/token",
		}

		if auth.IsOAuthEnabled() {
			t.Error("Expected OAuth to be disabled without client ID")
		}
	})
}

func TestOAuthGetTokenInfo(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{}

	t.Run("Success", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		expiresAt := time.Now().Add(1 * time.Hour)
		auth.config = &types.OAuthConfig{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    expiresAt,
			Scopes:       []string{"scope1", "scope2"},
		}
		auth.isAuth = true

		tokenInfo, err := auth.GetTokenInfo()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if tokenInfo.AccessToken != "access-token" {
			t.Error("Expected access token to match")
		}
		if tokenInfo.RefreshToken != "refresh-token" {
			t.Error("Expected refresh token to match")
		}
		if tokenInfo.ExpiresAt != expiresAt {
			t.Error("Expected expiry time to match")
		}
		if tokenInfo.IsExpired {
			t.Error("Expected token not to be expired")
		}
	})

	t.Run("NotAuthenticated", func(t *testing.T) {
		auth := NewOAuthAuthenticator("test", storage, config)
		_, err := auth.GetTokenInfo()
		if err == nil {
			t.Error("Expected error for not authenticated")
		}
	})
}

func TestOAuthHelper(t *testing.T) {
	config := &OAuthConfig{
		State: StateConfig{
			EnableValidation: true,
		},
	}
	helper := NewOAuthHelper(config)

	t.Run("ValidateCallback_Success", func(t *testing.T) {
		err := helper.ValidateCallback("state123", "state123", "code123", "")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("ValidateCallback_WithError", func(t *testing.T) {
		err := helper.ValidateCallback("state", "state", "code", "access_denied")
		if err == nil {
			t.Error("Expected error for OAuth error param")
		}
	})

	t.Run("ValidateCallback_StateMismatch", func(t *testing.T) {
		err := helper.ValidateCallback("state1", "state2", "code", "")
		if err == nil {
			t.Error("Expected error for state mismatch")
		}
	})

	t.Run("ValidateCallback_NoCode", func(t *testing.T) {
		err := helper.ValidateCallback("state", "state", "", "")
		if err == nil {
			t.Error("Expected error for missing code")
		}
	})

	t.Run("BuildAuthURL", func(t *testing.T) {
		authURL, err := helper.BuildAuthURL(
			"https://example.com/auth",
			"client-id",
			"https://example.com/callback",
			"state123",
			[]string{"scope1", "scope2"},
			"challenge",
		)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if authURL == "" {
			t.Error("Expected auth URL")
		}
	})

	t.Run("BuildAuthURL_InvalidURL", func(t *testing.T) {
		_, err := helper.BuildAuthURL(
			"not a valid url://",
			"client-id",
			"https://example.com/callback",
			"state123",
			[]string{"scope1"},
			"",
		)
		if err == nil {
			t.Error("Expected error for invalid URL")
		}
	})
}

func TestOAuthTokenExchange(t *testing.T) {
	// Create mock token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"access_token": "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in": 3600,
			"token_type": "Bearer"
		}`))
	}))
	defer server.Close()

	config := &OAuthConfig{
		HTTP: HTTPConfig{
			UserAgent: "test-agent",
		},
	}
	helper := NewOAuthHelper(config)

	ctx := context.Background()
	tokenResp, err := helper.ExchangeCodeForToken(
		ctx,
		server.URL,
		"client-id",
		"client-secret",
		"https://example.com/callback",
		"auth-code",
		"verifier",
	)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if tokenResp == nil {
		t.Fatal("Expected token response")
	}
	if tokenResp.AccessToken != "new-access-token" {
		t.Error("Expected access token to match")
	}
}

func TestOAuthGeneratePKCE(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{
		PKCE: PKCEConfig{
			Enabled:        true,
			Method:         "S256",
			VerifierLength: 128,
		},
	}
	auth := NewOAuthAuthenticator("test", storage, config)

	t.Run("S256Method", func(t *testing.T) {
		verifier, err := auth.generatePKCEVerifier()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if verifier == "" {
			t.Error("Expected non-empty verifier")
		}

		challenge := auth.generatePKCEChallenge(verifier)
		if challenge == "" {
			t.Error("Expected non-empty challenge")
		}
		if challenge == verifier {
			t.Error("Expected challenge to be different from verifier for S256")
		}
	})

	t.Run("PlainMethod", func(t *testing.T) {
		config2 := &OAuthConfig{
			PKCE: PKCEConfig{
				Method: "plain",
			},
		}
		auth2 := NewOAuthAuthenticator("test", storage, config2)

		verifier := "test-verifier"
		challenge := auth2.generatePKCEChallenge(verifier)
		if challenge != verifier {
			t.Error("Expected challenge to equal verifier for plain method")
		}
	})
}

func TestOAuthGenerateRandomState(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{
		State: StateConfig{
			Length: 32,
		},
	}
	auth := NewOAuthAuthenticator("test", storage, config)

	state1, err := auth.generateRandomState()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if state1 == "" {
		t.Error("Expected non-empty state")
	}

	state2, err := auth.generateRandomState()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if state1 == state2 {
		t.Error("Expected different states")
	}
}

func TestOAuthSetOAuthConfig(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{}
	auth := NewOAuthAuthenticator("test", storage, config)

	oauthConfig := &types.OAuthConfig{
		ClientID: "test-client",
	}

	auth.SetOAuthConfig(oauthConfig)

	if auth.config != oauthConfig {
		t.Error("Expected config to be set")
	}
}

func TestOAuthIsTokenExpired(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := &OAuthConfig{
		Refresh: RefreshConfig{
			Enabled: true,
			Buffer:  5 * time.Minute,
		},
	}
	auth := NewOAuthAuthenticator("test", storage, config)

	t.Run("NoExpiration", func(t *testing.T) {
		tokenConfig := &types.OAuthConfig{}
		if auth.isTokenExpired(tokenConfig) {
			t.Error("Expected token not to be expired when no expiration set")
		}
	})

	t.Run("NotExpired", func(t *testing.T) {
		tokenConfig := &types.OAuthConfig{
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		if auth.isTokenExpired(tokenConfig) {
			t.Error("Expected token not to be expired")
		}
	})

	t.Run("ExpiredWithBuffer", func(t *testing.T) {
		tokenConfig := &types.OAuthConfig{
			ExpiresAt: time.Now().Add(3 * time.Minute),
		}
		if !auth.isTokenExpired(tokenConfig) {
			t.Error("Expected token to be considered expired within buffer")
		}
	})

	t.Run("Expired", func(t *testing.T) {
		tokenConfig := &types.OAuthConfig{
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		if !auth.isTokenExpired(tokenConfig) {
			t.Error("Expected token to be expired")
		}
	})
}
