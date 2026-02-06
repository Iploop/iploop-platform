#!/usr/bin/env node
/**
 * Concurrency Edge Cases Test
 * 
 * Tests:
 * - Same URL from many clients
 * - Interleaved requests
 * - Request storms
 * - Slow clients
 * - Fast clients
 * - Mixed speeds
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

function makeRequest(url, timeout = 30000) {
  return new Promise((resolve) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
    const start = Date.now();
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: url,
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout
    };
    
    const req = http.request(options, (res) => {
      let size = 0;
      res.on('data', chunk => size += chunk.length);
      res.on('end', () => {
        resolve({ success: true, status: res.statusCode, latency: Date.now() - start, size });
      });
    });
    
    req.on('error', (err) => resolve({ success: false, error: err.code }));
    req.on('timeout', () => { req.destroy(); resolve({ success: false, error: 'TIMEOUT' }); });
    req.end();
  });
}

async function test1_SameUrlStorm() {
  console.log('\n--- Test 1: Same URL Storm (100 requests to same endpoint) ---');
  
  const url = 'https://httpbin.org/ip';
  const promises = [];
  
  for (let i = 0; i < 100; i++) {
    promises.push(makeRequest(url));
  }
  
  const results = await Promise.all(promises);
  const success = results.filter(r => r.success).length;
  const latencies = results.filter(r => r.success).map(r => r.latency);
  const avgLatency = latencies.reduce((a, b) => a + b, 0) / latencies.length;
  
  console.log(`Success: ${success}/100`);
  console.log(`Avg latency: ${avgLatency.toFixed(0)}ms`);
  console.log(`Min/Max: ${Math.min(...latencies)}ms / ${Math.max(...latencies)}ms`);
  
  return success >= 95;
}

async function test2_DifferentUrls() {
  console.log('\n--- Test 2: Different URLs (100 requests to different endpoints) ---');
  
  const urls = [
    'https://httpbin.org/ip',
    'https://httpbin.org/get',
    'https://httpbin.org/headers',
    'https://httpbin.org/user-agent',
    'https://httpbin.org/uuid'
  ];
  
  const promises = [];
  for (let i = 0; i < 100; i++) {
    promises.push(makeRequest(urls[i % urls.length]));
  }
  
  const results = await Promise.all(promises);
  const success = results.filter(r => r.success).length;
  
  console.log(`Success: ${success}/100`);
  
  return success >= 95;
}

async function test3_InterleavedSpeeds() {
  console.log('\n--- Test 3: Interleaved Fast/Slow Requests ---');
  
  const promises = [];
  
  // Mix of fast and slow requests
  for (let i = 0; i < 20; i++) {
    // Fast request
    promises.push(makeRequest('https://httpbin.org/ip'));
    // Slow request (3 second delay)
    promises.push(makeRequest('https://httpbin.org/delay/3'));
  }
  
  const start = Date.now();
  const results = await Promise.all(promises);
  const elapsed = Date.now() - start;
  
  const fastResults = results.filter((_, i) => i % 2 === 0);
  const slowResults = results.filter((_, i) => i % 2 === 1);
  
  const fastSuccess = fastResults.filter(r => r.success).length;
  const slowSuccess = slowResults.filter(r => r.success).length;
  
  console.log(`Fast requests: ${fastSuccess}/20`);
  console.log(`Slow requests: ${slowSuccess}/20`);
  console.log(`Total time: ${elapsed}ms (slow shouldn't block fast)`);
  
  // Fast requests should complete much faster than slow ones
  const fastAvg = fastResults.filter(r => r.success).reduce((a, r) => a + r.latency, 0) / fastSuccess;
  const slowAvg = slowResults.filter(r => r.success).reduce((a, r) => a + r.latency, 0) / slowSuccess;
  
  console.log(`Fast avg latency: ${fastAvg.toFixed(0)}ms`);
  console.log(`Slow avg latency: ${slowAvg.toFixed(0)}ms`);
  
  return fastSuccess >= 18 && slowSuccess >= 15 && fastAvg < 5000;
}

async function test4_RequestBurst() {
  console.log('\n--- Test 4: Request Burst (500 requests instant) ---');
  
  const promises = [];
  for (let i = 0; i < 500; i++) {
    promises.push(makeRequest('https://httpbin.org/ip'));
  }
  
  const start = Date.now();
  const results = await Promise.all(promises);
  const elapsed = Date.now() - start;
  
  const success = results.filter(r => r.success).length;
  const errors = {};
  results.filter(r => !r.success).forEach(r => {
    errors[r.error] = (errors[r.error] || 0) + 1;
  });
  
  console.log(`Success: ${success}/500 (${(success/500*100).toFixed(1)}%)`);
  console.log(`Time: ${elapsed}ms`);
  console.log(`Throughput: ${(success / (elapsed/1000)).toFixed(1)} req/s`);
  
  if (Object.keys(errors).length > 0) {
    console.log(`Errors: ${JSON.stringify(errors)}`);
  }
  
  return success >= 400; // 80% success under burst
}

async function test5_SustainedLoad() {
  console.log('\n--- Test 5: Sustained Load (50 concurrent for 30s) ---');
  
  const duration = 30000;
  const concurrency = 50;
  const endTime = Date.now() + duration;
  
  let total = 0;
  let success = 0;
  let active = 0;
  
  async function worker() {
    while (Date.now() < endTime) {
      active++;
      const result = await makeRequest('https://httpbin.org/ip');
      active--;
      total++;
      if (result.success) success++;
      await new Promise(r => setTimeout(r, 100));
    }
  }
  
  // Start workers
  const workers = [];
  for (let i = 0; i < concurrency; i++) {
    workers.push(worker());
  }
  
  // Progress
  const progress = setInterval(() => {
    const elapsed = (Date.now() - (endTime - duration)) / 1000;
    console.log(`  [${elapsed.toFixed(0)}s] Active: ${active} | Total: ${total} | Success: ${success}`);
  }, 5000);
  
  await Promise.all(workers);
  clearInterval(progress);
  
  const successRate = (success / total * 100).toFixed(1);
  console.log(`\nTotal requests: ${total}`);
  console.log(`Success rate: ${successRate}%`);
  console.log(`Throughput: ${(total / 30).toFixed(1)} req/s`);
  
  return parseFloat(successRate) >= 95;
}

async function test6_SlowClientSimulation() {
  console.log('\n--- Test 6: Slow Client Simulation (reading slowly) ---');
  
  // This tests if slow consumers cause backpressure issues
  const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
  
  return new Promise((resolve) => {
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: 'https://httpbin.org/bytes/102400', // 100KB
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout: 60000
    };
    
    const req = http.request(options, (res) => {
      let received = 0;
      
      res.on('data', (chunk) => {
        received += chunk.length;
        // Simulate slow consumer by pausing
        res.pause();
        setTimeout(() => res.resume(), 50);
      });
      
      res.on('end', () => {
        console.log(`Received ${received} bytes with artificial delays`);
        resolve(received >= 100000);
      });
    });
    
    req.on('error', () => resolve(false));
    req.end();
  });
}

async function runConcurrencyTests() {
  console.log('🔀 Concurrency Edge Cases Test');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log('='.repeat(60));
  
  const results = [];
  
  results.push({ name: 'Same URL Storm', pass: await test1_SameUrlStorm() });
  results.push({ name: 'Different URLs', pass: await test2_DifferentUrls() });
  results.push({ name: 'Interleaved Speeds', pass: await test3_InterleavedSpeeds() });
  results.push({ name: 'Request Burst', pass: await test4_RequestBurst() });
  results.push({ name: 'Sustained Load', pass: await test5_SustainedLoad() });
  results.push({ name: 'Slow Client', pass: await test6_SlowClientSimulation() });
  
  console.log('\n' + '='.repeat(60));
  console.log('CONCURRENCY TEST RESULTS');
  console.log('='.repeat(60));
  
  results.forEach(r => {
    console.log(`${r.pass ? '✅' : '❌'} ${r.name}`);
  });
  
  const passed = results.filter(r => r.pass).length;
  console.log(`\nPassed: ${passed}/${results.length}`);
}

runConcurrencyTests().catch(console.error);
