// Package openrouter provides an OpenRouter AI provider implementation.
// It includes support for model selection, API key management, OAuth authentication,
// and specialized features for the OpenRouter platform.
package openrouter

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
)

// ModelSelector implements different strategies for selecting models
type ModelSelector struct {
	models       []string
	strategy     string
	currentIndex int
	mutex        sync.Mutex
	failedModels map[string]bool
}

// NewModelSelector creates a new ModelSelector with the given models and strategy
func NewModelSelector(models []string, strategy string) *ModelSelector {
	return &ModelSelector{
		models:       models,
		strategy:     strategy,
		currentIndex: 0,
		mutex:        sync.Mutex{},
		failedModels: make(map[string]bool),
	}
}

// SelectModel selects a model based on the configured strategy
func (ms *ModelSelector) SelectModel() (string, error) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	if len(ms.models) == 0 {
		return "", fmt.Errorf("no models available")
	}

	switch ms.strategy {
	case "round-robin":
		return ms.selectRoundRobin(), nil
	case "random":
		return ms.selectRandom(), nil
	case "failover":
		fallthrough
	default:
		return ms.selectFailover()
	}
}

// RecordFailure marks a model as failed for the failover strategy
func (ms *ModelSelector) RecordFailure(model string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	ms.failedModels[model] = true
}

// Reset clears the failed models map
func (ms *ModelSelector) Reset() {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	ms.failedModels = make(map[string]bool)
}

func (ms *ModelSelector) selectFailover() (string, error) {
	// Find the first model that hasn't failed
	for _, model := range ms.models {
		if !ms.failedModels[model] {
			return model, nil
		}
	}

	// If all models have failed, reset and return the first model
	ms.failedModels = make(map[string]bool)
	if len(ms.models) > 0 {
		return ms.models[0], nil
	}

	return "", fmt.Errorf("no models available")
}

func (ms *ModelSelector) selectRoundRobin() string {
	model := ms.models[ms.currentIndex]
	ms.currentIndex = (ms.currentIndex + 1) % len(ms.models)
	return model
}

func (ms *ModelSelector) selectRandom() string {
	if len(ms.models) == 0 {
		return ""
	}

	// Use crypto/rand for secure random selection
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(ms.models))))
	if err != nil {
		// Fallback to first model if crypto/rand fails
		return ms.models[0]
	}

	return ms.models[nBig.Int64()]
}
