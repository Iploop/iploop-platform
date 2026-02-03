# IPLoop Node SDK

SDK for devices to join the IPLoop network and share bandwidth.

## Platforms

### Android (Ready)
Full-featured Android app that runs as a foreground service.

**Features:**
- 🔌 Persistent WebSocket connection to gateway
- 📊 Real-time stats tracking
- 💰 Earnings display
- 🔄 Auto-start on boot
- 🔋 Battery-optimized

**Building:**
```bash
cd android
./gradlew assembleRelease
```

### iOS (Ready)
SwiftUI app with background task support.

**Features:**
- 🔌 WebSocket connection to gateway
- 📊 Real-time stats tracking
- 💰 Earnings display
- 🔋 Background refresh support

**Building:**
```bash
cd ios/IPLoopNode
swift build
# Or open in Xcode
```

**Note:** iOS has limitations for background networking. App works best in foreground or with Background App Refresh enabled.

### Windows (Planned)
System tray application with service component.

### macOS (Planned)
Menu bar application with service component.

## Architecture

```
┌─────────────────┐      ┌─────────────────┐
│   Node Device   │      │  IPLoop Gateway │
│                 │      │                 │
│  ┌───────────┐  │ WSS  │  ┌───────────┐  │
│  │  Node SDK │◄─┼──────┼──┤  Gateway  │  │
│  └───────────┘  │      │  └───────────┘  │
│        │        │      │        │        │
│        ▼        │      │        ▼        │
│  ┌───────────┐  │      │  ┌───────────┐  │
│  │  Execute  │  │      │  │  Customer │  │
│  │  Request  │  │      │  │  Requests │  │
│  └───────────┘  │      │  └───────────┘  │
└─────────────────┘      └─────────────────┘
```

## Protocol

### Connection
1. Node connects to `wss://gateway.iploop.io/node/connect`
2. Authenticates with `X-Node-Id` and `Authorization` headers
3. Sends capabilities message
4. Receives proxy requests

### Messages

**Node → Gateway:**
- `capabilities` - Node features (protocols, max concurrent)
- `heartbeat` - Status update (bytes, requests, uptime)
- `proxy_response` - Response to proxy request
- `pong` - Response to ping

**Gateway → Node:**
- `proxy_request` - HTTP request to execute
- `config_update` - Node configuration changes
- `ping` - Keep-alive

### Proxy Request Format
```json
{
  "type": "proxy_request",
  "requestId": "uuid",
  "payload": {
    "method": "GET",
    "url": "https://example.com",
    "headers": {},
    "body": "base64..."
  }
}
```

### Proxy Response Format
```json
{
  "type": "proxy_response",
  "requestId": "uuid",
  "payload": {
    "statusCode": 200,
    "headers": {},
    "body": "base64...",
    "error": null
  }
}
```

## Earnings Model

Nodes earn based on:
- Bytes transferred (both directions)
- Request count
- Uptime
- Geographic location (premium locations earn more)
- Connection quality (latency, success rate)

Minimum withdrawal: $10
Payout methods: PayPal, Crypto (USDT), Bank Transfer
