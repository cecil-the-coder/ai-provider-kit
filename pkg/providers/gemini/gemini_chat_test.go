package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestGeminiProvider_GenerateChatCompletion_WithAPIKey(t *testing.T) {
	// Create a mock server
	mockServer := createMockServer(func(w http.ResponseWriter, r *http.Request) {
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
		response := createStandardMockResponse("Hello, this is a test response!")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
	defer mockServer.Close()

	// Create provider
	provider := createProviderWithMockServer(mockServer.URL)

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
	mockServer := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body has messages
		var req GenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if len(req.Contents) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(req.Contents))
		}

		// Return mock response
		response := createStandardMockResponse("Response to messages")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
	defer mockServer.Close()

	// Create provider
	provider := createProviderWithMockServer(mockServer.URL)

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
			mockServer := createMockServer(errorMockHandler(tt.statusCode, tt.responseBody))
			defer mockServer.Close()

			// Create provider
			provider := createProviderWithMockServer(mockServer.URL)

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

func TestGeminiProvider_GenerateChatCompletion_WithTools(t *testing.T) {
	// Create a mock server that verifies tools are passed
	mockServer := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		var req GenerateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Verify tools are included
		if len(req.Tools) == 0 {
			t.Error("Expected tools to be included in request")
		}

		// Return mock response
		response := createStandardMockResponse("Response with tools")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
	defer mockServer.Close()

	// Create provider
	provider := createProviderWithMockServer(mockServer.URL)

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
	mockServer := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		response := GenerateContentResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Parts: []Part{},
					},
					FinishReason: string(FinishReasonSafety),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
	defer mockServer.Close()

	// Create provider
	provider := createProviderWithMockServer(mockServer.URL)

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
	mockServer := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		response := GenerateContentResponse{
			Candidates: []Candidate{},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
	defer mockServer.Close()

	// Create provider
	provider := createProviderWithMockServer(mockServer.URL)

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

func TestGeminiProvider_GenerateChatCompletion_ModelResolution(t *testing.T) {
	// Create a mock server
	mockServer := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		// Verify the model in URL
		if !strings.Contains(r.URL.Path, "custom-model") {
			t.Errorf("Expected custom-model in URL, got: %s", r.URL.Path)
		}

		response := createStandardMockResponse("Response")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
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
