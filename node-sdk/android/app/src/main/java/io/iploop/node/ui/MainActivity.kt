package io.iploop.node.ui

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.view.View
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import io.iploop.node.BuildConfig
import io.iploop.node.databinding.ActivityMainBinding
import io.iploop.node.network.*
import io.iploop.node.service.NodeService
import io.iploop.node.service.NodeStats
import io.iploop.node.util.NodePreferences
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch
import java.util.UUID

class MainActivity : AppCompatActivity() {

    private lateinit var binding: ActivityMainBinding
    private lateinit var preferences: NodePreferences
    
    private val notificationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { isGranted ->
        if (isGranted) {
            startNodeService()
        } else {
            Toast.makeText(this, "Notification permission required for background operation", Toast.LENGTH_LONG).show()
        }
    }
    
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)
        
        preferences = NodePreferences(this)
        
        setupUI()
        observeServiceState()
        checkRegistration()
    }
    
    private fun setupUI() {
        binding.btnToggle.setOnClickListener {
            toggleService()
        }
        
        binding.btnSettings.setOnClickListener {
            // Open settings
        }
        
        binding.btnEarnings.setOnClickListener {
            // Open earnings
        }
    }
    
    private fun observeServiceState() {
        lifecycleScope.launch {
            NodeService.isRunning.collectLatest { isRunning ->
                updateUI(isRunning)
            }
        }
        
        lifecycleScope.launch {
            NodeService.stats.collectLatest { stats ->
                updateStats(stats)
            }
        }
        
        lifecycleScope.launch {
            preferences.statsFlow.collectLatest { localStats ->
                binding.tvTotalEarnings.text = String.format("$%.4f", localStats.totalEarnings)
                binding.tvTotalData.text = formatBytes(localStats.totalBytes)
            }
        }
    }
    
    private fun updateUI(isRunning: Boolean) {
        binding.btnToggle.text = if (isRunning) "Stop Sharing" else "Start Sharing"
        binding.btnToggle.setBackgroundColor(
            ContextCompat.getColor(this, 
                if (isRunning) android.R.color.holo_red_light 
                else android.R.color.holo_green_light
            )
        )
        binding.tvStatus.text = if (isRunning) "● Active" else "○ Inactive"
        binding.tvStatus.setTextColor(
            ContextCompat.getColor(this,
                if (isRunning) android.R.color.holo_green_dark
                else android.R.color.darker_gray
            )
        )
        binding.cardLiveStats.visibility = if (isRunning) View.VISIBLE else View.GONE
    }
    
    private fun updateStats(stats: NodeStats) {
        binding.tvSessionData.text = formatBytes(stats.totalBytesTransferred)
        binding.tvSessionRequests.text = "${stats.requestsHandled} requests"
    }
    
    private fun toggleService() {
        if (NodeService.isRunning.value) {
            stopNodeService()
        } else {
            checkPermissionsAndStart()
        }
    }
    
    private fun checkPermissionsAndStart() {
        // Check notification permission (Android 13+)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) 
                != PackageManager.PERMISSION_GRANTED) {
                notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
                return
            }
        }
        
        startNodeService()
    }
    
    private fun startNodeService() {
        val intent = Intent(this, NodeService::class.java).apply {
            action = NodeService.ACTION_START
        }
        
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }
    
    private fun stopNodeService() {
        val intent = Intent(this, NodeService::class.java).apply {
            action = NodeService.ACTION_STOP
        }
        startService(intent)
    }
    
    private fun checkRegistration() {
        lifecycleScope.launch {
            if (!preferences.isRegistered()) {
                registerNode()
            } else {
                binding.tvNodeId.text = "Node: ${preferences.getNodeId()?.take(8)}..."
            }
        }
    }
    
    private suspend fun registerNode() {
        binding.progressBar.visibility = View.VISIBLE
        binding.btnToggle.isEnabled = false
        
        try {
            // Get or generate device ID
            var deviceId = preferences.getDeviceId()
            if (deviceId == null) {
                deviceId = Settings.Secure.getString(contentResolver, Settings.Secure.ANDROID_ID)
                    ?: UUID.randomUUID().toString()
                preferences.setDeviceId(deviceId)
            }
            
            val request = NodeRegistrationRequest(
                deviceId = deviceId,
                deviceInfo = DeviceInfo(
                    model = Build.MODEL,
                    manufacturer = Build.MANUFACTURER,
                    osVersion = Build.VERSION.RELEASE,
                    sdkVersion = Build.VERSION.SDK_INT,
                    appVersion = BuildConfig.VERSION_NAME,
                    carrier = null, // Would need TelephonyManager
                    connectionType = getConnectionType(),
                    country = null, // Would need location
                    city = null
                )
            )
            
            val response = ApiClient.api.registerNode(request)
            
            if (response.isSuccessful && response.body()?.success == true) {
                val body = response.body()!!
                preferences.setNodeId(body.nodeId!!)
                preferences.setAuthToken(body.token!!)
                binding.tvNodeId.text = "Node: ${body.nodeId.take(8)}..."
                Toast.makeText(this, "Node registered successfully!", Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(this, "Registration failed: ${response.body()?.message}", Toast.LENGTH_LONG).show()
            }
        } catch (e: Exception) {
            Toast.makeText(this, "Registration error: ${e.message}", Toast.LENGTH_LONG).show()
        } finally {
            binding.progressBar.visibility = View.GONE
            binding.btnToggle.isEnabled = true
        }
    }
    
    private fun getConnectionType(): String {
        val cm = getSystemService(CONNECTIVITY_SERVICE) as ConnectivityManager
        val network = cm.activeNetwork ?: return "unknown"
        val capabilities = cm.getNetworkCapabilities(network) ?: return "unknown"
        
        return when {
            capabilities.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> "wifi"
            capabilities.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> "cellular"
            capabilities.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> "ethernet"
            else -> "unknown"
        }
    }
    
    private fun formatBytes(bytes: Long): String {
        return when {
            bytes >= 1_073_741_824 -> String.format("%.2f GB", bytes / 1_073_741_824.0)
            bytes >= 1_048_576 -> String.format("%.2f MB", bytes / 1_048_576.0)
            bytes >= 1024 -> String.format("%.2f KB", bytes / 1024.0)
            else -> "$bytes B"
        }
    }
}
