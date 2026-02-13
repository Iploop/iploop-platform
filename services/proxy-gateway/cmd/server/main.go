package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/joho/godotenv"

	"proxy-gateway/internal/proxy"
	"proxy-gateway/internal/auth"
	"proxy-gateway/internal/nodepool"
	"proxy-gateway/internal/config"
	"proxy-gateway/internal/metrics"
)

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

	logger := logrus.WithField("service", "proxy-gateway")

	// Initialize database connection
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		logger.Fatalf("Failed to ping database: %v", err)
	}
	logger.Info("Connected to database")

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL[8:], // Remove redis:// prefix
		Password: "",
		DB:       0,
	})
	defer rdb.Close()

	// Test Redis connection
	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		logger.Fatalf("Failed to connect to Redis: %v", err)
	}
	logger.Info("Connected to Redis")

	// Initialize components
	authenticator := auth.NewAuthenticator(db, rdb)
	nodePool := nodepool.NewNodePool(rdb, logger)
	wsNodePool := nodepool.NewWebSocketNodePool(nodePool, logger)
	metricsCollector := metrics.NewCollector()

	// Initialize warm pool for pre-validated fast-lane nodes
	nodeRegURL := os.Getenv("NODE_REGISTRATION_URL")
	if nodeRegURL == "" {
		nodeRegURL = "http://node-registration:8001"
	}
	warmPool := nodepool.NewWarmPool(nodePool, nodeRegURL, logger)
	defer warmPool.Stop()

	// Initialize pre-opened tunnel pool
	tunnelPool := nodepool.NewTunnelPool(nodePool, warmPool, nodeRegURL, logger)
	defer tunnelPool.Stop()

	// Initialize proxy servers (with WebSocket node pool for real-time routing)
	httpProxy := proxy.NewHTTPProxy(authenticator, nodePool, wsNodePool, metricsCollector, logger)
	httpProxy.SetWarmPool(warmPool)
	httpProxy.SetTunnelPool(tunnelPool)
	socksProxy := proxy.NewSOCKS5Proxy(authenticator, nodePool, wsNodePool, metricsCollector, logger)

	// Start HTTP proxy server
	httpListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.HTTPPort))
	if err != nil {
		logger.Fatalf("Failed to listen on HTTP port %s: %v", cfg.HTTPPort, err)
	}
	defer httpListener.Close()

	go func() {
		logger.Infof("HTTP proxy server starting on port %s", cfg.HTTPPort)
		if err := http.Serve(httpListener, httpProxy); err != nil {
			logger.Errorf("HTTP proxy server error: %v", err)
		}
	}()

	// Start SOCKS5 proxy server
	socksListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.SOCKSPort))
	if err != nil {
		logger.Fatalf("Failed to listen on SOCKS port %s: %v", cfg.SOCKSPort, err)
	}
	defer socksListener.Close()

	go func() {
		logger.Infof("SOCKS5 proxy server starting on port %s", cfg.SOCKSPort)
		if err := socksProxy.Serve(socksListener); err != nil {
			logger.Errorf("SOCKS5 proxy server error: %v", err)
		}
	}()

	// Start health check and metrics server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

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

		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"service":   "proxy-gateway",
		})
	})

	// Metrics endpoint
	router.GET("/metrics", func(c *gin.Context) {
		stats := metricsCollector.GetStats()
		c.JSON(http.StatusOK, stats)
	})

	// Node pool status endpoint
	router.GET("/nodes", func(c *gin.Context) {
		status := nodePool.GetStatus()
		c.JSON(http.StatusOK, status)
	})

	// WebSocket node pool status
	router.GET("/nodes/ws", func(c *gin.Context) {
		stats := wsNodePool.GetStats()
		c.JSON(http.StatusOK, stats)
	})

	// Warm pool status
	router.GET("/nodes/warm", func(c *gin.Context) {
		stats := warmPool.GetStats()
		c.JSON(http.StatusOK, stats)
	})

	// WebSocket endpoint for node connections
	router.GET("/node/connect", func(c *gin.Context) {
		wsNodePool.HandleNodeConnection(c.Writer, c.Request)
	})

	healthServer := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		logger.Info("Health check server starting on port 8080")
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Health server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	logger.Info("Shutting down servers...")

	// Shutdown health server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Health server forced to shutdown: %v", err)
	}

	logger.Info("Proxy gateway stopped")
}