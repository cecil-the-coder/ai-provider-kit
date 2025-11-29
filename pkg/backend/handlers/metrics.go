package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/virtual/racing"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// MetricsHandler handles metrics endpoints
type MetricsHandler struct {
	providers map[string]types.Provider
	startTime time.Time
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(providers map[string]types.Provider) *MetricsHandler {
	return &MetricsHandler{
		providers: providers,
		startTime: time.Now(),
	}
}

// ProviderMetricsResponse represents the response for provider metrics
type ProviderMetricsResponse struct {
	Providers map[string]types.ProviderMetrics `json:"providers"`
	Timestamp time.Time                        `json:"timestamp"`
}

// SystemMetricsResponse represents system-level metrics
type SystemMetricsResponse struct {
	Uptime          string    `json:"uptime"`
	Goroutines      int       `json:"goroutines"`
	MemoryAllocated uint64    `json:"memory_allocated_bytes"`
	MemoryTotal     uint64    `json:"memory_total_bytes"`
	MemorySys       uint64    `json:"memory_sys_bytes"`
	NumGC           uint32    `json:"num_gc"`
	Timestamp       time.Time `json:"timestamp"`
}

// RacingMetricsResponse represents racing provider performance stats
type RacingMetricsResponse struct {
	RacingProviders map[string]map[string]*racing.ProviderStats `json:"racing_providers"`
	Timestamp       time.Time                                   `json:"timestamp"`
}

// GetProviderMetrics handles GET /api/metrics/providers
// Returns metrics for all registered providers
func (h *MetricsHandler) GetProviderMetrics(w http.ResponseWriter, r *http.Request) {
	providerMetrics := make(map[string]types.ProviderMetrics)

	for name, provider := range h.providers {
		// Get metrics from each provider
		if metricsProvider, ok := provider.(interface{ GetMetrics() types.ProviderMetrics }); ok {
			providerMetrics[name] = metricsProvider.GetMetrics()
		}
	}

	response := ProviderMetricsResponse{
		Providers: providerMetrics,
		Timestamp: time.Now(),
	}

	SendSuccess(w, r, response)
}

// GetSystemMetrics handles GET /api/metrics/system
// Returns system-level metrics including runtime stats
func (h *MetricsHandler) GetSystemMetrics(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(h.startTime)

	response := SystemMetricsResponse{
		Uptime:          uptime.String(),
		Goroutines:      runtime.NumGoroutine(),
		MemoryAllocated: m.Alloc,
		MemoryTotal:     m.TotalAlloc,
		MemorySys:       m.Sys,
		NumGC:           m.NumGC,
		Timestamp:       time.Now(),
	}

	SendSuccess(w, r, response)
}

// GetRacingMetrics handles GET /api/metrics/racing
// Returns performance stats from racing providers
func (h *MetricsHandler) GetRacingMetrics(w http.ResponseWriter, r *http.Request) {
	racingProviders := make(map[string]map[string]*racing.ProviderStats)

	for name, provider := range h.providers {
		// Check if this is a racing provider
		if rp, ok := provider.(*racing.RacingProvider); ok {
			stats := rp.GetPerformanceStats()
			if len(stats) > 0 {
				racingProviders[name] = stats
			}
		}
	}

	response := RacingMetricsResponse{
		RacingProviders: racingProviders,
		Timestamp:       time.Now(),
	}

	SendSuccess(w, r, response)
}
