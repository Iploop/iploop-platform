#pragma once

#include "Types.h"
#include "ProxyConfig.h"
#include "Callbacks.h"

#ifdef IPLOOP_SHARED
    #ifdef IPLOOP_EXPORTS
        #define IPLOOP_API __declspec(dllexport)
    #else
        #define IPLOOP_API __declspec(dllimport)
    #endif
#else
    #define IPLOOP_API
#endif

namespace IPLoop {

/**
 * IPLoop SDK for Windows C++ - Main entry point
 * Thread-safe singleton providing residential proxy functionality
 * 
 * Architecture mirrors Android SDK:
 * - WebSocket connection to registration server
 * - Auto-reconnect with exponential backoff
 * - Enterprise proxy features (geo-targeting, sessions, profiles)
 * - Bandwidth tracking and statistics
 * - GDPR consent management
 */
class IPLOOP_API SDK {
public:
    /**
     * Get the singleton SDK instance
     */
    static SDK& getInstance();
    
    /**
     * Initialize the SDK with API key
     * @param apiKey Your IPLoop API key
     * @return true if successful, false if already initialized
     */
    bool initialize(const std::string& apiKey);
    
    /**
     * Initialize with custom server URL (for testing)
     */
    bool initialize(const std::string& apiKey, const std::string& serverUrl);
    
    /**
     * Check if SDK is initialized
     */
    bool isInitialized() const;
    
    /**
     * Start the SDK - begins WebSocket connection and proxy service
     * @param callback Optional callback for async result
     */
    void start(StatusCallback callback = nullptr);
    
    /**
     * Stop the SDK - closes all connections
     * @param callback Optional callback for async result
     */
    void stop(StatusCallback callback = nullptr);
    
    /**
     * Check if SDK is currently running
     */
    bool isRunning() const;
    
    /**
     * Get current SDK status
     */
    SDKStatus getStatus() const;
    
    /**
     * Set user consent for data usage (GDPR compliance)
     * @param consent true if user consents, false otherwise
     */
    void setUserConsent(bool consent);
    
    /**
     * Check if user has given consent
     */
    bool hasUserConsent() const;
    
    /**
     * Get current bandwidth statistics
     */
    BandwidthStats getStats() const;
    
    /**
     * Reset bandwidth statistics
     */
    void resetStats();
    
    /**
     * Enable/disable debug logging
     */
    void setLoggingEnabled(bool enabled);
    
    /**
     * Configure proxy settings for advanced features
     * @param config Proxy configuration object
     */
    void setProxyConfig(const ProxyConfig& config);
    
    /**
     * Get current proxy configuration
     */
    ProxyConfig getProxyConfig() const;
    
    /**
     * Generate proxy auth string for HTTP proxy usage
     * Format: "apikey-country-US-city-miami-session-sticky"
     * @param config Optional config override
     * @return Auth string for proxy username
     */
    std::string generateProxyAuth(const ProxyConfig* config = nullptr) const;
    
    /**
     * Get HTTP proxy URL for external applications
     * @return URL in format "http://user:auth@host:port"
     */
    std::string getHttpProxyUrl() const;
    
    /**
     * Get SOCKS5 proxy URL for external applications  
     * @return URL in format "socks5://user:auth@host:port"
     */
    std::string getSocks5ProxyUrl() const;
    
    /**
     * Set status change callback
     */
    void setStatusCallback(StatusChangeCallback callback);
    
    /**
     * Set bandwidth update callback
     */
    void setBandwidthCallback(BandwidthUpdateCallback callback);
    
    /**
     * Set error callback
     */
    void setErrorCallback(ErrorCallback callback);
    
    /**
     * Get SDK version
     */
    static std::string getVersion();
    
    /**
     * Get device information (for debugging)
     */
    std::string getDeviceInfo() const;
    
private:
    SDK();
    ~SDK();
    
    // Non-copyable
    SDK(const SDK&) = delete;
    SDK& operator=(const SDK&) = delete;
    
    class Impl;
    std::unique_ptr<Impl> pImpl;
};

} // namespace IPLoop

// C-style API for compatibility with other languages (outside namespace)
extern "C" {
    IPLOOP_API int IPLoop_Initialize(const char* apiKey);
    IPLOOP_API int IPLoop_Start();
    IPLOOP_API int IPLoop_Stop();
    IPLOOP_API int IPLoop_IsActive();
    IPLOOP_API void IPLoop_SetConsent(int consent);
    IPLOOP_API int IPLoop_GetTotalRequests();
    IPLOOP_API double IPLoop_GetTotalMB();
    IPLOOP_API const char* IPLoop_GetProxyURL();
    IPLOOP_API void IPLoop_SetCountry(const char* country);
    IPLOOP_API void IPLoop_SetCity(const char* city);
    IPLOOP_API const char* IPLoop_GetVersion();
}