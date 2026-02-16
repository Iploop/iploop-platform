#pragma once

#include <string>
#include <functional>
#include <memory>

namespace IPLoop {

/**
 * SDK Status values - matches Android SDK
 */
enum class SDKStatus {
    IDLE = 0,           // Not initialized
    INITIALIZING = 1,   // Initialization in progress
    CONNECTING = 2,     // Connecting to server
    CONNECTED = 3,      // Connected and active
    RECONNECTING = 4,   // Connection lost, attempting to reconnect
    STOPPING = 5,       // Stop requested, shutting down
    STOPPED = 6,        // Completely stopped
    ERROR = 7           // Error state
};

/**
 * Connection status for individual tunnels
 */
enum class ConnectionStatus {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    RECONNECTING,
    ERROR
};

/**
 * Bandwidth statistics - mirrors Android implementation
 */
struct BandwidthStats {
    uint64_t totalBytesUp = 0;      // Bytes uploaded
    uint64_t totalBytesDown = 0;    // Bytes downloaded  
    uint64_t totalRequests = 0;     // Total proxy requests handled
    uint32_t activeConnections = 0; // Current active connections
    uint32_t totalConnections = 0;  // Total connections since start
    double totalMB = 0.0;          // Total MB transferred (up + down)
    uint64_t sessionStartTime = 0;  // Unix timestamp of session start
    
    // Reset all counters
    void reset() {
        totalBytesUp = 0;
        totalBytesDown = 0;
        totalRequests = 0;
        activeConnections = 0;
        totalConnections = 0;
        totalMB = 0.0;
        sessionStartTime = 0;
    }
    
    // Update total MB from bytes
    void updateTotalMB() {
        totalMB = static_cast<double>(totalBytesUp + totalBytesDown) / (1024.0 * 1024.0);
    }
};

/**
 * Device information for registration
 */
struct DeviceInfo {
    std::string deviceId;           // Unique device identifier
    std::string osVersion;          // Windows version
    std::string architecture;       // x64, x86, arm64
    std::string sdkVersion;         // IPLoop SDK version
    std::string appName;            // Host application name
    std::string appVersion;         // Host application version
    std::string networkType;        // wifi, ethernet, mobile
    std::string ipAddress;          // Local IP address
    std::string macAddress;         // MAC address
    uint32_t availableMemory = 0;   // Available RAM in MB
    uint32_t cpuCores = 0;          // Number of CPU cores
};

/**
 * Connection information for WebSocket
 */
struct ConnectionInfo {
    std::string serverUrl;          // WebSocket server URL
    std::string deviceId;           // Device ID for this connection
    uint32_t reconnectAttempts = 0; // Current reconnect attempt count
    uint64_t lastConnectTime = 0;   // Last successful connection timestamp
    uint64_t totalUptime = 0;       // Total connected time in milliseconds
    ConnectionStatus status = ConnectionStatus::DISCONNECTED;
};

/**
 * Tunnel session information
 */
struct TunnelSession {
    std::string sessionId;          // Unique session ID
    std::string remoteHost;         // Target host
    uint16_t remotePort;           // Target port
    uint64_t bytesTransferred = 0; // Bytes transferred in this session
    uint64_t startTime = 0;        // Session start time
    bool isActive = false;         // Whether session is currently active
};

/**
 * Error information
 */
struct ErrorInfo {
    int code;                      // Error code
    std::string message;           // Error message
    std::string details;           // Additional details
    uint64_t timestamp;           // When error occurred
};

/**
 * Log levels - matches Android implementation
 */
enum class LogLevel {
    VERBOSE = 0,
    DEBUG = 1,
    INFO = 2,
    WARN = 3,
    ERROR = 4
};

/**
 * Network operation result
 */
template<typename T>
struct Result {
    bool success = false;
    T data = {};
    ErrorInfo error = {};
    
    static Result<T> Success(const T& value) {
        Result<T> result;
        result.success = true;
        result.data = value;
        return result;
    }
    
    static Result<T> Error(int code, const std::string& message) {
        Result<T> result;
        result.success = false;
        result.error.code = code;
        result.error.message = message;
        return result;
    }
};

} // namespace IPLoop