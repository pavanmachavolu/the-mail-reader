package jobtracker

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExtractDomain parses an email address string (e.g. "Jane <jane@acme.com>" or "jane@acme.com")
// and returns the clean lowercase domain portion ("acme.com").
func ExtractDomain(from string) string {
	from = strings.TrimSpace(from)
	if from == "" {
		return ""
	}

	// Try standard mail address parser first
	if addr, err := mail.ParseAddress(from); err == nil && addr.Address != "" {
		from = addr.Address
	}

	// Strip bracketed format if any remains
	if start := strings.Index(from, "<"); start != -1 {
		if end := strings.Index(from, ">"); end > start {
			from = from[start+1 : end]
		}
	}

	parts := strings.Split(from, "@")
	if len(parts) == 2 {
		domain := strings.ToLower(strings.TrimSpace(parts[1]))
		// Strip trailing punctuation
		domain = strings.Trim(domain, "> \t\r\n.")
		return domain
	}

	return ""
}

// ExcludedDomains lists domains that should be excluded from job tracking (e.g. schools, travel, utilities).
var ExcludedDomains = []string{
	"chirec.ac.in",
	"trenitalia.fr",
	"trenitalia.it",
	"trenitalia.com",
	"trenitalia",
}

// IsExcludedDomain returns true if the domain matches, is a subdomain, or matches an excluded keyword.
func IsExcludedDomain(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	for _, excl := range ExcludedDomains {
		excl = strings.ToLower(strings.TrimSpace(excl))
		if domain == excl || strings.HasSuffix(domain, "."+excl) {
			return true
		}
		if !strings.Contains(excl, ".") && strings.Contains(domain, excl) {
			return true
		}
	}
	return false
}

// ExtractUniqueDomains scans all JSON files in the given directory and collects unique sender domains.
func ExtractUniqueDomains(emailsDir string) ([]string, error) {
	entries, err := os.ReadDir(emailsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read emails directory %s: %w", emailsDir, err)
	}

	domainSet := make(map[string]struct{})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(emailsDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var msg EmailMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		domain := ExtractDomain(msg.From)
		if domain != "" && strings.Contains(domain, ".") && !IsExcludedDomain(domain) {
			domainSet[domain] = struct{}{}
		}
	}

	uniqueDomains := make([]string, 0, len(domainSet))
	for d := range domainSet {
		uniqueDomains = append(uniqueDomains, d)
	}
	sort.Strings(uniqueDomains)

	return uniqueDomains, nil
}
