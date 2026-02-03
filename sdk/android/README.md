# IPLoop Android SDK

The IPLoop Android SDK enables Android applications to participate in the IPLoop proxy platform, turning devices into secure proxy nodes while maintaining privacy and compliance.

## Features

- **Secure Proxy Traffic Routing** - HTTP and SOCKS5 proxy support
- **Privacy-First Design** - Clear consent flows, WiFi-only by default
- **Battery & Data Friendly** - Configurable limits and smart power management
- **Compliance Ready** - GDPR compliant with proper user controls
- **Production Ready** - Real WebSocket communication with IPLoop backend
- **Easy Integration** - Simple API with comprehensive configuration options

## Quick Start

### 1. Add Dependency

Add to your app's `build.gradle`:

```gradle
dependencies {
    implementation 'com.iploop:android-sdk:1.0.0'
}
```

### 2. Add Permissions

The SDK automatically adds required permissions via manifest merger:

```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
<uses-permission android:name="android.permission.ACCESS_WIFI_STATE" />
<uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
```

### 3. Initialize SDK

In your `Application` class:

```kotlin
class MyApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        
        // Initialize IPLoop SDK
        IPLoopSDK.init(
            context = this,
            sdkKey = "your_sdk_key_here",
            config = IPLoopConfig.Builder()
                .setWifiOnly(true)
                .setMaxBandwidthMB(100)
                .build()
        )
    }
}
```

### 4. Handle User Consent

```kotlin
// Check if user has given consent
if (!IPLoopSDK.hasConsent()) {
    // Show consent dialog
    IPLoopSDK.showConsentDialog(this@MainActivity)
}

// Or set consent programmatically
IPLoopSDK.setConsentGiven(true)
```

### 5. Start/Stop Service

```kotlin
// Start proxy service
lifecycleScope.launch {
    val result = IPLoopSDK.start()
    if (result.isSuccess) {
        println("IPLoop started successfully")
    } else {
        println("Failed to start: ${result.exceptionOrNull()}")
    }
}

// Stop proxy service
lifecycleScope.launch {
    IPLoopSDK.stop()
}
```

## Configuration Options

### IPLoopConfig.Builder Methods

| Method | Description | Default |
|--------|-------------|---------|
| `setWifiOnly(boolean)` | Restrict to WiFi connections only | `true` |
| `setMaxBandwidthMB(int)` | Daily bandwidth limit in MB | `100` |
| `setMaxSessionBandwidthMB(int)` | Per-session bandwidth limit | `10` |
| `setChargingOnly(boolean)` | Only operate when charging | `false` |
| `setMinBatteryLevel(int)` | Minimum battery level (0-100) | `20` |
| `setRegistrationUrl(String)` | WebSocket registration URL | IPLoop production |
| `setHeartbeatIntervalSec(int)` | Heartbeat interval in seconds | `30` |
| `setShareLocation(boolean)` | Enable location sharing | `false` |
| `setDebugMode(boolean)` | Enable debug logging | `false` |
| `setMaxConcurrentConnections(int)` | Max concurrent proxy connections | `5` |

### Preset Configurations

```kotlin
// Privacy-friendly configuration
val privacyConfig = IPLoopConfig.createPrivacyFriendly()

// High-performance configuration  
val performanceConfig = IPLoopConfig.createHighPerformance()

// Custom configuration
val customConfig = IPLoopConfig.Builder()
    .setWifiOnly(true)
    .setMaxBandwidthMB(200)
    .setChargingOnly(false)
    .setMinBatteryLevel(30)
    .setMaxConcurrentConnections(8)
    .build()
```

## Monitoring & Analytics

### Check Status

```kotlin
// Get current SDK status
val status = IPLoopSDK.getStatus()
when (status) {
    SDKStatus.STOPPED -> println("SDK is stopped")
    SDKStatus.RUNNING -> println("SDK is active")
    SDKStatus.CONSENT_REQUIRED -> println("User consent needed")
    SDKStatus.WAITING_WIFI -> println("Waiting for WiFi")
    SDKStatus.ERROR -> println("SDK error occurred")
}

// Check if running
val isRunning = IPLoopSDK.isRunning()
```

### Bandwidth Usage

```kotlin
// Get bandwidth statistics
val usage = IPLoopSDK.getBandwidthUsage()
usage?.let {
    println("Uploaded: ${it.uploadedMB} MB")
    println("Downloaded: ${it.downloadedMB} MB") 
    println("Total: ${it.totalMB} MB")
    println("Sessions: ${it.sessionsCount}")
}
```

### Device Information

```kotlin
// Get device/network info
val deviceInfo = IPLoopSDK.getDeviceInfo()
println("Device: ${deviceInfo["device_model"]}")
println("Connection: ${deviceInfo["connection_type"]}")
println("IP: ${deviceInfo["local_ip"]}")
```

## Status Flow Monitoring

```kotlin
// Observe SDK status changes
lifecycleScope.launch {
    IPLoopSDK.status.collect { status ->
        updateUI(status)
    }
}
```

## Privacy & Compliance

### Consent Management

The SDK provides GDPR-compliant consent management:

```kotlin
// Show detailed consent dialog
IPLoopSDK.showConsentDialog(context)

// Check consent status
val hasConsent = IPLoopSDK.hasConsent()

// Programmatic consent (after user action)
IPLoopSDK.setConsentGiven(true)  // User accepted
IPLoopSDK.setConsentGiven(false) // User declined
```

### Privacy Features

- **WiFi-only by default** - No cellular data usage without explicit consent
- **Battery-friendly** - Respects battery level and charging status
- **Bandwidth limits** - Configurable daily and session limits
- **Location optional** - Location sharing is opt-in only
- **Transparent operation** - Clear notifications and status indicators
- **Easy opt-out** - Users can disable anytime

## Integration Examples

### Basic Integration

```kotlin
class MainActivity : AppCompatActivity() {
    
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        
        // Initialize if not already done
        if (!IPLoopSDK.isInitialized()) {
            IPLoopSDK.init(this, "your_sdk_key", IPLoopConfig.createDefault())
        }
        
        setupUI()
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
    }
    
    private fun startIPLoop() {
        if (!IPLoopSDK.hasConsent()) {
            IPLoopSDK.showConsentDialog(this)
            return
        }
        
        lifecycleScope.launch {
            try {
                IPLoopSDK.start()
                showToast("IPLoop started")
            } catch (e: Exception) {
                showToast("Failed to start: ${e.message}")
            }
        }
    }
    
    private fun stopIPLoop() {
        lifecycleScope.launch {
            IPLoopSDK.stop()
            showToast("IPLoop stopped")
        }
    }
}
```

### Advanced Integration with Settings

```kotlin
class IPLoopSettingsFragment : PreferenceFragmentCompat() {
    
    override fun onCreatePreferences(savedInstanceState: Bundle?, rootKey: String?) {
        setPreferencesFromResource(R.xml.iploop_preferences, rootKey)
        
        setupPreferences()
    }
    
    private fun setupPreferences() {
        // Enable/Disable toggle
        findPreference<SwitchPreference>("iploop_enabled")?.apply {
            isChecked = IPLoopSDK.isRunning()
            setOnPreferenceChangeListener { _, newValue ->
                lifecycleScope.launch {
                    if (newValue as Boolean) {
                        IPLoopSDK.start()
                    } else {
                        IPLoopSDK.stop()
                    }
                }
                true
            }
        }
        
        // Bandwidth limit
        findPreference<SeekBarPreference>("bandwidth_limit")?.apply {
            value = getCurrentBandwidthLimit()
            setOnPreferenceChangeListener { _, newValue ->
                updateBandwidthLimit(newValue as Int)
                true
            }
        }
        
        // WiFi only
        findPreference<SwitchPreference>("wifi_only")?.apply {
            setOnPreferenceChangeListener { _, newValue ->
                updateWiFiOnlySetting(newValue as Boolean)
                true
            }
        }
    }
}
```

## Testing

### Debug Configuration

```kotlin
val debugConfig = IPLoopConfig.Builder()
    .setDebugMode(true)
    .setRegistrationUrl("ws://localhost:8080/ws") // Local test server
    .setMaxBandwidthMB(10) // Small limit for testing
    .setHeartbeatIntervalSec(10) // Faster heartbeat
    .build()
    
IPLoopSDK.init(context, "test_key", debugConfig)
```

### Mock Backend

For testing without the real IPLoop backend, you can run a local WebSocket server:

```bash
# Example using wscat
npm install -g wscat
wscat -l 8080

# The SDK will connect and send registration messages
```

## Error Handling

```kotlin
lifecycleScope.launch {
    val result = IPLoopSDK.start()
    
    if (result.isFailure) {
        when (val exception = result.exceptionOrNull()) {
            is IllegalStateException -> {
                // SDK not initialized or consent required
                showConsentDialog()
            }
            is IOException -> {
                // Network connectivity issues
                showRetryDialog()
            }
            else -> {
                // Other errors
                showErrorDialog(exception?.message)
            }
        }
    }
}
```

## Performance Considerations

### Battery Optimization

- The SDK automatically handles battery optimization
- Reduces activity when battery is low
- Stops operation if battery drops below configured threshold
- Uses efficient foreground service for background operation

### Data Usage

- Configurable bandwidth limits
- WiFi-only mode by default
- Real-time bandwidth tracking
- Automatic session termination when limits exceeded

### Memory Usage

- Efficient connection pooling
- Automatic cleanup of idle connections
- Minimal memory footprint (~2-5MB)

## Troubleshooting

### Common Issues

**SDK not starting:**
- Check if user consent is given
- Verify WiFi connection (if WiFi-only mode)
- Check battery level (if minimum battery threshold set)

**Connection issues:**
- Verify registration URL is correct
- Check network connectivity
- Ensure SDK key is valid

**High battery usage:**
- Reduce max concurrent connections
- Enable charging-only mode
- Increase minimum battery level

### Debug Logging

Enable debug mode to see detailed logs:

```kotlin
val config = IPLoopConfig.Builder()
    .setDebugMode(true)
    .build()
```

Look for logs with tag `IPLoop.*`:
- `IPLoop.SDK` - Main SDK events
- `IPLoop.ConnectionManager` - WebSocket connection
- `IPLoop.TunnelManager` - Proxy tunnels
- `IPLoop.TrafficRelay` - HTTP/SOCKS5 traffic
- `IPLoop.BandwidthTracker` - Data usage

## API Reference

### IPLoopSDK

| Method | Description |
|--------|-------------|
| `init(Context, String, IPLoopConfig)` | Initialize the SDK |
| `start()` | Start proxy service |
| `stop()` | Stop proxy service |
| `isRunning()` | Check if service is running |
| `getStatus()` | Get current status |
| `setConsentGiven(Boolean)` | Set user consent |
| `hasConsent()` | Check user consent |
| `showConsentDialog(Context)` | Show consent dialog |
| `getBandwidthUsage()` | Get bandwidth statistics |
| `getDeviceInfo()` | Get device information |

### SDKStatus Enum

- `STOPPED` - SDK is stopped
- `INITIALIZED` - SDK initialized but not started
- `STARTING` - SDK is starting up
- `RUNNING` - SDK is actively running
- `STOPPING` - SDK is shutting down
- `CONSENT_REQUIRED` - User consent needed
- `WAITING_WIFI` - Waiting for WiFi connection
- `ERROR` - Error occurred
- `DISABLED` - SDK disabled (kill switch)

## License

Copyright (c) 2026 IPLoop. All rights reserved.

## Support

For technical support, contact: sdk-support@iploop.com
For privacy questions, contact: privacy@iploop.com