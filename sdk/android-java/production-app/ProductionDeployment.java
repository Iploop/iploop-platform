package com.iploop.production;

import android.app.Activity;
import android.content.Context;
import android.content.SharedPreferences;
import android.os.Bundle;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.TextView;
import android.widget.Toast;
import android.util.Log;

import com.iploop.sdk.IPLoopSDK;
import com.iploop.sdk.IPLoopSDK.ProxyConfig;
import com.iploop.sdk.IPLoopSDK.Callback;

/**
 * IPLoop Production Deployment - Samsung Galaxy A17
 * Simplified production-ready activity
 */
public class ProductionDeployment extends Activity {
    private static final String TAG = "IPLoopProduction";
    
    private TextView statusText;
    private EditText customerInput;
    private EditText keyInput;
    private Button connectButton;
    private boolean isConnected = false;
    
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        
        setContentView(createLayout());
        initializeSDK();
        
        updateStatus("🚀 Samsung Galaxy A17 Production Ready!");
        Log.i(TAG, "IPLoop Enterprise Production loaded");
    }
    
    private View createLayout() {
        android.widget.LinearLayout layout = new android.widget.LinearLayout(this);
        layout.setOrientation(android.widget.LinearLayout.VERTICAL);
        layout.setPadding(50, 50, 50, 50);
        
        // Header
        TextView header = new TextView(this);
        header.setText("🔥 IPLoop Enterprise\nSamsung Galaxy A17\nSDK v1.0.20");
        header.setTextSize(20);
        header.setGravity(android.view.Gravity.CENTER);
        header.setPadding(0, 0, 0, 30);
        layout.addView(header);
        
        // Status
        statusText = new TextView(this);
        statusText.setTextSize(14);
        layout.addView(statusText);
        
        // Customer ID
        TextView customerLabel = new TextView(this);
        customerLabel.setText("Customer ID:");
        customerLabel.setPadding(0, 20, 0, 5);
        layout.addView(customerLabel);
        
        customerInput = new EditText(this);
        customerInput.setHint("Enter customer ID");
        customerInput.setText("samsung_enterprise");
        layout.addView(customerInput);
        
        // API Key
        TextView keyLabel = new TextView(this);
        keyLabel.setText("API Key:");
        keyLabel.setPadding(0, 10, 0, 5);
        layout.addView(keyLabel);
        
        keyInput = new EditText(this);
        keyInput.setHint("Enter API key");
        keyInput.setText("production_key_2024");
        layout.addView(keyInput);
        
        // Connect button
        connectButton = new Button(this);
        connectButton.setText("Connect to Korea Proxy");
        connectButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                if (!isConnected) {
                    connectToProxy();
                } else {
                    disconnectProxy();
                }
            }
        });
        connectButton.setPadding(0, 30, 0, 0);
        layout.addView(connectButton);
        
        // Features info
        TextView features = new TextView(this);
        features.setText("\n✅ Enterprise Features Active:\n" +
            "• Geographic targeting (Korea optimized)\n" +
            "• Session management (sticky/rotating)\n" +
            "• Mobile browser profiles\n" +
            "• High-speed configurations\n" +
            "• Enterprise authentication\n\n" +
            "📱 Device: Samsung Galaxy A17\n" +
            "🌍 Network: Korea proxy ready\n" +
            "🔥 Status: Production deployed!");
        features.setTextSize(12);
        features.setPadding(0, 20, 0, 0);
        layout.addView(features);
        
        return layout;
    }
    
    private void initializeSDK() {
        IPLoopSDK.init(this, "samsung_enterprise_production");
        IPLoopSDK.setLoggingEnabled(false); // Production = no logging
        IPLoopSDK.setConsentGiven(true);
    }
    
    private void connectToProxy() {
        final String customerId = customerInput.getText().toString().trim();
        final String apiKey = keyInput.getText().toString().trim();
        
        if (customerId.isEmpty() || apiKey.isEmpty()) {
            showToast("Please enter credentials");
            return;
        }
        
        updateStatus("🔄 Connecting to Korea proxy...");
        connectButton.setEnabled(false);
        
        // Production enterprise configuration
        ProxyConfig config = new ProxyConfig()
            .setCountry("KR")
            .setSessionType("sticky")
            .setLifetime(120) // 2 hours
            .setProfile("mobile-android")
            .setUserAgent("Samsung/Galaxy-A17 Enterprise Production")
            .setMinSpeed(100)
            .setMaxLatency(50)
            .setDebugMode(false);
        
        IPLoopSDK.configureProxy(config);
        
        IPLoopSDK.testProxy(customerId, apiKey, new Callback() {
            @Override
            public void onSuccess() {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        isConnected = true;
                        updateStatus("🟢 Connected! Korea proxy active");
                        connectButton.setText("Disconnect");
                        connectButton.setEnabled(true);
                        showToast("✅ Enterprise proxy connected!");
                        
                        customerInput.setEnabled(false);
                        keyInput.setEnabled(false);
                    }
                });
            }
            
            @Override
            public void onError(String error) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        updateStatus("❌ Connection failed: " + error);
                        connectButton.setEnabled(true);
                        showToast("Connection failed - check credentials");
                    }
                });
            }
        });
    }
    
    private void disconnectProxy() {
        IPLoopSDK.stop();
        isConnected = false;
        
        updateStatus("🔴 Disconnected from proxy");
        connectButton.setText("Connect to Korea Proxy");
        customerInput.setEnabled(true);
        keyInput.setEnabled(true);
        
        showToast("Disconnected");
    }
    
    private void updateStatus(String status) {
        statusText.setText("Status: " + status);
    }
    
    private void showToast(String message) {
        Toast.makeText(this, message, Toast.LENGTH_SHORT).show();
    }
    
    @Override
    protected void onDestroy() {
        super.onDestroy();
        if (isConnected) {
            IPLoopSDK.stop();
        }
    }
}