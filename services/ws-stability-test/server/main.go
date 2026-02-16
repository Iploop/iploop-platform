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
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "modernc.org/sqlite"
	_ "net/http/pprof"
)

// ─── Node Quality Scoring ──────────────────────────────────────────────────────

type NodeScore struct {
	SuccessCount int64     `json:"success_count"`
	FailCount    int64     `json:"fail_count"`
	TotalLatency int64     `json:"total_latency_ms"`
	LastUsed     time.Time `json:"last_used"`
	Quarantined  time.Time `json:"quarantined_until"`
}

func (ns *NodeScore) Total() int64 {
	return ns.SuccessCount + ns.FailCount
}

func (ns *NodeScore) SuccessRate() float64 {
	total := ns.Total()
	if total == 0 {
		return 0.5 // neutral for unknown nodes
	}
	return float64(ns.SuccessCount) / float64(total)
}

func (ns *NodeScore) AvgLatency() float64 {
	if ns.SuccessCount == 0 {
		return 9999
	}
	return float64(ns.TotalLatency) / float64(ns.SuccessCount)
}

func (ns *NodeScore) IsQuarantined() bool {
	return time.Now().Before(ns.Quarantined)
}

// ─── Partner Proxy Pool ────────────────────────────────────────────────────────

var partnerServers = []string{
	"23.29.117.114", "23.29.120.170", "23.29.126.26", "23.92.69.210", "23.92.70.250",
	"23.111.143.170", "23.111.152.174", "23.111.153.30", "23.111.153.170", "23.111.153.230",
	"23.111.154.2", "23.111.157.198", "23.111.159.206", "23.111.159.226", "23.111.161.134",
	"23.111.181.130", "23.227.174.122", "23.227.177.34", "23.227.182.10", "23.227.182.234",
	"23.227.186.202", "23.227.191.18", "37.72.172.226", "46.21.152.114", "66.165.226.242",
	"66.165.231.202", "66.165.236.2", "66.165.244.6", "66.206.17.98", "66.206.19.90",
	"66.206.28.42", "68.233.238.194", "79.127.221.23", "79.127.223.2", "79.127.223.3",
	"79.127.223.4", "79.127.223.5", "79.127.223.6", "79.127.223.7", "79.127.223.8",
	"79.127.223.9", "79.127.231.50", "79.127.232.196", "79.127.232.224", "79.127.250.3",
	"79.127.250.4", "79.127.250.5", "84.17.41.118", "84.17.41.120", "89.187.170.185",
	"89.187.170.186", "89.187.175.81", "89.187.175.119", "89.187.175.120", "89.187.175.121",
	"89.187.175.122", "89.187.175.123", "89.187.175.133", "89.187.185.93", "89.187.185.123",
	"89.187.185.194", "89.222.109.19", "89.222.109.153", "89.222.120.88", "89.222.120.106",
	"89.222.120.111", "89.222.120.123", "94.72.163.78", "94.72.166.170", "95.173.192.104",
	"95.173.192.105", "95.173.216.227", "104.156.50.114", "104.156.55.218", "107.155.67.26",
	"107.155.97.142", "107.155.100.194", "107.155.127.10", "121.127.40.55", "121.127.40.56",
	"121.127.44.58", "143.244.60.22", "143.244.60.79", "143.244.60.104", "143.244.60.28",
	"152.233.22.58", "152.233.22.59", "152.233.22.66", "152.233.23.65", "152.233.23.66",
	"152.233.23.67", "152.233.23.68", "152.233.23.69", "152.233.23.70", "152.233.23.71",
	"152.233.23.72", "152.233.23.73", "156.146.38.229", "156.146.43.28", "162.212.57.238",
	"162.213.193.82", "178.249.210.13", "178.249.210.26", "178.249.210.27", "185.156.47.83",
	"185.156.47.84", "185.156.47.85", "185.156.47.86", "185.156.47.87", "185.156.47.88",
	"185.156.47.89", "190.102.105.62", "199.167.144.34", "199.231.162.90", "209.133.213.162",
	"209.133.213.222", "209.133.221.6", "209.133.221.214", "212.102.58.203", "212.102.58.205",
	"212.102.58.210", "212.102.58.213", "212.102.58.214",
}

const (
	partnerPassword     = "ct43gf7xz"
	partnerPort         = "60003"
	partnerPeersPerSvr  = 2500
	partnerOwnPoolPct   = 5  // 5% chance to use our own nodes
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
	ReadBufferSize:  65536,
	WriteBufferSize: 65536,
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
	ProxyType      string    `json:"proxy_type"`
	OS             string    `json:"os"`
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
	BytesSentToNode     int64
	BytesRecvFromNode   int64
}

func (t *Tunnel) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	sent := atomic.LoadInt64(&t.BytesSentToNode)
	recv := atomic.LoadInt64(&t.BytesRecvFromNode)
	if sent > 0 || recv > 0 {
		log.Printf("[TUNNEL-BYTES] %s node=%s host=%s sent=%d recv=%d duration=%v",
			t.ID[:8], t.NodeID[:8], t.Host, sent, recv, time.Since(t.CreatedAt))
	}
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
		atomic.AddInt64(&t.BytesSentToNode, int64(len(data)))
		return nil
	case <-t.CloseCh:
		return fmt.Errorf("tunnel closed")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("tunnel write timeout")
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
	db, err = sql.Open("sqlite", "/root/iploop-node-server.db")
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

		CREATE TABLE IF NOT EXISTS requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT,
			node_id TEXT,
			target TEXT,
			success INTEGER,
			latency_ms INTEGER,
			error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_requests_at ON requests(created_at);
		CREATE INDEX IF NOT EXISTS idx_requests_success ON requests(success);

		CREATE TABLE IF NOT EXISTS snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			active_nodes INTEGER,
			unique_nodes_1h INTEGER,
			total_countries INTEGER,
			top_country TEXT,
			top_country_nodes INTEGER,
			tunnel_requests_1h INTEGER,
			tunnel_success_1h INTEGER,
			proxy_requests_1h INTEGER,
			proxy_success_1h INTEGER,
			sdk_versions TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_snapshots_at ON snapshots(created_at);

		CREATE TABLE IF NOT EXISTS bandwidth (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			customer TEXT NOT NULL,
			route_type TEXT NOT NULL,
			target TEXT,
			bytes_in INTEGER DEFAULT 0,
			bytes_out INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_bandwidth_customer ON bandwidth(customer);
		CREATE INDEX IF NOT EXISTS idx_bandwidth_at ON bandwidth(created_at);
		CREATE INDEX IF NOT EXISTS idx_bandwidth_customer_at ON bandwidth(customer, created_at);
	`)
	if err != nil {
		log.Fatal("Failed to create tables:", err)
	}
	log.Println("SQLite DB initialized at /root/iploop-node-server.db")
}

func storeBandwidth(customer, routeType, target string, bytesIn, bytesOut int64) {
	if customer == "" {
		customer = "unknown"
	}
	go func() {
		_, err := db.Exec(`INSERT INTO bandwidth (customer, route_type, target, bytes_in, bytes_out, created_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			customer, routeType, target, bytesIn, bytesOut)
		if err != nil {
			log.Printf("[DB_ERROR] store bandwidth: %v", err)
		}
	}()
}

func storeRequest(reqType, nodeID, target string, success bool, latencyMs int, errMsg string) {
	successInt := 0
	if success {
		successInt = 1
	}
	go func() {
		_, err := db.Exec(`INSERT INTO requests (type, node_id, target, success, latency_ms, error, created_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			reqType, nodeID, target, successInt, latencyMs, errMsg)
		if err != nil {
			log.Printf("[DB_ERROR] store request: %v", err)
		}
	}()
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
		DataCh:    make(chan []byte, 1024),
		WriteCh:   make(chan []byte, 1024),
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
			case <-time.After(10 * time.Second):
				log.Printf("[TUNNEL] %s binary send timeout, closing tunnel", tunnel.ID[:8])
				go tm.CloseTunnel(tunnel.ID)
				return
			case <-tunnel.CloseCh:
				return
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
		case <-time.After(5 * time.Second):
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
		atomic.AddInt64(&tunnel.BytesRecvFromNode, int64(len(payload)))
	case <-time.After(5 * time.Second):
		log.Printf("[TUNNEL] %s data channel timeout, closing tunnel (%d bytes lost)", data.TunnelID[:8], len(payload))
		go tm.CloseTunnel(data.TunnelID)
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

	scoresMu sync.RWMutex
	scores   map[string]*NodeScore
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]*Connection),
		disconnects: make([]DisconnectEvent, 0, 10000),
		cooldowns:   make(map[string]*NodeCooldown),
		scores:      make(map[string]*NodeScore),
	}
}

// RecordNodeResult updates quality scores for a node after a tunnel/proxy attempt.
func (h *Hub) RecordNodeResult(nodeID string, success bool, latencyMs int64) {
	h.scoresMu.Lock()
	defer h.scoresMu.Unlock()

	s, ok := h.scores[nodeID]
	if !ok {
		s = &NodeScore{}
		h.scores[nodeID] = s
	}

	if success {
		atomic.AddInt64(&s.SuccessCount, 1)
		atomic.AddInt64(&s.TotalLatency, latencyMs)
	} else {
		atomic.AddInt64(&s.FailCount, 1)
	}
	s.LastUsed = time.Now()

	// Quarantine: if >=5 requests and success rate < 30%, quarantine 10 min
	if s.Total() >= 5 && s.SuccessRate() < 0.30 {
		s.Quarantined = time.Now().Add(10 * time.Minute)
		log.Printf("[SCORE] Quarantined node %s: %.0f%% success (%d/%d)", nodeID, s.SuccessRate()*100, s.SuccessCount, s.Total())
	}
}

// GetNodeScore returns the score for a node (nil if unknown).
func (h *Hub) GetNodeScore(nodeID string) *NodeScore {
	h.scoresMu.RLock()
	defer h.scoresMu.RUnlock()
	return h.scores[nodeID]
}

// GetTopNodes returns up to `count` best connected tunnel-capable nodes, sorted by quality score.
// Excludes quarantined nodes and nodes in skip set.
func (h *Hub) GetTopNodes(country string, count int, skip map[string]bool) []*Connection {
	h.mu.RLock()
	eligible := make([]*Connection, 0, 64)
	for _, c := range h.connections {
		if skip[c.NodeID] {
			continue
		}
		if c.SendCh == nil {
			continue
		}
		if c.SDKVersion != "2.0" && c.SDKVersion != "stability-test-2.0" {
			continue
		}
		if country != "" && c.Country != country {
			continue
		}
		eligible = append(eligible, c)
	}
	// Fallback: ignore country
	if len(eligible) == 0 && country != "" {
		for _, c := range h.connections {
			if skip[c.NodeID] || c.SendCh == nil {
				continue
			}
			if c.SDKVersion != "2.0" && c.SDKVersion != "stability-test-2.0" {
				continue
			}
			eligible = append(eligible, c)
		}
	}
	h.mu.RUnlock()

	// Tiered node selection: good / unknown / bad
	// Good nodes get most traffic, unknown get exploration, bad get minimal
	h.scoresMu.RLock()
	var good, unknown, bad []*Connection
	for _, c := range eligible {
		s := h.scores[c.NodeID]
		if s != nil && s.IsQuarantined() {
			continue
		}
		// Deprioritize VPN/DCH/PUB within each tier
		isVPN := c.ProxyType == "VPN" || c.ProxyType == "DCH" || c.ProxyType == "PUB"

		if s == nil || s.Total() < 3 {
			// Not enough data — exploration pool
			if !isVPN {
				unknown = append(unknown, c)
			} else {
				bad = append(bad, c) // VPN with no data → bad
			}
		} else if s.SuccessRate() >= 0.5 {
			// Proven good node
			if !isVPN {
				good = append(good, c)
			} else {
				unknown = append(unknown, c) // VPN but works → unknown tier
			}
		} else {
			// Bad success rate
			bad = append(bad, c)
		}
	}
	h.scoresMu.RUnlock()

	// Shuffle each tier for load distribution
	rand.Shuffle(len(good), func(i, j int) { good[i], good[j] = good[j], good[i] })
	rand.Shuffle(len(unknown), func(i, j int) { unknown[i], unknown[j] = unknown[j], unknown[i] })
	rand.Shuffle(len(bad), func(i, j int) { bad[i], bad[j] = bad[j], bad[i] })

	// Weighted selection: 80% good, 15% unknown (exploration), 5% bad (re-evaluation)
	var selected []*Connection
	goodCount := int(math.Ceil(float64(count) * 0.80))
	unknownCount := int(math.Ceil(float64(count) * 0.15))
	if unknownCount < 1 {
		unknownCount = 1
	}

	// Take from good tier
	if goodCount > len(good) {
		goodCount = len(good)
	}
	selected = append(selected, good[:goodCount]...)

	// Take from unknown tier (exploration)
	remaining := count - len(selected)
	if unknownCount > remaining {
		unknownCount = remaining
	}
	if unknownCount > len(unknown) {
		unknownCount = len(unknown)
	}
	if unknownCount > 0 {
		selected = append(selected, unknown[:unknownCount]...)
	}

	// Fill remaining from whatever is available: good → unknown → bad
	remaining = count - len(selected)
	if remaining > 0 {
		// More good nodes
		extraGood := good[goodCount:]
		if remaining <= len(extraGood) {
			selected = append(selected, extraGood[:remaining]...)
		} else {
			selected = append(selected, extraGood...)
			remaining = count - len(selected)
			// More unknown
			extraUnknown := unknown[unknownCount:]
			if remaining <= len(extraUnknown) {
				selected = append(selected, extraUnknown[:remaining]...)
			} else {
				selected = append(selected, extraUnknown...)
				remaining = count - len(selected)
				// Last resort: bad nodes
				if remaining > len(bad) {
					remaining = len(bad)
				}
				if remaining > 0 {
					selected = append(selected, bad[:remaining]...)
				}
			}
		}
	}

	// Final shuffle so parallel tunnels don't always hit same order
	rand.Shuffle(len(selected), func(i, j int) { selected[i], selected[j] = selected[j], selected[i] })

	if len(selected) > count {
		selected = selected[:count]
	}
	return selected
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
	log.Printf("[CONNECT] node=%s ip=%s model=%s os=%s sdk=%s total=%d", nodeID, conn.IP, conn.DeviceModel, conn.OS, conn.SDKVersion, h.Count())
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
	// Extract geo fields from ip_info to update in-memory connection
	var cc, city, isp, asn string
	if ipInfo, ok := msg["ip_info"].(map[string]interface{}); ok {
		cc, _ = ipInfo["country_code"].(string)
		city, _ = ipInfo["city_name"].(string)
		isp, _ = ipInfo["isp"].(string)
		asn, _ = ipInfo["asn"].(string)
	}
	ip, _ := msg["ip"].(string)

	// Extract proxy type
	proxyType := ""
	if ipInfo, ok2 := msg["ip_info"].(map[string]interface{}); ok2 {
		if proxy, ok3 := ipInfo["proxy"].(map[string]interface{}); ok3 {
			proxyType, _ = proxy["proxy_type"].(string)
		}
	}

	h.mu.Lock()
	if conn, ok := h.connections[nodeID]; ok {
		conn.HasIPInfo = true
		if cc != "" {
			conn.Country = cc
		}
		if city != "" {
			conn.City = city
		}
		if isp != "" {
			conn.ISP = isp
		}
		if asn != "" {
			conn.ASN = asn
		}
		if ip != "" {
			conn.IP = ip
		}
		if proxyType != "" {
			conn.ProxyType = proxyType
		}
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
	// Exact match first
	if c, ok := h.connections[nodeID]; ok {
		return c
	}
	// Prefix match (short IDs like first 8 chars)
	if len(nodeID) >= 8 {
		for id, c := range h.connections {
			if strings.HasPrefix(id, nodeID) {
				return c
			}
		}
	}
	return nil
}

// GetConnectionByIP finds a connected node by its IP address
func (h *Hub) GetConnectionByIP(ip string) *Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.connections {
		if c.IP == ip {
			return c
		}
	}
	return nil
}

// parseProxyAuth extracts targeting params from Proxy-Authorization header
// Format: user:apikey-node-NODEID-ip-ADDRESS-country-XX
// Returns: nodeID, ip, country (any may be empty)
func parseProxyAuth(req *http.Request) (targetNode, targetIP, targetCountry string) {
	targetNode, targetIP, targetCountry, _ = parseProxyAuthFull(req)
	return
}

func parseProxyAuthFull(req *http.Request) (targetNode, targetIP, targetCountry, customer string) {
	auth := req.Header.Get("Proxy-Authorization")
	if auth == "" {
		return
	}
	if !strings.HasPrefix(auth, "Basic ") {
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) < 2 {
		return
	}
	customer = parts[0]
	password := parts[1]
	tokens := strings.Split(password, "-")
	for i := 0; i < len(tokens)-1; i++ {
		switch tokens[i] {
		case "node":
			targetNode = tokens[i+1]
			i++
		case "ip":
			targetIP = tokens[i+1]
			i++
		case "country":
			targetCountry = strings.ToUpper(tokens[i+1])
			i++
		}
	}
	return
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

// GetTunnelNode finds a connected node that supports tunnels (SDK 2.0+).
// Uses quality scoring to pick the best available node.
func (h *Hub) GetTunnelNode(country string, skip map[string]bool) *Connection {
	nodes := h.GetTopNodes(country, 1, skip)
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

func (h *Hub) Stats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var totalDur float64
	var minDur, maxDur float64
	minDur = 999999
	byModel := make(map[string]int)
	byCountry := make(map[string]int)
	bySDKVersion := make(map[string]int)
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
		sdk := conn.SDKVersion
		if sdk == "" {
			sdk = "unknown"
		}
		bySDKVersion[sdk]++
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
		"by_sdk_version":         bySDKVersion,
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
		OS          string `json:"os"`
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

	nodeOS := hello.OS
	if nodeOS == "" {
		nodeOS = "android"
	}

	nodeConn := &Connection{
		NodeID:      hello.NodeID,
		DeviceModel: hello.DeviceModel,
		SDKVersion:  hello.SDKVersion,
		OS:          nodeOS,
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
			if err == nil {
				// Reset read deadline on any message (not just pong)
				conn.SetReadDeadline(time.Now().Add(pingInterval + pongWait))
			}
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

			case "tunnel_stats":
				if hub.tunnelManager != nil {
					tunnelID, _ := m["tunnelId"].(string)
					if tunnelID != "" {
						sdkSentToTarget := int64(0)
						sdkRecvFromTarget := int64(0)
						if v, ok := m["bytesSentToTarget"].(float64); ok {
							sdkSentToTarget = int64(v)
						}
						if v, ok := m["bytesRecvFromTarget"].(float64); ok {
							sdkRecvFromTarget = int64(v)
						}
						hub.tunnelManager.mu.RLock()
						tunnel, exists := hub.tunnelManager.tunnels[tunnelID]
						hub.tunnelManager.mu.RUnlock()
						short := tunnelID
						if len(short) > 8 {
							short = short[:8]
						}
						if exists {
							serverSent := atomic.LoadInt64(&tunnel.BytesSentToNode)
							serverRecv := atomic.LoadInt64(&tunnel.BytesRecvFromNode)
							// Server sent to node should == SDK sent to target (node relays to target)
							// Server recv from node should == SDK recv from target (target replies via node)
							sentDiff := serverSent - sdkSentToTarget
							recvDiff := sdkRecvFromTarget - serverRecv
							if sentDiff != 0 || recvDiff != 0 {
								log.Printf("[BYTE-MISMATCH] %s server_sent=%d sdk_sentToTarget=%d diff=%d | sdk_recvFromTarget=%d server_recv=%d diff=%d",
									short, serverSent, sdkSentToTarget, sentDiff, sdkRecvFromTarget, serverRecv, recvDiff)
							} else {
								log.Printf("[BYTE-MATCH] %s bytes=%d/%d OK", short, serverSent, serverRecv)
							}
						} else {
							log.Printf("[TUNNEL-STATS] %s tunnel already closed, sdk_sent=%d sdk_recv=%d", short, sdkSentToTarget, sdkRecvFromTarget)
						}
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

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Active/connected nodes
	connected := hub.Count()

	// Top 5 countries from in-memory connections
	hub.mu.RLock()
	countryCount := make(map[string]int)
	for _, c := range hub.connections {
		cc := c.Country
		if cc == "" {
			cc = "unknown"
		}
		countryCount[cc]++
	}
	hub.mu.RUnlock()

	type countryEntry struct {
		Country string `json:"country"`
		Nodes   int    `json:"nodes"`
	}
	countries := make([]countryEntry, 0, len(countryCount))
	for cc, n := range countryCount {
		countries = append(countries, countryEntry{cc, n})
	}
	// Sort by count descending
	for i := 0; i < len(countries); i++ {
		for j := i + 1; j < len(countries); j++ {
			if countries[j].Nodes > countries[i].Nodes {
				countries[i], countries[j] = countries[j], countries[i]
			}
		}
	}
	top5 := countries
	if len(top5) > 5 {
		top5 = top5[:5]
	}

	// Unique nodes last 24h and 1h from ip_info table (updated_at tracks last seen)
	var uniqueNodes24h, uniqueNodes1h int
	db.QueryRow(`SELECT COUNT(*) FROM ip_info WHERE updated_at >= datetime('now', '-24 hours')`).Scan(&uniqueNodes24h)
	db.QueryRow(`SELECT COUNT(*) FROM ip_info WHERE updated_at >= datetime('now', '-1 hours')`).Scan(&uniqueNodes1h)

	// Request stats from requests table — split by type
	queryStats := func(reqType string) map[string]interface{} {
		var total24h, total1h, success24h, success1h int
		db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type = ? AND created_at >= datetime('now', '-24 hours')`, reqType).Scan(&total24h)
		db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type = ? AND created_at >= datetime('now', '-1 hours')`, reqType).Scan(&total1h)
		db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type = ? AND created_at >= datetime('now', '-24 hours') AND success = 1`, reqType).Scan(&success24h)
		db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type = ? AND created_at >= datetime('now', '-1 hours') AND success = 1`, reqType).Scan(&success1h)

		pct24h := float64(0)
		if total24h > 0 {
			pct24h = float64(success24h) * 100 / float64(total24h)
		}
		pct1h := float64(0)
		if total1h > 0 {
			pct1h = float64(success1h) * 100 / float64(total1h)
		}
		return map[string]interface{}{
			"total_24h":        total24h,
			"total_1h":         total1h,
			"success_rate_24h": fmt.Sprintf("%.1f%%", pct24h),
			"success_rate_1h":  fmt.Sprintf("%.1f%%", pct1h),
		}
	}

	tunnelStats := queryStats("tunnel")
	proxyStats := queryStats("proxy")

	result := map[string]interface{}{
		"active_nodes":     connected,
		"unique_nodes_24h": uniqueNodes24h,
		"unique_nodes_1h":  uniqueNodes1h,
		"top_countries":    top5,
		"tunnel":           tunnelStats,
		"proxy":            proxyStats,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(result)
}

func handleSnapshots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Default last 7 days, override with ?days=N
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
		if days < 1 { days = 1 }
		if days > 90 { days = 90 }
	}

	rows, err := db.Query(`SELECT active_nodes, unique_nodes_1h, total_countries,
		top_country, top_country_nodes, tunnel_requests_1h, tunnel_success_1h,
		proxy_requests_1h, proxy_success_1h, sdk_versions, created_at
		FROM snapshots WHERE created_at >= datetime('now', ? || ' days')
		ORDER BY created_at ASC`, fmt.Sprintf("-%d", days))
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	defer rows.Close()

	type snapshot struct {
		ActiveNodes      int    `json:"active_nodes"`
		UniqueNodes1h    int    `json:"unique_nodes_1h"`
		TotalCountries   int    `json:"total_countries"`
		TopCountry       string `json:"top_country"`
		TopCountryNodes  int    `json:"top_country_nodes"`
		TunnelRequests   int    `json:"tunnel_requests_1h"`
		TunnelSuccess    int    `json:"tunnel_success_1h"`
		ProxyRequests    int    `json:"proxy_requests_1h"`
		ProxySuccess     int    `json:"proxy_success_1h"`
		SDKVersions      string `json:"sdk_versions"`
		Timestamp        string `json:"timestamp"`
	}

	snapshots := make([]snapshot, 0)
	for rows.Next() {
		var s snapshot
		rows.Scan(&s.ActiveNodes, &s.UniqueNodes1h, &s.TotalCountries,
			&s.TopCountry, &s.TopCountryNodes, &s.TunnelRequests, &s.TunnelSuccess,
			&s.ProxyRequests, &s.ProxySuccess, &s.SDKVersions, &s.Timestamp)
		snapshots = append(snapshots, s)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":     len(snapshots),
		"days":      days,
		"snapshots": snapshots,
	})
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

	proxyStart := time.Now()
	target := payload.Host + ":" + payload.Port
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
			storeRequest("proxy", nodeID, target, true, int(time.Since(proxyStart).Milliseconds()), "")
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
			storeRequest("proxy", nodeID, target, false, int(time.Since(proxyStart).Milliseconds()), errMsg)
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
	storeRequest("proxy", nodeID, target, false, int(time.Since(proxyStart).Milliseconds()), lastErr)
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

// ─── API: Node Scores endpoint ─────────────────────────────────────────────────

func handleAPINodeScores(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	hub.scoresMu.RLock()
	type scoreEntry struct {
		NodeID      string  `json:"node_id"`
		Success     int64   `json:"success"`
		Fail        int64   `json:"fail"`
		SuccessRate float64 `json:"success_rate"`
		AvgLatency  float64 `json:"avg_latency_ms"`
		LastUsed    string  `json:"last_used"`
		Quarantined bool    `json:"quarantined"`
		Connected   bool    `json:"connected"`
	}

	entries := make([]scoreEntry, 0, len(hub.scores))
	for nodeID, s := range hub.scores {
		connected := hub.GetConnectionByNodeID(nodeID) != nil
		entries = append(entries, scoreEntry{
			NodeID:      nodeID,
			Success:     s.SuccessCount,
			Fail:        s.FailCount,
			SuccessRate: s.SuccessRate(),
			AvgLatency:  s.AvgLatency(),
			LastUsed:    s.LastUsed.Format(time.RFC3339),
			Quarantined: s.IsQuarantined(),
			Connected:   connected,
		})
	}
	hub.scoresMu.RUnlock()

	// Sort by success rate descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SuccessRate > entries[j].SuccessRate
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":  len(entries),
		"scores": entries,
	})
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

	// Parse targeting from auth
	pTargetNode, pTargetIP, pTargetCountry := parseProxyAuth(req)
	var alt *Connection
	if pTargetNode != "" {
		alt = hub.GetConnectionByNodeID(pTargetNode)
	} else if pTargetIP != "" {
		alt = hub.GetConnectionByIP(pTargetIP)
	} else {
		alt = hub.GetAlternativeNode(pTargetCountry, map[string]bool{})
	}
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

// ─── Partner Proxy Chaining ────────────────────────────────────────────────────

// shouldUsePartner decides whether to route through partner pool.
// Returns true for partner, false for own nodes.
// Only applies to US or unspecified country requests.
func shouldUsePartner(targetCountry string) bool {
	// Partner pool is US-only
	if targetCountry != "" && targetCountry != "US" {
		return false
	}
	// Roll dice: partnerOwnPoolPct% chance to use our own nodes
	return rand.Intn(100) >= partnerOwnPoolPct
}

// handlePartnerCONNECT chains a CONNECT request through a random partner server+peer.
// Returns true if handled (success or fail), false if partner unavailable and should fallback.
func handlePartnerCONNECT(clientConn net.Conn, target, customer string) bool {
	// Pick random server and peer
	server := partnerServers[rand.Intn(len(partnerServers))]
	peer := rand.Intn(partnerPeersPerSvr) + 1
	username := fmt.Sprintf("iploop-%d", peer)
	upstreamAddr := server + ":" + partnerPort

	connectStart := time.Now()

	// Connect to partner server
	upstream, err := net.DialTimeout("tcp", upstreamAddr, 15*time.Second)
	if err != nil {
		log.Printf("[PARTNER] Failed to connect to %s: %v", upstreamAddr, err)
		return false // fallback to own nodes
	}
	defer func() {
		// Only close if not already handed off
		if upstream != nil {
			upstream.Close()
		}
	}()

	// Send CONNECT through partner proxy with auth
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + partnerPassword))
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: Basic %s\r\n\r\n",
		target, target, auth)

	upstream.SetDeadline(time.Now().Add(15 * time.Second))
	if _, err := upstream.Write([]byte(connectReq)); err != nil {
		log.Printf("[PARTNER] Failed to send CONNECT to %s: %v", upstreamAddr, err)
		return false
	}

	// Read response
	reader := bufio.NewReader(upstream)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		log.Printf("[PARTNER] Failed to read CONNECT response from %s: %v", upstreamAddr, err)
		return false
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[PARTNER] CONNECT %s via %s peer=%d returned %d", target, server, peer, resp.StatusCode)
		return false
	}

	latencyMs := time.Since(connectStart).Milliseconds()
	log.Printf("[PARTNER] CONNECT %s via %s peer=%d established in %dms", target, server, peer, latencyMs)
	storeRequest("partner", fmt.Sprintf("%s-peer%d", server, peer), target, true, int(latencyMs), "")

	// Send 200 to client
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Clear deadlines for relay
	upstream.SetDeadline(time.Time{})
	clientConn.SetDeadline(time.Time{})

	// Bidirectional relay with byte counting
	var wg sync.WaitGroup
	wg.Add(2)
	var bytesIn, bytesOut int64

	// Client → Partner upstream (bytes_in = client upload)
	go func() {
		defer wg.Done()
		buf := make([]byte, 32768)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				atomic.AddInt64(&bytesIn, int64(n))
				if _, wErr := upstream.Write(buf[:n]); wErr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Partner upstream → Client (bytes_out = client download)
	go func() {
		defer wg.Done()
		buf := make([]byte, 32768)
		for {
			n, err := upstream.Read(buf)
			if n > 0 {
				atomic.AddInt64(&bytesOut, int64(n))
				if _, wErr := clientConn.Write(buf[:n]); wErr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
	}()

	wg.Wait()
	storeBandwidth(customer, "partner", target, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut))
	return true
}

func handleCONNECTRaw(clientConn net.Conn, req *http.Request) {
	host, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
		port = "443"
	}

	target := host + ":" + port
	connectStart := time.Now()
	stabilityWait := 300 * time.Millisecond

	// Parse targeting from auth: node-XXX, ip-XXX, country-XX
	targetNode, targetIP, targetCountry, customer := parseProxyAuthFull(req)

	// If specific node or IP requested, try direct routing
	if targetNode != "" || targetIP != "" {
		var directConn *Connection
		if targetNode != "" {
			directConn = hub.GetConnectionByNodeID(targetNode)
		} else if targetIP != "" {
			directConn = hub.GetConnectionByIP(targetIP)
		}
		if directConn == nil {
			label := targetNode
			if label == "" { label = targetIP }
			log.Printf("[HTTP_PROXY] CONNECT %s targeted node/ip=%s NOT FOUND", target, label)
			clientConn.Write([]byte("HTTP/1.1 503 Targeted Node Not Found\r\n\r\n"))
			return
		}
		log.Printf("[HTTP_PROXY] CONNECT %s targeted node=%s ip=%s", target, directConn.NodeID, directConn.IP)
		t, tErr := hub.tunnelManager.OpenTunnel(directConn.NodeID, host, port)
		if tErr != nil {
			log.Printf("[HTTP_PROXY] Targeted tunnel failed for %s: %v", target, tErr)
			clientConn.Write([]byte("HTTP/1.1 502 Targeted Node Tunnel Failed\r\n\r\n"))
			return
		}
		// Wait for stability
		select {
		case <-t.CloseCh:
			hub.tunnelManager.CloseTunnel(t.ID)
			clientConn.Write([]byte("HTTP/1.1 502 Targeted Tunnel Closed Early\r\n\r\n"))
			return
		case <-time.After(stabilityWait):
		}
		clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		latencyMs := int(time.Since(connectStart).Milliseconds())
		log.Printf("[HTTP_PROXY] Tunnel %s established for %s via targeted node %s in %dms",
			t.ID[:8], target, directConn.NodeID[:16], latencyMs)
		storeRequest("tunnel", directConn.NodeID, target, true, latencyMs, "")

		// Remove deadline for tunnel relay
		clientConn.SetDeadline(time.Time{})

		var wg sync.WaitGroup
		wg.Add(2)
		var tBytesIn, tBytesOut int64
		// Client → Node
		go func() {
			defer wg.Done()
			buf := make([]byte, 32768)
			for {
				n, rErr := clientConn.Read(buf)
				if n > 0 {
					atomic.AddInt64(&tBytesIn, int64(n))
					cpy := make([]byte, n)
					copy(cpy, buf[:n])
					if wErr := t.Write(cpy); wErr != nil { break }
				}
				if rErr != nil { break }
			}
		}()
		// Node → Client
		go func() {
			defer wg.Done()
			for {
				data, rErr := t.Read()
				if len(data) > 0 {
					atomic.AddInt64(&tBytesOut, int64(len(data)))
					if _, wErr := clientConn.Write(data); wErr != nil { break }
				}
				if rErr != nil { break }
			}
		}()
		wg.Wait()
		hub.tunnelManager.CloseTunnel(t.ID)
		storeBandwidth(customer, "tunnel", target, atomic.LoadInt64(&tBytesIn), atomic.LoadInt64(&tBytesOut))
		return
	}

	// ── Partner pool routing (US residential, 95/5 split) ──
	if shouldUsePartner(targetCountry) {
		if handlePartnerCONNECT(clientConn, target, customer) {
			return // handled by partner
		}
		// Partner failed, fall through to our own nodes
		log.Printf("[PARTNER] Fallback to own nodes for %s", target)
	}

	// Parallel tunnel opening with server-side retry (up to 2 rounds)
	type tunnelResult struct {
		tunnel *Tunnel
		conn   *Connection
		err    error
	}

	var winTunnel *Tunnel
	var winConn *Connection
	var lastErr string
	skip := map[string]bool{}
	maxRetries := 2

	for attempt := 0; attempt < maxRetries; attempt++ {
		nodes := hub.GetTopNodes(targetCountry, 3, skip)

		if len(nodes) == 0 {
			if attempt == 0 {
				clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
				log.Printf("[HTTP_PROXY] No tunnel nodes available for %s", target)
				storeRequest("tunnel", "", target, false, int(time.Since(connectStart).Milliseconds()), "no nodes available")
				return
			}
			break
		}

		// Mark these nodes as tried so retry picks different ones
		for _, n := range nodes {
			skip[n.NodeID] = true
		}

		results := make(chan tunnelResult, len(nodes))

		if attempt == 0 {
			log.Printf("[HTTP_PROXY] CONNECT %s parallel via %d nodes", target, len(nodes))
		} else {
			log.Printf("[HTTP_PROXY] CONNECT %s RETRY #%d via %d new nodes", target, attempt, len(nodes))
		}

		for _, node := range nodes {
			go func(n *Connection) {
				start := time.Now()
				t, err := hub.tunnelManager.OpenTunnel(n.NodeID, host, port)
				latency := time.Since(start).Milliseconds()
				if err != nil {
					hub.RecordNodeResult(n.NodeID, false, latency)
					results <- tunnelResult{nil, n, err}
					return
				}
				// Stability check: wait briefly for fast EOF
				select {
				case <-t.CloseCh:
					hub.RecordNodeResult(n.NodeID, false, latency)
					hub.tunnelManager.CloseTunnel(t.ID)
					results <- tunnelResult{nil, n, fmt.Errorf("fast EOF")}
				case <-time.After(stabilityWait):
					hub.RecordNodeResult(n.NodeID, true, latency)
					results <- tunnelResult{t, n, nil}
				}
			}(node)
		}

		// Collect results: use first success, close extras
		collected := 0
		extras := make([]*Tunnel, 0)

		timeout := time.After(10 * time.Second)
		for collected < len(nodes) {
			select {
			case r := <-results:
				collected++
				if r.err != nil {
					lastErr = r.err.Error()
					log.Printf("[HTTP_PROXY] Node %s failed for %s: %v", r.conn.NodeID, target, r.err)
					continue
				}
				if winTunnel == nil {
					winTunnel = r.tunnel
					winConn = r.conn
				} else {
					extras = append(extras, r.tunnel)
				}
				if winTunnel != nil && collected >= 1 {
					goto gotWinner
				}
			case <-timeout:
				goto endRound
			}
		}

	endRound:
		// Close extras from this round
		go func(ch chan tunnelResult, remaining int, ex []*Tunnel) {
			for remaining > 0 {
				select {
				case r := <-ch:
					remaining--
					if r.tunnel != nil {
						ex = append(ex, r.tunnel)
					}
				case <-time.After(12 * time.Second):
					remaining = 0
				}
			}
			for _, t := range ex {
				hub.tunnelManager.CloseTunnel(t.ID)
			}
		}(results, len(nodes)-collected, extras)

		if winTunnel != nil {
			break
		}
		// All failed — retry with different nodes
		continue

	gotWinner:
		// Close extras from this round
		go func(ch chan tunnelResult, remaining int, ex []*Tunnel) {
			for remaining > 0 {
				select {
				case r := <-ch:
					remaining--
					if r.tunnel != nil {
						ex = append(ex, r.tunnel)
					}
				case <-time.After(12 * time.Second):
					remaining = 0
				}
			}
			for _, t := range ex {
				hub.tunnelManager.CloseTunnel(t.ID)
			}
		}(results, len(nodes)-collected, extras)
		break
	}

	if winTunnel == nil {
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		log.Printf("[HTTP_PROXY] All tunnel attempts failed for %s after %d retries", target, maxRetries)
		storeRequest("tunnel", "", target, false, int(time.Since(connectStart).Milliseconds()), lastErr)
		return
	}

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	latencyMs := int(time.Since(connectStart).Milliseconds())
	log.Printf("[HTTP_PROXY] Tunnel %s established for %s via %s in %dms", winTunnel.ID[:8], target, winConn.NodeID, latencyMs)
	storeRequest("tunnel", winConn.NodeID, target, true, latencyMs, "")

	// Remove deadline for tunnel relay
	clientConn.SetDeadline(time.Time{})

	var wg sync.WaitGroup
	wg.Add(2)
	var mainBytesIn, mainBytesOut int64

	// Client → Node
	go func() {
		defer wg.Done()
		buf := make([]byte, 32768)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				atomic.AddInt64(&mainBytesIn, int64(n))
				cpy := make([]byte, n)
				copy(cpy, buf[:n])
				if writeErr := winTunnel.Write(cpy); writeErr != nil {
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
			data, err := winTunnel.ReadWithTimeout(30 * time.Second)
			if err != nil {
				break
			}
			atomic.AddInt64(&mainBytesOut, int64(len(data)))
			if _, writeErr := clientConn.Write(data); writeErr != nil {
				break
			}
		}
	}()

	wg.Wait()
	hub.tunnelManager.CloseTunnel(winTunnel.ID)
	storeBandwidth(customer, "tunnel", target, atomic.LoadInt64(&mainBytesIn), atomic.LoadInt64(&mainBytesOut))
	log.Printf("[HTTP_PROXY] Tunnel %s closed for %s", winTunnel.ID[:8], target)
}

// ─── Bandwidth API ─────────────────────────────────────────────────────────────

func handleAPIBandwidth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	customer := r.URL.Query().Get("customer")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}

	var since string
	switch period {
	case "1h":
		since = "datetime('now', '-1 hours')"
	case "24h":
		since = "datetime('now', '-24 hours')"
	case "7d":
		since = "datetime('now', '-7 days')"
	case "30d":
		since = "datetime('now', '-30 days')"
	case "all":
		since = "datetime('2000-01-01')"
	default:
		since = "datetime('now', '-24 hours')"
	}

	// Per-customer summary
	query := fmt.Sprintf(`SELECT customer, route_type, 
		COUNT(*) as requests, 
		SUM(bytes_in) as total_in, 
		SUM(bytes_out) as total_out,
		SUM(bytes_in + bytes_out) as total_bytes
		FROM bandwidth WHERE created_at >= %s`, since)
	
	if customer != "" {
		query += fmt.Sprintf(` AND customer = '%s'`, customer)
	}
	query += ` GROUP BY customer, route_type ORDER BY total_bytes DESC`

	rows, err := db.Query(query)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type BandwidthEntry struct {
		Customer   string  `json:"customer"`
		RouteType  string  `json:"route_type"`
		Requests   int64   `json:"requests"`
		BytesIn    int64   `json:"bytes_in"`
		BytesOut   int64   `json:"bytes_out"`
		TotalBytes int64   `json:"total_bytes"`
		TotalMB    float64 `json:"total_mb"`
		TotalGB    float64 `json:"total_gb"`
	}

	var entries []BandwidthEntry
	var grandTotal int64
	for rows.Next() {
		var e BandwidthEntry
		rows.Scan(&e.Customer, &e.RouteType, &e.Requests, &e.BytesIn, &e.BytesOut, &e.TotalBytes)
		e.TotalMB = float64(e.TotalBytes) / (1024 * 1024)
		e.TotalGB = float64(e.TotalBytes) / (1024 * 1024 * 1024)
		grandTotal += e.TotalBytes
		entries = append(entries, e)
	}

	result := map[string]interface{}{
		"period":      period,
		"entries":     entries,
		"total_bytes": grandTotal,
		"total_mb":    float64(grandTotal) / (1024 * 1024),
		"total_gb":    float64(grandTotal) / (1024 * 1024 * 1024),
	}
	json.NewEncoder(w).Encode(result)
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

	// Hourly snapshot — saves active node count + stats for graphing
	go func() {
		// Align to next hour boundary
		now := time.Now()
		next := now.Truncate(time.Hour).Add(time.Hour)
		time.Sleep(next.Sub(now))

		for {
			active := hub.Count()

			// Unique nodes last hour
			var unique1h int
			db.QueryRow(`SELECT COUNT(*) FROM ip_info WHERE updated_at >= datetime('now', '-1 hours')`).Scan(&unique1h)

			// Country count
			hub.mu.RLock()
			countrySet := make(map[string]int)
			for _, c := range hub.connections {
				cc := c.Country
				if cc == "" { cc = "unknown" }
				countrySet[cc]++
			}
			// SDK versions
			sdkSet := make(map[string]int)
			for _, c := range hub.connections {
				v := c.SDKVersion
				if v == "" { v = "unknown" }
				sdkSet[v]++
			}
			hub.mu.RUnlock()

			topCC, topN := "", 0
			for cc, n := range countrySet {
				if n > topN { topCC, topN = cc, n }
			}

			sdkJSON, _ := json.Marshal(sdkSet)

			// Request stats last hour
			var tunnelReqs1h, tunnelSuccess1h, proxyReqs1h, proxySuccess1h int
			db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type='tunnel' AND created_at >= datetime('now', '-1 hours')`).Scan(&tunnelReqs1h)
			db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type='tunnel' AND created_at >= datetime('now', '-1 hours') AND success=1`).Scan(&tunnelSuccess1h)
			db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type='proxy' AND created_at >= datetime('now', '-1 hours')`).Scan(&proxyReqs1h)
			db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type='proxy' AND created_at >= datetime('now', '-1 hours') AND success=1`).Scan(&proxySuccess1h)

			_, err := db.Exec(`INSERT INTO snapshots (active_nodes, unique_nodes_1h, total_countries,
				top_country, top_country_nodes, tunnel_requests_1h, tunnel_success_1h,
				proxy_requests_1h, proxy_success_1h, sdk_versions)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				active, unique1h, len(countrySet), topCC, topN,
				tunnelReqs1h, tunnelSuccess1h, proxyReqs1h, proxySuccess1h, string(sdkJSON))
			if err != nil {
				log.Printf("[SNAPSHOT] Error: %v", err)
			} else {
				log.Printf("[SNAPSHOT] Saved: active=%d unique_1h=%d countries=%d top=%s(%d) tunnel=%d/%d proxy=%d/%d",
					active, unique1h, len(countrySet), topCC, topN, tunnelSuccess1h, tunnelReqs1h, proxySuccess1h, proxyReqs1h)
			}

			time.Sleep(1 * time.Hour)
		}
	}()

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
	http.HandleFunc("/dashboard", handleDashboard)
	http.HandleFunc("/snapshots", handleSnapshots)
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

	// API: Node quality scores
	http.HandleFunc("/api/node-scores", handleAPINodeScores)
	http.HandleFunc("/api/bandwidth", handleAPIBandwidth)

	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	// Start HTTP CONNECT proxy on port 8880
	go startHTTPProxy("8880")

	if tlsCert != "" && tlsKey != "" {
		log.Printf("IPLoop Node Server starting on :%s (TLS)", port)
		log.Printf("  WebSocket: wss://0.0.0.0:%s/ws", port)
		log.Printf("  Stats:     https://0.0.0.0:%s/stats", port)
		log.Printf("  API:       https://0.0.0.0:%s/api/...", port)
		log.Printf("  Ping interval: %v, Pong timeout: %v", pingInterval, pongWait)
		if err := http.ListenAndServeTLS(":"+port, tlsCert, tlsKey, nil); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("IPLoop Node Server starting on :%s", port)
		log.Printf("  WebSocket: ws://0.0.0.0:%s/ws", port)
		log.Printf("  Stats:     http://0.0.0.0:%s/stats", port)
		log.Printf("  API:       http://0.0.0.0:%s/api/...", port)
		log.Printf("  Ping interval: %v, Pong timeout: %v", pingInterval, pongWait)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	}
}
