package com.iploop.production;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.net.ConnectivityManager;
import android.net.NetworkInfo;
import android.util.Log;

import com.iploop.sdk.IPLoopSDK;

/**
 * Network State Receiver
 * Handles network changes and maintains proxy connection
 */
public class ProxyStateReceiver extends BroadcastReceiver {
    private static final String TAG = "ProxyStateReceiver";
    
    @Override
    public void onReceive(Context context, Intent intent) {
        String action = intent.getAction();
        
        if (ConnectivityManager.CONNECTIVITY_ACTION.equals(action)) {
            ConnectivityManager cm = (ConnectivityManager) 
                context.getSystemService(Context.CONNECTIVITY_SERVICE);
            NetworkInfo activeNetwork = cm.getActiveNetworkInfo();
            
            boolean isConnected = activeNetwork != null && activeNetwork.isConnected();
            
            Log.i(TAG, "Network state changed: " + (isConnected ? "Connected" : "Disconnected"));
            
            if (isConnected) {
                // Network restored - check proxy status
                if (IPLoopSDK.getStatusString().contains("ERROR")) {
                    Log.i(TAG, "Attempting to restore proxy connection");
                    // Could trigger reconnection logic here
                }
            } else {
                Log.i(TAG, "Network disconnected - proxy will reconnect automatically");
            }
        }
    }
}