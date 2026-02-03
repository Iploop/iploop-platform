package plans

import (
	"context"
	"database/sql"
	"time"
)

// Plan represents a subscription plan
type Plan struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Type            string    `json:"type"` // "subscription" or "usage"
	StripePriceID   string    `json:"stripe_price_id"`
	PriceMonthly    int64     `json:"price_monthly"` // in cents
	PriceAnnual     int64     `json:"price_annual"`  // in cents
	BandwidthGB     int64     `json:"bandwidth_gb"`  // included bandwidth
	RequestsPerDay  int64     `json:"requests_per_day"`
	ConcurrentConns int       `json:"concurrent_connections"`
	Features        []string  `json:"features"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PlanService manages plans
type PlanService struct {
	db *sql.DB
}

// NewPlanService creates a new plan service
func NewPlanService(db *sql.DB) *PlanService {
	return &PlanService{db: db}
}

// GetDefaultPlans returns the default IPLoop plans
func GetDefaultPlans() []Plan {
	return []Plan{
		{
			ID:              "starter",
			Name:            "Starter",
			Description:     "Perfect for testing and small projects",
			Type:            "subscription",
			PriceMonthly:    4900,  // $49
			PriceAnnual:     47000, // $470 (2 months free)
			BandwidthGB:     5,
			RequestsPerDay:  10000,
			ConcurrentConns: 10,
			Features: []string{
				"5 GB bandwidth/month",
				"10,000 requests/day",
				"10 concurrent connections",
				"HTTP & SOCKS5 protocols",
				"Basic geo-targeting",
				"Email support",
			},
			IsActive: true,
		},
		{
			ID:              "growth",
			Name:            "Growth",
			Description:     "For growing businesses",
			Type:            "subscription",
			PriceMonthly:    14900,  // $149
			PriceAnnual:     143000, // $1,430
			BandwidthGB:     25,
			RequestsPerDay:  50000,
			ConcurrentConns: 50,
			Features: []string{
				"25 GB bandwidth/month",
				"50,000 requests/day",
				"50 concurrent connections",
				"HTTP & SOCKS5 protocols",
				"Advanced geo-targeting",
				"City-level targeting",
				"Priority support",
				"API access",
			},
			IsActive: true,
		},
		{
			ID:              "business",
			Name:            "Business",
			Description:     "For serious data operations",
			Type:            "subscription",
			PriceMonthly:    49900,  // $499
			PriceAnnual:     479000, // $4,790
			BandwidthGB:     100,
			RequestsPerDay:  200000,
			ConcurrentConns: 200,
			Features: []string{
				"100 GB bandwidth/month",
				"200,000 requests/day",
				"200 concurrent connections",
				"HTTP & SOCKS5 protocols",
				"Advanced geo-targeting",
				"City & ASN targeting",
				"Sticky sessions",
				"Dedicated account manager",
				"24/7 support",
				"SLA guarantee",
			},
			IsActive: true,
		},
		{
			ID:              "enterprise",
			Name:            "Enterprise",
			Description:     "Custom solutions for large scale",
			Type:            "usage",
			PriceMonthly:    0, // Custom pricing
			BandwidthGB:     0, // Unlimited
			RequestsPerDay:  0, // Unlimited
			ConcurrentConns: 0, // Custom
			Features: []string{
				"Unlimited bandwidth",
				"Unlimited requests",
				"Custom concurrent connections",
				"All protocols",
				"All targeting options",
				"Custom integration",
				"Dedicated infrastructure",
				"24/7 dedicated support",
				"Custom SLA",
				"Volume discounts",
			},
			IsActive: true,
		},
		{
			ID:              "payg",
			Name:            "Pay As You Go",
			Description:     "Pay only for what you use",
			Type:            "usage",
			PriceMonthly:    0,     // No base fee
			BandwidthGB:     0,     // Pay per GB
			RequestsPerDay:  50000, // Fair use limit
			ConcurrentConns: 25,
			Features: []string{
				"$5 per GB",
				"No monthly commitment",
				"50,000 requests/day",
				"25 concurrent connections",
				"HTTP & SOCKS5 protocols",
				"Basic geo-targeting",
				"Email support",
			},
			IsActive: true,
		},
	}
}

// GetAll retrieves all active plans
func (s *PlanService) GetAll(ctx context.Context) ([]Plan, error) {
	// For now, return default plans
	// In production, would query from database
	return GetDefaultPlans(), nil
}

// GetByID retrieves a plan by ID
func (s *PlanService) GetByID(ctx context.Context, id string) (*Plan, error) {
	plans := GetDefaultPlans()
	for _, p := range plans {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, sql.ErrNoRows
}

// CreatePlan creates a new plan
func (s *PlanService) CreatePlan(ctx context.Context, plan *Plan) error {
	query := `
		INSERT INTO plans (id, name, description, type, stripe_price_id, price_monthly, price_annual,
			bandwidth_gb, requests_per_day, concurrent_conns, features, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	
	_, err := s.db.ExecContext(ctx, query,
		plan.ID, plan.Name, plan.Description, plan.Type, plan.StripePriceID,
		plan.PriceMonthly, plan.PriceAnnual, plan.BandwidthGB, plan.RequestsPerDay,
		plan.ConcurrentConns, plan.Features, plan.IsActive,
	)
	return err
}

// UpdatePlan updates an existing plan
func (s *PlanService) UpdatePlan(ctx context.Context, plan *Plan) error {
	query := `
		UPDATE plans SET
			name = $2, description = $3, type = $4, stripe_price_id = $5,
			price_monthly = $6, price_annual = $7, bandwidth_gb = $8,
			requests_per_day = $9, concurrent_conns = $10, features = $11,
			is_active = $12, updated_at = NOW()
		WHERE id = $1
	`
	
	_, err := s.db.ExecContext(ctx, query,
		plan.ID, plan.Name, plan.Description, plan.Type, plan.StripePriceID,
		plan.PriceMonthly, plan.PriceAnnual, plan.BandwidthGB, plan.RequestsPerDay,
		plan.ConcurrentConns, plan.Features, plan.IsActive,
	)
	return err
}
