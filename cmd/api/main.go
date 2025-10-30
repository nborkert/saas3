package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"compliancesync-api/internal/api"
)

func main() {
	// Load configuration from environment variables
	config := &api.Config{
		Port:                getEnv("PORT", "8080"),
		ProjectID:           getEnv("GCP_PROJECT_ID", ""),
		FirebaseCredentials: getEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
		StorageBucket:       getEnv("STORAGE_BUCKET", ""),
		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		SendGridAPIKey:      getEnv("SENDGRID_API_KEY", ""),
		Environment:         getEnv("ENVIRONMENT", "development"),
	}

	// Validate required configuration
	if config.ProjectID == "" {
		log.Fatal("GCP_PROJECT_ID environment variable is required")
	}

	if config.StorageBucket == "" {
		log.Fatal("STORAGE_BUCKET environment variable is required")
	}

	// Create context
	ctx := context.Background()

	// Initialize server
	server, err := api.NewServer(ctx, config)
	if err != nil {
		log.Fatalf("failed to initialize server: %v", err)
	}

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Starting ComplianceSync API server on port %s", config.Port)
		serverErrors <- server.Start()
	}()

	// Block until shutdown signal or server error
	select {
	case err := <-serverErrors:
		log.Fatalf("server error: %v", err)

	case sig := <-shutdown:
		log.Printf("received shutdown signal: %v", sig)

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Graceful shutdown
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
			log.Printf("forcing shutdown")
		} else {
			log.Println("server shutdown complete")
		}
	}
}

// getEnv gets an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
