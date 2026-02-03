package com.iploop.sdk.internal

import android.content.Context
import com.iploop.sdk.IPLoopConfig
import kotlinx.coroutines.*
import org.json.JSONObject
import java.io.*
import java.net.*
import java.nio.charset.StandardCharsets
import java.util.concurrent.atomic.AtomicBoolean
import java.util.regex.Pattern

/**
 * Handles HTTP and SOCKS5 proxy traffic relay
 * Manages incoming proxy requests and routes them through device IP
 */
class TrafficRelay(
    private val context: Context,
    private val config: IPLoopConfig
) {
    private val isRunning = AtomicBoolean(false)
    private val relayScope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    
    private var httpProxyServer: ServerSocket? = null
    private var socks5ProxyServer: ServerSocket? = null
    private var tunnelManager: TunnelManager? = null
    
    // Relay statistics
    private var requestCount = 0L
    private var errorCount = 0L
    private var lastRequestTime = 0L
    
    companion object {
        private const val HTTP_PROXY_PORT = 8080
        private const val SOCKS5_PROXY_PORT = 1080
        private const val MAX_CONCURRENT_REQUESTS = 20
        
        // HTTP method pattern
        private val HTTP_METHOD_PATTERN = Pattern.compile("^(GET|POST|PUT|DELETE|HEAD|OPTIONS|PATCH|CONNECT)\\s")
    }
    
    /**
     * Start the traffic relay
     */
    fun start() {
        if (isRunning.get()) return
        
        IPLoopLogger.i("TrafficRelay", "Starting traffic relay")
        isRunning.set(true)
        
        tunnelManager = TunnelManager(context, config).apply { start() }
        
        // Start HTTP and SOCKS5 proxy servers
        startHttpProxyServer()
        startSocks5ProxyServer()
    }
    
    /**
     * Stop the traffic relay
     */
    fun stop() {
        if (!isRunning.get()) return
        
        IPLoopLogger.i("TrafficRelay", "Stopping traffic relay")
        isRunning.set(false)
        
        // Stop servers
        try {
            httpProxyServer?.close()
            socks5ProxyServer?.close()
        } catch (e: Exception) {
            IPLoopLogger.e("TrafficRelay", "Error stopping servers", e)
        }
        
        // Stop tunnel manager
        tunnelManager?.stop()
        
        // Cancel coroutines
        relayScope.cancel()
    }
    
    /**
     * Start HTTP proxy server
     */
    private fun startHttpProxyServer() {
        relayScope.launch {
            try {
                httpProxyServer = ServerSocket(HTTP_PROXY_PORT, 50, InetAddress.getLoopbackAddress())
                IPLoopLogger.i("TrafficRelay", "HTTP proxy listening on port $HTTP_PROXY_PORT")
                
                while (isRunning.get()) {
                    try {
                        val clientSocket = httpProxyServer!!.accept()
                        launch { handleHttpClient(clientSocket) }
                    } catch (e: SocketException) {
                        if (isRunning.get()) {
                            IPLoopLogger.e("TrafficRelay", "HTTP server socket error", e)
                        }
                    }
                }
            } catch (e: Exception) {
                IPLoopLogger.e("TrafficRelay", "HTTP proxy server failed", e)
            }
        }
    }
    
    /**
     * Start SOCKS5 proxy server
     */
    private fun startSocks5ProxyServer() {
        relayScope.launch {
            try {
                socks5ProxyServer = ServerSocket(SOCKS5_PROXY_PORT, 50, InetAddress.getLoopbackAddress())
                IPLoopLogger.i("TrafficRelay", "SOCKS5 proxy listening on port $SOCKS5_PROXY_PORT")
                
                while (isRunning.get()) {
                    try {
                        val clientSocket = socks5ProxyServer!!.accept()
                        launch { handleSocks5Client(clientSocket) }
                    } catch (e: SocketException) {
                        if (isRunning.get()) {
                            IPLoopLogger.e("TrafficRelay", "SOCKS5 server socket error", e)
                        }
                    }
                }
            } catch (e: Exception) {
                IPLoopLogger.e("TrafficRelay", "SOCKS5 proxy server failed", e)
            }
        }
    }
    
    /**
     * Handle HTTP proxy client connection
     */
    private suspend fun handleHttpClient(clientSocket: Socket) = withContext(Dispatchers.IO) {
        val sessionId = "http_${System.currentTimeMillis()}_${clientSocket.hashCode()}"
        
        try {
            clientSocket.soTimeout = config.trafficTimeoutSec * 1000
            
            val input = BufferedReader(InputStreamReader(clientSocket.getInputStream(), StandardCharsets.UTF_8))
            val output = clientSocket.getOutputStream()
            
            // Read HTTP request line
            val requestLine = input.readLine()
            if (requestLine == null || !HTTP_METHOD_PATTERN.matcher(requestLine).find()) {
                sendHttpError(output, 400, "Bad Request")
                return@withContext
            }
            
            val parts = requestLine.split(" ")
            if (parts.size < 3) {
                sendHttpError(output, 400, "Bad Request")
                return@withContext
            }
            
            val method = parts[0]
            val url = parts[1]
            val protocol = parts[2]
            
            IPLoopLogger.d("TrafficRelay", "HTTP request: $method $url")
            
            when (method) {
                "CONNECT" -> handleHttpConnect(clientSocket, url, sessionId)
                else -> handleHttpRequest(clientSocket, requestLine, input, sessionId)
            }
            
            requestCount++
            lastRequestTime = System.currentTimeMillis()
            
        } catch (e: Exception) {
            IPLoopLogger.e("TrafficRelay", "HTTP client error", e)
            errorCount++
        } finally {
            try {
                clientSocket.close()
            } catch (e: Exception) {
                // Ignore
            }
        }
    }
    
    /**
     * Handle HTTP CONNECT method (for HTTPS tunneling)
     */
    private suspend fun handleHttpConnect(clientSocket: Socket, hostPort: String, sessionId: String) {
        val parts = hostPort.split(":")
        if (parts.size != 2) {
            sendHttpError(clientSocket.getOutputStream(), 400, "Bad Request")
            return
        }
        
        val host = parts[0]
        val port = parts[1].toIntOrNull() ?: 80
        
        // Create tunnel
        val tunnelResult = tunnelManager!!.createTunnel(sessionId, host, port)
        if (tunnelResult.isFailure) {
            sendHttpError(clientSocket.getOutputStream(), 502, "Bad Gateway")
            return
        }
        
        val tunnel = tunnelResult.getOrNull()!!
        val connectResult = tunnel.connect()
        
        if (connectResult.isSuccess) {
            // Send success response
            val response = "HTTP/1.1 200 Connection established\r\n\r\n"
            clientSocket.getOutputStream().write(response.toByteArray())
            
            // Start bidirectional relay
            val job1 = tunnel.relayClientToTarget(clientSocket.getInputStream())
            val job2 = tunnel.relayTargetToClient(clientSocket.getOutputStream())
            
            // Wait for either job to complete
            job1.join()
            job2.cancel()
            
        } else {
            sendHttpError(clientSocket.getOutputStream(), 502, "Bad Gateway")
        }
        
        tunnelManager!!.closeTunnel(sessionId)
    }
    
    /**
     * Handle regular HTTP request
     */
    private suspend fun handleHttpRequest(
        clientSocket: Socket,
        requestLine: String,
        input: BufferedReader,
        sessionId: String
    ) {
        try {
            // Parse request URL
            val parts = requestLine.split(" ")
            val urlStr = parts[1]
            val url = URL(if (urlStr.startsWith("http")) urlStr else "http://$urlStr")
            
            val host = url.host
            val port = if (url.port != -1) url.port else 80
            
            // Create tunnel
            val tunnelResult = tunnelManager!!.createTunnel(sessionId, host, port)
            if (tunnelResult.isFailure) {
                sendHttpError(clientSocket.getOutputStream(), 502, "Bad Gateway")
                return
            }
            
            val tunnel = tunnelResult.getOrNull()!!
            val connectResult = tunnel.connect()
            
            if (connectResult.isSuccess) {
                // Forward the request
                val targetSocket = tunnel.getTargetSocket()
                if (targetSocket != null) {
                    val targetOutput = targetSocket.getOutputStream()
                    // Send request line
                    val path = if (url.path.isEmpty()) "/" else url.path + (url.query?.let { "?$it" } ?: "")
                    val modifiedRequestLine = "${parts[0]} $path ${parts[2]}\r\n"
                    targetOutput.write(modifiedRequestLine.toByteArray())
                    
                    // Forward headers
                    var line: String?
                    while (input.readLine().also { line = it } != null) {
                        if (line!!.isEmpty()) break // End of headers
                        targetOutput.write("$line\r\n".toByteArray())
                    }
                    targetOutput.write("\r\n".toByteArray())
                    
                    // Start bidirectional relay for response
                    val job = tunnel.relayTargetToClient(clientSocket.getOutputStream())
                    job.join()
                }
            } else {
                sendHttpError(clientSocket.getOutputStream(), 502, "Bad Gateway")
            }
            
        } catch (e: Exception) {
            IPLoopLogger.e("TrafficRelay", "HTTP request handling error", e)
            sendHttpError(clientSocket.getOutputStream(), 500, "Internal Server Error")
        } finally {
            tunnelManager!!.closeTunnel(sessionId)
        }
    }
    
    /**
     * Handle SOCKS5 proxy client connection
     */
    private suspend fun handleSocks5Client(clientSocket: Socket) = withContext(Dispatchers.IO) {
        val sessionId = "socks5_${System.currentTimeMillis()}_${clientSocket.hashCode()}"
        
        try {
            clientSocket.soTimeout = config.trafficTimeoutSec * 1000
            
            val input = clientSocket.getInputStream()
            val output = clientSocket.getOutputStream()
            
            // SOCKS5 authentication
            if (!socks5Handshake(input, output)) {
                return@withContext
            }
            
            // SOCKS5 connection request
            val connectionInfo = socks5ConnectionRequest(input, output)
            if (connectionInfo == null) {
                return@withContext
            }
            
            val (host, port) = connectionInfo
            IPLoopLogger.d("TrafficRelay", "SOCKS5 request: $host:$port")
            
            // Create tunnel
            val tunnelResult = tunnelManager!!.createTunnel(sessionId, host, port)
            if (tunnelResult.isFailure) {
                socks5SendError(output, 0x01) // General failure
                return@withContext
            }
            
            val tunnel = tunnelResult.getOrNull()!!
            val connectResult = tunnel.connect()
            
            if (connectResult.isSuccess) {
                // Send success response
                socks5SendSuccess(output)
                
                // Start bidirectional relay
                val job1 = tunnel.relayClientToTarget(input)
                val job2 = tunnel.relayTargetToClient(output)
                
                // Wait for completion
                job1.join()
                job2.cancel()
                
            } else {
                socks5SendError(output, 0x04) // Host unreachable
            }
            
            requestCount++
            lastRequestTime = System.currentTimeMillis()
            
        } catch (e: Exception) {
            IPLoopLogger.e("TrafficRelay", "SOCKS5 client error", e)
            errorCount++
        } finally {
            try {
                clientSocket.close()
            } catch (e: Exception) {
                // Ignore
            }
            tunnelManager!!.closeTunnel(sessionId)
        }
    }
    
    /**
     * SOCKS5 handshake
     */
    private fun socks5Handshake(input: InputStream, output: OutputStream): Boolean {
        try {
            // Read method selection request
            val version = input.read()
            if (version != 0x05) return false // Not SOCKS5
            
            val methodCount = input.read()
            val methods = ByteArray(methodCount)
            input.read(methods)
            
            // We support only "no authentication" (0x00)
            if (0x00 in methods) {
                output.write(byteArrayOf(0x05, 0x00)) // SOCKS5, no auth
                return true
            } else {
                output.write(byteArrayOf(0x05, 0xFF.toByte())) // No acceptable methods
                return false
            }
        } catch (e: Exception) {
            return false
        }
    }
    
    /**
     * SOCKS5 connection request
     */
    private fun socks5ConnectionRequest(input: InputStream, output: OutputStream): Pair<String, Int>? {
        try {
            val version = input.read()
            val command = input.read()
            val reserved = input.read()
            val addressType = input.read()
            
            if (version != 0x05 || command != 0x01) { // Not SOCKS5 or not CONNECT
                socks5SendError(output, 0x07) // Command not supported
                return null
            }
            
            val host = when (addressType) {
                0x01 -> { // IPv4
                    val addr = ByteArray(4)
                    input.read(addr)
                    InetAddress.getByAddress(addr).hostAddress
                }
                0x03 -> { // Domain name
                    val nameLength = input.read()
                    val nameBytes = ByteArray(nameLength)
                    input.read(nameBytes)
                    String(nameBytes, StandardCharsets.UTF_8)
                }
                0x04 -> { // IPv6
                    val addr = ByteArray(16)
                    input.read(addr)
                    InetAddress.getByAddress(addr).hostAddress
                }
                else -> {
                    socks5SendError(output, 0x08) // Address type not supported
                    return null
                }
            }
            
            // Read port
            val portBytes = ByteArray(2)
            input.read(portBytes)
            val port = ((portBytes[0].toInt() and 0xFF) shl 8) or (portBytes[1].toInt() and 0xFF)
            
            return host to port
            
        } catch (e: Exception) {
            socks5SendError(output, 0x01) // General failure
            return null
        }
    }
    
    /**
     * Send SOCKS5 success response
     */
    private fun socks5SendSuccess(output: OutputStream) {
        val response = byteArrayOf(
            0x05, 0x00, 0x00, 0x01, // SOCKS5, success, reserved, IPv4
            0x00, 0x00, 0x00, 0x00, // Bind IP (0.0.0.0)
            0x00, 0x00              // Bind port (0)
        )
        output.write(response)
    }
    
    /**
     * Send SOCKS5 error response
     */
    private fun socks5SendError(output: OutputStream, errorCode: Int) {
        val response = byteArrayOf(
            0x05, errorCode.toByte(), 0x00, 0x01,
            0x00, 0x00, 0x00, 0x00,
            0x00, 0x00
        )
        output.write(response)
    }
    
    /**
     * Send HTTP error response
     */
    private fun sendHttpError(output: OutputStream, code: Int, message: String) {
        val response = """
            HTTP/1.1 $code $message
            Content-Type: text/plain
            Connection: close
            
            $code $message
        """.trimIndent().replace("\n", "\r\n")
        
        output.write(response.toByteArray())
    }
    
    /**
     * Get relay statistics
     */
    fun getStats(): Map<String, Any> {
        return mapOf(
            "requests_count" to requestCount,
            "errors_count" to errorCount,
            "active_tunnels" to (tunnelManager?.getActiveTunnelCount() ?: 0),
            "bytes_transferred" to (tunnelManager?.getBytesTransferred() ?: 0),
            "last_request_time" to lastRequestTime,
            "uptime_ms" to (System.currentTimeMillis() - lastRequestTime)
        )
    }
}