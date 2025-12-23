// Package gemini provides OAuth-related functionality for Google Gemini AI provider.
package gemini

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/common/telemetry"
	commonhttp "github.com/cecil-the-coder/ai-provider-kit/internal/common/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OAuth constants for Google
const (
	// Google OAuth endpoints
	// #nosec G101 - These are public Google OAuth endpoints, not credentials
	googleTokenInfoURL = "https://www.googleapis.com/oauth2/v3/tokeninfo"
	googleAuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"

	// Google OAuth scopes for Gemini API
	geminiOAuthScope = "https://www.googleapis.com/auth/cloud-platform"
)

// ValidateToken validates the current OAuth token and returns detailed token information
func (p *GeminiProvider) ValidateToken(ctx context.Context) (*types.TokenInfo, error) {
	// Get current OAuth credentials
	if p.authHelper.OAuthManager == nil {
		return nil, types.NewAuthError(types.ProviderTypeGemini, "no OAuth manager configured").
			WithOperation("validate_token")
	}

	creds := p.authHelper.OAuthManager.GetCredentials()
	if len(creds) == 0 {
		return nil, types.NewAuthError(types.ProviderTypeGemini, "no OAuth credentials available").
			WithOperation("validate_token")
	}

	// Use the first available credential for validation
	// In practice, you might want to validate all credentials or select a specific one
	cred := creds[0]

	// Make request to Google's tokeninfo endpoint
	reqURL := fmt.Sprintf("%s?access_token=%s", googleTokenInfoURL, cred.AccessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to create token validation request").
			WithOperation("validate_token").
			WithOriginalErr(err)
	}

	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, types.NewNetworkError(types.ProviderTypeGemini, "failed to validate token").
			WithOperation("validate_token").
			WithOriginalErr(err)
	}
	defer commonhttp.SafeClose(resp)

	if err := commonhttp.HandleHTTPStatusError(resp, types.ProviderTypeGemini, "validate_token", "token validation failed"); err != nil {
		return nil, err
	}

	// Parse tokeninfo response
	var tokenInfoResponse struct {
		Aud           string `json:"aud"`
		Scope         string `json:"scope"`
		ExpiresIn     int64  `json:"expires_in"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Issuer        string `json:"iss"`
	}

	if err := commonhttp.UnmarshalJSONResponse(resp.Body, &tokenInfoResponse, types.ProviderTypeGemini, "validate_token"); err != nil {
		return nil, err
	}

	// Parse scopes
	var scopes []string
	if tokenInfoResponse.Scope != "" {
		scopes = strings.Split(tokenInfoResponse.Scope, " ")
	}

	// Calculate expiration time
	var expiresAt time.Time
	if tokenInfoResponse.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(tokenInfoResponse.ExpiresIn) * time.Second)
	} else {
		// Use stored expiration time if tokeninfo doesn't provide it
		expiresAt = cred.ExpiresAt
	}

	// Build user info
	userInfo := map[string]interface{}{
		"email":          tokenInfoResponse.Email,
		"email_verified": tokenInfoResponse.EmailVerified,
		"audience":       tokenInfoResponse.Aud,
		"issuer":         tokenInfoResponse.Issuer,
	}

	// Check if token is valid
	valid := time.Now().Before(expiresAt) && len(scopes) > 0

	return &types.TokenInfo{
		Valid:     valid,
		ExpiresAt: expiresAt,
		Scope:     scopes,
		UserInfo:  userInfo,
	}, nil
}

// RefreshToken refreshes the OAuth token using the stored refresh token
func (p *GeminiProvider) RefreshToken(ctx context.Context) error {
	// Use the existing OAuth refresh functionality
	return p.RefreshAllOAuthTokens(ctx)
}

// GetAuthURL generates an OAuth authorization URL for re-authentication
func (p *GeminiProvider) GetAuthURL(redirectURI string, state string) string {
	// Default client ID for Gemini/Google Cloud - this should be configurable
	// #nosec G101 - This is a public Google OAuth client ID, not a secret credential
	clientID := "936875672307-4r5272sc9k0c2e2d6dr3uj63btk3revo.apps.googleusercontent.com"

	// Note: This uses the default client ID for Gemini/Google Cloud OAuth.
	// Custom client ID configuration can be added in future versions.

	// Build the OAuth URL with required parameters
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		googleAuthURL,
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(geminiOAuthScope),
		url.QueryEscape(state),
	)

	// Add additional parameters for better UX
	authURL += "&access_type=offline&prompt=consent"

	return authURL
}
