package websocket

import (
	"encoding/base64"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// TunnelManager manages bidirectional TCP tunnels through WebSocket
type TunnelManager struct {
	hub      *Hub
	tunnels  map[string]*Tunnel
	mu       sync.RWMutex
	logger   *logrus.Entry
}

// Tunnel represents an active TCP tunnel through a node
type Tunnel struct {
	ID        string
	NodeID    string
	Host      string
	Port      string
	Client    *Client
	DataCh    chan []byte    // Data from node to proxy
	WriteCh   chan []byte    // Data from proxy to node
	CloseCh   chan struct{}
	ReadyCh   chan bool      // Signals when tunnel is ready (SDK confirmed)
	CreatedAt time.Time
	mu        sync.Mutex
	closed    bool
	ready     bool
	readyErr  string
}

// TunnelOpenRequest is sent to node to open a tunnel
type TunnelOpenRequest struct {
	TunnelID string `json:"tunnel_id"`
	Host     string `json:"host"`
	Port     string `json:"port"`
}

// TunnelOpenResponse from node
type TunnelOpenResponse struct {
	TunnelID string `json:"tunnel_id"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// TunnelDataMessage for sending/receiving data
type TunnelDataMessage struct {
	TunnelID string `json:"tunnel_id"`
	Data     string `json:"data"` // Base64 encoded
	EOF      bool   `json:"eof"`  // End of stream
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager(hub *Hub, logger *logrus.Entry) *TunnelManager {
	tm := &TunnelManager{
		hub:     hub,
		tunnels: make(map[string]*Tunnel),
		logger:  logger.WithField("component", "tunnel-manager"),
	}

	// Start cleanup routine
	go tm.cleanupExpiredTunnels()

	return tm
}

// OpenTunnel opens a new tunnel through a node and waits for confirmation
func (tm *TunnelManager) OpenTunnel(nodeID, host, port string) (*Tunnel, error) {
	client := tm.hub.GetClientByNodeID(nodeID)
	if client == nil {
		return nil, ErrNodeNotConnected
	}

	tunnelID := uuid.New().String()

	tunnel := &Tunnel{
		ID:        tunnelID,
		NodeID:    nodeID,
		Host:      host,
		Port:      port,
		Client:    client,
		DataCh:    make(chan []byte, 256),
		WriteCh:   make(chan []byte, 256),
		CloseCh:   make(chan struct{}),
		ReadyCh:   make(chan bool, 1),
		CreatedAt: time.Now(),
	}

	tm.mu.Lock()
	tm.tunnels[tunnelID] = tunnel
	tm.mu.Unlock()

	// Send tunnel open request to node
	msg := &Message{
		Type: "tunnel_open",
		Data: &TunnelOpenRequest{
			TunnelID: tunnelID,
			Host:     host,
			Port:     port,
		},
	}

	client.sendMessage(msg)
	tm.logger.Infof("Opening tunnel %s to %s:%s via node %s", tunnelID, host, port, nodeID)

	// Wait for SDK to confirm tunnel is ready (with timeout)
	select {
	case success := <-tunnel.ReadyCh:
		if !success {
			tm.mu.Lock()
			delete(tm.tunnels, tunnelID)
			tm.mu.Unlock()
			return nil, &TunnelError{tunnel.readyErr}
		}
		tm.logger.Infof("Tunnel %s confirmed ready by SDK", tunnelID)
	case <-time.After(6 * time.Second):
		tm.mu.Lock()
		delete(tm.tunnels, tunnelID)
		tm.mu.Unlock()
		return nil, ErrTunnelTimeout
	}

	// Start writer goroutine for this tunnel
	go tm.tunnelWriter(tunnel)

	return tunnel, nil
}

// tunnelWriter sends data from WriteCh to the node
func (tm *TunnelManager) tunnelWriter(tunnel *Tunnel) {
	for {
		select {
		case data := <-tunnel.WriteCh:
			msg := &Message{
				Type: "tunnel_data",
				Data: &TunnelDataMessage{
					TunnelID: tunnel.ID,
					Data:     base64.StdEncoding.EncodeToString(data),
					EOF:      false,
				},
			}
			tunnel.Client.sendMessage(msg)
			
		case <-tunnel.CloseCh:
			return
		}
	}
}

// GetTunnel returns a tunnel by ID
func (tm *TunnelManager) GetTunnel(tunnelID string) *Tunnel {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tunnels[tunnelID]
}

// CloseTunnel closes a tunnel
func (tm *TunnelManager) CloseTunnel(tunnelID string) {
	tm.mu.Lock()
	tunnel, exists := tm.tunnels[tunnelID]
	if exists {
		delete(tm.tunnels, tunnelID)
	}
	tm.mu.Unlock()

	if tunnel != nil {
		tunnel.Close()

		// Send EOF to node
		msg := &Message{
			Type: "tunnel_data",
			Data: &TunnelDataMessage{
				TunnelID: tunnelID,
				EOF:      true,
			},
		}
		tunnel.Client.sendMessage(msg)
		
		tm.logger.Infof("Closed tunnel %s", tunnelID)
	}
}

// HandleTunnelResponse handles tunnel open response from node
func (tm *TunnelManager) HandleTunnelResponse(resp *TunnelOpenResponse) {
	tunnel := tm.GetTunnel(resp.TunnelID)
	if tunnel == nil {
		tm.logger.Warnf("Received response for unknown tunnel: %s", resp.TunnelID)
		return
	}

	tunnel.mu.Lock()
	tunnel.ready = resp.Success
	tunnel.readyErr = resp.Error
	tunnel.mu.Unlock()

	// Signal that tunnel is ready (or failed)
	select {
	case tunnel.ReadyCh <- resp.Success:
	default:
		// Channel might be full if response comes twice
	}

	if resp.Success {
		tm.logger.Infof("Tunnel %s opened successfully", resp.TunnelID)
	} else {
		tm.logger.Errorf("Tunnel %s failed to open: %s", resp.TunnelID, resp.Error)
	}
}

// HandleTunnelData handles incoming data from node
func (tm *TunnelManager) HandleTunnelData(data *TunnelDataMessage) {
	tm.logger.Infof("HandleTunnelData: tunnel=%s eof=%v dataLen=%d", data.TunnelID[:8], data.EOF, len(data.Data))
	tunnel := tm.GetTunnel(data.TunnelID)
	if tunnel == nil {
		tm.logger.Warnf("Received data for unknown tunnel: %s", data.TunnelID)
		return
	}

	if data.EOF {
		tm.logger.Infof("Tunnel %s received EOF from SDK", data.TunnelID)
		tm.CloseTunnel(data.TunnelID)
		return
	}

	// Decode and forward data
	decoded, err := base64.StdEncoding.DecodeString(data.Data)
	if err != nil {
		tm.logger.Errorf("Failed to decode tunnel data: %v", err)
		return
	}

	tm.logger.Infof("Tunnel %s decoded %d bytes from SDK", data.TunnelID[:8], len(decoded))

	// Non-blocking send to data channel
	select {
	case tunnel.DataCh <- decoded:
		tm.logger.Infof("Tunnel %s forwarded to DataCh OK", data.TunnelID[:8])
		tm.logger.Debugf("Tunnel %s forwarded %d bytes to DataCh", data.TunnelID, len(decoded))
	default:
		tm.logger.Warnf("Tunnel %s data channel full, dropping %d bytes", data.TunnelID, len(decoded))
	}
}

// Close closes a tunnel
func (t *Tunnel) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}
	t.closed = true
	close(t.CloseCh)
}

// IsClosed returns true if tunnel is closed
func (t *Tunnel) IsClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

// Write sends data through the tunnel to the target
func (t *Tunnel) Write(data []byte) error {
	if t.IsClosed() {
		return ErrTunnelClosed
	}

	select {
	case t.WriteCh <- data:
		return nil
	case <-t.CloseCh:
		return ErrTunnelClosed
	default:
		return ErrTunnelFull
	}
}

// Read receives data from the target through the tunnel
func (t *Tunnel) Read() ([]byte, error) {
	select {
	case data := <-t.DataCh:
		return data, nil
	case <-t.CloseCh:
		return nil, ErrTunnelClosed
	}
}

// ReadWithTimeout reads with a timeout
func (t *Tunnel) ReadWithTimeout(timeout time.Duration) ([]byte, error) {
	select {
	case data := <-t.DataCh:
		return data, nil
	case <-t.CloseCh:
		return nil, ErrTunnelClosed
	case <-time.After(timeout):
		return nil, ErrTunnelTimeout
	}
}

// cleanupExpiredTunnels removes old tunnels
func (tm *TunnelManager) cleanupExpiredTunnels() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		tm.mu.Lock()
		now := time.Now()
		for id, tunnel := range tm.tunnels {
			// Close tunnels older than 10 minutes with no activity
			if now.Sub(tunnel.CreatedAt) > 10*time.Minute {
				delete(tm.tunnels, id)
				tunnel.Close()
				tm.logger.Infof("Cleaned up expired tunnel %s", id)
			}
		}
		tm.mu.Unlock()
	}
}

// Errors
var (
	ErrNodeNotConnected = &TunnelError{"node not connected"}
	ErrTunnelClosed     = &TunnelError{"tunnel closed"}
	ErrTunnelFull       = &TunnelError{"tunnel buffer full"}
	ErrTunnelTimeout    = &TunnelError{"tunnel read timeout"}
)

type TunnelError struct {
	msg string
}

func (e *TunnelError) Error() string {
	return e.msg
}
