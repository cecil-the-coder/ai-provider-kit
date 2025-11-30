package types

import (
	"testing"
)

func TestChatMessageHelpers(t *testing.T) {
	t.Run("GetContentParts with string Content", func(t *testing.T) {
		msg := ChatMessage{
			Role:    "user",
			Content: "Hello, world!",
		}

		parts := msg.GetContentParts()
		if len(parts) != 1 {
			t.Errorf("Expected 1 part, got %d", len(parts))
		}
		if parts[0].Type != ContentTypeText {
			t.Errorf("Expected type %s, got %s", ContentTypeText, parts[0].Type)
		}
		if parts[0].Text != "Hello, world!" {
			t.Errorf("Expected text 'Hello, world!', got '%s'", parts[0].Text)
		}
	})

	t.Run("GetContentParts with Parts", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
			Parts: []ContentPart{
				NewTextPart("Hello"),
				NewImagePart("image/png", "base64data"),
			},
		}

		parts := msg.GetContentParts()
		if len(parts) != 2 {
			t.Errorf("Expected 2 parts, got %d", len(parts))
		}
		if parts[0].Type != ContentTypeText {
			t.Errorf("Expected type %s, got %s", ContentTypeText, parts[0].Type)
		}
		if parts[1].Type != ContentTypeImage {
			t.Errorf("Expected type %s, got %s", ContentTypeImage, parts[1].Type)
		}
	})

	t.Run("GetContentParts with empty message", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
		}

		parts := msg.GetContentParts()
		if parts != nil {
			t.Errorf("Expected nil parts, got %v", parts)
		}
	})

	t.Run("GetTextContent from string Content", func(t *testing.T) {
		msg := ChatMessage{
			Role:    "user",
			Content: "Hello, world!",
		}

		text := msg.GetTextContent()
		if text != "Hello, world!" {
			t.Errorf("Expected 'Hello, world!', got '%s'", text)
		}
	})

	t.Run("GetTextContent from Parts", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
			Parts: []ContentPart{
				NewTextPart("Hello"),
				NewTextPart("World"),
			},
		}

		text := msg.GetTextContent()
		expected := "Hello\nWorld"
		if text != expected {
			t.Errorf("Expected '%s', got '%s'", expected, text)
		}
	})

	t.Run("GetTextContent with mixed parts", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
			Parts: []ContentPart{
				NewTextPart("Text before"),
				NewImagePart("image/png", "data"),
				NewTextPart("Text after"),
			},
		}

		text := msg.GetTextContent()
		expected := "Text before\nText after"
		if text != expected {
			t.Errorf("Expected '%s', got '%s'", expected, text)
		}
	})

	t.Run("HasImages returns true", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
			Parts: []ContentPart{
				NewTextPart("Hello"),
				NewImagePart("image/png", "data"),
			},
		}

		if !msg.HasImages() {
			t.Error("Expected HasImages to return true")
		}
	})

	t.Run("HasImages returns false", func(t *testing.T) {
		msg := ChatMessage{
			Role:    "user",
			Content: "Hello",
		}

		if msg.HasImages() {
			t.Error("Expected HasImages to return false")
		}
	})

	t.Run("HasMedia returns true for image", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
			Parts: []ContentPart{
				NewImagePart("image/png", "data"),
			},
		}

		if !msg.HasMedia() {
			t.Error("Expected HasMedia to return true")
		}
	})

	t.Run("HasMedia returns true for document", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
			Parts: []ContentPart{
				NewDocumentPart("application/pdf", "data"),
			},
		}

		if !msg.HasMedia() {
			t.Error("Expected HasMedia to return true")
		}
	})

	t.Run("HasMedia returns false", func(t *testing.T) {
		msg := ChatMessage{
			Role:    "user",
			Content: "Hello",
		}

		if msg.HasMedia() {
			t.Error("Expected HasMedia to return false")
		}
	})

	t.Run("SetTextContent", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
			Parts: []ContentPart{
				NewImagePart("image/png", "data"),
			},
		}

		msg.SetTextContent("New text")

		if msg.Content != "New text" {
			t.Errorf("Expected Content 'New text', got '%s'", msg.Content)
		}
		if msg.Parts != nil {
			t.Error("Expected Parts to be nil")
		}
	})

	t.Run("SetContentParts", func(t *testing.T) {
		msg := ChatMessage{
			Role:    "user",
			Content: "Old text",
		}

		parts := []ContentPart{
			NewTextPart("New text"),
			NewImagePart("image/png", "data"),
		}
		msg.SetContentParts(parts)

		if msg.Content != "" {
			t.Errorf("Expected empty Content, got '%s'", msg.Content)
		}
		if len(msg.Parts) != 2 {
			t.Errorf("Expected 2 parts, got %d", len(msg.Parts))
		}
	})

	t.Run("AddContentPart to empty message", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
		}

		msg.AddContentPart(NewTextPart("Hello"))

		if len(msg.Parts) != 1 {
			t.Errorf("Expected 1 part, got %d", len(msg.Parts))
		}
		if msg.Parts[0].Text != "Hello" {
			t.Errorf("Expected text 'Hello', got '%s'", msg.Parts[0].Text)
		}
	})

	t.Run("AddContentPart converts Content to Part", func(t *testing.T) {
		msg := ChatMessage{
			Role:    "user",
			Content: "Initial text",
		}

		msg.AddContentPart(NewImagePart("image/png", "data"))

		if msg.Content != "" {
			t.Errorf("Expected empty Content, got '%s'", msg.Content)
		}
		if len(msg.Parts) != 2 {
			t.Errorf("Expected 2 parts, got %d", len(msg.Parts))
		}
		if msg.Parts[0].Type != ContentTypeText {
			t.Errorf("Expected first part to be text, got %s", msg.Parts[0].Type)
		}
		if msg.Parts[0].Text != "Initial text" {
			t.Errorf("Expected first part text 'Initial text', got '%s'", msg.Parts[0].Text)
		}
		if msg.Parts[1].Type != ContentTypeImage {
			t.Errorf("Expected second part to be image, got %s", msg.Parts[1].Type)
		}
	})

	t.Run("AddContentPart to existing Parts", func(t *testing.T) {
		msg := ChatMessage{
			Role: "user",
			Parts: []ContentPart{
				NewTextPart("First"),
			},
		}

		msg.AddContentPart(NewTextPart("Second"))

		if len(msg.Parts) != 2 {
			t.Errorf("Expected 2 parts, got %d", len(msg.Parts))
		}
		if msg.Parts[1].Text != "Second" {
			t.Errorf("Expected second part text 'Second', got '%s'", msg.Parts[1].Text)
		}
	})

	t.Run("Backwards compatibility - Content still works", func(t *testing.T) {
		msg := ChatMessage{
			Role:    "user",
			Content: "Simple message",
		}

		// Old code accessing Content directly still works
		if msg.Content != "Simple message" {
			t.Errorf("Expected Content 'Simple message', got '%s'", msg.Content)
		}

		// But helper methods also work
		text := msg.GetTextContent()
		if text != "Simple message" {
			t.Errorf("Expected text 'Simple message', got '%s'", text)
		}
	})
}
