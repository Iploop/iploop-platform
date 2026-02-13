# IPLoop Node Server v2.0

**Release Date:** 2026-02-13
**Status:** Production ✅

## Overview

Production rename of the ws-stability-test into the official IPLoop Node Server. This is the same codebase with cleaned-up naming and backward compatibility.

## Components

| File | Description |
|------|-------------|
| `iploop-node-server` | Server binary (Linux amd64) |
| `iploop-sdk-2.0-bundle.jar` | Android SDK JAR (dexed bundle, min SDK 22) |
| `IPLoopSDK.java` | SDK source code |

## Changes from stability-test

### Naming
- Module: `ws-stability-test` → `iploop-node-server`
- SDK class: `StabilityTestSDK` → `IPLoopSDK`
- SDK package: `com.iploop.stabilitytest` → `com.iploop.sdk`
- SDK version string: `stability-test-2.0` → `2.0`
- DB path: `/root/stability-data.db` → `/root/iploop-node-server.db`
- Log prefix: "WS Stability+Hub Server" → "IPLoop Node Server"
- Systemd service: `ws-stability-server` → `iploop-node-server`

### Backward Compatibility
- Server accepts **both** `"2.0"` and `"stability-test-2.0"` SDK versions in `GetTunnelNode()`
- Existing nodes running old SDK versions will continue to work during transition
- Same ports: 9443 (WSS) + 8880 (CONNECT proxy)

## Deployment

### Gateway (159.65.95.169)
```bash
# Binary location
/root/iploop-node-server

# Systemd service
/etc/systemd/system/iploop-node-server.service

# Environment
PORT=9443
TLS_CERT=/etc/letsencrypt/live/gateway.iploop.io/fullchain.pem
TLS_KEY=/etc/letsencrypt/live/gateway.iploop.io/privkey.pem
GOMEMLIMIT=1500MiB

# Commands
systemctl status iploop-node-server
systemctl restart iploop-node-server
curl -sk https://localhost:9443/health
```

### SDK Integration
```java
import com.iploop.sdk.IPLoopSDK;

IPLoopSDK.init(context);
IPLoopSDK.start();
// ...
IPLoopSDK.stop();
```

## Verification
- Health: `curl -sk https://gateway.iploop.io:9443/health`
- Stats: `curl -sk https://gateway.iploop.io:9443/stats`
- Nodes: `curl -sk https://gateway.iploop.io:9443/api/nodes`
