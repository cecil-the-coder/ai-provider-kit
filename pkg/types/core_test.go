package types

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestCoreRequestBuilder(t *testing.T) {
	t.Run("Valid request", func(t *testing.T) {
		request, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{
				{Role: "user", Content: "Hello"},
			}).
			WithModel("gpt-4").
			WithMaxTokens(100).
			WithTemperature(0.7).
			Build()

		if err != nil {
			t.Fatalf("Failed to build request: %v", err)
		}

		if request.Model != "gpt-4" {
			t.Errorf("Expected model 'gpt-4', got '%s'", request.Model)
		}

		if request.MaxTokens != 100 {
			t.Errorf("Expected max_tokens 100, got %d", request.MaxTokens)
		}

		if request.Temperature != 0.7 {
			t.Errorf("Expected temperature 0.7, got %f", request.Temperature)
		}

		if len(request.Messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(request.Messages))
		}
	})

	t.Run("No messages", func(t *testing.T) {
		_, err := NewCoreRequestBuilder().
			WithModel("gpt-4").
			Build()

		if err == nil {
			t.Error("Expected error for request with no messages")
		}

		if !IsValidationError(err) {
			t.Error("Expected validation error")
		}
	})

	t.Run("Invalid temperature", func(t *testing.T) {
		_, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{
				{Role: "user", Content: "Hello"},
			}).
			WithTemperature(3.0). // Invalid: > 2.0
			Build()

		if err == nil {
			t.Error("Expected error for invalid temperature")
		}

		if !IsValidationError(err) {
			t.Error("Expected validation error")
		}
	})

	t.Run("Invalid max tokens", func(t *testing.T) {
		_, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{
				{Role: "user", Content: "Hello"},
			}).
			WithMaxTokens(-1). // Invalid: negative
			Build()

		if err == nil {
			t.Error("Expected error for invalid max tokens")
		}

		if !IsValidationError(err) {
			t.Error("Expected validation error")
		}
	})

	t.Run("Tool choice without tools", func(t *testing.T) {
		_, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{
				{Role: "user", Content: "Hello"},
			}).
			WithToolChoice(&ToolChoice{
				Mode: ToolChoiceAuto,
			}).
			Build()

		if err == nil {
			t.Error("Expected error for tool choice without tools")
		}

		if !IsValidationError(err) {
			t.Error("Expected validation error")
		}
	})
}

func TestStandardRequestToGenerateOptions(t *testing.T) {
	originalRequest := StandardRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Model:          "gpt-4",
		MaxTokens:      100,
		Temperature:    0.7,
		Stop:           []string{"END"},
		Stream:         true,
		Tools:          []Tool{{Name: "test", Description: "Test tool"}},
		ToolChoice:     &ToolChoice{Mode: ToolChoiceAuto},
		ResponseFormat: "json",
		Context:        context.Background(),
		Timeout:        time.Second * 30,
		Metadata:       map[string]interface{}{"key": "value"},
	}

	options := originalRequest.ToGenerateOptions()

	if options.Model != originalRequest.Model {
		t.Errorf("Model mismatch: expected %s, got %s", originalRequest.Model, options.Model)
	}

	if options.MaxTokens != originalRequest.MaxTokens {
		t.Errorf("MaxTokens mismatch: expected %d, got %d", originalRequest.MaxTokens, options.MaxTokens)
	}

	if len(options.Messages) != len(originalRequest.Messages) {
		t.Errorf("Messages length mismatch: expected %d, got %d", len(originalRequest.Messages), len(options.Messages))
	}
}

func TestDefaultExtensionRegistry(t *testing.T) {
	registry := NewExtensionRegistry()

	// Test registration
	extension := &mockExtension{
		BaseExtension: NewBaseExtension("test", "1.0.0", "Test extension", []string{"test"}),
	}

	err := registry.Register(ProviderTypeOpenAI, extension)
	if err != nil {
		t.Fatalf("Failed to register extension: %v", err)
	}

	// Test retrieval
	retrieved, err := registry.Get(ProviderTypeOpenAI)
	if err != nil {
		t.Fatalf("Failed to get extension: %v", err)
	}

	if retrieved.Name() != "test" {
		t.Errorf("Expected extension name 'test', got '%s'", retrieved.Name())
	}

	// Test duplicate registration
	err = registry.Register(ProviderTypeOpenAI, extension)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}

	// Test non-existent extension
	_, err = registry.Get(ProviderTypeAnthropic)
	if err == nil {
		t.Error("Expected error for non-existent extension")
	}

	// Test Has method
	if !registry.Has(ProviderTypeOpenAI) {
		t.Error("Expected Has to return true for registered extension")
	}

	if registry.Has(ProviderTypeAnthropic) {
		t.Error("Expected Has to return false for non-existent extension")
	}

	// Test List method
	extensions := registry.List()
	if len(extensions) != 1 {
		t.Errorf("Expected 1 extension in list, got %d", len(extensions))
	}
}

func TestCoreAPI(t *testing.T) {
	api := NewCoreAPI()

	// Register test extension
	extension := &mockExtension{
		BaseExtension: NewBaseExtension("test", "1.0.0", "Test extension", []string{"chat"}),
	}

	err := api.RegisterExtension(ProviderTypeOpenAI, extension)
	if err != nil {
		t.Fatalf("Failed to register extension: %v", err)
	}

	// Test conversion
	request := StandardRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
		Model:    "gpt-4",
	}

	// Convert to provider format
	providerReq, err := api.ConvertToProvider(ProviderTypeOpenAI, request)
	if err != nil {
		t.Fatalf("Failed to convert to provider format: %v", err)
	}

	// Should return the request as-is for mock extension
	if providerReq == nil {
		t.Error("Expected provider request to be non-nil")
	}

	// Test capabilities
	capabilities := api.GetProviderCapabilities(ProviderTypeOpenAI)
	if len(capabilities) != 1 || capabilities[0] != "chat" {
		t.Errorf("Expected capabilities ['chat'], got %v", capabilities)
	}

	// Test validation
	err = api.ValidateProviderOptions(ProviderTypeOpenAI, map[string]interface{}{})
	if err != nil {
		t.Errorf("Unexpected validation error: %v", err)
	}
}

func TestValidationError(t *testing.T) {
	err := NewValidationError("test error")
	if err.Error() != "test error" {
		t.Errorf("Expected error message 'test error', got '%s'", err.Error())
	}

	if !IsValidationError(err) {
		t.Error("Expected IsValidationError to return true")
	}

	otherErr := fmt.Errorf("other error")
	if IsValidationError(otherErr) {
		t.Error("Expected IsValidationError to return false for non-validation error")
	}
}

func TestStandardStreamAdapter(t *testing.T) {
	// Create mock chunks
	chunks := []ChatCompletionChunk{
		{
			Choices: []ChatChoice{
				{
					Delta: ChatMessage{Content: "Hello"},
				},
			},
			Done: false,
		},
		{
			Choices: []ChatChoice{
				{
					Delta: ChatMessage{Content: " world"},
				},
			},
			Done: true,
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}

	mockStream := &mockLegacyStream{chunks: chunks, index: 0}
	extension := &mockExtension{
		BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
	}

	adapter := &StandardStreamAdapter{
		providerStream: mockStream,
		extension:      extension,
	}

	// Test streaming
	var collectedContent string
	chunkCount := 0

	for {
		chunk, err := adapter.Next()
		if err != nil {
			break
		}

		if chunk != nil {
			collectedContent += chunk.Choices[0].Delta.Content
			chunkCount++

			if chunk.Done {
				break
			}
		}
	}

	if collectedContent != "Hello world" {
		t.Errorf("Expected content 'Hello world', got '%s'", collectedContent)
	}

	if chunkCount != 2 {
		t.Errorf("Expected 2 chunks, got %d", chunkCount)
	}

	// At this point, all chunks should be consumed and adapter should be done
	if !adapter.Done() {
		t.Error("Expected Done to be true after consuming all chunks")
	}

	// Test Close
	err := adapter.Close()
	if err != nil {
		t.Errorf("Unexpected error closing adapter: %v", err)
	}
}

func TestMockStandardStream(t *testing.T) {
	chunks := []StandardStreamChunk{
		{
			Choices: []StandardStreamChoice{
				{
					Delta: ChatMessage{Content: "Hello"},
				},
			},
			Done: false,
		},
		{
			Choices: []StandardStreamChoice{
				{
					Delta: ChatMessage{Content: " world"},
				},
			},
			Done: true,
		},
	}

	stream := NewMockStandardStream(chunks)

	// Test streaming
	var collectedContent string
	chunkCount := 0

	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}

		if chunk != nil {
			collectedContent += chunk.Choices[0].Delta.Content
			chunkCount++

			if chunk.Done {
				break
			}
		}
	}

	if collectedContent != "Hello world" {
		t.Errorf("Expected content 'Hello world', got '%s'", collectedContent)
	}

	if chunkCount != 2 {
		t.Errorf("Expected 2 chunks, got %d", chunkCount)
	}

	// Test Done after consuming
	if !stream.Done() {
		t.Error("Expected Done to be true after consuming all chunks")
	}

	// Test Close
	err := stream.Close()
	if err != nil {
		t.Errorf("Unexpected error closing stream: %v", err)
	}

	// Test reset after close
	if stream.Done() {
		t.Error("Expected Done to be false after close")
	}
}

// Mock extension for testing
type mockExtension struct {
	*BaseExtension
}

func (e *mockExtension) StandardToProvider(request StandardRequest) (interface{}, error) {
	// For testing, return the request as-is
	return request, nil
}

func (e *mockExtension) ProviderToStandard(response interface{}) (*StandardResponse, error) {
	// For testing, create a simple standard response
	return &StandardResponse{
		ID:      "test-id",
		Model:   "test-model",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Choices: []StandardChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: "Test response",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (e *mockExtension) ProviderToStandardChunk(chunk interface{}) (*StandardStreamChunk, error) {
	// For testing, create a simple standard chunk
	if chatChunk, ok := chunk.(*ChatCompletionChunk); ok {
		content := ""
		if len(chatChunk.Choices) > 0 {
			content = chatChunk.Choices[0].Delta.Content
		}

		return &StandardStreamChunk{
			ID:      "test-chunk-id",
			Model:   "test-model",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []StandardStreamChoice{
				{
					Index: 0,
					Delta: ChatMessage{
						Role:    "assistant",
						Content: content,
					},
					FinishReason: "",
				},
			},
			Done:  chatChunk.Done,
			Usage: &chatChunk.Usage,
		}, nil
	}

	return nil, fmt.Errorf("invalid chunk type")
}

// mockLegacyStream implements ChatCompletionStream for testing
type mockLegacyStream struct {
	chunks []ChatCompletionChunk
	index  int
}

func (m *mockLegacyStream) Next() (ChatCompletionChunk, error) {
	if m.index >= len(m.chunks) {
		return ChatCompletionChunk{}, fmt.Errorf("end of stream")
	}

	chunk := m.chunks[m.index]
	m.index++
	return chunk, nil
}

func (m *mockLegacyStream) Close() error {
	m.index = 0
	return nil
}
