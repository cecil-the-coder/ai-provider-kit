package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/racing"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Metrics Handler Tests (metrics.go)
// ============================================================================

func TestMetricsHandler_GetProviderMetrics(t *testing.T) {
	provider := &mockProvider{
		name: "test",
		metrics: types.ProviderMetrics{
			RequestCount: 100,
			SuccessCount: 95,
			ErrorCount:   5,
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewMetricsHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/metrics/providers", nil)

	handler.GetProviderMetrics(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMetricsHandler_GetSystemMetrics(t *testing.T) {
	handler := NewMetricsHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/metrics/system", nil)

	handler.GetSystemMetrics(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if _, ok := data["uptime"]; !ok {
		t.Error("Expected uptime in response")
	}

	if _, ok := data["goroutines"]; !ok {
		t.Error("Expected goroutines in response")
	}
}

func TestMetricsHandler_GetRacingMetrics(t *testing.T) {
	// Create a mock racing provider
	racingProvider := racing.NewRacingProvider("racing-test", nil)
	providers := map[string]types.Provider{
		"racing": racingProvider,
		"normal": &mockProvider{name: "normal"},
	}
	handler := NewMetricsHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/metrics/racing", nil)

	handler.GetRacingMetrics(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
