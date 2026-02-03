package nodepool

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type NodePool struct {
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
	ASN            int       `json:"asn"`
	ISP            string    `json:"isp"`
	Carrier        string    `json:"carrier"`
	ConnectionType string    `json:"connection_type"`
	DeviceType     string    `json:"device_type"`
	SDKVersion     string    `json:"sdk_version"`
	Status         string    `json:"status"`
	QualityScore   int       `json:"quality_score"`
	BandwidthUsed  int64     `json:"bandwidth_used_mb"`
	LastHeartbeat  time.Time `json:"last_heartbeat"`
	ConnectedSince time.Time `json:"connected_since"`
}

type NodeSelection struct {
	Country       string
	City          string
	SessionID     string
	RotateAfter   int    // Rotate IP after N requests (0 = no rotation)
	RotateOnError bool   // Rotate IP on error/timeout
}

type SessionState struct {
	NodeID       string    `json:"node_id"`
	Country      string    `json:"country"`
	City         string    `json:"city"`
	RequestCount int       `json:"request_count"`
	RotateAfter  int       `json:"rotate_after"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func NewNodePool(rdb *redis.Client, logger *logrus.Entry) *NodePool {
	pool := &NodePool{
		rdb:    rdb,
		logger: logger.WithField("component", "nodepool"),
	}

	// Start background cleanup routine
	go pool.cleanupInactiveNodes()

	return pool
}

// SelectNode selects the best available node based on targeting criteria
func (np *NodePool) SelectNode(selection *NodeSelection) (*Node, error) {
	ctx := context.Background()

	// If session ID is specified, try to get sticky node
	if selection.SessionID != "" {
		node, needsRotation, err := np.getStickyNode(selection.SessionID, selection.RotateAfter)
		if err == nil && node != nil && !needsRotation {
			// Check if node is blacklisted
			if !np.IsNodeBlacklisted(node.ID) {
				np.logger.Debugf("Using sticky node %s for session %s", node.ID, selection.SessionID)
				return node, nil
			}
			np.logger.Debugf("Sticky node %s is blacklisted, selecting new node", node.ID)
		}
		if needsRotation {
			np.logger.Debugf("Rotating IP for session %s", selection.SessionID)
		}
	}

	// Build Redis key pattern for node selection
	pattern := "node:*"
	if selection.Country != "" {
		pattern = fmt.Sprintf("node:%s:*", selection.Country)
		if selection.City != "" {
			pattern = fmt.Sprintf("node:%s:%s:*", selection.Country, selection.City)
		}
	}

	// Get available nodes
	keys, err := np.rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %v", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no nodes available for criteria: country=%s, city=%s", selection.Country, selection.City)
	}

	// Get node data
	var availableNodes []*Node
	for _, key := range keys {
		nodeData, err := np.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var node Node
		if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
			continue
		}

		// Check if node is available, healthy, and not blacklisted
		if node.Status == "available" && np.isNodeHealthy(&node) && !np.IsNodeBlacklisted(node.ID) {
			availableNodes = append(availableNodes, &node)
		}
	}

	if len(availableNodes) == 0 {
		return nil, fmt.Errorf("no healthy nodes available")
	}

	// Select best node based on quality score and load
	selectedNode := np.selectBestNode(availableNodes)

	// Create sticky session if session ID is provided
	if selection.SessionID != "" && selectedNode != nil {
		np.createStickySession(selection.SessionID, selectedNode, selection)
	}

	// Mark node as busy temporarily
	np.markNodeBusy(selectedNode.ID)

	np.logger.Debugf("Selected node %s (%s) for request", selectedNode.ID, selectedNode.IPAddress)
	return selectedNode, nil
}

// ReleaseNode marks a node as available again
func (np *NodePool) ReleaseNode(nodeID string) {
	ctx := context.Background()
	key := fmt.Sprintf("node:busy:%s", nodeID)
	np.rdb.Del(ctx, key)
	np.logger.Debugf("Released node %s", nodeID)
}

// GetNodeByID retrieves a specific node
func (np *NodePool) GetNodeByID(nodeID string) (*Node, error) {
	ctx := context.Background()
	
	// Try to find the node in any country/city combination
	keys, err := np.rdb.Keys(ctx, "node:*").Result()
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		nodeData, err := np.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var node Node
		if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
			continue
		}

		if node.ID == nodeID {
			return &node, nil
		}
	}

	return nil, fmt.Errorf("node not found: %s", nodeID)
}

func (np *NodePool) getStickyNode(sessionID string, rotateAfter int) (*Node, bool, error) {
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)

	sessionData, err := np.rdb.Get(ctx, sessionKey).Result()
	if err != nil {
		return nil, false, err
	}

	var session SessionState
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return nil, false, err
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		np.rdb.Del(ctx, sessionKey)
		return nil, false, fmt.Errorf("session expired")
	}

	// Check if we need to rotate based on request count
	needsRotation := false
	if session.RotateAfter > 0 && session.RequestCount >= session.RotateAfter {
		np.logger.Debugf("Session %s needs rotation after %d requests", sessionID, session.RequestCount)
		needsRotation = true
	}

	if needsRotation {
		// Delete the session to force rotation
		np.rdb.Del(ctx, sessionKey)
		return nil, true, fmt.Errorf("rotation needed")
	}

	// Get the node
	node, err := np.GetNodeByID(session.NodeID)
	return node, false, err
}

// IncrementSessionRequests increments the request count for a session
func (np *NodePool) IncrementSessionRequests(sessionID string) {
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)

	sessionData, err := np.rdb.Get(ctx, sessionKey).Result()
	if err != nil {
		return
	}

	var session SessionState
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return
	}

	session.RequestCount++
	sessionJSON, _ := json.Marshal(session)
	
	// Calculate remaining TTL
	ttl := time.Until(session.ExpiresAt)
	if ttl > 0 {
		np.rdb.Set(ctx, sessionKey, sessionJSON, ttl)
	}
}

// BlacklistNode temporarily blacklists a node (e.g., after errors)
func (np *NodePool) BlacklistNode(nodeID string, duration time.Duration) {
	ctx := context.Background()
	blacklistKey := fmt.Sprintf("blacklist:%s", nodeID)
	np.rdb.Set(ctx, blacklistKey, "1", duration)
	np.logger.Warnf("Node %s blacklisted for %v", nodeID, duration)
}

// IsNodeBlacklisted checks if a node is blacklisted
func (np *NodePool) IsNodeBlacklisted(nodeID string) bool {
	ctx := context.Background()
	blacklistKey := fmt.Sprintf("blacklist:%s", nodeID)
	exists, _ := np.rdb.Exists(ctx, blacklistKey).Result()
	return exists > 0
}

// RotateSession forces rotation for a session (e.g., after error)
func (np *NodePool) RotateSession(sessionID string) {
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	np.rdb.Del(ctx, sessionKey)
	np.logger.Debugf("Forced rotation for session %s", sessionID)
}

func (np *NodePool) createStickySession(sessionID string, node *Node, selection *NodeSelection) {
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)

	session := SessionState{
		NodeID:       node.ID,
		Country:      selection.Country,
		City:         selection.City,
		RequestCount: 0,
		RotateAfter:  selection.RotateAfter,
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		CreatedAt:    time.Now(),
	}

	sessionJSON, _ := json.Marshal(session)
	np.rdb.Set(ctx, sessionKey, sessionJSON, 30*time.Minute)
	np.logger.Debugf("Created sticky session %s -> node %s (rotate after %d)", sessionID, node.ID, selection.RotateAfter)
}

func (np *NodePool) selectBestNode(nodes []*Node) *Node {
	if len(nodes) == 1 {
		return nodes[0]
	}

	// Simple load balancing: prefer nodes with higher quality scores and lower current load
	bestNode := nodes[0]
	bestScore := np.calculateNodeScore(bestNode)

	for _, node := range nodes[1:] {
		score := np.calculateNodeScore(node)
		if score > bestScore {
			bestNode = node
			bestScore = score
		}
	}

	// Add some randomization to prevent all traffic going to the same "best" node
	if rand.Float64() < 0.2 {
		return nodes[rand.Intn(len(nodes))]
	}

	return bestNode
}

func (np *NodePool) calculateNodeScore(node *Node) float64 {
	qualityScore := float64(node.QualityScore) / 100.0
	
	// Check if node is currently busy
	ctx := context.Background()
	busyKey := fmt.Sprintf("node:busy:%s", node.ID)
	isBusy, _ := np.rdb.Exists(ctx, busyKey).Result()
	
	loadPenalty := 0.0
	if isBusy > 0 {
		loadPenalty = 0.5 // Penalize busy nodes
	}

	// Time since last heartbeat (fresher is better)
	timePenalty := time.Since(node.LastHeartbeat).Minutes() / 10.0
	if timePenalty > 1.0 {
		timePenalty = 1.0
	}

	return qualityScore - loadPenalty - timePenalty
}

func (np *NodePool) isNodeHealthy(node *Node) bool {
	// Node is healthy if:
	// 1. Last heartbeat was within 2 minutes
	// 2. Quality score is above 50
	// 3. Status is available

	if node.Status != "available" {
		return false
	}

	if node.QualityScore < 50 {
		return false
	}

	if time.Since(node.LastHeartbeat) > 2*time.Minute {
		return false
	}

	return true
}

func (np *NodePool) markNodeBusy(nodeID string) {
	ctx := context.Background()
	busyKey := fmt.Sprintf("node:busy:%s", nodeID)
	np.rdb.Set(ctx, busyKey, "1", 30*time.Second) // Mark as busy for 30 seconds
}

func (np *NodePool) cleanupInactiveNodes() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			np.performCleanup()
		}
	}
}

func (np *NodePool) performCleanup() {
	ctx := context.Background()
	cutoff := time.Now().Add(-3 * time.Minute) // 3 minutes without heartbeat

	keys, err := np.rdb.Keys(ctx, "node:*").Result()
	if err != nil {
		np.logger.Errorf("Failed to get node keys for cleanup: %v", err)
		return
	}

	cleaned := 0
	for _, key := range keys {
		nodeData, err := np.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var node Node
		if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
			continue
		}

		if node.LastHeartbeat.Before(cutoff) {
			np.rdb.Del(ctx, key)
			cleaned++
		}
	}

	if cleaned > 0 {
		np.logger.Infof("Cleaned up %d inactive nodes", cleaned)
	}
}

// GetStatus returns current node pool status
func (np *NodePool) GetStatus() map[string]interface{} {
	ctx := context.Background()

	keys, _ := np.rdb.Keys(ctx, "node:*").Result()
	
	stats := map[string]int{
		"total":     0,
		"available": 0,
		"busy":      0,
		"inactive":  0,
	}
	
	countries := make(map[string]int)

	for _, key := range keys {
		nodeData, err := np.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var node Node
		if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
			continue
		}

		stats["total"]++
		countries[node.Country]++

		if np.isNodeHealthy(&node) {
			stats["available"]++
		} else {
			stats["inactive"]++
		}

		// Check if busy
		busyKey := fmt.Sprintf("node:busy:%s", node.ID)
		if exists, _ := np.rdb.Exists(ctx, busyKey).Result(); exists > 0 {
			stats["busy"]++
		}
	}

	return map[string]interface{}{
		"stats":     stats,
		"countries": countries,
		"timestamp": time.Now().UTC(),
	}
}