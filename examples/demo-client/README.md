# AI Provider Kit - Demo Client

A comprehensive demonstration client for the AI Provider Kit that showcases how to properly integrate and use multiple AI providers with OAuth token management.

## Features

- ✅ Tests multiple AI providers (Anthropic, Gemini, Qwen, Cerebras, etc.)
- 🔄 Automatic OAuth token refresh with config file updates
- 📊 Health checks and metrics reporting
- 🌊 Streaming support testing
- 💾 Persistent token storage with automatic updates
- 🎮 Interactive mode for manual testing
- 🔒 Secure token handling and storage

## Setup

### 1. Configuration

Copy your `config.yaml` to this directory or create one based on `config.yaml.example`:

```bash
cp config.yaml.example config.yaml
```

Edit the config file and add your API keys and OAuth credentials:

```yaml
providers:
  anthropic:
    model: claude-3-opus-20240229
    oauth:
      access_token: "your-access-token"
      refresh_token: "your-refresh-token"
      expires_at: "2025-11-15T13:11:35-07:00"

  gemini:
    model: gemini-2.5-pro
    client_id: "your-client-id"
    client_secret: "your-client-secret"
    project_id: "your-project-id"
    oauth:
      access_token: "your-access-token"
      refresh_token: "your-refresh-token"
      expires_at: "2025-11-15T10:58:36-07:00"

  # Add more providers as needed
```

### 2. Dependencies

Install the required dependencies:

```bash
go mod tidy
```

## Usage

### Basic Usage

Test all enabled providers:

```bash
go run .
```

### Command Line Options

```bash
# Test a specific provider
go run . -provider gemini

# Use a custom prompt
go run . -prompt "Write a haiku about Go programming"

# Enable verbose output
go run . -verbose

# Test streaming capabilities
go run . -stream

# Use a different config file
go run . -config /path/to/config.yaml
```

### Interactive Mode

The demo client includes an interactive mode for manual testing:

```bash
go run .
# When prompted, enter 'y' for interactive mode

> list                    # Show available providers
> use gemini             # Select a provider
> prompt Tell me a joke  # Send a prompt
> stream Count to 10     # Test streaming
> health                 # Check provider health
> metrics               # Show provider metrics
> quit                  # Exit
```

## OAuth Token Refresh

The demo client automatically handles OAuth token refresh for providers that support it (Gemini, Qwen, Anthropic with OAuth).

### How it works:

1. **Token Expiry Detection**: Before each API call, the provider checks if the token is expired or near expiry
2. **Automatic Refresh**: If expired, the provider automatically refreshes the token using the refresh token
3. **Callback Notification**: The provider calls the registered callback with new tokens (when library supports it)
4. **Config Update**: The demo client updates the config.yaml file with new tokens
5. **Persistence**: New tokens are saved to disk for use across restarts

### Token Refresh Callback Implementation

```go
func CreateTokenRefreshCallback(providerName string) func(string, string, time.Time) error {
    return func(accessToken, refreshToken string, expiresAt time.Time) error {
        // 1. Log the update
        fmt.Printf("🔄 Token refreshed for %s\n", providerName)

        // 2. Update in-memory config
        updateConfigInMemory(providerName, accessToken, refreshToken, expiresAt)

        // 3. Persist to config.yaml
        saveConfigToFile()

        return nil
    }
}
```

## Provider-Specific Notes

### Anthropic
- Supports OAuth with refresh tokens
- Multiple API keys for failover (if using API key mode)
- Automatic retry with exponential backoff

### Gemini
- OAuth authentication with Google Cloud
- Requires project ID for Cloud API access
- Automatic token refresh before expiry
- Supports both OAuth and API key authentication

### Qwen
- OAuth authentication
- Token refresh on 401 errors
- Supports both OAuth and API key modes

### Cerebras
- API key authentication only
- High token limits (up to 131072)
- Optimized for fast inference

### Custom Providers (Groq, Synthetic, etc.)
- OpenAI-compatible API
- Configure with base URL and API key
- Specify available models in config

## Testing Scenarios

The demo client tests several aspects of each provider:

1. **Authentication**: Verifies provider is properly authenticated
2. **Health Check**: Ensures provider is reachable and healthy
3. **Model Listing**: Fetches available models (if supported)
4. **Chat Completion**: Sends a test prompt and receives response
5. **Streaming**: Tests real-time token streaming (if supported)
6. **Metrics**: Reports request counts, latency, and error rates

## Token Storage Security

The demo client implements secure token storage:

- Tokens are stored in `config.yaml` with file permissions 0644
- OAuth tokens are automatically refreshed before expiry
- Refresh tokens are never logged in full (only first/last characters)
- Token updates are atomic to prevent corruption

## Troubleshooting

### Common Issues

1. **"Provider not authenticated"**
   - Check your API keys or OAuth tokens in config.yaml
   - Ensure tokens haven't expired
   - Verify client ID and secret for OAuth providers

2. **"Failed to refresh token"**
   - Check internet connectivity
   - Verify refresh token is still valid
   - Check OAuth app permissions haven't been revoked

3. **"Config file not found"**
   - Ensure config.yaml exists in the demo-client directory
   - Use `-config` flag to specify alternate location

4. **"Provider not supported"**
   - The provider might not be implemented in the library
   - Try using it as an OpenAI-compatible provider

### Debug Mode

Enable verbose output for detailed debugging:

```bash
go run . -verbose
```

This will show:
- Detailed provider information
- All available models
- Token refresh events
- Full error messages
- Request/response timings

## Architecture

```
demo-client/
├── main.go           # Main application and testing logic
├── config.go         # Configuration management and token persistence
├── config.yaml       # Your provider configurations
├── .tokens.json      # Separate OAuth token storage (optional)
└── README.md         # This file
```

### Key Components:

1. **ConfigManager**: Loads and manages configuration from YAML
2. **TokenManager**: Handles OAuth token updates and persistence
3. **ProviderFactory**: Creates provider instances from configuration
4. **TestProvider**: Runs comprehensive tests on each provider
5. **Interactive Mode**: Manual testing interface

## Best Practices

1. **Never commit credentials**: Add `config.yaml` to `.gitignore`
2. **Use environment variables**: Reference with `${VAR_NAME}` in config
3. **Monitor token expiry**: Check logs for refresh events
4. **Handle failures gracefully**: Implement retry logic for production
5. **Secure token storage**: Use appropriate file permissions
6. **Regular health checks**: Monitor provider availability

## Production Considerations

This demo client is for testing and demonstration. For production use:

1. **Token Storage**: Use a secure secret manager (Vault, AWS Secrets Manager)
2. **Error Handling**: Implement comprehensive retry and fallback logic
3. **Monitoring**: Add metrics and alerting for token refresh failures
4. **Rate Limiting**: Implement rate limiting to avoid API quota issues
5. **Logging**: Use structured logging for better observability
6. **Concurrency**: Handle concurrent requests and token refresh properly
7. **Encryption**: Encrypt tokens at rest and in transit

## Contributing

To add support for a new provider:

1. Add provider configuration to `config.yaml.example`
2. Map provider name to `types.ProviderType` in main.go
3. Handle provider-specific configuration in the factory
4. Test OAuth token refresh if applicable
5. Update this README with provider-specific notes

## License

This demo client is part of the AI Provider Kit project. See the main project LICENSE file for details.