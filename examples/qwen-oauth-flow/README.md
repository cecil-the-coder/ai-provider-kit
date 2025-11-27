# Qwen OAuth Device Code Flow Example

This example demonstrates how to authenticate with Qwen using OAuth 2.0 Device Code Flow. This is the recommended authentication method for Qwen's API, especially for CLI applications and tools.

## What This Example Does

This program guides you through the complete OAuth authentication process:

1. **Requests a device code** from Qwen's OAuth endpoint
2. **Displays a user code** and verification URL
3. **Opens your browser** automatically to the verification page
4. **Polls for token completion** while you authorize in the browser
5. **Saves OAuth credentials** to `~/.mcp-code-api/config.yaml`
6. **Tests the token** with a simple API call
7. **Displays success information** including token expiration

## Prerequisites

- Go 1.21 or later
- Internet connection
- A Qwen account (create one at https://chat.qwen.ai/)

## Installation

```bash
cd examples/qwen-oauth-flow
go mod download
```

## Usage

Simply run the program:

```bash
go run main.go
```

Or build and run:

```bash
go build -o qwen-oauth
./qwen-oauth
```

## What to Expect

When you run the program, you'll see output like this:

```
=======================================================================
Qwen OAuth Authentication
=======================================================================

This tool will guide you through authenticating with Qwen using OAuth.
After successful authentication, your credentials will be saved to:
  ~/.mcp-code-api/config.yaml

Step 1: Requesting device code...
✓ Device code obtained!

Step 2: Please authenticate in your browser:

  Verification URL: https://chat.qwen.ai/device
  User Code: ABCD-EFGH

Opening browser automatically...

Step 3: Waiting for authentication...
  ⏳ Polling... (attempt 1/60)
  ⏳ Polling... (attempt 2/60)
✓ Authentication successful!

Step 4: Saving tokens to config...
✓ Tokens saved to ~/.mcp-code-api/config.yaml

Step 5: Testing token...
✓ Token is valid!

=======================================================================
Authentication Complete
=======================================================================

  Access Token:  diYgUnX8Qu... (first 10 chars)
  Refresh Token: Rv1VrP8cP3... (first 10 chars)
  Expires:       2025-11-18 09:30:00 (23h 59m)

You can now use the Qwen provider with OAuth authentication!
```

## Authentication Flow Details

### Device Code Flow (RFC 8628) with PKCE

This example implements the OAuth 2.0 Device Code Flow with PKCE (Proof Key for Code Exchange):

1. **PKCE Generation**: The application generates a code verifier and challenge
2. **Device Code Request**: The application requests a device code from Qwen with the PKCE challenge
3. **User Authorization**: You visit the verification URL and enter the user code
4. **Token Polling**: The application polls Qwen's token endpoint with the PKCE verifier
5. **Token Grant**: Once you authorize, Qwen returns access and refresh tokens
6. **Token Storage**: Tokens are saved to the config file

**Why PKCE?** Qwen requires PKCE for enhanced security. PKCE prevents authorization code interception attacks by using cryptographic proof during the token exchange.

### Qwen OAuth Endpoints

- **Device Authorization**: `https://chat.qwen.ai/api/v1/oauth2/device/code`
- **Token Endpoint**: `https://chat.qwen.ai/api/v1/oauth2/token`
- **Verification URL**: `https://chat.qwen.ai/device`

### Client Credentials

- **Client ID**: `f0304373b74a44d2b584a3fb70ca9e56` (public client)
- **Scopes**: `model.completion`

This is Qwen's public OAuth client ID for device code flow - no client secret is required.

## Config File Structure

After successful authentication, your `~/.mcp-code-api/config.yaml` will be updated with:

```yaml
providers:
  qwen:
    oauth_credentials:
      - id: default
        client_id: f0304373b74a44d2b584a3fb70ca9e56
        access_token: your_access_token_here
        refresh_token: your_refresh_token_here
        expires_at: "2025-11-18T09:30:00-07:00"
        scopes:
          - model.completion
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
   The demo client will automatically use your OAuth credentials.

3. **Test with curl** (using your access token):
   ```bash
   curl -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
        https://portal.qwen.ai/v1/models
   ```

## Token Refresh

The saved refresh token will be automatically used by the ai-provider-kit to refresh your access token when it expires. The provider implements automatic token refresh, so you don't need to re-authenticate manually.

## Troubleshooting

### Browser doesn't open automatically

If your browser doesn't open:
1. Copy the verification URL from the terminal
2. Open it manually in your browser
3. Enter the user code displayed

### "authentication timeout" error

This means you didn't complete the authorization within the time limit (usually 10 minutes):
1. Run the program again to get a new device code
2. Complete the authorization more quickly

### "expired_token" error

The device code expired before you authorized:
1. Run the program again to get a fresh device code
2. The device code is typically valid for 10 minutes

### "access_denied" error

You clicked "Deny" in the browser:
1. Run the program again
2. Click "Allow" when prompted

### Token test fails but tokens are saved

This is usually okay - it might mean:
- The `/models` endpoint requires additional permissions
- There's a temporary API issue
- Your account needs activation

The tokens are still valid and will work for chat completions.

### Config file exists but can't be read

Check permissions:
```bash
chmod 600 ~/.mcp-code-api/config.yaml
```

## Implementation Notes

### Key Implementation Details

1. **PKCE Support**: Implements PKCE (RFC 7636) as required by Qwen's OAuth flow
   - Generates cryptographically random code verifier
   - Creates SHA256 code challenge
   - Includes challenge in device code request
   - Includes verifier in token exchange
2. **Request Format**: Uses `application/x-www-form-urlencoded` (not JSON) as required by Qwen's OAuth endpoint
3. **Headers**: Includes `x-request-id` and proper `User-Agent` headers
4. **Polling Logic**: Implements exponential backoff on `slow_down` errors
5. **Atomic Writes**: Uses temp file + rename for atomic config updates
6. **Error Handling**: Properly handles all OAuth error codes

### Differences from Standard OAuth

Qwen's implementation has some quirks:
- **Requires PKCE**: Unlike most public clients, Qwen mandates PKCE even for device code flow
- Uses device code flow (not authorization code flow)
- Public client (no client secret needed)
- Specific scope format: `model.completion`
- May return different error codes than standard OAuth

## Security Considerations

1. **Token Storage**: Tokens are stored in `~/.mcp-code-api/config.yaml` with permissions 0600 (user read/write only)
2. **No Client Secret**: Device code flow doesn't require a client secret, making it safe for public applications
3. **Refresh Tokens**: Refresh tokens are long-lived and should be kept secure
4. **HTTPS Only**: All communication uses HTTPS

## Next Steps

After authenticating:

1. **Try the demo client**: See how to use the authenticated provider
2. **Build your application**: Use the ai-provider-kit with OAuth authentication
3. **Add more credentials**: You can add multiple OAuth credential sets for load balancing

## Related Examples

- `demo-client`: Basic usage with OAuth credentials
- `demo-client-streaming`: Streaming responses with OAuth
- `config-demo`: Understanding config file structure

## License

This example is part of the ai-provider-kit project.
