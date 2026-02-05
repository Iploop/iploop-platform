package websocket

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ProxyRequest represents a request to be proxied through a node
type ProxyRequest struct {
	RequestID string `json:"request_id"`
	Host      string `json:"host"`
	Port      string `json:"port"`
	Method    string `json:"method,omitempty"` // For HTTP requests
	URL       string `json:"url,omitempty"`    // For HTTP requests
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string `json:"body,omitempty"` // Base64 encoded
	Timeout   int    `json:"timeout_ms"`
	Profile   string `json:"profile,omitempty"` // Browser profile for User-Agent
}

// ProxyResponse represents the response from a proxied request
type ProxyResponse struct {
	RequestID  string `json:"request_id"`
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string `json:"body,omitempty"` // Base64 encoded
	Error      string `json:"error,omitempty"`
	BytesRead  int64  `json:"bytes_read"`
	BytesWrite int64  `json:"bytes_write"`
	LatencyMs  int64  `json:"latency_ms"`
}

// PendingRequest tracks a pending proxy request awaiting response
type PendingRequest struct {
	RequestID  string
	ResponseCh chan *ProxyResponse
	CreatedAt  time.Time
}

// ProxyManager handles routing proxy requests to connected nodes
type ProxyManager struct {
	hub            *Hub
	pendingMu      sync.RWMutex
	pendingReqs    map[string]*PendingRequest
	logger         *logrus.Entry
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(hub *Hub, logger *logrus.Entry) *ProxyManager {
	pm := &ProxyManager{
		hub:         hub,
		pendingReqs: make(map[string]*PendingRequest),
		logger:      logger.WithField("component", "proxy-manager"),
	}

	// Start cleanup routine for expired requests
	go pm.cleanupExpiredRequests()

	return pm
}

// SendProxyRequest sends a proxy request to a specific node and waits for response
func (pm *ProxyManager) SendProxyRequest(ctx context.Context, nodeID string, req *ProxyRequest) (*ProxyResponse, error) {
	// Find the client for this node
	client := pm.hub.GetClientByNodeID(nodeID)
	if client == nil {
		return nil, errors.New("node not connected")
	}

	// Generate request ID if not set
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}

	// Create pending request
	pending := &PendingRequest{
		RequestID:  req.RequestID,
		ResponseCh: make(chan *ProxyResponse, 1),
		CreatedAt:  time.Now(),
	}

	pm.pendingMu.Lock()
	pm.pendingReqs[req.RequestID] = pending
	pm.pendingMu.Unlock()

	// Cleanup on exit
	defer func() {
		pm.pendingMu.Lock()
		delete(pm.pendingReqs, req.RequestID)
		pm.pendingMu.Unlock()
	}()

	// Send request to node
	msg := &Message{
		Type: "proxy_request",
		Data: req,
	}

	client.sendMessage(msg)
	pm.logger.Debugf("Sent proxy request %s to node %s: %s:%s", req.RequestID, nodeID, req.Host, req.Port)

	// Wait for response or timeout
	timeout := time.Duration(req.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	select {
	case resp := <-pending.ResponseCh:
		return resp, nil
	case <-time.After(timeout):
		return nil, errors.New("proxy request timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HandleProxyResponse handles a proxy response from a node
func (pm *ProxyManager) HandleProxyResponse(resp *ProxyResponse) {
	pm.pendingMu.RLock()
	pending, exists := pm.pendingReqs[resp.RequestID]
	pm.pendingMu.RUnlock()

	if !exists {
		pm.logger.Warnf("Received response for unknown request: %s", resp.RequestID)
		return
	}

	// Non-blocking send
	select {
	case pending.ResponseCh <- resp:
		pm.logger.Debugf("Delivered proxy response %s (success=%v)", resp.RequestID, resp.Success)
	default:
		pm.logger.Warnf("Response channel full for request %s", resp.RequestID)
	}
}

// cleanupExpiredRequests removes old pending requests
func (pm *ProxyManager) cleanupExpiredRequests() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pm.pendingMu.Lock()
		now := time.Now()
		for id, req := range pm.pendingReqs {
			if now.Sub(req.CreatedAt) > 2*time.Minute {
				delete(pm.pendingReqs, id)
				pm.logger.Debugf("Cleaned up expired request %s", id)
			}
		}
		pm.pendingMu.Unlock()
	}
}

// GetClientByNodeID finds a connected client by node ID
func (h *Hub) GetClientByNodeID(nodeID string) *Client {
	for client := range h.clients {
		if client.nodeID == nodeID {
			return client
		}
	}
	return nil
}

// StreamTunnel handles TCP tunnel streaming (for CONNECT requests)
type StreamTunnel struct {
	RequestID string
	NodeID    string
	Client    *Client
	DataCh    chan []byte
	CloseCh   chan struct{}
	pm        *ProxyManager
}

// TunnelRequest for opening a TCP tunnel
type TunnelRequest struct {
	RequestID string `json:"request_id"`
	Host      string `json:"host"`
	Port      string `json:"port"`
}

// TunnelData for streaming data through tunnel
type TunnelData struct {
	RequestID string `json:"request_id"`
	Data      string `json:"data"` // Base64 encoded
	EOF       bool   `json:"eof"`
}

// OpenTunnel opens a TCP tunnel through a node
func (pm *ProxyManager) OpenTunnel(ctx context.Context, nodeID string, host, port string) (*StreamTunnel, error) {
	client := pm.hub.GetClientByNodeID(nodeID)
	if client == nil {
		return nil, errors.New("node not connected")
	}

	requestID := uuid.New().String()

	tunnel := &StreamTunnel{
		RequestID: requestID,
		NodeID:    nodeID,
		Client:    client,
		DataCh:    make(chan []byte, 100),
		CloseCh:   make(chan struct{}),
		pm:        pm,
	}

	// Register tunnel for receiving data
	pm.pendingMu.Lock()
	// We'll use a special prefix for tunnel data
	pm.pendingReqs["tunnel:"+requestID] = &PendingRequest{
		RequestID:  requestID,
		CreatedAt:  time.Now(),
	}
	pm.pendingMu.Unlock()

	// Send tunnel open request
	msg := &Message{
		Type: "tunnel_open",
		Data: &TunnelRequest{
			RequestID: requestID,
			Host:      host,
			Port:      port,
		},
	}

	client.sendMessage(msg)
	pm.logger.Debugf("Opening tunnel %s to %s:%s via node %s", requestID, host, port, nodeID)

	return tunnel, nil
}

// Write sends data through the tunnel
func (t *StreamTunnel) Write(data []byte) error {
	select {
	case <-t.CloseCh:
		return errors.New("tunnel closed")
	default:
	}

	msg := &Message{
		Type: "tunnel_data",
		Data: &TunnelData{
			RequestID: t.RequestID,
			Data:      base64.StdEncoding.EncodeToString(data),
			EOF:       false,
		},
	}

	t.Client.sendMessage(msg)
	return nil
}

// Close closes the tunnel
func (t *StreamTunnel) Close() error {
	select {
	case <-t.CloseCh:
		return nil // Already closed
	default:
		close(t.CloseCh)
	}

	// Send close message to node
	msg := &Message{
		Type: "tunnel_data",
		Data: &TunnelData{
			RequestID: t.RequestID,
			EOF:       true,
		},
	}

	t.Client.sendMessage(msg)

	// Cleanup
	t.pm.pendingMu.Lock()
	delete(t.pm.pendingReqs, "tunnel:"+t.RequestID)
	t.pm.pendingMu.Unlock()

	return nil
}

// Read receives data from the tunnel (called by hub when data arrives)
func (t *StreamTunnel) Read() ([]byte, error) {
	select {
	case data := <-t.DataCh:
		return data, nil
	case <-t.CloseCh:
		return nil, errors.New("tunnel closed")
	}
}
