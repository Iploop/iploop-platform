package usage

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-redis/redis/v8"
)

// UsageRecord represents a usage record
type UsageRecord struct {
	ID              string    `json:"id"`
	CustomerID      string    `json:"customer_id"`
	APIKeyID        string    `json:"api_key_id"`
	BytesTransfer   int64     `json:"bytes_transferred"`
	RequestCount    int64     `json:"request_count"`
	SuccessCount    int64     `json:"success_count"`
	ErrorCount      int64     `json:"error_count"`
	Country         string    `json:"country"`
	NodeID          string    `json:"node_id"`
	Timestamp       time.Time `json:"timestamp"`
	BillingPeriod   string    `json:"billing_period"` // YYYY-MM
}

// UsageSummary represents aggregated usage
type UsageSummary struct {
	CustomerID       string             `json:"customer_id"`
	Period           string             `json:"period"`
	TotalBytes       int64              `json:"total_bytes"`
	TotalRequests    int64              `json:"total_requests"`
	SuccessRate      float64            `json:"success_rate"`
	ByCountry        map[string]int64   `json:"by_country"`
	ByDay            map[string]int64   `json:"by_day"`
	EstimatedCost    float64            `json:"estimated_cost"`
	PlanLimit        int64              `json:"plan_limit_bytes"`
	UsagePercent     float64            `json:"usage_percent"`
}

// DailyUsage represents daily aggregated usage
type DailyUsage struct {
	Date          string  `json:"date"`
	BytesTransfer int64   `json:"bytes_transferred"`
	RequestCount  int64   `json:"request_count"`
	SuccessRate   float64 `json:"success_rate"`
}

// Tracker tracks and aggregates usage
type Tracker struct {
	db  *sql.DB
	rdb *redis.Client
}

// NewTracker creates a new usage tracker
func NewTracker(db *sql.DB, rdb *redis.Client) *Tracker {
	return &Tracker{
		db:  db,
		rdb: rdb,
	}
}

// RecordUsage records a usage event (called from proxy)
func (t *Tracker) RecordUsage(ctx context.Context, record *UsageRecord) error {
	// Store in Redis for real-time aggregation
	key := t.usageKey(record.CustomerID, record.BillingPeriod)
	
	pipe := t.rdb.Pipeline()
	pipe.HIncrBy(ctx, key, "bytes", record.BytesTransfer)
	pipe.HIncrBy(ctx, key, "requests", record.RequestCount)
	pipe.HIncrBy(ctx, key, "success", record.SuccessCount)
	pipe.HIncrBy(ctx, key, "errors", record.ErrorCount)
	pipe.HIncrBy(ctx, key+"_country:"+record.Country, "bytes", record.BytesTransfer)
	pipe.Expire(ctx, key, 45*24*time.Hour) // Keep for 45 days
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}
	
	// Also store in database for long-term storage (async)
	go t.storeInDatabase(record)
	
	return nil
}

// GetCurrentUsage gets current period usage for a customer
func (t *Tracker) GetCurrentUsage(ctx context.Context, customerID string) (*UsageSummary, error) {
	period := time.Now().Format("2006-01")
	return t.GetUsageForPeriod(ctx, customerID, period)
}

// GetUsageForPeriod gets usage for a specific billing period
func (t *Tracker) GetUsageForPeriod(ctx context.Context, customerID, period string) (*UsageSummary, error) {
	key := t.usageKey(customerID, period)
	
	// Get from Redis
	data, err := t.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	
	summary := &UsageSummary{
		CustomerID: customerID,
		Period:     period,
		ByCountry:  make(map[string]int64),
		ByDay:      make(map[string]int64),
	}
	
	if bytes, ok := data["bytes"]; ok {
		summary.TotalBytes = parseInt64(bytes)
	}
	if requests, ok := data["requests"]; ok {
		summary.TotalRequests = parseInt64(requests)
	}
	if success, ok := data["success"]; ok {
		successCount := parseInt64(success)
		if summary.TotalRequests > 0 {
			summary.SuccessRate = float64(successCount) / float64(summary.TotalRequests) * 100
		}
	}
	
	// Get country breakdown
	countryKeys, _ := t.rdb.Keys(ctx, key+"_country:*").Result()
	for _, ck := range countryKeys {
		country := ck[len(key+"_country:"):]
		bytes, _ := t.rdb.HGet(ctx, ck, "bytes").Int64()
		summary.ByCountry[country] = bytes
	}
	
	// Calculate estimated cost (for PAYG customers)
	// $5 per GB
	summary.EstimatedCost = float64(summary.TotalBytes) / (1024 * 1024 * 1024) * 5.0
	
	return summary, nil
}

// GetDailyUsage gets daily usage breakdown
func (t *Tracker) GetDailyUsage(ctx context.Context, customerID string, days int) ([]DailyUsage, error) {
	query := `
		SELECT 
			DATE(timestamp) as date,
			SUM(bytes_transferred) as bytes,
			SUM(request_count) as requests,
			CASE WHEN SUM(request_count) > 0 
				THEN SUM(success_count)::float / SUM(request_count) * 100 
				ELSE 0 
			END as success_rate
		FROM usage_records
		WHERE customer_id = $1 
			AND timestamp >= NOW() - INTERVAL '%d days'
		GROUP BY DATE(timestamp)
		ORDER BY date DESC
	`
	
	rows, err := t.db.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var usage []DailyUsage
	for rows.Next() {
		var u DailyUsage
		if err := rows.Scan(&u.Date, &u.BytesTransfer, &u.RequestCount, &u.SuccessRate); err != nil {
			continue
		}
		usage = append(usage, u)
	}
	
	return usage, nil
}

// CheckQuota checks if customer is within their plan quota
func (t *Tracker) CheckQuota(ctx context.Context, customerID string, planLimitBytes int64) (bool, float64, error) {
	summary, err := t.GetCurrentUsage(ctx, customerID)
	if err != nil {
		return false, 0, err
	}
	
	if planLimitBytes == 0 {
		return true, 0, nil // Unlimited
	}
	
	usagePercent := float64(summary.TotalBytes) / float64(planLimitBytes) * 100
	withinQuota := summary.TotalBytes < planLimitBytes
	
	return withinQuota, usagePercent, nil
}

// GetTopCountries gets top countries by usage
func (t *Tracker) GetTopCountries(ctx context.Context, customerID string, limit int) (map[string]int64, error) {
	period := time.Now().Format("2006-01")
	key := t.usageKey(customerID, period)
	
	result := make(map[string]int64)
	countryKeys, err := t.rdb.Keys(ctx, key+"_country:*").Result()
	if err != nil {
		return nil, err
	}
	
	for _, ck := range countryKeys {
		country := ck[len(key+"_country:"):]
		bytes, _ := t.rdb.HGet(ctx, ck, "bytes").Int64()
		result[country] = bytes
	}
	
	return result, nil
}

// Helper functions

func (t *Tracker) usageKey(customerID, period string) string {
	return "usage:" + customerID + ":" + period
}

func (t *Tracker) storeInDatabase(record *UsageRecord) {
	query := `
		INSERT INTO usage_records 
			(customer_id, api_key_id, bytes_transferred, request_count, 
			 success_count, error_count, country, node_id, timestamp, billing_period)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	t.db.ExecContext(ctx, query,
		record.CustomerID, record.APIKeyID, record.BytesTransfer, record.RequestCount,
		record.SuccessCount, record.ErrorCount, record.Country, record.NodeID,
		record.Timestamp, record.BillingPeriod,
	)
}

func parseInt64(s string) int64 {
	var result int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int64(c-'0')
		}
	}
	return result
}
