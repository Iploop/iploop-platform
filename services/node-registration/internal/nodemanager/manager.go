package nodemanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type NodeManager struct {
	db     *sql.DB
	rdb    *redis.Client
	logger *logrus.Entry
}

type Node struct {
	ID             string    `json:"id"`
	DeviceID       string    `json:"device_id"`
	IPAddress      string    `json:"ip_address"`
	Country        string    `json:"country"`
	CountryName    string    `json:"country_name"`
	City           string    `json:"city"`
	Region         string    `json:"region"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	ASN            int       `json:"asn"`
	ISP            string    `json:"isp"`
	Carrier        string    `json:"carrier"`
	ConnectionType string    `json:"connection_type"`
	DeviceType     string    `json:"device_type"`
	SDKVersion     string    `json:"sdk_version"`
	Status         string    `json:"status"`
	QualityScore   int       `json:"quality_score"`
	BandwidthUsed  int64     `json:"bandwidth_used_mb"`
	TotalRequests  int64     `json:"total_requests"`
	SuccessRequests int64    `json:"successful_requests"`
	LastHeartbeat  time.Time `json:"last_heartbeat"`
	ConnectedSince time.Time `json:"connected_since"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type NodeRegistration struct {
	DeviceID       string  `json:"device_id"`
	IPAddress      string  `json:"ip_address"`
	Country        string  `json:"country"`
	CountryName    string  `json:"country_name"`
	City           string  `json:"city"`
	Region         string  `json:"region"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	ASN            int     `json:"asn"`
	ISP            string  `json:"isp"`
	Carrier        string  `json:"carrier"`
	ConnectionType string  `json:"connection_type"`
	DeviceType     string  `json:"device_type"`
	SDKVersion     string  `json:"sdk_version"`
}

type Statistics struct {
	TotalNodes      int            `json:"total_nodes"`
	ActiveNodes     int            `json:"active_nodes"`
	InactiveNodes   int            `json:"inactive_nodes"`
	CountryBreakdown map[string]int `json:"country_breakdown"`
	DeviceTypes     map[string]int `json:"device_types"`
	ConnectionTypes map[string]int `json:"connection_types"`
	AverageQuality  float64        `json:"average_quality"`
	TotalBandwidth  int64          `json:"total_bandwidth_mb"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func NewNodeManager(db *sql.DB, rdb *redis.Client, logger *logrus.Entry) *NodeManager {
	nm := &NodeManager{
		db:     db,
		rdb:    rdb,
		logger: logger.WithField("component", "node-manager"),
	}

	// Start cleanup routine
	go nm.startCleanupRoutine()

	return nm
}

func (nm *NodeManager) RegisterNode(registration *NodeRegistration) (*Node, error) {
	ctx := context.Background()

	// Check if node already exists
	existingNode, err := nm.getNodeByDeviceID(registration.DeviceID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing node: %v", err)
	}

	var node *Node
	now := time.Now()

	if existingNode != nil {
		// Update existing node
		node = existingNode
		node.IPAddress = registration.IPAddress
		node.Country = registration.Country
		node.CountryName = registration.CountryName
		node.City = registration.City
		node.Region = registration.Region
		node.Latitude = registration.Latitude
		node.Longitude = registration.Longitude
		node.ASN = registration.ASN
		node.ISP = registration.ISP
		node.Carrier = registration.Carrier
		node.ConnectionType = registration.ConnectionType
		node.DeviceType = registration.DeviceType
		node.SDKVersion = registration.SDKVersion
		node.Status = "available"
		node.LastHeartbeat = now
		node.ConnectedSince = now
		node.UpdatedAt = now

		err = nm.updateNode(node)
	} else {
		// Create new node
		node = &Node{
			ID:             uuid.New().String(),
			DeviceID:       registration.DeviceID,
			IPAddress:      registration.IPAddress,
			Country:        registration.Country,
			CountryName:    registration.CountryName,
			City:           registration.City,
			Region:         registration.Region,
			Latitude:       registration.Latitude,
			Longitude:      registration.Longitude,
			ASN:            registration.ASN,
			ISP:            registration.ISP,
			Carrier:        registration.Carrier,
			ConnectionType: registration.ConnectionType,
			DeviceType:     registration.DeviceType,
			SDKVersion:     registration.SDKVersion,
			Status:         "available",
			QualityScore:   100, // Start with perfect score
			BandwidthUsed:  0,
			TotalRequests:  0,
			SuccessRequests: 0,
			LastHeartbeat:  now,
			ConnectedSince: now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		err = nm.insertNode(node)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to register node: %v", err)
	}

	// Store in Redis for fast lookup
	err = nm.storeNodeInRedis(node)
	if err != nil {
		nm.logger.Warnf("Failed to store node in Redis: %v", err)
		// Don't fail the registration if Redis fails
	}

	nm.logger.Infof("Node registered: %s (device: %s, country: %s, city: %s)", 
		node.ID, node.DeviceID, node.Country, node.City)

	return node, nil
}

func (nm *NodeManager) UpdateHeartbeat(nodeID string) error {
	ctx := context.Background()
	now := time.Now()

	// Update in database
	query := `
		UPDATE nodes 
		SET last_heartbeat = $1, status = 'available', updated_at = $1
		WHERE id = $2
	`
	_, err := nm.db.Exec(query, now, nodeID)
	if err != nil {
		return fmt.Errorf("failed to update heartbeat in database: %v", err)
	}

	// Update in Redis
	nodeKey := nm.getRedisNodeKey(nodeID)
	nodeData, err := nm.rdb.Get(ctx, nodeKey).Result()
	if err != nil {
		// If not in Redis, try to load from database
		node, dbErr := nm.getNodeByID(nodeID)
		if dbErr != nil {
			return fmt.Errorf("node not found: %s", nodeID)
		}
		node.LastHeartbeat = now
		node.Status = "available"
		return nm.storeNodeInRedis(node)
	}

	var node Node
	if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
		return fmt.Errorf("failed to parse node data: %v", err)
	}

	node.LastHeartbeat = now
	node.Status = "available"
	
	return nm.storeNodeInRedis(&node)
}

func (nm *NodeManager) DisconnectNode(nodeID string) error {
	ctx := context.Background()
	now := time.Now()

	// Update status to inactive
	query := `
		UPDATE nodes 
		SET status = 'inactive', updated_at = $1
		WHERE id = $2
	`
	_, err := nm.db.Exec(query, now, nodeID)
	if err != nil {
		return fmt.Errorf("failed to update node status: %v", err)
	}

	// Remove from Redis active pool
	nodeKey := nm.getRedisNodeKey(nodeID)
	nm.rdb.Del(ctx, nodeKey)

	nm.logger.Infof("Node disconnected: %s", nodeID)
	return nil
}

func (nm *NodeManager) GetAllNodes() []*Node {
	query := `
		SELECT id, device_id, ip_address, country, country_name, city, region,
			   latitude, longitude, asn, isp, carrier, connection_type, device_type,
			   sdk_version, status, quality_score, bandwidth_used_mb, total_requests,
			   successful_requests, last_heartbeat, connected_since, created_at, updated_at
		FROM nodes
		ORDER BY last_heartbeat DESC
	`

	rows, err := nm.db.Query(query)
	if err != nil {
		nm.logger.Errorf("Failed to fetch nodes: %v", err)
		return []*Node{}
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		node := &Node{}
		err := rows.Scan(
			&node.ID, &node.DeviceID, &node.IPAddress, &node.Country, &node.CountryName,
			&node.City, &node.Region, &node.Latitude, &node.Longitude, &node.ASN,
			&node.ISP, &node.Carrier, &node.ConnectionType, &node.DeviceType,
			&node.SDKVersion, &node.Status, &node.QualityScore, &node.BandwidthUsed,
			&node.TotalRequests, &node.SuccessRequests, &node.LastHeartbeat,
			&node.ConnectedSince, &node.CreatedAt, &node.UpdatedAt,
		)
		if err != nil {
			nm.logger.Errorf("Failed to scan node: %v", err)
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes
}

func (nm *NodeManager) GetStatistics() *Statistics {
	stats := &Statistics{
		CountryBreakdown: make(map[string]int),
		DeviceTypes:     make(map[string]int),
		ConnectionTypes: make(map[string]int),
		UpdatedAt:       time.Now(),
	}

	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN status = 'available' AND last_heartbeat > NOW() - INTERVAL '2 minutes' THEN 1 END) as active,
			AVG(quality_score) as avg_quality,
			SUM(bandwidth_used_mb) as total_bandwidth,
			country,
			device_type,
			connection_type
		FROM nodes
		GROUP BY country, device_type, connection_type
	`

	rows, err := nm.db.Query(query)
	if err != nil {
		nm.logger.Errorf("Failed to fetch statistics: %v", err)
		return stats
	}
	defer rows.Close()

	for rows.Next() {
		var total, active int
		var avgQuality float64
		var totalBandwidth int64
		var country, deviceType, connectionType string

		err := rows.Scan(&total, &active, &avgQuality, &totalBandwidth, 
			&country, &deviceType, &connectionType)
		if err != nil {
			continue
		}

		stats.TotalNodes += total
		stats.ActiveNodes += active
		stats.TotalBandwidth += totalBandwidth
		
		if avgQuality > 0 {
			stats.AverageQuality = avgQuality
		}

		stats.CountryBreakdown[country] += total
		stats.DeviceTypes[deviceType] += total
		stats.ConnectionTypes[connectionType] += total
	}

	stats.InactiveNodes = stats.TotalNodes - stats.ActiveNodes
	return stats
}

func (nm *NodeManager) getNodeByDeviceID(deviceID string) (*Node, error) {
	query := `
		SELECT id, device_id, ip_address, country, country_name, city, region,
			   latitude, longitude, asn, isp, carrier, connection_type, device_type,
			   sdk_version, status, quality_score, bandwidth_used_mb, total_requests,
			   successful_requests, last_heartbeat, connected_since, created_at, updated_at
		FROM nodes
		WHERE device_id = $1
	`

	node := &Node{}
	err := nm.db.QueryRow(query, deviceID).Scan(
		&node.ID, &node.DeviceID, &node.IPAddress, &node.Country, &node.CountryName,
		&node.City, &node.Region, &node.Latitude, &node.Longitude, &node.ASN,
		&node.ISP, &node.Carrier, &node.ConnectionType, &node.DeviceType,
		&node.SDKVersion, &node.Status, &node.QualityScore, &node.BandwidthUsed,
		&node.TotalRequests, &node.SuccessRequests, &node.LastHeartbeat,
		&node.ConnectedSince, &node.CreatedAt, &node.UpdatedAt,
	)

	return node, err
}

func (nm *NodeManager) getNodeByID(nodeID string) (*Node, error) {
	query := `
		SELECT id, device_id, ip_address, country, country_name, city, region,
			   latitude, longitude, asn, isp, carrier, connection_type, device_type,
			   sdk_version, status, quality_score, bandwidth_used_mb, total_requests,
			   successful_requests, last_heartbeat, connected_since, created_at, updated_at
		FROM nodes
		WHERE id = $1
	`

	node := &Node{}
	err := nm.db.QueryRow(query, nodeID).Scan(
		&node.ID, &node.DeviceID, &node.IPAddress, &node.Country, &node.CountryName,
		&node.City, &node.Region, &node.Latitude, &node.Longitude, &node.ASN,
		&node.ISP, &node.Carrier, &node.ConnectionType, &node.DeviceType,
		&node.SDKVersion, &node.Status, &node.QualityScore, &node.BandwidthUsed,
		&node.TotalRequests, &node.SuccessRequests, &node.LastHeartbeat,
		&node.ConnectedSince, &node.CreatedAt, &node.UpdatedAt,
	)

	return node, err
}

func (nm *NodeManager) insertNode(node *Node) error {
	query := `
		INSERT INTO nodes (
			id, device_id, ip_address, country, country_name, city, region,
			latitude, longitude, asn, isp, carrier, connection_type, device_type,
			sdk_version, status, quality_score, bandwidth_used_mb, total_requests,
			successful_requests, last_heartbeat, connected_since, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24
		)
	`

	_, err := nm.db.Exec(query,
		node.ID, node.DeviceID, node.IPAddress, node.Country, node.CountryName,
		node.City, node.Region, node.Latitude, node.Longitude, node.ASN,
		node.ISP, node.Carrier, node.ConnectionType, node.DeviceType,
		node.SDKVersion, node.Status, node.QualityScore, node.BandwidthUsed,
		node.TotalRequests, node.SuccessRequests, node.LastHeartbeat,
		node.ConnectedSince, node.CreatedAt, node.UpdatedAt,
	)

	return err
}

func (nm *NodeManager) updateNode(node *Node) error {
	query := `
		UPDATE nodes SET
			ip_address = $2, country = $3, country_name = $4, city = $5, region = $6,
			latitude = $7, longitude = $8, asn = $9, isp = $10, carrier = $11,
			connection_type = $12, device_type = $13, sdk_version = $14, status = $15,
			last_heartbeat = $16, connected_since = $17, updated_at = $18
		WHERE id = $1
	`

	_, err := nm.db.Exec(query,
		node.ID, node.IPAddress, node.Country, node.CountryName, node.City,
		node.Region, node.Latitude, node.Longitude, node.ASN, node.ISP,
		node.Carrier, node.ConnectionType, node.DeviceType, node.SDKVersion,
		node.Status, node.LastHeartbeat, node.ConnectedSince, node.UpdatedAt,
	)

	return err
}

func (nm *NodeManager) storeNodeInRedis(node *Node) error {
	ctx := context.Background()
	nodeKey := nm.getRedisNodeKey(node.ID)
	
	// Store in main pool
	nodeJSON, err := json.Marshal(node)
	if err != nil {
		return err
	}

	// Store with TTL of 5 minutes (will be refreshed by heartbeats)
	err = nm.rdb.Set(ctx, nodeKey, nodeJSON, 5*time.Minute).Err()
	if err != nil {
		return err
	}

	// Also store in country-specific key for faster targeting
	countryKey := fmt.Sprintf("node:%s:%s", node.Country, node.ID)
	nm.rdb.Set(ctx, countryKey, nodeJSON, 5*time.Minute)

	// And city-specific key
	if node.City != "" {
		cityKey := fmt.Sprintf("node:%s:%s:%s", node.Country, node.City, node.ID)
		nm.rdb.Set(ctx, cityKey, nodeJSON, 5*time.Minute)
	}

	return nil
}

func (nm *NodeManager) getRedisNodeKey(nodeID string) string {
	return fmt.Sprintf("node:%s", nodeID)
}

func (nm *NodeManager) startCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			nm.cleanupInactiveNodes()
		}
	}
}

func (nm *NodeManager) cleanupInactiveNodes() {
	cutoff := time.Now().Add(-5 * time.Minute)
	
	query := `
		UPDATE nodes 
		SET status = 'inactive', updated_at = NOW()
		WHERE last_heartbeat < $1 AND status != 'inactive'
	`

	result, err := nm.db.Exec(query, cutoff)
	if err != nil {
		nm.logger.Errorf("Failed to cleanup inactive nodes: %v", err)
		return
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		nm.logger.Infof("Marked %d nodes as inactive", affected)
	}
}