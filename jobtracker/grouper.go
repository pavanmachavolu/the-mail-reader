package jobtracker

import (
	"sort"
	"strings"
	"time"
)

// GroupTransitions groups flat transitions into Applications by company, role, and unique identifier.
func GroupTransitions(transitions []*StateTransition) map[string]*Application {
	grouped := make(map[string]*Application)

	for _, t := range transitions {
		matchedApp := findMatchingApp(grouped, t)

		if matchedApp != nil {
			// Merge into existing application
			matchedApp.Timeline = append(matchedApp.Timeline, *t)

			// Upgrade generic role title to specific role if available
			if isGenericRoleName(matchedApp.Role, matchedApp.Company) && !isGenericRoleName(t.Role, t.Company) {
				matchedApp.Role = t.Role
				matchedApp.UniqueIdentifier = sanitizeSlug(t.Role)
			}

			if matchedApp.JobID == "" && t.JobID != "" {
				matchedApp.JobID = t.JobID
			}
			if matchedApp.Team == "" && t.Team != "" {
				matchedApp.Team = t.Team
			}
		} else {
			// Create a new application
			appKey := buildGroupKey(t.Company, t.Role, t.UniqueIdentifier)
			app := &Application{
				Company:          t.Company,
				Role:             t.Role,
				UniqueIdentifier: t.UniqueIdentifier,
				JobID:            t.JobID,
				Team:             t.Team,
				Timeline:         []StateTransition{*t},
			}
			grouped[appKey] = app
		}
	}

	// Post-process each application: sort chronologically and determine status
	for _, app := range grouped {
		sort.SliceStable(app.Timeline, func(i, j int) bool {
			if app.Timeline[i].Date.Equal(app.Timeline[j].Date) {
				return app.Timeline[i].EmailID < app.Timeline[j].EmailID
			}
			return app.Timeline[i].Date.Before(app.Timeline[j].Date)
		})

		if len(app.Timeline) > 0 {
			app.FirstAppliedDate = app.Timeline[0].Date
			app.LastUpdatedDate = app.Timeline[len(app.Timeline)-1].Date
			app.TotalEmails = len(app.Timeline)
			app.CurrentStatus = deriveCurrentStatus(app.Timeline)
		}
	}

	return grouped
}

func findMatchingApp(apps map[string]*Application, t *StateTransition) *Application {
	for _, app := range apps {
		if !strings.EqualFold(app.Company, t.Company) {
			continue
		}

		// 1. Exact Job ID match
		if app.JobID != "" && t.JobID != "" && strings.EqualFold(app.JobID, t.JobID) {
			return app
		}

		// 2. Similar role titles
		if !isGenericRoleName(app.Role, app.Company) && !isGenericRoleName(t.Role, t.Company) {
			if isSimilarRole(app.Role, t.Role) {
				return app
			}
		}

		// 3. One or both has a generic role / screening name within 90 days of application activity
		if isGenericRoleName(t.Role, t.Company) || isGenericRoleName(app.Role, app.Company) {
			if len(app.Timeline) > 0 {
				lastDate := app.Timeline[len(app.Timeline)-1].Date
				diff := t.Date.Sub(lastDate)
				if diff < 0 {
					diff = -diff
				}
				if diff < 90*24*time.Hour {
					return app
				}
			}
		}
	}
	return nil
}

func isSimilarRole(r1, r2 string) bool {
	s1 := strings.ToLower(strings.TrimSpace(r1))
	s2 := strings.ToLower(strings.TrimSpace(r2))
	if s1 == s2 || strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return true
	}
	// Check significant word overlap
	w1 := strings.Fields(s1)
	w2 := strings.Fields(s2)
	matches := 0
	for _, word1 := range w1 {
		if len(word1) <= 2 {
			continue
		}
		for _, word2 := range w2 {
			if word1 == word2 {
				matches++
				break
			}
		}
	}
	return matches >= 2
}

func buildGroupKey(company, role, id string) string {
	c := strings.ToLower(strings.TrimSpace(company))
	r := strings.ToLower(strings.TrimSpace(role))
	i := strings.ToLower(strings.TrimSpace(id))
	return c + "::" + r + "::" + i
}

func deriveCurrentStatus(timeline []StateTransition) string {
	if len(timeline) == 0 {
		return StatusCommunication
	}

	latest := timeline[len(timeline)-1].Status

	hasRejection := false
	hasOffer := false
	for _, t := range timeline {
		if t.Status == StatusRejected {
			hasRejection = true
		} else if t.Status == StatusOffer {
			hasOffer = true
		}
	}

	if hasOffer {
		return StatusOffer
	}
	if hasRejection {
		return StatusRejected
	}

	if latest != StatusCommunication {
		return latest
	}

	// If the latest event was generic COMMUNICATION, keep the most recent meaningful state
	for i := len(timeline) - 1; i >= 0; i-- {
		st := timeline[i].Status
		if st != StatusCommunication {
			return st
		}
	}

	return StatusCommunication
}
