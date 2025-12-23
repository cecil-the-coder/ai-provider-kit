package decoders

import (
	"io"
	"strings"
	"testing"
)

// TestIntegration_FactoryWithSSEDecoder tests the factory with SSE decoder.
func TestIntegration_FactoryWithSSEDecoder(t *testing.T) {
	factory := NewDefaultDecoderFactory()

	// Create SSE decoder
	decoder, err := factory.CreateDecoder(StreamFormatSSE)
	if err != nil {
		t.Fatalf("Failed to create SSE decoder: %v", err)
	}

	// Use decoder to parse SSE data
	sseData := `data: test message

`
	reader := strings.NewReader(sseData)
	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Failed to decode SSE: %v", err)
	}

	if event.Data != "test message" {
		t.Errorf("Expected data 'test message', got '%s'", event.Data)
	}
}

// TestIntegration_AutoDetectAndDecode tests auto-detection followed by decoding.
func TestIntegration_AutoDetectAndDecode(t *testing.T) {
	detector := NewDefaultAutoDetector()
	factory := NewDefaultDecoderFactory()

	tests := []struct {
		name        string
		contentType string
		data        string
		expectedMsg string
	}{
		{
			name:        "SSE from content type",
			contentType: "text/event-stream",
			data:        "data: hello\n\n",
			expectedMsg: "hello",
		},
		{
			name:        "SSE from content detection",
			contentType: "",
			data:        "data: world\n\n",
			expectedMsg: "world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Detect format
			var format StreamFormat
			if tt.contentType != "" {
				format = detector.DetectFromContentType(tt.contentType)
			} else {
				format = detector.DetectFromBytes([]byte(tt.data))
			}

			if format == StreamFormatUnknown {
				t.Fatal("Failed to detect format")
			}

			// Create decoder
			decoder, err := factory.CreateDecoder(format)
			if err != nil {
				t.Fatalf("Failed to create decoder: %v", err)
			}

			// Decode data
			reader := strings.NewReader(tt.data)
			event, err := decoder.Decode(reader)
			if err != nil {
				t.Fatalf("Failed to decode: %v", err)
			}

			if event.Data != tt.expectedMsg {
				t.Errorf("Expected data '%s', got '%s'", tt.expectedMsg, event.Data)
			}
		})
	}
}

// TestIntegration_CustomDecoder tests registering and using a custom decoder.
func TestIntegration_CustomDecoder(t *testing.T) {
	factory := NewDefaultDecoderFactory()

	// Register custom decoder
	customFormat := StreamFormat("custom")
	factory.RegisterDecoder(customFormat, &CustomTestDecoder{})

	// Verify it's listed
	formats := factory.SupportedFormats()
	hasCustom := false
	for _, f := range formats {
		if f == customFormat {
			hasCustom = true
			break
		}
	}
	if !hasCustom {
		t.Error("Custom format not in supported formats")
	}

	// Use custom decoder
	decoder, err := factory.CreateDecoder(customFormat)
	if err != nil {
		t.Fatalf("Failed to create custom decoder: %v", err)
	}

	reader := strings.NewReader("custom data")
	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Failed to decode with custom decoder: %v", err)
	}

	if event.Type != "custom" {
		t.Errorf("Expected type 'custom', got '%s'", event.Type)
	}
}

// TestStreamFormat_Constants tests that format constants are defined correctly.
func TestStreamFormat_Constants(t *testing.T) {
	tests := []struct {
		format   StreamFormat
		expected string
	}{
		{StreamFormatSSE, "sse"},
		{StreamFormatNDJSON, "ndjson"},
		{StreamFormatEventStream, "event-stream"},
		{StreamFormatUnknown, "unknown"},
	}

	for _, tt := range tests {
		if string(tt.format) != tt.expected {
			t.Errorf("Expected format string '%s', got '%s'", tt.expected, string(tt.format))
		}
	}
}

// TestStreamEvent_Fields tests that StreamEvent has all required fields.
func TestStreamEvent_Fields(t *testing.T) {
	event := StreamEvent{
		Type:  "test",
		Data:  "data",
		ID:    "123",
		Retry: 1000,
		Raw:   "raw",
	}

	if event.Type != "test" {
		t.Errorf("Expected Type 'test', got '%s'", event.Type)
	}
	if event.Data != "data" {
		t.Errorf("Expected Data 'data', got '%s'", event.Data)
	}
	if event.ID != "123" {
		t.Errorf("Expected ID '123', got '%s'", event.ID)
	}
	if event.Retry != 1000 {
		t.Errorf("Expected Retry 1000, got %d", event.Retry)
	}
	if event.Raw != "raw" {
		t.Errorf("Expected Raw 'raw', got '%s'", event.Raw)
	}
}

// TestIntegration_RealWorldSSEStream tests a complete real-world SSE stream.
func TestIntegration_RealWorldSSEStream(t *testing.T) {
	// Simulate a real SSE stream from an AI provider
	sseStream := `: keep-alive comment

event: start
id: stream-001
data: {"status": "started"}

: heartbeat

event: chunk
id: stream-002
retry: 3000
data: {"delta": "Hello"}

event: chunk
id: stream-003
data: {"delta": " world"}

event: chunk
id: stream-004
data: {"delta": "!"}

event: end
id: stream-005
data: {"status": "completed", "usage": {"tokens": 10}}

`

	decoder := NewSSEDecoder()
	reader := strings.NewReader(sseStream)

	expectedEvents := []struct {
		eventType string
		id        string
		hasData   bool
	}{
		{"start", "stream-001", true},
		{"chunk", "stream-002", true},
		{"chunk", "stream-003", true},
		{"chunk", "stream-004", true},
		{"end", "stream-005", true},
	}

	for i, expected := range expectedEvents {
		event, err := decoder.Decode(reader)
		if err != nil {
			t.Fatalf("Event %d: unexpected error: %v", i, err)
		}

		if event.Type != expected.eventType {
			t.Errorf("Event %d: expected type '%s', got '%s'", i, expected.eventType, event.Type)
		}
		if event.ID != expected.id {
			t.Errorf("Event %d: expected ID '%s', got '%s'", i, expected.id, event.ID)
		}
		if expected.hasData && event.Data == "" {
			t.Errorf("Event %d: expected data but got empty string", i)
		}

		// Verify retry was set and persists
		if i == 1 && event.Retry != 3000 {
			t.Errorf("Event %d: expected retry 3000, got %d", i, event.Retry)
		}
		if i > 1 && event.Retry != 3000 {
			t.Errorf("Event %d: expected retry 3000 (persisted), got %d", i, event.Retry)
		}
	}

	// Should reach EOF
	_, err := decoder.Decode(reader)
	if err != io.EOF {
		t.Errorf("Expected io.EOF at end of stream, got %v", err)
	}
}

// TestIntegration_ContentTypeDetectionFlow tests the complete flow.
func TestIntegration_ContentTypeDetectionFlow(t *testing.T) {
	contentTypes := []string{
		"text/event-stream",
		"text/event-stream; charset=utf-8",
		"TEXT/EVENT-STREAM",
	}

	detector := NewDefaultAutoDetector()
	factory := NewDefaultDecoderFactory()

	for _, ct := range contentTypes {
		t.Run(ct, func(t *testing.T) {
			// Detect format
			format := detector.DetectFromContentType(ct)
			if format != StreamFormatSSE {
				t.Errorf("Expected SSE format, got %s", format)
			}

			// Create decoder
			decoder, err := factory.CreateDecoder(format)
			if err != nil {
				t.Fatalf("Failed to create decoder: %v", err)
			}

			// Verify decoder works
			if decoder.Format() != StreamFormatSSE {
				t.Errorf("Expected SSE decoder, got %s", decoder.Format())
			}
		})
	}
}

// TestIntegration_ErrorHandling tests error handling across components.
func TestIntegration_ErrorHandling(t *testing.T) {
	factory := NewDefaultDecoderFactory()

	// Test unsupported format
	_, err := factory.CreateDecoder(StreamFormat("unsupported"))
	if err == nil {
		t.Error("Expected error for unsupported format")
	}

	// Test unknown detection
	detector := NewDefaultAutoDetector()
	format := detector.DetectFromContentType("application/json")
	if format != StreamFormatUnknown {
		t.Errorf("Expected unknown format, got %s", format)
	}
}

// CustomTestDecoder is a custom decoder for testing.
type CustomTestDecoder struct{}

func (d *CustomTestDecoder) Decode(reader io.Reader) (StreamEvent, error) {
	return StreamEvent{
		Type: "custom",
		Data: "custom data",
	}, nil
}

func (d *CustomTestDecoder) Format() StreamFormat {
	return "custom"
}
