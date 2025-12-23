// Package gemini provides Vertex AI authentication support including service account authentication.
package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	commonhttp "github.com/cecil-the-coder/ai-provider-kit/internal/common/http"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ServiceAccountCredentials represents a GCP service account JSON key file
type ServiceAccountCredentials struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// VertexAIAuthenticator handles authentication for Vertex AI
type VertexAIAuthenticator struct {
	serviceAccount *ServiceAccountCredentials
	accessToken    string
	expiresAt      time.Time
	client         *http.Client
}

// NewVertexAIAuthenticator creates a new Vertex AI authenticator
// It can use service account JSON from various sources:
// 1. Explicit serviceAccountJSON parameter
// 2. GOOGLE_APPLICATION_CREDENTIALS environment variable (file path)
// 3. GOOGLE_SERVICE_ACCOUNT_JSON environment variable (JSON string)
func NewVertexAIAuthenticator(serviceAccountJSON string, client *http.Client) (*VertexAIAuthenticator, error) {
	var credentials *ServiceAccountCredentials

	// Try to get credentials from various sources
	if serviceAccountJSON != "" {
		// Parse explicit JSON
		var creds ServiceAccountCredentials
		if err := json.Unmarshal([]byte(serviceAccountJSON), &creds); err != nil {
			return nil, types.NewAuthError(types.ProviderTypeGemini, "failed to parse service account JSON").
				WithOperation("authenticate").
				WithOriginalErr(err)
		}
		credentials = &creds
	} else if credsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credsPath != "" {
		// Load from file path
		data, err := os.ReadFile(credsPath) // #nosec G304 -- File path from environment variable is acceptable for credential loading
		if err != nil {
			return nil, types.NewAuthError(types.ProviderTypeGemini, "failed to read service account file").
				WithOperation("authenticate").
				WithOriginalErr(err)
		}
		var creds ServiceAccountCredentials
		if err := json.Unmarshal(data, &creds); err != nil {
			return nil, types.NewAuthError(types.ProviderTypeGemini, "failed to parse service account file").
				WithOperation("authenticate").
				WithOriginalErr(err)
		}
		credentials = &creds
	} else if credsJSON := os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON"); credsJSON != "" {
		// Load from environment variable
		var creds ServiceAccountCredentials
		if err := json.Unmarshal([]byte(credsJSON), &creds); err != nil {
			return nil, types.NewAuthError(types.ProviderTypeGemini, "failed to parse GOOGLE_SERVICE_ACCOUNT_JSON").
				WithOperation("authenticate").
				WithOriginalErr(err)
		}
		credentials = &creds
	} else {
		// No credentials found - will fall back to default application credentials
		// which might be available from GCE metadata server or gcloud CLI
		return &VertexAIAuthenticator{
			client: client,
		}, nil
	}

	return &VertexAIAuthenticator{
		serviceAccount: credentials,
		client:         client,
	}, nil
}

// GetAccessToken returns a valid access token, refreshing if necessary
func (v *VertexAIAuthenticator) GetAccessToken(ctx context.Context) (string, error) {
	// Check if we have a cached token that's still valid
	if v.accessToken != "" && time.Now().Before(v.expiresAt) {
		return v.accessToken, nil
	}

	// Refresh the token
	if err := v.refreshToken(ctx); err != nil {
		return "", err
	}

	return v.accessToken, nil
}

// refreshToken obtains a new access token
func (v *VertexAIAuthenticator) refreshToken(ctx context.Context) error {
	if v.serviceAccount != nil {
		// Use service account authentication
		return v.refreshWithServiceAccount(ctx)
	}

	// Fall back to metadata server (for GCE instances)
	return v.refreshWithMetadataServer(ctx)
}

// refreshWithServiceAccount uses JWT assertion to get an access token
func (v *VertexAIAuthenticator) refreshWithServiceAccount(ctx context.Context) error {
	// Note: This is a simplified implementation
	// In production, you should use the official Google Cloud SDK
	// For now, we'll use the service account to call the OAuth2 token endpoint

	// Create JWT assertion (simplified - in production use proper JWT library)
	// For this implementation, we'll use the refresh token flow with service account
	tokenURL := v.serviceAccount.TokenURI // #nosec G101 -- Not a hardcoded credential, it's a URL field from config
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token" // #nosec G101 -- OAuth2 endpoint URL, not a credential
	}

	// Prepare the request
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	// Note: In a full implementation, you would create a proper JWT here
	// For now, this is a placeholder that shows the structure

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "failed to create token request").
			WithOperation("refresh_service_account_token").
			WithOriginalErr(err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "failed to get access token").
			WithOperation("refresh_service_account_token").
			WithOriginalErr(err)
	}
	defer commonhttp.SafeClose(resp)

	if err := commonhttp.HandleHTTPStatusError(resp, types.ProviderTypeGemini, "refresh_service_account_token", ""); err != nil {
		return err
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	if err := commonhttp.UnmarshalJSONResponse(resp.Body, &tokenResponse, types.ProviderTypeGemini, "refresh_service_account_token"); err != nil {
		return err
	}

	v.accessToken = tokenResponse.AccessToken
	v.expiresAt = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)

	return nil
}

// refreshWithMetadataServer gets credentials from GCE metadata server
func (v *VertexAIAuthenticator) refreshWithMetadataServer(ctx context.Context) error {
	// GCE metadata server endpoint
	metadataURL := "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "failed to create metadata request").
			WithOperation("refresh_service_account_token").
			WithOriginalErr(err)
	}

	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := v.client.Do(req)
	if err != nil {
		return types.NewNetworkError(types.ProviderTypeGemini, "failed to get token from metadata server").
			WithOperation("refresh_service_account_token").
			WithOriginalErr(err)
	}
	defer commonhttp.SafeClose(resp)

	if err := commonhttp.HandleHTTPStatusError(resp, types.ProviderTypeGemini, "refresh_service_account_token", ""); err != nil {
		return err
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	if err := commonhttp.UnmarshalJSONResponse(resp.Body, &tokenResponse, types.ProviderTypeGemini, "refresh_service_account_token"); err != nil {
		return err
	}

	v.accessToken = tokenResponse.AccessToken
	v.expiresAt = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)

	return nil
}

// GetProjectID returns the project ID from the service account
func (v *VertexAIAuthenticator) GetProjectID() string {
	if v.serviceAccount != nil {
		return v.serviceAccount.ProjectID
	}
	return ""
}

// HasServiceAccount returns true if service account credentials are configured
func (v *VertexAIAuthenticator) HasServiceAccount() bool {
	return v.serviceAccount != nil
}

// IsAuthenticated returns true if we have valid credentials
func (v *VertexAIAuthenticator) IsAuthenticated() bool {
	return v.serviceAccount != nil || v.canUseMetadataServer()
}

// canUseMetadataServer checks if we're running in a GCE environment
func (v *VertexAIAuthenticator) canUseMetadataServer() bool {
	// Simple check - in production you might want to verify the metadata server is actually available
	return os.Getenv("GCE_METADATA_HOST") != "" || v.isGCEEnvironment()
}

// isGCEEnvironment checks for common GCE environment indicators
func (v *VertexAIAuthenticator) isGCEEnvironment() bool {
	// Check for common GCE environment variables
	return os.Getenv("GOOGLE_CLOUD_PROJECT") != "" ||
		os.Getenv("GCP_PROJECT") != "" ||
		os.Getenv("GCLOUD_PROJECT") != ""
}
