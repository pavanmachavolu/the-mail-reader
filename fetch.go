package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Download emails as JSON files",
	Long: `Download emails from Outlook.com and save them as JSON files.
Filters can be applied with flags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := loadConfig()
		if err != nil {
			return fmt.Errorf("no configuration found, run 'mailreader auth' first: %w", err)
		}
		if config.ClientID == "" {
			return fmt.Errorf("client ID not configured, run 'mailreader auth' first")
		}

		folder, _ := cmd.Flags().GetString("folder")
		since, _ := cmd.Flags().GetString("since")
		until, _ := cmd.Flags().GetString("until")
		maxItems, _ := cmd.Flags().GetInt("max")
		outputDir, _ := cmd.Flags().GetString("output")

		if outputDir == "" {
			outputDir, _ = os.Getwd()
			outputDir = filepath.Join(outputDir, "emails")
		}

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		filter, err := buildDateFilter(since, until)
		if err != nil {
			return err
		}

		// Resolve folder
		var folderID, folderName string
		if folder == "" {
			folder = "inbox"
		}

		if folder == "all" {
			folders, err := getFolders(config)
			if err != nil {
				return fmt.Errorf("failed to fetch folders: %w", err)
			}
			for _, f := range folders {
				fmt.Printf("Fetching folder: %s...\n", f.DisplayName)
				messages, err := fetchMessages(config, f.ID, f.DisplayName, maxItems, filter)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error fetching folder %s: %v\n", f.DisplayName, err)
					continue
				}
				if err := saveMessages(messages, outputDir); err != nil {
					return err
				}
				fmt.Printf("  Saved %d messages\n", len(messages))
			}
			return nil
		}

		folders, err := getFolders(config)
		if err != nil {
			return fmt.Errorf("failed to fetch folders: %w", err)
		}

		found := false
		for _, f := range folders {
			if f.DisplayName == folder {
				folderID = f.ID
				folderName = f.DisplayName
				found = true
				break
			}
		}
		if !found {
			folderID = folder
			folderName = folder
		}

		fmt.Printf("Fetching messages from '%s'...\n", folderName)

		messages, err := fetchMessages(config, folderID, folderName, maxItems, filter)
		if err != nil {
			return fmt.Errorf("failed to fetch messages: %w", err)
		}

		if err := saveMessages(messages, outputDir); err != nil {
			return err
		}

		fmt.Printf("Saved %d messages to %s\n", len(messages), outputDir)
		return nil
	},
}

func buildDateFilter(since, until string) (string, error) {
	var filters []string
	if since != "" {
		sinceTime, err := time.Parse("2006-01-02", since)
		if err != nil {
			return "", fmt.Errorf("invalid --since date (use YYYY-MM-DD): %w", err)
		}
		filters = append(filters, fmt.Sprintf("receivedDateTime ge %s", sinceTime.Format("2006-01-02T00:00:00Z")))
	}
	if until != "" {
		untilTime, err := time.Parse("2006-01-02", until)
		if err != nil {
			return "", fmt.Errorf("invalid --until date (use YYYY-MM-DD): %w", err)
		}
		untilTime = untilTime.Add(24 * time.Hour)
		filters = append(filters, fmt.Sprintf("receivedDateTime lt %s", untilTime.Format("2006-01-02T00:00:00Z")))
	}
	return strings.Join(filters, " and "), nil
}

func saveMessages(messages []Message, outputDir string) error {
	if len(messages) == 0 {
		return nil
	}

	// Save one JSON file per message — scales to large mailboxes without
	// loading everything into memory at once.
	individualDir := filepath.Join(outputDir, "individual")
	if err := os.MkdirAll(individualDir, 0755); err != nil {
		return err
	}

	for _, m := range messages {
		filename := fmt.Sprintf("%s.json", m.ID)
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(individualDir, filename), data, 0644); err != nil {
			return err
		}
	}

	return nil
}

func init() {
	fetchCmd.Flags().StringP("folder", "f", "inbox", "Folder to fetch (inbox, sent, all, or folder name)")
	fetchCmd.Flags().StringP("since", "s", "", "Only fetch emails received on or after this date (YYYY-MM-DD)")
	fetchCmd.Flags().StringP("until", "u", "", "Only fetch emails received on or before this date (YYYY-MM-DD)")
	fetchCmd.Flags().IntP("max", "m", 500, "Maximum number of messages per request")
	fetchCmd.Flags().StringP("output", "o", "", "Output directory (default: ./emails)")
}
