package com.iploop.production;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.os.Bundle;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.TextView;
import android.widget.Toast;
import android.util.Log;
import android.net.ConnectivityManager;
import android.net.NetworkInfo;

import com.iploop.sdk.IPLoopSDK;
import com.iploop.sdk.IPLoopSDK.ProxyConfig;
import com.iploop.sdk.IPLoopSDK.Callback;
import com.iploop.sdk.IPLoopSDK.StatusCallback;

/**
 * IPLoop Enterprise Production Activity
 * Samsung Galaxy A17 - Production Ready
 * SDK v1.0.20 Enterprise Features
 */
public class ProductionActivity extends Activity {
    private static final String TAG = "IPLoopProduction";
    private static final String PREFS_NAME = "IPLoopSettings";
    private static final String KEY_CUSTOMER_ID = "customer_id";
    private static final String KEY_API_KEY = "api_key";
    private static final String KEY_COUNTRY = "country";
    private static final String KEY_AUTO_START = "auto_start";
    
    private TextView statusText;
    private TextView connectionText;
    private EditText customerIdInput;
    private EditText apiKeyInput;
    private EditText countryInput;
    private Button connectButton;
    private Button disconnectButton;
    private Button settingsButton;
    
    private SharedPreferences prefs;
    private boolean isConnected = false;
    
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        
        prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE);
        
        setContentView(createProductionLayout());
        initializeSDK();
        loadSettings();
        
        // Auto-connect if enabled and credentials saved
        if (prefs.getBoolean(KEY_AUTO_START, false) && hasValidCredentials()) {
            connectToProxy();
        }
        
        updateUI();
        Log.i(TAG, "IPLoop Enterprise initialized on Samsung Galaxy A17");
    }
    
    private View createProductionLayout() {
        android.widget.ScrollView scrollView = new android.widget.ScrollView(this);
        scrollView.setFillViewport(true);
        
        android.widget.LinearLayout layout = new android.widget.LinearLayout(this);
        layout.setOrientation(android.widget.LinearLayout.VERTICAL);
        layout.setPadding(40, 40, 40, 40);
        
        // Header
        TextView headerText = new TextView(this);
        headerText.setText("🔥 IPLoop Enterprise\nSamsung Galaxy A17\nSDK v" + IPLoopSDK.getVersion());
        headerText.setTextSize(18);
        headerText.setGravity(android.view.Gravity.CENTER);
        headerText.setPadding(0, 0, 0, 30);
        layout.addView(headerText);
        
        // Status
        statusText = new TextView(this);
        statusText.setText("Status: Initializing...");
        statusText.setTextSize(14);
        statusText.setPadding(0, 0, 0, 10);
        layout.addView(statusText);
        
        connectionText = new TextView(this);
        connectionText.setText("");
        connectionText.setTextSize(12);
        connectionText.setPadding(0, 0, 0, 20);
        layout.addView(connectionText);
        
        // Customer ID
        TextView customerLabel = new TextView(this);
        customerLabel.setText("Customer ID:");
        customerLabel.setTextSize(14);
        layout.addView(customerLabel);
        
        customerIdInput = new EditText(this);
        customerIdInput.setHint("Enter customer ID");
        customerIdInput.setSingleLine(true);
        layout.addView(customerIdInput);
        
        // API Key
        TextView apiKeyLabel = new TextView(this);
        apiKeyLabel.setText("API Key:");
        apiKeyLabel.setTextSize(14);
        apiKeyLabel.setPadding(0, 10, 0, 0);
        layout.addView(apiKeyLabel);
        
        apiKeyInput = new EditText(this);
        apiKeyInput.setHint("Enter API key");
        apiKeyInput.setSingleLine(true);
        apiKeyInput.setInputType(android.text.InputType.TYPE_CLASS_TEXT | 
                                 android.text.InputType.TYPE_TEXT_VARIATION_PASSWORD);
        layout.addView(apiKeyInput);
        
        // Country
        TextView countryLabel = new TextView(this);
        countryLabel.setText("Country (KR, US, GB, DE, JP):");
        countryLabel.setTextSize(14);
        countryLabel.setPadding(0, 10, 0, 0);
        layout.addView(countryLabel);
        
        countryInput = new EditText(this);
        countryInput.setHint("KR");
        countryInput.setSingleLine(true);
        countryInput.setFilters(new android.text.InputFilter[] {
            new android.text.InputFilter.LengthFilter(2),
            new android.text.InputFilter.AllCaps()
        });
        layout.addView(countryInput);
        
        // Buttons
        android.widget.LinearLayout buttonLayout = new android.widget.LinearLayout(this);
        buttonLayout.setOrientation(android.widget.LinearLayout.HORIZONTAL);
        buttonLayout.setPadding(0, 30, 0, 0);
        
        connectButton = new Button(this);
        connectButton.setText("Connect");
        connectButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                connectToProxy();
            }
        });
        buttonLayout.addView(connectButton);
        
        disconnectButton = new Button(this);
        disconnectButton.setText("Disconnect");
        disconnectButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                disconnectProxy();
            }
        });
        buttonLayout.addView(disconnectButton);
        
        settingsButton = new Button(this);
        settingsButton.setText("Settings");
        settingsButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                showSettings();
            }
        });
        buttonLayout.addView(settingsButton);
        
        layout.addView(buttonLayout);
        
        // Info
        TextView infoText = new TextView(this);
        infoText.setText("\n💡 Features:\n" +
            "• Geographic targeting (5+ countries)\n" +
            "• Session management (sticky/rotating)\n" +
            "• Mobile browser profiles\n" +
            "• High-speed configurations\n" +
            "• Enterprise authentication\n\n" +
            "📱 Device: Samsung Galaxy A17\n" +
            "🚀 Ready for production deployment!");
        infoText.setTextSize(11);
        infoText.setPadding(0, 20, 0, 0);
        layout.addView(infoText);
        
        scrollView.addView(layout);
        return scrollView;
    }
    
    private void initializeSDK() {
        IPLoopSDK.init(this, "samsung_production");
        IPLoopSDK.setLoggingEnabled(true);
        IPLoopSDK.setConsentGiven(true);
        
        IPLoopSDK.setStatusCallback(new StatusCallback() {
            @Override
            public void onStatusChanged(int newStatus) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        updateStatusText();
                    }
                });
            }
        });
    }
    
    private void loadSettings() {
        customerIdInput.setText(prefs.getString(KEY_CUSTOMER_ID, ""));
        apiKeyInput.setText(prefs.getString(KEY_API_KEY, ""));
        countryInput.setText(prefs.getString(KEY_COUNTRY, "KR"));
    }
    
    private void saveSettings() {
        prefs.edit()
            .putString(KEY_CUSTOMER_ID, customerIdInput.getText().toString().trim())
            .putString(KEY_API_KEY, apiKeyInput.getText().toString().trim())
            .putString(KEY_COUNTRY, countryInput.getText().toString().trim())
            .apply();
    }
    
    private boolean hasValidCredentials() {
        String customerId = prefs.getString(KEY_CUSTOMER_ID, "");
        String apiKey = prefs.getString(KEY_API_KEY, "");
        return !customerId.isEmpty() && !apiKey.isEmpty();
    }
    
    private void connectToProxy() {
        if (isConnected) {
            showToast("Already connected!");
            return;
        }
        
        final String customerId = customerIdInput.getText().toString().trim();
        final String apiKey = apiKeyInput.getText().toString().trim();
        String countryTemp = countryInput.getText().toString().trim();
        
        if (customerId.isEmpty() || apiKey.isEmpty()) {
            showToast("Please enter Customer ID and API Key");
            return;
        }
        
        if (countryTemp.isEmpty()) {
            countryTemp = "KR"; // Default to Korea
            countryInput.setText(countryTemp);
        }
        
        final String country = countryTemp;
        
        saveSettings();
        
        // Configure enterprise proxy
        ProxyConfig config = new ProxyConfig()
            .setCountry(country)
            .setSessionType("sticky")
            .setLifetime(60)
            .setProfile("mobile-android")
            .setUserAgent("Samsung/Galaxy-A17 Enterprise")
            .setMinSpeed(50)
            .setMaxLatency(100)
            .setDebugMode(false);
        
        IPLoopSDK.configureProxy(config);
        
        // Test connection
        IPLoopSDK.testProxy(customerId, apiKey, new Callback() {
            @Override
            public void onSuccess() {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        isConnected = true;
                        showToast("✅ Connected to " + country + " proxy!");
                        updateUI();
                        
                        // Start background service
                        startService(new Intent(ProductionActivity.this, IPLoopService.class));
                    }
                });
            }
            
            @Override
            public void onError(String error) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        showToast("❌ Connection failed: " + error);
                        updateUI();
                    }
                });
            }
        });
        
        updateUI();
        Log.i(TAG, "Connecting to proxy: " + customerId + " -> " + country);
    }
    
    private void disconnectProxy() {
        if (!isConnected) {
            showToast("Not connected!");
            return;
        }
        
        IPLoopSDK.stop();
        isConnected = false;
        showToast("Disconnected from proxy");
        updateUI();
        
        // Stop background service
        stopService(new Intent(this, IPLoopService.class));
        
        Log.i(TAG, "Disconnected from proxy");
    }
    
    private void showSettings() {
        android.app.AlertDialog.Builder builder = new android.app.AlertDialog.Builder(this);
        builder.setTitle("Settings");
        
        final android.widget.CheckBox autoStartCheck = new android.widget.CheckBox(this);
        autoStartCheck.setText("Auto-connect on startup");
        autoStartCheck.setChecked(prefs.getBoolean(KEY_AUTO_START, false));
        
        builder.setView(autoStartCheck);
        
        builder.setPositiveButton("Save", new android.content.DialogInterface.OnClickListener() {
            @Override
            public void onClick(android.content.DialogInterface dialog, int which) {
                prefs.edit()
                    .putBoolean(KEY_AUTO_START, autoStartCheck.isChecked())
                    .apply();
                showToast("Settings saved");
            }
        });
        
        builder.setNegativeButton("Cancel", null);
        builder.show();
    }
    
    private void updateUI() {
        updateStatusText();
        updateConnectionText();
        
        connectButton.setEnabled(!isConnected);
        disconnectButton.setEnabled(isConnected);
        customerIdInput.setEnabled(!isConnected);
        apiKeyInput.setEnabled(!isConnected);
        countryInput.setEnabled(!isConnected);
    }
    
    private void updateStatusText() {
        String status = isConnected ? "🟢 Connected" : "🔴 Disconnected";
        statusText.setText("Status: " + status + " | SDK: " + IPLoopSDK.getStatusString());
    }
    
    private void updateConnectionText() {
        ConnectivityManager cm = (ConnectivityManager) getSystemService(Context.CONNECTIVITY_SERVICE);
        NetworkInfo activeNetwork = cm.getActiveNetworkInfo();
        
        String networkInfo = "";
        if (activeNetwork != null && activeNetwork.isConnected()) {
            networkInfo = "Network: " + activeNetwork.getTypeName();
            if (activeNetwork.getType() == ConnectivityManager.TYPE_WIFI) {
                networkInfo += " (WiFi)";
            }
        } else {
            networkInfo = "Network: Disconnected";
        }
        
        if (isConnected) {
            String proxyInfo = "\nProxy: " + IPLoopSDK.getProxyHost() + ":" + IPLoopSDK.getHttpProxyPort();
            networkInfo += proxyInfo;
        }
        
        connectionText.setText(networkInfo);
    }
    
    private void showToast(String message) {
        Toast.makeText(this, message, Toast.LENGTH_SHORT).show();
    }
    
    @Override
    protected void onResume() {
        super.onResume();
        updateUI();
    }
    
    @Override
    protected void onDestroy() {
        super.onDestroy();
        if (isConnected && !prefs.getBoolean(KEY_AUTO_START, false)) {
            IPLoopSDK.stop();
        }
        Log.i(TAG, "Production activity destroyed");
    }
}