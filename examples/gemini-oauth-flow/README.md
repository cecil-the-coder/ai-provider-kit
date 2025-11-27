# Google Gemini OAuth Device Code Flow Example

This example demonstrates how to authenticate with Google Gemini using OAuth 2.0 Device Code Flow. This is the recommended authentication method for Gemini's Cloud Code API when using CLI applications and tools.

## What This Example Does

This program guides you through the complete OAuth authentication process:

1. **Requests a device code** from Google's OAuth endpoint
2. **Displays a user code** and verification URL
3. **Opens your browser** automatically to the verification page
4. **Polls for token completion** while you authorize in the browser
5. **Saves OAuth credentials** to `~/.mcp-code-api/config.yaml`
6. **Tests the token** to verify it's valid
7. **Displays success information** including token expiration

## Prerequisites

- Go 1.21 or later
- Internet connection
- A Google account
- (Optional) A Google Cloud project with the Generative Language API enabled

## Installation

```bash
cd examples/gemini-oauth-flow
go mod download
```

## Usage

Simply run the program:

```bash
go run main.go
```

Or build and run:

```bash
go build -o gemini-oauth
./gemini-oauth
```

## What to Expect

When you run the program, you'll see output like this:

```
=======================================================================
Google Gemini OAuth Authentication
=======================================================================

This tool will guide you through authenticating with Google Gemini using OAuth.
After successful authentication, your credentials will be saved to:
  ~/.mcp-code-api/config.yaml

Step 1: Requesting device code...
✓ Device code obtained!

Step 2: Please authenticate in your browser:

  Verification URL: https://www.google.com/device
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

  Access Token:  ya29.a0ATi... (first 10 chars)
  Refresh Token: 1//06egWan... (first 10 chars)
  Expires:       2025-11-17 10:30:00 (59m)

You can now use the Gemini provider with OAuth authentication!

Note: If you haven't set up a Google Cloud project yet, you may need to:
  1. Visit https://console.cloud.google.com/
  2. Create a new project or select an existing one
  3. Enable the Generative Language API
  4. Add the project_id to your config.yaml
```

## Authentication Flow Details

### Device Code Flow (RFC 8628)

This example implements the OAuth 2.0 Device Code Flow for limited-input devices:

1. **Device Code Request**: The application requests a device code from Google
2. **User Authorization**: You visit the verification URL and enter the user code
3. **Token Polling**: The application polls Google's token endpoint
4. **Token Grant**: Once you authorize, Google returns access and refresh tokens
5. **Token Storage**: Tokens are saved to the config file

### Google OAuth Endpoints

- **Device Authorization**: `https://oauth2.googleapis.com/device/code`
- **Token Endpoint**: `https://oauth2.googleapis.com/token`
- **Verification URL**: `https://www.google.com/device`

### Client Credentials

This example uses the official Google Gemini CLI OAuth credentials (these are PUBLIC and safe to use):

- **Client ID**: `681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com`
- **Client Secret**: `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`
- **Scopes**: `https://www.googleapis.com/auth/cloud-platform`

**Note**: These are the official credentials from Google's Gemini CLI, intentionally made public according to Google's OAuth2 installed application guidelines. The client secret is not treated as a secret in this context.

**Optional: Using Your Own Credentials**
If you prefer to use your own OAuth client:
1. Visit https://console.cloud.google.com/apis/credentials
2. Create a new OAuth 2.0 Client ID
3. Select "Web application" as the application type
4. Add `http://localhost:8080/oauth2callback` to authorized redirect URIs
5. Replace the values in main.go with your credentials

## Config File Structure

After successful authentication, your `~/.mcp-code-api/config.yaml` will be updated with:

```yaml
providers:
  gemini:
    oauth_credentials:
      - id: default
        client_id: 681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com  # Official Gemini CLI credentials
        client_secret: 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                              # Official Gemini CLI credentials
        access_token: ya29.a0ATi6K2v...
        refresh_token: 1//06egWanRJVY7M...
        expires_at: "2025-11-17T10:30:00-07:00"
        scopes:
          - https://www.googleapis.com/auth/cloud-platform
    project_id: ""  # Optional: Add your Google Cloud project ID here
```

## Setting Up Google Cloud Project (Optional)

While OAuth authentication will work without a project, you'll need a Google Cloud project to actually use the Gemini API:

1. **Create a Project**:
   - Visit https://console.cloud.google.com/
   - Click "Select a project" > "New Project"
   - Give it a name and create it

2. **Enable the API**:
   - Navigate to "APIs & Services" > "Enable APIs and Services"
   - Search for "Generative Language API" or "Cloud Code API"
   - Click "Enable"

3. **Add Project ID to Config**:
   - Edit `~/.mcp-code-api/config.yaml`
   - Under `gemini:`, add `project_id: your-project-id`

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
        https://generativelanguage.googleapis.com/v1beta/models
   ```

## Token Refresh

The saved refresh token will be automatically used by the ai-provider-kit to refresh your access token when it expires. The Gemini provider implements automatic token refresh, so you don't need to re-authenticate manually.

Access tokens typically expire after 1 hour, but the refresh token is long-lived and will be used to obtain new access tokens as needed.

## Troubleshooting

### Browser doesn't open automatically

If your browser doesn't open:
1. Copy the verification URL from the terminal
2. Open it manually in your browser
3. Enter the user code displayed

### "authentication timeout" error

This means you didn't complete the authorization within the time limit (usually 15 minutes):
1. Run the program again to get a new device code
2. Complete the authorization more quickly

### "expired_token" error

The device code expired before you authorized:
1. Run the program again to get a fresh device code
2. The device code is typically valid for 15 minutes

### "access_denied" error

You clicked "Deny" or "Cancel" in the browser:
1. Run the program again
2. Click "Allow" when prompted

### Token test fails but tokens are saved

This is usually okay - it might mean:
- You haven't set up a Google Cloud project yet
- The API isn't enabled in your project
- There's a temporary API issue

The tokens are still valid and will work once you set up your project.

### "insufficient permissions" error

Your Google account needs the appropriate permissions:
- Make sure you're using a Google account that has access to Google Cloud
- If using a workspace account, check with your administrator

## Creating Your Own OAuth Client

To use your own OAuth credentials instead of the example ones:

1. **Go to Google Cloud Console**:
   - Visit https://console.cloud.google.com/apis/credentials

2. **Create OAuth 2.0 Client**:
   - Click "Create Credentials" > "OAuth client ID"
   - Application type: "TVs and Limited Input devices"
   - Give it a name

3. **Update the Code**:
   - Replace `GoogleClientID` and `GoogleClientSecret` in `main.go`
   - Rebuild and run the program

## Implementation Notes

### Key Implementation Details

1. **Standard OAuth 2.0**: Implements the device code flow as defined in RFC 8628
2. **No PKCE Required**: Google's device code flow doesn't require PKCE (unlike some other providers)
3. **Request Format**: Uses `application/x-www-form-urlencoded` for OAuth requests
4. **Polling Logic**: Implements exponential backoff on `slow_down` errors
5. **Atomic Writes**: Uses temp file + rename for atomic config updates
6. **Error Handling**: Properly handles all OAuth error codes
7. **Token Validation**: Tests tokens using Google's tokeninfo endpoint

### Differences from Qwen OAuth

While both use device code flow, there are some differences:
- **No PKCE**: Google doesn't require PKCE for device code flow
- **Client Secret**: Google requires a client secret (Qwen doesn't)
- **Scopes**: Google uses standard OAuth scopes (cloud-platform)
- **Endpoints**: Different OAuth server endpoints
- **Token Format**: Google uses JWT-style tokens (ya29.*)

### Google OAuth Specifics

- **Token Expiry**: Access tokens expire after 1 hour
- **Refresh Tokens**: Long-lived, can be used indefinitely
- **Scope**: `cloud-platform` scope provides broad GCP access
- **Rate Limits**: Device code requests are rate limited
- **Security**: Always use HTTPS; tokens should be stored securely

## Security Considerations

1. **Token Storage**: Tokens are stored in `~/.mcp-code-api/config.yaml` with permissions 0600 (user read/write only)
2. **Client Secret**: The client secret is included in the code - for production, use environment variables
3. **Refresh Tokens**: Refresh tokens are long-lived and should be kept secure
4. **HTTPS Only**: All communication uses HTTPS
5. **Scope Access**: The `cloud-platform` scope provides broad access - consider narrowing it for production

## OAuth Scopes

The `https://www.googleapis.com/auth/cloud-platform` scope provides:
- Full access to all Google Cloud Platform services
- Ability to manage resources
- Access to Gemini/Generative Language API

For more restricted access, you could use:
- `https://www.googleapis.com/auth/generative-language` (if available)
- Custom scopes based on your needs

## Next Steps

After authenticating:

1. **Set up a Google Cloud project**: Enable the Generative Language API
2. **Add project_id to config**: Update your config.yaml with the project ID
3. **Try the demo client**: See how to use the authenticated provider
4. **Build your application**: Use the ai-provider-kit with OAuth authentication
5. **Add more credentials**: You can add multiple OAuth credential sets for load balancing

## Related Examples

- `demo-client`: Basic usage with OAuth credentials
- `demo-client-streaming`: Streaming responses with OAuth
- `qwen-oauth-flow`: Similar OAuth flow for Qwen provider
- `config-demo`: Understanding config file structure

## Additional Resources

- [Google OAuth 2.0 Documentation](https://developers.google.com/identity/protocols/oauth2)
- [Device Code Flow (RFC 8628)](https://tools.ietf.org/html/rfc8628)
- [Google Cloud Console](https://console.cloud.google.com/)
- [Generative Language API](https://developers.google.com/generative-ai)

## License

This example is part of the ai-provider-kit project.
