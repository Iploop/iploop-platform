#!/bin/bash
set -e

echo "IPLoop SDK Cross-Compilation Test (Linux -> Windows)"
echo "===================================================="

# Check if CMake is available
if ! command -v cmake >/dev/null 2>&1; then
    echo "ERROR: CMake not found. Please install cmake"
    exit 1
fi

# Check for MinGW-w64 cross-compiler
if ! command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
    echo "ERROR: MinGW-w64 cross-compiler not found."
    echo "Please install: sudo apt-get install mingw-w64"
    exit 1
fi

echo "Found: MinGW-w64 cross-compiler"
x86_64-w64-mingw32-gcc --version | head -1

# Clean previous build
rm -rf build

# Configure for cross-compilation
echo "Configuring for Windows x64 cross-compilation..."
cmake -B build \
    -DCMAKE_SYSTEM_NAME=Windows \
    -DCMAKE_C_COMPILER=x86_64-w64-mingw32-gcc \
    -DCMAKE_CXX_COMPILER=x86_64-w64-mingw32-g++ \
    -DCMAKE_RC_COMPILER=x86_64-w64-mingw32-windres \
    -DCMAKE_BUILD_TYPE=Release

# Build
echo "Building SDK..."
cmake --build build

echo
echo "========================================"
echo "CROSS-COMPILATION SUCCESSFUL!"
echo "========================================"
echo
echo "Generated files:"
ls -la build/*.exe build/*.a build/*.dll 2>/dev/null || true
echo
echo "Windows executables created (run on Windows):"
echo "  build/iploop_example.exe"
echo

# Create simple deployment package
echo "Creating deployment package..."
mkdir -p deploy/windows-x64
cp build/libiploop_sdk.a deploy/windows-x64/ 2>/dev/null || true
cp build/iploop_example.exe deploy/windows-x64/
cp include/iploop_sdk.h deploy/windows-x64/
cp README.md deploy/windows-x64/

echo "Deployment package ready: deploy/windows-x64/"