package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// OAuthAuthenticatorImpl implements OAuthAuthenticator
type OAuthAuthenticatorImpl struct {
	provider     string
	config       *types.OAuthConfig
	storage      TokenStorage
	client       *http.Client
	state        string
	lastAuth     time.Time
	isAuth       bool
	oauthConfig  *OAuthConfig
	pkceVerifier string
}

// NewOAuthAuthenticator creates a new OAuth authenticator
func NewOAuthAuthenticator(provider string, storage TokenStorage, config *OAuthConfig) *OAuthAuthenticatorImpl {
	return &OAuthAuthenticatorImpl{
		provider:    provider,
		storage:     storage,
		client:      &http.Client{Timeout: 30 * time.Second},
		oauthConfig: config,
	}
}

// Authenticate performs authentication with the given config
// Note: For OAuth authentication via this pkg/auth module, the OAuth config must be
// set during creation of the authenticator or via SetOAuthConfig.
// types.AuthConfig no longer supports OAuthConfig field.
func (a *OAuthAuthenticatorImpl) Authenticate(ctx context.Context, config types.AuthConfig) error {
	if config.Method != types.AuthMethodOAuth {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "OAuth authenticator only supports OAuth method",
		}
	}

	if a.config == nil {
		// Check if we have a stored token
		if storedToken, err := a.storage.RetrieveToken(a.provider); err == nil && storedToken != nil {
			a.config = storedToken
		} else {
			return &AuthError{
				Provider: a.provider,
				Code:     ErrCodeInvalidConfig,
				Message:  "OAuth config required - set via SetOAuthConfig or provide stored token",
			}
		}
	}

	// Check if token is valid
	if !a.isTokenExpired(a.config) {
		a.isAuth = true
		a.lastAuth = time.Now()
		return nil
	}

	// Token expired, try to refresh
	if err := a.RefreshToken(ctx); err != nil {
		// Refresh failed, need full OAuth flow
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeTokenExpired,
			Message:  "Stored token expired and refresh failed",
			Details:  err.Error(),
			Retry:    true,
		}
	}

	return nil
}

// SetOAuthConfig sets the OAuth configuration
func (a *OAuthAuthenticatorImpl) SetOAuthConfig(config *types.OAuthConfig) {
	a.config = config
}

// IsAuthenticated checks if currently authenticated
func (a *OAuthAuthenticatorImpl) IsAuthenticated() bool {
	if !a.isAuth || a.config == nil {
		return false
	}
	return !a.isTokenExpired(a.config)
}

// GetToken returns the current authentication token
func (a *OAuthAuthenticatorImpl) GetToken() (string, error) {
	if !a.IsAuthenticated() {
		return "", &AuthError{
			Provider: a.provider,
			Code:     ErrCodeTokenExpired,
			Message:  "Not authenticated",
		}
	}
	return a.config.AccessToken, nil
}

// RefreshToken refreshes the authentication token
func (a *OAuthAuthenticatorImpl) RefreshToken(ctx context.Context) error {
	if a.config == nil || a.config.RefreshToken == "" {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeRefreshFailed,
			Message:  "No refresh token available",
		}
	}

	// Check if refresh is enabled
	if a.oauthConfig != nil && !a.oauthConfig.Refresh.Enabled {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeRefreshFailed,
			Message:  "Token refresh is disabled",
		}
	}

	// Use token URL or refresh URL
	tokenURL := a.config.TokenURL
	if a.oauthConfig != nil && a.oauthConfig.Refresh.Enabled && a.oauthConfig.Refresh.Buffer > 0 {
		// Apply refresh buffer
		if !a.config.ExpiresAt.IsZero() && time.Until(a.config.ExpiresAt) > a.oauthConfig.Refresh.Buffer {
			return nil // Token is still valid, no refresh needed
		}
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", a.config.RefreshToken)
	data.Set("client_id", a.config.ClientID)
	data.Set("client_secret", a.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeNetworkError,
			Message:  "Failed to create refresh request",
			Details:  err.Error(),
		}
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Set user agent if configured
	if a.oauthConfig != nil && a.oauthConfig.HTTP.UserAgent != "" {
		req.Header.Set("User-Agent", a.oauthConfig.HTTP.UserAgent)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeNetworkError,
			Message:  "Network error during token refresh",
			Details:  err.Error(),
			Retry:    true,
		}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log the error but don't fail the operation
			// In a real application, you might want to log this
			_ = fmt.Sprintf("failed to close response body during token refresh: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeRefreshFailed,
			Message:  fmt.Sprintf("Token refresh failed with status %d", resp.StatusCode),
			Retry:    resp.StatusCode >= 500, // Retry on server errors
		}
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeRefreshFailed,
			Message:  "Failed to parse refresh response",
			Details:  err.Error(),
		}
	}

	// Update config with new token
	a.config.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		a.config.RefreshToken = tokenResp.RefreshToken
	}
	if tokenResp.ExpiresIn > 0 {
		a.config.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}
	// Note: TokenType field doesn't exist in types.OAuthConfig, skipping assignment

	// Store the updated token
	if err := a.storage.StoreToken(a.provider, a.config); err != nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeStorageError,
			Message:  "Failed to store refreshed token",
			Details:  err.Error(),
		}
	}

	a.isAuth = true
	a.lastAuth = time.Now()
	return nil
}

// Logout clears authentication state
func (a *OAuthAuthenticatorImpl) Logout(ctx context.Context) error {
	if err := a.storage.DeleteToken(a.provider); err != nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeStorageError,
			Message:  "Failed to delete stored token",
			Details:  err.Error(),
		}
	}

	a.config = nil
	a.isAuth = false
	a.lastAuth = time.Time{}
	a.state = ""
	a.pkceVerifier = ""
	return nil
}

// GetAuthMethod returns the authentication method type
func (a *OAuthAuthenticatorImpl) GetAuthMethod() types.AuthMethod {
	return types.AuthMethodOAuth
}

// StartOAuthFlow initiates the OAuth flow and returns the auth URL
func (a *OAuthAuthenticatorImpl) StartOAuthFlow(ctx context.Context, scopes []string) (string, error) {
	if a.config == nil {
		return "", &AuthError{
			Provider: a.provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "OAuth config not set",
		}
	}

	// Generate PKCE verifier if enabled
	var codeChallenge, codeVerifier string
	if a.oauthConfig != nil && a.oauthConfig.PKCE.Enabled {
		var err error
		codeVerifier, err = a.generatePKCEVerifier()
		if err != nil {
			return "", &AuthError{
				Provider: a.provider,
				Code:     ErrCodeOAuthFlowFailed,
				Message:  "Failed to generate PKCE verifier",
				Details:  err.Error(),
			}
		}
		codeChallenge = a.generatePKCEChallenge(codeVerifier)
		a.pkceVerifier = codeVerifier
	}

	// Use provided state if available, otherwise generate random state
	state := "" // State field doesn't exist in types.OAuthConfig, generate new one
	var err error
	state, err = a.generateRandomState()
	if err != nil {
		return "", &AuthError{
			Provider: a.provider,
			Code:     ErrCodeOAuthFlowFailed,
			Message:  "Failed to generate OAuth state",
			Details:  err.Error(),
		}
	}
	a.state = state

	// Build authorization URL
	authURL, err := url.Parse(a.config.AuthURL)
	if err != nil {
		return "", &AuthError{
			Provider: a.provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "Invalid auth URL",
			Details:  err.Error(),
		}
	}

	// Add OAuth parameters
	params := authURL.Query()
	params.Set("response_type", "code")
	params.Set("client_id", a.config.ClientID)
	params.Set("redirect_uri", a.config.RedirectURL)

	// Use provided scopes or default scopes
	finalScopes := scopes
	if len(finalScopes) == 0 && a.oauthConfig != nil {
		finalScopes = a.oauthConfig.DefaultScopes
	}
	params.Set("scope", strings.Join(finalScopes, " "))

	params.Set("state", state)

	// Add PKCE parameters if enabled
	if a.oauthConfig != nil && a.oauthConfig.PKCE.Enabled {
		params.Set("code_challenge", codeChallenge)
		params.Set("code_challenge_method", a.oauthConfig.PKCE.Method)
	}

	// Note: TokenType field doesn't exist in types.OAuthConfig, skipping token type parameter

	authURL.RawQuery = params.Encode()
	return authURL.String(), nil
}

// HandleCallback processes the OAuth callback
func (a *OAuthAuthenticatorImpl) HandleCallback(ctx context.Context, code, state string) error {
	if a.oauthConfig != nil && a.oauthConfig.State.EnableValidation && a.state != state {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeOAuthFlowFailed,
			Message:  "Invalid OAuth state",
		}
	}

	if a.config == nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeInvalidConfig,
			Message:  "OAuth config not set",
		}
	}

	// Exchange authorization code for access token
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", a.config.RedirectURL)
	data.Set("client_id", a.config.ClientID)
	data.Set("client_secret", a.config.ClientSecret)

	// Add PKCE verifier if enabled
	if a.oauthConfig != nil && a.oauthConfig.PKCE.Enabled && a.pkceVerifier != "" {
		data.Set("code_verifier", a.pkceVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeNetworkError,
			Message:  "Failed to create token request",
			Details:  err.Error(),
		}
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Set user agent if configured
	if a.oauthConfig != nil && a.oauthConfig.HTTP.UserAgent != "" {
		req.Header.Set("User-Agent", a.oauthConfig.HTTP.UserAgent)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeNetworkError,
			Message:  "Network error during token exchange",
			Details:  err.Error(),
			Retry:    true,
		}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log the error but don't fail the operation
			// In a real application, you might want to log this
			_ = fmt.Sprintf("failed to close response body during token exchange: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeOAuthFlowFailed,
			Message:  fmt.Sprintf("Token exchange failed with status %d", resp.StatusCode),
			Retry:    resp.StatusCode >= 500, // Retry on server errors
		}
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeOAuthFlowFailed,
			Message:  "Failed to parse token response",
			Details:  err.Error(),
		}
	}

	// Update config with received token
	a.config.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		a.config.RefreshToken = tokenResp.RefreshToken
	}
	if tokenResp.ExpiresIn > 0 {
		a.config.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}
	// Note: TokenType field doesn't exist in types.OAuthConfig, skipping assignment

	// Store the token
	if err := a.storage.StoreToken(a.provider, a.config); err != nil {
		return &AuthError{
			Provider: a.provider,
			Code:     ErrCodeStorageError,
			Message:  "Failed to store token",
			Details:  err.Error(),
		}
	}

	a.isAuth = true
	a.lastAuth = time.Now()

	// Clear state and PKCE verifier
	a.state = ""
	a.pkceVerifier = ""

	return nil
}

// IsOAuthEnabled checks if OAuth is properly configured
func (a *OAuthAuthenticatorImpl) IsOAuthEnabled() bool {
	return a.config != nil &&
		a.config.ClientID != "" &&
		a.config.ClientSecret != "" &&
		a.config.AuthURL != "" &&
		a.config.TokenURL != ""
}

// GetTokenInfo returns detailed token information
func (a *OAuthAuthenticatorImpl) GetTokenInfo() (*TokenInfo, error) {
	if !a.IsAuthenticated() {
		return nil, &AuthError{
			Provider: a.provider,
			Code:     ErrCodeTokenExpired,
			Message:  "Not authenticated",
		}
	}

	expiresIn := int64(0)
	if !a.config.ExpiresAt.IsZero() {
		expiresIn = int64(time.Until(a.config.ExpiresAt).Seconds())
		if expiresIn < 0 {
			expiresIn = 0
		}
	}

	return &TokenInfo{
		AccessToken:  a.config.AccessToken,
		RefreshToken: a.config.RefreshToken,
		TokenType:    "Bearer", // Default token type since field doesn't exist in types.OAuthConfig
		ExpiresAt:    a.config.ExpiresAt,
		Scopes:       a.config.Scopes,
		IsExpired:    a.isTokenExpired(a.config),
		ExpiresIn:    expiresIn,
	}, nil
}

// Helper methods

func (a *OAuthAuthenticatorImpl) isTokenExpired(config *types.OAuthConfig) bool {
	if config.ExpiresAt.IsZero() {
		return false // No expiration set, assume not expired
	}

	// Apply refresh buffer if configured
	buffer := 5 * time.Minute // Default 5 minute buffer
	if a.oauthConfig != nil && a.oauthConfig.Refresh.Enabled {
		buffer = a.oauthConfig.Refresh.Buffer
	}

	return time.Now().After(config.ExpiresAt.Add(-buffer))
}

func (a *OAuthAuthenticatorImpl) generateRandomState() (string, error) {
	length := 32
	if a.oauthConfig != nil && a.oauthConfig.State.Length > 0 {
		length = a.oauthConfig.State.Length
	}

	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (a *OAuthAuthenticatorImpl) generatePKCEVerifier() (string, error) {
	length := 128
	if a.oauthConfig != nil && a.oauthConfig.PKCE.VerifierLength > 0 {
		length = a.oauthConfig.PKCE.VerifierLength
	}

	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (a *OAuthAuthenticatorImpl) generatePKCEChallenge(verifier string) string {
	method := "S256"
	if a.oauthConfig != nil && a.oauthConfig.PKCE.Method != "" {
		method = a.oauthConfig.PKCE.Method
	}

	switch method {
	case "plain":
		return verifier
	case "S256":
		hash := sha256.Sum256([]byte(verifier))
		return base64.RawURLEncoding.EncodeToString(hash[:])
	default:
		return verifier
	}
}

// OAuthTokenResponse represents the token response from OAuth servers
type OAuthTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// OAuthHelper provides utility functions for OAuth flows
type OAuthHelper struct {
	config *OAuthConfig
}

// NewOAuthHelper creates a new OAuth helper
func NewOAuthHelper(config *OAuthConfig) *OAuthHelper {
	return &OAuthHelper{config: config}
}

// ValidateCallback validates an OAuth callback response
func (h *OAuthHelper) ValidateCallback(state, expectedState, code, errorParam string) error {
	// Check for OAuth error
	if errorParam != "" {
		return &AuthError{
			Code:    ErrCodeOAuthFlowFailed,
			Message: "OAuth error in callback",
			Details: errorParam,
		}
	}

	// Check state if validation is enabled
	if h.config != nil && h.config.State.EnableValidation && state != expectedState {
		return &AuthError{
			Code:    ErrCodeOAuthFlowFailed,
			Message: "Invalid OAuth state",
		}
	}

	// Check authorization code
	if code == "" {
		return &AuthError{
			Code:    ErrCodeOAuthFlowFailed,
			Message: "No authorization code received",
		}
	}

	return nil
}

// BuildAuthURL builds an OAuth authorization URL
func (h *OAuthHelper) BuildAuthURL(authURL, clientID, redirectURI, state string, scopes []string, pkceChallenge string) (string, error) {
	u, err := url.Parse(authURL)
	if err != nil {
		return "", fmt.Errorf("invalid auth URL: %w", err)
	}

	params := u.Query()
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", strings.Join(scopes, " "))
	params.Set("state", state)

	if pkceChallenge != "" {
		params.Set("code_challenge", pkceChallenge)
		params.Set("code_challenge_method", "S256")
	}

	u.RawQuery = params.Encode()
	return u.String(), nil
}

// ExchangeCodeForToken exchanges an authorization code for an access token
func (h *OAuthHelper) ExchangeCodeForToken(ctx context.Context, tokenURL, clientID, clientSecret, redirectURI, code, codeVerifier string) (*OAuthTokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	if h.config != nil && h.config.HTTP.UserAgent != "" {
		req.Header.Set("User-Agent", h.config.HTTP.UserAgent)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error during token exchange: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log the error but don't fail the operation
			// In a real application, you might want to log this
			_ = fmt.Sprintf("failed to close response body during helper token exchange: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	var tokenResp OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("OAuth error: %s - %s", tokenResp.Error, tokenResp.ErrorDescription)
	}

	return &tokenResp, nil
}
