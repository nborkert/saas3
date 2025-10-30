package models

import "time"

// RequirementCategory represents the category of a regulatory requirement
type RequirementCategory string

const (
	CategoryEmployeeTraining RequirementCategory = "employee_training"
	CategoryPolicyManagement RequirementCategory = "policy_management"
	CategoryAccessControls   RequirementCategory = "access_controls"
	CategoryRecordkeeping    RequirementCategory = "recordkeeping"
	CategoryLicensing        RequirementCategory = "licensing"
	CategoryConsumerProtection RequirementCategory = "consumer_protection"
	CategoryBusinessPractices RequirementCategory = "business_practices"
	CategoryPrivacySecurity  RequirementCategory = "privacy_security"
	CategoryRiskManagement   RequirementCategory = "risk_management"
	CategoryBusinessAssociates RequirementCategory = "business_associates"
	CategoryPatientRights    RequirementCategory = "patient_rights"
)

// RequirementFrequency represents how often a requirement needs to be satisfied
type RequirementFrequency string

const (
	FrequencyAnnual    RequirementFrequency = "annual"
	FrequencyQuarterly RequirementFrequency = "quarterly"
	FrequencyMonthly   RequirementFrequency = "monthly"
	FrequencyOngoing   RequirementFrequency = "ongoing"
	FrequencyOneTime   RequirementFrequency = "one_time"
)

// RequirementStatus represents the compliance status of a requirement
type RequirementStatus string

const (
	StatusNotStarted  RequirementStatus = "not_started"
	StatusInProgress  RequirementStatus = "in_progress"
	StatusCompliant   RequirementStatus = "compliant"
	StatusAtRisk      RequirementStatus = "at_risk"
	StatusNonCompliant RequirementStatus = "non_compliant"
)

// RequirementTemplate represents a pre-built regulatory requirement template
type RequirementTemplate struct {
	ID                  string               `firestore:"id" json:"id"`
	Title               string               `firestore:"title" json:"title"`
	Description         string               `firestore:"description" json:"description"`
	Category            RequirementCategory  `firestore:"category" json:"category"`
	RegulatoryFramework RegulatoryFramework  `firestore:"regulatory_framework" json:"regulatory_framework"`
	Authority           string               `firestore:"authority" json:"authority"` // e.g., "SEC Rule 206(4)-7"
	EvidenceTypes       []string             `firestore:"evidence_types" json:"evidence_types"`
	Frequency           RequirementFrequency `firestore:"frequency" json:"frequency"`
	IsActive            bool                 `firestore:"is_active" json:"is_active"`
	CreatedAt           time.Time            `firestore:"created_at" json:"created_at"`
	UpdatedAt           time.Time            `firestore:"updated_at" json:"updated_at"`
}

// Requirement represents an activated requirement for an organization
type Requirement struct {
	ID                  string               `firestore:"id" json:"id"`
	OrganizationID      string               `firestore:"organization_id" json:"organization_id"`
	TemplateID          string               `firestore:"template_id" json:"template_id"`
	Title               string               `firestore:"title" json:"title"`
	Description         string               `firestore:"description" json:"description"`
	Category            RequirementCategory  `firestore:"category" json:"category"`
	Authority           string               `firestore:"authority" json:"authority"`
	EvidenceTypes       []string             `firestore:"evidence_types" json:"evidence_types"`
	Frequency           RequirementFrequency `firestore:"frequency" json:"frequency"`
	Status              RequirementStatus    `firestore:"status" json:"status"`
	NextDueDate         *time.Time           `firestore:"next_due_date,omitempty" json:"next_due_date,omitempty"`
	LastCompletedDate   *time.Time           `firestore:"last_completed_date,omitempty" json:"last_completed_date,omitempty"`
	EvidenceCount       int                  `firestore:"evidence_count" json:"evidence_count"`
	Notes               string               `firestore:"notes,omitempty" json:"notes,omitempty"`
	ActivatedAt         time.Time            `firestore:"activated_at" json:"activated_at"`
	ActivatedBy         string               `firestore:"activated_by" json:"activated_by"`
	UpdatedAt           time.Time            `firestore:"updated_at" json:"updated_at"`
	UpdatedBy           string               `firestore:"updated_by,omitempty" json:"updated_by,omitempty"`
	IsActive            bool                 `firestore:"is_active" json:"is_active"`
}

// CalculateStatus determines the compliance status based on evidence and due dates
func (r *Requirement) CalculateStatus() RequirementStatus {
	if r.EvidenceCount == 0 {
		return StatusNotStarted
	}

	if r.NextDueDate == nil {
		// Ongoing requirements
		if r.EvidenceCount > 0 {
			return StatusCompliant
		}
		return StatusNotStarted
	}

	now := time.Now()
	daysUntilDue := int(r.NextDueDate.Sub(now).Hours() / 24)

	if daysUntilDue < 0 {
		// Past due
		return StatusNonCompliant
	} else if daysUntilDue <= 7 {
		// Due within 7 days
		return StatusAtRisk
	} else if r.EvidenceCount > 0 {
		return StatusCompliant
	}

	return StatusInProgress
}
