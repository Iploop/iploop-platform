package io.iploop.node.service

import android.app.*
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import androidx.core.app.NotificationCompat
import io.iploop.node.R
import io.iploop.node.network.GatewayConnection
import io.iploop.node.network.ProxyHandler
import io.iploop.node.ui.MainActivity
import io.iploop.node.util.NodePreferences
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

/**
 * IPLoop Node Service
 * 
 * Runs as a foreground service to maintain connection to the gateway
 * and handle proxy requests through this device's network.
 */
class NodeService : Service() {

    companion object {
        const val NOTIFICATION_ID = 1001
        const val CHANNEL_ID = "iploop_node_channel"
        
        const val ACTION_START = "io.iploop.node.START"
        const val ACTION_STOP = "io.iploop.node.STOP"
        
        private val _isRunning = MutableStateFlow(false)
        val isRunning: StateFlow<Boolean> = _isRunning
        
        private val _stats = MutableStateFlow(NodeStats())
        val stats: StateFlow<NodeStats> = _stats
    }

    private val serviceScope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    
    private lateinit var gatewayConnection: GatewayConnection
    private lateinit var proxyHandler: ProxyHandler
    private lateinit var preferences: NodePreferences
    private var wakeLock: PowerManager.WakeLock? = null
    
    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        preferences = NodePreferences(this)
        gatewayConnection = GatewayConnection(preferences)
        proxyHandler = ProxyHandler()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> startNode()
            ACTION_STOP -> stopNode()
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun startNode() {
        if (_isRunning.value) return
        
        startForeground(NOTIFICATION_ID, createNotification())
        acquireWakeLock()
        
        serviceScope.launch {
            try {
                _isRunning.value = true
                
                // Connect to gateway
                gatewayConnection.connect { request ->
                    // Handle incoming proxy request
                    proxyHandler.handleRequest(request) { bytesTransferred ->
                        updateStats(bytesTransferred)
                    }
                }
                
                // Start heartbeat
                startHeartbeat()
                
            } catch (e: Exception) {
                _isRunning.value = false
                stopSelf()
            }
        }
    }

    private fun stopNode() {
        serviceScope.launch {
            _isRunning.value = false
            gatewayConnection.disconnect()
            releaseWakeLock()
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
        }
    }

    private fun startHeartbeat() {
        serviceScope.launch {
            while (_isRunning.value) {
                gatewayConnection.sendHeartbeat(_stats.value)
                delay(30_000) // Every 30 seconds
            }
        }
    }

    private fun updateStats(bytesTransferred: Long) {
        _stats.value = _stats.value.copy(
            totalBytesTransferred = _stats.value.totalBytesTransferred + bytesTransferred,
            requestsHandled = _stats.value.requestsHandled + 1,
            lastActivityTime = System.currentTimeMillis()
        )
        updateNotification()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "IPLoop Node",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Shows when IPLoop node is active"
                setShowBadge(false)
            }
            
            val notificationManager = getSystemService(NotificationManager::class.java)
            notificationManager.createNotificationChannel(channel)
        }
    }

    private fun createNotification(): Notification {
        val intent = Intent(this, MainActivity::class.java)
        val pendingIntent = PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val stopIntent = Intent(this, NodeService::class.java).apply {
            action = ACTION_STOP
        }
        val stopPendingIntent = PendingIntent.getService(
            this, 0, stopIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val stats = _stats.value
        val bytesFormatted = formatBytes(stats.totalBytesTransferred)

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("IPLoop Node Active")
            .setContentText("Shared: $bytesFormatted • ${stats.requestsHandled} requests")
            .setSmallIcon(R.drawable.ic_node)
            .setContentIntent(pendingIntent)
            .addAction(R.drawable.ic_stop, "Stop", stopPendingIntent)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
    }

    private fun updateNotification() {
        val notificationManager = getSystemService(NotificationManager::class.java)
        notificationManager.notify(NOTIFICATION_ID, createNotification())
    }

    private fun acquireWakeLock() {
        val powerManager = getSystemService(POWER_SERVICE) as PowerManager
        wakeLock = powerManager.newWakeLock(
            PowerManager.PARTIAL_WAKE_LOCK,
            "IPLoopNode::WakeLock"
        ).apply {
            acquire(10 * 60 * 1000L) // 10 minutes, will be renewed
        }
    }

    private fun releaseWakeLock() {
        wakeLock?.let {
            if (it.isHeld) it.release()
        }
        wakeLock = null
    }

    private fun formatBytes(bytes: Long): String {
        return when {
            bytes >= 1_073_741_824 -> String.format("%.2f GB", bytes / 1_073_741_824.0)
            bytes >= 1_048_576 -> String.format("%.2f MB", bytes / 1_048_576.0)
            bytes >= 1024 -> String.format("%.2f KB", bytes / 1024.0)
            else -> "$bytes B"
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        serviceScope.cancel()
        releaseWakeLock()
        _isRunning.value = false
    }
}

data class NodeStats(
    val totalBytesTransferred: Long = 0,
    val requestsHandled: Long = 0,
    val lastActivityTime: Long = 0,
    val uptimeSeconds: Long = 0,
    val earnings: Double = 0.0
)
