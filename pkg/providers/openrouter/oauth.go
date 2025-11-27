// Package openrouter provides an OpenRouter AI provider implementation.
// It includes support for model selection, API key management, OAuth authentication,
// and specialized features for the OpenRouter platform.
package openrouter

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OAuthHelper provides OAuth flow utilities for OpenRouter
// OpenRouter's OAuth is unique: it provisions permanent API keys rather than temporary tokens
type OAuthHelper struct {
	baseURL  string
	authURL  string
	tokenURL string
	client   *http.Client
}

// NewOAuthHelper creates a new OAuth helper for OpenRouter
func NewOAuthHelper(baseURL string) *OAuthHelper {
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}

	return &OAuthHelper{
		baseURL:  baseURL,
		authURL:  baseURL + "/auth",
		tokenURL: baseURL + "/api/v1/auth/keys",
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// PKCEParams holds PKCE flow parameters
type PKCEParams struct {
	CodeVerifier  string
	CodeChallenge string
	Method        string // Always "S256" for OpenRouter
}

// GeneratePKCEParams generates PKCE parameters for OAuth flow
// OpenRouter requires S256 code challenge method
func (h *OAuthHelper) GeneratePKCEParams() (*PKCEParams, error) {
	// Generate 32 bytes of random data for code verifier
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	// Base64 URL encode the verifier
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate code challenge using S256 method
	codeChallenge := h.generateCodeChallenge(codeVerifier)

	return &PKCEParams{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		Method:        "S256",
	}, nil
}

// generateCodeChallenge creates SHA256 hash of the verifier
func (h *OAuthHelper) generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// BuildAuthURL constructs the OAuth authorization URL for OpenRouter
// OpenRouter's OAuth flow is simpler: no client_id/client_secret needed
func (h *OAuthHelper) BuildAuthURL(callbackURL string, pkceParams *PKCEParams) (string, error) {
	if callbackURL == "" {
		return "", fmt.Errorf("callback URL is required")
	}
	if pkceParams == nil {
		return "", fmt.Errorf("PKCE parameters are required")
	}

	// OpenRouter uses callback_url parameter instead of redirect_uri
	// Format: https://openrouter.ai/auth?callback_url={url}&code_challenge={challenge}&code_challenge_method=S256
	authURL := fmt.Sprintf("%s?callback_url=%s&code_challenge=%s&code_challenge_method=%s",
		h.authURL,
		callbackURL,
		pkceParams.CodeChallenge,
		pkceParams.Method,
	)

	return authURL, nil
}

// TokenExchangeRequest represents the request to exchange authorization code for API key
type TokenExchangeRequest struct {
	Code                string `json:"code"`
	CodeVerifier        string `json:"code_verifier"`
	CodeChallengeMethod string `json:"code_challenge_method"`
}

// TokenExchangeResponse represents the response from token exchange
// OpenRouter returns a permanent API key, not a temporary OAuth token
type TokenExchangeResponse struct {
	Key   string `json:"key"` // The API key (format: sk-or-v1-xxxxx)
	Error string `json:"error,omitempty"`
}

// ExchangeCodeForAPIKey exchanges the authorization code for an OpenRouter API key
// This is the final step of the OAuth flow
// Returns the permanent API key that can be added to the APIKeyManager pool
func (h *OAuthHelper) ExchangeCodeForAPIKey(ctx context.Context, authCode string, codeVerifier string) (string, error) {
	if authCode == "" {
		return "", fmt.Errorf("authorization code is required")
	}
	if codeVerifier == "" {
		return "", fmt.Errorf("code verifier is required")
	}

	// Prepare the request payload
	reqBody := TokenExchangeRequest{
		Code:                authCode,
		CodeVerifier:        codeVerifier,
		CodeChallengeMethod: "S256",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", h.tokenURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute the request
	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for non-200 status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var tokenResp TokenExchangeResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	// Check for error in response
	if tokenResp.Error != "" {
		return "", fmt.Errorf("token exchange error: %s", tokenResp.Error)
	}

	// Validate the API key format
	if tokenResp.Key == "" {
		return "", fmt.Errorf("no API key returned in response")
	}

	// OpenRouter API keys typically start with "sk-or-v1-"
	if !strings.HasPrefix(tokenResp.Key, "sk-or-") {
		return "", fmt.Errorf("unexpected API key format: %s", tokenResp.Key)
	}

	return tokenResp.Key, nil
}

// ValidateCallback validates the OAuth callback parameters
func (h *OAuthHelper) ValidateCallback(code, errorParam string) error {
	// Check for OAuth error
	if errorParam != "" {
		return fmt.Errorf("OAuth error: %s", errorParam)
	}

	// Check for authorization code
	if code == "" {
		return fmt.Errorf("no authorization code received")
	}

	return nil
}

// OAuthFlowState tracks the state of an OAuth flow in progress
// This should be stored temporarily during the OAuth flow
type OAuthFlowState struct {
	CodeVerifier string    `json:"code_verifier"`
	CallbackURL  string    `json:"callback_url"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// NewOAuthFlowState creates a new OAuth flow state
func NewOAuthFlowState(codeVerifier, callbackURL string) *OAuthFlowState {
	now := time.Now()
	return &OAuthFlowState{
		CodeVerifier: codeVerifier,
		CallbackURL:  callbackURL,
		CreatedAt:    now,
		ExpiresAt:    now.Add(10 * time.Minute), // OAuth flow expires after 10 minutes
	}
}

// IsExpired checks if the OAuth flow state has expired
func (s *OAuthFlowState) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// Validate validates the OAuth flow state
func (s *OAuthFlowState) Validate() error {
	if s.CodeVerifier == "" {
		return fmt.Errorf("code verifier is missing")
	}
	if s.CallbackURL == "" {
		return fmt.Errorf("callback URL is missing")
	}
	if s.IsExpired() {
		return fmt.Errorf("OAuth flow has expired")
	}
	return nil
}
