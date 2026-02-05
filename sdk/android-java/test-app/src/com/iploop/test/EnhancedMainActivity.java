package com.iploop.test;

import android.app.Activity;
import android.os.Bundle;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.Spinner;
import android.widget.TextView;
import android.widget.ArrayAdapter;
import android.widget.Toast;
import android.widget.CheckBox;
import android.util.Log;

import com.iploop.sdk.IPLoopSDK;
import com.iploop.sdk.IPLoopSDK.ProxyConfig;
import com.iploop.sdk.IPLoopSDK.Callback;
import com.iploop.sdk.IPLoopSDK.ProxyCallback;
import com.iploop.sdk.IPLoopSDK.StatusCallback;

/**
 * Enhanced Test Activity for IPLoop SDK v1.0.20
 * Tests all new enterprise proxy features
 */
public class EnhancedMainActivity extends Activity {
    private static final String TAG = "IPLoopEnhancedTest";
    
    // UI Elements
    private EditText customerIdInput;
    private EditText apiKeyInput;
    private Spinner countrySpinner;
    private EditText cityInput;
    private EditText asnInput;
    private EditText sessionIdInput;
    private Spinner sessionTypeSpinner;
    private EditText lifetimeInput;
    private Spinner rotateModeSpinner;
    private EditText rotateIntervalInput;
    private Spinner profileSpinner;
    private EditText userAgentInput;
    private EditText minSpeedInput;
    private EditText maxLatencyInput;
    private CheckBox debugCheckBox;
    private TextView statusText;
    private TextView authStringText;
    private TextView proxyUrlsText;
    private Button configureButton;
    private Button testProxyButton;
    private Button startSdkButton;
    private Button stopSdkButton;
    
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.enhanced_activity_main);
        
        initializeViews();
        setupSpinners();
        setupCallbacks();
        
        // Initialize SDK
        IPLoopSDK.init(this, "test_api_key");
        IPLoopSDK.setLoggingEnabled(true);
        IPLoopSDK.setConsentGiven(true);
        
        updateStatus("SDK initialized - v" + IPLoopSDK.getVersion());
        Log.i(TAG, "Enhanced test activity started with SDK v" + IPLoopSDK.getVersion());
    }
    
    private void initializeViews() {
        customerIdInput = findViewById(R.id.customerIdInput);
        apiKeyInput = findViewById(R.id.apiKeyInput);
        countrySpinner = findViewById(R.id.countrySpinner);
        cityInput = findViewById(R.id.cityInput);
        asnInput = findViewById(R.id.asnInput);
        sessionIdInput = findViewById(R.id.sessionIdInput);
        sessionTypeSpinner = findViewById(R.id.sessionTypeSpinner);
        lifetimeInput = findViewById(R.id.lifetimeInput);
        rotateModeSpinner = findViewById(R.id.rotateModeSpinner);
        rotateIntervalInput = findViewById(R.id.rotateIntervalInput);
        profileSpinner = findViewById(R.id.profileSpinner);
        userAgentInput = findViewById(R.id.userAgentInput);
        minSpeedInput = findViewById(R.id.minSpeedInput);
        maxLatencyInput = findViewById(R.id.maxLatencyInput);
        debugCheckBox = findViewById(R.id.debugCheckBox);
        statusText = findViewById(R.id.statusText);
        authStringText = findViewById(R.id.authStringText);
        proxyUrlsText = findViewById(R.id.proxyUrlsText);
        configureButton = findViewById(R.id.configureButton);
        testProxyButton = findViewById(R.id.testProxyButton);
        startSdkButton = findViewById(R.id.startSdkButton);
        stopSdkButton = findViewById(R.id.stopSdkButton);
        
        // Set default values
        customerIdInput.setText("test_customer_123");
        apiKeyInput.setText("test_api_key_xyz");
        cityInput.setText("miami");
        sessionIdInput.setText("test_session_" + System.currentTimeMillis());
        lifetimeInput.setText("30");
        rotateIntervalInput.setText("5");
        minSpeedInput.setText("50");
        maxLatencyInput.setText("200");
    }
    
    private void setupSpinners() {
        // Country spinner
        String[] countries = {"", "US", "GB", "DE", "FR", "JP", "CA", "AU", "BR", "IN", "CN"};
        ArrayAdapter<String> countryAdapter = new ArrayAdapter<>(this, android.R.layout.simple_spinner_item, countries);
        countryAdapter.setDropDownViewResource(android.R.layout.simple_spinner_dropdown_item);
        countrySpinner.setAdapter(countryAdapter);
        countrySpinner.setSelection(1); // Default to US
        
        // Session type spinner
        String[] sessionTypes = {"sticky", "rotating", "per-request"};
        ArrayAdapter<String> sessionAdapter = new ArrayAdapter<>(this, android.R.layout.simple_spinner_item, sessionTypes);
        sessionAdapter.setDropDownViewResource(android.R.layout.simple_spinner_dropdown_item);
        sessionTypeSpinner.setAdapter(sessionAdapter);
        
        // Rotate mode spinner
        String[] rotateModes = {"manual", "request", "time", "ip-change"};
        ArrayAdapter<String> rotateAdapter = new ArrayAdapter<>(this, android.R.layout.simple_spinner_item, rotateModes);
        rotateAdapter.setDropDownViewResource(android.R.layout.simple_spinner_dropdown_item);
        rotateModeSpinner.setAdapter(rotateAdapter);
        
        // Profile spinner
        String[] profiles = {"chrome-win", "firefox-mac", "safari-mac", "mobile-ios", "mobile-android"};
        ArrayAdapter<String> profileAdapter = new ArrayAdapter<>(this, android.R.layout.simple_spinner_item, profiles);
        profileAdapter.setDropDownViewResource(android.R.layout.simple_spinner_dropdown_item);
        profileSpinner.setAdapter(profileAdapter);
    }
    
    private void setupCallbacks() {
        // Status callback
        IPLoopSDK.setStatusCallback(new StatusCallback() {
            @Override
            public void onStatusChanged(int newStatus) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        updateStatus("Status: " + com.iploop.sdk.SDKStatus.toString(newStatus));
                    }
                });
            }
        });
        
        // Proxy callback
        IPLoopSDK.setProxyCallback(new ProxyCallback() {
            @Override
            public void onProxyConfigured(String proxyHost, int proxyPort) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        Toast.makeText(EnhancedMainActivity.this, 
                            "Proxy configured: " + proxyHost + ":" + proxyPort, 
                            Toast.LENGTH_SHORT).show();
                    }
                });
            }
            
            @Override
            public void onProxyError(String error) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        Toast.makeText(EnhancedMainActivity.this, 
                            "Proxy error: " + error, 
                            Toast.LENGTH_LONG).show();
                    }
                });
            }
        });
        
        // Configure button
        configureButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                configureProxy();
            }
        });
        
        // Test proxy button
        testProxyButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                testProxy();
            }
        });
        
        // Start SDK button
        startSdkButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                startSdk();
            }
        });
        
        // Stop SDK button
        stopSdkButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                stopSdk();
            }
        });
    }
    
    private void configureProxy() {
        try {
            ProxyConfig config = new ProxyConfig();
            
            // Get values from UI
            String country = countrySpinner.getSelectedItem().toString();
            if (!country.isEmpty()) {
                config.setCountry(country);
            }
            
            String city = cityInput.getText().toString().trim();
            if (!city.isEmpty()) {
                config.setCity(city);
            }
            
            String asnText = asnInput.getText().toString().trim();
            if (!asnText.isEmpty()) {
                config.setASN(Integer.parseInt(asnText));
            }
            
            String sessionId = sessionIdInput.getText().toString().trim();
            if (!sessionId.isEmpty()) {
                config.setSessionId(sessionId);
            }
            
            config.setSessionType(sessionTypeSpinner.getSelectedItem().toString());
            
            String lifetimeText = lifetimeInput.getText().toString().trim();
            if (!lifetimeText.isEmpty()) {
                config.setLifetime(Integer.parseInt(lifetimeText));
            }
            
            config.setRotateMode(rotateModeSpinner.getSelectedItem().toString());
            
            String intervalText = rotateIntervalInput.getText().toString().trim();
            if (!intervalText.isEmpty()) {
                config.setRotateInterval(Integer.parseInt(intervalText));
            }
            
            config.setProfile(profileSpinner.getSelectedItem().toString());
            
            String userAgent = userAgentInput.getText().toString().trim();
            if (!userAgent.isEmpty()) {
                config.setUserAgent(userAgent);
            }
            
            String speedText = minSpeedInput.getText().toString().trim();
            if (!speedText.isEmpty()) {
                config.setMinSpeed(Integer.parseInt(speedText));
            }
            
            String latencyText = maxLatencyInput.getText().toString().trim();
            if (!latencyText.isEmpty()) {
                config.setMaxLatency(Integer.parseInt(latencyText));
            }
            
            config.setDebugMode(debugCheckBox.isChecked());
            
            // Apply configuration
            IPLoopSDK.configureProxy(config);
            
            // Update UI with generated auth string
            String customerId = customerIdInput.getText().toString().trim();
            String apiKey = apiKeyInput.getText().toString().trim();
            
            String authString = config.generateProxyAuth(customerId, apiKey);
            authStringText.setText("Auth String:\n" + authString);
            
            // Update proxy URLs
            String httpUrl = IPLoopSDK.getHttpProxyUrl(customerId, apiKey);
            String socks5Url = IPLoopSDK.getSocks5ProxyUrl(customerId, apiKey);
            
            proxyUrlsText.setText("HTTP Proxy:\n" + httpUrl + "\n\nSOCKS5 Proxy:\n" + socks5Url);
            
            updateStatus("Proxy configuration applied successfully");
            Toast.makeText(this, "Proxy configured!", Toast.LENGTH_SHORT).show();
            
        } catch (Exception e) {
            String error = "Configuration error: " + e.getMessage();
            updateStatus(error);
            Toast.makeText(this, error, Toast.LENGTH_LONG).show();
            Log.e(TAG, error, e);
        }
    }
    
    private void testProxy() {
        String customerId = customerIdInput.getText().toString().trim();
        String apiKey = apiKeyInput.getText().toString().trim();
        
        if (customerId.isEmpty() || apiKey.isEmpty()) {
            Toast.makeText(this, "Customer ID and API key required", Toast.LENGTH_SHORT).show();
            return;
        }
        
        updateStatus("Testing proxy connection...");
        
        IPLoopSDK.testProxy(customerId, apiKey, new Callback() {
            @Override
            public void onSuccess() {
                updateStatus("Proxy test successful!");
                Toast.makeText(EnhancedMainActivity.this, "Proxy test passed!", Toast.LENGTH_SHORT).show();
            }
            
            @Override
            public void onError(String error) {
                String msg = "Proxy test failed: " + error;
                updateStatus(msg);
                Toast.makeText(EnhancedMainActivity.this, msg, Toast.LENGTH_LONG).show();
            }
        });
    }
    
    private void startSdk() {
        updateStatus("Starting SDK...");
        
        IPLoopSDK.start(new Runnable() {
            @Override
            public void run() {
                updateStatus("SDK started successfully!");
                Toast.makeText(EnhancedMainActivity.this, "SDK started!", Toast.LENGTH_SHORT).show();
            }
        }, new Callback() {
            @Override
            public void onSuccess() {
                // Already handled by runnable
            }
            
            @Override
            public void onError(String error) {
                String msg = "SDK start failed: " + error;
                updateStatus(msg);
                Toast.makeText(EnhancedMainActivity.this, msg, Toast.LENGTH_LONG).show();
            }
        });
    }
    
    private void stopSdk() {
        updateStatus("Stopping SDK...");
        
        IPLoopSDK.stop(new Runnable() {
            @Override
            public void run() {
                updateStatus("SDK stopped");
                Toast.makeText(EnhancedMainActivity.this, "SDK stopped", Toast.LENGTH_SHORT).show();
            }
        });
    }
    
    private void updateStatus(final String status) {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                statusText.setText(status);
                Log.i(TAG, status);
            }
        });
    }
    
    @Override
    protected void onDestroy() {
        super.onDestroy();
        IPLoopSDK.stop();
    }
}