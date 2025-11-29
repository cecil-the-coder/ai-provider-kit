package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHistogram(t *testing.T) {
	h := NewHistogram(100)
	assert.NotNil(t, h)
	assert.Equal(t, 100, h.capacity)

	// Test with invalid size
	h2 := NewHistogram(0)
	assert.NotNil(t, h2)
	assert.Equal(t, 1000, h2.capacity) // Should default to 1000
}

func TestHistogramAdd(t *testing.T) {
	h := NewHistogram(10)

	// Add some samples
	h.Add(100 * time.Millisecond)
	h.Add(200 * time.Millisecond)
	h.Add(150 * time.Millisecond)

	metrics := h.GetLatencyMetrics()
	assert.Equal(t, int64(3), metrics.TotalRequests)
	assert.Equal(t, 450*time.Millisecond, metrics.TotalLatency)
	assert.Equal(t, 150*time.Millisecond, metrics.AverageLatency)
	assert.Equal(t, 100*time.Millisecond, metrics.MinLatency)
	assert.Equal(t, 200*time.Millisecond, metrics.MaxLatency)
}

func TestHistogramPercentiles(t *testing.T) {
	h := NewHistogram(1000)

	// Add samples from 1ms to 100ms
	for i := 1; i <= 100; i++ {
		h.Add(time.Duration(i) * time.Millisecond)
	}

	metrics := h.GetLatencyMetrics()
	assert.Equal(t, int64(100), metrics.TotalRequests)

	// Check percentiles are in expected ranges
	assert.Greater(t, metrics.P50Latency, 40*time.Millisecond)
	assert.Less(t, metrics.P50Latency, 60*time.Millisecond)

	assert.Greater(t, metrics.P75Latency, 70*time.Millisecond)
	assert.Less(t, metrics.P75Latency, 80*time.Millisecond)

	assert.Greater(t, metrics.P90Latency, 85*time.Millisecond)
	assert.Less(t, metrics.P90Latency, 95*time.Millisecond)

	assert.Greater(t, metrics.P95Latency, 90*time.Millisecond)
	assert.Less(t, metrics.P95Latency, 100*time.Millisecond)

	assert.Greater(t, metrics.P99Latency, 95*time.Millisecond)
	assert.LessOrEqual(t, metrics.P99Latency, 100*time.Millisecond)
}

func TestHistogramCircularBuffer(t *testing.T) {
	h := NewHistogram(5) // Small buffer

	// Add more samples than buffer size
	for i := 1; i <= 10; i++ {
		h.Add(time.Duration(i) * time.Millisecond)
	}

	metrics := h.GetLatencyMetrics()
	assert.Equal(t, int64(10), metrics.TotalRequests) // Count all requests
	assert.Equal(t, 55*time.Millisecond, metrics.TotalLatency) // Sum of 1+2+3+...+10

	// Min should be 1ms (from total tracking)
	assert.Equal(t, 1*time.Millisecond, metrics.MinLatency)
	// Max should be 10ms (from total tracking)
	assert.Equal(t, 10*time.Millisecond, metrics.MaxLatency)

	// Percentiles should be calculated from last 5 samples (6, 7, 8, 9, 10)
	assert.Greater(t, metrics.P50Latency, 5*time.Millisecond)
}

func TestHistogramReset(t *testing.T) {
	h := NewHistogram(10)

	// Add samples
	h.Add(100 * time.Millisecond)
	h.Add(200 * time.Millisecond)

	metrics := h.GetLatencyMetrics()
	assert.Equal(t, int64(2), metrics.TotalRequests)

	// Reset
	h.Reset()

	// Verify it's empty
	metrics = h.GetLatencyMetrics()
	assert.Equal(t, int64(0), metrics.TotalRequests)
	assert.Equal(t, time.Duration(0), metrics.TotalLatency)
	assert.Equal(t, time.Duration(0), metrics.MinLatency)
	assert.Equal(t, time.Duration(0), metrics.MaxLatency)
}

func TestHistogramEmptyMetrics(t *testing.T) {
	h := NewHistogram(10)

	// Get metrics without adding any samples
	metrics := h.GetLatencyMetrics()
	assert.Equal(t, int64(0), metrics.TotalRequests)
	assert.Equal(t, time.Duration(0), metrics.TotalLatency)
	assert.Equal(t, time.Duration(0), metrics.AverageLatency)
	assert.Equal(t, time.Duration(0), metrics.P50Latency)
	assert.Equal(t, time.Duration(0), metrics.P99Latency)
}

func TestCalculatePercentiles(t *testing.T) {
	// Test with small sample
	samples := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	percentiles := calculatePercentiles(samples)

	// P50 (median) should be around 30ms
	assert.Equal(t, 30*time.Millisecond, percentiles[50])

	// P90 should be between 40 and 50
	assert.GreaterOrEqual(t, percentiles[90], 40*time.Millisecond)
	assert.LessOrEqual(t, percentiles[90], 50*time.Millisecond)
}

func TestCalculatePercentilesEmpty(t *testing.T) {
	samples := []time.Duration{}
	percentiles := calculatePercentiles(samples)

	assert.Equal(t, time.Duration(0), percentiles[50])
	assert.Equal(t, time.Duration(0), percentiles[99])
}

func TestGetPercentile(t *testing.T) {
	samples := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	// Test P0 (minimum)
	p0 := getPercentile(samples, 0)
	assert.Equal(t, 10*time.Millisecond, p0)

	// Test P100 (maximum)
	p100 := getPercentile(samples, 100)
	assert.Equal(t, 50*time.Millisecond, p100)

	// Test P50 (median)
	p50 := getPercentile(samples, 50)
	assert.Equal(t, 30*time.Millisecond, p50)

	// Test empty slice
	empty := []time.Duration{}
	p := getPercentile(empty, 50)
	assert.Equal(t, time.Duration(0), p)
}

func TestGetPercentileInterpolation(t *testing.T) {
	// Test interpolation with non-uniform data
	samples := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
		6 * time.Millisecond,
		7 * time.Millisecond,
		8 * time.Millisecond,
		9 * time.Millisecond,
		10 * time.Millisecond,
	}

	// P50 should be between 5 and 6
	p50 := getPercentile(samples, 50)
	assert.GreaterOrEqual(t, p50, 5*time.Millisecond)
	assert.LessOrEqual(t, p50, 6*time.Millisecond)

	// P90 should be between 9 and 10
	p90 := getPercentile(samples, 90)
	assert.GreaterOrEqual(t, p90, 9*time.Millisecond)
	assert.LessOrEqual(t, p90, 10*time.Millisecond)
}

func TestHistogramConcurrency(t *testing.T) {
	h := NewHistogram(1000)
	done := make(chan bool)

	// Spawn multiple goroutines adding samples concurrently
	for i := 0; i < 10; i++ {
		go func(offset int) {
			for j := 0; j < 100; j++ {
				h.Add(time.Duration(offset*100+j) * time.Millisecond)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all samples were recorded
	metrics := h.GetLatencyMetrics()
	assert.Equal(t, int64(1000), metrics.TotalRequests)
}
