package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestGeminiProvider_GenerateChatCompletion_WithImage verifies that images are properly converted
// and sent to the Gemini API in the correct format
func TestGeminiProvider_GenerateChatCompletion_WithImage(t *testing.T) {
	// Create a mock server to intercept the API request
	var capturedRequest GenerateContentRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the request body
		if err := json.NewDecoder(r.Body).Decode(&capturedRequest); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Return a valid response
		response := GenerateContentResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Role: "model",
						Parts: []Part{
							{Text: "I can see a cat in the image."},
						},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: &UsageMetadata{
				PromptTokenCount:     100,
				CandidatesTokenCount: 50,
				TotalTokenCount:      150,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create provider with test configuration
	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	}

	provider := NewGeminiProvider(config)

	// Create a message with image content
	messages := []types.ChatMessage{
		{
			Role: "user",
			Parts: []types.ContentPart{
				types.NewTextPart("What's in this image?"),
				types.NewImagePart("image/png", "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="),
			},
		},
	}

	// Generate chat completion
	stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
		Messages: messages,
		Model:    "gemini-2.5-flash",
	})

	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}

	// Verify the response
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Failed to get response chunk: %v", err)
	}

	if chunk.Content != "I can see a cat in the image." {
		t.Errorf("Expected content 'I can see a cat in the image.', got '%s'", chunk.Content)
	}

	// Verify the request format
	if len(capturedRequest.Contents) != 1 {
		t.Fatalf("Expected 1 content, got %d", len(capturedRequest.Contents))
	}

	content := capturedRequest.Contents[0]
	if content.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", content.Role)
	}

	if len(content.Parts) != 2 {
		t.Fatalf("Expected 2 parts (text + image), got %d", len(content.Parts))
	}

	// Verify text part
	if content.Parts[0].Text != "What's in this image?" {
		t.Errorf("Expected text 'What's in this image?', got '%s'", content.Parts[0].Text)
	}

	// Verify image part with InlineData
	if content.Parts[1].InlineData == nil {
		t.Fatal("Expected InlineData to be present for image")
	}

	if content.Parts[1].InlineData.MimeType != "image/png" {
		t.Errorf("Expected mime type 'image/png', got '%s'", content.Parts[1].InlineData.MimeType)
	}

	if content.Parts[1].InlineData.Data != "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" {
		t.Error("Image data does not match expected base64 data")
	}
}

// TestGeminiProvider_GenerateChatCompletion_WithImageURL verifies that image URLs
// are properly converted to FileData format
func TestGeminiProvider_GenerateChatCompletion_WithImageURL(t *testing.T) {
	var capturedRequest GenerateContentRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedRequest); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		response := GenerateContentResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Role: "model",
						Parts: []Part{
							{Text: "I can see an image from the URL."},
						},
					},
					FinishReason: "STOP",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	}

	provider := NewGeminiProvider(config)

	// Create a message with image URL (e.g., from Google Cloud Storage)
	messages := []types.ChatMessage{
		{
			Role: "user",
			Parts: []types.ContentPart{
				types.NewTextPart("Describe this image"),
				types.NewImageURLPart("image/jpeg", "gs://my-bucket/image.jpg"),
			},
		},
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
		Messages: messages,
		Model:    "gemini-2.5-flash",
	})

	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Failed to get response chunk: %v", err)
	}

	if chunk.Content != "I can see an image from the URL." {
		t.Errorf("Expected content 'I can see an image from the URL.', got '%s'", chunk.Content)
	}

	// Verify FileData format for URL
	if len(capturedRequest.Contents) != 1 {
		t.Fatalf("Expected 1 content, got %d", len(capturedRequest.Contents))
	}

	content := capturedRequest.Contents[0]
	if len(content.Parts) != 2 {
		t.Fatalf("Expected 2 parts (text + image), got %d", len(content.Parts))
	}

	// Verify image part with FileData
	if content.Parts[1].FileData == nil {
		t.Fatal("Expected FileData to be present for image URL")
	}

	if content.Parts[1].FileData.MimeType != "image/jpeg" {
		t.Errorf("Expected mime type 'image/jpeg', got '%s'", content.Parts[1].FileData.MimeType)
	}

	if content.Parts[1].FileData.FileURI != "gs://my-bucket/image.jpg" {
		t.Errorf("Expected file URI 'gs://my-bucket/image.jpg', got '%s'", content.Parts[1].FileData.FileURI)
	}
}

// TestGeminiProvider_GenerateChatCompletion_WithMultipleImages verifies handling of
// multiple images in a single message
func TestGeminiProvider_GenerateChatCompletion_WithMultipleImages(t *testing.T) {
	var capturedRequest GenerateContentRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedRequest); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		response := GenerateContentResponse{
			Candidates: []Candidate{
				{
					Content: Content{
						Role: "model",
						Parts: []Part{
							{Text: "I can see two images."},
						},
					},
					FinishReason: "STOP",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeGemini,
		APIKey:  "test-api-key",
		BaseURL: server.URL,
	}

	provider := NewGeminiProvider(config)

	// Create a message with multiple images
	messages := []types.ChatMessage{
		{
			Role: "user",
			Parts: []types.ContentPart{
				types.NewTextPart("Compare these two images:"),
				types.NewImagePart("image/png", "base64data1"),
				types.NewImagePart("image/jpeg", "base64data2"),
			},
		},
	}

	_, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
		Messages: messages,
		Model:    "gemini-2.5-flash",
	})

	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}

	// Verify request has all parts
	if len(capturedRequest.Contents) != 1 {
		t.Fatalf("Expected 1 content, got %d", len(capturedRequest.Contents))
	}

	content := capturedRequest.Contents[0]
	if len(content.Parts) != 3 {
		t.Fatalf("Expected 3 parts (text + 2 images), got %d", len(content.Parts))
	}

	// Verify first image
	if content.Parts[1].InlineData == nil {
		t.Fatal("Expected InlineData for first image")
	}
	if content.Parts[1].InlineData.MimeType != "image/png" {
		t.Errorf("Expected first image mime type 'image/png', got '%s'", content.Parts[1].InlineData.MimeType)
	}

	// Verify second image
	if content.Parts[2].InlineData == nil {
		t.Fatal("Expected InlineData for second image")
	}
	if content.Parts[2].InlineData.MimeType != "image/jpeg" {
		t.Errorf("Expected second image mime type 'image/jpeg', got '%s'", content.Parts[2].InlineData.MimeType)
	}
}

// TestGeminiModels_HaveVisionCapability verifies that all Gemini models
// report vision capabilities
func TestGeminiModels_HaveVisionCapability(t *testing.T) {
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
		t.Fatal("Expected at least one model")
	}

	// Verify all models have vision capability
	for _, model := range models {
		hasVision := false
		hasMultimodal := false

		for _, cap := range model.Capabilities {
			if cap == "vision" {
				hasVision = true
			}
			if cap == "multimodal" {
				hasMultimodal = true
			}
		}

		if !hasVision {
			t.Errorf("Model %s does not have 'vision' capability", model.ID)
		}

		if !hasMultimodal {
			t.Errorf("Model %s does not have 'multimodal' capability", model.ID)
		}
	}
}
