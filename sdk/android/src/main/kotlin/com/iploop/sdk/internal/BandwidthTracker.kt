package com.iploop.sdk.internal

import android.content.Context
import android.content.SharedPreferences
import com.iploop.sdk.BandwidthUsage
import kotlinx.coroutines.*
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Tracks bandwidth usage and enforces limits
 * Monitors data consumption for compliance and billing
 */
class BandwidthTracker(
    private val context: Context
) {
    private val prefs: SharedPreferences = context.getSharedPreferences("iploop_bandwidth", Context.MODE_PRIVATE)
    private val isRunning = AtomicBoolean(false)
    private val trackerScope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    
    // Current session counters
    private val sessionUploadedBytes = AtomicLong(0)
    private val sessionDownloadedBytes = AtomicLong(0)
    private val sessionCount = AtomicLong(0)
    
    // Persistent counters (stored in SharedPreferences)
    private var totalUploadedBytes: Long
        get() = prefs.getLong(KEY_TOTAL_UPLOADED, 0)
        set(value) = prefs.edit().putLong(KEY_TOTAL_UPLOADED, value).apply()
    
    private var totalDownloadedBytes: Long
        get() = prefs.getLong(KEY_TOTAL_DOWNLOADED, 0)
        set(value) = prefs.edit().putLong(KEY_TOTAL_DOWNLOADED, value).apply()
    
    private var dailyUploadedBytes: Long
        get() = prefs.getLong(KEY_DAILY_UPLOADED, 0)
        set(value) = prefs.edit().putLong(KEY_DAILY_UPLOADED, value).apply()
    
    private var dailyDownloadedBytes: Long
        get() = prefs.getLong(KEY_DAILY_DOWNLOADED, 0)
        set(value) = prefs.edit().putLong(KEY_DAILY_DOWNLOADED, value).apply()
    
    private var lastResetDate: String
        get() = prefs.getString(KEY_LAST_RESET_DATE, "") ?: ""
        set(value) = prefs.edit().putString(KEY_LAST_RESET_DATE, value).apply()
    
    private var lastActivity: Long
        get() = prefs.getLong(KEY_LAST_ACTIVITY, 0)
        set(value) = prefs.edit().putLong(KEY_LAST_ACTIVITY, value).apply()
    
    // Listeners
    private var onBandwidthLimitExceeded: (() -> Unit)? = null
    private var onUsageUpdate: ((BandwidthUsage) -> Unit)? = null
    
    companion object {
        private const val KEY_TOTAL_UPLOADED = "total_uploaded"
        private const val KEY_TOTAL_DOWNLOADED = "total_downloaded"
        private const val KEY_DAILY_UPLOADED = "daily_uploaded"
        private const val KEY_DAILY_DOWNLOADED = "daily_downloaded"
        private const val KEY_LAST_RESET_DATE = "last_reset_date"
        private const val KEY_LAST_ACTIVITY = "last_activity"
        
        private const val BYTES_PER_MB = 1024 * 1024
        private const val SYNC_INTERVAL_MS = 30000L // 30 seconds
    }
    
    /**
     * Start bandwidth tracking
     */
    fun start() {
        if (isRunning.get()) return
        
        IPLoopLogger.i("BandwidthTracker", "Starting bandwidth tracker")
        isRunning.set(true)
        
        // Reset daily counters if needed
        checkDailyReset()
        
        // Start periodic sync job
        startSyncJob()
    }
    
    /**
     * Stop bandwidth tracking
     */
    fun stop() {
        if (!isRunning.get()) return
        
        IPLoopLogger.i("BandwidthTracker", "Stopping bandwidth tracker")
        isRunning.set(false)
        
        // Sync final counts
        syncCounters()
        
        // Cancel coroutines
        trackerScope.cancel()
    }
    
    /**
     * Track uploaded data
     */
    fun trackUpload(bytes: Long) {
        if (bytes <= 0) return
        
        sessionUploadedBytes.addAndGet(bytes)
        lastActivity = System.currentTimeMillis()
        
        IPLoopLogger.d("BandwidthTracker", "Uploaded: $bytes bytes")
        
        // Check limits
        checkBandwidthLimits()
    }
    
    /**
     * Track downloaded data
     */
    fun trackDownload(bytes: Long) {
        if (bytes <= 0) return
        
        sessionDownloadedBytes.addAndGet(bytes)
        lastActivity = System.currentTimeMillis()
        
        IPLoopLogger.d("BandwidthTracker", "Downloaded: $bytes bytes")
        
        // Check limits
        checkBandwidthLimits()
    }
    
    /**
     * Track bidirectional data (when direction is unknown)
     */
    fun trackData(bytes: Long) {
        if (bytes <= 0) return
        
        // Split evenly between upload and download for unknown direction
        val halfBytes = bytes / 2
        trackUpload(halfBytes)
        trackDownload(bytes - halfBytes)
    }
    
    /**
     * Track a new session
     */
    fun trackSession() {
        sessionCount.incrementAndGet()
    }
    
    /**
     * Get current usage statistics
     */
    fun getCurrentUsage(): BandwidthUsage {
        syncCounters()
        
        val totalUploaded = totalUploadedBytes + sessionUploadedBytes.get()
        val totalDownloaded = totalDownloadedBytes + sessionDownloadedBytes.get()
        
        return BandwidthUsage(
            uploadedMB = totalUploaded / BYTES_PER_MB,
            downloadedMB = totalDownloaded / BYTES_PER_MB,
            totalMB = (totalUploaded + totalDownloaded) / BYTES_PER_MB,
            sessionsCount = sessionCount.get().toInt(),
            lastActivity = lastActivity
        )
    }
    
    /**
     * Get daily usage statistics
     */
    fun getDailyUsage(): BandwidthUsage {
        checkDailyReset()
        
        val dailyUploaded = dailyUploadedBytes + sessionUploadedBytes.get()
        val dailyDownloaded = dailyDownloadedBytes + sessionDownloadedBytes.get()
        
        return BandwidthUsage(
            uploadedMB = dailyUploaded / BYTES_PER_MB,
            downloadedMB = dailyDownloaded / BYTES_PER_MB,
            totalMB = (dailyUploaded + dailyDownloaded) / BYTES_PER_MB,
            sessionsCount = sessionCount.get().toInt(),
            lastActivity = lastActivity
        )
    }
    
    /**
     * Check if daily bandwidth limit is exceeded
     */
    fun isDailyLimitExceeded(maxDailyMB: Int): Boolean {
        val dailyUsage = getDailyUsage()
        return dailyUsage.totalMB >= maxDailyMB
    }
    
    /**
     * Check if session bandwidth limit is exceeded
     */
    fun isSessionLimitExceeded(maxSessionMB: Int): Boolean {
        val sessionTotal = (sessionUploadedBytes.get() + sessionDownloadedBytes.get()) / BYTES_PER_MB
        return sessionTotal >= maxSessionMB
    }
    
    /**
     * Reset daily counters if date changed
     */
    private fun checkDailyReset() {
        val currentDate = getCurrentDateString()
        if (lastResetDate != currentDate) {
            IPLoopLogger.i("BandwidthTracker", "Resetting daily counters for new day: $currentDate")
            
            dailyUploadedBytes = 0
            dailyDownloadedBytes = 0
            lastResetDate = currentDate
        }
    }
    
    /**
     * Check bandwidth limits and notify if exceeded
     */
    private fun checkBandwidthLimits() {
        // This would be called with specific limits from config
        // For now, just update usage
        notifyUsageUpdate()
    }
    
    /**
     * Start periodic sync job
     */
    private fun startSyncJob() {
        trackerScope.launch {
            while (isRunning.get()) {
                delay(SYNC_INTERVAL_MS)
                syncCounters()
                notifyUsageUpdate()
            }
        }
    }
    
    /**
     * Sync in-memory counters to persistent storage
     */
    private fun syncCounters() {
        val uploadedDelta = sessionUploadedBytes.getAndSet(0)
        val downloadedDelta = sessionDownloadedBytes.getAndSet(0)
        
        if (uploadedDelta > 0 || downloadedDelta > 0) {
            totalUploadedBytes += uploadedDelta
            totalDownloadedBytes += downloadedDelta
            dailyUploadedBytes += uploadedDelta
            dailyDownloadedBytes += downloadedDelta
            
            IPLoopLogger.d("BandwidthTracker", 
                "Synced: +${uploadedDelta}B up, +${downloadedDelta}B down")
        }
    }
    
    /**
     * Get current date string (YYYY-MM-DD)
     */
    private fun getCurrentDateString(): String {
        val calendar = java.util.Calendar.getInstance()
        return "%04d-%02d-%02d".format(
            calendar.get(java.util.Calendar.YEAR),
            calendar.get(java.util.Calendar.MONTH) + 1,
            calendar.get(java.util.Calendar.DAY_OF_MONTH)
        )
    }
    
    /**
     * Clear all bandwidth data (for testing)
     */
    fun clearAllData() {
        sessionUploadedBytes.set(0)
        sessionDownloadedBytes.set(0)
        sessionCount.set(0)
        totalUploadedBytes = 0
        totalDownloadedBytes = 0
        dailyUploadedBytes = 0
        dailyDownloadedBytes = 0
        lastActivity = 0
        
        IPLoopLogger.i("BandwidthTracker", "All bandwidth data cleared")
    }
    
    /**
     * Export usage data for reporting
     */
    fun exportUsageData(): Map<String, Any> {
        val currentUsage = getCurrentUsage()
        val dailyUsage = getDailyUsage()
        
        return mapOf(
            "total_usage" to mapOf(
                "uploaded_mb" to currentUsage.uploadedMB,
                "downloaded_mb" to currentUsage.downloadedMB,
                "total_mb" to currentUsage.totalMB
            ),
            "daily_usage" to mapOf(
                "uploaded_mb" to dailyUsage.uploadedMB,
                "downloaded_mb" to dailyUsage.downloadedMB,
                "total_mb" to dailyUsage.totalMB
            ),
            "sessions_count" to currentUsage.sessionsCount,
            "last_activity" to lastActivity,
            "tracking_date" to getCurrentDateString()
        )
    }
    
    /**
     * Set bandwidth limit exceeded listener
     */
    fun setBandwidthLimitExceededListener(listener: () -> Unit) {
        onBandwidthLimitExceeded = listener
    }
    
    /**
     * Set usage update listener
     */
    fun setUsageUpdateListener(listener: (BandwidthUsage) -> Unit) {
        onUsageUpdate = listener
    }
    
    /**
     * Notify usage update
     */
    private fun notifyUsageUpdate() {
        onUsageUpdate?.invoke(getCurrentUsage())
    }
}