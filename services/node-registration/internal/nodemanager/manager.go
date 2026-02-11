package nodemanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type NodeManager struct {
	db     *sql.DB
	rdb    *redis.Client
	logger *logrus.Entry

	// Batch sync: collect dirty node IDs, flush to Postgres periodically
	dirtyMu    sync.Mutex
	dirtyNodes map[string]bool // node IDs that need Postgres sync
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
		db:         db,
		rdb:        rdb,
		logger:     logger.WithField("component", "node-manager"),
		dirtyNodes: make(map[string]bool),
	}

	// Pre-load device→node mappings from Postgres into Redis
	nm.preloadDeviceMappings()

	// Start background routines
	go nm.startCleanupRoutine()
	go nm.startBatchSyncRoutine()

	return nm
}

// preloadDeviceMappings loads device_id→node_id mappings from Postgres into Redis
// so RegisterNode can skip Postgres lookups on reconnects.
func (nm *NodeManager) preloadDeviceMappings() {
	ctx := context.Background()
	query := `SELECT id, device_id FROM nodes WHERE status = 'available' OR last_heartbeat > NOW() - INTERVAL '1 hour'`
	rows, err := nm.db.Query(query)
	if err != nil {
		nm.logger.Warnf("Failed to preload device mappings: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	pipe := nm.rdb.Pipeline()
	for rows.Next() {
		var nodeID, deviceID string
		if err := rows.Scan(&nodeID, &deviceID); err != nil {
			continue
		}
		pipe.Set(ctx, fmt.Sprintf("device:%s", deviceID), nodeID, 24*time.Hour)
		count++
		if count%500 == 0 {
			pipe.Exec(ctx)
			pipe = nm.rdb.Pipeline()
		}
	}
	if count%500 != 0 {
		pipe.Exec(ctx)
	}
	nm.logger.Infof("Preloaded %d device→node mappings into Redis", count)
}

// ─── RegisterNode: Redis-first, Postgres async ───

func (nm *NodeManager) RegisterNode(registration *NodeRegistration) (*Node, error) {
	ctx := context.Background()
	now := time.Now()

	// Check Redis first for existing node by device_id
	deviceKey := fmt.Sprintf("device:%s", registration.DeviceID)
	existingNodeID, err := nm.rdb.Get(ctx, deviceKey).Result()

	var node *Node

	if err == nil && existingNodeID != "" {
		// Try to load existing node from Redis
		nodeData, redisErr := nm.rdb.Get(ctx, nm.getRedisNodeKey(existingNodeID)).Result()
		if redisErr == nil {
			var existing Node
			if json.Unmarshal([]byte(nodeData), &existing) == nil {
				node = &existing
			}
		}
	}

	if node == nil {
		// Check Postgres for existing node
		existingNode, dbErr := nm.getNodeByDeviceID(registration.DeviceID)
		if dbErr == nil && existingNode != nil && existingNode.ID != "" {
			node = existingNode
		}
	}

	if node != nil {
		// Update existing node
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
			QualityScore:   100,
			BandwidthUsed:  0,
			TotalRequests:  0,
			SuccessRequests: 0,
			LastHeartbeat:  now,
			ConnectedSince: now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
	}

	// Store in Redis immediately (this is the hot path)
	if err := nm.storeNodeInRedis(node); err != nil {
		nm.logger.Warnf("Failed to store node in Redis: %v", err)
	}

	// Store device→node mapping in Redis
	nm.rdb.Set(ctx, fmt.Sprintf("device:%s", node.DeviceID), node.ID, 24*time.Hour)

	// Mark dirty for async Postgres sync
	nm.markDirty(node.ID)

	nm.logger.Infof("Node registered: %s (device: %s, country: %s, city: %s)",
		node.ID, node.DeviceID, node.Country, node.City)

	return node, nil
}

// ─── UpdateHeartbeat: Redis-only, no Postgres ───

func (nm *NodeManager) UpdateHeartbeat(nodeID string) error {
	ctx := context.Background()
	now := time.Now()

	nodeKey := nm.getRedisNodeKey(nodeID)
	nodeData, err := nm.rdb.Get(ctx, nodeKey).Result()
	if err != nil {
		// Not in Redis — try loading from Postgres
		node, dbErr := nm.getNodeByID(nodeID)
		if dbErr != nil {
			return fmt.Errorf("node not found: %s", nodeID)
		}
		node.LastHeartbeat = now
		node.Status = "available"
		nm.storeNodeInRedis(node)
		return nil
	}

	var node Node
	if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
		return fmt.Errorf("failed to parse node data: %v", err)
	}

	node.LastHeartbeat = now
	node.Status = "available"
	node.UpdatedAt = now

	// Update Redis only — no Postgres write
	return nm.storeNodeInRedis(&node)
}

// ─── DisconnectNode: Redis-first, Postgres async ───

func (nm *NodeManager) DisconnectNode(nodeID string) error {
	ctx := context.Background()

	// Remove from Redis active pool
	nodeKey := nm.getRedisNodeKey(nodeID)

	// Try to get node data to clean up country/city keys
	nodeData, err := nm.rdb.Get(ctx, nodeKey).Result()
	if err == nil {
		var node Node
		if json.Unmarshal([]byte(nodeData), &node) == nil {
			// Clean up country and city keys
			nm.rdb.Del(ctx, fmt.Sprintf("node:%s:%s", node.Country, node.ID))
			if node.City != "" {
				nm.rdb.Del(ctx, fmt.Sprintf("node:%s:%s:%s", node.Country, node.City, node.ID))
			}
		}
	}

	nm.rdb.Del(ctx, nodeKey)

	// Mark for async Postgres update (status → inactive)
	nm.markDirtyDisconnect(nodeID)

	nm.logger.Infof("Node disconnected: %s", nodeID)
	return nil
}

// ─── Batch Postgres Sync ───

func (nm *NodeManager) markDirty(nodeID string) {
	nm.dirtyMu.Lock()
	nm.dirtyNodes[nodeID] = true // true = active/update
	nm.dirtyMu.Unlock()
}

func (nm *NodeManager) markDirtyDisconnect(nodeID string) {
	nm.dirtyMu.Lock()
	nm.dirtyNodes[nodeID] = false // false = disconnect
	nm.dirtyMu.Unlock()
}

func (nm *NodeManager) startBatchSyncRoutine() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		nm.flushToPostgres()
	}
}

func (nm *NodeManager) flushToPostgres() {
	nm.dirtyMu.Lock()
	if len(nm.dirtyNodes) == 0 {
		nm.dirtyMu.Unlock()
		return
	}
	// Snapshot and reset
	batch := nm.dirtyNodes
	nm.dirtyNodes = make(map[string]bool)
	nm.dirtyMu.Unlock()

	ctx := context.Background()
	synced := 0
	disconnected := 0
	errors := 0

	for nodeID, isActive := range batch {
		if !isActive {
			// Disconnect — just mark inactive in Postgres
			_, err := nm.db.Exec(
				`UPDATE nodes SET status = 'inactive', updated_at = NOW() WHERE id = $1`,
				nodeID,
			)
			if err != nil {
				errors++
			} else {
				disconnected++
			}
			continue
		}

		// Active node — load from Redis and upsert to Postgres
		nodeKey := nm.getRedisNodeKey(nodeID)
		nodeData, err := nm.rdb.Get(ctx, nodeKey).Result()
		if err != nil {
			// Node already expired from Redis, skip
			continue
		}

		var node Node
		if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
			continue
		}

		// Upsert by device_id (the true unique key for a physical device)
		_, err = nm.db.Exec(`
			INSERT INTO nodes (
				id, device_id, ip_address, country, country_name, city, region,
				latitude, longitude, asn, isp, carrier, connection_type, device_type,
				sdk_version, status, quality_score, bandwidth_used_mb, total_requests,
				successful_requests, last_heartbeat, connected_since, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
				$15, $16, $17, $18, $19, $20, $21, $22, $23, $24
			)
			ON CONFLICT (device_id) DO UPDATE SET
				ip_address = EXCLUDED.ip_address,
				country = EXCLUDED.country,
				country_name = EXCLUDED.country_name,
				city = EXCLUDED.city,
				region = EXCLUDED.region,
				latitude = EXCLUDED.latitude,
				longitude = EXCLUDED.longitude,
				asn = EXCLUDED.asn,
				isp = EXCLUDED.isp,
				carrier = EXCLUDED.carrier,
				connection_type = EXCLUDED.connection_type,
				device_type = EXCLUDED.device_type,
				sdk_version = EXCLUDED.sdk_version,
				status = EXCLUDED.status,
				last_heartbeat = EXCLUDED.last_heartbeat,
				connected_since = EXCLUDED.connected_since,
				updated_at = EXCLUDED.updated_at
		`,
			node.ID, node.DeviceID, node.IPAddress, node.Country, node.CountryName,
			node.City, node.Region, node.Latitude, node.Longitude, node.ASN,
			node.ISP, node.Carrier, node.ConnectionType, node.DeviceType,
			node.SDKVersion, node.Status, node.QualityScore, node.BandwidthUsed,
			node.TotalRequests, node.SuccessRequests, node.LastHeartbeat,
			node.ConnectedSince, node.CreatedAt, node.UpdatedAt,
		)
		if err != nil {
			errors++
			nm.logger.Warnf("Postgres sync failed for node %s: %v", nodeID, err)
		} else {
			synced++
		}
	}

	if synced > 0 || disconnected > 0 || errors > 0 {
		nm.logger.Infof("Postgres batch sync: %d upserted, %d disconnected, %d errors (from %d dirty)",
			synced, disconnected, errors, len(batch))
	}
}

// ─── Read operations (unchanged, still use Postgres for heavy queries) ───

func (nm *NodeManager) GetAllNodes() []*Node {
	query := `
		SELECT id, device_id, ip_address, country, country_name, city, region,
			   COALESCE(latitude, 0), COALESCE(longitude, 0), asn, isp, carrier, connection_type, device_type,
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
			COUNT(CASE WHEN status = 'available' AND last_heartbeat > NOW() - INTERVAL '6 minutes' THEN 1 END) as active,
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

// ─── Internal helpers ───

func (nm *NodeManager) getNodeByDeviceID(deviceID string) (*Node, error) {
	query := `
		SELECT id, device_id, ip_address, country, country_name, city, region,
			   COALESCE(latitude, 0), COALESCE(longitude, 0), asn, isp, carrier, connection_type, device_type,
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
			   COALESCE(latitude, 0), COALESCE(longitude, 0), asn, isp, carrier, connection_type, device_type,
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

func (nm *NodeManager) storeNodeInRedis(node *Node) error {
	ctx := context.Background()
	nodeKey := nm.getRedisNodeKey(node.ID)

	nodeJSON, err := json.Marshal(node)
	if err != nil {
		return err
	}

	// Store with TTL of 6 minutes (heartbeat every 5 min refreshes it)
	err = nm.rdb.Set(ctx, nodeKey, nodeJSON, 6*time.Minute).Err()
	if err != nil {
		return err
	}

	// Country-specific key for fast targeting
	countryKey := fmt.Sprintf("node:%s:%s", node.Country, node.ID)
	nm.rdb.Set(ctx, countryKey, nodeJSON, 6*time.Minute)

	// City-specific key
	if node.City != "" {
		cityKey := fmt.Sprintf("node:%s:%s:%s", node.Country, node.City, node.ID)
		nm.rdb.Set(ctx, cityKey, nodeJSON, 6*time.Minute)
	}

	return nil
}

func (nm *NodeManager) getRedisNodeKey(nodeID string) string {
	return fmt.Sprintf("node:%s", nodeID)
}

func (nm *NodeManager) startCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		nm.cleanupInactiveNodes()
	}
}

func (nm *NodeManager) cleanupInactiveNodes() {
	cutoff := time.Now().Add(-10 * time.Minute)

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
