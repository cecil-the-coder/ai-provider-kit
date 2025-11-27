package oauthmanager

import (
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// IsExpired checks if the access token is expired or will expire soon
// Uses a 5-minute buffer to avoid edge cases
func IsExpired(c *types.OAuthCredentialSet) bool {
	if c == nil {
		return true
	}
	if c.ExpiresAt.IsZero() {
		// If no expiry is set, assume token is valid
		// This is a defensive check for Phase 1
		return false
	}
	// Consider expired if less than 5 minutes remaining
	return time.Now().After(c.ExpiresAt.Add(-5 * time.Minute))
}

// Clone creates a deep copy of the credential set
// Used when updating credentials to avoid race conditions
func Clone(c *types.OAuthCredentialSet) *types.OAuthCredentialSet {
	if c == nil {
		return nil
	}

	clone := &types.OAuthCredentialSet{
		ID:             c.ID,
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		AccessToken:    c.AccessToken,
		RefreshToken:   c.RefreshToken,
		ExpiresAt:      c.ExpiresAt,
		LastRefresh:    c.LastRefresh,
		RefreshCount:   c.RefreshCount,
		OnTokenRefresh: c.OnTokenRefresh,
	}

	// Copy scopes slice
	if len(c.Scopes) > 0 {
		clone.Scopes = make([]string, len(c.Scopes))
		copy(clone.Scopes, c.Scopes)
	}

	return clone
}
