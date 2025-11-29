package extensions

import (
	"context"
	"net/http"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// GenerateRequest is a local type until backendtypes is ready
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

// Extension defines the interface for backend extensions
type Extension interface {
	Name() string
	Version() string
	Description() string
	Dependencies() []string

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

func (b *BaseExtension) Dependencies() []string                                                      { return nil }
func (b *BaseExtension) RegisterRoutes(r RouteRegistrar) error                                       { return nil }
func (b *BaseExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error              { return nil }
func (b *BaseExtension) AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error {
	return nil
}
func (b *BaseExtension) OnProviderError(ctx context.Context, provider types.Provider, err error) error {
	return nil
}
func (b *BaseExtension) OnProviderSelected(ctx context.Context, provider types.Provider) error { return nil }
func (b *BaseExtension) Shutdown(ctx context.Context) error                                    { return nil }
