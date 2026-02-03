#!/bin/bash
# Build IPLoop Node APK

set -e

echo "🔨 Building IPLoop Node APK..."

# Check if gradle wrapper exists
if [ ! -f "./gradlew" ]; then
    echo "📥 Downloading Gradle wrapper..."
    gradle wrapper --gradle-version 8.2
fi

# Make gradlew executable
chmod +x ./gradlew

# Clean and build
echo "🧹 Cleaning previous builds..."
./gradlew clean

echo "📦 Building release APK..."
./gradlew assembleRelease

# Check if build succeeded
APK_PATH="app/build/outputs/apk/release/app-release-unsigned.apk"
if [ -f "$APK_PATH" ]; then
    echo "✅ APK built successfully!"
    echo "📍 Location: $APK_PATH"
    
    # Show APK info
    ls -lh "$APK_PATH"
else
    echo "❌ Build failed - APK not found"
    exit 1
fi

echo ""
echo "📝 Next steps:"
echo "1. Sign the APK with your keystore"
echo "2. zipalign the APK"
echo "3. Upload to Google Play or distribute directly"
