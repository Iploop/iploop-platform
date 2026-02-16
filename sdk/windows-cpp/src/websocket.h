#pragma once

#include <string>
#include <vector>
#include <functional>
#include <atomic>
#include <memory>

#include <windows.h>
#include <winsock2.h>
#include <ws2tcpip.h>
#include <schannel.h>
#ifndef SECURITY_WIN32
#define SECURITY_WIN32
#endif
#include <security.h>
#include <sspi.h>

namespace iploop {

/**
 * WebSocket client with SSL support using WinSock2 + SChannel
 * No external dependencies (OpenSSL, etc.)
 */
class WebSocketClient {
public:
    /**
     * Message handler callback
     * @param opcode WebSocket opcode (1=text, 2=binary, 8=close, 9=ping, 10=pong)
     * @param data message payload
     */
    using MessageHandler = std::function<void(int opcode, const std::vector<unsigned char>& data)>;

    /**
     * Connection state change callback
     * @param connected true if connected, false if disconnected
     * @param reason disconnection reason (empty if connected)
     */
    using StateHandler = std::function<void(bool connected, const std::string& reason)>;

    WebSocketClient();
    ~WebSocketClient();

    /**
     * Set message handler
     * @param handler callback function
     */
    void setMessageHandler(const MessageHandler& handler);

    /**
     * Set state change handler
     * @param handler callback function
     */
    void setStateHandler(const StateHandler& handler);

    /**
     * Connect to WebSocket server
     * @param url WebSocket URL (ws:// or wss://)
     * @param timeoutMs connection timeout in milliseconds
     * @return true if connection successful
     */
    bool connect(const std::string& url, int timeoutMs = 15000);

    /**
     * Disconnect from server
     * @param reason optional reason string
     */
    void disconnect(const std::string& reason = "client_disconnect");

    /**
     * Check if connected
     * @return true if WebSocket connection is active
     */
    bool isConnected() const;

    /**
     * Send text message
     * @param text message text
     * @return true if sent successfully
     */
    bool sendText(const std::string& text);

    /**
     * Send binary message
     * @param data binary data
     * @return true if sent successfully
     */
    bool sendBinary(const std::vector<unsigned char>& data);

    /**
     * Send binary data from buffer
     * @param data data buffer
     * @param len data length
     * @return true if sent successfully
     */
    bool sendBinary(const unsigned char* data, size_t len);

    /**
     * Send ping frame
     * @param data optional ping data
     * @return true if sent successfully
     */
    bool sendPing(const std::vector<unsigned char>& data = {});

    /**
     * Start reading loop in background thread
     * Must call this after connect() to receive messages
     */
    void startReading();

    /**
     * Stop reading loop
     */
    void stopReading();

private:
    /**
     * Send WebSocket frame
     * @param opcode frame opcode
     * @param payload frame payload
     * @return true if sent successfully
     */
    bool sendFrame(int opcode, const std::vector<unsigned char>& payload);
    bool readExact(unsigned char* buf, size_t needed);
    void readLoop();
    struct Impl;
    std::unique_ptr<Impl> pImpl;
    
    // Non-copyable
    WebSocketClient(const WebSocketClient&) = delete;
    WebSocketClient& operator=(const WebSocketClient&) = delete;
};

/**
 * SSL/TLS context using SChannel
 */
class SChannelContext {
public:
    SChannelContext();
    ~SChannelContext();

    /**
     * Initialize SSL context for client connection
     * @param hostname server hostname for SNI
     * @return true if initialization successful
     */
    bool initializeClient(const std::string& hostname);

    /**
     * Perform SSL handshake
     * @param socket connected TCP socket
     * @return true if handshake successful
     */
    bool performHandshake(SOCKET socket);

    /**
     * Send encrypted data
     * @param socket TCP socket
     * @param data data to send
     * @param len data length
     * @return bytes sent or -1 on error
     */
    int send(SOCKET socket, const void* data, int len);

    /**
     * Receive encrypted data
     * @param socket TCP socket
     * @param buffer receive buffer
     * @param bufLen buffer length
     * @return bytes received or -1 on error
     */
    int recv(SOCKET socket, void* buffer, int bufLen);

    /**
     * Shutdown SSL connection
     */
    void shutdown();

private:
    struct SChannelImpl;
    std::unique_ptr<SChannelImpl> pImpl;
    
    // Non-copyable
    SChannelContext(const SChannelContext&) = delete;
    SChannelContext& operator=(const SChannelContext&) = delete;
};

} // namespace iploop