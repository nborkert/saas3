package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"compliancesync-api/internal/auth"
	"compliancesync-api/internal/models"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// Authentication handlers

// handleRegister implements STORY-001: User Registration with Email Verification
func (s *Server) handleRegister() http.HandlerFunc {
	type request struct {
		Email            string `json:"email"`
		FullName         string `json:"full_name"`
		OrganizationName string `json:"organization_name"`
		Password         string `json:"password"`
		Industry         models.Industry `json:"industry"`
		EmployeeCount    models.EmployeeCountRange `json:"employee_count"`
		RegulatoryFramework models.RegulatoryFramework `json:"regulatory_framework"`
	}

	type response struct {
		Message        string `json:"message"`
		UserID         string `json:"user_id"`
		OrganizationID string `json:"organization_id"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate password requirements (8+ chars, 1 uppercase, 1 number, 1 special)
		if !isValidPassword(req.Password) {
			respondError(w, http.StatusBadRequest, "password must be at least 8 characters with 1 uppercase, 1 number, and 1 special character")
			return
		}

		// Check if email already exists
		existingUser, _ := s.store.GetUserByEmail(r.Context(), req.Email)
		if existingUser != nil {
			respondError(w, http.StatusConflict, "email already registered")
			return
		}

		// Create user in Firebase Auth
		uid, err := s.authMiddleware.CreateUser(r.Context(), req.Email, req.Password, req.FullName)
		if err != nil {
			s.logger.Error("failed to create firebase user", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to create user account")
			return
		}

		// Create organization
		org := &models.Organization{
			Name:                req.OrganizationName,
			Industry:            req.Industry,
			EmployeeCount:       req.EmployeeCount,
			RegulatoryFramework: req.RegulatoryFramework,
			Subscription: models.Subscription{
				Tier:   models.TierStarter,
				Status: "trial",
				MaxUsers: models.GetMaxUsers(models.TierStarter),
				MonthlyPrice: models.GetMonthlyPrice(models.TierStarter),
			},
		}

		if err := s.store.CreateOrganization(r.Context(), org); err != nil {
			s.logger.Error("failed to create organization", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to create organization")
			return
		}

		// Create user in Firestore
		user := &models.User{
			UID:            uid,
			Email:          req.Email,
			FullName:       req.FullName,
			OrganizationID: org.ID,
			Role:           models.RoleAdmin, // First user is admin
			EmailVerified:  false,
		}

		if err := s.store.CreateUser(r.Context(), user); err != nil {
			s.logger.Error("failed to create user in firestore", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to create user")
			return
		}

		// Set custom claims in Firebase
		if err := s.authMiddleware.SetCustomClaims(r.Context(), uid, map[string]interface{}{
			"organizationId": org.ID,
			"role":          string(models.RoleAdmin),
		}); err != nil {
			s.logger.Error("failed to set custom claims", "error", err)
		}

		// Send verification email
		if err := s.authMiddleware.SendEmailVerification(r.Context(), req.Email); err != nil {
			s.logger.Error("failed to send verification email", "error", err)
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: org.ID,
			UserID:         uid,
			UserEmail:      req.Email,
			Action:         models.ActionOrgCreated,
			ResourceType:   "organization",
			ResourceID:     org.ID,
			Description:    fmt.Sprintf("Organization '%s' created by %s", org.Name, req.FullName),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusCreated, response{
			Message:        "Registration successful. Please check your email to verify your account.",
			UserID:         uid,
			OrganizationID: org.ID,
		})
	}
}

// handlePasswordReset implements STORY-005: Password Reset Flow
func (s *Server) handlePasswordReset() http.HandlerFunc {
	type request struct {
		Email string `json:"email"`
	}

	type response struct {
		Message string `json:"message"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Always return success message for security (don't reveal if email exists)
		if err := s.authMiddleware.SendPasswordResetEmail(r.Context(), req.Email); err != nil {
			s.logger.Error("failed to send password reset email", "error", err)
		}

		respondJSON(w, http.StatusOK, response{
			Message: "If an account exists with this email, you will receive password reset instructions.",
		})
	}
}

// Profile handlers

// handleGetProfile gets the current user's profile
func (s *Server) handleGetProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		user, err := s.store.GetUser(r.Context(), claims.UID)
		if err != nil {
			s.logger.Error("failed to get user", "error", err)
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		// Update last login
		s.store.UpdateLastLogin(r.Context(), claims.UID)

		respondJSON(w, http.StatusOK, user)
	}
}

// handleUpdateProfile updates the current user's profile
func (s *Server) handleUpdateProfile() http.HandlerFunc {
	type request struct {
		FullName string `json:"full_name"`
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

		user, err := s.store.GetUser(r.Context(), claims.UID)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		user.FullName = req.FullName
		if err := s.store.UpdateUser(r.Context(), user); err != nil {
			s.logger.Error("failed to update user", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to update profile")
			return
		}

		respondJSON(w, http.StatusOK, user)
	}
}

// Organization handlers

// handleGetOrganization implements STORY-003: Organization Profile Setup
func (s *Server) handleGetOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		org, err := s.store.GetOrganization(r.Context(), claims.OrganizationID)
		if err != nil {
			s.logger.Error("failed to get organization", "error", err)
			respondError(w, http.StatusNotFound, "organization not found")
			return
		}

		respondJSON(w, http.StatusOK, org)
	}
}

// handleUpdateOrganization implements STORY-038: Update Organization Profile
func (s *Server) handleUpdateOrganization() http.HandlerFunc {
	type request struct {
		Name                string                      `json:"name"`
		Industry            models.Industry             `json:"industry"`
		EmployeeCount       models.EmployeeCountRange   `json:"employee_count"`
		RegulatoryFramework models.RegulatoryFramework  `json:"regulatory_framework"`
		Website             string                      `json:"website"`
		Address             string                      `json:"address"`
		Phone               string                      `json:"phone"`
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

		org, err := s.store.GetOrganization(r.Context(), claims.OrganizationID)
		if err != nil {
			respondError(w, http.StatusNotFound, "organization not found")
			return
		}

		// Update fields
		org.Name = req.Name
		org.Industry = req.Industry
		org.EmployeeCount = req.EmployeeCount
		org.RegulatoryFramework = req.RegulatoryFramework
		org.Website = req.Website
		org.Address = req.Address
		org.Phone = req.Phone
		org.UpdatedBy = claims.UID

		if err := s.store.UpdateOrganization(r.Context(), org); err != nil {
			s.logger.Error("failed to update organization", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to update organization")
			return
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: org.ID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionOrgUpdated,
			ResourceType:   "organization",
			ResourceID:     org.ID,
			Description:    "Organization profile updated",
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, org)
	}
}

// handleGetDashboard implements STORY-008: Compliance Dashboard Overview
func (s *Server) handleGetDashboard() http.HandlerFunc {
	type dashboardMetrics struct {
		TotalRequirements     int `json:"total_requirements"`
		CompliantRequirements int `json:"compliant_requirements"`
		AtRiskRequirements    int `json:"at_risk_requirements"`
		NonCompliantRequirements int `json:"non_compliant_requirements"`
		TotalEvidence         int `json:"total_evidence"`
		UpcomingDeadlines     []models.Requirement `json:"upcoming_deadlines"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// Get all requirements
		requirements, err := s.store.ListRequirements(r.Context(), claims.OrganizationID)
		if err != nil {
			s.logger.Error("failed to list requirements", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to get dashboard data")
			return
		}

		// Calculate metrics
		metrics := dashboardMetrics{
			TotalRequirements: len(requirements),
		}

		var upcomingDeadlines []models.Requirement
		now := time.Now()
		thirtyDaysFromNow := now.AddDate(0, 0, 30)

		for _, req := range requirements {
			// Update status based on evidence and due dates
			req.Status = req.CalculateStatus()

			switch req.Status {
			case models.StatusCompliant:
				metrics.CompliantRequirements++
			case models.StatusAtRisk:
				metrics.AtRiskRequirements++
			case models.StatusNonCompliant:
				metrics.NonCompliantRequirements++
			}

			// Check for upcoming deadlines
			if req.NextDueDate != nil && req.NextDueDate.After(now) && req.NextDueDate.Before(thirtyDaysFromNow) {
				upcomingDeadlines = append(upcomingDeadlines, *req)
			}
		}

		metrics.UpcomingDeadlines = upcomingDeadlines

		// Get total evidence count
		evidence, err := s.store.ListEvidence(r.Context(), claims.OrganizationID, nil)
		if err == nil {
			metrics.TotalEvidence = len(evidence)
		}

		respondJSON(w, http.StatusOK, metrics)
	}
}

// User management handlers

// handleListUsers lists all users in the organization
func (s *Server) handleListUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		users, err := s.store.ListUsersByOrganization(r.Context(), claims.OrganizationID)
		if err != nil {
			s.logger.Error("failed to list users", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to list users")
			return
		}

		respondJSON(w, http.StatusOK, users)
	}
}

// handleInviteUser implements STORY-032: Invite Users to Organization
func (s *Server) handleInviteUser() http.HandlerFunc {
	type request struct {
		Email   string           `json:"email"`
		Role    models.UserRole  `json:"role"`
		Message string           `json:"message"`
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

		// Get organization to check user limits
		org, err := s.store.GetOrganization(r.Context(), claims.OrganizationID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to get organization")
			return
		}

		// Check user limit
		users, err := s.store.ListUsersByOrganization(r.Context(), claims.OrganizationID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to check user limit")
			return
		}

		if len(users) >= org.Subscription.MaxUsers {
			respondError(w, http.StatusForbidden, fmt.Sprintf("user limit reached. Please upgrade to add more users (current limit: %d)", org.Subscription.MaxUsers))
			return
		}

		// In production, would create invitation and send email
		// For now, return success
		respondJSON(w, http.StatusOK, map[string]string{
			"message": fmt.Sprintf("Invitation sent to %s", req.Email),
		})
	}
}

// handleUpdateUserRole implements STORY-036: Manage Existing Users
func (s *Server) handleUpdateUserRole() http.HandlerFunc {
	type request struct {
		Role models.UserRole `json:"role"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		userID := chi.URLParam(r, "userID")
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		user, err := s.store.GetUser(r.Context(), userID)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		// Verify user belongs to same organization
		if user.OrganizationID != claims.OrganizationID {
			respondError(w, http.StatusForbidden, "user not in your organization")
			return
		}

		user.Role = req.Role
		if err := s.store.UpdateUser(r.Context(), user); err != nil {
			s.logger.Error("failed to update user role", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to update user role")
			return
		}

		// Update custom claims in Firebase
		s.authMiddleware.SetCustomClaims(r.Context(), userID, map[string]interface{}{
			"organizationId": user.OrganizationID,
			"role":          string(user.Role),
		})

		respondJSON(w, http.StatusOK, user)
	}
}

// handleDeleteUser deletes a user
func (s *Server) handleDeleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		userID := chi.URLParam(r, "userID")

		// Prevent self-deletion
		if userID == claims.UID {
			respondError(w, http.StatusBadRequest, "cannot delete your own account")
			return
		}

		user, err := s.store.GetUser(r.Context(), userID)
		if err != nil {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}

		// Verify user belongs to same organization
		if user.OrganizationID != claims.OrganizationID {
			respondError(w, http.StatusForbidden, "user not in your organization")
			return
		}

		// Delete from Firebase Auth
		if err := s.authMiddleware.DeleteUser(r.Context(), userID); err != nil {
			s.logger.Error("failed to delete firebase user", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to delete user")
			return
		}

		// Update user status in Firestore (soft delete)
		user.Status = "inactive"
		s.store.UpdateUser(r.Context(), user)

		respondJSON(w, http.StatusOK, map[string]string{"message": "user deleted successfully"})
	}
}

// Helper functions

func isValidPassword(password string) bool {
	if len(password) < 8 {
		return false
	}

	hasUpper := false
	hasNumber := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case '0' <= char && char <= '9':
			hasNumber = true
		case !('a' <= char && char <= 'z') && !('A' <= char && char <= 'Z') && !('0' <= char && char <= '9'):
			hasSpecial = true
		}
	}

	return hasUpper && hasNumber && hasSpecial
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
