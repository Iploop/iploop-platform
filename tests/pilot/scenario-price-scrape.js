#!/usr/bin/env node
/**
 * IPLoop Real-World Scenario: Price Comparison Scraper
 * 
 * Simulates a realistic e-commerce scraping workflow:
 * 1. Search for products
 * 2. Browse listing pages
 * 3. Visit product detail pages
 * 4. Extract pricing data
 * 
 * Uses session stickiness for multi-page flows (like a real browser)
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key',
  
  concurrentScrapers: parseInt(process.env.SCRAPERS || '10'),
  pagesPerSession: parseInt(process.env.PAGES || '5')
};

// Simulated e-commerce flow using httpbin as proxy target
const SCRAPE_FLOW = [
  { name: 'Homepage', url: 'https://httpbin.org/html', delay: 500 },
  { name: 'Search', url: 'https://httpbin.org/get?q=laptop', delay: 1000 },
  { name: 'Listing Page 1', url: 'https://httpbin.org/get?page=1', delay: 800 },
  { name: 'Product Detail', url: 'https://httpbin.org/get?product=12345', delay: 1500 },
  { name: 'Price API', url: 'https://httpbin.org/json', delay: 300 }
];

const metrics = {
  scrapersStarted: 0,
  scrapersCompleted: 0,
  scrapersFailed: 0,
  totalRequests: 0,
  successfulRequests: 0,
  failedRequests: 0,
  totalLatency: 0,
  sessionSticky: 0,
  sessionBroken: 0
};

function makeRequest(url, sessionId) {
  return new Promise((resolve, reject) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}-session-${sessionId}-sesstype-sticky`;
    const startTime = Date.now();
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: url,
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64'),
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0',
        'Accept': 'text/html,application/json',
        'Accept-Language': 'en-US,en;q=0.9'
      },
      timeout: 30000
    };
    
    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        resolve({
          success: res.statusCode >= 200 && res.statusCode < 400,
          statusCode: res.statusCode,
          latency: Date.now() - startTime,
          size: data.length
        });
      });
    });
    
    req.on('error', (err) => reject(err));
    req.on('timeout', () => { req.destroy(); reject(new Error('Timeout')); });
    req.end();
  });
}

function getIP(sessionId) {
  return new Promise((resolve, reject) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}-session-${sessionId}-sesstype-sticky`;
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: 'https://httpbin.org/ip',
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout: 10000
    };
    
    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          resolve(JSON.parse(data).origin);
        } catch {
          resolve(null);
        }
      });
    });
    
    req.on('error', () => resolve(null));
    req.on('timeout', () => { req.destroy(); resolve(null); });
    req.end();
  });
}

async function runScrapeSession(scraperId) {
  const sessionId = `scraper_${Date.now()}_${scraperId}`;
  metrics.scrapersStarted++;
  
  // Get initial IP
  const startIp = await getIP(sessionId);
  let sessionOk = true;
  
  for (const step of SCRAPE_FLOW) {
    metrics.totalRequests++;
    
    try {
      const result = await makeRequest(step.url, sessionId);
      
      if (result.success) {
        metrics.successfulRequests++;
        metrics.totalLatency += result.latency;
      } else {
        metrics.failedRequests++;
        sessionOk = false;
      }
    } catch (err) {
      metrics.failedRequests++;
      sessionOk = false;
    }
    
    // Simulate user think time
    await new Promise(r => setTimeout(r, step.delay + Math.random() * 500));
  }
  
  // Check if IP stayed sticky
  const endIp = await getIP(sessionId);
  if (startIp && endIp && startIp === endIp) {
    metrics.sessionSticky++;
  } else if (startIp && endIp) {
    metrics.sessionBroken++;
  }
  
  if (sessionOk) {
    metrics.scrapersCompleted++;
  } else {
    metrics.scrapersFailed++;
  }
  
  return { scraperId, sessionOk, startIp, endIp };
}

async function runScenarioTest() {
  console.log('🛒 IPLoop Price Comparison Scraper Scenario');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log(`Concurrent scrapers: ${CONFIG.concurrentScrapers}`);
  console.log(`Pages per session: ${SCRAPE_FLOW.length}`);
  console.log('='.repeat(60));
  console.log('');
  console.log('Simulating e-commerce scraping workflow:');
  SCRAPE_FLOW.forEach((step, i) => console.log(`  ${i + 1}. ${step.name}`));
  console.log('');
  
  const startTime = Date.now();
  
  // Progress reporter
  const progressInterval = setInterval(() => {
    const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
    const rps = (metrics.totalRequests / parseFloat(elapsed)).toFixed(1);
    console.log(`[${elapsed}s] Scrapers: ${metrics.scrapersCompleted}/${CONFIG.concurrentScrapers} | Requests: ${metrics.totalRequests} (${rps}/s) | Success: ${metrics.successfulRequests}`);
  }, 3000);
  
  // Run scrapers concurrently
  const scraperPromises = [];
  for (let i = 0; i < CONFIG.concurrentScrapers; i++) {
    // Stagger starts
    await new Promise(r => setTimeout(r, 100));
    scraperPromises.push(runScrapeSession(i));
  }
  
  const results = await Promise.all(scraperPromises);
  clearInterval(progressInterval);
  
  const elapsed = (Date.now() - startTime) / 1000;
  
  // Report
  console.log('\n' + '='.repeat(60));
  console.log('📊 SCENARIO RESULTS');
  console.log('='.repeat(60));
  console.log(`Duration: ${elapsed.toFixed(1)}s`);
  console.log(`Throughput: ${(metrics.totalRequests / elapsed).toFixed(1)} req/s`);
  console.log('');
  
  console.log('Scraper Sessions:');
  console.log(`  ✅ Completed: ${metrics.scrapersCompleted}`);
  console.log(`  ❌ Failed: ${metrics.scrapersFailed}`);
  console.log('');
  
  console.log('Requests:');
  console.log(`  Total: ${metrics.totalRequests}`);
  console.log(`  Successful: ${metrics.successfulRequests} (${(metrics.successfulRequests/metrics.totalRequests*100).toFixed(1)}%)`);
  console.log(`  Failed: ${metrics.failedRequests}`);
  console.log(`  Avg latency: ${(metrics.totalLatency / metrics.successfulRequests).toFixed(0)}ms`);
  console.log('');
  
  console.log('Session Stickiness:');
  console.log(`  ✅ Maintained: ${metrics.sessionSticky}`);
  console.log(`  ❌ Broken: ${metrics.sessionBroken}`);
  
  // Verdict
  console.log('\n' + '='.repeat(60));
  const successRate = metrics.successfulRequests / metrics.totalRequests;
  const stickyRate = metrics.sessionSticky / (metrics.sessionSticky + metrics.sessionBroken);
  
  if (successRate >= 0.95 && stickyRate >= 0.95) {
    console.log('✅ EXCELLENT: Ready for production scraping workloads!');
  } else if (successRate >= 0.90) {
    console.log('⚠️  GOOD: Minor issues, but usable for most workloads');
  } else {
    console.log('❌ NEEDS WORK: Reliability issues for production use');
  }
}

runScenarioTest().catch(console.error);
