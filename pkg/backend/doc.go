// Package backend provides HTTP server infrastructure for building AI-powered backend services.
//
// This package serves as the foundation for applications built on top of ai-provider-kit,
// providing reusable components for HTTP routing, middleware, and request handling.
//
// # Architecture
//
// The backend package is organized into several sub-packages:
//
//   - handlers: Core API handlers for common operations (health checks, provider info, etc.)
//   - middleware: Reusable HTTP middleware (authentication, logging, rate limiting, etc.)
//   - extensions: Extension framework for adding custom functionality to the server
//
// # Usage
//
// Applications using this package typically:
//
//  1. Create a BackendConfig with server settings
//  2. Initialize providers using the factory package
//  3. Register middleware and extensions
//  4. Start the HTTP server
//
// # Example
//
//	config := backendtypes.BackendConfig{
//	    Server: backendtypes.ServerConfig{
//	        Host: "0.0.0.0",
//	        Port: 8080,
//	    },
//	}
//	server := backend.NewServer(config, providers)
//	server.Start()
//
// This package is designed to be extended by downstream applications like VelocityCode
// and Cortex, which add their own domain-specific handlers and extensions.
package backend
