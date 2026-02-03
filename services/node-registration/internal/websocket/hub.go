package websocket

import (
	"encoding/json"
	"net/http"
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

type Hub struct {
	clients     map[*Client]bool
	register    chan *Client
	unregister  chan *Client
	broadcast   chan []byte
	nodeManager *nodemanager.NodeManager
	logger      *logrus.Entry
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	nodeID   string
	deviceID string
	logger   *logrus.Entry
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
}

func NewHub(nodeManager *nodemanager.NodeManager, logger *logrus.Entry) *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcast:   make(chan []byte),
		nodeManager: nodeManager,
		logger:      logger.WithField("component", "websocket-hub"),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.logger.Infof("Client connected from %s", client.conn.RemoteAddr())

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				
				// Mark node as disconnected
				if client.nodeID != "" {
					h.nodeManager.DisconnectNode(client.nodeID)
				}
				
				h.logger.Infof("Client disconnected: %s (node: %s)", client.deviceID, client.nodeID)
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) Close() {
	for client := range h.clients {
		client.conn.Close()
	}
}

func (h *Hub) GetConnectedCount() int {
	return len(h.clients)
}

func HandleNodeConnection(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		hub.logger.Errorf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		logger: hub.logger.WithField("client", conn.RemoteAddr().String()),
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

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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
	ticker := time.NewTicker(54 * time.Second)
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
			w.Write(message)

			// Add queued messages to the current message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
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
	default:
		c.logger.Warnf("Unknown message type: %s", message.Type)
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
	c.logger.Infof("Node registered: %s (device: %s)", node.ID, node.DeviceID)
}

func (c *Client) handleHeartbeat(message *Message) {
	if c.nodeID == "" {
		c.sendError("Node not registered")
		return
	}

	err := c.hub.nodeManager.UpdateHeartbeat(c.nodeID)
	if err != nil {
		c.logger.Errorf("Failed to update heartbeat: %v", err)
		c.sendError("Failed to update heartbeat")
		return
	}

	// Send heartbeat acknowledgment
	response := Message{
		Type: "heartbeat_ack",
		Data: map[string]interface{}{
			"node_id":   c.nodeID,
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