package types

import (
	"context"
	"time"
)

// ProviderType represents the type of AI provider
type ProviderType string

const (
	ProviderTypeOpenAI      ProviderType = "openai"
	ProviderTypeAzureOpenAI ProviderType = "azure-openai"
	ProviderTypeAnthropic   ProviderType = "anthropic"
	ProviderTypeGemini      ProviderType = "gemini"
	ProviderTypeQwen        ProviderType = "qwen"
	ProviderTypeCerebras    ProviderType = "cerebras"
	ProviderTypeOpenRouter  ProviderType = "openrouter"
	ProviderTypeCopilot     ProviderType = "copilot"
	ProviderTypeSynthetic   ProviderType = "synthetic"
	ProviderTypexAI         ProviderType = "xai"
	ProviderTypeFireworks   ProviderType = "fireworks"
	ProviderTypeDeepseek    ProviderType = "deepseek"
	ProviderTypeMistral     ProviderType = "mistral"
	ProviderTypeLMStudio    ProviderType = "lmstudio"
	ProviderTypeLlamaCpp    ProviderType = "llamacpp"
	ProviderTypeOllama      ProviderType = "ollama"

	// Virtual providers
	ProviderTypeRacing      ProviderType = "racing"
	ProviderTypeFallback    ProviderType = "fallback"
	ProviderTypeLoadBalance ProviderType = "loadbalance"
)

// AuthMethod represents the authentication method
type AuthMethod string

const (
	AuthMethodAPIKey      AuthMethod = "api_key"
	AuthMethodBearerToken AuthMethod = "bearer_token"
	AuthMethodOAuth       AuthMethod = "oauth"
	AuthMethodCustom      AuthMethod = "custom"
)

// ToolFormat represents the format used for tool calling
type ToolFormat string

const (
	ToolFormatOpenAI    ToolFormat = "openai"
	ToolFormatAnthropic ToolFormat = "anthropic"
	ToolFormatGemini    ToolFormat = "gemini"
	ToolFormatXML       ToolFormat = "xml"
	ToolFormatHermes    ToolFormat = "hermes"
	ToolFormatText      ToolFormat = "text"
)

// HealthStatus represents the health status of a provider
type HealthStatus struct {
	Healthy      bool      `json:"healthy"`
	LastChecked  time.Time `json:"last_checked"`
	Message      string    `json:"message"`
	ResponseTime float64   `json:"response_time"`
	StatusCode   int       `json:"status_code"`
}

// ProviderMetrics represents metrics for a provider
type ProviderMetrics struct {
	RequestCount    int64         `json:"request_count"`
	SuccessCount    int64         `json:"success_count"`
	ErrorCount      int64         `json:"error_count"`
	TotalLatency    time.Duration `json:"total_latency"`
	AverageLatency  time.Duration `json:"average_latency"`
	LastRequestTime time.Time     `json:"last_request_time"`
	LastSuccessTime time.Time     `json:"last_success_time"`
	LastErrorTime   time.Time     `json:"last_error_time"`
	LastError       string        `json:"last_error"`
	TokensUsed      int64         `json:"tokens_used"`
	HealthStatus    HealthStatus  `json:"health_status"`
}

// ProviderInfo contains information about a provider
type ProviderInfo struct {
	Name           string       `json:"name"`
	Type           ProviderType `json:"type"`
	Description    string       `json:"description"`
	HealthStatus   HealthStatus `json:"health_status"`
	Models         []Model      `json:"models"`
	SupportedTools []string     `json:"supported_tools"`
	DefaultModel   string       `json:"default_model"`
}

// ModelCapabilityOverride allows users to override model capabilities
type ModelCapabilityOverride struct {
	MaxTokens         *int     `json:"max_tokens,omitempty"`
	ContextWindow     *int     `json:"context_window,omitempty"`
	SupportsStreaming *bool    `json:"supports_streaming,omitempty"`
	SupportsTools     *bool    `json:"supports_tools,omitempty"`
	SupportsVision    *bool    `json:"supports_vision,omitempty"`
	Capabilities      []string `json:"capabilities,omitempty"`
}

// ProviderConfig represents configuration for a specific provider
type ProviderConfig struct {
	Type           ProviderType           `json:"type"`
	Name           string                 `json:"name"`
	BaseURL        string                 `json:"base_url,omitempty"`
	APIKey         string                 `json:"api_key,omitempty"`
	APIKeyEnv      string                 `json:"api_key_env,omitempty"`
	DefaultModel   string                 `json:"default_model,omitempty"`
	Description    string                 `json:"description,omitempty"`
	ProviderConfig map[string]interface{} `json:"provider_config,omitempty"`

	// Multiple OAuth credentials for failover (multi-OAuth)
	OAuthCredentials []*OAuthCredentialSet `json:"oauth_credentials,omitempty"`

	// Model capability overrides - allows users to override model capabilities
	ModelCapabilities map[string]ModelCapabilityOverride `json:"model_capabilities,omitempty"`

	// Feature flags
	SupportsStreaming    bool `json:"supports_streaming"`
	SupportsToolCalling  bool `json:"supports_tool_calling"`
	SupportsResponsesAPI bool `json:"supports_responses_api"`

	// Limits and timeouts
	MaxTokens int           `json:"max_tokens,omitempty"`
	Timeout   time.Duration `json:"timeout,omitempty"`
	// ResponseHeaderTimeout is the timeout for waiting for the first response header
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout,omitempty"`

	// Tool format
	ToolFormat ToolFormat `json:"tool_format,omitempty"`

	// Logging configuration
	EnableVerboseLogging bool `json:"enable_verbose_logging,omitempty"`
}

// OAuthConfig represents OAuth configuration
type OAuthConfig struct {
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	RedirectURL  string    `json:"redirect_url,omitempty"`
	Scopes       []string  `json:"scopes"`
	AuthURL      string    `json:"auth_url,omitempty"`
	TokenURL     string    `json:"token_url,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`

	// OnTokenRefresh is called when OAuth tokens are refreshed
	// Parameters: accessToken, refreshToken, expiresAt
	// The callback should return an error if token persistence fails
	OnTokenRefresh func(accessToken, refreshToken string, expiresAt time.Time) error `json:"-"`
}

// OAuthCredentialSet represents a single set of OAuth credentials for multi-OAuth support
// This is used by the oauthmanager package for managing multiple OAuth credentials
type OAuthCredentialSet struct {
	// Unique identifier for this credential set (e.g., "account-1", "team-account")
	ID string `json:"id"`

	// OAuth client credentials
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`

	// OAuth tokens
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`

	// OAuth scopes for this credential
	Scopes []string `json:"scopes,omitempty"`

	// Metadata for tracking
	LastRefresh  time.Time `json:"last_refresh,omitempty"`
	RefreshCount int       `json:"refresh_count"`

	// Callback for when tokens are refreshed
	// Parameters: id, accessToken, refreshToken, expiresAt
	// The callback should persist the new tokens and return an error if persistence fails
	OnTokenRefresh func(id, accessToken, refreshToken string, expiresAt time.Time) error `json:"-"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Method       AuthMethod `json:"method"`
	APIKey       string     `json:"api_key,omitempty"`
	BaseURL      string     `json:"base_url,omitempty"`
	DefaultModel string     `json:"default_model,omitempty"`
}

// TokenStorage represents a token storage interface
type TokenStorage interface {
	StoreToken(key string, token *OAuthConfig) error
	RetrieveToken(key string) (*OAuthConfig, error)
	DeleteToken(key string) error
	ListTokens() ([]string, error)
}

// ============================================================================
// Interface Segregation - Focused Provider Interfaces
// ============================================================================

// CoreProvider defines the essential identity methods that all providers must implement.
// This interface provides basic information about the provider.
type CoreProvider interface {
	// Basic provider information
	Name() string
	Type() ProviderType
	Description() string
}

// ModelProvider defines methods for model discovery and management.
// This interface is for providers that expose multiple models.
type ModelProvider interface {
	// Model management
	GetModels(ctx context.Context) ([]Model, error)
	GetDefaultModel() string
}

// AuthenticatedProvider defines methods for authentication management.
// This interface is for providers that require authentication.
type AuthenticatedProvider interface {
	// Authentication
	Authenticate(ctx context.Context, authConfig AuthConfig) error
	IsAuthenticated() bool
	Logout(ctx context.Context) error
}

// ConfigurableProvider defines methods for configuration management.
// This interface is for providers that support runtime configuration.
type ConfigurableProvider interface {
	// Configuration
	Configure(config ProviderConfig) error
	GetConfig() ProviderConfig
}

// ChatProvider defines the core chat completion capability.
// This interface is for providers that can generate text responses.
type ChatProvider interface {
	// Core chat capability
	GenerateChatCompletion(ctx context.Context, options GenerateOptions) (ChatCompletionStream, error)
}

// ToolCallingProvider defines methods for tool/function calling capabilities.
// This interface is for providers that support external tool invocation.
type ToolCallingProvider interface {
	// Tool invocation
	InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error)

	// Tool format support
	SupportsToolCalling() bool
	GetToolFormat() ToolFormat
}

// CapabilityProvider provides methods to query provider capabilities.
// This interface is for providers that expose their feature support.
type CapabilityProvider interface {
	// Streaming support
	SupportsStreaming() bool

	// Responses API support
	SupportsResponsesAPI() bool
}

// HealthCheckProvider defines methods for health monitoring and metrics.
// This interface is for providers that expose health and performance information.
type HealthCheckProvider interface {
	// Health and metrics
	HealthCheck(ctx context.Context) error
	GetMetrics() ProviderMetrics
}

// AuthMethodDetector defines methods for detecting configured authentication methods.
// This optional interface is for providers that support multiple authentication methods
// (e.g., both OAuth and API key) and need to expose which methods are currently configured.
type AuthMethodDetector interface {
	// IsOAuthConfigured returns true if OAuth credentials are properly configured
	IsOAuthConfigured() bool

	// IsAPIKeyConfigured returns true if API key authentication is properly configured
	IsAPIKeyConfigured() bool
}

// QuotaProvider defines methods for quota management and reporting.
// This optional interface is for providers that support quota information reporting
// and management. Providers that implement this interface can provide real-time
// quota information and historical usage data.
//
// The QuotaProvider interface provides a provider-agnostic way to query and manage
// quotas across different AI providers. It allows applications to:
// - Check current quota status before making requests
// - Get historical usage patterns for analysis
// - Implement intelligent request routing based on quota availability
// - Set up quotas and limits where supported by the provider
//
// This interface integrates with the quota package which provides:
// - A unified quota management system
// - Integration with existing rate limit tracking
// - Historical usage tracking
// - Provider-agnostic quota APIs
type QuotaProvider interface {
	// GetQuotaInfo returns the current quota information for the provider
	// This method should fetch real-time quota information from the provider's API
	// or return cached quota information if real-time data is not available.
	//
	// Parameters:
	//   - ctx: Context for the request (can include auth credentials)
	//   - model: The model identifier (empty for provider-wide quota)
	//
	// Returns:
	//   - QuotaInfo: Current quota information including limits, usage, and reset times
	//   - error: Error if quota information cannot be retrieved
	GetQuotaInfo(ctx context.Context, model string) (*QuotaInfo, error)

	// GetQuotaHistory returns historical quota usage for the provider
	// This allows applications to analyze usage patterns over time and
	// implement quota-aware features like tiered access or smart routing.
	//
	// Parameters:
	//   - ctx: Context for the request
	//   - model: The model identifier (empty for all models)
	//   - startTime: Start of the time range for historical data
	//   - endTime: End of the time range for historical data
	//
	// Returns:
	//   - QuotaHistory: Historical quota usage records and aggregates
	//   - error: Error if history cannot be retrieved
	GetQuotaHistory(ctx context.Context, model string, startTime, endTime time.Time) (*QuotaHistory, error)

	// SupportsQuotaReporting returns true if the provider supports real-time quota reporting
	// through the GetQuotaInfo method. If this returns false, quota information is only
	// updated through rate limit headers after requests are made.
	SupportsQuotaReporting() bool

	// SupportsQuotaHistory returns true if the provider supports retrieving historical
	// quota usage data through the GetQuotaHistory method.
	SupportsQuotaHistory() bool
}

// QuotaInfo represents quota information for a provider or specific model.
// This type is defined in the quota package but re-exported here for convenience.
// See github.com/cecil-the-coder/ai-provider-kit/pkg/quota for the full definition.
type QuotaInfo struct {
	Provider             string                     `json:"provider"`
	ProviderType         ProviderType               `json:"provider_type"`
	Model                string                     `json:"model"`
	Timestamp            time.Time                  `json:"timestamp"`
	Quotas               map[QuotaType]*QuotaUsage  `json:"quotas"`
	ProviderQuotaConfigs map[QuotaType]*QuotaConfig `json:"provider_quota_configs,omitempty"`
	CustomUsage          map[string]interface{}     `json:"custom_usage,omitempty"`
	Metadata             map[string]interface{}     `json:"metadata,omitempty"`
}

// QuotaType represents the type of quota being measured.
// Common types include requests, tokens, daily limits, and custom provider-specific quotas.
type QuotaType string

const (
	// QuotaTypeRequests represents request-based quotas
	QuotaTypeRequests QuotaType = "requests"
	// QuotaTypeTokens represents token-based quotas (total)
	QuotaTypeTokens QuotaType = "tokens"
	// QuotaTypeInputTokens represents input token quotas
	QuotaTypeInputTokens QuotaType = "input_tokens"
	// QuotaTypeOutputTokens represents output token quotas
	QuotaTypeOutputTokens QuotaType = "output_tokens"
	// QuotaTypeDaily represents daily quotas
	QuotaTypeDaily QuotaType = "daily"
	// QuotaTypeCustom represents provider-specific custom quotas
	QuotaTypeCustom QuotaType = "custom"
)

// QuotaPeriod represents the time period for a quota limit.
type QuotaPeriod string

const (
	// QuotaPeriodMinute represents a per-minute quota
	QuotaPeriodMinute QuotaPeriod = "minute"
	// QuotaPeriodHour represents an hourly quota
	QuotaPeriodHour QuotaPeriod = "hour"
	// QuotaPeriodDay represents a daily quota
	QuotaPeriodDay QuotaPeriod = "day"
	// QuotaPeriodWeek represents a weekly quota
	QuotaPeriodWeek QuotaPeriod = "week"
	// QuotaPeriodMonth represents a monthly quota
	QuotaPeriodMonth QuotaPeriod = "month"
	// QuotaPeriodCustom represents a custom time period
	QuotaPeriodCustom QuotaPeriod = "custom"
)

// QuotaUsage represents current usage for a specific quota.
type QuotaUsage struct {
	Type             QuotaType   `json:"type"`
	Period           QuotaPeriod `json:"period"`
	Used             int         `json:"used"`
	Limit            int         `json:"limit"`
	Remaining        int         `json:"remaining"`
	RemainingPercent float64     `json:"remaining_percent"`
	ResetAt          time.Time   `json:"reset_at"`
	PeriodStartedAt  time.Time   `json:"period_started_at"`
}

// QuotaConfig represents a quota configuration for setting limits.
type QuotaConfig struct {
	Type       QuotaType              `json:"type"`
	Period     QuotaPeriod            `json:"period"`
	Limit      int                    `json:"limit"`
	ResetAt    time.Time              `json:"reset_at"`
	CustomData map[string]interface{} `json:"custom_data,omitempty"`
}

// QuotaHistory represents historical quota usage data.
type QuotaHistory struct {
	Provider   string            `json:"provider"`
	Model      string            `json:"model,omitempty"`
	Records    []*QuotaRecord    `json:"records"`
	TotalUsage map[QuotaType]int `json:"total_usage"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
}

// QuotaRecord represents a single quota usage event record.
type QuotaRecord struct {
	ID        string            `json:"id"`
	Provider  string            `json:"provider"`
	Model     string            `json:"model"`
	Timestamp time.Time         `json:"timestamp"`
	Operation string            `json:"operation"`
	Usage     map[QuotaType]int `json:"usage"`
}

// ============================================================================
// Composite Provider Interface
// ============================================================================

// Provider represents a complete AI provider with all capabilities.
// This interface composes all the smaller interfaces for backward compatibility.
//
// When to use smaller interfaces:
// - Use CoreProvider when you only need basic provider information
// - Use ModelProvider when you only need to list or select models
// - Use AuthenticatedProvider when you only need authentication management
// - Use ConfigurableProvider when you only need to configure the provider
// - Use ChatProvider when you only need basic chat completion
// - Use ToolCallingProvider when you only need tool/function calling
// - Use CapabilityProvider when you only need to check provider capabilities
// - Use HealthCheckProvider when you only need health monitoring
//
// This approach follows the Interface Segregation Principle, allowing clients
// to depend only on the methods they actually use.
type Provider interface {
	CoreProvider
	ModelProvider
	AuthenticatedProvider
	ConfigurableProvider
	ChatProvider
	ToolCallingProvider
	CapabilityProvider
	HealthCheckProvider
}

// ProviderFactory represents a factory for creating providers
type ProviderFactory interface {
	RegisterProvider(providerType ProviderType, factoryFunc func(ProviderConfig) Provider)
	CreateProvider(providerType ProviderType, config ProviderConfig) (Provider, error)
	GetSupportedProviders() []ProviderType
}
