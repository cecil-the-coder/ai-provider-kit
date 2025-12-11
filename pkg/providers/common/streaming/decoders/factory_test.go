package decoders

import (
	"io"
	"testing"
)

// TestDefaultDecoderFactory_CreateDecoder tests the decoder creation.
func TestDefaultDecoderFactory_CreateDecoder(t *testing.T) {
	factory := NewDefaultDecoderFactory()

	tests := []struct {
		name        string
		format      StreamFormat
		expectError bool
	}{
		{
			name:        "SSE decoder",
			format:      StreamFormatSSE,
			expectError: false,
		},
		{
			name:        "Unsupported format",
			format:      StreamFormat("unsupported"),
			expectError: true,
		},
		{
			name:        "NDJSON not registered by default",
			format:      StreamFormatNDJSON,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder, err := factory.CreateDecoder(tt.format)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if decoder != nil {
					t.Error("Expected nil decoder on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if decoder == nil {
					t.Error("Expected decoder but got nil")
				}
				if decoder.Format() != tt.format {
					t.Errorf("Expected format %s, got %s", tt.format, decoder.Format())
				}
			}
		})
	}
}

// TestDefaultDecoderFactory_RegisterDecoder tests custom decoder registration.
func TestDefaultDecoderFactory_RegisterDecoder(t *testing.T) {
	factory := NewDefaultDecoderFactory()

	// Register a mock decoder
	mockDecoder := &MockDecoder{format: StreamFormatNDJSON}
	factory.RegisterDecoder(StreamFormatNDJSON, mockDecoder)

	// Verify it can be created
	decoder, err := factory.CreateDecoder(StreamFormatNDJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if decoder.Format() != StreamFormatNDJSON {
		t.Errorf("Expected format %s, got %s", StreamFormatNDJSON, decoder.Format())
	}

	// Verify it's the same instance
	if decoder != mockDecoder {
		t.Error("Expected registered decoder instance")
	}
}

// TestDefaultDecoderFactory_RegisterDecoder_Replace tests replacing a decoder.
func TestDefaultDecoderFactory_RegisterDecoder_Replace(t *testing.T) {
	factory := NewDefaultDecoderFactory()

	// Get original SSE decoder
	original, _ := factory.CreateDecoder(StreamFormatSSE)

	// Register a replacement
	replacement := &MockDecoder{format: StreamFormatSSE}
	factory.RegisterDecoder(StreamFormatSSE, replacement)

	// Verify replacement
	current, _ := factory.CreateDecoder(StreamFormatSSE)
	if current == original {
		t.Error("Expected decoder to be replaced")
	}
	if current != replacement {
		t.Error("Expected replacement decoder")
	}
}

// TestDefaultDecoderFactory_SupportedFormats tests listing supported formats.
func TestDefaultDecoderFactory_SupportedFormats(t *testing.T) {
	factory := NewDefaultDecoderFactory()

	formats := factory.SupportedFormats()
	if len(formats) == 0 {
		t.Error("Expected at least one supported format")
	}

	// Check that SSE is included (default)
	hasSSE := false
	for _, format := range formats {
		if format == StreamFormatSSE {
			hasSSE = true
			break
		}
	}
	if !hasSSE {
		t.Error("Expected SSE to be in supported formats")
	}

	// Add another decoder and verify it's listed
	factory.RegisterDecoder(StreamFormatNDJSON, &MockDecoder{format: StreamFormatNDJSON})
	formats = factory.SupportedFormats()
	if len(formats) < 2 {
		t.Error("Expected at least two supported formats after registration")
	}
}

// TestDefaultAutoDetector_DetectFromContentType tests content type detection.
func TestDefaultAutoDetector_DetectFromContentType(t *testing.T) {
	detector := NewDefaultAutoDetector()

	tests := []struct {
		name        string
		contentType string
		expected    StreamFormat
	}{
		{
			name:        "SSE standard",
			contentType: "text/event-stream",
			expected:    StreamFormatSSE,
		},
		{
			name:        "SSE with charset",
			contentType: "text/event-stream; charset=utf-8",
			expected:    StreamFormatSSE,
		},
		{
			name:        "SSE uppercase",
			contentType: "TEXT/EVENT-STREAM",
			expected:    StreamFormatSSE,
		},
		{
			name:        "NDJSON standard",
			contentType: "application/x-ndjson",
			expected:    StreamFormatNDJSON,
		},
		{
			name:        "NDJSON alternative",
			contentType: "application/ndjson",
			expected:    StreamFormatNDJSON,
		},
		{
			name:        "NDJSON jsonlines",
			contentType: "application/jsonlines",
			expected:    StreamFormatNDJSON,
		},
		{
			name:        "Event stream",
			contentType: "application/stream+json",
			expected:    StreamFormatEventStream,
		},
		{
			name:        "Unknown format",
			contentType: "application/json",
			expected:    StreamFormatUnknown,
		},
		{
			name:        "Empty content type",
			contentType: "",
			expected:    StreamFormatUnknown,
		},
		{
			name:        "Partial match SSE",
			contentType: "text/custom-event-stream",
			expected:    StreamFormatSSE,
		},
		{
			name:        "Partial match NDJSON",
			contentType: "application/custom-ndjson",
			expected:    StreamFormatNDJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromContentType(tt.contentType)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestDefaultAutoDetector_DetectFromBytes tests format detection from content.
func TestDefaultAutoDetector_DetectFromBytes(t *testing.T) {
	detector := NewDefaultAutoDetector()

	tests := []struct {
		name     string
		content  string
		expected StreamFormat
	}{
		{
			name: "SSE with data field",
			content: `data: {"message": "hello"}

`,
			expected: StreamFormatSSE,
		},
		{
			name: "SSE with event field",
			content: `event: message
data: test

`,
			expected: StreamFormatSSE,
		},
		{
			name: "SSE with id field",
			content: `id: 123
data: test

`,
			expected: StreamFormatSSE,
		},
		{
			name: "SSE with comment",
			content: `: This is a comment
data: test

`,
			expected: StreamFormatSSE,
		},
		{
			name: "NDJSON format",
			content: `{"message": "hello"}
{"message": "world"}
`,
			expected: StreamFormatNDJSON,
		},
		{
			name: "NDJSON single object",
			content: `{"message": "hello", "status": "ok"}
`,
			expected: StreamFormatNDJSON,
		},
		{
			name:     "Empty content",
			content:  "",
			expected: StreamFormatUnknown,
		},
		{
			name:     "Plain text",
			content:  "Hello world\nThis is text\n",
			expected: StreamFormatUnknown,
		},
		{
			name:     "Only whitespace",
			content:  "\n\n\n",
			expected: StreamFormatUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromBytes([]byte(tt.content))
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestHasSSEPattern tests the SSE pattern detection helper.
func TestHasSSEPattern(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "data field",
			content:  "data: test",
			expected: true,
		},
		{
			name:     "event field",
			content:  "event: message",
			expected: true,
		},
		{
			name:     "id field",
			content:  "id: 123",
			expected: true,
		},
		{
			name:     "retry field",
			content:  "retry: 1000",
			expected: true,
		},
		{
			name:     "comment",
			content:  ": comment",
			expected: true,
		},
		{
			name:     "no SSE pattern",
			content:  "just text",
			expected: false,
		},
		{
			name:     "JSON content",
			content:  `{"data": "test"}`,
			expected: false,
		},
		{
			name:     "empty",
			content:  "",
			expected: false,
		},
		{
			name: "mixed content with SSE",
			content: `some text
data: test
more text`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasSSEPattern(tt.content)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestHasNDJSONPattern tests the NDJSON pattern detection helper.
func TestHasNDJSONPattern(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "single JSON object",
			content:  `{"message": "hello"}`,
			expected: true,
		},
		{
			name: "multiple JSON objects",
			content: `{"message": "hello"}
{"message": "world"}`,
			expected: true,
		},
		{
			name:     "JSON array",
			content:  `[1, 2, 3]`,
			expected: true,
		},
		{
			name:     "plain text",
			content:  "hello world",
			expected: false,
		},
		{
			name:     "empty",
			content:  "",
			expected: false,
		},
		{
			name: "mixed with non-JSON",
			content: `{"valid": "json"}
not json
{"more": "json"}`,
			expected: true,
		},
		{
			name:     "incomplete JSON",
			content:  `{"incomplete":`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasNDJSONPattern(tt.content)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestDefaultDecoderFactory_Concurrent tests thread safety.
func TestDefaultDecoderFactory_Concurrent(t *testing.T) {
	factory := NewDefaultDecoderFactory()

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			// Register decoders
			customFormat := StreamFormat("custom" + string(rune(id)))
			factory.RegisterDecoder(customFormat, &MockDecoder{format: customFormat})

			// Create decoders
			_, _ = factory.CreateDecoder(StreamFormatSSE)

			// List formats
			_ = factory.SupportedFormats()

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify factory is still functional
	decoder, err := factory.CreateDecoder(StreamFormatSSE)
	if err != nil {
		t.Errorf("Factory corrupted after concurrent access: %v", err)
	}
	if decoder == nil {
		t.Error("Expected decoder after concurrent access")
	}
}

// MockDecoder is a mock implementation for testing.
type MockDecoder struct {
	format StreamFormat
}

func (m *MockDecoder) Decode(reader io.Reader) (StreamEvent, error) {
	return StreamEvent{}, io.EOF
}

func (m *MockDecoder) Format() StreamFormat {
	return m.format
}
