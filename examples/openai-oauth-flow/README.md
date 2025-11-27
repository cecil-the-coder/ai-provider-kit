# OpenAI API Key Authentication Example

This example demonstrates how to configure OpenAI API key authentication for the AI Provider Kit.

**IMPORTANT: OpenAI uses API key authentication, NOT OAuth!**

Unlike providers like Qwen that support OAuth 2.0, OpenAI uses a simpler API key-based authentication system. This is the standard authentication method for OpenAI's API.

## What This Example Does

This program helps you set up OpenAI API key authentication:

1. **Prompts for your API key** with helpful instructions
2. **Validates the API key** by making a test API call
3. **Saves the key** to `~/.mcp-code-api/config.yaml`
4. **Provides security reminders** and next steps

## Why Not OAuth?

OpenAI has chosen to use API key authentication for their API instead of OAuth 2.0:

- **Simpler**: No complex authorization flows needed
- **Direct**: API keys work immediately without browser interactions
- **Suitable for server-to-server**: Perfect for backend applications
- **Easy to rotate**: Keys can be regenerated in the dashboard

OAuth is typically used when you need to access resources on behalf of users. For OpenAI's API, you're accessing resources on behalf of your own account, so API keys are more appropriate.

## Prerequisites

- Go 1.21 or later
- An OpenAI account with API access
- Active credits in your OpenAI account

## Getting Your API Key

Before running this tool, get your OpenAI API key:

1. Go to [https://platform.openai.com/api-keys](https://platform.openai.com/api-keys)
2. Sign in to your OpenAI account
3. Click **"Create new secret key"**
4. Give your key a name (e.g., "ai-provider-kit")
5. Copy the key immediately (you won't be able to see it again!)
6. The key should start with `sk-`

## Installation

```bash
cd examples/openai-oauth-flow
go mod download
```

## Usage

Simply run the program and follow the prompts:

```bash
go run main.go
```

Or build and run:

```bash
go build -o openai-auth
./openai-auth
```

## What to Expect

When you run the program, you'll see output like this:

```
=======================================================================
OpenAI API Key Authentication
=======================================================================

IMPORTANT: OpenAI uses API key authentication, NOT OAuth!

Unlike some AI providers, OpenAI does not support OAuth 2.0 for API
access. Instead, you need to use an API key from your OpenAI account.

This tool will help you:
  1. Validate your OpenAI API key
  2. Save it to the config file: ~/.mcp-code-api/config.yaml

=======================================================================

Step 1: Enter your OpenAI API Key

To get an API key:
  1. Go to https://platform.openai.com/api-keys
  2. Sign in to your OpenAI account
  3. Click 'Create new secret key'
  4. Copy the key (it starts with 'sk-')

Enter your OpenAI API key: sk-proj-...

Step 2: Validating API key...
  Found 58 available models (8 chat models)
✓ API key is valid!

Step 3: Saving API key to config...
✓ API key saved to ~/.mcp-code-api/config.yaml

=======================================================================
Configuration Complete
=======================================================================

  API Key: sk-proj-ab... (first 10 chars)

You can now use the OpenAI provider with API key authentication!

Next steps:
  - Try the demo-client example to test your configuration
  - Visit https://platform.openai.com/usage to monitor your usage
  - Visit https://platform.openai.com/api-keys to manage your keys

Security reminder:
  - Never commit your API key to version control
  - Never share your API key publicly
  - Rotate your keys periodically
  - Set usage limits in your OpenAI dashboard
```

## Config File Structure

After successful authentication, your `~/.mcp-code-api/config.yaml` will be updated with:

```yaml
providers:
  openai:
    api_key: sk-proj-your_actual_api_key_here
    default_model: gpt-4o
```

## How to Verify It Worked

After running the authentication flow, you can verify it worked by:

1. **Check the config file**:
   ```bash
   cat ~/.mcp-code-api/config.yaml
   ```

2. **Use the demo client**:
   ```bash
   cd ../demo-client
   go run main.go
   ```
   The demo client will automatically use your API key.

3. **Test with curl**:
   ```bash
   curl -H "Authorization: Bearer YOUR_API_KEY" \
        https://api.openai.com/v1/models
   ```

## Available Models

After authentication, you'll have access to OpenAI's models including:

- **GPT-4o**: Latest flagship model with high intelligence
- **GPT-4o Mini**: Efficient and affordable small model
- **GPT-4 Turbo**: Balanced GPT-4 model
- **GPT-4**: Previous flagship model
- **GPT-3.5 Turbo**: Fast and capable model

Run the validation step to see which models are available to your account.

## Troubleshooting

### API Key Validation Fails

If the API key validation fails:

1. **Check the key format**: OpenAI API keys start with `sk-`
2. **Verify credits**: Ensure your account has active credits at [https://platform.openai.com/usage](https://platform.openai.com/usage)
3. **Check key status**: Make sure the key hasn't been revoked at [https://platform.openai.com/api-keys](https://platform.openai.com/api-keys)
4. **Network issues**: Verify you can reach api.openai.com
5. **Key permissions**: Some keys may have restricted permissions

The tool will offer to save the key anyway if validation fails, which can be useful if you're behind a firewall or have other network restrictions.

### Invalid API Key Error (401)

This means the key is not recognized by OpenAI:
- The key may be incorrect (typo when copying)
- The key may have been revoked
- You may be using an old or invalid key format

Generate a new key and try again.

### Access Forbidden (403)

This means the key is valid but doesn't have permission:
- Your account may need verification
- The key may have restricted permissions
- Your account may be suspended

Check your OpenAI account status.

### Rate Limited (429)

You're making too many requests:
- Wait a few seconds and try again
- Check your rate limits in the OpenAI dashboard
- Consider upgrading your plan for higher limits

### Config File Permission Denied

If you can't write to the config file:

```bash
# Create the directory
mkdir -p ~/.mcp-code-api

# Set proper permissions
chmod 700 ~/.mcp-code-api
```

## Security Best Practices

### API Key Storage

- **File permissions**: Keys are stored with 0600 permissions (owner read/write only)
- **Location**: `~/.mcp-code-api/config.yaml` in your home directory
- **Never commit**: Add `config.yaml` to `.gitignore`
- **Environment variables**: Consider using environment variables for additional security

### Key Management

1. **Rotate regularly**: Generate new keys every few months
2. **Use separate keys**: Different keys for development, testing, and production
3. **Monitor usage**: Check [https://platform.openai.com/usage](https://platform.openai.com/usage) regularly
4. **Set limits**: Configure spending limits in your OpenAI dashboard
5. **Revoke unused keys**: Delete old keys you're no longer using

### Protect Your Keys

- Never share keys in chat, email, or tickets
- Never commit keys to git repositories
- Never include keys in client-side code
- Never log keys in application logs
- Use environment variables or secret management systems in production

## Comparison with OAuth Providers

This example is in the `openai-oauth-flow` directory for consistency with other provider examples, but it's important to understand the differences:

| Feature | OpenAI (API Keys) | Qwen (OAuth) |
|---------|------------------|--------------|
| **Authentication Method** | API Key | OAuth 2.0 Device Flow |
| **Setup Complexity** | Simple | Complex (multi-step) |
| **Browser Required** | No | Yes (for authorization) |
| **Refresh Tokens** | Not needed | Yes (automatic refresh) |
| **Expiration** | Never (until revoked) | Yes (tokens expire) |
| **Scope Management** | Not applicable | Yes (per-scope permissions) |
| **Best For** | Server applications | User-delegated access |

### When to Use Each

**API Keys (OpenAI's method):**
- Server-to-server applications
- CLI tools and scripts
- Bots and automation
- When you control the account

**OAuth (Qwen's method):**
- User-facing applications
- When accessing user resources
- Multi-tenant applications
- When users need to authorize access

## Multiple API Keys

For advanced use cases, you can configure multiple API keys for failover:

```yaml
providers:
  openai:
    api_key: sk-proj-primary-key-here
    api_keys:
      - sk-proj-failover-key-1
      - sk-proj-failover-key-2
    default_model: gpt-4o
```

The AI Provider Kit will automatically fail over to backup keys if the primary key fails or hits rate limits.

## Environment Variables

As an alternative to the config file, you can use environment variables:

```bash
export OPENAI_API_KEY="sk-proj-your-key-here"
```

The provider will check environment variables if no key is found in the config file.

## Monitoring and Usage

Keep track of your OpenAI usage:

- **Usage Dashboard**: [https://platform.openai.com/usage](https://platform.openai.com/usage)
- **API Keys**: [https://platform.openai.com/api-keys](https://platform.openai.com/api-keys)
- **Billing**: [https://platform.openai.com/account/billing](https://platform.openai.com/account/billing)
- **Rate Limits**: [https://platform.openai.com/account/limits](https://platform.openai.com/account/limits)

## Cost Management

OpenAI charges per token usage:

1. **Set spending limits**: Configure monthly budget caps
2. **Monitor usage**: Check the usage dashboard regularly
3. **Use cheaper models**: GPT-3.5 Turbo is much cheaper than GPT-4
4. **Implement caching**: Cache responses when appropriate
5. **Optimize prompts**: Shorter prompts = lower costs

## Next Steps

After authenticating:

1. **Try the demo client**: See how to use the authenticated provider
2. **Build your application**: Use the ai-provider-kit with API key authentication
3. **Explore models**: Try different OpenAI models for your use case
4. **Monitor usage**: Keep track of your API usage and costs

## Related Examples

- `demo-client`: Basic usage with API key authentication
- `demo-client-streaming`: Streaming responses with OpenAI
- `config-demo`: Understanding config file structure
- `qwen-oauth-flow`: Example of OAuth authentication (for comparison)

## Further Reading

- [OpenAI API Documentation](https://platform.openai.com/docs)
- [OpenAI Authentication Guide](https://platform.openai.com/docs/api-reference/authentication)
- [OpenAI Rate Limits](https://platform.openai.com/docs/guides/rate-limits)
- [OpenAI Pricing](https://openai.com/pricing)

## License

This example is part of the ai-provider-kit project.
