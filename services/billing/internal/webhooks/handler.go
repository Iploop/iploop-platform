package webhooks

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v76"
)

// Handler handles Stripe webhooks
type Handler struct {
	db            *sql.DB
	webhookSecret string
	logger        *logrus.Entry
}

// NewHandler creates a new webhook handler
func NewHandler(db *sql.DB, webhookSecret string, logger *logrus.Entry) *Handler {
	return &Handler{
		db:            db,
		webhookSecret: webhookSecret,
		logger:        logger.WithField("component", "webhooks"),
	}
}

// HandleWebhook processes incoming Stripe webhooks
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Errorf("Error reading request body: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	
	// Verify webhook signature
	sigHeader := r.Header.Get("Stripe-Signature")
	event, err := stripe.ConstructEvent(payload, sigHeader, h.webhookSecret)
	if err != nil {
		h.logger.Errorf("Webhook signature verification failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	h.logger.Infof("Received webhook event: %s", event.Type)
	
	// Handle event types
	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(event)
		
	case "customer.subscription.created":
		h.handleSubscriptionCreated(event)
		
	case "customer.subscription.updated":
		h.handleSubscriptionUpdated(event)
		
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(event)
		
	case "invoice.paid":
		h.handleInvoicePaid(event)
		
	case "invoice.payment_failed":
		h.handleInvoicePaymentFailed(event)
		
	case "customer.created":
		h.handleCustomerCreated(event)
		
	case "customer.updated":
		h.handleCustomerUpdated(event)
		
	default:
		h.logger.Debugf("Unhandled event type: %s", event.Type)
	}
	
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleCheckoutCompleted(event stripe.Event) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		h.logger.Errorf("Error parsing checkout session: %v", err)
		return
	}
	
	h.logger.Infof("Checkout completed for customer %s", session.Customer.ID)
	
	// Update customer record with subscription info
	ctx := context.Background()
	query := `
		UPDATE customers 
		SET stripe_subscription_id = $1, 
			subscription_status = 'active',
			updated_at = NOW()
		WHERE stripe_customer_id = $2
	`
	
	_, err := h.db.ExecContext(ctx, query, session.Subscription.ID, session.Customer.ID)
	if err != nil {
		h.logger.Errorf("Error updating customer subscription: %v", err)
	}
}

func (h *Handler) handleSubscriptionCreated(event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		h.logger.Errorf("Error parsing subscription: %v", err)
		return
	}
	
	h.logger.Infof("Subscription created: %s for customer %s", sub.ID, sub.Customer.ID)
	
	ctx := context.Background()
	query := `
		UPDATE customers 
		SET stripe_subscription_id = $1,
			subscription_status = $2,
			plan_id = $3,
			current_period_start = $4,
			current_period_end = $5,
			updated_at = NOW()
		WHERE stripe_customer_id = $6
	`
	
	planID := ""
	if len(sub.Items.Data) > 0 {
		planID = sub.Items.Data[0].Price.ID
	}
	
	_, err := h.db.ExecContext(ctx, query,
		sub.ID,
		string(sub.Status),
		planID,
		time.Unix(sub.CurrentPeriodStart, 0),
		time.Unix(sub.CurrentPeriodEnd, 0),
		sub.Customer.ID,
	)
	if err != nil {
		h.logger.Errorf("Error updating subscription: %v", err)
	}
}

func (h *Handler) handleSubscriptionUpdated(event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		h.logger.Errorf("Error parsing subscription: %v", err)
		return
	}
	
	h.logger.Infof("Subscription updated: %s status=%s", sub.ID, sub.Status)
	
	ctx := context.Background()
	query := `
		UPDATE customers 
		SET subscription_status = $1,
			current_period_start = $2,
			current_period_end = $3,
			cancel_at_period_end = $4,
			updated_at = NOW()
		WHERE stripe_subscription_id = $5
	`
	
	_, err := h.db.ExecContext(ctx, query,
		string(sub.Status),
		time.Unix(sub.CurrentPeriodStart, 0),
		time.Unix(sub.CurrentPeriodEnd, 0),
		sub.CancelAtPeriodEnd,
		sub.ID,
	)
	if err != nil {
		h.logger.Errorf("Error updating subscription status: %v", err)
	}
}

func (h *Handler) handleSubscriptionDeleted(event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		h.logger.Errorf("Error parsing subscription: %v", err)
		return
	}
	
	h.logger.Infof("Subscription deleted: %s", sub.ID)
	
	ctx := context.Background()
	query := `
		UPDATE customers 
		SET subscription_status = 'canceled',
			stripe_subscription_id = NULL,
			updated_at = NOW()
		WHERE stripe_subscription_id = $1
	`
	
	_, err := h.db.ExecContext(ctx, query, sub.ID)
	if err != nil {
		h.logger.Errorf("Error updating subscription deletion: %v", err)
	}
}

func (h *Handler) handleInvoicePaid(event stripe.Event) {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		h.logger.Errorf("Error parsing invoice: %v", err)
		return
	}
	
	h.logger.Infof("Invoice paid: %s for customer %s, amount: %d", inv.ID, inv.Customer.ID, inv.AmountPaid)
	
	// Record payment in database
	ctx := context.Background()
	query := `
		INSERT INTO payments (
			stripe_invoice_id, stripe_customer_id, amount, currency, status, paid_at
		) VALUES ($1, $2, $3, $4, 'paid', NOW())
		ON CONFLICT (stripe_invoice_id) DO UPDATE SET status = 'paid', paid_at = NOW()
	`
	
	_, err := h.db.ExecContext(ctx, query, inv.ID, inv.Customer.ID, inv.AmountPaid, inv.Currency)
	if err != nil {
		h.logger.Errorf("Error recording payment: %v", err)
	}
}

func (h *Handler) handleInvoicePaymentFailed(event stripe.Event) {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		h.logger.Errorf("Error parsing invoice: %v", err)
		return
	}
	
	h.logger.Warnf("Invoice payment failed: %s for customer %s", inv.ID, inv.Customer.ID)
	
	// Update customer status
	ctx := context.Background()
	query := `
		UPDATE customers 
		SET subscription_status = 'past_due',
			updated_at = NOW()
		WHERE stripe_customer_id = $1
	`
	
	_, err := h.db.ExecContext(ctx, query, inv.Customer.ID)
	if err != nil {
		h.logger.Errorf("Error updating customer status: %v", err)
	}
	
	// TODO: Send notification email
}

func (h *Handler) handleCustomerCreated(event stripe.Event) {
	var cust stripe.Customer
	if err := json.Unmarshal(event.Data.Raw, &cust); err != nil {
		h.logger.Errorf("Error parsing customer: %v", err)
		return
	}
	
	h.logger.Infof("Stripe customer created: %s (%s)", cust.ID, cust.Email)
}

func (h *Handler) handleCustomerUpdated(event stripe.Event) {
	var cust stripe.Customer
	if err := json.Unmarshal(event.Data.Raw, &cust); err != nil {
		h.logger.Errorf("Error parsing customer: %v", err)
		return
	}
	
	h.logger.Infof("Stripe customer updated: %s", cust.ID)
	
	// Sync email changes
	ctx := context.Background()
	query := `
		UPDATE customers 
		SET email = $1, updated_at = NOW()
		WHERE stripe_customer_id = $2
	`
	
	_, err := h.db.ExecContext(ctx, query, cust.Email, cust.ID)
	if err != nil {
		h.logger.Errorf("Error syncing customer email: %v", err)
	}
}
