# Stability Test — Next Version TODO

## v1.2 — Planned Changes

### 1. Pre-connect IP screening (HTTP POST)
- Add `POST /report-ip` endpoint on server
- SDK fetches IP info first, POSTs to server
- Server responds `{"allowed": true/false}` based on proxy_type (block DCH etc.)
- If blocked → SDK sleeps, doesn't attempt WebSocket
- Server keeps the IP data for records regardless

### 2. Cooldown system (server-side)
- Track reconnect frequency per node_id
- If >10 reconnects in 5 min → reject at WS upgrade (HTTP 429)
- SDK handles rejection: sleep for retry_after period

### 3. Blocklist management
- `/blocked` endpoint to view blocked nodes
- Configurable block rules (by proxy_type, ISP, etc.)
- API to manually add/remove from blocklist

### 4. Move IP info to disk/DB
- Store full ip_info JSON to SQLite or Redis on arrival
- Keep only essential fields in memory (country, city, ISP, proxy_type)
- Store disconnect events to DB instead of in-memory buffer
- Prevents 700MB+ memory bloat at scale

### 5. Add fetch timing to ip_info
- Measure ip2location HTTP request duration on SDK side
- Send `fetch_time_ms` with ip_info message
- Use as connection quality indicator

### 6. Reconnect-loop devices identified
- All DCH nodes are looping (DataCamp, M247, Clouvider, HostPapa)
- Top offender: 20a97f78f855ca34 (Mantrise Cloud LLC, Frankfurt)
- 27 DCH nodes total, 0 stable connections
