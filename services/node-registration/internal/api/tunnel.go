package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	ws "node-registration/internal/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for internal API
	},
	ReadBufferSize:  65536,
	WriteBufferSize: 65536,
}

// TunnelHandler handles WebSocket tunnel connections from proxy-gateway
type TunnelHandler struct {
	tunnelManager *ws.TunnelManager
	logger        *logrus.Entry
}

// NewTunnelHandler creates a new tunnel handler
func NewTunnelHandler(tm *ws.TunnelManager, logger *logrus.Entry) *TunnelHandler {
	return &TunnelHandler{
		tunnelManager: tm,
		logger:        logger.WithField("component", "tunnel-api"),
	}
}

// TunnelOpenPayload for HTTP tunnel open request
type TunnelOpenPayload struct {
	NodeID string `json:"node_id"`
	Host   string `json:"host"`
	Port   string `json:"port"`
}

// HandleTunnelWebSocket handles WebSocket connection for bidirectional tunnel
func (h *TunnelHandler) HandleTunnelWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get tunnel parameters from query
	nodeID := r.URL.Query().Get("node_id")
	host := r.URL.Query().Get("host")
	port := r.URL.Query().Get("port")

	if nodeID == "" || host == "" || port == "" {
		http.Error(w, "Missing required parameters: node_id, host, port", http.StatusBadRequest)
		return
	}

	h.logger.Infof("Tunnel WebSocket request: node=%s target=%s:%s", nodeID, host, port)

	// Upgrade to WebSocket
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Errorf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Open tunnel to node
	tunnel, err := h.tunnelManager.OpenTunnel(nodeID, host, port)
	if err != nil {
		h.logger.Errorf("Failed to open tunnel: %v", err)
		conn.WriteJSON(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	defer h.tunnelManager.CloseTunnel(tunnel.ID)

	// OpenTunnel already waits for SDK confirmation, so we can start relay immediately
	h.logger.Infof("Tunnel %s established, starting relay", tunnel.ID)

	var wg sync.WaitGroup
	wg.Add(2)

	// Proxy -> Node (read from WebSocket, write to tunnel)
	go func() {
		defer wg.Done()
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					h.logger.Debugf("Tunnel %s WebSocket read error: %v", tunnel.ID, err)
				}
				return
			}

			if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
				if err := tunnel.Write(data); err != nil {
					h.logger.Debugf("Tunnel %s write error: %v", tunnel.ID, err)
					return
				}
			}
		}
	}()

	// Node -> Proxy (read from tunnel, write to WebSocket)
	go func() {
		defer wg.Done()
		h.logger.Infof("Tunnel %s relay Node->Proxy started", tunnel.ID[:8])
		for {
			data, err := tunnel.ReadWithTimeout(30 * time.Second)
			if err != nil {
				h.logger.Infof("Tunnel %s relay read error: %v", tunnel.ID[:8], err)
				conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}

			h.logger.Infof("Tunnel %s relay got %d bytes, writing to WS", tunnel.ID[:8], len(data))
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				h.logger.Infof("Tunnel %s relay WS write error: %v", tunnel.ID[:8], err)
				return
			}
			h.logger.Infof("Tunnel %s relay WS write OK", tunnel.ID[:8])
		}
	}()

	wg.Wait()
	h.logger.Infof("Tunnel %s closed", tunnel.ID)
}

// HandleTunnelHTTP handles HTTP-based tunnel (for simple use cases)
// This creates a tunnel and returns its ID, then client can use separate endpoints
func (h *TunnelHandler) HandleTunnelHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload TunnelOpenPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if payload.NodeID == "" || payload.Host == "" || payload.Port == "" {
		http.Error(w, "Missing required fields: node_id, host, port", http.StatusBadRequest)
		return
	}

	tunnel, err := h.tunnelManager.OpenTunnel(payload.NodeID, payload.Host, payload.Port)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"tunnel_id": tunnel.ID,
	})
}

// StreamingTunnelHandler handles HTTP streaming for tunnel data
// Uses chunked transfer encoding for bidirectional communication
type StreamingTunnelHandler struct {
	tunnelManager *ws.TunnelManager
	logger        *logrus.Entry
}

func NewStreamingTunnelHandler(tm *ws.TunnelManager, logger *logrus.Entry) *StreamingTunnelHandler {
	return &StreamingTunnelHandler{
		tunnelManager: tm,
		logger:        logger.WithField("component", "streaming-tunnel"),
	}
}

// HandleStream handles bidirectional HTTP streaming
func (h *StreamingTunnelHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	host := r.URL.Query().Get("host")
	port := r.URL.Query().Get("port")

	if nodeID == "" || host == "" || port == "" {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	// Open tunnel
	tunnel, err := h.tunnelManager.OpenTunnel(nodeID, host, port)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer h.tunnelManager.CloseTunnel(tunnel.ID)

	// Wait for tunnel to establish
	time.Sleep(100 * time.Millisecond)

	// Set headers for streaming
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Hijack the connection for true bidirectional
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		// Fallback to half-duplex mode
		h.halfDuplexStream(w, r, tunnel, flusher)
		return
	}

	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		h.logger.Errorf("Hijack failed: %v", err)
		return
	}
	defer conn.Close()

	h.logger.Infof("Hijacked connection for tunnel %s", tunnel.ID)

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Target
	go func() {
		defer wg.Done()
		buf := make([]byte, 32768)
		for {
			n, err := bufrw.Read(buf)
			if err != nil {
				if err != io.EOF {
					h.logger.Debugf("Read from client error: %v", err)
				}
				return
			}
			if n > 0 {
				if err := tunnel.Write(buf[:n]); err != nil {
					return
				}
			}
		}
	}()

	// Target -> Client
	go func() {
		defer wg.Done()
		for {
			data, err := tunnel.ReadWithTimeout(30 * time.Second)
			if err != nil {
				return
			}
			if _, err := conn.Write(data); err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

func (h *StreamingTunnelHandler) halfDuplexStream(w http.ResponseWriter, r *http.Request, tunnel *ws.Tunnel, flusher http.Flusher) {
	// Read request body and forward to tunnel
	go func() {
		buf := make([]byte, 32768)
		for {
			n, err := r.Body.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				tunnel.Write(buf[:n])
			}
		}
	}()

	// Read from tunnel and write to response
	for {
		data, err := tunnel.ReadWithTimeout(30 * time.Second)
		if err != nil {
			return
		}
		w.Write(data)
		flusher.Flush()
	}
}
