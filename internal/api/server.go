package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"compliancesync-api/internal/auth"
	"compliancesync-api/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Server represents the API server
type Server struct {
	router        *chi.Mux
	store         *store.FirestoreStore
	authMiddleware *auth.AuthMiddleware
	storageClient *storage.Client
	logger        *slog.Logger
	config        *Config
}

// Config holds the server configuration
type Config struct {
	Port                string
	ProjectID           string
	FirebaseCredentials string
	StorageBucket       string
	StripeSecretKey     string
	SendGridAPIKey      string
	Environment         string
}

// NewServer creates a new API server
func NewServer(ctx context.Context, config *Config) (*Server, error) {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Initialize Firestore store
	firestoreStore, err := store.NewFirestoreStore(ctx, config.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize firestore store: %w", err)
	}

	// Initialize authentication middleware
	authMW, err := auth.NewAuthMiddleware(ctx, config.ProjectID, config.FirebaseCredentials)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize auth middleware: %w", err)
	}

	// Initialize Cloud Storage client
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage client: %w", err)
	}

	server := &Server{
		store:          firestoreStore,
		authMiddleware: authMW,
		storageClient:  storageClient,
		logger:         logger,
		config:         config,
	}

	// Initialize router
	server.router = server.setupRoutes()

	return server, nil
}

// setupRoutes configures all routes for the API
func (s *Server) setupRoutes() *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint (unauthenticated)
	r.Get("/health", s.handleHealth())

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes (authentication)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister())
			r.Post("/password-reset", s.handlePasswordReset())
		})

		// Protected routes (require authentication)
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware.Authenticate)

			// User profile
			r.Route("/profile", func(r chi.Router) {
				r.Get("/", s.handleGetProfile())
				r.Put("/", s.handleUpdateProfile())
			})

			// Organization routes (require organization membership)
			r.Group(func(r chi.Router) {
				r.Use(s.authMiddleware.RequireOrganization)

				// Organization management
				r.Route("/organization", func(r chi.Router) {
					r.Get("/", s.handleGetOrganization())
					r.Put("/", s.requireAdmin(s.handleUpdateOrganization()))
					r.Get("/dashboard", s.handleGetDashboard())
				})

				// User management
				r.Route("/users", func(r chi.Router) {
					r.Get("/", s.handleListUsers())
					r.Post("/invite", s.requireAdmin(s.handleInviteUser()))
					r.Put("/{userID}/role", s.requireAdmin(s.handleUpdateUserRole()))
					r.Delete("/{userID}", s.requireAdmin(s.handleDeleteUser()))
				})

				// Regulatory requirements
				r.Route("/requirements", func(r chi.Router) {
					r.Get("/", s.handleListRequirements())
					r.Post("/", s.requireWrite(s.handleCreateRequirement()))
					r.Get("/templates", s.handleListRequirementTemplates())
					r.Get("/{requirementID}", s.handleGetRequirement())
					r.Put("/{requirementID}", s.requireWrite(s.handleUpdateRequirement()))
					r.Delete("/{requirementID}", s.requireWrite(s.handleDeactivateRequirement()))
				})

				// Evidence management
				r.Route("/evidence", func(r chi.Router) {
					r.Get("/", s.handleListEvidence())
					r.Post("/upload-url", s.requireWrite(s.handleGenerateUploadURL()))
					r.Post("/", s.requireWrite(s.handleCreateEvidence()))
					r.Get("/{evidenceID}", s.handleGetEvidence())
					r.Put("/{evidenceID}", s.requireWrite(s.handleUpdateEvidence()))
					r.Delete("/{evidenceID}", s.requireWrite(s.handleDeleteEvidence()))
					r.Get("/{evidenceID}/download-url", s.handleGenerateDownloadURL())
				})

				// Audit logs
				r.Route("/audit-logs", func(r chi.Router) {
					r.Get("/", s.handleListAuditLogs())
					r.Get("/export", s.handleExportAuditLogs())
				})

				// Reports
				r.Route("/reports", func(r chi.Router) {
					r.Get("/", s.handleListReports())
					r.Post("/", s.handleGenerateReport())
					r.Get("/{reportID}", s.handleGetReport())
					r.Get("/{reportID}/download-url", s.handleGetReportDownloadURL())
				})

				// Integrations
				r.Route("/integrations", func(r chi.Router) {
					r.Get("/", s.handleListIntegrations())
					r.Post("/google/connect", s.requireAdmin(s.handleConnectGoogle()))
					r.Delete("/google/disconnect", s.requireAdmin(s.handleDisconnectGoogle()))
				})

				// Subscription management
				r.Route("/subscription", func(r chi.Router) {
					r.Get("/", s.handleGetSubscription())
					r.Post("/", s.requireAdmin(s.handleCreateSubscription()))
					r.Put("/", s.requireAdmin(s.handleUpdateSubscription()))
					r.Post("/cancel", s.requireAdmin(s.handleCancelSubscription()))
				})
			})
		})

		// Stripe webhook (public, verified by Stripe signature)
		r.Post("/webhooks/stripe", s.handleStripeWebhook())

		// Pub/Sub endpoints (protected by Cloud Run service-to-service auth in production)
		r.Post("/workers/gmail-poll", s.handleGmailPoll())
		r.Post("/workers/pdf-generate", s.handlePDFGenerate())
	})

	return r
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := ":" + s.config.Port
	s.logger.Info("starting server", "port", s.config.Port, "environment", s.config.Environment)

	server := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")

	// Close Firestore connection
	if err := s.store.Close(); err != nil {
		s.logger.Error("failed to close firestore", "error", err)
	}

	// Close storage client
	if err := s.storageClient.Close(); err != nil {
		s.logger.Error("failed to close storage client", "error", err)
	}

	return nil
}

// handleHealth returns a health check handler
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{
			"status": "healthy",
			"service": "compliancesync-api",
		})
	}
}

// Middleware helpers

// requireAdmin is a middleware that requires admin role
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if claims.Role != "admin" {
			respondError(w, http.StatusForbidden, "admin role required")
			return
		}

		next.ServeHTTP(w, r)
	}
}

// requireWrite is a middleware that requires write permissions (admin or compliance_officer)
func (s *Server) requireWrite(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.GetUserClaims(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if claims.Role != "admin" && claims.Role != "compliance_officer" {
			respondError(w, http.StatusForbidden, "write permission required")
			return
		}

		next.ServeHTTP(w, r)
	}
}

// Helper functions for JSON responses

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		if err := jsonEncode(w, data); err != nil {
			// Log error but don't return it to client since headers are already written
			fmt.Printf("failed to encode JSON response: %v\n", err)
		}
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func jsonEncode(w http.ResponseWriter, v interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
