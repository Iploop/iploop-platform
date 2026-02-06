#!/usr/bin/env node
/**
 * IPLoop Bandwidth Test
 * 
 * Tests:
 * - Download speed through proxy
 * - Large file handling
 * - Bandwidth metering accuracy
 * - Chunked transfer encoding
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

const TESTS = [
  { name: '1KB', url: 'https://httpbin.org/bytes/1024', expectedSize: 1024 },
  { name: '10KB', url: 'https://httpbin.org/bytes/10240', expectedSize: 10240 },
  { name: '100KB', url: 'https://httpbin.org/bytes/102400', expectedSize: 102400 },
  { name: '1MB', url: 'https://httpbin.org/bytes/1048576', expectedSize: 1048576 },
  { name: 'Stream 10KB', url: 'https://httpbin.org/stream-bytes/10240', expectedSize: 10240, chunked: true },
  { name: 'Drip 5KB/1s', url: 'https://httpbin.org/drip?duration=1&numbytes=5120&code=200', expectedSize: 5120, slow: true }
];

function makeRequest(url) {
  return new Promise((resolve, reject) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
    const startTime = Date.now();
    let bytesReceived = 0;
    let chunks = 0;
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: url,
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout: 60000
    };
    
    const req = http.request(options, (res) => {
      res.on('data', chunk => {
        bytesReceived += chunk.length;
        chunks++;
      });
      
      res.on('end', () => {
        const elapsed = Date.now() - startTime;
        const speedMbps = (bytesReceived * 8 / elapsed / 1000).toFixed(2);
        
        resolve({
          success: res.statusCode === 200,
          statusCode: res.statusCode,
          bytesReceived,
          chunks,
          elapsed,
          speedMbps
        });
      });
    });
    
    req.on('error', reject);
    req.on('timeout', () => { req.destroy(); reject(new Error('Timeout')); });
    req.end();
  });
}

async function runBandwidthTest() {
  console.log('📊 IPLoop Bandwidth Test');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log('='.repeat(60));
  console.log('');
  
  const results = [];
  
  for (const test of TESTS) {
    process.stdout.write(`Testing ${test.name.padEnd(15)}... `);
    
    try {
      const result = await makeRequest(test.url);
      results.push({ ...test, ...result });
      
      if (result.success) {
        const sizeMatch = Math.abs(result.bytesReceived - test.expectedSize) < test.expectedSize * 0.1;
        const icon = sizeMatch ? '✅' : '⚠️';
        console.log(`${icon} ${result.bytesReceived} bytes in ${result.elapsed}ms (${result.speedMbps} Mbps)`);
        
        if (!sizeMatch) {
          console.log(`   Expected ~${test.expectedSize}, got ${result.bytesReceived}`);
        }
      } else {
        console.log(`❌ HTTP ${result.statusCode}`);
      }
    } catch (err) {
      results.push({ ...test, error: err.message });
      console.log(`❌ Error: ${err.message}`);
    }
  }
  
  // Summary
  console.log('\n' + '='.repeat(60));
  console.log('BANDWIDTH SUMMARY');
  console.log('='.repeat(60));
  
  const successful = results.filter(r => r.success);
  if (successful.length > 0) {
    const avgSpeed = (successful.reduce((sum, r) => sum + parseFloat(r.speedMbps), 0) / successful.length).toFixed(2);
    const totalBytes = successful.reduce((sum, r) => sum + r.bytesReceived, 0);
    
    console.log(`Tests passed: ${successful.length}/${TESTS.length}`);
    console.log(`Total downloaded: ${(totalBytes / 1024 / 1024).toFixed(2)} MB`);
    console.log(`Average speed: ${avgSpeed} Mbps`);
    
    // Find fastest
    const fastest = successful.reduce((max, r) => parseFloat(r.speedMbps) > parseFloat(max.speedMbps) ? r : max);
    console.log(`Peak speed: ${fastest.speedMbps} Mbps (${fastest.name})`);
  }
  
  const failed = results.filter(r => !r.success || r.error);
  if (failed.length > 0) {
    console.log(`\n❌ Failed tests: ${failed.map(r => r.name).join(', ')}`);
  }
}

runBandwidthTest().catch(console.error);
