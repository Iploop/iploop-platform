#!/bin/bash
set -e

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$APP_DIR/build"
ANDROID_JAR="/opt/android-sdk/platforms/android-34/android.jar"
AAPT="/opt/android-sdk/build-tools/35.0.0/aapt"
D8="/opt/android-sdk/build-tools/35.0.0/d8"
APKSIGNER="/opt/android-sdk/build-tools/35.0.0/apksigner"

echo "Building IPLoop Test APK..."

# Clean
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/classes"
mkdir -p "$BUILD_DIR/gen"

# Generate R.java from resources (if res/ exists)
if [ -d "$APP_DIR/res" ]; then
    echo "Generating R.java..."
    $AAPT package -f -m -J "$BUILD_DIR/gen" -M "$APP_DIR/AndroidManifest.xml" -S "$APP_DIR/res" -I "$ANDROID_JAR"
fi

# Extract SDK v1.0.20 classes
SDK_JAR="/root/clawd-secure/iploop-platform/sdk/android-java/build/iploop-sdk-1.0.20-pure.jar"
echo "Extracting SDK classes..."
cd "$BUILD_DIR/classes"
jar xf "$SDK_JAR"
cd "$APP_DIR"

# Compile Java
echo "Compiling Java with SDK v1.0.20..."
JAVA_FILES="$APP_DIR/src/com/iploop/test/"*.java
if [ -f "$BUILD_DIR/gen/com/iploop/test/R.java" ]; then
    JAVA_FILES="$JAVA_FILES $BUILD_DIR/gen/com/iploop/test/R.java"
fi
javac -source 1.8 -target 1.8 \
    -bootclasspath "$ANDROID_JAR" \
    -cp "$ANDROID_JAR:$SDK_JAR" \
    -d "$BUILD_DIR/classes" \
    $JAVA_FILES

# Create DEX
echo "Creating DEX..."
$D8 --lib "$ANDROID_JAR" --min-api 21 --output "$BUILD_DIR" $(find "$BUILD_DIR/classes" -name "*.class")

# Create base APK with manifest and resources
echo "Creating APK..."
if [ -d "$APP_DIR/res" ]; then
    $AAPT package -f -M "$APP_DIR/AndroidManifest.xml" -S "$APP_DIR/res" -I "$ANDROID_JAR" -F "$BUILD_DIR/test-unsigned.apk"
else
    $AAPT package -f -M "$APP_DIR/AndroidManifest.xml" -I "$ANDROID_JAR" -F "$BUILD_DIR/test-unsigned.apk"
fi

# Add DEX to APK
cd "$BUILD_DIR"
zip -j test-unsigned.apk classes.dex

# Create debug keystore if needed
KEYSTORE="$APP_DIR/debug.keystore"
if [ ! -f "$KEYSTORE" ]; then
    echo "Creating debug keystore..."
    keytool -genkey -v -keystore "$KEYSTORE" -storepass android -alias androiddebugkey -keypass android -keyalg RSA -keysize 2048 -validity 10000 -dname "CN=Debug, O=Android, C=US" 2>/dev/null
fi

# Sign APK
echo "Signing APK..."
$APKSIGNER sign --ks "$KEYSTORE" --ks-pass pass:android --key-pass pass:android --out "$BUILD_DIR/iploop-test.apk" "$BUILD_DIR/test-unsigned.apk"

echo ""
echo "✅ APK built: $BUILD_DIR/iploop-test.apk"
ls -la "$BUILD_DIR/iploop-test.apk"
