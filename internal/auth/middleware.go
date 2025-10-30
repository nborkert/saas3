package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// ContextKey is a custom type for context keys to avoid collisions
type ContextKey string

const (
	// UserContextKey is the key for storing user claims in request context
	UserContextKey ContextKey = "user"
)

// UserClaims represents the authenticated user's claims from JWT
type UserClaims struct {
	UID            string
	Email          string
	EmailVerified  bool
	OrganizationID string // From custom claims
	Role           string // From custom claims
}

// AuthMiddleware provides JWT authentication using Firebase Identity Platform
type AuthMiddleware struct {
	authClient *auth.Client
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(ctx context.Context, projectID string, credentialsFile string) (*AuthMiddleware, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}

	config := &firebase.Config{ProjectID: projectID}
	app, err := firebase.NewApp(ctx, config, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize firebase app: %w", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize firebase auth: %w", err)
	}

	return &AuthMiddleware{authClient: authClient}, nil
}

// Authenticate is a middleware that verifies JWT tokens from Firebase
func (am *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		// Check Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		token := parts[1]

		// Verify the token
		decodedToken, err := am.authClient.VerifyIDToken(r.Context(), token)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Extract claims
		claims := &UserClaims{
			UID:           decodedToken.UID,
			Email:         decodedToken.Claims["email"].(string),
			EmailVerified: decodedToken.Claims["email_verified"].(bool),
		}

		// Extract custom claims if they exist
		if orgID, ok := decodedToken.Claims["organizationId"].(string); ok {
			claims.OrganizationID = orgID
		}

		if role, ok := decodedToken.Claims["role"].(string); ok {
			claims.Role = role
		}

		// Store claims in request context
		ctx := context.WithValue(r.Context(), UserContextKey, claims)

		// Call next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole is a middleware that checks if the user has a specific role
func (am *AuthMiddleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserContextKey).(*UserClaims)
			if !ok {
				respondError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			// Check if user has one of the required roles
			hasRole := false
			for _, role := range roles {
				if claims.Role == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				respondError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireOrganization is a middleware that ensures the user belongs to an organization
func (am *AuthMiddleware) RequireOrganization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(UserContextKey).(*UserClaims)
		if !ok {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if claims.OrganizationID == "" {
			respondError(w, http.StatusForbidden, "organization membership required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetUserClaims extracts user claims from the request context
func GetUserClaims(r *http.Request) (*UserClaims, error) {
	claims, ok := r.Context().Value(UserContextKey).(*UserClaims)
	if !ok {
		return nil, fmt.Errorf("user claims not found in context")
	}
	return claims, nil
}

// SetCustomClaims sets custom claims for a user in Firebase
func (am *AuthMiddleware) SetCustomClaims(ctx context.Context, uid string, claims map[string]interface{}) error {
	if err := am.authClient.SetCustomUserClaims(ctx, uid, claims); err != nil {
		return fmt.Errorf("failed to set custom claims: %w", err)
	}
	return nil
}

// CreateUser creates a new user in Firebase Auth
func (am *AuthMiddleware) CreateUser(ctx context.Context, email, password, displayName string) (string, error) {
	params := (&auth.UserToCreate{}).
		Email(email).
		Password(password).
		DisplayName(displayName).
		EmailVerified(false)

	user, err := am.authClient.CreateUser(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	return user.UID, nil
}

// SendPasswordResetEmail sends a password reset email
func (am *AuthMiddleware) SendPasswordResetEmail(ctx context.Context, email string) error {
	link, err := am.authClient.PasswordResetLink(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to generate password reset link: %w", err)
	}

	// In production, this would send an email via SendGrid
	// For now, we'll just return the link (could be logged or returned to user in dev mode)
	fmt.Printf("Password reset link for %s: %s\n", email, link)

	return nil
}

// SendEmailVerification sends an email verification link
func (am *AuthMiddleware) SendEmailVerification(ctx context.Context, email string) error {
	link, err := am.authClient.EmailVerificationLink(ctx, email)
	if err != nil {
		return fmt.Errorf("failed to generate email verification link: %w", err)
	}

	// In production, this would send an email via SendGrid
	fmt.Printf("Email verification link for %s: %s\n", email, link)

	return nil
}

// DeleteUser deletes a user from Firebase Auth
func (am *AuthMiddleware) DeleteUser(ctx context.Context, uid string) error {
	if err := am.authClient.DeleteUser(ctx, uid); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// Helper function to respond with JSON error
func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error": "%s"}`, message)
}
