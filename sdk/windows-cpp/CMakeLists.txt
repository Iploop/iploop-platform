cmake_minimum_required(VERSION 3.16)
project(IPLoopSDK VERSION 2.0.0 LANGUAGES CXX)

# Set C++17 standard
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Platform check
if(NOT WIN32)
    message(FATAL_ERROR "This SDK is designed for Windows only")
endif()

# Architecture check - force 64-bit
if(NOT CMAKE_SIZEOF_VOID_P EQUAL 8)
    message(FATAL_ERROR "This SDK requires 64-bit architecture")
endif()

# Compiler-specific settings
if(MSVC)
    # MSVC settings
    add_compile_options(/W4 /WX- /EHsc)
    add_definitions(-D_WIN32_WINNT=0x0601)  # Windows 7+
    add_definitions(-DWIN32_LEAN_AND_MEAN -DNOMINMAX)
    add_definitions(-D_CRT_SECURE_NO_WARNINGS)
    add_definitions(-DSECURITY_WIN32)
    add_definitions(-D_WINSOCK_DEPRECATED_NO_WARNINGS)
    
    # Release optimizations
    if(CMAKE_BUILD_TYPE STREQUAL "Release")
        add_compile_options(/O2 /GL)
        set(CMAKE_EXE_LINKER_FLAGS_RELEASE "${CMAKE_EXE_LINKER_FLAGS_RELEASE} /LTCG")
        set(CMAKE_STATIC_LINKER_FLAGS_RELEASE "${CMAKE_STATIC_LINKER_FLAGS_RELEASE} /LTCG")
    endif()
elseif(MINGW)
    # MinGW-w64 settings
    add_compile_options(-Wall -Wextra -Wpedantic -Werror)
    add_definitions(-D_WIN32_WINNT=0x0601)  # Windows 7+
    add_definitions(-DWIN32_LEAN_AND_MEAN -DNOMINMAX)
    
    # Release optimizations
    if(CMAKE_BUILD_TYPE STREQUAL "Release")
        add_compile_options(-O3 -flto -march=x86-64 -mtune=generic)
        set(CMAKE_EXE_LINKER_FLAGS_RELEASE "${CMAKE_EXE_LINKER_FLAGS_RELEASE} -flto -s")
    endif()
    
    # Static linking for MinGW
    set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} -static-libgcc -static-libstdc++")
else()
    message(WARNING "Unsupported compiler. This SDK is tested with MSVC and MinGW-w64 only.")
endif()

# Set default build type
if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release CACHE STRING "Choose the type of build" FORCE)
    set_property(CACHE CMAKE_BUILD_TYPE PROPERTY STRINGS "Debug" "Release" "MinSizeRel" "RelWithDebInfo")
endif()

# Include directories
include_directories(include)
include_directories(src)

# Source files
set(SDK_SOURCES
    src/iploop_sdk.cpp
    src/websocket.cpp
    src/tunnel.cpp
    src/utils.cpp
)

# Header files
set(SDK_HEADERS
    include/iploop_sdk.h
    src/websocket.h
    src/tunnel.h
    src/utils.h
)

# Static library
add_library(iploop_sdk_static STATIC ${SDK_SOURCES} ${SDK_HEADERS})
target_compile_definitions(iploop_sdk_static PRIVATE IPLOOP_SDK_STATIC_DEFINE)

# Shared library (DLL)
add_library(iploop_sdk_shared SHARED ${SDK_SOURCES} ${SDK_HEADERS})
target_compile_definitions(iploop_sdk_shared PRIVATE IPLOOP_SDK_EXPORTS)
target_compile_definitions(iploop_sdk_shared INTERFACE IPLOOP_SDK_IMPORTS)

# Set library output names
set_target_properties(iploop_sdk_static PROPERTIES OUTPUT_NAME "iploop_sdk")
set_target_properties(iploop_sdk_shared PROPERTIES OUTPUT_NAME "iploop_sdk")

# Windows libraries
set(WINDOWS_LIBS
    ws2_32      # WinSock2
    wininet     # WinINet (HTTP client)
    secur32     # SChannel (SSL/TLS)
    advapi32    # Registry access
    kernel32    # System functions
    user32      # User API
)

# Link libraries
target_link_libraries(iploop_sdk_static ${WINDOWS_LIBS})
target_link_libraries(iploop_sdk_shared ${WINDOWS_LIBS})

# Example executable
add_executable(iploop_example examples/main.cpp)
target_link_libraries(iploop_example iploop_sdk_static ${WINDOWS_LIBS})

# Installation
install(TARGETS iploop_sdk_static iploop_sdk_shared iploop_example
    RUNTIME DESTINATION bin
    LIBRARY DESTINATION lib
    ARCHIVE DESTINATION lib
)

install(FILES include/iploop_sdk.h DESTINATION include)

# Package configuration
set(CPACK_PACKAGE_NAME "IPLoopSDK")
set(CPACK_PACKAGE_VERSION "${PROJECT_VERSION}")
set(CPACK_PACKAGE_DESCRIPTION_SUMMARY "IPLoop SDK for Windows")
set(CPACK_PACKAGE_VENDOR "IPLoop")
set(CPACK_RESOURCE_FILE_LICENSE "${CMAKE_CURRENT_SOURCE_DIR}/LICENSE")
set(CPACK_RESOURCE_FILE_README "${CMAKE_CURRENT_SOURCE_DIR}/README.md")

if(MSVC)
    set(CPACK_GENERATOR "ZIP;NSIS")
else()
    set(CPACK_GENERATOR "ZIP;TXZ")
endif()

include(CPack)

# Build information
message(STATUS "IPLoop SDK Configuration:")
message(STATUS "  Version: ${PROJECT_VERSION}")
message(STATUS "  Build Type: ${CMAKE_BUILD_TYPE}")
message(STATUS "  Compiler: ${CMAKE_CXX_COMPILER_ID} ${CMAKE_CXX_COMPILER_VERSION}")
message(STATUS "  Architecture: ${CMAKE_SYSTEM_PROCESSOR}")
message(STATUS "  C++ Standard: C++${CMAKE_CXX_STANDARD}")

if(MSVC)
    message(STATUS "  MSVC Runtime: ${CMAKE_MSVC_RUNTIME_LIBRARY}")
endif()

# Build targets summary
message(STATUS "")
message(STATUS "Available targets:")
message(STATUS "  iploop_sdk_static   - Static library (.lib)")
message(STATUS "  iploop_sdk_shared   - Shared library (.dll)")
message(STATUS "  iploop_example      - Example application")
message(STATUS "  install             - Install SDK")
message(STATUS "  package             - Create distribution package")
message(STATUS "")
message(STATUS "Example build commands:")
if(MSVC)
    message(STATUS "  cmake -B build -A x64")
    message(STATUS "  cmake --build build --config Release")
else()
    message(STATUS "  cmake -B build -G \"MinGW Makefiles\" -DCMAKE_BUILD_TYPE=Release")
    message(STATUS "  cmake --build build")
endif()