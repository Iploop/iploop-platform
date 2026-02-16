#pragma once

#include <string>

namespace IPLoop {

/**
 * Advanced proxy configuration - mirrors Android SDK enterprise features
 * Supports geographic targeting, session management, and browser profiles
 */
class ProxyConfig {
public:
    // Geographic targeting
    std::string country;                    // Target country code (US, DE, FR, etc.)
    std::string city;                       // Target city name (miami, london, tokyo)
    int asn = 0;                           // Target ASN/ISP number
    
    // Session management
    std::string sessionId;                  // Custom session identifier
    std::string sessionType = "sticky";    // sticky, rotating, per-request
    int lifetimeMinutes = 30;              // Session lifetime
    std::string rotateMode = "manual";     // request, time, manual, ip-change
    int rotateIntervalMinutes = 5;         // Auto-rotation interval
    
    // Browser profiles
    std::string profile = "chrome-win";    // chrome-win, firefox-mac, mobile-ios, etc.
    std::string userAgent;                 // Custom User-Agent string
    
    // Performance requirements
    int minSpeedMbps = 10;                 // Minimum speed requirement
    int maxLatencyMs = 1000;               // Maximum latency requirement
    
    // Debug settings
    bool debugMode = false;                // Enable debug logging
    
    /**
     * Create default configuration
     */
    static ProxyConfig createDefault() {
        ProxyConfig config;
        config.sessionType = "sticky";
        config.lifetimeMinutes = 30;
        config.rotateMode = "manual";
        config.profile = "chrome-win";
        config.minSpeedMbps = 10;
        config.maxLatencyMs = 1000;
        return config;
    }
    
    /**
     * Builder pattern methods for easy configuration
     */
    ProxyConfig& setCountry(const std::string& country) {
        this->country = country;
        return *this;
    }
    
    ProxyConfig& setCity(const std::string& city) {
        this->city = city;
        return *this;
    }
    
    ProxyConfig& setASN(int asn) {
        this->asn = asn;
        return *this;
    }
    
    ProxyConfig& setSessionId(const std::string& sessionId) {
        this->sessionId = sessionId;
        return *this;
    }
    
    ProxyConfig& setSessionType(const std::string& type) {
        this->sessionType = type;
        return *this;
    }
    
    ProxyConfig& setLifetime(int minutes) {
        this->lifetimeMinutes = minutes;
        return *this;
    }
    
    ProxyConfig& setRotateMode(const std::string& mode) {
        this->rotateMode = mode;
        return *this;
    }
    
    ProxyConfig& setRotateInterval(int minutes) {
        this->rotateIntervalMinutes = minutes;
        return *this;
    }
    
    ProxyConfig& setProfile(const std::string& profile) {
        this->profile = profile;
        return *this;
    }
    
    ProxyConfig& setUserAgent(const std::string& userAgent) {
        this->userAgent = userAgent;
        return *this;
    }
    
    ProxyConfig& setMinSpeed(int mbps) {
        this->minSpeedMbps = mbps;
        return *this;
    }
    
    ProxyConfig& setMaxLatency(int ms) {
        this->maxLatencyMs = ms;
        return *this;
    }
    
    ProxyConfig& setDebugMode(bool enabled) {
        this->debugMode = enabled;
        return *this;
    }
    
    /**
     * Generate proxy auth string with parameters
     * Format: "apikey-country-US-city-miami-session-sticky-lifetime-30"
     */
    std::string generateAuthString(const std::string& apiKey) const {
        std::string auth = apiKey;
        
        if (!country.empty()) {
            auth += "-country-" + country;
        }
        
        if (!city.empty()) {
            auth += "-city-" + city;
        }
        
        if (asn > 0) {
            auth += "-asn-" + std::to_string(asn);
        }
        
        if (!sessionId.empty()) {
            auth += "-session-" + sessionId;
        }
        
        if (sessionType != "sticky") {
            auth += "-sesstype-" + sessionType;
        }
        
        if (lifetimeMinutes != 30) {
            auth += "-lifetime-" + std::to_string(lifetimeMinutes) + "m";
        }
        
        if (rotateMode != "manual") {
            auth += "-rotate-" + rotateMode;
        }
        
        if (profile != "chrome-win") {
            auth += "-profile-" + profile;
        }
        
        if (minSpeedMbps != 10) {
            auth += "-speed-" + std::to_string(minSpeedMbps);
        }
        
        if (maxLatencyMs != 1000) {
            auth += "-latency-" + std::to_string(maxLatencyMs);
        }
        
        if (debugMode) {
            auth += "-debug-1";
        }
        
        return auth;
    }
    
    /**
     * Validate configuration
     */
    bool isValid() const {
        if (lifetimeMinutes <= 0 || lifetimeMinutes > 1440) { // Max 24 hours
            return false;
        }
        
        if (minSpeedMbps < 1 || minSpeedMbps > 1000) {
            return false;
        }
        
        if (maxLatencyMs < 10 || maxLatencyMs > 30000) {
            return false;
        }
        
        if (sessionType != "sticky" && sessionType != "rotating" && sessionType != "per-request") {
            return false;
        }
        
        if (rotateMode != "manual" && rotateMode != "request" && 
            rotateMode != "time" && rotateMode != "ip-change") {
            return false;
        }
        
        return true;
    }
    
    /**
     * Get configuration as JSON string
     */
    std::string toJson() const;
    
    /**
     * Load configuration from JSON string
     */
    static ProxyConfig fromJson(const std::string& json);
};

} // namespace IPLoop