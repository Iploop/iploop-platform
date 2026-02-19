package main

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/joho/godotenv"

	"node-registration/internal/api"
	"node-registration/internal/config"
	"node-registration/internal/websocket"
	"node-registration/internal/nodemanager"
)

// --- DDoS / abuse protection ---

const (
	maxWSPerIP        = 50
	maxTotalWS        = 30000
	apiRateLimit      = 60 // requests per minute per IP
	apiRateWindowSecs = 60
)

var (
	wsPerIP      sync.Map // ip -> *int64 (atomic counter)
	totalWSConns int64    // atomic
)

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip, _, err := net.SplitHostPort(xff); err == nil {
			return ip
		}
		// might not have port
		return xff
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// wsIPTrack increments per-IP WS counter, returns true if allowed
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

// Simple sliding window rate limiter for API
type apiRateBucket struct {
	mu     sync.Mutex
	tokens int
	last   time.Time
}

var apiRateBuckets sync.Map // ip -> *apiRateBucket

func apiRateAllow(ip string) bool {
	now := time.Now()
	val, _ := apiRateBuckets.LoadOrStore(ip, &apiRateBucket{tokens: apiRateLimit, last: now})
	b := val.(*apiRateBucket)
	b.mu.Lock()
	defer b.mu.Unlock()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += int(elapsed * float64(apiRateLimit) / float64(apiRateWindowSecs))
	if b.tokens > apiRateLimit {
		b.tokens = apiRateLimit
	}
	b.last = now
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

func apiRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !apiRateAllow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

func main() {
	// Load environment variables
	godotenv.Load()

	// Initialize configuration
	cfg := config.Load()

	// Setup logger
	logrus.SetLevel(logrus.InfoLevel)
	if cfg.LogLevel == "debug" {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logrus.SetFormatter(&logrus.JSONFormatter{})

	logger := logrus.WithField("service", "node-registration")

	// Initialize database connection
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Fatalf("Failed to ping database: %v", err)
	}
	logger.Info("Connected to database")

	// Initialize Redis client
	redisAddr := cfg.RedisAddr
	if redisAddr == "" {
		// Fall back to REDIS_URL, strip redis:// prefix
		redisAddr = cfg.RedisURL
		if len(redisAddr) > 8 {
			redisAddr = redisAddr[8:]
		}
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	defer rdb.Close()

	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		logger.Fatalf("Failed to connect to Redis: %v", err)
	}
	logger.Info("Connected to Redis")

	// Initialize node manager
	nodeManager := nodemanager.NewNodeManager(db, rdb, logger)

	// Initialize WebSocket hub
	hub := websocket.NewHub(nodeManager, logger)
	go hub.Run()

	// Initialize proxy manager and wire it up
	proxyManager := websocket.NewProxyManager(hub, logger)
	hub.SetProxyManager(proxyManager)

	// Initialize tunnel manager and wire it up
	tunnelManager := websocket.NewTunnelManager(hub, logger)
	hub.SetTunnelManager(tunnelManager)

	// Initialize handlers for internal API
	proxyHandler := api.NewProxyHandler(proxyManager, logger)
	tunnelHandler := api.NewTunnelHandler(tunnelManager, logger)

	// Setup HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// WebSocket endpoint for node connections (with per-IP and total limits)
	router.GET("/ws", func(c *gin.Context) {
		// Check total connection cap
		if atomic.LoadInt64(&totalWSConns) >= maxTotalWS {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "server at capacity"})
			return
		}
		ip := c.ClientIP()
		if !wsIPTrack(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many connections from your IP"})
			return
		}
		atomic.AddInt64(&totalWSConns, 1)
		// NOTE: release counters when connection closes — handled via defer in a wrapper
		defer func() {
			atomic.AddInt64(&totalWSConns, -1)
			wsIPRelease(ip)
		}()
		websocket.HandleNodeConnection(hub, c.Writer, c.Request)
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		// Check database
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "database connection failed",
			})
			return
		}

		// Check Redis
		if _, err := rdb.Ping(ctx).Result(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "redis connection failed",
			})
			return
		}

		stats := nodeManager.GetStatistics()
		c.JSON(http.StatusOK, gin.H{
			"status":           "healthy",
			"timestamp":        time.Now().UTC(),
			"service":          "node-registration",
			"connected_nodes":  hub.GetConnectedCount(),
			"total_nodes":      stats.TotalNodes,
			"active_nodes":     stats.ActiveNodes,
			"inactive_nodes":   stats.InactiveNodes,
			"country_breakdown": stats.CountryBreakdown,
			"device_types":     stats.DeviceTypes,
			"connection_types": stats.ConnectionTypes,
		})
	})

	// Node status endpoint
	router.GET("/nodes", func(c *gin.Context) {
		nodes := nodeManager.GetAllNodes()
		c.JSON(http.StatusOK, gin.H{
			"nodes":     nodes,
			"count":     len(nodes),
			"timestamp": time.Now().UTC(),
		})
	})

	// Node statistics endpoint
	router.GET("/stats", func(c *gin.Context) {
		stats := nodeManager.GetStatistics()
		c.JSON(http.StatusOK, stats)
	})

	// Connected nodes endpoint — returns only nodes with active WebSocket
	router.GET("/internal/connected-nodes", func(c *gin.Context) {
		ids := hub.GetConnectedNodeIDs()
		c.JSON(http.StatusOK, gin.H{
			"node_ids": ids,
			"count":    len(ids),
		})
	})

	// Apply rate limiting to internal/API endpoints
	internal := router.Group("/")
	internal.Use(apiRateLimitMiddleware())

	// Internal proxy endpoint (called by proxy-gateway)
	internal.POST("/internal/proxy", func(c *gin.Context) {
		proxyHandler.HandleProxyRequest(c.Writer, c.Request)
	})

	// Internal tunnel WebSocket endpoint (called by proxy-gateway for CONNECT)
	internal.GET("/internal/tunnel", func(c *gin.Context) {
		tunnelHandler.HandleTunnelWebSocket(c.Writer, c.Request)
	})

	// Internal standby tunnel endpoint (pre-opened tunnel pool)
	internal.GET("/internal/tunnel-standby", func(c *gin.Context) {
		tunnelHandler.HandleTunnelStandby(c.Writer, c.Request)
	})

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Infof("Node registration server starting on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	logger.Info("Shutting down server...")

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	// Close WebSocket hub
	hub.Close()

	logger.Info("Node registration server stopped")
}