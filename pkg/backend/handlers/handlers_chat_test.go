package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Generate Handler Tests (generate.go)
// ============================================================================

func TestGenerateHandler_Generate(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Generated content",
			Usage:   types.Usage{PromptTokens: 5, CompletionTokens: 10, TotalTokens: 15},
		},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewGenerateHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{
		Prompt: "Test prompt",
		Model:  "test-model",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_InvalidJSON(t *testing.T) {
	handler := NewGenerateHandler(nil, nil, "test")

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", []byte("invalid json"))

	handler.Generate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_MissingContent(t *testing.T) {
	handler := NewGenerateHandler(nil, nil, "test")

	req := backendtypes.GenerateRequest{}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_ProviderNotFound(t *testing.T) {
	handler := NewGenerateHandler(map[string]types.Provider{}, nil, "default")

	req := backendtypes.GenerateRequest{
		Prompt:   "Test prompt",
		Provider: "nonexistent",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_WithExtensions(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Generated content",
		},
	}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewGenerateHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{
		Prompt: "Test prompt",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_ExtensionError(t *testing.T) {
	provider := &mockProvider{name: "test"}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{beforeErr: errors.New("extension failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewGenerateHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_Streaming(t *testing.T) {
	provider := &mockProvider{
		name:             "test",
		generateResponse: &types.ChatCompletionChunk{Content: "test response"},
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewGenerateHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{
		Prompt: "Test prompt",
		Stream: true,
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	// SSE streaming should return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify SSE headers
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Expected Cache-Control no-cache, got %s", cc)
	}

	// Verify response contains SSE data
	responseBody := w.Body.String()
	if !strings.Contains(responseBody, "data:") {
		t.Errorf("Expected SSE data in response, got: %s", responseBody)
	}
	if !strings.Contains(responseBody, "[DONE]") {
		t.Errorf("Expected [DONE] event in response, got: %s", responseBody)
	}
}

func TestGenerateHandler_Generate_GenerateError(t *testing.T) {
	provider := &mockProvider{
		name:        "test",
		generateErr: errors.New("generation failed"),
	}
	providers := map[string]types.Provider{"test": provider}
	handler := NewGenerateHandler(providers, nil, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_OnSelectedError(t *testing.T) {
	provider := &mockProvider{name: "test"}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{onSelectedErr: errors.New("selected failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewGenerateHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestGenerateHandler_Generate_AfterGenerateError(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		defaultModel: "test-model",
		generateResponse: &types.ChatCompletionChunk{
			Content: "Generated content",
		},
	}
	providers := map[string]types.Provider{"test": provider}

	ext := &mockExtension{afterErr: errors.New("after failed")}
	registry := &mockExtensionRegistry{}
	_ = registry.Register(ext)

	handler := NewGenerateHandler(providers, registry, "test")

	req := backendtypes.GenerateRequest{Prompt: "Test prompt"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := newRequestWithContext("POST", "/api/generate", body)

	handler.Generate(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestGenerateHandler_CollectStreamResponse(t *testing.T) {
	handler := NewGenerateHandler(nil, nil, "test")

	stream := &mockStream{
		chunk: &types.ChatCompletionChunk{
			Content: "Part 1",
			Choices: []types.ChatChoice{
				{Delta: types.ChatMessage{Content: " Part 2"}},
			},
		},
	}

	content, usage, err := handler.collectStreamResponse(stream)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedContent := "Part 1 Part 2"
	if content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, content)
	}

	if usage == nil {
		t.Fatal("Expected usage info")
	}

	if usage.TotalTokens != 30 {
		t.Errorf("Expected 30 total tokens, got %d", usage.TotalTokens)
	}
}

func TestBuildGenerateOptions(t *testing.T) {
	req := &backendtypes.GenerateRequest{
		Model:       "test-model",
		Prompt:      "test prompt",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
	}

	ctx := context.Background()
	options := buildGenerateOptions(req, ctx)

	if options.Model != "test-model" {
		t.Errorf("Expected model test-model, got %s", options.Model)
	}

	if options.MaxTokens != 100 {
		t.Errorf("Expected max tokens 100, got %d", options.MaxTokens)
	}

	if options.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", options.Temperature)
	}

	if !options.Stream {
		t.Error("Expected stream to be true")
	}

	// Check that prompt was converted to messages
	if len(options.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(options.Messages))
	}

	if options.Messages[0].Role != "user" {
		t.Errorf("Expected role user, got %s", options.Messages[0].Role)
	}

	if options.Messages[0].Content != "test prompt" {
		t.Errorf("Expected content 'test prompt', got %s", options.Messages[0].Content)
	}
}

func TestConvertFunctions(t *testing.T) {
	// Test convertToExtensionRequest
	req := &backendtypes.GenerateRequest{
		Provider:    "test",
		Model:       "test-model",
		Prompt:      "test prompt",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
	}

	extReq := convertToExtensionRequest(req)
	if extReq.Provider != "test" {
		t.Errorf("Expected provider test, got %s", extReq.Provider)
	}

	// Test updateFromExtensionRequest
	extReq.Provider = "new-provider"
	updateFromExtensionRequest(req, extReq)
	if req.Provider != "new-provider" {
		t.Errorf("Expected provider new-provider, got %s", req.Provider)
	}

	// Test convertToExtensionResponse
	resp := &backendtypes.GenerateResponse{
		Content:  "test content",
		Model:    "test-model",
		Provider: "test",
	}

	extResp := convertToExtensionResponse(resp)
	if extResp.Content != "test content" {
		t.Errorf("Expected content 'test content', got %s", extResp.Content)
	}

	// Test updateFromExtensionResponse
	extResp.Content = "new content"
	updateFromExtensionResponse(resp, extResp)
	if resp.Content != "new content" {
		t.Errorf("Expected content 'new content', got %s", resp.Content)
	}
}
