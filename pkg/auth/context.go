// Package auth provides authentication and credential management.
// This file provides context helpers for attaching authentication credentials
// to context objects throughout the request lifecycle.
package auth

import "context"

// Context keys for auth injection
type contextKey string

const (
	// ContextKeyOAuthToken is the context key for injecting an OAuth token
	//nolint:gosec // G101: This is a context key name, not a credential
	ContextKeyOAuthToken contextKey = "oauth_token"

	// ContextKeyAPIKey is the context key for injecting an API key
	//nolint:gosec // G101: This is a context key name, not a credential
	ContextKeyAPIKey contextKey = "api_key"

	// ContextKeyAuthType is the context key for specifying auth type ("bearer", "api_key")
	ContextKeyAuthType contextKey = "auth_type"
)

// WithAPIKey attaches an API key to the context for use in authentication.
// The API key can be retrieved later using GetAPIKey.
//
// Example:
//
//	ctx := auth.WithAPIKey(context.Background(), "sk-1234567890")
//
// The API key will be used by providers that require API key authentication.
func WithAPIKey(ctx context.Context, apiKey string) context.Context {
	return context.WithValue(ctx, ContextKeyAPIKey, apiKey)
}

// GetAPIKey extracts an API key from the context.
// Returns an empty string if no API key is present in the context.
//
// Example:
//
//	apiKey := auth.GetAPIKey(ctx)
//	if apiKey == "" {
//	    // Handle missing API key
//	}
func GetAPIKey(ctx context.Context) string {
	if apiKey, ok := ctx.Value(ContextKeyAPIKey).(string); ok {
		return apiKey
	}
	return ""
}

// WithOAuthToken attaches an OAuth token to the context for use in authentication.
// The OAuth token can be retrieved later using GetOAuthToken.
//
// Example:
//
//	ctx := auth.WithOAuthToken(context.Background(), "ya29.a0AfH6SMBx...")
//
// The OAuth token will be used by providers that require OAuth authentication.
func WithOAuthToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ContextKeyOAuthToken, token)
}

// GetOAuthToken extracts an OAuth token from the context.
// Returns an empty string if no OAuth token is present in the context.
//
// Example:
//
//	token := auth.GetOAuthToken(ctx)
//	if token == "" {
//	    // Handle missing OAuth token
//	}
func GetOAuthToken(ctx context.Context) string {
	if token, ok := ctx.Value(ContextKeyOAuthToken).(string); ok {
		return token
	}
	return ""
}

// WithAuthType attaches an authentication type to the context.
// The auth type specifies how credentials should be interpreted (e.g., "bearer", "api_key").
// The auth type can be retrieved later using GetAuthType.
//
// Example:
//
//	ctx := auth.WithAuthType(context.Background(), "bearer")
//
// Common auth types include:
//   - "bearer" - Bearer token authentication (default)
//   - "api_key" - API key authentication
func WithAuthType(ctx context.Context, authType string) context.Context {
	return context.WithValue(ctx, ContextKeyAuthType, authType)
}

// GetAuthType extracts the authentication type from the context.
// Returns "bearer" as the default if no auth type is present in the context.
//
// Example:
//
//	authType := auth.GetAuthType(ctx)
//	// authType will be "bearer" if not explicitly set
func GetAuthType(ctx context.Context) string {
	if authType, ok := ctx.Value(ContextKeyAuthType).(string); ok {
		return authType
	}
	return "bearer"
}
