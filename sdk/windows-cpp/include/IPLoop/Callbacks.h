#pragma once

#include "Types.h"
#include <functional>

namespace IPLoop {

/**
 * Callback function types - matches Android SDK callback patterns
 */

/**
 * General success/error callback
 */
using StatusCallback = std::function<void(bool success, const std::string& message)>;

/**
 * SDK status change callback
 */
using StatusChangeCallback = std::function<void(SDKStatus oldStatus, SDKStatus newStatus)>;

/**
 * Bandwidth update callback - called periodically with current stats
 */
using BandwidthUpdateCallback = std::function<void(const BandwidthStats& stats)>;

/**
 * Error callback - called when SDK encounters errors
 */
using ErrorCallback = std::function<void(const ErrorInfo& error)>;

/**
 * Connection state callback
 */
using ConnectionCallback = std::function<void(ConnectionStatus status, const std::string& message)>;

/**
 * Tunnel event callbacks
 */
using TunnelCreatedCallback = std::function<void(const std::string& sessionId)>;
using TunnelClosedCallback = std::function<void(const std::string& sessionId, uint64_t bytesTransferred)>;

/**
 * Log callback for custom logging
 */
using LogCallback = std::function<void(LogLevel level, const std::string& tag, const std::string& message)>;

/**
 * Progress callback for operations that take time
 */
using ProgressCallback = std::function<void(int percentage, const std::string& status)>;

/**
 * Proxy configuration callback - called when proxy settings are available
 */
using ProxyConfigCallback = std::function<void(const std::string& host, int httpPort, int socksPort)>;

} // namespace IPLoop