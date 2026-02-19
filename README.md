# IPLoop — Two-Sided Residential Proxy Network

Enterprise-grade residential proxy platform with two sides:

| Side | What | For Who | Install |
|------|------|---------|---------|
| 🖥️ **Demand** | Proxy & unblocking SDK | Developers, scrapers, data teams | `pip install iploop` / `npm install iploop` |
| 📱 **Supply** | Earn rewards SDK | App developers, device owners | Android SDK / Docker node |

## 🖥️ DEMAND SIDE — Proxy & Unblocking SDK

### Python
```bash
pip install iploop
```
```python
from iploop import IPLoop

client = IPLoop("your-api-key", country="US")
resp = client.get("https://example.com")
print(resp.status_code, resp.text[:100])
```

### Node.js
```bash
npm install iploop
```
```typescript
import { IPLoopClient } from 'iploop';

const client = new IPLoopClient({ apiKey: 'your-api-key', country: 'US' });
const resp = await client.get('https://example.com');
console.log(resp.status, resp.data);
```

## ✨ Features

- 🌍 **192+ Countries** — Target any country, city, or ASN
- 🔄 **Auto IP Rotation** — Fresh IP on every request
- 📌 **Sticky Sessions** — Same IP across multiple requests
- 🕵️ **Chrome Fingerprinting** — 14-header browser fingerprint, country-matched
- 🔁 **Auto-Retry** — 3 attempts with backoff and IP rotation
- ⚡ **Batch Scraping** — Concurrent requests (up to 25 workers)
- 🎯 **Site Presets** — Optimized for Google, Amazon, Twitter, Instagram, TikTok, YouTube, Reddit, eBay, LinkedIn, Nasdaq
- 📊 **Usage Stats** — Built-in request tracking
- 🔒 **Auth Validation** — Custom error types for all failure modes

## 🚀 Quick Examples

### Geo-Targeting
```python
# Python
resp = client.get("https://amazon.de", country="DE", city="berlin")

// Node.js
const resp = await client.get('https://amazon.de', { country: 'DE', city: 'berlin' });
```

### Sticky Sessions
```python
# Python — same IP across requests
session = client.session(country="US")
page1 = session.get("https://example.com/page1")
page2 = session.get("https://example.com/page2")

// Node.js
const session = client.session(undefined, 'US');
const page1 = await session.get('https://example.com/page1');
const page2 = await session.get('https://example.com/page2');
```

### Batch Scraping
```python
# Python — concurrent requests
urls = ["https://site.com/1", "https://site.com/2", "https://site.com/3"]
results = client.batch(max_workers=10).fetch(urls)

// Node.js
const results = await client.fetchAll(urls, {}, 10);
```

### Site Presets
```python
# Python — optimized for specific sites
results = client.google.search("residential proxy")
profile = client.twitter.profile("elonmusk")
product = client.amazon.product("B09V3KXJPB")
```

### Proxy URL (for Puppeteer, Playwright, etc.)
```python
# Python
proxy_url = client._proxy_url(country="US")
# → http://your-api-key:country-us@gateway.iploop.io:8880

// Node.js
const proxyUrl = client.getProxyUrl({ country: 'US' });
// HTTP: http://your-api-key:country-us@gateway.iploop.io:8880
// SOCKS5: socks5://your-api-key:country-us@gateway.iploop.io:1080
```

## 📱 SUPPLY SIDE — Earn Rewards SDK

Monetize unused bandwidth by integrating the IPLoop SDK into your Android app, or run a Docker node.

### Android SDK
```java
IPLoopSDK.init(context, "your-partner-id");
IPLoopSDK.start();  // shares idle bandwidth, earns rewards
```
- Min SDK 22 (Android 5.1+), < 50MB RAM, auto-reconnect
- [Download Android SDK](https://github.com/Iploop/iploop-node)

### Docker Node
```bash
docker run -d --name iploop-node --restart=always ultronloop2026/iploop-node:latest
```
1 GB shared = 1 GB proxy access. Supports Linux, macOS, Windows, Raspberry Pi.

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│                  IPLoop Platform                     │
│                                                      │
│  📱 SUPPLY (Earn Rewards)   🖥️ DEMAND (Proxy/Unblock) │
│  ┌──────────────┐          ┌──────────────────┐     │
│  │ Android SDK  │──WSS──▶  │ Node Registration │     │
│  │ 20K+ devices │          │   (port 9443)     │     │
│  └──────────────┘          └────────┬─────────┘     │
│                                     │                │
│                             ┌───────▼─────────┐     │
│  ┌──────────────┐          │  Proxy Gateway   │     │
│  │ Python SDK   │──HTTP──▶ │  HTTP  :8880     │     │
│  │ Node.js SDK  │          │  SOCKS5 :1080    │     │
│  └──────────────┘          └──────────────────┘     │
│                                                      │
│  ┌──────────────┐          ┌──────────────────┐     │
│  │  Dashboard   │◀─────── │  Customer API     │     │
│  │ gateway.iploop.io       │  Auth + Billing   │     │
│  └──────────────┘          └──────────────────┘     │
└─────────────────────────────────────────────────────┘
```

## 📊 Network Stats

- **20,000+** active residential nodes
- **192** countries covered
- **97%+** tunnel success rate
- **HTTP + SOCKS5** proxy protocols

## 🔗 Links

- **Website:** https://iploop.io
- **Dashboard:** https://gateway.iploop.io
- **PyPI:** https://pypi.org/project/iploop/
- **npm:** https://www.npmjs.com/package/iploop
- **Docker:** https://hub.docker.com/r/ultronloop2026/iploop-node

## 📁 Repository Structure

### Standalone SDKs (public, for customers)

| SDK | Repo | Package | Install |
|-----|------|---------|---------|
| 🐍 Python (Demand) | [`Iploop/iploop-python`](https://github.com/Iploop/iploop-python) | [PyPI](https://pypi.org/project/iploop/) | `pip install iploop` |
| 📦 Node.js (Demand) | [`Iploop/iploop-node-sdk`](https://github.com/Iploop/iploop-node-sdk) | [npm](https://www.npmjs.com/package/iploop) | `npm install iploop` |
| 📱 Android (Supply) | [`Iploop/iploop-node`](https://github.com/Iploop/iploop-node) | JAR | See docs |
| 🐳 Docker (Supply) | [Docker Hub](https://hub.docker.com/r/ultronloop2026/iploop-node) | Docker | `docker run ultronloop2026/iploop-node` |

### Platform (this repo)

```
sdk/
  python/              # Python demand SDK (mirrors Iploop/iploop-python)
  nodejs/              # Node.js demand SDK (mirrors Iploop/iploop-node-sdk)
  android-java/        # Android supply SDK
services/
  node-registration/   # WebSocket hub + node management
  proxy-gateway/       # HTTP/SOCKS5 proxy server (Go)
  customer-api/        # REST API + auth + billing
dashboard/             # Next.js customer dashboard
```

## License

MIT
