package io.iploop.node.network

import io.iploop.node.BuildConfig
import retrofit2.Response
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory
import retrofit2.http.*

/**
 * API Client for IPLoop Node Registration and Management
 */
interface ApiService {
    
    @POST("nodes/register")
    suspend fun registerNode(
        @Body request: NodeRegistrationRequest
    ): Response<NodeRegistrationResponse>
    
    @POST("nodes/{nodeId}/heartbeat")
    suspend fun sendHeartbeat(
        @Path("nodeId") nodeId: String,
        @Header("Authorization") token: String,
        @Body stats: HeartbeatRequest
    ): Response<HeartbeatResponse>
    
    @GET("nodes/{nodeId}/status")
    suspend fun getNodeStatus(
        @Path("nodeId") nodeId: String,
        @Header("Authorization") token: String
    ): Response<NodeStatusResponse>
    
    @GET("nodes/{nodeId}/earnings")
    suspend fun getEarnings(
        @Path("nodeId") nodeId: String,
        @Header("Authorization") token: String
    ): Response<EarningsResponse>
    
    @POST("nodes/{nodeId}/withdraw")
    suspend fun requestWithdrawal(
        @Path("nodeId") nodeId: String,
        @Header("Authorization") token: String,
        @Body request: WithdrawalRequest
    ): Response<WithdrawalResponse>
}

object ApiClient {
    private val retrofit = Retrofit.Builder()
        .baseUrl(BuildConfig.API_BASE_URL)
        .addConverterFactory(GsonConverterFactory.create())
        .build()
    
    val api: ApiService = retrofit.create(ApiService::class.java)
}

// Request/Response DTOs

data class NodeRegistrationRequest(
    val deviceId: String,
    val deviceInfo: DeviceInfo,
    val referralCode: String? = null
)

data class DeviceInfo(
    val model: String,
    val manufacturer: String,
    val osVersion: String,
    val sdkVersion: Int,
    val appVersion: String,
    val carrier: String?,
    val connectionType: String,
    val country: String?,
    val city: String?
)

data class NodeRegistrationResponse(
    val success: Boolean,
    val nodeId: String?,
    val token: String?,
    val message: String?
)

data class HeartbeatRequest(
    val bytesTransferred: Long,
    val requestsHandled: Long,
    val uptimeSeconds: Long,
    val batteryLevel: Int?,
    val isCharging: Boolean?,
    val connectionType: String,
    val signalStrength: Int?
)

data class HeartbeatResponse(
    val success: Boolean,
    val earnings: Double?,
    val config: NodeConfig?
)

data class NodeConfig(
    val maxConcurrentRequests: Int = 10,
    val allowedProtocols: List<String> = listOf("http", "https"),
    val rateLimitPerMinute: Int = 100,
    val maintenanceMode: Boolean = false
)

data class NodeStatusResponse(
    val nodeId: String,
    val status: String,
    val registeredAt: Long,
    val lastSeen: Long,
    val totalBytesTransferred: Long,
    val totalRequestsHandled: Long,
    val totalEarnings: Double,
    val pendingEarnings: Double,
    val withdrawnEarnings: Double
)

data class EarningsResponse(
    val totalEarnings: Double,
    val pendingEarnings: Double,
    val withdrawnEarnings: Double,
    val earningsHistory: List<EarningEntry>,
    val withdrawalHistory: List<WithdrawalEntry>
)

data class EarningEntry(
    val date: String,
    val bytesTransferred: Long,
    val requestsHandled: Long,
    val earnings: Double
)

data class WithdrawalEntry(
    val date: String,
    val amount: Double,
    val status: String,
    val method: String,
    val transactionId: String?
)

data class WithdrawalRequest(
    val amount: Double,
    val method: String, // "paypal", "crypto", "bank"
    val destination: String // email, wallet address, etc.
)

data class WithdrawalResponse(
    val success: Boolean,
    val message: String?,
    val transactionId: String?,
    val estimatedArrival: String?
)
