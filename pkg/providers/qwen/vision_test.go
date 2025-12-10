package qwen

import (
	"encoding/json"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestQwenVisionSupport(t *testing.T) {
	t.Run("Convert image ContentParts to Qwen format", func(t *testing.T) {
		// Create a message with image and text content parts
		parts := []types.ContentPart{
			types.NewTextPart("What's in this image?"),
			types.NewImagePart("image/png", "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="),
		}

		// Convert to Qwen format
		result := convertContentPartsToQwen(parts)

		// Should return array of content parts
		qwenParts, ok := result.([]QwenContentPart)
		if !ok {
			t.Fatalf("Expected []QwenContentPart, got %T", result)
		}

		if len(qwenParts) != 2 {
			t.Fatalf("Expected 2 content parts, got %d", len(qwenParts))
		}

		// Check text part
		if qwenParts[0].Type != "text" {
			t.Errorf("Expected first part type to be 'text', got '%s'", qwenParts[0].Type)
		}
		if qwenParts[0].Text != "What's in this image?" {
			t.Errorf("Expected text content, got '%s'", qwenParts[0].Text)
		}

		// Check image part
		if qwenParts[1].Type != "image_url" {
			t.Errorf("Expected second part type to be 'image_url', got '%s'", qwenParts[1].Type)
		}
		if qwenParts[1].ImageURL == nil {
			t.Fatal("Expected image_url to be set")
		}
		expectedURL := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
		if qwenParts[1].ImageURL.URL != expectedURL {
			t.Errorf("Expected data URL, got '%s'", qwenParts[1].ImageURL.URL)
		}
	})

	t.Run("Convert image URL ContentPart to Qwen format", func(t *testing.T) {
		parts := []types.ContentPart{
			types.NewImageURLPart("image/jpeg", "https://example.com/image.jpg"),
		}

		result := convertContentPartsToQwen(parts)

		qwenParts, ok := result.([]QwenContentPart)
		if !ok {
			t.Fatalf("Expected []QwenContentPart, got %T", result)
		}

		if len(qwenParts) != 1 {
			t.Fatalf("Expected 1 content part, got %d", len(qwenParts))
		}

		if qwenParts[0].Type != "image_url" {
			t.Errorf("Expected type to be 'image_url', got '%s'", qwenParts[0].Type)
		}
		if qwenParts[0].ImageURL.URL != "https://example.com/image.jpg" {
			t.Errorf("Expected URL 'https://example.com/image.jpg', got '%s'", qwenParts[0].ImageURL.URL)
		}
	})

	t.Run("Single text part returns string for backwards compatibility", func(t *testing.T) {
		parts := []types.ContentPart{
			types.NewTextPart("Hello, world!"),
		}

		result := convertContentPartsToQwen(parts)

		// Should return string, not array
		contentStr, ok := result.(string)
		if !ok {
			t.Fatalf("Expected string, got %T", result)
		}

		if contentStr != "Hello, world!" {
			t.Errorf("Expected 'Hello, world!', got '%s'", contentStr)
		}
	})

	t.Run("Build request with vision content", func(t *testing.T) {
		provider := NewQwenProvider(types.ProviderConfig{
			Type:   types.ProviderTypeQwen,
			APIKey: "test-key",
		})

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{
					Role: "user",
					Parts: []types.ContentPart{
						types.NewTextPart("Analyze this image"),
						types.NewImagePart("image/jpeg", "base64data"),
					},
				},
			},
			MaxTokens:   1000,
			Temperature: 0.7,
		}

		request := provider.buildQwenRequest(options)

		if len(request.Messages) != 1 {
			t.Fatalf("Expected 1 message, got %d", len(request.Messages))
		}

		msg := request.Messages[0]
		if msg.Role != "user" {
			t.Errorf("Expected role 'user', got '%s'", msg.Role)
		}

		// Content should be array of parts
		contentParts, ok := msg.Content.([]QwenContentPart)
		if !ok {
			t.Fatalf("Expected []QwenContentPart, got %T", msg.Content)
		}

		if len(contentParts) != 2 {
			t.Fatalf("Expected 2 content parts, got %d", len(contentParts))
		}

		// Verify it's proper JSON-serializable
		_, err := json.Marshal(request)
		if err != nil {
			t.Errorf("Failed to marshal request to JSON: %v", err)
		}
	})

	t.Run("Model capabilities include vision", func(t *testing.T) {
		provider := NewQwenProvider(types.ProviderConfig{
			Type:   types.ProviderTypeQwen,
			APIKey: "test-key",
		})

		models, err := provider.GetModels(nil)
		if err != nil {
			t.Fatalf("Failed to get models: %v", err)
		}

		for _, model := range models {
			hasVision := false
			for _, cap := range model.Capabilities {
				if cap == "vision" {
					hasVision = true
					break
				}
			}
			if !hasVision {
				t.Errorf("Model %s missing 'vision' capability", model.ID)
			}
		}
	})
}
