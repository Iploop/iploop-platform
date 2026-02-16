#pragma once

#include "../include/IPLoop/Types.h"
#include "../include/IPLoop/Callbacks.h"
#include <string>
#include <memory>

namespace IPLoop {

/**
 * Thread-safe logging system - matches Android SDK logging
 */
class Logger {
public:
    static Logger& getInstance();
    
    // Log methods
    static void verbose(const std::string& tag, const std::string& message);
    static void debug(const std::string& tag, const std::string& message);
    static void info(const std::string& tag, const std::string& message);
    static void warn(const std::string& tag, const std::string& message);
    static void error(const std::string& tag, const std::string& message);
    
    // Configuration
    void setCallback(LogCallback callback);
    void setMinLevel(LogLevel level);
    void setEnabled(bool enabled);
    
    // Internal log method
    void log(LogLevel level, const std::string& tag, const std::string& message);
    
private:
    Logger();
    ~Logger() = default;
    Logger(const Logger&) = delete;
    Logger& operator=(const Logger&) = delete;
    
    class Impl;
    std::unique_ptr<Impl> pImpl;
};

} // namespace IPLoop