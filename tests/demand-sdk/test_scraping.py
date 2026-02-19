#!/usr/bin/env python3
"""
IPLoop Python SDK - Web Scraping Use Cases Test
Tests real-world scraping scenarios: e-commerce, travel, social media, etc.

Usage:
    python test_scraping.py [--api-key KEY] [--categories CATS] [--timeout SEC]

Example:
    python test_scraping.py --api-key testkey123 --categories ecommerce,travel,social --timeout 15
"""

import argparse
import json
import time
from datetime import datetime
from iploop import IPLoop

# Real-world scraping targets organized by use case
SCRAPING_TARGETS = {
    'ecommerce': {
        'name': 'E-Commerce & Price Monitoring',
        'sites': [
            ('Amazon Product', 'https://www.amazon.com/dp/B0D1XD1ZV3', ['product', 'price', 'buy']),
            ('eBay Search', 'https://www.ebay.com/sch/i.html?_nkw=iphone+15', ['iphone', 'search', 'results']),
            ('Walmart Search', 'https://www.walmart.com/search?q=laptop', ['laptop', 'search', 'results']),
            ('Best Buy Search', 'https://www.bestbuy.com/site/searchpage.jsp?st=gpu', ['gpu', 'search']),
            ('Target Search', 'https://www.target.com/s?searchTerm=headphones', ['headphones', 'search']),
            ('AliExpress', 'https://www.aliexpress.com/wholesale?SearchText=earbuds', ['earbuds', 'search']),
            ('Etsy Search', 'https://www.etsy.com/search?q=handmade+jewelry', ['jewelry', 'handmade'])
        ]
    },
    'travel': {
        'name': 'Travel & Flights',
        'sites': [
            ('Google Flights', 'https://www.google.com/travel/flights', ['flights', 'travel']),
            ('Booking.com London', 'https://www.booking.com/searchresults.html?ss=London', ['london', 'hotel', 'booking']),
            ('Expedia Flights', 'https://www.expedia.com/Flights', ['flights', 'expedia']),
            ('Kayak', 'https://www.kayak.com/flights', ['flights', 'kayak']),
            ('Skyscanner', 'https://www.skyscanner.com/transport/flights/nyca/lond/', ['flights', 'search']),
            ('TripAdvisor Hotels', 'https://www.tripadvisor.com/Hotels-g186338-London_England-Hotels.html', ['hotel', 'london']),
            ('Airbnb Paris', 'https://www.airbnb.com/s/Paris/homes', ['paris', 'homes'])
        ]
    },
    'tickets': {
        'name': 'Tickets & Events',
        'sites': [
            ('Ticketmaster', 'https://www.ticketmaster.com/search?q=concert', ['concert', 'tickets']),
            ('StubHub', 'https://www.stubhub.com/', ['stubhub', 'tickets']),
            ('Eventbrite', 'https://www.eventbrite.com/d/online/events/', ['events', 'eventbrite']),
            ('Vivid Seats', 'https://www.vividseats.com/', ['tickets', 'vivid'])
        ]
    },
    'ads': {
        'name': 'Ad Verification',
        'sites': [
            ('Google Ads Search', 'https://www.google.com/search?q=buy+shoes+online', ['shoes', 'search', 'results']),
            ('Bing Ads Search', 'https://www.bing.com/search?q=best+vpn+2024', ['vpn', 'search']),
            ('Facebook Login', 'https://www.facebook.com', ['facebook', 'login'])
        ]
    },
    'seo': {
        'name': 'SEO & SERP Monitoring', 
        'sites': [
            ('Google SERP - Proxy', 'https://www.google.com/search?q=best+proxy+service', ['proxy', 'search', 'results']),
            ('Google SERP - API', 'https://www.google.com/search?q=residential+proxy+api', ['proxy', 'api']),
            ('Google DE', 'https://www.google.de/search?q=proxy+service', ['proxy', 'search']),
            ('Google UK', 'https://www.google.co.uk/search?q=proxy+service', ['proxy', 'search']),
            ('Bing SERP', 'https://www.bing.com/search?q=proxy+api', ['proxy', 'api']),
            ('DuckDuckGo', 'https://duckduckgo.com/?q=proxy+service', ['proxy', 'search'])
        ]
    },
    'social': {
        'name': 'Social Media',
        'sites': [
            ('Instagram Explore', 'https://www.instagram.com/explore/', ['explore', 'instagram']),
            ('TikTok Explore', 'https://www.tiktok.com/explore', ['tiktok', 'explore']),
            ('Pinterest Search', 'https://www.pinterest.com/search/pins/?q=design', ['design', 'pinterest']),
            ('Snapchat', 'https://www.snapchat.com/', ['snapchat']),
            ('Threads', 'https://www.threads.net/', ['threads'])
        ]
    },
    'realestate': {
        'name': 'Real Estate',
        'sites': [
            ('Zillow', 'https://www.zillow.com/homes/for_sale/', ['homes', 'sale', 'zillow']),
            ('Realtor.com', 'https://www.realtor.com/realestateandhomes-search/New-York', ['new york', 'homes']),
            ('Redfin', 'https://www.redfin.com/city/30749/NY/New-York', ['new york', 'redfin'])
        ]
    },
    'finance': {
        'name': 'Financial Data',
        'sites': [
            ('Yahoo Finance AAPL', 'https://finance.yahoo.com/quote/AAPL', ['aapl', 'stock', 'price']),
            ('NASDAQ AAPL', 'https://www.nasdaq.com/market-activity/stocks/aapl', ['aapl', 'nasdaq']),
            ('Bloomberg Markets', 'https://www.bloomberg.com/markets', ['markets', 'bloomberg'])
        ]
    }
}

def analyze_response(response, expected_keywords=None):
    """Analyze response to determine success, blocks, etc."""
    
    if not response:
        return 'FAIL', 'No response received'
    
    status_code = response.status_code
    content = response.text.lower() if hasattr(response, 'text') else ""
    
    # Check for blocked responses
    block_indicators = [
        'captcha', 'blocked', 'access denied', 'forbidden',
        'bot detection', 'cloudflare', 'security check',
        'ray id:', 'ddos protection', 'challenge', 'verification',
        'unusual traffic', 'automated requests', 'robot'
    ]
    
    is_blocked = any(indicator in content for indicator in block_indicators)
    
    # Determine status
    if status_code == 200:
        if is_blocked:
            return 'BLOCKED', 'CAPTCHA/Bot detection'
        elif expected_keywords and not any(keyword in content for keyword in expected_keywords):
            return 'PARTIAL', 'Content not loaded properly'
        elif len(content) < 1000:
            return 'PARTIAL', 'Minimal content returned'
        else:
            return 'PASS', f'Content loaded ({len(content)} bytes)'
    elif status_code == 403:
        return 'BLOCKED', 'HTTP 403 Forbidden'
    elif status_code == 429:
        return 'BLOCKED', 'Rate limited (HTTP 429)'
    elif status_code in [301, 302, 303, 307, 308]:
        location = response.headers.get('Location', 'Unknown')
        return 'REDIRECT', f'Redirected to: {location[:50]}'
    elif status_code >= 400:
        return 'FAIL', f'HTTP {status_code}'
    else:
        return 'PARTIAL', f'Unexpected status: {status_code}'

def test_site(client, site_name, url, expected_keywords, timeout=15):
    """Test a single site with comprehensive analysis"""
    
    try:
        start_time = time.time()
        response = client.get(url, timeout=timeout)
        response_time = (time.time() - start_time) * 1000
        
        status, details = analyze_response(response, expected_keywords)
        
        return {
            'site_name': site_name,
            'url': url,
            'status': status,
            'details': details,
            'status_code': response.status_code,
            'response_time_ms': response_time,
            'content_length': len(response.text) if hasattr(response, 'text') else 0,
            'redirects': len(response.history) if hasattr(response, 'history') else 0,
            'success': status == 'PASS'
        }
        
    except Exception as e:
        response_time = (time.time() - start_time) * 1000 if 'start_time' in locals() else 0
        
        error_str = str(e).lower()
        if 'timeout' in error_str:
            status, details = 'TIMEOUT', f'Request timeout ({timeout}s)'
        else:
            status, details = 'FAIL', str(e)
        
        return {
            'site_name': site_name,
            'url': url,
            'status': status,
            'details': details,
            'status_code': None,
            'response_time_ms': response_time,
            'content_length': 0,
            'redirects': 0,
            'success': False
        }

def test_category(api_key, category_key, timeout=15, delay_between_requests=1):
    """Test all sites in a category"""
    
    if category_key not in SCRAPING_TARGETS:
        raise ValueError(f"Unknown category: {category_key}")
    
    category = SCRAPING_TARGETS[category_key]
    print(f"\n🎯 Testing: {category['name']} ({len(category['sites'])} sites)")
    
    client = IPLoop(api_key=api_key)
    results = []
    
    for site_name, url, keywords in category['sites']:
        print(f"  Testing: {site_name}...")
        
        result = test_site(client, site_name, url, keywords, timeout)
        results.append(result)
        
        # Log result
        emoji = {
            'PASS': '✅',
            'FAIL': '❌', 
            'BLOCKED': '🚫',
            'PARTIAL': '🔶',
            'TIMEOUT': '⏰',
            'REDIRECT': '🔄'
        }.get(result['status'], '❓')
        
        print(f"    {emoji} {result['status']} ({result['response_time_ms']:.0f}ms) - {result['details']}")
        
        # Rate limiting
        if delay_between_requests > 0:
            time.sleep(delay_between_requests)
    
    # Category summary
    total = len(results)
    passed = len([r for r in results if r['status'] == 'PASS'])
    blocked = len([r for r in results if r['status'] == 'BLOCKED'])
    failed = len([r for r in results if r['status'] == 'FAIL'])
    
    avg_time = sum(r['response_time_ms'] for r in results) / len(results) if results else 0
    
    print(f"\n  📊 {category['name']} Summary:")
    print(f"    ✅ Passed: {passed}/{total} ({passed/total*100:.0f}%)")
    print(f"    🚫 Blocked: {blocked}/{total} ({blocked/total*100:.0f}%)")
    print(f"    ❌ Failed: {failed}/{total} ({failed/total*100:.0f}%)")
    print(f"    ⏱️ Avg time: {avg_time:.0f}ms")
    
    return {
        'category': category_key,
        'category_name': category['name'],
        'total_sites': total,
        'passed': passed,
        'blocked': blocked,
        'failed': failed,
        'success_rate': passed / total if total > 0 else 0,
        'block_rate': blocked / total if total > 0 else 0,
        'avg_response_time_ms': avg_time,
        'results': results
    }

def assess_business_viability(category_results):
    """Assess business viability for each use case"""
    
    viability_assessment = {}
    
    for category_key, result in category_results.items():
        success_rate = result['success_rate']
        block_rate = result['block_rate']
        
        # Determine viability based on success rate and block rate
        if success_rate >= 0.8:
            viability = 'EXCELLENT'
            status = '🟢'
            recommendation = 'Ready for production use'
        elif success_rate >= 0.6:
            viability = 'GOOD'
            status = '🟡'
            recommendation = 'Usable with minor improvements needed'
        elif success_rate >= 0.3:
            viability = 'LIMITED'
            status = '🟠'
            recommendation = 'Significant improvements required'
        else:
            viability = 'POOR'
            status = '🔴'
            recommendation = 'Not viable in current state'
        
        # Special considerations for high block rates
        if block_rate >= 0.5:
            if viability in ['EXCELLENT', 'GOOD']:
                viability = 'LIMITED'
                status = '🟠'
            recommendation += ' - High block rate detected'
        
        viability_assessment[category_key] = {
            'viability': viability,
            'status_emoji': status,
            'success_rate': success_rate,
            'block_rate': block_rate,
            'recommendation': recommendation,
            'category_name': result['category_name']
        }
    
    return viability_assessment

def main():
    parser = argparse.ArgumentParser(description='IPLoop Web Scraping Use Cases Test')
    parser.add_argument('--api-key', default='testkey123', help='IPLoop API key')
    parser.add_argument('--categories', 
                       default='ecommerce,travel,tickets,ads,seo,social,realestate,finance',
                       help='Comma-separated categories to test')
    parser.add_argument('--timeout', type=int, default=15, help='Request timeout in seconds')
    parser.add_argument('--delay', type=float, default=1.0, help='Delay between requests in seconds')
    parser.add_argument('--country', help='Target country (optional)')
    parser.add_argument('--output-file', help='Save results to JSON file')
    
    args = parser.parse_args()
    
    categories_to_test = [c.strip() for c in args.categories.split(',')]
    
    # Validate categories
    invalid_categories = [c for c in categories_to_test if c not in SCRAPING_TARGETS]
    if invalid_categories:
        print(f"❌ Invalid categories: {invalid_categories}")
        print(f"Available categories: {', '.join(SCRAPING_TARGETS.keys())}")
        return
    
    print("🌐 IPLoop Web Scraping Use Cases Test")
    print("=" * 60)
    print(f"API Key: {args.api_key}")
    print(f"Categories: {', '.join(categories_to_test)}")
    print(f"Timeout: {args.timeout}s")
    if args.country:
        print(f"Target Country: {args.country}")
    print(f"Test Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 60)
    
    all_results = {
        'timestamp': datetime.now().isoformat(),
        'api_key': args.api_key,
        'test_config': {
            'categories': categories_to_test,
            'timeout': args.timeout,
            'delay': args.delay,
            'country': args.country
        },
        'category_results': {},
        'overall_stats': {}
    }
    
    # Test each category
    category_results = {}
    
    for category_key in categories_to_test:
        try:
            result = test_category(args.api_key, category_key, args.timeout, args.delay)
            category_results[category_key] = result
            all_results['category_results'][category_key] = result
            
        except KeyboardInterrupt:
            print(f"\n⚠️ Testing interrupted by user")
            break
        except Exception as e:
            print(f"\n❌ Error testing category {category_key}: {e}")
            continue
    
    # Calculate overall statistics
    if category_results:
        total_sites = sum(r['total_sites'] for r in category_results.values())
        total_passed = sum(r['passed'] for r in category_results.values())
        total_blocked = sum(r['blocked'] for r in category_results.values())
        total_failed = sum(r['failed'] for r in category_results.values())
        
        overall_success_rate = total_passed / total_sites if total_sites > 0 else 0
        overall_block_rate = total_blocked / total_sites if total_sites > 0 else 0
        
        all_results['overall_stats'] = {
            'total_sites': total_sites,
            'total_passed': total_passed,
            'total_blocked': total_blocked,
            'total_failed': total_failed,
            'overall_success_rate': overall_success_rate,
            'overall_block_rate': overall_block_rate
        }
        
        # Business viability assessment
        viability = assess_business_viability(category_results)
        all_results['viability_assessment'] = viability
        
        # Print overall summary
        print("\n" + "=" * 60)
        print("📊 OVERALL WEB SCRAPING SUMMARY")
        print("=" * 60)
        
        print(f"\n🔢 Overall Statistics:")
        print(f"  Total sites tested: {total_sites}")
        print(f"  ✅ Passed: {total_passed} ({overall_success_rate:.1%})")
        print(f"  🚫 Blocked: {total_blocked} ({overall_block_rate:.1%})")
        print(f"  ❌ Failed: {total_failed} ({(total_failed/total_sites):.1%})")
        
        print(f"\n💼 Business Viability by Use Case:")
        for category_key, assessment in viability.items():
            print(f"  {assessment['status_emoji']} {assessment['category_name']}: {assessment['viability']}")
            print(f"     Success: {assessment['success_rate']:.1%}, Blocks: {assessment['block_rate']:.1%}")
            print(f"     → {assessment['recommendation']}")
        
        # Overall grade
        if overall_success_rate >= 0.7:
            grade = "PRODUCTION READY"
            grade_emoji = "🎉"
        elif overall_success_rate >= 0.5:
            grade = "NEEDS IMPROVEMENTS"
            grade_emoji = "🟡"
        elif overall_success_rate >= 0.3:
            grade = "SIGNIFICANT ISSUES"
            grade_emoji = "🟠"
        else:
            grade = "NOT VIABLE"
            grade_emoji = "🔴"
        
        print(f"\n{grade_emoji} Overall Assessment: {grade}")
        print(f"   Success Rate: {overall_success_rate:.1%}")
        print(f"   Block Rate: {overall_block_rate:.1%}")
        
        all_results['overall_grade'] = grade
        
        # Recommendations
        print(f"\n💡 Key Recommendations:")
        if overall_block_rate > 0.3:
            print(f"  🚨 URGENT: High block rate ({overall_block_rate:.1%}) - implement anti-detection")
        if overall_success_rate < 0.5:
            print(f"  🚨 URGENT: Low success rate - fundamental proxy issues")
        
        blocked_categories = [k for k, v in viability.items() if v['block_rate'] > 0.5]
        if blocked_categories:
            print(f"  🎯 Focus on these high-block categories: {', '.join(blocked_categories)}")
    
    # Save results
    if args.output_file:
        with open(args.output_file, 'w') as f:
            json.dump(all_results, f, indent=2)
        print(f"\n💾 Results saved to: {args.output_file}")
    
    return all_results

if __name__ == "__main__":
    main()