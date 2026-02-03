package io.iploop.node.network

import android.util.Base64
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit

/**
 * Handles proxy requests by executing HTTP requests through this device's network
 */
class ProxyHandler {
    
    private val client = OkHttpClient.Builder()
        .connectTimeout(30, TimeUnit.SECONDS)
        .readTimeout(60, TimeUnit.SECONDS)
        .writeTimeout(60, TimeUnit.SECONDS)
        .followRedirects(true)
        .followSslRedirects(true)
        .build()
    
    suspend fun handleRequest(
        request: ProxyRequest,
        onBytesTransferred: (Long) -> Unit
    ): ProxyResponse = withContext(Dispatchers.IO) {
        try {
            val okRequest = buildOkHttpRequest(request)
            val response = client.newCall(okRequest).execute()
            
            response.use { resp ->
                val bodyBytes = resp.body?.bytes() ?: ByteArray(0)
                val bytesTransferred = bodyBytes.size.toLong() + 
                    request.bodyBase64?.let { Base64.decode(it, Base64.DEFAULT).size }?.toLong().orZero()
                
                onBytesTransferred(bytesTransferred)
                
                ProxyResponse(
                    statusCode = resp.code,
                    headers = resp.headers.toMap(),
                    bodyBase64 = Base64.encodeToString(bodyBytes, Base64.NO_WRAP)
                )
            }
        } catch (e: Exception) {
            ProxyResponse(
                statusCode = 502,
                error = e.message ?: "Unknown error"
            )
        }
    }
    
    private fun buildOkHttpRequest(request: ProxyRequest): Request {
        val builder = Request.Builder()
            .url(request.url)
        
        // Add headers
        request.headers.forEach { (key, value) ->
            // Skip certain headers that OkHttp manages
            if (!key.equals("Host", ignoreCase = true) &&
                !key.equals("Content-Length", ignoreCase = true)) {
                builder.addHeader(key, value)
            }
        }
        
        // Set method and body
        val body = request.bodyBase64?.let {
            val bytes = Base64.decode(it, Base64.DEFAULT)
            val contentType = request.headers["Content-Type"]?.toMediaTypeOrNull()
            bytes.toRequestBody(contentType)
        }
        
        when (request.method.uppercase()) {
            "GET" -> builder.get()
            "POST" -> builder.post(body ?: "".toRequestBody())
            "PUT" -> builder.put(body ?: "".toRequestBody())
            "DELETE" -> {
                if (body != null) builder.delete(body) else builder.delete()
            }
            "PATCH" -> builder.patch(body ?: "".toRequestBody())
            "HEAD" -> builder.head()
            "OPTIONS" -> builder.method("OPTIONS", null)
            else -> builder.method(request.method, body)
        }
        
        return builder.build()
    }
    
    private fun Headers.toMap(): Map<String, String> {
        val map = mutableMapOf<String, String>()
        for (i in 0 until size) {
            map[name(i)] = value(i)
        }
        return map
    }
    
    private fun Long?.orZero(): Long = this ?: 0L
}
