# Development Build Tags

This directory contains development and testing utilities that are excluded from production builds using Go build constraints.

## Build Tags

The files in this package use the following build constraint:

```go
//go:build dev || debug
```

This means the code will only be included in builds when one of these build tags is specified:

- `dev`: For development builds that include development utilities
- `debug`: For debug builds that include additional debugging and testing tools

## Usage

### Including Development Utilities

To include development utilities in your build, use one of these commands:

```bash
# Build with dev tag
go build -tags="dev" ./...

# Build with debug tag
go build -tags="debug" ./...

# Build with both tags (redundant but valid)
go build -tags="dev,debug" ./...
```

### Production Builds

For production builds, simply build without any tags:

```bash
# Production build (dev utilities excluded)
go build ./...
```

## Available Utilities

### ConfigHelper (`helpers.go`)

Utilities for working with provider configurations:

- Load/save configuration from/to JSON files
- Create test configurations
- Validate provider configurations
- Get API keys from environment variables

**Example usage:**
```go
// This code requires build tags
//go:build dev || debug

package main

import (
    "fmt"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/dev"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func main() {
    configHelper := dev.NewConfigHelper()
    config := configHelper.CreateTestConfig(types.ProviderTypeOpenAI, "test-key")
    fmt.Printf("Created config: %+v\n", config)
}
```

### TestSuite (`testing.go`)

Comprehensive testing utilities for AI providers:

- Mock HTTP server for testing provider integrations
- Mock providers with configurable responses
- Performance testing utilities
- Debug logging for HTTP requests/responses

**Example usage:**
```go
// This code requires build tags
//go:build dev || debug

package main

import (
    "testing"
    "time"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/dev"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestProvider(t *testing.T) {
    config := dev.TestSuiteConfig{
        EnableLogging: true,
        LogRequests:   true,
        LogResponses:  true,
        DefaultTimeout: 30 * time.Second,
    }

    testSuite := dev.NewTestSuite(config)
    defer testSuite.Cleanup()

    // Add mock provider and responses
    mockConfig := types.ProviderConfig{
        Type:   types.ProviderTypeOpenAI,
        APIKey: "test-key",
    }

    provider := testSuite.AddMockProvider("test-openai", types.ProviderTypeOpenAI, mockConfig)

    // Add mock responses
    testSuite.AddMockResponse("test-openai", dev.MockResponse{
        StatusCode: 200,
        Body: map[string]interface{}{
            "choices": []map[string]interface{}{
                {
                    "message": map[string]interface{}{
                        "role":    "assistant",
                        "content": "Hello from mock!",
                    },
                },
            },
        },
    })

    // Run tests
    testSuite.RunProviderTest(t, "test-openai", func(t *testing.T, p *dev.MockProvider) {
        if p.RequestCount == 0 {
            t.Error("No requests made to mock provider")
        }
    })
}
```

## Environment Variables

Some utilities look for these environment variables:

- `OPENAI_API_KEY`: OpenAI API key for testing
- `ANTHROPIC_API_KEY`: Anthropic API key for testing
- `OPENROUTER_API_KEY`: OpenRouter API key for testing
- `CEREBRAS_API_KEY`: Cerebras API key for testing
- `GEMINI_API_KEY`: Gemini API key for testing
- `CI`: Set to true in CI environments
- `GO_ENV`: Set to "test" for test environment detection

## Build Integration

### Makefile Example

```makefile
# Production build
build:
	go build ./...

# Development build
build-dev:
	go build -tags="dev" ./...

# Debug build
build-debug:
	go build -tags="debug" ./...

# Development tests
test-dev:
	go test -tags="dev" ./...
```

### CI/CD Example

```yaml
# GitHub Actions example
name: Build
on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      # Production build (without dev utilities)
      - name: Build production
        run: go build -v ./...

      # Development build tests
      - name: Build with dev tags
        run: go build -tags="dev" -v ./...

      # Run development tests
      - name: Run dev tests
        run: go test -tags="dev" -v ./pkg/dev/...
```

## Benefits

Using build tags for development utilities provides several advantages:

1. **Smaller Production Binaries**: Development code is not included in production builds
2. **Cleaner Dependencies**: Development-only dependencies don't affect production dependency graph
3. **Explicit Development Mode**: Developers must explicitly opt-in to include development utilities
4. **Better Security**: Sensitive development utilities (like mock servers) are excluded from production

## Migration Notes

If you have existing code that depends on the `pkg/dev` package, you'll need to:

1. Add the appropriate build tags to your Go files that import from `pkg/dev`
2. Update your build scripts to include the build tags when needed
3. Ensure CI/CD pipelines use the correct build tags for development workflows

For example:

```go
// Before (included in all builds)
package main

import "github.com/cecil-the-coder/ai-provider-kit/pkg/dev"

// After (only included in dev/debug builds)
//go:build dev || debug

package main

import "github.com/cecil-the-coder/ai-provider-kit/pkg/dev"
```