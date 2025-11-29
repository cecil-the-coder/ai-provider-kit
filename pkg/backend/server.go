package backend

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/extensions"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/handlers"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backend/middleware"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// Server represents the backend HTTP server that ties all components together
type Server struct {
	config     backendtypes.BackendConfig
	httpServer *http.Server
	providers  map[string]types.Provider
	extensions extensions.ExtensionRegistry
	mux        *http.ServeMux
}

// NewServer creates a new backend server with the given configuration and providers
func NewServer(config backendtypes.BackendConfig, providers map[string]types.Provider) *Server {
	s := &Server{
		config:     config,
		providers:  providers,
		extensions: extensions.NewRegistry(),
		mux:        http.NewServeMux(),
	}

	// Initialize extensions if configured
	if len(config.Extensions) > 0 {
		// Convert backendtypes.ExtensionConfig to extensions.ExtensionConfig
		extConfigs := make(map[string]extensions.ExtensionConfig)
		for name, cfg := range config.Extensions {
			extConfigs[name] = extensions.ExtensionConfig{
				Enabled: cfg.Enabled,
				Config:  cfg.Config,
			}
		}
		if err := s.extensions.Initialize(extConfigs); err != nil {
			log.Printf("Warning: Failed to initialize extensions: %v", err)
		}
	}

	// Setup routes with handlers
	s.setupRoutes()

	return s
}

// setupRoutes registers all HTTP routes with their corresponding handlers
func (s *Server) setupRoutes() {
	// Create handlers
	healthHandler := handlers.NewHealthHandler(s.providers, s.config.Server.Version)
	providerHandler := handlers.NewProviderHandler(s.providers)

	// Determine default provider (first one in the map if not specified)
	defaultProvider := ""
	for name := range s.providers {
		defaultProvider = name
		break
	}
	generateHandler := handlers.NewGenerateHandler(s.providers, s.extensions, defaultProvider)

	// Health and status endpoints
	s.mux.HandleFunc("/health", healthHandler.Health)
	s.mux.HandleFunc("/status", healthHandler.Status)
	s.mux.HandleFunc("/version", healthHandler.Version)

	// Provider management endpoints
	s.mux.HandleFunc("/api/providers", providerHandler.ListProviders)
	s.mux.HandleFunc("/api/providers/", s.routeProviderRequests(providerHandler))

	// Generation endpoints
	s.mux.HandleFunc("/api/generate", generateHandler.Generate)
}

// routeProviderRequests routes provider-specific requests to the appropriate handler method
func (s *Server) routeProviderRequests(h *handlers.ProviderHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a health check request
		if r.URL.Path == "/api/providers/health" ||
			(len(r.URL.Path) > len("/api/providers/") &&
				r.URL.Path[len(r.URL.Path)-7:] == "/health") {
			h.HealthCheckProvider(w, r)
			return
		}

		// Check if it's a test request
		if r.URL.Path == "/api/providers/test" ||
			(len(r.URL.Path) > len("/api/providers/") &&
				r.URL.Path[len(r.URL.Path)-5:] == "/test") {
			h.TestProvider(w, r)
			return
		}

		// Handle GET/PUT requests for specific provider
		switch r.Method {
		case http.MethodGet:
			h.GetProvider(w, r)
		case http.MethodPut, http.MethodPost:
			h.UpdateProvider(w, r)
		default:
			handlers.SendError(w, r, "METHOD_NOT_ALLOWED",
				"Only GET, PUT, or POST methods are allowed",
				http.StatusMethodNotAllowed)
		}
	}
}

// Start starts the HTTP server and begins listening for requests
func (s *Server) Start() error {
	// Build middleware chain
	handler := s.applyMiddleware(s.mux)

	// Construct server address
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)

	// Create HTTP server with configured timeouts
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
	}

	log.Printf("Starting server on %s (version: %s)", addr, s.config.Server.Version)
	log.Printf("Registered %d provider(s): ", len(s.providers))
	for name := range s.providers {
		log.Printf("  - %s", name)
	}

	// Start listening
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server and all its components
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")

	// Shutdown extensions first
	if err := s.extensions.Shutdown(ctx); err != nil {
		log.Printf("Warning: Error shutting down extensions: %v", err)
	}

	// Shutdown HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
	}

	log.Println("Server shutdown complete")
	return nil
}

// applyMiddleware builds the middleware chain and applies it to the handler
// Middleware is applied in reverse order (last applied runs first)
func (s *Server) applyMiddleware(h http.Handler) http.Handler {
	// Apply in reverse order - outer middleware wraps inner
	// Execution order: Recovery -> Logging -> RequestID -> CORS -> Auth -> Handler

	// Apply auth middleware if enabled
	if s.config.Auth.Enabled {
		h = middleware.Auth(middleware.AuthConfig{
			Enabled:     true,
			APIPassword: s.config.Auth.APIPassword,
			APIKeyEnv:   s.config.Auth.APIKeyEnv,
			PublicPaths: s.config.Auth.PublicPaths,
		})(h)
	}

	// Apply CORS middleware if enabled
	if s.config.CORS.Enabled {
		h = middleware.CORS(middleware.CORSConfig{
			AllowedOrigins: s.config.CORS.AllowedOrigins,
			AllowedMethods: s.config.CORS.AllowedMethods,
			AllowedHeaders: s.config.CORS.AllowedHeaders,
		})(h)
	}

	// Apply request ID middleware (always enabled)
	h = middleware.RequestID(h)

	// Apply logging middleware (always enabled)
	h = middleware.Logging(h)

	// Apply recovery middleware (always enabled, outermost)
	h = middleware.Recovery(h)

	return h
}

// RegisterExtension allows registering an extension with the server
// This should be called before Start()
func (s *Server) RegisterExtension(ext extensions.Extension) error {
	return s.extensions.Register(ext)
}

// GetExtensionRegistry returns the extension registry for advanced use cases
func (s *Server) GetExtensionRegistry() extensions.ExtensionRegistry {
	return s.extensions
}

// GetProviders returns the registered providers map
func (s *Server) GetProviders() map[string]types.Provider {
	return s.providers
}

// GetConfig returns the server configuration
func (s *Server) GetConfig() backendtypes.BackendConfig {
	return s.config
}

// ListenAndServeWithGracefulShutdown starts the server and handles graceful shutdown
// This is a convenience method that starts the server and waits for shutdown signal
func (s *Server) ListenAndServeWithGracefulShutdown(shutdownSignal <-chan struct{}) error {
	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		return err
	case <-shutdownSignal:
		// Create shutdown context with timeout
		timeout := s.config.Server.ShutdownTimeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		return s.Shutdown(ctx)
	}
}
