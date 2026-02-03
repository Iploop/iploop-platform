import Foundation
import IPLoopSDK

@main
struct IPLoopDaemon {
    static func main() async {
        print("IPLoop Daemon v1.0.0")
        print("====================")
        
        // Check for API key
        guard let apiKey = ProcessInfo.processInfo.environment["IPLOOP_API_KEY"] 
              ?? UserDefaults.standard.string(forKey: "iploop_api_key") else {
            print("Error: IPLOOP_API_KEY environment variable not set")
            print("Usage: IPLOOP_API_KEY=your_key iploop-daemon")
            exit(1)
        }
        
        // Initialize SDK
        let sdk = IPLoopSDK.shared
        sdk.initialize(apiKey: apiKey)
        
        // Set up callbacks
        sdk.onStatusChange = { status in
            switch status {
            case .connecting: print("Status: Connecting...")
            case .connected: print("Status: Connected ✓")
            case .disconnecting: print("Status: Disconnecting...")
            case .disconnected: print("Status: Disconnected")
            case .error: print("Status: Error")
            }
        }
        
        // Handle signals for graceful shutdown
        signal(SIGINT) { _ in
            print("\nShutting down...")
            IPLoopSDK.shared.stop()
            exit(0)
        }
        
        signal(SIGTERM) { _ in
            print("\nShutting down...")
            IPLoopSDK.shared.stop()
            exit(0)
        }
        
        // Start SDK
        do {
            try await sdk.start()
            print("Daemon running. Press Ctrl+C to stop.")
            
            // Keep running and print stats periodically
            while true {
                try await Task.sleep(nanoseconds: 60_000_000_000) // 60 seconds
                let stats = sdk.getBandwidthStats()
                print("Stats: \(stats.totalRequests) requests, \(String(format: "%.2f", stats.totalMB)) MB transferred")
            }
        } catch {
            print("Error starting daemon: \(error)")
            exit(1)
        }
    }
}
