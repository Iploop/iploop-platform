# b64-stable — Base64 Text Protocol (Stable)

**Date:** 2026-02-13
**SDK Version:** stability-test-2.0

## What works
- WebSocket connection with TLS, ping/pong, auto-reconnect
- Proxy requests (SynchronousQueue + AbortPolicy, pool size 8-32)
- Tunnel support (CONNECT proxy on port 8880)
- IP info caching (SharedPreferences)
- Server-side retry on "node busy" (up to 3 nodes)
- 2MB max message size

## Performance
- 10 parallel proxy: 10/10, 1.5s wall
- 20 parallel proxy: 19/20, 15.4s wall
- 5 parallel tunnels: 5/5
- curl -x proxy latency: 700ms-1.4s per request

## Protocol
- All messages: JSON text WebSocket frames
- Tunnel data: base64-encoded in JSON (`{"type":"tunnel_data","data":{"tunnel_id":"xxx","data":"<base64>"}}`)
- ~33% overhead from base64 encoding

## Files
- `StabilityTestSDK.java` — Android SDK source
- `main.go` — Server source
- `stability-sdk-2.0-bundle.jar` — Compiled SDK (DEX)
- `ws-stability-server-tls` — Compiled server binary (linux/amd64)
