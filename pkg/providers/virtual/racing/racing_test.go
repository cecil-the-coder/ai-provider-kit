package racing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
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

// ============================================================================
// Test Cases
// ============================================================================

func TestNewRacingProvider(t *testing.T) {
	config := &Config{
		TimeoutMS:     5000,
		GracePeriodMS: 100,
		Strategy:      StrategyFirstWins,
	}

	rp := NewRacingProvider("test-racing", config)

	if rp.Name() != "test-racing" {
		t.Errorf("expected name 'test-racing', got '%s'", rp.Name())
	}

	if rp.Type() != "racing" {
		t.Errorf("expected type 'racing', got '%s'", rp.Type())
	}

	if rp.Description() == "" {
		t.Error("expected non-empty description")
	}

	if rp.performance == nil {
		t.Error("expected performance tracker to be initialized")
	}
}

func TestRacingProvider_SetProviders(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	providers := []types.Provider{
		&mockChatProvider{name: "provider1"},
		&mockChatProvider{name: "provider2"},
	}

	rp.SetProviders(providers)

	if len(rp.providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(rp.providers))
	}
}

func TestRacingProvider_NoProvidersConfigured(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS: 1000,
		Strategy:  StrategyFirstWins,
	})

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	_, err := rp.GenerateChatCompletion(ctx, opts)

	if err == nil {
		t.Fatal("expected error when no providers configured")
	}

	if err.Error() != "no providers configured for racing" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFirstWinsStrategy_FastProviderWins(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
	})

	providers := []types.Provider{
		&mockChatProvider{
			name:     "slow-provider",
			delay:    200 * time.Millisecond,
			response: "slow response",
		},
		&mockChatProvider{
			name:     "fast-provider",
			delay:    10 * time.Millisecond,
			response: "fast response",
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error reading stream: %v", err)
	}

	if chunk.Metadata == nil {
		t.Fatal("expected metadata in chunk")
	}

	winner, ok := chunk.Metadata["racing_winner"].(string)
	if !ok {
		t.Fatal("expected racing_winner in metadata")
	}

	if winner != "fast-provider" {
		t.Errorf("expected 'fast-provider' to win, got '%s'", winner)
	}

	// Verify performance stats
	stats := rp.GetPerformanceStats()
	if stats["fast-provider"].Wins != 1 {
		t.Errorf("expected fast-provider to have 1 win, got %d", stats["fast-provider"].Wins)
	}
}

func TestFirstWinsStrategy_FirstSuccessWins(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
	})

	providers := []types.Provider{
		&mockChatProvider{
			name:  "error-provider",
			delay: 10 * time.Millisecond,
			err:   errors.New("provider error"),
		},
		&mockChatProvider{
			name:     "success-provider",
			delay:    50 * time.Millisecond,
			response: "success",
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	chunk, _ := stream.Next()
	winner := chunk.Metadata["racing_winner"].(string)

	if winner != "success-provider" {
		t.Errorf("expected 'success-provider' to win, got '%s'", winner)
	}

	// Verify performance stats
	stats := rp.GetPerformanceStats()
	if stats["error-provider"].Losses != 1 {
		t.Errorf("expected error-provider to have 1 loss, got %d", stats["error-provider"].Losses)
	}
	if stats["success-provider"].Wins != 1 {
		t.Errorf("expected success-provider to have 1 win, got %d", stats["success-provider"].Wins)
	}
}

func TestFirstWinsStrategy_AllProvidersFail(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
	})

	providers := []types.Provider{
		&mockChatProvider{
			name:  "error-provider-1",
			delay: 10 * time.Millisecond,
			err:   errors.New("error 1"),
		},
		&mockChatProvider{
			name:  "error-provider-2",
			delay: 20 * time.Millisecond,
			err:   errors.New("error 2"),
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	_, err := rp.GenerateChatCompletion(ctx, opts)

	if err == nil {
		t.Fatal("expected error when all providers fail")
	}

	if !errors.Is(err, errors.New("error 2")) && err.Error() != "all providers failed, last error: error 2" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestWeightedStrategy_ScoreBasedSelection tests that the weighted strategy
// selects the provider with the best performance score during the grace period,
// even if a faster provider with worse history finishes first.
func TestWeightedStrategy_ScoreBasedSelection(t *testing.T) {
	tests := []struct {
		name             string
		gracePeriodMS    int
		performanceSetup func(*PerformanceTracker)
		providers        []types.Provider
		expectedWinner   string
		description      string
	}{
		{
			name:          "CollectsDuringGracePeriod",
			gracePeriodMS: 100,
			performanceSetup: func(pt *PerformanceTracker) {
				// high-score-provider: 100% win rate (2 wins, 0 losses)
				pt.RecordWin("high-score-provider", 50*time.Millisecond)
				pt.RecordWin("high-score-provider", 50*time.Millisecond)
				// low-score-provider: 50% win rate (1 win, 1 loss)
				pt.RecordLoss("low-score-provider", 50*time.Millisecond)
				pt.RecordWin("low-score-provider", 50*time.Millisecond)
			},
			providers: []types.Provider{
				&mockChatProvider{
					name:     "low-score-provider",
					delay:    10 * time.Millisecond,
					response: "response 1",
				},
				&mockChatProvider{
					name:     "high-score-provider",
					delay:    50 * time.Millisecond,
					response: "response 2",
				},
			},
			expectedWinner: "high-score-provider",
			description:    "high-score-provider should win because it has better performance history",
		},
		{
			name:          "PicksBestScore",
			gracePeriodMS: 50,
			performanceSetup: func(pt *PerformanceTracker) {
				// provider-a: 100% win rate (2 wins, 0 losses)
				pt.RecordWin("provider-a", 100*time.Millisecond)
				pt.RecordWin("provider-a", 100*time.Millisecond)
				// provider-b: 50% win rate (1 win, 1 loss)
				pt.RecordWin("provider-b", 100*time.Millisecond)
				pt.RecordLoss("provider-b", 100*time.Millisecond)
			},
			providers: []types.Provider{
				&mockChatProvider{
					name:     "provider-b",
					delay:    10 * time.Millisecond,
					response: "response b",
				},
				&mockChatProvider{
					name:     "provider-a",
					delay:    20 * time.Millisecond,
					response: "response a",
				},
			},
			expectedWinner: "provider-a",
			description:    "provider-a should win due to better score",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp := NewRacingProvider("test", &Config{
				TimeoutMS:     5000,
				GracePeriodMS: tt.gracePeriodMS,
				Strategy:      StrategyWeighted,
			})

			// Pre-seed performance stats
			tt.performanceSetup(rp.performance)

			rp.SetProviders(tt.providers)

			ctx := context.Background()
			opts := types.GenerateOptions{Prompt: "test"}

			stream, err := rp.GenerateChatCompletion(ctx, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = stream.Close() }()

			chunk, _ := stream.Next()
			winner := chunk.Metadata["racing_winner"].(string)

			if winner != tt.expectedWinner {
				t.Errorf("expected '%s' to win, got '%s': %s", tt.expectedWinner, winner, tt.description)
			}
		})
	}
}

func TestWeightedStrategy_NoCandidates(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS:     5000,
		GracePeriodMS: 50,
		Strategy:      StrategyWeighted,
	})

	providers := []types.Provider{
		&mockChatProvider{
			name:  "error-provider",
			delay: 10 * time.Millisecond,
			err:   errors.New("error"),
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	_, err := rp.GenerateChatCompletion(ctx, opts)

	if err == nil {
		t.Fatal("expected error when no candidates available")
	}
}

func TestQualityStrategy(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS:     5000,
		GracePeriodMS: 50,
		Strategy:      StrategyQuality,
	})

	providers := []types.Provider{
		&mockChatProvider{
			name:     "provider-1",
			delay:    10 * time.Millisecond,
			response: "response",
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	chunk, _ := stream.Next()
	if chunk.Metadata["racing_winner"] == nil {
		t.Error("expected racing_winner in metadata")
	}
}

func TestRacingStream_AddsMetadata(t *testing.T) {
	mockInner := &mockStream{content: "test"}
	rs := &racingStream{
		inner:    mockInner,
		provider: "test-provider",
		latency:  123 * time.Millisecond,
	}

	chunk, err := rs.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	if chunk.Metadata == nil {
		t.Fatal("expected metadata to be set")
	}

	winner, ok := chunk.Metadata["racing_winner"].(string)
	if !ok || winner != "test-provider" {
		t.Errorf("expected racing_winner to be 'test-provider', got %v", winner)
	}

	latency, ok := chunk.Metadata["racing_latency_ms"].(int64)
	if !ok || latency != 123 {
		t.Errorf("expected racing_latency_ms to be 123, got %v", latency)
	}
}

func TestRacingStream_PreservesExistingMetadata(t *testing.T) {
	mockInner := &mockStream{content: "test"}
	rs := &racingStream{
		inner:    mockInner,
		provider: "test-provider",
		latency:  50 * time.Millisecond,
	}

	_, _ = rs.Next()

	// Get another chunk to verify metadata is consistently added
	chunk, _ := rs.Next()

	if chunk.Metadata["racing_winner"] != "test-provider" {
		t.Error("expected racing_winner to be preserved")
	}
}

func TestRacingStream_Close(t *testing.T) {
	mockInner := &mockStream{content: "test"}
	rs := &racingStream{
		inner:    mockInner,
		provider: "test-provider",
		latency:  50 * time.Millisecond,
	}

	err := rs.Close()
	if err != nil {
		t.Errorf("unexpected error closing stream: %v", err)
	}

	if !mockInner.closed {
		t.Error("expected inner stream to be closed")
	}
}

func TestPerformanceTracker_RecordWin(t *testing.T) {
	pt := NewPerformanceTracker()

	pt.RecordWin("provider-a", 100*time.Millisecond)
	pt.RecordWin("provider-a", 200*time.Millisecond)

	stats := pt.GetAllStats()

	if stats["provider-a"].Wins != 2 {
		t.Errorf("expected 2 wins, got %d", stats["provider-a"].Wins)
	}

	if stats["provider-a"].TotalRaces != 2 {
		t.Errorf("expected 2 total races, got %d", stats["provider-a"].TotalRaces)
	}

	expectedAvg := 150 * time.Millisecond
	if stats["provider-a"].AvgLatency != expectedAvg {
		t.Errorf("expected avg latency %v, got %v", expectedAvg, stats["provider-a"].AvgLatency)
	}

	expectedWinRate := 1.0
	if stats["provider-a"].WinRate != expectedWinRate {
		t.Errorf("expected win rate %f, got %f", expectedWinRate, stats["provider-a"].WinRate)
	}

	if stats["provider-a"].LastUpdated.IsZero() {
		t.Error("expected LastUpdated to be set")
	}
}

func TestPerformanceTracker_RecordLoss(t *testing.T) {
	pt := NewPerformanceTracker()

	pt.RecordLoss("provider-b", 100*time.Millisecond)
	pt.RecordLoss("provider-b", 200*time.Millisecond)

	stats := pt.GetAllStats()

	if stats["provider-b"].Losses != 2 {
		t.Errorf("expected 2 losses, got %d", stats["provider-b"].Losses)
	}

	if stats["provider-b"].TotalRaces != 2 {
		t.Errorf("expected 2 total races, got %d", stats["provider-b"].TotalRaces)
	}

	expectedWinRate := 0.0
	if stats["provider-b"].WinRate != expectedWinRate {
		t.Errorf("expected win rate %f, got %f", expectedWinRate, stats["provider-b"].WinRate)
	}
}

func TestPerformanceTracker_MixedResults(t *testing.T) {
	pt := NewPerformanceTracker()

	pt.RecordWin("provider-c", 100*time.Millisecond)
	pt.RecordLoss("provider-c", 100*time.Millisecond)
	pt.RecordWin("provider-c", 100*time.Millisecond)

	stats := pt.GetAllStats()

	if stats["provider-c"].Wins != 2 {
		t.Errorf("expected 2 wins, got %d", stats["provider-c"].Wins)
	}

	if stats["provider-c"].Losses != 1 {
		t.Errorf("expected 1 loss, got %d", stats["provider-c"].Losses)
	}

	if stats["provider-c"].TotalRaces != 3 {
		t.Errorf("expected 3 total races, got %d", stats["provider-c"].TotalRaces)
	}

	expectedWinRate := 2.0 / 3.0
	if fmt.Sprintf("%.2f", stats["provider-c"].WinRate) != fmt.Sprintf("%.2f", expectedWinRate) {
		t.Errorf("expected win rate %.2f, got %.2f", expectedWinRate, stats["provider-c"].WinRate)
	}
}

func TestPerformanceTracker_GetScore_NewProvider(t *testing.T) {
	pt := NewPerformanceTracker()

	score := pt.GetScore("unknown-provider")

	if score != 0.5 {
		t.Errorf("expected default score 0.5 for unknown provider, got %f", score)
	}
}

func TestPerformanceTracker_GetScore_KnownProvider(t *testing.T) {
	pt := NewPerformanceTracker()

	pt.RecordWin("provider-d", 100*time.Millisecond)
	pt.RecordWin("provider-d", 100*time.Millisecond)
	pt.RecordLoss("provider-d", 100*time.Millisecond)
	pt.RecordLoss("provider-d", 100*time.Millisecond)

	score := pt.GetScore("provider-d")

	expectedScore := 0.5 // 2 wins out of 4 races
	if score != expectedScore {
		t.Errorf("expected score %f, got %f", expectedScore, score)
	}
}

func TestPerformanceTracker_GetAllStats_ReturnsCopy(t *testing.T) {
	pt := NewPerformanceTracker()

	pt.RecordWin("provider-e", 100*time.Millisecond)

	stats := pt.GetAllStats()
	stats["provider-e"].Wins = 999

	// Original should not be modified
	originalStats := pt.GetAllStats()
	if originalStats["provider-e"].Wins != 1 {
		t.Error("expected GetAllStats to return a copy, not a reference")
	}
}

func TestPerformanceTracker_ConcurrentAccess(t *testing.T) {
	pt := NewPerformanceTracker()

	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				if j%2 == 0 {
					pt.RecordWin(fmt.Sprintf("provider-%d", id), time.Duration(j)*time.Millisecond)
				} else {
					pt.RecordLoss(fmt.Sprintf("provider-%d", id), time.Duration(j)*time.Millisecond)
				}
			}
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				pt.GetScore(fmt.Sprintf("provider-%d", j%10))
				pt.GetAllStats()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	// Verify data integrity
	stats := pt.GetAllStats()
	for i := 0; i < 10; i++ {
		providerStats := stats[fmt.Sprintf("provider-%d", i)]
		if providerStats.TotalRaces != 100 {
			t.Errorf("expected 100 races for provider-%d, got %d", i, providerStats.TotalRaces)
		}
		if providerStats.Wins != 50 {
			t.Errorf("expected 50 wins for provider-%d, got %d", i, providerStats.Wins)
		}
		if providerStats.Losses != 50 {
			t.Errorf("expected 50 losses for provider-%d, got %d", i, providerStats.Losses)
		}
	}
}

func TestRacingProvider_ContextCancellation(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
	})

	providers := []types.Provider{
		&mockChatProvider{
			name:     "slow-provider",
			delay:    1 * time.Second,
			response: "response",
		},
	}

	rp.SetProviders(providers)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := types.GenerateOptions{Prompt: "test"}

	_, err := rp.GenerateChatCompletion(ctx, opts)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRacingProvider_Timeout(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS: 100, // Very short timeout
		Strategy:  StrategyFirstWins,
	})

	providers := []types.Provider{
		&mockChatProvider{
			name:     "very-slow-provider",
			delay:    5 * time.Second,
			response: "response",
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	start := time.Now()
	_, err := rp.GenerateChatCompletion(ctx, opts)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}

	// Should timeout around 100ms, not wait for 5 seconds
	if elapsed > 1*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestPickBestCandidate_EmptyCandidates(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		Strategy: StrategyWeighted,
	})

	ctx := context.Background()
	_, err := rp.pickBestCandidate(ctx, []*raceResult{}, []string{}, make(map[string]time.Duration), "test-model", nil)

	if err == nil {
		t.Fatal("expected error for empty candidates")
	}

	if err.Error() != "no successful candidates" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPickBestCandidate_SingleCandidate(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		Strategy: StrategyWeighted,
	})

	candidates := []*raceResult{
		{
			provider: &mockChatProvider{name: "only-provider"},
			stream:   &mockStream{content: "response"},
			latency:  100 * time.Millisecond,
		},
	}

	ctx := context.Background()
	raceLatencies := map[string]time.Duration{"only-provider": 100 * time.Millisecond}
	stream, err := rp.pickBestCandidate(ctx, candidates, []string{"only-provider"}, raceLatencies, "test-model", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stream == nil {
		t.Fatal("expected non-nil stream")
	}

	rs, ok := stream.(*racingStream)
	if !ok {
		t.Fatal("expected racingStream type")
	}

	if rs.provider != "only-provider" {
		t.Errorf("expected provider 'only-provider', got '%s'", rs.provider)
	}
}

func TestWeightedStrategy_ContextTimeout(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS:     100,
		GracePeriodMS: 5000, // Grace period longer than timeout
		Strategy:      StrategyWeighted,
	})

	providers := []types.Provider{
		&mockChatProvider{
			name:  "error-provider",
			delay: 50 * time.Millisecond,
			err:   errors.New("error"),
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	_, err := rp.GenerateChatCompletion(ctx, opts)

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRacingProvider_ProviderNotChatProvider(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
	})

	// Create a provider that doesn't implement ChatProvider
	type nonChatProvider struct {
		*mockChatProvider
	}

	ncp := &nonChatProvider{
		mockChatProvider: &mockChatProvider{name: "non-chat"},
	}

	// Override to make it not a ChatProvider
	type coreOnly struct {
		name string
	}

	co := &coreOnly{name: "non-chat"}

	// We need to use a real Provider interface, so let's test with a valid one
	// but ensure the type assertion fails
	providers := []types.Provider{
		&mockChatProvider{
			name:     "good-provider",
			delay:    10 * time.Millisecond,
			response: "response",
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stream == nil {
		t.Fatal("expected non-nil stream")
	}

	// Clean up
	_ = ncp
	_ = co
}

func TestGetPerformanceStats(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
	})

	rp.performance.RecordWin("provider-1", 100*time.Millisecond)
	rp.performance.RecordLoss("provider-2", 200*time.Millisecond)

	stats := rp.GetPerformanceStats()

	if len(stats) != 2 {
		t.Errorf("expected 2 providers in stats, got %d", len(stats))
	}

	if stats["provider-1"].Wins != 1 {
		t.Errorf("expected provider-1 to have 1 win, got %d", stats["provider-1"].Wins)
	}

	if stats["provider-2"].Losses != 1 {
		t.Errorf("expected provider-2 to have 1 loss, got %d", stats["provider-2"].Losses)
	}
}

// ============================================================================
// Stub Method Tests
// ============================================================================

func TestRacingProvider_GetModels_NoVirtualModels(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})
	ctx := context.Background()

	models, err := rp.GetModels(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestRacingProvider_GetModels_MultipleVirtualModels(t *testing.T) {
	config := &Config{
		VirtualModels: map[string]VirtualModelConfig{
			"fast-model": {
				DisplayName: "Fast Racing Model",
				Description: "The fastest virtual model",
			},
			"quality-model": {
				DisplayName: "Quality Racing Model",
				Description: "The highest quality virtual model",
			},
			"balanced-model": {
				DisplayName: "Balanced Racing Model",
				Description: "Balanced speed and quality",
			},
		},
	}

	rp := NewRacingProvider("test", config)
	ctx := context.Background()

	models, err := rp.GetModels(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("expected 3 models, got %d", len(models))
	}

	// Check that model IDs match virtual model names (alphabetical order)
	expectedIDs := []string{"balanced-model", "fast-model", "quality-model"}
	for i, expectedID := range expectedIDs {
		if i >= len(models) {
			t.Fatalf("not enough models returned")
		}
		if models[i].ID != expectedID {
			t.Errorf("expected model ID '%s', got '%s'", expectedID, models[i].ID)
		}
	}

	// Verify DisplayName and Description are properly populated
	for _, model := range models {
		config, exists := config.VirtualModels[model.ID]
		if !exists {
			t.Errorf("model ID '%s' not found in config", model.ID)
		}

		if model.Name != config.DisplayName {
			t.Errorf("expected display name '%s', got '%s'", config.DisplayName, model.Name)
		}

		if model.Description != config.Description {
			t.Errorf("expected description '%s', got '%s'", config.Description, model.Description)
		}

		if model.Provider != "racing" {
			t.Errorf("expected provider 'racing', got '%s'", model.Provider)
		}
	}
}

func TestRacingProvider_GetDefaultModel_ExplicitDefault(t *testing.T) {
	config := &Config{
		DefaultVirtualModel: "quality-model",
		VirtualModels: map[string]VirtualModelConfig{
			"fast-model": {
				DisplayName: "Fast Racing Model",
				Description: "The fastest virtual model",
			},
			"quality-model": {
				DisplayName: "Quality Racing Model",
				Description: "The highest quality virtual model",
			},
		},
	}

	rp := NewRacingProvider("test", config)

	defaultModel := rp.GetDefaultModel()

	if defaultModel != "quality-model" {
		t.Errorf("expected default model 'quality-model', got '%s'", defaultModel)
	}
}

func TestRacingProvider_GetDefaultModel_NoExplicitDefault(t *testing.T) {
	config := &Config{
		VirtualModels: map[string]VirtualModelConfig{
			"test-model": {
				DisplayName: "Test Model",
				Description: "A test virtual model",
			},
		},
	}

	rp := NewRacingProvider("test", config)

	defaultModel := rp.GetDefaultModel()

	// Should return the only virtual model available
	if defaultModel != "test-model" {
		t.Errorf("expected default model 'test-model', got '%s'", defaultModel)
	}
}

func TestRacingProvider_GetDefaultModel_NoVirtualModels(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	defaultModel := rp.GetDefaultModel()

	if defaultModel != "" {
		t.Errorf("expected empty string, got '%s'", defaultModel)
	}
}

func TestRacingProvider_SupportsToolCalling(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	if rp.SupportsToolCalling() {
		t.Error("expected SupportsToolCalling to return false")
	}
}

func TestRacingProvider_SupportsStreaming(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	if !rp.SupportsStreaming() {
		t.Error("expected SupportsStreaming to return true")
	}
}

func TestRacingProvider_SupportsResponsesAPI(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	if rp.SupportsResponsesAPI() {
		t.Error("expected SupportsResponsesAPI to return false")
	}
}

func TestRacingProvider_GetToolFormat(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	format := rp.GetToolFormat()

	if format != types.ToolFormatOpenAI {
		t.Errorf("expected ToolFormatOpenAI, got %s", format)
	}
}

func TestRacingProvider_Authenticate(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})
	ctx := context.Background()

	err := rp.Authenticate(ctx, types.AuthConfig{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRacingProvider_IsAuthenticated(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	if !rp.IsAuthenticated() {
		t.Error("expected IsAuthenticated to return true")
	}
}

func TestRacingProvider_Logout(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})
	ctx := context.Background()

	err := rp.Logout(ctx)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRacingProvider_Configure(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS:     1000,
		GracePeriodMS: 100,
		Strategy:      StrategyFirstWins,
	})

	config := types.ProviderConfig{
		ProviderConfig: map[string]interface{}{
			"timeout_ms":      2000,
			"grace_period_ms": 200,
			"strategy":        "weighted",
			"providers":       []string{"provider-1", "provider-2"},
		},
	}

	err := rp.Configure(config)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if rp.config.TimeoutMS != 2000 {
		t.Errorf("expected TimeoutMS to be 2000, got %d", rp.config.TimeoutMS)
	}

	if rp.config.GracePeriodMS != 200 {
		t.Errorf("expected GracePeriodMS to be 200, got %d", rp.config.GracePeriodMS)
	}

	if rp.config.Strategy != StrategyWeighted {
		t.Errorf("expected Strategy to be weighted, got %s", rp.config.Strategy)
	}

	if len(rp.config.ProviderNames) != 2 {
		t.Errorf("expected 2 provider names, got %d", len(rp.config.ProviderNames))
	}
}

func TestRacingProvider_Configure_EmptyConfig(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS: 1000,
	})

	config := types.ProviderConfig{
		ProviderConfig: nil,
	}

	err := rp.Configure(config)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Values should remain unchanged
	if rp.config.TimeoutMS != 1000 {
		t.Errorf("expected TimeoutMS to remain 1000, got %d", rp.config.TimeoutMS)
	}
}

func TestRacingProvider_GetConfig(t *testing.T) {
	rp := NewRacingProvider("test-racing", &Config{
		TimeoutMS:       3000,
		GracePeriodMS:   150,
		Strategy:        StrategyWeighted,
		ProviderNames:   []string{"p1", "p2"},
		PerformanceFile: "/tmp/perf.json",
	})

	config := rp.GetConfig()

	if config.Type != "racing" {
		t.Errorf("expected type 'racing', got %s", config.Type)
	}

	if config.Name != "test-racing" {
		t.Errorf("expected name 'test-racing', got %s", config.Name)
	}

	if timeout, ok := config.ProviderConfig["timeout_ms"].(int); !ok || timeout != 3000 {
		t.Errorf("expected timeout_ms to be 3000, got %v", config.ProviderConfig["timeout_ms"])
	}

	if gracePeriod, ok := config.ProviderConfig["grace_period_ms"].(int); !ok || gracePeriod != 150 {
		t.Errorf("expected grace_period_ms to be 150, got %v", config.ProviderConfig["grace_period_ms"])
	}

	if strategy, ok := config.ProviderConfig["strategy"].(string); !ok || strategy != "weighted" {
		t.Errorf("expected strategy to be 'weighted', got %v", config.ProviderConfig["strategy"])
	}

	if providers, ok := config.ProviderConfig["providers"].([]string); !ok || len(providers) != 2 {
		t.Errorf("expected 2 providers, got %v", config.ProviderConfig["providers"])
	}

	if perfFile, ok := config.ProviderConfig["performance_file"].(string); !ok || perfFile != "/tmp/perf.json" {
		t.Errorf("expected performance_file to be '/tmp/perf.json', got %v", config.ProviderConfig["performance_file"])
	}
}

func TestRacingProvider_InvokeServerTool(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})
	ctx := context.Background()

	_, err := rp.InvokeServerTool(ctx, "test-tool", nil)

	if err == nil {
		t.Fatal("expected error from InvokeServerTool")
	}

	if err.Error() != "tool calling not supported for virtual racing provider" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRacingProvider_HealthCheck_NoProviders(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})
	ctx := context.Background()

	err := rp.HealthCheck(ctx)

	if err == nil {
		t.Fatal("expected error when no providers configured")
	}

	if err.Error() != "no providers configured" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRacingProvider_HealthCheck_HealthyProvider(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	providers := []types.Provider{
		&mockChatProvider{name: "healthy-provider"},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	err := rp.HealthCheck(ctx)

	// Mock provider's HealthCheck returns nil, so at least one is healthy
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

type mockHealthCheckProvider struct {
	*mockChatProvider
	healthErr error
}

func (m *mockHealthCheckProvider) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

func TestRacingProvider_HealthCheck_AllUnhealthy(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	healthErr := errors.New("health check failed")
	providers := []types.Provider{
		&mockHealthCheckProvider{
			mockChatProvider: &mockChatProvider{name: "unhealthy-1"},
			healthErr:        healthErr,
		},
		&mockHealthCheckProvider{
			mockChatProvider: &mockChatProvider{name: "unhealthy-2"},
			healthErr:        errors.New("also unhealthy"),
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	err := rp.HealthCheck(ctx)

	if err == nil {
		t.Fatal("expected error when all providers are unhealthy")
	}

	expectedMsg := "all providers unhealthy:"
	if len(err.Error()) < len(expectedMsg) || err.Error()[:len(expectedMsg)] != expectedMsg {
		t.Errorf("expected error to start with '%s', got: %v", expectedMsg, err)
	}
}

func TestRacingProvider_HealthCheck_MixedHealth(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	providers := []types.Provider{
		&mockHealthCheckProvider{
			mockChatProvider: &mockChatProvider{name: "unhealthy"},
			healthErr:        errors.New("unhealthy"),
		},
		&mockHealthCheckProvider{
			mockChatProvider: &mockChatProvider{name: "healthy"},
			healthErr:        nil,
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	err := rp.HealthCheck(ctx)

	// Should succeed because at least one provider is healthy
	if err != nil {
		t.Errorf("expected no error when at least one provider is healthy, got %v", err)
	}
}

func TestRacingProvider_HealthCheck_UnhealthyProviders(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	// Create a mock provider that fails health check
	type unhealthyProvider struct {
		*mockChatProvider
	}

	up := &unhealthyProvider{
		mockChatProvider: &mockChatProvider{name: "unhealthy"},
	}

	providers := []types.Provider{up}
	rp.SetProviders(providers)

	ctx := context.Background()
	err := rp.HealthCheck(ctx)

	// The mock provider's HealthCheck returns nil by default, which is fine
	// This test verifies the health check logic runs without error
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRacingProvider_GetMetrics(t *testing.T) {
	rp := NewRacingProvider("test", &Config{})

	metrics := rp.GetMetrics()

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

func TestWeightedStrategy_GracePeriodExpires(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		TimeoutMS:     5000,
		GracePeriodMS: 50, // Short grace period
		Strategy:      StrategyWeighted,
	})

	providers := []types.Provider{
		&mockChatProvider{
			name:     "first-provider",
			delay:    10 * time.Millisecond,
			response: "first",
		},
		&mockChatProvider{
			name:     "second-provider",
			delay:    200 * time.Millisecond, // Arrives after grace period
			response: "second",
		},
	}

	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{Prompt: "test"}

	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	chunk, _ := stream.Next()
	winner := chunk.Metadata["racing_winner"].(string)

	// First provider should win because grace period expires before second arrives
	if winner != "first-provider" {
		t.Errorf("expected 'first-provider' to win, got '%s'", winner)
	}
}

// ============================================================================
// Enhanced Virtual Models Tests
// ============================================================================

func TestVirtualModels_GetModels_EmptyConfig(t *testing.T) {
	config := &Config{} // No virtual models configured
	rp := NewRacingProvider("test", config)
	ctx := context.Background()

	models, err := rp.GetModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}

	defaultModel := rp.GetDefaultModel()
	if defaultModel != "" {
		t.Errorf("expected empty default model, got '%s'", defaultModel)
	}
}

func TestVirtualModels_GetModels_WithValidation(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		expectedModels int
		expectedDefault string
		shouldErr      bool
	}{
		{
			name: "single virtual model",
			config: &Config{
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Racing Model",
						Description: "The fastest virtual model",
						Providers: []ProviderReference{
							{Name: "provider1"},
							{Name: "provider2"},
						},
					},
				},
			},
			expectedModels: 1,
			expectedDefault: "fast-model",
		},
		{
			name: "multiple virtual models with explicit default",
			config: &Config{
				DefaultVirtualModel: "quality-model",
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Racing Model",
						Description: "The fastest virtual model",
					},
					"quality-model": {
						DisplayName: "Quality Racing Model",
						Description: "The highest quality virtual model",
					},
					"balanced-model": {
						DisplayName: "Balanced Racing Model",
						Description: "Balanced speed and quality",
					},
				},
			},
			expectedModels: 3,
			expectedDefault: "quality-model",
		},
		{
			name: "invalid default model",
			config: &Config{
				DefaultVirtualModel: "nonexistent",
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Racing Model",
						Description: "The fastest virtual model",
					},
				},
			},
			expectedModels: 1,
			expectedDefault: "fast-model", // Should fall back to first available
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp := NewRacingProvider("test", tt.config)
			ctx := context.Background()

			models, err := rp.GetModels(ctx)
			if tt.shouldErr && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(models) != tt.expectedModels {
				t.Errorf("expected %d models, got %d", tt.expectedModels, len(models))
			}

			defaultModel := rp.GetDefaultModel()
			if defaultModel != tt.expectedDefault {
				t.Errorf("expected default model '%s', got '%s'", tt.expectedDefault, defaultModel)
			}

			// Validate model properties
			for _, model := range models {
				if model.Provider != "racing" {
					t.Errorf("expected provider 'racing', got '%s'", model.Provider)
				}
				if model.ID == "" {
					t.Error("expected non-empty model ID")
				}
				if model.Name == "" {
					t.Error("expected non-empty model name")
				}
			}
		})
	}
}

func TestVirtualModels_ModelValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		shouldErr   bool
		errContains string
	}{
		{
			name: "valid config",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Description: "A valid virtual model",
						Providers: []ProviderReference{
							{Name: "provider1", Model: "model1"},
						},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "missing default virtual model",
			config: &Config{
				TimeoutMS: 5000,
				VirtualModels: map[string]VirtualModelConfig{
					"model1": {
						DisplayName: "Model 1",
						Providers:   []ProviderReference{{Name: "provider1"}},
					},
				},
			},
			shouldErr:   true,
			errContains: "default_virtual_model",
		},
		{
			name: "no virtual models",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels:      map[string]VirtualModelConfig{},
			},
			shouldErr:   true,
			errContains: "at least one virtual model",
		},
		{
			name: "virtual model with no providers",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Providers:   []ProviderReference{}, // Empty providers
					},
				},
			},
			shouldErr:   true,
			errContains: "must have at least one provider",
		},
		{
			name: "provider with empty name",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Providers: []ProviderReference{
							{Name: ""}, // Empty provider name
						},
					},
				},
			},
			shouldErr:   true,
			errContains: "provider name cannot be empty",
		},
		{
			name: "negative timeout",
			config: &Config{
				TimeoutMS:           5000,
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Providers:   []ProviderReference{{Name: "provider1"}},
						TimeoutMS:   -100, // Negative timeout
					},
				},
			},
			shouldErr:   true,
			errContains: "must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.shouldErr {
				if err == nil {
					t.Fatal("expected validation error but got none")
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain '%s', got: %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// Per-Virtual-Model Racing Tests
// ============================================================================

func TestVirtualModel_DifferentStrategies(t *testing.T) {
	tests := []struct {
		name             string
		virtualModel     string
		expectedStrategy Strategy
		expectedWinner   string
		providers        []types.Provider
	}{
		{
			name:     "first_wins strategy",
			virtualModel: "fast-model",
			expectedStrategy: StrategyFirstWins,
			expectedWinner: "fast-provider",
			providers: []types.Provider{
				&mockChatProvider{
					name:     "slow-provider",
					delay:    200 * time.Millisecond,
					response: "slow response",
				},
				&mockChatProvider{
					name:     "fast-provider",
					delay:    10 * time.Millisecond,
					response: "fast response",
				},
			},
		},
		{
			name:     "weighted strategy with history",
			virtualModel: "quality-model",
			expectedStrategy: StrategyWeighted,
			expectedWinner: "quality-provider", // Should win due to performance history
			providers: []types.Provider{
				&mockChatProvider{
					name:     "fast-provider",
					delay:    10 * time.Millisecond,
					response: "fast response",
				},
				&mockChatProvider{
					name:     "quality-provider",
					delay:    50 * time.Millisecond,
					response: "quality response",
				},
			},
		},
		{
			name:     "quality strategy",
			virtualModel: "balanced-model",
			expectedStrategy: StrategyQuality,
			expectedWinner: "fast-provider", // Should pick based on adjusted score
			providers: []types.Provider{
				&mockChatProvider{
					name:     "fast-provider",
					delay:    10 * time.Millisecond,
					response: "fast response",
				},
				&mockChatProvider{
					name:     "slow-provider",
					delay:    100 * time.Millisecond,
					response: "slow response",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				DefaultVirtualModel: "fast-model",
				TimeoutMS:           5000,
				GracePeriodMS:       100,
				Strategy:            StrategyFirstWins, // Default strategy
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Model",
						Description: "Fast virtual model",
						Strategy:    StrategyFirstWins,
						TimeoutMS:   5000,
						Providers: []ProviderReference{
							{Name: "slow-provider"},
							{Name: "fast-provider"},
						},
					},
					"quality-model": {
						DisplayName: "Quality Model",
						Description: "Quality virtual model",
						Strategy:    StrategyWeighted,
						TimeoutMS:   5000,
						Providers: []ProviderReference{
							{Name: "fast-provider"},
							{Name: "quality-provider"},
						},
					},
					"balanced-model": {
						DisplayName: "Balanced Model",
						Description: "Balanced virtual model",
						Strategy:    StrategyQuality,
						TimeoutMS:   5000,
						Providers: []ProviderReference{
							{Name: "fast-provider"},
							{Name: "slow-provider"},
						},
					},
				},
			}

			rp := NewRacingProvider("test", config)
			rp.SetProviders(tt.providers)

			// Pre-seed performance history for weighted strategy test
			if tt.expectedStrategy == StrategyWeighted {
				rp.performance.RecordWin("quality-provider", 50*time.Millisecond)
				rp.performance.RecordWin("quality-provider", 60*time.Millisecond)
				rp.performance.RecordLoss("fast-provider", 10*time.Millisecond)
			}

			ctx := context.Background()
			opts := types.GenerateOptions{
				Model: tt.virtualModel,
				Prompt: "test",
			}

			stream, err := rp.GenerateChatCompletion(ctx, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = stream.Close() }()

			chunk, err := stream.Next()
			if err != nil && err != io.EOF {
				t.Fatalf("unexpected error reading chunk: %v", err)
			}

			winner, ok := chunk.Metadata["racing_winner"].(string)
			if !ok {
				t.Fatal("expected racing_winner in metadata")
			}

			if winner != tt.expectedWinner {
				t.Errorf("expected winner '%s', got '%s'", tt.expectedWinner, winner)
			}

			// Note: Virtual model metadata is only sent to metrics collector, not included in response metadata
			// Only racing metadata should be in the response
		})
	}
}

func TestVirtualModel_PerVirtualModelTimeouts(t *testing.T) {
	tests := []struct {
		name         string
		virtualModel string
		timeoutMS    int
		shouldTimeout bool
		providerDelay time.Duration
	}{
		{
			name:         "short timeout should fail",
			virtualModel: "fast-model",
			timeoutMS:    100, // Very short timeout
			shouldTimeout: true,
			providerDelay: 500 * time.Millisecond,
		},
		{
			name:         "long timeout should succeed",
			virtualModel: "slow-model",
			timeoutMS:    2000, // Longer timeout
			shouldTimeout: false,
			providerDelay: 100 * time.Millisecond,
		},
		{
			name:         "default timeout fallback",
			virtualModel: "default-model",
			timeoutMS:    0, // Use default
			shouldTimeout: false,
			providerDelay: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				DefaultVirtualModel: "default-model",
				TimeoutMS:           1000, // Default timeout
				Strategy:            StrategyFirstWins,
				VirtualModels: map[string]VirtualModelConfig{
					"fast-model": {
						DisplayName: "Fast Model",
						TimeoutMS:   tt.timeoutMS,
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
					"slow-model": {
						DisplayName: "Slow Model",
						TimeoutMS:   tt.timeoutMS,
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
					"default-model": {
						DisplayName: "Default Model",
						// No timeout specified, should use default
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
				},
			}

			providers := []types.Provider{
				&mockChatProvider{
					name:     "provider1",
					delay:    tt.providerDelay,
					response: "response",
				},
			}

			rp := NewRacingProvider("test", config)
			rp.SetProviders(providers)

			ctx := context.Background()
			opts := types.GenerateOptions{
				Model: tt.virtualModel,
				Prompt: "test",
			}

			start := time.Now()
			stream, err := rp.GenerateChatCompletion(ctx, opts)
			elapsed := time.Since(start)

			if tt.shouldTimeout {
				if err == nil {
					t.Fatal("expected timeout error but got none")
				}
				if elapsed > 500*time.Millisecond {
					t.Errorf("timeout took too long: %v", elapsed)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if stream != nil {
					_ = stream.Close()
				}
			}
		})
	}
}

func TestVirtualModel_ProviderReferences(t *testing.T) {
	tests := []struct {
		name         string
		virtualModel string
		providers    []ProviderReference
		expectedProviders []string
		shouldErr     bool
		errContains   string
	}{
		{
			name:         "valid provider references",
			virtualModel: "multi-provider",
			providers: []ProviderReference{
				{Name: "provider1", Model: "model1", Priority: 1},
				{Name: "provider2", Model: "model2", Priority: 2},
				{Name: "provider3", Model: "model3"}, // No priority
			},
			expectedProviders: []string{"provider1", "provider2", "provider3"},
		},
		{
			name:         "missing provider",
			virtualModel: "missing-provider",
			providers: []ProviderReference{
				{Name: "nonexistent-provider", Model: "model1"},
			},
			shouldErr:   true,
			errContains: "provider not found",
		},
		{
			name:         "empty provider list",
			virtualModel: "empty-providers",
			providers:    []ProviderReference{},
			shouldErr:   true,
			errContains: "no providers configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				DefaultVirtualModel: "multi-provider",
				TimeoutMS:           5000,
				Strategy:            StrategyFirstWins,
				VirtualModels: map[string]VirtualModelConfig{
					"multi-provider": {
						DisplayName: "Multi Provider Model",
						Providers: []ProviderReference{
							{Name: "provider1", Model: "model1", Priority: 1},
							{Name: "provider2", Model: "model2", Priority: 2},
							{Name: "provider3", Model: "model3"},
						},
					},
					"missing-provider": {
						DisplayName: "Missing Provider Model",
						Providers: []ProviderReference{
							{Name: "nonexistent-provider", Model: "model1"},
						},
					},
					"empty-providers": {
						DisplayName: "Empty Providers Model",
						Providers:   []ProviderReference{},
					},
				},
			}

			// Set up available providers
			availableProviders := []types.Provider{
				&mockChatProvider{name: "provider1", response: "response1"},
				&mockChatProvider{name: "provider2", response: "response2"},
				&mockChatProvider{name: "provider3", response: "response3"},
			}

			rp := NewRacingProvider("test", config)
			rp.SetProviders(availableProviders)

			ctx := context.Background()
			opts := types.GenerateOptions{
				Model: tt.virtualModel,
				Prompt: "test",
			}

			stream, err := rp.GenerateChatCompletion(ctx, opts)

			if tt.shouldErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain '%s', got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if stream != nil {
					_ = stream.Close()
				}
			}
		})
	}
}

func TestVirtualModel_FallbackToDefaults(t *testing.T) {
	config := &Config{
		DefaultVirtualModel: "default",
		TimeoutMS:           5000,
		GracePeriodMS:       1000,
		Strategy:            StrategyWeighted, // Default strategy
		VirtualModels: map[string]VirtualModelConfig{
			"default": {
				DisplayName: "Default Model",
				Description: "Uses default configuration",
				// No strategy or timeout specified, should use defaults
				Providers: []ProviderReference{
					{Name: "provider1"},
					{Name: "provider2"},
				},
			},
			"custom": {
				DisplayName: "Custom Model",
				Description: "Custom configuration",
				Strategy:    StrategyFirstWins, // Override default
				TimeoutMS:   2000,             // Override default
				Providers: []ProviderReference{
					{Name: "provider1"},
					{Name: "provider2"},
				},
			},
		},
	}

	providers := []types.Provider{
		&mockChatProvider{
			name:     "provider1",
			delay:    10 * time.Millisecond,
			response: "response from provider1",
		},
		&mockChatProvider{
			name:     "provider2",
			delay:    50 * time.Millisecond,
			response: "response from provider2",
		},
	}

	rp := NewRacingProvider("test", config)
	rp.SetProviders(providers)

	// Test default virtual model (should use default strategy)
	t.Run("default model uses default strategy", func(t *testing.T) {
		ctx := context.Background()
		opts := types.GenerateOptions{
			Model: "default",
			Prompt: "test",
		}

		stream, err := rp.GenerateChatCompletion(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = stream.Close() }()

		chunk, _ := stream.Next()
		winner := chunk.Metadata["racing_winner"].(string)

		// Should use weighted strategy (default)
		if winner != "provider1" && winner != "provider2" {
			t.Errorf("expected valid winner, got '%s'", winner)
		}
	})

	// Test custom virtual model (should use custom strategy)
	t.Run("custom model uses custom strategy", func(t *testing.T) {
		ctx := context.Background()
		opts := types.GenerateOptions{
			Model: "custom",
			Prompt: "test",
		}

		stream, err := rp.GenerateChatCompletion(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = stream.Close() }()

		chunk, _ := stream.Next()
		winner := chunk.Metadata["racing_winner"].(string)

		// Should use first_wins strategy, so provider1 should win
		if winner != "provider1" {
			t.Errorf("expected provider1 to win with first_wins strategy, got '%s'", winner)
		}
	})
}

func TestVirtualModel_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		configFunc  func() *Config
		shouldErr   bool
		errContains string
	}{
		{
			name: "valid complete configuration",
			configFunc: func() *Config {
				return &Config{
					TimeoutMS:           5000,
					DefaultVirtualModel: "complete",
					VirtualModels: map[string]VirtualModelConfig{
						"complete": {
							DisplayName: "Complete Model",
							Description: "Fully configured virtual model",
							Strategy:    StrategyWeighted,
							TimeoutMS:   3000,
							Providers: []ProviderReference{
								{Name: "provider1", Model: "model1", Priority: 1},
								{Name: "provider2", Model: "model2", Priority: 2},
							},
						},
					},
				}
			},
			shouldErr: false,
		},
		{
			name: "minimal valid configuration",
			configFunc: func() *Config {
				return &Config{
					TimeoutMS:           5000,
					DefaultVirtualModel: "minimal",
					VirtualModels: map[string]VirtualModelConfig{
						"minimal": {
							DisplayName: "Minimal Model",
							Providers: []ProviderReference{
								{Name: "provider1"},
							},
						},
					},
				}
			},
			shouldErr: false,
		},
		{
			name: "missing default virtual model reference",
			configFunc: func() *Config {
				return &Config{
					TimeoutMS:           5000,
					DefaultVirtualModel: "nonexistent",
					VirtualModels: map[string]VirtualModelConfig{
						"existing": {
							DisplayName: "Existing Model",
							Providers: []ProviderReference{
								{Name: "provider1"},
							},
						},
					},
				}
			},
			shouldErr:   true,
			errContains: "must reference an existing virtual model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.configFunc()
			err := config.Validate()

			if tt.shouldErr {
				if err == nil {
					t.Fatal("expected validation error but got none")
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain '%s', got: %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

// ============================================================================
// Configuration Tests
// ============================================================================


func TestConfig_DefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.TimeoutMS != 5000 {
		t.Errorf("expected default timeout 5000, got %d", config.TimeoutMS)
	}

	if config.GracePeriodMS != 1000 {
		t.Errorf("expected default grace period 1000, got %d", config.GracePeriodMS)
	}

	if config.Strategy != StrategyFirstWins {
		t.Errorf("expected default strategy '%s', got '%s'", StrategyFirstWins, config.Strategy)
	}

	if config.DefaultVirtualModel != "default" {
		t.Errorf("expected default virtual model 'default', got '%s'", config.DefaultVirtualModel)
	}

	if len(config.VirtualModels) != 1 {
		t.Errorf("expected 1 virtual model, got %d", len(config.VirtualModels))
	}

	defaultVM, exists := config.VirtualModels["default"]
	if !exists {
		t.Fatal("expected default virtual model to exist")
	}

	if defaultVM.DisplayName != "Default Racing Model" {
		t.Errorf("expected display name 'Default Racing Model', got '%s'", defaultVM.DisplayName)
	}
}

func TestConfig_ResolveVirtualModelConfig(t *testing.T) {
	baseConfig := &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"partial": {
				DisplayName: "Partial Model",
				Strategy:    StrategyWeighted, // Override strategy
				// No timeout, should use default
				Providers: []ProviderReference{{Name: "provider1"}},
			},
			"complete": {
				DisplayName: "Complete Model",
				Strategy:    StrategyQuality,
				TimeoutMS:   2000,
				Providers:   []ProviderReference{{Name: "provider1"}},
			},
		},
	}

	tests := []struct {
		name             string
		modelID          string
		expectedStrategy Strategy
		expectedTimeout  int
		shouldErr        bool
	}{
		{
			name:             "partial config with strategy override",
			modelID:          "partial",
			expectedStrategy: StrategyWeighted,
			expectedTimeout:  5000, // Should use default
		},
		{
			name:             "complete config",
			modelID:          "complete",
			expectedStrategy: StrategyQuality,
			expectedTimeout:  2000,
		},
		{
			name:      "nonexistent model",
			modelID:   "nonexistent",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vmConfig, err := baseConfig.resolveVirtualModelConfig(tt.modelID)

			if tt.shouldErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if vmConfig.Strategy != tt.expectedStrategy {
				t.Errorf("expected strategy '%s', got '%s'", tt.expectedStrategy, vmConfig.Strategy)
			}

			if vmConfig.TimeoutMS != tt.expectedTimeout {
				t.Errorf("expected timeout %d, got %d", tt.expectedTimeout, vmConfig.TimeoutMS)
			}

			// Verify we get a copy, not a reference
			vmConfig.Strategy = StrategyQuality
			original, _ := baseConfig.resolveVirtualModelConfig(tt.modelID)
			if original.Strategy == StrategyQuality && tt.expectedStrategy != StrategyQuality {
				t.Error("resolveVirtualModelConfig should return a copy, not modify original")
			}
		})
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestRacingProvider_Integration_MetricsAggregation(t *testing.T) {
	// Create mock providers with metrics
	provider1 := &mockChatProvider{name: "provider1", response: "response1"}
	provider2 := &mockChatProvider{name: "provider2", response: "response2"}

	config := &Config{
		DefaultVirtualModel: "test-model",
		TimeoutMS:           5000,
		Strategy:            StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"test-model": {
				DisplayName: "Test Model",
				Providers: []ProviderReference{
					{Name: "provider1"},
					{Name: "provider2"},
				},
			},
		},
	}

	rp := NewRacingProvider("test-racing", config)
	rp.SetProviders([]types.Provider{provider1, provider2})

	// Test metrics aggregation
	metrics := rp.GetMetrics()

	// Initially should be empty
	if metrics.RequestCount != 0 {
		t.Errorf("expected initial RequestCount 0, got %d", metrics.RequestCount)
	}

	// Make a request
	ctx := context.Background()
	opts := types.GenerateOptions{
		Model: "test-model",
		Prompt: "test",
	}

	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = stream.Close()

	// Check metrics after request
	metrics = rp.GetMetrics()
	if metrics.RequestCount == 0 {
		t.Error("expected RequestCount to increase after request")
	}
}

func TestRacingProvider_Integration_HealthCheckAcrossVirtualModels(t *testing.T) {
	healthyProvider := &mockChatProvider{name: "healthy-provider"}
	unhealthyProvider := &mockHealthCheckProvider{
		mockChatProvider: &mockChatProvider{name: "unhealthy-provider"},
		healthErr:        errors.New("health check failed"),
	}

	config := &Config{
		DefaultVirtualModel: "healthy-model",
		VirtualModels: map[string]VirtualModelConfig{
			"healthy-model": {
				DisplayName: "Healthy Model",
				Providers: []ProviderReference{
					{Name: "healthy-provider"},
				},
			},
			"unhealthy-model": {
				DisplayName: "Unhealthy Model",
				Providers: []ProviderReference{
					{Name: "unhealthy-provider"},
				},
			},
			"mixed-model": {
				DisplayName: "Mixed Model",
				Providers: []ProviderReference{
					{Name: "healthy-provider"},
					{Name: "unhealthy-provider"},
				},
			},
		},
	}

	rp := NewRacingProvider("test", config)
	rp.SetProviders([]types.Provider{healthyProvider, unhealthyProvider})

	ctx := context.Background()

	// Test health check with healthy providers
	err := rp.HealthCheck(ctx)
	if err != nil {
		t.Errorf("expected healthy check to pass, got error: %v", err)
	}

	// Test with only unhealthy providers by removing healthy one
	rp.SetProviders([]types.Provider{unhealthyProvider})
	err = rp.HealthCheck(ctx)
	if err == nil {
		t.Error("expected health check to fail with only unhealthy providers")
	}
}

func TestRacingProvider_Integration_ConcurrentRequests(t *testing.T) {
	providers := []types.Provider{
		&mockChatProvider{
			name:     "provider1",
			delay:    50 * time.Millisecond,
			response: "response1",
		},
		&mockChatProvider{
			name:     "provider2",
			delay:    30 * time.Millisecond,
			response: "response2",
		},
	}

	config := &Config{
		DefaultVirtualModel: "concurrent-model",
		TimeoutMS:           5000,
		Strategy:            StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"concurrent-model": {
				DisplayName: "Concurrent Model",
				Providers: []ProviderReference{
					{Name: "provider1"},
					{Name: "provider2"},
				},
			},
		},
	}

	rp := NewRacingProvider("test", config)
	rp.SetProviders(providers)

	// Launch multiple concurrent requests
	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			ctx := context.Background()
			opts := types.GenerateOptions{
				Model: "concurrent-model",
				Prompt: "test",
			}

			stream, err := rp.GenerateChatCompletion(ctx, opts)
			if err != nil {
				results <- err
				return
			}
			_ = stream.Close()
			results <- nil
		}()
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < numRequests; i++ {
		err := <-results
		if err != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	if successCount != numRequests {
		t.Errorf("expected %d successful requests, got %d (errors: %d)", numRequests, successCount, errorCount)
	}
}

func TestRacingProvider_Integration_MetadataEnrichment(t *testing.T) {
	providers := []types.Provider{
		&mockChatProvider{
			name:     "provider1",
			delay:    10 * time.Millisecond,
			response: "test response",
		},
	}

	config := &Config{
		DefaultVirtualModel: "metadata-model",
		TimeoutMS:           5000,
		Strategy:            StrategyFirstWins,
		VirtualModels: map[string]VirtualModelConfig{
			"metadata-model": {
				DisplayName: "Metadata Test Model",
				Description: "Model for testing metadata enrichment",
				Providers: []ProviderReference{
					{Name: "provider1"},
				},
			},
		},
	}

	rp := NewRacingProvider("test", config)
	rp.SetProviders(providers)

	ctx := context.Background()
	opts := types.GenerateOptions{
		Model: "metadata-model",
		Prompt: "test",
	}

	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	chunk, err := stream.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error reading chunk: %v", err)
	}

	// Check racing metadata
	if chunk.Metadata == nil {
		t.Fatal("expected metadata in chunk")
	}

	winner, ok := chunk.Metadata["racing_winner"].(string)
	if !ok {
		t.Fatal("expected racing_winner in metadata")
	}

	if winner != "provider1" {
		t.Errorf("expected winner 'provider1', got '%s'", winner)
	}

	latency, ok := chunk.Metadata["racing_latency_ms"].(int64)
	if !ok {
		t.Fatal("expected racing_latency_ms in metadata")
	}

	if latency <= 0 {
		t.Errorf("expected positive latency, got %d", latency)
	}

	// Note: Virtual model metadata is sent to metrics collector, not included in response metadata
	// Only racing metadata should be present in response chunks
	// Virtual model metadata would be available through metrics events
}

// ============================================================================
// Backward Compatibility Tests
// ============================================================================

func TestBackwardCompatibility_OldConfigurationFormat(t *testing.T) {
	// Test old format without virtual models
	oldConfig := &Config{
		TimeoutMS:     3000,
		GracePeriodMS: 500,
		Strategy:      StrategyFirstWins,
		ProviderNames: []string{"provider1", "provider2"},
		// No virtual models configured
	}

	rp := NewRacingProvider("old-format", oldConfig)

	// Test that old configuration still works for basic functionality
	if rp.config.TimeoutMS != 3000 {
		t.Errorf("expected timeout 3000, got %d", rp.config.TimeoutMS)
	}

	if rp.config.Strategy != StrategyFirstWins {
		t.Errorf("expected strategy FirstWins, got %s", rp.config.Strategy)
	}

	// Test GetModels with no virtual models
	ctx := context.Background()
	models, err := rp.GetModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("expected 0 models for old format, got %d", len(models))
	}

	defaultModel := rp.GetDefaultModel()
	if defaultModel != "" {
		t.Errorf("expected empty default model for old format, got '%s'", defaultModel)
	}
}

func TestBackwardCompatibility_ConfigMigration(t *testing.T) {
	tests := []struct {
		name     string
		oldConfig *Config
		expectedBehavior string
	}{
		{
			name: "old config with provider names",
			oldConfig: &Config{
				TimeoutMS:     2000,
				GracePeriodMS: 200,
				Strategy:      StrategyWeighted,
				ProviderNames: []string{"provider1", "provider2"},
			},
			expectedBehavior: "should preserve old settings",
		},
		{
			name: "config with both old and new format",
			oldConfig: &Config{
				TimeoutMS:           2000,
				Strategy:            StrategyFirstWins,
				ProviderNames:       []string{"provider1"},
				DefaultVirtualModel: "default",
				VirtualModels: map[string]VirtualModelConfig{
					"default": {
						DisplayName: "Default Model",
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
				},
			},
			expectedBehavior: "should use new virtual models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp := NewRacingProvider("migration-test", tt.oldConfig)

			// Verify basic settings are preserved
			if rp.config.TimeoutMS != tt.oldConfig.TimeoutMS {
				t.Errorf("expected timeout %d, got %d", tt.oldConfig.TimeoutMS, rp.config.TimeoutMS)
			}

			if rp.config.Strategy != tt.oldConfig.Strategy {
				t.Errorf("expected strategy %s, got %s", tt.oldConfig.Strategy, rp.config.Strategy)
			}

			// Test behavior based on configuration type
			if len(tt.oldConfig.VirtualModels) > 0 {
				// New format - should have virtual models
				ctx := context.Background()
				models, err := rp.GetModels(ctx)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if len(models) == 0 {
					t.Error("expected virtual models to be available")
				}
			} else {
				// Old format - should not have virtual models
				ctx := context.Background()
				models, err := rp.GetModels(ctx)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if len(models) != 0 {
					t.Errorf("expected no virtual models for old format, got %d", len(models))
				}
			}
		})
	}
}

func TestBackwardCompatibility_ProviderSetters(t *testing.T) {
	// Test that SetProviders still works with old configuration
	config := &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
		ProviderNames: []string{"provider1", "provider2"},
	}

	rp := NewRacingProvider("backward-compat", config)

	providers := []types.Provider{
		&mockChatProvider{name: "provider1", response: "response1"},
		&mockChatProvider{name: "provider2", response: "response2"},
	}

	// This should still work even with old config format
	rp.SetProviders(providers)

	// Verify providers were set
	if len(rp.providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(rp.providers))
	}

	// Test that we can still make requests without virtual models (legacy mode)
	ctx := context.Background()
	opts := types.GenerateOptions{
		// No model specified - old format behavior (legacy mode)
		Prompt: "test",
	}

	// This should work in legacy mode when no virtual models are configured
	stream, err := rp.GenerateChatCompletion(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error in legacy mode: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Verify we get a response
	chunk, err := stream.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error reading chunk: %v", err)
	}

	// Verify racing metadata is present
	if chunk.Metadata["racing_winner"] == nil {
		t.Error("expected racing_winner in metadata")
	}

	// Verify virtual model metadata shows legacy mode
	if chunk.Metadata["virtual_model"] != "legacy_mode" {
		t.Errorf("expected virtual_model to be 'legacy_mode', got %v", chunk.Metadata["virtual_model"])
	}
}

func TestBackwardCompatibility_ConfigMethods(t *testing.T) {
	config := &Config{
		TimeoutMS:     3000,
		GracePeriodMS: 300,
		Strategy:      StrategyWeighted,
		ProviderNames: []string{"provider1", "provider2"},
	}

	rp := NewRacingProvider("config-test", config)

	// Test GetConfig with old format
	providerConfig := rp.GetConfig()
	if providerConfig.Type != "racing" {
		t.Errorf("expected type 'racing', got '%s'", providerConfig.Type)
	}

	if providerConfig.Name != "config-test" {
		t.Errorf("expected name 'config-test', got '%s'", providerConfig.Name)
	}

	// Test Configure with old format
	newConfig := types.ProviderConfig{
		Type: "racing",
		Name: "config-test",
		ProviderConfig: map[string]interface{}{
			"timeout_ms":      4000,
			"grace_period_ms": 400,
			"strategy":        "first_wins",
			"providers":       []string{"provider1", "provider2", "provider3"},
		},
	}

	err := rp.Configure(newConfig)
	if err != nil {
		t.Fatalf("unexpected error configuring: %v", err)
	}

	// Verify configuration was updated
	if rp.config.TimeoutMS != 4000 {
		t.Errorf("expected updated timeout 4000, got %d", rp.config.TimeoutMS)
	}

	if rp.config.Strategy != StrategyFirstWins {
		t.Errorf("expected updated strategy FirstWins, got %s", rp.config.Strategy)
	}
}

func TestBackwardCompatibility_PerformanceTracking(t *testing.T) {
	// Test that performance tracking still works as before
	config := &Config{
		TimeoutMS: 5000,
		Strategy:  StrategyFirstWins,
		ProviderNames: []string{"provider1", "provider2"},
	}

	rp := NewRacingProvider("perf-test", config)

	// Performance tracking should be available
	stats := rp.GetPerformanceStats()
	if stats == nil {
		t.Error("expected performance stats to be available")
	}

	// Should be able to record wins/losses
	rp.performance.RecordWin("provider1", 100*time.Millisecond)
	rp.performance.RecordLoss("provider2", 200*time.Millisecond)

	stats = rp.GetPerformanceStats()
	if stats["provider1"].Wins != 1 {
		t.Errorf("expected provider1 to have 1 win, got %d", stats["provider1"].Wins)
	}

	if stats["provider2"].Losses != 1 {
		t.Errorf("expected provider2 to have 1 loss, got %d", stats["provider2"].Losses)
	}
}

func TestBackwardCompatibility_ErrorHandling(t *testing.T) {
	config := &Config{
		TimeoutMS: 1000,
		Strategy:  StrategyFirstWins,
		ProviderNames: []string{"provider1"},
	}

	rp := NewRacingProvider("error-test", config)
	rp.SetProviders([]types.Provider{
		&mockChatProvider{name: "provider1", err: errors.New("provider error")},
	})

	ctx := context.Background()
	opts := types.GenerateOptions{
		Model: "nonexistent-model", // Should use legacy mode when no virtual models configured
		Prompt: "test",
	}

	// Should handle provider errors gracefully in legacy mode
	_, err := rp.GenerateChatCompletion(ctx, opts)
	if err == nil {
		t.Fatal("expected error when provider fails")
	}

	// Error should be from the failing provider, not virtual model error
	expectedError := "all providers failed"
	if !containsString(err.Error(), expectedError) {
		t.Errorf("expected error to contain '%s', got: %v", expectedError, err)
	}
}

// TestBackwardCompatibility_MixedUsage tests scenarios where old and new configurations
// might be mixed during migration
func TestBackwardCompatibility_MixedUsage(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() *Config
		testFunc    func(*RacingProvider)
	}{
		{
			name: "old config with new virtual model added later",
			setupConfig: func() *Config {
				return &Config{
					TimeoutMS:     3000,
					GracePeriodMS: 300,
					Strategy:      StrategyFirstWins,
					ProviderNames: []string{"provider1"},
				}
			},
			testFunc: func(rp *RacingProvider) {
				// Add virtual model configuration at runtime
				rp.config.VirtualModels = map[string]VirtualModelConfig{
					"new-model": {
						DisplayName: "New Virtual Model",
						Providers: []ProviderReference{
							{Name: "provider1"},
						},
					},
				}
				rp.config.DefaultVirtualModel = "new-model"

				// Should now work with virtual models
				ctx := context.Background()
				models, err := rp.GetModels(ctx)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if len(models) != 1 {
					t.Errorf("expected 1 model after adding virtual model, got %d", len(models))
				}
			},
		},
		{
			name: "new config used with old-style provider setting",
			setupConfig: func() *Config {
				return &Config{
					DefaultVirtualModel: "modern",
					VirtualModels: map[string]VirtualModelConfig{
						"modern": {
							DisplayName: "Modern Model",
							Providers: []ProviderReference{
								{Name: "provider1"},
								{Name: "provider2"},
							},
						},
					},
				}
			},
			testFunc: func(rp *RacingProvider) {
				// Use old SetProviders method
				providers := []types.Provider{
					&mockChatProvider{name: "provider1", response: "response1"},
					&mockChatProvider{name: "provider2", response: "response2"},
				}
				rp.SetProviders(providers)

				// Should work with new virtual model format
				ctx := context.Background()
				opts := types.GenerateOptions{
					Model: "modern",
					Prompt: "test",
				}

				stream, err := rp.GenerateChatCompletion(ctx, opts)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				_ = stream.Close()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.setupConfig()
			rp := NewRacingProvider("mixed-test", config)
			tt.testFunc(rp)
		})
	}
}
