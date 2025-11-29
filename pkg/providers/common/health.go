package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// HealthChecker performs health checks on providers
type HealthChecker struct {
	interval       time.Duration
	healthStatus   map[types.ProviderType]*ProviderHealth
	mu             sync.RWMutex
	ticker         *time.Ticker
	stopChan       chan struct{}
	checkCallbacks []HealthCheckCallback
	running        bool           // tracks if the health checker is running
	wg             sync.WaitGroup // waits for goroutine to exit
}

// ProviderHealth represents the health status of a provider
type ProviderHealth struct {
	Provider     types.ProviderType     `json:"provider"`
	Healthy      bool                   `json:"healthy"`
	LastCheck    time.Time              `json:"last_check"`
	LastSuccess  time.Time              `json:"last_success"`
	LastError    time.Time              `json:"last_error"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	ResponseTime time.Duration          `json:"response_time"`
	CheckCount   int64                  `json:"check_count"`
	SuccessCount int64                  `json:"success_count"`
	FailureCount int64                  `json:"failure_count"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// HealthCheckCallback is called when a health check completes
type HealthCheckCallback func(provider types.ProviderType, health *ProviderHealth)

// HealthCheckResult represents the result of a single health check
type HealthCheckResult struct {
	Healthy      bool                   `json:"healthy"`
	ResponseTime time.Duration          `json:"response_time"`
	Error        string                 `json:"error,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(interval time.Duration) *HealthChecker {
	return &HealthChecker{
		interval:       interval,
		healthStatus:   make(map[types.ProviderType]*ProviderHealth),
		checkCallbacks: make([]HealthCheckCallback, 0),
	}
}

// Start starts the health checker
func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return // Already started
	}
	hc.running = true
	hc.ticker = time.NewTicker(hc.interval)
	hc.stopChan = make(chan struct{})
	ticker := hc.ticker
	stopChan := hc.stopChan
	hc.wg.Add(1)
	hc.mu.Unlock()

	go func() {
		defer hc.wg.Done()
		for {
			select {
			case <-ticker.C:
				hc.performAllChecks()
			case <-stopChan:
				return
			}
		}
	}()
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	if !hc.running {
		hc.mu.Unlock()
		return // Already stopped
	}
	hc.running = false
	ticker := hc.ticker
	stopChan := hc.stopChan
	hc.ticker = nil
	hc.stopChan = nil
	hc.mu.Unlock()

	if ticker != nil {
		ticker.Stop()
	}
	if stopChan != nil {
		close(stopChan)
	}
	// Wait for the goroutine to exit
	hc.wg.Wait()
}

// AddCallback adds a callback to be called when health checks complete
func (hc *HealthChecker) AddCallback(callback HealthCheckCallback) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checkCallbacks = append(hc.checkCallbacks, callback)
}

// CheckProvider performs a health check on a specific provider
func (hc *HealthChecker) CheckProvider(ctx context.Context, provider *InitializedProvider) error {
	result := hc.performHealthCheck(ctx, provider)
	hc.updateHealthStatus(provider.Type, result)

	// Call callbacks - make a copy of callbacks under lock
	hc.mu.RLock()
	callbacks := make([]HealthCheckCallback, len(hc.checkCallbacks))
	copy(callbacks, hc.checkCallbacks)
	hc.mu.RUnlock()

	for _, callback := range callbacks {
		go callback(provider.Type, hc.getHealthStatus(provider.Type))
	}

	if !result.Healthy {
		return fmt.Errorf("health check failed: %s", result.Error)
	}

	return nil
}

// performAllChecks performs health checks on all registered providers
func (hc *HealthChecker) performAllChecks() {
	hc.mu.RLock()
	providers := make([]types.ProviderType, 0, len(hc.healthStatus))
	for providerType := range hc.healthStatus {
		providers = append(providers, providerType)
	}
	hc.mu.RUnlock()

	for _, providerType := range providers {
		// This would need access to the actual provider instance
		// For now, we'll skip the actual check and just update the timestamp
		hc.mu.Lock()
		if health, exists := hc.healthStatus[providerType]; exists {
			health.LastCheck = time.Now()
			health.CheckCount++
		}
		hc.mu.Unlock()
	}
}

// performHealthCheck performs the actual health check for a provider
func (hc *HealthChecker) performHealthCheck(ctx context.Context, provider *InitializedProvider) HealthCheckResult {
	startTime := time.Now()

	endpoint, err := hc.getHealthCheckEndpoint(provider)
	if err != nil {
		return HealthCheckResult{
			Healthy:      false,
			ResponseTime: time.Since(startTime),
			Error:        err.Error(),
		}
	}

	// Create a simple GET request
	req, err := http.NewRequestBuilder("GET", endpoint).
		WithContext(ctx).
		WithHeaders(hc.getHealthCheckHeaders(provider)).
		Build()

	if err != nil {
		return HealthCheckResult{
			Healthy:      false,
			ResponseTime: time.Since(startTime),
			Error:        fmt.Sprintf("failed to create request: %v", err),
		}
	}

	// Make the request
	resp, err := provider.HTTPClient.Do(ctx, req)
	responseTime := time.Since(startTime)

	if err != nil {
		return HealthCheckResult{
			Healthy:      false,
			ResponseTime: responseTime,
			Error:        fmt.Sprintf("request failed: %v", err),
		}
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return HealthCheckResult{
			Healthy:      false,
			ResponseTime: responseTime,
			Error:        fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}

	return HealthCheckResult{
		Healthy:      true,
		ResponseTime: responseTime,
		Details: map[string]interface{}{
			"status_code": resp.StatusCode,
		},
	}
}

// getHealthCheckHeaders returns headers for health check requests
func (hc *HealthChecker) getHealthCheckHeaders(provider *InitializedProvider) map[string]string {
	headers := make(map[string]string)

	switch provider.Type {
	case types.ProviderTypeOpenAI, types.ProviderTypeOpenRouter, types.ProviderTypeCerebras:
		headers["Authorization"] = "Bearer " + provider.Config.APIKey
	case types.ProviderTypeAnthropic:
		headers["x-api-key"] = provider.Config.APIKey
		headers["anthropic-version"] = "2023-06-01"
	case types.ProviderTypeGemini:
		headers["x-goog-api-key"] = provider.Config.APIKey
	}

	return headers
}

// updateHealthStatus updates the health status for a provider
func (hc *HealthChecker) updateHealthStatus(providerType types.ProviderType, result HealthCheckResult) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	health, exists := hc.healthStatus[providerType]
	if !exists {
		health = &ProviderHealth{
			Provider: providerType,
			Details:  make(map[string]interface{}),
		}
		hc.healthStatus[providerType] = health
	}

	health.LastCheck = time.Now()
	health.ResponseTime = result.ResponseTime
	health.CheckCount++

	if result.Healthy {
		health.Healthy = true
		health.LastSuccess = time.Now()
		health.SuccessCount++
		health.ErrorMessage = ""
	} else {
		health.Healthy = false
		health.LastError = time.Now()
		health.FailureCount++
		health.ErrorMessage = result.Error
	}

	// Update details
	if result.Details != nil {
		for k, v := range result.Details {
			health.Details[k] = v
		}
	}
}

// getHealthStatus returns the current health status for a provider
func (hc *HealthChecker) getHealthStatus(providerType types.ProviderType) *ProviderHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	health, exists := hc.healthStatus[providerType]
	if !exists {
		return &ProviderHealth{
			Provider: providerType,
			Healthy:  false,
			Details:  make(map[string]interface{}),
		}
	}

	// Return a copy to avoid concurrent access
	copy := *health
	copy.Details = make(map[string]interface{})
	for k, v := range health.Details {
		copy.Details[k] = v
	}

	return &copy
}

// GetHealthStatus returns the current health status for a provider
func (hc *HealthChecker) GetHealthStatus(providerType types.ProviderType) ProviderHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	health, exists := hc.healthStatus[providerType]
	if !exists {
		return ProviderHealth{
			Provider: providerType,
			Healthy:  false,
			Details:  make(map[string]interface{}),
		}
	}

	// Return a copy to avoid concurrent access
	copy := *health
	copy.Details = make(map[string]interface{})
	for k, v := range health.Details {
		copy.Details[k] = v
	}

	return copy
}

// GetAllHealthStatus returns health status for all providers
func (hc *HealthChecker) GetAllHealthStatus() map[types.ProviderType]ProviderHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status := make(map[types.ProviderType]ProviderHealth)
	for providerType, health := range hc.healthStatus {
		copy := *health
		copy.Details = make(map[string]interface{})
		for k, v := range health.Details {
			copy.Details[k] = v
		}
		status[providerType] = copy
	}

	return status
}

// IsHealthy checks if a provider is currently healthy
func (hc *HealthChecker) IsHealthy(providerType types.ProviderType) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	health, exists := hc.healthStatus[providerType]
	if !exists {
		return false
	}

	return health.Healthy
}

// GetHealthyProviders returns a list of currently healthy providers
func (hc *HealthChecker) GetHealthyProviders() []types.ProviderType {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var healthy []types.ProviderType
	for providerType, health := range hc.healthStatus {
		if health.Healthy {
			healthy = append(healthy, providerType)
		}
	}

	return healthy
}

// GetUnhealthyProviders returns a list of currently unhealthy providers
func (hc *HealthChecker) GetUnhealthyProviders() []types.ProviderType {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var unhealthy []types.ProviderType
	for providerType, health := range hc.healthStatus {
		if !health.Healthy {
			unhealthy = append(unhealthy, providerType)
		}
	}

	return unhealthy
}

// ResetHealthStatus resets the health status for a provider
func (hc *HealthChecker) ResetHealthStatus(providerType types.ProviderType) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	delete(hc.healthStatus, providerType)
}

// GetHealthSummary returns a summary of health status across all providers
func (hc *HealthChecker) GetHealthSummary() HealthSummary {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	summary := HealthSummary{
		TotalProviders:     int64(len(hc.healthStatus)),
		HealthyProviders:   0,
		UnhealthyProviders: 0,
		LastCheckTimes:     make(map[types.ProviderType]time.Time),
	}

	for providerType, health := range hc.healthStatus {
		summary.LastCheckTimes[providerType] = health.LastCheck
		if health.Healthy {
			summary.HealthyProviders++
		} else {
			summary.UnhealthyProviders++
		}
	}

	return summary
}

// HealthSummary represents a summary of health status across providers
type HealthSummary struct {
	TotalProviders     int64                            `json:"total_providers"`
	HealthyProviders   int64                            `json:"healthy_providers"`
	UnhealthyProviders int64                            `json:"unhealthy_providers"`
	LastCheckTimes     map[types.ProviderType]time.Time `json:"last_check_times"`
}

// getHealthCheckEndpoint returns the health check endpoint for a provider
func (hc *HealthChecker) getHealthCheckEndpoint(provider *InitializedProvider) (string, error) {
	baseURL := provider.Config.BaseURL

	switch provider.Type {
	case types.ProviderTypeOpenAI:
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return baseURL + "/v1/models", nil
	case types.ProviderTypeAnthropic:
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		return baseURL + "/v1/messages", nil
	case types.ProviderTypeOpenRouter:
		if baseURL == "" {
			baseURL = "https://openrouter.ai"
		}
		return baseURL + "/api/v1/models", nil
	case types.ProviderTypeCerebras:
		if baseURL == "" {
			baseURL = "https://api.cerebras.ai"
		}
		return baseURL + "/v1/models", nil
	case types.ProviderTypeGemini:
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com"
		}
		return baseURL + "/v1/models", nil
	default:
		return "", fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}
