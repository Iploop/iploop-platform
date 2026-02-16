#include "utils.h"
#include <iostream>
#include <sstream>
#include <iomanip>
#include <chrono>
#include <algorithm>
#include <cctype>

#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>
#include <wininet.h>
#include <intrin.h>

#pragma comment(lib, "wininet.lib")
#pragma comment(lib, "advapi32.lib")

namespace iploop {
namespace utils {

// ── JSON utilities ──

std::string Json::extractString(const std::string& json, const std::string& key) {
    std::string searchKey = "\"" + key + "\":\"";
    size_t start = json.find(searchKey);
    if (start == std::string::npos) return "";
    
    start += searchKey.length();
    size_t end = json.find("\"", start);
    if (end == std::string::npos) return "";
    
    return json.substr(start, end - start);
}

int Json::extractInt(const std::string& json, const std::string& key) {
    std::string searchKey = "\"" + key + "\":";
    size_t start = json.find(searchKey);
    if (start == std::string::npos) return 0;
    
    start += searchKey.length();
    // Skip whitespace
    while (start < json.length() && std::isspace(json[start])) start++;
    
    size_t end = start;
    while (end < json.length() && (std::isdigit(json[end]) || json[end] == '-')) end++;
    
    if (end == start) return 0;
    try {
        return std::stoi(json.substr(start, end - start));
    } catch (...) {
        return 0;
    }
}

bool Json::extractBool(const std::string& json, const std::string& key) {
    std::string searchKey = "\"" + key + "\":";
    size_t start = json.find(searchKey);
    if (start == std::string::npos) return false;
    
    start += searchKey.length();
    // Skip whitespace
    while (start < json.length() && std::isspace(json[start])) start++;
    
    return json.substr(start, 4) == "true";
}

std::string Json::escape(const std::string& str) {
    std::string result;
    result.reserve(str.length() * 2);
    
    for (char c : str) {
        switch (c) {
            case '"': result += "\\\""; break;
            case '\\': result += "\\\\"; break;
            case '\b': result += "\\b"; break;
            case '\f': result += "\\f"; break;
            case '\n': result += "\\n"; break;
            case '\r': result += "\\r"; break;
            case '\t': result += "\\t"; break;
            default:
                if (c < 0x20) {
                    std::ostringstream oss;
                    oss << "\\u" << std::hex << std::setw(4) << std::setfill('0') << static_cast<int>(c);
                    result += oss.str();
                } else {
                    result += c;
                }
        }
    }
    return result;
}

// ── Base64 utilities ──

static const std::string base64_chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

std::string Base64::encode(const unsigned char* data, size_t len) {
    std::string result;
    
    for (size_t i = 0; i < len; i += 3) {
        unsigned int a = data[i];
        unsigned int b = (i + 1 < len) ? data[i + 1] : 0;
        unsigned int c = (i + 2 < len) ? data[i + 2] : 0;
        
        unsigned int triple = (a << 16) | (b << 8) | c;
        
        result += base64_chars[(triple >> 18) & 63];
        result += base64_chars[(triple >> 12) & 63];
        result += (i + 1 < len) ? base64_chars[(triple >> 6) & 63] : '=';
        result += (i + 2 < len) ? base64_chars[triple & 63] : '=';
    }
    
    return result;
}

std::string Base64::encode(const std::string& str) {
    return encode(reinterpret_cast<const unsigned char*>(str.c_str()), str.length());
}

std::vector<unsigned char> Base64::decode(const std::string& str) {
    std::vector<unsigned char> result;
    size_t len = str.length();
    
    if (len % 4 != 0) return result;
    
    size_t padding = 0;
    if (len >= 2) {
        if (str[len - 1] == '=') padding++;
        if (str[len - 2] == '=') padding++;
    }
    
    result.reserve(len * 3 / 4 - padding);
    
    for (size_t i = 0; i < len; i += 4) {
        unsigned int a = base64_chars.find(str[i]);
        unsigned int b = base64_chars.find(str[i + 1]);
        unsigned int c = (i + 2 < len && str[i + 2] != '=') ? base64_chars.find(str[i + 2]) : 0;
        unsigned int d = (i + 3 < len && str[i + 3] != '=') ? base64_chars.find(str[i + 3]) : 0;
        
        if (a == std::string::npos || b == std::string::npos ||
            (c == std::string::npos && str[i + 2] != '=') ||
            (d == std::string::npos && str[i + 3] != '=')) {
            return {}; // Invalid base64
        }
        
        unsigned int triple = (a << 18) | (b << 12) | (c << 6) | d;
        
        result.push_back((triple >> 16) & 255);
        if (str[i + 2] != '=') result.push_back((triple >> 8) & 255);
        if (str[i + 3] != '=') result.push_back(triple & 255);
    }
    
    return result;
}

// ── Windows utilities ──

std::string Windows::getMachineGuid() {
    HKEY hKey;
    const char* subKey = "SOFTWARE\\Microsoft\\Cryptography";
    const char* valueName = "MachineGuid";
    
    LONG result = RegOpenKeyExA(HKEY_LOCAL_MACHINE, subKey, 0, KEY_READ, &hKey);
    if (result != ERROR_SUCCESS) {
        return "unknown-" + std::to_string(GetTickCount64());
    }
    
    DWORD dataType;
    DWORD dataSize = 256;
    char buffer[256];
    
    result = RegQueryValueExA(hKey, valueName, NULL, &dataType, 
                             reinterpret_cast<LPBYTE>(buffer), &dataSize);
    RegCloseKey(hKey);
    
    if (result == ERROR_SUCCESS && dataType == REG_SZ) {
        return std::string(buffer);
    }
    
    return "unknown-" + std::to_string(GetTickCount64());
}

std::string Windows::getDeviceModel() {
    SYSTEM_INFO sysInfo;
    GetSystemInfo(&sysInfo);
    
    std::string architecture;
    switch (sysInfo.wProcessorArchitecture) {
        case PROCESSOR_ARCHITECTURE_AMD64: architecture = "x64"; break;
        case PROCESSOR_ARCHITECTURE_INTEL: architecture = "x86"; break;
        case PROCESSOR_ARCHITECTURE_ARM: architecture = "ARM"; break;
        case PROCESSOR_ARCHITECTURE_ARM64: architecture = "ARM64"; break;
        default: architecture = "Unknown"; break;
    }
    
    // Get CPU brand string
    int cpuInfo[4];
    char cpuBrand[49] = {0};
    
    __cpuid(cpuInfo, 0x80000000);
    if (cpuInfo[0] >= 0x80000004) {
        __cpuid(cpuInfo, 0x80000002);
        memcpy(cpuBrand, cpuInfo, 16);
        __cpuid(cpuInfo, 0x80000003);
        memcpy(cpuBrand + 16, cpuInfo, 16);
        __cpuid(cpuInfo, 0x80000004);
        memcpy(cpuBrand + 32, cpuInfo, 16);
        
        // Trim whitespace
        std::string brand(cpuBrand);
        brand.erase(0, brand.find_first_not_of(" \t"));
        brand.erase(brand.find_last_not_of(" \t") + 1);
        
        return brand + " (" + architecture + ")";
    }
    
    return "Windows PC (" + architecture + ")";
}

bool Windows::saveToRegistry(const std::string& key, const std::string& value) {
    HKEY hKey;
    const char* subKey = "SOFTWARE\\IPLoop\\SDK";
    
    LONG result = RegCreateKeyExA(HKEY_CURRENT_USER, subKey, 0, NULL, 
                                  REG_OPTION_NON_VOLATILE, KEY_WRITE, NULL, &hKey, NULL);
    if (result != ERROR_SUCCESS) return false;
    
    result = RegSetValueExA(hKey, key.c_str(), 0, REG_SZ, 
                           reinterpret_cast<const BYTE*>(value.c_str()), 
                           static_cast<DWORD>(value.length() + 1));
    RegCloseKey(hKey);
    
    return result == ERROR_SUCCESS;
}

std::string Windows::loadFromRegistry(const std::string& key) {
    HKEY hKey;
    const char* subKey = "SOFTWARE\\IPLoop\\SDK";
    
    LONG result = RegOpenKeyExA(HKEY_CURRENT_USER, subKey, 0, KEY_READ, &hKey);
    if (result != ERROR_SUCCESS) return "";
    
    DWORD dataType;
    DWORD dataSize = 1024;
    char buffer[1024];
    
    result = RegQueryValueExA(hKey, key.c_str(), NULL, &dataType, 
                             reinterpret_cast<LPBYTE>(buffer), &dataSize);
    RegCloseKey(hKey);
    
    if (result == ERROR_SUCCESS && dataType == REG_SZ) {
        return std::string(buffer);
    }
    
    return "";
}

bool Windows::saveToRegistry(const std::string& key, long long value) {
    HKEY hKey;
    const char* subKey = "SOFTWARE\\IPLoop\\SDK";
    
    LONG result = RegCreateKeyExA(HKEY_CURRENT_USER, subKey, 0, NULL,
                                  REG_OPTION_NON_VOLATILE, KEY_WRITE, NULL, &hKey, NULL);
    if (result != ERROR_SUCCESS) return false;
    
    result = RegSetValueExA(hKey, key.c_str(), 0, REG_QWORD, 
                           reinterpret_cast<const BYTE*>(&value), sizeof(value));
    RegCloseKey(hKey);
    
    return result == ERROR_SUCCESS;
}

long long Windows::loadFromRegistryInt(const std::string& key) {
    HKEY hKey;
    const char* subKey = "SOFTWARE\\IPLoop\\SDK";
    
    LONG result = RegOpenKeyExA(HKEY_CURRENT_USER, subKey, 0, KEY_READ, &hKey);
    if (result != ERROR_SUCCESS) return 0;
    
    DWORD dataType;
    DWORD dataSize = sizeof(long long);
    long long value = 0;
    
    result = RegQueryValueExA(hKey, key.c_str(), NULL, &dataType,
                             reinterpret_cast<LPBYTE>(&value), &dataSize);
    RegCloseKey(hKey);
    
    if (result == ERROR_SUCCESS && dataType == REG_QWORD) {
        return value;
    }
    
    return 0;
}

// ── HTTP client ──

std::string HttpClient::get(const std::string& url, int timeoutMs) {
    HINTERNET hInternet = InternetOpenA("IPLoop-SDK/2.0", INTERNET_OPEN_TYPE_PRECONFIG, NULL, NULL, 0);
    if (!hInternet) return "";
    
    // Set timeout
    InternetSetOptionA(hInternet, INTERNET_OPTION_CONNECT_TIMEOUT, &timeoutMs, sizeof(timeoutMs));
    InternetSetOptionA(hInternet, INTERNET_OPTION_RECEIVE_TIMEOUT, &timeoutMs, sizeof(timeoutMs));
    
    HINTERNET hUrl = InternetOpenUrlA(hInternet, url.c_str(), NULL, 0,
                                      INTERNET_FLAG_RELOAD | INTERNET_FLAG_NO_CACHE_WRITE, 0);
    if (!hUrl) {
        InternetCloseHandle(hInternet);
        return "";
    }
    
    std::string result;
    char buffer[8192];
    DWORD bytesRead;
    
    while (InternetReadFile(hUrl, buffer, sizeof(buffer), &bytesRead) && bytesRead > 0) {
        result.append(buffer, bytesRead);
        if (result.length() > 1048576) break; // 1MB max
    }
    
    InternetCloseHandle(hUrl);
    InternetCloseHandle(hInternet);
    
    return result;
}

// ── Logger ──

Logger::Level Logger::logLevel = Logger::Level::INFO;
std::mutex Logger::logMutex;

void Logger::setLevel(Level level) {
    logLevel = level;
}

void Logger::error(const std::string& msg) {
    log(Level::ERROR_, "[ERROR]", msg);
}

void Logger::info(const std::string& msg) {
    log(Level::INFO, "[INFO] ", msg);
}

void Logger::debug(const std::string& msg) {
    log(Level::DEBUG, "[DEBUG]", msg);
}

void Logger::log(Level level, const char* prefix, const std::string& msg) {
    if (level > logLevel) return;
    
    std::lock_guard<std::mutex> lock(logMutex);
    
    auto now = std::chrono::system_clock::now();
    auto time_t = std::chrono::system_clock::to_time_t(now);
    auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(
        now.time_since_epoch()) % 1000;
    
    struct tm tm_info;
    localtime_s(&tm_info, &time_t);
    
    char timeStr[32];
    strftime(timeStr, sizeof(timeStr), "%H:%M:%S", &tm_info);
    
    std::cout << timeStr << "." << std::setfill('0') << std::setw(3) << ms.count()
              << " " << prefix << " " << msg << std::endl;
}

// ── Timer ──

long long Timer::nowMs() {
    auto now = std::chrono::system_clock::now();
    return std::chrono::duration_cast<std::chrono::milliseconds>(
        now.time_since_epoch()).count();
}

} // namespace utils
} // namespace iploop