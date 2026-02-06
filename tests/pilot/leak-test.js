#!/usr/bin/env node
/**
 * IPLoop IP Leak Detection Test
 * 
 * Checks for IP/DNS leaks through proxy connections
 */

const http = require('http');
const https = require('https');
const dns = require('dns');
const { execSync } = require('child_process');

const CONFIG = {
  proxyHost: process.env.PROXY_HOST || 'localhost',
  proxyPort: parseInt(process.env.PROXY_PORT || '8080'),
  proxyUser: process.env.PROXY_USER || 'test_customer',
  proxyPass: process.env.PROXY_PASS || 'test_api_key'
};

// Get server's real public IP
function getServerIP() {
  try {
    return execSync('curl -s https://api.ipify.org', { timeout: 10000 }).toString().trim();
  } catch {
    return null;
  }
}

function makeProxyRequest(url) {
  return new Promise((resolve, reject) => {
    const auth = `${CONFIG.proxyUser}:${CONFIG.proxyPass}`;
    
    const options = {
      host: CONFIG.proxyHost,
      port: CONFIG.proxyPort,
      method: 'GET',
      path: url,
      headers: {
        'Proxy-Authorization': 'Basic ' + Buffer.from(auth).toString('base64'),
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
      },
      timeout: 30000
    };
    
    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => resolve({ status: res.statusCode, data }));
    });
    
    req.on('error', reject);
    req.on('timeout', () => { req.destroy(); reject(new Error('Timeout')); });
    req.end();
  });
}

async function runLeakTests() {
  console.log('🔍 IPLoop IP Leak Detection Test');
  console.log('='.repeat(50));
  
  // Get our real IP first
  console.log('\nDetecting server real IP...');
  const serverIP = getServerIP();
  if (serverIP) {
    console.log(`Server real IP: ${serverIP}`);
  } else {
    console.log('⚠️  Could not detect server IP');
  }
  
  const issues = [];
  
  // Test 1: Check proxy IP
  console.log('\n--- Test 1: Proxy IP Check ---');
  try {
    const result = await makeProxyRequest('https://api.ipify.org?format=json');
    const data = JSON.parse(result.data);
    console.log(`Proxy IP: ${data.ip}`);
    
    if (serverIP && data.ip === serverIP) {
      console.log('🔴 LEAK: Proxy returning server IP!');
      issues.push('IP leak: proxy returns server IP');
    } else {
      console.log('✅ Different IP (good)');
    }
  } catch (err) {
    console.log(`❌ Error: ${err.message}`);
    issues.push(`IP check failed: ${err.message}`);
  }
  
  // Test 2: Check headers for leaks
  console.log('\n--- Test 2: Header Leak Check ---');
  try {
    const result = await makeProxyRequest('https://httpbin.org/headers');
    const data = JSON.parse(result.data);
    const headers = data.headers;
    
    console.log('Checking for leak headers...');
    
    const leakHeaders = [
      'X-Forwarded-For',
      'X-Real-Ip',
      'Via',
      'X-Originating-Ip',
      'X-Client-Ip'
    ];
    
    for (const h of leakHeaders) {
      if (headers[h]) {
        const value = headers[h];
        if (serverIP && value.includes(serverIP)) {
          console.log(`🔴 LEAK in ${h}: ${value}`);
          issues.push(`Header leak: ${h} contains server IP`);
        } else {
          console.log(`⚠️  ${h}: ${value} (check if this is expected)`);
        }
      }
    }
    
    if (leakHeaders.every(h => !headers[h])) {
      console.log('✅ No leak headers detected');
    }
  } catch (err) {
    console.log(`❌ Error: ${err.message}`);
  }
  
  // Test 3: WebRTC-style leak (different IP services)
  console.log('\n--- Test 3: Consistency Check ---');
  const ipServices = [
    'https://api.ipify.org?format=json',
    'http://ip-api.com/json',
    'https://httpbin.org/ip'
  ];
  
  const ips = [];
  for (const service of ipServices) {
    try {
      const result = await makeProxyRequest(service);
      const data = JSON.parse(result.data);
      const ip = data.ip || data.query || data.origin;
      ips.push({ service, ip });
      console.log(`${service.split('/')[2]}: ${ip}`);
    } catch (err) {
      console.log(`${service.split('/')[2]}: Error - ${err.message}`);
    }
  }
  
  const uniqueIps = [...new Set(ips.map(i => i.ip))];
  if (uniqueIps.length === 1) {
    console.log('✅ All services report same IP');
  } else if (uniqueIps.length > 1) {
    console.log(`⚠️  Different IPs reported: ${uniqueIps.join(', ')}`);
    // This isn't necessarily a leak - could be IP rotation
  }
  
  // Test 4: Check for timezone/locale leaks
  console.log('\n--- Test 4: Fingerprint Check ---');
  try {
    const result = await makeProxyRequest('https://httpbin.org/headers');
    const data = JSON.parse(result.data);
    const headers = data.headers;
    
    if (headers['Accept-Language']) {
      console.log(`Accept-Language: ${headers['Accept-Language']}`);
    }
    
    // Check User-Agent isn't leaking something unexpected
    if (headers['User-Agent']) {
      console.log(`User-Agent: ${headers['User-Agent']}`);
    }
  } catch (err) {
    console.log(`Error: ${err.message}`);
  }
  
  // Summary
  console.log('\n' + '='.repeat(50));
  console.log('LEAK TEST SUMMARY');
  console.log('='.repeat(50));
  
  if (issues.length === 0) {
    console.log('✅ No IP leaks detected!');
  } else {
    console.log('Issues found:');
    issues.forEach(i => console.log(`  🔴 ${i}`));
  }
}

runLeakTests().catch(console.error);
