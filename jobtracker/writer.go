package jobtracker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ApplicationsSummary provides high-level statistics across all applications.
type ApplicationsSummary struct {
	GeneratedAt       time.Time              `json:"generated_at"`
	TotalApplications int                    `json:"total_applications"`
	StatusCounts      map[string]int         `json:"status_counts"`
	Companies         []string               `json:"companies"`
	Applications      []ApplicationIndexItem `json:"applications"`
}

// ApplicationIndexItem holds summary metadata for an individual application.
type ApplicationIndexItem struct {
	Company          string    `json:"company"`
	Role             string    `json:"role"`
	JobID            string    `json:"job_id,omitempty"`
	CurrentStatus    string    `json:"current_status"`
	FirstAppliedDate time.Time `json:"first_applied_date"`
	LastUpdatedDate  time.Time `json:"last_updated_date"`
	TotalEmails      int       `json:"total_emails"`
	FilePath         string    `json:"file_path"`
}

// WriteApplications writes each application into an individual structured JSON file
// and produces a consolidated applications_summary.json.
func WriteApplications(applications map[string]*Application, outputDir string) ([]string, error) {
	if outputDir == "" || outputDir == "/" || outputDir == "." {
		return nil, fmt.Errorf("invalid output directory: %q", outputDir)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Remove all previously generated JSON files in output directory before writing
	existingEntries, err := os.ReadDir(outputDir)
	if err == nil {
		for _, entry := range existingEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				_ = os.Remove(filepath.Join(outputDir, entry.Name()))
			}
		}
	}

	writtenFiles := make([]string, 0, len(applications))
	statusCounts := make(map[string]int)
	companySet := make(map[string]struct{})
	var indexItems []ApplicationIndexItem

	// Sort keys for deterministic output
	var keys []string
	for k := range applications {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		app := applications[k]
		if app == nil || len(app.Timeline) == 0 {
			continue
		}

		filename := buildFilename(app.Company, app.Role, app.UniqueIdentifier)
		fullPath := filepath.Join(outputDir, filename)

		data, err := json.MarshalIndent(app, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal application JSON: %w", err)
		}

		if err := os.WriteFile(fullPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", fullPath, err)
		}

		writtenFiles = append(writtenFiles, fullPath)
		statusCounts[app.CurrentStatus]++
		companySet[app.Company] = struct{}{}

		relPath, _ := filepath.Rel(outputDir, fullPath)
		indexItems = append(indexItems, ApplicationIndexItem{
			Company:          app.Company,
			Role:             app.Role,
			JobID:            app.JobID,
			CurrentStatus:    app.CurrentStatus,
			FirstAppliedDate: app.FirstAppliedDate,
			LastUpdatedDate:  app.LastUpdatedDate,
			TotalEmails:      app.TotalEmails,
			FilePath:         relPath,
		})
	}

	var companies []string
	for c := range companySet {
		companies = append(companies, c)
	}
	sort.Strings(companies)

	// Write summary index file
	summary := ApplicationsSummary{
		GeneratedAt:       time.Now().UTC(),
		TotalApplications: len(indexItems),
		StatusCounts:      statusCounts,
		Companies:         companies,
		Applications:      indexItems,
	}

	summaryData, err := json.MarshalIndent(summary, "", "  ")
	if err == nil {
		summaryPath := filepath.Join(outputDir, "applications_summary.json")
		_ = os.WriteFile(summaryPath, summaryData, 0644)
	}

	return writtenFiles, nil
}

func buildFilename(company, role, id string) string {
	c := cleanFilenamePart(company)
	r := cleanFilenamePart(role)
	i := cleanFilenamePart(id)

	if c == "" {
		c = "unknown_company"
	}
	if r == "" {
		r = "role"
	}

	var name string
	if i != "" && i != r {
		name = fmt.Sprintf("%s_%s_%s.json", c, r, i)
	} else {
		name = fmt.Sprintf("%s_%s.json", c, r)
	}

	// Limit length if excessively long
	if len(name) > 120 {
		name = name[:115] + ".json"
	}
	return name
}

func cleanFilenamePart(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	reg := regexp.MustCompile(`[^a-z0-9_-]+`)
	s = reg.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}
