package com.example.iploop.demo

import android.os.Bundle
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import com.iploop.sdk.IPLoopConfig
import com.iploop.sdk.IPLoopSDK
import com.iploop.sdk.SDKStatus
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.launch

/**
 * Example integration of IPLoop SDK
 * Demonstrates basic usage patterns
 */
class MainActivity : AppCompatActivity() {
    
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        
        initializeIPLoop()
        setupUI()
        observeStatus()
    }
    
    private fun initializeIPLoop() {
        // Initialize SDK with custom configuration
        val config = IPLoopConfig.Builder()
            .setWifiOnly(true)
            .setMaxBandwidthMB(150)
            .setMinBatteryLevel(25)
            .setDebugMode(BuildConfig.DEBUG)
            .build()
            
        IPLoopSDK.init(
            context = this,
            sdkKey = "demo_sdk_key_replace_with_real",
            config = config
        )
        
        showToast("IPLoop SDK initialized")
    }
    
    private fun setupUI() {
        findViewById<Button>(R.id.btnStart).setOnClickListener {
            startIPLoop()
        }
        
        findViewById<Button>(R.id.btnStop).setOnClickListener {
            stopIPLoop()
        }
        
        findViewById<Button>(R.id.btnConsent).setOnClickListener {
            IPLoopSDK.showConsentDialog(this)
        }
        
        findViewById<Button>(R.id.btnStatus).setOnClickListener {
            showStatus()
        }
        
        findViewById<Button>(R.id.btnUsage).setOnClickListener {
            showUsage()
        }
    }
    
    private fun observeStatus() {
        lifecycleScope.launch {
            IPLoopSDK.status.collect { status ->
                updateStatusDisplay(status)
            }
        }
    }
    
    private fun startIPLoop() {
        // Check consent first
        if (!IPLoopSDK.hasConsent()) {
            showToast("User consent required")
            IPLoopSDK.showConsentDialog(this)
            return
        }
        
        lifecycleScope.launch {
            try {
                val result = IPLoopSDK.start()
                if (result.isSuccess) {
                    showToast("IPLoop started successfully")
                } else {
                    showToast("Failed to start: ${result.exceptionOrNull()?.message}")
                }
            } catch (e: Exception) {
                showToast("Error: ${e.message}")
            }
        }
    }
    
    private fun stopIPLoop() {
        lifecycleScope.launch {
            try {
                IPLoopSDK.stop()
                showToast("IPLoop stopped")
            } catch (e: Exception) {
                showToast("Error stopping: ${e.message}")
            }
        }
    }
    
    private fun showStatus() {
        val status = IPLoopSDK.getStatus()
        val isRunning = IPLoopSDK.isRunning()
        val hasConsent = IPLoopSDK.hasConsent()
        
        val statusText = """
            Status: $status
            Running: $isRunning
            Consent: $hasConsent
        """.trimIndent()
        
        showToast(statusText)
    }
    
    private fun showUsage() {
        val usage = IPLoopSDK.getBandwidthUsage()
        if (usage != null) {
            val usageText = """
                Uploaded: ${usage.uploadedMB} MB
                Downloaded: ${usage.downloadedMB} MB
                Total: ${usage.totalMB} MB
                Sessions: ${usage.sessionsCount}
            """.trimIndent()
            showToast(usageText)
        } else {
            showToast("No usage data available")
        }
    }
    
    private fun updateStatusDisplay(status: SDKStatus) {
        val statusText = findViewById<TextView>(R.id.tvStatus)
        statusText?.text = "Status: $status"
        
        // Update UI based on status
        val btnStart = findViewById<Button>(R.id.btnStart)
        val btnStop = findViewById<Button>(R.id.btnStop)
        
        when (status) {
            SDKStatus.RUNNING -> {
                btnStart?.isEnabled = false
                btnStop?.isEnabled = true
            }
            SDKStatus.STOPPED, SDKStatus.INITIALIZED -> {
                btnStart?.isEnabled = true
                btnStop?.isEnabled = false
            }
            SDKStatus.CONSENT_REQUIRED -> {
                btnStart?.isEnabled = false
                btnStop?.isEnabled = false
                showToast("User consent required")
            }
            else -> {
                btnStart?.isEnabled = false
                btnStop?.isEnabled = true
            }
        }
    }
    
    private fun showToast(message: String) {
        Toast.makeText(this, message, Toast.LENGTH_SHORT).show()
    }
}