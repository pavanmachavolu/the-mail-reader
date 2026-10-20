package jobtracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Enricher is the interface for mapping domain names to company names.
type Enricher interface {
	Enrich(ctx context.Context, domains []string) (map[string]string, error)
}

// CompositeEnricher coordinates persistent caching, model enrichment, and heuristic fallbacks.
type CompositeEnricher struct {
	cachePath string
	cache     map[string]string
	apiKey    string
	modelURL  string
	modelName string
	client    *http.Client
}

// NewEnricher creates an enricher instance with caching and model capabilities.
func NewEnricher(cachePath, apiKey, modelURL, modelName string) *CompositeEnricher {
	if cachePath == "" {
		home, _ := os.UserHomeDir()
		cachePath = filepath.Join(home, ".the-mail-reader", "company_domains_cache.json")
	}

	if apiKey == "" {
		if k := os.Getenv("OPENAI_API_KEY"); k != "" {
			apiKey = k
		} else if k := os.Getenv("GEMINI_API_KEY"); k != "" {
			apiKey = k
		} else if k := os.Getenv("MODEL_API_KEY"); k != "" {
			apiKey = k
		}
	}

	if modelName == "" {
		if strings.HasPrefix(apiKey, "AIza") {
			modelName = "gemini-1.5-flash"
		} else {
			modelName = "gpt-4o-mini"
		}
	}

	e := &CompositeEnricher{
		cachePath: cachePath,
		cache:     make(map[string]string),
		apiKey:    apiKey,
		modelURL:  modelURL,
		modelName: modelName,
		client:    &http.Client{Timeout: 60 * time.Second},
	}

	e.loadCache()
	return e
}

func (e *CompositeEnricher) loadCache() {
	data, err := os.ReadFile(e.cachePath)
	if err == nil {
		_ = json.Unmarshal(data, &e.cache)
	}
}

func (e *CompositeEnricher) saveCache() {
	if len(e.cache) == 0 {
		return
	}
	_ = os.MkdirAll(filepath.Dir(e.cachePath), 0755)
	data, err := json.MarshalIndent(e.cache, "", "  ")
	if err == nil {
		_ = os.WriteFile(e.cachePath, data, 0644)
	}
}

// Enrich returns a mapping of domain -> company name for all provided domains.
func (e *CompositeEnricher) Enrich(ctx context.Context, domains []string) (map[string]string, error) {
	result := make(map[string]string)
	var missing []string

	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if company, ok := e.cache[d]; ok && company != "" {
			result[d] = company
		} else {
			missing = append(missing, d)
		}
	}

	if len(missing) == 0 {
		return result, nil
	}

	// If an LLM API key is available, query the model
	if e.apiKey != "" {
		fmt.Printf("Enriching %d domains using model %s...\n", len(missing), e.modelName)
		modelResults, err := e.queryModelBatches(ctx, missing)
		if err != nil {
			fmt.Printf("Warning: Model enrichment encountered error (%v). Falling back to heuristic enrichment.\n", err)
		} else {
			for d, company := range modelResults {
				if company != "" {
					result[d] = company
					e.cache[d] = company
				}
			}
		}
	}

	// For any remaining domains not resolved by the model, use domain heuristics
	for _, d := range missing {
		if _, ok := result[d]; !ok {
			company := HeuristicCompanyName(d)
			result[d] = company
			e.cache[d] = company
		}
	}

	e.saveCache()
	return result, nil
}

func (e *CompositeEnricher) queryModelBatches(ctx context.Context, domains []string) (map[string]string, error) {
	enriched := make(map[string]string)
	batchSize := 40

	for i := 0; i < len(domains); i += batchSize {
		end := i + batchSize
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]

		batchMap, err := e.queryModelSingleBatch(ctx, batch)
		if err != nil {
			return enriched, err
		}
		for k, v := range batchMap {
			enriched[k] = v
		}
	}

	return enriched, nil
}

func (e *CompositeEnricher) queryModelSingleBatch(ctx context.Context, domains []string) (map[string]string, error) {
	prompt := fmt.Sprintf(`You are a data enrichment assistant. Given the following email domains, determine the real company name associated with each domain.
Account for applicant tracking systems (e.g. "picnic.greenhouse.io" is "Picnic", "myworkday.com" might be general Workday or company subdomain), corporate subsidiaries, or company websites.
Return ONLY a valid JSON object mapping domain to company name. Do not include markdown code block formatting or explanation.

Domains:
%s`, strings.Join(domains, "\n"))

	// Check if this is a Gemini API key
	if strings.HasPrefix(e.apiKey, "AIza") {
		return e.queryGemini(ctx, prompt)
	}

	// Default: OpenAI or OpenAI-compatible endpoint
	return e.queryOpenAI(ctx, prompt)
}

func (e *CompositeEnricher) queryOpenAI(ctx context.Context, prompt string) (map[string]string, error) {
	url := "https://api.openai.com/v1/chat/completions"
	if e.modelURL != "" {
		url = e.modelURL
	}

	reqBody := map[string]interface{}{
		"model": e.modelName,
		"messages": []map[string]string{
			{"role": "system", "content": "You map internet domains to their real company names. Output raw JSON object only."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.0,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model API returned %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &res); err != nil || len(res.Choices) == 0 {
		return nil, fmt.Errorf("failed to parse model response: %w", err)
	}

	return parseJSONMapping(res.Choices[0].Message.Content)
}

func (e *CompositeEnricher) queryGemini(ctx context.Context, prompt string) (map[string]string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", e.modelName, e.apiKey)
	if e.modelURL != "" {
		url = e.modelURL
	}

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API returned %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &res); err != nil || len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	return parseJSONMapping(res.Candidates[0].Content.Parts[0].Text)
}

func parseJSONMapping(content string) (map[string]string, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var mapping map[string]string
	if err := json.Unmarshal([]byte(content), &mapping); err != nil {
		return nil, err
	}
	return mapping, nil
}

// HeuristicCompanyName infers a human-readable company name from a domain name.
func HeuristicCompanyName(domain string) string {
	d := strings.ToLower(domain)

	// Known specialized mappings
	known := map[string]string{
		"deshaw.com":               "D. E. Shaw",
		"hyd.deshaw.com":           "D. E. Shaw",
		"marshmallow.co":           "Marshmallow",
		"lockwood-publishing.com":  "Lockwood Publishing",
		"amazon.com":               "Amazon",
		"apple.com":                "Apple",
		"google.com":               "Google",
		"microsoft.com":            "Microsoft",
		"stripe.com":               "Stripe",
		"anthropic.com":            "Anthropic",
		"openai.com":               "OpenAI",
		"linkedin.com":             "LinkedIn",
		"indeed.com":               "Indeed",
		"greenhouse.io":            "Greenhouse ATS",
		"greenhouse-mail.io":       "Greenhouse ATS",
		"greenhouse-jobs.com":      "Greenhouse ATS",
		"lever.co":                 "Lever ATS",
		"myworkday.com":            "Workday ATS",
		"smartrecruiters.com":      "SmartRecruiters ATS",
		"ashbyhq.com":              "Ashby ATS",
		"workable.com":             "Workable ATS",
		"workablemail.com":         "Workable ATS",
		"breezy-mail.com":          "Breezy ATS",
		"breezy.hr":                "Breezy ATS",
		"comeet-notifications.com": "Comeet ATS",
		"recruitee.com":            "Recruitee ATS",
		"recruiterbox.com":         "Recruiterbox ATS",
		"applytojob.com":           "ApplyToJob ATS",
		"taleo.com":                "Taleo ATS",
		"taleo.net":                "Taleo ATS",
		"teamtailor.com":           "Teamtailor ATS",
		"teamtailor-mail.com":      "Teamtailor ATS",
		"customer.io":              "Customer.io",
	}

	if name, ok := known[d]; ok {
		return name
	}

	// Handle ATS subdomains e.g. <company>.greenhouse.io or <company>.lever.co
	for _, ats := range []string{".greenhouse.io", ".lever.co", ".ashbyhq.com", ".workable.com", ".myworkday.com"} {
		if strings.HasSuffix(d, ats) {
			sub := strings.TrimSuffix(d, ats)
			parts := strings.Split(sub, ".")
			return formatTitle(parts[len(parts)-1])
		}
	}

	// Strip common top-level and second-level domains (.com, .co.uk, .io, .org, .ai, .net, etc.)
	parts := strings.Split(d, ".")
	if len(parts) >= 2 {
		base := parts[len(parts)-2]
		// If base is something like "co" in "example.co.uk", look back one more
		if (base == "co" || base == "com" || base == "org" || base == "net") && len(parts) >= 3 {
			base = parts[len(parts)-3]
		}
		return formatTitle(base)
	}

	return formatTitle(d)
}

func formatTitle(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}
