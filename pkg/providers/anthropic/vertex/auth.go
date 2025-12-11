package vertex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// GCP scope for Vertex AI
	vertexAIScope = "https://www.googleapis.com/auth/cloud-platform"
)

// AuthProvider handles GCP authentication for Vertex AI
type AuthProvider struct {
	config       *VertexConfig
	tokenSource  oauth2.TokenSource
	currentToken *oauth2.Token
	mu           sync.RWMutex
}

// NewAuthProvider creates a new authentication provider
func NewAuthProvider(config *VertexConfig) (*AuthProvider, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	provider := &AuthProvider{
		config: config,
	}

	// Initialize token source based on auth type
	if err := provider.initializeTokenSource(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize token source: %w", err)
	}

	return provider, nil
}

// initializeTokenSource sets up the OAuth2 token source based on the config
func (a *AuthProvider) initializeTokenSource(ctx context.Context) error {
	switch a.config.AuthType {
	case AuthTypeBearerToken:
		// For static bearer tokens, create a static token source
		a.currentToken = &oauth2.Token{
			AccessToken: a.config.BearerToken,
			TokenType:   "Bearer",
			// Set expiry far in the future for static tokens
			Expiry: time.Now().Add(100 * 365 * 24 * time.Hour),
		}
		a.tokenSource = oauth2.StaticTokenSource(a.currentToken)
		return nil

	case AuthTypeServiceAccount:
		var credentialsJSON []byte
		var err error

		switch {
		case a.config.ServiceAccountJSON != "":
			// Use raw JSON content
			credentialsJSON = []byte(a.config.ServiceAccountJSON)
		case a.config.ServiceAccountFile != "":
			// Read from file
			credentialsJSON, err = os.ReadFile(a.config.ServiceAccountFile)
			if err != nil {
				return fmt.Errorf("failed to read service account file: %w", err)
			}
		default:
			return fmt.Errorf("no service account credentials provided")
		}

		// Validate JSON format
		var test map[string]interface{}
		if err := json.Unmarshal(credentialsJSON, &test); err != nil {
			return fmt.Errorf("invalid service account JSON: %w", err)
		}

		// Create credentials from JSON
		creds, err := google.CredentialsFromJSON(ctx, credentialsJSON, vertexAIScope)
		if err != nil {
			return fmt.Errorf("failed to create credentials from JSON: %w", err)
		}

		a.tokenSource = creds.TokenSource
		return nil

	case AuthTypeApplicationDefault:
		// Use Application Default Credentials
		creds, err := google.FindDefaultCredentials(ctx, vertexAIScope)
		if err != nil {
			return fmt.Errorf("failed to find default credentials: %w", err)
		}

		a.tokenSource = creds.TokenSource
		return nil

	default:
		return fmt.Errorf("unsupported auth type: %s", a.config.AuthType)
	}
}

// GetToken returns a valid OAuth2 token, refreshing if necessary
func (a *AuthProvider) GetToken(ctx context.Context) (*oauth2.Token, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Get token from source (will automatically refresh if needed)
	token, err := a.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	a.currentToken = token
	return token, nil
}

// SetAuthHeader sets the Authorization header on the request
func (a *AuthProvider) SetAuthHeader(ctx context.Context, req *http.Request) error {
	token, err := a.GetToken(ctx)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	return nil
}

// IsTokenExpired checks if the current token is expired or about to expire
func (a *AuthProvider) IsTokenExpired() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.currentToken == nil {
		return true
	}

	// Consider token expired if it expires in less than 5 minutes
	return a.currentToken.Expiry.Before(time.Now().Add(5 * time.Minute))
}

// RefreshToken explicitly refreshes the token
func (a *AuthProvider) RefreshToken(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// For bearer tokens, no refresh needed
	if a.config.AuthType == AuthTypeBearerToken {
		return nil
	}

	// Get a fresh token
	token, err := a.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	a.currentToken = token
	return nil
}

// GetTokenInfo returns information about the current token
func (a *AuthProvider) GetTokenInfo() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	info := map[string]interface{}{
		"auth_type": string(a.config.AuthType),
	}

	if a.currentToken != nil {
		info["has_token"] = true
		info["token_type"] = a.currentToken.TokenType
		info["expires_at"] = a.currentToken.Expiry.Format(time.RFC3339)
		info["is_expired"] = a.currentToken.Expiry.Before(time.Now())
		info["time_until_expiry"] = time.Until(a.currentToken.Expiry).String()
	} else {
		info["has_token"] = false
	}

	return info
}

// ValidateToken validates that we can get a valid token
func (a *AuthProvider) ValidateToken(ctx context.Context) error {
	token, err := a.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if token.AccessToken == "" {
		return fmt.Errorf("token has empty access token")
	}

	if token.Expiry.Before(time.Now()) {
		return fmt.Errorf("token is expired")
	}

	return nil
}
