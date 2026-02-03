# IPLoop iOS SDK

Enable your iOS app to participate in the IPLoop residential proxy network.

## Requirements

- iOS 13.0+
- macOS 10.15+
- Swift 5.7+

## Installation

### Swift Package Manager

Add the following to your `Package.swift`:

```swift
dependencies: [
    .package(url: "https://github.com/iploop/ios-sdk.git", from: "1.0.0")
]
```

Or in Xcode: File → Add Packages → Enter the repository URL.

## Quick Start

### 1. Initialize the SDK

```swift
import IPLoopSDK

// In your AppDelegate or app initialization
IPLoopSDK.shared.initialize(apiKey: "your_api_key")
```

### 2. Request User Consent (Required)

```swift
// Before starting the SDK, ensure user consent
func requestConsent() {
    let alert = UIAlertController(
        title: "Share Your Connection",
        message: "Help improve internet access by sharing your unused bandwidth. Your data remains private.",
        preferredStyle: .alert
    )
    
    alert.addAction(UIAlertAction(title: "Accept", style: .default) { _ in
        IPLoopSDK.shared.setUserConsent(true)
        self.startSDK()
    })
    
    alert.addAction(UIAlertAction(title: "Decline", style: .cancel))
    
    present(alert, animated: true)
}
```

### 3. Start the SDK

```swift
func startSDK() {
    guard IPLoopSDK.shared.hasUserConsent() else {
        print("User consent required")
        return
    }
    
    Task {
        do {
            try await IPLoopSDK.shared.start()
            print("SDK started successfully")
        } catch {
            print("Failed to start SDK: \(error)")
        }
    }
}
```

### 4. Monitor Status

```swift
IPLoopSDK.shared.onStatusChange = { status in
    switch status {
    case .connected:
        print("Connected to IPLoop network")
    case .disconnected:
        print("Disconnected from network")
    case .connecting:
        print("Connecting...")
    case .error:
        print("Error occurred")
    default:
        break
    }
}

IPLoopSDK.shared.onError = { error in
    print("SDK Error: \(error)")
}
```

### 5. Get Bandwidth Stats

```swift
let stats = IPLoopSDK.shared.getBandwidthStats()
print("Total bandwidth used: \(stats.totalMB) MB")
print("Total requests: \(stats.totalRequests)")
```

### 6. Stop the SDK

```swift
IPLoopSDK.shared.stop()
```

## Full Example

```swift
import SwiftUI
import IPLoopSDK

struct ContentView: View {
    @State private var isActive = false
    @State private var status = "Disconnected"
    @State private var bandwidthUsed = 0.0
    
    var body: some View {
        VStack(spacing: 20) {
            Text("IPLoop SDK")
                .font(.title)
            
            Text("Status: \(status)")
            Text("Bandwidth: \(String(format: "%.2f", bandwidthUsed)) MB")
            
            Toggle("Enable Sharing", isOn: $isActive)
                .onChange(of: isActive) { newValue in
                    if newValue {
                        startSDK()
                    } else {
                        IPLoopSDK.shared.stop()
                    }
                }
                .padding()
        }
        .onAppear {
            setupSDK()
        }
    }
    
    func setupSDK() {
        IPLoopSDK.shared.initialize(apiKey: "your_api_key")
        
        IPLoopSDK.shared.onStatusChange = { newStatus in
            switch newStatus {
            case .connected: status = "Connected"
            case .disconnected: status = "Disconnected"
            case .connecting: status = "Connecting..."
            default: break
            }
        }
        
        // Update bandwidth every 5 seconds
        Timer.scheduledTimer(withTimeInterval: 5, repeats: true) { _ in
            bandwidthUsed = IPLoopSDK.shared.getBandwidthStats().totalMB
        }
    }
    
    func startSDK() {
        Task {
            do {
                try await IPLoopSDK.shared.start()
            } catch {
                print("Error: \(error)")
                isActive = false
            }
        }
    }
}
```

## Privacy & Compliance

### GDPR Compliance

Always obtain user consent before starting the SDK:

```swift
// Check consent
if IPLoopSDK.shared.hasUserConsent() {
    // Safe to start
}

// Set consent
IPLoopSDK.shared.setUserConsent(true)
```

### App Store Guidelines

When submitting to the App Store, ensure your app:
1. Clearly discloses the bandwidth sharing functionality
2. Obtains explicit user consent
3. Provides an easy way to disable the feature
4. Explains what data is shared (only routing, no personal data)

## Support

- Email: support@iploop.io
- Documentation: https://docs.iploop.io
- Issues: https://github.com/iploop/ios-sdk/issues

## License

MIT License - see LICENSE file for details.
