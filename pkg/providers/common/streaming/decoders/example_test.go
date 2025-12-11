package decoders_test

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/streaming/decoders"
)

// ExampleSSEDecoder demonstrates basic SSE decoding
func ExampleSSEDecoder() {
	// Sample SSE stream
	sseStream := `event: start
data: {"status": "started"}

event: message
data: Hello

event: message
data: World

event: done
data: {"status": "completed"}

`

	// Create decoder
	decoder := decoders.NewSSEDecoder()
	reader := strings.NewReader(sseStream)

	// Decode events
	for {
		event, err := decoder.Decode(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		fmt.Printf("%s: %s\n", event.Type, event.Data)
	}

	// Output:
	// start: {"status": "started"}
	// message: Hello
	// message: World
	// done: {"status": "completed"}
}

// ExampleDefaultDecoderFactory demonstrates factory usage
func ExampleDefaultDecoderFactory() {
	// Create factory
	factory := decoders.NewDefaultDecoderFactory()

	// List supported formats
	formats := factory.SupportedFormats()
	fmt.Printf("Supported formats: %v\n", formats)

	// Create SSE decoder
	decoder, err := factory.CreateDecoder(decoders.StreamFormatSSE)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created decoder for format: %s\n", decoder.Format())

	// Output:
	// Supported formats: [sse]
	// Created decoder for format: sse
}

// ExampleDefaultAutoDetector demonstrates auto-detection
func ExampleDefaultAutoDetector() {
	detector := decoders.NewDefaultAutoDetector()

	// Detect from Content-Type header
	format1 := detector.DetectFromContentType("text/event-stream")
	fmt.Printf("Detected from header: %s\n", format1)

	// Detect from content
	sseContent := []byte("data: test\n\n")
	format2 := detector.DetectFromBytes(sseContent)
	fmt.Printf("Detected from content: %s\n", format2)

	// Output:
	// Detected from header: sse
	// Detected from content: sse
}

// ExampleSSEDecoder_multiLine demonstrates multi-line data handling
func ExampleSSEDecoder_multiLine() {
	sseStream := `data: line 1
data: line 2
data: line 3

`

	decoder := decoders.NewSSEDecoder()
	reader := strings.NewReader(sseStream)

	event, _ := decoder.Decode(reader)
	fmt.Println(event.Data)

	// Output:
	// line 1
	// line 2
	// line 3
}

// ExampleSSEDecoder_withID demonstrates ID persistence
func ExampleSSEDecoder_withID() {
	sseStream := `id: msg-001
data: first

data: second

id: msg-003
data: third

`

	decoder := decoders.NewSSEDecoder()
	reader := strings.NewReader(sseStream)

	for i := 1; i <= 3; i++ {
		event, _ := decoder.Decode(reader)
		fmt.Printf("Event %d - ID: %s, Data: %s\n", i, event.ID, event.Data)
	}

	// Output:
	// Event 1 - ID: msg-001, Data: first
	// Event 2 - ID: msg-001, Data: second
	// Event 3 - ID: msg-003, Data: third
}

// ExampleAutoDetector_integration demonstrates complete workflow
func ExampleAutoDetector_integration() {
	detector := decoders.NewDefaultAutoDetector()
	factory := decoders.NewDefaultDecoderFactory()

	// Simulate receiving a stream with Content-Type
	contentType := "text/event-stream; charset=utf-8"
	sseData := "data: Hello, World!\n\n"

	// Detect format
	format := detector.DetectFromContentType(contentType)
	fmt.Printf("Detected format: %s\n", format)

	// Create decoder
	decoder, _ := factory.CreateDecoder(format)

	// Decode event
	reader := strings.NewReader(sseData)
	event, _ := decoder.Decode(reader)

	fmt.Printf("Received: %s\n", event.Data)

	// Output:
	// Detected format: sse
	// Received: Hello, World!
}
