package anthropic

import (
	"context"
	"fmt"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/quota"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// quotaNotSupportedError is returned when quota operations are not supported
type quotaNotSupportedError struct {
	reason string
}

func (e *quotaNotSupportedError) Error() string {
	return fmt.Sprintf("quota operation not supported for Anthropic: %s", e.reason)
}

// GetQuota returns the current quota information for the provider.
// Anthropic provides quota information through rate limit headers in API responses.
// This method returns the most recently cached quota data from those headers.
func (p *AnthropicProvider) GetQuota(ctx context.Context, model string) (*quota.QuotaInfo, error) {
	// Get the latest rate limit info from the tracker
	info, exists := p.rateLimitHelper.GetRateLimitInfo(model)
	if !exists {
		return nil, &quotaNotSupportedError{
			reason: "no quota data available (make a request first to populate quota information)",
		}
	}

	// Convert ratelimit.Info to quota.QuotaInfo
	quotaInfo := p.convertRateLimitToQuotaInfo(info, model)
	return quotaInfo, nil
}

// GetQuotaHistory returns historical quota usage for the provider.
// Anthropic does not provide a programmatic API for historical usage data.
func (p *AnthropicProvider) GetQuotaHistory(ctx context.Context, model string, startTime, endTime time.Time) (*quota.QuotaHistory, error) {
	// Anthropic does not have an API endpoint for historical usage data
	return nil, &quotaNotSupportedError{
		reason: "historical usage data is not available via Anthropic's API. Use the Claude Console to view usage history: https://console.anthropic.com/settings/usage",
	}
}

// SupportsQuotaReporting returns true if the provider supports real-time quota reporting.
// Anthropic provides real-time quota information through rate limit response headers.
func (p *AnthropicProvider) SupportsQuotaReporting() bool {
	return true
}

// convertRateLimitToQuotaInfo converts ratelimit.Info to quota.QuotaInfo
func (p *AnthropicProvider) convertRateLimitToQuotaInfo(info *ratelimit.Info, model string) *quota.QuotaInfo {
	quotaInfo := &quota.QuotaInfo{
		Provider:     "anthropic",
		ProviderType: types.ProviderTypeAnthropic,
		Model:        model,
		Timestamp:    time.Now(),
		Quotas:       make(map[quota.QuotaType]*quota.QuotaUsage),
		Metadata:     make(map[string]interface{}),
	}

	// Request quota
	if info.RequestsLimit > 0 {
		quotaInfo.Quotas[quota.QuotaTypeRequests] = &quota.QuotaUsage{
			Type:             quota.QuotaTypeRequests,
			Period:           quota.QuotaPeriodMinute,
			Used:             info.RequestsLimit - info.RequestsRemaining,
			Limit:            info.RequestsLimit,
			Remaining:        info.RequestsRemaining,
			RemainingPercent: calculateRemainingPercent(info.RequestsRemaining, info.RequestsLimit),
			ResetAt:          info.RequestsReset,
			PeriodStartedAt:  calculatePeriodStart(info.RequestsReset, quota.QuotaPeriodMinute),
		}
	}

	// Input token quota
	if info.InputTokensLimit > 0 {
		quotaInfo.Quotas[quota.QuotaTypeInputTokens] = &quota.QuotaUsage{
			Type:             quota.QuotaTypeInputTokens,
			Period:           quota.QuotaPeriodMinute,
			Used:             info.InputTokensLimit - info.InputTokensRemaining,
			Limit:            info.InputTokensLimit,
			Remaining:        info.InputTokensRemaining,
			RemainingPercent: calculateRemainingPercent(info.InputTokensRemaining, info.InputTokensLimit),
			ResetAt:          info.InputTokensReset,
			PeriodStartedAt:  calculatePeriodStart(info.InputTokensReset, quota.QuotaPeriodMinute),
		}
	}

	// Output token quota
	if info.OutputTokensLimit > 0 {
		quotaInfo.Quotas[quota.QuotaTypeOutputTokens] = &quota.QuotaUsage{
			Type:             quota.QuotaTypeOutputTokens,
			Period:           quota.QuotaPeriodMinute,
			Used:             info.OutputTokensLimit - info.OutputTokensRemaining,
			Limit:            info.OutputTokensLimit,
			Remaining:        info.OutputTokensRemaining,
			RemainingPercent: calculateRemainingPercent(info.OutputTokensRemaining, info.OutputTokensLimit),
			ResetAt:          info.OutputTokensReset,
			PeriodStartedAt:  calculatePeriodStart(info.OutputTokensReset, quota.QuotaPeriodMinute),
		}
	}

	// Total token quota (for models that don't provide separate input/output)
	if info.TokensLimit > 0 && info.InputTokensLimit == 0 && info.OutputTokensLimit == 0 {
		quotaInfo.Quotas[quota.QuotaTypeTokens] = &quota.QuotaUsage{
			Type:             quota.QuotaTypeTokens,
			Period:           quota.QuotaPeriodMinute,
			Used:             info.TokensLimit - info.TokensRemaining,
			Limit:            info.TokensLimit,
			Remaining:        info.TokensRemaining,
			RemainingPercent: calculateRemainingPercent(info.TokensRemaining, info.TokensLimit),
			ResetAt:          info.TokensReset,
			PeriodStartedAt:  calculatePeriodStart(info.TokensReset, quota.QuotaPeriodMinute),
		}
	}

	// Add metadata
	if info.RequestID != "" {
		quotaInfo.Metadata["request_id"] = info.RequestID
	}
	if !info.Timestamp.IsZero() {
		quotaInfo.Metadata["rate_limit_timestamp"] = info.Timestamp
	}

	return quotaInfo
}

// calculateRemainingPercent calculates the percentage of quota remaining
func calculateRemainingPercent(remaining, limit int) float64 {
	if limit <= 0 {
		return 0
	}
	return float64(remaining) / float64(limit) * 100
}

// calculatePeriodStart estimates when the current quota period started
// based on the reset time and the period type.
func calculatePeriodStart(resetTime time.Time, period quota.QuotaPeriod) time.Time {
	if resetTime.IsZero() {
		return time.Time{}
	}

	now := time.Now()
	durationUntilReset := resetTime.Sub(now)

	// Ensure duration is positive and reasonable
	if durationUntilReset < 0 {
		durationUntilReset = 0
	} else if durationUntilReset > time.Hour*24 {
		// Cap at 24 hours to avoid unreasonable values
		durationUntilReset = time.Hour * 24
	}

	switch period {
	case quota.QuotaPeriodMinute:
		return resetTime.Add(-time.Minute)
	case quota.QuotaPeriodHour:
		return resetTime.Add(-time.Hour)
	case quota.QuotaPeriodDay:
		return resetTime.Add(-time.Hour * 24)
	case quota.QuotaPeriodWeek:
		return resetTime.Add(-time.Hour * 24 * 7)
	case quota.QuotaPeriodMonth:
		// Approximate as 30 days
		return resetTime.Add(-time.Hour * 24 * 30)
	default:
		return resetTime.Add(-time.Minute)
	}
}

// RefreshQuota triggers a minimal API request to refresh quota information
// This makes a lightweight request to the models endpoint to get fresh rate limit headers
func (p *AnthropicProvider) RefreshQuota(ctx context.Context, model string) (*quota.QuotaInfo, error) {
	// Use the TestConnectivity method which makes a request and updates rate limits from headers
	// This is a lightweight way to trigger a refresh of quota information
	if err := p.TestConnectivityWithOptions(ctx, true); err != nil {
		return nil, fmt.Errorf("failed to refresh quota: %w", err)
	}

	// Now get the updated quota
	return p.GetQuota(ctx, model)
}
