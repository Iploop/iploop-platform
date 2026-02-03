import Foundation
import Network

/// IPLoop SDK for macOS - Residential Proxy Network
public class IPLoopSDK {
    
    public static let shared = IPLoopSDK()
    
    private var apiKey: String?
    private var registrationURL = "http://178.128.172.81:8001"
    private var deviceId: String
    private var isRunning = false
    private var heartbeatTimer: Timer?
    private var listener: NWListener?
    private var bandwidthStats = BandwidthStats()
    
    public var onStatusChange: ((SDKStatus) -> Void)?
    public var onError: ((Error) -> Void)?
    
    private let sdkVersion = "1.0.0"
    private let heartbeatInterval: TimeInterval = 30
    
    private init() {
        deviceId = Self.getOrCreateDeviceId()
    }
    
    // MARK: - Public API
    
    public func initialize(apiKey: String) {
        self.apiKey = apiKey
        log("SDK initialized")
    }
    
    public func start() async throws {
        guard let apiKey = apiKey else {
            throw IPLoopError.notInitialized
        }
        
        guard !isRunning else { return }
        
        log("Starting SDK...")
        onStatusChange?(.connecting)
        
        // Register device
        try await registerDevice(apiKey: apiKey)
        
        // Start local proxy server
        try startProxyServer()
        
        // Start heartbeat
        startHeartbeat()
        
        isRunning = true
        onStatusChange?(.connected)
        log("SDK started successfully")
    }
    
    public func stop() {
        guard isRunning else { return }
        
        log("Stopping SDK...")
        onStatusChange?(.disconnecting)
        
        heartbeatTimer?.invalidate()
        heartbeatTimer = nil
        
        listener?.cancel()
        listener = nil
        
        Task { await unregisterDevice() }
        
        isRunning = false
        onStatusChange?(.disconnected)
        log("SDK stopped")
    }
    
    public var isActive: Bool { isRunning }
    
    public func getBandwidthStats() -> BandwidthStats { bandwidthStats }
    
    public func setUserConsent(_ consent: Bool) {
        UserDefaults.standard.set(consent, forKey: "iploop_consent")
    }
    
    public func hasUserConsent() -> Bool {
        UserDefaults.standard.bool(forKey: "iploop_consent")
    }
    
    // MARK: - Private
    
    private func registerDevice(apiKey: String) async throws {
        guard let url = URL(string: "\(registrationURL)/register") else {
            throw IPLoopError.invalidURL
        }
        
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue(apiKey, forHTTPHeaderField: "X-API-Key")
        
        let body: [String: Any] = [
            "device_id": deviceId,
            "device_type": "macos",
            "sdk_version": sdkVersion,
            "os_version": ProcessInfo.processInfo.operatingSystemVersionString,
            "device_model": getMacModel(),
            "connection_type": "wifi"
        ]
        
        request.httpBody = try JSONSerialization.data(withJSONObject: body)
        
        let (_, response) = try await URLSession.shared.data(for: request)
        
        guard let httpResponse = response as? HTTPURLResponse,
              httpResponse.statusCode == 200 || httpResponse.statusCode == 201 else {
            throw IPLoopError.registrationFailed
        }
    }
    
    private func unregisterDevice() async {
        guard let url = URL(string: "\(registrationURL)/unregister") else { return }
        
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try? JSONSerialization.data(withJSONObject: ["device_id": deviceId])
        
        _ = try? await URLSession.shared.data(for: request)
    }
    
    private func startProxyServer() throws {
        let params = NWParameters.tcp
        listener = try NWListener(using: params, on: 0)
        
        listener?.newConnectionHandler = { [weak self] connection in
            self?.handleConnection(connection)
        }
        
        listener?.start(queue: .global())
        log("Proxy server started")
    }
    
    private func handleConnection(_ connection: NWConnection) {
        connection.start(queue: .global())
        // In production: implement full SOCKS5/HTTP proxy handling
        bandwidthStats.totalRequests += 1
    }
    
    private func startHeartbeat() {
        heartbeatTimer = Timer.scheduledTimer(withTimeInterval: heartbeatInterval, repeats: true) { [weak self] _ in
            Task { await self?.sendHeartbeat() }
        }
        RunLoop.current.add(heartbeatTimer!, forMode: .common)
    }
    
    private func sendHeartbeat() async {
        guard let url = URL(string: "\(registrationURL)/heartbeat") else { return }
        
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        
        let body: [String: Any] = [
            "device_id": deviceId,
            "bandwidth_used_mb": bandwidthStats.totalMB,
            "total_requests": bandwidthStats.totalRequests
        ]
        request.httpBody = try? JSONSerialization.data(withJSONObject: body)
        
        _ = try? await URLSession.shared.data(for: request)
    }
    
    private static func getOrCreateDeviceId() -> String {
        if let id = UserDefaults.standard.string(forKey: "iploop_device_id") {
            return id
        }
        let id = UUID().uuidString
        UserDefaults.standard.set(id, forKey: "iploop_device_id")
        return id
    }
    
    private func getMacModel() -> String {
        var size = 0
        sysctlbyname("hw.model", nil, &size, nil, 0)
        var model = [CChar](repeating: 0, count: size)
        sysctlbyname("hw.model", &model, &size, nil, 0)
        return String(cString: model)
    }
    
    private func log(_ message: String) {
        #if DEBUG
        print("[IPLoopSDK] \(message)")
        #endif
    }
}

// MARK: - Models

public enum SDKStatus {
    case disconnected, connecting, connected, disconnecting, error
}

public enum IPLoopError: Error {
    case notInitialized, invalidURL, registrationFailed, networkError
}

public struct BandwidthStats {
    public var totalBytes: Int64 = 0
    public var totalRequests: Int = 0
    public var totalMB: Double { Double(totalBytes) / 1_048_576 }
}
