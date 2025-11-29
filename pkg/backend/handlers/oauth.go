package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/auth"
)

// OAuthHandler handles OAuth-related endpoints
type OAuthHandler struct {
	authManager auth.AuthManager
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(authManager auth.AuthManager) *OAuthHandler {
	return &OAuthHandler{
		authManager: authManager,
	}
}

// InitiateOAuth initiates OAuth flow for a provider
// GET /api/oauth/{provider}/initiate
func (h *OAuthHandler) InitiateOAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := h.extractProviderFromPath(r)
	if provider == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	// Parse optional scopes from query parameters
	scopesParam := r.URL.Query().Get("scopes")
	var scopes []string
	if scopesParam != "" {
		scopes = strings.Split(scopesParam, ",")
		// Trim whitespace from each scope
		for i, scope := range scopes {
			scopes[i] = strings.TrimSpace(scope)
		}
	}

	// Start OAuth flow
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	authURL, err := h.authManager.StartOAuthFlow(ctx, provider, scopes)
	if err != nil {
		SendError(w, r, "OAUTH_INITIATE_FAILED", fmt.Sprintf("Failed to initiate OAuth flow: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if client wants JSON response or redirect
	if r.URL.Query().Get("response") == "json" {
		SendSuccess(w, r, map[string]interface{}{
			"provider":      provider,
			"authorization_url": authURL,
			"message":       "Navigate to the authorization URL to complete OAuth flow",
		})
		return
	}

	// Default: redirect to provider's authorization page
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// OAuthCallback handles OAuth callback from provider
// GET /api/oauth/{provider}/callback
func (h *OAuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := h.extractProviderFromPath(r)
	if provider == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	// Extract authorization code and state from query parameters
	code := r.URL.Query().Get("code")
	if code == "" {
		SendError(w, r, "MISSING_PARAMETER", "Authorization code is required", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")

	// Handle OAuth callback
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	err := h.authManager.HandleOAuthCallback(ctx, provider, code, state)
	if err != nil {
		SendError(w, r, "OAUTH_CALLBACK_FAILED", fmt.Sprintf("Failed to handle OAuth callback: %v", err), http.StatusInternalServerError)
		return
	}

	// Get token info to return in response
	tokenInfo, err := h.authManager.GetTokenInfo(provider)
	if err != nil {
		// Still return success since callback was handled, but without token details
		SendSuccess(w, r, map[string]interface{}{
			"provider": provider,
			"status":   "authenticated",
			"message":  "OAuth flow completed successfully",
		})
		return
	}

	SendSuccess(w, r, map[string]interface{}{
		"provider":    provider,
		"status":      "authenticated",
		"message":     "OAuth flow completed successfully",
		"token_info": map[string]interface{}{
			"has_access_token":  tokenInfo.AccessToken != "",
			"has_refresh_token": tokenInfo.RefreshToken != "",
			"expires_at":        tokenInfo.ExpiresAt,
			"is_expired":        tokenInfo.IsExpired,
		},
	})
}

// RefreshToken refreshes an expired token for a provider
// POST /api/oauth/{provider}/refresh
func (h *OAuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := h.extractProviderFromPath(r)
	if provider == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	// Get the authenticator for this provider
	authenticator, err := h.authManager.GetAuthenticator(provider)
	if err != nil {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", provider), http.StatusNotFound)
		return
	}

	// Check if it's an OAuth authenticator
	oauthAuth, ok := authenticator.(auth.OAuthAuthenticator)
	if !ok {
		SendError(w, r, "NOT_OAUTH_PROVIDER", fmt.Sprintf("Provider '%s' does not support OAuth", provider), http.StatusBadRequest)
		return
	}

	// Refresh the token
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	err = oauthAuth.RefreshToken(ctx)
	if err != nil {
		SendError(w, r, "REFRESH_FAILED", fmt.Sprintf("Failed to refresh token: %v", err), http.StatusInternalServerError)
		return
	}

	// Get updated token info
	tokenInfo, err := h.authManager.GetTokenInfo(provider)
	if err != nil {
		SendSuccess(w, r, map[string]interface{}{
			"provider": provider,
			"status":   "refreshed",
			"message":  "Token refreshed successfully",
		})
		return
	}

	SendSuccess(w, r, map[string]interface{}{
		"provider": provider,
		"status":   "refreshed",
		"message":  "Token refreshed successfully",
		"token_info": map[string]interface{}{
			"has_access_token":  tokenInfo.AccessToken != "",
			"has_refresh_token": tokenInfo.RefreshToken != "",
			"expires_at":        tokenInfo.ExpiresAt,
			"is_expired":        tokenInfo.IsExpired,
		},
	})
}

// GetTokenStatus checks if token exists and is valid for a provider
// GET /api/oauth/{provider}/status
func (h *OAuthHandler) GetTokenStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := h.extractProviderFromPath(r)
	if provider == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	// Get the authenticator for this provider
	authenticator, err := h.authManager.GetAuthenticator(provider)
	if err != nil {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", provider), http.StatusNotFound)
		return
	}

	// Check if it's an OAuth authenticator
	oauthAuth, ok := authenticator.(auth.OAuthAuthenticator)
	if !ok {
		SendError(w, r, "NOT_OAUTH_PROVIDER", fmt.Sprintf("Provider '%s' does not support OAuth", provider), http.StatusBadRequest)
		return
	}

	// Check authentication status
	isAuthenticated := oauthAuth.IsAuthenticated()

	// Get token info
	tokenInfo, err := h.authManager.GetTokenInfo(provider)

	// Build response
	response := map[string]interface{}{
		"provider":        provider,
		"authenticated":   isAuthenticated,
	}

	if err == nil && tokenInfo != nil {
		response["token_exists"] = tokenInfo.AccessToken != ""
		response["has_refresh_token"] = tokenInfo.RefreshToken != ""
		response["expires_at"] = tokenInfo.ExpiresAt
		response["is_expired"] = tokenInfo.IsExpired

		// Calculate time until expiry if valid
		if !tokenInfo.ExpiresAt.IsZero() {
			timeUntilExpiry := time.Until(tokenInfo.ExpiresAt)
			response["expires_in_seconds"] = int64(timeUntilExpiry.Seconds())
		}
	} else {
		response["token_exists"] = false
		response["has_refresh_token"] = false
		response["is_expired"] = true
	}

	SendSuccess(w, r, response)
}

// RevokeToken revokes/deletes the OAuth token for a provider
// DELETE /api/oauth/{provider}/token
func (h *OAuthHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only DELETE or POST methods are allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := h.extractProviderFromPath(r)
	if provider == "" {
		SendError(w, r, "MISSING_PARAMETER", "Provider name is required", http.StatusBadRequest)
		return
	}

	// Get the authenticator for this provider
	authenticator, err := h.authManager.GetAuthenticator(provider)
	if err != nil {
		SendError(w, r, "PROVIDER_NOT_FOUND", fmt.Sprintf("Provider '%s' not found", provider), http.StatusNotFound)
		return
	}

	// Logout to clear the token
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err = authenticator.Logout(ctx)
	if err != nil {
		SendError(w, r, "REVOKE_FAILED", fmt.Sprintf("Failed to revoke token: %v", err), http.StatusInternalServerError)
		return
	}

	SendSuccess(w, r, map[string]interface{}{
		"provider": provider,
		"status":   "revoked",
		"message":  "OAuth token revoked successfully",
	})
}

// ListOAuthProviders lists all providers that support OAuth
// GET /api/oauth/providers
func (h *OAuthHandler) ListOAuthProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendError(w, r, "METHOD_NOT_ALLOWED", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all authenticated providers, then check which ones support OAuth
	providers := h.authManager.GetAuthenticatedProviders()
	oauthProviders := []map[string]interface{}{}

	for _, providerName := range providers {
		authenticator, err := h.authManager.GetAuthenticator(providerName)
		if err != nil {
			continue
		}

		// Check if it's an OAuth authenticator
		if _, ok := authenticator.(auth.OAuthAuthenticator); ok {
			providerInfo := map[string]interface{}{
				"name":          providerName,
				"authenticated": authenticator.IsAuthenticated(),
			}

			// Try to get token info
			if tokenInfo, err := h.authManager.GetTokenInfo(providerName); err == nil {
				providerInfo["has_token"] = tokenInfo.AccessToken != ""
				providerInfo["is_expired"] = tokenInfo.IsExpired
				providerInfo["expires_at"] = tokenInfo.ExpiresAt
			}

			oauthProviders = append(oauthProviders, providerInfo)
		}
	}

	SendSuccess(w, r, map[string]interface{}{
		"providers": oauthProviders,
		"count":     len(oauthProviders),
	})
}

// Helper functions

// extractProviderFromPath extracts provider name from URL path
// Supports patterns like /api/oauth/{provider}/initiate, /api/oauth/{provider}/callback, etc.
func (h *OAuthHandler) extractProviderFromPath(r *http.Request) string {
	// Remove /api/oauth/ prefix
	path := strings.TrimPrefix(r.URL.Path, "/api/oauth/")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return ""
	}

	// Extract first segment as provider name
	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}

	return ""
}

// OAuthRequest is used for parsing OAuth-related request bodies
type OAuthRequest struct {
	Provider string   `json:"provider"`
	Scopes   []string `json:"scopes,omitempty"`
	Code     string   `json:"code,omitempty"`
	State    string   `json:"state,omitempty"`
}

// parseOAuthRequest is a helper to parse OAuth request bodies
func parseOAuthRequest(r *http.Request) (*OAuthRequest, error) {
	var req OAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}
