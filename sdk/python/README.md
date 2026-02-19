# IPLoop Python SDK

Residential proxy SDK — one-liner web fetching through millions of real IPs.

```bash
pip install iploop
```

## Quick Start

```python
from iploop import IPLoop

ip = IPLoop("your-api-key")

# Fetch any URL through a residential proxy
response = ip.get("https://httpbin.org/ip")
print(response.text)

# Target a specific country
response = ip.get("https://example.com", country="DE")

# POST request
response = ip.post("https://api.example.com/data", json={"key": "value"})
```

## Smart Headers

Headers are automatically matched to the target country — correct language, timezone, and User-Agent:

```python
ip = IPLoop("key", country="JP")  # Japanese Chrome headers automatically
```

## Sticky Sessions

Keep the same IP across multiple requests:

```python
s = ip.session(country="US", city="newyork")
page1 = s.fetch("https://site.com/page1")  # same IP
page2 = s.fetch("https://site.com/page2")  # same IP
```

## Auto-Retry

Failed requests (403, 502, 503, timeouts) automatically retry with a fresh IP:

```python
# Retries up to 3 times with different IPs
response = ip.get("https://tough-site.com", retries=5)
```

## Async Support

```python
import asyncio
from iploop import AsyncIPLoop

async def main():
    async with AsyncIPLoop("key") as ip:
        results = await asyncio.gather(
            ip.get("https://site1.com"),
            ip.get("https://site2.com"),
            ip.get("https://site3.com"),
        )
        for r in results:
            print(r.status_code)

asyncio.run(main())
```

## Support API

```python
ip.usage()     # Check bandwidth quota
ip.status()    # Service status
ip.ask("how do I handle captchas?")  # Ask support
ip.countries() # List available countries
```

## Data Extraction (v1.2.0)

Auto-extract structured data from popular sites:

```python
# eBay — extract product listings
products = ip.ebay.search("laptop", extract=True)["products"]
# [{"title": "MacBook Pro 16", "price": "$1,299.00"}, ...]

# Nasdaq — extract stock quotes
quote = ip.nasdaq.quote("AAPL", extract=True)
# {"price": "$185.50", "change": "+2.30", "pct_change": "+1.25%"}

# Google — extract search results
results = ip.google.search("best proxy service", extract=True)["results"]
# [{"title": "...", "url": "..."}, ...]

# Twitter — extract profile info
profile = ip.twitter.profile("elonmusk", extract=True)
# {"name": "Elon Musk", "handle": "elonmusk", ...}

# YouTube — extract video metadata
video = ip.youtube.video("dQw4w9WgXcQ", extract=True)
# {"title": "...", "channel": "...", "views": 1234567}
```

## Smart Rate Limiting

Built-in per-site rate limiting prevents blocks automatically:

```python
# These calls auto-delay to respect site limits
for q in ["laptop", "phone", "tablet"]:
    ip.ebay.search(q)  # 15s delay between requests
```

## LinkedIn (New)

```python
ip.linkedin.profile("satyanadella")
ip.linkedin.company("microsoft")
```

## Concurrent Fetching (v1.3.0)

Batch fetch up to 25 URLs in parallel:

```python
# Concurrent fetching (safe up to 25)
batch = ip.batch(max_workers=10)
results = batch.fetch_all([
    "https://ebay.com/sch/i.html?_nkw=laptop",
    "https://ebay.com/sch/i.html?_nkw=phone",
    "https://ebay.com/sch/i.html?_nkw=tablet"
], country="US")

# Multi-country comparison
prices = batch.fetch_multi_country("https://ebay.com/sch/i.html?_nkw=iphone", ["US", "GB", "DE"])
```

## Chrome Fingerprinting (v1.3.0)

Every request auto-applies a 14-header Chrome desktop fingerprint — the universal recipe from Phase 9 testing:

```python
# Auto fingerprinting — no setup needed
html = ip.fetch("https://ebay.com", country="US")  # fingerprinted automatically

# Get fingerprint headers directly
headers = ip.fingerprint("DE")  # 14 headers for German Chrome
```

## Stats Tracking (v1.3.0)

```python
# After making requests...
print(ip.stats)
# {"requests": 10, "success": 9, "errors": 1, "total_time": 23.5, "avg_time": 2.35, "success_rate": 90.0}
```

## Debug Mode

```python
ip = IPLoop("key", debug=True)
# Logs: GET https://example.com → 200 (0.45s) country=US session=abc123
```

## Exceptions

```python
from iploop import AuthError, QuotaExceeded, ProxyError, TimeoutError

try:
    response = ip.get("https://example.com")
except QuotaExceeded:
    print("Upgrade at https://iploop.io/pricing")
except ProxyError:
    print("Proxy connection failed")
except TimeoutError:
    print("Request timed out")
```

## Links

- **Website**: https://iploop.io
- **Docs**: https://docs.iploop.io
- **Dashboard**: https://iploop.io/dashboard
