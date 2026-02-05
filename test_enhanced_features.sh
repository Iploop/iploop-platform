#!/bin/bash

echo "🚀 Testing IPLoop Enhanced Proxy Features v1.0.20"
echo "================================================="

# Test HTTP proxy with basic parameters
echo "1. Testing basic HTTP proxy..."
curl -v --connect-timeout 10 -x "test_user:test_pass@localhost:7777" \
    https://httpbin.org/ip 2>&1 | head -20

echo -e "\n2. Testing geographic targeting..."
curl -v --connect-timeout 10 -x "test_user:test_pass-country-US@localhost:7777" \
    https://httpbin.org/ip 2>&1 | head -10

echo -e "\n3. Testing city targeting..."
curl -v --connect-timeout 10 -x "test_user:test_pass-country-US-city-miami@localhost:7777" \
    https://httpbin.org/ip 2>&1 | head -10

echo -e "\n4. Testing session management..."
curl -v --connect-timeout 10 -x "test_user:test_pass-sesstype-sticky-lifetime-30m@localhost:7777" \
    https://httpbin.org/ip 2>&1 | head -10

echo -e "\n5. Testing browser profiles..."
curl -v --connect-timeout 10 -x "test_user:test_pass-profile-chrome-win-country-US@localhost:7777" \
    https://httpbin.org/headers 2>&1 | head -10

echo -e "\n6. Testing performance settings..."
curl -v --connect-timeout 10 -x "test_user:test_pass-speed-50-latency-200-country-US@localhost:7777" \
    https://httpbin.org/ip 2>&1 | head -10

echo -e "\n7. Testing complex configuration..."
curl -v --connect-timeout 10 -x "test_user:test_pass-country-DE-city-berlin-sesstype-sticky-lifetime-45m-profile-firefox-mac-speed-75-debug-1@localhost:7777" \
    https://httpbin.org/ip 2>&1 | head -10

echo -e "\n8. Testing SOCKS5 proxy..."
curl --socks5 "test_user:test_pass-country-GB@localhost:1080" \
    --connect-timeout 10 https://httpbin.org/ip 2>&1 | head -10 || echo "SOCKS5 test completed"

echo -e "\n✅ Enhanced Features Integration Test Complete!"
echo "All tests executed - check proxy gateway logs for processing details"