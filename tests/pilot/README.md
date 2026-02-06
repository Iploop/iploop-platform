# IPLoop Pilot Test Suite

## Scenario: E-commerce Price Comparison Scraping

Simulates a real customer use case — a price comparison service scraping product data across multiple retailers using IPLoop's residential proxy network.

## Test Suite Overview

| Test | File | Description |
|------|------|-------------|
| **Stress** | `stress-test.js` | Load testing with configurable profiles |
| **Geo** | `geo-test.js` | Geographic targeting verification |
| **Leak** | `leak-test.js` | IP/header leak detection |
| **Sticky** | `sticky-stress.js` | Session stickiness under load |
| **Rotation** | `rotation-test.js` | IP rotation behavior |
| **Bandwidth** | `bandwidth-test.js` | Download speed & large files |
| **Failure** | `failure-test.js` | Error handling & recovery |
| **Scenario** | `scenario-price-scrape.js` | Real-world scraper simulation |

## Quick Start

```bash
# Set proxy config
export PROXY_HOST=localhost
export PROXY_PORT=8080
export PROXY_USER=test_customer
export PROXY_PASS=test_api_key

# Run quick smoke test
./run-tests.sh smoke

# Run all basic tests
./run-tests.sh all

# Run comprehensive full suite
./run-tests.sh full
```

## Test Commands

```bash
./run-tests.sh smoke       # Quick 30s stress test
./run-tests.sh stress      # Full ramp (5 min, 100 concurrent)
./run-tests.sh sustained   # 10 min at 50 concurrent
./run-tests.sh burst       # Spike to 200 concurrent
./run-tests.sh endurance   # 1 hour at 25 concurrent
./run-tests.sh geo         # Geographic targeting
./run-tests.sh leak        # IP leak detection
./run-tests.sh sticky      # Session stickiness stress
./run-tests.sh rotation    # IP rotation behavior
./run-tests.sh bandwidth   # Download speed tests
./run-tests.sh failure     # Error handling
./run-tests.sh scenario    # Real-world scraper sim
./run-tests.sh all         # smoke + geo + leak
./run-tests.sh full        # ALL tests
```

## Stress Test Profiles

| Profile | Concurrent | Duration | Ramp |
|---------|------------|----------|------|
| smoke | 5 | 30s | 5s |
| ramp | 100 | 5 min | 2 min |
| sustained | 50 | 10 min | 30s |
| burst | 200 | 2 min | 10s |
| endurance | 25 | 1 hour | 1 min |

## Metrics Tracked

### Performance
- Request throughput (req/s)
- Latency (P50, P95, P99)
- Bandwidth (Mbps)
- Error rates by type

### IP Management
- Unique IPs observed
- IP rotation rate
- Session stickiness accuracy
- Geographic targeting accuracy

### Reliability
- Connection timeouts
- Failed requests
- Recovery after failures
- Peer disconnect handling

## Bug Detection Checklist

### Critical
- [ ] IP leaks (server IP exposed via proxy)
- [ ] Header leaks (X-Forwarded-For, Via)
- [ ] Auth bypass
- [ ] Connection failures > 5%

### Important
- [ ] Session stickiness breaking
- [ ] Geographic targeting wrong
- [ ] P95 latency > 10s
- [ ] Low IP diversity

### Minor
- [ ] Occasional timeouts under load
- [ ] Slow ramp-up
- [ ] Memory growth (long tests)

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PROXY_HOST` | localhost | Proxy hostname |
| `PROXY_PORT` | 8080 | Proxy port |
| `PROXY_USER` | test_customer | Auth username |
| `PROXY_PASS` | test_api_key | Auth password |
| `SESSIONS` | 50 | Concurrent sessions (sticky test) |
| `REQUESTS` | 50 | Requests per test |
| `SCRAPERS` | 10 | Concurrent scrapers (scenario) |

## Requirements

- Node.js 18+
- IPLoop gateway running
- At least 1 peer connected (more = better test)
- Network connectivity to httpbin.org

## Interpreting Results

### ✅ PASS Criteria
- Success rate ≥ 95%
- P95 latency < 10s
- Session stickiness ≥ 95%
- No IP leaks detected
- Geographic targeting ≥ 80% accurate

### ⚠️ Warning Signs
- Success rate 90-95%
- Low IP diversity (< 3 unique)
- Some session breaks
- Occasional timeouts

### ❌ Failure Indicators
- Success rate < 90%
- IP leaks detected
- Frequent session breaks
- Connection refused errors
