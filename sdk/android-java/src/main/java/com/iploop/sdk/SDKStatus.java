package com.iploop.sdk;

/**
 * SDK Status constants (using int instead of enum for d8 compatibility)
 */
public final class SDKStatus {
    public static final int IDLE = 0;
    public static final int INITIALIZED = 1;
    public static final int CONNECTING = 2;
    public static final int RUNNING = 3;
    public static final int STOPPED = 4;
    public static final int ERROR = 5;
    
    private SDKStatus() {} // Prevent instantiation
    
    public static String toString(int status) {
        switch (status) {
            case IDLE: return "IDLE";
            case INITIALIZED: return "INITIALIZED";
            case CONNECTING: return "CONNECTING";
            case RUNNING: return "RUNNING";
            case STOPPED: return "STOPPED";
            case ERROR: return "ERROR";
            default: return "UNKNOWN";
        }
    }
}
