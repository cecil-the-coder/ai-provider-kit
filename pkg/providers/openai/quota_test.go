package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/quota"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIProvider_GetQuota(t *testing.T) {
	// Create a test server with mock responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization
		auth := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer sk-test-key", auth)

		// Route based on URL path
		switch r.URL.Path {
		case "/v1/dashboard/billing/subscription":
			subscriptionResponse := OpenAISubscriptionResponse{
				Object: "billing.subscription",
				Subscription: OpenAISubscription{
					Plan: OpenAIPlan{
						ID:              "test-plan",
						Title:           "Test Plan",
						Description:     "Test billing plan",
						IsActive:        true,
						SoftLimitUSD:    100.0,
						HardLimitUSD:    120.0,
					},
					AccountName:       "Test Account",
					UserID:            "test-user-id",
					SoftLimitUSD:      100.0,
					HardLimitUSD:      120.0,
					HasPaymentMethod:  true,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(subscriptionResponse)

		case "/v1/dashboard/billing/usage":
			usageResponse := OpenAIUsageResponse{
				Object: "billing.usage",
				Daily: []OpenAIUsageDayData{
					{
						Timestamp: time.Now().Add(-24 * time.Hour).Unix(),
						LineItems: []OpenAIUsageLineItem{
							{
								Name:           "gpt-4",
								QuantifiedCost: 0.50,
							},
						},
						AggregatedUsage: []OpenAIAggregatedUsage{
							{
								Tokens:    250000,
								Requests:  100,
								Operation: "chat_completion",
								Model:     "gpt-4",
							},
						},
					},
				},
				Metadata: OpenAIUsageMetadata{
					BillingPeriod: OpenAIBillingPeriod{
						StartDate: time.Now().AddDate(0, -1, 0).Format("2006-01-02"),
						EndDate:   time.Now().Format("2006-01-02"),
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(usageResponse)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create provider with test server
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenAI,
		APIKey:  "sk-test-key",
		BaseURL: server.URL,
	}
	provider := NewOpenAIProvider(config)

	ctx := context.Background()

	// Test GetQuota for a specific model
	t.Run("GetQuota for specific model", func(t *testing.T) {
		quotaInfo, err := provider.GetQuota(ctx, "gpt-4")
		require.NoError(t, err)
		require.NotNil(t, quotaInfo)

		assert.Equal(t, "OpenAI", quotaInfo.Provider)
		assert.Equal(t, "gpt-4", quotaInfo.Model)
		assert.NotZero(t, quotaInfo.Timestamp)

		// Check daily quota
		dailyQuota, exists := quotaInfo.Quotas[quota.QuotaTypeDaily]
		assert.True(t, exists, "daily quota should exist")
		assert.Greater(t, dailyQuota.Limit, 0)
		assert.GreaterOrEqual(t, dailyQuota.Remaining, 0)
		assert.LessOrEqual(t, dailyQuota.RemainingPercent, 100.0)

		// Check tokens quota
		tokensQuota, exists := quotaInfo.Quotas[quota.QuotaTypeTokens]
		assert.True(t, exists, "tokens quota should exist")
		assert.Greater(t, tokensQuota.Limit, 0)

		// Check custom usage
		assert.Contains(t, quotaInfo.CustomUsage, "total_cost_usd")
		assert.Contains(t, quotaInfo.CustomUsage, "soft_limit_usd")
		assert.Contains(t, quotaInfo.CustomUsage, "hard_limit_usd")
	})

	// Test GetQuota without specific model (provider-wide)
	t.Run("GetQuota provider-wide", func(t *testing.T) {
		quotaInfo, err := provider.GetQuota(ctx, "")
		require.NoError(t, err)
		require.NotNil(t, quotaInfo)

		assert.Equal(t, "OpenAI", quotaInfo.Provider)
		assert.Equal(t, "", quotaInfo.Model)
	})

	// Test that we can get MockQuotaInfo via GetQuotaInfo
	t.Run("GetQuotaInfo - backward compatibility", func(t *testing.T) {
		quotaInfo, err := provider.GetQuotaInfo(ctx, "gpt-4")
		require.NoError(t, err)
		require.NotNil(t, quotaInfo)

		assert.Equal(t, "OpenAI", quotaInfo.Provider)
		assert.NotNil(t, quotaInfo.Quotas)
	})

	// Test SupportsQuotaReporting
	t.Run("SupportsQuotaReporting", func(t *testing.T) {
		assert.True(t, provider.SupportsQuotaReporting())
	})

	// Test with no API keys
	t.Run("GetQuota with no API keys", func(t *testing.T) {
		configNoKey := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "",
		}
		providerNoKey := NewOpenAIProvider(configNoKey)

		_, err := providerNoKey.GetQuota(ctx, "gpt-4")
		assert.Error(t, err)
	})

	// Test with custom base URL (OpenAI-compatible provider)
	t.Run("GetQuota with compatible provider", func(t *testing.T) {
		compatibleConfig := types.ProviderConfig{
			Type:    types.ProviderTypeOpenAI,
			BaseURL: "https://api.openai-compatible.example.com",
			APIKey:  "sk-test-key",
		}
		compatibleProvider := NewOpenAIProvider(compatibleConfig)

		quotaInfo, err := compatibleProvider.GetQuota(ctx, "gpt-4")
		require.NoError(t, err)
		require.NotNil(t, quotaInfo)

		// Should return mock data for compatible providers
		assert.NotNil(t, quotaInfo.Quotas)
	})
}

func TestOpenAIProvider_GetQuotaHistory(t *testing.T) {
	// Create a test server with mock usage history data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer sk-test-key", auth)

		if r.URL.Path == "/v1/dashboard/billing/usage" {
			// Generate mock daily data for the last 30 days
			daily := make([]OpenAIUsageDayData, 0)
			now := time.Now().UTC()

			for i := 7; i >= 0; i-- {
				day := now.AddDate(0, 0, -i)
				dayTimestamp := day.Truncate(24 * time.Hour).Unix()

				daily = append(daily, OpenAIUsageDayData{
					Timestamp: dayTimestamp,
					LineItems: []OpenAIUsageLineItem{
						{
							Name:           "gpt-4",
							QuantifiedCost: 0.50,
						},
					},
					AggregatedUsage: []OpenAIAggregatedUsage{
						{
							Tokens:    int64((8 - i) * 10000),
							Requests:  int64((8 - i) * 10),
							Operation: "chat_completion",
							Model:     "gpt-4",
						},
					},
				})
			}

			usageResponse := OpenAIUsageResponse{
				Object: "billing.usage",
				Daily:  daily,
				Metadata: OpenAIUsageMetadata{
					BillingPeriod: OpenAIBillingPeriod{
						StartDate: now.AddDate(0, -1, 0).Format("2006-01-02"),
						EndDate:   now.Format("2006-01-02"),
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(usageResponse)
		}
	}))
	defer server.Close()

	// Create provider with test server
	config := types.ProviderConfig{
		Type:    types.ProviderTypeOpenAI,
		APIKey:  "sk-test-key",
		BaseURL: server.URL,
	}
	provider := NewOpenAIProvider(config)

	ctx := context.Background()

	// Test GetQuotaHistory
	t.Run("GetQuotaHistory for time range", func(t *testing.T) {
		now := time.Now().UTC()
		startTime := now.AddDate(0, 0, -7)
		endTime := now

		history, err := provider.GetQuotaHistory(ctx, "gpt-4", startTime, endTime)
		require.NoError(t, err)
		require.NotNil(t, history)

		assert.Equal(t, "OpenAI", history.Provider)
		assert.Equal(t, "gpt-4", history.Model)
		assert.LessOrEqual(t, history.StartTime, endTime)
		assert.GreaterOrEqual(t, history.EndTime, startTime)

		// Check that we have records
		assert.Greater(t, len(history.Records), 0)

		// Check total usage
		assert.Contains(t, history.TotalUsage, quota.QuotaTypeTokens)
		assert.Contains(t, history.TotalUsage, quota.QuotaTypeRequests)
		assert.GreaterOrEqual(t, history.TotalUsage[quota.QuotaTypeTokens], 0)

		// Verify records have expected structure
		for _, record := range history.Records {
			assert.Equal(t, "OpenAI", record.Provider)
			assert.Equal(t, "gpt-4", record.Model)
			assert.NotEmpty(t, record.ID)
			assert.False(t, record.Timestamp.IsZero())
			assert.Contains(t, record.Usage, quota.QuotaTypeTokens)
			assert.Contains(t, record.Usage, quota.QuotaTypeRequests)
		}
	})

	// Test GetQuotaHistory for all models
	t.Run("GetQuotaHistory all models", func(t *testing.T) {
		now := time.Now().UTC()
		startTime := now.AddDate(0, 0, -7)
		endTime := now

		history, err := provider.GetQuotaHistory(ctx, "", startTime, endTime)
		require.NoError(t, err)
		require.NotNil(t, history)

		assert.Equal(t, "", history.Model)
		assert.Greater(t, len(history.Records), 0)
	})

	// Test SupportsQuotaHistory
	t.Run("SupportsQuotaHistory", func(t *testing.T) {
		assert.True(t, provider.SupportsQuotaHistory())
	})

	// Test with no API keys
	t.Run("GetQuotaHistory with no API keys", func(t *testing.T) {
		configNoKey := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "",
		}
		providerNoKey := NewOpenAIProvider(configNoKey)

		_, err := providerNoKey.GetQuotaHistory(ctx, "gpt-4", time.Now(), time.Now())
		assert.Error(t, err)
	})
}

func TestOpenAIQuotaTypes(t *testing.T) {
	t.Run("OpenAIUsageRequest structure", func(t *testing.T) {
		_ = OpenAIUsageRequest{
			StartDate: "2023-01-01",
			EndDate:   "2023-01-31",
		}
		// Structure is validated by compilation
	})

	t.Run("OpenAISubscriptionResponse structure", func(t *testing.T) {
		sub := OpenAISubscriptionResponse{
			Object: "billing.subscription",
			Subscription: OpenAISubscription{
				Plan: OpenAIPlan{
					ID:              "test-plan",
					Title:           "Test Plan",
					IsActive:        true,
					SoftLimitUSD:    100.0,
					HardLimitUSD:    120.0,
				},
				AccountName:      "Test Account",
				HasPaymentMethod: true,
			},
		}

		assert.Equal(t, "billing.subscription", sub.Object)
		assert.Equal(t, "test-plan", sub.Subscription.Plan.ID)
		assert.Equal(t, "Test Plan", sub.Subscription.Plan.Title)
		assert.True(t, sub.Subscription.Plan.IsActive)
		assert.Equal(t, 100.0, sub.Subscription.Plan.SoftLimitUSD)
	})
}

func TestOpenAIQuotaHelperFunctions(t *testing.T) {
	t.Run("isCodeModel", func(t *testing.T) {
		assert.True(t, isCodeModel("code-davinci-002"))
		assert.True(t, isCodeModel("code-cushman-001"))
		assert.False(t, isCodeModel("gpt-4"))
		assert.False(t, isCodeModel("gpt-3.5-turbo"))
	})

	t.Run("isChatModel", func(t *testing.T) {
		assert.True(t, isChatModel("gpt-4"))
		assert.True(t, isChatModel("gpt-4-turbo"))
		assert.True(t, isChatModel("gpt-4o"))
		assert.True(t, isChatModel("gpt-3.5-turbo"))
		assert.True(t, isChatModel("gpt-3.5-turbo-instruct"))
		assert.False(t, isChatModel("code-davinci-002"))
	})

	t.Run("percentageRemaining", func(t *testing.T) {
		// Full quota
		assert.Equal(t, 100.0, percentageRemaining(0, 100))
		// Half used
		assert.Equal(t, 50.0, percentageRemaining(50, 100))
		// All used
		assert.Equal(t, 0.0, percentageRemaining(100, 100))
		// Over limit
		assert.Equal(t, 0.0, percentageRemaining(150, 100))
		// Zero limit
		assert.Equal(t, 0.0, percentageRemaining(10, 0))
	})

	t.Run("getNextResetTime", func(t *testing.T) {
		resetTime := getNextResetTime()
		now := time.Now().UTC()
		nextDay := now.Add(24 * time.Hour)

		// Reset time should be tomorrow at midnight UTC
		assert.True(t, resetTime.After(now))
		assert.True(t, resetTime.Before(nextDay.Add(time.Hour)))
		assert.Equal(t, 0, resetTime.Hour())
		assert.Equal(t, 0, resetTime.Minute())
		assert.Equal(t, 0, resetTime.Second())
	})

	t.Run("getMonthlyResetTime", func(t *testing.T) {
		sub := &OpenAISubscriptionResponse{
			Subscription: OpenAISubscription{
				Plan: OpenAIPlan{
					SoftLimitUSD: 100.0,
				},
			},
		}
		resetTime := getMonthlyResetTime(sub)
		now := time.Now().UTC()

		// Reset time should be first day of next month
		assert.True(t, resetTime.After(now))
		assert.Equal(t, 1, resetTime.Day())
	})

	t.Run("getPeriodStartTime", func(t *testing.T) {
		now := time.Now().UTC()

		dayStart := getPeriodStartTime(now, "day")
		assert.Equal(t, now.Year(), dayStart.Year())
		assert.Equal(t, now.Month(), dayStart.Month())
		assert.Equal(t, now.Day(), dayStart.Day())
		assert.Equal(t, 0, dayStart.Hour())
		assert.Equal(t, 0, dayStart.Minute())

		monthStart := getPeriodStartTime(now, "month")
		assert.Equal(t, now.Year(), monthStart.Year())
		assert.Equal(t, now.Month(), monthStart.Month())
		assert.Equal(t, 1, monthStart.Day())
	})

	t.Run("SafeStringToInt64", func(t *testing.T) {
		assert.Equal(t, int64(123), SafeStringToInt64("123"))
		assert.Equal(t, int64(0), SafeStringToInt64("invalid"))
		assert.Equal(t, int64(0), SafeStringToInt64(""))
	})

	t.Run("SafeStringToFloat64", func(t *testing.T) {
		assert.Equal(t, float64(123.45), SafeStringToFloat64("123.45"))
		assert.Equal(t, float64(0), SafeStringToFloat64("invalid"))
		assert.Equal(t, float64(0), SafeStringToFloat64(""))
	})

	t.Run("contains", func(t *testing.T) {
		assert.True(t, contains("gpt-4-turbo", "gpt-4"))
		assert.True(t, contains("gpt-3.5-turbo", "gpt-3.5"))
		assert.False(t, contains("gpt-4", "gpt-3.5"))
		assert.False(t, contains("short", "longer"))
	})
}

func TestOpenAIQuota_SummarizeUsage(t *testing.T) {
	t.Run("summarizeUsage", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		now := time.Now().UTC()

		usage := &OpenAIUsageResponse{
			Object: "billing.usage",
			Daily: []OpenAIUsageDayData{
				{
					Timestamp: now.Add(-24 * time.Hour).Unix(),
					LineItems: []OpenAIUsageLineItem{
						{
							Name:           "gpt-4",
							QuantifiedCost: 1.5,
						},
					},
					AggregatedUsage: []OpenAIAggregatedUsage{
						{
							Tokens:    100000,
							Requests:  50,
							Operation: "chat_completion",
							Model:     "gpt-4",
						},
						{
							Tokens:    50000,
							Requests:  25,
							Operation: "chat_completion",
							Model:     "code-davinci-002",
						},
					},
				},
				{
					Timestamp: now.Truncate(24 * time.Hour).Unix(),
					LineItems: []OpenAIUsageLineItem{
						{
							Name:           "gpt-4-turbo",
							QuantifiedCost: 0.75,
						},
					},
					AggregatedUsage: []OpenAIAggregatedUsage{
						{
							Tokens:    50000,
							Requests:  25,
							Operation: "chat_completion",
							Model:     "gpt-4-turbo",
						},
					},
				},
			},
		}

		summary := provider.summarizeUsage(usage)

		assert.Greater(t, summary.TotalTokens, int64(0))
		assert.Greater(t, summary.TotalRequests, int64(0))
		assert.Greater(t, summary.TotalCostUSD, float64(0))
		assert.NotNil(t, summary.Models)
		assert.Greater(t, len(summary.Models), 0)

		// Check that models are properly tracked
		assert.Contains(t, summary.Models, "gpt-4")
		assert.Contains(t, summary.Models, "codex")
		assert.Contains(t, summary.Models, "chatgpt")

		// Verify model-specific usage
		gpt4Usage, ok := summary.Models["gpt-4"]
		assert.True(t, ok)
		assert.Equal(t, int64(100000), gpt4Usage.Tokens)

		// Verify codex aggregation
		codexUsage, ok := summary.Models["codex"]
		assert.True(t, ok)
		assert.Equal(t, int64(50000), codexUsage.Tokens)

		// Verify chatgpt aggregation
		chatgptUsage, ok := summary.Models["chatgpt"]
		assert.True(t, ok)
		assert.Equal(t, int64(150000), chatgptUsage.Tokens) // gpt-4 + gpt-4-turbo
	})
}

func TestOpenAIQuota_MockData(t *testing.T) {
	t.Run("getMockSubscription", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		sub := provider.getMockSubscription()

		assert.NotNil(t, sub)
		assert.Equal(t, "billing.subscription", sub.Object)
		assert.NotNil(t, sub.Subscription.Plan)
		assert.Equal(t, "mock-plan", sub.Subscription.Plan.ID)
		assert.Equal(t, "Mock Provider Plan", sub.Subscription.Plan.Title)
		assert.Greater(t, sub.Subscription.SoftLimitUSD, float64(0))
	})

	t.Run("getMockUsage", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		usage := provider.getMockUsage()

		assert.NotNil(t, usage)
		assert.Equal(t, "billing.usage", usage.Object)
		assert.NotNil(t, usage.Daily)
		assert.Len(t, usage.Daily, 30) // 30 days

		// Check that daily data has proper structure
		for _, day := range usage.Daily {
			assert.Greater(t, day.Timestamp, int64(0))
			assert.NotEmpty(t, day.LineItems)
			assert.NotEmpty(t, day.AggregatedUsage)
		}
	})
}

func TestOpenAIQuota_ConvertFunctions(t *testing.T) {
	t.Run("convertQuotaUsageMap", func(t *testing.T) {
		result := convertQuotaUsageMap(map[quota.QuotaType]*quota.QuotaUsage{})
		assert.NotNil(t, result)
		assert.Equal(t, 0, len(result))
	})

	t.Run("convertQuotaConfigMap", func(t *testing.T) {
		result := convertQuotaConfigMap(map[quota.QuotaType]*quota.QuotaConfig{})
		assert.NotNil(t, result)
		assert.Equal(t, 0, len(result))
	})
}

func TestOpenAIQuota_ComputeEffectiveLimit(t *testing.T) {
	t.Run("computeEffectiveLimit", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		sub := &OpenAISubscriptionResponse{
			Subscription: OpenAISubscription{
				Plan: OpenAIPlan{
					SoftLimitUSD: 100.0,
				},
				SoftLimitUSD: 100.0, // The function reads from this field
			},
		}

		limit := provider.computeEffectiveLimit(sub)

		// Limit should be roughly soft_limit * tokens_per_usd
		// $100 * 500,000 tokens/USD = 50,000,000 tokens
		assert.Equal(t, int64(50000000), limit)
	})
}

func TestOpenAIProvider_QuotaInterface(t *testing.T) {
	t.Run("Implements QuotaProvider interface", func(t *testing.T) {
		config := types.ProviderConfig{
			Type:   types.ProviderTypeOpenAI,
			APIKey: "sk-test-key",
		}
		provider := NewOpenAIProvider(config)

		// Verify that the provider implements the required methods
		_ = provider.GetQuota
		_ = provider.GetQuotaHistory
		_ = provider.SupportsQuotaReporting

		// Verify backward compatibility with types.QuotaProvider
		_ = provider.GetQuotaInfo
		_ = provider.SupportsQuotaHistory
	})
}
