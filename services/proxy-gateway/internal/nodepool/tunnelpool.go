package nodepool

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	// Target number of idle tunnels to maintain
	tunnelPoolSize = 50
	// How often to replenish the pool
	tunnelRefillInterval = 5 * time.Second
	// Max age before we close and replace an idle tunnel
	tunnelMaxIdleAge = 2 * time.Minute
	// Health check interval for idle tunnels
	tunnelHealthInterval = 30 * time.Second
	// Timeout for opening a pre-tunnel
	tunnelOpenTimeout = 8 * time.Second
	// Max concurrent tunnel opens during refill
	tunnelRefillConcurrency = 10
	// Stats log interval
	tunnelStatsInterval = 2 * time.Minute
)

// IdleTunnel represents a pre-opened tunnel ready for use.
// The WS connection is established between proxy-gateway and node-registration,
// and node-registration has verified the SDK node is alive and ready.
// The tunnel is NOT connected to a target yet — that happens on first use.
type IdleTunnel struct {
	NodeID    string
	Country   string
	Conn      *websocket.Conn
	CreatedAt time.Time
}

// TunnelPool maintains a pool of pre-opened tunnels to proven nodes.
// When a customer request arrives, we grab an idle tunnel and send the
// target info — saving the WS dial + node verification overhead.
type TunnelPool struct {
	nodePool   *NodePool
	warmPool   *WarmPool
	nodeRegURL string
	logger     *logrus.Entry
	stopCh     chan struct{}
	wg         sync.WaitGroup

	mu      sync.Mutex
	tunnels []*IdleTunnel

	// stats
	opened    int64 // tunnels pre-opened
	served    int64 // tunnels served to requests
	expired   int64 // tunnels expired (too old)
	failed    int64 // tunnel opens that failed
	healthErr int64 // tunnels killed by health check
}

// NewTunnelPool creates and starts the tunnel pool.
func NewTunnelPool(nodePool *NodePool, warmPool *WarmPool, nodeRegURL string, logger *logrus.Entry) *TunnelPool {
	tp := &TunnelPool{
		nodePool:   nodePool,
		warmPool:   warmPool,
		nodeRegURL: nodeRegURL,
		logger:     logger.WithField("component", "tunnel-pool"),
		stopCh:     make(chan struct{}),
		tunnels:    make([]*IdleTunnel, 0, tunnelPoolSize),
	}
	tp.wg.Add(2)
	go tp.refillLoop()
	go tp.statsLoop()
	return tp
}

// GetTunnel returns a pre-opened idle tunnel for the given country (or any).
// Returns nil if none available — caller should fall back to on-demand dial.
func (tp *TunnelPool) GetTunnel(country string) *IdleTunnel {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Prefer matching country
	if country != "" {
		for i, t := range tp.tunnels {
			if t.Country == country {
				tp.tunnels = append(tp.tunnels[:i], tp.tunnels[i+1:]...)
				atomic.AddInt64(&tp.served, 1)
				return t
			}
		}
	}

	// Fall back to any available
	if len(tp.tunnels) > 0 {
		t := tp.tunnels[0]
		tp.tunnels = tp.tunnels[1:]
		atomic.AddInt64(&tp.served, 1)
		return t
	}

	return nil
}

// Size returns current pool size.
func (tp *TunnelPool) Size() int {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	return len(tp.tunnels)
}

// Stop shuts down the tunnel pool and closes all idle tunnels.
func (tp *TunnelPool) Stop() {
	close(tp.stopCh)
	tp.wg.Wait()

	tp.mu.Lock()
	for _, t := range tp.tunnels {
		t.Conn.Close()
	}
	tp.tunnels = nil
	tp.mu.Unlock()
}

// refillLoop periodically tops up the pool and evicts stale tunnels.
func (tp *TunnelPool) refillLoop() {
	defer tp.wg.Done()
	ticker := time.NewTicker(tunnelRefillInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tp.stopCh:
			return
		case <-ticker.C:
			tp.evictStale()
			tp.refill()
		}
	}
}

// evictStale removes tunnels older than tunnelMaxIdleAge.
func (tp *TunnelPool) evictStale() {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	now := time.Now()
	fresh := make([]*IdleTunnel, 0, len(tp.tunnels))
	for _, t := range tp.tunnels {
		if now.Sub(t.CreatedAt) > tunnelMaxIdleAge {
			t.Conn.Close()
			atomic.AddInt64(&tp.expired, 1)
		} else {
			fresh = append(fresh, t)
		}
	}
	tp.tunnels = fresh
}

// refill opens new tunnels to reach the target pool size.
func (tp *TunnelPool) refill() {
	tp.mu.Lock()
	needed := tunnelPoolSize - len(tp.tunnels)
	tp.mu.Unlock()

	if needed <= 0 {
		return
	}

	// Don't open too many at once
	if needed > tunnelRefillConcurrency {
		needed = tunnelRefillConcurrency
	}

	// Get proven node IDs from warm pool
	nodeIDs := tp.getTargetNodes(needed)
	if len(nodeIDs) == 0 {
		return
	}

	// Open tunnels in parallel
	var wg sync.WaitGroup
	results := make(chan *IdleTunnel, len(nodeIDs))

	for _, nid := range nodeIDs {
		wg.Add(1)
		go func(nodeID string) {
			defer wg.Done()
			tunnel, err := tp.openPreTunnel(nodeID)
			if err != nil {
				atomic.AddInt64(&tp.failed, 1)
				return
			}
			results <- tunnel
		}(nid)
	}

	// Collect in background
	go func() {
		wg.Wait()
		close(results)
	}()

	for tunnel := range results {
		tp.mu.Lock()
		if len(tp.tunnels) < tunnelPoolSize {
			tp.tunnels = append(tp.tunnels, tunnel)
		} else {
			tunnel.Conn.Close() // pool full
		}
		tp.mu.Unlock()
	}
}

// getTargetNodes returns node IDs to pre-open tunnels to.
// Prefers fast nodes from the warm pool. Avoids nodes already in the pool.
func (tp *TunnelPool) getTargetNodes(count int) []string {
	tp.mu.Lock()
	existing := make(map[string]bool)
	for _, t := range tp.tunnels {
		existing[t.NodeID] = true
	}
	tp.mu.Unlock()

	nodeIDs := make([]string, 0, count)

	// Try warm pool first
	if tp.warmPool != nil {
		for i := 0; i < count*3 && len(nodeIDs) < count; i++ {
			fastID := tp.warmPool.GetFastNode("")
			if fastID == "" {
				break
			}
			if existing[fastID] {
				continue
			}
			existing[fastID] = true
			nodeIDs = append(nodeIDs, fastID)
		}
	}

	return nodeIDs
}

// openPreTunnel opens a "standby" tunnel to node-registration.
// It uses the /internal/tunnel-standby endpoint which:
// 1. Verifies the node is connected
// 2. Keeps the WS connection open and ready
// 3. On first write, receives host:port and opens the actual tunnel to the SDK
func (tp *TunnelPool) openPreTunnel(nodeID string) (*IdleTunnel, error) {
	// Look up node for country info
	node, err := tp.nodePool.GetNodeByID(nodeID)
	country := ""
	if err == nil && node != nil {
		country = node.Country
	}

	wsURL := tp.nodeRegURL
	wsURL = fmt.Sprintf("%s/internal/tunnel-standby?node_id=%s",
		wsURL, nodeID)
	// Convert http to ws
	if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}

	ctx, cancel := context.WithTimeout(context.Background(), tunnelOpenTimeout)
	defer cancel()

	dialer := websocket.Dialer{
		HandshakeTimeout: tunnelOpenTimeout,
		NetDialContext:   (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	}

	type dialResult struct {
		conn *websocket.Conn
		err  error
	}
	ch := make(chan dialResult, 1)

	go func() {
		conn, _, err := dialer.Dial(wsURL, nil)
		ch <- dialResult{conn, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}

		// Read the "ready" acknowledgment from node-reg
		res.conn.SetReadDeadline(time.Now().Add(tunnelOpenTimeout))
		_, msg, err := res.conn.ReadMessage()
		if err != nil {
			res.conn.Close()
			return nil, fmt.Errorf("standby ack read: %w", err)
		}

		// Verify it's an ack
		if string(msg) != "standby_ready" {
			res.conn.Close()
			return nil, fmt.Errorf("unexpected standby response: %s", string(msg))
		}

		// Clear deadline for idle
		res.conn.SetReadDeadline(time.Time{})

		atomic.AddInt64(&tp.opened, 1)
		tp.logger.Debugf("Pre-opened tunnel to node %s (%s)", nodeID, country)

		return &IdleTunnel{
			NodeID:    nodeID,
			Country:   country,
			Conn:      res.conn,
			CreatedAt: time.Now(),
		}, nil
	}
}

// ActivateTunnel sends the target host:port to a standby tunnel,
// converting it into an active tunnel. Returns the same WS connection
// which is now connected end-to-end.
func ActivateTunnel(tunnel *IdleTunnel, host, port string) error {
	msg := fmt.Sprintf("%s:%s", host, port)
	tunnel.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err := tunnel.Conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return fmt.Errorf("activate write: %w", err)
	}

	// Wait for "tunnel_active" confirmation
	tunnel.Conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	_, resp, err := tunnel.Conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("activate read: %w", err)
	}

	if string(resp) != "tunnel_active" {
		return fmt.Errorf("activation failed: %s", string(resp))
	}

	// Clear deadlines for relay
	tunnel.Conn.SetReadDeadline(time.Time{})
	tunnel.Conn.SetWriteDeadline(time.Time{})
	return nil
}

func (tp *TunnelPool) statsLoop() {
	defer tp.wg.Done()
	ticker := time.NewTicker(tunnelStatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tp.stopCh:
			return
		case <-ticker.C:
			tp.logger.Infof("[TUNNEL-POOL] size=%d opened=%d served=%d expired=%d failed=%d",
				tp.Size(),
				atomic.LoadInt64(&tp.opened),
				atomic.LoadInt64(&tp.served),
				atomic.LoadInt64(&tp.expired),
				atomic.LoadInt64(&tp.failed))
		}
	}
}
