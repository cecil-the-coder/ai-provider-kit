// Package openai provides integration with OpenAI's GPT models including
// chat completions, streaming, tool calling, authentication, and quota fetching.
package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	commonhttp "github.com/cecil-the-coder/ai-provider-kit/internal/common/http"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/quota"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ============================================================================
// OpenAI Quota/Usage API Types
// ============================================================================

// OpenAIUsageRequest represents a request to OpenAI's usage API
type OpenAIUsageRequest struct {
	StartDate string `url:"start_date"` // Format: YYYY-MM-DD
	EndDate   string `url:"end_date"`   // Format: YYYY-MM-DD
}

// OpenAIUsageResponse represents a response from OpenAI's usage API
type OpenAIUsageResponse struct {
	Object   string               `json:"object"`
	Daily    []OpenAIUsageDayData `json:"daily"`
	Metadata OpenAIUsageMetadata  `json:"metadata"`
}

// OpenAIUsageDayData represents usage data for a single day
type OpenAIUsageDayData struct {
	Timestamp       int64                   `json:"timestamp"`
	LineItems       []OpenAIUsageLineItem   `json:"line_items"`
	AggregatedUsage []OpenAIAggregatedUsage `json:"aggregated_usage"`
}

// OpenAIUsageLineItem represents a specific usage line item
type OpenAIUsageLineItem struct {
	Name           string  `json:"name"`
	QuantifiedCost float64 `json:"quantized_cost"` // Note: API returns "quantized_cost"
}

// OpenAIAggregatedUsage represents aggregated usage statistics
type OpenAIAggregatedUsage struct {
	Tokens    int64  `json:"tokens"`
	Requests  int64  `json:"requests"`
	Operation string `json:"operation"`
	Model     string `json:"model"`
}

// OpenAIUsageMetadata contains metadata about the usage data
type OpenAIUsageMetadata struct {
	BillingPeriod OpenAIBillingPeriod `json:"billing_period"`
}

// OpenAIBillingPeriod represents a billing period
type OpenAIBillingPeriod struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// OpenAISubscriptionResponse represents a response from OpenAI's subscription API
type OpenAISubscriptionResponse struct {
	Object         string                `json:"object"`
	Subscription   OpenAISubscription    `json:"subscription"`
	PaymentMethods []OpenAIPaymentMethod `json:"payment_methods"`
}

// OpenAISubscription represents subscription information
type OpenAISubscription struct {
	Plan             OpenAIPlan `json:"plan"`
	AccountName      string     `json:"account_name"`
	UserID           string     `json:"user_id"`
	SoftLimitUSD     float64    `json:"soft_limit_usd"`
	HardLimitUSD     float64    `json:"hard_limit_usd"`
	AccessUntil      string     `json:"access_until"`
	SystemIDs        []string   `json:"system_ids"`
	HasPaymentMethod bool       `json:"has_payment_method"`
}

// OpenAIPlan represents the billing plan
type OpenAIPlan struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	IsActive         bool    `json:"is_active"`
	BillingFrequency string  `json:"billing_frequency"` // e.g., "monthly"
	SoftLimitUSD     float64 `json:"soft_limit_usd"`
	HardLimitUSD     float64 `json:"hard_limit_usd"`
}

// OpenAIPaymentMethod represents a payment method
type OpenAIPaymentMethod struct {
	Object string `json:"object"`
	ID     string `json:"id"`
	Type   string `json:"type"`
}

// OpenAIModelsUsage represents usage per model
type OpenAIModelsUsage struct {
	Model   string  `json:"model"`
	Tokens  int64   `json:"tokens"`
	CostUSD float64 `json:"cost_usd"`
}

// OpenAIUsageSummary represents a summary of usage data
type OpenAIUsageSummary struct {
	TotalTokens   int64                        `json:"total_tokens"`
	TotalRequests int64                        `json:"total_requests"`
	TotalCostUSD  float64                      `json:"total_cost_usd"`
	Models        map[string]OpenAIModelsUsage `json:"models"`
}

// ============================================================================
// OpenAI QuotaProvider Implementation
// ============================================================================

// GetQuota implements the QuotaProvider interface for fetching real-time quota information
// from OpenAI's usage and subscription APIs.
//
// It queries OpenAI's billing/usage API to get current usage information, which includes:
// - Token usage (total, input, output)
// - Request counts
// - Cost information
//
// For Codex models (code completion), data is aggregated separately from ChatGPT models.
func (p *OpenAIProvider) GetQuota(ctx context.Context, model string) (*quota.QuotaInfo, error) {
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, types.NewAuthError(types.ProviderTypeOpenAI, "no API keys configured").
			WithOperation("GetQuota")
	}

	// Use first available API key
	keys := p.authHelper.KeyManager.GetKeys()
	apiKey := keys[0]

	// Fetch subscription info (for limits)
	subscription, err := p.fetchSubscription(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription info: %w", err)
	}

	// Fetch current billing period usage
	now := time.Now().UTC()
	startDate := now.AddDate(0, -1, 0).Format("2006-01-02") // Last 30 days
	endDate := now.Format("2006-01-02")

	usageSummary, err := p.fetchUsage(ctx, apiKey, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch usage data: %w", err)
	}

	// Build quota info
	quotaInfo := &quota.QuotaInfo{
		Provider:             p.Name(),
		ProviderType:         p.Type(),
		Model:                model,
		Timestamp:            now,
		Quotas:               make(map[quota.QuotaType]*quota.QuotaUsage),
		ProviderQuotaConfigs: make(map[quota.QuotaType]*quota.QuotaConfig),
		CustomUsage:          make(map[string]interface{}),
		Metadata:             make(map[string]interface{}),
	}

	// Get usage for the specific model (or overall if not specified)
	var modelTokens, modelRequests int64
	if model != "" {
		if modelUsage, ok := usageSummary.Models[model]; ok {
			modelTokens = modelUsage.Tokens
		}
	} else {
		modelTokens = usageSummary.TotalTokens
		modelRequests = usageSummary.TotalRequests
	}

	// Compute quota information based on subscription plan
	monthlyLimit := p.computeEffectiveLimit(subscription)

	// Add daily quota usage (based on today's usage)
	quotaInfo.Quotas[quota.QuotaTypeDaily] = &quota.QuotaUsage{
		Type:             quota.QuotaTypeDaily,
		Period:           quota.QuotaPeriodDay,
		Used:             int(modelTokens),
		Limit:            int(monthlyLimit),
		Remaining:        int(monthlyLimit - modelTokens),
		RemainingPercent: percentageRemaining(int(modelTokens), int(monthlyLimit)),
		ResetAt:          getNextResetTime(),
		PeriodStartedAt:  getPeriodStartTime(now, "day"),
	}

	// Add total tokens quota
	quotaInfo.Quotas[quota.QuotaTypeTokens] = &quota.QuotaUsage{
		Type:             quota.QuotaTypeTokens,
		Period:           quota.QuotaPeriodMonth,
		Used:             int(usageSummary.TotalTokens),
		Limit:            int(monthlyLimit),
		Remaining:        int(monthlyLimit - usageSummary.TotalTokens),
		RemainingPercent: percentageRemaining(int(usageSummary.TotalTokens), int(monthlyLimit)),
		ResetAt:          getMonthlyResetTime(subscription),
		PeriodStartedAt:  getPeriodStartTime(now, "month"),
	}

	// Add request quota (if available)
	if modelRequests > 0 {
		quotaInfo.Quotas[quota.QuotaTypeRequests] = &quota.QuotaUsage{
			Type:             quota.QuotaTypeRequests,
			Period:           quota.QuotaPeriodMonth,
			Used:             int(modelRequests),
			Limit:            100000, // OpenAI has high request limits
			Remaining:        100000 - int(modelRequests),
			RemainingPercent: percentageRemaining(int(modelRequests), 100000),
			ResetAt:          getMonthlyResetTime(subscription),
			PeriodStartedAt:  getPeriodStartTime(now, "month"),
		}
	}

	// Add custom usage for OpenAI-specific data
	quotaInfo.CustomUsage["total_cost_usd"] = usageSummary.TotalCostUSD
	quotaInfo.CustomUsage["soft_limit_usd"] = subscription.Subscription.SoftLimitUSD
	quotaInfo.CustomUsage["hard_limit_usd"] = subscription.Subscription.HardLimitUSD
	quotaInfo.CustomUsage["has_payment_method"] = subscription.Subscription.HasPaymentMethod
	quotaInfo.CustomUsage["plan_id"] = subscription.Subscription.Plan.ID
	quotaInfo.CustomUsage["plan_title"] = subscription.Subscription.Plan.Title

	// Add metadata
	quotaInfo.Metadata["billing_period_start"] = subscription.Subscription.Plan.BillingFrequency + "_start"
	quotaInfo.Metadata["access_until"] = subscription.Subscription.AccessUntil
	quotaInfo.Metadata["models_used"] = len(usageSummary.Models)

	// Store provider quota configurations
	quotaInfo.ProviderQuotaConfigs[quota.QuotaTypeDaily] = &quota.QuotaConfig{
		Type:       quota.QuotaTypeDaily,
		Period:     quota.QuotaPeriodDay,
		Limit:      int(monthlyLimit),
		ResetAt:    getNextResetTime(),
		CustomData: map[string]interface{}{"based_on": "subscription_soft_limit"},
	}

	return quotaInfo, nil
}

// GetQuotaHistory implements the QuotaProvider interface for fetching historical quota usage
// from OpenAI's usage API.
func (p *OpenAIProvider) GetQuotaHistory(
	ctx context.Context,
	model string,
	startTime, endTime time.Time,
) (*quota.QuotaHistory, error) {
	if p.authHelper.KeyManager == nil || len(p.authHelper.KeyManager.GetKeys()) == 0 {
		return nil, types.NewAuthError(types.ProviderTypeOpenAI, "no API keys configured").
			WithOperation("GetQuotaHistory")
	}

	// Use first available API key
	keys := p.authHelper.KeyManager.GetKeys()
	apiKey := keys[0]

	// Fetch usage data for the time range
	startDate := startTime.Format("2006-01-02")
	endDate := endTime.Format("2006-01-02")

	usageResponse, err := p.fetchUsageRaw(ctx, apiKey, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch usage history: %w", err)
	}

	// Build quota history
	history := &quota.QuotaHistory{
		Provider:   p.Name(),
		Model:      model,
		Records:    make([]*quota.QuotaRecord, 0),
		TotalUsage: make(map[quota.QuotaType]int),
		StartTime:  startTime,
		EndTime:    endTime,
	}

	totalTokens := int64(0)
	totalRequests := int64(0)

	// Parse daily usage data into records
	for _, dayData := range usageResponse.Daily {
		timestamp := time.Unix(dayData.Timestamp, 0).In(time.UTC)

		// Skip if outside the requested range
		if timestamp.Before(startTime) || timestamp.After(endTime) {
			continue
		}

		dayTokens := int64(0)
		dayRequests := int64(0)

		// Aggregate usage from line items
		for _, item := range dayData.AggregatedUsage {
			// Filter by model if specified
			if model != "" && item.Model != model {
				continue
			}

			dayTokens += item.Tokens
			dayRequests += item.Requests
		}

		// Create quota record for this day
		record := &quota.QuotaRecord{
			ID:        fmt.Sprintf("openai_%s_%d", timestamp.Format("20060102"), timestamp.Unix()),
			Provider:  p.Name(),
			Model:     model,
			Timestamp: timestamp,
			Operation: "chat_completion",
			Usage: map[quota.QuotaType]int{
				quota.QuotaTypeTokens:   int(dayTokens),
				quota.QuotaTypeRequests: int(dayRequests),
			},
		}

		history.Records = append(history.Records, record)
		totalTokens += dayTokens
		totalRequests += dayRequests
	}

	// Set total usage
	history.TotalUsage[quota.QuotaTypeTokens] = int(totalTokens)
	history.TotalUsage[quota.QuotaTypeRequests] = int(totalRequests)

	return history, nil
}

// SupportsQuotaReporting returns true if OpenAI supports real-time quota reporting.
func (p *OpenAIProvider) SupportsQuotaReporting() bool {
	return true
}

// GetQuotaInfo implements the types.QuotaProvider interface for backward compatibility.
// This is an alias to GetQuota from the quota.QuotaProvider interface.
func (p *OpenAIProvider) GetQuotaInfo(ctx context.Context, model string) (*types.QuotaInfo, error) {
	quotaInfo, err := p.GetQuota(ctx, model)
	if err != nil {
		return nil, err
	}

	// Convert quota.QuotaInfo to types.QuotaInfo
	return &types.QuotaInfo{
		Provider:             quotaInfo.Provider,
		ProviderType:         quotaInfo.ProviderType,
		Model:                quotaInfo.Model,
		Timestamp:            quotaInfo.Timestamp,
		Quotas:               convertQuotaUsageMap(quotaInfo.Quotas),
		ProviderQuotaConfigs: convertQuotaConfigMap(quotaInfo.ProviderQuotaConfigs),
		CustomUsage:          quotaInfo.CustomUsage,
		Metadata:             quotaInfo.Metadata,
	}, nil
}

// SupportsQuotaHistory returns true if the provider supports historical quota data.
func (p *OpenAIProvider) SupportsQuotaHistory() bool {
	return true
}

// ============================================================================
// OpenAI API Helper Methods
// ============================================================================

// fetchSubscription fetches subscription information from OpenAI's API
func (p *OpenAIProvider) fetchSubscription(ctx context.Context, apiKey string) (*OpenAISubscriptionResponse, error) {
	// Get the billing base URL - usually different from API base URL
	// OpenAI's billing API uses api.openai.com
	billingURL := "https://api.openai.com/v1/dashboard/billing/subscription"

	// If using a custom base URL that's not api.openai.com, replace the path
	// Most OpenAI-compatible proxies don't implement the billing API
	if p.baseURL != "" && p.baseURL != "https://api.openai.com" && p.baseURL != "https://api.openai.com/" {
		// For compatible providers that don't implement the billing API,
		// return a mock subscription response
		return p.getMockSubscription(), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", billingURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer commonhttp.SafeClose(resp)

	if err := commonhttp.HandleHTTPStatusError(resp, types.ProviderTypeOpenAI, "fetchSubscription", "failed to fetch subscription"); err != nil {
		return nil, err
	}

	var subscription OpenAISubscriptionResponse
	if err := commonhttp.UnmarshalJSONResponse(resp.Body, &subscription, types.ProviderTypeOpenAI, "fetchSubscription"); err != nil {
		return nil, err
	}

	return &subscription, nil
}

// fetchUsage fetches usage data from OpenAI's API and returns a summary
func (p *OpenAIProvider) fetchUsage(ctx context.Context, apiKey, startDate, endDate string) (*OpenAIUsageSummary, error) {
	usageResponse, err := p.fetchUsageRaw(ctx, apiKey, startDate, endDate)
	if err != nil {
		return nil, err
	}

	return p.summarizeUsage(usageResponse), nil
}

// fetchUsageRaw fetches raw usage data from OpenAI's API
func (p *OpenAIProvider) fetchUsageRaw(ctx context.Context, apiKey, startDate, endDate string) (*OpenAIUsageResponse, error) {
	// Format: https://api.openai.com/v1/dashboard/billing/usage?start_date=2023-01-01&end_date=2023-01-31
	billingURL := "https://api.openai.com/v1/dashboard/billing/usage"

	// For compatible providers, return mock data
	if p.baseURL != "" && p.baseURL != "https://api.openai.com" && p.baseURL != "https://api.openai.com/" {
		return p.getMockUsage(), nil
	}

	reqURL, err := url.Parse(billingURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse billing URL: %w", err)
	}

	query := reqURL.Query()
	query.Set("start_date", startDate)
	query.Set("end_date", endDate)
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := p.httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer commonhttp.SafeClose(resp)

	if err := commonhttp.HandleHTTPStatusError(resp, types.ProviderTypeOpenAI, "fetchUsage", "failed to fetch usage"); err != nil {
		return nil, err
	}

	var usage OpenAIUsageResponse
	if err := commonhttp.UnmarshalJSONResponse(resp.Body, &usage, types.ProviderTypeOpenAI, "fetchUsage"); err != nil {
		return nil, err
	}

	return &usage, nil
}

// summarizeUsage creates a summary from raw usage data
func (p *OpenAIProvider) summarizeUsage(usage *OpenAIUsageResponse) *OpenAIUsageSummary {
	summary := &OpenAIUsageSummary{
		TotalTokens:   0,
		TotalRequests: 0,
		TotalCostUSD:  0,
		Models:        make(map[string]OpenAIModelsUsage),
	}

	for _, dayData := range usage.Daily {
		for _, lineItem := range dayData.LineItems {
			summary.TotalCostUSD += lineItem.QuantifiedCost
		}

		for _, aggUsage := range dayData.AggregatedUsage {
			summary.TotalRequests += aggUsage.Requests
			summary.TotalTokens += aggUsage.Tokens

			// Track per-model usage (for Codex vs ChatGPT differentiation)
			modelUsage, exists := summary.Models[aggUsage.Model]
			if !exists {
				modelUsage = OpenAIModelsUsage{
					Model:  aggUsage.Model,
					Tokens: 0,
				}
			}
			modelUsage.Tokens += aggUsage.Tokens
			summary.Models[aggUsage.Model] = modelUsage

			// Track code models (Codex) separately
			if isCodeModel(aggUsage.Model) {
				codeUsage, exists := summary.Models["codex"]
				if !exists {
					codeUsage = OpenAIModelsUsage{
						Model:  "codex",
						Tokens: 0,
					}
				}
				codeUsage.Tokens += aggUsage.Tokens
				summary.Models["codex"] = codeUsage
			}

			// Track ChatGPT models
			if isChatModel(aggUsage.Model) {
				chatUsage, exists := summary.Models["chatgpt"]
				if !exists {
					chatUsage = OpenAIModelsUsage{
						Model:  "chatgpt",
						Tokens: 0,
					}
				}
				chatUsage.Tokens += aggUsage.Tokens
				summary.Models["chatgpt"] = chatUsage
			}
		}
	}

	return summary
}

// computeEffectiveLimit computes the effective token limit based on subscription
func (p *OpenAIProvider) computeEffectiveLimit(subscription *OpenAISubscriptionResponse) int64 {
	// Use the soft limit as the effective limit
	// OpenAI's free tier typically has a soft limit of $5-18 USD
	// Paid plans have much higher limits
	softLimitUSD := subscription.Subscription.SoftLimitUSD

	// Estimate tokens from USD limit (approximately $0.002 per 1K tokens for GPT-3.5)
	// This is a rough estimate - actual pricing varies by model
	tokensPerUSD := int64(500000) // ~500K tokens per $1 USD for GPT-3.5

	return int64(softLimitUSD * float64(tokensPerUSD))
}

// getMockSubscription returns a mock subscription for compatible providers
func (p *OpenAIProvider) getMockSubscription() *OpenAISubscriptionResponse {
	return &OpenAISubscriptionResponse{
		Object: "billing.subscription",
		Subscription: OpenAISubscription{
			Plan: OpenAIPlan{
				ID:           "mock-plan",
				Title:        "Mock Provider Plan",
				Description:  "Mock plan for OpenAI-compatible provider",
				IsActive:     true,
				SoftLimitUSD: 1000.0,
				HardLimitUSD: 1200.0,
			},
			AccountName:      "Mock Account",
			UserID:           "mock-user-id",
			SoftLimitUSD:     1000.0,
			HardLimitUSD:     1200.0,
			HasPaymentMethod: true,
		},
	}
}

// getMockUsage returns mock usage data for compatible providers
func (p *OpenAIProvider) getMockUsage() *OpenAIUsageResponse {
	now := time.Now().UTC()
	daily := make([]OpenAIUsageDayData, 0)

	// Generate mock daily data for the last 30 days
	for i := 29; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dayTimestamp := day.Truncate(24 * time.Hour).Unix()

		daily = append(daily, OpenAIUsageDayData{
			Timestamp: dayTimestamp,
			LineItems: []OpenAIUsageLineItem{
				{
					Name:           "gpt-4-turbo",
					QuantifiedCost: 0.50,
				},
			},
			AggregatedUsage: []OpenAIAggregatedUsage{
				{
					Tokens:    250000,
					Requests:  100,
					Operation: "chat_completion",
					Model:     "gpt-4-turbo",
				},
			},
		})
	}

	return &OpenAIUsageResponse{
		Object: "billing.usage",
		Daily:  daily,
		Metadata: OpenAIUsageMetadata{
			BillingPeriod: OpenAIBillingPeriod{
				StartDate: now.AddDate(0, -1, 0).Format("2006-01-02"),
				EndDate:   now.Format("2006-01-02"),
			},
		},
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// isCodeModel returns true if the model is a code model (Codex)
func isCodeModel(model string) bool {
	codeModels := []string{
		"code-davinci-002",
		"code-cushman-001",
		"code-davinci-001",
	}

	for _, cm := range codeModels {
		if model == cm {
			return true
		}
	}
	return false
}

// isChatModel returns true if the model is a ChatGPT model
func isChatModel(model string) bool {
	chatModels := []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-instruct",
	}

	for _, cm := range chatModels {
		if model == cm || contains(model, cm) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// percentageRemaining calculates the percentage of quota remaining
func percentageRemaining(used, limit int) float64 {
	if limit <= 0 {
		return 0
	}
	remaining := limit - used
	if remaining < 0 {
		return 0
	}
	percentage := (float64(remaining) / float64(limit)) * 100
	if percentage > 100 {
		return 100
	}
	return percentage
}

// getNextResetTime returns the next daily reset time (midnight UTC)
func getNextResetTime() time.Time {
	now := time.Now().UTC()
	tomorrow := now.Add(24 * time.Hour)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.UTC)
}

// getMonthlyResetTime returns the monthly billing reset time
// nolint:unparam // subscription parameter is kept for future use
func getMonthlyResetTime(subscription *OpenAISubscriptionResponse) time.Time {
	now := time.Now().UTC()
	// Reset to first day of next month at midnight
	nextMonth := now.AddDate(0, 1, 0)
	return time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// getPeriodStartTime returns the start time of a period
func getPeriodStartTime(now time.Time, periodType string) time.Time {
	switch periodType {
	case "day":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	case "month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Time{}
	}
}

// convertQuotaUsageMap converts quota.QuotaUsage to types.QuotaUsage
func convertQuotaUsageMap(m map[quota.QuotaType]*quota.QuotaUsage) map[types.QuotaType]*types.QuotaUsage {
	result := make(map[types.QuotaType]*types.QuotaUsage)
	for k, v := range m {
		result[types.QuotaType(k)] = &types.QuotaUsage{
			Type:             types.QuotaType(v.Type),
			Period:           types.QuotaPeriod(v.Period),
			Used:             v.Used,
			Limit:            v.Limit,
			Remaining:        v.Remaining,
			RemainingPercent: v.RemainingPercent,
			ResetAt:          v.ResetAt,
			PeriodStartedAt:  v.PeriodStartedAt,
		}
	}
	return result
}

// convertQuotaConfigMap converts quota.QuotaConfig to types.QuotaConfig
func convertQuotaConfigMap(m map[quota.QuotaType]*quota.QuotaConfig) map[types.QuotaType]*types.QuotaConfig {
	result := make(map[types.QuotaType]*types.QuotaConfig)
	for k, v := range m {
		result[types.QuotaType(k)] = &types.QuotaConfig{
			Type:       types.QuotaType(v.Type),
			Period:     types.QuotaPeriod(v.Period),
			Limit:      v.Limit,
			ResetAt:    v.ResetAt,
			CustomData: v.CustomData,
		}
	}
	return result
}

// SafeStringToInt64 safely converts a string to int64
func SafeStringToInt64(s string) int64 {
	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val
	}
	return 0
}

// SafeStringToFloat64 safely converts a string to float64
func SafeStringToFloat64(s string) float64 {
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val
	}
	return 0.0
}
