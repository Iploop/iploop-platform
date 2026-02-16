#include "../include/iploop_sdk.h"
#include "websocket.h"
#include "tunnel.h"
#include "utils.h"
#include <thread>
#include <atomic>
#include <chrono>
#include <sstream>
#include <regex>
#include <cstring>

namespace {

// Forward declarations
void handleTextMessage(const std::string& text);
void handleBinaryMessage(const std::vector<unsigned char>& data);
void handleCooldown(const std::string& text);
void handleTunnelOpen(const std::string& text);
void handleTunnelData(const std::string& text);
void handleProxyRequest(const std::string& text);
void sendTunnelResponse(const std::string& tunnelId, bool success, const std::string& error);
void sendBinaryTunnelData(const std::string& tunnelId, const unsigned char* data, size_t len, bool eof);
void sendProxyResponse(const std::string& requestId, bool success, int statusCode,
                      const std::string& responseBody, long long latencyMs, const std::string& error);
void sendHello();
void sendKeepalive();
void fetchAndSendIPInfo();
void sendIPInfo(const std::string& ip, const std::string& ipInfoJson, long long ipFetchMs, long long infoFetchMs);
void connectionLoop();
void keepaliveLoop();

// ── SDK state ──

const std::string SDK_VERSION = "2.0";
const std::string DEFAULT_SERVER = "wss://gateway.iploop.io:9443/ws";
const int KEEPALIVE_INTERVAL_MS = 55000;
const int RECONNECT_BASE_MS = 1000;
const int RECONNECT_MAX_MS = 30000;        // 30s cap during fast phase
const int RECONNECT_FAST_ATTEMPTS = 15;    // First 15 attempts: exponential backoff
const int RECONNECT_SLOW_MS = 600000;      // After that: 10 minute intervals, never give up
const int IP_CHECK_COOLDOWN_MS = 3600000; // 1 hour

std::string g_serverUrl = DEFAULT_SERVER;
std::string g_nodeId;
std::string g_deviceModel;

std::unique_ptr<iploop::WebSocketClient> g_webSocket;
std::unique_ptr<iploop::TunnelManager> g_tunnelManager;
std::unique_ptr<iploop::ProxyHandler> g_proxyHandler;

std::atomic<bool> g_running{false};
std::atomic<bool> g_connected{false};
std::atomic<long long> g_totalConnections{0};
std::atomic<long long> g_totalDisconnections{0};

std::thread g_connectionThread;
std::thread g_keepaliveThread;

// Connection state
std::atomic<int> g_reconnectAttempt{0};
std::atomic<long long> g_connectedSince{0};
std::atomic<long long> g_cooldownUntil{0};

// IP info cache
std::string g_cachedIP;
std::string g_cachedIPInfoJson;
std::atomic<long long> g_lastIPCheckTime{0};

// ── Helper functions ──

void initializeComponents() {
    if (!g_webSocket) {
        g_webSocket = std::make_unique<iploop::WebSocketClient>();
        g_webSocket->setStateHandler([](bool connected, const std::string& reason) {
            g_connected = connected;
            if (connected) {
                g_connectedSince = iploop::utils::Timer::nowMs();
                g_totalConnections++;
                iploop::utils::Logger::info("Connected! (#" + std::to_string(g_totalConnections.load()) + ")");
            } else if (!reason.empty()) {
                g_totalDisconnections++;
                long long duration = (iploop::utils::Timer::nowMs() - g_connectedSince) / 1000;
                iploop::utils::Logger::info("Disconnected: " + reason + 
                    " (connected " + std::to_string(duration) + "s, tunnels=" + 
                    std::to_string(g_tunnelManager ? g_tunnelManager->getActiveTunnelCount() : 0) + ")");
            }
        });
        
        g_webSocket->setMessageHandler([](int opcode, const std::vector<unsigned char>& data) {
            if (opcode == 1) { // Text message
                std::string text(data.begin(), data.end());
                handleTextMessage(text);
            } else if (opcode == 2) { // Binary message
                handleBinaryMessage(data);
            }
        });
    }
    
    if (!g_tunnelManager) {
        g_tunnelManager = std::make_unique<iploop::TunnelManager>();
        g_tunnelManager->setDataHandler([](const std::string& tunnelId, const unsigned char* data, size_t len, bool isEof) {
            sendBinaryTunnelData(tunnelId, data, len, isEof);
        });
        
        g_tunnelManager->setResponseHandler([](const std::string& tunnelId, bool success, const std::string& error) {
            sendTunnelResponse(tunnelId, success, error);
        });
    }
    
    if (!g_proxyHandler) {
        g_proxyHandler = std::make_unique<iploop::ProxyHandler>();
        g_proxyHandler->setResponseHandler([](const std::string& requestId, bool success, int statusCode,
                                             const std::string& responseBody, long long latencyMs, const std::string& error) {
            sendProxyResponse(requestId, success, statusCode, responseBody, latencyMs, error);
        });
    }
}

void handleTextMessage(const std::string& text) {
    try {
        if (text.find("\"welcome\"") != std::string::npos) {
            iploop::utils::Logger::info("Welcome received");
        } else if (text.find("\"keepalive_ack\"") != std::string::npos) {
            long long uptime = (iploop::utils::Timer::nowMs() - g_connectedSince) / 1000;
            iploop::utils::Logger::debug("Keepalive ACK (uptime=" + std::to_string(uptime) + "s)");
        } else if (text.find("\"cooldown\"") != std::string::npos) {
            handleCooldown(text);
        } else if (text.find("\"tunnel_open\"") != std::string::npos) {
            handleTunnelOpen(text);
        } else if (text.find("\"tunnel_data\"") != std::string::npos) {
            handleTunnelData(text);
        } else if (text.find("\"proxy_request\"") != std::string::npos) {
            handleProxyRequest(text);
        } else {
            std::string preview = text.substr(0, std::min(static_cast<size_t>(100), text.length()));
            iploop::utils::Logger::debug("Received: " + preview);
        }
    } catch (const std::exception& e) {
        iploop::utils::Logger::error("Error handling message: " + std::string(e.what()));
    }
}

void handleBinaryMessage(const std::vector<unsigned char>& data) {
    if (data.size() < 37) return;
    
    // Binary tunnel protocol: [36 bytes tunnel_id][1 byte flags][N bytes payload]
    std::string tunnelId(reinterpret_cast<const char*>(data.data()), 36);
    // Trim whitespace from tunnel ID
    size_t end = tunnelId.find_last_not_of(" \t\0");
    if (end != std::string::npos) {
        tunnelId = tunnelId.substr(0, end + 1);
    }
    
    bool eof = data[36] == 0x01;
    
    if (eof) {
        iploop::utils::Logger::info("Tunnel " + tunnelId.substr(0, 8) + " received binary EOF from server");
        if (g_tunnelManager) {
            g_tunnelManager->closeTunnel(tunnelId);
        }
        return;
    }
    
    // Write data to tunnel
    if (data.size() > 37 && g_tunnelManager) {
        const unsigned char* payload = data.data() + 37;
        size_t payloadLen = data.size() - 37;
        g_tunnelManager->writeTunnelData(tunnelId, payload, payloadLen);
    }
}

void handleCooldown(const std::string& text) {
    int retrySec = 600; // default 10 min
    
    std::regex retryRegex(R"(retry_after_sec["\s:]*(\d+))");
    std::smatch matches;
    if (std::regex_search(text, matches, retryRegex)) {
        try {
            retrySec = std::stoi(matches[1].str());
        } catch (...) {
            // Use default
        }
    }
    
    g_cooldownUntil = iploop::utils::Timer::nowMs() + (retrySec * 1000LL);
    iploop::utils::Logger::info("Server cooldown: sleeping " + std::to_string(retrySec) + "s");
    
    if (g_webSocket) {
        g_webSocket->disconnect("server_cooldown_" + std::to_string(retrySec) + "s");
    }
}

void handleTunnelOpen(const std::string& text) {
    iploop::utils::Logger::info("tunnel_open raw: " + text.substr(0, std::min(text.length(), static_cast<size_t>(300))));
    
    std::string tunnelId = iploop::utils::Json::extractString(text, "tunnel_id");
    std::string host = iploop::utils::Json::extractString(text, "host");
    std::string portStr = iploop::utils::Json::extractString(text, "port");
    
    iploop::utils::Logger::info("tunnel_open parsed: id=" + tunnelId + " host=" + host + " port=" + portStr);
    
    if (tunnelId.empty() || host.empty() || portStr.empty()) {
        iploop::utils::Logger::error("Invalid tunnel_open: missing fields (id=" + tunnelId + " host=" + host + " port=" + portStr + ")");
        return;
    }
    
    int port;
    try {
        port = std::stoi(portStr);
    } catch (...) {
        sendTunnelResponse(tunnelId, false, "invalid port: " + portStr);
        return;
    }
    
    iploop::utils::Logger::info("Opening tunnel " + tunnelId.substr(0, 8) + " to " + host + ":" + std::to_string(port));
    
    if (g_tunnelManager) {
        g_tunnelManager->openTunnel(tunnelId, host, port, 10000);
    }
}

void handleTunnelData(const std::string& text) {
    std::string tunnelId = iploop::utils::Json::extractString(text, "tunnel_id");
    if (tunnelId.empty()) return;
    
    // Check EOF
    if (text.find("\"eof\":true") != std::string::npos || 
        text.find("\"eof\": true") != std::string::npos) {
        iploop::utils::Logger::info("Tunnel " + tunnelId.substr(0, 8) + " received EOF from server");
        if (g_tunnelManager) {
            g_tunnelManager->closeTunnel(tunnelId);
        }
        return;
    }
    
    // Extract base64 data
    std::string b64Data = iploop::utils::Json::extractString(text, "data");
    if (!b64Data.empty() && g_tunnelManager) {
        auto decoded = iploop::utils::Base64::decode(b64Data);
        if (!decoded.empty()) {
            g_tunnelManager->writeTunnelData(tunnelId, decoded.data(), decoded.size());
        }
    }
}

void handleProxyRequest(const std::string& text) {
    std::string requestId = iploop::utils::Json::extractString(text, "request_id");
    std::string url = iploop::utils::Json::extractString(text, "url");
    std::string method = iploop::utils::Json::extractString(text, "method");
    std::string headers = iploop::utils::Json::extractString(text, "headers");
    std::string bodyBase64 = iploop::utils::Json::extractString(text, "body");
    int timeoutMs = iploop::utils::Json::extractInt(text, "timeout_ms");
    
    if (requestId.empty()) return;
    if (method.empty()) method = "GET";
    if (timeoutMs <= 0) timeoutMs = 30000;
    
    if (g_proxyHandler) {
        g_proxyHandler->handleProxyRequest(requestId, method, url, headers, bodyBase64, timeoutMs);
    }
}

void sendTunnelResponse(const std::string& tunnelId, bool success, const std::string& error) {
    if (!g_webSocket || !g_connected) return;
    
    std::ostringstream oss;
    oss << "{\"type\":\"tunnel_response\",\"data\":{\"tunnel_id\":\"" << tunnelId << "\"";
    oss << ",\"success\":" << (success ? "true" : "false");
    if (!success && !error.empty()) {
        oss << ",\"error\":\"" << iploop::utils::Json::escape(error) << "\"";
    }
    oss << "}}";
    
    g_webSocket->sendText(oss.str());
}

void sendBinaryTunnelData(const std::string& tunnelId, const unsigned char* data, size_t len, bool eof) {
    if (!g_webSocket || !g_connected) {
        iploop::utils::Logger::info("Tunnel " + tunnelId.substr(0, 8) + 
            " relay DROPPED (disconnected) " + std::to_string(len) + "B eof=" + (eof ? "true" : "false"));
        return;
    }
    
    // Binary protocol: [36 bytes tunnel_id][1 byte flags][N bytes data]
    std::vector<unsigned char> frame(37 + (data ? len : 0));
    
    // Pad tunnel ID to 36 bytes
    std::memset(frame.data(), 0, 36);
    size_t copyLen = std::min(tunnelId.length(), static_cast<size_t>(36));
    std::memcpy(frame.data(), tunnelId.c_str(), copyLen);
    
    // Flags: 0x00 = data, 0x01 = EOF
    frame[36] = eof ? 0x01 : 0x00;
    
    // Data payload
    if (data && len > 0) {
        std::memcpy(frame.data() + 37, data, len);
    }
    
    g_webSocket->sendBinary(frame);
}

void sendProxyResponse(const std::string& requestId, bool success, int statusCode,
                      const std::string& responseBody, long long latencyMs, const std::string& error) {
    if (!g_webSocket || !g_connected) return;
    
    std::ostringstream oss;
    oss << "{\"type\":\"proxy_response\",\"data\":{\"request_id\":\"" << requestId << "\"";
    oss << ",\"success\":" << (success ? "true" : "false");
    oss << ",\"latency_ms\":" << latencyMs;
    
    if (success) {
        oss << ",\"status_code\":" << statusCode;
        oss << ",\"body\":\"" << responseBody << "\"";
        oss << ",\"bytes_read\":" << (iploop::utils::Base64::decode(responseBody).size());
    } else {
        oss << ",\"error\":\"" << iploop::utils::Json::escape(error) << "\"";
    }
    
    oss << "}}";
    g_webSocket->sendText(oss.str());
}

void sendHello() {
    if (!g_webSocket || !g_connected) return;
    
    std::ostringstream oss;
    oss << "{\"type\":\"hello\",\"node_id\":\"" << g_nodeId << "\"";
    oss << ",\"device_model\":\"" << iploop::utils::Json::escape(g_deviceModel) << "\"";
    oss << ",\"sdk_version\":\"" << SDK_VERSION << "\"}";
    
    g_webSocket->sendText(oss.str());
}

void sendKeepalive() {
    if (!g_webSocket || !g_connected) return;
    
    long long uptime = (iploop::utils::Timer::nowMs() - g_connectedSince) / 1000;
    int tunnels = g_tunnelManager ? g_tunnelManager->getActiveTunnelCount() : 0;
    
    std::ostringstream oss;
    oss << "{\"type\":\"keepalive\",\"uptime_sec\":" << uptime;
    oss << ",\"active_tunnels\":" << tunnels << "}";
    
    g_webSocket->sendText(oss.str());
}

void fetchAndSendIPInfo() {
    if (!g_running || !g_connected) return;
    
    long long now = iploop::utils::Timer::nowMs();
    if (now - g_lastIPCheckTime < IP_CHECK_COOLDOWN_MS && !g_cachedIPInfoJson.empty()) {
        iploop::utils::Logger::info("IP check cooldown active, sending cached info");
        sendIPInfo(g_cachedIP, g_cachedIPInfoJson, 0, 0);
        return;
    }
    
    long long ipStart = iploop::utils::Timer::nowMs();
    std::string ip = iploop::utils::HttpClient::get("https://ip2location.io/ip", 10000);
    long long ipFetchMs = iploop::utils::Timer::nowMs() - ipStart;
    
    // Trim whitespace
    ip.erase(0, ip.find_first_not_of(" \t\n\r"));
    ip.erase(ip.find_last_not_of(" \t\n\r") + 1);
    
    if (ip.empty() || ip.length() > 45) {
        iploop::utils::Logger::error("Failed to get IP");
        return;
    }
    
    iploop::utils::Logger::info("Got IP: " + ip + " (" + std::to_string(ipFetchMs) + "ms)");
    g_lastIPCheckTime = now;
    
    if (ip == g_cachedIP && !g_cachedIPInfoJson.empty()) {
        iploop::utils::Logger::info("IP unchanged (" + ip + "), using cached info");
        sendIPInfo(ip, g_cachedIPInfoJson, ipFetchMs, 0);
        return;
    }
    
    iploop::utils::Logger::info("IP changed or first fetch, querying ip2location...");
    long long infoStart = iploop::utils::Timer::nowMs();
    std::string page = iploop::utils::HttpClient::get("https://www.ip2location.com/" + ip, 15000);
    long long infoFetchMs = iploop::utils::Timer::nowMs() - infoStart;
    
    std::string marker = "language-json\">";
    size_t start = page.find(marker);
    if (start == std::string::npos) {
        iploop::utils::Logger::error("Could not find language-json in page");
        return;
    }
    start += marker.length();
    
    size_t end = page.find("</code>", start);
    if (end == std::string::npos) {
        iploop::utils::Logger::error("Could not find closing </code>");
        return;
    }
    
    std::string ipInfoJson = page.substr(start, end - start);
    
    // HTML decode
    std::regex htmlRegex("&quot;");
    ipInfoJson = std::regex_replace(ipInfoJson, htmlRegex, "\"");
    htmlRegex = std::regex("&amp;");
    ipInfoJson = std::regex_replace(ipInfoJson, htmlRegex, "&");
    htmlRegex = std::regex("&lt;");
    ipInfoJson = std::regex_replace(ipInfoJson, htmlRegex, "<");
    htmlRegex = std::regex("&gt;");
    ipInfoJson = std::regex_replace(ipInfoJson, htmlRegex, ">");
    htmlRegex = std::regex("&#39;");
    ipInfoJson = std::regex_replace(ipInfoJson, htmlRegex, "'");
    
    // Trim
    ipInfoJson.erase(0, ipInfoJson.find_first_not_of(" \t\n\r"));
    ipInfoJson.erase(ipInfoJson.find_last_not_of(" \t\n\r") + 1);
    
    iploop::utils::Logger::info("Got IP info (" + std::to_string(infoFetchMs) + "ms)");
    
    // Cache the info
    g_cachedIP = ip;
    g_cachedIPInfoJson = ipInfoJson;
    iploop::utils::Windows::saveToRegistry("cached_ip", g_cachedIP);
    iploop::utils::Windows::saveToRegistry("cached_ip_info", g_cachedIPInfoJson);
    iploop::utils::Windows::saveToRegistry("last_ip_check", g_lastIPCheckTime.load());
    
    sendIPInfo(ip, ipInfoJson, ipFetchMs, infoFetchMs);
}

void sendIPInfo(const std::string& ip, const std::string& ipInfoJson, long long ipFetchMs, long long infoFetchMs) {
    if (!g_webSocket || !g_connected) return;
    
    std::ostringstream oss;
    oss << "{\"type\":\"ip_info\",\"node_id\":\"" << g_nodeId << "\"";
    oss << ",\"device_id\":\"" << iploop::utils::Json::escape(g_nodeId) << "\"";
    oss << ",\"device_model\":\"" << iploop::utils::Json::escape(g_deviceModel) << "\"";
    oss << ",\"ip\":\"" << iploop::utils::Json::escape(ip) << "\"";
    oss << ",\"ip_fetch_ms\":" << ipFetchMs;
    oss << ",\"info_fetch_ms\":" << infoFetchMs;
    oss << ",\"ip_info\":" << ipInfoJson << "}";
    
    g_webSocket->sendText(oss.str());
    iploop::utils::Logger::info("Sent IP info to server");
}

void loadIPCache() {
    g_cachedIP = iploop::utils::Windows::loadFromRegistry("cached_ip");
    g_cachedIPInfoJson = iploop::utils::Windows::loadFromRegistry("cached_ip_info");
    g_lastIPCheckTime = iploop::utils::Windows::loadFromRegistryInt("last_ip_check");
    
    if (!g_cachedIP.empty()) {
        long long age = (iploop::utils::Timer::nowMs() - g_lastIPCheckTime) / 1000;
        iploop::utils::Logger::info("Loaded IP cache: " + g_cachedIP + " (age=" + std::to_string(age) + "s)");
    }
}

void connectionLoop() {
    while (g_running) {
        try {
            initializeComponents();
            
            if (g_webSocket->connect(g_serverUrl, 15000)) {
                g_reconnectAttempt = 0;
                g_webSocket->startReading();
                
                // Send hello and start IP info fetch
                sendHello();
                
                std::thread ipThread([]() {
                    fetchAndSendIPInfo();
                });
                ipThread.detach();
                
                // Wait for disconnection
                while (g_running && g_connected) {
                    std::this_thread::sleep_for(std::chrono::milliseconds(1000));
                }
                
                g_webSocket->stopReading();
            }
        } catch (const std::exception& e) {
            iploop::utils::Logger::error("Connection error: " + std::string(e.what()));
        }
        
        if (!g_running) break;
        
        g_reconnectAttempt++;
        if (g_tunnelManager) {
            g_tunnelManager->closeAllTunnels();
        }
        
        // Check cooldown
        long long cooldownRemaining = g_cooldownUntil - iploop::utils::Timer::nowMs();
        if (cooldownRemaining > 0) {
            iploop::utils::Logger::info("On cooldown, sleeping " + std::to_string(cooldownRemaining / 1000) + "s");
            std::this_thread::sleep_for(std::chrono::milliseconds(cooldownRemaining));
            g_cooldownUntil = 0;
        } else {
            int attempt = g_reconnectAttempt.load();
            int delay;
            if (attempt <= RECONNECT_FAST_ATTEMPTS) {
                delay = std::min(RECONNECT_BASE_MS * (1 << std::min(attempt, 10)), RECONNECT_MAX_MS);
                iploop::utils::Logger::info("Reconnecting in " + std::to_string(delay) + "ms (attempt #" + std::to_string(attempt) + ")");
            } else {
                delay = RECONNECT_SLOW_MS;
                iploop::utils::Logger::info("Reconnecting in 10 minutes (slow mode, attempt #" + std::to_string(attempt) + ")");
            }
            std::this_thread::sleep_for(std::chrono::milliseconds(delay));
        }
    }
}

void keepaliveLoop() {
    while (g_running) {
        std::this_thread::sleep_for(std::chrono::milliseconds(KEEPALIVE_INTERVAL_MS));
        if (g_running && g_connected) {
            sendKeepalive();
        }
    }
}

} // anonymous namespace

// ── Public API implementation ──

void IPLoopSDK::init() {
    init(DEFAULT_SERVER);
}

void IPLoopSDK::init(const std::string& serverUrl) {
    g_serverUrl = serverUrl;
    g_nodeId = iploop::utils::Windows::getMachineGuid();
    g_deviceModel = iploop::utils::Windows::getDeviceModel();
    
    loadIPCache();
    
    iploop::utils::Logger::info("Initialized. nodeId=" + g_nodeId + " model=" + g_deviceModel + " version=" + SDK_VERSION);
}

void IPLoopSDK::start() {
    if (g_nodeId.empty()) {
        iploop::utils::Logger::error("Not initialized. Call init() first.");
        return;
    }
    
    if (g_running.exchange(true)) {
        iploop::utils::Logger::info("Already running.");
        return;
    }
    
    g_connectionThread = std::thread(connectionLoop);
    g_keepaliveThread = std::thread(keepaliveLoop);
    
    iploop::utils::Logger::info("Started. server=" + g_serverUrl);
}

void IPLoopSDK::stop() {
    g_running = false;
    
    if (g_tunnelManager) {
        g_tunnelManager->closeAllTunnels();
    }
    
    if (g_webSocket) {
        g_webSocket->disconnect("stop_called");
    }
    
    if (g_connectionThread.joinable()) {
        g_connectionThread.join();
    }
    
    if (g_keepaliveThread.joinable()) {
        g_keepaliveThread.join();
    }
    
    // Reset components
    g_webSocket.reset();
    g_tunnelManager.reset();
    g_proxyHandler.reset();
    
    iploop::utils::Logger::info("Stopped. conns=" + std::to_string(g_totalConnections.load()) + 
                               " disconns=" + std::to_string(g_totalDisconnections.load()));
}

bool IPLoopSDK::isConnected() {
    return g_connected;
}

bool IPLoopSDK::isRunning() {
    return g_running;
}

std::string IPLoopSDK::getNodeId() {
    return g_nodeId;
}

int IPLoopSDK::getActiveTunnelCount() {
    return g_tunnelManager ? g_tunnelManager->getActiveTunnelCount() : 0;
}

std::string IPLoopSDK::getVersion() {
    return SDK_VERSION;
}

std::pair<long long, long long> IPLoopSDK::getConnectionStats() {
    return {g_totalConnections.load(), g_totalDisconnections.load()};
}

void IPLoopSDK::setLogLevel(int level) {
    iploop::utils::Logger::setLevel(static_cast<iploop::utils::Logger::Level>(level));
}