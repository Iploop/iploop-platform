package main

import (
	"context"
	"database/sql"
	"encoding/json"
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

	"notifications/internal/email"
	"notifications/internal/webhooks"
)

func main() {
	godotenv.Load()

	// Setup logger
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logger := logrus.WithField("service", "notifications")

	// Database connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://iploop:iploop@localhost:5432/iploop?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Fatalf("Failed to ping database: %v", err)
	}
	logger.Info("Connected to database")

	// Redis connection for pub/sub
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisURL[8:],
	})
	defer rdb.Close()

	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		logger.Fatalf("Failed to connect to Redis: %v", err)
	}
	logger.Info("Connected to Redis")

	// Initialize services
	emailSender := email.NewSender()
	webhookDispatcher := webhooks.NewDispatcher(db, logger)

	// Subscribe to notification events from Redis
	go subscribeToEvents(ctx, rdb, emailSender, webhookDispatcher, logger)

	// Setup HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "notifications",
		})
	})

	// Send email endpoint (internal use)
	router.POST("/internal/email/send", func(c *gin.Context) {
		var req struct {
			To       string                 `json:"to" binding:"required"`
			Template string                 `json:"template" binding:"required"`
			Data     map[string]interface{} `json:"data"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := emailSender.Send(email.Email{
			To:       req.To,
			Subject:  getSubjectForTemplate(req.Template),
			Template: req.Template,
			Data:     req.Data,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "sent"})
	})

	// Send webhook endpoint (internal use)
	router.POST("/internal/webhook/dispatch", func(c *gin.Context) {
		var req struct {
			CustomerID string                 `json:"customer_id" binding:"required"`
			EventType  string                 `json:"event_type" binding:"required"`
			Data       map[string]interface{} `json:"data"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := webhookDispatcher.Dispatch(
			c.Request.Context(),
			req.CustomerID,
			webhooks.EventType(req.EventType),
			req.Data,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "dispatched"})
	})

	// Get available event types
	router.GET("/webhook/events", func(c *gin.Context) {
		events := webhooks.GetEventTypes()
		c.JSON(http.StatusOK, gin.H{"events": events})
	})

	// Test email endpoint
	router.POST("/test/email", func(c *gin.Context) {
		var req struct {
			To string `json:"to" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := emailSender.SendWelcome(req.To, "Test User"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "sent"})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8004"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		logger.Infof("Notification service starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down notification service...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	logger.Info("Notification service stopped")
}

func subscribeToEvents(ctx context.Context, rdb *redis.Client, emailSender *email.Sender, webhookDispatcher *webhooks.Dispatcher, logger *logrus.Entry) {
	pubsub := rdb.Subscribe(ctx, "notifications")
	defer pubsub.Close()

	ch := pubsub.Channel()

	logger.Info("Subscribed to notification events")

	for msg := range ch {
		var event struct {
			Type       string                 `json:"type"`
			CustomerID string                 `json:"customer_id"`
			Email      string                 `json:"email"`
			Name       string                 `json:"name"`
			Data       map[string]interface{} `json:"data"`
		}

		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			logger.Errorf("Failed to parse event: %v", err)
			continue
		}

		logger.Infof("Received notification event: %s", event.Type)

		// Handle event
		switch event.Type {
		case "welcome":
			go emailSender.SendWelcome(event.Email, event.Name)
			go webhookDispatcher.Dispatch(ctx, event.CustomerID, webhooks.EventType("customer.created"), event.Data)

		case "quota_warning":
			usagePercent := event.Data["usage_percent"].(float64)
			usedGB := event.Data["used_gb"].(float64)
			limitGB := event.Data["limit_gb"].(float64)
			go emailSender.SendQuotaWarning(event.Email, event.Name, usagePercent, usedGB, limitGB)
			go webhookDispatcher.Dispatch(ctx, event.CustomerID, webhooks.EventQuotaWarning, event.Data)

		case "payment_success":
			amount := event.Data["amount"].(float64)
			planName := event.Data["plan_name"].(string)
			invoiceID := event.Data["invoice_id"].(string)
			nextBilling := event.Data["next_billing_date"].(string)
			go emailSender.SendPaymentSuccess(event.Email, event.Name, amount, planName, invoiceID, nextBilling)
			go webhookDispatcher.Dispatch(ctx, event.CustomerID, webhooks.EventPaymentSuccess, event.Data)

		case "payment_failed":
			go emailSender.SendPaymentFailed(event.Email, event.Name)
			go webhookDispatcher.Dispatch(ctx, event.CustomerID, webhooks.EventPaymentFailed, event.Data)

		case "api_key_created":
			keyName := event.Data["key_name"].(string)
			apiKey := event.Data["api_key"].(string)
			go emailSender.SendAPIKeyCreated(event.Email, event.Name, keyName, apiKey)
			go webhookDispatcher.Dispatch(ctx, event.CustomerID, webhooks.EventAPIKeyCreated, event.Data)

		default:
			// For webhook-only events
			if event.CustomerID != "" {
				go webhookDispatcher.Dispatch(ctx, event.CustomerID, webhooks.EventType(event.Type), event.Data)
			}
		}
	}
}

func getSubjectForTemplate(template string) string {
	subjects := map[string]string{
		"welcome":         "Welcome to IPLoop! 🎉",
		"quota_warning":   "⚠️ IPLoop Quota Warning",
		"payment_success": "✅ Payment Received - IPLoop",
		"payment_failed":  "❌ Payment Failed - Action Required",
		"api_key_created": "🔑 New API Key Created - IPLoop",
	}

	if subject, ok := subjects[template]; ok {
		return subject
	}
	return "IPLoop Notification"
}
