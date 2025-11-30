package racing

import (
	"context"
	"fmt"
	"sort"
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
	requestCount     int64
	mu               sync.RWMutex
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
	r.mu.Lock()
	providers := r.providers
	collector := r.metricsCollector
	r.requestCount++ // Increment request counter
	r.mu.Unlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers configured for racing")
	}

	var raceProviders []types.Provider
	var virtualModelConfig *VirtualModelConfig
	var err error

	// Check if virtual models are configured
	if len(r.config.VirtualModels) > 0 {
		// Virtual model mode: Get virtual model configuration
		virtualModelConfig = r.config.GetVirtualModel(opts.Model)
		if virtualModelConfig == nil {
			return nil, fmt.Errorf("virtual model not found: %s", opts.Model)
		}

		// Filter providers based on virtual model configuration
		raceProviders, err = r.getProvidersForVirtualModel(virtualModelConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get providers for virtual model %s: %w", opts.Model, err)
		}

		if len(raceProviders) == 0 {
			return nil, fmt.Errorf("no providers configured for virtual model: %s", opts.Model)
		}
	} else {
		// Legacy mode: Use all providers
		raceProviders = providers
	}

	// Record race request
	metadata := map[string]interface{}{}
	if virtualModelConfig != nil {
		metadata["virtual_model"] = virtualModelConfig.DisplayName
		metadata["virtual_model_desc"] = virtualModelConfig.Description
	} else {
		metadata["virtual_model"] = "legacy_mode"
		metadata["virtual_model_desc"] = "Legacy racing mode using all providers"
	}
	if collector != nil {
		_ = collector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: r.name,
			ProviderType: r.Type(),
			ModelID:      opts.Model,
			Timestamp:    time.Now(),
			Metadata:     metadata,
		})
	}

	// Update provider's own metrics counters
	r.mu.Lock()
	if r.metricsCollector != nil {
		// Record a request to track racing provider activity
		_ = r.metricsCollector.RecordEvent(ctx, types.MetricEvent{
			Type:         types.MetricEventRequest,
			ProviderName: r.name,
			ProviderType: r.Type(),
			ModelID:      opts.Model,
			Timestamp:    time.Now(),
			Metadata:     metadata,
		})
	}
	r.mu.Unlock()

	// Use virtual model specific timeout or fallback to default
	timeout := time.Duration(r.config.GetEffectiveTimeout(opts.Model)) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create a cancellable context for racing goroutines that we can cancel early
	// when a winner is found, reducing resource waste
	raceCtx, raceCancel := context.WithCancel(ctx)

	results := make(chan *raceResult, len(raceProviders))
	var wg sync.WaitGroup

	// Collect provider names for race participants
	raceParticipants := make([]string, len(raceProviders))
	for i, provider := range raceProviders {
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

			// Update model options for this specific provider
			providerOpts := opts
			if virtualModelConfig != nil {
				if refProvider := r.findProviderReference(p.Name(), virtualModelConfig.Providers); refProvider != nil {
					providerOpts.Model = refProvider.Model
				}
			}
			// In legacy mode, use the original model as-is

			// Use raceCtx so this goroutine can be cancelled early when winner is found
			stream, err := chatProvider.GenerateChatCompletion(raceCtx, providerOpts)
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

	// Use appropriate winner selection method
	if virtualModelConfig != nil {
		return r.selectWinnerWithVirtualModel(ctx, results, raceCancel, raceParticipants, opts.Model, virtualModelConfig)
	} else {
		// Legacy mode: use standard selectWinner method with legacy virtual model info
		legacyVMConfig := &VirtualModelConfig{
			DisplayName: "legacy_mode",
			Description: "Legacy racing mode using all providers",
		}
		return r.selectWinner(ctx, results, raceCancel, raceParticipants, opts.Model, legacyVMConfig)
	}
}

func (r *RacingProvider) selectWinner(ctx context.Context, results chan *raceResult, cancelRace context.CancelFunc, raceParticipants []string, modelID string, virtualModelConfig *VirtualModelConfig) (types.ChatCompletionStream, error) {
	defer cancelRace() // Always cancel racing context when winner is selected or error occurs

	switch r.config.Strategy {
	case StrategyWeighted:
		return r.weightedStrategy(ctx, results, raceParticipants, modelID, virtualModelConfig)
	case StrategyQuality:
		return r.qualityStrategy(ctx, results, raceParticipants, modelID, virtualModelConfig)
	default:
		return r.firstWinsStrategy(ctx, results, raceParticipants, modelID, virtualModelConfig)
	}
}

func (r *RacingProvider) firstWinsStrategy(ctx context.Context, results chan *raceResult, raceParticipants []string, modelID string, virtualModelConfig *VirtualModelConfig) (types.ChatCompletionStream, error) {
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
		r.emitRaceWinnerEvents(ctx, collector, winner, raceParticipants, raceLatencies, modelID, "race_winner")

		// Record success for racing provider metrics
		r.mu.Lock()
		if r.metricsCollector != nil {
			_ = r.metricsCollector.RecordEvent(ctx, types.MetricEvent{
				Type:         types.MetricEventSuccess,
				ProviderName: r.name,
				ProviderType: r.Type(),
				ModelID:      modelID,
				Timestamp:    time.Now(),
				Latency:      winner.latency,
				Metadata: map[string]interface{}{
					"racing_winner": winner.provider.Name(),
				},
			})
		}
		r.mu.Unlock()

		// Prepare virtual model metadata for the stream
		var virtualModelName, virtualModelDesc string
		if virtualModelConfig != nil {
			virtualModelName = virtualModelConfig.DisplayName
			virtualModelDesc = virtualModelConfig.Description
		}

		return &racingStream{
			inner:            winner.stream,
			provider:         winner.provider.Name(),
			latency:          winner.latency,
			virtualModel:     virtualModelName,
			virtualModelDesc: virtualModelDesc,
		}, nil
	}

	// All providers failed
	if collector != nil {
		_ = collector.RecordEvent(ctx, types.MetricEvent{
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

func (r *RacingProvider) weightedStrategy(ctx context.Context, results chan *raceResult, raceParticipants []string, modelID string, virtualModelConfig *VirtualModelConfig) (types.ChatCompletionStream, error) {
	gracePeriod := time.Duration(r.config.GracePeriodMS) * time.Millisecond
	timer := time.NewTimer(gracePeriod)
	defer timer.Stop()

	var candidates []*raceResult
	raceLatencies := make(map[string]time.Duration)

	for {
		select {
		case result, ok := <-results:
			if !ok {
				return r.pickBestCandidate(ctx, candidates, raceParticipants, raceLatencies, modelID, virtualModelConfig)
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
				return r.pickBestCandidate(ctx, candidates, raceParticipants, raceLatencies, modelID, virtualModelConfig)
			}
		case <-ctx.Done():
			if len(candidates) > 0 {
				return r.pickBestCandidate(ctx, candidates, raceParticipants, raceLatencies, modelID, virtualModelConfig)
			}
			return nil, ctx.Err()
		}
	}
}

func (r *RacingProvider) qualityStrategy(ctx context.Context, results chan *raceResult, raceParticipants []string, modelID string, virtualModelConfig *VirtualModelConfig) (types.ChatCompletionStream, error) {
	return r.weightedStrategy(ctx, results, raceParticipants, modelID, virtualModelConfig)
}

func (r *RacingProvider) pickBestCandidate(ctx context.Context, candidates []*raceResult, raceParticipants []string, raceLatencies map[string]time.Duration, modelID string, virtualModelConfig *VirtualModelConfig) (types.ChatCompletionStream, error) {
	r.mu.RLock()
	collector := r.metricsCollector
	r.mu.RUnlock()

	if len(candidates) == 0 {
		// All providers failed
		if collector != nil {
			_ = collector.RecordEvent(ctx, types.MetricEvent{
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
		r.emitRaceWinnerEvents(ctx, collector, best, raceParticipants, raceLatencies, modelID, "race_winner_weighted")

		// Prepare virtual model metadata for the stream
		var virtualModelName, virtualModelDesc string
		if virtualModelConfig != nil {
			virtualModelName = virtualModelConfig.DisplayName
			virtualModelDesc = virtualModelConfig.Description
		}

		return &racingStream{
			inner:            best.stream,
			provider:         best.provider.Name(),
			latency:          best.latency,
			virtualModel:     virtualModelName,
			virtualModelDesc: virtualModelDesc,
		}, nil
	}

	// Fallback: if no best was found but we have candidates, use the first one
	// This should not happen in practice, but we check bounds for safety
	if len(candidates) > 0 {
		r.performance.RecordWin(candidates[0].provider.Name(), candidates[0].latency)
		r.emitRaceWinnerEvents(ctx, collector, candidates[0], raceParticipants, raceLatencies, modelID, "race_winner_fallback")

		// Prepare virtual model metadata for the stream
		var virtualModelName, virtualModelDesc string
		if virtualModelConfig != nil {
			virtualModelName = virtualModelConfig.DisplayName
			virtualModelDesc = virtualModelConfig.Description
		}

		return &racingStream{
			inner:            candidates[0].stream,
			provider:         candidates[0].provider.Name(),
			latency:          candidates[0].latency,
			virtualModel:     virtualModelName,
			virtualModelDesc: virtualModelDesc,
		}, nil
	}

	return nil, fmt.Errorf("no valid candidate found")
}

func (r *RacingProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	if len(r.config.VirtualModels) == 0 {
		return []types.Model{}, nil
	}

	// Get sorted keys for deterministic order
	names := make([]string, 0, len(r.config.VirtualModels))
	for name := range r.config.VirtualModels {
		names = append(names, name)
	}
	sort.Strings(names)

	models := make([]types.Model, 0, len(r.config.VirtualModels))
	for _, name := range names {
		vmConfig := r.config.VirtualModels[name]
		model := types.Model{
			ID:          name,
			Name:        vmConfig.DisplayName,
			Provider:    r.Type(),
			Description: vmConfig.Description,
			// Add other fields if needed (capabilities, etc.)
		}
		models = append(models, model)
	}

	return models, nil
}

func (r *RacingProvider) GetDefaultModel() string {
	// Check if default virtual model exists in the config
	if r.config.DefaultVirtualModel != "" {
		if _, exists := r.config.VirtualModels[r.config.DefaultVirtualModel]; exists {
			return r.config.DefaultVirtualModel
		}
		// Default virtual model doesn't exist, fall back to first available
	}

	// Return first virtual model alphabetically, if any exist
	for name := range r.config.VirtualModels {
		return name
	}

	return ""
}

// getProvidersForVirtualModel returns the providers that should participate in the race
// for the given virtual model configuration
func (r *RacingProvider) getProvidersForVirtualModel(vmConfig *VirtualModelConfig) ([]types.Provider, error) {
	r.mu.RLock()
	allProviders := r.providers
	r.mu.RUnlock()

	if len(vmConfig.Providers) == 0 {
		return nil, fmt.Errorf("virtual model has no providers configured")
	}

	var raceProviders []types.Provider
	providerMap := make(map[string]types.Provider)

	// Create a map of available providers for quick lookup
	for _, provider := range allProviders {
		providerMap[provider.Name()] = provider
	}

	// Find providers referenced in the virtual model configuration
	for _, providerRef := range vmConfig.Providers {
		if provider, exists := providerMap[providerRef.Name]; exists {
			raceProviders = append(raceProviders, provider)
		} else {
			return nil, fmt.Errorf("provider not found: %s", providerRef.Name)
		}
	}

	if len(raceProviders) == 0 {
		return nil, fmt.Errorf("no valid providers found for virtual model")
	}

	return raceProviders, nil
}

// findProviderReference finds the provider reference for a given provider name
func (r *RacingProvider) findProviderReference(providerName string, providerRefs []ProviderReference) *ProviderReference {
	for _, ref := range providerRefs {
		if ref.Name == providerName {
			return &ref
		}
	}
	return nil
}

// selectWinnerWithVirtualModel selects the winner using the virtual model's strategy
func (r *RacingProvider) selectWinnerWithVirtualModel(ctx context.Context, results chan *raceResult, cancelRace context.CancelFunc, raceParticipants []string, modelID string, vmConfig *VirtualModelConfig) (types.ChatCompletionStream, error) {
	defer cancelRace() // Always cancel racing context when winner is selected or error occurs

	// Use virtual model specific strategy or fallback to default
	strategy := vmConfig.Strategy
	if strategy == "" {
		strategy = r.config.Strategy
	}

	switch strategy {
	case StrategyWeighted:
		return r.weightedStrategy(ctx, results, raceParticipants, modelID, vmConfig)
	case StrategyQuality:
		return r.qualityStrategy(ctx, results, raceParticipants, modelID, vmConfig)
	default:
		return r.firstWinsStrategy(ctx, results, raceParticipants, modelID, vmConfig)
	}
}

func (r *RacingProvider) GetPerformanceStats() map[string]*ProviderStats {
	return r.performance.GetAllStats()
}

// emitRaceWinnerEvents emits the standard set of events for a race winner
func (r *RacingProvider) emitRaceWinnerEvents(ctx context.Context, collector types.MetricsCollector, winner *raceResult, raceParticipants []string, raceLatencies map[string]time.Duration, modelID, switchReason string) {
	if collector == nil {
		return
	}

	// Emit race complete event
	_ = collector.RecordEvent(ctx, types.MetricEvent{
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
	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventProviderSwitch,
		ProviderName: r.name,
		ProviderType: r.Type(),
		ModelID:      modelID,
		Timestamp:    time.Now(),
		ToProvider:   winner.provider.Name(),
		SwitchReason: switchReason,
		Latency:      winner.latency,
	})

	// Record success
	_ = collector.RecordEvent(ctx, types.MetricEvent{
		Type:         types.MetricEventSuccess,
		ProviderName: r.name,
		ProviderType: r.Type(),
		ModelID:      modelID,
		Timestamp:    time.Now(),
		Latency:      winner.latency,
	})
}

type racingStream struct {
	inner            types.ChatCompletionStream
	provider         string
	latency          time.Duration
	virtualModel     string
	virtualModelDesc string
}

func (s *racingStream) Next() (types.ChatCompletionChunk, error) {
	chunk, err := s.inner.Next()
	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}
	chunk.Metadata["racing_winner"] = s.provider
	chunk.Metadata["racing_latency_ms"] = s.latency.Milliseconds()
	if s.virtualModel != "" {
		chunk.Metadata["virtual_model"] = s.virtualModel
	}
	if s.virtualModelDesc != "" {
		chunk.Metadata["virtual_model_desc"] = s.virtualModelDesc
	}
	return chunk, err
}

func (s *racingStream) Close() error {
	return s.inner.Close()
}
