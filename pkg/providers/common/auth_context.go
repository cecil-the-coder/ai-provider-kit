package common

import "context"

// Context keys for auth injection
type contextKey string

const (
	// ContextKeyOAuthToken is the context key for injecting an OAuth token
	//nolint:gosec // G101: This is a context key name, not a credential
	ContextKeyOAuthToken contextKey = "oauth_token"

	// ContextKeyAuthType is the context key for specifying auth type ("bearer", "api_key")
	ContextKeyAuthType contextKey = "auth_type"
)

// WithOAuthToken returns a context with the OAuth token set
func WithOAuthToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ContextKeyOAuthToken, token)
}

// GetOAuthToken extracts OAuth token from context, returns empty string if not present
func GetOAuthToken(ctx context.Context) string {
	if token, ok := ctx.Value(ContextKeyOAuthToken).(string); ok {
		return token
	}
	return ""
}

// WithAuthType returns a context with the auth type set
func WithAuthType(ctx context.Context, authType string) context.Context {
	return context.WithValue(ctx, ContextKeyAuthType, authType)
}

// GetAuthType extracts auth type from context, defaults to "bearer"
func GetAuthType(ctx context.Context) string {
	if authType, ok := ctx.Value(ContextKeyAuthType).(string); ok {
		return authType
	}
	return "bearer"
}
