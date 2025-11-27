// Package common provides shared utilities and helper functions for AI provider implementations
package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/oauthmanager"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuthRefreshHelper provides common OAuth token refresh implementations
type OAuthRefreshHelper struct {
	ProviderName string
	HTTPClient   *http.Client
}

// NewOAuthRefreshHelper creates a new OAuth refresh helper
func NewOAuthRefreshHelper(providerName string, client *http.Client) *OAuthRefreshHelper {
	return &OAuthRefreshHelper{
		ProviderName: providerName,
		HTTPClient:   client,
	}
}

// AnthropicOAuthRefresh implements Anthropic's OAuth 2.0 token refresh
func (h *OAuthRefreshHelper) AnthropicOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
	// Anthropic uses JSON format for token refresh
	requestBody := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     h.getClientID(cred, "9d1c250a-e61b-44d9-88ed-5944d1962f5e"), // Default MAX plan client ID
		"refresh_token": cred.RefreshToken,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refresh request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://console.anthropic.com/v1/oauth/token", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return h.updateCredentialFromResponse(cred, tokenResponse.AccessToken, tokenResponse.RefreshToken, time.Duration(tokenResponse.ExpiresIn)*time.Second), nil
}

// OpenAIOAuthRefresh implements OpenAI's OAuth 2.0 token refresh
func (h *OAuthRefreshHelper) OpenAIOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
	// OpenAI uses form-encoded format for token refresh
	formData := url.Values{}
	formData.Set("grant_type", "refresh_token")
	formData.Set("client_id", h.getClientID(cred, "app_EMoamEEZ73f0CkXaXp7hrann")) // Default public client ID
	formData.Set("refresh_token", cred.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://auth.openai.com/oauth/token", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		IDToken      string `json:"id_token"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return h.updateCredentialFromResponse(cred, tokenResponse.AccessToken, tokenResponse.RefreshToken, time.Duration(tokenResponse.ExpiresIn)*time.Second), nil
}

// GeminiOAuthRefresh implements Google's OAuth 2.0 token refresh using the oauth2 library
func (h *OAuthRefreshHelper) GeminiOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
	oauth2Config := &oauth2.Config{
		ClientID:     cred.ClientID,
		ClientSecret: cred.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       cred.Scopes,
	}

	token := &oauth2.Token{
		RefreshToken: cred.RefreshToken,
	}

	// Use oauth2 library to refresh
	tokenSource := oauth2Config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Gemini OAuth token: %w", err)
	}

	// Calculate expiry time
	var expiresAt time.Time
	if newToken.Expiry.IsZero() {
		// Default to 1 hour if no expiry provided
		expiresAt = time.Now().Add(time.Hour)
	} else {
		expiresAt = newToken.Expiry
	}

	return h.updateCredentialFromResponse(cred, newToken.AccessToken, newToken.RefreshToken, time.Until(expiresAt)), nil
}

// QwenOAuthRefresh implements Qwen's OAuth 2.0 token refresh
func (h *OAuthRefreshHelper) QwenOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
	// Use Qwen's hardcoded client ID if no custom client_id is provided
	clientID := h.getClientID(cred, "f0304373b74a44d2b584a3fb70ca9e56") // Qwen's public client ID

	// Use direct string formatting like the working debug implementation (not url.Values)
	data := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s", cred.RefreshToken, clientID)

	// Only include client_secret if it's not empty (Qwen device flow doesn't use it)
	if cred.ClientSecret != "" {
		data += fmt.Sprintf("&client_secret=%s", cred.ClientSecret)
	}

	// Use Qwen OAuth endpoint
	//nolint:gosec // This is a public OAuth endpoint, not a hardcoded credential
	tokenURL := "https://chat.qwen.ai/api/v1/oauth2/token"

	// Create request
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create token refresh request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var refreshResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&refreshResponse); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	if refreshResponse.AccessToken == "" {
		return nil, fmt.Errorf("refresh response missing access token")
	}

	return h.updateCredentialFromResponse(cred, refreshResponse.AccessToken, refreshResponse.RefreshToken, time.Duration(refreshResponse.ExpiresIn)*time.Second), nil
}

// GenericOAuthRefresh provides a generic OAuth 2.0 token refresh implementation
func (h *OAuthRefreshHelper) GenericOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet, tokenURL string) (*types.OAuthCredentialSet, error) {
	// Use form-encoded format as it's most common
	formData := url.Values{}
	formData.Set("grant_type", "refresh_token")
	formData.Set("refresh_token", cred.RefreshToken)

	if cred.ClientID != "" {
		formData.Set("client_id", cred.ClientID)
	}
	if cred.ClientSecret != "" {
		formData.Set("client_secret", cred.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return h.updateCredentialFromResponse(cred, tokenResponse.AccessToken, tokenResponse.RefreshToken, time.Duration(tokenResponse.ExpiresIn)*time.Second), nil
}

// Helper methods

// getClientID returns the client ID from credential or falls back to default
func (h *OAuthRefreshHelper) getClientID(cred *types.OAuthCredentialSet, defaultID string) string {
	if cred.ClientID != "" {
		return cred.ClientID
	}
	return defaultID
}

// updateCredentialFromResponse creates an updated credential set from token response
func (h *OAuthRefreshHelper) updateCredentialFromResponse(cred *types.OAuthCredentialSet, accessToken, refreshToken string, expiresIn time.Duration) *types.OAuthCredentialSet {
	updated := *cred
	updated.AccessToken = accessToken
	if refreshToken != "" {
		updated.RefreshToken = refreshToken
	}
	updated.ExpiresAt = time.Now().Add(expiresIn)
	updated.LastRefresh = time.Now()
	updated.RefreshCount++
	return &updated
}

// RefreshFuncFactory creates refresh functions for different providers
type RefreshFuncFactory struct {
	Helper *OAuthRefreshHelper
}

// NewRefreshFuncFactory creates a new refresh function factory
func NewRefreshFuncFactory(providerName string, client *http.Client) *RefreshFuncFactory {
	return &RefreshFuncFactory{
		Helper: NewOAuthRefreshHelper(providerName, client),
	}
}

// CreateAnthropicRefreshFunc creates an Anthropic refresh function
func (f *RefreshFuncFactory) CreateAnthropicRefreshFunc() oauthmanager.RefreshFunc {
	return f.Helper.AnthropicOAuthRefresh
}

// CreateOpenAIRefreshFunc creates an OpenAI refresh function
func (f *RefreshFuncFactory) CreateOpenAIRefreshFunc() oauthmanager.RefreshFunc {
	return f.Helper.OpenAIOAuthRefresh
}

// CreateGeminiRefreshFunc creates a Gemini refresh function
func (f *RefreshFuncFactory) CreateGeminiRefreshFunc() oauthmanager.RefreshFunc {
	return f.Helper.GeminiOAuthRefresh
}

// CreateQwenRefreshFunc creates a Qwen refresh function
func (f *RefreshFuncFactory) CreateQwenRefreshFunc() oauthmanager.RefreshFunc {
	return f.Helper.QwenOAuthRefresh
}

// CreateGenericRefreshFunc creates a generic refresh function for custom providers
func (f *RefreshFuncFactory) CreateGenericRefreshFunc(tokenURL string) oauthmanager.RefreshFunc {
	return func(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
		return f.Helper.GenericOAuthRefresh(ctx, cred, tokenURL)
	}
}
