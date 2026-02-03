package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// EventType represents webhook event types
type EventType string

const (
	EventNodeOnline       EventType = "node.online"
	EventNodeOffline      EventType = "node.offline"
	EventRequestCompleted EventType = "request.completed"
	EventRequestFailed    EventType = "request.failed"
	EventQuotaWarning     EventType = "quota.warning"
	EventQuotaExceeded    EventType = "quota.exceeded"
	EventAPIKeyCreated    EventType = "api_key.created"
	EventAPIKeyDeleted    EventType = "api_key.deleted"
	EventPaymentSuccess   EventType = "payment.success"
	EventPaymentFailed    EventType = "payment.failed"
)

// WebhookEvent represents a webhook event payload
type WebhookEvent struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Webhook represents a customer's webhook configuration
type Webhook struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	URL        string    `json:"url"`
	Secret     string    `json:"secret"`
	Events     []string  `json:"events"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

// DeliveryAttempt represents a webhook delivery attempt
type DeliveryAttempt struct {
	ID           string    `json:"id"`
	WebhookID    string    `json:"webhook_id"`
	EventID      string    `json:"event_id"`
	URL          string    `json:"url"`
	StatusCode   int       `json:"status_code"`
	ResponseBody string    `json:"response_body"`
	Error        string    `json:"error"`
	Duration     int64     `json:"duration_ms"`
	CreatedAt    time.Time `json:"created_at"`
}

// Dispatcher handles webhook dispatching
type Dispatcher struct {
	db     *sql.DB
	logger *logrus.Entry
	client *http.Client
}

// NewDispatcher creates a new webhook dispatcher
func NewDispatcher(db *sql.DB, logger *logrus.Entry) *Dispatcher {
	return &Dispatcher{
		db:     db,
		logger: logger.WithField("component", "webhooks"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Dispatch sends a webhook event to all subscribed endpoints
func (d *Dispatcher) Dispatch(ctx context.Context, customerID string, eventType EventType, data map[string]interface{}) error {
	// Get webhooks for this customer that subscribe to this event
	webhooks, err := d.getWebhooksForEvent(ctx, customerID, eventType)
	if err != nil {
		return fmt.Errorf("failed to get webhooks: %v", err)
	}

	if len(webhooks) == 0 {
		return nil // No webhooks configured
	}

	// Create event
	event := WebhookEvent{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	// Store event
	if err := d.storeEvent(ctx, customerID, event); err != nil {
		d.logger.Errorf("Failed to store event: %v", err)
	}

	// Dispatch to each webhook
	for _, webhook := range webhooks {
		go d.deliverWebhook(webhook, event)
	}

	return nil
}

func (d *Dispatcher) getWebhooksForEvent(ctx context.Context, customerID string, eventType EventType) ([]Webhook, error) {
	query := `
		SELECT id, customer_id, url, secret, events, active, created_at
		FROM webhooks
		WHERE customer_id = $1 AND active = true
	`

	rows, err := d.db.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var w Webhook
		var eventsJSON []byte
		if err := rows.Scan(&w.ID, &w.CustomerID, &w.URL, &w.Secret, &eventsJSON, &w.Active, &w.CreatedAt); err != nil {
			continue
		}
		json.Unmarshal(eventsJSON, &w.Events)

		// Check if webhook subscribes to this event
		for _, e := range w.Events {
			if e == string(eventType) || e == "*" {
				webhooks = append(webhooks, w)
				break
			}
		}
	}

	return webhooks, nil
}

func (d *Dispatcher) deliverWebhook(webhook Webhook, event WebhookEvent) {
	start := time.Now()

	// Serialize event
	payload, err := json.Marshal(event)
	if err != nil {
		d.logger.Errorf("Failed to serialize event: %v", err)
		return
	}

	// Create request
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		d.logger.Errorf("Failed to create request: %v", err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "IPLoop-Webhooks/1.0")
	req.Header.Set("X-Webhook-ID", webhook.ID)
	req.Header.Set("X-Event-ID", event.ID)
	req.Header.Set("X-Event-Type", string(event.Type))
	req.Header.Set("X-Timestamp", event.Timestamp.Format(time.RFC3339))

	// Sign payload
	signature := d.signPayload(payload, webhook.Secret)
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Signature-256", "sha256="+signature)

	// Send request
	resp, err := d.client.Do(req)
	duration := time.Since(start).Milliseconds()

	attempt := DeliveryAttempt{
		ID:        uuid.New().String(),
		WebhookID: webhook.ID,
		EventID:   event.ID,
		URL:       webhook.URL,
		Duration:  duration,
		CreatedAt: time.Now(),
	}

	if err != nil {
		attempt.Error = err.Error()
		d.logger.Errorf("Webhook delivery failed: %v", err)
	} else {
		defer resp.Body.Close()
		attempt.StatusCode = resp.StatusCode

		// Read response body (limit to 1KB)
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		attempt.ResponseBody = string(body[:n])

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			d.logger.Infof("Webhook delivered successfully to %s", webhook.URL)
		} else {
			d.logger.Warnf("Webhook delivery got status %d from %s", resp.StatusCode, webhook.URL)
		}
	}

	// Store delivery attempt
	d.storeDeliveryAttempt(attempt)

	// Retry logic for failed deliveries
	if attempt.StatusCode == 0 || attempt.StatusCode >= 500 {
		go d.retryDelivery(webhook, event, 1)
	}
}

func (d *Dispatcher) retryDelivery(webhook Webhook, event WebhookEvent, attempt int) {
	maxRetries := 3
	if attempt > maxRetries {
		d.logger.Errorf("Max retries reached for webhook %s, event %s", webhook.ID, event.ID)
		return
	}

	// Exponential backoff: 1s, 4s, 9s
	delay := time.Duration(attempt*attempt) * time.Second
	time.Sleep(delay)

	d.logger.Infof("Retrying webhook delivery (attempt %d/%d)", attempt, maxRetries)
	d.deliverWebhook(webhook, event)
}

func (d *Dispatcher) signPayload(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func (d *Dispatcher) storeEvent(ctx context.Context, customerID string, event WebhookEvent) error {
	query := `
		INSERT INTO webhook_events (id, customer_id, type, data, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	
	dataJSON, _ := json.Marshal(event.Data)
	_, err := d.db.ExecContext(ctx, query, event.ID, customerID, event.Type, dataJSON, event.Timestamp)
	return err
}

func (d *Dispatcher) storeDeliveryAttempt(attempt DeliveryAttempt) {
	query := `
		INSERT INTO webhook_deliveries (id, webhook_id, event_id, url, status_code, response_body, error, duration_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	d.db.ExecContext(ctx, query,
		attempt.ID, attempt.WebhookID, attempt.EventID, attempt.URL,
		attempt.StatusCode, attempt.ResponseBody, attempt.Error,
		attempt.Duration, attempt.CreatedAt,
	)
}

// VerifySignature verifies a webhook signature (for documentation/SDK)
func VerifySignature(payload []byte, signature, secret string) bool {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	expected := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

// GetEventTypes returns all available event types
func GetEventTypes() []EventType {
	return []EventType{
		EventNodeOnline,
		EventNodeOffline,
		EventRequestCompleted,
		EventRequestFailed,
		EventQuotaWarning,
		EventQuotaExceeded,
		EventAPIKeyCreated,
		EventAPIKeyDeleted,
		EventPaymentSuccess,
		EventPaymentFailed,
	}
}
