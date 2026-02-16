#include "internal/Logger.h"
#include <iostream>
#include <mutex>
#include <chrono>
#include <iomanip>
#include <sstream>

namespace IPLoop {

class Logger::Impl {
public:
    Impl() : 
        minLevel(LogLevel::INFO),
        enabled(true)
    {
    }
    
    void log(LogLevel level, const std::string& tag, const std::string& message) {
        if (!enabled || level < minLevel) {
            return;
        }
        
        std::lock_guard<std::mutex> lock(mutex);
        
        if (callback) {
            callback(level, tag, message);
        } else {
            // Default console output
            std::string timestamp = getCurrentTimestamp();
            std::string levelStr = levelToString(level);
            std::cout << "[" << timestamp << "] [" << levelStr << "] " 
                     << tag << ": " << message << std::endl;
        }
    }
    
private:
    std::string getCurrentTimestamp() {
        auto now = std::chrono::system_clock::now();
        auto time_t = std::chrono::system_clock::to_time_t(now);
        auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(
            now.time_since_epoch()) % 1000;
        
        std::stringstream ss;
        ss << std::put_time(std::localtime(&time_t), "%H:%M:%S");
        ss << '.' << std::setfill('0') << std::setw(3) << ms.count();
        return ss.str();
    }
    
    std::string levelToString(LogLevel level) {
        switch (level) {
            case LogLevel::VERBOSE: return "V";
            case LogLevel::DEBUG: return "D";
            case LogLevel::INFO: return "I";
            case LogLevel::WARN: return "W";
            case LogLevel::ERROR: return "E";
            default: return "?";
        }
    }
    
public:
    LogLevel minLevel;
    bool enabled;
    LogCallback callback;
    std::mutex mutex;
};

Logger& Logger::getInstance() {
    static Logger instance;
    return instance;
}

Logger::Logger() : pImpl(std::make_unique<Impl>()) {}

void Logger::verbose(const std::string& tag, const std::string& message) {
    getInstance().log(LogLevel::VERBOSE, tag, message);
}

void Logger::debug(const std::string& tag, const std::string& message) {
    getInstance().log(LogLevel::DEBUG, tag, message);
}

void Logger::info(const std::string& tag, const std::string& message) {
    getInstance().log(LogLevel::INFO, tag, message);
}

void Logger::warn(const std::string& tag, const std::string& message) {
    getInstance().log(LogLevel::WARN, tag, message);
}

void Logger::error(const std::string& tag, const std::string& message) {
    getInstance().log(LogLevel::ERROR, tag, message);
}

void Logger::setCallback(LogCallback callback) {
    pImpl->callback = callback;
}

void Logger::setMinLevel(LogLevel level) {
    pImpl->minLevel = level;
}

void Logger::setEnabled(bool enabled) {
    pImpl->enabled = enabled;
}

void Logger::log(LogLevel level, const std::string& tag, const std::string& message) {
    pImpl->log(level, tag, message);
}

} // namespace IPLoop