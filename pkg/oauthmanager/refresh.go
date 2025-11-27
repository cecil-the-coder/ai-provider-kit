package oauthmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// RefreshBufferTime is the time before expiry when refresh should occur
// Tokens expiring within this window should be refreshed proactively
const RefreshBufferTime = 5 * time.Minute

// RefreshFunc is a provider-specific function that refreshes OAuth tokens
// It receives the current credential set and returns an updated credential set with new tokens
// The provider is responsible for:
// - Making the token refresh request to the OAuth server
// - Parsing the response and creating updated credential set
// - Updating LastRefresh and RefreshCount fields
type RefreshFunc func(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error)

// NeedsRefresh checks if a credential needs to be refreshed
// Returns true if the token expires within RefreshBufferTime or is already expired
// Returns false if:
// - Credential is nil
// - ExpiresAt is not set (zero time)
// - Token has sufficient time remaining (> RefreshBufferTime)
func NeedsRefresh(cred *types.OAuthCredentialSet) bool {
	if cred == nil || cred.ExpiresAt.IsZero() {
		return false
	}
	// Refresh if token expires within the buffer time
	return time.Now().After(cred.ExpiresAt.Add(-RefreshBufferTime))
}

// NoOpRefreshFunc is a refresh function that always returns an error
// Used when token refresh is not supported or configured for a provider
// This allows the OAuthKeyManager to be created even when refresh is not available
func NoOpRefreshFunc(ctx context.Context, cred *types.OAuthCredentialSet) (*types.OAuthCredentialSet, error) {
	return nil, fmt.Errorf("token refresh not configured for credential %s", cred.ID)
}
