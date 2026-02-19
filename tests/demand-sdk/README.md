# IPLoop Demand SDK Test Suite

Comprehensive test suite for validating the IPLoop Python SDK against real-world use cases.

## Quick Start

```bash
# Install dependencies
pip install --upgrade --break-system-packages iploop

# Run complete test suite
python test_full.py

# Run specific tests  
python test_sticky.py      # Session management
python test_countries.py   # Country targeting
python test_scraping.py    # Web scraping use cases
python test_performance.py # Performance benchmarks
```

## Test Scripts

| Script | Purpose | Duration | Key Metrics |
|--------|---------|----------|-------------|
| `test_full.py` | Complete SDK validation | ~15 min | Overall success rate, block rate |
| `test_sticky.py` | Session management | ~5 min | Session consistency, IP persistence |
| `test_countries.py` | Geo-targeting accuracy | ~10 min | Country availability, targeting accuracy |
| `test_scraping.py` | Real-world use cases | ~20 min | Site access, business viability |
| `test_performance.py` | Speed & throughput | ~15 min | Response times, concurrency |

## Documentation

- **[PLAYBOOK.md](PLAYBOOK.md)** - Complete QA manual with step-by-step instructions
- **[results/](results/)** - Test results archive (JSON + Markdown reports)

## Latest Results (2026-02-20)

- **Overall Success Rate:** 7.9% 🔴
- **Block Rate:** 67.1% 🔴  
- **Average Response Time:** 2,922ms 🟠
- **Production Ready:** NO 🚫

**Critical Issues:**
- Country targeting completely broken
- Major e-commerce sites 100% blocked
- No anti-detection or Cloudflare bypass
- Requires immediate fixes before production use

## Usage Examples

```bash
# Basic testing
python test_full.py --api-key your_key_here

# Test specific categories
python test_scraping.py --categories ecommerce,travel

# Performance benchmarking  
python test_performance.py --concurrent-workers 20

# Country targeting validation
python test_countries.py --countries US,GB,DE,FR

# Save results for analysis
python test_full.py --output json --verbose
```

## Interpreting Results

- ✅ **PASS** - Test successful
- 🔶 **PARTIAL** - Partially working
- 🚫 **BLOCKED** - Site blocked request
- ❌ **FAIL** - Test failed
- ⏰ **TIMEOUT** - Request timed out

See [PLAYBOOK.md](PLAYBOOK.md) for detailed interpretation guidance.

## Contributing

When adding new test cases:
1. Follow the existing test structure
2. Include proper error handling
3. Add meaningful assertions
4. Update this README
5. Document expected results in PLAYBOOK.md