#!/usr/bin/env node
/**
 * Latency Deep Analysis Test
 * 
 * Tests:
 * - Baseline latency
 * - Latency under load
 * - Latency consistency
 * - Timeout behavior
 * - First byte time vs total time
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

function makeTimedRequest(url, customTimeout = 30000) {
  return new Promise((resolve) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
    const times = {
      start: Date.now(),
      connected: null,
      firstByte: null,
      end: null
    };
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: url,
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout: customTimeout
    };
    
    const req = http.request(options, (res) => {
      times.connected = Date.now();
      let size = 0;
      let firstByte = false;
      
      res.on('data', chunk => {
        if (!firstByte) {
          times.firstByte = Date.now();
          firstByte = true;
        }
        size += chunk.length;
      });
      
      res.on('end', () => {
        times.end = Date.now();
        resolve({
          success: res.statusCode === 200,
          statusCode: res.statusCode,
          size,
          times: {
            total: times.end - times.start,
            toFirstByte: times.firstByte ? times.firstByte - times.start : null,
            download: times.firstByte ? times.end - times.firstByte : null
          }
        });
      });
    });
    
    req.on('error', (err) => {
      resolve({ success: false, error: err.code, times: { total: Date.now() - times.start } });
    });
    
    req.on('timeout', () => {
      req.destroy();
      resolve({ success: false, error: 'TIMEOUT', times: { total: Date.now() - times.start } });
    });
    
    req.end();
  });
}

function percentile(arr, p) {
  if (arr.length === 0) return 0;
  const sorted = [...arr].sort((a, b) => a - b);
  const idx = Math.ceil((p / 100) * sorted.length) - 1;
  return sorted[Math.max(0, idx)];
}

function stats(arr) {
  if (arr.length === 0) return { min: 0, max: 0, avg: 0, p50: 0, p95: 0, p99: 0 };
  return {
    min: Math.min(...arr),
    max: Math.max(...arr),
    avg: Math.round(arr.reduce((a, b) => a + b, 0) / arr.length),
    p50: percentile(arr, 50),
    p95: percentile(arr, 95),
    p99: percentile(arr, 99),
    stdDev: Math.round(Math.sqrt(arr.map(x => Math.pow(x - (arr.reduce((a, b) => a + b, 0) / arr.length), 2)).reduce((a, b) => a + b, 0) / arr.length))
  };
}

async function testBaselineLatency() {
  console.log('\n--- Test 1: Baseline Latency (50 sequential requests) ---\n');
  
  const latencies = [];
  const ttfb = []; // Time to first byte
  
  for (let i = 0; i < 50; i++) {
    const result = await makeTimedRequest('https://httpbin.org/ip');
    if (result.success) {
      latencies.push(result.times.total);
      if (result.times.toFirstByte) ttfb.push(result.times.toFirstByte);
    }
    
    if ((i + 1) % 10 === 0) {
      process.stdout.write(`  ${i + 1}/50...`);
    }
    
    await new Promise(r => setTimeout(r, 100));
  }
  console.log(' done\n');
  
  const latStats = stats(latencies);
  const ttfbStats = stats(ttfb);
  
  console.log('Total Latency:');
  console.log(`  Min: ${latStats.min}ms | Max: ${latStats.max}ms | Avg: ${latStats.avg}ms`);
  console.log(`  P50: ${latStats.p50}ms | P95: ${latStats.p95}ms | P99: ${latStats.p99}ms`);
  console.log(`  Std Dev: ${latStats.stdDev}ms`);
  
  console.log('\nTime to First Byte:');
  console.log(`  Min: ${ttfbStats.min}ms | Max: ${ttfbStats.max}ms | Avg: ${ttfbStats.avg}ms`);
  console.log(`  P50: ${ttfbStats.p50}ms | P95: ${ttfbStats.p95}ms`);
  
  return { latencies: latStats, ttfb: ttfbStats };
}

async function testLatencyUnderLoad() {
  console.log('\n--- Test 2: Latency Under Load (50 concurrent) ---\n');
  
  const promises = [];
  for (let i = 0; i < 50; i++) {
    promises.push(makeTimedRequest('https://httpbin.org/ip'));
  }
  
  const results = await Promise.all(promises);
  const latencies = results.filter(r => r.success).map(r => r.times.total);
  const errors = results.filter(r => !r.success).length;
  
  const latStats = stats(latencies);
  
  console.log(`Success: ${latencies.length}/50 | Errors: ${errors}`);
  console.log(`Latency under load:`);
  console.log(`  Min: ${latStats.min}ms | Max: ${latStats.max}ms | Avg: ${latStats.avg}ms`);
  console.log(`  P50: ${latStats.p50}ms | P95: ${latStats.p95}ms | P99: ${latStats.p99}ms`);
  
  return latStats;
}

async function testLatencyConsistency() {
  console.log('\n--- Test 3: Latency Consistency (5 batches of 20) ---\n');
  
  const batchResults = [];
  
  for (let batch = 0; batch < 5; batch++) {
    const latencies = [];
    
    for (let i = 0; i < 20; i++) {
      const result = await makeTimedRequest('https://httpbin.org/ip');
      if (result.success) latencies.push(result.times.total);
      await new Promise(r => setTimeout(r, 50));
    }
    
    const batchStats = stats(latencies);
    batchResults.push(batchStats);
    
    console.log(`Batch ${batch + 1}: Avg ${batchStats.avg}ms | P95 ${batchStats.p95}ms`);
    
    await new Promise(r => setTimeout(r, 2000)); // Gap between batches
  }
  
  // Check consistency
  const avgLatencies = batchResults.map(b => b.avg);
  const variance = Math.max(...avgLatencies) - Math.min(...avgLatencies);
  
  console.log(`\nVariance between batches: ${variance}ms`);
  console.log(variance < 500 ? '✅ Consistent' : '⚠️  High variance');
  
  return variance < 500;
}

async function testTimeoutBehavior() {
  console.log('\n--- Test 4: Timeout Behavior ---\n');
  
  const tests = [
    { name: '5s timeout, 3s delay', timeout: 5000, url: 'https://httpbin.org/delay/3', shouldSucceed: true },
    { name: '2s timeout, 5s delay', timeout: 2000, url: 'https://httpbin.org/delay/5', shouldSucceed: false },
    { name: '10s timeout, 8s delay', timeout: 10000, url: 'https://httpbin.org/delay/8', shouldSucceed: true },
    { name: '1s timeout, 2s delay', timeout: 1000, url: 'https://httpbin.org/delay/2', shouldSucceed: false }
  ];
  
  for (const test of tests) {
    process.stdout.write(`${test.name}: `);
    
    const result = await makeTimedRequest(test.url, test.timeout);
    const passed = test.shouldSucceed ? result.success : !result.success;
    
    if (passed) {
      console.log(`✅ ${result.success ? `Completed in ${result.times.total}ms` : `Timed out after ${result.times.total}ms`}`);
    } else {
      console.log(`❌ Unexpected: ${result.success ? 'succeeded' : 'failed'}`);
    }
  }
}

async function testDifferentPayloadSizes() {
  console.log('\n--- Test 5: Latency by Payload Size ---\n');
  
  const sizes = [
    { name: '1KB', url: 'https://httpbin.org/bytes/1024' },
    { name: '10KB', url: 'https://httpbin.org/bytes/10240' },
    { name: '100KB', url: 'https://httpbin.org/bytes/102400' },
    { name: '1MB', url: 'https://httpbin.org/bytes/1048576' }
  ];
  
  for (const size of sizes) {
    const latencies = [];
    
    for (let i = 0; i < 5; i++) {
      const result = await makeTimedRequest(size.url, 60000);
      if (result.success) latencies.push(result.times.total);
    }
    
    if (latencies.length > 0) {
      const avg = Math.round(latencies.reduce((a, b) => a + b, 0) / latencies.length);
      console.log(`${size.name.padEnd(8)}: Avg ${avg}ms`);
    } else {
      console.log(`${size.name.padEnd(8)}: Failed`);
    }
  }
}

async function runLatencyTests() {
  console.log('⏱️  Latency Deep Analysis Test');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log('='.repeat(60));
  
  const baseline = await testBaselineLatency();
  const underLoad = await testLatencyUnderLoad();
  const consistent = await testLatencyConsistency();
  await testTimeoutBehavior();
  await testDifferentPayloadSizes();
  
  console.log('\n' + '='.repeat(60));
  console.log('LATENCY TEST SUMMARY');
  console.log('='.repeat(60));
  
  console.log('\nBaseline vs Under Load:');
  console.log(`  Baseline P95: ${baseline.latencies.p95}ms`);
  console.log(`  Under Load P95: ${underLoad.p95}ms`);
  console.log(`  Degradation: ${((underLoad.p95 / baseline.latencies.p95 - 1) * 100).toFixed(0)}%`);
  
  const issues = [];
  
  if (baseline.latencies.p95 > 5000) {
    issues.push('High baseline latency (P95 > 5s)');
  }
  if (underLoad.p95 > baseline.latencies.p95 * 3) {
    issues.push('Severe latency degradation under load');
  }
  if (!consistent) {
    issues.push('Inconsistent latency between batches');
  }
  
  if (issues.length === 0) {
    console.log('\n✅ Latency characteristics are healthy');
  } else {
    console.log('\n⚠️  Issues:');
    issues.forEach(i => console.log(`  - ${i}`));
  }
}

runLatencyTests().catch(console.error);
