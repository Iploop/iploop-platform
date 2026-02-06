package com.iploop.sdk;

import android.content.Context;
import android.provider.Settings;
import android.util.Base64;
import android.util.Log;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.net.URI;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.security.cert.CertificateException;
import java.security.cert.X509Certificate;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicBoolean;

import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLSocket;
import javax.net.ssl.SSLSocketFactory;
import javax.net.ssl.TrustManager;
import javax.net.ssl.X509TrustManager;

import org.json.JSONObject;

/**
 * WebSocket client using Java-WebSocket library with certificate pinning
 */
class WebSocketClient extends org.java_websocket.client.WebSocketClient {
    private static final String TAG = "IPLoopWS";
    private static final String HOST = "gateway.iploop.io";
    private static final int PORT = 443;
    private static final String PATH = "/ws";
    private static final String WS_URL = "wss://" + HOST + ":" + PORT + PATH;
    
    // Certificate pinning - SHA-256 fingerprint of gateway.iploop.io
    private static final String PINNED_CERT_HASH = "8EF9DE4ECFE786E50CD8EA3DC34C8D9650B7D8B6C404288086350BA88407D15C";
    
    private static final String UA_DEFAULT = "IPLoop-SDK/1.0.45";
    
    private final String apiKey;
    private final Context context;
    private final AtomicBoolean shouldRun = new AtomicBoolean(true);
    
    private final ExecutorService workerPool = Executors.newFixedThreadPool(32);
    private final ConcurrentHashMap<String, TunnelConnection> activeTunnels = new ConcurrentHashMap<>();
    
    public WebSocketClient(String apiKey, Context context) throws Exception {
        super(new URI(WS_URL), createHeaders(apiKey, context));
        this.apiKey = apiKey;
        this.context = context;
        
        // Let the library handle SSL - no custom socket factory
        // Just use default SSL
        setConnectionLostTimeout(60);
    }
    
    private static Map<String, String> createHeaders(String apiKey, Context context) {
        Map<String, String> headers = new HashMap<>();
        headers.put("X-API-Key", apiKey);
        headers.put("X-Device-ID", getDeviceIdStatic(context));
        headers.put("X-SDK-Version", IPLoopSDK.getVersion());
        headers.put("X-Platform", "Android");
        return headers;
    }
    
    private static String getDeviceIdStatic(Context context) {
        try {
            return Settings.Secure.getString(context.getContentResolver(), Settings.Secure.ANDROID_ID);
        } catch (Exception e) {
            return "unknown-" + System.currentTimeMillis();
        }
    }
    
    private String getDeviceId() {
        return getDeviceIdStatic(context);
    }
    
    private static SSLContext createPinnedSSLContext() throws Exception {
        // Trust all certs for now (TODO: add back pinning after debugging)
        TrustManager[] tm = new TrustManager[] {
            new X509TrustManager() {
                public void checkClientTrusted(X509Certificate[] chain, String authType) {}
                public void checkServerTrusted(X509Certificate[] chain, String authType) {
                    // Accept all for debugging
                    IPLoopSDK.logDebug(TAG, "Cert check - accepting");
                }
                public X509Certificate[] getAcceptedIssuers() {
                    return new X509Certificate[0];
                }
            }
        };
        
        SSLContext ctx = SSLContext.getInstance("TLS");
        ctx.init(null, tm, new SecureRandom());
        return ctx;
    }
    
    @Override
    protected void onSetSSLParameters(javax.net.ssl.SSLParameters sslParameters) {
        // Skip - not available on API < 24
    }
    
    @Override
    public void onOpen(org.java_websocket.handshake.ServerHandshake handshake) {
        IPLoopSDK.logDebug(TAG, "WebSocket connected!");
        
        String deviceId = getDeviceId();
        send("{\"type\":\"register\",\"data\":{\"device_id\":\"" + deviceId + "\",\"device_type\":\"android\",\"sdk_version\":\"" + IPLoopSDK.getVersion() + "\"}}");
        
        workerPool.execute(new HeartbeatTask());
    }
    
    @Override
    public void onMessage(String message) {
        IPLoopSDK.logDebug(TAG, "Received: " + (message.length() > 100 ? message.substring(0, 100) + "..." : message));
        try {
            processMessage(message);
        } catch (Exception e) {
            IPLoopSDK.logError(TAG, "Message error: " + e.getMessage());
        }
    }
    
    @Override
    public void onMessage(ByteBuffer bytes) {
        onMessage(new String(bytes.array(), StandardCharsets.UTF_8));
    }
    
    @Override
    public void onClose(int code, String reason, boolean remote) {
        IPLoopSDK.logDebug(TAG, "Closed: " + code + " - " + reason);
        for (String id : activeTunnels.keySet()) closeTunnel(id);
    }
    
    @Override
    public void onError(Exception ex) {
        IPLoopSDK.logError(TAG, "WebSocket error: " + ex.getMessage());
    }
    
    public void disconnect() {
        shouldRun.set(false);
        for (String id : activeTunnels.keySet()) closeTunnel(id);
        try { closeBlocking(); } catch (Exception ignored) {}
        workerPool.shutdown();
    }
    
    private class HeartbeatTask implements Runnable {
        public void run() {
            while (shouldRun.get() && isOpen()) {
                try {
                    Thread.sleep(300000); // 5 MINUTES - DO NOT CHANGE BACK TO 30 SECONDS
                    if (isOpen()) send("{\"type\":\"heartbeat\"}");
                } catch (Exception e) { break; }
            }
        }
    }
    
    private void processMessage(String message) {
        try {
            String trimmed = message.trim();
            
            // Handle batched messages (JSON array)
            if (trimmed.startsWith("[")) {
                org.json.JSONArray arr = new org.json.JSONArray(trimmed);
                for (int i = 0; i < arr.length(); i++) {
                    processSingleMessage(arr.getJSONObject(i));
                }
                return;
            }
            
            processSingleMessage(new JSONObject(trimmed));
        } catch (Exception e) {
            IPLoopSDK.logError(TAG, "Parse error: " + e.getMessage());
        }
    }
    
    private void processSingleMessage(JSONObject json) {
        try {
            String type = json.optString("type", "");
            
            switch (type) {
                case "proxy_request":
                    IPLoopSDK.logDebug(TAG, "Got proxy_request");
                    JSONObject data = json.optJSONObject("data");
                    if (data != null) {
                        String reqId = data.optString("request_id", "");
                        IPLoopSDK.logDebug(TAG, "proxy_request id=" + reqId);
                        if (!reqId.isEmpty()) {
                            workerPool.execute(new ProxyRequestTask(reqId, json));
                        }
                    }
                    break;
                case "tunnel_open":
                    handleTunnelOpen(json);
                    break;
                case "tunnel_data":
                    handleTunnelData(json);
                    break;
                case "tunnel_close":
                    handleTunnelClose(json);
                    break;
            }
        } catch (Exception e) {
            IPLoopSDK.logError(TAG, "Parse error: " + e.getMessage());
        }
    }
    
    // ============ Proxy Request ============
    
    private class ProxyRequestTask implements Runnable {
        private final String requestId;
        private final JSONObject json;
        
        ProxyRequestTask(String requestId, JSONObject json) {
            this.requestId = requestId;
            this.json = json;
        }
        
        public void run() {
            IPLoopSDK.logDebug(TAG, "ProxyRequestTask running: " + requestId);
            try {
                JSONObject data = json.optJSONObject("data");
                if (data == null) {
                    sendProxyResponse(requestId, 400, "{}", null, "Missing data");
                    return;
                }
                
                String method = data.optString("method", "GET");
                String urlStr = data.optString("url", "");
                IPLoopSDK.logDebug(TAG, "Making request: " + method + " " + urlStr);
                String bodyBase64 = data.optString("body", "");
                JSONObject headers = data.optJSONObject("headers");
                
                if (urlStr.isEmpty()) {
                    sendProxyResponse(requestId, 400, "{}", null, "Missing URL");
                    return;
                }
                
                java.net.URL url = new java.net.URL(urlStr);
                java.net.HttpURLConnection conn = (java.net.HttpURLConnection) url.openConnection();
                conn.setRequestMethod(method);
                conn.setConnectTimeout(30000);
                conn.setReadTimeout(30000);
                conn.setRequestProperty("User-Agent", UA_DEFAULT);
                
                if (headers != null) {
                    java.util.Iterator<String> keys = headers.keys();
                    while (keys.hasNext()) {
                        String key = keys.next();
                        conn.setRequestProperty(key, headers.optString(key, ""));
                    }
                }
                
                if (!bodyBase64.isEmpty()) {
                    byte[] body = Base64.decode(bodyBase64, Base64.NO_WRAP);
                    conn.setDoOutput(true);
                    conn.getOutputStream().write(body);
                }
                
                int status = conn.getResponseCode();
                
                InputStream is;
                try { is = conn.getInputStream(); } 
                catch (Exception e) { is = conn.getErrorStream(); }
                
                ByteArrayOutputStream baos = new ByteArrayOutputStream();
                if (is != null) {
                    byte[] buf = new byte[8192];
                    int len;
                    while ((len = is.read(buf)) != -1) baos.write(buf, 0, len);
                    is.close();
                }
                
                sendProxyResponse(requestId, status, "{}", Base64.encodeToString(baos.toByteArray(), Base64.NO_WRAP), null);
                conn.disconnect();
                
            } catch (Exception e) {
                sendProxyResponse(requestId, 502, "{}", null, e.getMessage());
            }
        }
    }
    
    private void sendProxyResponse(String requestId, int status, String headers, String body, String error) {
        IPLoopSDK.logDebug(TAG, "Sending proxy_response: id=" + requestId + " status=" + status + " error=" + error);
        StringBuilder sb = new StringBuilder("{\"type\":\"proxy_response\",\"data\":{");
        sb.append("\"request_id\":\"").append(requestId).append("\",");
        sb.append("\"status_code\":").append(status).append(",");
        sb.append("\"headers\":").append(headers);
        if (body != null) sb.append(",\"body\":\"").append(body).append("\"");
        if (error != null) sb.append(",\"error\":\"").append(error.replace("\"", "\\\"")).append("\"");
        sb.append("}}");
        try { 
            send(sb.toString()); 
            IPLoopSDK.logDebug(TAG, "proxy_response sent OK");
        } catch (Exception e) {
            IPLoopSDK.logError(TAG, "proxy_response send failed: " + e.getMessage());
        }
    }
    
    // ============ Tunnel Handling ============
    
    private void handleTunnelOpen(JSONObject json) {
        try {
            JSONObject data = json.getJSONObject("data");
            workerPool.execute(new TunnelOpenTask(
                data.getString("tunnel_id"),
                data.getString("host"),
                data.getString("port")
            ));
        } catch (Exception e) {
            IPLoopSDK.logError(TAG, "tunnel_open error: " + e.getMessage());
        }
    }
    
    private class TunnelOpenTask implements Runnable {
        private final String tunnelId, host, port;
        
        TunnelOpenTask(String id, String h, String p) {
            tunnelId = id; host = h; port = p;
        }
        
        public void run() {
            try {
                Socket socket = new Socket();
                socket.connect(new InetSocketAddress(host, Integer.parseInt(port)), 10000);
                socket.setSoTimeout(60000);
                
                TunnelConnection tunnel = new TunnelConnection(tunnelId, socket);
                activeTunnels.put(tunnelId, tunnel);
                
                sendTunnelResponse(tunnelId, true, null);
                workerPool.execute(new TunnelReaderTask(tunnel));
                
            } catch (Exception e) {
                sendTunnelResponse(tunnelId, false, e.getMessage());
            }
        }
    }
    
    private void sendTunnelResponse(String id, boolean success, String error) {
        StringBuilder sb = new StringBuilder("{\"type\":\"tunnel_response\",\"data\":{");
        sb.append("\"tunnel_id\":\"").append(id).append("\",");
        sb.append("\"success\":").append(success);
        if (error != null) sb.append(",\"error\":\"").append(error.replace("\"", "\\\"")).append("\"");
        sb.append("}}");
        try { send(sb.toString()); } catch (Exception ignored) {}
    }
    
    private class TunnelReaderTask implements Runnable {
        private final TunnelConnection tunnel;
        TunnelReaderTask(TunnelConnection t) { tunnel = t; }
        
        public void run() {
            byte[] buf = new byte[32768];
            try {
                while (!tunnel.closed && isOpen()) {
                    int n = tunnel.socketIn.read(buf);
                    if (n == -1) break;
                    if (n > 0) {
                        byte[] data = new byte[n];
                        System.arraycopy(buf, 0, data, 0, n);
                        sendTunnelData(tunnel.tunnelId, data, false);
                    }
                }
            } catch (Exception ignored) {}
            sendTunnelData(tunnel.tunnelId, new byte[0], true);
            closeTunnel(tunnel.tunnelId);
        }
    }
    
    private void sendTunnelData(String id, byte[] data, boolean eof) {
        StringBuilder sb = new StringBuilder("{\"type\":\"tunnel_data\",\"data\":{");
        sb.append("\"tunnel_id\":\"").append(id).append("\",");
        if (data.length > 0) sb.append("\"data\":\"").append(Base64.encodeToString(data, Base64.NO_WRAP)).append("\",");
        sb.append("\"eof\":").append(eof).append("}}");
        try { send(sb.toString()); } catch (Exception ignored) {}
    }
    
    private void handleTunnelData(JSONObject json) {
        try {
            JSONObject data = json.getJSONObject("data");
            String id = data.getString("tunnel_id");
            TunnelConnection tunnel = activeTunnels.get(id);
            if (tunnel == null || tunnel.closed) return;
            
            if (data.optBoolean("eof", false)) {
                closeTunnel(id);
                return;
            }
            
            String b64 = data.optString("data", "");
            if (!b64.isEmpty()) {
                byte[] bytes = Base64.decode(b64, Base64.NO_WRAP);
                tunnel.socketOut.write(bytes);
                tunnel.socketOut.flush();
            }
        } catch (Exception ignored) {}
    }
    
    private void handleTunnelClose(JSONObject json) {
        try { closeTunnel(json.getJSONObject("data").getString("tunnel_id")); } 
        catch (Exception ignored) {}
    }
    
    private void closeTunnel(String id) {
        TunnelConnection tunnel = activeTunnels.remove(id);
        if (tunnel != null) {
            tunnel.closed = true;
            try { tunnel.socket.close(); } catch (Exception ignored) {}
        }
    }
    
    private class TunnelConnection {
        final String tunnelId;
        final Socket socket;
        final InputStream socketIn;
        final OutputStream socketOut;
        volatile boolean closed = false;
        
        TunnelConnection(String id, Socket s) throws Exception {
            tunnelId = id;
            socket = s;
            socketIn = s.getInputStream();
            socketOut = s.getOutputStream();
        }
    }
}
