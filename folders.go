package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var foldersCmd = &cobra.Command{
	Use:   "folders",
	Short: "List your mail folders",
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := loadConfig()
		if err != nil {
			return fmt.Errorf("no configuration found, run 'mailreader auth' first: %w", err)
		}

		folders, err := getFolders(config)
		if err != nil {
			return fmt.Errorf("failed to fetch folders: %w", err)
		}

		fmt.Printf("%-40s %s\n", "Name", "ID")
		fmt.Println(strings.Repeat("-", 90))
		for _, f := range folders {
			fmt.Printf("%-40s %s\n", f.DisplayName, f.ID)
		}
		return nil
	},
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "darwin":
		err = exec.Command("open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	default:
		return
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not open browser: %v\n", err)
	}
}
