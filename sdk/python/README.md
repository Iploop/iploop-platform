# IPLoop Python SDK

Official Python SDK for [IPLoop](https://iploop.io) residential proxy service.

## Installation

```bash
pip install iploop
```

For async support:
```bash
pip install iploop[async]
```

## Quick Start

```python
from iploop import IPLoopClient

# Initialize client
client = IPLoopClient(api_key="your_api_key")

# Make a simple request
response = client.get("https://httpbin.org/ip")
print(response.json())
```

## Features

### Country Targeting

```python
# Request from specific country
response = client.get("https://example.com", country="US")

# City-level targeting
response = client.get("https://example.com", country="US", city="new york")
```

### Sticky Sessions

```python
# Use same IP for multiple requests
response1 = client.get("https://example.com", session="my_session")
response2 = client.get("https://example.com/page2", session="my_session")
# Both requests use the same IP
```

### Using with requests library

```python
import requests
from iploop import IPLoopClient

client = IPLoopClient(api_key="your_api_key")

# Get proxy configuration
proxies = client.get_proxy(country="DE")

# Use with any requests call
response = requests.get("https://example.com", proxies=proxies)
```

### SOCKS5 Protocol

```python
proxies = client.get_proxy(country="US", protocol="socks5")
```

### Async Support

```python
import asyncio
from iploop import AsyncIPLoopClient

async def main():
    async with AsyncIPLoopClient(api_key="your_api_key") as client:
        response = await client.get("https://example.com", country="US")
        print(await response.text())

asyncio.run(main())
```

## API Methods

### Usage Statistics

```python
# Get current usage
usage = client.get_usage()
print(f"Used: {usage['total_bytes']} bytes")

# Get daily breakdown
daily = client.get_usage_daily(days=7)
```

### API Key Management

```python
# List all keys
keys = client.list_api_keys()

# Create new key
new_key = client.create_api_key(name="scraper-key")

# Delete key
client.delete_api_key(key_id="key_123")
```

### Subscription

```python
subscription = client.get_subscription()
print(f"Plan: {subscription['plan']}")
print(f"Status: {subscription['status']}")
```

## Error Handling

```python
from iploop import IPLoopClient, AuthenticationError, RateLimitError, QuotaExceededError

client = IPLoopClient(api_key="your_api_key")

try:
    response = client.get("https://example.com")
except AuthenticationError:
    print("Invalid API key")
except RateLimitError as e:
    print(f"Rate limited. Retry after {e.retry_after} seconds")
except QuotaExceededError as e:
    print(f"Quota exceeded: {e.quota_type}")
```

## Configuration

```python
client = IPLoopClient(
    api_key="your_api_key",
    proxy_host="proxy.iploop.io",  # Custom proxy host
    http_port=7777,                 # HTTP proxy port
    socks_port=1080,                # SOCKS5 proxy port
    timeout=30,                     # Request timeout
)
```

## License

MIT License - see LICENSE file for details.
