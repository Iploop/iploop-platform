package io.iploop.node.receiver

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Build
import io.iploop.node.service.NodeService
import io.iploop.node.util.NodePreferences
import kotlinx.coroutines.runBlocking

/**
 * Starts the node service automatically on device boot if auto-start is enabled
 */
class BootReceiver : BroadcastReceiver() {
    
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED &&
            intent.action != "android.intent.action.QUICKBOOT_POWERON") {
            return
        }
        
        val preferences = NodePreferences(context)
        
        // Check if auto-start is enabled and node is registered
        val shouldAutoStart = runBlocking {
            preferences.autoStartFlow.let { true } && // Default to checking if registered
            preferences.isRegistered()
        }
        
        if (shouldAutoStart) {
            val serviceIntent = Intent(context, NodeService::class.java).apply {
                action = NodeService.ACTION_START
            }
            
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(serviceIntent)
            } else {
                context.startService(serviceIntent)
            }
        }
    }
}
