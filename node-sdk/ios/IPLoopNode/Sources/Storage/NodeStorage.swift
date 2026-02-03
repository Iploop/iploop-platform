import Foundation

/// Persistent storage for IPLoop Node data
public class NodeStorage {
    
    public static let shared = NodeStorage()
    
    private let defaults = UserDefaults.standard
    
    private enum Keys {
        static let nodeId = "iploop_node_id"
        static let authToken = "iploop_auth_token"
        static let deviceId = "iploop_device_id"
        static let totalBytes = "iploop_total_bytes"
        static let totalRequests = "iploop_total_requests"
        static let totalEarnings = "iploop_total_earnings"
        static let autoStart = "iploop_auto_start"
        static let wifiOnly = "iploop_wifi_only"
    }
    
    private init() {}
    
    // MARK: - Node Identity
    
    public var nodeId: String? {
        get { defaults.string(forKey: Keys.nodeId) }
        set { defaults.set(newValue, forKey: Keys.nodeId) }
    }
    
    public var authToken: String? {
        get { defaults.string(forKey: Keys.authToken) }
        set { defaults.set(newValue, forKey: Keys.authToken) }
    }
    
    public var deviceId: String? {
        get { defaults.string(forKey: Keys.deviceId) }
        set { defaults.set(newValue, forKey: Keys.deviceId) }
    }
    
    public var isRegistered: Bool {
        nodeId != nil && authToken != nil
    }
    
    // MARK: - Stats
    
    public var totalBytes: Int64 {
        get { Int64(defaults.integer(forKey: Keys.totalBytes)) }
        set { defaults.set(Int(newValue), forKey: Keys.totalBytes) }
    }
    
    public var totalRequests: Int64 {
        get { Int64(defaults.integer(forKey: Keys.totalRequests)) }
        set { defaults.set(Int(newValue), forKey: Keys.totalRequests) }
    }
    
    public var totalEarnings: Double {
        get { defaults.double(forKey: Keys.totalEarnings) }
        set { defaults.set(newValue, forKey: Keys.totalEarnings) }
    }
    
    public func addBytes(_ bytes: Int64) {
        totalBytes += bytes
    }
    
    public func addRequests(_ count: Int64 = 1) {
        totalRequests += count
    }
    
    public func addEarnings(_ amount: Double) {
        totalEarnings += amount
    }
    
    // MARK: - Settings
    
    public var autoStart: Bool {
        get { defaults.bool(forKey: Keys.autoStart) }
        set { defaults.set(newValue, forKey: Keys.autoStart) }
    }
    
    public var wifiOnly: Bool {
        get { defaults.bool(forKey: Keys.wifiOnly) }
        set { defaults.set(newValue, forKey: Keys.wifiOnly) }
    }
    
    // MARK: - Clear
    
    public func clearAll() {
        let keys = [
            Keys.nodeId, Keys.authToken, Keys.deviceId,
            Keys.totalBytes, Keys.totalRequests, Keys.totalEarnings,
            Keys.autoStart, Keys.wifiOnly
        ]
        keys.forEach { defaults.removeObject(forKey: $0) }
    }
}
