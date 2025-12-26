package quota

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// QuotaType represents the type of quota being measured
type QuotaType string

const (
	// QuotaTypeRequests represents request-based quotas
	QuotaTypeRequests QuotaType = "requests"
	// QuotaTypeTokens represents token-based quotas
	QuotaTypeTokens QuotaType = "tokens"
	// QuotaTypeInputTokens represents input token quotas
	QuotaTypeInputTokens QuotaType = "input_tokens"
	// QuotaTypeOutputTokens represents output token quotas
	QuotaTypeOutputTokens QuotaType = "output_tokens"
	// QuotaTypeDaily represents daily quotas
	QuotaTypeDaily QuotaType = "daily"
	// QuotaTypeCustom represents provider-specific custom quotas
	QuotaTypeCustom QuotaType = "custom"
)

// QuotaPeriod represents the time period for a quota
type QuotaPeriod string

const (
	// QuotaPeriodMinute represents a per-minute quota
	QuotaPeriodMinute QuotaPeriod = "minute"
	// QuotaPeriodHour represents an hourly quota
	QuotaPeriodHour QuotaPeriod = "hour"
	// QuotaPeriodDay represents a daily quota
	QuotaPeriodDay QuotaPeriod = "day"
	// QuotaPeriodWeek represents a weekly quota
	QuotaPeriodWeek QuotaPeriod = "week"
	// QuotaPeriodMonth represents a monthly quota
	QuotaPeriodMonth QuotaPeriod = "month"
	// QuotaPeriodCustom represents a custom time period
	QuotaPeriodCustom QuotaPeriod = "custom"
)

// QuotaConfig represents a single quota configuration
type QuotaConfig struct {
	// Type is the type of quota (requests, tokens, etc.)
	Type QuotaType `json:"type"`

	// Period is the time period for this quota
	Period QuotaPeriod `json:"period"`

	// Limit is the maximum allowed value for this quota
	Limit int `json:"limit"`

	// ResetAt is when this quota will reset
	ResetAt time.Time `json:"reset_at"`

	// CustomData holds provider-specific quota configuration
	CustomData map[string]interface{} `json:"custom_data,omitempty"`
}

// QuotaUsage represents current usage for a quota
type QuotaUsage struct {
	// Type is the type of quota this usage represents
	Type QuotaType `json:"type"`

	// Period is the time period for this quota
	Period QuotaPeriod `json:"period"`

	// Used is the amount of quota currently used
	Used int `json:"used"`

	// Limit is the maximum allowed value for this quota
	Limit int `json:"limit"`

	// Remaining is the amount of quota remaining (Limit - Used)
	Remaining int `json:"remaining"`

	// RemainingPercent is the percentage of quota remaining (0-100)
	RemainingPercent float64 `json:"remaining_percent"`

	// ResetAt is when this quota will reset
	ResetAt time.Time `json:"reset_at"`

	// PeriodStartedAt is when the current quota period started
	PeriodStartedAt time.Time `json:"period_started_at"`
}

// IsExpired returns true if the quota has expired and should be reset
func (q *QuotaUsage) IsExpired() bool {
	return !q.ResetAt.IsZero() && time.Now().After(q.ResetAt)
}

// UsageRatio returns the ratio of usage (0 = none used, 1 = fully used)
func (q *QuotaUsage) UsageRatio() float64 {
	if q.Limit <= 0 {
		return 0
	}
	return float64(q.Used) / float64(q.Limit)
}

// QuotaInfo represents comprehensive quota information for a provider/model
type QuotaInfo struct {
	// Provider is the name of the AI provider
	Provider string `json:"provider"`

	// ProviderType is the type of the provider
	ProviderType types.ProviderType `json:"provider_type"`

	// Model is the model identifier
	Model string `json:"model"`

	// Timestamp is when this quota information was captured
	Timestamp time.Time `json:"timestamp"`

	// Quotas is a map of quota types to their current usage
	Quotas map[QuotaType]*QuotaUsage `json:"quotas"`

	// ProviderQuotaConfigs is a map of configured quota limits
	ProviderQuotaConfigs map[QuotaType]*QuotaConfig `json:"provider_quota_configs,omitempty"`

	// CustomUsage holds provider-specific usage information
	CustomUsage map[string]interface{} `json:"custom_usage,omitempty"`

	// Metadata holds additional quota metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Clone creates a deep copy of QuotaInfo
func (q *QuotaInfo) Clone() *QuotaInfo {
	if q == nil {
		return nil
	}

	clone := &QuotaInfo{
		Provider:             q.Provider,
		ProviderType:         q.ProviderType,
		Model:                q.Model,
		Timestamp:            q.Timestamp,
		Quotas:               make(map[QuotaType]*QuotaUsage, len(q.Quotas)),
		ProviderQuotaConfigs: make(map[QuotaType]*QuotaConfig, len(q.ProviderQuotaConfigs)),
		CustomUsage:          make(map[string]interface{}, len(q.CustomUsage)),
		Metadata:             make(map[string]interface{}, len(q.Metadata)),
	}

	for k, v := range q.Quotas {
		clone.Quotas[k] = &QuotaUsage{
			Type:             v.Type,
			Period:           v.Period,
			Used:             v.Used,
			Limit:            v.Limit,
			Remaining:        v.Remaining,
			RemainingPercent: v.RemainingPercent,
			ResetAt:          v.ResetAt,
			PeriodStartedAt:  v.PeriodStartedAt,
		}
	}

	for k, v := range q.ProviderQuotaConfigs {
		clone.ProviderQuotaConfigs[k] = &QuotaConfig{
			Type:       v.Type,
			Period:     v.Period,
			Limit:      v.Limit,
			ResetAt:    v.ResetAt,
			CustomData: make(map[string]interface{}),
		}
		for ck, cv := range v.CustomData {
			clone.ProviderQuotaConfigs[k].CustomData[ck] = cv
		}
	}

	for k, v := range q.CustomUsage {
		clone.CustomUsage[k] = v
	}

	for k, v := range q.Metadata {
		clone.Metadata[k] = v
	}

	return clone
}

// GetQuota returns the usage for a specific quota type
func (q *QuotaInfo) GetQuota(quotaType QuotaType) (*QuotaUsage, bool) {
	if q == nil {
		return nil, false
	}
	usage, exists := q.Quotas[quotaType]
	return usage, exists
}

// HasQuota returns true if the specified quota type exists and has available capacity
func (q *QuotaInfo) HasQuota(quotaType QuotaType) bool {
	usage, exists := q.GetQuota(quotaType)
	if !exists {
		return true // No quota configured, assume unlimited
	}
	return usage.Remaining > 0 && !usage.IsExpired()
}

// AnyQuotaExceeded returns true if any quota has been exceeded
func (q *QuotaInfo) AnyQuotaExceeded() bool {
	if q == nil {
		return false
	}
	for _, usage := range q.Quotas {
		if usage.Remaining <= 0 && !usage.IsExpired() {
			return true
		}
	}
	return false
}

// QuotaRecord represents a historical record of quota usage
type QuotaRecord struct {
	// ID is the unique identifier for this record
	ID string `json:"id"`

	// Provider is the name of the AI provider
	Provider string `json:"provider"`

	// Model is the model identifier
	Model string `json:"model"`

	// Timestamp is when this record was created
	Timestamp time.Time `json:"timestamp"`

	// Operation is the type of operation that consumed quota
	Operation string `json:"operation"`

	// Usage is the quota usage for this record
	Usage map[QuotaType]int `json:"usage"`
}

// QuotaHistory represents a collection of quota usage records
type QuotaHistory struct {
	// Provider is the name of the AI provider
	Provider string `json:"provider"`

	// Model is the model identifier (empty for all models)
	Model string `json:"model,omitempty"`

	// Records is the list of quota usage records
	Records []*QuotaRecord `json:"records"`

	// Total counts aggregate usage across all records
	TotalUsage map[QuotaType]int `json:"total_usage"`

	// StartTime is the start of the time period for this history
	StartTime time.Time `json:"start_time"`

	// EndTime is the end of the time period for this history
	EndTime time.Time `json:"end_time"`
}

// ============================================================================
// QuotaProvider Interface
// ============================================================================

// QuotaProvider defines the interface for providers that can report their quota information
//
// This is an optional interface that providers can implement to provide real-time
// quota information. Providers that don't implement this interface can still
// work with the quota system through rate limit header parsing.
type QuotaProvider interface {
	// GetQuota returns the current quota information for the provider
	// This method should fetch real-time quota information from the provider's API
	GetQuota(ctx context.Context, model string) (*QuotaInfo, error)

	// GetQuotaHistory returns historical quota usage for the provider
	// Parameters:
	//   - model: The model identifier (empty for all models)
	//   - startTime: Start of the time range
	//   - endTime: End of the time range
	GetQuotaHistory(ctx context.Context, model string, startTime, endTime time.Time) (*QuotaHistory, error)

	// SupportsQuotaReporting returns true if the provider supports real-time quota reporting
	SupportsQuotaReporting() bool
}

// QuotaParser defines the interface for parsing quota information from HTTP headers
//
// This allows each provider to implement their own header parsing logic
// while providing a unified quota information structure.
type QuotaParser interface {
	// ParseQuota extracts quota information from HTTP response headers
	ParseQuota(headers http.Header, provider string, model string) (*QuotaInfo, error)

	// ProviderName returns the name of the provider this parser handles
	ProviderName() string
}

// ============================================================================
// Quota Manager
// ============================================================================

// Manager manages quota information for multiple providers
type Manager struct {
	mu sync.RWMutex

	// quotas stores current quota information keyed by provider:model
	quotas map[string]*QuotaInfo

	// parsers stores quota parsers keyed by provider type
	parsers map[types.ProviderType]QuotaParser

	// providers stores QuotaProvider implementations
	providers map[string]QuotaProvider

	// history stores historical quota usage
	history []*QuotaRecord

	// historyMaxSize is the maximum number of history records to keep
	historyMaxSize int
}

// NewManager creates a new quota manager
func NewManager() *Manager {
	return &Manager{
		quotas:         make(map[string]*QuotaInfo),
		parsers:        make(map[types.ProviderType]QuotaParser),
		providers:      make(map[string]QuotaProvider),
		history:        make([]*QuotaRecord, 0),
		historyMaxSize: 1000,
	}
}

// NewManagerWithHistory creates a new quota manager with a specific history size
func NewManagerWithHistory(historyMaxSize int) *Manager {
	return &Manager{
		quotas:         make(map[string]*QuotaInfo),
		parsers:        make(map[types.ProviderType]QuotaParser),
		providers:      make(map[string]QuotaProvider),
		history:        make([]*QuotaRecord, 0),
		historyMaxSize: historyMaxSize,
	}
}

// RegisterParser registers a quota parser for a provider type
func (m *Manager) RegisterParser(providerType types.ProviderType, parser QuotaParser) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.parsers[providerType] = parser
}

// RegisterProvider registers a quota provider
func (m *Manager) RegisterProvider(name string, provider QuotaProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[name] = provider
}

// UpdateFromRateLimit updates quota information from rate limit info
func (m *Manager) UpdateFromRateLimit(info *ratelimit.Info) *QuotaInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	quotaKey := info.Provider + ":" + info.Model

	quotaInfo := &QuotaInfo{
		Provider:    info.Provider,
		Model:       info.Model,
		Timestamp:   info.Timestamp,
		Quotas:      make(map[QuotaType]*QuotaUsage),
		CustomUsage: info.CustomData,
		Metadata:    make(map[string]interface{}),
	}

	// Populate Quotas from rate limit info
	if info.RequestsLimit > 0 {
		quotaInfo.Quotas[QuotaTypeRequests] = &QuotaUsage{
			Type:             QuotaTypeRequests,
			Period:           QuotaPeriodMinute, // Most request quotas are per-minute
			Used:             info.RequestsLimit - info.RequestsRemaining,
			Limit:            info.RequestsLimit,
			Remaining:        info.RequestsRemaining,
			RemainingPercent: percentage(info.RequestsRemaining, info.RequestsLimit),
			ResetAt:          info.RequestsReset,
		}
	}

	if info.TokensLimit > 0 {
		quotaInfo.Quotas[QuotaTypeTokens] = &QuotaUsage{
			Type:             QuotaTypeTokens,
			Period:           QuotaPeriodMinute,
			Used:             info.TokensLimit - info.TokensRemaining,
			Limit:            info.TokensLimit,
			Remaining:        info.TokensRemaining,
			RemainingPercent: percentage(info.TokensRemaining, info.TokensLimit),
			ResetAt:          info.TokensReset,
		}
	}

	// Add Anthropic-specific quotas
	if info.InputTokensLimit > 0 {
		quotaInfo.Quotas[QuotaTypeInputTokens] = &QuotaUsage{
			Type:             QuotaTypeInputTokens,
			Period:           QuotaPeriodMinute,
			Used:             info.InputTokensLimit - info.InputTokensRemaining,
			Limit:            info.InputTokensLimit,
			Remaining:        info.InputTokensRemaining,
			RemainingPercent: percentage(info.InputTokensRemaining, info.InputTokensLimit),
			ResetAt:          info.InputTokensReset,
		}
	}

	if info.OutputTokensLimit > 0 {
		quotaInfo.Quotas[QuotaTypeOutputTokens] = &QuotaUsage{
			Type:             QuotaTypeOutputTokens,
			Period:           QuotaPeriodMinute,
			Used:             info.OutputTokensLimit - info.OutputTokensRemaining,
			Limit:            info.OutputTokensLimit,
			Remaining:        info.OutputTokensRemaining,
			RemainingPercent: percentage(info.OutputTokensRemaining, info.OutputTokensLimit),
			ResetAt:          info.OutputTokensReset,
		}
	}

	// Add Cerebras daily quotas
	if info.DailyRequestsLimit > 0 {
		quotaInfo.Quotas[QuotaTypeDaily] = &QuotaUsage{
			Type:             QuotaTypeDaily,
			Period:           QuotaPeriodDay,
			Used:             info.DailyRequestsLimit - info.DailyRequestsRemaining,
			Limit:            info.DailyRequestsLimit,
			Remaining:        info.DailyRequestsRemaining,
			RemainingPercent: percentage(info.DailyRequestsRemaining, info.DailyRequestsLimit),
			ResetAt:          info.DailyRequestsReset,
		}
	}

	// Add OpenRouter credits as custom quota
	if info.CreditsLimit > 0 {
		quotaInfo.Quotas[QuotaTypeCustom] = &QuotaUsage{
			Type:             QuotaTypeCustom,
			Period:           QuotaPeriodCustom,
			Used:             int(info.CreditsLimit - info.CreditsRemaining),
			Limit:            int(info.CreditsLimit),
			Remaining:        int(info.CreditsRemaining),
			RemainingPercent: (info.CreditsRemaining / info.CreditsLimit) * 100,
			ResetAt:          time.Time{}, // Credits don't typically reset
		}

		quotaInfo.CustomUsage["is_free_tier"] = info.IsFreeTier
	}

	m.quotas[quotaKey] = quotaInfo

	return quotaInfo
}

// GetQuota retrieves quota information for a provider/model
func (m *Manager) GetQuota(provider, model string) (*QuotaInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quotaKey := provider + ":" + model
	quota, exists := m.quotas[quotaKey]
	if !exists {
		return nil, false
	}

	// Return a clone to prevent external modification
	return quota.Clone(), true
}

// GetAllQuotas returns all quota information
func (m *Manager) GetAllQuotas() map[string]*QuotaInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*QuotaInfo, len(m.quotas))
	for k, v := range m.quotas {
		result[k] = v.Clone()
	}

	return result
}

// FetchQuota fetches real-time quota information from a provider
// This only works for providers that implement QuotaProvider
func (m *Manager) FetchQuota(ctx context.Context, provider, model string) (*QuotaInfo, error) {
	m.mu.RLock()
	quotaProvider, exists := m.providers[provider]
	m.mu.RUnlock()

	if !exists || !quotaProvider.SupportsQuotaReporting() {
		// Provider doesn't support real-time quota reporting
		// Return cached quota if available
		if quota, ok := m.GetQuota(provider, model); ok {
			return quota, nil
		}
		return nil, &QuotaNotSupportedError{Provider: provider}
	}

	quotaInfo, err := quotaProvider.GetQuota(ctx, model)
	if err != nil {
		return nil, err
	}

	// Update the cached quota
	m.mu.Lock()
	key := provider + ":" + model
	m.quotas[key] = quotaInfo
	m.mu.Unlock()

	return quotaInfo.Clone(), nil
}

// RecordUsage records a usage event in the history
func (m *Manager) RecordUsage(provider, model, operation string, usage map[QuotaType]int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := &QuotaRecord{
		ID:        generateID(),
		Provider:  provider,
		Model:     model,
		Timestamp: time.Now(),
		Operation: operation,
		Usage:     usage,
	}

	m.history = append(m.history, record)

	// Trim history if it exceeds max size
	if len(m.history) > m.historyMaxSize {
		m.history = m.history[1:]
	}
}

// GetHistory returns historical usage records
func (m *Manager) GetHistory(provider, model string, limit int) []*QuotaRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*QuotaRecord, 0)

	if provider == "" && model == "" {
		// Return all records
		result = append(result, m.history...)
	} else {
		// Filter by provider and/or model
		for _, record := range m.history {
			if provider != "" && record.Provider != provider {
				continue
			}
			if model != "" && record.Model != model {
				continue
			}
			result = append(result, record)
		}
	}

	// Apply limit
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}

	return result
}

// ClearHistory clears all historical usage records
func (m *Manager) ClearHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = make([]*QuotaRecord, 0)
}

// RemoveQuota removes quota information for a provider/model
func (m *Manager) RemoveQuota(provider, model string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := provider + ":" + model
	delete(m.quotas, key)
}

// ClearAll clears all quota information and history
func (m *Manager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.quotas = make(map[string]*QuotaInfo)
	m.history = make([]*QuotaRecord, 0)
}

// GetProviderQuotas returns all quotas for a specific provider
func (m *Manager) GetProviderQuotas(provider string) map[string]*QuotaInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*QuotaInfo)
	for key, quota := range m.quotas {
		if key == provider || startsWith(key, provider+":") {
			result[key] = quota.Clone()
		}
	}

	return result
}

// Summary returns a summary of all quotas
func (m *Manager) Summary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := make(map[string]interface{})

	providers := make(map[string][]string)
	modelsPerProvider := make(map[string]int)

	for _, quota := range m.quotas {
		provider := quota.Provider
		model := quota.Model

		providers[provider] = append(providers[provider], model)
		modelsPerProvider[provider]++
	}

	summary["total_providers"] = len(providers)
	summary["provider_models"] = providers
	summary["history_records"] = len(m.history)

	// Count quotas by type
	quotaTypes := make(map[QuotaType]int)
	for _, quota := range m.quotas {
		for quotaType := range quota.Quotas {
			quotaTypes[quotaType]++
		}
	}

	summary["quota_types"] = quotaTypes

	return summary
}

// ============================================================================
// Helper Functions
// ============================================================================

// percentage calculates the percentage of remaining quota
func percentage(remaining, limit int) float64 {
	if limit <= 0 {
		return 0
	}
	return float64(remaining) / float64(limit) * 100
}

// generateID generates a unique ID for a quota record
func generateID() string {
	// Simple ID generation using timestamp
	return "qr_" + time.Now().Format("20060102150405") + "_" +
		time.Now().Format(".000000000")
}

// startsWith checks if a string starts with a prefix
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ============================================================================
// Errors
// ============================================================================

// QuotaNotSupportedError is returned when a provider doesn't support quota reporting
type QuotaNotSupportedError struct {
	Provider string
}

func (e *QuotaNotSupportedError) Error() string {
	return "provider '" + e.Provider + "' does not support quota reporting"
}

// IsQuotaNotSupported checks if an error is a QuotaNotSupportedError
func IsQuotaNotSupported(err error) bool {
	_, ok := err.(*QuotaNotSupportedError)
	return ok
}

// QuotaExceededError is returned when attempting to exceed quota limits
type QuotaExceededError struct {
	Provider  string
	Model     string
	QuotaType QuotaType
	Limit     int
	Used      int
}

func (e *QuotaExceededError) Error() string {
	return "quota exceeded for provider '" + e.Provider + "', model '" + e.Model +
		"', type '" + string(e.QuotaType) + "': " +
		strconv.Itoa(e.Used) + "/" + strconv.Itoa(e.Limit) + " used"
}

// IsQuotaExceeded checks if an error is a QuotaExceededError
func IsQuotaExceeded(err error) bool {
	_, ok := err.(*QuotaExceededError)
	return ok
}
