#pragma once

#include <string>
#include <vector>
#include <map>
#include <functional>
#include <atomic>
#include <mutex>
#include <thread>
#include <memory>

#include <windows.h>
#include <winsock2.h>
#include <ws2tcpip.h>

namespace iploop {

/**
 * TCP tunnel connection through IPLoop node
 * Handles bidirectional data forwarding between client and target
 */
class TunnelConnection {
public:
    TunnelConnection(const std::string& tunnelId, const std::string& host, int port);
    ~TunnelConnection();

    /**
     * Connect to target server
     * @param timeoutMs connection timeout in milliseconds
     * @return true if connection successful
     */
    bool connect(int timeoutMs = 10000);

    /**
     * Close tunnel connection
     */
    void close();

    /**
     * Check if tunnel is connected
     * @return true if connected to target
     */
    bool isConnected() const;

    /**
     * Write data to target socket
     * @param data data to write
     * @param len data length
     * @return true if written successfully
     */
    bool writeData(const unsigned char* data, size_t len);

    /**
     * Start reading from target and call dataHandler for each chunk
     * @param dataHandler callback for data chunks (tunnelId, data, len, isEof)
     */
    void startReading(std::function<void(const std::string&, const unsigned char*, size_t, bool)> dataHandler);

    /**
     * Stop reading from target
     */
    void stopReading();

    /**
     * Get tunnel ID
     * @return tunnel identifier
     */
    std::string getTunnelId() const { return tunnelId_; }

    /**
     * Get target host
     * @return target hostname
     */
    std::string getHost() const { return host_; }

    /**
     * Get target port
     * @return target port number
     */
    int getPort() const { return port_; }

private:
    std::string tunnelId_;
    std::string host_;
    int port_;
    SOCKET socket_;
    std::atomic<bool> connected_;
    std::atomic<bool> closed_;
    std::thread readThread_;
    std::mutex writeMutex_;
    
    void readLoop(std::function<void(const std::string&, const unsigned char*, size_t, bool)> dataHandler);
};

/**
 * Tunnel manager - handles multiple tunnel connections
 * Thread-safe management of active tunnels
 */
class TunnelManager {
public:
    /**
     * Data handler callback for tunnel data
     * @param tunnelId tunnel identifier
     * @param data binary data
     * @param len data length
     * @param isEof true if end-of-file
     */
    using DataHandler = std::function<void(const std::string& tunnelId, const unsigned char* data, size_t len, bool isEof)>;

    /**
     * Response handler callback for tunnel operations
     * @param tunnelId tunnel identifier
     * @param success true if operation successful
     * @param error error message (empty if success)
     */
    using ResponseHandler = std::function<void(const std::string& tunnelId, bool success, const std::string& error)>;

    TunnelManager();
    ~TunnelManager();

    /**
     * Set data handler for tunnel data
     * @param handler callback function
     */
    void setDataHandler(const DataHandler& handler);

    /**
     * Set response handler for tunnel operations
     * @param handler callback function
     */
    void setResponseHandler(const ResponseHandler& handler);

    /**
     * Open new tunnel to target host:port
     * @param tunnelId tunnel identifier from server
     * @param host target hostname
     * @param port target port
     * @param timeoutMs connection timeout
     */
    void openTunnel(const std::string& tunnelId, const std::string& host, int port, int timeoutMs = 10000);

    /**
     * Write data to tunnel
     * @param tunnelId tunnel identifier
     * @param data binary data
     * @param len data length
     * @return true if data written
     */
    bool writeTunnelData(const std::string& tunnelId, const unsigned char* data, size_t len);

    /**
     * Close tunnel connection
     * @param tunnelId tunnel identifier
     */
    void closeTunnel(const std::string& tunnelId);

    /**
     * Close all active tunnels
     */
    void closeAllTunnels();

    /**
     * Get number of active tunnels
     * @return active tunnel count
     */
    int getActiveTunnelCount() const;

    /**
     * Get list of active tunnel IDs
     * @return vector of tunnel identifiers
     */
    std::vector<std::string> getActiveTunnelIds() const;

private:
    mutable std::mutex tunnelsMutex_;
    std::map<std::string, std::shared_ptr<TunnelConnection>> activeTunnels_;
    std::map<std::string, long long> recentlyClosedTunnels_;
    DataHandler dataHandler_;
    ResponseHandler responseHandler_;
    
    void cleanupRecentlyClosed();
    void handleTunnelData(const std::string& tunnelId, const unsigned char* data, size_t len, bool isEof);
};

/**
 * HTTP proxy request handler
 * Handles proxy_request messages from server
 */
class ProxyHandler {
public:
    /**
     * Response handler callback for proxy operations
     * @param requestId request identifier  
     * @param success true if request successful
     * @param statusCode HTTP status code (0 if failed)
     * @param responseBody response body (base64 encoded)
     * @param latencyMs request latency in milliseconds
     * @param error error message (empty if success)
     */
    using ResponseHandler = std::function<void(const std::string& requestId, bool success, int statusCode, 
                                              const std::string& responseBody, long long latencyMs, const std::string& error)>;

    ProxyHandler();
    ~ProxyHandler();

    /**
     * Set response handler for proxy operations
     * @param handler callback function
     */
    void setResponseHandler(const ResponseHandler& handler);

    /**
     * Handle proxy request from server
     * @param requestId request identifier
     * @param method HTTP method (GET, POST, etc.)
     * @param url target URL
     * @param headers request headers (JSON object string)
     * @param bodyBase64 request body (base64 encoded)
     * @param timeoutMs request timeout
     */
    void handleProxyRequest(const std::string& requestId, const std::string& method, 
                           const std::string& url, const std::string& headers,
                           const std::string& bodyBase64, int timeoutMs = 30000);

private:
    ResponseHandler responseHandler_;
    
    std::string makeHttpRequest(const std::string& method, const std::string& url,
                               const std::string& headers, const std::string& bodyBase64, 
                               int timeoutMs, long long& latencyMs);
};

} // namespace iploop