// Package gemini provides configuration and initialization for Google Gemini AI provider.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/internal/clientpool"
	pkghttp "github.com/cecil-the-coder/ai-provider-kit/internal/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/base"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
	commonconfig "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/telemetry"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"golang.org/x/time/rate"
)

// Constants for Gemini API
const (
	cloudcodeBaseURL      = "https://cloudcode-pa.googleapis.com/v1internal"
	standardGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	geminiDefaultModel    = "gemini-2.5-flash" // Updated to stable Gemini 2.5 Flash
	loadCodeAssistRoute   = ":loadCodeAssist"
	onboardUserRoute      = ":onboardUser"
	pollInterval          = 5 * time.Second
)

// GeminiConfig represents Gemini-specific configuration
type GeminiConfig struct {
	// Basic configuration
	APIKey      string   `json:"api_key"`
	APIKeys     []string `json:"api_keys,omitempty"`
	BaseURL     string   `json:"base_url,omitempty"`
	Model       string   `json:"model,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`

	// Cloud Code API project ID
	ProjectID string `json:"project_id,omitempty"`

	// Backend configuration (Gemini API vs Vertex AI)
	Backend  BackendType `json:"backend,omitempty"`  // "gemini-api" or "vertex-ai"
	Project  string      `json:"project,omitempty"`  // GCP project ID for Vertex AI
	Location string      `json:"location,omitempty"` // GCP region for Vertex AI (e.g., "us-central1")

	// Service account for Vertex AI
	ServiceAccountJSON string `json:"service_account_json,omitempty"`
}

// NewGeminiProvider creates a new Gemini provider
func NewGeminiProvider(config types.ProviderConfig) *GeminiProvider {
	// Use the shared config helper
	configHelper := commonconfig.NewConfigHelper("Gemini", types.ProviderTypeGemini)

	// Merge with defaults and extract configuration
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract Gemini-specific config
	var geminiConfig GeminiConfig
	if err := configHelper.ExtractProviderSpecificConfig(mergedConfig, &geminiConfig); err != nil {
		// If extraction fails, use empty config and let helper handle defaults
		geminiConfig = GeminiConfig{}
	}

	// Apply top-level overrides using helper
	if err := configHelper.ApplyTopLevelOverrides(mergedConfig, &geminiConfig); err != nil {
		// In constructor, we log the error but continue with default config
		log.Printf("Warning: failed to apply top-level overrides in NewGeminiProvider: %v", err)
	}

	// Determine base URL for client pooling
	baseURL := geminiConfig.BaseURL
	if baseURL == "" {
		baseURL = standardGeminiBaseURL
	}

	// Get shared HTTP client from pool keyed by base URL
	timeout := configHelper.ExtractTimeout(mergedConfig)
	httpClient := clientpool.GetClientWithTimeout(baseURL, timeout)

	// Extract the underlying http.Client for compatibility with existing code
	client := httpClient.Client()

	// Create auth helper
	authHelper := auth.NewAuthHelper("gemini", mergedConfig, client)

	// Setup API keys using shared helper
	authHelper.SetupAPIKeys()

	// Setup OAuth using shared helper with refresh function factory
	refreshFactory := auth.NewRefreshFuncFactory("gemini", client)
	authHelper.SetupOAuth(refreshFactory.CreateGeminiRefreshFunc())

	// Initialize backend router with auto-detection
	clientConfig := &ClientConfig{
		Backend:            geminiConfig.Backend,
		Project:            geminiConfig.Project,
		Location:           geminiConfig.Location,
		APIKey:             geminiConfig.APIKey,
		ServiceAccountJSON: geminiConfig.ServiceAccountJSON,
		BaseURL:            geminiConfig.BaseURL,
	}
	backendRouter := NewBackendRouter(clientConfig)

	provider := &GeminiProvider{
		BaseProvider:    base.NewBaseProvider("gemini", mergedConfig, client, log.Default()),
		authHelper:      authHelper,
		client:          client,
		config:          geminiConfig,
		displayName:     geminiConfig.DisplayName,
		rateLimitHelper: common.NewRateLimitHelper(ratelimit.NewGeminiParser()),
		// Client-side limits (free tier: 15 RPM, pay-as-you-go: 360 RPM)
		// Default to free tier - can be updated with UpdateRateLimitTier
		clientSideLimiter: rate.NewLimiter(rate.Every(time.Minute/15), 15),
		backendRouter:     backendRouter,
	}

	// Set project ID if available
	if geminiConfig.ProjectID != "" {
		provider.projectID = geminiConfig.ProjectID
	}

	return provider
}

// Authenticate updates the provider's authentication configuration
func (p *GeminiProvider) Authenticate(ctx context.Context, authConfig types.AuthConfig) error {
	// Handle empty method gracefully (for test consistency)
	if authConfig.Method == "" {
		return nil // No authentication needed for test
	}

	// Use auth helper to validate authentication configuration
	if err := p.authHelper.ValidateAuthConfig(authConfig); err != nil {
		return err
	}

	switch authConfig.Method {
	case types.AuthMethodAPIKey:
		p.config.APIKey = authConfig.APIKey
		p.config.BaseURL = authConfig.BaseURL
		p.config.Model = authConfig.DefaultModel

	case types.AuthMethodOAuth:
		return types.NewAuthError(types.ProviderTypeGemini, "legacy OAuth authentication not supported - use multi-OAuth via OAuthCredentials").
			WithOperation("authenticate")

	default:
		return types.NewInvalidRequestError(types.ProviderTypeGemini, fmt.Sprintf("unsupported authentication method: %s", authConfig.Method)).
			WithOperation("authenticate")
	}

	return p.Configure(p.GetConfig())
}

// Configure updates the provider configuration
func (p *GeminiProvider) Configure(config types.ProviderConfig) error {
	// Use the shared config helper for validation and extraction
	configHelper := commonconfig.NewConfigHelper("Gemini", types.ProviderTypeGemini)

	// Validate configuration
	validation := configHelper.ValidateProviderConfig(config)
	if !validation.Valid {
		return types.NewInvalidRequestError(types.ProviderTypeGemini, validation.Errors[0]).
			WithOperation("configure")
	}

	// Merge with defaults
	mergedConfig := configHelper.MergeWithDefaults(config)

	// Extract and merge Gemini-specific config
	var geminiConfig GeminiConfig
	if err := configHelper.ExtractProviderSpecificConfig(mergedConfig, &geminiConfig); err != nil {
		// If extraction fails, use empty config and let helper handle defaults
		geminiConfig = GeminiConfig{}
	}

	// Apply top-level overrides using helper
	if err := configHelper.ApplyTopLevelOverrides(mergedConfig, &geminiConfig); err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to apply top-level overrides").
			WithOperation("configure").
			WithOriginalErr(err)
	}

	// Update provider state
	p.config = geminiConfig
	p.displayName = geminiConfig.DisplayName

	// Update auth helper configuration
	p.authHelper.Config = mergedConfig

	// Re-setup authentication with new config
	p.authHelper.SetupAPIKeys()
	refreshFactory := auth.NewRefreshFuncFactory("gemini", p.client)
	p.authHelper.SetupOAuth(refreshFactory.CreateGeminiRefreshFunc())

	// Update project ID if available
	if geminiConfig.ProjectID != "" {
		p.projectID = geminiConfig.ProjectID
	}

	// Reinitialize backend router with new configuration
	clientConfig := &ClientConfig{
		Backend:            geminiConfig.Backend,
		Project:            geminiConfig.Project,
		Location:           geminiConfig.Location,
		APIKey:             geminiConfig.APIKey,
		ServiceAccountJSON: geminiConfig.ServiceAccountJSON,
		BaseURL:            geminiConfig.BaseURL,
	}
	p.backendRouter = NewBackendRouter(clientConfig)

	return p.BaseProvider.Configure(mergedConfig)
}

// getProjectID returns the project ID from various sources
func (p *GeminiProvider) getProjectID() string {
	// Check config first
	if p.projectID != "" {
		return p.projectID
	}

	// Check environment variable
	if envID := os.Getenv("GOOGLE_CLOUD_PROJECT"); envID != "" {
		return envID
	}

	return ""
}

// testConnectivityWithAPIKey tests connectivity using an API key
func (p *GeminiProvider) testConnectivityWithAPIKey(ctx context.Context, apiKey string) error {
	// Use backend router to build the URL
	url := p.backendRouter.BuildRequestURL(geminiDefaultModel, "generateContent", apiKey)

	// Create a minimal request for connectivity testing
	minimalRequest := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": "Hi"},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 1, // Minimal token usage
		},
	}

	jsonBody, err := json.Marshal(minimalRequest)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())

	// Make the request with a shorter timeout for connectivity testing
	testClient := pkghttp.NewHTTPClient(pkghttp.HTTPClientConfig{
		Timeout: 15 * time.Second,
	})

	resp, err := testClient.Client().Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeGemini, "invalid API key").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeGemini, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	return nil
}

// testConnectivityWithOAuth tests connectivity using OAuth credentials
func (p *GeminiProvider) testConnectivityWithOAuth(ctx context.Context, accessToken string) error {
	// Use backend router to build the URL (no API key for OAuth)
	url := p.backendRouter.BuildRequestURL(geminiDefaultModel, "generateContent", "")

	// Create a minimal request for connectivity testing
	minimalRequest := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": "Hi"},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 1, // Minimal token usage
		},
	}

	jsonBody, err := json.Marshal(minimalRequest)
	if err != nil {
		return types.NewInvalidRequestError(types.ProviderTypeGemini, "failed to marshal test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "failed to create connectivity test request").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", telemetry.GetUserAgent())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	// Make the request with a shorter timeout for connectivity testing
	testClient := pkghttp.NewHTTPClient(pkghttp.HTTPClientConfig{
		Timeout: 15 * time.Second,
	})

	resp, err := testClient.Client().Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "connectivity test failed").
			WithOperation("test_connectivity").
			WithOriginalErr(err)
	}
	defer func() {
		//nolint:staticcheck // Empty branch is intentional - we ignore close errors
		if err := resp.Body.Close(); err != nil {
			// Log the error if a logger is available, or ignore it
		}
	}()

	// Check response status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return types.NewAuthError(types.ProviderTypeGemini, "invalid or expired OAuth token").
			WithOperation("test_connectivity").
			WithStatusCode(resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.NewServerError(types.ProviderTypeGemini, resp.StatusCode,
			fmt.Sprintf("connectivity test failed: %s", string(body))).
			WithOperation("test_connectivity")
	}

	return nil
}
