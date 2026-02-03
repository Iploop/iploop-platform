# IPLoop Proxy Platform MVP

A complete proxy platform with SDK-based node network and customer management.

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                    IPLoop Platform                     │
│                                                        │
│  ┌─────────────┐    ┌──────────────┐    ┌───────────┐ │
│  │  Supply SDK  │───▶│ Proxy Engine │◀───│  Customer  │ │
│  │  (Devices)   │    │  (Backend)   │    │ Dashboard  │ │
│  └─────────────┘    └──────────────┘    └───────────┘ │
└──────────────────────────────────────────────────────┘
```

## Components

### 1. Proxy Gateway (Go)
- HTTP/SOCKS5 proxy server
- Customer authentication (API key based)
- Node pool management
- Country/geo targeting
- Bandwidth tracking
- Format: `customer_id:api_key@proxy.iploop.com:7777`

### 2. Node Registration Service (Go)
- WebSocket server for device connections
- Device registration and heartbeat monitoring
- Node health scoring
- Redis for real-time node pool state

### 3. Customer API (Node.js/Express)
- REST API for customer management
- JWT authentication
- API key generation
- Usage tracking and analytics
- Basic billing system

### 4. Dashboard (React/Next.js)
- Customer web interface
- Login/signup pages
- Usage dashboard
- API key management
- Node availability map

### 5. Docker Compose
- Full stack deployment
- All services + Redis + PostgreSQL
- One-command startup

## Quick Start

```bash
# Clone and setup
git clone <this-repo>
cd iploop-platform

# Start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| **Proxy Gateway** | 7777 | HTTP proxy endpoint |
| **Proxy Gateway** | 1080 | SOCKS5 proxy endpoint |
| **Node Registration** | 8001 | WebSocket for SDK nodes |
| **Customer API** | 8002 | REST API |
| **Dashboard** | 3000 | Web interface |
| **Redis** | 6379 | Node pool cache |
| **PostgreSQL** | 5432 | Main database |

## Customer Usage

```bash
# HTTP Proxy
curl -x http://customer1:key123@localhost:7777 http://httpbin.org/ip

# SOCKS5 Proxy
curl --socks5 customer1:key123@localhost:1080 http://httpbin.org/ip

# With targeting
curl -x http://customer1:key123-country-us@localhost:7777 http://httpbin.org/ip
```

## API Endpoints

### Authentication
- `POST /api/v1/auth/register` - User registration
- `POST /api/v1/auth/login` - User login

### Proxy Management
- `GET /api/v1/proxy/endpoint` - Get proxy endpoint
- `POST /api/v1/proxy/config` - Update targeting config

### Usage & Billing
- `GET /api/v1/usage` - Current usage stats
- `GET /api/v1/usage/history` - Usage history
- `GET /api/v1/billing/balance` - Account balance

### Network Status
- `GET /api/v1/network/status` - Overall network status
- `GET /api/v1/network/countries` - Available countries

## Development

Each service has its own README with detailed setup instructions:

- [`services/proxy-gateway/README.md`](services/proxy-gateway/README.md)
- [`services/node-registration/README.md`](services/node-registration/README.md)
- [`services/customer-api/README.md`](services/customer-api/README.md)
- [`services/dashboard/README.md`](services/dashboard/README.md)

## Environment Variables

Copy `.env.example` to `.env` and configure:

```env
# Database
POSTGRES_DB=iploop
POSTGRES_USER=iploop
POSTGRES_PASSWORD=securepassword123

# Redis
REDIS_URL=redis://localhost:6379

# JWT
JWT_SECRET=your-jwt-secret-key

# Services
PROXY_GATEWAY_HOST=0.0.0.0
PROXY_GATEWAY_HTTP_PORT=7777
PROXY_GATEWAY_SOCKS_PORT=1080
NODE_REGISTRATION_PORT=8001
CUSTOMER_API_PORT=8002
DASHBOARD_PORT=3000
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.