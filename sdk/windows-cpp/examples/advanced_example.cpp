#include <IPLoop/IPLoopSDK.h>
#include <iostream>
#include <thread>
#include <chrono>

int main() {
    std::cout << "IPLoop SDK Advanced Example - Enterprise Features\n";
    std::cout << "Version: " << IPLoop::SDK::getVersion() << "\n\n";
    
    auto& sdk = IPLoop::SDK::getInstance();
    
    // Set up comprehensive monitoring
    sdk.setStatusCallback([](IPLoop::SDKStatus oldStatus, IPLoop::SDKStatus newStatus) {
        std::cout << "[STATUS] Changed: " << static_cast<int>(oldStatus) 
                  << " -> " << static_cast<int>(newStatus) << std::endl;
    });
    
    sdk.setBandwidthCallback([](const IPLoop::BandwidthStats& stats) {
        std::cout << "[BANDWIDTH] Requests: " << stats.totalRequests 
                  << ", Up: " << (stats.totalBytesUp / 1024) << "KB"
                  << ", Down: " << (stats.totalBytesDown / 1024) << "KB"
                  << ", Active: " << stats.activeConnections << std::endl;
    });
    
    sdk.setErrorCallback([](const IPLoop::ErrorInfo& error) {
        std::cout << "[ERROR] " << error.message << " (Code: " << error.code << ")" << std::endl;
    });
    
    // Initialize with custom server (optional)
    std::cout << "Initializing SDK...\n";
    if (!sdk.initialize("your_api_key_here")) {
        std::cerr << "Failed to initialize SDK\n";
        return 1;
    }
    
    // Configure advanced proxy settings
    std::cout << "Configuring enterprise proxy settings...\n";
    
    auto proxyConfig = IPLoop::ProxyConfig::createDefault()
        .setCountry("US")                    // Target US proxies
        .setCity("miami")                    // Specifically Miami
        .setSessionType("sticky")            // Sticky sessions
        .setLifetime(60)                     // 1 hour session lifetime
        .setProfile("chrome-win")            // Chrome Windows profile
        .setMinSpeed(50)                     // Minimum 50 Mbps
        .setMaxLatency(200)                  // Maximum 200ms latency
        .setDebugMode(true);                 // Enable debug logging
    
    // Validate configuration
    if (!proxyConfig.isValid()) {
        std::cerr << "Invalid proxy configuration\n";
        return 1;
    }
    
    sdk.setProxyConfig(proxyConfig);
    sdk.setUserConsent(true);
    sdk.setLoggingEnabled(true);
    
    std::cout << "Proxy configuration:\n";
    std::cout << "- Country: " << proxyConfig.country << "\n";
    std::cout << "- City: " << proxyConfig.city << "\n";
    std::cout << "- Session type: " << proxyConfig.sessionType << "\n";
    std::cout << "- Lifetime: " << proxyConfig.lifetimeMinutes << " minutes\n";
    std::cout << "- Profile: " << proxyConfig.profile << "\n\n";
    
    // Generate custom auth string
    auto authString = sdk.generateProxyAuth();
    std::cout << "Generated auth string: " << authString << "\n\n";
    
    // Start SDK
    std::cout << "Starting SDK with enterprise configuration...\n";
    bool startResult = false;
    
    sdk.start([&startResult](bool success, const std::string& message) {
        startResult = success;
        std::cout << "[START] " << (success ? "SUCCESS" : "FAILED") << ": " << message << "\n";
    });
    
    // Wait for startup
    std::this_thread::sleep_for(std::chrono::seconds(3));
    
    if (startResult && sdk.isRunning()) {
        std::cout << "\n=== SDK RUNNING ===\n";
        std::cout << "HTTP Proxy: " << sdk.getHttpProxyUrl() << "\n";
        std::cout << "SOCKS5 Proxy: " << sdk.getSocks5ProxyUrl() << "\n";
        std::cout << "Status: " << static_cast<int>(sdk.getStatus()) << "\n";
        std::cout << "Device Info: " << sdk.getDeviceInfo() << "\n\n";
        
        std::cout << "Testing different configurations...\n\n";
        
        // Test configuration changes
        std::this_thread::sleep_for(std::chrono::seconds(2));
        
        // Change to UK proxies
        std::cout << "Switching to UK proxies...\n";
        proxyConfig.setCountry("GB").setCity("london");
        sdk.setProxyConfig(proxyConfig);
        
        std::cout << "New HTTP URL: " << sdk.getHttpProxyUrl() << "\n";
        std::cout << "New auth string: " << sdk.generateProxyAuth() << "\n\n";
        
        std::this_thread::sleep_for(std::chrono::seconds(2));
        
        // Change to rotating sessions
        std::cout << "Switching to rotating sessions...\n";
        proxyConfig.setSessionType("rotating").setRotateInterval(5);
        sdk.setProxyConfig(proxyConfig);
        
        std::cout << "New auth string: " << sdk.generateProxyAuth() << "\n\n";
        
        // Run for a while to collect stats
        std::cout << "Running for 10 seconds to collect statistics...\n";
        for (int i = 10; i > 0; i--) {
            std::cout << "Time remaining: " << i << "s\r";
            std::cout.flush();
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }
        std::cout << "\n\n";
        
        // Show comprehensive stats
        auto stats = sdk.getStats();
        std::cout << "=== FINAL STATISTICS ===\n";
        std::cout << "Total requests: " << stats.totalRequests << "\n";
        std::cout << "Total bandwidth: " << stats.totalMB << " MB\n";
        std::cout << "Bytes uploaded: " << stats.totalBytesUp << "\n";
        std::cout << "Bytes downloaded: " << stats.totalBytesDown << "\n";
        std::cout << "Active connections: " << stats.activeConnections << "\n";
        std::cout << "Total connections: " << stats.totalConnections << "\n";
        std::cout << "Session start time: " << stats.sessionStartTime << "\n\n";
        
    } else {
        std::cerr << "Failed to start SDK or not running\n";
    }
    
    // Graceful shutdown
    std::cout << "Shutting down SDK...\n";
    sdk.stop([](bool success, const std::string& message) {
        std::cout << "[STOP] " << (success ? "SUCCESS" : "FAILED") << ": " << message << "\n";
    });
    
    std::this_thread::sleep_for(std::chrono::seconds(2));
    
    std::cout << "Advanced example completed.\n";
    return 0;
}