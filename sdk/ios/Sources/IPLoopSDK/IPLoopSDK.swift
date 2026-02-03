import Foundation
import Network
import SystemConfiguration

/// IPLoop SDK for iOS - Residential Proxy Network
/// Enables devices to participate in the IPLoop proxy network
public class IPLoopSDK {
    
    // MARK: - Singleton
    public static let shared = IPLoopSDK()
    
    // MARK: - Configuration
    private var apiKey: String?
    private var serverURL: String = "https://api.iploop.io"
    private var registrationURL: String = "http://178.128.172.81:8001"
    private var deviceId: String?
    private var isRunning: Bool = false
    private var heartbeatTimer: Timer?
    private var proxyServer: ProxyServer?
    
    // MARK: - Callbacks
    public var onStatusChange: ((SDKStatus) -> Void)?
    public var onError: ((IPLoopError) -> Void)?
    public var onBandwidthUpdate: ((BandwidthStats) -> Void)?
    
    // MARK: - Constants
    private let heartbeatInterval: TimeInterval = 30
    private let sdkVersion = "1.0.0"
    
    private init() {
        deviceId = getDeviceId()
    }
    
    // MARK: - Public Methods
    
    /// Initialize the SDK with your API key
    /// - Parameters:
    ///   - apiKey: Your IPLoop API key
    ///   - serverURL: Optional custom server URL
    public func initialize(apiKey: String, serverURL: String? = nil) {
        self.apiKey = apiKey
        if let url = serverURL {
            self.serverURL = url
        }
        log("SDK initialized with API key: \(apiKey.prefix(8))...")
    }
    
    /// Start the proxy service
    /// Device will register with the network and begin accepting proxy requests
    public func start() async throws {
        guard let apiKey = apiKey else {
            throw IPLoopError.notInitialized
        }
        
        guard !isRunning else {
            log("SDK already running")
            return
        }
        
        log("Starting IPLoop SDK...")
        onStatusChange?(.connecting)
        
        // Get device info
        let deviceInfo = getDeviceInfo()
        
        // Register with server
        try await registerDevice(apiKey: apiKey, deviceInfo: deviceInfo)
        
        // Start proxy server
        proxyServer = ProxyServer()
        try await proxyServer?.start()
        
        // Start heartbeat
        startHeartbeat()
        
        isRunning = true
        onStatusChange?(.connected)
        log("SDK started successfully")
    }
    
    /// Stop the proxy service
    public func stop() {
        guard isRunning else { return }
        
        log("Stopping IPLoop SDK...")
        onStatusChange?(.disconnecting)
        
        // Stop heartbeat
        heartbeatTimer?.invalidate()
        heartbeatTimer = nil
        
        // Stop proxy server
        proxyServer?.stop()
        proxyServer = nil
        
        // Unregister from server
        Task {
            await unregisterDevice()
        }
        
        isRunning = false
        onStatusChange?(.disconnected)
        log("SDK stopped")
    }
    
    /// Check if SDK is currently running
    public var isActive: Bool {
        return isRunning
    }
    
    /// Get current bandwidth statistics
    public func getBandwidthStats() -> BandwidthStats {
        return proxyServer?.bandwidthStats ?? BandwidthStats()
    }
    
    /// Set user consent for data sharing (required for GDPR compliance)
    public func setUserConsent(_ consent: Bool) {
        UserDefaults.standard.set(consent, forKey: "iploop_user_consent")
        log("User consent set to: \(consent)")
    }
    
    /// Check if user has given consent
    public func hasUserConsent() -> Bool {
        return UserDefaults.standard.bool(forKey: "iploop_user_consent")
    }
    
    // MARK: - Private Methods
    
    private func registerDevice(apiKey: String, deviceInfo: DeviceInfo) async throws {
        guard let url = URL(string: "\(registrationURL)/register") else {
            throw IPLoopError.invalidURL
        }
        
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue(apiKey, forHTTPHeaderField: "X-API-Key")
        
        let body: [String: Any] = [
            "device_id": deviceInfo.deviceId,
            "device_type": "ios",
            "sdk_version": sdkVersion,
            "os_version": deviceInfo.osVersion,
            "device_model": deviceInfo.model,
            "connection_type": deviceInfo.connectionType,
            "carrier": deviceInfo.carrier ?? ""
        ]
        
        request.httpBody = try JSONSerialization.data(withJSONObject: body)
        
        let (data, response) = try await URLSession.shared.data(for: request)
        
        guard let httpResponse = response as? HTTPURLResponse else {
            throw IPLoopError.networkError
        }
        
        if httpResponse.statusCode != 200 && httpResponse.statusCode != 201 {
            if let errorJson = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
               let message = errorJson["error"] as? String {
                throw IPLoopError.serverError(message)
            }
            throw IPLoopError.registrationFailed
        }
        
        log("Device registered successfully")
    }
    
    private func unregisterDevice() async {
        guard let url = URL(string: "\(registrationURL)/unregister") else { return }
        guard let deviceId = deviceId else { return }
        
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        
        let body: [String: Any] = ["device_id": deviceId]
        request.httpBody = try? JSONSerialization.data(withJSONObject: body)
        
        _ = try? await URLSession.shared.data(for: request)
    }
    
    private func startHeartbeat() {
        heartbeatTimer = Timer.scheduledTimer(withTimeInterval: heartbeatInterval, repeats: true) { [weak self] _ in
            Task {
                await self?.sendHeartbeat()
            }
        }
    }
    
    private func sendHeartbeat() async {
        guard let url = URL(string: "\(registrationURL)/heartbeat") else { return }
        guard let deviceId = deviceId else { return }
        
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        
        let stats = getBandwidthStats()
        let body: [String: Any] = [
            "device_id": deviceId,
            "bandwidth_used_mb": stats.totalMB,
            "total_requests": stats.totalRequests,
            "successful_requests": stats.successfulRequests
        ]
        
        request.httpBody = try? JSONSerialization.data(withJSONObject: body)
        
        do {
            let (_, response) = try await URLSession.shared.data(for: request)
            if let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode != 200 {
                log("Heartbeat failed with status: \(httpResponse.statusCode)")
            }
        } catch {
            log("Heartbeat error: \(error.localizedDescription)")
        }
    }
    
    private func getDeviceId() -> String {
        if let stored = UserDefaults.standard.string(forKey: "iploop_device_id") {
            return stored
        }
        let newId = UUID().uuidString
        UserDefaults.standard.set(newId, forKey: "iploop_device_id")
        return newId
    }
    
    private func getDeviceInfo() -> DeviceInfo {
        #if os(iOS)
        import UIKit
        let device = UIDevice.current
        return DeviceInfo(
            deviceId: deviceId ?? UUID().uuidString,
            model: device.model,
            osVersion: device.systemVersion,
            connectionType: getConnectionType(),
            carrier: getCarrierName()
        )
        #else
        return DeviceInfo(
            deviceId: deviceId ?? UUID().uuidString,
            model: "Mac",
            osVersion: ProcessInfo.processInfo.operatingSystemVersionString,
            connectionType: "wifi",
            carrier: nil
        )
        #endif
    }
    
    private func getConnectionType() -> String {
        // Simplified - in production use NWPathMonitor
        return "wifi"
    }
    
    private func getCarrierName() -> String? {
        #if os(iOS)
        // In production, use CTTelephonyNetworkInfo
        return nil
        #else
        return nil
        #endif
    }
    
    private func log(_ message: String) {
        #if DEBUG
        print("[IPLoopSDK] \(message)")
        #endif
    }
}

// MARK: - Models

public enum SDKStatus {
    case disconnected
    case connecting
    case connected
    case disconnecting
    case error
}

public enum IPLoopError: Error {
    case notInitialized
    case invalidURL
    case networkError
    case registrationFailed
    case serverError(String)
    case noConsent
}

public struct BandwidthStats {
    public var totalBytes: Int64 = 0
    public var totalRequests: Int = 0
    public var successfulRequests: Int = 0
    
    public var totalMB: Double {
        return Double(totalBytes) / (1024 * 1024)
    }
}

struct DeviceInfo {
    let deviceId: String
    let model: String
    let osVersion: String
    let connectionType: String
    let carrier: String?
}

// MARK: - Proxy Server (Simplified)

class ProxyServer {
    var bandwidthStats = BandwidthStats()
    private var listener: NWListener?
    
    func start() async throws {
        // In production, implement full SOCKS5/HTTP proxy server
        // This is a simplified placeholder
        print("[IPLoopSDK] Proxy server started (placeholder)")
    }
    
    func stop() {
        listener?.cancel()
        listener = nil
        print("[IPLoopSDK] Proxy server stopped")
    }
}
