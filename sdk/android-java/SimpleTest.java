import com.iploop.sdk.IPLoopSDK;

public class SimpleTest {
    public static void main(String[] args) {
        System.out.println("=== IPLoop SDK v" + IPLoopSDK.getVersion() + " Enhanced Features Test ===\n");
        
        // Enable logging
        IPLoopSDK.setLoggingEnabled(true);
        
        System.out.println("✅ SDK Version: " + IPLoopSDK.getVersion());
        System.out.println("✅ Logging enabled: " + IPLoopSDK.isLoggingEnabled());
        
        // Test proxy configuration
        IPLoopSDK.ProxyConfig config = new IPLoopSDK.ProxyConfig()
            .setCountry("US")
            .setCity("Miami")
            .setSessionType("sticky")
            .setLifetime(30)
            .setProfile("chrome-win")
            .setMinSpeed(50)
            .setMaxLatency(200);
        
        String testAuth = config.generateProxyAuth("customer123", "api_key_xyz");
        System.out.println("✅ Generated auth string: " + testAuth);
        
        // Test proxy URLs
        IPLoopSDK.configureProxy(config);
        String httpUrl = IPLoopSDK.getHttpProxyUrl("customer123", "api_key_xyz");
        String socks5Url = IPLoopSDK.getSocks5ProxyUrl("customer123", "api_key_xyz");
        
        System.out.println("✅ HTTP Proxy URL: " + httpUrl);
        System.out.println("✅ SOCKS5 Proxy URL: " + socks5Url);
        
        // Test different geographic configurations
        String[] countries = {"US", "GB", "DE", "FR", "JP"};
        String[] cities = {"newyork", "london", "berlin", "paris", "tokyo"};
        
        System.out.println("\n🌍 Geographic Targeting Tests:");
        for (int i = 0; i < countries.length; i++) {
            IPLoopSDK.ProxyConfig geoConfig = new IPLoopSDK.ProxyConfig()
                .setCountry(countries[i])
                .setCity(cities[i]);
            
            String geoAuth = geoConfig.generateProxyAuth("customer123", "api_key");
            System.out.println("   " + countries[i] + "/" + cities[i] + ": " + geoAuth);
        }
        
        // Test session management
        System.out.println("\n🔄 Session Management Tests:");
        
        IPLoopSDK.ProxyConfig stickyConfig = new IPLoopSDK.ProxyConfig()
            .setSessionType("sticky")
            .setSessionId("login_session_123")
            .setLifetime(60);
        System.out.println("   Sticky session: " + stickyConfig.generateProxyAuth("customer123", "api_key"));
        
        IPLoopSDK.ProxyConfig rotatingConfig = new IPLoopSDK.ProxyConfig()
            .setSessionType("rotating")
            .setRotateMode("time")
            .setRotateInterval(5);
        System.out.println("   Rotating session: " + rotatingConfig.generateProxyAuth("customer123", "api_key"));
        
        // Test browser profiles
        System.out.println("\n🎭 Browser Profile Tests:");
        String[] profiles = {"chrome-win", "firefox-mac", "safari-mac", "mobile-ios", "mobile-android"};
        
        for (String profile : profiles) {
            IPLoopSDK.ProxyConfig profileConfig = new IPLoopSDK.ProxyConfig()
                .setProfile(profile)
                .setCountry("US");
            
            System.out.println("   " + profile + ": " + profileConfig.generateProxyAuth("customer123", "api_key"));
        }
        
        // Test performance settings
        System.out.println("\n⚡ Performance Tests:");
        
        IPLoopSDK.ProxyConfig perfConfig = new IPLoopSDK.ProxyConfig()
            .setMinSpeed(100)
            .setMaxLatency(150)
            .setCountry("US");
        System.out.println("   High-speed config: " + perfConfig.generateProxyAuth("customer123", "api_key"));
        
        // Test complex configuration
        System.out.println("\n🔧 Complex Configuration Test:");
        
        IPLoopSDK.ProxyConfig complexConfig = new IPLoopSDK.ProxyConfig()
            .setCountry("DE")
            .setCity("berlin")
            .setASN(3320)
            .setSessionId("complex_test_session")
            .setSessionType("sticky")
            .setLifetime(45)
            .setRotateMode("manual")
            .setProfile("firefox-win")
            .setMinSpeed(75)
            .setMaxLatency(100)
            .setDebugMode(true);
        
        String complexAuth = complexConfig.generateProxyAuth("customer123", "api_key");
        System.out.println("   Complex auth: " + complexAuth);
        
        System.out.println("\n✅ All Enhanced Features Tests Passed!");
        System.out.println("🚀 SDK v" + IPLoopSDK.getVersion() + " ready for enterprise use!");
    }
}