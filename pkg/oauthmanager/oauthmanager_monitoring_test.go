package oauthmanager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

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
