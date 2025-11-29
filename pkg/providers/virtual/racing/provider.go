package racing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// RacingProvider races multiple providers and returns the first successful response
type RacingProvider struct {
	name             string
	providers        []types.Provider
	config           *Config
	performance      *PerformanceTracker
	metricsCollector types.MetricsCollector
	mu               sync.RWMutex
}

type Config struct {
	TimeoutMS       int      `yaml:"timeout_ms"`
	GracePeriodMS   int      `yaml:"grace_period_ms"`
	Strategy        Strategy `yaml:"strategy"`
	ProviderNames   []string `yaml:"providers"`
	PerformanceFile string   `yaml:"performance_file,omitempty"`
}

type Strategy string

const (
	StrategyFirstWins Strategy = "first_wins"
	StrategyWeighted  Strategy = "weighted"
	StrategyQuality   Strategy = "quality"
)

type raceResult struct {
	index    int
	provider types.Provider
	stream   types.ChatCompletionStream
	err      error
	latency  time.Duration
}

func NewRacingProvider(name string, config *Config) *RacingProvider {
	return &RacingProvider{
		name:        name,
		config:      config,
		performance: NewPerformanceTracker(),
	}
}

func (r *RacingProvider) SetProviders(providers []types.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = providers
}

func (r *RacingProvider) Name() string             { return r.name }
func (r *RacingProvider) Type() types.ProviderType { return "racing" }
func (r *RacingProvider) Description() string      { return "Races multiple providers for fastest response" }

func (r *RacingProvider) GenerateChatCompletion(ctx context.Context, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
	r.mu.RLock()
	providers := r.providers
	collector := r.metricsCollector
	r.mu.RUnlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers configured for racing")
	}

	// Record race request
	if collector != nil {
		collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: r.name,
			ProviderType: r.Type(),
			ModelID:      opts.Model,
			Timestamp:    time.Now(),
		})
	}

	timeout := time.Duration(r.config.TimeoutMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create a cancellable context for racing goroutines that we can cancel early
	// when a winner is found, reducing resource waste
	raceCtx, raceCancel := context.WithCancel(ctx)

	results := make(chan *raceResult, len(providers))
	var wg sync.WaitGroup

	// Collect provider names for race participants
	raceParticipants := make([]string, len(providers))
	for i, provider := range providers {
		raceParticipants[i] = provider.Name()
		wg.Add(1)
		go func(idx int, p types.Provider) {
			defer wg.Done()
			start := time.Now()

			chatProvider, ok := p.(types.ChatProvider)
			if !ok {
				results <- &raceResult{index: idx, provider: p, err: fmt.Errorf("provider does not support chat")}
				return
			}

			// Use raceCtx so this goroutine can be cancelled early when winner is found
			stream, err := chatProvider.GenerateChatCompletion(raceCtx, opts)
			results <- &raceResult{
				index:    idx,
				provider: p,
				stream:   stream,
				err:      err,
				latency:  time.Since(start),
			}
		}(i, provider)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return r.selectWinner(ctx, results, raceCancel, raceParticipants, opts.Model)
}

func (r *RacingProvider) selectWinner(ctx context.Context, results chan *raceResult, cancelRace context.CancelFunc, raceParticipants []string, modelID string) (types.ChatCompletionStream, error) {
	defer cancelRace() // Always cancel racing context when winner is selected or error occurs

	switch r.config.Strategy {
	case StrategyWeighted:
		return r.weightedStrategy(ctx, results, raceParticipants, modelID)
	case StrategyQuality:
		return r.qualityStrategy(ctx, results, raceParticipants, modelID)
	default:
		return r.firstWinsStrategy(ctx, results, raceParticipants, modelID)
	}
}

func (r *RacingProvider) firstWinsStrategy(ctx context.Context, results chan *raceResult, raceParticipants []string, modelID string) (types.ChatCompletionStream, error) {
	r.mu.RLock()
	collector := r.metricsCollector
	r.mu.RUnlock()

	raceLatencies := make(map[string]time.Duration)
	var lastErr error
	var winner *raceResult

	for result := range results {
		raceLatencies[result.provider.Name()] = result.latency

		if result.err == nil && result.stream != nil && winner == nil {
			winner = result
			r.performance.RecordWin(result.provider.Name(), result.latency)
			// Continue collecting results to get complete latencies
		} else if result.err != nil {
			r.performance.RecordLoss(result.provider.Name(), result.latency)
			lastErr = result.err
		}
	}

	if winner != nil {
		// Emit race complete event
		if collector != nil {
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:             types.MetricEventRaceComplete,
				ProviderName:     r.name,
				ProviderType:     r.Type(),
				ModelID:          modelID,
				Timestamp:        time.Now(),
				RaceParticipants: raceParticipants,
				RaceLatencies:    raceLatencies,
				RaceWinner:       winner.provider.Name(),
				Latency:          winner.latency,
			})

			// Emit provider switch event (winner selected)
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventProviderSwitch,
				ProviderName: r.name,
				ProviderType: r.Type(),
				ModelID:      modelID,
				Timestamp:    time.Now(),
				ToProvider:   winner.provider.Name(),
				SwitchReason: "race_winner",
				Latency:      winner.latency,
			})

			// Record success
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventSuccess,
				ProviderName: r.name,
				ProviderType: r.Type(),
				ModelID:      modelID,
				Timestamp:    time.Now(),
				Latency:      winner.latency,
			})
		}

		return &racingStream{
			inner:    winner.stream,
			provider: winner.provider.Name(),
			latency:  winner.latency,
		}, nil
	}

	// All providers failed
	if collector != nil {
		collector.RecordEvent(ctx, types.MetricEvent{
			Type:             types.MetricEventError,
			ProviderName:     r.name,
			ProviderType:     r.Type(),
			ModelID:          modelID,
			Timestamp:        time.Now(),
			ErrorMessage:     "all providers failed",
			ErrorType:        "race_all_failed",
			RaceParticipants: raceParticipants,
			RaceLatencies:    raceLatencies,
		})
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("all providers failed")
}

func (r *RacingProvider) weightedStrategy(ctx context.Context, results chan *raceResult, raceParticipants []string, modelID string) (types.ChatCompletionStream, error) {
	gracePeriod := time.Duration(r.config.GracePeriodMS) * time.Millisecond
	timer := time.NewTimer(gracePeriod)
	defer timer.Stop()

	var candidates []*raceResult
	raceLatencies := make(map[string]time.Duration)

	for {
		select {
		case result, ok := <-results:
			if !ok {
				return r.pickBestCandidate(ctx, candidates, raceParticipants, raceLatencies, modelID)
			}
			raceLatencies[result.provider.Name()] = result.latency
			if result.err == nil && result.stream != nil {
				candidates = append(candidates, result)
				if len(candidates) == 1 {
					timer.Reset(gracePeriod)
				}
			}
		case <-timer.C:
			if len(candidates) > 0 {
				return r.pickBestCandidate(ctx, candidates, raceParticipants, raceLatencies, modelID)
			}
		case <-ctx.Done():
			if len(candidates) > 0 {
				return r.pickBestCandidate(ctx, candidates, raceParticipants, raceLatencies, modelID)
			}
			return nil, ctx.Err()
		}
	}
}

func (r *RacingProvider) qualityStrategy(ctx context.Context, results chan *raceResult, raceParticipants []string, modelID string) (types.ChatCompletionStream, error) {
	return r.weightedStrategy(ctx, results, raceParticipants, modelID)
}

func (r *RacingProvider) pickBestCandidate(ctx context.Context, candidates []*raceResult, raceParticipants []string, raceLatencies map[string]time.Duration, modelID string) (types.ChatCompletionStream, error) {
	r.mu.RLock()
	collector := r.metricsCollector
	r.mu.RUnlock()

	if len(candidates) == 0 {
		// All providers failed
		if collector != nil {
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:             types.MetricEventError,
				ProviderName:     r.name,
				ProviderType:     r.Type(),
				ModelID:          modelID,
				Timestamp:        time.Now(),
				ErrorMessage:     "no successful candidates",
				ErrorType:        "race_no_candidates",
				RaceParticipants: raceParticipants,
				RaceLatencies:    raceLatencies,
			})
		}
		return nil, fmt.Errorf("no successful candidates")
	}

	var best *raceResult
	var bestScore float64 = -1

	for _, c := range candidates {
		score := r.performance.GetScore(c.provider.Name())
		latencyFactor := 1.0 / (1.0 + c.latency.Seconds())
		adjustedScore := score * latencyFactor

		if adjustedScore > bestScore {
			bestScore = adjustedScore
			best = c
		}
	}

	if best != nil {
		r.performance.RecordWin(best.provider.Name(), best.latency)

		// Emit race complete event
		if collector != nil {
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:             types.MetricEventRaceComplete,
				ProviderName:     r.name,
				ProviderType:     r.Type(),
				ModelID:          modelID,
				Timestamp:        time.Now(),
				RaceParticipants: raceParticipants,
				RaceLatencies:    raceLatencies,
				RaceWinner:       best.provider.Name(),
				Latency:          best.latency,
			})

			// Emit provider switch event (winner selected)
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventProviderSwitch,
				ProviderName: r.name,
				ProviderType: r.Type(),
				ModelID:      modelID,
				Timestamp:    time.Now(),
				ToProvider:   best.provider.Name(),
				SwitchReason: "race_winner_weighted",
				Latency:      best.latency,
			})

			// Record success
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventSuccess,
				ProviderName: r.name,
				ProviderType: r.Type(),
				ModelID:      modelID,
				Timestamp:    time.Now(),
				Latency:      best.latency,
			})
		}

		return &racingStream{
			inner:    best.stream,
			provider: best.provider.Name(),
			latency:  best.latency,
		}, nil
	}

	// Fallback: if no best was found but we have candidates, use the first one
	// This should not happen in practice, but we check bounds for safety
	if len(candidates) > 0 {
		r.performance.RecordWin(candidates[0].provider.Name(), candidates[0].latency)

		// Emit events for fallback case
		if collector != nil {
			collector.RecordEvent(ctx, types.MetricEvent{
				Type:             types.MetricEventRaceComplete,
				ProviderName:     r.name,
				ProviderType:     r.Type(),
				ModelID:          modelID,
				Timestamp:        time.Now(),
				RaceParticipants: raceParticipants,
				RaceLatencies:    raceLatencies,
				RaceWinner:       candidates[0].provider.Name(),
				Latency:          candidates[0].latency,
			})

			collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventProviderSwitch,
				ProviderName: r.name,
				ProviderType: r.Type(),
				ModelID:      modelID,
				Timestamp:    time.Now(),
				ToProvider:   candidates[0].provider.Name(),
				SwitchReason: "race_winner_fallback",
				Latency:      candidates[0].latency,
			})

			collector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventSuccess,
				ProviderName: r.name,
				ProviderType: r.Type(),
				ModelID:      modelID,
				Timestamp:    time.Now(),
				Latency:      candidates[0].latency,
			})
		}

		return &racingStream{
			inner:    candidates[0].stream,
			provider: candidates[0].provider.Name(),
			latency:  candidates[0].latency,
		}, nil
	}

	return nil, fmt.Errorf("no valid candidate found")
}

func (r *RacingProvider) GetPerformanceStats() map[string]*ProviderStats {
	return r.performance.GetAllStats()
}

type racingStream struct {
	inner    types.ChatCompletionStream
	provider string
	latency  time.Duration
}

func (s *racingStream) Next() (types.ChatCompletionChunk, error) {
	chunk, err := s.inner.Next()
	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}
	chunk.Metadata["racing_winner"] = s.provider
	chunk.Metadata["racing_latency_ms"] = s.latency.Milliseconds()
	return chunk, err
}

func (s *racingStream) Close() error {
	return s.inner.Close()
}
