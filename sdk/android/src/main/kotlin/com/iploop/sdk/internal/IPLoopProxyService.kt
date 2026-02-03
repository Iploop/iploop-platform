package com.iploop.sdk.internal

import android.app.*
import android.content.Context
import android.content.Intent
import android.net.wifi.WifiManager
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import com.iploop.sdk.IPLoopSDK

/**
 * Foreground service for IPLoop proxy operations
 * Maintains background connectivity and compliance with Android background restrictions
 */
class IPLoopProxyService : Service() {
    
    companion object {
        private const val NOTIFICATION_ID = 1001
        private const val CHANNEL_ID = "iploop_proxy_channel"
        private const val CHANNEL_NAME = "IPLoop Proxy Service"
    }
    
    private var wifiLock: WifiManager.WifiLock? = null
    private var wakeLock: PowerManager.WakeLock? = null
    
    override fun onCreate() {
        super.onCreate()
        IPLoopLogger.i("IPLoopProxyService", "Service created")
        createNotificationChannel()
        acquireWakeLocks()
    }
    
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        IPLoopLogger.i("IPLoopProxyService", "Service started")
        
        // Start foreground service with notification
        startForeground(NOTIFICATION_ID, createNotification())
        
        return START_STICKY // Restart if killed
    }
    
    override fun onDestroy() {
        super.onDestroy()
        releaseWakeLocks()
        IPLoopLogger.i("IPLoopProxyService", "Service destroyed")
    }
    
    /**
     * Acquire WiFi and CPU wake locks to prevent connection drops
     */
    @Suppress("DEPRECATION")
    private fun acquireWakeLocks() {
        try {
            // WiFi lock - keeps WiFi active when screen is off
            val wifiManager = applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
            wifiLock = wifiManager.createWifiLock(WifiManager.WIFI_MODE_FULL_HIGH_PERF, "IPLoop:WifiLock")
            wifiLock?.setReferenceCounted(false)
            wifiLock?.acquire()
            IPLoopLogger.i("IPLoopProxyService", "WiFi lock acquired")
            
            // Partial wake lock - keeps CPU running
            val powerManager = getSystemService(Context.POWER_SERVICE) as PowerManager
            wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "IPLoop:WakeLock")
            wakeLock?.setReferenceCounted(false)
            wakeLock?.acquire()
            IPLoopLogger.i("IPLoopProxyService", "Wake lock acquired")
            
        } catch (e: Exception) {
            IPLoopLogger.e("IPLoopProxyService", "Failed to acquire wake locks", e)
        }
    }
    
    /**
     * Release wake locks
     */
    private fun releaseWakeLocks() {
        try {
            wifiLock?.let {
                if (it.isHeld) {
                    it.release()
                    IPLoopLogger.i("IPLoopProxyService", "WiFi lock released")
                }
            }
            wifiLock = null
            
            wakeLock?.let {
                if (it.isHeld) {
                    it.release()
                    IPLoopLogger.i("IPLoopProxyService", "Wake lock released")
                }
            }
            wakeLock = null
            
        } catch (e: Exception) {
            IPLoopLogger.e("IPLoopProxyService", "Failed to release wake locks", e)
        }
    }
    
    override fun onBind(intent: Intent?): IBinder? {
        return null // Not a bound service
    }
    
    /**
     * Create notification channel for Android O+
     */
    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                CHANNEL_NAME,
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Keeps IPLoop proxy service running in background"
                setShowBadge(false)
                enableLights(false)
                enableVibration(false)
                setSound(null, null)
            }
            
            val notificationManager = getSystemService(NotificationManager::class.java)
            notificationManager.createNotificationChannel(channel)
        }
    }
    
    /**
     * Create foreground service notification
     */
    @Suppress("DEPRECATION")
    private fun createNotification(): Notification {
        val appName = applicationInfo.loadLabel(packageManager).toString()
        
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
                .setContentTitle("$appName - Proxy Service")
                .setContentText("Sharing connection securely")
                .setSmallIcon(getNotificationIcon())
                .setOngoing(true)
                .setAutoCancel(false)
                .build()
        } else {
            Notification.Builder(this)
                .setContentTitle("$appName - Proxy Service")
                .setContentText("Sharing connection securely")
                .setSmallIcon(getNotificationIcon())
                .setOngoing(true)
                .setAutoCancel(false)
                .setPriority(Notification.PRIORITY_LOW)
                .build()
        }
    }
    
    /**
     * Get notification icon resource ID
     */
    private fun getNotificationIcon(): Int {
        return try {
            applicationInfo.icon
        } catch (e: Exception) {
            android.R.drawable.ic_dialog_info
        }
    }
    
    /**
     * Update notification with current status
     */
    @Suppress("DEPRECATION")
    fun updateNotification(status: String, bandwidthUsed: String? = null) {
        val appName = applicationInfo.loadLabel(packageManager).toString()
        val text = if (bandwidthUsed != null) {
            "$status - $bandwidthUsed used"
        } else {
            status
        }
        
        val notification = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
                .setContentTitle("$appName - Proxy Service")
                .setContentText(text)
                .setSmallIcon(getNotificationIcon())
                .setOngoing(true)
                .setAutoCancel(false)
                .build()
        } else {
            Notification.Builder(this)
                .setContentTitle("$appName - Proxy Service")
                .setContentText(text)
                .setSmallIcon(getNotificationIcon())
                .setOngoing(true)
                .setAutoCancel(false)
                .setPriority(Notification.PRIORITY_LOW)
                .build()
        }
        
        val notificationManager = getSystemService(NotificationManager::class.java)
        notificationManager.notify(NOTIFICATION_ID, notification)
        
        IPLoopLogger.d("IPLoopProxyService", "Notification updated: $text")
    }
}
