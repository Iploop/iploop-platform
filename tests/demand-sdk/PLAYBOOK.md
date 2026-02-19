# IPLoop Demand SDK QA Playbook

**Version:** 1.0  
**Last Updated:** 2026-02-20  
**Target Audience:** QA Engineers, Product Managers, Developers

---

## 🎯 Overview

This playbook provides step-by-step instructions for testing the IPLoop Python SDK across all real-world use cases. It covers everything from basic connectivity to complex web scraping scenarios.

## 📋 Prerequisites

### Environment Setup
```bash
# 1. Install IPLoop SDK
pip install --upgrade --break-system-packages iploop

# 2. Navigate to test directory
cd /root/clawd-secure/iploop-platform/tests/demand-sdk/

# 3. Verify installation
python -c "from iploop import IPLoop; print('IPLoop SDK ready')"
```

### API Key Configuration
- **Default Test Key:** `testkey123` (works for all tests)
- **Production Key:** Use actual customer API key when available
- **Invalid Key Test:** Use `invalid_key_12345` for error testing

---

## 🧪 Test Categories

### 1. Complete Test Suite (`test_full.py`)
**Purpose:** Comprehensive testing of all SDK functionality  
**Duration:** ~10-15 minutes  
**Use When:** Full QA validation, release testing, regression testing

```bash
# Basic run
python test_full.py

# With custom API key
python test_full.py --api-key your_key_here

# Verbose output
python test_full.py --verbose

# Save results
python test_full.py --output json
```

**Expected Results:**
- ✅ **80%+ success rate** = Production ready
- 🟡 **60-79% success rate** = Minor issues to fix
- 🟠 **40-59% success rate** = Significant problems
- 🔴 **<40% success rate** = Major issues, not production ready

**Key Metrics to Watch:**
- Basic connectivity should be 100% PASS
- Session consistency should be 100% PASS
- Country targeting >60% accuracy
- Web scraping <30% block rate

---

### 2. Sticky Sessions (`test_sticky.py`)
**Purpose:** Test session management and IP persistence  
**Duration:** ~5 minutes  
**Critical For:** Long-running scraping jobs, session-based applications

```bash
# Standard test
python test_sticky.py

# Stress test (more sessions and requests)
python test_sticky.py --session-count 10 --requests-per-session 20

# Longer persistence test
python test_sticky.py --persistence-delay 60

# Save detailed results
python test_sticky.py --output-file sticky_results.json
```

**Expected Results:**
- **Session Consistency:** 100% PASS (same IP across all requests in a session)
- **Session Diversity:** 80%+ PASS (different sessions get different IPs)
- **Session Persistence:** 100% PASS (IP persists over 30+ seconds)

**Red Flags:**
- ❌ Multiple IPs within same session = Session management broken
- ❌ All sessions get same IP = No IP diversity
- ❌ Session IP changes after delay = Poor persistence

---

### 3. Country Targeting (`test_countries.py`)
**Purpose:** Test geo-targeting accuracy and availability  
**Duration:** ~8-12 minutes (depending on countries tested)  
**Critical For:** Region-specific scraping, ad verification, localized content

```bash
# Standard test (7 major countries with geo verification)
python test_countries.py

# Test specific countries
python test_countries.py --countries US,GB,DE,FR,JP,AU

# Skip geo verification for faster testing
python test_countries.py --skip-geo

# Include performance testing
python test_countries.py --test-performance

# Save results
python test_countries.py --output-file country_results.json
```

**Expected Results:**
- **Availability:** 80%+ countries should return IPs
- **Accuracy (with geo verification):** 70%+ should be correctly located
- **Performance:** <3000ms average response time

**Geo Verification Notes:**
- Uses multiple APIs (IP-API, IPAPI.co, FreeGeoIP) for redundancy
- Consensus required (majority of APIs must agree)
- Some discrepancies are normal due to IP database differences

**Common Issues:**
- Low accuracy often indicates datacenter IPs instead of residential
- Complete failures may indicate country not supported
- High response times may indicate distant proxy servers

---

### 4. Web Scraping (`test_scraping.py`)
**Purpose:** Test real-world scraping scenarios  
**Duration:** ~15-25 minutes  
**Critical For:** Customer use cases validation

```bash
# Test all categories
python test_scraping.py

# Test specific use cases
python test_scraping.py --categories ecommerce,travel,social

# Faster testing (shorter timeout)
python test_scraping.py --timeout 10 --delay 0.5

# With country targeting
python test_scraping.py --country US

# Save comprehensive results
python test_scraping.py --output-file scraping_results.json
```

**Categories Available:**
- `ecommerce` - Amazon, eBay, Walmart, Target, etc.
- `travel` - Booking.com, Expedia, Kayak, etc.
- `tickets` - Ticketmaster, StubHub, Eventbrite, etc.
- `ads` - Google/Bing search ads verification
- `seo` - SERP monitoring (Google, Bing, DuckDuckGo)
- `social` - Instagram, TikTok, Pinterest, etc.
- `realestate` - Zillow, Realtor.com, Redfin
- `finance` - Yahoo Finance, NASDAQ, Bloomberg

**Expected Results:**
- **Overall Success Rate:**
  - 70%+ = Production ready for most use cases
  - 50-69% = Usable with workarounds
  - 30-49% = Limited viability
  - <30% = Not viable for production

**Business Viability Assessment:**
- 🟢 **EXCELLENT (80%+ success)** = Ready for customer use
- 🟡 **GOOD (60-79%)** = Usable with minor improvements
- 🟠 **LIMITED (30-59%)** = Significant issues need fixing
- 🔴 **POOR (<30%)** = Major overhaul required

**Block Rate Analysis:**
- <20% blocked = Excellent anti-detection
- 20-40% blocked = Acceptable for most use cases
- 40-60% blocked = High risk, improvements needed
- >60% blocked = Serious anti-detection issues

---

### 5. Performance Benchmarks (`test_performance.py`)
**Purpose:** Test response times, throughput, and scalability  
**Duration:** ~10-15 minutes  
**Critical For:** High-volume applications, SLA validation

```bash
# Full performance suite
python test_performance.py

# Basic performance only
python test_performance.py --tests basic

# High concurrency test
python test_performance.py --tests concurrent --concurrent-workers 20

# Custom request volume
python test_performance.py --basic-requests 100

# Save results for analysis
python test_performance.py --output-file performance_results.json
```

**Test Types:**
- **Basic:** Sequential requests (default: 50 requests)
- **Concurrent:** Parallel requests (default: 10 workers × 5 requests)
- **Scaling:** Test 1, 2, 5, 10 concurrent workers
- **Endpoints:** Different response types and sizes
- **Sessions:** Compare session vs non-session performance

**Expected Results:**
- **Average Response Time:**
  - <1000ms = Excellent
  - 1000-2000ms = Good
  - 2000-5000ms = Acceptable
  - >5000ms = Poor

- **Success Rate:**
  - >95% = Excellent
  - 90-95% = Good
  - 80-90% = Acceptable
  - <80% = Poor

- **Throughput:**
  - >10 req/s sequential = Good
  - >20 req/s concurrent = Good
  - Scaling efficiency >70% = Good

---

## 🔍 Interpreting Results

### Status Indicators
- ✅ **PASS** - Test successful, meets requirements
- 🔶 **PARTIAL** - Test partially successful, minor issues
- 🚫 **BLOCKED** - Site blocked the request (CAPTCHA/403/429)
- ❌ **FAIL** - Test failed due to error or timeout
- ⏰ **TIMEOUT** - Request timed out
- 🔄 **REDIRECT** - Request redirected

### Common Failure Patterns

#### High Block Rate (>40%)
**Symptoms:**
- Many sites return CAPTCHAs
- HTTP 403 Forbidden errors
- "Bot detection" messages

**Root Causes:**
- Poor IP reputation (datacenter IPs)
- Missing browser fingerprinting
- No anti-detection measures
- Predictable request patterns

**Solutions:**
- Implement stealth headers
- Add Cloudflare bypass
- Use residential IP pool
- Randomize request timing

#### Poor Country Targeting
**Symptoms:**
- Geo APIs return wrong countries
- Low accuracy percentages
- "XX" country codes

**Root Causes:**
- Datacenter IPs instead of residential
- Incorrect geo database
- Proxy routing issues

**Solutions:**
- Verify IP pool sources
- Implement geo verification
- Add fallback countries

#### Slow Performance
**Symptoms:**
- >3000ms average response time
- Poor concurrent scaling
- Frequent timeouts

**Root Causes:**
- Distant proxy servers
- Poor routing
- Overloaded infrastructure
- No connection pooling

**Solutions:**
- Optimize proxy locations
- Implement connection pooling
- Add performance monitoring
- Load balancing

---

## 📊 Reporting & Documentation

### Manual Test Report Template
```markdown
# IPLoop SDK QA Report
**Date:** YYYY-MM-DD
**Tester:** [Name]
**SDK Version:** [Version]
**Test Environment:** [Details]

## Executive Summary
- Overall Grade: [EXCELLENT/GOOD/NEEDS_WORK/POOR]
- Success Rate: [X%]
- Block Rate: [X%]
- Avg Response Time: [X]ms

## Test Results
### Basic Connectivity: [PASS/FAIL]
### Sticky Sessions: [PASS/FAIL]
### Country Targeting: [PASS/PARTIAL/FAIL] ([X]% accuracy)
### Web Scraping: [PASS/PARTIAL/FAIL] ([X]% success rate)
### Performance: [EXCELLENT/GOOD/ACCEPTABLE/POOR]

## Critical Issues
1. [Issue description]
2. [Issue description]

## Recommendations
1. [Recommendation]
2. [Recommendation]

## Business Impact
[Assessment of readiness for production use]
```

### Automated Reporting
All test scripts generate JSON output that can be:
- Parsed by CI/CD systems
- Imported into monitoring tools
- Analyzed with data visualization tools
- Archived for trend analysis

### Key Metrics Dashboard
Monitor these metrics over time:
- Overall success rate trend
- Block rate by category
- Average response time
- Country targeting accuracy
- Performance regression detection

---

## 🚨 Escalation Criteria

### Immediate Escalation (Block Release)
- Basic connectivity <90% success
- Session management completely broken
- >70% overall block rate
- Average response time >10 seconds
- No working countries

### High Priority Issues
- Success rate <50%
- Block rate >50%
- Major e-commerce sites all blocked
- Performance degradation >50%
- Country targeting accuracy <30%

### Medium Priority Issues
- Success rate 50-70%
- Block rate 30-50%
- Some category failures
- Performance issues <50% degradation

---

## 🔄 Continuous Testing

### Daily Smoke Tests
```bash
# Quick validation (5 minutes)
python test_full.py --api-key testkey123 | grep "Final Assessment"
```

### Weekly Comprehensive Testing
```bash
# Full test suite with results saving
python test_full.py --output json --verbose > weekly_results.log
python test_scraping.py --output-file weekly_scraping.json
python test_performance.py --output-file weekly_performance.json
```

### Monthly Deep Dive
- Run all tests with maximum settings
- Compare results month-over-month
- Update test targets based on customer feedback
- Review and update success criteria

---

## 📞 Support & Troubleshooting

### Common Problems

**Q: Tests fail with "Connection refused"**
A: Check if proxy service is running and accessible

**Q: All countries return "XX"**
A: Geo verification API may be down, try `--skip-geo` flag

**Q: Very slow performance**
A: Network issues or proxy overload, check system resources

**Q: High block rate suddenly**
A: Proxy IPs may be blacklisted, requires IP pool refresh

### Getting Help
1. Check test logs for specific error messages
2. Run individual test categories to isolate issues
3. Verify network connectivity and API key validity
4. Compare results with previous successful runs

### Test Data Files
- Results saved to `results/` directory
- JSON format for programmatic analysis
- Include timestamp and configuration details
- Archive for historical comparison

---

*This playbook should be updated as new features are added and customer requirements evolve.*