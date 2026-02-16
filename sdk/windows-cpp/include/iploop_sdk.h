#pragma once

#include <string>
#include <functional>

/**
 * IPLoop SDK for Windows - C++17 Implementation
 * 
 * Static API matching the Android Java SDK v2.0
 * Features:
 * - WebSocket connection with auto-reconnect
 * - IP info reporting with caching
 * - TCP tunnel support (binary protocol)
 * - HTTP proxy request handling
 * - Thread pool for tunnels using std::thread
 * - Device ID from Windows machine GUID
 * - Registry-based caching (no external dependencies)
 */
class IPLoopSDK {
public:
    /**
     * Initialize SDK with default server
     */
    static void init();

    /**
     * Initialize SDK with custom server URL
     * @param serverUrl WebSocket URL (e.g., "wss://gateway.iploop.io:9443/ws")
     */
    static void init(const std::string& serverUrl);

    /**
     * Start SDK - opens connection in background thread
     * Returns immediately
     */
    static void start();

    /**
     * Stop SDK and disconnect
     * Closes all active tunnels and shuts down threads
     */
    static void stop();

    /**
     * Check if connected to server
     * @return true if WebSocket connection is active
     */
    static bool isConnected();

    /**
     * Check if SDK is running (start() called but not stop())
     * @return true if SDK is in running state
     */
    static bool isRunning();

    /**
     * Get the node ID (Windows machine GUID)
     * @return device identifier string
     */
    static std::string getNodeId();

    /**
     * Get number of active tunnels
     * @return count of open tunnel connections
     */
    static int getActiveTunnelCount();

    /**
     * Get SDK version
     * @return version string
     */
    static std::string getVersion();

    /**
     * Get connection statistics
     * @return pair of (total_connections, total_disconnections)
     */
    static std::pair<long long, long long> getConnectionStats();

    /**
     * Set log level (0=none, 1=error, 2=info, 3=debug)
     * @param level log level
     */
    static void setLogLevel(int level);

private:
    IPLoopSDK() = delete;  // Static class only
};