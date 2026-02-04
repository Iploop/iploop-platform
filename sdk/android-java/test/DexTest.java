import android.content.Context;
import dalvik.system.DexClassLoader;
import java.io.File;
import java.lang.reflect.Method;

public class DexTest {
    public static void main(String[] args) {
        try {
            System.out.println("=== IPLoop SDK Dynamic Loading Test ===\n");
            
            String dexPath = "/sdcard/Download/iploop-sdk-1.0.6-pure.dex";
            String optimizedDir = "/data/local/tmp/dex-test";
            
            // Create optimized dir
            new File(optimizedDir).mkdirs();
            
            System.out.println("[1] Loading DEX: " + dexPath);
            DexClassLoader loader = new DexClassLoader(
                dexPath,
                optimizedDir,
                null,
                DexTest.class.getClassLoader()
            );
            
            // Load SDK class
            System.out.println("[2] Loading IPLoopSDK class...");
            Class<?> sdkClass = loader.loadClass("com.iploop.sdk.IPLoopSDK");
            System.out.println("    ✓ Class loaded: " + sdkClass.getName());
            
            // List all methods
            System.out.println("\n[3] Available methods:");
            Method[] methods = sdkClass.getDeclaredMethods();
            for (Method m : methods) {
                if (java.lang.reflect.Modifier.isPublic(m.getModifiers())) {
                    System.out.println("    - " + m.getName() + "()");
                }
            }
            
            // Test getVersion
            System.out.println("\n[4] Testing getVersion()...");
            Method getVersion = sdkClass.getMethod("getVersion");
            String version = (String) getVersion.invoke(null);
            System.out.println("    ✓ Version: " + version);
            
            // Test getStatus
            System.out.println("\n[5] Testing getStatus()...");
            Method getStatus = sdkClass.getMethod("getStatus");
            Object status = getStatus.invoke(null);
            System.out.println("    ✓ Status: " + status);
            
            // Test isRunning
            System.out.println("\n[6] Testing isRunning()...");
            Method isRunning = sdkClass.getMethod("isRunning");
            Boolean running = (Boolean) isRunning.invoke(null);
            System.out.println("    ✓ Running: " + running);
            
            // Test setConsentGiven
            System.out.println("\n[7] Testing setConsentGiven(true)...");
            Method setConsent = sdkClass.getMethod("setConsentGiven", boolean.class);
            setConsent.invoke(null, true);
            System.out.println("    ✓ Consent set");
            
            // Note: init() and start() need Context, can't test without app
            System.out.println("\n[8] init() and start() require Context - skip in CLI test");
            
            System.out.println("\n=== All tests passed! ===");
            
        } catch (Exception e) {
            System.err.println("ERROR: " + e.getMessage());
            e.printStackTrace();
        }
    }
}
