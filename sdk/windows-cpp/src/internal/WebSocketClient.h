#pragma once

#include "../include/IPLoop/Types.h"
#include <string>
#include <functional>
#include <memory>
#include <atomic>

namespace IPLoop {

/**
 * WebSocket client for communication with IPLoop registration server
 * Handles auto-reconnect with exponential backoff - mirrors Android implementation
 */
class WebSocketClient {
public:
    explicit WebSocketClient(const std::string& serverUrl);
    ~WebSocketClient();
    
    // Callbacks
    using ConnectedCallback = std::function<void()>;
    using DisconnectedCallback = std::function<void(const std::string& reason)>;
    using MessageCallback = std::function<void(const std::string& message)>;
    using ErrorCallback = std::function<void(const std::string& error)>;
    
    void setOnConnected(ConnectedCallback callback);
    void setOnDisconnected(DisconnectedCallback callback);
    void setOnMessage(MessageCallback callback);
    void setOnError(ErrorCallback callback);
    
    // Connection management
    Result<bool> connect();
    void disconnect();
    bool isConnected() const;
    
    // Send message to server
    bool sendMessage(const std::string& message);
    
    // Connection info
    ConnectionInfo getConnectionInfo() const;
    
    // Auto-reconnect settings (v2.0 - improved reconnect logic)
    void setReconnectConfig(int maxAttempts = 15, 
                          uint64_t baseDelayMs = 1000, 
                          uint64_t maxDelayMs = 30000,
                          uint64_t slowReconnectDelayMs = 600000);
    
    // v2.0: Binary message support (no base64 overhead)
    bool sendBinaryMessage(const std::vector<uint8_t>& data);
    void setOnBinaryMessage(std::function<void(const std::vector<uint8_t>&)> callback);
    
private:
    class Impl;
    std::unique_ptr<Impl> pImpl;
};

} // namespace IPLoop