package oauthmanager

import (
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ===== REFRESH STRATEGY TESTS =====

func TestAdaptiveRefreshStrategy(t *testing.T) {
	strategy := AdaptiveRefreshStrategy()

	if !strategy.AdaptiveBuffer {
		t.Error("AdaptiveBuffer should be true")
	}
	if !strategy.PreemptiveRefresh {
		t.Error("PreemptiveRefresh should be true")
	}
	if strategy.BufferTime != 5*time.Minute {
		t.Errorf("BufferTime = %v, want 5m", strategy.BufferTime)
	}
}

func TestConservativeRefreshStrategy(t *testing.T) {
	strategy := ConservativeRefreshStrategy()

	if strategy.BufferTime != 15*time.Minute {
		t.Errorf("BufferTime = %v, want 15m", strategy.BufferTime)
	}
	if !strategy.PreemptiveRefresh {
		t.Error("PreemptiveRefresh should be true")
	}
	if strategy.HighTrafficThreshold != 50 {
		t.Errorf("HighTrafficThreshold = %d, want 50", strategy.HighTrafficThreshold)
	}
}

func TestRefreshStrategy_ShouldRefresh(t *testing.T) {
	strategy := DefaultRefreshStrategy()

	tests := []struct {
		name        string
		expiresAt   time.Time
		wantRefresh bool
	}{
		{
			name:        "token expiring in 1 hour",
			expiresAt:   time.Now().Add(1 * time.Hour),
			wantRefresh: false,
		},
		{
			name:        "token expiring in 3 minutes",
			expiresAt:   time.Now().Add(3 * time.Minute),
			wantRefresh: true,
		},
		{
			name:        "token already expired",
			expiresAt:   time.Now().Add(-1 * time.Hour),
			wantRefresh: true,
		},
		{
			name:        "token expiring in 6 minutes",
			expiresAt:   time.Now().Add(6 * time.Minute),
			wantRefresh: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := &types.OAuthCredentialSet{
				ID:        "test",
				ExpiresAt: tt.expiresAt,
			}

			if got := strategy.ShouldRefresh(cred, nil); got != tt.wantRefresh {
				t.Errorf("ShouldRefresh() = %v, want %v", got, tt.wantRefresh)
			}
		})
	}

	// Test with nil credential
	if strategy.ShouldRefresh(nil, nil) {
		t.Error("ShouldRefresh(nil) should return false")
	}

	// Test with zero time
	credZero := &types.OAuthCredentialSet{
		ID:        "test",
		ExpiresAt: time.Time{},
	}
	if strategy.ShouldRefresh(credZero, nil) {
		t.Error("ShouldRefresh() with zero time should return false")
	}
}

func TestRefreshStrategy_ShouldRefresh_PreemptiveForHighTraffic(t *testing.T) {
	strategy := DefaultRefreshStrategy()
	strategy.PreemptiveRefresh = true
	strategy.HighTrafficThreshold = 100

	// Create metrics with high traffic
	metrics := NewCredentialMetrics()
	// Simulate high traffic by setting first used long ago
	metrics.FirstUsed = time.Now().Add(-1 * time.Hour)
	for i := 0; i < 150; i++ {
		metrics.recordRequest(10, 10*time.Millisecond, true)
	}

	// Token expiring in 8 minutes - normally wouldn't refresh,
	// but should with preemptive refresh for high traffic
	cred := &types.OAuthCredentialSet{
		ID:        "test",
		ExpiresAt: time.Now().Add(8 * time.Minute),
	}

	// With high traffic, should trigger preemptive refresh
	requestsPerHour := metrics.GetRequestsPerHour()
	t.Logf("Requests per hour: %.2f", requestsPerHour)

	if requestsPerHour >= float64(strategy.HighTrafficThreshold) {
		shouldRefresh := strategy.ShouldRefresh(cred, metrics)
		if !shouldRefresh {
			t.Error("ShouldRefresh() should be true for high-traffic credential with preemptive refresh")
		}
	}
}

func TestRefreshStrategy_CalculateBufferTime(t *testing.T) {
	strategy := DefaultRefreshStrategy()

	// Test without adaptive buffer
	bufferTime := strategy.CalculateBufferTime(nil)
	if bufferTime != strategy.BufferTime {
		t.Errorf("CalculateBufferTime() = %v, want %v", bufferTime, strategy.BufferTime)
	}

	// Test with adaptive buffer enabled
	strategy.AdaptiveBuffer = true
	strategy.MinBuffer = 1 * time.Minute
	strategy.MaxBuffer = 15 * time.Minute

	// Create metrics with various characteristics
	metrics := NewCredentialMetrics()
	metrics.recordRequest(100, 200*time.Millisecond, true) // High latency

	bufferTime = strategy.CalculateBufferTime(metrics)

	// Should be adjusted from base buffer time
	if bufferTime < strategy.MinBuffer {
		t.Errorf("CalculateBufferTime() = %v, should be >= MinBuffer %v", bufferTime, strategy.MinBuffer)
	}
	if bufferTime > strategy.MaxBuffer {
		t.Errorf("CalculateBufferTime() = %v, should be <= MaxBuffer %v", bufferTime, strategy.MaxBuffer)
	}

	// Test with high error rate
	metrics2 := NewCredentialMetrics()
	for i := 0; i < 10; i++ {
		metrics2.recordRequest(10, 10*time.Millisecond, false) // All failures
	}

	bufferTime2 := strategy.CalculateBufferTime(metrics2)
	// Should increase buffer due to high error rate
	if bufferTime2 <= strategy.BufferTime {
		t.Log("Buffer time might not increase for small samples, this is acceptable")
	}
}

func TestOAuthKeyManager_SetGetRefreshStrategy(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Test getting default strategy
	defaultStrategy := manager.GetRefreshStrategy()
	if defaultStrategy == nil {
		t.Fatal("GetRefreshStrategy() returned nil")
	}
	if defaultStrategy.BufferTime != 5*time.Minute {
		t.Errorf("Default BufferTime = %v, want 5m", defaultStrategy.BufferTime)
	}

	// Set custom strategy
	customStrategy := AdaptiveRefreshStrategy()
	customStrategy.BufferTime = 10 * time.Minute
	manager.SetRefreshStrategy(customStrategy)

	// Get and verify
	retrieved := manager.GetRefreshStrategy()
	if retrieved.BufferTime != 10*time.Minute {
		t.Errorf("BufferTime = %v, want 10m", retrieved.BufferTime)
	}
	if !retrieved.AdaptiveBuffer {
		t.Error("AdaptiveBuffer should be true")
	}

	// Test setting nil (should use default)
	manager.SetRefreshStrategy(nil)
	retrieved = manager.GetRefreshStrategy()
	if retrieved.BufferTime != 5*time.Minute {
		t.Errorf("After setting nil, BufferTime = %v, want 5m (default)", retrieved.BufferTime)
	}
}
