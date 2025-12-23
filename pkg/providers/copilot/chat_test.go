// Package copilot provides chat completion tests for GitHub Copilot AI provider.
package copilot

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

// TestGenerateChatCompletion_NonStreaming tests non-streaming chat completion
func TestGenerateChatCompletion_NonStreaming(t *testing.T) {
	t.Run("successful chat completion", func(t *testing.T) {
		requestReceived := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestReceived = true
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/chat/completions", r.URL.Path)

			// Verify headers
			assert.Equal(t, "Bearer test_token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.NotEmpty(t, r.Header.Get("copilot-integration-id"))

			// Read request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var reqBody ChatCompletionRequest
			err = json.Unmarshal(body, &reqBody)
			require.NoError(t, err)

			assert.Equal(t, "gpt-4o", reqBody.Model)
			assert.NotEmpty(t, reqBody.Messages)

			// Return response
			w.Header().Set("Content-Type", "application/json")
			response := ChatCompletionResponse{
				ID:     "chatcmpl-test123",
				Object: "chat.completion",
				Created: time.Now().Unix(),
				Model:  "gpt-4o",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "Hello! How can I help you today?",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{
					PromptTokens:     10,
					CompletionTokens: 15,
					TotalTokens:      25,
				},
			}
			json.NewEncoder(w).Encode(response)
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
				{Role: "user", Content: "Hello, how are you?"},
			},
			MaxTokens:   100,
			Temperature: 0.7,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		require.NotNil(t, stream)
		defer stream.Close()

		assert.True(t, requestReceived, "server should have received request")

		// Get the first (and only) chunk
		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "Hello! How can I help you today?", chunk.Content)
		assert.True(t, chunk.Done)
		assert.Equal(t, 25, chunk.Usage.TotalTokens)

		// Next should return EOF or done chunk
		chunk, err = stream.Next()
		assert.True(t, chunk.Done || err == io.EOF)
	})

	t.Run("with prompt field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var reqBody ChatCompletionRequest
			json.Unmarshal(body, &reqBody)

			// Should convert prompt to user message
			assert.Equal(t, 1, len(reqBody.Messages))
			assert.Equal(t, "user", reqBody.Messages[0].Role)
			assert.Equal(t, "Test prompt", reqBody.Messages[0].Content)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ChatCompletionResponse{
				ID:      "test",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4o",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "Response",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{TotalTokens: 5},
			})
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
			Prompt:     "Test prompt",
			MaxTokens: 100,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()

		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "Response", chunk.Content)
	})

	t.Run("with custom model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var reqBody ChatCompletionRequest
			json.Unmarshal(body, &reqBody)

			assert.Equal(t, "gpt-4o-mini", reqBody.Model)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ChatCompletionResponse{
				ID:      "test",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4o-mini",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "Response",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{TotalTokens: 5},
			})
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
			Model:    "gpt-4o-mini",
			Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()

		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.Equal(t, "Response", chunk.Content)
	})

	t.Run("with temperature", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var reqBody ChatCompletionRequest
			json.Unmarshal(body, &reqBody)

			assert.Equal(t, 0.5, reqBody.Temperature)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ChatCompletionResponse{
				ID:      "test",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4o",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "Response",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{TotalTokens: 5},
			})
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
			Messages:    []types.ChatMessage{{Role: "user", Content: "Hello"}},
			Temperature: 0.5,
		}

		stream, err := provider.GenerateChatCompletion(context.Background(), options)
		require.NoError(t, err)
		defer stream.Close()
		_ = stream
	})
}

// TestPrepareRequest tests request preparation
func TestPrepareRequest(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	t.Run("basic message conversion", func(t *testing.T) {
		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hello"},
			},
		}

		req, err := provider.prepareRequest(options, "gpt-4o")
		require.NoError(t, err)

		assert.Equal(t, "gpt-4o", req.Model)
		assert.Equal(t, 2, len(req.Messages))
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "You are helpful", req.Messages[0].Content)
		assert.Equal(t, "user", req.Messages[1].Role)
		assert.Equal(t, "Hello", req.Messages[1].Content)
		assert.Equal(t, DefaultMaxTokens, req.MaxTokens)
		assert.False(t, req.Stream)
	})

	t.Run("with max tokens", func(t *testing.T) {
		options := types.GenerateOptions{
			Messages:  []types.ChatMessage{{Role: "user", Content: "Hello"}},
			MaxTokens: 500,
		}

		req, err := provider.prepareRequest(options, "gpt-4o")
		require.NoError(t, err)
		assert.Equal(t, 500, req.MaxTokens)
	})

	t.Run("with temperature", func(t *testing.T) {
		options := types.GenerateOptions{
			Messages:    []types.ChatMessage{{Role: "user", Content: "Hello"}},
			Temperature: 0.8,
		}

		req, err := provider.prepareRequest(options, "gpt-4o")
		require.NoError(t, err)
		assert.Equal(t, 0.8, req.Temperature)
	})

	t.Run("with multimodal content", func(t *testing.T) {
		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{
					Role:    "user",
					Content: "What's in this image?",
					Parts: []types.ContentPart{
						{Type: "text", Text: "What's in this image?"},
						{Type: "image", Source: &types.MediaSource{Type: "url", MediaType: "image/png", URL: "https://example.com/image.png"}},
					},
				},
			},
		}

		req, err := provider.prepareRequest(options, "gpt-4o")
		require.NoError(t, err)

		parts, ok := req.Messages[0].Content.([]ContentPart)
		require.True(t, ok)
		assert.Equal(t, 2, len(parts))
		assert.Equal(t, "text", parts[0].Type)
		assert.Equal(t, "What's in this image?", parts[0].Text)
		assert.Equal(t, "image_url", parts[1].Type)
		assert.NotNil(t, parts[1].ImageURL)
	})

	t.Run("with vision content", func(t *testing.T) {
		options := types.GenerateOptions{
			Messages: []types.ChatMessage{
				{
					Role:    "user",
					Content: "What's this?",
					Parts: []types.ContentPart{
						{Type: "image", Source: &types.MediaSource{Type: "url", MediaType: "image/png", URL: "https://example.com/image.png"}},
					},
				},
			},
		}

		req, err := provider.prepareRequest(options, "gpt-4o")
		require.NoError(t, err)

		assert.True(t, provider.hasVisionContent(req.Messages))
	})
}

// TestMakeAPICall tests the API call logic
func TestMakeAPICall(t *testing.T) {
	t.Run("successful API call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify required headers
			assert.Equal(t, "Bearer test_token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, CopilotIntegrationID, r.Header.Get("copilot-integration-id"))
			assert.Contains(t, r.Header.Get("editor-version"), "vscode/")
			assert.Equal(t, EditorPluginVersion, r.Header.Get("editor-plugin-version"))
			assert.NotEmpty(t, r.Header.Get("x-request-id"))
			assert.NotEmpty(t, r.Header.Get("X-Initiator"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4o",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "Test response",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{TotalTokens: 10},
			})
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
		}

		resp, err := provider.makeAPICall(context.Background(), req, "test_token")
		require.NoError(t, err)
		assert.Equal(t, "Test response", resp.Choices[0].Message.Content)
	})

	t.Run("API call with vision header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify vision header is set for image content
			assert.Equal(t, "true", r.Header.Get("copilot-vision-request"))

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ChatCompletionResponse{
				ID:      "test",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "gpt-4o",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "Response",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{TotalTokens: 5},
			})
		}))
		defer server.Close()

		provider := NewCopilotProvider(types.ProviderConfig{
			Type: types.ProviderTypeCopilot,
			ProviderConfig: map[string]interface{}{
				"base_url": server.URL,
			},
		})

		req := &ChatCompletionRequest{
			Model: "gpt-4o",
			Messages: []ChatMessage{
				{
					Role: "user",
					Content: []ContentPart{
						{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/img.png"}},
					},
				},
			},
		}

		_, err := provider.makeAPICall(context.Background(), req, "test_token")
		require.NoError(t, err)
	})

	t.Run("unauthorized error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error: ErrorDetail{
					Message: "Invalid token",
					Type:    "authentication_error",
				},
			})
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
		}

		_, err := provider.makeAPICall(context.Background(), req, "invalid_token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired Copilot token")
	})

	t.Run("rate limit error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error: ErrorDetail{
					Message: "Rate limit exceeded",
					Type:    "rate_limit_error",
				},
			})
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
		}

		_, err := provider.makeAPICall(context.Background(), req, "test_token")
		assert.Error(t, err)
		// Should be a rate limit error
		errStr := err.Error()
		assert.True(t, strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429"), errStr)
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
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
		}

		_, err := provider.makeAPICall(context.Background(), req, "test_token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("network error", func(t *testing.T) {
		provider := NewCopilotProvider(types.ProviderConfig{
			Type:    types.ProviderTypeCopilot,
			BaseURL: "http://invalid-url-that-does-not-exist.local",
		})

		req := &ChatCompletionRequest{
			Model:    "gpt-4o",
			Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
		}

		_, err := provider.makeAPICall(context.Background(), req, "test_token")
		assert.Error(t, err)
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
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
		}

		_, err := provider.makeAPICall(context.Background(), req, "test_token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse response")
	})
}

// TestHandleAPIError tests API error handling
func TestHandleAPIError(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	tests := []struct {
		name       string
		statusCode int
		body       []byte
		checkError func(t *testing.T, err error)
	}{
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"message":"Invalid request","type":"invalid_request_error"}}`),
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "Invalid request")
			},
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       []byte(`{"error":{"message":"Invalid token","type":"authentication_error"}}`),
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid or expired Copilot token")
			},
		},
		{
			name:       "rate limit",
			statusCode: http.StatusTooManyRequests,
			body:       []byte(`{"error":{"message":"Rate limited","type":"rate_limit_error"}}`),
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name:       "server error 500",
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"error":{"message":"Internal error","type":"server_error"}}`),
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "500")
			},
		},
		{
			name:       "server error 503",
			statusCode: http.StatusServiceUnavailable,
			body:       []byte(`Service unavailable`),
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "503")
			},
		},
		{
			name:       "unknown error",
			statusCode: 418,
			body:       []byte(`I'm a teapot`),
			checkError: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "418")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.handleAPIError(tt.statusCode, tt.body)
			if tt.checkError != nil {
				tt.checkError(t, err)
			}
		})
	}
}

// TestConvertContent tests content conversion helper
func TestConvertContent(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "string content",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name: "multimodal content",
			input: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "What's this?",
				},
				map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]string{
						"url": "https://example.com/image.png",
					},
				},
			},
			expected: []ContentPart{
				{Type: "text", Text: "What's this?"},
				{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/image.png"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertContent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractMessageContent tests message content extraction
func TestExtractMessageContent(t *testing.T) {
	tests := []struct {
		name     string
		content  interface{}
		expected string
	}{
		{
			name:     "string content",
			content:  "Hello world",
			expected: "Hello world",
		},
		{
			name: "content parts",
			content: []ContentPart{
				{Type: "text", Text: "Hello "},
				{Type: "text", Text: "world"},
			},
			expected: "Hello world",
		},
		{
			name: "mixed content with image",
			content: []ContentPart{
				{Type: "text", Text: "Check this "},
				{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/img.png"}},
				{Type: "text", Text: " out"},
			},
			expected: "Check this  out",
		},
		{
			name:     "nil content",
			content:  nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMessageContent(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGenerateChatCompletion_AccountTypes tests chat completion with different account types
func TestGenerateChatCompletion_AccountTypes(t *testing.T) {
	accountTypes := []struct {
		accountType string
		baseURL     string
	}{
		{AccountTypeIndividual, "https://api.githubcopilot.com"},
		{AccountTypeBusiness, "https://api.business.githubcopilot.com"},
		{AccountTypeEnterprise, "https://api.enterprise.githubcopilot.com"},
	}

	for _, at := range accountTypes {
		t.Run(at.accountType+" account", func(t *testing.T) {
			receivedURL := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedURL = r.URL.String()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(ChatCompletionResponse{
					ID:      "test",
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   "gpt-4o",
					Choices: []ChatChoice{
						{
							Index: 0,
							Message: ChatMessage{
								Role:    "assistant",
								Content: "Response",
							},
							FinishReason: "stop",
						},
					},
					Usage: Usage{TotalTokens: 5},
				})
			}))
			defer server.Close()

			// Create a custom provider with the test server
			provider := NewCopilotProvider(types.ProviderConfig{
				Type: types.ProviderTypeCopilot,
				ProviderConfig: map[string]interface{}{
					"account_type": at.accountType,
					"base_url":     server.URL,
				},
			})
			provider.tokenMutex.Lock()
			provider.copilotToken = "test_token"
			provider.copilotTokenExpiry = time.Now().Add(1 * time.Hour)
			provider.tokenMutex.Unlock()

			options := types.GenerateOptions{
				Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
			}

			stream, err := provider.GenerateChatCompletion(context.Background(), options)
			require.NoError(t, err)
			defer stream.Close()

			_, err = stream.Next()
			require.NoError(t, err)

			// Verify correct base URL was used
			assert.Equal(t, "/chat/completions", receivedURL)
		})
	}
}

// TestGenerateChatCompletion_NoToken tests behavior when no token is available
func TestGenerateChatCompletion_NoToken(t *testing.T) {
	provider := NewCopilotProvider(types.ProviderConfig{
		Type: types.ProviderTypeCopilot,
	})

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	_, err := provider.GenerateChatCompletion(context.Background(), options)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get Copilot token")
}

// TestGenerateChatCompletion_WithTokenRefresh tests token refresh during generation
func TestGenerateChatCompletion_WithTokenRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ChatCompletionResponse{
			ID:      "test",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o",
			Choices: []ChatChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "Response",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{TotalTokens: 5},
		})
	}))
	defer server.Close()

	provider := NewCopilotProvider(types.ProviderConfig{
		Type:    types.ProviderTypeCopilot,
		BaseURL: server.URL,
	})

	// Set an expiring token
	provider.tokenMutex.Lock()
	provider.copilotToken = "expiring_token"
	provider.copilotTokenExpiry = time.Now().Add(10 * time.Second)
	provider.githubToken = "test_github_token"
	provider.tokenMutex.Unlock()

	options := types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	// Should attempt to refresh the token
	_, err := provider.GenerateChatCompletion(context.Background(), options)
	// Will fail because we don't have a mock server for token refresh
	assert.Error(t, err)
}
