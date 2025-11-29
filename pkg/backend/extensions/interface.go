package extensions

import (
	"context"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// GenerateRequest is a local type until backendtypes is ready.
//
// Metadata can contain "extension_config" key with per-extension settings:
//
//	{
//	  "extension_config": {
//	    "caching": {"enabled": false},
//	    "logging": {"enabled": true, "level": "debug"},
//	    "metrics": {"interval": 60}
//	  }
//	}
//
// Extensions can use GetExtensionConfig() or IsExtensionEnabled() to check their configuration.
// By default, extensions are enabled unless explicitly disabled via {"enabled": false}.
type GenerateRequest struct {
	Provider    string                 `json:"provider,omitempty"`
	Model       string                 `json:"model,omitempty"`
	Prompt      string                 `json:"prompt"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// GenerateResponse is a local type until backendtypes is ready
type GenerateResponse struct {
	Content  string                 `json:"content"`
	Model    string                 `json:"model"`
	Provider string                 `json:"provider"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ExtensionConfig is a local type until backendtypes is ready
type ExtensionConfig struct {
	Enabled bool                   `yaml:"enabled"`
	Config  map[string]interface{} `yaml:"config"`
}

// Priority constants define the execution order for extensions.
// Lower priority values execute first.
// Priority ranges:
//   - 0-199: Critical/Security extensions (e.g., authentication, rate limiting)
//   - 200-399: Caching and data layer extensions
//   - 400-599: Transform and business logic extensions
//   - 600-799: Monitoring and metrics extensions
//   - 800-999: Logging and auditing extensions
const (
	PrioritySecurity  = 100 // Security-critical extensions (auth, rate limiting)
	PriorityCache     = 200 // Caching extensions
	PriorityTransform = 500 // Transform and business logic (default)
	PriorityLogging   = 900 // Logging and auditing extensions
)

// Extension defines the interface for backend extensions
type Extension interface {
	Name() string
	Version() string
	Description() string
	Dependencies() []string
	Priority() int

	Initialize(config map[string]interface{}) error
	Shutdown(ctx context.Context) error

	RegisterRoutes(registrar RouteRegistrar) error

	BeforeGenerate(ctx context.Context, req *GenerateRequest) error
	AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error

	OnProviderError(ctx context.Context, provider types.Provider, err error) error
	OnProviderSelected(ctx context.Context, provider types.Provider) error
}

// RouteRegistrar allows extensions to register custom routes
type RouteRegistrar interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler http.HandlerFunc)
}

// ExtensionRegistry manages extension lifecycle
type ExtensionRegistry interface {
	Register(ext Extension) error
	Get(name string) (Extension, bool)
	List() []Extension
	Initialize(configs map[string]ExtensionConfig) error
	Shutdown(ctx context.Context) error
}

// BaseExtension provides default implementations for optional methods
type BaseExtension struct{}

func (b *BaseExtension) Dependencies() []string                                         { return nil }
func (b *BaseExtension) Priority() int                                                  { return PriorityTransform }
func (b *BaseExtension) RegisterRoutes(r RouteRegistrar) error                          { return nil }
func (b *BaseExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error { return nil }
func (b *BaseExtension) AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error {
	return nil
}
func (b *BaseExtension) OnProviderError(ctx context.Context, provider types.Provider, err error) error {
	return nil
}
func (b *BaseExtension) OnProviderSelected(ctx context.Context, provider types.Provider) error {
	return nil
}
func (b *BaseExtension) Shutdown(ctx context.Context) error { return nil }
