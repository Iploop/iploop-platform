# Add project specific ProGuard rules here.
# You can control the set of applied configuration files using the
# proguardFiles setting in build.gradle.

# IPLoop SDK internal ProGuard rules

# Keep native methods
-keepclasseswithmembernames class * {
    native <methods>;
}

# Keep WebSocket implementation
-dontwarn okhttp3.**
-dontwarn okio.**

# Keep coroutines
-keepnames class kotlinx.coroutines.internal.MainDispatcherFactory {}
-keepnames class kotlinx.coroutines.CoroutineExceptionHandler {}

# Keep JSON classes if using them
-keep class org.json.** { *; }