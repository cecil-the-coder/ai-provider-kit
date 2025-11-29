package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// SSEWriter handles Server-Sent Events (SSE) writing for streaming responses
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSEWriter and sets up SSE headers
// Returns an error if the http.ResponseWriter does not support flushing
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Ensure the ResponseWriter supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported: ResponseWriter does not implement http.Flusher")
	}

	return &SSEWriter{
		w:       w,
		flusher: flusher,
	}, nil
}

// WriteChunk writes a ChatCompletionChunk as an SSE event
func (s *SSEWriter) WriteChunk(chunk types.ChatCompletionChunk) error {
	chunkData, err := json.Marshal(chunk)
	if err != nil {
		s.WriteError("SERIALIZATION_ERROR", "Failed to serialize chunk: "+err.Error())
		return err
	}

	_, _ = fmt.Fprintf(s.w, "data: %s\n\n", chunkData)
	s.flusher.Flush()
	return nil
}

// WriteDone sends the SSE completion event
func (s *SSEWriter) WriteDone() {
	_, _ = fmt.Fprintf(s.w, "data: [DONE]\n\n")
	s.flusher.Flush()
}

// WriteError sends an error as an SSE event
func (s *SSEWriter) WriteError(code, message string) {
	errorData := map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}
	data, _ := json.Marshal(errorData)
	_, _ = fmt.Fprintf(s.w, "data: %s\n\n", data)
	s.flusher.Flush()
}
