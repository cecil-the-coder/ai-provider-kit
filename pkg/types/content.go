package types

// ContentPart represents a single part of message content (text, image, document, audio, etc.)
type ContentPart struct {
	Type string `json:"type"` // "text", "image", "document", "audio", "tool_use", "tool_result", "thinking"

	// Text content
	Text string `json:"text,omitempty"`

	// Media content (images, documents, audio)
	Source *MediaSource `json:"source,omitempty"`

	// Tool use (model calling a tool)
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`

	// Tool result (response to tool call)
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"` // string or []ContentPart

	// Extended thinking
	Thinking string `json:"thinking,omitempty"`

	// Escape hatch for future/provider-specific types
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// MediaSource represents the source of media content (images, documents, audio)
type MediaSource struct {
	Type      string `json:"type"`                 // "base64", "url"
	MediaType string `json:"media_type,omitempty"` // MIME type: "image/png", "application/pdf", "audio/wav"
	Data      string `json:"data,omitempty"`       // base64-encoded data
	URL       string `json:"url,omitempty"`        // URL to the media
}

// ContentType constants for common content types
const (
	ContentTypeText       = "text"
	ContentTypeImage      = "image"
	ContentTypeDocument   = "document"
	ContentTypeAudio      = "audio"
	ContentTypeToolUse    = "tool_use"
	ContentTypeToolResult = "tool_result"
	ContentTypeThinking   = "thinking"
)

// MediaSourceType constants
const (
	MediaSourceBase64 = "base64"
	MediaSourceURL    = "url"
)

// NewTextPart creates a text content part
func NewTextPart(text string) ContentPart {
	return ContentPart{Type: ContentTypeText, Text: text}
}

// NewImagePart creates an image content part from base64 data
func NewImagePart(mediaType, base64Data string) ContentPart {
	return ContentPart{
		Type: ContentTypeImage,
		Source: &MediaSource{
			Type:      MediaSourceBase64,
			MediaType: mediaType,
			Data:      base64Data,
		},
	}
}

// NewImageURLPart creates an image content part from a URL
func NewImageURLPart(mediaType, url string) ContentPart {
	return ContentPart{
		Type: ContentTypeImage,
		Source: &MediaSource{
			Type:      MediaSourceURL,
			MediaType: mediaType,
			URL:       url,
		},
	}
}

// NewDocumentPart creates a document content part (e.g., PDF)
func NewDocumentPart(mediaType, base64Data string) ContentPart {
	return ContentPart{
		Type: ContentTypeDocument,
		Source: &MediaSource{
			Type:      MediaSourceBase64,
			MediaType: mediaType,
			Data:      base64Data,
		},
	}
}

// IsMedia returns true if the content part contains media (image, document, audio)
func (c *ContentPart) IsMedia() bool {
	return c.Type == ContentTypeImage || c.Type == ContentTypeDocument || c.Type == ContentTypeAudio
}

// IsText returns true if the content part is text
func (c *ContentPart) IsText() bool {
	return c.Type == ContentTypeText
}

// IsToolRelated returns true if the content part is tool_use or tool_result
func (c *ContentPart) IsToolRelated() bool {
	return c.Type == ContentTypeToolUse || c.Type == ContentTypeToolResult
}
