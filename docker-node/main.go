package main

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const (
	version     = "1.0.0"
	gatewayURL  = "wss://gateway.iploop.io:9443/ws"
	ipInfoURL   = "http://ip-api.com/json/"
	pingPeriod  = 4*time.Minute + 30*time.Second
	maxTunnels  = 32
)

// ─── Types ─────────────────────────────────────────────────────────────────────

type IPInfo struct {
	IP          string  `json:"query"`
	Country     string  `json:"countryCode"`
	CountryName string  `json:"country"`
	City        string  `json:"city"`
	Region      string  `json:"regionName"`
	ISP         string  `json:"isp"`
	ASN         string  `json:"as"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

type TunnelOpen struct {
	TunnelID string `json:"tunnel_id"`
	Host     string `json:"host"`
	Port     string `json:"port"`
}

type NodeAgent struct {
	nodeID   string
	token    string
	gateway  string
	conn     *websocket.Conn
	writeMu  sync.Mutex
	tunnels  sync.Map // tunnel_id -> net.Conn
	done     chan struct{}
}

// ─── Main ──────────────────────────────────────────────────────────────────────

func main() {
	token := flag.String("token", os.Getenv("IPLOOP_TOKEN"), "Node authentication token")
	gateway := flag.String("gateway", gatewayURL, "Gateway WebSocket URL")
	flag.Parse()

	if *token == "" {
		log.Fatal("Token required: use --token or set IPLOOP_TOKEN")
	}

	agent := &NodeAgent{
		nodeID:  generateNodeID(*token),
		token:   *token,
		gateway: *gateway,
		done:    make(chan struct{}),
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[NODE] Shutting down...")
		close(agent.done)
	}()

	log.Printf("[NODE] IPLoop Docker Node v%s", version)
	log.Printf("[NODE] ID: %s", agent.nodeID)
	agent.runForever()
}

// ─── Connection Loop (never gives up) ──────────────────────────────────────────

func (a *NodeAgent) runForever() {
	attempt := 0
	for {
		select {
		case <-a.done:
			return
		default:
		}

		err := a.connect()
		if err != nil {
			attempt++
			delay := backoff(attempt)
			log.Printf("[NODE] Connection failed (%v), retry in %v", err, delay)
			select {
			case <-time.After(delay):
			case <-a.done:
				return
			}
		} else {
			attempt = 0
		}
	}
}

func backoff(attempt int) time.Duration {
	if attempt <= 15 {
		// Exponential: 1s, 2s, 4s... capped at 30s
		d := time.Duration(1<<uint(attempt-1)) * time.Second
		if d > 30*time.Second {
			d = 30 * time.Second
		}
		return d
	}
	return 10 * time.Minute // After 15 attempts, 10min intervals forever
}

// ─── Connect & Run ─────────────────────────────────────────────────────────────

func (a *NodeAgent) connect() error {
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		HandshakeTimeout: 15 * time.Second,
	}

	conn, _, err := dialer.Dial(a.gateway, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	a.conn = conn
	defer func() {
		conn.Close()
		a.conn = nil
	}()

	// TCP tuning
	if nc := conn.UnderlyingConn(); nc != nil {
		if tc, ok := nc.(*net.TCPConn); ok {
			tc.SetNoDelay(true)
			tc.SetReadBuffer(65536)
			tc.SetWriteBuffer(65536)
		}
	}

	// Send hello
	hello, _ := json.Marshal(map[string]interface{}{
		"type":         "hello",
		"node_id":      a.nodeID,
		"device_model": dockerDeviceModel(),
		"sdk_version":  "docker-" + version,
		"os":           "docker",
		"token":        a.token,
	})
	if err := a.safeWrite(websocket.TextMessage, hello); err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	// Read welcome
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("welcome: %w", err)
	}
	var welcome map[string]interface{}
	json.Unmarshal(msg, &welcome)
	if welcome["type"] == "cooldown" {
		sec, _ := welcome["retry_after_sec"].(float64)
		return fmt.Errorf("cooldown: retry after %vs", sec)
	}
	log.Printf("[NODE] Connected to gateway")

	// Send registration + IP info
	go a.registerAndSendIPInfo()

	// Pong handler
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pingPeriod + 45*time.Second))
		return nil
	})

	// Ping ticker
	pingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.writeMu.Lock()
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				a.writeMu.Unlock()
				if err != nil {
					return
				}
			case <-pingDone:
				return
			case <-a.done:
				return
			}
		}
	}()
	defer close(pingDone)

	// Keepalive ticker (every 5 min)
	kaliveDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ka, _ := json.Marshal(map[string]string{"type": "keepalive"})
				a.safeWrite(websocket.TextMessage, ka)
			case <-kaliveDone:
				return
			case <-a.done:
				return
			}
		}
	}()
	defer close(kaliveDone)

	// Read loop
	conn.SetReadDeadline(time.Now().Add(pingPeriod + 45*time.Second))
	for {
		select {
		case <-a.done:
			return nil
		default:
		}

		msgType, rawMsg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		conn.SetReadDeadline(time.Now().Add(pingPeriod + 45*time.Second))

		if msgType == websocket.BinaryMessage {
			a.handleBinaryTunnelData(rawMsg)
			continue
		}

		var m map[string]interface{}
		if json.Unmarshal(rawMsg, &m) != nil {
			continue
		}

		switch m["type"] {
		case "tunnel_open":
			dataBytes, _ := json.Marshal(m["data"])
			var req TunnelOpen
			if json.Unmarshal(dataBytes, &req) == nil {
				go a.handleTunnelOpen(req)
			}
		case "keepalive_ack":
			// OK
		case "heartbeat_ack":
			// OK
		}
	}
}

// ─── Tunnel Handling ───────────────────────────────────────────────────────────

func (a *NodeAgent) handleTunnelOpen(req TunnelOpen) {
	target := net.JoinHostPort(req.Host, req.Port)
	
	tcpConn, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		resp, _ := json.Marshal(map[string]interface{}{
			"type": "tunnel_response",
			"data": map[string]interface{}{
				"tunnel_id": req.TunnelID,
				"success":   false,
				"error":     err.Error(),
			},
		})
		a.safeWrite(websocket.TextMessage, resp)
		return
	}

	a.tunnels.Store(req.TunnelID, tcpConn)

	// Send success
	resp, _ := json.Marshal(map[string]interface{}{
		"type": "tunnel_response",
		"data": map[string]interface{}{
			"tunnel_id": req.TunnelID,
			"success":   true,
		},
	})
	a.safeWrite(websocket.TextMessage, resp)

	// Read from TCP → send to gateway as binary tunnel data
	go func() {
		defer func() {
			tcpConn.Close()
			a.tunnels.Delete(req.TunnelID)
		}()

		tunnelIDBytes := []byte(req.TunnelID)
		// Pad/truncate to 36 bytes
		idBuf := make([]byte, 36)
		copy(idBuf, tunnelIDBytes)

		buf := make([]byte, 32768)
		for {
			n, err := tcpConn.Read(buf)
			if n > 0 {
				// Binary frame: [36B tunnel_id][1B flags=0x00][payload]
				frame := make([]byte, 37+n)
				copy(frame[:36], idBuf)
				frame[36] = 0x00 // data flag
				copy(frame[37:], buf[:n])
				a.safeWrite(websocket.BinaryMessage, frame)
			}
			if err != nil {
				// Send EOF
				eofFrame := make([]byte, 37)
				copy(eofFrame[:36], idBuf)
				eofFrame[36] = 0x01 // EOF flag
				a.safeWrite(websocket.BinaryMessage, eofFrame)
				return
			}
		}
	}()
}

func (a *NodeAgent) handleBinaryTunnelData(raw []byte) {
	if len(raw) < 37 {
		return
	}

	tunnelID := strings.TrimRight(string(raw[:36]), "\x00")
	flags := raw[36]
	payload := raw[37:]

	val, ok := a.tunnels.Load(tunnelID)
	if !ok {
		return
	}
	tcpConn := val.(net.Conn)

	if flags == 0x01 { // EOF
		tcpConn.Close()
		a.tunnels.Delete(tunnelID)
		return
	}

	if len(payload) > 0 {
		tcpConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		tcpConn.Write(payload)
	}
}

// ─── IP Info ───────────────────────────────────────────────────────────────────

func (a *NodeAgent) registerAndSendIPInfo() {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ipInfoURL)
	if err != nil {
		log.Printf("[NODE] IP info fetch failed: %v", err)
		return
	}
	defer resp.Body.Close()

	var info IPInfo
	if json.NewDecoder(resp.Body).Decode(&info) != nil {
		return
	}

	msg, _ := json.Marshal(map[string]interface{}{
		"type": "ip_info",
		"ip_info": map[string]interface{}{
			"countryCode": info.Country,
			"country":     info.CountryName,
			"city":        info.City,
			"regionName":  info.Region,
			"isp":         info.ISP,
			"as":          info.ASN,
			"lat":         info.Lat,
			"lon":         info.Lon,
		},
	})
	a.safeWrite(websocket.TextMessage, msg)
	log.Printf("[NODE] IP: %s (%s, %s)", info.IP, info.Country, info.City)

	// Send register message (data wrapped for server parser)
	reg, _ := json.Marshal(map[string]interface{}{
		"type": "register",
		"data": map[string]interface{}{
			"device_id":       a.nodeID,
			"ip_address":      info.IP,
			"country":         info.Country,
			"country_name":    info.CountryName,
			"city":            info.City,
			"region":          info.Region,
			"isp":             info.ISP,
			"asn":             0,
			"connection_type": "wired",
			"device_type":     "docker",
			"sdk_version":     "2.0.0",
		},
	})
	a.safeWrite(websocket.TextMessage, reg)
	log.Printf("[NODE] Registered")
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

func (a *NodeAgent) safeWrite(msgType int, data []byte) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if a.conn == nil {
		return fmt.Errorf("not connected")
	}
	a.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return a.conn.WriteMessage(msgType, data)
}

func generateNodeID(token string) string {
	// Deterministic node ID from token + hostname
	hostname, _ := os.Hostname()
	seed := token + ":" + hostname
	h := uint64(0)
	for _, c := range seed {
		h = h*31 + uint64(c)
	}
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, h)
	return fmt.Sprintf("docker_%x%x%x%x%x",
		b[0:2], b[2:4], b[4:5], b[5:6], b[6:8])
}

func dockerDeviceModel() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("Docker/%s/%s (%s)", runtime.GOOS, runtime.GOARCH, hostname)
}

// Unused but kept for potential future use
var _ = rand.Intn
