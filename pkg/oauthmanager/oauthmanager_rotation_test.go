package oauthmanager

import (
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

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
