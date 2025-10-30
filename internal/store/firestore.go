package store

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"compliancesync-api/internal/models"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
)

// FirestoreStore implements the Store interface using Firestore
type FirestoreStore struct {
	client *firestore.Client
}

// NewFirestoreStore creates a new Firestore store
func NewFirestoreStore(ctx context.Context, projectID string) (*FirestoreStore, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}

	return &FirestoreStore{client: client}, nil
}

// Close closes the Firestore client
func (s *FirestoreStore) Close() error {
	return s.client.Close()
}

// Organization methods

// CreateOrganization creates a new organization
func (s *FirestoreStore) CreateOrganization(ctx context.Context, org *models.Organization) error {
	org.ID = uuid.New().String()
	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()
	org.ActiveUserCount = 1 // Creator is the first user

	_, err := s.client.Collection("organizations").Doc(org.ID).Set(ctx, org)
	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	return nil
}

// GetOrganization retrieves an organization by ID
func (s *FirestoreStore) GetOrganization(ctx context.Context, orgID string) (*models.Organization, error) {
	doc, err := s.client.Collection("organizations").Doc(orgID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	var org models.Organization
	if err := doc.DataTo(&org); err != nil {
		return nil, fmt.Errorf("failed to parse organization: %w", err)
	}

	return &org, nil
}

// UpdateOrganization updates an organization
func (s *FirestoreStore) UpdateOrganization(ctx context.Context, org *models.Organization) error {
	org.UpdatedAt = time.Now()

	_, err := s.client.Collection("organizations").Doc(org.ID).Set(ctx, org)
	if err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	return nil
}

// User methods

// CreateUser creates a new user
func (s *FirestoreStore) CreateUser(ctx context.Context, user *models.User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	user.Status = "active"

	_, err := s.client.Collection("users").Doc(user.UID).Set(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUser retrieves a user by UID
func (s *FirestoreStore) GetUser(ctx context.Context, uid string) (*models.User, error) {
	doc, err := s.client.Collection("users").Doc(uid).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user: %w", err)
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (s *FirestoreStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	iter := s.client.Collection("users").Where("email", "==", email).Limit(1).Documents(ctx)
	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user: %w", err)
	}

	return &user, nil
}

// UpdateUser updates a user
func (s *FirestoreStore) UpdateUser(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()

	_, err := s.client.Collection("users").Doc(user.UID).Set(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// ListUsersByOrganization lists all users in an organization
func (s *FirestoreStore) ListUsersByOrganization(ctx context.Context, orgID string) ([]*models.User, error) {
	iter := s.client.Collection("users").Where("organization_id", "==", orgID).Documents(ctx)

	var users []*models.User
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate users: %w", err)
		}

		var user models.User
		if err := doc.DataTo(&user); err != nil {
			return nil, fmt.Errorf("failed to parse user: %w", err)
		}
		users = append(users, &user)
	}

	return users, nil
}

// UpdateLastLogin updates the last login timestamp for a user
func (s *FirestoreStore) UpdateLastLogin(ctx context.Context, uid string) error {
	now := time.Now()
	_, err := s.client.Collection("users").Doc(uid).Update(ctx, []firestore.Update{
		{Path: "last_login_at", Value: now},
		{Path: "updated_at", Value: now},
	})
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// Requirement methods

// CreateRequirement creates a new requirement for an organization
func (s *FirestoreStore) CreateRequirement(ctx context.Context, req *models.Requirement) error {
	req.ID = uuid.New().String()
	req.ActivatedAt = time.Now()
	req.UpdatedAt = time.Now()
	req.Status = models.StatusNotStarted
	req.EvidenceCount = 0
	req.IsActive = true

	_, err := s.client.Collection("organizations").Doc(req.OrganizationID).
		Collection("requirements").Doc(req.ID).Set(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create requirement: %w", err)
	}

	return nil
}

// GetRequirement retrieves a requirement by ID
func (s *FirestoreStore) GetRequirement(ctx context.Context, orgID, reqID string) (*models.Requirement, error) {
	doc, err := s.client.Collection("organizations").Doc(orgID).
		Collection("requirements").Doc(reqID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get requirement: %w", err)
	}

	var req models.Requirement
	if err := doc.DataTo(&req); err != nil {
		return nil, fmt.Errorf("failed to parse requirement: %w", err)
	}

	return &req, nil
}

// ListRequirements lists all active requirements for an organization
func (s *FirestoreStore) ListRequirements(ctx context.Context, orgID string) ([]*models.Requirement, error) {
	iter := s.client.Collection("organizations").Doc(orgID).
		Collection("requirements").Where("is_active", "==", true).Documents(ctx)

	var requirements []*models.Requirement
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate requirements: %w", err)
		}

		var req models.Requirement
		if err := doc.DataTo(&req); err != nil {
			return nil, fmt.Errorf("failed to parse requirement: %w", err)
		}
		requirements = append(requirements, &req)
	}

	return requirements, nil
}

// UpdateRequirement updates a requirement
func (s *FirestoreStore) UpdateRequirement(ctx context.Context, req *models.Requirement) error {
	req.UpdatedAt = time.Now()

	_, err := s.client.Collection("organizations").Doc(req.OrganizationID).
		Collection("requirements").Doc(req.ID).Set(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update requirement: %w", err)
	}

	return nil
}

// Evidence methods

// CreateEvidence creates a new evidence item
func (s *FirestoreStore) CreateEvidence(ctx context.Context, evidence *models.Evidence) error {
	evidence.ID = uuid.New().String()
	evidence.CreatedAt = time.Now()
	evidence.UpdatedAt = time.Now()

	_, err := s.client.Collection("organizations").Doc(evidence.OrganizationID).
		Collection("evidence").Doc(evidence.ID).Set(ctx, evidence)
	if err != nil {
		return fmt.Errorf("failed to create evidence: %w", err)
	}

	// Update evidence count for associated requirements
	for _, reqID := range evidence.RequirementIDs {
		if err := s.incrementRequirementEvidenceCount(ctx, evidence.OrganizationID, reqID); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("failed to increment evidence count for requirement %s: %v\n", reqID, err)
		}
	}

	return nil
}

// GetEvidence retrieves an evidence item by ID
func (s *FirestoreStore) GetEvidence(ctx context.Context, orgID, evidenceID string) (*models.Evidence, error) {
	doc, err := s.client.Collection("organizations").Doc(orgID).
		Collection("evidence").Doc(evidenceID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get evidence: %w", err)
	}

	var evidence models.Evidence
	if err := doc.DataTo(&evidence); err != nil {
		return nil, fmt.Errorf("failed to parse evidence: %w", err)
	}

	return &evidence, nil
}

// ListEvidence lists all evidence for an organization
func (s *FirestoreStore) ListEvidence(ctx context.Context, orgID string, filters map[string]interface{}) ([]*models.Evidence, error) {
	query := s.client.Collection("organizations").Doc(orgID).Collection("evidence").
		Where("status", "==", "active")

	// Apply additional filters
	for key, value := range filters {
		query = query.Where(key, "==", value)
	}

	iter := query.Documents(ctx)

	var evidenceList []*models.Evidence
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate evidence: %w", err)
		}

		var evidence models.Evidence
		if err := doc.DataTo(&evidence); err != nil {
			return nil, fmt.Errorf("failed to parse evidence: %w", err)
		}
		evidenceList = append(evidenceList, &evidence)
	}

	return evidenceList, nil
}

// UpdateEvidence updates an evidence item
func (s *FirestoreStore) UpdateEvidence(ctx context.Context, evidence *models.Evidence) error {
	evidence.UpdatedAt = time.Now()

	_, err := s.client.Collection("organizations").Doc(evidence.OrganizationID).
		Collection("evidence").Doc(evidence.ID).Set(ctx, evidence)
	if err != nil {
		return fmt.Errorf("failed to update evidence: %w", err)
	}

	return nil
}

// DeleteEvidence soft deletes an evidence item
func (s *FirestoreStore) DeleteEvidence(ctx context.Context, orgID, evidenceID string) error {
	// Get the evidence first to update requirement counts
	evidence, err := s.GetEvidence(ctx, orgID, evidenceID)
	if err != nil {
		return err
	}

	// Update status to deleted
	_, err = s.client.Collection("organizations").Doc(orgID).
		Collection("evidence").Doc(evidenceID).Update(ctx, []firestore.Update{
			{Path: "status", Value: "deleted"},
			{Path: "updated_at", Value: time.Now()},
		})
	if err != nil {
		return fmt.Errorf("failed to delete evidence: %w", err)
	}

	// Decrement evidence count for associated requirements
	for _, reqID := range evidence.RequirementIDs {
		if err := s.decrementRequirementEvidenceCount(ctx, orgID, reqID); err != nil {
			fmt.Printf("failed to decrement evidence count for requirement %s: %v\n", reqID, err)
		}
	}

	return nil
}

// Audit log methods

// CreateAuditLog creates a new audit log entry
func (s *FirestoreStore) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	log.ID = uuid.New().String()
	log.Timestamp = time.Now()

	_, err := s.client.Collection("organizations").Doc(log.OrganizationID).
		Collection("audit_logs").Doc(log.ID).Set(ctx, log)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// ListAuditLogs lists audit logs for an organization with optional filters
func (s *FirestoreStore) ListAuditLogs(ctx context.Context, orgID string, filters map[string]interface{}, limit int) ([]*models.AuditLog, error) {
	query := s.client.Collection("organizations").Doc(orgID).
		Collection("audit_logs").OrderBy("timestamp", firestore.Desc)

	if limit > 0 {
		query = query.Limit(limit)
	}

	// Apply filters
	for key, value := range filters {
		query = query.Where(key, "==", value)
	}

	iter := query.Documents(ctx)

	var logs []*models.AuditLog
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate audit logs: %w", err)
		}

		var log models.AuditLog
		if err := doc.DataTo(&log); err != nil {
			return nil, fmt.Errorf("failed to parse audit log: %w", err)
		}
		logs = append(logs, &log)
	}

	return logs, nil
}

// Report methods

// CreateReport creates a new report
func (s *FirestoreStore) CreateReport(ctx context.Context, report *models.Report) error {
	report.ID = uuid.New().String()
	report.CreatedAt = time.Now()

	_, err := s.client.Collection("organizations").Doc(report.OrganizationID).
		Collection("reports").Doc(report.ID).Set(ctx, report)
	if err != nil {
		return fmt.Errorf("failed to create report: %w", err)
	}

	return nil
}

// GetReport retrieves a report by ID
func (s *FirestoreStore) GetReport(ctx context.Context, orgID, reportID string) (*models.Report, error) {
	doc, err := s.client.Collection("organizations").Doc(orgID).
		Collection("reports").Doc(reportID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get report: %w", err)
	}

	var report models.Report
	if err := doc.DataTo(&report); err != nil {
		return nil, fmt.Errorf("failed to parse report: %w", err)
	}

	return &report, nil
}

// UpdateReport updates a report
func (s *FirestoreStore) UpdateReport(ctx context.Context, report *models.Report) error {
	_, err := s.client.Collection("organizations").Doc(report.OrganizationID).
		Collection("reports").Doc(report.ID).Set(ctx, report)
	if err != nil {
		return fmt.Errorf("failed to update report: %w", err)
	}

	return nil
}

// GetRequirementTemplate retrieves a requirement template by ID
func (s *FirestoreStore) GetRequirementTemplate(ctx context.Context, templateID string) (*models.RequirementTemplate, error) {
	doc, err := s.client.Collection("requirement_templates").Doc(templateID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get requirement template: %w", err)
	}

	var template models.RequirementTemplate
	if err := doc.DataTo(&template); err != nil {
		return nil, fmt.Errorf("failed to parse requirement template: %w", err)
	}

	return &template, nil
}

// ListRequirementTemplates lists requirement templates by framework
func (s *FirestoreStore) ListRequirementTemplates(ctx context.Context, framework models.RegulatoryFramework) ([]*models.RequirementTemplate, error) {
	query := s.client.Collection("requirement_templates").
		Where("regulatory_framework", "==", framework).
		Where("is_active", "==", true)

	iter := query.Documents(ctx)

	var templates []*models.RequirementTemplate
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate templates: %w", err)
		}

		var template models.RequirementTemplate
		if err := doc.DataTo(&template); err != nil {
			return nil, fmt.Errorf("failed to parse template: %w", err)
		}
		templates = append(templates, &template)
	}

	return templates, nil
}

// Helper methods

func (s *FirestoreStore) incrementRequirementEvidenceCount(ctx context.Context, orgID, reqID string) error {
	_, err := s.client.Collection("organizations").Doc(orgID).
		Collection("requirements").Doc(reqID).Update(ctx, []firestore.Update{
			{Path: "evidence_count", Value: firestore.Increment(1)},
			{Path: "updated_at", Value: time.Now()},
		})
	return err
}

func (s *FirestoreStore) decrementRequirementEvidenceCount(ctx context.Context, orgID, reqID string) error {
	_, err := s.client.Collection("organizations").Doc(orgID).
		Collection("requirements").Doc(reqID).Update(ctx, []firestore.Update{
			{Path: "evidence_count", Value: firestore.Increment(-1)},
			{Path: "updated_at", Value: time.Now()},
		})
	return err
}
