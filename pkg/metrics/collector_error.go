package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// errorMetrics tracks error statistics
type errorMetrics struct {
	mu sync.Mutex

	totalErrors atomic.Int64

	errorsByType   map[string]*atomic.Int64
	errorsByStatus map[string]*atomic.Int64

	rateLimitErrors      atomic.Int64
	timeoutErrors        atomic.Int64
	authenticationErrors atomic.Int64
	invalidRequestErrors atomic.Int64
	serverErrors         atomic.Int64
	networkErrors        atomic.Int64
	unknownErrors        atomic.Int64

	consecutiveErrors atomic.Int64
	lastError         string
	lastErrorType     string
	lastErrorTime     time.Time

	lastUpdated time.Time
}

// newErrorMetrics creates a new errorMetrics instance
func newErrorMetrics() *errorMetrics {
	return &errorMetrics{
		errorsByType:   make(map[string]*atomic.Int64),
		errorsByStatus: make(map[string]*atomic.Int64),
	}
}

// RecordError records an error event
func (em *errorMetrics) RecordError(event types.MetricEvent) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.totalErrors.Add(1)
	em.lastError = event.ErrorMessage
	em.lastErrorType = event.ErrorType
	em.lastErrorTime = event.Timestamp
	em.lastUpdated = time.Now()

	// Track consecutive errors
	if event.Type.IsError() {
		em.consecutiveErrors.Add(1)
	} else {
		em.consecutiveErrors.Store(0)
	}

	// Track by type
	if event.ErrorType != "" {
		if em.errorsByType[event.ErrorType] == nil {
			em.errorsByType[event.ErrorType] = &atomic.Int64{}
		}
		em.errorsByType[event.ErrorType].Add(1)
	}

	// Track by status
	if event.StatusCode > 0 {
		statusStr := fmt.Sprintf("%d", event.StatusCode)
		if em.errorsByStatus[statusStr] == nil {
			em.errorsByStatus[statusStr] = &atomic.Int64{}
		}
		em.errorsByStatus[statusStr].Add(1)
	}

	// Categorize errors
	switch event.Type {
	case types.MetricEventRateLimit:
		em.rateLimitErrors.Add(1)
	case types.MetricEventTimeout:
		em.timeoutErrors.Add(1)
	}

	// Categorize by error type
	switch event.ErrorType {
	case "authentication":
		em.authenticationErrors.Add(1)
	case "invalid_request":
		em.invalidRequestErrors.Add(1)
	case "network":
		em.networkErrors.Add(1)
	}

	// Categorize by status code
	if event.StatusCode >= 500 {
		em.serverErrors.Add(1)
	}
}

// GetSnapshot returns a snapshot of the error metrics
func (em *errorMetrics) GetSnapshot(totalRequests int64) types.ErrorMetrics {
	em.mu.Lock()
	defer em.mu.Unlock()

	totalErrors := em.totalErrors.Load()

	errorsByType := make(map[string]int64)
	for errType, counter := range em.errorsByType {
		errorsByType[errType] = counter.Load()
	}

	errorsByStatus := make(map[string]int64)
	for status, counter := range em.errorsByStatus {
		errorsByStatus[status] = counter.Load()
	}

	return types.ErrorMetrics{
		TotalErrors:             totalErrors,
		ErrorRate:               calculateRate(totalErrors, totalRequests),
		ErrorsByType:            errorsByType,
		ErrorsByStatus:          errorsByStatus,
		ErrorsByProvider:        make(map[string]int64),
		ErrorsByModel:           make(map[string]int64),
		RateLimitErrors:         em.rateLimitErrors.Load(),
		TimeoutErrors:           em.timeoutErrors.Load(),
		AuthenticationErrors:    em.authenticationErrors.Load(),
		InvalidRequestErrors:    em.invalidRequestErrors.Load(),
		ServerErrors:            em.serverErrors.Load(),
		NetworkErrors:           em.networkErrors.Load(),
		UnknownErrors:           em.unknownErrors.Load(),
		RateLimitErrorRate:      calculateRate(em.rateLimitErrors.Load(), totalErrors),
		TimeoutErrorRate:        calculateRate(em.timeoutErrors.Load(), totalErrors),
		AuthenticationErrorRate: calculateRate(em.authenticationErrors.Load(), totalErrors),
		ServerErrorRate:         calculateRate(em.serverErrors.Load(), totalErrors),
		ConsecutiveErrors:       em.consecutiveErrors.Load(),
		LastError:               em.lastError,
		LastErrorType:           em.lastErrorType,
		LastErrorTime:           em.lastErrorTime,
		LastUpdated:             em.lastUpdated,
	}
}
