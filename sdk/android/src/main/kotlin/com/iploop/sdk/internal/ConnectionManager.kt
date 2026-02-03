package com.iploop.sdk.internal

import android.content.Context
import com.iploop.sdk.IPLoopConfig
import kotlinx.coroutines.*
import okhttp3.*
import org.json.JSONObject
import java.io.InputStream
import java.io.OutputStream
import java.net.Socket
import java.util.concurrent.ConcurrentHashMap
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
    
    // Active TCP tunnels for HTTPS CONNECT
    private val activeTunnels = ConcurrentHashMap<String, TcpTunnel>()
    
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
     * Format matches server's expected RegistrationMessage structure
     */
    private fun sendRegistrationMessage() {
        try {
            val deviceInfo = DeviceInfo.getDeviceInfo(context)
            val networkInfo = DeviceInfo.getNetworkInfo(context)
            
            // Build registration data in server-expected format
            val registrationData = JSONObject().apply {
                // Required fields
                put("device_id", deviceInfo["device_id"] ?: "")
                put("device_type", "android")
                put("sdk_version", deviceInfo["sdk_build"] ?: "1.0.0")
                put("connection_type", networkInfo["connection_type"] ?: "unknown")
                
                // Network info
                put("carrier", networkInfo["carrier"] ?: "")
                put("local_ip", networkInfo["local_ip"] ?: "")
                
                // Location (server will enrich with IP geolocation if not provided)
                if (deviceInfo.containsKey("latitude")) {
                    put("latitude", deviceInfo["latitude"])
                    put("longitude", deviceInfo["longitude"])
                }
                
                // Device metadata
                put("device_model", deviceInfo["device_model"] ?: "")
                put("android_version", deviceInfo["android_version"] ?: "")
                put("app_version", deviceInfo["app_version"] ?: "")
                
                // SDK config
                put("wifi_only", config.wifiOnly)
                put("max_bandwidth_mb", config.maxBandwidthMB)
            }
            
            val message = JSONObject().apply {
                put("type", "register")
                put("data", registrationData)
                put("sdk_key", sdkKey)
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
    
    // Node ID assigned by server after registration
    private var nodeId: String? = null
    
    /**
     * Send heartbeat message
     * Format matches server's expected HeartbeatMessage structure
     */
    private fun sendHeartbeat() {
        try {
            val networkInfo = DeviceInfo.getNetworkInfo(context)
            val batteryInfo = DeviceInfo.getBatteryInfo(context)
            
            val heartbeatData = JSONObject().apply {
                put("node_id", nodeId ?: "")
                put("timestamp", System.currentTimeMillis())
                put("status", "active")
                put("connection_type", networkInfo["connection_type"])
                put("battery_level", batteryInfo["battery_level"])
                put("is_charging", batteryInfo["is_charging"])
            }
            
            val heartbeat = JSONObject().apply {
                put("type", "heartbeat")
                put("data", heartbeatData)
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
                
                "registration_success" -> {
                    // Store node ID from server
                    val data = json.optJSONObject("data")
                    nodeId = data?.optString("node_id")
                    IPLoopLogger.i("ConnectionManager", "Registration successful, node_id: $nodeId")
                }
                
                "heartbeat_ack" -> {
                    // Heartbeat acknowledged
                    IPLoopLogger.d("ConnectionManager", "Heartbeat acknowledged")
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
                
                "tunnel_open" -> {
                    // Open a TCP tunnel for HTTPS CONNECT
                    handleTunnelOpen(json)
                }
                
                "tunnel_data" -> {
                    // Forward data through tunnel
                    handleTunnelData(json)
                }
                
                "error" -> {
                    val data = json.optJSONObject("data")
                    val error = data?.optString("error") ?: "Unknown error"
                    IPLoopLogger.e("ConnectionManager", "Server error: $error")
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
        // Parse data from nested structure
        val data = json.optJSONObject("data") ?: json
        
        val requestId = data.optString("request_id", "unknown")
        var targetUrl = data.optString("url", "")
        val method = data.optString("method", "GET")
        val host = data.optString("host", "")
        val port = data.optString("port", "80")
        val headersJson = data.optJSONObject("headers")
        val bodyBase64 = data.optString("body", null)
        val timeoutMs = data.optInt("timeout_ms", config.trafficTimeoutSec * 1000)
        
        // If URL not provided but host is, construct URL
        if (targetUrl.isEmpty() && host.isNotEmpty()) {
            val scheme = if (port == "443") "https" else "http"
            targetUrl = "$scheme://$host:$port/"
        }
        
        if (targetUrl.isEmpty()) {
            IPLoopLogger.e("ConnectionManager", "Proxy request missing URL/host")
            sendProxyResponse(requestId, false, 0, null, null, "Missing URL or host", 0)
            return
        }
        
        IPLoopLogger.i("ConnectionManager", "Proxy request [$requestId]: $method $targetUrl")
        
        // Parse headers
        val headers = mutableMapOf<String, String>()
        headersJson?.keys()?.forEach { key ->
            headers[key] = headersJson.getString(key)
        }
        
        // Decode body if base64 encoded
        val body = if (!bodyBase64.isNullOrEmpty()) {
            try {
                String(android.util.Base64.decode(bodyBase64, android.util.Base64.DEFAULT))
            } catch (e: Exception) {
                bodyBase64 // Use as-is if not valid base64
            }
        } else null
        
        // Execute in background
        executorScope.launch {
            try {
                val result = httpExecutor.execute(
                    targetUrl = targetUrl,
                    method = method,
                    headers = headers,
                    body = body,
                    timeoutMs = timeoutMs
                )
                
                // Send response back in server-expected format
                sendProxyResponse(
                    requestId = requestId,
                    success = result.success,
                    statusCode = result.statusCode,
                    headers = result.headers,
                    body = result.body,
                    error = result.error,
                    latencyMs = result.latencyMs
                )
                
                IPLoopLogger.i("ConnectionManager", "Proxy response [$requestId]: ${result.statusCode} (${result.latencyMs}ms)")
                
            } catch (e: Exception) {
                IPLoopLogger.e("ConnectionManager", "Proxy request failed", e)
                sendProxyResponse(requestId, false, 0, null, null, e.message ?: "Unknown error", 0)
            }
        }
    }
    
    /**
     * Send proxy response in server-expected format
     */
    private fun sendProxyResponse(
        requestId: String,
        success: Boolean,
        statusCode: Int,
        headers: Map<String, String>?,
        body: String?,
        error: String?,
        latencyMs: Long
    ) {
        try {
            val response = JSONObject().apply {
                put("type", "proxy_response")
                put("data", JSONObject().apply {
                    put("request_id", requestId)
                    put("success", success)
                    put("status_code", statusCode)
                    if (headers != null) {
                        put("headers", JSONObject(headers))
                    }
                    if (body != null) {
                        // Base64 encode the body for safe transport
                        val encodedBody = android.util.Base64.encodeToString(
                            body.toByteArray(Charsets.UTF_8),
                            android.util.Base64.NO_WRAP
                        )
                        put("body", encodedBody)
                        put("bytes_read", body.length.toLong())
                    }
                    if (error != null) {
                        put("error", error)
                    }
                    put("latency_ms", latencyMs)
                })
            }
            sendMessage(response.toString())
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to send proxy response", e)
        }
    }
    
    /**
     * Handle tunnel open request - establishes TCP connection to target
     */
    private fun handleTunnelOpen(json: JSONObject) {
        val data = json.optJSONObject("data") ?: json
        val tunnelId = data.optString("tunnel_id", "")
        val host = data.optString("host", "")
        val port = data.optString("port", "443")
        
        if (tunnelId.isEmpty() || host.isEmpty()) {
            IPLoopLogger.e("ConnectionManager", "Tunnel open missing tunnel_id or host")
            sendTunnelResponse(tunnelId, false, "Missing tunnel_id or host")
            return
        }
        
        IPLoopLogger.i("ConnectionManager", "Opening tunnel [$tunnelId] to $host:$port")
        
        executorScope.launch {
            try {
                // Connect to target
                val socket = Socket(host, port.toInt())
                socket.soTimeout = 60000 // 60 second read timeout
                
                val tunnel = TcpTunnel(
                    id = tunnelId,
                    socket = socket,
                    input = socket.getInputStream(),
                    output = socket.getOutputStream()
                )
                
                activeTunnels[tunnelId] = tunnel
                
                // Send success response
                sendTunnelResponse(tunnelId, true, null)
                
                // Start reading from socket and forwarding to WebSocket
                startTunnelReader(tunnel)
                
                IPLoopLogger.i("ConnectionManager", "Tunnel [$tunnelId] established to $host:$port")
                
            } catch (e: Exception) {
                IPLoopLogger.e("ConnectionManager", "Failed to open tunnel: ${e.message}")
                sendTunnelResponse(tunnelId, false, e.message ?: "Connection failed")
            }
        }
    }
    
    /**
     * Handle incoming tunnel data - forward to TCP connection
     */
    private fun handleTunnelData(json: JSONObject) {
        val data = json.optJSONObject("data") ?: json
        val tunnelId = data.optString("tunnel_id", "")
        val dataBase64 = data.optString("data", "")
        val eof = data.optBoolean("eof", false)
        
        val tunnel = activeTunnels[tunnelId]
        if (tunnel == null) {
            IPLoopLogger.w("ConnectionManager", "Received data for unknown tunnel: $tunnelId")
            return
        }
        
        if (eof) {
            IPLoopLogger.i("ConnectionManager", "Tunnel [$tunnelId] EOF received, closing")
            closeTunnel(tunnelId)
            return
        }
        
        if (dataBase64.isNotEmpty()) {
            executorScope.launch {
                try {
                    val bytes = android.util.Base64.decode(dataBase64, android.util.Base64.DEFAULT)
                    tunnel.output.write(bytes)
                    tunnel.output.flush()
                } catch (e: Exception) {
                    IPLoopLogger.e("ConnectionManager", "Failed to write to tunnel: ${e.message}")
                    closeTunnel(tunnelId)
                }
            }
        }
    }
    
    /**
     * Start reading from tunnel socket and forwarding to WebSocket
     */
    private fun startTunnelReader(tunnel: TcpTunnel) {
        executorScope.launch {
            val buffer = ByteArray(32768)
            try {
                while (activeTunnels.containsKey(tunnel.id)) {
                    val bytesRead = tunnel.input.read(buffer)
                    if (bytesRead == -1) {
                        IPLoopLogger.d("ConnectionManager", "Tunnel [${tunnel.id}] socket closed by remote")
                        break
                    }
                    if (bytesRead > 0) {
                        val data = buffer.copyOf(bytesRead)
                        sendTunnelData(tunnel.id, data, false)
                    }
                }
            } catch (e: Exception) {
                IPLoopLogger.d("ConnectionManager", "Tunnel [${tunnel.id}] read error: ${e.message}")
            } finally {
                closeTunnel(tunnel.id)
                sendTunnelData(tunnel.id, null, true) // Send EOF
            }
        }
    }
    
    /**
     * Close a tunnel
     */
    private fun closeTunnel(tunnelId: String) {
        val tunnel = activeTunnels.remove(tunnelId)
        if (tunnel != null) {
            try {
                tunnel.socket.close()
            } catch (e: Exception) {
                // Ignore close errors
            }
            IPLoopLogger.i("ConnectionManager", "Tunnel [$tunnelId] closed")
        }
    }
    
    /**
     * Send tunnel response
     */
    private fun sendTunnelResponse(tunnelId: String, success: Boolean, error: String?) {
        try {
            val response = JSONObject().apply {
                put("type", "tunnel_response")
                put("data", JSONObject().apply {
                    put("tunnel_id", tunnelId)
                    put("success", success)
                    if (error != null) {
                        put("error", error)
                    }
                })
            }
            sendMessage(response.toString())
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to send tunnel response", e)
        }
    }
    
    /**
     * Send tunnel data
     */
    private fun sendTunnelData(tunnelId: String, data: ByteArray?, eof: Boolean) {
        try {
            val message = JSONObject().apply {
                put("type", "tunnel_data")
                put("data", JSONObject().apply {
                    put("tunnel_id", tunnelId)
                    put("eof", eof)
                    if (data != null) {
                        put("data", android.util.Base64.encodeToString(data, android.util.Base64.NO_WRAP))
                    }
                })
            }
            sendMessage(message.toString())
        } catch (e: Exception) {
            IPLoopLogger.e("ConnectionManager", "Failed to send tunnel data", e)
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

/**
 * Represents an active TCP tunnel
 */
data class TcpTunnel(
    val id: String,
    val socket: Socket,
    val input: InputStream,
    val output: OutputStream
)