package com.iploop.sdk

/**
 * Configuration class for IPLoop SDK
 * Controls behavior, limits, and compliance settings
 */
data class IPLoopConfig(
    /**
     * Only operate on WiFi connections (no cellular data)
     * Default: true (privacy-friendly)
     */
    val wifiOnly: Boolean = true,
    
    /**
     * Maximum bandwidth per day in MB
     * Default: 100MB (conservative)
     */
    val maxBandwidthMB: Int = 100,
    
    /**
     * Maximum bandwidth per session in MB  
     * Default: 10MB
     */
    val maxSessionBandwidthMB: Int = 10,
    
    /**
     * Only operate when device is charging
     * Default: false (more flexible)
     */
    val chargingOnly: Boolean = false,
    
    /**
     * Only operate when device battery > threshold
     * Default: 20%
     */
    val minBatteryLevel: Int = 20,
    
    /**
     * Node registration service URL
     * Default: IPLoop production endpoint
     */
    val registrationUrl: String = "wss://api.iploop.com/ws",
    
    /**
     * Heartbeat interval in seconds
     * Default: 30 seconds
     */
    val heartbeatIntervalSec: Int = 30,
    
    /**
     * Connection timeout in seconds
     * Default: 15 seconds
     */
    val connectionTimeoutSec: Int = 15,
    
    /**
     * Traffic relay timeout in seconds
     * Default: 30 seconds
     */
    val trafficTimeoutSec: Int = 30,
    
    /**
     * Enable location sharing for geo-targeting
     * Default: false (privacy-friendly)
     */
    val shareLocation: Boolean = false,
    
    /**
     * Enable detailed logging
     * Default: false (production)
     */
    val debugMode: Boolean = false,
    
    /**
     * Custom device identifier
     * Default: null (auto-generate)
     */
    val customDeviceId: String? = null,
    
    /**
     * Maximum concurrent connections
     * Default: 5
     */
    val maxConcurrentConnections: Int = 5,
    
    /**
     * Enable consent dialog auto-show
     * Default: true
     */
    val autoShowConsent: Boolean = true,
    
    /**
     * Notification icon resource ID
     * Default: 0 (use SDK default)
     */
    val notificationIcon: Int = 0,
    
    /**
     * Custom notification title
     * Default: null (use SDK default)
     */
    val notificationTitle: String? = null,
    
    /**
     * Custom notification text
     * Default: null (use SDK default)
     */
    val notificationText: String? = null
) {
    
    /**
     * Builder pattern for creating IPLoopConfig
     */
    class Builder {
        private var wifiOnly: Boolean = true
        private var maxBandwidthMB: Int = 100
        private var maxSessionBandwidthMB: Int = 10
        private var chargingOnly: Boolean = false
        private var minBatteryLevel: Int = 20
        private var registrationUrl: String = "wss://api.iploop.com/ws"
        private var heartbeatIntervalSec: Int = 30
        private var connectionTimeoutSec: Int = 15
        private var trafficTimeoutSec: Int = 30
        private var shareLocation: Boolean = false
        private var debugMode: Boolean = false
        private var customDeviceId: String? = null
        private var maxConcurrentConnections: Int = 5
        private var autoShowConsent: Boolean = true
        private var notificationIcon: Int = 0
        private var notificationTitle: String? = null
        private var notificationText: String? = null
        
        fun setWifiOnly(wifiOnly: Boolean) = apply { this.wifiOnly = wifiOnly }
        fun setMaxBandwidthMB(maxBandwidthMB: Int) = apply { 
            require(maxBandwidthMB > 0) { "maxBandwidthMB must be positive" }
            this.maxBandwidthMB = maxBandwidthMB 
        }
        fun setMaxSessionBandwidthMB(maxSessionBandwidthMB: Int) = apply { 
            require(maxSessionBandwidthMB > 0) { "maxSessionBandwidthMB must be positive" }
            this.maxSessionBandwidthMB = maxSessionBandwidthMB 
        }
        fun setChargingOnly(chargingOnly: Boolean) = apply { this.chargingOnly = chargingOnly }
        fun setMinBatteryLevel(minBatteryLevel: Int) = apply { 
            require(minBatteryLevel in 0..100) { "minBatteryLevel must be 0-100" }
            this.minBatteryLevel = minBatteryLevel 
        }
        fun setRegistrationUrl(registrationUrl: String) = apply { 
            require(registrationUrl.startsWith("ws://") || registrationUrl.startsWith("wss://")) {
                "registrationUrl must be a WebSocket URL"
            }
            this.registrationUrl = registrationUrl 
        }
        fun setHeartbeatIntervalSec(heartbeatIntervalSec: Int) = apply { 
            require(heartbeatIntervalSec >= 10) { "heartbeatIntervalSec must be >= 10" }
            this.heartbeatIntervalSec = heartbeatIntervalSec 
        }
        fun setConnectionTimeoutSec(connectionTimeoutSec: Int) = apply { 
            require(connectionTimeoutSec > 0) { "connectionTimeoutSec must be positive" }
            this.connectionTimeoutSec = connectionTimeoutSec 
        }
        fun setTrafficTimeoutSec(trafficTimeoutSec: Int) = apply { 
            require(trafficTimeoutSec > 0) { "trafficTimeoutSec must be positive" }
            this.trafficTimeoutSec = trafficTimeoutSec 
        }
        fun setShareLocation(shareLocation: Boolean) = apply { this.shareLocation = shareLocation }
        fun setDebugMode(debugMode: Boolean) = apply { this.debugMode = debugMode }
        fun setCustomDeviceId(customDeviceId: String?) = apply { this.customDeviceId = customDeviceId }
        fun setMaxConcurrentConnections(maxConcurrentConnections: Int) = apply { 
            require(maxConcurrentConnections > 0) { "maxConcurrentConnections must be positive" }
            this.maxConcurrentConnections = maxConcurrentConnections 
        }
        fun setAutoShowConsent(autoShowConsent: Boolean) = apply { this.autoShowConsent = autoShowConsent }
        fun setNotificationIcon(notificationIcon: Int) = apply { this.notificationIcon = notificationIcon }
        fun setNotificationTitle(notificationTitle: String?) = apply { this.notificationTitle = notificationTitle }
        fun setNotificationText(notificationText: String?) = apply { this.notificationText = notificationText }
        
        fun build(): IPLoopConfig {
            return IPLoopConfig(
                wifiOnly = wifiOnly,
                maxBandwidthMB = maxBandwidthMB,
                maxSessionBandwidthMB = maxSessionBandwidthMB,
                chargingOnly = chargingOnly,
                minBatteryLevel = minBatteryLevel,
                registrationUrl = registrationUrl,
                heartbeatIntervalSec = heartbeatIntervalSec,
                connectionTimeoutSec = connectionTimeoutSec,
                trafficTimeoutSec = trafficTimeoutSec,
                shareLocation = shareLocation,
                debugMode = debugMode,
                customDeviceId = customDeviceId,
                maxConcurrentConnections = maxConcurrentConnections,
                autoShowConsent = autoShowConsent,
                notificationIcon = notificationIcon,
                notificationTitle = notificationTitle,
                notificationText = notificationText
            )
        }
    }
    
    companion object {
        /**
         * Create a default configuration
         */
        @JvmStatic
        fun createDefault(): IPLoopConfig = Builder().build()
        
        /**
         * Create a privacy-focused configuration
         */
        @JvmStatic
        fun createPrivacyFriendly(): IPLoopConfig = Builder()
            .setWifiOnly(true)
            .setMaxBandwidthMB(50)
            .setChargingOnly(true)
            .setMinBatteryLevel(30)
            .setShareLocation(false)
            .build()
            
        /**
         * Create a performance-optimized configuration
         */
        @JvmStatic
        fun createHighPerformance(): IPLoopConfig = Builder()
            .setWifiOnly(false)
            .setMaxBandwidthMB(500)
            .setMaxSessionBandwidthMB(50)
            .setMinBatteryLevel(15)
            .setMaxConcurrentConnections(10)
            .build()
    }
    
    /**
     * Validate configuration values
     */
    fun validate() {
        require(maxBandwidthMB > 0) { "maxBandwidthMB must be positive" }
        require(maxSessionBandwidthMB > 0) { "maxSessionBandwidthMB must be positive" }
        require(maxSessionBandwidthMB <= maxBandwidthMB) { "maxSessionBandwidthMB cannot exceed maxBandwidthMB" }
        require(minBatteryLevel in 0..100) { "minBatteryLevel must be 0-100" }
        require(heartbeatIntervalSec >= 10) { "heartbeatIntervalSec must be >= 10 seconds" }
        require(connectionTimeoutSec > 0) { "connectionTimeoutSec must be positive" }
        require(trafficTimeoutSec > 0) { "trafficTimeoutSec must be positive" }
        require(maxConcurrentConnections > 0) { "maxConcurrentConnections must be positive" }
        require(registrationUrl.startsWith("ws://") || registrationUrl.startsWith("wss://")) {
            "registrationUrl must be a WebSocket URL"
        }
    }
}