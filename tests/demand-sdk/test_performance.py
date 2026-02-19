#!/usr/bin/env python3
"""
IPLoop Python SDK - Performance Benchmarks
Tests response times, throughput, concurrent performance, and scaling.

Usage:
    python test_performance.py [--api-key KEY] [--tests LIST] [--concurrent N]

Example:
    python test_performance.py --api-key testkey123 --tests basic,concurrent,scaling --concurrent 10
"""

import argparse
import json
import time
import statistics
import threading
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed
from iploop import IPLoop

def safe_request(client, url, timeout=10):
    """Make a safe request with timing"""
    start_time = time.time()
    try:
        response = client.get(url, timeout=timeout)
        response_time = (time.time() - start_time) * 1000
        return {
            'success': True,
            'response_time_ms': response_time,
            'status_code': response.status_code,
            'content_length': len(response.text) if hasattr(response, 'text') else 0,
            'error': None
        }
    except Exception as e:
        response_time = (time.time() - start_time) * 1000
        return {
            'success': False,
            'response_time_ms': response_time,
            'status_code': None,
            'content_length': 0,
            'error': str(e)
        }

def test_basic_performance(api_key, request_count=50, target_url='https://httpbin.org/ip'):
    """Test basic sequential performance"""
    print(f"\n⚡ Basic Performance Test ({request_count} sequential requests)")
    print(f"   Target: {target_url}")
    
    client = IPLoop(api_key=api_key)
    results = []
    
    start_time = time.time()
    
    for i in range(request_count):
        result = safe_request(client, target_url)
        results.append(result)
        
        if i % 10 == 0 and i > 0:
            print(f"   Progress: {i}/{request_count}")
    
    total_time = time.time() - start_time
    
    # Calculate statistics
    successful_requests = [r for r in results if r['success']]
    failed_requests = [r for r in results if not r['success']]
    
    if successful_requests:
        response_times = [r['response_time_ms'] for r in successful_requests]
        
        stats = {
            'total_requests': request_count,
            'successful_requests': len(successful_requests),
            'failed_requests': len(failed_requests),
            'success_rate': len(successful_requests) / request_count,
            'total_time_seconds': total_time,
            'requests_per_second': request_count / total_time,
            'avg_response_time_ms': statistics.mean(response_times),
            'median_response_time_ms': statistics.median(response_times),
            'min_response_time_ms': min(response_times),
            'max_response_time_ms': max(response_times),
            'p95_response_time_ms': statistics.quantiles(response_times, n=20)[18] if len(response_times) >= 20 else max(response_times),
            'p99_response_time_ms': statistics.quantiles(response_times, n=100)[98] if len(response_times) >= 100 else max(response_times),
            'response_time_std_ms': statistics.stdev(response_times) if len(response_times) > 1 else 0
        }
    else:
        stats = {
            'total_requests': request_count,
            'successful_requests': 0,
            'failed_requests': len(failed_requests),
            'success_rate': 0,
            'total_time_seconds': total_time,
            'requests_per_second': 0
        }
    
    print(f"\n   📊 Basic Performance Results:")
    print(f"      Success rate: {stats.get('success_rate', 0):.1%} ({stats.get('successful_requests', 0)}/{request_count})")
    print(f"      Total time: {total_time:.1f}s")
    print(f"      Throughput: {stats.get('requests_per_second', 0):.1f} req/s")
    
    if 'avg_response_time_ms' in stats:
        print(f"      Avg response: {stats['avg_response_time_ms']:.0f}ms")
        print(f"      P50 response: {stats['median_response_time_ms']:.0f}ms")
        print(f"      P95 response: {stats['p95_response_time_ms']:.0f}ms")
        print(f"      Min/Max: {stats['min_response_time_ms']:.0f}ms / {stats['max_response_time_ms']:.0f}ms")
    
    if failed_requests:
        error_types = {}
        for req in failed_requests:
            error_types[req['error']] = error_types.get(req['error'], 0) + 1
        print(f"      Top errors: {dict(list(error_types.items())[:3])}")
    
    return {
        'test_name': 'basic_performance',
        'config': {'request_count': request_count, 'target_url': target_url},
        'stats': stats,
        'raw_results': results
    }

def test_concurrent_performance(api_key, concurrent_workers=10, requests_per_worker=5, target_url='https://httpbin.org/ip'):
    """Test concurrent request performance"""
    print(f"\n🔀 Concurrent Performance Test ({concurrent_workers} workers × {requests_per_worker} requests)")
    print(f"   Target: {target_url}")
    print(f"   Total requests: {concurrent_workers * requests_per_worker}")
    
    def worker_function(worker_id):
        """Function for each worker thread"""
        client = IPLoop(api_key=api_key)
        worker_results = []
        
        for i in range(requests_per_worker):
            result = safe_request(client, target_url)
            result['worker_id'] = worker_id
            result['request_id'] = i
            worker_results.append(result)
        
        return worker_results
    
    # Run concurrent test
    start_time = time.time()
    all_results = []
    
    with ThreadPoolExecutor(max_workers=concurrent_workers) as executor:
        # Submit all workers
        futures = [executor.submit(worker_function, i) for i in range(concurrent_workers)]
        
        # Collect results
        completed = 0
        for future in as_completed(futures):
            worker_results = future.result()
            all_results.extend(worker_results)
            completed += 1
            print(f"   Worker completed: {completed}/{concurrent_workers}")
    
    total_time = time.time() - start_time
    
    # Analyze results
    successful_requests = [r for r in all_results if r['success']]
    failed_requests = [r for r in all_results if not r['success']]
    
    if successful_requests:
        response_times = [r['response_time_ms'] for r in successful_requests]
        
        stats = {
            'total_requests': len(all_results),
            'successful_requests': len(successful_requests),
            'failed_requests': len(failed_requests),
            'success_rate': len(successful_requests) / len(all_results),
            'total_time_seconds': total_time,
            'concurrent_workers': concurrent_workers,
            'requests_per_worker': requests_per_worker,
            'actual_throughput_rps': len(all_results) / total_time,
            'avg_response_time_ms': statistics.mean(response_times),
            'median_response_time_ms': statistics.median(response_times),
            'min_response_time_ms': min(response_times),
            'max_response_time_ms': max(response_times),
            'p95_response_time_ms': statistics.quantiles(response_times, n=20)[18] if len(response_times) >= 20 else max(response_times)
        }
    else:
        stats = {
            'total_requests': len(all_results),
            'successful_requests': 0,
            'failed_requests': len(failed_requests),
            'success_rate': 0,
            'total_time_seconds': total_time,
            'concurrent_workers': concurrent_workers,
            'requests_per_worker': requests_per_worker,
            'actual_throughput_rps': 0
        }
    
    print(f"\n   📊 Concurrent Performance Results:")
    print(f"      Success rate: {stats['success_rate']:.1%} ({stats['successful_requests']}/{stats['total_requests']})")
    print(f"      Total time: {total_time:.1f}s")
    print(f"      Throughput: {stats['actual_throughput_rps']:.1f} req/s")
    
    if 'avg_response_time_ms' in stats:
        print(f"      Avg response: {stats['avg_response_time_ms']:.0f}ms")
        print(f"      P95 response: {stats['p95_response_time_ms']:.0f}ms")
    
    return {
        'test_name': 'concurrent_performance',
        'config': {
            'concurrent_workers': concurrent_workers,
            'requests_per_worker': requests_per_worker,
            'target_url': target_url
        },
        'stats': stats,
        'raw_results': all_results
    }

def test_scaling_performance(api_key, worker_counts=[1, 2, 5, 10], requests_per_test=20):
    """Test how performance scales with concurrency"""
    print(f"\n📈 Scaling Performance Test")
    print(f"   Worker counts: {worker_counts}")
    print(f"   Requests per test: {requests_per_test}")
    
    scaling_results = []
    
    for worker_count in worker_counts:
        print(f"\n   Testing with {worker_count} workers...")
        
        # Run concurrent test with this worker count
        result = test_concurrent_performance(
            api_key, 
            concurrent_workers=worker_count,
            requests_per_worker=requests_per_test // worker_count,
            target_url='https://httpbin.org/ip'
        )
        
        scaling_results.append({
            'worker_count': worker_count,
            'throughput_rps': result['stats']['actual_throughput_rps'],
            'avg_response_time_ms': result['stats'].get('avg_response_time_ms', 0),
            'success_rate': result['stats']['success_rate'],
            'total_time_seconds': result['stats']['total_time_seconds']
        })
        
        time.sleep(2)  # Brief pause between scaling tests
    
    print(f"\n   📊 Scaling Results:")
    print(f"      Workers | Throughput | Avg Response | Success Rate")
    print(f"      --------|------------|--------------|-------------")
    
    for result in scaling_results:
        print(f"      {result['worker_count']:7d} | {result['throughput_rps']:8.1f}rps | {result['avg_response_time_ms']:10.0f}ms | {result['success_rate']:10.1%}")
    
    # Calculate scaling efficiency
    baseline_throughput = scaling_results[0]['throughput_rps']
    scaling_analysis = []
    
    for result in scaling_results:
        workers = result['worker_count']
        throughput = result['throughput_rps']
        
        expected_throughput = baseline_throughput * workers
        scaling_efficiency = throughput / expected_throughput if expected_throughput > 0 else 0
        
        scaling_analysis.append({
            'workers': workers,
            'throughput': throughput,
            'expected_throughput': expected_throughput,
            'scaling_efficiency': scaling_efficiency
        })
    
    print(f"\n   📈 Scaling Efficiency:")
    print(f"      Workers | Actual RPS | Expected RPS | Efficiency")
    print(f"      --------|------------|--------------|----------")
    
    for analysis in scaling_analysis:
        print(f"      {analysis['workers']:7d} | {analysis['throughput']:8.1f}rps | {analysis['expected_throughput']:10.1f}rps | {analysis['scaling_efficiency']:8.1%}")
    
    return {
        'test_name': 'scaling_performance',
        'config': {
            'worker_counts': worker_counts,
            'requests_per_test': requests_per_test
        },
        'scaling_results': scaling_results,
        'scaling_analysis': scaling_analysis
    }

def test_different_endpoints(api_key, request_count=20):
    """Test performance across different types of endpoints"""
    print(f"\n🎯 Different Endpoints Performance Test ({request_count} requests each)")
    
    endpoints = [
        ('Simple IP', 'https://api.ipify.org'),
        ('JSON Response', 'https://httpbin.org/ip'),
        ('Headers Echo', 'https://httpbin.org/headers'),
        ('User Agent', 'https://httpbin.org/user-agent'),
        ('Large Response', 'https://httpbin.org/bytes/10240'),  # 10KB
        ('Slow Response', 'https://httpbin.org/delay/2'),      # 2s delay
    ]
    
    client = IPLoop(api_key=api_key)
    endpoint_results = []
    
    for name, url in endpoints:
        print(f"   Testing: {name}")
        
        results = []
        for i in range(request_count):
            if 'delay' in url:
                result = safe_request(client, url, timeout=15)  # Longer timeout for delay endpoint
            else:
                result = safe_request(client, url, timeout=10)
            results.append(result)
        
        # Analyze results
        successful = [r for r in results if r['success']]
        
        if successful:
            response_times = [r['response_time_ms'] for r in successful]
            content_sizes = [r['content_length'] for r in successful]
            
            endpoint_stats = {
                'endpoint_name': name,
                'url': url,
                'total_requests': request_count,
                'successful_requests': len(successful),
                'success_rate': len(successful) / request_count,
                'avg_response_time_ms': statistics.mean(response_times),
                'median_response_time_ms': statistics.median(response_times),
                'avg_content_size_bytes': statistics.mean(content_sizes),
                'throughput_bytes_per_second': statistics.mean(content_sizes) * len(successful) / sum(response_times) * 1000 if response_times else 0
            }
        else:
            endpoint_stats = {
                'endpoint_name': name,
                'url': url,
                'total_requests': request_count,
                'successful_requests': 0,
                'success_rate': 0
            }
        
        endpoint_results.append(endpoint_stats)
        
        print(f"      Success: {endpoint_stats['success_rate']:.1%}")
        if 'avg_response_time_ms' in endpoint_stats:
            print(f"      Avg time: {endpoint_stats['avg_response_time_ms']:.0f}ms")
            print(f"      Avg size: {endpoint_stats['avg_content_size_bytes']:.0f} bytes")
    
    print(f"\n   📊 Endpoint Comparison:")
    print(f"      Endpoint          | Success | Avg Time | Avg Size")
    print(f"      ------------------|---------|----------|----------")
    
    for result in endpoint_results:
        if 'avg_response_time_ms' in result:
            print(f"      {result['endpoint_name']:17s} | {result['success_rate']:6.1%} | {result['avg_response_time_ms']:7.0f}ms | {result['avg_content_size_bytes']:7.0f}B")
        else:
            print(f"      {result['endpoint_name']:17s} | {result['success_rate']:6.1%} |    FAIL  |    N/A")
    
    return {
        'test_name': 'different_endpoints',
        'config': {'request_count': request_count, 'endpoints': len(endpoints)},
        'endpoint_results': endpoint_results
    }

def test_session_performance(api_key, session_count=5, requests_per_session=10):
    """Test performance of sticky sessions vs no sessions"""
    print(f"\n🔒 Session Performance Test ({session_count} sessions × {requests_per_session} requests)")
    
    base_client = IPLoop(api_key=api_key)
    
    # Test 1: No sessions (new client each time)
    print(f"   Testing without sessions...")
    no_session_times = []
    
    for i in range(session_count * requests_per_session):
        client = IPLoop(api_key=api_key)  # New client each time
        result = safe_request(client, 'https://api.ipify.org')
        if result['success']:
            no_session_times.append(result['response_time_ms'])
    
    # Test 2: With sticky sessions
    print(f"   Testing with sticky sessions...")
    session_times = []
    
    for session_id in range(session_count):
        session_client = base_client.session(f"perf_test_{session_id}")
        
        for request_id in range(requests_per_session):
            result = safe_request(session_client, 'https://api.ipify.org')
            if result['success']:
                session_times.append(result['response_time_ms'])
    
    # Compare results
    comparison = {}
    
    if no_session_times:
        comparison['no_sessions'] = {
            'count': len(no_session_times),
            'avg_ms': statistics.mean(no_session_times),
            'median_ms': statistics.median(no_session_times),
            'min_ms': min(no_session_times),
            'max_ms': max(no_session_times)
        }
    
    if session_times:
        comparison['with_sessions'] = {
            'count': len(session_times),
            'avg_ms': statistics.mean(session_times),
            'median_ms': statistics.median(session_times),
            'min_ms': min(session_times),
            'max_ms': max(session_times)
        }
    
    if no_session_times and session_times:
        improvement = (statistics.mean(no_session_times) - statistics.mean(session_times)) / statistics.mean(no_session_times)
        comparison['improvement_percent'] = improvement
    
    print(f"\n   📊 Session Performance Comparison:")
    if 'no_sessions' in comparison:
        print(f"      Without sessions: {comparison['no_sessions']['avg_ms']:.0f}ms avg ({comparison['no_sessions']['count']} requests)")
    if 'with_sessions' in comparison:
        print(f"      With sessions:    {comparison['with_sessions']['avg_ms']:.0f}ms avg ({comparison['with_sessions']['count']} requests)")
    if 'improvement_percent' in comparison:
        if comparison['improvement_percent'] > 0:
            print(f"      Sessions are {comparison['improvement_percent']:.1%} faster")
        else:
            print(f"      Sessions are {-comparison['improvement_percent']:.1%} slower")
    
    return {
        'test_name': 'session_performance',
        'config': {
            'session_count': session_count,
            'requests_per_session': requests_per_session
        },
        'comparison': comparison
    }

def main():
    parser = argparse.ArgumentParser(description='IPLoop Performance Benchmarks')
    parser.add_argument('--api-key', default='testkey123', help='IPLoop API key')
    parser.add_argument('--tests', default='basic,concurrent,scaling,endpoints,sessions',
                       help='Comma-separated list of tests to run')
    parser.add_argument('--basic-requests', type=int, default=50, help='Number of requests for basic test')
    parser.add_argument('--concurrent-workers', type=int, default=10, help='Number of concurrent workers')
    parser.add_argument('--concurrent-requests-per-worker', type=int, default=5, help='Requests per worker')
    parser.add_argument('--output-file', help='Save results to JSON file')
    
    args = parser.parse_args()
    
    tests_to_run = [t.strip() for t in args.tests.split(',')]
    available_tests = ['basic', 'concurrent', 'scaling', 'endpoints', 'sessions']
    
    invalid_tests = [t for t in tests_to_run if t not in available_tests]
    if invalid_tests:
        print(f"❌ Invalid tests: {invalid_tests}")
        print(f"Available tests: {', '.join(available_tests)}")
        return
    
    print("⚡ IPLoop Performance Benchmarks")
    print("=" * 50)
    print(f"API Key: {args.api_key}")
    print(f"Tests: {', '.join(tests_to_run)}")
    print(f"Test Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 50)
    
    all_results = {
        'timestamp': datetime.now().isoformat(),
        'api_key': args.api_key,
        'tests_config': {
            'tests_run': tests_to_run,
            'basic_requests': args.basic_requests,
            'concurrent_workers': args.concurrent_workers,
            'concurrent_requests_per_worker': args.concurrent_requests_per_worker
        },
        'test_results': {}
    }
    
    # Run selected tests
    if 'basic' in tests_to_run:
        result = test_basic_performance(args.api_key, args.basic_requests)
        all_results['test_results']['basic'] = result
    
    if 'concurrent' in tests_to_run:
        result = test_concurrent_performance(
            args.api_key, 
            args.concurrent_workers, 
            args.concurrent_requests_per_worker
        )
        all_results['test_results']['concurrent'] = result
    
    if 'scaling' in tests_to_run:
        result = test_scaling_performance(args.api_key)
        all_results['test_results']['scaling'] = result
    
    if 'endpoints' in tests_to_run:
        result = test_different_endpoints(args.api_key)
        all_results['test_results']['endpoints'] = result
    
    if 'sessions' in tests_to_run:
        result = test_session_performance(args.api_key)
        all_results['test_results']['sessions'] = result
    
    # Overall assessment
    print("\n" + "=" * 50)
    print("📊 PERFORMANCE SUMMARY")
    print("=" * 50)
    
    # Extract key metrics
    basic_stats = all_results['test_results'].get('basic', {}).get('stats', {})
    concurrent_stats = all_results['test_results'].get('concurrent', {}).get('stats', {})
    
    if basic_stats:
        print(f"\n⚡ Key Performance Metrics:")
        print(f"  Sequential throughput: {basic_stats.get('requests_per_second', 0):.1f} req/s")
        print(f"  Average response time: {basic_stats.get('avg_response_time_ms', 0):.0f}ms")
        print(f"  P95 response time: {basic_stats.get('p95_response_time_ms', 0):.0f}ms")
        print(f"  Success rate: {basic_stats.get('success_rate', 0):.1%}")
    
    if concurrent_stats:
        print(f"  Concurrent throughput: {concurrent_stats.get('actual_throughput_rps', 0):.1f} req/s")
        print(f"  Concurrent avg response: {concurrent_stats.get('avg_response_time_ms', 0):.0f}ms")
    
    # Performance grade
    basic_avg = basic_stats.get('avg_response_time_ms', 10000)
    success_rate = basic_stats.get('success_rate', 0)
    
    if basic_avg < 1000 and success_rate > 0.95:
        grade = "EXCELLENT"
        grade_emoji = "🎉"
    elif basic_avg < 2000 and success_rate > 0.90:
        grade = "GOOD"
        grade_emoji = "🟢"
    elif basic_avg < 5000 and success_rate > 0.80:
        grade = "ACCEPTABLE"
        grade_emoji = "🟡"
    else:
        grade = "NEEDS_IMPROVEMENT"
        grade_emoji = "🔴"
    
    print(f"\n{grade_emoji} Performance Grade: {grade}")
    all_results['performance_grade'] = grade
    
    # Save results
    if args.output_file:
        with open(args.output_file, 'w') as f:
            json.dump(all_results, f, indent=2)
        print(f"\n💾 Results saved to: {args.output_file}")
    
    return all_results

if __name__ == "__main__":
    main()