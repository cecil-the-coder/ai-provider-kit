package oauthmanager

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OAuthKeyManager manages multiple OAuth credentials with load balancing and failover
type OAuthKeyManager struct {
	providerName string
	credentials  []*types.OAuthCredentialSet
	currentIndex uint32 // Atomic counter for round-robin
	credHealth   map[string]*credentialHealth

	// Provider-specific token refresh function
	refreshFunc RefreshFunc

	// Track ongoing refresh operations to prevent duplicates
	refreshInFlight map[string]bool

	// Phase 4: Advanced features
	credMetrics      map[string]*CredentialMetrics       // Credential-level metrics
	refreshStrategy  *RefreshStrategy                    // Smart refresh configuration
	rotationPolicy   *RotationPolicy                     // Token rotation policy
	rotationState    map[string]*credentialRotationState // Rotation state per credential
	monitoringConfig *MonitoringConfig                   // Monitoring and alerting config
	alertHistory     *alertHistory                       // Alert tracking to prevent spam

	mu sync.RWMutex
}

// NewOAuthKeyManager creates a new OAuth key manager with multiple credentials
// The refreshFunc parameter is a provider-specific function that refreshes OAuth tokens
// If no refresh is needed, pass NoOpRefreshFunc
func NewOAuthKeyManager(providerName string, credentials []*types.OAuthCredentialSet, refreshFunc RefreshFunc) *OAuthKeyManager {
	if len(credentials) == 0 {
		return nil
	}

	// Use NoOpRefreshFunc if no refresh function provided
	if refreshFunc == nil {
		refreshFunc = NoOpRefreshFunc
	}

	manager := &OAuthKeyManager{
		providerName:     providerName,
		credentials:      credentials,
		currentIndex:     0,
		credHealth:       make(map[string]*credentialHealth),
		refreshFunc:      refreshFunc,
		refreshInFlight:  make(map[string]bool),
		credMetrics:      make(map[string]*CredentialMetrics),
		refreshStrategy:  DefaultRefreshStrategy(),
		rotationPolicy:   DefaultRotationPolicy(),
		rotationState:    make(map[string]*credentialRotationState),
		monitoringConfig: nil, // Not enabled by default
		alertHistory: &alertHistory{
			lastAlerts:    make(map[string]time.Time),
			alertCooldown: 1 * time.Hour,
		},
	}

	// Initialize health tracking, metrics, and rotation state for all credentials
	for _, cred := range credentials {
		manager.credHealth[cred.ID] = &credentialHealth{
			isHealthy:   true,
			lastSuccess: time.Now(),
		}
		manager.credMetrics[cred.ID] = NewCredentialMetrics()
		manager.rotationState[cred.ID] = &credentialRotationState{
			CreatedAt: time.Now(),
		}
	}

	return manager
}

// ExecuteWithFailover attempts an operation with automatic failover to next credential on failure
// Automatically refreshes tokens if they are expired or expiring soon
func (m *OAuthKeyManager) ExecuteWithFailover(
	ctx context.Context,
	operation func(context.Context, *types.OAuthCredentialSet) (string, *types.Usage, error),
) (string, *types.Usage, error) {
	if len(m.credentials) == 0 {
		return "", nil, fmt.Errorf("no OAuth credentials configured for %s", m.providerName)
	}

	var lastErr error
	attemptsLimit := min(len(m.credentials), 3) // Try up to 3 credentials or all, whichever is less

	for attempt := 0; attempt < attemptsLimit; attempt++ {
		// Get next available credential
		cred, err := m.GetNextCredential(ctx)
		if err != nil {
			// All credentials exhausted or unavailable
			if lastErr != nil {
				return "", nil, fmt.Errorf("%s: all OAuth credentials failed, last error: %w", m.providerName, lastErr)
			}
			return "", nil, err
		}

		// Check if token needs refresh using the refresh strategy
		m.mu.RLock()
		strategy := m.refreshStrategy
		metrics := m.credMetrics[cred.ID]
		m.mu.RUnlock()

		if strategy != nil && strategy.ShouldRefresh(cred, metrics) {
			refreshed, err := m.refreshCredential(ctx, cred)
			if err != nil {
				m.ReportFailure(cred.ID, err)
				lastErr = fmt.Errorf("token refresh failed: %w", err)
				continue
			}
			// Use the refreshed credential for the operation
			cred = refreshed
		}

		// Execute the operation and track timing
		startTime := time.Now()
		result, usage, err := operation(ctx, cred)
		latency := time.Since(startTime)

		// Calculate tokens used (if usage is provided)
		var tokensUsed int64
		if usage != nil {
			tokensUsed = int64(usage.PromptTokens + usage.CompletionTokens)
		}

		if err != nil {
			lastErr = err
			m.ReportFailure(cred.ID, err)
			// Record failed request metrics
			m.RecordRequest(cred.ID, tokensUsed, latency, false)
			continue
		}

		// Success!
		m.ReportSuccess(cred.ID)
		// Record successful request metrics
		m.RecordRequest(cred.ID, tokensUsed, latency, true)
		return result, usage, nil
	}

	// All attempts failed
	return "", nil, fmt.Errorf("%s: all %d OAuth credential failover attempts failed, last error: %w",
		m.providerName, attemptsLimit, lastErr)
}

// GetNextCredential returns the next available OAuth credential using round-robin load balancing
func (m *OAuthKeyManager) GetNextCredential(ctx context.Context) (*types.OAuthCredentialSet, error) {
	if len(m.credentials) == 0 {
		return nil, fmt.Errorf("no OAuth credentials configured for %s", m.providerName)
	}

	// Single credential fast path
	if len(m.credentials) == 1 {
		m.mu.RLock()
		health := m.credHealth[m.credentials[0].ID]
		available := health.isCredentialAvailable()
		m.mu.RUnlock()

		if available {
			return m.credentials[0], nil
		}
		return nil, fmt.Errorf("only OAuth credential for %s is unavailable (in backoff)", m.providerName)
	}

	// Try all credentials in round-robin order
	startIndex := atomic.AddUint32(&m.currentIndex, 1) % uint32(len(m.credentials)) // #nosec G115 -- len() returns non-negative int, safe conversion

	for i := 0; i < len(m.credentials); i++ {
		index := (int(startIndex) + i) % len(m.credentials)

		m.mu.RLock()
		cred := m.credentials[index]
		health := m.credHealth[cred.ID]
		available := health.isCredentialAvailable()
		m.mu.RUnlock()

		if available {
			return cred, nil
		}
	}

	return nil, fmt.Errorf("all %d OAuth credentials for %s are currently unavailable", len(m.credentials), m.providerName)
}

// ReportSuccess reports that an API call succeeded with this credential
func (m *OAuthKeyManager) ReportSuccess(credentialID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.credHealth[credentialID]
	if !exists {
		return
	}

	health.recordSuccess()
}

// ReportFailure reports that an API call failed with this credential
func (m *OAuthKeyManager) ReportFailure(credentialID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	health, exists := m.credHealth[credentialID]
	if !exists {
		return
	}

	health.recordFailure()
}

// GetCredentials returns a copy of the credentials slice
// Returns clones to prevent external modification
func (m *OAuthKeyManager) GetCredentials() []*types.OAuthCredentialSet {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	clones := make([]*types.OAuthCredentialSet, len(m.credentials))
	for i, cred := range m.credentials {
		clones[i] = Clone(cred)
	}
	return clones
}

// GetCredentialHealth returns the health status of a specific credential
// Returns nil if credential ID not found
func (m *OAuthKeyManager) GetCredentialHealth(credentialID string) *credentialHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, exists := m.credHealth[credentialID]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	return &credentialHealth{
		failureCount:     health.failureCount,
		lastFailure:      health.lastFailure,
		lastSuccess:      health.lastSuccess,
		isHealthy:        health.isHealthy,
		backoffUntil:     health.backoffUntil,
		lastRefresh:      health.lastRefresh,
		refreshInFlight:  health.refreshInFlight,
		lastRefreshError: health.lastRefreshError,
		refreshFailCount: health.refreshFailCount,
	}
}

// refreshCredential refreshes an expired OAuth token with thread-safe coordination
// Prevents duplicate refresh operations for the same credential
func (m *OAuthKeyManager) refreshCredential(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
	m.mu.Lock()

	// Check if refresh already in flight for this credential
	if m.refreshInFlight[cred.ID] {
		m.mu.Unlock()
		// Wait briefly and retry getting credential (it may be refreshed by now)
		time.Sleep(100 * time.Millisecond)
		return nil, fmt.Errorf("refresh already in progress for credential %s", cred.ID)
	}

	// Mark refresh as in-flight
	m.refreshInFlight[cred.ID] = true
	m.mu.Unlock()

	// Always clear in-flight flag
	defer func() {
		m.mu.Lock()
		delete(m.refreshInFlight, cred.ID)
		m.mu.Unlock()
	}()

	// Perform the actual refresh (outside lock to avoid blocking)
	refreshed, err := m.refreshFunc(ctx, cred)
	if err != nil {
		// Update health tracking with refresh failure
		m.mu.Lock()
		if health, exists := m.credHealth[cred.ID]; exists {
			health.recordRefreshFailure(err)
		}
		m.mu.Unlock()

		// Send refresh failure notification
		m.notifyRefreshFailure(cred.ID, err)

		return nil, fmt.Errorf("failed to refresh credential %s: %w", cred.ID, err)
	}

	// Update the credential in the manager
	m.updateCredential(refreshed)

	// Update health tracking with successful refresh
	m.mu.Lock()
	if health, exists := m.credHealth[cred.ID]; exists {
		health.recordRefreshSuccess()
	}
	// Update metrics with successful refresh
	if metrics, exists := m.credMetrics[cred.ID]; exists {
		metrics.recordRefresh()
	}
	m.mu.Unlock()

	// Send refresh success notification
	m.notifyRefreshSuccess(cred.ID)

	// Call the callback if provided
	if refreshed.OnTokenRefresh != nil {
		if err := refreshed.OnTokenRefresh(
			refreshed.ID,
			refreshed.AccessToken,
			refreshed.RefreshToken,
			refreshed.ExpiresAt,
		); err != nil {
			// Log but don't fail - token is still valid in memory
			fmt.Printf("Warning: failed to persist refreshed token for %s: %v\n", refreshed.ID, err)
		}
	}

	return refreshed, nil
}

// ExecuteWithFailoverMessage attempts an operation with automatic failover (message variant)
// Returns full ChatMessage to preserve tool calls
func (m *OAuthKeyManager) ExecuteWithFailoverMessage(
	ctx context.Context,
	operation func(context.Context, *types.OAuthCredentialSet) (types.ChatMessage, *types.Usage, error),
) (types.ChatMessage, *types.Usage, error) {
	if len(m.credentials) == 0 {
		return types.ChatMessage{}, nil, fmt.Errorf("no OAuth credentials configured for %s", m.providerName)
	}

	var lastErr error
	attemptsLimit := min(len(m.credentials), 3) // Try up to 3 credentials or all, whichever is less

	for attempt := 0; attempt < attemptsLimit; attempt++ {
		// Get next available credential
		cred, err := m.GetNextCredential(ctx)
		if err != nil {
			// All credentials exhausted or unavailable
			if lastErr != nil {
				return types.ChatMessage{}, nil, fmt.Errorf("%s: all OAuth credentials failed, last error: %w", m.providerName, lastErr)
			}
			return types.ChatMessage{}, nil, err
		}

		// Check if token needs refresh using the refresh strategy
		m.mu.RLock()
		strategy := m.refreshStrategy
		metrics := m.credMetrics[cred.ID]
		m.mu.RUnlock()

		if strategy != nil && strategy.ShouldRefresh(cred, metrics) {
			refreshed, err := m.refreshCredential(ctx, cred)
			if err != nil {
				m.ReportFailure(cred.ID, err)
				lastErr = fmt.Errorf("token refresh failed: %w", err)
				continue
			}
			// Use the refreshed credential for the operation
			cred = refreshed
		}

		// Execute the operation and track timing
		startTime := time.Now()
		result, usage, err := operation(ctx, cred)
		latency := time.Since(startTime)

		// Calculate tokens used (if usage is provided)
		var tokensUsed int64
		if usage != nil {
			tokensUsed = int64(usage.PromptTokens + usage.CompletionTokens)
		}

		if err != nil {
			lastErr = err
			m.ReportFailure(cred.ID, err)
			// Record failed request metrics
			m.RecordRequest(cred.ID, tokensUsed, latency, false)
			continue
		}

		// Success!
		m.ReportSuccess(cred.ID)
		// Record successful request metrics
		m.RecordRequest(cred.ID, tokensUsed, latency, true)
		return result, usage, nil
	}

	// All attempts failed
	return types.ChatMessage{}, nil, fmt.Errorf("%s: all %d OAuth credential failover attempts failed, last error: %w",
		m.providerName, attemptsLimit, lastErr)
}

// updateCredential updates a credential after token refresh
func (m *OAuthKeyManager) updateCredential(updated *types.OAuthCredentialSet) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, cred := range m.credentials {
		if cred.ID == updated.ID {
			m.credentials[i] = updated
			break
		}
	}
}
