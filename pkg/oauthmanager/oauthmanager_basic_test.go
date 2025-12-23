package oauthmanager

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestNewOAuthKeyManager tests basic creation of OAuth key manager
func TestNewOAuthKeyManager(t *testing.T) {
	tests := []struct {
		name        string
		credentials []*types.OAuthCredentialSet
		wantNil     bool
	}{
		{
			name: "valid credentials",
			credentials: []*types.OAuthCredentialSet{
				{
					ID:           "test-1",
					ClientID:     "client-1",
					ClientSecret: "secret-1",
					AccessToken:  "token-1",
					ExpiresAt:    time.Now().Add(1 * time.Hour),
				},
			},
			wantNil: false,
		},
		{
			name:        "no credentials",
			credentials: []*types.OAuthCredentialSet{},
			wantNil:     true,
		},
		{
			name:        "nil credentials",
			credentials: nil,
			wantNil:     true,
		},
		{
			name: "multiple credentials",
			credentials: []*types.OAuthCredentialSet{
				{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
				{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2"},
				{ID: "cred-3", ClientID: "client-3", AccessToken: "token-3"},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewOAuthKeyManager("TestProvider", tt.credentials, nil)

			if tt.wantNil {
				if manager != nil {
					t.Errorf("NewOAuthKeyManager() expected nil, got %v", manager)
				}
				return
			}

			if manager == nil {
				t.Fatal("NewOAuthKeyManager() returned nil, expected manager")
			}

			if manager.providerName != "TestProvider" {
				t.Errorf("providerName = %v, want TestProvider", manager.providerName)
			}

			if len(manager.credentials) != len(tt.credentials) {
				t.Errorf("credentials length = %v, want %v", len(manager.credentials), len(tt.credentials))
			}

			if len(manager.credHealth) != len(tt.credentials) {
				t.Errorf("credHealth length = %v, want %v", len(manager.credHealth), len(tt.credentials))
			}

			// Verify all credentials have health tracking initialized
			for _, cred := range tt.credentials {
				health, exists := manager.credHealth[cred.ID]
				if !exists {
					t.Errorf("health tracking not initialized for credential %s", cred.ID)
				}
				if !health.isHealthy {
					t.Errorf("credential %s not marked as healthy initially", cred.ID)
				}
			}
		})
	}
}

// TestOAuthKeyManager_GetNextCredential tests round-robin behavior
func TestOAuthKeyManager_GetNextCredential(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2"},
		{ID: "cred-3", ClientID: "client-3", AccessToken: "token-3"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)
	if manager == nil {
		t.Fatal("NewOAuthKeyManager() returned nil")
	}

	ctx := context.Background()

	// Test round-robin distribution
	seen := make(map[string]int)
	for i := 0; i < 9; i++ {
		cred, err := manager.GetNextCredential(ctx)
		if err != nil {
			t.Fatalf("GetNextCredential() error = %v", err)
		}
		seen[cred.ID]++
	}

	// Each credential should be selected exactly 3 times (9 requests / 3 credentials)
	for id, count := range seen {
		if count != 3 {
			t.Errorf("credential %s selected %d times, expected 3", id, count)
		}
	}
}

// TestOAuthKeyManager_GetNextCredential_SingleCredential tests single credential behavior
func TestOAuthKeyManager_GetNextCredential_SingleCredential(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "only-cred", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)
	ctx := context.Background()

	// Should always return the same credential
	for i := 0; i < 5; i++ {
		cred, err := manager.GetNextCredential(ctx)
		if err != nil {
			t.Fatalf("GetNextCredential() error = %v", err)
		}
		if cred.ID != "only-cred" {
			t.Errorf("GetNextCredential() = %v, want only-cred", cred.ID)
		}
	}
}

// TestOAuthKeyManager_GetNextCredential_AllInBackoff tests behavior when all credentials are in backoff
func TestOAuthKeyManager_GetNextCredential_AllInBackoff(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)
	ctx := context.Background()

	// Put all credentials in backoff
	for _, cred := range credentials {
		manager.ReportFailure(cred.ID, errors.New("test failure"))
	}

	// Should return error when all credentials unavailable
	_, err := manager.GetNextCredential(ctx)
	if err == nil {
		t.Error("GetNextCredential() expected error when all credentials in backoff")
	}
}

// TestOAuthKeyManager_GetCredentials tests getting credential copies
func TestOAuthKeyManager_GetCredentials(t *testing.T) {
	original := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1", Scopes: []string{"scope1"}},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2", Scopes: []string{"scope2"}},
	}

	manager := NewOAuthKeyManager("TestProvider", original, nil)

	copies := manager.GetCredentials()

	if len(copies) != len(original) {
		t.Fatalf("GetCredentials() returned %d credentials, want %d", len(copies), len(original))
	}

	// Verify copies are deep copies
	copies[0].AccessToken = "modified"
	copies[0].Scopes[0] = "modified-scope"

	// Original should be unchanged
	if manager.credentials[0].AccessToken == "modified" {
		t.Error("modifying copy affected original credential")
	}
	if manager.credentials[0].Scopes[0] == "modified-scope" {
		t.Error("modifying copy scopes affected original credential")
	}
}

// TestCredentialSet_IsExpired tests token expiration checking
func TestCredentialSet_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "valid token with 1 hour remaining",
			expiresAt: time.Now().Add(1 * time.Hour),
			want:      false,
		},
		{
			name:      "expired token",
			expiresAt: time.Now().Add(-1 * time.Hour),
			want:      true,
		},
		{
			name:      "token expiring in 3 minutes (within buffer)",
			expiresAt: time.Now().Add(3 * time.Minute),
			want:      true,
		},
		{
			name:      "token expiring in 10 minutes (outside buffer)",
			expiresAt: time.Now().Add(10 * time.Minute),
			want:      false,
		},
		{
			name:      "zero time (no expiry set)",
			expiresAt: time.Time{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := &types.OAuthCredentialSet{
				ID:        "test",
				ExpiresAt: tt.expiresAt,
			}

			if got := IsExpired(cred); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}

	// Test nil credential
	t.Run("nil credential", func(t *testing.T) {
		if got := IsExpired(nil); got != true {
			t.Errorf("IsExpired(nil) = %v, want true", got)
		}
	})
}

// TestCredentialSet_Clone tests credential cloning
func TestCredentialSet_Clone(t *testing.T) {
	original := &types.OAuthCredentialSet{
		ID:           "test-1",
		ClientID:     "client-1",
		ClientSecret: "secret-1",
		AccessToken:  "token-1",
		RefreshToken: "refresh-1",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		Scopes:       []string{"scope1", "scope2"},
		LastRefresh:  time.Now(),
		RefreshCount: 5,
		OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
			return nil
		},
	}

	clone := Clone(original)

	// Verify all fields are copied
	if clone.ID != original.ID {
		t.Errorf("clone.ID = %v, want %v", clone.ID, original.ID)
	}
	if clone.ClientID != original.ClientID {
		t.Errorf("clone.ClientID = %v, want %v", clone.ClientID, original.ClientID)
	}
	if clone.AccessToken != original.AccessToken {
		t.Errorf("clone.AccessToken = %v, want %v", clone.AccessToken, original.AccessToken)
	}
	if clone.RefreshCount != original.RefreshCount {
		t.Errorf("clone.RefreshCount = %v, want %v", clone.RefreshCount, original.RefreshCount)
	}

	// Verify scopes are deep copied
	if len(clone.Scopes) != len(original.Scopes) {
		t.Fatalf("clone.Scopes length = %v, want %v", len(clone.Scopes), len(original.Scopes))
	}
	clone.Scopes[0] = "modified"
	if original.Scopes[0] == "modified" {
		t.Error("modifying clone scopes affected original")
	}

	// Test nil clone
	var nilCred *types.OAuthCredentialSet
	nilClone := Clone(nilCred)
	if nilClone != nil {
		t.Errorf("Clone() of nil = %v, want nil", nilClone)
	}
}

// TestNeedsRefresh tests the NeedsRefresh helper function
func TestNeedsRefresh(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "token with 1 hour remaining",
			expiresAt: time.Now().Add(1 * time.Hour),
			want:      false,
		},
		{
			name:      "expired token",
			expiresAt: time.Now().Add(-1 * time.Hour),
			want:      true,
		},
		{
			name:      "token expiring in 3 minutes (within buffer)",
			expiresAt: time.Now().Add(3 * time.Minute),
			want:      true,
		},
		{
			name:      "token expiring in 10 minutes (outside buffer)",
			expiresAt: time.Now().Add(10 * time.Minute),
			want:      false,
		},
		{
			name:      "token expiring at 6 minutes (just outside buffer)",
			expiresAt: time.Now().Add(6 * time.Minute),
			want:      false, // Just outside the 5-minute buffer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := &types.OAuthCredentialSet{
				ID:        "test",
				ExpiresAt: tt.expiresAt,
			}

			if got := NeedsRefresh(cred); got != tt.want {
				t.Errorf("NeedsRefresh() = %v, want %v", got, tt.want)
			}
		})
	}

	// Test nil credential
	t.Run("nil credential", func(t *testing.T) {
		if got := NeedsRefresh(nil); got != false {
			t.Errorf("NeedsRefresh(nil) = %v, want false", got)
		}
	})

	// Test zero time (no expiry set)
	t.Run("zero time", func(t *testing.T) {
		cred := &types.OAuthCredentialSet{
			ID:        "test",
			ExpiresAt: time.Time{},
		}
		if got := NeedsRefresh(cred); got != false {
			t.Errorf("NeedsRefresh(zero time) = %v, want false", got)
		}
	})
}

// TestNoOpRefreshFunc tests the no-op refresh function
func TestNoOpRefreshFunc(t *testing.T) {
	cred := &types.OAuthCredentialSet{
		ID:           "test",
		ClientID:     "client",
		AccessToken:  "token",
		RefreshToken: "refresh",
	}

	ctx := context.Background()
	_, err := NoOpRefreshFunc(ctx, cred)
	if err == nil {
		t.Error("NoOpRefreshFunc() expected error, got nil")
	}
}

// Example test demonstrating usage
func ExampleOAuthKeyManager() {
	// Create credentials
	credentials := []*types.OAuthCredentialSet{
		{
			ID:           "team-account",
			ClientID:     "client-id-1",
			ClientSecret: "client-secret-1",
			AccessToken:  "access-token-1",
			RefreshToken: "refresh-token-1",
			ExpiresAt:    time.Now().Add(1 * time.Hour),
		},
		{
			ID:           "personal-account",
			ClientID:     "client-id-2",
			ClientSecret: "client-secret-2",
			AccessToken:  "access-token-2",
			RefreshToken: "refresh-token-2",
			ExpiresAt:    time.Now().Add(2 * time.Hour),
		},
	}

	// Create manager
	manager := NewOAuthKeyManager("Gemini", credentials, nil)

	// Use in operation with automatic failover
	ctx := context.Background()
	result, usage, err := manager.ExecuteWithFailover(ctx,
		func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
			// Make API call with cred.AccessToken
			return fmt.Sprintf("API response using %s", cred.ID), &types.Usage{TotalTokens: 100}, nil
		})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Result: %s, Tokens: %d\n", result, usage.TotalTokens)
}
