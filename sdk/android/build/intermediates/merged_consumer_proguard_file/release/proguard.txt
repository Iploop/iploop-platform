# IPLoop SDK ProGuard Rules
# Keep public API classes
-keep public class com.iploop.sdk.** {
    public protected *;
}

# Keep enums
-keepclassmembers enum com.iploop.sdk.** {
    public static **[] values();
    public static ** valueOf(java.lang.String);
}

# Keep data classes
-keep @kotlinx.serialization.Serializable class com.iploop.sdk.** {
    *;
}

# Keep WebSocket listeners
-keep class * extends okhttp3.WebSocketListener {
    *;
}

# Keep service classes
-keep class com.iploop.sdk.internal.IPLoopProxyService {
    *;
}

# Keep consent activity
-keep class com.iploop.sdk.ConsentActivity {
    *;
}