package websocket

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"node-registration/internal/nodemanager"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin for MVP
	},
}

// IPGeoInfo contains geolocation data from IP lookup
type IPGeoInfo struct {
	Country     string  `json:"countryCode"`
	CountryName string  `json:"country"`
	City        string  `json:"city"`
	Region      string  `json:"regionName"`
	Latitude    float64 `json:"lat"`
	Longitude   float64 `json:"lon"`
	ISP         string  `json:"isp"`
	ASN         string  `json:"as"`
}

// isSDKVersionAllowed checks if SDK version is >= 1.0.62
func isSDKVersionAllowed(version string) bool {
	// Parse "1.0.62" format
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) != 3 {
		return false
	}
	var major, minor, patch int
	fmt.Sscanf(parts[0], "%d", &major)
	fmt.Sscanf(parts[1], "%d", &minor)
	fmt.Sscanf(parts[2], "%d", &patch)

	// Minimum: 1.0.62
	if major > 1 { return true }
	if major < 1 { return false }
	if minor > 0 { return true }
	if minor < 0 { return false }
	return patch >= 62
}

// lookupIPGeo fetches geolocation for an IP address
func lookupIPGeo(ip string) (*IPGeoInfo, error) {
	// Skip private/local IPs
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") || 
	   strings.HasPrefix(ip, "172.") || ip == "127.0.0.1" || ip == "::1" {
		return nil, fmt.Errorf("private IP address")
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s", ip))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var geo IPGeoInfo
	if err := json.Unmarshal(body, &geo); err != nil {
		return nil, err
	}

	return &geo, nil
}

// extractClientIP gets the real client IP from the request
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxied connections)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

type Hub struct {
	clients       map[*Client]bool
	clientsMu     sync.RWMutex
	register      chan *Client
	unregister    chan *Client
	broadcast     chan []byte
	nodeManager   *nodemanager.NodeManager
	proxyManager  *ProxyManager
	tunnelManager *TunnelManager
	logger        *logrus.Entry
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	nodeID    string
	deviceID  string
	clientIP  string
	logger    *logrus.Entry
}

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type HeartbeatMessage struct {
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`
}

type RegistrationMessage struct {
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
	// Fields from ipinfo.io (sent by SDK v1.0.62+)
	Loc            string  `json:"loc"`      // "lat,lng" format
	Org            string  `json:"org"`      // "AS12345 ISP Name" format
	Timezone       string  `json:"timezone"`
}

func NewHub(nodeManager *nodemanager.NodeManager, logger *logrus.Entry) *Hub {
	hub := &Hub{
		clients:     make(map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcast:   make(chan []byte),
		nodeManager: nodeManager,
		logger:      logger.WithField("component", "websocket-hub"),
	}
	// ProxyManager will be set after hub creation
	return hub
}

func (h *Hub) SetProxyManager(pm *ProxyManager) {
	h.proxyManager = pm
}

func (h *Hub) GetProxyManager() *ProxyManager {
	return h.proxyManager
}

func (h *Hub) SetTunnelManager(tm *TunnelManager) {
	h.tunnelManager = tm
}

func (h *Hub) GetTunnelManager() *TunnelManager {
	return h.tunnelManager
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			h.clients[client] = true
			h.clientsMu.Unlock()
			h.logger.Infof("Client connected from %s", client.conn.RemoteAddr())

		case client := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.clientsMu.Unlock()

				// Mark node as disconnected (outside lock)
				if client.nodeID != "" {
					h.nodeManager.DisconnectNode(client.nodeID)
				}

				h.logger.Infof("Client disconnected: %s (node: %s)", client.deviceID, client.nodeID)
			} else {
				h.clientsMu.Unlock()
			}

		case message := <-h.broadcast:
			h.clientsMu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.clientsMu.RUnlock()
		}
	}
}

func (h *Hub) Close() {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	for client := range h.clients {
		client.conn.Close()
	}
}

func (h *Hub) GetConnectedCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// GetConnectedNodeIDs returns all node IDs with active WebSocket connections
func (h *Hub) GetConnectedNodeIDs() []string {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	ids := make([]string, 0, len(h.clients))
	for client := range h.clients {
		if client.nodeID != "" {
			ids = append(ids, client.nodeID)
		}
	}
	return ids
}

func HandleNodeConnection(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		hub.logger.Errorf("WebSocket upgrade error: %v", err)
		return
	}

	clientIP := extractClientIP(r)

	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		clientIP: clientIP,
		logger:   hub.logger.WithField("client", clientIP),
	}

	client.hub.register <- client

	// Start goroutines for handling read and write
	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(524288) // 512KB - for proxy responses with base64 payload
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Errorf("WebSocket error: %v", err)
			}
			break
		}

		var message Message
		if err := json.Unmarshal(messageBytes, &message); err != nil {
			c.logger.Errorf("Failed to parse message: %v", err)
			continue
		}

		c.handleMessage(&message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(2 * time.Minute)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			// Collect all queued messages
			n := len(c.send)
			if n == 0 {
				// Single message - send as-is (no array wrapper)
				w.Write(message)
			} else {
				// Multiple messages - wrap in JSON array
				w.Write([]byte{'['})
				w.Write(message)
				for i := 0; i < n; i++ {
					w.Write([]byte{','})
					w.Write(<-c.send)
				}
				w.Write([]byte{']'})
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(message *Message) {
	switch message.Type {
	case "register":
		c.handleRegistration(message)
	case "heartbeat":
		c.handleHeartbeat(message)
	case "proxy_response":
		c.handleProxyResponse(message)
	case "tunnel_response":
		c.handleTunnelResponse(message)
	case "tunnel_data":
		c.handleTunnelData(message)
	default:
		c.logger.Warnf("Unknown message type: %s", message.Type)
	}
}

func (c *Client) handleProxyResponse(message *Message) {
	dataBytes, err := json.Marshal(message.Data)
	if err != nil {
		c.logger.Errorf("Failed to marshal proxy response: %v", err)
		return
	}

	var resp ProxyResponse
	if err := json.Unmarshal(dataBytes, &resp); err != nil {
		c.logger.Errorf("Failed to parse proxy response: %v", err)
		return
	}

	if c.hub.proxyManager != nil {
		c.hub.proxyManager.HandleProxyResponse(&resp)
	}
}

func (c *Client) handleTunnelResponse(message *Message) {
	dataBytes, err := json.Marshal(message.Data)
	if err != nil {
		c.logger.Errorf("Failed to marshal tunnel response: %v", err)
		return
	}

	var resp TunnelOpenResponse
	if err := json.Unmarshal(dataBytes, &resp); err != nil {
		c.logger.Errorf("Failed to parse tunnel response: %v", err)
		return
	}

	if c.hub.tunnelManager != nil {
		c.hub.tunnelManager.HandleTunnelResponse(&resp)
	}
}

func (c *Client) handleTunnelData(message *Message) {
	dataBytes, err := json.Marshal(message.Data)
	if err != nil {
		c.logger.Errorf("Failed to marshal tunnel data: %v", err)
		return
	}

	var data TunnelDataMessage
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		c.logger.Errorf("Failed to parse tunnel data: %v", err)
		return
	}

	if c.hub.tunnelManager != nil {
		c.hub.tunnelManager.HandleTunnelData(&data)
	}
}

func (c *Client) handleRegistration(message *Message) {
	dataBytes, err := json.Marshal(message.Data)
	if err != nil {
		c.sendError("Failed to parse registration data")
		return
	}

	var regData RegistrationMessage
	if err := json.Unmarshal(dataBytes, &regData); err != nil {
		c.sendError("Invalid registration data format")
		return
	}

	// Debug log the received data
	c.logger.Infof("Registration data: device_id=%s, device_type=%s, connection_type=%s, sdk_version=%s",
		regData.DeviceID, regData.DeviceType, regData.ConnectionType, regData.SDKVersion)

	// Reject old SDK versions (before 1.0.62)
	if !isSDKVersionAllowed(regData.SDKVersion) {
		c.logger.Warnf("Rejected old SDK %s from device %s", regData.SDKVersion, regData.DeviceID)
		c.sendError("SDK version too old. Minimum required: 1.0.62")
		return
	}

	// Map empty/unknown connection types to "unknown"
	if regData.ConnectionType == "" {
		regData.ConnectionType = "unknown"
	}

	// Use client's real IP address
	regData.IPAddress = c.clientIP

	// Parse ipinfo.io fields if SDK provided them (v1.0.62+)
	if regData.Country != "" {
		// SDK provided geo data — parse loc and org fields
		if regData.Loc != "" && regData.Latitude == 0 && regData.Longitude == 0 {
			parts := strings.SplitN(regData.Loc, ",", 2)
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &regData.Latitude)
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &regData.Longitude)
			}
		}
		if regData.Org != "" && regData.ISP == "" {
			// Parse "AS12345 ISP Name" format
			orgParts := strings.SplitN(regData.Org, " ", 2)
			if len(orgParts) >= 1 && strings.HasPrefix(orgParts[0], "AS") {
				fmt.Sscanf(orgParts[0], "AS%d", &regData.ASN)
			}
			if len(orgParts) >= 2 {
				regData.ISP = orgParts[1]
			}
		}
		if regData.CountryName == "" {
			regData.CountryName = regData.Country
		}
		c.logger.Infof("SDK provided geo: %s, %s (sdk_version=%s)", regData.Country, regData.City, regData.SDKVersion)
	} else {
		// No geo from SDK — fall back to server-side lookup
		geo, err := lookupIPGeo(c.clientIP)
		if err == nil && geo != nil {
			regData.Country = geo.Country
			regData.CountryName = geo.CountryName
			regData.City = geo.City
			regData.Region = geo.Region
			regData.Latitude = geo.Latitude
			regData.Longitude = geo.Longitude
			regData.ISP = geo.ISP
			if geo.ASN != "" {
				var asn int
				fmt.Sscanf(geo.ASN, "AS%d", &asn)
				regData.ASN = asn
			}
			c.logger.Infof("Server geo lookup: %s -> %s, %s", c.clientIP, regData.Country, regData.City)
		} else {
			c.logger.Warnf("Geo lookup failed for %s: %v", c.clientIP, err)
		}
	}

	// Convert to node manager format
	registration := &nodemanager.NodeRegistration{
		DeviceID:       regData.DeviceID,
		IPAddress:      regData.IPAddress,
		Country:        regData.Country,
		CountryName:    regData.CountryName,
		City:           regData.City,
		Region:         regData.Region,
		Latitude:       regData.Latitude,
		Longitude:      regData.Longitude,
		ASN:            regData.ASN,
		ISP:            regData.ISP,
		Carrier:        regData.Carrier,
		ConnectionType: regData.ConnectionType,
		DeviceType:     regData.DeviceType,
		SDKVersion:     regData.SDKVersion,
	}

	node, err := c.hub.nodeManager.RegisterNode(registration)
	if err != nil {
		c.logger.Errorf("Failed to register node: %v", err)
		c.sendError("Failed to register node")
		return
	}

	// Store node information in client
	c.nodeID = node.ID
	c.deviceID = node.DeviceID

	// Send registration success
	response := Message{
		Type: "registration_success",
		Data: map[string]interface{}{
			"node_id":    node.ID,
			"status":     "registered",
			"message":    "Node successfully registered",
			"timestamp":  time.Now().UTC(),
		},
	}

	c.sendMessage(&response)
	c.logger.Infof("Node registered: %s (device: %s, ip: %s, country: %s)", 
		node.ID, node.DeviceID, c.clientIP, regData.Country)
}

func (c *Client) handleHeartbeat(message *Message) {
	// Use stored nodeID if not provided in message
	nodeID := c.nodeID
	
	// Try to extract node_id from data if present
	if dataMap, ok := message.Data.(map[string]interface{}); ok {
		if id, exists := dataMap["node_id"]; exists {
			if idStr, ok := id.(string); ok && idStr != "" {
				nodeID = idStr
			}
		}
	}

	if nodeID == "" {
		c.sendError("Node not registered")
		return
	}

	err := c.hub.nodeManager.UpdateHeartbeat(nodeID)
	if err != nil {
		c.logger.Errorf("Failed to update heartbeat: %v", err)
		c.sendError("Failed to update heartbeat")
		return
	}

	// Send heartbeat acknowledgment
	response := Message{
		Type: "heartbeat_ack",
		Data: map[string]interface{}{
			"node_id":   nodeID,
			"timestamp": time.Now().UTC(),
			"status":    "active",
		},
	}

	c.sendMessage(&response)
}

func (c *Client) sendMessage(message *Message) {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		c.logger.Errorf("Failed to marshal message: %v", err)
		return
	}

	select {
	case c.send <- messageBytes:
	default:
		c.hub.unregister <- c
		close(c.send)
	}
}

func (c *Client) sendError(errorMsg string) {
	response := Message{
		Type: "error",
		Data: map[string]interface{}{
			"error":     errorMsg,
			"timestamp": time.Now().UTC(),
		},
	}
	c.sendMessage(&response)
}