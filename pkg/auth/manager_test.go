package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewAuthManager(t *testing.T) {
	t.Run("WithNilConfig", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		manager := NewAuthManager(storage, nil)
		if manager == nil {
			t.Fatal("Expected non-nil manager")
		}
		if manager.config == nil {
			t.Fatal("Expected default config to be set")
		}
	})

	t.Run("WithValidConfig", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		config := DefaultConfig()
		config.TokenStorage.File.Backup.Enabled = false // Disable ticker for tests
		manager := NewAuthManager(storage, config)
		if manager == nil {
			t.Fatal("Expected non-nil manager")
		}
		if manager.config != config {
			t.Error("Expected config to be set")
		}
	})
}

func TestAuthManagerBuilder(t *testing.T) {
	t.Run("BuildWithDefaults", func(t *testing.T) {
		builder := NewAuthManagerBuilder()
		manager, err := builder.Build()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if manager == nil {
			t.Error("Expected non-nil manager")
		}
	})

	t.Run("BuildWithCustomStorage", func(t *testing.T) {
		storage := NewMemoryTokenStorage(nil)
		builder := NewAuthManagerBuilder().WithStorage(storage)
		manager, err := builder.Build()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if manager.storage != storage {
			t.Error("Expected custom storage")
		}
	})

	t.Run("BuildWithLogger", func(t *testing.T) {
		logger := &TestLogger{}
		builder := NewAuthManagerBuilder().WithLogger(logger)
		manager, err := builder.Build()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if manager.logger != logger {
			t.Error("Expected custom logger")
		}
	})

	t.Run("BuildWithAuthenticators", func(t *testing.T) {
		auth := &MockAuthenticator{}
		builder := NewAuthManagerBuilder().WithAuthenticator("test", auth)
		manager, err := builder.Build()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		retrieved, err := manager.GetAuthenticator("test")
		if err != nil {
			t.Errorf("Expected to retrieve authenticator, got: %v", err)
		}
		if retrieved != auth {
			t.Error("Expected registered authenticator")
		}
	})
}

func TestRegisterAuthenticator(t *testing.T) {
	manager := NewAuthManager(NewMemoryTokenStorage(nil), nil)

	t.Run("Success", func(t *testing.T) {
		auth := &MockAuthenticator{}
		err := manager.RegisterAuthenticator("test", auth)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("EmptyProvider", func(t *testing.T) {
		auth := &MockAuthenticator{}
		err := manager.RegisterAuthenticator("", auth)
		if err == nil {
			t.Error("Expected error for empty provider")
		}
	})

	t.Run("NilAuthenticator", func(t *testing.T) {
		err := manager.RegisterAuthenticator("test", nil)
		if err == nil {
			t.Error("Expected error for nil authenticator")
		}
	})
}

func TestGetAuthenticator(t *testing.T) {
	manager := NewAuthManager(NewMemoryTokenStorage(nil), nil)

	t.Run("ExistingProvider", func(t *testing.T) {
		auth := &MockAuthenticator{}
		_ = manager.RegisterAuthenticator("test", auth)

		retrieved, err := manager.GetAuthenticator("test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if retrieved != auth {
			t.Error("Expected registered authenticator")
		}
	})

	t.Run("NonExistingProvider", func(t *testing.T) {
		_, err := manager.GetAuthenticator("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent provider")
		}
	})
}

func TestAuthenticate(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	manager := NewAuthManager(storage, nil)

	t.Run("Success", func(t *testing.T) {
		auth := &MockAuthenticator{}
		_ = manager.RegisterAuthenticator("test", auth)

		config := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: "test-key",
		}

		ctx := context.Background()
		err := manager.Authenticate(ctx, "test", config)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !auth.authenticated {
			t.Error("Expected authenticator to be authenticated")
		}
	})

	t.Run("ProviderNotRegistered", func(t *testing.T) {
		ctx := context.Background()
		config := types.AuthConfig{
			Method: types.AuthMethodAPIKey,
			APIKey: "test-key",
		}

		err := manager.Authenticate(ctx, "nonexistent", config)
		if err == nil {
			t.Error("Expected error for nonexistent provider")
		}
	})
}

func TestIsAuthenticated(t *testing.T) {
	manager := NewAuthManager(NewMemoryTokenStorage(nil), nil)

	t.Run("AuthenticatedProvider", func(t *testing.T) {
		auth := &MockAuthenticator{authenticated: true}
		_ = manager.RegisterAuthenticator("test", auth)

		if !manager.IsAuthenticated("test") {
			t.Error("Expected provider to be authenticated")
		}
	})

	t.Run("NotAuthenticatedProvider", func(t *testing.T) {
		auth := &MockAuthenticator{authenticated: false}
		_ = manager.RegisterAuthenticator("test2", auth)

		if manager.IsAuthenticated("test2") {
			t.Error("Expected provider to not be authenticated")
		}
	})

	t.Run("NonExistentProvider", func(t *testing.T) {
		if manager.IsAuthenticated("nonexistent") {
			t.Error("Expected nonexistent provider to not be authenticated")
		}
	})
}

func TestLogout(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	manager := NewAuthManager(storage, nil)

	t.Run("Success", func(t *testing.T) {
		auth := &MockAuthenticator{authenticated: true}
		_ = manager.RegisterAuthenticator("test", auth)

		ctx := context.Background()
		err := manager.Logout(ctx, "test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if auth.authenticated {
			t.Error("Expected authenticator to be logged out")
		}
	})

	t.Run("ProviderNotRegistered", func(t *testing.T) {
		ctx := context.Background()
		err := manager.Logout(ctx, "nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent provider")
		}
	})
}

func TestRefreshAllTokens(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	manager := NewAuthManager(storage, nil)

	t.Run("NoAuthenticators", func(t *testing.T) {
		ctx := context.Background()
		err := manager.RefreshAllTokens(ctx)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("MultipleAuthenticators", func(t *testing.T) {
		auth1 := &MockAuthenticator{authenticated: true}
		auth2 := &MockAuthenticator{authenticated: true}
		auth3 := &MockAuthenticator{authenticated: false}

		_ = manager.RegisterAuthenticator("test1", auth1)
		_ = manager.RegisterAuthenticator("test2", auth2)
		_ = manager.RegisterAuthenticator("test3", auth3)

		ctx := context.Background()
		err := manager.RefreshAllTokens(ctx)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("WithRefreshError", func(t *testing.T) {
		auth := &MockAuthenticator{
			authenticated: true,
			refreshError:  errors.New("refresh failed"),
		}
		manager2 := NewAuthManager(storage, nil)
		_ = manager2.RegisterAuthenticator("failing", auth)

		ctx := context.Background()
		err := manager2.RefreshAllTokens(ctx)
		if err == nil {
			t.Error("Expected error from failed refresh")
		}
	})
}

func TestGetAuthenticatedProviders(t *testing.T) {
	manager := NewAuthManager(NewMemoryTokenStorage(nil), nil)

	t.Run("NoProviders", func(t *testing.T) {
		providers := manager.GetAuthenticatedProviders()
		if len(providers) != 0 {
			t.Errorf("Expected 0 providers, got %d", len(providers))
		}
	})

	t.Run("MixedProviders", func(t *testing.T) {
		auth1 := &MockAuthenticator{authenticated: true}
		auth2 := &MockAuthenticator{authenticated: false}
		auth3 := &MockAuthenticator{authenticated: true}

		_ = manager.RegisterAuthenticator("auth1", auth1)
		_ = manager.RegisterAuthenticator("auth2", auth2)
		_ = manager.RegisterAuthenticator("auth3", auth3)

		providers := manager.GetAuthenticatedProviders()
		if len(providers) != 2 {
			t.Errorf("Expected 2 authenticated providers, got %d", len(providers))
		}
	})
}

func TestGetAuthStatus(t *testing.T) {
	manager := NewAuthManager(NewMemoryTokenStorage(nil), nil)

	auth1 := &MockAuthenticator{authenticated: true, authMethod: types.AuthMethodAPIKey}
	auth2 := &MockAuthenticator{authenticated: false, authMethod: types.AuthMethodOAuth}

	_ = manager.RegisterAuthenticator("test1", auth1)
	_ = manager.RegisterAuthenticator("test2", auth2)

	status := manager.GetAuthStatus()

	if len(status) != 2 {
		t.Errorf("Expected 2 status entries, got %d", len(status))
	}

	if status["test1"] == nil {
		t.Error("Expected status for test1")
	}
	if !status["test1"].Authenticated {
		t.Error("Expected test1 to be authenticated")
	}
	if status["test1"].Method != types.AuthMethodAPIKey {
		t.Error("Expected test1 method to be API key")
	}

	if status["test2"] == nil {
		t.Error("Expected status for test2")
	}
	if status["test2"].Authenticated {
		t.Error("Expected test2 to not be authenticated")
	}
}

func TestForEachAuthenticated(t *testing.T) {
	manager := NewAuthManager(NewMemoryTokenStorage(nil), nil)

	t.Run("Success", func(t *testing.T) {
		auth1 := &MockAuthenticator{authenticated: true}
		auth2 := &MockAuthenticator{authenticated: false}
		auth3 := &MockAuthenticator{authenticated: true}

		_ = manager.RegisterAuthenticator("test1", auth1)
		_ = manager.RegisterAuthenticator("test2", auth2)
		_ = manager.RegisterAuthenticator("test3", auth3)

		count := 0
		ctx := context.Background()
		err := manager.ForEachAuthenticated(ctx, func(provider string, auth Authenticator) error {
			count++
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected 2 calls, got %d", count)
		}
	})

	t.Run("WithError", func(t *testing.T) {
		auth := &MockAuthenticator{authenticated: true}
		manager2 := NewAuthManager(NewMemoryTokenStorage(nil), nil)
		_ = manager2.RegisterAuthenticator("test", auth)

		ctx := context.Background()
		err := manager2.ForEachAuthenticated(ctx, func(provider string, auth Authenticator) error {
			return errors.New("test error")
		})

		if err == nil {
			t.Error("Expected error to be propagated")
		}
	})
}

func TestGetTokenInfo(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := DefaultConfig()
	manager := NewAuthManager(storage, config)

	t.Run("OAuthAuthenticator", func(t *testing.T) {
		oauthAuth := NewOAuthAuthenticator("test", storage, &config.OAuth)
		oauthAuth.config = &types.OAuthConfig{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		}
		oauthAuth.isAuth = true

		_ = manager.RegisterAuthenticator("oauth", oauthAuth)

		tokenInfo, err := manager.GetTokenInfo("oauth")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if tokenInfo == nil {
			t.Error("Expected token info")
		}
	})

	t.Run("NonOAuthAuthenticator", func(t *testing.T) {
		auth := &MockAuthenticator{authenticated: true}
		_ = manager.RegisterAuthenticator("apikey", auth)

		_, err := manager.GetTokenInfo("apikey")
		if err == nil {
			t.Error("Expected error for non-OAuth authenticator")
		}
	})

	t.Run("NonExistentProvider", func(t *testing.T) {
		_, err := manager.GetTokenInfo("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent provider")
		}
	})
}

func TestStartOAuthFlow(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := DefaultConfig()
	manager := NewAuthManager(storage, config)

	t.Run("Success", func(t *testing.T) {
		oauthAuth := NewOAuthAuthenticator("test", storage, &config.OAuth)
		oauthAuth.config = &types.OAuthConfig{
			ClientID:    "test-client-id",
			AuthURL:     "https://example.com/auth",
			RedirectURL: "https://example.com/callback",
		}

		_ = manager.RegisterAuthenticator("oauth", oauthAuth)

		ctx := context.Background()
		authURL, err := manager.StartOAuthFlow(ctx, "oauth", []string{"scope1"})
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if authURL == "" {
			t.Error("Expected auth URL")
		}
	})

	t.Run("NonOAuthAuthenticator", func(t *testing.T) {
		auth := &MockAuthenticator{}
		_ = manager.RegisterAuthenticator("apikey", auth)

		ctx := context.Background()
		_, err := manager.StartOAuthFlow(ctx, "apikey", []string{"scope1"})
		if err == nil {
			t.Error("Expected error for non-OAuth authenticator")
		}
	})
}

func TestHandleOAuthCallback(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := DefaultConfig()
	manager := NewAuthManager(storage, config)

	t.Run("NonOAuthAuthenticator", func(t *testing.T) {
		auth := &MockAuthenticator{}
		_ = manager.RegisterAuthenticator("apikey", auth)

		ctx := context.Background()
		err := manager.HandleOAuthCallback(ctx, "apikey", "code", "state")
		if err == nil {
			t.Error("Expected error for non-OAuth authenticator")
		}
	})
}

func TestGetProviders(t *testing.T) {
	manager := NewAuthManager(NewMemoryTokenStorage(nil), nil)

	t.Run("NoProviders", func(t *testing.T) {
		providers := manager.GetProviders()
		if len(providers) != 0 {
			t.Errorf("Expected 0 providers, got %d", len(providers))
		}
	})

	t.Run("MultipleProviders", func(t *testing.T) {
		_ = manager.RegisterAuthenticator("test1", &MockAuthenticator{})
		_ = manager.RegisterAuthenticator("test2", &MockAuthenticator{})
		_ = manager.RegisterAuthenticator("test3", &MockAuthenticator{})

		providers := manager.GetProviders()
		if len(providers) != 3 {
			t.Errorf("Expected 3 providers, got %d", len(providers))
		}
	})
}

func TestRemoveAuthenticator(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	manager := NewAuthManager(storage, nil)

	t.Run("Success", func(t *testing.T) {
		auth := &MockAuthenticator{authenticated: true}
		_ = manager.RegisterAuthenticator("test", auth)

		err := manager.RemoveAuthenticator("test")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		_, err = manager.GetAuthenticator("test")
		if err == nil {
			t.Error("Expected error for removed authenticator")
		}
	})

	t.Run("NonExistentProvider", func(t *testing.T) {
		err := manager.RemoveAuthenticator("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent provider")
		}
	})
}

func TestClose(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := DefaultConfig()
	config.TokenStorage.File.Backup.Enabled = false
	manager := NewAuthManager(storage, config)

	auth1 := &MockAuthenticator{authenticated: true}
	auth2 := &MockAuthenticator{authenticated: true}

	_ = manager.RegisterAuthenticator("test1", auth1)
	_ = manager.RegisterAuthenticator("test2", auth2)

	err := manager.Close()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if auth1.authenticated {
		t.Error("Expected auth1 to be logged out")
	}
	if auth2.authenticated {
		t.Error("Expected auth2 to be logged out")
	}
}

func TestGetStorage(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	manager := NewAuthManager(storage, nil)

	if manager.GetStorage() != storage {
		t.Error("Expected same storage instance")
	}
}

func TestGetConfig(t *testing.T) {
	storage := NewMemoryTokenStorage(nil)
	config := DefaultConfig()
	manager := NewAuthManager(storage, config)

	if manager.GetConfig() != config {
		t.Error("Expected same config instance")
	}
}

func TestSetLogger(t *testing.T) {
	manager := NewAuthManager(NewMemoryTokenStorage(nil), nil)
	logger := &TestLogger{}

	manager.SetLogger(logger)

	if manager.logger != logger {
		t.Error("Expected logger to be set")
	}
}
