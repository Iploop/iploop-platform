#!/usr/bin/env node
/**
 * IPLoop IP Rotation Test
 * 
 * Tests:
 * - IP rotation per request
 * - Rotation frequency
 * - IP pool diversity
 */

const http = require('http');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key',
  
  requestCount: parseInt(process.env.REQUESTS || '50')
};

function makeRequest(rotationType = 'rotating') {
  return new Promise((resolve, reject) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}-sesstype-${rotationType}`;
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: 'https://httpbin.org/ip',
      headers: {
        'Host': 'httpbin.org',
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64')
      },
      timeout: 30000
    };
    
    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const json = JSON.parse(data);
          resolve({ ip: json.origin, status: res.statusCode });
        } catch (e) {
          reject(new Error('Parse error'));
        }
      });
    });
    
    req.on('error', reject);
    req.on('timeout', () => { req.destroy(); reject(new Error('Timeout')); });
    req.end();
  });
}

async function runRotationTest() {
  console.log('🔄 IPLoop IP Rotation Test');
  console.log('='.repeat(60));
  console.log(`Proxy: ${CONFIG.proxyHost}:${CONFIG.proxyPort}`);
  console.log(`Requests: ${CONFIG.requestCount}`);
  console.log('='.repeat(60));
  console.log('');
  
  const ips = [];
  const ipCounts = {};
  let errors = 0;
  
  console.log('Making requests with rotating session type...\n');
  
  for (let i = 0; i < CONFIG.requestCount; i++) {
    try {
      const result = await makeRequest('rotating');
      ips.push(result.ip);
      ipCounts[result.ip] = (ipCounts[result.ip] || 0) + 1;
      
      // Progress
      if ((i + 1) % 10 === 0) {
        const uniqueSoFar = Object.keys(ipCounts).length;
        console.log(`  [${i + 1}/${CONFIG.requestCount}] Unique IPs so far: ${uniqueSoFar}`);
      }
    } catch (err) {
      errors++;
    }
    
    // Small delay
    await new Promise(r => setTimeout(r, 200));
  }
  
  // Analysis
  const uniqueIps = Object.keys(ipCounts);
  const totalRequests = ips.length;
  
  // Calculate rotation rate
  let rotations = 0;
  for (let i = 1; i < ips.length; i++) {
    if (ips[i] !== ips[i-1]) rotations++;
  }
  const rotationRate = ((rotations / (totalRequests - 1)) * 100).toFixed(1);
  
  // Find most/least used IPs
  const sorted = Object.entries(ipCounts).sort((a, b) => b[1] - a[1]);
  const mostUsed = sorted[0];
  const leastUsed = sorted[sorted.length - 1];
  
  // Report
  console.log('\n' + '='.repeat(60));
  console.log('📊 ROTATION RESULTS');
  console.log('='.repeat(60));
  console.log(`Total requests: ${totalRequests}`);
  console.log(`Errors: ${errors}`);
  console.log(`Unique IPs: ${uniqueIps.length}`);
  console.log(`Rotation rate: ${rotationRate}% (IP changed between requests)`);
  console.log(`IP diversity ratio: ${(uniqueIps.length / totalRequests * 100).toFixed(1)}%`);
  
  console.log('\n📍 IP Distribution:');
  console.log(`  Most used:  ${mostUsed[0]} (${mostUsed[1]} times)`);
  console.log(`  Least used: ${leastUsed[0]} (${leastUsed[1]} times)`);
  
  if (uniqueIps.length <= 10) {
    console.log('\n  All IPs:');
    sorted.forEach(([ip, count]) => {
      const bar = '█'.repeat(Math.ceil(count / totalRequests * 20));
      console.log(`    ${ip.padEnd(16)} ${bar} ${count}`);
    });
  } else {
    console.log('\n  Top 5 IPs:');
    sorted.slice(0, 5).forEach(([ip, count]) => {
      const bar = '█'.repeat(Math.ceil(count / totalRequests * 20));
      console.log(`    ${ip.padEnd(16)} ${bar} ${count}`);
    });
  }
  
  // Verdict
  console.log('\n' + '='.repeat(60));
  
  if (uniqueIps.length === 1) {
    console.log('⚠️  WARNING: Only 1 IP observed - rotation may not be working');
    console.log('    (Could be expected if only 1 peer connected)');
  } else if (rotationRate < 50 && uniqueIps.length > 1) {
    console.log('⚠️  LOW ROTATION: IPs are rotating less than expected');
  } else if (uniqueIps.length >= 3 && rotationRate >= 50) {
    console.log('✅ GOOD: Healthy IP rotation observed');
  } else {
    console.log('ℹ️  Limited IP pool - add more peers for better diversity');
  }
}

runRotationTest().catch(console.error);
