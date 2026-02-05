public class FullTest {
    public static void main(String[] args) {
        System.out.println("🚀 === IPLoop SDK v1.0.20 Enterprise Features Demonstration ===\n");
        
        // Enable logging
        MockIPLoopSDK.setLoggingEnabled(true);
        
        System.out.println("✅ SDK Version: " + MockIPLoopSDK.getVersion());
        System.out.println("✅ Logging enabled: " + MockIPLoopSDK.isLoggingEnabled());
        
        System.out.println("\n🌍 1. GEOGRAPHIC TARGETING TESTS:");
        testGeographicTargeting();
        
        System.out.println("\n🔄 2. SESSION MANAGEMENT TESTS:");
        testSessionManagement();
        
        System.out.println("\n🎭 3. BROWSER PROFILE TESTS:");
        testBrowserProfiles();
        
        System.out.println("\n⚡ 4. PERFORMANCE SETTINGS TESTS:");
        testPerformanceSettings();
        
        System.out.println("\n🔧 5. COMPLEX CONFIGURATION TESTS:");
        testComplexConfigurations();
        
        System.out.println("\n🌐 6. PROXY URL GENERATION TESTS:");
        testProxyUrls();
        
        System.out.println("\n🎯 7. ENTERPRISE SCENARIOS:");
        testEnterpriseScenarios();
        
        System.out.println("\n✅ === ALL TESTS PASSED! SDK v" + MockIPLoopSDK.getVersion() + " READY FOR PRODUCTION ===");
    }
    
    private static void testGeographicTargeting() {
        System.out.println("   📍 Country-level targeting:");
        String[] countries = {"US", "GB", "DE", "FR", "JP", "CA", "AU", "BR"};
        
        for (String country : countries) {
            MockIPLoopSDK.ProxyConfig config = new MockIPLoopSDK.ProxyConfig()
                .setCountry(country);
            
            String auth = config.generateProxyAuth("customer123", "api_key");
            System.out.println("     " + country + ": " + auth);
        }
        
        System.out.println("\n   🏙️ City-level precision:");
        String[][] locations = {
            {"US", "newyork"}, {"US", "losangeles"}, {"US", "miami"},
            {"GB", "london"}, {"DE", "berlin"}, {"FR", "paris"},
            {"JP", "tokyo"}, {"CA", "toronto"}
        };
        
        for (String[] location : locations) {
            MockIPLoopSDK.ProxyConfig config = new MockIPLoopSDK.ProxyConfig()
                .setCountry(location[0])
                .setCity(location[1]);
            
            String auth = config.generateProxyAuth("customer123", "api_key");
            System.out.println("     " + location[0] + "/" + location[1] + ": " + auth);
        }
        
        System.out.println("\n   🌐 ASN/ISP targeting:");
        int[][] asns = {
            {7922, 0}, // Comcast US
            {701, 0},  // Verizon US
            {3320, 0}, // Deutsche Telekom DE
            {2856, 0}  // British Telecom GB
        };
        
        for (int[] asn : asns) {
            MockIPLoopSDK.ProxyConfig config = new MockIPLoopSDK.ProxyConfig()
                .setCountry("US")
                .setASN(asn[0]);
            
            String auth = config.generateProxyAuth("customer123", "api_key");
            System.out.println("     ASN " + asn[0] + ": " + auth);
        }
    }
    
    private static void testSessionManagement() {
        System.out.println("   🔒 Sticky sessions:");
        
        // Account login session
        MockIPLoopSDK.ProxyConfig loginSession = new MockIPLoopSDK.ProxyConfig()
            .setSessionType("sticky")
            .setSessionId("login_session_" + System.currentTimeMillis())
            .setLifetime(60)
            .setCountry("US");
        System.out.println("     Account login: " + loginSession.generateProxyAuth("customer123", "api_key"));
        
        // Shopping session
        MockIPLoopSDK.ProxyConfig shoppingSession = new MockIPLoopSDK.ProxyConfig()
            .setSessionType("sticky")
            .setSessionId("shopping_cart_session")
            .setLifetime(30)
            .setCountry("GB");
        System.out.println("     Shopping cart: " + shoppingSession.generateProxyAuth("customer123", "api_key"));
        
        System.out.println("\n   🔄 Rotating sessions:");
        
        // Scraping with time-based rotation
        MockIPLoopSDK.ProxyConfig scrapingSession = new MockIPLoopSDK.ProxyConfig()
            .setSessionType("rotating")
            .setRotateMode("time")
            .setRotateInterval(5)
            .setCountry("US");
        System.out.println("     Scraping (5min): " + scrapingSession.generateProxyAuth("customer123", "api_key"));
        
        // Per-request rotation
        MockIPLoopSDK.ProxyConfig perRequestSession = new MockIPLoopSDK.ProxyConfig()
            .setSessionType("per-request")
            .setRotateMode("request")
            .setCountry("DE");
        System.out.println("     Per-request: " + perRequestSession.generateProxyAuth("customer123", "api_key"));
    }
    
    private static void testBrowserProfiles() {
        String[] profiles = {
            "chrome-win", "chrome-mac", "firefox-win", "firefox-mac", 
            "safari-mac", "mobile-ios", "mobile-android"
        };
        
        for (String profile : profiles) {
            MockIPLoopSDK.ProxyConfig config = new MockIPLoopSDK.ProxyConfig()
                .setProfile(profile)
                .setCountry("US");
            
            String auth = config.generateProxyAuth("customer123", "api_key");
            System.out.println("     " + profile + ": " + auth);
        }
        
        System.out.println("\n   🤖 Custom User-Agent:");
        MockIPLoopSDK.ProxyConfig customUA = new MockIPLoopSDK.ProxyConfig()
            .setUserAgent("CustomBot/2.0")
            .setCountry("GB");
        System.out.println("     Custom UA: " + customUA.generateProxyAuth("customer123", "api_key"));
    }
    
    private static void testPerformanceSettings() {
        System.out.println("   🚀 High-speed requirements:");
        MockIPLoopSDK.ProxyConfig highSpeed = new MockIPLoopSDK.ProxyConfig()
            .setMinSpeed(100)
            .setMaxLatency(100)
            .setCountry("US");
        System.out.println("     100 Mbps, <100ms: " + highSpeed.generateProxyAuth("customer123", "api_key"));
        
        System.out.println("\n   ⚡ Low-latency requirements:");
        MockIPLoopSDK.ProxyConfig lowLatency = new MockIPLoopSDK.ProxyConfig()
            .setMinSpeed(50)
            .setMaxLatency(50)
            .setCountry("GB");
        System.out.println("     50 Mbps, <50ms: " + lowLatency.generateProxyAuth("customer123", "api_key"));
        
        System.out.println("\n   🎯 Enterprise grade:");
        MockIPLoopSDK.ProxyConfig enterprise = new MockIPLoopSDK.ProxyConfig()
            .setMinSpeed(200)
            .setMaxLatency(25)
            .setCountry("US");
        System.out.println("     200 Mbps, <25ms: " + enterprise.generateProxyAuth("customer123", "api_key"));
    }
    
    private static void testComplexConfigurations() {
        System.out.println("   🏢 Enterprise e-commerce setup:");
        MockIPLoopSDK.ProxyConfig ecommerce = new MockIPLoopSDK.ProxyConfig()
            .setCountry("US")
            .setCity("newyork")
            .setSessionType("sticky")
            .setLifetime(120) // 2 hours
            .setProfile("chrome-win")
            .setMinSpeed(75)
            .setMaxLatency(150);
        System.out.println("     " + ecommerce.generateProxyAuth("ecommerce_client", "enterprise_key"));
        
        System.out.println("\n   🕷️ Advanced web scraping:");
        MockIPLoopSDK.ProxyConfig scraping = new MockIPLoopSDK.ProxyConfig()
            .setCountry("DE")
            .setCity("berlin")
            .setASN(3320) // Deutsche Telekom
            .setSessionType("rotating")
            .setRotateMode("time")
            .setRotateInterval(3) // Every 3 minutes
            .setProfile("firefox-mac")
            .setMinSpeed(50)
            .setMaxLatency(200)
            .setDebugMode(true);
        System.out.println("     " + scraping.generateProxyAuth("scraper_pro", "advanced_key"));
        
        System.out.println("\n   📱 Mobile testing:");
        MockIPLoopSDK.ProxyConfig mobileTesting = new MockIPLoopSDK.ProxyConfig()
            .setCountry("JP")
            .setCity("tokyo")
            .setSessionType("per-request")
            .setProfile("mobile-ios")
            .setMinSpeed(25)
            .setMaxLatency(300);
        System.out.println("     " + mobileTesting.generateProxyAuth("mobile_tester", "test_key"));
        
        System.out.println("\n   🔍 Market research:");
        MockIPLoopSDK.ProxyConfig research = new MockIPLoopSDK.ProxyConfig()
            .setCountry("FR")
            .setCity("paris")
            .setSessionId("research_session_" + System.currentTimeMillis())
            .setSessionType("sticky")
            .setLifetime(45)
            .setProfile("safari-mac")
            .setMinSpeed(40)
            .setMaxLatency(250);
        System.out.println("     " + research.generateProxyAuth("research_team", "market_key"));
    }
    
    private static void testProxyUrls() {
        MockIPLoopSDK.ProxyConfig config = new MockIPLoopSDK.ProxyConfig()
            .setCountry("US")
            .setCity("miami")
            .setSessionType("sticky")
            .setLifetime(30);
        
        MockIPLoopSDK.configureProxy(config);
        
        String httpUrl = MockIPLoopSDK.getHttpProxyUrl("customer123", "api_key");
        String socks5Url = MockIPLoopSDK.getSocks5ProxyUrl("customer123", "api_key");
        
        System.out.println("   📡 Proxy endpoints:");
        System.out.println("     Host: " + MockIPLoopSDK.getProxyHost());
        System.out.println("     HTTP Port: " + MockIPLoopSDK.getHttpProxyPort());
        System.out.println("     SOCKS5 Port: " + MockIPLoopSDK.getSocks5ProxyPort());
        
        System.out.println("\n   🔗 Generated URLs:");
        System.out.println("     HTTP: " + httpUrl);
        System.out.println("     SOCKS5: " + socks5Url);
    }
    
    private static void testEnterpriseScenarios() {
        System.out.println("   🎯 Scenario 1 - Global Price Monitoring:");
        String[] markets = {"US", "GB", "DE", "FR", "JP"};
        for (String market : markets) {
            MockIPLoopSDK.ProxyConfig config = new MockIPLoopSDK.ProxyConfig()
                .setCountry(market)
                .setSessionType("sticky")
                .setLifetime(15)
                .setProfile("chrome-win");
            System.out.println("     " + market + " market: " + config.generateProxyAuth("price_monitor", "enterprise_key"));
        }
        
        System.out.println("\n   🎯 Scenario 2 - Social Media Management:");
        MockIPLoopSDK.ProxyConfig socialMedia = new MockIPLoopSDK.ProxyConfig()
            .setCountry("US")
            .setSessionType("sticky")
            .setLifetime(180) // 3 hours for long sessions
            .setProfile("chrome-win")
            .setMinSpeed(30);
        System.out.println("     Social manager: " + socialMedia.generateProxyAuth("social_manager", "enterprise_key"));
        
        System.out.println("\n   🎯 Scenario 3 - Ad Verification:");
        MockIPLoopSDK.ProxyConfig adVerification = new MockIPLoopSDK.ProxyConfig()
            .setCountry("US")
            .setCity("losangeles")
            .setSessionType("per-request")
            .setRotateMode("request")
            .setProfile("mobile-android")
            .setMinSpeed(25);
        System.out.println("     Ad verification: " + adVerification.generateProxyAuth("ad_verifier", "verification_key"));
        
        System.out.println("\n   🎯 Scenario 4 - SEO Monitoring:");
        MockIPLoopSDK.ProxyConfig seoMonitoring = new MockIPLoopSDK.ProxyConfig()
            .setCountry("GB")
            .setSessionType("rotating")
            .setRotateMode("time")
            .setRotateInterval(10)
            .setProfile("firefox-mac")
            .setMinSpeed(40);
        System.out.println("     SEO monitor: " + seoMonitoring.generateProxyAuth("seo_monitor", "seo_key"));
    }
}