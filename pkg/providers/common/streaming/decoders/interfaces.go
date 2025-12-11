// Package decoders provides pluggable stream decoders for various streaming formats
// including Server-Sent Events (SSE), NDJSON, and EventStream. The package supports
// automatic format detection and a factory pattern for creating appropriate decoders.
package decoders

import (
	"io"
)

// StreamFormat represents the format of a streaming response.
type StreamFormat string

const (
	// StreamFormatSSE represents Server-Sent Events format.
	// SSE is a standard protocol for server-to-client streaming with events.
	StreamFormatSSE StreamFormat = "sse"

	// StreamFormatNDJSON represents Newline-Delimited JSON format.
	// Each line is a complete JSON object, separated by newlines.
	StreamFormatNDJSON StreamFormat = "ndjson"

	// StreamFormatEventStream represents generic event stream format.
	// A flexible format that can be customized per provider.
	StreamFormatEventStream StreamFormat = "event-stream"

	// StreamFormatUnknown represents an unknown or undetected format.
	StreamFormatUnknown StreamFormat = "unknown"
)

// StreamEvent represents a decoded event from a streaming response.
// This is a generic structure that can represent events from various formats.
type StreamEvent struct {
	// Type is the event type (e.g., "message", "error", "ping").
	// For SSE, this comes from the "event:" field.
	Type string

	// Data is the event payload/content.
	// For SSE, this comes from the "data:" field (may be multi-line).
	Data string

	// ID is the event identifier.
	// For SSE, this comes from the "id:" field.
	ID string

	// Retry is the reconnection time in milliseconds.
	// For SSE, this comes from the "retry:" field.
	Retry int

	// Raw contains the raw event data before parsing (for debugging).
	Raw string
}

// StreamDecoder defines the interface for decoding streaming events.
// Different formats (SSE, NDJSON, etc.) implement this interface.
type StreamDecoder interface {
	// Decode reads and decodes the next event from the stream.
	// Returns io.EOF when the stream ends normally.
	// Returns other errors for parsing or connection issues.
	Decode(reader io.Reader) (StreamEvent, error)

	// Format returns the stream format this decoder handles.
	Format() StreamFormat
}

// DecoderFactory creates StreamDecoder instances based on stream format.
// This allows for registration of custom decoders and format detection.
type DecoderFactory interface {
	// CreateDecoder creates a decoder for the specified format.
	// Returns an error if the format is not supported.
	CreateDecoder(format StreamFormat) (StreamDecoder, error)

	// RegisterDecoder registers a custom decoder for a format.
	// This allows providers to add support for custom streaming formats.
	RegisterDecoder(format StreamFormat, decoder StreamDecoder)

	// SupportedFormats returns the list of formats this factory supports.
	SupportedFormats() []StreamFormat
}

// AutoDetector detects the stream format from content-type headers or initial bytes.
// This is useful when the format is not explicitly specified.
type AutoDetector interface {
	// DetectFromContentType detects the format from HTTP Content-Type header.
	// Returns StreamFormatUnknown if detection fails.
	DetectFromContentType(contentType string) StreamFormat

	// DetectFromBytes detects the format by examining initial bytes.
	// This is useful when Content-Type is missing or ambiguous.
	// The bytes parameter should contain at least the first few lines of the stream.
	DetectFromBytes(bytes []byte) StreamFormat
}
