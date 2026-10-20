package jobtracker

import (
	"time"
)

// EmailMessage represents the JSON structure of stored emails.
type EmailMessage struct {
	ID            string    `json:"id"`
	Subject       string    `json:"subject"`
	From          string    `json:"from"`
	To            []string  `json:"to"`
	ReceivedAt    time.Time `json:"received_at"`
	BodyText      string    `json:"body_text,omitempty"`
	BodyMarkdown  string    `json:"body_markdown,omitempty"`
	HasAttachment bool      `json:"has_attachment"`
	Folder        string    `json:"folder"`
	Read          bool      `json:"read"`
	Importance    string    `json:"importance"`
}

// JobStatus constants representing application states.
const (
	StatusReferred      = "REFERRED"
	StatusApplied       = "APPLIED"
	StatusUnderReview   = "UNDER_REVIEW"
	StatusInterview     = "INTERVIEW"
	StatusAssessment    = "ASSESSMENT"
	StatusOffer         = "OFFER"
	StatusRejected      = "REJECTED"
	StatusWithdrawn     = "WITHDRAWN"
	StatusCommunication = "COMMUNICATION"
)

// StateTransition records a single status change or communication event for an application.
type StateTransition struct {
	Date             time.Time `json:"date"`
	Status           string    `json:"status"`
	Subject          string    `json:"subject"`
	EmailID          string    `json:"email_id"`
	From             string    `json:"from"`
	Company          string    `json:"company"`
	Role             string    `json:"role,omitempty"`
	JobID            string    `json:"job_id,omitempty"`
	Team             string    `json:"team,omitempty"`
	UniqueIdentifier string    `json:"unique_identifier,omitempty"`
	Snippet          string    `json:"snippet,omitempty"`
}

// Application represents the consolidated chronological history of a job application.
type Application struct {
	Company          string            `json:"company"`
	Role             string            `json:"role"`
	UniqueIdentifier string            `json:"unique_identifier"`
	JobID            string            `json:"job_id,omitempty"`
	Team             string            `json:"team,omitempty"`
	CurrentStatus    string            `json:"current_status"`
	FirstAppliedDate time.Time         `json:"first_applied_date"`
	LastUpdatedDate  time.Time         `json:"last_updated_date"`
	TotalEmails      int               `json:"total_emails"`
	Timeline         []StateTransition `json:"timeline"`
}

// Config specifies the runtime settings for the job tracker pipeline.
type Config struct {
	EmailsDir   string
	OutputDir   string
	CacheFile   string
	ModelAPIKey string
	ModelURL    string
	ModelName   string
}
