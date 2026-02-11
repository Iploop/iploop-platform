package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
)

// AccountPlan represents the per-account proxy configuration
type AccountPlan struct {
	ID                     string   `json:"id"`
	UserID                 string   `json:"user_id"`
	PlanName               string   `json:"plan_name"`
	DefaultRotationMode    string   `json:"default_rotation_mode"`
	DefaultSessionTTL      int      `json:"default_session_ttl"`
	PoolQualityTier        string   `json:"pool_quality_tier"`
	MinNodeQualityScore    int      `json:"min_node_quality_score"`
	MaxConcurrency         int      `json:"max_concurrency"`
	BandwidthCapDailyMB    int64    `json:"bandwidth_cap_daily_mb"`
	BandwidthCapMonthlyMB  int64    `json:"bandwidth_cap_monthly_mb"`
	AllowedCountries       []string `json:"allowed_countries"`
	BlockedCountries       []string `json:"blocked_countries"`
	IPReuseCooldownSeconds int      `json:"ip_reuse_cooldown_seconds"`
	StickySessionsEnabled  bool     `json:"sticky_sessions_enabled"`
	GeoTargetingEnabled    bool     `json:"geo_targeting_enabled"`
	CityTargetingEnabled   bool     `json:"city_targeting_enabled"`
	IsActive               bool     `json:"is_active"`
}

// PlanLoader handles loading and caching of account plans
type PlanLoader struct {
	db  *sql.DB
	rdb *redis.Client
}

// NewPlanLoader creates a new PlanLoader
func NewPlanLoader(db *sql.DB, rdb *redis.Client) *PlanLoader {
	return &PlanLoader{
		db:  db,
		rdb: rdb,
	}
}

const planCacheTTL = 5 * time.Minute
const planCachePrefix = "account_plan:"

// LoadPlan loads a user's account plan, using Redis cache first
func (pl *PlanLoader) LoadPlan(userID string) (*AccountPlan, error) {
	ctx := context.Background()

	// Try cache first
	cacheKey := planCachePrefix + userID
	cached, err := pl.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var plan AccountPlan
		if err := json.Unmarshal([]byte(cached), &plan); err == nil {
			return &plan, nil
		}
	}

	// Load from database
	plan, err := pl.loadPlanFromDB(userID)
	if err != nil {
		// If no plan exists, return defaults
		if err == sql.ErrNoRows {
			plan = pl.defaultPlan(userID)
		} else {
			return nil, fmt.Errorf("failed to load account plan: %v", err)
		}
	}

	// Cache the result
	planJSON, err := json.Marshal(plan)
	if err == nil {
		pl.rdb.Set(ctx, cacheKey, string(planJSON), planCacheTTL)
	}

	return plan, nil
}

// loadPlanFromDB loads the account plan directly from PostgreSQL
func (pl *PlanLoader) loadPlanFromDB(userID string) (*AccountPlan, error) {
	plan := &AccountPlan{}

	query := `
		SELECT 
			id, user_id, plan_name,
			default_rotation_mode, default_session_ttl,
			pool_quality_tier, min_node_quality_score,
			max_concurrency, bandwidth_cap_daily_mb, bandwidth_cap_monthly_mb,
			allowed_countries, blocked_countries,
			ip_reuse_cooldown_seconds,
			sticky_sessions_enabled, geo_targeting_enabled, city_targeting_enabled,
			is_active
		FROM account_plans
		WHERE user_id = $1
	`

	var allowedCountries, blockedCountries pq.StringArray

	err := pl.db.QueryRow(query, userID).Scan(
		&plan.ID, &plan.UserID, &plan.PlanName,
		&plan.DefaultRotationMode, &plan.DefaultSessionTTL,
		&plan.PoolQualityTier, &plan.MinNodeQualityScore,
		&plan.MaxConcurrency, &plan.BandwidthCapDailyMB, &plan.BandwidthCapMonthlyMB,
		&allowedCountries, &blockedCountries,
		&plan.IPReuseCooldownSeconds,
		&plan.StickySessionsEnabled, &plan.GeoTargetingEnabled, &plan.CityTargetingEnabled,
		&plan.IsActive,
	)

	if err != nil {
		return nil, err
	}

	plan.AllowedCountries = []string(allowedCountries)
	plan.BlockedCountries = []string(blockedCountries)

	return plan, nil
}

// defaultPlan returns the default plan for users without an explicit plan
func (pl *PlanLoader) defaultPlan(userID string) *AccountPlan {
	return &AccountPlan{
		UserID:                 userID,
		PlanName:               "default",
		DefaultRotationMode:    "rotating",
		DefaultSessionTTL:      30,
		PoolQualityTier:        "standard",
		MinNodeQualityScore:    0,
		MaxConcurrency:         100,
		BandwidthCapDailyMB:    0,
		BandwidthCapMonthlyMB:  0,
		AllowedCountries:       []string{},
		BlockedCountries:       []string{},
		IPReuseCooldownSeconds: 0,
		StickySessionsEnabled:  true,
		GeoTargetingEnabled:    true,
		CityTargetingEnabled:   true,
		IsActive:               true,
	}
}

// InvalidateCache removes the cached plan for a user (call after plan updates)
func (pl *PlanLoader) InvalidateCache(userID string) error {
	ctx := context.Background()
	cacheKey := planCachePrefix + userID
	return pl.rdb.Del(ctx, cacheKey).Err()
}

// ApplyPlanDefaults applies plan defaults to a ProxyAuth when the client didn't specify params
func ApplyPlanDefaults(auth *ProxyAuth, plan *AccountPlan) {
	if auth == nil || plan == nil {
		return
	}

	// Apply default rotation mode if not specified by client
	if auth.SessionType == "" {
		auth.SessionType = plan.DefaultRotationMode
	}

	// Store plan reference on the auth
	auth.Plan = plan
}

// CheckPlanLimits validates the request against plan limits
// Returns nil if OK, error describing violation otherwise
func CheckPlanLimits(auth *ProxyAuth, plan *AccountPlan) error {
	if plan == nil {
		return nil
	}

	if !plan.IsActive {
		return fmt.Errorf("account plan is inactive")
	}

	// Check geo targeting permission
	if auth.Country != "" && !plan.GeoTargetingEnabled {
		return fmt.Errorf("geo targeting is not enabled for this account")
	}

	// Check city targeting permission
	if auth.City != "" && !plan.CityTargetingEnabled {
		return fmt.Errorf("city targeting is not enabled for this account")
	}

	// Check sticky sessions permission
	if auth.SessionType == "sticky" && !plan.StickySessionsEnabled {
		return fmt.Errorf("sticky sessions are not enabled for this account")
	}

	// Check allowed countries (empty means all allowed)
	if auth.Country != "" && len(plan.AllowedCountries) > 0 {
		found := false
		for _, c := range plan.AllowedCountries {
			if strings.EqualFold(c, auth.Country) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("country %s is not in your allowed countries list", auth.Country)
		}
	}

	// Check blocked countries
	if auth.Country != "" && len(plan.BlockedCountries) > 0 {
		for _, c := range plan.BlockedCountries {
			if strings.EqualFold(c, auth.Country) {
				return fmt.Errorf("country %s is blocked for this account", auth.Country)
			}
		}
	}

	return nil
}
