#include <IPLoop/IPLoopSDK.h>
#include <iostream>
#include <vector>
#include <thread>
#include <chrono>

/**
 * IPLoop SDK v2.0 Binary Protocol Example
 * Demonstrates the new binary tunnel capabilities
 */
int main() {
    std::cout << "IPLoop SDK v2.0 Binary Protocol Example\n";
    std::cout << "Version: " << IPLoop::SDK::getVersion() << "\n\n";
    
    auto& sdk = IPLoop::SDK::getInstance();
    
    // v2.0: Enhanced monitoring with binary protocol stats
    sdk.setStatusCallback([](IPLoop::SDKStatus oldStatus, IPLoop::SDKStatus newStatus) {
        std::cout << "[STATUS v2.0] " << static_cast<int>(oldStatus) 
                  << " -> " << static_cast<int>(newStatus) << std::endl;
    });
    
    sdk.setBandwidthCallback([](const IPLoop::BandwidthStats& stats) {
        std::cout << "[BANDWIDTH v2.0] " << stats.totalRequests << " requests, "
                  << stats.totalMB << " MB, " << stats.activeConnections 
                  << " active connections" << std::endl;
    });
    
    // Initialize with production v2.0 endpoints
    std::cout << "Initializing v2.0 SDK...\n";
    if (!sdk.initialize("your_api_key_here")) {
        std::cerr << "Failed to initialize SDK\n";
        return 1;
    }
    
    // Configure for high-performance v2.0 features
    auto config = IPLoop::ProxyConfig::createDefault()
        .setCountry("US")
        .setCity("miami") 
        .setSessionType("sticky")
        .setLifetime(60)
        .setProfile("chrome-win")
        .setDebugMode(true);
    
    sdk.setProxyConfig(config);
    sdk.setUserConsent(true);
    sdk.setLoggingEnabled(true);
    
    std::cout << "v2.0 Configuration:\n";
    std::cout << "- Binary protocol: enabled\n";
    std::cout << "- Production endpoint: wss://159.65.95.169:9443/ws\n";
    std::cout << "- CONNECT proxy: 159.65.95.169:8880\n";
    std::cout << "- Auth string: " << sdk.generateProxyAuth() << "\n\n";
    
    // Start v2.0 SDK
    std::cout << "Starting v2.0 SDK...\n";
    bool started = false;
    
    sdk.start([&started](bool success, const std::string& message) {
        started = success;
        std::cout << "[v2.0 START] " << (success ? "SUCCESS" : "FAILED") 
                  << ": " << message << "\n";
                  
        if (success) {
            std::cout << "v2.0 Proxy URLs:\n";
            std::cout << "- HTTP: " << IPLoop::SDK::getInstance().getHttpProxyUrl() << "\n";
            std::cout << "- SOCKS5: " << IPLoop::SDK::getInstance().getSocks5ProxyUrl() << "\n";
        }
    });
    
    // Wait for v2.0 connection
    std::this_thread::sleep_for(std::chrono::seconds(5));
    
    if (started && sdk.isRunning()) {
        std::cout << "\n=== v2.0 SDK RUNNING ===\n";
        std::cout << "Device info: " << sdk.getDeviceInfo() << "\n";
        std::cout << "Status: " << static_cast<int>(sdk.getStatus()) << "\n\n";
        
        // Test different v2.0 configurations
        std::cout << "Testing v2.0 enterprise features...\n";
        
        // Test country switching with v2.0 binary protocol
        config.setCountry("DE").setCity("berlin");
        sdk.setProxyConfig(config);
        std::cout << "Switched to Germany - Auth: " << sdk.generateProxyAuth() << "\n";
        
        std::this_thread::sleep_for(std::chrono::seconds(2));
        
        // Test session management
        config.setSessionType("rotating").setRotateInterval(10);
        sdk.setProxyConfig(config);
        std::cout << "Enabled rotation - Auth: " << sdk.generateProxyAuth() << "\n";
        
        // Monitor v2.0 performance for 15 seconds
        std::cout << "\nMonitoring v2.0 binary protocol performance...\n";
        for (int i = 15; i > 0; i--) {
            auto stats = sdk.getStats();
            std::cout << "[" << i << "s] Requests: " << stats.totalRequests 
                      << ", Bandwidth: " << stats.totalMB << " MB"
                      << ", Active: " << stats.activeConnections
                      << ", Total: " << stats.totalConnections << "\r";
            std::cout.flush();
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }
        std::cout << "\n\n";
        
        // Final v2.0 statistics
        auto finalStats = sdk.getStats();
        std::cout << "=== v2.0 FINAL STATISTICS ===\n";
        std::cout << "Protocol version: 2.0 (Binary)\n";
        std::cout << "Total requests: " << finalStats.totalRequests << "\n";
        std::cout << "Total bandwidth: " << finalStats.totalMB << " MB\n";
        std::cout << "Bytes up: " << finalStats.totalBytesUp << "\n";
        std::cout << "Bytes down: " << finalStats.totalBytesDown << "\n";
        std::cout << "Peak connections: " << finalStats.totalConnections << "\n";
        std::cout << "Session uptime: " << 
            ((std::chrono::duration_cast<std::chrono::seconds>(
                std::chrono::system_clock::now().time_since_epoch()).count()) - 
            finalStats.sessionStartTime) << " seconds\n\n";
        
    } else {
        std::cerr << "v2.0 SDK failed to start or not running\n";
    }
    
    // Shutdown v2.0 SDK
    std::cout << "Stopping v2.0 SDK...\n";
    sdk.stop([](bool success, const std::string& message) {
        std::cout << "[v2.0 STOP] " << (success ? "SUCCESS" : "FAILED") 
                  << ": " << message << "\n";
    });
    
    std::this_thread::sleep_for(std::chrono::seconds(2));
    
    std::cout << "v2.0 Binary Protocol Example completed.\n";
    std::cout << "Features demonstrated:\n";
    std::cout << "✅ Binary tunnel protocol (no base64 overhead)\n";
    std::cout << "✅ Production endpoints (159.65.95.169)\n";
    std::cout << "✅ CONNECT proxy support\n";
    std::cout << "✅ Enterprise geo-targeting\n";
    std::cout << "✅ Session management\n";
    std::cout << "✅ Advanced statistics\n";
    
    return 0;
}