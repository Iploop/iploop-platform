package com.iploop.sdk.internal

import kotlinx.coroutines.*
import org.json.JSONObject
import java.io.*
import java.net.*
import java.nio.charset.StandardCharsets
import javax.net.ssl.HttpsURLConnection

/**
 * Executes HTTP requests on behalf of the proxy backend
 * These requests go out from the device's IP address
 */
class HttpExecutor {
    
    companion object {
        private const val DEFAULT_TIMEOUT_MS = 30000
        private const val MAX_RESPONSE_SIZE = 10 * 1024 * 1024 // 10MB
    }
    
    /**
     * Execute an HTTP request and return the response
     */
    suspend fun execute(
        targetUrl: String,
        method: String = "GET",
        headers: Map<String, String> = emptyMap(),
        body: String? = null,
        timeoutMs: Int = DEFAULT_TIMEOUT_MS
    ): HttpResult = withContext(Dispatchers.IO) {
        val startTime = System.currentTimeMillis()
        
        try {
            val url = URL(targetUrl)
            val connection = url.openConnection() as HttpURLConnection
            
            // Configure connection
            connection.requestMethod = method
            connection.connectTimeout = timeoutMs
            connection.readTimeout = timeoutMs
            connection.instanceFollowRedirects = true
            
            // Set headers
            headers.forEach { (key, value) ->
                connection.setRequestProperty(key, value)
            }
            
            // Set default headers if not provided
            if (!headers.containsKey("User-Agent")) {
                connection.setRequestProperty("User-Agent", "Mozilla/5.0 (Linux; Android) IPLoop/1.0")
            }
            
            // Send body if present
            if (body != null && (method == "POST" || method == "PUT" || method == "PATCH")) {
                connection.doOutput = true
                connection.outputStream.use { os ->
                    os.write(body.toByteArray(StandardCharsets.UTF_8))
                }
            }
            
            // Get response
            val statusCode = connection.responseCode
            val responseHeaders = mutableMapOf<String, String>()
            connection.headerFields.forEach { (key, values) ->
                if (key != null && values.isNotEmpty()) {
                    responseHeaders[key] = values.joinToString(", ")
                }
            }
            
            // Read response body
            val inputStream = if (statusCode >= 400) {
                connection.errorStream
            } else {
                connection.inputStream
            }
            
            val responseBody = inputStream?.use { stream ->
                readStream(stream, MAX_RESPONSE_SIZE)
            } ?: ""
            
            connection.disconnect()
            
            val latencyMs = System.currentTimeMillis() - startTime
            
            IPLoopLogger.d("HttpExecutor", "$method $targetUrl -> $statusCode (${latencyMs}ms, ${responseBody.length} bytes)")
            
            HttpResult(
                success = true,
                statusCode = statusCode,
                headers = responseHeaders,
                body = responseBody,
                latencyMs = latencyMs,
                startedAt = startTime
            )
            
        } catch (e: SocketTimeoutException) {
            IPLoopLogger.e("HttpExecutor", "Timeout: $targetUrl", e)
            HttpResult(
                success = false,
                statusCode = 0,
                error = "Connection timeout",
                latencyMs = System.currentTimeMillis() - startTime,
                startedAt = startTime
            )
        } catch (e: UnknownHostException) {
            IPLoopLogger.e("HttpExecutor", "Unknown host: $targetUrl", e)
            HttpResult(
                success = false,
                statusCode = 0,
                error = "Unknown host: ${e.message}",
                latencyMs = System.currentTimeMillis() - startTime,
                startedAt = startTime
            )
        } catch (e: Exception) {
            IPLoopLogger.e("HttpExecutor", "Request failed: $targetUrl", e)
            HttpResult(
                success = false,
                statusCode = 0,
                error = e.message ?: "Unknown error",
                latencyMs = System.currentTimeMillis() - startTime,
                startedAt = startTime
            )
        }
    }
    
    /**
     * Read stream with size limit
     */
    private fun readStream(inputStream: InputStream, maxSize: Int): String {
        val result = ByteArrayOutputStream()
        val buffer = ByteArray(8192)
        var totalRead = 0
        
        while (true) {
            val bytesRead = inputStream.read(buffer)
            if (bytesRead == -1) break
            
            totalRead += bytesRead
            if (totalRead > maxSize) {
                IPLoopLogger.w("HttpExecutor", "Response truncated at $maxSize bytes")
                break
            }
            
            result.write(buffer, 0, bytesRead)
        }
        
        return result.toString(StandardCharsets.UTF_8.name())
    }
}

/**
 * Result of an HTTP request execution
 */
data class HttpResult(
    val success: Boolean,
    val statusCode: Int,
    val headers: Map<String, String> = emptyMap(),
    val body: String = "",
    val error: String? = null,
    val latencyMs: Long = 0,
    val startedAt: Long = 0
) {
    /**
     * Convert to JSON for sending back to server
     */
    fun toResponseJson(requestId: String): JSONObject {
        return if (success) {
            JSONObject().apply {
                put("type", "proxy_response")
                put("request_id", requestId)
                put("status_code", statusCode)
                put("headers", JSONObject(headers))
                put("body", body)
                put("latency_ms", latencyMs)
                put("started_at", startedAt)
            }
        } else {
            JSONObject().apply {
                put("type", "proxy_error")
                put("request_id", requestId)
                put("error", error ?: "Unknown error")
                put("latency_ms", latencyMs)
                put("started_at", startedAt)
            }
        }
    }
}
