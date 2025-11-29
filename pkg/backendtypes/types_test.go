package backendtypes

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ==================================================
// Config Types YAML Serialization Tests
// ==================================================

func TestBackendConfig_YAML(t *testing.T) {
	cfg := BackendConfig{
		Server: ServerConfig{
			Host:            "localhost",
			Port:            8080,
			Version:         "1.0.0",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
		Auth: AuthConfig{
			Enabled:     true,
			APIPassword: "secret123",
			APIKeyEnv:   "API_KEY",
			PublicPaths: []string{"/health", "/metrics"},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		CORS: CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
		},
		Providers: map[string]*types.ProviderConfig{
			"openai": {
				Type:         types.ProviderTypeOpenAI,
				APIKey:       "sk-test123",
				DefaultModel: "gpt-4",
			},
		},
		Extensions: map[string]ExtensionConfig{
			"caching": {
				Enabled: true,
				Config: map[string]interface{}{
					"ttl": 300,
				},
			},
		},
	}

	// Test marshaling
	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded BackendConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, cfg.Server.Host, decoded.Server.Host)
	assert.Equal(t, cfg.Server.Port, decoded.Server.Port)
	assert.Equal(t, cfg.Server.Version, decoded.Server.Version)
	assert.Equal(t, cfg.Server.ReadTimeout, decoded.Server.ReadTimeout)
	assert.Equal(t, cfg.Server.WriteTimeout, decoded.Server.WriteTimeout)
	assert.Equal(t, cfg.Server.ShutdownTimeout, decoded.Server.ShutdownTimeout)

	assert.Equal(t, cfg.Auth.Enabled, decoded.Auth.Enabled)
	assert.Equal(t, cfg.Auth.APIPassword, decoded.Auth.APIPassword)
	assert.Equal(t, cfg.Auth.APIKeyEnv, decoded.Auth.APIKeyEnv)
	assert.Equal(t, cfg.Auth.PublicPaths, decoded.Auth.PublicPaths)

	assert.Equal(t, cfg.Logging.Level, decoded.Logging.Level)
	assert.Equal(t, cfg.Logging.Format, decoded.Logging.Format)

	assert.Equal(t, cfg.CORS.Enabled, decoded.CORS.Enabled)
	assert.Equal(t, cfg.CORS.AllowedOrigins, decoded.CORS.AllowedOrigins)
	assert.Equal(t, cfg.CORS.AllowedMethods, decoded.CORS.AllowedMethods)
	assert.Equal(t, cfg.CORS.AllowedHeaders, decoded.CORS.AllowedHeaders)
}

func TestServerConfig_YAML(t *testing.T) {
	cfg := ServerConfig{
		Host:            "0.0.0.0",
		Port:            9090,
		Version:         "2.0.0",
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	var decoded ServerConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, cfg, decoded)
}

func TestAuthConfig_YAML(t *testing.T) {
	tests := []struct {
		name   string
		config AuthConfig
	}{
		{
			name: "enabled with password",
			config: AuthConfig{
				Enabled:     true,
				APIPassword: "password123",
				APIKeyEnv:   "MY_API_KEY",
				PublicPaths: []string{"/public", "/health"},
			},
		},
		{
			name: "disabled",
			config: AuthConfig{
				Enabled:     false,
				PublicPaths: []string{},
			},
		},
		{
			name: "with empty public paths",
			config: AuthConfig{
				Enabled:     true,
				APIPassword: "secret",
				PublicPaths: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(tt.config)
			require.NoError(t, err)

			var decoded AuthConfig
			err = yaml.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.config.Enabled, decoded.Enabled)
			assert.Equal(t, tt.config.APIPassword, decoded.APIPassword)
			assert.Equal(t, tt.config.APIKeyEnv, decoded.APIKeyEnv)
		})
	}
}

func TestLoggingConfig_YAML(t *testing.T) {
	tests := []struct {
		name   string
		config LoggingConfig
	}{
		{
			name:   "json format",
			config: LoggingConfig{Level: "debug", Format: "json"},
		},
		{
			name:   "text format",
			config: LoggingConfig{Level: "info", Format: "text"},
		},
		{
			name:   "error level",
			config: LoggingConfig{Level: "error", Format: "json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(tt.config)
			require.NoError(t, err)

			var decoded LoggingConfig
			err = yaml.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.config, decoded)
		})
	}
}

func TestCORSConfig_YAML(t *testing.T) {
	cfg := CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com", "https://api.example.com"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "X-API-Key"},
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	var decoded CORSConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, cfg, decoded)
}

func TestExtensionConfig_YAML(t *testing.T) {
	cfg := ExtensionConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"option1": "value1",
			"option2": 42,
			"option3": true,
			"nested": map[string]interface{}{
				"key": "value",
			},
		},
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	var decoded ExtensionConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, cfg.Enabled, decoded.Enabled)
	assert.NotNil(t, decoded.Config)
}

// ==================================================
// Request Types JSON Serialization Tests
// ==================================================

func TestGenerateRequest_JSON(t *testing.T) {
	req := GenerateRequest{
		Provider:    "openai",
		Model:       "gpt-4",
		Prompt:      "test prompt",
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Tools: []types.Tool{
			{
				Name:        "get_weather",
				Description: "Get current weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
		Metadata: map[string]interface{}{
			"user_id": "123",
		},
	}

	// Test marshaling
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify JSON structure
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)
	assert.Equal(t, "openai", jsonMap["provider"])
	assert.Equal(t, "gpt-4", jsonMap["model"])
	assert.Equal(t, "test prompt", jsonMap["prompt"])
	assert.Equal(t, float64(100), jsonMap["max_tokens"])
	assert.Equal(t, 0.7, jsonMap["temperature"])

	// Test unmarshaling
	var decoded GenerateRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.Provider, decoded.Provider)
	assert.Equal(t, req.Model, decoded.Model)
	assert.Equal(t, req.Prompt, decoded.Prompt)
	assert.Equal(t, req.MaxTokens, decoded.MaxTokens)
	assert.Equal(t, req.Temperature, decoded.Temperature)
	assert.Equal(t, req.Stream, decoded.Stream)
	assert.Len(t, decoded.Messages, 1)
	assert.Len(t, decoded.Tools, 1)
}

func TestGenerateRequest_MinimalJSON(t *testing.T) {
	req := GenerateRequest{
		Prompt: "simple prompt",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded GenerateRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.Prompt, decoded.Prompt)
	assert.Empty(t, decoded.Provider)
	assert.Empty(t, decoded.Model)
	assert.Zero(t, decoded.MaxTokens)
}

func TestGenerateRequest_WithMessages(t *testing.T) {
	req := GenerateRequest{
		Provider: "anthropic",
		Model:    "claude-3",
		Messages: []types.ChatMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
		MaxTokens:   200,
		Temperature: 0.5,
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded GenerateRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Messages, 3)
	assert.Equal(t, "user", decoded.Messages[0].Role)
	assert.Equal(t, "assistant", decoded.Messages[1].Role)
}

func TestProviderConfigRequest_JSON(t *testing.T) {
	tests := []struct {
		name string
		req  ProviderConfigRequest
	}{
		{
			name: "single API key",
			req: ProviderConfigRequest{
				Type:         "openai",
				APIKey:       "sk-test123",
				DefaultModel: "gpt-4",
				Enabled:      true,
			},
		},
		{
			name: "multiple API keys",
			req: ProviderConfigRequest{
				Type:         "anthropic",
				APIKeys:      []string{"key1", "key2", "key3"},
				DefaultModel: "claude-3",
				Enabled:      true,
			},
		},
		{
			name: "with base URL",
			req: ProviderConfigRequest{
				Type:         "openrouter",
				APIKey:       "sk-or-test",
				BaseURL:      "https://openrouter.ai/api/v1",
				DefaultModel: "anthropic/claude-3",
				Enabled:      true,
			},
		},
		{
			name: "disabled provider",
			req: ProviderConfigRequest{
				Type:    "gemini",
				Enabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.req)
			require.NoError(t, err)

			var decoded ProviderConfigRequest
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.req.Type, decoded.Type)
			assert.Equal(t, tt.req.APIKey, decoded.APIKey)
			assert.Equal(t, tt.req.APIKeys, decoded.APIKeys)
			assert.Equal(t, tt.req.DefaultModel, decoded.DefaultModel)
			assert.Equal(t, tt.req.BaseURL, decoded.BaseURL)
			assert.Equal(t, tt.req.Enabled, decoded.Enabled)
		})
	}
}

// ==================================================
// Response Types Tests
// ==================================================

func TestAPIResponse_Success(t *testing.T) {
	timestamp := time.Now()
	resp := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": "Operation successful",
			"count":   42,
		},
		RequestID: "req-123",
		Timestamp: timestamp,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded APIResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Success)
	assert.NotNil(t, decoded.Data)
	assert.Nil(t, decoded.Error)
	assert.Equal(t, "req-123", decoded.RequestID)
}

func TestAPIResponse_Error(t *testing.T) {
	timestamp := time.Now()
	resp := APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "INVALID_REQUEST",
			Message: "The request is invalid",
			Details: "Missing required field: prompt",
		},
		RequestID: "req-456",
		Timestamp: timestamp,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded APIResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.False(t, decoded.Success)
	assert.Nil(t, decoded.Data)
	require.NotNil(t, decoded.Error)
	assert.Equal(t, "INVALID_REQUEST", decoded.Error.Code)
	assert.Equal(t, "The request is invalid", decoded.Error.Message)
	assert.Equal(t, "Missing required field: prompt", decoded.Error.Details)
}

func TestAPIError_JSON(t *testing.T) {
	tests := []struct {
		name string
		err  APIError
	}{
		{
			name: "with details",
			err: APIError{
				Code:    "AUTH_ERROR",
				Message: "Authentication failed",
				Details: "Invalid API key",
			},
		},
		{
			name: "without details",
			err: APIError{
				Code:    "NOT_FOUND",
				Message: "Resource not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.err)
			require.NoError(t, err)

			var decoded APIError
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.err.Code, decoded.Code)
			assert.Equal(t, tt.err.Message, decoded.Message)
			assert.Equal(t, tt.err.Details, decoded.Details)
		})
	}
}

func TestGenerateResponse_JSON(t *testing.T) {
	resp := GenerateResponse{
		Content:  "Generated content here",
		Model:    "gpt-4",
		Provider: "openai",
		Usage: &UsageInfo{
			PromptTokens:     50,
			CompletionTokens: 100,
			TotalTokens:      150,
		},
		Metadata: map[string]interface{}{
			"finish_reason": "stop",
			"latency_ms":    250,
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded GenerateResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Content, decoded.Content)
	assert.Equal(t, resp.Model, decoded.Model)
	assert.Equal(t, resp.Provider, decoded.Provider)
	require.NotNil(t, decoded.Usage)
	assert.Equal(t, 50, decoded.Usage.PromptTokens)
	assert.Equal(t, 100, decoded.Usage.CompletionTokens)
	assert.Equal(t, 150, decoded.Usage.TotalTokens)
	assert.NotNil(t, decoded.Metadata)
}

func TestGenerateResponse_MinimalJSON(t *testing.T) {
	resp := GenerateResponse{
		Content:  "Simple response",
		Model:    "claude-3",
		Provider: "anthropic",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded GenerateResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Content, decoded.Content)
	assert.Equal(t, resp.Model, decoded.Model)
	assert.Equal(t, resp.Provider, decoded.Provider)
	assert.Nil(t, decoded.Usage)
	assert.Nil(t, decoded.Metadata)
}

func TestUsageInfo_JSON(t *testing.T) {
	usage := UsageInfo{
		PromptTokens:     123,
		CompletionTokens: 456,
		TotalTokens:      579,
	}

	data, err := json.Marshal(usage)
	require.NoError(t, err)

	var decoded UsageInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, usage, decoded)
}

func TestProviderInfo_JSON(t *testing.T) {
	info := ProviderInfo{
		Name:        "openai",
		Type:        "openai",
		Enabled:     true,
		Healthy:     true,
		Models:      []string{"gpt-4", "gpt-3.5-turbo"},
		Description: "OpenAI provider",
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded ProviderInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.Name, decoded.Name)
	assert.Equal(t, info.Type, decoded.Type)
	assert.Equal(t, info.Enabled, decoded.Enabled)
	assert.Equal(t, info.Healthy, decoded.Healthy)
	assert.Equal(t, info.Models, decoded.Models)
	assert.Equal(t, info.Description, decoded.Description)
}

func TestHealthResponse_JSON(t *testing.T) {
	resp := HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
		Uptime:  "2h30m15s",
		Providers: map[string]ProviderHealth{
			"openai": {
				Status:  "healthy",
				Latency: 120,
				Message: "OK",
			},
			"anthropic": {
				Status:  "degraded",
				Latency: 500,
				Message: "High latency",
			},
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded HealthResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Status, decoded.Status)
	assert.Equal(t, resp.Version, decoded.Version)
	assert.Equal(t, resp.Uptime, decoded.Uptime)
	assert.Len(t, decoded.Providers, 2)
	assert.Equal(t, "healthy", decoded.Providers["openai"].Status)
	assert.Equal(t, int64(120), decoded.Providers["openai"].Latency)
	assert.Equal(t, "degraded", decoded.Providers["anthropic"].Status)
}

func TestHealthResponse_MinimalJSON(t *testing.T) {
	resp := HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
		Uptime:  "1m30s",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded HealthResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Status, decoded.Status)
	assert.Equal(t, resp.Version, decoded.Version)
	assert.Equal(t, resp.Uptime, decoded.Uptime)
	assert.Nil(t, decoded.Providers)
}

func TestProviderHealth_JSON(t *testing.T) {
	tests := []struct {
		name   string
		health ProviderHealth
	}{
		{
			name: "healthy",
			health: ProviderHealth{
				Status:  "healthy",
				Latency: 100,
				Message: "All systems operational",
			},
		},
		{
			name: "unhealthy",
			health: ProviderHealth{
				Status:  "unhealthy",
				Latency: 0,
				Message: "Connection timeout",
			},
		},
		{
			name: "no message",
			health: ProviderHealth{
				Status:  "healthy",
				Latency: 50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.health)
			require.NoError(t, err)

			var decoded ProviderHealth
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.health.Status, decoded.Status)
			assert.Equal(t, tt.health.Latency, decoded.Latency)
			assert.Equal(t, tt.health.Message, decoded.Message)
		})
	}
}

// ==================================================
// Integration/Edge Case Tests
// ==================================================

func TestBackendConfig_EmptyConfig(t *testing.T) {
	cfg := BackendConfig{}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	var decoded BackendConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// YAML unmarshalling creates empty slices/maps instead of nil
	assert.Empty(t, decoded.Server.Host)
	assert.Zero(t, decoded.Server.Port)
	assert.False(t, decoded.Auth.Enabled)
	assert.False(t, decoded.CORS.Enabled)
}

func TestGenerateRequest_StreamingJSON(t *testing.T) {
	req := GenerateRequest{
		Provider:    "openai",
		Model:       "gpt-4",
		Prompt:      "test",
		Stream:      true,
		MaxTokens:   500,
		Temperature: 1.0,
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded GenerateRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Stream)
	assert.Equal(t, req.Provider, decoded.Provider)
}

func TestAPIResponse_NilError(t *testing.T) {
	resp := APIResponse{
		Success:   true,
		Data:      "test data",
		Error:     nil,
		RequestID: "req-789",
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded APIResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Success)
	assert.Nil(t, decoded.Error)
}

func TestGenerateResponse_WithoutUsage(t *testing.T) {
	resp := GenerateResponse{
		Content:  "Response without usage info",
		Model:    "test-model",
		Provider: "test-provider",
		Usage:    nil,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded GenerateResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Content, decoded.Content)
	assert.Nil(t, decoded.Usage)
}

func TestServerConfig_ZeroValues(t *testing.T) {
	cfg := ServerConfig{
		Host: "localhost",
		Port: 0, // Zero value
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	var decoded ServerConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "localhost", decoded.Host)
	assert.Equal(t, 0, decoded.Port)
}

func TestCORSConfig_Disabled(t *testing.T) {
	cfg := CORSConfig{
		Enabled: false,
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	var decoded CORSConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.False(t, decoded.Enabled)
}

func TestExtensionConfig_EmptyConfig(t *testing.T) {
	cfg := ExtensionConfig{
		Enabled: true,
		Config:  map[string]interface{}{},
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	var decoded ExtensionConfig
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Enabled)
	assert.NotNil(t, decoded.Config)
}

// ==================================================
// Benchmarks
// ==================================================

func BenchmarkBackendConfig_YAML_Marshal(b *testing.B) {
	cfg := BackendConfig{
		Server: ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Auth: AuthConfig{
			Enabled: true,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = yaml.Marshal(cfg)
	}
}

func BenchmarkGenerateRequest_JSON_Marshal(b *testing.B) {
	req := GenerateRequest{
		Provider:    "openai",
		Model:       "gpt-4",
		Prompt:      "test prompt",
		MaxTokens:   100,
		Temperature: 0.7,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(req)
	}
}

func BenchmarkAPIResponse_JSON_Marshal(b *testing.B) {
	resp := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": "test",
		},
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(resp)
	}
}
