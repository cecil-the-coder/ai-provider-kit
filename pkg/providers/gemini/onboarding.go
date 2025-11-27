package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// SetupUserProject performs the full onboarding flow and returns the Google Cloud project ID that
// the user belongs to. It will create a new project for the user if necessary and poll
// for the long‑running onboard operation to finish.
//
// The method is analogous to the TypeScript implementation in
// llxprt-code/packages/core/src/code_assist/setup.ts.
func (p *GeminiProvider) SetupUserProject(ctx context.Context) (string, error) {
	// Initialize onboarding context
	projectID, metadata := p.initializeOnboardingContext()

	// Load current state
	loadRes, err := p.loadCodeAssist(ctx, projectID, metadata)
	if err != nil {
		return "", fmt.Errorf("loadCodeAssist failed: %w", err)
	}

	// Debug logging
	p.logLoadCodeAssistResponse(loadRes)

	// Check if user already has tier and project
	if projectID, exists := p.checkExistingProject(loadRes, projectID); exists {
		return projectID, nil
	}

	// No current tier, perform onboarding
	return p.performOnboarding(ctx, loadRes, projectID, metadata)
}

// loadCodeAssist calls the loadCodeAssist endpoint and returns the response.
func (p *GeminiProvider) loadCodeAssist(ctx context.Context, projectID *string, metadata ClientMetadata) (*LoadCodeAssistResponse, error) {
	fmt.Printf("Gemini: Calling loadCodeAssist\n")
	reqBody := LoadCodeAssistRequest{
		CloudaicompanionProject: projectID,
		Metadata:                metadata,
	}
	resp, err := p.makeOnboardingRequest(ctx, "POST", loadCodeAssistRoute, reqBody)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loadCodeAssist returned %d: %s", resp.StatusCode, string(body))
	}

	var loadRes LoadCodeAssistResponse
	if err := json.NewDecoder(resp.Body).Decode(&loadRes); err != nil {
		return nil, fmt.Errorf("failed to decode loadCodeAssist response: %w", err)
	}
	return &loadRes, nil
}

// onboardUser calls the onboardUser endpoint and returns the LRO response.
func (p *GeminiProvider) onboardUser(ctx context.Context, req OnboardUserRequest) (*LongRunningOperationResponse, error) {
	fmt.Printf("Gemini: Calling onboardUser\n")
	resp, err := p.makeOnboardingRequest(ctx, "POST", onboardUserRoute, req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:staticcheck // Empty branch is intentional - we ignore close errors

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("onboardUser returned %d: %s", resp.StatusCode, string(body))
	}

	var lroResp LongRunningOperationResponse
	if err := json.NewDecoder(resp.Body).Decode(&lroResp); err != nil {
		return nil, fmt.Errorf("failed to decode onboardUser response: %w", err)
	}
	return &lroResp, nil
}

// makeOnboardingRequest makes a request to the CloudCode onboarding endpoints
func (p *GeminiProvider) makeOnboardingRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	// Get first available OAuth credential for onboarding
	if p.authHelper.OAuthManager == nil {
		return nil, fmt.Errorf("no OAuth credentials available for onboarding")
	}

	creds := p.authHelper.OAuthManager.GetCredentials()
	if len(creds) == 0 {
		return nil, fmt.Errorf("no OAuth credentials available")
	}

	// Use first credential for onboarding
	cred := creds[0]

	// Serialize request
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/%s", cloudcodeBaseURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cred.AccessToken))

	return p.client.Do(req)
}

// getOnboardTier chooses the default tier from the loadCodeAssist response or
// returns a minimal fallback if none is marked default.
func getOnboardTier(res *LoadCodeAssistResponse) *GeminiUserTier {
	if res == nil {
		return nil
	}
	for i := range res.AllowedTiers {
		tier := &res.AllowedTiers[i]
		if tier.IsDefault != nil && *tier.IsDefault {
			return tier
		}
	}
	// Fallback: return legacy tier with userDefinedCloudaicompanionProject true.
	return &GeminiUserTier{
		ID:                                 UserTierIDLegacy,
		Name:                               "",
		Description:                        "",
		UserDefinedCloudaicompanionProject: boolPtr(true),
		IsDefault:                          boolPtr(false),
	}
}

// =============================================================================
// Helper Functions for Onboarding
// =============================================================================

// initializeOnboardingContext initializes the onboarding context with project ID and metadata
func (p *GeminiProvider) initializeOnboardingContext() (*string, ClientMetadata) {
	// Fetch project ID from env if present
	var projectID *string
	if id := os.Getenv("GOOGLE_CLOUD_PROJECT"); id != "" {
		projectID = &id
	}

	metadata := ClientMetadata{
		IDEType:    IDETypeUnspecified,
		Platform:   PlatformUnspecified,
		PluginType: PluginTypeGemini,
	}

	return projectID, metadata
}

// logLoadCodeAssistResponse logs detailed information from the loadCodeAssist response
func (p *GeminiProvider) logLoadCodeAssistResponse(loadRes *LoadCodeAssistResponse) {
	fmt.Printf("Gemini: loadCodeAssist response - CurrentTier: %v, CloudaicompanionProject: %v, AllowedTiers count: %d\n",
		loadRes.CurrentTier != nil, loadRes.CloudaicompanionProject, len(loadRes.AllowedTiers))

	if loadRes.CurrentTier != nil {
		fmt.Printf("Gemini: Current tier ID: %s\n", loadRes.CurrentTier.ID)
	}

	for i, tier := range loadRes.AllowedTiers {
		isDefault := "nil"
		if tier.IsDefault != nil {
			isDefault = fmt.Sprintf("%v", *tier.IsDefault)
		}
		userDefined := "nil"
		if tier.UserDefinedCloudaicompanionProject != nil {
			userDefined = fmt.Sprintf("%v", *tier.UserDefinedCloudaicompanionProject)
		}
		fmt.Printf("Gemini: AllowedTier[%d] - ID: %s, Name: %s, IsDefault: %s, UserDefinedProject: %s\n",
			i, tier.ID, tier.Name, isDefault, userDefined)
	}
}

// checkExistingProject checks if user already has a tier and project, returns it if found
func (p *GeminiProvider) checkExistingProject(loadRes *LoadCodeAssistResponse, projectID *string) (string, bool) {
	if loadRes.CurrentTier == nil {
		return "", false
	}

	// Project from response, if any
	if loadRes.CloudaicompanionProject != nil && *loadRes.CloudaicompanionProject != "" {
		fmt.Printf("Gemini: User has currentTier, returning project from response: %s\n", *loadRes.CloudaicompanionProject)
		return *loadRes.CloudaicompanionProject, true
	}

	// Fallback to env project ID if provided
	if projectID != nil && *projectID != "" {
		fmt.Printf("Gemini: User has currentTier but no project in response, using env project ID: %s\n", *projectID)
		return *projectID, true
	}

	fmt.Printf("Gemini: User has currentTier but no project available\n")
	return "", false
}

// performOnboarding performs the onboarding process for users without a current tier
func (p *GeminiProvider) performOnboarding(ctx context.Context, loadRes *LoadCodeAssistResponse, projectID *string, metadata ClientMetadata) (string, error) {
	// No current tier, determine which tier to onboard
	tier := getOnboardTier(loadRes)
	if tier == nil {
		return "", fmt.Errorf("no onboard tier found")
	}

	if tier.UserDefinedCloudaicompanionProject != nil && *tier.UserDefinedCloudaicompanionProject && projectID == nil {
		return "", &ProjectIDRequiredError{}
	}

	// Prepare onboard request
	onboardReq := p.prepareOnboardRequest(tier, projectID, metadata)

	// Call onboardUser and poll until done
	lro, err := p.pollOnboardUser(ctx, onboardReq)
	if err != nil {
		return "", fmt.Errorf("onboardUser failed: %w", err)
	}

	// Extract project ID from response
	return p.extractProjectIDFromResponse(lro, projectID)
}

// prepareOnboardRequest prepares the onboarding request based on tier and project
func (p *GeminiProvider) prepareOnboardRequest(tier *GeminiUserTier, projectID *string, metadata ClientMetadata) OnboardUserRequest {
	onboardReq := OnboardUserRequest{
		TierID:   &tier.ID,
		Metadata: &metadata,
	}

	if tier.ID == UserTierIDFree {
		// Free tier uses managed project; skip explicit project ID
		onboardReq.CloudaicompanionProject = nil
	} else {
		onboardReq.CloudaicompanionProject = projectID
		// Include duetProject in metadata for non-free tiers
		if projectID != nil {
			metadata.DuetProject = *projectID
			onboardReq.Metadata = &metadata
		}
	}

	return onboardReq
}

// pollOnboardUser calls onboardUser and polls until the operation is done
func (p *GeminiProvider) pollOnboardUser(ctx context.Context, onboardReq OnboardUserRequest) (*LongRunningOperationResponse, error) {
	lro, err := p.onboardUser(ctx, onboardReq)
	if err != nil {
		return nil, fmt.Errorf("onboardUser failed: %w", err)
	}

	for !lro.Done {
		fmt.Printf("Gemini: onboardUser LRO not done, sleeping %s\n", pollInterval)
		time.Sleep(pollInterval)
		lro, err = p.onboardUser(ctx, onboardReq)
		if err != nil {
			return nil, fmt.Errorf("while polling onboardUser: %w", err)
		}
	}

	return lro, nil
}

// extractProjectIDFromResponse extracts the project ID from the onboarding response
func (p *GeminiProvider) extractProjectIDFromResponse(lro *LongRunningOperationResponse, projectID *string) (string, error) {
	// Inspect response for the project ID
	if lro.Response != nil && lro.Response.CloudaicompanionProject != nil && lro.Response.CloudaicompanionProject.ID != "" {
		return lro.Response.CloudaicompanionProject.ID, nil
	}

	if projectID != nil && *projectID != "" {
		return *projectID, nil
	}

	return "", &ProjectIDRequiredError{}
}
