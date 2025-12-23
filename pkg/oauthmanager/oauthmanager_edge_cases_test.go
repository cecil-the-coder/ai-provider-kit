package oauthmanager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ===== EDGE CASES AND ERROR HANDLING =====

func TestOAuthKeyManager_GetCredentials_NilManager(t *testing.T) {
	var manager *OAuthKeyManager
	creds := manager.GetCredentials()
	if creds != nil {
		t.Error("GetCredentials() on nil manager should return nil")
	}
}

func TestRecordRequest_NonExistentCredential(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Record request for non-existent credential - should create metrics
	manager.RecordRequest("non-existent", 100, 50*time.Millisecond, true)

	// Verify metrics were created
	metrics := manager.GetCredentialMetrics("non-existent")
	if metrics == nil {
		t.Error("RecordRequest() for non-existent credential should create metrics")
	} else if metrics.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", metrics.RequestCount)
	}
}

func TestRefreshStrategy_CalculateBufferTime_EdgeCases(t *testing.T) {
	strategy := DefaultRefreshStrategy()
	strategy.AdaptiveBuffer = true
	strategy.MinBuffer = 1 * time.Minute
	strategy.MaxBuffer = 15 * time.Minute

	// Test with high request rate
	metrics := NewCredentialMetrics()
	metrics.FirstUsed = time.Now().Add(-1 * time.Hour)
	for i := 0; i < 1000; i++ {
		metrics.recordRequest(10, 10*time.Millisecond, true)
	}

	bufferTime := strategy.CalculateBufferTime(metrics)

	// Should be capped at MaxBuffer
	if bufferTime > strategy.MaxBuffer {
		t.Errorf("CalculateBufferTime() = %v, should be capped at MaxBuffer %v", bufferTime, strategy.MaxBuffer)
	}

	// Should be at least MinBuffer
	if bufferTime < strategy.MinBuffer {
		t.Errorf("CalculateBufferTime() = %v, should be at least MinBuffer %v", bufferTime, strategy.MinBuffer)
	}
}

func TestCredentialHealth_RecordRefreshFailure_MultipleFailures(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
	}

	// Use a failing refresh function
	failRefresh := func(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
		return nil, fmt.Errorf("refresh failed")
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, failRefresh)
	ctx := context.Background()

	// Trigger multiple refresh failures
	for i := 0; i < 6; i++ {
		_, _ = manager.refreshCredential(ctx, credentials[0])
	}

	// Check health
	health := manager.GetCredentialHealth("cred-1")
	if health == nil {
		t.Fatal("GetCredentialHealth() returned nil")
	}

	// After 5+ refresh failures, should be unhealthy with backoff
	if health.refreshFailCount < 5 {
		t.Errorf("refreshFailCount = %d, want >= 5", health.refreshFailCount)
	}

	// Should have longer backoff for refresh failures
	if health.backoffUntil.IsZero() {
		t.Error("backoffUntil should be set after multiple refresh failures")
	}
}
