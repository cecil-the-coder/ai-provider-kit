# OAuth Manager

Enterprise-grade OAuth credential management for AI providers.

## Documentation

Complete documentation is available at:
- **[OAuth Manager Documentation](/docs/OAUTH_MANAGER.md)** - Full feature documentation, examples, and best practices

## Quick Links

- [Metrics Documentation](/docs/METRICS.md) - Provider-level and credential-level metrics
- [Multi-Key Strategies](/docs/MULTI_KEY_STRATEGIES.md) - Comparison of credential management approaches
- [KeyManager Package](/pkg/keymanager/) - Simple API key management

## Quick Start

```go
import "github.com/cecil-the-coder/ai-provider-kit/pkg/oauthmanager"

// Create manager with multiple credentials
manager := oauthmanager.NewOAuthKeyManager("Gemini", credentials, refreshFunc)

// Use with automatic failover and token refresh
result, usage, err := manager.ExecuteWithFailover(ctx,
    func(ctx context.Context, cred *types.OAuthCredentialSet) (string, *types.Usage, error) {
        return makeAPICall(ctx, cred.AccessToken)
    })
```

See the [full documentation](/docs/OAUTH_MANAGER.md) for detailed usage, advanced features, and examples.
