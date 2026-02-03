package com.iploop.sdk.internal

import android.Manifest
import android.annotation.SuppressLint
import android.content.Context
import android.content.pm.PackageManager
import android.location.Location
import android.location.LocationManager
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.wifi.WifiManager
import android.os.BatteryManager
import android.os.Build
import android.provider.Settings
import android.telephony.TelephonyManager
// Permission checks done via context.checkSelfPermission for API 23+
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.InetAddress
import java.net.NetworkInterface
import java.security.MessageDigest
import java.util.*

/**
 * Collects device and network information for node registration
 * Handles privacy-compliant data collection
 */
object DeviceInfo {
    
    /**
     * Get comprehensive device information for registration
     */
    @SuppressLint("HardwareIds")
    fun getDeviceInfo(context: Context): Map<String, Any> {
        val info = mutableMapOf<String, Any>()
        
        try {
            // Device basics
            info["device_type"] = "android"
            info["device_model"] = "${Build.MANUFACTURER} ${Build.MODEL}"
            info["android_version"] = Build.VERSION.RELEASE
            info["sdk_version"] = Build.VERSION.SDK_INT
            info["app_version"] = getAppVersion(context)
            info["sdk_build"] = "1.0.0"
            
            // Generate privacy-friendly device ID
            info["device_id"] = generateDeviceId(context)
            
            // Network information
            val networkInfo = getNetworkInfo(context)
            info.putAll(networkInfo)
            
            // Battery information
            val batteryInfo = getBatteryInfo(context)
            info.putAll(batteryInfo)
            
            // Location (if permitted)
            val locationInfo = getLocationInfo(context)
            if (locationInfo.isNotEmpty()) {
                info.putAll(locationInfo)
            }
            
            // Timestamp
            info["collected_at"] = System.currentTimeMillis()
            
        } catch (e: Exception) {
            IPLoopLogger.e("DeviceInfo", "Error collecting device info", e)
        }
        
        return info
    }
    
    /**
     * Get network connection information
     */
    fun getNetworkInfo(context: Context): Map<String, Any> {
        val info = mutableMapOf<String, Any>()
        
        try {
            val connectivityManager = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
            val activeNetwork = connectivityManager.activeNetwork
            val capabilities = connectivityManager.getNetworkCapabilities(activeNetwork)
            
            if (capabilities != null) {
                when {
                    capabilities.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> {
                        info["connection_type"] = "wifi"
                        addWifiInfo(context, info)
                    }
                    capabilities.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> {
                        info["connection_type"] = "cellular"
                        addCellularInfo(context, info)
                    }
                    capabilities.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> {
                        info["connection_type"] = "ethernet"
                    }
                    else -> {
                        info["connection_type"] = "unknown"
                    }
                }
                
                // Network capabilities
                info["has_internet"] = capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
                info["is_metered"] = !capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED)
            }
            
            // Get IP address
            val ipAddress = getLocalIpAddress()
            if (ipAddress != null) {
                info["local_ip"] = ipAddress
            }
            
        } catch (e: Exception) {
            IPLoopLogger.e("DeviceInfo", "Error getting network info", e)
            info["connection_type"] = "unknown"
        }
        
        return info
    }
    
    /**
     * Add WiFi specific information
     */
    @SuppressLint("HardwareIds")
    private fun addWifiInfo(context: Context, info: MutableMap<String, Any>) {
        try {
            val wifiManager = context.applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
            val wifiInfo = wifiManager.connectionInfo
            
            if (wifiInfo != null) {
                info["wifi_ssid"] = wifiInfo.ssid?.replace("\"", "") ?: "Unknown"
                info["wifi_bssid"] = wifiInfo.bssid ?: "Unknown"
                info["wifi_rssi"] = wifiInfo.rssi
                info["wifi_link_speed"] = wifiInfo.linkSpeed
                info["wifi_frequency"] = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
                    wifiInfo.frequency
                } else 0
            }
        } catch (e: Exception) {
            IPLoopLogger.e("DeviceInfo", "Error getting WiFi info", e)
        }
    }
    
    /**
     * Add cellular specific information
     */
    private fun addCellularInfo(context: Context, info: MutableMap<String, Any>) {
        try {
            val telephonyManager = context.getSystemService(Context.TELEPHONY_SERVICE) as TelephonyManager
            
            if (context.checkSelfPermission(Manifest.permission.READ_PHONE_STATE) == PackageManager.PERMISSION_GRANTED) {
                val carrierName = telephonyManager.networkOperatorName
                if (carrierName.isNotEmpty()) {
                    info["carrier"] = carrierName
                }
                
                val networkOperator = telephonyManager.networkOperator
                if (networkOperator.isNotEmpty() && networkOperator.length >= 3) {
                    info["mcc"] = networkOperator.substring(0, 3)
                    if (networkOperator.length >= 5) {
                        info["mnc"] = networkOperator.substring(3)
                    }
                }
                
                info["network_type"] = getNetworkTypeName(telephonyManager.dataNetworkType)
            }
        } catch (e: Exception) {
            IPLoopLogger.e("DeviceInfo", "Error getting cellular info", e)
        }
    }
    
    /**
     * Get battery information
     */
    fun getBatteryInfo(context: Context): Map<String, Any> {
        val info = mutableMapOf<String, Any>()
        
        try {
            val batteryManager = context.getSystemService(Context.BATTERY_SERVICE) as BatteryManager
            
            val batteryLevel = batteryManager.getIntProperty(BatteryManager.BATTERY_PROPERTY_CAPACITY)
            info["battery_level"] = batteryLevel
            
            val status = batteryManager.getIntProperty(BatteryManager.BATTERY_PROPERTY_STATUS)
            info["is_charging"] = status == BatteryManager.BATTERY_STATUS_CHARGING
            
            val plugged = batteryManager.getIntProperty(BatteryManager.BATTERY_PROPERTY_CHARGE_COUNTER)
            info["is_plugged"] = plugged > 0
            
        } catch (e: Exception) {
            IPLoopLogger.e("DeviceInfo", "Error getting battery info", e)
        }
        
        return info
    }
    
    /**
     * Get location information (if permission granted)
     */
    @SuppressLint("MissingPermission")
    private fun getLocationInfo(context: Context): Map<String, Any> {
        val info = mutableMapOf<String, Any>()
        
        if (context.checkSelfPermission(Manifest.permission.ACCESS_COARSE_LOCATION) != PackageManager.PERMISSION_GRANTED) {
            return info // No permission, return empty
        }
        
        try {
            val locationManager = context.getSystemService(Context.LOCATION_SERVICE) as LocationManager
            val providers = locationManager.getProviders(true)
            
            for (provider in providers) {
                val location = locationManager.getLastKnownLocation(provider)
                if (location != null && isLocationFresh(location)) {
                    info["latitude"] = location.latitude
                    info["longitude"] = location.longitude
                    info["location_accuracy"] = location.accuracy
                    info["location_provider"] = provider
                    info["location_time"] = location.time
                    break // Use first fresh location
                }
            }
        } catch (e: Exception) {
            IPLoopLogger.e("DeviceInfo", "Error getting location info", e)
        }
        
        return info
    }
    
    /**
     * Generate privacy-friendly device ID
     */
    @SuppressLint("HardwareIds")
    private fun generateDeviceId(context: Context): String {
        return try {
            // Use multiple identifiers to create a stable but anonymous ID
            val androidId = Settings.Secure.getString(context.contentResolver, Settings.Secure.ANDROID_ID)
            val model = Build.MODEL
            val manufacturer = Build.MANUFACTURER
            val combined = "$androidId-$manufacturer-$model"
            
            // Hash for privacy
            val digest = MessageDigest.getInstance("SHA-256")
            val hash = digest.digest(combined.toByteArray())
            hash.joinToString("") { "%02x".format(it) }.take(16)
        } catch (e: Exception) {
            // Fallback to UUID
            UUID.randomUUID().toString().replace("-", "").take(16)
        }
    }
    
    /**
     * Get local IP address
     */
    private fun getLocalIpAddress(): String? {
        try {
            val interfaces = NetworkInterface.getNetworkInterfaces()
            for (networkInterface in interfaces) {
                val addresses = networkInterface.inetAddresses
                for (address in addresses) {
                    if (!address.isLoopbackAddress && address is InetAddress) {
                        val hostAddress = address.hostAddress
                        if (hostAddress?.contains(":") == false) { // IPv4
                            return hostAddress
                        }
                    }
                }
            }
        } catch (e: Exception) {
            IPLoopLogger.e("DeviceInfo", "Error getting local IP", e)
        }
        return null
    }
    
    /**
     * Get app version
     */
    private fun getAppVersion(context: Context): String {
        return try {
            val packageInfo = context.packageManager.getPackageInfo(context.packageName, 0)
            packageInfo.versionName ?: "unknown"
        } catch (e: Exception) {
            "unknown"
        }
    }
    
    /**
     * Check if location is fresh (within 10 minutes)
     */
    private fun isLocationFresh(location: Location): Boolean {
        val maxAge = 10 * 60 * 1000 // 10 minutes
        return (System.currentTimeMillis() - location.time) < maxAge
    }
    
    /**
     * Get human-readable network type name
     */
    private fun getNetworkTypeName(networkType: Int): String {
        return when (networkType) {
            TelephonyManager.NETWORK_TYPE_GPRS -> "GPRS"
            TelephonyManager.NETWORK_TYPE_EDGE -> "EDGE"
            TelephonyManager.NETWORK_TYPE_UMTS -> "UMTS"
            TelephonyManager.NETWORK_TYPE_HSDPA -> "HSDPA"
            TelephonyManager.NETWORK_TYPE_HSUPA -> "HSUPA"
            TelephonyManager.NETWORK_TYPE_HSPA -> "HSPA"
            TelephonyManager.NETWORK_TYPE_CDMA -> "CDMA"
            TelephonyManager.NETWORK_TYPE_EVDO_0 -> "EVDO_0"
            TelephonyManager.NETWORK_TYPE_EVDO_A -> "EVDO_A"
            TelephonyManager.NETWORK_TYPE_EVDO_B -> "EVDO_B"
            TelephonyManager.NETWORK_TYPE_1xRTT -> "1xRTT"
            TelephonyManager.NETWORK_TYPE_IDEN -> "iDen"
            TelephonyManager.NETWORK_TYPE_LTE -> "LTE"
            TelephonyManager.NETWORK_TYPE_EHRPD -> "eHRPD"
            TelephonyManager.NETWORK_TYPE_HSPAP -> "HSPA+"
            else -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                when (networkType) {
                    TelephonyManager.NETWORK_TYPE_NR -> "5G"
                    else -> "Unknown"
                }
            } else "Unknown"
        }
    }
}