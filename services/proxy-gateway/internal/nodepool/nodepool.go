package nodepool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type NodePool struct {
	rdb            *redis.Client
	logger         *logrus.Entry
	nodeRegURL     string
	healthMu       sync.RWMutex
	healthChecking map[string]bool // tracks nodes currently being checked

	// Connected node cache: only nodes with active WebSocket to node-reg
	connectedMu    sync.RWMutex
	connectedNodes map[string]bool // node IDs confirmed connected

	// Quality tracking: nodes that actually completed proxy requests
	qualityMu     sync.RWMutex
	provenNodes   map[string]time.Time // node ID → last successful proxy time

	// In-memory node cache (avoids Redis roundtrips on every SelectNode)
	nodeCacheMu   sync.RWMutex
	nodeCache     map[string]*Node // node ID → cached node data
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
	ASN           int    // Target ASN/ISP
	MinSpeed      int    // Minimum speed in Mbps
	MaxLatency    int    // Maximum latency in ms
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
	nodeRegURL := os.Getenv("NODE_REGISTRATION_URL")
	if nodeRegURL == "" {
		nodeRegURL = "http://node-registration:8001"
	}

	pool := &NodePool{
		rdb:            rdb,
		logger:         logger.WithField("component", "nodepool"),
		nodeRegURL:     nodeRegURL,
		healthChecking: make(map[string]bool),
		connectedNodes: make(map[string]bool),
		provenNodes:    make(map[string]time.Time),
		nodeCache:      make(map[string]*Node),
	}

	// Start background routines
	go pool.cleanupInactiveNodes()
	go pool.healthCheckLoop()
	go pool.refreshConnectedNodes()
	go pool.refreshNodeCache()

	return pool
}

// refreshConnectedNodes periodically fetches connected node IDs from node-reg
func (np *NodePool) refreshConnectedNodes() {
	// Initial fetch immediately
	np.fetchConnectedNodes()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		np.fetchConnectedNodes()
	}
}

func (np *NodePool) fetchConnectedNodes() {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(np.nodeRegURL + "/internal/connected-nodes")
	if err != nil {
		np.logger.Warnf("Failed to fetch connected nodes: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var result struct {
		NodeIDs []string `json:"node_ids"`
		Count   int      `json:"count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		np.logger.Warnf("Failed to parse connected nodes: %v", err)
		return
	}

	newMap := make(map[string]bool, len(result.NodeIDs))
	for _, id := range result.NodeIDs {
		newMap[id] = true
	}

	np.connectedMu.Lock()
	np.connectedNodes = newMap
	np.connectedMu.Unlock()

	np.logger.Debugf("Refreshed connected nodes: %d", len(newMap))
}

// IsConnected checks if a node has an active WebSocket connection
func (np *NodePool) IsConnected(nodeID string) bool {
	np.connectedMu.RLock()
	defer np.connectedMu.RUnlock()
	return np.connectedNodes[nodeID]
}

// GetConnectedNodeIDs returns a copy of connected node IDs
func (np *NodePool) GetConnectedNodeIDs() []string {
	np.connectedMu.RLock()
	defer np.connectedMu.RUnlock()
	ids := make([]string, 0, len(np.connectedNodes))
	for id := range np.connectedNodes {
		ids = append(ids, id)
	}
	return ids
}

// refreshNodeCache periodically loads all connected node data from Redis into memory
func (np *NodePool) refreshNodeCache() {
	np.buildNodeCache()
	ticker := time.NewTicker(30 * time.Second) // every 30s — not too aggressive
	defer ticker.Stop()
	for range ticker.C {
		np.buildNodeCache()
	}
}

func (np *NodePool) buildNodeCache() {
	ctx := context.Background()
	connectedIDs := np.GetConnectedNodeIDs()
	if len(connectedIDs) == 0 {
		return
	}

	newCache := make(map[string]*Node, len(connectedIDs))
	
	// Batch in chunks of 500 to avoid overwhelming Redis
	chunkSize := 500
	for i := 0; i < len(connectedIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(connectedIDs) {
			end = len(connectedIDs)
		}
		chunk := connectedIDs[i:end]
		
		pipe := np.rdb.Pipeline()
		cmds := make(map[string]*redis.StringCmd, len(chunk))
		for _, nodeID := range chunk {
			cmds[nodeID] = pipe.Get(ctx, fmt.Sprintf("node:%s", nodeID))
		}
		pipe.Exec(ctx)

		for nodeID, cmd := range cmds {
			data, err := cmd.Result()
			if err != nil {
				continue
			}
			var node Node
			if err := json.Unmarshal([]byte(data), &node); err != nil {
				continue
			}
			newCache[nodeID] = &node
		}
	}

	np.nodeCacheMu.Lock()
	np.nodeCache = newCache
	np.nodeCacheMu.Unlock()

	np.logger.Debugf("Node cache refreshed: %d nodes", len(newCache))
}

// getCachedNode returns a node from the in-memory cache
func (np *NodePool) getCachedNode(nodeID string) *Node {
	np.nodeCacheMu.RLock()
	defer np.nodeCacheMu.RUnlock()
	return np.nodeCache[nodeID]
}

// MarkProven marks a node as having completed a successful proxy request
func (np *NodePool) MarkProven(nodeID string) {
	np.qualityMu.Lock()
	np.provenNodes[nodeID] = time.Now()
	np.qualityMu.Unlock()
}

// GetProvenNodes returns node IDs that successfully proxied in the last N minutes
func (np *NodePool) GetProvenNodes(maxAge time.Duration) []string {
	np.qualityMu.RLock()
	defer np.qualityMu.RUnlock()
	cutoff := time.Now().Add(-maxAge)
	ids := make([]string, 0)
	for id, t := range np.provenNodes {
		if t.After(cutoff) {
			ids = append(ids, id)
		}
	}
	return ids
}

// SelectNode selects the best available node based on targeting criteria
func (np *NodePool) SelectNode(selection *NodeSelection) (*Node, error) {
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

	// Fast path: pick random connected node from cache
	np.nodeCacheMu.RLock()
	cacheLen := len(np.nodeCache)
	np.nodeCacheMu.RUnlock()

	if cacheLen == 0 {
		return nil, fmt.Errorf("no connected nodes in cache")
	}

	country := strings.ToUpper(selection.Country)

	// Random sampling from cache — O(1) per attempt
	var selectedNode *Node
	maxTries := 50

	np.nodeCacheMu.RLock()
	// Build a snapshot of keys for random access
	cacheKeys := make([]string, 0, len(np.nodeCache))
	for k := range np.nodeCache {
		cacheKeys = append(cacheKeys, k)
	}
	np.nodeCacheMu.RUnlock()

	for tried := 0; tried < maxTries && tried < len(cacheKeys); tried++ {
		idx := rand.Intn(len(cacheKeys))
		nodeID := cacheKeys[idx]

		if np.IsNodeBlacklisted(nodeID) {
			continue
		}
		if !np.IsConnected(nodeID) {
			continue
		}

		node := np.getCachedNode(nodeID)
		if node == nil {
			continue
		}

		if country != "" && strings.ToUpper(node.Country) != country {
			continue
		}
		if selection.City != "" && normalizeCity(node.City) != normalizeCity(selection.City) {
			continue
		}
		if node.Status != "available" {
			continue
		}

		selectedNode = node
		break
	}

	if selectedNode == nil {
		return nil, fmt.Errorf("no matching nodes for: country=%s, city=%s (cache=%d)", selection.Country, selection.City, cacheLen)
	}

	// Create sticky session if session ID is provided
	if selection.SessionID != "" {
		np.createStickySession(selection.SessionID, selectedNode, selection)
	}

	// Mark node as busy temporarily
	np.markNodeBusy(selectedNode.ID)

	np.logger.Debugf("Selected node %s (%s, %s) for request", selectedNode.ID, selectedNode.IPAddress, selectedNode.Country)
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

	// Weighted random selection for maximum IP diversity
	// Shuffle nodes and pick from top candidates weighted by score
	// This ensures we spread traffic across all available nodes
	
	// Fisher-Yates shuffle
	shuffled := make([]*Node, len(nodes))
	copy(shuffled, nodes)
	for i := len(shuffled) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	// Pick from shuffled list with slight quality bias
	// Take top 50% by quality, then random from those
	if len(shuffled) > 4 {
		// Score all nodes
		type scored struct {
			node  *Node
			score float64
		}
		scoredNodes := make([]scored, len(shuffled))
		for i, n := range shuffled {
			scoredNodes[i] = scored{node: n, score: np.calculateNodeScore(n)}
		}
		// Sort by score descending (simple selection of top half)
		topHalf := len(scoredNodes) / 2
		if topHalf < 4 {
			topHalf = len(scoredNodes)
		}
		// Just pick random from the pool - maximize diversity
		return shuffled[rand.Intn(len(shuffled))]
	}

	return shuffled[rand.Intn(len(shuffled))]
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
	// 1. Last heartbeat was within 6 minutes (SDK sends every 5 min)
	// 2. Quality score is above 50
	// 3. Status is available

	if node.Status != "available" {
		return false
	}

	if node.QualityScore < 50 {
		return false
	}

	if time.Since(node.LastHeartbeat) > 6*time.Minute {
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
	cutoff := time.Now().Add(-10 * time.Minute) // 10 minutes without heartbeat (SDK sends every 5 min)

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
		"stats":        stats,
		"countries":    countries,
		"health_stats": np.GetHealthStats(),
		"timestamp":    time.Now().UTC(),
	}
}
// TEMP: Debug function
func init() {
	fmt.Println("[NODEPOOL DEBUG] Package initialized")
}

// ===== Health Check System =====

const (
	healthCheckInterval = 5 * time.Minute
	healthCheckTimeout  = 8 * time.Second
	healthKeyTTL        = 10 * time.Minute
	healthConcurrency   = 5 // max concurrent health checks
)

// healthCheckLoop runs continuously, checking all nodes periodically
func (np *NodePool) healthCheckLoop() {
	// Wait a bit on startup to let nodes register
	time.Sleep(30 * time.Second)
	np.logger.Info("Health checker started")

	// Run immediately on start, then every interval
	np.runHealthChecks()

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		np.runHealthChecks()
	}
}

// runHealthChecks checks all available nodes
func (np *NodePool) runHealthChecks() {
	ctx := context.Background()

	keys, err := np.rdb.Keys(ctx, "node:*").Result()
	if err != nil {
		np.logger.Errorf("Health check: failed to list nodes: %v", err)
		return
	}

	// Collect nodes that need checking
	var nodesToCheck []*Node
	for _, key := range keys {
		// Skip non-node keys (busy, etc)
		if strings.Contains(key, ":busy:") {
			continue
		}

		nodeData, err := np.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var node Node
		if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
			continue
		}

		if node.Status != "available" {
			continue
		}

		// Check if already has a valid health status that hasn't expired
		status := np.getHealthStatus(node.ID)
		if status == "verified" {
			// Already verified and TTL still valid, skip
			continue
		}

		nodesToCheck = append(nodesToCheck, &node)
	}

	if len(nodesToCheck) == 0 {
		return
	}

	np.logger.Debugf("Health check: %d nodes to check", len(nodesToCheck))

	// Use a semaphore to limit concurrency
	sem := make(chan struct{}, healthConcurrency)
	var wg sync.WaitGroup

	verified := 0
	failed := 0
	var mu sync.Mutex

	for _, node := range nodesToCheck {
		// Check if already being checked
		np.healthMu.Lock()
		if np.healthChecking[node.ID] {
			np.healthMu.Unlock()
			continue
		}
		np.healthChecking[node.ID] = true
		np.healthMu.Unlock()

		wg.Add(1)
		go func(n *Node) {
			defer wg.Done()
			defer func() {
				np.healthMu.Lock()
				delete(np.healthChecking, n.ID)
				np.healthMu.Unlock()
			}()

			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			ok := np.checkNodeHealth(n.ID)
			mu.Lock()
			if ok {
				verified++
			} else {
				failed++
			}
			mu.Unlock()
		}(node)
	}

	wg.Wait()

	if verified > 0 || failed > 0 {
		np.logger.Infof("Health check complete: %d verified, %d failed (of %d checked)", verified, failed, len(nodesToCheck))
	}
}

// checkNodeHealth tests a single node by sending an HTTP request through its tunnel
func (np *NodePool) checkNodeHealth(nodeID string) bool {
	ctx := context.Background()

	// Build WebSocket URL to node-registration tunnel endpoint
	wsURL := strings.Replace(np.nodeRegURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	tunnelURL := fmt.Sprintf("%s/internal/tunnel?node_id=%s&host=httpbin.org&port=80",
		wsURL, url.QueryEscape(nodeID))

	np.logger.Debugf("Health check node %s: connecting to %s", nodeID, tunnelURL)

	// Connect with timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.Dial(tunnelURL, nil)
	if err != nil {
		np.logger.Debugf("Health check node %s: tunnel connect failed: %v", nodeID, err)
		np.setHealthStatus(ctx, nodeID, "failed")
		return false
	}
	defer conn.Close()

	// Send HTTP request through the tunnel
	httpReq := "GET /ip HTTP/1.1\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n"
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte(httpReq)); err != nil {
		np.logger.Debugf("Health check node %s: write failed: %v", nodeID, err)
		np.setHealthStatus(ctx, nodeID, "failed")
		return false
	}

	// Read response with timeout
	conn.SetReadDeadline(time.Now().Add(healthCheckTimeout))

	var responseData []byte
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		responseData = append(responseData, msg...)
		// Check if we got a complete HTTP response
		if strings.Contains(string(responseData), "\r\n\r\n") {
			// We have headers at least - check for response body
			break
		}
	}

	responseStr := string(responseData)

	// Validate: must be a valid HTTP response with 200 status
	if strings.Contains(responseStr, "200 OK") || strings.Contains(responseStr, "200 ok") {
		np.logger.Debugf("Health check node %s: VERIFIED", nodeID)
		np.setHealthStatus(ctx, nodeID, "verified")
		return true
	}

	// Also accept if we got any HTTP response - the tunnel works
	if strings.HasPrefix(responseStr, "HTTP/") {
		// Parse status code
		scanner := bufio.NewScanner(strings.NewReader(responseStr))
		if scanner.Scan() {
			statusLine := scanner.Text()
			np.logger.Debugf("Health check node %s: got response '%s' - marking verified", nodeID, statusLine)
		}
		np.setHealthStatus(ctx, nodeID, "verified")
		return true
	}

	np.logger.Debugf("Health check node %s: FAILED (no valid HTTP response, got %d bytes)", nodeID, len(responseData))
	np.setHealthStatus(ctx, nodeID, "failed")
	return false
}

// setHealthStatus stores health status in Redis with TTL
func (np *NodePool) setHealthStatus(ctx context.Context, nodeID string, status string) {
	key := fmt.Sprintf("health:%s", nodeID)
	np.rdb.Set(ctx, key, status, healthKeyTTL)
}

// getHealthStatus retrieves health status from Redis
func (np *NodePool) getHealthStatus(nodeID string) string {
	ctx := context.Background()
	key := fmt.Sprintf("health:%s", nodeID)
	status, err := np.rdb.Get(ctx, key).Result()
	if err != nil {
		return "unchecked"
	}
	return status
}

// GetHealthStats returns verified/failed/unchecked counts
func (np *NodePool) GetHealthStats() map[string]int {
	ctx := context.Background()

	stats := map[string]int{
		"verified":  0,
		"failed":    0,
		"unchecked": 0,
	}

	keys, err := np.rdb.Keys(ctx, "node:*").Result()
	if err != nil {
		return stats
	}

	for _, key := range keys {
		if strings.Contains(key, ":busy:") {
			continue
		}

		nodeData, err := np.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var node Node
		if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
			continue
		}

		status := np.getHealthStatus(node.ID)
		switch status {
		case "verified":
			stats["verified"]++
		case "failed":
			stats["failed"]++
		default:
			stats["unchecked"]++
		}
	}

	return stats
}

// normalizeCity removes accents and lowercases for comparison
func normalizeCity(s string) string {
	// Simple replacements for common accented characters
	replacements := map[rune]rune{
		'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
		'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
		'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
		'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
		'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
		'ñ': 'n', 'ç': 'c',
		'Á': 'a', 'À': 'a', 'Â': 'a', 'Ã': 'a', 'Ä': 'a',
		'É': 'e', 'È': 'e', 'Ê': 'e', 'Ë': 'e',
		'Í': 'i', 'Ì': 'i', 'Î': 'i', 'Ï': 'i',
		'Ó': 'o', 'Ò': 'o', 'Ô': 'o', 'Õ': 'o', 'Ö': 'o',
		'Ú': 'u', 'Ù': 'u', 'Û': 'u', 'Ü': 'u',
		'Ñ': 'n', 'Ç': 'c',
	}
	var result []rune
	for _, r := range s {
		if replacement, ok := replacements[r]; ok {
			result = append(result, replacement)
		} else {
			result = append(result, r)
		}
	}
	return strings.ToLower(string(result))
}
