package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
	"compliancesync-api/internal/auth"
	"compliancesync-api/internal/models"
	"github.com/go-chi/chi/v5"
)

// handleListAuditLogs implements STORY-022: System Activity Audit Log
func (s *Server) handleListAuditLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// Parse query parameters for filtering
		filters := make(map[string]interface{})
		if userID := r.URL.Query().Get("user_id"); userID != "" {
			filters["user_id"] = userID
		}
		if action := r.URL.Query().Get("action"); action != "" {
			filters["action"] = models.AuditAction(action)
		}
		if resourceType := r.URL.Query().Get("resource_type"); resourceType != "" {
			filters["resource_type"] = resourceType
		}

		// Parse limit
		limit := 100
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
				limit = parsedLimit
			}
		}

		logs, err := s.store.ListAuditLogs(r.Context(), claims.OrganizationID, filters, limit)
		if err != nil {
			s.logger.Error("failed to list audit logs", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to get audit logs")
			return
		}

		respondJSON(w, http.StatusOK, logs)
	}
}

// handleExportAuditLogs implements STORY-025: Audit Log Export
func (s *Server) handleExportAuditLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// Parse query parameters for filtering
		filters := make(map[string]interface{})
		if userID := r.URL.Query().Get("user_id"); userID != "" {
			filters["user_id"] = userID
		}
		if action := r.URL.Query().Get("action"); action != "" {
			filters["action"] = models.AuditAction(action)
		}

		// Max 10,000 entries for export
		logs, err := s.store.ListAuditLogs(r.Context(), claims.OrganizationID, filters, 10000)
		if err != nil {
			s.logger.Error("failed to list audit logs for export", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to export audit logs")
			return
		}

		if len(logs) >= 10000 {
			respondError(w, http.StatusBadRequest, "too many results. Please narrow your date range (max 10,000 entries)")
			return
		}

		// Create CSV
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=audit_log_%s.csv", time.Now().Format("2006-01-02")))

		writer := csv.NewWriter(w)
		defer writer.Flush()

		// Write header
		writer.Write([]string{"Timestamp", "User Email", "User Name", "Action", "Resource Type", "Resource ID", "Description", "IP Address"})

		// Write rows
		for _, log := range logs {
			writer.Write([]string{
				log.Timestamp.Format(time.RFC3339),
				log.UserEmail,
				log.UserID,
				string(log.Action),
				log.ResourceType,
				log.ResourceID,
				log.Description,
				log.IPAddress,
			})
		}
	}
}

// handleGenerateReport implements STORY-023 & STORY-024: Generate compliance reports
func (s *Server) handleGenerateReport() http.HandlerFunc {
	type request struct {
		Type           string   `json:"type"` // "requirement_detail" or "comprehensive"
		RequirementIDs []string `json:"requirement_ids"`
		Title          string   `json:"title"`
		Description    string   `json:"description"`
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

		// Validate report type
		if req.Type != "requirement_detail" && req.Type != "comprehensive" {
			respondError(w, http.StatusBadRequest, "invalid report type")
			return
		}

		// Create report record
		report := &models.Report{
			OrganizationID: claims.OrganizationID,
			Title:          req.Title,
			Description:    req.Description,
			Type:           req.Type,
			RequirementIDs: req.RequirementIDs,
			Status:         "pending",
			GeneratedBy:    claims.UID,
		}

		if err := s.store.CreateReport(r.Context(), report); err != nil {
			s.logger.Error("failed to create report", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to create report")
			return
		}

		// In production, would publish to Pub/Sub topic for PDF generation worker
		// For now, just return the report record
		s.logger.Info("report generation requested", "report_id", report.ID, "type", req.Type)

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionReportGenerated,
			ResourceType:   "report",
			ResourceID:     report.ID,
			Description:    fmt.Sprintf("Generated %s report: %s", req.Type, req.Title),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusAccepted, report)
	}
}

// handleListReports lists all reports for the organization
func (s *Server) handleListReports() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// In a full implementation, would have a ListReports method in store
		// For now, return empty array
		respondJSON(w, http.StatusOK, []interface{}{})
	}
}

// handleGetReport gets a single report
func (s *Server) handleGetReport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		reportID := chi.URLParam(r, "reportID")

		report, err := s.store.GetReport(r.Context(), claims.OrganizationID, reportID)
		if err != nil {
			respondError(w, http.StatusNotFound, "report not found")
			return
		}

		respondJSON(w, http.StatusOK, report)
	}
}

// handleGetReportDownloadURL generates a signed URL for downloading a report
func (s *Server) handleGetReportDownloadURL() http.HandlerFunc {
	type response struct {
		DownloadURL string `json:"download_url"`
		ExpiresAt   string `json:"expires_at"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		reportID := chi.URLParam(r, "reportID")

		report, err := s.store.GetReport(r.Context(), claims.OrganizationID, reportID)
		if err != nil {
			respondError(w, http.StatusNotFound, "report not found")
			return
		}

		if report.Status != "completed" {
			respondError(w, http.StatusBadRequest, "report is not ready for download")
			return
		}

		// Generate signed URL for download
		expiresAt := time.Now().Add(1 * time.Hour)
		opts := &storage.SignedURLOptions{
			Scheme:  storage.SigningSchemeV4,
			Method:  "GET",
			Expires: expiresAt,
		}

		url, err := storage.SignedURL(s.config.StorageBucket, report.FileURL, opts)
		if err != nil {
			s.logger.Error("failed to generate report download URL", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to generate download URL")
			return
		}

		respondJSON(w, http.StatusOK, response{
			DownloadURL: url,
			ExpiresAt:   expiresAt.Format(time.RFC3339),
		})
	}
}
