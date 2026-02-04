# IPLoop Proxy Platform

Residential proxy platform with SDK-based node network and customer management.

## Live Endpoints

| Service | URL |
|---------|-----|
| **Dashboard** | https://gateway.iploop.io |
| **WebSocket (Nodes)** | wss://gateway.iploop.io/ws |
| **Customer API** | https://gateway.iploop.io/api |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       IPLoop Platform                            │
│                                                                  │
│  ┌─────────────┐                          ┌──────────────────┐  │
│  │  Android/   │◀── WSS ──▶ Cloudflare ──▶│ Node Registration │  │
│  │  iOS SDK    │           Tunnel         │    (port 8001)    │  │
│  └─────────────┘           │              └──────────────────┘  │
│                            │                                     │
│  ┌─────────────┐           │              ┌──────────────────┐  │
│  │  Customer   │◀── HTTPS ─┤              │  Customer API    │  │
│  │  Dashboard  │           │              │   (port 8002)    │  │
│  └─────────────┘           │              └──────────────────┘  │
│                            │                                     │
│  ┌─────────────┐           │              ┌──────────────────┐  │
│  │   Proxy     │◀── HTTP/  └─────────────▶│  Proxy Gateway   │  │
│  │  Customers  │   SOCKS5                 │  (7777/1080)     │  │
│  └─────────────┘                          └──────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Proxy Gateway (Go)
- HTTP proxy (port 7777)
- SOCKS5 proxy (port 1080)
- Customer authentication via API key
- Geo-targeting by country
- Bandwidth tracking

### 2. Node Registration Service (Go)
- WebSocket endpoint: `/ws`
- Device registration & heartbeat
- Node health scoring
- Redis for real-time state

### 3. Customer API (Node.js)
- REST API for customer management
- JWT authentication
- API key generation
- Usage tracking

### 4. Dashboard (Next.js)
- Customer web interface
- Real-time node monitoring
- API key management
- Usage analytics

### 5. Android SDK
- **Version:** 1.0.2
- **WebSocket:** `wss://gateway.iploop.io/ws`
- **Min SDK:** 21 (Android 5.0)

## Quick Start

```bash
# Start all services
cd iploop-platform
docker compose up -d

# Check status
docker compose ps
```

## SDK Integration

### Android

```kotlin
// Initialize
IPLoopSDK.init(context, "your-sdk-key", IPLoopConfig.createDefault())

// Start
IPLoopSDK.start()

// Stop
IPLoopSDK.stop()
```

### Required Permissions
```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
```

### Optional Permissions
```xml
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
<uses-permission android:name="android.permission.ACCESS_WIFI_STATE" />
```

## Proxy Usage

```bash
# HTTP Proxy
curl -x http://CUSTOMER_ID:API_KEY@gateway.iploop.io:7777 http://httpbin.org/ip

# SOCKS5 Proxy
curl --socks5 CUSTOMER_ID:API_KEY@gateway.iploop.io:1080 http://httpbin.org/ip

# With country targeting
curl -x http://CUSTOMER_ID:API_KEY-country-us@gateway.iploop.io:7777 http://httpbin.org/ip
```

## API Endpoints

### Authentication
- `POST /api/auth/register` - Register
- `POST /api/auth/login` - Login

### API Keys
- `GET /api/keys` - List keys
- `POST /api/keys` - Create key
- `DELETE /api/keys/:id` - Delete key

### Usage
- `GET /api/usage` - Current usage
- `GET /api/usage/history` - History

### Network
- `GET /api/network/status` - Network status
- `GET /api/network/countries` - Available countries

## Services (Docker)

| Container | Port | Health |
|-----------|------|--------|
| iploop-proxy-gateway | 7777, 1080 | HTTP/SOCKS |
| iploop-node-registration | 8001 | WebSocket |
| iploop-customer-api | 8002 | REST API |
| iploop-dashboard | 3000 | Web UI |
| iploop-postgres | 5432 | Database |
| iploop-redis | 6379 | Cache |

## Cloudflare Tunnel

The platform uses a named Cloudflare tunnel for secure public access:

- **Tunnel name:** `iploop-gateway`
- **Domain:** `gateway.iploop.io`
- **Config:** `/etc/cloudflared/config.yml`
- **Service:** `systemctl status cloudflared`

## Environment Variables

See `.env.example` for all configuration options.

## License

MIT
