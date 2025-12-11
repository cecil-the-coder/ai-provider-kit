package decoders

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// SSEDecoder implements StreamDecoder for Server-Sent Events (SSE) format.
// It properly parses SSE fields including multi-line data support.
//
// SSE format specification (https://html.spec.whatwg.org/multipage/server-sent-events.html):
//   - Lines starting with ':' are comments (ignored)
//   - Empty lines dispatch the current event
//   - Field format: "field: value" or "field:value"
//   - Supported fields: event, data, id, retry
//   - Multi-line data is concatenated with newlines
//
// Note: SSEDecoder maintains internal state across Decode calls.
// Create a new SSEDecoder instance for each independent stream.
type SSEDecoder struct {
	reader    *bufio.Reader
	eventType string
	dataLines []string
	eventID   string
	retryMS   int
}

// NewSSEDecoder creates a new SSE decoder.
func NewSSEDecoder() *SSEDecoder {
	return &SSEDecoder{
		dataLines: make([]string, 0),
	}
}

// Format returns the stream format this decoder handles.
func (d *SSEDecoder) Format() StreamFormat {
	return StreamFormatSSE
}

// Decode reads and decodes the next SSE event from the stream.
// It follows the SSE specification for proper field parsing and event dispatching.
func (d *SSEDecoder) Decode(reader io.Reader) (StreamEvent, error) {
	// Initialize reader on first use
	if d.reader == nil {
		d.reader = bufio.NewReader(reader)
	}

	// Read lines until we hit an empty line (event boundary)
	for {
		line, err := d.reader.ReadString('\n')

		// Handle EOF - may have partial line
		if err == io.EOF {
			// Process any remaining line content
			if line != "" {
				line = strings.TrimRight(line, "\r\n")
				if line != "" && !strings.HasPrefix(line, ":") {
					_ = d.parseField(line)
				}
			}
			// If we have buffered data, dispatch it before returning EOF
			if len(d.dataLines) > 0 || d.eventType != "" {
				event := d.buildEvent()
				d.reset()
				return event, nil
			}
			return StreamEvent{}, io.EOF
		}

		// Handle other errors
		if err != nil {
			return StreamEvent{}, fmt.Errorf("error reading stream: %w", err)
		}

		// Remove trailing newline/carriage return
		line = strings.TrimRight(line, "\r\n")

		// Empty line = dispatch event
		if line == "" {
			if len(d.dataLines) > 0 || d.eventType != "" {
				event := d.buildEvent()
				d.reset()
				return event, nil
			}
			// Skip multiple empty lines
			continue
		}

		// Comment line (starts with ':')
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse field
		if err := d.parseField(line); err != nil {
			// Skip malformed lines per SSE spec
			continue
		}
	}
}

// parseField parses a single SSE field line.
// Format: "field: value" or "field:value"
func (d *SSEDecoder) parseField(line string) error {
	// Find the first colon
	colonIndex := strings.Index(line, ":")
	if colonIndex == -1 {
		// No colon = ignore line per spec
		return fmt.Errorf("no colon in field")
	}

	field := line[:colonIndex]
	value := ""

	// Extract value, skipping optional leading space after colon
	if colonIndex+1 < len(line) {
		if line[colonIndex+1] == ' ' {
			value = line[colonIndex+2:]
		} else {
			value = line[colonIndex+1:]
		}
	}

	// Process field based on type
	switch field {
	case "event":
		d.eventType = value

	case "data":
		d.dataLines = append(d.dataLines, value)

	case "id":
		// ID must not contain null characters per spec
		if !strings.Contains(value, "\x00") {
			d.eventID = value
		}

	case "retry":
		// Parse retry as milliseconds
		if ms, err := strconv.Atoi(value); err == nil && ms >= 0 {
			d.retryMS = ms
		}

	default:
		// Unknown fields are ignored per spec
	}

	return nil
}

// buildEvent constructs a StreamEvent from the buffered fields.
func (d *SSEDecoder) buildEvent() StreamEvent {
	// Join data lines with newlines (per SSE spec)
	data := strings.Join(d.dataLines, "\n")

	// Default event type is "message" if not specified
	eventType := d.eventType
	if eventType == "" {
		eventType = "message"
	}

	return StreamEvent{
		Type:  eventType,
		Data:  data,
		ID:    d.eventID,
		Retry: d.retryMS,
		Raw:   data, // Store original data for debugging
	}
}

// reset clears the buffered event fields for the next event.
// Note: eventID and retryMS persist across events per SSE spec.
func (d *SSEDecoder) reset() {
	d.eventType = ""
	d.dataLines = make([]string, 0)
	// Note: d.eventID and d.retryMS are intentionally NOT reset
}

// SSEDecoderWithReader is a convenience wrapper that creates an SSE decoder
// bound to a specific reader. This is useful when you want to reuse the same
// decoder instance across multiple Decode calls.
type SSEDecoderWithReader struct {
	decoder *SSEDecoder
	reader  io.Reader
}

// NewSSEDecoderWithReader creates an SSE decoder bound to a specific reader.
func NewSSEDecoderWithReader(reader io.Reader) *SSEDecoderWithReader {
	return &SSEDecoderWithReader{
		decoder: NewSSEDecoder(),
		reader:  reader,
	}
}

// Decode reads the next event from the bound reader.
func (d *SSEDecoderWithReader) Decode() (StreamEvent, error) {
	return d.decoder.Decode(d.reader)
}

// Format returns the stream format.
func (d *SSEDecoderWithReader) Format() StreamFormat {
	return d.decoder.Format()
}
