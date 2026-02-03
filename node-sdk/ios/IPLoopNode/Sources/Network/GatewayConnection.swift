import Foundation
import Starscream
import Alamofire

/// Manages WebSocket connection to IPLoop Gateway
class GatewayConnection: WebSocketDelegate {
    
    private let gatewayURL: String
    private let nodeId: String
    private let token: String
    
    private var socket: WebSocket?
    private var pendingRequests: [String: CheckedContinuation<ProxyResponse, Error>] = [:]
    private let requestLock = NSLock()
    
    var onStatusChange: ((ConnectionStatus) -> Void)?
    var onStatsUpdate: ((Int64) -> Void)?
    
    init(gatewayURL: String, nodeId: String, token: String) {
        self.gatewayURL = gatewayURL
        self.nodeId = nodeId
        self.token = token
    }
    
    func connect() async throws {
        guard let url = URL(string: "\(gatewayURL)/node/connect") else {
            throw NodeError.connectionFailed
        }
        
        var request = URLRequest(url: url)
        request.setValue(nodeId, forHTTPHeaderField: "X-Node-Id")
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.timeoutInterval = 10
        
        socket = WebSocket(request: request)
        socket?.delegate = self
        
        onStatusChange?(.connecting)
        
        return try await withCheckedThrowingContinuation { continuation in
            var resumed = false
            
            socket?.onEvent = { [weak self] event in
                guard !resumed else { return }
                
                switch event {
                case .connected:
                    resumed = true
                    self?.onStatusChange?(.connected)
                    self?.sendCapabilities()
                    continuation.resume()
                    
                case .error(let error):
                    resumed = true
                    self?.onStatusChange?(.disconnected)
                    continuation.resume(throwing: error ?? NodeError.connectionFailed)
                    
                default:
                    break
                }
            }
            
            socket?.connect()
        }
    }
    
    func disconnect() {
        socket?.disconnect()
        socket = nil
        onStatusChange?(.disconnected)
    }
    
    func sendHeartbeat(stats: NodeStats) async {
        let message = WSMessage(
            type: "heartbeat",
            payload: [
                "bytesTransferred": stats.totalBytesTransferred,
                "requestsHandled": stats.requestsHandled,
                "uptime": 0
            ]
        )
        
        sendMessage(message)
    }
    
    // MARK: - WebSocketDelegate
    
    func didReceive(event: WebSocketEvent, client: WebSocketClient) {
        switch event {
        case .text(let text):
            handleMessage(text)
            
        case .binary(let data):
            if let text = String(data: data, encoding: .utf8) {
                handleMessage(text)
            }
            
        case .disconnected(_, _):
            onStatusChange?(.disconnected)
            attemptReconnect()
            
        case .error(let error):
            print("WebSocket error: \(error?.localizedDescription ?? "unknown")")
            onStatusChange?(.reconnecting)
            attemptReconnect()
            
        case .ping(_):
            break
            
        case .pong(_):
            break
            
        case .viabilityChanged(_):
            break
            
        case .reconnectSuggested(_):
            attemptReconnect()
            
        case .cancelled:
            onStatusChange?(.disconnected)
            
        case .connected(_):
            onStatusChange?(.connected)
            
        case .peerClosed:
            onStatusChange?(.disconnected)
        }
    }
    
    // MARK: - Private Methods
    
    private func sendCapabilities() {
        let capabilities = WSMessage(
            type: "capabilities",
            payload: [
                "version": Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0",
                "platform": "ios",
                "protocols": ["http", "https"],
                "maxConcurrent": 5
            ]
        )
        sendMessage(capabilities)
    }
    
    private func handleMessage(_ text: String) {
        guard let data = text.data(using: .utf8),
              let message = try? JSONDecoder().decode(WSMessage.self, from: data) else {
            return
        }
        
        switch message.type {
        case "proxy_request":
            handleProxyRequest(message)
            
        case "config_update":
            // Handle config updates
            break
            
        case "ping":
            let pong = WSMessage(type: "pong")
            sendMessage(pong)
            
        case "heartbeat_ack":
            // Update earnings from response
            break
            
        default:
            break
        }
    }
    
    private func handleProxyRequest(_ message: WSMessage) {
        guard let requestId = message.requestId,
              let payload = message.payload as? [String: Any],
              let method = payload["method"] as? String,
              let urlString = payload["url"] as? String,
              let url = URL(string: urlString) else {
            return
        }
        
        let headers = payload["headers"] as? [String: String] ?? [:]
        let bodyBase64 = payload["body"] as? String
        
        Task {
            do {
                let response = try await executeRequest(
                    method: method,
                    url: url,
                    headers: headers,
                    bodyBase64: bodyBase64
                )
                sendResponse(requestId: requestId, response: response)
            } catch {
                sendResponse(requestId: requestId, response: ProxyResponse(
                    statusCode: 502,
                    error: error.localizedDescription
                ))
            }
        }
    }
    
    private func executeRequest(
        method: String,
        url: URL,
        headers: [String: String],
        bodyBase64: String?
    ) async throws -> ProxyResponse {
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.timeoutInterval = 60
        
        for (key, value) in headers {
            request.setValue(value, forHTTPHeaderField: key)
        }
        
        if let bodyBase64 = bodyBase64,
           let bodyData = Data(base64Encoded: bodyBase64) {
            request.httpBody = bodyData
        }
        
        let (data, response) = try await URLSession.shared.data(for: request)
        
        guard let httpResponse = response as? HTTPURLResponse else {
            throw NodeError.connectionFailed
        }
        
        // Track bytes
        let bytesTransferred = Int64(data.count + (request.httpBody?.count ?? 0))
        onStatsUpdate?(bytesTransferred)
        
        // Convert headers
        var responseHeaders: [String: String] = [:]
        for (key, value) in httpResponse.allHeaderFields {
            if let keyString = key as? String, let valueString = value as? String {
                responseHeaders[keyString] = valueString
            }
        }
        
        return ProxyResponse(
            statusCode: httpResponse.statusCode,
            headers: responseHeaders,
            bodyBase64: data.base64EncodedString()
        )
    }
    
    private func sendResponse(requestId: String, response: ProxyResponse) {
        let message = WSMessage(
            type: "proxy_response",
            requestId: requestId,
            payload: [
                "statusCode": response.statusCode,
                "headers": response.headers,
                "body": response.bodyBase64 ?? "",
                "error": response.error ?? ""
            ]
        )
        sendMessage(message)
    }
    
    private func sendMessage(_ message: WSMessage) {
        guard let data = try? JSONEncoder().encode(message),
              let text = String(data: data, encoding: .utf8) else {
            return
        }
        socket?.write(string: text)
    }
    
    private func attemptReconnect() {
        Task {
            try? await Task.sleep(nanoseconds: 5_000_000_000) // 5 seconds
            if socket != nil {
                onStatusChange?(.reconnecting)
                socket?.connect()
            }
        }
    }
}

// MARK: - Message Types

struct WSMessage: Codable {
    let type: String
    var requestId: String?
    var payload: [String: Any]?
    
    enum CodingKeys: String, CodingKey {
        case type
        case requestId
        case payload
    }
    
    init(type: String, requestId: String? = nil, payload: [String: Any]? = nil) {
        self.type = type
        self.requestId = requestId
        self.payload = payload
    }
    
    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        type = try container.decode(String.self, forKey: .type)
        requestId = try container.decodeIfPresent(String.self, forKey: .requestId)
        
        // Decode payload as generic JSON
        if let payloadData = try? container.decode([String: AnyCodable].self, forKey: .payload) {
            payload = payloadData.mapValues { $0.value }
        }
    }
    
    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(type, forKey: .type)
        try container.encodeIfPresent(requestId, forKey: .requestId)
        
        if let payload = payload {
            let codablePayload = payload.mapValues { AnyCodable($0) }
            try container.encode(codablePayload, forKey: .payload)
        }
    }
}

struct ProxyResponse {
    let statusCode: Int
    var headers: [String: String] = [:]
    var bodyBase64: String?
    var error: String?
}

// Helper for encoding Any values
struct AnyCodable: Codable {
    let value: Any
    
    init(_ value: Any) {
        self.value = value
    }
    
    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        
        if let int = try? container.decode(Int.self) {
            value = int
        } else if let double = try? container.decode(Double.self) {
            value = double
        } else if let string = try? container.decode(String.self) {
            value = string
        } else if let bool = try? container.decode(Bool.self) {
            value = bool
        } else if let array = try? container.decode([AnyCodable].self) {
            value = array.map { $0.value }
        } else if let dict = try? container.decode([String: AnyCodable].self) {
            value = dict.mapValues { $0.value }
        } else {
            value = NSNull()
        }
    }
    
    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        
        switch value {
        case let int as Int:
            try container.encode(int)
        case let int64 as Int64:
            try container.encode(int64)
        case let double as Double:
            try container.encode(double)
        case let string as String:
            try container.encode(string)
        case let bool as Bool:
            try container.encode(bool)
        case let array as [Any]:
            try container.encode(array.map { AnyCodable($0) })
        case let dict as [String: Any]:
            try container.encode(dict.mapValues { AnyCodable($0) })
        default:
            try container.encodeNil()
        }
    }
}
