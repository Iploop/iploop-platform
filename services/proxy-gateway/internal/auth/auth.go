package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type Authenticator struct {
	db         *sql.DB
	rdb        *redis.Client
	planLoader *PlanLoader
}

type Customer struct {
	ID       string
	UserID   string
	Email    string
	GBBalance float64
	Plan     string
	Active   bool
}

type ProxyAuth struct {
	Customer     *Customer
	Country      string
	City         string
	SessionID    string
	SessionType  string // "sticky", "rotating", "per-request"
	Plan         *AccountPlan
	OriginalAuth string
}

func NewAuthenticator(db *sql.DB, rdb *redis.Client) *Authenticator {
	return &Authenticator{
		db:         db,
		rdb:        rdb,
		planLoader: NewPlanLoader(db, rdb),
	}
}

// ParseProxyAuth parses proxy authentication in the format:
// customer_id:api_key[@proxy.iploop.com:port]
// customer_id:api_key-country-us[@proxy.iploop.com:port]
// customer_id:api_key-country-us-city-newyork[@proxy.iploop.com:port]
// customer_id:api_key-session-abc123[@proxy.iploop.com:port]
func (a *Authenticator) ParseProxyAuth(authHeader string) (*ProxyAuth, error) {
	// Remove "Basic " prefix if present
	if strings.HasPrefix(authHeader, "Basic ") {
		authHeader = authHeader[6:]
		// Decode base64
		decoded, err := base64.StdEncoding.DecodeString(authHeader)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 encoding")
		}
		authHeader = string(decoded)
	}

	// For HTTP proxy, auth is now decoded (user:pass format)
	if strings.Contains(authHeader, ":") {
		parts := strings.SplitN(authHeader, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid auth format")
		}

		customerID := parts[0]
		keyAndParams := parts[1]

		// Parse targeting parameters from key
		auth := &ProxyAuth{
			OriginalAuth: authHeader,
		}

		// Split by dash to extract parameters
		keyParts := strings.Split(keyAndParams, "-")
		apiKey := keyParts[0]

		// Parse optional parameters
		for i := 1; i < len(keyParts)-1; i += 2 {
			if i+1 >= len(keyParts) {
				break
			}
			param := keyParts[i]
			value := keyParts[i+1]

			switch param {
			case "country":
				auth.Country = strings.ToUpper(value)
			case "city":
				auth.City = strings.ToLower(value)
			case "session":
				auth.SessionID = value
			case "sesstype", "stype":
				auth.SessionType = value
			}
		}

		// Authenticate customer
		customer, err := a.authenticateCustomer(customerID, apiKey)
		if err != nil {
			return nil, err
		}

		auth.Customer = customer

		// Load account plan and apply defaults
		if a.planLoader != nil && customer != nil {
			plan, err := a.planLoader.LoadPlan(customer.UserID)
			if err != nil {
				fmt.Printf("[AUTH] Warning: failed to load plan for user %s: %v\n", customer.UserID, err)
			} else {
				// Apply plan defaults (fills in missing params)
				ApplyPlanDefaults(auth, plan)

				// Check plan limits (geo restrictions, feature gates)
				if limitErr := CheckPlanLimits(auth, plan); limitErr != nil {
					return nil, fmt.Errorf("plan limit exceeded: %v", limitErr)
				}
			}
		}

		return auth, nil
	}

	return nil, fmt.Errorf("invalid auth format")
}

func (a *Authenticator) authenticateCustomer(customerID, apiKey string) (*Customer, error) {
	ctx := context.Background()

	// Hash the API key
	hasher := sha256.New()
	hasher.Write([]byte(apiKey))
	keyHash := hex.EncodeToString(hasher.Sum(nil))
	cacheKey := fmt.Sprintf("auth:%s:%s", customerID, apiKey)
	
	// Debug logging
	fmt.Printf("[AUTH DEBUG] customerID=%s, apiKey=%s, keyHash=%s\n", customerID, apiKey, keyHash)
	_ = ctx // avoid unused warning

	// Query database
	var customer Customer
	query := `
		SELECT 
			ak.id,
			u.id,
			u.email,
			COALESCE(up.gb_balance, 0) as gb_balance,
			p.name as plan,
			ak.is_active
		FROM api_keys ak
		JOIN users u ON ak.user_id = u.id
		LEFT JOIN user_plans up ON u.id = up.user_id AND up.status = 'active'
		LEFT JOIN plans p ON up.plan_id = p.id
		WHERE ak.key_hash = $1 AND ak.is_active = true AND u.status = 'active'
	`

	err := a.db.QueryRow(query, keyHash).Scan(
		&customer.ID,
		&customer.UserID,
		&customer.Email,
		&customer.GBBalance,
		&customer.Plan,
		&customer.Active,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("authentication error: %v", err)
	}

	if !customer.Active {
		return nil, fmt.Errorf("account suspended")
	}

	if customer.GBBalance <= 0 {
		return nil, fmt.Errorf("insufficient balance")
	}

	// Cache successful authentication for 5 minutes
	a.rdb.Set(ctx, cacheKey, "valid", 5*time.Minute)
	
	// Cache customer details
	customerKey := fmt.Sprintf("customer:%s", customer.ID)
	a.rdb.HMSet(ctx, customerKey, map[string]interface{}{
		"user_id":    customer.UserID,
		"email":      customer.Email,
		"gb_balance": customer.GBBalance,
		"plan":       customer.Plan,
		"active":     customer.Active,
	})
	a.rdb.Expire(ctx, customerKey, 5*time.Minute)

	// Update last_used_at for API key
	go func() {
		_, err := a.db.Exec("UPDATE api_keys SET last_used_at = NOW() WHERE key_hash = $1", keyHash)
		if err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to update api key last_used_at: %v\n", err)
		}
	}()

	return &customer, nil
}

func (a *Authenticator) getCustomerFromCache(customerID string) (*Customer, error) {
	ctx := context.Background()
	customerKey := fmt.Sprintf("customer:%s", customerID)

	result, err := a.rdb.HMGet(ctx, customerKey,
		"user_id", "email", "gb_balance", "plan", "active").Result()
	if err != nil {
		return nil, fmt.Errorf("customer not found in cache")
	}

	customer := &Customer{
		ID:     customerID,
		UserID: result[0].(string),
		Email:  result[1].(string),
		Plan:   result[3].(string),
		Active: result[4].(string) == "true",
	}

	// Parse gb_balance
	if result[2] != nil {
		fmt.Sscanf(result[2].(string), "%f", &customer.GBBalance)
	}

	return customer, nil
}

// RecordUsage records bandwidth usage for billing
func (a *Authenticator) RecordUsage(customerID string, bytesUsed int64, nodeID string, success bool, extras ...string) error {
	// extras[0] = country, extras[1] = target_host
	country := ""
	targetHost := ""
	if len(extras) > 0 {
		country = extras[0]
	}
	if len(extras) > 1 {
		targetHost = extras[1]
	}

	// Insert usage record
	query := `
		INSERT INTO usage_records (
			user_id, 
			node_id, 
			bytes_downloaded, 
			target_country,
			target_host,
			success,
			ended_at
		) VALUES (
			(SELECT user_id FROM api_keys WHERE id = $1),
			$2,
			$3,
			$4,
			$5,
			$6,
			NOW()
		)
	`
	
	_, err := a.db.Exec(query, customerID, nodeID, bytesUsed, country, targetHost, success)
	if err != nil {
		return fmt.Errorf("failed to record usage: %v", err)
	}

	// Update user balance (async)
	go func() {
		gbUsed := float64(bytesUsed) / (1024 * 1024 * 1024)
		updateQuery := `
			UPDATE user_plans 
			SET gb_used = gb_used + $1, gb_balance = gb_balance - $1
			WHERE user_id = (SELECT user_id FROM api_keys WHERE id = $2)
			AND status = 'active'
		`
		_, err := a.db.Exec(updateQuery, gbUsed, customerID)
		if err != nil {
			fmt.Printf("Failed to update user balance: %v\n", err)
		}
	}()

	return nil
}