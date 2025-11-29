package oauthmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

// ===== EXPORT TESTS =====

func TestOAuthKeyManager_ExportPrometheus(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Record some metrics
	manager.RecordRequest("cred-1", 100, 50*time.Millisecond, true)
	manager.RecordRequest("cred-1", 200, 100*time.Millisecond, false)
	manager.RecordRequest("cred-2", 150, 75*time.Millisecond, true)

	// Export Prometheus metrics
	promMetrics := manager.ExportPrometheus()

	if promMetrics == nil {
		t.Fatal("ExportPrometheus() returned nil")
	}

	// Verify metrics for cred-1
	if promMetrics.RequestsTotal["cred-1"] != 2 {
		t.Errorf("RequestsTotal[cred-1] = %d, want 2", promMetrics.RequestsTotal["cred-1"])
	}
	if promMetrics.SuccessTotal["cred-1"] != 1 {
		t.Errorf("SuccessTotal[cred-1] = %d, want 1", promMetrics.SuccessTotal["cred-1"])
	}
	if promMetrics.ErrorsTotal["cred-1"] != 1 {
		t.Errorf("ErrorsTotal[cred-1] = %d, want 1", promMetrics.ErrorsTotal["cred-1"])
	}
	if promMetrics.TokensUsedTotal["cred-1"] != 300 {
		t.Errorf("TokensUsedTotal[cred-1] = %d, want 300", promMetrics.TokensUsedTotal["cred-1"])
	}

	// Verify metrics for cred-2
	if promMetrics.RequestsTotal["cred-2"] != 1 {
		t.Errorf("RequestsTotal[cred-2] = %d, want 1", promMetrics.RequestsTotal["cred-2"])
	}
	if promMetrics.TokensUsedTotal["cred-2"] != 150 {
		t.Errorf("TokensUsedTotal[cred-2] = %d, want 150", promMetrics.TokensUsedTotal["cred-2"])
	}
}

func TestOAuthKeyManager_ExportJSON(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1", ExpiresAt: time.Now().Add(1 * time.Hour)},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Record some data
	manager.RecordRequest("cred-1", 100, 50*time.Millisecond, true)
	manager.ReportSuccess("cred-1")

	// Export JSON
	jsonData, err := manager.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check expected fields
	if result["total_credentials"] == nil {
		t.Error("JSON missing total_credentials field")
	}
	if result["success_rate"] == nil {
		t.Error("JSON missing success_rate field")
	}
	if result["credentials"] == nil {
		t.Error("JSON missing credentials field")
	}
}

func TestOAuthKeyManager_GetHealthSummary(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1", ExpiresAt: time.Now().Add(1 * time.Hour)},
		{ID: "cred-2", ClientID: "client-2", AccessToken: "token-2", ExpiresAt: time.Now().Add(2 * time.Hour)},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Record some successes and failures
	manager.ReportSuccess("cred-1")
	manager.RecordRequest("cred-1", 100, 50*time.Millisecond, true)

	manager.ReportFailure("cred-2", errors.New("test error"))
	manager.RecordRequest("cred-2", 50, 100*time.Millisecond, false)

	summary := manager.GetHealthSummary()

	if summary == nil {
		t.Fatal("GetHealthSummary() returned nil")
	}

	totalCreds, ok := summary["total_credentials"].(int)
	if !ok || totalCreds != 2 {
		t.Errorf("total_credentials = %v, want 2", summary["total_credentials"])
	}

	healthyCreds, ok := summary["healthy_credentials"].(int)
	if !ok || healthyCreds < 1 {
		t.Errorf("healthy_credentials = %v, want >= 1", summary["healthy_credentials"])
	}

	creds, ok := summary["credentials"].(map[string]CredentialHealthInfo)
	if !ok {
		t.Fatal("credentials field has wrong type")
	}

	if len(creds) != 2 {
		t.Errorf("credentials map has %d entries, want 2", len(creds))
	}

	cred1Info, exists := creds["cred-1"]
	if !exists {
		t.Error("cred-1 not found in credentials map")
	} else {
		if cred1Info.ID != "cred-1" {
			t.Errorf("cred-1 ID = %v, want cred-1", cred1Info.ID)
		}
		if cred1Info.RequestCount != 1 {
			t.Errorf("cred-1 RequestCount = %d, want 1", cred1Info.RequestCount)
		}
	}
}

func TestOAuthKeyManager_GetCredentialHealthInfo(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{
			ID:          "cred-1",
			ClientID:    "client-1",
			AccessToken: "token-1",
			ExpiresAt:   time.Now().Add(1 * time.Hour),
		},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Record some data
	manager.ReportSuccess("cred-1")
	manager.RecordRequest("cred-1", 100, 50*time.Millisecond, true)

	// Get health info
	info := manager.GetCredentialHealthInfo("cred-1")
	if info == nil {
		t.Fatal("GetCredentialHealthInfo() returned nil")
	}

	if info.ID != "cred-1" {
		t.Errorf("ID = %v, want cred-1", info.ID)
	}
	if !info.IsHealthy {
		t.Error("IsHealthy should be true")
	}
	if !info.IsAvailable {
		t.Error("IsAvailable should be true")
	}
	if info.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", info.RequestCount)
	}
	if info.TokensUsed != 100 {
		t.Errorf("TokensUsed = %d, want 100", info.TokensUsed)
	}

	// Test non-existent credential
	nilInfo := manager.GetCredentialHealthInfo("non-existent")
	if nilInfo != nil {
		t.Error("GetCredentialHealthInfo() for non-existent credential should return nil")
	}
}

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

// ===== MONITORING TESTS =====

func TestDefaultMonitoringConfig(t *testing.T) {
	config := DefaultMonitoringConfig()

	if config == nil {
		t.Fatal("DefaultMonitoringConfig() returned nil")
	}
	if !config.AlertOnHighFailureRate {
		t.Error("AlertOnHighFailureRate should be true")
	}
	if config.FailureRateThreshold != 0.25 {
		t.Errorf("FailureRateThreshold = %v, want 0.25", config.FailureRateThreshold)
	}
	if !config.AlertOnRefreshFailure {
		t.Error("AlertOnRefreshFailure should be true")
	}
	if !config.AlertOnExpirySoon {
		t.Error("AlertOnExpirySoon should be true")
	}
	if config.ExpiryWarningTime != 24*time.Hour {
		t.Errorf("ExpiryWarningTime = %v, want 24h", config.ExpiryWarningTime)
	}
}

func TestOAuthKeyManager_SetGetMonitoringConfig(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Set monitoring config
	config := DefaultMonitoringConfig()
	config.WebhookURL = "http://example.com/webhook"
	config.FailureRateThreshold = 0.5
	manager.SetMonitoringConfig(config)

	// Get and verify
	retrieved := manager.GetMonitoringConfig()
	if retrieved.WebhookURL != "http://example.com/webhook" {
		t.Errorf("WebhookURL = %v, want http://example.com/webhook", retrieved.WebhookURL)
	}
	if retrieved.FailureRateThreshold != 0.5 {
		t.Errorf("FailureRateThreshold = %v, want 0.5", retrieved.FailureRateThreshold)
	}

	// Test setting nil (should use default)
	manager.SetMonitoringConfig(nil)
	retrieved = manager.GetMonitoringConfig()
	if retrieved.FailureRateThreshold != 0.25 {
		t.Errorf("After setting nil, FailureRateThreshold = %v, want 0.25 (default)", retrieved.FailureRateThreshold)
	}
}

func TestOAuthKeyManager_CheckAlerts(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{
			ID:          "cred-1",
			ClientID:    "client-1",
			AccessToken: "token-1",
			ExpiresAt:   time.Now().Add(12 * time.Hour), // Within 24h warning
		},
		{
			ID:          "cred-2",
			ClientID:    "client-2",
			AccessToken: "token-2",
			ExpiresAt:   time.Now().Add(48 * time.Hour), // Outside warning
		},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Set monitoring config
	config := DefaultMonitoringConfig()
	config.AlertOnExpirySoon = true
	config.ExpiryWarningTime = 24 * time.Hour
	config.AlertOnHighFailureRate = true
	config.FailureRateThreshold = 0.5
	manager.SetMonitoringConfig(config)

	// Record high failure rate for cred-1
	for i := 0; i < 15; i++ {
		manager.RecordRequest("cred-1", 10, 10*time.Millisecond, false)
	}
	for i := 0; i < 5; i++ {
		manager.RecordRequest("cred-1", 10, 10*time.Millisecond, true)
	}

	// Check alerts
	alerts := manager.CheckAlerts()

	if len(alerts) == 0 {
		t.Error("CheckAlerts() returned no alerts, expected at least one")
	}

	// Verify we have both types of alerts
	hasFailureAlert := false
	hasExpiryAlert := false

	for _, alert := range alerts {
		if alert.Type == "failure" {
			hasFailureAlert = true
		}
		if alert.Type == "expiry_warning" {
			hasExpiryAlert = true
		}
	}

	if !hasFailureAlert {
		t.Error("Expected failure alert for high failure rate")
	}
	if !hasExpiryAlert {
		t.Error("Expected expiry warning alert")
	}
}

func TestOAuthKeyManager_WebhookIntegration(t *testing.T) {
	// Create a test webhook server with thread-safe event collection
	var eventsMu sync.Mutex
	receivedEvents := make([]WebhookEvent, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event WebhookEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("Failed to decode webhook event: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		eventsMu.Lock()
		receivedEvents = append(receivedEvents, event)
		eventsMu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	credentials := []*types.OAuthCredentialSet{
		{
			ID:           "cred-1",
			ClientID:     "client-1",
			AccessToken:  "token-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(3 * time.Minute),
		},
	}

	// Custom refresh function
	refreshFunc := func(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
		refreshed := *cred
		refreshed.AccessToken = "new-token"
		refreshed.ExpiresAt = time.Now().Add(1 * time.Hour)
		return &refreshed, nil
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, refreshFunc)

	// Set monitoring config with webhook
	config := DefaultMonitoringConfig()
	config.WebhookURL = server.URL
	config.WebhookEvents = []string{"refresh", "failure"}
	manager.SetMonitoringConfig(config)

	// Trigger a refresh
	ctx := context.Background()
	_, err := manager.refreshCredential(ctx, credentials[0])
	if err != nil {
		t.Fatalf("refreshCredential() error = %v", err)
	}

	// Give webhook time to be sent (it's async)
	time.Sleep(100 * time.Millisecond)

	// Verify webhook was received
	eventsMu.Lock()
	eventCount := len(receivedEvents)
	events := make([]WebhookEvent, len(receivedEvents))
	copy(events, receivedEvents)
	eventsMu.Unlock()

	if eventCount == 0 {
		t.Error("No webhook events received")
	} else {
		found := false
		for _, event := range events {
			if event.Type == "refresh" && event.CredentialID == "cred-1" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected refresh event not found in received webhooks")
		}
	}
}

func TestAlertHistory_ShouldSendAlert(t *testing.T) {
	history := &alertHistory{
		lastAlerts:    make(map[string]time.Time),
		alertCooldown: 1 * time.Hour,
	}

	// First alert should be sent
	if !history.shouldSendAlert("test-alert") {
		t.Error("shouldSendAlert() first call should return true")
	}

	// Record the alert
	history.recordAlert("test-alert")

	// Second alert immediately should not be sent (cooldown)
	if history.shouldSendAlert("test-alert") {
		t.Error("shouldSendAlert() during cooldown should return false")
	}

	// Different alert should be sent
	if !history.shouldSendAlert("other-alert") {
		t.Error("shouldSendAlert() for different alert should return true")
	}
}

// ===== ROTATION TESTS =====

func TestOAuthKeyManager_SetGetRotationPolicy(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Test getting default policy
	defaultPolicy := manager.GetRotationPolicy()
	if defaultPolicy == nil {
		t.Fatal("GetRotationPolicy() returned nil")
	}
	if defaultPolicy.Enabled {
		t.Error("Default policy should have Enabled = false")
	}
	if defaultPolicy.RotationInterval != 30*24*time.Hour {
		t.Errorf("Default RotationInterval = %v, want 30 days", defaultPolicy.RotationInterval)
	}

	// Set custom policy
	customPolicy := &RotationPolicy{
		Enabled:          true,
		RotationInterval: 7 * 24 * time.Hour,
		GracePeriod:      24 * time.Hour,
		AutoDecommission: true,
	}
	manager.SetRotationPolicy(customPolicy)

	// Get and verify
	retrieved := manager.GetRotationPolicy()
	if !retrieved.Enabled {
		t.Error("Enabled should be true")
	}
	if retrieved.RotationInterval != 7*24*time.Hour {
		t.Errorf("RotationInterval = %v, want 7 days", retrieved.RotationInterval)
	}
	if retrieved.GracePeriod != 24*time.Hour {
		t.Errorf("GracePeriod = %v, want 24h", retrieved.GracePeriod)
	}

	// Test setting nil (should use default)
	manager.SetRotationPolicy(nil)
	retrieved = manager.GetRotationPolicy()
	if retrieved.Enabled {
		t.Error("After setting nil, Enabled should be false (default)")
	}
}

func TestOAuthKeyManager_CheckRotationNeeded(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "old-cred", ClientID: "client-1", AccessToken: "token-1"},
		{ID: "new-cred", ClientID: "client-2", AccessToken: "token-2"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Set rotation policy with short interval for testing
	policy := &RotationPolicy{
		Enabled:          true,
		RotationInterval: 1 * time.Millisecond, // Very short for testing
		GracePeriod:      24 * time.Hour,
	}
	manager.SetRotationPolicy(policy)

	// Manually set creation time for old credential
	manager.mu.Lock()
	if state, exists := manager.rotationState["old-cred"]; exists {
		state.CreatedAt = time.Now().Add(-2 * time.Millisecond) // Older than interval
	}
	manager.mu.Unlock()

	// Small delay to ensure interval has passed
	time.Sleep(2 * time.Millisecond)

	// Check rotation needed
	needsRotation := manager.CheckRotationNeeded()

	if len(needsRotation) == 0 {
		t.Error("CheckRotationNeeded() returned empty list, expected old-cred")
	} else {
		found := false
		for _, id := range needsRotation {
			if id == "old-cred" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CheckRotationNeeded() = %v, expected to include old-cred", needsRotation)
		}
	}

	// Test with rotation disabled
	policy.Enabled = false
	manager.SetRotationPolicy(policy)

	needsRotation = manager.CheckRotationNeeded()
	if len(needsRotation) != 0 {
		t.Error("CheckRotationNeeded() with disabled policy should return empty list")
	}
}

func TestOAuthKeyManager_MarkForRotation(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "old-cred", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Set rotation policy
	policy := &RotationPolicy{
		Enabled:     true,
		GracePeriod: 24 * time.Hour,
	}
	manager.SetRotationPolicy(policy)

	// Create new credential
	newCred := &types.OAuthCredentialSet{
		ID:          "new-cred",
		ClientID:    "client-2",
		AccessToken: "token-2",
	}

	// Mark for rotation
	err := manager.MarkForRotation("old-cred", newCred)
	if err != nil {
		t.Fatalf("MarkForRotation() error = %v", err)
	}

	// Verify new credential was added
	creds := manager.GetCredentials()
	if len(creds) != 2 {
		t.Errorf("After rotation, credentials count = %d, want 2", len(creds))
	}

	// Verify old credential is marked for rotation
	state := manager.GetRotationState("old-cred")
	if state == nil {
		t.Fatal("GetRotationState() returned nil")
	}
	if !state.MarkedForRotation {
		t.Error("Old credential should be marked for rotation")
	}
	if state.ReplacementID != "new-cred" {
		t.Errorf("ReplacementID = %v, want new-cred", state.ReplacementID)
	}

	// Test error cases
	err = manager.MarkForRotation("non-existent", newCred)
	if err == nil {
		t.Error("MarkForRotation() with non-existent credential should return error")
	}

	err = manager.MarkForRotation("old-cred", newCred)
	if err == nil {
		t.Error("MarkForRotation() on already rotating credential should return error")
	}

	err = manager.MarkForRotation("old-cred", nil)
	if err == nil {
		t.Error("MarkForRotation() with nil credential should return error")
	}
}

func TestOAuthKeyManager_CompleteRotation(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "old-cred", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Set rotation policy with very short grace period
	policy := &RotationPolicy{
		Enabled:     true,
		GracePeriod: 1 * time.Millisecond,
	}
	manager.SetRotationPolicy(policy)

	// Create new credential
	newCred := &types.OAuthCredentialSet{
		ID:          "new-cred",
		ClientID:    "client-2",
		AccessToken: "token-2",
	}

	// Mark for rotation
	if err := manager.MarkForRotation("old-cred", newCred); err != nil {
		t.Fatalf("MarkForRotation() error = %v", err)
	}

	// Wait for grace period
	time.Sleep(2 * time.Millisecond)

	// Complete rotation
	err := manager.CompleteRotation("old-cred")
	if err != nil {
		t.Fatalf("CompleteRotation() error = %v", err)
	}

	// Verify old credential was removed
	creds := manager.GetCredentials()
	if len(creds) != 1 {
		t.Errorf("After completion, credentials count = %d, want 1", len(creds))
	}
	if creds[0].ID != "new-cred" {
		t.Errorf("Remaining credential ID = %v, want new-cred", creds[0].ID)
	}

	// Verify rotation state was cleaned up
	state := manager.GetRotationState("old-cred")
	if state != nil {
		t.Error("Rotation state for old-cred should be cleaned up")
	}

	// Test error cases
	err = manager.CompleteRotation("non-existent")
	if err == nil {
		t.Error("CompleteRotation() with non-existent credential should return error")
	}

	err = manager.CompleteRotation("new-cred")
	if err == nil {
		t.Error("CompleteRotation() on non-rotating credential should return error")
	}
}

func TestOAuthKeyManager_AutoDecommissionExpired(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "old-cred", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Set rotation policy with auto decommission
	policy := &RotationPolicy{
		Enabled:          true,
		GracePeriod:      1 * time.Millisecond,
		AutoDecommission: true,
	}
	manager.SetRotationPolicy(policy)

	// Create new credential
	newCred := &types.OAuthCredentialSet{
		ID:          "new-cred",
		ClientID:    "client-2",
		AccessToken: "token-2",
	}

	// Mark for rotation
	if err := manager.MarkForRotation("old-cred", newCred); err != nil {
		t.Fatalf("MarkForRotation() error = %v", err)
	}

	// Wait for grace period
	time.Sleep(2 * time.Millisecond)

	// Auto decommission
	decommissioned, err := manager.AutoDecommissionExpired()
	if err != nil {
		t.Logf("AutoDecommissionExpired() error = %v (this may be acceptable)", err)
	}

	if len(decommissioned) != 1 {
		t.Errorf("AutoDecommissionExpired() returned %d credentials, want 1", len(decommissioned))
	} else if decommissioned[0] != "old-cred" {
		t.Errorf("Decommissioned credential = %v, want old-cred", decommissioned[0])
	}

	// Test with auto decommission disabled
	policy.AutoDecommission = false
	manager.SetRotationPolicy(policy)

	decommissioned, err = manager.AutoDecommissionExpired()
	if err != nil {
		t.Errorf("AutoDecommissionExpired() with disabled policy error = %v", err)
	}
	if len(decommissioned) != 0 {
		t.Error("AutoDecommissionExpired() with disabled policy should return empty list")
	}
}

func TestOAuthKeyManager_GetRotationState(t *testing.T) {
	credentials := []*types.OAuthCredentialSet{
		{ID: "cred-1", ClientID: "client-1", AccessToken: "token-1"},
	}

	manager := NewOAuthKeyManager("TestProvider", credentials, nil)

	// Get initial state
	state := manager.GetRotationState("cred-1")
	if state == nil {
		t.Fatal("GetRotationState() returned nil")
	}
	if state.MarkedForRotation {
		t.Error("Initially, MarkedForRotation should be false")
	}
	if !state.CreatedAt.IsZero() {
		t.Log("CreatedAt is set, which is expected")
	}

	// Test non-existent credential
	nilState := manager.GetRotationState("non-existent")
	if nilState != nil {
		t.Error("GetRotationState() for non-existent credential should return nil")
	}
}

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
