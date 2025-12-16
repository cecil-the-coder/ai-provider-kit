package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/require"
)

// sendSSEStreamingResponse sends a complete SSE streaming response with content and done chunks
func sendSSEStreamingResponse(w http.ResponseWriter, flusher http.Flusher, id, model, content string) {
	chunk := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{"index": 0, "delta": map[string]interface{}{"content": content}, "finish_reason": nil},
		},
	}
	data, _ := json.Marshal(chunk)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	doneChunk := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{"index": 0, "delta": map[string]interface{}{}, "finish_reason": "stop"},
		},
	}
	doneData, _ := json.Marshal(doneChunk)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", doneData)
	flusher.Flush()
	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// TestOpenAIProvider_GLMStreamingRetry tests the GLM "No response requested" streaming retry logic
func TestOpenAIProvider_GLMStreamingRetry(t *testing.T) {
	t.Run("StreamingRetryOnNoResponseRequested", func(t *testing.T) {
		requestCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			currentRequest := requestCount
			requestCount++

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("Expected http.Flusher")
			}

			if currentRequest == 0 {
				sendSSEStreamingResponse(w, flusher, "chatcmpl-test", "glm-4.6", "No response requested.")
			} else {
				sendSSEStreamingResponse(w, flusher, "chatcmpl-retry", "glm-4.6", "Here is the actual streaming response.")
			}
		}))
		defer server.Close()

		config := types.ProviderConfig{
			Type:         types.ProviderTypeOpenAI,
			APIKey:       "sk-test-key",
			BaseURL:      server.URL,
			DefaultModel: "glm-4.6",
		}
		provider := NewOpenAIProvider(config)

		stream, err := provider.GenerateChatCompletion(context.Background(), types.GenerateOptions{
			Model:  "glm-4.6",
			Stream: true,
			Messages: []types.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		})
		require.NoError(t, err)
		defer func() { _ = stream.Close() }()

		// Collect all content from stream
		var content string
		for {
			chunk, err := stream.Next()
			if err != nil {
				break
			}
			content += chunk.Content
			if chunk.Done {
				break
			}
		}

		require.Equal(t, "Here is the actual streaming response.", content)
		require.Equal(t, 2, requestCount, "Expected 2 requests (initial + retry)")
	})
}
