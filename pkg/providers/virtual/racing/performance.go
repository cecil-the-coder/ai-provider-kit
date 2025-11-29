package racing

import (
	"sync"
	"time"
)

type PerformanceTracker struct {
	mu    sync.RWMutex
	stats map[string]*ProviderStats
}

type ProviderStats struct {
	TotalRaces   int64         `json:"total_races"`
	Wins         int64         `json:"wins"`
	Losses       int64         `json:"losses"`
	AvgLatency   time.Duration `json:"avg_latency"`
	TotalLatency time.Duration `json:"-"`
	WinRate      float64       `json:"win_rate"`
	LastUpdated  time.Time     `json:"last_updated"`
}

func NewPerformanceTracker() *PerformanceTracker {
	return &PerformanceTracker{
		stats: make(map[string]*ProviderStats),
	}
}

func (pt *PerformanceTracker) RecordWin(provider string, latency time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	stats := pt.getOrCreate(provider)
	stats.TotalRaces++
	stats.Wins++
	stats.TotalLatency += latency
	stats.AvgLatency = stats.TotalLatency / time.Duration(stats.TotalRaces)
	stats.WinRate = float64(stats.Wins) / float64(stats.TotalRaces)
	stats.LastUpdated = time.Now()
}

func (pt *PerformanceTracker) RecordLoss(provider string, latency time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	stats := pt.getOrCreate(provider)
	stats.TotalRaces++
	stats.Losses++
	stats.TotalLatency += latency
	stats.AvgLatency = stats.TotalLatency / time.Duration(stats.TotalRaces)
	stats.WinRate = float64(stats.Wins) / float64(stats.TotalRaces)
	stats.LastUpdated = time.Now()
}

func (pt *PerformanceTracker) GetScore(provider string) float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	stats, ok := pt.stats[provider]
	if !ok {
		return 0.5
	}

	return stats.WinRate
}

func (pt *PerformanceTracker) GetAllStats() map[string]*ProviderStats {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make(map[string]*ProviderStats)
	for k, v := range pt.stats {
		statsCopy := *v
		result[k] = &statsCopy
	}
	return result
}

func (pt *PerformanceTracker) getOrCreate(provider string) *ProviderStats {
	stats, ok := pt.stats[provider]
	if !ok {
		stats = &ProviderStats{}
		pt.stats[provider] = stats
	}
	return stats
}
