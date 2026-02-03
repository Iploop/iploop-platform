package stripe

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/invoice"
	"github.com/stripe/stripe-go/v76/paymentmethod"
	"github.com/stripe/stripe-go/v76/subscription"
	"github.com/stripe/stripe-go/v76/usagerecord"
)

// Client wraps Stripe API operations
type Client struct {
	webhookSecret string
}

// NewClient creates a new Stripe client
func NewClient() *Client {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	
	return &Client{
		webhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
}

// CreateCustomer creates a new Stripe customer
func (c *Client) CreateCustomer(email, name string, metadata map[string]string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	}
	
	for k, v := range metadata {
		params.AddMetadata(k, v)
	}
	
	return customer.New(params)
}

// GetCustomer retrieves a customer by ID
func (c *Client) GetCustomer(customerID string) (*stripe.Customer, error) {
	return customer.Get(customerID, nil)
}

// UpdateCustomer updates customer details
func (c *Client) UpdateCustomer(customerID string, params *stripe.CustomerParams) (*stripe.Customer, error) {
	return customer.Update(customerID, params)
}

// CreateCheckoutSession creates a checkout session for subscription
func (c *Client) CreateCheckoutSession(customerID, priceID, successURL, cancelURL string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"customer_id": customerID,
			},
		},
	}
	
	return session.New(params)
}

// CreateUsageBasedCheckout creates checkout for usage-based billing
func (c *Client) CreateUsageBasedCheckout(customerID, priceID, successURL, cancelURL string) (*stripe.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price: stripe.String(priceID),
			},
		},
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
	}
	
	return session.New(params)
}

// CreateSubscription creates a subscription directly
func (c *Client) CreateSubscription(customerID, priceID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(priceID),
			},
		},
	}
	
	return subscription.New(params)
}

// GetSubscription retrieves a subscription
func (c *Client) GetSubscription(subscriptionID string) (*stripe.Subscription, error) {
	return subscription.Get(subscriptionID, nil)
}

// CancelSubscription cancels a subscription
func (c *Client) CancelSubscription(subscriptionID string, immediately bool) (*stripe.Subscription, error) {
	if immediately {
		return subscription.Cancel(subscriptionID, nil)
	}
	
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}
	return subscription.Update(subscriptionID, params)
}

// ReportUsage reports usage for metered billing
func (c *Client) ReportUsage(subscriptionItemID string, quantity int64, timestamp int64) (*stripe.UsageRecord, error) {
	params := &stripe.UsageRecordParams{
		Quantity:         stripe.Int64(quantity),
		Timestamp:        stripe.Int64(timestamp),
		SubscriptionItem: stripe.String(subscriptionItemID),
		Action:           stripe.String(string(stripe.UsageRecordActionIncrement)),
	}
	
	return usagerecord.New(params)
}

// ListInvoices lists invoices for a customer
func (c *Client) ListInvoices(customerID string, limit int64) ([]*stripe.Invoice, error) {
	params := &stripe.InvoiceListParams{
		Customer: stripe.String(customerID),
	}
	params.Limit = stripe.Int64(limit)
	
	var invoices []*stripe.Invoice
	iter := invoice.List(params)
	for iter.Next() {
		invoices = append(invoices, iter.Invoice())
	}
	
	return invoices, iter.Err()
}

// GetUpcomingInvoice gets the upcoming invoice for a customer
func (c *Client) GetUpcomingInvoice(customerID string) (*stripe.Invoice, error) {
	params := &stripe.InvoiceUpcomingParams{
		Customer: stripe.String(customerID),
	}
	return invoice.Upcoming(params)
}

// AttachPaymentMethod attaches a payment method to a customer
func (c *Client) AttachPaymentMethod(paymentMethodID, customerID string) (*stripe.PaymentMethod, error) {
	params := &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(customerID),
	}
	return paymentmethod.Attach(paymentMethodID, params)
}

// SetDefaultPaymentMethod sets the default payment method for a customer
func (c *Client) SetDefaultPaymentMethod(customerID, paymentMethodID string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(paymentMethodID),
		},
	}
	return customer.Update(customerID, params)
}

// VerifyWebhookSignature verifies webhook signature
func (c *Client) VerifyWebhookSignature(payload []byte, signature string) (stripe.Event, error) {
	return stripe.ConstructEvent(payload, signature, c.webhookSecret)
}

// ParseWebhookEvent parses a webhook event
func (c *Client) ParseWebhookEvent(payload []byte) (*stripe.Event, error) {
	var event stripe.Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %v", err)
	}
	return &event, nil
}
