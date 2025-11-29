# Backend Package

HTTP server infrastructure for building AI-powered backend services with the AI Provider Kit.

## Architecture Overview

The backend package provides a complete HTTP server framework with these key components:

```
backend/
├── server.go          # Main server implementation
├── handlers/          # HTTP request handlers
│   ├── health.go      # Health check endpoints
│   ├── providers.go   # Provider management
│   └── generate.go    # Text generation endpoint
├── middleware/        # HTTP middleware chain
│   ├── auth.go        # API key authentication
│   ├── cors.go        # CORS headers
│   ├── logging.go     # Request logging
│   ├── recovery.go    # Panic recovery
│   └── requestid.go   # Request ID tracking
└── extensions/        # Extension framework
    ├── interface.go   # Extension interface
    └── registry.go    # Extension lifecycle
```

### Request Flow

1. **Recovery** - Catches panics and returns error responses
2. **Logging** - Logs request method, path, status, duration
3. **RequestID** - Generates/extracts unique request identifier
4. **CORS** - Adds CORS headers for cross-origin requests
5. **Auth** - Validates API key if authentication is enabled
6. **Handler** - Processes the request and generates response

Extensions hook into the request flow at key points (before/after generation, on provider selection, on errors).

## Quick Start

### Basic Server

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/backend"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/backendtypes"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    // Initialize providers
    providers := map[string]types.Provider{
        "openai": factory.NewOpenAI(types.ProviderConfig{
            Type:     "openai",
            Name:     "openai",
            APIKeyEnv: "OPENAI_API_KEY",
            DefaultModel: "gpt-4",
        }),
    }

    // Configure server
    config := backendtypes.BackendConfig{
        Server: backendtypes.ServerConfig{
            Host:            "0.0.0.0",
            Port:            8080,
            Version:         "1.0.0",
            ReadTimeout:     30 * time.Second,
            WriteTimeout:    30 * time.Second,
            ShutdownTimeout: 10 * time.Second,
        },
    }

    // Create and start server
    server := backend.NewServer(config, providers)

    // Graceful shutdown
    shutdown := make(chan struct{})
    go func() {
        sigint := make(chan os.Signal, 1)
        signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
        <-sigint
        close(shutdown)
    }()

    if err := server.ListenAndServeWithGracefulShutdown(shutdown); err != nil {
        log.Fatal(err)
    }
}
```

### With Authentication

```go
config := backendtypes.BackendConfig{
    Server: backendtypes.ServerConfig{
        Host: "0.0.0.0",
        Port: 8080,
    },
    Auth: backendtypes.AuthConfig{
        Enabled:   true,
        APIKeyEnv: "API_KEY",
        PublicPaths: []string{"/health", "/status"},
    },
}
```

Clients must include the API key in the Authorization header:

```bash
curl http://localhost:8080/api/generate \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello, world!"}'
```

### With CORS

```go
config := backendtypes.BackendConfig{
    CORS: backendtypes.CORSConfig{
        Enabled:        true,
        AllowedOrigins: []string{"http://localhost:3000", "https://app.example.com"},
        AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders: []string{"Content-Type", "Authorization"},
    },
}
```

## Configuration Reference

### BackendConfig

```go
type BackendConfig struct {
    Server     ServerConfig
    Auth       AuthConfig
    CORS       CORSConfig
    Providers  map[string]*types.ProviderConfig
    Extensions map[string]ExtensionConfig
}
```

### ServerConfig

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| Host | string | Server bind address | "0.0.0.0" |
| Port | int | Server port | 8080 |
| Version | string | API version for /version endpoint | "1.0.0" |
| ReadTimeout | time.Duration | Maximum time to read request | 30s |
| WriteTimeout | time.Duration | Maximum time to write response | 30s |
| ShutdownTimeout | time.Duration | Grace period for shutdown | 10s |

### AuthConfig

| Field | Type | Description |
|-------|------|-------------|
| Enabled | bool | Enable API key authentication |
| APIPassword | string | Static API key (not recommended for production) |
| APIKeyEnv | string | Environment variable containing API key |
| PublicPaths | []string | Paths that don't require authentication |

### CORSConfig

| Field | Type | Description |
|-------|------|-------------|
| Enabled | bool | Enable CORS middleware |
| AllowedOrigins | []string | Allowed origin domains (use "*" for all) |
| AllowedMethods | []string | Allowed HTTP methods |
| AllowedHeaders | []string | Allowed request headers |

## Middleware

Middleware executes in this order:

### 1. Recovery

Catches panics and returns structured error responses. Always enabled.

```json
{
  "success": false,
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "An internal error occurred"
  }
}
```

### 2. Logging

Logs all requests with method, path, status code, response size, and duration. Always enabled.

```
[abc123] GET /api/generate 200 1024 250ms
```

### 3. RequestID

Generates or extracts request ID from X-Request-ID header. Always enabled.

```go
// Access in handler
requestID := middleware.GetRequestID(r.Context())
```

### 4. CORS

Adds CORS headers based on configuration. Enable via config.

### 5. Auth

Validates Bearer token in Authorization header. Enable via config.

Protected endpoints return 401 if:
- Authorization header is missing
- Token doesn't match configured API key
- Path is not in PublicPaths list

## API Routes

### Health Endpoints

#### GET /health

Detailed health check with provider status.

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "version": "1.0.0",
    "uptime": "2h15m30s",
    "providers": {
      "openai": {
        "status": "ok"
      }
    }
  }
}
```

#### GET /status

Simple liveness check.

```json
{
  "success": true,
  "data": {
    "status": "ok"
  }
}
```

#### GET /version

Version information.

```json
{
  "success": true,
  "data": {
    "version": "1.0.0"
  }
}
```

### Provider Endpoints

#### GET /api/providers

List all configured providers.

```json
{
  "success": true,
  "data": [
    {
      "name": "openai",
      "type": "openai",
      "description": "OpenAI GPT models",
      "enabled": true,
      "healthy": true,
      "models": ["gpt-4", "gpt-3.5-turbo"]
    }
  ]
}
```

#### GET /api/providers/{name}

Get details for a specific provider.

#### PUT /api/providers/{name}

Update provider configuration.

```json
{
  "base_url": "https://api.openai.com/v1",
  "api_key": "new-key",
  "default_model": "gpt-4"
}
```

#### GET /api/providers/{name}/health

Check provider health with latency measurement.

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "message": "Provider is responding normally",
    "latency": 150
  }
}
```

#### POST /api/providers/{name}/test

Test provider with a simple generation request.

```json
{
  "prompt": "Hello, this is a test",
  "model": "gpt-4",
  "max_tokens": 50
}
```

### Generation Endpoint

#### POST /api/generate

Generate text completion.

**Request:**

```json
{
  "provider": "openai",
  "model": "gpt-4",
  "prompt": "Write a haiku about coding",
  "max_tokens": 100,
  "temperature": 0.7,
  "stream": false
}
```

Or with messages:

```json
{
  "provider": "openai",
  "model": "gpt-4",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant"
    },
    {
      "role": "user",
      "content": "What is the capital of France?"
    }
  ],
  "temperature": 0.7
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "content": "Code flows like streams\nThrough silicon valleys deep\nBugs bloom, then are fixed",
    "model": "gpt-4",
    "provider": "openai",
    "usage": {
      "prompt_tokens": 10,
      "completion_tokens": 20,
      "total_tokens": 30
    }
  }
}
```

## Error Handling

All errors follow this structure:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message"
  }
}
```

Common error codes:

- `INVALID_REQUEST` - Malformed request body
- `PROVIDER_NOT_FOUND` - Requested provider doesn't exist
- `UNAUTHORIZED` - Invalid or missing API key
- `METHOD_NOT_ALLOWED` - HTTP method not supported
- `GENERATION_ERROR` - Provider failed to generate
- `INTERNAL_ERROR` - Server panic or unexpected error

## Advanced Usage

### Custom Extensions

```go
// Create extension
ext := &MyExtension{}

// Register before starting server
server := backend.NewServer(config, providers)
server.RegisterExtension(ext)
server.Start()
```

See [extensions/README.md](extensions/README.md) for details.

### Multiple Providers

```go
providers := map[string]types.Provider{
    "openai":    openaiProvider,
    "anthropic": anthropicProvider,
    "gemini":    geminiProvider,
}

server := backend.NewServer(config, providers)
```

Request specific provider:

```json
{
  "provider": "anthropic",
  "model": "claude-3-opus-20240229",
  "prompt": "Hello"
}
```

### Virtual Providers

Combine multiple providers with racing, fallback, or load balancing:

```go
providers := map[string]types.Provider{
    "fast": racingProvider,      // Races OpenAI vs Anthropic
    "reliable": fallbackProvider, // Tries OpenAI, then Anthropic
    "balanced": loadbalanceProvider, // Round-robin distribution
}
```

See [../providers/virtual/README.md](../providers/virtual/README.md) for details.

## Best Practices

1. **Always use environment variables** for API keys, never hardcode
2. **Enable authentication** in production environments
3. **Set appropriate timeouts** based on your use case
4. **Use health checks** for monitoring and load balancer integration
5. **Implement graceful shutdown** to finish in-flight requests
6. **Add request IDs** to logs for request tracing
7. **Use extensions** for cross-cutting concerns (metrics, caching, etc.)

## Examples

See the [examples/](../../examples/) directory for complete applications:

- `simple-server/` - Basic HTTP server
- `multi-provider/` - Multiple providers with fallback
- `authenticated/` - Production-ready with auth and CORS
- `with-extensions/` - Custom extensions for metrics and caching
