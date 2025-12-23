package racing

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// Strategy Tests - First Wins
// ============================================================================

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

	// Verify performance stats - with StrategyFirstWins (returns fast),
	// only the winner's stats are guaranteed to be recorded
	stats := rp.GetPerformanceStats()
	if stats["success-provider"] == nil || stats["success-provider"].Wins != 1 {
		var wins int64
		if stats["success-provider"] != nil {
			wins = stats["success-provider"].Wins
		}
		t.Errorf("expected success-provider to have 1 win, got %d", wins)
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

	// With corrected StrategyFirstWins (returns fast), error is "no successful candidates"
	errStr := err.Error()
	if !containsString(errStr, "all providers failed") && !containsString(errStr, "no successful candidates") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================================
// Strategy Tests - Weighted
// ============================================================================

// TestWeightedStrategy_ScoreBasedSelection tests that StrategyWeighted
// picks the provider with the best performance score after collecting all results,
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
				Strategy:      StrategyWeighted, // Weighted strategy does score-based selection
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

// ============================================================================
// Strategy Tests - Quality
// ============================================================================

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

// ============================================================================
// Candidate Selection Tests
// ============================================================================

func TestPickBestCandidate_EmptyCandidates(t *testing.T) {
	rp := NewRacingProvider("test", &Config{
		Strategy: StrategyWeighted,
	})

	ctx := context.Background()
	cancelTimeout := func() {}
	cancelRace := func() {}
	_, err := rp.pickBestCandidate(ctx, []*raceResult{}, cancelTimeout, cancelRace, []string{}, make(map[string]time.Duration), "test-model", nil)

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
	cancelTimeout := func() {}
	cancelRace := func() {}
	raceLatencies := map[string]time.Duration{"only-provider": 100 * time.Millisecond}
	stream, err := rp.pickBestCandidate(ctx, candidates, cancelTimeout, cancelRace, []string{"only-provider"}, raceLatencies, "test-model", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stream == nil {
		t.Fatal("expected non-nil stream")
	}

	_, ok := stream.(*racingStream)
	if !ok {
		t.Fatal("expected racingStream type")
	}

	// The provider name is now in the StreamWrapper, check metadata instead
	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("expected no error on Next(), got %v", err)
	}

	if chunk.Metadata["racing_winner"] != "only-provider" {
		t.Errorf("expected racing_winner 'only-provider', got '%v'", chunk.Metadata["racing_winner"])
	}
}
