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

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
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
