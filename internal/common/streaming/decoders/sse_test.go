package decoders

import (
	"io"
	"strings"
	"testing"
)

// TestSSEDecoder_SimpleEvent tests decoding a simple SSE event.
func TestSSEDecoder_SimpleEvent(t *testing.T) {
	sseData := `data: hello world

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", event.Type)
	}
	if event.Data != "hello world" {
		t.Errorf("Expected data 'hello world', got '%s'", event.Data)
	}
	if event.ID != "" {
		t.Errorf("Expected empty ID, got '%s'", event.ID)
	}
	if event.Retry != 0 {
		t.Errorf("Expected retry 0, got %d", event.Retry)
	}
}

// TestSSEDecoder_MultiLineData tests multi-line data support.
func TestSSEDecoder_MultiLineData(t *testing.T) {
	sseData := `data: line 1
data: line 2
data: line 3

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "line 1\nline 2\nline 3"
	if event.Data != expected {
		t.Errorf("Expected data '%s', got '%s'", expected, event.Data)
	}
}

// TestSSEDecoder_AllFields tests an event with all fields.
func TestSSEDecoder_AllFields(t *testing.T) {
	sseData := `event: update
id: 123
retry: 5000
data: test data

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Type != "update" {
		t.Errorf("Expected type 'update', got '%s'", event.Type)
	}
	if event.Data != "test data" {
		t.Errorf("Expected data 'test data', got '%s'", event.Data)
	}
	if event.ID != "123" {
		t.Errorf("Expected ID '123', got '%s'", event.ID)
	}
	if event.Retry != 5000 {
		t.Errorf("Expected retry 5000, got %d", event.Retry)
	}
}

// TestSSEDecoder_Comments tests that comment lines are ignored.
func TestSSEDecoder_Comments(t *testing.T) {
	sseData := `: This is a comment
data: actual data
: Another comment

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Data != "actual data" {
		t.Errorf("Expected data 'actual data', got '%s'", event.Data)
	}
}

// TestSSEDecoder_EmptyLines tests that empty lines dispatch events.
func TestSSEDecoder_EmptyLines(t *testing.T) {
	sseData := `data: first


data: second

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	// First event
	event1, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on first event: %v", err)
	}
	if event1.Data != "first" {
		t.Errorf("Expected data 'first', got '%s'", event1.Data)
	}

	// Second event
	event2, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on second event: %v", err)
	}
	if event2.Data != "second" {
		t.Errorf("Expected data 'second', got '%s'", event2.Data)
	}
}

// TestSSEDecoder_NoSpace tests field format without space after colon.
func TestSSEDecoder_NoSpace(t *testing.T) {
	sseData := `data:no space
event:custom

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Data != "no space" {
		t.Errorf("Expected data 'no space', got '%s'", event.Data)
	}
	if event.Type != "custom" {
		t.Errorf("Expected type 'custom', got '%s'", event.Type)
	}
}

// TestSSEDecoder_WithSpace tests field format with space after colon.
func TestSSEDecoder_WithSpace(t *testing.T) {
	sseData := `data: with space
event: custom type

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Data != "with space" {
		t.Errorf("Expected data 'with space', got '%s'", event.Data)
	}
	if event.Type != "custom type" {
		t.Errorf("Expected type 'custom type', got '%s'", event.Type)
	}
}

// TestSSEDecoder_InvalidRetry tests that invalid retry values are ignored.
func TestSSEDecoder_InvalidRetry(t *testing.T) {
	sseData := `retry: invalid
data: test

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Retry != 0 {
		t.Errorf("Expected retry 0 (ignored invalid), got %d", event.Retry)
	}
}

// TestSSEDecoder_NegativeRetry tests that negative retry values are ignored.
func TestSSEDecoder_NegativeRetry(t *testing.T) {
	sseData := `retry: -100
data: test

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Retry != 0 {
		t.Errorf("Expected retry 0 (ignored negative), got %d", event.Retry)
	}
}

// TestSSEDecoder_IDWithNull tests that IDs with null characters are ignored.
func TestSSEDecoder_IDWithNull(t *testing.T) {
	sseData := "id: test\x00null\ndata: test\n\n"
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.ID != "" {
		t.Errorf("Expected empty ID (null character), got '%s'", event.ID)
	}
}

// TestSSEDecoder_IDPersistence tests that ID persists across events.
func TestSSEDecoder_IDPersistence(t *testing.T) {
	sseData := `id: 100
data: first

data: second

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	// First event with ID
	event1, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on first event: %v", err)
	}
	if event1.ID != "100" {
		t.Errorf("Expected ID '100', got '%s'", event1.ID)
	}

	// Second event should inherit ID
	event2, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on second event: %v", err)
	}
	if event2.ID != "100" {
		t.Errorf("Expected ID '100' (persisted), got '%s'", event2.ID)
	}
}

// TestSSEDecoder_RetryPersistence tests that retry persists across events.
func TestSSEDecoder_RetryPersistence(t *testing.T) {
	sseData := `retry: 3000
data: first

data: second

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	// First event with retry
	event1, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on first event: %v", err)
	}
	if event1.Retry != 3000 {
		t.Errorf("Expected retry 3000, got %d", event1.Retry)
	}

	// Second event should inherit retry
	event2, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on second event: %v", err)
	}
	if event2.Retry != 3000 {
		t.Errorf("Expected retry 3000 (persisted), got %d", event2.Retry)
	}
}

// TestSSEDecoder_EventTypeNotPersisted tests that event type doesn't persist.
func TestSSEDecoder_EventTypeNotPersisted(t *testing.T) {
	sseData := `event: custom
data: first

data: second

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	// First event with custom type
	event1, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on first event: %v", err)
	}
	if event1.Type != "custom" {
		t.Errorf("Expected type 'custom', got '%s'", event1.Type)
	}

	// Second event should have default type
	event2, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on second event: %v", err)
	}
	if event2.Type != "message" {
		t.Errorf("Expected type 'message' (default), got '%s'", event2.Type)
	}
}

// TestSSEDecoder_MalformedLines tests that malformed lines are skipped.
func TestSSEDecoder_MalformedLines(t *testing.T) {
	sseData := `no colon here
data: valid data
another malformed line

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Data != "valid data" {
		t.Errorf("Expected data 'valid data', got '%s'", event.Data)
	}
}

// TestSSEDecoder_UnknownFields tests that unknown fields are ignored.
func TestSSEDecoder_UnknownFields(t *testing.T) {
	sseData := `custom: value
unknown: ignored
data: test

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Data != "test" {
		t.Errorf("Expected data 'test', got '%s'", event.Data)
	}
}

// TestSSEDecoder_EmptyData tests event with empty data field.
func TestSSEDecoder_EmptyData(t *testing.T) {
	sseData := `data:

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Data != "" {
		t.Errorf("Expected empty data, got '%s'", event.Data)
	}
}

// TestSSEDecoder_EOFWithoutData tests EOF without any data.
func TestSSEDecoder_EOFWithoutData(t *testing.T) {
	sseData := ``
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	_, err := decoder.Decode(reader)
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}
}

// TestSSEDecoder_EOFWithBufferedData tests EOF with buffered data.
func TestSSEDecoder_EOFWithBufferedData(t *testing.T) {
	sseData := `data: final event`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Expected event before EOF, got error: %v", err)
	}
	if event.Data != "final event" {
		t.Errorf("Expected data 'final event', got '%s'", event.Data)
	}

	// Next read should return EOF
	_, err = decoder.Decode(reader)
	if err != io.EOF {
		t.Errorf("Expected io.EOF on second read, got %v", err)
	}
}

// TestSSEDecoder_MultipleEvents tests decoding multiple events.
func TestSSEDecoder_MultipleEvents(t *testing.T) {
	sseData := `event: start
data: first

event: middle
data: second

event: end
data: third

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	expected := []struct {
		eventType string
		data      string
	}{
		{"start", "first"},
		{"middle", "second"},
		{"end", "third"},
	}

	for i, exp := range expected {
		event, err := decoder.Decode(reader)
		if err != nil {
			t.Fatalf("Unexpected error on event %d: %v", i, err)
		}
		if event.Type != exp.eventType {
			t.Errorf("Event %d: expected type '%s', got '%s'", i, exp.eventType, event.Type)
		}
		if event.Data != exp.data {
			t.Errorf("Event %d: expected data '%s', got '%s'", i, exp.data, event.Data)
		}
	}

	// EOF after all events
	_, err := decoder.Decode(reader)
	if err != io.EOF {
		t.Errorf("Expected io.EOF after all events, got %v", err)
	}
}

// TestSSEDecoder_JSONData tests decoding JSON data.
func TestSSEDecoder_JSONData(t *testing.T) {
	sseData := `data: {"message": "hello", "count": 42}

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := `{"message": "hello", "count": 42}`
	if event.Data != expected {
		t.Errorf("Expected data '%s', got '%s'", expected, event.Data)
	}
}

// TestSSEDecoder_Format tests the Format method.
func TestSSEDecoder_Format(t *testing.T) {
	decoder := NewSSEDecoder()
	if decoder.Format() != StreamFormatSSE {
		t.Errorf("Expected format %s, got %s", StreamFormatSSE, decoder.Format())
	}
}

// TestSSEDecoderWithReader tests the convenience wrapper.
func TestSSEDecoderWithReader(t *testing.T) {
	sseData := `data: test event

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoderWithReader(reader)

	event, err := decoder.Decode()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Data != "test event" {
		t.Errorf("Expected data 'test event', got '%s'", event.Data)
	}

	if decoder.Format() != StreamFormatSSE {
		t.Errorf("Expected format %s, got %s", StreamFormatSSE, decoder.Format())
	}
}

// TestSSEDecoder_RealWorldExample tests a real-world SSE stream.
func TestSSEDecoder_RealWorldExample(t *testing.T) {
	// Example from OpenAI API
	sseData := `event: message
id: msg_001
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

event: message
id: msg_002
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}

event: done
id: msg_003
data: [DONE]

`
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	// First message
	event1, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on first event: %v", err)
	}
	if event1.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", event1.Type)
	}
	if event1.ID != "msg_001" {
		t.Errorf("Expected ID 'msg_001', got '%s'", event1.ID)
	}
	if !strings.Contains(event1.Data, "Hello") {
		t.Error("Expected data to contain 'Hello'")
	}

	// Second message
	event2, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on second event: %v", err)
	}
	if event2.ID != "msg_002" {
		t.Errorf("Expected ID 'msg_002', got '%s'", event2.ID)
	}

	// Done message
	event3, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error on done event: %v", err)
	}
	if event3.Type != "done" {
		t.Errorf("Expected type 'done', got '%s'", event3.Type)
	}
	if event3.Data != "[DONE]" {
		t.Errorf("Expected data '[DONE]', got '%s'", event3.Data)
	}
}

// TestSSEDecoder_WindowsLineEndings tests handling of CRLF line endings.
func TestSSEDecoder_WindowsLineEndings(t *testing.T) {
	sseData := "data: test\r\n\r\n"
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if event.Data != "test" {
		t.Errorf("Expected data 'test', got '%s'", event.Data)
	}
}

// TestSSEDecoder_MixedLineEndings tests handling of mixed line endings.
func TestSSEDecoder_MixedLineEndings(t *testing.T) {
	sseData := "data: line1\r\ndata: line2\n\n"
	reader := strings.NewReader(sseData)
	decoder := NewSSEDecoder()

	event, err := decoder.Decode(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := "line1\nline2"
	if event.Data != expected {
		t.Errorf("Expected data '%s', got '%s'", expected, event.Data)
	}
}

// BenchmarkSSEDecoder benchmarks the SSE decoder performance.
func BenchmarkSSEDecoder(b *testing.B) {
	sseData := `event: message
id: 123
data: {"message": "test"}

`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(sseData)
		decoder := NewSSEDecoder()
		_, _ = decoder.Decode(reader)
	}
}

// BenchmarkSSEDecoder_MultiLine benchmarks multi-line data decoding.
func BenchmarkSSEDecoder_MultiLine(b *testing.B) {
	sseData := `data: line 1
data: line 2
data: line 3
data: line 4
data: line 5

`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(sseData)
		decoder := NewSSEDecoder()
		_, _ = decoder.Decode(reader)
	}
}

// BenchmarkSSEDecoder_MultipleEvents benchmarks decoding multiple events.
func BenchmarkSSEDecoder_MultipleEvents(b *testing.B) {
	sseData := `data: event 1

data: event 2

data: event 3

`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(sseData)
		decoder := NewSSEDecoder()
		for {
			_, err := decoder.Decode(reader)
			if err == io.EOF {
				break
			}
		}
	}
}
