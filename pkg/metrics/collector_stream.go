package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// streamMetrics tracks streaming-specific metrics
type streamMetrics struct {
	mu sync.Mutex

	totalStreamRequests      atomic.Int64
	successfulStreamRequests atomic.Int64
	failedStreamRequests     atomic.Int64

	ttftHistogram *Histogram // Time to first token

	totalStreamedTokens atomic.Int64
	totalChunks         atomic.Int64

	minTokensPerSecond   float64
	maxTokensPerSecond   float64
	totalTokensPerSecond float64
	tpsCount             int64

	minStreamDuration   time.Duration
	maxStreamDuration   time.Duration
	totalStreamDuration time.Duration

	lastUpdated time.Time
}

// newStreamMetrics creates a new streamMetrics instance
func newStreamMetrics() *streamMetrics {
	return &streamMetrics{
		ttftHistogram: NewHistogram(1000),
	}
}

// RecordStreamStart records the start of a streaming request
func (sm *streamMetrics) RecordStreamStart(event types.MetricEvent) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.totalStreamRequests.Add(1)

	if event.TimeToFirstToken > 0 {
		sm.ttftHistogram.Add(event.TimeToFirstToken)
	}

	sm.lastUpdated = time.Now()
}

// RecordStreamEnd records the end of a streaming request
func (sm *streamMetrics) RecordStreamEnd(event types.MetricEvent) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.successfulStreamRequests.Add(1)

	if event.TokensUsed > 0 {
		sm.totalStreamedTokens.Add(event.TokensUsed)
	}

	if event.TokensPerSecond > 0 {
		if sm.minTokensPerSecond == 0 || event.TokensPerSecond < sm.minTokensPerSecond {
			sm.minTokensPerSecond = event.TokensPerSecond
		}
		if event.TokensPerSecond > sm.maxTokensPerSecond {
			sm.maxTokensPerSecond = event.TokensPerSecond
		}
		sm.totalTokensPerSecond += event.TokensPerSecond
		sm.tpsCount++
	}

	if event.Latency > 0 {
		if sm.minStreamDuration == 0 || event.Latency < sm.minStreamDuration {
			sm.minStreamDuration = event.Latency
		}
		if event.Latency > sm.maxStreamDuration {
			sm.maxStreamDuration = event.Latency
		}
		sm.totalStreamDuration += event.Latency
	}

	sm.lastUpdated = time.Now()
}

// RecordStreamAbort records an aborted streaming request
func (sm *streamMetrics) RecordStreamAbort() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.failedStreamRequests.Add(1)
	sm.lastUpdated = time.Now()
}

// GetSnapshot returns a snapshot of the stream metrics
func (sm *streamMetrics) GetSnapshot() *types.StreamMetrics {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	totalStream := sm.totalStreamRequests.Load()
	successStream := sm.successfulStreamRequests.Load()
	failedStream := sm.failedStreamRequests.Load()

	if totalStream == 0 {
		return nil // No streaming data
	}

	avgTPS := float64(0)
	if sm.tpsCount > 0 {
		avgTPS = sm.totalTokensPerSecond / float64(sm.tpsCount)
	}

	avgDuration := time.Duration(0)
	if successStream > 0 {
		avgDuration = sm.totalStreamDuration / time.Duration(successStream)
	}

	totalTokens := sm.totalStreamedTokens.Load()
	totalChunks := sm.totalChunks.Load()

	avgTokensPerStream := float64(0)
	if successStream > 0 {
		avgTokensPerStream = float64(totalTokens) / float64(successStream)
	}

	avgChunksPerStream := float64(0)
	if successStream > 0 {
		avgChunksPerStream = float64(totalChunks) / float64(successStream)
	}

	avgChunkSize := float64(0)
	if totalChunks > 0 {
		avgChunkSize = float64(totalTokens) / float64(totalChunks)
	}

	ttftMetrics := sm.ttftHistogram.GetLatencyMetrics()

	return &types.StreamMetrics{
		TotalStreamRequests:      totalStream,
		SuccessfulStreamRequests: successStream,
		FailedStreamRequests:     failedStream,
		StreamSuccessRate:        calculateRate(successStream, totalStream),
		TimeToFirstToken: types.TimeToFirstTokenMetrics{
			TotalMeasurements: ttftMetrics.TotalRequests,
			AverageTTFT:       ttftMetrics.AverageLatency,
			MinTTFT:           ttftMetrics.MinLatency,
			MaxTTFT:           ttftMetrics.MaxLatency,
			P50TTFT:           ttftMetrics.P50Latency,
			P75TTFT:           ttftMetrics.P75Latency,
			P90TTFT:           ttftMetrics.P90Latency,
			P95TTFT:           ttftMetrics.P95Latency,
			P99TTFT:           ttftMetrics.P99Latency,
			LastUpdated:       ttftMetrics.LastUpdated,
		},
		AverageTokensPerSecond: avgTPS,
		MinTokensPerSecond:     sm.minTokensPerSecond,
		MaxTokensPerSecond:     sm.maxTokensPerSecond,
		MedianTokensPerSecond:  avgTPS, // Simplified: use average as median
		AverageStreamDuration:  avgDuration,
		MinStreamDuration:      sm.minStreamDuration,
		MaxStreamDuration:      sm.maxStreamDuration,
		TotalStreamedTokens:    totalTokens,
		AverageTokensPerStream: avgTokensPerStream,
		TotalChunks:            totalChunks,
		AverageChunksPerStream: avgChunksPerStream,
		AverageChunkSize:       avgChunkSize,
		LastUpdated:            sm.lastUpdated,
	}
}
