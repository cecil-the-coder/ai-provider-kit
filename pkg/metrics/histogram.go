package metrics

import (
	"sort"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Histogram is a simple circular buffer for storing latency samples
// and calculating percentiles
type Histogram struct {
	mu          sync.RWMutex
	samples     []time.Duration
	capacity    int
	index       int
	count       int64
	total       time.Duration
	min         time.Duration
	max         time.Duration
	lastUpdated time.Time
}

// NewHistogram creates a new histogram with the given sample size
func NewHistogram(sampleSize int) *Histogram {
	if sampleSize <= 0 {
		sampleSize = 1000
	}
	return &Histogram{
		samples:  make([]time.Duration, sampleSize),
		capacity: sampleSize,
	}
}

// Add adds a latency sample to the histogram
func (h *Histogram) Add(latency time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Add to circular buffer
	h.samples[h.index] = latency
	h.index = (h.index + 1) % h.capacity
	h.count++

	// Update total
	h.total += latency

	// Update min/max
	if h.min == 0 || latency < h.min {
		h.min = latency
	}
	if latency > h.max {
		h.max = latency
	}

	h.lastUpdated = time.Now()
}

// GetLatencyMetrics returns the current latency metrics including percentiles
func (h *Histogram) GetLatencyMetrics() types.LatencyMetrics {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalRequests := h.count
	if totalRequests == 0 {
		return types.LatencyMetrics{
			TotalRequests: 0,
			LastUpdated:   h.lastUpdated,
		}
	}

	avgLatency := time.Duration(0)
	if totalRequests > 0 {
		avgLatency = h.total / time.Duration(totalRequests)
	}

	// Get current sample size
	sampleCount := int(totalRequests)
	if sampleCount > h.capacity {
		sampleCount = h.capacity
	}

	// Copy samples for percentile calculation
	samples := make([]time.Duration, sampleCount)
	if h.count < int64(h.capacity) {
		// Haven't filled the buffer yet
		copy(samples, h.samples[:h.count])
	} else {
		// Buffer is full, copy all samples
		copy(samples, h.samples)
	}

	// Calculate percentiles
	percentiles := calculatePercentiles(samples)

	return types.LatencyMetrics{
		TotalRequests:  totalRequests,
		TotalLatency:   h.total,
		AverageLatency: avgLatency,
		MinLatency:     h.min,
		MaxLatency:     h.max,
		P50Latency:     percentiles[50],
		P75Latency:     percentiles[75],
		P90Latency:     percentiles[90],
		P95Latency:     percentiles[95],
		P99Latency:     percentiles[99],
		LastUpdated:    h.lastUpdated,
	}
}

// calculatePercentiles calculates percentiles from a slice of durations
func calculatePercentiles(samples []time.Duration) map[int]time.Duration {
	result := make(map[int]time.Duration)

	if len(samples) == 0 {
		return result
	}

	// Sort samples
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate percentiles
	percentiles := []int{50, 75, 90, 95, 99}
	for _, p := range percentiles {
		result[p] = getPercentile(sorted, p)
	}

	return result
}

// getPercentile returns the value at the given percentile
func getPercentile(sortedSamples []time.Duration, percentile int) time.Duration {
	if len(sortedSamples) == 0 {
		return 0
	}

	if percentile >= 100 {
		return sortedSamples[len(sortedSamples)-1]
	}

	if percentile <= 0 {
		return sortedSamples[0]
	}

	// Linear interpolation between closest ranks
	rank := float64(percentile) / 100.0 * float64(len(sortedSamples)-1)
	lowerIndex := int(rank)
	upperIndex := lowerIndex + 1

	if upperIndex >= len(sortedSamples) {
		return sortedSamples[len(sortedSamples)-1]
	}

	// Interpolate between lower and upper values
	lowerValue := sortedSamples[lowerIndex]
	upperValue := sortedSamples[upperIndex]
	fraction := rank - float64(lowerIndex)

	interpolated := float64(lowerValue) + fraction*float64(upperValue-lowerValue)
	return time.Duration(interpolated)
}

// Reset clears all samples from the histogram
func (h *Histogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.samples = make([]time.Duration, h.capacity)
	h.index = 0
	h.count = 0
	h.total = 0
	h.min = 0
	h.max = 0
	h.lastUpdated = time.Time{}
}
