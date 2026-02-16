#pragma once

#include "../include/IPLoop/Types.h"
#include "../include/IPLoop/Callbacks.h"
#include <memory>

namespace IPLoop {

/**
 * Bandwidth tracking and statistics - matches Android SDK implementation
 */
class BandwidthTracker {
public:
    BandwidthTracker();
    ~BandwidthTracker();
    
    // Lifecycle
    void start();
    void stop();
    bool isRunning() const;
    
    // Statistics
    BandwidthStats getStats() const;
    void reset();
    
    // Update methods (called by tunnel manager)
    void recordRequest();
    void recordBytesUp(uint64_t bytes);
    void recordBytesDown(uint64_t bytes);
    void recordConnectionOpened();
    void recordConnectionClosed();
    
    // Callback for periodic updates
    void setCallback(BandwidthUpdateCallback callback);
    
    // Update interval (matches Android: every 5 seconds)
    void setUpdateInterval(uint32_t intervalMs);
    
private:
    class Impl;
    std::unique_ptr<Impl> pImpl;
};

} // namespace IPLoop