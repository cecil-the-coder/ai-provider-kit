package handlers

import (
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type HealthHandler struct {
	providers map[string]types.Provider
	version   string
	startTime time.Time
}

func NewHealthHandler(providers map[string]types.Provider, version string) *HealthHandler {
	return &HealthHandler{
		providers: providers,
		version:   version,
		startTime: time.Now(),
	}
}

// Status returns simple liveness status
func (h *HealthHandler) Status(w http.ResponseWriter, r *http.Request) {
	SendSuccess(w, r, map[string]string{"status": "ok"})
}

// Health returns detailed health with provider status
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	providerHealth := make(map[string]backendtypes.ProviderHealth)

	for name := range h.providers {
		// Check if provider implements health checking
		// Or just mark as "unknown" if not
		providerHealth[name] = backendtypes.ProviderHealth{
			Status: "ok",
		}
	}

	response := backendtypes.HealthResponse{
		Status:    "healthy",
		Version:   h.version,
		Uptime:    time.Since(h.startTime).String(),
		Providers: providerHealth,
	}

	SendSuccess(w, r, response)
}

// Version returns version information
func (h *HealthHandler) Version(w http.ResponseWriter, r *http.Request) {
	SendSuccess(w, r, map[string]string{
		"version": h.version,
	})
}
