/*
 * IPLoop SDK C API Example
 * Demonstrates how to use the SDK from C code or other languages
 */

#include <stdio.h>
#include <stdlib.h>

#ifdef _WIN32
    #include <windows.h>
    #define SLEEP_MS(ms) Sleep(ms)
#else
    #include <unistd.h>
    #define SLEEP_MS(ms) usleep((ms) * 1000)
#endif

// Function declarations (normally these would be in a header)
extern int IPLoop_Initialize(const char* apiKey);
extern int IPLoop_Start(void);
extern int IPLoop_Stop(void);
extern int IPLoop_IsActive(void);
extern void IPLoop_SetConsent(int consent);
extern int IPLoop_GetTotalRequests(void);
extern double IPLoop_GetTotalMB(void);
extern const char* IPLoop_GetProxyURL(void);
extern void IPLoop_SetCountry(const char* country);
extern void IPLoop_SetCity(const char* city);
extern const char* IPLoop_GetVersion(void);

int main() {
    printf("IPLoop SDK C API Example\n");
    printf("Version: %s\n\n", IPLoop_GetVersion());
    
    // Initialize SDK
    printf("Initializing SDK...\n");
    if (IPLoop_Initialize("your_api_key_here") != 0) {
        fprintf(stderr, "Failed to initialize SDK\n");
        return 1;
    }
    printf("SDK initialized successfully\n");
    
    // Set user consent
    printf("Setting user consent...\n");
    IPLoop_SetConsent(1);  // 1 = true, 0 = false
    
    // Configure proxy settings
    printf("Configuring proxy settings...\n");
    IPLoop_SetCountry("US");
    IPLoop_SetCity("miami");
    
    // Start SDK
    printf("Starting SDK...\n");
    if (IPLoop_Start() != 0) {
        fprintf(stderr, "Failed to start SDK\n");
        return 1;
    }
    printf("SDK started successfully\n");
    
    // Wait for connection
    SLEEP_MS(3000);
    
    // Check if active
    if (IPLoop_IsActive()) {
        printf("SDK is active and ready\n");
        printf("Proxy URL: %s\n", IPLoop_GetProxyURL());
    } else {
        printf("SDK is not active yet\n");
    }
    
    // Monitor for 10 seconds
    printf("\nMonitoring for 10 seconds...\n");
    for (int i = 0; i < 10; i++) {
        printf("Time: %ds, Requests: %d, Bandwidth: %.2f MB\n", 
               i + 1, IPLoop_GetTotalRequests(), IPLoop_GetTotalMB());
        SLEEP_MS(1000);
    }
    
    // Test different configurations
    printf("\nTesting configuration changes...\n");
    
    // Switch to UK
    printf("Switching to UK...\n");
    IPLoop_SetCountry("GB");
    IPLoop_SetCity("london");
    printf("New Proxy URL: %s\n", IPLoop_GetProxyURL());
    
    SLEEP_MS(2000);
    
    // Switch to Germany
    printf("Switching to Germany...\n");
    IPLoop_SetCountry("DE");
    IPLoop_SetCity("berlin");
    printf("New Proxy URL: %s\n", IPLoop_GetProxyURL());
    
    SLEEP_MS(2000);
    
    // Final stats
    printf("\n=== Final Statistics ===\n");
    printf("Total requests: %d\n", IPLoop_GetTotalRequests());
    printf("Total bandwidth: %.2f MB\n", IPLoop_GetTotalMB());
    printf("Is active: %s\n", IPLoop_IsActive() ? "Yes" : "No");
    
    // Stop SDK
    printf("\nStopping SDK...\n");
    if (IPLoop_Stop() != 0) {
        fprintf(stderr, "Failed to stop SDK\n");
        return 1;
    }
    printf("SDK stopped successfully\n");
    
    // Verify it's stopped
    SLEEP_MS(1000);
    printf("Is active after stop: %s\n", IPLoop_IsActive() ? "Yes" : "No");
    
    printf("\nC API example completed successfully\n");
    return 0;
}