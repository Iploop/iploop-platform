-- Billing related tables migration

-- Add Stripe fields to customers table
ALTER TABLE customers 
ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS stripe_subscription_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS subscription_status VARCHAR(50) DEFAULT 'none',
ADD COLUMN IF NOT EXISTS plan_id VARCHAR(50),
ADD COLUMN IF NOT EXISTS current_period_start TIMESTAMP,
ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMP,
ADD COLUMN IF NOT EXISTS cancel_at_period_end BOOLEAN DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_customers_stripe_customer_id ON customers(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_customers_stripe_subscription_id ON customers(stripe_subscription_id);
CREATE INDEX IF NOT EXISTS idx_customers_subscription_status ON customers(subscription_status);

-- Plans table
CREATE TABLE IF NOT EXISTS plans (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    type VARCHAR(20) DEFAULT 'subscription',
    stripe_price_id VARCHAR(255),
    stripe_price_id_annual VARCHAR(255),
    price_monthly INTEGER DEFAULT 0,
    price_annual INTEGER DEFAULT 0,
    bandwidth_gb INTEGER DEFAULT 0,
    requests_per_day INTEGER DEFAULT 0,
    concurrent_conns INTEGER DEFAULT 10,
    features JSONB DEFAULT '[]',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Usage records table (for long-term storage)
CREATE TABLE IF NOT EXISTS usage_records (
    id SERIAL PRIMARY KEY,
    customer_id VARCHAR(36) NOT NULL,
    api_key_id VARCHAR(36),
    bytes_transferred BIGINT DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    country VARCHAR(2),
    node_id VARCHAR(36),
    timestamp TIMESTAMP NOT NULL,
    billing_period VARCHAR(7) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_usage_records_customer_id ON usage_records(customer_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_billing_period ON usage_records(billing_period);
CREATE INDEX IF NOT EXISTS idx_usage_records_timestamp ON usage_records(timestamp);
CREATE INDEX IF NOT EXISTS idx_usage_records_customer_period ON usage_records(customer_id, billing_period);

-- Payments table
CREATE TABLE IF NOT EXISTS payments (
    id SERIAL PRIMARY KEY,
    stripe_invoice_id VARCHAR(255) UNIQUE,
    stripe_customer_id VARCHAR(255) NOT NULL,
    customer_id VARCHAR(36),
    amount INTEGER NOT NULL,
    currency VARCHAR(3) DEFAULT 'usd',
    status VARCHAR(50) DEFAULT 'pending',
    paid_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payments_customer_id ON payments(customer_id);
CREATE INDEX IF NOT EXISTS idx_payments_stripe_customer_id ON payments(stripe_customer_id);

-- Insert default plans
INSERT INTO plans (id, name, description, type, price_monthly, price_annual, bandwidth_gb, requests_per_day, concurrent_conns, features, is_active)
VALUES 
    ('starter', 'Starter', 'Perfect for testing and small projects', 'subscription', 4900, 47000, 5, 10000, 10, 
     '["5 GB bandwidth/month", "10,000 requests/day", "10 concurrent connections", "HTTP & SOCKS5 protocols", "Basic geo-targeting", "Email support"]', true),
    ('growth', 'Growth', 'For growing businesses', 'subscription', 14900, 143000, 25, 50000, 50,
     '["25 GB bandwidth/month", "50,000 requests/day", "50 concurrent connections", "HTTP & SOCKS5 protocols", "Advanced geo-targeting", "City-level targeting", "Priority support", "API access"]', true),
    ('business', 'Business', 'For serious data operations', 'subscription', 49900, 479000, 100, 200000, 200,
     '["100 GB bandwidth/month", "200,000 requests/day", "200 concurrent connections", "HTTP & SOCKS5 protocols", "Advanced geo-targeting", "City & ASN targeting", "Sticky sessions", "Dedicated account manager", "24/7 support", "SLA guarantee"]', true),
    ('payg', 'Pay As You Go', 'Pay only for what you use', 'usage', 0, 0, 0, 50000, 25,
     '["$5 per GB", "No monthly commitment", "50,000 requests/day", "25 concurrent connections", "HTTP & SOCKS5 protocols", "Basic geo-targeting", "Email support"]', true)
ON CONFLICT (id) DO NOTHING;

-- Node earnings table (for node operators)
CREATE TABLE IF NOT EXISTS node_earnings (
    id SERIAL PRIMARY KEY,
    node_id VARCHAR(36) NOT NULL,
    device_id VARCHAR(255),
    bytes_transferred BIGINT DEFAULT 0,
    requests_handled INTEGER DEFAULT 0,
    earnings_usd DECIMAL(10, 6) DEFAULT 0,
    period VARCHAR(7) NOT NULL,
    paid BOOLEAN DEFAULT FALSE,
    paid_at TIMESTAMP,
    payout_method VARCHAR(50),
    payout_destination VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_node_earnings_node_id ON node_earnings(node_id);
CREATE INDEX IF NOT EXISTS idx_node_earnings_period ON node_earnings(period);
CREATE INDEX IF NOT EXISTS idx_node_earnings_paid ON node_earnings(paid);

-- Withdrawal requests table
CREATE TABLE IF NOT EXISTS withdrawal_requests (
    id SERIAL PRIMARY KEY,
    node_id VARCHAR(36) NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,
    method VARCHAR(50) NOT NULL,
    destination VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    transaction_id VARCHAR(255),
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_withdrawals_node_id ON withdrawal_requests(node_id);
CREATE INDEX IF NOT EXISTS idx_withdrawals_status ON withdrawal_requests(status);
