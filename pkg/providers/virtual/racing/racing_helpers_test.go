package racing

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Mock Implementations
// ============================================================================

type mockChatProvider struct {
	name     string
	delay    time.Duration
	err      error
	response string
}

func (m *mockChatProvider) Name() string             { return m.name }
func (m *mockChatProvider) Type() types.ProviderType { return "mock" }
func (m *mockChatProvider) Description() string      { return "mock provider" }

func (m *mockChatProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	select {
	case <-time.After(m.delay):
		if m.err != nil {
			return nil, m.err
		}
		return &mockStream{content: m.response}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Unused interface methods for full Provider interface compliance
func (m *mockChatProvider) GetModels(ctx context.Context) ([]types.Model, error) { return nil, nil }
func (m *mockChatProvider) GetDefaultModel() string                              { return "" }
func (m *mockChatProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	return nil
}
func (m *mockChatProvider) IsAuthenticated() bool                       { return true }
func (m *mockChatProvider) Logout(ctx context.Context) error            { return nil }
func (m *mockChatProvider) Configure(config types.ProviderConfig) error { return nil }
func (m *mockChatProvider) GetConfig() types.ProviderConfig             { return types.ProviderConfig{} }
func (m *mockChatProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockChatProvider) SupportsToolCalling() bool             { return false }
func (m *mockChatProvider) GetToolFormat() types.ToolFormat       { return "" }
func (m *mockChatProvider) SupportsStreaming() bool               { return true }
func (m *mockChatProvider) SupportsResponsesAPI() bool            { return false }
func (m *mockChatProvider) HealthCheck(ctx context.Context) error { return nil }
func (m *mockChatProvider) GetMetrics() types.ProviderMetrics     { return types.ProviderMetrics{} }

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

type mockHealthCheckProvider struct {
	*mockChatProvider
	healthErr error
}

func (m *mockHealthCheckProvider) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

// ============================================================================
// Helper Functions
// ============================================================================

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}

func findSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}
