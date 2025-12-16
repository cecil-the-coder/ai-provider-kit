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
// Stream Handler Tests (stream.go)
// ============================================================================

func TestStreamHandler_StreamGenerate(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Streaming content",
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewStreamHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{
		Prompt: "Test prompt",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	// Should set SSE headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestStreamHandler_StreamGenerate_InvalidJSON(t *testing.T) {
	handler := NewStreamHandler(nil, nil, "test")

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", []byte("invalid"))

	handler.StreamGenerate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestStreamHandler_StreamGenerate_ProviderNotFound(t *testing.T) {
	handler := NewStreamHandler(map[string]types.Provider{}, nil, "default")

	req := backendtypes.GenerateRequest{
		Prompt:   "Test prompt",
		Provider: "nonexistent",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestStreamHandler_StreamGenerate_MissingContent(t *testing.T) {
	handler := NewStreamHandler(nil, nil, "test")

	req := backendtypes.GenerateRequest{}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestStreamHandler_StreamGenerate_WithExtensions(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Streaming content",
		},
	}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewStreamHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestStreamHandler_StreamGenerate_ExtensionBeforeError(t *testing.T) {
	provider := &mockProvider{name: "test"}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{beforeErr: errors.New("before failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewStreamHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	// Should have SSE headers set before error
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestStreamHandler_StreamGenerate_ExtensionOnSelectedError(t *testing.T) {
	provider := &mockProvider{name: "test"}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{onSelectedErr: errors.New("selected failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewStreamHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestStreamHandler_StreamGenerate_GenerateError(t *testing.T) {
	provider := &mockProvider{
		name:        "test",
		generateErr: errors.New("generate failed"),
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewStreamHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/stream", body)

	handler.StreamGenerate(w, r)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}
