#pragma once

#include "../include/IPLoop/Types.h"
#include <string>
#include <memory>

namespace IPLoop {

/**
 * Device information gathering for v2.0 registration
 * Provides system details needed by IPLoop servers
 * Note: Named DeviceInfoGatherer to avoid collision with DeviceInfo struct in Types.h
 */
class DeviceInfoGatherer {
public:
    DeviceInfoGatherer();
    ~DeviceInfoGatherer();
    
    /**
     * Gather current device information
     */
    DeviceInfo gather() const;
    
    /**
     * Generate unique device ID based on hardware
     */
    std::string generateDeviceId() const;
    
    /**
     * Get cached device ID (persistent across app restarts)
     */
    std::string getCachedDeviceId() const;
    
private:
    class Impl;
    std::unique_ptr<Impl> pImpl;
};

} // namespace IPLoop