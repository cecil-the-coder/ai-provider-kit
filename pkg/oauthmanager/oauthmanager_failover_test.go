package oauthmanager

import (
	"context"
	"errors"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestOAuthKeyManager_ExecuteWithFailover tests basic failover functionality
func TestOAuthKeyManager_ExecuteWithFailover(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2"},
		{ID: "cred-3", ClientID: "client-3", AccessToken: "token-3"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)
	ctx := context.Background()

	t.Run("success on first attempt", func(t *testing.T) {
		callCount := 0
		operation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
			callCount++
			return "success", &types.Usage{TotalTokens: 10}, nil
		}

		result, usage, err := manager.ExecuteWithFailover(ctx, operation)
		if err != nil {
			t.Fatalf("ExecuteWithFailover() error = %v", err)
		}
		if result != "success" {
			t.Errorf("result = %v, want success", result)
		}
		if usage.TotalTokens != 10 {
			t.Errorf("usage.TotalTokens = %v, want 10", usage.TotalTokens)
		}
		if callCount != 1 {
			t.Errorf("operation called %d times, expected 1", callCount)
		}
	})

	t.Run("failover on first credential failure", func(t *testing.T) {
		callCount := 0
		operation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
			callCount++
			if callCount == 1 {
				return "", nil, errors.New("first credential failed")
			}
			return "success", &types.Usage{TotalTokens: 20}, nil
		}

		result, usage, err := manager.ExecuteWithFailover(ctx, operation)
		if err != nil {
			t.Fatalf("ExecuteWithFailover() error = %v", err)
		}
		if result != "success" {
			t.Errorf("result = %v, want success", result)
		}
		if usage.TotalTokens != 20 {
			t.Errorf("usage.TotalTokens = %v, want 20", usage.TotalTokens)
		}
		if callCount != 2 {
			t.Errorf("operation called %d times, expected 2", callCount)
		}
	})

	t.Run("all credentials fail", func(t *testing.T) {
		operation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
			return "", nil, errors.New("operation failed")
		}

		_, _, err := manager.ExecuteWithFailover(ctx, operation)
		if err == nil {
			t.Error("ExecuteWithFailover() expected error when all credentials fail")
		}
	})
}
