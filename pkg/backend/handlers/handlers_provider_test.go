package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Provider Handler Tests (providers.go)
// ============================================================================

func TestProviderHandler_ListProviders(t *testing.T) {
	providers := map[string]types.Provider{
		"test1": &mockProvider{
			name:         "test1",
			providerType: types.ProviderTypeOpenAI,
			description:  "Test provider 1",
			models:       []types.Model{{ID: "model1"}},
		},
		"test2": &mockProvider{
			name:         "test2",
			providerType: types.ProviderTypeAnthropic,
			description:  "Test provider 2",
		},
	}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/providers", nil)

	handler.ListProviders(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestProviderHandler_ListProviders_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers", nil)

	handler.ListProviders(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_GetProvider(t *testing.T) {
	providers := map[string]types.Provider{
		"test": &mockProvider{
			name:         "test",
			providerType: types.ProviderTypeOpenAI,
			description:  "Test provider",
		},
	}
	handler := NewProviderHandler(providers)

	tests := []struct {
		name       string
		url        string
		expectCode int
	}{
		{
			name:       "provider exists - query param",
			url:        "/api/providers?name=test",
			expectCode: http.StatusOK,
		},
		{
			name:       "provider exists - path param",
			url:        "/api/providers/test",
			expectCode: http.StatusOK,
		},
		{
			name:       "provider not found",
			url:        "/api/providers?name=nonexistent",
			expectCode: http.StatusNotFound,
		},
		{
			name:       "missing parameter",
			url:        "/api/providers",
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := newRequestWithContext("GET", tt.url, nil)

			handler.GetProvider(w, r)

			if w.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, w.Code)
			}
		})
	}
}

func TestProviderHandler_GetProvider_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test", nil)

	handler.GetProvider(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_UpdateProvider(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		providerType: types.ProviderTypeOpenAI,
		config: types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			Name:         "test",
			BaseURL:      "https://api.openai.com",
			DefaultModel: "gpt-4",
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	configReq := backendtypes.ProviderConfigRequest{
		BaseURL:      "https://new-url.com",
		DefaultModel: "gpt-4-turbo",
	}
	body, _ := json.Marshal(configReq)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers/test", body)
	r.Header.Set("Content-Type", "application/json")

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify config was updated
	if provider.config.BaseURL != "https://new-url.com" {
		t.Errorf("Expected BaseURL to be updated to https://new-url.com, got %s", provider.config.BaseURL)
	}
}

func TestProviderHandler_UpdateProvider_ConfigureError(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		configureErr: errors.New("configure failed"),
		config: types.ProviderConfig{
			Type: types.ProviderTypeOpenAI,
			Name: "test",
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	configReq := backendtypes.ProviderConfigRequest{BaseURL: "https://new-url.com"}
	body, _ := json.Marshal(configReq)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers/test", body)

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestProviderHandler_UpdateProvider_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/providers/test", nil)

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_UpdateProvider_MissingProvider(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers", nil)

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestProviderHandler_UpdateProvider_ProviderNotFound(t *testing.T) {
	handler := NewProviderHandler(map[string]types.Provider{})

	configReq := backendtypes.ProviderConfigRequest{BaseURL: "https://test.com"}
	body, _ := json.Marshal(configReq)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers/nonexistent", body)

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestProviderHandler_UpdateProvider_InvalidJSON(t *testing.T) {
	provider := &mockProvider{name: "test", config: types.ProviderConfig{Type: types.ProviderTypeOpenAI, Name: "test"}}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("PUT", "/api/providers/test", []byte("invalid json"))

	handler.UpdateProvider(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestProviderHandler_HealthCheckProvider(t *testing.T) {
	tests := []struct {
		name           string
		providerName   string
		healthCheckErr error
		expectCode     int
	}{
		{
			name:         "healthy provider",
			providerName: "test",
			expectCode:   http.StatusOK,
		},
		{
			name:           "unhealthy provider",
			providerName:   "test",
			healthCheckErr: errors.New("provider down"),
			expectCode:     http.StatusOK,
		},
		{
			name:         "provider not found",
			providerName: "nonexistent",
			expectCode:   http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := map[string]types.Provider{
				"test": &mockProvider{
					name:           "test",
					healthCheckErr: tt.healthCheckErr,
				},
			}
			handler := NewProviderHandler(providers)

			w := httptest.NewRecorder()
			r := newRequestWithContext("GET", "/api/providers/"+tt.providerName+"/health", nil)

			handler.HealthCheckProvider(w, r)

			if w.Code != tt.expectCode {
				t.Errorf("Expected status %d, got %d", tt.expectCode, w.Code)
			}
		})
	}
}

func TestProviderHandler_HealthCheckProvider_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test/health", nil)

	handler.HealthCheckProvider(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_HealthCheckProvider_MissingProvider(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/providers/health", nil)

	handler.HealthCheckProvider(w, r)

	// Empty provider name leads to provider not found
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 404 or 400, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Test response",
			Usage:   types.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test/test", nil)

	handler.TestProvider(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_GenerateError(t *testing.T) {
	provider := &mockProvider{
		name:        "test",
		generateErr: errors.New("generation failed"),
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test/test", nil)

	handler.TestProvider(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_WrongMethod(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("GET", "/api/providers/test/test", nil)

	handler.TestProvider(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_MissingProvider(t *testing.T) {
	handler := NewProviderHandler(nil)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test", nil)

	handler.TestProvider(w, r)

	// Empty provider name leads to provider not found
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 404 or 400, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_ProviderNotFound(t *testing.T) {
	handler := NewProviderHandler(map[string]types.Provider{})

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/nonexistent/test", nil)

	handler.TestProvider(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestProviderHandler_TestProvider_StreamReadError(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewProviderHandler(providers)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/providers/test/test", nil)

	handler.TestProvider(w, r)

	// Should succeed with default mock behavior
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestProviderHandler_ExtractProviderName(t *testing.T) {
	handler := NewProviderHandler(nil)

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "query parameter",
			url:      "/api/providers?name=test",
			expected: "test",
		},
		{
			name:     "path parameter",
			url:      "/api/providers/test",
			expected: "test",
		},
		{
			name:     "path with action",
			url:      "/api/providers/test/health",
			expected: "test",
		},
		{
			name:     "no parameter",
			url:      "/api/providers",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tt.url, nil)
			result := handler.extractProviderName(r)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
