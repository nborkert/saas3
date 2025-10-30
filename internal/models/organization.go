package models

import "time"

// Industry represents the industry sector of an organization
type Industry string

const (
	IndustryFinancialServices Industry = "financial_services"
	IndustryInsurance         Industry = "insurance"
	IndustryHealthcare        Industry = "healthcare"
	IndustryOther             Industry = "other"
)

// EmployeeCountRange represents the employee count range of an organization
type EmployeeCountRange string

const (
	EmployeeRange1to10   EmployeeCountRange = "1-10"
	EmployeeRange11to25  EmployeeCountRange = "11-25"
	EmployeeRange26to50  EmployeeCountRange = "26-50"
	EmployeeRange51Plus  EmployeeCountRange = "51+"
)

// RegulatoryFramework represents the primary regulatory framework
type RegulatoryFramework string

const (
	FrameworkSECRIA    RegulatoryFramework = "sec_ria"
	FrameworkFINRA     RegulatoryFramework = "finra"
	FrameworkInsurance RegulatoryFramework = "state_insurance"
	FrameworkHIPAA     RegulatoryFramework = "hipaa"
)

// SubscriptionTier represents the subscription plan tier
type SubscriptionTier string

const (
	TierStarter      SubscriptionTier = "starter"
	TierProfessional SubscriptionTier = "professional"
	TierBusiness     SubscriptionTier = "business"
)

// Organization represents a customer organization
type Organization struct {
	ID                   string              `firestore:"id" json:"id"`
	Name                 string              `firestore:"name" json:"name"`
	Industry             Industry            `firestore:"industry" json:"industry"`
	EmployeeCount        EmployeeCountRange  `firestore:"employee_count" json:"employee_count"`
	RegulatoryFramework  RegulatoryFramework `firestore:"regulatory_framework" json:"regulatory_framework"`
	Website              string              `firestore:"website,omitempty" json:"website,omitempty"`
	Address              string              `firestore:"address,omitempty" json:"address,omitempty"`
	Phone                string              `firestore:"phone,omitempty" json:"phone,omitempty"`
	Subscription         Subscription        `firestore:"subscription" json:"subscription"`
	CreatedAt            time.Time           `firestore:"created_at" json:"created_at"`
	UpdatedAt            time.Time           `firestore:"updated_at" json:"updated_at"`
	UpdatedBy            string              `firestore:"updated_by,omitempty" json:"updated_by,omitempty"`
	ActiveUserCount      int                 `firestore:"active_user_count" json:"active_user_count"`
}

// Subscription represents an organization's subscription details
type Subscription struct {
	Tier              SubscriptionTier `firestore:"tier" json:"tier"`
	Status            string           `firestore:"status" json:"status"` // active, canceled, past_due
	StripeCustomerID  string           `firestore:"stripe_customer_id" json:"stripe_customer_id"`
	StripeSubscriptionID string        `firestore:"stripe_subscription_id,omitempty" json:"stripe_subscription_id,omitempty"`
	CurrentPeriodStart time.Time       `firestore:"current_period_start" json:"current_period_start"`
	CurrentPeriodEnd   time.Time       `firestore:"current_period_end" json:"current_period_end"`
	CancelAtPeriodEnd  bool            `firestore:"cancel_at_period_end" json:"cancel_at_period_end"`
	MaxUsers           int             `firestore:"max_users" json:"max_users"`
	MonthlyPrice       float64         `firestore:"monthly_price" json:"monthly_price"`
}

// GetMaxUsers returns the maximum number of users allowed for a subscription tier
func GetMaxUsers(tier SubscriptionTier) int {
	switch tier {
	case TierStarter:
		return 10
	case TierProfessional:
		return 25
	case TierBusiness:
		return 50
	default:
		return 10
	}
}

// GetMonthlyPrice returns the monthly price for a subscription tier
func GetMonthlyPrice(tier SubscriptionTier) float64 {
	switch tier {
	case TierStarter:
		return 149.00
	case TierProfessional:
		return 349.00
	case TierBusiness:
		return 699.00
	default:
		return 149.00
	}
}
