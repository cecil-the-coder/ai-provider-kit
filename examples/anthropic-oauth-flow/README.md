# Anthropic OAuth Authorization Code Flow Example

This example demonstrates how to authenticate with Anthropic using OAuth 2.0 Authorization Code Flow with PKCE (Proof Key for Code Exchange). This is the authentication method used by Claude Code for claude.ai subscription users.

## What This Example Does

This program guides you through the complete OAuth authentication process:

1. **Generates PKCE challenge** for secure authentication
2. **Starts a local callback server** to receive the OAuth response
3. **Opens your browser** to Anthropic's authorization page
4. **Receives the authorization code** from the callback
5. **Exchanges code for tokens** (access token + refresh token)
6. **Saves OAuth credentials** to `~/.mcp-code-api/config.yaml`
7. **Tests the token** with a simple API call
8. **Displays success information** including token expiration

## Important Notes

### OAuth Token Restrictions

Anthropic's OAuth tokens obtained through this flow are **primarily designed for Claude Code** and may have limitations:

- These tokens are intended for use with Claude Code specifically
- They may not work with general API endpoints the same way API keys do
- For unrestricted API access, use an API key from [console.anthropic.com](https://console.anthropic.com/)

### When to Use This vs API Keys

| Authentication Method | Use Case | How to Get |
|----------------------|----------|------------|
| **OAuth (this example)** | Claude Code integration, subscription users | This OAuth flow |
| **API Keys** | General API access, production applications | console.anthropic.com |

## Prerequisites

- Go 1.21 or later
- Internet connection
- A claude.ai account (Pro, Team, or Enterprise)
- Browser access for authentication

## Installation

```bash
cd examples/anthropic-oauth-flow
go mod download
```

## Usage

Simply run the program:

```bash
go run main.go
```

Or build and run:

```bash
go build -o anthropic-oauth
./anthropic-oauth
```

## What to Expect

When you run the program, you'll see output like this:

```
=======================================================================
Anthropic OAuth Authentication (Authorization Code Flow)
=======================================================================

This tool will guide you through authenticating with Anthropic using OAuth.
After successful authentication, your credentials will be saved to:
  ~/.mcp-code-api/config.yaml

NOTE: Anthropic OAuth is primarily designed for Claude Code.
These credentials may have limited API access compared to regular API keys.

Step 1: Starting local callback server...
✓ Callback server started on http://localhost:8765/callback

Step 2: Building authorization URL...
✓ Authorization URL ready!

Step 3: Please authenticate in your browser:

  Authorization URL: https://claude.ai/oauth/authorize?client_id=...

Opening browser automatically...

Step 4: Waiting for authorization...
  (Complete the authentication in your browser)

✓ Authorization code received!

Step 5: Exchanging code for access token...
✓ Access token obtained!

Step 6: Saving tokens to config...
✓ Tokens saved to ~/.mcp-code-api/config.yaml

Step 7: Testing token...
⚠ Warning: Token test failed: HTTP 403: ...
  Note: This is expected - Anthropic OAuth tokens are restricted to Claude Code.
  The token was saved successfully and will work with Claude Code.

=======================================================================
Authentication Complete
=======================================================================

  Access Token:  eyJhbGciO... (first 10 chars)
  Refresh Token: eyJhbGciO... (first 10 chars)
  Expires:       2025-11-18 09:30:00 (23h 59m)

You can now use the Anthropic provider with OAuth authentication!

Important: These OAuth credentials are designed for Claude Code.
For general API access, consider using an API key from console.anthropic.com
```

## Authentication Flow Details

### Authorization Code Flow with PKCE

This example implements the OAuth 2.0 Authorization Code Flow with PKCE:

1. **PKCE Generation**: The application generates a code verifier and challenge
2. **Authorization Request**: Browser opens to Anthropic's authorization page
3. **User Authorization**: You log in and approve the application
4. **Callback**: Anthropic redirects to local server with authorization code
5. **Token Exchange**: Application exchanges code for access/refresh tokens with PKCE verifier
6. **Token Storage**: Tokens are saved to the config file

**Why PKCE?** PKCE (Proof Key for Code Exchange) prevents authorization code interception attacks by using cryptographic proof during the token exchange.

### Anthropic OAuth Endpoints

- **Authorization Endpoint**: `https://claude.ai/oauth/authorize`
- **Token Endpoint**: `https://console.anthropic.com/v1/oauth/token`
- **Callback URI**: `http://localhost:8765/callback` (local)

### Client Credentials

- **Client ID**: `9d1c250a-e61b-44d9-88ed-5944d1962f5e` (public client for Claude Code)
- **Scopes**: `org:create_api_key user:profile user:inference`

This is Anthropic's public OAuth client ID for Claude Code - no client secret is required.

## Config File Structure

After successful authentication, your `~/.mcp-code-api/config.yaml` will be updated with:

```yaml
providers:
  anthropic:
    oauth_credentials:
      - id: default
        client_id: 9d1c250a-e61b-44d9-88ed-5944d1962f5e
        access_token: your_access_token_here
        refresh_token: your_refresh_token_here
        expires_at: "2025-11-18T09:30:00-07:00"
        scopes:
          - org:create_api_key
          - user:profile
          - user:inference
```

## How to Verify It Worked

After running the authentication flow, you can verify it worked by:

1. **Check the config file**:
   ```bash
   cat ~/.mcp-code-api/config.yaml
   ```

2. **Use with Claude Code** or compatible applications that support Anthropic OAuth

3. **Check token expiration** - OAuth tokens typically last 24 hours

## Token Refresh

The saved refresh token will be automatically used by the ai-provider-kit to refresh your access token when it expires. The provider implements automatic token refresh using the endpoint:

```
POST https://console.anthropic.com/v1/oauth/token
{
  "grant_type": "refresh_token",
  "client_id": "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
  "refresh_token": "your_refresh_token"
}
```

## Troubleshooting

### Browser doesn't open automatically

If your browser doesn't open:
1. Copy the authorization URL from the terminal
2. Open it manually in your browser
3. Complete the authentication

### "authorization timeout" error

This means you didn't complete the authorization within 5 minutes:
1. Run the program again to start a fresh OAuth flow
2. Complete the authorization more quickly

### "access_denied" error

You clicked "Deny" or cancelled in the browser:
1. Run the program again
2. Click "Allow" when prompted

### Token test fails (403 Forbidden)

**This is expected behavior!** Anthropic OAuth tokens are restricted to Claude Code:
- The tokens were saved successfully
- They will work with Claude Code
- For general API access, use an API key instead

### Port 8765 already in use

Another process is using the callback port:
1. Stop the other process
2. Or modify the `ServerPort` constant in main.go

### Config file exists but can't be read

Check permissions:
```bash
chmod 600 ~/.mcp-code-api/config.yaml
```

## Implementation Notes

### Key Implementation Details

1. **Authorization Code Flow**: Uses standard OAuth 2.0 authorization code flow (not device code flow like Qwen)
2. **PKCE Support**: Implements PKCE (RFC 7636) for enhanced security
   - Generates cryptographically random code verifier
   - Creates SHA256 code challenge
   - Includes challenge in authorization request
   - Includes verifier in token exchange
3. **Local Callback Server**: Runs on localhost:8765 to receive the OAuth callback
4. **JSON Format**: Uses JSON for token exchange (not form-encoded)
5. **Atomic Writes**: Uses temp file + rename for atomic config updates
6. **Error Handling**: Properly handles all OAuth error codes

### Differences from Qwen OAuth Flow

| Feature | Anthropic | Qwen |
|---------|-----------|------|
| **OAuth Flow** | Authorization Code Flow | Device Code Flow |
| **User Interaction** | Browser redirect to localhost | Manual code entry |
| **Callback Server** | Required (localhost:8765) | Not needed |
| **Token Format** | JSON request/response | Form-encoded |
| **Primary Use Case** | Claude Code integration | General API access |
| **Token Restrictions** | Limited to Claude Code | Full API access |

### Why Not Device Code Flow?

Anthropic uses **Authorization Code Flow** instead of Device Code Flow because:
- It's designed for browser-capable applications (like Claude Code)
- Provides better UX with automatic redirects
- Standard for desktop/CLI applications with browser access
- Device Code Flow is typically for devices without browsers (TVs, IoT, etc.)

## Security Considerations

1. **Token Storage**: Tokens are stored in `~/.mcp-code-api/config.yaml` with permissions 0600 (user read/write only)
2. **No Client Secret**: Authorization Code Flow with PKCE doesn't require a client secret, making it safe for public applications
3. **Refresh Tokens**: Refresh tokens are long-lived and should be kept secure
4. **HTTPS Only**: All communication uses HTTPS (except localhost callback)
5. **PKCE Protection**: PKCE prevents authorization code interception attacks

## Alternative: Using API Keys

If you need full API access without OAuth restrictions, use API keys instead:

1. Visit [console.anthropic.com](https://console.anthropic.com/)
2. Create an API key
3. Add it to your config:
   ```yaml
   providers:
     anthropic:
       api_key: sk-ant-api03-...
   ```

API keys provide:
- Full API access without restrictions
- No token expiration or refresh needed
- Simpler integration
- Support for all Anthropic API endpoints

However, they require a paid API plan, whereas OAuth works with claude.ai subscriptions.

## Next Steps

After authenticating:

1. **Use with Claude Code**: Your OAuth credentials will work with Claude Code
2. **Build applications**: Use the ai-provider-kit with OAuth authentication
3. **Add API keys**: Consider adding API keys for unrestricted API access

## Related Examples

- `qwen-oauth-flow`: Device Code Flow implementation (different OAuth flow)
- `demo-client`: Basic usage examples (uses API keys by default)

## References

- [OAuth 2.0 Authorization Code Flow](https://datatracker.ietf.org/doc/html/rfc6749#section-4.1)
- [PKCE (RFC 7636)](https://datatracker.ietf.org/doc/html/rfc7636)
- [Anthropic Console](https://console.anthropic.com/)
- [Claude API Documentation](https://docs.anthropic.com/)

## License

This example is part of the ai-provider-kit project.
