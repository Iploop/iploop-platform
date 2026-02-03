package proxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"proxy-gateway/internal/auth"
	"proxy-gateway/internal/nodepool"
	"proxy-gateway/internal/metrics"
)

type HTTPProxy struct {
	authenticator   *auth.Authenticator
	nodePool        *nodepool.NodePool
	metrics         *metrics.Collector
	logger          *logrus.Entry
	nodeRegURL      string
	httpClient      *http.Client
}

func NewHTTPProxy(authenticator *auth.Authenticator, nodePool *nodepool.NodePool, metrics *metrics.Collector, logger *logrus.Entry) *HTTPProxy {
	nodeRegURL := os.Getenv("NODE_REGISTRATION_URL")
	if nodeRegURL == "" {
		nodeRegURL = "http://node-registration:8001"
	}

	return &HTTPProxy{
		authenticator: authenticator,
		nodePool:      nodePool,
		metrics:       metrics,
		logger:        logger.WithField("component", "http-proxy"),
		nodeRegURL:    nodeRegURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
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

	p.logger.Infof("CONNECT tunnel request to %s:%s via node %s (%s)", host, port, node.ID, node.IPAddress)

	// Connect to node-registration tunnel WebSocket
	tunnelURL := strings.Replace(p.nodeRegURL, "http://", "ws://", 1)
	tunnelURL = strings.Replace(tunnelURL, "https://", "wss://", 1)
	tunnelURL = fmt.Sprintf("%s/internal/tunnel?node_id=%s&host=%s&port=%s", tunnelURL, node.ID, host, port)

	p.logger.Debugf("Connecting to tunnel WebSocket: %s", tunnelURL)

	// Connect to WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	wsConn, _, err := dialer.Dial(tunnelURL, nil)
	if err != nil {
		p.logger.Errorf("Failed to connect to tunnel WebSocket: %v", err)
		http.Error(w, "Failed to establish tunnel", http.StatusBadGateway)
		return
	}
	defer wsConn.Close()

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Send 200 Connection established to client
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	p.logger.Debugf("Tunnel established, starting relay")

	// Relay data bidirectionally
	var wg sync.WaitGroup
	var bytesUp, bytesDown int64
	wg.Add(2)

	// Client -> WebSocket (to node)
	go func() {
		defer wg.Done()
		buf := make([]byte, 32768)
		for {
			clientConn.SetReadDeadline(time.Now().Add(60 * time.Second))
			n, err := clientBuf.Read(buf)
			if err != nil {
				if err != io.EOF {
					p.logger.Debugf("Client read error: %v", err)
				}
				wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if n > 0 {
				bytesUp += int64(n)
				if err := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					p.logger.Debugf("WebSocket write error: %v", err)
					return
				}
			}
		}
	}()

	// WebSocket (from node) -> Client
	go func() {
		defer wg.Done()
		for {
			wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					p.logger.Debugf("WebSocket read error: %v", err)
				}
				return
			}
			if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
				bytesDown += int64(len(data))
				clientConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				if _, err := clientConn.Write(data); err != nil {
					p.logger.Debugf("Client write error: %v", err)
					return
				}
			}
		}
	}()

	wg.Wait()
	
	totalBytes := bytesUp + bytesDown
	p.logger.Infof("CONNECT tunnel closed: %s:%s via node %s, bytes: up=%d down=%d", host, port, node.ID, bytesUp, bytesDown)
	
	// Record usage
	p.authenticator.RecordUsage(auth.Customer.ID, totalBytes, node.ID, true)
}

func (p *HTTPProxy) handleHTTP(w http.ResponseWriter, r *http.Request, node *nodepool.Node, auth *auth.ProxyAuth) {
	// Create new request to forward through node
	targetURL := r.URL
	if !targetURL.IsAbs() {
		http.Error(w, "Absolute URL required", http.StatusBadRequest)
		return
	}

	// Collect headers (excluding proxy-specific ones)
	headers := make(map[string]string)
	for name, values := range r.Header {
		if !strings.HasPrefix(strings.ToLower(name), "proxy-") && len(values) > 0 {
			headers[name] = values[0]
		}
	}

	// Read request body
	var bodyBytes []byte
	var err error
	if r.Body != nil {
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			p.logger.Errorf("Failed to read request body: %v", err)
			http.Error(w, "Failed to read request", http.StatusInternalServerError)
			return
		}
	}

	// Send request through the node via node-registration service
	proxyResp, err := p.proxyThroughNode(node, r.Method, targetURL.String(), headers, bodyBytes)
	if err != nil {
		p.logger.Errorf("Request failed through node %s: %v", node.ID, err)
		http.Error(w, "Request failed", http.StatusBadGateway)
		return
	}

	if !proxyResp.Success {
		p.logger.Errorf("Proxy request failed: %s", proxyResp.Error)
		http.Error(w, proxyResp.Error, http.StatusBadGateway)
		return
	}

	// Copy response headers
	for name, value := range proxyResp.Headers {
		w.Header().Set(name, value)
	}

	// Set status code
	statusCode := proxyResp.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}
	w.WriteHeader(statusCode)

	// Write response body
	var bytesTransferred int64
	if proxyResp.Body != "" {
		bodyData, err := base64.StdEncoding.DecodeString(proxyResp.Body)
		if err != nil {
			p.logger.Errorf("Failed to decode response body: %v", err)
		} else {
			bytesTransferred = int64(len(bodyData))
			w.Write(bodyData)
		}
	}

	// Record usage
	p.authenticator.RecordUsage(auth.Customer.ID, bytesTransferred, node.ID, true)
	
	p.logger.Infof("Request completed via node %s (%s): %s %d bytes", 
		node.ID, node.IPAddress, targetURL.String(), bytesTransferred)
}

// ProxyRequestPayload for internal proxy API
type ProxyRequestPayload struct {
	NodeID    string            `json:"node_id"`
	Host      string            `json:"host"`
	Port      string            `json:"port"`
	Method    string            `json:"method,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
	TimeoutMs int               `json:"timeout_ms"`
}

// ProxyResponsePayload from internal proxy API
type ProxyResponsePayload struct {
	Success    bool              `json:"success"`
	StatusCode int               `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Error      string            `json:"error,omitempty"`
	BytesRead  int64             `json:"bytes_read"`
	BytesWrite int64             `json:"bytes_write"`
	LatencyMs  int64             `json:"latency_ms"`
}

// proxyThroughNode sends an HTTP request through a node via the node-registration service
func (p *HTTPProxy) proxyThroughNode(node *nodepool.Node, method, targetURL string, headers map[string]string, body []byte) (*ProxyResponsePayload, error) {
	p.logger.Infof("Routing HTTP request to %s via node %s (%s, %s)", targetURL, node.ID, node.IPAddress, node.Country)

	// Parse the URL to get host and port
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	payload := ProxyRequestPayload{
		NodeID:    node.ID,
		Host:      host,
		Port:      port,
		Method:    method,
		URL:       targetURL,
		Headers:   headers,
		TimeoutMs: 30000,
	}

	if body != nil {
		payload.Body = base64.StdEncoding.EncodeToString(body)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send to node-registration internal API
	url := p.nodeRegURL + "/internal/proxy"
	p.logger.Debugf("Calling node-registration at: %s", url)
	
	resp, err := p.httpClient.Post(
		url,
		"application/json",
		bytes.NewReader(payloadJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call node-registration: %v", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	
	p.logger.Debugf("Response from node-registration: status=%d, body=%s", resp.StatusCode, string(bodyBytes))

	var proxyResp ProxyResponsePayload
	if err := json.Unmarshal(bodyBytes, &proxyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v (body: %s)", err, string(bodyBytes))
	}

	return &proxyResp, nil
}

func (p *HTTPProxy) connectThroughNode(node *nodepool.Node, host, port string) (net.Conn, error) {
	// For TCP tunnel (CONNECT), we still need a direct connection approach
	// This is more complex as it requires streaming through WebSocket
	// For MVP, we'll use direct connection for HTTPS CONNECT
	// Real implementation would need bidirectional WebSocket tunnel
	
	p.logger.Warnf("CONNECT tunnel for %s:%s via node %s - using direct connection (TODO: implement WebSocket tunnel)", host, port, node.ID)
	
	// Direct connection (fallback for CONNECT tunnels)
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