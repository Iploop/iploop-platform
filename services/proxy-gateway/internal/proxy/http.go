package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"proxy-gateway/internal/auth"
	"proxy-gateway/internal/nodepool"
	"proxy-gateway/internal/metrics"
)

type HTTPProxy struct {
	authenticator *auth.Authenticator
	nodePool      *nodepool.NodePool
	metrics       *metrics.Collector
	logger        *logrus.Entry
}

func NewHTTPProxy(authenticator *auth.Authenticator, nodePool *nodepool.NodePool, metrics *metrics.Collector, logger *logrus.Entry) *HTTPProxy {
	return &HTTPProxy{
		authenticator: authenticator,
		nodePool:      nodePool,
		metrics:       metrics,
		logger:        logger.WithField("component", "http-proxy"),
	}
}

func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	
	// Authenticate request
	proxyAuth := r.Header.Get("Proxy-Authorization")
	if proxyAuth == "" {
		p.sendAuthRequired(w)
		return
	}

	auth, err := p.authenticator.ParseProxyAuth(proxyAuth)
	if err != nil {
		p.logger.Warnf("Authentication failed: %v", err)
		http.Error(w, "Authentication failed", http.StatusProxyAuthRequired)
		return
	}

	// Select node
	selection := &nodepool.NodeSelection{
		Country:   auth.Country,
		City:      auth.City,
		SessionID: auth.SessionID,
	}

	node, err := p.nodePool.SelectNode(selection)
	if err != nil {
		p.logger.Errorf("Failed to select node: %v", err)
		http.Error(w, "No nodes available", http.StatusBadGateway)
		return
	}
	defer p.nodePool.ReleaseNode(node.ID)

	// Handle CONNECT method (HTTPS tunneling)
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r, node, auth)
	} else {
		p.handleHTTP(w, r, node, auth)
	}

	// Record metrics
	duration := time.Since(start)
	p.metrics.RecordRequest(auth.Customer.ID, node.Country, duration, true)

	p.logger.Debugf("Request completed in %v via node %s", duration, node.ID)
}

func (p *HTTPProxy) handleConnect(w http.ResponseWriter, r *http.Request, node *nodepool.Node, auth *auth.ProxyAuth) {
	// Extract target host and port
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, "Invalid host", http.StatusBadRequest)
		return
	}

	// Connect to target through the selected node
	targetConn, err := p.connectThroughNode(node, host, port)
	if err != nil {
		p.logger.Errorf("Failed to connect through node %s: %v", node.ID, err)
		http.Error(w, "Failed to connect", http.StatusBadGateway)
		return
	}
	defer targetConn.Close()

	// Send 200 Connection established
	w.WriteHeader(http.StatusOK)

	// Get the underlying connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Start tunneling
	go func() {
		io.Copy(targetConn, clientConn)
		targetConn.Close()
	}()

	bytesTransferred, _ := io.Copy(clientConn, targetConn)
	
	// Record usage
	p.authenticator.RecordUsage(auth.Customer.ID, bytesTransferred, node.ID, true)
}

func (p *HTTPProxy) handleHTTP(w http.ResponseWriter, r *http.Request, node *nodepool.Node, auth *auth.ProxyAuth) {
	// Create new request to forward through node
	targetURL := r.URL
	if !targetURL.IsAbs() {
		http.Error(w, "Absolute URL required", http.StatusBadRequest)
		return
	}

	// Create HTTP client that routes through the node
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				return p.connectThroughNode(node, host, port)
			},
		},
		Timeout: 30 * time.Second,
	}

	// Create new request
	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Copy headers (excluding proxy-specific ones)
	for name, values := range r.Header {
		if !strings.HasPrefix(strings.ToLower(name), "proxy-") {
			for _, value := range values {
				proxyReq.Header.Add(name, value)
			}
		}
	}

	// Execute request
	resp, err := client.Do(proxyReq)
	if err != nil {
		p.logger.Errorf("Request failed through node %s: %v", node.ID, err)
		http.Error(w, "Request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body and count bytes
	bytesTransferred, err := io.Copy(w, resp.Body)
	if err != nil {
		p.logger.Errorf("Failed to copy response: %v", err)
		return
	}

	// Record usage
	p.authenticator.RecordUsage(auth.Customer.ID, bytesTransferred, node.ID, true)
}

func (p *HTTPProxy) connectThroughNode(node *nodepool.Node, host, port string) (net.Conn, error) {
	// For MVP, we'll connect directly from the proxy server
	// In production, this would route through the actual node
	
	// TODO: Implement actual node routing via WebSocket tunnel or similar
	// For now, simulate by connecting directly but logging the node used
	
	p.logger.Debugf("Routing connection to %s:%s via node %s (%s)", host, port, node.ID, node.IPAddress)
	
	// Direct connection (simulated node routing)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s:%s: %v", host, port, err)
	}

	return conn, nil
}

func (p *HTTPProxy) sendAuthRequired(w http.ResponseWriter) {
	w.Header().Set("Proxy-Authenticate", "Basic realm=\"IPLoop Proxy\"")
	w.WriteHeader(http.StatusProxyAuthRequired)
	w.Write([]byte("Proxy authentication required"))
}