#include "iploop_sdk.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <csignal>
#include <atomic>

// Global flag for graceful shutdown
std::atomic<bool> g_running{true};

void signalHandler(int signal) {
    std::cout << "\nReceived signal " << signal << ", shutting down..." << std::endl;
    g_running = false;
}

int main(int argc, char* argv[]) {
    std::cout << "IPLoop SDK Example - Version " << IPLoopSDK::getVersion() << std::endl;
    std::cout << "=========================================" << std::endl;
    
    // Set up signal handling for graceful shutdown
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);
    
    try {
        // Initialize SDK
        if (argc > 1) {
            std::string serverUrl = argv[1];
            std::cout << "Initializing with custom server: " << serverUrl << std::endl;
            IPLoopSDK::init(serverUrl);
        } else {
            std::cout << "Initializing with default server..." << std::endl;
            IPLoopSDK::init();
        }
        
        std::cout << "Node ID: " << IPLoopSDK::getNodeId() << std::endl;
        
        // Set log level (0=none, 1=error, 2=info, 3=debug)
        IPLoopSDK::setLogLevel(2); // Info level
        
        // Start SDK
        std::cout << "Starting SDK..." << std::endl;
        IPLoopSDK::start();
        
        // Main loop - show status every 10 seconds
        auto lastStats = std::chrono::steady_clock::now();
        int iterations = 0;
        
        while (g_running) {
            std::this_thread::sleep_for(std::chrono::seconds(1));
            
            auto now = std::chrono::steady_clock::now();
            if (std::chrono::duration_cast<std::chrono::seconds>(now - lastStats).count() >= 10) {
                // Show status
                bool connected = IPLoopSDK::isConnected();
                bool running = IPLoopSDK::isRunning();
                int tunnels = IPLoopSDK::getActiveTunnelCount();
                auto stats = IPLoopSDK::getConnectionStats();
                
                std::cout << std::endl;
                std::cout << "=== Status Report ===" << std::endl;
                std::cout << "Running: " << (running ? "YES" : "NO") << std::endl;
                std::cout << "Connected: " << (connected ? "YES" : "NO") << std::endl;
                std::cout << "Active Tunnels: " << tunnels << std::endl;
                std::cout << "Total Connections: " << stats.first << std::endl;
                std::cout << "Total Disconnections: " << stats.second << std::endl;
                std::cout << "Uptime: " << (++iterations * 10) << " seconds" << std::endl;
                std::cout << "===================" << std::endl;
                
                lastStats = now;
            }
        }
        
    } catch (const std::exception& e) {
        std::cerr << "ERROR: " << e.what() << std::endl;
        return 1;
    }
    
    // Stop SDK
    std::cout << "Stopping SDK..." << std::endl;
    IPLoopSDK::stop();
    
    std::cout << "SDK stopped. Goodbye!" << std::endl;
    return 0;
}