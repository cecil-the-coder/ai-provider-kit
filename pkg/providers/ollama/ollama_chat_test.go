package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaProvider_GenerateChatCompletion_WithMockServer(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and verify request body
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "llama3.1:8b", req["model"])
		assert.True(t, req["stream"].(bool))

		// Send streaming response (newline-delimited JSON, not SSE)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Write streaming chunks
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":" there"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"!"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":5,"eval_count":10}`,
		}

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

	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	var finalChunk types.ChatCompletionChunk
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
		finalChunk = chunk
	}

	// Verify content
	assert.Equal(t, "Hello there!", content.String())

	// Verify final chunk has usage
	assert.True(t, finalChunk.Done)
	assert.Equal(t, 5, finalChunk.Usage.PromptTokens)
	assert.Equal(t, 10, finalChunk.Usage.CompletionTokens)
	assert.Equal(t, 15, finalChunk.Usage.TotalTokens)

	// Close stream
	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_GenerateChatCompletion_WithTools(t *testing.T) {
	// Create mock server that expects tools
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify tools are present
		assert.NotEmpty(t, req.Tools)
		assert.Equal(t, "get_weather", req.Tools[0].Function.Name)

		// Send response with tool call
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_123","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"SF\"}"}}]},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":10,"eval_count":5}`,
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
			{Role: "user", Content: "What's the weather in SF?"},
		},
		Model: "llama3.1:8b",
		Tools: []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]string{"type": "string"},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read chunks
	chunks := []types.ChatCompletionChunk{}
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		chunks = append(chunks, chunk)
	}

	// Verify tool calls
	assert.NotEmpty(t, chunks)
	assert.NotEmpty(t, chunks[0].Choices)
	assert.NotEmpty(t, chunks[0].Choices[0].Delta.ToolCalls)

	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_StructuredOutputs_BasicJSON(t *testing.T) {
	// Create mock server that expects format field
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify format is set to "json"
		assert.NotNil(t, req.Format)
		assert.Equal(t, "json", req.Format)

		// Send streaming response with JSON content
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"{\"name\":"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"John\","},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"age\":30}"},"done":false}`,
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
			{Role: "user", Content: "Return a person object with name and age"},
		},
		Model:          "llama3.1:8b",
		ResponseFormat: "json", // Basic JSON mode
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)

	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
	}

	// Verify content is valid JSON
	result := content.String()
	assert.Contains(t, result, "John")
	assert.Contains(t, result, "30")

	// Verify it's valid JSON
	var jsonObj map[string]interface{}
	err = json.Unmarshal([]byte(result), &jsonObj)
	assert.NoError(t, err)

	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_StructuredOutputs_JSONSchema(t *testing.T) {
	// Create mock server that expects format field with schema
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify format is a JSON schema object
		assert.NotNil(t, req.Format)
		formatMap, ok := req.Format.(map[string]interface{})
		assert.True(t, ok, "Format should be a map")
		assert.Equal(t, "object", formatMap["type"])
		assert.NotNil(t, formatMap["properties"])

		// Send streaming response with structured JSON
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"{\"name\":"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"Alice\","},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"email\":\"alice@example.com\"}"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":12,"eval_count":18}`,
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

	// Define a JSON schema for structured output
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"name", "email"},
	}

	// Convert schema to JSON string
	schemaJSON, err := json.Marshal(schema)
	require.NoError(t, err)

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Return a user object with name and email"},
		},
		Model:          "llama3.1:8b",
		ResponseFormat: string(schemaJSON), // JSON schema
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)

	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
	}

	// Verify content matches schema
	result := content.String()
	var user map[string]interface{}
	err = json.Unmarshal([]byte(result), &user)
	assert.NoError(t, err)
	assert.Equal(t, "Alice", user["name"])
	assert.Equal(t, "alice@example.com", user["email"])

	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_StructuredOutputs_ComplexSchema(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify format is a complex schema
		assert.NotNil(t, req.Format)
		_, ok := req.Format.(map[string]interface{})
		assert.True(t, ok)

		// Send streaming response with nested JSON
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"{\"user\":"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"{\"name\":\"Bob\",\"age\":25},"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"\"items\":[\"apple\",\"banana\"]}"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":15,"eval_count":20}`,
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

	// Define a complex nested JSON schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"user": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "number"},
				},
				"required": []string{"name", "age"},
			},
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"user", "items"},
	}

	schemaJSON, err := json.Marshal(schema)
	require.NoError(t, err)

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Return a complex object with user and items"},
		},
		Model:          "llama3.1:8b",
		ResponseFormat: string(schemaJSON),
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	// Read all chunks
	var content strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
	}

	// Verify nested structure
	result := content.String()
	var obj map[string]interface{}
	err = json.Unmarshal([]byte(result), &obj)
	assert.NoError(t, err)

	// Verify user object
	user, ok := obj["user"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Bob", user["name"])
	assert.Equal(t, float64(25), user["age"])

	// Verify items array
	items, ok := obj["items"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, items, 2)
	assert.Equal(t, "apple", items[0])
	assert.Equal(t, "banana", items[1])

	err = stream.Close()
	assert.NoError(t, err)
}

func TestOllamaProvider_StructuredOutputs_NoFormat(t *testing.T) {
	// Test that requests without ResponseFormat don't include format field
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify format is nil/not set
		assert.Nil(t, req.Format)

		// Send normal response
		responses := []string{
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"llama3.1:8b","created_at":"2024-01-01T00:00:00Z","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":5,"eval_count":10}`,
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
		// No ResponseFormat specified
	}

	stream, err := provider.GenerateChatCompletion(ctx, options)
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	var content strings.Builder
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		content.WriteString(chunk.Content)
	}

	assert.Equal(t, "Hello", content.String())

	err = stream.Close()
	assert.NoError(t, err)
}
