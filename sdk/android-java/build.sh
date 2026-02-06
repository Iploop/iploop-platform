#!/bin/bash
set -e

SDK_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC_DIR="$SDK_DIR/src/main/java"
BUILD_DIR="$SDK_DIR/build"
VERSION="1.0.55"
OUTPUT="$BUILD_DIR/iploop-sdk-${VERSION}.jar"
DEX_OUTPUT="$BUILD_DIR/iploop-sdk-${VERSION}-bundle.jar"

ANDROID_JAR="/opt/android-sdk/platforms/android-34/android.jar"
WEBSOCKET_JAR="$SDK_DIR/libs/Java-WebSocket-1.5.6.jar"
SLF4J_JAR="$SDK_DIR/libs/slf4j-api-2.0.9.jar"

echo "Building IPLoop SDK v${VERSION}..."

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/classes"

echo "Compiling..."
javac -source 1.8 -target 1.8 \
    -bootclasspath "$ANDROID_JAR" \
    -cp "$ANDROID_JAR:$WEBSOCKET_JAR:$SLF4J_JAR" \
    -d "$BUILD_DIR/classes" \
    "$SRC_DIR/com/iploop/sdk/"*.java

echo "Packaging..."
cd "$BUILD_DIR/classes"
jar cf "$OUTPUT" .

echo "DEXing..."
/opt/android-sdk/build-tools/35.0.0/d8 --min-api 22 \
    --output "$DEX_OUTPUT" \
    "$OUTPUT" "$WEBSOCKET_JAR" "$SLF4J_JAR" "$SDK_DIR/libs/slf4j-nop-2.0.9.jar" 2>&1 | grep -v "^Warning" || true

echo "✅ Done"
ls -la "$DEX_OUTPUT"
