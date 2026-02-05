package com.iploop.sdk;

import android.content.Context;
import android.os.Build;
import android.provider.Settings;
import android.util.Base64;
import android.util.Log;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.security.KeyStore;
import java.security.SecureRandom;
import java.security.cert.Certificate;
import java.security.cert.CertificateFactory;
import java.security.cert.X509Certificate;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicBoolean;

import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLSocket;
import javax.net.ssl.SSLSocketFactory;
import javax.net.ssl.TrustManager;
import javax.net.ssl.TrustManagerFactory;
import javax.net.ssl.X509TrustManager;

/**
 * Minimal WebSocket client - pure Java, no dependencies
 * With bundled Google Trust Services cert for Android 6.0 compatibility
 */
class WebSocketClient {
    private static final String TAG = "IPLoopWS";
    private static final String HOST = "gateway.iploop.io";
    private static final int PORT = 443;
    private static final String PATH = "/ws";
    
    // Browser profile User-Agents
    private static final String UA_CHROME_WIN = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36";
    private static final String UA_CHROME_MAC = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36";
    private static final String UA_FIREFOX_WIN = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0";
    private static final String UA_FIREFOX_MAC = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0";
    private static final String UA_SAFARI_MAC = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15";
    private static final String UA_MOBILE_IOS = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1";
    private static final String UA_MOBILE_ANDROID = "Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36";
    private static final String UA_DEFAULT = "IPLoop-SDK/1.0.20";
    
    // TODO: Add certificate fingerprint pinning here for production
    
    private final String apiKey;
    private final Context context;
    private final AtomicBoolean connected = new AtomicBoolean(false);
    private final AtomicBoolean shouldRun = new AtomicBoolean(true);
    
    private SSLSocket socket;
    private InputStream inputStream;
    private OutputStream outputStream;
    private ExecutorService readerThread;
    private ExecutorService heartbeatThread;
    
    public WebSocketClient(String apiKey, Context context) {
        this.apiKey = apiKey;
        this.context = context;
    }
    
    private SSLSocketFactory createSSLSocketFactory() throws Exception {
        // Trust all certificates (for development/testing)
        // TODO: Add proper cert pinning or fingerprint validation later
        X509TrustManager trustAllCerts = new X509TrustManager() {
            public X509Certificate[] getAcceptedIssuers() {
                return new X509Certificate[0];
            }
            
            public void checkClientTrusted(X509Certificate[] chain, String authType) {
                // Trust all
            }
            
            public void checkServerTrusted(X509Certificate[] chain, String authType) {
                // Trust all - TODO: add fingerprint check
                IPLoopSDK.logDebug(TAG, "Server cert: " + (chain.length > 0 ? chain[0].getSubjectDN() : "none"));
            }
        };
        
        SSLContext sslContext = SSLContext.getInstance("TLS");
        sslContext.init(null, new TrustManager[] { trustAllCerts }, new SecureRandom());
        return sslContext.getSocketFactory();
    }
    
    public void connect() throws Exception {
        shouldRun.set(true);
        
        IPLoopSDK.logDebug(TAG, "Connecting to " + HOST + ":" + PORT + PATH);
        
        // Create SSL socket with custom trust manager
        SSLSocketFactory factory;
        try {
            factory = createSSLSocketFactory();
            IPLoopSDK.logDebug(TAG, "SSL factory created");
        } catch (Exception e) {
            IPLoopSDK.logError(TAG, "Factory error: " + e.getMessage());
            throw new IOException("SSL setup failed: " + e.getMessage());
        }
        
        try {
            socket = (SSLSocket) factory.createSocket(HOST, PORT);
            IPLoopSDK.logDebug(TAG, "Socket connected");
            
            // Force TLS 1.2 for Android 6.0 compatibility
            socket.setEnabledProtocols(new String[]{"TLSv1.2", "TLSv1.1", "TLSv1"});
            
            socket.setSoTimeout(30000);
            socket.startHandshake();
            IPLoopSDK.logDebug(TAG, "SSL handshake OK");
        } catch (Exception e) {
            IPLoopSDK.logError(TAG, "Connect failed: " + e.getClass().getSimpleName() + ": " + e.getMessage());
            Throwable cause = e.getCause();
            while (cause != null) {
                IPLoopSDK.logError(TAG, "Caused by: " + cause.getClass().getSimpleName() + ": " + cause.getMessage());
                cause = cause.getCause();
            }
            throw new IOException("SSL failed: " + e.getMessage());
        }
        
        inputStream = socket.getInputStream();
        outputStream = socket.getOutputStream();
        
        // WebSocket handshake
        String wsKey = generateWebSocketKey();
        String deviceId = getDeviceId();
        
        String handshake = "GET " + PATH + " HTTP/1.1\r\n" +
                "Host: " + HOST + "\r\n" +
                "Upgrade: websocket\r\n" +
                "Connection: Upgrade\r\n" +
                "Sec-WebSocket-Key: " + wsKey + "\r\n" +
                "Sec-WebSocket-Version: 13\r\n" +
                "X-API-Key: " + apiKey + "\r\n" +
                "X-Device-ID: " + deviceId + "\r\n" +
                "X-SDK-Version: 1.0.13\r\n" +
                "X-Platform: Android\r\n" +
                "\r\n";
        
        IPLoopSDK.logDebug(TAG, "Sending WebSocket handshake");
        outputStream.write(handshake.getBytes(StandardCharsets.UTF_8));
        outputStream.flush();
        
        // Read response
        byte[] buffer = new byte[1024];
        int len = inputStream.read(buffer);
        if (len <= 0) {
            throw new IOException("Handshake failed: no response from server");
        }
        String response = new String(buffer, 0, len, StandardCharsets.UTF_8);
        IPLoopSDK.logDebug(TAG, "Response: " + response.substring(0, Math.min(50, response.length())));
        
        if (!response.contains("101") || !response.toLowerCase().contains("upgrade")) {
            String firstLine = response.split("\r\n")[0];
            throw new IOException("Handshake failed: " + firstLine);
        }
        
        connected.set(true);
        IPLoopSDK.logDebug(TAG, "WebSocket connected!");
        
        // Send registration (nested data structure)
        sendText("{\"type\":\"register\",\"data\":{\"device_id\":\"" + deviceId + "\",\"device_type\":\"android\",\"sdk_version\":\"1.0.19\"}}");
        
        // Start reader thread
        readerThread = Executors.newSingleThreadExecutor();
        readerThread.execute(new Runnable() {
            public void run() {
                readLoop();
            }
        });
        
        // Start heartbeat
        heartbeatThread = Executors.newSingleThreadExecutor();
        heartbeatThread.execute(new Runnable() {
            public void run() {
                heartbeatLoop();
            }
        });
    }
    
    public void disconnect() {
        shouldRun.set(false);
        connected.set(false);
        
        try {
            if (socket != null && !socket.isClosed()) {
                sendCloseFrame();
                socket.close();
            }
        } catch (Exception e) {
            IPLoopSDK.logError(TAG, "Error closing socket: " + e.getMessage());
        }
        
        if (readerThread != null) readerThread.shutdownNow();
        if (heartbeatThread != null) heartbeatThread.shutdownNow();
    }
    
    private void readLoop() {
        while (shouldRun.get() && connected.get()) {
            try {
                int firstByte = inputStream.read();
                if (firstByte == -1) break;
                
                int secondByte = inputStream.read();
                if (secondByte == -1) break;
                
                int opcode = firstByte & 0x0F;
                boolean masked = (secondByte & 0x80) != 0;
                int payloadLen = secondByte & 0x7F;
                
                if (payloadLen == 126) {
                    payloadLen = (inputStream.read() << 8) | inputStream.read();
                } else if (payloadLen == 127) {
                    for (int i = 0; i < 4; i++) inputStream.read();
                    payloadLen = (inputStream.read() << 24) | (inputStream.read() << 16) | 
                                 (inputStream.read() << 8) | inputStream.read();
                }
                
                byte[] maskKey = null;
                if (masked) {
                    maskKey = new byte[4];
                    inputStream.read(maskKey);
                }
                
                byte[] payload = new byte[payloadLen];
                int read = 0;
                while (read < payloadLen) {
                    int r = inputStream.read(payload, read, payloadLen - read);
                    if (r == -1) break;
                    read += r;
                }
                
                if (masked && maskKey != null) {
                    for (int i = 0; i < payload.length; i++) {
                        payload[i] ^= maskKey[i % 4];
                    }
                }
                
                switch (opcode) {
                    case 0x1: handleMessage(new String(payload, StandardCharsets.UTF_8)); break;
                    case 0x2: handleBinaryMessage(payload); break;
                    case 0x8: disconnect(); break;
                    case 0x9: sendPong(payload); break;
                }
                
            } catch (Exception e) {
                if (shouldRun.get()) IPLoopSDK.logError(TAG, "Read error: " + e.getMessage());
                break;
            }
        }
    }
    
    private void heartbeatLoop() {
        while (shouldRun.get() && connected.get()) {
            try {
                Thread.sleep(30000); // 30 seconds (server timeout is 60s)
                if (connected.get()) sendText("{\"type\":\"heartbeat\"}");
            } catch (InterruptedException e) {
                break;
            } catch (Exception e) {
                IPLoopSDK.logError(TAG, "Heartbeat error: " + e.getMessage());
            }
        }
    }
    
    private void handleMessage(String message) {
        IPLoopSDK.logDebug(TAG, "Received: " + message);
        
        // Simple JSON parsing without org.json dependency for d8 compatibility
        if (message.contains("\"type\":\"proxy_request\"")) {
            // Gateway sends request_id in data object
            String requestId = extractJsonString(message, "request_id");
            if (requestId != null && !requestId.isEmpty()) {
                handleProxyRequest(requestId, message);
            } else {
                IPLoopSDK.logError(TAG, "proxy_request missing request_id");
            }
        } else if (message.contains("\"type\":\"tunnel_open\"")) {
            // Handle tunnel open request
            handleTunnelOpen(message);
        } else if (message.contains("\"type\":\"tunnel_data\"")) {
            // Handle incoming tunnel data
            handleTunnelData(message);
        } else if (message.contains("\"type\":\"tunnel_close\"")) {
            // Handle tunnel close
            handleTunnelClose(message);
        } else if (message.contains("\"type\":\"pong\"")) {
            IPLoopSDK.logDebug(TAG, "Received pong");
        }
    }
    
    private String extractJsonString(String json, String key) {
        String pattern = "\"" + key + "\":\"";
        int start = json.indexOf(pattern);
        if (start == -1) return null;
        start += pattern.length();
        int end = json.indexOf("\"", start);
        if (end == -1) return null;
        return json.substring(start, end);
    }
    
    private void handleProxyRequest(String requestId, String rawMessage) {
        // Execute in background thread
        ProxyRequestTask task = new ProxyRequestTask(requestId, rawMessage);
        Executors.newSingleThreadExecutor().execute(task);
    }
    
    // Named inner class to avoid d8 issues with anonymous classes capturing variables
    private class ProxyRequestTask implements Runnable {
        private final String requestId;
        private final String rawMessage;
        
        ProxyRequestTask(String requestId, String rawMessage) {
            this.requestId = requestId;
            this.rawMessage = rawMessage;
        }
        
        public void run() {
            executeProxyRequest(requestId, rawMessage);
        }
    }
    
    private void executeProxyRequest(String requestId, String rawMessage) {
        long startTime = System.currentTimeMillis();
        try {
            // Gateway sends data in "data" object, not "payload"
            int dataStart = rawMessage.indexOf("\"data\":");
            if (dataStart == -1) {
                sendProxyResponse(requestId, 400, "{}", null, "Missing data");
                return;
            }
            String dataJson = rawMessage.substring(dataStart);
            
            String method = extractJsonString(dataJson, "method");
            String urlStr = extractJsonString(dataJson, "url");
            String bodyBase64 = extractJsonString(dataJson, "body");
            String requestProfile = extractJsonString(dataJson, "profile"); // Per-request profile
            
            if (method == null) method = "GET";
            if (urlStr == null || urlStr.isEmpty()) {
                sendProxyResponse(requestId, 400, "{}", null, "Missing URL");
                return;
            }
            
            IPLoopSDK.logDebug(TAG, "Proxy request: " + method + " " + urlStr + " (profile: " + requestProfile + ")");
            
            // Execute HTTP request
            java.net.URL url = new java.net.URL(urlStr);
            java.net.HttpURLConnection conn = (java.net.HttpURLConnection) url.openConnection();
            conn.setRequestMethod(method);
            conn.setConnectTimeout(30000);
            conn.setReadTimeout(30000);
            conn.setRequestProperty("User-Agent", getProfileUserAgent(requestProfile));
            
            // Send body if present
            if (bodyBase64 != null && !bodyBase64.isEmpty() && !"GET".equals(method)) {
                conn.setDoOutput(true);
                byte[] body = Base64.decode(bodyBase64, Base64.NO_WRAP);
                conn.getOutputStream().write(body);
            }
            
            // Get response
            int statusCode = conn.getResponseCode();
            
            // Build response headers JSON
            StringBuilder headersJson = new StringBuilder("{");
            boolean first = true;
            for (int i = 0; ; i++) {
                String key = conn.getHeaderFieldKey(i);
                String value = conn.getHeaderField(i);
                if (key == null && value == null) break;
                if (key != null && value != null) {
                    if (!first) headersJson.append(",");
                    headersJson.append("\"").append(escapeJson(key)).append("\":\"").append(escapeJson(value)).append("\"");
                    first = false;
                }
            }
            headersJson.append("}");
            
            // Read response body
            InputStream is = statusCode >= 400 ? conn.getErrorStream() : conn.getInputStream();
            java.io.ByteArrayOutputStream baos = new java.io.ByteArrayOutputStream();
            if (is != null) {
                byte[] buffer = new byte[8192];
                int len;
                while ((len = is.read(buffer)) != -1) {
                    baos.write(buffer, 0, len);
                }
                is.close();
            }
            byte[] respBody = baos.toByteArray();
            String respBodyBase64 = Base64.encodeToString(respBody, Base64.NO_WRAP);
            
            conn.disconnect();
            
            // Send response
            long elapsed = System.currentTimeMillis() - startTime;
            sendProxyResponse(requestId, statusCode, headersJson.toString(), respBodyBase64, null);
            IPLoopSDK.logDebug(TAG, "Proxy response: " + statusCode + " | " + respBody.length + " bytes | " + elapsed + "ms");
            
        } catch (Exception e) {
            long elapsed = System.currentTimeMillis() - startTime;
            IPLoopSDK.logError(TAG, "Proxy request failed after " + elapsed + "ms: " + e.getMessage());
            sendProxyResponse(requestId, 502, "{}", null, e.getMessage());
        }
    }
    
    private String escapeJson(String s) {
        if (s == null) return "";
        return s.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", "\\n").replace("\r", "\\r");
    }
    
    private void sendProxyResponse(String requestId, int statusCode, String headersJson, String bodyBase64, String error) {
        try {
            StringBuilder json = new StringBuilder();
            // Gateway expects: request_id, success, status_code, headers, body, error
            json.append("{\"type\":\"proxy_response\",\"data\":{");
            json.append("\"request_id\":\"").append(requestId).append("\",");
            json.append("\"success\":").append(error == null ? "true" : "false").append(",");
            json.append("\"status_code\":").append(statusCode);
            if (headersJson != null) json.append(",\"headers\":").append(headersJson);
            if (bodyBase64 != null) json.append(",\"body\":\"").append(bodyBase64).append("\"");
            if (error != null) json.append(",\"error\":\"").append(escapeJson(error)).append("\"");
            json.append("}}");
            
            sendText(json.toString());
            IPLoopSDK.logDebug(TAG, "Sent proxy_response for " + requestId);
        } catch (Exception e) {
            IPLoopSDK.logError(TAG, "Failed to send proxy response: " + e.getMessage());
        }
    }
    
    private void handleBinaryMessage(byte[] data) {
        IPLoopSDK.logDebug(TAG, "Received binary: " + data.length + " bytes");
    }
    
    public void sendText(String message) throws IOException {
        sendFrame(0x1, message.getBytes(StandardCharsets.UTF_8));
    }
    
    public void sendBinary(byte[] data) throws IOException {
        sendFrame(0x2, data);
    }
    
    private void sendPong(byte[] payload) throws IOException {
        sendFrame(0xA, payload);
    }
    
    private void sendCloseFrame() throws IOException {
        sendFrame(0x8, new byte[0]);
    }
    
    private synchronized void sendFrame(int opcode, byte[] payload) throws IOException {
        if (outputStream == null) return;
        
        byte[] maskKey = new byte[4];
        new SecureRandom().nextBytes(maskKey);
        
        int headerLen = 2 + 4;
        if (payload.length > 125) headerLen += (payload.length > 65535) ? 8 : 2;
        
        byte[] frame = new byte[headerLen + payload.length];
        int idx = 0;
        
        frame[idx++] = (byte) (0x80 | opcode);
        
        if (payload.length <= 125) {
            frame[idx++] = (byte) (0x80 | payload.length);
        } else if (payload.length <= 65535) {
            frame[idx++] = (byte) (0x80 | 126);
            frame[idx++] = (byte) (payload.length >> 8);
            frame[idx++] = (byte) payload.length;
        } else {
            frame[idx++] = (byte) (0x80 | 127);
            for (int i = 0; i < 4; i++) frame[idx++] = 0;
            frame[idx++] = (byte) (payload.length >> 24);
            frame[idx++] = (byte) (payload.length >> 16);
            frame[idx++] = (byte) (payload.length >> 8);
            frame[idx++] = (byte) payload.length;
        }
        
        System.arraycopy(maskKey, 0, frame, idx, 4);
        idx += 4;
        
        for (int i = 0; i < payload.length; i++) {
            frame[idx++] = (byte) (payload[i] ^ maskKey[i % 4]);
        }
        
        outputStream.write(frame);
        outputStream.flush();
    }
    
    private String generateWebSocketKey() {
        byte[] bytes = new byte[16];
        new SecureRandom().nextBytes(bytes);
        return Base64.encodeToString(bytes, Base64.NO_WRAP);
    }
    
    /**
     * Get User-Agent based on configured browser profile
     * @param requestProfile Override profile from request (can be null)
     */
    private String getProfileUserAgent(String requestProfile) {
        IPLoopSDK.ProxyConfig config = IPLoopSDK.getProxyConfig();
        
        // If custom userAgent is set in config (and no request override), use it
        if ((requestProfile == null || requestProfile.isEmpty()) && 
            config != null && config.userAgent != null && !config.userAgent.isEmpty()) {
            return config.userAgent;
        }
        
        // Use request profile if provided, otherwise use config profile
        String profile;
        if (requestProfile != null && !requestProfile.isEmpty()) {
            profile = requestProfile;
        } else {
            profile = (config != null && config.profile != null) ? config.profile : "";
        }
        
        switch (profile.toLowerCase()) {
            case "chrome-win":
                return UA_CHROME_WIN;
            case "chrome-mac":
                return UA_CHROME_MAC;
            case "firefox-win":
                return UA_FIREFOX_WIN;
            case "firefox-mac":
                return UA_FIREFOX_MAC;
            case "safari-mac":
                return UA_SAFARI_MAC;
            case "mobile-ios":
                return UA_MOBILE_IOS;
            case "mobile-android":
                return UA_MOBILE_ANDROID;
            default:
                return UA_DEFAULT;
        }
    }
    
    private String getDeviceId() {
        try {
            return Settings.Secure.getString(context.getContentResolver(), Settings.Secure.ANDROID_ID);
        } catch (Exception e) {
            return "unknown-" + System.currentTimeMillis();
        }
    }
    
    // ========== TUNNEL SUPPORT ==========
    
    // Active tunnels map: tunnelId -> TunnelConnection
    private final java.util.concurrent.ConcurrentHashMap<String, TunnelConnection> activeTunnels = 
        new java.util.concurrent.ConcurrentHashMap<>();
    
    private static class TunnelConnection {
        final String tunnelId;
        final java.net.Socket socket;
        final OutputStream socketOut;
        final InputStream socketIn;
        volatile boolean closed = false;
        
        TunnelConnection(String tunnelId, java.net.Socket socket) throws IOException {
            this.tunnelId = tunnelId;
            this.socket = socket;
            this.socketOut = socket.getOutputStream();
            this.socketIn = socket.getInputStream();
        }
        
        void close() {
            closed = true;
            try { socket.close(); } catch (Exception ignored) {}
        }
    }
    
    private void handleTunnelOpen(String message) {
        // Extract tunnel parameters
        String tunnelId = extractJsonString(message, "tunnel_id");
        String host = extractJsonString(message, "host");
        String port = extractJsonString(message, "port");
        
        if (tunnelId == null || host == null || port == null) {
            IPLoopSDK.logError(TAG, "tunnel_open missing required fields");
            return;
        }
        
        IPLoopSDK.logDebug(TAG, "Opening tunnel " + tunnelId + " to " + host + ":" + port);
        
        // Execute in background thread (using inner class for d8 compatibility)
        TunnelOpenTask task = new TunnelOpenTask(tunnelId, host, port);
        Executors.newSingleThreadExecutor().execute(task);
    }
    
    // Named inner class for d8 compatibility (avoids lambda)
    private class TunnelOpenTask implements Runnable {
        private final String tunnelId;
        private final String host;
        private final String port;
        
        TunnelOpenTask(String tunnelId, String host, String port) {
            this.tunnelId = tunnelId;
            this.host = host;
            this.port = port;
        }
        
        public void run() {
            try {
                // Connect to target
                java.net.Socket socket = new java.net.Socket();
                socket.connect(new InetSocketAddress(host, Integer.parseInt(port)), 10000);
                socket.setSoTimeout(60000);
                
                TunnelConnection tunnel = new TunnelConnection(tunnelId, socket);
                activeTunnels.put(tunnelId, tunnel);
                
                // Send success response
                sendTunnelOpenResponse(tunnelId, true, null);
                IPLoopSDK.logDebug(TAG, "Tunnel " + tunnelId + " connected to " + host + ":" + port);
                
                // Start reading from socket in background
                startTunnelReader(tunnel);
                
            } catch (Exception e) {
                IPLoopSDK.logError(TAG, "Failed to open tunnel " + tunnelId + ": " + e.getMessage());
                sendTunnelOpenResponse(tunnelId, false, e.getMessage());
            }
        }
    }
    
    private void sendTunnelOpenResponse(String tunnelId, boolean success, String error) {
        StringBuilder json = new StringBuilder();
        json.append("{\"type\":\"tunnel_response\",\"data\":{");
        json.append("\"tunnel_id\":\"").append(tunnelId).append("\",");
        json.append("\"success\":").append(success);
        if (error != null) {
            json.append(",\"error\":\"").append(error.replace("\"", "\\\"")).append("\"");
        }
        json.append("}}");
        try {
            sendFrame(1, json.toString().getBytes(StandardCharsets.UTF_8));
        } catch (IOException e) {
            IPLoopSDK.logError(TAG, "Failed to send tunnel_open_response: " + e.getMessage());
        }
    }
    
    private void startTunnelReader(TunnelConnection tunnel) {
        // Using inner class for d8 compatibility (avoids lambda)
        TunnelReaderTask task = new TunnelReaderTask(tunnel);
        Executors.newSingleThreadExecutor().execute(task);
    }
    
    // Named inner class for d8 compatibility
    private class TunnelReaderTask implements Runnable {
        private final TunnelConnection tunnel;
        
        TunnelReaderTask(TunnelConnection tunnel) {
            this.tunnel = tunnel;
        }
        
        public void run() {
            byte[] buffer = new byte[32768];
            try {
                while (!tunnel.closed && connected.get()) {
                    int bytesRead = tunnel.socketIn.read(buffer);
                    if (bytesRead == -1) {
                        // EOF - connection closed by remote
                        break;
                    }
                    if (bytesRead > 0) {
                        // Send data to server
                        byte[] data = new byte[bytesRead];
                        System.arraycopy(buffer, 0, data, 0, bytesRead);
                        sendTunnelData(tunnel.tunnelId, data, false);
                    }
                }
            } catch (Exception e) {
                if (!tunnel.closed) {
                    IPLoopSDK.logDebug(TAG, "Tunnel " + tunnel.tunnelId + " read error: " + e.getMessage());
                }
            } finally {
                // Send EOF and cleanup
                sendTunnelData(tunnel.tunnelId, new byte[0], true);
                closeTunnel(tunnel.tunnelId);
            }
        }
    }
    
    private void sendTunnelData(String tunnelId, byte[] data, boolean eof) {
        String base64Data = Base64.encodeToString(data, Base64.NO_WRAP);
        StringBuilder json = new StringBuilder();
        json.append("{\"type\":\"tunnel_data\",\"data\":{");
        json.append("\"tunnel_id\":\"").append(tunnelId).append("\",");
        json.append("\"data\":\"").append(base64Data).append("\",");
        json.append("\"eof\":").append(eof);
        json.append("}}");
        try {
            sendFrame(1, json.toString().getBytes(StandardCharsets.UTF_8));
        } catch (IOException e) {
            IPLoopSDK.logError(TAG, "Failed to send tunnel_data: " + e.getMessage());
        }
    }
    
    private void handleTunnelData(String message) {
        String tunnelId = extractJsonString(message, "tunnel_id");
        String base64Data = extractJsonString(message, "data");
        boolean eof = message.contains("\"eof\":true");
        
        if (tunnelId == null) return;
        
        TunnelConnection tunnel = activeTunnels.get(tunnelId);
        if (tunnel == null || tunnel.closed) {
            IPLoopSDK.logDebug(TAG, "Received data for unknown/closed tunnel: " + tunnelId);
            return;
        }
        
        if (eof) {
            closeTunnel(tunnelId);
            return;
        }
        
        if (base64Data != null && !base64Data.isEmpty()) {
            try {
                byte[] data = Base64.decode(base64Data, Base64.DEFAULT);
                tunnel.socketOut.write(data);
                tunnel.socketOut.flush();
            } catch (Exception e) {
                IPLoopSDK.logError(TAG, "Failed to write to tunnel " + tunnelId + ": " + e.getMessage());
                closeTunnel(tunnelId);
            }
        }
    }
    
    private void handleTunnelClose(String message) {
        String tunnelId = extractJsonString(message, "tunnel_id");
        if (tunnelId != null) {
            closeTunnel(tunnelId);
        }
    }
    
    private void closeTunnel(String tunnelId) {
        TunnelConnection tunnel = activeTunnels.remove(tunnelId);
        if (tunnel != null) {
            tunnel.close();
            IPLoopSDK.logDebug(TAG, "Closed tunnel: " + tunnelId);
        }
    }
}
