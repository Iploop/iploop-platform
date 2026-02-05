package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type AnalyticsManager struct {
	db         *sql.DB
	rdb        *redis.Client
	logger     *logrus.Entry
	
	// Real-time metrics
	realTimeMetrics sync.Map // customerID -> CustomerMetrics
	
	// Batch processing
	batchSize    int
	batchBuffer  chan MetricRecord
	batchTicker  *time.Ticker
}

type CustomerMetrics struct {
	CustomerID         string    `json:"customer_id"`
	LastUpdated        time.Time `json:"last_updated"`
	
	// Current period (last hour)
	CurrentHour        HourlyMetrics `json:"current_hour"`
	
	// Session tracking
	ActiveSessions     int       `json:"active_sessions"`
	TotalSessions      int       `json:"total_sessions"`
	
	// Performance
	AvgLatency         float64   `json:"avg_latency_ms"`
	SuccessRate        float64   `json:"success_rate"`
	
	// Geographic distribution
	TopCountries       []CountryUsage `json:"top_countries"`
	TopCities          []CityUsage    `json:"top_cities"`
	
	mutex             sync.RWMutex `json:"-"`
}

type HourlyMetrics struct {
	Timestamp        time.Time `json:"timestamp"`
	RequestCount     int64     `json:"request_count"`
	BytesTransferred int64     `json:"bytes_transferred"`
	SuccessfulReqs   int64     `json:"successful_requests"`
	FailedReqs       int64     `json:"failed_requests"`
	UniqueIPs        int       `json:"unique_target_ips"`
	AvgLatency       float64   `json:"avg_latency_ms"`
	ErrorsByType     map[string]int64 `json:"errors_by_type"`
}

type CountryUsage struct {
	Country    string `json:"country"`
	Requests   int64  `json:"requests"`
	Bytes      int64  `json:"bytes"`
	Percentage float64 `json:"percentage"`
}

type CityUsage struct {
	City       string `json:"city"`
	Country    string `json:"country"`
	Requests   int64  `json:"requests"`
	Bytes      int64  `json:"bytes"`
	Percentage float64 `json:"percentage"`
}

type MetricRecord struct {
	CustomerID       string    `json:"customer_id"`
	SessionID        string    `json:"session_id"`
	NodeID           string    `json:"node_id"`
	Timestamp        time.Time `json:"timestamp"`
	
	// Request details
	Method           string    `json:"method"`
	TargetHost       string    `json:"target_host"`
	TargetPort       string    `json:"target_port"`
	TargetCountry    string    `json:"target_country"`
	TargetCity       string    `json:"target_city"`
	
	// Performance
	LatencyMs        int64     `json:"latency_ms"`
	BytesRequest     int64     `json:"bytes_request"`
	BytesResponse    int64     `json:"bytes_response"`
	StatusCode       int       `json:"status_code"`
	Success          bool      `json:"success"`
	ErrorType        string    `json:"error_type,omitempty"`
	
	// Session info
	SessionType      string    `json:"session_type"`
	AuthMethod       string    `json:"auth_method"`
	Protocol         string    `json:"protocol"`
	
	// Node info
	NodeCountry      string    `json:"node_country"`
	NodeCity         string    `json:"node_city"`
	NodeSpeed        int       `json:"node_speed"`
}

func NewAnalyticsManager(db *sql.DB, rdb *redis.Client, logger *logrus.Entry) *AnalyticsManager {
	am := &AnalyticsManager{
		db:          db,
		rdb:         rdb,
		logger:      logger.WithField("component", "analytics"),
		batchSize:   1000,
		batchBuffer: make(chan MetricRecord, 10000),
	}
	
	// Start background processing
	am.startBatchProcessor()
	am.startMetricsAggregator()
	
	return am
}

func (am *AnalyticsManager) RecordMetric(record MetricRecord) {
	// Add to batch buffer
	select {
	case am.batchBuffer <- record:
		// Successfully buffered
	default:
		// Buffer full, log warning
		am.logger.Warnf("Analytics batch buffer full, dropping metric for customer %s", record.CustomerID)
	}
	
	// Update real-time metrics
	am.updateRealTimeMetrics(record)
}

func (am *AnalyticsManager) updateRealTimeMetrics(record MetricRecord) {
	// Load or create customer metrics
	var metrics *CustomerMetrics
	if data, exists := am.realTimeMetrics.Load(record.CustomerID); exists {
		metrics = data.(*CustomerMetrics)
	} else {
		metrics = &CustomerMetrics{
			CustomerID:    record.CustomerID,
			LastUpdated:   time.Now(),
			CurrentHour:   am.initHourlyMetrics(),
			TopCountries:  make([]CountryUsage, 0),
			TopCities:     make([]CityUsage, 0),
		}
		am.realTimeMetrics.Store(record.CustomerID, metrics)
	}
	
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()
	
	// Update hourly metrics
	metrics.CurrentHour.RequestCount++
	metrics.CurrentHour.BytesTransferred += record.BytesRequest + record.BytesResponse
	
	if record.Success {
		metrics.CurrentHour.SuccessfulReqs++
	} else {
		metrics.CurrentHour.FailedReqs++
		if record.ErrorType != "" {
			if metrics.CurrentHour.ErrorsByType == nil {
				metrics.CurrentHour.ErrorsByType = make(map[string]int64)
			}
			metrics.CurrentHour.ErrorsByType[record.ErrorType]++
		}
	}
	
	// Update average latency (simple moving average)
	if metrics.CurrentHour.RequestCount == 1 {
		metrics.CurrentHour.AvgLatency = float64(record.LatencyMs)
	} else {
		// Weighted average: (old_avg * (count-1) + new_value) / count
		metrics.CurrentHour.AvgLatency = (metrics.CurrentHour.AvgLatency*float64(metrics.CurrentHour.RequestCount-1) + float64(record.LatencyMs)) / float64(metrics.CurrentHour.RequestCount)
	}
	
	// Update overall metrics
	metrics.LastUpdated = time.Now()
	if metrics.CurrentHour.RequestCount > 0 {
		metrics.SuccessRate = float64(metrics.CurrentHour.SuccessfulReqs) / float64(metrics.CurrentHour.RequestCount) * 100
	}
	metrics.AvgLatency = metrics.CurrentHour.AvgLatency
	
	// Update geographic stats
	am.updateGeographicStats(metrics, record)
}

func (am *AnalyticsManager) updateGeographicStats(metrics *CustomerMetrics, record MetricRecord) {
	if record.NodeCountry == "" {
		return
	}
	
	// Update country stats
	found := false
	for i, country := range metrics.TopCountries {
		if country.Country == record.NodeCountry {
			metrics.TopCountries[i].Requests++
			metrics.TopCountries[i].Bytes += record.BytesRequest + record.BytesResponse
			found = true
			break
		}
	}
	
	if !found {
		metrics.TopCountries = append(metrics.TopCountries, CountryUsage{
			Country:  record.NodeCountry,
			Requests: 1,
			Bytes:    record.BytesRequest + record.BytesResponse,
		})
	}
	
	// Sort and limit to top 10
	sort.Slice(metrics.TopCountries, func(i, j int) bool {
		return metrics.TopCountries[i].Requests > metrics.TopCountries[j].Requests
	})
	if len(metrics.TopCountries) > 10 {
		metrics.TopCountries = metrics.TopCountries[:10]
	}
	
	// Calculate percentages
	totalRequests := metrics.CurrentHour.RequestCount
	for i := range metrics.TopCountries {
		metrics.TopCountries[i].Percentage = float64(metrics.TopCountries[i].Requests) / float64(totalRequests) * 100
	}
	
	// Similar logic for cities
	if record.NodeCity != "" {
		found := false
		for i, city := range metrics.TopCities {
			if city.City == record.NodeCity && city.Country == record.NodeCountry {
				metrics.TopCities[i].Requests++
				metrics.TopCities[i].Bytes += record.BytesRequest + record.BytesResponse
				found = true
				break
			}
		}
		
		if !found {
			metrics.TopCities = append(metrics.TopCities, CityUsage{
				City:     record.NodeCity,
				Country:  record.NodeCountry,
				Requests: 1,
				Bytes:    record.BytesRequest + record.BytesResponse,
			})
		}
		
		sort.Slice(metrics.TopCities, func(i, j int) bool {
			return metrics.TopCities[i].Requests > metrics.TopCities[j].Requests
		})
		if len(metrics.TopCities) > 10 {
			metrics.TopCities = metrics.TopCities[:10]
		}
		
		for i := range metrics.TopCities {
			metrics.TopCities[i].Percentage = float64(metrics.TopCities[i].Requests) / float64(totalRequests) * 100
		}
	}
}

func (am *AnalyticsManager) startBatchProcessor() {
	go func() {
		batch := make([]MetricRecord, 0, am.batchSize)
		
		for {
			select {
			case record := <-am.batchBuffer:
				batch = append(batch, record)
				
				if len(batch) >= am.batchSize {
					am.processBatch(batch)
					batch = batch[:0] // Reset slice
				}
				
			case <-time.After(30 * time.Second):
				// Process partial batch on timeout
				if len(batch) > 0 {
					am.processBatch(batch)
					batch = batch[:0]
				}
			}
		}
	}()
}

func (am *AnalyticsManager) processBatch(batch []MetricRecord) {
	if len(batch) == 0 {
		return
	}
	
	// Prepare bulk insert
	query := `
		INSERT INTO analytics_records (
			customer_id, session_id, node_id, timestamp,
			method, target_host, target_port, target_country, target_city,
			latency_ms, bytes_request, bytes_response, status_code, success, error_type,
			session_type, auth_method, protocol,
			node_country, node_city, node_speed
		) VALUES `
	
	values := make([]interface{}, 0, len(batch)*21)
	placeholders := make([]string, 0, len(batch))
	
	for i, record := range batch {
		placeholder := "("
		for j := 0; j < 21; j++ {
			if j > 0 {
				placeholder += ","
			}
			placeholder += fmt.Sprintf("$%d", i*21+j+1)
		}
		placeholder += ")"
		placeholders = append(placeholders, placeholder)
		
		values = append(values,
			record.CustomerID, record.SessionID, record.NodeID, record.Timestamp,
			record.Method, record.TargetHost, record.TargetPort, record.TargetCountry, record.TargetCity,
			record.LatencyMs, record.BytesRequest, record.BytesResponse, record.StatusCode, record.Success, record.ErrorType,
			record.SessionType, record.AuthMethod, record.Protocol,
			record.NodeCountry, record.NodeCity, record.NodeSpeed,
		)
	}
	
	query += strings.Join(placeholders, ",")
	
	_, err := am.db.Exec(query, values...)
	if err != nil {
		am.logger.Errorf("Failed to insert analytics batch: %v", err)
	} else {
		am.logger.Debugf("Processed analytics batch of %d records", len(batch))
	}
}

func (am *AnalyticsManager) startMetricsAggregator() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				am.aggregateHourlyMetrics()
			}
		}
	}()
}

func (am *AnalyticsManager) aggregateHourlyMetrics() {
	now := time.Now()
	hourStart := now.Truncate(time.Hour)
	
	// Aggregate metrics from the last hour
	query := `
		SELECT 
			customer_id,
			COUNT(*) as request_count,
			SUM(bytes_request + bytes_response) as bytes_total,
			SUM(CASE WHEN success THEN 1 ELSE 0 END) as successful_requests,
			SUM(CASE WHEN NOT success THEN 1 ELSE 0 END) as failed_requests,
			AVG(latency_ms) as avg_latency,
			COUNT(DISTINCT target_host) as unique_ips
		FROM analytics_records 
		WHERE timestamp >= $1 AND timestamp < $2
		GROUP BY customer_id
	`
	
	rows, err := am.db.Query(query, hourStart, hourStart.Add(time.Hour))
	if err != nil {
		am.logger.Errorf("Failed to aggregate hourly metrics: %v", err)
		return
	}
	defer rows.Close()
	
	for rows.Next() {
		var customerID string
		var requestCount, bytesTotal, successfulReqs, failedReqs, uniqueIPs int64
		var avgLatency float64
		
		err := rows.Scan(&customerID, &requestCount, &bytesTotal, &successfulReqs, &failedReqs, &avgLatency, &uniqueIPs)
		if err != nil {
			am.logger.Errorf("Failed to scan hourly metrics: %v", err)
			continue
		}
		
		// Store in Redis for quick access
		hourlyData := HourlyMetrics{
			Timestamp:        hourStart,
			RequestCount:     requestCount,
			BytesTransferred: bytesTotal,
			SuccessfulReqs:   successfulReqs,
			FailedReqs:       failedReqs,
			UniqueIPs:        int(uniqueIPs),
			AvgLatency:       avgLatency,
		}
		
		key := fmt.Sprintf("hourly:%s:%d", customerID, hourStart.Unix())
		data, _ := json.Marshal(hourlyData)
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		am.rdb.Set(ctx, key, data, 48*time.Hour) // Keep for 48 hours
		cancel()
	}
}

// GetCustomerMetrics returns real-time metrics for a customer
func (am *AnalyticsManager) GetCustomerMetrics(customerID string) (*CustomerMetrics, error) {
	if data, exists := am.realTimeMetrics.Load(customerID); exists {
		metrics := data.(*CustomerMetrics)
		metrics.mutex.RLock()
		defer metrics.mutex.RUnlock()
		
		// Return a copy to prevent external modifications
		copy := *metrics
		copy.TopCountries = make([]CountryUsage, len(metrics.TopCountries))
		copy(copy.TopCountries, metrics.TopCountries)
		copy.TopCities = make([]CityUsage, len(metrics.TopCities))
		copy(copy.TopCities, metrics.TopCities)
		
		return &copy, nil
	}
	
	return nil, fmt.Errorf("no metrics found for customer %s", customerID)
}

// GetHourlyReport generates a historical hourly report
func (am *AnalyticsManager) GetHourlyReport(customerID string, hours int) ([]HourlyMetrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	metrics := make([]HourlyMetrics, 0, hours)
	now := time.Now()
	
	for i := hours - 1; i >= 0; i-- {
		hour := now.Add(time.Duration(-i) * time.Hour).Truncate(time.Hour)
		key := fmt.Sprintf("hourly:%s:%d", customerID, hour.Unix())
		
		data, err := am.rdb.Get(ctx, key).Result()
		if err == redis.Nil {
			// No data for this hour, add empty metrics
			metrics = append(metrics, HourlyMetrics{
				Timestamp: hour,
			})
			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to get hourly data: %v", err)
		}
		
		var hourlyMetrics HourlyMetrics
		if err := json.Unmarshal([]byte(data), &hourlyMetrics); err != nil {
			am.logger.Errorf("Failed to unmarshal hourly metrics: %v", err)
			continue
		}
		
		metrics = append(metrics, hourlyMetrics)
	}
	
	return metrics, nil
}

// GetTopDestinations returns most accessed destinations for a customer
func (am *AnalyticsManager) GetTopDestinations(customerID string, limit int) ([]DestinationStats, error) {
	query := `
		SELECT 
			target_host,
			target_country,
			COUNT(*) as request_count,
			SUM(bytes_request + bytes_response) as bytes_total,
			AVG(latency_ms) as avg_latency,
			SUM(CASE WHEN success THEN 1 ELSE 0 END)::float / COUNT(*) * 100 as success_rate
		FROM analytics_records 
		WHERE customer_id = $1 
		AND timestamp >= NOW() - INTERVAL '24 hours'
		GROUP BY target_host, target_country
		ORDER BY request_count DESC 
		LIMIT $2
	`
	
	rows, err := am.db.Query(query, customerID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top destinations: %v", err)
	}
	defer rows.Close()
	
	destinations := make([]DestinationStats, 0, limit)
	
	for rows.Next() {
		var dest DestinationStats
		err := rows.Scan(&dest.Host, &dest.Country, &dest.RequestCount, &dest.BytesTotal, &dest.AvgLatency, &dest.SuccessRate)
		if err != nil {
			am.logger.Errorf("Failed to scan destination stats: %v", err)
			continue
		}
		
		destinations = append(destinations, dest)
	}
	
	return destinations, nil
}

type DestinationStats struct {
	Host         string  `json:"host"`
	Country      string  `json:"country"`
	RequestCount int64   `json:"request_count"`
	BytesTotal   int64   `json:"bytes_total"`
	AvgLatency   float64 `json:"avg_latency"`
	SuccessRate  float64 `json:"success_rate"`
}

func (am *AnalyticsManager) initHourlyMetrics() HourlyMetrics {
	return HourlyMetrics{
		Timestamp:    time.Now().Truncate(time.Hour),
		ErrorsByType: make(map[string]int64),
	}
}

// GetSystemStats returns overall system performance stats
func (am *AnalyticsManager) GetSystemStats() (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_requests,
			COUNT(DISTINCT customer_id) as active_customers,
			SUM(bytes_request + bytes_response) as total_bytes,
			AVG(latency_ms) as avg_latency,
			SUM(CASE WHEN success THEN 1 ELSE 0 END)::float / COUNT(*) * 100 as success_rate
		FROM analytics_records 
		WHERE timestamp >= NOW() - INTERVAL '1 hour'
	`
	
	var totalRequests, activeCustomers, totalBytes int64
	var avgLatency, successRate float64
	
	err := am.db.QueryRow(query).Scan(&totalRequests, &activeCustomers, &totalBytes, &avgLatency, &successRate)
	if err != nil {
		return nil, fmt.Errorf("failed to query system stats: %v", err)
	}
	
	return map[string]interface{}{
		"total_requests":   totalRequests,
		"active_customers": activeCustomers,
		"total_bytes":      totalBytes,
		"avg_latency_ms":   avgLatency,
		"success_rate":     successRate,
		"timestamp":        time.Now(),
	}, nil
}