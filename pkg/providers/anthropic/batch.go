// Package anthropic provides batch processing support for Anthropic Claude AI provider.
// The Message Batches API allows asynchronous processing of large volumes of requests
// with 50% cost savings compared to standard API pricing.
package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// BatchRequest represents a single request in a message batch
type BatchRequest struct {
	CustomID string             `json:"custom_id"` // Unique identifier for the request
	Params   BatchRequestParams `json:"params"`    // Standard Messages API parameters
}

// BatchRequestParams contains the parameters for a message request in a batch
type BatchRequestParams struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	System        interface{}        `json:"system,omitempty"` // Can be string or []interface{}
	Messages      []AnthropicMessage `json:"messages"`
	Tools         []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice    interface{}        `json:"tool_choice,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	TopK          *int               `json:"top_k,omitempty"`
}

// BatchCreateRequest represents the request to create a new message batch
type BatchCreateRequest struct {
	Requests []BatchRequest `json:"requests"`
}

// BatchRequestCounts represents the count of requests in different states
type BatchRequestCounts struct {
	Processing int `json:"processing"` // Requests currently being processed
	Succeeded  int `json:"succeeded"`  // Successfully processed requests
	Errored    int `json:"errored"`    // Requests that encountered errors
	Canceled   int `json:"canceled"`   // Requests canceled by user
	Expired    int `json:"expired"`    // Requests that expired after 24 hours
}

// MessageBatch represents a batch of message requests
type MessageBatch struct {
	ID                string             `json:"id"`                  // Unique identifier for the batch
	Type              string             `json:"type"`                // Always "message_batch"
	ProcessingStatus  string             `json:"processing_status"`   // "in_progress", "canceling", or "ended"
	RequestCounts     BatchRequestCounts `json:"request_counts"`      // Count of requests in each state
	EndedAt           *time.Time         `json:"ended_at"`            // Time when processing ended
	CreatedAt         time.Time          `json:"created_at"`          // Time when batch was created
	ExpiresAt         time.Time          `json:"expires_at"`          // Time when batch expires (24 hours after creation)
	CancelInitiatedAt *time.Time         `json:"cancel_initiated_at"` // Time when cancellation was initiated
	ResultsURL        *string            `json:"results_url"`         // URL to download results (available when ended)
}

// BatchListResponse represents the response from listing batches
type BatchListResponse struct {
	Data    []MessageBatch `json:"data"`     // List of message batches
	HasMore bool           `json:"has_more"` // Whether there are more results
	FirstID *string        `json:"first_id"` // ID of the first batch in the list
	LastID  *string        `json:"last_id"`  // ID of the last batch in the list
}

// BatchResult represents the result of a single request in a batch
type BatchResult struct {
	CustomID string          `json:"custom_id"` // Custom ID from the request
	Result   BatchResultType `json:"result"`    // The result of the request
}

// BatchResultType represents the different types of batch results
type BatchResultType struct {
	Type    string             `json:"type"`              // "succeeded", "errored", "canceled", or "expired"
	Message *AnthropicResponse `json:"message,omitempty"` // The message result (for succeeded)
	Error   *BatchError        `json:"error,omitempty"`   // Error details (for errored)
}

// BatchError represents an error in a batch result
type BatchError struct {
	Type    string `json:"type"`    // Error type (e.g., "invalid_request", "overloaded")
	Message string `json:"message"` // Human-readable error message
}

// CreateBatch creates a new message batch for asynchronous processing
// Returns the created MessageBatch with processing status "in_progress"
func (p *AnthropicProvider) CreateBatch(ctx context.Context, requests []BatchRequest) (*MessageBatch, error) {
	if len(requests) == 0 {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "batch must contain at least one request").
			WithOperation("CreateBatch")
	}

	// Validate batch size limits
	if len(requests) > 100000 {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "batch cannot contain more than 100,000 requests").
			WithOperation("CreateBatch")
	}

	requestData := BatchCreateRequest{
		Requests: requests,
	}

	// Define OAuth operation
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		return p.makeBatchAPICallWithAuth(ctx, requestData, cred.AccessToken, "oauth")
	}

	// Define API key operation
	apiKeyOperation := func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
		return p.makeBatchAPICallWithAuth(ctx, requestData, apiKey, "api_key")
	}

	// Use auth helper to execute with automatic failover
	responseJSON, _, err := p.authHelper.ExecuteWithAuth(ctx, types.GenerateOptions{}, oauthOperation, apiKeyOperation)
	if err != nil {
		return nil, err
	}

	var batch MessageBatch
	if err := json.Unmarshal([]byte(responseJSON), &batch); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "failed to parse batch response").
			WithOperation("CreateBatch").
			WithOriginalErr(err)
	}

	return &batch, nil
}

// GetBatch retrieves the status and details of a specific message batch
func (p *AnthropicProvider) GetBatch(ctx context.Context, batchID string) (*MessageBatch, error) {
	if batchID == "" {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "batch ID is required").
			WithOperation("GetBatch")
	}

	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := fmt.Sprintf("%s/v1/messages/batches/%s", baseURL, batchID)

	// Define OAuth operation
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		return p.makeBatchGetRequest(ctx, url, cred.AccessToken, "oauth")
	}

	// Define API key operation
	apiKeyOperation := func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
		return p.makeBatchGetRequest(ctx, url, apiKey, "api_key")
	}

	// Use auth helper to execute with automatic failover
	responseJSON, _, err := p.authHelper.ExecuteWithAuth(ctx, types.GenerateOptions{}, oauthOperation, apiKeyOperation)
	if err != nil {
		return nil, err
	}

	var batch MessageBatch
	if err := json.Unmarshal([]byte(responseJSON), &batch); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "failed to parse batch response").
			WithOperation("GetBatch").
			WithOriginalErr(err)
	}

	return &batch, nil
}

// ListBatches retrieves a paginated list of message batches
// Parameters:
//   - limit: Maximum number of batches to return (1-100, default 20)
//   - beforeID: Return batches created before this batch ID
//   - afterID: Return batches created after this batch ID
func (p *AnthropicProvider) ListBatches(ctx context.Context, limit int, beforeID, afterID string) (*BatchListResponse, error) {
	if limit < 1 {
		limit = 20 // Default limit
	}
	if limit > 100 {
		limit = 100 // Max limit
	}

	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := fmt.Sprintf("%s/v1/messages/batches?limit=%d", baseURL, limit)

	if beforeID != "" {
		url += fmt.Sprintf("&before_id=%s", beforeID)
	}
	if afterID != "" {
		url += fmt.Sprintf("&after_id=%s", afterID)
	}

	// Define OAuth operation
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		return p.makeBatchGetRequest(ctx, url, cred.AccessToken, "oauth")
	}

	// Define API key operation
	apiKeyOperation := func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
		return p.makeBatchGetRequest(ctx, url, apiKey, "api_key")
	}

	// Use auth helper to execute with automatic failover
	responseJSON, _, err := p.authHelper.ExecuteWithAuth(ctx, types.GenerateOptions{}, oauthOperation, apiKeyOperation)
	if err != nil {
		return nil, err
	}

	var response BatchListResponse
	if err := json.Unmarshal([]byte(responseJSON), &response); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "failed to parse batch list response").
			WithOperation("ListBatches").
			WithOriginalErr(err)
	}

	return &response, nil
}

// CancelBatch cancels a message batch that is currently processing
// The batch will transition to "canceling" status and eventually to "ended"
// Cancelled batches may contain partial results for requests processed before cancellation
func (p *AnthropicProvider) CancelBatch(ctx context.Context, batchID string) (*MessageBatch, error) {
	if batchID == "" {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "batch ID is required").
			WithOperation("CancelBatch")
	}

	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := fmt.Sprintf("%s/v1/messages/batches/%s/cancel", baseURL, batchID)

	// Define OAuth operation
	oauthOperation := func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
		return p.makeBatchPostRequest(ctx, url, nil, cred.AccessToken, "oauth")
	}

	// Define API key operation
	apiKeyOperation := func(ctx context.Context, apiKey string) (string, *types.Usage, error) {
		return p.makeBatchPostRequest(ctx, url, nil, apiKey, "api_key")
	}

	// Use auth helper to execute with automatic failover
	responseJSON, _, err := p.authHelper.ExecuteWithAuth(ctx, types.GenerateOptions{}, oauthOperation, apiKeyOperation)
	if err != nil {
		return nil, err
	}

	var batch MessageBatch
	if err := json.Unmarshal([]byte(responseJSON), &batch); err != nil {
		return nil, types.NewInvalidRequestError(types.ProviderTypeAnthropic, "failed to parse batch response").
			WithOperation("CancelBatch").
			WithOriginalErr(err)
	}

	return &batch, nil
}

// StreamBatchResults streams the results of a completed batch using NDJSON format
// Each line in the stream is a JSON object representing the result of a single request
// Results are streamed to avoid loading large result sets into memory
// The callback function is called for each result line
func (p *AnthropicProvider) StreamBatchResults(ctx context.Context, batchID string, callback func(BatchResult) error) error {
	if batchID == "" {
		return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "batch ID is required").
			WithOperation("StreamBatchResults")
	}

	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := fmt.Sprintf("%s/v1/messages/batches/%s/results", baseURL, batchID)

	// Determine which credential to use
	var credential string
	var authType string

	if p.authHelper.OAuthManager != nil {
		creds := p.authHelper.OAuthManager.GetCredentialsWithContext(ctx)
		if len(creds) > 0 {
			credential = creds[0].AccessToken
			authType = "oauth"
		}
	}

	if credential == "" && p.authHelper.KeyManager != nil {
		keys := p.authHelper.KeyManager.GetKeys()
		if len(keys) > 0 {
			credential = keys[0]
			authType = "api_key"
		}
	}

	if credential == "" {
		return types.NewAuthError(types.ProviderTypeAnthropic, "no authentication credentials available").
			WithOperation("StreamBatchResults")
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "failed to create request").
			WithOperation("StreamBatchResults").
			WithOriginalErr(err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, credential, authType)
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "request failed").
			WithOperation("StreamBatchResults").
			WithOriginalErr(err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResponse AnthropicErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return types.NewServerError(types.ProviderTypeAnthropic, resp.StatusCode, errorResponse.Error.Message).
				WithOperation("StreamBatchResults")
		}
		return types.NewServerError(types.ProviderTypeAnthropic, resp.StatusCode, string(body)).
			WithOperation("StreamBatchResults")
	}

	// Stream results using NDJSON decoder
	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer size for large result lines (max 10MB per line)
	const maxScanTokenSize = 10 * 1024 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var result BatchResult
		if err := json.Unmarshal(line, &result); err != nil {
			return types.NewInvalidRequestError(types.ProviderTypeAnthropic, "failed to parse batch result").
				WithOperation("StreamBatchResults").
				WithOriginalErr(err)
		}

		// Call the callback function with the result
		if err := callback(result); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return types.NewNetworkError(types.ProviderTypeAnthropic, "error reading batch results stream").
			WithOperation("StreamBatchResults").
			WithOriginalErr(err)
	}

	return nil
}

// makeBatchAPICallWithAuth makes an API call to create a batch with authentication
func (p *AnthropicProvider) makeBatchAPICallWithAuth(ctx context.Context, requestData BatchCreateRequest, credential, authType string) (string, *types.Usage, error) {
	config := p.GetConfig()
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages/batches"

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Prepare JSON body
	jsonBody, err := p.requestHandler.PrepareJSONBody(requestData)
	if err != nil {
		return "", nil, err
	}
	req.Body = io.NopCloser(jsonBody)

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, credential, authType)
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request
	resp, err := p.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResponse AnthropicErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return "", nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		}
		return "", nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, string(body))
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), &types.Usage{}, nil
}

// makeBatchGetRequest makes a GET request with authentication
func (p *AnthropicProvider) makeBatchGetRequest(ctx context.Context, url, credential, authType string) (string, *types.Usage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, credential, authType)
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := p.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResponse AnthropicErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return "", nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		}
		return "", nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), &types.Usage{}, nil
}

// makeBatchPostRequest makes a POST request with authentication (for cancel)
func (p *AnthropicProvider) makeBatchPostRequest(ctx context.Context, url string, requestData interface{}, credential, authType string) (string, *types.Usage, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Prepare JSON body if there's request data
	if requestData != nil {
		jsonBody, err := p.requestHandler.PrepareJSONBody(requestData)
		if err != nil {
			return "", nil, err
		}
		req.Body = io.NopCloser(jsonBody)
	}

	// Set authentication headers
	p.authHelper.SetAuthHeaders(req, credential, authType)
	p.authHelper.SetProviderSpecificHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := p.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResponse AnthropicErrorResponse
		if parseErr := json.Unmarshal(body, &errorResponse); parseErr == nil {
			return "", nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, errorResponse.Error.Message)
		}
		return "", nil, fmt.Errorf("anthropic API error: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), &types.Usage{}, nil
}
