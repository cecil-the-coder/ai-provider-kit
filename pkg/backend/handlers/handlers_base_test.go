package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
)

// ============================================================================
// Base Handler Tests (base.go)
// ============================================================================

func TestSendSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/", nil)

	data := map[string]string{"key": "value"}
	SendSuccess(w, r, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}

	if response.RequestID != "test-request-id" {
		t.Errorf("Expected request ID test-request-id, got %s", response.RequestID)
	}
}

func TestSendError(t *testing.T) {
	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/", nil)

	SendError(w, r, "TEST_ERROR", "Test error message", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("Expected success to be false")
	}

	if response.Error == nil {
		t.Fatal("Expected error to be present")
	}

	if response.Error.Code != "TEST_ERROR" {
		t.Errorf("Expected error code TEST_ERROR, got %s", response.Error.Code)
	}

	if response.Error.Message != "Test error message" {
		t.Errorf("Expected error message 'Test error message', got %s", response.Error.Message)
	}
}

func TestSendCreated(t *testing.T) {
	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/", nil)

	data := map[string]string{"id": "123"}
	SendCreated(w, r, data)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response backendtypes.APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success to be true")
	}
}

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		expectErr bool
	}{
		{
			name:      "valid JSON",
			body:      `{"key": "value"}`,
			expectErr: false,
		},
		{
			name:      "invalid JSON",
			body:      `{invalid}`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(tt.body)))
			var target map[string]string
			err := ParseJSON(r, &target)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
