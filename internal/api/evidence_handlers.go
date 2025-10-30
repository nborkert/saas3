package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"compliancesync-api/internal/auth"
	"compliancesync-api/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// handleGenerateUploadURL implements STORY-013: Manual Evidence Upload (signed URL generation)
func (s *Server) handleGenerateUploadURL() http.HandlerFunc {
	type request struct {
		FileName    string `json:"file_name"`
		FileType    string `json:"file_type"`
		FileSize    int64  `json:"file_size"`
	}

	type response struct {
		UploadURL  string `json:"upload_url"`
		EvidenceID string `json:"evidence_id"`
		ExpiresAt  string `json:"expires_at"`
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

		// Validate file size (25MB max)
		if req.FileSize > 25*1024*1024 {
			respondError(w, http.StatusBadRequest, "file size exceeds maximum of 25MB")
			return
		}

		// Validate file type
		allowedTypes := map[string]bool{
			"application/pdf":  true,
			"application/msword": true,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
			"application/vnd.ms-excel": true,
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
			"image/png": true,
			"image/jpeg": true,
		}

		if !allowedTypes[req.FileType] {
			respondError(w, http.StatusBadRequest, "unsupported file type")
			return
		}

		// Generate unique evidence ID
		evidenceID := uuid.New().String()

		// Generate Cloud Storage path
		filePath := fmt.Sprintf("%s/evidence/%s-%s", claims.OrganizationID, evidenceID, req.FileName)

		// Generate signed URL for upload
		expiresAt := time.Now().Add(15 * time.Minute)
		opts := &storage.SignedURLOptions{
			Scheme:      storage.SigningSchemeV4,
			Method:      "PUT",
			Expires:     expiresAt,
			ContentType: req.FileType,
		}

		url, err := storage.SignedURL(s.config.StorageBucket, filePath, opts)
		if err != nil {
			s.logger.Error("failed to generate signed URL", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to generate upload URL")
			return
		}

		// Create pending evidence record
		evidence := &models.Evidence{
			ID:             evidenceID,
			OrganizationID: claims.OrganizationID,
			FileName:       req.FileName,
			FileSize:       req.FileSize,
			FileType:       req.FileType,
			FileURL:        filePath,
			Status:         "uploading",
			UploadedBy:     claims.UID,
		}

		if err := s.store.CreateEvidence(r.Context(), evidence); err != nil {
			s.logger.Error("failed to create evidence record", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to create evidence")
			return
		}

		respondJSON(w, http.StatusOK, response{
			UploadURL:  url,
			EvidenceID: evidenceID,
			ExpiresAt:  expiresAt.Format(time.RFC3339),
		})
	}
}

// handleCreateEvidence implements STORY-013: Manual Evidence Upload (complete upload and associate with requirements)
func (s *Server) handleCreateEvidence() http.HandlerFunc {
	type request struct {
		EvidenceID     string    `json:"evidence_id"` // From upload URL generation
		Title          string    `json:"title"`
		Description    string    `json:"description"`
		EvidenceDate   string    `json:"evidence_date"` // ISO 8601 format
		RequirementIDs []string  `json:"requirement_ids"`
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

		// Parse evidence date
		evidenceDate, err := time.Parse(time.RFC3339, req.EvidenceDate)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid evidence date format")
			return
		}

		// Get existing evidence record
		evidence, err := s.store.GetEvidence(r.Context(), claims.OrganizationID, req.EvidenceID)
		if err != nil {
			respondError(w, http.StatusNotFound, "evidence not found")
			return
		}

		// Update evidence record
		evidence.Title = req.Title
		evidence.Description = req.Description
		evidence.EvidenceDate = evidenceDate
		evidence.RequirementIDs = req.RequirementIDs
		evidence.Source = models.SourceManualUpload
		evidence.Status = "active"

		if err := s.store.UpdateEvidence(r.Context(), evidence); err != nil {
			s.logger.Error("failed to update evidence", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to complete evidence upload")
			return
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionEvidenceCreated,
			ResourceType:   "evidence",
			ResourceID:     evidence.ID,
			Description:    fmt.Sprintf("Uploaded evidence: %s", evidence.Title),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusCreated, evidence)
	}
}

// handleListEvidence implements STORY-014: Evidence List View and Search
func (s *Server) handleListEvidence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// Get query parameters for filtering
		filters := make(map[string]interface{})
		if source := r.URL.Query().Get("source"); source != "" {
			filters["source"] = models.EvidenceSource(source)
		}

		evidence, err := s.store.ListEvidence(r.Context(), claims.OrganizationID, filters)
		if err != nil {
			s.logger.Error("failed to list evidence", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to get evidence")
			return
		}

		respondJSON(w, http.StatusOK, evidence)
	}
}

// handleGetEvidence gets a single evidence item
func (s *Server) handleGetEvidence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		evidenceID := chi.URLParam(r, "evidenceID")

		evidence, err := s.store.GetEvidence(r.Context(), claims.OrganizationID, evidenceID)
		if err != nil {
			respondError(w, http.StatusNotFound, "evidence not found")
			return
		}

		// Create audit log for viewing evidence
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionEvidenceViewed,
			ResourceType:   "evidence",
			ResourceID:     evidence.ID,
			Description:    fmt.Sprintf("Viewed evidence: %s", evidence.Title),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, evidence)
	}
}

// handleUpdateEvidence implements STORY-020: Associate Evidence with Requirements
func (s *Server) handleUpdateEvidence() http.HandlerFunc {
	type request struct {
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		RequirementIDs []string `json:"requirement_ids"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		evidenceID := chi.URLParam(r, "evidenceID")
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		evidence, err := s.store.GetEvidence(r.Context(), claims.OrganizationID, evidenceID)
		if err != nil {
			respondError(w, http.StatusNotFound, "evidence not found")
			return
		}

		// Update fields
		oldRequirements := evidence.RequirementIDs
		evidence.Title = req.Title
		evidence.Description = req.Description
		evidence.RequirementIDs = req.RequirementIDs

		if err := s.store.UpdateEvidence(r.Context(), evidence); err != nil {
			s.logger.Error("failed to update evidence", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to update evidence")
			return
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionEvidenceUpdated,
			ResourceType:   "evidence",
			ResourceID:     evidence.ID,
			Description:    fmt.Sprintf("Updated evidence: %s", evidence.Title),
			Changes: map[string]interface{}{
				"requirement_ids": map[string]interface{}{
					"from": oldRequirements,
					"to":   req.RequirementIDs,
				},
			},
			IPAddress: r.RemoteAddr,
			UserAgent: r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, evidence)
	}
}

// handleDeleteEvidence implements STORY-021: Evidence Deletion and Retention
func (s *Server) handleDeleteEvidence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		evidenceID := chi.URLParam(r, "evidenceID")

		evidence, err := s.store.GetEvidence(r.Context(), claims.OrganizationID, evidenceID)
		if err != nil {
			respondError(w, http.StatusNotFound, "evidence not found")
			return
		}

		// Soft delete
		if err := s.store.DeleteEvidence(r.Context(), claims.OrganizationID, evidenceID); err != nil {
			s.logger.Error("failed to delete evidence", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to delete evidence")
			return
		}

		// Create audit log
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionEvidenceDeleted,
			ResourceType:   "evidence",
			ResourceID:     evidence.ID,
			Description:    fmt.Sprintf("Deleted evidence: %s", evidence.Title),
			Metadata: map[string]interface{}{
				"requirement_ids": evidence.RequirementIDs,
			},
			IPAddress: r.RemoteAddr,
			UserAgent: r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, map[string]string{"message": "evidence deleted successfully"})
	}
}

// handleGenerateDownloadURL generates a signed URL for downloading evidence
func (s *Server) handleGenerateDownloadURL() http.HandlerFunc {
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

		evidenceID := chi.URLParam(r, "evidenceID")

		evidence, err := s.store.GetEvidence(r.Context(), claims.OrganizationID, evidenceID)
		if err != nil {
			respondError(w, http.StatusNotFound, "evidence not found")
			return
		}

		// Generate signed URL for download
		expiresAt := time.Now().Add(1 * time.Hour)
		opts := &storage.SignedURLOptions{
			Scheme:  storage.SigningSchemeV4,
			Method:  "GET",
			Expires: expiresAt,
		}

		url, err := storage.SignedURL(s.config.StorageBucket, evidence.FileURL, opts)
		if err != nil {
			s.logger.Error("failed to generate download URL", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to generate download URL")
			return
		}

		// Create audit log for downloading evidence
		auditLog := &models.AuditLog{
			OrganizationID: claims.OrganizationID,
			UserID:         claims.UID,
			UserEmail:      claims.Email,
			Action:         models.ActionEvidenceDownloaded,
			ResourceType:   "evidence",
			ResourceID:     evidence.ID,
			Description:    fmt.Sprintf("Downloaded evidence: %s", evidence.Title),
			IPAddress:      r.RemoteAddr,
			UserAgent:      r.UserAgent(),
		}
		s.store.CreateAuditLog(r.Context(), auditLog)

		respondJSON(w, http.StatusOK, response{
			DownloadURL: url,
			ExpiresAt:   expiresAt.Format(time.RFC3339),
		})
	}
}
