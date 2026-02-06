#!/usr/bin/env node
/**
 * Long-Running Stability Test
 * 
 * Runs for extended period monitoring:
 * - Success rate over time
 * - Latency degradation
 * - Memory leaks (if monitoring available)
 * - Connection pool behavior
 * - Error rate trends
 */

const http = require('http');
const { execSync } = require('child_process');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key',
  
  durationMinutes: parseInt(process.env.DURATION || '10'),
  requestsPerSecond: parseInt(process.env.RPS || '5'),
  reportIntervalSeconds: 30
};

const metrics = {
  windows: [], // Time windows for trend analysis
  currentWindow: {
    start: Date.now(),
    requests: 0,
    success: 0,
    errors: {},
    latencies: [],
    ips: new Set()
  },
  overall: {
    requests: 0,
    success: 0,
    errors: {}
  }
};

function makeRequest() {
  return new Promise((resolve) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
    const start = Date.now();
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: 'https://httpbin.org/ip',
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout: 30000
    };
    
    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        const latency = Date.now() - start;
        let ip = null;
        try {
          ip = JSON.parse(data).origin;
        } catch {}
        
        resolve({ success: res.statusCode === 200, latency, ip });
      });
    });
    
    req.on('error', (err) => resolve({ success: false, error: err.code }));
    req.on('timeout', () => { req.destroy(); resolve({ success: false, error: 'TIMEOUT' }); });
    req.end();
  });
}

function percentile(arr, p) {
  if (arr.length === 0) return 0;
  const sorted = [...arr].sort((a, b) => a - b);
  const idx = Math.ceil((p / 100) * sorted.length) - 1;
  return sorted[Math.max(0, idx)];
}

function rotateWindow() {
  const window = metrics.currentWindow;
  const successRate = window.requests > 0 ? (window.success / window.requests * 100).toFixed(1) : 0;
  
  metrics.windows.push({
    timestamp: window.start,
    duration: Date.now() - window.start,
    requests: window.requests,
    successRate: parseFloat(successRate),
    p50: percentile(window.latencies, 50),
    p95: percentile(window.latencies, 95),
    p99: percentile(window.latencies, 99),
    uniqueIps: window.ips.size,
    errors: { ...window.errors }
  });
  
  // Reset current window
  metrics.currentWindow = {
    start: Date.now(),
    requests: 0,
    success: 0,
    errors: {},
    latencies: [],
    ips: new Set()
  };
}

function printReport() {
  const window = metrics.windows[metrics.windows.length - 1];
  if (!window) return;
  
  const elapsed = Math.floor((Date.now() - metrics.windows[0].timestamp) / 1000 / 60);
  
  console.log(`\n[${elapsed}m] Window Report:`);
  console.log(`  Requests: ${window.requests} | Success: ${window.successRate}%`);
  console.log(`  Latency P50/P95/P99: ${window.p50}/${window.p95}/${window.p99}ms`);
  console.log(`  Unique IPs: ${window.uniqueIps}`);
  
  if (Object.keys(window.errors).length > 0) {
    console.log(`  Errors: ${JSON.stringify(window.errors)}`);
  }
  
  // Trend analysis
  if (metrics.windows.length >= 3) {
    const recent = metrics.windows.slice(-3);
    const avgSuccess = recent.reduce((a, w) => a + w.successRate, 0) / 3;
    const avgP95 = recent.reduce((a, w) => a + w.p95, 0) / 3;
    
    const older = metrics.windows.slice(0, 3);
    const oldAvgSuccess = older.reduce((a, w) => a + w.successRate, 0) / 3;
    const oldAvgP95 = older.reduce((a, w) => a + w.p95, 0) / 3;
    
    if (avgSuccess < oldAvgSuccess - 5) {
      console.log(`  ⚠️  Success rate declining: ${oldAvgSuccess.toFixed(1)}% → ${avgSuccess.toFixed(1)}%`);
    }
    if (avgP95 > oldAvgP95 * 1.5) {
      console.log(`  ⚠️  Latency increasing: ${oldAvgP95.toFixed(0)}ms → ${avgP95.toFixed(0)}ms`);
    }
  }
}

function printFinalReport() {
  console.log('\n' + '='.repeat(70));
  console.log('📊 STABILITY TEST FINAL REPORT');
  console.log('='.repeat(70));
  
  const totalDuration = (Date.now() - metrics.windows[0].timestamp) / 1000 / 60;
  const totalRequests = metrics.overall.requests;
  const totalSuccess = metrics.overall.success;
  const overallSuccessRate = (totalSuccess / totalRequests * 100).toFixed(2);
  
  console.log(`\nDuration: ${totalDuration.toFixed(1)} minutes`);
  console.log(`Total requests: ${totalRequests}`);
  console.log(`Overall success rate: ${overallSuccessRate}%`);
  console.log(`Average RPS: ${(totalRequests / (totalDuration * 60)).toFixed(2)}`);
  
  // Aggregate latencies
  const allP50 = metrics.windows.map(w => w.p50);
  const allP95 = metrics.windows.map(w => w.p95);
  const allP99 = metrics.windows.map(w => w.p99);
  
  console.log(`\nLatency (across windows):`);
  console.log(`  P50 range: ${Math.min(...allP50)} - ${Math.max(...allP50)}ms`);
  console.log(`  P95 range: ${Math.min(...allP95)} - ${Math.max(...allP95)}ms`);
  console.log(`  P99 range: ${Math.min(...allP99)} - ${Math.max(...allP99)}ms`);
  
  // Success rate trend
  const successRates = metrics.windows.map(w => w.successRate);
  const minSuccess = Math.min(...successRates);
  const maxSuccess = Math.max(...successRates);
  
  console.log(`\nSuccess rate range: ${minSuccess}% - ${maxSuccess}%`);
  
  // Stability assessment
  console.log('\n' + '='.repeat(70));
  console.log('STABILITY ASSESSMENT');
  console.log('='.repeat(70));
  
  const issues = [];
  
  if (parseFloat(overallSuccessRate) < 95) {
    issues.push(`Overall success rate below 95%: ${overallSuccessRate}%`);
  }
  
  if (maxSuccess - minSuccess > 10) {
    issues.push(`High success rate variance: ${minSuccess}% to ${maxSuccess}%`);
  }
  
  // Check for degradation over time
  if (metrics.windows.length >= 6) {
    const firstHalf = metrics.windows.slice(0, Math.floor(metrics.windows.length / 2));
    const secondHalf = metrics.windows.slice(Math.floor(metrics.windows.length / 2));
    
    const firstAvg = firstHalf.reduce((a, w) => a + w.successRate, 0) / firstHalf.length;
    const secondAvg = secondHalf.reduce((a, w) => a + w.successRate, 0) / secondHalf.length;
    
    if (secondAvg < firstAvg - 5) {
      issues.push(`Performance degradation detected: ${firstAvg.toFixed(1)}% → ${secondAvg.toFixed(1)}%`);
    }
    
    const firstP95 = firstHalf.reduce((a, w) => a + w.p95, 0) / firstHalf.length;
    const secondP95 = secondHalf.reduce((a, w) => a + w.p95, 0) / secondHalf.length;
    
    if (secondP95 > firstP95 * 1.5) {
      issues.push(`Latency degradation: P95 ${firstP95.toFixed(0)}ms → ${secondP95.toFixed(0)}ms`);
    }
  }
  
  if (Object.keys(metrics.overall.errors).length > 0) {
    const errorCount = Object.values(metrics.overall.errors).reduce((a, b) => a + b, 0);
    if (errorCount > totalRequests * 0.05) {
      issues.push(`High error count: ${errorCount} errors`);
    }
    console.log(`\nErrors: ${JSON.stringify(metrics.overall.errors)}`);
  }
  
  if (issues.length === 0) {
    console.log('✅ STABLE: No significant issues detected');
  } else {
    console.log('⚠️  ISSUES DETECTED:');
    issues.forEach(i => console.log(`  - ${i}`));
  }
}

async function runStabilityTest() {
  console.log('⏱️  Long-Running Stability Test');
  console.log('='.repeat(70));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log(`Duration: ${CONFIG.durationMinutes} minutes`);
  console.log(`Target RPS: ${CONFIG.requestsPerSecond}`);
  console.log(`Report interval: ${CONFIG.reportIntervalSeconds}s`);
  console.log('='.repeat(70));
  console.log('\nStarting test...');
  
  const endTime = Date.now() + (CONFIG.durationMinutes * 60 * 1000);
  const requestInterval = 1000 / CONFIG.requestsPerSecond;
  
  // Report timer
  const reportTimer = setInterval(() => {
    rotateWindow();
    printReport();
  }, CONFIG.reportIntervalSeconds * 1000);
  
  // Main request loop
  while (Date.now() < endTime) {
    const result = await makeRequest();
    
    metrics.currentWindow.requests++;
    metrics.overall.requests++;
    
    if (result.success) {
      metrics.currentWindow.success++;
      metrics.overall.success++;
      metrics.currentWindow.latencies.push(result.latency);
      if (result.ip) metrics.currentWindow.ips.add(result.ip);
    } else {
      const err = result.error || 'UNKNOWN';
      metrics.currentWindow.errors[err] = (metrics.currentWindow.errors[err] || 0) + 1;
      metrics.overall.errors[err] = (metrics.overall.errors[err] || 0) + 1;
    }
    
    await new Promise(r => setTimeout(r, requestInterval));
  }
  
  clearInterval(reportTimer);
  rotateWindow(); // Final window
  printFinalReport();
}

runStabilityTest().catch(console.error);
