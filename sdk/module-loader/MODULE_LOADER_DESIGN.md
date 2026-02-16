# SDK Module Loader — Design

## Problem
Different partner SDKs have completely different interfaces:
- **Type A (Static):** `SomeSDK.init(context, deviceId, "key", true)`
- **Type B (Instance):** `sdk = new SomeSDK.Builder(context).setKey("x").build(); sdk.start()`
- **Type C (Simple):** `SomeSDK.start("apiKey")`

We need ONE loader that handles all patterns via JSON config.

## Architecture

```
┌──────────────────────────┐
│     ModuleManager        │  ← Reloads config every 5 min
│  ┌────────────────────┐  │
│  │  modules_config.json│  │  ← Downloaded from server
│  └────────┬───────────┘  │
│           ▼              │
│  ┌────────────────────┐  │
│  │  ModuleLoader      │  │  ← Parses JSON, uses reflection
│  └────────┬───────────┘  │
│           ▼              │
│  ┌────────────────────┐  │
│  │  Module instances  │  │
│  │  ├── SDK A         │  │
│  │  ├── SDK B         │  │
│  │  └── SDK C         │  │
│  └────────────────────┘  │
└──────────────────────────┘
```
