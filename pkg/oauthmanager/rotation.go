package oauthmanager

import (
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// RotationPolicy defines how and when credentials should be rotated
type RotationPolicy struct {
	Enabled          bool          // Enable automatic rotation
	RotationInterval time.Duration // How often to rotate (default: 30 days)
	GracePeriod      time.Duration // Overlap period (default: 7 days)
	AutoDecommission bool          // Automatically remove old credentials

	// Callbacks
	OnRotationNeeded func(credentialID string) error // Called when rotation is due
	OnDecommission   func(credentialID string) error // Called before removal
}

// credentialRotationState tracks rotation state for a credential
type credentialRotationState struct {
	CreatedAt         time.Time // When the credential was added to the manager
	MarkedForRotation bool      // Whether this credential is pending rotation
	RotationStartedAt time.Time // When rotation was initiated
	ReplacementID     string    // ID of the replacement credential (if any)
	DecommissionAt    time.Time // When this credential should be removed
}

// DefaultRotationPolicy returns a default rotation policy
func DefaultRotationPolicy() *RotationPolicy {
	return &RotationPolicy{
		Enabled:          false,
		RotationInterval: 30 * 24 * time.Hour, // 30 days
		GracePeriod:      7 * 24 * time.Hour,  // 7 days
		AutoDecommission: true,
	}
}

// SetRotationPolicy sets the rotation policy for the manager
func (m *OAuthKeyManager) SetRotationPolicy(policy *RotationPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy == nil {
		policy = DefaultRotationPolicy()
	}

	m.rotationPolicy = policy
}

// GetRotationPolicy returns the current rotation policy
func (m *OAuthKeyManager) GetRotationPolicy() *RotationPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.rotationPolicy == nil {
		return DefaultRotationPolicy()
	}

	// Return a copy
	return &RotationPolicy{
		Enabled:          m.rotationPolicy.Enabled,
		RotationInterval: m.rotationPolicy.RotationInterval,
		GracePeriod:      m.rotationPolicy.GracePeriod,
		AutoDecommission: m.rotationPolicy.AutoDecommission,
		OnRotationNeeded: m.rotationPolicy.OnRotationNeeded,
		OnDecommission:   m.rotationPolicy.OnDecommission,
	}
}

// CheckRotationNeeded returns a list of credential IDs that need rotation
func (m *OAuthKeyManager) CheckRotationNeeded() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.rotationPolicy == nil || !m.rotationPolicy.Enabled {
		return nil
	}

	var needsRotation []string
	now := time.Now()

	for credID, state := range m.rotationState {
		// Skip if already marked for rotation
		if state.MarkedForRotation {
			continue
		}

		// Check if credential has exceeded rotation interval
		if !state.CreatedAt.IsZero() {
			age := now.Sub(state.CreatedAt)
			if age >= m.rotationPolicy.RotationInterval {
				needsRotation = append(needsRotation, credID)
			}
		}
	}

	return needsRotation
}

// MarkForRotation marks a credential for rotation and adds the new credential
func (m *OAuthKeyManager) MarkForRotation(credentialID string, newCredential *types.OAuthCredentialSet) error {
	if newCredential == nil {
		return fmt.Errorf("new credential cannot be nil")
	}

	m.mu.Lock()

	// Check if the credential exists
	state, exists := m.rotationState[credentialID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("credential %s not found", credentialID)
	}

	// Check if already marked for rotation
	if state.MarkedForRotation {
		m.mu.Unlock()
		return fmt.Errorf("credential %s is already marked for rotation", credentialID)
	}

	// Add the new credential
	m.credentials = append(m.credentials, newCredential)

	// Initialize health tracking for new credential
	m.credHealth[newCredential.ID] = &credentialHealth{
		isHealthy:   true,
		lastSuccess: time.Now(),
	}

	// Initialize metrics for new credential
	m.credMetrics[newCredential.ID] = NewCredentialMetrics()

	// Initialize rotation state for new credential
	m.rotationState[newCredential.ID] = &credentialRotationState{
		CreatedAt: time.Now(),
	}

	// Mark the old credential for rotation
	state.MarkedForRotation = true
	state.RotationStartedAt = time.Now()
	state.ReplacementID = newCredential.ID

	// Calculate decommission time
	if m.rotationPolicy != nil {
		state.DecommissionAt = time.Now().Add(m.rotationPolicy.GracePeriod)
	} else {
		state.DecommissionAt = time.Now().Add(7 * 24 * time.Hour) // Default 7 days
	}

	// Get rotation policy for callbacks (before releasing lock)
	var rotationCallback func(string) error
	if m.rotationPolicy != nil {
		rotationCallback = m.rotationPolicy.OnRotationNeeded
	}

	// Release lock before calling callbacks/notifications
	m.mu.Unlock()

	// Call the rotation callback if provided
	if rotationCallback != nil {
		if err := rotationCallback(credentialID); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: rotation callback failed for %s: %v\n", credentialID, err)
		}
	}

	// Send rotation notification
	m.notifyRotation(credentialID, newCredential.ID)

	return nil
}

// CompleteRotation removes a credential that has completed its rotation grace period
func (m *OAuthKeyManager) CompleteRotation(credentialID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if the credential exists and is marked for rotation
	state, exists := m.rotationState[credentialID]
	if !exists {
		return fmt.Errorf("credential %s not found", credentialID)
	}

	if !state.MarkedForRotation {
		return fmt.Errorf("credential %s is not marked for rotation", credentialID)
	}

	// Check if grace period has passed
	if time.Now().Before(state.DecommissionAt) {
		return fmt.Errorf("grace period for credential %s has not yet elapsed (decommission at %s)",
			credentialID, state.DecommissionAt.Format(time.RFC3339))
	}

	// Call the decommission callback if provided
	if m.rotationPolicy != nil && m.rotationPolicy.OnDecommission != nil {
		if err := m.rotationPolicy.OnDecommission(credentialID); err != nil {
			return fmt.Errorf("decommission callback failed for %s: %w", credentialID, err)
		}
	}

	// Remove the credential from the credentials slice
	newCredentials := make([]*types.OAuthCredentialSet, 0, len(m.credentials)-1)
	for _, cred := range m.credentials {
		if cred.ID != credentialID {
			newCredentials = append(newCredentials, cred)
		}
	}
	m.credentials = newCredentials

	// Clean up tracking data
	delete(m.credHealth, credentialID)
	delete(m.credMetrics, credentialID)
	delete(m.rotationState, credentialID)

	return nil
}

// AutoDecommissionExpired automatically decommissions credentials whose grace period has expired
// Returns the list of decommissioned credential IDs
func (m *OAuthKeyManager) AutoDecommissionExpired() ([]string, error) {
	if m.rotationPolicy == nil || !m.rotationPolicy.AutoDecommission {
		return nil, nil
	}

	m.mu.RLock()
	var toDecommission []string
	now := time.Now()

	for credID, state := range m.rotationState {
		if state.MarkedForRotation && !state.DecommissionAt.IsZero() && now.After(state.DecommissionAt) {
			toDecommission = append(toDecommission, credID)
		}
	}
	m.mu.RUnlock()

	// Decommission each credential
	var decommissioned []string
	var lastErr error

	for _, credID := range toDecommission {
		if err := m.CompleteRotation(credID); err != nil {
			lastErr = err
			fmt.Printf("Warning: failed to auto-decommission credential %s: %v\n", credID, err)
		} else {
			decommissioned = append(decommissioned, credID)
		}
	}

	return decommissioned, lastErr
}

// GetRotationState returns the rotation state for a specific credential
func (m *OAuthKeyManager) GetRotationState(credentialID string) *credentialRotationState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.rotationState[credentialID]
	if !exists {
		return nil
	}

	// Return a copy
	return &credentialRotationState{
		CreatedAt:         state.CreatedAt,
		MarkedForRotation: state.MarkedForRotation,
		RotationStartedAt: state.RotationStartedAt,
		ReplacementID:     state.ReplacementID,
		DecommissionAt:    state.DecommissionAt,
	}
}
