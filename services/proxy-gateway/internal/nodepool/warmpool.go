package nodepool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	// How often we pick random nodes to test
	warmCheckInterval = 10 * time.Second
	// Timeout for a single health probe through the warm pool
	warmProbeTimeout = 5 * time.Second
	// Redis key TTL for fast-lane nodes
	fastNodeTTL = 10 * time.Minute
	// Redis key prefix for fast-lane nodes
	fastNodePrefix = "fastnode:"
	// Max concurrent warm probes
	warmConcurrency = 32
	// How often to log warm pool stats
	warmStatsInterval = 2 * time.Minute
)

// WarmPool pre-validates nodes and maintains a "fast lane" of recently-verified
// nodes that responded quickly. This avoids cold-start latency when a customer
// request arrives — SelectNode checks fast-lane nodes first.
type WarmPool struct {
	nodePool   *NodePool
	nodeRegURL string
	logger     *logrus.Entry
	stopCh     chan struct{}
	wg         sync.WaitGroup

	// stats (atomic for lock-free reads)
	probesRun    int64
	probesOK     int64
	probesFail   int64
	fastHits     int64
	fastMisses   int64
}

// NewWarmPool creates and starts the warm pool background goroutines.
func NewWarmPool(nodePool *NodePool, nodeRegURL string, logger *logrus.Entry) *WarmPool {
	wp := &WarmPool{
		nodePool:   nodePool,
		nodeRegURL: nodeRegURL,
		logger:     logger.WithField("component", "warm-pool"),
		stopCh:     make(chan struct{}),
	}

	wp.wg.Add(2)
	go wp.warmBackground()
	go wp.statsReporter()

	wp.logger.Info("Warm pool started")
	return wp
}

// Stop gracefully shuts down the warm pool.
func (wp *WarmPool) Stop() {
	close(wp.stopCh)
	wp.wg.Wait()
	wp.logger.Info("Warm pool stopped")
}

// GetFastNode returns a fast-lane node ID for the given country, or "" if none.
// The caller should fall back to normal selection when this returns "".
func (wp *WarmPool) GetFastNode(country string) string {
	ctx := context.Background()

	// Try country-specific fast nodes first
	if country != "" {
		key := fmt.Sprintf("%s%s", fastNodePrefix, strings.ToUpper(country))
		members, err := wp.nodePool.rdb.SMembers(ctx, key).Result()
		if err == nil && len(members) > 0 {
			// Pick a random fast node and verify it's not blacklisted
			shuffled := shuffleStrings(members)
			for _, nodeID := range shuffled {
				if !wp.nodePool.IsNodeBlacklisted(nodeID) && wp.nodePool.IsConnected(nodeID) {
					atomic.AddInt64(&wp.fastHits, 1)
					wp.nodePool.rdb.SRem(ctx, key, nodeID)
					return nodeID
				}
			}
		}
	}

	// Try "any" pool
	key := fmt.Sprintf("%sANY", fastNodePrefix)
	members, err := wp.nodePool.rdb.SMembers(ctx, key).Result()
	if err == nil && len(members) > 0 {
		shuffled := shuffleStrings(members)
		for _, nodeID := range shuffled {
			if !wp.nodePool.IsNodeBlacklisted(nodeID) && wp.nodePool.IsConnected(nodeID) {
				// If country was requested, verify the node matches
				if country != "" {
					node, err := wp.nodePool.GetNodeByID(nodeID)
					if err != nil || strings.ToUpper(node.Country) != strings.ToUpper(country) {
						continue
					}
				}
				atomic.AddInt64(&wp.fastHits, 1)
				wp.nodePool.rdb.SRem(ctx, key, nodeID)
				return nodeID
			}
		}
	}

	atomic.AddInt64(&wp.fastMisses, 1)
	return ""
}

// warmBackground continuously probes random nodes and adds fast ones to Redis.
func (wp *WarmPool) warmBackground() {
	defer wp.wg.Done()

	// Wait for startup
	select {
	case <-time.After(5 * time.Second):
	case <-wp.stopCh:
		return
	}

	ticker := time.NewTicker(warmCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wp.probeRandomNodes()
		case <-wp.stopCh:
			return
		}
	}
}

// probeRandomNodes picks a batch of random available nodes and tests them.
func (wp *WarmPool) probeRandomNodes() {
	ctx := context.Background()

	// Only probe nodes that are actually connected via WebSocket
	connectedIDs := wp.nodePool.GetConnectedNodeIDs()
	if len(connectedIDs) == 0 {
		return
	}

	// Collect available, non-blacklisted, connected nodes
	var candidates []*Node
	for _, nodeID := range connectedIDs {
		if wp.nodePool.IsNodeBlacklisted(nodeID) {
			continue
		}
		// Skip nodes already in fast lane
		if wp.isInFastLane(ctx, nodeID) {
			continue
		}
		nodeKey := fmt.Sprintf("node:%s", nodeID)
		nodeData, err := wp.nodePool.rdb.Get(ctx, nodeKey).Result()
		if err != nil {
			continue
		}
		var node Node
		if err := json.Unmarshal([]byte(nodeData), &node); err != nil {
			continue
		}
		if node.Status == "available" {
			candidates = append(candidates, &node)
		}
	}

	if len(candidates) == 0 {
		return
	}

	// Shuffle and pick up to warmConcurrency nodes
	for i := len(candidates) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}

	batch := candidates
	if len(batch) > warmConcurrency {
		batch = batch[:warmConcurrency]
	}

	// Probe concurrently
	sem := make(chan struct{}, warmConcurrency)
	var wg sync.WaitGroup

	for _, node := range batch {
		wg.Add(1)
		go func(n *Node) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			wp.probeNode(n)
		}(node)
	}

	wg.Wait()
}

// probeNode tests a single node by opening a tunnel to httpbin.org and sending GET /ip.
func (wp *WarmPool) probeNode(node *Node) {
	atomic.AddInt64(&wp.probesRun, 1)

	start := time.Now()

	// Build WebSocket URL
	wsURL := strings.Replace(wp.nodeRegURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	tunnelURL := fmt.Sprintf("%s/internal/tunnel?node_id=%s&host=httpbin.org&port=80",
		wsURL, url.QueryEscape(node.ID))

	dialer := websocket.Dialer{
		HandshakeTimeout: warmProbeTimeout,
	}

	conn, _, err := dialer.Dial(tunnelURL, nil)
	if err != nil {
		atomic.AddInt64(&wp.probesFail, 1)
		wp.logger.Debugf("Warm probe failed for node %s: dial error: %v", node.ID, err)
		return
	}
	defer conn.Close()

	// Send HTTP request
	httpReq := "GET /ip HTTP/1.1\r\nHost: httpbin.org\r\nConnection: close\r\n\r\n"
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte(httpReq)); err != nil {
		atomic.AddInt64(&wp.probesFail, 1)
		wp.logger.Debugf("Warm probe failed for node %s: write error: %v", node.ID, err)
		return
	}

	// Read response with timeout
	conn.SetReadDeadline(time.Now().Add(warmProbeTimeout))

	var responseData []byte
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		responseData = append(responseData, msg...)
		if strings.Contains(string(responseData), "\r\n\r\n") {
			break
		}
	}

	elapsed := time.Since(start)

	// Validate response
	responseStr := string(responseData)
	if !strings.HasPrefix(responseStr, "HTTP/") {
		atomic.AddInt64(&wp.probesFail, 1)
		wp.logger.Debugf("Warm probe failed for node %s: no HTTP response (%d bytes in %v)", node.ID, len(responseData), elapsed)
		return
	}

	// Check for valid status
	scanner := bufio.NewScanner(strings.NewReader(responseStr))
	if scanner.Scan() {
		statusLine := scanner.Text()
		if !strings.Contains(statusLine, "200") {
			// Non-200 but still a valid response — node works
			wp.logger.Debugf("Warm probe node %s: got %s in %v — adding to fast lane", node.ID, statusLine, elapsed)
		}
	}

	// Only add if response was fast enough
	if elapsed > warmProbeTimeout {
		atomic.AddInt64(&wp.probesFail, 1)
		wp.logger.Debugf("Warm probe node %s: too slow (%v)", node.ID, elapsed)
		return
	}

	// Success — add to fast lane in Redis
	atomic.AddInt64(&wp.probesOK, 1)
	wp.addToFastLane(node)
	wp.logger.Debugf("Warm probe node %s (%s): OK in %v — added to fast lane", node.ID, node.Country, elapsed)
}

// addToFastLane adds a verified node to the Redis fast-lane sets.
func (wp *WarmPool) addToFastLane(node *Node) {
	ctx := context.Background()

	// Add to country-specific set
	country := strings.ToUpper(node.Country)
	if country != "" {
		key := fmt.Sprintf("%s%s", fastNodePrefix, country)
		wp.nodePool.rdb.SAdd(ctx, key, node.ID)
		wp.nodePool.rdb.Expire(ctx, key, fastNodeTTL)
	}

	// Also add to "ANY" set
	anyKey := fmt.Sprintf("%sANY", fastNodePrefix)
	wp.nodePool.rdb.SAdd(ctx, anyKey, node.ID)
	wp.nodePool.rdb.Expire(ctx, anyKey, fastNodeTTL)
}

// isInFastLane checks if a node is already in any fast-lane set.
func (wp *WarmPool) isInFastLane(ctx context.Context, nodeID string) bool {
	anyKey := fmt.Sprintf("%sANY", fastNodePrefix)
	isMember, err := wp.nodePool.rdb.SIsMember(ctx, anyKey, nodeID).Result()
	if err != nil {
		return false
	}
	return isMember
}

// statsReporter logs warm pool statistics periodically.
func (wp *WarmPool) statsReporter() {
	defer wp.wg.Done()

	ticker := time.NewTicker(warmStatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			// Count fast nodes across all sets
			fastKeys, _ := wp.nodePool.rdb.Keys(ctx, fmt.Sprintf("%s*", fastNodePrefix)).Result()
			totalFast := 0
			for _, key := range fastKeys {
				count, _ := wp.nodePool.rdb.SCard(ctx, key).Result()
				totalFast += int(count)
			}

			wp.logger.Infof("Warm pool stats: probes=%d ok=%d fail=%d fast_nodes=%d hits=%d misses=%d",
				atomic.LoadInt64(&wp.probesRun),
				atomic.LoadInt64(&wp.probesOK),
				atomic.LoadInt64(&wp.probesFail),
				totalFast,
				atomic.LoadInt64(&wp.fastHits),
				atomic.LoadInt64(&wp.fastMisses),
			)
		case <-wp.stopCh:
			return
		}
	}
}

// GetStats returns warm pool statistics for the status API.
func (wp *WarmPool) GetStats() map[string]interface{} {
	ctx := context.Background()

	// Count fast nodes per country
	fastKeys, _ := wp.nodePool.rdb.Keys(ctx, fmt.Sprintf("%s*", fastNodePrefix)).Result()
	fastByCountry := make(map[string]int64)
	totalFast := int64(0)

	for _, key := range fastKeys {
		count, _ := wp.nodePool.rdb.SCard(ctx, key).Result()
		country := strings.TrimPrefix(key, fastNodePrefix)
		fastByCountry[country] = count
		if country != "ANY" {
			totalFast += count
		}
	}

	return map[string]interface{}{
		"probes_run":       atomic.LoadInt64(&wp.probesRun),
		"probes_ok":        atomic.LoadInt64(&wp.probesOK),
		"probes_fail":      atomic.LoadInt64(&wp.probesFail),
		"fast_nodes_total": totalFast,
		"fast_by_country":  fastByCountry,
		"fast_hits":        atomic.LoadInt64(&wp.fastHits),
		"fast_misses":      atomic.LoadInt64(&wp.fastMisses),
	}
}

// shuffleStrings returns a shuffled copy of the input slice.
func shuffleStrings(s []string) []string {
	result := make([]string, len(s))
	copy(result, s)
	for i := len(result) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		result[i], result[j] = result[j], result[i]
	}
	return result
}
