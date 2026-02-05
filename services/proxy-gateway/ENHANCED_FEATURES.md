# Enhanced IPLoop Proxy Gateway v2.0

🚀 **Enterprise-Grade Proxy Features for Partners**

## 🔥 New Features Overview

### **Authentication & Security**
- **Multiple Auth Methods**: Basic, Token, IP Whitelist, HMAC Signature
- **Parameter-Rich Auth**: Country, city, ASN targeting via credentials
- **Session Management**: Sticky sessions with custom lifetimes
- **Advanced Rate Limiting**: Per-customer, per-endpoint controls

### **Geographic Targeting**
- **Country Selection**: `-country-US`, `-geo-DE` parameters
- **City Targeting**: `-city-Miami`, `-city-London` precision
- **ASN Targeting**: `-asn-12345` for specific ISP networks
- **Regional Optimization**: Automatic routing to best nodes

### **Session Control**
- **Sticky Sessions**: 10-60 minute IP persistence for logins
- **Rotation Modes**: Per-request, timed, manual, IP-change triggers
- **Session IDs**: Custom session naming and grouping
- **Lifetime Control**: `-lifetime-30m`, `-ttl-2h` parameters

### **Protocol Support**
- **Enhanced SOCKS5**: Full TCP/UDP support with session management
- **HTTP/HTTPS Proxy**: Advanced header manipulation
- **WebSocket Routing**: Real-time connection management
- **Multiple Endpoints**: Different ports for different protocols

### **Browser Fingerprinting**
- **Profile System**: Chrome, Firefox, Safari, Mobile presets
- **Header Rotation**: Realistic User-Agent and header combinations
- **Geographic Headers**: Language/encoding matching by country
- **Fingerprint Resistance**: Randomized optional headers

### **Analytics & Monitoring**
- **Real-time Metrics**: Live performance dashboards
- **Usage Analytics**: Detailed bandwidth and request tracking
- **Geographic Stats**: Top countries, cities, destinations
- **Error Analysis**: Failure categorization and trending
- **Partner Portal**: Self-service analytics and controls

## 🛠️ Authentication Methods

### **1. Basic Authentication (Enhanced)**
```
Format: user:password-params
Example: customer123:api_key_here-country-US-city-Miami-session-sticky30m
```

**Available Parameters:**
- `country-XX` - Target country (US, DE, FR, etc.)
- `city-NAME` - Target city (Miami, London, Tokyo)
- `asn-12345` - Target ASN/ISP
- `session-ID` - Custom session identifier
- `sesstype-TYPE` - Session type: sticky, rotating, per-request
- `lifetime-TIME` - Session lifetime: 30m, 1h, 120s
- `rotate-MODE` - Rotation: request, time, manual, ip-change
- `rotateint-TIME` - Rotation interval: 5m, 30s
- `profile-NAME` - Browser profile: chrome-win, firefox-mac, mobile-ios
- `ua-STRING` - Custom User-Agent
- `speed-MBPS` - Minimum speed requirement
- `latency-MS` - Maximum latency requirement
- `debug-1` - Enable debug mode

### **2. Token Authentication**
```
Format: token:TOKEN-params
Example: token:abc123xyz789-country-US-session-sticky-lifetime-60m
```

### **3. IP Whitelist**
```
Format: ip:USER_ID
Example: ip:customer123
Note: Client IP must be pre-whitelisted
```

### **4. HMAC Signature**
```
Format: sig:USER_TIMESTAMP_SIGNATURE-params  
Example: sig:user123_1703275200_d41d8cd98f00b204e9800998ecf8427e-country-DE
Signature: HMAC-SHA256(userID + timestamp, secret_key)
```

## 🌍 Geographic Targeting Examples

### **Country-Level Targeting**
```bash
# US proxies
curl -x user:pass-country-US@proxy.iploop.com:8080 https://httpbin.org/ip

# German proxies  
curl -x user:pass-geo-DE@proxy.iploop.com:8080 https://httpbin.org/ip

# Multiple countries (will rotate)
curl -x user:pass-country-US,DE,FR@proxy.iploop.com:8080 https://httpbin.org/ip
```

### **City-Level Precision**
```bash
# New York, US
curl -x user:pass-country-US-city-newyork@proxy.iploop.com:8080 https://httpbin.org/ip

# London, UK  
curl -x user:pass-geo-GB-city-london@proxy.iploop.com:8080 https://httpbin.org/ip

# Tokyo, Japan
curl -x user:pass-country-JP-city-tokyo@proxy.iploop.com:8080 https://httpbin.org/ip
```

### **ASN/ISP Targeting**
```bash
# Target specific ISP (Comcast)
curl -x user:pass-asn-7922@proxy.iploop.com:8080 https://httpbin.org/ip

# Verizon network
curl -x user:pass-country-US-asn-701@proxy.iploop.com:8080 https://httpbin.org/ip
```

## 🔄 Session Management Examples

### **Sticky Sessions**
```bash
# 30-minute sticky session
curl -x user:pass-session-login123-lifetime-30m@proxy.iploop.com:8080 https://httpbin.org/ip

# 2-hour sticky session for account work
curl -x user:pass-sesstype-sticky-lifetime-2h-session-account_work@proxy.iploop.com:8080 https://httpbin.org/ip
```

### **Rotation Control**
```bash
# Rotate every 5 minutes
curl -x user:pass-rotate-time-rotateint-5m@proxy.iploop.com:8080 https://httpbin.org/ip

# Rotate on every request
curl -x user:pass-rotate-request@proxy.iploop.com:8080 https://httpbin.org/ip

# Manual rotation only
curl -x user:pass-rotate-manual-session-stable123@proxy.iploop.com:8080 https://httpbin.org/ip
```

## 🎭 Browser Profiles & Headers

### **Profile Examples**
```bash
# Chrome Windows profile
curl -x user:pass-profile-chrome-win@proxy.iploop.com:8080 https://httpbin.org/headers

# Mobile iOS profile  
curl -x user:pass-profile-mobile-ios@proxy.iploop.com:8080 https://httpbin.org/headers

# Firefox with German locale
curl -x user:pass-profile-firefox-win-country-DE@proxy.iploop.com:8080 https://httpbin.org/headers
```

### **Custom Headers**
```bash
# Custom User-Agent
curl -x user:pass-ua-"CustomBot/1.0"@proxy.iploop.com:8080 https://httpbin.org/headers

# Custom headers via API
POST /api/v1/sessions
{
  "customer_id": "customer123",
  "country": "US", 
  "headers": {
    "X-Custom-Header": "MyValue",
    "Accept-Language": "en-US,es;q=0.8"
  }
}
```

## 🔌 SOCKS5 Enhanced Usage

### **Basic SOCKS5**
```bash
# Standard SOCKS5 proxy
curl --socks5 user:pass@proxy.iploop.com:1080 https://httpbin.org/ip

# With geographic targeting
curl --socks5 user:pass-country-FR@proxy.iploop.com:1080 https://httpbin.org/ip
```

### **Advanced SOCKS5 Parameters**
```python
import requests

proxies = {
    'http': 'socks5://user:pass-country-US-city-Miami-session-scraper123@proxy.iploop.com:1080',
    'https': 'socks5://user:pass-country-US-city-Miami-session-scraper123@proxy.iploop.com:1080'
}

response = requests.get('https://httpbin.org/ip', proxies=proxies)
print(response.json())
```

## 📊 Analytics & Monitoring

### **Real-time Metrics API**
```bash
# Get customer metrics
GET /api/v1/analytics/metrics?customer_id=customer123

# Hourly report (last 24 hours)  
GET /api/v1/analytics/hourly?customer_id=customer123&hours=24

# Top destinations
GET /api/v1/analytics/destinations?customer_id=customer123&limit=10

# System-wide stats
GET /api/v1/analytics/system
```

### **Session Management API**
```bash
# List active sessions
GET /api/v1/sessions?customer_id=customer123

# Get session details
GET /api/v1/sessions/session123

# Force rotation
POST /api/v1/sessions/session123/rotate

# Terminate session
DELETE /api/v1/sessions/session123
```

## 🏗️ Partner Integration Examples

### **Web Scraping Setup**
```python
import requests
from requests.adapters import HTTPAdapter

class IPLoopSession(requests.Session):
    def __init__(self, customer_id, api_key, country="US", session_type="rotating"):
        super().__init__()
        
        # Configure proxy with parameters
        proxy_auth = f"{customer_id}:{api_key}-country-{country}-sesstype-{session_type}-rotate-request"
        self.proxies = {
            'http': f'http://{proxy_auth}@proxy.iploop.com:8080',
            'https': f'http://{proxy_auth}@proxy.iploop.com:8080'
        }
        
        # Set realistic headers
        self.headers.update({
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
        })

# Usage
scraper = IPLoopSession("customer123", "api_key", country="US")
response = scraper.get("https://example.com")
```

### **Account Management Setup**  
```python
class AccountManager:
    def __init__(self, customer_id, api_key):
        self.session = requests.Session()
        
        # Sticky session for 1 hour
        proxy_auth = f"{customer_id}:{api_key}-sesstype-sticky-lifetime-1h-session-account_{customer_id}"
        self.session.proxies = {
            'http': f'http://{proxy_auth}@proxy.iploop.com:8080',
            'https': f'http://{proxy_auth}@proxy.iploop.com:8080'
        }
    
    def login(self, username, password):
        # Login will maintain same IP for entire session
        return self.session.post("https://site.com/login", data={
            'username': username, 'password': password
        })
    
    def get_profile(self):
        # Same IP as login
        return self.session.get("https://site.com/profile")
```

### **Multi-Location Testing**
```python
locations = [
    ("US", "newyork"),
    ("GB", "london"), 
    ("DE", "berlin"),
    ("JP", "tokyo")
]

for country, city in locations:
    proxy_auth = f"customer123:api_key-country-{country}-city-{city}"
    proxies = {'http': f'http://{proxy_auth}@proxy.iploop.com:8080'}
    
    response = requests.get("https://httpbin.org/ip", proxies=proxies)
    print(f"{country}/{city}: {response.json()['origin']}")
```

## 📈 Performance Optimization

### **Speed Requirements**
```bash
# Require minimum 50 Mbps nodes
curl -x user:pass-speed-50-country-US@proxy.iploop.com:8080 https://httpbin.org/ip

# Maximum 200ms latency
curl -x user:pass-latency-200-country-US@proxy.iploop.com:8080 https://httpbin.org/ip
```

### **Load Balancing**
```bash
# Multiple endpoints for high availability
ENDPOINTS=(
  "proxy1.iploop.com:8080"
  "proxy2.iploop.com:8080"  
  "proxy3.iploop.com:8080"
)

# Round-robin through endpoints
for endpoint in "${ENDPOINTS[@]}"; do
  curl -x user:pass@$endpoint https://httpbin.org/ip
done
```

## 🔒 Security Features

### **IP Whitelisting**
```bash
# Pre-configure allowed IPs
POST /api/v1/customers/customer123/whitelist
{
  "ip_addresses": ["203.0.113.1", "203.0.113.0/24"],
  "description": "Office network"
}

# Use without credentials (IP-based auth)
curl -x proxy.iploop.com:8080 https://httpbin.org/ip
```

### **Token-Based Auth**
```bash
# Generate token
POST /api/v1/tokens
{
  "customer_id": "customer123",
  "permissions": ["proxy", "analytics"],
  "expires_in": "30d"
}

# Use token
curl -x token:abc123xyz@proxy.iploop.com:8080 https://httpbin.org/ip
```

## 🚀 Getting Started

### **1. Contact for Enterprise Access**
- Email: enterprise@iploop.com
- Request proxy partner account
- Receive customer ID and API credentials

### **2. Test Basic Connectivity**
```bash
curl -x customer_id:api_key@proxy.iploop.com:8080 https://httpbin.org/ip
```

### **3. Explore Geographic Targeting**
```bash
curl -x customer_id:api_key-country-US@proxy.iploop.com:8080 https://httpbin.org/ip
curl -x customer_id:api_key-country-DE@proxy.iploop.com:8080 https://httpbin.org/ip
```

### **4. Set Up Sticky Sessions**  
```bash
curl -x customer_id:api_key-sesstype-sticky-lifetime-30m@proxy.iploop.com:8080 https://httpbin.org/ip
```

### **5. Monitor via API**
```bash
curl "https://proxy.iploop.com/api/v1/analytics/metrics?customer_id=your_id"
```

## 📋 Feature Comparison

| Feature | Basic | Enterprise |
|---------|--------|-------------|
| HTTP/HTTPS Proxy | ✅ | ✅ |
| SOCKS5 Proxy | ❌ | ✅ |
| Country Targeting | ✅ | ✅ |
| City Targeting | ❌ | ✅ |
| ASN Targeting | ❌ | ✅ |
| Sticky Sessions | ❌ | ✅ |
| Custom Rotation | ❌ | ✅ |
| Browser Profiles | ❌ | ✅ |
| Header Manipulation | ❌ | ✅ |
| Real-time Analytics | ❌ | ✅ |
| API Management | ❌ | ✅ |
| Multiple Auth Methods | ❌ | ✅ |
| Performance Guarantees | ❌ | ✅ |

## 🆘 Support

- **Documentation**: https://docs.iploop.com/enterprise
- **API Reference**: https://docs.iploop.com/api
- **Status Page**: https://status.iploop.com  
- **Support**: support@iploop.com
- **Enterprise Sales**: enterprise@iploop.com