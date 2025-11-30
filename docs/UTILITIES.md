# Utility Functions Reference

Comprehensive documentation for the `pkg/utils` package - primitive utility functions for token estimation, tool call validation, and embedded error detection.

## Table of Contents

1. [Overview](#overview)
2. [Token Estimation](#token-estimation)
3. [Tool Call Validation](#tool-call-validation)
4. [Embedded Error Detection](#embedded-error-detection)
5. [Usage Examples](#usage-examples)
6. [Best Practices](#best-practices)

---

## Overview

The `pkg/utils` package provides a collection of primitive utility functions that form the building blocks for handling common challenges in AI provider interactions. These utilities follow a **primitives, not patterns** design philosophy.

### Design Philosophy: Primitives, Not Patterns

Unlike higher-level abstractions, these utilities provide fundamental building blocks that enable you to compose your own solutions:

- **No Opinions**: Utilities detect and report; they don't enforce behaviors
- **Composable**: Mix and match to build your own patterns
- **Provider-Agnostic**: Work with any AI provider or custom implementation
- **Lightweight**: Minimal dependencies, maximum flexibility
- **Transparent**: Clear inputs, predictable outputs, no hidden state

This approach allows you to build sophisticated systems - whether that's custom caching layers, validation pipelines, or error handling strategies - without being constrained by opinionated frameworks.

### Package Structure

```
pkg/utils/
├── tokens.go       # Token estimation utilities
├── toolcalls.go    # Tool call sequence validation
└── errors.go       # Embedded error detection
```

### Benefits

1. **Flexibility**: Build your own patterns using these primitives
2. **No Lock-in**: No framework-specific abstractions
3. **Easy Testing**: Pure functions with predictable behavior
4. **Provider Agnostic**: Works with any AI provider
5. **Production Ready**: Battle-tested utilities for real-world scenarios

---

## Token Estimation

Fast, approximation-based token counting for routing decisions and context management. These utilities use empirical averages (~4.7 bytes per token) to provide instant estimates without actual tokenization.

### Use Cases

- **Model Selection**: Route requests to appropriate models based on context size
- **Cost Estimation**: Approximate costs before making API calls
- **Context Management**: Quickly check if messages fit within context windows
- **Preprocessing**: Filter or truncate content before sending to providers
- **Validation**: Pre-flight checks for context limits

### Functions

#### EstimateTokensFromBytes

Estimates token count from byte length using empirical ratio.

```go
func EstimateTokensFromBytes(byteCount int) int
```

**Parameters:**
- `byteCount`: Number of bytes to estimate tokens for

**Returns:** Estimated token count (0 for non-positive input)

**Details:**
- Uses ratio of ~4.7 bytes per token
- Implemented as `(byteCount * 10) / 47` to avoid floating point
- Returns 0 for byteCount ≤ 0

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

content := "Your long text content here..."
bytes := len(content)
tokens := utils.EstimateTokensFromBytes(bytes)
fmt.Printf("Estimated %d tokens from %d bytes\n", tokens, bytes)
```

#### EstimateTokensFromString

Convenience wrapper for estimating tokens from string content.

```go
func EstimateTokensFromString(s string) int
```

**Parameters:**
- `s`: String content to estimate tokens for

**Returns:** Estimated token count

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

prompt := "Explain quantum computing in simple terms"
tokens := utils.EstimateTokensFromString(prompt)
fmt.Printf("Prompt uses ~%d tokens\n", tokens)
```

#### EstimateTokensFromMessages

Estimates total tokens across a conversation history, extracting text from both simple and multimodal messages.

```go
func EstimateTokensFromMessages(messages []types.ChatMessage) int
```

**Parameters:**
- `messages`: Slice of ChatMessage to estimate tokens for

**Returns:** Total estimated token count across all messages

**Details:**
- Uses `GetTextContent()` to extract text from messages
- Handles both simple string content and multimodal Parts
- Accumulates tokens across all messages in the conversation

**Example:**
```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

messages := []types.ChatMessage{
    {Role: "user", Content: "What is the weather?"},
    {Role: "assistant", Content: "I'll check that for you."},
    {Role: "user", Content: "Thanks!"},
}

totalTokens := utils.EstimateTokensFromMessages(messages)
fmt.Printf("Conversation uses ~%d tokens\n", totalTokens)
```

### Constants

#### BytesPerToken

The empirically-derived average bytes per token constant for custom calculations.

```go
const BytesPerToken = 4.7
```

**Use Case:** When you need to perform custom token/byte conversions outside the provided functions.

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

// Custom calculation
maxBytes := int(float64(maxTokens) * utils.BytesPerToken)
```

#### TokenThreshold Constants

Common context window sizes in tokens for quick comparisons.

```go
const (
    TokenThreshold4K   = 4096     // 4K context window
    TokenThreshold8K   = 8192     // 8K context window
    TokenThreshold16K  = 16384    // 16K context window
    TokenThreshold32K  = 32768    // 32K context window
    TokenThreshold128K = 131072   // 128K context window
)
```

**Use Case:** Route requests to appropriate models based on estimated token count.

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

tokens := utils.EstimateTokensFromMessages(messages)

switch {
case tokens < utils.TokenThreshold4K:
    model = "fast-model-4k"
case tokens < utils.TokenThreshold16K:
    model = "standard-model-16k"
case tokens < utils.TokenThreshold128K:
    model = "large-context-model-128k"
default:
    return fmt.Errorf("conversation too long: %d tokens", tokens)
}
```

#### ByteThresholdForTokens

Converts token thresholds to approximate byte sizes for quick content-length routing.

```go
func ByteThresholdForTokens(tokens int) int
```

**Parameters:**
- `tokens`: Token count to convert to bytes

**Returns:** Approximate byte count for the given token count

**Use Case:** Quick byte-based checks without string conversion.

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

maxBytes := utils.ByteThresholdForTokens(utils.TokenThreshold8K)
if len(content) > maxBytes {
    // Content likely exceeds 8K tokens
    content = content[:maxBytes]
}
```

---

## Tool Call Validation

Utilities for validating and managing tool call sequences in multi-turn conversations. These primitives help detect missing tool responses, orphaned responses, and fix incomplete sequences.

### Use Cases

- **Debugging**: Identify incomplete tool call sequences during development
- **Validation**: Ensure conversation history is well-formed before API calls
- **Auto-Recovery**: Automatically inject placeholder responses for missing tool calls
- **Testing**: Verify tool call handling in integration tests
- **Monitoring**: Detect and log tool call issues in production

### Types

#### ToolCallValidationError

Represents a missing or invalid tool response in a conversation sequence.

```go
type ToolCallValidationError struct {
    ToolCallID   string  // ID of the problematic tool call
    ToolName     string  // Name of the tool (for missing responses)
    MessageIndex int     // Index in message array where issue occurred
    Issue        string  // "missing_response" or "orphan_response"
}
```

**Fields:**
- `ToolCallID`: The tool call ID that has an issue
- `ToolName`: Tool name (populated for missing_response errors)
- `MessageIndex`: Position in the messages array
- `Issue`: Type of validation error

### Functions

#### ValidateToolCallSequence

Checks if all tool calls have matching responses in a conversation.

```go
func ValidateToolCallSequence(messages []types.ChatMessage) []ToolCallValidationError
```

**Parameters:**
- `messages`: Conversation history to validate

**Returns:**
- `nil` if sequence is valid
- Slice of `ToolCallValidationError` if issues found

**Validation Logic:**
- Tracks tool calls from assistant messages
- Matches them with corresponding tool responses
- Reports missing responses (tool calls without responses)
- Reports orphan responses (responses without matching calls)

**Example:**
```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

messages := []types.ChatMessage{
    {
        Role: "assistant",
        ToolCalls: []types.ToolCall{
            {ID: "call_123", Function: types.ToolFunction{Name: "get_weather"}},
        },
    },
    // Missing tool response here
}

errors := utils.ValidateToolCallSequence(messages)
if errors != nil {
    for _, err := range errors {
        fmt.Printf("Issue: %s for tool call %s at index %d\n",
            err.Issue, err.ToolCallID, err.MessageIndex)
    }
}
```

#### HasPendingToolCalls

Quick check for whether there are tool calls without responses.

```go
func HasPendingToolCalls(messages []types.ChatMessage) bool
```

**Parameters:**
- `messages`: Conversation history to check

**Returns:** `true` if there are pending tool calls, `false` otherwise

**Use Case:** Fast boolean check before validation or processing.

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

if utils.HasPendingToolCalls(messages) {
    // Handle pending tool calls
    pending := utils.GetPendingToolCalls(messages)
    fmt.Printf("Found %d pending tool calls\n", len(pending))
}
```

#### GetPendingToolCalls

Retrieves all tool calls that don't have responses yet.

```go
func GetPendingToolCalls(messages []types.ChatMessage) []types.ToolCall
```

**Parameters:**
- `messages`: Conversation history to analyze

**Returns:** Slice of ToolCall objects that are pending responses

**Details:**
- Returns empty slice if no pending calls
- Useful for determining which tools need execution
- Can be used to build automated tool execution pipelines

**Example:**
```go
import (
    "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

pending := utils.GetPendingToolCalls(messages)
for _, toolCall := range pending {
    // Execute each pending tool
    result := executeToolFunction(toolCall)

    // Add result to messages
    messages = append(messages, types.ChatMessage{
        Role:       "tool",
        Content:    result,
        ToolCallID: toolCall.ID,
    })
}
```

#### FixMissingToolResponses

Returns a new message slice with injected placeholder responses for missing tool calls.

```go
func FixMissingToolResponses(messages []types.ChatMessage, defaultResponse string) []types.ChatMessage
```

**Parameters:**
- `messages`: Original conversation history
- `defaultResponse`: Placeholder content for missing responses

**Returns:** New message slice with injected tool responses

**Details:**
- Returns original messages if no pending calls
- Appends tool response messages for each pending call
- Does not modify original slice (creates new one)
- Useful for recovery scenarios or testing

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

// Auto-fix missing responses with placeholder
fixed := utils.FixMissingToolResponses(messages, "Error: Tool execution timed out")

// Now the sequence is valid
errors := utils.ValidateToolCallSequence(fixed)
fmt.Printf("After fix: %d validation errors\n", len(errors))
```

---

## Embedded Error Detection

Utilities for scanning response bodies for embedded error patterns that might not trigger HTTP errors. Many providers return 200 OK with error messages in the response body.

### Use Cases

- **Error Detection**: Catch provider errors that bypass HTTP status codes
- **Quota Monitoring**: Detect quota/rate limit issues early
- **Custom Patterns**: Define your own error detection rules
- **Debugging**: Extract context around errors for debugging
- **Provider Quirks**: Handle provider-specific error patterns

### Types

#### EmbeddedError

Represents an error found in a successful response body.

```go
type EmbeddedError struct {
    Pattern string // The pattern that matched
    Context string // Surrounding text for debugging (up to 100 chars)
}
```

**Methods:**
```go
func (e *EmbeddedError) Error() string
```

**Fields:**
- `Pattern`: The error pattern that was matched
- `Context`: Text surrounding the match (with "..." markers)

**Example:**
```go
err := utils.CheckCommonErrors(responseBody)
if err != nil {
    embeddedErr := err.(*utils.EmbeddedError)
    fmt.Printf("Found error pattern: %s\n", embeddedErr.Pattern)
    fmt.Printf("Context: %s\n", embeddedErr.Context)
}
```

### Constants

#### CommonErrorPatterns

Slice of common provider error patterns for out-of-the-box detection.

```go
var CommonErrorPatterns = []string{
    "token quota is not enough",
    "rate limit exceeded",
    "context length exceeded",
    "insufficient_quota",
    "model_not_found",
    "invalid_api_key",
    "quota exceeded",
    "capacity exceeded",
    "overloaded",
}
```

**Use Case:** Ready-made patterns covering most common provider errors.

**Patterns Detected:**
- Quota/rate limiting issues
- Context length problems
- Authentication failures
- Model availability issues
- Service capacity problems

### Functions

#### CheckEmbeddedErrors

Scans response body for error patterns with case-insensitive matching.

```go
func CheckEmbeddedErrors(body string, patterns []string) *EmbeddedError
```

**Parameters:**
- `body`: Response body to scan
- `patterns`: Error patterns to search for

**Returns:**
- `nil` if no errors found
- `*EmbeddedError` for the first matching pattern

**Details:**
- Case-insensitive matching
- Returns immediately on first match
- Extracts 30 characters before and after match for context
- Adds "..." markers when context is truncated
- Returns nil for empty body or patterns

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

customPatterns := []string{
    "service unavailable",
    "maintenance mode",
    "database error",
}

if err := utils.CheckEmbeddedErrors(responseBody, customPatterns); err != nil {
    log.Printf("Embedded error detected: %s", err.Error())
    log.Printf("Context: %s", err.(*utils.EmbeddedError).Context)
}
```

#### CheckCommonErrors

Convenience function using CommonErrorPatterns for quick validation.

```go
func CheckCommonErrors(body string) *EmbeddedError
```

**Parameters:**
- `body`: Response body to scan

**Returns:** `*EmbeddedError` if common error found, `nil` otherwise

**Use Case:** Quick validation using built-in patterns.

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

responseBody := `{"status": "ok", "error": "rate limit exceeded", "data": null}`

if err := utils.CheckCommonErrors(responseBody); err != nil {
    // Handle embedded error
    return fmt.Errorf("provider error: %w", err)
}
```

#### ContainsAnyPattern

Boolean check for whether body contains any of the given patterns.

```go
func ContainsAnyPattern(body string, patterns []string) bool
```

**Parameters:**
- `body`: Text to search
- `patterns`: Patterns to look for

**Returns:** `true` if any pattern found, `false` otherwise

**Use Case:** Fast boolean check without error details.

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

if utils.ContainsAnyPattern(responseBody, []string{"error", "failed"}) {
    // Quick check without detailed error info
    return errors.New("response contains error markers")
}
```

#### ContainsCommonErrors

Boolean check using CommonErrorPatterns.

```go
func ContainsCommonErrors(body string) bool
```

**Parameters:**
- `body`: Response body to check

**Returns:** `true` if common errors found, `false` otherwise

**Example:**
```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"

if utils.ContainsCommonErrors(responseBody) {
    metrics.IncrementEmbeddedErrors()
}
```

---

## Usage Examples

### Example 1: Smart Model Selection Based on Context Size

Build a routing system that automatically selects the appropriate model based on conversation length.

```go
package main

import (
    "context"
    "fmt"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type ModelRouter struct {
    smallProvider  types.Provider  // 4K context
    mediumProvider types.Provider  // 16K context
    largeProvider  types.Provider  // 128K context
}

func (r *ModelRouter) RouteRequest(ctx context.Context, messages []types.ChatMessage, opts types.GenerateOptions) (types.ChatCompletionStream, error) {
    // Estimate tokens
    tokens := utils.EstimateTokensFromMessages(messages)

    // Select provider based on context size
    var provider types.Provider
    switch {
    case tokens < utils.TokenThreshold4K:
        provider = r.smallProvider
        fmt.Printf("Using fast model (estimated %d tokens)\n", tokens)
    case tokens < utils.TokenThreshold16K:
        provider = r.mediumProvider
        fmt.Printf("Using standard model (estimated %d tokens)\n", tokens)
    case tokens < utils.TokenThreshold128K:
        provider = r.largeProvider
        fmt.Printf("Using large context model (estimated %d tokens)\n", tokens)
    default:
        return nil, fmt.Errorf("conversation too long: ~%d tokens exceeds 128K limit", tokens)
    }

    opts.Messages = messages
    return provider.GenerateChatCompletion(ctx, opts)
}
```

### Example 2: Automatic Tool Call Recovery

Build a resilient tool execution system that handles missing responses.

```go
package main

import (
    "fmt"
    "time"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type ToolExecutor struct {
    timeout time.Duration
}

func (t *ToolExecutor) ExecuteWithRecovery(messages []types.ChatMessage) ([]types.ChatMessage, error) {
    // Validate current sequence
    if errors := utils.ValidateToolCallSequence(messages); errors != nil {
        fmt.Printf("Found %d validation errors\n", len(errors))
        for _, err := range errors {
            fmt.Printf("  - %s: %s (call_id: %s)\n", err.Issue, err.ToolName, err.ToolCallID)
        }
    }

    // Get pending tool calls
    pending := utils.GetPendingToolCalls(messages)
    if len(pending) == 0 {
        return messages, nil
    }

    fmt.Printf("Executing %d pending tool calls...\n", len(pending))

    // Execute each pending tool with timeout
    for _, toolCall := range pending {
        result := t.executeToolWithTimeout(toolCall)

        messages = append(messages, types.ChatMessage{
            Role:       "tool",
            Content:    result,
            ToolCallID: toolCall.ID,
        })
    }

    // Verify sequence is now valid
    if errors := utils.ValidateToolCallSequence(messages); errors != nil {
        return nil, fmt.Errorf("sequence still invalid after execution: %v", errors)
    }

    return messages, nil
}

func (t *ToolExecutor) executeToolWithTimeout(toolCall types.ToolCall) string {
    // Simulate tool execution with timeout
    done := make(chan string, 1)

    go func() {
        // Your actual tool execution here
        time.Sleep(100 * time.Millisecond)
        done <- fmt.Sprintf("Result from %s", toolCall.Function.Name)
    }()

    select {
    case result := <-done:
        return result
    case <-time.After(t.timeout):
        return fmt.Sprintf("Error: Tool execution timed out after %v", t.timeout)
    }
}
```

### Example 3: Robust Error Detection Pipeline

Build a comprehensive error detection system with custom patterns.

```go
package main

import (
    "fmt"
    "io"
    "net/http"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
)

type ErrorDetector struct {
    customPatterns []string
    metrics        *MetricsCollector
}

func NewErrorDetector(metrics *MetricsCollector) *ErrorDetector {
    return &ErrorDetector{
        customPatterns: []string{
            // Add provider-specific patterns
            "service temporarily unavailable",
            "model is currently overloaded",
            "upstream timeout",
        },
        metrics: metrics,
    }
}

func (d *ErrorDetector) CheckResponse(resp *http.Response, body []byte) error {
    bodyStr := string(body)

    // First check: HTTP status
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("HTTP error: %d", resp.StatusCode)
    }

    // Second check: Common error patterns
    if err := utils.CheckCommonErrors(bodyStr); err != nil {
        embeddedErr := err.(*utils.EmbeddedError)
        d.metrics.RecordEmbeddedError(embeddedErr.Pattern)
        return fmt.Errorf("embedded error: %w (context: %s)", err, embeddedErr.Context)
    }

    // Third check: Custom patterns
    if err := utils.CheckEmbeddedErrors(bodyStr, d.customPatterns); err != nil {
        embeddedErr := err.(*utils.EmbeddedError)
        d.metrics.RecordCustomError(embeddedErr.Pattern)
        return fmt.Errorf("custom pattern error: %w", err)
    }

    // Fourth check: Boolean flags
    if utils.ContainsAnyPattern(bodyStr, []string{"warning", "deprecated"}) {
        d.metrics.RecordWarning()
        // Don't return error, just log
        fmt.Println("Warning detected in response")
    }

    return nil
}

type MetricsCollector struct {
    embeddedErrors map[string]int
    customErrors   map[string]int
    warnings       int
}

func (m *MetricsCollector) RecordEmbeddedError(pattern string) {
    if m.embeddedErrors == nil {
        m.embeddedErrors = make(map[string]int)
    }
    m.embeddedErrors[pattern]++
}

func (m *MetricsCollector) RecordCustomError(pattern string) {
    if m.customErrors == nil {
        m.customErrors = make(map[string]int)
    }
    m.customErrors[pattern]++
}

func (m *MetricsCollector) RecordWarning() {
    m.warnings++
}
```

### Example 4: Pre-Flight Validation System

Combine all utilities for comprehensive pre-flight validation.

```go
package main

import (
    "fmt"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type RequestValidator struct {
    maxTokens int
}

func NewRequestValidator(maxTokens int) *RequestValidator {
    return &RequestValidator{maxTokens: maxTokens}
}

func (v *RequestValidator) Validate(messages []types.ChatMessage) error {
    // 1. Validate context size
    tokens := utils.EstimateTokensFromMessages(messages)
    if tokens > v.maxTokens {
        return fmt.Errorf("context too large: %d tokens (max: %d)", tokens, v.maxTokens)
    }

    // 2. Validate tool call sequence
    if errors := utils.ValidateToolCallSequence(messages); errors != nil {
        return fmt.Errorf("invalid tool sequence: %d errors", len(errors))
    }

    // 3. Check for pending tool calls
    if utils.HasPendingToolCalls(messages) {
        pending := utils.GetPendingToolCalls(messages)
        return fmt.Errorf("%d pending tool calls must be resolved", len(pending))
    }

    return nil
}

func (v *RequestValidator) ValidateWithRecovery(messages []types.ChatMessage) ([]types.ChatMessage, error) {
    // Check context size
    tokens := utils.EstimateTokensFromMessages(messages)
    if tokens > v.maxTokens {
        return nil, fmt.Errorf("context too large: %d tokens", tokens)
    }

    // Auto-fix missing tool responses
    if utils.HasPendingToolCalls(messages) {
        fmt.Println("Auto-fixing missing tool responses...")
        messages = utils.FixMissingToolResponses(messages, "Error: Tool execution not completed")
    }

    // Final validation
    if err := v.Validate(messages); err != nil {
        return nil, err
    }

    return messages, nil
}
```

### Example 5: Content Preprocessing Pipeline

Use token estimation for intelligent content truncation.

```go
package main

import (
    "fmt"

    "github.com/cecil-the-coder/ai-provider-kit/pkg/utils"
    "github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

type ContentProcessor struct {
    targetTokens int
}

func NewContentProcessor(targetTokens int) *ContentProcessor {
    return &ContentProcessor{targetTokens: targetTokens}
}

func (p *ContentProcessor) TruncateToFit(messages []types.ChatMessage) []types.ChatMessage {
    // Estimate current size
    currentTokens := utils.EstimateTokensFromMessages(messages)

    if currentTokens <= p.targetTokens {
        return messages // Already fits
    }

    fmt.Printf("Truncating from ~%d to ~%d tokens\n", currentTokens, p.targetTokens)

    // Strategy: Keep system message and most recent messages
    result := []types.ChatMessage{}

    // Always keep system message
    if len(messages) > 0 && messages[0].Role == "system" {
        result = append(result, messages[0])
        messages = messages[1:]
    }

    // Calculate target bytes for remaining messages
    usedTokens := utils.EstimateTokensFromMessages(result)
    remainingTokens := p.targetTokens - usedTokens
    targetBytes := utils.ByteThresholdForTokens(remainingTokens)

    // Add messages from end until we approach limit
    currentBytes := 0
    for i := len(messages) - 1; i >= 0; i-- {
        msgBytes := len(messages[i].GetTextContent())
        if currentBytes+msgBytes > targetBytes {
            break
        }
        result = append([]types.ChatMessage{messages[i]}, result...)
        currentBytes += msgBytes
    }

    finalTokens := utils.EstimateTokensFromMessages(result)
    fmt.Printf("Truncated to %d messages (~%d tokens)\n", len(result), finalTokens)

    return result
}
```

---

## Best Practices

### Token Estimation

1. **Understand It's an Estimate**: Token counts are approximate (~4.7 bytes/token average). For exact counts, use provider-specific tokenizers.

2. **Use for Routing, Not Billing**: Perfect for quick decisions, but use actual API responses for cost tracking.

3. **Add Safety Margins**: When routing based on estimates, leave 10-20% buffer below actual limits.

```go
// Good: Leave safety margin
if tokens < int(float64(utils.TokenThreshold4K) * 0.8) {
    // Use 4K model
}

// Risky: No margin for estimation error
if tokens < utils.TokenThreshold4K {
    // Might fail if estimate is low
}
```

4. **Combine with Byte Checks**: Use `ByteThresholdForTokens` for ultra-fast preprocessing.

```go
// Fast byte-based pre-check before string processing
maxBytes := utils.ByteThresholdForTokens(8192)
if len(rawContent) > maxBytes {
    rawContent = rawContent[:maxBytes]
}
```

### Tool Call Validation

1. **Validate Before API Calls**: Catch issues early, before sending to providers.

```go
// Validate before expensive API call
if errors := utils.ValidateToolCallSequence(messages); errors != nil {
    return fmt.Errorf("cannot proceed: %v", errors)
}
```

2. **Use HasPendingToolCalls for Quick Checks**: Faster than full validation when you only need boolean result.

```go
// Quick check in hot paths
if !utils.HasPendingToolCalls(messages) {
    return provider.GenerateChatCompletion(ctx, opts)
}
```

3. **FixMissingToolResponses for Recovery**: Good for testing and development, but investigate root causes in production.

```go
// Development/Testing
messages = utils.FixMissingToolResponses(messages, "placeholder")

// Production: Log and investigate
if pending := utils.GetPendingToolCalls(messages); len(pending) > 0 {
    logger.Error("pending tool calls detected", "count", len(pending))
    // Implement proper recovery
}
```

4. **Combine Validation Functions**: Build comprehensive checks.

```go
func validateConversation(messages []types.ChatMessage) error {
    // Quick check first
    if !utils.HasPendingToolCalls(messages) {
        return nil
    }

    // Detailed validation
    errors := utils.ValidateToolCallSequence(messages)
    if len(errors) > 0 {
        return fmt.Errorf("validation failed: %v", errors)
    }

    return nil
}
```

### Embedded Error Detection

1. **Check After Every API Call**: Don't trust HTTP 200 - many providers embed errors in response body.

```go
if resp.StatusCode == 200 {
    if err := utils.CheckCommonErrors(body); err != nil {
        return fmt.Errorf("embedded error: %w", err)
    }
}
```

2. **Use Context for Debugging**: Extract the context field for detailed error investigation.

```go
if err := utils.CheckCommonErrors(body); err != nil {
    embeddedErr := err.(*utils.EmbeddedError)
    logger.Error("embedded error",
        "pattern", embeddedErr.Pattern,
        "context", embeddedErr.Context,
    )
}
```

3. **Extend with Custom Patterns**: Add provider-specific or application-specific patterns.

```go
allPatterns := append(utils.CommonErrorPatterns,
    "your_custom_error_pattern",
    "another_provider_specific_pattern",
)
err := utils.CheckEmbeddedErrors(body, allPatterns)
```

4. **Use Boolean Checks for Metrics**: Track error rates without detailed error handling.

```go
if utils.ContainsCommonErrors(body) {
    metrics.IncrementEmbeddedErrorRate()
}
```

### General Patterns

1. **Compose Utilities**: Build higher-level abstractions from these primitives.

```go
type RequestPipeline struct {
    validator  *RequestValidator
    detector   *ErrorDetector
    processor  *ContentProcessor
}
```

2. **Keep Utilities Pure**: Don't add state or side effects to utility function calls.

```go
// Good: Pure function
tokens := utils.EstimateTokensFromMessages(messages)

// Bad: Side effects
// utils.EstimateAndLog(messages) // Don't add logging to utilities
```

3. **Build Your Own Patterns**: These are primitives - compose them into patterns that fit your use case.

4. **Test Thoroughly**: Utilities have predictable behavior - easy to unit test.

```go
func TestValidation(t *testing.T) {
    messages := []types.ChatMessage{
        {Role: "user", Content: "test"},
    }

    tokens := utils.EstimateTokensFromMessages(messages)
    if tokens == 0 {
        t.Error("expected non-zero token estimate")
    }
}
```
