package models

import "time"

// AuditAction represents the type of action performed
type AuditAction string

const (
	ActionLogin              AuditAction = "login"
	ActionLogout             AuditAction = "logout"
	ActionUserCreated        AuditAction = "user_created"
	ActionUserUpdated        AuditAction = "user_updated"
	ActionUserDeleted        AuditAction = "user_deleted"
	ActionOrgCreated         AuditAction = "organization_created"
	ActionOrgUpdated         AuditAction = "organization_updated"
	ActionRequirementActivated AuditAction = "requirement_activated"
	ActionRequirementUpdated AuditAction = "requirement_updated"
	ActionRequirementDeactivated AuditAction = "requirement_deactivated"
	ActionEvidenceCreated    AuditAction = "evidence_created"
	ActionEvidenceUpdated    AuditAction = "evidence_updated"
	ActionEvidenceDeleted    AuditAction = "evidence_deleted"
	ActionEvidenceViewed     AuditAction = "evidence_viewed"
	ActionEvidenceDownloaded AuditAction = "evidence_downloaded"
	ActionReportGenerated    AuditAction = "report_generated"
	ActionIntegrationConnected AuditAction = "integration_connected"
	ActionIntegrationDisconnected AuditAction = "integration_disconnected"
	ActionSubscriptionUpdated AuditAction = "subscription_updated"
	ActionPaymentMethodUpdated AuditAction = "payment_method_updated"
)

// AuditLog represents an immutable audit log entry
type AuditLog struct {
	ID             string      `firestore:"id" json:"id"`
	OrganizationID string      `firestore:"organization_id" json:"organization_id"`
	Timestamp      time.Time   `firestore:"timestamp" json:"timestamp"` // Server timestamp
	UserID         string      `firestore:"user_id" json:"user_id"`
	UserEmail      string      `firestore:"user_email" json:"user_email"`
	Action         AuditAction `firestore:"action" json:"action"`
	ResourceType   string      `firestore:"resource_type" json:"resource_type"` // requirement, evidence, user, etc.
	ResourceID     string      `firestore:"resource_id,omitempty" json:"resource_id,omitempty"`
	Description    string      `firestore:"description" json:"description"`
	Changes        map[string]interface{} `firestore:"changes,omitempty" json:"changes,omitempty"` // JSON diff of changes
	IPAddress      string      `firestore:"ip_address,omitempty" json:"ip_address,omitempty"`
	UserAgent      string      `firestore:"user_agent,omitempty" json:"user_agent,omitempty"`
	Metadata       map[string]interface{} `firestore:"metadata,omitempty" json:"metadata,omitempty"`
}

// Report represents a generated compliance report
type Report struct {
	ID             string    `firestore:"id" json:"id"`
	OrganizationID string    `firestore:"organization_id" json:"organization_id"`
	Title          string    `firestore:"title" json:"title"`
	Description    string    `firestore:"description,omitempty" json:"description,omitempty"`
	Type           string    `firestore:"type" json:"type"` // requirement_detail, comprehensive
	RequirementIDs []string  `firestore:"requirement_ids" json:"requirement_ids"`
	Status         string    `firestore:"status" json:"status"` // pending, generating, completed, failed
	FileURL        string    `firestore:"file_url,omitempty" json:"file_url,omitempty"` // Cloud Storage path
	GeneratedBy    string    `firestore:"generated_by" json:"generated_by"`
	CreatedAt      time.Time `firestore:"created_at" json:"created_at"`
	CompletedAt    *time.Time `firestore:"completed_at,omitempty" json:"completed_at,omitempty"`
	ErrorMessage   string    `firestore:"error_message,omitempty" json:"error_message,omitempty"`
}
