# Dynamic Model Discovery Demo

This demo showcases the dynamic model discovery feature implemented across all AI Provider Kit providers.

## Features Demonstrated

1. **Individual Provider Discovery** - Fetches and displays models from each configured provider
2. **Cache Behavior** - Shows how caching improves performance (typically 100x+ faster on second call)
3. **Cross-Provider Comparison** - Compares model counts and capabilities across providers
4. **JSON Export** - Exports all discovered models to JSON files for inspection

## Prerequisites

You can configure the demo using either:

### Option 1: Configuration File (Recommended)

Create a `config.yaml` file from the example:

```bash
cp config.yaml.example config.yaml
```

Edit `config.yaml` and add your API keys:

```yaml
providers:
  enabled:
    - anthropic
    - openai
    - gemini

  anthropic:
    api_key: "your-anthropic-key"

  openai:
    api_key: "your-openai-key"

  gemini:
    oauth_credentials:
      - id: default
        access_token: "your-access-token"
        refresh_token: "your-refresh-token"
```

### Option 2: Environment Variables

Set environment variables for the providers you want to test:

```bash
export OPENAI_API_KEY="your-openai-key"
export ANTHROPIC_API_KEY="your-anthropic-key"
export GEMINI_API_KEY="your-gemini-key"
export CEREBRAS_API_KEY="your-cerebras-key"
export OPENROUTER_API_KEY="your-openrouter-key"
export QWEN_API_KEY="your-qwen-key"
```

You need at least one API key to run the demo.

## Running the Demo

### With Configuration File

```bash
cd examples/model-discovery-demo
go run main.go -config config.yaml
```

### With Environment Variables

```bash
cd examples/model-discovery-demo
go run main.go
```

### Using go install

```bash
go install github.com/cecil-the-coder/ai-provider-kit/examples/model-discovery-demo@latest
model-discovery-demo -config config.yaml
```

## Configuration File Format

The config file supports multiple authentication methods:

### API Key (Simple)

```yaml
providers:
  enabled:
    - openai
  openai:
    api_key: "sk-..."
```

### Multiple API Keys

```yaml
providers:
  enabled:
    - cerebras
  cerebras:
    api_keys:
      - "csk-key1..."
      - "csk-key2..."
```

The demo uses the first API key from the array.

### OAuth Credentials

```yaml
providers:
  enabled:
    - gemini
  gemini:
    oauth_credentials:
      - id: default
        client_id: "your-client-id"
        client_secret: "your-client-secret"
        access_token: "ya29..."
        refresh_token: "1//06..."
        expires_at: "2025-11-16T18:38:22-07:00"
        scopes:
          - https://www.googleapis.com/auth/cloud-platform
```

### All Available Fields

```yaml
providers:
  enabled:
    - provider-name

  provider-name:
    type: ""                    # Optional: provider type override
    default_model: ""           # Optional: not used in discovery demo
    base_url: ""                # Optional: custom API endpoint
    api_key: ""                 # Option 1: single API key
    api_keys: []                # Option 2: array of API keys
    project_id: ""              # Optional: for Gemini
    max_tokens: 0               # Optional: not used in discovery demo
    temperature: 0              # Optional: not used in discovery demo
    oauth_credentials: []       # Option 3: OAuth authentication
```

## Expected Output

The demo will:

1. **Check for API keys** and report which providers are available
2. **Fetch models** from each provider and display:
   - Model ID
   - Display name
   - Maximum tokens supported
   - Streaming capability (✓/✗)
   - Tool calling support (✓/✗)

3. **Demonstrate caching** by making two consecutive calls:
   - First call: Fetches from API (slower, e.g., 500ms)
   - Second call: Returns from cache (faster, e.g., <1ms)
   - Shows speedup factor

4. **Compare providers** with statistics:
   - Total model count
   - Average max tokens
   - Percentage with streaming support
   - Percentage with tool calling support

5. **Export to JSON** - Creates `model-discovery-output/` directory with:
   - `openai-models.json`
   - `anthropic-models.json`
   - `gemini-models.json`
   - `cerebras-models.json`
   - `openrouter-models.json`
   - `qwen-models.json`

## Example Output

```
🤖 AI Provider Kit - Dynamic Model Discovery Demo
================================================================================

✓ Found API keys for providers:
  - Anthropic
  - OpenAI
  - OpenRouter

1️⃣  Individual Provider Model Discovery
--------------------------------------------------------------------------------

Anthropic Provider:
  ✓ Fetched 8 models in 347ms
  ID                                  Name                      Max Tokens  Streaming  Tools
  ----------------------------------------------------------------------
  claude-3-5-sonnet-20241022          Claude 3.5 Sonnet         200000      ✓          ✓
  claude-3-5-haiku-20241022           Claude 3.5 Haiku          200000      ✓          ✓
  claude-3-opus-20240229              Claude 3 Opus             200000      ✓          ✓
  ... and 5 more models

2️⃣  Cache Behavior Demonstration
--------------------------------------------------------------------------------
Testing with Anthropic provider...

First call (should fetch from API):
  ⏱️  Time: 356ms
  📊 Models: 8

Second call (should use cache):
  ⏱️  Time: 124µs
  📊 Models: 8

💡 Cache speedup: 2871x faster
✓ Cache is working correctly (< 10ms)

3️⃣  Cross-Provider Model Comparison
--------------------------------------------------------------------------------
Provider     Models  Avg Max Tokens  Streaming  Tool Calling
----------------------------------------------------------------------
Anthropic    8       200000          100%       100%
OpenAI       15      42666           100%       93%
OpenRouter   147     438528          100%       100%

🏆 OpenRouter has the most models (147)

4️⃣  Export Models to JSON
--------------------------------------------------------------------------------
✓ Exported 8 models to model-discovery-output/anthropic-models.json
✓ Exported 15 models to model-discovery-output/openai-models.json
✓ Exported 147 models to model-discovery-output/openrouter-models.json

💾 All model data saved to ./model-discovery-output/

✅ Demo completed successfully!
```

## Understanding the Output

### Model Table Columns

- **ID**: Unique model identifier used in API calls
- **Name**: Human-readable model name
- **Max Tokens**: Maximum context window size
  - `-` means not available/unknown
  - `4K` = 4,000 tokens
  - `128K` = 128,000 tokens
  - `2M` = 2,000,000 tokens
- **Streaming**: Whether the model supports real-time streaming
- **Tools**: Whether the model supports function/tool calling

### Cache Performance

The cache speedup demonstrates the efficiency of the caching system:
- **First call**: Makes actual HTTP request to provider API
- **Second call**: Returns instantly from in-memory cache
- **Typical speedup**: 100x - 5000x faster

Cache TTLs (time-to-live):
- OpenAI: 24 hours
- Anthropic: 6 hours
- Gemini: 2 hours
- Cerebras: 6 hours
- OpenRouter: 6 hours
- Qwen: N/A (static list)

### Provider Comparison

The comparison helps you understand:
- Which provider has the most model options
- Average capabilities across providers
- Adoption of features like streaming and tool calling

## Inspecting JSON Output

The exported JSON files contain detailed model information:

```json
[
  {
    "id": "claude-3-5-sonnet-20241022",
    "name": "Claude 3.5 Sonnet",
    "provider": "anthropic",
    "description": "Most capable Sonnet model",
    "max_tokens": 200000,
    "supports_streaming": true,
    "supports_tool_calling": true,
    "capabilities": ["chat", "analysis", "coding"]
  }
]
```

You can use these files to:
- Build model selection UIs
- Analyze model capabilities programmatically
- Track changes in provider offerings over time
- Generate documentation

## Troubleshooting

### No Providers Configured

```
❌ No providers configured. Please either:
   1. Use -config flag with a config file, or
   2. Set environment variables:
      - OPENAI_API_KEY
      - ANTHROPIC_API_KEY
      ...
```

**Solutions**:
- Create a `config.yaml` file and run with `-config config.yaml`
- Or export at least one API key as environment variable

### Failed to Fetch Models

```
❌ Failed to get models: authentication failed
```

**Solutions**:
- Verify your API key is correct
- Check your internet connection
- Ensure the API key has necessary permissions
- Check if the provider's API is experiencing issues

### Cache Not Working

If the second call doesn't show significant speedup:
- This could indicate the cache TTL is very short
- Or there was an issue with the first fetch (check for errors)
- The demo shows a warning if cache time > 10ms

## Next Steps

After running this demo, you might want to:

1. **Implement model selection** in your application
2. **Track model updates** by running periodically
3. **Build cost estimation** using OpenRouter pricing data
4. **Create model compatibility matrices** for your use cases

## Related Documentation

- [FEATURE_ROADMAP.md](../../FEATURE_ROADMAP.md) - Full feature roadmap
- [Dynamic Model Discovery Research](../../FEATURE_ROADMAP.md#research-findings-dynamic-model-discovery) - Technical details
- [Provider Documentation](../../docs/README.md) - Individual provider guides
