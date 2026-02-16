#include "internal/BandwidthTracker.h"
#include "internal/Logger.h"
#include "internal/Utils.h"

#include <thread>
#include <mutex>
#include <atomic>
#include <chrono>

namespace IPLoop {

class BandwidthTracker::Impl {
public:
    Impl() : 
        isRunning(false),
        updateIntervalMs(5000),  // 5 seconds (matches Android)
        stats{}
    {
        stats.sessionStartTime = Utils::getCurrentTimestamp();
    }
    
    ~Impl() {
        stop();
    }
    
    void start() {
        std::lock_guard<std::mutex> lock(mutex);
        
        if (isRunning) return;
        
        isRunning = true;
        stats.sessionStartTime = Utils::getCurrentTimestamp();
        
        // Start update thread
        updateThread = std::thread([this]() {
            updateLoop();
        });
        
        Logger::info("BandwidthTracker", "v2.0 bandwidth tracker started");
    }
    
    void stop() {
        std::lock_guard<std::mutex> lock(mutex);
        
        if (!isRunning) return;
        
        isRunning = false;
        
        if (updateThread.joinable()) {
            updateThread.join();
        }
        
        Logger::info("BandwidthTracker", "v2.0 bandwidth tracker stopped");
    }
    
    BandwidthStats getStats() const {
        std::lock_guard<std::mutex> lock(mutex);
        BandwidthStats currentStats = stats;
        currentStats.updateTotalMB();
        return currentStats;
    }
    
    void reset() {
        std::lock_guard<std::mutex> lock(mutex);
        stats.reset();
        stats.sessionStartTime = Utils::getCurrentTimestamp();
        Logger::info("BandwidthTracker", "v2.0 statistics reset");
    }
    
    void recordRequest() {
        std::lock_guard<std::mutex> lock(mutex);
        stats.totalRequests++;
    }
    
    void recordBytesUp(uint64_t bytes) {
        std::lock_guard<std::mutex> lock(mutex);
        stats.totalBytesUp += bytes;
    }
    
    void recordBytesDown(uint64_t bytes) {
        std::lock_guard<std::mutex> lock(mutex);
        stats.totalBytesDown += bytes;
    }
    
    void recordConnectionOpened() {
        std::lock_guard<std::mutex> lock(mutex);
        stats.activeConnections++;
        stats.totalConnections++;
    }
    
    void recordConnectionClosed() {
        std::lock_guard<std::mutex> lock(mutex);
        if (stats.activeConnections > 0) {
            stats.activeConnections--;
        }
    }
    
private:
    void updateLoop() {
        while (isRunning) {
            std::this_thread::sleep_for(std::chrono::milliseconds(updateIntervalMs));
            
            if (!isRunning) break;
            
            // Trigger callback with current stats
            if (callback) {
                auto currentStats = getStats();
                callback(currentStats);
            }
        }
    }
    
public:
    std::atomic<bool> isRunning;
    uint32_t updateIntervalMs;
    BandwidthStats stats;
    BandwidthUpdateCallback callback;
    
    std::thread updateThread;
    mutable std::mutex mutex;
};

// BandwidthTracker implementation
BandwidthTracker::BandwidthTracker() : pImpl(std::make_unique<Impl>()) {}
BandwidthTracker::~BandwidthTracker() = default;

void BandwidthTracker::start() {
    pImpl->start();
}

void BandwidthTracker::stop() {
    pImpl->stop();
}

bool BandwidthTracker::isRunning() const {
    return pImpl->isRunning;
}

BandwidthStats BandwidthTracker::getStats() const {
    return pImpl->getStats();
}

void BandwidthTracker::reset() {
    pImpl->reset();
}

void BandwidthTracker::recordRequest() {
    pImpl->recordRequest();
}

void BandwidthTracker::recordBytesUp(uint64_t bytes) {
    pImpl->recordBytesUp(bytes);
}

void BandwidthTracker::recordBytesDown(uint64_t bytes) {
    pImpl->recordBytesDown(bytes);
}

void BandwidthTracker::recordConnectionOpened() {
    pImpl->recordConnectionOpened();
}

void BandwidthTracker::recordConnectionClosed() {
    pImpl->recordConnectionClosed();
}

void BandwidthTracker::setCallback(BandwidthUpdateCallback callback) {
    pImpl->callback = callback;
}

void BandwidthTracker::setUpdateInterval(uint32_t intervalMs) {
    pImpl->updateIntervalMs = intervalMs;
}

} // namespace IPLoop