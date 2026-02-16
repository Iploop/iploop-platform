#include "IPLoop/IPLoopSDK.h"
#include "internal/WebSocketClient.h"
#include "internal/TunnelManager.h"
#include "internal/BandwidthTracker.h"
#include "internal/DeviceInfo.h"
#include "internal/Logger.h"
#include "internal/Utils.h"

#include <thread>
#include <mutex>
#include <atomic>
#include <memory>
#include <chrono>

namespace IPLoop {

// SDK Implementation (PIMPL pattern)
class SDK::Impl {
public:
    Impl() : 
        status(SDKStatus::IDLE),
        isRunning(false),
        isInitialized(false),
        hasConsent(false),
        loggingEnabled(false),
        serverUrl("wss://gateway.iploop.io:9443/ws"),
        proxyConfig(ProxyConfig::createDefault())
    {
        Logger::getInstance().setCallback([this](LogLevel level, const std::string& tag, const std::string& message) {
            if (logCallback && loggingEnabled) {
                logCallback(level, tag, message);
            }
        });
    }
    
    ~Impl() {
        stop(nullptr);
    }
    
    bool initialize(const std::string& apiKey, const std::string& customServerUrl = "") {
        std::lock_guard<std::mutex> lock(mutex);
        
        if (isInitialized) {
            Logger::warn("SDK", "Already initialized, ignoring");
            return false;
        }
        
        this->apiKey = apiKey;
        if (!customServerUrl.empty()) {
            this->serverUrl = customServerUrl;
        }
        
        // Initialize components
        deviceInfo = std::make_unique<DeviceInfoGatherer>();
        bandwidthTracker = std::make_unique<BandwidthTracker>();
        webSocketClient = std::make_unique<WebSocketClient>(serverUrl);
        tunnelManager = std::make_unique<TunnelManager>();
        
        // Setup callbacks
        setupCallbacks();
        
        isInitialized = true;
        status = SDKStatus::IDLE;
        
        Logger::info("SDK", "Initialized with key: " + apiKey.substr(0, 8) + "***");
        return true;
    }
    
    void start(StatusCallback callback) {
        std::lock_guard<std::mutex> lock(mutex);
        
        if (!isInitialized) {
            if (callback) callback(false, "SDK not initialized");
            return;
        }
        
        if (isRunning) {
            if (callback) callback(true, "Already running");
            return;
        }
        
        if (!hasConsent) {
            if (callback) callback(false, "User consent required");
            return;
        }
        
        // Start in background thread
        std::thread([this, callback]() {
            startInternal(callback);
        }).detach();
    }
    
    void stop(StatusCallback callback) {
        std::lock_guard<std::mutex> lock(mutex);
        
        if (!isRunning) {
            if (callback) callback(true, "Already stopped");
            return;
        }
        
        // Stop in background thread
        std::thread([this, callback]() {
            stopInternal(callback);
        }).detach();
    }
    
    void setUserConsent(bool consent) {
        std::lock_guard<std::mutex> lock(mutex);
        hasConsent = consent;
        Logger::info("SDK", "User consent: " + std::string(consent ? "granted" : "revoked"));
    }
    
    std::string generateProxyAuth(const ProxyConfig* config = nullptr) const {
        const ProxyConfig& cfg = config ? *config : proxyConfig;
        return cfg.generateAuthString(apiKey);
    }
    
    std::string getHttpProxyUrl() const {
        auto auth = generateProxyAuth();
        return "http://user:" + auth + "@159.65.95.169:8880";  // v2.0 CONNECT proxy
    }
    
    std::string getSocks5ProxyUrl() const {
        auto auth = generateProxyAuth();
        return "socks5://user:" + auth + "@159.65.95.169:1080";  // v2.0 SOCKS5 proxy
    }
    
private:
    void setupCallbacks() {
        // WebSocket callbacks
        webSocketClient->setOnConnected([this]() {
            Logger::info("WebSocket", "Connected to server");
            setStatus(SDKStatus::CONNECTED);
            
            // Start tunnel manager
            tunnelManager->start();
        });
        
        webSocketClient->setOnDisconnected([this](const std::string& reason) {
            Logger::warn("WebSocket", "Disconnected: " + reason);
            setStatus(SDKStatus::RECONNECTING);
            
            // Stop tunnel manager
            tunnelManager->stop();
        });
        
        webSocketClient->setOnMessage([this](const std::string& message) {
            handleServerMessage(message);
        });
        
        webSocketClient->setOnError([this](const std::string& error) {
            Logger::error("WebSocket", "Error: " + error);
            if (errorCallback) {
                ErrorInfo errorInfo;
                errorInfo.code = -1;
                errorInfo.message = error;
                errorInfo.timestamp = Utils::getCurrentTimestamp();
                errorCallback(errorInfo);
            }
        });
        
        // Bandwidth tracking
        bandwidthTracker->setCallback([this](const BandwidthStats& stats) {
            if (bandwidthCallback) {
                bandwidthCallback(stats);
            }
        });
        
        // Tunnel manager callbacks
        tunnelManager->setOnTunnelCreated([this](const std::string& sessionId) {
            Logger::debug("Tunnel", "Created session: " + sessionId);
            if (tunnelCreatedCallback) {
                tunnelCreatedCallback(sessionId);
            }
        });
        
        tunnelManager->setOnTunnelClosed([this](const std::string& sessionId, uint64_t bytes) {
            Logger::debug("Tunnel", "Closed session: " + sessionId + " (" + std::to_string(bytes) + " bytes)");
            if (tunnelClosedCallback) {
                tunnelClosedCallback(sessionId, bytes);
            }
        });
    }
    
    void startInternal(StatusCallback callback) {
        try {
            setStatus(SDKStatus::CONNECTING);
            
            // Connect to WebSocket server
            auto connectResult = webSocketClient->connect();
            if (!connectResult.success) {
                setStatus(SDKStatus::ERROR);
                if (callback) callback(false, connectResult.error.message);
                return;
            }
            
            // Send device registration
            auto regMessage = createRegistrationMessage();
            webSocketClient->sendMessage(regMessage);
            
            isRunning = true;
            
            // Start bandwidth tracker
            bandwidthTracker->start();
            
            Logger::info("SDK", "Started successfully");
            if (callback) callback(true, "Started successfully");
            
        } catch (const std::exception& e) {
            setStatus(SDKStatus::ERROR);
            if (callback) callback(false, e.what());
        }
    }
    
    void stopInternal(StatusCallback callback) {
        try {
            setStatus(SDKStatus::STOPPING);
            
            // Stop components
            if (tunnelManager) {
                tunnelManager->stop();
            }
            
            if (bandwidthTracker) {
                bandwidthTracker->stop();
            }
            
            if (webSocketClient) {
                webSocketClient->disconnect();
            }
            
            isRunning = false;
            setStatus(SDKStatus::STOPPED);
            
            Logger::info("SDK", "Stopped successfully");
            if (callback) callback(true, "Stopped successfully");
            
        } catch (const std::exception& e) {
            if (callback) callback(false, e.what());
        }
    }
    
    void setStatus(SDKStatus newStatus) {
        SDKStatus oldStatus = status;
        status = newStatus;
        
        if (statusChangeCallback && oldStatus != newStatus) {
            statusChangeCallback(oldStatus, newStatus);
        }
    }
    
    std::string createRegistrationMessage() const {
        auto info = deviceInfo->gather();
        
        // Create JSON registration message (v2.0 format)
        std::string json = "{"
            "\"type\":\"register\","
            "\"device_id\":\"" + info.deviceId + "\","
            "\"api_key\":\"" + apiKey + "\","
            "\"os\":\"windows\","
            "\"os_version\":\"" + info.osVersion + "\","
            "\"architecture\":\"" + info.architecture + "\","
            "\"sdk_version\":\"2.0.0\","
            "\"app_name\":\"" + info.appName + "\","
            "\"network_type\":\"" + info.networkType + "\","
            "\"ip_address\":\"" + info.ipAddress + "\","
            "\"memory_mb\":" + std::to_string(info.availableMemory) + ","
            "\"cpu_cores\":" + std::to_string(info.cpuCores) + ","
            "\"protocol_version\":\"2.0\","           // v2.0: Binary protocol support
            "\"supports_binary\":true,"              // v2.0: Binary tunnel capability
            "\"max_tunnels\":5"                      // v2.0: Connection pool size
        "}";
        
        return json;
    }
    
    void handleServerMessage(const std::string& message) {
        // Parse and handle server messages
        Logger::debug("Server", "Message: " + message);
        
        // TODO: Parse JSON and handle different message types
        // - tunnel_request: Create new tunnel
        // - tunnel_close: Close existing tunnel
        // - config_update: Update configuration
        // - stats_request: Send statistics
    }
    
public:
    // State
    std::atomic<SDKStatus> status;
    std::atomic<bool> isRunning;
    std::atomic<bool> isInitialized;
    std::atomic<bool> hasConsent;
    std::atomic<bool> loggingEnabled;
    
    std::string apiKey;
    std::string serverUrl;
    ProxyConfig proxyConfig;
    
    // Components
    std::unique_ptr<DeviceInfoGatherer> deviceInfo;
    std::unique_ptr<BandwidthTracker> bandwidthTracker;
    std::unique_ptr<WebSocketClient> webSocketClient;
    std::unique_ptr<TunnelManager> tunnelManager;
    
    // Callbacks
    StatusChangeCallback statusChangeCallback;
    BandwidthUpdateCallback bandwidthCallback;
    ErrorCallback errorCallback;
    LogCallback logCallback;
    TunnelCreatedCallback tunnelCreatedCallback;
    TunnelClosedCallback tunnelClosedCallback;
    
    // Thread safety
    std::mutex mutex;
};

// SDK Implementation
SDK& SDK::getInstance() {
    static SDK instance;
    return instance;
}

SDK::SDK() : pImpl(std::make_unique<Impl>()) {}
SDK::~SDK() = default;

bool SDK::initialize(const std::string& apiKey) {
    return pImpl->initialize(apiKey);
}

bool SDK::initialize(const std::string& apiKey, const std::string& serverUrl) {
    return pImpl->initialize(apiKey, serverUrl);
}

bool SDK::isInitialized() const {
    return pImpl->isInitialized;
}

void SDK::start(StatusCallback callback) {
    pImpl->start(callback);
}

void SDK::stop(StatusCallback callback) {
    pImpl->stop(callback);
}

bool SDK::isRunning() const {
    return pImpl->isRunning;
}

SDKStatus SDK::getStatus() const {
    return pImpl->status;
}

void SDK::setUserConsent(bool consent) {
    pImpl->setUserConsent(consent);
}

bool SDK::hasUserConsent() const {
    return pImpl->hasConsent;
}

BandwidthStats SDK::getStats() const {
    return pImpl->bandwidthTracker ? pImpl->bandwidthTracker->getStats() : BandwidthStats{};
}

void SDK::resetStats() {
    if (pImpl->bandwidthTracker) {
        pImpl->bandwidthTracker->reset();
    }
}

void SDK::setLoggingEnabled(bool enabled) {
    pImpl->loggingEnabled = enabled;
}

void SDK::setProxyConfig(const ProxyConfig& config) {
    std::lock_guard<std::mutex> lock(pImpl->mutex);
    pImpl->proxyConfig = config;
}

ProxyConfig SDK::getProxyConfig() const {
    std::lock_guard<std::mutex> lock(pImpl->mutex);
    return pImpl->proxyConfig;
}

std::string SDK::generateProxyAuth(const ProxyConfig* config) const {
    return pImpl->generateProxyAuth(config);
}

std::string SDK::getHttpProxyUrl() const {
    return pImpl->getHttpProxyUrl();
}

std::string SDK::getSocks5ProxyUrl() const {
    return pImpl->getSocks5ProxyUrl();
}

void SDK::setStatusCallback(StatusChangeCallback callback) {
    pImpl->statusChangeCallback = callback;
}

void SDK::setBandwidthCallback(BandwidthUpdateCallback callback) {
    pImpl->bandwidthCallback = callback;
}

void SDK::setErrorCallback(ErrorCallback callback) {
    pImpl->errorCallback = callback;
}

std::string SDK::getVersion() {
    return "2.0.0";
}

std::string SDK::getDeviceInfo() const {
    if (pImpl->deviceInfo) {
        auto info = pImpl->deviceInfo->gather();
        return "Device: " + info.deviceId + ", OS: " + info.osVersion + ", Arch: " + info.architecture;
    }
    return "Device info not available";
}

} // namespace IPLoop

// C API Implementation (outside namespace)
static IPLoop::SDK* g_sdk = nullptr;

extern "C" {

int IPLoop_Initialize(const char* apiKey) {
    if (!g_sdk) {
        g_sdk = &IPLoop::SDK::getInstance();
    }
    return g_sdk->initialize(std::string(apiKey)) ? 0 : -1;
}

int IPLoop_Start() {
    if (!g_sdk) return -1;
    
    bool success = false;
    g_sdk->start([&success](bool result, const std::string&) {
        success = result;
    });
    
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
    
    return success ? 0 : -1;
}

int IPLoop_Stop() {
    if (!g_sdk) return -1;
    
    bool success = false;
    g_sdk->stop([&success](bool result, const std::string&) {
        success = result;
    });
    
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
    
    return success ? 0 : -1;
}

int IPLoop_IsActive() {
    if (!g_sdk) return 0;
    return g_sdk->isRunning() ? 1 : 0;
}

void IPLoop_SetConsent(int consent) {
    if (g_sdk) {
        g_sdk->setUserConsent(consent != 0);
    }
}

int IPLoop_GetTotalRequests() {
    if (!g_sdk) return 0;
    return static_cast<int>(g_sdk->getStats().totalRequests);
}

double IPLoop_GetTotalMB() {
    if (!g_sdk) return 0.0;
    return g_sdk->getStats().totalMB;
}

const char* IPLoop_GetProxyURL() {
    static std::string url;
    if (!g_sdk) return "";
    url = g_sdk->getHttpProxyUrl();
    return url.c_str();
}

void IPLoop_SetCountry(const char* country) {
    if (!g_sdk) return;
    auto config = g_sdk->getProxyConfig();
    config.setCountry(std::string(country));
    g_sdk->setProxyConfig(config);
}

void IPLoop_SetCity(const char* city) {
    if (!g_sdk) return;
    auto config = g_sdk->getProxyConfig();
    config.setCity(std::string(city));
    g_sdk->setProxyConfig(config);
}

const char* IPLoop_GetVersion() {
    static std::string version = "2.0.0";
    return version.c_str();
}

} // extern "C"