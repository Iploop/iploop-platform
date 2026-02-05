package com.iploop.production;

import android.app.Service;
import android.content.Intent;
import android.os.IBinder;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.util.Log;
import android.os.Build;

import com.iploop.sdk.IPLoopSDK;

/**
 * IPLoop Background Service
 * Maintains proxy connection in background
 */
public class IPLoopService extends Service {
    private static final String TAG = "IPLoopService";
    private static final String CHANNEL_ID = "IPLoopChannel";
    private static final int NOTIFICATION_ID = 1001;
    
    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();
        Log.i(TAG, "IPLoop service created");
    }
    
    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        startForeground(NOTIFICATION_ID, createNotification());
        Log.i(TAG, "IPLoop service started in foreground");
        return START_STICKY; // Restart if killed
    }
    
    @Override
    public IBinder onBind(Intent intent) {
        return null; // No binding needed
    }
    
    @Override
    public void onDestroy() {
        super.onDestroy();
        Log.i(TAG, "IPLoop service destroyed");
    }
    
    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                CHANNEL_ID,
                "IPLoop Enterprise",
                NotificationManager.IMPORTANCE_LOW
            );
            channel.setDescription("IPLoop proxy connection service");
            channel.setShowBadge(false);
            
            NotificationManager manager = getSystemService(NotificationManager.class);
            manager.createNotificationChannel(channel);
        }
    }
    
    private Notification createNotification() {
        Intent intent = new Intent(this, ProductionActivity.class);
        PendingIntent pendingIntent = PendingIntent.getActivity(
            this, 0, intent, 
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.M ? 
                PendingIntent.FLAG_IMMUTABLE : 0
        );
        
        return new Notification.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_compass)
            .setContentTitle("🔥 IPLoop Enterprise")
            .setContentText("Proxy connection active on Samsung Galaxy A17")
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build();
    }
}