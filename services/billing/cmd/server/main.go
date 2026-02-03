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

	"billing/internal/stripe"
	"billing/internal/plans"
	"billing/internal/usage"
	"billing/internal/webhooks"
)

func main() {
	godotenv.Load()

	// Setup logger
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logger := logrus.WithField("service", "billing")

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

	// Redis connection
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
	stripeClient := stripe.NewClient()
	planService := plans.NewPlanService(db)
	usageTracker := usage.NewTracker(db, rdb)
	webhookHandler := webhooks.NewHandler(db, os.Getenv("STRIPE_WEBHOOK_SECRET"), logger)

	// Setup HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "billing",
		})
	})

	// Plans endpoints
	router.GET("/plans", func(c *gin.Context) {
		plans, err := planService.GetAll(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, plans)
	})

	router.GET("/plans/:id", func(c *gin.Context) {
		plan, err := planService.GetByID(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
			return
		}
		c.JSON(http.StatusOK, plan)
	})

	// Checkout endpoints
	router.POST("/checkout/session", func(c *gin.Context) {
		var req struct {
			CustomerID string `json:"customer_id" binding:"required"`
			PriceID    string `json:"price_id" binding:"required"`
			SuccessURL string `json:"success_url" binding:"required"`
			CancelURL  string `json:"cancel_url" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		session, err := stripeClient.CreateCheckoutSession(
			req.CustomerID, req.PriceID, req.SuccessURL, req.CancelURL,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"session_id":   session.ID,
			"checkout_url": session.URL,
		})
	})

	// Usage endpoints
	router.GET("/usage/:customer_id", func(c *gin.Context) {
		summary, err := usageTracker.GetCurrentUsage(c.Request.Context(), c.Param("customer_id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, summary)
	})

	router.GET("/usage/:customer_id/daily", func(c *gin.Context) {
		daily, err := usageTracker.GetDailyUsage(c.Request.Context(), c.Param("customer_id"), 30)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, daily)
	})

	// Subscription management
	router.GET("/subscription/:customer_id", func(c *gin.Context) {
		// Get subscription from database
		var sub struct {
			ID                 string    `json:"id"`
			Status             string    `json:"status"`
			PlanID             string    `json:"plan_id"`
			CurrentPeriodStart time.Time `json:"current_period_start"`
			CurrentPeriodEnd   time.Time `json:"current_period_end"`
			CancelAtPeriodEnd  bool      `json:"cancel_at_period_end"`
		}

		query := `
			SELECT stripe_subscription_id, subscription_status, plan_id,
				current_period_start, current_period_end, cancel_at_period_end
			FROM customers WHERE id = $1
		`

		err := db.QueryRowContext(c.Request.Context(), query, c.Param("customer_id")).Scan(
			&sub.ID, &sub.Status, &sub.PlanID,
			&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CancelAtPeriodEnd,
		)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}

		c.JSON(http.StatusOK, sub)
	})

	router.POST("/subscription/:subscription_id/cancel", func(c *gin.Context) {
		var req struct {
			Immediately bool `json:"immediately"`
		}
		c.ShouldBindJSON(&req)

		_, err := stripeClient.CancelSubscription(c.Param("subscription_id"), req.Immediately)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "canceled"})
	})

	// Invoices
	router.GET("/invoices/:customer_id", func(c *gin.Context) {
		// Get Stripe customer ID
		var stripeCustomerID string
		err := db.QueryRowContext(c.Request.Context(),
			"SELECT stripe_customer_id FROM customers WHERE id = $1",
			c.Param("customer_id"),
		).Scan(&stripeCustomerID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
			return
		}

		invoices, err := stripeClient.ListInvoices(stripeCustomerID, 10)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, invoices)
	})

	// Stripe webhook
	router.POST("/webhook/stripe", webhookHandler.HandleWebhook)

	// Internal endpoints for other services
	router.POST("/internal/record-usage", func(c *gin.Context) {
		var record usage.UsageRecord
		if err := c.ShouldBindJSON(&record); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		record.Timestamp = time.Now()
		record.BillingPeriod = time.Now().Format("2006-01")

		if err := usageTracker.RecordUsage(c.Request.Context(), &record); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "recorded"})
	})

	router.GET("/internal/check-quota/:customer_id", func(c *gin.Context) {
		// Get plan limit
		var planLimitGB int64
		err := db.QueryRowContext(c.Request.Context(),
			`SELECT p.bandwidth_gb FROM customers c 
			 JOIN plans p ON c.plan_id = p.id WHERE c.id = $1`,
			c.Param("customer_id"),
		).Scan(&planLimitGB)
		if err != nil {
			planLimitGB = 5 // Default 5GB
		}

		planLimitBytes := planLimitGB * 1024 * 1024 * 1024

		withinQuota, usagePercent, err := usageTracker.CheckQuota(
			c.Request.Context(),
			c.Param("customer_id"),
			planLimitBytes,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"within_quota":  withinQuota,
			"usage_percent": usagePercent,
		})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8003"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		logger.Infof("Billing service starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down billing service...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	logger.Info("Billing service stopped")
}
