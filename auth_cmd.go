package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Outlook.com",
	Long: `Authenticate with Outlook.com via Microsoft Graph API.

Requires an Azure App Registration (a free Microsoft developer account).
You'll need to provide the Application (client) ID. Optionally, if you
created a client secret, provide that too.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var clientID string
		var clientSecret string

		config, err := loadConfig()
		if err == nil && config.ClientID != "" {
			fmt.Printf("Existing configuration found (client ID: %s)\n", config.ClientID)
			fmt.Println("To re-authenticate, enter a new client ID or press Enter to keep the existing one.")
			fmt.Print("Client ID: ")
			fmt.Scanln(&clientID)
			if clientID == "" {
				clientID = config.ClientID
			}
		} else {
			fmt.Print("Microsoft Application (client) ID: ")
			fmt.Scanln(&clientID)
		}

		if clientID == "" {
			return fmt.Errorf("client ID is required")
		}

		fmt.Println("")
		fmt.Print("Client secret (optional, press Enter to skip): ")
		fmt.Scanln(&clientSecret)

		state := generateState()
		verifier := generateVerifier()

		authURL := buildAuthURL(clientID, state, verifier)

		fmt.Println("")
		fmt.Println("Opening your browser to authorize access...")
		fmt.Println("If the browser doesn't open, copy and paste this URL:")
		fmt.Println("")
		fmt.Println(authURL)
		fmt.Println("")

		openBrowser(authURL)

		code, err := StartCallbackServer(state)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		tokenResp, err := ExchangeCode(code, verifier, clientID, clientSecret)
		if err != nil {
			return fmt.Errorf("failed to exchange code: %w", err)
		}

		newConfig := &Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		}

		if err := saveConfig(newConfig); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Println("\n✓ Authentication successful! Configuration saved.")
		fmt.Println("  Run 'mailreader fetch' to download your emails.")
		return nil
	},
}
