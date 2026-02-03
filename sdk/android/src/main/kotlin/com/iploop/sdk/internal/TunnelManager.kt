package com.iploop.sdk.internal

import android.content.Context
import com.iploop.sdk.IPLoopConfig
import kotlinx.coroutines.*
import java.io.*
import java.net.*
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

/**
 * Manages secure tunnels for proxy traffic
 * Handles encryption, routing, and connection management
 */
class TunnelManager(
    private val context: Context,
    private val config: IPLoopConfig
) {
    private val isRunning = AtomicBoolean(false)
    private val tunnelScope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    
    // Active tunnels (session_id -> TunnelSession)
    private val activeTunnels = ConcurrentHashMap<String, TunnelSession>()
    
    // Statistics
    private val bytesTransferred = AtomicLong(0)
    private val connectionsCount = AtomicLong(0)
    
    // Listeners
    private var onTunnelCreated: ((String) -> Unit)? = null
    private var onTunnelClosed: ((String) -> Unit)? = null
    private var onBandwidthUpdate: ((Long) -> Unit)? = null
    
    /**
     * Start the tunnel manager
     */
    fun start() {
        if (isRunning.get()) return
        
        IPLoopLogger.i("TunnelManager", "Starting tunnel manager")
        isRunning.set(true)
        
        // Start cleanup job for idle tunnels
        startCleanupJob()
    }
    
    /**
     * Stop the tunnel manager
     */
    fun stop() {
        if (!isRunning.get()) return
        
        IPLoopLogger.i("TunnelManager", "Stopping tunnel manager")
        isRunning.set(false)
        
        // Close all active tunnels
        activeTunnels.values.forEach { tunnel ->
            tunnel.close()
        }
        activeTunnels.clear()
        
        // Cancel coroutines
        tunnelScope.cancel()
    }
    
    /**
     * Create a new tunnel for a session
     */
    fun createTunnel(sessionId: String, targetHost: String, targetPort: Int): Result<TunnelSession> {
        if (!isRunning.get()) {
            return Result.failure(IllegalStateException("TunnelManager not running"))
        }
        
        if (activeTunnels.size >= config.maxConcurrentConnections) {
            return Result.failure(IllegalStateException("Maximum concurrent connections reached"))
        }
        
        return try {
            val tunnel = TunnelSession(sessionId, targetHost, targetPort, this)
            activeTunnels[sessionId] = tunnel
            connectionsCount.incrementAndGet()
            
            IPLoopLogger.d("TunnelManager", "Created tunnel: $sessionId -> $targetHost:$targetPort")
            onTunnelCreated?.invoke(sessionId)
            
            Result.success(tunnel)
        } catch (e: Exception) {
            IPLoopLogger.e("TunnelManager", "Failed to create tunnel", e)
            Result.failure(e)
        }
    }
    
    /**
     * Close a tunnel
     */
    fun closeTunnel(sessionId: String) {
        activeTunnels.remove(sessionId)?.let { tunnel ->
            tunnel.close()
            IPLoopLogger.d("TunnelManager", "Closed tunnel: $sessionId")
            onTunnelClosed?.invoke(sessionId)
        }
    }
    
    /**
     * Get tunnel by session ID
     */
    fun getTunnel(sessionId: String): TunnelSession? {
        return activeTunnels[sessionId]
    }
    
    /**
     * Get active tunnel count
     */
    fun getActiveTunnelCount(): Int = activeTunnels.size
    
    /**
     * Get total bytes transferred
     */
    fun getBytesTransferred(): Long = bytesTransferred.get()
    
    /**
     * Get total connections count
     */
    fun getConnectionsCount(): Long = connectionsCount.get()
    
    /**
     * Add bytes to the transfer counter
     */
    internal fun addBytesTransferred(bytes: Long) {
        val total = bytesTransferred.addAndGet(bytes)
        onBandwidthUpdate?.invoke(total)
    }
    
    /**
     * Start cleanup job for idle tunnels
     */
    private fun startCleanupJob() {
        tunnelScope.launch {
            while (isRunning.get()) {
                delay(60000) // Check every minute
                
                val now = System.currentTimeMillis()
                val idleTunnels = activeTunnels.values.filter { tunnel ->
                    now - tunnel.lastActivity > 300000 // 5 minutes idle
                }
                
                idleTunnels.forEach { tunnel ->
                    IPLoopLogger.d("TunnelManager", "Closing idle tunnel: ${tunnel.sessionId}")
                    closeTunnel(tunnel.sessionId)
                }
            }
        }
    }
    
    /**
     * Set tunnel event listeners
     */
    fun setTunnelCreatedListener(listener: (String) -> Unit) {
        onTunnelCreated = listener
    }
    
    fun setTunnelClosedListener(listener: (String) -> Unit) {
        onTunnelClosed = listener
    }
    
    fun setBandwidthUpdateListener(listener: (Long) -> Unit) {
        onBandwidthUpdate = listener
    }
}

/**
 * Represents a single tunnel session
 */
class TunnelSession(
    val sessionId: String,
    private val targetHost: String,
    private val targetPort: Int,
    private val tunnelManager: TunnelManager
) {
    private var targetSocket: Socket? = null
    private val isActive = java.util.concurrent.atomic.AtomicBoolean(false)
    private val sessionScope = CoroutineScope(Dispatchers.IO)
    
    var lastActivity: Long = System.currentTimeMillis()
        private set
    
    /**
     * Connect to the target server
     */
    suspend fun connect(): Result<Unit> = withContext(Dispatchers.IO) {
        try {
            targetSocket = Socket()
            targetSocket!!.connect(InetSocketAddress(targetHost, targetPort), 15000) // 15 sec timeout
            targetSocket!!.soTimeout = 30000 // 30 sec read timeout
            
            this@TunnelSession.isActive.set(true)
            this@TunnelSession.lastActivity = System.currentTimeMillis()
            
            IPLoopLogger.d("TunnelSession", "Connected to $targetHost:$targetPort")
            Result.success(Unit)
            
        } catch (e: Exception) {
            IPLoopLogger.e("TunnelSession", "Failed to connect to $targetHost:$targetPort", e)
            close()
            Result.failure(e)
        }
    }
    
    /**
     * Relay data from client to target
     */
    fun relayClientToTarget(clientInput: InputStream): Job {
        return sessionScope.launch {
            try {
                val targetOutput = targetSocket?.getOutputStream()
                if (targetOutput != null) {
                    relay(clientInput, targetOutput, "client->target")
                }
            } catch (e: Exception) {
                IPLoopLogger.e("TunnelSession", "Client to target relay failed", e)
                close()
            }
        }
    }
    
    /**
     * Relay data from target to client
     */
    fun relayTargetToClient(clientOutput: OutputStream): Job {
        return sessionScope.launch {
            try {
                val targetInput = targetSocket?.getInputStream()
                if (targetInput != null) {
                    relay(targetInput, clientOutput, "target->client")
                }
            } catch (e: Exception) {
                IPLoopLogger.e("TunnelSession", "Target to client relay failed", e)
                close()
            }
        }
    }
    
    /**
     * Relay data between streams
     */
    private suspend fun relay(input: InputStream, output: OutputStream, direction: String) = withContext(Dispatchers.IO) {
        val buffer = ByteArray(8192)
        var bytesRelayed = 0L
        
        try {
            while (this@TunnelSession.isActive.get()) {
                val bytesRead = input.read(buffer)
                if (bytesRead == -1) break // End of stream
                
                output.write(buffer, 0, bytesRead)
                output.flush()
                
                bytesRelayed += bytesRead
                tunnelManager.addBytesTransferred(bytesRead.toLong())
                this@TunnelSession.lastActivity = System.currentTimeMillis()
            }
            
            IPLoopLogger.d("TunnelSession", "$direction relayed $bytesRelayed bytes")
            
        } catch (e: Exception) {
            if (this@TunnelSession.isActive.get()) {
                IPLoopLogger.e("TunnelSession", "Relay error ($direction)", e)
            }
        }
    }
    
    /**
     * Close the tunnel session
     */
    fun close() {
        if (!this.isActive.compareAndSet(true, false)) return
        
        try {
            targetSocket?.close()
        } catch (e: Exception) {
            IPLoopLogger.e("TunnelSession", "Error closing target socket", e)
        }
        
        targetSocket = null
        sessionScope.cancel()
        
        IPLoopLogger.d("TunnelSession", "Closed session: $sessionId")
    }
    
    /**
     * Check if session is active
     */
    fun isSessionActive(): Boolean = isActive.get()
    
    /**
     * Get target socket
     */
    fun getTargetSocket(): Socket? = targetSocket
    
    /**
     * Get target information
     */
    fun getTargetInfo(): Pair<String, Int> = targetHost to targetPort
}