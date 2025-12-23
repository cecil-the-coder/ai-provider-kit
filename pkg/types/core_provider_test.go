package types

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCoreProviderAdapter tests the CoreProviderAdapter
func TestCoreProviderAdapter(t *testing.T) {
	t.Run("NewCoreProviderAdapter", func(t *testing.T) {
		provider := newMockStreamProvider("test-provider", ProviderTypeOpenAI)
		extension := &mockExtension{
			BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
		}

		adapter := NewCoreProviderAdapter(provider, extension)
		assert.NotNil(t, adapter)
		assert.Equal(t, provider, adapter.provider)
		assert.Equal(t, extension, adapter.extension)
	})

	t.Run("GetCoreExtension", func(t *testing.T) {
		provider := newMockStreamProvider("test-provider", ProviderTypeOpenAI)
		extension := &mockExtension{
			BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
		}

		adapter := NewCoreProviderAdapter(provider, extension)
		retrieved := adapter.GetCoreExtension()
		assert.Equal(t, extension, retrieved)
	})

	t.Run("GetStandardCapabilities", func(t *testing.T) {
		provider := newMockStreamProvider("test-provider", ProviderTypeOpenAI)
		extension := &mockExtension{
			BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat", "streaming"}),
		}

		adapter := NewCoreProviderAdapter(provider, extension)
		caps := adapter.GetStandardCapabilities()
		assert.Equal(t, []string{"chat", "streaming"}, caps)
	})

	t.Run("ValidateStandardRequest", func(t *testing.T) {
		provider := newMockStreamProvider("test-provider", ProviderTypeOpenAI)
		extension := &mockExtension{
			BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
		}

		adapter := NewCoreProviderAdapter(provider, extension)

		// Valid request
		request := StandardRequest{
			Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
		}
		err := adapter.ValidateStandardRequest(request)
		assert.NoError(t, err)

		// Invalid request (no messages)
		request.Messages = nil
		err = adapter.ValidateStandardRequest(request)
		assert.Error(t, err)
		assert.Equal(t, ErrNoMessages, err)
	})

	t.Run("GenerateStandardCompletion", func(t *testing.T) {
		provider := newMockStreamProvider("test-provider", ProviderTypeOpenAI)
		extension := &mockExtension{
			BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
		}

		adapter := NewCoreProviderAdapter(provider, extension)
		ctx := context.Background()

		request := StandardRequest{
			Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
			Model:    "gpt-4",
		}

		response, err := adapter.GenerateStandardCompletion(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "test-id", response.ID)
	})

	t.Run("GenerateStandardStream", func(t *testing.T) {
		provider := newMockStreamProvider("test-provider", ProviderTypeOpenAI)
		extension := &mockExtension{
			BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
		}

		adapter := NewCoreProviderAdapter(provider, extension)
		ctx := context.Background()

		request := StandardRequest{
			Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
			Model:    "gpt-4",
		}

		stream, err := adapter.GenerateStandardStream(ctx, request)
		require.NoError(t, err)
		assert.NotNil(t, stream)

		// Read a chunk
		chunk, err := stream.Next()
		require.NoError(t, err)
		assert.NotNil(t, chunk)

		err = stream.Close()
		assert.NoError(t, err)
	})

	t.Run("convertToLegacyOptions", func(t *testing.T) {
		provider := newMockStreamProvider("test-provider", ProviderTypeOpenAI)
		extension := &mockExtension{
			BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
		}

		adapter := NewCoreProviderAdapter(provider, extension)
		ctx := context.Background()

		request := StandardRequest{
			Messages:       []ChatMessage{{Role: "user", Content: "Hello"}},
			Model:          "gpt-4",
			MaxTokens:      100,
			Temperature:    0.7,
			Stop:           []string{"END"},
			Stream:         true,
			Tools:          []Tool{{Name: "test"}},
			ToolChoice:     &ToolChoice{Mode: ToolChoiceAuto},
			ResponseFormat: "json",
			Context:        ctx,
			Metadata:       map[string]interface{}{"key": "value"},
		}

		options := adapter.convertToLegacyOptions(request)
		assert.Equal(t, request.Messages, options.Messages)
		assert.Equal(t, request.Model, options.Model)
		assert.Equal(t, request.MaxTokens, options.MaxTokens)
		assert.Equal(t, request.Temperature, options.Temperature)
		assert.Equal(t, request.Stop, options.Stop)
		assert.Equal(t, request.Stream, options.Stream)
		assert.Equal(t, request.Tools, options.Tools)
		assert.Equal(t, request.ToolChoice, options.ToolChoice)
		assert.Equal(t, request.ResponseFormat, options.ResponseFormat)
		assert.Equal(t, ctx, options.ContextObj)
		assert.Equal(t, request.Metadata, options.Metadata)
	})
}

// mockStreamProvider implements Provider with streaming support
// This is a local mock to avoid import cycle with testutil
type mockStreamProvider struct {
	name         string
	providerType ProviderType
}

func newMockStreamProvider(name string, providerType ProviderType) *mockStreamProvider {
	return &mockStreamProvider{
		name:         name,
		providerType: providerType,
	}
}

func (m *mockStreamProvider) Name() string                          { return m.name }
func (m *mockStreamProvider) Type() ProviderType                    { return m.providerType }
func (m *mockStreamProvider) Description() string                   { return "Mock provider for testing" }
func (m *mockStreamProvider) GetConfig() ProviderConfig             { return ProviderConfig{} }
func (m *mockStreamProvider) Configure(config ProviderConfig) error { return nil }
func (m *mockStreamProvider) GetModels(ctx context.Context) ([]Model, error) {
	return []Model{{ID: "test-model", Name: "Test Model"}}, nil
}
func (m *mockStreamProvider) GetDefaultModel() string { return "test-model" }
func (m *mockStreamProvider) Authenticate(ctx context.Context, authConfig AuthConfig) error {
	return nil
}
func (m *mockStreamProvider) IsAuthenticated() bool                 { return true }
func (m *mockStreamProvider) Logout(ctx context.Context) error      { return nil }
func (m *mockStreamProvider) HealthCheck(ctx context.Context) error { return nil }
func (m *mockStreamProvider) GetMetrics() ProviderMetrics           { return ProviderMetrics{} }
func (m *mockStreamProvider) SupportsToolCalling() bool             { return true }
func (m *mockStreamProvider) SupportsStreaming() bool               { return true }
func (m *mockStreamProvider) SupportsResponsesAPI() bool            { return false }
func (m *mockStreamProvider) GetToolFormat() ToolFormat             { return ToolFormatOpenAI }
func (m *mockStreamProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return "tool result", nil
}

func (m *mockStreamProvider) GenerateChatCompletion(ctx context.Context, options GenerateOptions) (ChatCompletionStream, error) {
	return &mockCompletionStream{
		chunks: []ChatCompletionChunk{
			{
				ID:      "chunk-1",
				Content: "Hello",
				Done:    false,
			},
			{
				ID:      "chunk-2",
				Content: " world",
				Done:    true,
				Usage: Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
		},
		index: 0,
	}, nil
}

type mockCompletionStream struct {
	chunks []ChatCompletionChunk
	index  int
}

func (m *mockCompletionStream) Next() (ChatCompletionChunk, error) {
	if m.index >= len(m.chunks) {
		return ChatCompletionChunk{}, fmt.Errorf("end of stream")
	}
	chunk := m.chunks[m.index]
	m.index++
	return chunk, nil
}

func (m *mockCompletionStream) Close() error {
	m.index = 0
	return nil
}

// TestDefaultProviderFactoryExtensions tests the provider factory extensions
func TestDefaultProviderFactoryExtensions(t *testing.T) {
	t.Run("NewDefaultProviderFactoryExtensions", func(t *testing.T) {
		factory := &mockProviderFactory{}
		extFactory := NewDefaultProviderFactoryExtensions(factory)
		assert.NotNil(t, extFactory)
		assert.Equal(t, factory, extFactory.ProviderFactory)
	})

	t.Run("CreateCoreProvider", func(t *testing.T) {
		// Register extension
		extension := &mockExtension{
			BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
		}
		err := RegisterDefaultExtension(ProviderTypeOpenAI, extension)
		require.NoError(t, err)

		factory := &mockProviderFactory{}
		extFactory := NewDefaultProviderFactoryExtensions(factory)

		config := ProviderConfig{
			Type: ProviderTypeOpenAI,
			Name: "test",
		}

		coreProvider, err := extFactory.CreateCoreProvider(ProviderTypeOpenAI, config)
		require.NoError(t, err)
		assert.NotNil(t, coreProvider)
	})

	t.Run("GetSupportedCoreProviders", func(t *testing.T) {
		factory := &mockProviderFactory{
			supportedTypes: []ProviderType{ProviderTypeOpenAI, ProviderTypeAnthropic, ProviderTypeGemini},
		}
		extFactory := NewDefaultProviderFactoryExtensions(factory)

		supported := extFactory.GetSupportedCoreProviders()
		// Should include OpenAI (has extension) but not Gemini (no extension)
		assert.Contains(t, supported, ProviderTypeOpenAI)
	})

	t.Run("SupportsCoreAPI", func(t *testing.T) {
		factory := &mockProviderFactory{}
		extFactory := NewDefaultProviderFactoryExtensions(factory)

		// OpenAI has extension
		assert.True(t, extFactory.SupportsCoreAPI(ProviderTypeOpenAI))

		// Gemini doesn't have extension
		assert.False(t, extFactory.SupportsCoreAPI(ProviderTypeGemini))
	})
}

// mockProviderFactory implements ProviderFactory for testing
type mockProviderFactory struct {
	supportedTypes []ProviderType
}

func (m *mockProviderFactory) RegisterProvider(providerType ProviderType, factoryFunc func(ProviderConfig) Provider) {
}

func (m *mockProviderFactory) CreateProvider(providerType ProviderType, config ProviderConfig) (Provider, error) {
	return newMockStreamProvider("mock-provider", providerType), nil
}

func (m *mockProviderFactory) GetSupportedProviders() []ProviderType {
	if len(m.supportedTypes) == 0 {
		return []ProviderType{ProviderTypeOpenAI, ProviderTypeAnthropic}
	}
	return m.supportedTypes
}

// TestStandardStreamAdapterEdgeCases tests stream adapter edge cases
func TestStandardStreamAdapterEdgeCases(t *testing.T) {
	t.Run("NextAfterDone", func(t *testing.T) {
		chunks := []ChatCompletionChunk{
			{
				Choices: []ChatChoice{
					{Delta: ChatMessage{Content: "Hello"}},
				},
				Done: true,
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

		// Read first chunk
		chunk, err := adapter.Next()
		require.NoError(t, err)
		assert.NotNil(t, chunk)
		assert.True(t, adapter.Done())

		// Try to read again
		chunk, err = adapter.Next()
		assert.Nil(t, chunk)
		assert.NoError(t, err)
	})

	t.Run("StreamError", func(t *testing.T) {
		mockStream := &mockErrorStream{}
		extension := &mockExtension{
			BaseExtension: NewBaseExtension("test", "1.0.0", "Test", []string{"chat"}),
		}

		adapter := &StandardStreamAdapter{
			providerStream: mockStream,
			extension:      extension,
		}

		_, err := adapter.Next()
		require.Error(t, err)
		assert.True(t, adapter.Done())
	})
}

// mockErrorStream simulates a stream that errors
type mockErrorStream struct{}

func (m *mockErrorStream) Next() (ChatCompletionChunk, error) {
	return ChatCompletionChunk{}, fmt.Errorf("stream error")
}

func (m *mockErrorStream) Close() error {
	return nil
}

// BenchmarkCoreProviderAdapter benchmarks the adapter
func BenchmarkCoreProviderAdapter(b *testing.B) {
	provider := newMockStreamProvider("bench-provider", ProviderTypeOpenAI)
	extension := &mockExtension{
		BaseExtension: NewBaseExtension("bench", "1.0.0", "Benchmark", []string{"chat"}),
	}

	adapter := NewCoreProviderAdapter(provider, extension)
	ctx := context.Background()

	request := StandardRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
		Model:    "gpt-4",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.GenerateStandardCompletion(ctx, request)
	}
}
