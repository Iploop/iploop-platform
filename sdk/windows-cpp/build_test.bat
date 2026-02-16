@echo off
echo IPLoop SDK Build Test
echo ====================

REM Check if CMake is available
cmake --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: CMake not found. Please install CMake and add to PATH.
    pause
    exit /b 1
)

REM Detect compiler
where cl.exe >nul 2>&1
if not errorlevel 1 (
    echo Found: Visual Studio MSVC compiler
    set USE_MSVC=1
) else (
    where g++.exe >nul 2>&1
    if not errorlevel 1 (
        echo Found: MinGW-w64 GCC compiler
        set USE_MINGW=1
    ) else (
        echo ERROR: No supported compiler found.
        echo Please install Visual Studio 2019+ or MinGW-w64
        pause
        exit /b 1
    )
)

REM Clean previous build
if exist build rmdir /s /q build

REM Configure build
if defined USE_MSVC (
    echo Configuring with Visual Studio...
    cmake -B build -A x64
    if errorlevel 1 goto build_error
) else if defined USE_MINGW (
    echo Configuring with MinGW-w64...
    cmake -B build -G "MinGW Makefiles" -DCMAKE_BUILD_TYPE=Release
    if errorlevel 1 goto build_error
)

REM Build
echo Building SDK...
if defined USE_MSVC (
    cmake --build build --config Release
) else if defined USE_MINGW (
    cmake --build build
)
if errorlevel 1 goto build_error

REM Success
echo.
echo ========================================
echo BUILD SUCCESSFUL!
echo ========================================
echo.
echo Generated files:
dir build\Release\*.exe 2>nul || dir build\*.exe 2>nul
dir build\Release\*.lib 2>nul || dir build\*.a 2>nul
dir build\Release\*.dll 2>nul || dir build\*.dll 2>nul
echo.
echo Test the example:
if defined USE_MSVC (
    echo   build\Release\iploop_example.exe
) else if defined USE_MINGW (
    echo   build\iploop_example.exe
)
echo.
pause
exit /b 0

:build_error
echo.
echo ========================================
echo BUILD FAILED!
echo ========================================
echo Please check the error messages above.
pause
exit /b 1