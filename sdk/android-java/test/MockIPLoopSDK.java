// Mock version of IPLoopSDK for testing without Android dependencies
public class MockIPLoopSDK {
    private static final String VERSION = "1.0.20";
    private static boolean loggingEnabled = false;
    private static ProxyConfig proxyConfig;
    
    public static class ProxyConfig {
        public String country = "";
        public String city = "";
        public int asn = 0;
        public String sessionId = "";
        public String sessionType = "sticky";
        public int lifetimeMinutes = 30;
        public String rotateMode = "manual";
        public int rotateIntervalMinutes = 5;
        public String profile = "chrome-win";
        public String userAgent = "";
        public int minSpeedMbps = 10;
        public int maxLatencyMs = 1000;
        public boolean debugMode = false;
        
        public ProxyConfig setCountry(String country) {
            this.country = country;
            return this;
        }
        
        public ProxyConfig setCity(String city) {
            this.city = city;
            return this;
        }
        
        public ProxyConfig setASN(int asn) {
            this.asn = asn;
            return this;
        }
        
        public ProxyConfig setSessionId(String sessionId) {
            this.sessionId = sessionId;
            return this;
        }
        
        public ProxyConfig setSessionType(String type) {
            this.sessionType = type;
            return this;
        }
        
        public ProxyConfig setLifetime(int minutes) {
            this.lifetimeMinutes = minutes;
            return this;
        }
        
        public ProxyConfig setRotateMode(String mode) {
            this.rotateMode = mode;
            return this;
        }
        
        public ProxyConfig setRotateInterval(int minutes) {
            this.rotateIntervalMinutes = minutes;
            return this;
        }
        
        public ProxyConfig setProfile(String profile) {
            this.profile = profile;
            return this;
        }
        
        public ProxyConfig setUserAgent(String userAgent) {
            this.userAgent = userAgent;
            return this;
        }
        
        public ProxyConfig setMinSpeed(int mbps) {
            this.minSpeedMbps = mbps;
            return this;
        }
        
        public ProxyConfig setMaxLatency(int ms) {
            this.maxLatencyMs = ms;
            return this;
        }
        
        public ProxyConfig setDebugMode(boolean debug) {
            this.debugMode = debug;
            return this;
        }
        
        public String generateProxyAuth(String customerId, String apiKey) {
            StringBuilder auth = new StringBuilder();
            auth.append(customerId).append(":").append(apiKey);
            
            if (!country.isEmpty()) {
                auth.append("-country-").append(country);
            }
            
            if (!city.isEmpty()) {
                auth.append("-city-").append(city);
            }
            
            if (asn > 0) {
                auth.append("-asn-").append(asn);
            }
            
            if (!sessionId.isEmpty()) {
                auth.append("-session-").append(sessionId);
            }
            
            if (!sessionType.equals("sticky")) {
                auth.append("-sesstype-").append(sessionType);
            }
            
            if (lifetimeMinutes != 30) {
                auth.append("-lifetime-").append(lifetimeMinutes).append("m");
            }
            
            if (!rotateMode.equals("manual")) {
                auth.append("-rotate-").append(rotateMode);
            }
            
            if (rotateIntervalMinutes != 5) {
                auth.append("-rotateint-").append(rotateIntervalMinutes).append("m");
            }
            
            if (!profile.equals("chrome-win")) {
                auth.append("-profile-").append(profile);
            }
            
            if (!userAgent.isEmpty()) {
                auth.append("-ua-").append(userAgent);
            }
            
            if (minSpeedMbps != 10) {
                auth.append("-speed-").append(minSpeedMbps);
            }
            
            if (maxLatencyMs != 1000) {
                auth.append("-latency-").append(maxLatencyMs);
            }
            
            if (debugMode) {
                auth.append("-debug-1");
            }
            
            return auth.toString();
        }
    }
    
    public static String getVersion() {
        return VERSION;
    }
    
    public static void setLoggingEnabled(boolean enabled) {
        loggingEnabled = enabled;
    }
    
    public static boolean isLoggingEnabled() {
        return loggingEnabled;
    }
    
    public static void configureProxy(ProxyConfig config) {
        proxyConfig = config;
    }
    
    public static ProxyConfig getProxyConfig() {
        if (proxyConfig == null) {
            proxyConfig = new ProxyConfig();
        }
        return proxyConfig;
    }
    
    public static String getProxyAuth(String customerId, String apiKey) {
        if (proxyConfig == null) {
            return customerId + ":" + apiKey;
        }
        return proxyConfig.generateProxyAuth(customerId, apiKey);
    }
    
    public static String getProxyHost() {
        return "proxy.iploop.com";
    }
    
    public static int getHttpProxyPort() {
        return 8080;
    }
    
    public static int getSocks5ProxyPort() {
        return 1080;
    }
    
    public static String getHttpProxyUrl(String customerId, String apiKey) {
        String auth = getProxyAuth(customerId, apiKey);
        return "http://" + auth + "@" + getProxyHost() + ":" + getHttpProxyPort();
    }
    
    public static String getSocks5ProxyUrl(String customerId, String apiKey) {
        String auth = getProxyAuth(customerId, apiKey);
        return "socks5://" + auth + "@" + getProxyHost() + ":" + getSocks5ProxyPort();
    }
}