# IPLoop Android SDK

Residential proxy SDK for Android devices.

## Version
**1.0.2** (February 2026)

## Requirements
- Android 5.0+ (API 21)
- Kotlin 1.9+

## Integration

### Gradle (AAR)
```gradle
implementation files('libs/iploop-sdk-release.aar')
```

### Gradle (JAR + dependencies)
```gradle
implementation files('libs/iploop-sdk-1.0.0.jar')
implementation 'com.squareup.okhttp3:okhttp:4.12.0'
implementation 'org.jetbrains.kotlinx:kotlinx-coroutines-android:1.7.3'
```

## Permissions

**Required:**
```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
```

**Optional (for enhanced features):**
```xml
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
<uses-permission android:name="android.permission.ACCESS_WIFI_STATE" />
<uses-permission android:name="android.permission.ACCESS_COARSE_LOCATION" />
```

To exclude optional permissions:
```xml
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" tools:node="remove"/>
```

## Quick Start

```kotlin
import com.iploop.sdk.IPLoopSDK
import com.iploop.sdk.IPLoopConfig

class MyApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        
        // Initialize with default config
        IPLoopSDK.init(this, "your-sdk-key", IPLoopConfig.createDefault())
    }
}
```

## Usage

```kotlin
// Start the SDK (requires user consent)
lifecycleScope.launch {
    val result = IPLoopSDK.start()
    if (result.isSuccess) {
        Log.d("IPLoop", "SDK started")
    }
}

// Stop the SDK
lifecycleScope.launch {
    IPLoopSDK.stop()
}

// Check status
val status = IPLoopSDK.status.value // STOPPED, STARTING, RUNNING, ERROR

// Show consent dialog
IPLoopSDK.showConsentDialog(activity)
```

## Configuration

```kotlin
val config = IPLoopConfig.Builder()
    .setWifiOnly(true)              // Only operate on WiFi
    .setMaxBandwidthMB(100)         // Daily limit
    .setChargingOnly(false)         // Require charging
    .setMinBatteryLevel(20)         // Min battery %
    .setShareLocation(false)        // Geo-targeting opt-in
    .setDebugMode(false)            // Verbose logging
    .build()

IPLoopSDK.init(context, "sdk-key", config)
```

### Preset Configurations

```kotlin
// Default (balanced)
IPLoopConfig.createDefault()

// Privacy-focused (conservative)
IPLoopConfig.createPrivacyFriendly()

// High performance (more bandwidth)
IPLoopConfig.createHighPerformance()
```

## Connection Details

- **WebSocket:** `wss://gateway.iploop.io/ws`
- **Protocol:** JSON over WebSocket
- **Heartbeat:** 30 seconds
- **Reconnect:** Automatic with backoff

## Status Callbacks

```kotlin
lifecycleScope.launch {
    IPLoopSDK.status.collect { status ->
        when (status) {
            SDKStatus.STOPPED -> { /* Not running */ }
            SDKStatus.STARTING -> { /* Connecting */ }
            SDKStatus.RUNNING -> { /* Active */ }
            SDKStatus.ERROR -> { /* Check logs */ }
        }
    }
}
```

## ProGuard

If using ProGuard/R8, add:
```proguard
-keep class com.iploop.sdk.** { *; }
-keepclassmembers class com.iploop.sdk.** { *; }
```

## Files

| File | Description |
|------|-------------|
| `iploop-sdk-release.aar` | Android Archive (recommended) |
| `iploop-sdk-1.0.0.jar` | Java Archive |
| `iploop-sdk-dexed.jar` | Pre-dexed for dynamic loading |
| `iploop-sdk-source.zip` | Source code |

## Support

- Dashboard: https://gateway.iploop.io
- API Docs: https://gateway.iploop.io/docs

## Changelog

### 1.0.2 (2026-02-04)
- Fixed WebSocket URL (now `/ws`)
- Permanent gateway: `gateway.iploop.io`

### 1.0.1
- Initial permanent URL support

### 1.0.0
- Initial release
