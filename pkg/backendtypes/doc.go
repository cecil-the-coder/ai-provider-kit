// Package backendtypes defines types for backend server configuration and API communication.
//
// This package provides shared type definitions used by the backend package and applications
// built on ai-provider-kit. It separates type definitions from implementation to allow
// clean imports without circular dependencies.
//
// # Type Categories
//
// The package defines several categories of types:
//
// # Configuration Types
//
// BackendConfig and related types define how the backend server is configured:
//
//   - ServerConfig: HTTP server settings (host, port, timeouts)
//   - AuthConfig: Authentication configuration
//   - LoggingConfig: Logging settings
//   - CORSConfig: Cross-origin resource sharing settings
//   - ExtensionConfig: Per-extension configuration
//
// # Request Types
//
// Request types define the structure of incoming API requests:
//
//   - ChatRequest: Chat completion requests
//   - StreamRequest: Streaming requests
//
// # Response Types
//
// Response types define the structure of API responses:
//
//   - ChatResponse: Chat completion responses
//   - ErrorResponse: Standard error format
//   - HealthResponse: Health check responses
//
// # Usage
//
// Import this package to use backend types without importing the full backend implementation:
//
//	import "github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
//
//	config := backendtypes.BackendConfig{
//	    Server: backendtypes.ServerConfig{Port: 8080},
//	}
package backendtypes
