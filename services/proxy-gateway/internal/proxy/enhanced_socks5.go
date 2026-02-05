package proxy

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/armon/go-socks5"
	"github.com/sirupsen/logrus"

	"proxy-gateway/internal/auth"
	"proxy-gateway/internal/nodepool"
	"proxy-gateway/internal/metrics"
	"proxy-gateway/internal/session"
	"proxy-gateway/internal/headers"
)

type EnhancedSOCKS5Proxy struct {
	authenticator   *auth.Authenticator
	nodePool        *nodepool.NodePool
	wsNodePool      *nodepool.WebSocketNodePool
	sessionManager  *session.SessionManager
	headerManager   *headers.HeaderManager
	metrics         *metrics.Collector
	logger          *logrus.Entry
	server          *socks5.Server
	
	// Connection context tracking
	connections     map[string]*ConnectionContext
	connectionsMutex sync.RWMutex
}

type ConnectionContext struct {
	Auth          *auth.EnhancedProxyAuth
	Session       *session.Session
	StartTime     time.Time
	BytesRead     int64
	BytesWritten  int64
	RequestCount  int64
	ClientIP      string
	Target        string
}

// CustomResolver implements socks5.NameResolver interface
type CustomResolver struct {
	proxy *EnhancedSOCKS5Proxy
}

func (r *CustomResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	// Custom DNS resolution through nodes if needed
	// For now, use standard resolution
	ips, err := net.LookupIP(name)
	if err != nil {
		return ctx, nil, err
	}
	
	if len(ips) == 0 {
		return ctx, nil, fmt.Errorf("no IPs found for %s", name)
	}
	
	return ctx, ips[0], nil
}

func NewEnhancedSOCKS5Proxy(
	authenticator *auth.Authenticator,
	nodePool *nodepool.NodePool,
	wsNodePool *nodepool.WebSocketNodePool,
	sessionManager *session.SessionManager,
	headerManager *headers.HeaderManager,
	metrics *metrics.Collector,
	logger *logrus.Entry,
) *EnhancedSOCKS5Proxy {
	
	proxy := &EnhancedSOCKS5Proxy{
		authenticator:  authenticator,
		nodePool:       nodePool,
		wsNodePool:     wsNodePool,
		sessionManager: sessionManager,
		headerManager:  headerManager,
		metrics:        metrics,
		logger:         logger.WithField("component", "enhanced-socks5"),
		connections:    make(map[string]*ConnectionContext),
	}
	
	// Configure enhanced SOCKS5 server
	conf := &socks5.Config{
		AuthMethods: []socks5.Authenticator{
			&socks5.UserPassAuthenticator{
				Credentials: proxy,
			},
			&socks5.NoAuthAuthenticator{}, // For IP whitelist auth
		},
		Dial:     proxy.dialThroughNode,
		Resolver: &CustomResolver{proxy: proxy},
	}
	
	server, err := socks5.New(conf)
	if err != nil {
		logger.Fatalf("Failed to create enhanced SOCKS5 server: %v", err)
	}
	
	proxy.server = server
	return proxy
}

func (p *EnhancedSOCKS5Proxy) Serve(listener net.Listener) error {
	p.logger.Infof("Enhanced SOCKS5 proxy listening on %s", listener.Addr().String())
	return p.server.Serve(listener)
}

// Valid implements the socks5 UserPassAuthenticator interface
func (p *EnhancedSOCKS5Proxy) Valid(user, password string) bool {
	// Get client IP for IP whitelist auth
	clientIP := p.getClientIP()
	
	// Parse enhanced authentication
	authString := fmt.Sprintf("%s:%s", user, password)
	auth, err := p.authenticator.ParseEnhancedAuth(authString, clientIP)
	if err != nil {
		p.logger.Warnf("SOCKS5 enhanced authentication failed for %s: %v", clientIP, err)
		return false
	}
	
	// Store authentication context for this connection
	p.storeConnectionContext(user, auth, clientIP)
	
	p.logger.Infof("SOCKS5 authentication successful for customer %s (%s) from %s", 
		auth.Customer.ID, auth.Method, clientIP)
	
	return true
}

func (p *EnhancedSOCKS5Proxy) dialThroughNode(ctx context.Context, network, addr string) (net.Conn, error) {
	start := time.Now()
	
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address %s: %v", addr, err)
	}
	
	// Get connection context (this is simplified - in production you'd track per connection)
	connCtx := p.getConnectionContext()
	if connCtx == nil {
		return nil, fmt.Errorf("no authentication context")
	}
	
	// Get or create session
	sess, err := p.sessionManager.GetOrCreateSession(connCtx.Auth)
	if err != nil {
		p.logger.Errorf("Failed to get session for SOCKS5: %v", err)
		return nil, fmt.Errorf("session error: %v", err)
	}
	
	connCtx.Session = sess
	connCtx.Target = addr
	
	// Check rate limits and quotas
	if err := p.checkLimits(connCtx); err != nil {
		return nil, err
	}
	
	// Connect through assigned node
	conn, err := p.connectThroughNode(sess, host, port)
	if err != nil {
		p.metrics.RecordRequest(sess.CustomerID, addr, time.Since(start), false)
		return nil, err
	}
	
	// Wrap connection for tracking and session management
	wrappedConn := &EnhancedTrackedConnection{
		Conn:        conn,
		proxy:       p,
		context:     connCtx,
		startTime:   start,
	}
	
	p.logger.Debugf("SOCKS5 connection established to %s via node %s (session: %s)", 
		addr, sess.CurrentNodeID, sess.ID)
	
	return wrappedConn, nil
}

func (p *EnhancedSOCKS5Proxy) connectThroughNode(sess *session.Session, host, port string) (net.Conn, error) {
	// Check if we should use WebSocket nodes or direct connection
	if sess.CurrentNodeID != "" {
		// Try WebSocket node first for better routing
		if p.wsNodePool != nil {
			wsConn, err := p.connectViaWebSocket(sess, host, port)
			if err == nil {
				return wsConn, nil
			}
			p.logger.Warnf("WebSocket connection failed, falling back to direct: %v", err)
		}
		
		// Fall back to direct connection through node IP
		return p.connectDirectly(sess.CurrentNodeIP, host, port)
	}
	
	return nil, fmt.Errorf("no node assigned to session")
}

func (p *EnhancedSOCKS5Proxy) connectViaWebSocket(sess *session.Session, host, port string) (net.Conn, error) {
	// This would implement WebSocket-based routing through the node network
	// For now, we'll implement direct connection
	return p.connectDirectly(sess.CurrentNodeIP, host, port)
}

func (p *EnhancedSOCKS5Proxy) connectDirectly(nodeIP, host, port string) (net.Conn, error) {
	// Connect to target through the assigned node's IP
	// In production, this would route through the actual node infrastructure
	
	targetAddr := net.JoinHostPort(host, port)
	p.logger.Debugf("SOCKS5 connecting to %s via node %s", targetAddr, nodeIP)
	
	// For MVP: direct connection (replace with actual node routing)
	conn, err := net.DialTimeout("tcp", targetAddr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", targetAddr, err)
	}
	
	return conn, nil
}

func (p *EnhancedSOCKS5Proxy) checkLimits(connCtx *ConnectionContext) error {
	auth := connCtx.Auth
	
	// Check bandwidth quota
	if auth.Customer.GBBalance <= 0 {
		return fmt.Errorf("insufficient bandwidth quota")
	}
	
	// Check concurrent connections limit
	// This would be implemented based on customer plan
	
	// Check rate limits
	// This would be implemented with Redis-based rate limiting
	
	return nil
}

func (p *EnhancedSOCKS5Proxy) getClientIP() string {
	// This is simplified - in production you'd extract from the actual connection
	return "127.0.0.1"
}

func (p *EnhancedSOCKS5Proxy) storeConnectionContext(connectionID string, auth *auth.EnhancedProxyAuth, clientIP string) {
	p.connectionsMutex.Lock()
	defer p.connectionsMutex.Unlock()
	
	p.connections[connectionID] = &ConnectionContext{
		Auth:      auth,
		StartTime: time.Now(),
		ClientIP:  clientIP,
	}
}

func (p *EnhancedSOCKS5Proxy) getConnectionContext() *ConnectionContext {
	p.connectionsMutex.RLock()
	defer p.connectionsMutex.RUnlock()
	
	// This is simplified - in production you'd track per actual connection
	for _, ctx := range p.connections {
		return ctx
	}
	return nil
}

// EnhancedTrackedConnection wraps connections with comprehensive tracking
type EnhancedTrackedConnection struct {
	net.Conn
	proxy       *EnhancedSOCKS5Proxy
	context     *ConnectionContext
	startTime   time.Time
	closed      bool
	mutex       sync.Mutex
}

func (etc *EnhancedTrackedConnection) Read(b []byte) (n int, err error) {
	n, err = etc.Conn.Read(b)
	
	etc.mutex.Lock()
	etc.context.BytesRead += int64(n)
	etc.mutex.Unlock()
	
	// Update session usage
	if etc.context.Session != nil {
		etc.proxy.sessionManager.RecordUsage(etc.context.Session.ID, int64(n), err == nil)
	}
	
	return n, err
}

func (etc *EnhancedTrackedConnection) Write(b []byte) (n int, err error) {
	n, err = etc.Conn.Write(b)
	
	etc.mutex.Lock()
	etc.context.BytesWritten += int64(n)
	etc.context.RequestCount++
	etc.mutex.Unlock()
	
	// Update session usage
	if etc.context.Session != nil {
		etc.proxy.sessionManager.RecordUsage(etc.context.Session.ID, int64(n), err == nil)
	}
	
	return n, err
}

func (etc *EnhancedTrackedConnection) Close() error {
	etc.mutex.Lock()
	defer etc.mutex.Unlock()
	
	if etc.closed {
		return nil
	}
	etc.closed = true
	
	// Calculate final metrics
	duration := time.Since(etc.startTime)
	totalBytes := etc.context.BytesRead + etc.context.BytesWritten
	success := etc.context.RequestCount > 0
	
	// Record usage for billing
	if etc.context.Auth != nil && etc.context.Auth.Customer != nil {
		err := etc.proxy.authenticator.RecordUsage(
			etc.context.Auth.Customer.ID,
			totalBytes,
			etc.context.Session.CurrentNodeID,
			success,
		)
		if err != nil {
			etc.proxy.logger.Warnf("Failed to record usage: %v", err)
		}
	}
	
	// Record metrics
	if etc.context.Session != nil {
		etc.proxy.metrics.RecordRequest(
			etc.context.Session.CustomerID,
			etc.context.Target,
			duration,
			success,
		)
	}
	
	etc.proxy.logger.Debugf("SOCKS5 connection closed - Duration: %v, Bytes: %d, Requests: %d", 
		duration, totalBytes, etc.context.RequestCount)
	
	return etc.Conn.Close()
}

// GetConnectionStats returns current connection statistics
func (p *EnhancedSOCKS5Proxy) GetConnectionStats() map[string]interface{} {
	p.connectionsMutex.RLock()
	defer p.connectionsMutex.RUnlock()
	
	stats := map[string]interface{}{
		"active_connections": len(p.connections),
		"connections":        make([]map[string]interface{}, 0, len(p.connections)),
	}
	
	for id, ctx := range p.connections {
		connStats := map[string]interface{}{
			"id":               id,
			"customer_id":      ctx.Auth.Customer.ID,
			"client_ip":        ctx.ClientIP,
			"target":           ctx.Target,
			"start_time":       ctx.StartTime,
			"duration":         time.Since(ctx.StartTime).Seconds(),
			"bytes_read":       ctx.BytesRead,
			"bytes_written":    ctx.BytesWritten,
			"request_count":    ctx.RequestCount,
			"auth_method":      ctx.Auth.Method,
			"session_id":       "",
			"node_id":          "",
		}
		
		if ctx.Session != nil {
			connStats["session_id"] = ctx.Session.ID
			connStats["node_id"] = ctx.Session.CurrentNodeID
		}
		
		stats["connections"] = append(stats["connections"].([]map[string]interface{}), connStats)
	}
	
	return stats
}

// StartMonitoring begins connection monitoring routines
func (p *EnhancedSOCKS5Proxy) StartMonitoring() {
	// Cleanup stale connections
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				p.cleanupStaleConnections()
			}
		}
	}()
}

func (p *EnhancedSOCKS5Proxy) cleanupStaleConnections() {
	p.connectionsMutex.Lock()
	defer p.connectionsMutex.Unlock()
	
	now := time.Now()
	staleThreshold := 10 * time.Minute
	
	for id, ctx := range p.connections {
		if now.Sub(ctx.StartTime) > staleThreshold {
			delete(p.connections, id)
			p.logger.Debugf("Cleaned up stale connection context: %s", id)
		}
	}
}