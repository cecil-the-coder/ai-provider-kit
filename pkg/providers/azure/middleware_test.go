package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAzureMiddleware(t *testing.T) {
	config := &AzureConfig{
		ResourceName: "test-resource",
		DeploymentID: "test-deployment",
		APIKey:       "test-key",
	}

	middleware, err := NewAzureMiddleware(config)
	require.NoError(t, err)
	require.NotNil(t, middleware)
	assert.Equal(t, "test-resource", middleware.config.ResourceName)
	assert.Equal(t, "test-deployment", middleware.config.DeploymentID)
}

func TestNewAzureMiddleware_CustomAPIVersion(t *testing.T) {
	config := &AzureConfig{
		ResourceName: "test-resource",
		DeploymentID: "test-deployment",
		APIVersion:   "2024-03-01-preview",
		APIKey:       "test-key",
	}

	middleware, err := NewAzureMiddleware(config)
	require.NoError(t, err)
	assert.Equal(t, "2024-03-01-preview", middleware.config.APIVersion)
}

func TestAzureConfig_Validate(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := &AzureConfig{
			ResourceName: "test-resource",
			DeploymentID: "test-deployment",
			APIKey:       "test-key",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("MissingResourceName", func(t *testing.T) {
		config := &AzureConfig{
			DeploymentID: "test-deployment",
			APIKey:       "test-key",
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("MissingDeploymentID", func(t *testing.T) {
		config := &AzureConfig{
			ResourceName: "test-resource",
			APIKey:       "test-key",
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("MissingAPIKey", func(t *testing.T) {
		config := &AzureConfig{
			ResourceName: "test-resource",
			DeploymentID: "test-deployment",
		}

		err := config.Validate()
		assert.Error(t, err)
	})
}
