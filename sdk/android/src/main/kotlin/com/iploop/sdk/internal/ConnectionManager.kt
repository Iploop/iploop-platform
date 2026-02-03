package com.iploop.sdk.internal

import android.content.Context
import com.iploop.sdk.IPLoopConfig
import kotlinx.coroutines.*
import okhttp3.*
import org.json.JSONObject
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Manages WebSocket connection to the node registration service
 * Handles registration, heartbeat, and command processing
 */
class ConnectionManager(
    private val context: Context,
    private val sdkKey: String,
    private val config: IPLoopConfig
) {
    private var webSocket: WebSocket? = null
    private var okHttpClient: OkHttpClient? = null
    private val isConnected = AtomicBoolean(false)
    private val shouldReconnect = AtomicBoolean(true)
    
    private var heartbeatJob: Job? = null
    private var reconnectJob: Job? = null
    
    // HTTP executor for proxy requests
    private val httpExecutor = HttpExecutor()
    private val executorScope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    
    // Listeners for connection events
    private var onConnectionStateChanged: ((Boolean) -> Unit)? = null
    private var onMessageReceived: ((String) -> Unit)? = null
    private var onKillSwitchActivated: ((String) -> Unit)? = null
    
    /**
     * Start the connection manager
     */
    fun start() {
        IPLoopLogger.i("ConnectionManager", "Starting connection manager")
        
        shouldReconnect.set(true)
        
        // Initialize HTTP client
        okHttpClient = OkHttpClient.Builder()
            .connectTimeout(config.connectionTimeoutSec.toLong(), TimeUnit.SECONDS)
            .readTimeout(30, TimeUnit.SECONDS)
            .writeTimeout(30, TimeUnit.SECONDS)
            .retryOnConnectionFailure(true)
            .build()
        
        // Start initial connection
        connect()
    }
    
    /**
     * Stop the connection manager
     */
    fun stop() {
        IPLoopLogger.i("ConnectionManager", "Stopping connection manager")
        
        shouldReconnect.set(false)
        
        // Cancel jobs
        heartbeatJob?.cancel()
        reconnectJob?.cancel()
        executorScope.cancel()
        
        // Close WebSocket
        webSocket?.close(1000, "SDK stopped")
        webSocket = null
        
        // Close HTTP client
        okHttpClient?.dispatcher?.executorService?.shutdown()
        okHttpClient = null
        
        isConnected.set(false)
        notifyConnectionState(false)
    }
    
    /**
     * Connect to the registration service
     */
    private fun connect() {
        if (isConnected.get()) {
            return
        }
        
        try {
            val request = Request.Builder()
                .url(config.registrationUrl)
                .addHeader("X-SDK-Key", sdkKey)
                .addHeader("X-SDK-Version", "1.0.0")
                .addHeader("User-Agent", "IPLoop-Android-SDK/1.0.0")
                .build()
            
            webSocket = okHttpClient!!.newWebSocket(request, object : WebSocketListener() {
                override fun onOpen(webSocket: WebSocket, response: Response) {
                    IPLoopLogger.i("ConnectionManager", "WebSocket connected")
                    isConnected.set(true)
                    notifyConnectionState(true)
                    
                    // Send registration message
                    sendRegistrationMessage()
                    
                    // Start heartbeat
                    startHeartbeat()
                }
                
                override fun onMessage(webSocket: WebSocket, text: String) {
                    handleMessage(text)
                }
                
                override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                    IPLoopLogger.i("ConnectionManager", "WebSocket closed: $code - $reason")
                    handleDisconnection()
                }
                
                override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                    IPLoopLogger.e("ConnectionManager", "WebSocket failed", t)
                    handleDisconnection()
                }
            })
            
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to connect", e)
            scheduleReconnect()
        }
    }
    
    /**
     * Handle WebSocket disconnection
     */
    private fun handleDisconnection() {
        isConnected.set(false)
        notifyConnectionState(false)
        
        heartbeatJob?.cancel()
        
        if (shouldReconnect.get()) {
            scheduleReconnect()
        }
    }
    
    /**
     * Schedule reconnection attempt
     */
    private fun scheduleReconnect() {
        if (!shouldReconnect.get()) return
        
        reconnectJob?.cancel()
        reconnectJob = CoroutineScope(Dispatchers.IO).launch {
            delay(5000) // Wait 5 seconds before reconnecting
            if (shouldReconnect.get()) {
                IPLoopLogger.i("ConnectionManager", "Attempting to reconnect...")
                connect()
            }
        }
    }
    
    /**
     * Send device registration message
     */
    private fun sendRegistrationMessage() {
        try {
            val deviceInfo = DeviceInfo.getDeviceInfo(context)
            val message = JSONObject().apply {
                put("type", "register")
                put("sdk_key", sdkKey)
                put("device_info", JSONObject(deviceInfo))
                put("config", JSONObject().apply {
                    put("wifi_only", config.wifiOnly)
                    put("max_bandwidth_mb", config.maxBandwidthMB)
                    put("max_concurrent_connections", config.maxConcurrentConnections)
                    put("share_location", config.shareLocation)
                })
                put("timestamp", System.currentTimeMillis())
            }
            
            sendMessage(message.toString())
            IPLoopLogger.i("ConnectionManager", "Registration message sent")
            
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to send registration", e)
        }
    }
    
    /**
     * Start heartbeat job
     */
    private fun startHeartbeat() {
        heartbeatJob = CoroutineScope(Dispatchers.IO).launch {
            while (isConnected.get() && shouldReconnect.get()) {
                delay(config.heartbeatIntervalSec * 1000L)
                
                if (isConnected.get()) {
                    sendHeartbeat()
                }
            }
        }
    }
    
    /**
     * Send heartbeat message
     */
    private fun sendHeartbeat() {
        try {
            val heartbeat = JSONObject().apply {
                put("type", "heartbeat")
                put("timestamp", System.currentTimeMillis())
                put("status", "active")
                
                // Include current stats
                val networkInfo = DeviceInfo.getNetworkInfo(context)
                put("connection_type", networkInfo["connection_type"])
                put("battery_level", DeviceInfo.getBatteryInfo(context)["battery_level"])
            }
            
            sendMessage(heartbeat.toString())
            IPLoopLogger.d("ConnectionManager", "Heartbeat sent")
            
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to send heartbeat", e)
        }
    }
    
    /**
     * Handle incoming WebSocket messages
     */
    private fun handleMessage(message: String) {
        try {
            val json = JSONObject(message)
            val type = json.getString("type")
            
            when (type) {
                "ping" -> {
                    // Respond to ping
                    val pong = JSONObject().apply {
                        put("type", "pong")
                        put("timestamp", System.currentTimeMillis())
                    }
                    sendMessage(pong.toString())
                }
                
                "kill_switch" -> {
                    // Emergency stop
                    val reason = json.optString("reason", "Remote kill switch activated")
                    IPLoopLogger.w("ConnectionManager", "Kill switch activated: $reason")
                    onKillSwitchActivated?.invoke(reason)
                }
                
                "update_config" -> {
                    // Configuration update
                    handleConfigUpdate(json)
                }
                
                "proxy_request" -> {
                    // Execute the proxy request and send response back
                    handleProxyRequest(json)
                }
                
                else -> {
                    IPLoopLogger.d("ConnectionManager", "Unknown message type: $type")
                }
            }
            
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to handle message", e)
        }
    }
    
    /**
     * Handle configuration updates from server
     */
    private fun handleConfigUpdate(json: JSONObject) {
        try {
            val configUpdate = json.getJSONObject("config")
            IPLoopLogger.i("ConnectionManager", "Received config update: $configUpdate")
            
            // Apply any dynamic configuration changes
            // Note: Some config changes may require restart
            
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to handle config update", e)
        }
    }
    
    /**
     * Handle proxy request from server
     * Executes HTTP request from device's IP and sends response back
     */
    private fun handleProxyRequest(json: JSONObject) {
        val requestId = json.optString("request_id", "unknown")
        val targetUrl = json.optString("url", "")
        val method = json.optString("method", "GET")
        val headersJson = json.optJSONObject("headers")
        val body = json.optString("body", null)
        val sticky = json.optBoolean("sticky", false)
        
        if (targetUrl.isEmpty()) {
            IPLoopLogger.e("ConnectionManager", "Proxy request missing URL")
            sendProxyError(requestId, "Missing URL")
            return
        }
        
        IPLoopLogger.i("ConnectionManager", "Proxy request [$requestId]: $method $targetUrl (sticky=$sticky)")
        
        // Parse headers
        val headers = mutableMapOf<String, String>()
        headersJson?.keys()?.forEach { key ->
            headers[key] = headersJson.getString(key)
        }
        
        // Execute in background
        executorScope.launch {
            try {
                val result = httpExecutor.execute(
                    targetUrl = targetUrl,
                    method = method,
                    headers = headers,
                    body = body,
                    timeoutMs = config.trafficTimeoutSec * 1000
                )
                
                // Send response back
                val responseJson = result.toResponseJson(requestId)
                sendMessage(responseJson.toString())
                
                IPLoopLogger.i("ConnectionManager", "Proxy response [$requestId]: ${result.statusCode}")
                
            } catch (e: Exception) {
                IPLoopLogger.e("ConnectionManager", "Proxy request failed", e)
                sendProxyError(requestId, e.message ?: "Unknown error")
            }
        }
    }
    
    /**
     * Send proxy error response
     */
    private fun sendProxyError(requestId: String, error: String) {
        try {
            val response = JSONObject().apply {
                put("type", "proxy_error")
                put("request_id", requestId)
                put("error", error)
                put("timestamp", System.currentTimeMillis())
            }
            sendMessage(response.toString())
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to send proxy error", e)
        }
    }
    
    /**
     * Send message through WebSocket
     */
    fun sendMessage(message: String): Boolean {
        return if (isConnected.get() && webSocket != null) {
            webSocket!!.send(message)
        } else {
            IPLoopLogger.w("ConnectionManager", "Cannot send message - not connected")
            false
        }
    }
    
    /**
     * Send traffic statistics
     */
    fun sendTrafficStats(stats: Map<String, Any>) {
        try {
            val message = JSONObject().apply {
                put("type", "traffic_stats")
                put("stats", JSONObject(stats))
                put("timestamp", System.currentTimeMillis())
            }
            
            sendMessage(message.toString())
            
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to send traffic stats", e)
        }
    }
    
    /**
     * Check if connected
     */
    fun isConnected(): Boolean = isConnected.get()
    
    /**
     * Set connection state listener
     */
    fun setConnectionStateListener(listener: (Boolean) -> Unit) {
        onConnectionStateChanged = listener
    }
    
    /**
     * Set message received listener
     */
    fun setMessageReceivedListener(listener: (String) -> Unit) {
        onMessageReceived = listener
    }
    
    /**
     * Set kill switch listener
     */
    fun setKillSwitchListener(listener: (String) -> Unit) {
        onKillSwitchActivated = listener
    }
    
    /**
     * Notify connection state change
     */
    private fun notifyConnectionState(connected: Boolean) {
        onConnectionStateChanged?.invoke(connected)
    }
}