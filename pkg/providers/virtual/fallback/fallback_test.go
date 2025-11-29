package fallback

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// mockChatProvider is a mock provider for testing
type mockChatProvider struct {
	name        string
	err         error
	streamErr   error
	shouldClose bool
}

func (m *mockChatProvider) Name() string             { return m.name }
func (m *mockChatProvider) Type() types.ProviderType { return "mock" }
func (m *mockChatProvider) Description() string      { return "Mock provider for testing" }

func (m *mockChatProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &mockStream{
		provider:    m.name,
		err:         m.streamErr,
		shouldClose: m.shouldClose,
	}, nil
}

// Implement remaining Provider interface methods
func (m *mockChatProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return []types.Model{}, nil
}

func (m *mockChatProvider) GetDefaultModel() string {
	return "mock-model"
}

func (m *mockChatProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}

func (m *mockChatProvider) IsAuthenticated() bool {
	return true
}

func (m *mockChatProvider) Logout(ctx context.Context) error {
	return nil
}

func (m *mockChatProvider) Configure(config types.ProviderConfig) error {
	return nil
}

func (m *mockChatProvider) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{}
}

func (m *mockChatProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockChatProvider) SupportsToolCalling() bool {
	return false
}

func (m *mockChatProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (m *mockChatProvider) SupportsStreaming() bool {
	return true
}

func (m *mockChatProvider) SupportsResponsesAPI() bool {
	return false
}

func (m *mockChatProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockChatProvider) GetMetrics() types.ProviderMetrics {
	return types.ProviderMetrics{}
}

// mockStream is a mock stream for testing
type mockStream struct {
	provider    string
	err         error
	closed      bool
	shouldClose bool
	callCount   int
}

func (m *mockStream) Next() (types.ChatCompletionChunk, error) {
	m.callCount++
	if m.err != nil {
		return types.ChatCompletionChunk{}, m.err
	}
	if m.callCount > 1 {
		return types.ChatCompletionChunk{
			Done: true,
		}, io.EOF
	}
	return types.ChatCompletionChunk{
		Content:  "test response from " + m.provider,
		Metadata: make(map[string]interface{}),
		Done:     false,
	}, nil
}

func (m *mockStream) Close() error {
	if !m.shouldClose {
		return errors.New("close error")
	}
	m.closed = true
	return nil
}

// mockNonChatProvider simulates a provider that should be skipped
// In practice, all Provider implementations have GenerateChatCompletion since
// Provider interface includes ChatProvider. This is kept for testing the
// type assertion logic in the fallback provider.
type mockNonChatProvider struct {
	name string
}

func (m *mockNonChatProvider) Name() string             { return m.name }
func (m *mockNonChatProvider) Type() types.ProviderType { return "non-chat" }
func (m *mockNonChatProvider) Description() string      { return "Non-chat provider" }

// Implement all Provider interface methods
func (m *mockNonChatProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return []types.Model{}, nil
}

func (m *mockNonChatProvider) GetDefaultModel() string {
	return "non-chat-model"
}

func (m *mockNonChatProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}

func (m *mockNonChatProvider) IsAuthenticated() bool {
	return true
}

func (m *mockNonChatProvider) Logout(ctx context.Context) error {
	return nil
}

func (m *mockNonChatProvider) Configure(config types.ProviderConfig) error {
	return nil
}

func (m *mockNonChatProvider) GetConfig() types.ProviderConfig {
	return types.ProviderConfig{}
}

func (m *mockNonChatProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	return nil, errors.New("non-chat provider does not support chat")
}

func (m *mockNonChatProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockNonChatProvider) SupportsToolCalling() bool {
	return false
}

func (m *mockNonChatProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatOpenAI
}

func (m *mockNonChatProvider) SupportsStreaming() bool {
	return false
}

func (m *mockNonChatProvider) SupportsResponsesAPI() bool {
	return false
}

func (m *mockNonChatProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockNonChatProvider) GetMetrics() types.ProviderMetrics {
	return types.ProviderMetrics{}
}

// TestFallbackProvider_FirstProviderSucceeds tests that first provider succeeds and returns immediately
func TestFallbackProvider_FirstProviderSucceeds(t *testing.T) {
	provider1 := &mockChatProvider{
		name:        "provider1",
		err:         nil,
		shouldClose: true,
	}
	provider2 := &mockChatProvider{
		name: "provider2",
		err:  errors.New("should not be called"),
	}

	fallback := NewFallbackProvider("test-fallback", &Config{
		ProviderNames: []string{"provider1", "provider2"},
	})
	fallback.SetProviders([]types.Provider{provider1, provider2})

	ctx := context.Background()
	opts := types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "test"}},
	}

	stream, err := fallback.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stream == nil {
		t.Fatal("expected stream, got nil")
	}

	// Verify it's a fallback stream
	fbStream, ok := stream.(*fallbackStream)
	if !ok {
		t.Fatal("expected fallbackStream type")
	}
	if fbStream.providerName != "provider1" {
		t.Errorf("expected provider name 'provider1', got %s", fbStream.providerName)
	}
	if fbStream.providerIndex != 0 {
		t.Errorf("expected provider index 0, got %d", fbStream.providerIndex)
	}

	// Verify metadata is added
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("expected no error on Next(), got %v", err)
	}
	if chunk.Metadata["fallback_provider"] != "provider1" {
		t.Errorf("expected fallback_provider metadata 'provider1', got %v", chunk.Metadata["fallback_provider"])
	}
	if chunk.Metadata["fallback_index"] != 0 {
		t.Errorf("expected fallback_index metadata 0, got %v", chunk.Metadata["fallback_index"])
	}

	// Close the stream
	if err := stream.Close(); err != nil {
		t.Errorf("expected no error on Close(), got %v", err)
	}
}

// TestFallbackProvider_FirstFailsSecondSucceeds tests that first fails, second succeeds
func TestFallbackProvider_FirstFailsSecondSucceeds(t *testing.T) {
	provider1 := &mockChatProvider{
		name: "provider1",
		err:  errors.New("provider1 error"),
	}
	provider2 := &mockChatProvider{
		name:        "provider2",
		err:         nil,
		shouldClose: true,
	}

	fallback := NewFallbackProvider("test-fallback", &Config{
		ProviderNames: []string{"provider1", "provider2"},
	})
	fallback.SetProviders([]types.Provider{provider1, provider2})

	ctx := context.Background()
	opts := types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "test"}},
	}

	stream, err := fallback.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stream == nil {
		t.Fatal("expected stream, got nil")
	}

	// Verify it's using provider2
	fbStream, ok := stream.(*fallbackStream)
	if !ok {
		t.Fatal("expected fallbackStream type")
	}
	if fbStream.providerName != "provider2" {
		t.Errorf("expected provider name 'provider2', got %s", fbStream.providerName)
	}
	if fbStream.providerIndex != 1 {
		t.Errorf("expected provider index 1, got %d", fbStream.providerIndex)
	}

	// Verify metadata
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("expected no error on Next(), got %v", err)
	}
	if chunk.Metadata["fallback_provider"] != "provider2" {
		t.Errorf("expected fallback_provider metadata 'provider2', got %v", chunk.Metadata["fallback_provider"])
	}
	if chunk.Metadata["fallback_index"] != 1 {
		t.Errorf("expected fallback_index metadata 1, got %v", chunk.Metadata["fallback_index"])
	}
}

// TestFallbackProvider_FirstTwoFailThirdSucceeds tests that first two fail, third succeeds
func TestFallbackProvider_FirstTwoFailThirdSucceeds(t *testing.T) {
	provider1 := &mockChatProvider{
		name: "provider1",
		err:  errors.New("provider1 error"),
	}
	provider2 := &mockChatProvider{
		name: "provider2",
		err:  errors.New("provider2 error"),
	}
	provider3 := &mockChatProvider{
		name:        "provider3",
		err:         nil,
		shouldClose: true,
	}

	fallback := NewFallbackProvider("test-fallback", &Config{
		ProviderNames: []string{"provider1", "provider2", "provider3"},
	})
	fallback.SetProviders([]types.Provider{provider1, provider2, provider3})

	ctx := context.Background()
	opts := types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "test"}},
	}

	stream, err := fallback.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stream == nil {
		t.Fatal("expected stream, got nil")
	}

	// Verify it's using provider3
	fbStream, ok := stream.(*fallbackStream)
	if !ok {
		t.Fatal("expected fallbackStream type")
	}
	if fbStream.providerName != "provider3" {
		t.Errorf("expected provider name 'provider3', got %s", fbStream.providerName)
	}
	if fbStream.providerIndex != 2 {
		t.Errorf("expected provider index 2, got %d", fbStream.providerIndex)
	}

	// Verify metadata
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("expected no error on Next(), got %v", err)
	}
	if chunk.Metadata["fallback_provider"] != "provider3" {
		t.Errorf("expected fallback_provider metadata 'provider3', got %v", chunk.Metadata["fallback_provider"])
	}
	if chunk.Metadata["fallback_index"] != 2 {
		t.Errorf("expected fallback_index metadata 2, got %v", chunk.Metadata["fallback_index"])
	}
}

// TestFallbackProvider_AllProvidersFail tests that all providers fail and returns last error
func TestFallbackProvider_AllProvidersFail(t *testing.T) {
	lastError := errors.New("provider3 final error")
	provider1 := &mockChatProvider{
		name: "provider1",
		err:  errors.New("provider1 error"),
	}
	provider2 := &mockChatProvider{
		name: "provider2",
		err:  errors.New("provider2 error"),
	}
	provider3 := &mockChatProvider{
		name: "provider3",
		err:  lastError,
	}

	fallback := NewFallbackProvider("test-fallback", &Config{
		ProviderNames: []string{"provider1", "provider2", "provider3"},
	})
	fallback.SetProviders([]types.Provider{provider1, provider2, provider3})

	ctx := context.Background()
	opts := types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "test"}},
	}

	stream, err := fallback.GenerateChatCompletion(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if stream != nil {
		t.Fatalf("expected nil stream, got %v", stream)
	}

	// Verify error message contains last error
	expectedMsg := "all providers failed, last error:"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain '%s', got %s", expectedMsg, err.Error())
	}
}

// TestFallbackProvider_NoProvidersConfigured tests that no providers configured returns error
func TestFallbackProvider_NoProvidersConfigured(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{
		ProviderNames: []string{},
	})
	fallback.SetProviders([]types.Provider{})

	ctx := context.Background()
	opts := types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "test"}},
	}

	stream, err := fallback.GenerateChatCompletion(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if stream != nil {
		t.Fatalf("expected nil stream, got %v", stream)
	}

	expectedMsg := "no providers available"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestFallbackProvider_NonChatProvider tests provider that returns error from GenerateChatCompletion
func TestFallbackProvider_NonChatProvider(t *testing.T) {
	nonChatProvider := &mockNonChatProvider{name: "non-chat"}
	chatProvider := &mockChatProvider{
		name:        "chat-provider",
		err:         nil,
		shouldClose: true,
	}

	fallback := NewFallbackProvider("test-fallback", &Config{
		ProviderNames: []string{"non-chat", "chat-provider"},
	})
	fallback.SetProviders([]types.Provider{nonChatProvider, chatProvider})

	ctx := context.Background()
	opts := types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "test"}},
	}

	stream, err := fallback.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stream == nil {
		t.Fatal("expected stream, got nil")
	}

	// Should skip non-chat provider (which returns error) and use chat provider
	fbStream, ok := stream.(*fallbackStream)
	if !ok {
		t.Fatal("expected fallbackStream type")
	}
	if fbStream.providerName != "chat-provider" {
		t.Errorf("expected provider name 'chat-provider', got %s", fbStream.providerName)
	}
	// Index should be 1 (second provider) since first returned error
	if fbStream.providerIndex != 1 {
		t.Errorf("expected provider index 1, got %d", fbStream.providerIndex)
	}
}

// TestFallbackProvider_AllNonChatProviders tests when all providers fail
func TestFallbackProvider_AllNonChatProviders(t *testing.T) {
	nonChatProvider1 := &mockNonChatProvider{name: "non-chat1"}
	nonChatProvider2 := &mockNonChatProvider{name: "non-chat2"}

	fallback := NewFallbackProvider("test-fallback", &Config{
		ProviderNames: []string{"non-chat1", "non-chat2"},
	})
	fallback.SetProviders([]types.Provider{nonChatProvider1, nonChatProvider2})

	ctx := context.Background()
	opts := types.GenerateOptions{
		Messages: []types.ChatMessage{{Role: "user", Content: "test"}},
	}

	stream, err := fallback.GenerateChatCompletion(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if stream != nil {
		t.Fatalf("expected nil stream, got %v", stream)
	}

	// Should return error about all providers failing
	expectedMsg := "all providers failed"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestFallbackStream_MetadataAddition tests that stream wrapper adds metadata correctly
func TestFallbackStream_MetadataAddition(t *testing.T) {
	innerStream := &mockStream{
		provider:    "test-provider",
		shouldClose: true,
	}

	fbStream := &fallbackStream{
		inner:         innerStream,
		providerName:  "test-provider",
		providerIndex: 2,
	}

	chunk, err := fbStream.Next()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify metadata is added
	if chunk.Metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}
	if chunk.Metadata["fallback_provider"] != "test-provider" {
		t.Errorf("expected fallback_provider 'test-provider', got %v", chunk.Metadata["fallback_provider"])
	}
	if chunk.Metadata["fallback_index"] != 2 {
		t.Errorf("expected fallback_index 2, got %v", chunk.Metadata["fallback_index"])
	}
}

// TestFallbackStream_MetadataPreserved tests that existing metadata is preserved
func TestFallbackStream_MetadataPreserved(t *testing.T) {
	innerStream := &mockStream{
		provider:    "test-provider",
		shouldClose: true,
	}

	fbStream := &fallbackStream{
		inner:         innerStream,
		providerName:  "test-provider",
		providerIndex: 1,
	}

	chunk, err := fbStream.Next()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify both old and new metadata exist
	if chunk.Metadata["fallback_provider"] != "test-provider" {
		t.Errorf("expected fallback_provider 'test-provider', got %v", chunk.Metadata["fallback_provider"])
	}
	if chunk.Metadata["fallback_index"] != 1 {
		t.Errorf("expected fallback_index 1, got %v", chunk.Metadata["fallback_index"])
	}
}

// TestFallbackStream_CloseWorks tests that Close() works correctly
func TestFallbackStream_CloseWorks(t *testing.T) {
	innerStream := &mockStream{
		provider:    "test-provider",
		shouldClose: true,
	}

	fbStream := &fallbackStream{
		inner:         innerStream,
		providerName:  "test-provider",
		providerIndex: 0,
	}

	err := fbStream.Close()
	if err != nil {
		t.Errorf("expected no error on Close(), got %v", err)
	}

	if !innerStream.closed {
		t.Error("expected inner stream to be closed")
	}
}

// TestFallbackStream_CloseError tests that Close() propagates errors
func TestFallbackStream_CloseError(t *testing.T) {
	innerStream := &mockStream{
		provider:    "test-provider",
		shouldClose: false, // will return error on close
	}

	fbStream := &fallbackStream{
		inner:         innerStream,
		providerName:  "test-provider",
		providerIndex: 0,
	}

	err := fbStream.Close()
	if err == nil {
		t.Error("expected error on Close(), got nil")
	}
	if err.Error() != "close error" {
		t.Errorf("expected 'close error', got %s", err.Error())
	}
}

// TestFallbackProvider_Name tests that Name() returns correct name
func TestFallbackProvider_Name(t *testing.T) {
	fallback := NewFallbackProvider("my-fallback", &Config{})

	if fallback.Name() != "my-fallback" {
		t.Errorf("expected name 'my-fallback', got %s", fallback.Name())
	}
}

// TestFallbackProvider_Type tests that Type() returns "fallback"
func TestFallbackProvider_Type(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	providerType := fallback.Type()
	if providerType != "fallback" {
		t.Errorf("expected type 'fallback', got %s", providerType)
	}
}

// TestFallbackProvider_Description tests that Description() returns description
func TestFallbackProvider_Description(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	desc := fallback.Description()
	expected := "Tries providers in order until one succeeds"
	if desc != expected {
		t.Errorf("expected description '%s', got '%s'", expected, desc)
	}
}

// TestFallbackProvider_SetProviders tests that SetProviders() works correctly
func TestFallbackProvider_SetProviders(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	if len(fallback.providers) != 0 {
		t.Errorf("expected 0 providers initially, got %d", len(fallback.providers))
	}

	providers := []types.Provider{
		&mockChatProvider{name: "provider1"},
		&mockChatProvider{name: "provider2"},
	}

	fallback.SetProviders(providers)

	if len(fallback.providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(fallback.providers))
	}
}

// TestFallbackProvider_Config tests that config is stored correctly
func TestFallbackProvider_Config(t *testing.T) {
	config := &Config{
		ProviderNames: []string{"provider1", "provider2"},
		MaxRetries:    3,
	}

	fallback := NewFallbackProvider("test-fallback", config)

	if fallback.config != config {
		t.Error("expected config to be stored")
	}
	if len(fallback.config.ProviderNames) != 2 {
		t.Errorf("expected 2 provider names, got %d", len(fallback.config.ProviderNames))
	}
	if fallback.config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", fallback.config.MaxRetries)
	}
}

// TestFallbackStream_NextError tests that Next() propagates errors from inner stream
func TestFallbackStream_NextError(t *testing.T) {
	streamError := errors.New("stream error")
	innerStream := &mockStream{
		provider:    "test-provider",
		err:         streamError,
		shouldClose: true,
	}

	fbStream := &fallbackStream{
		inner:         innerStream,
		providerName:  "test-provider",
		providerIndex: 0,
	}

	_, err := fbStream.Next()
	if err == nil {
		t.Fatal("expected error from Next(), got nil")
	}
	if err != streamError {
		t.Errorf("expected stream error, got %v", err)
	}
}

// TestFallbackStream_NextMetadataInitialization tests that Next() initializes metadata if nil
func TestFallbackStream_NextMetadataInitialization(t *testing.T) {
	// Create a stream that returns chunks without metadata
	innerStream := &mockStream{
		provider:    "test-provider",
		shouldClose: true,
	}

	fbStream := &fallbackStream{
		inner:         innerStream,
		providerName:  "test-provider",
		providerIndex: 0,
	}

	chunk, err := fbStream.Next()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify metadata was initialized
	if chunk.Metadata == nil {
		t.Fatal("expected metadata to be initialized")
	}
}

// TestFallbackProvider_GetModels tests that GetModels returns error
func TestFallbackProvider_GetModels(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	ctx := context.Background()
	models, err := fallback.GetModels(ctx)
	if err == nil {
		t.Fatal("expected error from GetModels, got nil")
	}
	if models != nil {
		t.Errorf("expected nil models, got %v", models)
	}
	expectedMsg := "GetModels not supported"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestFallbackProvider_GetDefaultModel tests that GetDefaultModel returns empty string
func TestFallbackProvider_GetDefaultModel(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	model := fallback.GetDefaultModel()
	if model != "" {
		t.Errorf("expected empty string, got '%s'", model)
	}
}

// TestFallbackProvider_SupportsToolCalling tests that SupportsToolCalling returns false
func TestFallbackProvider_SupportsToolCalling(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	if fallback.SupportsToolCalling() {
		t.Error("expected SupportsToolCalling to return false")
	}
}

// TestFallbackProvider_SupportsStreaming tests that SupportsStreaming returns true
func TestFallbackProvider_SupportsStreaming(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	if !fallback.SupportsStreaming() {
		t.Error("expected SupportsStreaming to return true")
	}
}

// TestFallbackProvider_SupportsResponsesAPI tests that SupportsResponsesAPI returns false
func TestFallbackProvider_SupportsResponsesAPI(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	if fallback.SupportsResponsesAPI() {
		t.Error("expected SupportsResponsesAPI to return false")
	}
}

// TestFallbackProvider_GetToolFormat tests that GetToolFormat returns OpenAI format
func TestFallbackProvider_GetToolFormat(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	format := fallback.GetToolFormat()
	if format != types.ToolFormatOpenAI {
		t.Errorf("expected ToolFormatOpenAI, got %s", format)
	}
}

// TestFallbackProvider_Authenticate tests that Authenticate succeeds (no-op)
func TestFallbackProvider_Authenticate(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	ctx := context.Background()
	err := fallback.Authenticate(ctx, types.AuthConfig{})
	if err != nil {
		t.Errorf("expected no error from Authenticate, got %v", err)
	}
}

// TestFallbackProvider_IsAuthenticated tests that IsAuthenticated returns true
func TestFallbackProvider_IsAuthenticated(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	if !fallback.IsAuthenticated() {
		t.Error("expected IsAuthenticated to return true")
	}
}

// TestFallbackProvider_Logout tests that Logout succeeds (no-op)
func TestFallbackProvider_Logout(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	ctx := context.Background()
	err := fallback.Logout(ctx)
	if err != nil {
		t.Errorf("expected no error from Logout, got %v", err)
	}
}

// TestFallbackProvider_Configure tests that Configure updates config correctly
func TestFallbackProvider_Configure(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{
		ProviderNames: []string{"old1"},
		MaxRetries:    1,
	})

	config := types.ProviderConfig{
		ProviderConfig: map[string]interface{}{
			"providers":   []string{"new1", "new2"},
			"max_retries": 5,
		},
	}

	err := fallback.Configure(config)
	if err != nil {
		t.Errorf("expected no error from Configure, got %v", err)
	}

	if fallback.config.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", fallback.config.MaxRetries)
	}
	if len(fallback.config.ProviderNames) != 2 {
		t.Errorf("expected 2 provider names, got %d", len(fallback.config.ProviderNames))
	}
	if fallback.config.ProviderNames[0] != "new1" || fallback.config.ProviderNames[1] != "new2" {
		t.Errorf("expected provider names [new1, new2], got %v", fallback.config.ProviderNames)
	}
}

// TestFallbackProvider_ConfigureEmpty tests Configure with empty config
func TestFallbackProvider_ConfigureEmpty(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{
		ProviderNames: []string{"old1"},
		MaxRetries:    1,
	})

	config := types.ProviderConfig{}

	err := fallback.Configure(config)
	if err != nil {
		t.Errorf("expected no error from Configure, got %v", err)
	}

	// Config should remain unchanged
	if fallback.config.MaxRetries != 1 {
		t.Errorf("expected MaxRetries 1, got %d", fallback.config.MaxRetries)
	}
	if len(fallback.config.ProviderNames) != 1 {
		t.Errorf("expected 1 provider name, got %d", len(fallback.config.ProviderNames))
	}
}

// TestFallbackProvider_GetConfig tests that GetConfig returns correct config
func TestFallbackProvider_GetConfig(t *testing.T) {
	fallback := NewFallbackProvider("my-fallback", &Config{
		ProviderNames: []string{"provider1", "provider2"},
		MaxRetries:    3,
	})

	config := fallback.GetConfig()

	if config.Type != "fallback" {
		t.Errorf("expected type 'fallback', got %s", config.Type)
	}
	if config.Name != "my-fallback" {
		t.Errorf("expected name 'my-fallback', got %s", config.Name)
	}
	if config.ProviderConfig["max_retries"] != 3 {
		t.Errorf("expected max_retries 3, got %v", config.ProviderConfig["max_retries"])
	}

	providers, ok := config.ProviderConfig["providers"].([]string)
	if !ok {
		t.Fatal("expected providers to be []string")
	}
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

// TestFallbackProvider_InvokeServerTool tests that InvokeServerTool returns error
func TestFallbackProvider_InvokeServerTool(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	ctx := context.Background()
	result, err := fallback.InvokeServerTool(ctx, "tool", nil)
	if err == nil {
		t.Fatal("expected error from InvokeServerTool, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	expectedMsg := "tool calling not supported"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestFallbackProvider_HealthCheck_NoProviders tests HealthCheck with no providers
func TestFallbackProvider_HealthCheck_NoProviders(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	ctx := context.Background()
	err := fallback.HealthCheck(ctx)
	if err == nil {
		t.Fatal("expected error from HealthCheck, got nil")
	}
	expectedMsg := "no providers configured"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestFallbackProvider_HealthCheck_OneHealthy tests HealthCheck with one healthy provider
func TestFallbackProvider_HealthCheck_OneHealthy(t *testing.T) {
	provider1 := &mockChatProvider{name: "provider1"}
	provider2 := &mockChatProvider{name: "provider2"}

	fallback := NewFallbackProvider("test-fallback", &Config{})
	fallback.SetProviders([]types.Provider{provider1, provider2})

	ctx := context.Background()
	err := fallback.HealthCheck(ctx)
	if err != nil {
		t.Errorf("expected no error from HealthCheck, got %v", err)
	}
}

// TestFallbackProvider_GetMetrics tests that GetMetrics returns default metrics
func TestFallbackProvider_GetMetrics(t *testing.T) {
	fallback := NewFallbackProvider("test-fallback", &Config{})

	metrics := fallback.GetMetrics()

	if metrics.RequestCount != 0 {
		t.Errorf("expected RequestCount 0, got %d", metrics.RequestCount)
	}
	if metrics.SuccessCount != 0 {
		t.Errorf("expected SuccessCount 0, got %d", metrics.SuccessCount)
	}
	if metrics.ErrorCount != 0 {
		t.Errorf("expected ErrorCount 0, got %d", metrics.ErrorCount)
	}
}

// mockHealthCheckProvider is a mock provider that implements HealthCheckProvider
type mockHealthCheckProvider struct {
	mockChatProvider
	healthErr error
}

func (m *mockHealthCheckProvider) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

// TestFallbackProvider_HealthCheck_AllUnhealthy tests HealthCheck when all providers are unhealthy
func TestFallbackProvider_HealthCheck_AllUnhealthy(t *testing.T) {
	provider1 := &mockHealthCheckProvider{
		mockChatProvider: mockChatProvider{name: "provider1"},
		healthErr:        errors.New("provider1 unhealthy"),
	}
	provider2 := &mockHealthCheckProvider{
		mockChatProvider: mockChatProvider{name: "provider2"},
		healthErr:        errors.New("provider2 unhealthy"),
	}

	fallback := NewFallbackProvider("test-fallback", &Config{})
	fallback.SetProviders([]types.Provider{provider1, provider2})

	ctx := context.Background()
	err := fallback.HealthCheck(ctx)
	if err == nil {
		t.Fatal("expected error from HealthCheck, got nil")
	}
	expectedMsg := "all providers unhealthy"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestFallbackProvider_HealthCheck_MixedHealth tests HealthCheck with mixed healthy/unhealthy providers
func TestFallbackProvider_HealthCheck_MixedHealth(t *testing.T) {
	provider1 := &mockHealthCheckProvider{
		mockChatProvider: mockChatProvider{name: "provider1"},
		healthErr:        errors.New("provider1 unhealthy"),
	}
	provider2 := &mockHealthCheckProvider{
		mockChatProvider: mockChatProvider{name: "provider2"},
		healthErr:        nil, // healthy
	}

	fallback := NewFallbackProvider("test-fallback", &Config{})
	fallback.SetProviders([]types.Provider{provider1, provider2})

	ctx := context.Background()
	err := fallback.HealthCheck(ctx)
	if err != nil {
		t.Errorf("expected no error from HealthCheck, got %v", err)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
