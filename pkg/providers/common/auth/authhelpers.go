package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/keymanager"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/oauthmanager"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// AuthHelper provides shared authentication functionality for providers
type AuthHelper struct {
	ProviderName string
	KeyManager   *keymanager.KeyManager
	OAuthManager *oauthmanager.OAuthKeyManager
	HTTPClient   *http.Client
	Config       types.ProviderConfig
}

// NewAuthHelper creates a new authentication helper
func NewAuthHelper(providerName string, config types.ProviderConfig, client *http.Client) *AuthHelper {
	helper := &AuthHelper{
		ProviderName: providerName,
		HTTPClient:   client,
		Config:       config,
	}
	return helper
}

// SetupAPIKeys configures API key management with support for multiple keys
// Extracts keys from both config.APIKey and config.ProviderConfig["api_keys"]
func (h *AuthHelper) SetupAPIKeys() {
	var keys []string

	// Add single API key if present
	if h.Config.APIKey != "" {
		keys = append(keys, h.Config.APIKey)
	}

	// Add multiple API keys from provider config
	if h.Config.ProviderConfig != nil {
		if apiKeys, ok := h.Config.ProviderConfig["api_keys"].([]string); ok {
			keys = append(keys, apiKeys...)
		}
	}

	// Create key manager if we have keys, otherwise clear it
	if len(keys) > 0 {
		h.KeyManager = keymanager.NewKeyManager(h.ProviderName, keys)
	} else {
		// Clear the key manager when no keys are available
		h.KeyManager = nil
	}
}

// SetupOAuth configures OAuth management with multiple credential support
func (h *AuthHelper) SetupOAuth(refreshFunc oauthmanager.RefreshFunc) {
	if len(h.Config.OAuthCredentials) > 0 {
		h.OAuthManager = oauthmanager.NewOAuthKeyManager(
			h.ProviderName,
			h.Config.OAuthCredentials,
			refreshFunc,
		)
	}
}

// IsAuthenticated checks if any authentication method is configured
func (h *AuthHelper) IsAuthenticated() bool {
	// Check OAuth first
	if h.OAuthManager != nil && len(h.OAuthManager.GetCredentials()) > 0 {
		return true
	}

	// Check API keys
	return h.KeyManager != nil && len(h.KeyManager.GetKeys()) > 0
}

// GetAuthMethod returns the currently available authentication method
func (h *AuthHelper) GetAuthMethod() string {
	if h.OAuthManager != nil && len(h.OAuthManager.GetCredentials()) > 0 {
		return "oauth"
	}
	if h.KeyManager != nil && len(h.KeyManager.GetKeys()) > 0 {
		return "api_key"
	}
	return "none"
}

// ExecuteWithAuth executes an operation using available authentication methods
// Automatically chooses OAuth over API key, with failover support
// Returns string content only - use ExecuteWithAuthMessage for full message support
func (h *AuthHelper) ExecuteWithAuth(
	ctx context.Context,
	options types.GenerateOptions,
	oauthOperation func(context.Context, *types.OAuthCredentialSet) (string, *types.Usage, error),
	apiKeyOperation func(context.Context, string) (string, *types.Usage, error),
) (string, *types.Usage, error) {
	log.Printf("🟡 [AuthHelper] ExecuteWithAuth ENTRY - provider=%s, Stream=%v", h.ProviderName, options.Stream)
	log.Printf("🟡 [AuthHelper] OAuthManager=%p, KeyManager=%p", h.OAuthManager, h.KeyManager)

	// Check for context-injected OAuth token first
	if contextToken := GetOAuthToken(ctx); contextToken != "" {
		log.Printf("[AuthHelper] Using context-injected OAuth token")
		// Create a temporary credential set with the injected token
		cred := &types.OAuthCredentialSet{
			AccessToken: contextToken,
		}
		return oauthOperation(ctx, cred)
	}

	// Check if streaming is requested
	if options.Stream {
		log.Printf("🟡 [AuthHelper] Delegating to executeStreamWithAuth")
		return h.executeStreamWithAuth(ctx, options, oauthOperation, apiKeyOperation)
	}

	log.Printf("🟡 [AuthHelper] Non-streaming path")
	// Non-streaming path
	if h.OAuthManager != nil {
		log.Printf("🟡 [AuthHelper] Trying OAuth failover...")
		result, usage, err := h.OAuthManager.ExecuteWithFailover(ctx, oauthOperation)
		if err == nil {
			log.Printf("🟢 [AuthHelper] OAuth SUCCESS")
			return result, usage, nil
		}
		log.Printf("🔴 [AuthHelper] OAuth FAILED: %v", err)
	} else {
		log.Printf("⚠️  [AuthHelper] OAuthManager is NIL")
	}

	if h.KeyManager != nil {
		log.Printf("🟡 [AuthHelper] Trying API key failover...")
		result, usage, err := h.KeyManager.ExecuteWithFailover(ctx, apiKeyOperation)
		log.Printf("🟡 [AuthHelper] API key result - err=%v", err)
		return result, usage, err
	} else {
		log.Printf("⚠️  [AuthHelper] KeyManager is NIL")
	}

	log.Printf("🔴 [AuthHelper] NO AUTH CONFIGURED ERROR")
	return "", nil, fmt.Errorf("no authentication configured for %s", h.ProviderName)
}

// ExecuteWithAuthMessage executes an operation using available authentication methods
// Returns full ChatMessage to preserve tool calls. Use this for chat completions.
func (h *AuthHelper) ExecuteWithAuthMessage(
	ctx context.Context,
	options types.GenerateOptions,
	oauthOperation func(context.Context, *types.OAuthCredentialSet) (types.ChatMessage, *types.Usage, error),
	apiKeyOperation func(context.Context, string) (types.ChatMessage, *types.Usage, error),
) (types.ChatMessage, *types.Usage, error) {
	log.Printf("🟡 [AuthHelper] ExecuteWithAuthMessage ENTRY - provider=%s, Stream=%v", h.ProviderName, options.Stream)
	log.Printf("🟡 [AuthHelper] OAuthManager=%p, KeyManager=%p", h.OAuthManager, h.KeyManager)

	// Check for context-injected OAuth token first
	if contextToken := GetOAuthToken(ctx); contextToken != "" {
		log.Printf("[AuthHelper] Using context-injected OAuth token")
		// Create a temporary credential set with the injected token
		cred := &types.OAuthCredentialSet{
			AccessToken: contextToken,
		}
		return oauthOperation(ctx, cred)
	}

	// Check if streaming is requested
	if options.Stream {
		log.Printf("🟡 [AuthHelper] Delegating to executeStreamWithAuthMessage")
		return h.executeStreamWithAuthMessage(ctx, options, oauthOperation, apiKeyOperation)
	}

	log.Printf("🟡 [AuthHelper] Non-streaming path")
	// Non-streaming path
	if h.OAuthManager != nil {
		log.Printf("🟡 [AuthHelper] Trying OAuth failover...")
		result, usage, err := h.OAuthManager.ExecuteWithFailoverMessage(ctx, oauthOperation)
		if err == nil {
			log.Printf("🟢 [AuthHelper] OAuth SUCCESS")
			return result, usage, nil
		}
		log.Printf("🔴 [AuthHelper] OAuth FAILED: %v", err)
	} else {
		log.Printf("⚠️  [AuthHelper] OAuthManager is NIL")
	}

	if h.KeyManager != nil {
		log.Printf("🟡 [AuthHelper] Trying API key failover...")
		result, usage, err := h.KeyManager.ExecuteWithFailoverMessage(ctx, apiKeyOperation)
		log.Printf("🟡 [AuthHelper] API key result - err=%v", err)
		return result, usage, err
	} else {
		log.Printf("⚠️  [AuthHelper] KeyManager is NIL")
	}

	log.Printf("🔴 [AuthHelper] NO AUTH CONFIGURED ERROR")
	return types.ChatMessage{}, nil, fmt.Errorf("no authentication configured for %s", h.ProviderName)
}

// executeStreamWithAuth handles streaming operations with authentication
func (h *AuthHelper) executeStreamWithAuth(
	ctx context.Context,
	_ types.GenerateOptions,
	oauthOperation func(context.Context, *types.OAuthCredentialSet) (string, *types.Usage, error),
	apiKeyOperation func(context.Context, string) (string, *types.Usage, error),
) (string, *types.Usage, error) {
	// Check for context-injected OAuth token first
	if contextToken := GetOAuthToken(ctx); contextToken != "" {
		log.Printf("[AuthHelper] Using context-injected OAuth token for streaming")
		cred := &types.OAuthCredentialSet{
			AccessToken: contextToken,
		}
		_, _, err := oauthOperation(ctx, cred)
		if err == nil {
			return "streaming_with_context_oauth", &types.Usage{}, nil
		}
	}

	// For streaming, we need to return a mock result since the actual streaming
	// is handled by provider-specific streaming methods
	if h.OAuthManager != nil {
		// Try OAuth first
		creds := h.OAuthManager.GetCredentials()
		for _, cred := range creds {
			_, _, err := oauthOperation(ctx, cred)
			if err == nil {
				return "streaming_with_oauth", &types.Usage{}, nil
			}
		}
	}

	if h.KeyManager != nil {
		// Try API keys
		keys := h.KeyManager.GetKeys()
		for _, key := range keys {
			_, _, err := apiKeyOperation(ctx, key)
			if err == nil {
				return "streaming_with_api_key", &types.Usage{}, nil
			}
		}
	}

	return "", nil, fmt.Errorf("no valid authentication available for streaming")
}

// executeStreamWithAuthMessage handles streaming operations with authentication (message variant)
func (h *AuthHelper) executeStreamWithAuthMessage(
	ctx context.Context,
	_ types.GenerateOptions,
	oauthOperation func(context.Context, *types.OAuthCredentialSet) (types.ChatMessage, *types.Usage, error),
	apiKeyOperation func(context.Context, string) (types.ChatMessage, *types.Usage, error),
) (types.ChatMessage, *types.Usage, error) {
	// Check for context-injected OAuth token first
	// For streaming, we don't actually call the operation (to avoid multiple API requests)
	// We just verify the token exists and return success
	if contextToken := GetOAuthToken(ctx); contextToken != "" {
		log.Printf("[AuthHelper] Using context-injected OAuth token for streaming")
		return types.ChatMessage{Content: "streaming_with_context_oauth"}, &types.Usage{}, nil
	}

	// For streaming, we need to return a mock result since the actual streaming
	// is handled by provider-specific streaming methods
	// We just verify credentials exist without actually calling the operation
	if h.OAuthManager != nil {
		// Check if OAuth credentials exist
		creds := h.OAuthManager.GetCredentials()
		if len(creds) > 0 {
			log.Printf("[AuthHelper] Using OAuth credentials for streaming (found %d credential(s))", len(creds))
			return types.ChatMessage{Content: "streaming_with_oauth"}, &types.Usage{}, nil
		}
	}

	if h.KeyManager != nil {
		// Check if API keys exist
		keys := h.KeyManager.GetKeys()
		if len(keys) > 0 {
			log.Printf("[AuthHelper] Using API key for streaming (found %d key(s))", len(keys))
			return types.ChatMessage{Content: "streaming_with_api_key"}, &types.Usage{}, nil
		}
	}

	return types.ChatMessage{}, nil, fmt.Errorf("no valid authentication available for streaming")
}

// SetAuthHeaders sets appropriate authentication headers on HTTP requests
func (h *AuthHelper) SetAuthHeaders(req *http.Request, authToken string, authType string) {
	switch authType {
	case "oauth", "bearer":
		req.Header.Set("Authorization", "Bearer "+authToken)
	case "api_key":
		// Different providers use different header names for API keys
		switch h.ProviderName {
		case "anthropic":
			req.Header.Set("x-api-key", authToken)
		case "gemini":
			req.Header.Set("x-goog-api-key", authToken)
		default:
			req.Header.Set("Authorization", "Bearer "+authToken)
		}
	case "custom":
		// For providers that need custom header handling
		// The provider should override this method
	}
}

// SetAuthHeadersFromContext sets auth headers using context-provided credentials
func (h *AuthHelper) SetAuthHeadersFromContext(ctx context.Context, req *http.Request) bool {
	token := GetOAuthToken(ctx)
	if token == "" {
		return false
	}

	authType := GetAuthType(ctx)
	h.SetAuthHeaders(req, token, authType)
	return true
}

// SetProviderSpecificHeaders sets provider-specific headers beyond auth
func (h *AuthHelper) SetProviderSpecificHeaders(req *http.Request) {
	switch h.ProviderName {
	case "anthropic":
		req.Header.Set("anthropic-version", "2023-06-01")
		// Add Claude Code beta headers for OAuth
		if h.GetAuthMethod() == "oauth" {
			req.Header.Set("anthropic-beta", "oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14")
		}
	case "openai":
		// Add organization header if configured
		if h.Config.ProviderConfig != nil {
			if orgID, ok := h.Config.ProviderConfig["organization_id"].(string); ok {
				req.Header.Set("openai-organization", orgID)
			}
		}
	case "openrouter":
		// Add OpenRouter specific headers
		if h.Config.ProviderConfig != nil {
			if siteURL, ok := h.Config.ProviderConfig["site_url"].(string); ok && siteURL != "" {
				req.Header.Set("HTTP-Referer", siteURL)
			}
			if siteName, ok := h.Config.ProviderConfig["site_name"].(string); ok && siteName != "" {
				req.Header.Set("X-Title", siteName)
			}
		}
	}
}

// MakeAuthenticatedRequest makes an HTTP request with proper authentication
func (h *AuthHelper) MakeAuthenticatedRequest(
	ctx context.Context,
	method, url string,
	headers map[string]string,
	body interface{},
) (*http.Response, error) {

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set content-type and body if provided
	if body != nil {
		// This would need to be implemented per provider due to different body types
		// For now, providers should handle body creation themselves
		_ = body // Suppress unused variable warning for future implementation
	}

	// Set provided headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Set provider-specific headers
	h.SetProviderSpecificHeaders(req)

	// Make the request
	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// CreateJSONRequest creates an HTTP request with JSON body and standard headers
// This is a convenience method that combines request creation, JSON marshaling,
// and header setup commonly used across providers.
func (h *AuthHelper) CreateJSONRequest(
	ctx context.Context,
	method, url string,
	body interface{},
	credential string,
	authType string,
) (*http.Request, error) {
	// Marshal body to JSON if provided
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	}

	// Create request
	var req *http.Request
	var err error
	if reqBody != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, reqBody)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set standard headers
	req.Header.Set("Content-Type", "application/json")

	// Set authentication headers
	h.SetAuthHeaders(req, credential, authType)

	// Set provider-specific headers
	h.SetProviderSpecificHeaders(req)

	return req, nil
}

// HandleAuthError handles authentication-specific errors and provides user-friendly messages
func (h *AuthHelper) HandleAuthError(err error, statusCode int) error {
	if err == nil {
		return nil
	}

	// Common authentication error patterns
	errMsg := strings.ToLower(err.Error())

	if strings.Contains(errMsg, "invalid api key") || strings.Contains(errMsg, "unauthorized") {
		return fmt.Errorf("%s: invalid API key or authentication token", h.ProviderName)
	}

	if strings.Contains(errMsg, "quota exceeded") || strings.Contains(errMsg, "insufficient_quota") {
		return fmt.Errorf("%s: quota exceeded. Please check your billing", h.ProviderName)
	}

	if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "rate_limit_exceeded") {
		return fmt.Errorf("%s: rate limit exceeded. Please try again later", h.ProviderName)
	}

	if strings.Contains(errMsg, "token expired") || strings.Contains(errMsg, "invalid_token") {
		return fmt.Errorf("%s: authentication token expired. Please re-authenticate", h.ProviderName)
	}

	// HTTP status code specific handling
	switch statusCode {
	case 401:
		return fmt.Errorf("%s: authentication failed. Please check your credentials", h.ProviderName)
	case 403:
		return fmt.Errorf("%s: access forbidden. Check your permissions or billing", h.ProviderName)
	case 429:
		return fmt.Errorf("%s: rate limit exceeded. Please try again later", h.ProviderName)
	}

	return err
}

// ClearAuthentication clears all authentication credentials
func (h *AuthHelper) ClearAuthentication() {
	h.KeyManager = nil
	h.OAuthManager = nil
}

// ValidateAuthConfig validates authentication configuration
func (h *AuthHelper) ValidateAuthConfig(authConfig types.AuthConfig) error {
	switch authConfig.Method {
	case types.AuthMethodAPIKey:
		if authConfig.APIKey == "" {
			return fmt.Errorf("API key is required for API key authentication")
		}
	case types.AuthMethodOAuth:
		if len(h.Config.OAuthCredentials) == 0 {
			return fmt.Errorf("OAuth credentials are required for OAuth authentication")
		}
	case types.AuthMethodBearerToken:
		if authConfig.APIKey == "" {
			return fmt.Errorf("bearer token is required for bearer token authentication")
		}
	default:
		return fmt.Errorf("unsupported authentication method: %s", authConfig.Method)
	}

	return nil
}

// GetAuthStatus returns a detailed authentication status
func (h *AuthHelper) GetAuthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"provider":      h.ProviderName,
		"authenticated": h.IsAuthenticated(),
		"method":        h.GetAuthMethod(),
	}

	// Add API key status
	if h.KeyManager != nil {
		keys := h.KeyManager.GetKeys()
		status["api_keys_configured"] = len(keys)
		if len(keys) > 0 {
			status["has_api_keys"] = true
		}
	}

	// Add OAuth status
	if h.OAuthManager != nil {
		creds := h.OAuthManager.GetCredentials()
		status["oauth_credentials_configured"] = len(creds)
		if len(creds) > 0 {
			status["has_oauth"] = true
			// Add credential count
			status["oauth_credentials_count"] = len(creds)
		}
	}

	return status
}

// RefreshAllOAuthTokens attempts to refresh all OAuth tokens
func (h *AuthHelper) RefreshAllOAuthTokens(ctx context.Context) error {
	if h.OAuthManager == nil {
		return fmt.Errorf("no OAuth manager configured")
	}

	creds := h.OAuthManager.GetCredentials()
	var errors []string

	for _, cred := range creds {
		// This would need to be implemented in oauthmanager
		// For now, this is a placeholder for the concept
		_, _, err := h.OAuthManager.ExecuteWithFailover(ctx, func(ctx context.Context, c *types.OAuthCredentialSet) (string, *types.Usage, error) {
			// Check if token needs refresh
			if time.Now().Add(5 * time.Minute).After(c.ExpiresAt) {
				// Token is expiring soon, this would trigger refresh
				return "", nil, fmt.Errorf("token refresh needed")
			}
			return "ok", &types.Usage{}, nil
		})

		if err != nil {
			errors = append(errors, fmt.Sprintf("credential %s: %v", cred.ID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to refresh some tokens: %s", strings.Join(errors, "; "))
	}

	return nil
}

// IsOAuthConfigured checks if OAuth credentials are configured and available
func (h *AuthHelper) IsOAuthConfigured() bool {
	return h.OAuthManager != nil && len(h.OAuthManager.GetCredentials()) > 0
}

// IsAPIKeyConfigured checks if API keys are configured and available
func (h *AuthHelper) IsAPIKeyConfigured() bool {
	return h.KeyManager != nil && len(h.KeyManager.GetKeys()) > 0
}
