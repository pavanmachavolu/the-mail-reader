package jobtracker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PipelineResult captures metrics and output from executing the job tracking workflow.
type PipelineResult struct {
	TotalEmailsProcessed int
	UniqueDomainsFound   int
	JobRelatedEmails     int
	TotalApplications    int
	OutputDirectory      string
	Duration             time.Duration
}

// Run executes the complete 5-step job application tracking pipeline.
func Run(ctx context.Context, cfg Config) (*PipelineResult, error) {
	startTime := time.Now()

	if cfg.EmailsDir == "" {
		cfg.EmailsDir = filepath.Join("emails", "individual")
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join("job_applications")
	}

	fmt.Println("====================================================")
	fmt.Println("🚀 Starting Job Application Extraction Pipeline")
	fmt.Println("====================================================")

	// ----------------------------------------------------
	// STEP 1: Extract unique sender domains from JSON files
	// ----------------------------------------------------
	fmt.Println("\n[Step 1/5] Extracting unique sender domains from emails...")
	uniqueDomains, err := ExtractUniqueDomains(cfg.EmailsDir)
	if err != nil {
		return nil, fmt.Errorf("step 1 failed: %w", err)
	}
	fmt.Printf("✓ Found %d unique sender domains across emails.\n", len(uniqueDomains))

	// ----------------------------------------------------
	// STEP 2: Enrich unique domains with company names
	// ----------------------------------------------------
	fmt.Println("\n[Step 2/5] Enriching domains with company names...")
	enricher := NewEnricher(cfg.CacheFile, cfg.ModelAPIKey, cfg.ModelURL, cfg.ModelName)
	domainCompanyMap, err := enricher.Enrich(ctx, uniqueDomains)
	if err != nil {
		return nil, fmt.Errorf("step 2 failed: %w", err)
	}
	fmt.Printf("✓ Enriched %d domains with corresponding company names.\n", len(domainCompanyMap))

	// ----------------------------------------------------
	// STEP 3: Iterate through original files again for job transitions & identifiers
	// ----------------------------------------------------
	fmt.Println("\n[Step 3/5] Scanning emails for job-related events, transitions, and identifiers...")
	entries, err := os.ReadDir(cfg.EmailsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read emails directory: %w", err)
	}

	var transitions []*StateTransition
	totalProcessed := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		totalProcessed++

		filePath := filepath.Join(cfg.EmailsDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var msg EmailMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		domain := ExtractDomain(msg.From)
		if IsExcludedDomain(domain) {
			continue
		}

		if !IsJobRelated(&msg) {
			continue
		}
		company := domainCompanyMap[domain]
		company = ResolveActualCompany(domain, company, msg.Subject, msg.BodyText+" "+msg.BodyMarkdown)

		role, jobID, team, uniqueID := ExtractMetadata(&msg, company)
		status, snippet := ExtractTransitionState(msg.Subject, msg.BodyText+" "+msg.BodyMarkdown)

		transition := &StateTransition{
			Date:             msg.ReceivedAt,
			Status:           status,
			Subject:          msg.Subject,
			EmailID:          msg.ID,
			From:             msg.From,
			Company:          company,
			Role:             role,
			JobID:            jobID,
			Team:             team,
			UniqueIdentifier: uniqueID,
			Snippet:          snippet,
		}

		transitions = append(transitions, transition)
	}

	fmt.Printf("✓ Identified %d job-related emails across %d total processed files.\n", len(transitions), totalProcessed)

	// Sort all transitions strictly by send/receive date time (never relying on file reading order)
	sort.SliceStable(transitions, func(i, j int) bool {
		if transitions[i].Date.Equal(transitions[j].Date) {
			return transitions[i].EmailID < transitions[j].EmailID
		}
		return transitions[i].Date.Before(transitions[j].Date)
	})

	// ----------------------------------------------------
	// STEP 4: Group statuses by company, role, and unique identifier
	// ----------------------------------------------------
	fmt.Println("\n[Step 4/5] Grouping transitions into applications and building chronological histories...")
	applications := GroupTransitions(transitions)
	fmt.Printf("✓ Grouped into %d distinct job applications.\n", len(applications))

	// ----------------------------------------------------
	// STEP 5: Write consolidated chronological data into separate structured JSON files
	// ----------------------------------------------------
	fmt.Printf("\n[Step 5/5] Writing separate application JSON files to %s...\n", cfg.OutputDir)
	writtenFiles, err := WriteApplications(applications, cfg.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("step 5 failed: %w", err)
	}
	fmt.Printf("✓ Successfully wrote %d application JSON files and applications_summary.json.\n", len(writtenFiles))

	result := &PipelineResult{
		TotalEmailsProcessed: totalProcessed,
		UniqueDomainsFound:   len(uniqueDomains),
		JobRelatedEmails:     len(transitions),
		TotalApplications:    len(applications),
		OutputDirectory:      cfg.OutputDir,
		Duration:             time.Since(startTime),
	}

	fmt.Println("\n====================================================")
	fmt.Printf("🎉 Pipeline complete in %s\n", result.Duration.Round(time.Millisecond))
	fmt.Printf("📁 Output stored in: %s\n", result.OutputDirectory)
	fmt.Println("====================================================")

	return result, nil
}
