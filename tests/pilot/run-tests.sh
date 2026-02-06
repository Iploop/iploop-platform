#!/bin/bash
#
# IPLoop Pilot Test Suite Runner
# Comprehensive proxy testing for production readiness
#

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Default config - override with env vars
export PROXY_HOST="${PROXY_HOST:-localhost}"
export PROXY_PORT="${PROXY_PORT:-8080}"
export PROXY_USER="${PROXY_USER:-test_customer}"
export PROXY_PASS="${PROXY_PASS:-test_api_key}"

echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║              IPLoop Pilot Test Suite                             ║"
echo "║                Heavy Duty Testing                                ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo ""
echo "Proxy: $PROXY_HOST:$PROXY_PORT"
echo "Auth: $PROXY_USER:****"
echo ""

usage() {
  echo "Usage: $0 <command>"
  echo ""
  echo "=== QUICK TESTS ==="
  echo "  smoke         Quick stress test (30s, 5 concurrent)"
  echo "  quick         smoke + leak + protocol"
  echo ""
  echo "=== STRESS TESTS ==="
  echo "  stress        Full ramp (5 min, 100 concurrent)"
  echo "  sustained     10 min at 50 concurrent"
  echo "  burst         Spike to 200 concurrent"
  echo "  endurance     1 hour at 25 concurrent"
  echo ""
  echo "=== FEATURE TESTS ==="
  echo "  geo           Geographic targeting (8 countries)"
  echo "  sticky        Sticky session stress (50 sessions)"
  echo "  rotation      IP rotation behavior"
  echo "  peer          Peer load balancing & behavior"
  echo ""
  echo "=== SECURITY TESTS ==="
  echo "  leak          IP/header leak detection"
  echo "  security      Full security audit"
  echo "  auth          Authentication edge cases"
  echo ""
  echo "=== PROTOCOL & RELIABILITY ==="
  echo "  protocol      HTTP methods, headers, encoding"
  echo "  bandwidth     Download speed tests"
  echo "  failure       Error handling & recovery"
  echo "  latency       Latency deep analysis"
  echo "  connection    Connection limits & pooling"
  echo "  concurrency   Concurrency edge cases"
  echo ""
  echo "=== LONG RUNNING ==="
  echo "  stability     Long-running stability (10 min)"
  echo "  scenario      Real-world scraper simulation"
  echo ""
  echo "=== SUITES ==="
  echo "  all           smoke + geo + leak + protocol"
  echo "  full          ALL tests (comprehensive)"
  echo "  production    Pre-production checklist"
  echo ""
  echo "Environment variables:"
  echo "  PROXY_HOST    Proxy hostname (default: localhost)"
  echo "  PROXY_PORT    Proxy port (default: 8080)"
  echo "  PROXY_USER    Proxy username"
  echo "  PROXY_PASS    Proxy password"
  echo "  DURATION      Stability test duration in minutes"
  echo "  RPS           Stability test requests per second"
  echo "  SESSIONS      Concurrent sessions for sticky test"
  exit 1
}

check_proxy() {
  echo "Checking proxy availability..."
  if ! nc -z -w5 "$PROXY_HOST" "$PROXY_PORT" 2>/dev/null; then
    echo "❌ Cannot connect to $PROXY_HOST:$PROXY_PORT"
    echo "   Make sure the IPLoop gateway is running."
    exit 1
  fi
  echo "✅ Proxy is reachable"
  echo ""
}

divider() {
  echo ""
  echo "════════════════════════════════════════════════════════════════════"
  echo "  $1"
  echo "════════════════════════════════════════════════════════════════════"
}

run_with_report() {
  node lib/test-wrapper.js "$@"
}

case "${1:-}" in
  # Quick tests
  smoke)
    check_proxy
    run_with_report stress-test.js --profile=smoke
    ;;
  quick)
    check_proxy
    divider "SMOKE TEST"
    run_with_report stress-test.js --profile=smoke
    divider "LEAK TEST"
    run_with_report leak-test.js
    divider "PROTOCOL TEST"
    run_with_report protocol-test.js
    ;;
    
  # Stress tests
  stress|ramp)
    check_proxy
    run_with_report stress-test.js --profile=ramp
    ;;
  sustained)
    check_proxy
    run_with_report stress-test.js --profile=sustained
    ;;
  burst)
    check_proxy
    run_with_report stress-test.js --profile=burst
    ;;
  endurance)
    check_proxy
    run_with_report stress-test.js --profile=endurance
    ;;
    
  # Feature tests
  geo)
    check_proxy
    run_with_report geo-test.js
    ;;
  sticky)
    check_proxy
    run_with_report sticky-stress.js
    ;;
  rotation)
    check_proxy
    run_with_report rotation-test.js
    ;;
  peer)
    check_proxy
    run_with_report peer-behavior-test.js
    ;;
    
  # Security tests
  leak)
    check_proxy
    run_with_report leak-test.js
    ;;
  security)
    check_proxy
    run_with_report security-test.js
    ;;
  auth)
    check_proxy
    run_with_report auth-test.js
    ;;
    
  # Protocol & reliability
  protocol)
    check_proxy
    run_with_report protocol-test.js
    ;;
  bandwidth)
    check_proxy
    run_with_report bandwidth-test.js
    ;;
  failure)
    check_proxy
    run_with_report failure-test.js
    ;;
  latency)
    check_proxy
    run_with_report latency-test.js
    ;;
  connection)
    check_proxy
    run_with_report connection-limits.js
    ;;
  concurrency)
    check_proxy
    run_with_report concurrency-edge.js
    ;;
    
  # Long running
  stability)
    check_proxy
    run_with_report stability-test.js
    ;;
  scenario)
    check_proxy
    run_with_report scenario-price-scrape.js
    ;;
    
  # Suites
  all)
    check_proxy
    divider "SMOKE TEST"
    run_with_report stress-test.js --profile=smoke
    divider "GEO TEST"
    run_with_report geo-test.js
    divider "LEAK TEST"
    run_with_report leak-test.js
    divider "PROTOCOL TEST"
    run_with_report protocol-test.js
    ;;
    
  full)
    check_proxy
    divider "1/15 - SMOKE TEST"
    run_with_report stress-test.js --profile=smoke
    divider "2/15 - GEO TEST"
    run_with_report geo-test.js
    divider "3/15 - LEAK TEST"
    run_with_report leak-test.js
    divider "4/15 - SECURITY TEST"
    run_with_report security-test.js
    divider "5/15 - AUTH TEST"
    run_with_report auth-test.js
    divider "6/15 - PROTOCOL TEST"
    run_with_report protocol-test.js
    divider "7/15 - BANDWIDTH TEST"
    run_with_report bandwidth-test.js
    divider "8/15 - FAILURE TEST"
    run_with_report failure-test.js
    divider "9/15 - ROTATION TEST"
    run_with_report rotation-test.js
    divider "10/15 - STICKY TEST"
    run_with_report sticky-stress.js
    divider "11/15 - LATENCY TEST"
    run_with_report latency-test.js
    divider "12/15 - CONNECTION TEST"
    run_with_report connection-limits.js
    divider "13/15 - CONCURRENCY TEST"
    run_with_report concurrency-edge.js
    divider "14/15 - PEER TEST"
    run_with_report peer-behavior-test.js
    divider "15/15 - SCENARIO TEST"
    run_with_report scenario-price-scrape.js
    divider "FULL TEST SUITE COMPLETE"
    ;;
    
  production)
    check_proxy
    echo "🚀 Production Readiness Checklist"
    echo ""
    divider "SECURITY AUDIT"
    run_with_report security-test.js
    divider "AUTH VERIFICATION"
    run_with_report auth-test.js
    divider "LEAK DETECTION"
    run_with_report leak-test.js
    divider "PROTOCOL COMPLIANCE"
    run_with_report protocol-test.js
    divider "FAILURE HANDLING"
    run_with_report failure-test.js
    divider "SUSTAINED LOAD"
    run_with_report stress-test.js --profile=sustained
    divider "STABILITY CHECK (5 min)"
    DURATION=5 run_with_report stability-test.js
    divider "PRODUCTION CHECKLIST COMPLETE"
    echo ""
    echo "Review results above before deploying to production."
    ;;
    
  *)
    usage
    ;;
esac
