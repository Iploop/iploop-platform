# IPLoop Platform — Product Requirements Document (PRD)

**Version:** 1.0  
**Date:** February 17, 2026  
**Author:** IPLoop Engineering  
**Audience:** External UX/UI Designer  
**Status:** For Redesign

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Product Overview & Architecture](#2-product-overview--architecture)
3. [User Personas](#3-user-personas)
4. [Authentication & Onboarding Flows](#4-authentication--onboarding-flows)
5. [Dashboard Pages — Complete Specification](#5-dashboard-pages--complete-specification)
6. [Data Models](#6-data-models)
7. [API Endpoints Reference](#7-api-endpoints-reference)
8. [SDK Management](#8-sdk-management)
9. [Node & Device Management](#9-node--device-management)
10. [Proxy Configuration](#10-proxy-configuration)
11. [Customer Management & Billing](#11-customer-management--billing)
12. [Analytics & Reporting](#12-analytics--reporting)
13. [Admin Panel](#13-admin-panel)
14. [Webhooks & Notifications](#14-webhooks--notifications)
15. [Credit / Earnings System](#15-credit--earnings-system)
16. [Error States & Edge Cases](#16-error-states--edge-cases)
17. [Responsive Design Requirements](#17-responsive-design-requirements)
18. [Feature Status Matrix](#18-feature-status-matrix)
19. [Glossary](#19-glossary)

---

## 1. Executive Summary

**IPLoop** is a residential proxy platform that aggregates bandwidth from real mobile and desktop devices (via an SDK embedded in partner apps) and sells it as a proxy service to customers who need web scraping, ad verification, price comparison, and market research capabilities.

### Business Model

```
SDK Partners (supply side)          IPLoop Platform           Proxy Customers (demand side)
┌─────────────────────┐      ┌──────────────────────┐      ┌─────────────────────┐
│ Android/iOS/Desktop  │      │  Gateway Server       │      │ Web scrapers         │
│ apps embed IPLoop SDK│─────▶│  Node Registration    │◀─────│ Ad verification      │
│ → share bandwidth    │      │  Proxy Gateway        │      │ Price monitoring      │
│ → earn revenue share │      │  Customer API         │      │ Market research       │
└─────────────────────┘      │  Dashboard            │      └─────────────────────┘
                              └──────────────────────┘
```

### Key Metrics (Production)
- **Peak nodes connected:** 10,050+ devices
- **Countries covered:** 111+
- **Steady-state nodes:** ~5,900
- **Protocols:** HTTP, HTTPS (CONNECT), SOCKS5
- **SDK:** Pure Java (Android 5.1+), v1.0.57

### Platform URL
- **Dashboard:** https://iploop.io
- **Gateway:** gateway.iploop.io (WSS for nodes, HTTP/SOCKS5 for proxy)

---

## 2. Product Overview & Architecture

### 2.1 System Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Dashboard   │     │  Customer    │     │   Billing    │
│  (Next.js 14) │────▶│    API       │────▶│  (Go/Stripe) │
│  Port: 3000   │     │ (Node.js)    │     │  Port: 8003  │
└──────────────┘     │ Port: 3001   │     └──────────────┘
                      └──────────────┘
                             │
                      ┌──────┴──────┐
                      │             │
               ┌──────────┐  ┌──────────┐
               │ PostgreSQL│  │  Redis   │
               │  Port:5432│  │ Port:6379│
               └──────────┘  └──────────┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
┌──────────────┐┌──────────────┐┌──────────────┐
│    Node      ││   Proxy      ││  Autoscaler  │
│ Registration ││  Gateway     ││              │
│  (Go)        ││  (Go)        ││  (Go)        │
│ Port: 8001   ││ HTTP: 7777   ││ Port: 8090   │
│ WSS: /ws     ││ SOCKS5: 1080 ││              │
└──────────────┘└──────────────┘└──────────────┘
        ▲
        │ WebSocket (WSS)
        │
┌──────────────┐
│  SDK Devices  │
│ (Android/etc) │
│ 10,000+ nodes │
└──────────────┘
```

### 2.2 Technology Stack

| Component | Technology |
|---|---|
| Dashboard | Next.js 14, React, TypeScript, Tailwind CSS, shadcn/ui, Recharts |
| Customer API | Node.js, Express, JWT, bcrypt, Joi validation |
| Proxy Gateway | Go, HTTP CONNECT proxy, SOCKS5 |
| Node Registration | Go, WebSocket hub, GeoIP |
| Billing | Go, Stripe integration |
| Database | PostgreSQL 15 |
| Cache | Redis 7 |
| SDK | Pure Java (Android min SDK 22) |
| Monitoring | Prometheus + Grafana |
| Containerization | Docker Compose |
| SSL | Nginx + Let's Encrypt |

### 2.3 Docker Services

| Service | Container Name | Port(s) | Description |
|---|---|---|---|
| postgres | iploop-postgres | 5432 | Primary database |
| redis | iploop-redis | 6379 | Caching & sessions |
| proxy-gateway | iploop-proxy-gateway | 7777, 1080 | HTTP & SOCKS5 proxy |
| node-registration | iploop-node-registration | 8001 | WebSocket hub for SDK devices |
| customer-api | iploop-customer-api | 3001 | REST API for dashboard |
| billing | iploop-billing | 8003 | Stripe billing service |
| dashboard | iploop-dashboard | 3000 | Next.js web UI |
| nginx-proxy | iploop-nginx-proxy | 3000 | Routes traffic to dashboard/home |
| prometheus | iploop-prometheus | 9090 | Metrics collection |
| grafana | iploop-grafana | 3001 | Metrics visualization |

---

## 3. User Personas

### 3.1 Proxy Customer (Demand Side)

| Attribute | Description |
|---|---|
| **Role** | Web scraper, ad verifier, price monitor, SEO researcher |
| **Goal** | Access residential IPs from specific countries to gather data |
| **Tech Level** | Developer or technical ops person |
| **Key Actions** | Sign up, generate API keys, configure proxy endpoints, monitor usage, manage billing |
| **Pain Points** | IP blocking, slow proxies, expensive bandwidth, complex setup |
| **Plans** | Starter ($49/mo, 5GB), Growth ($149/mo, 25GB), Business ($499/mo, 100GB), Pay-as-you-go ($5/GB) |

### 3.2 SDK Partner (Supply Side)

| Attribute | Description |
|---|---|
| **Role** | App developer who embeds IPLoop SDK for revenue |
| **Goal** | Monetize their app's idle bandwidth without affecting UX |
| **Tech Level** | Mobile developer (Android/iOS) |
| **Key Actions** | Integrate SDK, monitor node count & earnings, get revenue share |
| **Revenue** | Configurable revenue share (default 70%) |
| **Managed Via** | Admin panel → Partners section |

### 3.3 Platform Admin

| Attribute | Description |
|---|---|
| **Role** | IPLoop team member managing the platform |
| **Goal** | Monitor network health, manage customers, configure plans, track revenue |
| **Key Actions** | User management, plan configuration, node monitoring, partner management |
| **Access** | Admin-only pages in dashboard (role-gated) |

---

## 4. Authentication & Onboarding Flows

### 4.1 Registration Flow

```
[Landing Page] → [Sign Up Form] → [Email Verification] → [Dashboard]
                      │
                      ▼
              Fields:
              - Email (required)
              - Password (min 8 chars)
              - Confirm Password
              - First Name (required)
              - Last Name (required)
              - Company (optional)
```

**Post-registration:**
- Auto-assigned to **Starter plan** with **5 GB free** balance
- JWT token generated (24h expiry)
- Stored in localStorage: `token`, `user` object
- Redirect to `/dashboard`

**API:** `POST /api/auth/register`

### 4.2 Login Flow

```
[Login Page] → [Email + Password] → [JWT Token] → [Dashboard]
```

**Fields:**
- Email (required)
- Password (required)
- "Show password" toggle (eye icon)
- "Forgot password?" link
- "Don't have an account? Sign Up" link

**API:** `POST /api/auth/login`

**Response stores:** `token` and `user` (id, email, firstName, lastName, company, role) in localStorage.

### 4.3 Password Reset Flow ✅

```
[Login] → "Forgot Password?" → [Enter Email] → [Reset Email Sent]
                                                        │
                                                        ▼
                                              [Reset Password Page]
                                              - New password
                                              - Confirm password
                                                        │
                                                        ▼
                                              [Password Updated → Login]
```

**APIs:**
- `POST /api/auth/forgot-password` — sends reset email (1h token expiry)
- `GET /api/auth/verify-reset-token?token=xxx` — validates token
- `POST /api/auth/reset-password` — sets new password

### 4.4 Email Verification ✅

- `POST /api/auth/resend-verification` — resends verification email
- `GET /api/auth/verify-email?token=xxx` — marks email as verified

### 4.5 Session Management

- JWT tokens stored in localStorage
- 24h expiry by default
- Token blacklisting on logout (stored in Redis with TTL)
- `POST /api/auth/logout` — blacklists current token

---

## 5. Dashboard Pages — Complete Specification

### 5.1 Sidebar Navigation

The sidebar is the primary navigation element. It appears on all authenticated pages.

| Nav Item | Path | Icon | Admin Only | Description |
|---|---|---|---|---|
| Dashboard | `/dashboard` | BarChart3 | No | Main overview |
| Nodes | `/nodes` | Smartphone | No | Connected devices |
| Analytics | `/analytics` | TrendingUp | No | Usage analytics |
| API Keys | `/api-keys` | Key | No | API key management |
| Webhooks | `/webhooks` | Webhook | No | Webhook configuration |
| Proxy Endpoints | `/endpoints` | Globe | No | Connection details |
| Docs | `/docs` | Book | No | API documentation |
| AI Team | `/ai-team` | Users | No | AI agents overview |
| AI Assistant | `/ai-assistant` | Bot | No | AI chat support |
| Billing | `/billing` | CreditCard | No | Plans & invoices |
| Settings | `/settings` | Settings | No | Account settings |
| Support | `/support` | MessageCircle | No | Live support chat |
| Accounts | `/admin/users` | Users | **Yes** | User management |
| Admin | `/admin` | ShieldCheck | **Yes** | Admin dashboard |

**Sidebar behavior:**
- Collapsed on mobile (hamburger menu toggle)
- Shows current user name and email at bottom
- Logout button at bottom
- Active item highlighted
- Admin items only visible when `user.role === 'admin'`

---

### 5.2 Dashboard Overview (`/dashboard`) ✅

**Purpose:** Real-time network status overview. The first page users see after login.

#### Stats Grid (4 cards)

| Card | Value Source | Icon | Description |
|---|---|---|---|
| Active Nodes | `health.connected_nodes` or active node count | Smartphone | Currently online devices |
| Total Nodes | `stats.total_nodes` | Users | All registered devices |
| Countries | Unique countries from `stats.country_breakdown` | Globe | Geographic coverage |
| System Status | `health.status` | Activity | "Healthy" or "Unknown" |

#### Active Nodes Panel (left column)

A scrollable list of currently connected nodes. Each entry shows:

| Element | Data |
|---|---|
| Green dot | Status indicator (available = green, busy = yellow, offline = red) |
| IP Address | `node.ip_address` |
| Location | `node.city, node.country` with MapPin icon |
| Device Type badge | `node.device_type` (android, ios, windows, etc.) |
| Connection type | `node.connection_type` with Wifi icon |

#### Network Statistics Panel (right column)

| Metric | Display | Badge Color |
|---|---|---|
| Service Status | `health.status` | Green if healthy, red if not |
| Connected Nodes | `health.connected_nodes` | Secondary |
| Average Quality | `stats.average_quality` + "%" | Secondary |
| Bandwidth Used | `stats.total_bandwidth_mb` + " MB" | Secondary |

#### Global Node Map

- **Component:** WorldMap (dynamic import, client-side only)
- **Data source:** `stats.country_breakdown` (object: `{ "US": 1500, "DE": 300, ... }`)
- **Description:** Interactive world map showing node distribution with color intensity by count
- **Caption:** Shows total country count and active node count

**Data refresh:** Every 30 seconds via `setInterval`

**API:** `GET /api/nodes` — returns `{ nodes, nodeCount, stats, health, timestamp }`

---

### 5.3 Nodes Page (`/nodes`) ✅

**Purpose:** Detailed view of all connected SDK devices.

#### Node List

Table/card view of all nodes. Each node shows:

| Field | Data Type | Description |
|---|---|---|
| Status indicator | Dot (green/yellow/red) | available/busy/offline |
| Device ID | string | `device_id` |
| IP Address | string | `ip_address` |
| Country | string | `country` flag + `country_name` |
| City | string | `city` |
| Region | string | `region` |
| ISP | string | `isp` |
| Carrier | string | `carrier` |
| Connection Type | enum | wifi / cellular / ethernet |
| Device Type | enum | android / ios / windows / mac / browser |
| SDK Version | string | `sdk_version` |
| Quality Score | integer 0-100 | Visual bar or badge |
| Bandwidth Used | number | `bandwidth_used_mb` MB |
| Last Heartbeat | timestamp | Relative time ("2m ago") |
| Connected Since | timestamp | Relative time |

**Auto-refresh:** Every 10 seconds

**Filters (🔜 planned):**
- Country dropdown
- Status filter (available/busy/offline)
- Connection type filter
- Search by IP/device ID

**API:** `GET /api/nodes`

---

### 5.4 Analytics Page (`/analytics`) ✅

**Purpose:** Visualize proxy usage with charts and metrics.

#### Summary Cards

| Metric | Field | Description |
|---|---|---|
| Total Requests | `summary.totalRequests` | In selected period |
| Success Rate | `summary.successRate` + "%" | Percentage |
| Bandwidth Used | `summary.totalGbTransferred` + " GB" | Total transfer |
| Avg Response Time | `summary.avgResponseTimeMs` + " ms" | Latency |

#### Period Selector

Buttons: **7 days**, **30 days** (default), **3 months**, **1 year** (🔜)

#### Charts

1. **Daily Usage Area Chart** — Requests over time
   - X-axis: Date
   - Y-axis: Request count
   - Area fill with gradient
   - Tooltip showing date, requests, successful, MB transferred

2. **Country Distribution Pie Chart** — Top countries by usage
   - 6 color palette
   - Legend with country names
   - Tooltip showing request count and MB

3. **Daily Bandwidth Bar Chart** — MB transferred per day
   - X-axis: Date
   - Y-axis: MB

**Data Sources (APIs):**
- `GET /api/usage/summary?days={period}`
- `GET /api/usage/daily?days={period}`
- `GET /api/usage/by-country?days={period}`

**Fallback:** Mock data displayed when no real usage exists (demo mode).

---

### 5.5 API Keys Page (`/api-keys`) ✅

**Purpose:** Generate, manage, and revoke proxy API keys.

#### Create Key Section

| Element | Type | Details |
|---|---|---|
| "+ Create New API Key" button | Button | Opens inline form |
| Key Name input | Text field | Required, min 1 char |
| "Create" button | Submit | Creates key via API |
| "Cancel" button | Button | Closes form |

#### Newly Created Key Alert

When a key is created, a **one-time display** alert shows:
- Full API key (e.g., `iploop_a1b2c3d4...`)
- Copy button
- ⚠️ Warning: "Save this API key now! It will not be shown again."

#### Key List

| Column | Data | Description |
|---|---|---|
| Name | `name` | User-defined name |
| Key Preview | `keyPrefix` | First 14 chars + "..." |
| Status | `isActive` | Green "Active" or red "Inactive" badge |
| Created | `createdAt` | Formatted date |
| Last Used | `lastUsedAt` | Formatted date or "Never" |
| Actions | Buttons | Toggle active, Delete (with confirm) |

**Max keys:** 5 per user

**APIs:**
- `GET /api/proxy/keys` — list keys
- `POST /api/proxy/keys` — create key (body: `{ name }`)
- `DELETE /api/proxy/keys/:keyId` — delete key
- `PATCH /api/proxy/keys/:keyId` — toggle active (body: `{ isActive }`)
- `GET /api/proxy/keys/:keyId` — get key with IP whitelist
- `PUT /api/proxy/keys/:keyId/whitelist` — update IP whitelist (body: `{ ips: string[] }`)

---

### 5.6 Proxy Endpoints Page (`/endpoints`) ✅

**Purpose:** Show customers how to connect to the proxy.

#### Endpoint Cards

| Endpoint | Port | Protocol | Status |
|---|---|---|---|
| HTTP Proxy | 7777 | HTTP/HTTPS | Active ✅ |
| SOCKS5 Proxy | 1080 | SOCKS5 | Active ✅ |

Each card shows:
- Endpoint name
- Host: `proxy.iploop.io`
- Port number
- Protocol description
- Copy button for connection string

#### Code Examples (with copy buttons)

1. **cURL** example
```bash
curl -x http://USERNAME:API_KEY-country-IL@proxy.iploop.io:7777 https://httpbin.org/ip
```

2. **Python** example (requests library)

3. **Node.js** example (axios)

#### Targeting Parameters Documentation

| Parameter | Format | Example | Description |
|---|---|---|---|
| Country | `-country-XX` | `-country-US` | Target country (ISO 2-letter) |
| City | `-city-NAME` | `-city-miami` | Target city |
| ASN | `-asn-XXXXX` | `-asn-12345` | Target ISP/ASN |
| Session | `-session-ID` | `-session-abc123` | Sticky session |
| Session Type | `-sesstype-TYPE` | `-sesstype-sticky` | sticky / rotating / per-request |
| Lifetime | `-lifetime-TIME` | `-lifetime-30` | Session lifetime (minutes) |
| Rotation | `-rotate-MODE` | `-rotate-request` | request / time / manual |
| Browser Profile | `-profile-NAME` | `-profile-chrome-win` | User-agent preset |
| Speed | `-speed-MBPS` | `-speed-10` | Min speed requirement |
| Latency | `-latency-MS` | `-latency-100` | Max latency |
| Debug | `-debug-1` | | Enable debug mode |

---

### 5.7 Usage Page (`/usage`) ✅

**Purpose:** Detailed usage breakdown with exportable data.

#### Time Range Selector

Buttons: 7 days, 30 days, 3 months, 1 year

#### Charts

1. **Daily Usage Area Chart** — Requests, bandwidth, success rate, errors
2. **Hourly Distribution Bar Chart** — Request volume by hour of day
3. **Status Code Pie Chart** — 200 OK, 301, 404, 500, Other (with percentages)

#### Endpoint Usage Table

| Column | Description |
|---|---|
| Endpoint | API path |
| Requests | Total count |
| Avg Response | ms |
| Error Rate | Percentage |

#### Daily Usage Table

| Column | Description |
|---|---|
| Date | Day |
| Requests | Count |
| Bandwidth | GB |
| Success Rate | % |
| Errors | Count |

**APIs:**
- `GET /api/usage/summary?days=N`
- `GET /api/usage/daily?days=N`
- `GET /api/usage/by-country?days=N`
- `GET /api/usage/by-key?days=N`
- `GET /api/usage/recent?limit=50`

---

### 5.8 Webhooks Page (`/webhooks`) ✅

**Purpose:** Configure webhook endpoints for real-time event notifications.

#### Webhook List (Table)

| Column | Description |
|---|---|
| URL | Webhook endpoint URL |
| Events | Badge list of subscribed events |
| Status | Active (green) / Inactive (red) toggle |
| Secret | Masked, with show/hide toggle |
| Last Triggered | Timestamp or "Never" |
| Success Rate | Percentage 🔜 |
| Failure Count | Number |
| Actions | Edit, Test, Delete, Regenerate Secret |

#### Create Webhook Dialog

| Field | Type | Validation |
|---|---|---|
| URL | Text input | Must be valid URL |
| Events | Checkbox list | At least one required |
| Description | Text input | Optional |

#### Available Events

| Event ID | Name | Description |
|---|---|---|
| `usage.threshold` | Usage Threshold | When usage reaches configured limit |
| `balance.low` | Balance Low | When balance drops below threshold |
| `node.connected` | Node Online | When a node comes online |
| `node.disconnected` | Node Offline | When a node goes offline |
| `request.failed` | Request Failed | When a proxy request fails |
| `quota.warning` | Quota Warning | Usage reaches 80% 🔜 |
| `quota.exceeded` | Quota Exceeded | Usage limit reached 🔜 |
| `api_key.created` | API Key Created | New API key generated 🔜 |
| `api_key.deleted` | API Key Deleted | API key removed 🔜 |
| `payment.success` | Payment Success | Payment processed 🔜 |
| `payment.failed` | Payment Failed | Payment failed 🔜 |

#### Test Webhook Button

Sends a test payload with HMAC-SHA256 signature:
```json
{
  "event": "test",
  "timestamp": "2026-02-17T20:00:00Z",
  "data": { "message": "This is a test webhook from IPLoop" }
}
```

Headers: `X-IPLoop-Signature`, `X-IPLoop-Event`

#### Delivery Logs Tab 🔜

| Column | Description |
|---|---|
| Event ID | Unique event identifier |
| Event Type | Event name |
| Status Code | HTTP response code |
| Duration | ms |
| Error | Error message if failed |
| Timestamp | When delivered |

**APIs:**
- `GET /api/webhooks`
- `POST /api/webhooks`
- `PUT /api/webhooks/:id`
- `DELETE /api/webhooks/:id`
- `POST /api/webhooks/:id/test`
- `POST /api/webhooks/:id/regenerate-secret`

---

### 5.9 Billing Page (`/billing`) 🔜 (Partially built)

**Purpose:** Subscription management, plan upgrades, payment history.

#### Current Plan Card

| Element | Description |
|---|---|
| Plan Name | Starter / Growth / Business / Pay-as-you-go |
| Monthly Price | $49 / $149 / $499 / $5/GB |
| Usage Progress Bar | GB used / GB limit (percentage) |
| Renewal Date | Next billing date |
| Cancel/Downgrade button | Subscription management |

#### Plan Comparison Table

| Feature | Starter ($49/mo) | Growth ($149/mo) | Business ($499/mo) | Pay-as-you-go |
|---|---|---|---|---|
| Bandwidth | 5 GB/mo | 25 GB/mo | 100 GB/mo | Unlimited |
| Price per GB | $9.80 | $5.96 | $4.99 | $5.00 |
| Requests/day | 10,000 | 50,000 | 200,000 | 50,000 |
| Concurrent connections | 10 | 50 | 200 | 25 |
| Geo-targeting | Basic | Advanced + city | Advanced + city + ASN | Basic |
| Sticky sessions | ❌ | ✅ | ✅ | ❌ |
| Support | Email | Priority | 24/7 + dedicated manager | Email |
| SLA | — | — | 99.9% | — |

#### Upgrade Button

Triggers Stripe checkout flow (🔜 — Stripe integration ready, test mode).

#### Invoice History Table 🔜

| Column | Description |
|---|---|
| Invoice # | Stripe invoice ID |
| Date | Created date |
| Amount | USD amount |
| Status | Paid / Pending / Failed |
| PDF | Download link |

**APIs (planned):**
- `GET /api/billing/plans`
- `GET /api/billing/subscription`
- `POST /api/billing/checkout`
- `GET /api/billing/invoices`

---

### 5.10 Settings Page (`/settings`) ✅

**Purpose:** Account profile, security, and notification preferences.

#### Profile Section

| Field | Type | Current Behavior |
|---|---|---|
| First Name | Text input | Editable |
| Last Name | Text input | Editable |
| Email | Text input (disabled) | Read-only |
| Company | Text input | Editable |
| Phone | Text input | Editable |
| Timezone | Dropdown | Editable |
| "Save Changes" button | Submit | Updates profile |

#### Password Section

| Field | Type |
|---|---|
| Current Password | Password input with show/hide toggle |
| New Password | Password input with show/hide toggle |
| Confirm Password | Password input with show/hide toggle |
| "Change Password" button | Submit |

**API:** `PUT /api/auth/password`

#### Notification Preferences

| Setting | Type | Default |
|---|---|---|
| Email Alerts | Toggle | ON |
| Usage Alerts | Toggle | ON |
| Billing Alerts | Toggle | ON |
| Security Alerts | Toggle | ON |
| Maintenance Alerts | Toggle | OFF |
| Weekly Reports | Toggle | ON |
| Monthly Reports | Toggle | ON |

#### Security Section 🔜

| Setting | Type | Status |
|---|---|---|
| Two-Factor Authentication | Toggle | 🔜 Planned |
| Session Timeout | Dropdown (15/30/60 min) | 🔜 Planned |
| IP Whitelist | Editable list | 🔜 Planned |

#### API Settings

| Setting | Type |
|---|---|
| Rate Limit Alerts | Toggle |
| Webhook URL | Text input |
| Retry Attempts | Number input |
| Timeout (seconds) | Number input |

#### Danger Zone

- **Delete Account** button (red, requires confirmation dialog)

---

### 5.11 Support Page (`/support`) ✅

**Purpose:** AI-powered live chat support.

#### Chat Interface

- Full-height chat window
- Message bubbles (user = right-aligned, assistant = left-aligned)
- Bot avatar (🤖) for assistant messages
- User avatar for user messages
- Timestamp on each message
- Auto-scroll to bottom on new messages

#### Initial Bot Message

> "Hi! I'm the IPLoop Support Assistant. I can help you with:
> • Getting started with the platform
> • API and SDK integration  
> • Troubleshooting issues
> • General questions
> How can I help you today?"

#### Input Area

- Text input with send button
- Send on Enter key
- Loading indicator while waiting for response
- Disabled during loading

#### Quick Links Panel

- 📖 Documentation link
- 📧 Email: support@iploop.io
- ❓ FAQ link 🔜

**API:** `POST /api/support/chat` (body: `{ message, history }`)

---

### 5.12 Docs Page (`/docs`) ✅

**Purpose:** Interactive API documentation with code examples.

#### Structure

Collapsible sections with code blocks (copy button on each):

1. **Quick Start** — 3-step setup (get key, configure proxy, make request)
2. **Authentication** — API key format, header usage
3. **HTTP Proxy** — cURL, Python, Node.js, Java examples
4. **SOCKS5 Proxy** — Connection examples
5. **Geo-Targeting** — Country/city/ASN targeting syntax
6. **Session Management** — Sticky vs rotating sessions
7. **Response Codes** — Error code reference
8. **SDK Integration** — Android SDK setup guide

#### Quick Links Grid (4 cards)

| Card | Icon | Description |
|---|---|---|
| Quick Start | Zap | Get running in 5 minutes |
| Authentication | Key | API key setup |
| Geo-Targeting | Globe | Country & city targeting |
| SDK | Smartphone | Android SDK integration |

---

### 5.13 Login Page (`/login`) ✅

**Purpose:** Authentication gateway.

#### Layout

- Centered card on dark gradient background
- IPLoop logo + brand name
- Toggle between Login and Sign Up tabs

#### Login Form

| Field | Type | Validation |
|---|---|---|
| Email | Email input | Required, valid email |
| Password | Password input | Required |
| Show Password | Eye icon toggle | |
| Submit button | "Sign In" | Loading state |
| Forgot Password | Link | → `/forgot-password` |
| Switch to Sign Up | Link | Toggles form |

#### Sign Up Form

| Field | Type | Validation |
|---|---|---|
| First Name | Text | Required, min 2 chars |
| Last Name | Text | Required, min 2 chars |
| Email | Email | Required, valid email |
| Company | Text | Optional |
| Password | Password | Required, min 8 chars |
| Confirm Password | Password | Must match |
| Submit button | "Create Account" | Loading state |

#### Error Display

- Red alert banner below form header
- Error messages from API displayed verbatim

---

### 5.14 Forgot Password Page (`/forgot-password`) ✅

| Field | Type |
|---|---|
| Email | Email input |
| "Send Reset Link" button | Submit |
| Back to Login | Link |

### 5.15 Reset Password Page (`/reset-password`) ✅

| Field | Type |
|---|---|
| New Password | Password input |
| Confirm Password | Password input |
| "Reset Password" button | Submit |

Reads `?token=xxx` from URL query parameter.

### 5.16 Email Verification Page (`/verify-email`) ✅

Reads `?token=xxx`, calls API, shows success/error message.

---

### 5.17 Finance Page (`/finance`) ✅

**Purpose:** Financial overview dashboard.

#### Summary Cards (4)

| Card | Description |
|---|---|
| Weekly AI Cost | Platform AI costs |
| Monthly Revenue | Total revenue |
| Pending Invoices | Count + total amount |
| Monthly Projection | Projected AI costs |

#### Navigation Grid (4 link cards)

| Card | Path | Description |
|---|---|---|
| System Costs | `/finance/system-costs` | AI & infrastructure costs |
| Invoices | `/billing` | Billing history |
| Revenue | `/finance/revenue` 🔜 | Revenue tracking |
| Expenses | `/finance/expenses` 🔜 | Budget tracking |

---

### 5.18 AI Team Page (`/ai-team`) ✅

**Purpose:** Overview of AI agents powering the platform.

*Note: Primarily informational. Shows AI team members and their responsibilities.*

### 5.19 AI Assistant Page (`/ai-assistant`) ✅

**Purpose:** Direct chat interface with AI for platform-related questions.

---

### 5.20 DSP/SSP Login Pages 🔜

- `/dsp/login` — Demand-side platform login
- `/ssp/login` — Supply-side platform login

These are planned portal interfaces for programmatic ad-style proxy buying/selling.

---

## 6. Data Models

### 6.1 Users

| Column | Type | Constraints | Description |
|---|---|---|---|
| id | UUID | PK, auto-generated | User identifier |
| email | VARCHAR(255) | UNIQUE, NOT NULL | Login email |
| password_hash | VARCHAR(255) | NOT NULL | bcrypt hash |
| first_name | VARCHAR(100) | NOT NULL | |
| last_name | VARCHAR(100) | NOT NULL | |
| company | VARCHAR(255) | nullable | Company name |
| phone | VARCHAR(20) | nullable | Phone number |
| status | VARCHAR(20) | DEFAULT 'active' | active / suspended / deleted |
| role | VARCHAR(20) | DEFAULT 'customer' | customer / admin |
| email_verified | BOOLEAN | DEFAULT FALSE | |
| created_at | TIMESTAMPTZ | auto | |
| updated_at | TIMESTAMPTZ | auto-trigger | |
| last_login_at | TIMESTAMPTZ | nullable | |

### 6.2 API Keys

| Column | Type | Description |
|---|---|---|
| id | UUID | PK |
| user_id | UUID | FK → users |
| key_hash | VARCHAR(255) | SHA-256 of actual key |
| key_prefix | VARCHAR(20) | First 14 chars for display |
| name | VARCHAR(100) | User-defined name |
| permissions | JSONB | Default: `["proxy"]` |
| ip_whitelist | JSONB | Array of IPs/CIDRs (max 50) |
| is_active | BOOLEAN | Toggleable |
| last_used_at | TIMESTAMPTZ | |
| expires_at | TIMESTAMPTZ | Optional expiry |
| created_at | TIMESTAMPTZ | |

**Key format:** `iploop_` + 48 hex chars (24 random bytes)

### 6.3 Plans

| Column | Type | Description |
|---|---|---|
| id | UUID or VARCHAR | PK |
| name | VARCHAR(100) | Starter / Growth / Business / Pay-as-you-go |
| description | TEXT | |
| price_per_gb | DECIMAL(10,4) | Per GB rate |
| price_monthly | INTEGER | Monthly price in cents |
| price_annual | INTEGER | Annual price in cents |
| included_gb | INTEGER | Monthly GB allowance |
| requests_per_day | INTEGER | Daily request limit |
| max_concurrent_connections | INTEGER | Connection limit |
| features | JSONB | Feature list |
| stripe_price_id | VARCHAR(255) | Stripe monthly price ID 🔜 |
| is_active | BOOLEAN | |

### 6.4 User Plans (Subscriptions)

| Column | Type | Description |
|---|---|---|
| id | UUID | PK |
| user_id | UUID | FK → users |
| plan_id | UUID | FK → plans |
| gb_balance | DECIMAL(15,6) | Remaining GB |
| gb_used | DECIMAL(15,6) | Used GB |
| status | VARCHAR(20) | active / suspended / cancelled |
| started_at | TIMESTAMPTZ | |
| expires_at | TIMESTAMPTZ | |

### 6.5 Nodes (SDK Devices)

| Column | Type | Description |
|---|---|---|
| id | UUID | PK |
| device_id | VARCHAR(255) | UNIQUE hardware ID |
| ip_address | INET | Current IP |
| country | VARCHAR(2) | ISO country code |
| country_name | VARCHAR(100) | Full name |
| city | VARCHAR(100) | |
| region | VARCHAR(100) | |
| latitude | DECIMAL(10,8) | |
| longitude | DECIMAL(11,8) | |
| asn | INTEGER | Autonomous System Number |
| isp | VARCHAR(255) | Internet Service Provider |
| carrier | VARCHAR(255) | Mobile carrier |
| connection_type | VARCHAR(20) | wifi / cellular / ethernet |
| device_type | VARCHAR(20) | android / ios / windows / mac / browser |
| sdk_version | VARCHAR(20) | |
| status | VARCHAR(20) | available / busy / inactive / banned |
| quality_score | INTEGER 0-100 | |
| bandwidth_used_mb | BIGINT | |
| total_requests | BIGINT | |
| successful_requests | BIGINT | |
| last_heartbeat | TIMESTAMPTZ | |
| connected_since | TIMESTAMPTZ | |
| partner_id | UUID | FK → partners (nullable) |

### 6.6 Usage Records

| Column | Type | Description |
|---|---|---|
| id | UUID | PK |
| user_id | UUID | FK → users |
| api_key_id | UUID | FK → api_keys |
| node_id | UUID | FK → nodes |
| session_id | VARCHAR(255) | |
| bytes_downloaded | BIGINT | |
| bytes_uploaded | BIGINT | |
| total_bytes | BIGINT | Generated: down + up |
| request_count | INTEGER | |
| target_country | VARCHAR(2) | |
| target_city | VARCHAR(100) | |
| proxy_type | VARCHAR(10) | http / socks5 |
| success | BOOLEAN | |
| error_message | TEXT | |
| started_at | TIMESTAMPTZ | |
| ended_at | TIMESTAMPTZ | |
| duration_ms | INTEGER | Generated |

### 6.7 Node Sessions (Sticky IPs)

| Column | Type | Description |
|---|---|---|
| id | UUID | PK |
| session_key | VARCHAR(255) | UNIQUE |
| user_id | UUID | FK |
| node_id | UUID | FK |
| target_country | VARCHAR(2) | |
| target_city | VARCHAR(100) | |
| expires_at | TIMESTAMPTZ | |
| last_used_at | TIMESTAMPTZ | |

### 6.8 Billing Transactions

| Column | Type | Description |
|---|---|---|
| id | UUID | PK |
| user_id | UUID | FK |
| type | VARCHAR(20) | purchase / usage / refund / bonus |
| amount | DECIMAL(15,6) | USD |
| gb_amount | DECIMAL(15,6) | GB |
| description | TEXT | |
| stripe_payment_id | VARCHAR(255) | |
| status | VARCHAR(20) | pending / completed / failed / refunded |
| metadata | JSONB | |

### 6.9 Partners

| Column | Type | Description |
|---|---|---|
| id | UUID | PK |
| name | VARCHAR(100) | Partner company name |
| email | VARCHAR(255) | Contact email |
| api_key_hash | VARCHAR(255) | SHA-256 of partner key |
| api_key_prefix | VARCHAR(20) | Display prefix |
| is_active | BOOLEAN | |
| revenue_share | DECIMAL(5,2) | Percentage (default 70%) |
| total_nodes | INTEGER | Total registered nodes |
| total_earnings | DECIMAL(15,6) | Cumulative earnings |

**Partner key format:** `iplp_` + 48 hex chars

### 6.10 Webhooks

| Column | Type | Description |
|---|---|---|
| id | VARCHAR(36) | PK |
| user_id | VARCHAR(36) | FK → users |
| url | VARCHAR(500) | Endpoint URL |
| secret | VARCHAR(64) | HMAC signing secret |
| secret_preview | VARCHAR(20) | Masked preview |
| events | JSONB | Array of event types |
| is_active | BOOLEAN | |
| failure_count | INTEGER | Consecutive failures |
| last_triggered_at | TIMESTAMPTZ | |

### 6.11 Password Reset Tokens

| Column | Type | Description |
|---|---|---|
| id | UUID | PK |
| user_id | UUID | FK |
| token_hash | VARCHAR(255) | SHA-256 of reset token |
| expires_at | TIMESTAMPTZ | 1 hour from creation |
| used_at | TIMESTAMPTZ | Marks token as consumed |

---

## 7. API Endpoints Reference

### 7.1 Authentication (`/api/auth`)

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/register` | No | Create account |
| POST | `/login` | No | Get JWT token |
| GET | `/profile` | JWT | Get user profile with plan info |
| POST | `/api-keys` | JWT | Generate API key |
| GET | `/api-keys` | JWT | List API keys |
| DELETE | `/api-keys/:keyId` | JWT | Revoke API key |
| PUT | `/password` | JWT | Change password |
| POST | `/logout` | JWT | Blacklist token |
| POST | `/forgot-password` | No | Request reset email |
| POST | `/reset-password` | No | Set new password with token |
| GET | `/verify-reset-token` | No | Check if token is valid |
| POST | `/resend-verification` | JWT | Resend email verification |
| GET | `/verify-email` | No | Verify email with token |

### 7.2 Proxy Management (`/api/proxy`)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/keys` | JWT | List proxy API keys |
| POST | `/keys` | JWT | Create proxy API key |
| GET | `/keys/:keyId` | JWT | Get key with whitelist |
| DELETE | `/keys/:keyId` | JWT | Delete key |
| PATCH | `/keys/:keyId` | JWT | Toggle active status |
| PUT | `/keys/:keyId/whitelist` | JWT | Update IP whitelist |
| GET | `/endpoint` | JWT | Get proxy connection details |
| GET | `/config` | JWT | Get proxy config |
| POST | `/config` | JWT | Update proxy config |
| POST | `/test` | JWT | Test proxy connection |
| GET | `/locations` | No | Available countries & cities |
| GET | `/stats` | JWT | 30-day usage statistics |

### 7.3 Usage (`/api/usage`)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/summary?days=N` | JWT | Usage summary for period |
| GET | `/daily?days=N` | JWT | Daily breakdown |
| GET | `/by-country?days=N` | JWT | Country breakdown |
| GET | `/by-key?days=N` | JWT | Per-API-key breakdown |
| GET | `/recent?limit=N` | JWT | Recent requests (max 100) |

### 7.4 Nodes (`/api/nodes`)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/` | JWT | List available nodes |
| GET | `/stats` | JWT | Node statistics summary |
| GET | `/countries` | JWT | Available countries list |

### 7.5 Webhooks (`/api/webhooks`)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/` | JWT | List webhooks |
| POST | `/` | JWT | Create webhook |
| PUT | `/:webhookId` | JWT | Update webhook |
| DELETE | `/:webhookId` | JWT | Delete webhook |
| POST | `/:webhookId/test` | JWT | Send test event |
| POST | `/:webhookId/regenerate-secret` | JWT | New signing secret |

### 7.6 Admin (`/api/admin`) — Requires admin role

| Method | Path | Description |
|---|---|---|
| GET | `/users?page=N&limit=N&status=X&search=X` | List all users (paginated) |
| POST | `/users/create` | Create user |
| GET | `/users/:userId` | Get user details |
| PUT | `/users/:userId` | Update user (status, role, balance, plan) |
| DELETE | `/users/:userId` | Delete user |
| POST | `/users/:userId/make-admin` | Promote to admin |
| POST | `/users/:userId/remove-admin` | Demote from admin |
| GET | `/plans` | List all plans |
| POST | `/plans` | Create plan |
| PUT | `/plans/:planId` | Update plan |
| GET | `/stats` | System-wide statistics |

### 7.7 Partners (`/api/partners`) — Requires admin role

| Method | Path | Description |
|---|---|---|
| GET | `/` | List all partners |
| POST | `/` | Create partner (generates API key) |
| GET | `/:partnerId` | Partner details + nodes |
| PATCH | `/:partnerId` | Update partner |
| POST | `/:partnerId/regenerate-key` | New partner API key |
| DELETE | `/:partnerId` | Delete partner (no active nodes) |

---

## 8. SDK Management

### 8.1 Android SDK (Production) ✅

| Property | Value |
|---|---|
| Language | Pure Java |
| Min SDK | 22 (Android 5.1+) |
| Latest Version | v1.0.57 |
| Package | `com.iploop.sdk` |
| Main Class | `IPLoopSDK` |
| JAR Size | ~50KB |
| Connection | WebSocket (WSS) to gateway |

### 8.2 SDK Features

| Feature | Status | Description |
|---|---|---|
| WebSocket connection | ✅ | Persistent WSS connection to gateway |
| Auto-reconnect | ✅ | Never gives up — 10min intervals after backoff |
| HTTP proxy tunneling | ✅ | CONNECT method support |
| HTTPS tunneling | ✅ | Binary tunnel protocol |
| Bandwidth tracking | ✅ | Reports bytes shared |
| Heartbeat | ✅ | 5-minute interval |
| Geographic targeting | ✅ | Country, city, ASN |
| Session management | ✅ | Sticky, rotating, per-request |
| Browser profiles | ✅ | Chrome, Firefox, Safari, Mobile presets |
| Performance controls | ✅ | Speed/latency requirements |
| Debug mode | ✅ | Verbose logging |
| iOS SDK | 🔜 | Planned |
| macOS SDK | 🔜 | Planned |
| Windows SDK | 🔜 | Planned |
| Python SDK | 🔜 | Planned |
| Node.js SDK | 🔜 | Planned |

### 8.3 SDK Integration Flow

```
Partner App → IPLoopSDK.init(context, partnerApiKey)
            → WSS connection to gateway.iploop.io/ws
            → Device registration (sends device info, geo, etc.)
            → Heartbeat loop (5 min)
            → Proxy request handling (binary tunnel)
            → Bandwidth reporting
```

---

## 9. Node & Device Management

### 9.1 Node Lifecycle

```
[SDK Init] → [WebSocket Connect] → [Registration] → [Available]
                                                         │
                                           ┌─────────────┼─────────────┐
                                           ▼             ▼             ▼
                                        [Busy]       [Inactive]    [Banned]
                                    (handling req)  (no heartbeat)  (abuse)
                                           │             │
                                           ▼             ▼
                                      [Available]   [Removed]
```

### 9.2 Node Health Metrics

| Metric | Source | Threshold |
|---|---|---|
| Quality Score | 0-100, computed | < 50 = poor |
| Last Heartbeat | WSS heartbeat | > 5 min = inactive |
| Success Rate | successful/total requests | |
| Bandwidth | Cumulative MB | |
| Connection Type | wifi/cellular/ethernet | |

### 9.3 Geographic Distribution

The node registration service uses GeoIP to determine:
- Country (ISO 2-letter code)
- City
- Region
- ASN / ISP
- Carrier (mobile)
- Latitude / Longitude

This data is stored per node and used for geographic proxy targeting.

---

## 10. Proxy Configuration

### 10.1 Proxy Authentication Format

```
http://CUSTOMER_ID:API_KEY[-params]@proxy.iploop.io:7777
socks5://CUSTOMER_ID:API_KEY[-params]@proxy.iploop.io:1080
```

### 10.2 Parameter Syntax

Parameters are appended to the API key with `-` separators:

```
API_KEY-country-US-city-miami-session-abc123-sesstype-sticky-lifetime-30
```

### 10.3 Session Types

| Type | Description |
|---|---|
| `rotating` (default) | New IP for each request |
| `sticky` | Same IP for session duration |
| `per-request` | Explicitly different IP each time |

### 10.4 Session Lifetime

| Format | Example | Description |
|---|---|---|
| Minutes | `lifetime-30` | 30 minutes |
| Hours | `lifetime-1h` | 1 hour |
| Seconds | `lifetime-120s` | 120 seconds |

### 10.5 Rotation Modes

| Mode | Description |
|---|---|
| `request` | Rotate on each new request |
| `time` | Rotate after time interval |
| `manual` | Only rotate on explicit request |
| `ip-change` | Rotate when exit IP changes |

---

## 11. Customer Management & Billing

### 11.1 Customer Lifecycle

```
[Register] → [Free Starter Plan, 5GB] → [Use Proxy] → [Upgrade Plan] → [Ongoing Usage]
                                                             │
                                                    [Stripe Checkout] 🔜
```

### 11.2 Pricing Plans

| Plan | Monthly | Annual | GB/mo | Req/day | Connections | Key Features |
|---|---|---|---|---|---|---|
| Starter | $49 | $470 | 5 | 10,000 | 10 | Basic geo, email support |
| Growth | $149 | $1,430 | 25 | 50,000 | 50 | City targeting, priority support |
| Business | $499 | $4,790 | 100 | 200,000 | 200 | ASN targeting, sticky sessions, 24/7 support, SLA |
| Pay-as-you-go | $0 | — | ∞ | 50,000 | 25 | $5/GB, no commitment |

### 11.3 Stripe Integration 🔜

- **Publishable Key:** Configured (test mode)
- **Secret Key:** Configured (test mode)
- **Services:**
  - `services/billing/` — Go service handling Stripe webhooks
  - `services/customer-api/src/services/stripe.js` — Stripe client wrapper
- **Webhook events handled:** `customer.subscription.created`, `customer.subscription.updated`, `customer.subscription.deleted`, `invoice.paid`, `invoice.payment_failed`

### 11.4 Email Service 🔜

- **Provider:** Resend API (SMTP blocked on DigitalOcean)
- **Templates ready:**
  - Welcome email
  - Password reset
  - Email verification
  - Usage alerts
  - Payment confirmations

---

## 12. Analytics & Reporting

### 12.1 Available Analytics

| Metric | API | Status |
|---|---|---|
| Total requests | `/api/usage/summary` | ✅ |
| Success rate | `/api/usage/summary` | ✅ |
| Total bandwidth (GB) | `/api/usage/summary` | ✅ |
| Average response time | `/api/usage/summary` | ✅ |
| Daily breakdown | `/api/usage/daily` | ✅ |
| Country breakdown | `/api/usage/by-country` | ✅ |
| Per-API-key breakdown | `/api/usage/by-key` | ✅ |
| Recent requests log | `/api/usage/recent` | ✅ |
| Hourly distribution | — | 🔜 |
| Status code distribution | — | 🔜 |
| Real-time dashboard | — | 🔜 |

### 12.2 Admin Analytics

| Metric | API | Status |
|---|---|---|
| Total users | `/api/admin/stats` | ✅ |
| Active users | `/api/admin/stats` | ✅ |
| Admin count | `/api/admin/stats` | ✅ |
| Total platform requests | `/api/admin/stats` | ✅ |
| Total bandwidth | `/api/admin/stats` | ✅ |
| Node count by country | `/api/nodes/stats` | ✅ |
| Node count by device type | `/api/nodes/stats` | ✅ |
| Revenue tracking | — | 🔜 |

---

## 13. Admin Panel

### 13.1 Admin Dashboard (`/admin`) ✅

**Tabs:** Users | Plans | Supply (Nodes) | Settings

#### Users Tab

| Column | Description |
|---|---|
| Email | User email |
| Name | First + Last |
| Company | Company name |
| Status | active/suspended badge |
| Role | customer/admin badge |
| Plan | Plan name |
| GB Balance | Available GB |
| GB Used | Consumed GB |
| Created | Date |
| Last Login | Date |
| Actions | Edit, Suspend, Delete, Make Admin |

**Features:**
- Search by email/name
- Status filter
- Create new user dialog
- Pagination

#### Plans Tab

| Column | Description |
|---|---|
| Name | Plan name |
| Price/GB | Dollar amount |
| Monthly GB | Included bandwidth |
| Connections | Max concurrent |
| Status | Active/Inactive |
| Actions | Edit, Toggle active |

**Create Plan Dialog:** Name, description, pricePerGb, includedGb, maxConnections

#### Supply (Nodes) Tab

Embedded node monitoring with:
- Stats cards: Total, Active, Inactive nodes
- Country breakdown
- Device type breakdown
- Connection type breakdown
- Node list table with all fields from 6.5

#### Settings Tab 🔜

Platform-wide configuration (planned).

### 13.2 Admin Users Page (`/admin/users`) ✅

Full-featured user management table with:
- Search
- Bulk actions (🔜)
- User detail view
- Create user dialog
- Suspend/activate/delete
- Make/remove admin
- Adjust balance
- Change plan

### 13.3 Admin Nodes Page (`/admin/nodes`) ✅

| Feature | Status |
|---|---|
| Node list table | ✅ |
| Search by IP/device | ✅ |
| Country filter | ✅ |
| Status filter | ✅ |
| Auto-refresh (10s) | ✅ |
| Toggle auto-refresh | ✅ |
| Stats cards (total, available, busy, offline) | ✅ |
| Country breakdown | ✅ |
| Connection type breakdown | ✅ |
| Ban/blacklist node | 🔜 |
| Node detail view | 🔜 |

---

## 14. Webhooks & Notifications

### 14.1 Webhook Delivery

**Signature format:** HMAC-SHA256 of JSON body using webhook secret

**Headers sent:**
```
Content-Type: application/json
X-IPLoop-Signature: <hex-digest>
X-IPLoop-Event: <event-type>
```

**Failure handling:**
- Retry up to 3 times 🔜
- Track failure_count
- Auto-disable after 10 consecutive failures 🔜

### 14.2 Email Notifications 🔜

| Template | Trigger |
|---|---|
| Welcome | On registration |
| Password Reset | On forgot-password |
| Email Verification | On registration |
| Quota Warning | Usage > 80% |
| Payment Success | Invoice paid |
| Payment Failed | Invoice failed |
| Weekly Report | Scheduled |

### 14.3 Notification Preferences

Stored per-user in `notification_preferences` table. Configurable in Settings page.

---

## 15. Credit / Earnings System 🔜

### 15.1 Overview

A planned system where SDK users (supply side) earn credits for sharing bandwidth, redeemable for free VPN access or proxy credits.

### 15.2 Credit Rates

| Action | Credits |
|---|---|
| Share 1 GB bandwidth | 100 credits |
| 1 proxy request cost | 1 credit |

### 15.3 Multipliers

| Bonus | Condition | Multiplier |
|---|---|---|
| Multi-device | 3+ active devices | 2x |
| 24h Uptime | Continuous online | 1.5x |
| Rare Geo | Traffic from rare countries | Up to 3x |

Multipliers stack: max `2.0 × 1.5 × 3.0 = 9x`

### 15.4 VPN Access

Any user with at least 1 active device gets free VPN access.

### 15.5 Earn Landing Pages ✅

Static pages exist at:
- `/earn-landing/index.html` — Landing page
- `/earn-landing/signup.html` — Sign up
- `/earn-landing/dashboard.html` — Earn dashboard
- `/earn-landing/terms.html` — Terms
- `/earn-landing/privacy.html` — Privacy

---

## 16. Error States & Edge Cases

### 16.1 Authentication Errors

| Scenario | Response | UI Treatment |
|---|---|---|
| Invalid credentials | 401 | Red alert: "Invalid email or password" |
| Account suspended | 401 | Red alert: "Account is not active" |
| Duplicate email | 409 | Red alert: "User already exists" |
| Weak password | 400 | Red alert: "Password must be at least 8 characters" |
| Token expired | 401 | Redirect to login |
| Token blacklisted | 401 | Redirect to login |

### 16.2 API Key Errors

| Scenario | Response | UI Treatment |
|---|---|---|
| Max keys reached (5) | 400 | Red alert: "Maximum number of API keys reached" |
| Key not found | 404 | Error toast |
| Name too short | 400 | Inline validation |

### 16.3 Proxy Errors

| Scenario | Description |
|---|---|
| No available nodes | 503: No nodes in requested country |
| Node timeout | 504: Device didn't respond |
| Invalid targeting | 400: Invalid country code |
| Rate limit exceeded | 429: Too many requests |
| Balance depleted | 402: Insufficient balance |

### 16.4 Network Errors

| Scenario | UI Treatment |
|---|---|
| API unreachable | "Failed to connect to server" error |
| WebSocket disconnect | Auto-reconnect with exponential backoff |
| Slow response | Loading spinners on all data fetches |
| Empty state | Meaningful empty messages (e.g., "No active nodes") |

### 16.5 Loading States

Every page should show:
- Skeleton loaders or spinner during initial data fetch
- "Loading..." text as minimum
- Disabled buttons during form submission
- Optimistic UI updates where possible

---

## 17. Responsive Design Requirements

### 17.1 Breakpoints

| Breakpoint | Width | Layout |
|---|---|---|
| Mobile | < 768px | Single column, hamburger menu, collapsed sidebar |
| Tablet | 768px - 1024px | Two columns where appropriate |
| Desktop | > 1024px | Full layout with persistent sidebar |

### 17.2 Sidebar Behavior

- **Desktop:** Always visible, fixed left (w-64)
- **Mobile:** Hidden by default, slide-in overlay on hamburger click
- **Transition:** Smooth slide animation

### 17.3 Component Responsiveness

| Component | Mobile | Desktop |
|---|---|---|
| Stats grid | 1 column (stacked) | 2-4 columns |
| Data tables | Horizontal scroll | Full width |
| Charts | Full width, smaller height | Side-by-side possible |
| Forms | Full width inputs | Max-width constrained |
| Modals/Dialogs | Full-screen on mobile | Centered overlay |
| Code blocks | Horizontal scroll | Full width |
| Node list | Card view | Table view |
| World map | Hidden or simplified | Full interactive |

### 17.4 Theme

- **Current:** Dark theme (zinc/gray palette)
- **Requirement:** Support both dark and light themes
- **Primary color:** Blue (#3b82f6)
- **Success:** Green (#10b981)
- **Warning:** Yellow (#f59e0b)
- **Error:** Red (#ef4444)
- **Font:** System font stack (Inter recommended)

---

## 18. Feature Status Matrix

### ✅ Implemented & Working

| Feature | Location |
|---|---|
| User registration & login | Auth routes + Login page |
| JWT authentication | Auth middleware |
| Password reset flow | Auth routes + pages |
| Email verification | Auth routes + page |
| Dashboard overview with stats | `/dashboard` |
| Real-time node list | `/nodes` |
| World map visualization | `/dashboard` WorldMap component |
| API key CRUD | `/api-keys` + proxy routes |
| IP whitelist per API key | Proxy routes |
| Proxy endpoint documentation | `/endpoints` |
| Usage analytics with charts | `/analytics` |
| Usage breakdown (daily, country, key) | `/usage` |
| Webhook management | `/webhooks` |
| API documentation page | `/docs` |
| Settings page (profile, password, notifications) | `/settings` |
| AI support chat | `/support` |
| Admin user management | `/admin` + `/admin/users` |
| Admin plan management | `/admin` |
| Admin node monitoring | `/admin/nodes` |
| Partner management (CRUD) | Partners API routes |
| SDK (Android Java v1.0.57) | Production |
| Proxy gateway (HTTP + SOCKS5) | Production |
| Node registration (WebSocket) | Production |
| Geographic targeting (country, city, ASN) | Proxy gateway |
| Session management (sticky, rotating) | Proxy gateway |
| Browser profiles | SDK + proxy gateway |
| Auto-scaling service | Autoscaler |
| Binary tunnel protocol | v2.0 production |
| Docker Compose orchestration | Full stack |
| Prometheus + Grafana monitoring | Optional profile |
| Finance overview page | `/finance` |

### 🔜 Planned / Not Yet Built

| Feature | Priority | Notes |
|---|---|---|
| Stripe payment integration | HIGH | Test keys configured, service scaffolded |
| Email notifications (Resend) | HIGH | Service built, templates ready |
| Real-time usage dashboard | MEDIUM | WebSocket-based live updates |
| iOS SDK | MEDIUM | README placeholder exists |
| Windows SDK | MEDIUM | README placeholder exists |
| macOS SDK | LOW | README placeholder exists |
| Python SDK | LOW | README placeholder exists |
| Node.js SDK | LOW | README placeholder exists |
| Credit/earnings system | MEDIUM | Schema designed, not deployed |
| DSP/SSP portals | LOW | Login pages exist, no backend |
| Two-factor authentication | MEDIUM | Settings UI placeholder |
| Webhook delivery retry | HIGH | Logic designed, not implemented |
| Auto-disable failing webhooks | MEDIUM | |
| Bulk user operations (admin) | LOW | |
| Export usage data (CSV) | MEDIUM | |
| Rate limiting per API key | HIGH | Limiter module exists in Go |
| Revenue tracking dashboard | MEDIUM | |
| Partner earnings dashboard | MEDIUM | |
| Node detail view (admin) | LOW | |
| Light theme | MEDIUM | |
| Internationalization (i18n) | LOW | |
| Mobile app (React Native) | LOW | Scaffold exists |

---

## 19. Glossary

| Term | Definition |
|---|---|
| **Node** | A device running the IPLoop SDK that shares its bandwidth |
| **SDK Partner** | A company that integrates the IPLoop SDK into their app |
| **Proxy Customer** | A user who buys proxy bandwidth for web scraping etc. |
| **Sticky Session** | A proxy session that maintains the same exit IP |
| **Rotating Proxy** | A new exit IP for each request |
| **CONNECT Tunnel** | HTTP CONNECT method used for HTTPS proxying |
| **Quality Score** | 0-100 rating of a node's reliability and speed |
| **GeoIP** | Technology to determine geographic location from IP address |
| **ASN** | Autonomous System Number — identifies an ISP/network |
| **WSS** | WebSocket Secure — encrypted WebSocket connection |
| **JWT** | JSON Web Token — authentication token format |
| **Revenue Share** | Percentage of proxy revenue paid to SDK partners |
| **SOCKS5** | Socket Secure protocol for proxy connections |
| **Heartbeat** | Periodic ping from SDK to server to confirm device is alive |

---

*End of PRD. This document covers the complete IPLoop platform as of February 2026. A UX/UI designer should be able to redesign every screen, flow, and interaction from this specification alone.*
