# IPLoop macOS SDK

Enable your macOS application to participate in the IPLoop residential proxy network.

## Requirements

- macOS 11.0+
- Swift 5.7+
- Xcode 14+

## Installation

### Swift Package Manager

```swift
dependencies: [
    .package(url: "https://github.com/iploop/macos-sdk.git", from: "1.0.0")
]
```

### Build from source

```bash
swift build -c release
```

## Quick Start

### As a Library

```swift
import IPLoopSDK

// Initialize
IPLoopSDK.shared.initialize(apiKey: "your_api_key")

// Request consent
IPLoopSDK.shared.setUserConsent(true)

// Start
Task {
    try await IPLoopSDK.shared.start()
}

// Monitor status
IPLoopSDK.shared.onStatusChange = { status in
    print("Status: \(status)")
}

// Stop
IPLoopSDK.shared.stop()
```

### As a Daemon

```bash
# Build
swift build -c release

# Run
IPLOOP_API_KEY=your_key .build/release/iploop-daemon
```

### Run as LaunchDaemon

Create `/Library/LaunchDaemons/io.iploop.daemon.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.iploop.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/iploop-daemon</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>IPLOOP_API_KEY</key>
        <string>your_api_key</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

Then:
```bash
sudo launchctl load /Library/LaunchDaemons/io.iploop.daemon.plist
```

## API Reference

### Initialize
```swift
IPLoopSDK.shared.initialize(apiKey: String)
```

### Start/Stop
```swift
try await IPLoopSDK.shared.start()
IPLoopSDK.shared.stop()
```

### Status
```swift
IPLoopSDK.shared.isActive // Bool
IPLoopSDK.shared.onStatusChange = { status in ... }
```

### Consent
```swift
IPLoopSDK.shared.setUserConsent(true)
IPLoopSDK.shared.hasUserConsent() // Bool
```

### Stats
```swift
let stats = IPLoopSDK.shared.getBandwidthStats()
print(stats.totalMB) // MB transferred
print(stats.totalRequests) // Request count
```

## Support

- Email: support@iploop.io
- Docs: https://docs.iploop.io
