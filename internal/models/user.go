package models

import "time"

// UserRole represents the role of a user within an organization
type UserRole string

const (
	RoleAdmin            UserRole = "admin"
	RoleComplianceOfficer UserRole = "compliance_officer"
	RoleViewer           UserRole = "viewer"
)

// User represents a user account
type User struct {
	UID              string    `firestore:"uid" json:"uid"` // Firebase Auth UID
	Email            string    `firestore:"email" json:"email"`
	FullName         string    `firestore:"full_name" json:"full_name"`
	OrganizationID   string    `firestore:"organization_id" json:"organization_id"`
	Role             UserRole  `firestore:"role" json:"role"`
	Status           string    `firestore:"status" json:"status"` // active, pending, inactive
	EmailVerified    bool      `firestore:"email_verified" json:"email_verified"`
	CreatedAt        time.Time `firestore:"created_at" json:"created_at"`
	UpdatedAt        time.Time `firestore:"updated_at" json:"updated_at"`
	LastLoginAt      *time.Time `firestore:"last_login_at,omitempty" json:"last_login_at,omitempty"`
}

// Invitation represents a pending user invitation
type Invitation struct {
	ID             string    `firestore:"id" json:"id"`
	Email          string    `firestore:"email" json:"email"`
	OrganizationID string    `firestore:"organization_id" json:"organization_id"`
	Role           UserRole  `firestore:"role" json:"role"`
	InvitedBy      string    `firestore:"invited_by" json:"invited_by"`
	Message        string    `firestore:"message,omitempty" json:"message,omitempty"`
	Token          string    `firestore:"token" json:"-"` // Not exposed in JSON
	Status         string    `firestore:"status" json:"status"` // pending, accepted, expired, revoked
	CreatedAt      time.Time `firestore:"created_at" json:"created_at"`
	ExpiresAt      time.Time `firestore:"expires_at" json:"expires_at"`
}

// HasPermission checks if a user has permission for a specific action
func (u *User) HasPermission(action string) bool {
	switch u.Role {
	case RoleAdmin:
		return true // Admins have all permissions
	case RoleComplianceOfficer:
		// Compliance officers can do most things except manage users and org settings
		return action != "manage_users" && action != "manage_billing" && action != "manage_integrations"
	case RoleViewer:
		// Viewers can only read
		return action == "view_dashboard" || action == "view_requirements" ||
		       action == "view_evidence" || action == "view_audit_log" ||
		       action == "generate_reports"
	default:
		return false
	}
}

// CanWrite checks if a user has write permissions
func (u *User) CanWrite() bool {
	return u.Role == RoleAdmin || u.Role == RoleComplianceOfficer
}

// IsAdmin checks if a user is an admin
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}
