import Foundation
import UIKit
import BackgroundTasks

/// IPLoop Node Service
/// Manages the node connection and proxy request handling
@MainActor
public class NodeService: ObservableObject {
    
    public static let shared = NodeService()
    
    // MARK: - Published Properties
    @Published public private(set) var isRunning = false
    @Published public private(set) var connectionStatus: ConnectionStatus = .disconnected
    @Published public private(set) var stats = NodeStats()
    
    // MARK: - Private Properties
    private var gatewayConnection: GatewayConnection?
    private let storage = NodeStorage.shared
    private var backgroundTask: UIBackgroundTaskIdentifier = .invalid
    
    // MARK: - Configuration
    #if DEBUG
    private let apiBaseURL = "http://localhost:8001"
    private let gatewayURL = "ws://localhost:8080"
    #else
    private let apiBaseURL = "https://api.iploop.io"
    private let gatewayURL = "wss://gateway.iploop.io"
    #endif
    
    private init() {
        setupBackgroundTasks()
    }
    
    // MARK: - Public Methods
    
    public func start() async throws {
        guard !isRunning else { return }
        
        // Ensure we're registered
        if storage.nodeId == nil {
            try await register()
        }
        
        guard let nodeId = storage.nodeId,
              let token = storage.authToken else {
            throw NodeError.notRegistered
        }
        
        // Start background task
        beginBackgroundTask()
        
        // Connect to gateway
        gatewayConnection = GatewayConnection(
            gatewayURL: gatewayURL,
            nodeId: nodeId,
            token: token
        )
        
        gatewayConnection?.onStatusChange = { [weak self] status in
            Task { @MainActor in
                self?.connectionStatus = status
            }
        }
        
        gatewayConnection?.onStatsUpdate = { [weak self] bytesTransferred in
            Task { @MainActor in
                self?.stats.totalBytesTransferred += bytesTransferred
                self?.stats.requestsHandled += 1
                self?.storage.addBytes(bytesTransferred)
            }
        }
        
        try await gatewayConnection?.connect()
        isRunning = true
        
        // Start heartbeat
        startHeartbeat()
    }
    
    public func stop() {
        gatewayConnection?.disconnect()
        gatewayConnection = nil
        isRunning = false
        connectionStatus = .disconnected
        endBackgroundTask()
    }
    
    // MARK: - Registration
    
    private func register() async throws {
        let deviceId = await UIDevice.current.identifierForVendor?.uuidString ?? UUID().uuidString
        
        let deviceInfo = DeviceInfo(
            model: await UIDevice.current.model,
            systemVersion: await UIDevice.current.systemVersion,
            appVersion: Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0",
            connectionType: getConnectionType()
        )
        
        let request = NodeRegistrationRequest(
            deviceId: deviceId,
            deviceInfo: deviceInfo
        )
        
        let url = URL(string: "\(apiBaseURL)/nodes/register")!
        var urlRequest = URLRequest(url: url)
        urlRequest.httpMethod = "POST"
        urlRequest.setValue("application/json", forHTTPHeaderField: "Content-Type")
        urlRequest.httpBody = try JSONEncoder().encode(request)
        
        let (data, response) = try await URLSession.shared.data(for: urlRequest)
        
        guard let httpResponse = response as? HTTPURLResponse,
              httpResponse.statusCode == 200 else {
            throw NodeError.registrationFailed
        }
        
        let result = try JSONDecoder().decode(NodeRegistrationResponse.self, from: data)
        
        guard result.success,
              let nodeId = result.nodeId,
              let token = result.token else {
            throw NodeError.registrationFailed
        }
        
        storage.nodeId = nodeId
        storage.authToken = token
        storage.deviceId = deviceId
    }
    
    // MARK: - Background Tasks
    
    private func setupBackgroundTasks() {
        BGTaskScheduler.shared.register(
            forTaskWithIdentifier: "io.iploop.node.refresh",
            using: nil
        ) { task in
            self.handleBackgroundRefresh(task: task as! BGAppRefreshTask)
        }
    }
    
    private func handleBackgroundRefresh(task: BGAppRefreshTask) {
        scheduleBackgroundRefresh()
        
        task.expirationHandler = {
            task.setTaskCompleted(success: false)
        }
        
        Task {
            if isRunning {
                // Send heartbeat
                await gatewayConnection?.sendHeartbeat(stats: stats)
            }
            task.setTaskCompleted(success: true)
        }
    }
    
    private func scheduleBackgroundRefresh() {
        let request = BGAppRefreshTaskRequest(identifier: "io.iploop.node.refresh")
        request.earliestBeginDate = Date(timeIntervalSinceNow: 15 * 60) // 15 minutes
        
        do {
            try BGTaskScheduler.shared.submit(request)
        } catch {
            print("Could not schedule background refresh: \(error)")
        }
    }
    
    private func beginBackgroundTask() {
        backgroundTask = UIApplication.shared.beginBackgroundTask { [weak self] in
            self?.endBackgroundTask()
        }
    }
    
    private func endBackgroundTask() {
        if backgroundTask != .invalid {
            UIApplication.shared.endBackgroundTask(backgroundTask)
            backgroundTask = .invalid
        }
    }
    
    // MARK: - Heartbeat
    
    private func startHeartbeat() {
        Task {
            while isRunning {
                try? await Task.sleep(nanoseconds: 30_000_000_000) // 30 seconds
                if isRunning {
                    await gatewayConnection?.sendHeartbeat(stats: stats)
                }
            }
        }
    }
    
    // MARK: - Helpers
    
    private func getConnectionType() -> String {
        // Simplified - would need Network framework for accurate detection
        return "wifi"
    }
}

// MARK: - Supporting Types

public enum ConnectionStatus {
    case disconnected
    case connecting
    case connected
    case reconnecting
}

public struct NodeStats {
    public var totalBytesTransferred: Int64 = 0
    public var requestsHandled: Int64 = 0
    public var earnings: Double = 0
}

public enum NodeError: Error {
    case notRegistered
    case registrationFailed
    case connectionFailed
    case authenticationFailed
}

// MARK: - API Types

struct NodeRegistrationRequest: Codable {
    let deviceId: String
    let deviceInfo: DeviceInfo
}

struct DeviceInfo: Codable {
    let model: String
    let systemVersion: String
    let appVersion: String
    let connectionType: String
}

struct NodeRegistrationResponse: Codable {
    let success: Bool
    let nodeId: String?
    let token: String?
    let message: String?
}
