package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Health Handler Tests (health.go)
// ============================================================================

func TestHealthHandler_Status(t *testing.T) {
	providers := map[string]types.Provider{
		"test": &mockProvider{name: "test"},
	}
	handler := NewHealthHandler(providers, "1.0.0")

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/status", nil)

	handler.Status(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}
}

func TestHealthHandler_Health(t *testing.T) {
	providers := map[string]types.Provider{
		"test": &mockProvider{name: "test"},
	}
	handler := NewHealthHandler(providers, "1.0.0")

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/health", nil)

	handler.Health(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be a map")
	}

	if data["status"] != "healthy" {
		t.Errorf("Expected status healthy, got %v", data["status"])
	}

	if data["version"] != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %v", data["version"])
	}
}

func TestHealthHandler_Version(t *testing.T) {
	handler := NewHealthHandler(nil, "2.0.0")

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/version", nil)

	handler.Version(w, r)

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

	if data["version"] != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %v", data["version"])
	}
}
