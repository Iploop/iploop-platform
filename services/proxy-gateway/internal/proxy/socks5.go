package proxy

import (
	"fmt"
	"net"
	"time"

	"github.com/armon/go-socks5"
	"github.com/sirupsen/logrus"

	"proxy-gateway/internal/auth"
	"proxy-gateway/internal/nodepool"
	"proxy-gateway/internal/metrics"
)

type SOCKS5Proxy struct {
	authenticator *auth.Authenticator
	nodePool      *nodepool.NodePool
	metrics       *metrics.Collector
	logger        *logrus.Entry
	server        *socks5.Server
}

func NewSOCKS5Proxy(authenticator *auth.Authenticator, nodePool *nodepool.NodePool, metrics *metrics.Collector, logger *logrus.Entry) *SOCKS5Proxy {
	proxy := &SOCKS5Proxy{
		authenticator: authenticator,
		nodePool:      nodePool,
		metrics:       metrics,
		logger:        logger.WithField("component", "socks5-proxy"),
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

func (p *SOCKS5Proxy) dialThroughNode(network, addr string) (net.Conn, error) {
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
		startTime:     start,
		bytesRead:     0,
		bytesWritten:  0,
	}

	p.logger.Debugf("SOCKS5 connection established to %s via node %s", addr, node.ID)
	return wrappedConn, nil
}

func (p *SOCKS5Proxy) connectThroughNode(node *nodepool.Node, host, port string) (net.Conn, error) {
	// For MVP, connect directly (same as HTTP proxy)
	// In production, route through actual node
	
	p.logger.Debugf("SOCKS5 routing to %s:%s via node %s (%s)", host, port, node.ID, node.IPAddress)
	
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s:%s: %v", host, port, err)
	}

	return conn, nil
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
			tc.proxy.authenticator.RecordUsage(tc.customerID, totalBytes, tc.nodeID, true)
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