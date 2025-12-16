package gemini

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestGeminiProvider_GenerateChatCompletion_Streaming(t *testing.T) {
	// Create a mock server for streaming
	mockServer := createMockServer(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a streaming request
		if !strings.Contains(r.URL.Path, "streamGenerateContent") {
			t.Errorf("Expected streaming endpoint, got %s", r.URL.Path)
		}

		// Write SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Write chunks
		chunks := []string{
			`data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`,
			`data: {"candidates":[{"content":{"parts":[{"text":" world"}]}}]}`,
			`data: {"candidates":[{"content":{"parts":[{"text":"!"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":10,"totalTokenCount":15}}`,
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "%s\n\n", chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	})
	defer mockServer.Close()

	// Create provider
	provider := createProviderWithMockServer(mockServer.URL)

	// Test streaming
	options := types.GenerateOptions{
		Prompt: "Hello",
		Model:  "gemini-1.5-pro",
		Stream: true,
	}

	stream, err := provider.GenerateChatCompletion(context.Background(), options)
	if err != nil {
		t.Fatalf("GenerateChatCompletion failed: %v", err)
	}

	if stream == nil {
		t.Fatal("Expected non-nil stream")
	}

	// Read chunks
	var fullContent strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read chunk: %v", err)
		}
		fullContent.WriteString(chunk.Content)
	}

	content := fullContent.String()
	if !strings.Contains(content, "Hello") {
		t.Errorf("Expected content to contain 'Hello', got '%s'", content)
	}
}

func TestMockStream(t *testing.T) {
	stream := &MockStream{
		chunks: []types.ChatCompletionChunk{
			{Content: "Hello", Done: false},
			{Content: " World", Done: true},
		},
	}

	// Read first chunk
	chunk1, err := stream.Next()
	if err != nil {
		t.Fatalf("Failed to read first chunk: %v", err)
	}
	if chunk1.Content != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", chunk1.Content)
	}

	// Read second chunk
	chunk2, err := stream.Next()
	if err != nil {
		t.Fatalf("Failed to read second chunk: %v", err)
	}
	if chunk2.Content != " World" {
		t.Errorf("Expected ' World', got '%s'", chunk2.Content)
	}

	// Close stream
	err = stream.Close()
	if err != nil {
		t.Fatalf("Failed to close stream: %v", err)
	}

	// Verify stream was reset
	if stream.index != 0 {
		t.Errorf("Expected index to be reset to 0, got %d", stream.index)
	}
}

func TestGeminiStream_Close(t *testing.T) {
	// Create a mock response
	mockResp := createMockHTTPResponse("")

	stream := &GeminiStream{
		response: mockResp,
		done:     false,
	}

	err := stream.Close()
	if err != nil {
		t.Fatalf("Failed to close stream: %v", err)
	}

	if !stream.done {
		t.Error("Expected stream to be marked as done after close")
	}
}

func TestGeminiStream_NextEOF(t *testing.T) {
	// Create a stream with EOF response
	mockResp := createMockHTTPResponse("")

	stream := &GeminiStream{
		response: mockResp,
		reader:   bufio.NewReader(mockResp.Body),
		done:     false,
	}

	// Reading from empty stream should return EOF
	_, err := stream.Next()
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}

	// Stream should be marked as done
	if !stream.done {
		t.Error("Expected stream to be marked as done")
	}
}

func TestGeminiStream_NextWithData(t *testing.T) {
	// Create a stream with SSE data
	sseData := `data: {"candidates":[{"content":{"parts":[{"text":"Test"}]}}]}

`
	mockResp := createMockHTTPResponse(sseData)

	stream := &GeminiStream{
		response: mockResp,
		reader:   bufio.NewReader(mockResp.Body),
		done:     false,
	}

	// Read the chunk
	chunk, err := stream.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("Unexpected error: %v", err)
	}

	if chunk.Content != "Test" {
		t.Errorf("Expected content 'Test', got '%s'", chunk.Content)
	}
}
