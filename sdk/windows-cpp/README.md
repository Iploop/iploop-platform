# IPLoop SDK for Windows (C++)

**Production-quality C++17 SDK for IPLoop proxy/tunneling services**

Version: 2.0  
Platform: Windows x64 only  
Compilers: Visual Studio (MSVC) 2019+ and MinGW-w64

## Features

- 🔌 **WebSocket Connection**: Secure connection to `wss://gateway.iploop.io:9443/ws`
- 🔒 **SSL/TLS Support**: Built-in SChannel (no OpenSSL dependency)
- 🚇 **TCP Tunneling**: Full tunnel management with binary protocol
- 🌐 **HTTP Proxy**: Handle proxy requests through IPLoop nodes
- 📊 **IP Info Reporting**: Automatic IP geolocation with caching
- 🔄 **Auto-Reconnect**: Exponential backoff with cooldown handling
- 🧵 **Thread Pool**: Efficient tunnel handling using std::thread
- 💾 **Registry Cache**: Windows registry for persistent data
- 🎯 **Static API**: Matches Android Java SDK exactly

## Protocol Compatibility

✅ **Exact protocol match with Android Java SDK v2.0:**
- Hello messages with device info
- Keepalive every 55 seconds  
- Binary tunnel protocol: `[36 bytes tunnel_id][1 byte flags][N bytes data]`
- JSON text messages for control
- HTTP proxy request handling
- IP info caching (1-hour cooldown)
- Server cooldown handling

## Requirements

### System Requirements
- **OS**: Windows 7+ (x64 only)
- **RAM**: 256 MB minimum
- **Disk**: 50 MB for SDK
- **Network**: Internet connection

### Build Requirements
- **CMake**: 3.16 or newer
- **Compiler**: One of:
  - Visual Studio 2019+ (Community/Professional/Enterprise)
  - MinGW-w64 (GCC 9.0+)

### Runtime Dependencies
- **Windows Libraries** (built-in):
  - WinSock2 (`ws2_32.dll`)
  - WinINet (`wininet.dll`) 
  - SChannel (`secur32.dll`)
  - Registry API (`advapi32.dll`)

**No external dependencies** - everything uses Windows built-in libraries.

## Quick Start

### 1. Build the SDK

**Visual Studio:**
```cmd
# Open x64 Native Tools Command Prompt
git clone <repo>
cd iploop-platform/sdk/windows-cpp
cmake -B build -A x64
cmake --build build --config Release
```

**MinGW-w64:**
```cmd
# Open MinGW-w64 terminal
git clone <repo>
cd iploop-platform/sdk/windows-cpp
cmake -B build -G "MinGW Makefiles" -DCMAKE_BUILD_TYPE=Release
cmake --build build
```

### 2. Run Example

```cmd
# Run with default server
build/Release/iploop_example.exe

# Run with custom server  
build/Release/iploop_example.exe wss://your-server.com:9443/ws
```

### 3. Use in Your Project

**Include the header:**
```cpp
#include "iploop_sdk.h"
```

**Link the library:**
```cpp
// Link: iploop_sdk.lib ws2_32.lib wininet.lib secur32.lib advapi32.lib
```

**Basic usage:**
```cpp
#include "iploop_sdk.h"
#include <iostream>
#include <thread>

int main() {
    // Initialize
    IPLoopSDK::init(); // Uses default server
    // IPLoopSDK::init("wss://custom-server.com:9443/ws");
    
    std::cout << "Node ID: " << IPLoopSDK::getNodeId() << std::endl;
    
    // Start
    IPLoopSDK::start();
    
    // Monitor status
    while (true) {
        std::this_thread::sleep_for(std::chrono::seconds(10));
        
        std::cout << "Connected: " << IPLoopSDK::isConnected() << std::endl;
        std::cout << "Tunnels: " << IPLoopSDK::getActiveTunnelCount() << std::endl;
        
        auto stats = IPLoopSDK::getConnectionStats();
        std::cout << "Connections: " << stats.first << std::endl;
    }
    
    // Stop
    IPLoopSDK::stop();
    return 0;
}
```

## API Reference

### Core Functions

```cpp
class IPLoopSDK {
public:
    // Initialization
    static void init();
    static void init(const std::string& serverUrl);
    
    // Control
    static void start();
    static void stop();
    
    // Status
    static bool isConnected();
    static bool isRunning();
    
    // Information
    static std::string getNodeId();
    static int getActiveTunnelCount();
    static std::string getVersion();
    static std::pair<long long, long long> getConnectionStats();
    
    // Configuration
    static void setLogLevel(int level); // 0=none, 1=error, 2=info, 3=debug
};
```

### Device ID

The SDK automatically generates a unique device ID using:
1. **Windows Machine GUID** from registry (`HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid`)
2. **Fallback**: `unknown-<timestamp>` if GUID unavailable

### Caching

Data is cached in Windows Registry under:
```
HKCU\SOFTWARE\IPLoop\SDK\
├── cached_ip          (string)
├── cached_ip_info     (string)  
└── last_ip_check      (QWORD)
```

## Build Configurations

### Visual Studio (Recommended)

```cmd
# Debug build
cmake -B build -A x64 -DCMAKE_BUILD_TYPE=Debug
cmake --build build --config Debug

# Release build (optimized)
cmake -B build -A x64 -DCMAKE_BUILD_TYPE=Release  
cmake --build build --config Release
```

**Generated files:**
- `build/Release/iploop_sdk.lib` (static library)
- `build/Release/iploop_sdk.dll` (shared library)
- `build/Release/iploop_example.exe` (example)

### MinGW-w64

```cmd
# Release build
cmake -B build -G "MinGW Makefiles" -DCMAKE_BUILD_TYPE=Release
cmake --build build

# Debug build
cmake -B build -G "MinGW Makefiles" -DCMAKE_BUILD_TYPE=Debug
cmake --build build
```

**Generated files:**
- `build/libiploop_sdk.a` (static library)
- `build/libiploop_sdk.dll` (shared library)
- `build/iploop_example.exe` (example)

## Integration Examples

### CMake Integration

```cmake
# Add SDK to your CMakeLists.txt
add_subdirectory(path/to/iploop-sdk)
target_link_libraries(your_app iploop_sdk_static)
```

### Visual Studio Project

1. **Add include directory**: `path\to\sdk\include`
2. **Add library directory**: `path\to\sdk\build\Release`
3. **Link libraries**: `iploop_sdk.lib;ws2_32.lib;wininet.lib;secur32.lib;advapi32.lib`

### Code::Blocks / Dev-C++

1. **Compiler flags**: `-std=c++17 -DWIN32_LEAN_AND_MEAN`
2. **Include paths**: `path/to/sdk/include`
3. **Library paths**: `path/to/sdk/build`
4. **Link libraries**: `-liploop_sdk -lws2_32 -lwininet -lsecur32 -ladvapi32`

## Troubleshooting

### Build Issues

**"CMake not found":**
- Install CMake from https://cmake.org/download/
- Add to PATH: `C:\Program Files\CMake\bin`

**"Compiler not found":**
- **Visual Studio**: Install "Desktop development with C++" workload
- **MinGW-w64**: Download from https://www.mingw-w64.org/downloads/

**"Architecture mismatch":**
- Use x64 tools only: `x64 Native Tools Command Prompt`
- For MinGW: ensure 64-bit version (`x86_64-*-mingw32`)

### Runtime Issues

**"DLL not found":**
- Use static library (`iploop_sdk_static`) to avoid DLL dependencies
- Or copy `iploop_sdk.dll` to your executable directory

**"Connection failed":**
- Check Windows Firewall settings
- Verify internet connectivity
- Test with: `telnet gateway.iploop.io 9443`

**"Access denied (registry)":**
- Run as Administrator once to create registry keys
- Or use portable mode (disable caching)

### Performance Tuning

**Memory usage:**
- Default: ~10 MB base + ~100 KB per tunnel
- Reduce with: `IPLoopSDK::setLogLevel(0)` (disable logging)

**CPU usage:**
- Normal: ~1-5% idle, ~10-20% under load
- Optimize: Build with `-O2` (Release mode)

**Network usage:**
- Keepalive: ~50 bytes every 55 seconds
- IP check: ~5 KB every hour (cached)
- Tunnel data: No overhead (direct forwarding)

## Architecture

```
┌─────────────────────────────────────────────┐
│                IPLoopSDK                    │
│            (Public API)                     │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│             SDK Core                        │
│  ┌─────────────┐ ┌──────────────────────┐   │
│  │ Connection  │ │    Message Handler   │   │
│  │   Manager   │ │                      │   │
│  └─────────────┘ └──────────────────────┘   │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│               Components                    │
│ ┌─────────────┐ ┌─────────────┐ ┌─────────┐ │
│ │  WebSocket  │ │   Tunnel    │ │  Proxy  │ │
│ │   Client    │ │   Manager   │ │ Handler │ │
│ └─────────────┘ └─────────────┘ └─────────┘ │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│              Windows APIs                   │
│  WinSock2 │ WinINet │ SChannel │ Registry   │
└─────────────────────────────────────────────┘
```

## Protocol Details

### WebSocket Connection
- **URL**: `wss://gateway.iploop.io:9443/ws`
- **Protocol**: WebSocket over SSL/TLS
- **Handshake**: Standard RFC 6455
- **SSL**: SChannel (Windows native)

### Message Types

**Hello** (SDK → Server):
```json
{
  "type": "hello",
  "node_id": "machine-guid",
  "device_model": "CPU Info (x64)",
  "sdk_version": "2.0"
}
```

**Keepalive** (SDK → Server, every 55s):
```json
{
  "type": "keepalive", 
  "uptime_sec": 3600,
  "active_tunnels": 5
}
```

**Tunnel Open** (Server → SDK):
```json
{
  "type": "tunnel_open",
  "data": {
    "tunnel_id": "550e8400-e29b-41d4-a716-446655440000",
    "host": "example.com",
    "port": "443"
  }
}
```

**Binary Tunnel Data** (bidirectional):
```
[36 bytes: tunnel_id padded with spaces]
[1 byte: flags - 0x00=data, 0x01=EOF]  
[N bytes: raw data]
```

### Reconnection Logic

1. **Immediate**: First failure
2. **Exponential backoff**: 2s, 4s, 8s, 16s, ..., max 5 minutes
3. **Cooldown**: Server can request specific delay
4. **Never give up**: Keeps retrying indefinitely

## License

Copyright (c) 2024 IPLoop. All rights reserved.

This SDK is proprietary software. Contact IPLoop for licensing terms.

## Support

- **Documentation**: This README
- **Issues**: Contact IPLoop support
- **Updates**: Check IPLoop dashboard

---

**IPLoop SDK for Windows v2.0** - Production ready ✅