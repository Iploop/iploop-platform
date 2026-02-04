package com.iploop.sdk

import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Handler
import android.os.Looper
import com.iploop.sdk.internal.*
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

/**
 * IPLoop Android SDK - Main entry point
 * 
 * Simple Java-compatible API for integrating IPLoop proxy functionality.
 */
object IPLoopSDK {
    
    private lateinit var applicationContext: Context
    private lateinit var sdkKey: String
    private lateinit var config: IPLoopConfig
    
    private var connectionManager: ConnectionManager? = null
    private var bandwidthTracker: BandwidthTracker? = null
    private var consentManager: ConsentManager? = null
    
    private val executor: ExecutorService = Executors.newSingleThreadExecutor()
    private val mainHandler = Handler(Looper.getMainLooper())
    
    private val _status = AtomicReference(SDKStatus.STOPPED)
    private val _isRunning = AtomicBoolean(false)
    private val _isInitialized = AtomicBoolean(false)
    
    /**
     * Initialize the SDK (Java-friendly, config optional)
     */
    @JvmStatic
    fun init(context: Context, sdkKey: String) {
        init(context, sdkKey, null)
    }
    
    /**
     * Initialize the SDK
     * @param config Optional - if null, uses defaults
     */
    @JvmStatic
    fun init(context: Context, sdkKey: String, config: IPLoopConfig?) {
        if (_isInitialized.get()) {
            IPLoopLogger.w("IPLoopSDK", "Already initialized, ignoring")
            return
        }
        
        this.applicationContext = context.applicationContext
        this.sdkKey = sdkKey
        this.config = config ?: IPLoopConfig.createDefault()
        
        IPLoopLogger.i("IPLoopSDK", "Initialized with key: ${sdkKey.take(8)}***")
        
        // Initialize managers
        consentManager = ConsentManager(applicationContext, this.config)
        connectionManager = ConnectionManager(applicationContext, sdkKey, this.config)
        bandwidthTracker = BandwidthTracker(applicationContext)
        
        _isInitialized.set(true)
        _status.set(SDKStatus.INITIALIZED)
    }
    
    /**
     * Start the SDK (non-blocking)
     */
    @JvmStatic
    fun start() {
        start(null, null)
    }
    
    /**
     * Start the SDK with callbacks
     */
    @JvmStatic
    fun start(onSuccess: Runnable?, onError: java.util.function.Consumer<String>?) {
        checkInitialized()
        
        if (_isRunning.get()) {
            onSuccess?.run()
            return
        }
        
        executor.execute {
            try {
                _status.set(SDKStatus.STARTING)
                
                // Check consent
                if (consentManager?.hasConsent() != true) {
                    _status.set(SDKStatus.CONSENT_REQUIRED)
                    mainHandler.post { onError?.accept("User consent required") }
                    return@execute
                }
                
                // Check network conditions
                val networkInfo = DeviceInfo.getNetworkInfo(applicationContext)
                if (config.wifiOnly && networkInfo["connection_type"] != "wifi") {
                    _status.set(SDKStatus.WAITING_WIFI)
                    mainHandler.post { onError?.accept("WiFi required but not connected") }
                    return@execute
                }
                
                // Start connection manager
                connectionManager?.start()
                bandwidthTracker?.start()
                
                // Start foreground service
                try {
                    val serviceIntent = Intent(applicationContext, IPLoopProxyService::class.java)
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                        applicationContext.startForegroundService(serviceIntent)
                    } else {
                        applicationContext.startService(serviceIntent)
                    }
                } catch (e: Exception) {
                    IPLoopLogger.w("IPLoopSDK", "Could not start foreground service: ${e.message}")
                }
                
                _isRunning.set(true)
                _status.set(SDKStatus.RUNNING)
                
                IPLoopLogger.i("IPLoopSDK", "Started successfully")
                mainHandler.post { onSuccess?.run() }
                
            } catch (e: Exception) {
                IPLoopLogger.e("IPLoopSDK", "Failed to start", e)
                _status.set(SDKStatus.ERROR)
                mainHandler.post { onError?.accept(e.message ?: "Unknown error") }
            }
        }
    }
    
    /**
     * Stop the SDK (non-blocking)
     */
    @JvmStatic
    fun stop() {
        stop(null)
    }
    
    /**
     * Stop the SDK with callback
     */
    @JvmStatic
    fun stop(onComplete: Runnable?) {
        if (!_isInitialized.get()) return
        if (!_isRunning.get()) {
            onComplete?.run()
            return
        }
        
        executor.execute {
            try {
                _status.set(SDKStatus.STOPPING)
                
                connectionManager?.stop()
                bandwidthTracker?.stop()
                
                // Stop foreground service
                try {
                    val serviceIntent = Intent(applicationContext, IPLoopProxyService::class.java)
                    applicationContext.stopService(serviceIntent)
                } catch (e: Exception) {
                    IPLoopLogger.w("IPLoopSDK", "Could not stop service: ${e.message}")
                }
                
                _isRunning.set(false)
                _status.set(SDKStatus.STOPPED)
                
                IPLoopLogger.i("IPLoopSDK", "Stopped successfully")
                mainHandler.post { onComplete?.run() }
                
            } catch (e: Exception) {
                IPLoopLogger.e("IPLoopSDK", "Error during stop", e)
                mainHandler.post { onComplete?.run() }
            }
        }
    }
    
    /**
     * Check if SDK is running
     */
    @JvmStatic
    fun isRunning(): Boolean = _isRunning.get()
    
    /**
     * Get current status
     */
    @JvmStatic
    fun getStatus(): SDKStatus = _status.get()
    
    /**
     * Set user consent
     */
    @JvmStatic
    fun setConsentGiven(given: Boolean) {
        checkInitialized()
        consentManager?.setConsent(given)
    }
    
    /**
     * Check if user has given consent
     */
    @JvmStatic
    fun hasConsent(): Boolean {
        return consentManager?.hasConsent() ?: false
    }
    
    /**
     * Show consent dialog
     */
    @JvmStatic
    fun showConsentDialog(context: Context) {
        checkInitialized()
        consentManager?.showConsentDialog(context)
    }
    
    private fun checkInitialized() {
        if (!_isInitialized.get()) {
            throw IllegalStateException("IPLoopSDK not initialized. Call init() first.")
        }
    }
}

/**
 * SDK Status enum
 */
enum class SDKStatus {
    STOPPED,
    INITIALIZED,
    STARTING,
    RUNNING,
    STOPPING,
    CONSENT_REQUIRED,
    WAITING_WIFI,
    ERROR
}

/**
 * Bandwidth usage data
 */
data class BandwidthUsage(
    val uploadedMB: Long,
    val downloadedMB: Long,
    val totalMB: Long,
    val sessionsCount: Int,
    val lastActivity: Long
)
