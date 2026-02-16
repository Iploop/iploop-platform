#include "internal/WebSocketClient.h"
#include "internal/Logger.h"
#include "internal/Utils.h"

#include <thread>
#include <chrono>
#include <mutex>
#include <atomic>

// Windows WebSocket implementation
#ifdef _WIN32
#include <windows.h>
#include <winhttp.h>
#pragma comment(lib, "winhttp.lib")
#endif

namespace IPLoop {

class WebSocketClient::Impl {
public:
    Impl(const std::string& url) : 
        serverUrl(url),
        isConnected(false),
        shouldReconnect(true),
        reconnectAttempts(0),
        maxReconnectAttempts(15),
        baseReconnectDelayMs(1000),
        maxReconnectDelayMs(30000),
        slowReconnectDelayMs(600000)
    {
        connectionInfo.serverUrl = url;
    }
    
    ~Impl() {
        disconnect();
    }
    
    Result<bool> connect() {
        std::lock_guard<std::mutex> lock(mutex);
        
        if (isConnected) {
            return Result<bool>::Success(true);
        }
        
        try {
            // Start connection in background thread
            shouldReconnect = true;
            reconnectAttempts = 0;
            
            connectionThread = std::thread([this]() {
                connectionLoop();
            });
            
            return Result<bool>::Success(true);
            
        } catch (const std::exception& e) {
            return Result<bool>::Error(-1, "Failed to start connection: " + std::string(e.what()));
        }
    }
    
    void disconnect() {
        std::lock_guard<std::mutex> lock(mutex);
        
        shouldReconnect = false;
        
        if (connectionThread.joinable()) {
            connectionThread.join();
        }
        
        closeConnection();
        isConnected = false;
        connectionInfo.status = ConnectionStatus::DISCONNECTED;
    }
    
    bool sendMessage(const std::string& message) {
        std::lock_guard<std::mutex> lock(mutex);
        
        if (!isConnected) {
            Logger::warn("WebSocket", "Cannot send message: not connected");
            return false;
        }
        
        return sendMessageInternal(message);
    }
    
private:
    void connectionLoop() {
        while (shouldReconnect) {
            try {
                if (connectInternal()) {
                    isConnected = true;
                    connectionInfo.status = ConnectionStatus::CONNECTED;
                    connectionInfo.lastConnectTime = Utils::getCurrentTimestamp();
                    reconnectAttempts = 0;
                    
                    if (onConnected) {
                        onConnected();
                    }
                    
                    // Message receive loop
                    messageLoop();
                    
                } else {
                    handleConnectionFailure();
                }
                
            } catch (const std::exception& e) {
                Logger::error("WebSocket", "Connection error: " + std::string(e.what()));
                handleConnectionFailure();
            }
            
            // Cleanup after disconnection
            closeConnection();
            isConnected = false;
            connectionInfo.status = ConnectionStatus::DISCONNECTED;
            
            if (onDisconnected && shouldReconnect) {
                onDisconnected("Connection lost");
            }
        }
    }
    
    bool connectInternal() {
        Logger::info("WebSocket", "Connecting to " + serverUrl);
        
#ifdef _WIN32
        return connectWindows();
#else
        return connectGeneric();
#endif
    }
    
#ifdef _WIN32
    bool connectWindows() {
        // Parse URL
        auto urlInfo = parseWebSocketUrl(serverUrl);
        if (!urlInfo.isValid) {
            Logger::error("WebSocket", "Invalid WebSocket URL: " + serverUrl);
            return false;
        }
        
        // Initialize WinHTTP
        hSession = WinHttpOpen(L"IPLoopSDK/1.0", 
                             WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
                             WINHTTP_NO_PROXY_NAME, 
                             WINHTTP_NO_PROXY_BYPASS, 
                             0);
        
        if (!hSession) {
            Logger::error("WebSocket", "WinHttpOpen failed");
            return false;
        }
        
        // Create connection
        std::wstring hostW(urlInfo.host.begin(), urlInfo.host.end());
        hConnect = WinHttpConnect(hSession, hostW.c_str(), urlInfo.port, 0);
        
        if (!hConnect) {
            Logger::error("WebSocket", "WinHttpConnect failed");
            WinHttpCloseHandle(hSession);
            return false;
        }
        
        // Create WebSocket request
        std::wstring pathW(urlInfo.path.begin(), urlInfo.path.end());
        hRequest = WinHttpOpenRequest(hConnect, L"GET", pathW.c_str(),
                                    NULL, WINHTTP_NO_REFERER,
                                    WINHTTP_DEFAULT_ACCEPT_TYPES,
                                    urlInfo.secure ? WINHTTP_FLAG_SECURE : 0);
        
        if (!hRequest) {
            Logger::error("WebSocket", "WinHttpOpenRequest failed");
            return false;
        }
        
        // WebSocket upgrade headers
        LPCWSTR headers = L"Connection: Upgrade\r\n"
                         L"Upgrade: websocket\r\n"
                         L"Sec-WebSocket-Version: 13\r\n"
                         L"Sec-WebSocket-Key: x3JJHMbDL1EzLkh9GBhXDw==\r\n";
        
        // Send request
        if (!WinHttpSendRequest(hRequest, headers, -1, NULL, 0, 0, 0)) {
            Logger::error("WebSocket", "WinHttpSendRequest failed");
            return false;
        }
        
        // Receive response
        if (!WinHttpReceiveResponse(hRequest, NULL)) {
            Logger::error("WebSocket", "WinHttpReceiveResponse failed");
            return false;
        }
        
        // Check status code
        DWORD statusCode = 0;
        DWORD statusCodeSize = sizeof(statusCode);
        WinHttpQueryHeaders(hRequest, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
                          WINHTTP_HEADER_NAME_BY_INDEX, &statusCode, &statusCodeSize, 
                          WINHTTP_NO_HEADER_INDEX);
        
        if (statusCode != 101) { // WebSocket upgrade
            Logger::error("WebSocket", "WebSocket upgrade failed, status: " + std::to_string(statusCode));
            return false;
        }
        
        // Convert to WebSocket handle
        hWebSocket = WinHttpWebSocketCompleteUpgrade(hRequest, 0);
        if (!hWebSocket) {
            Logger::error("WebSocket", "WinHttpWebSocketCompleteUpgrade failed");
            return false;
        }
        
        Logger::info("WebSocket", "Connected successfully");
        return true;
    }
#endif
    
    bool connectGeneric() {
        // Fallback implementation for non-Windows or when WinHTTP is not available
        Logger::warn("WebSocket", "Generic WebSocket implementation not yet implemented");
        return false;
    }
    
    void messageLoop() {
#ifdef _WIN32
        char buffer[4096];
        DWORD bytesRead = 0;
        WINHTTP_WEB_SOCKET_BUFFER_TYPE bufferType;
        
        while (isConnected && shouldReconnect) {
            DWORD result = WinHttpWebSocketReceive(hWebSocket, buffer, sizeof(buffer),
                                                 &bytesRead, &bufferType);
            
            if (result == NO_ERROR) {
                if (bufferType == WINHTTP_WEB_SOCKET_UTF8_MESSAGE_BUFFER_TYPE ||
                    bufferType == WINHTTP_WEB_SOCKET_UTF8_FRAGMENT_BUFFER_TYPE) {
                    
                    std::string message(buffer, bytesRead);
                    if (onMessage) {
                        onMessage(message);
                    }
                    
                } else if (bufferType == WINHTTP_WEB_SOCKET_BINARY_MESSAGE_BUFFER_TYPE ||
                          bufferType == WINHTTP_WEB_SOCKET_BINARY_FRAGMENT_BUFFER_TYPE) {
                    
                    std::vector<uint8_t> binaryData(buffer, buffer + bytesRead);
                    if (onBinaryMessage) {
                        onBinaryMessage(binaryData);
                    }
                }
                
            } else if (result == ERROR_WINHTTP_OPERATION_CANCELLED) {
                Logger::info("WebSocket", "Receive cancelled");
                break;
                
            } else {
                Logger::error("WebSocket", "Receive error: " + std::to_string(result));
                break;
            }
        }
#endif
    }
    
    bool sendMessageInternal(const std::string& message) {
#ifdef _WIN32
        if (!hWebSocket) return false;
        
        DWORD result = WinHttpWebSocketSend(hWebSocket, 
                                          WINHTTP_WEB_SOCKET_UTF8_MESSAGE_BUFFER_TYPE,
                                          (PVOID)message.c_str(), 
                                          static_cast<DWORD>(message.length()));
        
        return result == NO_ERROR;
#else
        return false;
#endif
    }
    
    void closeConnection() {
#ifdef _WIN32
        if (hWebSocket) {
            WinHttpWebSocketClose(hWebSocket, WINHTTP_WEB_SOCKET_SUCCESS_CLOSE_STATUS, NULL, 0);
            WinHttpCloseHandle(hWebSocket);
            hWebSocket = NULL;
        }
        
        if (hRequest) {
            WinHttpCloseHandle(hRequest);
            hRequest = NULL;
        }
        
        if (hConnect) {
            WinHttpCloseHandle(hConnect);
            hConnect = NULL;
        }
        
        if (hSession) {
            WinHttpCloseHandle(hSession);
            hSession = NULL;
        }
#endif
    }
    
    void handleConnectionFailure() {
        reconnectAttempts++;
        
        if (reconnectAttempts <= maxReconnectAttempts) {
            // Fast reconnect with exponential backoff
            uint64_t delay = std::min(baseReconnectDelayMs * (1ULL << (reconnectAttempts - 1)), 
                                    maxReconnectDelayMs);
            
            Logger::info("WebSocket", "Reconnecting in " + std::to_string(delay) + "ms (attempt " + 
                        std::to_string(reconnectAttempts) + "/" + std::to_string(maxReconnectAttempts) + ")");
            
            std::this_thread::sleep_for(std::chrono::milliseconds(delay));
            
        } else {
            // Slow reconnect after fast attempts exhausted
            Logger::info("WebSocket", "Slow reconnect in " + std::to_string(slowReconnectDelayMs) + "ms");
            std::this_thread::sleep_for(std::chrono::milliseconds(slowReconnectDelayMs));
            reconnectAttempts = 0; // Reset for next cycle
        }
        
        connectionInfo.reconnectAttempts = reconnectAttempts;
    }
    
    struct UrlInfo {
        std::string host;
        int port;
        std::string path;
        bool secure;
        bool isValid;
    };
    
    UrlInfo parseWebSocketUrl(const std::string& url) {
        UrlInfo info = {};
        
        // Simple URL parsing for WebSocket URLs
        if (url.substr(0, 5) == "ws://") {
            info.secure = false;
            info.port = 80;
            auto hostPath = url.substr(5);
            
            auto slashPos = hostPath.find('/');
            if (slashPos != std::string::npos) {
                info.host = hostPath.substr(0, slashPos);
                info.path = hostPath.substr(slashPos);
            } else {
                info.host = hostPath;
                info.path = "/";
            }
            
        } else if (url.substr(0, 6) == "wss://") {
            info.secure = true;
            info.port = 443;
            auto hostPath = url.substr(6);
            
            auto slashPos = hostPath.find('/');
            if (slashPos != std::string::npos) {
                info.host = hostPath.substr(0, slashPos);
                info.path = hostPath.substr(slashPos);
            } else {
                info.host = hostPath;
                info.path = "/";
            }
        } else {
            info.isValid = false;
            return info;
        }
        
        // Check for port in host
        auto colonPos = info.host.find(':');
        if (colonPos != std::string::npos) {
            try {
                info.port = std::stoi(info.host.substr(colonPos + 1));
                info.host = info.host.substr(0, colonPos);
            } catch (...) {
                info.isValid = false;
                return info;
            }
        }
        
        info.isValid = !info.host.empty();
        return info;
    }
    
public:
    // Configuration
    std::string serverUrl;
    int maxReconnectAttempts;
    uint64_t baseReconnectDelayMs;
    uint64_t maxReconnectDelayMs;
    uint64_t slowReconnectDelayMs;
    
    // State
    std::atomic<bool> isConnected;
    std::atomic<bool> shouldReconnect;
    std::atomic<int> reconnectAttempts;
    ConnectionInfo connectionInfo;
    
    // Threading
    std::thread connectionThread;
    std::mutex mutex;
    
    // Callbacks
    ConnectedCallback onConnected;
    DisconnectedCallback onDisconnected;
    MessageCallback onMessage;
    ErrorCallback onError;
    std::function<void(const std::vector<uint8_t>&)> onBinaryMessage;  // v2.0: Binary callback
    
    // v2.0: Binary message support
    bool sendBinaryMessage(const std::vector<uint8_t>& data) {
        std::lock_guard<std::mutex> lock(mutex);
        
        if (!isConnected) {
            Logger::warn("WebSocket", "Cannot send binary message: not connected");
            return false;
        }
        
#ifdef _WIN32
        if (!hWebSocket) return false;
        DWORD result = WinHttpWebSocketSend(hWebSocket, 
                                          WINHTTP_WEB_SOCKET_BINARY_MESSAGE_BUFFER_TYPE,
                                          (PVOID)data.data(), 
                                          static_cast<DWORD>(data.size()));
        return result == NO_ERROR;
#else
        return false;
#endif
    }
    
#ifdef _WIN32
    // Windows WebSocket handles
    HINTERNET hSession = NULL;
    HINTERNET hConnect = NULL;
    HINTERNET hRequest = NULL;
    HINTERNET hWebSocket = NULL;
#endif
};

// WebSocketClient implementation
WebSocketClient::WebSocketClient(const std::string& serverUrl) : 
    pImpl(std::make_unique<Impl>(serverUrl)) {}

WebSocketClient::~WebSocketClient() = default;

void WebSocketClient::setOnConnected(ConnectedCallback callback) {
    pImpl->onConnected = callback;
}

void WebSocketClient::setOnDisconnected(DisconnectedCallback callback) {
    pImpl->onDisconnected = callback;
}

void WebSocketClient::setOnMessage(MessageCallback callback) {
    pImpl->onMessage = callback;
}

void WebSocketClient::setOnError(ErrorCallback callback) {
    pImpl->onError = callback;
}

Result<bool> WebSocketClient::connect() {
    return pImpl->connect();
}

void WebSocketClient::disconnect() {
    pImpl->disconnect();
}

bool WebSocketClient::isConnected() const {
    return pImpl->isConnected;
}

bool WebSocketClient::sendMessage(const std::string& message) {
    return pImpl->sendMessage(message);
}

ConnectionInfo WebSocketClient::getConnectionInfo() const {
    return pImpl->connectionInfo;
}

void WebSocketClient::setReconnectConfig(int maxAttempts, uint64_t baseDelayMs, 
                                        uint64_t maxDelayMs, uint64_t slowReconnectDelayMs) {
    pImpl->maxReconnectAttempts = maxAttempts;
    pImpl->baseReconnectDelayMs = baseDelayMs;
    pImpl->maxReconnectDelayMs = maxDelayMs;
    pImpl->slowReconnectDelayMs = slowReconnectDelayMs;
}

bool WebSocketClient::sendBinaryMessage(const std::vector<uint8_t>& data) {
    return pImpl->sendBinaryMessage(data);
}

void WebSocketClient::setOnBinaryMessage(std::function<void(const std::vector<uint8_t>&)> callback) {
    pImpl->onBinaryMessage = callback;
}

} // namespace IPLoop