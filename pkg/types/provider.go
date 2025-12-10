package types

import (
	"context"
	"io"
	"net/http"
	"time"
)

// ProviderType represents the type of AI provider
type ProviderType string

const (
	ProviderTypeOpenAI     ProviderType = "openai"
	ProviderTypeAnthropic  ProviderType = "anthropic"
	ProviderTypeGemini     ProviderType = "gemini"
	ProviderTypeQwen       ProviderType = "qwen"
	ProviderTypeCerebras   ProviderType = "cerebras"
	ProviderTypeOpenRouter ProviderType = "openrouter"
	ProviderTypeSynthetic  ProviderType = "synthetic"
	ProviderTypexAI        ProviderType = "xai"
	ProviderTypeFireworks  ProviderType = "fireworks"
	ProviderTypeDeepseek   ProviderType = "deepseek"
	ProviderTypeMistral    ProviderType = "mistral"
	ProviderTypeLMStudio   ProviderType = "lmstudio"
	ProviderTypeLlamaCpp   ProviderType = "llamacpp"
	ProviderTypeOllama     ProviderType = "ollama"

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

// Options represents configuration options for a provider
type Options interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
	GetDuration(key string) time.Duration
	GetStringSlice(key string) []string
}

// Router represents a router for provider selection
type Router interface {
	SelectProvider(prompt string, options interface{}) (Provider, error)
	GetAvailableProviders() []ProviderInfo
	GetProvider(name string) (Provider, error)
	SetPreference(providerName string) error
}

// ProviderRegistry represents a registry of providers
type ProviderRegistry interface {
	Register(provider Provider) error
	Unregister(name string) error
	Get(name string) (Provider, error)
	List() []Provider
	ListByType(providerType ProviderType) []Provider
	GetAvailable() []Provider
	GetHealthy() []Provider
}

// Logger represents a logger interface
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
}

// HTTPClient represents an HTTP client interface
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
	Get(url string) (*http.Response, error)
	Post(url string, bodyType string, body io.Reader) (*http.Response, error)
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

// StreamingProvider defines streaming capabilities.
// This interface is for providers that support real-time streaming responses.
type StreamingProvider interface {
	// Streaming support
	SupportsStreaming() bool
}

// ResponsesAPIProvider defines support for structured responses API.
// This interface is for providers that support structured response formats.
type ResponsesAPIProvider interface {
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
// - Use StreamingProvider when you only need to check streaming support
// - Use ResponsesAPIProvider when you only need responses API support
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
	StreamingProvider
	ResponsesAPIProvider
	HealthCheckProvider
}

// ProviderFactory represents a factory for creating providers
type ProviderFactory interface {
	RegisterProvider(providerType ProviderType, factoryFunc func(ProviderConfig) Provider)
	CreateProvider(providerType ProviderType, config ProviderConfig) (Provider, error)
	GetSupportedProviders() []ProviderType
}
