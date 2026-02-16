#pragma once

#include "../include/IPLoop/Types.h"
#include "../include/IPLoop/Callbacks.h"
#include <memory>
#include <vector>
#include <cstdint>

namespace IPLoop {

/**
 * Tunnel Manager v2.0 - Binary Protocol
 * 
 * Key v2.0 improvements:
 * - Binary tunnel protocol (no base64 overhead)
 * - Better connection pooling and reuse
 * - Smart retry with node scoring
 * - Optimized for production throughput
 */
class TunnelManager {
public:
    TunnelManager();
    ~TunnelManager();
    
    // Lifecycle
    void start();
    void stop();
    bool isRunning() const;
    
    // v2.0: Binary tunnel creation
    struct TunnelRequest {
        std::string sessionId;
        std::string targetHost;
        uint16_t targetPort;
        std::vector<uint8_t> initialData;  // Binary data, not base64
        std::string proxyAuth;             // Enterprise auth string
    };
    
    struct TunnelResponse {
        bool success;
        std::string tunnelId;
        std::string errorMessage;
        uint32_t assignedNode;             // Node ID for this tunnel
    };
    
    // Create tunnel with binary protocol
    TunnelResponse createTunnel(const TunnelRequest& request);
    
    // Send binary data through tunnel (v2.0: no encoding overhead)
    bool sendTunnelData(const std::string& tunnelId, const std::vector<uint8_t>& data);
    
    // Close tunnel
    void closeTunnel(const std::string& tunnelId);
    
    // v2.0: Tunnel statistics with better metrics
    struct TunnelStats {
        uint32_t activeTunnels = 0;
        uint32_t totalTunnels = 0;
        uint64_t bytesTransferred = 0;
        uint32_t averageLatencyMs = 0;
        uint32_t failedConnections = 0;
        double throughputMbps = 0.0;
        uint64_t sessionStartTime = 0;
        
        // v2.0: Node performance tracking
        std::vector<std::pair<uint32_t, double>> nodeScores;  // nodeId, score
    };
    
    TunnelStats getStats() const;
    
    // Callbacks for tunnel events
    void setOnTunnelCreated(TunnelCreatedCallback callback);
    void setOnTunnelClosed(TunnelClosedCallback callback);
    void setOnTunnelData(std::function<void(const std::string&, const std::vector<uint8_t>&)> callback);
    void setOnTunnelError(std::function<void(const std::string&, const std::string&)> callback);
    
    // v2.0: Connection pool management
    void setMaxTunnelsPerNode(uint32_t maxTunnels);  // Default: 5 tunnels per node
    void setTunnelTimeout(uint32_t timeoutMs);       // Default: 30 seconds
    void enableNodeScoring(bool enabled);           // Smart node selection
    
private:
    class Impl;
    std::unique_ptr<Impl> pImpl;
};

} // namespace IPLoop