#include "websocket.h"
#include "utils.h"
#include <thread>
#include <random>
#include <regex>
#include <sstream>
#include <cstring>
#include <algorithm>

#pragma comment(lib, "ws2_32.lib")
#pragma comment(lib, "secur32.lib")

namespace iploop {

// ── SChannel implementation ──

struct SChannelContext::SChannelImpl {
    CredHandle hClientCreds = {0};
    CtxtHandle hContext = {0};
    SecPkgContext_StreamSizes streamSizes = {0};
    bool initialized = false;
    bool handshakeComplete = false;
    std::string hostname;
    
    // Encryption buffers
    std::vector<unsigned char> readBuffer;       // Raw encrypted data from socket
    std::vector<unsigned char> decryptBuffer;
    size_t readBufferPos = 0;                    // Bytes of encrypted data in readBuffer
    
    // Decrypted plaintext buffer (leftover from previous decrypt)
    std::vector<unsigned char> plaintextBuffer;
    size_t plaintextPos = 0;                     // Read position in plaintextBuffer
    size_t plaintextLen = 0;                     // Valid bytes in plaintextBuffer
    
    ~SChannelImpl() {
        if (handshakeComplete) {
            DeleteSecurityContext(&hContext);
        }
        if (initialized) {
            FreeCredentialsHandle(&hClientCreds);
        }
    }
};

SChannelContext::SChannelContext() : pImpl(std::make_unique<SChannelImpl>()) {}
SChannelContext::~SChannelContext() = default;

bool SChannelContext::initializeClient(const std::string& hostname) {
    pImpl->hostname = hostname;
    
    SCHANNEL_CRED schannelCred = {0};
    schannelCred.dwVersion = SCHANNEL_CRED_VERSION;
    schannelCred.dwFlags = SCH_CRED_NO_DEFAULT_CREDS | SCH_CRED_AUTO_CRED_VALIDATION;
    
    SECURITY_STATUS status = AcquireCredentialsHandleA(
        NULL, const_cast<char*>(UNISP_NAME_A), SECPKG_CRED_OUTBOUND,
        NULL, &schannelCred, NULL, NULL, &pImpl->hClientCreds, NULL);
    
    if (status != SEC_E_OK) {
        utils::Logger::error("Failed to acquire credentials handle: " + std::to_string(status));
        return false;
    }
    
    pImpl->initialized = true;
    return true;
}

bool SChannelContext::performHandshake(SOCKET socket) {
    SecBufferDesc outBufferDesc = {0};
    SecBuffer outBuffer = {0};
    SecBufferDesc inBufferDesc = {0};
    SecBuffer inBuffers[2] = {0};
    
    DWORD contextAttributes;
    TimeStamp expiry;
    SECURITY_STATUS status;
    
    // Initial handshake
    outBufferDesc.ulVersion = SECBUFFER_VERSION;
    outBufferDesc.cBuffers = 1;
    outBufferDesc.pBuffers = &outBuffer;
    outBuffer.BufferType = SECBUFFER_TOKEN;
    outBuffer.cbBuffer = 0;
    outBuffer.pvBuffer = NULL;
    
    status = InitializeSecurityContextA(
        &pImpl->hClientCreds, NULL, const_cast<char*>(pImpl->hostname.c_str()),
        ISC_REQ_SEQUENCE_DETECT | ISC_REQ_REPLAY_DETECT | ISC_REQ_CONFIDENTIALITY |
        ISC_REQ_EXTENDED_ERROR | ISC_REQ_ALLOCATE_MEMORY | ISC_REQ_STREAM,
        0, SECURITY_NATIVE_DREP, NULL, 0, &pImpl->hContext,
        &outBufferDesc, &contextAttributes, &expiry);
    
    if (status != SEC_I_CONTINUE_NEEDED) {
        utils::Logger::error("InitializeSecurityContext failed: " + std::to_string(status));
        return false;
    }
    
    // Send initial token
    if (outBuffer.cbBuffer > 0) {
        int sent = ::send(socket, reinterpret_cast<char*>(outBuffer.pvBuffer), outBuffer.cbBuffer, 0);
        FreeContextBuffer(outBuffer.pvBuffer);
        if (sent <= 0) return false;
    }
    
    std::vector<unsigned char> handshakeBuffer;
    handshakeBuffer.reserve(16384);
    
    while (status == SEC_I_CONTINUE_NEEDED || status == SEC_E_INCOMPLETE_MESSAGE) {
        // Receive data
        char tempBuffer[4096];
        int received = ::recv(socket, tempBuffer, sizeof(tempBuffer), 0);
        if (received <= 0) return false;
        
        handshakeBuffer.insert(handshakeBuffer.end(), tempBuffer, tempBuffer + received);
        
        // Setup input buffers
        inBuffers[0].BufferType = SECBUFFER_TOKEN;
        inBuffers[0].cbBuffer = static_cast<DWORD>(handshakeBuffer.size());
        inBuffers[0].pvBuffer = handshakeBuffer.data();
        inBuffers[1].BufferType = SECBUFFER_EMPTY;
        inBuffers[1].cbBuffer = 0;
        inBuffers[1].pvBuffer = NULL;
        
        inBufferDesc.ulVersion = SECBUFFER_VERSION;
        inBufferDesc.cBuffers = 2;
        inBufferDesc.pBuffers = inBuffers;
        
        // Setup output buffer
        outBuffer.BufferType = SECBUFFER_TOKEN;
        outBuffer.cbBuffer = 0;
        outBuffer.pvBuffer = NULL;
        
        status = InitializeSecurityContextA(
            &pImpl->hClientCreds, &pImpl->hContext, NULL,
            ISC_REQ_SEQUENCE_DETECT | ISC_REQ_REPLAY_DETECT | ISC_REQ_CONFIDENTIALITY |
            ISC_REQ_EXTENDED_ERROR | ISC_REQ_ALLOCATE_MEMORY | ISC_REQ_STREAM,
            0, SECURITY_NATIVE_DREP, &inBufferDesc, 0, NULL,
            &outBufferDesc, &contextAttributes, &expiry);
        
        // Send response if needed
        if (outBuffer.cbBuffer > 0) {
            int sent = ::send(socket, reinterpret_cast<char*>(outBuffer.pvBuffer), outBuffer.cbBuffer, 0);
            FreeContextBuffer(outBuffer.pvBuffer);
            if (sent <= 0) return false;
        }
        
        // Handle extra data
        if (inBuffers[1].BufferType == SECBUFFER_EXTRA && inBuffers[1].cbBuffer > 0) {
            size_t extraBytes = inBuffers[1].cbBuffer;
            std::memmove(handshakeBuffer.data(),
                        handshakeBuffer.data() + handshakeBuffer.size() - extraBytes,
                        extraBytes);
            handshakeBuffer.resize(extraBytes);
        } else if (status != SEC_E_INCOMPLETE_MESSAGE) {
            handshakeBuffer.clear();
        }
    }
    
    if (status != SEC_E_OK) {
        utils::Logger::error("SSL handshake failed: " + std::to_string(status));
        return false;
    }
    
    // Get stream sizes
    status = QueryContextAttributes(&pImpl->hContext, SECPKG_ATTR_STREAM_SIZES, &pImpl->streamSizes);
    if (status != SEC_E_OK) {
        utils::Logger::error("Failed to get stream sizes: " + std::to_string(status));
        return false;
    }
    
    pImpl->handshakeComplete = true;
    pImpl->readBuffer.resize(pImpl->streamSizes.cbMaximumMessage + pImpl->streamSizes.cbHeader + pImpl->streamSizes.cbTrailer);
    pImpl->decryptBuffer.resize(pImpl->streamSizes.cbMaximumMessage);
    pImpl->plaintextBuffer.resize(pImpl->streamSizes.cbMaximumMessage);
    pImpl->plaintextPos = 0;
    pImpl->plaintextLen = 0;
    
    return true;
}

int SChannelContext::send(SOCKET socket, const void* data, int len) {
    if (!pImpl->handshakeComplete || len <= 0) return -1;
    
    int totalSent = 0;
    const char* dataPtr = reinterpret_cast<const char*>(data);
    
    while (totalSent < len) {
        int chunkSize = std::min(len - totalSent, static_cast<int>(pImpl->streamSizes.cbMaximumMessage));
        
        std::vector<unsigned char> encryptBuffer(pImpl->streamSizes.cbHeader + chunkSize + pImpl->streamSizes.cbTrailer);
        
        SecBuffer buffers[4];
        buffers[0].BufferType = SECBUFFER_STREAM_HEADER;
        buffers[0].cbBuffer = pImpl->streamSizes.cbHeader;
        buffers[0].pvBuffer = encryptBuffer.data();
        
        buffers[1].BufferType = SECBUFFER_DATA;
        buffers[1].cbBuffer = chunkSize;
        buffers[1].pvBuffer = encryptBuffer.data() + pImpl->streamSizes.cbHeader;
        std::memcpy(buffers[1].pvBuffer, dataPtr + totalSent, chunkSize);
        
        buffers[2].BufferType = SECBUFFER_STREAM_TRAILER;
        buffers[2].cbBuffer = pImpl->streamSizes.cbTrailer;
        buffers[2].pvBuffer = encryptBuffer.data() + pImpl->streamSizes.cbHeader + chunkSize;
        
        buffers[3].BufferType = SECBUFFER_EMPTY;
        buffers[3].cbBuffer = 0;
        buffers[3].pvBuffer = NULL;
        
        SecBufferDesc bufferDesc;
        bufferDesc.ulVersion = SECBUFFER_VERSION;
        bufferDesc.cBuffers = 4;
        bufferDesc.pBuffers = buffers;
        
        SECURITY_STATUS status = EncryptMessage(&pImpl->hContext, 0, &bufferDesc, 0);
        if (status != SEC_E_OK) return -1;
        
        int encryptedSize = buffers[0].cbBuffer + buffers[1].cbBuffer + buffers[2].cbBuffer;
        int sent = ::send(socket, reinterpret_cast<char*>(encryptBuffer.data()), encryptedSize, 0);
        if (sent <= 0) return -1;
        
        totalSent += chunkSize;
    }
    
    return totalSent;
}

int SChannelContext::recv(SOCKET socket, void* buffer, int bufLen) {
    if (!pImpl->handshakeComplete || bufLen <= 0) return -1;
    
    // 1. Return from plaintext buffer first (leftover from previous decrypt)
    if (pImpl->plaintextLen > pImpl->plaintextPos) {
        size_t available = pImpl->plaintextLen - pImpl->plaintextPos;
        int copySize = std::min(bufLen, static_cast<int>(available));
        std::memcpy(buffer, pImpl->plaintextBuffer.data() + pImpl->plaintextPos, copySize);
        pImpl->plaintextPos += copySize;
        if (pImpl->plaintextPos >= pImpl->plaintextLen) {
            pImpl->plaintextPos = 0;
            pImpl->plaintextLen = 0;
        }
        return copySize;
    }
    
    // 2. Decrypt loop
    while (true) {
        if (pImpl->readBufferPos > 0) {
            SecBuffer buffers[4];
            buffers[0].BufferType = SECBUFFER_DATA;
            buffers[0].cbBuffer = static_cast<DWORD>(pImpl->readBufferPos);
            buffers[0].pvBuffer = pImpl->readBuffer.data();
            buffers[1].BufferType = SECBUFFER_EMPTY;
            buffers[1].cbBuffer = 0;
            buffers[1].pvBuffer = NULL;
            buffers[2].BufferType = SECBUFFER_EMPTY;
            buffers[2].cbBuffer = 0;
            buffers[2].pvBuffer = NULL;
            buffers[3].BufferType = SECBUFFER_EMPTY;
            buffers[3].cbBuffer = 0;
            buffers[3].pvBuffer = NULL;
            
            SecBufferDesc bufferDesc;
            bufferDesc.ulVersion = SECBUFFER_VERSION;
            bufferDesc.cBuffers = 4;
            bufferDesc.pBuffers = buffers;
            
            SECURITY_STATUS status = DecryptMessage(&pImpl->hContext, &bufferDesc, 0, NULL);
            
            if (status == SEC_E_OK) {
                for (int i = 0; i < 4; i++) {
                    if (buffers[i].BufferType == SECBUFFER_DATA && buffers[i].cbBuffer > 0) {
                        int decryptedSize = static_cast<int>(buffers[i].cbBuffer);
                        int copySize = std::min(bufLen, decryptedSize);
                        std::memcpy(buffer, buffers[i].pvBuffer, copySize);
                        
                        // Store leftover decrypted data in plaintext buffer
                        if (copySize < decryptedSize) {
                            int leftover = decryptedSize - copySize;
                            std::memcpy(pImpl->plaintextBuffer.data(),
                                       reinterpret_cast<unsigned char*>(buffers[i].pvBuffer) + copySize,
                                       leftover);
                            pImpl->plaintextPos = 0;
                            pImpl->plaintextLen = leftover;
                        }
                        
                        // Handle extra encrypted data (next TLS record)
                        pImpl->readBufferPos = 0;
                        for (int j = 0; j < 4; j++) {
                            if (buffers[j].BufferType == SECBUFFER_EXTRA && buffers[j].cbBuffer > 0) {
                                std::memmove(pImpl->readBuffer.data(),
                                           reinterpret_cast<unsigned char*>(buffers[j].pvBuffer),
                                           buffers[j].cbBuffer);
                                pImpl->readBufferPos = buffers[j].cbBuffer;
                                break;
                            }
                        }
                        
                        return copySize;
                    }
                }
            } else if (status == SEC_E_INCOMPLETE_MESSAGE) {
                // Need more data from socket
            } else {
                return -1;
            }
        }
        
        // Receive more encrypted data from socket
        int received = ::recv(socket,
                             reinterpret_cast<char*>(pImpl->readBuffer.data() + pImpl->readBufferPos),
                             static_cast<int>(pImpl->readBuffer.size() - pImpl->readBufferPos), 0);
        if (received <= 0) return received;
        
        pImpl->readBufferPos += received;
    }
}

void SChannelContext::shutdown() {
    if (pImpl->handshakeComplete) {
        DeleteSecurityContext(&pImpl->hContext);
        pImpl->handshakeComplete = false;
    }
}

// ── WebSocket implementation ──

struct WebSocketClient::Impl {
    SOCKET socket = INVALID_SOCKET;
    std::unique_ptr<SChannelContext> sslContext;
    bool isSSL = false;
    std::atomic<bool> connected{false};
    std::atomic<bool> reading{false};
    std::thread readThread;
    std::mutex sendMutex;
    
    MessageHandler messageHandler;
    StateHandler stateHandler;
    
    ~Impl() {
        disconnect();
    }
    
    void disconnect() {
        if (socket != INVALID_SOCKET) {
            connected = false;
            reading = false;
            if (readThread.joinable()) {
                closesocket(socket);
                // Avoid self-join when called from the read thread (e.g. on server disconnect)
                if (readThread.get_id() == std::this_thread::get_id()) {
                    readThread.detach();
                } else {
                    readThread.join();
                }
            }
            if (sslContext) {
                sslContext->shutdown();
                sslContext.reset();
            }
            socket = INVALID_SOCKET;
        }
    }
    
    int socketSend(const void* data, int len) {
        if (isSSL && sslContext) {
            return sslContext->send(socket, data, len);
        } else {
            return ::send(socket, reinterpret_cast<const char*>(data), len, 0);
        }
    }
    
    int socketRecv(void* buffer, int len) {
        if (isSSL && sslContext) {
            return sslContext->recv(socket, buffer, len);
        } else {
            return ::recv(socket, reinterpret_cast<char*>(buffer), len, 0);
        }
    }
};

WebSocketClient::WebSocketClient() : pImpl(std::make_unique<Impl>()) {
    // Initialize Winsock
    WSADATA wsaData;
    WSAStartup(MAKEWORD(2, 2), &wsaData);
}

WebSocketClient::~WebSocketClient() {
    pImpl->disconnect();
    WSACleanup();
}

void WebSocketClient::setMessageHandler(const MessageHandler& handler) {
    pImpl->messageHandler = handler;
}

void WebSocketClient::setStateHandler(const StateHandler& handler) {
    pImpl->stateHandler = handler;
}

bool WebSocketClient::connect(const std::string& url, int timeoutMs) {
    std::regex urlRegex(R"(^(wss?)://([^:/]+)(?::(\d+))?(/.*)?$)");
    std::smatch matches;
    
    if (!std::regex_match(url, matches, urlRegex)) {
        utils::Logger::error("Invalid WebSocket URL: " + url);
        return false;
    }
    
    std::string scheme = matches[1].str();
    std::string host = matches[2].str();
    int port = matches[3].matched ? std::stoi(matches[3].str()) : (scheme == "wss" ? 443 : 80);
    std::string path = matches[4].matched ? matches[4].str() : "/";
    
    pImpl->isSSL = (scheme == "wss");
    
    // Create socket
    pImpl->socket = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
    if (pImpl->socket == INVALID_SOCKET) {
        utils::Logger::error("Failed to create socket");
        return false;
    }
    
    // Disable Nagle for low-latency relay
    int flag = 1;
    setsockopt(pImpl->socket, IPPROTO_TCP, TCP_NODELAY, reinterpret_cast<char*>(&flag), sizeof(flag));
    
    // Bigger socket buffers (64KB)
    int bufSize = 65536;
    setsockopt(pImpl->socket, SOL_SOCKET, SO_RCVBUF, reinterpret_cast<char*>(&bufSize), sizeof(bufSize));
    setsockopt(pImpl->socket, SOL_SOCKET, SO_SNDBUF, reinterpret_cast<char*>(&bufSize), sizeof(bufSize));
    
    // Set timeout
    setsockopt(pImpl->socket, SOL_SOCKET, SO_RCVTIMEO, reinterpret_cast<char*>(&timeoutMs), sizeof(timeoutMs));
    setsockopt(pImpl->socket, SOL_SOCKET, SO_SNDTIMEO, reinterpret_cast<char*>(&timeoutMs), sizeof(timeoutMs));
    
    // Resolve hostname
    struct addrinfo hints = {0}, *result = nullptr;
    hints.ai_family = AF_INET;
    hints.ai_socktype = SOCK_STREAM;
    
    if (getaddrinfo(host.c_str(), std::to_string(port).c_str(), &hints, &result) != 0) {
        utils::Logger::error("Failed to resolve hostname: " + host);
        closesocket(pImpl->socket);
        pImpl->socket = INVALID_SOCKET;
        return false;
    }
    
    // Connect
    bool connectSuccess = false;
    for (struct addrinfo* addr = result; addr != nullptr; addr = addr->ai_next) {
        if (::connect(pImpl->socket, addr->ai_addr, static_cast<int>(addr->ai_addrlen)) == 0) {
            connectSuccess = true;
            break;
        }
    }
    freeaddrinfo(result);
    
    if (!connectSuccess) {
        utils::Logger::error("Failed to connect to " + host + ":" + std::to_string(port));
        closesocket(pImpl->socket);
        pImpl->socket = INVALID_SOCKET;
        return false;
    }
    
    // SSL handshake if needed
    if (pImpl->isSSL) {
        pImpl->sslContext = std::make_unique<SChannelContext>();
        if (!pImpl->sslContext->initializeClient(host) ||
            !pImpl->sslContext->performHandshake(pImpl->socket)) {
            utils::Logger::error("SSL handshake failed");
            disconnect();
            return false;
        }
    }
    
    // WebSocket handshake
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<> dis(0, 255);
    
    std::vector<unsigned char> keyBytes(16);
    for (int i = 0; i < 16; i++) keyBytes[i] = dis(gen);
    std::string wsKey = utils::Base64::encode(keyBytes.data(), 16);
    
    std::ostringstream handshake;
    handshake << "GET " << path << " HTTP/1.1\r\n"
              << "Host: " << host << ":" << port << "\r\n"
              << "Upgrade: websocket\r\n"
              << "Connection: Upgrade\r\n"
              << "Sec-WebSocket-Key: " << wsKey << "\r\n"
              << "Sec-WebSocket-Version: 13\r\n"
              << "\r\n";
    
    std::string handshakeStr = handshake.str();
    if (pImpl->socketSend(handshakeStr.c_str(), static_cast<int>(handshakeStr.length())) <= 0) {
        utils::Logger::error("Failed to send WebSocket handshake");
        disconnect();
        return false;
    }
    
    // Read handshake response
    char buffer[4096];
    int received = pImpl->socketRecv(buffer, sizeof(buffer) - 1);
    if (received <= 0) {
        utils::Logger::error("Failed to receive WebSocket handshake response");
        disconnect();
        return false;
    }
    
    buffer[received] = '\0';
    std::string response(buffer);
    
    // Case-insensitive check for upgrade header
    std::string responseLower = response;
    std::transform(responseLower.begin(), responseLower.end(), responseLower.begin(), ::tolower);
    if (response.find("101") == std::string::npos || 
        responseLower.find("upgrade: websocket") == std::string::npos) {
        utils::Logger::error("WebSocket handshake failed: " + response.substr(0, 200));
        disconnect();
        return false;
    }
    
    // Remove socket timeouts — read loop needs to block indefinitely
    int noTimeout = 0;
    setsockopt(pImpl->socket, SOL_SOCKET, SO_RCVTIMEO, reinterpret_cast<char*>(&noTimeout), sizeof(noTimeout));
    setsockopt(pImpl->socket, SOL_SOCKET, SO_SNDTIMEO, reinterpret_cast<char*>(&noTimeout), sizeof(noTimeout));
    
    pImpl->connected = true;
    utils::Logger::info("Connected to " + url);
    
    if (pImpl->stateHandler) {
        pImpl->stateHandler(true, "");
    }
    
    return true;
}

void WebSocketClient::disconnect(const std::string& reason) {
    if (pImpl->connected) {
        utils::Logger::info("Disconnecting: " + reason);
        if (pImpl->stateHandler) {
            pImpl->stateHandler(false, reason);
        }
        pImpl->disconnect();
    }
}

bool WebSocketClient::isConnected() const {
    return pImpl->connected;
}

bool WebSocketClient::sendText(const std::string& text) {
    std::vector<unsigned char> data(text.begin(), text.end());
    return sendFrame(0x1, data);
}

bool WebSocketClient::sendBinary(const std::vector<unsigned char>& data) {
    return sendFrame(0x2, data);
}

bool WebSocketClient::sendBinary(const unsigned char* data, size_t len) {
    std::vector<unsigned char> vec(data, data + len);
    return sendFrame(0x2, vec);
}

bool WebSocketClient::sendPing(const std::vector<unsigned char>& data) {
    return sendFrame(0x9, data);
}

bool WebSocketClient::sendFrame(int opcode, const std::vector<unsigned char>& payload) {
    if (!pImpl->connected) return false;
    
    std::lock_guard<std::mutex> lock(pImpl->sendMutex);
    
    // Generate mask
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<unsigned short> dis(0, 255);
    
    unsigned char mask[4];
    for (int i = 0; i < 4; i++) mask[i] = static_cast<unsigned char>(dis(gen));
    
    std::vector<unsigned char> frame;
    
    // First byte: FIN + opcode
    frame.push_back(0x80 | opcode);
    
    // Payload length + mask bit
    size_t len = payload.size();
    if (len < 126) {
        frame.push_back(0x80 | static_cast<unsigned char>(len));
    } else if (len < 65536) {
        frame.push_back(0x80 | 126);
        frame.push_back((len >> 8) & 0xFF);
        frame.push_back(len & 0xFF);
    } else {
        frame.push_back(0x80 | 127);
        for (int i = 7; i >= 0; i--) {
            frame.push_back((len >> (8 * i)) & 0xFF);
        }
    }
    
    // Mask
    for (int i = 0; i < 4; i++) frame.push_back(mask[i]);
    
    // Masked payload
    for (size_t i = 0; i < payload.size(); i++) {
        frame.push_back(payload[i] ^ mask[i % 4]);
    }
    
    int sent = pImpl->socketSend(frame.data(), static_cast<int>(frame.size()));
    return sent > 0;
}

void WebSocketClient::startReading() {
    if (pImpl->reading || !pImpl->connected) return;
    
    pImpl->reading = true;
    pImpl->readThread = std::thread([this]() {
        readLoop();
    });
}

void WebSocketClient::stopReading() {
    pImpl->reading = false;
    if (pImpl->readThread.joinable()) {
        pImpl->readThread.join();
    }
}

// Helper: read exactly 'needed' bytes from socket (handles partial reads)
bool WebSocketClient::readExact(unsigned char* buf, size_t needed) {
    size_t got = 0;
    while (got < needed && pImpl->reading && pImpl->connected) {
        int r = pImpl->socketRecv(buf + got, static_cast<int>(needed - got));
        if (r <= 0) {
            utils::Logger::error("readExact: recv returned " + std::to_string(r) + 
                " (needed " + std::to_string(needed) + ", got " + std::to_string(got) + ")");
            return false;
        }
        got += r;
    }
    return got == needed;
}

void WebSocketClient::readLoop() {
    unsigned char hdr[2];
    
    while (pImpl->reading && pImpl->connected) {
        try {
            // Read frame header (exactly 2 bytes)
            if (!readExact(hdr, 2)) break;
            
            int opcode = hdr[0] & 0x0F;
            bool fin = (hdr[0] & 0x80) != 0;
            bool masked = (hdr[1] & 0x80) != 0;
            uint64_t payloadLen = hdr[1] & 0x7F;
            
            // Extended payload length
            if (payloadLen == 126) {
                unsigned char ext[2];
                if (!readExact(ext, 2)) break;
                payloadLen = ((ext[0] & 0xFF) << 8) | (ext[1] & 0xFF);
            } else if (payloadLen == 127) {
                unsigned char ext[8];
                if (!readExact(ext, 8)) break;
                payloadLen = 0;
                for (int i = 0; i < 8; i++) {
                    payloadLen = (payloadLen << 8) | (ext[i] & 0xFF);
                }
            }
            
            // Mask key
            unsigned char maskKey[4] = {0};
            if (masked) {
                if (!readExact(maskKey, 4)) break;
            }
            
            // Read payload
            std::vector<unsigned char> payload;
            if (payloadLen > 0) {
                payload.resize(static_cast<size_t>(payloadLen));
                if (!readExact(payload.data(), static_cast<size_t>(payloadLen))) break;
                
                // Unmask
                if (masked) {
                    for (size_t i = 0; i < payload.size(); i++) {
                        payload[i] ^= maskKey[i % 4];
                    }
                }
            }
            
            // Handle frame
            if (opcode == 0x8) { // Close frame
                utils::Logger::info("Received close frame from server");
                disconnect("server_close");
                break;
            } else if (opcode == 0x9) { // Ping frame
                utils::Logger::info("Received ping, sending pong (" + std::to_string(payload.size()) + " bytes)");
                bool pongOk = sendFrame(0xA, payload); // Pong response
                if (!pongOk) {
                    utils::Logger::error("Failed to send pong!");
                }
            } else if (pImpl->messageHandler) {
                pImpl->messageHandler(opcode, payload);
            }
            
        } catch (...) {
            break;
        }
    }
    
    if (pImpl->connected) {
        disconnect("read_error");
    }
}

} // namespace iploop