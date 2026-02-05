#!/bin/bash
set -e

SDK_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC_DIR="$SDK_DIR/src/main/java"
BUILD_DIR="$SDK_DIR/build"
VERSION="1.0.20"
OUTPUT="$BUILD_DIR/iploop-sdk-${VERSION}-pure.jar"
DEX_OUTPUT="$BUILD_DIR/iploop-sdk-${VERSION}-pure.dex"

ANDROID_JAR="/opt/android-sdk/platforms/android-34/android.jar"

echo "Building IPLoop SDK v${VERSION} with Enhanced Enterprise Features..."

# Clean
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/classes"

# Compile
echo "Compiling..."
javac -source 1.8 -target 1.8 \
    -bootclasspath "$ANDROID_JAR" \
    -cp "$ANDROID_JAR" \
    -d "$BUILD_DIR/classes" \
    "$SRC_DIR/com/iploop/sdk/"*.java

# Create JAR
echo "Packaging JAR..."
cd "$BUILD_DIR/classes"
jar cf "$OUTPUT" .

# Create DEX using d8 from build-tools 35.0.0 (34.0.0 has a bug)
echo "Converting to DEX..."
/opt/android-sdk/build-tools/35.0.0/d8 --min-api 22 --output "$BUILD_DIR" "$OUTPUT" 2>&1 || true
mv "$BUILD_DIR/classes.dex" "$DEX_OUTPUT" 2>/dev/null || echo "Warning: DEX conversion may have failed"

# Report
echo ""
echo "✅ Build complete!"
ls -la "$OUTPUT" "$DEX_OUTPUT" 2>/dev/null || ls -la "$OUTPUT"
echo ""
echo "Files:"
jar tf "$OUTPUT"
