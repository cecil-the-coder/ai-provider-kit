package common

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestNewStreamProcessor(t *testing.T) {
	body := bytes.NewBufferString("test data")
	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	processor := NewStreamProcessor(resp)

	if processor == nil {
		t.Fatal("expected non-nil processor")
	}
	if processor.done {
		t.Error("expected done to be false initially")
	}
	if processor.reader == nil {
		t.Error("expected non-nil reader")
	}
}

func TestStreamProcessor_NextChunk(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		expectDone  bool
		expectError bool
	}{
		{
			name:        "valid SSE data",
			data:        "data: {\"content\": \"test\"}\n",
			expectDone:  false,
			expectError: false,
		},
		{
			name:        "done marker",
			data:        "data: [DONE]\n",
			expectDone:  true,
			expectError: false,
		},
		{
			name:        "empty lines",
			data:        "\n\ndata: {\"content\": \"test\"}\n",
			expectDone:  false,
			expectError: false,
		},
		{
			name:        "non-data lines",
			data:        "event: message\ndata: {\"content\": \"test\"}\n",
			expectDone:  false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewBufferString(tt.data)
			resp := &http.Response{
				Body: io.NopCloser(body),
			}

			processor := NewStreamProcessor(resp)

			processLine := func(line string) (types.ChatCompletionChunk, error, bool) {
				if line == "[DONE]" {
					return types.ChatCompletionChunk{Done: true}, nil, true
				}
				return types.ChatCompletionChunk{Content: "test"}, nil, false
			}

			chunk, err := processor.NextChunk(processLine)

			if tt.expectError {
				if err == nil && err != io.EOF {
					t.Error("expected error but got none")
				}
			}

			if tt.expectDone {
				if !chunk.Done {
					t.Error("expected chunk to be done")
				}
				if err != io.EOF {
					t.Errorf("expected EOF error, got %v", err)
				}
			}
		})
	}
}

func TestStreamProcessor_Close(t *testing.T) {
	body := bytes.NewBufferString("test")
	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	processor := NewStreamProcessor(resp)

	if err := processor.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !processor.done {
		t.Error("expected done to be true after close")
	}
}

func TestStreamProcessor_IsDone(t *testing.T) {
	body := bytes.NewBufferString("test")
	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	processor := NewStreamProcessor(resp)

	if processor.IsDone() {
		t.Error("expected done to be false initially")
	}

	processor.MarkDone()

	if !processor.IsDone() {
		t.Error("expected done to be true after MarkDone")
	}
}

func TestStreamProcessor_MarkDone(t *testing.T) {
	body := bytes.NewBufferString("test")
	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	processor := NewStreamProcessor(resp)
	processor.MarkDone()

	if !processor.done {
		t.Error("expected done to be true")
	}
}

func TestMockStream(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "Hello"},
		{Content: " World"},
		{Done: true},
	}

	stream := NewMockStream(chunks)

	// Read first chunk
	chunk, err := stream.Next()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if chunk.Content != "Hello" {
		t.Errorf("got %q, expected %q", chunk.Content, "Hello")
	}

	// Read second chunk
	chunk, err = stream.Next()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if chunk.Content != " World" {
		t.Errorf("got %q, expected %q", chunk.Content, " World")
	}

	// Read done chunk
	chunk, err = stream.Next()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !chunk.Done {
		t.Error("expected done chunk")
	}

	// Read beyond end
	chunk, err = stream.Next()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
	if !chunk.Done {
		t.Error("expected done chunk")
	}
}

func TestMockStream_Close(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "test"},
	}

	stream := NewMockStream(chunks)

	if err := stream.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// After close, index should be reset
	if stream.index != 0 {
		t.Errorf("expected index to be 0 after close, got %d", stream.index)
	}
}

func TestNewStandardStreamParser(t *testing.T) {
	parser := NewStandardStreamParser()

	if parser == nil {
		t.Fatal("expected non-nil parser")
	}

	if parser.ContentField != "choices.0.delta.content" {
		t.Errorf("unexpected ContentField: %s", parser.ContentField)
	}
}

func TestStandardStreamParser_ParseLine(t *testing.T) {
	parser := NewStandardStreamParser()

	tests := []struct {
		name           string
		data           string
		expectError    bool
		expectDone     bool
		expectContent  string
		expectUsage    bool
		expectToolCall bool
	}{
		{
			name:          "valid content",
			data:          `{"choices": [{"delta": {"content": "Hello"}}]}`,
			expectError:   false,
			expectDone:    false,
			expectContent: "Hello",
		},
		{
			name:        "finish reason",
			data:        `{"choices": [{"delta": {}, "finish_reason": "stop"}]}`,
			expectError: false,
			expectDone:  true,
		},
		{
			name:        "with usage",
			data:        `{"choices": [{"delta": {}}], "usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}}`,
			expectError: false,
			expectUsage: true,
		},
		{
			name:        "invalid JSON",
			data:        `{invalid json}`,
			expectError: true,
		},
		{
			name:        "empty response",
			data:        `{}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, isDone, err := parser.ParseLine(tt.data)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if isDone != tt.expectDone {
				t.Errorf("got isDone=%v, expected %v", isDone, tt.expectDone)
			}

			if tt.expectContent != "" && chunk.Content != tt.expectContent {
				t.Errorf("got content %q, expected %q", chunk.Content, tt.expectContent)
			}

			if tt.expectUsage {
				if chunk.Usage.PromptTokens != 10 {
					t.Errorf("got prompt tokens %d, expected 10", chunk.Usage.PromptTokens)
				}
				if chunk.Usage.CompletionTokens != 20 {
					t.Errorf("got completion tokens %d, expected 20", chunk.Usage.CompletionTokens)
				}
			}
		})
	}
}

func TestGetNestedValue(t *testing.T) {
	data := map[string]interface{}{
		"simple": "value",
		"nested": map[string]interface{}{
			"key": "nested_value",
		},
		"array": []interface{}{
			map[string]interface{}{
				"item": "first",
			},
			map[string]interface{}{
				"item": "second",
			},
		},
		"choices": []interface{}{
			map[string]interface{}{
				"delta": map[string]interface{}{
					"content": "test_content",
				},
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
		found    bool
	}{
		{
			name:     "simple value",
			path:     "simple",
			expected: "value",
			found:    true,
		},
		{
			name:     "nested value",
			path:     "nested.key",
			expected: "nested_value",
			found:    true,
		},
		{
			name:     "array element",
			path:     "array.0.item",
			expected: "first",
			found:    true,
		},
		{
			name:     "complex nested",
			path:     "choices.0.delta.content",
			expected: "test_content",
			found:    true,
		},
		{
			name:     "non-existent",
			path:     "nonexistent",
			expected: nil,
			found:    false,
		},
		{
			name:     "invalid array index",
			path:     "array.10.item",
			expected: nil,
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found := getNestedValue(data, tt.path)

			if found != tt.found {
				t.Errorf("got found=%v, expected %v", found, tt.found)
			}

			if tt.found && value != tt.expected {
				t.Errorf("got value %v, expected %v", value, tt.expected)
			}
		})
	}
}

func TestIsArrayIndex(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"0", true},
		{"1", true},
		{"123", true},
		{"", false},
		{"abc", false},
		{"1a", false},
		{"-1", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isArrayIndex(tt.input)
			if result != tt.expected {
				t.Errorf("isArrayIndex(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseArrayIndex(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"123", 123},
		{"999", 999},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseArrayIndex(tt.input)
			if result != tt.expected {
				t.Errorf("parseArrayIndex(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertToolCalls(t *testing.T) {
	toolCallsArray := []interface{}{
		map[string]interface{}{
			"id":   "call_123",
			"type": "function",
			"function": map[string]interface{}{
				"name":      "get_weather",
				"arguments": `{"location": "NYC"}`,
			},
		},
	}

	toolCalls := convertToolCalls(toolCallsArray)

	if len(toolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].ID != "call_123" {
		t.Errorf("got ID %q, expected %q", toolCalls[0].ID, "call_123")
	}

	if toolCalls[0].Type != "function" {
		t.Errorf("got type %q, expected %q", toolCalls[0].Type, "function")
	}

	if toolCalls[0].Function.Name != "get_weather" {
		t.Errorf("got function name %q, expected %q", toolCalls[0].Function.Name, "get_weather")
	}
}

func TestAnthropicStreamParser(t *testing.T) {
	parser := NewAnthropicStreamParser()

	tests := []struct {
		name          string
		data          string
		expectError   bool
		expectDone    bool
		expectContent string
	}{
		{
			name:          "content block delta",
			data:          `{"type": "content_block_delta", "delta": {"text": "Hello"}}`,
			expectError:   false,
			expectDone:    false,
			expectContent: "Hello",
		},
		{
			name:        "message stop",
			data:        `{"type": "message_stop"}`,
			expectError: false,
			expectDone:  true,
		},
		{
			name:        "error event",
			data:        `{"type": "error", "error": {"message": "test error"}}`,
			expectError: true,
			expectDone:  true,
		},
		{
			name:        "invalid JSON",
			data:        `{invalid}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, isDone, err := parser.ParseLine(tt.data)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if isDone != tt.expectDone {
				t.Errorf("got isDone=%v, expected %v", isDone, tt.expectDone)
			}

			if tt.expectContent != "" && chunk.Content != tt.expectContent {
				t.Errorf("got content %q, expected %q", chunk.Content, tt.expectContent)
			}
		})
	}
}

func TestContextAwareStream(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "test"},
	}
	baseStream := NewMockStream(chunks)

	// Test with normal context
	ctx := context.Background()
	stream := StreamFromContext(ctx, baseStream)

	chunk, err := stream.Next()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if chunk.Content != "test" {
		t.Errorf("got content %q, expected %q", chunk.Content, "test")
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stream = StreamFromContext(ctx, baseStream)
	chunk, err = stream.Next()
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestContextAwareStream_Close(t *testing.T) {
	chunks := []types.ChatCompletionChunk{
		{Content: "test"},
	}
	baseStream := NewMockStream(chunks)

	ctx := context.Background()
	stream := StreamFromContext(ctx, baseStream)

	if err := stream.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateOpenAIStream(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"test\"}}]}\n\n"))
		flusher.Flush()
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	stream := CreateOpenAIStream(resp)
	if stream == nil {
		t.Fatal("expected non-nil stream")
	}

	// Clean up
	_ = stream.Close()
}

func TestCreateAnthropicStream(t *testing.T) {
	body := bytes.NewBufferString("data: {\"type\": \"content_block_delta\", \"delta\": {\"text\": \"test\"}}\n")
	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	stream := CreateAnthropicStream(resp)
	if stream == nil {
		t.Fatal("expected non-nil stream")
	}

	_ = stream.Close()
}

func TestErrorStream(t *testing.T) {
	testErr := io.ErrUnexpectedEOF
	stream := CreateErrorStream(testErr)

	chunk, err := stream.Next()
	if err != testErr {
		t.Errorf("expected error %v, got %v", testErr, err)
	}

	if !chunk.Done {
		t.Error("expected done chunk")
	}

	if err := stream.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

func TestBaseStream_Integration(t *testing.T) {
	// Create SSE-style response
	sseData := strings.Join([]string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\" World\"}}]}\n",
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n",
	}, "\n")

	body := bytes.NewBufferString(sseData)
	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	processor := NewStreamProcessor(resp)
	parser := NewStandardStreamParser()
	stream := NewBaseStream(processor, parser)

	// Read first chunk
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Content != "Hello" {
		t.Errorf("got %q, expected %q", chunk.Content, "Hello")
	}

	// Read second chunk
	chunk, err = stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunk.Content != " World" {
		t.Errorf("got %q, expected %q", chunk.Content, " World")
	}

	// Read final chunk
	chunk, err = stream.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
	if !chunk.Done {
		t.Error("expected done chunk")
	}

	_ = stream.Close()
}

func TestStreamProcessor_ThreadSafety(t *testing.T) {
	// Create a large data stream
	var data strings.Builder
	for i := 0; i < 100; i++ {
		data.WriteString("data: {\"content\": \"test\"}\n")
	}

	body := bytes.NewBufferString(data.String())
	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	processor := NewStreamProcessor(resp)

	// Try reading from multiple goroutines (should be safe with mutex)
	done := make(chan bool)
	processLine := func(line string) (types.ChatCompletionChunk, error, bool) {
		return types.ChatCompletionChunk{Content: "test"}, nil, false
	}

	go func() {
		for {
			_, err := processor.NextChunk(processLine)
			if err == io.EOF {
				break
			}
		}
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for stream to complete")
	}
}

func TestCreateCustomStream(t *testing.T) {
	body := bytes.NewBufferString("data: test\n")
	resp := &http.Response{
		Body: io.NopCloser(body),
	}

	parser := NewStandardStreamParser()
	stream := CreateCustomStream(resp, parser)

	if stream == nil {
		t.Fatal("expected non-nil stream")
	}

	_ = stream.Close()
}

func TestSSELineProcessor(t *testing.T) {
	data := `{"content": "test"}`

	responseParser := func(s string) (types.ChatCompletionChunk, bool) {
		return types.ChatCompletionChunk{Content: "test"}, false
	}

	chunk, err, isDone := SSELineProcessor(data, responseParser)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if isDone {
		t.Error("expected not done")
	}

	if chunk.Content != "test" {
		t.Errorf("got content %q, expected %q", chunk.Content, "test")
	}
}

func TestJSONLineProcessor(t *testing.T) {
	data := `{"text": "hello"}`

	var target map[string]interface{}
	chunkExtractor := func(i interface{}) types.ChatCompletionChunk {
		if m, ok := i.(*map[string]interface{}); ok {
			if text, ok := (*m)["text"].(string); ok {
				return types.ChatCompletionChunk{Content: text}
			}
		}
		return types.ChatCompletionChunk{}
	}

	chunk, err, _ := JSONLineProcessor(data, &target, chunkExtractor)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if chunk.Content != "hello" {
		t.Errorf("got content %q, expected %q", chunk.Content, "hello")
	}
}
