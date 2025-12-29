// Package gemini provides a Google Gemini AI provider implementation.
// It includes support for chat completions, streaming, tool calling, and OAuth authentication.
package gemini

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common"
	"github.com/cecil-the-coder/ai-provider-kit/internal/common/auth"
	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"golang.org/x/time/rate"
)

// GeminiProvider implements the Provider interface for Google Gemini with OAuth support
type GeminiProvider struct {
	*base.BaseProvider
	authHelper        *auth.AuthHelper // Shared authentication helper
	client            *http.Client
	httpClient        *pkghttp.HTTPClient // Pooled HTTP client with retry logic
	config            GeminiConfig
	projectID         string
	displayName       string
	rateLimitHelper   *common.RateLimitHelper
	rateLimitMutex    sync.RWMutex
	clientSideLimiter *rate.Limiter
	backendRouter     *BackendRouter    // Backend router for Gemini API vs Vertex AI
	codeAssist        *CodeAssistClient // Code Assist API client (only for BackendCodeAssist)
}

// Name returns the display name of the provider
func (p *GeminiProvider) Name() string {
	if p.displayName != "" {
		return p.displayName
	}
	return "gemini"
}

// Type returns the provider type
func (p *GeminiProvider) Type() types.ProviderType {
	return types.ProviderTypeGemini
}

// Description returns a description of the provider
func (p *GeminiProvider) Description() string {
	return "Google Gemini with multi-OAuth failover and load balancing"
}

// GetModels returns the list of available models
// Context lengths differ by backend:
// - Code Assist backend: 1M tokens for all models (to align with ecosystem tools)
// - Gemini API/Vertex AI backends: Various context lengths per model (2M, 1M, 512K)
func (p *GeminiProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	// Determine context lengths based on backend
	// Code Assist has a 1M context limit for all models
	var gemini3ProTokens, gemini25ProTokens, gemini25FlashTokens, gemini25FlashLiteTokens, gemini20FlashTokens, gemini20FlashLiteTokens int

	if p.backendRouter.GetBackend() == BackendCodeAssist {
		// Code Assist backend: 1M context for all models (per ecosystem tools report)
		gemini3ProTokens = 1048576
		gemini25ProTokens = 1048576
		gemini25FlashTokens = 1048576
		gemini25FlashLiteTokens = 1048576
		gemini20FlashTokens = 1048576
		gemini20FlashLiteTokens = 1048576
	} else {
		// Gemini API and Vertex AI backends: use standard context lengths
		gemini3ProTokens = 2097152
		gemini25ProTokens = 2097152
		gemini25FlashTokens = 1048576
		gemini25FlashLiteTokens = 524288
		gemini20FlashTokens = 1048576
		gemini20FlashLiteTokens = 524288
	}

	return []types.Model{
		// Gemini 3 Series (Preview)
		{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro Preview", Provider: p.Type(), MaxTokens: gemini3ProTokens, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Google's latest Gemini 3 Pro model (preview)"},
		{ID: "gemini-3-pro-image-preview", Name: "Gemini 3 Pro Image Preview", Provider: p.Type(), MaxTokens: gemini3ProTokens, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Gemini 3 Pro with enhanced image understanding (preview)"},

		// Gemini 2.5 Series
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: p.Type(), MaxTokens: gemini25ProTokens, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "State-of-the-art thinking model for complex problems"},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: p.Type(), MaxTokens: gemini25FlashTokens, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Best price-performance for high-volume tasks and agentic use cases"},
		{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Provider: p.Type(), MaxTokens: gemini25FlashLiteTokens, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Built for massive scale, optimized for efficiency"},

		// Gemini 2.0 Series
		{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: p.Type(), MaxTokens: gemini20FlashTokens, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Multimodal model for general-purpose tasks"},
		{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash Lite", Provider: p.Type(), MaxTokens: gemini20FlashLiteTokens, SupportsStreaming: true, SupportsToolCalling: true, Capabilities: []string{"vision", "multimodal"}, Description: "Ultra-efficient for simple, high-frequency tasks"},
	}, nil
}

// GetDefaultModel returns the default model for the provider
func (p *GeminiProvider) GetDefaultModel() string {
	if p.config.Model != "" {
		return p.config.Model
	}
	return geminiDefaultModel
}

// IsAuthenticated checks if the provider is authenticated
func (p *GeminiProvider) IsAuthenticated() bool {
	return p.authHelper.IsAuthenticated()
}

// IsOAuthConfigured checks if OAuth authentication is properly configured
func (p *GeminiProvider) IsOAuthConfigured() bool {
	return p.authHelper.IsOAuthConfigured()
}

// IsAPIKeyConfigured checks if API key authentication is properly configured
func (p *GeminiProvider) IsAPIKeyConfigured() bool {
	return p.authHelper.IsAPIKeyConfigured()
}

// SetCredentialProvider sets a dynamic credential provider for OAuth credentials
// This allows external systems to manage credential storage and provide fresh
// credentials on-demand, rather than relying on cached credentials.
func (p *GeminiProvider) SetCredentialProvider(provider types.CredentialProvider) {
	if p.authHelper.OAuthManager != nil {
		p.authHelper.OAuthManager.SetCredentialProvider(provider)
	}
}

// GetAuthStatus provides detailed authentication status using shared helper
func (p *GeminiProvider) GetAuthStatus() map[string]interface{} {
	return p.authHelper.GetAuthStatus()
}

// Logout clears the authentication credentials
func (p *GeminiProvider) Logout(ctx context.Context) error {
	// Use auth helper to clear authentication
	p.authHelper.ClearAuthentication()

	// Clear local config
	p.config.APIKey = ""
	p.config.APIKeys = nil

	newConfig := p.GetConfig()
	return p.Configure(newConfig)
}

// TestConnectivity performs a lightweight connectivity test to verify the provider can reach its service
func (p *GeminiProvider) TestConnectivity(ctx context.Context) error {
	// Check for OAuth token in context first (injected by caller)
	contextToken := auth.GetOAuthToken(ctx)

	// Check if we have API keys configured
	hasAPIKeys := p.authHelper.KeyManager != nil && len(p.authHelper.KeyManager.GetKeys()) > 0
	// Use GetCredentialsWithContext to fetch fresh credentials from dynamic provider
	hasOAuth := p.authHelper.OAuthManager != nil && len(p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)) > 0
	hasContextOAuth := contextToken != ""

	if !hasAPIKeys && !hasOAuth && !hasContextOAuth {
		return types.NewAuthError(types.ProviderTypeGemini, "no API keys or OAuth credentials configured").
			WithOperation("test_connectivity")
	}

	// For API keys, make a minimal API call to test connectivity
	if hasAPIKeys {
		apiKey := p.authHelper.KeyManager.GetKeys()[0]
		if err := p.testConnectivityWithAPIKey(ctx, apiKey); err == nil {
			return nil
		}
	}

	// For OAuth, prefer context token, then stored credentials
	if hasContextOAuth {
		return p.testConnectivityWithOAuth(ctx, contextToken)
	}
	if hasOAuth {
		// Use GetCredentialsWithContext to fetch fresh credentials from dynamic provider
		creds := p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)
		return p.testConnectivityWithOAuth(ctx, creds[0].AccessToken)
	}

	return types.NewAuthError(types.ProviderTypeGemini, "no valid authentication credentials available").
		WithOperation("test_connectivity")
}

// SupportsToolCalling returns true if the provider supports tool calling
func (p *GeminiProvider) SupportsToolCalling() bool {
	return true
}

// SupportsStreaming returns true if the provider supports streaming
func (p *GeminiProvider) SupportsStreaming() bool {
	return true
}

// SupportsResponsesAPI returns false as Gemini doesn't support the Responses API
func (p *GeminiProvider) SupportsResponsesAPI() bool {
	return false
}

// GetToolFormat returns the Gemini tool format
func (p *GeminiProvider) GetToolFormat() types.ToolFormat {
	return types.ToolFormatGemini
}

// InvokeServerTool invokes a server tool (not yet implemented for Gemini)
func (p *GeminiProvider) InvokeServerTool(
	ctx context.Context,
	toolName string,
	params interface{},
) (interface{}, error) {
	return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "tool invocation not yet implemented for Gemini provider").
		WithOperation("invoke_tool")
}

// RefreshAllOAuthTokens using shared helper
func (p *GeminiProvider) RefreshAllOAuthTokens(ctx context.Context) error {
	return p.authHelper.RefreshAllOAuthTokens(ctx)
}

// ============================================================================
// QuotaProvider Interface Implementation
// ============================================================================

// GetQuotaInfo returns the current quota information for the provider.
// This method is part of the types.QuotaProvider interface and fetches real-time
// quota information from the provider's API when using the Code Assist backend.
//
// When using the Code Assist backend with OAuth authentication, this method
// fetches quota information from the Code Assist API. For other backends
// (Gemini API, Vertex AI), quota information is not available via API and
// the method returns an appropriate error indicating that quota reporting
// is not supported for those backends.
//
// Parameters:
//   - ctx: Context for the request
//   - model: The model identifier (empty for provider-wide quota)
//
// Returns:
//   - QuotaInfo: Current quota information including limits, usage, and reset times
//   - error: Error if quota information cannot be retrieved or if not supported
func (p *GeminiProvider) GetQuotaInfo(ctx context.Context, model string) (*types.QuotaInfo, error) {
	// Only Code Assist backend supports quota reporting
	if p.backendRouter.GetBackend() != BackendCodeAssist {
		return nil, types.NewInvalidRequestError(
			types.ProviderTypeGemini,
			"quota reporting is only supported for Code Assist backend with OAuth authentication",
		).WithOperation("get_quota_info")
	}

	// Check if Code Assist client is available
	if p.codeAssist == nil {
		return nil, types.NewInvalidRequestError(
			types.ProviderTypeGemini,
			"Code Assist client not available - ensure OAuth credentials are configured",
		).WithOperation("get_quota_info")
	}

	// Get project ID
	projectID := p.getProjectID()
	if projectID == "" {
		return nil, types.NewInvalidRequestError(
			types.ProviderTypeGemini,
			"project ID is required for quota reporting - set GOOGLE_CLOUD_PROJECT environment variable or configure in provider config",
		).WithOperation("get_quota_info")
	}

	// Build quota request with optional model filter
	req := GetQuotaRequest{
		Project: projectID,
		Model:   model,
	}

	// Call Code Assist API
	resp, err := p.codeAssist.GetQuota(ctx, projectID, req)
	if err != nil {
		return nil, err
	}

	// Convert GetQuotaResponse to types.QuotaInfo directly
	quotaInfo := p.convertToQuotaInfo(resp, model)

	return quotaInfo, nil
}

// mapTypesQuotaType converts a string quota type to a types.QuotaType.
func mapTypesQuotaType(qt string) types.QuotaType {
	switch {
	case qt == "requests" || qt == "api_requests":
		return types.QuotaTypeRequests
	case qt == "tokens" || qt == "total_tokens":
		return types.QuotaTypeTokens
	case qt == "input_tokens":
		return types.QuotaTypeInputTokens
	case qt == "output_tokens":
		return types.QuotaTypeOutputTokens
	case qt == "daily_tokens" || qt == "daily_requests":
		return types.QuotaTypeDaily
	default:
		return types.QuotaTypeCustom
	}
}

// mapTypesQuotaPeriod converts a string quota period to a types.QuotaPeriod.
func mapTypesQuotaPeriod(qp string) types.QuotaPeriod {
	switch qp {
	case "minute", "1m":
		return types.QuotaPeriodMinute
	case "hour", "1h":
		return types.QuotaPeriodHour
	case "day", "1d", "daily":
		return types.QuotaPeriodDay
	case "week", "1w":
		return types.QuotaPeriodWeek
	case "month", "1M":
		return types.QuotaPeriodMonth
	default:
		return types.QuotaPeriodCustom
	}
}

// calculateTypesPeriodStartTime calculates when the current quota period started based on reset time.
func calculateTypesPeriodStartTime(resetAt time.Time, qp types.QuotaPeriod) time.Time {
	switch qp {
	case types.QuotaPeriodDay:
		return resetAt.AddDate(0, 0, -1)
	case types.QuotaPeriodMonth:
		return resetAt.AddDate(0, -1, 0)
	case types.QuotaPeriodHour:
		return resetAt.Add(-time.Hour)
	case types.QuotaPeriodMinute:
		return resetAt.Add(-time.Minute)
	default:
		return time.Time{}
	}
}

// convertToQuotaInfo converts a GetQuotaResponse to types.QuotaInfo format.
// This is called internally by GetQuotaInfo.
func (p *GeminiProvider) convertToQuotaInfo(resp *GetQuotaResponse, model string) *types.QuotaInfo {
	if resp == nil {
		return nil
	}

	info := &types.QuotaInfo{
		Provider:     "gemini",
		ProviderType: types.ProviderTypeGemini,
		Model:        model,
		Timestamp:    time.Now(),
		Quotas:       make(map[types.QuotaType]*types.QuotaUsage),
		CustomUsage:  make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
	}

	// Set project and tier in metadata
	info.Metadata["project"] = resp.Project
	info.Metadata["tier"] = resp.Tier

	// Copy custom data
	for k, v := range resp.CustomData {
		info.CustomUsage[k] = v
	}

	// Process quota limits
	for _, q := range resp.Quotas {
		qt := mapTypesQuotaType(q.Type)
		qp := mapTypesQuotaPeriod(q.Period)

		// Parse reset time
		var resetAt time.Time
		if q.ResetTime != "" {
			resetAt, _ = time.Parse(time.RFC3339, q.ResetTime)
		}

		// Calculate remaining percent
		var remainingPercent float64
		if q.Limit > 0 {
			remainingPercent = float64(q.Remaining) / float64(q.Limit) * 100
		}

		info.Quotas[qt] = &types.QuotaUsage{
			Type:             qt,
			Period:           qp,
			Used:             int(q.Usage),
			Limit:            int(q.Limit),
			Remaining:        int(q.Remaining),
			RemainingPercent: remainingPercent,
			ResetAt:          resetAt,
			PeriodStartedAt:  calculateTypesPeriodStartTime(resetAt, qp),
		}
	}

	return info
}

// GetQuotaHistory returns historical quota usage for the provider.
// This method is part of the types.QuotaProvider interface and fetches historical
// usage data from the provider's API when using the Code Assist backend.
//
// When using the Code Assist backend with OAuth authentication, this method
// fetches historical usage records from the Code Assist API. For other backends
// (Gemini API, Vertex AI), historical usage is not available via API and
// the method returns an appropriate error.
//
// Parameters:
//   - ctx: Context for the request
//   - model: The model identifier (empty for all models)
//   - startTime: Start of the time range for historical data
//   - endTime: End of the time range for historical data
//
// Returns:
//   - QuotaHistory: Historical quota usage records and aggregates
//   - error: Error if history cannot be retrieved or if not supported
func (p *GeminiProvider) GetQuotaHistory(ctx context.Context, model string, startTime, endTime time.Time) (*types.QuotaHistory, error) {
	// Only Code Assist backend supports quota history
	if p.backendRouter.GetBackend() != BackendCodeAssist {
		return nil, types.NewInvalidRequestError(
			types.ProviderTypeGemini,
			"quota history is only supported for Code Assist backend with OAuth authentication",
		).WithOperation("get_quota_history")
	}

	// Check if Code Assist client is available
	if p.codeAssist == nil {
		return nil, types.NewInvalidRequestError(
			types.ProviderTypeGemini,
			"Code Assist client not available - ensure OAuth credentials are configured",
		).WithOperation("get_quota_history")
	}

	// Get project ID
	projectID := p.getProjectID()
	if projectID == "" {
		return nil, types.NewInvalidRequestError(
			types.ProviderTypeGemini,
			"project ID is required for quota history - set GOOGLE_CLOUD_PROJECT environment variable or configure in provider config",
		).WithOperation("get_quota_history")
	}

	// Build quota history request
	req := GetQuotaHistoryRequest{
		StartTime: startTime.Format(time.RFC3339),
		EndTime:   endTime.Format(time.RFC3339),
		Model:     model,
	}

	// Call Code Assist API
	resp, err := p.codeAssist.GetQuotaHistory(ctx, projectID, req)
	if err != nil {
		return nil, err
	}

	// Convert GetQuotaHistoryResponse to types.QuotaHistory directly
	quotaHistory := p.convertToQuotaHistory(resp, projectID, model)

	return quotaHistory, nil
}

// convertToQuotaHistory converts a GetQuotaHistoryResponse to types.QuotaHistory format.
// This is called internally by GetQuotaHistory.
// nolint:dupl,unparam // Similar logic to code_assist.ConvertToQuotaHistory but uses types package; projectID kept for API consistency
func (p *GeminiProvider) convertToQuotaHistory(resp *GetQuotaHistoryResponse, projectID string, model string) *types.QuotaHistory {
	if resp == nil {
		return nil
	}

	history := &types.QuotaHistory{
		Provider:   "gemini",
		Model:      model,
		Records:    make([]*types.QuotaRecord, 0, len(resp.Records)),
		TotalUsage: make(map[types.QuotaType]int),
	}

	// Parse time range from summary
	if resp.Summary.StartTime != "" {
		startTime, _ := time.Parse(time.RFC3339, resp.Summary.StartTime)
		history.StartTime = startTime
	} else {
		history.StartTime = time.Now().AddDate(0, -1, 0)
	}

	if resp.Summary.EndTime != "" {
		endTime, _ := time.Parse(time.RFC3339, resp.Summary.EndTime)
		history.EndTime = endTime
	} else {
		history.EndTime = time.Now()
	}

	// Convert records
	for _, record := range resp.Records {
		timestamp, _ := time.Parse(time.RFC3339, record.Timestamp)

		usage := map[types.QuotaType]int{}
		if record.InputTokens > 0 {
			usage[types.QuotaTypeInputTokens] = int(record.InputTokens)
		}
		if record.OutputTokens > 0 {
			usage[types.QuotaTypeOutputTokens] = int(record.OutputTokens)
		}
		if record.TotalTokens > 0 {
			usage[types.QuotaTypeTokens] = int(record.TotalTokens)
		}
		if record.RequestCount > 0 {
			usage[types.QuotaTypeRequests] = int(record.RequestCount)
		}

		// Record the model actually used in the record
		recordModel := record.Model
		if model != "" && recordModel == "" {
			recordModel = model
		}

		history.Records = append(history.Records, &types.QuotaRecord{
			ID:        fmt.Sprintf("gr_%d", timestamp.UnixNano()),
			Provider:  "gemini",
			Model:     recordModel,
			Timestamp: timestamp,
			Operation: record.Operation,
			Usage:     usage,
		})

		// Accumulate total usage
		for k, v := range usage {
			history.TotalUsage[k] += v
		}
	}

	return history
}

// SupportsQuotaReporting returns true if the provider supports real-time quota reporting.
//
// For the Gemini provider, quota reporting is only supported when:
// 1. Using the Code Assist backend (BackendCodeAssist)
// 2. OAuth authentication is configured
// 3. A Code Assist client is available
//
// The Gemini API and Vertex AI backends do not expose quota information
// through their APIs.
func (p *GeminiProvider) SupportsQuotaReporting() bool {
	// Only Code Assist backend supports quota reporting
	if p.backendRouter.GetBackend() != BackendCodeAssist {
		return false
	}

	// Check if Code Assist client is available and has OAuth token
	return p.codeAssist != nil && p.codeAssist.SupportsQuotaReporting()
}

// SupportsQuotaHistory returns true if the provider supports retrieving historical
// quota usage data.
//
// Has the same requirements as SupportsQuotaReporting: only supported for
// the Code Assist backend with OAuth authentication.
func (p *GeminiProvider) SupportsQuotaHistory() bool {
	// Only Code Assist backend supports quota history
	if p.backendRouter.GetBackend() != BackendCodeAssist {
		return false
	}

	// Check if Code Assist client is available and has OAuth token
	return p.codeAssist != nil && p.codeAssist.SupportsQuotaHistory()
}
