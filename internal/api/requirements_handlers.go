package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"compliancesync-api/internal/auth"
	"compliancesync-api/internal/models"
	"github.com/go-chi/chi/v5"
)

// handleListRequirementTemplates implements STORY-006: View Pre-Built Regulatory Requirement Templates
func (s *Server) handleListRequirementTemplates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// Get organization to determine regulatory framework
		org, err := s.store.GetOrganization(r.Context(), claims.OrganizationID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to get organization")
			return
		}

		// Get templates for the organization's regulatory framework
		templates, err := s.store.ListRequirementTemplates(r.Context(), org.RegulatoryFramework)
		if err != nil {
			s.logger.Error("failed to list requirement templates", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to get requirement templates")
			return
		}

		respondJSON(w, http.StatusOK, templates)
	}
}

// handleCreateRequirement implements STORY-007: Activate Regulatory Requirements for My Organization
func (s *Server) handleCreateRequirement() http.HandlerFunc {
	type request struct {
		TemplateID string `json:"template_id"`
		Notes      string `json:"notes"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Get the template
		template, err := s.store.GetRequirementTemplate(r.Context(), req.TemplateID)
		if err != nil {
			respondError(w, http.StatusNotFound, "requirement template not found")
			return
		}

		// Create requirement from template
		requirement := &models.Requirement{
			OrganizationID: claims.OrganizationID,
			TemplateID:     template.ID,
			Title:          template.Title,
			Description:    template.Description,
			Category:       template.Category,
			Authority:      template.Authority,
			EvidenceTypes:  template.EvidenceTypes,
			Frequency:      template.Frequency,
			Notes:          req.Notes,
			ActivatedBy:    claims.UID,
		}

		if err := s.store.CreateRequirement(r.Context(), requirement); err != nil {
			s.logger.Error("failed to create requirement", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to activate requirement")
			return
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionRequirementActivated,
			ResourceType:   "requirement",
			ResourceID:     requirement.ID,
			Description:    fmt.Sprintf("Activated requirement: %s", requirement.Title),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusCreated, requirement)
	}
}

// handleListRequirements lists all active requirements for the organization
func (s *Server) handleListRequirements() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		requirements, err := s.store.ListRequirements(r.Context(), claims.OrganizationID)
		if err != nil {
			s.logger.Error("failed to list requirements", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to get requirements")
			return
		}

		// Update status for each requirement
		for _, req := range requirements {
			req.Status = req.CalculateStatus()
		}

		respondJSON(w, http.StatusOK, requirements)
	}
}

// handleGetRequirement implements STORY-009: Requirement Detail View
func (s *Server) handleGetRequirement() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		requirementID := chi.URLParam(r, "requirementID")

		requirement, err := s.store.GetRequirement(r.Context(), claims.OrganizationID, requirementID)
		if err != nil {
			respondError(w, http.StatusNotFound, "requirement not found")
			return
		}

		// Update status
		requirement.Status = requirement.CalculateStatus()

		respondJSON(w, http.StatusOK, requirement)
	}
}

// handleUpdateRequirement updates a requirement
func (s *Server) handleUpdateRequirement() http.HandlerFunc {
	type request struct {
		Notes       string                   `json:"notes"`
		NextDueDate *string                  `json:"next_due_date"` // ISO 8601 format
		Status      models.RequirementStatus `json:"status"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		requirementID := chi.URLParam(r, "requirementID")
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		requirement, err := s.store.GetRequirement(r.Context(), claims.OrganizationID, requirementID)
		if err != nil {
			respondError(w, http.StatusNotFound, "requirement not found")
			return
		}

		// Update fields
		requirement.Notes = req.Notes
		requirement.UpdatedBy = claims.UID

		if err := s.store.UpdateRequirement(r.Context(), requirement); err != nil {
			s.logger.Error("failed to update requirement", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to update requirement")
			return
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionRequirementUpdated,
			ResourceType:   "requirement",
			ResourceID:     requirement.ID,
			Description:    fmt.Sprintf("Updated requirement: %s", requirement.Title),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, requirement)
	}
}

// handleDeactivateRequirement deactivates a requirement
func (s *Server) handleDeactivateRequirement() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		requirementID := chi.URLParam(r, "requirementID")

		requirement, err := s.store.GetRequirement(r.Context(), claims.OrganizationID, requirementID)
		if err != nil {
			respondError(w, http.StatusNotFound, "requirement not found")
			return
		}

		requirement.IsActive = false
		requirement.UpdatedBy = claims.UID

		if err := s.store.UpdateRequirement(r.Context(), requirement); err != nil {
			s.logger.Error("failed to deactivate requirement", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to deactivate requirement")
			return
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionRequirementDeactivated,
			ResourceType:   "requirement",
			ResourceID:     requirement.ID,
			Description:    fmt.Sprintf("Deactivated requirement: %s", requirement.Title),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, map[string]string{"message": "requirement deactivated successfully"})
	}
}
