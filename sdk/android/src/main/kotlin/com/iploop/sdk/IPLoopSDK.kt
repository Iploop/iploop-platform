package com.iploop.sdk

import android.content.Context
import android.content.Intent
import android.os.Build
import com.iploop.sdk.internal.*
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.coroutines.CoroutineContext

/**
 * IPLoop Android SDK - Main entry point
 * 
 * Converts Android devices into proxy nodes for the IPLoop platform.
 * Handles consent, connectivity, traffic relay, and compliance.
 */
object IPLoopSDK : CoroutineScope {
    
    private lateinit var applicationContext: Context
    private lateinit var sdkKey: String
    private lateinit var config: IPLoopConfig
    
    private var connectionManager: ConnectionManager? = null
    private var tunnelManager: TunnelManager? = null
    private var trafficRelay: TrafficRelay? = null
    private var bandwidthTracker: BandwidthTracker? = null
    private var consentManager: ConsentManager? = null
    
    private val job = SupervisorJob()
    override val coroutineContext: CoroutineContext = Dispatchers.Main + job
    
    private val _status = MutableStateFlow(SDKStatus.STOPPED)
    val status: StateFlow<SDKStatus> = _status.asStateFlow()
    
    private val _isRunning = AtomicBoolean(false)
    
    /**
     * Initialize the SDK with default config
     * Call this once in your Application's onCreate()
     */
    @JvmStatic
    fun init(context: Context, sdkKey: String) {
        init(context, sdkKey, null)
    }
    
    /**
     * Initialize the SDK
     * Call this once in your Application's onCreate()
     * @param config Optional - if null, uses IPLoopConfig.createDefault()
     */
    @JvmStatic
    @JvmOverloads
    fun init(context: Context, sdkKey: String, config: IPLoopConfig?) {
        if (::applicationContext.isInitialized) {
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
        tunnelManager = TunnelManager(applicationContext, this.config)
        trafficRelay = TrafficRelay(applicationContext, this.config)
        bandwidthTracker = BandwidthTracker(applicationContext)
        
        _status.value = SDKStatus.INITIALIZED
    }
    
    /**
     * Start the proxy service
     * Requires user consent and proper network conditions
     */
    @JvmStatic
    suspend fun start(): Result<Unit> = withContext(Dispatchers.IO) {
        checkInitialized()
        
        if (_isRunning.get()) {
            return@withContext Result.success(Unit)
        }
        
        try {
            _status.value = SDKStatus.STARTING
            
            // Check consent
            if (!consentManager!!.hasConsent()) {
                _status.value = SDKStatus.CONSENT_REQUIRED
                return@withContext Result.failure(Exception("User consent required"))
            }
            
            // Check network conditions
            val networkInfo = DeviceInfo.getNetworkInfo(applicationContext)
            if (config.wifiOnly && networkInfo["connection_type"] != "wifi") {
                _status.value = SDKStatus.WAITING_WIFI
                return@withContext Result.failure(Exception("WiFi required but not connected"))
            }
            
            // Start services
            connectionManager!!.start()
            tunnelManager!!.start()
            trafficRelay!!.start()
            bandwidthTracker!!.start()
            
            // Start foreground service
            val serviceIntent = Intent(applicationContext, IPLoopProxyService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                applicationContext.startForegroundService(serviceIntent)
            } else {
                applicationContext.startService(serviceIntent)
            }
            
            _isRunning.set(true)
            _status.value = SDKStatus.RUNNING
            
            IPLoopLogger.i("IPLoopSDK", "Started successfully")
            Result.success(Unit)
            
        } catch (e: Exception) {
            IPLoopLogger.e("IPLoopSDK", "Failed to start", e)
            _status.value = SDKStatus.ERROR
            Result.failure(e)
        }
    }
    
    /**
     * Stop the proxy service
     */
    @JvmStatic
    suspend fun stop() = withContext(Dispatchers.IO) {
        checkInitialized()
        
        if (!_isRunning.get()) return@withContext
        
        try {
            _status.value = SDKStatus.STOPPING
            
            // Stop services
            trafficRelay?.stop()
            tunnelManager?.stop()
            connectionManager?.stop()
            bandwidthTracker?.stop()
            
            // Stop foreground service
            val serviceIntent = Intent(applicationContext, IPLoopProxyService::class.java)
            applicationContext.stopService(serviceIntent)
            
            _isRunning.set(false)
            _status.value = SDKStatus.STOPPED
            
            IPLoopLogger.i("IPLoopSDK", "Stopped successfully")
            
        } catch (e: Exception) {
            IPLoopLogger.e("IPLoopSDK", "Error during stop", e)
        }
    }
    
    /**
     * Check if the SDK is currently running
     */
    @JvmStatic
    fun isRunning(): Boolean = _isRunning.get()
    
    /**
     * Get current SDK status
     */
    @JvmStatic
    fun getStatus(): SDKStatus = _status.value
    
    /**
     * Set user consent for proxy operation
     */
    @JvmStatic
    fun setConsentGiven(consent: Boolean) {
        checkInitialized()
        consentManager!!.setConsent(consent)
        
        if (!consent && isRunning()) {
            launch { stop() }
        }
    }
    
    /**
     * Check if user has given consent
     */
    @JvmStatic
    fun hasConsent(): Boolean {
        checkInitialized()
        return consentManager!!.hasConsent()
    }
    
    /**
     * Get bandwidth usage statistics
     */
    @JvmStatic
    fun getBandwidthUsage(): BandwidthUsage? {
        return bandwidthTracker?.getCurrentUsage()
    }
    
    /**
     * Get current device/network information
     */
    @JvmStatic
    fun getDeviceInfo(): Map<String, Any> {
        checkInitialized()
        return DeviceInfo.getDeviceInfo(applicationContext)
    }
    
    /**
     * Force kill switch - remotely disable the SDK
     */
    @JvmStatic
    internal fun emergencyStop(reason: String) {
        IPLoopLogger.w("IPLoopSDK", "Emergency stop triggered: $reason")
        launch { stop() }
        
        // Mark as disabled
        consentManager?.setConsent(false)
        _status.value = SDKStatus.DISABLED
    }
    
    /**
     * Show consent dialog to user
     */
    @JvmStatic
    fun showConsentDialog(context: Context) {
        checkInitialized()
        consentManager!!.showConsentDialog(context)
    }
    
    /**
     * Check if SDK is initialized
     */
    @JvmStatic
    fun isInitialized(): Boolean = ::applicationContext.isInitialized
    
    private fun checkInitialized() {
        if (!::applicationContext.isInitialized) {
            throw IllegalStateException("IPLoopSDK not initialized. Call init() first.")
        }
    }
}

/**
 * SDK Status enumeration
 */
enum class SDKStatus {
    STOPPED,
    INITIALIZED, 
    STARTING,
    RUNNING,
    STOPPING,
    CONSENT_REQUIRED,
    WAITING_WIFI,
    ERROR,
    DISABLED
}

/**
 * Bandwidth usage data class
 */
data class BandwidthUsage(
    val uploadedMB: Long,
    val downloadedMB: Long,
    val totalMB: Long,
    val sessionsCount: Int,
    val lastActivity: Long
)