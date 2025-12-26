// Package gemini provides Code Assist API client for Google Gemini AI provider.
// This module handles project provisioning APIs for Gemini Code Assist.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/quota"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

const (
	// codeAssistBaseURL is the base URL for Code Assist API
	codeAssistBaseURL = "https://cloudcode-pa.googleapis.com/v1internal"
)

// CodeAssistClient handles Code Assist API operations for project provisioning.
// This client is only used when the backend is BackendCodeAssist.
type CodeAssistClient struct {
	httpClient *http.Client
	baseURL    string
	oauthToken string
}

// NewCodeAssistClient creates a new Code Assist API client.
func NewCodeAssistClient(httpClient *http.Client, oauthToken string) *CodeAssistClient {
	return &CodeAssistClient{
		httpClient: httpClient,
		baseURL:    codeAssistBaseURL,
		oauthToken: oauthToken,
	}
}

// LoadCodeAssist calls the loadCodeAssist endpoint to check user tier and get project ID.
//
// Endpoint: GET /v1internal/projects/{project}:loadCodeAssist
//
// Returns:
//   - LoadCodeAssistResponse: User tier information and managed project ID
//   - error: Any error that occurred during the request
func (c *CodeAssistClient) LoadCodeAssist(ctx context.Context, projectID string) (*LoadCodeAssistResponse, error) {
	// Build URL: GET /v1internal/projects/{project}:loadCodeAssist
	url := fmt.Sprintf("%s/projects/%s%s", c.baseURL, projectID, loadCodeAssistRoute)

	// Create request - using POST with request body as per existing implementation
	// The API spec says GET but the actual implementation uses POST
	metadata := ClientMetadata{
		IDEType:    IDETypeUnspecified,
		Platform:   PlatformUnspecified,
		PluginType: PluginTypeGemini,
	}
	reqBody := LoadCodeAssistRequest{
		CloudaicompanionProject: &projectID,
		Metadata:                metadata,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal request").
			WithOperation("load_code_assist").
			WithOriginalErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to create request").
			WithOperation("load_code_assist").
			WithOriginalErr(err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.oauthToken))

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "request failed").
			WithOperation("load_code_assist").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to read response body").
			WithOperation("load_code_assist").
			WithOriginalErr(err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(responseBody)).
			WithOperation("load_code_assist").
			WithStatusCode(resp.StatusCode)
	}

	// Parse response
	var response LoadCodeAssistResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse response").
			WithOperation("load_code_assist").
			WithOriginalErr(err)
	}

	return &response, nil
}

// OnboardUser calls the onboardUser endpoint to create/provision a project for new users.
//
// Endpoint: POST /v1internal/projects:onboardUser
//
// Returns:
//   - OnboardUserResponse: Long-running operation (LRO) response
//   - error: Any error that occurred during the request
func (c *CodeAssistClient) OnboardUser(ctx context.Context, req OnboardUserRequest) (*OnboardUserResponse, error) {
	// Build URL: POST /v1internal/projects:onboardUser
	url := fmt.Sprintf("%s/projects%s", c.baseURL, onboardUserRoute)

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal request").
			WithOperation("onboard_user").
			WithOriginalErr(err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to create request").
			WithOperation("onboard_user").
			WithOriginalErr(err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", telemetry.GetUserAgent())
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.oauthToken))

	// Make the request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "request failed").
			WithOperation("onboard_user").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to read response body").
			WithOperation("onboard_user").
			WithOriginalErr(err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(responseBody)).
			WithOperation("onboard_user").
			WithStatusCode(resp.StatusCode)
	}

	// Parse LRO response
	var lroResp LongRunningOperationResponse
	if err := json.Unmarshal(responseBody, &lroResp); err != nil {
		return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse response").
			WithOperation("onboard_user").
			WithOriginalErr(err)
	}

	// Extract OnboardUserResponse from LRO response
	var onboardResp OnboardUserResponse
	if lroResp.Response != nil {
		onboardResp = *lroResp.Response
	}

	return &onboardResp, nil
}

// GetQuota retrieves the current quota information for the Code Assist API.
//
// Endpoint: POST /v1internal/projects/{project}:getQuota
//
// Parameters:
//   - ctx: Context for the request
//   - projectID: The Google Cloud project ID
//   - req: The get quota request parameters
//
// Returns:
//   - GetQuotaResponse: Quota information including limits and usage
//   - error: Any error that occurred during the request
func (c *CodeAssistClient) GetQuota(ctx context.Context, projectID string, req GetQuotaRequest) (*GetQuotaResponse, error) {
	// Build URL: POST /v1internal/projects/{project}:getQuota
	url := fmt.Sprintf("%s/projects/%s%s", c.baseURL, projectID, getQuotaRoute)

	// Ensure project ID is set in request
	if req.Project == "" {
		req.Project = projectID
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal request").
			WithOperation("get_quota").
			WithOriginalErr(err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to create request").
			WithOperation("get_quota").
			WithOriginalErr(err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", telemetry.GetUserAgent())
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.oauthToken))

	// Make the request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "request failed").
			WithOperation("get_quota").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to read response body").
			WithOperation("get_quota").
			WithOriginalErr(err)
	}

	// Check status code - accept 200 OK and 404 (quota not available)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(responseBody)).
			WithOperation("get_quota").
			WithStatusCode(resp.StatusCode)
	}

	// For 404, return empty response as quota may not be available yet
	if resp.StatusCode == http.StatusNotFound {
		return &GetQuotaResponse{
			Project: projectID,
			Tier:    "unknown",
			Quotas:  []QuotaLimit{},
		}, nil
	}

	// Parse response
	var quotaResp GetQuotaResponse
	if err := json.Unmarshal(responseBody, &quotaResp); err != nil {
		return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse response").
			WithOperation("get_quota").
			WithOriginalErr(err)
	}

	return &quotaResp, nil
}

// GetQuotaUsage retrieves usage statistics for a time period.
//
// Endpoint: POST /v1internal/projects/{project}:getQuotaUsage
//
// Parameters:
//   - ctx: Context for the request
//   - projectID: The Google Cloud project ID
//   - req: The get quota usage request parameters
//
// Returns:
//   - GetQuotaUsageResponse: Usage statistics and records
//   - error: Any error that occurred during the request
func (c *CodeAssistClient) GetQuotaUsage(ctx context.Context, projectID string, req GetQuotaUsageRequest) (*GetQuotaUsageResponse, error) {
	// Build URL: POST /v1internal/projects/{project}:getQuotaUsage
	url := fmt.Sprintf("%s/projects/%s%s", c.baseURL, projectID, getQuotaUsageRoute)

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal request").
			WithOperation("get_quota_usage").
			WithOriginalErr(err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to create request").
			WithOperation("get_quota_usage").
			WithOriginalErr(err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", telemetry.GetUserAgent())
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.oauthToken))

	// Make the request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "request failed").
			WithOperation("get_quota_usage").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to read response body").
			WithOperation("get_quota_usage").
			WithOriginalErr(err)
	}

	// Check status code - accept 200 OK and 404 (usage not available)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(responseBody)).
			WithOperation("get_quota_usage").
			WithStatusCode(resp.StatusCode)
	}

	// For 404, return empty response as usage may not be available yet
	if resp.StatusCode == http.StatusNotFound {
		return &GetQuotaUsageResponse{
			Records: []UsageRecord{},
			Summary: UsageSummary{
				StartTime: time.Now().AddDate(0, -1, 0).Format(time.RFC3339),
				EndTime:   time.Now().Format(time.RFC3339),
			},
		}, nil
	}

	// Parse response
	var usageResp GetQuotaUsageResponse
	if err := json.Unmarshal(responseBody, &usageResp); err != nil {
		return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse response").
			WithOperation("get_quota_usage").
			WithOriginalErr(err)
	}

	return &usageResp, nil
}

// GetQuotaHistory retrieves historical usage records.
//
// Endpoint: POST /v1internal/projects/{project}:getQuotaHistory
//
// Parameters:
//   - ctx: Context for the request
//   - projectID: The Google Cloud project ID
//   - req: The get quota history request parameters
//
// Returns:
//   - GetQuotaHistoryResponse: Historical usage records
//   - error: Any error that occurred during the request
func (c *CodeAssistClient) GetQuotaHistory(ctx context.Context, projectID string, req GetQuotaHistoryRequest) (*GetQuotaHistoryResponse, error) {
	// Build URL: POST /v1internal/projects/{project}:getQuotaHistory
	url := fmt.Sprintf("%s/projects/%s%s", c.baseURL, projectID, getQuotaHistoryRoute)

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal request").
			WithOperation("get_quota_history").
			WithOriginalErr(err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to create request").
			WithOperation("get_quota_history").
			WithOriginalErr(err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", telemetry.GetUserAgent())
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.oauthToken))

	// Make the request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "request failed").
			WithOperation("get_quota_history").
			WithOriginalErr(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to read response body").
			WithOperation("get_quota_history").
			WithOriginalErr(err)
	}

	// Check status code - accept 200 OK and 404 (history not available)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		errCode := types.ClassifyHTTPError(resp.StatusCode)
		return nil, types.NewProviderError(types.ProviderTypeGemini, errCode, string(responseBody)).
			WithOperation("get_quota_history").
			WithStatusCode(resp.StatusCode)
	}

	// For 404, return empty response as history may not be available yet
	if resp.StatusCode == http.StatusNotFound {
		return &GetQuotaHistoryResponse{
			Records: []UsageRecord{},
			Summary: UsageSummary{
				StartTime: time.Now().AddDate(0, -1, 0).Format(time.RFC3339),
				EndTime:   time.Now().Format(time.RFC3339),
			},
		}, nil
	}

	// Parse response
	var historyResp GetQuotaHistoryResponse
	if err := json.Unmarshal(responseBody, &historyResp); err != nil {
		return nil, types.NewServerError(types.ProviderTypeGemini, 0, "failed to parse response").
			WithOperation("get_quota_history").
			WithOriginalErr(err)
	}

	return &historyResp, nil
}

// ConvertToQuotaInfo converts a GetQuotaResponse to the package-level QuotaInfo.
// This implements a helper for Code Assist quota compatibility with the quota package.
func (c *CodeAssistClient) ConvertToQuotaInfo(resp *GetQuotaResponse, model string) *quota.QuotaInfo {
	if resp == nil {
		return nil
	}

	info := &quota.QuotaInfo{
		Provider:     "gemini",
		ProviderType: types.ProviderTypeGemini,
		Model:        model,
		Timestamp:    time.Now(),
		Quotas:       make(map[quota.QuotaType]*quota.QuotaUsage),
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
		var qt quota.QuotaType
		var qp quota.QuotaPeriod

		// Map quota types
		switch {
		case q.Type == "requests" || q.Type == "api_requests":
			qt = quota.QuotaTypeRequests
		case q.Type == "tokens" || q.Type == "total_tokens":
			qt = quota.QuotaTypeTokens
		case q.Type == "input_tokens":
			qt = quota.QuotaTypeInputTokens
		case q.Type == "output_tokens":
			qt = quota.QuotaTypeOutputTokens
		case q.Type == "daily_tokens" || q.Type == "daily_requests":
			qt = quota.QuotaTypeDaily
		default:
			qt = quota.QuotaTypeCustom
		}

		// Map quota periods
		switch q.Period {
		case "minute", "1m":
			qp = quota.QuotaPeriodMinute
		case "hour", "1h":
			qp = quota.QuotaPeriodHour
		case "day", "1d", "daily":
			qp = quota.QuotaPeriodDay
		case "week", "1w":
			qp = quota.QuotaPeriodWeek
		case "month", "1M":
			qp = quota.QuotaPeriodMonth
		default:
			qp = quota.QuotaPeriodCustom
		}

		// Parse reset time
		var resetAt time.Time
		if q.ResetTime != "" {
			// Parse as RFC3339 timestamp
			resetAt, _ = time.Parse(time.RFC3339, q.ResetTime)
		}

		// Calculate remaining percent
		remainingPercent := 0.0
		if q.Limit > 0 {
			remainingPercent = float64(q.Remaining) / float64(q.Limit) * 100
		}

		// Determine period start time
		var periodStartedAt time.Time
		if !resetAt.IsZero() {
			switch qp {
			case quota.QuotaPeriodDay:
				periodStartedAt = resetAt.AddDate(0, 0, -1)
			case quota.QuotaPeriodMonth:
				periodStartedAt = resetAt.AddDate(0, -1, 0)
			case quota.QuotaPeriodHour:
				periodStartedAt = resetAt.Add(-time.Hour)
			case quota.QuotaPeriodMinute:
				periodStartedAt = resetAt.Add(-time.Minute)
			}
		}

		info.Quotas[qt] = &quota.QuotaUsage{
			Type:             qt,
			Period:           qp,
			Used:             int(q.Usage),
			Limit:            int(q.Limit),
			Remaining:        int(q.Remaining),
			RemainingPercent: remainingPercent,
			ResetAt:          resetAt,
			PeriodStartedAt:  periodStartedAt,
		}
	}

	return info
}

// ConvertToQuotaHistory converts a GetQuotaHistoryResponse to the package-level QuotaHistory.
func (c *CodeAssistClient) ConvertToQuotaHistory(resp *GetQuotaHistoryResponse, projectID string, model string) *quota.QuotaHistory {
	if resp == nil {
		return nil
	}

	history := &quota.QuotaHistory{
		Provider:   "gemini",
		Model:      model,
		Records:    make([]*quota.QuotaRecord, 0, len(resp.Records)),
		TotalUsage: make(map[quota.QuotaType]int),
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

		usage := map[quota.QuotaType]int{}
		if record.InputTokens > 0 {
			usage[quota.QuotaTypeInputTokens] = int(record.InputTokens)
		}
		if record.OutputTokens > 0 {
			usage[quota.QuotaTypeOutputTokens] = int(record.OutputTokens)
		}
		if record.TotalTokens > 0 {
			usage[quota.QuotaTypeTokens] = int(record.TotalTokens)
		}
		if record.RequestCount > 0 {
			usage[quota.QuotaTypeRequests] = int(record.RequestCount)
		}

		// Record the model actually used in the record
		recordModel := record.Model
		if model != "" && recordModel == "" {
			recordModel = model
		}

		history.Records = append(history.Records, &quota.QuotaRecord{
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

// SupportsQuotaReporting returns true if the Code Assist client supports quota reporting.
// This checks if an OAuth token is available.
func (c *CodeAssistClient) SupportsQuotaReporting() bool {
	return c.oauthToken != ""
}

// SupportsQuotaHistory returns true if the Code Assist client supports quota history.
func (c *CodeAssistClient) SupportsQuotaHistory() bool {
	return c.oauthToken != ""
}
