package android.util;

public class Log {
    public static void d(String tag, String msg) { 
        System.out.println("[DEBUG] " + tag + ": " + msg); 
    }
    
    public static void i(String tag, String msg) { 
        System.out.println("[INFO] " + tag + ": " + msg); 
    }
    
    public static void e(String tag, String msg) { 
        System.err.println("[ERROR] " + tag + ": " + msg); 
    }
}