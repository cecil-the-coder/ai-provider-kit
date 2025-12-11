# Stream Decoders Package

This package provides a flexible framework for decoding streaming responses in various formats, including Server-Sent Events (SSE), NDJSON, and custom event streams.

## Overview

The decoders package implements the following key components:

1. **Stream Format Types** - Constants for common streaming formats (SSE, NDJSON, EventStream)
2. **Stream Events** - A generic structure representing decoded events
3. **Stream Decoders** - Implementations for parsing different stream formats
4. **Decoder Factory** - Factory pattern for creating and managing decoders
5. **Auto-Detection** - Automatic format detection from Content-Type headers or content

## Core Interfaces

### StreamDecoder

The main interface for decoding streaming events:

```go
type StreamDecoder interface {
    Decode(reader io.Reader) (StreamEvent, error)
    Format() StreamFormat
}
```

### DecoderFactory

Creates and manages decoder instances:

```go
type DecoderFactory interface {
    CreateDecoder(format StreamFormat) (StreamDecoder, error)
    RegisterDecoder(format StreamFormat, decoder StreamDecoder)
    SupportedFormats() []StreamFormat
}
```

### AutoDetector

Detects stream format from metadata or content:

```go
type AutoDetector interface {
    DetectFromContentType(contentType string) StreamFormat
    DetectFromBytes(bytes []byte) StreamFormat
}
```

## Usage Examples

### Basic SSE Decoding

```go
// Create a factory
factory := decoders.NewDefaultDecoderFactory()

// Create an SSE decoder
decoder, err := factory.CreateDecoder(decoders.StreamFormatSSE)
if err != nil {
    log.Fatal(err)
}

// Decode events from a stream
for {
    event, err := decoder.Decode(reader)
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Printf("Error: %v", err)
        continue
    }

    fmt.Printf("Event: %s, Data: %s\n", event.Type, event.Data)
}
```

### Auto-Detection

```go
detector := decoders.NewDefaultAutoDetector()
factory := decoders.NewDefaultDecoderFactory()

// Detect format from HTTP header
format := detector.DetectFromContentType(resp.Header.Get("Content-Type"))

// Or detect from initial bytes
initialBytes := make([]byte, 512)
n, _ := reader.Read(initialBytes)
format := detector.DetectFromBytes(initialBytes[:n])

// Create appropriate decoder
decoder, err := factory.CreateDecoder(format)
```

### Custom Decoder Registration

```go
// Implement a custom decoder
type MyCustomDecoder struct{}

func (d *MyCustomDecoder) Decode(reader io.Reader) (decoders.StreamEvent, error) {
    // Custom parsing logic
    return decoders.StreamEvent{
        Type: "custom",
        Data: "parsed data",
    }, nil
}

func (d *MyCustomDecoder) Format() decoders.StreamFormat {
    return "my-custom-format"
}

// Register it
factory := decoders.NewDefaultDecoderFactory()
factory.RegisterDecoder("my-custom-format", &MyCustomDecoder{})
```

## Stream Formats

### SSE (Server-Sent Events)

The SSE decoder follows the [WHATWG SSE specification](https://html.spec.whatwg.org/multipage/server-sent-events.html):

- Supports `event:`, `data:`, `id:`, and `retry:` fields
- Multi-line data concatenation with newlines
- Comment lines (starting with `:`) are ignored
- Empty lines dispatch events
- `id` and `retry` values persist across events

**Example SSE stream:**

```
event: message
id: 123
data: {"content": "Hello"}

event: message
id: 124
data: {"content": "World"}

data: [DONE]
```

### NDJSON (Newline-Delimited JSON)

NDJSON format is detected but not yet implemented. Each line contains a complete JSON object:

```
{"message": "first"}
{"message": "second"}
```

### EventStream

Generic event stream format for custom protocols.

## StreamEvent Structure

```go
type StreamEvent struct {
    Type  string  // Event type (e.g., "message", "error")
    Data  string  // Event payload/content
    ID    string  // Event identifier
    Retry int     // Reconnection time in milliseconds
    Raw   string  // Raw event data for debugging
}
```

## Performance

The SSE decoder is optimized for streaming performance:

- **BenchmarkSSEDecoder**: ~1,491 ns/op (single event)
- **BenchmarkSSEDecoder_MultiLine**: ~2,480 ns/op (5-line event)
- **BenchmarkSSEDecoder_MultipleEvents**: ~4,743 ns/op (3 events)

## Thread Safety

- `DefaultDecoderFactory` is thread-safe and can be used concurrently
- Individual `SSEDecoder` instances maintain state and should not be shared across goroutines
- Each `CreateDecoder()` call returns a fresh decoder instance

## Error Handling

Decoders handle errors according to their format specifications:

- **SSE Decoder**: Malformed lines are silently skipped per SSE spec
- **EOF**: Returned when stream ends, buffered data is dispatched first
- **Parse Errors**: Format-specific errors are returned for invalid data

## Best Practices

1. **Create fresh decoders**: Use `CreateDecoder()` for each new stream
2. **Handle EOF properly**: Check for `io.EOF` to detect normal stream completion
3. **Validate events**: Check event fields based on your protocol requirements
4. **Use auto-detection**: Leverage `AutoDetector` when format is uncertain
5. **Register early**: Register custom decoders during initialization

## Testing

Comprehensive test coverage includes:

- Unit tests for all decoders and utilities
- Integration tests for factory and auto-detection
- Real-world SSE stream examples
- Benchmark tests for performance validation
- Thread safety tests for concurrent usage

Run tests:

```bash
go test ./pkg/providers/common/streaming/decoders/...
```

Run benchmarks:

```bash
go test -bench=. ./pkg/providers/common/streaming/decoders/...
```
