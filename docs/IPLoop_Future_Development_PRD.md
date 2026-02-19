# IPLoop + ProxyClaw — Future Development PRD

**Version:** 2.0  
**Date:** February 17, 2026  
**Author:** IPLoop Engineering  
**Status:** Comprehensive Future Development Plan  
**Audience:** Development Team, Product Managers, Stakeholders

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Current State Assessment](#2-current-state-assessment)
3. [ProxyClaw Node/Earn Platform](#3-proxyclaw-nodearn-platform)
4. [Referral & Affiliate System](#4-referral--affiliate-system)
5. [Billing & Payments (Stripe)](#5-billing--payments-stripe)
6. [SDK Partner Portal](#6-sdk-partner-portal)
7. [Advanced Proxy Features](#7-advanced-proxy-features)
8. [Analytics & Reporting](#8-analytics--reporting)
9. [Security Features](#9-security-features)
10. [Admin Panel Enhancements](#10-admin-panel-enhancements)
11. [Mobile Apps](#11-mobile-apps)
12. [API v2](#12-api-v2)
13. [White-Label / Reseller](#13-white-label--reseller)
14. [Marketing & Growth](#14-marketing--growth)
15. [Implementation Roadmap](#15-implementation-roadmap)
16. [Appendix: Competitor Analysis](#16-appendix-competitor-analysis)

---

## 1. Executive Summary

This PRD covers **all planned and future development** for the IPLoop proxy platform and its consumer-facing brand, **ProxyClaw**. IPLoop is the B2B proxy infrastructure; ProxyClaw is the node-operator/earn side where individuals share bandwidth for rewards.

### What Exists Today (Built)
- Proxy gateway (HTTP/SOCKS5) with geo-targeting, sticky sessions, browser profiles
- Node registration via WebSocket (10K+ peak nodes, 111 countries)
- Customer dashboard (Next.js 14): login, nodes, analytics, API keys, webhooks, docs, settings, admin
- Android SDK v1.0.57 (pure Java)
- Customer API (Node.js/Express) with JWT auth
- PostgreSQL + Redis backend
- Prometheus + Grafana monitoring
- Credit system schema designed (not deployed)
- Stripe service scaffolded (test keys configured)
- Email service scaffolded (Resend API)
- Mobile app scaffold (Expo/React Native)
- Earn landing pages (static HTML)

### What This PRD Covers (To Build)
Everything listed as 🔜 in the existing PRD, plus all new systems needed to scale from MVP to production-grade commercial platform.

### Priority Framework

| Priority | Definition | Timeline |
|---|---|---|
| **P0** | Critical — blocks revenue or core functionality | 0-4 weeks |
| **P1** | High — needed for commercial launch | 1-2 months |
| **P2** | Medium — improves product significantly | 2-4 months |
| **P3** | Nice-to-have — competitive advantage | 4-6+ months |

### Complexity Framework

| Size | Definition | Typical Effort |
|---|---|---|
| **S** | Small — single component, < 2 days | 1-2 days |
| **M** | Medium — multiple components, < 1 week | 3-5 days |
| **L** | Large — cross-service, 1-2 weeks | 1-2 weeks |
| **XL** | Extra Large — major system, 2-4 weeks | 2-4 weeks |

---

## 2. Current State Assessment

### 2.1 What's Working ✅

| Component | Status | Notes |
|---|---|---|
| Proxy Gateway (Go) | Production | HTTP :7777, SOCKS5 :1080 |
| Node Registration (Go) | Production | WSS, 10K+ peak |
| Customer API (Node.js) | Production | All CRUD endpoints |
| Dashboard (Next.js) | Production | 15+ pages |
| Android SDK | v1.0.57 | Pure Java, auto-reconnect |
| Auth system | Working | JWT, password reset, email verification |
| API keys | Working | CRUD, IP whitelist, toggle |
| Webhooks | Working | CRUD, test, HMAC signing |
| Admin panel | Working | Users, plans, nodes |
| Docker Compose | Working | Full stack orchestration |

### 2.2 What's Scaffolded (Partially Built)

| Component | State | What's Missing |
|---|---|---|
| Stripe billing | Service exists, test keys configured | Checkout flow, webhook handler, subscription lifecycle |
| Email (Resend) | Service exists, templates ready | Integration with triggers, email verification in prod |
| Credit system | SQL schema designed | Backend service, API endpoints, dashboard UI |
| Mobile app | Expo scaffold, package.json | All screens, logic, API integration |
| Earn landing pages | Static HTML exists | Dynamic backend, user accounts, real data |

### 2.3 What's Not Built At All

| Feature | Priority |
|---|---|
| Cashout/payout system | P0 |
| Referral/affiliate system | P1 |
| SDK partner portal (self-service) | P1 |
| Node operator dashboard | P0 |
| 2FA | P1 |
| White-label/reseller | P3 |
| iOS/Windows/macOS SDKs | P2 |
| Real-time WebSocket dashboard | P2 |
| Rate limiting per API key (enforcement) | P0 |
| Webhook delivery retry/logs | P1 |

---

## 3. ProxyClaw Node/Earn Platform

### 3.1 Overview

ProxyClaw is the consumer-facing brand for node operators who share their device's idle bandwidth and earn rewards. This is the **supply side** of the proxy network.

**Competitors:**
- **Honeygain** — $0.10/GB shared, $20 minimum cashout (PayPal/BTC/JumpTask), referral 10%, content delivery bonus, Lucky Pot daily reward
- **EarnApp** — $0.10-0.25/GB (varies by country), $2.50 minimum (PayPal), 10% referral, supports Windows/Mac/Linux/Android/Raspberry Pi
- **PacketStream** — $0.10/GB, $5 minimum cashout (PayPal), 20% referral, Docker support
- **IPRoyal Pawns** — $0.10-0.70/GB (residential premium), $5 minimum (PayPal/BTC/ETH/USDT), 10% referral lifetime, auto-cashout
- **Repocket** — $0.10/GB, $20 minimum, 5% referral
- **Peer2Profit** — $0.10/GB, $2 minimum (crypto), multi-device support

### 3.2 Node Operator Onboarding

#### 3.2.1 Supported Platforms

| Platform | Client Type | Priority | Complexity | Status |
|---|---|---|---|---|
| Android | SDK (Java) embedded in ProxyClaw app | P0 | M | ✅ SDK ready |
| Docker | Container image `iploop/proxyclaw-node` | P0 | S | 🔜 |
| Windows | Desktop app (.exe installer) | P1 | L | 🔜 |
| Linux | CLI binary + systemd service | P1 | M | 🔜 |
| macOS | Desktop app (.dmg) | P2 | L | 🔜 |
| Android TV / Fire TV | Sideload APK or Play Store | P2 | M | 🔜 |
| Smart TV (Samsung/LG) | Tizen/webOS app | P3 | XL | 🔜 |
| Raspberry Pi | ARM binary + systemd | P2 | S | 🔜 |

#### 3.2.2 Onboarding Flow

```
[Visit proxyclaw.io/earn] → [Sign Up (email + password)]
         │
         ▼
[Email Verification] → [Choose Platform]
         │
         ├── Android → [Download from Play Store] → [Enter earn code] → [Start sharing]
         ├── Docker  → [Copy docker run command with API key] → [Start container]
         ├── Windows → [Download installer] → [Login in app] → [Start sharing]
         ├── Linux   → [curl install script] → [proxyclaw login] → [proxyclaw start]
         └── macOS   → [Download .dmg] → [Login in app] → [Start sharing]
         │
         ▼
[Dashboard: See earnings in real-time]
```

**User Stories:**
- US-3.2.1: As a node operator, I want to sign up with my email and start earning within 5 minutes
- US-3.2.2: As a node operator, I want to install the client on Docker with a single command
- US-3.2.3: As a node operator, I want to see my earnings immediately after my device starts sharing
- US-3.2.4: As a node operator, I want to run nodes on multiple devices from one account

**Acceptance Criteria:**
- [ ] Sign-up completes in < 60 seconds
- [ ] Docker node connects within 30 seconds of `docker run`
- [ ] Dashboard shows device as "online" within 10 seconds of connection
- [ ] Each platform has a dedicated download/install page with copy-paste commands
- [ ] User can see which devices are connected and their status

#### 3.2.3 Docker Node Implementation

**Priority:** P0 | **Complexity:** S

```dockerfile
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY proxyclaw-node /usr/local/bin/
ENV PROXYCLAW_API_KEY=""
ENV PROXYCLAW_DEVICE_NAME=""
ENTRYPOINT ["proxyclaw-node"]
```

**Docker run command (shown to user):**
```bash
docker run -d --name proxyclaw \
  --restart unless-stopped \
  -e PROXYCLAW_API_KEY=your_api_key_here \
  -e PROXYCLAW_DEVICE_NAME=my-server \
  iploop/proxyclaw-node:latest
```

**Docker Compose (shown to user):**
```yaml
version: '3.8'
services:
  proxyclaw:
    image: iploop/proxyclaw-node:latest
    restart: unless-stopped
    environment:
      - PROXYCLAW_API_KEY=your_api_key_here
      - PROXYCLAW_DEVICE_NAME=my-server
```

**API Endpoint:** `POST /api/earn/devices/register`

**Data Model:**
```sql
-- Added to devices table
ALTER TABLE devices ADD COLUMN device_name VARCHAR(100);
ALTER TABLE devices ADD COLUMN install_method VARCHAR(20); -- docker, windows, linux, android, macos
ALTER TABLE devices ADD COLUMN node_version VARCHAR(20);
```

**How competitors do it:**
- **Honeygain:** Docker image `honeygain/honeygain`, uses `-device` flag and email/password inline
- **EarnApp:** `earnapp/earnapp` Docker image, requires device UUID registration
- **PacketStream:** `packetstream/psclient` Docker image, uses CID (customer ID)
- **IPRoyal Pawns:** `iproyal/pawns-cli` Docker image, uses email/password/device-name

#### 3.2.4 Windows Desktop App

**Priority:** P1 | **Complexity:** L

**Tech Stack:** Electron or Tauri (Rust-based, smaller binary)

**Features:**
- System tray icon (green = sharing, yellow = paused, red = disconnected)
- Login screen
- Mini dashboard (earnings today, bandwidth shared, uptime)
- Settings: bandwidth limit, auto-start on boot, pause sharing
- Notification when earnings milestone reached

**UI Description:**
- System tray icon with context menu: Open Dashboard, Pause/Resume, Settings, Quit
- Main window: 400x600px, dark theme matching ProxyClaw brand
- Top: logo + user email
- Middle: circular earnings counter (credits earned today), bandwidth shared gauge
- Bottom: device status, connection quality indicator
- Settings tab: max bandwidth slider (1-100 Mbps), auto-start toggle, notification preferences

**Installer:** NSIS or WiX, signed with code signing certificate

#### 3.2.5 Linux CLI

**Priority:** P1 | **Complexity:** M

**Install script:**
```bash
curl -sSL https://get.proxyclaw.io | bash
```

**Commands:**
```bash
proxyclaw login          # Interactive login
proxyclaw start          # Start sharing (foreground)
proxyclaw start -d       # Start as daemon
proxyclaw stop           # Stop daemon
proxyclaw status         # Show current status, earnings
proxyclaw config set     # Set bandwidth limit, device name
proxyclaw logout         # Clear credentials
```

**Systemd service file installed automatically:**
```ini
[Unit]
Description=ProxyClaw Node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/proxyclaw start
Restart=always
RestartSec=30
User=proxyclaw

[Install]
WantedBy=multi-user.target
```

### 3.3 Rewards System

#### 3.3.1 Tier Structure

**Priority:** P0 | **Complexity:** M

| Tier | Requirements | Base Rate | Bonus |
|---|---|---|---|
| **Bronze** | 0-50 GB shared lifetime | $0.10/GB | — |
| **Silver** | 50-500 GB shared lifetime | $0.12/GB | +20% |
| **Gold** | 500+ GB shared lifetime | $0.15/GB | +50% |

**User Stories:**
- US-3.3.1: As a node operator, I want to see my current tier and progress to the next tier
- US-3.3.2: As a node operator, I want to earn more per GB as I share more bandwidth over time
- US-3.3.3: As a node operator, I want to see a progress bar showing how much more I need to reach the next tier

**Data Model:**
```sql
CREATE TYPE earn_tier AS ENUM ('bronze', 'silver', 'gold');

ALTER TABLE users ADD COLUMN earn_tier earn_tier DEFAULT 'bronze';
ALTER TABLE users ADD COLUMN total_gb_shared DECIMAL(15,6) DEFAULT 0;
ALTER TABLE users ADD COLUMN tier_updated_at TIMESTAMPTZ;

CREATE TABLE tier_config (
    tier earn_tier PRIMARY KEY,
    min_gb_shared DECIMAL(15,6) NOT NULL,
    rate_per_gb DECIMAL(10,4) NOT NULL,
    bonus_multiplier DECIMAL(4,2) NOT NULL DEFAULT 1.0,
    badge_color VARCHAR(7), -- hex color
    badge_icon VARCHAR(50)
);

INSERT INTO tier_config VALUES
    ('bronze', 0, 0.10, 1.0, '#CD7F32', 'shield-bronze'),
    ('silver', 50, 0.12, 1.2, '#C0C0C0', 'shield-silver'),
    ('gold', 500, 0.15, 1.5, '#FFD700', 'shield-gold');
```

**Acceptance Criteria:**
- [ ] Tier automatically upgrades when threshold is crossed
- [ ] Tier never downgrades
- [ ] Dashboard shows current tier with badge icon and color
- [ ] Progress bar shows GB until next tier
- [ ] Tier bonus applies retroactively to current billing period (or from upgrade date — configurable)

#### 3.3.2 Multi-Device Bonus

**Priority:** P1 | **Complexity:** S

| Active Devices | Multiplier |
|---|---|
| 1 | 1.0x |
| 2 | 1.0x |
| 3+ | 2.0x |

Already defined in `credits/schema.sql` via `multiplier_rules` table and `get_user_multiplier()` function.

**User Stories:**
- US-3.3.4: As a node operator, I want a bonus for running 3+ devices simultaneously
- US-3.3.5: As a node operator, I want to see my active multiplier and how to increase it

#### 3.3.3 Uptime Bonus

**Priority:** P2 | **Complexity:** M

| Condition | Multiplier |
|---|---|
| 24h continuous uptime (any device) | 1.5x |

**Implementation:**
- Track per-device uptime via heartbeat timestamps
- Calculate continuous uptime: `now() - connected_since` where no gap > 10 minutes
- Apply multiplier to credits earned during the qualifying period
- Reset multiplier if device goes offline for > 10 minutes

#### 3.3.4 Rare Geography Bonus

**Priority:** P2 | **Complexity:** M

| Tier | Countries (examples) | Multiplier |
|---|---|---|
| Common | US, UK, DE, FR, CA, AU | 1.0x |
| Uncommon | BR, MX, PL, RO, ZA, TH | 1.5x |
| Rare | NG, KE, PK, BD, EG, VN | 2.0x |
| Ultra-Rare | Countries with < 10 nodes | 3.0x |

Already defined in `credits/schema.sql` via `geo_multipliers` table.

**Data Model:**
```sql
-- Seed geo multipliers
INSERT INTO geo_multipliers (country, multiplier, tier) VALUES
    ('US', 1.0, 'common'), ('GB', 1.0, 'common'), ('DE', 1.0, 'common'),
    ('FR', 1.0, 'common'), ('CA', 1.0, 'common'), ('AU', 1.0, 'common'),
    ('BR', 1.5, 'uncommon'), ('MX', 1.5, 'uncommon'), ('PL', 1.5, 'uncommon'),
    ('NG', 2.0, 'rare'), ('KE', 2.0, 'rare'), ('PK', 2.0, 'rare'),
    ('BD', 2.0, 'rare'), ('EG', 2.0, 'rare'), ('VN', 2.0, 'rare');
-- Countries not in the table default to 1.0
-- Ultra-rare (< 10 active nodes) computed dynamically
```

#### 3.3.5 Multiplier Stacking

All multipliers stack multiplicatively:
- Max possible: Tier Gold 1.5x × Multi-device 2.0x × Uptime 1.5x × Rare Geo 3.0x = **13.5x**
- Realistic max: Gold 1.5x × Multi-device 2.0x × Uptime 1.5x = **4.5x** (common country)
- Base rate $0.10/GB → max effective rate $1.35/GB (ultra-rare country, Gold tier, 3+ devices, 24h uptime)

**API Endpoint:** `GET /api/earn/multipliers` — returns current multipliers and total

### 3.4 Credit Accumulation and Tracking

#### 3.4.1 Credit System

**Priority:** P0 | **Complexity:** L

The credit system schema already exists in `credits/schema.sql`. Need to build:

1. **Credit Service** (new Go or Node.js microservice)
2. **API endpoints**
3. **Dashboard UI**
4. **Cron jobs for credit calculation**

**Credit Flow:**
```
[Device shares bandwidth] → [bandwidth_contributions table records bytes]
                                    │
                          [Every 5 minutes: cron job]
                                    │
                          [Calculate credits = bytes_to_gb × rate × multipliers]
                                    │
                          [Insert into credits_ledger]
                                    │
                          [Update users.credit_balance]
                                    │
                          [Dashboard shows updated balance in real-time]
```

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| GET | `/api/earn/balance` | Current credit balance, tier, multipliers |
| GET | `/api/earn/history?page=1&limit=20` | Credit ledger history (paginated) |
| GET | `/api/earn/stats?period=7d` | Earnings stats for period |
| GET | `/api/earn/devices` | List devices with individual earnings |
| GET | `/api/earn/multipliers` | Active multipliers breakdown |
| POST | `/api/earn/devices/:id/pause` | Pause sharing on device |
| POST | `/api/earn/devices/:id/resume` | Resume sharing on device |
| DELETE | `/api/earn/devices/:id` | Remove device |

**Acceptance Criteria:**
- [ ] Credits calculated within 5 minutes of bandwidth sharing
- [ ] Ledger is append-only (immutable audit trail)
- [ ] Balance always equals sum of ledger entries
- [ ] User can see per-device earnings breakdown
- [ ] User can see daily/weekly/monthly earnings charts

#### 3.4.2 Credit Display

**Dashboard Widget:**
```
┌─────────────────────────────────────────┐
│  💰 Your Earnings                       │
│                                         │
│  $12.47    ←─── Total balance (USD)     │
│  124,700 credits                        │
│                                         │
│  Today: $0.83 (+830 credits)            │
│  This week: $5.21                       │
│  This month: $12.47                     │
│                                         │
│  🥈 Silver Tier (120 GB / 500 GB)       │
│  ████████░░░░░░░░ 24% to Gold           │
│                                         │
│  Active Multipliers:                    │
│  • Multi-device (3 devices): 2.0x       │
│  • 24h uptime bonus: 1.5x              │
│  • Total: 3.0x                          │
│                                         │
│  [💸 Cash Out]                          │
└─────────────────────────────────────────┘
```

### 3.5 Cashout System

#### 3.5.1 Overview

**Priority:** P0 | **Complexity:** XL

Node operators must be able to convert credits to real money.

**How competitors do it:**
- **Honeygain:** $20 minimum, PayPal/BTC/JumpTask (JMPT token), verification required
- **EarnApp:** $2.50 minimum, PayPal/Amazon Gift Card, no crypto
- **PacketStream:** $5 minimum, PayPal only
- **IPRoyal Pawns:** $5 minimum, PayPal/BTC/ETH/USDT/USDC/Revolut, auto-cashout option
- **Peer2Profit:** $2 minimum, BTC/USDT/LTC/Wire transfer

**ProxyClaw Cashout Methods:**

| Method | Minimum | Fee | Processing Time | Priority |
|---|---|---|---|---|
| PayPal | $5.00 | Free | 1-3 business days | P0 |
| USDT (TRC-20) | $5.00 | $1.00 | 1-24 hours | P0 |
| Bitcoin | $10.00 | Network fee | 1-24 hours | P1 |
| Bank Transfer (SWIFT) | $50.00 | $5.00 | 3-7 business days | P2 |
| Amazon Gift Card | $5.00 | Free | Instant | P2 |
| Revolut | $5.00 | Free | 1-2 business days | P3 |

#### 3.5.2 Cashout Flow

```
[Dashboard: Cash Out button] → [Select method] → [Enter amount]
         │
         ▼
[Enter payment details (PayPal email / crypto address / bank details)]
         │
         ▼
[Confirm cashout] → [Email verification code] → [Submit]
         │
         ▼
[Status: Pending] → [Admin reviews (first cashout only)] → [Approved]
         │
         ▼
[Payment processed] → [Email receipt] → [Status: Completed]
```

**User Stories:**
- US-3.5.1: As a node operator, I want to cash out my earnings to PayPal
- US-3.5.2: As a node operator, I want to cash out to cryptocurrency
- US-3.5.3: As a node operator, I want to see my cashout history and status
- US-3.5.4: As a node operator, I want auto-cashout when my balance reaches a threshold
- US-3.5.5: As an admin, I want to review and approve first-time cashouts to prevent fraud

**Data Model:**
```sql
CREATE TYPE cashout_method AS ENUM ('paypal', 'usdt', 'bitcoin', 'bank_transfer', 'amazon_gift_card', 'revolut');
CREATE TYPE cashout_status AS ENUM ('pending', 'approved', 'processing', 'completed', 'rejected', 'cancelled');

CREATE TABLE cashout_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    method cashout_method NOT NULL,
    amount_credits BIGINT NOT NULL,
    amount_usd DECIMAL(10,2) NOT NULL,
    fee_usd DECIMAL(10,2) NOT NULL DEFAULT 0,
    net_amount_usd DECIMAL(10,2) NOT NULL,
    payment_details JSONB NOT NULL, -- encrypted: {paypal_email, crypto_address, bank_details}
    status cashout_status NOT NULL DEFAULT 'pending',
    admin_notes TEXT,
    reviewed_by UUID REFERENCES users(user_id),
    reviewed_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    transaction_id VARCHAR(255), -- PayPal/blockchain tx ID
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cashout_user ON cashout_requests (user_id, created_at DESC);
CREATE INDEX idx_cashout_status ON cashout_requests (status);

CREATE TABLE cashout_methods_config (
    method cashout_method PRIMARY KEY,
    min_amount_usd DECIMAL(10,2) NOT NULL,
    fee_usd DECIMAL(10,2) NOT NULL DEFAULT 0,
    fee_percent DECIMAL(5,2) NOT NULL DEFAULT 0,
    processing_time_hours INTEGER NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    requires_verification BOOLEAN NOT NULL DEFAULT TRUE,
    auto_approve_after INTEGER DEFAULT NULL -- number of successful cashouts before auto-approve
);

INSERT INTO cashout_methods_config VALUES
    ('paypal', 5.00, 0, 0, 72, TRUE, TRUE, 3),
    ('usdt', 5.00, 1.00, 0, 24, TRUE, TRUE, 3),
    ('bitcoin', 10.00, 0, 0, 24, TRUE, TRUE, 5),
    ('bank_transfer', 50.00, 5.00, 0, 168, TRUE, TRUE, NULL),
    ('amazon_gift_card', 5.00, 0, 0, 1, TRUE, TRUE, 3),
    ('revolut', 5.00, 0, 0, 48, FALSE, TRUE, 3);

CREATE TABLE saved_payment_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    method cashout_method NOT NULL,
    label VARCHAR(100), -- "My PayPal", "BTC Wallet"
    details_encrypted JSONB NOT NULL, -- encrypted payment details
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Auto-cashout settings
CREATE TABLE auto_cashout_config (
    user_id UUID PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    threshold_usd DECIMAL(10,2) NOT NULL DEFAULT 20.00,
    payment_method_id UUID REFERENCES saved_payment_methods(id),
    last_auto_cashout_at TIMESTAMPTZ
);
```

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| GET | `/api/earn/cashout/methods` | Available cashout methods with limits |
| POST | `/api/earn/cashout` | Request cashout |
| GET | `/api/earn/cashout/history` | Cashout history |
| GET | `/api/earn/cashout/:id` | Single cashout detail |
| DELETE | `/api/earn/cashout/:id` | Cancel pending cashout |
| GET | `/api/earn/payment-methods` | Saved payment methods |
| POST | `/api/earn/payment-methods` | Add payment method |
| DELETE | `/api/earn/payment-methods/:id` | Remove payment method |
| PUT | `/api/earn/auto-cashout` | Configure auto-cashout |

**PayPal Integration:**
- Use PayPal Payouts API (batch payouts)
- Requires PayPal Business account
- Send via email address
- Webhook for status updates

**Crypto Integration:**
- Use Coinbase Commerce API or direct blockchain RPC
- Generate unique deposit addresses for verification
- Confirm via blockchain explorer API
- USDT on TRC-20 (low fees)

**Acceptance Criteria:**
- [ ] User can request cashout with minimum $5
- [ ] First 3 cashouts require admin approval
- [ ] After 3 successful cashouts, auto-approve enabled
- [ ] User receives email notification at each status change
- [ ] User can save payment methods for quick cashout
- [ ] Auto-cashout triggers daily if threshold met
- [ ] Admin dashboard shows pending cashouts for review
- [ ] Payment details are encrypted at rest (AES-256)

### 3.6 Node Operator Dashboard

#### 3.6.1 Overview

**Priority:** P0 | **Complexity:** L

A dedicated dashboard for node operators (separate from the proxy customer dashboard, or a tab within the existing dashboard for users who are both).

**URL:** `https://proxyclaw.io/dashboard` or `https://iploop.io/earn/dashboard`

#### 3.6.2 Dashboard Pages

**Main Earn Dashboard (`/earn`)**

```
┌─────────────────────────────────────────────────────────────────┐
│  [ProxyClaw Logo]        Earn Dashboard         [User] [⚙️]    │
├─────────┬───────────────────────────────────────────────────────┤
│         │                                                       │
│ 📊 Home │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐│
│ 📱 Devic│  │ Balance  │ │ Today    │ │ Active   │ │ Tier     ││
│ 💰 Earni│  │ $12.47   │ │ $0.83   │ │ 3 devices│ │ 🥈 Silver││
│ 💸 Cash │  │ 124.7K cr│ │ +830 cr  │ │ 2.0x mult│ │ 24% Gold ││
│ 👥 Refer│  └──────────┘ └──────────┘ └──────────┘ └──────────┘│
│ ⚙️ Setti│                                                      │
│         │  [Earnings Chart — 30 day area chart]                 │
│         │                                                       │
│         │  ┌─ Active Devices ─────────────────────────────────┐ │
│         │  │ 🖥️ my-server (Docker) — 2.1 GB today — Online   │ │
│         │  │ 📱 Samsung A17 (Android) — 0.5 GB today — Online │ │
│         │  │ 💻 Desktop-PC (Windows) — 1.3 GB today — Online  │ │
│         │  │                                                   │ │
│         │  │ [+ Add New Device]                                │ │
│         │  └───────────────────────────────────────────────────┘ │
└─────────┴───────────────────────────────────────────────────────┘
```

**Devices Page (`/earn/devices`)**

| Column | Description |
|---|---|
| Device Name | User-defined or auto-generated |
| Platform | android/docker/windows/linux/macos icon |
| IP Address | Current (masked: 192.168.xxx.xxx) |
| Country | Flag + country name |
| Status | 🟢 Online / 🟡 Paused / 🔴 Offline |
| Bandwidth Today | GB shared today |
| Bandwidth Total | Lifetime GB |
| Uptime | Current session duration |
| Earnings Today | Credits earned today |
| Actions | Pause, Resume, Remove, Rename |

**Earnings Page (`/earn/earnings`)**

- Earnings chart (daily, weekly, monthly toggle)
- Per-device earnings breakdown (stacked bar chart)
- Multiplier history (when bonuses applied)
- Earnings table (date, device, GB shared, base credits, multiplier, total credits)
- Export as CSV

**Cashout Page (`/earn/cashout`)**

- Current balance prominently displayed
- "Cash Out" button → method selection → amount → details → confirm
- Cashout history table (date, amount, method, status, transaction ID)
- Saved payment methods management
- Auto-cashout toggle

**Referral Page (`/earn/referral`)**

- Unique referral link + copy button
- Referral code display
- Share buttons (Twitter, Facebook, WhatsApp, Telegram, Email)
- Referral stats (clicks, sign-ups, earnings from referrals)
- Referred users table (username masked, join date, status, your earnings from them)

**Settings Page (`/earn/settings`)**

- Profile (name, email, avatar)
- Notification preferences (email on cashout, daily earnings digest, referral sign-up)
- Bandwidth limits per device
- Auto-cashout configuration
- Delete account

#### 3.6.3 Data Model Additions

```sql
-- Earnings summary (materialized view, refreshed every 5 min)
CREATE MATERIALIZED VIEW earn_daily_summary AS
SELECT
    user_id,
    date_trunc('day', ts) AS day,
    SUM(bytes_shared) AS total_bytes,
    SUM(bytes_shared) / 1073741824.0 AS total_gb,
    COUNT(DISTINCT device_id) AS active_devices
FROM bandwidth_contributions
GROUP BY user_id, date_trunc('day', ts);

CREATE UNIQUE INDEX idx_earn_daily ON earn_daily_summary (user_id, day);

-- Device earnings (computed per credit calculation cycle)
CREATE TABLE device_earnings (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(device_id),
    user_id UUID NOT NULL REFERENCES users(user_id),
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    bytes_shared BIGINT NOT NULL,
    base_credits BIGINT NOT NULL,
    multiplier DECIMAL(6,2) NOT NULL DEFAULT 1.0,
    total_credits BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_device_earnings ON device_earnings (user_id, period_start DESC);
```

### 3.7 Node Operator Referral Program

**Priority:** P1 | **Complexity:** M

**Structure (industry standard):**
- Referrer earns **10% of referee's earnings** for lifetime
- Referee gets **$0.50 sign-up bonus** (500 credits)
- Both parties notified on sign-up and first cashout

**How competitors do it:**
- **Honeygain:** 10% of referred user's earnings, lifetime
- **EarnApp:** 10% of referral's earnings
- **IPRoyal Pawns:** 10% lifetime earnings
- **PacketStream:** 20% of referral's earnings (unusually high)
- **Repocket:** 5% of referral's earnings

**Data Model:**
```sql
CREATE TABLE referrals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    referrer_id UUID NOT NULL REFERENCES users(user_id),
    referee_id UUID NOT NULL REFERENCES users(user_id),
    referral_code VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, active, churned
    referee_total_earnings BIGINT NOT NULL DEFAULT 0,
    referrer_total_commission BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users ADD COLUMN referral_code VARCHAR(20) UNIQUE;
ALTER TABLE users ADD COLUMN referred_by UUID REFERENCES users(user_id);

CREATE INDEX idx_referrals_referrer ON referrals (referrer_id);
```

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| GET | `/api/earn/referral/code` | Get user's referral code + link |
| GET | `/api/earn/referral/stats` | Referral stats (count, earnings) |
| GET | `/api/earn/referral/list` | List referred users |
| POST | `/api/auth/register?ref=CODE` | Register with referral code |

---

## 4. Referral & Affiliate System

### 4.1 Overview

This section covers the **customer-side** referral and affiliate program — for people who refer proxy buyers (not node operators, which is covered in Section 3.7).

**Priority:** P1 | **Complexity:** L

**How competitors do it:**
- **Bright Data:** 50% revenue share for affiliates (up to $2,500 per customer), 15% second-tier on referred affiliates' earnings. Uses PartnerStack platform.
- **Oxylabs:** 30-50% commission, dedicated affiliate managers, custom deals for top performers
- **SOAX:** Up to 30% recurring commission, CJ Affiliate network
- **Smartproxy:** Up to 50% commission on first purchase, 15% recurring
- **NetNut:** Custom affiliate deals, typically 20-30%

### 4.2 Commission Structure

**ProxyClaw/IPLoop Affiliate Program:**

| Tier | Monthly Revenue Generated | Commission Rate | Cap per Customer |
|---|---|---|---|
| Standard | $0 - $5,000 | 20% recurring | $500/customer |
| Silver | $5,000 - $20,000 | 25% recurring | $750/customer |
| Gold | $20,000+ | 30% recurring | $1,000/customer |

- Recurring commission for 12 months from customer's first payment
- $100 bonus for each customer who spends $100+ in first month
- Cookie duration: 90 days
- Payment: Monthly via PayPal or bank transfer
- Minimum payout: $50

### 4.3 Referral Code System

**Data Model:**
```sql
CREATE TABLE affiliate_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id), -- NULL if external affiliate
    name VARCHAR(200) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    company VARCHAR(200),
    website VARCHAR(500),
    affiliate_code VARCHAR(20) NOT NULL UNIQUE,
    tier VARCHAR(20) NOT NULL DEFAULT 'standard', -- standard, silver, gold
    commission_rate DECIMAL(5,2) NOT NULL DEFAULT 20.00,
    total_referrals INTEGER NOT NULL DEFAULT 0,
    total_revenue_generated DECIMAL(15,2) NOT NULL DEFAULT 0,
    total_commissions_earned DECIMAL(15,2) NOT NULL DEFAULT 0,
    total_commissions_paid DECIMAL(15,2) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, suspended
    payment_method VARCHAR(20), -- paypal, bank_transfer
    payment_details JSONB,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE affiliate_referrals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_accounts(id),
    referred_user_id UUID NOT NULL REFERENCES users(id),
    click_id VARCHAR(100), -- tracking click ID
    landing_page VARCHAR(500),
    referrer_url VARCHAR(500),
    ip_address INET,
    user_agent TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'signed_up', -- clicked, signed_up, converted, active
    first_payment_at TIMESTAMPTZ,
    total_revenue DECIMAL(15,2) NOT NULL DEFAULT 0,
    total_commission DECIMAL(15,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE affiliate_commissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_accounts(id),
    referral_id UUID NOT NULL REFERENCES affiliate_referrals(id),
    invoice_id VARCHAR(255), -- Stripe invoice ID
    revenue_amount DECIMAL(15,2) NOT NULL,
    commission_rate DECIMAL(5,2) NOT NULL,
    commission_amount DECIMAL(15,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, paid
    month VARCHAR(7), -- YYYY-MM
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE affiliate_payouts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    affiliate_id UUID NOT NULL REFERENCES affiliate_accounts(id),
    amount DECIMAL(15,2) NOT NULL,
    method VARCHAR(20) NOT NULL,
    transaction_id VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);

CREATE TABLE affiliate_clicks (
    id BIGSERIAL PRIMARY KEY,
    affiliate_id UUID NOT NULL REFERENCES affiliate_accounts(id),
    click_id VARCHAR(100) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    referrer_url VARCHAR(500),
    landing_page VARCHAR(500),
    converted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_aff_clicks_affiliate ON affiliate_clicks (affiliate_id, created_at DESC);
```

### 4.4 Affiliate Dashboard

**URL:** `https://iploop.io/affiliates` or `https://affiliates.iploop.io`

**Pages:**

**Overview Dashboard:**
```
┌─────────────────────────────────────────────────────────────┐
│  IPLoop Affiliate Dashboard                    [User] [⚙️]  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │
│  │ Clicks  │ │ Sign-ups│ │Revenue  │ │ Earned  │          │
│  │ 1,234   │ │ 89      │ │ $4,500  │ │ $900    │          │
│  │ This Mo │ │ This Mo │ │ This Mo │ │ This Mo │          │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘          │
│                                                             │
│  [Conversion Funnel Chart]                                  │
│  Clicks → Sign-ups → First Payment → Active Customer       │
│  1,234  →    89     →      34       →      28              │
│                                                             │
│  [Monthly Earnings Chart — 12 months]                       │
│                                                             │
│  Your Referral Link:                                        │
│  https://iploop.io/?ref=ABCD1234  [Copy] [QR Code]         │
│                                                             │
│  Coupon Code: PARTNER20 (20% off first month)              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Referrals Page:** Table of referred customers (anonymized email, sign-up date, status, revenue generated, commission earned)

**Payouts Page:** Payout history, pending amount, next payout date, payment method settings

**Marketing Materials Page:**
- Pre-made banners (300x250, 728x90, 160x600)
- Text link generator with UTM parameters
- Email templates
- Social media post templates
- Landing page builder (custom branded landing pages)

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| POST | `/api/affiliates/apply` | Apply for affiliate program |
| GET | `/api/affiliates/dashboard` | Dashboard stats |
| GET | `/api/affiliates/referrals` | List referrals |
| GET | `/api/affiliates/commissions` | Commission history |
| GET | `/api/affiliates/payouts` | Payout history |
| POST | `/api/affiliates/payouts/request` | Request payout |
| GET | `/api/affiliates/links` | Generate tracking links |
| GET | `/api/affiliates/materials` | Marketing materials |
| PUT | `/api/affiliates/settings` | Update payment settings |

**Tracking Implementation:**
- Referral link: `https://iploop.io/?ref=CODE`
- Set cookie `iploop_ref=CODE` with 90-day expiry
- On registration, check cookie and associate with affiliate
- On each invoice payment, calculate commission and record

**Acceptance Criteria:**
- [ ] Affiliate can apply and get approved within 24h
- [ ] Tracking cookie persists for 90 days
- [ ] Commission calculated automatically on invoice payment
- [ ] Dashboard shows real-time stats
- [ ] Monthly payout processed automatically for amounts > $50
- [ ] Affiliate tier upgrades automatically based on revenue

---

## 5. Billing & Payments (Stripe)

### 5.1 Overview

**Priority:** P0 | **Complexity:** XL

Stripe integration is partially scaffolded. Need complete implementation of subscription lifecycle, usage-based billing, invoicing, and payment management.

**Current state:**
- Stripe test keys configured
- `services/billing/` Go service exists
- `services/customer-api/src/services/stripe.js` wrapper exists
- Plans defined in database

**How competitors do it:**
- **Bright Data:** Pay-as-you-go ($5.04/GB residential), subscription plans, prepaid credits, volume discounts
- **SOAX:** $3.60/GB starter (25GB), $2.00/GB business (800GB), all proxy types unified pricing
- **Oxylabs:** $8/GB residential, $15/GB mobile, subscription with overage
- **Smartproxy:** $4.00/GB (100GB plan), $2.20/GB (1TB plan)

### 5.2 Pricing Plans (Revised)

Based on competitor analysis, revise pricing to be competitive:

| Plan | Monthly | GB Included | Price/GB | Overage/GB | Requests/day | Connections |
|---|---|---|---|---|---|---|
| **Trial** | Free | 0.5 GB | — | — | 1,000 | 5 |
| **Starter** | $49 | 5 GB | $9.80 | $7.00 | 10,000 | 10 |
| **Growth** | $149 | 25 GB | $5.96 | $5.00 | 50,000 | 50 |
| **Business** | $499 | 100 GB | $4.99 | $4.00 | 200,000 | 200 |
| **Enterprise** | Custom | Custom | Custom | Custom | Unlimited | Unlimited |
| **Pay-as-you-go** | $0 | 0 | $5.00/GB | $5.00/GB | 50,000 | 25 |

Annual plans: 20% discount.

### 5.3 Stripe Integration

#### 5.3.1 Subscription Lifecycle

```
[User selects plan] → [Stripe Checkout Session] → [Payment]
         │
         ▼ (webhook: checkout.session.completed)
[Create Stripe Subscription] → [Update user plan in DB]
         │
         ▼ (webhook: invoice.paid — monthly)
[Reset bandwidth allowance] → [Record payment]
         │
         ▼ (webhook: customer.subscription.updated)
[Handle plan changes (upgrade/downgrade)]
         │
         ▼ (webhook: customer.subscription.deleted)
[Downgrade to free/trial plan]
         │
         ▼ (webhook: invoice.payment_failed)
[Send failed payment email] → [Retry 3x over 7 days] → [Suspend if still failing]
```

#### 5.3.2 Stripe Objects Mapping

| IPLoop Concept | Stripe Object |
|---|---|
| Customer account | `Customer` |
| Subscription plan | `Product` + `Price` |
| Monthly subscription | `Subscription` |
| Bandwidth overage | `Metered Usage Record` |
| One-time credit purchase | `Payment Intent` |
| Invoice | `Invoice` |
| Payment method | `PaymentMethod` (attached to Customer) |

#### 5.3.3 Webhook Handling

**Webhook endpoint:** `POST /api/billing/webhooks/stripe`

| Event | Action |
|---|---|
| `checkout.session.completed` | Create subscription record, activate plan |
| `customer.subscription.created` | Log subscription, send welcome email |
| `customer.subscription.updated` | Update plan, adjust limits |
| `customer.subscription.deleted` | Revert to free plan |
| `invoice.paid` | Record payment, reset bandwidth, send receipt |
| `invoice.payment_failed` | Send notification, attempt retry, flag account |
| `invoice.finalized` | Store invoice PDF URL |
| `payment_intent.succeeded` | Credit account for one-time purchases |
| `customer.updated` | Sync customer data |

**Webhook verification:** Verify `stripe-signature` header using webhook secret

**Data Model additions:**
```sql
CREATE TABLE stripe_customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id),
    stripe_customer_id VARCHAR(255) NOT NULL UNIQUE,
    default_payment_method VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    stripe_subscription_id VARCHAR(255) NOT NULL UNIQUE,
    stripe_price_id VARCHAR(255) NOT NULL,
    plan_id UUID NOT NULL REFERENCES plans(id),
    status VARCHAR(20) NOT NULL, -- active, past_due, cancelled, trialing
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN DEFAULT FALSE,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    stripe_invoice_id VARCHAR(255) NOT NULL UNIQUE,
    stripe_subscription_id VARCHAR(255),
    amount_due INTEGER NOT NULL, -- cents
    amount_paid INTEGER NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'usd',
    status VARCHAR(20) NOT NULL, -- draft, open, paid, void, uncollectible
    invoice_pdf VARCHAR(500),
    hosted_invoice_url VARCHAR(500),
    period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE payment_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    stripe_payment_method_id VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL, -- card, bank_account
    card_brand VARCHAR(20), -- visa, mastercard, amex
    card_last4 VARCHAR(4),
    card_exp_month INTEGER,
    card_exp_year INTEGER,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE credit_purchases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    amount_usd DECIMAL(10,2) NOT NULL,
    gb_amount DECIMAL(10,2) NOT NULL,
    stripe_payment_intent_id VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usage metering for overage billing
CREATE TABLE bandwidth_metering (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    included_gb DECIMAL(10,2) NOT NULL,
    used_gb DECIMAL(10,6) NOT NULL,
    overage_gb DECIMAL(10,6) NOT NULL DEFAULT 0,
    overage_cost DECIMAL(10,2) NOT NULL DEFAULT 0,
    reported_to_stripe BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 5.4 Billing Dashboard UI

**Priority:** P0 | **Complexity:** L

**Billing Page (`/billing`):**

```
┌─────────────────────────────────────────────────────────────────┐
│  Billing & Subscription                                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Current Plan: Growth ($149/mo)                                 │
│  Bandwidth: 18.7 GB / 25 GB used                               │
│  ████████████████████░░░░░ 74.8%                                │
│  Renews: March 15, 2026                                         │
│  [Change Plan] [Cancel Subscription]                            │
│                                                                 │
│  ┌─ Payment Method ──────────────────────────────────────────┐  │
│  │  💳 Visa ending in 4242  (default)   [Edit] [Remove]     │  │
│  │  [+ Add Payment Method]                                   │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌─ Prepaid Credits ─────────────────────────────────────────┐  │
│  │  Balance: $25.00 (5 GB)                                   │  │
│  │  [Buy Credits: $25 (5GB) | $50 (12GB) | $100 (25GB)]     │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌─ Invoices ────────────────────────────────────────────────┐  │
│  │  # | Date       | Amount  | Status | PDF                 │  │
│  │  1 | 2026-02-15 | $149.00 | Paid   | [Download]          │  │
│  │  2 | 2026-01-15 | $149.00 | Paid   | [Download]          │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌─ Usage Alerts ────────────────────────────────────────────┐  │
│  │  ☑ Alert at 80% usage    ☑ Alert at 100% usage           │  │
│  │  ☑ Auto-purchase 5GB at 100% ($25)  [Configure]          │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| GET | `/api/billing/subscription` | Current subscription details |
| POST | `/api/billing/checkout` | Create Stripe Checkout session |
| POST | `/api/billing/portal` | Create Stripe Customer Portal session |
| GET | `/api/billing/invoices` | List invoices |
| GET | `/api/billing/invoices/:id/pdf` | Download invoice PDF |
| GET | `/api/billing/payment-methods` | List payment methods |
| POST | `/api/billing/payment-methods` | Add payment method (via Stripe SetupIntent) |
| DELETE | `/api/billing/payment-methods/:id` | Remove payment method |
| PUT | `/api/billing/payment-methods/:id/default` | Set default |
| POST | `/api/billing/credits/purchase` | Buy prepaid credits |
| GET | `/api/billing/credits/balance` | Credit balance |
| PUT | `/api/billing/alerts` | Configure usage alerts |
| POST | `/api/billing/webhooks/stripe` | Stripe webhook handler |

**Acceptance Criteria:**
- [ ] User can subscribe to any plan via Stripe Checkout
- [ ] User can upgrade/downgrade (prorated)
- [ ] User can cancel (effective end of period)
- [ ] Invoices generated and stored on each payment
- [ ] PDF invoices downloadable
- [ ] Usage alerts trigger email at 80% and 100%
- [ ] Overage billing works (metered usage reported to Stripe)
- [ ] Prepaid credits can be purchased
- [ ] Failed payments retry 3x then suspend account

---

## 6. SDK Partner Portal

### 6.1 Overview

**Priority:** P1 | **Complexity:** XL

SDK partners are app developers who embed the IPLoop SDK to monetize idle bandwidth. Currently managed via admin panel only. Need a self-service portal.

**How competitors do it:**
- **Bright Data (SDK/EarnApp):** Partners apply, get SDK + docs, dashboard shows nodes/earnings/revenue
- **Honeygain:** Business SDK program, dedicated partner managers, revenue share based on volume
- **PacketStream:** Open API, anyone can become a packeter (supply) or consumer (demand)
- **Infatica:** Partner program with SDK for Android/Windows, revenue sharing dashboard

### 6.2 Partner Onboarding Flow

```
[Partner visits partners.iploop.io] → [Apply]
         │
         ▼
[Application Form: company name, app name, estimated DAU, platforms, use case]
         │
         ▼
[Admin reviews application] → [Approve/Reject]
         │ (if approved)
         ▼
[Partner receives welcome email with portal access]
         │
         ▼
[Login to Partner Portal] → [Generate SDK API Key] → [Download SDK]
         │
         ▼
[Integrate SDK following docs] → [Test integration] → [Submit for review]
         │
         ▼
[IPLoop team reviews integration] → [Approve] → [Go live]
```

### 6.3 Partner Portal Pages

**Partner Dashboard:**
```
┌─────────────────────────────────────────────────────────────────┐
│  IPLoop Partner Portal              [AppName Inc.]    [⚙️]      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │ Active   │ │ Bandwidth│ │ Revenue  │ │ This     │          │
│  │ Nodes    │ │ Today    │ │ This Mo  │ │ Month    │          │
│  │ 3,450    │ │ 124 GB   │ │ $2,340   │ │ $8,670   │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
│                                                                 │
│  [Node Count Chart — 30 days]                                   │
│  [Revenue Chart — 12 months]                                    │
│                                                                 │
│  ┌─ Apps ─────────────────────────────────────────────────────┐ │
│  │ MyWeatherApp (Android)  — 2,100 nodes — $1,450/mo         │ │
│  │ CleanerPro (Windows)    — 1,350 nodes — $890/mo           │ │
│  │ [+ Register New App]                                       │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**SDK Integration Page:**
- SDK download links (Android JAR, Windows DLL, etc.)
- Quick start guide
- API reference
- Code examples (Java, Kotlin, C#, Python)
- Test mode toggle
- Integration checklist

**Revenue Dashboard:**
- Revenue by app, by country, by day
- Revenue share percentage (configurable per partner)
- Payout history
- Projected revenue

**API Keys Page:**
- Generate/revoke partner API keys
- Key permissions (which apps, which platforms)
- Usage stats per key

**Compliance Page:**
- Privacy policy requirements
- User consent flow requirements
- SDK disclosure requirements
- Legal agreements (click-to-accept)
- Compliance checklist

### 6.4 Data Model

```sql
CREATE TABLE partner_apps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    partner_id UUID NOT NULL REFERENCES partners(id),
    app_name VARCHAR(200) NOT NULL,
    package_name VARCHAR(200), -- com.example.app
    platform VARCHAR(20) NOT NULL, -- android, ios, windows, mac
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, testing, approved, suspended
    sdk_api_key VARCHAR(255) NOT NULL UNIQUE,
    sdk_api_key_hash VARCHAR(255) NOT NULL,
    daily_active_users INTEGER DEFAULT 0,
    total_nodes_contributed BIGINT DEFAULT 0,
    total_bandwidth_gb DECIMAL(15,2) DEFAULT 0,
    total_revenue_share DECIMAL(15,2) DEFAULT 0,
    privacy_policy_url VARCHAR(500),
    terms_url VARCHAR(500),
    consent_mechanism TEXT, -- how user consent is obtained
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE partner_revenue (
    id BIGSERIAL PRIMARY KEY,
    partner_id UUID NOT NULL REFERENCES partners(id),
    app_id UUID NOT NULL REFERENCES partner_apps(id),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    total_nodes INTEGER NOT NULL DEFAULT 0,
    total_bandwidth_gb DECIMAL(15,6) NOT NULL DEFAULT 0,
    gross_revenue DECIMAL(15,2) NOT NULL DEFAULT 0,
    revenue_share_percent DECIMAL(5,2) NOT NULL,
    partner_payout DECIMAL(15,2) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'calculated', -- calculated, invoiced, paid
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE partner_payouts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    partner_id UUID NOT NULL REFERENCES partners(id),
    amount DECIMAL(15,2) NOT NULL,
    method VARCHAR(20) NOT NULL, -- bank_transfer, paypal
    invoice_number VARCHAR(50),
    period VARCHAR(7) NOT NULL, -- YYYY-MM
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    transaction_id VARCHAR(255),
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_partner_revenue ON partner_revenue (partner_id, period_start DESC);
```

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| POST | `/api/partners/apply` | Apply to become partner |
| GET | `/api/partners/dashboard` | Partner dashboard stats |
| GET | `/api/partners/apps` | List registered apps |
| POST | `/api/partners/apps` | Register new app |
| PUT | `/api/partners/apps/:id` | Update app details |
| GET | `/api/partners/apps/:id/nodes` | Nodes for specific app |
| GET | `/api/partners/revenue` | Revenue breakdown |
| GET | `/api/partners/revenue/export` | Export revenue CSV |
| GET | `/api/partners/payouts` | Payout history |
| GET | `/api/partners/sdk/download` | SDK download links |
| GET | `/api/partners/sdk/docs` | Integration documentation |
| POST | `/api/partners/keys` | Generate new SDK key |
| DELETE | `/api/partners/keys/:id` | Revoke SDK key |

**Acceptance Criteria:**
- [ ] Partner can self-register and apply
- [ ] Admin can approve/reject applications
- [ ] Partner can register multiple apps
- [ ] Each app gets unique SDK key
- [ ] Revenue dashboard shows per-app breakdown
- [ ] Monthly payouts calculated automatically
- [ ] Partner can download SDK and integration docs
- [ ] Compliance checklist enforced before app goes live

---

## 7. Advanced Proxy Features

### 7.1 Sticky Sessions (Enhancement)

**Priority:** P1 | **Complexity:** M | **Status:** Basic implementation exists

**Current:** Session manager exists in `services/proxy-gateway/internal/session/manager.go`

**Enhancements needed:**
- Session persistence across gateway restarts (store in Redis)
- Session health monitoring (auto-failover if node goes offline)
- Maximum session duration configurable per plan
- Session pool pre-warming for frequently requested geos

**Data Model:**
```sql
CREATE TABLE sticky_sessions (
    session_id VARCHAR(255) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    node_id UUID NOT NULL,
    node_ip INET NOT NULL,
    target_country VARCHAR(2),
    target_city VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    request_count INTEGER NOT NULL DEFAULT 0,
    bytes_transferred BIGINT NOT NULL DEFAULT 0
);
```

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| GET | `/api/proxy/sessions` | List active sessions |
| DELETE | `/api/proxy/sessions/:id` | Force-close session |
| POST | `/api/proxy/sessions/refresh` | Extend session lifetime |

### 7.2 Geographic Targeting (Enhancement)

**Priority:** P0 | **Complexity:** M | **Status:** Basic country/city/ASN exists

**Enhancements:**
- ISP-level targeting (e.g., "Comcast", "AT&T")
- Zip code targeting (US only initially)
- Coordinates-based targeting (lat/lng + radius)
- "Best available" mode — prefer nodes with highest quality in target geo
- Geo inventory API — show available IPs per location in real-time

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| GET | `/api/proxy/locations` | Available locations with node counts |
| GET | `/api/proxy/locations/:country` | Cities in country with counts |
| GET | `/api/proxy/locations/:country/:city/isps` | ISPs in city |
| GET | `/api/proxy/inventory` | Real-time IP inventory by geo |

**Acceptance Criteria:**
- [ ] User can target by country, city, ASN, ISP
- [ ] Inventory API returns real-time node counts per location
- [ ] "Best available" selects highest quality node matching criteria
- [ ] Returns 503 with available alternatives if no exact match

### 7.3 Browser Fingerprinting Profiles

**Priority:** P2 | **Complexity:** M | **Status:** Basic profiles exist in SDK

**Current profiles:** chrome-win, firefox-mac, mobile-ios, mobile-android

**Enhanced profiles system:**
```sql
CREATE TABLE browser_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    user_agent TEXT NOT NULL,
    accept_language VARCHAR(100),
    platform VARCHAR(50),
    screen_resolution VARCHAR(20),
    color_depth INTEGER,
    timezone VARCHAR(50),
    webgl_vendor VARCHAR(200),
    webgl_renderer VARCHAR(200),
    fonts JSONB, -- array of font names
    plugins JSONB, -- array of plugin objects
    canvas_noise BOOLEAN DEFAULT FALSE,
    webrtc_policy VARCHAR(20) DEFAULT 'default', -- default, disable, relay_only
    is_system BOOLEAN DEFAULT TRUE, -- system vs user-created
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Features:**
- Pre-built profiles for common browser/OS combinations (50+ profiles)
- Custom profile builder (user creates their own)
- Random profile selection per request
- Profile rotation strategies
- WebRTC leak prevention
- Canvas fingerprint randomization

### 7.4 Bandwidth Quality Scoring

**Priority:** P1 | **Complexity:** M

**Quality Score (0-100) factors:**

| Factor | Weight | Measurement |
|---|---|---|
| Speed (Mbps) | 30% | Download speed test |
| Latency (ms) | 25% | Round-trip time |
| Uptime | 20% | Online time / total time |
| Success Rate | 15% | Successful requests / total |
| Stability | 10% | Connection drops per hour |

**Implementation:**
- Run periodic speed tests on nodes (every 30 min)
- Calculate rolling average quality score
- Customers on Business+ plans get first pick of high-quality nodes
- Quality-based routing: route to highest quality node matching criteria

**API:** `GET /api/proxy/quality?country=US` — returns average quality by location

### 7.5 IP Rotation Strategies

**Priority:** P1 | **Complexity:** M | **Status:** Basic rotation exists

**Strategies:**

| Strategy | Description | Use Case |
|---|---|---|
| Per-request | New IP every request | Scraping |
| Timed | New IP every N minutes | Long sessions |
| Manual | Same IP until explicitly rotated | Testing |
| Smart | Rotate on 403/captcha | Anti-detection |
| Geographic | Rotate within same geo | Geo-specific scraping |

**Smart rotation implementation:**
- Monitor response codes from target sites
- If 403, 429, or captcha detected, auto-rotate to new node
- Configurable retry count before giving up
- Backoff between retries

**Parameter:** `-rotate-smart` or `-rotate-on-block`

### 7.6 Protocol Support

**Priority:** P0 (SOCKS5 auth), P2 (others) | **Complexity:** varies

| Protocol | Port | Status | Notes |
|---|---|---|---|
| HTTP | 7777 | ✅ Working | CONNECT method for HTTPS |
| SOCKS5 | 1080 | ✅ Working | Username/password auth |
| HTTPS (direct) | 7778 | 🔜 P2 | TLS termination at proxy |
| SOCKS5 over TLS | 1081 | 🔜 P3 | Encrypted SOCKS5 |
| WireGuard VPN | 51820 | 🔜 P3 | Full VPN mode |

### 7.7 Programmatic Proxy Management API

**Priority:** P1 | **Complexity:** L

Full API for managing proxy settings programmatically (not just via URL parameters).

**Endpoints:**

| Method | Path | Description |
|---|---|---|
| POST | `/api/proxy/request` | Make proxy request via API (body contains target URL, config) |
| GET | `/api/proxy/pool` | Get list of available IPs |
| POST | `/api/proxy/pool/reserve` | Reserve specific IPs for exclusive use |
| DELETE | `/api/proxy/pool/reserve/:id` | Release reserved IPs |
| GET | `/api/proxy/pool/count` | Count available IPs by criteria |

---

## 8. Analytics & Reporting

### 8.1 Real-Time Dashboard

**Priority:** P1 | **Complexity:** L

**Implementation:** WebSocket-based live updates pushed from proxy gateway.

**Real-time metrics:**
- Active nodes count (updated every 5s)
- Requests per second (live counter)
- Bandwidth per second (live gauge)
- Active sessions count
- Error rate (last 60s)
- Geographic heatmap (nodes appearing/disappearing live)

**Tech:** Server-Sent Events (SSE) or WebSocket from customer-api to dashboard

**API:** `GET /api/analytics/stream` (SSE) or `WS /api/analytics/ws`

### 8.2 Geographic Heatmap

**Priority:** P2 | **Complexity:** M

**Description:** Interactive world map showing:
- Node distribution (dots with size = node count)
- Color intensity by density
- Click on country → drill down to city level
- Tooltip: country, node count, avg quality, available IPs
- Filter by connection type (wifi/cellular)

**Tech:** Mapbox GL JS or Leaflet with custom tile layer

### 8.3 Customer Usage Analytics

**Priority:** P1 | **Complexity:** M

**Enhanced analytics per customer:**

| Metric | Chart Type | Period |
|---|---|---|
| Requests over time | Area chart | 7d/30d/90d/1y |
| Bandwidth over time | Bar chart | 7d/30d/90d/1y |
| Success vs error rate | Stacked area | 7d/30d/90d/1y |
| Response time distribution | Histogram | 30d |
| Top target domains | Horizontal bar | 30d |
| Geographic usage | Heatmap | 30d |
| Per-API-key breakdown | Table | 30d |
| Hourly usage pattern | Heatmap (24h × 7d) | 7d |
| Status code distribution | Pie chart | 30d |

### 8.4 Revenue Analytics (Admin)

**Priority:** P1 | **Complexity:** M

**Metrics:**
- Monthly Recurring Revenue (MRR)
- Annual Run Rate (ARR)
- Average Revenue Per User (ARPU)
- Churn rate
- Net Revenue Retention
- Revenue by plan tier
- Revenue by customer
- Growth rate (MoM, YoY)
- Customer Lifetime Value (LTV)
- Customer Acquisition Cost (CAC) — manual input

**Dashboard:**
```
┌─────────────────────────────────────────────────────────────────┐
│  Revenue Dashboard (Admin)                                      │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │ MRR      │ │ ARR      │ │ ARPU     │ │ Churn    │          │
│  │ $12,450  │ │ $149,400 │ │ $124.50  │ │ 3.2%     │          │
│  │ +12% MoM │ │          │ │ +$8 MoM  │ │ -0.5%    │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
│                                                                 │
│  [MRR Chart — 12 months, stacked by plan]                       │
│  [Customer Count — 12 months, stacked by plan]                  │
│  [Revenue by Geography — Pie chart]                             │
│                                                                 │
│  ┌─ Top Customers ───────────────────────────────────────────┐  │
│  │  1. Company A — $2,490/mo (Business)                      │  │
│  │  2. Company B — $1,490/mo (Business)                      │  │
│  │  3. Company C — $499/mo (Business)                        │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 8.5 Node Health Monitoring

**Priority:** P1 | **Complexity:** M

**Monitoring per node:**
- Connection uptime (% online over period)
- Heartbeat regularity
- Request success rate
- Average response time
- Bandwidth throughput
- Error types (timeout, connection refused, DNS failure)

**Alerting:**
- Alert when node pool drops below threshold (e.g., < 1000 nodes)
- Alert when quality score drops in a country
- Alert when error rate spikes

### 8.6 Export Capabilities

**Priority:** P2 | **Complexity:** S

| Export | Format | Data |
|---|---|---|
| Usage report | CSV, PDF | Daily usage, bandwidth, costs |
| Invoice | PDF | Stripe-generated invoice |
| Node list | CSV | All nodes with metadata |
| Analytics | CSV | Charts data export |
| Audit log | CSV | Security events |

**API:** Add `?format=csv` query parameter to existing endpoints, `?format=pdf` for PDF generation.

---

## 9. Security Features

### 9.1 Two-Factor Authentication (2FA)

**Priority:** P1 | **Complexity:** M

**Implementation:** TOTP (Time-based One-Time Password) using Google Authenticator, Authy, or any TOTP app.

**Flow:**
```
[Settings → Enable 2FA] → [Show QR code + secret] → [User scans with authenticator app]
         │
         ▼
[Enter 6-digit code to verify] → [Generate 8 backup codes] → [2FA active]
         │
         ▼
[Next login: Email + Password + 6-digit TOTP code]
```

**Data Model:**
```sql
ALTER TABLE users ADD COLUMN totp_secret VARCHAR(100); -- encrypted
ALTER TABLE users ADD COLUMN totp_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN totp_verified_at TIMESTAMPTZ;

CREATE TABLE backup_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    code_hash VARCHAR(255) NOT NULL, -- bcrypt hash
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| POST | `/api/auth/2fa/setup` | Generate TOTP secret + QR code |
| POST | `/api/auth/2fa/verify` | Verify TOTP and enable 2FA |
| POST | `/api/auth/2fa/disable` | Disable 2FA (requires current code) |
| POST | `/api/auth/2fa/backup-codes` | Regenerate backup codes |
| POST | `/api/auth/login` | Modified to accept `totpCode` field |

**Libraries:** `otplib` (Node.js), `qrcode` for QR generation

**Acceptance Criteria:**
- [ ] User can enable 2FA via TOTP
- [ ] Login requires TOTP code when 2FA enabled
- [ ] 8 single-use backup codes generated
- [ ] User can disable 2FA (requires TOTP code + password)
- [ ] Admin can force-disable 2FA for locked-out users
- [ ] 2FA enforced for admin accounts (mandatory)

### 9.2 API Key Rotation

**Priority:** P1 | **Complexity:** S

**Current:** Keys can be created/deleted. Need rotation (create new, deprecate old with grace period).

**Flow:**
```
[Rotate Key] → [New key generated] → [Old key has 24h grace period] → [Old key expires]
```

**API:** `POST /api/proxy/keys/:keyId/rotate` — returns new key, marks old as expiring

### 9.3 IP Whitelisting

**Priority:** P1 | **Complexity:** S | **Status:** Partially built (per API key)

**Enhancement:** Account-level IP whitelist (in addition to per-key whitelist).

**Data Model:**
```sql
ALTER TABLE users ADD COLUMN ip_whitelist JSONB DEFAULT '[]'; -- account-level
-- Per-key whitelist already exists in api_keys.ip_whitelist
```

### 9.4 Session Management

**Priority:** P2 | **Complexity:** M

**Features:**
- View active sessions (device, IP, location, last active)
- Terminate individual sessions
- Terminate all sessions ("log out everywhere")
- Session timeout configuration (15/30/60/120 min)
- Email notification on login from new device/location

**Data Model:**
```sql
CREATE TABLE user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    token_hash VARCHAR(255) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    device_type VARCHAR(50),
    location VARCHAR(200), -- "Tel Aviv, Israel" (from GeoIP)
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
```

### 9.5 Audit Logs

**Priority:** P1 | **Complexity:** M

**All security-relevant actions logged:**

| Event | Details Captured |
|---|---|
| Login | IP, user agent, success/failure |
| Logout | Token ID |
| Password change | IP |
| 2FA enable/disable | IP |
| API key create/delete/rotate | Key name, IP |
| Webhook create/delete | Webhook URL |
| Plan change | Old plan → new plan |
| Admin actions | Target user, action type |
| IP whitelist change | Old list → new list |
| Cashout request | Amount, method |

**Data Model:**
```sql
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50), -- user, api_key, webhook, etc.
    resource_id VARCHAR(255),
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_user ON audit_logs (user_id, created_at DESC);
CREATE INDEX idx_audit_action ON audit_logs (action, created_at DESC);
```

**API:** `GET /api/audit-logs?page=1&limit=50&action=login` (admin or own user)

### 9.6 Rate Limiting

**Priority:** P0 | **Complexity:** M | **Status:** Limiter module exists in Go, not enforced per API key

**Rate limits by plan:**

| Plan | Requests/sec | Requests/day | Concurrent |
|---|---|---|---|
| Trial | 2 | 1,000 | 5 |
| Starter | 10 | 10,000 | 10 |
| Growth | 50 | 50,000 | 50 |
| Business | 200 | 200,000 | 200 |
| Enterprise | Custom | Custom | Custom |

**Implementation:**
- Redis-based token bucket or sliding window
- Key: `ratelimit:{user_id}:{window}`
- Response headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
- 429 response when exceeded with `Retry-After` header

### 9.7 DDoS Protection

**Priority:** P2 | **Complexity:** M

- Cloudflare in front of all public endpoints
- Rate limiting at nginx level (connection limit per IP)
- WebSocket connection limit per IP
- Automated blocking of IPs with > 100 failed auth attempts/hour

### 9.8 GDPR Compliance

**Priority:** P1 | **Complexity:** L

**Requirements:**
- Data processing agreement (DPA) for customers
- Right to deletion (account + all data)
- Data export (all user data as JSON/CSV)
- Cookie consent banner on website
- Privacy policy covering SDK data collection
- Node operator consent for bandwidth sharing
- Data retention policy (90 days usage logs, 7 years financial)

**API Endpoints:**
| Method | Path | Description |
|---|---|---|
| GET | `/api/privacy/export` | Export all user data (GDPR) |
| DELETE | `/api/privacy/delete` | Request account deletion |
| GET | `/api/privacy/policy` | Current privacy policy version |

---

## 10. Admin Panel Enhancements

### 10.1 Customer Support Tools

**Priority:** P1 | **Complexity:** M

**Features:**
- View customer's dashboard as them (impersonation/view-only)
- Add manual credits to customer account
- Override plan limits temporarily
- Send direct message/notification to customer
- Ticket system integration (or built-in)
- Customer activity timeline

**API Endpoints:**

| Method | Path | Description |
|---|---|---|
| POST | `/api/admin/users/:id/impersonate` | Get JWT for user (read-only) |
| POST | `/api/admin/users/:id/credit` | Add credits manually |
| POST | `/api/admin/users/:id/notify` | Send notification |
| GET | `/api/admin/users/:id/activity` | Activity timeline |
| POST | `/api/admin/users/:id/override` | Temporary plan override |

### 10.2 Node Blacklisting/Moderation

**Priority:** P1 | **Complexity:** S

**Features:**
- Ban individual nodes by device ID
- Ban IP ranges
- Ban by ASN (block entire ISP)
- Temporary vs permanent bans
- Reason tracking
- Auto-ban rules (quality < 10, > 50% error rate)

**Data Model:**
```sql
CREATE TABLE node_bans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ban_type VARCHAR(20) NOT NULL, -- device_id, ip, ip_range, asn
    ban_value VARCHAR(255) NOT NULL,
    reason TEXT,
    banned_by UUID REFERENCES users(id),
    permanent BOOLEAN DEFAULT FALSE,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 10.3 Financial Reporting

**Priority:** P1 | **Complexity:** M

- Revenue report (MRR, ARR, by plan, by customer)
- Expense tracking (server costs, partner payouts, cashouts)
- Profit/loss statement
- Partner payout management (approve, process batch payouts)
- Tax report generation

### 10.4 System Health Monitoring

**Priority:** P0 | **Complexity:** M

**Dashboard widgets:**
- CPU/Memory/Disk of all services
- Docker container status
- PostgreSQL connection pool usage
- Redis memory usage
- WebSocket connections count
- Proxy gateway throughput
- Error rate (5xx responses)
- Latency percentiles (p50, p95, p99)

### 10.5 Feature Flags

**Priority:** P2 | **Complexity:** M

```sql
CREATE TABLE feature_flags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    rollout_percentage INTEGER DEFAULT 0, -- 0-100
    target_users JSONB DEFAULT '[]', -- specific user IDs
    target_plans JSONB DEFAULT '[]', -- specific plan IDs
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**API:** `GET /api/features` — returns features enabled for current user

### 10.6 A/B Testing

**Priority:** P3 | **Complexity:** L

- Experiment definition (name, variants, traffic split)
- User assignment (consistent, based on user ID hash)
- Event tracking (conversion events)
- Statistical significance calculator
- Dashboard showing experiment results

---

## 11. Mobile Apps

### 11.1 Node Operator App (ProxyClaw)

**Priority:** P2 | **Complexity:** XL

**Tech:** React Native (Expo) — scaffold already exists

**Platforms:** Android (P0), iOS (P2)

**Screens:**
1. **Splash/Onboarding** — 3-screen tutorial (what ProxyClaw is, how it works, start earning)
2. **Login/Register** — email + password, social login (Google, Apple)
3. **Home** — earnings today, balance, active device status, bandwidth sharing toggle
4. **Earnings** — charts (daily/weekly/monthly), earnings history
5. **Devices** — manage connected devices
6. **Cashout** — request payout, history
7. **Referrals** — referral code, share buttons, referral stats
8. **Settings** — profile, notifications, bandwidth limit, theme
9. **Support** — FAQ, contact support

**Key Features:**
- Background service to share bandwidth (Android)
- Push notifications (earnings milestones, referral sign-ups)
- Dark/light theme
- Offline support (cache last known data)
- Biometric login (fingerprint/Face ID)

### 11.2 Customer App (IPLoop)

**Priority:** P3 | **Complexity:** XL

**Screens:**
1. **Dashboard** — network stats, active nodes, bandwidth usage
2. **Proxy Config** — manage targeting, sessions
3. **API Keys** — view/create keys
4. **Usage** — analytics charts
5. **Billing** — plan info, payment management
6. **Settings** — account, 2FA, notifications

---

## 12. API v2

### 12.1 RESTful API Improvements

**Priority:** P1 | **Complexity:** L

**Improvements over current API:**
- Consistent response format: `{ success: bool, data: {}, error: { code, message }, meta: { page, limit, total } }`
- API versioning via URL prefix: `/api/v2/...`
- Pagination on all list endpoints: `?page=1&limit=20`
- Filtering: `?status=active&country=US`
- Sorting: `?sort=created_at&order=desc`
- Field selection: `?fields=id,email,status`
- Rate limit headers on every response
- ETag/If-None-Match caching
- CORS configuration

### 12.2 WebSocket Real-Time Feeds

**Priority:** P2 | **Complexity:** M

**Channels:**
- `nodes` — node connect/disconnect events
- `usage` — real-time usage counters
- `alerts` — billing/security alerts
- `system` — health status changes

**Protocol:**
```json
// Subscribe
{"action": "subscribe", "channel": "nodes"}

// Receive
{"channel": "nodes", "event": "node.connected", "data": {"country": "US", "city": "Miami"}}
```

### 12.3 GraphQL Consideration

**Priority:** P3 | **Complexity:** XL

**Decision:** Defer. REST v2 covers all use cases. GraphQL adds complexity without sufficient benefit for current user base. Revisit when API consumers request it.

### 12.4 Rate Limiting Tiers

Already covered in Section 9.6.

### 12.5 API Versioning Strategy

- URL-based: `/api/v1/`, `/api/v2/`
- v1 supported for 12 months after v2 launch
- Deprecation headers: `Sunset: <date>`, `Deprecation: <date>`
- Migration guide documentation
- v1 → v2 changelog

---

## 13. White-Label / Reseller

### 13.1 Overview

**Priority:** P3 | **Complexity:** XL

Allow companies to resell IPLoop proxy service under their own brand.

**How competitors do it:**
- **Oxylabs:** Full white-label program for resellers
- **Bright Data:** Reseller program with volume discounts
- **Smartproxy:** White-label dashboard, custom domain, branding

### 13.2 White-Label Features

| Feature | Description |
|---|---|
| Custom domain | `proxy.theirbrand.com` (CNAME) |
| Custom branding | Logo, colors, company name throughout dashboard |
| Custom email templates | Branded emails from their domain |
| Sub-account management | Create/manage end-customer accounts |
| Custom pricing | Set their own pricing for end-customers |
| Usage reporting | Per-sub-account usage |
| Billing integration | Reseller bills their customers directly |
| API rebranding | Custom endpoint URLs |

### 13.3 Reseller Dashboard

```
┌─────────────────────────────────────────────────────────────────┐
│  [Reseller Brand] Reseller Dashboard                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │ Customers│ │ Revenue  │ │ Bandwidth│ │ Profit   │          │
│  │ 45       │ │ $8,900   │ │ 450 GB   │ │ $3,560   │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
│                                                                 │
│  ┌─ Sub-Accounts ─────────────────────────────────────────────┐ │
│  │  Company A — Growth — 25 GB — $149/mo                     │ │
│  │  Company B — Starter — 5 GB — $49/mo                      │ │
│  │  [+ Create Sub-Account]                                    │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**Data Model:**
```sql
CREATE TABLE reseller_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    company_name VARCHAR(200) NOT NULL,
    custom_domain VARCHAR(255),
    branding JSONB, -- {logo_url, primary_color, secondary_color, company_name}
    wholesale_discount DECIMAL(5,2) DEFAULT 30.00, -- % discount on retail
    max_sub_accounts INTEGER DEFAULT 100,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reseller_sub_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reseller_id UUID NOT NULL REFERENCES reseller_accounts(id),
    user_id UUID NOT NULL REFERENCES users(id), -- the end customer
    custom_plan JSONB, -- reseller-defined plan override
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 14. Marketing & Growth

### 14.1 Landing Pages

**Priority:** P1 | **Complexity:** M

**Pages needed:**

| Page | URL | Purpose |
|---|---|---|
| Main landing | iploop.io | Product overview, pricing, sign-up CTA |
| Earn/ProxyClaw | proxyclaw.io or iploop.io/earn | Node operator recruitment |
| Pricing | iploop.io/pricing | Plan comparison |
| For Developers | iploop.io/developers | API docs, code examples |
| For Enterprise | iploop.io/enterprise | Custom solutions, contact sales |
| Partners | iploop.io/partners | SDK partner program info |
| Affiliates | iploop.io/affiliates | Affiliate program info |
| Use Cases | iploop.io/use-cases/* | Web scraping, ad verification, etc. |
| About | iploop.io/about | Company story |
| Contact | iploop.io/contact | Contact form |

**Tech:** Static generation (Next.js SSG) for SEO performance.

### 14.2 Email Automation

**Priority:** P1 | **Complexity:** M

**Email sequences (using Resend API):**

| Sequence | Trigger | Emails |
|---|---|---|
| **Customer Onboarding** | Registration | Welcome (0h), Getting Started (1d), First API Key (3d), Need Help? (7d) |
| **Node Operator Onboarding** | Earn sign-up | Welcome (0h), Install Guide (1d), First Earnings (3d), Invite Friends (7d) |
| **Re-engagement** | 14d inactive | We miss you (14d), Special offer (21d), Last chance (28d) |
| **Usage alerts** | Threshold | 80% usage, 100% usage, auto-upgrade offer |
| **Billing** | Payment event | Receipt, failed payment, retry, final warning |
| **Referral** | Referral event | Someone signed up, someone earned |
| **Weekly digest** | Scheduled | Weekly usage summary, earnings summary |

### 14.3 In-App Notifications

**Priority:** P2 | **Complexity:** M

**Notification types:**
- Usage alerts (approaching limit)
- New feature announcements
- System maintenance notices
- Earnings milestones
- Referral activity

**Implementation:**
```sql
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    type VARCHAR(50) NOT NULL,
    title VARCHAR(200) NOT NULL,
    message TEXT NOT NULL,
    action_url VARCHAR(500),
    read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**API:**
| Method | Path | Description |
|---|---|---|
| GET | `/api/notifications?unread=true` | List notifications |
| PUT | `/api/notifications/:id/read` | Mark as read |
| PUT | `/api/notifications/read-all` | Mark all as read |

### 14.4 Status Page

**Priority:** P2 | **Complexity:** M

**URL:** `status.iploop.io`

**Components monitored:**
- Dashboard (website)
- Proxy Gateway (HTTP)
- Proxy Gateway (SOCKS5)
- API
- Node Registration (WebSocket)
- Database

**Features:**
- Real-time status indicators (operational, degraded, outage)
- 90-day uptime history bars
- Incident reports with timeline
- Email/SMS subscription for status updates
- Scheduled maintenance announcements

**Implementation:** Use open-source Upptime (GitHub Actions based) or Cachet, or build custom.

### 14.5 Documentation Site

**Priority:** P1 | **Complexity:** L

**URL:** `docs.iploop.io`

**Sections:**
1. Quick Start (5-minute guide)
2. Authentication (API keys, auth formats)
3. Proxy Configuration (targeting, sessions, rotation)
4. API Reference (auto-generated from OpenAPI spec)
5. SDK Integration (Android, iOS, Windows, Linux, Docker)
6. Code Examples (cURL, Python, Node.js, Java, Go, PHP, Ruby, C#)
7. Webhooks (setup, events, verification)
8. Billing (plans, usage, credits)
9. FAQ
10. Changelog
11. Rate Limits
12. Error Reference

**Tech:** Nextra (Next.js based), Mintlify, or Docusaurus

---

## 15. Implementation Roadmap

### Phase 1: Revenue Foundation (Weeks 1-4) — P0

| Feature | Section | Effort |
|---|---|---|
| Stripe billing (full) | 5 | XL |
| Rate limiting enforcement | 9.6 | M |
| Cashout system (PayPal + USDT) | 3.5 | XL |
| Node operator dashboard (basic) | 3.6 | L |
| Credit system backend | 3.4 | L |
| Docker node image | 3.2.3 | S |
| System health monitoring | 10.4 | M |

### Phase 2: Growth Features (Weeks 5-8) — P1

| Feature | Section | Effort |
|---|---|---|
| 2FA | 9.1 | M |
| Referral system (node operators) | 3.7 | M |
| Affiliate program (customers) | 4 | L |
| Email automation | 14.2 | M |
| Webhook retry/delivery logs | Existing PRD | M |
| Landing pages | 14.1 | M |
| Documentation site | 14.5 | L |
| Audit logs | 9.5 | M |
| GDPR compliance | 9.8 | L |
| SDK partner portal (basic) | 6 | XL |

### Phase 3: Enhancement (Weeks 9-12) — P2

| Feature | Section | Effort |
|---|---|---|
| Real-time dashboard | 8.1 | L |
| Geographic heatmap | 8.2 | M |
| Windows desktop app | 3.2.4 | L |
| Linux CLI | 3.2.5 | M |
| Browser fingerprint profiles | 7.3 | M |
| Quality scoring | 7.4 | M |
| Status page | 14.4 | M |
| In-app notifications | 14.3 | M |
| Feature flags | 10.5 | M |
| Session management | 9.4 | M |
| API v2 | 12.1 | L |
| Revenue analytics | 8.4 | M |
| Node operator mobile app | 11.1 | XL |

### Phase 4: Scale (Weeks 13+) — P3

| Feature | Section | Effort |
|---|---|---|
| White-label/reseller | 13 | XL |
| iOS SDK | Existing PRD | L |
| macOS SDK | Existing PRD | L |
| WebSocket real-time feeds | 12.2 | M |
| A/B testing | 10.6 | L |
| Customer mobile app | 11.2 | XL |
| Smart TV apps | 3.2.1 | XL |
| WireGuard VPN mode | 7.6 | L |

### Total Estimated Effort

| Phase | Duration | Features | Priority |
|---|---|---|---|
| Phase 1 | 4 weeks | 7 features | P0 |
| Phase 2 | 4 weeks | 10 features | P1 |
| Phase 3 | 4 weeks | 13 features | P2 |
| Phase 4 | Ongoing | 8 features | P3 |

---

## 16. Appendix: Competitor Analysis

### 16.1 Pricing Comparison

| Provider | Residential/GB | Mobile/GB | Min Plan | Free Trial |
|---|---|---|---|---|
| **Bright Data** | $5.04 | $17.50 | $500/mo | 7-day |
| **Oxylabs** | $8.00 | $15.00 | $99/mo | 7-day |
| **SOAX** | $3.60 | $3.60 | $90/mo | 3-day |
| **Smartproxy** | $4.00 | $7.00 | $80/mo | 3-day |
| **NetNut** | $6.00 | — | $300/mo | 7-day |
| **IPRoyal** | $1.75 | — | Pay-as-you-go | No |
| **PacketStream** | $1.00 | — | Pay-as-you-go | No |
| **IPLoop** | $4.99 | — | $49/mo | 0.5 GB free |

### 16.2 Earn/Node Operator Comparison

| Provider | Rate/GB | Min Payout | Payout Methods | Referral | Platforms |
|---|---|---|---|---|---|
| **Honeygain** | $0.10 | $20 | PayPal, BTC, JumpTask | 10% | Win, Mac, Linux, Android, Docker |
| **EarnApp** | $0.10-0.25 | $2.50 | PayPal, Amazon | 10% | Win, Mac, Linux, Android, Raspberry Pi |
| **PacketStream** | $0.10 | $5 | PayPal | 20% | Win, Mac, Linux, Docker |
| **IPRoyal Pawns** | $0.10-0.70 | $5 | PayPal, BTC, ETH, USDT, Revolut | 10% | Win, Mac, Linux, Android, Docker |
| **Repocket** | $0.10 | $20 | PayPal, BTC | 5% | Win, Mac, Linux, Android, Docker |
| **Peer2Profit** | $0.10 | $2 | BTC, USDT, LTC, Wire | — | Win, Mac, Linux, Android, Docker |
| **ProxyClaw** | $0.10-0.15 | $5 | PayPal, USDT, BTC, Bank, Amazon | 10% | Android, Docker, Win, Linux, Mac |

### 16.3 Affiliate Program Comparison

| Provider | Commission | Type | Duration | Min Payout | Platform |
|---|---|---|---|---|---|
| **Bright Data** | 50% rev share | Recurring (capped $2.5K/customer) | Ongoing | — | PartnerStack |
| **Oxylabs** | 30-50% | Custom | 12 months | $100 | In-house |
| **SOAX** | Up to 30% | Recurring | Ongoing | $50 | CJ Affiliate |
| **Smartproxy** | 50% first + 15% recurring | Hybrid | Ongoing | $50 | In-house |
| **IPLoop** | 20-30% | Recurring (12 months) | 12 months | $50 | In-house |

### 16.4 Feature Comparison Matrix

| Feature | Bright Data | Oxylabs | SOAX | IPLoop (planned) |
|---|---|---|---|---|
| Residential proxy | ✅ | ✅ | ✅ | ✅ |
| Mobile proxy | ✅ | ✅ | ✅ | ✅ (via mobile nodes) |
| Datacenter proxy | ✅ | ✅ | ✅ | ❌ |
| ISP proxy | ✅ | ✅ | ✅ | ❌ |
| Geo targeting (country) | ✅ | ✅ | ✅ | ✅ |
| Geo targeting (city) | ✅ | ✅ | ✅ | ✅ |
| ASN targeting | ✅ | ✅ | ✅ | ✅ |
| Sticky sessions | ✅ | ✅ | ✅ | ✅ |
| HTTP/HTTPS | ✅ | ✅ | ✅ | ✅ |
| SOCKS5 | ✅ | ✅ | ✅ | ✅ |
| API management | ✅ | ✅ | ✅ | ✅ |
| Dashboard | ✅ | ✅ | ✅ | ✅ |
| Pay-as-you-go | ✅ | ❌ | ✅ | ✅ |
| White-label | ✅ | ✅ | ❌ | 🔜 |
| Web scraping API | ✅ | ✅ | ✅ | ❌ |
| Browser API | ✅ | ✅ | ❌ | ❌ |
| 2FA | ✅ | ✅ | ✅ | 🔜 |
| Affiliate program | ✅ | ✅ | ✅ | 🔜 |
| SDK partner program | ✅ | ❌ | ❌ | 🔜 |

---

## 17. Technical Dependencies

### 17.1 Third-Party Services Required

| Service | Purpose | Priority | Estimated Cost |
|---|---|---|---|
| **Stripe** | Billing & payments | P0 | 2.9% + $0.30 per txn |
| **Resend** | Transactional email | P0 | Free tier (3K/mo) → $20/mo |
| **PayPal Payouts** | Node operator cashouts | P0 | $0.25/payout |
| **Coinbase Commerce** | Crypto cashouts | P0 | 1% per transaction |
| **Cloudflare** | CDN, DDoS, DNS | P0 | Free → Pro ($20/mo) |
| **Mapbox** | Geographic heatmap | P2 | Free tier (50K loads/mo) |
| **Twilio** | SMS for 2FA (optional) | P2 | $0.0079/SMS |
| **Sentry** | Error tracking | P1 | Free tier (5K events/mo) |
| **PostHog** | Product analytics | P2 | Free tier (1M events/mo) |

### 17.2 Infrastructure Scaling

| Current | Target (6 months) | Target (12 months) |
|---|---|---|
| 1 main server + 1 gateway | 2 main + 2 gateway (HA) | 4+ gateway (load balanced) |
| 10K peak nodes | 50K peak nodes | 200K+ peak nodes |
| ~100 customers | 1,000 customers | 10,000 customers |
| Single PostgreSQL | PostgreSQL + read replicas | PostgreSQL cluster + TimescaleDB |
| Single Redis | Redis Sentinel | Redis Cluster |

---

## 18. Success Metrics

### 18.1 Supply Side (ProxyClaw)

| Metric | Current | 3-Month Target | 6-Month Target |
|---|---|---|---|
| Registered node operators | ~0 | 5,000 | 25,000 |
| Active nodes (peak) | 10,050 | 30,000 | 100,000 |
| Countries covered | 111 | 150 | 190+ |
| Avg node uptime | Unknown | > 8 hours/day | > 12 hours/day |
| Monthly cashouts processed | 0 | $5,000 | $25,000 |

### 18.2 Demand Side (IPLoop)

| Metric | Current | 3-Month Target | 6-Month Target |
|---|---|---|---|
| Paying customers | ~0 | 50 | 200 |
| MRR | $0 | $5,000 | $25,000 |
| Bandwidth sold/month | ~0 | 500 GB | 5 TB |
| Average customer LTV | — | $500 | $1,000 |
| Churn rate | — | < 10% | < 7% |

### 18.3 Platform Health

| Metric | Target |
|---|---|
| Proxy success rate | > 95% |
| Average response time | < 3 seconds |
| Platform uptime | > 99.5% |
| API latency (p95) | < 200ms |
| Node connection success | > 98% |

---

*End of Future Development PRD. This document should enable a development team to build the complete IPLoop + ProxyClaw platform from current state to full commercial launch. Last updated: February 17, 2026.*