package io.iploop.node.util

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.*
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.runBlocking

private val Context.dataStore: DataStore<Preferences> by preferencesDataStore(name = "iploop_node")

/**
 * Manages persistent storage for the IPLoop Node
 */
class NodePreferences(private val context: Context) {
    
    companion object {
        private val NODE_ID = stringPreferencesKey("node_id")
        private val AUTH_TOKEN = stringPreferencesKey("auth_token")
        private val DEVICE_ID = stringPreferencesKey("device_id")
        private val TOTAL_BYTES = longPreferencesKey("total_bytes")
        private val TOTAL_REQUESTS = longPreferencesKey("total_requests")
        private val TOTAL_EARNINGS = doublePreferencesKey("total_earnings")
        private val FIRST_LAUNCH = booleanPreferencesKey("first_launch")
        private val AUTO_START = booleanPreferencesKey("auto_start")
        private val WIFI_ONLY = booleanPreferencesKey("wifi_only")
        private val BATTERY_SAVER = booleanPreferencesKey("battery_saver")
        private val REFERRAL_CODE = stringPreferencesKey("referral_code")
    }
    
    // Node Identity
    
    suspend fun setNodeId(nodeId: String) {
        context.dataStore.edit { it[NODE_ID] = nodeId }
    }
    
    suspend fun getNodeId(): String? {
        return context.dataStore.data.first()[NODE_ID]
    }
    
    fun getNodeIdBlocking(): String? = runBlocking { getNodeId() }
    
    suspend fun setAuthToken(token: String) {
        context.dataStore.edit { it[AUTH_TOKEN] = token }
    }
    
    suspend fun getAuthToken(): String? {
        return context.dataStore.data.first()[AUTH_TOKEN]
    }
    
    suspend fun setDeviceId(deviceId: String) {
        context.dataStore.edit { it[DEVICE_ID] = deviceId }
    }
    
    suspend fun getDeviceId(): String? {
        return context.dataStore.data.first()[DEVICE_ID]
    }
    
    // Stats
    
    suspend fun addBytes(bytes: Long) {
        context.dataStore.edit { prefs ->
            val current = prefs[TOTAL_BYTES] ?: 0L
            prefs[TOTAL_BYTES] = current + bytes
        }
    }
    
    suspend fun addRequests(requests: Long) {
        context.dataStore.edit { prefs ->
            val current = prefs[TOTAL_REQUESTS] ?: 0L
            prefs[TOTAL_REQUESTS] = current + requests
        }
    }
    
    suspend fun addEarnings(earnings: Double) {
        context.dataStore.edit { prefs ->
            val current = prefs[TOTAL_EARNINGS] ?: 0.0
            prefs[TOTAL_EARNINGS] = current + earnings
        }
    }
    
    val statsFlow: Flow<NodeLocalStats> = context.dataStore.data.map { prefs ->
        NodeLocalStats(
            totalBytes = prefs[TOTAL_BYTES] ?: 0L,
            totalRequests = prefs[TOTAL_REQUESTS] ?: 0L,
            totalEarnings = prefs[TOTAL_EARNINGS] ?: 0.0
        )
    }
    
    // Settings
    
    suspend fun setAutoStart(enabled: Boolean) {
        context.dataStore.edit { it[AUTO_START] = enabled }
    }
    
    val autoStartFlow: Flow<Boolean> = context.dataStore.data.map { it[AUTO_START] ?: false }
    
    suspend fun setWifiOnly(enabled: Boolean) {
        context.dataStore.edit { it[WIFI_ONLY] = enabled }
    }
    
    val wifiOnlyFlow: Flow<Boolean> = context.dataStore.data.map { it[WIFI_ONLY] ?: true }
    
    suspend fun setBatterySaver(enabled: Boolean) {
        context.dataStore.edit { it[BATTERY_SAVER] = enabled }
    }
    
    val batterySaverFlow: Flow<Boolean> = context.dataStore.data.map { it[BATTERY_SAVER] ?: true }
    
    // First Launch
    
    suspend fun isFirstLaunch(): Boolean {
        return context.dataStore.data.first()[FIRST_LAUNCH] ?: true
    }
    
    suspend fun setFirstLaunchCompleted() {
        context.dataStore.edit { it[FIRST_LAUNCH] = false }
    }
    
    // Referral
    
    suspend fun setReferralCode(code: String) {
        context.dataStore.edit { it[REFERRAL_CODE] = code }
    }
    
    suspend fun getReferralCode(): String? {
        return context.dataStore.data.first()[REFERRAL_CODE]
    }
    
    // Clear all data
    
    suspend fun clearAll() {
        context.dataStore.edit { it.clear() }
    }
    
    fun isRegistered(): Boolean = runBlocking {
        getNodeId() != null && getAuthToken() != null
    }
}

data class NodeLocalStats(
    val totalBytes: Long,
    val totalRequests: Long,
    val totalEarnings: Double
)
