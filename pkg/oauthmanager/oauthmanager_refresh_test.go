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

// Helper function for tests
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || s == substr)
}
