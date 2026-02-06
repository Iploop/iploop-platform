package com.iploop.sdk;

import android.content.Context;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * IPLoop SDK - Pure Java implementation for dynamic loading
 * No Kotlin, no coroutines, no external dependencies
 */
public class IPLoopSDK {
    private static final String TAG = "IPLoopSDK";
    private static final String VERSION = "1.0.56";
    
    // Logging control
    private static boolean loggingEnabled = false;
    
    private static Context appContext;
    private static String apiKey;
    private static ExecutorService executor;
    private static Handler mainHandler;
    private static WebSocketClient wsClient;
    private static final AtomicBoolean running = new AtomicBoolean(false);
    private static final AtomicInteger reconnectAttempts = new AtomicInteger(0);
    private static final int MAX_RECONNECT_ATTEMPTS = 10;
    private static final long BASE_RECONNECT_DELAY_MS = 1000; // 1 second
    private static final long MAX_RECONNECT_DELAY_MS = 60000; // 60 seconds
    private static final AtomicBoolean consentGiven = new AtomicBoolean(false);
    private static final AtomicInteger status = new AtomicInteger(SDKStatus.IDLE);
    
    // Proxy configuration
    private static ProxyConfig proxyConfig;
    
    // Callbacks
    public interface Callback {
        void onSuccess();
        void onError(String error);
    }
    
    public interface StatusCallback {
        void onStatusChanged(int newStatus);
    }
    
    public interface ProxyCallback {
        void onProxyConfigured(String proxyHost, int proxyPort);
        void onProxyError(String error);
    }
    
    private static StatusCallback statusCallback;
    private static ProxyCallback proxyCallback;
    
    /**
     * Proxy configuration class for enhanced features
     */
    public static class ProxyConfig {
        public String country = "";
        public String city = "";
        public int asn = 0;
        public String sessionId = "";
        public String sessionType = "sticky"; // sticky, rotating, per-request
        public int lifetimeMinutes = 30;
        public String rotateMode = "manual"; // request, time, manual, ip-change
        public int rotateIntervalMinutes = 5;
        public String profile = "chrome-win"; // chrome-win, firefox-mac, mobile-ios, etc.
        public String userAgent = "";
        public int minSpeedMbps = 10;
        public int maxLatencyMs = 1000;
        public boolean debugMode = false;
        
        public ProxyConfig() {}
        
        public ProxyConfig setCountry(String country) {
            this.country = country;
            return this;
        }
        
        public ProxyConfig setCity(String city) {
            this.city = city;
            return this;
        }
        
        public ProxyConfig setASN(int asn) {
            this.asn = asn;
            return this;
        }
        
        public ProxyConfig setSessionId(String sessionId) {
            this.sessionId = sessionId;
            return this;
        }
        
        public ProxyConfig setSessionType(String type) {
            this.sessionType = type;
            return this;
        }
        
        public ProxyConfig setLifetime(int minutes) {
            this.lifetimeMinutes = minutes;
            return this;
        }
        
        public ProxyConfig setRotateMode(String mode) {
            this.rotateMode = mode;
            return this;
        }
        
        public ProxyConfig setRotateInterval(int minutes) {
            this.rotateIntervalMinutes = minutes;
            return this;
        }
        
        public ProxyConfig setProfile(String profile) {
            this.profile = profile;
            return this;
        }
        
        public ProxyConfig setUserAgent(String userAgent) {
            this.userAgent = userAgent;
            return this;
        }
        
        public ProxyConfig setMinSpeed(int mbps) {
            this.minSpeedMbps = mbps;
            return this;
        }
        
        public ProxyConfig setMaxLatency(int ms) {
            this.maxLatencyMs = ms;
            return this;
        }
        
        public ProxyConfig setDebugMode(boolean debug) {
            this.debugMode = debug;
            return this;
        }
        
        /**
         * Generate proxy authentication string with parameters
         */
        public String generateProxyAuth(String customerId, String apiKey) {
            StringBuilder auth = new StringBuilder();
            auth.append(customerId).append(":").append(apiKey);
            
            if (!country.isEmpty()) {
                auth.append("-country-").append(country);
            }
            
            if (!city.isEmpty()) {
                auth.append("-city-").append(city);
            }
            
            if (asn > 0) {
                auth.append("-asn-").append(asn);
            }
            
            if (!sessionId.isEmpty()) {
                auth.append("-session-").append(sessionId);
            }
            
            if (!sessionType.equals("sticky")) {
                auth.append("-sesstype-").append(sessionType);
            }
            
            if (lifetimeMinutes != 30) {
                auth.append("-lifetime-").append(lifetimeMinutes).append("m");
            }
            
            if (!rotateMode.equals("manual")) {
                auth.append("-rotate-").append(rotateMode);
            }
            
            if (rotateIntervalMinutes != 5) {
                auth.append("-rotateint-").append(rotateIntervalMinutes).append("m");
            }
            
            if (!profile.equals("chrome-win")) {
                auth.append("-profile-").append(profile);
            }
            
            if (!userAgent.isEmpty()) {
                auth.append("-ua-").append(userAgent);
            }
            
            if (minSpeedMbps != 10) {
                auth.append("-speed-").append(minSpeedMbps);
            }
            
            if (maxLatencyMs != 1000) {
                auth.append("-latency-").append(maxLatencyMs);
            }
            
            if (debugMode) {
                auth.append("-debug-1");
            }
            
            return auth.toString();
        }
    }
    
    /**
     * Initialize the SDK
     */
    public static void init(Context context, String key) {
        appContext = context.getApplicationContext();
        apiKey = key;
        mainHandler = new Handler(Looper.getMainLooper());
        executor = Executors.newSingleThreadExecutor();
        setStatus(SDKStatus.INITIALIZED);
        logInfo(TAG, "IPLoop SDK v" + VERSION + " initialized");
    }
    
    /**
     * Start the SDK (background thread)
     */
    public static void start() {
        start(null, null);
    }
    
    /**
     * Start with callbacks
     */
    public static void start(final Runnable onSuccess, final Callback callback) {
        if (appContext == null || apiKey == null) {
            if (callback != null) {
                postError(callback, "SDK not initialized. Call init() first.");
            }
            return;
        }
        
        if (!consentGiven.get()) {
            if (callback != null) {
                postError(callback, "User consent not given. Call setConsentGiven(true) first.");
            }
            return;
        }
        
        if (running.get()) {
            if (onSuccess != null) {
                postToMain(onSuccess);
            }
            return;
        }
        
        executor.execute(new Runnable() {
            public void run() {
                try {
                    running.set(true);
                    setStatus(SDKStatus.CONNECTING);
                    
                    // Connect to WebSocket (blocking to ensure connection succeeds)
                    wsClient = new WebSocketClient(apiKey, appContext);
                    wsClient.setOnDisconnectCallback(new Runnable() {
                        @Override
                        public void run() {
                            scheduleReconnect();
                        }
                    });
                    wsClient.connectBlocking();
                    
                    setStatus(SDKStatus.RUNNING);
                    logInfo(TAG, "SDK started successfully");
                    
                    if (onSuccess != null) {
                        postToMain(onSuccess);
                    }
                    if (callback != null) {
                        postSuccess(callback);
                    }
                } catch (final Exception e) {
                    running.set(false);
                    setStatus(SDKStatus.ERROR);
                    logError(TAG, "Failed to start: " + e.getMessage());
                    if (callback != null) {
                        postError(callback, e.getMessage());
                    }
                }
            }
        });
    }
    
    /**
     * Schedule a reconnection attempt with exponential backoff
     */
    private static void scheduleReconnect() {
        if (!running.get()) {
            logDebug(TAG, "Not running, skipping reconnect");
            return;
        }
        
        int attempts = reconnectAttempts.incrementAndGet();
        if (attempts > MAX_RECONNECT_ATTEMPTS) {
            logError(TAG, "Max reconnect attempts reached (" + MAX_RECONNECT_ATTEMPTS + "), giving up");
            setStatus(SDKStatus.ERROR);
            running.set(false);
            return;
        }
        
        // Exponential backoff: 1s, 2s, 4s, 8s, 16s, 32s, 60s, 60s...
        long delay = Math.min(BASE_RECONNECT_DELAY_MS * (1L << (attempts - 1)), MAX_RECONNECT_DELAY_MS);
        logInfo(TAG, "Scheduling reconnect attempt " + attempts + "/" + MAX_RECONNECT_ATTEMPTS + " in " + delay + "ms");
        
        setStatus(SDKStatus.CONNECTING);
        
        executor.submit(new Runnable() {
            @Override
            public void run() {
                try {
                    Thread.sleep(delay);
                    doReconnect();
                } catch (InterruptedException e) {
                    logDebug(TAG, "Reconnect interrupted");
                }
            }
        });
    }
    
    /**
     * Perform the actual reconnection
     */
    private static void doReconnect() {
        if (!running.get()) {
            logDebug(TAG, "Not running, aborting reconnect");
            return;
        }
        
        logInfo(TAG, "Attempting reconnect...");
        
        try {
            // Close old client if exists
            if (wsClient != null) {
                try { wsClient.closeBlocking(); } catch (Exception ignored) {}
            }
            
            // Create new client
            wsClient = new WebSocketClient(apiKey, appContext);
            wsClient.setOnDisconnectCallback(new Runnable() {
                @Override
                public void run() {
                    scheduleReconnect();
                }
            });
            wsClient.connectBlocking();
            
            // Success!
            reconnectAttempts.set(0);
            setStatus(SDKStatus.RUNNING);
            logInfo(TAG, "Reconnected successfully!");
            
        } catch (Exception e) {
            logError(TAG, "Reconnect failed: " + e.getMessage());
            scheduleReconnect();
        }
    }
    
    /**
     * Stop the SDK
     */
    public static void stop() {
        stop(null);
    }
    
    /**
     * Stop with callback
     */
    public static void stop(final Runnable onComplete) {
        executor.execute(new Runnable() {
            public void run() {
                try {
                    if (wsClient != null) {
                        wsClient.disconnect();
                        wsClient = null;
                    }
                    running.set(false);
                    setStatus(SDKStatus.STOPPED);
                    logInfo(TAG, "SDK stopped");
                    
                    if (onComplete != null) {
                        postToMain(onComplete);
                    }
                } catch (Exception e) {
                    logError(TAG, "Error stopping: " + e.getMessage());
                    if (onComplete != null) {
                        postToMain(onComplete);
                    }
                }
            }
        });
    }
    
    /**
     * Set user consent
     */
    public static void setConsentGiven(boolean consent) {
        consentGiven.set(consent);
        logInfo(TAG, "Consent " + (consent ? "granted" : "revoked"));
    }
    
    /**
     * Check if running
     */
    public static boolean isRunning() {
        return running.get();
    }
    
    /**
     * Get current status
     */
    public static int getStatus() {
        return status.get();
    }
    
    /**
     * Get status as string
     */
    public static String getStatusString() {
        return SDKStatus.toString(status.get());
    }
    
    /**
     * Set status callback
     */
    public static void setStatusCallback(StatusCallback callback) {
        statusCallback = callback;
    }
    
    /**
     * Get version
     */
    public static String getVersion() {
        return VERSION;
    }
    
    /**
     * Enable or disable SDK logging (default: disabled)
     */
    public static void setLoggingEnabled(boolean enabled) {
        loggingEnabled = enabled;
    }
    
    /**
     * Check if logging is enabled
     */
    public static boolean isLoggingEnabled() {
        return loggingEnabled;
    }
    
    /**
     * Configure proxy settings (Enhanced v1.0.20)
     */
    public static void configureProxy(ProxyConfig config) {
        proxyConfig = config;
        logInfo(TAG, "Proxy configured: " + config.generateProxyAuth("test", "test"));
    }
    
    /**
     * Get current proxy configuration
     */
    public static ProxyConfig getProxyConfig() {
        if (proxyConfig == null) {
            proxyConfig = new ProxyConfig();
        }
        return proxyConfig;
    }
    
    /**
     * Set proxy callback for connection updates
     */
    public static void setProxyCallback(ProxyCallback callback) {
        proxyCallback = callback;
    }
    
    /**
     * Get proxy authentication string for HTTP proxy
     */
    public static String getProxyAuth(String customerId, String apiKey) {
        if (proxyConfig == null) {
            return customerId + ":" + apiKey;
        }
        return proxyConfig.generateProxyAuth(customerId, apiKey);
    }
    
    /**
     * Get proxy host for HTTP/SOCKS5 proxy
     */
    public static String getProxyHost() {
        return "proxy.iploop.com"; // Can be made configurable
    }
    
    /**
     * Get HTTP proxy port
     */
    public static int getHttpProxyPort() {
        return 8080;
    }
    
    /**
     * Get SOCKS5 proxy port
     */
    public static int getSocks5ProxyPort() {
        return 1080;
    }
    
    /**
     * Create configured HTTP proxy URL
     */
    public static String getHttpProxyUrl(String customerId, String apiKey) {
        String auth = getProxyAuth(customerId, apiKey);
        return "http://" + auth + "@" + getProxyHost() + ":" + getHttpProxyPort();
    }
    
    /**
     * Create configured SOCKS5 proxy URL
     */
    public static String getSocks5ProxyUrl(String customerId, String apiKey) {
        String auth = getProxyAuth(customerId, apiKey);
        return "socks5://" + auth + "@" + getProxyHost() + ":" + getSocks5ProxyPort();
    }
    
    /**
     * Test proxy connectivity
     */
    public static void testProxy(final String customerId, final String apiKey, final Callback callback) {
        if (customerId == null || apiKey == null) {
            if (callback != null) {
                postError(callback, "Customer ID and API key required");
            }
            return;
        }
        
        executor.execute(new Runnable() {
            public void run() {
                try {
                    // Test HTTP proxy connection
                    String proxyAuth = getProxyAuth(customerId, apiKey);
                    logInfo(TAG, "Testing proxy with auth: " + proxyAuth);
                    
                    // Simulate proxy test (in real implementation, make HTTP request)
                    Thread.sleep(1000); // Simulate network call
                    
                    if (proxyCallback != null) {
                        mainHandler.post(new Runnable() {
                            public void run() {
                                proxyCallback.onProxyConfigured(getProxyHost(), getHttpProxyPort());
                            }
                        });
                    }
                    
                    if (callback != null) {
                        postSuccess(callback);
                    }
                    
                    logInfo(TAG, "Proxy test successful");
                    
                } catch (final Exception e) {
                    logError(TAG, "Proxy test failed: " + e.getMessage());
                    
                    if (proxyCallback != null) {
                        mainHandler.post(new Runnable() {
                            public void run() {
                                proxyCallback.onProxyError(e.getMessage());
                            }
                        });
                    }
                    
                    if (callback != null) {
                        postError(callback, e.getMessage());
                    }
                }
            }
        });
    }
    
    // Internal logging helpers
    static void logDebug(String tag, String message) {
        if (loggingEnabled) {
            Log.d(tag, message);
        }
    }
    
    static void logInfo(String tag, String message) {
        if (loggingEnabled) {
            Log.i(tag, message);
        }
    }
    
    static void logError(String tag, String message) {
        if (loggingEnabled) {
            Log.e(tag, message);
        }
    }
    
    // Internal helpers
    private static void setStatus(final int newStatus) {
        status.set(newStatus);
        if (statusCallback != null && mainHandler != null) {
            mainHandler.post(new Runnable() {
                public void run() {
                    statusCallback.onStatusChanged(newStatus);
                }
            });
        }
    }
    
    private static void postToMain(final Runnable runnable) {
        if (mainHandler != null) {
            mainHandler.post(runnable);
        }
    }
    
    private static void postSuccess(final Callback callback) {
        if (mainHandler != null) {
            mainHandler.post(new Runnable() {
                public void run() {
                    callback.onSuccess();
                }
            });
        }
    }
    
    private static void postError(final Callback callback, final String error) {
        if (mainHandler != null) {
            mainHandler.post(new Runnable() {
                public void run() {
                    callback.onError(error);
                }
            });
        }
    }
    
    /**
     * Get the current thread count for this app's process.
     * Useful for detecting thread leaks.
     */
    public static int getThreadCount() {
        ThreadGroup rootGroup = Thread.currentThread().getThreadGroup();
        while (rootGroup.getParent() != null) {
            rootGroup = rootGroup.getParent();
        }
        return rootGroup.activeCount();
    }
    
    /**
     * Log the current thread count with a label.
     * Always logs regardless of loggingEnabled setting (for diagnostics).
     */
    public static void logThreadCount(String label) {
        int count = getThreadCount();
        Log.i(TAG, "[THREADS] " + label + ": " + count + " active threads");
    }
    
    /**
     * Log thread count only if logging is enabled.
     */
    static void logThreadCountDebug(String label) {
        if (loggingEnabled) {
            int count = getThreadCount();
            Log.d(TAG, "[THREADS] " + label + ": " + count + " active threads");
        }
    }
}
