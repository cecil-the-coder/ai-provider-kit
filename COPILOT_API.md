# GitHub Copilot Native Integration - Technical Specification

This document contains all technical details required to implement native GitHub Copilot support in ai-provider-kit without external dependencies.

**Status:** Implementation Pending
**Last Updated:** 2025-12-10
**Based on:** copilot-api reverse engineering analysis

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication Flow](#authentication-flow)
3. [Token Management](#token-management)
4. [API Endpoints](#api-endpoints)
5. [Request/Response Formats](#requestresponse-formats)
6. [HTTP Headers](#http-headers)
7. [Implementation Details](#implementation-details)
8. [Error Handling](#error-handling)
9. [Rate Limiting](#rate-limiting)
10. [Usage Tracking](#usage-tracking)
11. [Constants Reference](#constants-reference)
12. [Complete Examples](#complete-examples)

---

## Overview

GitHub Copilot uses an OpenAI-compatible API accessible at `https://api.githubcopilot.com`. Authentication requires:
1. GitHub OAuth device flow to obtain GitHub token
2. Exchange GitHub token for Copilot-specific token
3. Periodic refresh of Copilot token (every ~19 minutes)

**Key Insight:** Copilot's API is natively OpenAI-compatible. No request/response translation needed - standard OpenAI Chat Completions format works directly.

---

## Authentication Flow

### Step 1: GitHub OAuth Device Code Flow

GitHub Copilot uses a hardcoded OAuth application for authentication.

#### 1.1: Request Device Code

**Endpoint:**
```
POST https://github.com/login/device/code
```

**Headers:**
```http
Content-Type: application/json
Accept: application/json
```

**Request Body:**
```json
{
  "client_id": "Iv1.b507a08c87ecfe98",
  "scope": "read:user"
}
```

**Response (200 OK):**
```json
{
  "device_code": "3584d83530557fdd1f46af8289938c8ef79f9dc5",
  "user_code": "WDJB-MJHT",
  "verification_uri": "https://github.com/login/device",
  "expires_in": 899,
  "interval": 5
}
```

**Response Fields:**
- `device_code`: Use this to poll for the access token
- `user_code`: Display this to the user
- `verification_uri`: User visits this URL to authorize
- `expires_in`: Device code expiration time (seconds)
- `interval`: Minimum seconds between polling requests

#### 1.2: User Authorization

**User Action Required:**
1. Display to user: "Visit https://github.com/login/device"
2. Display to user: "Enter code: WDJB-MJHT"
3. User completes authorization in browser

#### 1.3: Poll for Access Token

**Endpoint:**
```
POST https://github.com/login/oauth/access_token
```

**Headers:**
```http
Content-Type: application/json
Accept: application/json
```

**Request Body:**
```json
{
  "client_id": "Iv1.b507a08c87ecfe98",
  "device_code": "3584d83530557fdd1f46af8289938c8ef79f9dc5",
  "grant_type": "urn:ietf:params:oauth:grant-type:device_code"
}
```

**Polling Logic:**
- Sleep duration: `(interval + 1) * 1000` milliseconds
  - Add 1 second buffer to avoid rate limiting
  - Example: if `interval=5`, sleep 6 seconds between polls
- Continue polling until success or expiration
- Ignore non-200 responses (user hasn't authorized yet)

**Success Response (200 OK):**
```json
{
  "access_token": "gho_16C7e42F292c6912E7710c838347Ae178B4a",
  "token_type": "bearer",
  "scope": "read:user"
}
```

**Pending Response (varies):**
```json
{
  "error": "authorization_pending",
  "error_description": "The authorization request is still pending."
}
```

Continue polling when receiving `authorization_pending`.

**Error Responses:**
```json
{
  "error": "expired_token",
  "error_description": "The device code has expired."
}
```

### Step 2: Exchange GitHub Token for Copilot Token

Once you have the GitHub access token, exchange it for a Copilot-specific token.

**Endpoint:**
```
GET https://api.github.com/copilot_internal/v2/token
```

**Headers:**
```http
Content-Type: application/json
Accept: application/json
Authorization: token gho_16C7e42F292c6912E7710c838347Ae178B4a
editor-version: vscode/1.104.3
editor-plugin-version: copilot-chat/0.26.7
user-agent: GitHubCopilotChat/0.26.7
x-github-api-version: 2025-04-01
x-vscode-user-agent-library-version: electron-fetch
```

**Response (200 OK):**
```json
{
  "expires_at": 1733861400,
  "refresh_in": 1200,
  "token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response Fields:**
- `token`: The Copilot API token (JWT format)
- `expires_at`: Unix timestamp when token expires
- `refresh_in`: Recommended refresh interval (seconds)

**Error Response (401 Unauthorized):**
```json
{
  "message": "Bad credentials",
  "documentation_url": "https://docs.github.com/rest"
}
```

This means:
- GitHub token is invalid/expired
- User doesn't have Copilot subscription
- Copilot access has been revoked

---

## Token Management

### Token Storage

**GitHub Token:**
- **Should be persisted:** Yes
- **Location:** `~/.config/ai-provider-kit/copilot_github_token` (or similar)
- **Format:** Plain text file with token string
- **Purpose:** Avoid re-authentication on restart
- **Security:** File should be chmod 600 (user read/write only)

**Copilot Token:**
- **Should be persisted:** No
- **Location:** In-memory only
- **Lifetime:** ~20 minutes (1200 seconds)
- **Purpose:** Used for API requests

### Token Refresh Logic

The Copilot token must be refreshed before it expires.

**Refresh Interval Calculation:**
```go
refreshInterval := (refreshIn - 60) * time.Second
```

**Example:**
- `refresh_in` from response: 1200 seconds
- Actual refresh interval: 1140 seconds (19 minutes)
- Safety buffer: 60 seconds

**Refresh Implementation:**

```go
func (p *CopilotProvider) startTokenRefreshLoop(ctx context.Context) {
    ticker := time.NewTicker(p.calculateRefreshInterval())
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := p.refreshCopilotToken(ctx); err != nil {
                log.Printf("Failed to refresh Copilot token: %v", err)
                // Token refresh failure is critical - service should stop
                return
            }
        case <-ctx.Done():
            return
        }
    }
}

func (p *CopilotProvider) refreshCopilotToken(ctx context.Context) error {
    // Make same request as initial token exchange
    // GET https://api.github.com/copilot_internal/v2/token
    // Update p.copilotToken with new token
    // Update refresh interval if changed
}
```

**Error Handling:**
- If refresh fails, retry once after 5 seconds
- If retry fails, invalidate token and require re-authentication
- Log errors for debugging

---

## API Endpoints

### Base URLs by Account Type

GitHub Copilot uses different base URLs based on account type:

```go
func getCopilotBaseURL(accountType string) string {
    switch accountType {
    case "individual":
        return "https://api.githubcopilot.com"
    case "business":
        return "https://api.business.githubcopilot.com"
    case "enterprise":
        return "https://api.enterprise.githubcopilot.com"
    default:
        return "https://api.githubcopilot.com"
    }
}
```

**Account Type Detection:**
- Can be inferred from GitHub user info
- Can be configured manually
- Default to "individual" if unknown

### Chat Completions

**Endpoint:**
```
POST {base_url}/chat/completions
```

**Format:** OpenAI Chat Completions API (identical)

**Request:** See [Request/Response Formats](#requestresponse-formats)

**Response:** OpenAI-compatible response

**Streaming:** Set `stream: true` for Server-Sent Events (SSE)

### Models

**Endpoint:**
```
GET {base_url}/models
```

**Headers:** Same as chat completions (see [HTTP Headers](#http-headers))

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "created": 1686935002,
      "owned_by": "openai",
      "name": "GPT-4o",
      "vendor": "OpenAI",
      "version": "2024-05-13",
      "preview": false,
      "model_picker_enabled": true,
      "capabilities": {
        "object": "model_capabilities",
        "family": "gpt-4o",
        "limits": {
          "max_context_window_tokens": 128000,
          "max_output_tokens": 16384,
          "max_prompt_tokens": 111616
        },
        "supports": {
          "tool_calls": true,
          "parallel_tool_calls": true
        },
        "tokenizer": "o200k_base",
        "type": "chat"
      }
    },
    {
      "id": "gpt-4o-mini",
      "object": "model",
      "created": 1686935002,
      "owned_by": "openai",
      "name": "GPT-4o mini",
      "vendor": "OpenAI",
      "version": "2024-07-18",
      "preview": false,
      "model_picker_enabled": true,
      "capabilities": {
        "object": "model_capabilities",
        "family": "gpt-4o-mini",
        "limits": {
          "max_context_window_tokens": 128000,
          "max_output_tokens": 16384,
          "max_prompt_tokens": 111616
        },
        "supports": {
          "tool_calls": true,
          "parallel_tool_calls": true
        },
        "tokenizer": "o200k_base",
        "type": "chat"
      }
    }
  ]
}
```

**Model Capabilities Fields:**
- `limits.max_context_window_tokens`: Total context window
- `limits.max_output_tokens`: Maximum output tokens
- `limits.max_prompt_tokens`: Maximum input tokens
- `supports.tool_calls`: Whether model supports function calling
- `supports.parallel_tool_calls`: Whether model supports parallel tool calls

**Usage:**
- Cache models list with 1-hour TTL
- Use `capabilities.limits.max_output_tokens` as default `max_tokens` if not specified
- Filter models by `model_picker_enabled: true` for user selection

### Embeddings

**Endpoint:**
```
POST {base_url}/embeddings
```

**Request:**
```json
{
  "input": "The quick brown fox jumps over the lazy dog",
  "model": "text-embedding-ada-002"
}
```

**Multiple inputs:**
```json
{
  "input": [
    "First text to embed",
    "Second text to embed"
  ],
  "model": "text-embedding-ada-002"
}
```

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.0023064255, -0.009327292, ...]
    }
  ],
  "model": "text-embedding-ada-002",
  "usage": {
    "prompt_tokens": 8,
    "total_tokens": 8
  }
}
```

---

## Request/Response Formats

### Chat Completion Request

Copilot uses **standard OpenAI Chat Completions format**.

**Full Request Schema:**
```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello!"
    }
  ],
  "temperature": 0.7,
  "top_p": 1.0,
  "max_tokens": 4096,
  "stop": null,
  "n": 1,
  "stream": false,
  "frequency_penalty": 0.0,
  "presence_penalty": 0.0,
  "logit_bias": null,
  "logprobs": false,
  "response_format": null,
  "seed": null,
  "tools": [],
  "tool_choice": "auto",
  "user": null
}
```

**Required Fields:**
- `model`: Model ID (e.g., "gpt-4o")
- `messages`: Array of message objects

**Optional Fields:**
- `temperature`: 0.0 to 2.0 (default: 1.0)
- `top_p`: 0.0 to 1.0 (default: 1.0)
- `max_tokens`: Maximum output tokens
- `stop`: String or array of stop sequences
- `n`: Number of completions to generate
- `stream`: Boolean for streaming
- `frequency_penalty`: -2.0 to 2.0
- `presence_penalty`: -2.0 to 2.0
- `logit_bias`: Token ID to bias mapping
- `logprobs`: Include log probabilities
- `response_format`: `{"type": "json_object"}` for JSON mode
- `seed`: Integer for deterministic sampling
- `tools`: Array of tool definitions
- `tool_choice`: "none" | "auto" | "required" | specific tool
- `user`: End-user identifier

### Message Format

**Basic Text Message:**
```json
{
  "role": "user",
  "content": "Hello, how are you?"
}
```

**Roles:**
- `"system"`: System instructions
- `"user"`: User message
- `"assistant"`: Assistant response
- `"tool"`: Tool/function result
- `"developer"`: Developer instructions (alternative to system)

**Multimodal Message (Text + Image):**
```json
{
  "role": "user",
  "content": [
    {
      "type": "text",
      "text": "What's in this image?"
    },
    {
      "type": "image_url",
      "image_url": {
        "url": "data:image/jpeg;base64,/9j/4AAQSkZJRg...",
        "detail": "high"
      }
    }
  ]
}
```

**Image URL Options:**
- **Base64:** `data:image/jpeg;base64,...` (recommended)
- **HTTP URL:** `https://example.com/image.jpg`
- **Detail level:** `"low"` | `"high"` | `"auto"`

**Assistant Message with Tool Calls:**
```json
{
  "role": "assistant",
  "content": null,
  "tool_calls": [
    {
      "id": "call_abc123",
      "type": "function",
      "function": {
        "name": "get_weather",
        "arguments": "{\"location\":\"San Francisco\",\"unit\":\"celsius\"}"
      }
    }
  ]
}
```

**Tool Result Message:**
```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "{\"temperature\":15,\"condition\":\"cloudy\"}"
}
```

### Tool/Function Calling

**Tool Definition:**
```json
{
  "type": "function",
  "function": {
    "name": "get_weather",
    "description": "Get the current weather for a location",
    "parameters": {
      "type": "object",
      "properties": {
        "location": {
          "type": "string",
          "description": "The city and state, e.g. San Francisco, CA"
        },
        "unit": {
          "type": "string",
          "enum": ["celsius", "fahrenheit"],
          "description": "The temperature unit"
        }
      },
      "required": ["location"]
    }
  }
}
```

**Tool Choice Options:**
```json
// Auto-decide whether to call tools
"tool_choice": "auto"

// Never call tools
"tool_choice": "none"

// Must call at least one tool
"tool_choice": "required"

// Must call specific tool
"tool_choice": {
  "type": "function",
  "function": {
    "name": "get_weather"
  }
}
```

**Request with Tools:**
```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": "What's the weather in San Francisco?"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get current weather",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {"type": "string"}
          },
          "required": ["location"]
        }
      }
    }
  ],
  "tool_choice": "auto"
}
```

### Non-Streaming Response

**Response Format:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! I'm doing well, thank you for asking. How can I help you today?"
      },
      "finish_reason": "stop",
      "logprobs": null
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30,
    "prompt_tokens_details": {
      "cached_tokens": 0
    }
  }
}
```

**Finish Reasons:**
- `"stop"`: Natural completion
- `"length"`: Max tokens reached
- `"tool_calls"`: Model wants to call tools
- `"content_filter"`: Content filtered

**Response with Tool Calls:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": "{\"location\":\"San Francisco\",\"unit\":\"celsius\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls",
      "logprobs": null
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 25,
    "total_tokens": 40
  }
}
```

### Streaming Response

**Format:** Server-Sent Events (SSE)

**Stream Structure:**
```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk",...}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk",...}

data: [DONE]
```

**Chunk Format:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion.chunk",
  "created": 1677652288,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "delta": {
        "role": "assistant",
        "content": "Hello"
      },
      "finish_reason": null,
      "logprobs": null
    }
  ]
}
```

**Delta Object:**
- **First chunk:** Contains `role`
- **Middle chunks:** Contains `content` (text fragments)
- **Last chunk:** `finish_reason` is set, `delta` may be empty

**Tool Call Streaming:**

Tool calls are streamed incrementally:

```json
// Chunk 1: Tool call starts
{
  "choices": [{
    "delta": {
      "tool_calls": [{
        "index": 0,
        "id": "call_abc123",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": ""
        }
      }]
    }
  }]
}

// Chunk 2: Arguments build up
{
  "choices": [{
    "delta": {
      "tool_calls": [{
        "index": 0,
        "function": {
          "arguments": "{\"loc"
        }
      }]
    }
  }]
}

// Chunk 3: More arguments
{
  "choices": [{
    "delta": {
      "tool_calls": [{
        "index": 0,
        "function": {
          "arguments": "ation\":\""
        }
      }]
    }
  }]
}

// Final chunk
{
  "choices": [{
    "delta": {},
    "finish_reason": "tool_calls"
  }]
}
```

**Usage in Final Chunk:**
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion.chunk",
  "created": 1677652288,
  "model": "gpt-4o",
  "choices": [{
    "index": 0,
    "delta": {},
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}
```

**Parsing Streaming Responses:**

```go
scanner := bufio.NewScanner(resp.Body)
for scanner.Scan() {
    line := scanner.Text()

    if !strings.HasPrefix(line, "data: ") {
        continue
    }

    data := strings.TrimPrefix(line, "data: ")

    if data == "[DONE]" {
        break
    }

    var chunk ChatCompletionChunk
    if err := json.Unmarshal([]byte(data), &chunk); err != nil {
        return err
    }

    // Process chunk
}
```

---

## HTTP Headers

### Required Headers for All Copilot API Requests

```http
Authorization: Bearer {copilot_token}
Content-Type: application/json
copilot-integration-id: vscode-chat
editor-version: vscode/1.104.3
editor-plugin-version: copilot-chat/0.26.7
user-agent: GitHubCopilotChat/0.26.7
openai-intent: conversation-panel
x-github-api-version: 2025-04-01
x-request-id: {uuid_v4}
x-vscode-user-agent-library-version: electron-fetch
```

**Header Descriptions:**

- `Authorization`: Copilot token (from token exchange)
- `Content-Type`: Always "application/json"
- `copilot-integration-id`: Always "vscode-chat"
- `editor-version`: VSCode version (can be hardcoded or fetched)
- `editor-plugin-version`: Copilot plugin version
- `user-agent`: Copilot user agent string
- `openai-intent`: Always "conversation-panel"
- `x-github-api-version`: API version (date format)
- `x-request-id`: Unique UUID for each request
- `x-vscode-user-agent-library-version`: Always "electron-fetch"

### Optional Headers

**Vision Request:**
```http
copilot-vision-request: true
```

Set this header when any message contains images (content type "image_url").

**Initiator:**
```http
X-Initiator: user
```
or
```http
X-Initiator: agent
```

**Logic:**
- `"agent"`: If any message has role "assistant" or "tool"
- `"user"`: Otherwise (all messages are user/system)

### Example Header Generation (Go)

```go
func (p *CopilotProvider) buildHeaders(messages []Message) http.Header {
    headers := http.Header{
        "Authorization":                        []string{"Bearer " + p.copilotToken},
        "Content-Type":                         []string{"application/json"},
        "copilot-integration-id":               []string{"vscode-chat"},
        "editor-version":                       []string{"vscode/1.104.3"},
        "editor-plugin-version":                []string{"copilot-chat/0.26.7"},
        "user-agent":                           []string{"GitHubCopilotChat/0.26.7"},
        "openai-intent":                        []string{"conversation-panel"},
        "x-github-api-version":                 []string{"2025-04-01"},
        "x-request-id":                         []string{uuid.New().String()},
        "x-vscode-user-agent-library-version": []string{"electron-fetch"},
    }

    if hasVisionContent(messages) {
        headers.Set("copilot-vision-request", "true")
    }

    headers.Set("X-Initiator", getInitiator(messages))

    return headers
}

func hasVisionContent(messages []Message) bool {
    for _, msg := range messages {
        if parts, ok := msg.Content.([]ContentPart); ok {
            for _, part := range parts {
                if part.Type == "image_url" {
                    return true
                }
            }
        }
    }
    return false
}

func getInitiator(messages []Message) string {
    for _, msg := range messages {
        if msg.Role == "assistant" || msg.Role == "tool" {
            return "agent"
        }
    }
    return "user"
}
```

---

## Implementation Details

### Max Tokens Auto-Fill

If `max_tokens` is not specified in the request, set it to the model's maximum output tokens.

```go
func (p *CopilotProvider) fillMaxTokens(req *ChatCompletionRequest) error {
    if req.MaxTokens != nil && *req.MaxTokens > 0 {
        return nil // Already set
    }

    model, err := p.getModel(req.Model)
    if err != nil {
        return err
    }

    if model.Capabilities.Limits.MaxOutputTokens > 0 {
        maxTokens := model.Capabilities.Limits.MaxOutputTokens
        req.MaxTokens = &maxTokens
    }

    return nil
}
```

### VSCode Version Detection

**Option 1: Hardcode (Simplest)**
```go
const VSCodeVersion = "1.104.3"
```

**Option 2: Fetch Dynamically (Optional)**
```go
func getVSCodeVersion() string {
    // Fetch from https://code.visualstudio.com/updates
    // Parse latest version
    // Fall back to hardcoded version if fetch fails
    return "1.104.3"
}
```

Hardcoding is recommended for simplicity. Update periodically.

### Account Type Detection

**Method 1: User Configuration**
```go
type CopilotConfig struct {
    AccountType string `json:"account_type"` // "individual", "business", "enterprise"
}
```

**Method 2: Infer from GitHub API** (Future Enhancement)
```
GET https://api.github.com/user/copilot_seats
```

Parse response to determine account type.

### Model Caching

Cache models list to avoid excessive API calls:

```go
type ModelCache struct {
    models    []Model
    fetchedAt time.Time
    ttl       time.Duration
    mu        sync.RWMutex
}

func (c *ModelCache) Get(fetchFunc func() ([]Model, error)) ([]Model, error) {
    c.mu.RLock()
    if time.Since(c.fetchedAt) < c.ttl && len(c.models) > 0 {
        models := c.models
        c.mu.RUnlock()
        return models, nil
    }
    c.mu.RUnlock()

    c.mu.Lock()
    defer c.mu.Unlock()

    // Double-check after acquiring write lock
    if time.Since(c.fetchedAt) < c.ttl && len(c.models) > 0 {
        return c.models, nil
    }

    models, err := fetchFunc()
    if err != nil {
        return nil, err
    }

    c.models = models
    c.fetchedAt = time.Now()
    return models, nil
}
```

**TTL Recommendation:** 1 hour

---

## Error Handling

### GitHub OAuth Errors

**Device Code Request Failed:**
```json
{
  "error": "invalid_request",
  "error_description": "The request is missing a required parameter."
}
```

**Device Code Expired:**
```json
{
  "error": "expired_token",
  "error_description": "The device code has expired."
}
```

**Access Denied:**
```json
{
  "error": "access_denied",
  "error_description": "The user denied the authorization request."
}
```

### Copilot Token Exchange Errors

**No Copilot Subscription (401):**
```json
{
  "message": "Bad credentials",
  "documentation_url": "https://docs.github.com/rest"
}
```

**User Action:** Direct user to subscribe to GitHub Copilot

### Copilot API Errors

**Rate Limited (429):**
```json
{
  "error": {
    "message": "Rate limit exceeded. Please retry after some time.",
    "type": "rate_limit_error"
  }
}
```

**Headers:**
```http
x-ratelimit-limit: 100
x-ratelimit-remaining: 0
x-ratelimit-reset: 1733861400
retry-after: 60
```

**Invalid Request (400):**
```json
{
  "error": {
    "message": "Invalid request: model 'invalid-model' not found",
    "type": "invalid_request_error"
  }
}
```

**Server Error (500):**
```json
{
  "error": {
    "message": "Internal server error",
    "type": "server_error"
  }
}
```

### Error Handling Strategy

```go
func (p *CopilotProvider) handleAPIError(statusCode int, body []byte) error {
    switch statusCode {
    case 400:
        return types.NewInvalidRequestError(types.ProviderTypeCopilot,
            parseErrorMessage(body))
    case 401:
        return types.NewAuthError(types.ProviderTypeCopilot,
            "Copilot token expired or invalid - re-authentication required")
    case 429:
        retryAfter := parseRetryAfter(body)
        return types.NewRateLimitError(types.ProviderTypeCopilot, retryAfter)
    case 500, 502, 503, 504:
        return types.NewServerError(types.ProviderTypeCopilot, statusCode,
            "Copilot API server error - retry later")
    default:
        return types.NewUnknownError(types.ProviderTypeCopilot,
            fmt.Sprintf("HTTP %d: %s", statusCode, string(body)))
    }
}

func parseErrorMessage(body []byte) string {
    var errResp struct {
        Error struct {
            Message string `json:"message"`
        } `json:"error"`
    }
    if err := json.Unmarshal(body, &errResp); err == nil {
        return errResp.Error.Message
    }
    return string(body)
}
```

---

## Rate Limiting

### GitHub's Rate Limits

GitHub Copilot enforces rate limits based on subscription type:
- Requests per minute
- Tokens per day
- Concurrent requests

**Note:** Exact limits are not publicly documented and may vary.

### Rate Limit Headers

```http
x-ratelimit-limit: 100
x-ratelimit-remaining: 95
x-ratelimit-reset: 1733861400
```

**Fields:**
- `x-ratelimit-limit`: Total requests allowed in window
- `x-ratelimit-remaining`: Requests remaining
- `x-ratelimit-reset`: Unix timestamp when limit resets

### Client-Side Rate Limiting (Recommended)

Implement client-side throttling to avoid hitting GitHub's limits:

```go
type RateLimiter struct {
    minInterval time.Duration
    lastRequest time.Time
    mu          sync.Mutex
}

func (r *RateLimiter) Wait(ctx context.Context) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    elapsed := time.Since(r.lastRequest)
    if elapsed < r.minInterval {
        sleepDuration := r.minInterval - elapsed

        select {
        case <-time.After(sleepDuration):
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    r.lastRequest = time.Now()
    return nil
}
```

**Recommended Minimum Interval:** 100ms between requests

### Handling 429 Responses

```go
func (p *CopilotProvider) handleRateLimit(headers http.Header) error {
    retryAfterStr := headers.Get("retry-after")
    if retryAfterStr != "" {
        retryAfter, err := strconv.Atoi(retryAfterStr)
        if err == nil {
            time.Sleep(time.Duration(retryAfter) * time.Second)
            return nil
        }
    }

    resetStr := headers.Get("x-ratelimit-reset")
    if resetStr != "" {
        reset, err := strconv.ParseInt(resetStr, 10, 64)
        if err == nil {
            resetTime := time.Unix(reset, 0)
            sleepDuration := time.Until(resetTime)
            if sleepDuration > 0 && sleepDuration < 5*time.Minute {
                time.Sleep(sleepDuration)
                return nil
            }
        }
    }

    // Default: wait 60 seconds
    time.Sleep(60 * time.Second)
    return nil
}
```

---

## Usage Tracking

### Get Copilot Usage Info

**Endpoint:**
```
GET https://api.github.com/copilot_internal/user
```

**Headers:**
```http
Authorization: token {github_token}
Accept: application/json
editor-version: vscode/1.104.3
editor-plugin-version: copilot-chat/0.26.7
user-agent: GitHubCopilotChat/0.26.7
x-github-api-version: 2025-04-01
```

**Response:**
```json
{
  "copilot_enabled": true,
  "chat_enabled": true,
  "copilot_plan": "copilot_individual",
  "quota_reset_date": "2025-01-01T00:00:00Z",
  "quota_snapshots": {
    "chat": {
      "quota_remaining": 75,
      "entitlement": 100,
      "percent_remaining": 75.0
    },
    "completions": {
      "quota_remaining": 950,
      "entitlement": 1000,
      "percent_remaining": 95.0
    }
  },
  "public_code_suggestions": "allowed",
  "chat_jetbrains_enabled": true,
  "ide_chat_enabled": true,
  "telemetry": "enabled",
  "tracking_id": "12345"
}
```

**Usage:**
- Poll periodically (e.g., every hour) to check quota
- Display quota warnings to users
- Implement soft limits based on remaining quota

---

## Constants Reference

```go
package copilot

const (
    // OAuth
    GitHubClientID           = "Iv1.b507a08c87ecfe98"
    GitHubOAuthScopes        = "read:user"

    // GitHub Endpoints
    GitHubDeviceCodeURL      = "https://github.com/login/device/code"
    GitHubAccessTokenURL     = "https://github.com/login/oauth/access_token"
    GitHubVerificationURL    = "https://github.com/login/device"
    GitHubAPIBaseURL         = "https://api.github.com"
    GitHubCopilotTokenURL    = "https://api.github.com/copilot_internal/v2/token"
    GitHubCopilotUserURL     = "https://api.github.com/copilot_internal/user"

    // Copilot API Endpoints
    CopilotBaseURL           = "https://api.githubcopilot.com"
    CopilotBusinessBaseURL   = "https://api.business.githubcopilot.com"
    CopilotEnterpriseBaseURL = "https://api.enterprise.githubcopilot.com"

    // Version Information
    VSCodeVersion            = "1.104.3"
    CopilotChatVersion       = "0.26.7"

    // Headers
    EditorPluginVersion      = "copilot-chat/0.26.7"
    UserAgent                = "GitHubCopilotChat/0.26.7"
    CopilotIntegrationID     = "vscode-chat"
    OpenAIIntent             = "conversation-panel"
    GitHubAPIVersion         = "2025-04-01"
    VSCodeUserAgentLibrary   = "electron-fetch"

    // Token Management
    TokenRefreshBuffer       = 60 // seconds before expiry to refresh

    // Defaults
    DefaultMaxTokens         = 4096
    DefaultTemperature       = 1.0
    DefaultTopP              = 1.0
    DefaultModel             = "gpt-4o"
)
```

---

## Complete Examples

### Example 1: Full Authentication Flow

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"
    "time"
)

const (
    GitHubClientID        = "Iv1.b507a08c87ecfe98"
    GitHubDeviceCodeURL   = "https://github.com/login/device/code"
    GitHubAccessTokenURL  = "https://github.com/login/oauth/access_token"
    GitHubCopilotTokenURL = "https://api.github.com/copilot_internal/v2/token"
)

type DeviceCodeResponse struct {
    DeviceCode      string `json:"device_code"`
    UserCode        string `json:"user_code"`
    VerificationURI string `json:"verification_uri"`
    ExpiresIn       int    `json:"expires_in"`
    Interval        int    `json:"interval"`
}

type AccessTokenResponse struct {
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
    Scope       string `json:"scope"`
    Error       string `json:"error"`
}

type CopilotTokenResponse struct {
    Token     string `json:"token"`
    ExpiresAt int64  `json:"expires_at"`
    RefreshIn int    `json:"refresh_in"`
}

func main() {
    ctx := context.Background()

    // Step 1: Get device code
    deviceCode, err := getDeviceCode(ctx)
    if err != nil {
        fmt.Printf("Failed to get device code: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("Visit: %s\n", deviceCode.VerificationURI)
    fmt.Printf("Enter code: %s\n", deviceCode.UserCode)
    fmt.Println("Waiting for authorization...")

    // Step 2: Poll for access token
    githubToken, err := pollForAccessToken(ctx, deviceCode)
    if err != nil {
        fmt.Printf("Failed to get access token: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("GitHub token obtained!")

    // Step 3: Exchange for Copilot token
    copilotToken, err := getCopilotToken(ctx, githubToken)
    if err != nil {
        fmt.Printf("Failed to get Copilot token: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("Copilot token obtained! Expires at: %d, Refresh in: %d seconds\n",
        copilotToken.ExpiresAt, copilotToken.RefreshIn)
}

func getDeviceCode(ctx context.Context) (*DeviceCodeResponse, error) {
    reqBody := strings.NewReader(`{"client_id":"` + GitHubClientID + `","scope":"read:user"}`)

    req, err := http.NewRequestWithContext(ctx, "POST", GitHubDeviceCodeURL, reqBody)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var deviceCode DeviceCodeResponse
    if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
        return nil, err
    }

    return &deviceCode, nil
}

func pollForAccessToken(ctx context.Context, deviceCode *DeviceCodeResponse) (string, error) {
    interval := time.Duration(deviceCode.Interval+1) * time.Second
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            token, err := checkAccessToken(ctx, deviceCode.DeviceCode)
            if err != nil {
                continue // Keep polling
            }
            return token, nil
        case <-ctx.Done():
            return "", ctx.Err()
        }
    }
}

func checkAccessToken(ctx context.Context, deviceCode string) (string, error) {
    reqBody := fmt.Sprintf(`{"client_id":"%s","device_code":"%s","grant_type":"urn:ietf:params:oauth:grant-type:device_code"}`,
        GitHubClientID, deviceCode)

    req, err := http.NewRequestWithContext(ctx, "POST", GitHubAccessTokenURL, strings.NewReader(reqBody))
    if err != nil {
        return "", err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var tokenResp AccessTokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return "", err
    }

    if tokenResp.AccessToken != "" {
        return tokenResp.AccessToken, nil
    }

    return "", fmt.Errorf("pending")
}

func getCopilotToken(ctx context.Context, githubToken string) (*CopilotTokenResponse, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", GitHubCopilotTokenURL, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "token "+githubToken)
    req.Header.Set("Accept", "application/json")
    req.Header.Set("editor-version", "vscode/1.104.3")
    req.Header.Set("editor-plugin-version", "copilot-chat/0.26.7")
    req.Header.Set("user-agent", "GitHubCopilotChat/0.26.7")
    req.Header.Set("x-github-api-version", "2025-04-01")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
    }

    var copilotToken CopilotTokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&copilotToken); err != nil {
        return nil, err
    }

    return &copilotToken, nil
}
```

### Example 2: Chat Completion Request

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"

    "github.com/google/uuid"
)

type ChatCompletionRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    MaxTokens   int       `json:"max_tokens,omitempty"`
    Temperature float64   `json:"temperature,omitempty"`
    Stream      bool      `json:"stream,omitempty"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ChatCompletionResponse struct {
    ID      string   `json:"id"`
    Object  string   `json:"object"`
    Created int64    `json:"created"`
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}

type Choice struct {
    Index        int     `json:"index"`
    Message      Message `json:"message"`
    FinishReason string  `json:"finish_reason"`
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

func chatCompletion(ctx context.Context, copilotToken string) error {
    request := ChatCompletionRequest{
        Model: "gpt-4o",
        Messages: []Message{
            {Role: "system", Content: "You are a helpful assistant."},
            {Role: "user", Content: "Write a hello world function in Go."},
        },
        MaxTokens:   2048,
        Temperature: 0.7,
        Stream:      false,
    }

    jsonBody, err := json.Marshal(request)
    if err != nil {
        return err
    }

    url := "https://api.githubcopilot.com/chat/completions"
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
    if err != nil {
        return err
    }

    // Set required headers
    req.Header.Set("Authorization", "Bearer "+copilotToken)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("copilot-integration-id", "vscode-chat")
    req.Header.Set("editor-version", "vscode/1.104.3")
    req.Header.Set("editor-plugin-version", "copilot-chat/0.26.7")
    req.Header.Set("user-agent", "GitHubCopilotChat/0.26.7")
    req.Header.Set("openai-intent", "conversation-panel")
    req.Header.Set("x-github-api-version", "2025-04-01")
    req.Header.Set("x-request-id", uuid.New().String())
    req.Header.Set("x-vscode-user-agent-library-version", "electron-fetch")
    req.Header.Set("X-Initiator", "user")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
    }

    var response ChatCompletionResponse
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return err
    }

    fmt.Printf("Response: %s\n", response.Choices[0].Message.Content)
    fmt.Printf("Usage: %d prompt + %d completion = %d total tokens\n",
        response.Usage.PromptTokens,
        response.Usage.CompletionTokens,
        response.Usage.TotalTokens)

    return nil
}
```

### Example 3: Streaming Chat Completion

```go
package main

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"

    "github.com/google/uuid"
)

type ChatCompletionChunk struct {
    ID      string        `json:"id"`
    Object  string        `json:"object"`
    Created int64         `json:"created"`
    Model   string        `json:"model"`
    Choices []ChunkChoice `json:"choices"`
}

type ChunkChoice struct {
    Index        int         `json:"index"`
    Delta        DeltaMessage `json:"delta"`
    FinishReason *string     `json:"finish_reason"`
}

type DeltaMessage struct {
    Role    string `json:"role,omitempty"`
    Content string `json:"content,omitempty"`
}

func streamingChatCompletion(ctx context.Context, copilotToken string) error {
    request := ChatCompletionRequest{
        Model: "gpt-4o",
        Messages: []Message{
            {Role: "user", Content: "Count from 1 to 5."},
        },
        MaxTokens: 100,
        Stream:    true,
    }

    jsonBody, err := json.Marshal(request)
    if err != nil {
        return err
    }

    url := "https://api.githubcopilot.com/chat/completions"
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
    if err != nil {
        return err
    }

    // Set headers (same as non-streaming)
    req.Header.Set("Authorization", "Bearer "+copilotToken)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("copilot-integration-id", "vscode-chat")
    req.Header.Set("editor-version", "vscode/1.104.3")
    req.Header.Set("editor-plugin-version", "copilot-chat/0.26.7")
    req.Header.Set("user-agent", "GitHubCopilotChat/0.26.7")
    req.Header.Set("openai-intent", "conversation-panel")
    req.Header.Set("x-github-api-version", "2025-04-01")
    req.Header.Set("x-request-id", uuid.New().String())
    req.Header.Set("x-vscode-user-agent-library-version", "electron-fetch")
    req.Header.Set("X-Initiator", "user")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("HTTP %d", resp.StatusCode)
    }

    // Parse SSE stream
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()

        if !strings.HasPrefix(line, "data: ") {
            continue
        }

        data := strings.TrimPrefix(line, "data: ")

        if data == "[DONE]" {
            fmt.Println("\nStream complete!")
            break
        }

        var chunk ChatCompletionChunk
        if err := json.Unmarshal([]byte(data), &chunk); err != nil {
            fmt.Printf("Parse error: %v\n", err)
            continue
        }

        if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
            fmt.Print(chunk.Choices[0].Delta.Content)
        }
    }

    return scanner.Err()
}
```

### Example 4: Tool Calling

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/google/uuid"
)

type Tool struct {
    Type     string       `json:"type"`
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    Parameters  map[string]interface{} `json:"parameters"`
}

type ToolCall struct {
    ID       string               `json:"id"`
    Type     string               `json:"type"`
    Function ToolCallFunction     `json:"function"`
}

type ToolCallFunction struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}

func toolCallingExample(ctx context.Context, copilotToken string) error {
    tools := []Tool{
        {
            Type: "function",
            Function: ToolFunction{
                Name:        "get_weather",
                Description: "Get the current weather for a location",
                Parameters: map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "location": map[string]string{
                            "type":        "string",
                            "description": "The city and state, e.g. San Francisco, CA",
                        },
                        "unit": map[string]interface{}{
                            "type":        "string",
                            "enum":        []string{"celsius", "fahrenheit"},
                            "description": "Temperature unit",
                        },
                    },
                    "required": []string{"location"},
                },
            },
        },
    }

    request := map[string]interface{}{
        "model": "gpt-4o",
        "messages": []map[string]string{
            {"role": "user", "content": "What's the weather in San Francisco?"},
        },
        "tools":       tools,
        "tool_choice": "auto",
        "max_tokens":  1000,
    }

    jsonBody, _ := json.Marshal(request)

    url := "https://api.githubcopilot.com/chat/completions"
    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))

    req.Header.Set("Authorization", "Bearer "+copilotToken)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("copilot-integration-id", "vscode-chat")
    req.Header.Set("editor-version", "vscode/1.104.3")
    req.Header.Set("editor-plugin-version", "copilot-chat/0.26.7")
    req.Header.Set("user-agent", "GitHubCopilotChat/0.26.7")
    req.Header.Set("openai-intent", "conversation-panel")
    req.Header.Set("x-github-api-version", "2025-04-01")
    req.Header.Set("x-request-id", uuid.New().String())
    req.Header.Set("x-vscode-user-agent-library-version", "electron-fetch")
    req.Header.Set("X-Initiator", "user")

    resp, _ := http.DefaultClient.Do(req)
    defer resp.Body.Close()

    var response struct {
        Choices []struct {
            Message struct {
                ToolCalls []ToolCall `json:"tool_calls"`
            } `json:"message"`
        } `json:"choices"`
    }

    json.NewDecoder(resp.Body).Decode(&response)

    if len(response.Choices) > 0 && len(response.Choices[0].Message.ToolCalls) > 0 {
        toolCall := response.Choices[0].Message.ToolCalls[0]
        fmt.Printf("Model wants to call: %s\n", toolCall.Function.Name)
        fmt.Printf("With arguments: %s\n", toolCall.Function.Arguments)
    }

    return nil
}
```

---

## Implementation Checklist

- [ ] GitHub OAuth device flow implementation
- [ ] GitHub token storage (file-based)
- [ ] Copilot token exchange
- [ ] Copilot token refresh loop
- [ ] Chat completions API
- [ ] Streaming support
- [ ] Tool calling support
- [ ] Vision (image) support
- [ ] Models API
- [ ] Embeddings API
- [ ] Rate limiting
- [ ] Error handling
- [ ] Usage tracking
- [ ] Model caching
- [ ] Unit tests
- [ ] Integration tests
- [ ] Documentation

---

## References

- **Source:** copilot-api reverse engineering analysis
- **GitHub OAuth:** https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps
- **Device Flow:** https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow
- **OpenAI API:** https://platform.openai.com/docs/api-reference/chat

---

**End of Technical Specification**
