-- IPLoop Credit/Earnings System Schema
-- PostgreSQL 15+

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Enums
CREATE TYPE platform_type AS ENUM ('mac', 'linux', 'windows', 'docker', 'android', 'firetv', 'androidtv');
CREATE TYPE device_status AS ENUM ('active', 'paused', 'offline');
CREATE TYPE credit_type AS ENUM ('earned', 'spent', 'bonus', 'expired');
CREATE TYPE multiplier_type AS ENUM ('multi_device', 'uptime_24h', 'rare_geo');

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE users (
    user_id       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         TEXT NOT NULL UNIQUE,
    api_key       TEXT NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(32), 'hex'),
    sdk_key       TEXT NOT NULL UNIQUE DEFAULT encode(gen_random_bytes(32), 'hex'),
    vpn_enabled   BOOLEAN NOT NULL DEFAULT FALSE,
    credit_balance BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_api_key ON users (api_key);
CREATE INDEX idx_users_sdk_key ON users (sdk_key);

-- ============================================================
-- DEVICES
-- ============================================================
CREATE TABLE devices (
    device_id     UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    platform      platform_type NOT NULL,
    os_version    TEXT,
    ip_country    CHAR(2),
    ip_city       TEXT,
    first_seen    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen     TIMESTAMPTZ NOT NULL DEFAULT now(),
    status        device_status NOT NULL DEFAULT 'offline'
);

CREATE INDEX idx_devices_user ON devices (user_id);
CREATE INDEX idx_devices_status ON devices (status);
CREATE INDEX idx_devices_country ON devices (ip_country);

-- ============================================================
-- BANDWIDTH CONTRIBUTIONS
-- ============================================================
CREATE TABLE bandwidth_contributions (
    id                      BIGSERIAL PRIMARY KEY,
    device_id               UUID NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
    user_id                 UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    ts                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    bytes_shared            BIGINT NOT NULL CHECK (bytes_shared >= 0),
    session_duration_seconds INTEGER NOT NULL CHECK (session_duration_seconds >= 0),
    country                 CHAR(2)
);

CREATE INDEX idx_bw_user_ts ON bandwidth_contributions (user_id, ts DESC);
CREATE INDEX idx_bw_device_ts ON bandwidth_contributions (device_id, ts DESC);

-- ============================================================
-- CREDITS LEDGER (append-only)
-- ============================================================
CREATE TABLE credits_ledger (
    id                BIGSERIAL PRIMARY KEY,
    user_id           UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    type              credit_type NOT NULL,
    amount            BIGINT NOT NULL, -- positive for earned/bonus, negative for spent/expired
    reason            TEXT,
    related_device_id UUID REFERENCES devices(device_id) ON DELETE SET NULL,
    related_request_id UUID,
    ts                TIMESTAMPTZ NOT NULL DEFAULT now(),
    balance_after     BIGINT NOT NULL
);

CREATE INDEX idx_ledger_user_ts ON credits_ledger (user_id, ts DESC);
CREATE INDEX idx_ledger_type ON credits_ledger (type);

-- ============================================================
-- PROXY USAGE
-- ============================================================
CREATE TABLE proxy_usage (
    request_id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    ts                TIMESTAMPTZ NOT NULL DEFAULT now(),
    target_url_hash   TEXT,
    exit_country      CHAR(2),
    exit_city         TEXT,
    bytes_transferred BIGINT NOT NULL DEFAULT 0,
    credits_spent     BIGINT NOT NULL DEFAULT 0,
    session_id        UUID
);

CREATE INDEX idx_proxy_user_ts ON proxy_usage (user_id, ts DESC);
CREATE INDEX idx_proxy_session ON proxy_usage (session_id);

-- ============================================================
-- MULTIPLIERS CONFIG
-- ============================================================
CREATE TABLE multiplier_rules (
    id              SERIAL PRIMARY KEY,
    mtype           multiplier_type NOT NULL UNIQUE,
    multiplier      NUMERIC(4,2) NOT NULL,
    description     TEXT,
    min_threshold   JSONB, -- e.g. {"devices": 3} or {"uptime_hours": 24}
    active          BOOLEAN NOT NULL DEFAULT TRUE
);

-- Seed default multipliers
INSERT INTO multiplier_rules (mtype, multiplier, description, min_threshold) VALUES
    ('multi_device', 2.0,  '3+ active devices = 2x credits',    '{"min_devices": 3}'),
    ('uptime_24h',   1.5,  '24h continuous uptime = 1.5x bonus', '{"uptime_hours": 24}'),
    ('rare_geo',     3.0,  'Rare country bonus up to 3x',        '{"tier": "rare"}');

-- ============================================================
-- RARE GEO TIERS
-- ============================================================
CREATE TABLE geo_multipliers (
    country     CHAR(2) PRIMARY KEY,
    multiplier  NUMERIC(4,2) NOT NULL DEFAULT 1.0,
    tier        TEXT -- 'common', 'uncommon', 'rare'
);

-- ============================================================
-- CREDIT RATES CONFIG
-- ============================================================
CREATE TABLE credit_rates (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    credits_per_unit NUMERIC(10,2) NOT NULL,
    unit            TEXT NOT NULL,
    active          BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO credit_rates (name, credits_per_unit, unit) VALUES
    ('bandwidth_share', 100,  '1 GB shared'),
    ('proxy_request',   1,    '1 proxy request');

-- ============================================================
-- VPN ACCESS VIEW
-- Users with at least one active device get VPN
-- ============================================================
CREATE OR REPLACE VIEW user_vpn_status AS
SELECT
    u.user_id,
    u.email,
    COUNT(d.device_id) FILTER (WHERE d.status = 'active') AS active_devices,
    COUNT(d.device_id) FILTER (WHERE d.status = 'active') > 0 AS vpn_eligible
FROM users u
LEFT JOIN devices d ON d.user_id = u.user_id
GROUP BY u.user_id, u.email;

-- ============================================================
-- HELPER: Calculate user multiplier
-- ============================================================
CREATE OR REPLACE FUNCTION get_user_multiplier(p_user_id UUID)
RETURNS NUMERIC AS $$
DECLARE
    v_mult NUMERIC := 1.0;
    v_active_devices INT;
BEGIN
    SELECT COUNT(*) INTO v_active_devices
    FROM devices WHERE user_id = p_user_id AND status = 'active';

    -- Multi-device bonus
    IF v_active_devices >= 3 THEN
        v_mult := v_mult * 2.0;
    END IF;

    RETURN v_mult;
END;
$$ LANGUAGE plpgsql STABLE;

-- ============================================================
-- HELPER: Credit a user (atomic ledger insert + balance update)
-- ============================================================
CREATE OR REPLACE FUNCTION credit_user(
    p_user_id UUID,
    p_type credit_type,
    p_amount BIGINT,
    p_reason TEXT DEFAULT NULL,
    p_device_id UUID DEFAULT NULL,
    p_request_id UUID DEFAULT NULL
) RETURNS BIGINT AS $$
DECLARE
    v_new_balance BIGINT;
BEGIN
    UPDATE users SET credit_balance = credit_balance + p_amount, updated_at = now()
    WHERE user_id = p_user_id
    RETURNING credit_balance INTO v_new_balance;

    INSERT INTO credits_ledger (user_id, type, amount, reason, related_device_id, related_request_id, balance_after)
    VALUES (p_user_id, p_type, p_amount, p_reason, p_device_id, p_request_id, v_new_balance);

    -- Update VPN flag
    UPDATE users SET vpn_enabled = EXISTS(
        SELECT 1 FROM devices WHERE user_id = p_user_id AND status = 'active'
    ) WHERE user_id = p_user_id;

    RETURN v_new_balance;
END;
$$ LANGUAGE plpgsql;
