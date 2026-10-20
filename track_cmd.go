package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"the-mail-reader/jobtracker"
)

var trackCmd = &cobra.Command{
	Use:     "track-jobs",
	Aliases: []string{"track", "process-jobs"},
	Short:   "Extract and consolidate job application histories from emails",
	Long: `Analyze downloaded email JSON files to:
1. Extract unique sender domains.
2. Enrich domains with correct company names using an AI model (or heuristics).
3. Scan for job-related keywords, state transitions (applied, interview, rejected, offer), and identifiers.
4. Group statuses by company, role, and identifier.
5. Export separate structured JSON files per application with chronological timelines.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		emailsDir, _ := cmd.Flags().GetString("emails")
		outputDir, _ := cmd.Flags().GetString("output")
		apiKey, _ := cmd.Flags().GetString("api-key")
		modelURL, _ := cmd.Flags().GetString("model-url")
		modelName, _ := cmd.Flags().GetString("model")
		cacheFile, _ := cmd.Flags().GetString("cache")

		cfg := jobtracker.Config{
			EmailsDir:   emailsDir,
			OutputDir:   outputDir,
			CacheFile:   cacheFile,
			ModelAPIKey: apiKey,
			ModelURL:    modelURL,
			ModelName:   modelName,
		}

		ctx := context.Background()
		_, err := jobtracker.Run(ctx, cfg)
		if err != nil {
			return fmt.Errorf("job tracker failed: %w", err)
		}

		return nil
	},
}

func init() {
	trackCmd.Flags().StringP("emails", "e", "emails/individual", "Path to directory containing individual email JSON files")
	trackCmd.Flags().StringP("output", "o", "job_applications", "Output directory for structured application JSON files")
	trackCmd.Flags().StringP("api-key", "k", "", "API key for LLM domain enrichment (falls back to OPENAI_API_KEY or GEMINI_API_KEY)")
	trackCmd.Flags().String("model-url", "", "Custom LLM endpoint URL (e.g. for local Ollama / vLLM / OpenAI proxy)")
	trackCmd.Flags().StringP("model", "m", "", "Model name for domain enrichment (default: gpt-4o-mini or gemini-1.5-flash)")
	trackCmd.Flags().String("cache", "", "Path to custom company domain cache JSON file")
}
