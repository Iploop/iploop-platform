#pragma once

#include <string>
#include <vector>
#include <cstdint>

namespace IPLoop {
namespace Utils {

/**
 * Utility functions for IPLoop SDK v2.0
 */

// Time utilities
uint64_t getCurrentTimestamp();                        // Unix timestamp in milliseconds
std::string getCurrentTimeString();                    // Human-readable time string
uint64_t getSystemUptime();                           // System uptime in milliseconds

// String utilities
std::string generateUUID();                           // Generate UUID v4
std::string sha256(const std::string& input);         // SHA-256 hash
std::string base64Encode(const std::vector<uint8_t>& input);  // Base64 encoding (legacy support)
std::vector<uint8_t> base64Decode(const std::string& input); // Base64 decoding (legacy support)
std::string urlEncode(const std::string& input);      // URL encoding
std::string trim(const std::string& input);           // Trim whitespace
std::vector<std::string> split(const std::string& input, char delimiter);  // String splitting

// Network utilities
std::string getLocalIP();                             // Get local IP address
std::string getMACAddress();                          // Get primary MAC address
bool isValidIP(const std::string& ip);               // Validate IP address
std::string resolveHostname(const std::string& hostname);  // DNS resolution

// System utilities
std::string getOSVersion();                           // Get Windows version
std::string getArchitecture();                       // Get CPU architecture (x64, x86, arm64)
uint32_t getCPUCores();                              // Get CPU core count
uint32_t getAvailableMemoryMB();                    // Get available RAM in MB
std::string getHostname();                           // Get computer name

// File utilities
bool fileExists(const std::string& path);            // Check file existence
std::string readFile(const std::string& path);       // Read entire file
bool writeFile(const std::string& path, const std::string& content);  // Write file
std::string getAppDataPath();                        // Get %APPDATA% path
std::string getTempPath();                           // Get temp directory

// Conversion utilities
std::wstring stringToWString(const std::string& str);  // String to wide string
std::string wstringToString(const std::wstring& wstr); // Wide string to string
std::string bytesToHex(const std::vector<uint8_t>& bytes);  // Bytes to hex string
std::vector<uint8_t> hexToBytes(const std::string& hex);    // Hex string to bytes

// v2.0: Binary protocol utilities
std::vector<uint8_t> packBinaryMessage(const std::string& type, const std::vector<uint8_t>& payload);
std::pair<std::string, std::vector<uint8_t>> unpackBinaryMessage(const std::vector<uint8_t>& data);
uint32_t calculateCRC32(const std::vector<uint8_t>& data);  // CRC32 checksum
bool validateBinaryMessage(const std::vector<uint8_t>& data);  // Message validation

} // namespace Utils
} // namespace IPLoop