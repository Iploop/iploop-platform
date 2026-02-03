-- IPLoop Platform Database Schema
-- Created: 2026-02-02

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    company VARCHAR(255),
    phone VARCHAR(20),
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    email_verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE
);

-- API Keys table
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    permissions JSONB DEFAULT '["proxy"]'::jsonb,
    is_active BOOLEAN DEFAULT TRUE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Plans table (for billing)
CREATE TABLE plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price_per_gb DECIMAL(10,4) NOT NULL,
    included_gb INTEGER DEFAULT 0,
    max_concurrent_connections INTEGER DEFAULT 10,
    features JSONB DEFAULT '{}'::jsonb,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- User Plans (subscriptions)
CREATE TABLE user_plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES plans(id),
    gb_balance DECIMAL(15,6) DEFAULT 0,
    gb_used DECIMAL(15,6) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'cancelled')),
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Nodes table (SDK devices)
CREATE TABLE nodes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    device_id VARCHAR(255) NOT NULL UNIQUE,
    ip_address INET NOT NULL,
    country VARCHAR(2) NOT NULL,
    country_name VARCHAR(100),
    city VARCHAR(100),
    region VARCHAR(100),
    latitude DECIMAL(10,8),
    longitude DECIMAL(11,8),
    asn INTEGER,
    isp VARCHAR(255),
    carrier VARCHAR(255),
    connection_type VARCHAR(20) CHECK (connection_type IN ('wifi', 'cellular', 'ethernet')),
    device_type VARCHAR(20) CHECK (device_type IN ('android', 'ios', 'windows', 'mac', 'browser')),
    sdk_version VARCHAR(20),
    status VARCHAR(20) DEFAULT 'available' CHECK (status IN ('available', 'busy', 'inactive', 'banned')),
    quality_score INTEGER DEFAULT 100 CHECK (quality_score >= 0 AND quality_score <= 100),
    bandwidth_used_mb BIGINT DEFAULT 0,
    total_requests BIGINT DEFAULT 0,
    successful_requests BIGINT DEFAULT 0,
    last_heartbeat TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    connected_since TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Usage Tracking
CREATE TABLE usage_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    node_id UUID REFERENCES nodes(id) ON DELETE SET NULL,
    session_id VARCHAR(255),
    bytes_downloaded BIGINT NOT NULL DEFAULT 0,
    bytes_uploaded BIGINT NOT NULL DEFAULT 0,
    total_bytes BIGINT GENERATED ALWAYS AS (bytes_downloaded + bytes_uploaded) STORED,
    request_count INTEGER DEFAULT 1,
    target_country VARCHAR(2),
    target_city VARCHAR(100),
    proxy_type VARCHAR(10) CHECK (proxy_type IN ('http', 'socks5')),
    success BOOLEAN DEFAULT TRUE,
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    ended_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER GENERATED ALWAYS AS (EXTRACT(milliseconds FROM (ended_at - started_at))) STORED
);

-- Billing Transactions
CREATE TABLE billing_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('purchase', 'usage', 'refund', 'bonus')),
    amount DECIMAL(15,6) NOT NULL, -- in USD
    gb_amount DECIMAL(15,6), -- GB purchased/used
    description TEXT,
    stripe_payment_id VARCHAR(255),
    status VARCHAR(20) DEFAULT 'completed' CHECK (status IN ('pending', 'completed', 'failed', 'refunded')),
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Node Sessions (for sticky IPs)
CREATE TABLE node_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_key VARCHAR(255) NOT NULL UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    target_country VARCHAR(2),
    target_city VARCHAR(100),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_nodes_device_id ON nodes(device_id);
CREATE INDEX idx_nodes_status ON nodes(status);
CREATE INDEX idx_nodes_country ON nodes(country);
CREATE INDEX idx_nodes_city ON nodes(country, city);
CREATE INDEX idx_nodes_heartbeat ON nodes(last_heartbeat);
CREATE INDEX idx_usage_user_id ON usage_records(user_id);
CREATE INDEX idx_usage_started_at ON usage_records(started_at);
CREATE INDEX idx_usage_node_id ON usage_records(node_id);
CREATE INDEX idx_billing_user_id ON billing_transactions(user_id);
CREATE INDEX idx_billing_created_at ON billing_transactions(created_at);
CREATE INDEX idx_sessions_key ON node_sessions(session_key);
CREATE INDEX idx_sessions_expires ON node_sessions(expires_at);

-- Insert default plans
INSERT INTO plans (name, description, price_per_gb, included_gb, max_concurrent_connections, features) VALUES
('Starter', 'Perfect for testing and small projects', 3.00, 0, 5, '{"countries": ["US", "UK", "DE"], "support": "email"}'),
('Professional', 'For growing businesses', 2.50, 20, 20, '{"countries": "all", "sticky_sessions": true, "support": "priority"}'),
('Enterprise', 'For high-volume applications', 2.00, 100, 100, '{"countries": "all", "sticky_sessions": true, "city_targeting": true, "support": "phone", "sla": "99.9%"}');

-- Create a default admin user (password: admin123)
INSERT INTO users (email, password_hash, first_name, last_name, company, status, email_verified) VALUES
('admin@iploop.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Admin', 'User', 'IPLoop', 'active', TRUE);

-- Create a test customer (password: test123)
INSERT INTO users (email, password_hash, first_name, last_name, company, status, email_verified) VALUES
('test@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Test', 'Customer', 'Test Corp', 'active', TRUE);

-- Give test customer a starter plan with 10GB
INSERT INTO user_plans (user_id, plan_id, gb_balance) 
SELECT u.id, p.id, 10.0 
FROM users u, plans p 
WHERE u.email = 'test@example.com' AND p.name = 'Starter';

-- Create some sample nodes for testing
INSERT INTO nodes (device_id, ip_address, country, country_name, city, region, asn, isp, connection_type, device_type, sdk_version, status, quality_score) VALUES
('device_us_1', '192.168.1.100', 'US', 'United States', 'New York', 'NY', 12345, 'Verizon', 'wifi', 'android', '1.0.0', 'available', 95),
('device_us_2', '192.168.1.101', 'US', 'United States', 'Los Angeles', 'CA', 12346, 'T-Mobile', 'cellular', 'ios', '1.0.0', 'available', 88),
('device_uk_1', '192.168.2.100', 'UK', 'United Kingdom', 'London', 'England', 54321, 'BT', 'wifi', 'android', '1.0.0', 'available', 92),
('device_de_1', '192.168.3.100', 'DE', 'Germany', 'Berlin', 'Berlin', 98765, 'Deutsche Telekom', 'wifi', 'windows', '1.0.0', 'available', 87);

-- Update timestamps function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add updated_at triggers
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_plans_updated_at BEFORE UPDATE ON plans FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_user_plans_updated_at BEFORE UPDATE ON user_plans FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_nodes_updated_at BEFORE UPDATE ON nodes FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_billing_updated_at BEFORE UPDATE ON billing_transactions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();