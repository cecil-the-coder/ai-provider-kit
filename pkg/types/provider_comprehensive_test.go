package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuthCredentialSet tests the OAuthCredentialSet struct
func TestOAuthCredentialSet(t *testing.T) {
	t.Run("ZeroValue", func(t *testing.T) {
		var creds OAuthCredentialSet
		assert.Empty(t, creds.ID)
		assert.Empty(t, creds.ClientID)
		assert.Empty(t, creds.ClientSecret)
		assert.Empty(t, creds.AccessToken)
		assert.Empty(t, creds.RefreshToken)
		assert.True(t, creds.ExpiresAt.IsZero())
		assert.Empty(t, creds.Scopes)
		assert.True(t, creds.LastRefresh.IsZero())
		assert.Equal(t, 0, creds.RefreshCount)
	})

	t.Run("FullCredentials", func(t *testing.T) {
		expiresAt := time.Now().Add(time.Hour)
		lastRefresh := time.Now().Add(-30 * time.Minute)

		creds := OAuthCredentialSet{
			ID:           "cred-1",
			ClientID:     "client-id-123",
			ClientSecret: "client-secret-456",
			AccessToken:  "access-token-789",
			RefreshToken: "refresh-token-abc",
			ExpiresAt:    expiresAt,
			Scopes:       []string{"read", "write", "admin"},
			LastRefresh:  lastRefresh,
			RefreshCount: 5,
		}

		assert.Equal(t, "cred-1", creds.ID)
		assert.Equal(t, "client-id-123", creds.ClientID)
		assert.Equal(t, "client-secret-456", creds.ClientSecret)
		assert.Equal(t, "access-token-789", creds.AccessToken)
		assert.Equal(t, "refresh-token-abc", creds.RefreshToken)
		assert.Equal(t, expiresAt, creds.ExpiresAt)
		assert.Equal(t, []string{"read", "write", "admin"}, creds.Scopes)
		assert.Equal(t, lastRefresh, creds.LastRefresh)
		assert.Equal(t, 5, creds.RefreshCount)
	})

	t.Run("JSONSerialization", func(t *testing.T) {
		creds := OAuthCredentialSet{
			ID:           "cred-test",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			AccessToken:  "test-access",
			RefreshToken: "test-refresh",
			ExpiresAt:    time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			Scopes:       []string{"read", "write"},
			LastRefresh:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			RefreshCount: 3,
		}

		data, err := json.Marshal(creds)
		require.NoError(t, err)

		var result OAuthCredentialSet
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, creds.ID, result.ID)
		assert.Equal(t, creds.ClientID, result.ClientID)
		assert.Equal(t, creds.ClientSecret, result.ClientSecret)
		assert.Equal(t, creds.AccessToken, result.AccessToken)
		assert.Equal(t, creds.RefreshToken, result.RefreshToken)
		assert.Equal(t, creds.Scopes, result.Scopes)
		assert.Equal(t, creds.RefreshCount, result.RefreshCount)
	})

	t.Run("IsExpired", func(t *testing.T) {
		// Not expired
		creds := OAuthCredentialSet{
			ExpiresAt: time.Now().Add(time.Hour),
		}
		assert.False(t, creds.IsExpired())

		// Expired
		creds.ExpiresAt = time.Now().Add(-time.Hour)
		assert.True(t, creds.IsExpired())

		// No expiration set
		creds.ExpiresAt = time.Time{}
		assert.False(t, creds.IsExpired())
	})

	t.Run("NeedsRefresh", func(t *testing.T) {
		// Needs refresh (expires soon)
		creds := OAuthCredentialSet{
			ExpiresAt: time.Now().Add(4 * time.Minute), // Less than 5 minutes
		}
		assert.True(t, creds.NeedsRefresh())

		// Doesn't need refresh
		creds.ExpiresAt = time.Now().Add(10 * time.Minute)
		assert.False(t, creds.NeedsRefresh())

		// Already expired
		creds.ExpiresAt = time.Now().Add(-time.Hour)
		assert.True(t, creds.NeedsRefresh())

		// No expiration
		creds.ExpiresAt = time.Time{}
		assert.False(t, creds.NeedsRefresh())
	})

	t.Run("UpdateTokens", func(t *testing.T) {
		creds := OAuthCredentialSet{
			ID:           "cred-1",
			RefreshCount: 2,
		}

		newExpiresAt := time.Now().Add(time.Hour)
		creds.UpdateTokens("new-access", "new-refresh", newExpiresAt)

		assert.Equal(t, "new-access", creds.AccessToken)
		assert.Equal(t, "new-refresh", creds.RefreshToken)
		assert.Equal(t, newExpiresAt, creds.ExpiresAt)
		assert.Equal(t, 3, creds.RefreshCount)
		assert.False(t, creds.LastRefresh.IsZero())
	})

	t.Run("CallbackHandling", func(t *testing.T) {
		callbackCalled := false
		var callbackID, callbackAccess, callbackRefresh string
		var callbackExpires time.Time

		creds := OAuthCredentialSet{
			ID: "cred-callback",
			OnTokenRefresh: func(id, accessToken, refreshToken string, expiresAt time.Time) error {
				callbackCalled = true
				callbackID = id
				callbackAccess = accessToken
				callbackRefresh = refreshToken
				callbackExpires = expiresAt
				return nil
			},
		}

		expiresAt := time.Now().Add(time.Hour)
		err := creds.RefreshAndNotify("new-access", "new-refresh", expiresAt)

		require.NoError(t, err)
		assert.True(t, callbackCalled)
		assert.Equal(t, "cred-callback", callbackID)
		assert.Equal(t, "new-access", callbackAccess)
		assert.Equal(t, "new-refresh", callbackRefresh)
		assert.Equal(t, expiresAt, callbackExpires)
	})

	t.Run("IsValid", func(t *testing.T) {
		// Valid credentials
		creds := OAuthCredentialSet{
			ID:           "cred-1",
			ClientID:     "client",
			ClientSecret: "secret",
			AccessToken:  "access",
			RefreshToken: "refresh",
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		assert.True(t, creds.IsValid())

		// Missing ID
		creds.ID = ""
		assert.False(t, creds.IsValid())

		// Missing client ID
		creds.ID = "cred-1"
		creds.ClientID = ""
		assert.False(t, creds.IsValid())

		// Expired
		creds.ClientID = "client"
		creds.ExpiresAt = time.Now().Add(-time.Hour)
		assert.False(t, creds.IsValid())
	})
}

// IsExpired checks if the credentials have expired
func (c *OAuthCredentialSet) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

// NeedsRefresh checks if the credentials need to be refreshed (expires within 5 minutes)
func (c *OAuthCredentialSet) NeedsRefresh() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}

// UpdateTokens updates the access and refresh tokens
func (c *OAuthCredentialSet) UpdateTokens(accessToken, refreshToken string, expiresAt time.Time) {
	c.AccessToken = accessToken
	c.RefreshToken = refreshToken
	c.ExpiresAt = expiresAt
	c.LastRefresh = time.Now()
	c.RefreshCount++
}

// RefreshAndNotify updates tokens and calls the callback if set
func (c *OAuthCredentialSet) RefreshAndNotify(accessToken, refreshToken string, expiresAt time.Time) error {
	c.UpdateTokens(accessToken, refreshToken, expiresAt)
	if c.OnTokenRefresh != nil {
		return c.OnTokenRefresh(c.ID, accessToken, refreshToken, expiresAt)
	}
	return nil
}

// IsValid checks if the credential set has all required fields and is not expired
func (c *OAuthCredentialSet) IsValid() bool {
	if c.ID == "" || c.ClientID == "" || c.ClientSecret == "" {
		return false
	}
	if c.AccessToken == "" {
		return false
	}
	return !c.IsExpired()
}

// TestVirtualProviderTypes tests virtual provider constants
func TestVirtualProviderTypes(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected string
	}{
		{"Racing", ProviderTypeRacing, "racing"},
		{"Fallback", ProviderTypeFallback, "fallback"},
		{"LoadBalance", ProviderTypeLoadBalance, "loadbalance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.provider))

			// Test JSON marshaling
			data, err := json.Marshal(tt.provider)
			require.NoError(t, err)
			assert.Equal(t, `"`+tt.expected+`"`, string(data))

			var result ProviderType
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)
			assert.Equal(t, tt.provider, result)
		})
	}
}

// TestProviderConfigEdgeCases tests provider config edge cases
func TestProviderConfigEdgeCases(t *testing.T) {
	t.Run("EmptyConfig", func(t *testing.T) {
		config := ProviderConfig{}
		assert.False(t, config.Validate())
	})

	t.Run("MinimalValidConfig", func(t *testing.T) {
		config := ProviderConfig{
			Type: ProviderTypeOpenAI,
			Name: "test",
		}
		assert.True(t, config.Validate())
	})

	t.Run("MultipleOAuthCredentials", func(t *testing.T) {
		config := ProviderConfig{
			Type: ProviderTypeOpenAI,
			Name: "multi-oauth",
			OAuthCredentials: []*OAuthCredentialSet{
				{
					ID:           "account-1",
					ClientID:     "client-1",
					ClientSecret: "secret-1",
					AccessToken:  "token-1",
					RefreshToken: "refresh-1",
					ExpiresAt:    time.Now().Add(time.Hour),
				},
				{
					ID:           "account-2",
					ClientID:     "client-2",
					ClientSecret: "secret-2",
					AccessToken:  "token-2",
					RefreshToken: "refresh-2",
					ExpiresAt:    time.Now().Add(2 * time.Hour),
				},
			},
		}

		assert.Len(t, config.OAuthCredentials, 2)
		assert.Equal(t, "account-1", config.OAuthCredentials[0].ID)
		assert.Equal(t, "account-2", config.OAuthCredentials[1].ID)
	})

	t.Run("GetActiveOAuthCredential", func(t *testing.T) {
		config := ProviderConfig{
			Type: ProviderTypeOpenAI,
			Name: "test",
			OAuthCredentials: []*OAuthCredentialSet{
				{
					ID:           "expired",
					ClientID:     "client",
					ClientSecret: "secret",
					AccessToken:  "token",
					ExpiresAt:    time.Now().Add(-time.Hour), // Expired
				},
				{
					ID:           "active",
					ClientID:     "client",
					ClientSecret: "secret",
					AccessToken:  "token",
					ExpiresAt:    time.Now().Add(time.Hour), // Valid
				},
			},
		}

		active := config.GetActiveOAuthCredential()
		require.NotNil(t, active)
		assert.Equal(t, "active", active.ID)
	})

	t.Run("NoActiveOAuthCredential", func(t *testing.T) {
		config := ProviderConfig{
			Type:             ProviderTypeOpenAI,
			Name:             "test",
			OAuthCredentials: []*OAuthCredentialSet{},
		}

		active := config.GetActiveOAuthCredential()
		assert.Nil(t, active)
	})

	t.Run("HasFeature", func(t *testing.T) {
		config := ProviderConfig{
			Type:                 ProviderTypeOpenAI,
			Name:                 "test",
			SupportsStreaming:    true,
			SupportsToolCalling:  true,
			SupportsResponsesAPI: false,
		}

		assert.True(t, config.HasFeature("streaming"))
		assert.True(t, config.HasFeature("tools"))
		assert.False(t, config.HasFeature("responses"))
		assert.False(t, config.HasFeature("unknown"))
	})
}

// GetActiveOAuthCredential returns the first valid (non-expired) OAuth credential
func (c *ProviderConfig) GetActiveOAuthCredential() *OAuthCredentialSet {
	for _, cred := range c.OAuthCredentials {
		if cred.IsValid() {
			return cred
		}
	}
	return nil
}

// HasFeature checks if a provider config has a specific feature
func (c *ProviderConfig) HasFeature(feature string) bool {
	switch feature {
	case "streaming":
		return c.SupportsStreaming
	case "tools", "tool_calling":
		return c.SupportsToolCalling
	case "responses", "responses_api":
		return c.SupportsResponsesAPI
	default:
		return false
	}
}

// TestHealthStatusHelpers tests health status helper methods
func TestHealthStatusHelpers(t *testing.T) {
	t.Run("IsHealthy", func(t *testing.T) {
		status := HealthStatus{
			Healthy:      true,
			StatusCode:   200,
			ResponseTime: 50.0,
		}
		assert.True(t, status.IsHealthy())

		status.Healthy = false
		assert.False(t, status.IsHealthy())
	})

	t.Run("IsResponseTimeFast", func(t *testing.T) {
		status := HealthStatus{ResponseTime: 50.0}
		assert.True(t, status.IsResponseTimeFast())

		status.ResponseTime = 200.0
		assert.False(t, status.IsResponseTimeFast())
	})

	t.Run("IsStale", func(t *testing.T) {
		status := HealthStatus{
			LastChecked: time.Now(),
		}
		assert.False(t, status.IsStale())

		status.LastChecked = time.Now().Add(-10 * time.Minute)
		assert.True(t, status.IsStale())
	})
}

// IsHealthy checks if the status is healthy
func (h HealthStatus) IsHealthy() bool {
	return h.Healthy
}

// IsResponseTimeFast checks if response time is under 100ms
func (h HealthStatus) IsResponseTimeFast() bool {
	return h.ResponseTime < 100.0
}

// IsStale checks if the health check is older than 5 minutes
func (h HealthStatus) IsStale() bool {
	return time.Since(h.LastChecked) > 5*time.Minute
}

// TestProviderMetricsHelpers tests provider metrics helpers
func TestProviderMetricsHelpers(t *testing.T) {
	t.Run("CalculateErrorRate", func(t *testing.T) {
		metrics := ProviderMetrics{
			RequestCount: 100,
			ErrorCount:   10,
		}
		assert.Equal(t, 0.1, metrics.CalculateErrorRate())

		metrics.RequestCount = 0
		assert.Equal(t, 0.0, metrics.CalculateErrorRate())
	})

	t.Run("CalculateSuccessRate", func(t *testing.T) {
		metrics := ProviderMetrics{
			RequestCount: 100,
			SuccessCount: 95,
		}
		assert.Equal(t, 0.95, metrics.CalculateSuccessRate())
	})

	t.Run("GetAvgTokensPerRequest", func(t *testing.T) {
		metrics := ProviderMetrics{
			RequestCount: 10,
			TokensUsed:   1000,
		}
		assert.Equal(t, int64(100), metrics.GetAvgTokensPerRequest())

		metrics.RequestCount = 0
		assert.Equal(t, int64(0), metrics.GetAvgTokensPerRequest())
	})
}

// CalculateErrorRate returns the error rate
func (m *ProviderMetrics) CalculateErrorRate() float64 {
	if m.RequestCount == 0 {
		return 0.0
	}
	return float64(m.ErrorCount) / float64(m.RequestCount)
}

// CalculateSuccessRate returns the success rate
func (m *ProviderMetrics) CalculateSuccessRate() float64 {
	if m.RequestCount == 0 {
		return 0.0
	}
	return float64(m.SuccessCount) / float64(m.RequestCount)
}

// GetAvgTokensPerRequest returns average tokens per request
func (m *ProviderMetrics) GetAvgTokensPerRequest() int64 {
	if m.RequestCount == 0 {
		return 0
	}
	return m.TokensUsed / m.RequestCount
}

// TestProviderInfoHelpers tests provider info helpers
func TestProviderInfoHelpers(t *testing.T) {
	t.Run("GetModelByID", func(t *testing.T) {
		info := ProviderInfo{
			Models: []Model{
				{ID: "gpt-4", Name: "GPT-4"},
				{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo"},
			},
		}

		model := info.GetModelByID("gpt-4")
		require.NotNil(t, model)
		assert.Equal(t, "gpt-4", model.ID)

		model = info.GetModelByID("nonexistent")
		assert.Nil(t, model)
	})

	t.Run("HasModel", func(t *testing.T) {
		info := ProviderInfo{
			Models: []Model{
				{ID: "gpt-4"},
			},
		}

		assert.True(t, info.HasModel("gpt-4"))
		assert.False(t, info.HasModel("gpt-3"))
	})

	t.Run("SupportsTool", func(t *testing.T) {
		info := ProviderInfo{
			SupportedTools: []string{"code_interpreter", "browsing"},
		}

		assert.True(t, info.SupportsTool("code_interpreter"))
		assert.False(t, info.SupportsTool("dalle"))
	})
}

// GetModelByID returns a model by ID
func (p *ProviderInfo) GetModelByID(id string) *Model {
	for i := range p.Models {
		if p.Models[i].ID == id {
			return &p.Models[i]
		}
	}
	return nil
}

// HasModel checks if a model exists
func (p *ProviderInfo) HasModel(id string) bool {
	return p.GetModelByID(id) != nil
}

// SupportsTool checks if a tool is supported
func (p *ProviderInfo) SupportsTool(toolName string) bool {
	for _, tool := range p.SupportedTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// BenchmarkProviderConfigValidate benchmarks config validation
func BenchmarkProviderConfigValidate(b *testing.B) {
	config := ProviderConfig{
		Type: ProviderTypeOpenAI,
		Name: "benchmark-provider",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}

// BenchmarkOAuthCredentialSetIsValid benchmarks credential validation
func BenchmarkOAuthCredentialSetIsValid(b *testing.B) {
	creds := OAuthCredentialSet{
		ID:           "bench-cred",
		ClientID:     "client",
		ClientSecret: "secret",
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = creds.IsValid()
	}
}
