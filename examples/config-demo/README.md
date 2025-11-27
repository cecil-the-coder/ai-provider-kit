# Config Demo

This demonstration program shows how to use the shared config package to parse `config.yaml` files and construct proper configuration structures needed by the ai-provider-kit module.

## What This Example Demonstrates

This educational example shows you how to:

1. **Load and parse YAML configuration files** in the demo-client format
2. **Handle multiple authentication methods**:
   - Single API keys (`api_key` field)
   - Multiple API keys (`api_keys` array for load balancing/failover)
   - OAuth credentials (`oauth_credentials` array with multiple credential sets)
3. **Construct `types.ProviderConfig` structures** correctly for each provider
4. **Handle custom providers** that use OpenAI-compatible APIs
5. **Convert OAuth credential formats** from config file to `types.OAuthCredentialSet`
6. **Display configuration details** for debugging and verification

## Important Notes

**This example does NOT:**
- Create actual provider instances
- Make API calls
- Validate credentials
- Connect to provider services

**This example ONLY:**
- Parses configuration files
- Shows how to construct the correct config structures
- Demonstrates the pattern for your own applications

## Config File Format

The configuration file follows this structure:

```yaml
providers:
  enabled:
    - provider1
    - provider2

  preferred_order:
    - provider1
    - provider2

  # Built-in providers
  anthropic:
    type: ""
    default_model: claude-sonnet-4-5
    base_url: ""
    api_key: ""                    # Single API key
    api_keys: []                   # OR multiple API keys
    oauth_credentials: []          # OR OAuth credentials
    max_tokens: 8192
    temperature: 0.7

  # Custom providers
  custom:
    groq:
      type: openai                 # Which provider API to use
      base_url: https://api.groq.com/openai/v1
      api_key: your_key
      default_model: llama-3.3-70b-versatile
```

### Authentication Methods

#### 1. Single API Key
```yaml
provider_name:
  api_key: sk-your-key-here
  api_keys: []
  oauth_credentials: []
```

#### 2. Multiple API Keys (Load Balancing)
```yaml
provider_name:
  api_key: ""
  api_keys:
    - key1
    - key2
    - key3
  oauth_credentials: []
```

#### 3. OAuth Credentials
```yaml
provider_name:
  api_key: ""
  api_keys: []
  oauth_credentials:
    - id: default
      client_id: your_client_id
      client_secret: your_client_secret
      access_token: your_access_token
      refresh_token: your_refresh_token
      expires_at: "2025-12-31T23:59:59-07:00"
      scopes:
        - scope1
        - scope2
```

#### 4. Multiple OAuth Credentials (Failover)
```yaml
provider_name:
  oauth_credentials:
    - id: primary
      client_id: client_id_1
      access_token: token_1
      # ... more fields
    - id: backup
      client_id: client_id_2
      access_token: token_2
      # ... more fields
```

## How to Run

### 1. Build the example
```bash
cd examples/config-demo
go build -o config-demo .
```

### 2. Run with default config (config.yaml in current directory)
```bash
./config-demo
```

### 3. Run with a specific config file
```bash
./config-demo -config config.yaml.example
```

### 4. Run with demo-client config
```bash
./config-demo -config ../demo-client/config.yaml
```

### Command Line Options

```
-config string
    Path to config file (default "config.yaml" in current directory)
```

By default, the demo looks for `config.yaml` in the current working directory. This allows you to run the demo from any directory by simply placing your config file there.


## Example Output

```
=======================================================================
AI Provider Kit - Config Demo
=======================================================================

Loading configuration from: config.yaml.example
Configuration loaded successfully!

Configuration Summary:
-----------------------------------------------------------------------
  Enabled Providers: 7
    - openai
    - anthropic
    - gemini
    - cerebras
    - qwen
    - groq
    - synthetic

=======================================================================
Processing Enabled Providers
=======================================================================

[1/7] Processing: openai
-----------------------------------------------------------------------
  Provider Name: openai
  Provider Type: openai
  Authentication: Single API Key
  API Key: sk-p...HERE
  Default Model: gpt-4o
  Max Tokens: 4096

  Constructed types.ProviderConfig:
    Type: openai
    Name: openai
    APIKey: sk-p...HERE
    DefaultModel: gpt-4o
    MaxTokens: 4096

[2/7] Processing: anthropic
-----------------------------------------------------------------------
  Provider Name: anthropic
  Provider Type: anthropic
  Authentication: OAuth (1 credential sets)
  API Key: sk-a...HERE
  OAuth Credentials:
    [1] ID: default
        Client ID: your...t_id
        Access Token: sk-a...HERE
        Refresh Token: sk-a...HERE
        Expires: 2025-12-31T23:59:59-07:00
  Default Model: claude-sonnet-4-5
  Max Tokens: 8192

  Constructed types.ProviderConfig:
    Type: anthropic
    Name: anthropic
    APIKey: sk-a...HERE
    DefaultModel: claude-sonnet-4-5
    MaxTokens: 8192
    OAuthCredentials: 1 sets

[3/7] Processing: cerebras
-----------------------------------------------------------------------
  Provider Name: cerebras
  Provider Type: cerebras
  Authentication: Multiple API Keys (3 keys)
  API Key: csk-...E_KEY
  Additional API Keys:
    [2] csk-...E_KEY
    [3] csk-...E_KEY
  Default Model: llama3.1-8b
  Max Tokens: 131072

  Constructed types.ProviderConfig:
    Type: cerebras
    Name: cerebras
    APIKey: csk-...E_KEY
    DefaultModel: llama3.1-8b
    MaxTokens: 131072
```

## Understanding the Code

### Key Components

#### 1. Configuration Structures

The example defines Go structures that match the YAML config format:

```go
type DemoConfig struct {
    Providers ProvidersConfig `yaml:"providers"`
    // ...
}

type ProviderConfigEntry struct {
    Type             string                   `yaml:"type"`
    DefaultModel     string                   `yaml:"default_model"`
    BaseURL          string                   `yaml:"base_url"`
    APIKey           string                   `yaml:"api_key"`
    APIKeys          []string                 `yaml:"api_keys"`
    OAuthCredentials []OAuthCredentialEntry   `yaml:"oauth_credentials"`
    // ...
}
```

#### 2. Loading Configuration

```go
func loadConfig(filename string) (*DemoConfig, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }

    var config DemoConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse YAML: %w", err)
    }

    return &config, nil
}
```

#### 3. Building ProviderConfig

The core function that converts config entries to `types.ProviderConfig`:

```go
func buildProviderConfig(name string, entry *ProviderConfigEntry) types.ProviderConfig {
    config := types.ProviderConfig{
        Name: name,
        Type: determineProviderType(name, entry),
    }

    // Priority: api_key > api_keys[0] > oauth_credentials[0].access_token
    if entry.APIKey != "" {
        config.APIKey = entry.APIKey
    } else if len(entry.APIKeys) > 0 {
        config.APIKey = entry.APIKeys[0]
    } else if len(entry.OAuthCredentials) > 0 {
        config.APIKey = entry.OAuthCredentials[0].AccessToken
    }

    // Set optional fields
    if entry.BaseURL != "" {
        config.BaseURL = entry.BaseURL
    }
    if entry.DefaultModel != "" {
        config.DefaultModel = entry.DefaultModel
    }
    if entry.MaxTokens > 0 {
        config.MaxTokens = entry.MaxTokens
    }

    // Convert OAuth credentials
    if len(entry.OAuthCredentials) > 0 {
        config.OAuthCredentials = convertOAuthCredentials(entry.OAuthCredentials)
    }

    return config
}
```

#### 4. OAuth Conversion

Converting from config format to `types.OAuthCredentialSet`:

```go
func convertOAuthCredentials(entries []OAuthCredentialEntry) []*types.OAuthCredentialSet {
    credSets := make([]*types.OAuthCredentialSet, 0, len(entries))

    for _, entry := range entries {
        var expiresAt time.Time
        if entry.ExpiresAt != "" {
            if t, err := time.Parse(time.RFC3339, entry.ExpiresAt); err == nil {
                expiresAt = t
            }
        }

        credSet := &types.OAuthCredentialSet{
            ID:           entry.ID,
            ClientID:     entry.ClientID,
            ClientSecret: entry.ClientSecret,
            AccessToken:  entry.AccessToken,
            RefreshToken: entry.RefreshToken,
            ExpiresAt:    expiresAt,
            Scopes:       entry.Scopes,
        }

        credSets = append(credSets, credSet)
    }

    return credSets
}
```

## Integrating Into Your Application

To use this pattern in your own application:

### 1. Copy the config structures
```go
// Copy DemoConfig, ProvidersConfig, ProviderConfigEntry, etc.
```

### 2. Load your config file
```go
config, err := loadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}
```

### 3. Build ProviderConfig for each enabled provider
```go
for _, providerName := range config.Providers.Enabled {
    entry := getProviderEntry(config, providerName)
    if entry == nil {
        continue
    }

    providerConfig := buildProviderConfig(providerName, entry)

    // Now use providerConfig to create the actual provider
    provider, err := factory.CreateProvider(providerConfig.Type, providerConfig)
    if err != nil {
        log.Printf("Failed to create provider %s: %v", providerName, err)
        continue
    }

    // Use the provider...
}
```

### 4. Handle OAuth token refresh
```go
// Set up token refresh callback
for i := range providerConfig.OAuthCredentials {
    providerConfig.OAuthCredentials[i].OnTokenRefresh = func(
        id, accessToken, refreshToken string,
        expiresAt time.Time,
    ) error {
        // Save the new tokens to your config file or database
        return saveTokens(id, accessToken, refreshToken, expiresAt)
    }
}
```

## Configuration Best Practices

### 1. Security
- **Never commit actual credentials** to version control
- Use environment variables or secret management for sensitive data
- Consider encrypting the config file
- Use appropriate file permissions (e.g., 0600)

### 2. Multiple API Keys
When using multiple API keys:
- Implement load balancing by rotating through keys
- Implement failover by trying next key on error
- Track usage per key for rate limiting

### 3. OAuth Credentials
When using OAuth:
- Always implement token refresh callbacks
- Persist refreshed tokens immediately
- Handle token expiration gracefully
- Consider using multiple credential sets for high availability

### 4. Custom Providers
For OpenAI-compatible APIs:
- Set `type: openai` in the config
- Specify the correct `base_url`
- Some may require specific headers or formats

## Error Handling

In production code, handle these scenarios:

```go
// Missing provider config
entry := getProviderEntry(config, providerName)
if entry == nil {
    return fmt.Errorf("no config for provider: %s", providerName)
}

// Missing credentials
if entry.APIKey == "" && len(entry.APIKeys) == 0 && len(entry.OAuthCredentials) == 0 {
    return fmt.Errorf("no credentials for provider: %s", providerName)
}

// Invalid OAuth expiration
expiresAt, err := time.Parse(time.RFC3339, entry.OAuthCredentials[0].ExpiresAt)
if err != nil {
    return fmt.Errorf("invalid expires_at format: %w", err)
}

// Expired OAuth token
if time.Now().After(expiresAt) {
    // Trigger refresh or return error
}
```

## Advanced Usage

### Dynamic Provider Selection

```go
// Use preferred_order for provider selection
for _, providerName := range config.Providers.PreferredOrder {
    if provider, err := createProvider(providerName, config); err == nil {
        // Use this provider
        break
    }
}
```

### Load Balancing API Keys

```go
type KeyRotator struct {
    keys    []string
    current int
    mu      sync.Mutex
}

func (kr *KeyRotator) Next() string {
    kr.mu.Lock()
    defer kr.mu.Unlock()

    key := kr.keys[kr.current]
    kr.current = (kr.current + 1) % len(kr.keys)
    return key
}

// Use it:
if len(entry.APIKeys) > 0 {
    rotator := &KeyRotator{keys: entry.APIKeys}
    providerConfig.APIKey = rotator.Next()
}
```

### OAuth Failover

```go
func createProviderWithFailover(config types.ProviderConfig) (types.Provider, error) {
    for i, cred := range config.OAuthCredentials {
        // Try each credential set until one works
        tempConfig := config
        tempConfig.APIKey = cred.AccessToken

        provider, err := factory.CreateProvider(config.Type, tempConfig)
        if err == nil {
            if err := provider.HealthCheck(context.Background()); err == nil {
                log.Printf("Using OAuth credential set %d: %s", i, cred.ID)
                return provider, nil
            }
        }
    }
    return nil, fmt.Errorf("all OAuth credentials failed")
}
```

## Related Examples

- **demo-client**: Full client implementation using this config format
- **model-discovery-demo**: Demonstrates dynamic model discovery
- **demo-client-streaming**: Shows streaming with provider configuration

## Additional Resources

- [ai-provider-kit Documentation](../../README.md)
- [Provider Types Reference](../../pkg/types/provider.go)
- [Factory Documentation](../../pkg/factory/factory.go)
- [OAuth Manager](../../pkg/oauthmanager/)

## Troubleshooting

### Config file not found
```
Error: failed to read config file: open config.yaml: no such file or directory
```
**Solution**: Provide the correct path to your config file.

### Invalid YAML syntax
```
Error: failed to parse YAML: yaml: line 42: ...
```
**Solution**: Validate your YAML syntax. Common issues:
- Incorrect indentation (use spaces, not tabs)
- Missing colons
- Unquoted special characters

### Missing provider configuration
```
WARNING: No configuration found for provider 'xyz'
```
**Solution**: Add the provider to your config file under `providers:` or `providers.custom:`.

### Empty credentials
```
Authentication: None
```
**Solution**: Ensure you've set either `api_key`, `api_keys`, or `oauth_credentials`.

## License

This example is part of the ai-provider-kit project. See the main LICENSE file for details.
