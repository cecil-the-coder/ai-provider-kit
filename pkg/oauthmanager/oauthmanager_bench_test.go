package oauthmanager

import (
	"context"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Benchmark tests
func BenchmarkOAuthKeyManager_GetNextCredential(b *testing.B) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2"},
		{ID: "cred-3", ClientID: "client-3", AccessToken: "token-3"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.GetNextCredential(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOAuthKeyManager_ExecuteWithFailover(b *testing.B) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2"},
		{ID: "cred-3", ClientID: "client-3", AccessToken: "token-3"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)
	ctx := context.Background()

	operation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		return "success", &types.Usage{TotalTokens: 10}, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := manager.ExecuteWithFailover(ctx, operation)
		if err != nil {
			b.Fatal(err)
		}
	}
}
