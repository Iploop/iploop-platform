#include "IPLoop/ProxyConfig.h"
#include <sstream>

namespace IPLoop {

std::string ProxyConfig::toJson() const {
    std::ostringstream json;
    json << "{"
         << "\"country\":\"" << country << "\","
         << "\"city\":\"" << city << "\","
         << "\"asn\":" << asn << ","
         << "\"sessionId\":\"" << sessionId << "\","
         << "\"sessionType\":\"" << sessionType << "\","
         << "\"lifetimeMinutes\":" << lifetimeMinutes << ","
         << "\"rotateMode\":\"" << rotateMode << "\","
         << "\"rotateIntervalMinutes\":" << rotateIntervalMinutes << ","
         << "\"profile\":\"" << profile << "\","
         << "\"userAgent\":\"" << userAgent << "\","
         << "\"minSpeedMbps\":" << minSpeedMbps << ","
         << "\"maxLatencyMs\":" << maxLatencyMs << ","
         << "\"debugMode\":" << (debugMode ? "true" : "false")
         << "}";
    return json.str();
}

ProxyConfig ProxyConfig::fromJson(const std::string& json) {
    // Simplified JSON parsing for basic implementation
    // In production, would use a proper JSON library
    ProxyConfig config = createDefault();
    
    // Basic string search for key values (not robust, for demonstration only)
    size_t pos;
    
    // Extract country
    pos = json.find("\"country\":\"");
    if (pos != std::string::npos) {
        pos += 11;
        size_t endPos = json.find("\"", pos);
        if (endPos != std::string::npos) {
            config.country = json.substr(pos, endPos - pos);
        }
    }
    
    // Extract city
    pos = json.find("\"city\":\"");
    if (pos != std::string::npos) {
        pos += 8;
        size_t endPos = json.find("\"", pos);
        if (endPos != std::string::npos) {
            config.city = json.substr(pos, endPos - pos);
        }
    }
    
    // Extract sessionType
    pos = json.find("\"sessionType\":\"");
    if (pos != std::string::npos) {
        pos += 15;
        size_t endPos = json.find("\"", pos);
        if (endPos != std::string::npos) {
            config.sessionType = json.substr(pos, endPos - pos);
        }
    }
    
    // Extract lifetimeMinutes (simplified integer parsing)
    pos = json.find("\"lifetimeMinutes\":");
    if (pos != std::string::npos) {
        pos += 18;
        size_t endPos = json.find_first_of(",}", pos);
        if (endPos != std::string::npos) {
            try {
                config.lifetimeMinutes = std::stoi(json.substr(pos, endPos - pos));
            } catch (...) {
                // Keep default value
            }
        }
    }
    
    return config;
}

} // namespace IPLoop