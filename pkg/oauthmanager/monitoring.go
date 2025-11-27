package oauthmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// MonitoringConfig defines monitoring and alerting configuration
type MonitoringConfig struct {
	// Webhooks
	WebhookURL    string   // URL for webhook notifications
	WebhookEvents []string // Events to notify: "refresh", "failure", "rotation", "expiry_warning"

	// Alerting
	AlertOnHighFailureRate bool    // Alert if error rate > threshold
	FailureRateThreshold   float64 // Default: 0.25 (25%)

	AlertOnRefreshFailure bool          // Alert on refresh failures
	AlertOnExpirySoon     bool          // Alert when tokens expire soon
	ExpiryWarningTime     time.Duration // Default: 24 hours
}

// WebhookEvent represents an event to be sent via webhook
type WebhookEvent struct {
	Type         string                 `json:"type"` // "refresh", "failure", "rotation", "expiry_warning"
	CredentialID string                 `json:"credential_id"`
	Timestamp    time.Time              `json:"timestamp"`
	Message      string                 `json:"message"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// alertHistory tracks when alerts were last sent to prevent spam
type alertHistory struct {
	mu            sync.RWMutex
	lastAlerts    map[string]time.Time // key: "type:credentialID"
	alertCooldown time.Duration        // Minimum time between same alerts
}

// DefaultMonitoringConfig returns a default monitoring configuration
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		WebhookURL:             "",
		WebhookEvents:          []string{"refresh", "failure", "rotation", "expiry_warning"},
		AlertOnHighFailureRate: true,
		FailureRateThreshold:   0.25, // 25%
		AlertOnRefreshFailure:  true,
		AlertOnExpirySoon:      true,
		ExpiryWarningTime:      24 * time.Hour,
	}
}

// SetMonitoringConfig sets the monitoring configuration for the manager
func (m *OAuthKeyManager) SetMonitoringConfig(config *MonitoringConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config == nil {
		config = DefaultMonitoringConfig()
	}

	m.monitoringConfig = config

	// Initialize alert history if needed
	if m.alertHistory == nil {
		m.alertHistory = &alertHistory{
			lastAlerts:    make(map[string]time.Time),
			alertCooldown: 1 * time.Hour, // Default cooldown
		}
	}
}

// GetMonitoringConfig returns the current monitoring configuration
func (m *OAuthKeyManager) GetMonitoringConfig() *MonitoringConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.monitoringConfig == nil {
		return DefaultMonitoringConfig()
	}

	// Return a copy
	eventsCopy := make([]string, len(m.monitoringConfig.WebhookEvents))
	copy(eventsCopy, m.monitoringConfig.WebhookEvents)

	return &MonitoringConfig{
		WebhookURL:             m.monitoringConfig.WebhookURL,
		WebhookEvents:          eventsCopy,
		AlertOnHighFailureRate: m.monitoringConfig.AlertOnHighFailureRate,
		FailureRateThreshold:   m.monitoringConfig.FailureRateThreshold,
		AlertOnRefreshFailure:  m.monitoringConfig.AlertOnRefreshFailure,
		AlertOnExpirySoon:      m.monitoringConfig.AlertOnExpirySoon,
		ExpiryWarningTime:      m.monitoringConfig.ExpiryWarningTime,
	}
}

// sendWebhook sends a webhook notification
func (m *OAuthKeyManager) sendWebhook(event *WebhookEvent) error {
	m.mu.RLock()
	config := m.monitoringConfig
	m.mu.RUnlock()

	if config == nil || config.WebhookURL == "" {
		return nil // Webhooks not configured
	}

	// Check if this event type should be sent
	eventEnabled := false
	for _, enabledType := range config.WebhookEvents {
		if enabledType == event.Type {
			eventEnabled = true
			break
		}
	}

	if !eventEnabled {
		return nil
	}

	// Marshal the event
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook event: %w", err)
	}

	// Send the webhook (asynchronously to not block operations)
	go func() {
		resp, err := http.Post(config.WebhookURL, "application/json", bytes.NewReader(eventJSON))
		if err != nil {
			fmt.Printf("Warning: failed to send webhook: %v\n", err)
			return
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				// Log the error if a logger is available, or ignore it
				_ = fmt.Sprintf("failed to close webhook response body: %v", err)
			}
		}()

		if resp.StatusCode >= 400 {
			fmt.Printf("Warning: webhook returned status %d\n", resp.StatusCode)
		}
	}()

	return nil
}

// shouldSendAlert checks if an alert should be sent based on cooldown
func (h *alertHistory) shouldSendAlert(alertKey string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	lastSent, exists := h.lastAlerts[alertKey]
	if !exists {
		return true
	}

	return time.Since(lastSent) >= h.alertCooldown
}

// recordAlert records that an alert was sent
func (h *alertHistory) recordAlert(alertKey string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastAlerts[alertKey] = time.Now()
}

// CheckAlerts checks for conditions that should trigger alerts
// Returns a list of alerts that were generated
func (m *OAuthKeyManager) CheckAlerts() []WebhookEvent {
	m.mu.RLock()
	config := m.monitoringConfig
	m.mu.RUnlock()

	if config == nil {
		return nil
	}

	alerts := make([]WebhookEvent, 0, 10) // Estimate capacity

	// Check high failure rates
	if config.AlertOnHighFailureRate {
		alerts = append(alerts, m.checkFailureRateAlerts()...)
	}

	// Check expiring credentials
	if config.AlertOnExpirySoon {
		alerts = append(alerts, m.checkExpiryAlerts()...)
	}

	// Send webhooks for alerts
	for i := range alerts {
		if err := m.sendWebhook(&alerts[i]); err != nil {
			fmt.Printf("Warning: failed to send alert webhook: %v\n", err)
		}
	}

	return alerts
}

// checkFailureRateAlerts checks for credentials with high failure rates
func (m *OAuthKeyManager) checkFailureRateAlerts() []WebhookEvent {
	m.mu.RLock()
	config := m.monitoringConfig
	metrics := m.credMetrics
	m.mu.RUnlock()

	if config == nil {
		return nil
	}

	alertCount := len(metrics)
	alerts := make([]WebhookEvent, 0, alertCount) // Pre-allocate based on metrics size

	for credID, credMetrics := range metrics {
		successRate := credMetrics.GetSuccessRate()
		failureRate := 1.0 - successRate

		// Skip if failure rate is below threshold
		if failureRate <= config.FailureRateThreshold {
			continue
		}

		// Skip if we have too few requests to make a determination
		snapshot := credMetrics.GetSnapshot()
		if snapshot.RequestCount < 10 {
			continue
		}

		// Check alert cooldown
		alertKey := fmt.Sprintf("failure:%s", credID)
		if m.alertHistory != nil && !m.alertHistory.shouldSendAlert(alertKey) {
			continue
		}

		// Create alert
		alert := WebhookEvent{
			Type:         "failure",
			CredentialID: credID,
			Timestamp:    time.Now(),
			Message:      fmt.Sprintf("High failure rate detected: %.1f%%", failureRate*100),
			Details: map[string]interface{}{
				"failure_rate":   failureRate,
				"success_rate":   successRate,
				"total_requests": snapshot.RequestCount,
				"error_count":    snapshot.ErrorCount,
			},
		}

		alerts = append(alerts, alert)

		// Record alert
		if m.alertHistory != nil {
			m.alertHistory.recordAlert(alertKey)
		}
	}

	return alerts
}

// checkExpiryAlerts checks for credentials that will expire soon
func (m *OAuthKeyManager) checkExpiryAlerts() []WebhookEvent {
	m.mu.RLock()
	config := m.monitoringConfig
	credentials := m.credentials
	m.mu.RUnlock()

	if config == nil {
		return nil
	}

	alerts := make([]WebhookEvent, 0, len(credentials)) // Pre-allocate based on credentials count
	warningTime := config.ExpiryWarningTime
	if warningTime == 0 {
		warningTime = 24 * time.Hour
	}

	now := time.Now()

	for _, cred := range credentials {
		// Skip if no expiry set
		if cred.ExpiresAt.IsZero() {
			continue
		}

		// Check if expires within warning time
		timeUntilExpiry := cred.ExpiresAt.Sub(now)
		if timeUntilExpiry <= 0 || timeUntilExpiry > warningTime {
			continue
		}

		// Check alert cooldown
		alertKey := fmt.Sprintf("expiry_warning:%s", cred.ID)
		if m.alertHistory != nil && !m.alertHistory.shouldSendAlert(alertKey) {
			continue
		}

		// Create alert
		alert := WebhookEvent{
			Type:         "expiry_warning",
			CredentialID: cred.ID,
			Timestamp:    time.Now(),
			Message:      fmt.Sprintf("Credential expires in %s", timeUntilExpiry.Round(time.Minute)),
			Details: map[string]interface{}{
				"expires_at":        cred.ExpiresAt,
				"time_until_expiry": timeUntilExpiry.String(),
				"hours_remaining":   timeUntilExpiry.Hours(),
			},
		}

		alerts = append(alerts, alert)

		// Record alert
		if m.alertHistory != nil {
			m.alertHistory.recordAlert(alertKey)
		}
	}

	return alerts
}

// notifyRefreshSuccess sends a notification when a token is refreshed
func (m *OAuthKeyManager) notifyRefreshSuccess(credentialID string) {
	event := &WebhookEvent{
		Type:         "refresh",
		CredentialID: credentialID,
		Timestamp:    time.Now(),
		Message:      "Token successfully refreshed",
		Details: map[string]interface{}{
			"success": true,
		},
	}

	if err := m.sendWebhook(event); err != nil {
		fmt.Printf("Warning: failed to send refresh notification: %v\n", err)
	}
}

// notifyRefreshFailure sends a notification when a token refresh fails
func (m *OAuthKeyManager) notifyRefreshFailure(credentialID string, err error) {
	m.mu.RLock()
	config := m.monitoringConfig
	m.mu.RUnlock()

	if config == nil || !config.AlertOnRefreshFailure {
		return
	}

	// Check alert cooldown
	alertKey := fmt.Sprintf("refresh_failure:%s", credentialID)
	if m.alertHistory != nil && !m.alertHistory.shouldSendAlert(alertKey) {
		return
	}

	event := &WebhookEvent{
		Type:         "failure",
		CredentialID: credentialID,
		Timestamp:    time.Now(),
		Message:      "Token refresh failed",
		Details: map[string]interface{}{
			"error": err.Error(),
		},
	}

	if err := m.sendWebhook(event); err != nil {
		fmt.Printf("Warning: failed to send refresh failure notification: %v\n", err)
	}

	// Record alert
	if m.alertHistory != nil {
		m.alertHistory.recordAlert(alertKey)
	}
}

// notifyRotation sends a notification when credential rotation occurs
func (m *OAuthKeyManager) notifyRotation(credentialID, replacementID string) {
	event := &WebhookEvent{
		Type:         "rotation",
		CredentialID: credentialID,
		Timestamp:    time.Now(),
		Message:      "Credential marked for rotation",
		Details: map[string]interface{}{
			"replacement_id": replacementID,
		},
	}

	if err := m.sendWebhook(event); err != nil {
		fmt.Printf("Warning: failed to send rotation notification: %v\n", err)
	}
}
