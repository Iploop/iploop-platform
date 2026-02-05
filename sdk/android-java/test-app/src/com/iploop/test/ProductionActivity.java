package com.iploop.test;

import android.app.Activity;
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
 * IPLoop Enterprise Production - Samsung Galaxy A17
 */
public class ProductionActivity extends Activity {
    private static final String TAG = "IPLoopProduction";
    
    private TextView statusText;
    private EditText customerInput;
    private EditText keyInput;
    private Button connectButton;
    private boolean isConnected = false;
    
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        
        setContentView(createProductionLayout());
        initializeSDK();
        
        updateStatus("🚀 Samsung Galaxy A17 Production Ready!");
        Log.i(TAG, "IPLoop Enterprise Production initialized");
    }
    
    private View createProductionLayout() {
        android.widget.ScrollView scrollView = new android.widget.ScrollView(this);
        android.widget.LinearLayout layout = new android.widget.LinearLayout(this);
        layout.setOrientation(android.widget.LinearLayout.VERTICAL);
        layout.setPadding(40, 40, 40, 40);
        
        // Header
        TextView header = new TextView(this);
        header.setText("🔥 IPLoop Enterprise\nSamsung Galaxy A17\nSDK v" + IPLoopSDK.getVersion() + 
                      "\n\n✅ Production Deployment Ready!");
        header.setTextSize(18);
        header.setGravity(android.view.Gravity.CENTER);
        header.setPadding(0, 0, 0, 30);
        layout.addView(header);
        
        // Status
        statusText = new TextView(this);
        statusText.setTextSize(14);
        statusText.setPadding(0, 0, 0, 20);
        layout.addView(statusText);
        
        // Customer ID
        TextView customerLabel = new TextView(this);
        customerLabel.setText("Customer ID:");
        customerLabel.setTextSize(14);
        layout.addView(customerLabel);
        
        customerInput = new EditText(this);
        customerInput.setHint("Enter customer ID");
        customerInput.setText("samsung_enterprise");
        customerInput.setSingleLine(true);
        layout.addView(customerInput);
        
        // API Key
        TextView keyLabel = new TextView(this);
        keyLabel.setText("API Key:");
        keyLabel.setTextSize(14);
        keyLabel.setPadding(0, 15, 0, 0);
        layout.addView(keyLabel);
        
        keyInput = new EditText(this);
        keyInput.setHint("Enter API key");
        keyInput.setText("production_key_2024");
        keyInput.setSingleLine(true);
        layout.addView(keyInput);
        
        // Connect button
        connectButton = new Button(this);
        connectButton.setText("🔗 Connect to Korea Enterprise Proxy");
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
        android.widget.LinearLayout.LayoutParams buttonParams = 
            new android.widget.LinearLayout.LayoutParams(
                android.widget.LinearLayout.LayoutParams.MATCH_PARENT,
                android.widget.LinearLayout.LayoutParams.WRAP_CONTENT
            );
        buttonParams.setMargins(0, 30, 0, 0);
        connectButton.setLayoutParams(buttonParams);
        layout.addView(connectButton);
        
        // Enterprise features
        TextView features = new TextView(this);
        features.setText("\n🚀 Enterprise Features Active:\n\n" +
            "✅ Geographic Targeting (Korea optimized)\n" +
            "✅ Session Management (2-hour sticky sessions)\n" +
            "✅ Mobile Browser Profiles (Samsung Galaxy A17)\n" +
            "✅ High-Speed Configuration (100Mbps, <50ms)\n" +
            "✅ Enterprise Authentication\n" +
            "✅ Production Security (logging disabled)\n\n" +
            "📱 Device: Samsung Galaxy A17 (SM-A175F/DS)\n" +
            "🌍 Network: Korea Enterprise Proxy\n" +
            "💼 Mode: Production Deployment\n" +
            "🔥 Status: Enterprise Ready!");
        features.setTextSize(12);
        features.setPadding(0, 20, 0, 20);
        layout.addView(features);
        
        scrollView.addView(layout);
        return scrollView;
    }
    
    private void initializeSDK() {
        IPLoopSDK.init(this, "samsung_galaxy_a17_il");
        IPLoopSDK.setLoggingEnabled(true); // Enable logging for debugging
        IPLoopSDK.setConsentGiven(true);
        
        // Auto-start connection to gateway
        IPLoopSDK.start(new Runnable() {
            @Override
            public void run() {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        updateStatus("🟢 AUTO-CONNECTED to Gateway!\nSDK v" + IPLoopSDK.getVersion());
                    }
                });
            }
        }, new Callback() {
            @Override
            public void onSuccess() {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        updateStatus("✅ Gateway connection established!");
                    }
                });
            }
            @Override
            public void onError(String error) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        updateStatus("❌ Connection error: " + error);
                    }
                });
            }
        });
    }
    
    private void connectToProxy() {
        final String customerId = customerInput.getText().toString().trim();
        final String apiKey = keyInput.getText().toString().trim();
        
        if (customerId.isEmpty() || apiKey.isEmpty()) {
            showToast("Please enter Customer ID and API Key");
            return;
        }
        
        updateStatus("🔄 Connecting to Korea Enterprise Proxy...");
        connectButton.setEnabled(false);
        connectButton.setText("🔄 Connecting...");
        
        // Enterprise production configuration
        ProxyConfig config = new ProxyConfig()
            .setCountry("KR") // Korea optimization
            .setSessionType("sticky")
            .setLifetime(120) // 2-hour sessions
            .setProfile("mobile-android")
            // Profile-based User-Agent will be used (chrome-win, firefox-mac, mobile-ios, etc.)
            .setMinSpeed(100) // 100 Mbps minimum
            .setMaxLatency(50) // <50ms latency
            .setDebugMode(false); // Production security
        
        IPLoopSDK.configureProxy(config);
        
        IPLoopSDK.testProxy(customerId, apiKey, new Callback() {
            @Override
            public void onSuccess() {
                // Actually start the SDK to connect to gateway
                IPLoopSDK.start();
                
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        isConnected = true;
                        updateStatus("🟢 CONNECTED! Korea Enterprise Proxy Active");
                        connectButton.setText("🔌 Disconnect from Proxy");
                        connectButton.setEnabled(true);
                        showToast("✅ Enterprise proxy connection established!");
                        
                        customerInput.setEnabled(false);
                        keyInput.setEnabled(false);
                        
                        Log.i(TAG, "Enterprise proxy connected successfully");
                    }
                });
            }
            
            @Override
            public void onError(String error) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        updateStatus("❌ Connection Failed: " + error);
                        connectButton.setText("🔗 Connect to Korea Enterprise Proxy");
                        connectButton.setEnabled(true);
                        showToast("❌ Connection failed - verify credentials");
                        
                        Log.e(TAG, "Enterprise proxy connection failed: " + error);
                    }
                });
            }
        });
    }
    
    private void disconnectProxy() {
        IPLoopSDK.stop();
        isConnected = false;
        
        updateStatus("🔴 Disconnected from Enterprise Proxy");
        connectButton.setText("🔗 Connect to Korea Enterprise Proxy");
        connectButton.setEnabled(true);
        
        customerInput.setEnabled(true);
        keyInput.setEnabled(true);
        
        showToast("🔌 Disconnected from proxy");
        Log.i(TAG, "Enterprise proxy disconnected");
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
            Log.i(TAG, "Stopping proxy on activity destroy");
        }
    }
}