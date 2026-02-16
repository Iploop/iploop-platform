#include "tunnel.h"
#include "utils.h"
#include <algorithm>
#include <chrono>
#include <sstream>
#include <regex>
#include <wininet.h>

#pragma comment(lib, "ws2_32.lib")
#pragma comment(lib, "wininet.lib")

namespace iploop {

// ── TunnelConnection implementation ──

TunnelConnection::TunnelConnection(const std::string& tunnelId, const std::string& host, int port)
    : tunnelId_(tunnelId), host_(host), port_(port), socket_(INVALID_SOCKET), 
      connected_(false), closed_(false) {
}

TunnelConnection::~TunnelConnection() {
    close();
}

bool TunnelConnection::connect(int timeoutMs) {
    if (connected_ || closed_) return false;
    
    // Create socket
    socket_ = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
    if (socket_ == INVALID_SOCKET) {
        utils::Logger::error("Failed to create tunnel socket for " + tunnelId_.substr(0, 8));
        return false;
    }
    
    // Set socket options
    int flag = 1;
    setsockopt(socket_, IPPROTO_TCP, TCP_NODELAY, reinterpret_cast<char*>(&flag), sizeof(flag));
    setsockopt(socket_, SOL_SOCKET, SO_KEEPALIVE, reinterpret_cast<char*>(&flag), sizeof(flag));
    setsockopt(socket_, SOL_SOCKET, SO_RCVTIMEO, reinterpret_cast<char*>(&timeoutMs), sizeof(timeoutMs));
    setsockopt(socket_, SOL_SOCKET, SO_SNDTIMEO, reinterpret_cast<char*>(&timeoutMs), sizeof(timeoutMs));
    
    // Resolve hostname
    struct addrinfo hints = {0}, *result = nullptr;
    hints.ai_family = AF_INET;
    hints.ai_socktype = SOCK_STREAM;
    
    if (getaddrinfo(host_.c_str(), std::to_string(port_).c_str(), &hints, &result) != 0) {
        utils::Logger::error("Failed to resolve " + host_ + " for tunnel " + tunnelId_.substr(0, 8));
        closesocket(socket_);
        socket_ = INVALID_SOCKET;
        return false;
    }
    
    // Connect to target
    bool connectSuccess = false;
    for (struct addrinfo* addr = result; addr != nullptr; addr = addr->ai_next) {
        if (::connect(socket_, addr->ai_addr, static_cast<int>(addr->ai_addrlen)) == 0) {
            connectSuccess = true;
            break;
        }
    }
    freeaddrinfo(result);
    
    if (!connectSuccess) {
        utils::Logger::error("Failed to connect to " + host_ + ":" + std::to_string(port_) + 
                           " for tunnel " + tunnelId_.substr(0, 8));
        closesocket(socket_);
        socket_ = INVALID_SOCKET;
        return false;
    }
    
    connected_ = true;
    utils::Logger::info("Tunnel " + tunnelId_.substr(0, 8) + " connected to " + host_ + ":" + std::to_string(port_));
    return true;
}

void TunnelConnection::close() {
    if (closed_) return;
    
    closed_ = true;
    connected_ = false;
    
    if (socket_ != INVALID_SOCKET) {
        shutdown(socket_, SD_BOTH);
        closesocket(socket_);
        socket_ = INVALID_SOCKET;
    }
    
    if (readThread_.joinable()) {
        if (readThread_.get_id() == std::this_thread::get_id()) {
            readThread_.detach();  // Can't join ourselves — called from read thread via EOF handler
        } else {
            readThread_.join();
        }
    }
    
    utils::Logger::info("Tunnel " + tunnelId_.substr(0, 8) + " closed");
}

bool TunnelConnection::isConnected() const {
    return connected_ && !closed_;
}

bool TunnelConnection::writeData(const unsigned char* data, size_t len) {
    if (!connected_ || closed_ || socket_ == INVALID_SOCKET) return false;
    
    std::lock_guard<std::mutex> lock(writeMutex_);
    
    size_t totalSent = 0;
    while (totalSent < len) {
        int sent = send(socket_, reinterpret_cast<const char*>(data + totalSent), 
                       static_cast<int>(len - totalSent), 0);
        if (sent <= 0) {
            utils::Logger::error("Tunnel " + tunnelId_.substr(0, 8) + " write error: " + std::to_string(WSAGetLastError()));
            return false;
        }
        totalSent += sent;
    }
    
    return true;
}

void TunnelConnection::startReading(std::function<void(const std::string&, const unsigned char*, size_t, bool)> dataHandler) {
    if (readThread_.joinable() || !connected_) return;
    
    readThread_ = std::thread([this, dataHandler]() {
        readLoop(dataHandler);
    });
}

void TunnelConnection::stopReading() {
    if (readThread_.joinable()) {
        closed_ = true;
        if (socket_ != INVALID_SOCKET) {
            shutdown(socket_, SD_RECEIVE);
        }
        readThread_.join();
    }
}

void TunnelConnection::readLoop(std::function<void(const std::string&, const unsigned char*, size_t, bool)> dataHandler) {
    const int bufferSize = 65536; // 64KB buffer
    std::vector<unsigned char> buffer(bufferSize);
    
    while (connected_ && !closed_ && socket_ != INVALID_SOCKET) {
        int received = recv(socket_, reinterpret_cast<char*>(buffer.data()), bufferSize, 0);
        
        if (received > 0) {
            dataHandler(tunnelId_, buffer.data(), received, false);
        } else if (received == 0) {
            // EOF from target
            utils::Logger::info("Tunnel " + tunnelId_.substr(0, 8) + " target EOF");
            dataHandler(tunnelId_, nullptr, 0, true);
            break;
        } else {
            int error = WSAGetLastError();
            if (error != WSAETIMEDOUT && !closed_) {
                utils::Logger::error("Tunnel " + tunnelId_.substr(0, 8) + " read error: " + std::to_string(error));
            }
            break;
        }
    }
    
    // Don't call dataHandler again here — EOF already sent from inside the loop.
    // Double-calling caused duplicate closeTunnel and potential races.
}

// ── TunnelManager implementation ──

TunnelManager::TunnelManager() {
    // Initialize Winsock if not already done
    WSADATA wsaData;
    WSAStartup(MAKEWORD(2, 2), &wsaData);
}

TunnelManager::~TunnelManager() {
    closeAllTunnels();
}

void TunnelManager::setDataHandler(const DataHandler& handler) {
    dataHandler_ = handler;
}

void TunnelManager::setResponseHandler(const ResponseHandler& handler) {
    responseHandler_ = handler;
}

void TunnelManager::openTunnel(const std::string& tunnelId, const std::string& host, int port, int timeoutMs) {
    std::thread([this, tunnelId, host, port, timeoutMs]() {
        auto tunnel = std::make_shared<TunnelConnection>(tunnelId, host, port);
        
        if (tunnel->connect(timeoutMs)) {
            {
                std::lock_guard<std::mutex> lock(tunnelsMutex_);
                activeTunnels_[tunnelId] = tunnel;
            }
            
            if (responseHandler_) {
                responseHandler_(tunnelId, true, "");
            }
            
            // Start reading from target
            tunnel->startReading([this](const std::string& id, const unsigned char* data, size_t len, bool isEof) {
                handleTunnelData(id, data, len, isEof);
            });
        } else {
            if (responseHandler_) {
                responseHandler_(tunnelId, false, "Failed to connect to " + host + ":" + std::to_string(port));
            }
        }
    }).detach();
}

bool TunnelManager::writeTunnelData(const std::string& tunnelId, const unsigned char* data, size_t len) {
    std::lock_guard<std::mutex> lock(tunnelsMutex_);
    auto it = activeTunnels_.find(tunnelId);
    if (it == activeTunnels_.end()) {
        // Check if recently closed (race condition)
        auto recentIt = recentlyClosedTunnels_.find(tunnelId);
        if (recentIt != recentlyClosedTunnels_.end()) {
            return false; // Silently ignore
        }
        utils::Logger::debug("Data for unknown tunnel: " + tunnelId.substr(0, 8));
        return false;
    }
    
    return it->second->writeData(data, len);
}

void TunnelManager::closeTunnel(const std::string& tunnelId) {
    std::shared_ptr<TunnelConnection> tunnel;
    
    {
        std::lock_guard<std::mutex> lock(tunnelsMutex_);
        auto it = activeTunnels_.find(tunnelId);
        if (it != activeTunnels_.end()) {
            tunnel = it->second;
            activeTunnels_.erase(it);
            recentlyClosedTunnels_[tunnelId] = utils::Timer::nowMs();
        }
    }
    
    if (tunnel) {
        tunnel->close();
        utils::Logger::info("Closed tunnel " + tunnelId.substr(0, 8) + ". Active: " + std::to_string(getActiveTunnelCount()));
    }
    
    cleanupRecentlyClosed();
}

void TunnelManager::closeAllTunnels() {
    std::vector<std::shared_ptr<TunnelConnection>> tunnels;
    
    {
        std::lock_guard<std::mutex> lock(tunnelsMutex_);
        for (auto& pair : activeTunnels_) {
            tunnels.push_back(pair.second);
        }
        int count = static_cast<int>(activeTunnels_.size());
        activeTunnels_.clear();
        if (count > 0) {
            utils::Logger::info("Closing all " + std::to_string(count) + " tunnels");
        }
    }
    
    // Close tunnels outside of lock to avoid deadlock
    for (auto& tunnel : tunnels) {
        tunnel->close();
    }
}

int TunnelManager::getActiveTunnelCount() const {
    std::lock_guard<std::mutex> lock(tunnelsMutex_);
    return static_cast<int>(activeTunnels_.size());
}

std::vector<std::string> TunnelManager::getActiveTunnelIds() const {
    std::lock_guard<std::mutex> lock(tunnelsMutex_);
    std::vector<std::string> ids;
    ids.reserve(activeTunnels_.size());
    for (const auto& pair : activeTunnels_) {
        ids.push_back(pair.first);
    }
    return ids;
}

void TunnelManager::cleanupRecentlyClosed() {
    std::lock_guard<std::mutex> lock(tunnelsMutex_);
    long long now = utils::Timer::nowMs();
    auto it = recentlyClosedTunnels_.begin();
    while (it != recentlyClosedTunnels_.end()) {
        if (now - it->second > 10000) { // 10 seconds
            it = recentlyClosedTunnels_.erase(it);
        } else {
            ++it;
        }
    }
}

void TunnelManager::handleTunnelData(const std::string& tunnelId, const unsigned char* data, size_t len, bool isEof) {
    if (dataHandler_) {
        dataHandler_(tunnelId, data, len, isEof);
    }
    
    if (isEof) {
        // Must not call closeTunnel on the read thread (self-join crash).
        // Spawn a cleanup thread instead.
        std::thread([this, tunnelId]() {
            closeTunnel(tunnelId);
        }).detach();
    }
}

// ── ProxyHandler implementation ──

ProxyHandler::ProxyHandler() {
    // Initialize Winsock if not already done
    WSADATA wsaData;
    WSAStartup(MAKEWORD(2, 2), &wsaData);
}

ProxyHandler::~ProxyHandler() = default;

void ProxyHandler::setResponseHandler(const ResponseHandler& handler) {
    responseHandler_ = handler;
}

void ProxyHandler::handleProxyRequest(const std::string& requestId, const std::string& method,
                                     const std::string& url, const std::string& headers,
                                     const std::string& bodyBase64, int timeoutMs) {
    std::thread([this, requestId, method, url, headers, bodyBase64, timeoutMs]() {
        long long latencyMs = 0;
        
        try {
            std::string response = makeHttpRequest(method, url, headers, bodyBase64, timeoutMs, latencyMs);
            
            if (!response.empty()) {
                // Extract status code from response
                int statusCode = 200;
                if (response.find("HTTP/") == 0) {
                    size_t spacePos = response.find(' ');
                    if (spacePos != std::string::npos) {
                        std::string statusStr = response.substr(spacePos + 1, 3);
                        try {
                            statusCode = std::stoi(statusStr);
                        } catch (...) {
                            statusCode = 200;
                        }
                    }
                }
                
                // Find body (after \r\n\r\n)
                size_t bodyStart = response.find("\r\n\r\n");
                std::string body = (bodyStart != std::string::npos) ? 
                                  response.substr(bodyStart + 4) : response;
                
                std::string bodyBase64 = utils::Base64::encode(body);
                
                if (responseHandler_) {
                    responseHandler_(requestId, true, statusCode, bodyBase64, latencyMs, "");
                }
                
                utils::Logger::info("Proxy " + requestId.substr(0, 8) + " → " + 
                                  std::to_string(statusCode) + " (" + std::to_string(latencyMs) + 
                                  "ms, " + std::to_string(body.length()) + "B)");
            } else {
                if (responseHandler_) {
                    responseHandler_(requestId, false, 0, "", latencyMs, "Request failed");
                }
            }
        } catch (const std::exception& e) {
            if (responseHandler_) {
                responseHandler_(requestId, false, 0, "", latencyMs, e.what());
            }
            utils::Logger::error("Proxy " + requestId.substr(0, 8) + " failed: " + e.what());
        }
    }).detach();
}

std::string ProxyHandler::makeHttpRequest(const std::string& method, const std::string& url,
                                        const std::string& headers, const std::string& bodyBase64,
                                        int timeoutMs, long long& latencyMs) {
    auto startTime = std::chrono::steady_clock::now();
    
    HINTERNET hInternet = InternetOpenA("IPLoop-SDK/2.0", INTERNET_OPEN_TYPE_PRECONFIG, NULL, NULL, 0);
    if (!hInternet) {
        throw std::runtime_error("Failed to initialize WinINet");
    }
    
    // Set timeouts
    InternetSetOptionA(hInternet, INTERNET_OPTION_CONNECT_TIMEOUT, &timeoutMs, sizeof(timeoutMs));
    InternetSetOptionA(hInternet, INTERNET_OPTION_RECEIVE_TIMEOUT, &timeoutMs, sizeof(timeoutMs));
    InternetSetOptionA(hInternet, INTERNET_OPTION_SEND_TIMEOUT, &timeoutMs, sizeof(timeoutMs));
    
    HINTERNET hRequest = NULL;
    std::string result;
    
    try {
        if (method == "GET" || method == "HEAD") {
            hRequest = InternetOpenUrlA(hInternet, url.c_str(), NULL, 0,
                                       INTERNET_FLAG_RELOAD | INTERNET_FLAG_NO_CACHE_WRITE, 0);
        } else {
            // Parse URL for POST/PUT requests
            std::regex urlRegex(R"(^https?://([^/]+)(/.*)?$)");
            std::smatch matches;
            if (!std::regex_match(url, matches, urlRegex)) {
                throw std::runtime_error("Invalid URL format");
            }
            
            std::string host = matches[1].str();
            std::string path = matches[2].matched ? matches[2].str() : "/";
            bool isHttps = url.find("https://") == 0;
            
            HINTERNET hConnect = InternetConnectA(hInternet, host.c_str(), 
                                                 isHttps ? 443 : 80, NULL, NULL,
                                                 INTERNET_SERVICE_HTTP, 0, 0);
            if (!hConnect) {
                throw std::runtime_error("Failed to connect to server");
            }
            
            DWORD flags = INTERNET_FLAG_RELOAD | INTERNET_FLAG_NO_CACHE_WRITE;
            if (isHttps) flags |= INTERNET_FLAG_SECURE;
            
            hRequest = HttpOpenRequestA(hConnect, method.c_str(), path.c_str(),
                                       "HTTP/1.1", NULL, NULL, flags, 0);
            InternetCloseHandle(hConnect);
        }
        
        if (!hRequest) {
            throw std::runtime_error("Failed to create HTTP request");
        }
        
        // Send request
        std::vector<unsigned char> body;
        if (!bodyBase64.empty()) {
            body = utils::Base64::decode(bodyBase64);
        }
        
        BOOL success;
        if (body.empty()) {
            success = HttpSendRequestA(hRequest, NULL, 0, NULL, 0);
        } else {
            success = HttpSendRequestA(hRequest, "Content-Type: application/octet-stream\r\n", -1,
                                      body.data(), static_cast<DWORD>(body.size()));
        }
        
        if (!success) {
            throw std::runtime_error("Failed to send HTTP request");
        }
        
        // Read response
        char buffer[8192];
        DWORD bytesRead;
        while (InternetReadFile(hRequest, buffer, sizeof(buffer), &bytesRead) && bytesRead > 0) {
            result.append(buffer, bytesRead);
            if (result.length() > 1048576) break; // 1MB max
        }
        
    } catch (...) {
        if (hRequest) InternetCloseHandle(hRequest);
        InternetCloseHandle(hInternet);
        throw;
    }
    
    if (hRequest) InternetCloseHandle(hRequest);
    InternetCloseHandle(hInternet);
    
    auto endTime = std::chrono::steady_clock::now();
    latencyMs = std::chrono::duration_cast<std::chrono::milliseconds>(endTime - startTime).count();
    
    return result;
}

} // namespace iploop