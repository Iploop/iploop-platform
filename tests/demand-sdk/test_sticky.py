#!/usr/bin/env python3
"""
IPLoop Python SDK - Sticky Session Tests
Tests session persistence, IP consistency, and session management.

Usage:
    python test_sticky.py [--api-key KEY] [--session-count N] [--requests-per-session N]

Example:
    python test_sticky.py --api-key testkey123 --session-count 5 --requests-per-session 20
"""

import argparse
import json
import time
import statistics
from datetime import datetime
from iploop import IPLoop

def test_session_consistency(api_key, session_id, request_count=20):
    """Test that a session maintains the same IP across multiple requests"""
    print(f"Testing session consistency: {session_id} ({request_count} requests)")
    
    base_client = IPLoop(api_key=api_key)
    session_client = base_client.session(session_id)
    
    ips = []
    timings = []
    errors = []
    
    for i in range(request_count):
        try:
            start_time = time.time()
            response = session_client.get('https://api.ipify.org')
            timing = (time.time() - start_time) * 1000
            
            if response.status_code == 200:
                ip = response.text.strip()
                ips.append(ip)
                timings.append(timing)
                print(f"  Request {i+1:2d}: {ip} ({timing:.0f}ms)")
            else:
                errors.append(f"Request {i+1}: HTTP {response.status_code}")
                print(f"  Request {i+1:2d}: ERROR - HTTP {response.status_code}")
                
        except Exception as e:
            errors.append(f"Request {i+1}: {str(e)}")
            print(f"  Request {i+1:2d}: ERROR - {str(e)}")
        
        time.sleep(0.5)  # Small delay between requests
    
    # Analyze results
    unique_ips = set(ips)
    success_count = len(ips)
    error_count = len(errors)
    
    result = {
        'session_id': session_id,
        'total_requests': request_count,
        'successful_requests': success_count,
        'failed_requests': error_count,
        'unique_ips': len(unique_ips),
        'ips_used': list(unique_ips),
        'avg_response_time_ms': statistics.mean(timings) if timings else 0,
        'min_response_time_ms': min(timings) if timings else 0,
        'max_response_time_ms': max(timings) if timings else 0,
        'consistency_rate': (1 if len(unique_ips) <= 1 else 0) if unique_ips else 0,
        'errors': errors
    }
    
    print(f"\n📊 Session {session_id} Results:")
    print(f"   Successful requests: {success_count}/{request_count}")
    print(f"   Unique IPs: {len(unique_ips)}")
    if unique_ips:
        print(f"   IP(s) used: {', '.join(unique_ips)}")
    if timings:
        print(f"   Avg response time: {statistics.mean(timings):.0f}ms")
    if errors:
        print(f"   Errors: {len(errors)}")
        for error in errors[:3]:  # Show first 3 errors
            print(f"     - {error}")
    
    # Determine status
    if len(unique_ips) == 1 and success_count >= request_count * 0.9:
        print(f"   ✅ PASS - Perfect session consistency")
        result['status'] = 'PASS'
    elif len(unique_ips) <= 2 and success_count >= request_count * 0.8:
        print(f"   🔶 PARTIAL - Mostly consistent with minor IP changes")
        result['status'] = 'PARTIAL'
    else:
        print(f"   ❌ FAIL - Poor session consistency")
        result['status'] = 'FAIL'
    
    return result

def test_different_sessions(api_key, session_count=5):
    """Test that different sessions get different IPs"""
    print(f"\nTesting different sessions get different IPs ({session_count} sessions)")
    
    base_client = IPLoop(api_key=api_key)
    session_ips = {}
    session_results = []
    
    for i in range(session_count):
        session_id = f"test_different_{i+1}"
        print(f"  Testing session: {session_id}")
        
        try:
            session_client = base_client.session(session_id)
            start_time = time.time()
            response = session_client.get('https://api.ipify.org')
            timing = (time.time() - start_time) * 1000
            
            if response.status_code == 200:
                ip = response.text.strip()
                session_ips[session_id] = ip
                session_results.append({
                    'session_id': session_id,
                    'ip': ip,
                    'response_time_ms': timing,
                    'success': True
                })
                print(f"    IP: {ip} ({timing:.0f}ms)")
            else:
                session_results.append({
                    'session_id': session_id,
                    'error': f"HTTP {response.status_code}",
                    'success': False
                })
                print(f"    ERROR: HTTP {response.status_code}")
                
        except Exception as e:
            session_results.append({
                'session_id': session_id,
                'error': str(e),
                'success': False
            })
            print(f"    ERROR: {str(e)}")
        
        time.sleep(1)  # Delay between session creations
    
    # Analyze results
    successful_sessions = [r for r in session_results if r['success']]
    unique_ips = set(r['ip'] for r in successful_sessions)
    
    result = {
        'total_sessions': session_count,
        'successful_sessions': len(successful_sessions),
        'unique_ips': len(unique_ips),
        'diversity_rate': len(unique_ips) / len(successful_sessions) if successful_sessions else 0,
        'session_results': session_results,
        'ip_distribution': dict(Counter(r['ip'] for r in successful_sessions))
    }
    
    print(f"\n📊 Different Sessions Results:")
    print(f"   Successful sessions: {len(successful_sessions)}/{session_count}")
    print(f"   Unique IPs: {len(unique_ips)}")
    print(f"   Diversity rate: {result['diversity_rate']:.1%}")
    
    if len(unique_ips) >= session_count * 0.8:
        print(f"   ✅ PASS - Good IP diversity across sessions")
        result['status'] = 'PASS'
    elif len(unique_ips) >= session_count * 0.5:
        print(f"   🔶 PARTIAL - Moderate IP diversity")
        result['status'] = 'PARTIAL'
    else:
        print(f"   ❌ FAIL - Poor IP diversity - sessions sharing IPs")
        result['status'] = 'FAIL'
    
    return result

def test_session_persistence(api_key, session_id, delay_seconds=30):
    """Test session persistence over time"""
    print(f"\nTesting session persistence: {session_id} (with {delay_seconds}s delay)")
    
    base_client = IPLoop(api_key=api_key)
    session_client = base_client.session(session_id)
    
    # First request
    print("  Making first request...")
    try:
        start_time = time.time()
        response1 = session_client.get('https://api.ipify.org')
        timing1 = (time.time() - start_time) * 1000
        
        if response1.status_code == 200:
            ip1 = response1.text.strip()
            print(f"    First IP: {ip1} ({timing1:.0f}ms)")
        else:
            return {
                'session_id': session_id,
                'status': 'FAIL',
                'error': f"First request failed: HTTP {response1.status_code}"
            }
    except Exception as e:
        return {
            'session_id': session_id,
            'status': 'FAIL', 
            'error': f"First request failed: {str(e)}"
        }
    
    # Wait
    print(f"  Waiting {delay_seconds} seconds...")
    time.sleep(delay_seconds)
    
    # Second request
    print("  Making second request...")
    try:
        start_time = time.time()
        response2 = session_client.get('https://api.ipify.org')
        timing2 = (time.time() - start_time) * 1000
        
        if response2.status_code == 200:
            ip2 = response2.text.strip()
            print(f"    Second IP: {ip2} ({timing2:.0f}ms)")
        else:
            return {
                'session_id': session_id,
                'status': 'FAIL',
                'error': f"Second request failed: HTTP {response2.status_code}"
            }
    except Exception as e:
        return {
            'session_id': session_id,
            'status': 'FAIL',
            'error': f"Second request failed: {str(e)}"
        }
    
    # Analyze persistence
    result = {
        'session_id': session_id,
        'delay_seconds': delay_seconds,
        'first_ip': ip1,
        'second_ip': ip2,
        'persistent': ip1 == ip2,
        'first_timing_ms': timing1,
        'second_timing_ms': timing2
    }
    
    if ip1 == ip2:
        print(f"   ✅ PASS - Session persistent over {delay_seconds}s")
        result['status'] = 'PASS'
    else:
        print(f"   ❌ FAIL - Session IP changed: {ip1} → {ip2}")
        result['status'] = 'FAIL'
    
    return result

def main():
    parser = argparse.ArgumentParser(description='IPLoop Sticky Session Tests')
    parser.add_argument('--api-key', default='testkey123', help='IPLoop API key')
    parser.add_argument('--session-count', type=int, default=5, help='Number of different sessions to test')
    parser.add_argument('--requests-per-session', type=int, default=10, help='Requests per session for consistency test')
    parser.add_argument('--persistence-delay', type=int, default=30, help='Delay in seconds for persistence test')
    parser.add_argument('--output-file', help='Save results to JSON file')
    
    args = parser.parse_args()
    
    print("🚀 IPLoop Sticky Session Tests")
    print("=" * 50)
    print(f"API Key: {args.api_key}")
    print(f"Test Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 50)
    
    all_results = {
        'timestamp': datetime.now().isoformat(),
        'api_key': args.api_key,
        'test_config': {
            'session_count': args.session_count,
            'requests_per_session': args.requests_per_session,
            'persistence_delay': args.persistence_delay
        },
        'tests': {}
    }
    
    # Test 1: Session consistency
    print("\n🔒 Test 1: Session Consistency")
    consistency_result = test_session_consistency(args.api_key, "consistency_test", args.requests_per_session)
    all_results['tests']['consistency'] = consistency_result
    
    # Test 2: Different sessions
    print("\n🔄 Test 2: Different Sessions Get Different IPs")
    from collections import Counter  # Import here to avoid issues if not used
    diversity_result = test_different_sessions(args.api_key, args.session_count)
    all_results['tests']['diversity'] = diversity_result
    
    # Test 3: Session persistence
    print("\n⏰ Test 3: Session Persistence Over Time")
    persistence_result = test_session_persistence(args.api_key, "persistence_test", args.persistence_delay)
    all_results['tests']['persistence'] = persistence_result
    
    # Overall assessment
    print("\n" + "=" * 50)
    print("📊 STICKY SESSION TEST SUMMARY")
    print("=" * 50)
    
    tests = [
        ("Session Consistency", consistency_result['status']),
        ("Session Diversity", diversity_result['status']),
        ("Session Persistence", persistence_result['status'])
    ]
    
    pass_count = sum(1 for _, status in tests if status == 'PASS')
    partial_count = sum(1 for _, status in tests if status == 'PARTIAL')
    fail_count = sum(1 for _, status in tests if status == 'FAIL')
    
    for test_name, status in tests:
        emoji = {"PASS": "✅", "PARTIAL": "🔶", "FAIL": "❌"}[status]
        print(f"{emoji} {test_name}: {status}")
    
    print(f"\nOverall Results:")
    print(f"  ✅ Passed: {pass_count}/3")
    print(f"  🔶 Partial: {partial_count}/3") 
    print(f"  ❌ Failed: {fail_count}/3")
    
    # Final grade
    if pass_count == 3:
        print(f"\n🎉 EXCELLENT: All sticky session tests passed!")
        all_results['overall_grade'] = 'EXCELLENT'
    elif pass_count >= 2:
        print(f"\n🟡 GOOD: Most sticky session functionality works")
        all_results['overall_grade'] = 'GOOD'
    elif pass_count >= 1:
        print(f"\n🟠 NEEDS WORK: Some sticky session issues detected")
        all_results['overall_grade'] = 'NEEDS_WORK'
    else:
        print(f"\n🔴 POOR: Sticky session functionality has major problems")
        all_results['overall_grade'] = 'POOR'
    
    # Save results
    if args.output_file:
        with open(args.output_file, 'w') as f:
            json.dump(all_results, f, indent=2)
        print(f"\n💾 Results saved to: {args.output_file}")
    
    return all_results

if __name__ == "__main__":
    main()