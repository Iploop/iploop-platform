#!/bin/bash

# IPLoop Enterprise Production App Builder
# Samsung Galaxy A17 Production Deployment

set -e

echo "Building IPLoop Enterprise Production App..."

# Setup paths
BUILD_DIR="build"
ANDROID_JAR="/opt/android-sdk/platforms/android-34/android.jar"
SDK_JAR="/root/clawd-secure/iploop-platform/sdk/android-java/build/iploop-sdk-1.0.20-pure.jar"
AAPT="/opt/android-sdk/build-tools/34.0.0/aapt"
D8="/opt/android-sdk/build-tools/34.0.0/d8"
ZIPALIGN="/opt/android-sdk/build-tools/34.0.0/zipalign"
APKSIGNER="/opt/android-sdk/build-tools/34.0.0/apksigner"

# Clean and create build directory
rm -rf $BUILD_DIR
mkdir -p $BUILD_DIR

# Generate R.java
echo "Generating R.java..."
$AAPT package -f -m \
    -S res \
    -M AndroidManifest.xml \
    -I $ANDROID_JAR \
    -J $BUILD_DIR

# Extract SDK classes
echo "Extracting SDK classes..."
cd $BUILD_DIR
jar -xf $SDK_JAR
cd ..

# Compile Java with SDK v1.0.20
echo "Compiling Java with SDK v1.0.20..."
javac -source 8 -target 8 \
    -cp $ANDROID_JAR:$SDK_JAR \
    -d $BUILD_DIR \
    src/com/iploop/production/*.java \
    $BUILD_DIR/com/iploop/production/R.java

# Create DEX
echo "Creating DEX..."
find $BUILD_DIR -name "*.class" -exec $D8 --output $BUILD_DIR/ --lib $ANDROID_JAR {} +

# Create APK
echo "Creating APK..."
$AAPT package -f \
    -M AndroidManifest.xml \
    -S res \
    -I $ANDROID_JAR \
    -F $BUILD_DIR/iploop-enterprise-unsigned.apk

# Add classes.dex to APK
cd $BUILD_DIR
zip -q iploop-enterprise-unsigned.apk classes.dex
cd ..

# Sign APK with debug key
echo "Signing APK..."
if [ ! -f debug.keystore ]; then
    # Create debug keystore if it doesn't exist
    keytool -genkey -v -keystore debug.keystore \
        -alias androiddebugkey \
        -keyalg RSA -keysize 2048 -validity 10000 \
        -storepass android -keypass android \
        -dname "CN=Android Debug, O=Android, C=US"
fi

$APKSIGNER sign \
    --ks debug.keystore \
    --ks-key-alias androiddebugkey \
    --ks-pass pass:android \
    --key-pass pass:android \
    --out $BUILD_DIR/iploop-enterprise.apk \
    $BUILD_DIR/iploop-enterprise-unsigned.apk

echo ""
echo "✅ Production APK built: $BUILD_DIR/iploop-enterprise.apk"
ls -la $BUILD_DIR/iploop-enterprise.apk
echo ""
echo "🚀 Ready for Samsung Galaxy A17 deployment!"
echo ""

# Verify APK
$AAPT dump badging $BUILD_DIR/iploop-enterprise.apk | head -10