package types

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCoreRequestBuilderComprehensive tests all builder methods
func TestCoreRequestBuilderComprehensive(t *testing.T) {
	t.Run("WithStop", func(t *testing.T) {
		request, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithStop([]string{"END", "STOP"}).
			Build()

		require.NoError(t, err)
		assert.Equal(t, []string{"END", "STOP"}, request.Stop)
	})

	t.Run("WithStreaming", func(t *testing.T) {
		request, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithStreaming(true).
			Build()

		require.NoError(t, err)
		assert.True(t, request.Stream)

		request, err = NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithStreaming(false).
			Build()

		require.NoError(t, err)
		assert.False(t, request.Stream)
	})

	t.Run("WithTools", func(t *testing.T) {
		tools := []Tool{
			{Name: "search", Description: "Search tool"},
			{Name: "calculate", Description: "Calculator"},
		}

		request, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithTools(tools).
			Build()

		require.NoError(t, err)
		assert.Equal(t, tools, request.Tools)
	})

	t.Run("WithResponseFormat", func(t *testing.T) {
		request, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithResponseFormat("json").
			Build()

		require.NoError(t, err)
		assert.Equal(t, "json", request.ResponseFormat)
	})

	t.Run("WithContext", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "test", "value")

		request, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithContext(ctx).
			Build()

		require.NoError(t, err)
		assert.NotNil(t, request.Context)
		assert.Equal(t, "value", request.Context.Value("test"))
	})

	t.Run("WithTimeout", func(t *testing.T) {
		timeout := 30 * time.Second

		request, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithTimeout(timeout).
			Build()

		require.NoError(t, err)
		assert.Equal(t, timeout, request.Timeout)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		request, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithMetadata("key1", "value1").
			WithMetadata("key2", 123).
			Build()

		require.NoError(t, err)
		assert.Equal(t, "value1", request.Metadata["key1"])
		assert.Equal(t, 123, request.Metadata["key2"])
	})

	t.Run("FromGenerateOptions", func(t *testing.T) {
		ctx := context.Background()
		language := "go"
		options := GenerateOptions{
			Messages:       []ChatMessage{{Role: "user", Content: "Test"}},
			Model:          "gpt-4",
			MaxTokens:      1000,
			Temperature:    0.8,
			Stop:           []string{"END"},
			Stream:         true,
			Tools:          []Tool{{Name: "test"}},
			ToolChoice:     &ToolChoice{Mode: ToolChoiceAuto},
			ResponseFormat: "json",
			ContextObj:     ctx,
			Timeout:        60 * time.Second,
			Metadata:       map[string]interface{}{"test": "value"},
			Language:       &language,
		}

		request, err := NewCoreRequestBuilder().
			FromGenerateOptions(options).
			Build()

		require.NoError(t, err)
		assert.Equal(t, options.Model, request.Model)
		assert.Equal(t, options.MaxTokens, request.MaxTokens)
		assert.Equal(t, options.Temperature, request.Temperature)
		assert.Equal(t, options.Stop, request.Stop)
		assert.Equal(t, options.Stream, request.Stream)
		assert.Equal(t, options.ResponseFormat, request.ResponseFormat)
		assert.Equal(t, ctx, request.Context)
		assert.Equal(t, options.Timeout, request.Timeout)
		assert.Equal(t, "value", request.Metadata["test"])
	})

	t.Run("EdgeCaseTemperature", func(t *testing.T) {
		// Zero temperature
		request, err := NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithTemperature(0).
			Build()
		require.NoError(t, err)
		assert.Equal(t, 0.0, request.Temperature)

		// Max valid temperature
		request, err = NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithTemperature(2.0).
			Build()
		require.NoError(t, err)
		assert.Equal(t, 2.0, request.Temperature)

		// Negative temperature
		_, err = NewCoreRequestBuilder().
			WithMessages([]ChatMessage{{Role: "user", Content: "Hello"}}).
			WithTemperature(-0.1).
			Build()
		require.Error(t, err)
		assert.True(t, IsValidationError(err))
	})
}

// TestStandardRequestSerialization tests JSON serialization
func TestStandardRequestSerialization(t *testing.T) {
	t.Run("FullRequestSerialization", func(t *testing.T) {
		request := StandardRequest{
			Messages:       []ChatMessage{{Role: "user", Content: "Hello"}},
			Model:          "gpt-4",
			MaxTokens:      100,
			Temperature:    0.7,
			Stop:           []string{"END"},
			Stream:         true,
			Tools:          []Tool{{Name: "test", Description: "Test tool"}},
			ToolChoice:     &ToolChoice{Mode: ToolChoiceAuto},
			ResponseFormat: "json",
			Metadata:       map[string]interface{}{"key": "value"},
		}

		data, err := json.Marshal(request)
		require.NoError(t, err)

		var result StandardRequest
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, request.Model, result.Model)
		assert.Equal(t, request.MaxTokens, result.MaxTokens)
		assert.Equal(t, request.Temperature, result.Temperature)
		assert.Equal(t, request.Stream, result.Stream)
		assert.Equal(t, request.ResponseFormat, result.ResponseFormat)
	})
}

// TestStandardResponseSerialization tests response types
func TestStandardResponseSerialization(t *testing.T) {
	t.Run("StandardResponse", func(t *testing.T) {
		response := StandardResponse{
			ID:      "resp-123",
			Model:   "gpt-4",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Choices: []StandardChoice{
				{
					Index:        0,
					Message:      ChatMessage{Role: "assistant", Content: "Hello!"},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
			ProviderMetadata: map[string]interface{}{"provider": "openai"},
		}

		data, err := json.Marshal(response)
		require.NoError(t, err)

		var result StandardResponse
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, response.ID, result.ID)
		assert.Equal(t, response.Model, result.Model)
		assert.Equal(t, response.Object, result.Object)
		assert.Equal(t, response.Created, result.Created)
		assert.Len(t, result.Choices, 1)
		assert.Equal(t, response.Usage, result.Usage)
	})

	t.Run("StandardStreamChunk", func(t *testing.T) {
		usage := Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8}
		chunk := StandardStreamChunk{
			ID:      "chunk-123",
			Model:   "gpt-4",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []StandardStreamChoice{
				{
					Index:        0,
					Delta:        ChatMessage{Content: "Hello"},
					FinishReason: "",
				},
			},
			Usage:            &usage,
			Done:             false,
			ProviderMetadata: map[string]interface{}{"provider": "openai"},
		}

		data, err := json.Marshal(chunk)
		require.NoError(t, err)

		var result StandardStreamChunk
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, chunk.ID, result.ID)
		assert.Equal(t, chunk.Model, result.Model)
		assert.Equal(t, chunk.Done, result.Done)
		assert.NotNil(t, result.Usage)
		assert.Equal(t, usage, *result.Usage)
	})
}

// TestToolChoiceConstants tests tool choice modes
func TestToolChoiceConstants(t *testing.T) {
	tests := []struct {
		name     string
		mode     ToolChoiceMode
		expected string
	}{
		{"Auto", ToolChoiceAuto, "auto"},
		{"Required", ToolChoiceRequired, "required"},
		{"None", ToolChoiceNone, "none"},
		{"Specific", ToolChoiceSpecific, "specific"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.mode))

			// Test JSON marshaling
			data, err := json.Marshal(tt.mode)
			require.NoError(t, err)
			assert.Equal(t, `"`+tt.expected+`"`, string(data))

			// Test JSON unmarshaling
			var result ToolChoiceMode
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)
			assert.Equal(t, tt.mode, result)
		})
	}
}

// TestToolChoiceSerialization tests tool choice struct
func TestToolChoiceSerialization(t *testing.T) {
	t.Run("AutoMode", func(t *testing.T) {
		tc := ToolChoice{Mode: ToolChoiceAuto}
		data, err := json.Marshal(tc)
		require.NoError(t, err)

		var result ToolChoice
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, ToolChoiceAuto, result.Mode)
		assert.Empty(t, result.FunctionName)
	})

	t.Run("SpecificMode", func(t *testing.T) {
		tc := ToolChoice{Mode: ToolChoiceSpecific, FunctionName: "get_weather"}
		data, err := json.Marshal(tc)
		require.NoError(t, err)

		var result ToolChoice
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)
		assert.Equal(t, ToolChoiceSpecific, result.Mode)
		assert.Equal(t, "get_weather", result.FunctionName)
	})
}

// TestExtensionRegistryConcurrency tests thread safety
func TestExtensionRegistryConcurrency(t *testing.T) {
	registry := NewExtensionRegistry()
	ext1 := &mockExtension{
		BaseExtension: NewBaseExtension("ext1", "1.0.0", "Extension 1", []string{"test"}),
	}
	ext2 := &mockExtension{
		BaseExtension: NewBaseExtension("ext2", "1.0.0", "Extension 2", []string{"test"}),
	}

	// Concurrent registration
	done := make(chan bool, 2)
	go func() {
		_ = registry.Register(ProviderTypeOpenAI, ext1)
		done <- true
	}()
	go func() {
		_ = registry.Register(ProviderTypeAnthropic, ext2)
		done <- true
	}()

	<-done
	<-done

	// Verify both registered
	assert.True(t, registry.Has(ProviderTypeOpenAI))
	assert.True(t, registry.Has(ProviderTypeAnthropic))

	// Concurrent reads
	done2 := make(chan bool, 2)
	go func() {
		_, _ = registry.Get(ProviderTypeOpenAI)
		done2 <- true
	}()
	go func() {
		_ = registry.List()
		done2 <- true
	}()

	<-done2
	<-done2
}

// TestBaseExtensionMethods tests base extension methods
func TestBaseExtensionMethods(t *testing.T) {
	ext := NewBaseExtension("test-ext", "2.0.0", "Test extension", []string{"chat", "streaming"})

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, "test-ext", ext.Name())
	})

	t.Run("Version", func(t *testing.T) {
		assert.Equal(t, "2.0.0", ext.Version())
	})

	t.Run("Description", func(t *testing.T) {
		assert.Equal(t, "Test extension", ext.Description())
	})

	t.Run("GetCapabilities", func(t *testing.T) {
		caps := ext.GetCapabilities()
		assert.Equal(t, []string{"chat", "streaming"}, caps)

		// Verify it's a copy
		caps[0] = "modified"
		assert.Equal(t, "chat", ext.GetCapabilities()[0])
	})

	t.Run("ValidateOptions", func(t *testing.T) {
		// Base implementation returns nil
		err := ext.ValidateOptions(map[string]interface{}{"test": "value"})
		assert.NoError(t, err)
	})
}

// TestCoreAPIFunctions tests core API functions
func TestCoreAPIFunctions(t *testing.T) {
	api := NewCoreAPI()
	ext := &mockExtension{
		BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
	}

	t.Run("RegisterAndGet", func(t *testing.T) {
		err := api.RegisterExtension(ProviderTypeOpenAI, ext)
		require.NoError(t, err)

		retrieved, err := api.GetExtension(ProviderTypeOpenAI)
		require.NoError(t, err)
		assert.Equal(t, ext.Name(), retrieved.Name())
	})

	t.Run("HasExtension", func(t *testing.T) {
		assert.True(t, api.HasExtension(ProviderTypeOpenAI))
		assert.False(t, api.HasExtension(ProviderTypeGemini))
	})

	t.Run("ListExtensions", func(t *testing.T) {
		extensions := api.ListExtensions()
		assert.Contains(t, extensions, ProviderTypeOpenAI)
	})

	t.Run("ConvertToProviderNoExtension", func(t *testing.T) {
		request := StandardRequest{
			Messages: []ChatMessage{{Role: "user", Content: "Test"}},
			Model:    "test-model",
		}

		result, err := api.ConvertToProvider(ProviderTypeGemini, request)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("ConvertFromProviderNoExtension", func(t *testing.T) {
		response := &StandardResponse{
			ID:    "test-id",
			Model: "test-model",
		}

		result, err := api.ConvertFromProvider(ProviderTypeGemini, response)
		require.NoError(t, err)
		assert.Equal(t, response, result)
	})

	t.Run("ConvertFromProviderError", func(t *testing.T) {
		_, err := api.ConvertFromProvider(ProviderTypeGemini, "invalid")
		require.Error(t, err)
	})

	t.Run("ConvertChunkFromProviderNoExtension", func(t *testing.T) {
		chunk := &StandardStreamChunk{
			ID:    "chunk-id",
			Model: "test-model",
		}

		result, err := api.ConvertChunkFromProvider(ProviderTypeGemini, chunk)
		require.NoError(t, err)
		assert.Equal(t, chunk, result)
	})

	t.Run("ConvertChunkFromProviderError", func(t *testing.T) {
		_, err := api.ConvertChunkFromProvider(ProviderTypeGemini, "invalid")
		require.Error(t, err)
	})

	t.Run("ValidateProviderOptionsNoExtension", func(t *testing.T) {
		err := api.ValidateProviderOptions(ProviderTypeGemini, map[string]interface{}{})
		assert.NoError(t, err)
	})

	t.Run("GetProviderCapabilitiesNoExtension", func(t *testing.T) {
		caps := api.GetProviderCapabilities(ProviderTypeGemini)
		assert.Equal(t, []string{"chat", "streaming"}, caps)
	})
}

// TestDefaultCoreAPIFunctions tests global default functions
func TestDefaultCoreAPIFunctions(t *testing.T) {
	t.Run("GetDefaultCoreAPI", func(t *testing.T) {
		api := GetDefaultCoreAPI()
		assert.NotNil(t, api)
	})

	t.Run("RegisterDefaultExtension", func(t *testing.T) {
		ext := &mockExtension{
			BaseExtension: NewBaseExtension("default-test", "1.0.0", "Test", []string{"test"}),
		}

		err := RegisterDefaultExtension(ProviderTypeCerebras, ext)
		require.NoError(t, err)

		// Verify it was registered
		assert.True(t, HasDefaultExtension(ProviderTypeCerebras))
	})

	t.Run("GetDefaultExtension", func(t *testing.T) {
		ext, err := GetDefaultExtension(ProviderTypeCerebras)
		require.NoError(t, err)
		assert.Equal(t, "default-test", ext.Name())
	})

	t.Run("GetDefaultExtensionNotFound", func(t *testing.T) {
		_, err := GetDefaultExtension(ProviderTypeFireworks)
		require.Error(t, err)
	})
}

// TestValidationErrorEdgeCases tests validation error handling
func TestValidationErrorEdgeCases(t *testing.T) {
	t.Run("ErrorMessage", func(t *testing.T) {
		err := NewValidationError("custom validation error")
		assert.Equal(t, "custom validation error", err.Error())
	})

	t.Run("IsValidationError", func(t *testing.T) {
		err := NewValidationError("test")
		assert.True(t, IsValidationError(err))

		otherErr := assert.AnError
		assert.False(t, IsValidationError(otherErr))

		assert.False(t, IsValidationError(nil))
	})

	t.Run("PredefinedErrors", func(t *testing.T) {
		assert.Equal(t, "at least one message is required", ErrNoMessages.Error())
		assert.Equal(t, "temperature must be between 0 and 2", ErrInvalidTemperature.Error())
		assert.Equal(t, "max_tokens must be non-negative", ErrInvalidMaxTokens.Error())
		assert.Equal(t, "tool_choice specified but no tools provided", ErrToolChoiceWithoutTools.Error())
	})
}

// BenchmarkCoreRequestBuilder benchmarks request building
func BenchmarkCoreRequestBuilder(b *testing.B) {
	messages := []ChatMessage{{Role: "user", Content: "Hello"}}
	tools := []Tool{{Name: "test", Description: "Test tool"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewCoreRequestBuilder().
			WithMessages(messages).
			WithModel("gpt-4").
			WithMaxTokens(100).
			WithTemperature(0.7).
			WithTools(tools).
			WithStreaming(true).
			Build()
	}
}

// BenchmarkStandardRequestMarshaling benchmarks JSON marshaling
func BenchmarkStandardRequestMarshaling(b *testing.B) {
	request := StandardRequest{
		Messages:    []ChatMessage{{Role: "user", Content: "Test"}},
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      true,
		Tools:       []Tool{{Name: "test"}},
		Metadata:    map[string]interface{}{"key": "value"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(request)
	}
}

// BenchmarkExtensionRegistryGet benchmarks registry lookup
func BenchmarkExtensionRegistryGet(b *testing.B) {
	registry := NewExtensionRegistry()
	ext := &mockExtension{
		BaseExtension: NewBaseExtension("bench", "1.0.0", "Benchmark", []string{"test"}),
	}
	_ = registry.Register(ProviderTypeOpenAI, ext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = registry.Get(ProviderTypeOpenAI)
	}
}
