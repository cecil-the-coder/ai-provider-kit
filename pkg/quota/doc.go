// Package quota provides a provider-agnostic quota management API for AI providers.
//
// This package defines interfaces and types for managing usage quotas across
// different AI providers, allowing for unified quota tracking, querying, and
// management regardless of the underlying provider implementation.
//
// Features:
//   - Provider-agnostic quota information structure
//   - Standardized quota types (requests, tokens, daily limits, custom)
//   - Quota history and usage tracking
//   - Extensible provider-specific quota metadata
//
// The quota system is designed to work with the existing ratelimit package,
// providing a higher-level API for quota management while ratelimit handles
// the low-level rate limit parsing and tracking.
//
// Usage Example:
//
//	// Create a new quota manager
//	manager := quota.NewManager()
//
//	// Update quota information from provider response
//	info := manager.UpdateFromRateLimit(rateLimitInfo)
//
//	// Query quota for a specific provider/model
//	quota, err := manager.GetQuota(provider, model)
//	if err == nil {
//		fmt.Printf("Remaining requests: %d/%d\n", quota.RequestsRemaining, quota.RequestsLimit)
//	}
//
// Provider Implementations:
//
// Providers can implement the QuotaProvider interface to provide real-time
// quota information from their APIs. The Manager aggregates this information
// and provides a unified API for querying quotas.
package quota
