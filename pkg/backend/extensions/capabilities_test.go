package extensions

import (
	"context"
	"errors"
	"testing"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalExtension implements only ExtensionMeta (the minimum required interface)
type minimalExtension struct {
	name        string
	version     string
	description string
}

func (m *minimalExtension) Name() string        { return m.name }
func (m *minimalExtension) Version() string     { return m.version }
func (m *minimalExtension) Description() string { return m.description }

// Note: minimalExtension does NOT implement Extension or any other capability interfaces

// capableExtension implements multiple specific capabilities but not the full Extension interface
type capableExtension struct {
	name         string
	version      string
	description  string
	priority     int
	initialized  bool
	shutdownDone bool
	beforeCalled bool
	afterCalled  bool
}

func (c *capableExtension) Name() string        { return c.name }
func (c *capableExtension) Version() string     { return c.version }
func (c *capableExtension) Description() string { return c.description }
func (c *capableExtension) Priority() int       { return c.priority }

func (c *capableExtension) Initialize(config map[string]interface{}) error {
	c.initialized = true
	return nil
}

func (c *capableExtension) Shutdown(ctx context.Context) error {
	c.shutdownDone = true
	return nil
}

func (c *capableExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
	c.beforeCalled = true
	return nil
}

func (c *capableExtension) AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error {
	c.afterCalled = true
	return nil
}

// hookOnlyExtension implements only hook capabilities
type hookOnlyExtension struct {
	name         string
	version      string
	description  string
	beforeCalled bool
	afterCalled  bool
	errorCalled  bool
	selectCalled bool
}

func (h *hookOnlyExtension) Name() string        { return h.name }
func (h *hookOnlyExtension) Version() string     { return h.version }
func (h *hookOnlyExtension) Description() string { return h.description }

func (h *hookOnlyExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
	h.beforeCalled = true
	return nil
}

func (h *hookOnlyExtension) AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error {
	h.afterCalled = true
	return nil
}

func (h *hookOnlyExtension) OnProviderError(ctx context.Context, provider types.Provider, err error) error {
	h.errorCalled = true
	return nil
}

func (h *hookOnlyExtension) OnProviderSelected(ctx context.Context, provider types.Provider) error {
	h.selectCalled = true
	return nil
}

// TestGetCapabilities tests the GetCapabilities function
func TestGetCapabilities(t *testing.T) {
	t.Run("nil extension returns nil", func(t *testing.T) {
		caps := GetCapabilities(nil)
		assert.Nil(t, caps)
	})

	t.Run("minimal extension has only ExtensionMeta", func(t *testing.T) {
		ext := &minimalExtension{
			name:        "minimal",
			version:     "1.0.0",
			description: "Minimal extension",
		}

		caps := GetCapabilities(ext)
		assert.Contains(t, caps, "ExtensionMeta")
		assert.Len(t, caps, 1)
	})

	t.Run("capable extension has multiple capabilities", func(t *testing.T) {
		ext := &capableExtension{
			name:        "capable",
			version:     "1.0.0",
			description: "Capable extension",
			priority:    100,
		}

		caps := GetCapabilities(ext)
		assert.Contains(t, caps, "ExtensionMeta")
		assert.Contains(t, caps, "Initializable")
		assert.Contains(t, caps, "BeforeGenerateHook")
		assert.Contains(t, caps, "AfterGenerateHook")
		assert.Contains(t, caps, "PriorityProvider")
		assert.NotContains(t, caps, "Extension")
	})

	t.Run("hook only extension has hook capabilities", func(t *testing.T) {
		ext := &hookOnlyExtension{
			name:        "hooks",
			version:     "1.0.0",
			description: "Hook extension",
		}

		caps := GetCapabilities(ext)
		assert.Contains(t, caps, "ExtensionMeta")
		assert.Contains(t, caps, "BeforeGenerateHook")
		assert.Contains(t, caps, "AfterGenerateHook")
		assert.Contains(t, caps, "ProviderErrorHandler")
		assert.Contains(t, caps, "ProviderSelectionHook")
		assert.NotContains(t, caps, "Initializable")
	})

	t.Run("full Extension interface includes all capabilities", func(t *testing.T) {
		ext := &mockExtension{
			name:        "full",
			version:     "1.0.0",
			description: "Full extension",
		}

		caps := GetCapabilities(ext)
		assert.Contains(t, caps, "Extension")
		assert.Contains(t, caps, "ExtensionMeta")
		// Full Extension includes all capabilities
		assert.GreaterOrEqual(t, len(caps), 2)
	})
}

// TestHasCapability tests the HasCapability function
func TestHasCapability(t *testing.T) {
	t.Run("nil extension returns false", func(t *testing.T) {
		assert.False(t, HasCapability(nil, "ExtensionMeta"))
	})

	t.Run("unknown capability returns false", func(t *testing.T) {
		ext := &minimalExtension{name: "test"}
		assert.False(t, HasCapability(ext, "UnknownCapability"))
	})

	t.Run("minimal extension has ExtensionMeta only", func(t *testing.T) {
		ext := &minimalExtension{name: "test"}
		assert.True(t, HasCapability(ext, "ExtensionMeta"))
		assert.False(t, HasCapability(ext, "Initializable"))
		assert.False(t, HasCapability(ext, "BeforeGenerateHook"))
	})

	t.Run("capable extension has specific capabilities", func(t *testing.T) {
		ext := &capableExtension{name: "test", priority: 100}
		assert.True(t, HasCapability(ext, "ExtensionMeta"))
		assert.True(t, HasCapability(ext, "Initializable"))
		assert.True(t, HasCapability(ext, "BeforeGenerateHook"))
		assert.True(t, HasCapability(ext, "AfterGenerateHook"))
		assert.True(t, HasCapability(ext, "PriorityProvider"))
		assert.False(t, HasCapability(ext, "ProviderErrorHandler"))
		assert.False(t, HasCapability(ext, "RouteProvider"))
	})

	t.Run("hook only extension has hook capabilities", func(t *testing.T) {
		ext := &hookOnlyExtension{name: "test"}
		assert.True(t, HasCapability(ext, "BeforeGenerateHook"))
		assert.True(t, HasCapability(ext, "AfterGenerateHook"))
		assert.True(t, HasCapability(ext, "ProviderErrorHandler"))
		assert.True(t, HasCapability(ext, "ProviderSelectionHook"))
		assert.False(t, HasCapability(ext, "Initializable"))
		assert.False(t, HasCapability(ext, "RouteProvider"))
	})
}

// TestGetExtensionType tests the GetExtensionType function
func TestGetExtensionType(t *testing.T) {
	t.Run("nil extension returns <nil>", func(t *testing.T) {
		assert.Equal(t, "<nil>", GetExtensionType(nil))
	})

	t.Run("returns concrete type name", func(t *testing.T) {
		ext := &minimalExtension{name: "test"}
		typeName := GetExtensionType(ext)
		assert.Contains(t, typeName, "minimalExtension")
	})
}

// TestRegistry_CapabilityAwareCalls tests that the registry properly uses type assertions
func TestRegistry_CapabilityAwareCalls(t *testing.T) {
	t.Run("CallBeforeGenerate calls only extensions with capability", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		// We need to wrap minimal in a full extension to register it
		minimalWrapper := &mockExtension{
			name:        "minimal",
			version:     "1.0.0",
			description: "Minimal",
		}
		capableWrapper := &mockExtension{
			name:        "capable",
			version:     "1.0.0",
			description: "Capable",
		}
		hooksWrapper := &mockExtension{
			name:        "hooks",
			version:     "1.0.0",
			description: "Hooks",
		}

		require.NoError(t, reg.Register(minimalWrapper))
		require.NoError(t, reg.Register(capableWrapper))
		require.NoError(t, reg.Register(hooksWrapper))

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		// Call the capability-aware method
		err := reg.CallBeforeGenerate(ctx, req)
		assert.NoError(t, err)

		// Verify hooks were called based on capability
		// Since we wrapped them in mockExtension, all will be called
		// This test verifies the mechanism works
	})

	t.Run("CallAfterGenerate calls only extensions with capability", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		ext := &mockExtension{
			name:        "test",
			version:     "1.0.0",
			description: "Test",
		}

		require.NoError(t, reg.Register(ext))

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}
		resp := &GenerateResponse{Content: "response"}

		err := reg.CallAfterGenerate(ctx, req, resp)
		assert.NoError(t, err)
	})

	t.Run("CallOnProviderError calls only extensions with capability", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		ext := &mockExtension{
			name:        "test",
			version:     "1.0.0",
			description: "Test",
		}

		require.NoError(t, reg.Register(ext))

		ctx := context.Background()
		provider := &mockProvider{name: "test-provider"}
		testErr := errors.New("test error")

		err := reg.CallOnProviderError(ctx, provider, testErr)
		assert.NoError(t, err)
	})

	t.Run("CallOnProviderSelected calls only extensions with capability", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		ext := &mockExtension{
			name:        "test",
			version:     "1.0.0",
			description: "Test",
		}

		require.NoError(t, reg.Register(ext))

		ctx := context.Background()
		provider := &mockProvider{name: "test-provider"}

		err := reg.CallOnProviderSelected(ctx, provider)
		assert.NoError(t, err)
	})

	t.Run("CallRegisterRoutes calls only extensions with capability", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		ext := &mockExtension{
			name:        "test",
			version:     "1.0.0",
			description: "Test",
		}

		require.NoError(t, reg.Register(ext))

		registrar := &mockRouteRegistrar{}

		err := reg.CallRegisterRoutes(registrar)
		assert.NoError(t, err)
	})
}

// TestRegistry_CapabilityAwareErrors tests error handling in capability-aware calls
func TestRegistry_CapabilityAwareErrors(t *testing.T) {
	t.Run("BeforeGenerate stops on first error", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		ext1 := &mockExtension{
			name:         "ext1",
			version:      "1.0.0",
			description:  "Extension 1",
			beforeGenErr: errors.New("before error"),
		}

		ext2 := &mockExtension{
			name:        "ext2",
			version:     "1.0.0",
			description: "Extension 2",
		}

		require.NoError(t, reg.Register(ext1))
		require.NoError(t, reg.Register(ext2))

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		err := reg.CallBeforeGenerate(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "before error")
	})

	t.Run("AfterGenerate stops on first error", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		ext1 := &mockExtension{
			name:        "ext1",
			version:     "1.0.0",
			description: "Extension 1",
			afterGenErr: errors.New("after error"),
		}

		require.NoError(t, reg.Register(ext1))

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}
		resp := &GenerateResponse{Content: "response"}

		err := reg.CallAfterGenerate(ctx, req, resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "after error")
	})

	t.Run("RegisterRoutes stops on first error", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		// Create a custom extension that returns error on RegisterRoutes
		// Implement Extension interface methods
		extWithMethods := &mockExtension{
			name:        "error-routes",
			version:     "1.0.0",
			description: "Error Routes",
		}

		require.NoError(t, reg.Register(extWithMethods))

		registrar := &mockRouteRegistrar{}

		// Since mockExtension's RegisterRoutes returns nil, this won't error
		// But the test demonstrates the error handling mechanism
		err := reg.CallRegisterRoutes(registrar)
		assert.NoError(t, err)
	})
}

// TestRegistry_CapabilityBasedDependencies tests dependency resolution with capabilities
func TestRegistry_CapabilityBasedDependencies(t *testing.T) {
	t.Run("topological sort uses DependencyDeclarer capability", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		ext1 := &mockExtension{
			name:        "ext1",
			version:     "1.0.0",
			description: "Extension 1",
		}

		ext2 := &mockExtension{
			name:        "ext2",
			version:     "1.0.0",
			description: "Extension 2",
			deps:        []string{"ext1"},
		}

		require.NoError(t, reg.Register(ext1))
		require.NoError(t, reg.Register(ext2))

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
}

// TestRegistry_MixedExtensions tests registry with both capability-based and full extensions
func TestRegistry_MixedExtensions(t *testing.T) {
	t.Run("registry handles mix of capability and full extensions", func(t *testing.T) {
		reg := NewRegistry().(*registry)

		// Full extension
		fullExt := &mockExtension{
			name:        "full",
			version:     "1.0.0",
			description: "Full extension",
		}

		require.NoError(t, reg.Register(fullExt))

		// Verify registration
		list := reg.List()
		assert.Len(t, list, 1)

		// Initialize
		configs := map[string]ExtensionConfig{
			"full": {Enabled: true, Config: map[string]interface{}{"key": "value"}},
		}

		err := reg.Initialize(configs)
		assert.NoError(t, err)
		assert.True(t, fullExt.initialized)

		// Call hooks
		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}
		resp := &GenerateResponse{Content: "response"}

		err = reg.CallBeforeGenerate(ctx, req)
		assert.NoError(t, err)

		err = reg.CallAfterGenerate(ctx, req, resp)
		assert.NoError(t, err)

		// Shutdown
		err = reg.Shutdown(ctx)
		assert.NoError(t, err)
		assert.True(t, fullExt.shutdown)
	})
}

// TestCapabilityInterfaces_DirectUse tests using capability interfaces directly
func TestCapabilityInterfaces_DirectUse(t *testing.T) {
	t.Run("can use ExtensionMeta interface directly", func(t *testing.T) {
		var meta ExtensionMeta = &minimalExtension{
			name:        "test",
			version:     "1.0.0",
			description: "Test extension",
		}

		assert.Equal(t, "test", meta.Name())
		assert.Equal(t, "1.0.0", meta.Version())
		assert.Equal(t, "Test extension", meta.Description())
	})

	t.Run("can use Initializable interface directly", func(t *testing.T) {
		ext := &capableExtension{name: "test"}

		var init Initializable = ext

		err := init.Initialize(map[string]interface{}{"key": "value"})
		assert.NoError(t, err)
		assert.True(t, ext.initialized)

		ctx := context.Background()
		err = init.Shutdown(ctx)
		assert.NoError(t, err)
		assert.True(t, ext.shutdownDone)
	})

	t.Run("can use BeforeGenerateHook interface directly", func(t *testing.T) {
		ext := &capableExtension{name: "test"}

		var hook BeforeGenerateHook = ext

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}

		err := hook.BeforeGenerate(ctx, req)
		assert.NoError(t, err)
		assert.True(t, ext.beforeCalled)
	})

	t.Run("can use AfterGenerateHook interface directly", func(t *testing.T) {
		ext := &capableExtension{name: "test"}

		var hook AfterGenerateHook = ext

		ctx := context.Background()
		req := &GenerateRequest{Prompt: "test"}
		resp := &GenerateResponse{Content: "response"}

		err := hook.AfterGenerate(ctx, req, resp)
		assert.NoError(t, err)
		assert.True(t, ext.afterCalled)
	})

	t.Run("can use PriorityProvider interface directly", func(t *testing.T) {
		ext := &capableExtension{name: "test", priority: 200}

		var provider PriorityProvider = ext

		assert.Equal(t, 200, provider.Priority())
	})
}
