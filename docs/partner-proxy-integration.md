# Residential Proxy — Integration Guide

**Customer:** iploop  
**Service:** Residential Proxy with Peer Selection (US)  
**Protocol:** HTTP/HTTPS CONNECT  
**Port:** 60003  

---

## Overview

The residential proxy service provides access to a large pool of US residential IP addresses. You select which IP (peer) you want by appending a number to your username. Each number maps to a specific residential peer — the same number on the same server will consistently route to the same IP.

The pool contains **2,500 residential peers** per server. Numbers above 2,500 wrap around (modulo 2,500), so peer `2501` maps to the same peer as `1`, peer `2502` = `2`, etc.

---

## Authentication

| Field | Value |
|-------|-------|
| **Username format** | `iploop-{NUMBER}` |
| **Password** | *(provided separately)* |
| **Port** | `60003` |

- `{NUMBER}` — peer selection number (1–2500 for unique peers)

**Peer behavior:**
- Same `server + number` → routes to the **same residential IP**
- Different numbers → different IPs
- Numbers beyond 2500 wrap: `2501` = `1`, `5001` = `1`, etc.
- If a peer is offline, the request may time out — switch to a different number

---

## Quick Start (curl)

### Basic request — get a residential IP via peer #1:

```bash
curl -x "http://iploop-1:PASSWORD@185.156.47.88:60003" \
  https://httpbin.org/ip
```

### Verify peer consistency (same number = same IP):

```bash
# Both return the same IP
curl -x "http://iploop-1:PASSWORD@185.156.47.88:60003" https://httpbin.org/ip
curl -x "http://iploop-1:PASSWORD@185.156.47.88:60003" https://httpbin.org/ip
```

### Get different IPs — change the peer number:

```bash
curl -x "http://iploop-2:PASSWORD@185.156.47.88:60003" https://httpbin.org/ip
curl -x "http://iploop-3:PASSWORD@185.156.47.88:60003" https://httpbin.org/ip
curl -x "http://iploop-100:PASSWORD@185.156.47.88:60003" https://httpbin.org/ip
```

### HTTPS target (recommended):

```bash
curl -x "http://iploop-42:PASSWORD@89.222.109.19:60003" \
  https://api.ipify.org?format=json
```

---

## Available Servers

All 123 servers below support port 60003 with a pool of 2,500 residential peers each.

| # | Server IP | # | Server IP | # | Server IP |
|---|-----------|---|-----------|---|-----------|
| 1 | 23.29.117.114 | 42 | 79.127.231.50 | 83 | 143.244.60.79 |
| 2 | 23.29.120.170 | 43 | 79.127.232.196 | 84 | 143.244.60.104 |
| 3 | 23.29.126.26 | 44 | 79.127.232.224 | 85 | 152.233.22.58 |
| 4 | 23.92.69.210 | 45 | 79.127.250.3 | 86 | 152.233.22.59 |
| 5 | 23.92.70.250 | 46 | 79.127.250.4 | 87 | 152.233.22.66 |
| 6 | 23.111.143.170 | 47 | 79.127.250.5 | 88 | 152.233.23.65 |
| 7 | 23.111.152.174 | 48 | 84.17.41.118 | 89 | 152.233.23.66 |
| 8 | 23.111.153.30 | 49 | 84.17.41.120 | 90 | 152.233.23.67 |
| 9 | 23.111.153.170 | 50 | 89.187.170.185 | 91 | 152.233.23.68 |
| 10 | 23.111.153.230 | 51 | 89.187.170.186 | 92 | 152.233.23.69 |
| 11 | 23.111.154.2 | 52 | 89.187.175.81 | 93 | 152.233.23.70 |
| 12 | 23.111.157.198 | 53 | 89.187.175.119 | 94 | 152.233.23.71 |
| 13 | 23.111.159.206 | 54 | 89.187.175.120 | 95 | 152.233.23.72 |
| 14 | 23.111.159.226 | 55 | 89.187.175.121 | 96 | 152.233.23.73 |
| 15 | 23.111.161.134 | 56 | 89.187.175.122 | 97 | 156.146.38.229 |
| 16 | 23.111.181.130 | 57 | 89.187.175.123 | 98 | 156.146.43.28 |
| 17 | 23.227.174.122 | 58 | 89.187.175.133 | 99 | 162.212.57.238 |
| 18 | 23.227.177.34 | 59 | 89.187.185.93 | 100 | 162.213.193.82 |
| 19 | 23.227.182.10 | 60 | 89.187.185.123 | 101 | 178.249.210.13 |
| 20 | 23.227.182.234 | 61 | 89.187.185.194 | 102 | 178.249.210.26 |
| 21 | 23.227.186.202 | 62 | 89.222.109.19 | 103 | 178.249.210.27 |
| 22 | 23.227.191.18 | 63 | 89.222.109.153 | 104 | 185.156.47.83 |
| 23 | 37.72.172.226 | 64 | 89.222.120.88 | 105 | 185.156.47.84 |
| 24 | 46.21.152.114 | 65 | 89.222.120.106 | 106 | 185.156.47.85 |
| 25 | 66.165.226.242 | 66 | 89.222.120.111 | 107 | 185.156.47.86 |
| 26 | 66.165.231.202 | 67 | 89.222.120.123 | 108 | 185.156.47.87 |
| 27 | 66.165.236.2 | 68 | 94.72.163.78 | 109 | 185.156.47.88 |
| 28 | 66.165.244.6 | 69 | 94.72.166.170 | 110 | 185.156.47.89 |
| 29 | 66.206.17.98 | 70 | 95.173.192.104 | 111 | 190.102.105.62 |
| 30 | 66.206.19.90 | 71 | 95.173.192.105 | 112 | 199.167.144.34 |
| 31 | 66.206.28.42 | 72 | 95.173.216.227 | 113 | 199.231.162.90 |
| 32 | 68.233.238.194 | 73 | 104.156.50.114 | 114 | 209.133.213.162 |
| 33 | 79.127.221.23 | 74 | 104.156.55.218 | 115 | 209.133.213.222 |
| 34 | 79.127.223.2 | 75 | 107.155.67.26 | 116 | 209.133.221.6 |
| 35 | 79.127.223.3 | 76 | 107.155.97.142 | 117 | 209.133.221.214 |
| 36 | 79.127.223.4 | 77 | 107.155.100.194 | 118 | 212.102.58.203 |
| 37 | 79.127.223.5 | 78 | 107.155.127.10 | 119 | 212.102.58.205 |
| 38 | 79.127.223.6 | 79 | 121.127.40.55 | 120 | 212.102.58.210 |
| 39 | 79.127.223.7 | 80 | 121.127.40.56 | 121 | 212.102.58.213 |
| 40 | 79.127.223.8 | 81 | 121.127.44.58 | 122 | 212.102.58.214 |
| 41 | 79.127.223.9 | 82 | 143.244.60.22 | 123 | 143.244.60.28 |

---

## Code Examples

### Python (requests)

```python
import requests

server = "185.156.47.88"
peer_number = 1

proxy_url = f"http://iploop-{peer_number}:PASSWORD@{server}:60003"
proxies = {"http": proxy_url, "https": proxy_url}

response = requests.get("https://httpbin.org/ip", proxies=proxies, timeout=15)
print(response.json())

# Rotate through 10 different IPs:
for n in range(1, 11):
    proxy_url = f"http://iploop-{n}:PASSWORD@{server}:60003"
    proxies = {"http": proxy_url, "https": proxy_url}
    resp = requests.get("https://httpbin.org/ip", proxies=proxies, timeout=15)
    print(f"Peer {n}: {resp.json()['origin']}")
```

### Node.js (axios)

```javascript
const axios = require('axios');
const HttpsProxyAgent = require('https-proxy-agent');

const server = '185.156.47.88';
const peerNumber = 1;

const agent = new HttpsProxyAgent(
  `http://iploop-${peerNumber}:PASSWORD@${server}:60003`
);

axios.get('https://httpbin.org/ip', { httpsAgent: agent })
  .then(res => console.log(res.data))
  .catch(err => console.error(err.message));
```

### Go

```go
package main

import (
    "fmt"
    "io"
    "net/http"
    "net/url"
)

func main() {
    server := "185.156.47.88"
    peerNumber := 1

    proxyURL, _ := url.Parse(fmt.Sprintf(
        "http://iploop-%d:PASSWORD@%s:60003",
        peerNumber, server,
    ))
    client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

    resp, err := client.Get("https://httpbin.org/ip")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    fmt.Println(string(body))
}
```

---

## Best Practices

1. **Distribute across servers** — Spread requests across multiple servers to maximize your available IP pool. Each server has its own set of 2,500 peers, so using 5 servers gives you access to up to 12,500 unique IPs.

2. **Handle timeouts gracefully** — If a peer is offline, the request may time out. Retry with a different peer number or a different server.

3. **Use HTTPS targets** — We recommend targeting HTTPS URLs for best reliability.

4. **Peer number strategy** — Use sequential numbers (1, 2, 3...) for simplicity. Numbers 1–2500 are unique peers per server. Beyond 2500, numbers wrap around.

5. **Timeouts** — Set a 15-second request timeout. If a request times out, try a different peer number or a different server.

6. **Scale with servers** — To get more unique IPs, use the same peer numbers across different servers. Peer #1 on server A is a different IP than peer #1 on server B.

---

## Summary

| Metric | Value |
|--------|-------|
| Servers | 123 |
| Peers per server | 2,500 |
| Total unique IPs | up to ~307,500 |
| Port | 60003 |
| Username format | `iploop-{1-2500}` |

---

## Important Notes

- **RFC 1918 blocking**: Requests to private/internal IP addresses (10.x.x.x, 172.16–31.x.x, 192.168.x.x) are blocked for security — including domains that resolve to private IPs.
- **DNS resolution**: All target domains are resolved before forwarding to protect the network from internal access attempts.

---

## Support

For technical issues or questions, contact your account manager.
