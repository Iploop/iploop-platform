#!/usr/bin/env python3
"""
IPLoop Python SDK - Complete Test Suite
Comprehensive testing of all SDK functionality including real-world use cases.

Usage:
    python test_full.py [--api-key KEY] [--output FORMAT] [--verbose]

Example:
    python test_full.py --api-key testkey123 --output json --verbose
"""

import argparse
import json
import time
import statistics
import sys
from datetime import datetime
from iploop import IPLoop
import requests

class IPLoopTester:
    def __init__(self, api_key, verbose=False):
        self.api_key = api_key
        self.verbose = verbose
        self.results = []
        self.start_time = time.time()
        
    def log(self, message, level="INFO"):
        """Log message with timestamp"""
        timestamp = datetime.now().strftime("%H:%M:%S")
        if self.verbose or level in ["ERROR", "CRITICAL"]:
            print(f"[{timestamp}] {level}: {message}")
        sys.stdout.flush()
        
    def log_result(self, test_name, status, timing=None, details=None, category="General"):
        """Log test result"""
        emoji = {"PASS": "✅", "FAIL": "❌", "PARTIAL": "🔶", "BLOCKED": "🚫", "TIMEOUT": "⏰"}.get(status, "❓")
        
        result = {
            'test_name': test_name,
            'category': category,
            'status': status,
            'timing_ms': timing,
            'details': details,
            'timestamp': datetime.now().isoformat()
        }
        self.results.append(result)
        
        timing_str = f" ({timing:.0f}ms)" if timing else ""
        print(f"{emoji} [{category}] {test_name} - {status}{timing_str}")
        if details:
            print(f"    → {details}")
            
    def safe_request(self, client, method, url, **kwargs):
        """Safely execute request with error handling"""
        start_time = time.time() * 1000
        try:
            func = getattr(client, method.lower())
            response = func(url, **kwargs)
            timing = time.time() * 1000 - start_time
            return True, response, timing, None
        except Exception as e:
            timing = time.time() * 1000 - start_time
            return False, None, timing, str(e)

    def test_basic_connectivity(self):
        """Test basic connectivity and HTTP/HTTPS support"""
        self.log("Starting basic connectivity tests")
        
        client = IPLoop(api_key=self.api_key)
        
        # Test HTTPS
        success, response, timing, error = self.safe_request(client, 'get', 'https://httpbin.org/ip')
        if success and response.status_code == 200:
            ip_data = response.json()
            self.log_result("Basic HTTPS Request", "PASS", timing, 
                          f"IP: {ip_data.get('origin')}", "Connectivity")
        else:
            self.log_result("Basic HTTPS Request", "FAIL", timing, 
                          error, "Connectivity")
            
        # Test HTTP vs HTTPS comparison
        success_http, _, timing_http, _ = self.safe_request(client, 'get', 'http://httpbin.org/ip')
        success_https, _, timing_https, _ = self.safe_request(client, 'get', 'https://httpbin.org/ip')
        
        if success_http and success_https:
            avg_timing = (timing_http + timing_https) / 2
            self.log_result("HTTP vs HTTPS", "PASS", avg_timing,
                          f"HTTP: {timing_http:.0f}ms, HTTPS: {timing_https:.0f}ms", "Connectivity")
        else:
            self.log_result("HTTP vs HTTPS", "FAIL", None, 
                          "One or both protocols failed", "Connectivity")

    def test_sticky_sessions(self):
        """Test sticky session functionality"""
        self.log("Starting sticky session tests")
        
        base_client = IPLoop(api_key=self.api_key)
        
        # Test session IP consistency (10 requests)
        session_client = base_client.session("test_session_consistency")
        ips = []
        timings = []
        
        for i in range(10):
            success, response, timing, error = self.safe_request(session_client, 'get', 'https://api.ipify.org')
            if success:
                ips.append(response.text.strip())
                timings.append(timing)
        
        unique_ips = set(ips)
        avg_timing = statistics.mean(timings) if timings else 0
        
        if len(ips) == 10 and len(unique_ips) == 1:
            self.log_result("Session Consistency", "PASS", avg_timing,
                          f"All 10 requests used IP: {list(unique_ips)[0]}", "Sessions")
        else:
            self.log_result("Session Consistency", "FAIL", avg_timing,
                          f"Got {len(unique_ips)} unique IPs from {len(ips)} requests", "Sessions")
        
        # Test different sessions get different IPs
        session_ips = {}
        for i in range(1, 4):
            client_session = base_client.session(f"test_different_{i}")
            success, response, timing, error = self.safe_request(client_session, 'get', 'https://api.ipify.org')
            if success:
                session_ips[f"session_{i}"] = response.text.strip()
        
        unique_session_ips = set(session_ips.values())
        if len(unique_session_ips) >= 2:
            self.log_result("Different Sessions Different IPs", "PASS", None,
                          f"Got {len(unique_session_ips)} unique IPs from 3 sessions", "Sessions")
        else:
            self.log_result("Different Sessions Different IPs", "FAIL", None,
                          f"Sessions all got same IP: {session_ips}", "Sessions")

    def test_ip_rotation(self):
        """Test IP rotation without sessions"""
        self.log("Starting IP rotation tests")
        
        rotation_ips = []
        timings = []
        
        for i in range(10):
            client = IPLoop(api_key=self.api_key)  # New client each time
            success, response, timing, error = self.safe_request(client, 'get', 'https://api.ipify.org')
            if success:
                rotation_ips.append(response.text.strip())
                timings.append(timing)
            time.sleep(0.5)  # Small delay
        
        unique_ips = set(rotation_ips)
        avg_timing = statistics.mean(timings) if timings else 0
        
        if len(unique_ips) >= 5:
            self.log_result("IP Rotation", "PASS", avg_timing,
                          f"{len(unique_ips)}/10 unique IPs", "Rotation")
        elif len(unique_ips) >= 3:
            self.log_result("IP Rotation", "PARTIAL", avg_timing,
                          f"{len(unique_ips)}/10 unique IPs (limited rotation)", "Rotation")
        else:
            self.log_result("IP Rotation", "FAIL", avg_timing,
                          f"Poor rotation: only {len(unique_ips)} unique IPs", "Rotation")

    def test_country_targeting(self):
        """Test country targeting functionality"""
        self.log("Starting country targeting tests")
        
        countries = ["US", "GB", "DE", "FR", "CA"]
        working_countries = []
        
        for country in countries:
            client = IPLoop(api_key=self.api_key, country=country)
            success, response, timing, error = self.safe_request(client, 'get', 'https://api.ipify.org')
            
            if success:
                ip = response.text.strip()
                
                # Verify country with geo API
                try:
                    geo_response = requests.get(f'http://ip-api.com/json/{ip}', timeout=5)
                    geo_data = geo_response.json()
                    detected_country = geo_data.get('countryCode', 'XX')
                    
                    if detected_country == country:
                        working_countries.append(country)
                        self.log_result(f"Country Targeting: {country}", "PASS", timing,
                                      f"IP: {ip}, Verified: {detected_country}", "Country")
                    else:
                        self.log_result(f"Country Targeting: {country}", "FAIL", timing,
                                      f"Expected {country}, got {detected_country}", "Country")
                except Exception as geo_error:
                    self.log_result(f"Country Targeting: {country}", "PARTIAL", timing,
                                  f"IP: {ip}, Geo check failed: {geo_error}", "Country")
            else:
                self.log_result(f"Country Targeting: {country}", "FAIL", timing,
                              error, "Country")
            
            time.sleep(1)  # Rate limiting
        
        # Summary
        success_rate = len(working_countries) / len(countries) * 100
        self.log_result("Country Targeting Overall", 
                      "PASS" if success_rate >= 60 else "PARTIAL" if success_rate >= 30 else "FAIL",
                      None, f"{len(working_countries)}/{len(countries)} countries working ({success_rate:.0f}%)", 
                      "Country")

    def test_web_scraping_use_cases(self):
        """Test real-world web scraping scenarios"""
        self.log("Starting web scraping use case tests")
        
        # E-commerce sites
        ecommerce_sites = [
            ("Amazon Product", "https://www.amazon.com/dp/B0D1XD1ZV3"),
            ("eBay Search", "https://www.ebay.com/sch/i.html?_nkw=iphone"),
            ("Walmart Search", "https://www.walmart.com/search?q=laptop"),
        ]
        
        # Travel sites
        travel_sites = [
            ("Booking.com", "https://www.booking.com/"),
            ("Expedia", "https://www.expedia.com/"),
            ("Kayak", "https://www.kayak.com/"),
        ]
        
        # Social media
        social_sites = [
            ("Instagram", "https://www.instagram.com/"),
            ("Reddit", "https://www.reddit.com/"),
            ("Twitter", "https://twitter.com/"),
        ]
        
        all_sites = [
            (ecommerce_sites, "E-Commerce"),
            (travel_sites, "Travel"),
            (social_sites, "Social Media")
        ]
        
        client = IPLoop(api_key=self.api_key)
        
        for sites, category in all_sites:
            for site_name, url in sites:
                success, response, timing, error = self.safe_request(client, 'get', url, timeout=15)
                
                if success:
                    status_code = response.status_code
                    content = response.text.lower()
                    
                    # Check for blocks/CAPTCHAs
                    blocked_indicators = ['captcha', 'blocked', 'access denied', 'cloudflare', 'security check']
                    is_blocked = any(indicator in content for indicator in blocked_indicators)
                    
                    if status_code == 200 and not is_blocked:
                        self.log_result(f"{site_name}", "PASS", timing,
                                      f"Status {status_code}, content loaded", category)
                    elif is_blocked or status_code == 403:
                        self.log_result(f"{site_name}", "BLOCKED", timing,
                                      f"CAPTCHA/block detected (Status {status_code})", category)
                    else:
                        self.log_result(f"{site_name}", "PARTIAL", timing,
                                      f"Status {status_code}", category)
                else:
                    self.log_result(f"{site_name}", "FAIL", timing, error, category)
                
                time.sleep(1)  # Rate limiting

    def test_performance(self):
        """Test performance characteristics"""
        self.log("Starting performance tests")
        
        client = IPLoop(api_key=self.api_key)
        
        # Sequential requests test
        sequential_times = []
        for i in range(20):
            success, response, timing, error = self.safe_request(client, 'get', 'https://httpbin.org/ip')
            if success and response.status_code == 200:
                sequential_times.append(timing)
        
        if sequential_times:
            avg_time = statistics.mean(sequential_times)
            p50_time = statistics.median(sequential_times)
            min_time = min(sequential_times)
            max_time = max(sequential_times)
            
            self.log_result("Sequential Performance", "PASS", avg_time,
                          f"20 req: avg={avg_time:.0f}ms p50={p50_time:.0f}ms min={min_time:.0f}ms max={max_time:.0f}ms",
                          "Performance")
        else:
            self.log_result("Sequential Performance", "FAIL", None,
                          "No successful requests", "Performance")

    def test_error_handling(self):
        """Test error handling and edge cases"""
        self.log("Starting error handling tests")
        
        # Invalid API key test
        bad_client = IPLoop(api_key="invalid_key_12345")
        success, response, timing, error = self.safe_request(bad_client, 'get', 'https://httpbin.org/ip')
        
        if not success or (response and response.status_code in [401, 403]):
            self.log_result("Invalid API Key", "PASS", timing,
                          "Properly rejected invalid key", "Error Handling")
        else:
            self.log_result("Invalid API Key", "FAIL", timing,
                          f"Should have failed but got {response.status_code if response else 'no response'}",
                          "Error Handling")
        
        # Timeout test
        client = IPLoop(api_key=self.api_key)
        success, response, timing, error = self.safe_request(client, 'get', 'https://httpbin.org/delay/10', timeout=5)
        
        if not success and "timeout" in str(error).lower():
            self.log_result("Timeout Handling", "PASS", timing,
                          "Properly timed out", "Error Handling")
        else:
            self.log_result("Timeout Handling", "PARTIAL", timing,
                          f"Unexpected result: {error}", "Error Handling")

    def run_all_tests(self):
        """Run complete test suite"""
        print("🚀 IPLoop Python SDK - Complete Test Suite")
        print("=" * 60)
        print(f"API Key: {self.api_key}")
        print(f"Start Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print("=" * 60)
        
        # Run all test categories
        try:
            self.test_basic_connectivity()
            self.test_sticky_sessions()
            self.test_ip_rotation()
            self.test_country_targeting()
            self.test_web_scraping_use_cases()
            self.test_performance()
            self.test_error_handling()
        except KeyboardInterrupt:
            self.log("Tests interrupted by user", "WARNING")
        except Exception as e:
            self.log(f"Unexpected error during testing: {e}", "ERROR")
        
        return self.generate_report()

    def generate_report(self):
        """Generate comprehensive test report"""
        total_time = time.time() - self.start_time
        
        # Count results by status
        status_counts = {}
        category_stats = {}
        timings = []
        
        for result in self.results:
            status = result['status']
            category = result['category']
            
            status_counts[status] = status_counts.get(status, 0) + 1
            
            if category not in category_stats:
                category_stats[category] = {'PASS': 0, 'FAIL': 0, 'PARTIAL': 0, 'BLOCKED': 0, 'TIMEOUT': 0}
            category_stats[category][status] += 1
            
            if result['timing_ms']:
                timings.append(result['timing_ms'])
        
        total_tests = len(self.results)
        success_rate = (status_counts.get('PASS', 0) / total_tests * 100) if total_tests > 0 else 0
        
        print("\n" + "=" * 60)
        print("📊 TEST REPORT SUMMARY")
        print("=" * 60)
        
        print(f"\n🔢 Overall Statistics:")
        print(f"   Total tests: {total_tests}")
        print(f"   ✅ Passed: {status_counts.get('PASS', 0)} ({status_counts.get('PASS', 0)/total_tests*100:.1f}%)")
        print(f"   ❌ Failed: {status_counts.get('FAIL', 0)} ({status_counts.get('FAIL', 0)/total_tests*100:.1f}%)")
        print(f"   🔶 Partial: {status_counts.get('PARTIAL', 0)} ({status_counts.get('PARTIAL', 0)/total_tests*100:.1f}%)")
        print(f"   🚫 Blocked: {status_counts.get('BLOCKED', 0)} ({status_counts.get('BLOCKED', 0)/total_tests*100:.1f}%)")
        
        if timings:
            avg_timing = statistics.mean(timings)
            print(f"   ⏱️ Avg response: {avg_timing:.0f}ms")
        
        print(f"   🕒 Total duration: {total_time:.1f}s")
        
        print(f"\n📋 By Category:")
        for category, stats in category_stats.items():
            total_cat = sum(stats.values())
            pass_rate = stats['PASS'] / total_cat * 100 if total_cat > 0 else 0
            print(f"   {category}: {stats['PASS']}/{total_cat} passed ({pass_rate:.0f}%)")
        
        # Generate report data
        report = {
            'timestamp': datetime.now().isoformat(),
            'total_duration_seconds': total_time,
            'summary': {
                'total_tests': total_tests,
                'success_rate_percent': success_rate,
                'status_counts': status_counts,
                'category_stats': category_stats,
                'avg_response_time_ms': statistics.mean(timings) if timings else 0
            },
            'detailed_results': self.results
        }
        
        return report

def main():
    parser = argparse.ArgumentParser(description='IPLoop Python SDK Complete Test Suite')
    parser.add_argument('--api-key', default='testkey123', help='IPLoop API key (default: testkey123)')
    parser.add_argument('--output', choices=['json', 'console', 'both'], default='both', 
                       help='Output format (default: both)')
    parser.add_argument('--verbose', '-v', action='store_true', help='Verbose logging')
    
    args = parser.parse_args()
    
    tester = IPLoopTester(args.api_key, verbose=args.verbose)
    report = tester.run_all_tests()
    
    if args.output in ['json', 'both']:
        filename = f"iploop-test-results-{datetime.now().strftime('%Y%m%d-%H%M%S')}.json"
        with open(filename, 'w') as f:
            json.dump(report, f, indent=2)
        print(f"\n💾 Results saved to: {filename}")
    
    # Final assessment
    success_rate = report['summary']['success_rate_percent']
    print(f"\n🎯 Final Assessment:")
    if success_rate >= 80:
        print(f"   🟢 EXCELLENT: {success_rate:.1f}% success rate - Production ready!")
    elif success_rate >= 60:
        print(f"   🟡 GOOD: {success_rate:.1f}% success rate - Minor issues to address")
    elif success_rate >= 40:
        print(f"   🟠 FAIR: {success_rate:.1f}% success rate - Significant improvements needed")
    else:
        print(f"   🔴 POOR: {success_rate:.1f}% success rate - Major issues require immediate attention")
    
    return report

if __name__ == "__main__":
    main()