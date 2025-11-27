# Common Utilities Reference

Comprehensive documentation for the `pkg/providers/common` package - shared utilities used across all provider implementations to reduce code duplication and ensure consistent behavior.

## Table of Contents

1. [Overview](#overview)
2. [Language Detection](#language-detection)
3. [File Utilities](#file-utilities)
4. [Phase 2: Streaming Abstraction](#phase-2-streaming-abstraction)
5. [Phase 2: Authentication Helpers](#phase-2-authentication-helpers)
6. [Phase 2: OAuth Token Refresh](#phase-2-oauth-token-refresh)
7. [Phase 2: Configuration Helper](#phase-2-configuration-helper)
8. [Rate Limit Helper](#rate-limit-helper)
9. [Model Cache](#model-cache)
10. [Testing Framework](#testing-framework)
11. [Provider Development Guide with Phase 2](#provider-development-guide-with-phase-2)
12. [Migration Guide to Phase 2](#migration-guide-to-phase-2)
13. [Best Practices](#best-practices)
14. [Examples](#examples)

---

## Overview

The `pkg/providers/common` package provides a comprehensive set of shared utilities designed to:

- **Reduce Code Duplication**: Common functionality implemented once and reused across all providers
- **Ensure Consistency**: Standardized behavior for rate limiting, file processing, etc.
- **Accelerate Development**: New providers can be implemented quickly using battle-tested utilities
- **Improve Testing**: Comprehensive testing helpers reduce boilerplate and ensure thorough coverage
- **Simplify Maintenance**: Centralized code fixes benefit all providers automatically

### Package Structure

```
pkg/providers/common/
├── streaming.go           # Phase 2: Streaming abstraction
├── authhelpers.go         # Phase 2: Authentication helpers
├── oauth_refresh.go       # Phase 2: OAuth token refresh
├── confighelper.go        # Phase 2: Configuration helper
├── language.go           # Programming language detection
├── fileutils.go          # File reading and processing utilities
├── ratelimit.go          # Rate limiting helper and wrapper
├── modelcache.go         # Thread-safe model list caching
└── testing/              # Testing framework and helpers
    ├── helpers.go        # Provider testing utilities
    ├── tool_helpers.go   # Tool calling test helpers
    ├── config_helpers.go # Configuration test helpers
    └── auth_helpers.go   # Authentication test helpers
```

### Benefits for Provider Developers

1. **Phase 2 Dramatic Reduction**: 90%+ reduction in authentication, streaming, and configuration code
2. **Zero Boilerplate**: No need to write OAuth refresh, stream parsing, or configuration validation
3. **Production-Ready**: Automatic token refresh, failover, rate limiting, and error handling
4. **Provider-Agnostic**: Write once, works with all providers
5. **Consistency**: Identical behavior across all providers
6. **Testing**: Comprehensive test helpers for all components
7. **Documentation**: Clear examples and patterns for all operations

---

## Language Detection

The language detection utility provides consistent programming language identification across all providers.

### DetectLanguage Function

Detects the programming language based on file extension or special filenames.

```go
func DetectLanguage(filename string) string
```

**Parameters:**
- `filename`: File name or path to analyze

**Returns:** Language name as a string (e.g., "go", "javascript", "python")

#### Supported Languages

| Extension | Language | Special Files |
|-----------|----------|---------------|
| `.go` | go | Dockerfile → dockerfile |
| `.js`, `.jsx` | javascript | Makefile → makefile |
| `.ts`, `.tsx` | typescript | README.md → markdown |
| `.py` | python | |
| `.java` | java | |
| `.cpp`, `.cc`, `.cxx` | cpp | |
| `.c` | c | |
| `.cs` | csharp | |
| `.php` | php | |
| `.rb` | ruby | |
| `.swift` | swift | |
| `.kt` | kotlin | |
| `.rs` | rust | |
| `.html` | html | |
| `.css` | css | |
| `.scss`, `.sass` | scss | |
| `.json` | json | |
| `.xml` | xml | |
| `.yaml`, `.yml` | yaml | |
| `.sql` | sql | |
| `.sh`, `.bash` | bash | |
| `.ps1` | powershell | |
| `.md` | markdown | |
| *unknown* | text | |

#### Usage Examples

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"

// Basic usage
language := common.DetectLanguage("main.go")        // "go"
language = common.DetectLanguage("script.py")       // "python"
language = common.DetectLanguage("Dockerfile")     // "dockerfile"
language = common.DetectLanguage("README.md")      // "markdown"
language = common.DetectLanguage("unknown.xyz")    // "text"

// In file processing
func processFile(filename string) error {
    language := common.DetectLanguage(filename)

    switch language {
    case "go", "python", "javascript":
        return processCodeFile(filename, language)
    case "markdown":
        return processDocumentation(filename)
    default:
        return processTextFile(filename)
    }
}

// For syntax highlighting or special processing
func addLanguageContext(files []string) string {
    var builder strings.Builder
    for _, file := range files {
        language := common.DetectLanguage(file)
        builder.WriteString(fmt.Sprintf("// File: %s (Language: %s)\n", file, language))

        content, err := common.ReadFileContent(file)
        if err != nil {
            continue
        }
        builder.WriteString(content)
        builder.WriteString("\n\n")
    }
    return builder.String()
}
```

---

## File Utilities

File utilities provide consistent file reading and processing across all providers.

### ReadFileContent

Reads file content and returns it as a string.

```go
func ReadFileContent(filename string) (string, error)
```

**Parameters:**
- `filename`: Path to the file to read

**Returns:** File content as string and any error encountered

```go
// Basic usage
content, err := common.ReadFileContent("config.yaml")
if err != nil {
    return fmt.Errorf("failed to read config: %w", err)
}
fmt.Printf("File content: %s", content)

// In context file processing
func loadContextFiles(files []string) (string, error) {
    var context strings.Builder
    for _, file := range files {
        content, err := common.ReadFileContent(file)
        if err != nil {
            return "", fmt.Errorf("failed to read %s: %w", file, err)
        }
        context.WriteString(content)
        context.WriteString("\n\n")
    }
    return context.String(), nil
}
```

### FilterContextFiles

Filters out the output file from context files to avoid duplication.

```go
func FilterContextFiles(contextFiles []string, outputFile string) []string
```

**Parameters:**
- `contextFiles`: List of context file paths
- `outputFile`: Path to the output file to exclude from context

**Returns:** Filtered list of context files excluding the output file

```go
// Prevent including the output file in context
contextFiles := []string{"file1.txt", "file2.txt", "output.txt"}
outputFile := "output.txt"

filteredFiles := common.FilterContextFiles(contextFiles, outputFile)
// Result: ["file1.txt", "file2.txt"]

// In provider implementation
func (p *MyProvider) processContextFiles(options *types.GenerateOptions) error {
    if len(options.ContextFiles) == 0 {
        return nil
    }

    // Filter out output file to avoid duplication
    filteredFiles := common.FilterContextFiles(options.ContextFiles, options.OutputFile)

    // Process filtered files
    for _, file := range filteredFiles {
        content, err := common.ReadFileContent(file)
        if err != nil {
            return fmt.Errorf("failed to read context file %s: %w", file, err)
        }
        // Process content...
    }
    return nil
}
```

### ReadConfigFile

Reads a configuration file and returns its raw byte content.

```go
func ReadConfigFile(configPath string) ([]byte, error)
```

**Parameters:**
- `configPath`: Path to the configuration file

**Returns:** Raw file content as bytes and any error encountered

```go
// Load configuration
data, err := common.ReadConfigFile("app.yaml")
if err != nil {
    log.Fatal(err)
}

var config Config
err = yaml.Unmarshal(data, &config)
if err != nil {
    log.Fatal(err)
}

// In provider initialization
func loadProviderConfig(configPath string) (types.ProviderConfig, error) {
    data, err := common.ReadConfigFile(configPath)
    if err != nil {
        return types.ProviderConfig{}, err
    }

    var config struct {
        Provider types.ProviderConfig `yaml:"provider"`
    }
    err = yaml.Unmarshal(data, &config)
    return config.Provider, err
}
```

---

## Phase 2: Streaming Abstraction

The streaming abstraction provides a unified interface for handling streaming responses across all providers, eliminating provider-specific streaming code.

### Overview

The streaming utilities include:
- **StreamProcessor**: Common streaming functionality with built-in parsing
- **Standard Parsers**: OpenAI-compatible and Anthropic-specific parsers
- **Custom Parser Support**: Easy integration for provider-specific formats
- **Error Handling**: Robust error handling and recovery
- **Thread Safety**: Safe for concurrent use

### StreamProcessor

The core streaming processor handles HTTP streaming responses.

```go
type StreamProcessor struct {
    response *http.Response
    reader   *bufio.Reader
    done     bool
    mutex    sync.Mutex
}
```

#### Creating a StreamProcessor

```go
// Create from HTTP response
resp, err := client.Do(req)
if err != nil {
    return err
}
defer resp.Body.Close()

processor := common.NewStreamProcessor(resp)
```

#### Processing Streaming Data

```go
// Process chunks with custom line processing
for {
    chunk, err := processor.NextChunk(func(line string) (types.ChatCompletionChunk, error, bool) {
        // Custom line processing logic
        if strings.HasPrefix(line, "data: ") {
            data := strings.TrimPrefix(line, "data: ")
            if data == "[DONE]" {
                return types.ChatCompletionChunk{Done: true}, nil, true
            }

            // Parse JSON and return chunk
            var response map[string]interface{}
            if err := json.Unmarshal([]byte(data), &response); err != nil {
                return types.ChatCompletionChunk{}, nil, false
            }

            return common.ExtractChunk(response), nil, false
        }
        return types.ChatCompletionChunk{}, nil, false
    })

    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }

    fmt.Print(chunk.Content)
    if chunk.Done {
        break
    }
}
```

### Standard Stream Parsers

Built-in parsers for common streaming formats.

#### OpenAI-Compatible Stream Parser

```go
// Create OpenAI-compatible stream
stream := common.CreateOpenAIStream(resp)
defer stream.Close()

// Process chunks
for {
    chunk, err := stream.Next()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }

    fmt.Print(chunk.Content)
    if chunk.Done {
        fmt.Printf("\nTokens used: %d\n", chunk.Usage.TotalTokens)
        break
    }
}
```

#### Anthropic Stream Parser

```go
// Create Anthropic stream
stream := common.CreateAnthropicStream(resp)
defer stream.Close()

// Process Anthropic-specific events
for {
    chunk, err := stream.Next()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }

    // Handle content blocks, tool use, etc.
    if chunk.Content != "" {
        fmt.Print(chunk.Content)
    }

    if chunk.Done {
        fmt.Printf("\nTokens used: %d\n", chunk.Usage.TotalTokens)
        break
    }
}
```

#### Custom Stream Parser

```go
// Implement StreamParser interface for custom formats
type CustomStreamParser struct {
    contentField string
    doneField    string
}

func (p *CustomStreamParser) ParseLine(data string) (types.ChatCompletionChunk, bool, error) {
    var response map[string]interface{}
    if err := json.Unmarshal([]byte(data), &response); err != nil {
        return types.ChatCompletionChunk{}, false, err
    }

    chunk := types.ChatCompletionChunk{}

    // Extract content using custom field paths
    if content := p.extractNestedValue(response, p.contentField); content != nil {
        if contentStr, ok := content.(string); ok {
            chunk.Content = contentStr
        }
    }

    // Check if done
    if done := p.extractNestedValue(response, p.doneField); done != nil {
        if doneStr, ok := done.(string); ok && doneStr != "" {
            chunk.Done = true
        }
    }

    return chunk, chunk.Done, nil
}

// Use custom parser
stream := common.CreateCustomStream(resp, &CustomStreamParser{
    contentField: "response.text",
    doneField:    "response.finished",
})
```

### Mock Stream for Testing

```go
// Create mock stream for testing
mockChunks := []types.ChatCompletionChunk{
    {Content: "Hello", Done: false},
    {Content: " world", Done: false},
    {Content: "", Done: true, Usage: types.Usage{TotalTokens: 10}},
}

stream := common.NewMockStream(mockChunks)
defer stream.Close()

// Process like a real stream
for {
    chunk, err := stream.Next()
    if err == io.EOF {
        break
    }
    fmt.Print(chunk.Content)
}
```

### Advanced Streaming Features

#### Context-Aware Streaming

```go
// Create stream that respects context cancellation
stream := common.StreamFromContext(ctx, baseStream)

// Stream will automatically stop if context is cancelled
for {
    chunk, err := stream.Next()
    if err != nil {
        if err == context.Canceled {
            log.Println("Stream cancelled by context")
        }
        break
    }
    fmt.Print(chunk.Content)
}
```

#### Error Stream

```go
// Create stream that immediately returns an error
errStream := common.CreateErrorStream(fmt.Errorf("API unavailable"))

chunk, err := errStream.Next()
// err will contain the error message
```

---

## Phase 2: Authentication Helpers

The authentication helpers provide unified support for API keys and OAuth with automatic failover, eliminating provider-specific authentication code.

### Overview

The authentication utilities include:
- **AuthHelper**: Unified authentication with automatic failover
- **Multi-key Support**: Load balancing and rotation for API keys
- **OAuth Management**: Token refresh and credential management
- **Header Management**: Provider-specific header handling
- **Error Handling**: User-friendly authentication error messages

### AuthHelper

The core authentication helper manages both API keys and OAuth credentials.

```go
type AuthHelper struct {
    ProviderName    string
    KeyManager      *keymanager.KeyManager
    OAuthManager    *oauthmanager.OAuthKeyManager
    HTTPClient      *http.Client
    Config          types.ProviderConfig
}
```

#### Creating an AuthHelper

```go
// Create auth helper
authHelper := common.NewAuthHelper("myprovider", config, client)

// Setup API keys (single and multi-key)
authHelper.SetupAPIKeys()

// Setup OAuth if configured
authHelper.SetupOAuth(refreshFunc)
```

#### Multi-API Key Configuration

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeMyProvider,
    // Single API key
    APIKey: "sk-primary-key",

    // Or multiple API keys with failover
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{
            "sk-key1", "sk-key2", "sk-key3",
        },
    },
}

authHelper := common.NewAuthHelper("myprovider", config, client)
authHelper.SetupAPIKeys()

// Check authentication status
status := authHelper.GetAuthStatus()
fmt.Printf("Authenticated: %v\n", status["authenticated"])
fmt.Printf("Method: %s\n", status["method"])
fmt.Printf("API keys: %d\n", status["api_keys_configured"])
```

#### OAuth Configuration

```go
config := types.ProviderConfig{
    Type: types.ProviderTypeMyProvider,
    OAuthCredentials: []*types.OAuthCredentialSet{
        {
            ID:           "primary",
            ClientID:     "your-client-id",
            ClientSecret: "your-client-secret",
            AccessToken:  "access-token",
            RefreshToken: "refresh-token",
            ExpiresAt:    time.Now().Add(24 * time.Hour),
            Scopes:       []string{"api.access"},
        },
    },
}

// Setup OAuth with refresh function
factory := common.NewRefreshFuncFactory("myprovider", client)
refreshFunc := factory.CreateGenericRefreshFunc("https://api.myprovider.com/oauth/token")

authHelper := common.NewAuthHelper("myprovider", config, client)
authHelper.SetupOAuth(refreshFunc)
```

### Execute with Automatic Failover

```go
// Execute operations with automatic authentication failover
result, usage, err := authHelper.ExecuteWithAuth(ctx, options,
    // OAuth operation
    func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
        return p.makeOAuthRequest(ctx, cred, options)
    },
    // API key operation
    func(ctx context.Context, key string) (string, *types.Usage, error) {
        return p.makeAPIKeyRequest(ctx, key, options)
    },
)

// Automatically tries OAuth first, then API keys with failover
```

### HTTP Request Handling

#### Setting Authentication Headers

```go
req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
if err != nil {
    return nil, err
}

// Set authentication headers (provider-specific)
authHelper.SetAuthHeaders(req, "sk-api-key", "api_key")
// Sets Authorization: Bearer sk-api-key for OpenAI
// Sets x-api-key: sk-api-key for Anthropic
// Sets x-goog-api-key: sk-api-key for Gemini

// Set provider-specific headers
authHelper.SetProviderSpecificHeaders(req)
// Sets anthropic-version for Anthropic
// Sets openai-organization for OpenAI
```

#### Making Authenticated Requests

```go
// Simple authenticated request
resp, err := authHelper.MakeAuthenticatedRequest(ctx, "POST", endpoint,
    map[string]string{"Content-Type": "application/json"}, requestBody)

if err != nil {
    return nil, err
}
defer resp.Body.Close()

// Handle authentication errors
if resp.StatusCode >= 400 {
    return nil, authHelper.HandleAuthError(fmt.Errorf("request failed"), resp.StatusCode)
}
```

### Authentication Monitoring

#### Authentication Status

```go
status := authHelper.GetAuthStatus()
// Returns map[string]interface{}{
//     "provider":     "myprovider",
//     "authenticated": true,
//     "method":       "oauth",
//     "oauth_credentials_count": 1,
//     "api_keys_configured": 3,
// }

// Check if authenticated
if !status["authenticated"].(bool) {
    log.Println("Provider not authenticated")
}

// Check method
method := status["method"].(string)
switch method {
case "oauth":
    log.Println("Using OAuth authentication")
case "api_key":
    log.Println("Using API key authentication")
}
```

#### OAuth Token Refresh

```go
// Refresh all OAuth tokens
err := authHelper.RefreshAllOAuthTokens(ctx)
if err != nil {
    log.Printf("Failed to refresh OAuth tokens: %v", err)
}

// The auth helper automatically handles token refresh during operations
// No manual refresh needed for normal usage
```

---

## Phase 2: OAuth Token Refresh

The OAuth refresh utilities provide pre-built implementations for major providers, eliminating the need to write OAuth token refresh code.

### Overview

The OAuth refresh utilities include:
- **OAuthRefreshHelper**: Provider-specific token refresh implementations
- **RefreshFuncFactory**: Easy creation of refresh functions
- **Major Provider Support**: OpenAI, Anthropic, Gemini, Qwen
- **Generic Refresh**: Support for custom OAuth providers
- **Automatic Integration**: Seamless integration with AuthHelper

### OAuthRefreshHelper

Core helper for OAuth token operations.

```go
type OAuthRefreshHelper struct {
    ProviderName string
    HTTPClient   *http.Client
}
```

#### Creating an OAuth Refresh Helper

```go
helper := common.NewOAuthRefreshHelper("openai", client)
```

### Provider-Specific Refresh Methods

#### OpenAI OAuth Refresh

```go
helper := common.NewOAuthRefreshHelper("openai", client)

updatedCred, err := helper.OpenAIOAuthRefresh(ctx, credential)
if err != nil {
    return fmt.Errorf("failed to refresh OpenAI token: %w", err)
}

// Uses form-encoded format with public client ID
// Automatic token refresh with proper error handling
```

#### Anthropic OAuth Refresh

```go
helper := common.NewOAuthRefreshHelper("anthropic", client)

updatedCred, err := helper.AnthropicOAuthRefresh(ctx, credential)
if err != nil {
    return fmt.Errorf("failed to refresh Anthropic token: %w", err)
}

// Uses JSON format with MAX plan client ID
// Handles Claude Code OAuth flows
```

#### Gemini OAuth Refresh

```go
helper := common.NewOAuthRefreshHelper("gemini", client)

updatedCred, err := helper.GeminiOAuthRefresh(ctx, credential)
if err != nil {
    return fmt.Errorf("failed to refresh Gemini token: %w", err)
}

// Uses oauth2 library with Google endpoints
// Handles service account and OAuth flows
```

#### Qwen OAuth Refresh

```go
helper := common.NewOAuthRefreshHelper("qwen", client)

updatedCred, err := helper.QwenOAuthRefresh(ctx, credential)
if err != nil {
    return fmt.Errorf("failed to refresh Qwen token: %w", err)
}

// Uses form-encoded format with public client ID
// Handles device code OAuth flows
```

#### Generic OAuth Refresh

```go
helper := common.NewOAuthRefreshHelper("custom", client)

updatedCred, err := helper.GenericOAuthRefresh(ctx, credential, "https://api.custom.com/oauth/token")
if err != nil {
    return fmt.Errorf("failed to refresh custom token: %w", err)
}

// Works with any OAuth 2.0 compliant provider
// Uses standard form-encoded refresh request
```

### RefreshFuncFactory

Factory for creating refresh functions with easy integration.

```go
type RefreshFuncFactory struct {
    Helper *OAuthRefreshHelper
}
```

#### Creating Refresh Functions

```go
factory := common.NewRefreshFuncFactory("anthropic", client)

// Create provider-specific refresh functions
anthropicRefresh := factory.CreateAnthropicRefreshFunc()
openaiRefresh := factory.CreateOpenAIRefreshFunc()
geminiRefresh := factory.CreateGeminiRefreshFunc()
qwenRefresh := factory.CreateQwenRefreshFunc()

// Create generic refresh for custom providers
genericRefresh := factory.CreateGenericRefreshFunc("https://api.example.com/oauth/token")

// Use refresh function with OAuth manager
oauthManager := oauthmanager.NewOAuthKeyManager("anthropic", credentials, anthropicRefresh)
```

### Integration with AuthHelper

```go
// Setup OAuth with automatic refresh
config := types.ProviderConfig{
    Type: types.ProviderTypeAnthropic,
    OAuthCredentials: []*types.OAuthCredentialSet{
        {
            ID:           "primary",
            ClientID:     "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
            AccessToken:  "current-token",
            RefreshToken: "refresh-token",
            ExpiresAt:    time.Now().Add(24 * time.Hour),
        },
    },
}

authHelper := common.NewAuthHelper("anthropic", config, client)

// Setup OAuth with automatic refresh
factory := common.NewRefreshFuncFactory("anthropic", client)
refreshFunc := factory.CreateAnthropicRefreshFunc()
authHelper.SetupOAuth(refreshFunc)

// Now all operations will automatically handle token refresh
result, usage, err := authHelper.ExecuteWithAuth(ctx, options,
    func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
        // This will automatically refresh tokens if needed
        return p.makeRequest(ctx, cred.AccessToken, options)
    },
    func(ctx context.Context, key string) (string, *types.Usage, error) {
        return p.makeAPIKeyRequest(ctx, key, options)
    },
)
```

### Custom OAuth Provider Implementation

```go
// Implement custom OAuth refresh for a new provider
func (h *OAuthRefreshHelper) CustomProviderOAuthRefresh(ctx context.Context, cred *types.OAuthCredentialSet, tokenURL string) (*types.OAuthCredentialSet, error) {
    // Custom refresh logic
    formData := url.Values{}
    formData.Set("grant_type", "refresh_token")
    formData.Set("refresh_token", cred.RefreshToken)

    if cred.ClientID != "" {
        formData.Set("client_id", cred.ClientID)
    }
    if cred.ClientSecret != "" {
        formData.Set("client_secret", cred.ClientSecret)
    }

    // Add provider-specific parameters
    formData.Set("provider_specific", "value")

    req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(formData.Encode()))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    resp, err := h.HTTPClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("refresh failed with status %d", resp.StatusCode)
    }

    var tokenResponse struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
        ExpiresIn    int64  `json:"expires_in"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
        return nil, err
    }

    return h.updateCredentialFromResponse(cred, tokenResponse.AccessToken, tokenResponse.RefreshToken, time.Duration(tokenResponse.ExpiresIn)*time.Second), nil
}
```

---

## Phase 2: Configuration Helper

The configuration helper provides standardized configuration validation, extraction, and defaults for all AI providers, eliminating provider-specific configuration code.

### Overview

The configuration utilities include:
- **ConfigHelper**: Unified configuration management
- **Validation**: Comprehensive configuration validation
- **Defaults**: Provider-specific default values
- **Extraction**: Easy extraction of configuration values
- **Sanitization**: Safe logging without sensitive data

### ConfigHelper

Core configuration helper.

```go
type ConfigHelper struct {
    providerName string
    providerType types.ProviderType
}
```

#### Creating a ConfigHelper

```go
// Create config helper for a provider
configHelper := common.NewConfigHelper("openai", types.ProviderTypeOpenAI)

// Validate configuration
validation := configHelper.ValidateProviderConfig(config)
if !validation.Valid {
    return fmt.Errorf("invalid config: %v", validation.Errors)
}
```

### Configuration Validation

#### Basic Validation

```go
validation := configHelper.ValidateProviderConfig(config)

if !validation.Valid {
    for _, err := range validation.Errors {
        log.Printf("Config error: %s", err)
    }
    return fmt.Errorf("configuration validation failed")
}

// Validation checks:
// - Provider type matches
// - Timeout is not negative
// - Max tokens is not negative
// - Other provider-specific validations
```

#### Provider-Specific Validation

```go
// ConfigHelper automatically validates based on provider type
openAIHelper := common.NewConfigHelper("openai", types.ProviderTypeOpenAI)
anthropicHelper := common.NewConfigHelper("anthropic", types.ProviderTypeAnthropic)

// Each helper validates provider-specific requirements
```

### Configuration Extraction

#### Extracting Common Values

```go
// Extract base URL with provider-specific defaults
baseURL := configHelper.ExtractBaseURL(config)
// OpenAI: https://api.openai.com/v1
// Anthropic: https://api.anthropic.com
// Gemini: https://generativelanguage.googleapis.com/v1beta

// Extract default model with provider fallbacks
defaultModel := configHelper.ExtractDefaultModel(config)
// OpenAI: gpt-4o
// Anthropic: claude-3-5-sonnet-20241022
// Gemini: gemini-1.5-pro

// Extract timeout with sensible defaults
timeout := configHelper.ExtractTimeout(config)
// Default: 60 seconds if not specified

// Extract max tokens with provider defaults
maxTokens := configHelper.ExtractMaxTokens(config)
// OpenAI: 4096
// Anthropic: 4096
// Gemini: 8192
```

#### Extracting API Keys

```go
// Extract API keys from multiple sources
keys := configHelper.ExtractAPIKeys(config)
// Returns: ["sk-single", "sk-multi1", "sk-multi2", ...]

// Supports both single API key and multiple API keys
config := types.ProviderConfig{
    APIKey: "sk-single",  // Single key
    ProviderConfig: map[string]interface{}{
        "api_keys": []string{"sk-multi1", "sk-multi2"}, // Multiple keys
    },
}
```

#### Extracting Provider-Specific Configuration

```go
// Extract provider-specific configuration into struct
var openAIConfig struct {
    OrganizationID string `json:"organization_id"`
    ProjectID      string `json:"project_id"`
}

err := configHelper.ExtractProviderSpecificConfig(config, &openAIConfig)
if err != nil {
    return fmt.Errorf("failed to extract provider config: %w", err)
}

// Extract individual fields with fallback
orgID := configHelper.ExtractStringField(config, "organization_id", "default-org")
projectID := configHelper.ExtractStringField(config, "project_id", "default-project")

// Extract string slices
scopes := configHelper.ExtractStringSliceField(config, "scopes")
```

### Configuration Management

#### Applying Defaults and Overrides

```go
// Merge configuration with provider defaults
mergedConfig := configHelper.MergeWithDefaults(config)

// Apply top-level overrides to provider config
var providerConfig struct {
    APIKey  string `json:"api_key"`
    BaseURL string `json:"base_url"`
    Model   string `json:"model"`
}

err := configHelper.ApplyTopLevelOverrides(config, &providerConfig)
if err != nil {
    return err
}

// Automatically applies:
// - APIKey -> api_key
// - BaseURL -> base_url
// - DefaultModel -> model
```

#### Provider Capabilities

```go
// Get provider capabilities
toolCalling, streaming, responsesAPI := configHelper.GetProviderCapabilities()

fmt.Printf("Tool calling: %v\n", toolCalling)
fmt.Printf("Streaming: %v\n", streaming)
fmt.Printf("Responses API: %v\n", responsesAPI)

// Results:
// OpenAI: true, true, true
// Anthropic: true, true, false
// Gemini: true, true, false
```

### Configuration Monitoring

#### Safe Configuration Logging

```go
// Sanitize config for logging (removes sensitive data)
safeConfig := configHelper.SanitizeConfigForLogging(config)

log.Printf("Configuration: %+v", safeConfig)
// API keys and OAuth tokens are removed from logs
```

#### Configuration Summary

```go
// Get human-readable configuration summary
summary := configHelper.ConfigSummary(config)

fmt.Printf("Provider: %s\n", summary["provider"])
fmt.Printf("Base URL: %s\n", summary["base_url"])
fmt.Printf("Default Model: %s\n", summary["default_model"])
fmt.Printf("Auth Methods: %v\n", summary["auth_methods"])
fmt.Printf("Capabilities: %v\n", summary["capabilities"])

// Returns:
// map[string]interface{}{
//     "provider": "openai",
//     "type": "openai",
//     "base_url": "https://api.openai.com/v1",
//     "default_model": "gpt-4o",
//     "timeout": "60s",
//     "max_tokens": 4096,
//     "auth_methods": ["api_key"],
//     "capabilities": map[string]bool{
//         "tool_calling": true,
//         "streaming": true,
//         "responses_api": true,
//     },
// }
```

### Advanced Usage

#### Custom Provider Configuration

```go
// Create config helper for custom provider
type CustomProviderConfig struct {
    APIEndpoint    string            `json:"api_endpoint"`
    APIVersion     string            `json:"api_version"`
    CustomHeaders  map[string]string `json:"custom_headers"`
    RateLimits     map[string]int    `json:"rate_limits"`
}

func NewCustomProvider(config types.ProviderConfig) (*CustomProvider, error) {
    // Create config helper
    configHelper := common.NewConfigHelper("custom", types.ProviderTypeCustom)

    // Validate configuration
    if validation := configHelper.ValidateProviderConfig(config); !validation.Valid {
        return nil, fmt.Errorf("invalid config: %v", validation.Errors)
    }

    // Extract provider-specific configuration
    var providerConfig CustomProviderConfig
    if err := configHelper.ExtractProviderSpecificConfig(config, &providerConfig); err != nil {
        return nil, fmt.Errorf("failed to extract config: %w", err)
    }

    // Apply defaults for missing values
    if providerConfig.APIVersion == "" {
        providerConfig.APIVersion = "v1"
    }

    return &CustomProvider{
        configHelper:   configHelper,
        providerConfig: providerConfig,
    }, nil
}
```

#### Configuration Validation Extension

```go
// Extend validation for custom provider
func (p *CustomProvider) validateCustomConfig() error {
    // Use helper validation first
    validation := p.configHelper.ValidateProviderConfig(p.config)
    if !validation.Valid {
        return fmt.Errorf("base validation failed: %v", validation.Errors)
    }

    // Add custom validations
    if p.providerConfig.APIEndpoint == "" {
        return fmt.Errorf("api_endpoint is required")
    }

    if !strings.HasPrefix(p.providerConfig.APIEndpoint, "https://") {
        return fmt.Errorf("api_endpoint must use HTTPS")
    }

    return nil
}
```

---

## Rate Limit Helper

The RateLimitHelper provides a unified interface for rate limit tracking and enforcement across all providers.

### Overview

The RateLimitHelper encapsulates:
- **Rate Limit Tracking**: Monitor requests and tokens usage
- **Header Parsing**: Extract rate limit information from HTTP headers
- **Automatic Waiting**: Sleep when rate limits are hit
- **Provider-Specific Logic**: Different handling for each provider's rate limits
- **Thread Safety**: Safe for concurrent use

### Creating a RateLimitHelper

```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
)

// Create with provider-specific parser
parser := ratelimit.NewOpenAIParser()
helper := common.NewRateLimitHelper(parser)

// In provider struct
type MyProvider struct {
    rateLimitHelper *common.RateLimitHelper
    // ... other fields
}

func NewMyProvider(config types.ProviderConfig) *MyProvider {
    parser := &MyProviderParser{} // Implement ratelimit.Parser interface
    return &MyProvider{
        rateLimitHelper: common.NewRateLimitHelper(parser),
    }
}
```

### Core Methods

#### CanMakeRequest

Checks if a request can be made for the given model and estimated tokens.

```go
func (h *RateLimitHelper) CanMakeRequest(model string, estimatedTokens int) bool
```

```go
// Before making a request
estimatedTokens := len(prompt) / 4 // Rough estimate
if !helper.CanMakeRequest("gpt-4", estimatedTokens) {
    // Rate limited - need to wait or use different strategy
    return
}

// Proceed with request
resp, err := client.Do(req)
```

#### CheckRateLimitAndWait

Checks rate limits and waits if necessary before making a request.

```go
func (h *RateLimitHelper) CheckRateLimitAndWait(model string, estimatedTokens int) bool
```

```go
// Automatic rate limit handling
func (p *MyProvider) makeRequest(ctx context.Context, req request) (*response, error) {
    estimatedTokens := p.estimateTokens(req)

    // Automatically wait if rate limited
    if !p.rateLimitHelper.CheckRateLimitAndWait(req.model, estimatedTokens) {
        // Rate limit was hit and we waited, but should retry
        return nil, errors.New("rate limited, retry required")
    }

    // Proceed with request
    return p.doHTTPRequest(req)
}
```

#### ParseAndUpdateRateLimits

Parses rate limit headers from an HTTP response and updates the tracker.

```go
func (h *RateLimitHelper) ParseAndUpdateRateLimits(headers http.Header, model string)
```

```go
// After receiving API response
resp, err := client.Do(req)
if err != nil {
    return nil, err
}
defer resp.Body.Close()

// Update rate limits from response headers
p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, req.model)

// Process response...
```

#### GetRateLimitInfo

Retrieves current rate limit information for a model.

```go
func (h *RateLimitHelper) GetRateLimitInfo(model string) (*ratelimit.Info, bool)
```

```go
// Get current rate limit status
if info, exists := p.rateLimitHelper.GetRateLimitInfo("gpt-4"); exists {
    fmt.Printf("Requests remaining: %d/%d\n", info.RequestsRemaining, info.RequestsLimit)
    fmt.Printf("Tokens remaining: %d/%d\n", info.TokensRemaining, info.TokensLimit)
    fmt.Printf("Next reset: %v\n", info.RequestsReset)

    // Make decisions based on rate limits
    if info.RequestsRemaining < 10 {
        log.Printf("Low on requests: %d remaining", info.RequestsRemaining)
    }
}
```

#### ShouldThrottle

Determines if requests should be throttled based on current usage.

```go
func (h *RateLimitHelper) ShouldThrottle(model string, threshold float64) bool
```

```go
// Implement smart throttling
func (p *MyProvider) makeRequestWithThrottling(model string, req request) (*response, error) {
    // Start throttling at 80% of limits
    if p.rateLimitHelper.ShouldThrottle(model, 0.8) {
        // Add delay to avoid hitting limits
        time.Sleep(1 * time.Second)
    }

    return p.makeRequest(model, req)
}
```

### Provider-Specific Rate Limit Parsers

Each provider needs to implement the `ratelimit.Parser` interface:

```go
type Parser interface {
    ProviderName() string
    Parse(headers http.Header, model string) (*Info, error)
}
```

#### Example: Custom Provider Parser

```go
type MyProviderParser struct{}

func (p *MyProviderParser) ProviderName() string {
    return "MyProvider"
}

func (p *MyProviderParser) Parse(headers http.Header, model string) (*ratelimit.Info, error) {
    info := &ratelimit.Info{
        Model: model,
    }

    // Parse standard rate limit headers
    if limit := headers.Get("x-ratelimit-limit-requests"); limit != "" {
        info.RequestsLimit, _ = strconv.Atoi(limit)
    }
    if remaining := headers.Get("x-ratelimit-remaining-requests"); remaining != "" {
        info.RequestsRemaining, _ = strconv.Atoi(remaining)
    }
    if reset := headers.Get("x-ratelimit-reset-requests"); reset != "" {
        if timestamp, err := strconv.ParseInt(reset, 10, 64); err == nil {
            info.RequestsReset = time.Unix(timestamp, 0)
        }
    }

    // Parse token limits
    if tokenLimit := headers.Get("x-ratelimit-limit-tokens"); tokenLimit != "" {
        info.TokensLimit, _ = strconv.Atoi(tokenLimit)
    }
    if tokenRemaining := headers.Get("x-ratelimit-remaining-tokens"); tokenRemaining != "" {
        info.TokensRemaining, _ = strconv.Atoi(tokenRemaining)
    }

    // Handle provider-specific headers
    if dailyLimit := headers.Get("x-ratelimit-limit-daily"); dailyLimit != "" {
        info.DailyRequestsLimit, _ = strconv.Atoi(dailyLimit)
    }
    if dailyRemaining := headers.Get("x-ratelimit-remaining-daily"); dailyRemaining != "" {
        info.DailyRequestsRemaining, _ = strconv.Atoi(dailyRemaining)
    }

    return info, nil
}
```

### Advanced Usage

#### Custom Rate Limit Logic

```go
// Custom rate limit handling
func (p *MyProvider) handleRateLimits(model string, req request) error {
    // Check current limits
    info, exists := p.rateLimitHelper.GetRateLimitInfo(model)
    if !exists {
        // No rate limit info available, proceed with caution
        return nil
    }

    // Custom logic based on provider characteristics
    if info.RequestsRemaining < 5 {
        // Low on requests, use longer delays
        time.Sleep(5 * time.Second)
    } else if info.RequestsRemaining < 20 {
        // Moderate usage, use short delays
        time.Sleep(1 * time.Second)
    }

    // Check if we can make the request
    estimatedTokens := p.estimateTokens(req)
    if !p.rateLimitHelper.CanMakeRequest(model, estimatedTokens) {
        // Wait automatically
        p.rateLimitHelper.CheckRateLimitAndWait(model, estimatedTokens)
    }

    return nil
}
```

#### Rate Limit Monitoring

```go
// Monitor rate limits across all models
func (p *MyProvider) logRateLimitStatus() {
    models := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-sonnet"}

    for _, model := range models {
        if info, exists := p.rateLimitHelper.GetRateLimitInfo(model); exists {
            log.Printf("Model %s: %d/%d requests remaining, %d/%d tokens remaining",
                model, info.RequestsRemaining, info.RequestsLimit,
                info.TokensRemaining, info.TokensLimit)
        }
    }
}
```

---

## Model Cache

The ModelCache provides thread-safe caching of model lists with TTL (Time-To-Live) support.

### Overview

The ModelCache handles:
- **Thread-Safe Access**: Safe for concurrent reads and writes
- **TTL Management**: Automatic expiration of cached data
- **Fallback Handling**: Graceful degradation when API calls fail
- **Cache Invalidation**: Manual cache clearing and refresh

### Creating a ModelCache

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"

// Create with 1 hour TTL
cache := common.NewModelCache(1 * time.Hour)

// Create with 30 minute TTL
cache = common.NewModelCache(30 * time.Minute)

// In provider struct
type MyProvider struct {
    modelCache *common.ModelCache
    // ... other fields
}

func NewMyProvider(config types.ProviderConfig) *MyProvider {
    return &MyProvider{
        modelCache: common.NewModelCache(30 * time.Minute),
    }
}
```

### Core Methods

#### GetModels

Returns cached models if available and fresh, or calls the fetch function.

```go
func (mc *ModelCache) GetModels(
    fetchFunc func() ([]types.Model, error),
    fallbackFunc func() []types.Model
) ([]types.Model, error)
```

```go
func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        // Fetch fresh models from API
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx)
        },
        // Fallback to static list if API fails
        func() []types.Model {
            return []types.Model{
                {ID: "my-model-v1", Name: "My Model v1"},
                {ID: "my-model-v2", Name: "My Model v2"},
            }
        },
    )
}
```

#### Manual Cache Management

```go
// Check if cache is stale
if cache.IsStale() {
    log.Println("Model cache is stale, will refresh on next request")
}

// Get currently cached models (without fetching)
models := cache.Get()
fmt.Printf("Cached %d models\n", len(models))

// Update cache manually
newModels := []types.Model{{ID: "new-model", Name: "New Model"}}
cache.Update(newModels)

// Clear cache entirely
cache.Clear()

// Get cache metadata
timestamp := cache.GetTimestamp()
ttl := cache.GetTTL()
fmt.Printf("Cache updated at: %v, TTL: %v\n", timestamp, ttl)

// Update TTL
cache.SetTTL(2 * time.Hour)
```

### Advanced Usage Patterns

#### Cache with Refresh Strategy

```go
type MyProvider struct {
    modelCache *common.ModelCache
    lastRefresh time.Time
    refreshInterval time.Duration
}

func (p *MyProvider) GetModelsWithRefresh(ctx context.Context) ([]types.Model, error) {
    // Refresh cache if it's old, even if not expired
    if time.Since(p.lastRefresh) > p.refreshInterval {
        err := p.refreshModelCache(ctx)
        if err != nil {
            log.Printf("Failed to refresh model cache: %v", err)
        }
        p.lastRefresh = time.Now()
    }

    return p.modelCache.GetModels(
        func() ([]types.Model, error) { return p.fetchModelsFromAPI(ctx) },
        func() []types.Model { return p.getStaticModels() },
    )
}

func (p *MyProvider) refreshModelCache(ctx context.Context) error {
    models, err := p.fetchModelsFromAPI(ctx)
    if err != nil {
        return err
    }
    p.modelCache.Update(models)
    return nil
}
```

#### Cache with Validation

```go
func (p *MyProvider) GetValidatedModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        func() ([]types.Model, error) {
            models, err := p.fetchModelsFromAPI(ctx)
            if err != nil {
                return nil, err
            }

            // Validate models
            var validModels []types.Model
            for _, model := range models {
                if p.validateModel(model) {
                    validModels = append(validModels, model)
                }
            }

            return validModels, nil
        },
        func() []types.Model {
            // Return validated static models
            return p.getValidStaticModels()
        },
    )
}

func (p *MyProvider) validateModel(model types.Model) bool {
    // Custom validation logic
    return model.ID != "" && model.Name != "" && model.MaxTokens > 0
}
```

---

## Testing Framework

The testing framework provides comprehensive utilities for testing provider implementations.

### Overview

The testing framework includes:
- **Mock Servers**: Configurable HTTP mock servers
- **Provider Helpers**: Common provider test patterns
- **Tool Calling Helpers**: Specialized tool calling tests
- **Configuration Helpers**: Test configuration utilities
- **Authentication Helpers**: Auth flow testing utilities

### Mock Servers

#### Basic Mock Server

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/testing"

// Create mock server
server := testing.NewMockServer(`{
    "choices": [{"message": {"content": "Hello, world!"}}],
    "usage": {"prompt_tokens": 10, "completion_tokens": 5}
}`, 200)

// Set custom headers
server.SetHeader("Content-Type", "application/json")
server.SetHeader("x-ratelimit-limit-requests", "100")
server.SetHeader("x-ratelimit-remaining-requests", "99")

// Start server and get URL
url := server.Start()
defer server.Close()

// Use in provider tests
config := types.ProviderConfig{
    BaseURL: url,
    APIKey:  "test-key",
}
provider := NewMyProvider(config)
```

#### Streaming Mock Server

```go
// Create streaming response chunks
chunks := []string{
    `data: {"choices":[{"delta":{"content":"Hello"}}]}`,
    `data: {"choices":[{"delta":{"content":" world"}}]}`,
    `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
    `data: [DONE]`,
}

// Create streaming mock server
server := testing.CreateStreamingMockServer(chunks)
url := server.Start()
defer server.Close()

// Test streaming
stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "Hello",
    Stream: true,
})
```

### Provider Test Helpers

#### Basic Provider Testing

```go
func TestMyProvider(t *testing.T) {
    config := types.ProviderConfig{
        Type:         types.ProviderTypeMyProvider,
        APIKey:       "test-key",
        DefaultModel: "my-model-v1",
    }
    provider := NewMyProvider(config)

    // Create test helper
    helper := testing.NewProviderTestHelpers(t, provider)

    // Test basic functionality
    t.Run("Basics", func(t *testing.T) {
        helper.AssertProviderBasics("myprovider", "myprovider")
        helper.AssertAuthenticated(true)
        helper.AssertSupportsFeatures(true, true, false)
    })

    // Test model availability
    t.Run("Models", func(t *testing.T) {
        helper.AssertModelExists("my-model-v1")
        helper.AssertDefaultModel("my-model-v1")
    })
}
```

#### Comprehensive Test Suite

```go
func TestMyProviderComprehensive(t *testing.T) {
    config := types.ProviderConfig{
        Type:         types.ProviderTypeMyProvider,
        APIKey:       "test-key",
        DefaultModel: "my-model-v1",
    }
    provider := NewMyProvider(config)

    // Run comprehensive test suite
    testing.RunProviderTests(t, provider, config, []string{
        "my-model-v1",
        "my-model-v2",
    })
}
```

#### Custom Provider Tests

```go
func TestMyProviderSpecificFeatures(t *testing.T) {
    provider := NewMyProvider(testConfig)
    helper := testing.NewProviderTestHelpers(t, provider)

    t.Run("Authentication", func(t *testing.T) {
        authConfig := types.AuthConfig{
            Method: types.AuthMethodAPIKey,
            APIKey: "test-key",
        }
        helper.TestAuthenticationFlow(authConfig)
    })

    t.Run("Configuration", func(t *testing.T) {
        config := types.ProviderConfig{
            Type:         types.ProviderTypeMyProvider,
            APIKey:       "new-key",
            DefaultModel: "different-model",
        }
        helper.TestConfiguration(config)
    })

    t.Run("Health", func(t *testing.T) {
        helper.TestHealthCheck()
    })

    t.Run("Metrics", func(t *testing.T) {
        helper.TestMetrics()
    })
}
```

### Tool Calling Test Helpers

#### Basic Tool Calling Tests

```go
func TestMyProviderToolCalling(t *testing.T) {
    provider := NewMyProvider(testConfig)

    if !provider.SupportsToolCalling() {
        t.Skip("Provider does not support tool calling")
    }

    // Create tool calling test helper
    toolHelper := testing.NewToolCallTestHelper(t)

    // Run standard tool calling test suite
    toolHelper.StandardToolTestSuite(provider)
}
```

#### Custom Tool Tests

```go
func TestMyProviderCustomTools(t *testing.T) {
    provider := NewMyProvider(testConfig)
    toolHelper := testing.NewToolCallTestHelper(t)

    // Create test tools
    tools := []types.Tool{
        testing.CreateTestTool("get_weather", "Get current weather"),
        testing.CreateTestTool("calculator", "Perform calculations"),
    }

    t.Run("ToolChoiceModes", func(t *testing.T) {
        toolHelper.TestToolChoiceModes(provider, tools)
    })

    t.Run("ParallelToolCalls", func(t *testing.T) {
        toolHelper.TestParallelToolCalls(provider)
    })

    t.Run("StreamingToolCalls", func(t *testing.T) {
        toolHelper.TestStreamingToolCalls(provider, tools)
    })
}
```

### Test Utilities

#### Creating Test Data

```go
// Create test tool
tool := testing.CreateTestTool("my_tool", "My test tool")

// Create test tool call
toolCall := testing.CreateTestToolCall("call_123", "my_tool", `{"param":"value"}`)

// Create mock chat completion response
response := testing.CreateMockChatCompletionResponse(
    "gpt-4",
    "Hello, world!",
    []types.ToolCall{toolCall},
)

// Create mock streaming response
streamingChunks := testing.CreateMockStreamResponse(
    "gpt-4",
    "Hello, world!",
    []types.ToolCall{toolCall},
)
```

#### Assertion Helpers

```go
// Assert string contains substring
testing.AssertContains(t, response, "expected content")

// Clean code response (remove markdown)
cleaned := testing.CleanCodeResponse("```python\nprint('hello')\n```")
// Result: "print('hello')"

// Test provider interface compliance
testing.TestProviderInterface(t, provider)
```

### Integration Testing

#### Mock Server Integration

```go
func TestMyProviderWithMockServer(t *testing.T) {
    // Create mock server with custom response
    mockResponse := `{
        "id": "chatcmpl-test",
        "model": "my-model-v1",
        "choices": [{
            "message": {"content": "Test response"},
            "finish_reason": "stop"
        }],
        "usage": {"total_tokens": 15}
    }`

    server := testing.NewMockServer(mockResponse, 200)
    server.SetHeader("Content-Type", "application/json")
    server.SetHeader("x-ratelimit-limit-requests", "100")
    server.SetHeader("x-ratelimit-remaining-requests", "99")
    url := server.Start()
    defer server.Close()

    // Configure provider to use mock server
    config := types.ProviderConfig{
        Type:    types.ProviderTypeMyProvider,
        BaseURL: url,
        APIKey:  "test-key",
    }

    provider := NewMyProvider(config)
    ctx := context.Background()

    // Test the provider
    stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
        Prompt: "Test prompt",
        Model:  "my-model-v1",
    })
    require.NoError(t, err)
    defer stream.Close()

    chunk, err := stream.Next()
    require.NoError(t, err)
    assert.Equal(t, "Test response", chunk.Content)
    assert.Equal(t, int64(15), chunk.Usage.TotalTokens)
}
```

---

## Provider Development Guide with Phase 2

This section provides a comprehensive guide for developing new providers using all the Phase 2 shared utilities, resulting in minimal code with maximum functionality.

### Complete Provider with All Phase 2 Utilities

Here's a modern provider implementation using all Phase 2 utilities:

```go
package myprovider

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type MyProvider struct {
    // Phase 2 utilities - all the heavy lifting is done by these helpers
    authHelper   *common.AuthHelper      // Handles API keys, OAuth, and failover
    configHelper *common.ConfigHelper    // Handles validation, defaults, and extraction
    customizer   common.ProviderCustomizer // Handles prompt formatting

    // HTTP client with connection pooling
    client *http.Client

    // Basic provider info
    config types.ProviderConfig
}

func NewMyProvider(config types.ProviderConfig) (*MyProvider, error) {
    // 1. Setup configuration helper for validation and defaults
    configHelper := common.NewConfigHelper("myprovider", types.ProviderTypeMyProvider)

    // Validate configuration
    validation := configHelper.ValidateProviderConfig(config)
    if !validation.Valid {
        return nil, fmt.Errorf("invalid configuration: %v", validation.Errors)
    }

    // Create optimized HTTP client
    client := &http.Client{
        Timeout: configHelper.ExtractTimeout(config),
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 10,
            IdleConnTimeout:     90 * time.Second,
            DisableCompression:  false,
        },
    }

    // 2. Setup authentication helper for API keys and OAuth
    authHelper := common.NewAuthHelper("myprovider", config, client)
    authHelper.SetupAPIKeys()

    // Setup OAuth if configured
    if len(config.OAuthCredentials) > 0 {
        factory := common.NewRefreshFuncFactory("myprovider", client)
        refreshFunc := factory.CreateGenericRefreshFunc("https://api.myprovider.com/oauth/token")
        authHelper.SetupOAuth(refreshFunc)
    }

    // 3. Create provider customizer for prompt formatting
    customizer := &MyProviderCustomizer{}

    return &MyProvider{
        authHelper:   authHelper,
        configHelper: configHelper,
        customizer:   customizer,
        client:       client,
        config:       config,
    }, nil
}
```

// Required interface methods
func (p *MyProvider) Name() string { return "myprovider" }
func (p *MyProvider) Type() types.ProviderType { return types.ProviderTypeMyProvider }
func (p *MyProvider) Description() string { return "My Custom AI Provider" }
func (p *MyProvider) GetDefaultModel() string { return p.config.DefaultModel }
func (p *MyProvider) GetConfig() types.ProviderConfig { return p.config }

// Model management with caching
func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx)
        },
        func() []types.Model {
            return p.getStaticModels()
        },
    )
}

// Rate limit-aware request handling
func (p *MyProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Process context files with language detection
    if err := p.processContextFiles(&options); err != nil {
        return nil, err
    }

    // Estimate tokens for rate limiting
    estimatedTokens := p.estimateTokens(options)

    // Check rate limits and wait if necessary
    if !p.rateLimitHelper.CanMakeRequest(options.Model, estimatedTokens) {
        p.rateLimitHelper.CheckRateLimitAndWait(options.Model, estimatedTokens)
    }

    // Make request
    resp, err := p.makeRequest(ctx, options)
    if err != nil {
        return nil, err
    }

    // Update rate limits from response headers
    p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, options.Model)

    // Process response
    return p.handleResponse(resp, options.Stream)
}

// Context file processing using shared utilities
func (p *MyProvider) processContextFiles(options *types.GenerateOptions) error {
    if len(options.ContextFiles) == 0 {
        return nil
    }

    // Filter out output file from context
    filteredFiles := common.FilterContextFiles(options.ContextFiles, options.OutputFile)

    var contextBuilder strings.Builder
    for _, file := range filteredFiles {
        // Detect language for potential syntax highlighting
        language := common.DetectLanguage(file)

        // Read file content
        content, err := common.ReadFileContent(file)
        if err != nil {
            return fmt.Errorf("failed to read context file %s: %w", file, err)
        }

        // Add to context with language annotation
        contextBuilder.WriteString(fmt.Sprintf("// File: %s (Language: %s)\n", file, language))
        contextBuilder.WriteString(content)
        contextBuilder.WriteString("\n\n")
    }

    // Add context to prompt
    if contextBuilder.Len() > 0 {
        if options.Prompt == "" {
            options.Prompt = contextBuilder.String()
        } else {
            options.Prompt = contextBuilder.String() + "\n\n" + options.Prompt
        }
    }

    return nil
}

// Implement other required methods...
```

### Provider Registration

```go
package myprovider

import "github.com/cecil-the-coder/ai-provider-kit/pkg/factory"

func RegisterWithFactory(f *factory.ProviderFactory) {
    f.RegisterProvider(types.ProviderTypeMyProvider, func(config types.ProviderConfig) types.Provider {
        return NewMyProvider(config)
    })
}
```

### Testing Your Provider

```go
package myprovider

import (
    "testing"
    "github.com/stretchr/testify/require"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/testing"
)

func TestMyProvider(t *testing.T) {
    config := types.ProviderConfig{
        Type:         types.ProviderTypeMyProvider,
        APIKey:       "test-key",
        DefaultModel: "my-model-v1",
    }

    provider := NewMyProvider(config)

    // Run comprehensive test suite
    t.Run("Comprehensive", func(t *testing.T) {
        testing.RunProviderTests(t, provider, config, []string{
            "my-model-v1",
            "my-model-v2",
        })
    })

    // Test tool calling if supported
    if provider.SupportsToolCalling() {
        t.Run("ToolCalling", func(t *testing.T) {
            toolHelper := testing.NewToolCallTestHelper(t)
            toolHelper.StandardToolTestSuite(provider)
        })
    }
}
```

---

## Migration Guide

This section helps you migrate existing provider implementations to use the common utilities.

### Before Migration

```go
// Old implementation without shared utilities
type OldProvider struct {
    config     types.ProviderConfig
    client     *http.Client
    models     []types.Model
    modelsLock sync.Mutex
    lastUpdate time.Time
}

func (p *OldProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    p.modelsLock.Lock()
    defer p.modelsLock.Unlock()

    // Manual cache checking
    if time.Since(p.lastUpdate) < 30*time.Minute && len(p.models) > 0 {
        return p.models, nil
    }

    // Manual API call
    resp, err := p.client.Get(p.baseURL + "/models")
    if err != nil {
        return p.models, nil // Return stale cache on error
    }
    defer resp.Body.Close()

    // Manual parsing
    var result struct {
        Data []Model `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return p.models, nil // Return stale cache on error
    }

    // Manual conversion
    p.models = make([]types.Model, len(result.Data))
    for i, m := range result.Data {
        p.models[i] = types.Model{
            ID:   m.ID,
            Name: m.ID,
            // ... other fields
        }
    }
    p.lastUpdate = time.Now()
    return p.models, nil
}

func (p *OldProvider) processFile(filename string) (string, error) {
    // Manual language detection
    ext := strings.ToLower(filepath.Ext(filename))
    var language string
    switch ext {
    case ".go":
        language = "go"
    case ".py":
        language = "python"
    case ".js":
        language = "javascript"
    default:
        language = "text"
    }

    // Manual file reading
    data, err := os.ReadFile(filename)
    if err != nil {
        return "", err
    }
    content := string(data)

    return fmt.Sprintf("// File: %s (Language: %s)\n%s", filename, language, content), nil
}
```

### After Migration

```go
// New implementation with shared utilities
type NewProvider struct {
    config          types.ProviderConfig
    client          *http.Client
    modelCache      *common.ModelCache
    rateLimitHelper *common.RateLimitHelper
}

func (p *NewProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    // Use shared model cache
    return p.modelCache.GetModels(
        // Fetch fresh models from API
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx)
        },
        // Fallback to static list if API fails
        func() []types.Model {
            return p.getStaticModels()
        },
    )
}

func (p *NewProvider) processFile(filename string) (string, error) {
    // Use shared language detection
    language := common.DetectLanguage(filename)

    // Use shared file reading
    content, err := common.ReadFileContent(filename)
    if err != nil {
        return "", err
    }

    return fmt.Sprintf("// File: %s (Language: %s)\n%s", filename, language, content), nil
}

func (p *NewProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
    resp, err := p.client.Get(p.baseURL + "/models")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result struct {
        Data []Model `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    models := make([]types.Model, len(result.Data))
    for i, m := range result.Data {
        models[i] = types.Model{
            ID:   m.ID,
            Name: m.ID,
            // ... other fields
        }
    }
    return models, nil
}

func (p *NewProvider) getStaticModels() []types.Model {
    return []types.Model{
        {ID: "fallback-model", Name: "Fallback Model"},
    }
}
```

### Migration Benefits

1. **Reduced Code**: ~50 lines of code reduced to ~15 lines
2. **Better Error Handling**: Automatic fallback and stale cache handling
3. **Thread Safety**: Built-in thread-safe operations
4. **Consistency**: Standardized behavior across all providers
5. **Testing**: Built-in test helpers reduce testing overhead
6. **Maintenance**: Centralized bug fixes and improvements

### Migration Steps

1. **Add Shared Utilities Import**
   ```go
   import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
   ```

2. **Replace Model Cache**
   - Remove manual caching logic
   - Add `modelCache *common.ModelCache` field
   - Use `common.NewModelCache(ttl)` in constructor
   - Replace `GetModels()` with `modelCache.GetModels()`

3. **Replace File Operations**
   - Replace manual language detection with `common.DetectLanguage()`
   - Replace file reading with `common.ReadFileContent()`
   - Use `common.FilterContextFiles()` for context file filtering

4. **Add Rate Limiting**
   - Add `rateLimitHelper *common.RateLimitHelper` field
   - Create provider-specific parser implementing `ratelimit.Parser`
   - Use `rateLimitHelper.CheckRateLimitAndWait()` before requests
   - Use `rateLimitHelper.ParseAndUpdateRateLimits()` after responses

5. **Update Tests**
   - Use `testing.NewProviderTestHelpers()` for basic tests
   - Use `testing.RunProviderTests()` for comprehensive testing
   - Use `testing.NewMockServer()` for integration tests

6. **Verify Functionality**
   - Run existing tests to ensure no regressions
   - Add new tests using shared testing utilities
   - Verify rate limiting and caching behavior

---

## Best Practices

This section covers best practices for using the common utilities effectively.

### Rate Limiting Best Practices

#### Always Check Rate Limits

```go
// Good: Check rate limits before every request
func (p *MyProvider) makeRequest(ctx context.Context, req request) (*response, error) {
    estimatedTokens := p.estimateTokens(req)

    if !p.rateLimitHelper.CanMakeRequest(req.model, estimatedTokens) {
        p.rateLimitHelper.CheckRateLimitAndWait(req.model, estimatedTokens)
    }

    // Make request...
}

// Bad: Ignore rate limits
func (p *MyProvider) makeRequest(ctx context.Context, req request) (*response, error) {
    // Direct request without rate limit checking
    return p.client.Do(req)
}
```

#### Update Rate Limits After Every Response

```go
// Good: Always update rate limits from response headers
resp, err := p.client.Do(req)
if err != nil {
    return nil, err
}
defer resp.Body.Close()

// Update rate limits
p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, req.model)

// Process response...

// Bad: Forget to update rate limits
resp, err := p.client.Do(req)
if err != nil {
    return nil, err
}
// Missing rate limit update!
```

#### Implement Provider-Specific Logic

```go
// Good: Handle provider-specific rate limit characteristics
func (p *MyProviderParser) Parse(headers http.Header, model string) (*ratelimit.Info, error) {
    info := &ratelimit.Info{Model: model}

    // Parse standard headers
    if limit := headers.Get("x-ratelimit-limit-requests"); limit != "" {
        info.RequestsLimit, _ = strconv.Atoi(limit)
    }

    // Handle provider-specific headers
    if myProviderHeader := headers.Get("x-myprovider-special-limit"); myProviderHeader != "" {
        info.CustomData = map[string]interface{}{
            "special_limit": myProviderHeader,
        }
    }

    return info, nil
}
```

### Caching Best Practices

#### Use Appropriate TTL Values

```go
// Good: Choose TTL based on how often models change
func NewMyProvider(config types.ProviderConfig) *MyProvider {
    // Models rarely change, use longer TTL
    modelCache := common.NewModelCache(1 * time.Hour)

    return &MyProvider{
        modelCache: modelCache,
    }
}

// Bad: Use very short TTL for models that rarely change
func NewMyProvider(config types.ProviderConfig) *MyProvider {
    // 5 minutes is too short for model lists
    modelCache := common.NewModelCache(5 * time.Minute)

    return &MyProvider{
        modelCache: modelCache,
    }
}
```

#### Always Provide Fallback

```go
// Good: Always provide fallback function
func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx)
        },
        func() []types.Model {
            // Return static fallback models
            return []types.Model{
                {ID: "fallback-model", Name: "Fallback Model"},
            }
        },
    )
}

// Bad: No fallback function
func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx) // If this fails, no models returned
        },
        nil, // No fallback!
    )
}
```

#### Handle Cache Staleness

```go
// Good: Check cache freshness and refresh if needed
func (p *MyProvider) ensureFreshModels(ctx context.Context) error {
    if p.modelCache.IsStale() {
        // Try to refresh in background
        go func() {
            models, err := p.fetchModelsFromAPI(ctx)
            if err == nil {
                p.modelCache.Update(models)
            }
        }()
    }
    return nil
}
```

### File Processing Best Practices

#### Always Use Language Detection

```go
// Good: Use language detection for better context
func (p *MyProvider) processContextFiles(files []string) (string, error) {
    var builder strings.Builder

    for _, file := range files {
        language := common.DetectLanguage(file)
        content, err := common.ReadFileContent(file)
        if err != nil {
            return "", err
        }

        // Include language information
        builder.WriteString(fmt.Sprintf("// File: %s (Language: %s)\n", file, language))
        builder.WriteString(content)
        builder.WriteString("\n\n")
    }

    return builder.String(), nil
}

// Bad: Ignore language information
func (p *MyProvider) processContextFiles(files []string) (string, error) {
    var builder strings.Builder

    for _, file := range files {
        content, err := common.ReadFileContent(file)
        if err != nil {
            return "", err
        }

        builder.WriteString(content) // No language context
        builder.WriteString("\n\n")
    }

    return builder.String(), nil
}
```

#### Filter Context Files

```go
// Good: Always filter out output file
func (p *MyProvider) prepareContext(options *types.GenerateOptions) error {
    if len(options.ContextFiles) == 0 {
        return nil
    }

    // Filter out output file to avoid duplication
    filteredFiles := common.FilterContextFiles(options.ContextFiles, options.OutputFile)

    // Process filtered files
    return p.processFiles(filteredFiles)
}
```

### Testing Best Practices

#### Use Comprehensive Test Helpers

```go
// Good: Use built-in test helpers
func TestMyProvider(t *testing.T) {
    config := types.ProviderConfig{
        Type:         types.ProviderTypeMyProvider,
        APIKey:       "test-key",
        DefaultModel: "my-model-v1",
    }
    provider := NewMyProvider(config)

    // Run comprehensive test suite
    testing.RunProviderTests(t, provider, config, []string{
        "my-model-v1",
        "my-model-v2",
    })
}

// Bad: Manual testing implementation
func TestMyProvider(t *testing.T) {
    provider := NewMyProvider(config)

    // Manual tests - incomplete and error-prone
    if provider.Name() != "myprovider" {
        t.Error("Wrong name")
    }
    // ... many more manual tests needed
}
```

#### Use Mock Servers for Integration Tests

```go
// Good: Use mock servers for isolated testing
func TestMyProviderIntegration(t *testing.T) {
    // Create mock server
    server := testing.NewMockServer(mockResponse, 200)
    server.SetHeader("Content-Type", "application/json")
    url := server.Start()
    defer server.Close()

    // Configure provider to use mock
    config := types.ProviderConfig{
        BaseURL: url,
        APIKey:  "test-key",
    }
    provider := NewMyProvider(config)

    // Test against mock
    stream, err := provider.GenerateChatCompletion(ctx, testOptions)
    require.NoError(t, err)
    // ... assertions
}
```

### Error Handling Best Practices

#### Handle Errors Gracefully

```go
// Good: Handle errors with context
func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        func() ([]types.Model, error) {
            models, err := p.fetchModelsFromAPI(ctx)
            if err != nil {
                log.Printf("Failed to fetch models from API: %v", err)
                return nil, err // Return error to trigger fallback
            }
            return models, nil
        },
        func() []types.Model {
            log.Printf("Using fallback model list")
            return p.getStaticModels()
        },
    )
}

// Bad: Ignore errors
func (p *MyProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    models, _ := p.modelCache.GetModels(
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx) // Error ignored
        },
        func() []types.Model {
            return p.getStaticModels()
        },
    )
    return models, nil
}
```

#### Provide Meaningful Error Messages

```go
// Good: Provide context in error messages
func (p *MyProvider) processContextFiles(options *types.GenerateOptions) error {
    for _, file := range options.ContextFiles {
        content, err := common.ReadFileContent(file)
        if err != nil {
            return fmt.Errorf("failed to read context file %s: %w", file, err)
        }
        // ...
    }
    return nil
}

// Bad: Generic error messages
func (p *MyProvider) processContextFiles(options *types.GenerateOptions) error {
    for _, file := range options.ContextFiles {
        content, err := common.ReadFileContent(file)
        if err != nil {
            return errors.New("failed to read file") // Which file?
        }
        // ...
    }
    return nil
}
```

---

## Examples

This section provides practical examples of using the common utilities in real-world scenarios.

### Example 1: Complete Provider Implementation

```go
package exampleprovider

import (
    "context"
    "fmt"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/ratelimit"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type ExampleProvider struct {
    config          types.ProviderConfig
    client          *http.Client
    rateLimitHelper *common.RateLimitHelper
    modelCache      *common.ModelCache
    mutex           sync.RWMutex
}

func NewExampleProvider(config types.ProviderConfig) *ExampleProvider {
    return &ExampleProvider{
        config:          config,
        client:          &http.Client{Timeout: 30 * time.Second},
        rateLimitHelper: common.NewRateLimitHelper(&ExampleParser{}),
        modelCache:      common.NewModelCache(1 * time.Hour),
    }
}

// Basic provider information
func (p *ExampleProvider) Name() string        { return "example" }
func (p *ExampleProvider) Type() types.ProviderType { return "example" }
func (p *ExampleProvider) Description() string { return "Example Provider using shared utilities" }
func (p *ExampleProvider) GetDefaultModel() string { return p.config.DefaultModel }
func (p *ExampleProvider) GetConfig() types.ProviderConfig { return p.config }

// Feature support
func (p *ExampleProvider) SupportsToolCalling() bool  { return true }
func (p *ExampleProvider) SupportsStreaming() bool   { return true }
func (p *ExampleProvider) SupportsResponsesAPI() bool { return false }
func (p *ExampleProvider) GetToolFormat() types.ToolFormat { return types.ToolFormatOpenAI }

// Model management with caching
func (p *ExampleProvider) GetModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        func() ([]types.Model, error) {
            return p.fetchModelsFromAPI(ctx)
        },
        func() []types.Model {
            return []types.Model{
                {
                    ID:                 "example-model-v1",
                    Name:               "Example Model v1",
                    Provider:           p.Type(),
                    SupportsStreaming:  true,
                    SupportsToolCalling: true,
                    MaxTokens:          4096,
                },
                {
                    ID:                 "example-model-v2",
                    Name:               "Example Model v2",
                    Provider:           p.Type(),
                    SupportsStreaming:  true,
                    SupportsToolCalling: true,
                    MaxTokens:          8192,
                },
            }
        },
    )
}

func (p *ExampleProvider) fetchModelsFromAPI(ctx context.Context) ([]types.Model, error) {
    url := fmt.Sprintf("%s/models", p.config.BaseURL)
    resp, err := p.client.Get(url)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch models: %w", err)
    }
    defer resp.Body.Close()

    var response struct {
        Data []struct {
            ID      string `json:"id"`
            Object  string `json:"object"`
            Created int64  `json:"created"`
        } `json:"data"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, fmt.Errorf("failed to decode models response: %w", err)
    }

    models := make([]types.Model, len(response.Data))
    for i, model := range response.Data {
        models[i] = types.Model{
            ID:                 model.ID,
            Name:               model.ID,
            Provider:           p.Type(),
            SupportsStreaming:  true,
            SupportsToolCalling: true,
            MaxTokens:          p.inferTokenLimit(model.ID),
        }
    }

    return models, nil
}

func (p *ExampleProvider) inferTokenLimit(modelID string) int {
    // Simple heuristic based on model name
    if strings.Contains(modelID, "v2") {
        return 8192
    }
    return 4096
}

// Rate limit-aware request handling
func (p *ExampleProvider) GenerateChatCompletion(ctx context.Context, options types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Process context files with language detection
    if err := p.processContextFiles(&options); err != nil {
        return nil, fmt.Errorf("failed to process context files: %w", err)
    }

    // Estimate tokens for rate limiting
    estimatedTokens := p.estimateTokens(options)

    // Check rate limits and wait if necessary
    if !p.rateLimitHelper.CanMakeRequest(options.Model, estimatedTokens) {
        p.rateLimitHelper.CheckRateLimitAndWait(options.Model, estimatedTokens)
    }

    // Build and make request
    req, err := p.buildRequest(options)
    if err != nil {
        return nil, fmt.Errorf("failed to build request: %w", err)
    }

    resp, err := p.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to make request: %w", err)
    }

    // Update rate limits from response headers
    p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, options.Model)

    // Process response
    if options.Stream {
        return p.handleStreamingResponse(resp)
    }
    return p.handleNonStreamingResponse(resp)
}

func (p *ExampleProvider) processContextFiles(options *types.GenerateOptions) error {
    if len(options.ContextFiles) == 0 {
        return nil
    }

    // Filter out output file from context
    filteredFiles := common.FilterContextFiles(options.ContextFiles, options.OutputFile)

    var contextBuilder strings.Builder
    for _, file := range filteredFiles {
        // Detect language for potential syntax highlighting
        language := common.DetectLanguage(file)

        // Read file content
        content, err := common.ReadFileContent(file)
        if err != nil {
            return fmt.Errorf("failed to read context file %s: %w", file, err)
        }

        // Add to context with language annotation
        contextBuilder.WriteString(fmt.Sprintf("// File: %s (Language: %s)\n", file, language))
        contextBuilder.WriteString(content)
        contextBuilder.WriteString("\n\n")
    }

    // Add context to prompt
    if contextBuilder.Len() > 0 {
        if options.Prompt == "" {
            options.Prompt = contextBuilder.String()
        } else {
            options.Prompt = contextBuilder.String() + "\n\n" + options.Prompt
        }
    }

    return nil
}

func (p *ExampleProvider) estimateTokens(options types.GenerateOptions) int {
    // Rough estimation: 1 token ≈ 4 characters
    totalLength := len(options.Prompt)
    for _, msg := range options.Messages {
        totalLength += len(msg.Content)
    }
    return totalLength / 4
}

// Rate limit parser implementation
type ExampleParser struct{}

func (p *ExampleParser) ProviderName() string {
    return "ExampleProvider"
}

func (p *ExampleParser) Parse(headers http.Header, model string) (*ratelimit.Info, error) {
    info := &ratelimit.Info{
        Model: model,
    }

    // Parse standard rate limit headers
    if limit := headers.Get("x-ratelimit-limit-requests"); limit != "" {
        info.RequestsLimit, _ = strconv.Atoi(limit)
    }
    if remaining := headers.Get("x-ratelimit-remaining-requests"); remaining != "" {
        info.RequestsRemaining, _ = strconv.Atoi(remaining)
    }
    if reset := headers.Get("x-ratelimit-reset-requests"); reset != "" {
        if timestamp, err := strconv.ParseInt(reset, 10, 64); err == nil {
            info.RequestsReset = time.Unix(timestamp, 0)
        }
    }

    // Parse token limits
    if tokenLimit := headers.Get("x-ratelimit-limit-tokens"); tokenLimit != "" {
        info.TokensLimit, _ = strconv.Atoi(tokenLimit)
    }
    if tokenRemaining := headers.Get("x-ratelimit-remaining-tokens"); tokenRemaining != "" {
        info.TokensRemaining, _ = strconv.Atoi(tokenRemaining)
    }

    return info, nil
}

// Implement other required interface methods...
```

### Example 2: Comprehensive Test Suite

```go
package exampleprovider

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/testing"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

func TestExampleProvider(t *testing.T) {
    config := types.ProviderConfig{
        Type:         types.ProviderTypeExample,
        APIKey:       "test-api-key",
        DefaultModel: "example-model-v1",
        BaseURL:      "https://api.example.com/v1",
    }

    provider := NewExampleProvider(config)

    // Run comprehensive test suite
    t.Run("Comprehensive", func(t *testing.T) {
        testing.RunProviderTests(t, provider, config, []string{
            "example-model-v1",
            "example-model-v2",
        })
    })

    // Test tool calling
    if provider.SupportsToolCalling() {
        t.Run("ToolCalling", func(t *testing.T) {
            toolHelper := testing.NewToolCallTestHelper(t)
            toolHelper.StandardToolTestSuite(provider)
        })
    }
}

func TestExampleProviderWithMockServer(t *testing.T) {
    // Create mock server response
    mockResponse := `{
        "id": "chatcmpl-test",
        "object": "chat.completion",
        "created": 1234567890,
        "model": "example-model-v1",
        "choices": [{
            "index": 0,
            "message": {
                "role": "assistant",
                "content": "Hello, world!"
            },
            "finish_reason": "stop"
        }],
        "usage": {
            "prompt_tokens": 10,
            "completion_tokens": 5,
            "total_tokens": 15
        }
    }`

    // Create mock server
    server := testing.NewMockServer(mockResponse, 200)
    server.SetHeader("Content-Type", "application/json")
    server.SetHeader("x-ratelimit-limit-requests", "100")
    server.SetHeader("x-ratelimit-remaining-requests", "99")
    url := server.Start()
    defer server.Close()

    // Configure provider to use mock server
    config := types.ProviderConfig{
        Type:    types.ProviderTypeExample,
        BaseURL: url,
        APIKey:  "test-key",
    }

    provider := NewExampleProvider(config)
    ctx := context.Background()

    // Test basic completion
    t.Run("BasicCompletion", func(t *testing.T) {
        stream, err := provider.GenerateChatCompletion(ctx, types.GenerateOptions{
            Prompt: "Hello, world!",
            Model:  "example-model-v1",
            Stream: false,
        })
        require.NoError(t, err)
        defer stream.Close()

        chunk, err := stream.Next()
        require.NoError(t, err)
        assert.Equal(t, "Hello, world!", chunk.Content)
        assert.Equal(t, int64(15), chunk.Usage.TotalTokens)
    })

    // Test streaming completion
    t.Run("StreamingCompletion", func(t *testing.T) {
        chunks := []string{
            `data: {"choices":[{"delta":{"content":"Hello"}}]}`,
            `data: {"choices":[{"delta":{"content":" world"}}]}`,
            `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
            `data: [DONE]`,
        }

        streamingServer := testing.CreateStreamingMockServer(chunks)
        streamingURL := streamingServer.Start()
        defer streamingServer.Close()

        config.BaseURL = streamingURL
        streamingProvider := NewExampleProvider(config)

        stream, err := streamingProvider.GenerateChatCompletion(ctx, types.GenerateOptions{
            Prompt: "Hello",
            Model:  "example-model-v1",
            Stream: true,
        })
        require.NoError(t, err)
        defer stream.Close()

        var content strings.Builder
        for {
            chunk, err := stream.Next()
            if err != nil {
                break
            }
            content.WriteString(chunk.Content)
            if chunk.Done {
                break
            }
        }

        assert.Equal(t, "Hello world", content.String())
    })
}

func TestExampleProviderRateLimiting(t *testing.T) {
    config := types.ProviderConfig{
        Type:         types.ProviderTypeExample,
        APIKey:       "test-key",
        DefaultModel: "example-model-v1",
    }

    provider := NewExampleProvider(config)

    t.Run("RateLimitInfo", func(t *testing.T) {
        // Initially no rate limit info
        info, exists := provider.rateLimitHelper.GetRateLimitInfo("example-model-v1")
        assert.False(t, exists)
        assert.Nil(t, info)

        // Simulate rate limit update
        // In real implementation, this would be called from HTTP response processing
        // Here we'll test the underlying functionality
    })
}

func TestExampleProviderModelCache(t *testing.T) {
    config := types.ProviderConfig{
        Type:         types.ProviderTypeExample,
        APIKey:       "test-key",
        DefaultModel: "example-model-v1",
    }

    provider := NewExampleProvider(config)

    t.Run("ModelCache", func(t *testing.T) {
        // Initially cache should be empty
        assert.True(t, provider.modelCache.IsStale())

        // Get models (should use fallback)
        models, err := provider.GetModels(context.Background())
        require.NoError(t, err)
        assert.Len(t, models, 2) // Should have fallback models

        // Cache should now be populated
        assert.False(t, provider.modelCache.IsStale())

        // Get models again (should use cache)
        models2, err := provider.GetModels(context.Background())
        require.NoError(t, err)
        assert.Equal(t, models, models2)
    })
}

func TestExampleProviderContextFileProcessing(t *testing.T) {
    config := types.ProviderConfig{
        Type:         types.ProviderTypeExample,
        APIKey:       "test-key",
        DefaultModel: "example-model-v1",
    }

    provider := NewExampleProvider(config)

    t.Run("ContextFileProcessing", func(t *testing.T) {
        // Create temporary test files
        tmpDir := t.TempDir()

        goFile := tmpDir + "/test.go"
        err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {}"), 0644)
        require.NoError(t, err)

        pyFile := tmpDir + "/test.py"
        err = os.WriteFile(pyFile, []byte("print('hello')"), 0644)
        require.NoError(t, err)

        options := types.GenerateOptions{
            ContextFiles: []string{goFile, pyFile, outputFile},
            OutputFile:   outputFile, // This should be filtered out
        }

        err = provider.processContextFiles(&options)
        require.NoError(t, err)

        // Check that prompt contains language annotations
        assert.Contains(t, options.Prompt, "Language: go")
        assert.Contains(t, options.Prompt, "Language: python")
        assert.Contains(t, options.Prompt, "package main")
        assert.Contains(t, options.Prompt, "print('hello')")
    })
}
```

### Example 3: Advanced Rate Limit Handling

```go
package exampleprovider

import (
    "fmt"
    "log"
    "time"
)

// Advanced rate limit handling with custom logic
func (p *ExampleProvider) makeRequestWithAdvancedRateLimiting(ctx context.Context, options types.GenerateOptions) (*response, error) {
    model := options.Model
    estimatedTokens := p.estimateTokens(options)

    // Get current rate limit info
    info, exists := p.rateLimitHelper.GetRateLimitInfo(model)
    if exists {
        // Custom logic based on rate limit status
        if info.RequestsRemaining < 5 {
            log.Printf("Low on requests for %s: %d remaining", model, info.RequestsRemaining)

            // Implement backoff strategy
            backoffDuration := time.Duration(5-info.RequestsRemaining) * time.Second
            log.Printf("Applying backoff: waiting %v", backoffDuration)
            time.Sleep(backoffDuration)
        }

        // Log detailed rate limit information
        log.Printf("Rate limit status for %s: %d/%d requests, %d/%d tokens",
            model,
            info.RequestsRemaining, info.RequestsLimit,
            info.TokensRemaining, info.TokensLimit)
    }

    // Check if we can make the request
    if !p.rateLimitHelper.CanMakeRequest(model, estimatedTokens) {
        log.Printf("Rate limited for %s, waiting...", model)

        // Wait automatically
        if !p.rateLimitHelper.CheckRateLimitAndWait(model, estimatedTokens) {
            return nil, fmt.Errorf("rate limit exceeded for model %s", model)
        }

        log.Printf("Rate limit wait completed for %s", model)
    }

    // Implement retry logic with exponential backoff
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        if attempt > 0 {
            waitTime := time.Duration(1<<uint(attempt)) * time.Second
            log.Printf("Retrying request for %s after %v (attempt %d)", model, waitTime, attempt+1)
            time.Sleep(waitTime)
        }

        // Make the request
        resp, err := p.makeHTTPRequest(ctx, options)
        if err != nil {
            lastErr = err

            // Check if error is rate limit related
            if isRateLimitError(err) {
                log.Printf("Rate limit error on attempt %d for %s: %v", attempt+1, model, err)

                // Update rate limits and retry
                p.rateLimitHelper.CheckRateLimitAndWait(model, estimatedTokens)
                continue
            }

            // Non-rate-limit error, don't retry
            break
        }

        // Success - update rate limits and return
        p.rateLimitHelper.ParseAndUpdateRateLimits(resp.Header, model)
        return resp, nil
    }

    return nil, fmt.Errorf("request failed after 3 attempts: %w", lastErr)
}

func isRateLimitError(err error) bool {
    // Check if error indicates rate limiting
    errStr := err.Error()
    return strings.Contains(errStr, "rate limit") ||
           strings.Contains(errStr, "too many requests") ||
           strings.Contains(errStr, "429")
}

// Rate limit monitoring
func (p *ExampleProvider) logRateLimitStatus() {
    models := []string{"example-model-v1", "example-model-v2"}

    for _, model := range models {
        if info, exists := p.rateLimitHelper.GetRateLimitInfo(model); exists {
            log.Printf("Model %s rate limits:", model)
            log.Printf("  Requests: %d/%d remaining", info.RequestsRemaining, info.RequestsLimit)
            log.Printf("  Tokens: %d/%d remaining", info.TokensRemaining, info.TokensLimit)

            if !info.RequestsReset.IsZero() {
                timeUntilReset := time.Until(info.RequestsReset)
                log.Printf("  Reset in: %v", timeUntilReset)
            }

            // Alert if running low
            if info.RequestsRemaining < 10 {
                log.Printf("  WARNING: Low on requests!")
            }
            if info.TokensRemaining < 1000 {
                log.Printf("  WARNING: Low on tokens!")
            }
        } else {
            log.Printf("No rate limit info available for model %s", model)
        }
    }
}

// Implement smart throttling based on usage patterns
func (p *ExampleProvider) shouldThrottle(model string) bool {
    info, exists := p.rateLimitHelper.GetRateLimitInfo(model)
    if !exists {
        return false
    }

    // Calculate usage percentage
    requestUsage := float64(info.RequestsLimit-info.RequestsRemaining) / float64(info.RequestsLimit)
    tokenUsage := float64(info.TokensLimit-info.TokensRemaining) / float64(info.TokensLimit)

    // Throttle if either usage is high
    return requestUsage > 0.8 || tokenUsage > 0.8
}
```

### Example 4: Model Cache with Advanced Features

```go
package exampleprovider

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"
)

// Enhanced model cache with refresh strategies
type EnhancedModelCache struct {
    *common.ModelCache
    provider         *ExampleProvider
    refreshMutex     sync.Mutex
    lastRefresh      time.Time
    refreshInterval  time.Duration
    refreshInProgress bool
}

func NewEnhancedModelCache(provider *ExampleProvider, ttl time.Duration) *EnhancedModelCache {
    return &EnhancedModelCache{
        ModelCache:      common.NewModelCache(ttl),
        provider:        provider,
        refreshInterval: ttl / 2, // Refresh at half TTL
    }
}

func (c *EnhancedModelCache) GetModelsWithRefresh(ctx context.Context) ([]types.Model, error) {
    // Check if background refresh is needed
    c.checkAndStartBackgroundRefresh(ctx)

    // Get models (will use cache or fetch if stale)
    return c.GetModels(
        func() ([]types.Model, error) {
            return c.provider.fetchModelsFromAPI(ctx)
        },
        func() []types.Model {
            return c.provider.getStaticModels()
        },
    )
}

func (c *EnhancedModelCache) checkAndStartBackgroundRefresh(ctx context.Context) {
    c.refreshMutex.Lock()
    defer c.refreshMutex.Unlock()

    // Check if refresh is needed
    if c.refreshInProgress || time.Since(c.lastRefresh) < c.refreshInterval {
        return
    }

    // Start background refresh
    c.refreshInProgress = true
    go func() {
        defer func() {
            c.refreshMutex.Lock()
            c.refreshInProgress = false
            c.lastRefresh = time.Now()
            c.refreshMutex.Unlock()
        }()

        models, err := c.provider.fetchModelsFromAPI(ctx)
        if err != nil {
            log.Printf("Background model refresh failed: %v", err)
            return
        }

        // Update cache
        c.Update(models)
        log.Printf("Background model refresh completed, cached %d models", len(models))
    }()
}

// Model cache with validation
func (p *ExampleProvider) GetValidatedModels(ctx context.Context) ([]types.Model, error) {
    return p.modelCache.GetModels(
        func() ([]types.Model, error) {
            models, err := p.fetchModelsFromAPI(ctx)
            if err != nil {
                return nil, err
            }

            // Validate models
            var validModels []types.Model
            for _, model := range models {
                if p.validateModel(model) {
                    validModels = append(validModels, model)
                } else {
                    log.Printf("Skipping invalid model: %s", model.ID)
                }
            }

            return validModels, nil
        },
        func() []types.Model {
            return p.getValidStaticModels()
        },
    )
}

func (p *ExampleProvider) validateModel(model types.Model) bool {
    // Basic validation
    if model.ID == "" || model.Name == "" {
        return false
    }

    // Validate max tokens
    if model.MaxTokens <= 0 {
        log.Printf("Model %s has invalid max tokens: %d", model.ID, model.MaxTokens)
        return false
    }

    // Validate provider
    if model.Provider != p.Type() {
        log.Printf("Model %s has wrong provider: %s", model.ID, model.Provider)
        return false
    }

    return true
}

func (p *ExampleProvider) getValidStaticModels() []types.Model {
    models := []types.Model{
        {
            ID:                 "example-model-v1",
            Name:               "Example Model v1",
            Provider:           p.Type(),
            SupportsStreaming:  true,
            SupportsToolCalling: true,
            MaxTokens:          4096,
        },
        {
            ID:                 "example-model-v2",
            Name:               "Example Model v2",
            Provider:           p.Type(),
            SupportsStreaming:  true,
            SupportsToolCalling: true,
            MaxTokens:          8192,
        },
    }

    // Filter valid models
    var validModels []types.Model
    for _, model := range models {
        if p.validateModel(model) {
            validModels = append(validModels, model)
        }
    }

    return validModels
}

// Model cache with metrics
type ModelCacheMetrics struct {
    CacheHits    int64
    CacheMisses  int64
    APIRequests  int64
    APIErrors    int64
    LastRefresh  time.Time
}

func (p *ExampleProvider) GetModelsWithMetrics(ctx context.Context) ([]types.Model, error) {
    start := time.Now()

    models, err := p.modelCache.GetModels(
        func() ([]types.Model, error) {
            // Increment API request counter
            p.metrics.CacheMisses++
            p.metrics.APIRequests++

            models, err := p.fetchModelsFromAPI(ctx)
            if err != nil {
                p.metrics.APIErrors++
                return nil, err
            }

            p.metrics.LastRefresh = time.Now()
            return models, nil
        },
        func() []types.Model {
            p.metrics.CacheHits++
            return p.getStaticModels()
        },
    )

    duration := time.Since(start)
    log.Printf("GetModels completed in %v, cache status: hits=%d, misses=%d",
        duration, p.metrics.CacheHits, p.metrics.CacheMisses)

    return models, err
}
```

---

## Phase 3: Standardized Core API

Phase 3 introduced a standardized core API that provides consistent request/response patterns across all providers while preserving provider-specific capabilities through extensions.

### Overview

The standardized core API includes:
- **StandardRequest/StandardResponse**: Universal request/response formats
- **CoreRequestBuilder**: Type-safe request construction with validation
- **Extension System**: Provider-specific adapters for unique capabilities
- **CoreChatProvider**: Unified interface using standardized API
- **ExtensionRegistry**: Centralized extension management

### CoreRequestBuilder

The CoreRequestBuilder provides a fluent interface for building validated requests.

```go
type CoreRequestBuilder struct {
    // Internal builder state
}
```

#### Creating Requests with Builder

```go
// Basic request building
request, err := types.NewCoreRequestBuilder().
    WithMessages([]types.ChatMessage{
        {Role: "user", Content: "Hello, world!"},
    }).
    WithModel("gpt-4o").
    WithMaxTokens(1000).
    WithTemperature(0.7).
    Build()

// Advanced request with tools
request, err := types.NewCoreRequestBuilder().
    WithMessages(messages).
    WithModel("gpt-4o").
    WithMaxTokens(2000).
    WithTemperature(0.5).
    WithTools(tools).
    WithToolChoice(&types.ToolChoice{Mode: types.ToolChoiceAuto}).
    WithStreaming(true).
    Build()
```

#### Builder Methods

```go
// Core content methods
func (b *CoreRequestBuilder) WithMessages(messages []types.ChatMessage) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithModel(model string) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithMaxTokens(maxTokens int) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithTemperature(temperature float64) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithStreaming(streaming bool) *CoreRequestBuilder

// Tool calling methods
func (b *CoreRequestBuilder) WithTools(tools []types.Tool) *CoreRequestBuilder
func (b *CoreRequestBuilder) WithToolChoice(toolChoice *ToolChoice) *CoreRequestBuilder

// Metadata methods for provider-specific features
func (b *CoreRequestBuilder) WithMetadata(key string, value interface{}) *CoreRequestBuilder

// Validation and building
func (b *CoreRequestBuilder) Build() (*StandardRequest, error)
```

#### Request Validation

The builder automatically validates requests:

```go
request, err := types.NewCoreRequestBuilder().
    WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
    WithModel("gpt-4o").
    WithMaxTokens(100000).  // May be too large for some providers
    WithTemperature(2.5).   // May exceed provider limits
    Build()

if err != nil {
    // Validation error with specific details
    return fmt.Errorf("invalid request: %w", err)
}
```

### Standard Types

#### StandardRequest

Universal request format used across all providers.

```go
type StandardRequest struct {
    Messages     []ChatMessage          `json:"messages"`
    Model        string                 `json:"model"`
    MaxTokens    int                    `json:"max_tokens,omitempty"`
    Temperature  float64                `json:"temperature,omitempty"`
    Stream       bool                   `json:"stream,omitempty"`
    Tools        []Tool                 `json:"tools,omitempty"`
    ToolChoice   *ToolChoice            `json:"tool_choice,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
```

#### StandardResponse

Consistent response format across providers.

```go
type StandardResponse struct {
    ID      string         `json:"id"`
    Object  string         `json:"object"`
    Created int64          `json:"created"`
    Model   string         `json:"model"`
    Choices []StandardChoice `json:"choices"`
    Usage   Usage          `json:"usage"`
}

type StandardChoice struct {
    Index        int         `json:"index"`
    Message      ChatMessage `json:"message"`
    FinishReason string      `json:"finish_reason"`
}
```

#### StandardStreamChunk

Standardized streaming chunk format.

```go
type StandardStreamChunk struct {
    ID      string         `json:"id"`
    Object  string         `json:"object"`
    Created int64          `json:"created"`
    Model   string         `json:"model"`
    Choices []StandardChoice `json:"choices"`
}
```

### CoreChatProvider Interface

The CoreChatProvider interface provides a unified way to interact with all providers using the standardized API.

```go
type CoreChatProvider interface {
    GenerateStandardCompletion(ctx context.Context, request StandardRequest) (*StandardResponse, error)
    GenerateStandardStream(ctx context.Context, request StandardRequest) (StandardStream, error)
    GetCoreExtension() CoreProviderExtension
    GetStandardCapabilities() []string
    ValidateStandardRequest(request StandardRequest) error
}
```

#### Using CoreChatProvider

```go
// Create core provider using extension
extension, _ := GetExtensionForProvider(types.ProviderTypeOpenAI)
coreProvider := types.NewCoreProviderAdapter(legacyProvider, extension)

// Generate completion
response, err := coreProvider.GenerateStandardCompletion(ctx, *request)
if err != nil {
    return nil, err
}

fmt.Printf("Response: %s\n", response.Choices[0].Message.Content)
fmt.Printf("Tokens used: %d\n", response.Usage.TotalTokens)
```

### Provider Extensions

Extensions preserve provider-specific capabilities while using the standardized API.

#### CoreProviderExtension Interface

```go
type CoreProviderExtension interface {
    Name() string
    Version() string
    Description() string
    StandardToProvider(request StandardRequest) (interface{}, error)
    ProviderToStandard(response interface{}) (*StandardResponse, error)
    ProviderToStandardChunk(chunk interface{}) (*StandardStreamChunk, error)
    ValidateOptions(options map[string]interface{}) error
    GetCapabilities() []string
}
```

#### OpenAI Extension Example

```go
type OpenAIExtension struct {
    // Extension implementation
}

func (e *OpenAIExtension) StandardToProvider(request StandardRequest) (interface{}, error) {
    // Convert StandardRequest to OpenAI format
    openAIReq := openai.ChatCompletionRequest{
        Model:       request.Model,
        Messages:    e.convertMessages(request.Messages),
        MaxTokens:   request.MaxTokens,
        Temperature: request.Temperature,
        Stream:      request.Stream,
        Tools:       e.convertTools(request.Tools),
    }

    // Handle provider-specific metadata
    if responseFormat, ok := request.Metadata["response_format"].(map[string]interface{}); ok {
        openAIReq.ResponseFormat = responseFormat
    }

    return openAIReq, nil
}

func (e *OpenAIExtension) GetCapabilities() []string {
    return []string{
        "chat",
        "streaming",
        "tool_calling",
        "parallel_tool_calls",
        "json_mode",
        "reproducible_results",
        "top_p_sampling",
    }
}
```

### Extension Registry

The ExtensionRegistry manages provider-specific extensions.

```go
type ExtensionRegistry struct {
    // Internal registry state
}

func NewExtensionRegistry() *ExtensionRegistry
func (r *ExtensionRegistry) RegisterExtension(providerType ProviderType, extension CoreProviderExtension) error
func (r *ExtensionRegistry) GetExtension(providerType ProviderType) (CoreProviderExtension, error)
func (r *ExtensionRegistry) ListExtensions() []ProviderType
func (r *ExtensionRegistry) GetCapabilities(providerType ProviderType) []string
```

#### Using Extension Registry

```go
// Create registry
registry := types.NewExtensionRegistry()

// Register extensions
registry.RegisterExtension(types.ProviderTypeOpenAI, extension.NewOpenAIExtension())
registry.RegisterExtension(types.ProviderTypeAnthropic, extension.NewAnthropicExtension())

// Get extension for provider
extension, err := registry.GetExtension(types.ProviderTypeOpenAI)
if err != nil {
    return nil, fmt.Errorf("no extension for provider: %w", err)
}

// Check capabilities
capabilities := registry.GetCapabilities(types.ProviderTypeOpenAI)
fmt.Printf("OpenAI capabilities: %v\n", capabilities)
```

### Core Provider Adapter

The CoreProviderAdapter wraps legacy providers with extensions.

```go
func NewCoreProviderAdapter(provider Provider, extension CoreProviderExtension) CoreChatProvider
```

#### Creating Core Providers

```go
// Method 1: Direct adapter creation
legacyProvider, _ := factory.CreateProvider(types.ProviderTypeOpenAI, config)
extension, _ := GetExtensionForProvider(types.ProviderTypeOpenAI)
coreProvider := types.NewCoreProviderAdapter(legacyProvider, extension)

// Method 2: Factory function
coreProvider, _ := CreateCoreProvider(types.ProviderTypeOpenAI, config)

// Use standardized API
request, _ := types.NewCoreRequestBuilder().
    WithMessages(messages).
    WithModel("gpt-4o").
    Build()

response, _ := coreProvider.GenerateStandardCompletion(ctx, *request)
```

### Migration to Standardized API

#### Gradual Migration Strategy

```go
// Step 1: Add core provider support
type MyService struct {
    legacyProvider Provider
    coreProvider   CoreChatProvider
    useCoreAPI    bool
}

func NewService(provider Provider) *MyService {
    // Create core provider adapter
    extension, _ := GetExtensionForProvider(provider.Type())
    coreProvider := NewCoreProviderAdapter(provider, extension)

    return &MyService{
        legacyProvider: provider,
        coreProvider:   coreProvider,
        useCoreAPI:    false, // Start with legacy
    }
}

// Step 2: Switch between APIs based on feature flag
func (s *MyService) Generate(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    if s.useCoreAPI {
        return s.generateWithStandardAPI(ctx, req)
    }
    return s.generateWithLegacyAPI(ctx, req)
}

func (s *MyService) generateWithStandardAPI(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Build standardized request
    request, err := types.NewCoreRequestBuilder().
        WithMessages(req.Messages).
        WithModel(req.Model).
        WithMaxTokens(req.MaxTokens).
        WithTemperature(req.Temperature).
        Build()
    if err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }

    // Generate using standardized API
    response, err := s.coreProvider.GenerateStandardCompletion(ctx, *request)
    if err != nil {
        return nil, fmt.Errorf("generation failed: %w", err)
    }

    return &ChatResponse{
        Content: response.Choices[0].Message.Content,
        Usage:   response.Usage,
        Model:   response.Model,
        ID:      response.ID,
    }, nil
}
```

#### Feature-Based Migration

```go
type HybridService struct {
    provider Provider
    // Add core provider for specific features
    coreProvider CoreChatProvider
}

func (s *HybridService) BasicChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Use standardized API for basic chat
    request, _ := types.NewCoreRequestBuilder().
        WithMessages(req.Messages).
        WithModel(req.Model).
        Build()

    response, err := s.coreProvider.GenerateStandardCompletion(ctx, *request)
    if err != nil {
        return nil, err
    }

    return s.convertToChatResponse(response), nil
}

func (s *HybridService) ComplexToolCalling(ctx context.Context, req ToolRequest) (*ToolResponse, error) {
    // Use legacy API for complex tool calling until standardized version is ready
    stream, err := s.provider.GenerateChatCompletion(ctx, req.LegacyOptions)
    // ... legacy processing
}
```

### Best Practices for Core API Usage

#### Always Use Request Builder

```go
// Good: Use builder for validated requests
request, err := types.NewCoreRequestBuilder().
    WithMessages(messages).
    WithModel("gpt-4o").
    WithMaxTokens(1000).
    WithTemperature(0.7).
    Build()
if err != nil {
    return nil, fmt.Errorf("failed to build request: %w", err)
}

// Bad: Create request manually (no validation)
request := &types.StandardRequest{
    Messages:     messages,
    Model:        "gpt-4o",
    MaxTokens:    1000,
    Temperature:  0.7,
    // No validation!
}
```

#### Validate Provider Capabilities

```go
// Good: Check capabilities before using features
extension := coreProvider.GetCoreExtension()
capabilities := extension.GetCapabilities()

supportsTools := false
for _, capability := range capabilities {
    if capability == "tool_calling" {
        supportsTools = true
        break
    }
}

if supportsTools && len(req.Tools) > 0 {
    // Use tool calling
    request = request.WithTools(req.Tools).WithToolChoice(req.ToolChoice)
}

// Bad: Assume all features are supported
request := request.WithTools(req.Tools) // May not be supported!
```

#### Handle Provider-Specific Features Properly

```go
// Good: Use metadata for provider-specific features
request, err := types.NewCoreRequestBuilder().
    WithMessages(messages).
    WithModel("gpt-4o").
    Build()

// Add OpenAI-specific JSON mode
request = request.WithMetadata("response_format", map[string]interface{}{
    "type": "json_object",
    "schema": jsonSchema,
})

// Add Anthropic-specific thinking mode
request = request.WithMetadata("thinking", true)

// Bad: Mix provider-specific logic
if providerType == types.ProviderTypeOpenAI {
    // Handle OpenAI JSON mode
} else if providerType == types.ProviderTypeAnthropic {
    // Handle Anthropic thinking mode
}
// This defeats the purpose of standardized API!
```

### Extension Development

#### Creating Custom Extensions

```go
type CustomProviderExtension struct {
    name        string
    version     string
    description string
}

func (e *CustomProviderExtension) StandardToProvider(request StandardRequest) (interface{}, error) {
    // Convert to provider-specific format
    providerReq := CustomRequest{
        Messages:     e.convertMessages(request.Messages),
        Model:        request.Model,
        MaxTokens:    request.MaxTokens,
        Temperature:  request.Temperature,
        Stream:       request.Stream,
    }

    // Handle provider-specific metadata
    if customFeature, ok := request.Metadata["custom_feature"].(bool); ok && customFeature {
        providerReq.EnableCustomFeature = true
    }

    return providerReq, nil
}

func (e *CustomProviderExtension) GetCapabilities() []string {
    return []string{
        "chat",
        "streaming",
        "custom_feature",
        "large_context",
    }
}
```

#### Extension Validation

```go
func (e *CustomProviderExtension) ValidateOptions(options map[string]interface{}) error {
    // Validate provider-specific options
    if customParam, ok := options["custom_param"].(string); ok {
        if len(customParam) > 1000 {
            return fmt.Errorf("custom_param too long (max 1000 chars)")
        }
    }

    return nil
}
```

### Testing with Standardized API

#### Mock Extensions

```go
type MockExtension struct {
    name string
}

func (m *MockExtension) StandardToProvider(request StandardRequest) (interface{}, error) {
    // Return mock provider request
    return MockRequest{Model: request.Model}, nil
}

func (m *MockExtension) ProviderToStandard(response interface{}) (*StandardResponse, error) {
    // Return mock standardized response
    return &StandardResponse{
        ID:     "mock-response",
        Object: "chat.completion",
        Model:  "mock-model",
        Choices: []StandardChoice{{
            Index:   0,
            Message: ChatMessage{Role: "assistant", Content: "Mock response"},
        }},
        Usage: Usage{TotalTokens: 10},
    }, nil
}

// Use in tests
func TestCoreProvider(t *testing.T) {
    mockProvider := &MockProvider{}
    mockExtension := &MockExtension{name: "mock"}

    coreProvider := NewCoreProviderAdapter(mockProvider, mockExtension)

    request, _ := types.NewCoreRequestBuilder().
        WithMessages([]types.ChatMessage{{Role: "user", Content: "Hello"}}).
        WithModel("mock-model").
        Build()

    response, err := coreProvider.GenerateStandardCompletion(context.Background(), *request)
    require.NoError(t, err)
    assert.Equal(t, "Mock response", response.Choices[0].Message.Content)
}
```

This standardized core API provides consistency across all providers while preserving the ability to access provider-specific features through extensions, making it easier to build portable AI applications.

---

This comprehensive documentation covers all aspects of the common utilities package, providing both theoretical understanding and practical examples for effective usage in provider development.