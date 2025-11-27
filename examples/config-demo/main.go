package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/examples/config"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// This example demonstrates how to use the shared config package to parse
// the demo-client config.yaml format and construct proper types.ProviderConfig
// structures for the ai-provider-kit module.
//
// It shows how to handle:
// - Single API keys
// - Multiple API keys (api_keys array)
// - OAuth credentials (with multiple credential sets)
// - Custom providers with different types
// - Provider-specific settings (base_url, default_model, max_tokens, etc.)

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.yaml", "Path to config file (default: config.yaml in current directory)")
	flag.Parse()

	fmt.Println("=======================================================================")
	fmt.Println("AI Provider Kit - Config Demo")
	fmt.Println("=======================================================================")
	fmt.Println()
	fmt.Println("This demo shows how to use the shared config package to parse")
	fmt.Println("config.yaml files and construct types.ProviderConfig structures.")
	fmt.Println()

	// Load and parse the configuration file using the shared config package
	fmt.Printf("Loading configuration from: %s\n", *configFile)
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Printf("Error loading config from '%s': %v\n", *configFile, err)
		fmt.Println()
		fmt.Println("Hint: Create a config.yaml file in the current directory or use -config flag")
		fmt.Println("Example:")
		fmt.Println("  ./config-demo -config config.yaml.example")
		fmt.Println("  ./config-demo -config ../demo-client/config.yaml")
		os.Exit(1)
	}

	fmt.Println("Configuration loaded successfully!")
	fmt.Println()

	// Display configuration summary
	displayConfigSummary(cfg)

	// Process each enabled provider
	fmt.Println("=======================================================================")
	fmt.Println("Processing Enabled Providers")
	fmt.Println("=======================================================================")
	fmt.Println()

	for i, providerName := range cfg.Providers.Enabled {
		fmt.Printf("[%d/%d] Processing: %s\n", i+1, len(cfg.Providers.Enabled), providerName)
		fmt.Println("-----------------------------------------------------------------------")

		// Get the provider config entry using the shared package
		providerEntry := config.GetProviderEntry(cfg, providerName)
		if providerEntry == nil {
			fmt.Printf("  WARNING: No configuration found for provider '%s'\n", providerName)
			fmt.Println()
			continue
		}

		// Build the types.ProviderConfig using the shared package
		providerConfig := config.BuildProviderConfig(providerName, providerEntry)

		// Display the constructed config
		displayProviderConfig(providerName, providerEntry, providerConfig)

		fmt.Println()
	}

	// Show how to construct OAuthCredentialSet arrays
	fmt.Println("=======================================================================")
	fmt.Println("OAuth Credentials Conversion Examples")
	fmt.Println("=======================================================================")
	fmt.Println()

	demonstrateOAuthConversion(cfg)

	fmt.Println("=======================================================================")
	fmt.Println("Example Complete!")
	fmt.Println("=======================================================================")
	fmt.Println()
	fmt.Println("Key Takeaways:")
	fmt.Println("  1. Use the shared config package to parse config.yaml files")
	fmt.Println("  2. Use types.ProviderConfig for provider configuration")
	fmt.Println("  3. Handle multiple auth methods: api_key, api_keys[], oauth_credentials[]")
	fmt.Println("  4. For OAuth, use config.ConvertOAuthCredentials() to convert to []*types.OAuthCredentialSet")
	fmt.Println("  5. Custom providers use their 'type' field to determine the API")
	fmt.Println("  6. Set BaseURL for custom providers or provider-specific endpoints")
	fmt.Println()
}

// =============================================================================
// Display Functions
// =============================================================================

// displayConfigSummary shows a high-level summary of the configuration
func displayConfigSummary(cfg *config.DemoConfig) {
	fmt.Println("Configuration Summary:")
	fmt.Println("-----------------------------------------------------------------------")
	fmt.Printf("  Enabled Providers: %d\n", len(cfg.Providers.Enabled))
	for _, name := range cfg.Providers.Enabled {
		fmt.Printf("    - %s\n", name)
	}
	fmt.Println()

	if len(cfg.Providers.PreferredOrder) > 0 {
		fmt.Printf("  Preferred Order: %d providers\n", len(cfg.Providers.PreferredOrder))
		for i, name := range cfg.Providers.PreferredOrder {
			fmt.Printf("    %d. %s\n", i+1, name)
		}
		fmt.Println()
	}

	if cfg.Metrics.Enabled {
		fmt.Printf("  Metrics: Enabled (port %d)\n", cfg.Metrics.Port)
	}

	if cfg.Async.Enabled {
		fmt.Println("  Async: Enabled")
	}
	fmt.Println()
}

// displayProviderConfig shows detailed information about a provider configuration
func displayProviderConfig(name string, entry *config.ProviderConfigEntry, cfg types.ProviderConfig) {
	fmt.Printf("  Provider Name: %s\n", name)
	fmt.Printf("  Provider Type: %s\n", cfg.Type)

	// Authentication method
	authMethod := "None"
	if cfg.APIKey != "" {
		if len(entry.APIKeys) > 0 {
			authMethod = fmt.Sprintf("Multiple API Keys (%d keys)", len(entry.APIKeys))
		} else if len(entry.OAuthCredentials) > 0 {
			authMethod = fmt.Sprintf("OAuth (%d credential sets)", len(entry.OAuthCredentials))
		} else {
			authMethod = "Single API Key"
		}
	}
	fmt.Printf("  Authentication: %s\n", authMethod)

	// Show API key (masked)
	if cfg.APIKey != "" {
		maskedKey := config.MaskAPIKey(cfg.APIKey)
		fmt.Printf("  API Key: %s\n", maskedKey)
	}

	// Show additional API keys if present
	if len(entry.APIKeys) > 1 {
		fmt.Println("  Additional API Keys:")
		for i, key := range entry.APIKeys[1:] {
			fmt.Printf("    [%d] %s\n", i+2, config.MaskAPIKey(key))
		}
	}

	// Show OAuth credentials if present
	if len(entry.OAuthCredentials) > 0 {
		fmt.Println("  OAuth Credentials:")
		for i, cred := range entry.OAuthCredentials {
			fmt.Printf("    [%d] ID: %s\n", i+1, cred.ID)
			if cred.ClientID != "" {
				fmt.Printf("        Client ID: %s\n", config.MaskAPIKey(cred.ClientID))
			}
			if cred.AccessToken != "" {
				fmt.Printf("        Access Token: %s\n", config.MaskAPIKey(cred.AccessToken))
			}
			if cred.RefreshToken != "" {
				fmt.Printf("        Refresh Token: %s\n", config.MaskAPIKey(cred.RefreshToken))
			}
			if cred.ExpiresAt != "" {
				fmt.Printf("        Expires: %s\n", cred.ExpiresAt)
			}
			if len(cred.Scopes) > 0 {
				fmt.Printf("        Scopes: %v\n", cred.Scopes)
			}
		}
	}

	// Show optional configuration
	if cfg.BaseURL != "" {
		fmt.Printf("  Base URL: %s\n", cfg.BaseURL)
	}

	if cfg.DefaultModel != "" {
		fmt.Printf("  Default Model: %s\n", cfg.DefaultModel)
	}

	if cfg.MaxTokens > 0 {
		fmt.Printf("  Max Tokens: %d\n", cfg.MaxTokens)
	}

	if entry.Temperature > 0 {
		fmt.Printf("  Temperature: %.2f\n", entry.Temperature)
	}

	if entry.ProjectID != "" {
		fmt.Printf("  Project ID: %s\n", entry.ProjectID)
	}

	// Show types.ProviderConfig structure
	fmt.Println()
	fmt.Println("  Constructed types.ProviderConfig:")
	fmt.Printf("    Type: %s\n", cfg.Type)
	fmt.Printf("    Name: %s\n", cfg.Name)
	if cfg.BaseURL != "" {
		fmt.Printf("    BaseURL: %s\n", cfg.BaseURL)
	}
	if cfg.APIKey != "" {
		fmt.Printf("    APIKey: %s\n", config.MaskAPIKey(cfg.APIKey))
	}
	if cfg.DefaultModel != "" {
		fmt.Printf("    DefaultModel: %s\n", cfg.DefaultModel)
	}
	if cfg.MaxTokens > 0 {
		fmt.Printf("    MaxTokens: %d\n", cfg.MaxTokens)
	}
	if len(cfg.OAuthCredentials) > 0 {
		fmt.Printf("    OAuthCredentials: %d sets\n", len(cfg.OAuthCredentials))
	}
}

// demonstrateOAuthConversion shows how to convert OAuth credentials
func demonstrateOAuthConversion(cfg *config.DemoConfig) {
	hasOAuth := false

	// Check all providers for OAuth credentials
	for _, providerName := range cfg.Providers.Enabled {
		entry := config.GetProviderEntry(cfg, providerName)
		if entry != nil && len(entry.OAuthCredentials) > 0 {
			if !hasOAuth {
				hasOAuth = true
				fmt.Println("OAuth Credential Conversion:")
				fmt.Println()
			}

			fmt.Printf("Provider: %s\n", providerName)
			fmt.Println("  Original (config.yaml format):")
			for i, cred := range entry.OAuthCredentials {
				fmt.Printf("    [%d] id: %s, client_id: %s\n", i, cred.ID, config.MaskAPIKey(cred.ClientID))
			}

			fmt.Println("  Converted (types.OAuthCredentialSet):")
			credSets := config.ConvertOAuthCredentials(entry.OAuthCredentials)
			for i, credSet := range credSets {
				fmt.Printf("    [%d] ID: %s, ClientID: %s\n", i, credSet.ID, config.MaskAPIKey(credSet.ClientID))
				if !credSet.ExpiresAt.IsZero() {
					fmt.Printf("        ExpiresAt: %s\n", credSet.ExpiresAt.Format(time.RFC3339))
				}
			}
			fmt.Println()
		}
	}

	if !hasOAuth {
		fmt.Println("No OAuth credentials found in configuration.")
		fmt.Println()
		fmt.Println("OAuth credentials should be structured as:")
		fmt.Println("  oauth_credentials:")
		fmt.Println("    - id: default")
		fmt.Println("      client_id: your_client_id")
		fmt.Println("      client_secret: your_client_secret")
		fmt.Println("      access_token: your_access_token")
		fmt.Println("      refresh_token: your_refresh_token")
		fmt.Println("      expires_at: \"2025-11-16T18:38:22-07:00\"")
		fmt.Println("      scopes:")
		fmt.Println("        - scope1")
		fmt.Println("        - scope2")
		fmt.Println()
	}
}
