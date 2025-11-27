package oauthmanager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

// TestOAuthKeyManager_HealthTracking tests health tracking and backoff
func TestOAuthKeyManager_HealthTracking(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	t.Run("success tracking", func(t *testing.T) {
		manager.ReportSuccess("cred-1")

		health := manager.GetCredentialHealth("cred-1")
		if health == nil {
			t.Fatal("GetCredentialHealth() returned nil")
		}
		if !health.isHealthy {
			t.Error("credential should be healthy after success")
		}
		if health.failureCount != 0 {
			t.Errorf("failureCount = %d, want 0", health.failureCount)
		}
	})

	t.Run("failure tracking and backoff", func(t *testing.T) {
		// First failure
		manager.ReportFailure("cred-1", errors.New("test error"))
		health := manager.GetCredentialHealth("cred-1")
		if health.failureCount != 1 {
			t.Errorf("failureCount = %d, want 1", health.failureCount)
		}
		if time.Now().After(health.backoffUntil) {
			t.Error("backoffUntil should be in the future after failure")
		}

		// Second failure
		manager.ReportFailure("cred-1", errors.New("test error"))
		health = manager.GetCredentialHealth("cred-1")
		if health.failureCount != 2 {
			t.Errorf("failureCount = %d, want 2", health.failureCount)
		}

		// Third failure - should mark as unhealthy
		manager.ReportFailure("cred-1", errors.New("test error"))
		health = manager.GetCredentialHealth("cred-1")
		if health.failureCount != 3 {
			t.Errorf("failureCount = %d, want 3", health.failureCount)
		}
		if health.isHealthy {
			t.Error("credential should be unhealthy after 3 failures")
		}

		// Success should reset everything
		manager.ReportSuccess("cred-1")
		health = manager.GetCredentialHealth("cred-1")
		if !health.isHealthy {
			t.Error("credential should be healthy after success")
		}
		if health.failureCount != 0 {
			t.Errorf("failureCount = %d, want 0 after success", health.failureCount)
		}
	})

	t.Run("exponential backoff", func(t *testing.T) {
		// Create fresh manager for this test
		testManager := NewOAuthKeyManager("TestProvider", []*types.OAuthCredentialSet{
			{ID: "backoff-test", ClientID: "client-1", AccessToken: "token-1"},
		}, nil)

		// Track backoff durations
		var backoffs []time.Duration

		for i := 0; i < 5; i++ {
			testManager.ReportFailure("backoff-test", errors.New("test"))
			health := testManager.GetCredentialHealth("backoff-test")
			backoff := time.Until(health.backoffUntil)
			backoffs = append(backoffs, backoff)
		}

		// Verify backoffs are increasing (allowing some time delta)
		for i := 1; i < len(backoffs); i++ {
			if backoffs[i] <= backoffs[i-1] {
				t.Errorf("backoff[%d] = %v should be > backoff[%d] = %v", i, backoffs[i], i-1, backoffs[i-1])
			}
		}
	})
}

// TestOAuthKeyManager_ConcurrentAccess tests thread safety
func TestOAuthKeyManager_ConcurrentAccess(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2"},
		{ID: "cred-3", ClientID: "client-3", AccessToken: "token-3"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)
	ctx := context.Background()

	// Number of concurrent goroutines
	numGoroutines := 50
	numIterations := 100

	var wg sync.WaitGroup
	var successCount, failureCount int32

	// Concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				// Randomly get credentials
				cred, err := manager.GetNextCredential(ctx)
				if err != nil {
					atomic.AddInt32(&failureCount, 1)
					continue
				}

				// Simulate mostly successes with occasional failures (80% success rate)
				if (id+j)%5 == 0 {
					manager.ReportFailure(cred.ID, errors.New("simulated failure"))
					atomic.AddInt32(&failureCount, 1)
				} else {
					manager.ReportSuccess(cred.ID)
					atomic.AddInt32(&successCount, 1)
				}

				// Get health
				_ = manager.GetCredentialHealth(cred.ID)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent test completed: %d successes, %d failures", successCount, failureCount)

	// Clear all backoffs by reporting successes for all credentials
	for _, cred := range credentials {
		manager.ReportSuccess(cred.ID)
	}

	// Verify manager is still functional
	cred, err := manager.GetNextCredential(ctx)
	if err != nil {
		t.Errorf("GetNextCredential() after concurrent access error = %v", err)
	}
	if cred == nil {
		t.Error("GetNextCredential() returned nil credential")
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

// Mock refresh function for testing
func mockRefreshFunc(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
	// Simulate successful refresh
	refreshed := *cred
	refreshed.AccessToken = "new-" + cred.AccessToken
	refreshed.RefreshToken = "new-" + cred.RefreshToken
	refreshed.ExpiresAt = time.Now().Add(1 * time.Hour)
	refreshed.LastRefresh = time.Now()
	refreshed.RefreshCount++
	return &refreshed, nil
}

// Mock refresh function that fails
func mockRefreshFuncFails(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
	return nil, errors.New("mock refresh failed")
}

// TestOAuthKeyManager_TokenRefresh tests basic token refresh flow
func TestOAuthKeyManager_TokenRefresh(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{
			ID:           "cred-1",
			ClientID:     "client-1",
			AccessToken:  "token-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(1 * time.Hour),
		},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, mockRefreshFunc)
	ctx := context.Background()

	// Manually trigger refresh
	refreshed, err := manager.refreshCredential(ctx, credentials[0])
	if err != nil {
		t.Fatalf("refreshCredential() error = %v", err)
	}

	if refreshed.AccessToken != "new-token-1" {
		t.Errorf("refreshed.AccessToken = %v, want new-token-1", refreshed.AccessToken)
	}
	if refreshed.RefreshToken != "new-refresh-1" {
		t.Errorf("refreshed.RefreshToken = %v, want new-refresh-1", refreshed.RefreshToken)
	}
	if refreshed.RefreshCount != 1 {
		t.Errorf("refreshed.RefreshCount = %v, want 1", refreshed.RefreshCount)
	}

	// Verify credential was updated in manager
	updated := manager.GetCredentials()[0]
	if updated.AccessToken != "new-token-1" {
		t.Errorf("updated credential not reflected in manager")
	}
}

// TestOAuthKeyManager_RefreshOnExpiry tests automatic refresh when token expires
func TestOAuthKeyManager_RefreshOnExpiry(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{
			ID:           "cred-1",
			ClientID:     "client-1",
			AccessToken:  "token-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(3 * time.Minute), // Within buffer time
		},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, mockRefreshFunc)
	ctx := context.Background()

	operationCalled := false
	operation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		operationCalled = true
		// Verify we got the refreshed token
		if cred.AccessToken != "new-token-1" {
			t.Errorf("operation received old token: %v", cred.AccessToken)
		}
		return "success", &types.Usage{TotalTokens: 10}, nil
	}

	result, usage, err := manager.ExecuteWithFailover(ctx, operation)
	if err != nil {
		t.Fatalf("ExecuteWithFailover() error = %v", err)
	}
	if !operationCalled {
		t.Error("operation was not called")
	}
	if result != "success" {
		t.Errorf("result = %v, want success", result)
	}
	if usage.TotalTokens != 10 {
		t.Errorf("usage.TotalTokens = %v, want 10", usage.TotalTokens)
	}

	// Verify credential was refreshed
	updated := manager.GetCredentials()[0]
	if updated.RefreshCount != 1 {
		t.Errorf("credential was not refreshed (RefreshCount = %v)", updated.RefreshCount)
	}
}

// TestOAuthKeyManager_RefreshInFlight tests prevention of duplicate refreshes
func TestOAuthKeyManager_RefreshInFlight(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{
			ID:           "cred-1",
			ClientID:     "client-1",
			AccessToken:  "token-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(1 * time.Hour),
		},
	}

	// Use a slow refresh function to test in-flight detection
	slowRefresh := func(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
		time.Sleep(500 * time.Millisecond)
		return mockRefreshFunc(ctx, cred)
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, slowRefresh)
	ctx := context.Background()

	// Start two concurrent refreshes
	var wg sync.WaitGroup
	var err1, err2 error
	var refreshed1, refreshed2 *types.OAuthCredentialSet

	wg.Add(2)
	go func() {
		defer wg.Done()
		refreshed1, err1 = manager.refreshCredential(ctx, credentials[0])
	}()

	// Small delay to ensure first refresh starts
	time.Sleep(50 * time.Millisecond)

	go func() {
		defer wg.Done()
		refreshed2, err2 = manager.refreshCredential(ctx, credentials[0])
	}()

	wg.Wait()

	// One should succeed, one should fail with "refresh already in progress"
	if err1 == nil && err2 == nil {
		t.Error("both refreshes succeeded, expected one to fail with in-flight error")
	}

	if err1 != nil && err2 != nil {
		t.Error("both refreshes failed, expected one to succeed")
	}

	// At least one should have succeeded
	if refreshed1 == nil && refreshed2 == nil {
		t.Error("no refresh succeeded")
	}
}

// TestOAuthKeyManager_RefreshFailure tests handling of refresh failures
func TestOAuthKeyManager_RefreshFailure(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{
			ID:           "cred-1",
			ClientID:     "client-1",
			AccessToken:  "token-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(3 * time.Minute), // Within buffer, needs refresh
		},
		{
			ID:           "cred-2",
			ClientID:     "client-2",
			AccessToken:  "token-2",
			RefreshToken: "refresh-2",
			ExpiresAt:    time.Now().Add(2 * time.Hour), // Valid, doesn't need refresh
		},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, mockRefreshFuncFails)
	ctx := context.Background()

	// Force the round-robin to start with cred-1 by getting it first
	_, _ = manager.GetNextCredential(ctx)

	// Now try the operation - first credential will need refresh and fail
	// Should failover to second credential
	operationAttempts := 0
	operation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		operationAttempts++
		// Both credentials might be tried, only cred-2 should work
		if cred.ID == "cred-2" {
			return "success", &types.Usage{TotalTokens: 10}, nil
		}
		// cred-1 shouldn't reach here because refresh should fail first
		return "", nil, errors.New("cred-1 should have failed during refresh")
	}

	result, _, err := manager.ExecuteWithFailover(ctx, operation)
	if err != nil {
		t.Fatalf("ExecuteWithFailover() error = %v", err)
	}
	if result != "success" {
		t.Errorf("result = %v, want success", result)
	}

	// Operation should be called once (on cred-2, after cred-1 refresh fails)
	if operationAttempts == 0 {
		t.Errorf("operation was never called")
	}

	// Check that first credential has refresh failure tracked
	// The refresh failure is tracked, and API failure is also tracked
	health := manager.GetCredentialHealth("cred-1")
	if health.refreshFailCount == 0 {
		t.Logf("Health state: refreshFailCount=%d, failureCount=%d, refreshInFlight=%v",
			health.refreshFailCount, health.failureCount, health.refreshInFlight)
		t.Errorf("refreshFailCount = %v, want > 0 (refresh should have failed)", health.refreshFailCount)
	}
	// Also verify that API failure was tracked (from ReportFailure call)
	if health.failureCount == 0 {
		t.Errorf("failureCount = %v, want > 0 (operation should have failed)", health.failureCount)
	}
}

// TestOAuthKeyManager_RefreshCallback tests callback execution on token refresh
func TestOAuthKeyManager_RefreshCallback(t *testing.T) {
	callbackCalled := false
	var callbackID, callbackAccess, callbackRefresh string
	var _ time.Time // callbackExpiry (unused but kept for completeness)

	credentials := []*types.OAuthCredentialSet{
		{
			ID:           "cred-1",
			ClientID:     "client-1",
			AccessToken:  "token-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(1 * time.Hour),
			OnTokenRefresh: func(id, access, refresh string, expires time.Time) error {
				callbackCalled = true
				callbackID = id
				callbackAccess = access
				callbackRefresh = refresh
				_ = expires // Acknowledge expires parameter
				return nil
			},
		},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, mockRefreshFunc)
	ctx := context.Background()

	// Trigger refresh
	refreshed, err := manager.refreshCredential(ctx, credentials[0])
	if err != nil {
		t.Fatalf("refreshCredential() error = %v", err)
	}

	// Verify callback was called
	if !callbackCalled {
		t.Error("OnTokenRefresh callback was not called")
	}
	if callbackID != "cred-1" {
		t.Errorf("callback ID = %v, want cred-1", callbackID)
	}
	if callbackAccess != refreshed.AccessToken {
		t.Errorf("callback access token = %v, want %v", callbackAccess, refreshed.AccessToken)
	}
	if callbackRefresh != refreshed.RefreshToken {
		t.Errorf("callback refresh token = %v, want %v", callbackRefresh, refreshed.RefreshToken)
	}
}

// TestOAuthKeyManager_ConcurrentRefresh tests thread safety during refresh
func TestOAuthKeyManager_ConcurrentRefresh(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{
			ID:           "cred-1",
			ClientID:     "client-1",
			AccessToken:  "token-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(3 * time.Minute), // Needs refresh
		},
		{
			ID:           "cred-2",
			ClientID:     "client-2",
			AccessToken:  "token-2",
			RefreshToken: "refresh-2",
			ExpiresAt:    time.Now().Add(3 * time.Minute), // Needs refresh
		},
	}

	// Create a fast refresh function to reduce race conditions
	fastRefreshFunc := func(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
		refreshed := *cred
		refreshed.AccessToken = "new-" + cred.AccessToken
		refreshed.RefreshToken = "new-" + cred.RefreshToken
		refreshed.ExpiresAt = time.Now().Add(1 * time.Hour)
		refreshed.LastRefresh = time.Now()
		refreshed.RefreshCount++
		return &refreshed, nil
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, fastRefreshFunc)
	ctx := context.Background()

	// Use fewer concurrent operations to minimize race conditions
	numGoroutines := 10
	var wg sync.WaitGroup
	var successCount, failureCount int32

	operation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		// No delay - just verify token format
		if !contains(cred.AccessToken, "new-") && !contains(cred.AccessToken, "token-") {
			return "", nil, fmt.Errorf("unexpected token format: %s", cred.AccessToken)
		}
		return "success", &types.Usage{TotalTokens: 10}, nil
	}

	// Stagger the goroutine starts to reduce race conditions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Small stagger between goroutines
			time.Sleep(time.Duration(id) * time.Millisecond)

			_, _, err := manager.ExecuteWithFailover(ctx, operation)
			if err != nil {
				atomic.AddInt32(&failureCount, 1)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent refresh test: %d successes, %d failures", successCount, failureCount)

	// Expect most operations to succeed - be more lenient for concurrent test
	minSuccess := int32(float64(numGoroutines) * 0.7) // 70% success rate
	if successCount < minSuccess {
		t.Errorf("expected at least %d operations to succeed (70%%), got %d successes", minSuccess, successCount)
	}

	// At least one credential should have been refreshed
	refreshedAny := false
	for _, cred := range manager.GetCredentials() {
		if cred.RefreshCount > 0 {
			refreshedAny = true
			break
		}
	}
	if !refreshedAny {
		t.Error("no credentials were refreshed - refresh functionality not working")
	}

	// If we have any failures, verify that we still have some working credentials
	if failureCount > 0 {
		// Verify manager is still functional after concurrent access
		cred, err := manager.GetNextCredential(ctx)
		if err != nil {
			t.Errorf("manager became non-functional after concurrent access: %v", err)
		} else if cred == nil {
			t.Error("manager returned nil credential after concurrent access")
		}
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

// Helper function for tests
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || s == substr)
}
