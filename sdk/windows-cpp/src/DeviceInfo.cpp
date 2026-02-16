#include "internal/DeviceInfo.h"
#include "internal/Utils.h"
#include <fstream>

namespace IPLoop {

class DeviceInfoGatherer::Impl {
public:
    Impl() {
        deviceId = getCachedDeviceId();
    }
    
    DeviceInfo gather() const {
        DeviceInfo info;
        
        info.deviceId = deviceId;
        info.osVersion = Utils::getOSVersion();
        info.architecture = Utils::getArchitecture();
        info.sdkVersion = "2.0.0";
        info.appName = "IPLoopSDK";
        info.appVersion = "2.0.0";
        info.networkType = "ethernet";  // Simplified
        info.ipAddress = Utils::getLocalIP();
        info.macAddress = Utils::getMACAddress();
        info.availableMemory = Utils::getAvailableMemoryMB();
        info.cpuCores = Utils::getCPUCores();
        
        return info;
    }
    
    std::string generateDeviceId() const {
        std::string hostname = Utils::getHostname();
        std::string mac = Utils::getMACAddress();
        std::string osVersion = Utils::getOSVersion();
        
        std::string combined = hostname + "|" + mac + "|" + osVersion;
        return "win_" + Utils::sha256(combined).substr(0, 16);
    }
    
    std::string getCachedDeviceId() const {
        std::string configPath = Utils::getAppDataPath() + "\\IPLoop\\device_id.txt";
        
        if (Utils::fileExists(configPath)) {
            std::string cached = Utils::readFile(configPath);
            if (!cached.empty()) {
                return Utils::trim(cached);
            }
        }
        
        std::string newId = generateDeviceId();
        Utils::writeFile(configPath, newId);
        
        return newId;
    }
    
private:
    std::string deviceId;
};

DeviceInfoGatherer::DeviceInfoGatherer() : pImpl(std::make_unique<Impl>()) {}
DeviceInfoGatherer::~DeviceInfoGatherer() = default;

DeviceInfo DeviceInfoGatherer::gather() const {
    return pImpl->gather();
}

std::string DeviceInfoGatherer::generateDeviceId() const {
    return pImpl->generateDeviceId();
}

std::string DeviceInfoGatherer::getCachedDeviceId() const {
    return pImpl->getCachedDeviceId();
}

} // namespace IPLoop
