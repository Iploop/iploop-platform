package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "modernc.org/sqlite"
	_ "net/http/pprof"
)

// ─── Constants ─────────────────────────────────────────────────────────────────

const (
	pingInterval   = 60 * time.Second
	pongWait       = 45 * time.Second
	writeWait      = 10 * time.Second
	maxMessageSize = 2097152 // 2MB for proxy responses with base64 payloads
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// ─── Connection ────────────────────────────────────────────────────────────────

// Connection tracks a single node
type Connection struct {
	NodeID         string    `json:"node_id"`
	DeviceID       string    `json:"device_id"`
	DeviceModel    string    `json:"device_model"`
	IP             string    `json:"ip"`
	Country        string    `json:"country"`
	City           string    `json:"city"`
	ISP            string    `json:"isp"`
	ASN            string    `json:"asn"`
	SDKVersion     string    `json:"sdk_version"`
	ConnectionType string    `json:"connection_type"`
	DeviceType     string    `json:"device_type"`
	ConnectedAt    time.Time `json:"connected_at"`
	LastPong       time.Time `json:"last_pong"`
	PingsSent      int64     `json:"pings_sent"`
	PongsRecv      int64     `json:"pongs_received"`
	HasIPInfo      bool      `json:"has_ip_info"`
	Registered     bool      `json:"registered"`

	// WebSocket writer — guarded by write mutex
	Ws        *websocket.Conn         `json:"-"`
	SafeWrite func(int, []byte) error `json:"-"`
	SendCh    chan []byte              `json:"-"`
	BinaryCh  chan []byte              `json:"-"` // Binary tunnel data frames
}

// ─── Message types ─────────────────────────────────────────────────────────────

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
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
	Loc            string  `json:"loc"`
	Org            string  `json:"org"`
	Timezone       string  `json:"timezone"`
}

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

// ─── Tunnel types ──────────────────────────────────────────────────────────────

type TunnelOpenRequest struct {
	TunnelID string `json:"tunnel_id"`
	Host     string `json:"host"`
	Port     string `json:"port"`
}

type TunnelOpenResponse struct {
	TunnelID string `json:"tunnel_id"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

type TunnelDataMessage struct {
	TunnelID string `json:"tunnel_id"`
	Data     string `json:"data"`
	RawData  []byte `json:"-"` // Binary data (no base64)
	EOF      bool   `json:"eof"`
}

type Tunnel struct {
	ID        string
	NodeID    string
	Host      string
	Port      string
	Conn      *Connection
	DataCh    chan []byte
	WriteCh   chan []byte
	CloseCh   chan struct{}
	ReadyCh   chan bool
	CreatedAt time.Time
	mu        sync.Mutex
	closed    bool
	ready     bool
	readyErr  string
}

func (t *Tunnel) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	close(t.CloseCh)
}

func (t *Tunnel) IsClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

func (t *Tunnel) Write(data []byte) error {
	if t.IsClosed() {
		return fmt.Errorf("tunnel closed")
	}
	select {
	case t.WriteCh <- data:
		return nil
	case <-t.CloseCh:
		return fmt.Errorf("tunnel closed")
	default:
		return fmt.Errorf("tunnel buffer full")
	}
}

func (t *Tunnel) Read() ([]byte, error) {
	select {
	case data := <-t.DataCh:
		return data, nil
	case <-t.CloseCh:
		return nil, fmt.Errorf("tunnel closed")
	}
}

func (t *Tunnel) ReadWithTimeout(timeout time.Duration) ([]byte, error) {
	select {
	case data := <-t.DataCh:
		return data, nil
	case <-t.CloseCh:
		return nil, fmt.Errorf("tunnel closed")
	case <-time.After(timeout):
		return nil, fmt.Errorf("tunnel read timeout")
	}
}

// ─── Proxy types ───────────────────────────────────────────────────────────────

type ProxyRequest struct {
	RequestID string            `json:"request_id"`
	Host      string            `json:"host"`
	Port      string            `json:"port"`
	Method    string            `json:"method,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
	Timeout   int               `json:"timeout_ms"`
	Profile   string            `json:"profile,omitempty"`
}

type ProxyResponse struct {
	RequestID  string            `json:"request_id"`
	Success    bool              `json:"success"`
	StatusCode int               `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Error      string            `json:"error,omitempty"`
	BytesRead  int64             `json:"bytes_read"`
	BytesWrite int64             `json:"bytes_write"`
	LatencyMs  int64             `json:"latency_ms"`
}

type PendingRequest struct {
	RequestID  string
	ResponseCh chan *ProxyResponse
	CreatedAt  time.Time
}

// ─── Disconnect event ──────────────────────────────────────────────────────────

type DisconnectEvent struct {
	NodeID      string  `json:"node_id"`
	IP          string  `json:"ip"`
	Reason      string  `json:"reason"`
	Duration    string  `json:"duration"`
	DurationSec float64 `json:"duration_sec"`
	PingsSent   int64   `json:"pings_sent"`
	PongsRecv   int64   `json:"pongs_received"`
	At          time.Time `json:"at"`
	ProxyType   string  `json:"proxy_type,omitempty"`
	Country     string  `json:"country,omitempty"`
}

// ─── Cooldown ──────────────────────────────────────────────────────────────────

type NodeCooldown struct {
	ConnectTimes  []time.Time
	CooldownUntil time.Time
}

const (
	maxReconnectsWindow = 5 * time.Minute
	maxReconnects       = 10
	cooldownDuration    = 10 * time.Minute
)

// ─── DB ────────────────────────────────────────────────────────────────────────

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "/root/stability-data.db")
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ip_info (
			node_id TEXT PRIMARY KEY,
			device_id TEXT,
			device_model TEXT,
			ip TEXT,
			ip_fetch_ms INTEGER DEFAULT 0,
			info_fetch_ms INTEGER DEFAULT 0,
			country_code TEXT,
			city TEXT,
			isp TEXT,
			asn TEXT,
			proxy_type TEXT,
			ip_info_json TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS disconnects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT,
			ip TEXT,
			reason TEXT,
			duration_sec REAL,
			pings_sent INTEGER,
			pongs_recv INTEGER,
			proxy_type TEXT,
			country TEXT,
			disconnected_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_disconnects_node ON disconnects(node_id);
		CREATE INDEX IF NOT EXISTS idx_disconnects_at ON disconnects(disconnected_at);
		CREATE INDEX IF NOT EXISTS idx_ip_info_proxy ON ip_info(proxy_type);
		CREATE INDEX IF NOT EXISTS idx_ip_info_country ON ip_info(country_code);
	`)
	if err != nil {
		log.Fatal("Failed to create tables:", err)
	}
	log.Println("SQLite DB initialized at /root/stability-data.db")
}

func storeIPInfo(nodeID string, msg map[string]interface{}) {
	ipInfo, _ := msg["ip_info"].(map[string]interface{})
	if ipInfo == nil {
		return
	}

	deviceID, _ := msg["device_id"].(string)
	deviceModel, _ := msg["device_model"].(string)
	ip, _ := msg["ip"].(string)
	ipFetchMs, _ := msg["ip_fetch_ms"].(float64)
	infoFetchMs, _ := msg["info_fetch_ms"].(float64)

	countryCode, _ := ipInfo["country_code"].(string)
	city, _ := ipInfo["city_name"].(string)
	isp, _ := ipInfo["isp"].(string)
	asn, _ := ipInfo["asn"].(string)

	proxyType := "-"
	if proxy, ok := ipInfo["proxy"].(map[string]interface{}); ok {
		if pt, ok := proxy["proxy_type"].(string); ok {
			proxyType = pt
		}
	}

	ipInfoJSON, _ := json.Marshal(ipInfo)

	_, err := db.Exec(`
		INSERT INTO ip_info (node_id, device_id, device_model, ip, ip_fetch_ms, info_fetch_ms, 
			country_code, city, isp, asn, proxy_type, ip_info_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(node_id) DO UPDATE SET
			device_id=?, device_model=?, ip=?, ip_fetch_ms=?, info_fetch_ms=?,
			country_code=?, city=?, isp=?, asn=?, proxy_type=?, ip_info_json=?, updated_at=CURRENT_TIMESTAMP
	`, nodeID, deviceID, deviceModel, ip, int(ipFetchMs), int(infoFetchMs),
		countryCode, city, isp, asn, proxyType, string(ipInfoJSON),
		deviceID, deviceModel, ip, int(ipFetchMs), int(infoFetchMs),
		countryCode, city, isp, asn, proxyType, string(ipInfoJSON))

	if err != nil {
		log.Printf("[DB_ERROR] store ip_info: %v", err)
	}

	log.Printf("[IP_INFO] node=%s country=%s city=%s isp=%s proxy=%s ip_fetch=%dms info_fetch=%dms",
		nodeID, countryCode, city, isp, proxyType, int(ipFetchMs), int(infoFetchMs))
}

func storeDisconnect(event DisconnectEvent) {
	_, err := db.Exec(`
		INSERT INTO disconnects (node_id, ip, reason, duration_sec, pings_sent, pongs_recv, proxy_type, country)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, event.NodeID, event.IP, event.Reason, event.DurationSec, event.PingsSent, event.PongsRecv, event.ProxyType, event.Country)
	if err != nil {
		log.Printf("[DB_ERROR] store disconnect: %v", err)
	}
}

// ─── Helper functions ──────────────────────────────────────────────────────────

// isSDKVersionAllowed checks if SDK version is >= 1.0.62
func isSDKVersionAllowed(version string) bool {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) != 3 {
		return false
	}
	var major, minor, patch int
	fmt.Sscanf(parts[0], "%d", &major)
	fmt.Sscanf(parts[1], "%d", &minor)
	fmt.Sscanf(parts[2], "%d", &patch)

	if major > 1 {
		return true
	}
	if major < 1 {
		return false
	}
	if minor > 0 {
		return true
	}
	if minor < 0 {
		return false
	}
	return patch >= 62
}

// lookupIPGeo fetches geolocation for an IP address via ip-api.com
func lookupIPGeo(ip string) (*IPGeoInfo, error) {
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
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ─── TunnelManager ─────────────────────────────────────────────────────────────

type TunnelManager struct {
	hub     *Hub
	tunnels map[string]*Tunnel
	mu      sync.RWMutex
}

func NewTunnelManager(hub *Hub) *TunnelManager {
	tm := &TunnelManager{
		hub:     hub,
		tunnels: make(map[string]*Tunnel),
	}
	go tm.cleanupExpiredTunnels()
	return tm
}

// OpenTunnel opens a new tunnel through a node and waits for confirmation
func (tm *TunnelManager) OpenTunnel(nodeID, host, port string) (*Tunnel, error) {
	conn := tm.hub.GetConnectionByNodeID(nodeID)
	if conn == nil {
		return nil, fmt.Errorf("node not connected")
	}

	tunnelID := uuid.New().String()

	tunnel := &Tunnel{
		ID:        tunnelID,
		NodeID:    nodeID,
		Host:      host,
		Port:      port,
		Conn:      conn,
		DataCh:    make(chan []byte, 256),
		WriteCh:   make(chan []byte, 256),
		CloseCh:   make(chan struct{}),
		ReadyCh:   make(chan bool, 1),
		CreatedAt: time.Now(),
	}

	tm.mu.Lock()
	tm.tunnels[tunnelID] = tunnel
	tm.mu.Unlock()

	// Send tunnel_open to node
	msg, _ := json.Marshal(Message{
		Type: "tunnel_open",
		Data: &TunnelOpenRequest{
			TunnelID: tunnelID,
			Host:     host,
			Port:     port,
		},
	})

	select {
	case conn.SendCh <- msg:
	default:
		tm.mu.Lock()
		delete(tm.tunnels, tunnelID)
		tm.mu.Unlock()
		return nil, fmt.Errorf("send channel full")
	}

	log.Printf("[TUNNEL] Opening %s to %s:%s via node %s", tunnelID[:8], host, port, nodeID)

	// Wait for SDK to confirm
	select {
	case success := <-tunnel.ReadyCh:
		if !success {
			tm.mu.Lock()
			delete(tm.tunnels, tunnelID)
			tm.mu.Unlock()
			return nil, fmt.Errorf("tunnel open failed: %s", tunnel.readyErr)
		}
		log.Printf("[TUNNEL] %s confirmed ready by SDK", tunnelID[:8])
	case <-time.After(10 * time.Second):
		tm.mu.Lock()
		delete(tm.tunnels, tunnelID)
		tm.mu.Unlock()
		return nil, fmt.Errorf("tunnel open timeout")
	}

	// Start writer goroutine for this tunnel
	go tm.tunnelWriter(tunnel)

	return tunnel, nil
}

// encodeBinaryTunnelData creates a binary tunnel frame:
// [36 bytes tunnel_id][1 byte flags: 0x00=data, 0x01=EOF][N bytes payload]
func encodeBinaryTunnelData(tunnelID string, data []byte, eof bool) []byte {
	idBytes := []byte(tunnelID)
	if len(idBytes) < 36 {
		padded := make([]byte, 36)
		copy(padded, idBytes)
		idBytes = padded
	}
	flags := byte(0x00)
	if eof {
		flags = 0x01
	}
	frame := make([]byte, 36+1+len(data))
	copy(frame[0:36], idBytes[:36])
	frame[36] = flags
	if len(data) > 0 {
		copy(frame[37:], data)
	}
	return frame
}

// decodeBinaryTunnelData parses a binary tunnel frame
func decodeBinaryTunnelData(frame []byte) (tunnelID string, data []byte, eof bool, err error) {
	if len(frame) < 37 {
		return "", nil, false, fmt.Errorf("binary frame too short: %d bytes", len(frame))
	}
	tunnelID = strings.TrimRight(string(frame[0:36]), "\x00")
	eof = frame[36] == 0x01
	data = frame[37:]
	return
}

// tunnelWriter sends data from WriteCh to the node via binary WebSocket frames
func (tm *TunnelManager) tunnelWriter(tunnel *Tunnel) {
	for {
		select {
		case data := <-tunnel.WriteCh:
			frame := encodeBinaryTunnelData(tunnel.ID, data, false)
			select {
			case tunnel.Conn.BinaryCh <- frame:
			default:
				log.Printf("[TUNNEL] %s binary send channel full, dropping data", tunnel.ID[:8])
			}
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

// CloseTunnel closes a tunnel and sends EOF to node
func (tm *TunnelManager) CloseTunnel(tunnelID string) {
	tm.mu.Lock()
	tunnel, exists := tm.tunnels[tunnelID]
	if exists {
		delete(tm.tunnels, tunnelID)
	}
	tm.mu.Unlock()

	if tunnel != nil {
		tunnel.Close()

		// Send binary EOF to node
		eofFrame := encodeBinaryTunnelData(tunnelID, nil, true)
		select {
		case tunnel.Conn.BinaryCh <- eofFrame:
		default:
		}

		log.Printf("[TUNNEL] Closed %s", tunnelID[:8])
	}
}

// HandleTunnelResponse handles tunnel open response from node
func (tm *TunnelManager) HandleTunnelResponse(resp *TunnelOpenResponse) {
	tunnel := tm.GetTunnel(resp.TunnelID)
	if tunnel == nil {
		log.Printf("[TUNNEL] Response for unknown tunnel: %s", resp.TunnelID)
		return
	}

	tunnel.mu.Lock()
	tunnel.ready = resp.Success
	tunnel.readyErr = resp.Error
	tunnel.mu.Unlock()

	select {
	case tunnel.ReadyCh <- resp.Success:
	default:
	}

	if resp.Success {
		log.Printf("[TUNNEL] %s opened successfully", resp.TunnelID[:8])
	} else {
		log.Printf("[TUNNEL] %s failed: %s", resp.TunnelID[:8], resp.Error)
	}
}

// HandleTunnelData handles incoming data from node
func (tm *TunnelManager) HandleTunnelData(data *TunnelDataMessage) {
	tunnel := tm.GetTunnel(data.TunnelID)
	if tunnel == nil {
		log.Printf("[TUNNEL] Data for unknown tunnel: %s", data.TunnelID[:8])
		return
	}

	if data.EOF {
		log.Printf("[TUNNEL] %s received EOF from SDK", data.TunnelID[:8])
		tm.CloseTunnel(data.TunnelID)
		return
	}

	// Use RawData (binary) if available, otherwise decode base64 (legacy)
	var payload []byte
	if len(data.RawData) > 0 {
		payload = data.RawData
	} else {
		var err error
		payload, err = base64.StdEncoding.DecodeString(data.Data)
		if err != nil {
			log.Printf("[TUNNEL] Failed to decode data for %s: %v", data.TunnelID[:8], err)
			return
		}
	}

	select {
	case tunnel.DataCh <- payload:
	default:
		log.Printf("[TUNNEL] %s data channel full, dropping %d bytes", data.TunnelID[:8], len(payload))
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
			if now.Sub(tunnel.CreatedAt) > 10*time.Minute {
				delete(tm.tunnels, id)
				tunnel.Close()
				log.Printf("[TUNNEL] Cleaned up expired tunnel %s", id[:8])
			}
		}
		tm.mu.Unlock()
	}
}

// ─── ProxyManager ──────────────────────────────────────────────────────────────

type ProxyManager struct {
	hub         *Hub
	pendingMu   sync.RWMutex
	pendingReqs map[string]*PendingRequest
}

func NewProxyManager(hub *Hub) *ProxyManager {
	pm := &ProxyManager{
		hub:         hub,
		pendingReqs: make(map[string]*PendingRequest),
	}
	go pm.cleanupExpiredRequests()
	return pm
}

// SendProxyRequest sends a proxy request to a node and waits for response
func (pm *ProxyManager) SendProxyRequest(ctx context.Context, nodeID string, req *ProxyRequest) (*ProxyResponse, error) {
	conn := pm.hub.GetConnectionByNodeID(nodeID)
	if conn == nil {
		return nil, fmt.Errorf("node not connected")
	}

	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}

	pending := &PendingRequest{
		RequestID:  req.RequestID,
		ResponseCh: make(chan *ProxyResponse, 1),
		CreatedAt:  time.Now(),
	}

	pm.pendingMu.Lock()
	pm.pendingReqs[req.RequestID] = pending
	pm.pendingMu.Unlock()

	defer func() {
		pm.pendingMu.Lock()
		delete(pm.pendingReqs, req.RequestID)
		pm.pendingMu.Unlock()
	}()

	msg, _ := json.Marshal(Message{
		Type: "proxy_request",
		Data: req,
	})

	select {
	case conn.SendCh <- msg:
	default:
		return nil, fmt.Errorf("send channel full")
	}

	log.Printf("[PROXY] Sent request %s to node %s: %s:%s", req.RequestID[:8], nodeID, req.Host, req.Port)

	timeout := time.Duration(req.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	select {
	case resp := <-pending.ResponseCh:
		return resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("proxy request timeout")
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
		log.Printf("[PROXY] Response for unknown request: %s", resp.RequestID)
		return
	}

	select {
	case pending.ResponseCh <- resp:
		log.Printf("[PROXY] Delivered response %s (success=%v)", resp.RequestID[:8], resp.Success)
	default:
		log.Printf("[PROXY] Response channel full for %s", resp.RequestID[:8])
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
			}
		}
		pm.pendingMu.Unlock()
	}
}

// ─── Hub ───────────────────────────────────────────────────────────────────────

type Hub struct {
	mu             sync.RWMutex
	connections    map[string]*Connection
	disconnects    []DisconnectEvent
	totalConns     int64
	totalDiscons   int64
	cooldowns      map[string]*NodeCooldown
	totalCooldowns int64
	proxyManager   *ProxyManager
	tunnelManager  *TunnelManager
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]*Connection),
		disconnects: make([]DisconnectEvent, 0, 10000),
		cooldowns:   make(map[string]*NodeCooldown),
	}
}

func (h *Hub) CheckCooldown(nodeID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	cd, exists := h.cooldowns[nodeID]
	if !exists {
		cd = &NodeCooldown{}
		h.cooldowns[nodeID] = cd
	}

	if now.Before(cd.CooldownUntil) {
		return false
	}

	cutoff := now.Add(-maxReconnectsWindow)
	fresh := cd.ConnectTimes[:0]
	for _, t := range cd.ConnectTimes {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	cd.ConnectTimes = append(fresh, now)

	if len(cd.ConnectTimes) > maxReconnects {
		cd.CooldownUntil = now.Add(cooldownDuration)
		cd.ConnectTimes = nil
		h.totalCooldowns++
		log.Printf("[COOLDOWN] node=%s triggered cooldown (%d reconnects in %v), blocked until %s",
			nodeID, maxReconnects, maxReconnectsWindow, cd.CooldownUntil.Format("15:04:05"))
		return false
	}

	return true
}

func (h *Hub) Add(nodeID string, conn *Connection) {
	h.mu.Lock()
	h.connections[nodeID] = conn
	h.totalConns++
	h.mu.Unlock()
	log.Printf("[CONNECT] node=%s ip=%s model=%s sdk=%s total=%d", nodeID, conn.IP, conn.DeviceModel, conn.SDKVersion, h.Count())
}

func (h *Hub) Remove(nodeID, reason string) {
	h.mu.Lock()
	conn, ok := h.connections[nodeID]
	if ok {
		dur := time.Since(conn.ConnectedAt)
		proxyType := ""
		country := conn.Country
		// Look up proxy type from DB
		row := db.QueryRow("SELECT proxy_type FROM ip_info WHERE node_id = ?", nodeID)
		row.Scan(&proxyType)
		if country == "" {
			row2 := db.QueryRow("SELECT country_code FROM ip_info WHERE node_id = ?", nodeID)
			row2.Scan(&country)
		}
		event := DisconnectEvent{
			NodeID:      nodeID,
			IP:          conn.IP,
			Reason:      reason,
			Duration:    dur.String(),
			DurationSec: dur.Seconds(),
			PingsSent:   conn.PingsSent,
			PongsRecv:   conn.PongsRecv,
			At:          time.Now(),
			ProxyType:   proxyType,
			Country:     country,
		}
		h.disconnects = append(h.disconnects, event)
		if len(h.disconnects) > 1000 {
			h.disconnects = h.disconnects[len(h.disconnects)-500:]
		}
		delete(h.connections, nodeID)
		h.totalDiscons++
		h.mu.Unlock()
		go storeDisconnect(event)
		log.Printf("[DISCONNECT] node=%s reason=%q duration=%s pings=%d/%d total=%d",
			nodeID, reason, dur.Round(time.Second), conn.PongsRecv, conn.PingsSent, h.Count())
	} else {
		h.mu.Unlock()
	}
}

func (h *Hub) UpdatePong(nodeID string) {
	h.mu.Lock()
	if conn, ok := h.connections[nodeID]; ok {
		conn.LastPong = time.Now()
		conn.PongsRecv++
	}
	h.mu.Unlock()
}

func (h *Hub) SetIPInfo(nodeID string, msg map[string]interface{}) {
	h.mu.Lock()
	if conn, ok := h.connections[nodeID]; ok {
		conn.HasIPInfo = true
	}
	h.mu.Unlock()
	go storeIPInfo(nodeID, msg)
}

func (h *Hub) IncrPing(nodeID string) {
	h.mu.Lock()
	if conn, ok := h.connections[nodeID]; ok {
		conn.PingsSent++
	}
	h.mu.Unlock()
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// GetConnectionByNodeID returns a connection by node ID
func (h *Hub) GetConnectionByNodeID(nodeID string) *Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connections[nodeID]
}

// GetAllConnections returns a snapshot of all connections
func (h *Hub) GetAllConnections() []*Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns := make([]*Connection, 0, len(h.connections))
	for _, c := range h.connections {
		conns = append(conns, c)
	}
	return conns
}

// GetAlternativeNode finds another connected node, optionally matching country.
// Excludes nodes in the skip set.
func (h *Hub) GetAlternativeNode(country string, skip map[string]bool) *Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	// First pass: match country if specified
	if country != "" {
		for _, c := range h.connections {
			if skip[c.NodeID] { continue }
			if c.Country == country && c.SendCh != nil {
				return c
			}
		}
	}
	
	// Second pass: any node
	for _, c := range h.connections {
		if skip[c.NodeID] { continue }
		if c.SendCh != nil {
			return c
		}
	}
	return nil
}

func (h *Hub) Stats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var totalDur float64
	var minDur, maxDur float64
	minDur = 999999
	byModel := make(map[string]int)
	byCountry := make(map[string]int)
	registered := 0

	for _, conn := range h.connections {
		dur := time.Since(conn.ConnectedAt).Seconds()
		totalDur += dur
		if dur < minDur {
			minDur = dur
		}
		if dur > maxDur {
			maxDur = dur
		}
		model := conn.DeviceModel
		if model == "" {
			model = "unknown"
		}
		byModel[model]++
		cc := conn.Country
		if cc == "" {
			cc = "unknown"
		}
		byCountry[cc]++
		if conn.Registered {
			registered++
		}
	}

	avgDur := float64(0)
	if len(h.connections) > 0 {
		avgDur = totalDur / float64(len(h.connections))
	}

	reasonCounts := make(map[string]int)
	proxyTypeDiscons := make(map[string]int)
	countryDiscons := make(map[string]int)
	var recentDiscons []DisconnectEvent
	cutoff := time.Now().Add(-1 * time.Hour)
	for _, d := range h.disconnects {
		reasonCounts[d.Reason]++
		pt := d.ProxyType
		if pt == "" {
			pt = "unknown"
		}
		proxyTypeDiscons[pt]++
		cc := d.Country
		if cc == "" {
			cc = "unknown"
		}
		countryDiscons[cc]++
		if d.At.After(cutoff) {
			recentDiscons = append(recentDiscons, d)
		}
	}

	return map[string]interface{}{
		"connected":              len(h.connections),
		"registered":             registered,
		"total_connections":      h.totalConns,
		"total_disconnects":      h.totalDiscons,
		"avg_duration_sec":       int(avgDur),
		"min_duration_sec":       int(minDur),
		"max_duration_sec":       int(maxDur),
		"by_model":               byModel,
		"by_country":             byCountry,
		"total_cooldowns":        h.totalCooldowns,
		"active_cooldowns": func() int {
			count := 0
			now := time.Now()
			for _, cd := range h.cooldowns {
				if now.Before(cd.CooldownUntil) {
					count++
				}
			}
			return count
		}(),
		"disconnect_reasons":      reasonCounts,
		"disconnects_by_proxy":    proxyTypeDiscons,
		"disconnects_by_country":  countryDiscons,
		"recent_disconnects_1h":   len(recentDiscons),
		"last_10_disconnects": func() []DisconnectEvent {
			if len(h.disconnects) <= 10 {
				return h.disconnects
			}
			return h.disconnects[len(h.disconnects)-10:]
		}(),
	}
}

// ─── WebSocket handler ─────────────────────────────────────────────────────────

var hub = NewHub()

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] upgrade: %v", err)
		return
	}

	ip := extractClientIP(r)

	conn.SetReadLimit(maxMessageSize)

	// Wait for hello message with node ID
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Printf("[ERROR] no hello from %s: %v", ip, err)
		conn.Close()
		return
	}

	var hello struct {
		Type        string `json:"type"`
		NodeID      string `json:"node_id"`
		DeviceModel string `json:"device_model"`
		SDKVersion  string `json:"sdk_version"`
	}
	if err := json.Unmarshal(msg, &hello); err != nil || hello.NodeID == "" {
		log.Printf("[ERROR] bad hello from %s: %v", ip, err)
		conn.Close()
		return
	}

	// Check cooldown before accepting
	if !hub.CheckCooldown(hello.NodeID) {
		reject, _ := json.Marshal(map[string]interface{}{
			"type":            "cooldown",
			"reason":          "too many reconnects",
			"retry_after_sec": int(cooldownDuration.Seconds()),
		})
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		conn.WriteMessage(websocket.TextMessage, reject)
		conn.Close()
		return
	}

	// Write mutex to prevent concurrent writes to websocket
	var writeMu sync.Mutex
	safeWrite := func(messageType int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		return conn.WriteMessage(messageType, data)
	}

	sendCh := make(chan []byte, 256)
	binaryCh := make(chan []byte, 256)

	nodeConn := &Connection{
		NodeID:      hello.NodeID,
		DeviceModel: hello.DeviceModel,
		SDKVersion:  hello.SDKVersion,
		IP:          ip,
		ConnectedAt: time.Now(),
		LastPong:    time.Now(),
		Ws:          conn,
		SafeWrite:   safeWrite,
		SendCh:      sendCh,
		BinaryCh:    binaryCh,
	}
	hub.Add(hello.NodeID, nodeConn)

	// Send welcome
	welcome, _ := json.Marshal(map[string]interface{}{
		"type":          "welcome",
		"ping_interval": pingInterval.Seconds(),
		"server_time":   time.Now().UTC().Format(time.RFC3339),
	})
	safeWrite(websocket.TextMessage, welcome)

	// Set up pong handler
	conn.SetPongHandler(func(appData string) error {
		hub.UpdatePong(hello.NodeID)
		conn.SetReadDeadline(time.Now().Add(pingInterval + pongWait))
		return nil
	})

	// writePump goroutine — reads from SendCh (text) and BinaryCh (binary) and writes to WS
	done := make(chan struct{})
	go func() {
		for {
			select {
			case msg, ok := <-sendCh:
				if !ok {
					return
				}
				if err := safeWrite(websocket.TextMessage, msg); err != nil {
					log.Printf("[SEND_ERR] node=%s: %v", hello.NodeID, err)
					return
				}
			case binMsg, ok := <-binaryCh:
				if !ok {
					return
				}
				if err := safeWrite(websocket.BinaryMessage, binMsg); err != nil {
					log.Printf("[SEND_ERR_BIN] node=%s: %v", hello.NodeID, err)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Reader goroutine
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		conn.SetReadDeadline(time.Now().Add(pingInterval + pongWait))
		for {
			msgType, rawMsg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					hub.Remove(hello.NodeID, fmt.Sprintf("read_error: %v", err))
				} else {
					hub.Remove(hello.NodeID, fmt.Sprintf("close: %v", err))
				}
				return
			}

			// Handle binary frames (tunnel data)
			if msgType == websocket.BinaryMessage {
				if hub.tunnelManager != nil {
					tunnelID, data, eof, decErr := decodeBinaryTunnelData(rawMsg)
					if decErr == nil {
						hub.tunnelManager.HandleTunnelData(&TunnelDataMessage{
							TunnelID: tunnelID,
							Data:     "", // unused for binary
							RawData:  data,
							EOF:      eof,
						})
					}
				}
				continue
			}

			var m map[string]interface{}
			if json.Unmarshal(rawMsg, &m) != nil {
				continue
			}

			jsonMsgType, _ := m["type"].(string)
			switch jsonMsgType {
			case "keepalive":
				resp, _ := json.Marshal(map[string]string{"type": "keepalive_ack"})
				safeWrite(websocket.TextMessage, resp)

			case "ip_info":
				if _, ok := m["ip_info"].(map[string]interface{}); ok {
					hub.SetIPInfo(hello.NodeID, m)
				}

			case "register":
				handleRegistration(hello.NodeID, ip, nodeConn, m)

			case "heartbeat":
				resp, _ := json.Marshal(Message{
					Type: "heartbeat_ack",
					Data: map[string]interface{}{
						"node_id":   hello.NodeID,
						"timestamp": time.Now().UTC(),
						"status":    "active",
					},
				})
				safeWrite(websocket.TextMessage, resp)

			case "proxy_response":
				if hub.proxyManager != nil {
					dataBytes, _ := json.Marshal(m["data"])
					var resp ProxyResponse
					if json.Unmarshal(dataBytes, &resp) == nil {
						hub.proxyManager.HandleProxyResponse(&resp)
					}
				}

			case "tunnel_response":
				if hub.tunnelManager != nil {
					dataBytes, _ := json.Marshal(m["data"])
					var resp TunnelOpenResponse
					if json.Unmarshal(dataBytes, &resp) == nil {
						hub.tunnelManager.HandleTunnelResponse(&resp)
					}
				}

			case "tunnel_data":
				if hub.tunnelManager != nil {
					dataBytes, _ := json.Marshal(m["data"])
					var data TunnelDataMessage
					if json.Unmarshal(dataBytes, &data) == nil {
						hub.tunnelManager.HandleTunnelData(&data)
					}
				}

			default:
				log.Printf("[MSG] unknown type=%s from node=%s", msgType, hello.NodeID)
			}
		}
	}()

	// Ping loop
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hub.IncrPing(hello.NodeID)
			if err := safeWrite(websocket.PingMessage, nil); err != nil {
				hub.Remove(hello.NodeID, fmt.Sprintf("ping_write_error: %v", err))
				conn.Close()
				close(done)
				return
			}
		case <-readerDone:
			conn.Close()
			close(done)
			return
		}
	}
}

// handleRegistration processes a "register" message from the SDK
func handleRegistration(nodeID, clientIP string, conn *Connection, m map[string]interface{}) {
	dataBytes, _ := json.Marshal(m["data"])
	var regData RegistrationMessage
	if err := json.Unmarshal(dataBytes, &regData); err != nil {
		errMsg, _ := json.Marshal(Message{
			Type: "error",
			Data: map[string]interface{}{
				"error":     "Invalid registration data",
				"timestamp": time.Now().UTC(),
			},
		})
		conn.SafeWrite(websocket.TextMessage, errMsg)
		return
	}

	log.Printf("[REGISTER] node=%s device_id=%s device_type=%s sdk_version=%s",
		nodeID, regData.DeviceID, regData.DeviceType, regData.SDKVersion)

	// Validate SDK version
	if !isSDKVersionAllowed(regData.SDKVersion) {
		log.Printf("[REGISTER] Rejected old SDK %s from device %s", regData.SDKVersion, regData.DeviceID)
		errMsg, _ := json.Marshal(Message{
			Type: "error",
			Data: map[string]interface{}{
				"error":     "SDK version too old. Minimum required: 1.0.62",
				"timestamp": time.Now().UTC(),
			},
		})
		conn.SafeWrite(websocket.TextMessage, errMsg)
		return
	}

	if regData.ConnectionType == "" {
		regData.ConnectionType = "unknown"
	}
	regData.IPAddress = clientIP

	// Parse geo from SDK or fallback to ip-api.com
	if regData.Country != "" {
		// SDK provided geo data
		if regData.Loc != "" && regData.Latitude == 0 && regData.Longitude == 0 {
			parts := strings.SplitN(regData.Loc, ",", 2)
			if len(parts) == 2 {
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &regData.Latitude)
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &regData.Longitude)
			}
		}
		if regData.Org != "" && regData.ISP == "" {
			orgParts := strings.SplitN(regData.Org, " ", 2)
			if len(orgParts) >= 1 && strings.HasPrefix(orgParts[0], "AS") {
				fmt.Sscanf(orgParts[0], "AS%d", &regData.ASN)
			}
			if len(orgParts) >= 2 {
				regData.ISP = orgParts[1]
			}
		}
		log.Printf("[REGISTER] SDK geo: %s, %s (sdk=%s)", regData.Country, regData.City, regData.SDKVersion)
	} else {
		// Fallback to server-side geo lookup
		geo, err := lookupIPGeo(clientIP)
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
			log.Printf("[REGISTER] Server geo lookup: %s -> %s, %s", clientIP, regData.Country, regData.City)
		} else {
			log.Printf("[REGISTER] Geo lookup failed for %s: %v", clientIP, err)
		}
	}

	// Update connection with registration data
	hub.mu.Lock()
	conn.DeviceID = regData.DeviceID
	conn.Country = regData.Country
	conn.City = regData.City
	conn.ISP = regData.ISP
	conn.ASN = fmt.Sprintf("%d", regData.ASN)
	conn.SDKVersion = regData.SDKVersion
	conn.ConnectionType = regData.ConnectionType
	conn.DeviceType = regData.DeviceType
	conn.Registered = true
	hub.mu.Unlock()

	// Send registration_success
	resp, _ := json.Marshal(Message{
		Type: "registration_success",
		Data: map[string]interface{}{
			"node_id":   nodeID,
			"status":    "registered",
			"message":   "Node successfully registered",
			"timestamp": time.Now().UTC(),
		},
	})
	conn.SafeWrite(websocket.TextMessage, resp)

	log.Printf("[REGISTER] Success: node=%s device=%s ip=%s country=%s city=%s isp=%s",
		nodeID, regData.DeviceID, clientIP, regData.Country, regData.City, regData.ISP)
}

// ─── HTTP Handlers ─────────────────────────────────────────────────────────────

func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hub.Stats())
}

func handleIPInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	hub.mu.RLock()
	nodeIDs := make([]string, 0, len(hub.connections))
	for _, conn := range hub.connections {
		if conn.HasIPInfo {
			nodeIDs = append(nodeIDs, conn.NodeID)
		}
	}
	hub.mu.RUnlock()

	result := make([]map[string]interface{}, 0)
	rows, err := db.Query(`SELECT node_id, device_model, ip, country_code, city, isp, asn, 
		proxy_type, ip_fetch_ms, info_fetch_ms FROM ip_info`)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	defer rows.Close()

	connectedSet := make(map[string]bool, len(nodeIDs))
	for _, nid := range nodeIDs {
		connectedSet[nid] = true
	}

	for rows.Next() {
		var nodeID, deviceModel, ip, cc, city, isp, asn, proxyType string
		var ipFetchMs, infoFetchMs int
		rows.Scan(&nodeID, &deviceModel, &ip, &cc, &city, &isp, &asn, &proxyType, &ipFetchMs, &infoFetchMs)
		if connectedSet[nodeID] {
			result = append(result, map[string]interface{}{
				"node_id":       nodeID,
				"device_model":  deviceModel,
				"ip":            ip,
				"country_code":  cc,
				"city":          city,
				"isp":           isp,
				"asn":           asn,
				"proxy_type":    proxyType,
				"ip_fetch_ms":   ipFetchMs,
				"info_fetch_ms": infoFetchMs,
			})
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_with_ip_info": len(result),
		"nodes":              result,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK connected=%d\n", hub.Count())
}

// ─── API: Tunnel endpoints ─────────────────────────────────────────────────────

func handleAPITunnelOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		NodeID string `json:"node_id"`
		Host   string `json:"host"`
		Port   string `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if payload.NodeID == "" || payload.Host == "" || payload.Port == "" {
		http.Error(w, "Missing required fields: node_id, host, port", http.StatusBadRequest)
		return
	}

	sticky := r.URL.Query().Get("sticky") == "true"
	maxRetries := 3
	if sticky {
		maxRetries = 1
	}

	tried := map[string]bool{}
	nodeID := payload.NodeID
	var lastErr string

	origConn := hub.GetConnectionByNodeID(nodeID)
	country := ""
	if origConn != nil {
		country = origConn.Country
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		tried[nodeID] = true

		tunnel, err := hub.tunnelManager.OpenTunnel(nodeID, payload.Host, payload.Port)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":   true,
				"tunnel_id": tunnel.ID,
				"node_id":   nodeID,
				"attempts":  attempt + 1,
			})
			return
		}

		lastErr = err.Error()
		isNodeBusy := strings.Contains(lastErr, "node busy") || strings.Contains(lastErr, "pool full")
		if !isNodeBusy {
			break
		}

		alt := hub.GetAlternativeNode(country, tried)
		if alt == nil {
			break
		}
		log.Printf("[TUNNEL_RETRY] node=%s failed (%s), retrying on node=%s (attempt %d)", nodeID, lastErr, alt.NodeID, attempt+2)
		nodeID = alt.NodeID
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  false,
		"error":    lastErr,
		"attempts": len(tried),
	})
}

func handleAPITunnelData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		TunnelID string `json:"tunnel_id"`
		Data     string `json:"data"` // base64
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tunnel := hub.tunnelManager.GetTunnel(payload.TunnelID)
	if tunnel == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "tunnel not found",
		})
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		http.Error(w, "Invalid base64 data", http.StatusBadRequest)
		return
	}

	if err := tunnel.Write(decoded); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func handleAPITunnelClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		TunnelID string `json:"tunnel_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	hub.tunnelManager.CloseTunnel(payload.TunnelID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// ─── API: Proxy endpoint ───────────────────────────────────────────────────────

func handleAPIProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		NodeID    string            `json:"node_id"`
		Host      string            `json:"host"`
		Port      string            `json:"port"`
		Method    string            `json:"method,omitempty"`
		URL       string            `json:"url,omitempty"`
		Headers   map[string]string `json:"headers,omitempty"`
		Body      string            `json:"body,omitempty"`
		TimeoutMs int               `json:"timeout_ms"`
		Profile   string            `json:"profile,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if payload.NodeID == "" || payload.Host == "" || payload.Port == "" {
		http.Error(w, "Missing required fields: node_id, host, port", http.StatusBadRequest)
		return
	}

	timeout := payload.TimeoutMs
	if timeout <= 0 {
		timeout = 30000
	}

	req := &ProxyRequest{
		Host:    payload.Host,
		Port:    payload.Port,
		Method:  payload.Method,
		URL:     payload.URL,
		Headers: payload.Headers,
		Body:    payload.Body,
		Timeout: timeout,
		Profile: payload.Profile,
	}

	sticky := r.URL.Query().Get("sticky") == "true"
	maxRetries := 3
	if sticky {
		maxRetries = 1
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeout+5000)*time.Millisecond)
	defer cancel()

	tried := map[string]bool{}
	nodeID := payload.NodeID
	var lastErr string
	
	// Get country of original node for targeting
	origConn := hub.GetConnectionByNodeID(nodeID)
	country := ""
	if origConn != nil {
		country = origConn.Country
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		tried[nodeID] = true
		
		resp, err := hub.proxyManager.SendProxyRequest(ctx, nodeID, req)
		if err == nil && resp.Success {
			// Success
			if attempt > 0 {
				resp.Headers = map[string]string{"X-Retried-Node": nodeID, "X-Attempts": fmt.Sprintf("%d", attempt+1)}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Check if retryable (node busy / timeout)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else if resp != nil {
			errMsg = resp.Error
		}
		lastErr = errMsg
		
		isNodeBusy := strings.Contains(errMsg, "node busy") || strings.Contains(errMsg, "pool full")
		
		if !isNodeBusy {
			// Non-retryable error — return immediately
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			if resp != nil {
				json.NewEncoder(w).Encode(resp)
			} else {
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": errMsg})
			}
			return
		}

		// Find alternative node
		alt := hub.GetAlternativeNode(country, tried)
		if alt == nil {
			break // No more nodes to try
		}
		
		log.Printf("[PROXY_RETRY] node=%s failed (%s), retrying on node=%s (attempt %d)", nodeID, errMsg, alt.NodeID, attempt+2)
		nodeID = alt.NodeID
	}

	// All retries exhausted
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  false,
		"error":    lastErr,
		"attempts": len(tried),
	})
}

// ─── API: Nodes endpoints ──────────────────────────────────────────────────────

func handleAPINodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conns := hub.GetAllConnections()
	nodes := make([]map[string]interface{}, 0, len(conns))
	for _, c := range conns {
		nodes = append(nodes, map[string]interface{}{
			"node_id":         c.NodeID,
			"device_id":       c.DeviceID,
			"device_model":    c.DeviceModel,
			"ip":              c.IP,
			"country":         c.Country,
			"city":            c.City,
			"isp":             c.ISP,
			"asn":             c.ASN,
			"sdk_version":     c.SDKVersion,
			"connection_type": c.ConnectionType,
			"device_type":     c.DeviceType,
			"connected_at":    c.ConnectedAt,
			"last_pong":       c.LastPong,
			"registered":      c.Registered,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total": len(nodes),
		"nodes": nodes,
	})
}

func handleAPINodeByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract node ID from path: /api/nodes/{id}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[3] == "" {
		http.Error(w, "Missing node_id", http.StatusBadRequest)
		return
	}
	nodeID := parts[3]

	conn := hub.GetConnectionByNodeID(nodeID)
	if conn == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "node not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"node_id":         conn.NodeID,
		"device_id":       conn.DeviceID,
		"device_model":    conn.DeviceModel,
		"ip":              conn.IP,
		"country":         conn.Country,
		"city":            conn.City,
		"isp":             conn.ISP,
		"asn":             conn.ASN,
		"sdk_version":     conn.SDKVersion,
		"connection_type": conn.ConnectionType,
		"device_type":     conn.DeviceType,
		"connected_at":    conn.ConnectedAt,
		"last_pong":       conn.LastPong,
		"registered":      conn.Registered,
		"pings_sent":      conn.PingsSent,
		"pongs_received":  conn.PongsRecv,
	})
}

// ─── API: Tunnel WebSocket (for proxy-gateway bidirectional relay) ─────────────

func handleAPITunnelWS(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	host := r.URL.Query().Get("host")
	port := r.URL.Query().Get("port")

	if nodeID == "" || host == "" || port == "" {
		http.Error(w, "Missing required parameters: node_id, host, port", http.StatusBadRequest)
		return
	}

	wsUpgrader := websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  65536,
		WriteBufferSize: 65536,
	}

	wsConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[TUNNEL_WS] Upgrade failed: %v", err)
		return
	}
	defer wsConn.Close()

	tunnel, err := hub.tunnelManager.OpenTunnel(nodeID, host, port)
	if err != nil {
		log.Printf("[TUNNEL_WS] Failed to open tunnel: %v", err)
		wsConn.WriteJSON(map[string]interface{}{"error": err.Error()})
		return
	}
	defer hub.tunnelManager.CloseTunnel(tunnel.ID)

	log.Printf("[TUNNEL_WS] Tunnel %s established, starting relay", tunnel.ID[:8])

	var wg sync.WaitGroup
	wg.Add(2)

	// Proxy -> Node
	go func() {
		defer wg.Done()
		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
				if err := tunnel.Write(data); err != nil {
					return
				}
			}
		}
	}()

	// Node -> Proxy
	go func() {
		defer wg.Done()
		for {
			data, err := tunnel.ReadWithTimeout(30 * time.Second)
			if err != nil {
				wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := wsConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return
			}
		}
	}()

	wg.Wait()
	log.Printf("[TUNNEL_WS] Tunnel %s closed", tunnel.ID[:8])
}

// ─── API: Tunnel Standby (pre-opened tunnel pools) ────────────────────────────

func handleAPITunnelStandby(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		http.Error(w, "Missing node_id", http.StatusBadRequest)
		return
	}

	// Verify node is connected BEFORE upgrading
	conn := hub.GetConnectionByNodeID(nodeID)
	if conn == nil {
		http.Error(w, "Node not connected", http.StatusBadGateway)
		return
	}

	wsUpgrader := websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  65536,
		WriteBufferSize: 65536,
	}

	wsConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[TUNNEL_STANDBY] WS upgrade failed: %v", err)
		return
	}
	defer wsConn.Close()

	// Phase 1: Send standby_ready
	wsConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := wsConn.WriteMessage(websocket.TextMessage, []byte("standby_ready")); err != nil {
		return
	}
	wsConn.SetWriteDeadline(time.Time{})

	log.Printf("[TUNNEL_STANDBY] Ready for node %s, waiting for target...", nodeID)

	// Phase 2: Wait for target host:port
	wsConn.SetReadDeadline(time.Now().Add(3 * time.Minute))
	_, msg, err := wsConn.ReadMessage()
	if err != nil {
		log.Printf("[TUNNEL_STANDBY] Closed for node %s: %v", nodeID, err)
		return
	}
	wsConn.SetReadDeadline(time.Time{})

	target := string(msg)
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		host = target
		port = "80"
	}

	log.Printf("[TUNNEL_STANDBY] Activating: node=%s target=%s:%s", nodeID, host, port)

	// Re-verify node still connected
	conn = hub.GetConnectionByNodeID(nodeID)
	if conn == nil {
		wsConn.WriteMessage(websocket.TextMessage, []byte("error:node_disconnected"))
		return
	}

	tunnel, err := hub.tunnelManager.OpenTunnel(nodeID, host, port)
	if err != nil {
		log.Printf("[TUNNEL_STANDBY] Activation failed: %v", err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("error:"+err.Error()))
		return
	}
	defer hub.tunnelManager.CloseTunnel(tunnel.ID)

	wsConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := wsConn.WriteMessage(websocket.TextMessage, []byte("tunnel_active")); err != nil {
		return
	}
	wsConn.SetWriteDeadline(time.Time{})

	log.Printf("[TUNNEL_STANDBY] Tunnel %s activated: node=%s target=%s:%s", tunnel.ID[:8], nodeID, host, port)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
				if err := tunnel.Write(data); err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			data, err := tunnel.ReadWithTimeout(30 * time.Second)
			if err != nil {
				wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := wsConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return
			}
		}
	}()

	wg.Wait()
	log.Printf("[TUNNEL_STANDBY] Tunnel %s closed", tunnel.ID[:8])
}

// ─── Main ──────────────────────────────────────────────────────────────────────

// ─── HTTP CONNECT Proxy (for curl -x) ──────────────────────────────────────

func startHTTPProxy(proxyPort string) {
	listener, err := net.Listen("tcp", ":"+proxyPort)
	if err != nil {
		log.Printf("HTTP Proxy listen error: %v", err)
		return
	}
	port := listener.Addr().(*net.TCPAddr).Port
	log.Printf("HTTP Proxy listening on :%d", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[HTTP_PROXY] Accept error: %v", err)
			continue
		}
		go handleProxyConn(conn)
	}
}

func handleProxyConn(clientConn net.Conn) {
	defer clientConn.Close()

	clientConn.SetDeadline(time.Now().Add(60 * time.Second))

	reader := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}

	if req.Method == http.MethodConnect {
		handleCONNECTRaw(clientConn, req)
		return
	}

	// Plain HTTP proxy
	host := req.Host
	if host == "" {
		clientConn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}
	port := "80"
	if h, p, splitErr := net.SplitHostPort(host); splitErr == nil {
		host = h
		port = p
	}

	alt := hub.GetAlternativeNode("", map[string]bool{})
	if alt == nil {
		clientConn.Write([]byte("HTTP/1.1 503 No Nodes Available\r\n\r\n"))
		return
	}

	log.Printf("[HTTP_PROXY] %s %s via node=%s", req.Method, req.URL.String(), alt.NodeID)

	reqBody := ""
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(io.LimitReader(req.Body, 1048576))
		if len(bodyBytes) > 0 {
			reqBody = base64.StdEncoding.EncodeToString(bodyBytes)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, proxyErr := hub.proxyManager.SendProxyRequest(ctx, alt.NodeID, &ProxyRequest{
		Host:    host,
		Port:    port,
		Method:  req.Method,
		URL:     req.URL.String(),
		Body:    reqBody,
		Timeout: 30000,
	})
	if proxyErr != nil {
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}

	if !resp.Success {
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}

	body := []byte{}
	if resp.Body != "" {
		body, _ = base64.StdEncoding.DecodeString(resp.Body)
	}
	hdr := fmt.Sprintf("HTTP/1.1 %d OK\r\nContent-Length: %d\r\n\r\n", resp.StatusCode, len(body))
	clientConn.Write([]byte(hdr))
	clientConn.Write(body)
}

func handleCONNECTRaw(clientConn net.Conn, req *http.Request) {
	host, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
		port = "443"
	}

	// For now, only use nodes with stability SDK (tunnel support)
	// TODO: track SDK version per node and filter properly
	targetNodeID := "d4e22419ee659730" // emulator with stability SDK
	conn := hub.GetConnectionByNodeID(targetNodeID)
	if conn == nil {
		log.Printf("[HTTP_PROXY] Emulator node %s not found, checking all nodes...", targetNodeID)
		// Try partial match
		for _, c := range hub.GetAllConnections() {
			if strings.Contains(c.NodeID, "d4e22419") {
				conn = c
				targetNodeID = c.NodeID
				break
			}
		}
	}
	if conn == nil {
		clientConn.Write([]byte("HTTP/1.1 503 Emulator node not connected\r\n\r\n"))
		log.Printf("[HTTP_PROXY] No emulator node available")
		return
	}

	log.Printf("[HTTP_PROXY] CONNECT %s:%s via node=%s", host, port, targetNodeID)

	tunnel, err := hub.tunnelManager.OpenTunnel(targetNodeID, host, port)
	if err != nil {
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		log.Printf("[HTTP_PROXY] Tunnel open failed: %v", err)
		return
	}

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	log.Printf("[HTTP_PROXY] Tunnel %s established for %s:%s via %s", tunnel.ID[:8], host, port, conn.NodeID)

	// Remove deadline for tunnel relay
	clientConn.SetDeadline(time.Time{})

	var wg sync.WaitGroup
	wg.Add(2)

	// Client → Node
	go func() {
		defer wg.Done()
		buf := make([]byte, 32768)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				if writeErr := tunnel.Write(buf[:n]); writeErr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Node → Client
	go func() {
		defer wg.Done()
		for {
			data, err := tunnel.ReadWithTimeout(30 * time.Second)
			if err != nil {
				break
			}
			if _, writeErr := clientConn.Write(data); writeErr != nil {
				break
			}
		}
	}()

	wg.Wait()
	hub.tunnelManager.CloseTunnel(tunnel.ID)
	log.Printf("[HTTP_PROXY] Tunnel %s closed for %s:%s", tunnel.ID[:8], host, port)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	initDB()
	defer db.Close()

	// Initialize tunnel and proxy managers
	hub.tunnelManager = NewTunnelManager(hub)
	hub.proxyManager = NewProxyManager(hub)

	// Memory monitor - logs every 30s, dumps heap when crossing thresholds
	go func() {
		var lastAlloc uint64
		heapDumped := false
		for {
			time.Sleep(30 * time.Second)
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			conns := hub.Count()
			goroutines := runtime.NumGoroutine()
			allocMB := m.Alloc / 1024 / 1024
			sysMB := m.Sys / 1024 / 1024
			stackMB := m.StackInuse / 1024 / 1024
			heapMB := m.HeapInuse / 1024 / 1024
			gcPause := m.PauseNs[(m.NumGC+255)%256] / 1000000

			delta := ""
			if lastAlloc > 0 {
				diff := int64(m.Alloc) - int64(lastAlloc)
				delta = fmt.Sprintf(" delta=%+dMB", diff/1024/1024)
			}
			lastAlloc = m.Alloc

			log.Printf("[MEMORY] conns=%d goroutines=%d alloc=%dMB sys=%dMB heap=%dMB stack=%dMB gc_pause=%dms gc_count=%d%s",
				conns, goroutines, allocMB, sysMB, heapMB, stackMB, gcPause, m.NumGC, delta)

			if sysMB > 1200 && !heapDumped {
				heapDumped = true
				f, err := os.Create(fmt.Sprintf("/root/heap-%d.pprof", time.Now().Unix()))
				if err == nil {
					pprof.WriteHeapProfile(f)
					f.Close()
					log.Printf("[MEMORY] ⚠️ Heap profile dumped at %dMB sys", sysMB)
				}
				gf, err := os.Create(fmt.Sprintf("/root/goroutines-%d.txt", time.Now().Unix()))
				if err == nil {
					pprof.Lookup("goroutine").WriteTo(gf, 1)
					gf.Close()
					log.Printf("[MEMORY] ⚠️ Goroutine profile dumped")
				}
			}
		}
	}()

	// WebSocket + existing endpoints
	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/stats", handleStats)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/ip-info", handleIPInfo)

	// API: Tunnel endpoints
	http.HandleFunc("/api/tunnel/open", handleAPITunnelOpen)
	http.HandleFunc("/api/tunnel/data", handleAPITunnelData)
	http.HandleFunc("/api/tunnel/close", handleAPITunnelClose)
	http.HandleFunc("/api/tunnel/ws", handleAPITunnelWS)
	http.HandleFunc("/api/tunnel/standby", handleAPITunnelStandby)

	// API: Proxy endpoint
	http.HandleFunc("/api/proxy", handleAPIProxy)

	// API: Nodes endpoints
	http.HandleFunc("/api/nodes", handleAPINodes)
	http.HandleFunc("/api/nodes/", handleAPINodeByID)

	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	// Start HTTP CONNECT proxy on port 8880
	go startHTTPProxy("8880")

	if tlsCert != "" && tlsKey != "" {
		log.Printf("WS Stability+Hub Server starting on :%s (TLS)", port)
		log.Printf("  WebSocket: wss://0.0.0.0:%s/ws", port)
		log.Printf("  Stats:     https://0.0.0.0:%s/stats", port)
		log.Printf("  API:       https://0.0.0.0:%s/api/...", port)
		log.Printf("  Ping interval: %v, Pong timeout: %v", pingInterval, pongWait)
		if err := http.ListenAndServeTLS(":"+port, tlsCert, tlsKey, nil); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("WS Stability+Hub Server starting on :%s", port)
		log.Printf("  WebSocket: ws://0.0.0.0:%s/ws", port)
		log.Printf("  Stats:     http://0.0.0.0:%s/stats", port)
		log.Printf("  API:       http://0.0.0.0:%s/api/...", port)
		log.Printf("  Ping interval: %v, Pong timeout: %v", pingInterval, pongWait)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	}
}
