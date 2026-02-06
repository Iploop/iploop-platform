#!/usr/bin/env node
/**
 * IPLoop Pilot Stress Test
 * 
 * Simulates real-world proxy usage: price comparison scraping
 * Tests throughput, reliability, IP rotation, and geographic targeting
 */

const http = require('http');
const https = require('https');
const { URL } = require('url');
const fs = require('fs');
const path = require('path');

// Configuration
const CONFIG = {
  // IPLoop proxy endpoint
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key',
  
  // Test profiles
  profiles: {
    smoke: { concurrent: 5, duration: 30, rampTime: 5 },
    ramp: { concurrent: 100, duration: 300, rampTime: 120 },
    sustained: { concurrent: 50, duration: 600, rampTime: 30 },
    burst: { concurrent: 200, duration: 120, rampTime: 10 },
    endurance: { concurrent: 25, duration: 3600, rampTime: 60 }
  },
  
  requestTimeout: 30000,
  
  // Reporting
  reportInterval: 5000
};

// Load targets
const targets = JSON.parse(fs.readFileSync(path.join(__dirname, 'targets.json'), 'utf8'));

// Metrics
const metrics = {
  started: Date.now(),
  totalRequests: 0,
  successfulRequests: 0,
  failedRequests: 0,
  latencies: [],
  errors: {},
  ipsObserved: new Set(),
  geoResults: {},
  sessionTests: { passed: 0, failed: 0 },
  bytesTransferred: 0
};

// Pick random target based on weight
function pickTarget() {
  const totalWeight = targets.endpoints.reduce((sum, t) => sum + (t.weight || 1), 0);
  let random = Math.random() * totalWeight;
  
  for (const target of targets.endpoints) {
    random -= (target.weight || 1);
    if (random <= 0) return target;
  }
  return targets.endpoints[0];
}

// Build proxy auth string with optional parameters
function buildProxyAuth(options = {}) {
  let auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
  
  if (options.country) auth += `-country-${options.country}`;
  if (options.city) auth += `-city-${options.city}`;
  if (options.session) auth += `-session-${options.session}`;
  if (options.sesstype) auth += `-sesstype-${options.sesstype}`;
  
  return auth;
}

// Make request through proxy
function makeProxyRequest(targetUrl, proxyAuth) {
  return new Promise((resolve, reject) => {
    const url = new URL(targetUrl);
    const isHttps = url.protocol === 'https:';
    
    const startTime = Date.now();
    let dataSize = 0;
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: targetUrl,
      headers: {
        'Host': url.host,
        'Proxy-Authorization': 'Basic ' + Buffer.from(proxyAuth).toString('base64'),
        'User-Agent': 'IPLoop-StressTest/1.0'
      },
      timeout: CONFIG.requestTimeout
    };
    
    const req = http.request(options, (res) => {
      let data = '';
      
      res.on('data', (chunk) => {
        data += chunk;
        dataSize += chunk.length;
      });
      
      res.on('end', () => {
        const latency = Date.now() - startTime;
        resolve({
          success: res.statusCode >= 200 && res.statusCode < 400,
          statusCode: res.statusCode,
          latency,
          data,
          dataSize
        });
      });
    });
    
    req.on('error', (err) => {
      reject({ error: err.message, code: err.code });
    });
    
    req.on('timeout', () => {
      req.destroy();
      reject({ error: 'timeout', code: 'ETIMEDOUT' });
    });
    
    req.end();
  });
}

// Run single test request
async function runSingleTest(options = {}) {
  const target = options.target || pickTarget();
  const proxyAuth = buildProxyAuth(options);
  
  metrics.totalRequests++;
  
  try {
    const result = await makeProxyRequest(target.url, proxyAuth);
    
    if (result.success) {
      metrics.successfulRequests++;
      metrics.latencies.push(result.latency);
      metrics.bytesTransferred += result.dataSize;
      
      // Extract IP if available
      if (target.validate === 'origin' || target.name === 'httpbin-ip') {
        try {
          const json = JSON.parse(result.data);
          if (json.origin) {
            metrics.ipsObserved.add(json.origin);
          }
        } catch (e) {}
      }
      
      // Extract geo if available
      if (target.validate === 'geo') {
        try {
          const json = JSON.parse(result.data);
          const country = json.countryCode || json.country;
          metrics.geoResults[country] = (metrics.geoResults[country] || 0) + 1;
        } catch (e) {}
      }
      
      return { success: true, latency: result.latency };
    } else {
      metrics.failedRequests++;
      const errKey = `HTTP_${result.statusCode}`;
      metrics.errors[errKey] = (metrics.errors[errKey] || 0) + 1;
      return { success: false, error: errKey };
    }
  } catch (err) {
    metrics.failedRequests++;
    const errKey = err.code || err.error || 'UNKNOWN';
    metrics.errors[errKey] = (metrics.errors[errKey] || 0) + 1;
    return { success: false, error: errKey };
  }
}

// Run session stickiness test
async function runSessionTest() {
  const flow = targets.sessionFlows[0];
  const sessionId = `sess_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  const ips = [];
  
  for (const url of flow.steps) {
    try {
      const result = await makeProxyRequest(url, buildProxyAuth({ 
        session: sessionId, 
        sesstype: 'sticky' 
      }));
      
      if (result.success && url.includes('/ip')) {
        try {
          const json = JSON.parse(result.data);
          if (json.origin) ips.push(json.origin);
        } catch (e) {}
      }
    } catch (e) {
      metrics.sessionTests.failed++;
      return;
    }
  }
  
  // Check if all IPs are the same
  if (ips.length >= 2 && ips.every(ip => ip === ips[0])) {
    metrics.sessionTests.passed++;
  } else {
    metrics.sessionTests.failed++;
  }
}

// Calculate percentile
function percentile(arr, p) {
  if (arr.length === 0) return 0;
  const sorted = [...arr].sort((a, b) => a - b);
  const idx = Math.ceil((p / 100) * sorted.length) - 1;
  return sorted[Math.max(0, idx)];
}

// Print report
function printReport(final = false) {
  const elapsed = (Date.now() - metrics.started) / 1000;
  const successRate = metrics.totalRequests > 0 
    ? ((metrics.successfulRequests / metrics.totalRequests) * 100).toFixed(2)
    : 0;
  const rps = (metrics.totalRequests / elapsed).toFixed(2);
  
  console.log('\n' + '='.repeat(60));
  console.log(final ? '📊 FINAL REPORT' : '📈 Progress Report');
  console.log('='.repeat(60));
  console.log(`Duration: ${elapsed.toFixed(1)}s`);
  console.log(`Total Requests: ${metrics.totalRequests}`);
  console.log(`Successful: ${metrics.successfulRequests} (${successRate}%)`);
  console.log(`Failed: ${metrics.failedRequests}`);
  console.log(`Throughput: ${rps} req/s`);
  console.log(`Data Transferred: ${(metrics.bytesTransferred / 1024 / 1024).toFixed(2)} MB`);
  
  if (metrics.latencies.length > 0) {
    console.log('\n⏱️  Latency:');
    console.log(`  P50: ${percentile(metrics.latencies, 50)}ms`);
    console.log(`  P95: ${percentile(metrics.latencies, 95)}ms`);
    console.log(`  P99: ${percentile(metrics.latencies, 99)}ms`);
  }
  
  console.log(`\n🌐 Unique IPs Observed: ${metrics.ipsObserved.size}`);
  
  if (Object.keys(metrics.geoResults).length > 0) {
    console.log('\n🗺️  Geographic Distribution:');
    for (const [country, count] of Object.entries(metrics.geoResults).sort((a, b) => b[1] - a[1]).slice(0, 10)) {
      console.log(`  ${country}: ${count}`);
    }
  }
  
  if (Object.keys(metrics.errors).length > 0) {
    console.log('\n❌ Errors:');
    for (const [err, count] of Object.entries(metrics.errors).sort((a, b) => b[1] - a[1])) {
      console.log(`  ${err}: ${count}`);
    }
  }
  
  console.log(`\n🔗 Session Stickiness Tests: ${metrics.sessionTests.passed} passed, ${metrics.sessionTests.failed} failed`);
  
  if (final) {
    // Bug detection summary
    console.log('\n' + '='.repeat(60));
    console.log('🐛 BUG DETECTION SUMMARY');
    console.log('='.repeat(60));
    
    const issues = [];
    
    if (parseFloat(successRate) < 95) {
      issues.push(`⚠️  Success rate below 95%: ${successRate}%`);
    }
    if (percentile(metrics.latencies, 95) > 10000) {
      issues.push(`⚠️  P95 latency > 10s: ${percentile(metrics.latencies, 95)}ms`);
    }
    if (metrics.ipsObserved.size < 3 && metrics.totalRequests > 50) {
      issues.push(`⚠️  Low IP diversity: only ${metrics.ipsObserved.size} unique IPs`);
    }
    if (metrics.sessionTests.failed > metrics.sessionTests.passed) {
      issues.push(`⚠️  Session stickiness failing: ${metrics.sessionTests.failed}/${metrics.sessionTests.passed + metrics.sessionTests.failed}`);
    }
    if (metrics.errors['ETIMEDOUT'] > metrics.totalRequests * 0.1) {
      issues.push(`⚠️  High timeout rate: ${metrics.errors['ETIMEDOUT']} timeouts`);
    }
    if (metrics.errors['ECONNREFUSED']) {
      issues.push(`🔴 Connection refused errors: proxy may be down`);
    }
    
    if (issues.length === 0) {
      console.log('✅ No major issues detected!');
    } else {
      issues.forEach(i => console.log(i));
    }
    
    // Save report to file
    const reportPath = path.join(__dirname, `report_${Date.now()}.json`);
    fs.writeFileSync(reportPath, JSON.stringify({
      ...metrics,
      ipsObserved: [...metrics.ipsObserved],
      elapsed,
      successRate: parseFloat(successRate),
      rps: parseFloat(rps),
      latencyP50: percentile(metrics.latencies, 50),
      latencyP95: percentile(metrics.latencies, 95),
      latencyP99: percentile(metrics.latencies, 99)
    }, null, 2));
    console.log(`\n📁 Full report saved to: ${reportPath}`);
  }
}

// Main test runner
async function runStressTest(profileName) {
  const profile = CONFIG.profiles[profileName];
  if (!profile) {
    console.error(`Unknown profile: ${profileName}`);
    console.log('Available profiles:', Object.keys(CONFIG.profiles).join(', '));
    process.exit(1);
  }
  
  console.log('🚀 IPLoop Pilot Stress Test');
  console.log('='.repeat(60));
  console.log(`Profile: ${profileName}`);
  console.log(`Target concurrency: ${profile.concurrent}`);
  console.log(`Duration: ${profile.duration}s`);
  console.log(`Ramp time: ${profile.rampTime}s`);
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log('='.repeat(60));
  
  const endTime = Date.now() + (profile.duration * 1000);
  const rampEndTime = Date.now() + (profile.rampTime * 1000);
  let activeWorkers = 0;
  
  // Report timer
  const reportTimer = setInterval(() => printReport(), CONFIG.reportInterval);
  
  // Worker function
  async function worker(workerId) {
    while (Date.now() < endTime) {
      await runSingleTest();
      
      // Occasionally run session test
      if (Math.random() < 0.05) {
        await runSessionTest();
      }
      
      // Small delay between requests
      await new Promise(r => setTimeout(r, 100 + Math.random() * 200));
    }
    activeWorkers--;
  }
  
  // Ramp up workers
  const rampInterval = setInterval(() => {
    const elapsed = Date.now() - metrics.started;
    const rampProgress = Math.min(1, elapsed / (profile.rampTime * 1000));
    const targetWorkers = Math.floor(profile.concurrent * rampProgress);
    
    while (activeWorkers < targetWorkers && Date.now() < endTime) {
      activeWorkers++;
      worker(activeWorkers);
    }
    
    if (Date.now() > rampEndTime) {
      clearInterval(rampInterval);
    }
  }, 100);
  
  // Wait for test to complete
  await new Promise(r => setTimeout(r, profile.duration * 1000 + 1000));
  
  clearInterval(reportTimer);
  clearInterval(rampInterval);
  
  printReport(true);
}

// CLI
const args = process.argv.slice(2);
let profile = 'smoke';

for (let i = 0; i < args.length; i++) {
  if (args[i].startsWith('--profile=')) {
    profile = args[i].split('=')[1];
  } else if (args[i] === '--profile' && args[i + 1]) {
    profile = args[++i];
  } else if (args[i] === '--help') {
    console.log('Usage: node stress-test.js [options]');
    console.log('\nOptions:');
    console.log('  --profile=NAME    Test profile (smoke, ramp, sustained, burst, endurance)');
    console.log('\nEnvironment:');
    console.log('  PROXY_HOST        Proxy hostname (default: localhost)');
    console.log('  PROXY_PORT        Proxy port (default: 8080)');
    console.log('  PROXY_USER        Proxy username');
    console.log('  PROXY_PASS        Proxy password');
    process.exit(0);
  }
}

runStressTest(profile).catch(console.error);
