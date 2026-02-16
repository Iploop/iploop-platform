// Linux WebSocket test client — same framing as Windows SDK, using OpenSSL
// Tests frame parsing against real gateway
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <netdb.h>
#include <arpa/inet.h>
#include <openssl/ssl.h>
#include <openssl/err.h>
#include <random>
#include <string>
#include <vector>
#include <thread>
#include <chrono>
#include <atomic>

SSL *g_ssl = nullptr;
std::atomic<bool> g_running{true};

// Base64 encode (simple)
static const char b64chars[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
std::string base64_encode(const unsigned char* data, size_t len) {
    std::string out;
    int val = 0, valb = -6;
    for (size_t i = 0; i < len; i++) {
        val = (val << 8) + data[i];
        valb += 8;
        while (valb >= 0) {
            out.push_back(b64chars[(val >> valb) & 0x3F]);
            valb -= 6;
        }
    }
    if (valb > -6) out.push_back(b64chars[((val << 8) >> (valb + 8)) & 0x3F]);
    while (out.size() % 4) out.push_back('=');
    return out;
}

int ssl_recv(void* buf, int len) {
    return SSL_read(g_ssl, buf, len);
}

int ssl_send(const void* buf, int len) {
    return SSL_write(g_ssl, buf, len);
}

// Same readExact as Windows SDK
bool readExact(unsigned char* buf, size_t needed) {
    size_t got = 0;
    while (got < needed) {
        int r = ssl_recv(buf + got, (int)(needed - got));
        if (r <= 0) {
            printf("[readExact] recv returned %d (needed %zu, got %zu)\n", r, needed, got);
            return false;
        }
        got += r;
    }
    return true;
}

// Same sendFrame as Windows SDK
bool sendFrame(int opcode, const unsigned char* payload, size_t len) {
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<unsigned short> dis(0, 255);

    unsigned char mask[4];
    for (int i = 0; i < 4; i++) mask[i] = (unsigned char)dis(gen);

    std::vector<unsigned char> frame;
    frame.push_back(0x80 | opcode);

    if (len < 126) {
        frame.push_back(0x80 | (unsigned char)len);
    } else if (len < 65536) {
        frame.push_back(0x80 | 126);
        frame.push_back((len >> 8) & 0xFF);
        frame.push_back(len & 0xFF);
    } else {
        frame.push_back(0x80 | 127);
        for (int i = 7; i >= 0; i--) frame.push_back((len >> (8*i)) & 0xFF);
    }

    for (int i = 0; i < 4; i++) frame.push_back(mask[i]);
    for (size_t i = 0; i < len; i++) frame.push_back(payload[i] ^ mask[i % 4]);

    return ssl_send(frame.data(), (int)frame.size()) > 0;
}

bool sendText(const std::string& text) {
    return sendFrame(0x1, (const unsigned char*)text.c_str(), text.size());
}

void readLoop() {
    unsigned char hdr[2];
    int frameCount = 0;

    while (g_running) {
        if (!readExact(hdr, 2)) break;

        int opcode = hdr[0] & 0x0F;
        bool fin = (hdr[0] & 0x80) != 0;
        bool masked = (hdr[1] & 0x80) != 0;
        uint64_t payloadLen = hdr[1] & 0x7F;

        printf("[Frame #%d] hdr=[0x%02X 0x%02X] fin=%d opcode=%d masked=%d rawlen=%lu\n",
               ++frameCount, hdr[0], hdr[1], fin, opcode, masked, (unsigned long)payloadLen);

        if (payloadLen == 126) {
            unsigned char ext[2];
            if (!readExact(ext, 2)) break;
            payloadLen = (ext[0] << 8) | ext[1];
            printf("  Extended len (16-bit): %lu\n", (unsigned long)payloadLen);
        } else if (payloadLen == 127) {
            unsigned char ext[8];
            if (!readExact(ext, 8)) break;
            payloadLen = 0;
            for (int i = 0; i < 8; i++) payloadLen = (payloadLen << 8) | ext[i];
            printf("  Extended len (64-bit): %lu\n", (unsigned long)payloadLen);
        }

        unsigned char maskKey[4] = {};
        if (masked) {
            if (!readExact(maskKey, 4)) break;
        }

        std::vector<unsigned char> payload;
        if (payloadLen > 0) {
            payload.resize((size_t)payloadLen);
            if (!readExact(payload.data(), (size_t)payloadLen)) break;
            if (masked) {
                for (size_t i = 0; i < payload.size(); i++) payload[i] ^= maskKey[i % 4];
            }
        }

        if (opcode == 0x8) {
            printf("  -> Close frame\n");
            break;
        } else if (opcode == 0x9) {
            printf("  -> Ping! Sending pong...\n");
            sendFrame(0xA, payload.data(), payload.size());
        } else if (opcode == 0x1) {
            std::string msg(payload.begin(), payload.end());
            printf("  -> Text (%lu bytes): %s\n", (unsigned long)msg.size(), msg.c_str());
        } else if (opcode == 0x2) {
            printf("  -> Binary (%lu bytes)\n", (unsigned long)payload.size());
            // Print first 40 bytes as hex
            for (size_t i = 0; i < std::min(payload.size(), (size_t)40); i++) {
                printf("%02X ", payload[i]);
            }
            printf("\n");
        } else {
            printf("  -> Unknown opcode %d\n", opcode);
        }
    }

    printf("[readLoop] exited\n");
    g_running = false;
}

int main() {
    setbuf(stdout, NULL); // disable buffering
    setbuf(stderr, NULL);
    const char* host = "gateway.iploop.io";
    int port = 9443;

    // Resolve
    struct addrinfo hints = {}, *res;
    hints.ai_family = AF_INET;
    hints.ai_socktype = SOCK_STREAM;
    if (getaddrinfo(host, std::to_string(port).c_str(), &hints, &res) != 0) {
        printf("DNS resolve failed\n"); return 1;
    }

    int sock = socket(AF_INET, SOCK_STREAM, 0);
    if (connect(sock, res->ai_addr, res->ai_addrlen) != 0) {
        printf("TCP connect failed\n"); return 1;
    }
    freeaddrinfo(res);
    printf("TCP connected to %s:%d\n", host, port);

    // SSL
    SSL_library_init();
    SSL_CTX* ctx = SSL_CTX_new(TLS_client_method());
    g_ssl = SSL_new(ctx);
    SSL_set_fd(g_ssl, sock);
    SSL_set_tlsext_host_name(g_ssl, host);
    if (SSL_connect(g_ssl) != 1) {
        printf("SSL handshake failed\n"); return 1;
    }
    printf("SSL connected\n");

    // WebSocket handshake
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<> dis(0, 255);
    unsigned char keyBytes[16];
    for (int i = 0; i < 16; i++) keyBytes[i] = dis(gen);
    std::string wsKey = base64_encode(keyBytes, 16);

    char req[512];
    snprintf(req, sizeof(req),
        "GET /ws HTTP/1.1\r\n"
        "Host: %s:%d\r\n"
        "Upgrade: websocket\r\n"
        "Connection: Upgrade\r\n"
        "Sec-WebSocket-Key: %s\r\n"
        "Sec-WebSocket-Version: 13\r\n"
        "\r\n", host, port, wsKey.c_str());

    ssl_send(req, strlen(req));

    char resp[4096];
    int n = ssl_recv(resp, sizeof(resp) - 1);
    resp[n] = 0;
    printf("Handshake response:\n%s\n", resp);

    if (!strstr(resp, "101")) {
        printf("Handshake failed!\n"); return 1;
    }
    printf("WebSocket connected!\n\n");

    // Start read thread
    std::thread reader(readLoop);

    // Send hello
    std::string hello = "{\"type\":\"hello\",\"node_id\":\"linux-test-" + std::to_string(time(nullptr)) + "\",\"device_model\":\"Linux Test Client\",\"sdk_version\":\"2.0\"}";
    printf("Sending: %s\n\n", hello.c_str());
    sendText(hello);

    // Wait — the server should send welcome, then we wait for tunnel_open
    printf("Waiting for messages (Ctrl+C to quit)...\n\n");

    // Keepalive loop
    while (g_running) {
        std::this_thread::sleep_for(std::chrono::seconds(55));
        if (!g_running) break;
        std::string ka = "{\"type\":\"keepalive\",\"uptime_sec\":55}";
        printf("Sending keepalive\n");
        sendText(ka);
    }

    reader.join();
    SSL_shutdown(g_ssl);
    SSL_free(g_ssl);
    close(sock);
    SSL_CTX_free(ctx);
    return 0;
}
