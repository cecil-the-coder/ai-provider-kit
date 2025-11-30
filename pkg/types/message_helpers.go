package types

import "strings"

// GetContentParts returns content as []ContentPart
// If Content is a non-empty string and Parts is nil, returns [{Type: "text", Text: content}]
// If Parts is set, returns Parts
func (m *ChatMessage) GetContentParts() []ContentPart {
	if len(m.Parts) > 0 {
		return m.Parts
	}
	if m.Content != "" {
		return []ContentPart{NewTextPart(m.Content)}
	}
	return nil
}

// GetTextContent returns concatenated text from all text parts
// Works with both string Content and Parts array
func (m *ChatMessage) GetTextContent() string {
	if len(m.Parts) > 0 {
		var texts []string
		for _, part := range m.Parts {
			if part.IsText() && part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return m.Content
}

// HasImages returns true if any content part is an image
func (m *ChatMessage) HasImages() bool {
	for _, part := range m.Parts {
		if part.Type == ContentTypeImage {
			return true
		}
	}
	return false
}

// HasMedia returns true if any content part has media (image, document, audio)
func (m *ChatMessage) HasMedia() bool {
	for _, part := range m.Parts {
		if part.IsMedia() {
			return true
		}
	}
	return false
}

// SetTextContent sets Content to a simple string and clears Parts
func (m *ChatMessage) SetTextContent(text string) {
	m.Content = text
	m.Parts = nil
}

// SetContentParts sets Parts and clears Content string
func (m *ChatMessage) SetContentParts(parts []ContentPart) {
	m.Parts = parts
	m.Content = ""
}

// AddContentPart appends a content part to Parts
// If Content string was set, converts it to a text part first
func (m *ChatMessage) AddContentPart(part ContentPart) {
	// If we have a Content string but no Parts yet, convert Content to a text part first
	if m.Content != "" && len(m.Parts) == 0 {
		m.Parts = []ContentPart{NewTextPart(m.Content)}
		m.Content = ""
	}
	m.Parts = append(m.Parts, part)
}
