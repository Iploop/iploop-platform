# IPLoop Infrastructure Scaling Plan

> Last updated: 2026-02-08  
> Status: **Draft — ready for team review**

---

## Table of Contents

1. [Current State Assessment](#current-state-assessment)
2. [Capacity Analysis & Bottlenecks](#capacity-analysis--bottlenecks)
3. [Phase 1: Dedicated IPLoop Server (Immediate → 5K nodes)](#phase-1-dedicated-iploop-server)
4. [Phase 2: Service Separation (5K → 10K nodes)](#phase-2-service-separation)
5. [Phase 3: Multi-Region (10K → 50K nodes)](#phase-3-multi-region)
6. [Phase 4: Full Scale (50K → 100K+ nodes)](#phase-4-full-scale)
7. [Bandwidth & Proxy Traffic Modeling](#bandwidth--proxy-traffic-modeling)
8. [Cost Summary](#cost-summary)
9. [Migration Playbook: Phase 1](#migration-playbook-phase-1)
10. [Decision Framework](#decision-framework)

---

## Current State Assessment

### Infrastructure

| Component | Current Setup |
|-----------|--------------|
| **Server** | 1× DigitalOcean Premium AMD Droplet |
| **Specs** | 8 vCPU / 16 GB RAM / 309 GB SSD |
| **Shared With** | Clawdbot (agent, bots, scripts) |
| **Region** | LON1 (London) |
| **OS** | Ubuntu 22.04 |
| **Ingress** | Cloudflare Tunnel (HTTPS/WSS) |
| **Estimated Cost** | ~$96/mo (droplet) |

### Services Running (Docker Compose)

| Service | Language | Role | Resource Profile |
|---------|----------|------|-----------------|
| `node-registration` | Go | WebSocket hub, node auth | **CPU + Memory heavy** (holds all WS connections) |
| `proxy-gateway` | Go | SOCKS5/HTTP proxy routing | **CPU + Network heavy** (proxies all traffic) |
| `customer-api` | Node.js | REST API for customers | Light |
| `dashboard` | Next.js | Web UI | Light |
| `nginx-proxy` | Nginx | Reverse proxy / routing | Light |
| `postgres` | PostgreSQL 15 | Primary database | Moderate (I/O) |
| `redis` | Redis 7 | Cache, session state, node registry | Moderate (Memory) |
| `prometheus` | Prometheus | Metrics collection | Light-Moderate |
| `grafana` | Grafana | Dashboards | Light |
| `autoscaler` | Go | Auto-scaling logic | Light |
| `home-page` | Nginx/static | Marketing page | Negligible |

### Current Resource Usage (Live Snapshot)

```
Memory:  4.3 GB used / 15 GB total (29% — but 11 GB is buff/cache)
Swap:    1.0 GB used / 1.0 GB total (100% — ⚠️ BAD)
Disk:    139 GB used / 309 GB (45%)
CPU:     8 vCPU (shared with Clawdbot)
Nodes:   ~1,500+ connected via WebSocket
```

**⚠️ Red Flags:**
- **Swap is 100% used** — system is memory-constrained
- Clawdbot competes for CPU/memory on the same box
- Single point of failure — one box runs everything
- No database backups (self-hosted Postgres)
- No redundancy on any layer

---

## Capacity Analysis & Bottlenecks

### WebSocket Memory Footprint (Go)

Per WebSocket connection in Go (with gorilla/websocket or similar):

| Component | Per Connection | 1.5K | 5K | 10K | 50K | 100K |
|-----------|---------------|------|-----|------|------|------|
| Goroutine stacks (2× per conn) | ~8 KB | 12 MB | 40 MB | 80 MB | 400 MB | 800 MB |
| Read/write buffers | ~8 KB | 12 MB | 40 MB | 80 MB | 400 MB | 800 MB |
| Connection metadata | ~2 KB | 3 MB | 10 MB | 20 MB | 100 MB | 200 MB |
| **Subtotal** | **~18 KB** | **27 MB** | **90 MB** | **180 MB** | **900 MB** | **1.8 GB** |

> With optimizations (epoll/kqueue, goroutine pools, zero-copy reads), this can be reduced by 5-10×. But the naive gorilla/websocket approach uses ~18-20 KB per connection.

### Key Bottlenecks by Scale

| Scale | Primary Bottleneck | Secondary |
|-------|-------------------|-----------|
| **1.5K** (now) | Shared resources with Clawdbot, swap thrashing | Single region |
| **5K** | Memory for WS connections + proxy buffers | CPU for proxy traffic |
| **10K** | File descriptors, network throughput | Database connections |
| **50K** | Single-server limits, geographic latency | Bandwidth costs |
| **100K+** | Everything — need distributed architecture | Operational complexity |

### Linux Kernel Limits to Address

```bash
# Required tuning for high connection counts
sysctl -w net.core.somaxconn=65535
sysctl -w net.ipv4.tcp_max_syn_backlog=65535
sysctl -w fs.file-max=1000000
sysctl -w net.ipv4.ip_local_port_range="1024 65535"
sysctl -w net.core.rmem_max=16777216
sysctl -w net.core.wmem_max=16777216

# ulimits
ulimit -n 1000000  # file descriptors
```

---

## Phase 1: Dedicated IPLoop Server

**Timeline:** Immediate  
**Target:** 1.5K → 5K nodes  
**Trigger:** Now (swap is thrashing, shared resources)

### Architecture

```
┌─────────────────────────┐     ┌───────────────────────┐
│   EXISTING DROPLET      │     │  NEW IPLOOP DROPLET   │
│   (LON1, 8vCPU/16GB)   │     │  (NYC3, 4vCPU/8GB)   │
│                         │     │                       │
│   • Clawdbot            │     │  • node-registration  │
│   • Silent Reader Bot   │     │  • proxy-gateway      │
│   • Scripts             │     │  • customer-api       │
│   • Monitoring          │     │  • dashboard          │
│                         │     │  • nginx-proxy        │
│                         │     │  • postgres           │
│                         │     │  • redis              │
│                         │     │  • prometheus/grafana  │
└─────────────────────────┘     └───────────────────────┘
                                         ↑
                                  Cloudflare Tunnel
                                  (gateway.iploop.io)
```

### Server Options

#### Option A: DigitalOcean (Recommended for simplicity)

| Plan | vCPU | RAM | SSD | Transfer | Price/mo |
|------|------|-----|-----|----------|----------|
| **Basic 4vCPU/8GB** | 4 (shared) | 8 GB | 160 GB | 5 TB | **$48** |
| **Basic 8vCPU/16GB** ⭐ | 8 (shared) | 16 GB | 320 GB | 6 TB | **$96** |
| CPU-Opt 2vCPU/4GB | 2 (dedicated) | 4 GB | 25 GB | 4 TB | $42 |
| CPU-Opt 4vCPU/8GB | 4 (dedicated) | 8 GB | 50 GB | 5 TB | $84 |
| General 2vCPU/8GB | 2 (dedicated) | 8 GB | 25 GB | 4 TB | $63 |

#### Option B: Hetzner Cloud (Better value, EU/US regions)

| Plan | vCPU | RAM | SSD | Transfer | Price/mo |
|------|------|-----|-----|----------|----------|
| CX33 (shared) | 4 | 8 GB | 80 GB | 20 TB | **€5.49 (~$6)** |
| CX43 (shared) | 8 | 16 GB | 160 GB | 20 TB | **€9.49 (~$10)** |
| **CX53 (shared)** ⭐ | 16 | 32 GB | 320 GB | 20 TB | **€17.49 (~$19)** |
| CCX23 (dedicated) | 4 | 16 GB | 80 GB | 20 TB | **€14.09 (~$15)** |
| CCX33 (dedicated) | 8 | 32 GB | 160 GB | 20 TB | **€24.49 (~$26)** |

> **Hetzner is 5-10× cheaper than DigitalOcean** for equivalent specs. Trade-off: less managed services, US availability is Ashburn/Hillsboro only. EU locations (Nuremberg, Helsinki) are excellent.

#### Option C: Hybrid — Keep DO for Clawdbot, Hetzner for IPLoop

This is the **recommended approach**:

| Server | Provider | Region | Specs | Cost/mo |
|--------|----------|--------|-------|---------|
| Clawdbot | DigitalOcean | LON1 | Current 8vCPU/16GB (or downsize to 4vCPU/8GB) | $48-96 |
| **IPLoop** | **Hetzner** | **ASH (Ashburn, VA)** | **CX53: 16vCPU/32GB** | **~$19** |

**Why Ashburn?** Majority of proxy customers target US IPs. Ashburn is "Data Center Alley" — lowest latency to most US users and peering points.

### Phase 1 Monthly Costs

| Component | Option A (DO only) | Option B (Hetzner only) | **Option C (Hybrid)** ⭐ |
|-----------|-------------------|------------------------|------------------------|
| IPLoop server | $96 | ~$19 | ~$19 |
| Clawdbot server | (shared) | ~$10 | $48 (downsize) |
| Cloudflare Tunnel | Free | Free | Free |
| Domain/DNS | ~$0 | ~$0 | ~$0 |
| **Total** | **$96** | **~$29** | **~$67** |
| vs. current | Same cost, better perf | -70% | -30% |

### Phase 1 Performance Projections

| Metric | Current (shared) | Phase 1 (dedicated IPLoop) |
|--------|-----------------|---------------------------|
| Max WS connections | ~3-5K (memory limited) | **10-15K** (32GB, tuned) |
| Proxy throughput | Bottlenecked by Clawdbot | **Full CPU/network available** |
| Database I/O | Competing with Clawdbot | Dedicated disk I/O |
| Swap usage | 100% (critical) | 0% (target) |
| Recovery time | Manual, everything coupled | Independent restarts |

---

## Phase 2: Service Separation

**Timeline:** When approaching 5-10K nodes  
**Target:** 5K → 10K nodes  
**Trigger:** CPU consistently >70% or memory >80% on Phase 1 server

### Architecture

```
                    Cloudflare Tunnel
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
   ┌──────────┐   ┌──────────┐   ┌──────────┐
   │  WS Hub  │   │  Proxy   │   │  API +   │
   │  Server  │   │  Server  │   │ Dashboard│
   │          │   │          │   │          │
   │ node-reg │   │ proxy-gw │   │ cust-api │
   │          │   │          │   │ dashboard│
   │ Hetzner  │   │ Hetzner  │   │ nginx    │
   │ CX43     │   │ CX43     │   │          │
   │ 8v/16GB  │   │ 8v/16GB  │   │ Hetzner  │
   │ ~$10/mo  │   │ ~$10/mo  │   │ CX33     │
   │          │   │          │   │ 4v/8GB   │
   └─────┬────┘   └─────┬────┘   │ ~$6/mo  │
         │              │        └────┬─────┘
         │              │             │
    ┌────┴──────────────┴─────────────┴────┐
    │         Managed Database Tier         │
    │                                       │
    │  PostgreSQL (Hetzner or DO Managed)   │
    │  Redis/Valkey (Hetzner or DO Managed) │
    └───────────────────────────────────────┘
```

### Why Separate?

1. **WebSocket Hub** is memory-bound (connection state) — needs RAM
2. **Proxy Gateway** is CPU/network-bound (data forwarding) — needs throughput
3. **API/Dashboard** is light — smallest server possible
4. **Database** should be managed — backups, failover, no ops burden

### Server Breakdown

| Service | Server | Specs | Provider | Cost/mo |
|---------|--------|-------|----------|---------|
| WebSocket Hub | CX43 | 8 vCPU / 16 GB | Hetzner ASH | ~$10 |
| Proxy Gateway | CX43 | 8 vCPU / 16 GB | Hetzner ASH | ~$10 |
| API + Dashboard | CX33 | 4 vCPU / 8 GB | Hetzner ASH | ~$6 |
| Clawdbot | Basic 4v/8GB | 4 vCPU / 8 GB | DO LON1 | $48 |

### Database Options

#### Option A: Self-hosted on API server (budget)

- PostgreSQL + Redis on the API/Dashboard server
- **Cost:** $0 additional
- **Risk:** No automatic backups, no failover
- **Mitigation:** Cron-based pg_dump to object storage

#### Option B: DigitalOcean Managed (recommended when revenue supports it)

| Service | Plan | Cost/mo |
|---------|------|---------|
| Managed PostgreSQL | 1 vCPU / 1 GB / 10 GB | $15 |
| Managed Valkey (Redis) | 1 vCPU / 1 GB | $15 |
| **Subtotal** | | **$30** |

#### Option C: Hetzner + manual backups (middle ground)

- Run Postgres/Redis on a small dedicated Hetzner server (CX23, ~$4/mo)
- Automated backups via cron + Hetzner snapshots ($0.011/GB/mo)
- **Cost:** ~$5-8/mo total

### Phase 2 Monthly Costs

| Component | Budget Path | Recommended Path |
|-----------|------------|-----------------|
| WS Hub server | ~$10 | ~$10 |
| Proxy server | ~$10 | ~$10 |
| API + Dashboard | ~$6 | ~$6 |
| Database | $0 (self-hosted) | $30 (managed) |
| Clawdbot | $48 | $48 |
| Monitoring (Prometheus/Grafana) | $0 (on API server) | $0 |
| Cloudflare | Free | Free |
| **Total** | **~$74** | **~$104** |

### Phase 2 Performance Projections

| Metric | Phase 1 | Phase 2 |
|--------|---------|---------|
| Max WS connections | 10-15K | **20-30K** |
| Proxy throughput | Shared CPU | **Dedicated, 2× capacity** |
| Database resilience | None | Managed backups + failover |
| Blast radius | Total outage | Partial (services independent) |

---

## Phase 3: Multi-Region

**Timeline:** When approaching 10-50K nodes  
**Target:** 10K → 50K nodes  
**Trigger:** Geographic latency complaints, single-region capacity limits

### Architecture

```
                      Cloudflare Global LB (DNS-based)
                               │
            ┌──────────────────┼──────────────────┐
            ▼                  ▼                  ▼
    ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
    │  US-EAST      │  │  EU-WEST      │  │  US-WEST      │
    │  (Ashburn)    │  │  (Frankfurt)  │  │  (Hillsboro)  │
    │               │  │               │  │               │
    │  WS Hub       │  │  WS Hub       │  │  WS Hub       │
    │  Proxy GW     │  │  Proxy GW     │  │  Proxy GW     │
    │  API replica  │  │  API replica  │  │  API replica  │
    │               │  │               │  │               │
    │  Hetzner      │  │  Hetzner      │  │  Hetzner      │
    │  CX53         │  │  CX43         │  │  CX43         │
    └───────┬───────┘  └───────┬───────┘  └───────┬───────┘
            │                  │                  │
            └──────────────────┼──────────────────┘
                               │
                    ┌──────────┴──────────┐
                    │  Primary Database   │
                    │  (DO Managed PG)    │
                    │  NYC3 + Read Replica│
                    │  in FRA1            │
                    └─────────────────────┘
                    ┌─────────────────────┐
                    │  Redis per region   │
                    │  (local instance)   │
                    └─────────────────────┘
```

### Region Selection Strategy

Based on typical residential proxy node distribution:

| Region | Expected Node % | Server Location | Provider |
|--------|----------------|-----------------|----------|
| **US East** | 40-50% | Ashburn, VA | Hetzner |
| **US West** | 15-20% | Hillsboro, OR | Hetzner |
| **EU** | 15-20% | Frankfurt/Nuremberg | Hetzner |
| **UK** | 5-10% | London | DigitalOcean LON1 |
| **APAC** | 5-10% | Singapore | Hetzner SGP (future) |

### Inter-Region Communication

- **Node Registration:** Region-local (each hub handles its own nodes)
- **Proxy Routing:** Cross-region when customer requests a specific geo
  - Customer in EU wants US IP → routes to US-East hub → selects US node
  - Latency: ~80-120ms cross-Atlantic (acceptable for proxy use)
- **Database:** 
  - Write to primary (US-East)
  - Read replicas in each region for fast reads
  - Redis is region-local (node state, session cache)

### Cloudflare Integration

Cloudflare handles geo-routing natively:
- DNS-based load balancing (Cloudflare LB or simple geo-DNS)
- Health checks per region
- Automatic failover
- **Cloudflare Load Balancing:** $5/mo + $0.50 per 500K DNS queries

### Phase 3 Monthly Costs

| Component | Specs | Cost/mo |
|-----------|-------|---------|
| **US-East (primary)** | CX53 (16v/32GB) | ~$19 |
| **EU (Frankfurt)** | CX43 (8v/16GB) | ~$10 |
| **US-West** | CX43 (8v/16GB) | ~$10 |
| Managed PostgreSQL (primary) | DO 4GB/2vCPU | $61 |
| Managed PostgreSQL (read replica) | DO 2GB/1vCPU | $30 |
| Redis per region (self-hosted) | Included on app servers | $0 |
| Cloudflare LB | Geo-routing | ~$5 |
| Clawdbot | DO Basic 4v/8GB | $48 |
| Bandwidth overage (est.) | ~2-5 TB extra | $20-50 |
| **Total** | | **~$203-233** |

### Phase 3 Performance Projections

| Metric | Phase 2 | Phase 3 |
|--------|---------|---------|
| Max WS connections | 20-30K | **50-80K** (distributed) |
| Proxy latency (same region) | 50-100ms | **20-50ms** |
| Proxy latency (cross-region) | N/A | 80-150ms |
| Geographic coverage | Single region | US + EU |
| Redundancy | None | Regional failover |

---

## Phase 4: Full Scale

**Timeline:** 50K+ nodes, significant revenue  
**Target:** 50K → 100K+ nodes  
**Trigger:** Revenue justifies infrastructure, enterprise customers demand SLAs

### Architecture Evolution

At this scale, consider:

1. **Kubernetes (DO or Hetzner)** — Auto-scaling, rolling deploys
2. **Dedicated database cluster** — PostgreSQL with Patroni for HA
3. **Message queue** — NATS or Redis Streams for cross-region node routing
4. **CDN for dashboard** — Cloudflare Pages or Vercel
5. **Separated proxy tiers** — Premium (dedicated proxy servers) vs. Standard

### Optimized WebSocket Server

At 100K+ connections, the Go WebSocket server needs optimization:

```go
// Switch from gorilla/websocket to gobwas/ws + netpoll
// Reduces per-connection overhead from ~18KB to ~2-3KB
// 100K connections: 200-300MB instead of 1.8GB

// Use epoll-based event loop instead of goroutine-per-connection
// See: github.com/gobwas/ws + github.com/mailru/easygo/netpoll
```

### Phase 4 Monthly Costs (Estimated)

| Component | Specs | Cost/mo |
|-----------|-------|---------|
| 5× Regional servers | CX53 each | ~$95 |
| 2× Dedicated proxy servers | CCX33 (8v/32GB dedicated) | ~$52 |
| Managed PostgreSQL HA | DO 8GB/4vCPU + standby | $244 |
| Managed Redis/Valkey | DO 4GB/2vCPU × 3 regions | $180 |
| Cloudflare Business | Adv. features + LB | $200+ |
| Monitoring (Grafana Cloud) | Pro tier | $29 |
| Bandwidth (est. 20-50TB) | | $200-500 |
| Clawdbot | DO Basic | $48 |
| **Total** | | **~$1,050-1,350** |

### Phase 4 Performance Projections

| Metric | Phase 3 | Phase 4 |
|--------|---------|---------|
| Max WS connections | 50-80K | **200K+** |
| Proxy throughput | Moderate | **High (dedicated)** |
| Regions | 3 | 5+ |
| SLA target | Best-effort | 99.9% |
| Recovery time | Minutes | Seconds (auto-failover) |

---

## Bandwidth & Proxy Traffic Modeling

### Traffic Per Node

Each node's traffic depends on proxy usage:

| Scenario | Avg Data/Node/Day | Monthly/Node |
|----------|------------------|-------------|
| **Idle** (connected, no proxy traffic) | ~1 MB (heartbeats) | ~30 MB |
| **Light usage** (occasional browsing) | ~50 MB | ~1.5 GB |
| **Moderate usage** (regular proxy) | ~200 MB | ~6 GB |
| **Heavy usage** (scraping, high-volume) | ~1 GB | ~30 GB |

### Bandwidth Cost Projections

| Nodes | Avg Usage | Monthly Outbound | DO Cost ($0.01/GB excess) | Hetzner Cost |
|-------|-----------|-----------------|--------------------------|-------------|
| 5K | Light | ~7.5 TB | ~$25 (over 5TB free) | **$0** (20TB free) |
| 10K | Light | ~15 TB | ~$100 | **$0** (20TB free) |
| 10K | Moderate | ~60 TB | ~$550 | ~$40 |
| 50K | Light | ~75 TB | ~$700 | ~$55 |
| 50K | Moderate | ~300 TB | ~$2,950 | ~$280 |
| 100K | Moderate | ~600 TB | ~$5,950 | ~$580 |

> **⚠️ Bandwidth is the #1 cost driver at scale.** Hetzner's 20TB included + €1/TB overage is dramatically cheaper than DigitalOcean's $10/TB.

### Cloudflare Tunnel Bandwidth

- **Free plan:** No stated bandwidth limits for tunnels
- **Limitation:** TOS prohibits serving disproportionate non-HTML content
- **WebSocket traffic:** Supported, no hard limits found
- **Proxy traffic:** Goes direct through your server's network, NOT through Cloudflare (tunnel is only for WSS ingress and dashboard)
- **Risk:** At very high volume, Cloudflare may ask you to upgrade to Business/Enterprise

### Bandwidth Optimization Strategies

1. **Proxy traffic bypasses Cloudflare** — SOCKS5/HTTP ports go direct, only WS registration uses tunnel
2. **Compression** — gzip WebSocket frames for heartbeats and metadata
3. **Regional routing** — same-region proxy traffic avoids cross-region bandwidth
4. **Traffic caps per node** — Prevent bandwidth abuse by limiting per-node throughput

---

## Cost Summary

### Monthly Cost Comparison Across Phases

| Phase | Nodes | Infrastructure | Est. Bandwidth | **Total/mo** |
|-------|-------|---------------|----------------|-------------|
| **Current** | 1.5K | ~$96 | Included | **~$96** |
| **Phase 1** | ≤5K | ~$67 | Included | **~$67** |
| **Phase 2** | ≤10K | ~$74-104 | ~$0-40 | **~$74-144** |
| **Phase 3** | ≤50K | ~$203-233 | ~$40-280 | **~$243-513** |
| **Phase 4** | ≤100K+ | ~$1,050-1,350 | ~$280-580 | **~$1,330-1,930** |

### Cost Per Node

| Phase | Total Cost | Nodes | **Cost/Node/mo** |
|-------|-----------|-------|-----------------|
| Current | $96 | 1,500 | $0.064 |
| Phase 1 | $67 | 5,000 | **$0.013** |
| Phase 2 | $104 | 10,000 | **$0.010** |
| Phase 3 | $350 | 50,000 | **$0.007** |
| Phase 4 | $1,500 | 100,000 | **$0.015** |

> Target: Keep cost per node under **$0.02/mo** to maintain margins. Revenue per node should be 5-10× this.

### Revenue Needed to Break Even

Assuming proxy service pricing of $2-5/GB sold to customers:

| Phase | Monthly Cost | GB Sold Needed (@$3/GB) | Equivalent to |
|-------|-------------|------------------------|---------------|
| Phase 1 | $67 | 23 GB | ~1 customer |
| Phase 2 | $144 | 48 GB | ~2-3 customers |
| Phase 3 | $513 | 171 GB | ~10 customers |
| Phase 4 | $1,930 | 643 GB | ~30 customers |

---

## Migration Playbook: Phase 1

### Pre-Migration (Day 0)

```bash
# 1. Create Hetzner Cloud account (if not exists)
# Sign up at hetzner.com/cloud
# Add payment method, verify account

# 2. Create server
# Location: Ashburn (ASH) or Hillsboro (HIL)
# Type: CX53 (16 vCPU, 32 GB RAM, 320 GB NVMe)
# OS: Ubuntu 24.04
# Enable backups (20% of server cost = ~$4/mo)
# Add SSH key

# 3. Note the server IP (keep private!)
```

### Server Setup (Day 1)

```bash
# SSH into new server
ssh root@<NEW_SERVER_IP>

# 1. System updates
apt update && apt upgrade -y

# 2. Install Docker + Docker Compose
curl -fsSL https://get.docker.com | sh
apt install docker-compose-plugin -y

# 3. Install Cloudflare tunnel (cloudflared)
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb -o cloudflared.deb
dpkg -i cloudflared.deb

# 4. Kernel tuning for high connection counts
cat >> /etc/sysctl.conf << 'EOF'
# IPLoop WebSocket tuning
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
fs.file-max = 1000000
net.ipv4.ip_local_port_range = 1024 65535
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
net.core.netdev_max_backlog = 65536
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15
EOF
sysctl -p

# 5. Increase file descriptor limits
cat >> /etc/security/limits.conf << 'EOF'
* soft nofile 1000000
* hard nofile 1000000
root soft nofile 1000000
root hard nofile 1000000
EOF

# 6. Docker daemon config for ulimits
cat > /etc/docker/daemon.json << 'EOF'
{
  "default-ulimits": {
    "nofile": { "Name": "nofile", "Hard": 1000000, "Soft": 1000000 }
  },
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
EOF
systemctl restart docker
```

### Application Migration (Day 1-2)

```bash
# 1. Clone the iploop-platform repo to new server
# (or rsync the directory)
rsync -avz --progress /root/clawd-secure/iploop-platform/ \
  root@<NEW_SERVER_IP>:/root/iploop-platform/

# 2. Database migration
# On OLD server:
docker exec iploop-postgres pg_dumpall -U iploop > /tmp/iploop-db-backup.sql
scp /tmp/iploop-db-backup.sql root@<NEW_SERVER_IP>:/tmp/

# On NEW server:
cd /root/iploop-platform
docker compose up -d postgres redis
sleep 10
docker exec -i iploop-postgres psql -U iploop < /tmp/iploop-db-backup.sql

# 3. Start all services
docker compose up -d

# 4. Verify services are healthy
docker compose ps
curl -s http://localhost:8001/health  # node-registration
curl -s http://localhost:8002/health  # customer-api
```

### Cloudflare Tunnel Migration (Day 2)

```bash
# Option A: Move the existing tunnel
# On OLD server:
cloudflared service uninstall

# On NEW server:
cloudflared tunnel login
cloudflared tunnel route dns iploop-gateway gateway.iploop.io
cloudflared service install <TUNNEL_TOKEN>
systemctl enable cloudflared
systemctl start cloudflared

# Option B: Create new tunnel, update DNS (zero-downtime)
cloudflared tunnel create iploop-gateway-v2
# Configure ingress rules in config.yml
# Update Cloudflare DNS to point to new tunnel
# Wait for all nodes to reconnect (SDK auto-reconnects)
# Delete old tunnel
```

### DNS & Traffic Cutover (Day 2-3)

```bash
# 1. Update Cloudflare tunnel config to point to new server
# 2. Nodes will auto-reconnect via SDK (v1.0.57 has persistent reconnect)
# 3. Monitor:
#    - WebSocket connection count recovery
#    - Proxy traffic flowing
#    - Error rates in logs
#    - Memory/CPU on new server

# 4. Keep old server running for 48h as fallback
# 5. Once stable, tear down IPLoop services on old server
```

### Post-Migration Verification

```bash
# Check node count recovered
curl -s http://localhost:8001/api/stats | jq '.connected_nodes'

# Check proxy is working
curl -x socks5://localhost:1080 -U customer:api_key https://httpbin.org/ip

# Check dashboard loads
curl -s -o /dev/null -w '%{http_code}' https://iploop.io

# Monitor for 24h
watch -n 30 'docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"'
```

### Rollback Plan

If migration fails:
1. Re-enable Cloudflare tunnel on old server
2. Nodes auto-reconnect within 10 minutes (SDK backoff)
3. Investigate issues on new server
4. Retry when ready

---

## Decision Framework

### When to Move to Next Phase

| Trigger | Action |
|---------|--------|
| Swap usage > 0 on IPLoop server | Upgrade server or optimize |
| CPU sustained > 70% | Scale up or separate services |
| Memory > 80% | Scale up or optimize WS library |
| Node count > 80% of projected capacity | Begin next phase planning |
| Customer complaints about latency | Add regional server |
| Revenue > 3× infrastructure cost | Invest in next phase |
| Single region failure impacts all customers | Urgently deploy multi-region |

### Provider Decision Matrix

| Factor | DigitalOcean | Hetzner |
|--------|-------------|---------|
| **Price** | Higher (5-10×) | ✅ Lowest |
| **US regions** | NYC, SFO, ATL | ASH, HIL |
| **EU regions** | LON, AMS, FRA | NBG, HEL, FRA |
| **Managed DB** | ✅ Excellent | ❌ Not available |
| **Managed K8s** | ✅ DOKS | ❌ Not available |
| **Bandwidth** | $10/TB overage | ✅ ~$1/TB overage |
| **API/Automation** | ✅ Mature | ✅ Good |
| **Support** | Good | Basic |
| **Best for** | Managed services, K8s | Raw compute, bandwidth |

### Recommended Path

```
NOW ──→ Phase 1 (Hetzner ASH, ~$67/mo)
         │
         ├── 5K nodes, stable revenue
         ▼
      Phase 2 (Separate services, ~$104/mo)
         │
         ├── 10K+ nodes, geo demand
         ▼
      Phase 3 (Multi-region, ~$350/mo)
         │
         ├── 50K+ nodes, enterprise customers
         ▼
      Phase 4 (Full scale, ~$1,500/mo)
```

---

## Appendix: Quick Reference

### DigitalOcean Regions

| Slug | Location | Best For |
|------|----------|----------|
| nyc1/nyc3 | New York City | US East customers |
| sfo3 | San Francisco | US West customers |
| lon1 | London | UK/EU customers |
| fra1 | Frankfurt | EU customers |
| tor1 | Toronto | Canadian customers |
| syd1 | Sydney | APAC customers |
| sgp1 | Singapore | Southeast Asia |
| ams3 | Amsterdam | EU customers |
| blr1 | Bangalore | India |
| atl1 | Atlanta | US Southeast |

### Hetzner Regions

| Location | Slug | Best For |
|----------|------|----------|
| Nuremberg, DE | nbg1 | EU (cheapest) |
| Helsinki, FI | hel1 | Northern EU |
| Ashburn, VA | ash | US East ⭐ |
| Hillsboro, OR | hil | US West |
| Singapore | sin | APAC |

### Key Metrics to Monitor

- `ws_connections_active` — Current WebSocket connections
- `proxy_requests_total` — Proxy requests per second
- `proxy_bytes_transferred` — Bandwidth consumption
- `node_registration_latency_ms` — Time to register new node
- `proxy_request_latency_ms` — End-to-end proxy latency
- `system_memory_used_percent` — Server memory utilization
- `system_cpu_used_percent` — Server CPU utilization
- `go_goroutines` — Active goroutines (WS server)
- `postgres_connections_active` — DB connection pool usage

---

*This plan is a living document. Update as the platform grows and pricing changes.*
