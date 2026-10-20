package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"the-mail-reader/jobtracker"
)

var serveCmd = &cobra.Command{
	Use:     "serve",
	Aliases: []string{"ui", "web", "dashboard"},
	Short:   "Start web UI to view job applications and timelines",
	Long: `Start an in-memory HTTP server rendering an interactive web interface
with chronological timelines of job application transitions, ordered with the latest update on top.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		dir, _ := cmd.Flags().GetString("dir")

		server, err := jobtracker.NewAppServer(dir, port)
		if err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}

		return server.Start()
	},
}

func init() {
	serveCmd.Flags().IntP("port", "p", 8085, "HTTP server port")
	serveCmd.Flags().StringP("dir", "d", "job_applications", "Directory containing application JSON files")
}
