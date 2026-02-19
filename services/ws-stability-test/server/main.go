package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
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

	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	_ "net/http/pprof"
)

// ─── Sticky Session Map ────────────────────────────────────────────────────────

type stickySession struct {
	nodeID   string
	nodeIP   string
	lastUsed time.Time
	country  string
}

// Redis-backed sticky session helpers (shared across GW instances)
const stickySessionTTL = 10 * time.Minute
const stickyPrefix = "sticky:"

func stickyStore(sessionID string, s *stickySession) {
	if rdb == nil { return }
	val := s.nodeID + "|" + s.country + "|" + s.nodeIP
	rdb.Set(context.Background(), stickyPrefix+sessionID, val, stickySessionTTL)
}

func stickyLoad(sessionID string) (*stickySession, bool) {
	if rdb == nil { return nil, false }
	val, err := rdb.Get(context.Background(), stickyPrefix+sessionID).Result()
	if err != nil { return nil, false }
	parts := strings.SplitN(val, "|", 3)
	country := ""
	nodeIP := ""
	if len(parts) > 1 { country = parts[1] }
	if len(parts) > 2 { nodeIP = parts[2] }
	return &stickySession{nodeID: parts[0], nodeIP: nodeIP, lastUsed: time.Now(), country: country}, true
}

func stickyDelete(sessionID string) {
	if rdb == nil { return }
	rdb.Del(context.Background(), stickyPrefix+sessionID)
}

// Legacy in-memory map kept as local cache only
var sessionMap sync.Map // session_id -> *stickySession

// ─── Redis Node Registry ───────────────────────────────────────────────────────

var (
	rdb        *redis.Client
	instanceID string
)

func initRedis() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")

	rdb = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     redisPassword,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     20,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[REDIS] ⚠️ Failed to connect to %s: %v (continuing without Redis)", redisAddr, err)
	} else {
		log.Printf("[REDIS] Connected to %s", redisAddr)
	}

	// Generate instance ID
	instanceID = os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		port := os.Getenv("PORT")
		if port == "" {
			port = "9090"
		}
		instanceID = hostname + ":" + port
	}
	log.Printf("[REDIS] Instance ID: %s", instanceID)

	// Register this instance
	go func() {
		ctx := context.Background()
		if err := rdb.SAdd(ctx, "instances:active", instanceID).Err(); err != nil {
			log.Printf("[REDIS] Failed to register instance: %v", err)
		}
	}()

	// Periodic instance heartbeat (refresh every 2 minutes)
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			rdb.SAdd(ctx, "instances:active", instanceID)
			cancel()
		}
	}()
}

// redisRegisterNode registers a node in Redis on connect (async, non-blocking)
func redisRegisterNode(nodeID, ip, country, city, os, model, sdkVersion string) {
	if rdb == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		key := "node:" + nodeID
		fields := map[string]interface{}{
			"instance_id":  instanceID,
			"ip":           ip,
			"country":      country,
			"city":         city,
			"os":           os,
			"model":        model,
			"sdk":          sdkVersion,
			"connected_at": time.Now().UTC().Format(time.RFC3339),
			"capacity":     1,
		}
		if err := rdb.HSet(ctx, key, fields).Err(); err != nil {
			log.Printf("[REDIS] Failed to register node %s: %v", nodeID, err)
			return
		}
		rdb.Expire(ctx, key, 360*time.Second)
		rdb.SAdd(ctx, "nodes:instance:"+instanceID, nodeID)
		if country != "" {
			rdb.SAdd(ctx, "nodes:country:"+country, nodeID)
		}
	}()
}

// redisUnregisterNode removes a node from Redis on disconnect (async, non-blocking)
func redisUnregisterNode(nodeID, country string) {
	if rdb == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		rdb.Del(ctx, "node:"+nodeID)
		rdb.SRem(ctx, "nodes:instance:"+instanceID, nodeID)
		if country != "" {
			rdb.SRem(ctx, "nodes:country:"+country, nodeID)
		}
	}()
}

// redisRefreshNodeTTL refreshes the TTL for a node on pong (async, non-blocking)
func redisRefreshNodeTTL(nodeID string) {
	if rdb == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		rdb.Expire(ctx, "node:"+nodeID, 360*time.Second)
	}()
}

// redisUpdateNodeIPInfo updates geo info for a node in Redis (async, non-blocking)
func redisUpdateNodeIPInfo(nodeID, oldCountry, newCountry, city, isp string) {
	if rdb == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		fields := map[string]interface{}{}
		if newCountry != "" {
			fields["country"] = newCountry
		}
		if city != "" {
			fields["city"] = city
		}
		if isp != "" {
			fields["isp"] = isp
		}
		if len(fields) > 0 {
			rdb.HSet(ctx, "node:"+nodeID, fields)
		}

		// Update country index if changed
		if oldCountry != newCountry && newCountry != "" {
			if oldCountry != "" {
				rdb.SRem(ctx, "nodes:country:"+oldCountry, nodeID)
			}
			rdb.SAdd(ctx, "nodes:country:"+newCountry, nodeID)
		}
	}()
}

// findNodeForProxy finds a random node in a country via Redis for cross-instance routing
func findNodeForProxy(country string) (nodeID string, nodeInstanceID string, err error) {
	if rdb == nil {
		return "", "", fmt.Errorf("redis not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := "nodes:country:" + country
	nodeID, err = rdb.SRandMember(ctx, key).Result()
	if err != nil {
		return "", "", fmt.Errorf("no nodes in country %s: %v", country, err)
	}

	nodeInstanceID, err = rdb.HGet(ctx, "node:"+nodeID, "instance_id").Result()
	if err != nil {
		return "", "", fmt.Errorf("failed to get instance for node %s: %v", nodeID, err)
	}

	return nodeID, nodeInstanceID, nil
}

// redisGetStats returns Redis-based stats for the dashboard
func redisGetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"instance_id": instanceID,
		"redis_nodes": 0,
		"instances":   []string{},
	}
	if rdb == nil {
		return stats
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Count node keys
	var cursor uint64
	var nodeCount int64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, "node:*", 1000).Result()
		if err != nil {
			break
		}
		nodeCount += int64(len(keys))
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	stats["redis_nodes"] = nodeCount

	// Active instances
	instances, err := rdb.SMembers(ctx, "instances:active").Result()
	if err == nil {
		stats["instances"] = instances
	}

	return stats
}

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
	pingInterval   = 5 * 60 * time.Second // 5 minutes — reduces heartbeat CPU ~80%
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
	Token          string    `json:"token"`
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

// ─── Anti-Abuse: Device Fingerprint ────────────────────────────────────────────
// Hash of node_id + OS + model + IP /24 subnet. Limits registrations per subnet.

var (
	deviceFingerprints   = make(map[string]int)    // fingerprint -> registration count
	deviceFingerprintsMu sync.Mutex
	subnetNodeCount      = make(map[string]int)    // /24 subnet -> node count
	subnetNodeCountMu    sync.Mutex
)

const maxNodesPerSubnet = 5 // max 5 nodes per /24 subnet

// getIPSubnet extracts /24 subnet from IP (e.g., "1.2.3.4" -> "1.2.3.0/24")
func getIPSubnet(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	parsed = parsed.To4()
	if parsed == nil {
		return ip
	}
	return fmt.Sprintf("%d.%d.%d.0/24", parsed[0], parsed[1], parsed[2])
}

// computeDeviceFingerprint hashes node_id + OS + model + /24 subnet
func computeDeviceFingerprint(nodeID, os, model, ip string) string {
	subnet := getIPSubnet(ip)
	h := sha256.Sum256([]byte(nodeID + "|" + os + "|" + model + "|" + subnet))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// ─── Anti-Abuse: Credit Velocity Cap ───────────────────────────────────────────
// Max 10 GB worth of credits per user per day. Resets at midnight UTC.

const maxCreditsPerDay = 10.0 / defaultGBPerCredit // 10 GB / 0.001 GB per credit = 10000 credits/day

var (
	dailyCreditTotals   = make(map[string]float64) // user_id -> credits earned today
	dailyCreditTotalsMu sync.Mutex
	dailyCreditResetDay int                         // day of month for reset tracking
)

// addDailyCredits adds credits for a user, returns actual amount added (may be capped)
func addDailyCredits(userID string, amount float64) float64 {
	dailyCreditTotalsMu.Lock()
	defer dailyCreditTotalsMu.Unlock()

	// Reset at midnight UTC (new day)
	today := time.Now().UTC().Day()
	if today != dailyCreditResetDay {
		dailyCreditTotals = make(map[string]float64)
		dailyCreditResetDay = today
	}

	current := dailyCreditTotals[userID]
	remaining := maxCreditsPerDay - current
	if remaining <= 0 {
		return 0 // already at daily cap
	}
	if amount > remaining {
		amount = remaining // cap to remaining allowance
	}
	dailyCreditTotals[userID] += amount
	return amount
}

// ─── Anti-Abuse: Minimum Uptime Before Credits ────────────────────────────────
// Nodes must be connected >= 5 minutes before earning credits.

const minUptimeForCredits = 5 * time.Minute

// ─── Anti-Abuse: Disposable Email Domains ──────────────────────────────────────

var disposableEmailDomains = map[string]bool{
	"mailinator.com":       true,
	"tempmail.com":         true,
	"guerrillamail.com":    true,
	"throwaway.email":      true,
	"yopmail.com":          true,
	"sharklasers.com":      true,
	"guerrillamailblock.com": true,
	"grr.la":               true,
	"dispostable.com":      true,
	"maildrop.cc":          true,
}

// isDisposableEmail checks if email uses a known disposable domain
func isDisposableEmail(email string) bool {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])
	return disposableEmailDomains[domain]
}

// ─── Anti-Abuse: Global Fallback Rate Limits ───────────────────────────────────
// Unknown/new API keys get max 10 rps, 20 concurrent (instead of auto-created 50).

const (
	globalFallbackRPS        = 10
	globalFallbackConcurrent = 20
)

// ─── DB ────────────────────────────────────────────────────────────────────────

var db *sql.DB

func initDB() {
	var err error
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://iploop:xK9mP2vL7nQ4wR8s@localhost/iploop?sslmode=disable"
	}
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)

	if err = db.Ping(); err != nil {
		log.Fatal("Failed to ping DB:", err)
	}
	log.Println("PostgreSQL DB connected (iploop@localhost)")

	// Create users table for auth system
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		api_key TEXT UNIQUE NOT NULL,
		token TEXT UNIQUE NOT NULL,
		plan TEXT DEFAULT 'free',
		free_gb_remaining REAL DEFAULT 0.5,
		created_at TIMESTAMP DEFAULT NOW(),
		last_login TIMESTAMP,
		ip_address TEXT,
		signup_ip TEXT,
		email_verified BOOLEAN DEFAULT FALSE
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_token ON users(token)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_signup_ip ON users(signup_ip)`)

	// Referral system columns on users
	db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code TEXT`)
	db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS referred_by TEXT`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_referral_code ON users(referral_code)`)

	// Referral events table
	db.Exec(`CREATE TABLE IF NOT EXISTS referrals (
		id SERIAL PRIMARY KEY,
		referrer_user_id TEXT NOT NULL,
		referred_user_id TEXT,
		referral_code TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		referrer_bonus_mb REAL DEFAULT 2048,
		referred_bonus_mb REAL DEFAULT 2048,
		node_boost_pct REAL DEFAULT 50,
		node_boost_days INT DEFAULT 7,
		referrer_passive_pct REAL DEFAULT 10,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		redeemed_at TIMESTAMP
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_referrals_code ON referrals(referral_code)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_user_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_referrals_referred ON referrals(referred_user_id)`)
}

func storeBandwidth(customer, routeType, target string, bytesIn, bytesOut int64) {
	if customer == "" {
		customer = "unknown"
	}
	go func() {
		_, err := db.Exec(`INSERT INTO bandwidth (customer, route_type, target, bytes_in, bytes_out, created_at)
			VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)`,
			customer, routeType, target, bytesIn, bytesOut)
		if err != nil {
			log.Printf("[DB_ERROR] store bandwidth: %v", err)
		}
	}()
}

// ─── Credit System ─────────────────────────────────────────────────────────────

const (
	// Tiered credit rates (credits = MB of proxy access)
	// Bronze: 0-6h uptime  → 50 MB/hour
	// Silver: 6-24h uptime → 75 MB/hour
	// Gold:   24h+ uptime  → 100 MB/hour
	creditRateBronze   = 50.0  // 0-6h
	creditRateSilver   = 75.0  // 6-24h
	creditRateGold     = 100.0 // 24h+
	defaultGBPerCredit = 0.001 // 1000 credits = 1 GB
	multiDeviceBonus   = 0.2

	// Referral system constants
	referralBonusMB    = 2048.0 // 2 GB each (referrer + referred)
	referralBoostPct   = 50.0   // +50% node credits for 7 days
	referralBoostDays  = 7
	referralPassivePct = 10.0   // 10% of referee's earnings forever
)

// calculateTieredCredits computes credits for a session spanning cumulative uptime
// Bronze (0-6h): 50 MB/h, Silver (6-24h): 75 MB/h, Gold (24h+): 100 MB/h
func calculateTieredCredits(startH, endH float64) float64 {
	var credits float64
	// Bronze tier: 0-6h
	if startH < 6 {
		bronzeEnd := math.Min(endH, 6)
		credits += (bronzeEnd - startH) * creditRateBronze
	}
	// Silver tier: 6-24h
	if endH > 6 && startH < 24 {
		silverStart := math.Max(startH, 6)
		silverEnd := math.Min(endH, 24)
		credits += (silverEnd - silverStart) * creditRateSilver
	}
	// Gold tier: 24h+
	if endH > 24 {
		goldStart := math.Max(startH, 24)
		credits += (endH - goldStart) * creditRateGold
	}
	return credits
}

func creditNodeConnected(nodeID, token string) {
	if token == "" {
		return
	}
	go func() {
		// Ensure user exists
		db.Exec(`INSERT INTO credit_users (user_id, token) VALUES ($1, $2) ON CONFLICT (user_id) DO NOTHING`, token, token)
		// Open session
		db.Exec(`INSERT INTO credit_sessions (node_id, user_id, connected_at) VALUES ($1, $2, CURRENT_TIMESTAMP)`,
			nodeID, token)
		log.Printf("[CREDITS] session opened: node=%s user=%s", nodeID, token)
	}()
}

func creditNodeDisconnected(nodeID string) {
	go func() {
		var sessionID int64
		var userID string
		var connectedAt string
		err := db.QueryRow(
			`SELECT id, user_id, connected_at FROM credit_sessions
			 WHERE node_id = $1 AND disconnected_at IS NULL
			 ORDER BY connected_at DESC LIMIT 1`, nodeID,
		).Scan(&sessionID, &userID, &connectedAt)
		if err != nil {
			return // No active credit session
		}

		// Calculate hours from connected_at
		var hours float64
		db.QueryRow(`SELECT EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - $1::timestamp)) / 3600`, connectedAt).Scan(&hours)
		if hours < 0.001 {
			hours = 0.001
		}

		// Anti-abuse: Minimum uptime before credits (5 minutes)
		if hours < minUptimeForCredits.Hours() {
			// Close session without awarding credits
			db.Exec(`UPDATE credit_sessions SET disconnected_at = CURRENT_TIMESTAMP, uptime_hours = $1, credits_earned = 0 WHERE id = $2`, hours, sessionID)
			log.Printf("[CREDITS] session closed (no credits, uptime %.1fmin < 5min): node=%s user=%s", hours*60, nodeID, userID)
			return
		}

		// Tiered credit calculation based on cumulative uptime
		var totalUptime float64
		db.QueryRow(`SELECT COALESCE(SUM(uptime_hours), 0) FROM credit_sessions WHERE user_id = $1 AND disconnected_at IS NOT NULL`, userID).Scan(&totalUptime)
		cumulativeStart := totalUptime
		cumulativeEnd := totalUptime + hours
		credits := calculateTieredCredits(cumulativeStart, cumulativeEnd)

		// Multi-device bonus
		var activeDevices int
		db.QueryRow(`SELECT COUNT(*) FROM credit_sessions WHERE user_id = $1 AND disconnected_at IS NULL`, userID).Scan(&activeDevices)
		if activeDevices > 1 {
			credits *= 1.0 + float64(activeDevices-1)*multiDeviceBonus
		}

		// Referral boost: +50% for first 7 days if referred
		credits *= getReferralBoostMultiplier(userID)

		// Anti-abuse: Credit velocity cap — max 10 GB/day per user
		credits = addDailyCredits(userID, credits)
		if credits <= 0 {
			db.Exec(`UPDATE credit_sessions SET disconnected_at = CURRENT_TIMESTAMP, uptime_hours = $1, credits_earned = 0 WHERE id = $2`, hours, sessionID)
			log.Printf("[CREDITS] session closed (daily cap reached): node=%s user=%s hours=%.2f", nodeID, userID, hours)
			return
		}

		// Close session + add credits
		db.Exec(`UPDATE credit_sessions SET disconnected_at = CURRENT_TIMESTAMP, uptime_hours = $1, credits_earned = $2 WHERE id = $3`,
			hours, credits, sessionID)
		db.Exec(`UPDATE credit_users SET credits = credits + $1, total_earned = total_earned + $2 WHERE user_id = $3`,
			credits, credits, userID)
		db.Exec(`INSERT INTO credit_log (user_id, amount, reason) VALUES ($1, $2, $3)`,
			userID, credits, fmt.Sprintf("uptime %.2fh node=%s", hours, nodeID))
		log.Printf("[CREDITS] session closed: node=%s user=%s hours=%.2f credits=%.2f", nodeID, userID, hours, credits)

		// Passive referral earnings: award 10% to referrer
		go awardReferralPassive(userID, credits)
	}()
}

// creditPeriodicUpdate runs every hour and updates credits for all active sessions
// without closing them — so users see real-time balance.
func creditPeriodicUpdate() {
	for {
		time.Sleep(1 * time.Hour)
		rows, err := db.Query(`SELECT id, node_id, user_id, connected_at FROM credit_sessions WHERE disconnected_at IS NULL`)
		if err != nil {
			log.Printf("[CREDITS] periodic update query error: %v", err)
			continue
		}
		updated := 0
		for rows.Next() {
			var sessionID int64
			var nodeID, userID, connectedAt string
			rows.Scan(&sessionID, &nodeID, &userID, &connectedAt)

			var hours float64
			db.QueryRow(`SELECT EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - $1::timestamp)) / 3600`, connectedAt).Scan(&hours)
			if hours < 0.001 {
				continue
			}

			// Anti-abuse: skip credit update if node uptime < 5 minutes
			if hours < minUptimeForCredits.Hours() {
				continue
			}

			// Tiered credits based on cumulative uptime
			var totalUptime float64
			db.QueryRow(`SELECT COALESCE(SUM(uptime_hours), 0) FROM credit_sessions WHERE user_id = $1 AND disconnected_at IS NOT NULL`, userID).Scan(&totalUptime)
			credits := calculateTieredCredits(totalUptime, totalUptime+hours)

			// Referral boost: +50% for first 7 days if referred
			credits *= getReferralBoostMultiplier(userID)

			// Get previous earned for this session to calculate delta
			var prevEarned float64
			db.QueryRow(`SELECT COALESCE(credits_earned, 0) FROM credit_sessions WHERE id = $1`, sessionID).Scan(&prevEarned)
			delta := credits - prevEarned
			if delta <= 0 {
				continue
			}

			// Anti-abuse: Credit velocity cap — apply daily limit
			delta = addDailyCredits(userID, delta)
			if delta <= 0 {
				continue
			}

			// Update session and user balance
			db.Exec(`UPDATE credit_sessions SET uptime_hours = $1, credits_earned = $2 WHERE id = $3`, hours, prevEarned+delta, sessionID)
			db.Exec(`UPDATE credit_users SET credits = credits + $1, total_earned = total_earned + $2 WHERE user_id = $3`, delta, delta, userID)
			// Passive referral earnings
			go awardReferralPassive(userID, delta)
			updated++
		}
		rows.Close()
		if updated > 0 {
			log.Printf("[CREDITS] periodic update: %d active sessions updated", updated)
		}
	}
}

// ─── Customer Auth & Rate Limiting ──────────────────────────────────────────────

type Customer struct {
	APIKey        string `json:"api_key"`
	Name          string `json:"name"`
	QuotaBytes    int64  `json:"quota_bytes"`
	RateLimit     int    `json:"rate_limit"`      // base requests/sec
	MaxConcurrent int    `json:"max_concurrent"`
	Enabled       bool   `json:"enabled"`
}

var (
	customerCache   = make(map[string]*Customer)
	customerCacheMu sync.RWMutex
	
	// Per-customer concurrent connection counter
	customerConns   = make(map[string]*int64)
	customerConnsMu sync.Mutex
	
	// Per-customer sliding window rate limiter
	customerRequests   = make(map[string]*rateLimiter)
	customerRequestsMu sync.Mutex
)

type rateLimiter struct {
	tokens    float64
	maxTokens float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

func newRateLimiter(rps float64) *rateLimiter {
	return &rateLimiter{
		tokens:     rps * 2, // start with 2 seconds of burst
		maxTokens:  rps * 5, // allow 5 second burst
		refillRate: rps,
		lastRefill: time.Now(),
	}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens = math.Min(rl.maxTokens, rl.tokens + elapsed * rl.refillRate)
	rl.lastRefill = now
	
	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

func (rl *rateLimiter) updateRate(rps float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refillRate = rps
	rl.maxTokens = rps * 5
}

// getCustomer looks up a customer by API key. Auto-creates with 5GB free if not found.
func getCustomer(apiKey string) (*Customer, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("no API key")
	}
	
	// Check cache
	customerCacheMu.RLock()
	if c, ok := customerCache[apiKey]; ok {
		customerCacheMu.RUnlock()
		return c, nil
	}
	customerCacheMu.RUnlock()
	
	// Check DB
	var c Customer
	var enabled int
	err := db.QueryRow(`SELECT api_key, name, quota_bytes, rate_limit, max_concurrent, enabled 
		FROM customers WHERE api_key = $1`, apiKey).Scan(
		&c.APIKey, &c.Name, &c.QuotaBytes, &c.RateLimit, &c.MaxConcurrent, &enabled)
	
	if err != nil {
		// Auto-create new customer with 5GB free and restricted fallback limits
		c = Customer{
			APIKey:        apiKey,
			Name:          apiKey,
			QuotaBytes:    5 * 1024 * 1024 * 1024, // 5GB
			RateLimit:     globalFallbackRPS,        // Anti-abuse: new/unknown keys get 10 rps
			MaxConcurrent: globalFallbackConcurrent,  // Anti-abuse: new/unknown keys get 20 concurrent
			Enabled:       true,
		}
		_, dbErr := db.Exec(`INSERT INTO customers (api_key, name, quota_bytes, rate_limit, max_concurrent, enabled)
			VALUES ($1, $2, $3, $4, $5, 1)`, c.APIKey, c.Name, c.QuotaBytes, c.RateLimit, c.MaxConcurrent)
		if dbErr != nil {
			log.Printf("[CUSTOMER] Failed to create customer %s: %v", apiKey, dbErr)
		} else {
			log.Printf("[CUSTOMER] Auto-created customer %s with 5GB free (fallback limits: %d rps, %d concurrent)", apiKey, globalFallbackRPS, globalFallbackConcurrent)
		}
	} else {
		c.Enabled = enabled == 1
	}
	
	if !c.Enabled {
		return nil, fmt.Errorf("account disabled")
	}
	
	// Cache it
	customerCacheMu.Lock()
	customerCache[apiKey] = &c
	customerCacheMu.Unlock()
	
	return &c, nil
}

// getCustomerUsedBytes returns total bytes used this month
func getCustomerUsedBytes(apiKey string) int64 {
	var used int64
	db.QueryRow(`SELECT COALESCE(SUM(bytes_in + bytes_out), 0) FROM bandwidth 
		WHERE customer = $1 AND created_at >= date_trunc('month', CURRENT_TIMESTAMP)`, apiKey).Scan(&used)
	return used
}

// dynamicRateLimit returns effective rate limit based on usage.
// New accounts (< 100 requests) get 2x rate to help them succeed.
func dynamicRateLimit(c *Customer) float64 {
	var reqCount int64
	db.QueryRow(`SELECT COUNT(*) FROM bandwidth WHERE customer = $1 AND created_at >= NOW() - INTERVAL '1 hour'`, c.APIKey).Scan(&reqCount)
	
	baseRate := float64(c.RateLimit)
	if reqCount < 100 {
		return baseRate * 2 // 2x for new/light users
	}
	return baseRate
}

// getConnCounter returns atomic counter for customer concurrent connections
func getConnCounter(apiKey string) *int64 {
	customerConnsMu.Lock()
	defer customerConnsMu.Unlock()
	if c, ok := customerConns[apiKey]; ok {
		return c
	}
	var counter int64
	customerConns[apiKey] = &counter
	return &counter
}

// getRateLimiter returns rate limiter for customer
func getRateLimiter(apiKey string, rps float64) *rateLimiter {
	customerRequestsMu.Lock()
	defer customerRequestsMu.Unlock()
	if rl, ok := customerRequests[apiKey]; ok {
		rl.updateRate(rps)
		return rl
	}
	rl := newRateLimiter(rps)
	customerRequests[apiKey] = rl
	return rl
}

// authorizeProxy validates customer auth, checks quota, rate limit, and concurrency.
// Auth format: username:apikey-country-XX-node-YY
// API key = first token of password. Customer = username.
func authorizeProxy(req *http.Request) (string, *Customer, string) {
	auth := req.Header.Get("Proxy-Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Basic ") {
		return "", nil, "auth required"
	}
	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return "", nil, "invalid auth"
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) < 2 {
		return "", nil, "invalid auth format"
	}
	
	_ = parts[0] // customerName (username)
	tokens := strings.Split(parts[1], "-")
	apiKey := tokens[0] // first token before any -params
	
	c, err2 := getCustomer(apiKey)
	if err2 != nil {
		return apiKey, nil, err2.Error()
	}
	
	// Check bandwidth quota
	used := getCustomerUsedBytes(apiKey)
	if used >= c.QuotaBytes {
		return apiKey, c, fmt.Sprintf("quota exceeded (used %.2f GB / %.2f GB)", 
			float64(used)/(1024*1024*1024), float64(c.QuotaBytes)/(1024*1024*1024))
	}
	
	// Check rate limit (dynamic — new accounts get 2x)
	effectiveRate := dynamicRateLimit(c)
	rl := getRateLimiter(apiKey, effectiveRate)
	if !rl.allow() {
		return apiKey, c, "rate limit exceeded"
	}
	
	// Check concurrent connections
	counter := getConnCounter(apiKey)
	current := atomic.AddInt64(counter, 1)
	if current > int64(c.MaxConcurrent) {
		atomic.AddInt64(counter, -1)
		return apiKey, c, fmt.Sprintf("max concurrent connections (%d)", c.MaxConcurrent)
	}
	
	return apiKey, c, ""
}

// releaseConn decrements concurrent connection counter
func releaseConn(apiKey string) {
	if apiKey == "" { return }
	counter := getConnCounter(apiKey)
	atomic.AddInt64(counter, -1)
}

func storeRequest(reqType, nodeID, target string, success bool, latencyMs int, errMsg string) {
	successInt := 0
	if success {
		successInt = 1
	}
	go func() {
		_, err := db.Exec(`INSERT INTO requests (type, node_id, target, success, latency_ms, error, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)`,
			reqType, nodeID, target, successInt, latencyMs, errMsg)
		if err != nil {
			log.Printf("[DB_ERROR] store request: %v", err)
		}
	}()
}

// Cache for IP info DB writes — skip duplicate writes for same node+IP within 1h
var (
	ipInfoWriteCacheMu sync.RWMutex
	ipInfoWriteCache   = make(map[string]time.Time) // key: nodeID+ip -> last write time
)

func storeIPInfo(nodeID string, msg map[string]interface{}, nodeOS string) {
	if nodeOS == "" {
		nodeOS = "android"
	}
	ipInfo, _ := msg["ip_info"].(map[string]interface{})
	if ipInfo == nil {
		return
	}

	// Skip DB write if same node+IP was written within 1 hour
	ip, _ := msg["ip"].(string)
	cacheKey := nodeID + ":" + ip
	ipInfoWriteCacheMu.RLock()
	if lastWrite, ok := ipInfoWriteCache[cacheKey]; ok && time.Since(lastWrite) < 24*time.Hour {
		ipInfoWriteCacheMu.RUnlock()
		return
	}
	ipInfoWriteCacheMu.RUnlock()

	deviceID, _ := msg["device_id"].(string)
	deviceModel, _ := msg["device_model"].(string)
	// ip already extracted above for cache key
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
			country_code, city, isp, asn, proxy_type, ip_info_json, os, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, CURRENT_TIMESTAMP)
		ON CONFLICT(node_id) DO UPDATE SET
			device_id=EXCLUDED.device_id, device_model=EXCLUDED.device_model, ip=EXCLUDED.ip,
			ip_fetch_ms=EXCLUDED.ip_fetch_ms, info_fetch_ms=EXCLUDED.info_fetch_ms,
			country_code=EXCLUDED.country_code, city=EXCLUDED.city, isp=EXCLUDED.isp,
			asn=EXCLUDED.asn, proxy_type=EXCLUDED.proxy_type, ip_info_json=EXCLUDED.ip_info_json,
			os=EXCLUDED.os, updated_at=CURRENT_TIMESTAMP
	`, nodeID, deviceID, deviceModel, ip, int(ipFetchMs), int(infoFetchMs),
		countryCode, city, isp, asn, proxyType, string(ipInfoJSON), nodeOS)

	if err != nil {
		log.Printf("[DB_ERROR] store ip_info: %v", err)
	} else {
		// Mark in write cache
		ipInfoWriteCacheMu.Lock()
		ipInfoWriteCache[cacheKey] = time.Now()
		ipInfoWriteCacheMu.Unlock()
	}
}

func storeDisconnect(event DisconnectEvent) {
	_, err := db.Exec(`
		INSERT INTO disconnects (node_id, ip, reason, duration_sec, pings_sent, pongs_recv, proxy_type, country)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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

// ─── IP Geo Cache ──────────────────────────────────────────────────────────────
// Cache IP geo lookups to avoid repeated HTTP calls for the same IP.
// Nodes reconnect frequently with the same IP — no need to re-fetch.

type geoEntry struct {
	info      *IPGeoInfo
	fetchedAt time.Time
}

var (
	geoCacheMu sync.RWMutex
	geoCache   = make(map[string]geoEntry)
	geoCacheTTL = 30 * 24 * time.Hour // 30 days — geo data rarely changes
)

func geoCacheCleanup() {
	for {
		time.Sleep(10 * time.Minute)
		geoCacheMu.Lock()
		now := time.Now()
		for k, v := range geoCache {
			if now.Sub(v.fetchedAt) > geoCacheTTL {
				delete(geoCache, k)
			}
		}
		geoCacheMu.Unlock()
	}
}

// lookupIPGeo fetches geolocation for an IP address via ip-api.com (with cache)
func lookupIPGeo(ip string) (*IPGeoInfo, error) {
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "172.") || ip == "127.0.0.1" || ip == "::1" {
		return nil, fmt.Errorf("private IP address")
	}

	// Check cache first
	geoCacheMu.RLock()
	if entry, ok := geoCache[ip]; ok && time.Since(entry.fetchedAt) < geoCacheTTL {
		geoCacheMu.RUnlock()
		return entry.info, nil
	}
	geoCacheMu.RUnlock()

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

	// Store in cache
	geoCacheMu.Lock()
	geoCache[ip] = geoEntry{info: &geo, fetchedAt: time.Now()}
	geoCacheMu.Unlock()

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
	// Log every 100th connection to reduce CPU overhead
	if h.totalConns%100 == 0 {
		log.Printf("[CONNECT] total=%d (last: node=%s ip=%s os=%s)", h.Count(), nodeID, conn.IP, conn.OS)
	}
	// Credit tracking for Docker/community nodes
	if conn.Token != "" {
		creditNodeConnected(nodeID, conn.Token)
	}
	// Redis node registry
	redisRegisterNode(nodeID, conn.IP, conn.Country, conn.City, conn.OS, conn.DeviceModel, conn.SDKVersion)
}

func (h *Hub) Remove(nodeID, reason string) {
	h.mu.Lock()
	conn, ok := h.connections[nodeID]
	if ok {
		dur := time.Since(conn.ConnectedAt)
		proxyType := conn.ProxyType
		country := conn.Country
		// Use in-memory data instead of DB queries on every disconnect
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
		// Credit tracking
		if conn.Token != "" {
			creditNodeDisconnected(nodeID)
		}
		// Redis node registry
		redisUnregisterNode(nodeID, country)
		// Log every 100th disconnect to reduce overhead
		if h.totalDiscons%100 == 0 {
			log.Printf("[DISCONNECT] total_disconnects=%d active=%d (last: node=%s duration=%s)",
				h.totalDiscons, h.Count(), nodeID, dur.Round(time.Second))
		}
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
	// Refresh Redis TTL on pong
	redisRefreshNodeTTL(nodeID)
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

	nodeOS := "android"
	var oldCountry string
	h.mu.Lock()
	if conn, ok := h.connections[nodeID]; ok {
		conn.HasIPInfo = true
		oldCountry = conn.Country
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
		nodeOS = conn.OS
	}
	h.mu.Unlock()
	go storeIPInfo(nodeID, msg, nodeOS)
	// Update Redis with new geo info
	redisUpdateNodeIPInfo(nodeID, oldCountry, cc, city, isp)
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
	targetNode, targetIP, targetCountry, _, _ = parseProxyAuthFull(req)
	return
}

func parseProxyAuthFull(req *http.Request) (targetNode, targetIP, targetCountry, customer, sessionID string) {
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
		case "session":
			sessionID = tokens[i+1]
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

// ─── DDoS / abuse protection ───────────────────────────────────────────────────

const (
	maxWSPerIP       = 50
	maxTotalWS       = 30000
	apiRateLimitN    = 60 // requests per minute per IP
	apiRateWindowSec = 60
)

var (
	wsPerIP      sync.Map // ip -> *int64
	totalWSConns int64
)

func wsIPTrack(ip string) bool {
	val, _ := wsPerIP.LoadOrStore(ip, new(int64))
	cnt := val.(*int64)
	if atomic.AddInt64(cnt, 1) > maxWSPerIP {
		atomic.AddInt64(cnt, -1)
		return false
	}
	return true
}

func wsIPRelease(ip string) {
	if val, ok := wsPerIP.Load(ip); ok {
		cnt := val.(*int64)
		if atomic.AddInt64(cnt, -1) <= 0 {
			wsPerIP.Delete(ip)
		}
	}
}

type apiRateBucket struct {
	mu      sync.Mutex
	tokens  int
	limit   int
	last    time.Time
	resetAt time.Time
}

var apiRateBuckets sync.Map

// getPlanRateLimit returns requests-per-minute for a given plan
func getPlanRateLimit(plan string) int {
	switch strings.ToLower(plan) {
	case "starter":
		return 120
	case "growth":
		return 300
	case "business":
		return 600
	case "enterprise":
		return 1000
	case "free":
		return 30
	default:
		return 30
	}
}

// lookupPlanByAPIKey returns the plan for an API key (cached in memory for 60s)
var planCache sync.Map // apiKey -> {plan string, expiry time.Time}

type planCacheEntry struct {
	plan   string
	expiry time.Time
}

func lookupPlanByAPIKey(apiKey string) string {
	if apiKey == "" {
		return "free"
	}
	// Check cache
	if v, ok := planCache.Load(apiKey); ok {
		entry := v.(*planCacheEntry)
		if time.Now().Before(entry.expiry) {
			return entry.plan
		}
	}
	// DB lookup
	var plan string
	err := db.QueryRow(`SELECT COALESCE(plan, 'free') FROM users WHERE api_key = $1`, apiKey).Scan(&plan)
	if err != nil {
		plan = "free"
	}
	planCache.Store(apiKey, &planCacheEntry{plan: plan, expiry: time.Now().Add(60 * time.Second)})
	return plan
}

func apiRateAllowTiered(key string, limit int) (allowed bool, remaining int, resetUnix int64) {
	now := time.Now()
	windowEnd := now.Truncate(time.Minute).Add(time.Minute)

	val, _ := apiRateBuckets.LoadOrStore(key, &apiRateBucket{tokens: limit, limit: limit, last: now, resetAt: windowEnd})
	b := val.(*apiRateBucket)
	b.mu.Lock()
	defer b.mu.Unlock()

	// If window has passed, reset
	if now.After(b.resetAt) {
		b.tokens = limit
		b.limit = limit
		b.resetAt = now.Truncate(time.Minute).Add(time.Minute)
	} else if b.limit != limit {
		// Plan changed — adjust
		b.limit = limit
		if b.tokens > limit {
			b.tokens = limit
		}
	}

	resetUnix = b.resetAt.Unix()

	if b.tokens <= 0 {
		return false, 0, resetUnix
	}
	b.tokens--
	return true, b.tokens, resetUnix
}

func rateLimitAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := extractAPIKey(r)
		plan := lookupPlanByAPIKey(apiKey)
		limit := getPlanRateLimit(plan)

		// Rate limit key: prefer api_key, fallback to IP
		rateKey := apiKey
		if rateKey == "" {
			rateKey = "ip:" + extractClientIP(r)
		}

		allowed, remaining, resetUnix := apiRateAllowTiered(rateKey, limit)

		// Always set rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetUnix, 10))

		if !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "rate limit exceeded",
				"limit": limit,
				"reset": resetUnix,
			})
			return
		}
		next(w, r)
	}
}

// ─── WebSocket handler ─────────────────────────────────────────────────────────

var hub = NewHub()

func handleWS(w http.ResponseWriter, r *http.Request) {
	// DDoS protection: total connection cap
	if atomic.LoadInt64(&totalWSConns) >= maxTotalWS {
		http.Error(w, `{"error":"server at capacity"}`, http.StatusServiceUnavailable)
		return
	}
	ip := extractClientIP(r)
	// DDoS protection: per-IP connection limit
	if !wsIPTrack(ip) {
		http.Error(w, `{"error":"too many connections from your IP"}`, http.StatusTooManyRequests)
		return
	}
	atomic.AddInt64(&totalWSConns, 1)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] upgrade: %v", err)
		return
	}

	// DDoS protection: release counters when this handler returns
	defer func() {
		atomic.AddInt64(&totalWSConns, -1)
		wsIPRelease(ip)
	}()

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
		Token       string `json:"token"`
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
		Token:       hello.Token,
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

	// Anti-abuse: Device fingerprint — limit registrations per /24 subnet
	fingerprint := computeDeviceFingerprint(nodeID, conn.OS, conn.DeviceModel, clientIP)
	subnet := getIPSubnet(clientIP)

	subnetNodeCountMu.Lock()
	subnetCount := subnetNodeCount[subnet]
	if subnetCount >= maxNodesPerSubnet {
		subnetNodeCountMu.Unlock()
		log.Printf("[REGISTER] Rejected: too many nodes from subnet %s (count=%d, fingerprint=%s)", subnet, subnetCount, fingerprint[:16])
		errMsg, _ := json.Marshal(Message{
			Type: "error",
			Data: map[string]interface{}{
				"error":     "Too many nodes registered from this network",
				"timestamp": time.Now().UTC(),
			},
		})
		conn.SafeWrite(websocket.TextMessage, errMsg)
		return
	}
	subnetNodeCount[subnet]++
	subnetNodeCountMu.Unlock()

	// Track fingerprint
	deviceFingerprintsMu.Lock()
	deviceFingerprints[fingerprint]++
	deviceFingerprintsMu.Unlock()

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
		// No server-side lookups — SDK provides all geo data
		log.Printf("[REGISTER] No geo from SDK: node=%s ip=%s sdk=%s", regData.DeviceID, clientIP, regData.SDKVersion)
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
	osCount := make(map[string]int)
	for _, c := range hub.connections {
		cc := c.Country
		if cc == "" {
			cc = "unknown"
		}
		countryCount[cc]++
		osName := c.OS
		if osName == "" {
			osName = "android"
		}
		osCount[osName]++
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
	db.QueryRow(`SELECT COUNT(*) FROM ip_info WHERE updated_at >= NOW() - INTERVAL '24 hours'`).Scan(&uniqueNodes24h)
	db.QueryRow(`SELECT COUNT(*) FROM ip_info WHERE updated_at >= NOW() - INTERVAL '1 hour'`).Scan(&uniqueNodes1h)

	// Unique nodes OS breakdown (24h)
	uniqueOSCount := make(map[string]int)
	osRows, err := db.Query(`SELECT COALESCE(os, 'android') as os, COUNT(*) FROM ip_info WHERE updated_at >= NOW() - INTERVAL '24 hours' GROUP BY os`)
	if err == nil {
		for osRows.Next() {
			var osName string
			var cnt int
			osRows.Scan(&osName, &cnt)
			if osName == "" {
				osName = "android"
			}
			uniqueOSCount[osName] += cnt
		}
		osRows.Close()
	}

	// Request stats from requests table — split by type
	queryStats := func(reqType string) map[string]interface{} {
		var total24h, total1h, success24h, success1h int
		db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type = $1 AND created_at >= NOW() - INTERVAL '24 hours'`, reqType).Scan(&total24h)
		db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type = $1 AND created_at >= NOW() - INTERVAL '1 hour'`, reqType).Scan(&total1h)
		db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type = $1 AND created_at >= NOW() - INTERVAL '24 hours' AND success = 1`, reqType).Scan(&success24h)
		db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type = $1 AND created_at >= NOW() - INTERVAL '1 hour' AND success = 1`, reqType).Scan(&success1h)

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

	// Redis stats
	redisStats := redisGetStats()

	// Global node count from Redis (all instances combined)
	globalNodes := redisStats["redis_nodes"]
	if globalNodes == nil || globalNodes == 0 {
		globalNodes = connected // fallback to local if Redis unavailable
	}

	// Global country counts from Redis
	globalCountries := make(map[string]int)
	if rdb != nil {
		rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer rcancel()
		var ccCursor uint64
		for {
			keys, nextCursor, err := rdb.Scan(rctx, ccCursor, "nodes:country:*", 100).Result()
			if err != nil {
				break
			}
			for _, k := range keys {
				cc := k[len("nodes:country:"):]
				cnt, err := rdb.SCard(rctx, k).Result()
				if err == nil {
					globalCountries[cc] = int(cnt)
				}
			}
			ccCursor = nextCursor
			if ccCursor == 0 {
				break
			}
		}
	}

	// Build global top countries if available
	if len(globalCountries) > 0 {
		countries = make([]countryEntry, 0, len(globalCountries))
		for cc, n := range globalCountries {
			countries = append(countries, countryEntry{cc, n})
		}
		for i := 0; i < len(countries); i++ {
			for j := i + 1; j < len(countries); j++ {
				if countries[j].Nodes > countries[i].Nodes {
					countries[i], countries[j] = countries[j], countries[i]
				}
			}
		}
		top5 = countries
		if len(top5) > 5 {
			top5 = top5[:5]
		}
	}

	result := map[string]interface{}{
		"active_nodes":     globalNodes,
		"local_nodes":      connected,
		"unique_nodes_24h": uniqueNodes24h,
		"unique_nodes_1h":  uniqueNodes1h,
		"total_countries":  len(globalCountries),
		"top_countries":    top5,
		"os_active":        osCount,
		"os_unique_24h":    uniqueOSCount,
		"tunnel":           tunnelStats,
		"proxy":            proxyStats,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"redis_nodes":      redisStats["redis_nodes"],
		"instance_id":      redisStats["instance_id"],
		"instances":        redisStats["instances"],
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
		FROM snapshots WHERE created_at >= NOW() - ($1 || ' days')::interval
		ORDER BY created_at ASC`, fmt.Sprintf("%d", days))
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
	pTargetNode, pTargetIP, pTargetCountry, _, pSessionID := parseProxyAuthFull(req)

	// Sticky session for plain HTTP (Redis-backed)
	if pSessionID != "" && pTargetNode == "" && pTargetIP == "" {
		s, found := stickyLoad(pSessionID)
		if !found {
			if val, ok := sessionMap.Load(pSessionID); ok {
				s = val.(*stickySession)
				found = true
			}
		}
		if found {
			conn := hub.GetConnectionByNodeID(s.nodeID)
			// Cross-GW: node might be on the other GW, try finding same IP locally
			if conn == nil && s.nodeIP != "" {
				conn = hub.GetConnectionByIP(s.nodeIP)
			}
			if conn != nil {
				s.lastUsed = time.Now()
				s.nodeID = conn.NodeID // update to local node ID
				s.nodeIP = conn.IP
				stickyStore(pSessionID, s)
				sessionMap.Store(pSessionID, s)
				pTargetNode = conn.NodeID
			} else {
				// Don't delete from Redis — other GW might still have the node
				sessionMap.Delete(pSessionID)
			}
		}
	}

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

	// Store sticky session for plain HTTP (Redis + local)
	if pSessionID != "" {
		ss := &stickySession{nodeID: alt.NodeID, nodeIP: alt.IP, lastUsed: time.Now(), country: pTargetCountry}
		stickyStore(pSessionID, ss)
		sessionMap.Store(pSessionID, ss)
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

// handlePartnerCONNECTExcluding tries a partner server not in the exclude set.
// Adds the tried server to exclude. Returns true only on SUCCESS (relay started).
func handlePartnerCONNECTExcluding(clientConn net.Conn, target, customer string, exclude map[string]bool) bool {
	// Pick random server not in exclude set (max 10 attempts to find one)
	var server string
	for i := 0; i < 10; i++ {
		s := partnerServers[rand.Intn(len(partnerServers))]
		if !exclude[s] {
			server = s
			break
		}
	}
	if server == "" {
		return false // all servers tried
	}
	exclude[server] = true
	return handlePartnerCONNECTWithServer(clientConn, target, customer, server)
}

// handlePartnerCONNECT chains a CONNECT request through a random partner server+peer.
// Returns true if handled (success or fail), false if partner unavailable and should fallback.
func handlePartnerCONNECT(clientConn net.Conn, target, customer string) bool {
	return handlePartnerCONNECTWithServer(clientConn, target, customer, partnerServers[rand.Intn(len(partnerServers))])
}

func handlePartnerCONNECTWithServer(clientConn net.Conn, target, customer, server string) bool {
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

	// Authorize customer
	customer, cust, rejectReason := authorizeProxy(req)
	if rejectReason != "" {
		log.Printf("[AUTH] Rejected %s for %s: %s", customer, target, rejectReason)
		clientConn.Write([]byte("HTTP/1.1 407 " + rejectReason + "\r\n\r\n"))
		return
	}
	_ = cust
	defer releaseConn(customer)

	// Parse targeting from auth: node-XXX, ip-XXX, country-XX, session-XXX
	targetNode, targetIP, targetCountry, _, sessionID := parseProxyAuthFull(req)

	// Sticky session: resolve session_id → node_id (Redis-backed, shared across GW instances)
	if sessionID != "" && targetNode == "" && targetIP == "" {
		s, found := stickyLoad(sessionID)
		if !found {
			// Fallback: check local cache
			if val, ok := sessionMap.Load(sessionID); ok {
				s = val.(*stickySession)
				found = true
			}
		}
		if found {
			// Check if the node is still connected
			conn := hub.GetConnectionByNodeID(s.nodeID)
			// Cross-GW: node might be on the other GW, try finding same IP locally
			if conn == nil && s.nodeIP != "" {
				conn = hub.GetConnectionByIP(s.nodeIP)
			}
			if conn != nil {
				// Node still alive — use it
				s.lastUsed = time.Now()
				s.nodeID = conn.NodeID
				s.nodeIP = conn.IP
				stickyStore(sessionID, s)
				sessionMap.Store(sessionID, s)
				targetNode = conn.NodeID
				log.Printf("[SESSION] Reusing session %s → node=%s ip=%s", sessionID, targetNode, conn.IP)
			} else {
				// Don't delete from Redis — other GW might still have the node
				sessionMap.Delete(sessionID)
				log.Printf("[SESSION] Node %s (ip=%s) not on this GW, clearing local cache for session %s", s.nodeID, s.nodeIP, sessionID)
			}
		}
	}

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

		// Store sticky session mapping (Redis + local)
		if sessionID != "" {
			ss := &stickySession{nodeID: directConn.NodeID, nodeIP: directConn.IP, lastUsed: time.Now(), country: targetCountry}
			stickyStore(sessionID, ss)
			sessionMap.Store(sessionID, ss)
			log.Printf("[SESSION] Stored session %s → node=%s ip=%s (Redis)", sessionID, directConn.NodeID, directConn.IP)
		}

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
	if sessionID == "" && shouldUsePartner(targetCountry) {
		// Retry up to 3 different partner servers before fallback
		triedServers := map[string]bool{}
		for attempt := 0; attempt < 3; attempt++ {
			if handlePartnerCONNECTExcluding(clientConn, target, customer, triedServers) {
				return // handled by partner
			}
		}
		// All partner attempts failed, fall through to our own nodes
		log.Printf("[PARTNER] All 3 retries failed, fallback to own nodes for %s", target)
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

	// Store sticky session mapping (Redis + local)
	if sessionID != "" {
		ss := &stickySession{nodeID: winConn.NodeID, nodeIP: winConn.IP, lastUsed: time.Now(), country: targetCountry}
		stickyStore(sessionID, ss)
		sessionMap.Store(sessionID, ss)
		log.Printf("[SESSION] Stored session %s → node=%s (Redis)", sessionID, winConn.NodeID)
	}

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
		since = "NOW() - INTERVAL '1 hour'"
	case "24h":
		since = "NOW() - INTERVAL '24 hours'"
	case "7d":
		since = "NOW() - INTERVAL '7 days'"
	case "30d":
		since = "NOW() - INTERVAL '30 days'"
	case "all":
		since = "'2000-01-01'::timestamp"
	default:
		since = "NOW() - INTERVAL '24 hours'"
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

func handleAPICredits(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := r.URL.Query().Get("user")

	if userID != "" {
		// Single user balance
		var credits, totalEarned, totalSpent, proxyGBUsed float64
		err := db.QueryRow(`SELECT credits, total_earned, total_spent, proxy_gb_used FROM credit_users WHERE user_id = $1`, userID).
			Scan(&credits, &totalEarned, &totalSpent, &proxyGBUsed)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
			return
		}
		var activeDevices int
		db.QueryRow(`SELECT COUNT(*) FROM credit_sessions WHERE user_id = $1 AND disconnected_at IS NULL`, userID).Scan(&activeDevices)
		var totalUptime float64
		db.QueryRow(`SELECT COALESCE(SUM(uptime_hours), 0) FROM credit_sessions WHERE user_id = $1`, userID).Scan(&totalUptime)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"user_id":          userID,
			"credits":          credits,
			"total_earned":     totalEarned,
			"total_spent":      totalSpent,
			"proxy_gb_used":    proxyGBUsed,
			"proxy_gb_balance": credits * defaultGBPerCredit,
			"active_devices":   activeDevices,
			"total_uptime_h":   totalUptime,
		})
		return
	}

	// All users summary
	rows, err := db.Query(`SELECT user_id, credits, total_earned, total_spent, proxy_gb_used, created_at FROM credit_users ORDER BY credits DESC`)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var uid, createdAt string
		var credits, earned, spent, gbUsed float64
		rows.Scan(&uid, &credits, &earned, &spent, &gbUsed, &createdAt)
		users = append(users, map[string]interface{}{
			"user_id":      uid,
			"credits":      credits,
			"total_earned": earned,
			"total_spent":  spent,
			"proxy_gb":     gbUsed,
			"created_at":   createdAt,
		})
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

func handleAPICustomers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "POST" {
		// Create/update customer
		var input struct {
			APIKey        string `json:"api_key"`
			Name          string `json:"name"`
			QuotaGB       float64 `json:"quota_gb"`
			RateLimit     int    `json:"rate_limit"`
			MaxConcurrent int    `json:"max_concurrent"`
			Enabled       bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		quotaBytes := int64(input.QuotaGB * 1024 * 1024 * 1024)
		if quotaBytes == 0 { quotaBytes = 5 * 1024 * 1024 * 1024 }
		if input.RateLimit == 0 { input.RateLimit = 10 }
		if input.MaxConcurrent == 0 { input.MaxConcurrent = 50 }
		enabled := 1
		if !input.Enabled { enabled = 0 }

		_, err := db.Exec(`INSERT INTO customers (api_key, name, quota_bytes, rate_limit, max_concurrent, enabled)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT(api_key) DO UPDATE SET name=EXCLUDED.name, quota_bytes=EXCLUDED.quota_bytes, rate_limit=EXCLUDED.rate_limit, max_concurrent=EXCLUDED.max_concurrent, enabled=EXCLUDED.enabled`,
			input.APIKey, input.Name, quotaBytes, input.RateLimit, input.MaxConcurrent, enabled)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		// Invalidate cache
		customerCacheMu.Lock()
		delete(customerCache, input.APIKey)
		customerCacheMu.Unlock()

		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "api_key": input.APIKey})
		return
	}

	// List customers with usage
	rows, err := db.Query(`SELECT c.api_key, c.name, c.quota_bytes, c.rate_limit, c.max_concurrent, c.enabled,
		COALESCE(b.used, 0) as used_bytes
		FROM customers c
		LEFT JOIN (SELECT customer, SUM(bytes_in + bytes_out) as used 
			FROM bandwidth WHERE created_at >= date_trunc('month', CURRENT_TIMESTAMP) GROUP BY customer) b
		ON c.api_key = b.customer
		ORDER BY used_bytes DESC`)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type CustomerInfo struct {
		APIKey        string  `json:"api_key"`
		Name          string  `json:"name"`
		QuotaGB       float64 `json:"quota_gb"`
		UsedGB        float64 `json:"used_gb"`
		UsedPct       float64 `json:"used_pct"`
		RateLimit     int     `json:"rate_limit"`
		MaxConcurrent int     `json:"max_concurrent"`
		Enabled       bool    `json:"enabled"`
	}

	var customers []CustomerInfo
	for rows.Next() {
		var apiKey, name string
		var quotaBytes, usedBytes int64
		var rateLimit, maxConcurrent, enabled int
		rows.Scan(&apiKey, &name, &quotaBytes, &rateLimit, &maxConcurrent, &enabled, &usedBytes)
		ci := CustomerInfo{
			APIKey:        apiKey,
			Name:          name,
			QuotaGB:       float64(quotaBytes) / (1024 * 1024 * 1024),
			UsedGB:        float64(usedBytes) / (1024 * 1024 * 1024),
			RateLimit:     rateLimit,
			MaxConcurrent: maxConcurrent,
			Enabled:       enabled == 1,
		}
		if quotaBytes > 0 {
			ci.UsedPct = float64(usedBytes) / float64(quotaBytes) * 100
		}
		customers = append(customers, ci)
	}
	json.NewEncoder(w).Encode(customers)
}

// ─── CORS helper ───────────────────────────────────────────────────────────────

func corsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func generateHexToken(n int) string {
	b := make([]byte, n)
	crand.Read(b)
	return hex.EncodeToString(b)
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func getAPIKeyFromRequest(r *http.Request) string {
	if k := r.URL.Query().Get("api_key"); k != "" {
		return k
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return ""
}

// ─── Referral System ────────────────────────────────────────────────────────────

func generateReferralCode() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	crand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 || len(parts[0]) == 0 {
		return "***"
	}
	name := parts[0]
	if len(name) <= 1 {
		return name + "***@" + parts[1]
	}
	return name[:1] + "***@" + parts[1]
}

// applyReferralBoost checks if user was referred and within boost window, returns multiplier
func getReferralBoostMultiplier(userID string) float64 {
	var referredBy sql.NullString
	var createdAt time.Time
	err := db.QueryRow(`SELECT referred_by, created_at FROM users WHERE token = $1`, userID).Scan(&referredBy, &createdAt)
	if err != nil || !referredBy.Valid || referredBy.String == "" {
		return 1.0
	}
	if time.Since(createdAt).Hours() > float64(referralBoostDays*24) {
		return 1.0
	}
	return 1.0 + referralBoostPct/100.0
}

// awardReferralPassive awards passive earnings to referrer (10% of referee's credits)
func awardReferralPassive(userID string, credits float64) {
	if credits <= 0 {
		return
	}
	var referredBy sql.NullString
	err := db.QueryRow(`SELECT referred_by FROM users WHERE token = $1`, userID).Scan(&referredBy)
	if err != nil || !referredBy.Valid || referredBy.String == "" {
		return
	}
	// Find referrer's token
	var referrerToken string
	err = db.QueryRow(`SELECT token FROM users WHERE referral_code = $1`, referredBy.String).Scan(&referrerToken)
	if err != nil || referrerToken == "" {
		return
	}
	passive := credits * referralPassivePct / 100.0
	if passive < 0.01 {
		return
	}
	db.Exec(`UPDATE credit_users SET credits = credits + $1, total_earned = total_earned + $2 WHERE user_id = $3`, passive, passive, referrerToken)
	db.Exec(`INSERT INTO credit_log (user_id, amount, reason) VALUES ($1, $2, $3)`, referrerToken, passive, fmt.Sprintf("referral_passive from %s", userID))
}

func handleReferralInfo(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(200)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	apiKey := getAPIKeyFromRequest(r)
	if apiKey == "" {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing api_key"})
		return
	}

	var token, referralCode string
	err := db.QueryRow(`SELECT token, COALESCE(referral_code, '') FROM users WHERE api_key = $1`, apiKey).Scan(&token, &referralCode)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	// Count referrals
	var totalReferrals int
	db.QueryRow(`SELECT COUNT(*) FROM referrals WHERE referrer_user_id = $1 AND status = 'redeemed'`, token).Scan(&totalReferrals)

	// Total earned from referral bonuses
	var totalEarnedMB float64
	db.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM credit_log WHERE user_id = $1 AND (reason = 'referral_bonus' OR reason LIKE 'referral_passive%%')`, token).Scan(&totalEarnedMB)

	// Passive earnings only
	var passiveEarnings float64
	db.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM credit_log WHERE user_id = $1 AND reason LIKE 'referral_passive%%'`, token).Scan(&passiveEarnings)

	// Get referral list
	type referralEntry struct {
		Email       string `json:"email"`
		Date        string `json:"date"`
		Status      string `json:"status"`
		NodeRunning bool   `json:"node_running"`
	}
	var referrals []referralEntry

	rows, err := db.Query(`SELECT r.referred_user_id, r.created_at, r.status FROM referrals r WHERE r.referrer_user_id = $1 ORDER BY r.created_at DESC LIMIT 50`, token)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var refUserID, status string
			var createdAt time.Time
			rows.Scan(&refUserID, &createdAt, &status)

			var email string
			db.QueryRow(`SELECT email FROM users WHERE token = $1`, refUserID).Scan(&email)

			// Check if referred user has active node
			var nodeRunning bool
			var activeCount int
			db.QueryRow(`SELECT COUNT(*) FROM credit_sessions WHERE user_id = $1 AND disconnected_at IS NULL`, refUserID).Scan(&activeCount)
			nodeRunning = activeCount > 0

			referrals = append(referrals, referralEntry{
				Email:       maskEmail(email),
				Date:        createdAt.Format("2006-01-02"),
				Status:      status,
				NodeRunning: nodeRunning,
			})
		}
	}
	if referrals == nil {
		referrals = []referralEntry{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"referral_code":      referralCode,
		"referral_link":      "https://iploop.io/signup.html?ref=" + referralCode,
		"total_referrals":    totalReferrals,
		"total_earned_mb":    totalEarnedMB,
		"passive_earnings_mb": passiveEarnings,
		"referrals":          referrals,
	})
}

func handleReferralStats(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(200)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	apiKey := getAPIKeyFromRequest(r)
	if apiKey == "" {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing api_key"})
		return
	}

	var token string
	err := db.QueryRow(`SELECT token FROM users WHERE api_key = $1`, apiKey).Scan(&token)
	if err != nil {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	// User's referral count
	var myReferrals int
	db.QueryRow(`SELECT COUNT(*) FROM referrals WHERE referrer_user_id = $1 AND status = 'redeemed'`, token).Scan(&myReferrals)

	// Leaderboard position
	var position int
	db.QueryRow(`SELECT COUNT(DISTINCT referrer_user_id) + 1 FROM referrals WHERE status = 'redeemed' GROUP BY referrer_user_id HAVING COUNT(*) > $1`, myReferrals).Scan(&position)
	if position == 0 {
		position = 1
	}

	// Total users with referrals
	var totalReferrers int
	db.QueryRow(`SELECT COUNT(DISTINCT referrer_user_id) FROM referrals WHERE status = 'redeemed'`).Scan(&totalReferrers)

	// Top 10 leaderboard
	type leaderEntry struct {
		Email     string `json:"email"`
		Referrals int    `json:"referrals"`
	}
	var leaderboard []leaderEntry
	rows, err := db.Query(`SELECT r.referrer_user_id, COUNT(*) as cnt FROM referrals r WHERE r.status = 'redeemed' GROUP BY r.referrer_user_id ORDER BY cnt DESC LIMIT 10`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var uid string
			var cnt int
			rows.Scan(&uid, &cnt)
			var email string
			db.QueryRow(`SELECT email FROM users WHERE token = $1`, uid).Scan(&email)
			leaderboard = append(leaderboard, leaderEntry{Email: maskEmail(email), Referrals: cnt})
		}
	}
	if leaderboard == nil {
		leaderboard = []leaderEntry{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"my_referrals":    myReferrals,
		"position":        position,
		"total_referrers": totalReferrers,
		"leaderboard":     leaderboard,
	})
}

func handleAuthSignup(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(200)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Ref      string `json:"ref"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Ref = strings.TrimSpace(strings.ToUpper(req.Ref))
	if !emailRegex.MatchString(req.Email) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid email format"})
		return
	}

	// Anti-abuse: block disposable email domains
	if isDisposableEmail(req.Email) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "disposable email addresses are not allowed"})
		return
	}

	if len(req.Password) < 8 {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "password must be at least 8 characters"})
		return
	}

	// Anti-abuse: max 3 signups per IP per day
	clientIP := strings.Split(r.RemoteAddr, ":")[0]
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		clientIP = strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	var ipCount int
	db.QueryRow(`SELECT COUNT(*) FROM users WHERE signup_ip = $1 AND created_at >= NOW() - INTERVAL '1 day'`, clientIP).Scan(&ipCount)
	if ipCount >= 3 {
		w.WriteHeader(429)
		json.NewEncoder(w).Encode(map[string]string{"error": "too many signups from this IP"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
		return
	}

	apiKey := generateHexToken(16) // 32 hex chars
	token := generateHexToken(16)

	var userID int
	err = db.QueryRow(`INSERT INTO users (email, password_hash, api_key, token, signup_ip, ip_address)
		VALUES ($1, $2, $3, $4, $5, $5) RETURNING id`,
		req.Email, string(hash), apiKey, token, clientIP).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			w.WriteHeader(409)
			json.NewEncoder(w).Encode(map[string]string{"error": "email already registered"})
		} else {
			log.Printf("[AUTH] signup error: %v", err)
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
		}
		return
	}

	// Auto-create credit_users entry
	db.Exec(`INSERT INTO credit_users (user_id, token) VALUES ($1, $2) ON CONFLICT (user_id) DO NOTHING`, token, token)

	// Auto-create customers entry with 0.5GB free
	db.Exec(`INSERT INTO customers (api_key, name, quota_bytes, rate_limit, max_concurrent, enabled)
		VALUES ($1, $2, $3, $4, $5, 1) ON CONFLICT (api_key) DO NOTHING`,
		apiKey, req.Email, int64(0.5*1024*1024*1024), 10, 50)

	// Generate referral code for new user
	refCode := generateReferralCode()
	db.Exec(`UPDATE users SET referral_code = $1 WHERE id = $2`, refCode, userID)

	// Process referral if ref code provided
	var referredByCode string
	if req.Ref != "" {
		var referrerToken string
		err := db.QueryRow(`SELECT token FROM users WHERE referral_code = $1`, req.Ref).Scan(&referrerToken)
		if err == nil && referrerToken != "" {
			referredByCode = req.Ref
			// Set referred_by on new user
			db.Exec(`UPDATE users SET referred_by = $1 WHERE id = $2`, req.Ref, userID)

			// Award bonus credits to both (2 GB each)
			db.Exec(`UPDATE credit_users SET credits = credits + $1, total_earned = total_earned + $2 WHERE user_id = $3`, referralBonusMB, referralBonusMB, referrerToken)
			db.Exec(`INSERT INTO credit_log (user_id, amount, reason) VALUES ($1, $2, 'referral_bonus')`, referrerToken, referralBonusMB)

			db.Exec(`UPDATE credit_users SET credits = credits + $1, total_earned = total_earned + $2 WHERE user_id = $3`, referralBonusMB, referralBonusMB, token)
			db.Exec(`INSERT INTO credit_log (user_id, amount, reason) VALUES ($1, $2, 'referral_bonus')`, token, referralBonusMB)

			// Log referral event
			db.Exec(`INSERT INTO referrals (referrer_user_id, referred_user_id, referral_code, status, redeemed_at) VALUES ($1, $2, $3, 'redeemed', CURRENT_TIMESTAMP)`,
				referrerToken, token, req.Ref)

			log.Printf("[REFERRAL] %s referred by %s (code=%s), 2GB bonus each", req.Email, referrerToken, req.Ref)
		}
	}

	log.Printf("[AUTH] signup: user=%d email=%s ip=%s ref=%s", userID, req.Email, clientIP, referredByCode)

	// Send welcome email (async, non-blocking)
	go sendWelcomeEmail(req.Email, apiKey, token)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":       userID,
		"email":         req.Email,
		"api_key":       apiKey,
		"token":         token,
		"free_gb":       0.5,
		"referral_code": refCode,
	})
}

// ─── Welcome Email via Resend ──────────────────────────────────────────────────

const resendAPIKey = "re_be1nbzw6_LwbJ9idGDigxztWK2uKKtVVv"

func sendWelcomeEmail(email, apiKey, token string) {
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:600px;margin:0 auto;background:#0a0a0a;color:#e0e0e0;padding:40px 20px;">

<div style="text-align:center;margin-bottom:30px;">
  <h1 style="color:#00d4ff;font-size:28px;margin:0;">🌐 Welcome to IPLoop</h1>
  <p style="color:#888;font-size:14px;">Your residential proxy network is ready</p>
</div>

<div style="background:#1a1a2e;border-radius:12px;padding:24px;margin-bottom:20px;">
  <h2 style="color:#00d4ff;font-size:18px;margin-top:0;">🎁 Your Free Tier</h2>
  <p>You start with <strong style="color:#00ff88;">0.5 GB free</strong> proxy data. No credit card needed.</p>
  <p style="font-size:13px;color:#888;">Your API Key:</p>
  <code style="background:#0d0d1a;color:#00ff88;padding:8px 12px;border-radius:6px;display:block;word-break:break-all;font-size:13px;">%s</code>
</div>

<div style="background:#1a1a2e;border-radius:12px;padding:24px;margin-bottom:20px;">
  <h2 style="color:#00d4ff;font-size:18px;margin-top:0;">🚀 Quick Start</h2>
  <p>Use your proxy right now:</p>
  <code style="background:#0d0d1a;color:#ccc;padding:8px 12px;border-radius:6px;display:block;font-size:12px;word-break:break-all;">curl -x "http://user:%s@gateway.iploop.io:8880" https://httpbin.org/ip</code>
  <p style="margin-top:12px;">Target a country:</p>
  <code style="background:#0d0d1a;color:#ccc;padding:8px 12px;border-radius:6px;display:block;font-size:12px;word-break:break-all;">curl -x "http://user:%s-country-US@gateway.iploop.io:8880" https://example.com</code>
</div>

<div style="background:#1a1a2e;border-radius:12px;padding:24px;margin-bottom:20px;">
  <h2 style="color:#00d4ff;font-size:18px;margin-top:0;">💰 Earn Free Proxy Credits</h2>
  <p>Share your unused bandwidth and earn credits. Just run:</p>
  <code style="background:#0d0d1a;color:#00ff88;padding:8px 12px;border-radius:6px;display:block;font-size:12px;">docker run -d --name iploop-node --restart=always ultronloop2026/iploop-node:latest</code>

  <table style="width:100%%;margin-top:16px;border-collapse:collapse;font-size:14px;">
    <tr style="border-bottom:1px solid #333;">
      <th style="text-align:left;padding:8px;color:#888;">Tier</th>
      <th style="text-align:left;padding:8px;color:#888;">Uptime</th>
      <th style="text-align:left;padding:8px;color:#888;">Rate</th>
      <th style="text-align:left;padding:8px;color:#888;">Daily</th>
    </tr>
    <tr style="border-bottom:1px solid #222;">
      <td style="padding:8px;">🥉 Bronze</td>
      <td style="padding:8px;">0 – 6h</td>
      <td style="padding:8px;">50 MB/hr</td>
      <td style="padding:8px;">300 MB</td>
    </tr>
    <tr style="border-bottom:1px solid #222;">
      <td style="padding:8px;">🥈 Silver</td>
      <td style="padding:8px;">6 – 24h</td>
      <td style="padding:8px;">75 MB/hr</td>
      <td style="padding:8px;">1.35 GB</td>
    </tr>
    <tr>
      <td style="padding:8px;">🥇 Gold</td>
      <td style="padding:8px;">24h+</td>
      <td style="padding:8px;">100 MB/hr</td>
      <td style="padding:8px;color:#00ff88;"><strong>2.4 GB/day</strong></td>
    </tr>
  </table>
  <p style="color:#888;font-size:13px;margin-top:8px;">Run 24/7 → earn <strong style="color:#00ff88;">~70 GB/month</strong> in free proxy credits!</p>
</div>

<div style="background:#1a1a2e;border-radius:12px;padding:24px;margin-bottom:20px;">
  <h2 style="color:#00d4ff;font-size:18px;margin-top:0;">📊 Upgrade Plans</h2>
  <table style="width:100%%;border-collapse:collapse;font-size:14px;">
    <tr style="border-bottom:1px solid #333;">
      <th style="text-align:left;padding:8px;color:#888;">Plan</th>
      <th style="text-align:left;padding:8px;color:#888;">Data</th>
      <th style="text-align:left;padding:8px;color:#888;">Price</th>
      <th style="text-align:left;padding:8px;color:#888;">Per GB</th>
    </tr>
    <tr style="border-bottom:1px solid #222;">
      <td style="padding:8px;">Free</td><td style="padding:8px;">0.5 GB</td><td style="padding:8px;">$0</td><td style="padding:8px;">Free</td>
    </tr>
    <tr style="border-bottom:1px solid #222;">
      <td style="padding:8px;">Starter</td><td style="padding:8px;">10 GB</td><td style="padding:8px;">$10/mo</td><td style="padding:8px;">$1.00</td>
    </tr>
    <tr style="border-bottom:1px solid #222;">
      <td style="padding:8px;">Growth</td><td style="padding:8px;">50 GB</td><td style="padding:8px;">$40/mo</td><td style="padding:8px;">$0.80</td>
    </tr>
    <tr>
      <td style="padding:8px;">Business</td><td style="padding:8px;">200 GB</td><td style="padding:8px;">$120/mo</td><td style="padding:8px;">$0.60</td>
    </tr>
    <tr>
      <td style="padding:8px;">Enterprise</td><td style="padding:8px;">1 TB+</td><td style="padding:8px;">Custom</td><td style="padding:8px;">$0.40+</td>
    </tr>
  </table>
  <p style="color:#aaa;font-size:14px;margin-top:12px;">Need more? Enterprise plans from $0.40/GB → <a href="mailto:partners@iploop.io" style="color:#00d4ff;">partners@iploop.io</a></p>
</div>

<div style="background:#1a1a2e;border-radius:12px;padding:24px;margin-bottom:20px;">
  <h2 style="color:#00d4ff;font-size:18px;margin-top:0;">🌍 Network</h2>
  <p><strong>2,000,000+</strong> residential IPs • <strong>50+</strong> countries • Android, Windows, Mac & Smart TV devices</p>
</div>

<div style="text-align:center;padding:20px 0;border-top:1px solid #222;">
  <p style="color:#888;font-size:13px;">
    <a href="https://iploop.io" style="color:#00d4ff;text-decoration:none;">Dashboard</a> •
    <a href="https://github.com/iploop/iploop-node" style="color:#00d4ff;text-decoration:none;">GitHub</a> •
    <a href="https://iploop.io/privacy.html" style="color:#00d4ff;text-decoration:none;">Privacy</a> •
    <a href="https://iploop.io/terms.html" style="color:#00d4ff;text-decoration:none;">Terms</a>
  </p>
  <p style="color:#888;font-size:13px;">Questions? Email <a href="mailto:partners@iploop.io" style="color:#00d4ff;text-decoration:none;">partners@iploop.io</a> or chat on <a href="https://t.me/iploop_support" style="color:#00d4ff;text-decoration:none;">Telegram</a></p>
  <p style="color:#555;font-size:11px;">© 2026 IPLoop. All rights reserved.</p>
</div>

</body>
</html>`, apiKey, apiKey, apiKey)

	payload := fmt.Sprintf(`{"from":"IPLoop <noreply@iploop.io>","to":["%s"],"subject":"Welcome to IPLoop — Your API Key & Free 0.5 GB","html":%s}`,
		email, strconv.Quote(htmlBody))

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", strings.NewReader(payload))
	if err != nil {
		log.Printf("[EMAIL] welcome email build error: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+resendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[EMAIL] welcome email send error: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		log.Printf("[EMAIL] welcome email failed (%d): %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	} else {
		log.Printf("[EMAIL] welcome email sent to %s", email)
	}
}

func handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(200)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var userID int
	var passwordHash, apiKey, token, plan string
	var freeGB float32
	err := db.QueryRow(`SELECT id, password_hash, api_key, token, plan, free_gb_remaining FROM users WHERE email = $1`,
		req.Email).Scan(&userID, &passwordHash, &apiKey, &token, &plan, &freeGB)
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid email or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid email or password"})
		return
	}

	clientIP := strings.Split(r.RemoteAddr, ":")[0]
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		clientIP = strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	db.Exec(`UPDATE users SET last_login = NOW(), ip_address = $1 WHERE id = $2`, clientIP, userID)

	log.Printf("[AUTH] login: user=%d email=%s", userID, req.Email)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":          userID,
		"email":            req.Email,
		"api_key":          apiKey,
		"token":            token,
		"plan":             plan,
		"free_gb_remaining": freeGB,
	})
}

func handleAuthMe(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(200)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	apiKey := getAPIKeyFromRequest(r)
	if apiKey == "" {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "api_key required"})
		return
	}

	var userID int
	var email, token, plan string
	var freeGB float32
	err := db.QueryRow(`SELECT id, email, token, plan, free_gb_remaining FROM users WHERE api_key = $1`, apiKey).
		Scan(&userID, &email, &token, &plan, &freeGB)
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid api_key"})
		return
	}

	// Credit balance
	var credits, totalEarned, totalSpent, proxyGBUsed float64
	db.QueryRow(`SELECT COALESCE(credits,0), COALESCE(total_earned,0), COALESCE(total_spent,0), COALESCE(proxy_gb_used,0) FROM credit_users WHERE token = $1`, token).
		Scan(&credits, &totalEarned, &totalSpent, &proxyGBUsed)

	// Active nodes
	var activeNodes int
	hub.mu.RLock()
	for _, c := range hub.connections {
		if c.Token == token {
			activeNodes++
		}
	}
	hub.mu.RUnlock()

	// Bandwidth used this month
	var bytesUsed int64
	db.QueryRow(`SELECT COALESCE(SUM(bytes_in + bytes_out), 0) FROM bandwidth WHERE customer = $1 AND created_at >= date_trunc('month', CURRENT_TIMESTAMP)`, apiKey).Scan(&bytesUsed)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":          userID,
		"email":            email,
		"plan":             plan,
		"free_gb_remaining": freeGB,
		"credits":          credits,
		"total_earned":     totalEarned,
		"total_spent":      totalSpent,
		"proxy_gb_used":    proxyGBUsed,
		"active_nodes":     activeNodes,
		"bandwidth_bytes":  bytesUsed,
		"bandwidth_gb":     float64(bytesUsed) / (1024 * 1024 * 1024),
	})
}

func handleUserDashboard(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(200)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	apiKey := getAPIKeyFromRequest(r)
	if apiKey == "" {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "api_key required"})
		return
	}

	var userID int
	var email, token, plan string
	var freeGB float32
	var createdAt time.Time
	err := db.QueryRow(`SELECT id, email, token, plan, free_gb_remaining, created_at FROM users WHERE api_key = $1`, apiKey).
		Scan(&userID, &email, &token, &plan, &freeGB, &createdAt)
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid api_key"})
		return
	}

	// Credit balance
	var credits, totalEarned, totalSpent, proxyGBUsed float64
	db.QueryRow(`SELECT COALESCE(credits,0), COALESCE(total_earned,0), COALESCE(total_spent,0), COALESCE(proxy_gb_used,0) FROM credit_users WHERE token = $1`, token).
		Scan(&credits, &totalEarned, &totalSpent, &proxyGBUsed)

	// Active nodes
	var activeNodes int
	hub.mu.RLock()
	for _, c := range hub.connections {
		if c.Token == token {
			activeNodes++
		}
	}
	hub.mu.RUnlock()

	// Bandwidth
	var bytesUsed int64
	db.QueryRow(`SELECT COALESCE(SUM(bytes_in + bytes_out), 0) FROM bandwidth WHERE customer = $1 AND created_at >= date_trunc('month', CURRENT_TIMESTAMP)`, apiKey).Scan(&bytesUsed)

	// Recent activity (last 10 credit_log entries)
	var activity []map[string]interface{}
	rows, err := db.Query(`SELECT amount, reason, created_at FROM credit_log WHERE user_id = $1 ORDER BY created_at DESC LIMIT 10`, token)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var amount float64
			var reason string
			var at time.Time
			if rows.Scan(&amount, &reason, &at) == nil {
				activity = append(activity, map[string]interface{}{
					"amount":     amount,
					"reason":     reason,
					"created_at": at,
				})
			}
		}
	}
	if activity == nil {
		activity = []map[string]interface{}{}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": map[string]interface{}{
			"id":               userID,
			"email":            email,
			"plan":             plan,
			"free_gb_remaining": freeGB,
			"created_at":       createdAt,
		},
		"credits": map[string]interface{}{
			"balance":       credits,
			"total_earned":  totalEarned,
			"total_spent":   totalSpent,
			"proxy_gb_used": proxyGBUsed,
		},
		"nodes": map[string]interface{}{
			"active": activeNodes,
		},
		"bandwidth": map[string]interface{}{
			"bytes_used": bytesUsed,
			"gb_used":    float64(bytesUsed) / (1024 * 1024 * 1024),
		},
		"recent_activity": activity,
	})
}

// ─── Stripe Integration ────────────────────────────────────────────────────────

const stripeSecretKey = "sk_test_51Sx6WYCqi59cXfGOVmWwgjliKRgchLOzn665bzSOIdea6QDSRsAeXMihDBp2aZdE9ahsh7tV1SclApxqskIuVtzl00kYBHyTyE"

var stripePlans = map[string]struct {
	PriceID string
	GB      int64
	Name    string
}{
	"starter":  {PriceID: "price_1T1ll8Cqi59cXfGOsPHCfQQ3", GB: 10, Name: "Starter - 10GB"},
	"growth":   {PriceID: "price_1T1ll8Cqi59cXfGOTu9HMKsM", GB: 50, Name: "Growth - 50GB"},
	"business": {PriceID: "price_1T1ll9Cqi59cXfGO1GbApLoe", GB: 200, Name: "Business - 200GB"},
}

func stripeAPI(method, endpoint string, params map[string]string) (map[string]interface{}, error) {
	data := ""
	for k, v := range params {
		if data != "" {
			data += "&"
		}
		data += k + "=" + v
	}
	req, err := http.NewRequest(method, "https://api.stripe.com/v1/"+endpoint, strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(stripeSecretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("stripe error %d: %v", resp.StatusCode, result)
	}
	return result, nil
}

func handleStripeCheckout(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(200)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	var body struct {
		APIKey string `json:"api_key"`
		Plan   string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
		return
	}
	plan, ok := stripePlans[body.Plan]
	if !ok {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid plan: starter, growth, or business"})
		return
	}
	// Verify api_key exists
	var name string
	err := db.QueryRow("SELECT name FROM customers WHERE api_key=$1", body.APIKey).Scan(&name)
	if err != nil {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid api_key"})
		return
	}
	result, err := stripeAPI("POST", "checkout/sessions", map[string]string{
		"mode":                 "payment",
		"line_items[0][price]": plan.PriceID,
		"line_items[0][quantity]": "1",
		"client_reference_id":  body.APIKey,
		"success_url":          "https://gateway.iploop.io:9443/api/stripe/success?session_id={CHECKOUT_SESSION_ID}",
		"cancel_url":           "https://gateway.iploop.io:9443/dashboard.html",
		"metadata[plan]":       body.Plan,
	})
	if err != nil {
		log.Printf("[STRIPE] checkout error: %v", err)
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": "stripe error"})
		return
	}
	url, _ := result["url"].(string)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"checkout_url": url})
}

func handleStripeSuccess(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Redirect(w, r, "/dashboard.html?error=missing_session", http.StatusFound)
		return
	}
	result, err := stripeAPI("GET", "checkout/sessions/"+sessionID, nil)
	if err != nil {
		log.Printf("[STRIPE] session retrieve error: %v", err)
		http.Redirect(w, r, "/dashboard.html?error=stripe_error", http.StatusFound)
		return
	}
	paymentStatus, _ := result["payment_status"].(string)
	if paymentStatus != "paid" {
		http.Redirect(w, r, "/dashboard.html?error=not_paid", http.StatusFound)
		return
	}
	apiKey, _ := result["client_reference_id"].(string)
	metadata, _ := result["metadata"].(map[string]interface{})
	planName, _ := metadata["plan"].(string)
	plan, ok := stripePlans[planName]
	if !ok || apiKey == "" {
		http.Redirect(w, r, "/dashboard.html?error=invalid_plan", http.StatusFound)
		return
	}
	addBytes := plan.GB * 1024 * 1024 * 1024
	_, err = db.Exec("UPDATE customers SET quota_bytes = quota_bytes + $1 WHERE api_key = $2", addBytes, apiKey)
	if err != nil {
		log.Printf("[STRIPE] DB update error: %v", err)
		http.Redirect(w, r, "/dashboard.html?error=db_error", http.StatusFound)
		return
	}
	log.Printf("[STRIPE] ✅ Added %dGB to customer %s (plan: %s)", plan.GB, apiKey, planName)
	http.Redirect(w, r, "/dashboard.html?payment=success&plan="+planName, http.StatusFound)
}

func handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	log.Printf("[STRIPE] webhook received: %s", string(body[:min(len(body), 200)]))
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Internal Relay Endpoint (cross-instance routing) ──────────────────────────

func handleInternalRelay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		NodeID string `json:"node_id"`
		Target string `json:"target"` // host:port
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if payload.NodeID == "" || payload.Target == "" {
		http.Error(w, "Missing node_id or target", http.StatusBadRequest)
		return
	}

	host, port, err := net.SplitHostPort(payload.Target)
	if err != nil {
		host = payload.Target
		port = "443"
	}

	// Verify node is on THIS instance
	conn := hub.GetConnectionByNodeID(payload.NodeID)
	if conn == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "node not found on this instance",
		})
		return
	}

	tunnel, err := hub.tunnelManager.OpenTunnel(payload.NodeID, host, port)
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
		"success":     true,
		"tunnel_id":   tunnel.ID,
		"instance_id": instanceID,
	})
}

// ─── Support API Endpoints ──────────────────────────────────────────────────────

func extractAPIKey(r *http.Request) string {
	if k := r.URL.Query().Get("api_key"); k != "" {
		return k
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func getActiveNodeCount() int {
	// Use Redis to get global count across all instances
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	total := 0
	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, "nodes:country:*", 100).Result()
		if err != nil {
			break
		}
		for _, k := range keys {
			cnt, err := rdb.SCard(ctx, k).Result()
			if err == nil {
				total += int(cnt)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	// Fallback to local hub count if Redis returned 0
	if total == 0 {
		total = hub.Count()
	}
	return total
}

func getTopCountries(n int) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	type cc struct {
		name  string
		count int64
	}
	var countries []cc
	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, "nodes:country:*", 100).Result()
		if err != nil {
			break
		}
		for _, k := range keys {
			country := k[len("nodes:country:"):]
			if country == "" || country == "unknown" {
				continue
			}
			cnt, err := rdb.SCard(ctx, k).Result()
			if err == nil && cnt > 0 {
				countries = append(countries, cc{country, cnt})
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	sort.Slice(countries, func(i, j int) bool { return countries[i].count > countries[j].count })
	result := make([]string, 0, n)
	for i := 0; i < len(countries) && i < n; i++ {
		result = append(result, strings.ToUpper(countries[i].name))
	}
	return result
}

func getAvailableCountryCount() int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	count := 0
	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, "nodes:country:*", 100).Result()
		if err != nil {
			break
		}
		for _, k := range keys {
			country := k[len("nodes:country:"):]
			if country == "" || country == "unknown" {
				continue
			}
			cnt, err := rdb.SCard(ctx, k).Result()
			if err == nil && cnt > 0 {
				count++
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return count
}

func handleSupportStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":              "operational",
		"network":             "High volume residential IP pool worldwide",
		"countries_available": 195,
		"coverage":            "High availability across all regions",
		"protocols":           []string{"HTTP CONNECT", "SOCKS5"},
		"proxy_endpoint":      "gateway.iploop.io:8880",
		"supported_targeting": []string{"country", "city", "session", "sticky"},
		"docs":                "https://docs.iploop.io",
	})
}

func handleSupportDiagnose(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	apiKey := extractAPIKey(r)
	if apiKey == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"key_valid": false,
			"error":     "API key required",
			"fix":       "Provide api_key query param or Authorization: Bearer header",
		})
		return
	}

	// Look up user by api_key
	var plan string
	var freeGBRemaining float64
	err := db.QueryRow(`SELECT COALESCE(plan, 'free'), COALESCE(free_gb_remaining, 0.5) FROM users WHERE api_key = $1`, apiKey).Scan(&plan, &freeGBRemaining)
	if err != nil {
		// Also check customers table
		cust, custErr := getCustomer(apiKey)
		if custErr != nil || cust == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"key_valid": false,
				"error":     "API key not found",
				"fix":       "Sign up at https://iploop.io/signup.html",
			})
			return
		}
		plan = "starter"
		freeGBRemaining = float64(cust.QuotaBytes) / (1024 * 1024 * 1024)
	}

	// Get quota based on plan
	var quotaGB float64
	switch plan {
	case "free":
		quotaGB = freeGBRemaining
	case "starter":
		quotaGB = 10.0
	case "pro":
		quotaGB = 100.0
	case "enterprise":
		quotaGB = 1000.0
	default:
		quotaGB = freeGBRemaining
	}

	// Get used bytes from bandwidth table
	usedBytes := getCustomerUsedBytes(apiKey)
	usedGB := float64(usedBytes) / (1024 * 1024 * 1024)
	remainingGB := quotaGB - usedGB
	if remainingGB < 0 {
		remainingGB = 0
	}
	usagePercent := 0
	if quotaGB > 0 {
		usagePercent = int((usedGB / quotaGB) * 100)
	}

	// Suggestion
	suggestion := fmt.Sprintf("Your key is healthy. %d%% quota remaining.", 100-usagePercent)
	if usagePercent > 100 {
		suggestion = "Quota exceeded. Upgrade at dashboard.iploop.io"
	} else if usagePercent > 80 {
		suggestion = fmt.Sprintf("⚠️ Warning: %d%% of quota used. Consider upgrading at dashboard.iploop.io", usagePercent)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key_valid":           true,
		"plan":                plan,
		"quota_gb":            math.Round(quotaGB*100) / 100,
		"used_gb":             math.Round(usedGB*100) / 100,
		"remaining_gb":        math.Round(remainingGB*100) / 100,
		"usage_percent":       usagePercent,
		"network":             "High volume residential IP pool available",
		"countries_available": 195,
		"best_countries":      []string{"US", "GB", "CA", "DE", "IN", "FR", "BR", "AU", "JP", "KR"},
		"suggestion":          suggestion,
	})
}

func handleSupportErrors(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	// Extract error code from path: /api/support/errors/407
	path := r.URL.Path
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	codeStr := ""
	if len(parts) > 0 {
		codeStr = parts[len(parts)-1]
	}
	code, _ := strconv.Atoi(codeStr)

	errorMap := map[int][2]string{
		407: {"Proxy Authentication Required — invalid or missing API key", "Check api_key in proxy password field. Format: user:YOUR_API_KEY-country-US"},
		403: {"Forbidden — quota exceeded or account suspended", "Check quota: GET /api/support/diagnose?api_key=YOUR_KEY"},
		502: {"Bad Gateway — no available node for requested country", "Try a different country or retry in 30 seconds"},
		503: {"Service temporarily unavailable", "System is under maintenance. Retry in a few minutes"},
		504: {"Gateway Timeout — node did not respond", "Retry the request. If persistent, try country-US for best availability"},
		429: {"Rate limit exceeded", "Reduce request frequency. Max 60 requests/min per IP"},
	}

	w.Header().Set("Content-Type", "application/json")
	if entry, ok := errorMap[code]; ok {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":        code,
			"error":       entry[0],
			"fix":         entry[1],
			"docs":        "https://docs.iploop.io",
			"diagnose":    "/api/support/diagnose?api_key=YOUR_KEY",
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    code,
			"error":   "Unknown error code",
			"fix":     "Contact support@iploop.io",
			"docs":    "https://docs.iploop.io",
		})
	}
}

func handleSupportAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	apiKey := extractAPIKey(r)
	if apiKey == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "API key required",
			"fix":   "Provide api_key query param or Authorization: Bearer header",
		})
		return
	}

	var body struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Question == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Please provide a question in JSON body: {\"question\": \"your question\"}",
		})
		return
	}

	q := strings.ToLower(body.Question)
	var answer string
	related := map[string]interface{}{}

	switch {
	case strings.Contains(q, "timeout") || strings.Contains(q, "slow") || strings.Contains(q, "latency") || strings.Contains(q, "speed") || strings.Contains(q, "performance") || strings.Contains(q, "response time"):
		answer = "For best speed: 1) Use country-US — largest pool, fastest response. 2) Use sticky sessions for multi-request flows. 3) Avoid small countries during off-peak hours. 4) Set timeout to 30s for first request. 5) If persistent timeouts, check your quota via /api/support/diagnose. Avg response time: <2s for US/EU targets."
		related["best_countries_speed"] = []string{"US", "GB", "DE", "CA", "FR"}
		related["tip"] = "US pool has highest availability 24/7"
		related["diagnose"] = "/api/support/diagnose?api_key=" + apiKey

	case strings.Contains(q, "quota") || strings.Contains(q, "limit") || strings.Contains(q, "bandwidth") || strings.Contains(q, "usage") || strings.Contains(q, "remaining") || strings.Contains(q, "how much"):
		// Get their usage
		usedBytes := getCustomerUsedBytes(apiKey)
		usedGB := float64(usedBytes) / (1024 * 1024 * 1024)
		answer = fmt.Sprintf("Your current usage this month: %.2f GB. Check your quota: GET /api/support/diagnose?api_key=YOUR_KEY. Shows plan, used GB, remaining GB, usage percent. When quota is exceeded, requests return 403. Upgrade at dashboard.iploop.io or earn credits by running a Docker node.", usedGB)
		related["check"] = "/api/support/diagnose?api_key=" + apiKey
		related["earn"] = "docker run -d ultronloop2026/iploop-node:latest"
		related["upgrade"] = "https://dashboard.iploop.io"

	case strings.Contains(q, "how many") || strings.Contains(q, "pool size") || strings.Contains(q, "ips"):
		answer = "Our network spans high volume residential IPs across 195+ countries with high availability in all major regions."
		related["docs"] = "https://docs.iploop.io"

	case strings.Contains(q, "countr") || strings.Contains(q, "geo") || strings.Contains(q, "location"):
		answer = "IPLoop covers 195+ countries with high volume residential IPs. High availability in US, UK, Canada, Germany, India, France, Brazil, Australia, Japan, South Korea and many more."
		related["example"] = "user:YOUR_API_KEY-country-DE"
		related["docs"] = "https://docs.iploop.io"

	case strings.Contains(q, "price") || strings.Contains(q, "plan") || strings.Contains(q, "upgrade"):
		answer = "Plans: Free (0.5 GB), Starter ($49/mo, 10 GB), Pro ($199/mo, 100 GB), Enterprise (custom). Upgrade at dashboard.iploop.io"
		related["pricing"] = "https://iploop.io/#pricing"
		related["dashboard"] = "https://dashboard.iploop.io"

	case strings.Contains(q, "error") || strings.Contains(q, "fail"):
		answer = "Run a diagnostic check on your API key to identify the issue. Common errors: 407 (bad auth), 403 (quota exceeded), 502 (no nodes for country)."
		related["diagnose"] = "/api/support/diagnose?api_key=" + apiKey
		related["errors"] = "/api/support/errors/{code}"

	case strings.Contains(q, "captcha") || strings.Contains(q, "recaptcha") || strings.Contains(q, "hcaptcha") || strings.Contains(q, "challenge"):
		answer = "IPLoop provides clean residential IPs to minimize CAPTCHAs. For CAPTCHA solving, we recommend integrating a third-party solver alongside our proxy: 2Captcha (2captcha.com), Anti-Captcha (anti-captcha.com), CapMonster (capmonster.cloud), or CapsOver (capsolver.com). Use sticky sessions (-session-ID) to maintain the same IP during CAPTCHA solving."
		related["tip"] = "Use -session-ID in your proxy password for consistent IP during CAPTCHA flow"
		related["recommended_solvers"] = []string{"2captcha.com", "anti-captcha.com", "capmonster.cloud", "capsolver.com"}

	case strings.Contains(q, "block") || strings.Contains(q, "blocked") || strings.Contains(q, "ban") || strings.Contains(q, "banned") || strings.Contains(q, "detect") || strings.Contains(q, "fingerprint"):
		answer = "To reduce blocks: 1) Use sticky sessions for consistent IP identity. 2) Rotate user-agents matching your target country. 3) Use city-level targeting for local IP appearance. 4) Add delays between requests (2-5s). 5) For browser automation, use residential IPs with tools like Puppeteer/Playwright with stealth plugins."
		related["tip"] = "Format: user:KEY-country-US-city-miami-session-myid@gateway.iploop.io:8880"
		related["tools"] = []string{"puppeteer-extra-plugin-stealth", "playwright-stealth"}

	case strings.Contains(q, "browser") || strings.Contains(q, "headless") || strings.Contains(q, "puppeteer") || strings.Contains(q, "playwright") || strings.Contains(q, "selenium"):
		answer = "IPLoop works great with browser automation. For Puppeteer/Playwright, set the proxy in launch options. Use sticky sessions for multi-page flows. Combine with stealth plugins to reduce detection. Example: --proxy-server=http://gateway.iploop.io:8880 with auth user:KEY-country-US-session-browse1"
		related["example_puppeteer"] = "puppeteer.launch({args: ['--proxy-server=http://gateway.iploop.io:8880']})"
		related["stealth"] = "npm i puppeteer-extra-plugin-stealth"

	case strings.Contains(q, "serp") || strings.Contains(q, "google") || strings.Contains(q, "search engine") || strings.Contains(q, "bing") || strings.Contains(q, "search results"):
		answer = "For SERP scraping through IPLoop: 1) Use country-targeted IPs matching your search locale (country-US for google.com). 2) Use sticky sessions per search session (-session-google1). 3) Rotate IPs between searches, not during. 4) Set realistic headers (Accept-Language matching country). 5) Add 2-5s random delays between requests. 6) For Google specifically: use city-level targeting for localized results. We recommend Puppeteer/Playwright with stealth plugin for JS-rendered SERPs."
		related["example"] = "user:KEY-country-US-city-newyork-session-serp001@gateway.iploop.io:8880"
		related["tools"] = []string{"puppeteer-extra-plugin-stealth", "cheerio for parsing"}
		related["tip"] = "Rotate session ID per search, keep same session for pagination"

	case strings.Contains(q, "scale") || strings.Contains(q, "scaling") || strings.Contains(q, "high volume") || strings.Contains(q, "100k") || strings.Contains(q, "10k") || strings.Contains(q, "1000") || strings.Contains(q, "concurrent") || strings.Contains(q, "parallel") || strings.Contains(q, "bulk"):
		answer = "Scaling best practices with IPLoop: 1) Use rotating sessions (new session ID per request) for maximum IP diversity. 2) Spread requests across multiple countries when possible. 3) For 1K+/hour: use connection pooling, keep-alive, and async requests. 4) For 10K+/hour: distribute across multiple API keys or contact us for Enterprise rate limits. 5) For 100K+/day: use our Enterprise plan with dedicated support. Monitor your usage via /api/support/diagnose."
		related["rate_limits"] = map[string]string{"free": "30 req/min", "starter": "120 req/min", "growth": "300 req/min", "business": "600 req/min", "enterprise": "custom"}
		related["tip"] = "Enterprise plan includes dedicated account manager and custom rate limits"

	case strings.Contains(q, "data") || strings.Contains(q, "pipeline") || strings.Contains(q, "etl") || strings.Contains(q, "scrape") || strings.Contains(q, "scraping") || strings.Contains(q, "crawl") || strings.Contains(q, "crawling") || strings.Contains(q, "extract"):
		answer = "Building a data pipeline with IPLoop: 1) Proxy layer: use our HTTP CONNECT proxy with country targeting. 2) Browser layer: Puppeteer/Playwright for JS-rendered pages, or raw HTTP for static content. 3) Parser layer: Cheerio/BeautifulSoup for HTML, or use readability for article extraction. 4) Storage: stream results to your DB/S3. 5) Use sticky sessions for multi-page crawls (login flows, pagination). 6) Monitor quota via /api/support/diagnose. We handle the IP infrastructure — you build the logic."
		related["stack"] = map[string]string{"proxy": "IPLoop HTTP CONNECT", "browser": "Puppeteer + stealth", "parser": "Cheerio / BeautifulSoup", "scheduler": "Bull / Celery"}
		related["tip"] = "Use session IDs to maintain state across paginated crawls"

	case strings.Contains(q, "rotate") || strings.Contains(q, "rotation") || strings.Contains(q, "ip rotation") || strings.Contains(q, "new ip"):
		answer = "IP rotation with IPLoop: 1) Auto-rotate: every new request without a session ID gets a fresh IP. 2) Sticky sessions: add -session-MYID to keep the same IP for multiple requests. 3) Timed rotation: create new session IDs on your schedule (per minute, per page, etc). 4) Per-country rotation: combine country targeting with rotation for geo-specific pools. Best practice: rotate between searches, keep same IP during multi-step flows."
		related["formats"] = map[string]string{"auto_rotate": "user:KEY-country-US@gateway.iploop.io:8880", "sticky": "user:KEY-country-US-session-abc@gateway.iploop.io:8880"}
		related["tip"] = "Session IDs can be any string — use descriptive names like 'google-search-1'"

	case strings.Contains(q, "auth") || strings.Contains(q, "authentication") || strings.Contains(q, "login") || strings.Contains(q, "password") || strings.Contains(q, "credentials") || strings.Contains(q, "proxy auth"):
		answer = "Proxy authentication format: http://user:YOUR_API_KEY@gateway.iploop.io:8880. Your API key goes in the password field. Username can be anything. Add targeting: user:KEY-country-US-session-abc@gateway.iploop.io:8880. Get your key at iploop.io/signup.html"
		related["format"] = "http://user:API_KEY-country-XX-session-ID@gateway.iploop.io:8880"

	case strings.Contains(q, "setup") || strings.Contains(q, "install") || strings.Contains(q, "getting started") || strings.Contains(q, "how to use") || strings.Contains(q, "quickstart") || strings.Contains(q, "tutorial"):
		answer = "Quick start: 1) Sign up at iploop.io/signup.html — get your API key instantly. 2) Test: curl -x http://user:YOUR_KEY@gateway.iploop.io:8880 https://httpbin.org/ip. 3) Add country: curl -x http://user:YOUR_KEY-country-US@gateway.iploop.io:8880 https://httpbin.org/ip. 4) Check quota: GET /api/support/diagnose?api_key=YOUR_KEY. Done in 30 seconds."
		related["signup"] = "https://iploop.io/signup.html"
		related["docs"] = "https://github.com/iploop/iploop-node/blob/main/docs/SUPPORT-API.md"

	case strings.Contains(q, "connect") || strings.Contains(q, "connection") || strings.Contains(q, "refused") || strings.Contains(q, "reset") || strings.Contains(q, "closed") || strings.Contains(q, "socket") || strings.Contains(q, "tcp"):
		answer = "Connection issues: 1) Verify your API key is valid: GET /api/support/diagnose?api_key=YOUR_KEY. 2) Check proxy format: http://user:KEY@gateway.iploop.io:8880. 3) Port 8880 for HTTP CONNECT. 4) If connection refused: check your firewall allows outbound to port 8880. 5) If connection reset: retry — a different node will be selected. 6) For persistent issues: contact support@iploop.io"
		related["ports"] = map[string]int{"http_connect": 8880, "api": 9443}

	case strings.Contains(q, "dns") || strings.Contains(q, "resolve") || strings.Contains(q, "domain"):
		answer = "DNS resolution happens on the exit node (residential IP), not on your machine. This means target sites see DNS queries from residential IPs. If you need specific DNS: use SOCKS5 proxy mode. For DNS leaks: ensure your client is configured to resolve through the proxy."
		related["tip"] = "HTTP CONNECT proxies resolve DNS on the exit node by default"

	case strings.Contains(q, "bill") || strings.Contains(q, "billing") || strings.Contains(q, "invoice") || strings.Contains(q, "charge") || strings.Contains(q, "payment") || strings.Contains(q, "pay") || strings.Contains(q, "credit card") || strings.Contains(q, "refund"):
		answer = "Billing info: Manage your subscription at dashboard.iploop.io. Plans: Free (0.5GB), Starter $10/10GB, Growth $40/50GB, Business $120/200GB, Enterprise custom. Upgrade anytime — prorated. Cancel anytime. For billing issues: support@iploop.io"
		related["dashboard"] = "https://dashboard.iploop.io"
		related["plans"] = map[string]string{"free": "0.5GB", "starter": "10GB/$10", "growth": "50GB/$40", "business": "200GB/$120"}

	case strings.Contains(q, "cancel") || strings.Contains(q, "downgrade") || strings.Contains(q, "unsubscribe"):
		answer = "To cancel or downgrade: visit dashboard.iploop.io or contact support@iploop.io. Cancellation is immediate, no lock-in. Remaining quota stays active until end of billing period. You can always use the Free tier (0.5GB/month) after cancellation."
		related["support"] = "support@iploop.io"

	case strings.Contains(q, "python") || strings.Contains(q, "requests") || strings.Contains(q, "aiohttp") || strings.Contains(q, "httpx"):
		answer = "Python integration: proxies = {'http': 'http://user:KEY-country-US@gateway.iploop.io:8880', 'https': 'http://user:KEY-country-US@gateway.iploop.io:8880'}. Works with requests, aiohttp, httpx, scrapy. For async: use aiohttp with proxy parameter. For Scrapy: set HTTPPROXY_AUTH_ENCODING and proxy middleware."
		related["requests"] = "requests.get(url, proxies=proxies)"
		related["aiohttp"] = "session.get(url, proxy='http://user:KEY@gateway.iploop.io:8880')"
		related["scrapy"] = "DOWNLOADER_MIDDLEWARES HttpProxyMiddleware"

	case strings.Contains(q, "nodejs") || strings.Contains(q, "javascript") || strings.Contains(q, "axios") || strings.Contains(q, "fetch") || strings.Contains(q, "got"):
		answer = "Node.js integration: Use axios with proxy config, node-fetch with agent, or got with proxy option. For axios: {proxy: {host: 'gateway.iploop.io', port: 8880, auth: {username: 'user', password: 'KEY-country-US'}}}. For undici/fetch: use ProxyAgent."
		related["axios_example"] = "axios.get(url, {proxy: {host: 'gateway.iploop.io', port: 8880, auth: {username: 'user', password: 'KEY'}}})"
		related["packages"] = []string{"axios", "proxy-agent", "node-fetch"}

	case strings.Contains(q, "java") || strings.Contains(q, "kotlin") || strings.Contains(q, "android"):
		answer = "Java/Android integration: Use java.net.Proxy with InetSocketAddress('gateway.iploop.io', 8880). Set Authenticator for proxy auth. For OkHttp: use .proxy() and .proxyAuthenticator(). For Android SDK integration: contact partners@iploop.io"
		related["okhttp"] = "new OkHttpClient.Builder().proxy(proxy).proxyAuthenticator(auth).build()"

	case strings.Contains(q, "curl") || strings.Contains(q, "wget") || strings.Contains(q, "cli") || strings.Contains(q, "command line") || strings.Contains(q, "terminal"):
		answer = "CLI usage: curl -x http://user:KEY-country-US@gateway.iploop.io:8880 https://target.com. For wget: use -e use_proxy=yes -e http_proxy=... For environment variables: export HTTP_PROXY=http://user:KEY@gateway.iploop.io:8880 and HTTPS_PROXY same."
		related["curl"] = "curl -x http://user:KEY@gateway.iploop.io:8880 URL"
		related["env"] = "export HTTPS_PROXY=http://user:KEY@gateway.iploop.io:8880"

	case strings.Contains(q, "socks") || strings.Contains(q, "socks5") || strings.Contains(q, "socks4"):
		answer = "SOCKS5 supported on gateway.iploop.io:8880. Format: socks5://user:KEY-country-US@gateway.iploop.io:8880. Works with curl --socks5, Python socks, and all major libraries. SOCKS5 supports UDP and DNS resolution on exit node."
		related["curl"] = "curl --socks5 user:KEY@gateway.iploop.io:8880 URL"
		related["python"] = "pip install pysocks"

	case strings.Contains(q, "ecommerce") || strings.Contains(q, "amazon") || strings.Contains(q, "ebay") || strings.Contains(q, "shopify") || strings.Contains(q, "price monitoring"):
		answer = "For e-commerce scraping: 1) Use country-targeted IPs matching the marketplace locale. 2) Sticky sessions for browsing flows (search → product → details). 3) Rotate IPs between products. 4) Use residential IPs to avoid marketplace blocks. 5) City targeting for local pricing. Best practice: 3-5s delays between page loads."
		related["tip"] = "Use session IDs per product browsing flow"
		related["recommended_delay"] = "3-5 seconds"

	case strings.Contains(q, "social") || strings.Contains(q, "instagram") || strings.Contains(q, "facebook") || strings.Contains(q, "twitter") || strings.Contains(q, "tiktok") || strings.Contains(q, "linkedin") || strings.Contains(q, "social media"):
		answer = "For social media: 1) Use sticky sessions — social platforms track IP consistency. 2) Match country to account locale. 3) Use city targeting for location-based content. 4) Residential IPs essential — datacenter IPs are instantly blocked. 5) Rate limit yourself: 1-2 requests per 5-10 seconds. 6) Rotate accounts across different sticky sessions."
		related["warning"] = "Respect platform ToS. Use for legitimate research only."
		related["tip"] = "One sticky session per account"

	case strings.Contains(q, "seo") || strings.Contains(q, "rank") || strings.Contains(q, "ranking") || strings.Contains(q, "keyword") || strings.Contains(q, "backlink"):
		answer = "For SEO monitoring: 1) Use country+city targeting for localized SERP results. 2) Sticky sessions per search query. 3) Rotate IPs between different keywords. 4) Support for Google, Bing, Yahoo, Yandex, Baidu. 5) Use headless browser for accurate rendering. Format: user:KEY-country-US-city-newyork-session-seo1@gateway.iploop.io:8880"
		related["engines"] = []string{"Google", "Bing", "Yahoo", "Yandex", "Baidu"}
		related["tip"] = "City targeting gives localized SERP results"

	case strings.Contains(q, "ad verification") || strings.Contains(q, "ads") || strings.Contains(q, "verification") || strings.Contains(q, "creative"):
		answer = "For ad verification: 1) Use city-level targeting to check ads in specific geolocations. 2) Sticky sessions to maintain advertiser view. 3) Residential IPs see real ads — datacenter IPs get filtered. 4) Support all major ad networks. 5) Rotate across cities to verify geo-targeting compliance."
		related["tip"] = "City targeting is key for accurate ad verification"

	case strings.Contains(q, "ticket") || strings.Contains(q, "sneaker") || strings.Contains(q, "drop") || strings.Contains(q, "bot") || strings.Contains(q, "checkout") || strings.Contains(q, "nike") || strings.Contains(q, "supreme") || strings.Contains(q, "retail"):
		answer = "For retail/ticket bots: 1) Use sticky sessions per checkout flow. 2) US residential IPs for US drops. 3) City targeting near fulfillment centers. 4) Pre-warm sessions before drop time. 5) High-speed rotation between attempts. Growth or Business plan recommended for rate limits. Format: user:KEY-country-US-city-losangeles-session-drop001@gateway.iploop.io:8880"
		related["recommended_plan"] = "Growth or Business for higher rate limits"
		related["tip"] = "Create sticky sessions 5-10min before drop"

	case strings.Contains(q, "market") || strings.Contains(q, "research") || strings.Contains(q, "competitive") || strings.Contains(q, "intelligence") || strings.Contains(q, "competitor"):
		answer = "For market research: 1) Multi-country targeting to compare pricing, content, availability across regions. 2) Sticky sessions for consistent browsing. 3) City targeting for local market data. 4) Use /api/support/diagnose to monitor usage. 5) Combine with headless browser for JS-rendered content. Enterprise plan available for high-volume research."
		related["tip"] = "Target specific cities for hyper-local market data"

	case strings.Contains(q, "api") || strings.Contains(q, "documentation") || strings.Contains(q, "docs") || strings.Contains(q, "help") || strings.Contains(q, "support"):
		answer = "Full API documentation: https://github.com/iploop/iploop-node/blob/main/docs/SUPPORT-API.md. Endpoints: /api/support/status, /api/support/diagnose, /api/support/errors/{code}, /api/support/ask. For account help: support@iploop.io. Dashboard: dashboard.iploop.io"
		related["docs"] = "https://github.com/iploop/iploop-node/blob/main/docs/SUPPORT-API.md"
		related["support"] = "support@iploop.io"

	default:
		answer = "Visit docs.iploop.io for full documentation or contact support@iploop.io for personalized help."
		related["docs"] = "https://docs.iploop.io"
		related["support"] = "support@iploop.io"
		related["diagnose"] = "/api/support/diagnose?api_key=" + apiKey
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"answer":  answer,
		"related": related,
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	initDB()
	defer db.Close()

	// Initialize Redis for node registry
	initRedis()

	// Initialize tunnel and proxy managers
	hub.tunnelManager = NewTunnelManager(hub)
	hub.proxyManager = NewProxyManager(hub)

	// Periodic credit update — real-time balance for active sessions
	go creditPeriodicUpdate()
	go geoCacheCleanup()

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
			db.QueryRow(`SELECT COUNT(*) FROM ip_info WHERE updated_at >= NOW() - INTERVAL '1 hour'`).Scan(&unique1h)

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
			db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type='tunnel' AND created_at >= NOW() - INTERVAL '1 hour'`).Scan(&tunnelReqs1h)
			db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type='tunnel' AND created_at >= NOW() - INTERVAL '1 hour' AND success=1`).Scan(&tunnelSuccess1h)
			db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type='proxy' AND created_at >= NOW() - INTERVAL '1 hour'`).Scan(&proxyReqs1h)
			db.QueryRow(`SELECT COUNT(*) FROM requests WHERE type='proxy' AND created_at >= NOW() - INTERVAL '1 hour' AND success=1`).Scan(&proxySuccess1h)

			_, err := db.Exec(`INSERT INTO snapshots (active_nodes, unique_nodes_1h, total_countries,
				top_country, top_country_nodes, tunnel_requests_1h, tunnel_success_1h,
				proxy_requests_1h, proxy_success_1h, sdk_versions)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
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

	// API: Tunnel endpoints (rate limited)
	http.HandleFunc("/api/tunnel/open", rateLimitAPI(handleAPITunnelOpen))
	http.HandleFunc("/api/tunnel/data", rateLimitAPI(handleAPITunnelData))
	http.HandleFunc("/api/tunnel/close", rateLimitAPI(handleAPITunnelClose))
	http.HandleFunc("/api/tunnel/ws", rateLimitAPI(handleAPITunnelWS))
	http.HandleFunc("/api/tunnel/standby", rateLimitAPI(handleAPITunnelStandby))

	// API: Proxy endpoint (rate limited)
	http.HandleFunc("/api/proxy", rateLimitAPI(handleAPIProxy))

	// API: Nodes endpoints (rate limited)
	http.HandleFunc("/api/nodes", rateLimitAPI(handleAPINodes))
	http.HandleFunc("/api/nodes/", rateLimitAPI(handleAPINodeByID))

	// API: Node quality scores (rate limited)
	http.HandleFunc("/api/node-scores", rateLimitAPI(handleAPINodeScores))
	http.HandleFunc("/api/bandwidth", rateLimitAPI(handleAPIBandwidth))
	http.HandleFunc("/api/credits", rateLimitAPI(handleAPICredits))
	http.HandleFunc("/api/customers", rateLimitAPI(handleAPICustomers))

	// Auth endpoints (rate limited)
	http.HandleFunc("/api/auth/signup", rateLimitAPI(handleAuthSignup))
	http.HandleFunc("/api/auth/login", rateLimitAPI(handleAuthLogin))
	http.HandleFunc("/api/auth/me", rateLimitAPI(handleAuthMe))
	http.HandleFunc("/api/user/dashboard", rateLimitAPI(handleUserDashboard))
	http.HandleFunc("/api/referral", rateLimitAPI(handleReferralInfo))
	http.HandleFunc("/api/referral/stats", rateLimitAPI(handleReferralStats))

	// Support API endpoints
	http.HandleFunc("/api/support/status", rateLimitAPI(handleSupportStatus))
	http.HandleFunc("/api/support/diagnose", rateLimitAPI(handleSupportDiagnose))
	http.HandleFunc("/api/support/errors/", rateLimitAPI(handleSupportErrors))
	http.HandleFunc("/api/support/ask", rateLimitAPI(handleSupportAsk))

	// Internal cross-instance relay
	http.HandleFunc("/internal/relay", rateLimitAPI(handleInternalRelay))

	// Stripe endpoints (rate limited)
	http.HandleFunc("/api/stripe/checkout", rateLimitAPI(handleStripeCheckout))
	http.HandleFunc("/api/stripe/success", rateLimitAPI(handleStripeSuccess))
	http.HandleFunc("/api/stripe/webhook", rateLimitAPI(handleStripeWebhook))

	// Serve dashboard.html
	http.HandleFunc("/dashboard.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/root/dashboard.html")
	})

	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	// Start HTTP CONNECT proxy on port 8880
	go startHTTPProxy("8880")

	// Slowloris protection via timeouts
	srv := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if tlsCert != "" && tlsKey != "" {
		log.Printf("IPLoop Node Server starting on :%s (TLS)", port)
		log.Printf("  WebSocket: wss://0.0.0.0:%s/ws", port)
		log.Printf("  Stats:     https://0.0.0.0:%s/stats", port)
		log.Printf("  API:       https://0.0.0.0:%s/api/...", port)
		log.Printf("  Ping interval: %v, Pong timeout: %v", pingInterval, pongWait)
		log.Printf("  Protection: maxWSPerIP=%d, maxTotalWS=%d, apiRate=%d/min", maxWSPerIP, maxTotalWS, apiRateLimitN)
		if err := srv.ListenAndServeTLS(tlsCert, tlsKey); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("IPLoop Node Server starting on :%s", port)
		log.Printf("  WebSocket: ws://0.0.0.0:%s/ws", port)
		log.Printf("  Stats:     http://0.0.0.0:%s/stats", port)
		log.Printf("  API:       http://0.0.0.0:%s/api/...", port)
		log.Printf("  Ping interval: %v, Pong timeout: %v", pingInterval, pongWait)
		log.Printf("  Protection: maxWSPerIP=%d, maxTotalWS=%d, apiRate=%d/min", maxWSPerIP, maxTotalWS, apiRateLimitN)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}
}
