package factory

import (
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVirtualProviderRegistration(t *testing.T) {
	factory := NewProviderFactory()
	RegisterDefaultProviders(factory)

	t.Run("Racing Provider", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeRacing,
			Name: "test-racing",
			ProviderConfig: map[string]interface{}{
				"timeout_ms":       10000,
				"grace_period_ms":  200,
				"strategy":         "first_wins",
				"providers":        []string{"openai", "anthropic"},
				"performance_file": "/tmp/perf.json",
			},
		}

		provider, err := factory.CreateProvider(types.ProviderTypeRacing, config)
		require.NoError(t, err)
		require.NotNil(t, provider)

		assert.Equal(t, "test-racing", provider.Name())
		assert.Equal(t, types.ProviderType("racing"), provider.Type())
		assert.Contains(t, provider.Description(), "Races")
	})

	t.Run("Fallback Provider", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeFallback,
			Name: "test-fallback",
			ProviderConfig: map[string]interface{}{
				"max_retries": 5,
				"providers":   []string{"openai", "anthropic", "gemini"},
			},
		}

		provider, err := factory.CreateProvider(types.ProviderTypeFallback, config)
		require.NoError(t, err)
		require.NotNil(t, provider)

		assert.Equal(t, "test-fallback", provider.Name())
		assert.Equal(t, types.ProviderType("fallback"), provider.Type())
		assert.Contains(t, provider.Description(), "order")
	})

	t.Run("LoadBalance Provider", func(t *testing.T) {
		config := types.ProviderConfig{
			Type: types.ProviderTypeLoadBalance,
			Name: "test-loadbalance",
			ProviderConfig: map[string]interface{}{
				"strategy":  "round_robin",
				"providers": []string{"openai", "anthropic"},
			},
		}

		provider, err := factory.CreateProvider(types.ProviderTypeLoadBalance, config)
		require.NoError(t, err)
		require.NotNil(t, provider)

		assert.Equal(t, "test-loadbalance", provider.Name())
		assert.Equal(t, types.ProviderType("loadbalance"), provider.Type())
		assert.Contains(t, provider.Description(), "Distributes")
	})

	t.Run("Virtual Providers in Supported List", func(t *testing.T) {
		supportedProviders := factory.GetSupportedProviders()

		// Check that virtual providers are in the supported list
		hasRacing := false
		hasFallback := false
		hasLoadBalance := false

		for _, pt := range supportedProviders {
			if pt == types.ProviderTypeRacing {
				hasRacing = true
			}
			if pt == types.ProviderTypeFallback {
				hasFallback = true
			}
			if pt == types.ProviderTypeLoadBalance {
				hasLoadBalance = true
			}
		}

		assert.True(t, hasRacing, "Racing provider should be registered")
		assert.True(t, hasFallback, "Fallback provider should be registered")
		assert.True(t, hasLoadBalance, "LoadBalance provider should be registered")
	})

	t.Run("Virtual Provider Default Config", func(t *testing.T) {
		// Test creating virtual providers with minimal config
		config := types.ProviderConfig{
			Type: types.ProviderTypeRacing,
			Name: "minimal-racing",
		}

		provider, err := factory.CreateProvider(types.ProviderTypeRacing, config)
		require.NoError(t, err)
		require.NotNil(t, provider)

		// Provider should be created with defaults
		assert.Equal(t, "minimal-racing", provider.Name())
	})
}
