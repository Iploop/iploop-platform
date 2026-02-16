#pragma once

#include <string>
#include <vector>
#include <map>
#include <functional>
#include <mutex>

namespace iploop {
namespace utils {

/**
 * Simple JSON utilities for the IPLoop protocol
 * No external dependencies - basic parsing only
 */
class Json {
public:
    /**
     * Extract string value from JSON: "key":"value"
     * @param json JSON string
     * @param key key to find
     * @return value string or empty if not found
     */
    static std::string extractString(const std::string& json, const std::string& key);

    /**
     * Extract integer value from JSON: "key":123
     * @param json JSON string  
     * @param key key to find
     * @return value or 0 if not found
     */
    static int extractInt(const std::string& json, const std::string& key);

    /**
     * Extract boolean value from JSON: "key":true
     * @param json JSON string
     * @param key key to find  
     * @return value or false if not found
     */
    static bool extractBool(const std::string& json, const std::string& key);

    /**
     * Escape string for JSON
     * @param str input string
     * @return escaped string
     */
    static std::string escape(const std::string& str);
};

/**
 * Base64 encoding/decoding
 */
class Base64 {
public:
    /**
     * Encode binary data to base64 string
     * @param data binary data
     * @param len data length
     * @return base64 string
     */
    static std::string encode(const unsigned char* data, size_t len);

    /**
     * Encode string to base64
     * @param str input string
     * @return base64 string
     */
    static std::string encode(const std::string& str);

    /**
     * Decode base64 string to binary data
     * @param str base64 string
     * @return decoded binary data
     */
    static std::vector<unsigned char> decode(const std::string& str);
};

/**
 * Windows-specific utilities
 */
class Windows {
public:
    /**
     * Get Windows machine GUID from registry
     * @return machine GUID string
     */
    static std::string getMachineGuid();

    /**
     * Get device model string (CPU info)
     * @return device model description
     */
    static std::string getDeviceModel();

    /**
     * Save string to registry
     * @param key registry key name
     * @param value string value
     * @return true on success
     */
    static bool saveToRegistry(const std::string& key, const std::string& value);

    /**
     * Load string from registry
     * @param key registry key name
     * @return value string or empty if not found
     */
    static std::string loadFromRegistry(const std::string& key);

    /**
     * Save integer to registry
     * @param key registry key name
     * @param value integer value
     * @return true on success
     */
    static bool saveToRegistry(const std::string& key, long long value);

    /**
     * Load integer from registry
     * @param key registry key name
     * @return value or 0 if not found
     */
    static long long loadFromRegistryInt(const std::string& key);
};

/**
 * HTTP client for IP info fetching
 */
class HttpClient {
public:
    /**
     * Simple HTTP GET request
     * @param url URL to fetch
     * @param timeoutMs timeout in milliseconds
     * @return response body or empty on error
     */
    static std::string get(const std::string& url, int timeoutMs = 15000);
};

/**
 * Thread-safe logger
 */
class Logger {
public:
    enum class Level { NONE = 0, ERROR_ = 1, INFO = 2, DEBUG = 3 };
    
    static void setLevel(Level level);
    static void error(const std::string& msg);
    static void info(const std::string& msg);
    static void debug(const std::string& msg);

private:
    static Level logLevel;
    static std::mutex logMutex;
    static void log(Level level, const char* prefix, const std::string& msg);
};

/**
 * High-resolution timer
 */
class Timer {
public:
    /**
     * Get current time in milliseconds since epoch
     * @return timestamp in milliseconds
     */
    static long long nowMs();
};

} // namespace utils
} // namespace iploop