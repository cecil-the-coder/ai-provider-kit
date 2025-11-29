package extensions

import (
	"context"
	"fmt"
	"sync"
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

		for _, dep := range ext.Dependencies() {
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
