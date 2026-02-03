package io.iploop.node.network

import io.iploop.node.BuildConfig
import io.iploop.node.service.NodeStats
import io.iploop.node.util.NodePreferences
import kotlinx.coroutines.*
import okhttp3.*
import okio.ByteString
import com.google.gson.Gson
import java.util.concurrent.TimeUnit

/**
 * Manages WebSocket connection to the IPLoop Gateway
 * 
 * Protocol:
 * 1. Node connects and authenticates with nodeId + token
 * 2. Gateway sends proxy requests to node
 * 3. Node executes requests and returns responses
 * 4. Node sends periodic heartbeats with stats
 */
class GatewayConnection(
    private val preferences: NodePreferences
) {
    private val client = OkHttpClient.Builder()
        .pingInterval(30, TimeUnit.SECONDS)
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .build()
    
    private var webSocket: WebSocket? = null
    private val gson = Gson()
    private var requestHandler: (suspend (ProxyRequest) -> ProxyResponse)? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    
    suspend fun connect(onRequest: suspend (ProxyRequest) -> ProxyResponse) {
        requestHandler = onRequest
        
        val nodeId = preferences.getNodeId() ?: throw IllegalStateException("Node not registered")
        val token = preferences.getAuthToken() ?: throw IllegalStateException("No auth token")
        
        val request = Request.Builder()
            .url("${BuildConfig.GATEWAY_URL}/node/connect")
            .addHeader("X-Node-Id", nodeId)
            .addHeader("Authorization", "Bearer $token")
            .build()
        
        webSocket = client.newWebSocket(request, createListener())
    }
    
    fun disconnect() {
        webSocket?.close(1000, "Node stopped")
        webSocket = null
        scope.cancel()
    }
    
    fun sendHeartbeat(stats: NodeStats) {
        val message = GatewayMessage(
            type = "heartbeat",
            payload = mapOf(
                "bytesTransferred" to stats.totalBytesTransferred,
                "requestsHandled" to stats.requestsHandled,
                "uptime" to stats.uptimeSeconds
            )
        )
        webSocket?.send(gson.toJson(message))
    }
    
    private fun sendResponse(requestId: String, response: ProxyResponse) {
        val message = GatewayMessage(
            type = "proxy_response",
            requestId = requestId,
            payload = mapOf(
                "statusCode" to response.statusCode,
                "headers" to response.headers,
                "body" to response.bodyBase64,
                "error" to response.error
            )
        )
        webSocket?.send(gson.toJson(message))
    }
    
    private fun createListener() = object : WebSocketListener() {
        override fun onOpen(webSocket: WebSocket, response: Response) {
            // Send node capabilities
            val capabilities = GatewayMessage(
                type = "capabilities",
                payload = mapOf(
                    "version" to BuildConfig.VERSION_NAME,
                    "platform" to "android",
                    "protocols" to listOf("http", "https", "socks5"),
                    "maxConcurrent" to 10
                )
            )
            webSocket.send(gson.toJson(capabilities))
        }
        
        override fun onMessage(webSocket: WebSocket, text: String) {
            scope.launch {
                try {
                    val message = gson.fromJson(text, GatewayMessage::class.java)
                    handleMessage(message)
                } catch (e: Exception) {
                    // Log error
                }
            }
        }
        
        override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
            // Binary message handling for large payloads
        }
        
        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
            webSocket.close(code, reason)
        }
        
        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
            // Attempt reconnection
            scope.launch {
                delay(5000)
                try {
                    requestHandler?.let { connect(it) }
                } catch (e: Exception) {
                    // Log error
                }
            }
        }
    }
    
    private suspend fun handleMessage(message: GatewayMessage) {
        when (message.type) {
            "proxy_request" -> {
                val request = parseProxyRequest(message)
                val response = requestHandler?.invoke(request) ?: ProxyResponse(
                    statusCode = 500,
                    error = "Handler not available"
                )
                sendResponse(message.requestId ?: "", response)
            }
            "config_update" -> {
                // Handle config updates from gateway
            }
            "ping" -> {
                val pong = GatewayMessage(type = "pong")
                webSocket?.send(gson.toJson(pong))
            }
        }
    }
    
    private fun parseProxyRequest(message: GatewayMessage): ProxyRequest {
        val payload = message.payload as? Map<*, *> ?: emptyMap<String, Any>()
        return ProxyRequest(
            id = message.requestId ?: "",
            method = payload["method"] as? String ?: "GET",
            url = payload["url"] as? String ?: "",
            headers = (payload["headers"] as? Map<*, *>)?.mapKeys { it.key.toString() }
                ?.mapValues { it.value.toString() } ?: emptyMap(),
            bodyBase64 = payload["body"] as? String
        )
    }
}

data class GatewayMessage(
    val type: String,
    val requestId: String? = null,
    val payload: Any? = null
)

data class ProxyRequest(
    val id: String,
    val method: String,
    val url: String,
    val headers: Map<String, String>,
    val bodyBase64: String?
)

data class ProxyResponse(
    val statusCode: Int,
    val headers: Map<String, String> = emptyMap(),
    val bodyBase64: String? = null,
    val error: String? = null
)
