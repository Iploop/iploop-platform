#!/bin/bash
set -e

SDK_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$SDK_DIR/build"
FATJAR_DIR="$BUILD_DIR/fatjar"
VERSION="1.0.5"
OUTPUT="$BUILD_DIR/libs/iploop-sdk-${VERSION}-fat.jar"

echo "Building fat JAR v${VERSION}..."

# Clean and create work dir
rm -rf "$FATJAR_DIR"
mkdir -p "$FATJAR_DIR/classes"
mkdir -p "$BUILD_DIR/libs"

# Extract SDK classes
echo "Extracting SDK classes..."
unzip -q -o "$BUILD_DIR/intermediates/aar_main_jar/release/classes.jar" -d "$FATJAR_DIR/classes"

# Find and extract all dependency JARs
echo "Extracting dependencies..."
GRADLE_CACHE="$HOME/.gradle/caches/modules-2/files-2.1"

# Key dependencies to include
DEPS=(
    "org.jetbrains.kotlin/kotlin-stdlib"
    "org.jetbrains.kotlinx/kotlinx-coroutines-core"
    "org.jetbrains.kotlinx/kotlinx-coroutines-android"
    "com.squareup.okhttp3/okhttp"
    "com.squareup.okio/okio"
    "com.squareup.okio/okio-jvm"
)

for dep in "${DEPS[@]}"; do
    echo "  - $dep"
    find "$GRADLE_CACHE/$dep" -name "*.jar" 2>/dev/null | while read jar; do
        unzip -q -o "$jar" -d "$FATJAR_DIR/classes" -x 'META-INF/*.SF' 'META-INF/*.DSA' 'META-INF/*.RSA' 'META-INF/MANIFEST.MF' 2>/dev/null || true
    done
done

# Also include atomicfu
find "$GRADLE_CACHE/org.jetbrains.kotlinx/atomicfu" -name "*.jar" 2>/dev/null | while read jar; do
    echo "  - atomicfu: $jar"
    unzip -q -o "$jar" -d "$FATJAR_DIR/classes" -x 'META-INF/*.SF' 'META-INF/*.DSA' 'META-INF/*.RSA' 'META-INF/MANIFEST.MF' 2>/dev/null || true
done

# Create manifest
echo "Creating manifest..."
mkdir -p "$FATJAR_DIR/classes/META-INF"
cat > "$FATJAR_DIR/classes/META-INF/MANIFEST.MF" << EOF
Manifest-Version: 1.0
Implementation-Title: IPLoop SDK
Implementation-Version: ${VERSION}
Created-By: IPLoop Build Script
EOF

# Build the JAR
echo "Creating fat JAR..."
cd "$FATJAR_DIR/classes"
jar cf "$OUTPUT" .

# Report
SIZE=$(du -h "$OUTPUT" | cut -f1)
echo ""
echo "✅ Fat JAR created: $OUTPUT"
echo "   Size: $SIZE"
ls -la "$OUTPUT"
