package decoders

import (
	"fmt"
	"strings"
	"sync"
)

// DecoderConstructor is a function that creates a new decoder instance.
type DecoderConstructor func() StreamDecoder

// DefaultDecoderFactory is the default implementation of DecoderFactory.
// It provides thread-safe registration and creation of stream decoders.
type DefaultDecoderFactory struct {
	constructors map[StreamFormat]DecoderConstructor
	mu           sync.RWMutex
}

// NewDefaultDecoderFactory creates a new decoder factory with built-in decoders.
// The factory comes pre-registered with SSE decoder support.
func NewDefaultDecoderFactory() *DefaultDecoderFactory {
	factory := &DefaultDecoderFactory{
		constructors: make(map[StreamFormat]DecoderConstructor),
	}

	// Register built-in decoder constructors
	factory.registerConstructor(StreamFormatSSE, func() StreamDecoder {
		return NewSSEDecoder()
	})

	return factory
}

// CreateDecoder creates a NEW decoder instance for the specified format.
// Each call returns a fresh decoder instance to avoid state conflicts.
// Returns an error if the format is not supported.
func (f *DefaultDecoderFactory) CreateDecoder(format StreamFormat) (StreamDecoder, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	constructor, ok := f.constructors[format]
	if !ok {
		return nil, fmt.Errorf("unsupported stream format: %s", format)
	}

	// Create a new instance
	return constructor(), nil
}

// registerConstructor registers a decoder constructor for a format.
func (f *DefaultDecoderFactory) registerConstructor(format StreamFormat, constructor DecoderConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.constructors[format] = constructor
}

// RegisterDecoder registers a custom decoder for a format.
// If a decoder is already registered for this format, it will be replaced.
// Note: The provided decoder instance is used as a prototype - the factory
// will return the SAME instance for all CreateDecoder calls. If you need
// independent instances, wrap your decoder creation in a constructor function
// and use registerConstructor instead (though that's a private method).
func (f *DefaultDecoderFactory) RegisterDecoder(format StreamFormat, decoder StreamDecoder) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Store as a constructor that returns the same instance
	f.constructors[format] = func() StreamDecoder {
		return decoder
	}
}

// SupportedFormats returns the list of formats this factory supports.
func (f *DefaultDecoderFactory) SupportedFormats() []StreamFormat {
	f.mu.RLock()
	defer f.mu.RUnlock()

	formats := make([]StreamFormat, 0, len(f.constructors))
	for format := range f.constructors {
		formats = append(formats, format)
	}

	return formats
}

// DefaultAutoDetector implements AutoDetector for common stream formats.
type DefaultAutoDetector struct{}

// NewDefaultAutoDetector creates a new auto-detector.
func NewDefaultAutoDetector() *DefaultAutoDetector {
	return &DefaultAutoDetector{}
}

// DetectFromContentType detects the stream format from HTTP Content-Type header.
// Recognizes common MIME types for SSE, NDJSON, and event streams.
func (d *DefaultAutoDetector) DetectFromContentType(contentType string) StreamFormat {
	// Normalize content type (lowercase and remove parameters)
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	switch contentType {
	case "text/event-stream":
		return StreamFormatSSE
	case "application/x-ndjson", "application/jsonlines", "application/ndjson":
		return StreamFormatNDJSON
	case "application/stream+json":
		return StreamFormatEventStream
	default:
		// Check for partial matches
		if strings.Contains(contentType, "event-stream") {
			return StreamFormatSSE
		}
		if strings.Contains(contentType, "ndjson") || strings.Contains(contentType, "jsonlines") {
			return StreamFormatNDJSON
		}
		return StreamFormatUnknown
	}
}

// DetectFromBytes detects the format by examining initial bytes.
// This performs heuristic detection based on common patterns.
func (d *DefaultAutoDetector) DetectFromBytes(bytes []byte) StreamFormat {
	if len(bytes) == 0 {
		return StreamFormatUnknown
	}

	content := string(bytes)

	// Check for SSE patterns (field: value format)
	// SSE typically starts with patterns like "event:", "data:", "id:", or ":"
	if hasSSEPattern(content) {
		return StreamFormatSSE
	}

	// Check for NDJSON pattern (one JSON object per line)
	if hasNDJSONPattern(content) {
		return StreamFormatNDJSON
	}

	return StreamFormatUnknown
}

// hasSSEPattern checks if the content contains SSE field patterns.
func hasSSEPattern(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for SSE field patterns
		if strings.HasPrefix(line, "event:") ||
			strings.HasPrefix(line, "data:") ||
			strings.HasPrefix(line, "id:") ||
			strings.HasPrefix(line, "retry:") ||
			strings.HasPrefix(line, ":") { // Comment line
			return true
		}
	}
	return false
}

// hasNDJSONPattern checks if the content looks like NDJSON.
func hasNDJSONPattern(content string) bool {
	lines := strings.Split(content, "\n")
	jsonLineCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if line looks like JSON
		if (strings.HasPrefix(line, "{") && strings.Contains(line, "}")) ||
			(strings.HasPrefix(line, "[") && strings.Contains(line, "]")) {
			jsonLineCount++
		}
	}

	// If most non-empty lines look like JSON, it's probably NDJSON
	return jsonLineCount > 0
}
