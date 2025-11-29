package extensions

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type registry struct {
	mu         sync.RWMutex
	extensions map[string]Extension
	order      []string
}

func NewRegistry() ExtensionRegistry {
	return &registry{
		extensions: make(map[string]Extension),
		order:      make([]string, 0),
	}
}

func (r *registry) Register(ext Extension) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := ext.Name()
	if _, exists := r.extensions[name]; exists {
		return fmt.Errorf("extension %s already registered", name)
	}

	r.extensions[name] = ext
	r.order = append(r.order, name)
	return nil
}

func (r *registry) Get(name string) (Extension, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ext, ok := r.extensions[name]
	return ext, ok
}

func (r *registry) List() []Extension {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Extension, 0, len(r.extensions))
	for _, name := range r.order {
		result = append(result, r.extensions[name])
	}

	// Sort by priority (lower runs first), using stable sort to preserve
	// registration order for extensions with the same priority
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Priority() < result[j].Priority()
	})

	return result
}

func (r *registry) Initialize(configs map[string]ExtensionConfig) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sorted, err := r.topologicalSort()
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	for _, name := range sorted {
		ext := r.extensions[name]
		cfg, ok := configs[name]
		if !ok || !cfg.Enabled {
			continue
		}
		if err := ext.Initialize(cfg.Config); err != nil {
			return fmt.Errorf("failed to initialize extension %s: %w", name, err)
		}
	}
	return nil
}

func (r *registry) Shutdown(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := len(r.order) - 1; i >= 0; i-- {
		name := r.order[i]
		if err := r.extensions[name].Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown extension %s: %w", name, err)
		}
	}
	return nil
}

func (r *registry) topologicalSort() ([]string, error) {
	visited := make(map[string]bool)
	result := make([]string, 0, len(r.extensions))

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		visited[name] = true

		ext, ok := r.extensions[name]
		if !ok {
			return nil
		}

		// Use capability-based dependency checking
		var deps []string
		if declarer, ok := ext.(DependencyDeclarer); ok {
			deps = declarer.Dependencies()
		} else {
			deps = ext.Dependencies()
		}

		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}

		result = append(result, name)
		return nil
	}

	for _, name := range r.order {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// CallBeforeGenerate invokes BeforeGenerate hooks on all registered extensions that implement it.
// Extensions are called in priority order (lowest priority first).
// If any extension returns an error, iteration stops and the error is returned.
func (r *registry) CallBeforeGenerate(ctx context.Context, req *GenerateRequest) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exts := r.List()
	for _, ext := range exts {
		if hook, ok := ext.(BeforeGenerateHook); ok {
			if err := hook.BeforeGenerate(ctx, req); err != nil {
				return err
			}
		}
	}
	return nil
}

// CallAfterGenerate invokes AfterGenerate hooks on all registered extensions that implement it.
// Extensions are called in priority order (lowest priority first).
// If any extension returns an error, iteration stops and the error is returned.
func (r *registry) CallAfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exts := r.List()
	for _, ext := range exts {
		if hook, ok := ext.(AfterGenerateHook); ok {
			if err := hook.AfterGenerate(ctx, req, resp); err != nil {
				return err
			}
		}
	}
	return nil
}

// CallOnProviderError invokes OnProviderError hooks on all registered extensions that implement it.
// Extensions are called in priority order (lowest priority first).
// If any extension returns an error, iteration stops and the error is returned.
func (r *registry) CallOnProviderError(ctx context.Context, provider types.Provider, err error) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exts := r.List()
	for _, ext := range exts {
		if handler, ok := ext.(ProviderErrorHandler); ok {
			if hookErr := handler.OnProviderError(ctx, provider, err); hookErr != nil {
				return hookErr
			}
		}
	}
	return nil
}

// CallOnProviderSelected invokes OnProviderSelected hooks on all registered extensions that implement it.
// Extensions are called in priority order (lowest priority first).
// If any extension returns an error, iteration stops and the error is returned.
func (r *registry) CallOnProviderSelected(ctx context.Context, provider types.Provider) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exts := r.List()
	for _, ext := range exts {
		if hook, ok := ext.(ProviderSelectionHook); ok {
			if err := hook.OnProviderSelected(ctx, provider); err != nil {
				return err
			}
		}
	}
	return nil
}

// CallRegisterRoutes invokes RegisterRoutes on all registered extensions that implement it.
// Extensions are called in registration order.
// If any extension returns an error, iteration stops and the error is returned.
func (r *registry) CallRegisterRoutes(registrar RouteRegistrar) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use registration order, not priority order, for route registration
	for _, name := range r.order {
		ext := r.extensions[name]
		if provider, ok := ext.(RouteProvider); ok {
			if err := provider.RegisterRoutes(registrar); err != nil {
				return fmt.Errorf("failed to register routes for extension %s: %w", name, err)
			}
		}
	}
	return nil
}
