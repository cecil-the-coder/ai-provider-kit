# AI Provider Kit - Version-Specific Migration Guide

This guide provides detailed upgrade instructions for migrating between different versions of AI Provider Kit, including breaking changes, API modifications, deprecations, and configuration changes.

## Table of Contents

1. [Version Overview](#version-overview)
2. [Migrating to v1.0.16](#migrating-to-v1016-from-v1015)
3. [Migrating to v1.0.15](#migrating-to-v1015-from-v1014)
4. [Migrating to v1.0.14](#migrating-to-v1014-from-v1013)
5. [Migrating to v1.0.13](#migrating-to-v1013-from-v1012)
6. [Migrating to v1.0.12](#migrating-to-v1012-from-v1011)
7. [Migrating to v1.0.11](#migrating-to-v1011-from-v1010)
8. [Migrating to v1.0.10](#migrating-to-v1010-from-v109)
9. [Migrating to v1.0.9](#migrating-to-v109-from-v108)
10. [Migrating to v1.0.8](#migrating-to-v108-from-v107)
11. [Migrating to v1.0.7](#migrating-to-v107-from-v106)
12. [Migrating to v1.0.6](#migrating-to-v106-from-v105)
13. [Migrating to v1.0.5](#migrating-to-v105-from-v104)
14. [Migrating to v1.0.4](#migrating-to-v104-from-v103)
15. [Migrating to v1.0.3](#migrating-to-v103-from-v102)
16. [Migrating to v1.0.2](#migrating-to-v102-from-v101)
17. [Multi-Version Migrations](#multi-version-migrations)
18. [Migration Checklist](#migration-checklist)

---

## Version Overview

### Release Timeline

| Version | Release Date | Type | Key Changes |
|---------|-------------|------|-------------|
| v1.0.16 | 2025-12-02 | Enhancement | Context field in GenerateRequest, API key context helpers |
| v1.0.15 | 2025-12-01 | Bugfix | Test timeout fix |
| v1.0.14 | 2025-11-30 | Maintenance | Build sync |
| v1.0.13 | 2025-11-30 | Major Refactor | Package restructuring, API standardization |
| v1.0.12 | 2025-11-30 | Enhancement | Context token checks in streaming |
| v1.0.11 | 2025-11-30 | Breaking | ExecuteWithAuthMessage for tool call support |
| v1.0.10 | 2025-11-30 | Enhancement | Context-based OAuth token injection |
| v1.0.9  | 2025-11-30 | Simplification | Static model list for Gemini |
| v1.0.8  | 2025-11-29 | Major Refactor | High-priority refactoring Round 2 |
| v1.0.7  | 2025-11-29 | Bugfix | Staticcheck fixes |
| v1.0.6  | 2025-11-29 | Cleanup | Dead code removal |
| v1.0.5  | 2025-11-29 | Breaking | System prompt behavior change |
| v1.0.4  | 2025-11-29 | Bugfix | Import ordering fixes |
| v1.0.3  | 2025-11-29 | Major Enhancement | Extension system capabilities |
| v1.0.2  | 2025-11-29 | Enhancement | SSE streaming support |
| v1.0.1  | 2025-11-29 | Initial | Provider test files |
| v1.0.0  | 2025-11-16 | Initial | Stable release |

### Breaking Changes Summary

| Version | Breaking Changes | Impact |
|---------|-----------------|--------|
| v1.0.13 | Package restructuring (`providers/common` → sub-packages) | High - Import paths changed |
| v1.0.11 | New `ExecuteWithAuthMessage` method | Medium - OAuth providers need update |
| v1.0.5  | Removed default system prompt | Medium - Anthropic behavior change |
| v1.0.3  | Extension interface additions | Low - Optional methods |

---

## Migrating to v1.0.16 (from v1.0.15)

**Release Date:** 2025-12-02
**Type:** Enhancement
**Risk Level:** Low

### What's New

#### Context Field in GenerateRequest

A new optional `Context` field has been added to `GenerateRequest` to propagate authentication context from extensions to providers.

```go
type GenerateRequest struct {
    Provider    string                 `json:"provider,omitempty"`
    Model       string                 `json:"model,omitempty"`
    // ... other fields ...
    Context     context.Context        `json:"-"` // NEW: Optional context propagation
}
```

#### API Key Context Helpers

New helper functions matching the OAuth context helpers:

```go
// In pkg/providers/common/auth/auth_context.go
func WithAPIKey(ctx context.Context, apiKey string) context.Context
func GetAPIKey(ctx context.Context) (string, bool)
```

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.16
go mod tidy
```

#### 2. Optional: Use Context Field in Extensions

If you have custom extensions that need to propagate authentication:

**Before (v1.0.15):**
```go
func (e *MyExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
    // No way to pass auth context to provider
    return nil
}
```

**After (v1.0.16):**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"

func (e *MyExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
    // Inject API key into context for provider use
    req.Context = auth.WithAPIKey(ctx, "runtime-api-key")
    return nil
}
```

#### 3. Optional: Use API Key Helpers

**Before (v1.0.15):**
```go
// Manual context key handling
type contextKey string
const apiKeyContextKey contextKey = "api-key"

ctx = context.WithValue(ctx, apiKeyContextKey, apiKey)
```

**After (v1.0.16):**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"

ctx = auth.WithAPIKey(ctx, apiKey)
apiKey, ok := auth.GetAPIKey(ctx)
```

### Breaking Changes

**None** - This is a fully backwards-compatible enhancement.

### Deprecations

**None**

### Testing Recommendations

- Test extension-to-provider context propagation if using the new Context field
- Verify existing authentication mechanisms still work
- Test custom extensions with the new context helpers

---

## Migrating to v1.0.15 (from v1.0.14)

**Release Date:** 2025-12-01
**Type:** Bugfix
**Risk Level:** Low

### What's Changed

Fixed test timeout issue in `TestBackwardCompatibility_MixedUsage`.

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.15
go mod tidy
```

### Breaking Changes

**None**

### Testing Recommendations

- Re-run test suite to verify timeout issues are resolved
- No application code changes required

---

## Migrating to v1.0.14 (from v1.0.13)

**Release Date:** 2025-11-30
**Type:** Maintenance
**Risk Level:** Low

### What's Changed

Build synchronization and maintenance updates.

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.14
go mod tidy
```

### Breaking Changes

**None**

---

## Migrating to v1.0.13 (from v1.0.12)

**Release Date:** 2025-11-30
**Type:** Major Refactor
**Risk Level:** High

### What's Changed

Major package restructuring and API standardization. The `providers/common` package has been split into focused sub-packages.

#### Package Restructuring

The monolithic `providers/common` package has been reorganized:

```
providers/common/
├── auth/          # Authentication helpers, OAuth refresh
├── config/        # Configuration helpers, cache TTLs
├── models/        # Model metadata, cache, registry
└── streaming/     # SSE parser, stream processing
```

#### Import Path Changes

| Old Import | New Import |
|-----------|------------|
| `providers/common.AuthHelper` | `providers/common/auth.AuthHelper` |
| `providers/common.OAuthRefresh` | `providers/common/auth.OAuthRefresh` |
| `providers/common.ConfigHelper` | `providers/common/config.ConfigHelper` |
| `providers/common.ModelMetadata` | `providers/common/models.ModelMetadata` |
| `providers/common.ModelCache` | `providers/common/models.ModelCache` |
| `providers/common.SSEParser` | `providers/common/streaming.SSEParser` |
| `providers/common.StreamProcessor` | `providers/common/streaming.StreamProcessor` |

#### API Standardization

1. **Unified streaming method names**: All providers now use `executeStreamWithAuth` internally
2. **Gemini tool format**: Added `ToolFormatGemini` constant (was incorrectly using `ToolFormatOpenAI`)
3. **Error handling**: Standardized error categorization across providers

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.13
go mod tidy
```

#### 2. Update Import Statements

**Before (v1.0.12):**
```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"
)

func MyFunction() {
    helper := common.NewAuthHelper(config)
    parser := common.NewSSEParser()
    cache := common.NewModelCache()
}
```

**After (v1.0.13):**
```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/streaming"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
)

func MyFunction() {
    helper := auth.NewAuthHelper(config)
    parser := streaming.NewSSEParser()
    cache := models.NewModelCache()
}
```

#### 3. Update Tool Format Constants

**Before (v1.0.12):**
```go
// Gemini incorrectly reported as OpenAI format
if provider.GetToolFormat() == types.ToolFormatOpenAI {
    // Handle both OpenAI and Gemini
}
```

**After (v1.0.13):**
```go
if provider.GetToolFormat() == types.ToolFormatGemini {
    // Handle Gemini specifically
} else if provider.GetToolFormat() == types.ToolFormatOpenAI {
    // Handle OpenAI
}
```

#### 4. Update Custom Providers (if any)

If you have custom provider implementations using common utilities:

```go
// Before
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"

type CustomProvider struct {
    authHelper *common.AuthHelper
}

// After
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"

type CustomProvider struct {
    authHelper *auth.AuthHelper
}
```

### Breaking Changes

1. **Import paths changed** - All code importing from `providers/common` must update to use sub-packages
2. **Tool format constant** - Gemini now correctly reports `ToolFormatGemini` instead of `ToolFormatOpenAI`

### Deprecations

- Direct imports from `providers/common` are deprecated (code still in parent package for backwards compatibility but will be removed in v2.0)

### Migration Automation

You can use this sed script to automatically update most imports:

```bash
#!/bin/bash
# update-imports-v1.0.13.sh

find . -name "*.go" -type f -exec sed -i \
  -e 's|providers/common\.AuthHelper|providers/common/auth.AuthHelper|g' \
  -e 's|providers/common\.OAuthRefresh|providers/common/auth.OAuthRefresh|g' \
  -e 's|providers/common\.ConfigHelper|providers/common/config.ConfigHelper|g' \
  -e 's|providers/common\.ModelMetadata|providers/common/models.ModelMetadata|g' \
  -e 's|providers/common\.ModelCache|providers/common/models.ModelCache|g' \
  -e 's|providers/common\.SSEParser|providers/common/streaming.SSEParser|g' \
  {} +

# Update import statements
find . -name "*.go" -type f -exec sed -i \
  -e 's|"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"|"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"\n\t"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"\n\t"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"\n\t"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/streaming"|g' \
  {} +

# Run goimports to clean up
goimports -w .
```

### Testing Recommendations

- **Unit tests**: Re-run all tests after updating imports
- **Integration tests**: Verify provider initialization still works
- **Tool calling**: Test Gemini tool calling specifically to ensure format is correct
- **Custom providers**: If you have custom providers, test thoroughly

### Rollback Plan

If issues arise:

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.12
go mod tidy
```

Then revert import changes.

---

## Migrating to v1.0.12 (from v1.0.11)

**Release Date:** 2025-11-30
**Type:** Enhancement
**Risk Level:** Low

### What's New

Added context token checks to Anthropic and Gemini streaming paths for better error handling.

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.12
go mod tidy
```

### What You Get

Better error messages when streaming with invalid context tokens:

```go
// Will now fail fast with clear error message
stream, err := provider.GenerateChatCompletion(ctx, options)
if err != nil {
    // Error clearly indicates context token issue
    log.Printf("Streaming failed: %v", err)
}
```

### Breaking Changes

**None**

### Testing Recommendations

- Test streaming with valid and invalid context tokens
- Verify error messages are clear and actionable

---

## Migrating to v1.0.11 (from v1.0.10)

**Release Date:** 2025-11-30
**Type:** Breaking
**Risk Level:** Medium

### What's Changed

Added `ExecuteWithAuthMessage` method to support full `ChatMessage` responses with tool calls. This replaces the limitation of `ExecuteWithAuth` which only returned string content.

#### New Methods

**In `pkg/providers/common/authhelpers.go`:**
```go
// Returns full ChatMessage including tool calls
func (h *AuthHelper) ExecuteWithAuthMessage(ctx context.Context, ...) (types.ChatMessage, error)
```

**In `pkg/keymanager/keymanager.go`:**
```go
func (km *KeyManager) ExecuteWithFailoverMessage(...) (types.ChatMessage, error)
```

**In `pkg/oauthmanager/oauthmanager.go`:**
```go
func (om *OAuthKeyManager) ExecuteWithFailoverMessage(...) (types.ChatMessage, error)
```

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.11
go mod tidy
```

#### 2. Update Tool Calling Code

If you're using OAuth providers (Anthropic, Gemini, Qwen) with tool calling:

**Before (v1.0.10):**
```go
// Could only get text responses, tool calls were lost
responseText, err := authHelper.ExecuteWithAuth(ctx, func(apiKey string) (string, error) {
    // Limited to string responses
    return "response text", nil
})
```

**After (v1.0.11):**
```go
// Now preserves tool calls in response
responseMsg, err := authHelper.ExecuteWithAuthMessage(ctx, func(apiKey string) (types.ChatMessage, error) {
    // Returns full ChatMessage with tool calls
    return types.ChatMessage{
        Role:    "assistant",
        Content: "response text",
        ToolCalls: []types.ToolCall{
            // Tool calls are preserved
        },
    }, nil
})

// Access tool calls
for _, toolCall := range responseMsg.ToolCalls {
    // Process tool call
}
```

#### 3. Update Custom Providers Using AuthHelper

If you have custom providers:

**Before:**
```go
type MyProvider struct {
    authHelper *common.AuthHelper
}

func (p *MyProvider) Generate(ctx context.Context, options types.GenerateOptions) error {
    response, err := p.authHelper.ExecuteWithAuth(ctx, func(apiKey string) (string, error) {
        // Make API call
        return responseString, nil
    })
}
```

**After:**
```go
type MyProvider struct {
    authHelper *common.AuthHelper
}

func (p *MyProvider) Generate(ctx context.Context, options types.GenerateOptions) error {
    message, err := p.authHelper.ExecuteWithAuthMessage(ctx, func(apiKey string) (types.ChatMessage, error) {
        // Make API call, return full message
        return types.ChatMessage{
            Role:      "assistant",
            Content:   responseString,
            ToolCalls: toolCalls, // Now preserved
        }, nil
    })
}
```

### Breaking Changes

**For Custom Provider Authors:**
- If you were relying on `ExecuteWithAuth` for non-text responses, you must migrate to `ExecuteWithAuthMessage`
- The callback signature changed from `func(string) (string, error)` to `func(string) (types.ChatMessage, error)`

**For Library Users:**
- No breaking changes if using standard providers
- Anthropic, Gemini, and Qwen providers now fully support tool calls with OAuth

### Deprecations

`ExecuteWithAuth` is maintained for backwards compatibility but is considered limited for tool calling scenarios.

### Testing Recommendations

- Test OAuth-based providers with tool calling
- Verify tool calls are preserved in responses
- Test multi-turn conversations with tools
- Verify backwards compatibility with simple text responses

---

## Migrating to v1.0.10 (from v1.0.9)

**Release Date:** 2025-11-30
**Type:** Enhancement
**Risk Level:** Low

### What's New

Context-based OAuth token injection support. Allows runtime OAuth token injection via context, taking priority over configured credentials.

#### New API

**In `pkg/providers/common/auth_context.go`:**
```go
func WithOAuthToken(ctx context.Context, token string) context.Context
func GetOAuthToken(ctx context.Context) (string, bool)
```

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.10
go mod tidy
```

#### 2. Optional: Use Runtime OAuth Injection

**Before (v1.0.9):**
```go
// OAuth tokens had to be configured upfront
config := types.ProviderConfig{
    OAuth: &types.OAuthConfig{
        // Static configuration
    },
}
provider, _ := factory.CreateProvider(types.ProviderTypeAnthropic, config)
```

**After (v1.0.10):**
```go
// Option 1: Static configuration (still works)
config := types.ProviderConfig{
    OAuth: &types.OAuthConfig{
        // Static configuration
    },
}
provider, _ := factory.CreateProvider(types.ProviderTypeAnthropic, config)

// Option 2: Runtime injection via context (NEW)
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"

ctx := context.Background()
ctx = common.WithOAuthToken(ctx, "runtime-oauth-token")

// This token takes priority over configured credentials
stream, _ := provider.GenerateChatCompletion(ctx, options)
```

### Use Cases

Runtime OAuth token injection is useful for:

1. **Multi-tenant applications**: Different users with different OAuth tokens
2. **Dynamic token refresh**: Inject newly refreshed tokens per request
3. **Testing**: Inject test tokens without modifying provider configuration
4. **Token rotation**: Rotate tokens on a per-request basis

### Example: Multi-Tenant Application

```go
func HandleUserRequest(userID string, w http.ResponseWriter, r *http.Request) {
    // Get user-specific OAuth token from database
    token := getUserOAuthToken(userID)

    // Inject into context
    ctx := common.WithOAuthToken(r.Context(), token)

    // Provider uses user's token
    stream, err := provider.GenerateChatCompletion(ctx, options)
    // ...
}
```

### Breaking Changes

**None** - This is fully backwards compatible.

### Testing Recommendations

- Test context-based token injection
- Verify context tokens take priority over config
- Test multi-tenant scenarios
- Verify token rotation

---

## Migrating to v1.0.9 (from v1.0.8)

**Release Date:** 2025-11-30
**Type:** Simplification
**Risk Level:** Low

### What's Changed

Gemini provider now uses a static model list instead of dynamic discovery. This improves performance and reliability.

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.9
go mod tidy
```

### Impact

**Positive:**
- Faster provider initialization (no model discovery API call)
- More reliable (no dependency on model discovery endpoint)
- Consistent model names

**Potential Issues:**
- New Gemini models may not be immediately available
- Must update library for new model support

### Workaround for Custom Models

If you need a model not in the static list:

```go
config := types.ProviderConfig{
    Type:         types.ProviderTypeGemini,
    DefaultModel: "gemini-custom-model", // Will be passed through
}
```

The provider will attempt to use any model name you specify, even if not in the static list.

### Breaking Changes

**None** - Model discovery was an internal implementation detail.

### Testing Recommendations

- Verify your Gemini models are in the static list
- Test provider initialization speed
- Test with both listed and unlisted models

---

## Migrating to v1.0.8 (from v1.0.7)

**Release Date:** 2025-11-29
**Type:** Major Refactor
**Risk Level:** Medium

### What's Changed

High-priority refactoring Round 2, including:
- Code organization improvements
- Performance optimizations
- Internal API cleanup

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.8
go mod tidy
```

### Breaking Changes

**None for public API** - Internal refactoring only.

### Testing Recommendations

- Full regression test suite
- Performance benchmarks (should see improvements)
- Memory usage monitoring

---

## Migrating to v1.0.7 (from v1.0.6)

**Release Date:** 2025-11-29
**Type:** Bugfix
**Risk Level:** Low

### What's Changed

Fixed staticcheck SA9003 warning (empty branch in recover()).

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.7
go mod tidy
```

### Breaking Changes

**None**

---

## Migrating to v1.0.6 (from v1.0.5)

**Release Date:** 2025-11-29
**Type:** Cleanup
**Risk Level:** Low

### What's Changed

Removed dead code including unused utilities and functions (~4,100 lines reduced).

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.6
go mod tidy
```

#### 2. Check for Removed Functions

If you were using any internal utilities, verify they still exist. Removed functions were:
- Unused file utilities in `providers/common/fileutils.go`
- Deprecated helper functions
- Test-only utilities

### Breaking Changes

**Potentially** - If you were using undocumented internal utilities, they may have been removed.

### Deprecations

All removed code was either:
- Unused internally
- Not part of public API
- Marked as deprecated

### Testing Recommendations

- Recompile to catch missing function errors
- Review any internal utility usage

---

## Migrating to v1.0.5 (from v1.0.4)

**Release Date:** 2025-11-29
**Type:** Breaking
**Risk Level:** Medium

### What's Changed

Removed automatic "expert programmer" fallback system prompt from Anthropic provider. This was causing inappropriate behavior for non-code requests.

### Behavior Changes

**Before (v1.0.4):**
```go
// When no system prompt provided, Anthropic automatically added:
// "You are an expert programmer. Generate production-ready code..."
provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "What is 2+2?",
    // No system prompt
})
// Response: Code-focused answer (inappropriate)
```

**After (v1.0.5):**
```go
// When no system prompt provided, only OAuth identifier added (if needed)
provider.GenerateChatCompletion(ctx, types.GenerateOptions{
    Prompt: "What is 2+2?",
    // No system prompt
})
// Response: Natural answer (appropriate)
```

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.5
go mod tidy
```

#### 2. Review Anthropic Usage

If you were relying on the automatic code-generation prompt:

**Before (v1.0.4):**
```go
// Relied on automatic prompt
options := types.GenerateOptions{
    Prompt: "Write a function",
}
```

**After (v1.0.5):**
```go
// Explicitly specify system prompt if needed
options := types.GenerateOptions{
    Prompt: "Write a function",
    Messages: []types.ChatMessage{
        {
            Role: "system",
            Content: "You are an expert programmer. Generate production-ready code.",
        },
    },
}
```

#### 3. Update Code Generation Workflows

If you have code generation workflows using Anthropic:

```go
func GenerateCode(prompt string) (string, error) {
    options := types.GenerateOptions{
        Prompt: prompt,
        Messages: []types.ChatMessage{
            {
                Role: "system",
                Content: "You are an expert programmer. Focus on code quality, readability, and best practices.",
            },
        },
    }
    // ...
}
```

### Breaking Changes

1. **Default behavior changed** - Anthropic no longer adds code-generation prompt automatically
2. **System prompt handling** - Must explicitly provide system prompts for specialized behavior

### Impact Analysis

**Low Impact:**
- General conversational use
- Q&A applications
- Multi-purpose chatbots

**High Impact:**
- Code generation applications relying on default behavior
- Automated code review tools
- Developer assistance tools

### Testing Recommendations

- Test all Anthropic-based code generation
- Verify non-code requests get appropriate responses
- Test with and without system prompts
- Compare response quality before/after

### Rollback Plan

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.4
```

And remove explicit system prompts if added.

---

## Migrating to v1.0.4 (from v1.0.3)

**Release Date:** 2025-11-29
**Type:** Bugfix
**Risk Level:** Low

### What's Changed

Fixed gci import ordering in `errors_test.go`.

### Migration Steps

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.4
go mod tidy
```

### Breaking Changes

**None**

---

## Migrating to v1.0.3 (from v1.0.2)

**Release Date:** 2025-11-29
**Type:** Major Enhancement
**Risk Level:** Low to Medium

### What's New

Comprehensive enhancement to the extension system with four major features:

1. **Priority/Ordering System**
2. **Provider Interceptor Pattern**
3. **Per-Request Extension Configuration**
4. **Capability-Based Interfaces**

#### 1. Priority System

Extensions now execute in priority order:

```go
const (
    PrioritySecurity  = 100 // Highest priority
    PriorityCache     = 200
    PriorityTransform = 500 // Default
    PriorityLogging   = 900 // Lowest priority
)

type Extension interface {
    // ... existing methods ...
    Priority() int // NEW
}
```

#### 2. Provider Interceptor Pattern

Chain multiple interceptors for request processing:

```go
type ProviderInterceptor interface {
    Intercept(ctx context.Context, req interface{}, next InterceptorFunc) (interface{}, error)
}

// Chain interceptors
chain := NewInterceptorChain(
    loggingInterceptor,
    timeoutInterceptor,
    cachingInterceptor,
)
```

#### 3. Per-Request Extension Config

Enable/disable extensions per request:

```go
req := &GenerateRequest{
    Prompt: "Hello",
    Metadata: map[string]interface{}{
        "extensions": map[string]interface{}{
            "caching": map[string]interface{}{
                "enabled": false, // Disable caching for this request
            },
        },
    },
}
```

#### 4. Capability-Based Interfaces

Optional interfaces for fine-grained control:

```go
type Initializable interface {
    Initialize(ctx context.Context, config Config) error
}

type BeforeGenerateHook interface {
    BeforeGenerate(ctx context.Context, req *GenerateRequest) error
}

type AfterGenerateHook interface {
    AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error
}

type ProviderErrorHandler interface {
    OnProviderError(ctx context.Context, provider string, err error) error
}
```

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.3
go mod tidy
```

#### 2. Add Priority Method to Custom Extensions

**Before (v1.0.2):**
```go
type MyExtension struct{}

func (e *MyExtension) Name() string { return "my-extension" }
func (e *MyExtension) Init(config Config) error { return nil }
```

**After (v1.0.3):**
```go
type MyExtension struct{}

func (e *MyExtension) Name() string { return "my-extension" }
func (e *MyExtension) Init(config Config) error { return nil }
func (e *MyExtension) Priority() int {
    return PriorityTransform // Or appropriate priority
}
```

#### 3. Optional: Implement Capability Interfaces

Instead of implementing all methods, implement only what you need:

**Before (v1.0.2):**
```go
type MyExtension struct{}

// Had to implement ALL methods even if not used
func (e *MyExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
    return nil // No-op
}
func (e *MyExtension) AfterGenerate(ctx context.Context, req *GenerateRequest, resp *GenerateResponse) error {
    return nil // No-op
}
// ... etc
```

**After (v1.0.3):**
```go
type MyExtension struct{}

// Implement only the base interface
func (e *MyExtension) Name() string { return "my-extension" }
func (e *MyExtension) Priority() int { return PriorityTransform }

// Optionally implement capability interfaces
func (e *MyExtension) BeforeGenerate(ctx context.Context, req *GenerateRequest) error {
    // Only implement if needed
    return e.doSomething()
}

// No need to implement AfterGenerate if not needed
```

#### 4. Optional: Use Provider Interceptors

For advanced request processing:

```go
// Create interceptor
type LoggingInterceptor struct{}

func (i *LoggingInterceptor) Intercept(ctx context.Context, req interface{}, next InterceptorFunc) (interface{}, error) {
    log.Printf("Before: %+v", req)
    resp, err := next(ctx, req)
    log.Printf("After: %+v", resp)
    return resp, err
}

// Register interceptor
registry := NewInterceptorRegistry()
registry.Register("logging", &LoggingInterceptor{})

// Use in chain
chain := NewInterceptorChain(
    registry.Get("logging"),
    registry.Get("timeout"),
)
```

#### 5. Optional: Use Per-Request Config

Disable extensions for specific requests:

```go
// Disable caching for a sensitive request
req := &GenerateRequest{
    Prompt: "Sensitive data",
    Metadata: map[string]interface{}{
        "extensions": map[string]interface{}{
            "caching": map[string]interface{}{
                "enabled": false,
            },
        },
    },
}
```

### Breaking Changes

**Minor** - Extensions must implement `Priority()` method.

**Migration for custom extensions:**
```go
// Add this method to existing extensions:
func (e *MyExtension) Priority() int {
    return PriorityTransform // Choose appropriate priority
}
```

### Deprecations

**None** - All existing APIs maintained.

### Testing Recommendations

- Test extension execution order with priorities
- Verify per-request config works as expected
- Test interceptor chains
- Ensure capability-based interfaces work correctly
- Test backwards compatibility with v1.0.2 extensions

---

## Migrating to v1.0.2 (from v1.0.1)

**Release Date:** 2025-11-29
**Type:** Enhancement
**Risk Level:** Low

### What's New

Added SSE (Server-Sent Events) streaming support to `GenerateHandler`. Streaming requests no longer need separate endpoint handling.

### Migration Steps

#### 1. Update Dependencies

```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.2
go mod tidy
```

#### 2. Update Streaming Endpoints (Optional)

**Before (v1.0.1):**
```go
// Had to route streaming to separate endpoint
router.POST("/api/generate", handlers.GenerateHandler)
router.POST("/api/stream", handlers.StreamHandler) // Separate handler
```

**After (v1.0.2):**
```go
// Single endpoint handles both
router.POST("/api/generate", handlers.GenerateHandler)
// No need for separate stream endpoint
```

#### 3. Client Code Remains Same

Client streaming code doesn't change:

```go
req := &GenerateRequest{
    Prompt: "Hello",
    Stream: true, // SSE streaming
}

resp, err := http.Post("/api/generate", "application/json", body)
// Read SSE events from response
```

### What You Get

- **Unified endpoint**: Single endpoint for streaming and non-streaming
- **Proper SSE headers**: Automatic `Content-Type: text/event-stream` setup
- **Extension integration**: BeforeGenerate, AfterGenerate hooks work with streaming
- **Error handling**: Errors sent as SSE error events
- **Done signaling**: Explicit `done` event marks stream completion

### SSE Event Format

```
data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" world"}}]}

event: done
data: {}

```

### Breaking Changes

**None** - This is backwards compatible.

### Testing Recommendations

- Test SSE streaming responses
- Verify extension hooks fire correctly
- Test error handling in streams
- Verify done events sent

---

## Multi-Version Migrations

### Skipping Multiple Versions

If migrating across multiple versions (e.g., v1.0.0 → v1.0.16), follow these steps:

#### 1. Identify Breaking Changes

Review all versions between your current and target version for breaking changes:

| From | To | Breaking Changes |
|------|-----|-----------------|
| v1.0.0 | v1.0.16 | v1.0.3, v1.0.5, v1.0.11, v1.0.13 |
| v1.0.5 | v1.0.16 | v1.0.11, v1.0.13 |
| v1.0.10 | v1.0.16 | v1.0.11, v1.0.13 |

#### 2. Sequential Migration Strategy

**Recommended approach:**

```bash
# Migrate to each breaking version sequentially
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.3
# Fix breaking changes, test
go test ./...

go get github.com/cecil-the-coder/ai-provider-kit@v1.0.5
# Fix breaking changes, test
go test ./...

go get github.com/cecil-the-coder/ai-provider-kit@v1.0.11
# Fix breaking changes, test
go test ./...

go get github.com/cecil-the-coder/ai-provider-kit@v1.0.13
# Fix breaking changes, test
go test ./...

go get github.com/cecil-the-coder/ai-provider-kit@v1.0.16
# Final test
go test ./...
```

#### 3. Direct Migration Strategy

**For experienced users:**

```bash
# Jump directly to target version
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.16
go mod tidy

# Fix all breaking changes at once
# - Add Priority() to extensions (v1.0.3)
# - Add explicit system prompts for Anthropic (v1.0.5)
# - Update to ExecuteWithAuthMessage for tool calling (v1.0.11)
# - Update import paths for common sub-packages (v1.0.13)

# Test thoroughly
go test ./...
```

### Example: v1.0.0 → v1.0.16 Migration

Complete migration guide for the full version range:

#### Step 1: Update Dependencies
```bash
go get github.com/cecil-the-coder/ai-provider-kit@v1.0.16
go mod tidy
```

#### Step 2: Fix Extension Priority (v1.0.3)
```go
// Add to all custom extensions
func (e *MyExtension) Priority() int {
    return PriorityTransform
}
```

#### Step 3: Fix Anthropic System Prompts (v1.0.5)
```go
// Add explicit system prompt for code generation
options := types.GenerateOptions{
    Prompt: "Write a function",
    Messages: []types.ChatMessage{
        {
            Role: "system",
            Content: "You are an expert programmer.",
        },
    },
}
```

#### Step 4: Update Tool Calling (v1.0.11)
```go
// Replace ExecuteWithAuth with ExecuteWithAuthMessage
message, err := authHelper.ExecuteWithAuthMessage(ctx, func(apiKey string) (types.ChatMessage, error) {
    return types.ChatMessage{
        Role:      "assistant",
        Content:   "response",
        ToolCalls: toolCalls,
    }, nil
})
```

#### Step 5: Update Import Paths (v1.0.13)
```go
// Replace
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"

// With
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/auth"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/config"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/models"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common/streaming"
)
```

#### Step 6: Test Everything
```bash
go test ./...
go build ./...
```

---

## Migration Checklist

Use this checklist when migrating between any versions:

### Pre-Migration

- [ ] Review release notes for target version
- [ ] Identify breaking changes in range
- [ ] Back up current codebase
- [ ] Create feature branch for migration
- [ ] Document current behavior/tests

### During Migration

- [ ] Update go.mod to target version
- [ ] Run `go mod tidy`
- [ ] Fix compilation errors
- [ ] Update import statements
- [ ] Update deprecated API calls
- [ ] Add new required methods/fields
- [ ] Update tests
- [ ] Run full test suite

### Post-Migration

- [ ] Verify all tests pass
- [ ] Run integration tests
- [ ] Test in staging environment
- [ ] Compare behavior with previous version
- [ ] Update documentation
- [ ] Update CI/CD pipelines
- [ ] Monitor for runtime issues

### Version-Specific Checks

#### For v1.0.13
- [ ] Update all `providers/common` imports to sub-packages
- [ ] Verify Gemini tool format usage
- [ ] Test custom providers with new imports

#### For v1.0.11
- [ ] Migrate to `ExecuteWithAuthMessage` for tool calling
- [ ] Test OAuth providers with tools
- [ ] Verify `ChatMessage` responses

#### For v1.0.10
- [ ] Test context-based OAuth injection (if using)
- [ ] Verify context tokens take priority

#### For v1.0.5
- [ ] Add explicit system prompts for Anthropic code generation
- [ ] Test all Anthropic-based workflows
- [ ] Verify non-code requests work correctly

#### For v1.0.3
- [ ] Add `Priority()` method to all extensions
- [ ] Test extension execution order
- [ ] Implement capability interfaces if needed

#### For v1.0.2
- [ ] Test SSE streaming endpoints
- [ ] Verify extension hooks work with streaming
- [ ] Test error handling in streams

---

## Support and Resources

### Getting Help

- **Documentation**: https://pkg.go.dev/github.com/cecil-the-coder/ai-provider-kit
- **Issues**: https://github.com/cecil-the-coder/ai-provider-kit/issues
- **Discussions**: https://github.com/cecil-the-coder/ai-provider-kit/discussions

### Reporting Migration Issues

When reporting migration issues, include:

1. **Version information**: Current version → Target version
2. **Error messages**: Full compilation or runtime errors
3. **Code samples**: Minimal reproducible example
4. **Migration steps taken**: What you've tried
5. **Expected vs actual behavior**

### Example Migration Issue Report

```markdown
**Title**: Import error after upgrading from v1.0.12 to v1.0.13

**Current Version**: v1.0.12
**Target Version**: v1.0.13

**Error**:
```
cannot find package "github.com/.../providers/common" imported as common
```

**Steps Taken**:
1. Updated go.mod to v1.0.13
2. Ran go mod tidy
3. Attempted to build

**Code Sample**:
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/providers/common"

func MyFunc() {
    helper := common.NewAuthHelper(config)
}
```

**Expected**: Should compile
**Actual**: Import not found error
```

---

## Appendix: Complete API Change Log

### Type Changes

| Version | Type | Change | Impact |
|---------|------|--------|--------|
| v1.0.16 | `GenerateRequest` | Added `Context` field | Low - Optional field |
| v1.0.13 | `types.ToolFormat` | Added `ToolFormatGemini` | Low - New constant |

### Method Changes

| Version | Method | Change | Impact |
|---------|--------|--------|--------|
| v1.0.16 | `WithAPIKey/GetAPIKey` | Added | Low - New helpers |
| v1.0.11 | `ExecuteWithAuthMessage` | Added | Medium - New method for tool calls |
| v1.0.10 | `WithOAuthToken/GetOAuthToken` | Added | Low - New helpers |
| v1.0.3 | `Priority()` | Added to Extension interface | Medium - Required implementation |

### Package Changes

| Version | Package | Change | Impact |
|---------|---------|--------|--------|
| v1.0.13 | `providers/common` | Split into sub-packages | High - Import paths changed |

### Behavior Changes

| Version | Component | Change | Impact |
|---------|-----------|--------|--------|
| v1.0.5 | Anthropic Provider | Removed default system prompt | Medium - Explicit prompts needed |
| v1.0.9 | Gemini Provider | Static model list | Low - Performance improvement |

---

**Document Version**: 1.0
**Last Updated**: 2025-12-03
**Covers Versions**: v1.0.0 through v1.0.16
