package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewGeminiProvider(t *testing.T) {
	config := types.ProviderConfig{
		Type:         types.ProviderTypeGemini,
		Name:         "test-gemini",
		APIKey:       "test-api-key",
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta",
		DefaultModel: "gemini-1.5-pro",
	}

	provider := NewGeminiProvider(config)
	if provider == nil {
		t.Fatal("NewGeminiProvider returned nil")
	}

	if provider.Name() != "gemini" {
		t.Errorf("Expected name 'gemini', got '%s'", provider.Name())
	}

	if provider.Type() != types.ProviderTypeGemini {
		t.Errorf("Expected type %s, got %s", types.ProviderTypeGemini, provider.Type())
	}

	if provider.config.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", provider.config.APIKey)
	}

	if provider.config.Model != "gemini-1.5-pro" {
		t.Errorf("Expected model 'gemini-1.5-pro', got '%s'", provider.config.Model)
	}
}

func TestGeminiProvider_DisplayName(t *testing.T) {
	config := types.ProviderConfig{
		Type: types.ProviderTypeGemini,
		ProviderConfig: map[string]interface{}{
			"display_name": "Custom Gemini",
		},
	}

	provider := NewGeminiProvider(config)
	if provider.Name() != "Custom Gemini" {
		t.Errorf("Expected display name 'Custom Gemini', got '%s'", provider.Name())
	}
}

func TestGeminiProvider_GetModels_StaticFallback(t *testing.T) {
	// Test that GetModels returns static fallback when API key is configured
	// but API call fails (which is expected in tests)
	config := types.ProviderConfig{
		Type:   types.ProviderTypeGemini,
		APIKey: "test-api-key",
	}
	provider := NewGeminiProvider(config)

	models, err := provider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("No models returned")
	}

	// Check for models from static fallback
	modelMap := make(map[string]types.Model)
	for _, model := range models {
		modelMap[model.ID] = model
	}

	// Should have some models from static fallback (Gemini 2.5/2.0 series)
	expectedModels := []string{
		"gemini-2.5-flash",
		"gemini-2.5-pro",
		"gemini-2.0-flash",
	}

	for _, expectedID := range expectedModels {
		if _, exists := modelMap[expectedID]; !exists {
			t.Errorf("Expected model '%s' not found in fallback", expectedID)
		}
	}

	// Verify model metadata is enriched
	if model, exists := modelMap["gemini-2.5-flash"]; exists {
		if !model.SupportsStreaming {
			t.Error("Expected gemini-2.5-flash to support streaming")
		}
		if !model.SupportsToolCalling {
			t.Error("Expected gemini-2.5-flash to support tool calling")
		}
		if model.Provider != types.ProviderTypeGemini {
			t.Errorf("Expected provider to be %s, got %s", types.ProviderTypeGemini, model.Provider)
		}
	}
}

func TestGeminiProvider_GetModels_Fallback(t *testing.T) {
	// Create provider without API key to test fallback
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	models, err := provider.GetModels(context.Background())
	if err != nil {
		t.Fatalf("GetModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("No models returned from fallback")
	}

	// Check that fallback models are returned
	modelMap := make(map[string]types.Model)
	for _, model := range models {
		modelMap[model.ID] = model
	}

	// Should have some models from static fallback
	if _, exists := modelMap["gemini-2.5-flash"]; !exists {
		t.Error("Expected default model 'gemini-2.5-flash' in fallback")
	}
}

func TestGeminiProvider_GetDefaultModel(t *testing.T) {
	tests := []struct {
		name     string
		config   GeminiConfig
		expected string
	}{
		{
			name: "Configured model",
			config: GeminiConfig{
				Model: "gemini-1.5-pro",
			},
			expected: "gemini-1.5-pro",
		},
		{
			name:     "Default model",
			config:   GeminiConfig{},
			expected: geminiDefaultModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &GeminiProvider{
				config: tt.config,
			}
			actual := provider.GetDefaultModel()
			if actual != tt.expected {
				t.Errorf("Expected default model '%s', got '%s'", tt.expected, actual)
			}
		})
	}
}

func TestGeminiProvider_Authenticate(t *testing.T) {
	tests := []struct {
		name       string
		authConfig types.AuthConfig
		expectErr  bool
	}{
		{
			name: "API key authentication",
			authConfig: types.AuthConfig{
				Method:       types.AuthMethodAPIKey,
				APIKey:       "new-api-key",
				BaseURL:      "https://example.com",
				DefaultModel: "gemini-1.5-pro",
			},
			expectErr: false,
		},
		{
			name: "Unsupported method",
			authConfig: types.AuthConfig{
				Method: types.AuthMethodBearerToken,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})
			err := provider.Authenticate(context.Background(), tt.authConfig)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestGeminiProvider_IsAuthenticated(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *GeminiProvider
		expected  bool
	}{
		{
			name: "API key authenticated",
			setupFunc: func() *GeminiProvider {
				config := types.ProviderConfig{
					Type:   types.ProviderTypeGemini,
					APIKey: "test-api-key",
				}
				return NewGeminiProvider(config)
			},
			expected: true,
		},
		{
			name: "Not authenticated",
			setupFunc: func() *GeminiProvider {
				config := types.ProviderConfig{
					Type: types.ProviderTypeGemini,
				}
				return NewGeminiProvider(config)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.setupFunc()
			actual := provider.IsAuthenticated()
			if actual != tt.expected {
				t.Errorf("Expected authenticated=%v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestGeminiProvider_SupportsFeatures(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	if !provider.SupportsToolCalling() {
		t.Error("Gemini should support tool calling")
	}

	if !provider.SupportsStreaming() {
		t.Error("Gemini should support streaming")
	}

	if provider.SupportsResponsesAPI() {
		t.Error("Gemini should not support Responses API")
	}

	if provider.GetToolFormat() != types.ToolFormatOpenAI {
		t.Errorf("Expected tool format %s, got %s", types.ToolFormatOpenAI, provider.GetToolFormat())
	}
}

func TestProjectIDRequiredError(t *testing.T) {
	err := &ProjectIDRequiredError{}

	if err.Error() == "" {
		t.Error("ProjectIDRequiredError.Error() should return non-empty string")
	}

	if !IsProjectIDRequired(err) {
		t.Error("IsProjectIDRequired should return true for ProjectIDRequiredError")
	}

	if IsProjectIDRequired(fmt.Errorf("different error")) {
		t.Error("IsProjectIDRequired should return false for other errors")
	}
}

func TestGeminiProvider_GenerateChatCompletion_WithAPIKey(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API key is in URL
		apiKey := r.URL.Query().Get("key")
		if apiKey != "test-api-key" {
			t.Errorf("Expected API key 'test-api-key', got '%s'", apiKey)
		}

		// Verify request body
		var req GenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Return mock response
		response := GenerateContentResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Parts: []Part{
							{Text: "Hello, this is a test response!"},
						},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &UsageMetadata{
				PromptTokenCount:     10,
				CandidatesTokenCount: 20,
				TotalTokenCount:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create provider
	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
	}
	provider := NewGeminiProvider(config)
	provider.config.BaseURL = mockServer.URL

	// Test non-streaming completion
	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "gemini-1.5-pro",
		Stream: false,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}

	if stream == nil {
		t.Fatal("Expected non-nil stream")
	}

	// Read the response
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Failed to read chunk: %v", err)
	}

	if !strings.Contains(chunk.Content, "test response") {
		t.Errorf("Expected response to contain 'test response', got '%s'", chunk.Content)
	}

	if chunk.Usage.TotalTokens != 30 {
		t.Errorf("Expected 30 total tokens, got %d", chunk.Usage.TotalTokens)
	}
}

func TestGeminiProvider_GenerateChatCompletion_WithMessages(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body has messages
		var req GenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if len(req.Contents) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(req.Contents))
		}

		// Return mock response
		response := GenerateContentResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Parts: []Part{
							{Text: "Response to messages"},
						},
					},
					FinishReason: "STOP",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create provider
	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
	}
	provider := NewGeminiProvider(config)
	provider.config.BaseURL = mockServer.URL

	// Test with messages
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
		Model:  "gemini-1.5-pro",
		Stream: false,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}

	if stream == nil {
		t.Fatal("Expected non-nil stream")
	}

	// Read the response
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Failed to read chunk: %v", err)
	}

	if !strings.Contains(chunk.Content, "messages") {
		t.Errorf("Expected response to contain 'messages', got '%s'", chunk.Content)
	}
}

func TestGeminiProvider_GenerateChatCompletion_Streaming(t *testing.T) {
	// Create a mock server for streaming
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a streaming request
		if !strings.Contains(r.URL.Path, "streamGenerateContent") {
			t.Errorf("Expected streaming endpoint, got %s", r.URL.Path)
		}

		// Write SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Write chunks
		chunks := []string{
			`data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`,
			`data: {"candidates":[{"content":{"parts":[{"text":" world"}]}}]}`,
			`data: {"candidates":[{"content":{"parts":[{"text":"!"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":10,"totalTokenCount":15}}`,
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "%s\n\n", chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer mockServer.Close()

	// Create provider
	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
	}
	provider := NewGeminiProvider(config)
	provider.config.BaseURL = mockServer.URL

	// Test streaming
	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "gemini-1.5-pro",
		Stream: true,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}

	if stream == nil {
		t.Fatal("Expected non-nil stream")
	}

	// Read chunks
	var fullContent strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read chunk: %v", err)
		}
		fullContent.WriteString(chunk.Content)
	}

	content := fullContent.String()
	if !strings.Contains(content, "Hello") {
		t.Errorf("Expected content to contain 'Hello', got '%s'", content)
	}
}

func TestGeminiProvider_GenerateChatCompletion_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedErrMsg string
	}{
		{
			name:           "API Error 500",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"error": "Internal server error"}`,
			expectedErrMsg: "500",
		},
		{
			name:           "Rate Limit Error",
			statusCode:     http.StatusTooManyRequests,
			responseBody:   `{"error": "Rate limit exceeded"}`,
			expectedErrMsg: "429",
		},
		{
			name:           "Unauthorized",
			statusCode:     http.StatusUnauthorized,
			responseBody:   `{"error": "Invalid API key"}`,
			expectedErrMsg: "401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server that returns error
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer mockServer.Close()

			// Create provider
			config := types.ProviderConfig{
				Type:    types.ProviderTypeGemini,
				APIKey:  "test-api-key",
				BaseURL: mockServer.URL,
			}
			provider := NewGeminiProvider(config)
			provider.config.BaseURL = mockServer.URL

			// Test error handling
			options := types.GenerateOptions{
				Prompt: "Hello",
				Model:  "gemini-1.5-pro",
				Stream: false,
			}

			_, err := provider.GenerateChatCompletion(context.Background(), options)
			if err == nil {
				t.Error("Expected error but got none")
			}

			if !strings.Contains(err.Error(), tt.expectedErrMsg) {
				t.Errorf("Expected error to contain '%s', got '%v'", tt.expectedErrMsg, err)
			}
		})
	}
}

func TestGeminiProvider_GenerateChatCompletion_NoAuth(t *testing.T) {
	// Create provider without API key or OAuth
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "gemini-1.5-pro",
		Stream: false,
	}

	_, err := provider.GenerateChatCompletion(context.Background(), options)
	if err == nil {
		t.Error("Expected error for no authentication, but got none")
	}

	if !strings.Contains(err.Error(), "authentication") {
		t.Errorf("Expected authentication error, got: %v", err)
	}
}

func TestGeminiProvider_Configure(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	newConfig := types.ProviderConfig{
		Type:         types.ProviderTypeGemini,
		APIKey:       "new-key",
		BaseURL:      "https://new-url.com",
		DefaultModel: "gemini-2.0",
		ProviderConfig: map[string]interface{}{
			"display_name": "New Gemini",
			"project_id":   "test-project",
		},
	}

	err := provider.Configure(newConfig)
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}

	if provider.config.APIKey != "new-key" {
		t.Errorf("Expected API key 'new-key', got '%s'", provider.config.APIKey)
	}

	if provider.displayName != "New Gemini" {
		t.Errorf("Expected display name 'New Gemini', got '%s'", provider.displayName)
	}

	if provider.projectID != "test-project" {
		t.Errorf("Expected project ID 'test-project', got '%s'", provider.projectID)
	}
}

func TestGeminiProvider_Logout(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeGemini,
		APIKey: "test-api-key",
	}
	provider := NewGeminiProvider(config)

	// Verify authenticated before logout
	if !provider.IsAuthenticated() {
		t.Error("Expected provider to be authenticated before logout")
	}

	// Logout - verify it doesn't error and calls the necessary cleanup
	err := provider.Logout(context.Background())
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// Logout should have been called successfully
	// The actual behavior is that it calls authHelper.ClearAuthentication() and Configure()
	// which may re-setup authentication from the config, so we just verify no error occurred
}

func TestGeminiProvider_Description(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})
	desc := provider.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestGeminiProvider_UpdateRateLimitTier(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Update to pay-as-you-go tier
	provider.UpdateRateLimitTier(360)

	// Verify the limiter was updated (we can't directly test the rate,
	// but we can verify the method doesn't panic)
	if provider.clientSideLimiter == nil {
		t.Error("Expected rate limiter to be set")
	}
}

func TestGeminiProvider_InvokeServerTool(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	_, err := provider.InvokeServerTool(context.Background(), "test_tool", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for unimplemented tool invocation")
	}
}

func TestGeminiProvider_GetAuthStatus(t *testing.T) {
	config := types.ProviderConfig{
		Type:   types.ProviderTypeGemini,
		APIKey: "test-api-key",
	}
	provider := NewGeminiProvider(config)

	status := provider.GetAuthStatus()
	if status == nil {
		t.Error("Expected non-nil auth status")
	}

	// Should have some status information
	if len(status) == 0 {
		t.Error("Expected auth status to have some entries")
	}
}

func TestMockStream(t *testing.T) {
	stream := &MockStream{
		chunks: []types.ChatCompletionChunk{
			{Content: "Hello", Done: false},
			{Content: " World", Done: true},
		},
	}

	// Read first chunk
	chunk1, err := stream.Next()
	if err != nil {
		t.Fatalf("Failed to read first chunk: %v", err)
	}
	if chunk1.Content != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", chunk1.Content)
	}

	// Read second chunk
	chunk2, err := stream.Next()
	if err != nil {
		t.Fatalf("Failed to read second chunk: %v", err)
	}
	if chunk2.Content != " World" {
		t.Errorf("Expected ' World', got '%s'", chunk2.Content)
	}

	// Close stream
	err = stream.Close()
	if err != nil {
		t.Fatalf("Failed to close stream: %v", err)
	}

	// Verify stream was reset
	if stream.index != 0 {
		t.Errorf("Expected index to be reset to 0, got %d", stream.index)
	}
}

func TestGeminiStream_Close(t *testing.T) {
	// Create a mock response
	mockResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("")),
	}

	stream := &GeminiStream{
		response: mockResp,
		done:     false,
	}

	err := stream.Close()
	if err != nil {
		t.Fatalf("Failed to close stream: %v", err)
	}

	if !stream.done {
		t.Error("Expected stream to be marked as done after close")
	}
}

func TestGeminiProvider_GenerateChatCompletion_WithTools(t *testing.T) {
	// Create a mock server that verifies tools are passed
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req GenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Verify tools are included
		if len(req.Tools) == 0 {
			t.Error("Expected tools to be included in request")
		}

		// Return mock response
		response := GenerateContentResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Parts: []Part{
							{Text: "Response with tools"},
						},
					},
					FinishReason: "STOP",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create provider
	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
	}
	provider := NewGeminiProvider(config)
	provider.config.BaseURL = mockServer.URL

	// Test with tools
	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "gemini-1.5-pro",
		Stream: false,
		Tools: []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}

	if stream == nil {
		t.Fatal("Expected non-nil stream")
	}
}

func TestGeminiProvider_GenerateChatCompletion_SafetyFilter(t *testing.T) {
	// Create a mock server that returns safety filter response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GenerateContentResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Parts: []Part{},
					},
					FinishReason: "SAFETY",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create provider
	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
	}
	provider := NewGeminiProvider(config)
	provider.config.BaseURL = mockServer.URL

	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "gemini-1.5-pro",
		Stream: false,
	}

	_, err := provider.GenerateChatCompletion(context.Background(), options)
	if err == nil {
		t.Error("Expected error for safety-filtered content")
	}

	if !strings.Contains(err.Error(), "safety") {
		t.Errorf("Expected safety error, got: %v", err)
	}
}

func TestGeminiProvider_GenerateChatCompletion_EmptyResponse(t *testing.T) {
	// Create a mock server that returns empty candidates
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GenerateContentResponse{
			Candidates: []Candidate{},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create provider
	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
	}
	provider := NewGeminiProvider(config)
	provider.config.BaseURL = mockServer.URL

	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "gemini-1.5-pro",
		Stream: false,
	}

	_, err := provider.GenerateChatCompletion(context.Background(), options)
	if err == nil {
		t.Error("Expected error for empty candidates")
	}

	if !strings.Contains(err.Error(), "candidates") {
		t.Errorf("Expected candidates error, got: %v", err)
	}
}

func TestGeminiProvider_RefreshAllOAuthTokens(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Should return error when OAuth is not configured
	err := provider.RefreshAllOAuthTokens(context.Background())
	if err == nil {
		t.Error("Expected error when OAuth is not configured")
	}
}

func TestGeminiProvider_Type(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})
	if provider.Type() != types.ProviderTypeGemini {
		t.Errorf("Expected type %s, got %s", types.ProviderTypeGemini, provider.Type())
	}
}

func TestGeminiProvider_ConfigureInvalid(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with invalid type
	invalidConfig := types.ProviderConfig{
		Type: types.ProviderTypeAnthropic, // Wrong type
	}

	err := provider.Configure(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid provider type in config")
	}
}

func TestGeminiProvider_AuthenticateEmptyMethod(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with empty auth method (should succeed for test compatibility)
	authConfig := types.AuthConfig{
		Method: "",
	}

	err := provider.Authenticate(context.Background(), authConfig)
	if err != nil {
		t.Errorf("Expected no error for empty auth method, got: %v", err)
	}
}

func TestGeminiProvider_AuthenticateOAuth(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with OAuth method (should return error - not supported via Authenticate)
	authConfig := types.AuthConfig{
		Method: types.AuthMethodOAuth,
	}

	err := provider.Authenticate(context.Background(), authConfig)
	if err == nil {
		t.Error("Expected error for OAuth authentication method")
	}

	if !strings.Contains(err.Error(), "OAuth") {
		t.Errorf("Expected OAuth error message, got: %v", err)
	}
}

func TestGeminiProvider_GetProjectID(t *testing.T) {
	tests := []struct {
		name     string
		provider *GeminiProvider
		expected string
	}{
		{
			name: "From config",
			provider: &GeminiProvider{
				projectID: "config-project-id",
			},
			expected: "config-project-id",
		},
		{
			name: "Empty",
			provider: &GeminiProvider{
				projectID: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.provider.getProjectID()
			if actual != tt.expected {
				t.Errorf("Expected project ID '%s', got '%s'", tt.expected, actual)
			}
		})
	}
}

func TestGeminiProvider_GenerateChatCompletion_ModelResolution(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the model in URL
		if !strings.Contains(r.URL.Path, "custom-model") {
			t.Errorf("Expected custom-model in URL, got: %s", r.URL.Path)
		}

		response := GenerateContentResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Parts: []Part{
							{Text: "Response"},
						},
					},
					FinishReason: "STOP",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create provider with default model
	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
		ProviderConfig: map[string]interface{}{
			"model": "default-model",
		},
	}
	provider := NewGeminiProvider(config)
	provider.config.BaseURL = mockServer.URL

	// Test with custom model in options (should override default)
	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "custom-model",
		Stream: false,
	}

	_, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}
}

func TestGeminiStream_NextEOF(t *testing.T) {
	// Create a stream with EOF response
	mockResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("")),
	}

	stream := &GeminiStream{
		response: mockResp,
		reader:   bufio.NewReader(mockResp.Body),
		done:     false,
	}

	// Reading from empty stream should return EOF
	_, err := stream.Next()
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}

	// Stream should be marked as done
	if !stream.done {
		t.Error("Expected stream to be marked as done")
	}
}

func TestGeminiStream_NextWithData(t *testing.T) {
	// Create a stream with SSE data
	sseData := `data: {"candidates":[{"content":{"parts":[{"text":"Test"}]}}]}

`
	mockResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	stream := &GeminiStream{
		response: mockResp,
		reader:   bufio.NewReader(mockResp.Body),
		done:     false,
	}

	// Read the chunk
	chunk, err := stream.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("Unexpected error: %v", err)
	}

	if chunk.Content != "Test" {
		t.Errorf("Expected content 'Test', got '%s'", chunk.Content)
	}
}

func TestEnrichModels(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	models := []types.Model{
		{
			ID:       "test-model",
			Name:     "Test Model",
			Provider: types.ProviderTypeGemini,
		},
	}

	enriched := provider.enrichModels(models)

	if len(enriched) != 1 {
		t.Fatalf("Expected 1 enriched model, got %d", len(enriched))
	}

	if !enriched[0].SupportsStreaming {
		t.Error("Expected enriched model to support streaming")
	}

	if !enriched[0].SupportsToolCalling {
		t.Error("Expected enriched model to support tool calling")
	}
}

func TestGetStaticFallback(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	models := provider.getStaticFallback()

	if len(models) == 0 {
		t.Error("Expected static fallback to return models")
	}

	// All models should have provider set
	for _, model := range models {
		if model.Provider != types.ProviderTypeGemini {
			t.Errorf("Expected provider %s, got %s", types.ProviderTypeGemini, model.Provider)
		}
		if model.MaxTokens == 0 {
			t.Errorf("Model %s should have MaxTokens set", model.ID)
		}
	}
}

func TestPrepareStandardRequest(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with prompt
	options := types.GenerateOptions{
		Prompt: "Test prompt",
	}

	req := provider.prepareStandardRequest(options)

	if len(req.Contents) != 1 {
		t.Errorf("Expected 1 content, got %d", len(req.Contents))
	}

	if req.Contents[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", req.Contents[0].Role)
	}

	if req.Contents[0].Parts[0].Text != "Test prompt" {
		t.Errorf("Expected text 'Test prompt', got '%s'", req.Contents[0].Parts[0].Text)
	}

	if req.GenerationConfig == nil {
		t.Error("Expected GenerationConfig to be set")
	}
}

func TestPrepareStandardRequest_WithMessages(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with messages
	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
		},
	}

	req := provider.prepareStandardRequest(options)

	if len(req.Contents) != 2 {
		t.Errorf("Expected 2 contents, got %d", len(req.Contents))
	}
}

func TestPrepareStandardRequest_WithTools(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Test with tools
	options := types.GenerateOptions{
		Prompt: "Test",
		Tools: []types.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			},
		},
	}

	req := provider.prepareStandardRequest(options)

	if len(req.Tools) == 0 {
		t.Error("Expected tools to be included")
	}
}

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name          string
		providerModel string
		inputModel    string
		optionsModel  string
		expectedModel string
	}{
		{
			name:          "Use input model",
			providerModel: "provider-default",
			inputModel:    "input-model",
			optionsModel:  "",
			expectedModel: "input-model",
		},
		{
			name:          "Use options model",
			providerModel: "provider-default",
			inputModel:    "",
			optionsModel:  "options-model",
			expectedModel: "options-model",
		},
		{
			name:          "Use provider default",
			providerModel: "provider-default",
			inputModel:    "",
			optionsModel:  "",
			expectedModel: "provider-default",
		},
		{
			name:          "Use system default",
			providerModel: "",
			inputModel:    "",
			optionsModel:  "",
			expectedModel: geminiDefaultModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &GeminiProvider{
				config: GeminiConfig{
					Model: tt.providerModel,
				},
			}

			options := types.GenerateOptions{
				Model: tt.optionsModel,
			}

			result := provider.resolveModel(tt.inputModel, options)
			if result != tt.expectedModel {
				t.Errorf("Expected model '%s', got '%s'", tt.expectedModel, result)
			}
		})
	}
}

func TestParseStandardGeminiResponse_EmptyParts(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Response with empty parts
	response := GenerateContentResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Parts: []Part{},
				},
			},
		},
	}

	responseBody, _ := json.Marshal(response)

	_, _, err := provider.parseStandardGeminiResponse(responseBody, "test-model")
	if err == nil {
		t.Error("Expected error for empty parts")
	}
}

func TestParseStandardGeminiResponse_WithUsage(t *testing.T) {
	provider := NewGeminiProvider(types.ProviderConfig{Type: types.ProviderTypeGemini})

	// Response with usage metadata
	response := GenerateContentResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Parts: []Part{
						{Text: "Test response"},
					},
				},
			},
		},
		UsageMetadata: &UsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 20,
			TotalTokenCount:      30,
		},
	}

	responseBody, _ := json.Marshal(response)

	content, usage, err := provider.parseStandardGeminiResponse(responseBody, "test-model")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if content != "Test response" {
		t.Errorf("Expected 'Test response', got '%s'", content)
	}

	if usage == nil {
		t.Fatal("Expected usage to be set")
	}

	if usage.TotalTokens != 30 {
		t.Errorf("Expected 30 total tokens, got %d", usage.TotalTokens)
	}
}
