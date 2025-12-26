package backendtypes

import "github.com/cecil-the-coder/ai-provider-kit/pkg/types"

// QuotaRequest represents a request for quota information
type QuotaRequest struct {
	// Provider is the name of the AI provider
	Provider string `json:"provider"`

	// Model is the model identifier (optional - if empty, returns provider-wide quota)
	Model string `json:"model,omitempty"`

	// FetchFromProvider if true, fetches real-time quota from the provider API
	// if the provider supports it. If false, returns cached quota information.
	FetchFromProvider bool `json:"fetch_from_provider,omitempty"`
}

// QuotaUsageResponse represents quota usage in API responses
type QuotaUsageResponse struct {
	Type             string  `json:"type"`   // "requests", "tokens", "input_tokens", "output_tokens", "daily", "custom"
	Period           string  `json:"period"` // "minute", "hour", "day", "week", "month", "custom"
	Used             int     `json:"used"`
	Limit            int     `json:"limit"`
	Remaining        int     `json:"remaining"`
	RemainingPercent float64 `json:"remaining_percent"`
	ResetAt          int64   `json:"reset_at"`          // Unix timestamp
	PeriodStartedAt  int64   `json:"period_started_at"` // Unix timestamp
}

// QuotaConfigResponse represents quota configuration in API responses
type QuotaConfigResponse struct {
	Type       string                 `json:"type"`
	Period     string                 `json:"period"`
	Limit      int                    `json:"limit"`
	ResetAt    int64                  `json:"reset_at"` // Unix timestamp
	CustomData map[string]interface{} `json:"custom_data,omitempty"`
}

// QuotaResponse represents quota information in API responses
type QuotaResponse struct {
	Provider             string                          `json:"provider"`
	ProviderType         string                          `json:"provider_type"`
	Model                string                          `json:"model"`
	Timestamp            int64                           `json:"timestamp"` // Unix timestamp
	Quotas               map[string]*QuotaUsageResponse  `json:"quotas"`
	ProviderQuotaConfigs map[string]*QuotaConfigResponse `json:"provider_quota_configs,omitempty"`
	CustomUsage          map[string]interface{}          `json:"custom_usage,omitempty"`
	Metadata             map[string]interface{}          `json:"metadata,omitempty"`

	// Computed fields
	AnyQuotaExceeded bool `json:"any_quota_exceeded"`
	Healthy          bool `json:"healthy"`
}

// QuotaHistoryRequest represents a request for quota history
type QuotaHistoryRequest struct {
	// Provider is the name of the AI provider
	Provider string `json:"provider"`

	// Model is the model identifier (optional - if empty, returns all models)
	Model string `json:"model,omitempty"`

	// StartTime is the start of the time range (Unix timestamp)
	StartTime int64 `json:"start_time,omitempty"`

	// EndTime is the end of the time range (Unix timestamp)
	EndTime int64 `json:"end_time,omitempty"`

	// Limit is the maximum number of records to return
	Limit int `json:"limit,omitempty"`
}

// QuotaRecordResponse represents a quota usage record in API responses
type QuotaRecordResponse struct {
	ID        string         `json:"id"`
	Provider  string         `json:"provider"`
	Model     string         `json:"model"`
	Timestamp int64          `json:"timestamp"` // Unix timestamp
	Operation string         `json:"operation"`
	Usage     map[string]int `json:"usage"`
}

// QuotaHistoryResponse represents historical quota usage in API responses
type QuotaHistoryResponse struct {
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model,omitempty"`
	Records     []*QuotaRecordResponse `json:"records"`
	TotalUsage  map[string]int         `json:"total_usage"`
	StartTime   int64                  `json:"start_time"` // Unix timestamp
	EndTime     int64                  `json:"end_time"`   // Unix timestamp
	RecordCount int                    `json:"record_count"`
}

// QuotaSummaryRequest represents a request for quota summary
type QuotaSummaryRequest struct {
	// Provider filters to a specific provider (optional)
	Provider string `json:"provider,omitempty"`
}

// QuotaSummaryResponse represents a summary of all quotas
type QuotaSummaryResponse struct {
	// TotalProviders is the number of providers with quota data
	TotalProviders int `json:"total_providers"`

	// ProviderModels maps provider names to their models
	ProviderModels map[string][]string `json:"provider_models"`

	// HistoryRecords is the total number of history records
	HistoryRecords int `json:"history_records"`

	// QuotaTypes counts quota types across all providers
	QuotaTypes map[string]int `json:"quota_types"`
}

// QuotaAllResponse represents a response containing all quota information
type QuotaAllResponse struct {
	// Quotas maps provider:model to quota information
	Quotas map[string]*QuotaResponse `json:"quotas"`

	// Summary provides overall summary
	Summary *QuotaSummaryResponse `json:"summary"`
}

// RegisterQuotaProviderRequest represents a request to register a quota provider
type RegisterQuotaProviderRequest struct {
	// Name is the unique name for the quota provider
	Name string `json:"name"`

	// ProviderType is the type of the AI provider
	ProviderType types.ProviderType `json:"provider_type"`
}

// SetQuotaRequest represents a request to set quota limits
type SetQuotaRequest struct {
	// Provider is the name of the AI provider
	Provider string `json:"provider"`

	// Model is the model identifier (optional - if empty, applies to all models)
	Model string `json:"model,omitempty"`

	// Quotas is a map of quota type to limit value
	Quotas map[string]int `json:"quotas"`

	// ResetTime is the reset time for the quota (Unix timestamp, optional)
	ResetTime int64 `json:"reset_time,omitempty"`
}

// QuotaHealthResponse represents the health status of quota tracking
type QuotaHealthResponse struct {
	// Healthy indicates if quota tracking is operational
	Healthy bool `json:"healthy"`

	// Message provides additional health information
	Message string `json:"message,omitempty"`

	// ProvidersTracked is the number of providers being tracked
	ProvidersTracked int `json:"providers_tracked"`

	// LastUpdate is when quota information was last updated
	LastUpdate int64 `json:"last_update,omitempty"` // Unix timestamp
}
