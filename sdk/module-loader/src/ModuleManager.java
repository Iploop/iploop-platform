package io.iploop.modules;

import android.content.Context;
import android.util.Log;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

import dalvik.system.DexClassLoader;

/**
 * ModuleManager — Downloads SDK JARs at runtime, loads them via DexClassLoader.
 *
 * Flow:
 * 1. Fetch modules_config.json from server (every 5 min)
 * 2. For each module: download JAR if not cached
 * 3. Load JAR with DexClassLoader → get Class via reflection
 * 4. Invoke init/start methods based on type (static/instance/builder)
 *
 * JARs must contain classes.dex (use dx/d8 to convert .class → .dex)
 */
public class ModuleManager {

    private static final String TAG = "IPLoop-ModuleManager";
    private static final long DEFAULT_RELOAD_INTERVAL = 300; // 5 min

    private final Context context;
    private final String configUrl;
    private final String baseDownloadUrl;  // e.g. https://cdn.iploop.io/sdks/
    private final String deviceId;

    private final File modulesDir;   // downloaded JARs go here
    private final File dexCacheDir;  // DexClassLoader optimized dex cache

    private final Map<String, LoadedModule> loadedModules = new ConcurrentHashMap<>();
    private final Map<String, DexClassLoader> classLoaders = new ConcurrentHashMap<>();
    private final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor();

    private JSONObject currentConfig;
    private int currentVersion = 0;

    public ModuleManager(Context context, String configUrl, String baseDownloadUrl, String deviceId) {
        this.context = context.getApplicationContext();
        this.configUrl = configUrl;
        this.baseDownloadUrl = baseDownloadUrl;
        this.deviceId = deviceId;

        // Storage dirs
        this.modulesDir = new File(context.getFilesDir(), "iploop_modules");
        this.dexCacheDir = new File(context.getCacheDir(), "iploop_dex");
        modulesDir.mkdirs();
        dexCacheDir.mkdirs();
    }

    // ═══════════════════════════════════════════
    //  LIFECYCLE
    // ═══════════════════════════════════════════

    public void start() {
        Log.i(TAG, "Starting ModuleManager...");
        Log.i(TAG, "Modules dir: " + modulesDir.getAbsolutePath());
        Log.i(TAG, "Dex cache: " + dexCacheDir.getAbsolutePath());

        // Initial load
        fetchAndApplyConfig();

        // Schedule reload
        long interval = DEFAULT_RELOAD_INTERVAL;
        if (currentConfig != null) {
            interval = currentConfig.optLong("reload_interval_seconds", DEFAULT_RELOAD_INTERVAL);
        }
        scheduler.scheduleWithFixedDelay(
            this::fetchAndApplyConfig,
            interval, interval, TimeUnit.SECONDS
        );
        Log.i(TAG, "Reload scheduled every " + interval + "s");
    }

    public void stop() {
        Log.i(TAG, "Stopping ModuleManager...");
        scheduler.shutdown();
        for (Map.Entry<String, LoadedModule> entry : loadedModules.entrySet()) {
            try {
                entry.getValue().stop();
                Log.i(TAG, "Stopped: " + entry.getKey());
            } catch (Exception e) {
                Log.e(TAG, "Error stopping " + entry.getKey(), e);
            }
        }
        loadedModules.clear();
        classLoaders.clear();
    }

    public int getActiveModuleCount() {
        return loadedModules.size();
    }

    // ═══════════════════════════════════════════
    //  CONFIG FETCH & APPLY
    // ═══════════════════════════════════════════

    private void fetchAndApplyConfig() {
        try {
            String json = httpGet(configUrl);
            if (json == null || json.isEmpty()) {
                Log.w(TAG, "Empty config response");
                return;
            }

            JSONObject newConfig = new JSONObject(json);
            int newVersion = newConfig.optInt("version", 0);

            if (newVersion <= currentVersion && currentVersion > 0) {
                Log.d(TAG, "Config unchanged (v" + currentVersion + ")");
                return;
            }

            Log.i(TAG, "New config v" + newVersion + " (was v" + currentVersion + ")");
            currentConfig = newConfig;
            currentVersion = newVersion;
            applyConfig(newConfig);

        } catch (Exception e) {
            Log.e(TAG, "Config fetch failed", e);
        }
    }

    private void applyConfig(JSONObject config) {
        try {
            JSONArray modules = config.getJSONArray("modules");
            Map<String, Boolean> seen = new ConcurrentHashMap<>();

            for (int i = 0; i < modules.length(); i++) {
                JSONObject def = modules.getJSONObject(i);
                String id = def.getString("id");
                boolean enabled = def.optBoolean("enabled", true);
                seen.put(id, true);

                if (!enabled) {
                    stopAndRemove(id);
                    continue;
                }

                if (loadedModules.containsKey(id)) {
                    Log.d(TAG, "Already running: " + id);
                    continue;
                }

                // Load new module in background
                final JSONObject moduleDef = def;
                scheduler.execute(() -> {
                    try {
                        LoadedModule instance = loadModule(moduleDef);
                        if (instance != null) {
                            loadedModules.put(id, instance);
                            Log.i(TAG, "✅ Loaded: " + id);
                        }
                    } catch (Exception e) {
                        Log.e(TAG, "❌ Failed to load: " + id, e);
                    }
                });
            }

            // Stop removed modules
            for (String id : loadedModules.keySet()) {
                if (!seen.containsKey(id)) {
                    stopAndRemove(id);
                }
            }

        } catch (Exception e) {
            Log.e(TAG, "Error applying config", e);
        }
    }

    private void stopAndRemove(String id) {
        LoadedModule m = loadedModules.remove(id);
        if (m != null) {
            try {
                m.stop();
                Log.i(TAG, "Stopped & removed: " + id);
            } catch (Exception e) {
                Log.e(TAG, "Error stopping " + id, e);
            }
        }
        classLoaders.remove(id);
    }

    // ═══════════════════════════════════════════
    //  DYNAMIC JAR DOWNLOAD & LOADING
    // ═══════════════════════════════════════════

    /**
     * Download JAR if not cached, load with DexClassLoader, instantiate module.
     */
    private LoadedModule loadModule(JSONObject def) throws Exception {
        String id = def.getString("id");
        String jarName = def.getString("jar");
        String className = def.getString("class");
        String type = def.optString("type", "static");
        String jarHash = def.optString("hash", "");  // optional integrity check

        Log.i(TAG, "Loading [" + id + "] type=" + type + " jar=" + jarName + " class=" + className);

        // 1. Download JAR if needed
        File jarFile = new File(modulesDir, jarName);
        if (!jarFile.exists() || !jarHash.isEmpty()) {
            String downloadUrl = def.optString("jar_url", baseDownloadUrl + jarName);
            downloadJar(downloadUrl, jarFile);
        }

        if (!jarFile.exists()) {
            throw new Exception("JAR download failed: " + jarName);
        }

        // 2. Load JAR with DexClassLoader
        DexClassLoader loader = new DexClassLoader(
            jarFile.getAbsolutePath(),
            dexCacheDir.getAbsolutePath(),
            null,  // native lib path
            context.getClassLoader()  // parent classloader
        );
        classLoaders.put(id, loader);

        // 3. Load the target class
        Class<?> clazz = loader.loadClass(className);
        Log.i(TAG, "Class loaded: " + className);

        // 4. Instantiate based on type
        switch (type) {
            case "static":
                return loadStaticModule(clazz, def);
            case "instance":
                return loadInstanceModule(clazz, def);
            case "builder":
                return loadBuilderModule(clazz, def);
            default:
                throw new Exception("Unknown type: " + type);
        }
    }

    /**
     * Download a JAR from URL to local file.
     */
    private void downloadJar(String urlStr, File dest) throws Exception {
        Log.i(TAG, "Downloading: " + urlStr + " → " + dest.getName());

        URL url = new URL(urlStr);
        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
        conn.setConnectTimeout(30000);
        conn.setReadTimeout(60000);

        int code = conn.getResponseCode();
        if (code != 200) {
            throw new Exception("Download HTTP " + code + ": " + urlStr);
        }

        // Atomic write — download to temp, then rename
        File temp = new File(dest.getParent(), dest.getName() + ".tmp");
        InputStream in = conn.getInputStream();
        FileOutputStream out = new FileOutputStream(temp);

        byte[] buf = new byte[8192];
        int len;
        long total = 0;
        while ((len = in.read(buf)) != -1) {
            out.write(buf, 0, len);
            total += len;
        }
        out.close();
        in.close();
        conn.disconnect();

        // Rename temp → final
        if (dest.exists()) dest.delete();
        temp.renameTo(dest);

        Log.i(TAG, "Downloaded: " + dest.getName() + " (" + total + " bytes)");
    }

    // ═══════════════════════════════════════════
    //  MODULE TYPE HANDLERS
    // ═══════════════════════════════════════════

    /**
     * Type A — Static: SomeSDK.init(ctx, id, "key", true); SomeSDK.start()
     */
    private LoadedModule loadStaticModule(Class<?> clazz, JSONObject def) throws Exception {
        JSONObject initDef = def.getJSONObject("init");
        invokeMethod(null, clazz, initDef);

        JSONObject startDef = def.getJSONObject("start");
        invokeMethod(null, clazz, startDef);

        return new LoadedModule(null, clazz, def.getJSONObject("stop"));
    }

    /**
     * Type B — Instance: sdk = new SDK(ctx); sdk.setKey("x"); sdk.connect()
     */
    private LoadedModule loadInstanceModule(Class<?> clazz, JSONObject def) throws Exception {
        // Create instance
        JSONObject createDef = def.getJSONObject("create");
        Object[] ctorArgs = resolveParams(createDef.optJSONArray("params"));
        Class<?>[] ctorTypes = resolveParamTypes(createDef.optJSONArray("params"));

        Object instance;
        if (ctorTypes.length == 0) {
            instance = clazz.getConstructor().newInstance();
        } else {
            instance = clazz.getConstructor(ctorTypes).newInstance(ctorArgs);
        }

        // Configure — chain of setter calls
        JSONArray configureCalls = def.optJSONArray("configure");
        if (configureCalls != null) {
            for (int i = 0; i < configureCalls.length(); i++) {
                invokeMethod(instance, clazz, configureCalls.getJSONObject(i));
            }
        }

        // Start
        invokeMethod(instance, clazz, def.getJSONObject("start"));

        return new LoadedModule(instance, clazz, def.getJSONObject("stop"));
    }

    /**
     * Type C — Builder: new Builder(ctx).key("x").build().start()
     */
    private LoadedModule loadBuilderModule(Class<?> clazz, JSONObject def) throws Exception {
        JSONObject buildDef = def.getJSONObject("build");

        // Load builder class (might be inner class)
        String builderClassName = buildDef.getString("builder_class");
        DexClassLoader loader = classLoaders.get(def.getString("id"));
        Class<?> builderClass = (loader != null)
            ? loader.loadClass(builderClassName)
            : Class.forName(builderClassName);

        // Create builder
        Object[] ctorArgs = resolveParams(buildDef.optJSONArray("constructor_params"));
        Class<?>[] ctorTypes = resolveParamTypes(buildDef.optJSONArray("constructor_params"));
        Object builder = builderClass.getConstructor(ctorTypes).newInstance(ctorArgs);

        // Chain methods
        JSONArray chain = buildDef.optJSONArray("chain");
        if (chain != null) {
            for (int i = 0; i < chain.length(); i++) {
                Object result = invokeMethod(builder, builderClass, chain.getJSONObject(i));
                if (result != null && builderClass.isInstance(result)) {
                    builder = result;  // builder returns self
                }
            }
        }

        // Finalize — .build()
        String finalizeMethod = buildDef.optString("finalize", "build");
        Object instance = builderClass.getMethod(finalizeMethod).invoke(builder);

        // Start
        invokeMethod(instance, instance.getClass(), def.getJSONObject("start"));

        return new LoadedModule(instance, instance.getClass(), def.getJSONObject("stop"));
    }

    // ═══════════════════════════════════════════
    //  REFLECTION HELPERS
    // ═══════════════════════════════════════════

    private Object invokeMethod(Object target, Class<?> clazz, JSONObject methodDef) throws Exception {
        String methodName = methodDef.getString("method");
        JSONArray paramsDef = methodDef.optJSONArray("params");

        Object[] args = resolveParams(paramsDef);
        Class<?>[] types = resolveParamTypes(paramsDef);

        // Find method — try exact match first, then search
        java.lang.reflect.Method method = findMethod(clazz, methodName, types);

        if (target == null) {
            return method.invoke(null, args);  // static
        } else {
            return method.invoke(target, args);
        }
    }

    /**
     * Find method by name and param types. Falls back to name-only search
     * (handles cases where param types don't match exactly, e.g. Object vs String).
     */
    private java.lang.reflect.Method findMethod(Class<?> clazz, String name, Class<?>[] types) 
            throws NoSuchMethodException {
        try {
            return clazz.getMethod(name, types);
        } catch (NoSuchMethodException e) {
            // Fallback: find by name and param count
            for (java.lang.reflect.Method m : clazz.getMethods()) {
                if (m.getName().equals(name) && m.getParameterTypes().length == types.length) {
                    return m;
                }
            }
            throw e;
        }
    }

    private Object[] resolveParams(JSONArray paramsDef) {
        if (paramsDef == null || paramsDef.length() == 0) return new Object[0];
        Object[] args = new Object[paramsDef.length()];
        for (int i = 0; i < paramsDef.length(); i++) {
            try {
                args[i] = resolveValue(paramsDef.getJSONObject(i));
            } catch (Exception e) {
                args[i] = null;
            }
        }
        return args;
    }

    private Class<?>[] resolveParamTypes(JSONArray paramsDef) {
        if (paramsDef == null || paramsDef.length() == 0) return new Class[0];
        Class<?>[] types = new Class[paramsDef.length()];
        for (int i = 0; i < paramsDef.length(); i++) {
            try {
                types[i] = resolveType(paramsDef.getJSONObject(i));
            } catch (Exception e) {
                types[i] = Object.class;
            }
        }
        return types;
    }

    private Object resolveValue(JSONObject p) {
        String type = p.optString("type", "string");
        switch (type) {
            case "context":     return context;
            case "device_id":   return deviceId;
            case "string":      return p.optString("value", "");
            case "boolean":     return p.optBoolean("value", false);
            case "int":         return p.optInt("value", 0);
            case "long":        return p.optLong("value", 0L);
            case "float":       return (float) p.optDouble("value", 0.0);
            case "double":      return p.optDouble("value", 0.0);
            default:            return p.optString("value", "");
        }
    }

    private Class<?> resolveType(JSONObject p) {
        String type = p.optString("type", "string");
        switch (type) {
            case "context":     return Context.class;
            case "device_id":
            case "string":      return String.class;
            case "boolean":     return boolean.class;
            case "int":         return int.class;
            case "long":        return long.class;
            case "float":       return float.class;
            case "double":      return double.class;
            default:            return String.class;
        }
    }

    // ═══════════════════════════════════════════
    //  HTTP
    // ═══════════════════════════════════════════

    private String httpGet(String urlStr) {
        try {
            URL url = new URL(urlStr);
            HttpURLConnection conn = (HttpURLConnection) url.openConnection();
            conn.setConnectTimeout(10000);
            conn.setReadTimeout(10000);
            if (conn.getResponseCode() != 200) return null;

            BufferedReader reader = new BufferedReader(new InputStreamReader(conn.getInputStream()));
            StringBuilder sb = new StringBuilder();
            String line;
            while ((line = reader.readLine()) != null) sb.append(line);
            reader.close();
            return sb.toString();
        } catch (Exception e) {
            Log.e(TAG, "HTTP GET failed", e);
            return null;
        }
    }

    // ═══════════════════════════════════════════
    //  LOADED MODULE HANDLE
    // ═══════════════════════════════════════════

    static class LoadedModule {
        final Object instance;
        final Class<?> clazz;
        final JSONObject stopDef;

        LoadedModule(Object instance, Class<?> clazz, JSONObject stopDef) {
            this.instance = instance;
            this.clazz = clazz;
            this.stopDef = stopDef;
        }

        void stop() throws Exception {
            String method = stopDef.getString("method");
            java.lang.reflect.Method m = clazz.getMethod(method);
            m.invoke(instance);  // null instance = static call
        }
    }
}
