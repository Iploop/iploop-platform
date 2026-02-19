#!/usr/bin/env python3
"""
IPLoop Python SDK - Country Targeting Tests
Tests geo-targeting functionality and country verification.

Usage:
    python test_countries.py [--api-key KEY] [--countries LIST] [--verify-geo]

Example:
    python test_countries.py --api-key testkey123 --countries US,GB,DE,FR,CA --verify-geo
"""

import argparse
import json
import time
import requests
from datetime import datetime
from iploop import IPLoop

# Major countries with their expected names/codes
COUNTRIES_DB = {
    'US': {'name': 'United States', 'region': 'North America'},
    'GB': {'name': 'United Kingdom', 'region': 'Europe'},
    'DE': {'name': 'Germany', 'region': 'Europe'}, 
    'FR': {'name': 'France', 'region': 'Europe'},
    'CA': {'name': 'Canada', 'region': 'North America'},
    'AU': {'name': 'Australia', 'region': 'Oceania'},
    'BR': {'name': 'Brazil', 'region': 'South America'},
    'IN': {'name': 'India', 'region': 'Asia'},
    'JP': {'name': 'Japan', 'region': 'Asia'},
    'KR': {'name': 'South Korea', 'region': 'Asia'},
    'IL': {'name': 'Israel', 'region': 'Middle East'},
    'IT': {'name': 'Italy', 'region': 'Europe'},
    'ES': {'name': 'Spain', 'region': 'Europe'},
    'NL': {'name': 'Netherlands', 'region': 'Europe'},
    'SE': {'name': 'Sweden', 'region': 'Europe'},
    'PL': {'name': 'Poland', 'region': 'Europe'},
    'RU': {'name': 'Russia', 'region': 'Europe/Asia'},
    'UA': {'name': 'Ukraine', 'region': 'Europe'},
    'TR': {'name': 'Turkey', 'region': 'Europe/Asia'},
    'MX': {'name': 'Mexico', 'region': 'North America'}
}

def verify_ip_country(ip, expected_country, use_multiple_apis=True):
    """Verify IP country using multiple geo APIs for accuracy"""
    
    # Multiple geo APIs for redundancy
    geo_apis = [
        {
            'url': f'http://ip-api.com/json/{ip}?fields=countryCode,country,regionName,city,isp,query',
            'country_field': 'countryCode',
            'name': 'IP-API'
        },
        {
            'url': f'https://ipapi.co/{ip}/json/',
            'country_field': 'country_code',  
            'name': 'IPAPI.co'
        },
        {
            'url': f'https://freegeoip.app/json/{ip}',
            'country_field': 'country_code',
            'name': 'FreeGeoIP'
        }
    ]
    
    results = []
    
    for api in geo_apis:
        try:
            response = requests.get(api['url'], timeout=5)
            if response.status_code == 200:
                data = response.json()
                detected_country = data.get(api['country_field'], '').upper()
                
                result = {
                    'api': api['name'],
                    'detected_country': detected_country,
                    'matches': detected_country == expected_country.upper(),
                    'full_data': data
                }
                results.append(result)
                
                if not use_multiple_apis:
                    break  # Just use first successful API
                    
        except Exception as e:
            results.append({
                'api': api['name'],
                'error': str(e),
                'matches': False
            })
    
    # Determine consensus
    matches = [r['matches'] for r in results if 'matches' in r]
    consensus = sum(matches) > len(matches) / 2 if matches else False
    
    return {
        'consensus_match': consensus,
        'api_results': results,
        'verification_count': len([r for r in results if 'matches' in r])
    }

def test_single_country(api_key, country_code, verify_geo=True):
    """Test targeting for a single country"""
    print(f"\n🌍 Testing country: {country_code} ({COUNTRIES_DB.get(country_code, {}).get('name', 'Unknown')})")
    
    try:
        # Create client with country targeting
        client = IPLoop(api_key=api_key, country=country_code)
        
        # Make request to get IP
        start_time = time.time()
        response = client.get('https://api.ipify.org')
        response_time = (time.time() - start_time) * 1000
        
        if response.status_code != 200:
            return {
                'country': country_code,
                'success': False,
                'error': f'HTTP {response.status_code}',
                'response_time_ms': response_time
            }
        
        ip = response.text.strip()
        print(f"  📍 Got IP: {ip} ({response_time:.0f}ms)")
        
        result = {
            'country': country_code,
            'ip': ip,
            'response_time_ms': response_time,
            'success': True
        }
        
        # Verify geo location if requested
        if verify_geo:
            print(f"  🔍 Verifying geo location...")
            geo_verification = verify_ip_country(ip, country_code)
            result['geo_verification'] = geo_verification
            
            if geo_verification['consensus_match']:
                print(f"  ✅ GEO VERIFIED: IP is from {country_code}")
                result['geo_match'] = True
                result['status'] = 'PASS'
            else:
                print(f"  ❌ GEO MISMATCH: IP not from {country_code}")
                result['geo_match'] = False
                result['status'] = 'FAIL'
                
                # Show details from APIs
                for api_result in geo_verification['api_results']:
                    if 'detected_country' in api_result:
                        print(f"    {api_result['api']}: {api_result['detected_country']}")
        else:
            result['geo_verification'] = None
            result['geo_match'] = None
            result['status'] = 'PASS'  # Assume pass if not verifying
        
        return result
        
    except Exception as e:
        print(f"  ❌ ERROR: {str(e)}")
        return {
            'country': country_code,
            'success': False,
            'error': str(e),
            'response_time_ms': (time.time() - start_time) * 1000 if 'start_time' in locals() else 0
        }

def test_country_availability(api_key, countries):
    """Test which countries are available (can get IPs)"""
    print("\n🗺️  Testing country availability...")
    
    results = []
    available_countries = []
    failed_countries = []
    
    for country in countries:
        try:
            client = IPLoop(api_key=api_key, country=country)
            response = client.get('https://api.ipify.org', timeout=10)
            
            if response.status_code == 200:
                ip = response.text.strip()
                available_countries.append(country)
                results.append({
                    'country': country,
                    'available': True,
                    'ip': ip
                })
                print(f"  ✅ {country}: Available (IP: {ip})")
            else:
                failed_countries.append(country)
                results.append({
                    'country': country, 
                    'available': False,
                    'error': f'HTTP {response.status_code}'
                })
                print(f"  ❌ {country}: Failed (HTTP {response.status_code})")
                
        except Exception as e:
            failed_countries.append(country)
            results.append({
                'country': country,
                'available': False,
                'error': str(e)
            })
            print(f"  ❌ {country}: Error ({str(e)})")
        
        time.sleep(0.5)  # Rate limiting
    
    print(f"\n📊 Availability Summary:")
    print(f"  Available: {len(available_countries)}/{len(countries)} ({len(available_countries)/len(countries)*100:.0f}%)")
    print(f"  Available countries: {', '.join(available_countries)}")
    if failed_countries:
        print(f"  Failed countries: {', '.join(failed_countries)}")
    
    return {
        'total_tested': len(countries),
        'available_count': len(available_countries),
        'failed_count': len(failed_countries),
        'availability_rate': len(available_countries) / len(countries),
        'available_countries': available_countries,
        'failed_countries': failed_countries,
        'detailed_results': results
    }

def test_country_accuracy(api_key, countries, verify_geo=True):
    """Test accuracy of country targeting"""
    print(f"\n🎯 Testing country targeting accuracy...")
    
    results = []
    accurate_countries = []
    inaccurate_countries = []
    
    for country in countries:
        country_result = test_single_country(api_key, country, verify_geo)
        results.append(country_result)
        
        if country_result['success'] and country_result.get('geo_match'):
            accurate_countries.append(country)
        elif country_result['success'] and not verify_geo:
            # If not verifying geo, count as accurate if we got an IP
            accurate_countries.append(country)
        else:
            inaccurate_countries.append(country)
        
        time.sleep(1)  # Rate limiting between countries
    
    print(f"\n🎯 Accuracy Summary:")
    total_successful = len([r for r in results if r['success']])
    if verify_geo:
        accurate_count = len(accurate_countries)
        print(f"  Geo-accurate: {accurate_count}/{total_successful} ({accurate_count/total_successful*100:.0f}%)")
        print(f"  Accurate countries: {', '.join(accurate_countries)}")
        if inaccurate_countries:
            print(f"  Inaccurate countries: {', '.join(inaccurate_countries)}")
    else:
        print(f"  Successfully connected: {total_successful}/{len(countries)}")
    
    return {
        'total_tested': len(countries),
        'successful_connections': total_successful,
        'accurate_count': len(accurate_countries),
        'inaccurate_count': len(inaccurate_countries),
        'accuracy_rate': len(accurate_countries) / total_successful if total_successful > 0 else 0,
        'accurate_countries': accurate_countries,
        'inaccurate_countries': inaccurate_countries,
        'detailed_results': results
    }

def test_country_performance(api_key, countries, requests_per_country=5):
    """Test performance of different countries"""
    print(f"\n⚡ Testing country performance ({requests_per_country} requests per country)...")
    
    performance_results = {}
    
    for country in countries:
        print(f"  Testing {country} performance...")
        client = IPLoop(api_key=api_key, country=country)
        
        timings = []
        errors = 0
        
        for i in range(requests_per_country):
            try:
                start_time = time.time()
                response = client.get('https://httpbin.org/ip')
                timing = (time.time() - start_time) * 1000
                
                if response.status_code == 200:
                    timings.append(timing)
                else:
                    errors += 1
            except:
                errors += 1
            
            time.sleep(0.3)  # Small delay between requests
        
        if timings:
            import statistics
            performance_results[country] = {
                'avg_ms': statistics.mean(timings),
                'min_ms': min(timings),
                'max_ms': max(timings),
                'median_ms': statistics.median(timings),
                'successful_requests': len(timings),
                'failed_requests': errors,
                'success_rate': len(timings) / requests_per_country
            }
            
            print(f"    Avg: {statistics.mean(timings):.0f}ms, Success: {len(timings)}/{requests_per_country}")
        else:
            performance_results[country] = {
                'avg_ms': 0,
                'successful_requests': 0,
                'failed_requests': requests_per_country,
                'success_rate': 0
            }
            print(f"    All requests failed")
        
        time.sleep(1)  # Delay between countries
    
    # Sort by performance
    sorted_countries = sorted(
        [(country, data) for country, data in performance_results.items() if data['avg_ms'] > 0],
        key=lambda x: x[1]['avg_ms']
    )
    
    print(f"\n⚡ Performance Ranking:")
    for i, (country, data) in enumerate(sorted_countries[:10]):  # Top 10
        print(f"  {i+1:2d}. {country}: {data['avg_ms']:.0f}ms avg ({data['success_rate']:.0%} success)")
    
    return performance_results

def main():
    parser = argparse.ArgumentParser(description='IPLoop Country Targeting Tests')
    parser.add_argument('--api-key', default='testkey123', help='IPLoop API key')
    parser.add_argument('--countries', default='US,GB,DE,FR,CA,AU,JP', 
                       help='Comma-separated list of country codes to test')
    parser.add_argument('--verify-geo', action='store_true', default=True,
                       help='Verify geo location using external APIs')
    parser.add_argument('--skip-geo', action='store_true',
                       help='Skip geo verification (faster)')
    parser.add_argument('--test-availability', action='store_true', default=True,
                       help='Test country availability')
    parser.add_argument('--test-performance', action='store_true',
                       help='Test performance for each country')
    parser.add_argument('--output-file', help='Save results to JSON file')
    
    args = parser.parse_args()
    
    if args.skip_geo:
        args.verify_geo = False
    
    countries = [c.strip().upper() for c in args.countries.split(',')]
    
    print("🌍 IPLoop Country Targeting Tests")
    print("=" * 50)
    print(f"API Key: {args.api_key}")
    print(f"Countries: {', '.join(countries)} ({len(countries)} total)")
    print(f"Geo Verification: {'Enabled' if args.verify_geo else 'Disabled'}")
    print(f"Test Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 50)
    
    all_results = {
        'timestamp': datetime.now().isoformat(),
        'api_key': args.api_key,
        'countries_tested': countries,
        'geo_verification_enabled': args.verify_geo,
        'tests': {}
    }
    
    # Test 1: Country availability
    if args.test_availability:
        availability_results = test_country_availability(args.api_key, countries)
        all_results['tests']['availability'] = availability_results
    
    # Test 2: Country accuracy
    accuracy_results = test_country_accuracy(args.api_key, countries, args.verify_geo)
    all_results['tests']['accuracy'] = accuracy_results
    
    # Test 3: Performance (optional)
    if args.test_performance:
        performance_results = test_country_performance(args.api_key, countries)
        all_results['tests']['performance'] = performance_results
    
    # Overall assessment
    print("\n" + "=" * 50)
    print("📊 COUNTRY TARGETING SUMMARY")
    print("=" * 50)
    
    if args.test_availability:
        avail_rate = availability_results['availability_rate']
        print(f"Country Availability: {avail_rate:.0%} ({availability_results['available_count']}/{availability_results['total_tested']})")
    
    acc_rate = accuracy_results['accuracy_rate']
    successful = accuracy_results['successful_connections']
    print(f"Targeting Accuracy: {acc_rate:.0%} ({accuracy_results['accurate_count']}/{successful})")
    
    # Overall grade
    if args.verify_geo:
        if acc_rate >= 0.8 and avail_rate >= 0.8:
            grade = "EXCELLENT"
            emoji = "🎉"
        elif acc_rate >= 0.6 and avail_rate >= 0.6:
            grade = "GOOD"
            emoji = "🟢"
        elif acc_rate >= 0.4 and avail_rate >= 0.4:
            grade = "NEEDS_WORK"
            emoji = "🟡"
        else:
            grade = "POOR"
            emoji = "🔴"
    else:
        if successful >= len(countries) * 0.8:
            grade = "GOOD"
            emoji = "🟢"
        else:
            grade = "NEEDS_WORK"
            emoji = "🟡"
    
    print(f"\n{emoji} Overall Grade: {grade}")
    all_results['overall_grade'] = grade
    
    # Recommendations
    print(f"\n💡 Recommendations:")
    if args.verify_geo and acc_rate < 0.7:
        print(f"  - Improve geo-targeting accuracy - many IPs not from requested countries")
    if args.test_availability and avail_rate < 0.8:
        print(f"  - Add more countries to proxy pool")
    if successful < len(countries) * 0.5:
        print(f"  - Fix basic connectivity issues for country targeting")
    
    # Save results
    if args.output_file:
        with open(args.output_file, 'w') as f:
            json.dump(all_results, f, indent=2)
        print(f"\n💾 Results saved to: {args.output_file}")
    
    return all_results

if __name__ == "__main__":
    main()