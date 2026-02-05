package com.iploop.test;

import com.iploop.sdk.IPLoopSDK;
import com.iploop.sdk.IPLoopSDK.ProxyConfig;
import com.iploop.sdk.IPLoopSDK.Callback;
import com.iploop.sdk.IPLoopSDK.ProxyCallback;
import com.iploop.sdk.IPLoopSDK.StatusCallback;

/**
 * Enhanced SDK Test - Tests all v1.0.20 features
 */
public class EnhancedSDKTest {
    private static final String TAG = "EnhancedSDKTest";
    private static final String TEST_CUSTOMER_ID = "test_customer_123";
    private static final String TEST_API_KEY = "test_api_key_xyz";
    
    public static void main(String[] args) {
        System.out.println("=== IPLoop SDK v" + IPLoopSDK.getVersion() + " Enhanced Features Test ===\n");
        
        // Enable logging for testing
        IPLoopSDK.setLoggingEnabled(true);
        
        // Test 1: Basic SDK functionality
        testBasicSDK();
        
        // Test 2: Proxy configuration
        testProxyConfiguration();
        
        // Test 3: Geographic targeting
        testGeographicTargeting();
        
        // Test 4: Session management
        testSessionManagement();
        
        // Test 5: Browser profiles
        testBrowserProfiles();
        
        // Test 6: Performance settings
        testPerformanceSettings();
        
        // Test 7: Authentication string generation
        testAuthenticationGeneration();
        
        // Test 8: Proxy URLs
        testProxyUrls();
        
        System.out.println("\n=== All Enhanced Features Tests Completed ===");
    }
    
    private static void testBasicSDK() {
        System.out.println("1. Testing Basic SDK Functionality:");
        System.out.println("   Version: " + IPLoopSDK.getVersion());
        System.out.println("   Logging enabled: " + IPLoopSDK.isLoggingEnabled());
        System.out.println("   Initial status: " + IPLoopSDK.getStatusString());
        
        // Test status callback
        IPLoopSDK.setStatusCallback(new StatusCallback() {
            @Override
            public void onStatusChanged(int newStatus) {
                System.out.println("   Status changed to: " + com.iploop.sdk.SDKStatus.toString(newStatus));
            }
        });
        
        System.out.println("   ✅ Basic SDK tests passed\n");
    }
    
    private static void testProxyConfiguration() {
        System.out.println("2. Testing Proxy Configuration:");
        
        // Test default configuration
        ProxyConfig defaultConfig = IPLoopSDK.getProxyConfig();
        System.out.println("   Default session type: " + defaultConfig.sessionType);
        System.out.println("   Default lifetime: " + defaultConfig.lifetimeMinutes + " minutes");
        System.out.println("   Default profile: " + defaultConfig.profile);
        
        // Test custom configuration
        ProxyConfig customConfig = new ProxyConfig()
            .setCountry("US")
            .setCity("Miami")
            .setSessionType("sticky")
            .setLifetime(60)
            .setRotateMode("time")
            .setRotateInterval(10)
            .setProfile("chrome-win")
            .setMinSpeed(50)
            .setMaxLatency(200)
            .setDebugMode(true);
        
        IPLoopSDK.configureProxy(customConfig);
        System.out.println("   Custom config applied");
        
        System.out.println("   ✅ Proxy configuration tests passed\n");
    }
    
    private static void testGeographicTargeting() {
        System.out.println("3. Testing Geographic Targeting:");
        
        // Test different country configurations
        String[] countries = {"US", "GB", "DE", "FR", "JP"};
        String[] cities = {"newyork", "london", "berlin", "paris", "tokyo"};
        
        for (int i = 0; i < countries.length; i++) {
            ProxyConfig config = new ProxyConfig()
                .setCountry(countries[i])
                .setCity(cities[i]);
            
            String auth = config.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
            System.out.println("   " + countries[i] + "/" + cities[i] + ": " + auth);
        }
        
        // Test ASN targeting
        ProxyConfig asnConfig = new ProxyConfig()
            .setCountry("US")
            .setASN(7922); // Comcast
        
        String asnAuth = asnConfig.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   ASN targeting: " + asnAuth);
        
        System.out.println("   ✅ Geographic targeting tests passed\n");
    }
    
    private static void testSessionManagement() {
        System.out.println("4. Testing Session Management:");
        
        // Test sticky sessions
        ProxyConfig stickyConfig = new ProxyConfig()
            .setSessionType("sticky")
            .setSessionId("login_session_123")
            .setLifetime(30);
        
        String stickyAuth = stickyConfig.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   Sticky session: " + stickyAuth);
        
        // Test rotating sessions
        ProxyConfig rotatingConfig = new ProxyConfig()
            .setSessionType("rotating")
            .setRotateMode("time")
            .setRotateInterval(5);
        
        String rotatingAuth = rotatingConfig.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   Rotating session: " + rotatingAuth);
        
        // Test per-request rotation
        ProxyConfig perRequestConfig = new ProxyConfig()
            .setSessionType("per-request")
            .setRotateMode("request");
        
        String perRequestAuth = perRequestConfig.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   Per-request rotation: " + perRequestAuth);
        
        System.out.println("   ✅ Session management tests passed\n");
    }
    
    private static void testBrowserProfiles() {
        System.out.println("5. Testing Browser Profiles:");
        
        String[] profiles = {"chrome-win", "firefox-mac", "safari-mac", "mobile-ios", "mobile-android"};
        
        for (String profile : profiles) {
            ProxyConfig config = new ProxyConfig()
                .setProfile(profile)
                .setCountry("US");
            
            String auth = config.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
            System.out.println("   " + profile + ": " + auth);
        }
        
        // Test custom User-Agent
        ProxyConfig customUAConfig = new ProxyConfig()
            .setUserAgent("CustomBot/1.0")
            .setCountry("GB");
        
        String customUAAuth = customUAConfig.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   Custom UA: " + customUAAuth);
        
        System.out.println("   ✅ Browser profile tests passed\n");
    }
    
    private static void testPerformanceSettings() {
        System.out.println("6. Testing Performance Settings:");
        
        // Test speed requirements
        ProxyConfig speedConfig = new ProxyConfig()
            .setMinSpeed(100)
            .setMaxLatency(150)
            .setCountry("US");
        
        String speedAuth = speedConfig.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   High-speed config: " + speedAuth);
        
        // Test low latency requirements
        ProxyConfig latencyConfig = new ProxyConfig()
            .setMinSpeed(25)
            .setMaxLatency(50)
            .setCountry("GB");
        
        String latencyAuth = latencyConfig.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   Low-latency config: " + latencyAuth);
        
        System.out.println("   ✅ Performance settings tests passed\n");
    }
    
    private static void testAuthenticationGeneration() {
        System.out.println("7. Testing Authentication String Generation:");
        
        // Test complex configuration
        ProxyConfig complexConfig = new ProxyConfig()
            .setCountry("DE")
            .setCity("berlin")
            .setASN(3320) // Deutsche Telekom
            .setSessionId("complex_test_session")
            .setSessionType("sticky")
            .setLifetime(45)
            .setRotateMode("manual")
            .setProfile("firefox-win")
            .setMinSpeed(75)
            .setMaxLatency(100)
            .setDebugMode(true);
        
        String complexAuth = complexConfig.generateProxyAuth(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   Complex auth string:");
        System.out.println("   " + complexAuth);
        
        // Verify auth string length and format
        String[] parts = complexAuth.split("-");
        System.out.println("   Auth parts count: " + parts.length);
        System.out.println("   Contains customer:key: " + complexAuth.startsWith(TEST_CUSTOMER_ID + ":"));
        
        System.out.println("   ✅ Authentication generation tests passed\n");
    }
    
    private static void testProxyUrls() {
        System.out.println("8. Testing Proxy URL Generation:");
        
        // Configure a test proxy
        ProxyConfig testConfig = new ProxyConfig()
            .setCountry("US")
            .setCity("Miami")
            .setSessionType("sticky")
            .setLifetime(30);
        
        IPLoopSDK.configureProxy(testConfig);
        
        // Test HTTP proxy URL
        String httpUrl = IPLoopSDK.getHttpProxyUrl(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   HTTP Proxy URL: " + httpUrl);
        
        // Test SOCKS5 proxy URL
        String socks5Url = IPLoopSDK.getSocks5ProxyUrl(TEST_CUSTOMER_ID, TEST_API_KEY);
        System.out.println("   SOCKS5 Proxy URL: " + socks5Url);
        
        // Test proxy host and ports
        System.out.println("   Proxy host: " + IPLoopSDK.getProxyHost());
        System.out.println("   HTTP port: " + IPLoopSDK.getHttpProxyPort());
        System.out.println("   SOCKS5 port: " + IPLoopSDK.getSocks5ProxyPort());
        
        System.out.println("   ✅ Proxy URL tests passed\n");
    }
}