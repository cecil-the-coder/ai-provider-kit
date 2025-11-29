package common

import (
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewHealthChecker(t *testing.T) {
	interval := 5 * time.Minute
	checker := NewHealthChecker(interval)

	if checker == nil {
		t.Fatal("expected non-nil checker")
	}

	if checker.interval != interval {
		t.Errorf("got interval %v, expected %v", checker.interval, interval)
	}

	if checker.healthStatus == nil {
		t.Error("expected healthStatus map to be initialized")
	}

	if checker.checkCallbacks == nil {
		t.Error("expected checkCallbacks to be initialized")
	}
}

func TestHealthChecker_Start_Stop(t *testing.T) {
	checker := NewHealthChecker(100 * time.Millisecond)

	// Start should not crash
	checker.Start()

	// Starting again should be idempotent
	checker.Start()

	// Give it time to run
	time.Sleep(150 * time.Millisecond)

	// Stop should not crash
	checker.Stop()

	// Stopping again should be idempotent
	checker.Stop()
}

func TestHealthChecker_AddCallback(t *testing.T) {
	checker := NewHealthChecker(time.Minute)

	callbackCalled := false
	callback := func(provider types.ProviderType, health *ProviderHealth) {
		callbackCalled = true
	}

	checker.AddCallback(callback)

	if len(checker.checkCallbacks) != 1 {
		t.Errorf("expected 1 callback, got %d", len(checker.checkCallbacks))
	}

	// Call the callback to verify it works
	checker.checkCallbacks[0](types.ProviderTypeOpenAI, &ProviderHealth{})

	if !callbackCalled {
		t.Error("expected callback to be called")
	}
}

func TestHealthChecker_GetHealthStatus(t *testing.T) {
	checker := NewHealthChecker(time.Minute)

	// Non-existent provider
	status := checker.GetHealthStatus(types.ProviderTypeOpenAI)

	if status.Provider != types.ProviderTypeOpenAI {
		t.Errorf("got provider %s, expected %s", status.Provider, types.ProviderTypeOpenAI)
	}

	if status.Healthy {
		t.Error("expected unhealthy for non-existent provider")
	}

	// Set up a status manually
	checker.healthStatus[types.ProviderTypeOpenAI] = &ProviderHealth{
		Provider:   types.ProviderTypeOpenAI,
		Healthy:    true,
		LastCheck:  time.Now(),
		CheckCount: 5,
		Details:    map[string]interface{}{"test": "value"},
	}

	status = checker.GetHealthStatus(types.ProviderTypeOpenAI)

	if !status.Healthy {
		t.Error("expected healthy status")
	}

	if status.CheckCount != 5 {
		t.Errorf("got check count %d, expected 5", status.CheckCount)
	}

	// Verify it's a copy (not a reference)
	status.Healthy = false
	originalStatus := checker.GetHealthStatus(types.ProviderTypeOpenAI)
	if !originalStatus.Healthy {
		t.Error("expected original status to remain unchanged")
	}
}

func TestHealthChecker_GetAllHealthStatus(t *testing.T) {
	checker := NewHealthChecker(time.Minute)

	// Set up multiple statuses
	checker.healthStatus[types.ProviderTypeOpenAI] = &ProviderHealth{
		Provider: types.ProviderTypeOpenAI,
		Healthy:  true,
		Details:  make(map[string]interface{}),
	}

	checker.healthStatus[types.ProviderTypeAnthropic] = &ProviderHealth{
		Provider: types.ProviderTypeAnthropic,
		Healthy:  false,
		Details:  make(map[string]interface{}),
	}

	allStatus := checker.GetAllHealthStatus()

	if len(allStatus) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(allStatus))
	}

	openAIStatus, exists := allStatus[types.ProviderTypeOpenAI]
	if !exists {
		t.Fatal("expected OpenAI status to exist")
	}

	if !openAIStatus.Healthy {
		t.Error("expected OpenAI to be healthy")
	}

	anthropicStatus, exists := allStatus[types.ProviderTypeAnthropic]
	if !exists {
		t.Fatal("expected Anthropic status to exist")
	}

	if anthropicStatus.Healthy {
		t.Error("expected Anthropic to be unhealthy")
	}
}

func TestHealthChecker_IsHealthy(t *testing.T) {
	checker := NewHealthChecker(time.Minute)

	// Non-existent provider
	if checker.IsHealthy(types.ProviderTypeOpenAI) {
		t.Error("expected unhealthy for non-existent provider")
	}

	// Add healthy provider
	checker.healthStatus[types.ProviderTypeOpenAI] = &ProviderHealth{
		Provider: types.ProviderTypeOpenAI,
		Healthy:  true,
		Details:  make(map[string]interface{}),
	}

	if !checker.IsHealthy(types.ProviderTypeOpenAI) {
		t.Error("expected healthy provider")
	}

	// Add unhealthy provider
	checker.healthStatus[types.ProviderTypeAnthropic] = &ProviderHealth{
		Provider: types.ProviderTypeAnthropic,
		Healthy:  false,
		Details:  make(map[string]interface{}),
	}

	if checker.IsHealthy(types.ProviderTypeAnthropic) {
		t.Error("expected unhealthy provider")
	}
}

func TestHealthChecker_GetHealthyProviders(t *testing.T) {
	testGetProvidersByHealthStatus(t, true, []types.ProviderType{
		types.ProviderTypeOpenAI,
		types.ProviderTypeGemini,
	})
}

func TestHealthChecker_GetUnhealthyProviders(t *testing.T) {
	testGetProvidersByHealthStatus(t, false, []types.ProviderType{
		types.ProviderTypeAnthropic,
		types.ProviderTypeCerebras,
	})
}

// testGetProvidersByHealthStatus is a helper function to test both GetHealthyProviders and GetUnhealthyProviders
func testGetProvidersByHealthStatus(t *testing.T, healthy bool, expectedProviders []types.ProviderType) {
	t.Helper()
	checker := NewHealthChecker(time.Minute)

	// Set up mixed statuses - always use the same providers for consistency
	checker.healthStatus[types.ProviderTypeOpenAI] = &ProviderHealth{
		Provider: types.ProviderTypeOpenAI,
		Healthy:  true,
		Details:  make(map[string]interface{}),
	}

	checker.healthStatus[types.ProviderTypeAnthropic] = &ProviderHealth{
		Provider: types.ProviderTypeAnthropic,
		Healthy:  false,
		Details:  make(map[string]interface{}),
	}

	if healthy {
		checker.healthStatus[types.ProviderTypeGemini] = &ProviderHealth{
			Provider: types.ProviderTypeGemini,
			Healthy:  true,
			Details:  make(map[string]interface{}),
		}
	} else {
		checker.healthStatus[types.ProviderTypeCerebras] = &ProviderHealth{
			Provider: types.ProviderTypeCerebras,
			Healthy:  false,
			Details:  make(map[string]interface{}),
		}
	}

	// Get providers based on health status
	var providers []types.ProviderType
	if healthy {
		providers = checker.GetHealthyProviders()
	} else {
		providers = checker.GetUnhealthyProviders()
	}

	// Verify count
	if len(providers) != len(expectedProviders) {
		t.Errorf("expected %d providers, got %d", len(expectedProviders), len(providers))
	}

	// Verify all expected providers are in the list
	for _, expected := range expectedProviders {
		found := false
		for _, provider := range providers {
			if provider == expected {
				found = true
				break
			}
		}
		if !found {
			statusStr := "healthy"
			if !healthy {
				statusStr = "unhealthy"
			}
			t.Errorf("expected %s in %s providers", expected, statusStr)
		}
	}
}

func TestHealthChecker_ResetHealthStatus(t *testing.T) {
	checker := NewHealthChecker(time.Minute)

	// Add a status
	checker.healthStatus[types.ProviderTypeOpenAI] = &ProviderHealth{
		Provider: types.ProviderTypeOpenAI,
		Healthy:  true,
		Details:  make(map[string]interface{}),
	}

	// Verify it exists
	if !checker.IsHealthy(types.ProviderTypeOpenAI) {
		t.Fatal("expected provider to be healthy before reset")
	}

	// Reset
	checker.ResetHealthStatus(types.ProviderTypeOpenAI)

	// Verify it's gone
	if checker.IsHealthy(types.ProviderTypeOpenAI) {
		t.Error("expected provider to be unhealthy after reset")
	}

	status := checker.GetHealthStatus(types.ProviderTypeOpenAI)
	if status.CheckCount != 0 {
		t.Error("expected reset status to have zero counts")
	}
}

func TestHealthChecker_GetHealthSummary(t *testing.T) {
	checker := NewHealthChecker(time.Minute)

	// Set up mixed statuses
	checker.healthStatus[types.ProviderTypeOpenAI] = &ProviderHealth{
		Provider:  types.ProviderTypeOpenAI,
		Healthy:   true,
		LastCheck: time.Now(),
		Details:   make(map[string]interface{}),
	}

	checker.healthStatus[types.ProviderTypeAnthropic] = &ProviderHealth{
		Provider:  types.ProviderTypeAnthropic,
		Healthy:   false,
		LastCheck: time.Now(),
		Details:   make(map[string]interface{}),
	}

	checker.healthStatus[types.ProviderTypeGemini] = &ProviderHealth{
		Provider:  types.ProviderTypeGemini,
		Healthy:   true,
		LastCheck: time.Now(),
		Details:   make(map[string]interface{}),
	}

	summary := checker.GetHealthSummary()

	if summary.TotalProviders != 3 {
		t.Errorf("expected 3 total providers, got %d", summary.TotalProviders)
	}

	if summary.HealthyProviders != 2 {
		t.Errorf("expected 2 healthy providers, got %d", summary.HealthyProviders)
	}

	if summary.UnhealthyProviders != 1 {
		t.Errorf("expected 1 unhealthy provider, got %d", summary.UnhealthyProviders)
	}

	if len(summary.LastCheckTimes) != 3 {
		t.Errorf("expected 3 last check times, got %d", len(summary.LastCheckTimes))
	}
}

func TestProviderHealth_Structure(t *testing.T) {
	now := time.Now()
	health := &ProviderHealth{
		Provider:     types.ProviderTypeOpenAI,
		Healthy:      true,
		LastCheck:    now,
		LastSuccess:  now,
		ResponseTime: 100 * time.Millisecond,
		CheckCount:   10,
		SuccessCount: 9,
		FailureCount: 1,
		Details:      map[string]interface{}{"test": "value"},
	}

	if health.Provider != types.ProviderTypeOpenAI {
		t.Errorf("got provider %s, expected OpenAI", health.Provider)
	}

	if !health.Healthy {
		t.Error("expected healthy status")
	}

	if health.CheckCount != 10 {
		t.Errorf("got check count %d, expected 10", health.CheckCount)
	}

	if health.SuccessCount != 9 {
		t.Errorf("got success count %d, expected 9", health.SuccessCount)
	}

	if health.FailureCount != 1 {
		t.Errorf("got failure count %d, expected 1", health.FailureCount)
	}

	if health.ResponseTime != 100*time.Millisecond {
		t.Errorf("got response time %v, expected 100ms", health.ResponseTime)
	}

	if testVal, ok := health.Details["test"]; !ok || testVal != "value" {
		t.Error("expected details to be preserved")
	}
}

func TestHealthCheckResult_Structure(t *testing.T) {
	result := HealthCheckResult{
		Healthy:      true,
		ResponseTime: 50 * time.Millisecond,
		Details: map[string]interface{}{
			"status_code": 200,
		},
	}

	if !result.Healthy {
		t.Error("expected healthy result")
	}

	if result.ResponseTime != 50*time.Millisecond {
		t.Errorf("got response time %v, expected 50ms", result.ResponseTime)
	}

	if result.Error != "" {
		t.Errorf("expected empty error, got %q", result.Error)
	}

	if statusCode, ok := result.Details["status_code"]; !ok || statusCode != 200 {
		t.Error("expected status code in details")
	}
}

func TestHealthSummary_Structure(t *testing.T) {
	summary := HealthSummary{
		TotalProviders:     3,
		HealthyProviders:   2,
		UnhealthyProviders: 1,
		LastCheckTimes: map[types.ProviderType]time.Time{
			types.ProviderTypeOpenAI: time.Now(),
		},
	}

	if summary.TotalProviders != 3 {
		t.Errorf("got %d total providers, expected 3", summary.TotalProviders)
	}

	if summary.HealthyProviders != 2 {
		t.Errorf("got %d healthy providers, expected 2", summary.HealthyProviders)
	}

	if summary.UnhealthyProviders != 1 {
		t.Errorf("got %d unhealthy providers, expected 1", summary.UnhealthyProviders)
	}

	if len(summary.LastCheckTimes) != 1 {
		t.Errorf("expected 1 last check time, got %d", len(summary.LastCheckTimes))
	}
}

func TestHealthChecker_GetHealthCheckEndpoint(t *testing.T) {
	checker := NewHealthChecker(time.Minute)

	tests := []struct {
		providerType types.ProviderType
		baseURL      string
		expected     string
	}{
		{
			providerType: types.ProviderTypeOpenAI,
			baseURL:      "",
			expected:     "https://api.openai.com/v1/models",
		},
		{
			providerType: types.ProviderTypeOpenAI,
			baseURL:      "https://custom.com",
			expected:     "https://custom.com/v1/models",
		},
		{
			providerType: types.ProviderTypeAnthropic,
			baseURL:      "",
			expected:     "https://api.anthropic.com/v1/messages",
		},
		{
			providerType: types.ProviderTypeOpenRouter,
			baseURL:      "",
			expected:     "https://openrouter.ai/api/v1/models",
		},
		{
			providerType: types.ProviderTypeCerebras,
			baseURL:      "",
			expected:     "https://api.cerebras.ai/v1/models",
		},
		{
			providerType: types.ProviderTypeGemini,
			baseURL:      "",
			expected:     "https://generativelanguage.googleapis.com/v1/models",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			provider := &InitializedProvider{
				Type: tt.providerType,
				Config: types.ProviderConfig{
					BaseURL: tt.baseURL,
				},
			}

			endpoint, err := checker.getHealthCheckEndpoint(provider)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if endpoint != tt.expected {
				t.Errorf("got endpoint %q, expected %q", endpoint, tt.expected)
			}
		})
	}
}

func TestHealthChecker_GetHealthCheckEndpoint_UnsupportedProvider(t *testing.T) {
	checker := NewHealthChecker(time.Minute)

	provider := &InitializedProvider{
		Type: "unsupported",
	}

	_, err := checker.getHealthCheckEndpoint(provider)

	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestHealthChecker_ThreadSafety(t *testing.T) {
	checker := NewHealthChecker(time.Minute)

	// Start the checker
	checker.Start()
	defer checker.Stop()

	// Add some initial status
	checker.healthStatus[types.ProviderTypeOpenAI] = &ProviderHealth{
		Provider: types.ProviderTypeOpenAI,
		Healthy:  true,
		Details:  make(map[string]interface{}),
	}

	// Concurrent operations
	done := make(chan bool)

	// Reader goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = checker.GetHealthStatus(types.ProviderTypeOpenAI)
				_ = checker.GetAllHealthStatus()
				_ = checker.IsHealthy(types.ProviderTypeOpenAI)
				_ = checker.GetHealthyProviders()
				_ = checker.GetUnhealthyProviders()
				_ = checker.GetHealthSummary()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
