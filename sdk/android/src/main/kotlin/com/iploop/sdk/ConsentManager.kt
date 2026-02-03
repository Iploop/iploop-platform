package com.iploop.sdk

import android.app.Activity
import android.app.AlertDialog
import android.content.Context
import android.content.Intent
import android.content.SharedPreferences
import android.os.Bundle
import android.webkit.WebView
import android.widget.Button
import android.widget.LinearLayout
import com.iploop.sdk.internal.IPLoopLogger

/**
 * Manages user consent for proxy operations
 * Handles GDPR compliance, privacy notices, and opt-in/opt-out mechanisms
 */
class ConsentManager(
    private val context: Context,
    private val config: IPLoopConfig
) {
    companion object {
        private const val PREFS_NAME = "iploop_consent"
        private const val KEY_CONSENT_GIVEN = "consent_given"
        private const val KEY_CONSENT_TIMESTAMP = "consent_timestamp"
        private const val KEY_CONSENT_VERSION = "consent_version"
        private const val CURRENT_CONSENT_VERSION = 1
    }
    
    private val prefs: SharedPreferences = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
    
    /**
     * Check if user has given valid consent
     */
    fun hasConsent(): Boolean {
        val consentGiven = prefs.getBoolean(KEY_CONSENT_GIVEN, false)
        val consentVersion = prefs.getInt(KEY_CONSENT_VERSION, 0)
        
        // Require re-consent if version has changed
        if (consentVersion < CURRENT_CONSENT_VERSION) {
            IPLoopLogger.i("ConsentManager", "Consent version outdated, requiring re-consent")
            return false
        }
        
        return consentGiven
    }
    
    /**
     * Set user consent
     */
    fun setConsent(consent: Boolean) {
        prefs.edit()
            .putBoolean(KEY_CONSENT_GIVEN, consent)
            .putLong(KEY_CONSENT_TIMESTAMP, System.currentTimeMillis())
            .putInt(KEY_CONSENT_VERSION, CURRENT_CONSENT_VERSION)
            .apply()
            
        IPLoopLogger.i("ConsentManager", "Consent set to: $consent")
    }
    
    /**
     * Show consent dialog to user
     */
    fun showConsentDialog(activityContext: Context) {
        val dialog = createConsentDialog(activityContext)
        dialog.show()
    }
    
    /**
     * Create consent dialog
     */
    private fun createConsentDialog(activityContext: Context): AlertDialog {
        val title = "IPLoop Proxy Service"
        val message = buildConsentMessage()
        
        return AlertDialog.Builder(activityContext)
            .setTitle(title)
            .setMessage(message)
            .setPositiveButton("Accept") { _, _ ->
                setConsent(true)
                IPLoopLogger.i("ConsentManager", "User accepted consent")
            }
            .setNegativeButton("Decline") { _, _ ->
                setConsent(false)
                IPLoopLogger.i("ConsentManager", "User declined consent")
            }
            .setNeutralButton("Learn More") { _, _ ->
                showDetailedConsent(activityContext)
            }
            .setCancelable(false)
            .create()
    }
    
    /**
     * Show detailed consent information
     */
    private fun showDetailedConsent(activityContext: Context) {
        val intent = Intent(activityContext, ConsentActivity::class.java)
        activityContext.startActivity(intent)
    }
    
    /**
     * Build the consent message text
     */
    private fun buildConsentMessage(): String {
        return buildString {
            append("This app uses IPLoop technology to help improve internet connectivity. ")
            append("By accepting, you agree to:\n\n")
            
            append("• Share your internet connection securely\n")
            append("• Allow encrypted traffic routing through your device\n")
            
            if (config.wifiOnly) {
                append("• WiFi connections only (no mobile data)\n")
            }
            
            append("• Maximum ${config.maxBandwidthMB}MB per day\n")
            
            if (config.chargingOnly) {
                append("• Only when device is charging\n")
            }
            
            if (config.minBatteryLevel > 0) {
                append("• Only when battery > ${config.minBatteryLevel}%\n")
            }
            
            append("\nYour privacy is protected:\n")
            append("• All traffic is encrypted\n")
            append("• No personal data is accessed\n")
            append("• You can opt-out anytime\n")
            
            if (!config.shareLocation) {
                append("• Location is not shared\n")
            }
            
            append("\nYou can change these settings anytime in your app preferences.")
        }
    }
    
    /**
     * Get consent timestamp
     */
    fun getConsentTimestamp(): Long {
        return prefs.getLong(KEY_CONSENT_TIMESTAMP, 0)
    }
    
    /**
     * Get consent version
     */
    fun getConsentVersion(): Int {
        return prefs.getInt(KEY_CONSENT_VERSION, 0)
    }
    
    /**
     * Clear all consent data (for testing or reset)
     */
    fun clearConsent() {
        prefs.edit().clear().apply()
        IPLoopLogger.i("ConsentManager", "Consent data cleared")
    }
    
    /**
     * Check if consent dialog should be auto-shown
     */
    fun shouldAutoShowConsent(): Boolean {
        return config.autoShowConsent && !hasConsent()
    }
}

/**
 * Consent Activity - Full screen consent information
 */
class ConsentActivity : Activity() {
    
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        
        // Create simple layout programmatically to avoid resource dependencies
        val webView = WebView(this)
        val button = Button(this)
        
        webView.loadData(getConsentHtml(), "text/html", "UTF-8")
        
        button.text = "Close"
        button.setOnClickListener { finish() }
        
        // Simple vertical layout
        val layout = LinearLayout(this)
        layout.orientation = LinearLayout.VERTICAL
        
        val webViewParams = LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT, 0, 1f)
        layout.addView(webView, webViewParams)
        
        val buttonParams = LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT, 
            LinearLayout.LayoutParams.WRAP_CONTENT)
        layout.addView(button, buttonParams)
        
        setContentView(layout)
    }
    
    private fun getConsentHtml(): String {
        return """
            <html>
            <head>
                <meta name="viewport" content="width=device-width, initial-scale=1">
                <style>
                    body { font-family: sans-serif; padding: 20px; line-height: 1.5; }
                    h1 { color: #2196F3; }
                    h2 { color: #666; margin-top: 30px; }
                    .highlight { background-color: #E3F2FD; padding: 10px; border-radius: 5px; }
                </style>
            </head>
            <body>
                <h1>IPLoop Privacy Notice</h1>
                
                <h2>What is IPLoop?</h2>
                <p>IPLoop is a technology that helps improve internet connectivity by creating a 
                secure network of devices that can route traffic for legitimate purposes.</p>
                
                <h2>How it works</h2>
                <p>When you consent to IPLoop:</p>
                <ul>
                    <li>Your device may route encrypted internet traffic for other users</li>
                    <li>All traffic is encrypted and secure</li>
                    <li>No personal data is accessed or stored</li>
                    <li>Usage is limited to prevent battery drain or data overuse</li>
                </ul>
                
                <h2>Your Control</h2>
                <div class="highlight">
                    <p><strong>You are always in control:</strong></p>
                    <ul>
                        <li>You can opt-out at any time</li>
                        <li>You can set data and battery limits</li>
                        <li>You can restrict to WiFi only</li>
                        <li>No usage without your explicit permission</li>
                    </ul>
                </div>
                
                <h2>Privacy Protection</h2>
                <ul>
                    <li>End-to-end encryption for all traffic</li>
                    <li>No logging of personal information</li>
                    <li>No access to your apps or personal data</li>
                    <li>Compliance with GDPR and privacy regulations</li>
                </ul>
                
                <h2>Security</h2>
                <p>IPLoop actively prevents:</p>
                <ul>
                    <li>Malicious traffic routing</li>
                    <li>Illegal content distribution</li>
                    <li>Network abuse or attacks</li>
                    <li>Privacy violations</li>
                </ul>
                
                <h2>Contact</h2>
                <p>For questions about IPLoop, contact us at privacy@iploop.com</p>
            </body>
            </html>
        """.trimIndent()
    }
}
