package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOllamaStream_NormalFlow tests the normal streaming flow with Ollama native endpoint
func TestOllamaStream_NormalFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Send streaming response chunks
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":" world"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"!"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":10,"eval_count":15}`,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Model: "llama3.1:8b",
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	chunkCount := 0
	var finalChunk types.ChatCompletionChunk

	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		content.WriteString(chunk.Content)
		chunkCount++
		finalChunk = chunk
	}

	// Verify results
	assert.Equal(t, "Hello world!", content.String())
	assert.Equal(t, 4, chunkCount)
	assert.True(t, finalChunk.Done)
	assert.Equal(t, 10, finalChunk.Usage.PromptTokens)
	assert.Equal(t, 15, finalChunk.Usage.CompletionTokens)
	assert.Equal(t, 25, finalChunk.Usage.TotalTokens)

	// Close stream
	err = stream.Close()
	assert.NoError(t, err)
}

// TestOllamaStream_WithToolCalls tests streaming with tool calls
func TestOllamaStream_WithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify tool calls are in the request
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.NotEmpty(t, req.Tools)

		// Send response with tool calls
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_123","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"San Francisco\"}"}}]},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":20,"eval_count":5}`,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Model: "llama3.1:8b",
		Tools: []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The location",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read chunks
	var chunks []types.ChatCompletionChunk
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		chunks = append(chunks, chunk)
	}

	// Verify tool calls
	require.NotEmpty(t, chunks)
	firstChunk := chunks[0]
	require.NotEmpty(t, firstChunk.Choices)
	require.NotEmpty(t, firstChunk.Choices[0].Delta.ToolCalls)

	toolCall := firstChunk.Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "call_123", toolCall.ID)
	assert.Equal(t, "function", toolCall.Type)
	assert.Equal(t, "get_weather", toolCall.Function.Name)
	assert.Contains(t, toolCall.Function.Arguments, "San Francisco")

	// Verify final chunk
	finalChunk := chunks[len(chunks)-1]
	assert.True(t, finalChunk.Done)
	assert.Equal(t, 20, finalChunk.Usage.PromptTokens)
	assert.Equal(t, 5, finalChunk.Usage.CompletionTokens)

	// Close stream
	err = stream.Close()
	assert.NoError(t, err)
}

// TestOllamaStream_EmptyChunks tests handling of empty chunks
func TestOllamaStream_EmptyChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send response with empty lines and chunks
		responses := []string{
			"",
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":false}`,
			"",
			"",
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":5,"eval_count":2}`,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hi"},
		},
		Model: "llama3.1:8b",
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		content.WriteString(chunk.Content)
	}

	// Should still get content despite empty lines
	assert.Equal(t, "Hi", content.String())

	err = stream.Close()
	assert.NoError(t, err)
}

// TestOllamaStream_ErrorHandling tests error handling during streaming
func TestOllamaStream_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Internal server error"}`))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Model: "llama3.1:8b",
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)

	// Should get an error for 500 status
	assert.Error(t, err)
	assert.Nil(t, stream)

	// Verify it's a server error
	providerErr, ok := err.(*types.ProviderError)
	assert.True(t, ok)
	assert.Equal(t, 500, providerErr.StatusCode)
}

// TestOllamaStream_ContextCancellation tests stream cancellation via context
func TestOllamaStream_ContextCancellation(t *testing.T) {
	// Test cancellation by using a timeout context
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send one chunk
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"llama3.1:8b","message":{"role":"assistant","content":"Start"},"done":false}` + "\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Simulate long processing - but we'll cancel before this completes
		time.Sleep(100 * time.Millisecond)

		// Try to send more chunks (but context will be cancelled)
		_, _ = w.Write([]byte(`{"model":"llama3.1:8b","message":{"role":"assistant","content":" end"},"done":true}` + "\n"))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)

	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Model: "llama3.1:8b",
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read first chunk
	chunk, err := stream.Next()
	require.NoError(t, err)
	assert.Equal(t, "Start", chunk.Content)

	// Wait for context to timeout
	time.Sleep(60 * time.Millisecond)

	// Next read should fail with context error
	_, err = stream.Next()
	assert.Error(t, err)
	// Should be either context.DeadlineExceeded or context.Canceled
	assert.True(t, err == context.DeadlineExceeded || err == context.Canceled)

	// Close stream
	err = stream.Close()
	assert.NoError(t, err)
}

// TestOllamaStream_Metadata tests metadata tracking
func TestOllamaStream_Metadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responses := []string{
			`{"model":"llama3.1:8b","message":{"role":"assistant","content":"Test"},"done":false}`,
			`{"model":"llama3.1:8b","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":8,"eval_count":12}`,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Test"},
		},
		Model: "llama3.1:8b",
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Cast to OllamaStream to access metadata
	ollamaStream, ok := stream.(*OllamaStream)
	require.True(t, ok)

	// Read all chunks
	for {
		_, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}

	// Check metadata
	metadata := ollamaStream.GetMetadata()
	assert.Equal(t, 8, metadata.PromptTokens)
	assert.Equal(t, 12, metadata.OutputTokens)
	assert.Equal(t, 20, metadata.TotalTokens)
	assert.Greater(t, metadata.Duration, time.Duration(0))

	err = stream.Close()
	assert.NoError(t, err)
}

// TestOllamaStream_OpenAIEndpoint tests OpenAI-compatible endpoint with SSE format
func TestOllamaStream_OpenAIEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the path for OpenAI endpoint
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)

		// Send SSE-formatted responses
		responses := []string{
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
			`data: [DONE]`,
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
		ProviderConfig: map[string]interface{}{
			"stream_endpoint": "openai",
		},
	}

	provider := NewOllamaProvider(config)
	assert.Equal(t, StreamEndpointOpenAI, provider.streamEndpoint)

	ctx := context.Background()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Model: "llama3.1:8b",
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	var finalChunk types.ChatCompletionChunk

	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		content.WriteString(chunk.Content)
		finalChunk = chunk
	}

	// Verify results
	assert.Equal(t, "Hello world", content.String())
	assert.True(t, finalChunk.Done)
	assert.Equal(t, 10, finalChunk.Usage.PromptTokens)
	assert.Equal(t, 5, finalChunk.Usage.CompletionTokens)
	assert.Equal(t, 15, finalChunk.Usage.TotalTokens)

	err = stream.Close()
	assert.NoError(t, err)
}

// TestOllamaStream_OpenAIEndpoint_WithToolCalls tests OpenAI endpoint with incremental tool calls
func TestOllamaStream_OpenAIEndpoint_WithToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send SSE-formatted responses with incremental tool calls
		responses := []string{
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":"}}]},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"SF\"}"}}]},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"llama3.1:8b","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}`,
			`data: [DONE]`,
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		for _, resp := range responses {
			_, _ = w.Write([]byte(resp + "\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
		ProviderConfig: map[string]interface{}{
			"stream_endpoint": "openai",
		},
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "What's the weather?"},
		},
		Model: "llama3.1:8b",
		Tools: []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]string{"type": "string"},
					},
				},
			},
		},
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Read all chunks - accumulate tool calls from all chunks
	var chunks []types.ChatCompletionChunk
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		chunks = append(chunks, chunk)
	}

	// Should have received multiple chunks
	require.NotEmpty(t, chunks)

	// Find a chunk with tool calls (should be accumulated in later chunks)
	var toolCallChunk *types.ChatCompletionChunk
	for i := range chunks {
		if len(chunks[i].Choices) > 0 && len(chunks[i].Choices[0].Delta.ToolCalls) > 0 {
			toolCallChunk = &chunks[i]
		}
	}

	// Verify we got tool calls
	require.NotNil(t, toolCallChunk, "Should have at least one chunk with tool calls")
	require.NotEmpty(t, toolCallChunk.Choices)
	require.NotEmpty(t, toolCallChunk.Choices[0].Delta.ToolCalls)

	toolCall := toolCallChunk.Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "call_abc", toolCall.ID)
	assert.Equal(t, "function", toolCall.Type)
	assert.Equal(t, "get_weather", toolCall.Function.Name)
	// Arguments should be accumulated from incremental chunks
	assert.Equal(t, `{"location":"SF"}`, toolCall.Function.Arguments)

	// Verify final chunk
	finalChunk := chunks[len(chunks)-1]
	assert.True(t, finalChunk.Done)

	// Verify usage
	assert.Equal(t, 20, finalChunk.Usage.PromptTokens)
	assert.Equal(t, 10, finalChunk.Usage.CompletionTokens)
	assert.Equal(t, 30, finalChunk.Usage.TotalTokens)

	err = stream.Close()
	assert.NoError(t, err)
}

// TestOllamaStream_EndpointConfiguration tests switching between endpoints
func TestOllamaStream_EndpointConfiguration(t *testing.T) {
	tests := []struct {
		name             string
		streamEndpoint   string
		expectedEndpoint StreamEndpoint
	}{
		{
			name:             "Default Ollama endpoint",
			streamEndpoint:   "",
			expectedEndpoint: StreamEndpointOllama,
		},
		{
			name:             "Explicit Ollama endpoint",
			streamEndpoint:   "ollama",
			expectedEndpoint: StreamEndpointOllama,
		},
		{
			name:             "OpenAI endpoint",
			streamEndpoint:   "openai",
			expectedEndpoint: StreamEndpointOpenAI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := types.ProviderConfig{
				Type:    types.ProviderTypeOllama,
				Name:    "ollama-test",
				BaseURL: "http://localhost:11434",
			}

			if tt.streamEndpoint != "" {
				config.ProviderConfig = map[string]interface{}{
					"stream_endpoint": tt.streamEndpoint,
				}
			}

			provider := NewOllamaProvider(config)
			assert.Equal(t, tt.expectedEndpoint, provider.streamEndpoint)
		})
	}
}

// TestOllamaStream_CloseIdempotent tests that Close() can be called multiple times safely
func TestOllamaStream_CloseIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"llama3.1:8b","message":{"role":"assistant","content":"Hi"},"done":true}` + "\n"))
	}))
	defer server.Close()

	config := types.ProviderConfig{
		Type:    types.ProviderTypeOllama,
		Name:    "ollama-test",
		BaseURL: server.URL,
	}

	provider := NewOllamaProvider(config)
	ctx := context.Background()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hi"},
		},
		Model: "llama3.1:8b",
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	require.NoError(t, err)

	// Read the chunk
	_, err = stream.Next()
	require.NoError(t, err)

	// Close multiple times - should not panic or error
	err = stream.Close()
	assert.NoError(t, err)

	err = stream.Close()
	assert.NoError(t, err)

	err = stream.Close()
	assert.NoError(t, err)
}
