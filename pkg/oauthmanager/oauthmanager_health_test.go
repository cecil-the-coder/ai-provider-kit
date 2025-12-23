package oauthmanager

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

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
