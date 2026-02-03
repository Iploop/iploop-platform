package main

import (
	"context"
	"database/sql"
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

	"node-registration/internal/config"
	"node-registration/internal/websocket"
	"node-registration/internal/nodemanager"
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
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL[8:], // Remove redis:// prefix
		Password: "",
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

	// Setup HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// WebSocket endpoint for node connections
	router.GET("/ws", func(c *gin.Context) {
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

		c.JSON(http.StatusOK, gin.H{
			"status":         "healthy",
			"timestamp":      time.Now().UTC(),
			"service":        "node-registration",
			"connected_nodes": hub.GetConnectedCount(),
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

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
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