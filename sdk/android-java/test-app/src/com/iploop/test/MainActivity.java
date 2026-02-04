package com.iploop.test;

import android.app.Activity;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;
import android.widget.TextView;
import android.widget.ScrollView;
import dalvik.system.DexClassLoader;
import java.io.File;
import java.lang.reflect.Method;

public class MainActivity extends Activity {
    private static final String TAG = "IPLoopTest";
    private StringBuilder logBuilder = new StringBuilder();
    private TextView logView;
    private Class<?> sdkClass;
    
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        
        ScrollView scroll = new ScrollView(this);
        logView = new TextView(this);
        logView.setPadding(20, 20, 20, 20);
        logView.setTextSize(14);
        scroll.addView(logView);
        setContentView(scroll);
        
        new Thread(new Runnable() {
            public void run() {
                runTests();
            }
        }).start();
    }
    
    private void log(String msg) {
        Log.d(TAG, msg);
        logBuilder.append(msg).append("\n");
        runOnUiThread(new Runnable() {
            public void run() {
                logView.setText(logBuilder.toString());
            }
        });
    }
    
    private void runTests() {
        log("=== IPLoop SDK Test ===\n");
        
        try {
            // 1. Load DEX
            String dexPath = "/data/local/tmp/iploop-sdk-1.0.6-pure.dex";
            File dexFile = new File(dexPath);
            
            if (!dexFile.exists()) {
                log("ERROR: DEX not found at " + dexPath);
                return;
            }
            
            log("[1] Loading DEX...");
            File optDir = getDir("dex", 0);
            DexClassLoader loader = new DexClassLoader(
                dexPath,
                optDir.getAbsolutePath(),
                null,
                getClassLoader()
            );
            log("    ✓ DEX loaded");
            
            // 2. Load class
            log("\n[2] Loading IPLoopSDK class...");
            sdkClass = loader.loadClass("com.iploop.sdk.IPLoopSDK");
            log("    ✓ Class: " + sdkClass.getName());
            
            // 3. Test getVersion
            log("\n[3] getVersion()...");
            Method getVersion = sdkClass.getMethod("getVersion");
            String version = (String) getVersion.invoke(null);
            log("    ✓ Version: " + version);
            
            // 4. Test getStatus before init
            log("\n[4] getStatus() before init...");
            Method getStatus = sdkClass.getMethod("getStatus");
            int status = (Integer) getStatus.invoke(null);
            log("    ✓ Status: " + status + " (0=IDLE)");
            
            // 5. Test isRunning before init
            log("\n[5] isRunning() before init...");
            Method isRunning = sdkClass.getMethod("isRunning");
            boolean running = (Boolean) isRunning.invoke(null);
            log("    ✓ Running: " + running);
            
            // 6. Test init
            log("\n[6] init(context, key)...");
            Method init = sdkClass.getMethod("init", android.content.Context.class, String.class);
            init.invoke(null, this, "test-api-key-12345");
            log("    ✓ Initialized");
            
            // 7. Check status after init
            log("\n[7] getStatus() after init...");
            status = (Integer) getStatus.invoke(null);
            log("    ✓ Status: " + status + " (1=INITIALIZED)");
            
            // 8. Test setConsentGiven
            log("\n[8] setConsentGiven(true)...");
            Method setConsent = sdkClass.getMethod("setConsentGiven", boolean.class);
            setConsent.invoke(null, true);
            log("    ✓ Consent granted");
            
            // 9. Test start (will try to connect)
            log("\n[9] start()...");
            Method start = sdkClass.getMethod("start");
            start.invoke(null);
            log("    ✓ Start called (connecting in background)");
            
            // Wait for proxy testing (30 seconds)
            log("    ⏳ Waiting 30s for proxy testing...");
            Thread.sleep(30000);
            
            // 10. Check status after start
            log("\n[10] getStatus() after start...");
            status = (Integer) getStatus.invoke(null);
            String statusStr = "UNKNOWN";
            switch(status) {
                case 0: statusStr = "IDLE"; break;
                case 1: statusStr = "INITIALIZED"; break;
                case 2: statusStr = "CONNECTING"; break;
                case 3: statusStr = "RUNNING"; break;
                case 4: statusStr = "STOPPED"; break;
                case 5: statusStr = "ERROR"; break;
            }
            log("    ✓ Status: " + status + " (" + statusStr + ")");
            
            // 11. Check isRunning after start
            log("\n[11] isRunning() after start...");
            running = (Boolean) isRunning.invoke(null);
            log("    ✓ Running: " + running);
            
            // 12. Test stop
            log("\n[12] stop()...");
            Method stop = sdkClass.getMethod("stop");
            stop.invoke(null);
            log("    ✓ Stop called");
            
            Thread.sleep(500);
            
            // 13. Check status after stop
            log("\n[13] getStatus() after stop...");
            status = (Integer) getStatus.invoke(null);
            switch(status) {
                case 0: statusStr = "IDLE"; break;
                case 1: statusStr = "INITIALIZED"; break;
                case 2: statusStr = "CONNECTING"; break;
                case 3: statusStr = "RUNNING"; break;
                case 4: statusStr = "STOPPED"; break;
                case 5: statusStr = "ERROR"; break;
            }
            log("    ✓ Status: " + status + " (" + statusStr + ")");
            
            // 14. Check isRunning after stop
            log("\n[14] isRunning() after stop...");
            running = (Boolean) isRunning.invoke(null);
            log("    ✓ Running: " + running);
            
            log("\n=== ALL TESTS COMPLETE ===");
            
        } catch (Exception e) {
            log("\nERROR: " + e.getClass().getSimpleName());
            log("Message: " + e.getMessage());
            if (e.getCause() != null) {
                log("Cause: " + e.getCause().getMessage());
            }
            e.printStackTrace();
        }
    }
}
