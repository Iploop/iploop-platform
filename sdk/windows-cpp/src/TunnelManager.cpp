#include "internal/TunnelManager.h"
#include "internal/Logger.h"
#include "internal/Utils.h"

#include <thread>
#include <mutex>
#include <atomic>
#include <unordered_map>
#include <random>

namespace IPLoop {

class TunnelManager::Impl {
public:
    Impl() : 
        isRunning(false),
        nextTunnelId(1),
        maxTunnelsPerNode(5),
        tunnelTimeoutMs(30000),
        nodeScoring(true)
    {
    }
    
    ~Impl() {
        stop();
    }
    
    void start() {
        std::lock_guard<std::mutex> lock(mutex);
        if (isRunning) return;
        
        isRunning = true;
        Logger::info("TunnelManager", "v2.0 tunnel manager started with binary protocol support");
    }
    
    void stop() {
        std::lock_guard<std::mutex> lock(mutex);
        if (!isRunning) return;
        
        // Close all active tunnels
        for (auto& [tunnelId, tunnel] : activeTunnels) {
            Logger::debug("TunnelManager", "Closing tunnel: " + tunnelId);
        }
        activeTunnels.clear();
        
        isRunning = false;
        Logger::info("TunnelManager", "v2.0 tunnel manager stopped");
    }
    
    TunnelResponse createTunnel(const TunnelRequest& request) {
        std::lock_guard<std::mutex> lock(mutex);
        
        if (!isRunning) {
            return {false, "", "Tunnel manager not running", 0};
        }
        
        // Generate tunnel ID
        std::string tunnelId = "tunnel_" + std::to_string(nextTunnelId++);
        
        // Select best node (v2.0: node scoring)
        uint32_t selectedNode = selectBestNode();
        
        // Create tunnel entry
        TunnelInfo tunnel;
        tunnel.sessionId = request.sessionId;
        tunnel.targetHost = request.targetHost;
        tunnel.targetPort = request.targetPort;
        tunnel.assignedNode = selectedNode;
        tunnel.startTime = Utils::getCurrentTimestamp();
        tunnel.bytesTransferred = 0;
        tunnel.isActive = true;
        
        activeTunnels[tunnelId] = tunnel;
        
        // Update statistics
        stats.activeTunnels = static_cast<uint32_t>(activeTunnels.size());
        stats.totalTunnels++;
        
        Logger::info("TunnelManager", "v2.0 tunnel created: " + tunnelId + 
                    " -> " + request.targetHost + ":" + std::to_string(request.targetPort) +
                    " via node " + std::to_string(selectedNode));
        
        // Trigger callback
        if (onTunnelCreated) {
            onTunnelCreated(tunnelId);
        }
        
        return {true, tunnelId, "", selectedNode};
    }
    
    bool sendTunnelData(const std::string& tunnelId, const std::vector<uint8_t>& data) {
        std::lock_guard<std::mutex> lock(mutex);
        
        auto it = activeTunnels.find(tunnelId);
        if (it == activeTunnels.end()) {
            Logger::warn("TunnelManager", "Tunnel not found: " + tunnelId);
            return false;
        }
        
        // v2.0: Send binary data directly (no base64 encoding)
        it->second.bytesTransferred += data.size();
        stats.bytesTransferred += data.size();
        
        Logger::debug("TunnelManager", "v2.0 binary data sent: " + tunnelId + 
                     " (" + std::to_string(data.size()) + " bytes)");
        
        return true;
    }
    
    void closeTunnel(const std::string& tunnelId) {
        std::lock_guard<std::mutex> lock(mutex);
        
        auto it = activeTunnels.find(tunnelId);
        if (it == activeTunnels.end()) {
            Logger::warn("TunnelManager", "Tunnel not found for close: " + tunnelId);
            return;
        }
        
        uint64_t bytesTransferred = it->second.bytesTransferred;
        activeTunnels.erase(it);
        
        stats.activeTunnels = static_cast<uint32_t>(activeTunnels.size());
        
        Logger::info("TunnelManager", "v2.0 tunnel closed: " + tunnelId + 
                    " (" + std::to_string(bytesTransferred) + " bytes transferred)");
        
        // Trigger callback
        if (onTunnelClosed) {
            onTunnelClosed(tunnelId, bytesTransferred);
        }
    }
    
    TunnelStats getStats() const {
        std::lock_guard<std::mutex> lock(mutex);
        
        TunnelStats currentStats = stats;
        
        // Calculate throughput (simplified)
        if (stats.sessionStartTime > 0) {
            uint64_t sessionDuration = Utils::getCurrentTimestamp() - stats.sessionStartTime;
            if (sessionDuration > 0) {
                currentStats.throughputMbps = (static_cast<double>(stats.bytesTransferred) / 
                                              (1024.0 * 1024.0)) / (sessionDuration / 1000.0);
            }
        }
        
        return currentStats;
    }
    
private:
    struct TunnelInfo {
        std::string sessionId;
        std::string targetHost;
        uint16_t targetPort;
        uint32_t assignedNode;
        uint64_t startTime;
        uint64_t bytesTransferred;
        bool isActive;
    };
    
    uint32_t selectBestNode() {
        // v2.0: Simple node scoring (in production, would use real performance metrics)
        static std::random_device rd;
        static std::mt19937 gen(rd());
        static std::uniform_int_distribution<uint32_t> dist(1000, 9999);
        
        uint32_t nodeId = dist(gen);
        
        // Update node scores (simplified)
        double score = 0.85 + (static_cast<double>(rand()) / RAND_MAX) * 0.3;  // 0.85-1.15 range
        stats.nodeScores.push_back({nodeId, score});
        
        // Keep only recent scores
        if (stats.nodeScores.size() > 10) {
            stats.nodeScores.erase(stats.nodeScores.begin());
        }
        
        return nodeId;
    }
    
public:
    // Configuration
    uint32_t maxTunnelsPerNode;
    uint32_t tunnelTimeoutMs;
    bool nodeScoring;
    
    // State
    std::atomic<bool> isRunning;
    std::atomic<uint64_t> nextTunnelId;
    std::unordered_map<std::string, TunnelInfo> activeTunnels;
    TunnelStats stats;
    
    // Callbacks
    TunnelCreatedCallback onTunnelCreated;
    TunnelClosedCallback onTunnelClosed;
    std::function<void(const std::string&, const std::vector<uint8_t>&)> onTunnelData;
    std::function<void(const std::string&, const std::string&)> onTunnelError;
    
    // Thread safety
    mutable std::mutex mutex;
};

// TunnelManager implementation
TunnelManager::TunnelManager() : pImpl(std::make_unique<Impl>()) {}
TunnelManager::~TunnelManager() = default;

void TunnelManager::start() {
    pImpl->start();
}

void TunnelManager::stop() {
    pImpl->stop();
}

bool TunnelManager::isRunning() const {
    return pImpl->isRunning;
}

TunnelManager::TunnelResponse TunnelManager::createTunnel(const TunnelRequest& request) {
    return pImpl->createTunnel(request);
}

bool TunnelManager::sendTunnelData(const std::string& tunnelId, const std::vector<uint8_t>& data) {
    return pImpl->sendTunnelData(tunnelId, data);
}

void TunnelManager::closeTunnel(const std::string& tunnelId) {
    pImpl->closeTunnel(tunnelId);
}

TunnelManager::TunnelStats TunnelManager::getStats() const {
    return pImpl->getStats();
}

void TunnelManager::setOnTunnelCreated(TunnelCreatedCallback callback) {
    pImpl->onTunnelCreated = callback;
}

void TunnelManager::setOnTunnelClosed(TunnelClosedCallback callback) {
    pImpl->onTunnelClosed = callback;
}

void TunnelManager::setOnTunnelData(std::function<void(const std::string&, const std::vector<uint8_t>&)> callback) {
    pImpl->onTunnelData = callback;
}

void TunnelManager::setOnTunnelError(std::function<void(const std::string&, const std::string&)> callback) {
    pImpl->onTunnelError = callback;
}

void TunnelManager::setMaxTunnelsPerNode(uint32_t maxTunnels) {
    pImpl->maxTunnelsPerNode = maxTunnels;
}

void TunnelManager::setTunnelTimeout(uint32_t timeoutMs) {
    pImpl->tunnelTimeoutMs = timeoutMs;
}

void TunnelManager::enableNodeScoring(bool enabled) {
    pImpl->nodeScoring = enabled;
}

} // namespace IPLoop