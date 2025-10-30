package models

import "time"

// EvidenceSource represents the source of evidence
type EvidenceSource string

const (
	SourceManualUpload EvidenceSource = "manual_upload"
	SourceGmail        EvidenceSource = "gmail"
	SourceGoogleDrive  EvidenceSource = "google_drive"
	SourceGoogleCalendar EvidenceSource = "google_calendar"
	SourceExchange     EvidenceSource = "exchange"
	SourceOneDrive     EvidenceSource = "onedrive"
	SourceSlack        EvidenceSource = "slack"
)

// Evidence represents a piece of compliance evidence
type Evidence struct {
	ID             string         `firestore:"id" json:"id"`
	OrganizationID string         `firestore:"organization_id" json:"organization_id"`
	Title          string         `firestore:"title" json:"title"`
	Description    string         `firestore:"description,omitempty" json:"description,omitempty"`
	Source         EvidenceSource `firestore:"source" json:"source"`
	EvidenceDate   time.Time      `firestore:"evidence_date" json:"evidence_date"` // When the compliance activity occurred
	FileURL        string         `firestore:"file_url,omitempty" json:"file_url,omitempty"` // Cloud Storage path
	FileName       string         `firestore:"file_name,omitempty" json:"file_name,omitempty"`
	FileSize       int64          `firestore:"file_size,omitempty" json:"file_size,omitempty"`
	FileType       string         `firestore:"file_type,omitempty" json:"file_type,omitempty"`
	ExternalLink   string         `firestore:"external_link,omitempty" json:"external_link,omitempty"` // Link to source (Gmail, Drive, etc.)
	Metadata       map[string]interface{} `firestore:"metadata,omitempty" json:"metadata,omitempty"` // Additional metadata based on source
	RequirementIDs []string       `firestore:"requirement_ids" json:"requirement_ids"` // Associated requirements
	UploadedBy     string         `firestore:"uploaded_by" json:"uploaded_by"` // User UID
	CreatedAt      time.Time      `firestore:"created_at" json:"created_at"`
	UpdatedAt      time.Time      `firestore:"updated_at" json:"updated_at"`
	Status         string         `firestore:"status" json:"status"` // uploading, active, deleted
}

// EvidenceCaptureRule represents a rule for automatically capturing evidence
type EvidenceCaptureRule struct {
	ID             string         `firestore:"id" json:"id"`
	OrganizationID string         `firestore:"organization_id" json:"organization_id"`
	Name           string         `firestore:"name" json:"name"`
	Description    string         `firestore:"description,omitempty" json:"description,omitempty"`
	Source         EvidenceSource `firestore:"source" json:"source"`
	IsActive       bool           `firestore:"is_active" json:"is_active"`
	Conditions     RuleConditions `firestore:"conditions" json:"conditions"`
	RequirementIDs []string       `firestore:"requirement_ids,omitempty" json:"requirement_ids,omitempty"` // Auto-associate with these requirements
	CreatedBy      string         `firestore:"created_by" json:"created_by"`
	CreatedAt      time.Time      `firestore:"created_at" json:"created_at"`
	UpdatedAt      time.Time      `firestore:"updated_at" json:"updated_at"`
	LastCapturedAt *time.Time     `firestore:"last_captured_at,omitempty" json:"last_captured_at,omitempty"`
	CaptureCount   int            `firestore:"capture_count" json:"capture_count"`
}

// RuleConditions represents the conditions for evidence capture
type RuleConditions struct {
	// Gmail/Exchange conditions
	SenderEmail      string   `firestore:"sender_email,omitempty" json:"sender_email,omitempty"`
	SubjectKeywords  []string `firestore:"subject_keywords,omitempty" json:"subject_keywords,omitempty"`
	RecipientEmails  []string `firestore:"recipient_emails,omitempty" json:"recipient_emails,omitempty"`
	EmailLabel       string   `firestore:"email_label,omitempty" json:"email_label,omitempty"`
	EmailFolder      string   `firestore:"email_folder,omitempty" json:"email_folder,omitempty"`

	// Google Drive/OneDrive conditions
	FolderPath      string   `firestore:"folder_path,omitempty" json:"folder_path,omitempty"`
	FileNameKeywords []string `firestore:"file_name_keywords,omitempty" json:"file_name_keywords,omitempty"`
	FileTypes       []string `firestore:"file_types,omitempty" json:"file_types,omitempty"`

	// Google Calendar conditions
	CalendarName    string   `firestore:"calendar_name,omitempty" json:"calendar_name,omitempty"`
	EventTitleKeywords []string `firestore:"event_title_keywords,omitempty" json:"event_title_keywords,omitempty"`
	AttendeeEmails  []string `firestore:"attendee_emails,omitempty" json:"attendee_emails,omitempty"`

	// Slack conditions
	ChannelName     string   `firestore:"channel_name,omitempty" json:"channel_name,omitempty"`
	MessageKeywords []string `firestore:"message_keywords,omitempty" json:"message_keywords,omitempty"`
}

// Integration represents an OAuth integration with a third-party service
type Integration struct {
	ID              string         `firestore:"id" json:"id"`
	OrganizationID  string         `firestore:"organization_id" json:"organization_id"`
	Type            EvidenceSource `firestore:"type" json:"type"` // gmail, google_drive, exchange, etc.
	IsEnabled       bool           `firestore:"is_enabled" json:"is_enabled"`
	ConnectedBy     string         `firestore:"connected_by" json:"connected_by"` // User UID
	ConnectedEmail  string         `firestore:"connected_email" json:"connected_email"`
	// OAuth tokens stored encrypted (actual encryption handled at service layer)
	AccessToken     string    `firestore:"access_token" json:"-"` // Not exposed in JSON
	RefreshToken    string    `firestore:"refresh_token" json:"-"`
	TokenExpiry     time.Time `firestore:"token_expiry" json:"-"`
	LastPolledAt    *time.Time `firestore:"last_polled_at,omitempty" json:"last_polled_at,omitempty"`
	CreatedAt       time.Time `firestore:"created_at" json:"created_at"`
	UpdatedAt       time.Time `firestore:"updated_at" json:"updated_at"`
}
