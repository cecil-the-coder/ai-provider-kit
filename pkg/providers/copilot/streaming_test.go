// Package copilot provides streaming tests for GitHub Copilot AI provider.
package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// TestGenerateChatCompletion_Streaming tests streaming chat completion
func TestGenerateChatCompletion_Streaming(t *testing.T) {
	t.Run("successful streaming", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/chat/completions", r.URL.Path)

			// Read request body
			body, _ := io.ReadAll(r.Body)
			var reqBody ChatCompletionRequest
			json.Unmarshal(body, &reqBody)

			// Verify stream is enabled
			assert.True(t, reqBody.Stream)

			// Set SSE headers
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok, "server must support flushing")

			// Send multiple chunks
			chunks := []string{
				"Hello",
				" there",
				"!",
			}

			for _, content := range chunks {
				chunk := map[string]interface{}{
					"id":      "chatcmpl-test123",
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   "gpt-4o",
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"delta": map[string]interface{}{
								"content": content,
							},
							"finish_reason": nil,
						},
					},
				}
				data, _ := json.Marshal(chunk)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}

			// Send final chunk
			finalChunk := map[string]interface{}{
				"id":      "chatcmpl-test123",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": "stop",
					},
				},
			}
			finalData, _ := json.Marshal(finalChunk)
			fmt.Fprintf(w, "data: %s\n\n", finalData)
			flusher.Flush()
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Say hello"},
			},
			Stream: true,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()

		// Collect all chunks
		var fullContent string
		chunkCount := 0

		for {
			chunk, err := stream.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			fullContent += chunk.Content
			chunkCount++

			if chunk.Done {
				break
			}
		}

		assert.Equal(t, "Hello there!", fullContent)
		assert.GreaterOrEqual(t, chunkCount, 3)
	})

	t.Run("streaming with tool calls", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			// Send chunk with tool call start
			toolCallChunk := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"tool_calls": []map[string]interface{}{
								{
									"index": 0,
									"id":    "call_abc123",
									"type":  "function",
									"function": map[string]interface{}{
										"name":      "get_weather",
										"arguments": "",
									},
								},
							},
						},
						"finish_reason": nil,
					},
				},
			}
			data, _ := json.Marshal(toolCallChunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Send chunk with arguments
			argsChunk := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"tool_calls": []map[string]interface{}{
								{
									"index": 0,
									"function": map[string]interface{}{
										"arguments": `{"location":"NYC"}`,
									},
								},
							},
						},
						"finish_reason": nil,
					},
				},
			}
			argsData, _ := json.Marshal(argsChunk)
			fmt.Fprintf(w, "data: %s\n\n", argsData)
			flusher.Flush()

			// Send final chunk
			finalChunk := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": "tool_calls",
					},
				},
			}
			finalData, _ := json.Marshal(finalChunk)
			fmt.Fprintf(w, "data: %s\n\n", finalData)
			flusher.Flush()
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		tools := []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather",
				InputSchema: map[string]interface{}{
					"type": "object",
				},
			},
		}

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "user", Content: "What's the weather in NYC?"},
			},
			Tools:  tools,
			Stream: true,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()

		// Collect chunks
		chunks := []types.ChatCompletionChunk{}
		for {
			chunk, err := stream.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			chunks = append(chunks, chunk)
			if chunk.Done {
				break
			}
		}

		// Verify we got tool calls
		hasToolCalls := false
		for _, chunk := range chunks {
			if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
				hasToolCalls = true
				break
			}
		}
		assert.True(t, hasToolCalls, "should have received tool call chunks")
	})

	t.Run("streaming with usage info", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			// Send content chunk
			contentChunk := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": "Hello",
						},
						"finish_reason": nil,
					},
				},
			}
			data, _ := json.Marshal(contentChunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Send final chunk with usage
			finalChunk := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
				},
			}
			finalData, _ := json.Marshal(finalChunk)
			fmt.Fprintf(w, "data: %s\n\n", finalData)
			flusher.Flush()
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
			Stream:   true,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()

		// Find the chunk with usage info
		var usageChunk *types.ChatCompletionChunk
		for {
			chunk, err := stream.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			if chunk.Usage.TotalTokens > 0 {
				usageChunk = &chunk
			}

			if chunk.Done {
				break
			}
		}

		require.NotNil(t, usageChunk)
		assert.Equal(t, 10, usageChunk.Usage.PromptTokens)
		assert.Equal(t, 5, usageChunk.Usage.CompletionTokens)
		assert.Equal(t, 15, usageChunk.Usage.TotalTokens)
	})

	t.Run("streaming with vision request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify vision header is set
			assert.Equal(t, "true", r.Header.Get("copilot-vision-request"))

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			chunk := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": "I see an image",
						},
						"finish_reason": nil,
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			finalChunk := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": "stop",
					},
				},
			}
			finalData, _ := json.Marshal(finalChunk)
			fmt.Fprintf(w, "data: %s\n\n", finalData)
			flusher.Flush()
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{
					Role:    "user",
					Content: "What's in this image?",
					Parts: []types.ContentPart{
						{Type: "image", Source: &types.MediaSource{Type: "url", MediaType: "image/png", URL: "https://example.com/image.png"}},
					},
				},
			},
			Stream: true,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()

		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "I see an image", chunk.Content)
	})

	t.Run("streaming with error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "data: {\"error\":{\"message\":\"Invalid token\"}}\n\n")
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
			Stream:   true,
		}

		_, err := provider.GenerateChatCompletion(context.Background(), options)
		assert.Error(t, err)
	})

	t.Run("streaming with empty chunks", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			// Send some empty lines and comments (should be skipped)
			fmt.Fprintf(w, "\n\n")
			flusher.Flush()
			fmt.Fprintf(w, ": comment\n\n")
			flusher.Flush()

			// Send actual data
			chunk := map[string]interface{}{
				"id":      "chatcmpl-test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": "Response",
						},
						"finish_reason": nil,
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Send done marker
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
			Stream:   true,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()

		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "Response", chunk.Content)
	})
}

// TestCopilotStream tests the CopilotStream implementation
func TestCopilotStream(t *testing.T) {
	t.Run("stream reading and closing", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			chunk := map[string]interface{}{
				"id":      "test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": "Test",
						},
						"finish_reason": nil,
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		stream := NewCopilotStream(resp)

		// Read first chunk
		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "Test", chunk.Content)
		assert.False(t, chunk.Done)

		// Read done chunk
		chunk, err = stream.Next()
		require.NoError(t, err)
		assert.True(t, chunk.Done)

		// Close stream
		err = stream.Close()
		assert.NoError(t, err)

		// Reading after close returns EOF
		_, err = stream.Next()
		assert.Error(t, err)
	})

	t.Run("stream with parse error continues", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			// Send invalid JSON (should be skipped)
			fmt.Fprintf(w, "data: {invalid json}\n\n")
			flusher.Flush()

			// Send valid chunk
			chunk := map[string]interface{}{
				"id":      "test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": "Valid",
						},
						"finish_reason": nil,
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		stream := NewCopilotStream(resp)

		// Should skip invalid chunk and return valid one
		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "Valid", chunk.Content)

		stream.Close()
	})

	t.Run("stream handles scanner errors", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Close immediately to cause error
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		stream := NewCopilotStream(resp)

		// Should either return EOF or error
		chunk, err := stream.Next()
		if err == nil {
			// If no error, should be done
			assert.True(t, chunk.Done)
		} else {
			// Should be EOF or connection error
			assert.True(t, err == io.EOF || strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "closed"))
		}

		stream.Close()
	})
}

// TestMakeStreamingAPICall tests the streaming API call logic
func TestMakeStreamingAPICall(t *testing.T) {
	t.Run("successful streaming call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify headers
			assert.Equal(t, "Bearer test_token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, CopilotIntegrationID, r.Header.Get("copilot-integration-id"))
			assert.Contains(t, r.Header.Get("editor-version"), "vscode/")
			assert.NotEmpty(t, r.Header.Get("x-request-id"))
			assert.NotEmpty(t, r.Header.Get("X-Initiator"))

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			chunk := map[string]interface{}{
				"id":      "test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": "Streaming",
						},
						"finish_reason": nil,
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})

		req := &ChatCompletionRequest{
			Model:    "gpt-4o",
			Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
			Stream:   true,
		}

		stream, err := provider.makeStreamingAPICall(context.Background(), req, "test_token")
		require.NoError(t, err)
		defer stream.Close()

		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "Streaming", chunk.Content)
	})

	t.Run("streaming call with error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"message":"Invalid token"}}`))
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})

		req := &ChatCompletionRequest{
			Model:    "gpt-4o",
			Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
			Stream:   true,
		}

		_, err := provider.makeStreamingAPICall(context.Background(), req, "test_token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired Copilot token")
	})
}

// TestExecuteStreamWithAuth tests streaming with authentication
func TestExecuteStreamWithAuth(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			require.True(t, ok)

			chunk := map[string]interface{}{
				"id":      "test",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "gpt-4o",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": "Test",
						},
						"finish_reason": nil,
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})
		provider.tokenMutex.Lock()
		provider.copilotToken = "test_token"
		provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
		provider.tokenMutex.Unlock()

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
			Stream:   true,
		}

		stream, err := provider.executeStreamWithAuth(context.Background(), options, "gpt-4o")
		require.NoError(t, err)
		defer stream.Close()

		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "Test", chunk.Content)
	})

	t.Run("no token available", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
		})

		options := types.GenerateOptions{
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
			Stream:   true,
		}

		_, err := provider.executeStreamWithAuth(context.Background(), options, "gpt-4o")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get Copilot token")
	})
}

// TestNewStreamScanner tests the stream scanner creation
func TestNewStreamScanner(t *testing.T) {
	t.Run("scanner splits by newlines", func(t *testing.T) {
		resp := &http.Response{
			Body: io.NopCloser(strings.NewReader("line1\nline2\nline3\n")),
		}

		scanner := newStreamScanner(resp)

		lines := []string{}
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		assert.Equal(t, []string{"line1", "line2", "line3"}, lines)
	})
}

// TestGetFinishReason tests finish reason extraction
func TestGetFinishReason(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{
			name:     "stop reason",
			input:    stringPtr("stop"),
			expected: "stop",
		},
		{
			name:     "tool_calls reason",
			input:    stringPtr("tool_calls"),
			expected: "tool_calls",
		},
		{
			name:     "nil reason",
			input:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFinishReason(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}

// TestConvertToolCalls tests tool call conversion
func TestConvertToolCalls(t *testing.T) {
	inputCalls := []ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "get_weather",
				Arguments: `{"location":"NYC"}`,
			},
		},
		{
			ID:   "call_2",
			Type: "function",
			Function: ToolCallFunction{
				Name:      "get_time",
				Arguments: `{"timezone":"UTC"}`,
			},
		},
	}

	result := convertToolCalls(inputCalls)

	require.Equal(t, 2, len(result))
	assert.Equal(t, "call_1", result[0].ID)
	assert.Equal(t, "function", result[0].Type)
	assert.Equal(t, "get_weather", result[0].Function.Name)
	assert.Equal(t, `{"location":"NYC"}`, result[0].Function.Arguments)
	assert.Equal(t, "call_2", result[1].ID)
}

// TestStreamCloseMultipleTimes tests that stream can be closed multiple times safely
func TestStreamCloseMultipleTimes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"id\":\"test\",\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)
	stream := NewCopilotStream(resp)

	// Close multiple times - should not panic
	err := stream.Close()
	assert.NoError(t, err)

	err = stream.Close()
	assert.NoError(t, err)

	err = stream.Close()
	assert.NoError(t, err)
}
