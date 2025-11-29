package loadbalance

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Mock Implementations
// ============================================================================

type mockChatProvider struct {
	name     string
	err      error
	response string
}

func (m *mockChatProvider) Name() string              { return m.name }
func (m *mockChatProvider) Type() types.ProviderType { return "mock" }
func (m *mockChatProvider) Description() string      { return "mock provider" }

func (m *mockChatProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &mockStream{content: m.response}, nil
}

// Unused interface methods for full Provider interface compliance
func (m *mockChatProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return nil, nil
}
func (m *mockChatProvider) GetDefaultModel() string                                { return "" }
func (m *mockChatProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}
func (m *mockChatProvider) IsAuthenticated() bool { return true }
func (m *mockChatProvider) Logout(ctx context.Context) error { return nil }
func (m *mockChatProvider) Configure(config types.ProviderConfig) error { return nil }
func (m *mockChatProvider) GetConfig() types.ProviderConfig { return types.ProviderConfig{} }
func (m *mockChatProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockChatProvider) SupportsToolCalling() bool    { return false }
func (m *mockChatProvider) GetToolFormat() types.ToolFormat { return "" }
func (m *mockChatProvider) SupportsStreaming() bool      { return true }
func (m *mockChatProvider) SupportsResponsesAPI() bool   { return false }
func (m *mockChatProvider) HealthCheck(ctx context.Context) error { return nil }
func (m *mockChatProvider) GetMetrics() types.ProviderMetrics { return types.ProviderMetrics{} }

type mockStream struct {
	content string
	index   int
	closed  bool
}

func (s *mockStream) Next() (types.ChatCompletionChunk, error) {
	if s.closed {
		return types.ChatCompletionChunk{}, io.EOF
	}
	if s.index >= len(s.content) {
		s.closed = true
		return types.ChatCompletionChunk{Done: true}, io.EOF
	}

	chunk := types.ChatCompletionChunk{
		Content: string(s.content[s.index]),
		Done:    false,
	}
	s.index++
	return chunk, nil
}

func (s *mockStream) Close() error {
	s.closed = true
	return nil
}

// mockNonChatProvider is a provider that doesn't support chat
type mockNonChatProvider struct {
	name string
}

func (m *mockNonChatProvider) Name() string              { return m.name }
func (m *mockNonChatProvider) Type() types.ProviderType { return "non-chat" }
func (m *mockNonChatProvider) Description() string      { return "Non-chat provider" }
func (m *mockNonChatProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return nil, nil
}
func (m *mockNonChatProvider) GetDefaultModel() string { return "" }
func (m *mockNonChatProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}
func (m *mockNonChatProvider) IsAuthenticated() bool { return true }
func (m *mockNonChatProvider) Logout(ctx context.Context) error { return nil }
func (m *mockNonChatProvider) Configure(config types.ProviderConfig) error { return nil }
func (m *mockNonChatProvider) GetConfig() types.ProviderConfig { return types.ProviderConfig{} }
func (m *mockNonChatProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	return nil, errors.New("non-chat provider does not support chat")
}
func (m *mockNonChatProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockNonChatProvider) SupportsToolCalling() bool    { return false }
func (m *mockNonChatProvider) GetToolFormat() types.ToolFormat { return "" }
func (m *mockNonChatProvider) SupportsStreaming() bool      { return false }
func (m *mockNonChatProvider) SupportsResponsesAPI() bool   { return false }
func (m *mockNonChatProvider) HealthCheck(ctx context.Context) error { return nil }
func (m *mockNonChatProvider) GetMetrics() types.ProviderMetrics { return types.ProviderMetrics{} }

type mockHealthCheckProvider struct {
	mockChatProvider
	healthErr error
}

func (m *mockHealthCheckProvider) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

// ============================================================================
// Test Cases - Basic Functionality
// ============================================================================

func TestNewLoadBalanceProvider(t *testing.T) {
	config := &Config{
		Strategy:      StrategyRoundRobin,
		ProviderNames: []string{"provider1", "provider2"},
	}

	lb := NewLoadBalanceProvider("test-lb", config)

	if lb.Name() != "test-lb" {
		t.Errorf("expected name 'test-lb', got '%s'", lb.Name())
	}

	if lb.Type() != "loadbalance" {
		t.Errorf("expected type 'loadbalance', got '%s'", lb.Type())
	}

	if lb.Description() == "" {
		t.Error("expected non-empty description")
	}

	expectedDesc := "Distributes requests across providers"
	if lb.Description() != expectedDesc {
		t.Errorf("expected description '%s', got '%s'", expectedDesc, lb.Description())
	}
}

func TestLoadBalanceProvider_SetProviders(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1"},
		&mockChatProvider{name: "provider2"},
	}

	lb.SetProviders(providers)

	if len(lb.providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(lb.providers))
	}
}

// ============================================================================
// Test Cases - Round-Robin Strategy
// ============================================================================

func TestRoundRobinStrategy_DistributesEvenly(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	provider1 := &mockChatProvider{name: "provider1", response: "1"}
	provider2 := &mockChatProvider{name: "provider2", response: "2"}
	provider3 := &mockChatProvider{name: "provider3", response: "3"}

	lb.SetProviders([]types.Provider{provider1, provider2, provider3})

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	// Track which providers are selected
	selectedProviders := make(map[string]int)
	for i := 0; i < 9; i++ {
		stream, err := lb.GenerateChatCompletion(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}

		// Read from stream to identify which provider was used
		chunk, _ := stream.Next()
		content := chunk.Content
		_ = stream.Close()

		selectedProviders[content]++
	}

	// Each provider should have been selected exactly 3 times
	if selectedProviders["1"] != 3 {
		t.Errorf("expected provider1 to be selected 3 times, got %d", selectedProviders["1"])
	}
	if selectedProviders["2"] != 3 {
		t.Errorf("expected provider2 to be selected 3 times, got %d", selectedProviders["2"])
	}
	if selectedProviders["3"] != 3 {
		t.Errorf("expected provider3 to be selected 3 times, got %d", selectedProviders["3"])
	}
}

func TestRoundRobinStrategy_CounterIncrementsCorrectly(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "a"},
		&mockChatProvider{name: "provider2", response: "b"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	// First call should use provider1 (counter=0)
	stream1, _ := lb.GenerateChatCompletion(ctx, opts)
	chunk1, _ := stream1.Next()
	_ = stream1.Close()

	// Second call should use provider2 (counter=1)
	stream2, _ := lb.GenerateChatCompletion(ctx, opts)
	chunk2, _ := stream2.Next()
	_ = stream2.Close()

	if chunk1.Content == chunk2.Content {
		t.Error("expected different providers to be selected")
	}
}

func TestRoundRobinStrategy_WrapsAround(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "a"},
		&mockChatProvider{name: "provider2", response: "b"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	// Get responses from multiple iterations
	var responses []string
	for i := 0; i < 5; i++ {
		stream, _ := lb.GenerateChatCompletion(ctx, opts)
		chunk, _ := stream.Next()
		responses = append(responses, chunk.Content)
		_ = stream.Close()
	}

	// Pattern should be: a, b, a, b, a
	expected := []string{"a", "b", "a", "b", "a"}
	for i, exp := range expected {
		if responses[i] != exp {
			t.Errorf("at index %d: expected '%s', got '%s'", i, exp, responses[i])
		}
	}
}

// ============================================================================
// Test Cases - Random Strategy
// ============================================================================

func TestRandomStrategy_SelectsProviders(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRandom,
	})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "a"},
		&mockChatProvider{name: "provider2", response: "b"},
		&mockChatProvider{name: "provider3", response: "c"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	// Run many times and verify we get different providers
	selectedProviders := make(map[string]bool)
	for i := 0; i < 50; i++ {
		stream, err := lb.GenerateChatCompletion(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		chunk, _ := stream.Next()
		selectedProviders[chunk.Content] = true
		_ = stream.Close()
	}

	// We should see at least 2 different providers selected over 50 iterations
	if len(selectedProviders) < 2 {
		t.Errorf("expected at least 2 different providers to be selected, got %d", len(selectedProviders))
	}
}

func TestRandomStrategy_DoesNotPanic(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRandom,
	})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "a"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	// Should not panic with single provider
	_, err := lb.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================================
// Test Cases - Error Scenarios
// ============================================================================

func TestLoadBalanceProvider_NoProvidersConfigured(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	_, err := lb.GenerateChatCompletion(ctx, opts)

	if err == nil {
		t.Fatal("expected error when no providers configured")
	}

	expectedMsg := "no providers configured"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestLoadBalanceProvider_ProviderDoesNotSupportChat(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	// Add a non-chat provider that returns an error
	providers := []types.Provider{
		&mockNonChatProvider{name: "non-chat"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	_, err := lb.GenerateChatCompletion(ctx, opts)

	if err == nil {
		t.Fatal("expected error when provider doesn't support chat")
	}

	// The error comes from the mock provider's GenerateChatCompletion
	expectedMsg := "non-chat provider does not support chat"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

// ============================================================================
// Test Cases - Concurrency
// ============================================================================

func TestRoundRobinStrategy_ConcurrentAccess(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "a"},
		&mockChatProvider{name: "provider2", response: "b"},
		&mockChatProvider{name: "provider3", response: "c"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	var wg sync.WaitGroup
	numGoroutines := uint64(100)
	errChan := make(chan error, numGoroutines)

	for i := uint64(0); i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stream, err := lb.GenerateChatCompletion(ctx, opts)
			if err != nil {
				errChan <- err
				return
			}
			_ = stream.Close()
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("unexpected error in concurrent access: %v", err)
	}

	// Counter should be exactly numGoroutines
	if lb.counter != numGoroutines {
		t.Errorf("expected counter to be %d, got %d", numGoroutines, lb.counter)
	}
}

// ============================================================================
// Test Cases - Stub Methods
// ============================================================================

func TestLoadBalanceProvider_GetModels(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})
	ctx := context.Background()

	_, err := lb.GetModels(ctx)

	if err == nil {
		t.Fatal("expected error from GetModels")
	}

	expectedMsg := "GetModels not supported for virtual load balance provider"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestLoadBalanceProvider_GetDefaultModel(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	model := lb.GetDefaultModel()

	if model != "" {
		t.Errorf("expected empty string, got '%s'", model)
	}
}

func TestLoadBalanceProvider_SupportsToolCalling(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	if lb.SupportsToolCalling() {
		t.Error("expected SupportsToolCalling to return false")
	}
}

func TestLoadBalanceProvider_SupportsStreaming(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	if !lb.SupportsStreaming() {
		t.Error("expected SupportsStreaming to return true")
	}
}

func TestLoadBalanceProvider_SupportsResponsesAPI(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	if lb.SupportsResponsesAPI() {
		t.Error("expected SupportsResponsesAPI to return false")
	}
}

func TestLoadBalanceProvider_GetToolFormat(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	format := lb.GetToolFormat()

	if format != types.ToolFormatOpenAI {
		t.Errorf("expected ToolFormatOpenAI, got %s", format)
	}
}

func TestLoadBalanceProvider_Authenticate(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})
	ctx := context.Background()

	err := lb.Authenticate(ctx, types.AuthConfig{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestLoadBalanceProvider_IsAuthenticated(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	if !lb.IsAuthenticated() {
		t.Error("expected IsAuthenticated to return true")
	}
}

func TestLoadBalanceProvider_Logout(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})
	ctx := context.Background()

	err := lb.Logout(ctx)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestLoadBalanceProvider_Configure(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy:      StrategyRoundRobin,
		ProviderNames: []string{"old1"},
	})

	config := types.ProviderConfig{
		ProviderConfig: map[string]interface{}{
			"strategy":  "random",
			"providers": []string{"new1", "new2"},
		},
	}

	err := lb.Configure(config)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if lb.config.Strategy != StrategyRandom {
		t.Errorf("expected strategy to be 'random', got '%s'", lb.config.Strategy)
	}

	if len(lb.config.ProviderNames) != 2 {
		t.Errorf("expected 2 provider names, got %d", len(lb.config.ProviderNames))
	}

	if lb.config.ProviderNames[0] != "new1" || lb.config.ProviderNames[1] != "new2" {
		t.Errorf("expected provider names [new1, new2], got %v", lb.config.ProviderNames)
	}
}

func TestLoadBalanceProvider_ConfigureEmpty(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy:      StrategyRoundRobin,
		ProviderNames: []string{"old1"},
	})

	config := types.ProviderConfig{}

	err := lb.Configure(config)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Config should remain unchanged
	if lb.config.Strategy != StrategyRoundRobin {
		t.Errorf("expected strategy to remain 'round_robin', got '%s'", lb.config.Strategy)
	}

	if len(lb.config.ProviderNames) != 1 {
		t.Errorf("expected 1 provider name, got %d", len(lb.config.ProviderNames))
	}
}

func TestLoadBalanceProvider_GetConfig(t *testing.T) {
	lb := NewLoadBalanceProvider("test-lb", &Config{
		Strategy:      StrategyRandom,
		ProviderNames: []string{"provider1", "provider2"},
	})

	config := lb.GetConfig()

	if config.Type != "loadbalance" {
		t.Errorf("expected type 'loadbalance', got %s", config.Type)
	}

	if config.Name != "test-lb" {
		t.Errorf("expected name 'test-lb', got %s", config.Name)
	}

	strategy, ok := config.ProviderConfig["strategy"].(string)
	if !ok || strategy != "random" {
		t.Errorf("expected strategy 'random', got %v", config.ProviderConfig["strategy"])
	}

	providers, ok := config.ProviderConfig["providers"].([]string)
	if !ok || len(providers) != 2 {
		t.Errorf("expected 2 providers, got %v", config.ProviderConfig["providers"])
	}
}

func TestLoadBalanceProvider_InvokeServerTool(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})
	ctx := context.Background()

	_, err := lb.InvokeServerTool(ctx, "test-tool", nil)

	if err == nil {
		t.Fatal("expected error from InvokeServerTool")
	}

	expectedMsg := "tool calling not supported for virtual load balance provider"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestLoadBalanceProvider_HealthCheck_NoProviders(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})
	ctx := context.Background()

	err := lb.HealthCheck(ctx)

	if err == nil {
		t.Fatal("expected error when no providers configured")
	}

	expectedMsg := "no providers configured"
	if err.Error() != expectedMsg {
		t.Errorf("expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestLoadBalanceProvider_HealthCheck_HealthyProvider(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	providers := []types.Provider{
		&mockChatProvider{name: "healthy-provider"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	err := lb.HealthCheck(ctx)

	// Mock provider's HealthCheck returns nil, so at least one is healthy
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestLoadBalanceProvider_HealthCheck_AllUnhealthy(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	healthErr := errors.New("health check failed")
	providers := []types.Provider{
		&mockHealthCheckProvider{
			mockChatProvider: mockChatProvider{name: "unhealthy-1"},
			healthErr:        healthErr,
		},
		&mockHealthCheckProvider{
			mockChatProvider: mockChatProvider{name: "unhealthy-2"},
			healthErr:        errors.New("also unhealthy"),
		},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	err := lb.HealthCheck(ctx)

	if err == nil {
		t.Fatal("expected error when all providers are unhealthy")
	}

	expectedPrefix := "all providers unhealthy:"
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error to start with '%s', got: %v", expectedPrefix, err)
	}
}

func TestLoadBalanceProvider_HealthCheck_MixedHealth(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	providers := []types.Provider{
		&mockHealthCheckProvider{
			mockChatProvider: mockChatProvider{name: "unhealthy"},
			healthErr:        errors.New("unhealthy"),
		},
		&mockHealthCheckProvider{
			mockChatProvider: mockChatProvider{name: "healthy"},
			healthErr:        nil,
		},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	err := lb.HealthCheck(ctx)

	// Should succeed because at least one provider is healthy
	if err != nil {
		t.Errorf("expected no error when at least one provider is healthy, got %v", err)
	}
}

func TestLoadBalanceProvider_HealthCheck_NoHealthCheckProviders(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	// Use regular mock providers that don't implement HealthCheckProvider interface
	providers := []types.Provider{
		&mockChatProvider{name: "provider1"},
		&mockChatProvider{name: "provider2"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	err := lb.HealthCheck(ctx)

	// Should succeed because providers don't have health check capability
	// (the default implementation returns nil)
	if err != nil {
		t.Errorf("expected no error when providers don't implement health check, got %v", err)
	}
}

func TestLoadBalanceProvider_GetMetrics(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{})

	metrics := lb.GetMetrics()

	if metrics.RequestCount != 0 {
		t.Errorf("expected RequestCount to be 0, got %d", metrics.RequestCount)
	}

	if metrics.SuccessCount != 0 {
		t.Errorf("expected SuccessCount to be 0, got %d", metrics.SuccessCount)
	}

	if metrics.ErrorCount != 0 {
		t.Errorf("expected ErrorCount to be 0, got %d", metrics.ErrorCount)
	}
}

// ============================================================================
// Test Cases - Helper Functions
// ============================================================================

func TestRandomInt(t *testing.T) {
	// Test that randomInt returns values in valid range
	max := 10
	for i := 0; i < 100; i++ {
		result := randomInt(max)
		if result < 0 || result >= max {
			t.Errorf("randomInt(%d) returned %d, expected value in range [0, %d)", max, result, max)
		}
	}
}

func TestRandomInt_SingleValue(t *testing.T) {
	// With max=1, should always return 0
	for i := 0; i < 10; i++ {
		result := randomInt(1)
		if result != 0 {
			t.Errorf("randomInt(1) returned %d, expected 0", result)
		}
	}
}

// ============================================================================
// Test Cases - Edge Cases
// ============================================================================

func TestLoadBalanceProvider_SingleProvider(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	provider := &mockChatProvider{name: "only-provider", response: "response"}
	lb.SetProviders([]types.Provider{provider})

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	// Call multiple times - should always use the same provider
	for i := 0; i < 5; i++ {
		stream, err := lb.GenerateChatCompletion(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		_ = stream.Close()
	}

	// Counter should have incremented
	if lb.counter != 5 {
		t.Errorf("expected counter to be 5, got %d", lb.counter)
	}
}

func TestLoadBalanceProvider_DefaultStrategy(t *testing.T) {
	// When no strategy is specified, should default to round-robin
	lb := NewLoadBalanceProvider("test", &Config{})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "a"},
		&mockChatProvider{name: "provider2", response: "b"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	// Should use round-robin by default
	stream1, _ := lb.GenerateChatCompletion(ctx, opts)
	chunk1, _ := stream1.Next()
	_ = stream1.Close()

	stream2, _ := lb.GenerateChatCompletion(ctx, opts)
	chunk2, _ := stream2.Next()
	_ = stream2.Close()

	stream3, _ := lb.GenerateChatCompletion(ctx, opts)
	chunk3, _ := stream3.Next()
	_ = stream3.Close()

	// Should cycle through providers
	if chunk1.Content == chunk2.Content {
		t.Error("expected different providers for consecutive calls")
	}
	if chunk1.Content != chunk3.Content {
		t.Error("expected same provider after full cycle")
	}
}

// ============================================================================
// Test Cases - Context Handling
// ============================================================================

func TestLoadBalanceProvider_ContextCancellation(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	// Create a provider that respects context cancellation
	provider := &mockChatProvider{name: "provider", response: "response"}
	lb.SetProviders([]types.Provider{provider})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := types.GenerateOptions{Prompt: "test"}

	// The provider itself doesn't check context in our mock,
	// but the load balancer should pass it through
	_, err := lb.GenerateChatCompletion(ctx, opts)

	// Our mock doesn't actually check context, but this tests that
	// the context is passed through to the provider
	if err != nil {
		// If the provider checked context, it would fail here
		// Our mock doesn't, so we just verify no panic occurred
	}
}

// ============================================================================
// Test Cases - Weighted Strategy (Future)
// ============================================================================

func TestWeightedStrategy_FallsBackToRoundRobin(t *testing.T) {
	// Weighted strategy is not yet implemented, should fall back to round-robin
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyWeighted,
	})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "a"},
		&mockChatProvider{name: "provider2", response: "b"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	// Should fall back to round-robin behavior
	stream1, _ := lb.GenerateChatCompletion(ctx, opts)
	chunk1, _ := stream1.Next()
	_ = stream1.Close()

	stream2, _ := lb.GenerateChatCompletion(ctx, opts)
	chunk2, _ := stream2.Next()
	_ = stream2.Close()

	// Should cycle through providers like round-robin
	if chunk1.Content == chunk2.Content {
		t.Error("expected different providers for consecutive calls")
	}
}

// ============================================================================
// Test Cases - Stream Integration
// ============================================================================

func TestLoadBalanceProvider_StreamReturned(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	provider := &mockChatProvider{name: "provider", response: "hello"}
	lb.SetProviders([]types.Provider{provider})

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	stream, err := lb.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Read all chunks
	var content string
	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error reading stream: %v", err)
		}
		if !chunk.Done {
			content += chunk.Content
		}
	}

	if content != "hello" {
		t.Errorf("expected content 'hello', got '%s'", content)
	}
}

func TestLoadBalanceProvider_StreamClose(t *testing.T) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	provider := &mockChatProvider{name: "provider", response: "test"}
	lb.SetProviders([]types.Provider{provider})

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	stream, err := lb.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = stream.Close()
	if err != nil {
		t.Errorf("unexpected error closing stream: %v", err)
	}

	// Stream should be closed
	mockStr, ok := stream.(*mockStream)
	if ok && !mockStr.closed {
		t.Error("expected stream to be closed")
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkRoundRobinSelection(b *testing.B) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRoundRobin,
	})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "a"},
		&mockChatProvider{name: "provider2", response: "b"},
		&mockChatProvider{name: "provider3", response: "c"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, _ := lb.GenerateChatCompletion(ctx, opts)
		_ = stream.Close()
	}
}

func BenchmarkRandomSelection(b *testing.B) {
	lb := NewLoadBalanceProvider("test", &Config{
		Strategy: StrategyRandom,
	})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "a"},
		&mockChatProvider{name: "provider2", response: "b"},
		&mockChatProvider{name: "provider3", response: "c"},
	}

	lb.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, _ := lb.GenerateChatCompletion(ctx, opts)
		_ = stream.Close()
	}
}
