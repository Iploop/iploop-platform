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
    private static final String VERSION = "1.0.19";
    
    // Logging control
    private static boolean loggingEnabled = false;
    
    private static Context appContext;
    private static String apiKey;
    private static ExecutorService executor;
    private static Handler mainHandler;
    private static WebSocketClient wsClient;
    private static final AtomicBoolean running = new AtomicBoolean(false);
    private static final AtomicBoolean consentGiven = new AtomicBoolean(false);
    private static final AtomicInteger status = new AtomicInteger(SDKStatus.IDLE);
    
    // Callbacks
    public interface Callback {
        void onSuccess();
        void onError(String error);
    }
    
    public interface StatusCallback {
        void onStatusChanged(int newStatus);
    }
    
    private static StatusCallback statusCallback;
    
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
                    
                    // Connect to WebSocket
                    wsClient = new WebSocketClient(apiKey, appContext);
                    wsClient.connect();
                    
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
}
