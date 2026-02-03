package metrics

import (
	"sync"
	"time"
)

type Collector struct {
	mutex    sync.RWMutex
	requests map[string]*CustomerStats
}

type CustomerStats struct {
	CustomerID     string            `json:"customer_id"`
	TotalRequests  int64             `json:"total_requests"`
	SuccessRequests int64            `json:"success_requests"`
	FailedRequests int64             `json:"failed_requests"`
	TotalBytes     int64             `json:"total_bytes"`
	AvgDuration    time.Duration     `json:"avg_duration"`
	Countries      map[string]int64  `json:"countries"`
	LastRequest    time.Time         `json:"last_request"`
}

type OverallStats struct {
	TotalRequests   int64                      `json:"total_requests"`
	SuccessRequests int64                      `json:"success_requests"`
	FailedRequests  int64                      `json:"failed_requests"`
	TotalBytes      int64                      `json:"total_bytes"`
	ActiveCustomers int64                      `json:"active_customers"`
	Countries       map[string]int64           `json:"countries"`
	Customers       map[string]*CustomerStats  `json:"customers"`
	Uptime          time.Duration              `json:"uptime"`
	StartTime       time.Time                  `json:"start_time"`
}

func NewCollector() *Collector {
	return &Collector{
		requests: make(map[string]*CustomerStats),
	}
}

func (c *Collector) RecordRequest(customerID, country string, duration time.Duration, success bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	stats, exists := c.requests[customerID]
	if !exists {
		stats = &CustomerStats{
			CustomerID: customerID,
			Countries:  make(map[string]int64),
		}
		c.requests[customerID] = stats
	}

	stats.TotalRequests++
	stats.LastRequest = time.Now()

	if success {
		stats.SuccessRequests++
	} else {
		stats.FailedRequests++
	}

	if country != "" {
		stats.Countries[country]++
	}

	// Update average duration (simple moving average)
	if stats.TotalRequests == 1 {
		stats.AvgDuration = duration
	} else {
		// Weighted average: (old_avg * (n-1) + new_duration) / n
		oldWeight := float64(stats.TotalRequests - 1)
		newWeight := 1.0
		totalWeight := oldWeight + newWeight
		
		oldAvgMs := float64(stats.AvgDuration.Milliseconds())
		newDurationMs := float64(duration.Milliseconds())
		
		newAvgMs := (oldAvgMs*oldWeight + newDurationMs*newWeight) / totalWeight
		stats.AvgDuration = time.Duration(newAvgMs) * time.Millisecond
	}
}

func (c *Collector) RecordBandwidth(customerID string, bytes int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	stats, exists := c.requests[customerID]
	if !exists {
		stats = &CustomerStats{
			CustomerID: customerID,
			Countries:  make(map[string]int64),
		}
		c.requests[customerID] = stats
	}

	stats.TotalBytes += bytes
}

func (c *Collector) GetCustomerStats(customerID string) *CustomerStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	stats, exists := c.requests[customerID]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	statsCopy := &CustomerStats{
		CustomerID:      stats.CustomerID,
		TotalRequests:   stats.TotalRequests,
		SuccessRequests: stats.SuccessRequests,
		FailedRequests:  stats.FailedRequests,
		TotalBytes:      stats.TotalBytes,
		AvgDuration:     stats.AvgDuration,
		Countries:       make(map[string]int64),
		LastRequest:     stats.LastRequest,
	}

	for country, count := range stats.Countries {
		statsCopy.Countries[country] = count
	}

	return statsCopy
}

func (c *Collector) GetStats() *OverallStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	overall := &OverallStats{
		Countries:   make(map[string]int64),
		Customers:   make(map[string]*CustomerStats),
		StartTime:   time.Now(), // This should be set when the collector is created
		ActiveCustomers: int64(len(c.requests)),
	}

	// Calculate uptime (simplified - in production you'd track actual start time)
	overall.Uptime = time.Since(overall.StartTime)

	// Aggregate stats across all customers
	for customerID, stats := range c.requests {
		overall.TotalRequests += stats.TotalRequests
		overall.SuccessRequests += stats.SuccessRequests
		overall.FailedRequests += stats.FailedRequests
		overall.TotalBytes += stats.TotalBytes

		// Copy customer stats
		customerCopy := &CustomerStats{
			CustomerID:      stats.CustomerID,
			TotalRequests:   stats.TotalRequests,
			SuccessRequests: stats.SuccessRequests,
			FailedRequests:  stats.FailedRequests,
			TotalBytes:      stats.TotalBytes,
			AvgDuration:     stats.AvgDuration,
			Countries:       make(map[string]int64),
			LastRequest:     stats.LastRequest,
		}

		for country, count := range stats.Countries {
			customerCopy.Countries[country] = count
			overall.Countries[country] += count
		}

		overall.Customers[customerID] = customerCopy
	}

	return overall
}

func (c *Collector) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.requests = make(map[string]*CustomerStats)
}

func (c *Collector) GetTopCountries(limit int) map[string]int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	countries := make(map[string]int64)
	
	for _, stats := range c.requests {
		for country, count := range stats.Countries {
			countries[country] += count
		}
	}

	// For MVP, return all countries (sorting would be added in production)
	return countries
}

func (c *Collector) GetActiveCustomers() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	customers := make([]string, 0, len(c.requests))
	for customerID := range c.requests {
		customers = append(customers, customerID)
	}

	return customers
}