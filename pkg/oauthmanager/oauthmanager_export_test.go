package oauthmanager

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

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
