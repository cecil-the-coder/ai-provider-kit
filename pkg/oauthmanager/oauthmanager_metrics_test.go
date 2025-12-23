package oauthmanager

import (
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ===== METRICS TESTS =====

func TestCredentialMetrics_GetSnapshot(t *testing.T) {
	metrics := NewCredentialMetrics()

	// Record some data
	metrics.recordRequest(100, 50*time.Millisecond, true)
	metrics.recordRequest(200, 100*time.Millisecond, true)
	metrics.recordRequest(150, 75*time.Millisecond, false)

	snapshot := metrics.GetSnapshot()

	if snapshot.RequestCount != 3 {
		t.Errorf("RequestCount = %d, want 3", snapshot.RequestCount)
	}
	if snapshot.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d, want 2", snapshot.SuccessCount)
	}
	if snapshot.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", snapshot.ErrorCount)
	}
	if snapshot.TokensUsed != 450 {
		t.Errorf("TokensUsed = %d, want 450", snapshot.TokensUsed)
	}
	if snapshot.AverageLatency == 0 {
		t.Error("AverageLatency should not be 0")
	}
}

func TestCredentialMetrics_GetSuccessRate(t *testing.T) {
	tests := []struct {
		name      string
		successes int
		failures  int
		wantRate  float64
	}{
		{
			name:      "100% success",
			successes: 10,
			failures:  0,
			wantRate:  1.0,
		},
		{
			name:      "50% success",
			successes: 5,
			failures:  5,
			wantRate:  0.5,
		},
		{
			name:      "no requests",
			successes: 0,
			failures:  0,
			wantRate:  1.0,
		},
		{
			name:      "25% success",
			successes: 1,
			failures:  3,
			wantRate:  0.25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := NewCredentialMetrics()

			for i := 0; i < tt.successes; i++ {
				metrics.recordRequest(10, 10*time.Millisecond, true)
			}
			for i := 0; i < tt.failures; i++ {
				metrics.recordRequest(10, 10*time.Millisecond, false)
			}

			rate := metrics.GetSuccessRate()
			if rate != tt.wantRate {
				t.Errorf("GetSuccessRate() = %v, want %v", rate, tt.wantRate)
			}
		})
	}
}

func TestCredentialMetrics_GetRequestsPerHour(t *testing.T) {
	metrics := NewCredentialMetrics()

	// Test with no requests
	rate := metrics.GetRequestsPerHour()
	if rate != 0.0 {
		t.Errorf("GetRequestsPerHour() with no requests = %v, want 0.0", rate)
	}

	// Record some requests
	metrics.recordRequest(10, 10*time.Millisecond, true)
	metrics.recordRequest(10, 10*time.Millisecond, true)

	// Should return a rate (exact value depends on timing)
	rate = metrics.GetRequestsPerHour()
	if rate <= 0 {
		t.Errorf("GetRequestsPerHour() = %v, want > 0", rate)
	}
}

func TestOAuthKeyManager_GetCredentialMetrics(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Record some metrics
	manager.RecordRequest("cred-1", 100, 50*time.Millisecond, true)
	manager.RecordRequest("cred-1", 200, 100*time.Millisecond, true)

	// Get metrics
	metrics := manager.GetCredentialMetrics("cred-1")
	if metrics == nil {
		t.Fatal("GetCredentialMetrics() returned nil")
	}

	if metrics.RequestCount != 2 {
		t.Errorf("RequestCount = %d, want 2", metrics.RequestCount)
	}
	if metrics.TokensUsed != 300 {
		t.Errorf("TokensUsed = %d, want 300", metrics.TokensUsed)
	}

	// Test non-existent credential
	nilMetrics := manager.GetCredentialMetrics("non-existent")
	if nilMetrics != nil {
		t.Error("GetCredentialMetrics() for non-existent credential should return nil")
	}
}

func TestOAuthKeyManager_GetAllMetrics(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Record metrics for both
	manager.RecordRequest("cred-1", 100, 50*time.Millisecond, true)
	manager.RecordRequest("cred-2", 200, 100*time.Millisecond, true)

	allMetrics := manager.GetAllMetrics()

	if len(allMetrics) != 2 {
		t.Errorf("GetAllMetrics() returned %d metrics, want 2", len(allMetrics))
	}

	if allMetrics["cred-1"] == nil {
		t.Error("Metrics for cred-1 not found")
	}
	if allMetrics["cred-2"] == nil {
		t.Error("Metrics for cred-2 not found")
	}
}
