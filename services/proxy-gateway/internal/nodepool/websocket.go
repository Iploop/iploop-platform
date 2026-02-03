package nodepool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 64, // 64KB
	WriteBufferSize: 1024 * 64,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ConnectedNode represents an active WebSocket connection to a node
type ConnectedNode struct {
	ID            string
	Conn          *websocket.Conn
	Node          *Node
	PendingReqs   map[string]chan *ProxyResponse
	mu            sync.RWMutex
	LastPing      time.Time
	logger        *logrus.Entry
}

// WebSocketNodePool manages real-time WebSocket connections to nodes
type WebSocketNodePool struct {
	nodes       map[string]*ConnectedNode
	nodesByGeo  map[string][]*ConnectedNode // "country:city" -> nodes
	mu          sync.RWMutex
	pool        *NodePool // Reference to Redis-based pool for persistence
	logger      *logrus.Entry
}

// WebSocket message types
type WSMessage struct {
	Type      string      `json:"type"`
	RequestID string      `json:"requestId,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
}

type ProxyRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body,omitempty"` // base64 encoded
}

type ProxyResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body,omitempty"` // base64 encoded
	Error      string            `json:"error,omitempty"`
}

type NodeCapabilities struct {
	Version       string   `json:"version"`
	Platform      string   `json:"platform"`
	Protocols     []string `json:"protocols"`
	MaxConcurrent int      `json:"maxConcurrent"`
}

type HeartbeatData struct {
	BytesTransferred int64  `json:"bytesTransferred"`
	RequestsHandled  int64  `json:"requestsHandled"`
	Uptime           int64  `json:"uptime"`
	BatteryLevel     *int   `json:"batteryLevel,omitempty"`
	IsCharging       *bool  `json:"isCharging,omitempty"`
	ConnectionType   string `json:"connectionType,omitempty"`
}

func NewWebSocketNodePool(pool *NodePool, logger *logrus.Entry) *WebSocketNodePool {
	wsPool := &WebSocketNodePool{
		nodes:      make(map[string]*ConnectedNode),
		nodesByGeo: make(map[string][]*ConnectedNode),
		pool:       pool,
		logger:     logger.WithField("component", "ws-nodepool"),
	}

	// Start cleanup routine
	go wsPool.cleanupRoutine()

	return wsPool
}

// HandleNodeConnection handles a new WebSocket connection from a node
func (wp *WebSocketNodePool) HandleNodeConnection(w http.ResponseWriter, r *http.Request) {
	nodeID := r.Header.Get("X-Node-Id")
	authToken := r.Header.Get("Authorization")

	if nodeID == "" {
		http.Error(w, "Missing X-Node-Id header", http.StatusBadRequest)
		return
	}

	// TODO: Validate auth token
	_ = authToken

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		wp.logger.Errorf("WebSocket upgrade failed: %v", err)
		return
	}

	// Get node info from Redis
	node, err := wp.pool.GetNodeByID(nodeID)
	if err != nil {
		wp.logger.Warnf("Unknown node %s connecting, will register on capabilities", nodeID)
		node = &Node{ID: nodeID, Status: "connecting"}
	}

	connectedNode := &ConnectedNode{
		ID:          nodeID,
		Conn:        conn,
		Node:        node,
		PendingReqs: make(map[string]chan *ProxyResponse),
		LastPing:    time.Now(),
		logger:      wp.logger.WithField("node", nodeID),
	}

	// Register connection
	wp.registerNode(connectedNode)

	connectedNode.logger.Info("Node connected via WebSocket")

	// Handle messages
	go wp.handleNodeMessages(connectedNode)
}

func (wp *WebSocketNodePool) registerNode(node *ConnectedNode) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	// Remove old connection if exists
	if old, exists := wp.nodes[node.ID]; exists {
		old.Conn.Close()
	}

	wp.nodes[node.ID] = node

	// Update geo index if node has location
	if node.Node.Country != "" {
		geoKey := fmt.Sprintf("%s:%s", node.Node.Country, node.Node.City)
		wp.nodesByGeo[geoKey] = append(wp.nodesByGeo[geoKey], node)
	}
}

func (wp *WebSocketNodePool) unregisterNode(nodeID string) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	node, exists := wp.nodes[nodeID]
	if !exists {
		return
	}

	// Close any pending requests
	node.mu.Lock()
	for _, ch := range node.PendingReqs {
		close(ch)
	}
	node.mu.Unlock()

	// Remove from geo index
	if node.Node.Country != "" {
		geoKey := fmt.Sprintf("%s:%s", node.Node.Country, node.Node.City)
		nodes := wp.nodesByGeo[geoKey]
		for i, n := range nodes {
			if n.ID == nodeID {
				wp.nodesByGeo[geoKey] = append(nodes[:i], nodes[i+1:]...)
				break
			}
		}
	}

	delete(wp.nodes, nodeID)
	wp.logger.Infof("Node %s disconnected", nodeID)
}

func (wp *WebSocketNodePool) handleNodeMessages(node *ConnectedNode) {
	defer func() {
		wp.unregisterNode(node.ID)
		node.Conn.Close()
	}()

	for {
		_, messageData, err := node.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				node.logger.Errorf("WebSocket read error: %v", err)
			}
			return
		}

		var msg WSMessage
		if err := json.Unmarshal(messageData, &msg); err != nil {
			node.logger.Warnf("Invalid message format: %v", err)
			continue
		}

		switch msg.Type {
		case "capabilities":
			wp.handleCapabilities(node, msg.Payload)

		case "heartbeat":
			wp.handleHeartbeat(node, msg.Payload)

		case "proxy_response":
			wp.handleProxyResponse(node, msg.RequestID, msg.Payload)

		case "pong":
			node.LastPing = time.Now()

		default:
			node.logger.Warnf("Unknown message type: %s", msg.Type)
		}
	}
}

func (wp *WebSocketNodePool) handleCapabilities(node *ConnectedNode, payload interface{}) {
	data, _ := json.Marshal(payload)
	var caps NodeCapabilities
	json.Unmarshal(data, &caps)

	node.Node.SDKVersion = caps.Version
	node.Node.DeviceType = caps.Platform
	node.Node.Status = "available"

	// Update in Redis
	ctx := context.Background()
	nodeKey := fmt.Sprintf("node:%s:%s:%s", node.Node.Country, node.Node.City, node.ID)
	nodeData, _ := json.Marshal(node.Node)
	wp.pool.rdb.Set(ctx, nodeKey, nodeData, 5*time.Minute)

	node.logger.Infof("Node capabilities: %s %s, protocols=%v", caps.Platform, caps.Version, caps.Protocols)
}

func (wp *WebSocketNodePool) handleHeartbeat(node *ConnectedNode, payload interface{}) {
	data, _ := json.Marshal(payload)
	var hb HeartbeatData
	json.Unmarshal(data, &hb)

	node.Node.LastHeartbeat = time.Now()
	node.Node.BandwidthUsed = hb.BytesTransferred / (1024 * 1024) // Convert to MB
	node.Node.ConnectionType = hb.ConnectionType

	// Update in Redis
	ctx := context.Background()
	nodeKey := fmt.Sprintf("node:%s:%s:%s", node.Node.Country, node.Node.City, node.ID)
	nodeData, _ := json.Marshal(node.Node)
	wp.pool.rdb.Set(ctx, nodeKey, nodeData, 5*time.Minute)

	// Send back earnings update
	response := WSMessage{
		Type: "heartbeat_ack",
		Payload: map[string]interface{}{
			"earnings": 0.0001, // Calculate based on bandwidth
		},
	}
	responseData, _ := json.Marshal(response)
	node.Conn.WriteMessage(websocket.TextMessage, responseData)
}

func (wp *WebSocketNodePool) handleProxyResponse(node *ConnectedNode, requestID string, payload interface{}) {
	node.mu.RLock()
	ch, exists := node.PendingReqs[requestID]
	node.mu.RUnlock()

	if !exists {
		node.logger.Warnf("Response for unknown request: %s", requestID)
		return
	}

	data, _ := json.Marshal(payload)
	var resp ProxyResponse
	json.Unmarshal(data, &resp)

	ch <- &resp
}

// SelectConnectedNode selects an available WebSocket-connected node
func (wp *WebSocketNodePool) SelectConnectedNode(country, city string) (*ConnectedNode, error) {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	// Try exact geo match first
	if country != "" {
		geoKey := fmt.Sprintf("%s:%s", country, city)
		if nodes, exists := wp.nodesByGeo[geoKey]; exists && len(nodes) > 0 {
			return wp.selectBestConnectedNode(nodes), nil
		}

		// Try country-level match
		for key, nodes := range wp.nodesByGeo {
			if len(key) >= 2 && key[:2] == country[:2] && len(nodes) > 0 {
				return wp.selectBestConnectedNode(nodes), nil
			}
		}
	}

	// Fallback: any available node
	var availableNodes []*ConnectedNode
	for _, node := range wp.nodes {
		if node.Node.Status == "available" {
			availableNodes = append(availableNodes, node)
		}
	}

	if len(availableNodes) == 0 {
		return nil, fmt.Errorf("no connected nodes available")
	}

	return wp.selectBestConnectedNode(availableNodes), nil
}

func (wp *WebSocketNodePool) selectBestConnectedNode(nodes []*ConnectedNode) *ConnectedNode {
	if len(nodes) == 1 {
		return nodes[0]
	}

	// Simple selection: least pending requests
	best := nodes[0]
	bestPending := len(best.PendingReqs)

	for _, node := range nodes[1:] {
		pending := len(node.PendingReqs)
		if pending < bestPending {
			best = node
			bestPending = pending
		}
	}

	return best
}

// ExecuteProxyRequest sends a proxy request to a node and waits for response
func (wp *WebSocketNodePool) ExecuteProxyRequest(node *ConnectedNode, req *ProxyRequest, timeout time.Duration) (*ProxyResponse, error) {
	requestID := uuid.New().String()

	// Create response channel
	respChan := make(chan *ProxyResponse, 1)
	node.mu.Lock()
	node.PendingReqs[requestID] = respChan
	node.mu.Unlock()

	defer func() {
		node.mu.Lock()
		delete(node.PendingReqs, requestID)
		node.mu.Unlock()
	}()

	// Send request to node
	msg := WSMessage{
		Type:      "proxy_request",
		RequestID: requestID,
		Payload:   req,
	}
	msgData, _ := json.Marshal(msg)

	if err := node.Conn.WriteMessage(websocket.TextMessage, msgData); err != nil {
		return nil, fmt.Errorf("failed to send request to node: %v", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		if resp == nil {
			return nil, fmt.Errorf("node disconnected")
		}
		return resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("request timeout")
	}
}

// ForwardHTTPRequest forwards an HTTP request through a connected node
func (wp *WebSocketNodePool) ForwardHTTPRequest(r *http.Request, country, city string) (*http.Response, error) {
	node, err := wp.SelectConnectedNode(country, city)
	if err != nil {
		return nil, err
	}

	// Build proxy request
	var bodyBase64 string
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyBase64 = base64.StdEncoding.EncodeToString(bodyBytes)
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	proxyReq := &ProxyRequest{
		Method:  r.Method,
		URL:     r.URL.String(),
		Headers: headers,
		Body:    bodyBase64,
	}

	// Execute request
	resp, err := wp.ExecuteProxyRequest(node, proxyReq, 60*time.Second)
	if err != nil {
		return nil, err
	}

	// Build HTTP response
	httpResp := &http.Response{
		StatusCode: resp.StatusCode,
		Header:     make(http.Header),
	}

	for k, v := range resp.Headers {
		httpResp.Header.Set(k, v)
	}

	if resp.Body != "" {
		bodyBytes, _ := base64.StdEncoding.DecodeString(resp.Body)
		httpResp.Body = io.NopCloser(io.NewSectionReader(
			&bytesReaderAt{data: bodyBytes}, 0, int64(len(bodyBytes)),
		))
		httpResp.ContentLength = int64(len(bodyBytes))
	}

	return httpResp, nil
}

func (wp *WebSocketNodePool) cleanupRoutine() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		wp.mu.RLock()
		for _, node := range wp.nodes {
			// Send ping
			msg := WSMessage{Type: "ping"}
			msgData, _ := json.Marshal(msg)
			node.Conn.WriteMessage(websocket.TextMessage, msgData)

			// Check for stale connections
			if time.Since(node.LastPing) > 2*time.Minute {
				go func(nodeID string) {
					wp.unregisterNode(nodeID)
				}(node.ID)
			}
		}
		wp.mu.RUnlock()
	}
}

// GetStats returns current WebSocket pool statistics
func (wp *WebSocketNodePool) GetStats() map[string]interface{} {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	countries := make(map[string]int)
	totalPending := 0

	for _, node := range wp.nodes {
		countries[node.Node.Country]++
		totalPending += len(node.PendingReqs)
	}

	return map[string]interface{}{
		"connected_nodes":  len(wp.nodes),
		"pending_requests": totalPending,
		"countries":        countries,
		"timestamp":        time.Now().UTC(),
	}
}

// Helper for body reading
type bytesReaderAt struct {
	data []byte
}

func (b *bytesReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n = copy(p, b.data[off:])
	return n, nil
}
