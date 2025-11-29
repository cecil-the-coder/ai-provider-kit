package extensions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockExtension implements the Extension interface for testing
type mockExtension struct {
	BaseExtension
	name         string
	version      string
	description  string
	deps         []string
	initialized  bool
	shutdown     bool
	initError    error
	shutdownErr  error
	beforeGenErr error
	afterGenErr  error
}

func (m *mockExtension) Name() string        { return m.name }
func (m *mockExtension) Version() string     { return m.version }
func (m *mockExtension) Description() string { return m.description }

func (m *mockExtension) Dependencies() []string {
	if m.deps != nil {
		return m.deps
	}
	return m.BaseExtension.Dependencies()
}

func (m *mockExtension) Initialize(config map[string]interface{}) error {
	if m.initError != nil {
		return m.initError
	}
	m.initialized = true
	return nil
}

func (m *mockExtension) Shutdown(ctx context.Context) error {
	if m.shutdownErr != nil {
		return m.shutdownErr
	}
	m.shutdown = true
	return nil
}

func (m *mockExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
	if m.beforeGenErr != nil {
		return m.beforeGenErr
	}
	return m.BaseExtension.BeforeGenerate(ctx, req)
}

func (m *mockExtension) AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error {
	if m.afterGenErr != nil {
		return m.afterGenErr
	}
	return m.BaseExtension.AfterGenerate(ctx, req, resp)
}

// mockRouteRegistrar implements RouteRegistrar for testing
type mockRouteRegistrar struct {
	patterns []string
	handlers []http.Handler
}

func (m *mockRouteRegistrar) Handle(pattern string, handler http.Handler) {
	m.patterns = append(m.patterns, pattern)
	m.handlers = append(m.handlers, handler)
}

func (m *mockRouteRegistrar) HandleFunc(pattern string, handler http.HandlerFunc) {
	m.patterns = append(m.patterns, pattern)
	m.handlers = append(m.handlers, handler)
}

// mockProvider implements types.Provider for testing
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string             { return m.name }
func (m *mockProvider) Type() types.ProviderType { return "mock" }
func (m *mockProvider) Description() string      { return "Mock Provider" }
func (m *mockProvider) GetDefaultModel() string  { return "mock-model" }
func (m *mockProvider) GetModels(ctx context.Context) ([]types.Model, error) {
	return []types.Model{}, nil
}
func (m *mockProvider) IsAuthenticated() bool { return true }
func (m *mockProvider) Authenticate(ctx context.Context, config types.AuthConfig) error {
	return nil
}
func (m *mockProvider) Logout(ctx context.Context) error { return nil }
func (m *mockProvider) Configure(config types.ProviderConfig) error {
	return nil
}
func (m *mockProvider) GetConfig() types.ProviderConfig { return types.ProviderConfig{} }
func (m *mockProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
	return nil, nil
}
func (m *mockProvider) InvokeServerTool(ctx context.Context, toolName string, params interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockProvider) SupportsToolCalling() bool       { return false }
func (m *mockProvider) GetToolFormat() types.ToolFormat { return types.ToolFormatOpenAI }
func (m *mockProvider) SupportsStreaming() bool         { return false }
func (m *mockProvider) SupportsResponsesAPI() bool      { return false }
func (m *mockProvider) HealthCheck(ctx context.Context) error {
	return nil
}
func (m *mockProvider) GetMetrics() types.ProviderMetrics { return types.ProviderMetrics{} }

// TestBaseExtension_DefaultImplementations tests that all BaseExtension default methods return nil/empty
func TestBaseExtension_DefaultImplementations(t *testing.T) {
	base := &BaseExtension{}
	ctx := context.Background()

	t.Run("Dependencies returns nil", func(t *testing.T) {
		deps := base.Dependencies()
		assert.Nil(t, deps)
	})

	t.Run("RegisterRoutes returns nil", func(t *testing.T) {
		registrar := &mockRouteRegistrar{}
		err := base.RegisterRoutes(registrar)
		assert.NoError(t, err)
	})

	t.Run("BeforeGenerate returns nil", func(t *testing.T) {
		req := &GenerateRequest{Prompt: "test"}
		err := base.BeforeGenerate(ctx, req)
		assert.NoError(t, err)
	})

	t.Run("AfterGenerate returns nil", func(t *testing.T) {
		req := &GenerateRequest{Prompt: "test"}
		resp := &GenerateResponse{Content: "response"}
		err := base.AfterGenerate(ctx, req, resp)
		assert.NoError(t, err)
	})

	t.Run("OnProviderError returns nil", func(t *testing.T) {
		provider := &mockProvider{name: "test"}
		testErr := errors.New("test error")
		err := base.OnProviderError(ctx, provider, testErr)
		assert.NoError(t, err)
	})

	t.Run("OnProviderSelected returns nil", func(t *testing.T) {
		provider := &mockProvider{name: "test"}
		err := base.OnProviderSelected(ctx, provider)
		assert.NoError(t, err)
	})

	t.Run("Shutdown returns nil", func(t *testing.T) {
		err := base.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

// TestRegistry_NewRegistry tests registry creation
func TestRegistry_NewRegistry(t *testing.T) {
	reg := NewRegistry()
	assert.NotNil(t, reg)

	// Verify empty registry
	list := reg.List()
	assert.Empty(t, list)
}

// TestRegistry_Register tests registering extensions
func TestRegistry_Register(t *testing.T) {
	t.Run("register extension successfully", func(t *testing.T) {
		reg := NewRegistry()
		ext := &mockExtension{
			name:        "test-ext",
			version:     "1.0.0",
			description: "Test extension",
		}

		err := reg.Register(ext)
		assert.NoError(t, err)

		// Verify extension is registered
		retrieved, ok := reg.Get("test-ext")
		assert.True(t, ok)
		assert.Equal(t, ext, retrieved)
	})

	t.Run("duplicate registration returns error", func(t *testing.T) {
		reg := NewRegistry()
		ext1 := &mockExtension{name: "test-ext"}
		ext2 := &mockExtension{name: "test-ext"}

		err := reg.Register(ext1)
		assert.NoError(t, err)

		err = reg.Register(ext2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("register multiple extensions", func(t *testing.T) {
		reg := NewRegistry()
		ext1 := &mockExtension{name: "ext1"}
		ext2 := &mockExtension{name: "ext2"}
		ext3 := &mockExtension{name: "ext3"}

		assert.NoError(t, reg.Register(ext1))
		assert.NoError(t, reg.Register(ext2))
		assert.NoError(t, reg.Register(ext3))

		list := reg.List()
		assert.Len(t, list, 3)
	})
}

// TestRegistry_Get tests retrieving extensions
func TestRegistry_Get(t *testing.T) {
	t.Run("get existing extension", func(t *testing.T) {
		reg := NewRegistry()
		ext := &mockExtension{name: "test-ext"}
		_ = reg.Register(ext)

		retrieved, ok := reg.Get("test-ext")
		assert.True(t, ok)
		assert.Equal(t, ext, retrieved)
	})

	t.Run("get non-existing extension returns false", func(t *testing.T) {
		reg := NewRegistry()

		retrieved, ok := reg.Get("non-existent")
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})
}

// TestRegistry_List tests listing extensions
func TestRegistry_List(t *testing.T) {
	t.Run("list returns extensions in registration order", func(t *testing.T) {
		reg := NewRegistry()
		ext1 := &mockExtension{name: "ext1"}
		ext2 := &mockExtension{name: "ext2"}
		ext3 := &mockExtension{name: "ext3"}

		_ = reg.Register(ext1)
		_ = reg.Register(ext2)
		_ = reg.Register(ext3)

		list := reg.List()
		assert.Len(t, list, 3)
		assert.Equal(t, "ext1", list[0].Name())
		assert.Equal(t, "ext2", list[1].Name())
		assert.Equal(t, "ext3", list[2].Name())
	})

	t.Run("list returns empty slice for empty registry", func(t *testing.T) {
		reg := NewRegistry()
		list := reg.List()
		assert.Empty(t, list)
		assert.NotNil(t, list) // Should be empty slice, not nil
	})
}

// TestRegistry_Initialize tests initialization
func TestRegistry_Initialize(t *testing.T) {
	t.Run("initialize calls extensions in dependency order", func(t *testing.T) {
		reg := NewRegistry()

		// Create extensions with dependencies: ext3 -> ext2 -> ext1
		ext1 := &mockExtension{name: "ext1"}
		ext2 := &mockExtension{name: "ext2", deps: []string{"ext1"}}
		ext3 := &mockExtension{name: "ext3", deps: []string{"ext2"}}

		_ = reg.Register(ext1)
		_ = reg.Register(ext2)
		_ = reg.Register(ext3)

		configs := map[string]ExtensionConfig{
			"ext1": {Enabled: true, Config: map[string]interface{}{"key": "value1"}},
			"ext2": {Enabled: true, Config: map[string]interface{}{"key": "value2"}},
			"ext3": {Enabled: true, Config: map[string]interface{}{"key": "value3"}},
		}

		err := reg.Initialize(configs)
		assert.NoError(t, err)

		// Verify all extensions were initialized
		assert.True(t, ext1.initialized)
		assert.True(t, ext2.initialized)
		assert.True(t, ext3.initialized)
	})

	t.Run("initialize skips disabled extensions", func(t *testing.T) {
		reg := NewRegistry()
		ext1 := &mockExtension{name: "ext1"}
		ext2 := &mockExtension{name: "ext2"}

		_ = reg.Register(ext1)
		_ = reg.Register(ext2)

		configs := map[string]ExtensionConfig{
			"ext1": {Enabled: true},
			"ext2": {Enabled: false}, // Disabled
		}

		err := reg.Initialize(configs)
		assert.NoError(t, err)

		assert.True(t, ext1.initialized)
		assert.False(t, ext2.initialized) // Should not be initialized
	})

	t.Run("initialize skips extensions without config", func(t *testing.T) {
		reg := NewRegistry()
		ext1 := &mockExtension{name: "ext1"}
		ext2 := &mockExtension{name: "ext2"}

		_ = reg.Register(ext1)
		_ = reg.Register(ext2)

		configs := map[string]ExtensionConfig{
			"ext1": {Enabled: true},
			// ext2 has no config
		}

		err := reg.Initialize(configs)
		assert.NoError(t, err)

		assert.True(t, ext1.initialized)
		assert.False(t, ext2.initialized) // Should not be initialized
	})

	t.Run("initialize returns error on extension initialization failure", func(t *testing.T) {
		reg := NewRegistry()
		ext := &mockExtension{
			name:      "test-ext",
			initError: errors.New("initialization failed"),
		}

		_ = reg.Register(ext)

		configs := map[string]ExtensionConfig{
			"test-ext": {Enabled: true},
		}

		err := reg.Initialize(configs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize extension")
		assert.Contains(t, err.Error(), "initialization failed")
	})

	t.Run("initialize with empty config map", func(t *testing.T) {
		reg := NewRegistry()
		ext := &mockExtension{name: "test-ext"}
		_ = reg.Register(ext)

		err := reg.Initialize(map[string]ExtensionConfig{})
		assert.NoError(t, err)
		assert.False(t, ext.initialized) // Should not be initialized
	})
}

// TestRegistry_Shutdown tests shutdown
func TestRegistry_Shutdown(t *testing.T) {
	t.Run("shutdown calls extensions successfully", func(t *testing.T) {
		reg := NewRegistry()
		ext1 := &mockExtension{name: "ext1"}
		ext2 := &mockExtension{name: "ext2"}
		ext3 := &mockExtension{name: "ext3"}

		_ = reg.Register(ext1)
		_ = reg.Register(ext2)
		_ = reg.Register(ext3)

		ctx := context.Background()
		err := reg.Shutdown(ctx)
		assert.NoError(t, err)

		// Verify shutdown was called on all extensions
		assert.True(t, ext1.shutdown)
		assert.True(t, ext2.shutdown)
		assert.True(t, ext3.shutdown)
	})

	t.Run("shutdown returns error on extension shutdown failure", func(t *testing.T) {
		reg := NewRegistry()
		ext := &mockExtension{
			name:        "test-ext",
			shutdownErr: errors.New("shutdown failed"),
		}

		_ = reg.Register(ext)

		ctx := context.Background()
		err := reg.Shutdown(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to shutdown extension")
		assert.Contains(t, err.Error(), "shutdown failed")
	})

	t.Run("shutdown with empty registry", func(t *testing.T) {
		reg := NewRegistry()
		ctx := context.Background()
		err := reg.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

// TestRegistry_TopologicalSort tests dependency resolution
func TestRegistry_TopologicalSort(t *testing.T) {
	t.Run("topological sort handles simple dependencies", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		ext1 := &mockExtension{name: "ext1"}
		ext2 := &mockExtension{name: "ext2", deps: []string{"ext1"}}

		_ = reg.Register(ext1)
		_ = reg.Register(ext2)

		sorted, err := reg.topologicalSort()
		assert.NoError(t, err)
		assert.Len(t, sorted, 2)

		// ext1 should come before ext2
		ext1Idx := -1
		ext2Idx := -1
		for i, name := range sorted {
			if name == "ext1" {
				ext1Idx = i
			}
			if name == "ext2" {
				ext2Idx = i
			}
		}
		assert.True(t, ext1Idx < ext2Idx, "ext1 should come before ext2")
	})

	t.Run("topological sort handles complex dependencies", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		// Create a diamond dependency graph:
		//     ext4
		//    /    \
		//  ext2   ext3
		//    \    /
		//     ext1
		ext1 := &mockExtension{name: "ext1"}
		ext2 := &mockExtension{name: "ext2", deps: []string{"ext1"}}
		ext3 := &mockExtension{name: "ext3", deps: []string{"ext1"}}
		ext4 := &mockExtension{name: "ext4", deps: []string{"ext2", "ext3"}}

		_ = reg.Register(ext1)
		_ = reg.Register(ext2)
		_ = reg.Register(ext3)
		_ = reg.Register(ext4)

		sorted, err := reg.topologicalSort()
		assert.NoError(t, err)
		assert.Len(t, sorted, 4)

		// ext1 should come before ext2, ext3, ext4
		// ext2 and ext3 should come before ext4
		positions := make(map[string]int)
		for i, name := range sorted {
			positions[name] = i
		}

		assert.True(t, positions["ext1"] < positions["ext2"])
		assert.True(t, positions["ext1"] < positions["ext3"])
		assert.True(t, positions["ext1"] < positions["ext4"])
		assert.True(t, positions["ext2"] < positions["ext4"])
		assert.True(t, positions["ext3"] < positions["ext4"])
	})

	t.Run("topological sort handles missing dependencies", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		// ext2 depends on ext1, but ext1 is not registered
		ext2 := &mockExtension{name: "ext2", deps: []string{"ext1"}}

		_ = reg.Register(ext2)

		sorted, err := reg.topologicalSort()
		assert.NoError(t, err) // Missing dependencies are ignored
		assert.Len(t, sorted, 1)
		assert.Equal(t, "ext2", sorted[0])
	})

	t.Run("topological sort with no dependencies", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		ext1 := &mockExtension{name: "ext1"}
		ext2 := &mockExtension{name: "ext2"}
		ext3 := &mockExtension{name: "ext3"}

		_ = reg.Register(ext1)
		_ = reg.Register(ext2)
		_ = reg.Register(ext3)

		sorted, err := reg.topologicalSort()
		assert.NoError(t, err)
		assert.Len(t, sorted, 3)

		// All extensions should be in the result (order doesn't matter without dependencies)
		names := make(map[string]bool)
		for _, name := range sorted {
			names[name] = true
		}
		assert.True(t, names["ext1"])
		assert.True(t, names["ext2"])
		assert.True(t, names["ext3"])
	})

	t.Run("topological sort with empty registry", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		sorted, err := reg.topologicalSort()
		assert.NoError(t, err)
		assert.Empty(t, sorted)
	})
}

// TestRegistry_ConcurrentAccess tests thread safety
func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent register and get operations", func(t *testing.T) {
		reg := NewRegistry()
		var wg sync.WaitGroup
		numGoroutines := 50

		// Register extensions concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				ext := &mockExtension{
					name:        fmt.Sprintf("ext-%d", id),
					version:     "1.0.0",
					description: fmt.Sprintf("Extension %d", id),
				}
				err := reg.Register(ext)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Verify all extensions were registered
		list := reg.List()
		assert.Len(t, list, numGoroutines)

		// Concurrent get operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				ext, ok := reg.Get(fmt.Sprintf("ext-%d", id))
				assert.True(t, ok)
				assert.NotNil(t, ext)
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent list operations", func(t *testing.T) {
		reg := NewRegistry()

		// Register some extensions first
		for i := 0; i < 10; i++ {
			ext := &mockExtension{name: fmt.Sprintf("ext-%d", i)}
			_ = reg.Register(ext)
		}

		var wg sync.WaitGroup
		numGoroutines := 50

		// Concurrent list operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				list := reg.List()
				assert.Len(t, list, 10)
			}()
		}

		wg.Wait()
	})
}

// TestRegistry_Integration tests full registry lifecycle
func TestRegistry_Integration(t *testing.T) {
	t.Run("complete lifecycle with dependencies", func(t *testing.T) {
		reg := NewRegistry()

		// Create extensions with dependencies
		ext1 := &mockExtension{
			name:        "logger",
			version:     "1.0.0",
			description: "Logging extension",
		}

		ext2 := &mockExtension{
			name:        "metrics",
			version:     "1.0.0",
			description: "Metrics extension",
			deps:        []string{"logger"},
		}

		ext3 := &mockExtension{
			name:        "monitoring",
			version:     "1.0.0",
			description: "Monitoring extension",
			deps:        []string{"logger", "metrics"},
		}

		// Register extensions
		require.NoError(t, reg.Register(ext1))
		require.NoError(t, reg.Register(ext2))
		require.NoError(t, reg.Register(ext3))

		// Verify registration
		list := reg.List()
		assert.Len(t, list, 3)

		// Initialize extensions
		configs := map[string]ExtensionConfig{
			"logger":     {Enabled: true, Config: map[string]interface{}{"level": "info"}},
			"metrics":    {Enabled: true, Config: map[string]interface{}{"port": 9090}},
			"monitoring": {Enabled: true, Config: map[string]interface{}{"interval": 60}},
		}

		err := reg.Initialize(configs)
		require.NoError(t, err)

		// Verify all initialized
		assert.True(t, ext1.initialized)
		assert.True(t, ext2.initialized)
		assert.True(t, ext3.initialized)

		// Shutdown
		ctx := context.Background()
		err = reg.Shutdown(ctx)
		require.NoError(t, err)

		// Verify all shutdown
		assert.True(t, ext1.shutdown)
		assert.True(t, ext2.shutdown)
		assert.True(t, ext3.shutdown)
	})
}

// TestMockExtension_Coverage tests mock extension coverage
func TestMockExtension_Coverage(t *testing.T) {
	t.Run("mock extension methods", func(t *testing.T) {
		ext := &mockExtension{
			name:        "test",
			version:     "1.0.0",
			description: "Test extension",
			deps:        []string{"dep1"},
		}

		assert.Equal(t, "test", ext.Name())
		assert.Equal(t, "1.0.0", ext.Version())
		assert.Equal(t, "Test extension", ext.Description())
		assert.Equal(t, []string{"dep1"}, ext.Dependencies())
	})

	t.Run("mock extension with nil dependencies uses base", func(t *testing.T) {
		ext := &mockExtension{
			name:        "test",
			version:     "1.0.0",
			description: "Test extension",
			deps:        nil,
		}

		assert.Nil(t, ext.Dependencies())
	})

	t.Run("mock extension error handling", func(t *testing.T) {
		ctx := context.Background()

		ext := &mockExtension{
			name:         "test",
			beforeGenErr: errors.New("before gen error"),
			afterGenErr:  errors.New("after gen error"),
		}

		req := &GenerateRequest{Prompt: "test"}
		resp := &GenerateResponse{Content: "response"}

		err := ext.BeforeGenerate(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, "before gen error", err.Error())

		err = ext.AfterGenerate(ctx, req, resp)
		assert.Error(t, err)
		assert.Equal(t, "after gen error", err.Error())
	})

	t.Run("mock extension successful hooks", func(t *testing.T) {
		ctx := context.Background()

		ext := &mockExtension{name: "test"}
		req := &GenerateRequest{Prompt: "test"}
		resp := &GenerateResponse{Content: "response"}

		err := ext.BeforeGenerate(ctx, req)
		assert.NoError(t, err)

		err = ext.AfterGenerate(ctx, req, resp)
		assert.NoError(t, err)
	})
}

// TestRouteRegistrar tests route registration
func TestRouteRegistrar(t *testing.T) {
	t.Run("mock route registrar", func(t *testing.T) {
		registrar := &mockRouteRegistrar{}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		registrar.Handle("/test", handler)
		registrar.HandleFunc("/test-func", func(w http.ResponseWriter, r *http.Request) {})

		assert.Len(t, registrar.patterns, 2)
		assert.Contains(t, registrar.patterns, "/test")
		assert.Contains(t, registrar.patterns, "/test-func")
		assert.Len(t, registrar.handlers, 2)
	})

	t.Run("extension can register routes", func(t *testing.T) {
		ext := &mockExtension{name: "test"}
		registrar := &mockRouteRegistrar{}

		err := ext.RegisterRoutes(registrar)
		assert.NoError(t, err)
	})
}
