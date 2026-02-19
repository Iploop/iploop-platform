# PRD — IPLoop Platform Integration
**Date:** 2026-02-19  
**Priority:** HIGH  
**Owner:** Gil (@mikafurhman)

---

## Overview

Connect all IPLoop web properties to the live backend + published SDKs. Every page must clearly communicate our **two-sided model:**

- 🖥️ **DEMAND** — Proxy & unblocking SDK for developers (`pip install iploop` / `npm install iploop`)
- 📱 **SUPPLY** — Earn rewards by sharing bandwidth (Android SDK / Docker node)

---

## Properties to Update

| # | Property | URL | Status |
|---|----------|-----|--------|
| 1 | Platform Dashboard (Web) | https://primepattern.ai/platforminterface/pages/dashboard.html | ⚠️ Old endpoints |
| 2 | Platform Dashboard (Mobile) | https://primepattern.ai/platforminterface/mobile/dashboard.html | ⚠️ Old endpoints |
| 3 | IPLoop Website | https://iploop.io | ⚠️ Missing SDK install |
| 4 | ProxyClaw Landing | https://proxyclaw.ai | ❌ DNS not resolving |

---

## Task 1: Dashboard — Fix Endpoints & Add SDK Examples

**Files:** `dashboard.html` (web + mobile)

### 1.1 Replace old proxy endpoint
```
OLD: pr.iploop.io:7777
NEW: gateway.iploop.io:8880
```

All code examples must use the new endpoint.

### 1.2 Update code examples in Integration section

**cURL tab:**
```bash
curl -x "http://user:YOUR_API_KEY@gateway.iploop.io:8880" \
  https://ip.iploop.io/location

# With country targeting
curl -x "http://user:YOUR_API_KEY-country-US@gateway.iploop.io:8880" \
  https://example.com
```

**Python tab:**
```python
# Install: pip install iploop
from iploop import IPLoop

client = IPLoop("YOUR_API_KEY", country="US")
resp = client.get("https://example.com")
print(resp.status_code, resp.text[:100])

# Geo-targeting
resp = client.get("https://amazon.de", country="DE", city="berlin")

# Sticky session
session = client.session(country="US")
page1 = session.get("https://example.com/page1")
page2 = session.get("https://example.com/page2")
```

**Node.js tab:**
```typescript
// Install: npm install iploop
import { IPLoopClient } from 'iploop';

const client = new IPLoopClient({ apiKey: 'YOUR_API_KEY', country: 'US' });
const resp = await client.get('https://example.com');
console.log(resp.status, resp.data);

// Geo-targeting
const de = await client.get('https://amazon.de', { country: 'DE', city: 'berlin' });

// Sticky session
const session = client.session(undefined, 'US');
```

**PHP tab:**
```php
<?php
$proxy = "http://user:YOUR_API_KEY@gateway.iploop.io:8880";
$ch = curl_init("https://example.com");
curl_setopt($ch, CURLOPT_PROXY, $proxy);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
$response = curl_exec($ch);
curl_close($ch);
echo $response;
```

**Java tab:**
```java
import java.net.*;
Proxy proxy = new Proxy(Proxy.Type.HTTP, new InetSocketAddress("gateway.iploop.io", 8880));
HttpURLConnection conn = (HttpURLConnection) new URL("https://example.com").openConnection(proxy);
conn.setRequestProperty("Proxy-Authorization", "Basic " + 
    Base64.getEncoder().encodeToString("user:YOUR_API_KEY".getBytes()));
```

**Go tab:**
```go
proxyURL, _ := url.Parse("http://user:YOUR_API_KEY@gateway.iploop.io:8880")
client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
resp, _ := client.Get("https://example.com")
```

### 1.3 Connect Dashboard to Real API

**Authentication:**
- Signup: `POST https://gateway.iploop.io:9443/api/auth/signup`
- Login: `POST https://gateway.iploop.io:9443/api/auth/login`
- Returns JWT token for subsequent requests

**Usage Stats (for dashboard cards):**
- `GET https://gateway.iploop.io:9443/api/usage?api_key=KEY`
- Returns: `total_bytes`, `used_bytes`, `remaining_bytes`, `request_count`

**API Key Management:**
- Generate: `POST https://gateway.iploop.io:9443/api/keys/generate`
- List: `GET https://gateway.iploop.io:9443/api/keys`
- Revoke: `DELETE https://gateway.iploop.io:9443/api/keys/{id}`

**Earnings (Supply side):**
- `GET https://gateway.iploop.io:9443/api/credits?token=NODE_TOKEN`
- Returns: `earned_gb`, `withdrawable_usd`, `node_uptime`

---

## Task 2: iploop.io — Add SDK Install Section

Add a prominent "Get Started" section above the fold or right after hero:

```html
<section id="get-started">
  <h2>Get Started in 30 Seconds</h2>
  
  <div class="tabs">
    <tab label="Python">
      <code>pip install iploop</code>
      <pre>
from iploop import IPLoop
client = IPLoop("your-api-key", country="US")
resp = client.get("https://example.com")
      </pre>
    </tab>
    
    <tab label="Node.js">
      <code>npm install iploop</code>
      <pre>
import { IPLoopClient } from 'iploop';
const client = new IPLoopClient({ apiKey: 'your-api-key' });
const resp = await client.get('https://example.com');
      </pre>
    </tab>
    
    <tab label="curl">
      <pre>
curl -x "http://user:API_KEY@gateway.iploop.io:8880" https://example.com
      </pre>
    </tab>
  </div>
  
  <div class="links">
    <a href="https://pypi.org/project/iploop/">PyPI</a>
    <a href="https://www.npmjs.com/package/iploop">npm</a>
    <a href="https://github.com/Iploop">GitHub</a>
  </div>
</section>
```

### Two-Sided Messaging on iploop.io

Add section making it crystal clear:

```
🖥️ USE PROXIES                    📱 EARN REWARDS
─────────────────                  ─────────────────
pip install iploop                 Android SDK
npm install iploop                 Docker Node
192+ countries                     Share bandwidth
Unblock any site                   Earn per GB
Auto IP rotation                   Passive income
$0.50/GB starting                  $0.10+/GB earned
```

---

## Task 3: ProxyClaw Landing — Fix DNS

**proxyclaw.ai** is not resolving. Options:

1. **Set DNS A record** → point to Hostinger/Cloudflare IP
2. **Or redirect** → `proxyclaw.ai` → `iploop.io/proxyclaw`

If using Cloudflare:
```
Type: A
Name: @
Value: <server IP>
Proxy: ON
```

---

## Task 4: Connect Stripe Billing

**Test keys ready:**
- Publishable: `pk_test_51Sx6WYCqi59cXfGO...`
- Secret: `sk_test_51Sx6WYCqi59cXfGO...`

### Plans to create in Stripe:

| Plan | Price | Data | Stripe Price ID |
|------|-------|------|-----------------|
| Free | $0 | 0.5 GB | — |
| Starter | $10/mo | 10 GB | TBD |
| Growth | $40/mo | 50 GB | TBD |
| Business | $120/mo | 200 GB | TBD |
| Enterprise | Custom | 1 TB+ | Contact |

### Dashboard billing page should:
1. Show current plan + usage
2. Upgrade/downgrade buttons → Stripe Checkout
3. Payment history
4. Invoice downloads

---

## Task 5: Consistent Branding Across All Pages

### Colors
- Primary: IPLoop blue
- Accent: ProxyClaw green (for earn/supply section)

### Must appear on EVERY page:
- `pip install iploop` / `npm install iploop` (demand)
- "Earn rewards" / "Share bandwidth" (supply)
- Link to GitHub: github.com/Iploop
- Link to PyPI + npm

### Footer (all pages):
```
IPLoop — Two-Sided Residential Proxy Network
🖥️ Demand: pip install iploop | npm install iploop
📱 Supply: Android SDK | Docker Node
GitHub | PyPI | npm | Docker Hub | Discord
© 2026 IPLoop. All rights reserved.
```

---

## Priority Order

1. **Dashboard endpoints** (breaking — old ones don't work) → ASAP
2. **iploop.io SDK section** → Today
3. **proxyclaw.ai DNS** → Today
4. **Dashboard ↔ API connection** → This week
5. **Stripe billing** → This week

---

## Live Endpoints Reference

| Service | URL | Auth |
|---------|-----|------|
| Proxy (HTTP) | `gateway.iploop.io:8880` | API key in proxy auth |
| Proxy (SOCKS5) | `gateway.iploop.io:1080` | API key in proxy auth |
| API | `https://gateway.iploop.io:9443/api/` | Bearer token |
| WebSocket (nodes) | `wss://gateway.iploop.io:9443/ws` | Node token |
| Dashboard | `https://gateway.iploop.io` | Session |

---

## SDK Links

| SDK | Install | Package | Source |
|-----|---------|---------|--------|
| Python | `pip install iploop` | [pypi.org/project/iploop](https://pypi.org/project/iploop/) | [github.com/Iploop/iploop-python](https://github.com/Iploop/iploop-python) |
| Node.js | `npm install iploop` | [npmjs.com/package/iploop](https://www.npmjs.com/package/iploop) | [github.com/Iploop/iploop-node-sdk](https://github.com/Iploop/iploop-node-sdk) |
| Android | JAR/AAR | — | [github.com/Iploop/iploop-node](https://github.com/Iploop/iploop-node) |
| Docker | `docker pull ultronloop2026/iploop-node` | [Docker Hub](https://hub.docker.com/r/ultronloop2026/iploop-node) | — |
