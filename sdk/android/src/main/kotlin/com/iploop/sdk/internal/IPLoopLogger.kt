package com.iploop.sdk.internal

import android.util.Log

/**
 * Internal logging utility for IPLoop SDK
 * Handles debug/production logging with proper tagging
 */
internal object IPLoopLogger {
    
    private const val TAG_PREFIX = "IPLoop"
    private var debugMode = false
    
    /**
     * Set debug mode
     */
    fun setDebugMode(enabled: Boolean) {
        debugMode = enabled
    }
    
    /**
     * Log debug message
     */
    fun d(tag: String, message: String) {
        if (debugMode) {
            Log.d("$TAG_PREFIX.$tag", message)
        }
    }
    
    /**
     * Log info message
     */
    fun i(tag: String, message: String) {
        Log.i("$TAG_PREFIX.$tag", message)
    }
    
    /**
     * Log warning message
     */
    fun w(tag: String, message: String) {
        Log.w("$TAG_PREFIX.$tag", message)
    }
    
    /**
     * Log error message
     */
    fun e(tag: String, message: String, throwable: Throwable? = null) {
        if (throwable != null) {
            Log.e("$TAG_PREFIX.$tag", message, throwable)
        } else {
            Log.e("$TAG_PREFIX.$tag", message)
        }
    }
    
    /**
     * Log verbose message
     */
    fun v(tag: String, message: String) {
        if (debugMode) {
            Log.v("$TAG_PREFIX.$tag", message)
        }
    }
}