package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RateLimiter provides rate limiting using Redis
type RateLimiter struct {
	rdb *redis.Client
}

// Config holds rate limit configuration
type Config struct {
	RequestsPerMinute int
	RequestsPerHour   int
	RequestsPerDay    int
	BurstSize         int
}

// DefaultConfig returns default rate limit configuration
func DefaultConfig() *Config {
	return &Config{
		RequestsPerMinute: 60,
		RequestsPerHour:   1000,
		RequestsPerDay:    10000,
		BurstSize:         10,
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	return &RateLimiter{rdb: rdb}
}

// Result contains rate limit check result
type Result struct {
	Allowed    bool
	Remaining  int64
	ResetAt    time.Time
	RetryAfter time.Duration
}

// Check checks if a request is allowed for the given customer
func (rl *RateLimiter) Check(ctx context.Context, customerID string, config *Config) (*Result, error) {
	if config == nil {
		config = DefaultConfig()
	}

	now := time.Now()
	
	// Check per-minute limit (sliding window)
	minuteKey := fmt.Sprintf("ratelimit:minute:%s", customerID)
	minuteCount, err := rl.incrementWindow(ctx, minuteKey, time.Minute)
	if err != nil {
		return nil, err
	}

	if minuteCount > int64(config.RequestsPerMinute) {
		return &Result{
			Allowed:    false,
			Remaining:  0,
			ResetAt:    now.Add(time.Minute).Truncate(time.Minute),
			RetryAfter: time.Until(now.Add(time.Minute).Truncate(time.Minute)),
		}, nil
	}

	// Check per-hour limit
	hourKey := fmt.Sprintf("ratelimit:hour:%s", customerID)
	hourCount, err := rl.incrementWindow(ctx, hourKey, time.Hour)
	if err != nil {
		return nil, err
	}

	if hourCount > int64(config.RequestsPerHour) {
		return &Result{
			Allowed:    false,
			Remaining:  0,
			ResetAt:    now.Add(time.Hour).Truncate(time.Hour),
			RetryAfter: time.Until(now.Add(time.Hour).Truncate(time.Hour)),
		}, nil
	}

	// Check per-day limit
	dayKey := fmt.Sprintf("ratelimit:day:%s", customerID)
	dayCount, err := rl.incrementWindow(ctx, dayKey, 24*time.Hour)
	if err != nil {
		return nil, err
	}

	if dayCount > int64(config.RequestsPerDay) {
		return &Result{
			Allowed:    false,
			Remaining:  0,
			ResetAt:    now.Add(24 * time.Hour).Truncate(24 * time.Hour),
			RetryAfter: time.Until(now.Add(24 * time.Hour).Truncate(24 * time.Hour)),
		}, nil
	}

	remaining := int64(config.RequestsPerMinute) - minuteCount
	if remaining < 0 {
		remaining = 0
	}

	return &Result{
		Allowed:   true,
		Remaining: remaining,
		ResetAt:   now.Add(time.Minute).Truncate(time.Minute),
	}, nil
}

func (rl *RateLimiter) incrementWindow(ctx context.Context, key string, window time.Duration) (int64, error) {
	pipe := rl.rdb.Pipeline()
	
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	return incr.Val(), nil
}

// GetUsage returns current rate limit usage for a customer
func (rl *RateLimiter) GetUsage(ctx context.Context, customerID string) (map[string]int64, error) {
	minuteKey := fmt.Sprintf("ratelimit:minute:%s", customerID)
	hourKey := fmt.Sprintf("ratelimit:hour:%s", customerID)
	dayKey := fmt.Sprintf("ratelimit:day:%s", customerID)

	minuteCount, _ := rl.rdb.Get(ctx, minuteKey).Int64()
	hourCount, _ := rl.rdb.Get(ctx, hourKey).Int64()
	dayCount, _ := rl.rdb.Get(ctx, dayKey).Int64()

	return map[string]int64{
		"per_minute": minuteCount,
		"per_hour":   hourCount,
		"per_day":    dayCount,
	}, nil
}

// Reset resets rate limits for a customer (admin use)
func (rl *RateLimiter) Reset(ctx context.Context, customerID string) error {
	minuteKey := fmt.Sprintf("ratelimit:minute:%s", customerID)
	hourKey := fmt.Sprintf("ratelimit:hour:%s", customerID)
	dayKey := fmt.Sprintf("ratelimit:day:%s", customerID)

	return rl.rdb.Del(ctx, minuteKey, hourKey, dayKey).Err()
}
