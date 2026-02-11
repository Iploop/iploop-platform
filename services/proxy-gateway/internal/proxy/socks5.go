package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/armon/go-socks5"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"proxy-gateway/internal/auth"
	"proxy-gateway/internal/nodepool"
	"proxy-gateway/internal/metrics"
)

type SOCKS5Proxy struct {
	authenticator *auth.Authenticator
	nodePool      *nodepool.NodePool
	wsNodePool    *nodepool.WebSocketNodePool
	metrics       *metrics.Collector
	logger        *logrus.Entry
	server        *socks5.Server
	nodeRegURL    string
}

// SOCKS5WSConn wraps a WebSocket connection to implement net.Conn for SOCKS5
type SOCKS5WSConn struct {
	ws       *websocket.Conn
	readBuf  []byte
	readPos  int
	logger   *logrus.Entry
}

func (c *SOCKS5WSConn) Read(b []byte) (int, error) {
	// Return buffered data first
	if c.readPos < len(c.readBuf) {
		n := copy(b, c.readBuf[c.readPos:])
		c.readPos += n
		if c.readPos >= len(c.readBuf) {
			c.readBuf = nil
			c.readPos = 0
		}
		return n, nil
	}

	// Read next WebSocket message
	c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	messageType, data, err := c.ws.ReadMessage()
	if err != nil {
		// Handle WebSocket close as EOF (normal end of stream)
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return 0, io.EOF
		}
		// Also handle unexpected close errors as EOF
		if strings.Contains(err.Error(), "close") {
			return 0, io.EOF
		}
		return 0, err
	}

	// Handle close message type
	if messageType == websocket.CloseMessage {
		return 0, io.EOF
	}

	if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
		return 0, fmt.Errorf("unexpected message type: %d", messageType)
	}

	n := copy(b, data)
	if n < len(data) {
		c.readBuf = data[n:]
		c.readPos = 0
	}
	return n, nil
}

func (c *SOCKS5WSConn) Write(b []byte) (int, error) {
	c.ws.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := c.ws.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *SOCKS5WSConn) Close() error {
	c.ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	return c.ws.Close()
}

func (c *SOCKS5WSConn) LocalAddr() net.Addr  { return c.ws.LocalAddr() }
func (c *SOCKS5WSConn) RemoteAddr() net.Addr { return c.ws.RemoteAddr() }

func (c *SOCKS5WSConn) SetDeadline(t time.Time) error {
	if err := c.ws.SetReadDeadline(t); err != nil {
		return err
	}
	return c.ws.SetWriteDeadline(t)
}

func (c *SOCKS5WSConn) SetReadDeadline(t time.Time) error  { return c.ws.SetReadDeadline(t) }
func (c *SOCKS5WSConn) SetWriteDeadline(t time.Time) error { return c.ws.SetWriteDeadline(t) }

func NewSOCKS5Proxy(authenticator *auth.Authenticator, nodePool *nodepool.NodePool, wsNodePool *nodepool.WebSocketNodePool, metrics *metrics.Collector, logger *logrus.Entry) *SOCKS5Proxy {
	nodeRegURL := os.Getenv("NODE_REGISTRATION_URL")
	if nodeRegURL == "" {
		nodeRegURL = "http://node-registration:8001"
	}
	
	proxy := &SOCKS5Proxy{
		authenticator: authenticator,
		nodePool:      nodePool,
		wsNodePool:    wsNodePool,
		metrics:       metrics,
		logger:        logger.WithField("component", "socks5-proxy"),
		nodeRegURL:    nodeRegURL,
	}

	// Configure SOCKS5 server
	conf := &socks5.Config{
		AuthMethods: []socks5.Authenticator{
			&socks5.UserPassAuthenticator{
				Credentials: proxy,
			},
		},
		Dial: proxy.dialThroughNode,
	}

	server, err := socks5.New(conf)
	if err != nil {
		logger.Fatalf("Failed to create SOCKS5 server: %v", err)
	}

	proxy.server = server
	return proxy
}

func (p *SOCKS5Proxy) Serve(listener net.Listener) error {
	return p.server.Serve(listener)
}

// Valid implements the socks5 UserPassAuthenticator interface
func (p *SOCKS5Proxy) Valid(user, password string) bool {
	// Parse authentication (same format as HTTP proxy)
	authString := fmt.Sprintf("%s:%s", user, password)
	auth, err := p.authenticator.ParseProxyAuth(authString)
	if err != nil {
		p.logger.Warnf("SOCKS5 authentication failed: %v", err)
		return false
	}

	// Store auth in connection context (we'll retrieve this in dialThroughNode)
	// For simplicity in MVP, we'll use a simple cache
	p.storeAuthForConnection(user, auth)

	return true
}

func (p *SOCKS5Proxy) dialThroughNode(ctx context.Context, network, addr string) (net.Conn, error) {
	start := time.Now()

	// Extract username from the connection (this is a simplified approach)
	// In production, you'd want a more robust way to track connection context
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %v", err)
	}

	// For MVP, we'll use the most recently authenticated user
	// This is not thread-safe and should be improved for production
	auth := p.getLastAuth()
	if auth == nil {
		return nil, fmt.Errorf("no authentication context")
	}

	// Select node
	selection := &nodepool.NodeSelection{
		Country:   auth.Country,
		City:      auth.City,
		SessionID: auth.SessionID,
	}

	node, err := p.nodePool.SelectNode(selection)
	if err != nil {
		p.logger.Errorf("Failed to select node for SOCKS5: %v", err)
		return nil, fmt.Errorf("no nodes available")
	}

	// Connect through node
	conn, err := p.connectThroughNode(node, host, port)
	if err != nil {
		p.nodePool.ReleaseNode(node.ID)
		return nil, err
	}

	// Wrap connection to track usage and release node when closed
	wrappedConn := &trackedConnection{
		Conn:          conn,
		proxy:         p,
		nodeID:        node.ID,
		customerID:    auth.Customer.ID,
		nodeCountry:   node.Country,
		targetHost:    host,
		startTime:     start,
		bytesRead:     0,
		bytesWritten:  0,
	}

	p.logger.Debugf("SOCKS5 connection established to %s via node %s", addr, node.ID)
	return wrappedConn, nil
}

func (p *SOCKS5Proxy) connectThroughNode(node *nodepool.Node, host, port string) (net.Conn, error) {
	p.logger.Infof("SOCKS5 routing to %s:%s via node %s (%s)", host, port, node.ID, node.IPAddress)
	
	// Build WebSocket tunnel URL to node-registration service
	tunnelURL := strings.Replace(p.nodeRegURL, "http://", "ws://", 1)
	tunnelURL = strings.Replace(tunnelURL, "https://", "wss://", 1)
	tunnelURL = fmt.Sprintf("%s/internal/tunnel?node_id=%s&host=%s&port=%s", 
		tunnelURL, node.ID, host, port)

	p.logger.Debugf("SOCKS5 connecting via WebSocket tunnel: %s", tunnelURL)

	// Connect to WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	wsConn, _, err := dialer.Dial(tunnelURL, nil)
	if err != nil {
		p.logger.Errorf("SOCKS5 WebSocket tunnel failed: %v", err)
		return nil, fmt.Errorf("failed to establish tunnel: %v", err)
	}

	p.logger.Infof("SOCKS5 tunnel established to %s:%s via node %s", host, port, node.ID)

	return &SOCKS5WSConn{
		ws:     wsConn,
		logger: p.logger,
	}, nil
}

// Simple auth storage for MVP (not production-ready)
var lastAuth *auth.ProxyAuth

func (p *SOCKS5Proxy) storeAuthForConnection(user string, auth *auth.ProxyAuth) {
	lastAuth = auth
}

func (p *SOCKS5Proxy) getLastAuth() *auth.ProxyAuth {
	return lastAuth
}

// trackedConnection wraps a net.Conn to track bandwidth usage
type trackedConnection struct {
	net.Conn
	proxy        *SOCKS5Proxy
	nodeID       string
	customerID   string
	nodeCountry  string
	targetHost   string
	startTime    time.Time
	bytesRead    int64
	bytesWritten int64
	closed       bool
}

func (tc *trackedConnection) Read(b []byte) (n int, err error) {
	n, err = tc.Conn.Read(b)
	tc.bytesRead += int64(n)
	return n, err
}

func (tc *trackedConnection) Write(b []byte) (n int, err error) {
	n, err = tc.Conn.Write(b)
	tc.bytesWritten += int64(n)
	return n, err
}

func (tc *trackedConnection) Close() error {
	if !tc.closed {
		tc.closed = true
		
		// Record usage
		totalBytes := tc.bytesRead + tc.bytesWritten
		if totalBytes > 0 {
			tc.proxy.authenticator.RecordUsage(tc.customerID, totalBytes, tc.nodeID, true, tc.nodeCountry, tc.targetHost)
		}

		// Record metrics
		duration := time.Since(tc.startTime)
		tc.proxy.metrics.RecordRequest(tc.customerID, "", duration, true)

		// Release node
		tc.proxy.nodePool.ReleaseNode(tc.nodeID)

		tc.proxy.logger.Debugf("SOCKS5 connection closed, transferred %d bytes via node %s", totalBytes, tc.nodeID)
	}
	
	return tc.Conn.Close()
}