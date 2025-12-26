package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/quota"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// QuotaHandler manages quota-related API endpoints
type QuotaHandler struct {
	providers    map[string]types.Provider
	quotaManager *quota.Manager
}

// NewQuotaHandler creates a new quota handler
func NewQuotaHandler(providers map[string]types.Provider, quotaManager *quota.Manager) *QuotaHandler {
	return &QuotaHandler{
		providers:    providers,
		quotaManager: quotaManager,
	}
}

// GetQuota returns quota information for a specific provider/model
// GET /api/quota
func (h *QuotaHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req backendtypes.QuotaRequest
	if err := ParseJSON(r, &req); err != nil {
		// If body is empty, try query parameters
		req.Provider = r.URL.Query().Get("provider")
		req.Model = r.URL.Query().Get("model")
		if req.Provider == "" {
			SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
			return
		}
	}

	if req.Provider == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	// Check that the provider exists
	if _, exists := h.providers[req.Provider]; !exists {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", req.Provider), http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var quotaInfo *quota.QuotaInfo
	var err error

	if req.FetchFromProvider {
		quotaInfo, err = h.quotaManager.FetchQuota(ctx, req.Provider, req.Model)
	} else {
		quotaInfo, _ = h.quotaManager.GetQuota(req.Provider, req.Model)
	}

	if err != nil {
		if quota.IsQuotaNotSupported(err) {
			// Return cached quota if available, or empty quota info
			if cachedInfo, ok := h.quotaManager.GetQuota(req.Provider, req.Model); ok {
				quotaInfo = cachedInfo
			} else {
				quotaInfo = &quota.QuotaInfo{
					Provider:    req.Provider,
					Model:       req.Model,
					Timestamp:   time.Now(),
					Quotas:      make(map[quota.QuotaType]*quota.QuotaUsage),
					CustomUsage: make(map[string]interface{}),
					Metadata:    make(map[string]interface{}),
				}
			}
		} else {
			SendError(w, r, "QUOTA_ERROR", fmt.Sprintf("Failed to get quota: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := h.buildQuotaResponse(quotaInfo)
	SendSuccess(w, r, response)
}

// GetAllQuotas returns all quota information
// GET /api/quota/all
func (h *QuotaHandler) GetAllQuotas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	allQuotas := h.quotaManager.GetAllQuotas()
	quotaResponses := make(map[string]*backendtypes.QuotaResponse, len(allQuotas))

	for key, quotaInfo := range allQuotas {
		quotaResponses[key] = h.buildQuotaResponse(quotaInfo)
	}

	// Build summary
	summary := h.buildQuotaSummary(h.quotaManager.Summary())

	response := &backendtypes.QuotaAllResponse{
		Quotas:  quotaResponses,
		Summary: summary,
	}

	SendSuccess(w, r, response)
}

// GetQuotaHistory returns historical quota usage
// GET /api/quota/history
func (h *QuotaHandler) GetQuotaHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req backendtypes.QuotaHistoryRequest

	// Try parsing from query parameters
	req.Provider = r.URL.Query().Get("provider")
	req.Model = r.URL.Query().Get("model")
	req.Limit = parseOptionalInt(r.URL.Query(), "limit", 100)

	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if startTime, err := parseUnixTimestamp(startTimeStr); err == nil {
			req.StartTime = startTime
		}
	}

	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		if endTime, err := parseUnixTimestamp(endTimeStr); err == nil {
			req.EndTime = endTime
		}
	}

	// Try parsing from body if query params are empty
	if req.Provider == "" {
		if err := ParseJSON(r, &req); err != nil {
			// Return error if both query params and body parsing fail
			SendError(w, r, "INVALID_REQUEST", fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}
	}

	records := h.quotaManager.GetHistory(req.Provider, req.Model, req.Limit)

	response := h.buildQuotaHistoryResponse(records, req.Provider, req.Model)
	SendSuccess(w, r, response)
}

// GetQuotaSummary returns a summary of all quotas
// GET /api/quota/summary
func (h *QuotaHandler) GetQuotaSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Optional provider filter
	var req backendtypes.QuotaSummaryRequest
	if provider := r.URL.Query().Get("provider"); provider != "" {
		req.Provider = provider
	}

	summary := h.quotaManager.Summary()
	response := h.buildQuotaSummary(summary)
	SendSuccess(w, r, response)
}

// GetQuotaHealth returns the health status of quota tracking
// GET /api/quota/health
func (h *QuotaHandler) GetQuotaHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	allQuotas := h.quotaManager.GetAllQuotas()
	providersTracked := len(allQuotas)

	var lastUpdate int64
	for _, quotaInfo := range allQuotas {
		if quotaInfo.Timestamp.Unix() > lastUpdate {
			lastUpdate = quotaInfo.Timestamp.Unix()
		}
	}

	response := &backendtypes.QuotaHealthResponse{
		Healthy:          true,
		Message:          "Quota tracking is operational",
		ProvidersTracked: providersTracked,
		LastUpdate:       lastUpdate,
	}

	SendSuccess(w, r, response)
}

// ClearQuota clears quota information for a specific provider/model
// DELETE /api/quota
func (h *QuotaHandler) ClearQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only DELETE method is allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := r.URL.Query().Get("provider")
	model := r.URL.Query().Get("model")

	if provider == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	if model == "" {
		// Clear all quotas for this provider
		providerQuotas := h.quotaManager.GetProviderQuotas(provider)
		count := len(providerQuotas)
		for key := range providerQuotas {
			p, m := extractProviderModel(key)
			h.quotaManager.RemoveQuota(p, m)
		}

		SendSuccess(w, r, map[string]interface{}{
			"message":  fmt.Sprintf("Cleared %d quota entries for provider '%s'", count, provider),
			"provider": provider,
			"count":    count,
		})
		return
	}

	h.quotaManager.RemoveQuota(provider, model)

	SendSuccess(w, r, map[string]interface{}{
		"message":  fmt.Sprintf("Cleared quota for provider '%s', model '%s'", provider, model),
		"provider": provider,
		"model":    model,
	})
}

// ClearQuotaHistory clears all quota history
// DELETE /api/quota/history
func (h *QuotaHandler) ClearQuotaHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only DELETE method is allowed", http.StatusMethodNotAllowed)
		return
	}

	h.quotaManager.ClearHistory()

	SendSuccess(w, r, map[string]interface{}{
		"message": "Quota history cleared",
	})
}

// RecordUsage records a usage event in the quota history
// POST /api/quota/record
func (h *QuotaHandler) RecordUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Provider  string         `json:"provider"`
		Model     string         `json:"model"`
		Operation string         `json:"operation"`
		Usage     map[string]int `json:"usage"`
	}

	if err := ParseJSON(r, &req); err != nil {
		SendError(w, r, "INVALID_REQUEST", fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Provider == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	typeMap := make(map[quota.QuotaType]int)
	for k, v := range req.Usage {
		typeMap[quota.QuotaType(k)] = v
	}

	h.quotaManager.RecordUsage(req.Provider, req.Model, req.Operation, typeMap)

	SendSuccess(w, r, map[string]interface{}{
		"message":   "Usage recorded successfully",
		"provider":  req.Provider,
		"model":     req.Model,
		"operation": req.Operation,
		"usage":     req.Usage,
	})
}

// GetProviderQuotas returns all quotas for a specific provider
// GET /api/quota/provider/{name}
func (h *QuotaHandler) GetProviderQuotas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := h.extractProviderName(r)
	if provider == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	if _, exists := h.providers[provider]; !exists {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", provider), http.StatusNotFound)
		return
	}

	providerQuotas := h.quotaManager.GetProviderQuotas(provider)
	quotaResponses := make(map[string]*backendtypes.QuotaResponse, len(providerQuotas))

	for key, quotaInfo := range providerQuotas {
		quotaResponses[key] = h.buildQuotaResponse(quotaInfo)
	}

	SendSuccess(w, r, quotaResponses)
}

// ============================================================================
// Helper functions
// ============================================================================

// buildQuotaResponse converts QuotaInfo to QuotaResponse
func (h *QuotaHandler) buildQuotaResponse(qi *quota.QuotaInfo) *backendtypes.QuotaResponse {
	if qi == nil {
		return nil
	}

	response := &backendtypes.QuotaResponse{
		Provider:             qi.Provider,
		ProviderType:         string(qi.ProviderType),
		Model:                qi.Model,
		Timestamp:            qi.Timestamp.Unix(),
		Quotas:               make(map[string]*backendtypes.QuotaUsageResponse, len(qi.Quotas)),
		ProviderQuotaConfigs: make(map[string]*backendtypes.QuotaConfigResponse, len(qi.ProviderQuotaConfigs)),
		CustomUsage:          qi.CustomUsage,
		Metadata:             qi.Metadata,
		AnyQuotaExceeded:     qi.AnyQuotaExceeded(),
		Healthy:              !qi.AnyQuotaExceeded(),
	}

	for quotaType, usage := range qi.Quotas {
		response.Quotas[string(quotaType)] = &backendtypes.QuotaUsageResponse{
			Type:             string(usage.Type),
			Period:           string(usage.Period),
			Used:             usage.Used,
			Limit:            usage.Limit,
			Remaining:        usage.Remaining,
			RemainingPercent: usage.RemainingPercent,
			ResetAt:          usage.ResetAt.Unix(),
			PeriodStartedAt:  usage.PeriodStartedAt.Unix(),
		}
	}

	for quotaType, config := range qi.ProviderQuotaConfigs {
		response.ProviderQuotaConfigs[string(quotaType)] = &backendtypes.QuotaConfigResponse{
			Type:       string(config.Type),
			Period:     string(config.Period),
			Limit:      config.Limit,
			ResetAt:    config.ResetAt.Unix(),
			CustomData: config.CustomData,
		}
	}

	return response
}

// buildQuotaHistoryResponse converts history records to QuotaHistoryResponse
func (h *QuotaHandler) buildQuotaHistoryResponse(records []*quota.QuotaRecord, provider, model string) *backendtypes.QuotaHistoryResponse {
	if len(records) == 0 {
		return &backendtypes.QuotaHistoryResponse{
			Provider:    provider,
			Model:       model,
			Records:     []*backendtypes.QuotaRecordResponse{},
			TotalUsage:  make(map[string]int),
			StartTime:   0,
			EndTime:     time.Now().Unix(),
			RecordCount: 0,
		}
	}

	responseRecords := make([]*backendtypes.QuotaRecordResponse, len(records))
	totalUsage := make(map[string]int)

	var startTime, endTime int64
	startTime = records[0].Timestamp.Unix()
	endTime = records[len(records)-1].Timestamp.Unix()

	for i, record := range records {
		recordTime := record.Timestamp.Unix()
		if recordTime < startTime {
			startTime = recordTime
		}
		if recordTime > endTime {
			endTime = recordTime
		}

		usageStrings := make(map[string]int)
		for k, v := range record.Usage {
			usageStrings[string(k)] = v
			totalUsage[string(k)] += v
		}

		responseRecords[i] = &backendtypes.QuotaRecordResponse{
			ID:        record.ID,
			Provider:  record.Provider,
			Model:     record.Model,
			Timestamp: recordTime,
			Operation: record.Operation,
			Usage:     usageStrings,
		}
	}

	return &backendtypes.QuotaHistoryResponse{
		Provider:    provider,
		Model:       model,
		Records:     responseRecords,
		TotalUsage:  totalUsage,
		StartTime:   startTime,
		EndTime:     endTime,
		RecordCount: len(records),
	}
}

// buildQuotaSummary converts summary to QuotaSummaryResponse
func (h *QuotaHandler) buildQuotaSummary(summary map[string]interface{}) *backendtypes.QuotaSummaryResponse {
	response := &backendtypes.QuotaSummaryResponse{
		ProviderModels: make(map[string][]string),
		QuotaTypes:     make(map[string]int),
	}

	if providers, ok := summary["total_providers"].(int); ok {
		response.TotalProviders = providers
	}

	if providerModels, ok := summary["provider_models"].(map[string][]string); ok {
		response.ProviderModels = providerModels
	}

	if historyRecords, ok := summary["history_records"].(int); ok {
		response.HistoryRecords = historyRecords
	}

	if quotaTypes, ok := summary["quota_types"].(map[quota.QuotaType]int); ok {
		newQuotaTypes := make(map[string]int)
		for k, v := range quotaTypes {
			newQuotaTypes[string(k)] = v
		}
		response.QuotaTypes = newQuotaTypes
	}

	return response
}

// extractProviderName extracts provider name from the URL path
func (h *QuotaHandler) extractProviderName(r *http.Request) string {
	// Try query parameter first
	if name := r.URL.Query().Get("provider"); name != "" {
		return name
	}

	// Try path extraction
	path := r.URL.Path
	// Expected pattern: /api/quota/provider/{name}
	if len(path) > len("/api/quota/provider/") {
		return path[len("/api/quota/provider/"):]
	}

	return ""
}

// parseOptionalInt parses an optional integer from query parameters
func parseOptionalInt(params map[string][]string, key string, defaultValue int) int {
	values := params[key]
	if len(values) == 0 {
		return defaultValue
	}

	var result int
	if _, err := fmt.Sscanf(values[0], "%d", &result); err != nil {
		return defaultValue
	}

	return result
}

// parseUnixTimestamp parses a Unix timestamp string
func parseUnixTimestamp(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// extractProviderModel extracts provider and model from a combined key
func extractProviderModel(key string) (provider, model string) {
	for i, c := range key {
		if c == ':' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}
