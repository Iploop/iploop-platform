package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"proxy-gateway/internal/analytics"
	"proxy-gateway/internal/auth"
	"proxy-gateway/internal/headers"
	"proxy-gateway/internal/metrics"
	"proxy-gateway/internal/nodepool"
	"proxy-gateway/internal/proxy"
	"proxy-gateway/internal/session"
)

type EnhancedProxyGateway struct {
	db             *sql.DB
	rdb            *redis.Client
	authenticator  *auth.Authenticator
	nodePool       *nodepool.NodePool
	wsNodePool     *nodepool.WebSocketNodePool
	sessionManager *session.SessionManager
	headerManager  *headers.HeaderManager
	analytics      *analytics.AnalyticsManager
	metrics        *metrics.Collector
	logger         *logrus.Entry

	// Proxy servers
	httpProxy   *proxy.HTTPProxy
	socks5Proxy *proxy.EnhancedSOCKS5Proxy

	// API server
	apiServer *gin.Engine
}

func main() {
	// Setup logging
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)
	logger := logrus.WithField("service", "enhanced-proxy-gateway")

	// Load configuration
	config := loadConfig()
	
	logger.Infof("Starting Enhanced IPLoop Proxy Gateway v2.0")
	logger.Infof("Config: HTTP=%s, SOCKS5=%s, API=%s", config.HTTPPort, config.SOCKS5Port, config.APIPort)

	// Initialize gateway
	gateway, err := NewEnhancedProxyGateway(config, logger)
	if err != nil {
		logger.Fatalf("Failed to initialize gateway: %v", err)
	}

	// Start all services
	if err := gateway.Start(config); err != nil {
		logger.Fatalf("Failed to start gateway: %v", err)
	}

	// Wait for shutdown signal
	gateway.WaitForShutdown()
}

type Config struct {
	DatabaseURL  string
	RedisURL     string
	HTTPPort     string
	SOCKS5Port   string
	APIPort      string
	MetricsPort  string
	NodeRegURL   string
	Environment  string
}

func loadConfig() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:pass@localhost/iploop?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		SOCKS5Port:  getEnv("SOCKS5_PORT", "1080"),
		APIPort:     getEnv("API_PORT", "8090"),
		MetricsPort: getEnv("METRICS_PORT", "8091"),
		NodeRegURL:  getEnv("NODE_REGISTRATION_URL", "http://node-registration:8001"),
		Environment: getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func NewEnhancedProxyGateway(config *Config, logger *logrus.Entry) (*EnhancedProxyGateway, error) {
	// Initialize database
	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %v", err)
	}

	// Initialize Redis
	opt, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %v", err)
	}
	rdb := redis.NewClient(opt)

	// Initialize core components
	authenticator := auth.NewAuthenticator(db, rdb)
	nodePool := nodepool.NewNodePool(db, rdb, logger)
	wsNodePool := nodepool.NewWebSocketNodePool(config.NodeRegURL, logger)
	sessionManager := session.NewSessionManager(rdb, nodePool, wsNodePool, logger)
	headerManager := headers.NewHeaderManager()
	metricsCollector := metrics.NewCollector(rdb, logger)
	analyticsManager := analytics.NewAnalyticsManager(db, rdb, logger)

	// Initialize proxy servers
	httpProxy := proxy.NewHTTPProxy(authenticator, nodePool, wsNodePool, metricsCollector, logger)
	socks5Proxy := proxy.NewEnhancedSOCKS5Proxy(
		authenticator, nodePool, wsNodePool, sessionManager, 
		headerManager, metricsCollector, logger)

	gateway := &EnhancedProxyGateway{
		db:             db,
		rdb:            rdb,
		authenticator:  authenticator,
		nodePool:       nodePool,
		wsNodePool:     wsNodePool,
		sessionManager: sessionManager,
		headerManager:  headerManager,
		analytics:      analyticsManager,
		metrics:        metricsCollector,
		logger:         logger,
		httpProxy:      httpProxy,
		socks5Proxy:    socks5Proxy,
	}

	// Setup API server
	gateway.setupAPIServer()

	return gateway, nil
}

func (g *EnhancedProxyGateway) Start(config *Config) error {
	// Start background services
	g.sessionManager.StartCleanupRoutine()
	g.socks5Proxy.StartMonitoring()

	// Start HTTP Proxy
	go func() {
		listener, err := net.Listen("tcp", ":"+config.HTTPPort)
		if err != nil {
			g.logger.Fatalf("Failed to start HTTP proxy: %v", err)
		}
		g.logger.Infof("HTTP Proxy listening on :%s", config.HTTPPort)
		
		if err := http.Serve(listener, g.httpProxy); err != nil {
			g.logger.Errorf("HTTP Proxy error: %v", err)
		}
	}()

	// Start SOCKS5 Proxy
	go func() {
		listener, err := net.Listen("tcp", ":"+config.SOCKS5Port)
		if err != nil {
			g.logger.Fatalf("Failed to start SOCKS5 proxy: %v", err)
		}
		g.logger.Infof("SOCKS5 Proxy listening on :%s", config.SOCKS5Port)
		
		if err := g.socks5Proxy.Serve(listener); err != nil {
			g.logger.Errorf("SOCKS5 Proxy error: %v", err)
		}
	}()

	// Start API Server
	go func() {
		g.logger.Infof("API Server listening on :%s", config.APIPort)
		if err := g.apiServer.Run(":" + config.APIPort); err != nil {
			g.logger.Errorf("API Server error: %v", err)
		}
	}()

	// Start Metrics Server
	go func() {
		metricsHandler := g.metrics.Handler()
		g.logger.Infof("Metrics Server listening on :%s", config.MetricsPort)
		if err := http.ListenAndServe(":"+config.MetricsPort, metricsHandler); err != nil {
			g.logger.Errorf("Metrics Server error: %v", err)
		}
	}()

	return nil
}

func (g *EnhancedProxyGateway) setupAPIServer() {
	if g.logger.Logger.Level == logrus.DebugLevel {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"timestamp": time.Now(),
			"version": "2.0.0",
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Authentication endpoints
		v1.POST("/auth/validate", g.handleAuthValidation)
		
		// Session management
		v1.GET("/sessions", g.handleGetSessions)
		v1.POST("/sessions", g.handleCreateSession) 
		v1.GET("/sessions/:id", g.handleGetSession)
		v1.DELETE("/sessions/:id", g.handleDeleteSession)
		v1.POST("/sessions/:id/rotate", g.handleRotateSession)
		
		// Analytics
		v1.GET("/analytics/metrics", g.handleGetMetrics)
		v1.GET("/analytics/hourly", g.handleGetHourlyReport)
		v1.GET("/analytics/destinations", g.handleGetDestinations)
		v1.GET("/analytics/system", g.handleGetSystemStats)
		
		// Proxy stats
		v1.GET("/proxy/stats", g.handleGetProxyStats)
		v1.GET("/proxy/connections", g.handleGetConnections)
		
		// Node management
		v1.GET("/nodes", g.handleGetNodes)
		v1.GET("/nodes/:id", g.handleGetNode)
		
		// Configuration
		v1.GET("/profiles", g.handleGetProfiles)
		v1.POST("/profiles", g.handleCreateProfile)
	}

	g.apiServer = router
}

// Authentication validation endpoint
func (g *EnhancedProxyGateway) handleAuthValidation(c *gin.Context) {
	var req struct {
		AuthString string `json:"auth_string"`
		ClientIP   string `json:"client_ip"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	auth, err := g.authenticator.ParseEnhancedAuth(req.AuthString, req.ClientIP)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"valid": true,
		"customer": auth.Customer,
		"method": auth.Method,
		"country": auth.Country,
		"session_type": auth.SessionType,
	})
}

// Session management endpoints
func (g *EnhancedProxyGateway) handleGetSessions(c *gin.Context) {
	customerID := c.Query("customer_id")
	if customerID == "" {
		c.JSON(400, gin.H{"error": "customer_id required"})
		return
	}

	// In a real implementation, you'd query active sessions
	c.JSON(200, gin.H{
		"sessions": []interface{}{},
		"total": 0,
	})
}

func (g *EnhancedProxyGateway) handleCreateSession(c *gin.Context) {
	var req struct {
		CustomerID    string `json:"customer_id"`
		Country       string `json:"country"`
		City          string `json:"city"`
		SessionType   string `json:"session_type"`
		Lifetime      string `json:"lifetime"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	// Create enhanced auth object
	customer := &auth.Customer{ID: req.CustomerID} // In production, validate customer
	enhancedAuth := &auth.EnhancedProxyAuth{
		Customer:    customer,
		Country:     req.Country,
		City:        req.City,
		SessionType: req.SessionType,
		Lifetime:    30 * time.Minute, // Parse req.Lifetime
	}

	session, err := g.sessionManager.GetOrCreateSession(enhancedAuth)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"session": session,
		"created": true,
	})
}

func (g *EnhancedProxyGateway) handleGetSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	session, err := g.sessionManager.GetSessionStats(sessionID)
	if err != nil {
		c.JSON(404, gin.H{"error": "session not found"})
		return
	}

	c.JSON(200, gin.H{"session": session})
}

func (g *EnhancedProxyGateway) handleDeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	err := g.sessionManager.TerminateSession(sessionID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"deleted": true})
}

func (g *EnhancedProxyGateway) handleRotateSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	// Implementation would rotate the node for this session
	c.JSON(200, gin.H{
		"rotated": true,
		"session_id": sessionID,
		"timestamp": time.Now(),
	})
}

// Analytics endpoints
func (g *EnhancedProxyGateway) handleGetMetrics(c *gin.Context) {
	customerID := c.Query("customer_id")
	if customerID == "" {
		c.JSON(400, gin.H{"error": "customer_id required"})
		return
	}

	metrics, err := g.analytics.GetCustomerMetrics(customerID)
	if err != nil {
		c.JSON(404, gin.H{"error": "metrics not found"})
		return
	}

	c.JSON(200, metrics)
}

func (g *EnhancedProxyGateway) handleGetHourlyReport(c *gin.Context) {
	customerID := c.Query("customer_id")
	hoursStr := c.DefaultQuery("hours", "24")
	
	hours, err := strconv.Atoi(hoursStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid hours parameter"})
		return
	}

	report, err := g.analytics.GetHourlyReport(customerID, hours)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"report": report,
		"customer_id": customerID,
		"hours": hours,
	})
}

func (g *EnhancedProxyGateway) handleGetDestinations(c *gin.Context) {
	customerID := c.Query("customer_id")
	limitStr := c.DefaultQuery("limit", "10")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid limit parameter"})
		return
	}

	destinations, err := g.analytics.GetTopDestinations(customerID, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"destinations": destinations})
}

func (g *EnhancedProxyGateway) handleGetSystemStats(c *gin.Context) {
	stats, err := g.analytics.GetSystemStats()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, stats)
}

// Proxy statistics endpoints
func (g *EnhancedProxyGateway) handleGetProxyStats(c *gin.Context) {
	socks5Stats := g.socks5Proxy.GetConnectionStats()
	
	c.JSON(200, gin.H{
		"socks5": socks5Stats,
		"timestamp": time.Now(),
	})
}

func (g *EnhancedProxyGateway) handleGetConnections(c *gin.Context) {
	socks5Stats := g.socks5Proxy.GetConnectionStats()
	
	c.JSON(200, gin.H{
		"active_connections": socks5Stats["active_connections"],
		"connections": socks5Stats["connections"],
	})
}

// Node management endpoints
func (g *EnhancedProxyGateway) handleGetNodes(c *gin.Context) {
	// Implementation would return available nodes
	c.JSON(200, gin.H{
		"nodes": []interface{}{},
		"total": 0,
	})
}

func (g *EnhancedProxyGateway) handleGetNode(c *gin.Context) {
	nodeID := c.Param("id")
	
	// Implementation would return specific node details
	c.JSON(200, gin.H{
		"node_id": nodeID,
		"status": "online",
	})
}

// Profile management endpoints
func (g *EnhancedProxyGateway) handleGetProfiles(c *gin.Context) {
	profiles := g.headerManager.GetProfileList()
	
	c.JSON(200, gin.H{
		"profiles": profiles,
	})
}

func (g *EnhancedProxyGateway) handleCreateProfile(c *gin.Context) {
	var profile headers.BrowserProfile
	
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(400, gin.H{"error": "invalid profile"})
		return
	}

	g.headerManager.AddCustomProfile(profile.Name, profile)
	
	c.JSON(200, gin.H{
		"created": true,
		"profile": profile.Name,
	})
}

func (g *EnhancedProxyGateway) WaitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	sig := <-sigChan
	g.logger.Infof("Received signal %v, shutting down gracefully...", sig)
	
	// Close database connections
	if g.db != nil {
		g.db.Close()
	}
	
	// Close Redis connections
	if g.rdb != nil {
		g.rdb.Close()
	}
	
	g.logger.Infof("Enhanced Proxy Gateway shutdown complete")
}