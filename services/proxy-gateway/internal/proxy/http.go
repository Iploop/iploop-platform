package proxy

import (
	"bufio"
	"bytes"
	"context"
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
	"sync/atomic"
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
	wsNodePool      *nodepool.WebSocketNodePool
	warmPool        *nodepool.WarmPool
	tunnelPool      *nodepool.TunnelPool
	metrics         *metrics.Collector
	logger          *logrus.Entry
	nodeRegURL      string
	httpClient      *http.Client
}

// SetWarmPool attaches a warm pool for fast-lane node selection.
func (p *HTTPProxy) SetWarmPool(wp *nodepool.WarmPool) {
	p.warmPool = wp
}

// SetTunnelPool attaches a pre-opened tunnel pool.
func (p *HTTPProxy) SetTunnelPool(tp *nodepool.TunnelPool) {
	p.tunnelPool = tp
}

func NewHTTPProxy(authenticator *auth.Authenticator, nodePool *nodepool.NodePool, wsNodePool *nodepool.WebSocketNodePool, metrics *metrics.Collector, logger *logrus.Entry) *HTTPProxy {
	nodeRegURL := os.Getenv("NODE_REGISTRATION_URL")
	if nodeRegURL == "" {
		nodeRegURL = "http://node-registration:8001"
	}

	return &HTTPProxy{
		authenticator: authenticator,
		nodePool:      nodePool,
		wsNodePool:    wsNodePool,
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
	// For rotating/per-request sessions, don't use session ID (forces new node each time)
	sessionID := auth.SessionID
	if auth.SessionType == "rotating" || auth.SessionType == "per-request" {
		sessionID = "" // Empty session = fresh node selection every request
	}
	selection := &nodepool.NodeSelection{
		Country:   auth.Country,
		City:      auth.City,
		SessionID: sessionID,
	}

	if r.Method == http.MethodConnect {
		// ── CONNECT: try pre-opened tunnel first, then race ──
		node, ok := p.tryTunnelPoolConnect(w, r, auth)
		if !ok {
			node, ok = p.raceConnectTunnel(w, r, auth, selection)
		}
		if !ok {
			return
		}
		duration := time.Since(start)
		p.metrics.RecordRequest(auth.Customer.ID, node.Country, duration, true)
		p.logger.Debugf("Request completed in %v via node %s (race winner)", duration, node.ID)
	} else {
		// ── Plain HTTP: parallel node racing with retry ──
		// Buffer body so retries can re-send it
		if r.Body != nil {
			bodyData, _ := io.ReadAll(r.Body)
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(bodyData))
			r.ContentLength = int64(len(bodyData))
			// Store for retries
			r = r.Clone(r.Context())
			r.Body = io.NopCloser(bytes.NewReader(bodyData))
			r.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(bodyData)), nil
			}
		}
		maxHTTPRetries := 3
		deadline := time.Now().Add(20 * time.Second) // total budget for all retries
		var node *nodepool.Node
		var ok bool
		for attempt := 0; attempt < maxHTTPRetries; attempt++ {
			if time.Now().After(deadline) {
				p.logger.Warnf("HTTP retries exhausted time budget for %s", r.URL.String())
				break
			}
			if attempt > 0 {
				p.logger.Infof("HTTP retry attempt %d/%d for %s", attempt+1, maxHTTPRetries, r.URL.String())
				// Reset body for retry
				if r.GetBody != nil {
					r.Body, _ = r.GetBody()
				}
			}
			// Try pre-opened tunnel pool first
			if attempt == 0 {
				node, ok = p.tryTunnelPoolHTTP(w, r, auth)
			}
			if !ok {
				node, ok = p.raceHTTPTunnel(w, r, auth, selection)
			}
			if ok {
				break
			}
		}
		if !ok {
			http.Error(w, "All proxy attempts failed after retries", http.StatusBadGateway)
			return
		}
		duration := time.Since(start)
		p.metrics.RecordRequest(auth.Customer.ID, node.Country, duration, true)
		p.logger.Debugf("HTTP request completed in %v via node %s (race winner)", duration, node.ID)
	}
}

// raceResult carries the outcome of one parallel tunnel dial attempt.
type raceResult struct {
	node   *nodepool.Node
	wsConn *websocket.Conn
	err    error
	warm   bool // true if this candidate came from the warm pool
}

// raceConnectTunnel selects up to 3 candidate nodes (preferring fast-lane),
// dials them all in parallel, and uses the first successful tunnel.
// Losers are closed/blacklisted. Returns the winning node or writes an error.
func (p *HTTPProxy) raceConnectTunnel(w http.ResponseWriter, r *http.Request, proxyAuth *auth.ProxyAuth, selection *nodepool.NodeSelection) (*nodepool.Node, bool) {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, "Invalid host", http.StatusBadRequest)
		return nil, false
	}

	const racers = 5
	p.logger.Infof("CONNECT race to %s:%s — selecting up to %d candidates", host, port, racers)

	// ── Gather candidates (warm-pool first, then normal pool) ──
	candidates := make([]*nodepool.Node, 0, racers)
	seen := make(map[string]bool) // de-duplicate by node ID
	warmFlags := make([]bool, 0, racers)

	// Pull from warm pool
	if p.warmPool != nil && (proxyAuth.SessionType == "rotating" || proxyAuth.SessionType == "per-request" || selection.SessionID == "") {
		for len(candidates) < racers {
			fastID := p.warmPool.GetFastNode(proxyAuth.Country)
			if fastID == "" {
				break
			}
			if seen[fastID] {
				continue
			}
			n, err := p.nodePool.GetNodeByID(fastID)
			if err != nil || n == nil {
				continue
			}
			seen[n.ID] = true
			candidates = append(candidates, n)
			warmFlags = append(warmFlags, true)
		}
	}

	// Fill remaining slots from normal pool
	for len(candidates) < racers {
		n, err := p.nodePool.SelectNode(selection)
		if err != nil {
			break
		}
		if seen[n.ID] {
			p.nodePool.ReleaseNode(n.ID)
			continue
		}
		seen[n.ID] = true
		candidates = append(candidates, n)
		warmFlags = append(warmFlags, false)
	}

	if len(candidates) == 0 {
		http.Error(w, "No nodes available", http.StatusBadGateway)
		return nil, false
	}

	p.logger.Infof("CONNECT race to %s:%s — racing %d nodes", host, port, len(candidates))

	// ── Launch parallel dials ──
	resultCh := make(chan raceResult, len(candidates))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Track how many goroutines are still running so we can drain safely.
	var launched int32

	for i, node := range candidates {
		atomic.AddInt32(&launched, 1)
		go func(n *nodepool.Node, isWarm bool) {
			wsConn, dialErr := p.dialTunnel(ctx, n, host, port)
			// If context was already cancelled and dial succeeded, close immediately.
			select {
			case <-ctx.Done():
				if wsConn != nil {
					wsConn.Close()
				}
				// Still send result so the drain loop can count it.
				resultCh <- raceResult{node: n, wsConn: nil, err: context.Canceled, warm: isWarm}
			case resultCh <- raceResult{node: n, wsConn: wsConn, err: dialErr, warm: isWarm}:
			}
		}(node, warmFlags[i])
	}

	// ── Wait for first success (or all failures) ──
	var winner *raceResult
	received := 0
	total := int(atomic.LoadInt32(&launched))

	for received < total {
		res := <-resultCh
		received++

		if res.err == nil && winner == nil {
			// First successful dial — claim it and cancel the rest.
			winner = &res
			cancel()
			p.logger.Infof("CONNECT race winner: node %s (%s, warm=%v) in slot %d/%d",
				res.node.ID, res.node.Country, res.warm, received, total)
		} else if res.err == nil && winner != nil {
			// A later success after we already have a winner — close it.
			if res.wsConn != nil {
				res.wsConn.Close()
			}
			p.nodePool.ReleaseNode(res.node.ID)
			p.logger.Debugf("CONNECT race: closing runner-up node %s", res.node.ID)
		} else if res.err != nil && res.err != context.Canceled {
			// Genuine failure — blacklist.
			p.logger.Warnf("CONNECT race: node %s failed: %v — blacklisting", res.node.ID, res.err)
			p.nodePool.BlacklistNode(res.node.ID, 15*time.Minute)
			p.nodePool.ReleaseNode(res.node.ID)
		} else {
			// Cancelled — just release.
			p.nodePool.ReleaseNode(res.node.ID)
		}
	}

	if winner == nil {
		http.Error(w, "All tunnel attempts failed", http.StatusBadGateway)
		return nil, false
	}

	// ── Hand off winning connection to the relay ──
	p.handleConnectTunnel(w, r, winner.node, proxyAuth, winner.wsConn, host, port)
	p.nodePool.ReleaseNode(winner.node.ID)
	return winner.node, true
}

// raceHTTPTunnel selects up to 3 candidate nodes, dials them in parallel,
// and uses the first successful tunnel for a plain HTTP request.
func (p *HTTPProxy) raceHTTPTunnel(w http.ResponseWriter, r *http.Request, proxyAuth *auth.ProxyAuth, selection *nodepool.NodeSelection) (*nodepool.Node, bool) {
	targetURL := r.URL
	if !targetURL.IsAbs() {
		http.Error(w, "Absolute URL required", http.StatusBadRequest)
		return nil, false
	}

	host := targetURL.Hostname()
	port := targetURL.Port()
	if port == "" {
		port = "80"
	}

	const racers = 5
	p.logger.Infof("HTTP race to %s — selecting up to %d candidates", targetURL.String(), racers)

	// ── Gather candidates (warm-pool first, then normal pool) ──
	candidates := make([]*nodepool.Node, 0, racers)
	seen := make(map[string]bool)
	warmFlags := make([]bool, 0, racers)

	if p.warmPool != nil && (proxyAuth.SessionType == "rotating" || proxyAuth.SessionType == "per-request" || selection.SessionID == "") {
		for len(candidates) < racers {
			fastID := p.warmPool.GetFastNode(proxyAuth.Country)
			if fastID == "" {
				break
			}
			if seen[fastID] {
				continue
			}
			n, err := p.nodePool.GetNodeByID(fastID)
			if err != nil || n == nil {
				continue
			}
			seen[n.ID] = true
			candidates = append(candidates, n)
			warmFlags = append(warmFlags, true)
		}
	}

	for len(candidates) < racers {
		n, err := p.nodePool.SelectNode(selection)
		if err != nil {
			break
		}
		if seen[n.ID] {
			p.nodePool.ReleaseNode(n.ID)
			continue
		}
		seen[n.ID] = true
		candidates = append(candidates, n)
		warmFlags = append(warmFlags, false)
	}

	if len(candidates) == 0 {
		p.logger.Warnf("HTTP race: no nodes available for %s", targetURL.String())
		return nil, false
	}

	p.logger.Infof("HTTP race to %s — racing %d nodes", targetURL.String(), len(candidates))

	// ── Launch parallel dials ──
	resultCh := make(chan raceResult, len(candidates))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var launched int32

	for i, node := range candidates {
		atomic.AddInt32(&launched, 1)
		go func(n *nodepool.Node, isWarm bool) {
			wsConn, dialErr := p.dialTunnel(ctx, n, host, port)
			select {
			case <-ctx.Done():
				if wsConn != nil {
					wsConn.Close()
				}
				resultCh <- raceResult{node: n, wsConn: nil, err: context.Canceled, warm: isWarm}
			case resultCh <- raceResult{node: n, wsConn: wsConn, err: dialErr, warm: isWarm}:
			}
		}(node, warmFlags[i])
	}

	// ── Wait for first success (or all failures) ──
	var winner *raceResult
	received := 0
	total := int(atomic.LoadInt32(&launched))

	for received < total {
		res := <-resultCh
		received++

		if res.err == nil && winner == nil {
			winner = &res
			cancel()
			p.logger.Infof("HTTP race winner: node %s (%s, warm=%v) in slot %d/%d",
				res.node.ID, res.node.Country, res.warm, received, total)
		} else if res.err == nil && winner != nil {
			if res.wsConn != nil {
				res.wsConn.Close()
			}
			p.nodePool.ReleaseNode(res.node.ID)
		} else if res.err != nil && res.err != context.Canceled {
			p.logger.Warnf("HTTP race: node %s failed: %v — blacklisting", res.node.ID, res.err)
			p.nodePool.BlacklistNode(res.node.ID, 15*time.Minute)
			p.nodePool.ReleaseNode(res.node.ID)
		} else {
			p.nodePool.ReleaseNode(res.node.ID)
		}
	}

	if winner == nil {
		p.logger.Warnf("HTTP race: all %d tunnel attempts failed for %s", len(candidates), targetURL.String())
		return nil, false
	}

	// ── Send HTTP request through winning tunnel ──
	success := p.handleHTTPWithConn(w, r, winner.node, proxyAuth, winner.wsConn, host)
	p.nodePool.ReleaseNode(winner.node.ID)
	if !success {
		return winner.node, false // malformed response — caller should retry
	}
	return winner.node, true
}

// handleHTTPWithConn sends an HTTP request through a pre-established WebSocket tunnel.
// Returns true if response was successfully parsed and written, false if malformed (should retry).
func (p *HTTPProxy) handleHTTPWithConn(w http.ResponseWriter, r *http.Request, node *nodepool.Node, proxyAuth *auth.ProxyAuth, wsConn *websocket.Conn, host string) bool {
	defer wsConn.Close()

	targetURL := r.URL

	// Build raw HTTP request
	var reqBuf bytes.Buffer
	path := targetURL.RequestURI()
	if path == "" {
		path = "/"
	}
	reqBuf.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", r.Method, path))
	reqBuf.WriteString(fmt.Sprintf("Host: %s\r\n", targetURL.Host))

	for name, values := range r.Header {
		lowerName := strings.ToLower(name)
		if lowerName == "proxy-authorization" || lowerName == "proxy-connection" {
			continue
		}
		for _, v := range values {
			reqBuf.WriteString(fmt.Sprintf("%s: %s\r\n", name, v))
		}
	}

	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
	}
	if len(bodyBytes) > 0 {
		reqBuf.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(bodyBytes)))
	}
	reqBuf.WriteString("\r\n")
	if len(bodyBytes) > 0 {
		reqBuf.Write(bodyBytes)
	}

	if err := wsConn.WriteMessage(websocket.BinaryMessage, reqBuf.Bytes()); err != nil {
		p.logger.Errorf("Failed to send request through tunnel: %v", err)
		return false // retry
	}
	bytesUp := int64(reqBuf.Len())

	// Read response
	var respBuf bytes.Buffer
	bytesDown := int64(0)

	wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
	messageType, data, err := wsConn.ReadMessage()
	if err != nil {
		p.logger.Errorf("Failed to read response from tunnel: %v", err)
		return false // retry
	}
	if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
		respBuf.Write(data)
		bytesDown += int64(len(data))
	}

	for {
		wsConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		messageType, data, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		if messageType == websocket.CloseMessage {
			break
		}
		if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
			respBuf.Write(data)
			bytesDown += int64(len(data))
		}
	}

	respReader := bufio.NewReader(&respBuf)
	httpResp, err := http.ReadResponse(respReader, r)
	if err != nil {
		p.logger.Warnf("Malformed HTTP response from node %s, blacklisting and retrying: %v", node.ID, err)
		p.nodePool.BlacklistNode(node.ID, 15*time.Minute)
		return false // retry with different node
	}
	defer httpResp.Body.Close()

	for name, values := range httpResp.Header {
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}
	w.WriteHeader(httpResp.StatusCode)
	bodyWritten, _ := io.Copy(w, httpResp.Body)

	// Flush response to client immediately
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	totalBytes := bytesUp + bytesDown
	p.authenticator.RecordUsage(proxyAuth.Customer.ID, totalBytes, node.ID, true, node.Country, host)

	// Mark this node as proven — it actually completed a request
	p.nodePool.MarkProven(node.ID)

	p.logger.Infof("HTTP tunnel completed: %s via node %s, status=%d, bytes: up=%d down=%d body=%d",
		targetURL.String(), node.ID, httpResp.StatusCode, bytesUp, bytesDown, bodyWritten)
	return true
}

// dialTunnel opens a WebSocket tunnel to node-registration for a single node.
// It respects ctx cancellation so in-flight dials don't linger after a winner is chosen.
func (p *HTTPProxy) dialTunnel(ctx context.Context, node *nodepool.Node, host, port string) (*websocket.Conn, error) {
	tunnelURL := strings.Replace(p.nodeRegURL, "http://", "ws://", 1)
	tunnelURL = strings.Replace(tunnelURL, "https://", "wss://", 1)
	tunnelURL = fmt.Sprintf("%s/internal/tunnel?node_id=%s&host=%s&port=%s",
		tunnelURL, node.ID, host, port)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// websocket.Dialer doesn't natively take a context for cancellation,
	// but we can use DialContext via the underlying net.Dialer.
	dialer.NetDialContext = (&net.Dialer{Timeout: 10 * time.Second}).DialContext

	// Use a channel to bridge gorilla's Dial with our context.
	type dialResult struct {
		conn *websocket.Conn
		err  error
	}
	ch := make(chan dialResult, 1)

	go func() {
		conn, _, err := dialer.Dial(tunnelURL, nil)
		ch <- dialResult{conn, err}
	}()

	select {
	case <-ctx.Done():
		// Context cancelled — the dial goroutine will finish eventually;
		// if it succeeds the caller (raceConnectTunnel) will close it.
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		// Double-check context hasn't been cancelled while we waited.
		select {
		case <-ctx.Done():
			res.conn.Close()
			return nil, ctx.Err()
		default:
		}
		return res.conn, nil
	}
}

// tryTunnelPoolConnect attempts to use a pre-opened tunnel for CONNECT requests.
func (p *HTTPProxy) tryTunnelPoolConnect(w http.ResponseWriter, r *http.Request, proxyAuth *auth.ProxyAuth) (*nodepool.Node, bool) {
	if p.tunnelPool == nil {
		return nil, false
	}

	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		return nil, false
	}

	idle := p.tunnelPool.GetTunnel(proxyAuth.Country)
	if idle == nil {
		return nil, false
	}

	p.logger.Infof("CONNECT using pre-opened tunnel to node %s for %s:%s", idle.NodeID, host, port)

	// Activate the standby tunnel with the target
	if err := nodepool.ActivateTunnel(idle, host, port); err != nil {
		p.logger.Warnf("Pre-opened tunnel activation failed for node %s: %v — falling back to race", idle.NodeID, err)
		idle.Conn.Close()
		return nil, false
	}

	// Get node info
	node, err := p.nodePool.GetNodeByID(idle.NodeID)
	if err != nil || node == nil {
		node = &nodepool.Node{ID: idle.NodeID, Country: idle.Country}
	}

	p.logger.Infof("CONNECT pre-opened tunnel activated: node %s target %s:%s", idle.NodeID, host, port)
	p.handleConnectTunnel(w, r, node, proxyAuth, idle.Conn, host, port)
	return node, true
}

// tryTunnelPoolHTTP attempts to use a pre-opened tunnel for plain HTTP requests.
func (p *HTTPProxy) tryTunnelPoolHTTP(w http.ResponseWriter, r *http.Request, proxyAuth *auth.ProxyAuth) (*nodepool.Node, bool) {
	if p.tunnelPool == nil {
		return nil, false
	}

	targetURL := r.URL
	host := targetURL.Hostname()
	port := targetURL.Port()
	if port == "" {
		if targetURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	idle := p.tunnelPool.GetTunnel(proxyAuth.Country)
	if idle == nil {
		return nil, false
	}

	p.logger.Infof("HTTP using pre-opened tunnel to node %s for %s:%s", idle.NodeID, host, port)

	// Activate the standby tunnel
	if err := nodepool.ActivateTunnel(idle, host, port); err != nil {
		p.logger.Warnf("Pre-opened HTTP tunnel activation failed for node %s: %v — falling back to race", idle.NodeID, err)
		idle.Conn.Close()
		return nil, false
	}

	node, err := p.nodePool.GetNodeByID(idle.NodeID)
	if err != nil || node == nil {
		node = &nodepool.Node{ID: idle.NodeID, Country: idle.Country}
	}

	p.logger.Infof("HTTP pre-opened tunnel activated: node %s target %s:%s", idle.NodeID, host, port)
	p.handleHTTPWithConn(w, r, node, proxyAuth, idle.Conn, host)
	return node, true
}

func (p *HTTPProxy) handleConnectTunnel(w http.ResponseWriter, r *http.Request, node *nodepool.Node, auth *auth.ProxyAuth, wsConn *websocket.Conn, host, port string) {
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
	
	// Blacklist nodes that can't route traffic (very low response = tunnel failed)
	if bytesDown < 100 && bytesUp > 100 {
		p.logger.Warnf("Node %s returned only %d bytes — blacklisting for 15 min", node.ID, bytesDown)
		p.nodePool.BlacklistNode(node.ID, 15*time.Minute)
	}

	// Record usage with country and target host
	p.authenticator.RecordUsage(auth.Customer.ID, totalBytes, node.ID, bytesDown >= 100, node.Country, host)
}

func (p *HTTPProxy) handleHTTP(w http.ResponseWriter, r *http.Request, node *nodepool.Node, auth *auth.ProxyAuth) {
	// Use WebSocket tunnel (same as CONNECT) - raw TCP streaming, no base64
	targetURL := r.URL
	if !targetURL.IsAbs() {
		http.Error(w, "Absolute URL required", http.StatusBadRequest)
		return
	}

	host := targetURL.Hostname()
	port := targetURL.Port()
	if port == "" {
		port = "80"
	}

	p.logger.Infof("HTTP tunnel request to %s via node %s (%s)", targetURL.String(), node.ID, node.IPAddress)

	// Connect to node-registration tunnel WebSocket
	tunnelURL := strings.Replace(p.nodeRegURL, "http://", "ws://", 1)
	tunnelURL = strings.Replace(tunnelURL, "https://", "wss://", 1)
	tunnelURL = fmt.Sprintf("%s/internal/tunnel?node_id=%s&host=%s&port=%s", tunnelURL, node.ID, host, port)

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}
	wsConn, _, err := dialer.Dial(tunnelURL, nil)
	if err != nil {
		p.logger.Errorf("Failed to connect to tunnel WebSocket: %v", err)
		http.Error(w, "Failed to establish tunnel", http.StatusBadGateway)
		return
	}
	defer wsConn.Close()

	// Build raw HTTP request
	var reqBuf bytes.Buffer
	
	// Request line: GET /path HTTP/1.1
	path := targetURL.RequestURI()
	if path == "" {
		path = "/"
	}
	reqBuf.WriteString(fmt.Sprintf("%s %s HTTP/1.1\r\n", r.Method, path))
	
	// Host header
	reqBuf.WriteString(fmt.Sprintf("Host: %s\r\n", targetURL.Host))
	
	// Copy other headers (excluding proxy-specific)
	for name, values := range r.Header {
		lowerName := strings.ToLower(name)
		if lowerName == "proxy-authorization" || lowerName == "proxy-connection" {
			continue
		}
		for _, v := range values {
			reqBuf.WriteString(fmt.Sprintf("%s: %s\r\n", name, v))
		}
	}
	
	// Read request body
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
	}
	
	// Add Content-Length if body exists
	if len(bodyBytes) > 0 {
		reqBuf.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(bodyBytes)))
	}
	
	// End headers
	reqBuf.WriteString("\r\n")
	
	// Add body
	if len(bodyBytes) > 0 {
		reqBuf.Write(bodyBytes)
	}

	// Send raw HTTP request through tunnel
	if err := wsConn.WriteMessage(websocket.BinaryMessage, reqBuf.Bytes()); err != nil {
		p.logger.Errorf("Failed to send request through tunnel: %v", err)
		http.Error(w, "Failed to send request", http.StatusBadGateway)
		return
	}
	
	bytesUp := int64(reqBuf.Len())

	// Read response from tunnel - for HTTP we expect a single response chunk
	var respBuf bytes.Buffer
	bytesDown := int64(0)
	
	// First read - wait for response with longer timeout
	wsConn.SetReadDeadline(time.Now().Add(60 * time.Second))
	messageType, data, err := wsConn.ReadMessage()
	if err != nil {
		p.logger.Errorf("Failed to read response from tunnel: %v", err)
		http.Error(w, "Failed to read response", http.StatusBadGateway)
		return
	}
	if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
		respBuf.Write(data)
		bytesDown += int64(len(data))
	}

	// Try to read more with short timeout (for chunked responses)
	for {
		wsConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		messageType, data, err := wsConn.ReadMessage()
		if err != nil {
			break // Timeout or close - we have enough data
		}
		if messageType == websocket.CloseMessage {
			break
		}
		if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
			respBuf.Write(data)
			bytesDown += int64(len(data))
		}
	}

	// Parse HTTP response
	respReader := bufio.NewReader(&respBuf)
	httpResp, err := http.ReadResponse(respReader, r)
	if err != nil {
		p.logger.Errorf("Failed to parse HTTP response: %v", err)
		// Fallback: write raw response
		w.WriteHeader(http.StatusBadGateway)
		w.Write(respBuf.Bytes())
		return
	}
	defer httpResp.Body.Close()

	// Copy response headers
	for name, values := range httpResp.Header {
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}

	// Write status code
	w.WriteHeader(httpResp.StatusCode)

	// Copy response body
	bodyWritten, _ := io.Copy(w, httpResp.Body)

	// Record usage with country and target host
	totalBytes := bytesUp + bytesDown
	p.authenticator.RecordUsage(auth.Customer.ID, totalBytes, node.ID, true, node.Country, host)

	p.logger.Infof("HTTP tunnel completed: %s via node %s, status=%d, bytes: up=%d down=%d body=%d", 
		targetURL.String(), node.ID, httpResp.StatusCode, bytesUp, bytesDown, bodyWritten)
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