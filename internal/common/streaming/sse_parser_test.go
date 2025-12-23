package streaming

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestGenericSSEStream_OpenAIFormat tests the generic SSE stream with OpenAI-compatible parser
func TestGenericSSEStream_OpenAIFormat(t *testing.T) {
	// Create mock SSE response data
	sseData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`

	// Create HTTP response with SSE data
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	// Create parser and stream
	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	// Read first chunk
	chunk1, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error on first chunk, got: %v", err)
	}
	if chunk1.Content != "Hello" {
		t.Errorf("Expected content 'Hello', got: %s", chunk1.Content)
	}
	if chunk1.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got: %s", chunk1.Model)
	}
	if chunk1.Done {
		t.Error("Expected Done to be false for first chunk")
	}

	// Read second chunk
	chunk2, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error on second chunk, got: %v", err)
	}
	if chunk2.Content != " world" {
		t.Errorf("Expected content ' world', got: %s", chunk2.Content)
	}
	if chunk2.Done {
		t.Error("Expected Done to be false for second chunk")
	}

	// Read third chunk (finish_reason set)
	chunk3, err := stream.Next()
	if err != io.EOF {
		t.Errorf("Expected io.EOF on finish chunk, got: %v", err)
	}
	if !chunk3.Done {
		t.Error("Expected Done to be true when finish_reason is set")
	}

	// Subsequent reads should return EOF
	chunk4, err := stream.Next()
	if err != io.EOF {
		t.Errorf("Expected io.EOF after stream done, got: %v", err)
	}
	if !chunk4.Done {
		t.Error("Expected Done to be true after stream is complete")
	}
}

// TestGenericSSEStream_EmptyLines tests that empty lines are properly skipped
func TestGenericSSEStream_EmptyLines(t *testing.T) {
	sseData := `

data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}]}


data: [DONE]

`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if chunk.Content != "test" {
		t.Errorf("Expected content 'test', got: %s", chunk.Content)
	}
}

// TestGenericSSEStream_Comments tests that SSE comment lines are properly skipped
func TestGenericSSEStream_Comments(t *testing.T) {
	sseData := `: This is a comment
data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}]}
: Another comment
data: [DONE]
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if chunk.Content != "test" {
		t.Errorf("Expected content 'test', got: %s", chunk.Content)
	}
}

// TestGenericSSEStream_ToolCalls tests parsing of tool calls in the stream
func TestGenericSSEStream_ToolCalls(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"NYC\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(chunk.Choices) == 0 {
		t.Fatal("Expected at least one choice")
	}
	if len(chunk.Choices[0].Delta.ToolCalls) == 0 {
		t.Fatal("Expected tool calls in delta")
	}

	toolCall := chunk.Choices[0].Delta.ToolCalls[0]
	if toolCall.ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got: %s", toolCall.ID)
	}
	if toolCall.Function.Name != "get_weather" {
		t.Errorf("Expected function name 'get_weather', got: %s", toolCall.Function.Name)
	}
	if toolCall.Function.Arguments != "{\"location\":\"NYC\"}" {
		t.Errorf("Expected arguments '{\"location\":\"NYC\"}', got: %s", toolCall.Function.Arguments)
	}
}

// TestGenericSSEStream_UsageInfo tests parsing of usage information
func TestGenericSSEStream_UsageInfo(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}

data: [DONE]
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	// Skip first chunk
	_, _ = stream.Next()

	// Read chunk with usage
	chunk, err := stream.Next()
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got: %v", err)
	}

	if chunk.Usage.PromptTokens != 10 {
		t.Errorf("Expected prompt tokens 10, got: %d", chunk.Usage.PromptTokens)
	}
	if chunk.Usage.CompletionTokens != 20 {
		t.Errorf("Expected completion tokens 20, got: %d", chunk.Usage.CompletionTokens)
	}
	if chunk.Usage.TotalTokens != 30 {
		t.Errorf("Expected total tokens 30, got: %d", chunk.Usage.TotalTokens)
	}
}

// TestGenericSSEStream_MalformedJSON tests that malformed JSON is skipped
func TestGenericSSEStream_MalformedJSON(t *testing.T) {
	sseData := `data: {invalid json}
data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"valid"},"finish_reason":null}]}
data: [DONE]
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	// Should skip malformed JSON and return the valid chunk
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if chunk.Content != "valid" {
		t.Errorf("Expected content 'valid', got: %s", chunk.Content)
	}
}

// TestGenericSSEStream_DataWithoutSpace tests "data:" format without space
func TestGenericSSEStream_DataWithoutSpace(t *testing.T) {
	sseData := `data:{"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}]}
data:[DONE]
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if chunk.Content != "test" {
		t.Errorf("Expected content 'test', got: %s", chunk.Content)
	}
}

// TestGenericSSEStream_Close tests that Close properly cleans up resources
func TestGenericSSEStream_Close(t *testing.T) {
	sseData := `data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}]}
data: [DONE]
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)

	// Close immediately
	err := stream.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got: %v", err)
	}

	// After close, Next should return EOF
	chunk, err := stream.Next()
	if err != io.EOF {
		t.Errorf("Expected io.EOF after close, got: %v", err)
	}
	if !chunk.Done {
		t.Error("Expected Done to be true after close")
	}
}

// TestOpenAICompatibleParser_IsDone tests the IsDone method
func TestOpenAICompatibleParser_IsDone(t *testing.T) {
	parser := NewOpenAICompatibleParser()

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "DONE signal",
			line:     "[DONE]",
			expected: true,
		},
		{
			name:     "DONE signal with spaces",
			line:     "  [DONE]  ",
			expected: true,
		},
		{
			name:     "Regular data",
			line:     `{"id":"test"}`,
			expected: false,
		},
		{
			name:     "Empty string",
			line:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.IsDone(tt.line)
			if result != tt.expected {
				t.Errorf("IsDone(%q) = %v, expected %v", tt.line, result, tt.expected)
			}
		})
	}
}

// TestOpenAICompatibleParser_ParseLine tests the ParseLine method directly
func TestOpenAICompatibleParser_ParseLine(t *testing.T) {
	parser := NewOpenAICompatibleParser()

	tests := []struct {
		name          string
		line          string
		expectError   bool
		expectedChunk types.ChatCompletionChunk
	}{
		{
			name: "Simple content chunk",
			line: `{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			expectedChunk: types.ChatCompletionChunk{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "gpt-4",
				Content: "Hello",
				Done:    false,
			},
		},
		{
			name: "Chunk with finish_reason",
			line: `{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			expectedChunk: types.ChatCompletionChunk{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "gpt-4",
				Done:    true,
			},
		},
		{
			name:        "Invalid JSON",
			line:        `{invalid}`,
			expectError: true,
		},
		{
			name: "Empty choices array",
			line: `{"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[]}`,
			expectedChunk: types.ChatCompletionChunk{
				ID:      "chatcmpl-123",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "gpt-4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := parser.ParseLine(tt.line)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if chunk.ID != tt.expectedChunk.ID {
				t.Errorf("ID: got %q, expected %q", chunk.ID, tt.expectedChunk.ID)
			}
			if chunk.Model != tt.expectedChunk.Model {
				t.Errorf("Model: got %q, expected %q", chunk.Model, tt.expectedChunk.Model)
			}
			if chunk.Content != tt.expectedChunk.Content {
				t.Errorf("Content: got %q, expected %q", chunk.Content, tt.expectedChunk.Content)
			}
			if chunk.Done != tt.expectedChunk.Done {
				t.Errorf("Done: got %v, expected %v", chunk.Done, tt.expectedChunk.Done)
			}
		})
	}
}

// MockSSELineParser is a mock implementation for testing
type MockSSELineParser struct {
	parseFunc  func(line string) (types.ChatCompletionChunk, error)
	isDoneFunc func(line string) bool
}

func (m *MockSSELineParser) ParseLine(line string) (types.ChatCompletionChunk, error) {
	if m.parseFunc != nil {
		return m.parseFunc(line)
	}
	return types.ChatCompletionChunk{Content: line}, nil
}

func (m *MockSSELineParser) IsDone(line string) bool {
	if m.isDoneFunc != nil {
		return m.isDoneFunc(line)
	}
	return line == "DONE"
}

// TestGenericSSEStream_CustomParser tests using a custom parser
func TestGenericSSEStream_CustomParser(t *testing.T) {
	sseData := `data: first
data: second
data: DONE
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := &MockSSELineParser{}
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	// First chunk
	chunk1, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error on first chunk, got: %v", err)
	}
	if chunk1.Content != "first" {
		t.Errorf("Expected content 'first', got: %s", chunk1.Content)
	}

	// Second chunk
	chunk2, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error on second chunk, got: %v", err)
	}
	if chunk2.Content != "second" {
		t.Errorf("Expected content 'second', got: %s", chunk2.Content)
	}

	// Done chunk
	chunk3, err := stream.Next()
	if err != io.EOF {
		t.Errorf("Expected io.EOF on done chunk, got: %v", err)
	}
	if !chunk3.Done {
		t.Error("Expected Done to be true")
	}
}

// TestGenericSSEStream_ConcurrentAccess tests thread safety
func TestGenericSSEStream_ConcurrentAccess(t *testing.T) {
	sseData := `data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}]}
data: [DONE]
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	// Try to read and close concurrently
	done := make(chan bool)
	go func() {
		_, _ = stream.Next()
		done <- true
	}()

	go func() {
		_ = stream.Close()
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Should not panic or deadlock
}

// BenchmarkGenericSSEStream benchmarks the SSE stream processing
func BenchmarkGenericSSEStream(b *testing.B) {
	sseData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}]}
data: [DONE]
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(sseData))),
		}

		parser := NewOpenAICompatibleParser()
		stream := NewGenericSSEStream(resp, parser)

		for {
			_, err := stream.Next()
			if err == io.EOF {
				break
			}
		}
		_ = stream.Close()
	}
}

// TestGenericSSEStream_NonDataSSEFields tests that non-data SSE fields are skipped
func TestGenericSSEStream_NonDataSSEFields(t *testing.T) {
	sseData := `event: message
id: 123
data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}]}
retry: 1000
data: [DONE]
`

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	parser := NewOpenAICompatibleParser()
	stream := NewGenericSSEStream(resp, parser)
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if chunk.Content != "test" {
		t.Errorf("Expected content 'test', got: %s", chunk.Content)
	}
}
