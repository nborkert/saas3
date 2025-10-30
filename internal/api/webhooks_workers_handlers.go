package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"compliancesync-api/internal/auth"
	"compliancesync-api/internal/models"
)

// Subscription and Integration handlers

// handleGetSubscription implements STORY-039: View and Manage Subscription
func (s *Server) handleGetSubscription() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		org, err := s.store.GetOrganization(r.Context(), claims.OrganizationID)
		if err != nil {
			respondError(w, http.StatusNotFound, "organization not found")
			return
		}

		respondJSON(w, http.StatusOK, org.Subscription)
	}
}

// handleCreateSubscription implements STORY-004: Subscription Tier Selection
func (s *Server) handleCreateSubscription() http.HandlerFunc {
	type request struct {
		Tier            models.SubscriptionTier `json:"tier"`
		PaymentMethodID string                  `json:"payment_method_id"` // Stripe payment method ID
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

		// In production, would create Stripe customer and subscription here
		// For now, just update the organization record
		org.Subscription.Tier = req.Tier
		org.Subscription.Status = "active"
		org.Subscription.MaxUsers = models.GetMaxUsers(req.Tier)
		org.Subscription.MonthlyPrice = models.GetMonthlyPrice(req.Tier)

		if err := s.store.UpdateOrganization(r.Context(), org); err != nil {
			s.logger.Error("failed to update subscription", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to create subscription")
			return
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionSubscriptionUpdated,
			ResourceType:   "subscription",
			ResourceID:     org.ID,
			Description:    fmt.Sprintf("Subscription created: %s tier", req.Tier),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusCreated, org.Subscription)
	}
}

// handleUpdateSubscription updates a subscription
func (s *Server) handleUpdateSubscription() http.HandlerFunc {
	type request struct {
		Tier models.SubscriptionTier `json:"tier"`
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

		// Update subscription tier
		oldTier := org.Subscription.Tier
		org.Subscription.Tier = req.Tier
		org.Subscription.MaxUsers = models.GetMaxUsers(req.Tier)
		org.Subscription.MonthlyPrice = models.GetMonthlyPrice(req.Tier)

		if err := s.store.UpdateOrganization(r.Context(), org); err != nil {
			s.logger.Error("failed to update subscription", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to update subscription")
			return
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionSubscriptionUpdated,
			ResourceType:   "subscription",
			ResourceID:     org.ID,
			Description:    fmt.Sprintf("Subscription updated from %s to %s", oldTier, req.Tier),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, org.Subscription)
	}
}

// handleCancelSubscription implements STORY-042: Cancel Subscription
func (s *Server) handleCancelSubscription() http.HandlerFunc {
	type request struct {
		ConfirmOrganizationName string `json:"confirm_organization_name"`
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

		// Verify organization name confirmation
		if req.ConfirmOrganizationName != org.Name {
			respondError(w, http.StatusBadRequest, "organization name confirmation does not match")
			return
		}

		// Mark subscription for cancellation at period end
		org.Subscription.CancelAtPeriodEnd = true

		if err := s.store.UpdateOrganization(r.Context(), org); err != nil {
			s.logger.Error("failed to cancel subscription", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to cancel subscription")
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"message": "Subscription will be canceled at the end of the current billing period",
		})
	}
}

// Integration handlers

// handleListIntegrations lists all integrations for the organization
func (s *Server) handleListIntegrations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// In production, would query integrations collection
		// For now, return empty array
		respondJSON(w, http.StatusOK, []interface{}{})
	}
}

// handleConnectGoogle implements STORY-015: Google Workspace Integration - OAuth Connection
func (s *Server) handleConnectGoogle() http.HandlerFunc {
	type request struct {
		AuthorizationCode string `json:"authorization_code"`
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

		// In production, would exchange authorization code for tokens
		// and store encrypted in Firestore
		s.logger.Info("google workspace connection requested", "org_id", claims.OrganizationID)

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionIntegrationConnected,
			ResourceType:   "integration",
			ResourceID:     "google_workspace",
			Description:    "Connected Google Workspace integration",
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, map[string]string{
			"message": "Google Workspace connected successfully",
			"status":  "connected",
		})
	}
}

// handleDisconnectGoogle disconnects Google Workspace integration
func (s *Server) handleDisconnectGoogle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// In production, would revoke tokens and delete integration record
		s.logger.Info("google workspace disconnection requested", "org_id", claims.OrganizationID)

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionIntegrationDisconnected,
			ResourceType:   "integration",
			ResourceID:     "google_workspace",
			Description:    "Disconnected Google Workspace integration",
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, map[string]string{
			"message": "Google Workspace disconnected successfully",
		})
	}
}

// Webhook handlers

// handleStripeWebhook handles Stripe webhook events
func (s *Server) handleStripeWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// In production, would verify Stripe signature and process webhook events
		// This would handle subscription status changes, payment failures, etc.
		s.logger.Info("stripe webhook received")

		respondJSON(w, http.StatusOK, map[string]bool{"received": true})
	}
}

// Worker handlers (triggered by Pub/Sub)

// handleGmailPoll handles Gmail polling worker requests
func (s *Server) handleGmailPoll() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// In production, would:
		// 1. Parse Pub/Sub message
		// 2. Query organizations with Gmail integration enabled
		// 3. Poll Gmail API for each organization
		// 4. Create evidence records for matching emails
		s.logger.Info("gmail poll worker triggered")

		respondJSON(w, http.StatusOK, map[string]string{"status": "processed"})
	}
}

// handlePDFGenerate handles PDF generation worker requests
func (s *Server) handlePDFGenerate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// In production, would:
		// 1. Parse Pub/Sub message with report ID
		// 2. Query requirements and evidence
		// 3. Generate PDF using Puppeteer or similar
		// 4. Upload to Cloud Storage
		// 5. Update report status
		s.logger.Info("pdf generation worker triggered")

		respondJSON(w, http.StatusOK, map[string]string{"status": "processed"})
	}
}
