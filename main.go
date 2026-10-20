package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "mailreader",
		Short: "Download Outlook.com emails via Microsoft Graph API",
		Long:  `A tool to authenticate with Outlook.com and download emails as JSON files for offline processing.`,
	}

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(foldersCmd)
	rootCmd.AddCommand(trackCmd)
	rootCmd.AddCommand(serveCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
