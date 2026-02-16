#include <IPLoop/IPLoopSDK.h>
#include <iostream>
#include <thread>
#include <chrono>

int main() {
    std::cout << "IPLoop SDK Basic Example\n";
    std::cout << "Version: " << IPLoop::SDK::getVersion() << "\n\n";
    
    // Get SDK instance
    auto& sdk = IPLoop::SDK::getInstance();
    
    // Set callbacks for monitoring
    sdk.setStatusCallback([](IPLoop::SDKStatus oldStatus, IPLoop::SDKStatus newStatus) {
        std::cout << "Status changed from " << static_cast<int>(oldStatus) 
                  << " to " << static_cast<int>(newStatus) << "\n";
    });
    
    sdk.setBandwidthCallback([](const IPLoop::BandwidthStats& stats) {
        std::cout << "Stats: " << stats.totalRequests << " requests, " 
                  << stats.totalMB << " MB transferred\n";
    });
    
    sdk.setErrorCallback([](const IPLoop::ErrorInfo& error) {
        std::cout << "Error: " << error.message << "\n";
    });
    
    // Initialize SDK
    std::cout << "Initializing SDK...\n";
    if (!sdk.initialize("your_api_key_here")) {
        std::cerr << "Failed to initialize SDK\n";
        return 1;
    }
    
    // Set user consent (required for GDPR compliance)
    sdk.setUserConsent(true);
    
    // Start SDK
    std::cout << "Starting SDK...\n";
    bool startResult = false;
    
    sdk.start([&startResult](bool success, const std::string& message) {
        startResult = success;
        std::cout << "Start result: " << (success ? "Success" : "Failed") << " - " << message << "\n";
        
        if (success) {
            std::cout << "HTTP Proxy URL: " << IPLoop::SDK::getInstance().getHttpProxyUrl() << "\n";
            std::cout << "SOCKS5 Proxy URL: " << IPLoop::SDK::getInstance().getSocks5ProxyUrl() << "\n";
        }
    });
    
    // Wait for start to complete
    std::this_thread::sleep_for(std::chrono::seconds(2));
    
    if (startResult) {
        std::cout << "\nSDK is running. You can now use the proxy:\n";
        std::cout << "- HTTP proxy: " << sdk.getHttpProxyUrl() << "\n";
        std::cout << "- SOCKS5 proxy: " << sdk.getSocks5ProxyUrl() << "\n";
        std::cout << "\nPress Enter to stop...\n";
        std::cin.get();
        
        // Show final stats
        auto stats = sdk.getStats();
        std::cout << "\nFinal statistics:\n";
        std::cout << "- Total requests: " << stats.totalRequests << "\n";
        std::cout << "- Total bandwidth: " << stats.totalMB << " MB\n";
        std::cout << "- Active connections: " << stats.activeConnections << "\n";
        
    } else {
        std::cerr << "Failed to start SDK\n";
    }
    
    // Stop SDK
    std::cout << "\nStopping SDK...\n";
    sdk.stop([](bool success, const std::string& message) {
        std::cout << "Stop result: " << (success ? "Success" : "Failed") << " - " << message << "\n";
    });
    
    // Wait for stop to complete
    std::this_thread::sleep_for(std::chrono::seconds(1));
    
    std::cout << "Example completed.\n";
    return 0;
}